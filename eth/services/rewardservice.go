package services

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/eth"
)

var (
	ErrRewardServiceStarted = fmt.Errorf("reward service already started")
	ErrRewardServiceStopped = fmt.Errorf("reward service already stopped")
)

type RewardService struct {
	eventMonitor eth.EventMonitor
	client       eth.LivepeerEthClient
	sub          ethereum.Subscription
	logsCh       chan types.Log
}

func NewRewardService(eventMonitor eth.EventMonitor, client eth.LivepeerEthClient) *RewardService {
	return &RewardService{
		eventMonitor: eventMonitor,
		client:       client,
	}
}

func (s *RewardService) Start(ctx context.Context) error {
	if s.sub != nil {
		return ErrRewardServiceStarted
	}

	logsCh := make(chan types.Log)
	sub, err := s.eventMonitor.SubscribeNewRound(ctx, logsCh, func(l types.Log) (bool, error) {
		round := parseNewRoundLog(l)
		return s.tryReward(round)
	})

	if err != nil {
		return err
	}

	s.logsCh = logsCh
	s.sub = sub

	return nil
}

func (s *RewardService) Stop() error {
	if s.sub == nil {
		return ErrRewardServiceStopped
	}

	close(s.logsCh)
	s.sub.Unsubscribe()

	s.logsCh = nil
	s.sub = nil

	return nil
}

func (s *RewardService) tryReward(round *big.Int) (bool, error) {
	active, err := s.client.IsActiveTranscoder()
	if err != nil {
		return false, err
	}

	if !active {
		glog.Infof("Not active for round %v", round)
		return true, nil
	}

	tx, err := s.client.Reward()
	if err != nil {
		return false, err
	}

	err = s.client.CheckTx(tx)
	if err != nil {
		return false, err
	}

	tp, err := s.client.GetTranscoderTokenPoolsForRound(s.client.Account().Address, round)
	if err != nil {
		return false, err
	}

	glog.Infof("Called reward for round %v - %v rewards minted", round, eth.FormatUnits(tp.RewardPool, "LPTU"))

	return true, nil
}

func parseNewRoundLog(log types.Log) (round *big.Int) {
	return new(big.Int).SetBytes(log.Data[0:32])
}
