package eth

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/eth/contracts"
)

var SubscribeRetry = uint64(3)

type logCallback func(types.Log) (bool, error)
type headerCallback func(*types.Header) (bool, error)

type EventMonitor interface {
	SubscribeNewJob(context.Context, string, chan types.Log, ethcommon.Address, logCallback) (ethereum.Subscription, error)
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
	contractAddrMap map[string]ethcommon.Address
	eventSubMap     map[string]*EventSubscription
}

func NewEventMonitor(backend *ethclient.Client, contractAddrMap map[string]ethcommon.Address) EventMonitor {
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
		Addresses: []ethcommon.Address{roundsManagerAddr},
		Topics:    [][]ethcommon.Hash{[]ethcommon.Hash{eventId}},
	}

	subscribe := func() error {
		sub, err := em.backend.SubscribeFilterLogs(ctx, q, logsCh)
		if err != nil {
			glog.V(common.DEBUG).Infof("SubscribeNewRound error: %v. Retrying...", err)
			return err
		} else {
			glog.V(common.DEBUG).Infof("SubscribeNewRound successful.")
		}

		em.eventSubMap[subName] = &EventSubscription{
			sub:    sub,
			logsCh: logsCh,
			active: true,
		}

		return nil
	}

	if err := backoff.Retry(subscribe, backoff.NewConstantBackOff(time.Second*2)); err != nil {
		glog.Errorf("SubscribeNewRound error: %v", err)
		return nil, err
	}

	go em.watchLogs(subName, cb, func() {
		glog.V(common.DEBUG).Infof("Trying to resubscribe for %v", subName)
		if err := backoff.Retry(subscribe, backoff.NewConstantBackOff(time.Second*2)); err != nil {
			glog.V(common.DEBUG).Infof("Resubscription error: %v", err)
			return
		}
	})

	return em.eventSubMap[subName].sub, nil
}

func (em *eventMonitor) SubscribeNewJob(ctx context.Context, subName string, logsCh chan types.Log, broadcasterAddr ethcommon.Address, cb logCallback) (ethereum.Subscription, error) {
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
			Addresses: []ethcommon.Address{jobsManagerAddr},
			Topics:    [][]ethcommon.Hash{[]ethcommon.Hash{eventId}, []ethcommon.Hash{}, []ethcommon.Hash{ethcommon.BytesToHash(ethcommon.LeftPadBytes(broadcasterAddr[:], 32))}},
		}
	} else {
		q = ethereum.FilterQuery{
			Addresses: []ethcommon.Address{jobsManagerAddr},
			Topics:    [][]ethcommon.Hash{[]ethcommon.Hash{eventId}},
		}
	}

	subscribe := func() error {
		sub, err := em.backend.SubscribeFilterLogs(ctx, q, logsCh)
		if err != nil {
			glog.V(common.DEBUG).Infof("SubscribeNewJob error: %v. retrying...", err)
			return err
		} else {
			glog.V(common.DEBUG).Infof("SubscribedNewJob successful.")
		}

		em.eventSubMap[subName] = &EventSubscription{
			sub:    sub,
			logsCh: logsCh,
			active: true,
		}
		return nil
	}

	if err = backoff.Retry(subscribe, backoff.NewConstantBackOff(time.Second*2)); err != nil {
		glog.Errorf("SubscribeNewJob failed: %v", err)
		return nil, err
	}

	go em.watchLogs(subName, cb, func() {
		glog.V(common.DEBUG).Infof("Trying to resubscribe for %v", subName)
		if err := backoff.Retry(subscribe, backoff.NewConstantBackOff(time.Second*2)); err != nil {
			glog.V(common.DEBUG).Infof("Resubscribe failed: %v", err)
			return
		}
	})

	return em.eventSubMap[subName].sub, nil
}

func (em *eventMonitor) SubscribeNewBlock(ctx context.Context, subName string, headersCh chan *types.Header, cb headerCallback) (ethereum.Subscription, error) {
	if _, ok := em.eventSubMap[subName]; ok {
		return nil, fmt.Errorf("Event subscription already registered as active with name: %v", subName)
	}

	subscribe := func() error {
		sub, err := em.backend.SubscribeNewHead(ctx, headersCh)
		if err != nil {
			glog.V(common.DEBUG).Infof("SubscribeNewHead error: %v. retrying...", err)
			return err
		} else {
			glog.V(common.DEBUG).Infof("SubscribeNewHead successful.")
		}

		em.eventSubMap[subName] = &EventSubscription{
			sub:       sub,
			headersCh: headersCh,
			active:    true,
		}
		return nil
	}
	if err := backoff.Retry(subscribe, backoff.NewConstantBackOff(time.Second*2)); err != nil {
		glog.Errorf("SubscribeNewHead failed: %v", err)
		return nil, err
	}

	go em.watchBlocks(subName, em.eventSubMap[subName].sub, headersCh, cb, func() {
		glog.V(common.DEBUG).Infof("Trying to resubscribe for %v", subName)
		if err := backoff.Retry(subscribe, backoff.NewConstantBackOff(time.Second*2)); err != nil {
			glog.V(common.DEBUG).Infof("Resubscribe failed: %v", err)
			return
		}
	})

	return em.eventSubMap[subName].sub, nil
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
			glog.Errorf("Error with log subscription for %v: %v", subName, err)

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
			glog.Errorf("Error with header subscription for %v: %v", subName, err)

			errCb()
		}
	}
}
