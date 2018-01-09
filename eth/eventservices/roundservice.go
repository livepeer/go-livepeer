package eventservices

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/eth"
)

var (
	BlocksToWait = big.NewInt(5)

	ErrRoundsServiceStarted = fmt.Errorf("rounds service already started")
	ErrRoundsServiceStopped = fmt.Errorf("rounds service already stopped")
)

type RoundsService struct {
	eventMonitor eth.EventMonitor
	client       eth.LivepeerEthClient
	sub          ethereum.Subscription
	headersCh    chan *types.Header
}

func NewRoundsService(eventMonitor eth.EventMonitor, client eth.LivepeerEthClient) *RoundsService {
	return &RoundsService{
		eventMonitor: eventMonitor,
		client:       client,
	}
}

func (s *RoundsService) Start(ctx context.Context) error {
	if s.sub != nil {
		return ErrRoundsServiceStarted
	}

	headersCh := make(chan *types.Header)
	sub, err := s.eventMonitor.SubscribeNewBlock(ctx, headersCh, func(h *types.Header) (bool, error) {
		return s.tryInitializeRound(h.Number, h.Hash())
	})

	if err != nil {
		return err
	}

	s.headersCh = headersCh
	s.sub = sub

	return nil
}

func (s *RoundsService) Stop() error {
	if s.sub == nil {
		return ErrRoundsServiceStopped
	}

	close(s.headersCh)
	s.sub.Unsubscribe()

	s.headersCh = nil
	s.sub = nil

	return nil
}

func (s *RoundsService) tryInitializeRound(blkNum *big.Int, blkHash common.Hash) (bool, error) {
	currentRound, err := s.client.CurrentRound()
	if err != nil {
		return false, err
	}

	lastInitializedRound, err := s.client.LastInitializedRound()
	if err != nil {
		return false, err
	}

	if lastInitializedRound.Cmp(currentRound) == -1 {
		roundLength, err := s.client.RoundLength()
		if err != nil {
			return false, err
		}

		currentRoundStartBlock := new(big.Int).Mul(currentRound, roundLength)

		shouldInitialize, err := s.shouldInitializeRound(currentRoundStartBlock, blkNum, blkHash)
		if err != nil {
			return false, err
		}

		if shouldInitialize {
			glog.Infof("New round - preparing to initialize round to join active set")

			tx, err := s.client.InitializeRound()
			if err != nil {
				return false, err
			}

			err = s.client.CheckTx(tx)
			if err != nil {
				return false, err
			}

			glog.Infof("Initialized round %v", currentRound)
		}
	}

	return true, nil
}

func (s *RoundsService) shouldInitializeRound(currentRoundStartBlock *big.Int, blkNum *big.Int, blkHash common.Hash) (bool, error) {
	// Check to initialize round only in multiples of BlocksToWait blocks
	// to make sure a previous initializeRound call by someone else is processed
	blockDiff := new(big.Int).Sub(blkNum, currentRoundStartBlock)
	blkNumMod := new(big.Int).Mod(blockDiff, BlocksToWait)
	if blkNumMod.Cmp(big.NewInt(0)) != 0 {
		return false, nil
	}

	transcoders, err := s.client.RegisteredTranscoders()
	if err != nil {
		return false, err
	}

	numActive, err := s.client.NumActiveTranscoders()
	if err != nil {
		return false, err
	}

	numTranscoders := big.NewInt(int64(len(transcoders)))

	if numActive.Cmp(numTranscoders) == 1 {
		// If # of registered transcoders < the size of the active set # of active = # of registered
		numActive = numTranscoders
	}

	hashNum := new(big.Int).SetBytes(blkHash.Bytes())
	result := new(big.Int).Mod(hashNum, numActive)

	rank := -1
	for i := 0; i < int(numActive.Int64()); i++ {
		if transcoders[i].Address == s.client.Account().Address {
			rank = i
			break
		}
	}

	if rank == -1 {
		// Not in the upcoming active set
		return false, nil
	}

	// If blockHash % numActive == my rank, it is my turn to initialize the round
	// Else it is not my turn to initialize the round
	if result.Cmp(big.NewInt(int64(rank))) == 0 {
		return true, nil
	} else {
		return false, nil
	}
}
