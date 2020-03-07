package watchers

import (
	"errors"
	"math/big"
	"testing"
	"time"

	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/eth"
	"github.com/livepeer/go-livepeer/eth/blockwatch"
	lpTypes "github.com/livepeer/go-livepeer/eth/types"
	"github.com/livepeer/go-livepeer/pm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOrchWatcher_WatchAndStop(t *testing.T) {
	assert := assert.New(t)
	watcher := &stubBlockWatcher{}
	stubStore := &stubOrchestratorStore{}
	lpEth := &eth.StubClient{}
	tw := &stubTimeWatcher{}

	ow, err := NewOrchestratorWatcher(stubBondingManagerAddr, watcher, stubStore, lpEth, tw)
	assert.Nil(err)

	go ow.Watch()
	time.Sleep(2 * time.Millisecond)

	// Test Stop
	ow.Stop()
	time.Sleep(2 * time.Millisecond)
	assert.True(watcher.sub.unsubscribed)
}

func TestOrchWatcher_HandleLog_TranscoderActivated(t *testing.T) {
	assert := assert.New(t)
	watcher := &stubBlockWatcher{}
	stubStore := &stubOrchestratorStore{}
	lpEth := &eth.StubClient{
		Orch: &lpTypes.Transcoder{
			Address:           pm.RandAddress(),
			ActivationRound:   big.NewInt(5),
			DeactivationRound: big.NewInt(100),
			ServiceURI:        "http://mytranscoder.lpt:1337",
		},
	}
	tw := &stubTimeWatcher{}

	ow, err := NewOrchestratorWatcher(stubBondingManagerAddr, watcher, stubStore, lpEth, tw)
	assert.Nil(err)

	header := defaultMiniHeader()
	header.Logs = append(header.Logs, newStubTranscoderActivatedLog())

	blockEvent := &blockwatch.Event{
		Type:        blockwatch.Added,
		BlockHeader: header,
	}

	go ow.Watch()
	defer ow.Stop()
	time.Sleep(2 * time.Millisecond)

	watcher.sink <- []*blockwatch.Event{blockEvent}
	time.Sleep(2 * time.Millisecond)
	assert.Equal(stubActivationRound.Int64(), stubStore.activationRound)
	assert.Equal(stubStore.deactivationRound, maxFutureRound)
	assert.Equal(stubStore.ethereumAddr, stubTranscoder.String())
	assert.Equal(stubStore.serviceURI, "http://mytranscoder.lpt:1337")

	// test GetServiceURI error
	errorLogsBefore := glog.Stats.Error.Lines()
	lpEth.Err = errors.New("GetServiceURI error")
	watcher.sink <- []*blockwatch.Event{blockEvent}
	time.Sleep(2 * time.Millisecond)
	errorLogsAfter := glog.Stats.Error.Lines()
	assert.Equal(int64(1), errorLogsAfter-errorLogsBefore)
	lpEth.Err = nil

	blockEvent.Type = blockwatch.Removed
	lpEth.Orch.ServiceURI = "http://mytranscoder.lpt:0000"
	watcher.sink <- []*blockwatch.Event{blockEvent}
	time.Sleep(2 * time.Millisecond)
	assert.Equal(stubStore.activationRound, int64(5))
	assert.Equal(stubStore.deactivationRound, int64(100))
	assert.Equal(stubStore.ethereumAddr, lpEth.Orch.Address.String())
	assert.Equal(stubStore.serviceURI, "http://mytranscoder.lpt:0000")

	// test GetTranscoder error
	errorLogsBefore = glog.Stats.Error.Lines()
	lpEth.Err = errors.New("GetTranscoder error")
	watcher.sink <- []*blockwatch.Event{blockEvent}
	time.Sleep(2 * time.Millisecond)
	errorLogsAfter = glog.Stats.Error.Lines()
	assert.Equal(int64(1), errorLogsAfter-errorLogsBefore)
	lpEth.Err = nil
}

func TestOrchWatcher_HandleLog_TranscoderDeactivated(t *testing.T) {
	assert := assert.New(t)
	watcher := &stubBlockWatcher{}
	stubStore := &stubOrchestratorStore{}
	lpEth := &eth.StubClient{
		Orch: &lpTypes.Transcoder{
			Address:           pm.RandAddress(),
			ActivationRound:   big.NewInt(5),
			DeactivationRound: big.NewInt(10),
		},
	}
	tw := &stubTimeWatcher{}

	ow, err := NewOrchestratorWatcher(stubBondingManagerAddr, watcher, stubStore, lpEth, tw)
	assert.Nil(err)

	header := defaultMiniHeader()
	header.Logs = append(header.Logs, newStubTranscoderDeactivatedLog())

	blockEvent := &blockwatch.Event{
		Type:        blockwatch.Added,
		BlockHeader: header,
	}

	go ow.Watch()
	defer ow.Stop()
	time.Sleep(2 * time.Millisecond)

	watcher.sink <- []*blockwatch.Event{blockEvent}
	time.Sleep(2 * time.Millisecond)
	assert.Equal(stubDeactivationRound.Int64(), stubStore.deactivationRound)
	assert.Equal(stubStore.ethereumAddr, stubTranscoder.String())

	blockEvent.Type = blockwatch.Removed
	watcher.sink <- []*blockwatch.Event{blockEvent}
	time.Sleep(2 * time.Millisecond)
	assert.Equal(stubStore.deactivationRound, int64(10))
	assert.Equal(stubStore.activationRound, int64(5))
	assert.Equal(stubStore.ethereumAddr, lpEth.Orch.Address.String())
}

func TestOrchWatcher_HandleRoundEvent_CacheOrchestratorStake(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	expStake, ok := new(big.Int).SetString("5000000000000000000000", 10) // 5000 LPT
	require.True(ok)

	watcher := &stubBlockWatcher{}
	stubStore := &stubOrchestratorStore{}
	lpEth := &eth.StubClient{
		TotalStake: expStake,
	}
	tw := &stubTimeWatcher{}

	ow, err := NewOrchestratorWatcher(stubBondingManagerAddr, watcher, stubStore, lpEth, tw)
	require.Nil(err)
	require.NotNil(ow)

	// Fire a new round event
	newRoundEvent := newStubNewRoundLog()

	go ow.Watch()
	defer ow.Stop()

	time.Sleep(2 * time.Millisecond)
	tw.sink <- newRoundEvent
	time.Sleep(2 * time.Millisecond)

	assert.Equal(stubStore.stake, int64(500000000))

	// LivepeerEthClient.CurrentRound() error
	lpEth.RoundsErr = errors.New("CurrentRound error")
	errorLogsBefore := glog.Stats.Error.Lines()
	tw.sink <- newRoundEvent
	time.Sleep(2 * time.Millisecond)
	errorLogsAfter := glog.Stats.Error.Lines()
	assert.Equal(int64(1), errorLogsAfter-errorLogsBefore)
	lpEth.RoundsErr = nil

	// OrchestratorStore.SelectOrchs error
	stubStore.selectErr = errors.New("SelectOrchs error")
	errorLogsBefore = glog.Stats.Error.Lines()
	tw.sink <- newRoundEvent
	time.Sleep(2 * time.Millisecond)
	errorLogsAfter = glog.Stats.Error.Lines()
	assert.Equal(int64(1), errorLogsAfter-errorLogsBefore)
	stubStore.selectErr = nil

	// LivepeerEthClient.GetTranscoderEarningsPoolForRound() error
	lpEth.TranscoderPoolError = errors.New("TranscoderEarningsPoolForRound error")
	errorLogsBefore = glog.Stats.Error.Lines()
	tw.sink <- newRoundEvent
	time.Sleep(2 * time.Millisecond)
	errorLogsAfter = glog.Stats.Error.Lines()
	assert.Equal(int64(1), errorLogsAfter-errorLogsBefore)
	lpEth.TranscoderPoolError = nil

	// OrchestratorStore.UpdateOrch error
	stubStore.updateErr = errors.New("UpdateOrch error")
	errorLogsBefore = glog.Stats.Error.Lines()
	tw.sink <- newRoundEvent
	time.Sleep(2 * time.Millisecond)
	errorLogsAfter = glog.Stats.Error.Lines()
	assert.Equal(int64(1), errorLogsAfter-errorLogsBefore)
	stubStore.updateErr = nil
}
