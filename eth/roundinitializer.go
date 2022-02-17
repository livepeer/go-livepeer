package eth

import (
	"math/big"
	"sync"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
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
	client LivepeerEthClient
	tw     timeWatcher
	quit   chan struct{}

	nextRoundStartL1Block *big.Int
	mu                    sync.Mutex
}

// NewRoundInitializer creates a RoundInitializer instance
func NewRoundInitializer(client LivepeerEthClient, tw timeWatcher) *RoundInitializer {
	return &RoundInitializer{
		client: client,
		tw:     tw,
		quit:   make(chan struct{}),
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

	currentL1Blk := r.tw.LastSeenL1Block()
	lastInitializedL1BlkHash := r.tw.LastInitializedL1BlockHash()

	epochSeed := r.currentEpochSeed(currentL1Blk, r.nextRoundStartL1Block, lastInitializedL1BlkHash)

	ok, err := r.shouldInitialize(epochSeed)
	if err != nil {
		return err
	}

	// Noop if the caller should not initialize the round
	if !ok {
		return nil
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

func (r *RoundInitializer) shouldInitialize(epochSeed *big.Int) (bool, error) {
	transcoders, err := r.client.TranscoderPool()
	if err != nil {
		return false, err
	}

	numActive := big.NewInt(int64(len(transcoders)))

	// Should not initialize if the upcoming active set is empty
	if numActive.Cmp(big.NewInt(0)) == 0 {
		return false, nil
	}

	// Find the caller's rank in the upcoming active set
	rank := int64(-1)
	maxRank := numActive.Int64()
	caller := r.client.Account().Address
	for i := int64(0); i < maxRank; i++ {
		if transcoders[i].Address == caller {
			rank = i
			break
		}
	}

	// Should not initialize if the caller is not in the upcoming active set
	if rank == -1 {
		return false, nil
	}

	// Use the seed to select a position within the active set
	selection := new(big.Int).Mod(epochSeed, numActive)
	// Should not initialize if the selection does not match the caller's rank in the active set
	if selection.Int64() != int64(rank) {
		return false, nil
	}

	// If the selection matches the caller's rank the caller should initialize the round
	return true, nil
}

// Returns the seed used to select a round initializer in the current epoch for the current round
// This seed is not meant to be unpredictable. The only requirement for the seed is that it is calculated the same way for each
// party running the round initializer
func (r *RoundInitializer) currentEpochSeed(currentL1Block, roundStartL1Block *big.Int, lastInitializedL1BlkHash [32]byte) *big.Int {
	epochNum := new(big.Int).Sub(currentL1Block, roundStartL1Block)
	epochNum.Div(epochNum, epochL1Blocks)

	// The seed for the current epoch is calculated as:
	// keccak256(lastInitializedL1BlkHash | epochNum)
	return crypto.Keccak256Hash(append(lastInitializedL1BlkHash[:], epochNum.Bytes()...)).Big()
}
