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

func TestRoundInitializer_CurrentEpochSeed(t *testing.T) {
	initializer := NewRoundInitializer(nil, nil)

	assert := assert.New(t)

	// Test epochNum = 0
	blkHash := [32]byte{123}

	epochSeed := initializer.currentEpochSeed(big.NewInt(5), big.NewInt(5), blkHash)
	// epochNum = (5 - 5) / 5 = 0
	// epochSeed = keccak256(blkHash | 0) = 53205358842179480591542570540016728811976439286094436690881169143335261643310
	expEpochSeed, _ := new(big.Int).SetString("53205358842179480591542570540016728811976439286094436690881169143335261643310", 10)
	assert.Equal(expEpochSeed, epochSeed)

	// Test epochNum > 0
	epochSeed = initializer.currentEpochSeed(big.NewInt(20), big.NewInt(5), blkHash)
	// epochNum = (20 - 5) / 5 = 3
	// epochSeed = keccak256(blkHash | 3) = 42541119854153860846042329644941941146216657514071318786342840580076059276721
	expEpochSeed.SetString("42541119854153860846042329644941941146216657514071318786342840580076059276721", 10)
	assert.Equal(expEpochSeed, epochSeed)

	// Test epochNum > 0 with some # of blocks into the epoch
	epochSeed = initializer.currentEpochSeed(big.NewInt(20), big.NewInt(4), blkHash)
	// epochNum = (20 - 4) / 5 = 3.2 -> 3
	assert.Equal(expEpochSeed, epochSeed)
}

func TestRoundInitializer_ShouldInitialize(t *testing.T) {
	client := &MockClient{}
	tw := &stubTimeWatcher{}
	initializer := NewRoundInitializer(client, tw)

	assert := assert.New(t)

	// Test error getting transcoders
	expErr := errors.New("TranscoderPool error")
	client.On("TranscoderPool").Return(nil, expErr).Once()

	ok, err := initializer.shouldInitialize(nil)
	assert.EqualError(err, expErr.Error())
	assert.False(ok)

	// Test active set is empty because no registered transcoders
	client.On("TranscoderPool").Return([]*lpTypes.Transcoder{}, nil).Once()
	ok, err = initializer.shouldInitialize(nil)
	assert.Nil(err)
	assert.False(ok)

	// Test that caller is not in active set because it is not registered
	caller := ethcommon.BytesToAddress([]byte("foo"))
	client.On("Account").Return(accounts.Account{Address: caller})

	registered := []*lpTypes.Transcoder{
		{Address: ethcommon.BytesToAddress([]byte("jar"))},
		{Address: ethcommon.BytesToAddress([]byte("bar"))},
	}
	client.On("TranscoderPool").Return(registered, nil).Once()

	ok, err = initializer.shouldInitialize(nil)
	assert.Nil(err)
	assert.False(ok)

	// Test not selected
	registered = append(registered, &lpTypes.Transcoder{Address: caller})
	client.On("TranscoderPool").Return(registered, nil)

	seed := big.NewInt(3)
	ok, err = initializer.shouldInitialize(seed)
	assert.Nil(err)
	assert.False(ok)

	// Test caller selected
	seed = big.NewInt(5)
	ok, err = initializer.shouldInitialize(seed)
	assert.Nil(err)
	assert.True(ok)
}

func TestRoundInitializer_TryInitialize(t *testing.T) {
	client := &MockClient{}
	tw := &stubTimeWatcher{
		lastBlock:                big.NewInt(5),
		lastInitializedRound:     big.NewInt(100),
		lastInitializedBlockHash: [32]byte{123},
	}
	initializer := NewRoundInitializer(client, tw)
	initializer.nextRoundStartL1Block = big.NewInt(5)
	assert := assert.New(t)

	// Test error checking should initialize
	expErr := errors.New("shouldInitialize error")
	client.On("TranscoderPool").Return(nil, expErr).Once()

	err := initializer.tryInitialize()
	assert.EqualError(err, expErr.Error())

	// Test should not initialize
	caller := ethcommon.BytesToAddress([]byte("foo"))
	client.On("Account").Return(accounts.Account{Address: caller})

	registered := []*lpTypes.Transcoder{
		{Address: ethcommon.BytesToAddress([]byte("jar"))},
	}
	client.On("TranscoderPool").Return(registered, nil).Once()

	err = initializer.tryInitialize()
	assert.Nil(err)

	// Test error when submitting initialization tx
	registered = []*lpTypes.Transcoder{{Address: caller}}
	client.On("TranscoderPool").Return(registered, nil)
	expErr = errors.New("InitializeRound error")
	client.On("InitializeRound").Return(nil, expErr).Once()

	err = initializer.tryInitialize()
	assert.EqualError(err, expErr.Error())

	// Test error checking initialization tx
	tx := &types.Transaction{}
	client.On("InitializeRound").Return(tx, nil)
	expErr = errors.New("CheckTx error")
	client.On("CheckTx", mock.Anything).Return(expErr).Once()

	err = initializer.tryInitialize()
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
