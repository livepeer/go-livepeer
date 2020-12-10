package eventservices

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/eth"
	"github.com/livepeer/go-livepeer/eth/watchers"
)

var (
	ErrRewardServiceStarted = fmt.Errorf("reward service already started")
	ErrRewardServiceStopped = fmt.Errorf("reward service already stopped")
)

type RewardService struct {
	client       eth.LivepeerEthClient
	working      bool
	cancelWorker context.CancelFunc
	tw           roundsWatcher
}

type roundsWatcher interface {
	SubscribeRounds(sink chan<- types.Log) event.Subscription
	LastInitializedRound() *big.Int
}

func NewRewardService(client eth.LivepeerEthClient, tw *watchers.TimeWatcher) *RewardService {
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
				glog.Errorf("Round subscription error err=%v", err)
			}
		case <-rounds:
			err := s.tryReward()
			if err != nil {
				glog.Errorf("Error trying to call reward err=%v", err)
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

		err = s.client.CheckTx(tx)
		if err != nil {
			if err == context.DeadlineExceeded {
				glog.Infof("Reward tx did not confirm within defined time window - will try to replace pending tx once")
				// Previous attempt to call reward() still pending
				// Replace pending tx by bumping gas price
				tx, err = s.client.ReplaceTransaction(tx, "reward", nil)
				if err != nil {
					return err
				}
				if err := s.client.CheckTx(tx); err != nil {
					return err
				}
			} else {
				return err
			}
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
