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
	KafkaBatchSize      = 100
	KafkaChannelSize    = 100
)

type KafkaProducer struct {
	writer         *kafka.Writer
	topic          string
	events         chan GatewayEvent
	gatewayAddress string
}

type GatewayEvent struct {
	ID        *string     `json:"id,omitempty"`
	Type      *string     `json:"type"`
	Timestamp *string     `json:"timestamp"`
	Gateway   *string     `json:"gateway,omitempty"`
	Data      interface{} `json:"data"`
}

var kafkaProducer *KafkaProducer

func InitKafkaProducer(bootstrapServers, user, password, topic, gatewayAddress string) error {
	producer, err := newKafkaProducer(bootstrapServers, user, password, topic, gatewayAddress)
	if err != nil {
		return err
	}
	kafkaProducer = producer
	go producer.processEvents()
	return nil
}

func newKafkaProducer(bootstrapServers, user, password, topic, gatewayAddress string) (*KafkaProducer, error) {
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

	writer := kafka.NewWriter(kafka.WriterConfig{
		Brokers:  []string{bootstrapServers},
		Topic:    topic,
		Balancer: kafka.CRC32Balancer{},
		Dialer:   dialer,
	})

	return &KafkaProducer{
		writer:         writer,
		topic:          topic,
		events:         make(chan GatewayEvent, KafkaChannelSize),
		gatewayAddress: gatewayAddress,
	}, nil
}

func (p *KafkaProducer) processEvents() {
	ticker := time.NewTicker(KafkaBatchInterval)
	defer ticker.Stop()

	var eventsBatch []kafka.Message

	for {
		select {
		case event := <-p.events:
			value, err := json.Marshal(event)
			if err != nil {
				glog.Errorf("error while marshalling gateway log to Kafka, err=%v", err)
				continue
			}

			msg := kafka.Message{
				Key:   []byte(*event.ID),
				Value: value,
			}
			eventsBatch = append(eventsBatch, msg)

			// Send batch if it reaches the defined size
			if len(eventsBatch) >= KafkaBatchSize {
				p.sendBatch(eventsBatch)
				eventsBatch = nil
			}

		case <-ticker.C:
			if len(eventsBatch) > 0 {
				p.sendBatch(eventsBatch)
				eventsBatch = nil
			}
		}
	}
}

func (p *KafkaProducer) sendBatch(eventsBatch []kafka.Message) {
	// We retry sending messages to Kafka in case of a failure
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

func SendQueueEventAsync(eventType string, data interface{}) {
	if kafkaProducer == nil {
		return
	}

	randomID := uuid.New().String()
	timestampMs := time.Now().UnixMilli()

	event := GatewayEvent{
		ID:        stringPtr(randomID),
		Gateway:   stringPtr(kafkaProducer.gatewayAddress),
		Type:      &eventType,
		Timestamp: stringPtr(fmt.Sprint(timestampMs)),
		Data:      data,
	}

	select {
	case kafkaProducer.events <- event:
	default:
		glog.Warningf("kafka producer event queue is full, dropping event %q", eventType)
	}
}

func stringPtr(s string) *string {
	return &s
}
