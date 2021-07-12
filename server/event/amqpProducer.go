package event

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/golang/glog"
	"github.com/streadway/amqp"
)

const (
	PublishQueueSize     = 100
	RetryMinDelay        = 5 * time.Second
	PublishLogSampleRate = 0.1
	MaxRetries           = 3
)

type publishMessage struct {
	amqp.Publishing
	Exchange, Key string

	// internal loop state
	retries int
}

type amqpChan interface {
	Publish(exchange, key string, mandatory, immediate bool, msg amqp.Publishing) error
}

type connectFunc func(ctx context.Context, uri, exchange string, confirms chan amqp.Confirmation, closed chan *amqp.Error) (amqpChan, error)

type amqpProducer struct {
	ctx             context.Context
	amqpURI         string
	exchange, keyNs string
	publishQ        chan *publishMessage
	connectFn       connectFunc
}

func NewAMQPProducer(ctx context.Context, uri, exchange, keyNs string) Producer {
	return newAMQPProducerInternal(ctx, uri, exchange, keyNs, amqpConnect)
}

func newAMQPProducerInternal(ctx context.Context, uri, exchange, keyNs string, connectFn connectFunc) Producer {
	amqp := &amqpProducer{
		ctx:       ctx,
		amqpURI:   uri,
		exchange:  exchange,
		keyNs:     keyNs,
		publishQ:  make(chan *publishMessage, PublishQueueSize),
		connectFn: connectFn,
	}
	go amqp.mainLoop()
	return amqp
}

func (p *amqpProducer) Publish(ctx context.Context, key string, body interface{}) error {
	bodyRaw, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to marshal body to json: %w", err)
	}
	select {
	case p.publishQ <- p.newPublishMessage(key, bodyRaw):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-p.ctx.Done():
		return fmt.Errorf("producer context done: %w", p.ctx.Err())
	}
}

func (p *amqpProducer) newPublishMessage(key string, bodyRaw []byte) *publishMessage {
	if p.keyNs != "" {
		key = p.keyNs + "." + key
	}
	return &publishMessage{
		Exchange: p.exchange,
		Key:      key,
		Publishing: amqp.Publishing{
			Headers:         amqp.Table{},
			ContentType:     "application/json",
			ContentEncoding: "",
			Body:            bodyRaw,
			DeliveryMode:    amqp.Transient,
			Priority:        1,
		},
	}
}

func (p *amqpProducer) mainLoop() {
	defer func() {
		if rec := recover(); rec != nil {
			glog.Fatalf("Panic in background AMQP publisher: value=%v", rec)
		}
	}()

	for {
		retryAfter := time.After(RetryMinDelay)
		err := p.connectAndLoopPublish()
		if p.ctx.Err() != nil {
			return
		}
		<-retryAfter
		glog.Errorf("Recovering AMQP connection: error=%q", err)
	}
}

func (p *amqpProducer) connectAndLoopPublish() error {
	var (
		ctx, cancel = context.WithCancel(p.ctx)
		confirms    = make(chan amqp.Confirmation, PublishQueueSize)
		closed      = make(chan *amqp.Error, 1)
	)
	defer cancel()
	channel, err := p.connectFn(ctx, p.amqpURI, p.exchange, confirms, closed)
	if err != nil {
		return fmt.Errorf("error setting up AMQP connection: %w", err)
	}

	nextMsgTag := uint64(1)
	outstandingMsgs := map[uint64]*publishMessage{}
	defer func() {
		// we only return on connection errors, so retry all outstanding messages
		for _, msg := range outstandingMsgs {
			p.retryMsg(msg)
		}
	}()

	for {
		select {
		case <-p.ctx.Done():
			return p.ctx.Err()
		case err := <-closed:
			return fmt.Errorf("channel or connection closed: %w", err)
		case msg := <-p.publishQ:
			mandatory, immediate := false, false
			err := channel.Publish(p.exchange, msg.Key, mandatory, immediate, msg.Publishing)
			if err != nil {
				p.retryMsg(msg)
				glog.Errorf("Error publishing message: exchange=%q, key=%q, error=%q, body=%q", p.exchange, msg.Key, err, msg.Body)
				return err
			}

			outstandingMsgs[nextMsgTag] = msg
			nextMsgTag++

			if glog.V(4) && rand.Float32() < PublishLogSampleRate {
				glog.Infof("Sampled: Message published: exchange=%q, key=%q, body=%q", p.exchange, msg.Key, msg.Body)
			}
		case conf, ok := <-confirms:
			if !ok {
				return errors.New("channel or connection closed")
			}
			tag, success := conf.DeliveryTag, conf.Ack
			msg, ok := outstandingMsgs[tag]
			if !ok {
				glog.Errorf("Received confirmation for unknown message: tag=%v, success=%v", tag, success)
				break
			}
			delete(outstandingMsgs, tag)
			if !success {
				p.retryMsg(msg)
			}
		}
	}
}

func (p *amqpProducer) retryMsg(msg *publishMessage) {
	msg.retries++
	if msg.retries >= MaxRetries {
		glog.Errorf("Dropping message reaching max retries: exchange=%q, key=%q, body=%q", p.exchange, msg.Key, msg.Body)
		return
	}

	select {
	case p.publishQ <- msg:
	default:
		glog.Errorf("Failed to re-enqueue message: exchange=%q, key=%q, body=%q", p.exchange, msg.Key, msg.Body)
	}
}

func amqpConnect(ctx context.Context, uri, exchange string,
	confirms chan amqp.Confirmation, closed chan *amqp.Error) (amqpChan, error) {

	conn, err := amqp.Dial(uri)
	if err != nil {
		return nil, fmt.Errorf("dial: %w", err)
	}
	go func() {
		<-ctx.Done()
		conn.Close()
	}()

	channel, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("open channel: %w", err)
	}
	if err := channel.Confirm(false); err != nil {
		conn.Close()
		return nil, fmt.Errorf("request confirms: %w", err)
	}

	var (
		durable     = true
		autoDeleted = false
		internal    = false
		noWait      = false
	)
	err = channel.ExchangeDeclare(exchange, "topic", durable, autoDeleted, internal, noWait, nil)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("exchange declare: %w", err)
	}
	channel.NotifyPublish(confirms)
	channel.NotifyClose(closed)
	return channel, nil
}
