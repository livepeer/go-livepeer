package eth

import (
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/golang/glog"
)

// Number of blocks in an epoch which is the time period during which the caller should
// initialize the round if it is selected and if the round is not initialized
var epochBlocks = big.NewInt(5)

// BlockNumReader describes methods for reading the last seen block number
type BlockNumReader interface {
	LastSeenBlock() (*big.Int, error)
}

// BlockHashReader describes methods for reading the last initialized block hash
type BlockHashReader interface {
	LastInitializedBlockHash() [32]byte
}

// RoundInitializer is a service that automatically initializes the current round. Each round is split into epochs with a length of
// epochBlocks. During each epoch a member of the upcoming active set is selected to initialize the round
// This selection process is purely a client side implementation that attempts to minimize on-chain transaction collisions, but
// collisions are still possible if initialization transactions are submitted by parties that are not using this selection process
type RoundInitializer struct {
	client          LivepeerEthClient
	blkNumRdr       BlockNumReader
	blkHashRdr      BlockHashReader
	pollingInterval time.Duration

	quit chan struct{}
}

// NewRoundInitializer creates a RoundInitializer instance
func NewRoundInitializer(client LivepeerEthClient, blkNumRdr BlockNumReader, blkHashRdr BlockHashReader, pollingInterval time.Duration) *RoundInitializer {
	return &RoundInitializer{
		client:          client,
		blkNumRdr:       blkNumRdr,
		blkHashRdr:      blkHashRdr,
		pollingInterval: pollingInterval,
		quit:            make(chan struct{}),
	}
}

// Start kicks off a loop that checks if the round should be initialized
func (r *RoundInitializer) Start() {
	ticker := time.NewTicker(r.pollingInterval)

	for {
		select {
		case <-r.quit:
			ticker.Stop()
			return
		case <-ticker.C:
			if err := r.tryInitialize(); err != nil {
				glog.Errorf("error trying to initialize round: %v", err)
			}
		}
	}
}

// Stop signals the polling loop to exit gracefully
func (r *RoundInitializer) Stop() {
	close(r.quit)
}

func (r *RoundInitializer) tryInitialize() error {
	initialized, err := r.client.CurrentRoundInitialized()
	if err != nil {
		return err
	}

	// Noop if the current round is initialized
	if initialized {
		return nil
	}

	currentRoundStartBlk, err := r.client.CurrentRoundStartBlock()
	if err != nil {
		return err
	}

	lastInitializedBlockHash := r.blkHashRdr.LastInitializedBlockHash()
	epochSeed, err := r.currentEpochSeed(currentRoundStartBlk, lastInitializedBlockHash)
	if err != nil {
		return err
	}

	ok, err := r.shouldInitialize(epochSeed)
	if err != nil {
		return err
	}

	// Noop if the caller should not initialize the round
	if !ok {
		return nil
	}

	currentRound, err := r.client.CurrentRound()
	if err != nil {
		return err
	}

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

	numActive, err := r.client.GetTranscoderPoolMaxSize()
	if err != nil {
		return false, err
	}

	numTranscoders := big.NewInt(int64(len(transcoders)))
	// If the # of registered transcoders is less than the max active set size then set
	// the active set size to the # of registered transcoders
	if numActive.Cmp(numTranscoders) > 0 {
		numActive = numTranscoders
	}

	// Should not initialize if the upcoming active set is empty
	if numActive.Cmp(big.NewInt(0)) == 0 {
		return false, nil
	}

	// Find the caller's rank in the upcoming active set
	rank := int64(-1)
	maxRank := numActive.Int64()
	for i := int64(0); i < maxRank; i++ {
		if transcoders[i].Address == r.client.Account().Address {
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
func (r *RoundInitializer) currentEpochSeed(roundStartBlk *big.Int, lastInitializedBlkHash [32]byte) (*big.Int, error) {
	currentBlk, err := r.blkNumRdr.LastSeenBlock()
	if err != nil {
		return nil, err
	}

	epochNum := new(big.Int).Sub(currentBlk, roundStartBlk)
	epochNum.Div(epochNum, epochBlocks)

	// The seed for the current epoch is calculated as:
	// keccak256(lastInitializedBlkHash | epochNum)
	return crypto.Keccak256Hash(append(lastInitializedBlkHash[:], epochNum.Bytes()...)).Big(), nil
}
