package eth

import (
	"context"
	"strings"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/eth/contracts"
)

type logCallback func(types.Log) (bool, error)
type headerCallback func(*types.Header) (bool, error)

type EventMonitor interface {
	SubscribeNewJob(context.Context, chan types.Log, common.Address, logCallback) (ethereum.Subscription, error)
	SubscribeNewRound(context.Context, chan types.Log, logCallback) (ethereum.Subscription, error)
	SubscribeNewBlock(context.Context, chan *types.Header, headerCallback) (ethereum.Subscription, error)
}

type eventMonitor struct {
	backend         *ethclient.Client
	contractAddrMap map[string]common.Address
}

func NewEventMonitor(backend *ethclient.Client, contractAddrMap map[string]common.Address) EventMonitor {
	return &eventMonitor{
		backend:         backend,
		contractAddrMap: contractAddrMap,
	}
}

func (em *eventMonitor) SubscribeNewRound(ctx context.Context, logsCh chan types.Log, cb logCallback) (ethereum.Subscription, error) {
	abiJSON, err := abi.JSON(strings.NewReader(contracts.RoundsManagerABI))
	if err != nil {
		return nil, err
	}

	eventId := abiJSON.Events["NewRound"].Id()
	roundsManagerAddr := em.contractAddrMap["RoundsManager"]

	q := ethereum.FilterQuery{
		Addresses: []common.Address{roundsManagerAddr},
		Topics:    [][]common.Hash{[]common.Hash{eventId}},
	}

	sub, err := em.backend.SubscribeFilterLogs(ctx, q, logsCh)
	if err != nil {
		return nil, err
	}

	go watchLogs(sub, logsCh, cb)

	return sub, nil
}

func (em *eventMonitor) SubscribeNewJob(ctx context.Context, logsCh chan types.Log, broadcasterAddr common.Address, cb logCallback) (ethereum.Subscription, error) {
	abiJSON, err := abi.JSON(strings.NewReader(contracts.JobsManagerABI))
	if err != nil {
		return nil, err
	}

	eventId := abiJSON.Events["NewJob"].Id()
	jobsManagerAddr := em.contractAddrMap["JobsManager"]

	var q ethereum.FilterQuery
	if !IsNullAddress(broadcasterAddr) {
		q = ethereum.FilterQuery{
			Addresses: []common.Address{jobsManagerAddr},
			Topics:    [][]common.Hash{[]common.Hash{eventId}, []common.Hash{}, []common.Hash{common.BytesToHash(common.LeftPadBytes(broadcasterAddr[:], 32))}},
		}
	} else {
		q = ethereum.FilterQuery{
			Addresses: []common.Address{jobsManagerAddr},
			Topics:    [][]common.Hash{[]common.Hash{eventId}},
		}
	}

	sub, err := em.backend.SubscribeFilterLogs(ctx, q, logsCh)
	if err != nil {
		return nil, err
	}

	go watchLogs(sub, logsCh, cb)

	return sub, nil
}

func (es *eventMonitor) SubscribeNewBlock(ctx context.Context, headersCh chan *types.Header, cb headerCallback) (ethereum.Subscription, error) {
	sub, err := es.backend.SubscribeNewHead(ctx, headersCh)
	if err != nil {
		return nil, err
	}

	go watchBlocks(sub, headersCh, cb)

	return sub, nil
}

func watchLogs(sub ethereum.Subscription, logsCh chan types.Log, cb logCallback) {
	for {
		select {
		case l, ok := <-logsCh:
			if !ok {
				return
			}

			watch, err := cb(l)
			if err != nil {
				glog.Errorf("Error with log callback: %v", err)
				return
			}

			if !watch {
				glog.Infof("Done watching")
				return
			}
		case err := <-sub.Err():
			glog.Errorf("Error with log subscription: %v", err)
			return
		}
	}
}

func watchBlocks(sub ethereum.Subscription, headersCh chan *types.Header, cb headerCallback) {
	for {
		select {
		case h, ok := <-headersCh:
			if !ok {
				return
			}

			watch, err := cb(h)
			if err != nil {
				glog.Errorf("Error with header callback: %v", err)
				return
			}

			if !watch {
				glog.Infof("Done watching")
				return
			}
		case err := <-sub.Err():
			glog.Errorf("Error with header subscription: %v", err)
			return
		}
	}
}
