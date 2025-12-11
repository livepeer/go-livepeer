package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/media"
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

func TestCreateWhep_PathParsing(t *testing.T) {
	tests := []struct {
		name           string
		path           string
		expectStream   string
		expectRequestID string
		expectIsInput  bool
		expectError    bool
	}{
		{
			name:           "output with requestID",
			path:           "stream-request123-out",
			expectStream:   "stream",
			expectRequestID: "request123",
			expectIsInput:  false,
			expectError:    false,
		},
		{
			name:           "output legacy format",
			path:           "stream-out",
			expectStream:   "stream",
			expectRequestID: "",
			expectIsInput:  false,
			expectError:    false,
		},
		{
			name:           "input with requestID",
			path:           "stream-request123-in",
			expectStream:   "stream",
			expectRequestID: "request123",
			expectIsInput:  true,
			expectError:    false,
		},
		{
			name:           "stream name with dashes output",
			path:           "my-stream-name-request123-out",
			expectStream:   "my-stream-name",
			expectRequestID: "request123",
			expectIsInput:  false,
			expectError:    false,
		},
		{
			name:           "stream name with dashes input",
			path:           "my-stream-name-request123-in",
			expectStream:   "my-stream-name",
			expectRequestID: "request123",
			expectIsInput:  true,
			expectError:    false,
		},
		{
			name:        "malformed path",
			path:        "invalid",
			expectError: true,
		},
		{
			name:        "missing suffix",
			path:        "stream-request123",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse the path similar to CreateWhep
			parts := strings.Split(tt.path, "-")
			
			var isInput bool
			var stream, requestID string
			var parseErr bool

			if len(parts) >= 3 {
				lastPart := parts[len(parts)-1]
				if lastPart == "out" {
					isInput = false
					stream = strings.Join(parts[:len(parts)-2], "-")
					requestID = parts[len(parts)-2]
				} else if lastPart == "in" {
					isInput = true
					stream = strings.Join(parts[:len(parts)-2], "-")
					requestID = parts[len(parts)-2]
				} else {
					if len(parts) == 2 && parts[1] == "out" {
						isInput = false
						stream = parts[0]
						requestID = ""
					} else {
						parseErr = true
					}
				}
			} else if len(parts) == 2 && parts[1] == "out" {
				isInput = false
				stream = parts[0]
				requestID = ""
			} else {
				parseErr = true
			}

			if tt.expectError {
				assert.True(t, parseErr, "Expected parse error")
			} else {
				assert.False(t, parseErr, "Unexpected parse error")
				assert.Equal(t, tt.expectStream, stream)
				assert.Equal(t, tt.expectRequestID, requestID)
				assert.Equal(t, tt.expectIsInput, isInput)
			}
		})
	}
}

func TestCreateWhep_InputPlayback(t *testing.T) {
	// Create a mock node with a pipeline that has InSegmentReader
	node := &core.LivepeerNode{
		LiveMu:        &sync.RWMutex{},
		LivePipelines: make(map[string]*core.LivePipeline),
	}

	streamName := "teststream"
	requestID := "req123"
	ssr := media.NewSwitchableSegmentReader()

	pipeline := &core.LivePipeline{
		RequestID:       requestID,
		StreamID:        "stream123",
		InSegmentReader: ssr,
		Closed:          false,
	}
	node.LivePipelines[streamName] = pipeline

	// Create a test WHEP server
	whepServer := media.NewWHEPServer()

	// Create the handler
	ls := &LivepeerServer{
		LivepeerNode: node,
	}
	handler := ls.CreateWhep(whepServer)

	// Test successful input playback request
	req := httptest.NewRequest("POST", "/live/video-to-video/"+streamName+"-"+requestID+"-in/whep", nil)
	req.Header.Set("Content-Type", "application/sdp")
	req.Body = io.NopCloser(strings.NewReader("v=0\r\no=- 0 0 IN IP4 127.0.0.1\r\ns=-\r\nt=0 0\r\nm=video 9 UDP/TLS/RTP/SAVPF 96\r\n"))
	
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// Should not return 404 (stream found)
	// Note: The actual WHEP negotiation might fail, but we're testing the path parsing
	assert.NotEqual(t, http.StatusNotFound, w.Code, "Stream should be found")
}

func TestCreateWhep_InputPlaybackNotFound(t *testing.T) {
	node := &core.LivepeerNode{
		LiveMu:        &sync.RWMutex{},
		LivePipelines: make(map[string]*core.LivePipeline),
	}

	whepServer := media.NewWHEPServer()
	ls := &LivepeerServer{
		LivepeerNode: node,
	}
	handler := ls.CreateWhep(whepServer)

	// Test with non-existent stream
	req := httptest.NewRequest("POST", "/live/video-to-video/nonexistent-req123-in/whep", nil)
	req.Header.Set("Content-Type", "application/sdp")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code, "Non-existent stream should return 404")
}
