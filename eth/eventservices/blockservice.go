package eventservices

import (
	"context"
	"fmt"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/eth"
)

var (
	ErrBlockServiceStarted = fmt.Errorf("block service already started")
	ErrBlockServiceStopped = fmt.Errorf("block service already stopped")
)

type BlockService struct {
	eventMonitor eth.EventMonitor
	db           *common.DB
	working      bool
	sub          ethereum.Subscription
}

func NewBlockService(eventMonitor eth.EventMonitor, db *common.DB) *BlockService {
	return &BlockService{
		eventMonitor: eventMonitor,
		db:           db,
	}
}

func (s *BlockService) Start(ctx context.Context) error {
	if s.working {
		return ErrBlockServiceStarted
	}

	sub, err := s.eventMonitor.SubscribeNewBlock(ctx, "BlockWatcher", make(chan *types.Header), func(h *types.Header) (bool, error) {
		s.db.SetLastSeenBlock(h.Number)
		return true, nil
	})
	if err != nil {
		return err
	}

	s.sub = sub

	return nil
}

func (s *BlockService) Stop() error {
	if !s.working {
		return ErrBlockServiceStopped
	}

	s.sub.Unsubscribe()

	s.sub = nil

	return nil
}

func (s *BlockService) IsWorking() bool {
	return s.sub != nil
}
