package server

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
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
		PriceInfo: runner.LiveRunnerPriceInfo{
			PricePerUnit:  10,
			PixelsPerUnit: 1,
			Unit:          "USD",
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
	req.Header.Set("Authorization", lp.orchestrator.RegistrationSecret())
	lp.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp runner.LiveRunnerHeartbeatResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Equal(t, "runner-1", resp.RunnerID)
	require.Equal(t, lp.orchestrator.ServiceURI().String(), resp.Orchestrator)
	require.NotEmpty(t, resp.HeartbeatSecret)

	// Check missing auth after bootstrap.
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/runners/heartbeat", bytes.NewReader(body))
	lp.ServeHTTP(w, req)
	require.Equal(t, http.StatusUnauthorized, w.Code)

	// Check follow-up heartbeat auth.
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/runners/heartbeat", bytes.NewReader(body))
	req.Header.Set("Authorization", resp.HeartbeatSecret)
	lp.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	var nextResp runner.LiveRunnerHeartbeatResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &nextResp))
	require.Equal(t, "runner-1", nextResp.RunnerID)
	require.Empty(t, nextResp.HeartbeatSecret)
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

func TestLiveRunnerReserveSession(t *testing.T) {
	lp := newLiveRunnerHTTP(t, true)
	registerLiveRunnerForSession(t, lp, "runner-1", "https://runner.example.com", 1)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/apps/runner-1/session", nil)
	lp.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp liveRunnerSessionResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.NotEmpty(t, resp.SessionID)
	require.Equal(t, "http://localhost:1234/apps/runner-1/session/"+resp.SessionID+"/app", resp.AppURL)
}

func TestLiveRunnerReserveSessionNoCapacity(t *testing.T) {
	lp := newLiveRunnerHTTP(t, true)
	registerLiveRunnerForSession(t, lp, "runner-1", "https://runner.example.com", 1)

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
	registerLiveRunnerForSession(t, lp, "runner-1", "https://runner.example.com", 1)
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

func TestLiveRunnerProxyForwardsGetPathQueryAndSessionHeaders(t *testing.T) {
	var gotPath, gotQuery, gotSessionID, gotSessionToken string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		gotSessionID = r.Header.Get("Livepeer-Session-Id")
		gotSessionToken = r.Header.Get("Livepeer-Session-Token")
		w.Header().Set("X-Upstream", "ok")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte("proxied"))
	}))
	defer upstream.Close()

	lp := newLiveRunnerHTTP(t, true)
	registerLiveRunnerForSession(t, lp, "runner-1", upstream.URL+"/base", 1)
	sessionID := reserveLiveRunnerSession(t, lp, "runner-1")

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/apps/runner-1/session/"+sessionID+"/app/v1/foo/bar?x=1&y=two", nil)
	lp.ServeHTTP(w, req)

	require.Equal(t, http.StatusAccepted, w.Code)
	require.Equal(t, "proxied", w.Body.String())
	require.Equal(t, "ok", w.Header().Get("X-Upstream"))
	require.Equal(t, "/base/v1/foo/bar", gotPath)
	require.Equal(t, "x=1&y=two", gotQuery)
	require.Equal(t, sessionID, gotSessionID)
	require.NotEmpty(t, gotSessionToken)
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
	registerLiveRunnerForSession(t, lp, "runner-1", upstream.URL, 1)
	sessionID := reserveLiveRunnerSession(t, lp, "runner-1")

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/apps/runner-1/session/"+sessionID+"/app/generate", strings.NewReader(`{"prompt":"hi"}`))
	req.Header.Set("Content-Type", "application/json")
	lp.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, http.MethodPost, gotMethod)
	require.Equal(t, `{"prompt":"hi"}`, gotBody)
}

func TestLiveRunnerProxyRejectsInvalidSession(t *testing.T) {
	lp := newLiveRunnerHTTP(t, true)
	registerLiveRunnerForSession(t, lp, "runner-1", "https://runner.example.com", 1)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/apps/runner-1/session/missing/app/v1/foo", nil)
	lp.ServeHTTP(w, req)

	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestLiveRunnerCreateTrickleChannel(t *testing.T) {
	lp := newLiveRunnerHTTP(t, true)
	registerLiveRunnerForSession(t, lp, "runner-1", "https://runner.example.com", 1)
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
	require.Equal(t, sessionID+"-events", resp.Channels[1].ChannelName)

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/ai/trickle/"+sessionID+"-foo-bar/next", nil)
	lp.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
}

func TestLiveRunnerCreateTrickleChannelReturnsExisting(t *testing.T) {
	lp := newLiveRunnerHTTP(t, true)
	registerLiveRunnerForSession(t, lp, "runner-1", "https://runner.example.com", 1)
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
	require.Equal(t, "video/MP2T", second.Channels[0].MimeType)
}

func TestLiveRunnerCreateTrickleChannelRejectsInvalidSessionAndName(t *testing.T) {
	lp := newLiveRunnerHTTP(t, true)
	registerLiveRunnerForSession(t, lp, "runner-1", "https://runner.example.com", 1)

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
	registerLiveRunnerForSession(t, lp, "runner-1", "https://runner.example.com", 1)
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
	registerLiveRunnerForSession(t, lp, "runner-1", "https://runner.example.com", 2)
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
	registerLiveRunnerForSession(t, lp, "runner-1", "https://runner.example.com", 1)
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
	registerLiveRunnerForSession(t, lp, "runner-1", "https://runner.example.com", 1)
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
	registerLiveRunnerForSession(t, lp, "runner-1", "https://runner.example.com", 1)
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
	lp := &lphttp{
		orchestrator: newStubOrchestrator(),
		node:         node,
		transRPC:     http.NewServeMux(),
	}
	if withManager {
		node.LiveRunnerManager = runner.NewLiveRunnerRegistry(runner.LiveRunnerRegistryConfig{Host: lp.orchestrator})
	}
	require.NoError(t, startAIServer(lp))
	return lp
}

func newServerlessLiveRunnerHTTP(t *testing.T, withManager bool, capacity int) *lphttp {
	t.Helper()
	node, err := core.NewLivepeerNode(nil, t.TempDir(), nil)
	require.NoError(t, err)
	lp := &lphttp{
		orchestrator: newStubOrchestrator(),
		node:         node,
		transRPC:     http.NewServeMux(),
	}
	if withManager {
		node.LiveRunnerManager = runner.NewLiveRunnerRegistry(runner.LiveRunnerRegistryConfig{Host: lp.orchestrator})
	}
	serverlessWorker, err := worker.NewServerlessWorker("ws://serverless.example.com/ws", capacity)
	require.NoError(t, err)
	node.AIWorker = serverlessWorker
	node.Capabilities = createStubAIWorkerCapabilitiesForPipelineModelId("live-video-to-video", "scope")
	require.NoError(t, startAIServer(lp))
	return lp
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

func registerLiveRunnerForSession(t *testing.T, lp *lphttp, runnerID, runnerURL string, capacity int) {
	t.Helper()
	manager, ok := lp.liveRunnerManager()
	require.True(t, ok)
	_, err := manager.Heartbeat(runner.LiveRunnerHeartbeatRequest{
		RunnerID:  runnerID,
		RunnerURL: runnerURL,
		Status:    "ready",
		App:       "live-video-to-video/scope",
		Capacity:  capacity,
		PriceInfo: runner.LiveRunnerPriceInfo{
			PricePerUnit:  10,
			PixelsPerUnit: 1,
			Unit:          "WEI",
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
