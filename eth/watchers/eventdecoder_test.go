package watchers

import (
	"fmt"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/livepeer/go-livepeer/eth/contracts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTopicToEventName_AllEventsIncluded(t *testing.T) {
	abi := "[{\"anonymous\": false,\"inputs\": [],\"name\": \"First\",\"type\": \"event\"},{\"anonymous\": false,\"inputs\": [],\"name\": \"Second\",\"type\": \"event\"},{\"anonymous\": false,\"inputs\": [],\"name\": \"Third\",\"type\": \"event\"}]"
	assert := assert.New(t)

	addr := ethcommon.HexToAddress("0x692a70d2e424a56d2c6c27aa97d1a86395877b3a")
	dec, err := NewEventDecoder(addr, abi)
	assert.Nil(err)
	assert.Equal(dec.topicToEventName[crypto.Keccak256Hash([]byte("First()"))], "First")
	assert.Equal(dec.topicToEventName[crypto.Keccak256Hash([]byte("Second()"))], "Second")
	assert.Equal(dec.topicToEventName[crypto.Keccak256Hash([]byte("Third()"))], "Third")

}

func TestEventDecoder_FindEventType(t *testing.T) {
	dec, err := NewEventDecoder(stubBondingManagerAddr, contracts.BondingManagerABI)
	require.Nil(t, err)

	assert := assert.New(t)

	// Test unknown contract address
	log := newStubBaseLog()
	log.Address = common.BytesToAddress([]byte("foo"))
	_, err = dec.FindEventName(log)
	assert.EqualError(err, "log not from known contract")

	// Test unknown contract event
	log.Address = stubBondingManagerAddr
	log.Topics = []common.Hash{common.BytesToHash([]byte("foo"))}
	_, err = dec.FindEventName(log)
	assert.EqualError(err, fmt.Sprintf("unknown event for %v", stubBondingManagerAddr.Hex()))

	// Test known contract address
	log = newStubUnbondLog()
	eventName, err := dec.FindEventName(log)
	assert.Nil(err)
	assert.Equal("Unbond", eventName)
}

func TestEventDecoder_Decode(t *testing.T) {
	dec, err := NewEventDecoder(stubBondingManagerAddr, contracts.BondingManagerABI)
	require.Nil(t, err)

	assert := assert.New(t)

	// Test unknown contract address
	log := newStubBaseLog()
	log.Address = common.BytesToAddress([]byte("foo"))

	var dummyEvent struct {
		foo string
	}
	err = dec.Decode("foo", log, dummyEvent)
	assert.EqualError(err, "log not from known contract")

	// Test known contract address
	log = newStubUnbondLog()

	var unbondEvent contracts.BondingManagerUnbond
	err = dec.Decode("Unbond", log, &unbondEvent)
	assert.Nil(err)
}
