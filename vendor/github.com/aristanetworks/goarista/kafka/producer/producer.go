// Copyright (C) 2016  Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package producer

import (
	"os"
	"sync"

	"github.com/Shopify/sarama"
	"github.com/aristanetworks/glog"
	"github.com/aristanetworks/goarista/kafka"
	"github.com/aristanetworks/goarista/kafka/openconfig"
	"github.com/golang/protobuf/proto"
)

// Producer forwards messages recvd on a channel to kafka.
type Producer interface {
	Start()
	Write(proto.Message)
	Stop()
}

type producer struct {
	notifsChan    chan proto.Message
	kafkaProducer sarama.AsyncProducer
	encoder       kafka.MessageEncoder
	done          chan struct{}
	wg            sync.WaitGroup
}

// New creates new Kafka producer
func New(encoder kafka.MessageEncoder,
	kafkaAddresses []string, kafkaConfig *sarama.Config) (Producer, error) {

	if kafkaConfig == nil {
		kafkaConfig := sarama.NewConfig()
		hostname, err := os.Hostname()
		if err != nil {
			hostname = ""
		}
		kafkaConfig.ClientID = hostname
		kafkaConfig.Producer.Compression = sarama.CompressionSnappy
		kafkaConfig.Producer.Return.Successes = true
		kafkaConfig.Producer.RequiredAcks = sarama.WaitForAll
	}

	kafkaProducer, err := sarama.NewAsyncProducer(kafkaAddresses, kafkaConfig)
	if err != nil {
		return nil, err
	}

	p := &producer{
		notifsChan:    make(chan proto.Message),
		kafkaProducer: kafkaProducer,
		encoder:       encoder,
		done:          make(chan struct{}),
		wg:            sync.WaitGroup{},
	}
	return p, nil
}

// Start makes producer to start processing writes.
// This method is non-blocking.
func (p *producer) Start() {
	p.wg.Add(3)
	go p.handleSuccesses()
	go p.handleErrors()
	go p.run()
}

func (p *producer) run() {
	defer p.wg.Done()
	for {
		select {
		case batch, open := <-p.notifsChan:
			if !open {
				return
			}
			err := p.produceNotifications(batch)
			if err != nil {
				if _, ok := err.(openconfig.UnhandledSubscribeResponseError); !ok {
					panic(err)
				}
			}
		case <-p.done:
			return
		}
	}
}

func (p *producer) Write(msg proto.Message) {
	select {
	case p.notifsChan <- msg:
	case <-p.done:
		// TODO: This should probably return an EOF error, but that
		// would change the API
	}
}

func (p *producer) Stop() {
	close(p.done)
	p.wg.Wait()
	p.kafkaProducer.Close()
}

func (p *producer) produceNotifications(protoMessage proto.Message) error {
	messages, err := p.encoder.Encode(protoMessage)
	if err != nil {
		return err
	}
	for _, m := range messages {
		select {
		case <-p.done:
			return nil
		case p.kafkaProducer.Input() <- m:
			glog.V(9).Infof("Message produced to Kafka: %s", m)
		}
	}
	return nil
}

// handleSuccesses reads from the producer's successes channel and collects some
// information for monitoring
func (p *producer) handleSuccesses() {
	defer p.wg.Done()
	for {
		select {
		case msg, open := <-p.kafkaProducer.Successes():
			if !open {
				return
			}
			p.encoder.HandleSuccess(msg)
		case <-p.done:
			return
		}
	}
}

// handleErrors reads from the producer's errors channel and collects some information
// for monitoring
func (p *producer) handleErrors() {
	defer p.wg.Done()
	for {
		select {
		case msg, open := <-p.kafkaProducer.Errors():
			if !open {
				return
			}
			p.encoder.HandleError(msg)
		case <-p.done:
			return
		}
	}
}
