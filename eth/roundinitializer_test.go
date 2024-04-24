package eth

import (
	"errors"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/accounts"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
	"github.com/golang/glog"
	lpTypes "github.com/livepeer/go-livepeer/eth/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestRoundInitializer_TryInitialize(t *testing.T) {
	client := &MockClient{}
	tw := &stubTimeWatcher{
		lastBlock:                big.NewInt(5),
		lastInitializedRound:     big.NewInt(100),
		lastInitializedBlockHash: [32]byte{123},
	}
	initializer := NewRoundInitializer(client, tw, 0)
	initializer.nextRoundStartL1Block = big.NewInt(5)
	assert := assert.New(t)

	// Test error checking initialization tx
	tx := &types.Transaction{}
	client.On("InitializeRound").Return(tx, nil)
	expErr := errors.New("CheckTx error")
	client.On("CheckTx", mock.Anything).Return(expErr).Once()

	err := initializer.tryInitialize()
	assert.EqualError(err, expErr.Error())

	// Test success
	client.On("CheckTx", mock.Anything).Return(nil)

	err = initializer.tryInitialize()
	assert.Nil(err)
}

func TestRoundInitializer_Start_Stop(t *testing.T) {
	assert := assert.New(t)
	tw := &stubTimeWatcher{}
	client := &MockClient{}
	quit := make(chan struct{})

	initializer := &RoundInitializer{
		client: client,
		tw:     tw,
		quit:   quit,
	}

	// RoundLength error
	expErr := errors.New("RoundLength error")
	client.On("RoundLength").Return(nil, expErr).Once()
	err := initializer.Start()
	assert.EqualError(err, expErr.Error())

	// CurrentRoundStartBlock error
	expErr = errors.New("CurrentRoundStartBlock error")
	client.On("RoundLength").Return(big.NewInt(100), nil)
	client.On("CurrentRoundStartBlock").Return(nil, expErr).Once()
	err = initializer.Start()
	assert.EqualError(err, expErr.Error())

	client.On("CurrentRoundStartBlock").Return(big.NewInt(5), nil)
	// test start and stop loop
	errC := make(chan error)
	go func() {
		errC <- initializer.Start()
	}()

	time.Sleep(1 * time.Second)
	initializer.Stop()
	err = <-errC
	assert.Nil(err)
	// should have set next round start block
	assert.Equal(initializer.nextRoundStartL1Block, big.NewInt(105)) // 100 + 5
}

func TestRoundInitializer_RoundSubscription(t *testing.T) {
	assert := assert.New(t)
	tw := &stubTimeWatcher{}
	client := &MockClient{}
	quit := make(chan struct{})

	initializer := &RoundInitializer{
		client: client,
		tw:     tw,
		quit:   quit,
	}

	roundLength := big.NewInt(5)

	client.On("RoundLength").Return(roundLength, nil)
	client.On("CurrentRoundStartBlock").Return(big.NewInt(5), nil)

	// test start and stop loop
	errC := make(chan error)
	go func() {
		errC <- initializer.Start()
	}()

	// Test set next round start block on initializer
	time.Sleep(1 * time.Second)
	assert.Equal(initializer.nextRoundStartL1Block, big.NewInt(10)) // 5 +5
	tw.currentRoundStartBlock = big.NewInt(100)
	tw.roundSink <- types.Log{}
	time.Sleep(1 * time.Second)

	initializer.Stop()
	err := <-errC
	assert.Nil(err)
	assert.Equal(initializer.nextRoundStartL1Block, new(big.Int).Add(roundLength, tw.currentRoundStartBlock))
}

func TestRoundInitializer_BlockSubscription(t *testing.T) {
	assert := assert.New(t)
	tw := &stubTimeWatcher{
		lastBlock:                big.NewInt(5),
		lastInitializedRound:     big.NewInt(100),
		lastInitializedBlockHash: [32]byte{123},
	}
	client := &MockClient{}
	quit := make(chan struct{})

	initializer := &RoundInitializer{
		client: client,
		tw:     tw,
		quit:   quit,
	}

	roundLength := big.NewInt(5)

	client.On("RoundLength").Return(roundLength, nil)
	client.On("CurrentRoundStartBlock").Return(big.NewInt(5), nil)

	// test start and stop loop
	errC := make(chan error)
	go func() {
		errC <- initializer.Start()
	}()
	time.Sleep(1 * time.Second)
	// block < next round start block do nothing
	tw.blockSink <- big.NewInt(5)
	time.Sleep(1 * time.Second)
	assert.Equal(initializer.nextRoundStartL1Block, big.NewInt(10))

	// block >= next round start block , try initialize
	caller := ethcommon.HexToAddress("foo")
	registered := []*lpTypes.Transcoder{{Address: caller}}
	client.On("Account").Return(accounts.Account{Address: caller})
	client.On("TranscoderPool").Return(registered, nil)
	client.On("InitializeRound").Return(nil, errors.New("some error")).Once()
	errLinesBefore := glog.Stats.Error.Lines()
	// tryInitialize error
	tw.lastBlock = big.NewInt(100)
	tw.blockSink <- tw.lastBlock
	time.Sleep(1 * time.Second)
	errLinesAfter := glog.Stats.Error.Lines()
	assert.Equal(int64(1), errLinesAfter-errLinesBefore)

	// try initialize success
	client.On("InitializeRound").Return(&types.Transaction{}, nil).Once()
	client.On("CheckTx").Return(nil)
	errLinesBefore = glog.Stats.Error.Lines()
	// tryInitialize error
	tw.blockSink <- big.NewInt(100)
	time.Sleep(1 * time.Second)
	errLinesAfter = glog.Stats.Error.Lines()
	assert.Equal(int64(0), errLinesAfter-errLinesBefore)
	client.AssertCalled(t, "InitializeRound")
	client.AssertCalled(t, "CheckTx")

	initializer.Stop()
	err := <-errC
	assert.Nil(err)
}

type stubTimeWatcher struct {
	lastBlock                *big.Int
	currentRoundStartBlock   *big.Int
	poolSize                 *big.Int
	lastInitializedBlockHash [32]byte
	lastInitializedRound     *big.Int

	roundSink chan<- types.Log
	roundSub  event.Subscription

	blockSink chan<- *big.Int
	blockSub  event.Subscription
}

func (m *stubTimeWatcher) LastSeenL1Block() *big.Int {
	return m.lastBlock
}

func (m *stubTimeWatcher) LastInitializedRound() *big.Int {
	return m.lastInitializedRound
}

func (m *stubTimeWatcher) LastInitializedL1BlockHash() [32]byte {
	return m.lastInitializedBlockHash
}

func (m *stubTimeWatcher) GetTranscoderPoolSize() *big.Int {
	return nil
}

func (m *stubTimeWatcher) CurrentRoundStartL1Block() *big.Int {
	return m.currentRoundStartBlock
}

func (m *stubTimeWatcher) SubscribeRounds(sink chan<- types.Log) event.Subscription {
	m.roundSink = sink
	m.roundSub = &stubSubscription{errCh: make(<-chan error)}
	return m.roundSub
}

func (m *stubTimeWatcher) SubscribeL1Blocks(sink chan<- *big.Int) event.Subscription {
	m.blockSink = sink
	m.blockSub = &stubSubscription{errCh: make(<-chan error)}
	return m.blockSub
}

type stubSubscription struct {
	errCh        <-chan error
	unsubscribed bool
}

func (s *stubSubscription) Unsubscribe() {
	s.unsubscribed = true
}

func (s *stubSubscription) Err() <-chan error {
	return s.errCh
}
