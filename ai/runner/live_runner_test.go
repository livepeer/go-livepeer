package runner

import (
	"context"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"testing"
	"time"

	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/eth"
	"github.com/livepeer/go-livepeer/trickle"
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

func liveRunnerTestHeartbeat(runnerID string) LiveRunnerHeartbeatRequest {
	return LiveRunnerHeartbeatRequest{
		RunnerID:  runnerID,
		Label:     "test-runner",
		RunnerURL: "https://runner.example.com",
		Version:   "1.2.3",
		Status:    "ready",
		GPU: &LiveRunnerGPU{
			ID:     "gpu-0",
			Name:   "NVIDIA L40S",
			VRAMMB: 46068,
		},
		App:      "new-ai-pipeline/model-a",
		Capacity: 1,
		PriceInfo: LiveRunnerPriceInfo{
			PricePerUnit:  1250,
			PixelsPerUnit: 1,
			Unit:          "USD",
		},
	}
}

const liveRunnerTestBootstrapSecret = "bootstrap-secret"

var (
	liveRunnerTestGeneratedRunnerIDPattern  = regexp.MustCompile(`^runner_[a-z2-7]{8}$`)
	liveRunnerTestGeneratedSessionIDPattern = regexp.MustCompile(`^session_[a-z2-7]{8}$`)
	liveRunnerTestSecretPattern             = regexp.MustCompile(`^[a-z2-7]+$`)
)

type liveRunnerTestHost struct{}

func (liveRunnerTestHost) ServiceURI() *url.URL {
	u, _ := url.Parse("http://localhost:1234")
	return u
}

func (liveRunnerTestHost) RegistrationSecret() string {
	return liveRunnerTestBootstrapSecret
}

func liveRunnerTestRegister(t *testing.T, registry *LiveRunnerRegistry, req LiveRunnerHeartbeatRequest) *LiveRunnerHeartbeatResponse {
	t.Helper()
	resp, err := registry.Heartbeat(req, liveRunnerTestBootstrapSecret)
	if err != nil {
		t.Fatal(err)
	}
	if resp.HeartbeatSecret == "" {
		t.Fatal("expected heartbeat secret")
	}
	if !liveRunnerTestSecretPattern.MatchString(resp.HeartbeatSecret) {
		t.Fatalf("expected base32 heartbeat secret, got %q", resp.HeartbeatSecret)
	}
	if resp.Orchestrator != "http://localhost:1234" {
		t.Fatalf("unexpected orchestrator URL: %s", resp.Orchestrator)
	}
	return resp
}

func TestLiveRunnerRegistry_HeartbeatUpsertCapacity(t *testing.T) {
	registry := NewLiveRunnerRegistry(LiveRunnerRegistryConfig{Host: liveRunnerTestHost{}})

	resp := liveRunnerTestRegister(t, registry, liveRunnerTestHeartbeat(""))
	if resp.RunnerID == "" {
		t.Fatal("expected runner id")
	}
	if !liveRunnerTestGeneratedRunnerIDPattern.MatchString(resp.RunnerID) {
		t.Fatalf("expected generated base32 runner id, got %q", resp.RunnerID)
	}
	runner := registry.runners[resp.RunnerID]
	if runner == nil || runner.Capacity != 1 {
		t.Fatalf("unexpected initial runner state: %+v", runner)
	}

	_, err := registry.Heartbeat(LiveRunnerHeartbeatRequest{
		RunnerID:  resp.RunnerID,
		Label:     "test-runner",
		RunnerURL: "https://runner.example.com",
		Status:    "ready",
		App:       "new-ai-pipeline/model-a",
		Capacity:  2,
		PriceInfo: LiveRunnerPriceInfo{
			PricePerUnit:  1250,
			PixelsPerUnit: 1,
			Unit:          "USD",
		},
	}, resp.HeartbeatSecret)
	if err != nil {
		t.Fatal(err)
	}
	runner = registry.runners[resp.RunnerID]
	if runner == nil || runner.Capacity != 2 {
		t.Fatalf("unexpected heartbeat runner state: %+v", runner)
	}
}

func TestLiveRunnerRegistry_HeartbeatUnknownIDCreatesRunner(t *testing.T) {
	registry := NewLiveRunnerRegistry(LiveRunnerRegistryConfig{Host: liveRunnerTestHost{}})
	req := liveRunnerTestHeartbeat("runner_custom_1")

	resp := liveRunnerTestRegister(t, registry, req)
	if resp.RunnerID != "runner_custom_1" {
		t.Fatalf("expected supplied runner id to be used, got %s", resp.RunnerID)
	}
	runner := registry.runners[resp.RunnerID]
	if runner == nil || runner.Capacity != 1 {
		t.Fatalf("unexpected state after create-by-unknown-id heartbeat: %+v", runner)
	}
}

func TestLiveRunnerRegistry_DefaultCapacityWhenUnset(t *testing.T) {
	registry := NewLiveRunnerRegistry(LiveRunnerRegistryConfig{Host: liveRunnerTestHost{}})
	req := liveRunnerTestHeartbeat("runner_default_capacity")
	req.Capacity = 0

	resp := liveRunnerTestRegister(t, registry, req)
	registry.mu.Lock()
	runner := registry.runners[resp.RunnerID]
	registry.mu.Unlock()
	if runner == nil || runner.Capacity != 1 {
		t.Fatalf("expected default capacity=1, got %+v", runner)
	}
}

func TestLiveRunnerRegistry_RunnersDiscoveryShape(t *testing.T) {
	registry := NewLiveRunnerRegistry(LiveRunnerRegistryConfig{Host: liveRunnerTestHost{}})

	liveRunnerTestRegister(t, registry, liveRunnerTestHeartbeat("runner_discovery_1"))

	runners := registry.Runners()
	if len(runners) != 1 {
		t.Fatalf("expected one runner, got %d", len(runners))
	}
	runner := runners[0]
	if runner.Endpoint != "https://runner.example.com" {
		t.Fatalf("unexpected endpoint: %s", runner.Endpoint)
	}
	if runner.GPU == nil || runner.GPU.Name != "NVIDIA L40S" {
		t.Fatalf("unexpected gpu: %+v", runner.GPU)
	}
	if runner.App != "new-ai-pipeline/model-a" {
		t.Fatalf("unexpected app: %s", runner.App)
	}
	if runner.PriceInfo.Unit != "USD" {
		t.Fatalf("unexpected runner price info: %+v", runner.PriceInfo)
	}
	if runner.Version != "1.2.3" {
		t.Fatalf("unexpected runner version: %s", runner.Version)
	}
}

func TestLiveRunnerRegistry_ConvertsUSDToWEI(t *testing.T) {
	prevWatcher := core.PriceFeedWatcher
	core.PriceFeedWatcher = stubPriceFeedWatcher{price: eth.PriceData{Price: big.NewRat(2000, 1)}}
	defer func() { core.PriceFeedWatcher = prevWatcher }()

	registry := NewLiveRunnerRegistry(LiveRunnerRegistryConfig{Host: liveRunnerTestHost{}})
	req := liveRunnerTestHeartbeat("runner-1")
	req.PriceInfo = LiveRunnerPriceInfo{PricePerUnit: 10, PixelsPerUnit: 1, Unit: "USD"}
	liveRunnerTestRegister(t, registry, req)

	runners := registry.Runners()
	if len(runners) != 1 {
		t.Fatalf("expected one runner, got %d", len(runners))
	}
	got := big.NewRat(runners[0].PriceInfo.PricePerUnit, runners[0].PriceInfo.PixelsPerUnit)
	expectedFixed, err := common.PriceToFixed(big.NewRat(5_000_000_000_000_000, 1280*720*30*3600))
	if err != nil {
		t.Fatal(err)
	}
	expected := big.NewRat(expectedFixed, 1)
	if got.Cmp(expected) != 0 {
		t.Fatalf("unexpected converted price: got=%s want=%s", got, expected)
	}
	if runners[0].PriceInfo.Unit != "WEI" {
		t.Fatalf("unexpected converted unit: %s", runners[0].PriceInfo.Unit)
	}
}

func TestLiveRunnerRegistry_SharedEndpointAppKeepsPerRunnerPrices(t *testing.T) {
	registry := NewLiveRunnerRegistry(LiveRunnerRegistryConfig{Host: liveRunnerTestHost{}})

	req1 := liveRunnerTestHeartbeat("runner-1")
	req1.PriceInfo = LiveRunnerPriceInfo{PricePerUnit: 10, PixelsPerUnit: 1, Unit: "WEI"}
	req2 := liveRunnerTestHeartbeat("runner-2")
	req2.PriceInfo = LiveRunnerPriceInfo{PricePerUnit: 20, PixelsPerUnit: 1, Unit: "WEI"}
	resp1 := liveRunnerTestRegister(t, registry, req1)
	liveRunnerTestRegister(t, registry, req2)

	if err := registry.Unregister("runner-1", resp1.HeartbeatSecret); err != nil {
		t.Fatal(err)
	}
	runners := registry.Runners()
	if len(runners) != 1 {
		t.Fatalf("expected one remaining runner, got %d", len(runners))
	}
	if got := runners[0].PriceInfo.PricePerUnit; got != 20 {
		t.Fatalf("expected remaining runner price to stay isolated, got %d", got)
	}
}

func TestLiveRunnerRegistry_ReserveSessionCapacity(t *testing.T) {
	registry := NewLiveRunnerRegistry(LiveRunnerRegistryConfig{Host: liveRunnerTestHost{}})
	req := liveRunnerTestHeartbeat("runner-1")
	req.Capacity = 2
	liveRunnerTestRegister(t, registry, req)

	sessionID1, endpoint, err := registry.ReserveSession("runner-1")
	if err != nil {
		t.Fatal(err)
	}
	if endpoint != req.RunnerURL {
		t.Fatalf("unexpected endpoint: %s", endpoint)
	}
	if !liveRunnerTestGeneratedSessionIDPattern.MatchString(sessionID1) {
		t.Fatalf("expected generated base32 session id, got %q", sessionID1)
	}
	sessionID2, _, err := registry.ReserveSession("runner-1")
	if err != nil {
		t.Fatal(err)
	}
	if sessionID1 == sessionID2 {
		t.Fatalf("expected unique session IDs, got %s", sessionID1)
	}

	registry.mu.Lock()
	active := len(registry.runners["runner-1"].sessions)
	registry.mu.Unlock()
	if active != 2 {
		t.Fatalf("expected 2 active sessions, got %d", active)
	}

	if _, _, err := registry.ReserveSession("runner-1"); !isRunnerErrorStatus(err, http.StatusServiceUnavailable) {
		t.Fatalf("expected no capacity runner error, got %v", err)
	}
}

func TestLiveRunnerRegistry_ReleaseSessionFreesCapacity(t *testing.T) {
	registry := NewLiveRunnerRegistry(LiveRunnerRegistryConfig{Host: liveRunnerTestHost{}})
	liveRunnerTestRegister(t, registry, liveRunnerTestHeartbeat("runner-1"))
	sessionID, _, err := registry.ReserveSession("runner-1")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := registry.ReserveSession("runner-1"); !isRunnerErrorStatus(err, http.StatusServiceUnavailable) {
		t.Fatalf("expected no capacity runner error, got %v", err)
	}
	if err := registry.ReleaseSession("runner-1", sessionID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := registry.ReserveSession("runner-1"); err != nil {
		t.Fatalf("expected capacity after release, got %v", err)
	}
}

func TestLiveRunnerRegistry_SessionToken(t *testing.T) {
	registry := NewLiveRunnerRegistry(LiveRunnerRegistryConfig{Host: liveRunnerTestHost{}})
	req := liveRunnerTestHeartbeat("runner-1")
	req.Capacity = 2
	liveRunnerTestRegister(t, registry, req)
	sessionID, _, err := registry.ReserveSession("runner-1")
	if err != nil {
		t.Fatal(err)
	}

	// Check session token encoding and validation.
	token, err := registry.SessionTokenForSession("runner-1", sessionID)
	if err != nil {
		t.Fatal(err)
	}
	if !liveRunnerTestSecretPattern.MatchString(token) {
		t.Fatalf("expected base32 session token, got %q", token)
	}
	if err := registry.ValidSessionToken("runner-1", sessionID, token); err != nil {
		t.Fatal(err)
	}

	otherSessionID, _, err := registry.ReserveSession("runner-1")
	if err != nil {
		t.Fatal(err)
	}
	// Check token is scoped to its session.
	if err := registry.ValidSessionToken("runner-1", otherSessionID, token); !isRunnerErrorStatus(err, http.StatusUnauthorized) {
		t.Fatalf("expected token to be scoped to one session, got %v", err)
	}
}

func TestLiveRunnerRegistry_CreateTrickleChannelForSession(t *testing.T) {
	trickleSrv, channelBaseURL, _ := newLiveRunnerTestTrickleServer(t)
	registry := NewLiveRunnerRegistry(LiveRunnerRegistryConfig{Host: liveRunnerTestHost{}})
	registry.SetTrickleServer(trickleSrv, channelBaseURL)
	liveRunnerTestRegister(t, registry, liveRunnerTestHeartbeat("runner-1"))
	sessionID, _, err := registry.ReserveSession("runner-1")
	if err != nil {
		t.Fatal(err)
	}

	channel, err := registry.CreateTrickleChannel("runner-1", sessionID, "foo/bar", "video/MP2T")
	if err != nil {
		t.Fatal(err)
	}
	expectedName := sessionID + "-foo-bar"
	if channel.ChannelName != expectedName {
		t.Fatalf("unexpected channel name channel=%+v want=%s", channel, expectedName)
	}
	if channel.Name != "foo-bar" {
		t.Fatalf("unexpected sanitized name: %s", channel.Name)
	}
	if channel.URL != channelBaseURL+expectedName {
		t.Fatalf("unexpected channel URL: %s", channel.URL)
	}
}

func TestLiveRunnerRegistry_CreateTrickleChannelReturnsExisting(t *testing.T) {
	trickleSrv, channelBaseURL, _ := newLiveRunnerTestTrickleServer(t)
	registry := NewLiveRunnerRegistry(LiveRunnerRegistryConfig{Host: liveRunnerTestHost{}})
	registry.SetTrickleServer(trickleSrv, channelBaseURL)
	liveRunnerTestRegister(t, registry, liveRunnerTestHeartbeat("runner-1"))
	sessionID, _, err := registry.ReserveSession("runner-1")
	if err != nil {
		t.Fatal(err)
	}

	first, err := registry.CreateTrickleChannel("runner-1", sessionID, "existing", "video/MP2T")
	if err != nil {
		t.Fatal(err)
	}
	second, err := registry.CreateTrickleChannel("runner-1", sessionID, "existing", "application/json")
	if err != nil {
		t.Fatal(err)
	}
	if first.URL != second.URL || second.MimeType != "video/MP2T" {
		t.Fatalf("expected existing channel metadata, first=%+v second=%+v", first, second)
	}
}

func TestLiveRunnerRegistry_CreateTrickleChannelRejectsInvalidName(t *testing.T) {
	trickleSrv, channelBaseURL, _ := newLiveRunnerTestTrickleServer(t)
	registry := NewLiveRunnerRegistry(LiveRunnerRegistryConfig{Host: liveRunnerTestHost{}})
	registry.SetTrickleServer(trickleSrv, channelBaseURL)
	liveRunnerTestRegister(t, registry, liveRunnerTestHeartbeat("runner-1"))
	sessionID, _, err := registry.ReserveSession("runner-1")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := registry.CreateTrickleChannel("runner-1", sessionID, "bad.name", ""); !isRunnerErrorStatus(err, http.StatusBadRequest) {
		t.Fatalf("expected bad request for invalid channel name, got %v", err)
	}
	if _, err := registry.CreateTrickleChannel("runner-1", sessionID, "", ""); !isRunnerErrorStatus(err, http.StatusBadRequest) {
		t.Fatalf("expected bad request for empty channel name, got %v", err)
	}
}

func TestLiveRunnerRegistry_TrickleChannelCleanup(t *testing.T) {
	trickleSrv, channelBaseURL, channelStatus := newLiveRunnerTestTrickleServer(t)
	registry := NewLiveRunnerRegistry(LiveRunnerRegistryConfig{Host: liveRunnerTestHost{}})
	registry.SetTrickleServer(trickleSrv, channelBaseURL)
	resp1 := liveRunnerTestRegister(t, registry, liveRunnerTestHeartbeat("runner-1"))
	sessionID, _, err := registry.ReserveSession("runner-1")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := registry.CreateTrickleChannel("runner-1", sessionID, "cleanup", ""); err != nil {
		t.Fatal(err)
	}
	if status := channelStatus(sessionID + "-cleanup"); status != http.StatusOK {
		t.Fatalf("expected channel to exist, got status=%d", status)
	}
	if err := registry.ReleaseSession("runner-1", sessionID); err != nil {
		t.Fatal(err)
	}
	if status := channelStatus(sessionID + "-cleanup"); status != http.StatusNotFound {
		t.Fatalf("expected release to close channel, got status=%d", status)
	}

	sessionID, _, err = registry.ReserveSession("runner-1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := registry.CreateTrickleChannel("runner-1", sessionID, "cleanup", ""); err != nil {
		t.Fatal(err)
	}
	if err := registry.Unregister("runner-1", resp1.HeartbeatSecret); err != nil {
		t.Fatal(err)
	}
	if status := channelStatus(sessionID + "-cleanup"); status != http.StatusNotFound {
		t.Fatalf("expected unregister to close channel, got status=%d", status)
	}

	registry.heartbeatTTL = defaultLiveRunnerHeartbeatTTL
	liveRunnerTestRegister(t, registry, liveRunnerTestHeartbeat("runner-2"))
	sessionID, _, err = registry.ReserveSession("runner-2")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := registry.CreateTrickleChannel("runner-2", sessionID, "cleanup", ""); err != nil {
		t.Fatal(err)
	}
	registry.heartbeatTTL = time.Nanosecond
	time.Sleep(time.Millisecond)
	if _, err := registry.RunnerEndpointForSession("runner-2", sessionID); !isRunnerErrorStatus(err, http.StatusNotFound) {
		t.Fatalf("expected expired runner to be removed, got %v", err)
	}
	if status := channelStatus(sessionID + "-cleanup"); status != http.StatusNotFound {
		t.Fatalf("expected expiry to close channel, got status=%d", status)
	}
}

func TestLiveRunnerRegistry_ExpireAndUnregisterClearSessions(t *testing.T) {
	registry := NewLiveRunnerRegistry(LiveRunnerRegistryConfig{Host: liveRunnerTestHost{}})
	liveRunnerTestRegister(t, registry, liveRunnerTestHeartbeat("runner-1"))
	sessionID, _, err := registry.ReserveSession("runner-1")
	if err != nil {
		t.Fatal(err)
	}
	registry.heartbeatTTL = time.Nanosecond
	time.Sleep(time.Millisecond)
	if _, err := registry.RunnerEndpointForSession("runner-1", sessionID); !isRunnerErrorStatus(err, http.StatusNotFound) {
		t.Fatalf("expected expired runner to be removed, got %v", err)
	}

	registry.heartbeatTTL = defaultLiveRunnerHeartbeatTTL
	resp2 := liveRunnerTestRegister(t, registry, liveRunnerTestHeartbeat("runner-2"))
	if _, _, err := registry.ReserveSession("runner-2"); err != nil {
		t.Fatal(err)
	}
	if err := registry.Unregister("runner-2", resp2.HeartbeatSecret); err != nil {
		t.Fatal(err)
	}
	if _, err := registry.RunnerEndpointForSession("runner-2", sessionID); !isRunnerErrorStatus(err, http.StatusNotFound) {
		t.Fatalf("expected unregistered runner to be removed, got %v", err)
	}
}

func TestLiveRunnerRegistry_NotReadyCannotReserveSession(t *testing.T) {
	registry := NewLiveRunnerRegistry(LiveRunnerRegistryConfig{Host: liveRunnerTestHost{}})
	req := liveRunnerTestHeartbeat("runner-1")
	req.Status = "busy"
	liveRunnerTestRegister(t, registry, req)
	if _, _, err := registry.ReserveSession("runner-1"); !isRunnerErrorStatus(err, http.StatusNotFound) {
		t.Fatalf("expected not-ready runner to be unavailable, got %v", err)
	}
}

func isRunnerErrorStatus(err error, statusCode int) bool {
	var runnerErr *RunnerError
	return errors.As(err, &runnerErr) && runnerErr.StatusCode == statusCode
}

func newLiveRunnerTestTrickleServer(t *testing.T) (*trickle.Server, string, func(string) int) {
	t.Helper()
	mux := http.NewServeMux()
	trickleSrv := trickle.ConfigureServer(trickle.TrickleServerConfig{
		Mux:      mux,
		BasePath: "/ai/trickle/",
	})
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	channelBaseURL := ts.URL + "/ai/trickle/"
	channelStatus := func(channelName string) int {
		t.Helper()
		resp, err := http.Get(channelBaseURL + channelName + "/next")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		return resp.StatusCode
	}
	return trickleSrv, channelBaseURL, channelStatus
}
