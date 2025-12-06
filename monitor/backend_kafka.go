package monitor

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang/glog"
	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl/plain"
)

const (
	KafkaBatchInterval  = 1 * time.Second
	KafkaRequestTimeout = 60 * time.Second
	KafkaBatchSize      = 100
	KafkaChannelSize    = 100
)

type gatewayEvent struct {
	ID        *string     `json:"id,omitempty"`
	Type      *string     `json:"type"`
	Timestamp *string     `json:"timestamp"`
	Gateway   *string     `json:"gateway,omitempty"`
	Data      interface{} `json:"data"`
}

type kafkaProducer struct {
	writer         *kafka.Writer
	topic          string
	events         chan gatewayEvent
	gatewayAddress string
}

type kafkaBackend struct {
	producer *kafkaProducer
	wg       sync.WaitGroup
}

func init() {
	RegisterBackendFactory("kafka", newKafkaBackend)
}

func newKafkaBackend(u *url.URL, opts BackendOptions) (EventBackend, error) {
	query := u.Query()

	topic := strings.TrimSpace(query.Get("topic"))
	if topic == "" {
		return nil, fmt.Errorf("kafka sink %q missing topic", u.String())
	}

	brokers := splitAndTrim(query.Get("brokers"), ",")
	if len(brokers) == 0 {
		host := strings.TrimSpace(u.Host)
		if host == "" {
			return nil, fmt.Errorf("kafka sink %q missing brokers", u.String())
		}
		brokers = []string{host}
	}

	username := ""
	password := ""
	if u.User != nil {
		username = u.User.Username()
		password, _ = u.User.Password()
	}

	producer, err := newKafkaProducer(brokers, username, password, topic, opts.GatewayAddress)
	if err != nil {
		return nil, err
	}

	return &kafkaBackend{producer: producer}, nil
}

func newKafkaProducer(brokers []string, user, password, topic, gatewayAddress string) (*kafkaProducer, error) {
	if len(brokers) == 0 {
		return nil, fmt.Errorf("kafka producer requires at least one broker")
	}

	dialer := &kafka.Dialer{Timeout: KafkaRequestTimeout, DualStack: true}

	if user != "" {
		dialer.TLS = &tls.Config{MinVersion: tls.VersionTLS12}
		dialer.SASLMechanism = &plain.Mechanism{Username: user, Password: password}
	}

	writer := kafka.NewWriter(kafka.WriterConfig{
		Brokers:  brokers,
		Topic:    topic,
		Balancer: kafka.CRC32Balancer{},
		Dialer:   dialer,
	})

	return &kafkaProducer{
		writer:         writer,
		topic:          topic,
		events:         make(chan gatewayEvent, KafkaChannelSize),
		gatewayAddress: gatewayAddress,
	}, nil
}

func (b *kafkaBackend) Start(ctx context.Context) error {
	b.wg.Add(1)
	go func() {
		defer b.wg.Done()
		b.producer.run(ctx)
	}()
	return nil
}

func (b *kafkaBackend) Publish(_ context.Context, batch []EventEnvelope) error {
	for _, evt := range batch {
		payload, err := decodePayload(evt.Payload)
		if err != nil {
			glog.Errorf("kafka backend payload decode error for event %s: %v", evt.ID, err)
			payload = string(evt.Payload)
		}

		id := evt.ID
		ts := evt.Timestamp
		tsMillis := ts.UnixMilli()
		tsStr := strconv.FormatInt(tsMillis, 10)

		kafkaEvt := gatewayEvent{
			Data:      payload,
			Timestamp: stringPtr(tsStr),
		}

		if id != "" {
			kafkaEvt.ID = stringPtr(id)
		}

		if evt.Type != "" {
			kafkaEvt.Type = stringPtr(evt.Type)
		}

		if b.producer.gatewayAddress != "" {
			kafkaEvt.Gateway = stringPtr(b.producer.gatewayAddress)
		}

		b.producer.enqueue(kafkaEvt)
	}
	return nil
}

func (b *kafkaBackend) Stop(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		defer close(done)
		b.wg.Wait()
	}()

	select {
	case <-done:
	case <-ctx.Done():
	}

	return b.producer.close()
}

func (p *kafkaProducer) enqueue(evt gatewayEvent) {
	select {
	case p.events <- evt:
	default:
		if evt.Type != nil {
			glog.Warningf("kafka producer event queue is full, dropping event %q", *evt.Type)
		} else {
			glog.Warning("kafka producer event queue is full, dropping event with unknown type")
		}
	}
}

func (p *kafkaProducer) run(ctx context.Context) {
	ticker := time.NewTicker(KafkaBatchInterval)
	defer ticker.Stop()

	eventsBatch := make([]kafka.Message, 0, KafkaBatchSize)

	flush := func() {
		if len(eventsBatch) == 0 {
			return
		}
		p.sendBatch(eventsBatch)
		eventsBatch = eventsBatch[:0]
	}

	for {
		select {
		case <-ctx.Done():
			flush()
			return
		case event, ok := <-p.events:
			if !ok {
				flush()
				return
			}

			value, err := json.Marshal(event)
			if err != nil {
				glog.Errorf("error while marshalling gateway event to Kafka, err=%v", err)
				continue
			}

			key := ""
			if event.ID != nil {
				key = *event.ID
			}
			if key == "" {
				key = strconv.FormatInt(time.Now().UnixNano(), 10)
			}

			msg := kafka.Message{Key: []byte(key), Value: value}
			eventsBatch = append(eventsBatch, msg)

			if len(eventsBatch) >= KafkaBatchSize {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}

func (p *kafkaProducer) sendBatch(eventsBatch []kafka.Message) {
	if len(eventsBatch) == 0 {
		return
	}

	kafkaWriteRetries := 3
	var writeErr error
	for i := 0; i < kafkaWriteRetries; i++ {
		writeErr = p.writer.WriteMessages(context.Background(), eventsBatch...)
		if writeErr == nil {
			return
		}
		glog.Warningf("error while sending gateway log batch to Kafka, retrying, topic=%s, try=%d, err=%v", p.topic, i, writeErr)
	}
	if writeErr != nil {
		glog.Errorf("error while sending gateway log batch to Kafka, the gateway logs are lost, err=%v", writeErr)
	}
}

func (p *kafkaProducer) close() error {
	return p.writer.Close()
}

func decodePayload(raw json.RawMessage) (interface{}, error) {
	if raw == nil {
		return nil, nil
	}
	if len(raw) == 0 {
		return nil, nil
	}
	var decoded interface{}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return nil, err
	}
	return decoded, nil
}

func splitAndTrim(raw, sep string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, sep)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func stringPtr(s string) *string {
	return &s
}
