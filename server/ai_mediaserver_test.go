package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/livepeer/go-livepeer/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockLivepeerNode creates a mock LivepeerNode with test pipelines
func mockLivepeerNode(streamIDs []string) *core.LivepeerNode {
	node := &core.LivepeerNode{
		LiveMu:        &sync.RWMutex{},
		LivePipelines: make(map[string]*core.LivePipeline),
	}

	for _, streamID := range streamIDs {
		pipeline := &core.LivePipeline{
			StreamID: streamID,
		}
		node.LivePipelines[streamID] = pipeline
	}

	return node
}

func TestSendHeartbeat_Success(t *testing.T) {
	// Test successful heartbeat with valid stream IDs
	streamIDs := []string{"stream1", "stream2", "stream3"}
	node := mockLivepeerNode(streamIDs)

	// Create test server to capture the request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request method and headers
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "test-value", r.Header.Get("X-Test-Header"))

		// Verify request body
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)

		var requestData map[string][]string
		err = json.Unmarshal(body, &requestData)
		require.NoError(t, err)

		ids, ok := requestData["ids"]
		require.True(t, ok)
		require.Len(t, ids, 3)

		assert.ElementsMatch(t, ids, streamIDs)

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	headers := map[string]string{
		"X-Test-Header": "test-value",
	}

	ctx := context.Background()
	sendHeartbeat(ctx, node, server.URL, headers)
}

func TestSendHeartbeat_EmptyStreamIDs(t *testing.T) {
	// Test heartbeat with no stream IDs
	node := mockLivepeerNode([]string{})

	// Create test server to capture the request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request body contains empty stream list
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)

		var requestData map[string][]string
		err = json.Unmarshal(body, &requestData)
		require.NoError(t, err)

		require.Len(t, requestData["ids"], 0, "ids array should be empty")

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ctx := context.Background()
	sendHeartbeat(ctx, node, server.URL, map[string]string{})
}
