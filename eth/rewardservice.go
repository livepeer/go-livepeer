package eth

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/monitor"
)

var (
	ErrRewardServiceStarted = fmt.Errorf("reward service already started")
	ErrRewardServiceStopped = fmt.Errorf("reward service already stopped")

	DefaultRetryInterval  = 30 * time.Second // Avoid same-block retries
	DefaultMaxElapsedTime = 20 * time.Hour   // Avoid retries outside round
)

type RewardService struct {
	client            LivepeerEthClient
	working           bool
	cancelWorker      context.CancelFunc
	tw                timeWatcher
	mu                sync.Mutex
	cancelRetryWorker context.CancelFunc
	rewardRetryTimes  int
	retryInterval     time.Duration
	maxElapsedTime    time.Duration
	wg                sync.WaitGroup
}

func NewRewardService(client LivepeerEthClient, tw timeWatcher, rewardRetryTimes int) *RewardService {
	return &RewardService{
		client:           client,
		tw:               tw,
		rewardRetryTimes: rewardRetryTimes,
		retryInterval:    DefaultRetryInterval,
		maxElapsedTime:   DefaultMaxElapsedTime,
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
			s.stopPreviousWorker()

			// Start a new worker for the initialized round.
			currentRound := s.tw.LastInitializedRound()
			s.startNewWorker(cancelCtx, currentRound.Int64())
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

func (s *RewardService) startNewWorker(ctx context.Context, round int64) {
	retryCtx, cancelRetry := context.WithCancel(ctx)
	s.cancelRetryWorker = cancelRetry

	// Increment WaitGroup counter to track the retry worker.
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.callRewardWithRetries(retryCtx, round)
	}()
}

func (s *RewardService) stopPreviousWorker() {
	if s.cancelRetryWorker != nil {
		s.cancelRetryWorker()
		s.wg.Wait()
	}
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

func (s *RewardService) callRewardWithRetries(ctx context.Context, round int64) {
	expBackoff := backoff.NewExponentialBackOff()
	expBackoff.InitialInterval = s.retryInterval
	expBackoff.MaxInterval = 1 * time.Hour // Cap maximum interval between retries.
	expBackoff.MaxElapsedTime = s.maxElapsedTime

	backoffCtx := backoff.WithContext(expBackoff, ctx)

	retryCount := 0
	err := backoff.Retry(func() error {
		if retryCount >= s.rewardRetryTimes {
			return backoff.Permanent(fmt.Errorf("max retry attempts reached"))
		}

		// Validate the round before retrying.
		currentRound := s.tw.LastInitializedRound()
		if currentRound.Int64() != round {
			return backoff.Permanent(fmt.Errorf("round %v is no longer valid", round))
		}

		err := s.tryReward()
		if err == nil {
			return nil
		}

		retryCount++
		glog.Errorf("Error trying to call reward (attempt %d): %v", retryCount, err)
		return err // Retry on transient errors
	}, backoffCtx)

	if ctx.Err() != nil {
		glog.Infof("Retry context canceled, stopping retries")
		return
	}

	if err != nil {
		glog.Errorf("Failed to call reward after %d retries for round %v: %v", retryCount, round, err)
		if monitor.Enabled {
			monitor.RewardCallError(err.Error())
		}
	}
}
