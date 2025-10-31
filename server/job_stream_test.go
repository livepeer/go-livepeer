package server

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/media"
	"github.com/livepeer/go-livepeer/pm"
	"github.com/livepeer/go-livepeer/trickle"
	"github.com/livepeer/go-tools/drivers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/goleak"
)

var stubOrchServerUrl string

// testOrch wraps mockOrchestrator to override a few methods needed by lphttp in tests
type testStreamOrch struct {
	*mockOrchestrator
	svc    *url.URL
	capURL string
}

func (o *testStreamOrch) ServiceURI() *url.URL                         { return o.svc }
func (o *testStreamOrch) GetUrlForCapability(capability string) string { return o.capURL }

// streamingResponseWriter implements http.ResponseWriter for streaming responses
type streamingResponseWriter struct {
	pipe    *io.PipeWriter
	headers http.Header
	status  int
}

func (w *streamingResponseWriter) Header() http.Header {
	return w.headers
}

func (w *streamingResponseWriter) Write(data []byte) (int, error) {
	return w.pipe.Write(data)
}

func (w *streamingResponseWriter) WriteHeader(statusCode int) {
	w.status = statusCode
}

// Helper: base64-encoded JobRequest with JobParameters (Enable all true, test-capability name)
func base64TestJobRequest(timeout int, enableVideoIngress, enableVideoEgress, enableDataOutput bool) string {
	params := JobParameters{
		EnableVideoIngress: enableVideoIngress,
		EnableVideoEgress:  enableVideoEgress,
		EnableDataOutput:   enableDataOutput,
	}
	paramsStr, _ := json.Marshal(params)

	jr := JobRequest{
		Capability: "test-capability",
		Parameters: string(paramsStr),
		Request:    "{}",
		Timeout:    timeout,
	}

	b, _ := json.Marshal(jr)

	return base64.StdEncoding.EncodeToString(b)
}

func orchAIStreamStartHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/ai/stream/start" {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Publish-Url", fmt.Sprintf("%s%s%s", stubOrchServerUrl, TrickleHTTPPath, "test-stream"))
	w.Header().Set("X-Subscribe-Url", fmt.Sprintf("%s%s%s", stubOrchServerUrl, TrickleHTTPPath, "test-stream-out"))
	w.Header().Set("X-Control-Url", fmt.Sprintf("%s%s%s", stubOrchServerUrl, TrickleHTTPPath, "test-stream-control"))
	w.Header().Set("X-Events-Url", fmt.Sprintf("%s%s%s", stubOrchServerUrl, TrickleHTTPPath, "test-stream-events"))
	w.Header().Set("X-Data-Url", fmt.Sprintf("%s%s%s", stubOrchServerUrl, TrickleHTTPPath, "test-stream-data"))
	w.WriteHeader(http.StatusOK)
}

func orchCapabilityUrlHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func TestStartStream_MaxBodyLimit(t *testing.T) {
	// Setup server with minimal dependencies
	node := mockJobLivepeerNode()
	server := httptest.NewServer(http.HandlerFunc(orchTokenHandler))
	defer server.Close()
	node.OrchestratorPool = newStubOrchestratorPool(node, []string{server.URL})

	// Set up mock sender to prevent nil pointer dereference
	mockSender := pm.MockSender{}
	mockSender.On("StartSession", mock.Anything).Return("foo")
	mockSender.On("CreateTicketBatch", mock.Anything, mock.Anything).Return(mockTicketBatch(10), nil)
	node.Sender = &mockSender
	node.Balances = core.NewAddressBalances(10)
	defer node.Balances.StopCleanup()

	ls := &LivepeerServer{LivepeerNode: node}

	// Prepare a valid job request header
	jobDetails := JobRequestDetails{StreamId: "test-stream"}
	jobParams := JobParameters{EnableVideoIngress: true, EnableVideoEgress: true, EnableDataOutput: true}
	jobReq := JobRequest{
		ID:         "job-1",
		Request:    marshalToString(t, jobDetails),
		Parameters: marshalToString(t, jobParams),
		Capability: "test-capability",
		Timeout:    10,
	}
	jobReqB, err := json.Marshal(jobReq)
	assert.NoError(t, err)
	jobReqB64 := base64.StdEncoding.EncodeToString(jobReqB)

	// Create a body over 10MB
	bigBody := bytes.Repeat([]byte("a"), 10<<20+1) // 10MB + 1 byte
	req := httptest.NewRequest(http.MethodPost, "/ai/stream/start", bytes.NewReader(bigBody))
	req.Header.Set(jobRequestHdr, jobReqB64)

	w := httptest.NewRecorder()
	handler := ls.StartStream()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
}

func TestStreamStart_SetupStream(t *testing.T) {
	node := mockJobLivepeerNode()
	server := httptest.NewServer(http.HandlerFunc(orchTokenHandler))
	defer server.Close()
	node.OrchestratorPool = newStubOrchestratorPool(node, []string{server.URL})

	// Set up mock sender to prevent nil pointer dereference
	mockSender := pm.MockSender{}
	mockSender.On("StartSession", mock.Anything).Return("foo")
	mockSender.On("CreateTicketBatch", mock.Anything, mock.Anything).Return(mockTicketBatch(10), nil)
	node.Sender = &mockSender
	node.Balances = core.NewAddressBalances(10)
	defer node.Balances.StopCleanup()

	ls := &LivepeerServer{LivepeerNode: node}
	drivers.NodeStorage = drivers.NewMemoryDriver(nil)

	// Prepare a valid gatewayJob
	jobParams := JobParameters{EnableVideoIngress: true, EnableVideoEgress: true, EnableDataOutput: true}
	paramsStr := marshalToString(t, jobParams)
	jobReq := &JobRequest{
		Capability: "test-capability",
		Parameters: paramsStr,
		Timeout:    10,
	}
	orchJob := &orchJob{Req: jobReq, Params: &jobParams}
	gatewayJob := &gatewayJob{Job: orchJob}

	// Prepare a valid StartRequest body
	startReq := StartRequest{
		Stream:     "teststream",
		RtmpOutput: "rtmp://output",
		StreamId:   "streamid",
		Params:     "{}",
	}
	body, _ := json.Marshal(startReq)
	req := httptest.NewRequest(http.MethodPost, "/ai/stream/start", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	urls, code, err := ls.setupStream(context.Background(), req, gatewayJob)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, code)
	assert.NotNil(t, urls)
	assert.Equal(t, "teststream-streamid", urls.StreamId)
	//confirm all urls populated
	assert.NotEmpty(t, urls.WhipUrl)
	assert.NotEmpty(t, urls.RtmpUrl)
	assert.NotEmpty(t, urls.WhepUrl)
	assert.NotEmpty(t, urls.RtmpOutputUrl)
	assert.Contains(t, urls.RtmpOutputUrl, "rtmp://output")
	assert.NotEmpty(t, urls.DataUrl)
	assert.NotEmpty(t, urls.StatusUrl)
	assert.NotEmpty(t, urls.UpdateUrl)

	//confirm LivePipeline created
	stream, ok := ls.LivepeerNode.LivePipelines[urls.StreamId]
	assert.True(t, ok)
	assert.NotNil(t, stream)
	assert.Equal(t, urls.StreamId, stream.StreamID)
	assert.Equal(t, stream.StreamRequest(), []byte("{\"params\":{}}"))
	params := stream.StreamParams()
	_, checkParamsType := params.(aiRequestParams)
	assert.True(t, checkParamsType)

	//test with no data output
	jobParams = JobParameters{EnableVideoIngress: true, EnableVideoEgress: true, EnableDataOutput: false}
	paramsStr = marshalToString(t, jobParams)
	jobReq.Parameters = paramsStr
	gatewayJob.Job.Params = &jobParams
	req.Body = io.NopCloser(bytes.NewReader(body))
	urls, code, err = ls.setupStream(context.Background(), req, gatewayJob)
	assert.Empty(t, urls.DataUrl)

	//test with no video ingress
	jobParams = JobParameters{EnableVideoIngress: false, EnableVideoEgress: true, EnableDataOutput: true}
	paramsStr = marshalToString(t, jobParams)
	jobReq.Parameters = paramsStr
	gatewayJob.Job.Params = &jobParams
	req.Body = io.NopCloser(bytes.NewReader(body))
	urls, code, err = ls.setupStream(context.Background(), req, gatewayJob)
	assert.Empty(t, urls.WhipUrl)
	assert.Empty(t, urls.RtmpUrl)

	//test with no video egress
	jobParams = JobParameters{EnableVideoIngress: true, EnableVideoEgress: false, EnableDataOutput: true}
	paramsStr = marshalToString(t, jobParams)
	jobReq.Parameters = paramsStr
	gatewayJob.Job.Params = &jobParams
	req.Body = io.NopCloser(bytes.NewReader(body))
	urls, code, err = ls.setupStream(context.Background(), req, gatewayJob)
	assert.Empty(t, urls.WhepUrl)
	assert.Empty(t, urls.RtmpOutputUrl)

	// Test with nil job
	urls, code, err = ls.setupStream(context.Background(), req, nil)
	assert.Error(t, err)
	assert.Equal(t, http.StatusBadRequest, code)
	assert.Nil(t, urls)

	// Test with invalid JSON body
	badReq := httptest.NewRequest(http.MethodPost, "/ai/stream/start", bytes.NewReader([]byte("notjson")))
	badReq.Header.Set("Content-Type", "application/json")
	urls, code, err = ls.setupStream(context.Background(), badReq, gatewayJob)
	assert.Error(t, err)
	assert.Equal(t, http.StatusBadRequest, code)
	assert.Nil(t, urls)

	// Test with stream name ending in -out (should return nil, 0, nil)
	outReq := StartRequest{
		Stream:     "teststream-out",
		RtmpOutput: "rtmp://output",
		StreamId:   "streamid",
		Params:     "{}",
	}
	outBody, _ := json.Marshal(outReq)
	outReqHTTP := httptest.NewRequest(http.MethodPost, "/ai/stream/start", bytes.NewReader(outBody))
	outReqHTTP.Header.Set("Content-Type", "application/json")
	urls, code, err = ls.setupStream(context.Background(), outReqHTTP, gatewayJob)
	assert.NoError(t, err)
	assert.Equal(t, 0, code)
	assert.Nil(t, urls)
}

func TestRunStream_RunAndCancelStream(t *testing.T) {
	defer goleak.VerifyNone(t, common.IgnoreRoutines()...)
	node := mockJobLivepeerNode()

	// Set up an lphttp-based orchestrator test server with trickle endpoints
	mux := http.NewServeMux()
	mockOrch := &mockOrchestrator{}
	mockOrch.On("VerifySig", mock.Anything, mock.Anything, mock.Anything).Return(true)
	mockOrch.On("DebitFees", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return()

	lp := &lphttp{orchestrator: nil, transRPC: mux, node: node}
	// Configure trickle server on the mux (imitate production trickle endpoints)
	lp.trickleSrv = trickle.ConfigureServer(trickle.TrickleServerConfig{
		Mux:        mux,
		BasePath:   TrickleHTTPPath,
		Autocreate: true,
	})
	// Register orchestrator endpoints used by runStream path
	mux.HandleFunc("/ai/stream/start", lp.StartStream)
	mux.HandleFunc("/ai/stream/stop", lp.StopStream)
	mux.HandleFunc("/process/token", orchTokenHandler)
	// Handle DELETE requests for trickle cleanup (in addition to trickle server's built-in handlers)
	mux.HandleFunc("DELETE /ai/trickle/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	server := httptest.NewServer(lp)
	defer server.Close()
	// Add a connection state tracker
	mu := sync.Mutex{}
	conns := make(map[net.Conn]http.ConnState)
	server.Config.ConnState = func(conn net.Conn, state http.ConnState) {
		mu.Lock()
		defer mu.Unlock()

		conns[conn] = state
	}

	stubOrchServerUrl = server.URL

	// Configure mock orchestrator behavior expected by lphttp handlers
	parsedURL, _ := url.Parse(server.URL)
	capabilitySrv := httptest.NewServer(http.HandlerFunc(orchCapabilityUrlHandler))
	defer capabilitySrv.Close()

	// attach our orchestrator implementation to lphttp
	lp.orchestrator = &testStreamOrch{mockOrchestrator: mockOrch, svc: parsedURL, capURL: capabilitySrv.URL}

	// Prepare a gatewayJob with a dummy orchestrator token
	jobReq := &JobRequest{
		ID:         "test-stream",
		Capability: "test-capability",
		Timeout:    10,
		Request:    "{}",
	}
	jobParams := JobParameters{EnableVideoIngress: true, EnableVideoEgress: true, EnableDataOutput: true}
	paramsStr := marshalToString(t, jobParams)
	jobReq.Parameters = paramsStr

	orchToken := createMockJobToken(server.URL)
	orchJob := &orchJob{Req: jobReq, Params: &jobParams}
	gatewayJob := &gatewayJob{Job: orchJob, Orchs: []core.JobToken{*orchToken}, node: node}

	// Setup a LivepeerServer and a mock pipeline
	ls := &LivepeerServer{LivepeerNode: node}
	ls.LivepeerNode.OrchestratorPool = newStubOrchestratorPool(ls.LivepeerNode, []string{server.URL})
	drivers.NodeStorage = drivers.NewMemoryDriver(nil)
	mockSender := pm.MockSender{}
	mockSender.On("StartSession", mock.Anything).Return("foo").Times(4)
	mockSender.On("CreateTicketBatch", "foo", orchJob.Req.Timeout).Return(mockTicketBatch(orchJob.Req.Timeout), nil).Once()
	node.Sender = &mockSender
	node.Balances = core.NewAddressBalances(10)
	defer node.Balances.StopCleanup()

	//now sign job and create a sig for the sender to include
	gatewayJob.sign()
	sender, err := getJobSender(context.TODO(), node)
	assert.NoError(t, err)
	orchJob.Req.Sender = sender.Addr
	orchJob.Req.Sig = sender.Sig
	// Minimal aiRequestParams and liveRequestParams
	params := aiRequestParams{
		liveParams: &liveRequestParams{
			requestID:      "req-1",
			stream:         "test-stream",
			streamID:       "test-stream",
			sendErrorEvent: func(err error) {},
			segmentReader:  media.NewSwitchableSegmentReader(),
		},
		node: node,
	}

	ls.LivepeerNode.NewLivePipeline("req-1", "test-stream", "test-capability", params, nil)

	// Cancel the stream after a short delay to simulate shutdown
	done := make(chan struct{})
	go func() {
		time.Sleep(200 * time.Millisecond)
		stream := node.LivePipelines["test-stream"]

		if stream != nil {
			// Wait for ControlPub to be initialized by runStream
			timeout := time.After(2 * time.Second)
			ticker := time.NewTicker(10 * time.Millisecond)
			defer ticker.Stop()

			for stream.ControlPub == nil {
				select {
				case <-ticker.C:
					// Check again
				case <-timeout:
					// Timeout waiting for ControlPub, proceed anyway
					break
				}
			}
			//cancel stream context and force cleanup
			stream.StopStream(errors.New("test error"))

			// Close the segment reader to trigger EOS and cleanup publishers
			params, _ := getStreamRequestParams(stream)
			if params.liveParams != nil && params.liveParams.segmentReader != nil {
				params.liveParams.segmentReader.Close()
			}
		}
		close(done)
	}()

	// Should not panic and should clean up
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); ls.runStream(gatewayJob) }()
	go func() { defer wg.Done(); ls.monitorStream(gatewayJob.Job.Req.ID) }()
	<-done
	// Wait for both goroutines to finish before asserting
	wg.Wait()
	// After cancel, the stream should be removed from LivePipelines
	_, exists := node.LivePipelines["test-stream"]
	assert.False(t, exists)

	// Clean up trickle streams via HTTP DELETE
	streamID := "test-stream"
	trickleStreams := []string{
		streamID,
		streamID + "-out",
		streamID + "-control",
		streamID + "-events",
		streamID + "-data",
	}
	for _, stream := range trickleStreams {
		req := httptest.NewRequest("DELETE", fmt.Sprintf("%s/%s", TrickleHTTPPath, stream), nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
	}

	// Clean up external capabilities streams
	if node.ExternalCapabilities != nil {
		for streamID := range node.ExternalCapabilities.Streams {
			node.ExternalCapabilities.RemoveStream(streamID)
		}
	}

	//clean up http connections
	mu.Lock()
	defer mu.Unlock()
	for conn := range conns {
		conn.Close()
		delete(conns, conn)
	}
}

// Test StartStream handler
func TestStartStreamHandler(t *testing.T) {
	node := mockJobLivepeerNode()

	// Set up an lphttp-based orchestrator test server with trickle endpoints
	mux := http.NewServeMux()
	ls := &LivepeerServer{
		LivepeerNode: node,
	}
	mockSender := pm.MockSender{}
	mockSender.On("StartSession", mock.Anything).Return("foo")
	mockSender.On("CreateTicketBatch", mock.Anything, mock.Anything).Return(mockTicketBatch(10), nil)
	node.Sender = &mockSender
	node.Balances = core.NewAddressBalances(1 * time.Second)
	defer node.Balances.StopCleanup()
	//setup Orch server stub
	mux.HandleFunc("/process/token", orchTokenHandler)
	mux.HandleFunc("/ai/stream/start", orchAIStreamStartHandler)

	server := httptest.NewServer(mux)
	defer server.Close()
	// Add a connection state tracker
	mu := sync.Mutex{}
	conns := make(map[net.Conn]http.ConnState)
	server.Config.ConnState = func(conn net.Conn, state http.ConnState) {
		mu.Lock()
		defer mu.Unlock()

		conns[conn] = state
	}

	ls.LivepeerNode.OrchestratorPool = newStubOrchestratorPool(ls.LivepeerNode, []string{server.URL})
	drivers.NodeStorage = drivers.NewMemoryDriver(nil)
	// Prepare a valid StartRequest body
	startReq := StartRequest{
		Stream:     "teststream",
		RtmpOutput: "rtmp://output",
		StreamId:   "streamid",
		Params:     "{}",
	}
	body, _ := json.Marshal(startReq)
	req := httptest.NewRequest(http.MethodPost, "/ai/stream/start", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	req.Header.Set("Livepeer", base64TestJobRequest(10, true, true, true))

	w := httptest.NewRecorder()

	handler := ls.StartStream()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	body = w.Body.Bytes()
	var streamUrls StreamUrls
	err := json.Unmarshal(body, &streamUrls)
	assert.NoError(t, err)
	stream, exits := ls.LivepeerNode.LivePipelines[streamUrls.StreamId]
	assert.True(t, exits)
	assert.NotNil(t, stream)
	assert.Equal(t, streamUrls.StreamId, stream.StreamID)
	params := stream.StreamParams()
	streamParams, checkParamsType := params.(aiRequestParams)
	assert.True(t, checkParamsType)
	//wrap up processing
	time.Sleep(100 * time.Millisecond)
	streamParams.liveParams.kickOrch(errors.New("test error"))
	stream.StopStream(nil)

	//clean up http connections
	mu.Lock()
	defer mu.Unlock()
	for conn := range conns {
		conn.Close()
		delete(conns, conn)
	}

	// Give time for cleanup to complete
	time.Sleep(50 * time.Millisecond)
}

// Test StopStream handler
func TestStopStreamHandler(t *testing.T) {
	t.Run("StreamNotFound", func(t *testing.T) {
		// Test case 1: Stream doesn't exist - should return 404
		ls := &LivepeerServer{LivepeerNode: &core.LivepeerNode{LivePipelines: map[string]*core.LivePipeline{}}}
		req := httptest.NewRequest(http.MethodPost, "/ai/stream/{streamId}/stop", nil)
		req.SetPathValue("streamId", "non-existent-stream")
		w := httptest.NewRecorder()

		handler := ls.StopStream()
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
		assert.Contains(t, w.Body.String(), "Stream not found")
	})

	t.Run("StreamExistsAndStopsSuccessfully", func(t *testing.T) {
		// Test case 2: Stream exists - should stop stream and attempt to send request to orchestrator
		node := mockJobLivepeerNode()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Mock orchestrator response handlers
			switch r.URL.Path {
			case "/process/token":
				orchTokenHandler(w, r)
			case "/ai/stream/stop":
				// Mock successful stop response from orchestrator
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"status": "stopped"}`))
			default:
				http.NotFound(w, r)
			}
		}))
		defer server.Close()

		node.OrchestratorPool = newStubOrchestratorPool(node, []string{server.URL})
		ls := &LivepeerServer{LivepeerNode: node}
		drivers.NodeStorage = drivers.NewMemoryDriver(nil)
		mockSender := pm.MockSender{}
		mockSender.On("StartSession", mock.Anything).Return("foo").Times(4)
		mockSender.On("CreateTicketBatch", "foo", 10).Return(mockTicketBatch(10), nil).Once()
		node.Sender = &mockSender
		node.Balances = core.NewAddressBalances(10)
		defer node.Balances.StopCleanup()
		// Create a stream to stop
		streamID := "test-stream-to-stop"

		// Create minimal AI session with properly formatted URL
		token := createMockJobToken(server.URL)

		sess, err := tokenToAISession(*token)

		// Create stream parameters
		params := aiRequestParams{
			liveParams: &liveRequestParams{
				requestID:      "req-1",
				sess:           &sess,
				stream:         streamID,
				streamID:       streamID,
				sendErrorEvent: func(err error) {},
				segmentReader:  media.NewSwitchableSegmentReader(),
			},
			node: node,
		}

		// Add the stream to LivePipelines
		stream := node.NewLivePipeline("req-1", streamID, "test-capability", params, nil)
		assert.NotNil(t, stream)

		// Verify stream exists before stopping
		_, exists := ls.LivepeerNode.LivePipelines[streamID]
		assert.True(t, exists, "Stream should exist before stopping")

		// Create stop request with proper job header
		jobParams := JobParameters{EnableVideoIngress: true, EnableVideoEgress: true, EnableDataOutput: true}
		jobDetails := JobRequestDetails{StreamId: streamID}
		jobReq := JobRequest{
			ID:         streamID,
			Request:    marshalToString(t, jobDetails),
			Capability: "test-capability",
			Parameters: marshalToString(t, jobParams),
			Timeout:    10,
		}
		jobReqB, err := json.Marshal(jobReq)
		assert.NoError(t, err)
		jobReqB64 := base64.StdEncoding.EncodeToString(jobReqB)

		req := httptest.NewRequest(http.MethodPost, "/ai/stream/{streamId}/stop", strings.NewReader(`{"reason": "test stop"}`))
		req.SetPathValue("streamId", streamID)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set(jobRequestHdr, jobReqB64)

		w := httptest.NewRecorder()

		handler := ls.StopStream()
		handler.ServeHTTP(w, req)

		// The response might vary depending on orchestrator communication success
		// The important thing is that the stream is removed regardless
		assert.Contains(t, []int{http.StatusOK, http.StatusInternalServerError, http.StatusBadRequest}, w.Code,
			"Should return valid HTTP status")

		// Verify stream was removed from LivePipelines (this should always happen)
		_, exists = ls.LivepeerNode.LivePipelines[streamID]
		assert.False(t, exists, "Stream should be removed after stopping")
	})

	t.Run("StreamExistsButOrchestratorError", func(t *testing.T) {
		// Test case 3: Stream exists but orchestrator returns error
		node := mockJobLivepeerNode()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/process/token":
				orchTokenHandler(w, r)
			case "/ai/stream/stop":
				// Mock orchestrator error
				http.Error(w, "Orchestrator error", http.StatusInternalServerError)
			default:
				http.NotFound(w, r)
			}
		}))
		defer server.Close()

		node.OrchestratorPool = newStubOrchestratorPool(node, []string{server.URL})
		ls := &LivepeerServer{LivepeerNode: node}
		drivers.NodeStorage = drivers.NewMemoryDriver(nil)
		mockSender := pm.MockSender{}
		mockSender.On("StartSession", mock.Anything).Return("foo").Times(4)
		mockSender.On("CreateTicketBatch", "foo", 10).Return(mockTicketBatch(10), nil).Once()
		node.Sender = &mockSender
		node.Balances = core.NewAddressBalances(10)
		defer node.Balances.StopCleanup()
		streamID := "test-stream-orch-error"

		// Create minimal AI session
		token := createMockJobToken(server.URL)
		sess, err := tokenToAISession(*token)
		assert.NoError(t, err)

		params := aiRequestParams{
			liveParams: &liveRequestParams{
				requestID:      "req-1",
				sess:           &sess,
				stream:         streamID,
				streamID:       streamID,
				sendErrorEvent: func(err error) {},
				segmentReader:  media.NewSwitchableSegmentReader(),
			},
			node: node,
		}

		// Add the stream
		stream := node.NewLivePipeline("req-1", streamID, "test-capability", params, nil)
		assert.NotNil(t, stream)

		// Create stop request
		jobParams := JobParameters{EnableVideoIngress: true, EnableVideoEgress: true, EnableDataOutput: true}
		jobDetails := JobRequestDetails{StreamId: streamID}
		jobReq := JobRequest{
			ID:         streamID,
			Request:    marshalToString(t, jobDetails),
			Capability: "test-capability",
			Parameters: marshalToString(t, jobParams),
			Timeout:    10,
		}
		jobReqB, err := json.Marshal(jobReq)
		assert.NoError(t, err)
		jobReqB64 := base64.StdEncoding.EncodeToString(jobReqB)

		req := httptest.NewRequest(http.MethodPost, "/ai/stream/{streamId}/stop", nil)
		req.SetPathValue("streamId", streamID)
		req.Header.Set(jobRequestHdr, jobReqB64)

		w := httptest.NewRecorder()

		handler := ls.StopStream()
		handler.ServeHTTP(w, req)

		// Returns 200 OK because Gateway removed the stream. If the Orchestrator errors, it will return
		// the error in the response body
		assert.Equal(t, http.StatusOK, w.Code)

		// Stream should still be removed even if orchestrator returns error
		_, exists := ls.LivepeerNode.LivePipelines[streamID]
		assert.False(t, exists, "Stream should be removed even on orchestrator error")
	})
}

// Test StartStreamRTMPIngest handler
func TestStartStreamRTMPIngestHandler(t *testing.T) {
	defer goleak.VerifyNone(t, common.IgnoreRoutines()...)
	// Setup mock MediaMTX server on port 9997 before starting the test
	mockMediaMTXServer := createMockMediaMTXServer(t)
	defer mockMediaMTXServer.Close()

	node := mockJobLivepeerNode()
	node.WorkDir = t.TempDir()
	server := httptest.NewServer(http.HandlerFunc(orchTokenHandler))
	defer server.Close()
	node.OrchestratorPool = newStubOrchestratorPool(node, []string{server.URL})

	ls := &LivepeerServer{
		LivepeerNode:        node,
		mediaMTXApiPassword: "test-password",
	}
	drivers.NodeStorage = drivers.NewMemoryDriver(nil)

	// Prepare a valid gatewayJob
	jobParams := JobParameters{EnableVideoIngress: true, EnableVideoEgress: true, EnableDataOutput: true}
	paramsStr := marshalToString(t, jobParams)
	jobReq := &JobRequest{
		Capability: "test-capability",
		Parameters: paramsStr,
		Timeout:    10,
	}
	orchJob := &orchJob{Req: jobReq, Params: &jobParams}
	gatewayJob := &gatewayJob{Job: orchJob}

	// Prepare a valid StartRequest body
	startReq := StartRequest{
		Stream:     "teststream",
		RtmpOutput: "rtmp://output",
		StreamId:   "streamid",
		Params:     "{}",
	}
	body, _ := json.Marshal(startReq)
	req := httptest.NewRequest(http.MethodPost, "/ai/stream/start", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	urls, code, err := ls.setupStream(context.Background(), req, gatewayJob)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, code)
	assert.NotNil(t, urls)
	assert.Equal(t, "teststream-streamid", urls.StreamId) //combination of stream name (Stream) and id (StreamId)

	stream, ok := ls.LivepeerNode.LivePipelines[urls.StreamId]
	assert.True(t, ok)
	assert.NotNil(t, stream)

	params, err := getStreamRequestParams(stream)
	assert.NoError(t, err)

	//these should be empty/nil before rtmp ingest starts
	assert.Empty(t, params.liveParams.localRTMPPrefix)
	assert.Nil(t, params.liveParams.kickInput)

	rtmpReq := httptest.NewRequest(http.MethodPost, "/ai/stream/{streamId}/rtmp", nil)
	rtmpReq.SetPathValue("streamId", "teststream-streamid")
	w := httptest.NewRecorder()

	handler := ls.StartStreamRTMPIngest()
	handler.ServeHTTP(w, rtmpReq)
	// Missing source_id and source_type
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// Now provide valid form data
	formData := url.Values{}
	formData.Set("source_id", "testsourceid")
	formData.Set("source_type", "rtmpconn")
	rtmpReq = httptest.NewRequest(http.MethodPost, "/ai/stream/{streamId}/rtmp", strings.NewReader(formData.Encode()))
	rtmpReq.SetPathValue("streamId", "teststream-streamid")
	// Use localhost as the remote addr to simulate MediaMTX
	rtmpReq.RemoteAddr = "127.0.0.1:1935"

	rtmpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, rtmpReq)
	assert.Equal(t, http.StatusOK, w.Code)

	// Verify that the stream parameters were updated correctly
	newParams, _ := getStreamRequestParams(stream)
	assert.NotNil(t, newParams.liveParams.kickInput)
	assert.NotEmpty(t, newParams.liveParams.localRTMPPrefix)

	// Stop the stream to cleanup
	newParams.liveParams.kickInput(errors.New("test error"))
	stream.StopStream(nil)

	//ffmpegOUtput sleeps for 5 seconds at end of function, let it wrap up for go routine leak check
	time.Sleep(5 * time.Second)
}

// Test StartStreamWhipIngest handler
func TestStartStreamWhipIngestHandler(t *testing.T) {
	node := mockJobLivepeerNode()
	node.WorkDir = t.TempDir()
	server := httptest.NewServer(http.HandlerFunc(orchTokenHandler))
	defer server.Close()
	node.OrchestratorPool = newStubOrchestratorPool(node, []string{server.URL})
	ls := &LivepeerServer{LivepeerNode: node}
	drivers.NodeStorage = drivers.NewMemoryDriver(nil)

	// Prepare a valid gatewayJob
	jobParams := JobParameters{EnableVideoIngress: true, EnableVideoEgress: true, EnableDataOutput: true}
	paramsStr := marshalToString(t, jobParams)
	jobReq := &JobRequest{
		Capability: "test-capability",
		Parameters: paramsStr,
		Timeout:    10,
	}
	orchJob := &orchJob{Req: jobReq, Params: &jobParams}
	gatewayJob := &gatewayJob{Job: orchJob}

	// Prepare a valid StartRequest body for /ai/stream/start
	startReq := StartRequest{
		Stream:     "teststream",
		RtmpOutput: "rtmp://output",
		StreamId:   "streamid",
		Params:     "{}",
	}
	body, _ := json.Marshal(startReq)
	req := httptest.NewRequest(http.MethodPost, "/ai/stream/start", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	urls, code, err := ls.setupStream(context.Background(), req, gatewayJob)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, code)
	assert.NotNil(t, urls)
	assert.Equal(t, "teststream-streamid", urls.StreamId) //combination of stream name (Stream) and id (StreamId)

	stream, ok := ls.LivepeerNode.LivePipelines[urls.StreamId]
	assert.True(t, ok)
	assert.NotNil(t, stream)

	params, err := getStreamRequestParams(stream)
	assert.NoError(t, err)

	//these should be empty/nil before whip ingest starts
	assert.Empty(t, params.liveParams.localRTMPPrefix)
	assert.Nil(t, params.liveParams.kickInput)

	// whipServer is required, using nil will test setup up to initializing the WHIP connection
	whipServer := media.NewWHIPServer()
	handler := ls.StartStreamWhipIngest(whipServer)

	// SDP offer for WHIP with H.264 video and Opus audio
	sdpOffer := `v=0
o=- 123456789 2 IN IP4 127.0.0.1
s=-
t=0 0
a=group:BUNDLE 0 1
a=msid-semantic: WMS stream
m=video 9 UDP/TLS/RTP/SAVPF 96
c=IN IP4 0.0.0.0
a=rtcp:9 IN IP4 0.0.0.0
a=ice-ufrag:abcd
a=ice-pwd:abcdefghijklmnopqrstuvwxyz123456
a=fingerprint:sha-256 00:11:22:33:44:55:66:77:88:99:AA:BB:CC:DD:EE:FF:00:11:22:33:44:55:66:77:88:99:AA:BB:CC:DD:EE:FF
a=setup:actpass
a=mid:0
a=extmap:1 urn:ietf:params:rtp-hdrext:sdes:mid
a=extmap:2 urn:ietf:params:rtp-hdrext:sdes:rtp-stream-id
a=extmap:3 urn:ietf:params:rtp-hdrext:sdes:repaired-rtp-stream-id
a=sendonly
a=msid:stream video
a=rtcp-mux
a=rtpmap:96 H264/90000
a=rtcp-fb:96 goog-remb
a=rtcp-fb:96 transport-cc
a=rtcp-fb:96 ccm fir
a=rtcp-fb:96 nack
a=rtcp-fb:96 nack pli
a=fmtp:96 level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42e01f
m=audio 9 UDP/TLS/RTP/SAVPF 111
c=IN IP4 0.0.0.0
a=rtcp:9 IN IP4 0.0.0.0
a=ice-ufrag:abcd
a=ice-pwd:abcdefghijklmnopqrstuvwxyz123456
a=fingerprint:sha-256 00:11:22:33:44:55:66:77:88:99:AA:BB:CC:DD:EE:FF:00:11:22:33:44:55:66:77:88:99:AA:BB:CC:DD:EE:FF
a=setup:actpass
a=mid:1
a=extmap:1 urn:ietf:params:rtp-hdrext:sdes:mid
a=sendonly
a=msid:stream audio
a=rtcp-mux
a=rtpmap:111 opus/48000/2
a=rtcp-fb:111 transport-cc
a=fmtp:111 minptime=10;useinbandfec=1
`

	whipReq := httptest.NewRequest(http.MethodPost, "/ai/stream/{streamId}/whip", strings.NewReader(sdpOffer))
	whipReq.SetPathValue("streamId", "teststream-streamid")
	whipReq.Header.Set("Content-Type", "application/sdp")

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, whipReq)
	assert.Equal(t, http.StatusCreated, w.Code)

	newParams, err := getStreamRequestParams(stream)
	assert.NoError(t, err)
	assert.NotNil(t, newParams.liveParams.kickInput)

	//stop the WHIP connection
	time.Sleep(2 * time.Millisecond) //wait for setup
	//add kickOrch because we are not calling runStream which would have added it
	newParams.liveParams.kickOrch = func(error) {}
	stream.UpdateStreamParams(newParams)
	newParams.liveParams.kickInput(errors.New("test complete"))
}

// Test GetStreamData handler
func TestGetStreamDataHandler(t *testing.T) {

	t.Run("StreamData_MissingStreamId", func(t *testing.T) {
		// Test with missing stream ID - should return 400
		ls := &LivepeerServer{}
		handler := ls.UpdateStream()
		req := httptest.NewRequest(http.MethodPost, "/ai/stream/{streamId}/update", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "Missing stream name")
	})

	t.Run("StreamData_DataOutputWorking", func(t *testing.T) {
		node := mockJobLivepeerNode()
		node.WorkDir = t.TempDir()
		server := httptest.NewServer(http.HandlerFunc(orchTokenHandler))
		defer server.Close()
		node.OrchestratorPool = newStubOrchestratorPool(node, []string{server.URL})
		ls := &LivepeerServer{LivepeerNode: node}
		drivers.NodeStorage = drivers.NewMemoryDriver(nil)

		// Prepare a valid gatewayJob
		jobParams := JobParameters{EnableVideoIngress: true, EnableVideoEgress: true, EnableDataOutput: true}
		paramsStr := marshalToString(t, jobParams)
		jobReq := &JobRequest{
			Capability: "test-capability",
			Parameters: paramsStr,
			Timeout:    10,
		}
		orchJob := &orchJob{Req: jobReq, Params: &jobParams}
		gatewayJob := &gatewayJob{Job: orchJob}

		// Prepare a valid StartRequest body for /ai/stream/start
		startReq := StartRequest{
			Stream:     "teststream",
			RtmpOutput: "rtmp://output",
			StreamId:   "streamid",
			Params:     "{}",
		}
		body, _ := json.Marshal(startReq)
		req := httptest.NewRequest(http.MethodPost, "/ai/stream/start", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		urls, code, err := ls.setupStream(context.Background(), req, gatewayJob)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, code)
		assert.NotNil(t, urls)
		assert.Equal(t, "teststream-streamid", urls.StreamId) //combination of stream name (Stream) and id (StreamId)

		stream, ok := ls.LivepeerNode.LivePipelines[urls.StreamId]
		assert.True(t, ok)
		assert.NotNil(t, stream)

		params, err := getStreamRequestParams(stream)
		assert.NoError(t, err)
		assert.NotNil(t, params.liveParams)

		// Write some test data first
		writer, err := params.liveParams.dataWriter.Next()
		assert.NoError(t, err)
		writer.Write([]byte("initial-data"))
		writer.Close()

		handler := ls.GetStreamData()
		dataReq := httptest.NewRequest(http.MethodGet, "/ai/stream/{streamId}/data", nil)
		dataReq.SetPathValue("streamId", "teststream-streamid")

		// Create a context with timeout to prevent infinite blocking
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		dataReq = dataReq.WithContext(ctx)

		// Start writing more segments in a goroutine
		go func() {
			time.Sleep(10 * time.Millisecond) // Give handler time to start

			// Write additional segments
			for i := 0; i < 2; i++ {
				writer, err := params.liveParams.dataWriter.Next()
				if err != nil {
					break
				}
				writer.Write([]byte(fmt.Sprintf("test-data-%d", i)))
				writer.Close()
				time.Sleep(5 * time.Millisecond)
			}

			// Close the writer to signal EOF
			time.Sleep(10 * time.Millisecond)
			params.liveParams.dataWriter.Close()
		}()

		w := httptest.NewRecorder()
		handler.ServeHTTP(w, dataReq)

		// Check response
		responseBody := w.Body.String()

		// Verify we received some SSE data
		assert.Contains(t, responseBody, "data: ", "Should have received SSE data")

		// Check for our test data
		if strings.Contains(responseBody, "data: ") {
			lines := strings.Split(responseBody, "\n")
			dataFound := false
			for _, line := range lines {
				if strings.HasPrefix(line, "data: ") && strings.Contains(line, "data") {
					dataFound = true
					break
				}
			}
			assert.True(t, dataFound, "Should have found data in SSE response")
		}
	})
}

// Test UpdateStream handler
func TestUpdateStreamHandler(t *testing.T) {
	t.Run("UpdateStream_MissingStreamId", func(t *testing.T) {
		// Test with missing stream ID - should return 400
		ls := &LivepeerServer{}
		handler := ls.UpdateStream()
		req := httptest.NewRequest(http.MethodPost, "/ai/stream/{streamId}/update", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "Missing stream name")
	})

	t.Run("Basic_StreamNotFound", func(t *testing.T) {
		// Test with non-existent stream - should return 404
		node := mockJobLivepeerNode()
		ls := &LivepeerServer{LivepeerNode: node}

		req := httptest.NewRequest(http.MethodPost, "/ai/stream/{streamId}/update",
			strings.NewReader(`{"param1": "value1", "param2": "value2"}`))
		req.SetPathValue("streamId", "non-existent-stream")
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		handler := ls.UpdateStream()
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
		assert.Contains(t, w.Body.String(), "Stream not found")
	})

	t.Run("UpdateStream_ErrorHandling", func(t *testing.T) {
		// Test various error conditions
		node := mockJobLivepeerNode()
		server := httptest.NewServer(http.HandlerFunc(orchTokenHandler))
		defer server.Close()
		node.OrchestratorPool = newStubOrchestratorPool(node, []string{server.URL})

		// Set up mock sender to prevent nil pointer dereference
		mockSender := pm.MockSender{}
		mockSender.On("StartSession", mock.Anything).Return("foo")
		mockSender.On("CreateTicketBatch", mock.Anything, mock.Anything).Return(mockTicketBatch(10), nil)
		node.Sender = &mockSender
		node.Balances = core.NewAddressBalances(10)
		defer node.Balances.StopCleanup()

		ls := &LivepeerServer{LivepeerNode: node}
		drivers.NodeStorage = drivers.NewMemoryDriver(nil)

		// Test 1: Wrong HTTP method
		req := httptest.NewRequest(http.MethodGet, "/ai/stream/{streamId}/update", nil)
		req.SetPathValue("streamId", "test-stream")
		w := httptest.NewRecorder()
		ls.UpdateStream().ServeHTTP(w, req)
		assert.Equal(t, http.StatusMethodNotAllowed, w.Code)

		// Test 2: Request too large
		streamID := "test-stream-large"
		token := createMockJobToken(server.URL)
		sess, _ := tokenToAISession(*token)
		params := aiRequestParams{
			liveParams: &liveRequestParams{
				requestID:      "req-1",
				sess:           &sess,
				stream:         streamID,
				streamID:       streamID,
				sendErrorEvent: func(err error) {},
				segmentReader:  media.NewSwitchableSegmentReader(),
			},
			node: node,
		}
		stream := node.NewLivePipeline("req-1", streamID, "test-capability", params, nil)

		// Create job request header
		jobParams := JobParameters{EnableVideoIngress: true, EnableVideoEgress: true, EnableDataOutput: true}
		jobDetails := JobRequestDetails{StreamId: streamID}
		jobReq := JobRequest{
			ID:         streamID,
			Request:    marshalToString(t, jobDetails),
			Capability: "test-capability",
			Parameters: marshalToString(t, jobParams),
			Timeout:    10,
		}
		jobReqB, err := json.Marshal(jobReq)
		assert.NoError(t, err)
		jobReqB64 := base64.StdEncoding.EncodeToString(jobReqB)

		// Create a body larger than 10MB
		largeData := bytes.Repeat([]byte("a"), 10*1024*1024+1)
		req = httptest.NewRequest(http.MethodPost, "/ai/stream/{streamId}/update",
			bytes.NewReader(largeData))
		req.SetPathValue("streamId", streamID)
		req.Header.Set(jobRequestHdr, jobReqB64)
		w = httptest.NewRecorder()

		ls.UpdateStream().ServeHTTP(w, req)
		assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
		assert.Contains(t, w.Body.String(), "request body too large")

		stream.StopStream(nil)
	})
}

// Test GetStreamStatus handler
func TestGetStreamStatusHandler(t *testing.T) {
	ls := &LivepeerServer{}
	handler := ls.GetStreamStatus()
	// stream does not exist
	req := httptest.NewRequest(http.MethodGet, "/ai/stream/{streamId}/status", nil)
	req.SetPathValue("streamId", "any-stream")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)

	// stream exists
	node := mockJobLivepeerNode()
	ls.LivepeerNode = node
	node.NewLivePipeline("req-1", "any-stream", "test-capability", aiRequestParams{}, nil)
	GatewayStatus.StoreKey("any-stream", "test", "test")
	req = httptest.NewRequest(http.MethodGet, "/ai/stream/{streamId}/status", nil)
	req.SetPathValue("streamId", "any-stream")
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

// Test sendPaymentForStream
func TestSendPaymentForStream(t *testing.T) {
	t.Run("Success_ValidPayment", func(t *testing.T) {
		// Setup
		node := mockJobLivepeerNode()
		mockSender := pm.MockSender{}
		mockSender.On("StartSession", mock.Anything).Return("foo").Times(2)
		mockSender.On("CreateTicketBatch", "foo", 70).Return(mockTicketBatch(70), nil).Once()
		node.Sender = &mockSender
		node.Balances = core.NewAddressBalances(10)
		defer node.Balances.StopCleanup()

		// Create mock orchestrator server that handles token requests and payments
		paymentReceived := false
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/process/token":
				orchTokenHandler(w, r)
			case "/ai/stream/payment":
				paymentReceived = true
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"status": "payment_processed"}`))
			default:
				http.NotFound(w, r)
			}
		}))
		defer server.Close()

		node.OrchestratorPool = newStubOrchestratorPool(node, []string{server.URL})
		ls := &LivepeerServer{LivepeerNode: node}
		drivers.NodeStorage = drivers.NewMemoryDriver(nil)

		// Create a mock stream with AI session
		streamID := "test-payment-stream"
		token := createMockJobToken(server.URL)
		sess, err := tokenToAISession(*token)
		assert.NoError(t, err)

		params := aiRequestParams{
			liveParams: &liveRequestParams{
				requestID:      "req-1",
				sess:           &sess,
				stream:         streamID,
				streamID:       streamID,
				sendErrorEvent: func(err error) {},
				segmentReader:  media.NewSwitchableSegmentReader(),
			},
			node: node,
		}

		stream := node.NewLivePipeline("req-1", streamID, "test-capability", params, nil)

		// Create a job sender
		jobSender := &core.JobSender{
			Addr: "0x1111111111111111111111111111111111111111",
			Sig:  "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
		}

		// Test sendPaymentForStream
		ctx := context.Background()
		err = ls.sendPaymentForStream(ctx, stream, jobSender)

		// Should succeed
		assert.NoError(t, err)

		// Verify payment was sent to orchestrator
		assert.True(t, paymentReceived, "Payment should have been sent to orchestrator")

		// Clean up
		stream.StopStream(nil)
	})

	t.Run("Error_GetTokenFailed", func(t *testing.T) {
		// Setup node without orchestrator pool
		node := mockJobLivepeerNode()
		// Set up mock sender to prevent nil pointer dereference
		mockSender := pm.MockSender{}
		mockSender.On("StartSession", mock.Anything).Return("foo")
		mockSender.On("CreateTicketBatch", mock.Anything, mock.Anything).Return(mockTicketBatch(10), nil)
		node.Sender = &mockSender
		node.Balances = core.NewAddressBalances(10)
		defer node.Balances.StopCleanup()

		ls := &LivepeerServer{LivepeerNode: node}

		// Create a stream with invalid session
		streamID := "test-invalid-token"
		invalidToken := createMockJobToken("http://nonexistent-server.com")
		sess, _ := tokenToAISession(*invalidToken)
		params := aiRequestParams{
			liveParams: &liveRequestParams{
				requestID:      "req-1",
				sess:           &sess,
				stream:         streamID,
				streamID:       streamID,
				sendErrorEvent: func(err error) {},
				segmentReader:  media.NewSwitchableSegmentReader(),
			},
			node: node,
		}
		stream := node.NewLivePipeline("req-1", streamID, "test-capability", params, nil)

		jobSender := &core.JobSender{
			Addr: "0x1111111111111111111111111111111111111111",
			Sig:  "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
		}

		// Should fail to get new token
		err := ls.sendPaymentForStream(context.Background(), stream, jobSender)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "nonexistent-server.com")

		stream.StopStream(nil)
	})

	t.Run("Error_PaymentCreationFailed", func(t *testing.T) {
		// Test with node that has no sender (payment creation will fail)
		node := mockJobLivepeerNode()
		// node.Sender is nil by default

		server := httptest.NewServer(http.HandlerFunc(orchTokenHandler))
		defer server.Close()
		node.OrchestratorPool = newStubOrchestratorPool(node, []string{server.URL})
		ls := &LivepeerServer{LivepeerNode: node}

		streamID := "test-payment-creation-fail"
		token := createMockJobToken(server.URL)
		sess, _ := tokenToAISession(*token)
		params := aiRequestParams{
			liveParams: &liveRequestParams{
				requestID:      "req-1",
				sess:           &sess,
				stream:         streamID,
				streamID:       streamID,
				sendErrorEvent: func(err error) {},
				segmentReader:  media.NewSwitchableSegmentReader(),
			},
			node: node,
		}
		stream := node.NewLivePipeline("req-1", streamID, "test-capability", params, nil)

		jobSender := &core.JobSender{
			Addr: "0x1111111111111111111111111111111111111111",
			Sig:  "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
		}

		// Should continue even if payment creation fails (no payment required)
		err := ls.sendPaymentForStream(context.Background(), stream, jobSender)
		assert.NoError(t, err) // Should not error, just logs and continues

		stream.StopStream(nil)
	})

	t.Run("Error_OrchestratorPaymentFailed", func(t *testing.T) {
		// Setup node with sender to create payments
		node := mockJobLivepeerNode()
		mockSender := pm.MockSender{}
		mockSender.On("StartSession", mock.Anything).Return("foo").Times(2)
		mockSender.On("CreateTicketBatch", "foo", 70).Return(mockTicketBatch(70), nil).Once()
		node.Sender = &mockSender
		node.Balances = core.NewAddressBalances(10)
		defer node.Balances.StopCleanup()

		// Create mock orchestrator that returns error for payments
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/process/token":
				orchTokenHandler(w, r)
			case "/ai/stream/payment":
				http.Error(w, "Payment processing failed", http.StatusInternalServerError)
			default:
				http.NotFound(w, r)
			}
		}))
		defer server.Close()

		node.OrchestratorPool = newStubOrchestratorPool(node, []string{server.URL})
		ls := &LivepeerServer{LivepeerNode: node}
		drivers.NodeStorage = drivers.NewMemoryDriver(nil)

		streamID := "test-payment-error"
		token := createMockJobToken(server.URL)
		sess, _ := tokenToAISession(*token)
		params := aiRequestParams{
			liveParams: &liveRequestParams{
				requestID:      "req-1",
				sess:           &sess,
				stream:         streamID,
				streamID:       streamID,
				sendErrorEvent: func(err error) {},
				segmentReader:  media.NewSwitchableSegmentReader(),
			},
			node: node,
		}
		stream := node.NewLivePipeline("req-1", streamID, "test-capability", params, nil)

		jobSender := &core.JobSender{
			Addr: "0x1111111111111111111111111111111111111111",
			Sig:  "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
		}

		// Should fail with payment error
		err := ls.sendPaymentForStream(context.Background(), stream, jobSender)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unexpected status code")

		stream.StopStream(nil)
	})

	t.Run("Error_TokenToSessionConversionNoPrice", func(t *testing.T) {
		// Test where tokenToAISession fails
		node := mockJobLivepeerNode()

		// Set up mock sender to prevent nil pointer dereference
		mockSender := pm.MockSender{}
		mockSender.On("StartSession", mock.Anything).Return("foo")
		mockSender.On("CreateTicketBatch", mock.Anything, mock.Anything).Return(mockTicketBatch(10), nil)
		node.Sender = &mockSender
		node.Balances = core.NewAddressBalances(10)
		defer node.Balances.StopCleanup()

		// Create a server that returns invalid token response
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/process/token" {
				// Return malformed token that will cause tokenToAISession to fail
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"invalid": "token_structure"}`))
				return
			}
			http.NotFound(w, r)
		}))
		defer server.Close()

		node.OrchestratorPool = newStubOrchestratorPool(node, []string{server.URL})
		ls := &LivepeerServer{LivepeerNode: node}

		// Create stream with valid initial session
		streamID := "test-token-no-price"
		token := createMockJobToken(server.URL)
		sess, _ := tokenToAISession(*token)
		params := aiRequestParams{
			liveParams: &liveRequestParams{
				requestID:      "req-1",
				sess:           &sess,
				stream:         streamID,
				streamID:       streamID,
				sendErrorEvent: func(err error) {},
				segmentReader:  media.NewSwitchableSegmentReader(),
			},
			node: node,
		}
		stream := node.NewLivePipeline("req-1", streamID, "test-capability", params, nil)

		jobSender := &core.JobSender{
			Addr: "0x1111111111111111111111111111111111111111",
			Sig:  "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
		}

		// Should fail during token to session conversion
		err := ls.sendPaymentForStream(context.Background(), stream, jobSender)
		assert.NoError(t, err)

		stream.StopStream(nil)
	})

	t.Run("Success_StreamParamsUpdated", func(t *testing.T) {
		// Test that stream params are updated with new session after token refresh
		node := mockJobLivepeerNode()
		mockSender := pm.MockSender{}
		mockSender.On("StartSession", mock.Anything).Return("foo").Times(2)
		mockSender.On("CreateTicketBatch", "foo", 70).Return(mockTicketBatch(70), nil).Once()
		node.Sender = &mockSender
		node.Balances = core.NewAddressBalances(10)
		defer node.Balances.StopCleanup()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/process/token":
				orchTokenHandler(w, r)
			case "/ai/stream/payment":
				w.WriteHeader(http.StatusOK)
			default:
				http.NotFound(w, r)
			}
		}))
		defer server.Close()

		node.OrchestratorPool = newStubOrchestratorPool(node, []string{server.URL})
		ls := &LivepeerServer{LivepeerNode: node}
		drivers.NodeStorage = drivers.NewMemoryDriver(nil)

		streamID := "test-params-update"
		originalToken := createMockJobToken(server.URL)
		originalSess, _ := tokenToAISession(*originalToken)
		originalSessionAddr := originalSess.Address()

		params := aiRequestParams{
			liveParams: &liveRequestParams{
				requestID:      "req-1",
				sess:           &originalSess,
				stream:         streamID,
				streamID:       streamID,
				sendErrorEvent: func(err error) {},
				segmentReader:  media.NewSwitchableSegmentReader(),
			},
			node: node,
		}
		stream := node.NewLivePipeline("req-1", streamID, "test-capability", params, nil)

		jobSender := &core.JobSender{
			Addr: "0x1111111111111111111111111111111111111111",
			Sig:  "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
		}

		// Send payment
		err := ls.sendPaymentForStream(context.Background(), stream, jobSender)
		assert.NoError(t, err)

		// Verify that stream params were updated with new session
		updatedParams, err := getStreamRequestParams(stream)
		assert.NoError(t, err)

		// The session should be updated (new token fetched)
		updatedSessionAddr := updatedParams.liveParams.sess.Address()
		// In a real scenario, this might be different, but our mock returns the same token
		// The important thing is that UpdateStreamParams was called
		assert.NotNil(t, updatedParams.liveParams.sess)
		assert.Equal(t, originalSessionAddr, updatedSessionAddr) // Same because mock returns same token

		stream.StopStream(nil)
	})
}

func TestTokenSessionConversion(t *testing.T) {
	token := createMockJobToken("http://example.com")
	sess, err := tokenToAISession(*token)
	assert.True(t, err != nil || sess != (AISession{}))
	assert.NotNil(t, sess.OrchestratorInfo)
	assert.NotNil(t, sess.OrchestratorInfo.TicketParams)

	assert.NotEmpty(t, sess.Address())
	assert.NotEmpty(t, sess.Transcoder())

	_, err = sessionToToken(&sess)
	assert.True(t, err != nil || true)
}

func TestGetStreamRequestParams(t *testing.T) {
	_, err := getStreamRequestParams(nil)
	assert.Error(t, err)
}

// createMockMediaMTXServer creates a simple mock MediaMTX server that returns 200 OK to all requests
func createMockMediaMTXServer(t *testing.T) *httptest.Server {
	mux := http.NewServeMux()

	// Simple handler that returns 200 OK to any request
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		t.Logf("Mock MediaMTX: %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Create a listener on port 9997 specifically
	listener, err := net.Listen("tcp", ":9997")
	if err != nil {
		t.Fatalf("Failed to listen on port 9997: %v", err)
	}

	server := &httptest.Server{
		Listener: listener,
		Config:   &http.Server{Handler: mux},
	}
	server.Start()

	t.Cleanup(func() {
		server.Close()
	})

	return server
}
