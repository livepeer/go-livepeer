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

// TimeWatcher is a type for a thread safe in-memory cache that watches for the following on-chain events:
//	* New rounds
//	* New blocks
//	* Transcoder (de)activation

// TimeWatcher allows for subscriptions to certain data feeds using a caller provided sink channel
// consumers of the TimeWatcher can subscribe to following data feeds:
// 	* Last Initialized Round Number
//	* Last Seen Block Number
type TimeWatcher struct {
	// state
	mu                       sync.RWMutex
	lastInitializedRound     *big.Int
	lastInitializedBlockHash [32]byte
	currentRoundStartBlock   *big.Int
	transcoderPoolSize       *big.Int
	lastSeenBlock            *big.Int

	// last initialized round number subscription feeds
	roundSubFeed  event.Feed
	roundSubScope event.SubscriptionScope
	// last seen block number subscription feeds
	blockSubFeed  event.Feed
	blockSubScope event.SubscriptionScope

	watcher BlockWatcher
	lpEth   eth.LivepeerEthClient
	dec     *EventDecoder

	quit chan struct{}
}

// NewTimeWatcher creates a new instance of TimeWatcher and sets the initial cache through an RPC call to an ethereum node
func NewTimeWatcher(roundsManagerAddr ethcommon.Address, watcher BlockWatcher, lpEth eth.LivepeerEthClient) (*TimeWatcher, error) {
	dec, err := NewEventDecoder(roundsManagerAddr, contracts.RoundsManagerABI)
	if err != nil {
		return nil, fmt.Errorf("error creating decoder: %v", err)
	}

	tw := &TimeWatcher{
		quit:    make(chan struct{}),
		watcher: watcher,
		lpEth:   lpEth,
		dec:     dec,
	}

	lr, err := tw.lpEth.LastInitializedRound()
	if err != nil {
		return nil, fmt.Errorf("error fetching initial lastInitializedRound value err=%q", err)
	}
	bh, err := tw.lpEth.BlockHashForRound(lr)
	if err != nil {
		return nil, fmt.Errorf("error fetching initial lastInitializedBlockHash value err=%q", err)
	}
	num, err := tw.lpEth.CurrentRoundStartBlock()
	if err != nil {
		return nil, fmt.Errorf("error fetching current round start block err=%q", err)
	}
	tw.setLastInitializedRound(lr, bh, num)

	lastSeenBlock, err := tw.watcher.GetLatestBlock()
	if err != nil {
		return nil, fmt.Errorf("error fetching last seen block err=%q", err)
	}
	blockNum := big.NewInt(0)
	if lastSeenBlock != nil {
		blockNum = lastSeenBlock.Number
	}
	tw.setLastSeenBlock(blockNum)

	size, err := tw.lpEth.GetTranscoderPoolSize()
	if err != nil {
		return nil, fmt.Errorf("error fetching initial transcoderPoolSize err=%q", err)
	}
	tw.setTranscoderPoolSize(size)

	return tw, nil
}

// LastInitializedRound gets the last initialized round from cache
func (tw *TimeWatcher) LastInitializedRound() *big.Int {
	tw.mu.RLock()
	defer tw.mu.RUnlock()
	return tw.lastInitializedRound
}

// LastInitializedBlockHash returns the blockhash of the block the last round was initiated in
func (tw *TimeWatcher) LastInitializedBlockHash() [32]byte {
	tw.mu.RLock()
	defer tw.mu.RUnlock()
	return tw.lastInitializedBlockHash
}

func (tw *TimeWatcher) CurrentRoundStartBlock() *big.Int {
	tw.mu.RLock()
	defer tw.mu.RUnlock()
	return tw.currentRoundStartBlock
}

func (tw *TimeWatcher) setLastInitializedRound(round *big.Int, hash [32]byte, startBlk *big.Int) {
	tw.mu.Lock()
	defer tw.mu.Unlock()
	tw.lastInitializedRound = round
	tw.lastInitializedBlockHash = hash
	tw.currentRoundStartBlock = startBlk
}

func (tw *TimeWatcher) GetTranscoderPoolSize() *big.Int {
	tw.mu.RLock()
	defer tw.mu.RUnlock()
	if tw.transcoderPoolSize == nil {
		return big.NewInt(0)
	}
	return tw.transcoderPoolSize
}

func (tw *TimeWatcher) setTranscoderPoolSize(size *big.Int) {
	tw.mu.Lock()
	defer tw.mu.Unlock()
	tw.transcoderPoolSize = size
}

func (tw *TimeWatcher) LastSeenBlock() *big.Int {
	tw.mu.RLock()
	defer tw.mu.RUnlock()
	return tw.lastSeenBlock
}

func (tw *TimeWatcher) setLastSeenBlock(blockNum *big.Int) {
	tw.mu.Lock()
	defer tw.mu.Unlock()
	tw.lastSeenBlock = blockNum
}

// Watch the blockwatch subscription for NewRound events
func (tw *TimeWatcher) Watch() error {
	events := make(chan []*blockwatch.Event, 10)
	sub := tw.watcher.Subscribe(events)
	defer sub.Unsubscribe()
	for {
		select {
		case <-tw.quit:
			return nil
		case err := <-sub.Err():
			glog.Error(err)
		case events := <-events:
			tw.handleBlockEvents(events)
		}
	}
}

// SubscribeRounds allows one to subscribe to new round events
// To unsubscribe, simply call `Unsubscribe` on the returned subscription.
// The sink channel should have ample buffer space to avoid blocking other subscribers.
// Slow subscribers are not dropped.
func (tw *TimeWatcher) SubscribeRounds(sink chan<- types.Log) event.Subscription {
	return tw.roundSubScope.Track(tw.roundSubFeed.Subscribe(sink))
}

// SubscribeBlocks allows one to subscribe to newly seen block numbers
// To unsubscribe, simply call `Unsubscribe` on the returned subscription.
// The sink channel should have ample buffer space to avoid blocking other subscribers.
// Slow subscribers are not dropped.
func (tw *TimeWatcher) SubscribeBlocks(sink chan<- *big.Int) event.Subscription {
	return tw.blockSubScope.Track(tw.blockSubFeed.Subscribe(sink))
}

// Stop TimeWatcher
func (tw *TimeWatcher) Stop() {
	close(tw.quit)
	tw.blockSubScope.Close()
	tw.roundSubScope.Close()
}

func (tw *TimeWatcher) handleBlockEvents(events []*blockwatch.Event) {
	for _, event := range events {
		tw.handleBlockNum(event)
		for _, log := range event.BlockHeader.Logs {
			if event.Type == blockwatch.Removed {
				log.Removed = true
			}
			if err := tw.handleLog(log); err != nil {
				glog.Error(err)
			}
		}
	}
}

func (tw *TimeWatcher) handleBlockNum(event *blockwatch.Event) {
	last := tw.LastSeenBlock()
	new := event.BlockHeader.Number
	if last == nil || last.Cmp(new) != 0 {
		tw.setLastSeenBlock(new)
		tw.blockSubFeed.Send(new)
	}
}

func (tw *TimeWatcher) handleLog(log types.Log) error {
	eventName, err := tw.dec.FindEventName(log)
	if err != nil {
		// Noop if we cannot find the event name
		return nil
	}

	if eventName != "NewRound" {
		return fmt.Errorf("eventName is not NewRound")
	}

	var nr contracts.RoundsManagerNewRound
	if err := tw.dec.Decode("NewRound", log, &nr); err != nil {
		return fmt.Errorf("unable to decode event: %v", err)
	}

	roundStartBlock, err := tw.lpEth.CurrentRoundStartBlock()
	if err != nil {
		return err
	}
	if log.Removed {
		lr, err := tw.lpEth.LastInitializedRound()
		if err != nil {
			return err
		}
		bh, err := tw.lpEth.BlockHashForRound(lr)
		if err != nil {
			return err
		}
		tw.setLastInitializedRound(lr, bh, roundStartBlock)
	} else {
		tw.setLastInitializedRound(nr.Round, nr.BlockHash, roundStartBlock)
	}

	// Get the active transcoder pool size when we receive a NewRound event
	size, err := tw.lpEth.GetTranscoderPoolSize()
	if err != nil {
		return err
	}
	tw.setTranscoderPoolSize(size)

	tw.roundSubFeed.Send(log)

	return nil
}
