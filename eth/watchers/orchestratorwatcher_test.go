package watchers

import (
	"math/big"
	"testing"
	"time"

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

	blockEvent.Type = blockwatch.Removed
	watcher.sink <- []*blockwatch.Event{blockEvent}
	time.Sleep(2 * time.Millisecond)
	assert.Equal(stubStore.activationRound, int64(5))
	assert.Equal(stubStore.deactivationRound, int64(100))
	assert.Equal(stubStore.ethereumAddr, lpEth.Orch.Address.String())
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
