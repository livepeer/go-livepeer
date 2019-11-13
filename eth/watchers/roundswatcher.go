package watchers

import (
	"fmt"
	"math/big"
	"sync"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
	"github.com/livepeer/go-livepeer/eth/contracts"

	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/eth"
	"github.com/livepeer/go-livepeer/eth/blockwatch"
)

// RoundsWatcher is a type for a thread safe in-memory cache that watches on-chain events for new initialized rounds
type RoundsWatcher struct {
	mu                       sync.RWMutex
	lastInitializedRound     *big.Int
	lastInitializedBlockHash [32]byte
	transcoderPoolSize       *big.Int

	quit chan struct{}

	subFeed  event.Feed
	subScope event.SubscriptionScope // Subscription scope tracking current live listeners

	watcher BlockWatcher
	lpEth   eth.LivepeerEthClient
	dec     *EventDecoder
}

// NewRoundsWatcher creates a new instance of RoundsWatcher and sets the initial cache through an RPC call to an ethereum node
func NewRoundsWatcher(roundsManagerAddr ethcommon.Address, watcher BlockWatcher, lpEth eth.LivepeerEthClient) (*RoundsWatcher, error) {
	dec, err := NewEventDecoder(roundsManagerAddr, contracts.RoundsManagerABI)
	if err != nil {
		return nil, fmt.Errorf("error creating decoder: %v", err)
	}

	return &RoundsWatcher{
		quit:    make(chan struct{}),
		watcher: watcher,
		lpEth:   lpEth,
		dec:     dec,
	}, nil
}

// LastInitializedRound gets the last initialized round from cache
func (rw *RoundsWatcher) LastInitializedRound() *big.Int {
	rw.mu.RLock()
	defer rw.mu.RUnlock()
	return rw.lastInitializedRound
}

// LastInitializedBlockHash returns the blockhash of the block the last round was initiated in
func (rw *RoundsWatcher) LastInitializedBlockHash() [32]byte {
	rw.mu.RLock()
	defer rw.mu.RUnlock()
	return rw.lastInitializedBlockHash
}

func (rw *RoundsWatcher) setLastInitializedRound(round *big.Int, hash [32]byte) {
	rw.mu.Lock()
	defer rw.mu.Unlock()
	rw.lastInitializedRound = round
	rw.lastInitializedBlockHash = hash
}

func (rw *RoundsWatcher) GetTranscoderPoolSize() *big.Int {
	rw.mu.RLock()
	defer rw.mu.RUnlock()
	return rw.transcoderPoolSize
}

func (rw *RoundsWatcher) setTranscoderPoolSize(size *big.Int) {
	rw.mu.Lock()
	defer rw.mu.Unlock()
	rw.transcoderPoolSize = size
}

// Watch the blockwatch subscription for NewRound events
func (rw *RoundsWatcher) Watch() error {
	lr, err := rw.lpEth.LastInitializedRound()
	if err != nil {
		return fmt.Errorf("error fetching initial lastInitializedRound value: %v", err)
	}
	bh, err := rw.lpEth.BlockHashForRound(lr)
	if err != nil {
		return fmt.Errorf("error fetching initial lastInitializedBlockHash value: %v", err)
	}
	rw.setLastInitializedRound(lr, bh)

	if err := rw.fetchAndSetTranscoderPoolSize(); err != nil {
		return fmt.Errorf("error fetching initial transcoderPoolSize: %v", err)
	}

	events := make(chan []*blockwatch.Event, 10)
	sub := rw.watcher.Subscribe(events)
	defer sub.Unsubscribe()
	for {
		select {
		case <-rw.quit:
			return nil
		case err := <-sub.Err():
			glog.Error(err)
		case events := <-events:
			rw.handleBlockEvents(events)
		}
	}
}

// Subscribe allows one to subscribe to the block events emitted by the Watcher.
// To unsubscribe, simply call `Unsubscribe` on the returned subscription.
// The sink channel should have ample buffer space to avoid blocking other subscribers.
// Slow subscribers are not dropped.
func (rw *RoundsWatcher) Subscribe(sink chan<- types.Log) event.Subscription {
	return rw.subScope.Track(rw.subFeed.Subscribe(sink))
}

// Stop watching for NewRound events
func (rw *RoundsWatcher) Stop() {
	close(rw.quit)
	rw.subScope.Close()
}

func (rw *RoundsWatcher) handleBlockEvents(events []*blockwatch.Event) {
	for _, event := range events {
		for _, log := range event.BlockHeader.Logs {
			if event.Type == blockwatch.Removed {
				log.Removed = true
			}
			if err := rw.handleLog(log); err != nil {
				glog.Error(err)
			}
		}
	}
}

func (rw *RoundsWatcher) handleLog(log types.Log) error {
	eventName, err := rw.dec.FindEventName(log)
	if err != nil {
		// Noop if we cannot find the event name
		return nil
	}

	if eventName != "NewRound" {
		return fmt.Errorf("eventName is not NewRound")
	}

	var nr contracts.RoundsManagerNewRound
	if err := rw.dec.Decode("NewRound", log, &nr); err != nil {
		return fmt.Errorf("unable to decode event: %v", err)
	}

	rw.subFeed.Send(log)

	if log.Removed {
		lr, err := rw.lpEth.LastInitializedRound()
		if err != nil {
			return err
		}
		bh, err := rw.lpEth.BlockHashForRound(lr)
		if err != nil {
			return err
		}
		rw.setLastInitializedRound(lr, bh)
	} else {
		rw.setLastInitializedRound(nr.Round, nr.BlockHash)
	}

	// Get the active transcoder pool size when we receive a NewRound event
	if err := rw.fetchAndSetTranscoderPoolSize(); err != nil {
		return err
	}

	return nil
}

func (rw *RoundsWatcher) fetchAndSetTranscoderPoolSize() error {
	size, err := rw.lpEth.GetTranscoderPoolSize()
	if err != nil {
		return fmt.Errorf("error fetching initial transcoderPoolSize: %v", err)
	}
	rw.setTranscoderPoolSize(size)
	return nil
}
