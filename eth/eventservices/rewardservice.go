package eventservices

import (
	"context"
	"fmt"
	"time"

	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/eth"
)

var (
	ErrRewardServiceStarted = fmt.Errorf("reward service already started")
	ErrRewardServiceStopped = fmt.Errorf("reward service already stopped")

	TryRewardPollingInterval = time.Minute * 30 // Poll to try to call reward once 30 minutes
)

type RewardService struct {
	client       eth.LivepeerEthClient
	working      bool
	cancelWorker context.CancelFunc
}

func NewRewardService(client eth.LivepeerEthClient) *RewardService {
	return &RewardService{
		client: client,
	}
}

func (s *RewardService) Start(ctx context.Context) error {
	if s.working {
		return ErrRewardServiceStarted
	}

	cancelCtx, cancel := context.WithCancel(context.Background())
	s.cancelWorker = cancel

	tickCh := time.NewTicker(TryRewardPollingInterval).C

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

	t, err := s.client.GetTranscoder(s.client.Account().Address)
	if err != nil {
		return err
	}

	active, err := s.client.IsActiveTranscoder()
	if err != nil {
		return err
	}

	if t.LastRewardRound.Cmp(currentRound) == -1 && active {
		tx, err := s.client.Reward()
		if err != nil {
			return err
		}

		err = s.client.CheckTx(tx)
		if err != nil {
			return err
		}

		tp, err := s.client.GetTranscoderEarningsPoolForRound(s.client.Account().Address, currentRound)
		if err != nil {
			return err
		}

		glog.Infof("Called reward for round %v - %v rewards minted", currentRound, eth.FormatUnits(tp.RewardPool, "LPTU"))

		return nil
	}

	return nil
}
