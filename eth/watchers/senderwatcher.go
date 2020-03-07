package watchers

import (
	"fmt"
	"math/big"
	"sync"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/eth"
	"github.com/livepeer/go-livepeer/eth/blockwatch"
	"github.com/livepeer/go-livepeer/eth/contracts"
	"github.com/livepeer/go-livepeer/pm"
)

// SenderWatcher maintains a concurrency-safe map with SenderInfo
type SenderWatcher struct {
	senders        map[ethcommon.Address]*pm.SenderInfo
	claimedReserve map[ethcommon.Address]*big.Int // map representing how much a recipient has drawn from a sender's reserve
	mu             sync.RWMutex
	quit           chan struct{}
	watcher        BlockWatcher
	tw             timeWatcher
	lpEth          eth.LivepeerEthClient
	dec            *EventDecoder
}

// NewSenderWatcher initiates a new SenderWatcher
func NewSenderWatcher(ticketBrokerAddr ethcommon.Address, watcher BlockWatcher, lpEth eth.LivepeerEthClient, tw timeWatcher) (*SenderWatcher, error) {
	dec, err := NewEventDecoder(ticketBrokerAddr, contracts.TicketBrokerABI)
	if err != nil {
		return nil, err
	}

	return &SenderWatcher{
		quit:           make(chan struct{}),
		watcher:        watcher,
		tw:             tw,
		lpEth:          lpEth,
		senders:        make(map[ethcommon.Address]*pm.SenderInfo),
		claimedReserve: make(map[ethcommon.Address]*big.Int),
		dec:            dec,
	}, nil
}

// GetSenderInfo returns information about a sender's deposit and reserve
// if values for a sender are not cached an RPC call to a remote ethereum node will be made to initialize the cache
func (sw *SenderWatcher) GetSenderInfo(addr ethcommon.Address) (*pm.SenderInfo, error) {
	sw.mu.RLock()
	cache := sw.senders[addr]
	sw.mu.RUnlock()
	if cache == nil {
		info, err := sw.lpEth.GetSenderInfo(addr)
		if err != nil {
			return nil, fmt.Errorf("GetSenderInfo RPC call to remote node failed: %v", err)
		}
		sw.setSenderInfo(addr, info)
		return info, nil
	}
	return cache, nil
}

func (sw *SenderWatcher) setSenderInfo(addr ethcommon.Address, info *pm.SenderInfo) {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	sw.senders[addr] = info
}

// ClaimedReserve returns the amount claimed from a sender's reserve by the node operator
func (sw *SenderWatcher) ClaimedReserve(reserveHolder ethcommon.Address, claimant ethcommon.Address) (*big.Int, error) {
	sw.mu.RLock()
	claimed := sw.claimedReserve[reserveHolder]
	sw.mu.RUnlock()
	if claimed != nil {
		return claimed, nil
	}
	claimed, err := sw.lpEth.ClaimedReserve(reserveHolder, claimant)
	if err != nil {
		return nil, fmt.Errorf("ClaimedReserve RPC call to remote node failed: %v", err)
	}
	sw.mu.Lock()
	sw.claimedReserve[reserveHolder] = claimed
	sw.mu.Unlock()
	return claimed, nil
}

// Watch starts the event watching loop
func (sw *SenderWatcher) Watch() {
	roundEvents := make(chan types.Log, 10)
	roundSub := sw.tw.SubscribeRounds(roundEvents)
	defer roundSub.Unsubscribe()

	events := make(chan []*blockwatch.Event, 10)
	sub := sw.watcher.Subscribe(events)
	defer sub.Unsubscribe()

	for {
		select {
		case <-sw.quit:
			return
		case err := <-sub.Err():
			glog.Error(err)
		case events := <-events:
			sw.handleBlockEvents(events)
		case roundEvent := <-roundEvents:
			if err := sw.handleRoundEvent(roundEvent); err != nil {
				glog.Errorf("error handling new round event: %v", err)
			}
		}
	}
}

// Stop watching for events
func (sw *SenderWatcher) Stop() {
	close(sw.quit)
}

// Clear removes a key-value pair from the map
func (sw *SenderWatcher) Clear(addr ethcommon.Address) {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	if _, ok := sw.senders[addr]; ok {
		delete(sw.senders, addr)
	}
	if _, ok := sw.claimedReserve[addr]; ok {
		delete(sw.claimedReserve, addr)
	}
}

func (sw *SenderWatcher) handleBlockEvents(events []*blockwatch.Event) {
	for _, event := range events {
		for _, log := range event.BlockHeader.Logs {
			if event.Type == blockwatch.Removed {
				log.Removed = true
			}
			if err := sw.handleLog(log); err != nil {
				glog.Error(err)
			}
		}
	}
}

func (sw *SenderWatcher) handleLog(log types.Log) error {
	eventName, err := sw.dec.FindEventName(log)
	if err != nil {
		// Noop if we cannot find the event name
		return nil
	}

	sw.mu.Lock()
	defer sw.mu.Unlock()

	var sender ethcommon.Address
	switch eventName {
	case "DepositFunded":
		var depositFunded contracts.TicketBrokerDepositFunded
		if err := sw.dec.Decode("DepositFunded", log, &depositFunded); err != nil {
			return fmt.Errorf("failed to decode DepositFunded event: %v", err)
		}
		sender = depositFunded.Sender
		if info, ok := sw.senders[sender]; ok && !log.Removed {
			info.Deposit.Add(info.Deposit, depositFunded.Amount)
		}
	case "ReserveFunded":
		var reserveFunded contracts.TicketBrokerReserveFunded
		if err := sw.dec.Decode("ReserveFunded", log, &reserveFunded); err != nil {
			return fmt.Errorf("failed to decode ReserveFunded event: %v", err)
		}
		sender = reserveFunded.ReserveHolder
		if info, ok := sw.senders[sender]; ok && !log.Removed {
			info.Reserve.FundsRemaining.Add(info.Reserve.FundsRemaining, reserveFunded.Amount)
		}
	case "Withdrawal":
		var withdrawal contracts.TicketBrokerWithdrawal
		if err := sw.dec.Decode("Withdrawal", log, &withdrawal); err != nil {
			return fmt.Errorf("failed to decode Withdrawal event: %v", err)
		}
		sender = withdrawal.Sender
		if info, ok := sw.senders[sender]; ok && !log.Removed {
			info.Deposit = big.NewInt(0)
			info.Reserve.FundsRemaining = big.NewInt(0)
			info.Reserve.ClaimedInCurrentRound = big.NewInt(0)
			sw.claimedReserve[sender] = big.NewInt(0)
		}
	case "WinningTicketTransfer":
		var winningTicketTransfer contracts.TicketBrokerWinningTicketTransfer
		if err := sw.dec.Decode("WinningTicketTransfer", log, &winningTicketTransfer); err != nil {
			return fmt.Errorf("failed to decode WinningTicketTransfer event: %v", err)
		}
		amount := winningTicketTransfer.Amount
		sender = winningTicketTransfer.Sender

		if info, ok := sw.senders[sender]; ok && !log.Removed {
			// See if amount > deposit
			if info.Deposit.Cmp(amount) < 0 {
				// Draw from reserve
				diff := new(big.Int).Sub(amount, info.Deposit)
				info.Deposit = big.NewInt(0)
				// Substract the difference from the remaining reserve
				info.Reserve.FundsRemaining.Sub(info.Reserve.FundsRemaining, diff)
				// Add the difference to the amount claimed from the reserve in the current round
				info.Reserve.ClaimedInCurrentRound.Add(info.Reserve.ClaimedInCurrentRound, diff)
				// if ticket recipient is the node operator, add to amount the recipient has claimed from a reserve
				if sw.lpEth.Account().Address == winningTicketTransfer.Recipient {
					if claimed, ok := sw.claimedReserve[sender]; ok {
						claimed.Add(claimed, diff)
					} else {
						sw.claimedReserve[sender] = diff
					}
				}
			} else {
				// Draw from deposit
				info.Deposit.Sub(info.Deposit, amount)
			}
		}
	case "Unlock":
		// Set withdraw block
		var unlock contracts.TicketBrokerUnlock
		if err := sw.dec.Decode("Unlock", log, &unlock); err != nil {
			return fmt.Errorf("failed to decode Unlock event: %v", err)
		}
		sender = unlock.Sender
		if info, ok := sw.senders[sender]; ok && !log.Removed {
			info.WithdrawRound = unlock.EndRound
		}
	case "UnlockCancelled":
		// Unset withdrawRound
		var unlockCancelled contracts.TicketBrokerUnlockCancelled
		if err := sw.dec.Decode("UnlockCancelled", log, &unlockCancelled); err != nil {
			return fmt.Errorf("failed to decode UnlockCancelled event: %v", err)
		}
		sender = unlockCancelled.Sender
		if info, ok := sw.senders[sender]; ok && !log.Removed {
			info.WithdrawRound = big.NewInt(0)
		}
	default:
		return nil
	}

	if _, ok := sw.senders[sender]; ok && log.Removed {
		info, err := sw.lpEth.GetSenderInfo(sender)
		if err != nil {
			return fmt.Errorf("GetSenderInfo RPC call to remote node failed: %v", err)
		}
		sw.senders[sender] = info
	}

	return nil
}

func (sw *SenderWatcher) handleRoundEvent(log types.Log) error {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	for sender, info := range sw.senders {
		if log.Removed {
			i, err := sw.lpEth.GetSenderInfo(sender)
			if err != nil {
				return fmt.Errorf("GetSenderInfo RPC call to remote node failed: %v", err)
			}
			info = i
		} else {
			info.Reserve.ClaimedInCurrentRound = big.NewInt(0)
		}
	}

	for sender := range sw.claimedReserve {
		if log.Removed {
			c, err := sw.lpEth.ClaimedReserve(sender, sw.lpEth.Account().Address)
			if err != nil {
				return fmt.Errorf("ClaimedReserve RPC call to remote node failed: %v", err)
			}
			sw.claimedReserve[sender] = c
		} else {
			sw.claimedReserve[sender] = big.NewInt(0)
		}
	}

	return nil
}
