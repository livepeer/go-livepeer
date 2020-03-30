package watchers

import (
	"errors"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/livepeer/go-livepeer/eth"
	"github.com/livepeer/go-livepeer/eth/blockwatch"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetAndGet_LastInitializedRound_LastInitializedBlockHash(t *testing.T) {
	assert := assert.New(t)
	tw := &TimeWatcher{}
	round := big.NewInt(5)
	var hash [32]byte
	copy(hash[:], "hello world")
	tw.setLastInitializedRound(round, hash)
	assert.Equal(tw.lastInitializedRound, round)
	assert.Equal(tw.lastInitializedBlockHash, hash)

	r := tw.LastInitializedRound()
	assert.Equal(r, round)
	h := tw.LastInitializedBlockHash()
	assert.Equal(h, hash)
}

func TestSetAndGet_TranscoderPoolSize(t *testing.T) {
	assert := assert.New(t)
	tw := &TimeWatcher{}
	size := big.NewInt(50)
	tw.setTranscoderPoolSize(size)
	assert.Equal(size, tw.transcoderPoolSize)
	assert.Equal(size, tw.GetTranscoderPoolSize())
}

func TestTimeWatcher_WatchAndStop(t *testing.T) {
	assert := assert.New(t)
	size := big.NewInt(50)
	block := big.NewInt(10)
	round := big.NewInt(1)
	hash := ethcommon.HexToHash("foo")
	lpEth := &eth.StubClient{
		PoolSize:          size,
		BlockNum:          block,
		BlockHashToReturn: hash,
		Round:             round,
	}
	watcher := &stubBlockWatcher{
		latestHeader: &blockwatch.MiniHeader{Number: block},
	}
	tw, err := NewTimeWatcher(stubRoundsManagerAddr, watcher, lpEth)
	assert.Nil(err)

	header := defaultMiniHeader()
	newRoundEvent := newStubNewRoundLog()

	header.Logs = append(header.Logs, newRoundEvent)
	blockEvent := &blockwatch.Event{
		Type:        blockwatch.Added,
		BlockHeader: header,
	}

	go tw.Watch()
	time.Sleep(2 * time.Millisecond)
	// Check state initialization
	bh := tw.LastInitializedBlockHash()
	assert.Equal(hash, common.BytesToHash(bh[:]))
	assert.Equal(round, tw.LastInitializedRound())
	assert.Equal(size, tw.GetTranscoderPoolSize())
	assert.Equal(block, tw.LastSeenBlock())
	tw.Stop()

	// if watcher.GetLatestBlock() == nil, initialise lastSeenBlock to big.NewInt(0)
	watcher.latestHeader = nil
	tw, err = NewTimeWatcher(stubRoundsManagerAddr, watcher, lpEth)
	assert.Nil(err)
	go tw.Watch()
	time.Sleep(2 * time.Millisecond)
	// Check state initialization
	bh = tw.LastInitializedBlockHash()
	assert.Equal(hash, common.BytesToHash(bh[:]))
	assert.Equal(round, tw.LastInitializedRound())
	assert.Equal(size, tw.GetTranscoderPoolSize())
	assert.Equal(big.NewInt(0), tw.LastSeenBlock())

	// New Round event
	watcher.sink <- []*blockwatch.Event{blockEvent}
	time.Sleep(2 * time.Millisecond)
	lastRound := tw.LastInitializedRound()
	assert.Zero(lastRound.Cmp(big.NewInt(8)))
	bhForRound := tw.LastInitializedBlockHash()
	var expectedHashForRound [32]byte
	copy(expectedHashForRound[:], newRoundEvent.Data[:])
	assert.Equal(bhForRound, expectedHashForRound)
	assert.Equal(size, tw.GetTranscoderPoolSize())
	assert.Equal(header.Number, tw.LastSeenBlock())

	// Test no NewRound events, values on rw remain the same
	tw.setTranscoderPoolSize(big.NewInt(10))
	blockEvent.BlockHeader.Logs = header.Logs[:1]
	watcher.sink <- []*blockwatch.Event{blockEvent}
	time.Sleep(2 * time.Millisecond)
	lastRound = tw.LastInitializedRound()
	assert.Zero(lastRound.Cmp(big.NewInt(8)))
	bhForRound = tw.LastInitializedBlockHash()
	copy(expectedHashForRound[:], newRoundEvent.Data[:])
	assert.Equal(bhForRound, expectedHashForRound)
	assert.Equal(big.NewInt(10), tw.GetTranscoderPoolSize())
	assert.Equal(header.Number, tw.LastSeenBlock())

	// Test RPC paths (event removed)
	blockEvent.BlockHeader.Logs = append(blockEvent.BlockHeader.Logs, newRoundEvent)
	blockEvent.Type = blockwatch.Removed
	watcher.sink <- []*blockwatch.Event{blockEvent}
	time.Sleep(2 * time.Millisecond)
	bhForRound = tw.LastInitializedBlockHash()
	assert.Equal(hash, common.BytesToHash(bh[:]))
	assert.Equal(round, tw.LastInitializedRound())
	assert.Equal(size, tw.GetTranscoderPoolSize())
	assert.Equal(header.Number, tw.LastSeenBlock())

	// Test Stop
	tw.Stop()
	time.Sleep(2 * time.Millisecond)
	assert.True(watcher.sub.unsubscribed)

	// Test watch error when RPC calls fail
	tw = &TimeWatcher{
		lpEth: &eth.StubClient{
			RoundsErr: errors.New("timewatcher error"),
		},
	}
	err = tw.Watch()
	assert.NotNil(err)
	assert.Contains(err.Error(), "timewatcher error")
}

func TestTimeWatcher_HandleLog(t *testing.T) {
	lpEth := &eth.StubClient{}
	watcher := &stubBlockWatcher{}
	tw, err := NewTimeWatcher(stubRoundsManagerAddr, watcher, lpEth)
	require.Nil(t, err)

	assert := assert.New(t)

	// Test unknown event
	log := newStubBaseLog()
	log.Topics = []ethcommon.Hash{ethcommon.BytesToHash([]byte("foo"))}

	err = tw.handleLog(log)
	assert.Nil(err)
	assert.Nil(tw.LastInitializedRound())
	assert.Equal([32]byte{}, tw.LastInitializedBlockHash())
}

func TestLastSeenBlock(t *testing.T) {
	assert := assert.New(t)
	tw := &TimeWatcher{}
	block := big.NewInt(5)

	tw.setLastSeenBlock(block)
	tw.LastSeenBlock()
	assert.Equal(big.NewInt(5), tw.LastSeenBlock())
}

func TestHandleBlockNum(t *testing.T) {
	assert := assert.New(t)
	watcher := &stubBlockWatcher{
		latestHeader: &blockwatch.MiniHeader{Number: big.NewInt(1)},
	}

	tw, err := NewTimeWatcher(stubRoundsManagerAddr, watcher, &eth.StubClient{})
	assert.Nil(err)
	header := defaultMiniHeader()
	header.Number = big.NewInt(10)
	blockEvent := &blockwatch.Event{
		Type:        blockwatch.Added,
		BlockHeader: header,
	}

	go tw.Watch()
	time.Sleep(2 * time.Millisecond)

	watcher.sink <- []*blockwatch.Event{blockEvent}
	time.Sleep(2 * time.Millisecond)
	assert.Equal(tw.LastSeenBlock(), header.Number)
}

func TestSubscribeBlocks(t *testing.T) {
	assert := assert.New(t)
	watcher := &stubBlockWatcher{
		latestHeader: &blockwatch.MiniHeader{Number: big.NewInt(1)},
	}

	tw, err := NewTimeWatcher(stubRoundsManagerAddr, watcher, &eth.StubClient{})
	assert.Nil(err)
	header := defaultMiniHeader()
	header.Number = big.NewInt(10)
	blockEvent := &blockwatch.Event{
		Type:        blockwatch.Added,
		BlockHeader: header,
	}

	events := make(chan *big.Int, 10)
	sub := tw.SubscribeBlocks(events)
	defer sub.Unsubscribe()

	go tw.Watch()
	time.Sleep(2 * time.Millisecond)

	watcher.sink <- []*blockwatch.Event{blockEvent}
	time.Sleep(2 * time.Millisecond)
	update := <-events
	assert.Equal(update, header.Number)
}

func TestSubscribeRounds(t *testing.T) {
	assert := assert.New(t)
	watcher := &stubBlockWatcher{
		latestHeader: &blockwatch.MiniHeader{Number: big.NewInt(1)},
	}

	tw, err := NewTimeWatcher(stubRoundsManagerAddr, watcher, &eth.StubClient{})
	assert.Nil(err)
	header := defaultMiniHeader()
	newRoundEvent := newStubNewRoundLog()

	header.Logs = append(header.Logs, newRoundEvent)
	blockEvent := &blockwatch.Event{
		Type:        blockwatch.Added,
		BlockHeader: header,
	}

	events := make(chan types.Log, 10)
	sub := tw.SubscribeRounds(events)
	defer sub.Unsubscribe()

	go tw.Watch()
	time.Sleep(2 * time.Millisecond)

	watcher.sink <- []*blockwatch.Event{blockEvent}
	time.Sleep(2 * time.Millisecond)
	update := <-events
	assert.Equal(newRoundEvent, update)
}
