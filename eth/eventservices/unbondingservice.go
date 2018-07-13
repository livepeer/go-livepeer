package eventservices

import (
	"context"
	"fmt"
	"math/big"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/eth"
	"github.com/livepeer/go-livepeer/eth/contracts"
)

var (
	ErrUnbondingServiceStarted = fmt.Errorf("unbonding service already started")
	ErrUnbondingServiceStopped = fmt.Errorf("unbonding service already stopped")
)

type UnbondingService struct {
	client                   eth.LivepeerEthClient
	db                       *common.DB
	working                  bool
	cancelWorker             context.CancelFunc
	unbondResubscribe        bool
	rebondResubscribe        bool
	withdrawStakeResubscribe bool
}

func NewUnbondingService(client eth.LivepeerEthClient, db *common.DB) *UnbondingService {
	return &UnbondingService{
		client:                   client,
		db:                       db,
		unbondResubscribe:        true,
		rebondResubscribe:        true,
		withdrawStakeResubscribe: true,
	}
}

func (s *UnbondingService) Start(ctx context.Context) error {
	if s.working {
		return ErrUnbondingServiceStarted
	}

	ctx, cancel := context.WithCancel(ctx)
	s.cancelWorker = cancel

	go func() {
		var (
			unbondSink        = make(chan *contracts.BondingManagerUnbond)
			rebondSink        = make(chan *contracts.BondingManagerRebond)
			withdrawStakeSink = make(chan *contracts.BondingManagerWithdrawStake)
			unbondSub         ethereum.Subscription
			rebondSub         ethereum.Subscription
			withdrawStakeSub  ethereum.Subscription
			err               error
		)

		resubscribe := func(subFunc func(*big.Int) (ethereum.Subscription, error)) (ethereum.Subscription, error) {
			startBlock, err := s.db.LastSeenBlock()
			if err != nil {
				return nil, err
			}

			return subFunc(startBlock)
		}

		for {
			if s.unbondResubscribe {
				unbondSub, err = resubscribe(func(startBlock *big.Int) (ethereum.Subscription, error) {
					return s.client.WatchForUnbond(startBlock, unbondSink)
				})
				if err != nil {
					glog.Error(err)
				} else {
					s.unbondResubscribe = false
				}
			}

			if s.rebondResubscribe {
				rebondSub, err = resubscribe(func(startBlock *big.Int) (ethereum.Subscription, error) {
					return s.client.WatchForRebond(startBlock, rebondSink)
				})
				if err != nil {
					glog.Error(err)
				} else {
					s.rebondResubscribe = false
				}
			}

			if s.withdrawStakeResubscribe {
				withdrawStakeSub, err = resubscribe(func(startBlock *big.Int) (ethereum.Subscription, error) {
					return s.client.WatchForWithdrawStake(startBlock, withdrawStakeSink)
				})
				if err != nil {
					glog.Error(err)
				} else {
					s.withdrawStakeResubscribe = false
				}
			}

			select {
			case newUnbond := <-unbondSink:
				// Insert new unbonding lock into database
				err := s.db.InsertUnbondingLock(newUnbond.UnbondingLockId, newUnbond.Delegator, newUnbond.Amount, newUnbond.WithdrawRound)
				if err != nil {
					glog.Error(err)
				}
			case newRebond := <-rebondSink:
				// Update unbonding lock in database as used
				err := s.db.UseUnbondingLock(newRebond.UnbondingLockId, newRebond.Delegator, new(big.Int).SetUint64(newRebond.Raw.BlockNumber))
				if err != nil {
					glog.Error(err)
				}
			case newWithdrawStake := <-withdrawStakeSink:
				// Update unbonding lock in database as used
				err := s.db.UseUnbondingLock(newWithdrawStake.UnbondingLockId, newWithdrawStake.Delegator, new(big.Int).SetUint64(newWithdrawStake.Raw.BlockNumber))
				if err != nil {
					glog.Error(err)
				}
			case unbondErr := <-unbondSub.Err():
				unbondSub.Unsubscribe()
				s.unbondResubscribe = true

				glog.Error("Error with Unbond subscription ", unbondErr)
			case rebondErr := <-rebondSub.Err():
				rebondSub.Unsubscribe()
				s.rebondResubscribe = true

				glog.Error("Error with Rebond subscription ", rebondErr)
			case withdrawStakeErr := <-withdrawStakeSub.Err():
				withdrawStakeSub.Unsubscribe()
				s.withdrawStakeResubscribe = true

				glog.Error("Error with WithdrawStake subscription ", withdrawStakeErr)
			case <-ctx.Done():
				unbondSub.Unsubscribe()
				rebondSub.Unsubscribe()
				withdrawStakeSub.Unsubscribe()

				glog.Infof("Received cancellation for unbonding service; stopping")

				return
			}
		}
	}()

	return nil
}

func (s *UnbondingService) Stop() error {
	if !s.working {
		return ErrUnbondingServiceStopped
	}

	s.cancelWorker()
	s.working = false

	return nil
}

func (s *UnbondingService) IsWorking() bool {
	return s.working
}
