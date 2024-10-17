package eth

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/monitor"
)

var (
	ErrRewardServiceStarted = fmt.Errorf("reward service already started")
	ErrRewardServiceStopped = fmt.Errorf("reward service already stopped")
)

type RewardService struct {
	client              LivepeerEthClient
	working             bool
	cancelWorker        context.CancelFunc
	tw                  timeWatcher
	mu                  sync.Mutex
	rewardRetryInterval time.Duration
	cancelRetryWorker   context.CancelFunc
}

func NewRewardService(client LivepeerEthClient, tw timeWatcher, rewardRetryInterval time.Duration) *RewardService {
	return &RewardService{
		client:              client,
		tw:                  tw,
		rewardRetryInterval: rewardRetryInterval,
	}
}

func (s *RewardService) Start(ctx context.Context) error {
	if s.working {
		return ErrRewardServiceStarted
	}

	cancelCtx, cancel := context.WithCancel(ctx)
	s.cancelWorker = cancel

	roundSink := make(chan types.Log, 10)
	sub := s.tw.SubscribeRounds(roundSink)
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
		case <-roundSink:
			s.cancelRetryWorker()

			retryCtx, cancelRetry := context.WithCancel(cancelCtx)
			s.cancelRetryWorker = cancelRetry

			go s.callRewardWithRetries(retryCtx)
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
	s.mu.Lock()
	defer s.mu.Unlock()

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

func (s *RewardService) callRewardWithRetries(ctx context.Context) {
	if s.rewardRetryInterval == 0 {
		err := s.tryReward()
		if err != nil {
			glog.Errorf("Error trying to call reward for round %v err=%q", s.tw.LastInitializedRound(), err)
			if monitor.Enabled {
				monitor.RewardCallError(err.Error())
			}
		}
		return
	}

	ticker := time.NewTicker(s.rewardRetryInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			err := s.tryReward()
			if err == nil {
				return
			}

			glog.Errorf("Error trying to call reward for round %v err=%q", s.tw.LastInitializedRound(), err)
			if monitor.Enabled {
				monitor.RewardCallError(err.Error())
			}
		case <-ctx.Done():
			return
		}
	}
}
