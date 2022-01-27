package watchers

import (
	"fmt"
	"math/big"
	"sync"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/eth/blockwatch"
	"github.com/livepeer/go-livepeer/eth/contracts"
)

type unbondingLockStore interface {
	InsertUnbondingLock(id *big.Int, delegator ethcommon.Address, amount, withdrawRound *big.Int) error
	DeleteUnbondingLock(id *big.Int, delegator ethcommon.Address) error
	UseUnbondingLock(id *big.Int, delegator ethcommon.Address, usedBlock *big.Int) error
}

// UnbondingWatcher watches for on-chain events to update the state of an unbonding lock store
type UnbondingWatcher struct {
	addr  ethcommon.Address // Watching for on-chain events pertaining to this address
	bw    BlockWatcher
	store unbondingLockStore
	dec   *EventDecoder

	quit chan struct{}

	mu sync.Mutex
}

// NewUnbondingWatcher creates an UnbondingWatcher instance
func NewUnbondingWatcher(addr ethcommon.Address, bondingManagerAddr ethcommon.Address, bw BlockWatcher, store unbondingLockStore) (*UnbondingWatcher, error) {
	dec, err := NewEventDecoder(bondingManagerAddr, contracts.BondingManagerABI)
	if err != nil {
		return nil, err
	}

	return &UnbondingWatcher{
		addr:  addr,
		bw:    bw,
		store: store,
		dec:   dec,
		quit:  make(chan struct{}),
	}, nil
}

// Watch kicks off a loop that handles events from a block subscription
func (w *UnbondingWatcher) Watch() {
	blockSink := make(chan []*blockwatch.Event, 10)
	sub := w.bw.Subscribe(blockSink)
	defer sub.Unsubscribe()

	for {
		select {
		case <-w.quit:
			return
		case err := <-sub.Err():
			glog.Errorf("error with block subscription: %v", err)
		case block := <-blockSink:
			go w.handleBlockEvents(block)
		}
	}
}

// Stop signals the watcher loop to exit gracefully
func (w *UnbondingWatcher) Stop() {
	close(w.quit)
}

func (w *UnbondingWatcher) handleBlockEvents(events []*blockwatch.Event) {
	for _, event := range events {
		for _, log := range event.BlockHeader.Logs {
			if event.Type == blockwatch.Removed {
				log.Removed = true
			}
			if err := w.handleLog(log); err != nil {
				glog.Error(err)
			}
		}
	}
}

func (w *UnbondingWatcher) handleLog(log types.Log) error {
	eventName, err := w.dec.FindEventName(log)
	if err != nil {
		// Noop if we cannot find the event name
		return nil
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	switch eventName {
	case "Unbond":
		var unbondEvent contracts.BondingManagerUnbond
		if err := w.dec.Decode("Unbond", log, &unbondEvent); err != nil {
			return fmt.Errorf("failed to decode Unbond event: %v", err)
		}

		// Skip event if it does not pertain to the configured address
		if unbondEvent.Delegator != w.addr {
			return nil
		}

		if log.Removed {
			if err := w.store.DeleteUnbondingLock(unbondEvent.UnbondingLockId, unbondEvent.Delegator); err != nil {
				return processEventError("Unbond", true, err)
			}
		} else {
			if err := w.store.InsertUnbondingLock(unbondEvent.UnbondingLockId, unbondEvent.Delegator, unbondEvent.Amount, unbondEvent.WithdrawRound); err != nil {
				return processEventError("Unbond", false, err)
			}
		}
	case "Rebond":
		var rebondEvent contracts.BondingManagerRebond
		if err := w.dec.Decode("Rebond", log, &rebondEvent); err != nil {
			return fmt.Errorf("failed to decode Rebond event: %v", err)
		}

		// Skip event if it does not pertain to the configured address
		if rebondEvent.Delegator != w.addr {
			return nil
		}

		var usedBlock *big.Int
		if !log.Removed {
			usedBlock = new(big.Int).SetUint64(log.BlockNumber)
		}

		if err := w.store.UseUnbondingLock(rebondEvent.UnbondingLockId, rebondEvent.Delegator, usedBlock); err != nil {
			return processEventError("Rebond", log.Removed, err)
		}
	case "WithdrawStake":
		var withdrawStakeEvent contracts.BondingManagerWithdrawStake
		if err := w.dec.Decode("WithdrawStake", log, &withdrawStakeEvent); err != nil {
			return fmt.Errorf("failed to decode WithdrawStake event: %v", err)
		}

		// Skip event if it does not pertain to the configured address
		if withdrawStakeEvent.Delegator != w.addr {
			return nil
		}

		var usedBlock *big.Int
		if !log.Removed {
			usedBlock = new(big.Int).SetUint64(log.BlockNumber)
		}

		if err := w.store.UseUnbondingLock(withdrawStakeEvent.UnbondingLockId, withdrawStakeEvent.Delegator, usedBlock); err != nil {
			return processEventError("WithdrawStake", log.Removed, err)
		}
	default:
		return nil
	}

	return nil
}

func processEventError(eventName string, logRemoved bool, err error) error {
	if logRemoved {
		return fmt.Errorf("error processing removed %v event: %v", eventName, err)
	}
	return fmt.Errorf("error processing added %v event: %v", eventName, err)
}
