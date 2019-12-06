package watchers

import (
	"math"
	"math/big"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/eth"
	"github.com/livepeer/go-livepeer/eth/blockwatch"
	"github.com/livepeer/go-livepeer/eth/contracts"
)

const maxFutureRound = int64(math.MaxInt64)

type OrchestratorWatcher struct {
	store   common.OrchestratorStore
	dec     *EventDecoder
	watcher BlockWatcher
	lpEth   eth.LivepeerEthClient
	rw      EventWatcher
	quit    chan struct{}
}

func NewOrchestratorWatcher(bondingManagerAddr ethcommon.Address, watcher BlockWatcher, store common.OrchestratorStore, lpEth eth.LivepeerEthClient, rw EventWatcher) (*OrchestratorWatcher, error) {
	dec, err := NewEventDecoder(bondingManagerAddr, contracts.BondingManagerABI)
	if err != nil {
		return nil, err
	}

	return &OrchestratorWatcher{
		store:   store,
		dec:     dec,
		watcher: watcher,
		lpEth:   lpEth,
		rw:      rw,
		quit:    make(chan struct{}),
	}, nil
}

// Watch starts the event watching loop
func (ow *OrchestratorWatcher) Watch() {
	roundEvents := make(chan types.Log, 10)
	roundSub := ow.rw.Subscribe(roundEvents)
	defer roundSub.Unsubscribe()

	events := make(chan []*blockwatch.Event, 10)
	sub := ow.watcher.Subscribe(events)
	defer sub.Unsubscribe()

	for {
		select {
		case <-ow.quit:
			return
		case err := <-sub.Err():
			glog.Error(err)
		case events := <-events:
			ow.handleBlockEvents(events)
		case roundEvent := <-roundEvents:
			if err := ow.handleRoundEvent(roundEvent); err != nil {
				glog.Errorf("error handling new round event: %v", err)
			}
		}
	}
}

// Stop watching for events
func (ow *OrchestratorWatcher) Stop() {
	close(ow.quit)
}

func (ow *OrchestratorWatcher) handleBlockEvents(events []*blockwatch.Event) {
	for _, event := range events {
		for _, log := range event.BlockHeader.Logs {
			if event.Type == blockwatch.Removed {
				log.Removed = true
			}
			if err := ow.handleLog(log); err != nil {
				glog.Error(err)
			}
		}
	}
}

func (ow *OrchestratorWatcher) handleLog(log types.Log) error {
	eventName, err := ow.dec.FindEventName(log)
	if err != nil {
		// Noop if we cannot find the event name
		return nil
	}

	switch eventName {
	case "TranscoderActivated":
		return ow.handleTranscoderActivated(log)
	case "TranscoderDeactivated":
		return ow.handleTranscoderDeactivated(log)
	default:
		return nil
	}
}

func (ow *OrchestratorWatcher) handleTranscoderActivated(log types.Log) error {
	var transcoderActivated contracts.BondingManagerTranscoderActivated
	if err := ow.dec.Decode("TranscoderActivated", log, &transcoderActivated); err != nil {
		return err
	}

	if !log.Removed {
		uri, err := ow.lpEth.GetServiceURI(transcoderActivated.Transcoder)
		if err != nil {
			return err
		}

		return ow.store.UpdateOrch(
			&common.DBOrch{
				EthereumAddr:      transcoderActivated.Transcoder.String(),
				ServiceURI:        uri,
				ActivationRound:   transcoderActivated.ActivationRound.Int64(),
				DeactivationRound: maxFutureRound,
			},
		)
	}
	t, err := ow.lpEth.GetTranscoder(transcoderActivated.Transcoder)
	if err != nil {
		return err
	}
	return ow.store.UpdateOrch(
		&common.DBOrch{
			EthereumAddr:      t.Address.String(),
			ServiceURI:        t.ServiceURI,
			ActivationRound:   t.ActivationRound.Int64(),
			DeactivationRound: t.DeactivationRound.Int64(),
		},
	)
}

func (ow *OrchestratorWatcher) handleTranscoderDeactivated(log types.Log) error {
	var transcoderDeactivated contracts.BondingManagerTranscoderDeactivated
	if err := ow.dec.Decode("TranscoderDeactivated", log, &transcoderDeactivated); err != nil {
		return err
	}

	if !log.Removed {
		return ow.store.UpdateOrch(
			&common.DBOrch{
				EthereumAddr:      transcoderDeactivated.Transcoder.String(),
				DeactivationRound: transcoderDeactivated.DeactivationRound.Int64(),
			},
		)
	}
	t, err := ow.lpEth.GetTranscoder(transcoderDeactivated.Transcoder)
	if err != nil {
		return err
	}
	return ow.store.UpdateOrch(
		&common.DBOrch{
			EthereumAddr:      t.Address.String(),
			ActivationRound:   t.ActivationRound.Int64(),
			DeactivationRound: t.DeactivationRound.Int64(),
		},
	)
}

func (ow *OrchestratorWatcher) handleRoundEvent(log types.Log) error {
	round, err := ow.lpEth.CurrentRound()
	if err != nil {
		return err
	}

	orchs, err := ow.store.SelectOrchs(&common.DBOrchFilter{CurrentRound: round})
	if err != nil {
		return err
	}

	for _, o := range orchs {
		if err := ow.cacheOrchestratorStake(ethcommon.HexToAddress(o.EthereumAddr), round); err != nil {
			glog.Errorf("could not cache stake update for orchestrator %v and round %v", o.EthereumAddr, round)
		}
	}

	return nil
}

func (ow *OrchestratorWatcher) cacheOrchestratorStake(addr ethcommon.Address, round *big.Int) error {
	ep, err := ow.lpEth.GetTranscoderEarningsPoolForRound(addr, round)
	if err != nil {
		return err
	}

	if err := ow.store.UpdateOrch(
		&common.DBOrch{
			EthereumAddr: addr.Hex(),
			Stake:        ep.TotalStake.Bytes(),
		},
	); err != nil {
		return err
	}

	return nil
}
