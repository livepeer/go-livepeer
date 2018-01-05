package services

import (
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/eth"
)

var (
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
		return s.tryInitializeRound()
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

func (s *RoundsService) tryInitializeRound() (bool, error) {
	currentRound, err := s.client.CurrentRound()
	if err != nil {
		return false, err
	}

	lastInitializedRound, err := s.client.LastInitializedRound()
	if err != nil {
		return false, err
	}

	if lastInitializedRound.Cmp(currentRound) == -1 {
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

	return true, nil
}
