package eth

import (
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/golang/glog"
)

var (
	ErrRewardServiceStarted = fmt.Errorf("reward service already started")
	ErrRewardServiceStopped = fmt.Errorf("reward service already stopped")
)

type RewardService struct {
	client       LivepeerEthClient
	working      bool
	cancelWorker context.CancelFunc
	tw           timeWatcher
}

func NewRewardService(client LivepeerEthClient, tw timeWatcher) *RewardService {
	return &RewardService{
		client: client,
		tw:     tw,
	}
}

func (s *RewardService) Start(ctx context.Context) error {
	if s.working {
		return ErrRewardServiceStarted
	}

	cancelCtx, cancel := context.WithCancel(ctx)
	s.cancelWorker = cancel

	rounds := make(chan types.Log, 10)
	sub := s.tw.SubscribeRounds(rounds)
	defer sub.Unsubscribe()

	s.working = true
	defer func() {
		s.working = false
	}()

	for {
		select {
		case err := <-sub.Err():
			if err != nil {
				glog.Errorf("Round subscription error err=%q", err)
			}
		case <-rounds:
			err := s.tryReward()
			if err != nil {
				glog.Errorf("Error trying to call reward err=%q", err)
			}
		case <-cancelCtx.Done():
			glog.V(5).Infof("Reward service done")
			return nil
		}
	}
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
	currentRound := s.tw.LastInitializedRound()

	t, err := s.client.GetTranscoder(s.client.Account().Address)
	if err != nil {
		return err
	}

	if t.LastRewardRound.Cmp(currentRound) == -1 && t.Active {
		tx, err := s.client.Reward()
		if err != nil {
			return err
		}

		if err := s.client.CheckTx(tx); err != nil {
			return err
		}

		glog.Infof("Called reward for round %v", currentRound)

		return nil
	}

	return nil
}
