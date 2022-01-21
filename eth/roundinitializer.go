package eth

import (
	"math/big"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/event"
	"github.com/golang/glog"
)

// Number of blocks in an epoch which is the time period during which the caller should
// initialize the round if it is selected and if the round is not initialized
var epochBlocks = big.NewInt(5)

type timeWatcher interface {
	LastSeenBlock() *big.Int
	LastInitializedRound() *big.Int
	LastInitializedBlockHash() [32]byte
	CurrentRoundStartBlock() *big.Int
	SubscribeRounds(sink chan<- types.Log) event.Subscription
	SubscribeBlocks(sink chan<- *big.Int) event.Subscription
	GetTranscoderPoolSize() *big.Int
}

// RoundInitializer is a service that automatically initializes the current round. Each round is split into epochs with a length of
// epochBlocks. During each epoch a member of the upcoming active set is selected to initialize the round
// This selection process is purely a client side implementation that attempts to minimize on-chain transaction collisions, but
// collisions are still possible if initialization transactions are submitted by parties that are not using this selection process
type RoundInitializer struct {
	client LivepeerEthClient
	tw     timeWatcher
	quit   chan struct{}

	nextRoundStartBlock *big.Int
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
	blockSink := make(chan *big.Int, 10)
	blockSub := r.tw.SubscribeBlocks(blockSink)
	defer blockSub.Unsubscribe()

	roundSink := make(chan types.Log, 10)
	roundSub := r.tw.SubscribeRounds(roundSink)
	defer roundSub.Unsubscribe()

	roundLength, err := r.client.RoundLength()
	if err != nil {
		return err
	}

	currentRoundStartBlock, err := r.client.CurrentRoundStartBlock()
	if err != nil {
		return err
	}

	r.nextRoundStartBlock = new(big.Int).Add(currentRoundStartBlock, roundLength)

	for {
		select {
		case <-r.quit:
			glog.Infof("Stopping round initializer")
			return nil
		case err := <-blockSub.Err():
			if err != nil {
				glog.Errorf("Block subscription error err=%q", err)
			}
		case err := <-roundSub.Err():
			if err != nil {
				glog.Errorf("Round subscription error err=%q", err)
			}
		case <-roundSink:
			r.nextRoundStartBlock = r.nextRoundStartBlock.Add(r.tw.CurrentRoundStartBlock(), roundLength)
		case block := <-blockSink:
			if block.Cmp(r.nextRoundStartBlock) >= 0 {
				if err := r.tryInitialize(); err != nil {
					glog.Error(err)
				}
			}
		}
	}
}

// Stop signals the polling loop to exit gracefully
func (r *RoundInitializer) Stop() {
	close(r.quit)
}

func (r *RoundInitializer) tryInitialize() error {
	currentBlk := r.tw.LastSeenBlock()
	lastInitializedBlkHash := r.tw.LastInitializedBlockHash()

	epochSeed := r.currentEpochSeed(currentBlk, r.nextRoundStartBlock, lastInitializedBlkHash)

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
func (r *RoundInitializer) currentEpochSeed(currentBlock, roundStartBlock *big.Int, lastInitializedBlkHash [32]byte) *big.Int {
	epochNum := new(big.Int).Sub(currentBlock, roundStartBlock)
	epochNum.Div(epochNum, epochBlocks)

	// The seed for the current epoch is calculated as:
	// keccak256(lastInitializedBlkHash | epochNum)
	return crypto.Keccak256Hash(append(lastInitializedBlkHash[:], epochNum.Bytes()...)).Big()
}
