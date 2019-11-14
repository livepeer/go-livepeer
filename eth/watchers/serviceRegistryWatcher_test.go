package watchers

import (
	"testing"
	"time"

	"github.com/livepeer/go-livepeer/eth"
	"github.com/livepeer/go-livepeer/eth/blockwatch"
	lpTypes "github.com/livepeer/go-livepeer/eth/types"
	"github.com/stretchr/testify/assert"
)

func TestServiceRegistryWatcher_WatchAndStop(t *testing.T) {
	assert := assert.New(t)
	watcher := &stubBlockWatcher{}
	stubStore := &stubOrchestratorStore{}
	lpEth := &eth.StubClient{}
	srw, err := NewServiceRegistryWatcher(stubServiceRegistryAddr, watcher, stubStore, lpEth)
	assert.Nil(err)

	go srw.Watch()
	time.Sleep(2 * time.Millisecond)

	// Test Stop
	srw.Stop()
	time.Sleep(2 * time.Millisecond)
	assert.True(watcher.sub.unsubscribed)
}

func TestServiceRegistryWatcher_HandleLog_HandleServiceURIUpdate(t *testing.T) {
	assert := assert.New(t)
	watcher := &stubBlockWatcher{}
	stubStore := &stubOrchestratorStore{}
	lpEth := &eth.StubClient{
		Orch: &lpTypes.Transcoder{
			Address:    stubTranscoder,
			ServiceURI: "http://127.0.0.1:1337",
		},
	}
	srw, err := NewServiceRegistryWatcher(stubServiceRegistryAddr, watcher, stubStore, lpEth)
	assert.Nil(err)

	header := defaultMiniHeader()
	header.Logs = append(header.Logs, newStubServiceURIUpdateLog())

	blockEvent := &blockwatch.Event{
		Type:        blockwatch.Added,
		BlockHeader: header,
	}

	go srw.Watch()
	defer srw.Stop()
	time.Sleep(2 * time.Millisecond)

	watcher.sink <- []*blockwatch.Event{blockEvent}
	time.Sleep(2 * time.Millisecond)
	assert.Equal(stubUpdatedServiceURI, stubStore.serviceURI)
	assert.Equal(stubStore.ethereumAddr, stubTranscoder.String())

	// Test log removed
	blockEvent.Type = blockwatch.Removed
	watcher.sink <- []*blockwatch.Event{blockEvent}
	time.Sleep(2 * time.Millisecond)
	assert.Equal(lpEth.Orch.ServiceURI, stubStore.serviceURI)
	assert.Equal(lpEth.Orch.Address.String(), stubStore.ethereumAddr)
}
