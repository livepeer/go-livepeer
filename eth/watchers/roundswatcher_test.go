package watchers

import (
	"errors"
	"math/big"
	"testing"
	"time"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/livepeer/go-livepeer/eth"
	"github.com/livepeer/go-livepeer/eth/blockwatch"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetAndGet_LastInitializedRound_LastInitializedBlockHash(t *testing.T) {
	assert := assert.New(t)
	rw := &RoundsWatcher{}
	round := big.NewInt(5)
	var hash [32]byte
	copy(hash[:], "hello world")
	rw.setLastInitializedRound(round, hash)
	assert.Equal(rw.lastInitializedRound, round)
	assert.Equal(rw.lastInitializedBlockHash, hash)

	r := rw.LastInitializedRound()
	assert.Equal(r, round)
	h := rw.LastInitializedBlockHash()
	assert.Equal(h, hash)
}

func TestSetAndGet_TranscoderPoolSize(t *testing.T) {
	assert := assert.New(t)
	rw := &RoundsWatcher{}
	size := big.NewInt(50)
	rw.setTranscoderPoolSize(size)
	assert.Equal(size, rw.transcoderPoolSize)
	assert.Equal(size, rw.GetTranscoderPoolSize())
}

func TestRoundsWatcher_WatchAndStop(t *testing.T) {
	assert := assert.New(t)
	size := big.NewInt(50)
	lpEth := &eth.StubClient{
		PoolSize: size,
	}
	watcher := &stubBlockWatcher{}
	rw, err := NewRoundsWatcher(stubRoundsManagerAddr, watcher, lpEth)
	assert.Nil(err)

	header := defaultMiniHeader()
	newRoundEvent := newStubNewRoundLog()

	header.Logs = append(header.Logs, newRoundEvent)
	blockEvent := &blockwatch.Event{
		Type:        blockwatch.Added,
		BlockHeader: header,
	}

	go rw.Watch()
	time.Sleep(2 * time.Millisecond)

	// New Round event
	watcher.sink <- []*blockwatch.Event{blockEvent}
	time.Sleep(2 * time.Millisecond)
	lastRound := rw.LastInitializedRound()
	assert.Zero(lastRound.Cmp(big.NewInt(8)))
	bhForRound := rw.LastInitializedBlockHash()
	var expectedHashForRound [32]byte
	copy(expectedHashForRound[:], newRoundEvent.Data[:])
	assert.Equal(bhForRound, expectedHashForRound)
	assert.Equal(size, rw.GetTranscoderPoolSize())

	// Test no NewRound events, values on rw remain the same
	rw.setTranscoderPoolSize(big.NewInt(10))
	blockEvent.BlockHeader.Logs = header.Logs[:1]
	watcher.sink <- []*blockwatch.Event{blockEvent}
	time.Sleep(2 * time.Millisecond)
	lastRound = rw.LastInitializedRound()
	assert.Zero(lastRound.Cmp(big.NewInt(8)))
	bhForRound = rw.LastInitializedBlockHash()
	copy(expectedHashForRound[:], newRoundEvent.Data[:])
	assert.Equal(bhForRound, expectedHashForRound)
	assert.Equal(big.NewInt(10), rw.GetTranscoderPoolSize())

	// Test RPC paths (event removed)
	blockEvent.BlockHeader.Logs = append(blockEvent.BlockHeader.Logs, newRoundEvent)
	blockEvent.Type = blockwatch.Removed
	watcher.sink <- []*blockwatch.Event{blockEvent}
	time.Sleep(2 * time.Millisecond)
	lastRound = rw.LastInitializedRound()
	bhForRound = rw.LastInitializedBlockHash()
	assert.Equal(lastRound.Int64(), int64(0))
	assert.Equal(bhForRound, [32]byte{})
	assert.Equal(size, rw.GetTranscoderPoolSize())

	// Test Stop
	rw.Stop()
	time.Sleep(2 * time.Millisecond)
	assert.True(watcher.sub.unsubscribed)

	// Test watch error when RPC calls fail
	rw = &RoundsWatcher{
		lpEth: &eth.StubClient{
			RoundsErr: errors.New("roundswatcher error"),
		},
	}
	err = rw.Watch()
	assert.NotNil(err)
	assert.Contains(err.Error(), "roundswatcher error")
}

func TestRoundsWatcher_HandleLog(t *testing.T) {
	lpEth := &eth.StubClient{}
	watcher := &stubBlockWatcher{}
	rw, err := NewRoundsWatcher(stubRoundsManagerAddr, watcher, lpEth)
	require.Nil(t, err)

	assert := assert.New(t)

	// Test unknown event
	log := newStubBaseLog()
	log.Topics = []ethcommon.Hash{ethcommon.BytesToHash([]byte("foo"))}

	err = rw.handleLog(log)
	assert.Nil(err)
	assert.Nil(rw.LastInitializedRound())
	assert.Equal([32]byte{}, rw.LastInitializedBlockHash())
}
