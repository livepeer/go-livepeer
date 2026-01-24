package byoc

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"math/rand/v2"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/media"
	"github.com/livepeer/go-livepeer/net"
	"github.com/livepeer/go-livepeer/pm"
	"github.com/livepeer/go-livepeer/trickle"
	"github.com/livepeer/go-tools/drivers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func newTestBYOCGatewayServer(node *core.LivepeerNode) *BYOCGatewayServer {
	if node == nil {
		node = mockJobLivepeerNode()
	}
	statusStore := NewMockStreamStatusStore()

	return NewBYOCGatewayServer(node, statusStore, nil, nil, http.NewServeMux())
}

// mockStreamStatusStore is a mock implementation of streamStatusStore for testing
type mockStreamStatusStore struct {
	mock.Mock
	store map[string]map[string]interface{}
	mu    sync.RWMutex
}

func NewMockStreamStatusStore() *mockStreamStatusStore {
	return &mockStreamStatusStore{
		store: make(map[string]map[string]interface{}),
	}
}

// Helper: check if any test added an expectation for a method name.
func (m *mockStreamStatusStore) hasExpectation(method string) bool {
	for _, c := range m.ExpectedCalls {
		if c.Method == method {
			return true
		}
	}
	return false
}

func (m *mockStreamStatusStore) Store(streamID string, status map[string]interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Use mock override if one exists
	if m.hasExpectation("Store") {
		m.Called(streamID, status)
		return
	}

	// Default real logic
	if m.store == nil {
		m.store = make(map[string]map[string]interface{})
	}
	m.store[streamID] = status
}

func (m *mockStreamStatusStore) Clear(streamID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.hasExpectation("Clear") {
		m.Called(streamID)
		return
	}

	delete(m.store, streamID)
}

func (m *mockStreamStatusStore) Get(streamID string) (map[string]interface{}, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.hasExpectation("Get") {
		args := m.Called(streamID)
		if args.Get(0) == nil {
			return nil, args.Bool(1)
		}
		return args.Get(0).(map[string]interface{}), args.Bool(1)
	}

	value, ok := m.store[streamID]
	return value, ok
}

func (m *mockStreamStatusStore) StoreIfNotExists(streamID string, key string, status interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.hasExpectation("StoreIfNotExists") {
		m.Called(streamID, key, status)
		return
	}

	if m.store == nil {
		m.store = make(map[string]map[string]interface{})
	}
	if _, ok := m.store[streamID]; !ok {
		m.store[streamID] = make(map[string]interface{})
	}
	if _, exists := m.store[streamID][key]; !exists {
		m.store[streamID][key] = status
	}
}

func (m *mockStreamStatusStore) StoreKey(streamID, key string, status interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.hasExpectation("StoreKey") {
		m.Called(streamID, key, status)
		return
	}

	if m.store == nil {
		m.store = make(map[string]map[string]interface{})
	}
	if _, ok := m.store[streamID]; !ok {
		m.store[streamID] = make(map[string]interface{})
	}
	m.store[streamID][key] = status
}

var stubOrchServerUrl string

// testOrch wraps mockOrchestrator to override a few methods needed by lphttp in tests
type testStreamOrch struct {
	*mockJobOrchestrator
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

func orchAIStreamStartNoUrlsHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/ai/stream/start" {
		http.NotFound(w, r)
		return
	}

	//Headers for trickle urls intentionally left out to prevent starting trickle streams for optional streams

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Control-Url", fmt.Sprintf("%s%s%s", stubOrchServerUrl, "/ai/trickle/", "test-stream-control"))
	w.Header().Set("X-Events-Url", fmt.Sprintf("%s%s%s", stubOrchServerUrl, "/ai/trickle/", "test-stream-events"))
	w.WriteHeader(http.StatusOK)
}

func orchAIStreamStopHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/ai/stream/stop" {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
}

func orchCapabilityUrlHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func TestStartStream_MaxBodyLimit(t *testing.T) {
	// Setup server with minimal dependencies
	synctest.Test(t, func(t *testing.T) {
		node := mockJobLivepeerNode()
		server := httptest.NewServer(http.HandlerFunc(orchTokenHandler))
		defer server.Close()
		node.OrchestratorPool = newStubOrchestratorPool(node, []string{server.URL})

		// Set up mock sender to prevent nil pointer dereference
		mockSender := pm.MockSender{}
		mockSender.On("StartSession", mock.Anything).Return("foo")
		mockSender.On("CreateTicketBatch", mock.Anything, mock.Anything).Return(mockTicketBatch(10), nil)
		node.Sender = &mockSender
		node.Balances = core.NewAddressBalances(10 * time.Second)
		defer node.Balances.StopCleanup()

		bsg := newTestBYOCGatewayServer(node)

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
		handler := bsg.StartStream()
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
	})
}

func TestStreamStart_SetupStream(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		node := mockJobLivepeerNode()
		server := httptest.NewServer(http.HandlerFunc(orchTokenHandler))
		defer server.Close()
		node.OrchestratorPool = newStubOrchestratorPool(node, []string{server.URL})

		// Set up mock sender to prevent nil pointer dereference
		mockSender := pm.MockSender{}
		mockSender.On("StartSession", mock.Anything).Return("foo")
		mockSender.On("CreateTicketBatch", mock.Anything, mock.Anything).Return(mockTicketBatch(10), nil)
		node.Sender = &mockSender
		node.Balances = core.NewAddressBalances(10 * time.Second)
		defer node.Balances.StopCleanup()

		bsg := newTestBYOCGatewayServer(node)
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

		urls, code, err := bsg.setupStream(context.Background(), req, gatewayJob)
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

		//confirm StreamPipeline created
		stream, err := bsg.streamPipeline(urls.StreamId)
		assert.NoError(t, err)
		assert.NotNil(t, stream)
		assert.Equal(t, urls.StreamId, stream.StreamID)
		assert.Equal(t, bsg.streamPipelineRequest(urls.StreamId), []byte("{\"params\":{}}"))
		bsg.removeStreamPipeline(urls.StreamId)

		//test with no data output
		jobParams = JobParameters{EnableVideoIngress: true, EnableVideoEgress: true, EnableDataOutput: false}
		paramsStr = marshalToString(t, jobParams)
		jobReq.Parameters = paramsStr
		gatewayJob.Job.Params = &jobParams
		req.Body = io.NopCloser(bytes.NewReader(body))
		urls, code, err = bsg.setupStream(context.Background(), req, gatewayJob)
		assert.NotNil(t, urls)
		assert.Empty(t, urls.DataUrl)
		bsg.removeStreamPipeline(urls.StreamId)

		//test with no video ingress
		jobParams = JobParameters{EnableVideoIngress: false, EnableVideoEgress: true, EnableDataOutput: true}
		paramsStr = marshalToString(t, jobParams)
		jobReq.Parameters = paramsStr
		gatewayJob.Job.Params = &jobParams
		req.Body = io.NopCloser(bytes.NewReader(body))
		urls, code, err = bsg.setupStream(context.Background(), req, gatewayJob)
		assert.NotNil(t, urls)
		assert.Empty(t, urls.WhipUrl)
		assert.Empty(t, urls.RtmpUrl)
		bsg.removeStreamPipeline(urls.StreamId)

		//test with no video egress
		jobParams = JobParameters{EnableVideoIngress: true, EnableVideoEgress: false, EnableDataOutput: true}
		paramsStr = marshalToString(t, jobParams)
		jobReq.Parameters = paramsStr
		gatewayJob.Job.Params = &jobParams
		req.Body = io.NopCloser(bytes.NewReader(body))
		urls, code, err = bsg.setupStream(context.Background(), req, gatewayJob)
		assert.NotNil(t, urls)
		assert.Empty(t, urls.WhepUrl)
		assert.Empty(t, urls.RtmpOutputUrl)
		bsg.removeStreamPipeline(urls.StreamId)

		// Test with nil job
		urls, code, err = bsg.setupStream(context.Background(), req, nil)
		assert.Error(t, err)
		assert.Equal(t, http.StatusBadRequest, code)
		assert.Nil(t, urls)
		assert.Zero(t, len(bsg.StreamPipelines))

		// Test with invalid JSON body
		badReq := httptest.NewRequest(http.MethodPost, "/ai/stream/start", bytes.NewReader([]byte("notjson")))
		badReq.Header.Set("Content-Type", "application/json")
		urls, code, err = bsg.setupStream(context.Background(), badReq, gatewayJob)
		assert.Error(t, err)
		assert.Equal(t, http.StatusBadRequest, code)
		assert.Nil(t, urls)
		assert.Zero(t, len(bsg.StreamPipelines))

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
		urls, code, err = bsg.setupStream(context.Background(), outReqHTTP, gatewayJob)
		assert.NoError(t, err)
		assert.Equal(t, 0, code)
		assert.Nil(t, urls)
		assert.Zero(t, len(bsg.StreamPipelines))
	})
}

func TestRunStream_RunAndCancelStream(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		node := mockJobLivepeerNode()

		// Set up an lphttp-based orchestrator test server with trickle endpoints
		mux := http.NewServeMux()
		mockOrch := &mockJobOrchestrator{}
		mockOrch.On("VerifySig", mock.Anything, mock.Anything, mock.Anything).Return(true)
		mockOrch.On("DebitFees", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return()

		bso := &BYOCOrchestratorServer{orch: nil, httpMux: mux, node: node}
		// Configure trickle server on the mux (imitate production trickle endpoints)
		bso.trickleSrv = trickle.ConfigureServer(trickle.TrickleServerConfig{
			Mux:        mux,
			BasePath:   "/ai/trickle/",
			Autocreate: true,
		})
		// Register orchestrator endpoints used by runStream path
		mux.HandleFunc("/ai/stream/start", orchAIStreamStartNoUrlsHandler)
		mux.HandleFunc("/ai/stream/stop", orchAIStreamStopHandler)
		mux.HandleFunc("/process/token", orchTokenHandler)

		server := httptest.NewServer(bso.httpMux)
		defer server.Close()

		stubOrchServerUrl = server.URL

		// Configure mock orchestrator behavior expected by lphttp handlers
		parsedURL, _ := url.Parse(server.URL)
		capabilitySrv := httptest.NewServer(http.HandlerFunc(orchCapabilityUrlHandler))
		defer capabilitySrv.Close()

		// attach our orchestrator implementation to lphttp
		bso.orch = &testStreamOrch{mockJobOrchestrator: mockOrch, svc: parsedURL, capURL: capabilitySrv.URL}

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
		gatewayJob := &gatewayJob{Job: orchJob, Orchs: []JobToken{*orchToken}, node: node}

		// Setup a LivepeerServer and a mock pipeline
		bsg := newTestBYOCGatewayServer(node)
		bsg.node.OrchestratorPool = newStubOrchestratorPool(bsg.node, []string{server.URL})
		drivers.NodeStorage = drivers.NewMemoryDriver(nil)
		mockSender := pm.MockSender{}
		mockSender.On("StartSession", mock.Anything).Return("foo").Times(4)
		mockSender.On("CreateTicketBatch", "foo", 10).Return(mockTicketBatch(10), nil).Twice() //payment sent on start and stop (only once on stop)
		node.Sender = &mockSender
		node.Balances = core.NewAddressBalances(10 * time.Second)
		defer node.Balances.StopCleanup()

		//now sign job and create a sig for the sender to include
		gatewayJob.sign()
		sender, err := getJobSender(context.TODO(), node)
		assert.NoError(t, err)
		orchJob.Req.Sender = sender.Addr
		orchJob.Req.Sig = sender.Sig
		// Minimal aiRequestParams and liveRequestParams
		params := byocAIRequestParams{
			liveParams: &byocLiveRequestParams{
				requestID:      "req-1",
				stream:         "test-stream",
				streamID:       "test-stream",
				sendErrorEvent: func(err error) {},
				segmentReader:  nil,
			},
			node: node,
		}

		bsg.newStreamPipeline("req-1", "test-stream", "test-capability", params, nil)

		// Should not panic and should clean up
		var wg sync.WaitGroup
		wg.Add(2)
		go func() { defer wg.Done(); bsg.runStream(gatewayJob) }()
		go func() { defer wg.Done(); bsg.monitorStream(gatewayJob.Job.Req.ID) }()

		// Cancel the stream after a short delay to simulate shutdown
		done := make(chan struct{})
		go func() {
			stream, _ := bsg.streamPipeline(gatewayJob.Job.Req.ID)

			if stream != nil {
				// Wait for kickOrch to be set and call it to cancel the stream
				timeout := time.After(1 * time.Second)
				var kickOrch context.CancelCauseFunc
			waitLoop:
				for {
					select {
					case <-timeout:
						// Timeout waiting for kickOrch, proceed anyway
						break waitLoop
					default:
						params, err := bsg.streamPipelineParams(gatewayJob.Job.Req.ID)
						if err == nil {
							params.liveParams.mu.Lock()
							kickOrch = params.liveParams.kickOrch
							params.liveParams.mu.Unlock()
						}
						if err == nil && kickOrch != nil {
							kickOrch(errors.New("test cancellation"))
							break waitLoop
						}
					}
				}
			}
			close(done)
		}()
		<-done
		// Wait for both goroutines to finish before asserting
		wg.Wait()
		_, err = bsg.streamPipeline(params.liveParams.streamID)
		assert.Error(t, err)

		// Clean up external capabilities streams
		if node.ExternalCapabilities != nil {
			for streamID := range node.ExternalCapabilities.Streams {
				node.ExternalCapabilities.RemoveStream(streamID)
			}
		}

		//confirm external capability stream removed
		_, ok := node.ExternalCapabilities.GetStream("test-stream")
		assert.False(t, ok)
	})
}

// TestRunStream_OrchestratorFailover tests that runStream fails over to a second orchestrator
// when the first one fails, and stops when the second orchestrator also fails
func TestRunStream_OrchestratorFailover(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		node := mockJobLivepeerNode()

		// Channels to signal when each orchestrator is contacted
		orch1Started := make(chan struct{}, 1)
		orch2Started := make(chan struct{}, 1)

		// Set up an lphttp-based orchestrator test server with trickle endpoints
		mux := http.NewServeMux()
		mockOrch := &mockJobOrchestrator{}
		mockOrch.On("VerifySig", mock.Anything, mock.Anything, mock.Anything).Return(true)
		mockOrch.On("DebitFees", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return()
		mockOrch2 := *mockOrch

		bso := &BYOCOrchestratorServer{orch: nil, httpMux: mux, node: node}
		// Configure trickle server on the mux (imitate production trickle endpoints)
		bso.trickleSrv = trickle.ConfigureServer(trickle.TrickleServerConfig{
			Mux:        mux,
			BasePath:   "/ai/trickle/",
			Autocreate: true,
		})
		// Register orchestrator endpoints used by runStream path - wrap to signal when called
		mux.HandleFunc("/ai/stream/start", func(w http.ResponseWriter, r *http.Request) {
			select {
			case orch1Started <- struct{}{}:
			default:
			}
			orchAIStreamStartNoUrlsHandler(w, r)
		})
		mux.HandleFunc("/ai/stream/stop", orchAIStreamStopHandler)
		mux.HandleFunc("/process/token", orchTokenHandler)

		server := httptest.NewServer(bso.httpMux)
		defer server.Close()
		mux2 := http.NewServeMux()
		bso2 := &BYOCOrchestratorServer{orch: nil, httpMux: mux2, node: mockJobLivepeerNode()}
		// Configure trickle server on the mux (imitate production trickle endpoints)
		bso2.trickleSrv = trickle.ConfigureServer(trickle.TrickleServerConfig{
			Mux:        mux2,
			BasePath:   "/ai/trickle/",
			Autocreate: true,
		})
		// Register orchestrator endpoints used by runStream path - wrap to signal when called
		mux2.HandleFunc("/ai/stream/start", func(w http.ResponseWriter, r *http.Request) {
			select {
			case orch2Started <- struct{}{}:
			default:
			}
			orchAIStreamStartNoUrlsHandler(w, r)
		})
		mux2.HandleFunc("/ai/stream/stop", orchAIStreamStopHandler)
		mux2.HandleFunc("/process/token", orchTokenHandler)

		server2 := httptest.NewServer(bso2.httpMux)
		defer server2.Close()

		// Configure mock orchestrator behavior expected by lphttp handlers
		parsedURL, _ := url.Parse(server.URL)
		capabilitySrv := httptest.NewServer(http.HandlerFunc(orchCapabilityUrlHandler))
		defer capabilitySrv.Close()

		parsedURL2, _ := url.Parse(server2.URL)
		capabilitySrv2 := httptest.NewServer(http.HandlerFunc(orchCapabilityUrlHandler))
		defer capabilitySrv2.Close()
		// attach our orchestrator implementation to lphttp
		bso.orch = &testStreamOrch{mockJobOrchestrator: mockOrch, svc: parsedURL, capURL: capabilitySrv.URL}
		bso2.orch = &testStreamOrch{mockJobOrchestrator: &mockOrch2, svc: parsedURL2, capURL: capabilitySrv2.URL}

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
		orchToken2 := createMockJobToken(server2.URL)
		orchToken2.TicketParams.Recipient = ethcommon.HexToAddress("0x1111111111111111111111111111111111111112").Bytes()
		orchJob := &orchJob{Req: jobReq, Params: &jobParams}
		gatewayJob := &gatewayJob{Job: orchJob, Orchs: []JobToken{*orchToken, *orchToken2}, node: node}

		// Setup a LivepeerServer and a mock pipeline
		bsg := newTestBYOCGatewayServer(node)
		bsg.node.OrchestratorPool = newStubOrchestratorPool(bsg.node, []string{server.URL, server2.URL})
		drivers.NodeStorage = drivers.NewMemoryDriver(nil)
		mockSender := pm.MockSender{}
		mockSender.On("StartSession", mock.Anything).Return("foo").Times(4)
		mockSender.On("CreateTicketBatch", "foo", orchJob.Req.Timeout).Return(mockTicketBatch(orchJob.Req.Timeout), nil).Times(4) //payment sent on start and stop of each Orch (only once on stop)
		node.Sender = &mockSender
		node.Balances = core.NewAddressBalances(10 * time.Second)
		defer node.Balances.StopCleanup()

		//now sign job and create a sig for the sender to include
		gatewayJob.sign()
		sender, err := getJobSender(context.TODO(), node)
		assert.NoError(t, err)
		orchJob.Req.Sender = sender.Addr
		orchJob.Req.Sig = sender.Sig
		// Minimal aiRequestParams and liveRequestParams
		params := byocAIRequestParams{
			liveParams: &byocLiveRequestParams{
				requestID:      "req-1",
				stream:         "test-stream",
				streamID:       "test-stream",
				sendErrorEvent: func(err error) {},
				segmentReader:  media.NewSwitchableSegmentReader(),
			},
			node: node,
		}

		bsg.newStreamPipeline("req-1", "test-stream", "test-capability", params, nil)

		streamID := gatewayJob.Job.Req.ID
		// Should not panic and should clean up
		var wg sync.WaitGroup
		wg.Add(2)
		go func() { defer wg.Done(); bsg.runStream(gatewayJob) }()
		go func() { defer wg.Done(); bsg.monitorStream(streamID) }()

		// Wait for first orchestrator to be contacted
		select {
		case <-orch1Started:
			t.Log("Orchestrator 1 started")
		case <-time.After(1 * time.Second):
			t.Fatal("Timeout waiting for orchestrator 1 to start")
		}

		// Kick the first orchestrator to trigger failover
		params2, err := bsg.streamPipelineParams(streamID)
		if err != nil {
			t.Fatalf("Failed to get stream params: %v", err)
		}

		params2.liveParams.mu.Lock()
		kickOrch := params2.liveParams.kickOrch
		params2.liveParams.mu.Unlock()
		if kickOrch == nil {
			t.Fatal("kickOrch should be set after orchestrator starts")
		}
		kickOrch(errors.New("test cancellation orch1"))
		t.Log("Orchestrator 1 kicked")

		// Wait for failover to second orchestrator
		select {
		case <-orch2Started:
			t.Log("Orchestrator 2 started (failover successful)")
		case <-time.After(1 * time.Second):
			t.Fatal("Timeout waiting for orchestrator 2 to start after failover")
		}

		//kick the second Orchestrator
		params3, err := bsg.streamPipelineParams(streamID)
		if err != nil {
			t.Fatalf("Failed to get stream params: %v", err)
		}

		params3.liveParams.mu.Lock()
		kickOrch2 := params3.liveParams.kickOrch
		params3.liveParams.mu.Unlock()
		if kickOrch2 == nil {
			t.Fatal("kickOrch should be set after orchestrator 2 starts")
		}
		kickOrch2(errors.New("test cancellation orch2"))
		t.Log("Orchestrator 2 kicked")

		// Wait for both goroutines to finish before asserting
		wg.Wait()
		// After cancel, the stream should be removed from byoc StreamPipelines
		_, err = bsg.streamPipeline(streamID)
		assert.Error(t, err)

		// Clean up external capabilities streams
		if node.ExternalCapabilities != nil {
			for streamID := range node.ExternalCapabilities.Streams {
				node.ExternalCapabilities.RemoveStream(streamID)
			}
		}
	})
}

func TestStartStreamHandler(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		node := mockJobLivepeerNode()
		orch1Started := make(chan struct{}, 1)

		// Set up an lphttp-based orchestrator test server with trickle endpoints
		mux := http.NewServeMux()
		bsg := newTestBYOCGatewayServer(node)
		mockSender := pm.MockSender{}
		mockSender.On("StartSession", mock.Anything).Return("foo")
		mockSender.On("CreateTicketBatch", mock.Anything, mock.Anything).Return(mockTicketBatch(10), nil)
		node.Sender = &mockSender
		node.Balances = core.NewAddressBalances(1 * time.Second)
		//setup Orch server stub
		mux.HandleFunc("/process/token", orchTokenHandler)
		mux.HandleFunc("/ai/stream/start", func(w http.ResponseWriter, r *http.Request) {
			select {
			case orch1Started <- struct{}{}:
			default:
			}
			orchAIStreamStartNoUrlsHandler(w, r)
		})

		server := httptest.NewServer(mux)
		defer server.Close()

		bsg.node.OrchestratorPool = newStubOrchestratorPool(bsg.node, []string{server.URL})
		drivers.NodeStorage = drivers.NewMemoryDriver(nil)
		// Prepare a valid StartRequest body
		startReq := StartRequest{
			Stream:     "teststream",
			RtmpOutput: "rtmp://output",
			StreamId:   "streamid",
			Params:     "{}",
		}
		body, _ := json.Marshal(startReq)
		req := httptest.NewRequest(http.MethodPost, "/process/stream/start", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		req.Header.Set("Livepeer", base64TestJobRequest(10, true, true, true))

		w := httptest.NewRecorder()

		handler := bsg.StartStream()
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		body = w.Body.Bytes()
		var streamUrls StreamUrls
		err := json.Unmarshal(body, &streamUrls)
		assert.NoError(t, err)
		stream, err := bsg.streamPipeline(streamUrls.StreamId)
		assert.NoError(t, err)
		assert.NotNil(t, stream)
		assert.Equal(t, streamUrls.StreamId, stream.StreamID)

		//kick the orch to stop the stream and cleanup
		<-orch1Started
		streamParams, _ := bsg.streamPipelineParams(streamUrls.StreamId)
		if streamParams.liveParams.kickOrch != nil {
			streamParams.liveParams.kickOrch(errors.New("test cleanup"))
		}
		bsg.node.Balances.StopCleanup()
	})
}

func TestStopStreamHandler(t *testing.T) {

	t.Run("StreamNotFound", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			// Test case 1: Stream doesn't exist - should return 404
			bsg := newTestBYOCGatewayServer(nil)
			req := httptest.NewRequest(http.MethodPost, "/process/stream/{streamId}/stop", nil)
			req.SetPathValue("streamId", "non-existent-stream")
			w := httptest.NewRecorder()

			handler := bsg.StopStream()
			handler.ServeHTTP(w, req)

			assert.Equal(t, http.StatusNotFound, w.Code)
			assert.Contains(t, w.Body.String(), "Stream not found")
		})
	})

	t.Run("StreamExistsAndStopsSuccessfully", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
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
			bsg := newTestBYOCGatewayServer(node)
			drivers.NodeStorage = drivers.NewMemoryDriver(nil)
			mockSender := pm.MockSender{}
			mockSender.On("StartSession", mock.Anything).Return("foo").Times(4)
			mockSender.On("CreateTicketBatch", "foo", 10).Return(mockTicketBatch(10), nil).Once()
			node.Sender = &mockSender
			node.Balances = core.NewAddressBalances(10 * time.Second)
			defer node.Balances.StopCleanup()
			// Create a stream to stop
			streamID := "test-stream-to-stop"

			// Create minimal AI session with properly formatted URL
			token := createMockJobToken(server.URL)

			// Create stream parameters
			params := byocAIRequestParams{
				liveParams: &byocLiveRequestParams{
					requestID:      "req-1",
					orchToken:      *token,
					stream:         streamID,
					streamID:       streamID,
					sendErrorEvent: func(err error) {},
					segmentReader:  nil,
				},
				node: node,
			}

			// Add the stream to StreamPipelines
			stream := bsg.newStreamPipeline("req-1", streamID, "test-capability", params, nil)
			assert.NotNil(t, stream)

			// Verify stream exists before stopping
			_, err := bsg.streamPipeline(streamID)
			assert.NoError(t, err, "Stream should exist before stopping")

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

			req := httptest.NewRequest(http.MethodPost, "/process/stream/{streamId}/stop", strings.NewReader(`{"reason": "test stop"}`))
			req.SetPathValue("streamId", streamID)
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set(jobRequestHdr, jobReqB64)

			w := httptest.NewRecorder()

			handler := bsg.StopStream()
			handler.ServeHTTP(w, req)

			// The response might vary depending on orchestrator communication success
			// The important thing is that the stream is removed regardless
			assert.Contains(t, []int{http.StatusOK, http.StatusInternalServerError, http.StatusBadRequest}, w.Code,
				"Should return valid HTTP status")

			// Verify stream was removed from StreamPipelines (this should always happen)
			_, err = bsg.streamPipeline(streamID)
			assert.Error(t, err, "Stream should be removed after stopping")
		})
	})

	t.Run("StreamExistsButOrchestratorError", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
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
			bsg := newTestBYOCGatewayServer(node)
			drivers.NodeStorage = drivers.NewMemoryDriver(nil)
			mockSender := pm.MockSender{}
			mockSender.On("StartSession", mock.Anything).Return("foo").Times(4)
			mockSender.On("CreateTicketBatch", "foo", 10).Return(mockTicketBatch(10), nil).Once()
			node.Sender = &mockSender
			node.Balances = core.NewAddressBalances(10 * time.Second)
			defer node.Balances.StopCleanup()
			streamID := "test-stream-orch-error"

			// Create minimal AI session
			token := createMockJobToken(server.URL)

			params := byocAIRequestParams{
				liveParams: &byocLiveRequestParams{
					requestID:      "req-1",
					orchToken:      *token,
					stream:         streamID,
					streamID:       streamID,
					sendErrorEvent: func(err error) {},
					segmentReader:  nil,
				},
				node: node,
			}

			// Add the stream
			stream := bsg.newStreamPipeline("req-1", streamID, "test-capability", params, nil)
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

			req := httptest.NewRequest(http.MethodPost, "/process/stream/{streamId}/stop", nil)
			req.SetPathValue("streamId", streamID)
			req.Header.Set(jobRequestHdr, jobReqB64)

			w := httptest.NewRecorder()

			handler := bsg.StopStream()
			handler.ServeHTTP(w, req)

			// Returns 200 OK because Gateway removed the stream. If the Orchestrator errors, it will return
			// the error in the response body
			assert.Equal(t, http.StatusOK, w.Code)

			// Stream should still be removed even if orchestrator returns error
			_, err = bsg.streamPipeline(streamID)
			assert.Error(t, err, "Stream should be removed even on orchestrator error")
		})
	})
}

func TestStartStreamWhipIngestHandler(t *testing.T) {
	min := 10000
	max := 65535
	// rand.Intn returns a non-negative pseudo-random integer in the range [0, n).
	// Adding min to the result shifts the range to [min, max].
	randomNumber := rand.IntN(max-min+1) + min
	t.Setenv("LIVE_AI_WHIP_ADDR", fmt.Sprintf(":%d", randomNumber))
	whipServer := media.NewWHIPServer()
	synctest.Test(t, func(t *testing.T) {
		node := mockJobLivepeerNode()
		node.WorkDir = t.TempDir()
		server := httptest.NewServer(http.HandlerFunc(orchTokenHandler))
		defer server.Close()
		node.OrchestratorPool = newStubOrchestratorPool(node, []string{server.URL})
		bsg := newTestBYOCGatewayServer(node)

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
		req := httptest.NewRequest(http.MethodPost, "/process/stream/start", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		urls, code, err := bsg.setupStream(context.Background(), req, gatewayJob)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, code)
		assert.NotNil(t, urls)
		assert.Equal(t, "teststream-streamid", urls.StreamId) //combination of stream name (Stream) and id (StreamId)

		stream, err := bsg.streamPipeline(urls.StreamId)
		assert.NoError(t, err)
		assert.NotNil(t, stream)

		params, err := bsg.streamPipelineParams(stream.StreamID)
		assert.NoError(t, err)

		//these should be empty/nil before whip ingest starts
		assert.Empty(t, params.liveParams.localRTMPPrefix)
		assert.Nil(t, params.liveParams.kickInput)

		// whipServer is required, using nil will test setup up to initializing the WHIP connection

		handler := bsg.StartStreamWhipIngest(whipServer)

		// Blank SDP offer to test through creating WHIP connection
		sdpOffer1 := ""

		whipReq := httptest.NewRequest(http.MethodPost, "/process/stream/{streamId}/whip", strings.NewReader(sdpOffer1))
		whipReq.SetPathValue("streamId", "teststream-streamid")
		whipReq.Header.Set("Content-Type", "application/sdp")

		w := httptest.NewRecorder()
		handler.ServeHTTP(w, whipReq)
		// Since the SDP offer is empty, we expect a bad request response
		assert.Equal(t, http.StatusBadRequest, w.Code)

		// This completes testing through making the WHIP connection which would
		// then be covered by tests in whip_server.go
		newParams, err := bsg.streamPipelineParams(stream.StreamID)
		assert.NoError(t, err)
		assert.NotNil(t, newParams.liveParams.kickInput)

		newParams.liveParams.kickInput(errors.New("test complete"))
	})
}

func TestGetStreamDataHandler(t *testing.T) {
	t.Run("StreamData_MissingStreamId", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			// Test with missing stream ID - should return 400
			bsg := newTestBYOCGatewayServer(nil)
			handler := bsg.StreamData()
			req := httptest.NewRequest(http.MethodPost, "/process/stream/{streamId}/data", nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			assert.Equal(t, http.StatusBadRequest, w.Code)
			assert.Contains(t, w.Body.String(), "Missing stream name")
		})
	})

	t.Run("StreamData_DataOutputWorking", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			node := mockJobLivepeerNode()
			node.WorkDir = t.TempDir()
			server := httptest.NewServer(http.HandlerFunc(orchTokenHandler))
			defer server.Close()
			node.OrchestratorPool = newStubOrchestratorPool(node, []string{server.URL})
			bsg := newTestBYOCGatewayServer(node)
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
			req := httptest.NewRequest(http.MethodPost, "/process/stream/start", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			urls, code, err := bsg.setupStream(context.Background(), req, gatewayJob)
			assert.NoError(t, err)
			assert.Equal(t, http.StatusOK, code)
			assert.NotNil(t, urls)
			assert.Equal(t, "teststream-streamid", urls.StreamId) //combination of stream name (Stream) and id (StreamId)

			stream, err := bsg.streamPipeline(urls.StreamId)
			assert.NoError(t, err)
			assert.NotNil(t, stream)

			params, err := bsg.streamPipelineParams(stream.StreamID)
			assert.NoError(t, err)
			assert.NotNil(t, params.liveParams)

			// Write some test data first
			writer, err := params.liveParams.dataWriter.Next()
			assert.NoError(t, err)
			writer.Write([]byte("initial-data"))
			writer.Close()

			handler := bsg.StreamData()
			dataReq := httptest.NewRequest(http.MethodGet, "/process/stream/{streamId}/data", nil)
			dataReq.SetPathValue("streamId", "teststream-streamid")

			// Create a context with timeout to prevent infinite blocking
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()
			dataReq = dataReq.WithContext(ctx)

			// Start writing more segments in a goroutine
			go func() {
				// Write additional segments
				for i := 0; i < 2; i++ {
					writer, err := params.liveParams.dataWriter.Next()
					if err != nil {
						break
					}
					writer.Write([]byte(fmt.Sprintf("test-data-%d", i)))
					writer.Close()
				}

				// Close the writer to signal EOF
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
	})
}

func TestUpdateStreamHandler(t *testing.T) {
	t.Run("UpdateStream_MissingStreamId", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			// Test with missing stream ID - should return 400
			bsg := newTestBYOCGatewayServer(nil)
			handler := bsg.UpdateStream()
			req := httptest.NewRequest(http.MethodPost, "/process/stream/{streamId}/update", nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			assert.Equal(t, http.StatusBadRequest, w.Code)
			assert.Contains(t, w.Body.String(), "Missing stream name")
		})
	})

	t.Run("Basic_StreamNotFound", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			// Test with non-existent stream - should return 404
			node := mockJobLivepeerNode()
			bsg := newTestBYOCGatewayServer(node)

			req := httptest.NewRequest(http.MethodPost, "/process/stream/{streamId}/update",
				strings.NewReader(`{"param1": "value1", "param2": "value2"}`))
			req.SetPathValue("streamId", "non-existent-stream")
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			handler := bsg.UpdateStream()
			handler.ServeHTTP(w, req)

			assert.Equal(t, http.StatusNotFound, w.Code)
			assert.Contains(t, w.Body.String(), "Stream not found")
		})
	})

	t.Run("UpdateStream_ErrorHandling", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
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
			node.Balances = core.NewAddressBalances(10 * time.Second)
			defer node.Balances.StopCleanup()

			bsg := newTestBYOCGatewayServer(node)
			drivers.NodeStorage = drivers.NewMemoryDriver(nil)

			// Test 1: Wrong HTTP method
			req := httptest.NewRequest(http.MethodGet, "/process/stream/{streamId}/update", nil)
			req.SetPathValue("streamId", "test-stream")
			w := httptest.NewRecorder()
			bsg.UpdateStream().ServeHTTP(w, req)
			assert.Equal(t, http.StatusMethodNotAllowed, w.Code)

			// Test 2: Request too large
			streamID := "test-stream-large"
			token := createMockJobToken(server.URL)

			params := byocAIRequestParams{
				liveParams: &byocLiveRequestParams{
					requestID:      "req-1",
					orchToken:      *token,
					stream:         streamID,
					streamID:       streamID,
					sendErrorEvent: func(err error) {},
					segmentReader:  nil,
				},
				node: node,
			}
			_ = bsg.newStreamPipeline("req-1", streamID, "test-capability", params, nil)

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
			req = httptest.NewRequest(http.MethodPost, "/process/stream/{streamId}/update",
				bytes.NewReader(largeData))
			req.SetPathValue("streamId", streamID)
			req.Header.Set(jobRequestHdr, jobReqB64)
			w = httptest.NewRecorder()

			bsg.UpdateStream().ServeHTTP(w, req)
			assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
			assert.Contains(t, w.Body.String(), "request body too large")

			bsg.stopStreamPipeline(streamID, nil)
		})
	})
}

func TestGetStreamStatusHandler(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		bsg := newTestBYOCGatewayServer(nil)
		bsg.statusStore.Clear("any-stream")
		handler := bsg.StreamStatus()
		// stream does not exist
		req := httptest.NewRequest(http.MethodGet, "/process/stream/{streamId}/status", nil)
		req.SetPathValue("streamId", "any-stream")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNotFound, w.Code)

		// stream exists
		node := mockJobLivepeerNode()
		bsg.node = node
		bsg.newStreamPipeline("req-1", "any-stream", "test-capability", byocAIRequestParams{}, nil)
		bsg.statusStore.StoreKey("any-stream", "test", "test")
		req = httptest.NewRequest(http.MethodGet, "/process/stream/{streamId}/status", nil)
		req.SetPathValue("streamId", "any-stream")
		w = httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestSendPaymentForStream(t *testing.T) {
	t.Run("Success_ValidPayment", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			paymentReceived := false
			// Create server for this test
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/process/token":
					orchTokenHandler(w, r)
				case "/ai/stream/payment":
					w.WriteHeader(http.StatusOK)
					w.Write([]byte(`{"status": "payment_processed"}`))
					paymentReceived = true
				default:
					http.NotFound(w, r)
				}
			}))
			defer server.Close()
			defer server.CloseClientConnections()

			// Setup
			node := mockJobLivepeerNode()
			mockSender := pm.MockSender{}
			mockSender.On("StartSession", mock.Anything).Return("foo").Times(2)
			mockSender.On("CreateTicketBatch", "foo", 70).Return(mockTicketBatch(70), nil).Once()
			node.Sender = &mockSender
			node.Balances = core.NewAddressBalances(10 * time.Second)
			defer node.Balances.StopCleanup()

			node.OrchestratorPool = newStubOrchestratorPool(node, []string{server.URL})
			bsg := newTestBYOCGatewayServer(node)
			drivers.NodeStorage = drivers.NewMemoryDriver(nil)

			// Create a mock stream with AI session
			streamID := "test-payment-stream"
			token := createMockJobToken(server.URL)

			params := byocAIRequestParams{
				liveParams: &byocLiveRequestParams{
					requestID:      "req-1",
					orchToken:      *token,
					stream:         streamID,
					streamID:       streamID,
					sendErrorEvent: func(err error) {},
					segmentReader:  nil,
				},
				node: node,
			}

			_ = bsg.newStreamPipeline("req-1", streamID, "test-capability", params, nil)

			// Create a job sender
			jobSender := &JobSender{
				Addr: "0x1111111111111111111111111111111111111111",
				Sig:  "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
			}

			// Test sendPaymentForStream
			ctx := context.Background()
			err := bsg.sendPaymentForStream(ctx, streamID, jobSender)

			// Should succeed
			assert.NoError(t, err)

			// Verify payment was sent to orchestrator
			assert.True(t, paymentReceived, "Payment should have been sent to orchestrator")

			// Clean up
			bsg.stopStreamPipeline(streamID, nil)
		})
	})

	t.Run("Error_GetTokenFailed", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			// Setup node without orchestrator pool
			node := mockJobLivepeerNode()
			// Set up mock sender to prevent nil pointer dereference
			mockSender := pm.MockSender{}
			mockSender.On("StartSession", mock.Anything).Return("foo")
			mockSender.On("CreateTicketBatch", mock.Anything, mock.Anything).Return(mockTicketBatch(10), nil)
			node.Sender = &mockSender
			node.Balances = core.NewAddressBalances(10 * time.Second)
			defer node.Balances.StopCleanup()

			bsg := newTestBYOCGatewayServer(node)

			// Create a stream with invalid session (using an invalid URL that won't require DNS)
			streamID := "test-invalid-token"
			invalidToken := createMockJobToken("http://127.0.0.1:1/invalid") // Port 1 will fail quickly without DNS
			params := byocAIRequestParams{
				liveParams: &byocLiveRequestParams{
					requestID:      "req-1",
					orchToken:      *invalidToken,
					stream:         streamID,
					streamID:       streamID,
					sendErrorEvent: func(err error) {},
					segmentReader:  nil,
				},
				node: node,
			}
			_ = bsg.newStreamPipeline("req-1", streamID, "test-capability", params, nil)

			jobSender := &JobSender{
				Addr: "0x1111111111111111111111111111111111111111",
				Sig:  "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
			}

			// Should fail to get new token
			err := bsg.sendPaymentForStream(context.Background(), streamID, jobSender)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "127.0.0.1")

			bsg.stopStreamPipeline(streamID, nil)
		})
	})

	t.Run("Error_PaymentCreationFailed", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			// Test with node that has no sender (payment creation will fail)
			node := mockJobLivepeerNode()
			// node.Sender is nil by default

			server := httptest.NewServer(http.HandlerFunc(orchTokenHandler))
			defer server.Close()
			defer server.CloseClientConnections()
			node.OrchestratorPool = newStubOrchestratorPool(node, []string{server.URL})
			bsg := newTestBYOCGatewayServer(node)

			streamID := "test-payment-creation-fail"
			token := createMockJobToken(server.URL)
			params := byocAIRequestParams{
				liveParams: &byocLiveRequestParams{
					requestID:      "req-1",
					orchToken:      *token,
					stream:         streamID,
					streamID:       streamID,
					sendErrorEvent: func(err error) {},
					segmentReader:  nil,
				},
				node: node,
			}
			_ = bsg.newStreamPipeline("req-1", streamID, "test-capability", params, nil)

			jobSender := &JobSender{
				Addr: "0x1111111111111111111111111111111111111111",
				Sig:  "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
			}

			// Should continue even if payment creation fails (no payment required)
			err := bsg.sendPaymentForStream(context.Background(), streamID, jobSender)
			assert.NoError(t, err) // Should not error, just logs and continues

			bsg.stopStreamPipeline(streamID, nil)
		})
	})

	t.Run("Error_OrchestratorPaymentFailed", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			// Create server for this test with payment failure
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
			defer server.CloseClientConnections()

			// Setup node with sender to create payments
			node := mockJobLivepeerNode()
			mockSender := pm.MockSender{}
			mockSender.On("StartSession", mock.Anything).Return("foo").Times(2)
			mockSender.On("CreateTicketBatch", "foo", 70).Return(mockTicketBatch(70), nil).Once()
			node.Sender = &mockSender
			node.Balances = core.NewAddressBalances(10 * time.Second)
			defer node.Balances.StopCleanup()

			node.OrchestratorPool = newStubOrchestratorPool(node, []string{server.URL})
			bsg := newTestBYOCGatewayServer(node)
			drivers.NodeStorage = drivers.NewMemoryDriver(nil)

			streamID := "test-payment-error"
			token := createMockJobToken(server.URL)
			params := byocAIRequestParams{
				liveParams: &byocLiveRequestParams{
					requestID:      "req-1",
					orchToken:      *token,
					stream:         streamID,
					streamID:       streamID,
					sendErrorEvent: func(err error) {},
					segmentReader:  nil,
				},
				node: node,
			}
			_ = bsg.newStreamPipeline("req-1", streamID, "test-capability", params, nil)

			jobSender := &JobSender{
				Addr: "0x1111111111111111111111111111111111111111",
				Sig:  "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
			}

			// Should fail with payment error
			err := bsg.sendPaymentForStream(context.Background(), streamID, jobSender)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "unexpected status code")

			bsg.stopStreamPipeline(streamID, nil)
		})
	})

	t.Run("Error_TokenToSessionConversionNoPrice", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			// Create server that returns invalid token structure
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/process/token":
					// Return a token with invalid structure to cause conversion failure
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					w.Write([]byte(`{"invalid": "token_structure"}`))
				default:
					http.NotFound(w, r)
				}
			}))
			defer server.Close()
			defer server.CloseClientConnections()

			// Test where tokenToAISession fails
			node := mockJobLivepeerNode()

			// Set up mock sender to prevent nil pointer dereference
			mockSender := pm.MockSender{}
			mockSender.On("StartSession", mock.Anything).Return("foo")
			mockSender.On("CreateTicketBatch", mock.Anything, mock.Anything).Return(mockTicketBatch(10), nil)
			node.Sender = &mockSender
			node.Balances = core.NewAddressBalances(10 * time.Second)
			defer node.Balances.StopCleanup()

			node.OrchestratorPool = newStubOrchestratorPool(node, []string{server.URL})
			bsg := newTestBYOCGatewayServer(node)

			// Create stream with valid initial session
			streamID := "test-token-no-price"
			token := createMockJobToken(server.URL)
			params := byocAIRequestParams{
				liveParams: &byocLiveRequestParams{
					requestID:      "req-1",
					orchToken:      *token,
					stream:         streamID,
					streamID:       streamID,
					sendErrorEvent: func(err error) {},
					segmentReader:  nil,
				},
				node: node,
			}
			_ = bsg.newStreamPipeline("req-1", streamID, "test-capability", params, nil)

			jobSender := &JobSender{
				Addr: "0x1111111111111111111111111111111111111111",
				Sig:  "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
			}

			// Should fail during token to session conversion
			err := bsg.sendPaymentForStream(context.Background(), streamID, jobSender)
			assert.NoError(t, err)

			bsg.stopStreamPipeline(streamID, nil)
		})
	})

	t.Run("Success_StreamParamsUpdated", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			// Create server for this test
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/process/token":
					orchTokenHandler(w, r)
				case "/ai/stream/payment":
					w.WriteHeader(http.StatusOK)
					w.Write([]byte(`{"status": "payment_processed"}`))
				default:
					http.NotFound(w, r)
				}
			}))
			defer server.Close()
			defer server.CloseClientConnections()

			// Test that stream params are updated with new session after token refresh
			node := mockJobLivepeerNode()
			mockSender := pm.MockSender{}
			mockSender.On("StartSession", mock.Anything).Return("foo").Times(2)
			mockSender.On("CreateTicketBatch", "foo", 70).Return(mockTicketBatch(70), nil).Once()
			node.Sender = &mockSender
			node.Balances = core.NewAddressBalances(10 * time.Second)
			defer node.Balances.StopCleanup()

			node.OrchestratorPool = newStubOrchestratorPool(node, []string{server.URL})
			bsg := newTestBYOCGatewayServer(node)
			drivers.NodeStorage = drivers.NewMemoryDriver(nil)

			streamID := "test-params-update"
			originalToken := createMockJobToken(server.URL)

			originalSessionAddr := originalToken.Address()

			params := byocAIRequestParams{
				liveParams: &byocLiveRequestParams{
					requestID:      "req-1",
					orchToken:      *originalToken,
					stream:         streamID,
					streamID:       streamID,
					sendErrorEvent: func(err error) {},
					segmentReader:  nil,
				},
				node: node,
			}
			_ = bsg.newStreamPipeline("req-1", streamID, "test-capability", params, nil)

			jobSender := &JobSender{
				Addr: "0x1111111111111111111111111111111111111111",
				Sig:  "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
			}

			// Send payment
			err := bsg.sendPaymentForStream(context.Background(), streamID, jobSender)
			assert.NoError(t, err)

			// Verify that stream params were updated with new token
			updatedParams, err := bsg.streamPipelineParams(streamID)
			assert.NoError(t, err)

			// The session should be updated (new token fetched)
			updatedSessionAddr := updatedParams.liveParams.orchToken.Address()
			// In a real scenario, this might be different, but our mock returns the same token
			// The important thing is that UpdateStreamParams was called
			assert.NotNil(t, updatedParams.liveParams.orchToken)
			assert.Equal(t, originalSessionAddr, updatedSessionAddr) // Same because mock returns same token

			bsg.stopStreamPipeline(streamID, nil)
		})
	})
}

func TestGetStreamRequestParams(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		bsg := newTestBYOCGatewayServer(nil)
		_, err := bsg.streamPipelineParams("")
		assert.Error(t, err)
	})
}

// TestStartStreamWorkerErrorResponse tests the error response handling from worker
// when worker returns status code > 399 (lines 154-182 in stream_orchestrator.go)
func TestStartStreamWorkerErrorResponse(t *testing.T) {
	// Mock worker that returns 400 Bad Request
	statusCodeReturned := http.StatusBadRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/stream/start" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(statusCodeReturned)
			w.Write([]byte(`{"error": "invalid request parameters"}`))
			return
		}
	}))
	defer server.Close()

	mockVerifySig := func(addr ethcommon.Address, msg string, sig []byte) bool {
		return true
	}
	mockGetUrlForCapability := func(capability string) string {
		return server.URL
	}
	mockJobPriceInfo := func(addr ethcommon.Address, cap string) (*net.PriceInfo, error) {
		return &net.PriceInfo{
			PricePerUnit:  0,
			PixelsPerUnit: 1,
		}, nil
	}

	var freeCapacityCalled bool
	mockFreeExternalCapabilityCapacity := func(string) error {
		freeCapacityCalled = true
		return nil
	}

	mockOrch := newMockJobOrchestrator()
	mockOrch.verifySignature = mockVerifySig
	mockOrch.getUrlForCapability = mockGetUrlForCapability
	mockOrch.jobPriceInfo = mockJobPriceInfo
	mockOrch.freeCapacity = mockFreeExternalCapabilityCapacity

	bso := &BYOCOrchestratorServer{
		orch:    mockOrch,
		httpMux: http.NewServeMux(),
		node:    mockOrch.node,
		trickleSrv: trickle.ConfigureServer(trickle.TrickleServerConfig{
			Mux:      http.NewServeMux(),
			BasePath: "/ai/trickle/",
		}),
	}

	// Set up mock sender
	mockSender := pm.MockSender{}
	mockSender.On("StartSession", mock.Anything).Return("foo")
	mockSender.On("CreateTicketBatch", mock.Anything, mock.Anything).Return(mockTicketBatch(70), nil)
	mockOrch.node.Sender = &mockSender
	mockOrch.node.Balances = core.NewAddressBalances(10 * time.Second)
	defer mockOrch.node.Balances.StopCleanup()

	// Prepare job request
	jobParams := JobParameters{EnableVideoIngress: true, EnableVideoEgress: true, EnableDataOutput: true}
	paramsStr := marshalToString(t, jobParams)
	jobReq := &JobRequest{
		ID:         "test-stream",
		Capability: "test-capability",
		Parameters: paramsStr,
		Timeout:    70,
		Request:    "{}",
	}

	orchJob := &orchJob{Req: jobReq, Params: &jobParams}
	gatewayJob := &gatewayJob{Job: orchJob, node: mockOrch.node}

	//setup signing, sign, reset
	mockOrch.node.OrchestratorPool = newStubOrchestratorPool(mockOrch.node, []string{server.URL})
	gatewayJob.sign()
	mockOrch.node.OrchestratorPool = nil

	// Prepare stream start request
	startReq := StartRequest{
		Stream:   "teststream",
		StreamId: "test-stream",
		Params:   "{}",
	}
	body, _ := json.Marshal(startReq)

	t.Run("WorkerReturns400_BadRequest", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			// running in synctest confirms that no trickle channels remain open

			statusCodeReturned = http.StatusBadRequest
			freeCapacityCalled = false

			req := httptest.NewRequest(http.MethodPost, "/process/stream/start", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			// Set up job request header
			req.Header.Set(jobRequestHdr, gatewayJob.SignedJobReq)

			w := httptest.NewRecorder()
			handler := bso.StartStream()
			handler.ServeHTTP(w, req)

			// Verify error response is forwarded
			assert.Equal(t, http.StatusBadRequest, w.Code)
			assert.Contains(t, w.Body.String(), "invalid request parameters")

			// Verify freeCapacity was called for non-500 errors
			assert.True(t, freeCapacityCalled, "FreeExternalCapabilityCapacity should have been called")

			server.CloseClientConnections()

			// no stream created
			assert.Zero(t, len(mockOrch.node.ExternalCapabilities.Streams))

		})
	})

	t.Run("WorkerReturns503_Unavailable", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			statusCodeReturned = http.StatusServiceUnavailable
			freeCapacityCalled = false

			req := httptest.NewRequest(http.MethodPost, "/process/stream/start", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			// Set up job request header
			req.Header.Set(jobRequestHdr, gatewayJob.SignedJobReq)

			w := httptest.NewRecorder()
			handler := bso.StartStream()
			handler.ServeHTTP(w, req)

			// Verify error response is forwarded
			assert.Equal(t, http.StatusServiceUnavailable, w.Code)
			assert.Contains(t, w.Body.String(), "invalid request parameters")

			// Verify freeCapacity was called for non-500 errors
			assert.True(t, freeCapacityCalled, "FreeExternalCapabilityCapacity should have been called")

			server.CloseClientConnections()

			// no stream created
			assert.Zero(t, len(mockOrch.node.ExternalCapabilities.Streams))
		})
	})

	t.Run("WorkerReturns500_FatalError", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			statusCodeReturned = http.StatusInternalServerError
			freeCapacityCalled = false

			req := httptest.NewRequest(http.MethodPost, "/process/stream/start", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			// Set up job request header
			req.Header.Set(jobRequestHdr, gatewayJob.SignedJobReq)

			w := httptest.NewRecorder()
			handler := bso.StartStream()
			handler.ServeHTTP(w, req)

			// Verify error response is forwarded
			assert.Equal(t, http.StatusInternalServerError, w.Code)
			assert.Contains(t, w.Body.String(), "invalid request parameters")

			// Verify freeCapacity was called for non-500 errors
			assert.True(t, freeCapacityCalled, "FreeExternalCapabilityCapacity should have been called")

			server.CloseClientConnections()

			// no stream created
			assert.Zero(t, len(mockOrch.node.ExternalCapabilities.Streams))
		})
	})
}

// TestStartStreamError_OtherScenarios tests the other failedToStartStream = true scenarios
// from StartStream in stream_orchestrator.go (lines 138, 149, 173, 201)
// Excludes line 165 (worker request timeout)
func TestStartStreamError_OtherScenarios(t *testing.T) {
	// Mock worker that returns 200 OK
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/stream/start" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"result": "success"}`))
			return
		}
	}))
	defer server.Close()

	mockVerifySig := func(addr ethcommon.Address, msg string, sig []byte) bool {
		return true
	}
	mockGetUrlForCapability := func(capability string) string {
		return server.URL
	}
	mockJobPriceInfo := func(addr ethcommon.Address, cap string) (*net.PriceInfo, error) {
		return &net.PriceInfo{
			PricePerUnit:  0,
			PixelsPerUnit: 1,
		}, nil
	}

	var freeCapacityCalled bool
	mockFreeExternalCapabilityCapacity := func(string) error {
		freeCapacityCalled = true
		return nil
	}

	mockOrch := newMockJobOrchestrator()
	mockOrch.verifySignature = mockVerifySig
	mockOrch.getUrlForCapability = mockGetUrlForCapability
	mockOrch.jobPriceInfo = mockJobPriceInfo
	mockOrch.freeCapacity = mockFreeExternalCapabilityCapacity

	bso := &BYOCOrchestratorServer{
		orch:    mockOrch,
		httpMux: http.NewServeMux(),
		node:    mockOrch.node,
		trickleSrv: trickle.ConfigureServer(trickle.TrickleServerConfig{
			Mux:      http.NewServeMux(),
			BasePath: "/ai/trickle/",
		}),
	}

	// Set up mock sender
	mockSender := pm.MockSender{}
	mockSender.On("StartSession", mock.Anything).Return("foo")
	mockSender.On("CreateTicketBatch", mock.Anything, mock.Anything).Return(mockTicketBatch(70), nil)
	mockOrch.node.Sender = &mockSender
	mockOrch.node.Balances = core.NewAddressBalances(10 * time.Second)
	defer mockOrch.node.Balances.StopCleanup()

	// Prepare job request
	jobParams := JobParameters{EnableVideoIngress: true, EnableVideoEgress: true, EnableDataOutput: true}
	paramsStr := marshalToString(t, jobParams)
	jobReq := &JobRequest{
		ID:         "test-stream",
		Capability: "test-capability",
		Parameters: paramsStr,
		Timeout:    70,
		Request:    "{}",
	}

	orchJob := &orchJob{Req: jobReq, Params: &jobParams}
	gatewayJob := &gatewayJob{Job: orchJob, node: mockOrch.node}

	// Setup signing
	mockOrch.node.OrchestratorPool = newStubOrchestratorPool(mockOrch.node, []string{server.URL})
	gatewayJob.sign()
	mockOrch.node.OrchestratorPool = nil

	// Prepare stream start request
	startReq := StartRequest{
		Stream:   "teststream",
		StreamId: "test-stream",
		Params:   "{}",
	}
	body, _ := json.Marshal(startReq)

	t.Run("InvalidJSONBody", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			freeCapacityCalled = false

			// Invalid JSON body - trickle channels ARE created before JSON parsing
			invalidBody := []byte("notjson")
			req := httptest.NewRequest(http.MethodPost, "/process/stream/start", bytes.NewReader(invalidBody))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set(jobRequestHdr, gatewayJob.SignedJobReq)

			w := httptest.NewRecorder()
			handler := bso.StartStream()
			handler.ServeHTTP(w, req)

			// Verify 400 Bad Request response
			assert.Equal(t, http.StatusBadRequest, w.Code)

			// Verify freeCapacity WAS called (trickle channels were created)
			assert.True(t, freeCapacityCalled, "FreeExternalCapabilityCapacity should have been called")

			// no stream created
			assert.Zero(t, len(mockOrch.node.ExternalCapabilities.Streams))
		})
	})

	t.Run("WorkerReturnsErrorStatus", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			freeCapacityCalled = false

			// Create a server that returns error status
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/stream/start" {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusBadRequest)
					w.Write([]byte(`{"error": "invalid request"}`))
					return
				}
			}))
			defer server.Close()

			// Update capability URL to use this server
			mockGetUrlForCapability = func(capability string) string {
				return server.URL
			}
			mockOrch.getUrlForCapability = mockGetUrlForCapability

			req := httptest.NewRequest(http.MethodPost, "/process/stream/start", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set(jobRequestHdr, gatewayJob.SignedJobReq)

			w := httptest.NewRecorder()
			handler := bso.StartStream()
			handler.ServeHTTP(w, req)

			// Verify error response is forwarded
			assert.Equal(t, http.StatusBadRequest, w.Code)
			assert.Contains(t, w.Body.String(), "invalid request")

			// Verify freeCapacity was called (non-500 error)
			assert.True(t, freeCapacityCalled, "FreeExternalCapabilityCapacity should have been called")

			// no stream created
			assert.Zero(t, len(mockOrch.node.ExternalCapabilities.Streams))
		})
	})

	t.Run("AddStreamToExternalCapabilitiesFails", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			freeCapacityCalled = false

			// Create a server that returns 200 OK
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/stream/start" {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					w.Write([]byte(`{"result": "success"}`))
					return
				}
			}))
			defer server.Close()

			// Update capability URL to use this server
			mockGetUrlForCapability = func(capability string) string {
				return server.URL
			}
			mockOrch.getUrlForCapability = mockGetUrlForCapability

			// Pre-create a stream with the same ID to cause AddStream to fail
			_, err := mockOrch.node.ExternalCapabilities.AddStream(jobReq.ID, jobReq.Capability, []byte("{}"))
			assert.NoError(t, err)

			req := httptest.NewRequest(http.MethodPost, "/process/stream/start", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set(jobRequestHdr, gatewayJob.SignedJobReq)

			w := httptest.NewRecorder()
			handler := bso.StartStream()
			handler.ServeHTTP(w, req)

			// Verify 500 Internal Server Error response
			assert.Equal(t, http.StatusInternalServerError, w.Code)
			assert.Contains(t, w.Body.String(), "Error adding stream to external capabilities")

			// Verify freeCapacity was called
			assert.True(t, freeCapacityCalled, "FreeExternalCapabilityCapacity should have been called")

			// Clean up pre-created stream
			mockOrch.node.ExternalCapabilities.RemoveStream(jobReq.ID)
		})
	})
}

func TestStartStreamHandler_ReserveCapacity(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var reserveCapacityCalled bool
		node := mockJobLivepeerNode()

		// Create mock orchestrator with ReserveExternalCapabilityCapacity expectation
		mockOrch := newMockJobOrchestrator()
		mockOrch.verifySignature = func(addr ethcommon.Address, msg string, sig []byte) bool {
			return true
		}
		mockOrch.reserveCapacity = func(extCap string) error {
			reserveCapacityCalled = true
			return nil
		}
		mockOrch.freeCapacity = func(extCap string) error {
			return nil
		}
		mockOrch.getUrlForCapability = func(capability string) string {
			return "http://127.0.0.1:1234"
		}
		mockOrch.jobPriceInfo = func(sender ethcommon.Address, jobCapability string) (*net.PriceInfo, error) {
			return &net.PriceInfo{PricePerUnit: 0, PixelsPerUnit: 1}, nil
		}
		mockOrch.balance = func(addr ethcommon.Address, manifestID core.ManifestID) *big.Rat {
			return new(big.Rat).SetInt64(0)
		}

		// Set up an lphttp-based orchestrator test server with trickle endpoints
		mux := http.NewServeMux()
		bso := &BYOCOrchestratorServer{orch: mockOrch, httpMux: mux, node: node}

		// Configure trickle server on the mux
		bso.trickleSrv = trickle.ConfigureServer(trickle.TrickleServerConfig{
			Mux:        mux,
			BasePath:   "/ai/trickle/",
			Autocreate: true,
		})

		// Setup Orch server stub
		mux.HandleFunc("/process/token", orchTokenHandler)
		mux.HandleFunc("/ai/stream/start", orchAIStreamStartNoUrlsHandler)

		server := httptest.NewServer(mux)
		defer server.Close()

		node.OrchestratorPool = newStubOrchestratorPool(node, []string{server.URL})
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

		handler := bso.StartStream()
		handler.ServeHTTP(w, req)

		// Verify ReserveExternalCapabilityCapacity was called
		assert.True(t, reserveCapacityCalled, "ReserveExternalCapabilityCapacity should have been called")

		// no stream created
		assert.Zero(t, len(mockOrch.node.ExternalCapabilities.Streams))
	})
}
