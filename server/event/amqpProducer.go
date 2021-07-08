package event

import (
	"context"
	"encoding/json"
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
)

type publishMessage struct {
	amqp.Publishing
	Exchange, Key string
}

type amqpProducer struct {
	ctx             context.Context
	amqpURI         string
	exchange, keyNs string
	publishQ        chan *publishMessage
}

func NewAMQPProducer(ctx context.Context, uri, exchange, keyNs string) Producer {
	amqp := &amqpProducer{
		ctx:      ctx,
		amqpURI:  uri,
		exchange: exchange,
		keyNs:    keyNs,
		publishQ: make(chan *publishMessage, PublishQueueSize),
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
	conn, channel, err := p.setupConnection()
	if err != nil {
		return fmt.Errorf("error setting up AMQP connection: %w", err)
	}
	defer conn.Close()

	closed := channel.NotifyClose(make(chan *amqp.Error, 1))

	for {
		select {
		case <-p.ctx.Done():
			return p.ctx.Err()
		case err := <-closed:
			return fmt.Errorf("channel or connection closed: %w", err)
		case msg := <-p.publishQ:
			mandatory, immediate := false, false
			err := channel.Publish(p.exchange, msg.Key, mandatory, immediate, msg.Publishing)
			if err == amqp.ErrClosed {
				select {
				case p.publishQ <- msg:
				default:
					glog.Errorf("Failed to re-enqueue message: exchange=%q, key=%q, body=%q", p.exchange, msg.Key, msg.Body)
				}
				return err
			}

			if err != nil {
				glog.Errorf("Error publishing message: exchange=%q, key=%q, error=%q, body=%q", p.exchange, msg.Key, err, msg.Body)
				break
			}
			if glog.V(4) && rand.Float32() < PublishLogSampleRate {
				glog.Infof("Sampled: Message published: exchange=%q, key=%q, body=%q", p.exchange, msg.Key, msg.Body)
			}
		}
	}
}

func (p *amqpProducer) setupConnection() (*amqp.Connection, *amqp.Channel, error) {
	conn, err := amqp.Dial(p.amqpURI)
	if err != nil {
		return nil, nil, fmt.Errorf("dial: %w", err)
	}

	channel, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, nil, fmt.Errorf("open channel: %w", err)
	}

	var (
		durable     = true
		autoDeleted = false
		internal    = false
		noWait      = false
	)
	err = channel.ExchangeDeclare(p.exchange, "topic", durable, autoDeleted, internal, noWait, nil)
	if err != nil {
		conn.Close()
		return nil, nil, fmt.Errorf("exchange declare: %w", err)
	}
	return conn, channel, nil
}
