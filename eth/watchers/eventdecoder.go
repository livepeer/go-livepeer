package watchers

import (
	"errors"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

// EventDecoder decodes logs into events for known contracts
type EventDecoder struct {
	addr             ethcommon.Address
	contract         *bind.BoundContract
	topicToEventName map[ethcommon.Hash]string
}

// NewEventDecoder returns a new instance of EventDecoder with a contract binding to the provided ABI string
func NewEventDecoder(addr ethcommon.Address, abiJSON string) (*EventDecoder, error) {
	abi, err := abi.JSON(strings.NewReader(abiJSON))
	if err != nil {
		return nil, err
	}

	topicToEventName := make(map[ethcommon.Hash]string)
	for _, event := range abi.Events {
		topicToEventName[event.ID] = event.Name
	}

	return &EventDecoder{
		addr: addr,
		// Create BoundContract without a backend because we just need to access
		// log unpacking without contract interaction
		contract:         bind.NewBoundContract(addr, abi, nil, nil, nil),
		topicToEventName: topicToEventName,
	}, nil
}

// FindEventName returns the event name for a log. An error will be returned if the log is not emitted
// from a known contract or if it does not map to a known event
func (e *EventDecoder) FindEventName(log types.Log) (string, error) {
	if log.Address != e.addr {
		return "", errors.New("log not from known contract")
	}
	eventName, ok := e.topicToEventName[log.Topics[0]]
	if !ok {
		return "", fmt.Errorf("unknown event for %v", e.addr.Hex())
	}

	return eventName, nil
}

// Decode decodes a log into an event struct. An error will be returned if the log is not emitted
// from a known contract or if it does not map to a known event
func (e *EventDecoder) Decode(eventName string, log types.Log, decodedLog interface{}) error {
	if log.Address != e.addr {
		return errors.New("log not from known contract")

	}
	return e.contract.UnpackLog(decodedLog, eventName, log)
}
