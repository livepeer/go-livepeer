package eth

import (
	"math/big"
	"math/rand"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
	"github.com/golang/glog"
)

// Number of L1 blocks in an epoch which is the time period during which the caller should
// initialize the round if it is selected and if the round is not initialized
var epochL1Blocks = big.NewInt(5)

type timeWatcher interface {
	LastSeenL1Block() *big.Int
	LastInitializedRound() *big.Int
	LastInitializedL1BlockHash() [32]byte
	CurrentRoundStartL1Block() *big.Int
	SubscribeRounds(sink chan<- types.Log) event.Subscription
	SubscribeL1Blocks(sink chan<- *big.Int) event.Subscription
	GetTranscoderPoolSize() *big.Int
}

// RoundInitializer is a service that automatically initializes the current round. Each round is split into epochs with a length of
// epochL1Blocks. During each epoch a member of the upcoming active set is selected to initialize the round
// This selection process is purely a client side implementation that attempts to minimize on-chain transaction collisions, but
// collisions are still possible if initialization transactions are submitted by parties that are not using this selection process
type RoundInitializer struct {
	maxDelay time.Duration
	client   LivepeerEthClient
	tw       timeWatcher
	quit     chan struct{}

	nextRoundStartL1Block *big.Int
	mu                    sync.Mutex
}

// NewRoundInitializer creates a RoundInitializer instance
func NewRoundInitializer(client LivepeerEthClient, tw timeWatcher, maxDelay time.Duration) *RoundInitializer {
	return &RoundInitializer{
		maxDelay: maxDelay,
		client:   client,
		tw:       tw,
		quit:     make(chan struct{}),
	}
}

// Start kicks off a loop that checks if the round should be initialized
func (r *RoundInitializer) Start() error {
	l1BlockSink := make(chan *big.Int, 10)
	l1BlockSub := r.tw.SubscribeL1Blocks(l1BlockSink)
	defer l1BlockSub.Unsubscribe()

	roundSink := make(chan types.Log, 10)
	roundSub := r.tw.SubscribeRounds(roundSink)
	defer roundSub.Unsubscribe()

	roundLength, err := r.client.RoundLength()
	if err != nil {
		return err
	}

	currentRoundStartL1Block, err := r.client.CurrentRoundStartBlock()
	if err != nil {
		return err
	}

	r.nextRoundStartL1Block = new(big.Int).Add(currentRoundStartL1Block, roundLength)

	for {
		select {
		case <-r.quit:
			glog.Infof("Stopping round initializer")
			return nil
		case err := <-l1BlockSub.Err():
			if err != nil {
				glog.Errorf("L1 Block subscription error err=%q", err)
			}
		case err := <-roundSub.Err():
			if err != nil {
				glog.Errorf("Round subscription error err=%q", err)
			}
		case <-roundSink:
			r.nextRoundStartL1Block = r.nextRoundStartL1Block.Add(r.tw.CurrentRoundStartL1Block(), roundLength)
		case l1Block := <-l1BlockSink:
			if l1Block.Cmp(r.nextRoundStartL1Block) >= 0 {
				go func() {
					if err := r.tryInitialize(); err != nil {
						glog.Error(err)
					}
				}()
			}
		}
	}
}

// Stop signals the polling loop to exit gracefully
func (r *RoundInitializer) Stop() {
	close(r.quit)
}

func (r *RoundInitializer) tryInitialize() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.tw.LastSeenL1Block().Cmp(r.nextRoundStartL1Block) < 0 {
		// Round already initialized
		return nil
	}

	if r.maxDelay > 0 {
		randDelay := time.Duration(rand.Int63n(int64(r.maxDelay)))
		glog.Infof("Waiting %v before attempting to initialize round", randDelay)
		time.Sleep(randDelay)

		if r.tw.LastSeenL1Block().Cmp(r.nextRoundStartL1Block) < 0 {
			glog.Infof("Round is already initialized, not initializing")
			return nil
		}
	}

	currentRound := new(big.Int).Add(r.tw.LastInitializedRound(), big.NewInt(1))
	glog.Infof("New round - preparing to initialize round to join active set, current round is %d", currentRound)

	tx, err := r.client.InitializeRound()
	if err != nil {
		return err
	}

	if err := r.client.CheckTx(tx); err != nil {
		return err
	}

	glog.Infof("Initialized round %d", currentRound)

	return nil
}
