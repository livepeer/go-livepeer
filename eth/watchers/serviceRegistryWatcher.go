package watchers

import (
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/eth"
	"github.com/livepeer/go-livepeer/eth/blockwatch"
	"github.com/livepeer/go-livepeer/eth/contracts"
)

type ServiceRegistryWatcher struct {
	store   common.OrchestratorStore
	dec     *EventDecoder
	watcher BlockWatcher
	lpEth   eth.LivepeerEthClient
	quit    chan struct{}
}

func NewServiceRegistryWatcher(serviceRegistryAddr ethcommon.Address, watcher BlockWatcher, store common.OrchestratorStore, lpEth eth.LivepeerEthClient) (*ServiceRegistryWatcher, error) {
	dec, err := NewEventDecoder(serviceRegistryAddr, contracts.ServiceRegistryABI)
	if err != nil {
		return nil, err
	}

	return &ServiceRegistryWatcher{
		store:   store,
		dec:     dec,
		watcher: watcher,
		lpEth:   lpEth,
		quit:    make(chan struct{}),
	}, nil
}

// Watch starts the event watching loop
func (srw *ServiceRegistryWatcher) Watch() {
	events := make(chan []*blockwatch.Event, 10)
	sub := srw.watcher.Subscribe(events)
	defer sub.Unsubscribe()

	for {
		select {
		case <-srw.quit:
			return
		case err := <-sub.Err():
			glog.Error(err)
		case events := <-events:
			srw.handleBlockEvents(events)
		}
	}
}

// Stop watching for events
func (srw *ServiceRegistryWatcher) Stop() {
	close(srw.quit)
}

func (srw *ServiceRegistryWatcher) handleBlockEvents(events []*blockwatch.Event) {
	for _, event := range events {
		for _, log := range event.BlockHeader.Logs {
			if event.Type == blockwatch.Removed {
				log.Removed = true
			}
			if err := srw.handleLog(log); err != nil {
				glog.Error(err)
			}
		}
	}
}

func (srw *ServiceRegistryWatcher) handleLog(log types.Log) error {
	eventName, err := srw.dec.FindEventName(log)
	if err != nil {
		// Noop if we cannot find the event name
		return nil
	}

	switch eventName {
	case "ServiceURIUpdate":
		return srw.handleServiceURIUpdate(log)
	default:
		return nil
	}
}

func (srw *ServiceRegistryWatcher) handleServiceURIUpdate(log types.Log) error {
	var serviceURIUpdate contracts.ServiceRegistryServiceURIUpdate
	if err := srw.dec.Decode("ServiceURIUpdate", log, &serviceURIUpdate); err != nil {
		return err
	}

	if !log.Removed {
		return srw.store.UpdateOrch(
			&common.DBOrch{
				EthereumAddr: serviceURIUpdate.Addr.String(),
				ServiceURI:   serviceURIUpdate.ServiceURI,
			},
		)
	}
	uri, err := srw.lpEth.GetServiceURI(serviceURIUpdate.Addr)
	if err != nil {
		return err
	}
	return srw.store.UpdateOrch(
		&common.DBOrch{
			EthereumAddr: serviceURIUpdate.Addr.String(),
			ServiceURI:   uri,
		},
	)
}
