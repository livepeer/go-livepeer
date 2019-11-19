package watchers

import (
	"errors"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/livepeer/go-livepeer/eth/blockwatch"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnbondingWatcherLoop(t *testing.T) {
	bw := &stubBlockWatcher{}
	store := newStubUnbondingLockStore()
	watcherAddr := common.HexToAddress("0xF75b78571F6563e8Acf1899F682Fb10A9248CCE8")
	watcher, err := NewUnbondingWatcher(watcherAddr, stubBondingManagerAddr, bw, store)
	require.Nil(t, err)

	assert := assert.New(t)

	go watcher.Watch()

	time.Sleep(2 * time.Millisecond)

	createBlockEvent := func(eventType blockwatch.EventType, blkNum uint64, logs []types.Log) *blockwatch.Event {
		for i := 0; i < len(logs); i++ {
			logs[i].BlockNumber = blkNum
		}

		return &blockwatch.Event{
			Type: eventType,
			BlockHeader: &blockwatch.MiniHeader{
				Logs: logs,
			},
		}
	}

	addedBlockEvents := make([]*blockwatch.Event, 2)
	addedBlockEvents[0] = createBlockEvent(blockwatch.Added, uint64(20), []types.Log{newStubUnbondLog(), newStubRebondLog()})
	addedBlockEvents[1] = createBlockEvent(blockwatch.Added, uint64(24), []types.Log{newStubWithdrawStakeLog()})

	bw.sink <- addedBlockEvents

	time.Sleep(2 * time.Millisecond)

	lock := store.Get(1)
	amount, _ := new(big.Int).SetString("11111000000000000000", 10)
	assert.Equal(amount, lock.Amount)
	assert.Equal(big.NewInt(1457), lock.WithdrawRound)
	assert.Equal(big.NewInt(24), lock.UsedBlock)

	removedBlockEvents := make([]*blockwatch.Event, 1)
	removedBlockEvents[0] = createBlockEvent(blockwatch.Removed, uint64(30), []types.Log{newStubUnbondLog()})

	bw.sink <- removedBlockEvents

	time.Sleep(2 * time.Millisecond)

	assert.Nil(store.Get(1))

	watcher.Stop()

	time.Sleep(2 * time.Millisecond)

	assert.True(bw.sub.unsubscribed)
}

func TestUnbondingWatcher_HandleLog(t *testing.T) {
	bw := &stubBlockWatcher{}
	store := newStubUnbondingLockStore()
	watcherAddr := common.HexToAddress("0xF75b78571F6563e8Acf1899F682Fb10A9248CCE8")
	watcher, err := NewUnbondingWatcher(watcherAddr, stubBondingManagerAddr, bw, store)
	require.Nil(t, err)

	assert := assert.New(t)

	// Test unknown event
	log := newStubBaseLog()
	log.Topics = []common.Hash{common.BytesToHash([]byte("foo"))}

	// Stub errors so we know that handleLog() returns early and does not
	// process any events - otherwise the method would return an error
	store.insertErr = errors.New("Insert error")
	store.deleteErr = errors.New("Delete error")
	store.useErr = errors.New("Use error")

	err = watcher.handleLog(log)
	assert.Nil(err)

	store.insertErr = nil
	store.deleteErr = nil
	store.useErr = nil

	// Test Unbond decode error
	log = newStubUnbondLog()
	log.Data = []byte("foo") // Corrupt log data

	err = watcher.handleLog(log)
	assert.Contains(err.Error(), "failed to decode Unbond event")

	// Test skip non-relevant Unbond
	log = newStubUnbondLog()
	log.Topics[2] = common.BytesToHash([]byte("non-relevant"))

	err = watcher.handleLog(log)
	assert.Nil(err)
	assert.Nil(store.Get(1))

	// Test error with added Unbond
	store.insertErr = errors.New("InsertUnbondingLock error")
	log = newStubUnbondLog()

	err = watcher.handleLog(log)
	assert.Contains(err.Error(), "error processing added Unbond event")

	// Test added Unbond
	store.insertErr = nil
	log = newStubUnbondLog()

	err = watcher.handleLog(log)
	assert.Nil(err)
	lock := store.Get(1)
	amount, _ := new(big.Int).SetString("11111000000000000000", 10)
	assert.Equal(lock.Amount, amount)
	assert.Equal(lock.WithdrawRound, big.NewInt(1457))

	// Test error with removed Unbond
	store.deleteErr = errors.New("DeleteUnbondingLock error")
	log = newStubUnbondLog()
	log.Removed = true

	err = watcher.handleLog(log)
	assert.Contains(err.Error(), "error processing removed Unbond event")

	// Test removed Unbond
	store.deleteErr = nil
	log = newStubUnbondLog()
	log.Removed = true

	err = watcher.handleLog(log)
	assert.Nil(err)
	assert.Nil(store.Get(1))

	// Add Unbond back
	log = newStubUnbondLog()

	err = watcher.handleLog(log)
	assert.Nil(err)

	// Test Rebond decode error
	log = newStubRebondLog()
	log.Data = []byte("foo") // Corrupt log data

	err = watcher.handleLog(log)
	assert.Contains(err.Error(), "failed to decode Rebond event")

	// Test skip non-relevant Rebond
	log = newStubRebondLog()
	log.Topics[2] = common.BytesToHash([]byte("non-relevant"))

	err = watcher.handleLog(log)
	assert.Nil(err)
	assert.Zero(store.Get(1).UsedBlock)

	// Test error with added Rebond
	store.useErr = errors.New("UseUnbondingLock error")
	log = newStubRebondLog()

	err = watcher.handleLog(log)
	assert.Contains(err.Error(), "error processing added Rebond event")

	// Test added Rebond
	store.useErr = nil
	log = newStubRebondLog()

	err = watcher.handleLog(log)
	assert.Nil(err)
	assert.Equal(big.NewInt(30), store.Get(1).UsedBlock)

	// Test error with removed Rebond
	store.useErr = errors.New("UseUnbondingLock error")
	log = newStubRebondLog()
	log.Removed = true

	err = watcher.handleLog(log)
	assert.Contains(err.Error(), "error processing removed Rebond event")

	// Test removed Rebond
	store.useErr = nil
	log = newStubRebondLog()
	log.Removed = true

	err = watcher.handleLog(log)
	assert.Nil(err)
	assert.Nil(store.Get(1).UsedBlock)

	// Test WithdrawStake decode error
	log = newStubWithdrawStakeLog()
	log.Data = []byte("foo") // Corrupt log data

	err = watcher.handleLog(log)
	assert.Contains(err.Error(), "failed to decode WithdrawStake event")

	// Test skip non-relevant WithdrawStake
	log = newStubWithdrawStakeLog()
	log.Topics[1] = common.BytesToHash([]byte("non-relevant"))

	err = watcher.handleLog(log)
	assert.Nil(err)
	assert.Nil(store.Get(1).UsedBlock)

	// Test error with added WithdrawStake
	store.useErr = errors.New("UseUnbondingLock error")
	log = newStubWithdrawStakeLog()

	err = watcher.handleLog(log)
	assert.Contains(err.Error(), "error processing added WithdrawStake event")

	// Test added WithdrawStake
	store.useErr = nil
	log = newStubWithdrawStakeLog()
	log.BlockNumber = uint64(99)

	err = watcher.handleLog(log)
	assert.Nil(err)
	assert.Equal(big.NewInt(99), store.Get(1).UsedBlock)

	// Test error with removed WithdrawStake
	store.useErr = errors.New("UseUnbondingLock error")
	log = newStubWithdrawStakeLog()
	log.Removed = true

	err = watcher.handleLog(log)
	assert.Contains(err.Error(), "error processing removed WithdrawStake event")

	// Test removed WithdrawStake
	store.useErr = nil
	log = newStubWithdrawStakeLog()
	log.Removed = true

	err = watcher.handleLog(log)
	assert.Nil(err)
	assert.Nil(store.Get(1).UsedBlock)

	// Test event that is not supported
	log = newStubUnbondLog()
	// Set first topic to topic hash for Reward()
	log.Topics = []common.Hash{common.HexToHash("0x619caafabdd75649b302ba8419e48cccf64f37f1983ac4727cfb38b57703ffc9")}

	err = watcher.handleLog(log)
	assert.Nil(err)
}
