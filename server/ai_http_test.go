package server

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/golang/protobuf/proto"
	"github.com/livepeer/go-livepeer/ai/runner"
	"github.com/livepeer/go-livepeer/ai/worker"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/eth"
	lpnet "github.com/livepeer/go-livepeer/net"
	"github.com/livepeer/go-livepeer/trickle"
	"github.com/livepeer/go-tools/drivers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubPriceFeedWatcher struct {
	price eth.PriceData
}

type testLiveRunnerHost struct {
	orchestrator  Orchestrator
	liveRunnerURI *url.URL
}

func (h testLiveRunnerHost) ServiceURI() *url.URL {
	return h.orchestrator.ServiceURI()
}

func (h testLiveRunnerHost) LiveRunnerURI() *url.URL {
	if h.liveRunnerURI != nil {
		v := *h.liveRunnerURI
		return &v
	}
	return h.ServiceURI()
}

func (h testLiveRunnerHost) RegistrationSecret() string {
	return h.orchestrator.RegistrationSecret()
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
	liveRunnerAddr, err := url.Parse("http://go-livepeer:8935")
	require.NoError(t, err)
	lp.node.LiveRunnerAddr = liveRunnerAddr
	manager, ok := lp.liveRunnerManager()
	require.True(t, ok)
	_, err = manager.Heartbeat(runner.LiveRunnerHeartbeatRequest{
		RunnerID:  t.Name(),
		RunnerURL: "https://runner.example.com",
		Version:   "1.2.3",
		Metadata:  `{"region":"us-west"}`,
		Status:    "ready",
		GPU:       &runner.LiveRunnerGPU{Name: "NVIDIA L40S", VRAMMB: 46068},
		App:       "live-video-to-video/scope",
		Capacity:  2,
		PriceInfo: runner.LiveRunnerPriceInfo{
			Price: json.Number("10"),
		},
	}, lp.orchestrator.RegistrationSecret())
	require.NoError(t, err)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/discovery", nil)
	lp.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp []liveRunnerDiscoveryEntry
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Len(t, resp, 1)
	require.Len(t, resp[0].Runners, 1)
	require.Equal(t, "http://localhost:1234/apps/"+t.Name()+"/session", resp[0].Runners[0].URL)
	require.Equal(t, "live-video-to-video/scope", resp[0].Runners[0].App)
	require.Equal(t, `{"region":"us-west"}`, resp[0].Runners[0].Metadata)
	require.Equal(t, 2, resp[0].Runners[0].Capacity)
	require.Equal(t, 0, resp[0].Runners[0].CapacityUsed)
	require.Equal(t, 2, resp[0].Runners[0].CapacityAvailable)
	require.NotContains(t, w.Body.String(), "endpoint")
	require.NotContains(t, w.Body.String(), "price_info")
}

func TestLiveRunnerDiscoveryReportsCapacityUsed(t *testing.T) {
	lp := newLiveRunnerHTTP(t, true)
	manager, ok := lp.liveRunnerManager()
	require.True(t, ok)
	_, err := manager.Heartbeat(runner.LiveRunnerHeartbeatRequest{
		RunnerID:  t.Name(),
		RunnerURL: "https://runner.example.com",
		Status:    "ready",
		App:       "live-video-to-video/scope",
		Capacity:  2,
	}, lp.orchestrator.RegistrationSecret())
	require.NoError(t, err)
	_, _, err = manager.ReserveSession(t.Name())
	require.NoError(t, err)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/discovery", nil)
	lp.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp []liveRunnerDiscoveryEntry
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Len(t, resp, 1)
	require.Len(t, resp[0].Runners, 1)
	require.Equal(t, 2, resp[0].Runners[0].Capacity)
	require.Equal(t, 1, resp[0].Runners[0].CapacityUsed)
	require.Equal(t, 1, resp[0].Runners[0].CapacityAvailable)
	require.Contains(t, w.Body.String(), `"capacity":2`)
	require.Contains(t, w.Body.String(), `"capacity_used":1`)
	require.Contains(t, w.Body.String(), `"capacity_available":1`)
}

func TestLiveRunnerDiscoverySingleShotReturnsProxiedURL(t *testing.T) {
	lp := newLiveRunnerHTTP(t, true)
	manager, ok := lp.liveRunnerManager()
	require.True(t, ok)
	_, err := manager.Heartbeat(runner.LiveRunnerHeartbeatRequest{
		RunnerID:  t.Name(),
		RunnerURL: "https://runner.example.com",
		Status:    "ready",
		Mode:      runner.LiveRunnerModeSingleShot,
		App:       "live-video-to-video/scope",
		Capacity:  1,
	}, lp.orchestrator.RegistrationSecret())
	require.NoError(t, err)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/discovery", nil)
	lp.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp []liveRunnerDiscoveryEntry
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Len(t, resp, 1)
	require.Len(t, resp[0].Runners, 1)
	require.Equal(t, "http://localhost:1234/apps/"+t.Name()+"/app", resp[0].Runners[0].URL)
	require.Equal(t, runner.LiveRunnerModeSingleShot, resp[0].Runners[0].Mode)
	require.Equal(t, 1, resp[0].Runners[0].Capacity)
	require.Equal(t, 0, resp[0].Runners[0].CapacityUsed)
	require.Equal(t, 1, resp[0].Runners[0].CapacityAvailable)
	require.NotContains(t, w.Body.String(), "endpoint")
	require.NotContains(t, w.Body.String(), "price_info")
}

func TestLiveRunnerDiscoveryOnchainIncludesPriceInfo(t *testing.T) {
	lp := newLiveRunnerHTTP(t, true)
	manager := runner.NewLiveRunnerRegistry(runner.LiveRunnerRegistryConfig{Host: testLiveRunnerHost{orchestrator: lp.orchestrator}, Onchain: true})
	t.Cleanup(manager.Stop)
	lp.node.LiveRunnerManager = manager
	trickleBaseURL := lp.orchestrator.ServiceURI().JoinPath(TrickleHTTPPath).String()
	manager.SetTrickleServer(lp.trickleSrv, trickleBaseURL, trickleBaseURL)
	prevWatcher := core.PriceFeedWatcher
	core.PriceFeedWatcher = stubPriceFeedWatcher{price: eth.PriceData{Price: big.NewRat(2000, 1)}}
	defer func() { core.PriceFeedWatcher = prevWatcher }()
	_, err := manager.Heartbeat(runner.LiveRunnerHeartbeatRequest{
		RunnerID:  t.Name(),
		RunnerURL: "https://runner.example.com",
		Version:   "1.2.3",
		Status:    "ready",
		App:       "live-video-to-video/scope",
		Capacity:  2,
		PriceInfo: runner.LiveRunnerPriceInfo{
			Price: json.Number("10"),
		},
	}, lp.orchestrator.RegistrationSecret())
	require.NoError(t, err)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/discovery", nil)
	lp.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp []liveRunnerDiscoveryEntry
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Len(t, resp, 1)
	require.Len(t, resp[0].Runners, 1)
	require.Equal(t, "http://localhost:1234/apps/"+t.Name()+"/session", resp[0].Runners[0].URL)
	require.Equal(t, 2, resp[0].Runners[0].Capacity)
	require.Equal(t, 0, resp[0].Runners[0].CapacityUsed)
	require.Equal(t, 2, resp[0].Runners[0].CapacityAvailable)
	require.NotNil(t, resp[0].Runners[0].PriceInfo)
	require.Equal(t, "1388888888888", resp[0].Runners[0].PriceInfo.Price.String())
	require.Equal(t, "wei", resp[0].Runners[0].PriceInfo.Currency)
	require.Equal(t, "seconds", resp[0].Runners[0].PriceInfo.Unit)
	require.Contains(t, w.Body.String(), "price_info")
}

func TestLiveRunnerDiscoveryOnchainConverts720pPriceToPixel(t *testing.T) {
	lp := newLiveRunnerHTTP(t, true)
	manager := runner.NewLiveRunnerRegistry(runner.LiveRunnerRegistryConfig{Host: testLiveRunnerHost{orchestrator: lp.orchestrator}, Onchain: true})
	t.Cleanup(manager.Stop)
	lp.node.LiveRunnerManager = manager
	trickleBaseURL := lp.orchestrator.ServiceURI().JoinPath(TrickleHTTPPath).String()
	manager.SetTrickleServer(lp.trickleSrv, trickleBaseURL, trickleBaseURL)
	prevWatcher := core.PriceFeedWatcher
	core.PriceFeedWatcher = stubPriceFeedWatcher{price: eth.PriceData{Price: big.NewRat(2000, 1)}}
	defer func() { core.PriceFeedWatcher = prevWatcher }()
	_, err := manager.Heartbeat(runner.LiveRunnerHeartbeatRequest{
		RunnerID:  t.Name(),
		RunnerURL: "https://runner.example.com",
		Version:   "1.2.3",
		Status:    "ready",
		App:       "live-video-to-video/scope",
		Capacity:  2,
		PriceInfo: runner.LiveRunnerPriceInfo{
			Price: json.Number("0.001990656"),
			Unit:  "720p",
		},
	}, lp.orchestrator.RegistrationSecret())
	require.NoError(t, err)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/discovery", nil)
	lp.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp []liveRunnerDiscoveryEntry
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Len(t, resp, 1)
	require.Len(t, resp[0].Runners, 1)
	require.NotNil(t, resp[0].Runners[0].PriceInfo)
	require.Equal(t, "10", resp[0].Runners[0].PriceInfo.Price.String())
	require.Equal(t, "wei", resp[0].Runners[0].PriceInfo.Currency)
	require.Equal(t, "720p-pixel-seconds", resp[0].Runners[0].PriceInfo.Unit)
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
	require.Equal(t, "http://localhost:1234/scope", discoveryRunner.URL)
	require.NotNil(t, discoveryRunner.GPU)
	require.Equal(t, "H100", discoveryRunner.GPU.Name)
	require.Equal(t, "live-video-to-video/scope", discoveryRunner.App)
	require.Equal(t, "serverless-1.0.0", discoveryRunner.Version)
	require.Equal(t, 3, discoveryRunner.Capacity)
	require.Equal(t, 0, discoveryRunner.CapacityUsed)
	require.Equal(t, 3, discoveryRunner.CapacityAvailable)
	require.NotNil(t, discoveryRunner.PriceInfo)
	require.Equal(t, "7", discoveryRunner.PriceInfo.Price.String())
	require.Equal(t, "wei", discoveryRunner.PriceInfo.Currency)
	require.Equal(t, "720p-pixel-seconds", discoveryRunner.PriceInfo.Unit)
}

func TestLiveRunnerDiscoveryReturnsHeartbeatAndServerlessRunners(t *testing.T) {
	lp := newServerlessLiveRunnerHTTP(t, true, 2)
	manager, ok := lp.liveRunnerManager()
	require.True(t, ok)
	prevWatcher := core.PriceFeedWatcher
	core.PriceFeedWatcher = stubPriceFeedWatcher{price: eth.PriceData{Price: big.NewRat(2000, 1)}}
	defer func() { core.PriceFeedWatcher = prevWatcher }()
	_, err := manager.Heartbeat(runner.LiveRunnerHeartbeatRequest{
		RunnerID:  t.Name(),
		RunnerURL: "https://runner.example.com",
		Version:   "1.2.3",
		Status:    "ready",
		GPU:       &runner.LiveRunnerGPU{Name: "NVIDIA L40S", VRAMMB: 46068},
		App:       "live-video-to-video/scope",
		Capacity:  1,
		PriceInfo: runner.LiveRunnerPriceInfo{
			Price: json.Number("10"),
		},
	}, lp.orchestrator.RegistrationSecret())
	require.NoError(t, err)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/discovery", nil)
	lp.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp []liveRunnerDiscoveryEntry
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Len(t, resp, 1)
	require.Len(t, resp[0].Runners, 2)
	require.Equal(t, "http://localhost:1234/apps/"+t.Name()+"/session", resp[0].Runners[0].URL)
	require.Equal(t, 1, resp[0].Runners[0].Capacity)
	require.Equal(t, 0, resp[0].Runners[0].CapacityUsed)
	require.Equal(t, 1, resp[0].Runners[0].CapacityAvailable)
	require.Equal(t, "http://localhost:1234/scope", resp[0].Runners[1].URL)
	require.Equal(t, "H100", resp[0].Runners[1].GPU.Name)
	require.Equal(t, 2, resp[0].Runners[1].Capacity)
	require.Equal(t, 0, resp[0].Runners[1].CapacityUsed)
	require.Equal(t, 2, resp[0].Runners[1].CapacityAvailable)
}

func TestLiveRunnerHeartbeat(t *testing.T) {
	lp := newLiveRunnerHTTP(t, true)
	body, err := json.Marshal(runner.LiveRunnerHeartbeatRequest{
		RunnerID:  t.Name(),
		RunnerURL: "https://runner.example.com",
		App:       "live-video-to-video/scope",
		Capacity:  3,
		PriceInfo: runner.LiveRunnerPriceInfo{
			Price: json.Number("10"),
		},
		SessionIDs: []string{"runner-reported-session"},
	})
	require.NoError(t, err)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/runners/heartbeat", bytes.NewReader(body))
	req.Header.Set("Authorization", lp.orchestrator.RegistrationSecret())
	lp.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp runner.LiveRunnerHeartbeatResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Equal(t, t.Name(), resp.RunnerID)
	require.Equal(t, lp.orchestrator.ServiceURI().String(), resp.Orchestrator)
	require.NotEmpty(t, resp.HeartbeatSecret)
	require.Empty(t, resp.SessionIDs)
	require.NotNil(t, resp.O2R)
	require.Equal(t, runner.LiveRunnerTrickleOrchestratorToRunner, resp.O2R.Name)
	for _, channel := range []runner.LiveRunnerTrickleChannel{*resp.O2R} {
		require.True(t, strings.HasPrefix(channel.ChannelName, resp.RunnerID+"-"))
		require.True(t, strings.HasSuffix(channel.ChannelName, "-"+channel.Name))
		require.NotEqual(t, resp.RunnerID+"-"+channel.Name, channel.ChannelName)
		require.Equal(t, "application/json", channel.MimeType)
		require.Equal(t, "http://localhost:1234/ai/trickle/"+channel.ChannelName, channel.URL)

		w = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodGet, "/ai/trickle/"+channel.ChannelName+"/next", nil)
		lp.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code)
	}

	// Check missing auth after bootstrap.
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/runners/heartbeat", bytes.NewReader(body))
	lp.ServeHTTP(w, req)
	require.Equal(t, http.StatusUnauthorized, w.Code)

	manager, ok := lp.liveRunnerManager()
	require.True(t, ok)
	_, _, err = manager.ReserveSession(t.Name(), "session-z-oldest")
	require.NoError(t, err)
	time.Sleep(time.Millisecond)
	_, _, err = manager.ReserveSession(t.Name(), "session-a-newest")
	require.NoError(t, err)

	// Check follow-up heartbeat auth.
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/runners/heartbeat", bytes.NewReader(body))
	req.Header.Set("Authorization", resp.HeartbeatSecret)
	lp.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	var nextResp runner.LiveRunnerHeartbeatResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &nextResp))
	require.Equal(t, t.Name(), nextResp.RunnerID)
	require.Empty(t, nextResp.HeartbeatSecret)
	require.Nil(t, nextResp.O2R)
	require.Equal(t, []string{"session-z-oldest", "session-a-newest"}, nextResp.SessionIDs)
}

func TestLiveRunnerHeartbeatUsesLiveRunnerAddr(t *testing.T) {
	node, err := core.NewLivepeerNode(nil, t.TempDir(), nil)
	require.NoError(t, err)
	node.LiveRunnerAddr, err = url.Parse("http://go-livepeer:8935")
	require.NoError(t, err)
	orch := newStubOrchestrator()
	orch.serviceURI = "https://public.example.com"
	orch.secret = "live-runner-secret"
	lp := &lphttp{
		orchestrator: orch,
		node:         node,
		transRPC:     http.NewServeMux(),
	}
	manager := runner.NewLiveRunnerRegistry(runner.LiveRunnerRegistryConfig{
		Host: testLiveRunnerHost{orchestrator: lp.orchestrator, liveRunnerURI: node.LiveRunnerAddr},
	})
	t.Cleanup(manager.Stop)
	node.LiveRunnerManager = manager
	require.NoError(t, startAIServer(lp))

	body, err := json.Marshal(runner.LiveRunnerHeartbeatRequest{
		RunnerID:  t.Name(),
		RunnerURL: "https://runner.example.com",
		App:       "live-video-to-video/scope",
		Capacity:  1,
		PriceInfo: runner.LiveRunnerPriceInfo{
			Price: json.Number("10"),
		},
	})
	require.NoError(t, err)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/runners/heartbeat", bytes.NewReader(body))
	req.Header.Set("Authorization", lp.orchestrator.RegistrationSecret())
	lp.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp runner.LiveRunnerHeartbeatResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Equal(t, "http://go-livepeer:8935", resp.Orchestrator)
}

func TestLiveRunnerHeartbeatDefaultsMissingPriceInfoCurrencyAndUnit(t *testing.T) {
	lp := newLiveRunnerHTTPOnchain(t)
	body := []byte(fmt.Sprintf(`{
		"runner_id":%q,
		"runner_url":"https://runner.example.com",
		"price_info":{"price":1},
		"app":"live-video-to-video/scope",
		"capacity":1
	}`, t.Name()))

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/runners/heartbeat", bytes.NewReader(body))
	req.Header.Set("Authorization", lp.orchestrator.RegistrationSecret())
	lp.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
}

func TestLiveRunnerHeartbeatRejectsMalformedUTF8Metadata(t *testing.T) {
	lp := newLiveRunnerHTTP(t, true)
	body := []byte(fmt.Sprintf(`{
		"runner_id":%q,
		"runner_url":"https://runner.example.com",
		"metadata":"invalid-`, t.Name()))
	body = append(body, 0xff)
	body = append(body, []byte(`",
		"app":"live-video-to-video/scope",
		"capacity":1
	}`)...)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/runners/heartbeat", bytes.NewReader(body))
	req.Header.Set("Authorization", lp.orchestrator.RegistrationSecret())
	lp.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), "U+FFFD")
	manager, ok := lp.liveRunnerManager()
	require.True(t, ok)
	require.Empty(t, manager.Runners())
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

func TestLiveRunnerReserveSession(t *testing.T) {
	lp := newLiveRunnerHTTP(t, true)
	registerLiveRunnerForSession(t, lp, nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/apps/runner-1/session", nil)
	lp.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp liveRunnerSessionResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.NotEmpty(t, resp.SessionID)
	require.Equal(t, "http://localhost:1234/apps/runner-1/session/"+resp.SessionID+"/app", resp.AppURL)
	require.Equal(t, "http://localhost:1234/apps/runner-1/session/"+resp.SessionID, resp.ControlURL)
}

func TestLiveRunnerReserveSessionRejectsSingleShotRunner(t *testing.T) {
	lp := newLiveRunnerHTTP(t, true)
	registerLiveRunnerForSession(t, lp, &liveRunnerRegistrationOptions{
		RunnerURL: "https://private-runner.example.com",
		Mode:      runner.LiveRunnerModeSingleShot,
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/apps/runner-1/session", nil)
	lp.ServeHTTP(w, req)
	require.Equal(t, http.StatusBadRequest, w.Code)
	require.NotContains(t, w.Body.String(), "private-runner.example.com")
	manager := lp.node.LiveRunnerManager.(*runner.LiveRunnerRegistry)
	require.Equal(t, 0, manager.Runners()[0].CapacityUsed)
}

func TestLiveRunnerReserveSessionReturnsAutomaticAppProxy(t *testing.T) {
	var gotPath, gotQuery, gotRunnerRoute, gotSessionID, gotSessionToken string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		gotRunnerRoute = r.Header.Get("Livepeer-Runner-Route")
		gotSessionID = r.Header.Get("Livepeer-Session-Id")
		gotSessionToken = r.Header.Get("Livepeer-Session-Token")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte("proxied"))
	}))
	defer upstream.Close()

	lp := newLiveRunnerHTTP(t, true)
	registerLiveRunnerForSession(t, lp, &liveRunnerRegistrationOptions{
		RunnerURL: upstream.URL + "/base",
		Capacity:  2,
		Mode:      runner.LiveRunnerModePersistent,
		Proxy:     true,
	})
	manager := lp.node.LiveRunnerManager.(*runner.LiveRunnerRegistry)
	require.Equal(t, "http://localhost:1234/apps/runner-1/session", manager.Runners()[0].URL)

	reserve := func() liveRunnerSessionResponse {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/apps/runner-1/session", nil)
		lp.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code)
		var resp liveRunnerSessionResponse
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		return resp
	}
	first := reserve()
	second := reserve()
	require.NotEmpty(t, first.SessionID)
	require.NotEmpty(t, second.SessionID)
	require.NotEqual(t, first.SessionID, second.SessionID)
	require.Contains(t, first.AppURL, "http://localhost:1234/run/")
	require.Contains(t, second.AppURL, "http://localhost:1234/run/")
	require.NotEqual(t, first.AppURL, second.AppURL)
	require.NotContains(t, first.AppURL, first.SessionID)
	require.Equal(t, "http://localhost:1234/apps/runner-1/session/"+first.SessionID, first.ControlURL)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, first.AppURL+"/foo/bar?x=1", strings.NewReader("body"))
	lp.ServeHTTP(w, req)
	require.Equal(t, http.StatusAccepted, w.Code)
	require.Equal(t, "proxied", w.Body.String())
	require.Equal(t, "/base/foo/bar", gotPath)
	require.Equal(t, "x=1", gotQuery)
	require.Equal(t, "runner-1", gotRunnerRoute)
	require.Equal(t, first.SessionID, gotSessionID)
	require.NotEmpty(t, gotSessionToken)

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/apps/runner-1/session/"+first.SessionID+"/stop", nil)
	lp.ServeHTTP(w, req)
	require.Equal(t, http.StatusNoContent, w.Code)
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, first.AppURL+"/after-stop", nil)
	lp.ServeHTTP(w, req)
	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestLiveRunnerReserveSessionOnchainReturnsPaymentChallenge(t *testing.T) {
	lp := newLiveRunnerHTTPOnchain(t)
	registerLiveRunnerForSession(t, lp, nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/apps/runner-1/session", nil)
	setRequestHeaders(req, liveRunnerSenderHeaders(lp.orchestrator.(*stubOrchestrator)))
	lp.ServeHTTP(w, req)

	require.Equal(t, http.StatusPaymentRequired, w.Code)
	challenge, oInfo := decodeLiveRunnerPaymentChallenge(t, w.Body.Bytes())
	require.NotEmpty(t, challenge.PaymentParams)
	require.Equal(t, lp.orchestrator.ServiceURI().String(), challenge.Orchestrator)
	require.NotEmpty(t, challenge.ManifestID)
	require.Equal(t, challenge.ManifestID, oInfo.GetAuthToken().GetSessionId())
	require.NotNil(t, oInfo.GetTicketParams())
	require.NotNil(t, oInfo.GetPriceInfo())
	require.Equal(t, int64(10), oInfo.GetPriceInfo().GetPricePerUnit())
	require.Equal(t, int64(1), oInfo.GetPriceInfo().GetPixelsPerUnit())
	require.Equal(t, challenge.Orchestrator, oInfo.GetTranscoder())
	require.Nil(t, oInfo.GetCapabilities())
}

func TestRunnerOrchInfoUsesWeiSecondsPrice(t *testing.T) {
	lp := newLiveRunnerHTTPOnchain(t)
	orch := lp.orchestrator.(*stubOrchestrator)

	oInfo, err := lp.runnerOrchInfo(orch.Address(), &runner.LiveRunnerPriceInfo{
		Price:    json.Number("10"),
		Currency: "wei",
		Unit:     "seconds",
	})

	require.NoError(t, err)
	require.NotNil(t, oInfo.GetPriceInfo())
	require.Equal(t, int64(10), oInfo.GetPriceInfo().GetPricePerUnit())
	require.Equal(t, int64(1), oInfo.GetPriceInfo().GetPixelsPerUnit())
}

func TestLiveRunnerReserveSessionOnchainAcceptsPaidReservation(t *testing.T) {
	lp := newLiveRunnerHTTPOnchain(t)
	registerLiveRunnerForSession(t, lp, nil)

	challenge, oInfo := requestLiveRunnerPaymentChallenge(t, lp, "runner-1")
	orch := lp.orchestrator.(*stubOrchestrator)
	headers := liveRunnerReservationPaymentHeaders(t, orch, oInfo.GetAuthToken(), challenge.ManifestID)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/apps/runner-1/session", nil)
	setRequestHeaders(req, headers)
	lp.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp liveRunnerSessionResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Equal(t, challenge.ManifestID, resp.SessionID)
	require.Equal(t, "http://localhost:1234/apps/runner-1/session/"+resp.SessionID+"/app", resp.AppURL)
	require.Equal(t, "http://localhost:1234/apps/runner-1/session/"+resp.SessionID, resp.ControlURL)
}

func TestLiveRunnerSessionPaymentAcceptsPayment(t *testing.T) {
	oldStorage := drivers.NodeStorage
	drivers.NodeStorage = drivers.NewMemoryDriver(nil)
	t.Cleanup(func() { drivers.NodeStorage = oldStorage })

	lp := newLiveRunnerHTTPOnchain(t)
	registerLiveRunnerForSession(t, lp, nil)

	orch := lp.orchestrator.(*stubOrchestrator)
	orch.balances = make(map[ethcommon.Address]map[core.ManifestID]*big.Rat)
	orch.paymentCredit = big.NewRat(100, 1)

	challenge, oInfo := requestLiveRunnerPaymentChallenge(t, lp, "runner-1")
	headers := liveRunnerReservationPaymentHeaders(t, orch, oInfo.GetAuthToken(), challenge.ManifestID)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/apps/runner-1/session", nil)
	setRequestHeaders(req, headers)
	lp.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/apps/runner-1/session/"+challenge.ManifestID+"/payment", nil)
	setRequestHeaders(req, headers)
	lp.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var paymentResult lpnet.PaymentResult
	require.NoError(t, proto.Unmarshal(w.Body.Bytes(), &paymentResult))
	require.NotNil(t, paymentResult.GetInfo())
	require.Equal(t, challenge.ManifestID, paymentResult.GetInfo().GetAuthToken().GetSessionId())

	balance := orch.Balance(orch.Address(), core.ManifestID(challenge.ManifestID))
	require.NotNil(t, balance)
	require.Equal(t, "200", balance.FloatString(0))
}

func TestLiveRunnerSessionPaymentRejectsMissingSession(t *testing.T) {
	lp := newLiveRunnerHTTPOnchain(t)
	registerLiveRunnerForSession(t, lp, nil)

	challenge, oInfo := requestLiveRunnerPaymentChallenge(t, lp, "runner-1")
	headers := liveRunnerReservationPaymentHeaders(t, lp.orchestrator.(*stubOrchestrator), oInfo.GetAuthToken(), challenge.ManifestID)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/apps/runner-1/session/missing/payment", nil)
	setRequestHeaders(req, headers)
	lp.ServeHTTP(w, req)

	require.Equal(t, http.StatusNotFound, w.Code)
	require.Contains(t, w.Body.String(), "runner session not found")
}

func TestLiveRunnerSessionPaymentRejectsReleasedSession(t *testing.T) {
	lp := newLiveRunnerHTTPOnchain(t)
	registerLiveRunnerForSession(t, lp, nil)

	challenge, oInfo := requestLiveRunnerPaymentChallenge(t, lp, "runner-1")
	headers := liveRunnerReservationPaymentHeaders(t, lp.orchestrator.(*stubOrchestrator), oInfo.GetAuthToken(), challenge.ManifestID)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/apps/runner-1/session", nil)
	setRequestHeaders(req, headers)
	lp.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/apps/runner-1/session/"+challenge.ManifestID+"/stop", nil)
	lp.ServeHTTP(w, req)
	require.Equal(t, http.StatusNoContent, w.Code)

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/apps/runner-1/session/"+challenge.ManifestID+"/payment", nil)
	setRequestHeaders(req, headers)
	lp.ServeHTTP(w, req)

	require.Equal(t, http.StatusNotFound, w.Code)
	require.Contains(t, w.Body.String(), "runner session not found")
}

func TestLiveRunnerSessionPaymentRejectsManifestMismatch(t *testing.T) {
	lp := newLiveRunnerHTTPOnchain(t)
	registerLiveRunnerForSession(t, lp, &liveRunnerRegistrationOptions{Capacity: 2})

	sessionID := reservePaidLiveRunnerSession(t, lp, "runner-1", &lpnet.PriceInfo{PricePerUnit: 10, PixelsPerUnit: 1})

	_, oInfo := requestLiveRunnerPaymentChallenge(t, lp, "runner-1")
	headers := liveRunnerReservationPaymentHeaders(t, lp.orchestrator.(*stubOrchestrator), oInfo.GetAuthToken(), "different-manifest")

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/apps/runner-1/session/"+sessionID+"/payment", nil)
	setRequestHeaders(req, headers)
	lp.ServeHTTP(w, req)

	require.Equal(t, http.StatusForbidden, w.Code)
	require.Contains(t, w.Body.String(), "mismatched session and payment manifest")
}

func TestLiveRunnerSessionPaymentRejectsBadPaymentHeader(t *testing.T) {
	lp := newLiveRunnerHTTPOnchain(t)
	registerLiveRunnerForSession(t, lp, nil)

	sessionID := reservePaidLiveRunnerSession(t, lp, "runner-1", &lpnet.PriceInfo{PricePerUnit: 10, PixelsPerUnit: 1})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/apps/runner-1/session/"+sessionID+"/payment", nil)
	req.Header.Set(paymentHeader, "not-base64")
	lp.ServeHTTP(w, req)

	require.Equal(t, http.StatusPaymentRequired, w.Code)
	require.Contains(t, w.Body.String(), "base64 decode error")
}

func TestLiveRunnerPaidSessionMonitorDebitsBalance(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		lp := newLiveRunnerHTTPOnchain(t)
		lp.node.LivePaymentInterval = time.Second
		registerLiveRunnerForSession(t, lp, nil)
		manager := lp.node.LiveRunnerManager.(*runner.LiveRunnerRegistry)
		defer manager.Stop()

		orch := lp.orchestrator.(*stubOrchestrator)
		orch.balances = make(map[ethcommon.Address]map[core.ManifestID]*big.Rat)
		orch.paymentCredit = big.NewRat(3, 1)
		priceInfo := liveRunnerTestPricePerSecond(1)

		sessionID := reservePaidLiveRunnerSession(t, lp, "runner-1", priceInfo)
		balance := orch.Balance(orch.Address(), core.ManifestID(sessionID))
		require.NotNil(t, balance)
		require.Equal(t, "3", balance.FloatString(0))

		time.Sleep(time.Second)
		synctest.Wait()

		balance = orch.Balance(orch.Address(), core.ManifestID(sessionID))
		require.NotNil(t, balance)
		require.Equal(t, "2", balance.FloatString(0))

		require.NoError(t, manager.ReleaseSession("runner-1", sessionID))
		time.Sleep(time.Second)
		synctest.Wait()
	})
}

func TestPreparePaymentProcessor(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tests := []struct {
		unit          string
		expectedUnits int64
	}{
		{unit: "seconds", expectedUnits: 1},
		{unit: "720p-pixel-seconds", expectedUnits: 1280 * 720 * 30},
	}
	for _, tt := range tests {
		t.Run(tt.unit, func(t *testing.T) {
			newPaymentProcessor, err := preparePaymentProcessor(tt.unit)
			require.NoError(t, err)
			processor := newPaymentProcessor(ctx, time.Second, func(int64) error { return nil })
			require.Equal(t, tt.expectedUnits, processor.units)
		})
	}

	_, err := preparePaymentProcessor("minutes")
	require.ErrorContains(t, err, "unsupported live runner payment unit")
}

func TestLiveRunnerPaidSessionMonitorDebitsSecondsBalance(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		lp := newLiveRunnerHTTPOnchain(t)
		lp.node.LivePaymentInterval = time.Second
		registerLiveRunnerForSession(t, lp, &liveRunnerRegistrationOptions{PriceUnit: "hour"})
		manager := lp.node.LiveRunnerManager.(*runner.LiveRunnerRegistry)
		defer manager.Stop()

		orch := lp.orchestrator.(*stubOrchestrator)
		orch.balances = make(map[ethcommon.Address]map[core.ManifestID]*big.Rat)
		orch.paymentCredit = big.NewRat(3, 1)

		sessionID := reservePaidLiveRunnerSession(t, lp, "runner-1", liveTestPricePerSecond(1))
		balance := orch.Balance(orch.Address(), core.ManifestID(sessionID))
		require.NotNil(t, balance)
		require.Equal(t, "3", balance.FloatString(0))

		time.Sleep(time.Second)
		synctest.Wait()

		balance = orch.Balance(orch.Address(), core.ManifestID(sessionID))
		require.NotNil(t, balance)
		require.Equal(t, "2", balance.FloatString(0))

		require.NoError(t, manager.ReleaseSession("runner-1", sessionID))
		time.Sleep(time.Second)
		synctest.Wait()
	})
}

func TestLiveRunnerPaidSessionMonitorReleasesOnInsufficientBalance(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		lp := newLiveRunnerHTTPOnchain(t)
		lp.node.LivePaymentInterval = time.Second
		registerLiveRunnerForSession(t, lp, nil)
		manager := lp.node.LiveRunnerManager.(*runner.LiveRunnerRegistry)
		defer manager.Stop()

		orch := lp.orchestrator.(*stubOrchestrator)
		orch.balances = make(map[ethcommon.Address]map[core.ManifestID]*big.Rat)
		orch.paymentCredit = big.NewRat(0, 1)

		sessionID := reservePaidLiveRunnerSession(t, lp, "runner-1", liveRunnerTestPricePerSecond(1))

		time.Sleep(time.Second)
		synctest.Wait()

		_, err := manager.RunnerEndpointForSession("runner-1", sessionID)
		var runnerErr *runner.RunnerError
		require.True(t, errors.As(err, &runnerErr), "expected runner error, got %v", err)
		require.Equal(t, http.StatusNotFound, runnerErr.StatusCode)
	})
}

func TestLiveRunnerPaidSessionMonitorExitsAfterManualStop(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		lp := newLiveRunnerHTTPOnchain(t)
		lp.node.LivePaymentInterval = time.Second
		registerLiveRunnerForSession(t, lp, nil)
		manager := lp.node.LiveRunnerManager.(*runner.LiveRunnerRegistry)
		defer manager.Stop()

		orch := lp.orchestrator.(*stubOrchestrator)
		orch.balances = make(map[ethcommon.Address]map[core.ManifestID]*big.Rat)
		orch.paymentCredit = big.NewRat(3, 1)

		sessionID := reservePaidLiveRunnerSession(t, lp, "runner-1", liveRunnerTestPricePerSecond(1))

		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/apps/runner-1/session/"+sessionID+"/stop", nil)
		lp.ServeHTTP(w, req)
		require.Equal(t, http.StatusNoContent, w.Code)

		time.Sleep(time.Second)
		synctest.Wait()

		balance := orch.Balance(orch.Address(), core.ManifestID(sessionID))
		require.NotNil(t, balance)
		require.Equal(t, "3", balance.FloatString(0))
	})
}

func TestLiveRunnerOffchainSessionDoesNotStartPaymentMonitor(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		lp := newLiveRunnerHTTP(t, true)
		lp.node.LivePaymentInterval = time.Second
		registerLiveRunnerForSession(t, lp, nil)
		manager := lp.node.LiveRunnerManager.(*runner.LiveRunnerRegistry)
		defer manager.Stop()

		sessionID := reserveLiveRunnerSession(t, lp, "runner-1")

		time.Sleep(time.Second)
		synctest.Wait()

		if _, err := manager.RunnerEndpointForSession("runner-1", sessionID); err != nil {
			t.Fatal(err)
		}
		require.NoError(t, manager.ReleaseSession("runner-1", sessionID))
	})
}

func TestLiveRunnerReserveSessionRejectsManifestAuthMismatch(t *testing.T) {
	lp := newLiveRunnerHTTPOnchain(t)
	registerLiveRunnerForSession(t, lp, nil)

	_, oInfo := requestLiveRunnerPaymentChallenge(t, lp, "runner-1")
	orch := lp.orchestrator.(*stubOrchestrator)
	headers := liveRunnerReservationPaymentHeaders(t, orch, oInfo.GetAuthToken(), "different-manifest")

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/apps/runner-1/session", nil)
	setRequestHeaders(req, headers)
	lp.ServeHTTP(w, req)

	require.Equal(t, http.StatusForbidden, w.Code)
	require.Contains(t, w.Body.String(), "mismatched manifest and auth token")
}

func TestLiveRunnerReserveSessionOnchainUsesPublicServiceURIForPaymentChallenge(t *testing.T) {
	lp := newLiveRunnerHTTPOnchain(t)
	liveRunnerAddr, err := url.Parse("http://go-livepeer:8935")
	require.NoError(t, err)
	lp.node.LiveRunnerAddr = liveRunnerAddr
	lp.orchestrator.(*stubOrchestrator).serviceURI = "https://public.example.com"
	registerLiveRunnerForSession(t, lp, nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/apps/runner-1/session", nil)
	setRequestHeaders(req, liveRunnerSenderHeaders(lp.orchestrator.(*stubOrchestrator)))
	lp.ServeHTTP(w, req)

	require.Equal(t, http.StatusPaymentRequired, w.Code)
	challenge, oInfo := decodeLiveRunnerPaymentChallenge(t, w.Body.Bytes())
	require.Equal(t, "https://public.example.com", challenge.Orchestrator)
	require.Equal(t, challenge.Orchestrator, oInfo.GetTranscoder())
}

func TestScopeUsesPublicURLsInResponseWhenLiveRunnerAddrIsConfigured(t *testing.T) {
	lp := newScopeOffchainHTTP(t)
	liveRunnerAddr, err := url.Parse("http://go-livepeer:8935")
	require.NoError(t, err)
	lp.node.LiveRunnerAddr = liveRunnerAddr
	lp.orchestrator.(*stubOrchestrator).serviceURI = "https://public.example.com"

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/scope", strings.NewReader(`{"model_id":"scope"}`))
	lp.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp worker.LiveVideoToVideoResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.NotNil(t, resp.ControlUrl)
	require.True(t, strings.HasPrefix(*resp.ControlUrl, "https://public.example.com/"))
	require.NotNil(t, resp.EventsUrl)
	require.True(t, strings.HasPrefix(*resp.EventsUrl, "https://public.example.com/"))
	require.NotNil(t, resp.ManifestId)
	closeScopeEvents(t, lp, *resp.ManifestId)
}

func TestScopePaidRetryUsesChallengeManifestID(t *testing.T) {
	lp := newScopeHTTP(t)
	orch := lp.orchestrator.(*stubOrchestrator)
	orch.balances = make(map[ethcommon.Address]map[core.ManifestID]*big.Rat)
	orch.paymentCredit = big.NewRat(100, 1)
	challenge, oInfo := requestScopePaymentChallenge(t, lp)
	headers := liveRunnerReservationPaymentHeaders(t, orch, oInfo.GetAuthToken(), challenge.ManifestID)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/scope", strings.NewReader(`{"model_id":"scope"}`))
	setRequestHeaders(req, headers)
	lp.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp worker.LiveVideoToVideoResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.NotNil(t, resp.ManifestId)
	require.Equal(t, challenge.ManifestID, *resp.ManifestId)
	require.NotNil(t, resp.ControlUrl)
	require.Contains(t, *resp.ControlUrl, *resp.ManifestId+"-control")
	require.NotNil(t, resp.EventsUrl)
	require.Contains(t, *resp.EventsUrl, *resp.ManifestId+"-events")
	closeScopeEvents(t, lp, *resp.ManifestId)

	balance := orch.Balance(orch.Address(), core.ManifestID(challenge.ManifestID))
	require.NotNil(t, balance)
	require.Equal(t, "100", balance.FloatString(0))

	orch.balanceMu.Lock()
	balanceBuckets := make(map[core.ManifestID]*big.Rat, len(orch.balances[orch.Address()]))
	for manifestID, balance := range orch.balances[orch.Address()] {
		balanceBuckets[manifestID] = balance
	}
	orch.balanceMu.Unlock()
	require.Len(t, balanceBuckets, 1)
	require.Contains(t, balanceBuckets, core.ManifestID(challenge.ManifestID))
}

func TestScopePaidRetryRecurringAccountingUsesChallengeManifestID(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		lp := newScopeHTTP(t)
		lp.node.LivePaymentInterval = time.Second
		orch := lp.orchestrator.(*stubOrchestrator)
		orch.balances = make(map[ethcommon.Address]map[core.ManifestID]*big.Rat)
		orch.paymentCredit = big.NewRat(3, 1)

		challenge, oInfo := requestScopePaymentChallenge(t, lp)
		headers := liveRunnerReservationPaymentHeadersWithPrice(t, orch, oInfo.GetAuthToken(), challenge.ManifestID, liveRunnerTestPricePerSecond(1))

		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/scope", strings.NewReader(`{"model_id":"scope"}`))
		setRequestHeaders(req, headers)
		lp.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)
		var resp worker.LiveVideoToVideoResponse
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		require.NotNil(t, resp.ManifestId)
		require.Equal(t, challenge.ManifestID, *resp.ManifestId)

		balance := orch.Balance(orch.Address(), core.ManifestID(challenge.ManifestID))
		require.NotNil(t, balance)
		require.Equal(t, "3", balance.FloatString(0))

		time.Sleep(time.Second)
		eventsCh := trickle.NewLocalPublisher(lp.trickleSrv, challenge.ManifestID+"-events", "application/json")
		require.NoError(t, eventsCh.Write(strings.NewReader(`{"event":"tick"}`)))
		synctest.Wait()

		balance = orch.Balance(orch.Address(), core.ManifestID(challenge.ManifestID))
		require.NotNil(t, balance)
		require.Equal(t, "2", balance.FloatString(0))

		orch.balanceMu.Lock()
		balanceBuckets := make(map[core.ManifestID]*big.Rat, len(orch.balances[orch.Address()]))
		for manifestID, balance := range orch.balances[orch.Address()] {
			balanceBuckets[manifestID] = balance
		}
		orch.balanceMu.Unlock()
		require.Len(t, balanceBuckets, 1)
		require.Contains(t, balanceBuckets, core.ManifestID(challenge.ManifestID))

		require.NoError(t, eventsCh.Close())
		synctest.Wait()
	})
}

func TestScopePaidRetryAllowsSegmentManifestToDifferFromAuthToken(t *testing.T) {
	lp := newScopeHTTP(t)
	orch := lp.orchestrator.(*stubOrchestrator)
	orch.balances = make(map[ethcommon.Address]map[core.ManifestID]*big.Rat)
	orch.paymentCredit = big.NewRat(100, 1)
	_, oInfo := requestScopePaymentChallenge(t, lp)
	legacyManifestID := "different-manifest"
	headers := liveRunnerReservationPaymentHeaders(t, orch, oInfo.GetAuthToken(), legacyManifestID)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/scope", strings.NewReader(`{"model_id":"scope"}`))
	setRequestHeaders(req, headers)
	lp.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp worker.LiveVideoToVideoResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.NotNil(t, resp.ManifestId)
	require.Equal(t, legacyManifestID, *resp.ManifestId)
	require.NotNil(t, resp.ControlUrl)
	require.Contains(t, *resp.ControlUrl, legacyManifestID+"-control")
	require.NotNil(t, resp.EventsUrl)
	require.Contains(t, *resp.EventsUrl, legacyManifestID+"-events")
	closeScopeEvents(t, lp, legacyManifestID)

	balance := orch.Balance(orch.Address(), core.ManifestID(legacyManifestID))
	require.NotNil(t, balance)
	require.Equal(t, "100", balance.FloatString(0))
	require.Nil(t, orch.Balance(orch.Address(), core.ManifestID(oInfo.GetAuthToken().GetSessionId())))
}

func TestScopeServerlessOffchainDoesNotRequirePayment(t *testing.T) {
	lp := newScopeOffchainHTTP(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/scope", strings.NewReader(`{"model_id":"scope"}`))
	lp.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp worker.LiveVideoToVideoResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.NotNil(t, resp.ManifestId)
	require.NotEmpty(t, *resp.ManifestId)
	require.NotNil(t, resp.ControlUrl)
	require.Contains(t, *resp.ControlUrl, *resp.ManifestId+"-control")
	require.NotNil(t, resp.EventsUrl)
	require.Contains(t, *resp.EventsUrl, *resp.ManifestId+"-events")
	closeScopeEvents(t, lp, *resp.ManifestId)
}

func TestScopeRejectsOversizedPayload(t *testing.T) {
	lp := newScopeHTTP(t)

	oversizedBody := `{"model_id":"scope","gateway_request_id":"` + strings.Repeat("a", maxScopeRequestBodySize) + `"}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/scope", strings.NewReader(oversizedBody))
	setRequestHeaders(req, liveRunnerSenderHeaders(lp.orchestrator.(*stubOrchestrator)))
	lp.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), "http: request body too large")
}

func TestLiveRunnerReserveSessionOnchainRejectsInvalidPayer(t *testing.T) {
	lp := newLiveRunnerHTTPOnchain(t)
	registerLiveRunnerForSession(t, lp, nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/apps/runner-1/session", nil)
	req.Header.Set(liveRunnerSenderHeader, "not-an-address")
	lp.ServeHTTP(w, req)

	require.Equal(t, http.StatusPaymentRequired, w.Code)
	require.Contains(t, w.Body.String(), "invalid live runner payment signer address")
}

func TestLiveRunnerReserveSessionNoCapacity(t *testing.T) {
	lp := newLiveRunnerHTTP(t, true)
	registerLiveRunnerForSession(t, lp, nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/apps/runner-1/session", nil)
	lp.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/apps/runner-1/session", nil)
	lp.ServeHTTP(w, req)
	require.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestLiveRunnerStopSessionReleasesCapacity(t *testing.T) {
	lp := newLiveRunnerHTTP(t, true)
	registerLiveRunnerForSession(t, lp, nil)
	sessionID := reserveLiveRunnerSession(t, lp, "runner-1")

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/apps/runner-1/session/"+sessionID+"/stop", nil)
	lp.ServeHTTP(w, req)
	require.Equal(t, http.StatusNoContent, w.Code)

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/apps/runner-1/session", nil)
	lp.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
}

func TestLiveRunnerInternalStopSessionRequiresAuth(t *testing.T) {
	lp := newLiveRunnerHTTP(t, true)
	registerLiveRunnerForSession(t, lp, nil)
	sessionID := reserveLiveRunnerSession(t, lp, "runner-1")

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/runner/runner-1/session/"+sessionID+"/stop", nil)
	lp.ServeHTTP(w, req)
	require.Equal(t, http.StatusUnauthorized, w.Code)

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/runner/runner-1/session/"+sessionID+"/stop", nil)
	req.Header.Set("Livepeer-Session-Token", "wrong-secret")
	lp.ServeHTTP(w, req)
	require.Equal(t, http.StatusUnauthorized, w.Code)

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/apps/runner-1/session", nil)
	lp.ServeHTTP(w, req)
	require.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestLiveRunnerInternalStopSessionReleasesCapacity(t *testing.T) {
	lp := newLiveRunnerHTTP(t, true)
	registerLiveRunnerForSession(t, lp, nil)
	sessionID := reserveLiveRunnerSession(t, lp, "runner-1")

	w := httptest.NewRecorder()
	req := newLiveRunnerChannelRequest(lp, http.MethodPost, "/runner/runner-1/session/"+sessionID+"/stop", "")
	lp.ServeHTTP(w, req)
	require.Equal(t, http.StatusNoContent, w.Code)

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/apps/runner-1/session", nil)
	lp.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
}

func TestLiveRunnerInternalStopSessionRejectsTokenForDifferentSession(t *testing.T) {
	lp := newLiveRunnerHTTP(t, true)
	registerLiveRunnerForSession(t, lp, &liveRunnerRegistrationOptions{Capacity: 2})
	sessionID1 := reserveLiveRunnerSession(t, lp, "runner-1")
	sessionID2 := reserveLiveRunnerSession(t, lp, "runner-1")
	require.NotEqual(t, sessionID1, sessionID2)

	manager, ok := lp.liveRunnerManager()
	require.True(t, ok)
	token1, err := manager.SessionTokenForSession("runner-1", sessionID1)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/runner/runner-1/session/"+sessionID2+"/stop", nil)
	req.Header.Set("Livepeer-Session-Token", token1)
	lp.ServeHTTP(w, req)
	require.Equal(t, http.StatusUnauthorized, w.Code)

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/apps/runner-1/session", nil)
	lp.ServeHTTP(w, req)
	require.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestLiveRunnerProxyForwardsGetPathQueryAndSessionHeaders(t *testing.T) {
	var gotPath, gotQuery, gotRunnerRoute, gotSessionID, gotSessionToken, gotSessionControl string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		gotRunnerRoute = r.Header.Get("Livepeer-Runner-Route")
		gotSessionID = r.Header.Get("Livepeer-Session-Id")
		gotSessionToken = r.Header.Get("Livepeer-Session-Token")
		gotSessionControl = r.Header.Get("Livepeer-Session-Control")
		w.Header().Set("X-Upstream", "ok")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte("proxied"))
	}))
	defer upstream.Close()

	lp := newLiveRunnerHTTP(t, true)
	registerLiveRunnerForSession(t, lp, &liveRunnerRegistrationOptions{RunnerURL: upstream.URL + "/base"})
	sessionID := reserveLiveRunnerSession(t, lp, "runner-1")

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/apps/runner-1/session/"+sessionID+"/app/v1/foo/bar?x=1&y=two", nil)
	lp.ServeHTTP(w, req)

	require.Equal(t, http.StatusAccepted, w.Code)
	require.Equal(t, "proxied", w.Body.String())
	require.Equal(t, "ok", w.Header().Get("X-Upstream"))
	require.Equal(t, "/base/v1/foo/bar", gotPath)
	require.Equal(t, "x=1&y=two", gotQuery)
	require.Equal(t, "runner-1", gotRunnerRoute)
	require.Equal(t, sessionID, gotSessionID)
	require.NotEmpty(t, gotSessionToken)
	require.Equal(t, "http://localhost:1234/runner/runner-1/session/"+sessionID, gotSessionControl)
}

func TestLiveRunnerProxySessionControlBaseURL(t *testing.T) {
	for _, tc := range []struct {
		name           string
		liveRunnerAddr string
		wantBase       string
	}{
		{"falls back to serviceURI", "", "http://localhost:1234"},
		{"uses liveRunnerAddr", "http://go-livepeer:8935", "http://go-livepeer:8935"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var gotSessionControl string
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotSessionControl = r.Header.Get("Livepeer-Session-Control")
				w.WriteHeader(http.StatusAccepted)
			}))
			defer upstream.Close()

			lp := newLiveRunnerHTTP(t, true)
			if tc.liveRunnerAddr != "" {
				addr, err := url.Parse(tc.liveRunnerAddr)
				require.NoError(t, err)
				lp.node.LiveRunnerAddr = addr
			}
			registerLiveRunnerForSession(t, lp, &liveRunnerRegistrationOptions{RunnerURL: upstream.URL})
			sessionID := reserveLiveRunnerSession(t, lp, "runner-1")

			req := httptest.NewRequest(http.MethodGet, "/apps/runner-1/session/"+sessionID+"/app/x", nil)
			lp.ServeHTTP(httptest.NewRecorder(), req)

			require.Equal(t, tc.wantBase+"/runner/runner-1/session/"+sessionID, gotSessionControl)
		})
	}
}

func TestLiveRunnerProxyForwardsPostBody(t *testing.T) {
	var gotMethod, gotBody string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		gotBody = string(body)
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	lp := newLiveRunnerHTTP(t, true)
	registerLiveRunnerForSession(t, lp, &liveRunnerRegistrationOptions{RunnerURL: upstream.URL})
	sessionID := reserveLiveRunnerSession(t, lp, "runner-1")

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/apps/runner-1/session/"+sessionID+"/app/generate", strings.NewReader(`{"prompt":"hi"}`))
	req.Header.Set("Content-Type", "application/json")
	lp.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, http.MethodPost, gotMethod)
	require.Equal(t, `{"prompt":"hi"}`, gotBody)
}

func TestLiveRunnerSingleShotProxyForwardsAndReleasesCapacity(t *testing.T) {
	var gotPath, gotQuery, gotRunnerRoute, gotSessionID, gotSessionToken, gotSessionControl, gotMethod, gotBody string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		gotRunnerRoute = r.Header.Get("Livepeer-Runner-Route")
		gotSessionID = r.Header.Get("Livepeer-Session-Id")
		gotSessionToken = r.Header.Get("Livepeer-Session-Token")
		gotSessionControl = r.Header.Get("Livepeer-Session-Control")
		gotMethod = r.Method
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		gotBody = string(body)
		w.Header().Set("X-Upstream", "ok")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte("single-shot"))
	}))
	defer upstream.Close()

	lp := newLiveRunnerHTTP(t, true)
	registerLiveRunnerForSession(t, lp, &liveRunnerRegistrationOptions{RunnerURL: upstream.URL + "/base", Mode: runner.LiveRunnerModeSingleShot})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/apps/runner-1/app/v1/foo/bar?x=1&y=two", strings.NewReader(`{"prompt":"hi"}`))
	req.Header.Set("Content-Type", "application/json")
	lp.ServeHTTP(w, req)

	require.Equal(t, http.StatusAccepted, w.Code)
	require.Equal(t, "single-shot", w.Body.String())
	require.Equal(t, "ok", w.Header().Get("X-Upstream"))
	require.Equal(t, "/base/v1/foo/bar", gotPath)
	require.Equal(t, "x=1&y=two", gotQuery)
	require.Equal(t, "runner-1", gotRunnerRoute)
	require.NotEmpty(t, gotSessionID)
	require.NotEmpty(t, gotSessionToken)
	require.Equal(t, "http://localhost:1234/runner/runner-1/session/"+gotSessionID, gotSessionControl)
	require.Equal(t, http.MethodPost, gotMethod)
	require.Equal(t, `{"prompt":"hi"}`, gotBody)

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/apps/runner-1/app/next", nil)
	lp.ServeHTTP(w, req)
	require.Equal(t, http.StatusAccepted, w.Code)
}

func TestLiveRunnerSingleShotProxyOnchainReturnsPaymentChallenge(t *testing.T) {
	lp := newLiveRunnerHTTPOnchain(t)
	registerLiveRunnerForSession(t, lp, &liveRunnerRegistrationOptions{Mode: runner.LiveRunnerModeSingleShot})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/apps/runner-1/app/v1/foo", strings.NewReader(`{"prompt":"hi"}`))
	setRequestHeaders(req, liveRunnerSenderHeaders(lp.orchestrator.(*stubOrchestrator)))
	lp.ServeHTTP(w, req)

	require.Equal(t, http.StatusPaymentRequired, w.Code)
	challenge, oInfo := decodeLiveRunnerPaymentChallenge(t, w.Body.Bytes())
	require.NotEmpty(t, challenge.ManifestID)
	require.Equal(t, int64(5000), challenge.PaymentIntervalMs)
	require.Equal(t, int64(10), oInfo.GetPriceInfo().GetPricePerUnit())
	require.Equal(t, int64(1), oInfo.GetPriceInfo().GetPixelsPerUnit())
}

func TestLiveRunnerSingleShotProxyAllowsPaymentWhileRequestOpen(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-release
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("done"))
	}))
	defer upstream.Close()

	oldStorage := drivers.NodeStorage
	drivers.NodeStorage = drivers.NewMemoryDriver(nil)
	t.Cleanup(func() { drivers.NodeStorage = oldStorage })

	lp := newLiveRunnerHTTPOnchain(t)
	registerLiveRunnerForSession(t, lp, &liveRunnerRegistrationOptions{RunnerURL: upstream.URL, Mode: runner.LiveRunnerModeSingleShot})
	orch := lp.orchestrator.(*stubOrchestrator)
	orch.balances = make(map[ethcommon.Address]map[core.ManifestID]*big.Rat)
	orch.paymentCredit = big.NewRat(100, 1)

	challenge, oInfo := requestLiveRunnerPaymentChallenge(t, lp, "runner-1")
	headers := liveRunnerReservationPaymentHeaders(t, orch, oInfo.GetAuthToken(), challenge.ManifestID)

	proxyDone := make(chan int, 1)
	go func() {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/apps/runner-1/app/hold", nil)
		setRequestHeaders(req, headers)
		lp.ServeHTTP(w, req)
		proxyDone <- w.Code
	}()

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for single-shot request to reach runner")
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/apps/runner-1/session/"+challenge.ManifestID+"/payment", nil)
	setRequestHeaders(req, headers)
	lp.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var paymentResult lpnet.PaymentResult
	require.NoError(t, proto.Unmarshal(w.Body.Bytes(), &paymentResult))
	require.NotNil(t, paymentResult.GetInfo())
	require.Equal(t, challenge.ManifestID, paymentResult.GetInfo().GetAuthToken().GetSessionId())

	close(release)
	require.Equal(t, http.StatusOK, <-proxyDone)
}

func TestLiveRunnerSingleShotProxyNoCapacityWhileRequestInFlight(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-release
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	lp := newLiveRunnerHTTP(t, true)
	registerLiveRunnerForSession(t, lp, &liveRunnerRegistrationOptions{RunnerURL: upstream.URL, Mode: runner.LiveRunnerModeSingleShot})

	firstDone := make(chan int, 1)
	go func() {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/apps/runner-1/app/hold", nil)
		lp.ServeHTTP(w, req)
		firstDone <- w.Code
	}()

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for first single-shot request")
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/apps/runner-1/app/blocked", nil)
	lp.ServeHTTP(w, req)
	require.Equal(t, http.StatusServiceUnavailable, w.Code)

	close(release)
	require.Equal(t, http.StatusOK, <-firstDone)
}

func TestLiveRunnerSingleShotProxyRejectsPersistentRunner(t *testing.T) {
	lp := newLiveRunnerHTTP(t, true)
	registerLiveRunnerForSession(t, lp, nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/apps/runner-1/app/v1/foo", nil)
	lp.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), "single-shot")
}

func TestLiveRunnerProxyRejectsInvalidSession(t *testing.T) {
	lp := newLiveRunnerHTTP(t, true)
	registerLiveRunnerForSession(t, lp, nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/apps/runner-1/session/missing/app/v1/foo", nil)
	lp.ServeHTTP(w, req)

	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestLiveRunnerCreateSessionProxyAndProxyDefaultPath(t *testing.T) {
	var gotPath, gotQuery, gotSessionID string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		gotSessionID = r.Header.Get("Livepeer-Session-Id")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte("proxied"))
	}))
	defer upstream.Close()

	lp := newLiveRunnerHTTP(t, true)
	registerLiveRunnerForSession(t, lp, &liveRunnerRegistrationOptions{RunnerURL: upstream.URL + "/base"})
	sessionID := reserveLiveRunnerSession(t, lp, "runner-1")

	w := httptest.NewRecorder()
	req := newLiveRunnerChannelRequest(lp, http.MethodPost, "/runner/runner-1/session/"+sessionID+"/proxy", `{"target_url":`+strconv.Quote(upstream.URL+"/target")+`}`)
	lp.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	var proxy runner.LiveRunnerProxy
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &proxy))
	require.NotEmpty(t, proxy.ProxyID)
	require.Equal(t, "http://localhost:1234/run/"+proxy.ProxyID, proxy.URL)

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, proxy.URL+"/foo/bar?x=1", nil)
	lp.ServeHTTP(w, req)
	require.Equal(t, http.StatusAccepted, w.Code)
	require.Equal(t, "proxied", w.Body.String())
	require.Equal(t, "/target/foo/bar", gotPath)
	require.Equal(t, "x=1", gotQuery)
	require.Equal(t, sessionID, gotSessionID)
}

func TestLiveRunnerDynamicSingleShotProxyForwardsAndReleasesSession(t *testing.T) {
	var gotPath, gotRunnerRoute, gotSessionID string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotRunnerRoute = r.Header.Get("Livepeer-Runner-Route")
		gotSessionID = r.Header.Get("Livepeer-Session-Id")
		w.WriteHeader(http.StatusAccepted)
	}))
	defer upstream.Close()

	lp := newLiveRunnerHTTP(t, true)
	registerLiveRunnerForSession(t, lp, &liveRunnerRegistrationOptions{
		RunnerURL: upstream.URL + "/base",
		Mode:      runner.LiveRunnerModeSingleShot,
		Proxy:     true,
	})
	manager := lp.node.LiveRunnerManager.(*runner.LiveRunnerRegistry)
	require.Equal(t, "http://localhost:1234/run/runner-1", manager.Runners()[0].URL)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://localhost:1234/run/runner-1/foo", nil)
	lp.ServeHTTP(w, req)
	require.Equal(t, http.StatusAccepted, w.Code)
	require.Equal(t, "/base/foo", gotPath)
	require.Equal(t, "runner-1", gotRunnerRoute)
	require.NotEmpty(t, gotSessionID)
	require.Equal(t, 0, manager.Runners()[0].CapacityUsed)
}

func TestLiveRunnerStaticProxyForwardsAndReleasesSingleShotSession(t *testing.T) {
	var gotPath, gotQuery, gotRunnerRoute, gotSessionID, gotSessionToken string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		gotRunnerRoute = r.Header.Get("Livepeer-Runner-Route")
		gotSessionID = r.Header.Get("Livepeer-Session-Id")
		gotSessionToken = r.Header.Get("Livepeer-Session-Token")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte("proxied"))
	}))
	defer upstream.Close()
	healthSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer healthSrv.Close()

	lp := newLiveRunnerHTTP(t, true)
	manager := lp.node.LiveRunnerManager.(*runner.LiveRunnerRegistry)
	_, err := manager.RegisterStaticRunners(runner.StaticLiveRunnerConfig{Runners: []runner.StaticLiveRunnerConfigEntry{{
		Label:     "static-proxy",
		Routing:   runner.LiveRunnerRoutingLabel,
		Proxy:     true,
		RunnerURL: upstream.URL + "/base",
		App:       "live-video-to-video/scope",
		Mode:      runner.LiveRunnerModeSingleShot,
		Capacity:  1,
		HealthURL: healthSrv.URL,
	}}})
	require.NoError(t, err)
	require.Equal(t, "http://localhost:1234/run/static-proxy", manager.Runners()[0].URL)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://localhost:1234/run/static-proxy/foo/bar?x=1", nil)
	lp.ServeHTTP(w, req)

	require.Equal(t, http.StatusAccepted, w.Code)
	require.Equal(t, "proxied", w.Body.String())
	require.Equal(t, "/base/foo/bar", gotPath)
	require.Equal(t, "x=1", gotQuery)
	require.Equal(t, "static-proxy", gotRunnerRoute)
	require.NotEmpty(t, gotSessionID)
	require.NotEmpty(t, gotSessionToken)
	runners := manager.Runners()
	require.Len(t, runners, 1)
	require.Equal(t, 0, runners[0].CapacityUsed)
	require.Equal(t, 1, runners[0].CapacityAvailable)
}

func TestLiveRunnerCreateSessionProxyDefaultsToAppPath(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "empty body"},
		{name: "empty object", body: `{}`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var gotPath, gotQuery, gotSessionID string
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.Path
				gotQuery = r.URL.RawQuery
				gotSessionID = r.Header.Get("Livepeer-Session-Id")
				w.WriteHeader(http.StatusAccepted)
				_, _ = w.Write([]byte("proxied"))
			}))
			defer upstream.Close()

			lp := newLiveRunnerHTTP(t, true)
			registerLiveRunnerForSession(t, lp, &liveRunnerRegistrationOptions{RunnerURL: upstream.URL + "/base"})
			sessionID := reserveLiveRunnerSession(t, lp, "runner-1")

			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/apps/runner-1/session/"+sessionID+"/app/foo/bar?x=1", strings.NewReader("app-body"))
			lp.ServeHTTP(w, req)
			require.Equal(t, http.StatusAccepted, w.Code)
			require.Equal(t, "proxied", w.Body.String())
			require.Equal(t, "/base/foo/bar", gotPath)
			require.Equal(t, "x=1", gotQuery)
			require.Equal(t, sessionID, gotSessionID)
			appPath, appQuery, appSessionID := gotPath, gotQuery, gotSessionID

			w = httptest.NewRecorder()
			req = newLiveRunnerChannelRequest(lp, http.MethodPost, "/runner/runner-1/session/"+sessionID+"/proxy", test.body)
			lp.ServeHTTP(w, req)
			require.Equal(t, http.StatusOK, w.Code)
			var proxy runner.LiveRunnerProxy
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &proxy))
			require.NotEmpty(t, proxy.ProxyID)
			require.Equal(t, "http://localhost:1234/run/"+proxy.ProxyID, proxy.URL)

			w = httptest.NewRecorder()
			req = httptest.NewRequest(http.MethodPost, proxy.URL+"/foo/bar?x=1", strings.NewReader("proxied-body"))
			lp.ServeHTTP(w, req)
			require.Equal(t, http.StatusAccepted, w.Code)
			require.Equal(t, "proxied", w.Body.String())
			require.Equal(t, appPath, gotPath)
			require.Equal(t, appQuery, gotQuery)
			require.Equal(t, appSessionID, gotSessionID)
		})
	}
}

func TestLiveRunnerSessionProxyHostTemplate(t *testing.T) {
	var gotPath string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusAccepted)
	}))
	defer upstream.Close()

	node, err := core.NewLivepeerNode(nil, t.TempDir(), nil)
	require.NoError(t, err)
	orch := newStubOrchestrator()
	orch.secret = "live-runner-secret"
	lp := &lphttp{
		orchestrator: orch,
		node:         node,
		transRPC:     http.NewServeMux(),
	}
	manager := runner.NewLiveRunnerRegistry(runner.LiveRunnerRegistryConfig{
		Host:             testLiveRunnerHost{orchestrator: lp.orchestrator},
		ProxyURLTemplate: "https://{proxy}.daydream.example.com",
	})
	t.Cleanup(manager.Stop)
	node.LiveRunnerManager = manager
	require.NoError(t, startAIServer(lp))

	registerLiveRunnerForSession(t, lp, &liveRunnerRegistrationOptions{RunnerURL: upstream.URL + "/base"})
	sessionID := reserveLiveRunnerSession(t, lp, "runner-1")
	w := httptest.NewRecorder()
	req := newLiveRunnerChannelRequest(lp, http.MethodPost, "/runner/runner-1/session/"+sessionID+"/proxy", `{"target_url":`+strconv.Quote(upstream.URL+"/target")+`}`)
	lp.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	var proxy runner.LiveRunnerProxy
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &proxy))
	require.Equal(t, "https://"+proxy.ProxyID+".daydream.example.com", proxy.URL)

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "https://"+proxy.ProxyID+".daydream.example.com/foo", nil)
	lp.ServeHTTP(w, req)
	require.Equal(t, http.StatusAccepted, w.Code)
	require.Equal(t, "/target/foo", gotPath)
}

func TestLiveRunnerCreateSessionProxyRejectsUnauthorizedAndBadTarget(t *testing.T) {
	lp := newLiveRunnerHTTP(t, true)
	registerLiveRunnerForSession(t, lp, nil)
	sessionID := reserveLiveRunnerSession(t, lp, "runner-1")

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/runner/runner-1/session/"+sessionID+"/proxy", strings.NewReader(`{"target_url":"https://runner.example.com/app"}`))
	lp.ServeHTTP(w, req)
	require.Equal(t, http.StatusUnauthorized, w.Code)

	w = httptest.NewRecorder()
	req = newLiveRunnerChannelRequest(lp, http.MethodPost, "/runner/runner-1/session/"+sessionID+"/proxy", `{"target_url":"https://other.example.com/app"}`)
	lp.ServeHTTP(w, req)
	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), "hostname")
}

func TestLiveRunnerCreateTrickleChannel(t *testing.T) {
	lp := newLiveRunnerHTTP(t, true)
	registerLiveRunnerForSession(t, lp, nil)
	sessionID := reserveLiveRunnerSession(t, lp, "runner-1")

	w := httptest.NewRecorder()
	req := newLiveRunnerChannelRequest(lp, http.MethodPost, "/runner/runner-1/session/"+sessionID+"/channels", `{"channels":[{"name":"foo/bar","mime_type":"video/MP2T"},{"name":"events","mime_type":"application/json"}]}`)
	lp.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp liveRunnerTrickleChannelsResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Len(t, resp.Channels, 2)
	require.Equal(t, "foo-bar", resp.Channels[0].Name)
	require.Equal(t, sessionID+"-foo-bar", resp.Channels[0].ChannelName)
	require.Equal(t, "video/MP2T", resp.Channels[0].MimeType)
	require.Equal(t, "http://localhost:1234/ai/trickle/"+sessionID+"-foo-bar", resp.Channels[0].URL)
	require.Equal(t, "http://localhost:1234/ai/trickle/"+sessionID+"-foo-bar", resp.Channels[0].InternalURL)
	require.Equal(t, sessionID+"-events", resp.Channels[1].ChannelName)

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/ai/trickle/"+sessionID+"-foo-bar/next", nil)
	lp.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
}

func TestLiveRunnerCreateTrickleChannelUsesLiveRunnerAddr(t *testing.T) {
	node, err := core.NewLivepeerNode(nil, t.TempDir(), nil)
	require.NoError(t, err)
	node.LiveRunnerAddr, err = url.Parse("http://go-livepeer:8935")
	require.NoError(t, err)
	orch := newStubOrchestrator()
	orch.secret = "live-runner-secret"
	lp := &lphttp{
		orchestrator: orch,
		node:         node,
		transRPC:     http.NewServeMux(),
	}
	manager := runner.NewLiveRunnerRegistry(runner.LiveRunnerRegistryConfig{Host: testLiveRunnerHost{orchestrator: lp.orchestrator}})
	t.Cleanup(manager.Stop)
	node.LiveRunnerManager = manager
	require.NoError(t, startAIServer(lp))

	registerLiveRunnerForSession(t, lp, nil)
	sessionID := reserveLiveRunnerSession(t, lp, "runner-1")

	w := httptest.NewRecorder()
	req := newLiveRunnerChannelRequest(lp, http.MethodPost, "/runner/runner-1/session/"+sessionID+"/channels", `{"channels":[{"name":"override"}]}`)
	lp.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp liveRunnerTrickleChannelsResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Len(t, resp.Channels, 1)
	require.Equal(t, "http://localhost:1234/ai/trickle/"+sessionID+"-override", resp.Channels[0].URL)
	require.Equal(t, "http://go-livepeer:8935/ai/trickle/"+sessionID+"-override", resp.Channels[0].InternalURL)
}

func TestLiveRunnerCreateTrickleChannelReturnsExisting(t *testing.T) {
	lp := newLiveRunnerHTTP(t, true)
	registerLiveRunnerForSession(t, lp, nil)
	sessionID := reserveLiveRunnerSession(t, lp, "runner-1")

	w := httptest.NewRecorder()
	req := newLiveRunnerChannelRequest(lp, http.MethodPost, "/runner/runner-1/session/"+sessionID+"/channels", `{"channels":[{"name":"existing","mime_type":"video/MP2T"}]}`)
	lp.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	var first liveRunnerTrickleChannelsResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &first))

	w = httptest.NewRecorder()
	req = newLiveRunnerChannelRequest(lp, http.MethodPost, "/runner/runner-1/session/"+sessionID+"/channels", `{"channels":[{"name":"existing","mime_type":"application/json"}]}`)
	lp.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	var second liveRunnerTrickleChannelsResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &second))
	require.Equal(t, first, second)
	require.NotEmpty(t, second.Channels[0].URL)
	require.NotEmpty(t, second.Channels[0].InternalURL)
	require.Equal(t, "video/MP2T", second.Channels[0].MimeType)
}

func TestLiveRunnerCreateTrickleChannelRejectsInvalidSessionAndName(t *testing.T) {
	lp := newLiveRunnerHTTP(t, true)
	registerLiveRunnerForSession(t, lp, nil)

	w := httptest.NewRecorder()
	req := newLiveRunnerChannelRequest(lp, http.MethodPost, "/runner/runner-1/session/missing/channels", `{"channels":[{"name":"valid"}]}`)
	lp.ServeHTTP(w, req)
	require.Equal(t, http.StatusNotFound, w.Code)

	sessionID := reserveLiveRunnerSession(t, lp, "runner-1")
	w = httptest.NewRecorder()
	req = newLiveRunnerChannelRequest(lp, http.MethodPost, "/runner/runner-1/session/"+sessionID+"/channels", `{"channels":[{"name":"bad.name"}]}`)
	lp.ServeHTTP(w, req)
	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestLiveRunnerTrickleChannelRequiresAuth(t *testing.T) {
	lp := newLiveRunnerHTTP(t, true)
	registerLiveRunnerForSession(t, lp, nil)
	sessionID := reserveLiveRunnerSession(t, lp, "runner-1")

	// Check missing session token.
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/runner/runner-1/session/"+sessionID+"/channels", strings.NewReader(`{"channels":[{"name":"unauthorized"}]}`))
	req.Header.Set("Authorization", "wrong-secret")
	lp.ServeHTTP(w, req)
	require.Equal(t, http.StatusUnauthorized, w.Code)

	// Check delete requires session token too.
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodDelete, "/runner/runner-1/session/"+sessionID+"/channels", strings.NewReader(`{"channels":["unauthorized"]}`))
	req.Header.Set("Authorization", "wrong-secret")
	lp.ServeHTTP(w, req)
	require.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestLiveRunnerTrickleChannelRejectsTokenForDifferentSession(t *testing.T) {
	lp := newLiveRunnerHTTP(t, true)
	registerLiveRunnerForSession(t, lp, &liveRunnerRegistrationOptions{Capacity: 2})
	sessionID1 := reserveLiveRunnerSession(t, lp, "runner-1")
	sessionID2 := reserveLiveRunnerSession(t, lp, "runner-1")
	require.NotEqual(t, sessionID1, sessionID2)

	manager, ok := lp.liveRunnerManager()
	require.True(t, ok)
	token1, err := manager.SessionTokenForSession("runner-1", sessionID1)
	require.NoError(t, err)

	// Check token is scoped to its session.
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/runner/runner-1/session/"+sessionID2+"/channels", strings.NewReader(`{"channels":[{"name":"wrong-session"}]}`))
	req.Header.Set("Livepeer-Session-Token", token1)
	lp.ServeHTTP(w, req)
	require.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestLiveRunnerTrickleChannelBatchLimit(t *testing.T) {
	lp := newLiveRunnerHTTP(t, true)
	registerLiveRunnerForSession(t, lp, nil)
	sessionID := reserveLiveRunnerSession(t, lp, "runner-1")

	createReq := liveRunnerTrickleChannelsRequest{Channels: make([]liveRunnerTrickleChannelRequest, maxLiveRunnerTrickleChannelsPerRequest+1)}
	deleteReq := deleteLiveRunnerTrickleChannelsRequest{Channels: make([]string, maxLiveRunnerTrickleChannelsPerRequest+1)}
	for i := range createReq.Channels {
		name := fmt.Sprintf("channel-%d", i)
		createReq.Channels[i] = liveRunnerTrickleChannelRequest{Name: name}
		deleteReq.Channels[i] = name
	}

	body, err := json.Marshal(createReq)
	require.NoError(t, err)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/runner/runner-1/session/"+sessionID+"/channels", bytes.NewReader(body))
	setLiveRunnerSessionToken(t, lp, req, "runner-1", sessionID)
	lp.ServeHTTP(w, req)
	require.Equal(t, http.StatusBadRequest, w.Code)

	body, err = json.Marshal(deleteReq)
	require.NoError(t, err)
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodDelete, "/runner/runner-1/session/"+sessionID+"/channels", bytes.NewReader(body))
	setLiveRunnerSessionToken(t, lp, req, "runner-1", sessionID)
	lp.ServeHTTP(w, req)
	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestLiveRunnerDeleteTrickleChannel(t *testing.T) {
	lp := newLiveRunnerHTTP(t, true)
	registerLiveRunnerForSession(t, lp, nil)
	sessionID := reserveLiveRunnerSession(t, lp, "runner-1")

	w := httptest.NewRecorder()
	req := newLiveRunnerChannelRequest(lp, http.MethodPost, "/runner/runner-1/session/"+sessionID+"/channels", `{"channels":[{"name":"delete-me"},{"name":"delete-me-too"}]}`)
	lp.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	w = httptest.NewRecorder()
	req = newLiveRunnerChannelRequest(lp, http.MethodDelete, "/runner/runner-1/session/"+sessionID+"/channels", `{"channels":["delete-me","delete-me-too"]}`)
	lp.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	var resp deleteLiveRunnerTrickleChannelsResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Equal(t, []string{"delete-me", "delete-me-too"}, resp.Deleted)

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/ai/trickle/"+sessionID+"-delete-me/next", nil)
	lp.ServeHTTP(w, req)
	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestLiveRunnerStopSessionClosesTrickleChannels(t *testing.T) {
	lp := newLiveRunnerHTTP(t, true)
	registerLiveRunnerForSession(t, lp, nil)
	sessionID := reserveLiveRunnerSession(t, lp, "runner-1")

	w := httptest.NewRecorder()
	req := newLiveRunnerChannelRequest(lp, http.MethodPost, "/runner/runner-1/session/"+sessionID+"/channels", `{"channels":[{"name":"stop-cleanup"}]}`)
	lp.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/apps/runner-1/session/"+sessionID+"/stop", nil)
	lp.ServeHTTP(w, req)
	require.Equal(t, http.StatusNoContent, w.Code)

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/ai/trickle/"+sessionID+"-stop-cleanup/next", nil)
	lp.ServeHTTP(w, req)
	require.Equal(t, http.StatusNotFound, w.Code)
}

func newLiveRunnerHTTP(t *testing.T, withManager bool) *lphttp {
	t.Helper()
	node, err := core.NewLivepeerNode(nil, t.TempDir(), nil)
	require.NoError(t, err)
	orch := newStubOrchestrator()
	orch.secret = "live-runner-secret"
	lp := &lphttp{
		orchestrator: orch,
		node:         node,
		transRPC:     http.NewServeMux(),
	}
	if withManager {
		manager := runner.NewLiveRunnerRegistry(runner.LiveRunnerRegistryConfig{Host: testLiveRunnerHost{orchestrator: lp.orchestrator}})
		t.Cleanup(manager.Stop)
		node.LiveRunnerManager = manager
	}
	require.NoError(t, startAIServer(lp))
	return lp
}

func newLiveRunnerHTTPOnchain(t *testing.T) *lphttp {
	t.Helper()
	prevWatcher := core.PriceFeedWatcher
	core.PriceFeedWatcher = stubPriceFeedWatcher{price: eth.PriceData{Price: big.NewRat(2000, 1)}}
	t.Cleanup(func() { core.PriceFeedWatcher = prevWatcher })

	node, err := core.NewLivepeerNode(nil, t.TempDir(), nil)
	require.NoError(t, err)
	node.LivePaymentInterval = 5 * time.Second
	orch := newStubOrchestrator()
	orch.secret = "live-runner-secret"
	orch.ticketParams = defaultTicketParams()
	lp := &lphttp{
		orchestrator: orch,
		node:         node,
		transRPC:     http.NewServeMux(),
	}
	manager := runner.NewLiveRunnerRegistry(runner.LiveRunnerRegistryConfig{Host: testLiveRunnerHost{orchestrator: lp.orchestrator}, Onchain: true})
	t.Cleanup(manager.Stop)
	node.LiveRunnerManager = manager
	require.NoError(t, startAIServer(lp))
	return lp
}

func newServerlessLiveRunnerHTTP(t *testing.T, withManager bool, capacity int) *lphttp {
	t.Helper()
	node, err := core.NewLivepeerNode(nil, t.TempDir(), nil)
	require.NoError(t, err)
	orch := newStubOrchestrator()
	orch.secret = "live-runner-secret"
	lp := &lphttp{
		orchestrator: orch,
		node:         node,
		transRPC:     http.NewServeMux(),
	}
	if withManager {
		manager := runner.NewLiveRunnerRegistry(runner.LiveRunnerRegistryConfig{Host: testLiveRunnerHost{orchestrator: lp.orchestrator}})
		t.Cleanup(manager.Stop)
		node.LiveRunnerManager = manager
	}
	serverlessWorker, err := worker.NewServerlessWorker("ws://serverless.example.com/ws", capacity)
	require.NoError(t, err)
	node.AIWorker = serverlessWorker
	node.Capabilities = createStubAIWorkerCapabilitiesForPipelineModelId("live-video-to-video", "scope")
	require.NoError(t, startAIServer(lp))
	return lp
}

func newScopeHTTP(t *testing.T) *lphttp {
	t.Helper()
	oldStorage := drivers.NodeStorage
	drivers.NodeStorage = drivers.NewMemoryDriver(nil)
	t.Cleanup(func() { drivers.NodeStorage = oldStorage })
	node, err := core.NewLivepeerNode(nil, t.TempDir(), nil)
	require.NoError(t, err)
	node.Eth = &eth.StubClient{}
	node.Capabilities = createStubAIWorkerCapabilitiesForPipelineModelId("live-video-to-video", "scope")
	orch := newStubOrchestrator()
	orch.ticketParams = defaultTicketParams()
	lp := &lphttp{
		orchestrator: orch,
		node:         node,
		transRPC:     http.NewServeMux(),
	}
	require.NoError(t, startAIServer(lp))
	return lp
}

func newScopeOffchainHTTP(t *testing.T) *lphttp {
	t.Helper()
	lp := newScopeHTTP(t)
	lp.node.Eth = nil
	return lp
}

func newLiveRunnerHTTPWithNode(t *testing.T, node *core.LivepeerNode) *lphttp {
	t.Helper()
	orch := newStubOrchestrator()
	orch.secret = "live-runner-secret"
	lp := &lphttp{
		orchestrator: orch,
		node:         node,
		transRPC:     http.NewServeMux(),
	}
	require.NoError(t, startAIServer(lp))
	return lp
}

func requestLiveRunnerPaymentChallenge(t *testing.T, lp *lphttp, runnerID string) (liveRunnerPaymentChallengeResponse, *lpnet.OrchestratorInfo) {
	t.Helper()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/apps/"+runnerID+"/session", nil)
	setRequestHeaders(req, liveRunnerSenderHeaders(lp.orchestrator.(*stubOrchestrator)))
	lp.ServeHTTP(w, req)
	require.Equal(t, http.StatusPaymentRequired, w.Code)
	return decodeLiveRunnerPaymentChallenge(t, w.Body.Bytes())
}

func requestScopePaymentChallenge(t *testing.T, lp *lphttp) (liveRunnerPaymentChallengeResponse, *lpnet.OrchestratorInfo) {
	t.Helper()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/scope", strings.NewReader(`{"model_id":"scope"}`))
	setRequestHeaders(req, liveRunnerSenderHeaders(lp.orchestrator.(*stubOrchestrator)))
	lp.ServeHTTP(w, req)
	require.Equal(t, http.StatusPaymentRequired, w.Code)
	return decodeLiveRunnerPaymentChallenge(t, w.Body.Bytes())
}

func closeScopeEvents(t *testing.T, lp *lphttp, manifestID string) {
	t.Helper()
	eventsCh := trickle.NewLocalPublisher(lp.trickleSrv, manifestID+"-events", "application/json")
	require.NoError(t, eventsCh.Close())
}

func decodeLiveRunnerPaymentChallenge(t *testing.T, body []byte) (liveRunnerPaymentChallengeResponse, *lpnet.OrchestratorInfo) {
	t.Helper()
	var challenge liveRunnerPaymentChallengeResponse
	require.NoError(t, json.Unmarshal(body, &challenge))
	require.NotEmpty(t, challenge.PaymentParams)
	raw, err := base64.StdEncoding.DecodeString(challenge.PaymentParams)
	require.NoError(t, err)
	var oInfo lpnet.OrchestratorInfo
	require.NoError(t, proto.Unmarshal(raw, &oInfo))
	return challenge, &oInfo
}

func liveRunnerSenderHeaders(orch *stubOrchestrator) http.Header {
	headers := http.Header{}
	headers.Set(liveRunnerSenderHeader, orch.Address().Hex())
	return headers
}

func setRequestHeaders(req *http.Request, headers http.Header) {
	for k, values := range headers {
		for _, v := range values {
			req.Header.Add(k, v)
		}
	}
}

func liveRunnerReservationPaymentHeaders(t *testing.T, orch *stubOrchestrator, authToken *lpnet.AuthToken, manifestID string) http.Header {
	t.Helper()
	return liveRunnerReservationPaymentHeadersWithPrice(t, orch, authToken, manifestID, &lpnet.PriceInfo{PricePerUnit: 10, PixelsPerUnit: 1})
}

func liveRunnerReservationPaymentHeadersWithPrice(t *testing.T, orch *stubOrchestrator, authToken *lpnet.AuthToken, manifestID string, priceInfo *lpnet.PriceInfo) http.Header {
	t.Helper()
	md := &core.SegTranscodingMetadata{
		ManifestID: core.ManifestID(manifestID),
		Caps:       core.NewCapabilities(nil, nil),
		AuthToken:  authToken,
	}
	sig, err := orch.Sign(md.Flatten())
	require.NoError(t, err)
	segData, err := core.NetSegData(md)
	require.NoError(t, err)
	segData.Sig = sig
	segBytes, err := proto.Marshal(segData)
	require.NoError(t, err)

	payment := &lpnet.Payment{
		Sender:        orch.Address().Bytes(),
		ExpectedPrice: priceInfo,
	}
	paymentBytes, err := proto.Marshal(payment)
	require.NoError(t, err)

	headers := http.Header{}
	headers.Set(segmentHeader, base64.StdEncoding.EncodeToString(segBytes))
	headers.Set(paymentHeader, base64.StdEncoding.EncodeToString(paymentBytes))
	return headers
}

func liveRunnerTestPricePerSecond(pricePerSecond int64) *lpnet.PriceInfo {
	return &lpnet.PriceInfo{
		PricePerUnit:  pricePerSecond,
		PixelsPerUnit: int64(defaultSegInfo.Height) * int64(defaultSegInfo.Width) * int64(defaultSegInfo.FPS),
	}
}

func liveTestPricePerSecond(pricePerSecond int64) *lpnet.PriceInfo {
	return &lpnet.PriceInfo{
		PricePerUnit:  pricePerSecond,
		PixelsPerUnit: 1,
	}
}

type liveRunnerRegistrationOptions struct {
	RunnerID  string
	RunnerURL string
	Capacity  int
	Mode      string
	Proxy     bool
	PriceUnit string
}

func registerLiveRunnerForSession(t *testing.T, lp *lphttp, opts *liveRunnerRegistrationOptions) {
	t.Helper()
	if lp == nil {
		require.FailNow(t, "live runner lphttp is required")
	}
	if opts == nil {
		opts = &liveRunnerRegistrationOptions{}
	}
	runnerID := opts.RunnerID
	if runnerID == "" {
		runnerID = "runner-1"
	}
	if runnerID != strings.TrimSpace(runnerID) || strings.Contains(runnerID, "/") {
		require.FailNow(t, "invalid live runner ID", "runnerID=%q", runnerID)
	}
	runnerURL := opts.RunnerURL
	if runnerURL == "" {
		runnerURL = "https://runner.example.com"
	}
	if strings.TrimSpace(runnerURL) == "" {
		require.FailNow(t, "live runner URL is required")
	}
	capacity := opts.Capacity
	if capacity == 0 {
		capacity = 1
	}
	if capacity < 0 {
		require.FailNow(t, "live runner capacity must be non-negative", "capacity=%d", capacity)
	}
	if opts.Mode != "" && opts.Mode != runner.LiveRunnerModePersistent && opts.Mode != runner.LiveRunnerModeSingleShot {
		require.FailNow(t, "invalid live runner mode", "mode=%q", opts.Mode)
	}
	priceUnit := opts.PriceUnit
	if priceUnit == "" {
		priceUnit = "720p"
	}
	manager, ok := lp.liveRunnerManager()
	require.True(t, ok)
	_, err := manager.Heartbeat(runner.LiveRunnerHeartbeatRequest{
		RunnerID:  runnerID,
		Proxy:     opts.Proxy,
		RunnerURL: runnerURL,
		Status:    "ready",
		Mode:      opts.Mode,
		App:       "live-video-to-video/scope",
		Capacity:  capacity,
		PriceInfo: runner.LiveRunnerPriceInfo{
			Price: json.Number("0.001990656"),
			Unit:  priceUnit,
		},
	}, lp.orchestrator.RegistrationSecret())
	require.NoError(t, err)
}

func reserveLiveRunnerSession(t *testing.T, lp *lphttp, runnerID string) string {
	t.Helper()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/apps/"+runnerID+"/session", nil)
	lp.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	var resp liveRunnerSessionResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.NotEmpty(t, resp.SessionID)
	return resp.SessionID
}

func reservePaidLiveRunnerSession(t *testing.T, lp *lphttp, runnerID string, priceInfo *lpnet.PriceInfo) string {
	t.Helper()
	challenge, oInfo := requestLiveRunnerPaymentChallenge(t, lp, runnerID)
	headers := liveRunnerReservationPaymentHeadersWithPrice(t, lp.orchestrator.(*stubOrchestrator), oInfo.GetAuthToken(), challenge.ManifestID, priceInfo)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/apps/"+runnerID+"/session", nil)
	setRequestHeaders(req, headers)
	lp.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	var resp liveRunnerSessionResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Equal(t, challenge.ManifestID, resp.SessionID)
	return resp.SessionID
}

func newLiveRunnerChannelRequest(lp *lphttp, method, target, body string) *http.Request {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	path := strings.Split(strings.TrimPrefix(target, "/"), "?")[0]
	parts := strings.Split(path, "/")
	if len(parts) >= 5 && parts[0] == "runner" && parts[2] == "session" {
		setLiveRunnerSessionToken(nil, lp, req, parts[1], parts[3])
	}
	return req
}

func setLiveRunnerSessionToken(t require.TestingT, lp *lphttp, req *http.Request, runnerID, sessionID string) {
	manager, ok := lp.liveRunnerManager()
	if !ok {
		if t != nil {
			require.FailNow(t, "live runner manager unavailable")
		}
		return
	}
	token, err := manager.SessionTokenForSession(runnerID, sessionID)
	if err != nil {
		return
	}
	req.Header.Set("Livepeer-Session-Token", token)
}
