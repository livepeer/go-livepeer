package server

import (
	"context"
	"github.com/livepeer/go-livepeer/core"
	"github.com/stretchr/testify/require"
	"sync"
	"testing"
)

func TestOrchestratorSwapper(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()

	stream := "stream-123"
	params := aiRequestParams{
		liveParams: &liveRequestParams{
			stream: stream,
		},
		node: &core.LivepeerNode{
			LiveMu:        &sync.RWMutex{},
			LivePipelines: make(map[string]*core.LivePipeline),
		},
	}
	orchSwapper := NewOrchestratorSwapper(params)

	// No input, should not swap
	require.False(orchSwapper.shouldSwap(ctx))

	// Set input, should swap
	params.node.LivePipelines[stream] = &core.LivePipeline{}
	require.True(orchSwapper.shouldSwap(ctx))

	// Too many swaps in a short time, should not swap
	for i := 0; i < 5; i++ {
		orchSwapper.shouldSwap(ctx)
	}
	require.False(orchSwapper.shouldSwap(ctx))
}
