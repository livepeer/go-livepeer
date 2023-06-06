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
//   - Last Initialized Round Number
//   - Last Seen Block Number
type TimeWatcher struct {
	// state
	mu                            sync.RWMutex
	lastInitializedRound          *big.Int
	lastInitializedL1BlockHash    [32]byte
	preLastInitializedL1BlockHash [32]byte
	currentRoundStartL1Block      *big.Int
	transcoderPoolSize            *big.Int
	lastSeenL1Block               *big.Int

	// last initialized round number subscription feeds
	roundSubFeed  event.Feed
	roundSubScope event.SubscriptionScope
	// last seen block number subscription feeds
	l1BlockSubFeed  event.Feed
	l1BlockSubScope event.SubscriptionScope
	feedMu          sync.Mutex

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
		return nil, fmt.Errorf("error fetching initial lastInitializedL1BlockHash value err=%q", err)
	}
	pbh, err := tw.lpEth.BlockHashForRound(prevRound(lr))
	if err != nil {
		return nil, fmt.Errorf("error fetching initial preLastInitializedL1BlockHash value err=%q", err)
	}
	num, err := tw.lpEth.CurrentRoundStartBlock()
	if err != nil {
		return nil, fmt.Errorf("error fetching current round start block err=%q", err)
	}
	tw.setLastInitializedRound(lr, bh, pbh, num)

	lastSeenBlock, err := tw.watcher.GetLatestBlock()
	if err != nil {
		return nil, fmt.Errorf("error fetching last seen block err=%q", err)
	}
	l1BlockNum := big.NewInt(0)
	if lastSeenBlock != nil {
		l1BlockNum = lastSeenBlock.L1BlockNumber
	}
	tw.setLastSeenL1Block(l1BlockNum)

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

// LastInitializedL1BlockHash returns the blockhash of the L1 block the last round was initiated in
func (tw *TimeWatcher) LastInitializedL1BlockHash() [32]byte {
	tw.mu.RLock()
	defer tw.mu.RUnlock()
	return tw.lastInitializedL1BlockHash
}

// PreLastInitializedL1BlockHash returns the blockhash of the L1 block for the round proceeding the last initialized round
func (tw *TimeWatcher) PreLastInitializedL1BlockHash() [32]byte {
	tw.mu.RLock()
	defer tw.mu.RUnlock()
	return tw.preLastInitializedL1BlockHash
}

func (tw *TimeWatcher) CurrentRoundStartL1Block() *big.Int {
	tw.mu.RLock()
	defer tw.mu.RUnlock()
	return tw.currentRoundStartL1Block
}

func (tw *TimeWatcher) setLastInitializedRound(round *big.Int, hash [32]byte, preHash [32]byte, startBlk *big.Int) {
	tw.mu.Lock()
	defer tw.mu.Unlock()
	tw.lastInitializedRound = round
	tw.lastInitializedL1BlockHash = hash
	tw.preLastInitializedL1BlockHash = preHash
	tw.currentRoundStartL1Block = startBlk
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

func (tw *TimeWatcher) LastSeenL1Block() *big.Int {
	tw.mu.RLock()
	defer tw.mu.RUnlock()
	return tw.lastSeenL1Block
}

func (tw *TimeWatcher) setLastSeenL1Block(blockNum *big.Int) {
	tw.mu.Lock()
	defer tw.mu.Unlock()
	tw.lastSeenL1Block = blockNum
}

// Watch the blockwatch subscription for NewRound events
func (tw *TimeWatcher) Watch() error {
	blockSink := make(chan []*blockwatch.Event, 10)
	sub := tw.watcher.Subscribe(blockSink)
	defer sub.Unsubscribe()
	for {
		select {
		case <-tw.quit:
			return nil
		case err := <-sub.Err():
			glog.Error(err)
		case block := <-blockSink:
			go tw.handleBlockEvents(block)
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

// SubscribeL1Blocks allows one to subscribe to newly seen L1 block numbers
// To unsubscribe, simply call `Unsubscribe` on the returned subscription.
// The sink channel should have ample buffer space to avoid blocking other subscribers.
// Slow subscribers are not dropped.
func (tw *TimeWatcher) SubscribeL1Blocks(sink chan<- *big.Int) event.Subscription {
	return tw.l1BlockSubScope.Track(tw.l1BlockSubFeed.Subscribe(sink))
}

// Stop TimeWatcher
func (tw *TimeWatcher) Stop() {
	close(tw.quit)
	tw.l1BlockSubScope.Close()
	tw.roundSubScope.Close()
}

func (tw *TimeWatcher) handleBlockEvents(events []*blockwatch.Event) {
	for _, event := range events {
		tw.handleL1BlockNum(event)
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

func (tw *TimeWatcher) handleL1BlockNum(event *blockwatch.Event) {
	tw.feedMu.Lock()
	defer tw.feedMu.Unlock()

	last := tw.LastSeenL1Block()
	new := event.BlockHeader.L1BlockNumber
	if new != nil && (last == nil || last.Cmp(new) != 0) {
		tw.setLastSeenL1Block(new)
		tw.l1BlockSubFeed.Send(new)
	}
}

func (tw *TimeWatcher) handleLog(log types.Log) error {
	eventName, err := tw.dec.FindEventName(log)
	if err != nil {
		// Noop if we cannot find the event name
		return nil
	}

	tw.feedMu.Lock()
	defer tw.feedMu.Unlock()

	if eventName != "NewRound" {
		return fmt.Errorf("eventName is not NewRound")
	}

	var nr contracts.RoundsManagerNewRound
	if err := tw.dec.Decode("NewRound", log, &nr); err != nil {
		return fmt.Errorf("unable to decode event: %v", err)
	}

	return tw.handleDecodedLog(log, nr)
}

func (tw *TimeWatcher) handleDecodedLog(log types.Log, nr contracts.RoundsManagerNewRound) error {
	roundStartL1Block, err := tw.lpEth.CurrentRoundStartBlock()
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
		pbh, err := tw.lpEth.BlockHashForRound(prevRound(lr))
		if err != nil {
			return err
		}
		tw.setLastInitializedRound(lr, bh, pbh, roundStartL1Block)
	} else {
		var pbh [32]byte
		if nr.Round.Cmp(big.NewInt(0).Add(tw.lastInitializedRound, big.NewInt(1))) == 0 {
			pbh = tw.lastInitializedL1BlockHash
		} else {
			pbh, err = tw.lpEth.BlockHashForRound(prevRound(nr.Round))
			if err != nil {
				return err
			}
		}
		tw.setLastInitializedRound(nr.Round, nr.BlockHash, pbh, roundStartL1Block)
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

func prevRound(lr *big.Int) *big.Int {
	return big.NewInt(0).Sub(lr, big.NewInt(1))
}
