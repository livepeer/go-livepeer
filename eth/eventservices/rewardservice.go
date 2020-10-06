package eventservices

import (
	"context"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/eth"
)

var (
	ErrRewardServiceStarted = fmt.Errorf("reward service already started")
	ErrRewardServiceStopped = fmt.Errorf("reward service already stopped")
)

const blockTime = 15 * time.Second

type RewardService struct {
	client          eth.LivepeerEthClient
	pendingTx       *types.Transaction
	working         bool
	cancelWorker    context.CancelFunc
	pollingInterval time.Duration
}

func NewRewardService(client eth.LivepeerEthClient, pollingInterval time.Duration) *RewardService {
	return &RewardService{
		client:          client,
		pollingInterval: pollingInterval,
	}
}

func (s *RewardService) Start(ctx context.Context) error {
	if s.working {
		return ErrRewardServiceStarted
	}

	cancelCtx, cancel := context.WithCancel(ctx)
	s.cancelWorker = cancel

	tickCh := time.NewTicker(s.pollingInterval).C

	go func(ctx context.Context) {
		for {
			select {
			case <-tickCh:
				err := s.tryReward()
				if err != nil {
					glog.Errorf("Error trying to call reward: %v", err)
				}
			case <-ctx.Done():
				glog.V(5).Infof("Reward service done")
				return
			}
		}
	}(cancelCtx)

	s.working = true

	return nil
}

func (s *RewardService) Stop() error {
	if !s.working {
		return ErrRewardServiceStopped
	}

	s.cancelWorker()
	s.working = false

	return nil
}

func (s *RewardService) IsWorking() bool {
	return s.working
}

func (s *RewardService) tryReward() error {
	currentRound, err := s.client.CurrentRound()
	if err != nil {
		return err
	}

	initialized, err := s.client.CurrentRoundInitialized()
	if err != nil {
		return err
	}

	t, err := s.client.GetTranscoder(s.client.Account().Address)
	if err != nil {
		return err
	}

	active, err := s.client.IsActiveTranscoder()
	if err != nil {
		return err
	}

	if t.LastRewardRound.Cmp(currentRound) == -1 && initialized && active {
		var (
			tx  *types.Transaction
			err error
		)

		if s.pendingTx != nil {
			// Previous attempt to call reward() still pending
			// Replace pending tx by bumping gas price
			tx, err = s.client.ReplaceTransaction(s.pendingTx, "reward", nil)
			if err != nil {
				if err == eth.ErrReplacingMinedTx || err.Error() == "nonce too low" {
					// Pending tx confirmed so we should not try to replace next time
					s.pendingTx = nil
				}

				return err
			}
		} else {
			// No previous attempt to call reward(), invoke with next nonce
			tx, err = s.client.Reward()
			if err != nil {
				return err
			}
		}

		s.pendingTx = tx

		err = s.client.CheckTx(tx)
		if err != nil {
			if err == context.DeadlineExceeded {
				glog.Infof("Reward tx did not confirm within defined time window - will try to replace pending tx next time")
			}

			return err
		}

		s.pendingTx = nil

		tp, err := s.client.GetTranscoderEarningsPoolForRound(s.client.Account().Address, currentRound)
		if err != nil {
			return err
		}

		glog.Infof("Called reward for round %v - %v rewards minted", currentRound, eth.FormatUnits(tp.RewardPool, "LPTU"))

		return nil
	}

	return nil
}
