package eth

import (
	"context"
	"fmt"
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
	SubscribeNewJob(context.Context, string, chan types.Log, common.Address, logCallback) (ethereum.Subscription, error)
	SubscribeNewRound(context.Context, string, chan types.Log, logCallback) (ethereum.Subscription, error)
	SubscribeNewBlock(context.Context, string, chan *types.Header, headerCallback) (ethereum.Subscription, error)
	EventSubscriptions() map[string]bool
}

type EventSubscription struct {
	sub       ethereum.Subscription
	logsCh    chan types.Log
	headersCh chan *types.Header
	active    bool
}

type eventMonitor struct {
	backend         *ethclient.Client
	contractAddrMap map[string]common.Address
	eventSubMap     map[string]*EventSubscription
}

func NewEventMonitor(backend *ethclient.Client, contractAddrMap map[string]common.Address) EventMonitor {
	return &eventMonitor{
		backend:         backend,
		contractAddrMap: contractAddrMap,
		eventSubMap:     make(map[string]*EventSubscription),
	}
}

func (em *eventMonitor) EventSubscriptions() map[string]bool {
	activeSubMap := make(map[string]bool)

	for k, v := range em.eventSubMap {
		if v.active {
			activeSubMap[k] = true
		}
	}

	return activeSubMap
}

func (em *eventMonitor) SubscribeNewRound(ctx context.Context, subName string, logsCh chan types.Log, cb logCallback) (ethereum.Subscription, error) {
	if _, ok := em.eventSubMap[subName]; ok {
		return nil, fmt.Errorf("Event subscription already registered as active with name: %v", subName)
	}

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

	em.eventSubMap[subName] = &EventSubscription{
		sub:    sub,
		logsCh: logsCh,
	}

	go em.watchLogs(subName, cb, func() {
		glog.Infof("Trying to resubscribe for %v", subName)

		sub, err = em.backend.SubscribeFilterLogs(ctx, q, logsCh)
		if err != nil {
			glog.Error(err)
			return
		}

		em.eventSubMap[subName].sub = sub
		em.eventSubMap[subName].logsCh = logsCh
	})

	return sub, nil
}

func (em *eventMonitor) SubscribeNewJob(ctx context.Context, subName string, logsCh chan types.Log, broadcasterAddr common.Address, cb logCallback) (ethereum.Subscription, error) {
	if _, ok := em.eventSubMap[subName]; ok {
		return nil, fmt.Errorf("Event subscription already registered as active with name: %v", subName)
	}

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

	em.eventSubMap[subName] = &EventSubscription{
		sub:    sub,
		logsCh: logsCh,
	}

	go em.watchLogs(subName, cb, func() {
		glog.Infof("Trying to resubscribe for %v", subName)

		sub, err = em.backend.SubscribeFilterLogs(ctx, q, logsCh)
		if err != nil {
			glog.Error(err)
			return
		}

		em.eventSubMap[subName].sub = sub
		em.eventSubMap[subName].logsCh = logsCh
	})

	return sub, nil
}

func (em *eventMonitor) SubscribeNewBlock(ctx context.Context, subName string, headersCh chan *types.Header, cb headerCallback) (ethereum.Subscription, error) {
	if _, ok := em.eventSubMap[subName]; ok {
		return nil, fmt.Errorf("Event subscription already registered as active with name: %v", subName)
	}

	sub, err := em.backend.SubscribeNewHead(ctx, headersCh)
	if err != nil {
		return nil, err
	}

	em.eventSubMap[subName] = &EventSubscription{
		sub:       sub,
		headersCh: headersCh,
	}

	go em.watchBlocks(subName, sub, headersCh, cb, func() {
		glog.Infof("Trying to resubscribe for %v", subName)

		sub, err = em.backend.SubscribeNewHead(ctx, headersCh)
		if err != nil {
			glog.Error(err)
			return
		}

		em.eventSubMap[subName].sub = sub
		em.eventSubMap[subName].headersCh = headersCh
	})

	return sub, nil
}

func (em *eventMonitor) setSubActive(subName string) {
	em.eventSubMap[subName].active = true
}

func (em *eventMonitor) setSubInactive(subName string) {
	em.eventSubMap[subName].active = false
}

func (em *eventMonitor) watchLogs(subName string, eventCb logCallback, errCb func()) {
	em.setSubActive(subName)
	defer em.setSubInactive(subName)

	for {
		select {
		case l, ok := <-em.eventSubMap[subName].logsCh:
			if !ok {
				glog.Errorf("Logs channel closed")
				return
			}

			watch, err := eventCb(l)
			if err != nil {
				glog.Errorf("Error with log callback: %v", err)
			}

			if !watch {
				glog.Infof("Done watching for logs")
				return
			}
		case err := <-em.eventSubMap[subName].sub.Err():
			glog.Errorf("Error with log subscription: %v", err)

			errCb()
		}
	}
}

func (em *eventMonitor) watchBlocks(subName string, sub ethereum.Subscription, headersCh chan *types.Header, eventCb headerCallback, errCb func()) {
	em.setSubActive(subName)
	defer em.setSubInactive(subName)

	for {
		select {
		case h, ok := <-em.eventSubMap[subName].headersCh:
			if !ok {
				glog.Errorf("Logs channel closed")
				return
			}

			watch, err := eventCb(h)
			if err != nil {
				glog.Errorf("Error with header callback: %v", err)
			}

			if !watch {
				glog.Infof("Done watching")
				return
			}
		case err := <-em.eventSubMap[subName].sub.Err():
			glog.Errorf("Error with header subscription: %v", err)

			errCb()
		}
	}
}
