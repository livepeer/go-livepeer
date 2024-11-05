package monitor

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"time"

	"github.com/golang/glog"
	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl/plain"
)

const (
	KafkaBatchInterval  = 1 * time.Second
	KafkaRequestTimeout = 60 * time.Second
)

type KafkaProducer struct {
	writer *kafka.Writer
	topic  string
}

type GatewayEvent struct {
	ID        *string     `json:"id,omitempty"`
	Type      *string     `json:"type"`
	Timestamp *string     `json:"timestamp"`
	Gateway   *string     `json:"gateway,omitempty"`
	Data      interface{} `json:"data"`
}

var kafkaProducer *KafkaProducer

func InitKafkaProducer(bootstrapServers, user, password, topic string) error {
	producer, err := NewKafkaProducer(bootstrapServers, user, password, topic)
	if err != nil {
		return err
	}
	kafkaProducer = producer
	return nil
}

func NewKafkaProducer(bootstrapServers, user, password, topic string) (*KafkaProducer, error) {
	dialer := &kafka.Dialer{
		Timeout: KafkaRequestTimeout,
		SASLMechanism: plain.Mechanism{
			Username: user,
			Password: password,
		},
		DualStack: true,
		TLS: &tls.Config{
			MinVersion: tls.VersionTLS12,
		},
	}

	// Create a new Kafka writer
	writer := kafka.NewWriter(kafka.WriterConfig{
		Brokers:  []string{bootstrapServers},
		Topic:    topic,
		Balancer: kafka.CRC32Balancer{},
		Dialer:   dialer,
	})

	return &KafkaProducer{
		writer: writer,
		topic:  topic,
	}, nil
}

func (p *KafkaProducer) SendEvent(event GatewayEvent) {
	p.SendEvents([]GatewayEvent{event})
}

func (p *KafkaProducer) SendEvents(events []GatewayEvent) {
	var msgs []kafka.Message

	for _, e := range events {
		value, err := json.Marshal(e)
		if err != nil {
			glog.Errorf("error while marshalling gateway log to Kafka, err=%v", err)
			continue
		}
		msgs = append(msgs, kafka.Message{
			Key:   []byte(*e.ID),
			Value: value,
		})
	}

	// We retry sending messages to Kafka in case of a failure
	kafkaWriteRetries := 3
	var err error
	for i := 0; i < kafkaWriteRetries; i++ {
		err = p.writer.WriteMessages(context.Background(), msgs...)
		if err == nil {
			return
		} else {
			glog.Warningf("error while sending gateway log to Kafka, retrying, topic=%s, try=%d, err=%v", p.topic, i, err)
		}
	}
	glog.Errorf("error while sending gateway log to Kafka, the gateway logs are lost, err=%d", err)
}

func SendQueueEvent(eventType string, data interface{}) {
	if kafkaProducer == nil {
		return
	}

	randomID := uuid.New().String()
	timestampMs := time.Now().UnixMilli()

	event := GatewayEvent{
		ID:        stringPtr(randomID),
		Gateway:   stringPtr(""),
		Type:      &eventType,
		Timestamp: stringPtr(fmt.Sprint(timestampMs)),
		Data:      data,
	}

	kafkaProducer.SendEvent(event)
}

func stringPtr(s string) *string {
	return &s
}
