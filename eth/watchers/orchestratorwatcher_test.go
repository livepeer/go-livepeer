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
)

func TestOrchWatcher_WatchAndStop(t *testing.T) {
	assert := assert.New(t)
	watcher := &stubBlockWatcher{}
	stubStore := &stubOrchestratorStore{}
	lpEth := &eth.StubClient{}
	ow, err := NewOrchestratorWatcher(stubBondingManagerAddr, watcher, stubStore, lpEth)
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
	ow, err := NewOrchestratorWatcher(stubBondingManagerAddr, watcher, stubStore, lpEth)
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
	ow, err := NewOrchestratorWatcher(stubBondingManagerAddr, watcher, stubStore, lpEth)
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
