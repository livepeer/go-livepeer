package watchers

/*
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
)

type TranscoderPoolWatcher struct {
	poolSize *big.Int
	mu       sync.RWMutex
	quit     chan struct{}
	watcher  BlockWatcher
	lpEth    eth.LivepeerEthClient
	dec      *EventDecoder
}

func NewTranscoderPoolWatcher(bondingManagerAddr ethcommon.Address, watcher BlockWatcher, lpEth eth.LivepeerEthClient) (*TranscoderPoolWatcher, error) {
	dec, err := NewEventDecoder(bondingManagerAddr, contracts.BondingManagerABI)
	if err != nil {
		return nil, err
	}
	return &TranscoderPoolWatcher{
		quit:    make(chan struct{}),
		watcher: watcher,
		lpEth:   lpEth,
		dec:     dec,
	}, nil
}

func (w *TranscoderPoolWatcher) GetPoolSize() *big.Int {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.poolSize
}

func (w *TranscoderPoolWatcher) setPoolSize(size *big.Int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.poolSize = size
}

func (w *TranscoderPoolWatcher) Watch() error {
	// Fetch intial size through RPC call
	size, err := w.lpEth.GetTranscoderPoolSize()
	if err != nil {
		return fmt.Errorf("failed to fetch initial poolSize from remote ethereum node: %v", err)
	}
	w.setPoolSize(size)

	// Start watching for events
	events := make(chan []*blockwatch.Event, 10)
	sub := w.watcher.Subscribe(events)
	defer sub.Unsubscribe()
	for {
		select {
		case <-w.quit:
			return nil
		case err := <-sub.Err():
			glog.Error(err)
		case events := <-events:
			w.handleBlockEvents(events)
		}
	}
}

// Stop watching for events
func (w *TranscoderPoolWatcher) Stop() {
	close(w.quit)
}

func (w *TranscoderPoolWatcher) handleBlockEvents(events []*blockwatch.Event) {
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

func (w *TranscoderPoolWatcher) handleLog(log types.Log) error {
	eventName, err := sw.dec.FindEventName(log)
	if err != nil {
		// Noop if we cannot find the event name
		return nil
	}

	sw.mu.Lock()
	defer sw.mu.Unlock()

	switch eventName {
	case "TranscoderEvicted":
		// If log.Added sub from poolSize
		// if log.Removed add to poolSize
	case "TranscoderUpdated":

	}
}
*/
