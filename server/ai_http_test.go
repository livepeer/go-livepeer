package server

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/livepeer/go-livepeer/ai/runner"
	"github.com/livepeer/go-livepeer/ai/worker"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/eth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubPriceFeedWatcher struct {
	price eth.PriceData
}

func (s stubPriceFeedWatcher) Currencies() (string, string, error) {
	return "ETH", "USD", nil
}

func (s stubPriceFeedWatcher) Current() (eth.PriceData, error) {
	return s.price, nil
}

func (s stubPriceFeedWatcher) Subscribe(context.Context, chan<- eth.PriceData) {}

func TestAIWorkerResults_ErrorsWhenAuthHeaderMissing(t *testing.T) {
	var l lphttp

	var w = httptest.NewRecorder()
	r, err := http.NewRequest(http.MethodPost, "/aiResults", nil)
	require.NoError(t, err)

	code, body := aiResultsTest(l, w, r)

	require.Equal(t, http.StatusUnauthorized, code)
	require.Contains(t, body, "Unauthorized")
}

func TestAIWorkerResults_ErrorsWhenCredentialsInvalid(t *testing.T) {
	var l lphttp
	l.orchestrator = newStubOrchestrator()
	l.orchestrator.TranscoderSecret()
	var w = httptest.NewRecorder()

	r, err := http.NewRequest(http.MethodPost, "/aiResults", nil)
	require.NoError(t, err)

	r.Header.Set("Authorization", protoVerAIWorker)
	r.Header.Set("Credentials", "BAD CREDENTIALS")

	code, body := aiResultsTest(l, w, r)
	require.Equal(t, http.StatusUnauthorized, code)
	require.Contains(t, body, "invalid secret")
}

func TestAIWorkerResults_ErrorsWhenContentTypeMissing(t *testing.T) {
	var l lphttp
	l.orchestrator = newStubOrchestrator()
	l.orchestrator.TranscoderSecret()
	var w = httptest.NewRecorder()

	r, err := http.NewRequest(http.MethodPost, "/aiResults", nil)
	require.NoError(t, err)

	r.Header.Set("Authorization", protoVerAIWorker)
	r.Header.Set("Credentials", "")

	code, body := aiResultsTest(l, w, r)

	require.Equal(t, http.StatusUnsupportedMediaType, code)
	require.Contains(t, body, "mime: no media type")
}

func TestAIWorkerResults_ErrorsWhenTaskIDMissing(t *testing.T) {
	var l lphttp
	l.orchestrator = newStubOrchestrator()
	l.orchestrator.TranscoderSecret()
	var w = httptest.NewRecorder()

	r, err := http.NewRequest(http.MethodPost, "/aiResults", nil)
	require.NoError(t, err)

	r.Header.Set("Authorization", protoVerAIWorker)
	r.Header.Set("Credentials", "")
	r.Header.Set("Content-Type", "application/json")

	code, body := aiResultsTest(l, w, r)

	require.Equal(t, http.StatusBadRequest, code)
	require.Contains(t, body, "Invalid Task ID")
}

func TestAIWorkerResults_BadRequestType(t *testing.T) {
	httpc := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}}

	assert := assert.New(t)
	assert.Nil(nil)
	resultData := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := io.ReadAll(r.Body)
		assert.NoError(err)
		w.Write([]byte("result binary data"))
	}))
	defer resultData.Close()
	// sending bad request
	notify := createAIJob(742, "text-to-image-invalid", "livepeer/model1", "")

	wkr := stubAIWorker{}
	node, _ := core.NewLivepeerNode(nil, "/tmp/thisdirisnotactuallyusedinthistest", nil)
	node.OrchSecret = "verbigsecret"
	node.AIWorker = &wkr
	node.Capabilities = createStubAIWorkerCapabilitiesForPipelineModelId("text-to-image", "livepeer/model1")

	var headers http.Header
	var body []byte
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		out, err := io.ReadAll(r.Body)
		assert.NoError(err)
		headers = r.Header
		body = out
		w.Write(nil)
	}))
	defer ts.Close()
	parsedURL, _ := url.Parse(ts.URL)
	// send empty request data
	runAIJob(node, parsedURL.Host, httpc, notify)
	time.Sleep(3 * time.Millisecond)

	assert.NotNil(body)
	assert.Equal("742", headers.Get("TaskId"))
	assert.Equal(aiWorkerErrorMimeType, headers.Get("Content-Type"))
	assert.Equal(node.OrchSecret, headers.Get("Credentials"))
	assert.Equal(protoVerAIWorker, headers.Get("Authorization"))
	assert.Equal("AI request validation failed for", string(body)[0:32])
}

func TestLiveRunnerDiscoveryEndpoint(t *testing.T) {
	lp := newLiveRunnerHTTP(t, true)
	manager, ok := lp.liveRunnerManager()
	require.True(t, ok)
	prevWatcher := core.PriceFeedWatcher
	core.PriceFeedWatcher = stubPriceFeedWatcher{price: eth.PriceData{Price: big.NewRat(2000, 1)}}
	defer func() { core.PriceFeedWatcher = prevWatcher }()
	_, err := manager.Heartbeat(runner.LiveRunnerHeartbeatRequest{
		RunnerID:  "runner-1",
		RunnerURL: "https://runner.example.com",
		Version:   "1.2.3",
		Status:    "ready",
		GPU:       &runner.LiveRunnerGPU{Name: "NVIDIA L40S", VRAMMB: 46068},
		App:       "live-video-to-video/scope",
		Capacity:  2,
		InUse:     []string{"job-1"},
		PriceInfo: runner.LiveRunnerPriceInfo{
			PricePerUnit:  10,
			PixelsPerUnit: 1,
			Unit:          "USD",
		},
	})
	require.NoError(t, err)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/discovery", nil)
	lp.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp []liveRunnerDiscoveryEntry
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Len(t, resp, 1)
	require.Len(t, resp[0].Runners, 1)
	require.Equal(t, "https://runner.example.com", resp[0].Runners[0].Endpoint)
	require.Equal(t, "live-video-to-video/scope", resp[0].Runners[0].App)
	require.NotZero(t, resp[0].Runners[0].PriceInfo.PricePerUnit)
}

func TestLiveRunnerDiscoveryServerlessWorker(t *testing.T) {
	lp := newServerlessLiveRunnerHTTP(t, false, 3)
	lp.node.SetBasePriceForCap("default", core.Capability_LiveVideoToVideo, "scope", core.NewFixedPrice(big.NewRat(7, 1)))

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/discovery", nil)
	lp.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp []liveRunnerDiscoveryEntry
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Len(t, resp, 1)
	require.Equal(t, "http://localhost:1234", resp[0].Address)
	require.Len(t, resp[0].Runners, 1)

	discoveryRunner := resp[0].Runners[0]
	require.Equal(t, "http://localhost:1234", discoveryRunner.Endpoint)
	require.NotNil(t, discoveryRunner.GPU)
	require.Equal(t, "H100", discoveryRunner.GPU.Name)
	require.Equal(t, "live-video-to-video/scope", discoveryRunner.App)
	require.Equal(t, "serverless-1.0.0", discoveryRunner.Version)
	require.Equal(t, int64(7), discoveryRunner.PriceInfo.PricePerUnit)
	require.Equal(t, int64(1), discoveryRunner.PriceInfo.PixelsPerUnit)
}

func TestLiveRunnerDiscoveryReturnsHeartbeatAndServerlessRunners(t *testing.T) {
	lp := newServerlessLiveRunnerHTTP(t, true, 2)
	manager, ok := lp.liveRunnerManager()
	require.True(t, ok)
	prevWatcher := core.PriceFeedWatcher
	core.PriceFeedWatcher = stubPriceFeedWatcher{price: eth.PriceData{Price: big.NewRat(2000, 1)}}
	defer func() { core.PriceFeedWatcher = prevWatcher }()
	_, err := manager.Heartbeat(runner.LiveRunnerHeartbeatRequest{
		RunnerID:  "runner-1",
		RunnerURL: "https://runner.example.com",
		Version:   "1.2.3",
		Status:    "ready",
		GPU:       &runner.LiveRunnerGPU{Name: "NVIDIA L40S", VRAMMB: 46068},
		App:       "live-video-to-video/scope",
		Capacity:  1,
		PriceInfo: runner.LiveRunnerPriceInfo{
			PricePerUnit:  10,
			PixelsPerUnit: 1,
			Unit:          "USD",
		},
	})
	require.NoError(t, err)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/discovery", nil)
	lp.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp []liveRunnerDiscoveryEntry
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Len(t, resp, 1)
	require.Len(t, resp[0].Runners, 2)
	require.Equal(t, "https://runner.example.com", resp[0].Runners[0].Endpoint)
	require.Equal(t, "http://localhost:1234", resp[0].Runners[1].Endpoint)
	require.Equal(t, "H100", resp[0].Runners[1].GPU.Name)
}

func TestLiveRunnerHeartbeat(t *testing.T) {
	lp := newLiveRunnerHTTP(t, true)
	body, err := json.Marshal(runner.LiveRunnerHeartbeatRequest{
		RunnerID:  "runner-1",
		RunnerURL: "https://runner.example.com",
		App:       "live-video-to-video/scope",
		Capacity:  1,
		PriceInfo: runner.LiveRunnerPriceInfo{
			PricePerUnit:  10,
			PixelsPerUnit: 1,
			Unit:          "USD",
		},
	})
	require.NoError(t, err)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/runners/heartbeat", bytes.NewReader(body))
	lp.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp runner.LiveRunnerHeartbeatResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Equal(t, "runner-1", resp.RunnerID)
	require.Equal(t, lp.orchestrator.ServiceURI().String(), resp.Orchestrator)
}

func TestLiveRunnerHeartbeatRejectsMissingPriceInfoUnit(t *testing.T) {
	lp := newLiveRunnerHTTP(t, true)
	body := []byte(`{
		"runner_id":"runner-1",
		"runner_url":"https://runner.example.com",
		"price_info":{"price_per_unit":1,"pixels_per_unit":1},
		"app":"live-video-to-video/scope",
		"capacity":1,
		"pricing":{"usd_per_hour":10}
	}`)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/runners/heartbeat", bytes.NewReader(body))
	lp.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), "price_info.unit")
}

func TestLiveRunnerEndpointsUnsupportedWithoutManager(t *testing.T) {
	lp := newLiveRunnerHTTP(t, false)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/discovery", nil)
	lp.ServeHTTP(w, req)
	require.Equal(t, http.StatusNotFound, w.Code)
	require.Contains(t, w.Body.String(), "live runners are not supported")

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/runners/heartbeat", bytes.NewReader([]byte(`{}`)))
	lp.ServeHTTP(w, req)
	require.Equal(t, http.StatusNotFound, w.Code)
	require.Contains(t, w.Body.String(), "live runners are not supported")
}

func TestLiveRunnerDiscoverySupportedWithoutManagerWhenServerlessWorkerPresent(t *testing.T) {
	lp := newServerlessLiveRunnerHTTP(t, false, 1)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/discovery", nil)
	lp.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/runners/heartbeat", bytes.NewReader([]byte(`{}`)))
	lp.ServeHTTP(w, req)
	require.Equal(t, http.StatusNotFound, w.Code)
	require.Contains(t, w.Body.String(), "live runners are not supported")
}

func newLiveRunnerHTTP(t *testing.T, withManager bool) *lphttp {
	t.Helper()
	node, err := core.NewLivepeerNode(nil, t.TempDir(), nil)
	require.NoError(t, err)
	if withManager {
		node.LiveRunnerManager = runner.NewLiveRunnerRegistry()
	}
	return newLiveRunnerHTTPWithNode(t, node)
}

func newServerlessLiveRunnerHTTP(t *testing.T, withManager bool, capacity int) *lphttp {
	t.Helper()
	node, err := core.NewLivepeerNode(nil, t.TempDir(), nil)
	require.NoError(t, err)
	if withManager {
		node.LiveRunnerManager = runner.NewLiveRunnerRegistry()
	}
	serverlessWorker, err := worker.NewServerlessWorker("ws://serverless.example.com/ws", capacity)
	require.NoError(t, err)
	node.AIWorker = serverlessWorker
	node.Capabilities = createStubAIWorkerCapabilitiesForPipelineModelId("live-video-to-video", "scope")
	return newLiveRunnerHTTPWithNode(t, node)
}

func newLiveRunnerHTTPWithNode(t *testing.T, node *core.LivepeerNode) *lphttp {
	t.Helper()
	lp := &lphttp{
		orchestrator: newStubOrchestrator(),
		node:         node,
		transRPC:     http.NewServeMux(),
	}
	require.NoError(t, startAIServer(lp))
	return lp
}
