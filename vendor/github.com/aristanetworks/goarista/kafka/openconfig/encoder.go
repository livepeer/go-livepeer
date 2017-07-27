// Copyright (C) 2016  Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package openconfig

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/Shopify/sarama"
	"github.com/aristanetworks/glog"
	"github.com/aristanetworks/goarista/elasticsearch"
	"github.com/aristanetworks/goarista/kafka"
	"github.com/aristanetworks/goarista/openconfig"
	"github.com/golang/protobuf/proto"

	pb "github.com/openconfig/reference/rpc/openconfig"
)

// UnhandledMessageError is used for proto messages not matching the handled types
type UnhandledMessageError struct {
	message proto.Message
}

func (e UnhandledMessageError) Error() string {
	return fmt.Sprintf("Unexpected type %T in proto message: %#v", e.message, e.message)
}

// UnhandledSubscribeResponseError is used for subscribe responses not matching the handled types
type UnhandledSubscribeResponseError struct {
	response *pb.SubscribeResponse
}

func (e UnhandledSubscribeResponseError) Error() string {
	return fmt.Sprintf("Unexpected type %T in subscribe response: %#v", e.response, e.response)
}

type elasticsearchMessageEncoder struct {
	*kafka.BaseEncoder
	topic   string
	dataset string
	key     sarama.Encoder
}

// NewEncoder creates and returns a new elasticsearch MessageEncoder
func NewEncoder(topic string, key sarama.Encoder, dataset string) kafka.MessageEncoder {
	baseEncoder := kafka.NewBaseEncoder("elasticsearch")
	return &elasticsearchMessageEncoder{
		BaseEncoder: baseEncoder,
		topic:       topic,
		dataset:     dataset,
		key:         key,
	}
}

func (e *elasticsearchMessageEncoder) Encode(message proto.Message) ([]*sarama.ProducerMessage,
	error) {

	response, ok := message.(*pb.SubscribeResponse)
	if !ok {
		return nil, UnhandledMessageError{message: message}
	}
	update := response.GetUpdate()
	if update == nil {
		return nil, UnhandledSubscribeResponseError{response: response}
	}
	updateMap, err := openconfig.NotificationToMap(e.dataset, update,
		elasticsearch.EscapeFieldName)
	if err != nil {
		return nil, err
	}
	// Convert time to ms to make elasticsearch happy
	updateMap["timestamp"] = updateMap["timestamp"].(int64) / 1000000
	updateJSON, err := json.Marshal(updateMap)
	if err != nil {
		return nil, err
	}
	glog.V(9).Infof("kafka: %s", updateJSON)
	return []*sarama.ProducerMessage{
		&sarama.ProducerMessage{
			Topic:    e.topic,
			Key:      e.key,
			Value:    sarama.ByteEncoder(updateJSON),
			Metadata: kafka.Metadata{StartTime: time.Unix(0, update.Timestamp), NumMessages: 1},
		},
	}, nil

}
