package runner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

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

func (h liveRunnerTestHost) LiveRunnerURI() *url.URL {
	return h.ServiceURI()
}

func (liveRunnerTestHost) RegistrationSecret() string {
	return liveRunnerTestBootstrapSecret
}

type liveRunnerSplitURIHost struct{}

func (liveRunnerSplitURIHost) ServiceURI() *url.URL {
	u, _ := url.Parse("https://public.example.com")
	return u
}

func (liveRunnerSplitURIHost) LiveRunnerURI() *url.URL {
	u, _ := url.Parse("http://go-livepeer:8935")
	return u
}

func (liveRunnerSplitURIHost) RegistrationSecret() string {
	return liveRunnerTestBootstrapSecret
}

type liveRunnerTestHostWithoutSecret struct{}

func (liveRunnerTestHostWithoutSecret) ServiceURI() *url.URL {
	u, _ := url.Parse("http://localhost:1234")
	return u
}

func (h liveRunnerTestHostWithoutSecret) LiveRunnerURI() *url.URL {
	return h.ServiceURI()
}

func (liveRunnerTestHostWithoutSecret) RegistrationSecret() string {
	return ""
}

func newLiveRunnerTestRegistry() *LiveRunnerRegistry {
	return NewLiveRunnerRegistry(LiveRunnerRegistryConfig{Host: liveRunnerTestHost{}})
}

func newOnchainLiveRunnerTestRegistry() *LiveRunnerRegistry {
	return NewLiveRunnerRegistry(LiveRunnerRegistryConfig{Host: liveRunnerTestHost{}, Onchain: true})
}

func liveRunnerTestRegister(t *testing.T, registry *LiveRunnerRegistry, req LiveRunnerHeartbeatRequest) *LiveRunnerHeartbeatResponse {
	t.Helper()
	ensureLiveRunnerTestTrickleServer(t, registry)
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

func ensureLiveRunnerTestTrickleServer(t *testing.T, registry *LiveRunnerRegistry) (string, func(string) int) {
	t.Helper()
	registry.mu.Lock()
	hasTrickleServer := registry.trickleSrv != nil
	channelBaseURL := registry.internalTrickleBaseURL
	registry.mu.Unlock()
	if hasTrickleServer {
		return channelBaseURL, nil
	}
	trickleSrv, channelBaseURL, channelStatus := newLiveRunnerTestTrickleServer(t)
	registry.SetTrickleServer(trickleSrv, channelBaseURL, channelBaseURL)
	return channelBaseURL, channelStatus
}

func liveRunnerTestBootstrapChannels(resp *LiveRunnerHeartbeatResponse) []LiveRunnerTrickleChannel {
	channels := []LiveRunnerTrickleChannel{}
	if resp.O2R != nil {
		channels = append(channels, *resp.O2R)
	}
	return channels
}

func TestLiveRunnerRegistry_HeartbeatUpsertCapacity(t *testing.T) {
	registry := newLiveRunnerTestRegistry()

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
	}, resp.HeartbeatSecret)
	if err != nil {
		t.Fatal(err)
	}
	runner = registry.runners[resp.RunnerID]
	if runner == nil || runner.Capacity != 2 {
		t.Fatalf("unexpected heartbeat runner state: %+v", runner)
	}
}

func TestLiveRunnerRegistry_SplitsPublicAndRunnerURIs(t *testing.T) {
	registry := NewLiveRunnerRegistry(LiveRunnerRegistryConfig{Host: liveRunnerSplitURIHost{}})
	t.Cleanup(registry.Stop)
	trickleSrv, _, _ := newLiveRunnerTestTrickleServer(t)
	registry.SetTrickleServer(trickleSrv, "https://public.example.com/ai/trickle/", "http://go-livepeer:8935/ai/trickle/")

	resp, err := registry.Heartbeat(liveRunnerTestHeartbeat("runner-split-uri"), liveRunnerTestBootstrapSecret)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Orchestrator != "http://go-livepeer:8935" {
		t.Fatalf("expected heartbeat orchestrator to use runner URI, got %q", resp.Orchestrator)
	}
	if resp.O2R == nil || !strings.HasPrefix(resp.O2R.URL, "http://go-livepeer:8935/ai/trickle/") {
		t.Fatalf("expected O2R channel to use runner trickle URL, got %+v", resp.O2R)
	}
	if resp.O2R.InternalURL != "" {
		t.Fatalf("expected O2R channel to omit internal URL, got %+v", resp.O2R)
	}

	runners := registry.Runners()
	if len(runners) != 1 {
		t.Fatalf("expected one discovery runner, got %d", len(runners))
	}
	if runners[0].URL != "https://public.example.com/apps/runner-split-uri/session" {
		t.Fatalf("expected discovery URL to use public service URI, got %q", runners[0].URL)
	}

	sessionID, _, err := registry.ReserveSession("runner-split-uri")
	if err != nil {
		t.Fatal(err)
	}
	channel, err := registry.CreateTrickleChannel("runner-split-uri", sessionID, "events", "application/json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(channel.URL, "https://public.example.com/ai/trickle/") {
		t.Fatalf("expected runner-created trickle channel to use public trickle URL, got %q", channel.URL)
	}
	if !strings.HasPrefix(channel.InternalURL, "http://go-livepeer:8935/ai/trickle/") {
		t.Fatalf("expected runner-created trickle channel to include internal trickle URL, got %q", channel.InternalURL)
	}
	token, err := registry.SessionTokenForSession("runner-split-uri", sessionID)
	if err != nil {
		t.Fatal(err)
	}
	if err := registry.ValidSessionToken("runner-split-uri", sessionID, token); err != nil {
		t.Fatal(err)
	}
}

func TestLiveRunnerRegistry_HeartbeatUnknownIDCreatesRunner(t *testing.T) {
	registry := newLiveRunnerTestRegistry()
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

func TestLiveRunnerRegistry_HeartbeatCreatesBootstrapTrickleChannels(t *testing.T) {
	trickleSrv, channelBaseURL, channelStatus := newLiveRunnerTestTrickleServer(t)
	registry := newLiveRunnerTestRegistry()
	registry.SetTrickleServer(trickleSrv, channelBaseURL, channelBaseURL)
	req := liveRunnerTestHeartbeat("runner-bootstrap")

	resp := liveRunnerTestRegister(t, registry, req)
	channels := liveRunnerTestBootstrapChannels(resp)
	if len(channels) != 1 {
		t.Fatalf("expected bootstrap trickle channels, got %+v", resp)
	}
	expectedNames := []string{
		LiveRunnerTrickleOrchestratorToRunner,
	}
	for i, expectedName := range expectedNames {
		channel := channels[i]
		if channel.Name != expectedName {
			t.Fatalf("unexpected bootstrap channel name at %d: %+v", i, channel)
		}
		if !strings.HasPrefix(channel.ChannelName, resp.RunnerID+"-") || !strings.HasSuffix(channel.ChannelName, "-"+expectedName) {
			t.Fatalf("expected randomized backing channel for %q, got %q", expectedName, channel.ChannelName)
		}
		if channel.ChannelName == resp.RunnerID+"-"+expectedName {
			t.Fatalf("expected non-guessable backing channel name, got %q", channel.ChannelName)
		}
		if channel.URL != channelBaseURL+channel.ChannelName {
			t.Fatalf("unexpected bootstrap channel URL: %s", channel.URL)
		}
		if channel.MimeType != "application/json" {
			t.Fatalf("unexpected bootstrap channel MIME type: %s", channel.MimeType)
		}
		if status := channelStatus(channel.ChannelName); status != http.StatusOK {
			t.Fatalf("expected bootstrap channel to exist, got status=%d", status)
		}
	}
}

func TestLiveRunnerRegistry_O2RKeepaliveWriteLoop(t *testing.T) {
	restore := liveRunnerTestSetO2RKeepaliveInterval(t, 10*time.Millisecond)
	defer restore()

	trickleSrv, channelBaseURL, _ := newLiveRunnerTestTrickleServer(t)
	registry := newLiveRunnerTestRegistry()
	t.Cleanup(registry.Stop)
	registry.SetTrickleServer(trickleSrv, channelBaseURL, channelBaseURL)
	resp := liveRunnerTestRegister(t, registry, liveRunnerTestHeartbeat("runner-o2r-keepalive"))

	msg := liveRunnerTestReadTrickleMessage(t, trickleSrv, resp.O2R.ChannelName, func(msg string) bool {
		return msg == liveRunnerO2RKeepaliveMessage
	})
	if msg != liveRunnerO2RKeepaliveMessage {
		t.Fatalf("unexpected keepalive message: %q", msg)
	}
}

func TestLiveRunnerRegistry_O2RSessionLifecycleMessages(t *testing.T) {
	restore := liveRunnerTestSetO2RKeepaliveInterval(t, time.Hour)
	defer restore()

	trickleSrv, channelBaseURL, _ := newLiveRunnerTestTrickleServer(t)
	registry := newLiveRunnerTestRegistry()
	t.Cleanup(registry.Stop)
	registry.SetTrickleServer(trickleSrv, channelBaseURL, channelBaseURL)
	req := liveRunnerTestHeartbeat("runner-o2r-session")
	req.Capacity = 2
	resp := liveRunnerTestRegister(t, registry, req)

	sessionID, _, err := registry.ReserveSession("runner-o2r-session", "manifest-id")
	if err != nil {
		t.Fatal(err)
	}
	liveRunnerTestReadSessionEvent(t, trickleSrv, resp.O2R.ChannelName, "reserved", sessionID)

	if err := registry.ReleaseSession("runner-o2r-session", sessionID); err != nil {
		t.Fatal(err)
	}
	liveRunnerTestReadSessionEvent(t, trickleSrv, resp.O2R.ChannelName, "released", sessionID)
}

func TestLiveRunnerRegistry_O2RWriteLoopClosesOnUnregister(t *testing.T) {
	restore := liveRunnerTestSetO2RKeepaliveInterval(t, time.Hour)
	defer restore()

	registry := newLiveRunnerTestRegistry()
	t.Cleanup(registry.Stop)
	resp := liveRunnerTestRegister(t, registry, liveRunnerTestHeartbeat("runner-o2r-close"))

	registry.mu.Lock()
	runner := registry.runners["runner-o2r-close"]
	registry.mu.Unlock()
	if runner == nil || runner.o2r == nil {
		t.Fatal("expected registered runner with o2r channel")
	}
	o2r := runner.o2r

	if err := registry.Unregister("runner-o2r-close", resp.HeartbeatSecret); err != nil {
		t.Fatal(err)
	}
	select {
	case <-o2r.done:
	case <-time.After(time.Second):
		t.Fatal("expected o2r write loop to close on unregister")
	}
}

func TestLiveRunnerRegistry_HeartbeatDoesNotReturnBootstrapChannelsAgain(t *testing.T) {
	registry := newLiveRunnerTestRegistry()
	resp := liveRunnerTestRegister(t, registry, liveRunnerTestHeartbeat("runner-bootstrap-once"))

	nextResp, err := registry.Heartbeat(liveRunnerTestHeartbeat(resp.RunnerID), resp.HeartbeatSecret)
	if err != nil {
		t.Fatal(err)
	}
	if nextResp.O2R != nil {
		t.Fatalf("expected no bootstrap channels on follow-up heartbeat, got %+v", nextResp)
	}
	registry.mu.Lock()
	runner := registry.runners[resp.RunnerID]
	registry.mu.Unlock()
	runner.mu.Lock()
	defer runner.mu.Unlock()
	if runner.o2r == nil {
		t.Fatalf("expected existing bootstrap channel to be preserved, got o2r=%+v", runner.o2r)
	}
}

func TestLiveRunnerRegistry_HeartbeatSessionIDs(t *testing.T) {
	registry := newLiveRunnerTestRegistry()
	req := liveRunnerTestHeartbeat("runner-session-ids")
	req.Capacity = 3
	req.SessionIDs = []string{"runner-reported-session"}
	resp := liveRunnerTestRegister(t, registry, req)
	if len(resp.SessionIDs) != 0 {
		t.Fatalf("expected no active session ids on initial heartbeat, got %+v", resp.SessionIDs)
	}

	registry.mu.Lock()
	runner := registry.runners[resp.RunnerID]
	registry.mu.Unlock()
	runner.mu.Lock()
	if !slices.Equal(runner.LiveRunnerHeartbeatRequest.SessionIDs, []string{"runner-reported-session"}) {
		t.Fatalf("expected request session_ids to be retained, got %+v", runner.LiveRunnerHeartbeatRequest.SessionIDs)
	}
	runner.mu.Unlock()

	if _, _, err := registry.ReserveSession(resp.RunnerID, "session-z-oldest"); err != nil {
		t.Fatal(err)
	}
	time.Sleep(time.Millisecond)
	if _, _, err := registry.ReserveSession(resp.RunnerID, "session-a-newest"); err != nil {
		t.Fatal(err)
	}

	followupReq := liveRunnerTestHeartbeat(resp.RunnerID)
	followupReq.Capacity = 3
	followupReq.SessionIDs = []string{"runner-reported-session"}
	nextResp, err := registry.Heartbeat(followupReq, resp.HeartbeatSecret)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"session-z-oldest", "session-a-newest"}
	if !slices.Equal(nextResp.SessionIDs, want) {
		t.Fatalf("unexpected session ids: got %+v want %+v", nextResp.SessionIDs, want)
	}

	registry.mu.Lock()
	runner = registry.runners[resp.RunnerID]
	registry.mu.Unlock()
	runner.mu.Lock()
	defer runner.mu.Unlock()
	if !slices.Equal(runner.LiveRunnerHeartbeatRequest.SessionIDs, []string{"runner-reported-session"}) {
		t.Fatalf("expected follow-up request session_ids to be retained, got %+v", runner.LiveRunnerHeartbeatRequest.SessionIDs)
	}
}

func TestLiveRunnerRegistry_InvalidHeartbeatAuthDoesNotLeakRegistryLock(t *testing.T) {
	registry := newLiveRunnerTestRegistry()
	req := liveRunnerTestHeartbeat("runner-lock-leak")
	resp := liveRunnerTestRegister(t, registry, req)

	const attempts = 200
	done := make(chan error, attempts+1)
	start := make(chan struct{})

	for i := 0; i < attempts; i++ {
		go func(i int) {
			<-start
			badReq := req
			_, err := registry.Heartbeat(badReq, fmt.Sprintf("wrong-secret-%d", i))
			if !isRunnerErrorStatus(err, http.StatusUnauthorized) {
				done <- fmt.Errorf("wrong-secret heartbeat %d: expected unauthorized, got %v", i, err)
				return
			}
			done <- nil
		}(i)
	}

	go func() {
		<-start
		for i := 0; i < attempts; i++ {
			if runners := registry.Runners(); len(runners) != 1 {
				done <- fmt.Errorf("Runners during bad-auth storm: expected one runner, got %d", len(runners))
				return
			}
			goodReq := req
			if _, err := registry.Heartbeat(goodReq, resp.HeartbeatSecret); err != nil {
				done <- fmt.Errorf("valid heartbeat during bad-auth storm: %w", err)
				return
			}
		}
		done <- nil
	}()

	close(start)
	timeout := time.After(5 * time.Second)
	for i := 0; i < attempts+1; i++ {
		select {
		case err := <-done:
			if err != nil {
				t.Fatal(err)
			}
		case <-timeout:
			t.Fatal("timed out waiting for concurrent heartbeat auth checks; registry lock may be leaked")
		}
	}
}

func TestLiveRunnerRegistry_HeartbeatDisabledWithoutRegistrationSecret(t *testing.T) {
	registry := NewLiveRunnerRegistry(LiveRunnerRegistryConfig{Host: liveRunnerTestHostWithoutSecret{}})
	_, err := registry.Heartbeat(liveRunnerTestHeartbeat("runner-1"), "")
	if !isRunnerErrorStatus(err, http.StatusNotFound) {
		t.Fatalf("expected dynamic registration disabled error, got %v", err)
	}
}

func TestLiveRunnerRegistry_DefaultCapacityWhenUnset(t *testing.T) {
	registry := newLiveRunnerTestRegistry()
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

func TestLiveRunnerRegistry_DefaultModeWhenUnset(t *testing.T) {
	registry := newLiveRunnerTestRegistry()
	resp := liveRunnerTestRegister(t, registry, liveRunnerTestHeartbeat("runner_default_mode"))

	mode, err := registry.RunnerMode(resp.RunnerID)
	if err != nil {
		t.Fatal(err)
	}
	if mode != LiveRunnerModePersistent {
		t.Fatalf("expected default mode %q, got %q", LiveRunnerModePersistent, mode)
	}
}

func TestLiveRunnerRegistry_HeartbeatSingleShotMode(t *testing.T) {
	registry := newLiveRunnerTestRegistry()
	req := liveRunnerTestHeartbeat("runner-single-shot")
	req.Mode = LiveRunnerModeSingleShot
	resp := liveRunnerTestRegister(t, registry, req)

	mode, err := registry.RunnerMode(resp.RunnerID)
	if err != nil {
		t.Fatal(err)
	}
	if mode != LiveRunnerModeSingleShot {
		t.Fatalf("expected mode %q, got %q", LiveRunnerModeSingleShot, mode)
	}
}

func TestLiveRunnerRegistry_HeartbeatSingleShotModeAlias(t *testing.T) {
	registry := newLiveRunnerTestRegistry()
	req := liveRunnerTestHeartbeat("runner-single-shot-alias")
	req.Mode = "single_shot"
	resp := liveRunnerTestRegister(t, registry, req)

	mode, err := registry.RunnerMode(resp.RunnerID)
	if err != nil {
		t.Fatal(err)
	}
	if mode != LiveRunnerModeSingleShot {
		t.Fatalf("expected mode %q, got %q", LiveRunnerModeSingleShot, mode)
	}
}

func TestLiveRunnerRegistry_InvalidMode(t *testing.T) {
	registry := newLiveRunnerTestRegistry()
	req := liveRunnerTestHeartbeat("runner-invalid-mode")
	req.Mode = "invalid"
	if _, err := registry.Heartbeat(req, liveRunnerTestBootstrapSecret); !isRunnerErrorStatus(err, http.StatusBadRequest) || !strings.Contains(err.Error(), "mode") {
		t.Fatalf("expected invalid mode bad request, got %v", err)
	}
}

func TestLiveRunnerRegistry_OnchainRequiresHeartbeatPriceInfo(t *testing.T) {
	registry := newOnchainLiveRunnerTestRegistry()
	req := liveRunnerTestHeartbeat("runner-missing-price")

	_, err := registry.Heartbeat(req, liveRunnerTestBootstrapSecret)
	if !isRunnerErrorStatus(err, http.StatusBadRequest) || !strings.Contains(err.Error(), "price_info") {
		t.Fatalf("expected missing price_info bad request, got %v", err)
	}
}

func TestLiveRunnerRegistry_OnchainRequiresStaticRunnerPriceInfo(t *testing.T) {
	registry := newOnchainLiveRunnerTestRegistry()

	_, err := registry.RegisterStaticRunners(StaticLiveRunnerConfig{Runners: []StaticLiveRunnerConfigEntry{{
		Label:     "static-missing-price",
		RunnerURL: "https://runner.example.com",
		App:       "live-video-to-video/scope",
		HealthURL: "https://runner.example.com/health",
	}}})
	if !isRunnerErrorStatus(err, http.StatusBadRequest) || !strings.Contains(err.Error(), "price_info") {
		t.Fatalf("expected missing price_info bad request, got %v", err)
	}
}

func TestLiveRunnerRegistry_OnchainAcceptsWEIPrice(t *testing.T) {
	registry := newOnchainLiveRunnerTestRegistry()
	req := liveRunnerTestHeartbeat("runner-wei-price")
	req.PriceInfo = LiveRunnerPriceInfo{PricePerUnit: 10, PixelsPerUnit: 2, Unit: "wei"}

	liveRunnerTestRegister(t, registry, req)
	runners := registry.Runners()
	if len(runners) != 1 {
		t.Fatalf("expected one runner, got %d", len(runners))
	}
	if runners[0].PriceInfo == nil || *runners[0].PriceInfo != (LiveRunnerPriceInfo{PricePerUnit: 10, PixelsPerUnit: 2, Unit: "WEI"}) {
		t.Fatalf("unexpected runner price info: %+v", runners[0].PriceInfo)
	}
}

func TestLiveRunnerRegistry_OffchainIgnoresUSDPriceWatcher(t *testing.T) {
	prevWatcher := core.PriceFeedWatcher
	core.PriceFeedWatcher = nil
	defer func() { core.PriceFeedWatcher = prevWatcher }()

	registry := newLiveRunnerTestRegistry()
	req := liveRunnerTestHeartbeat("runner-offchain-usd")
	req.PriceInfo = LiveRunnerPriceInfo{PricePerUnit: 10, PixelsPerUnit: 1, Unit: "USD"}

	liveRunnerTestRegister(t, registry, req)
	runners := registry.Runners()
	if len(runners) != 1 {
		t.Fatalf("expected one runner, got %d", len(runners))
	}
	if runners[0].PriceInfo != nil {
		t.Fatalf("expected offchain discovery price to be suppressed, got %+v", runners[0].PriceInfo)
	}
}

func TestLiveRunnerRegistry_ClonesRunnerGPUMetadata(t *testing.T) {
	registry := newLiveRunnerTestRegistry()
	req := liveRunnerTestHeartbeat("runner-cloned-gpu")
	req.GPU.Name = "original-dynamic"
	liveRunnerTestRegister(t, registry, req)
	req.GPU.Name = "mutated-dynamic"

	runners := registry.Runners()
	if len(runners) != 1 {
		t.Fatalf("expected one dynamic runner, got %d", len(runners))
	}
	if runners[0].GPU == nil || runners[0].GPU.Name != "original-dynamic" {
		t.Fatalf("expected dynamic runner GPU to be cloned, got %+v", runners[0].GPU)
	}

	healthSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer healthSrv.Close()

	staticGPU := &LiveRunnerGPU{Name: "original-static"}
	if _, err := registry.RegisterStaticRunners(StaticLiveRunnerConfig{Runners: []StaticLiveRunnerConfigEntry{{
		Label:     "static-cloned-gpu",
		RunnerURL: "https://runner.example.com",
		GPU:       staticGPU,
		App:       "live-video-to-video/scope",
		HealthURL: healthSrv.URL,
	}}}); err != nil {
		t.Fatal(err)
	}
	staticGPU.Name = "mutated-static"

	runners = registry.Runners()
	var foundStatic bool
	for _, runner := range runners {
		if runner.App != "live-video-to-video/scope" {
			continue
		}
		foundStatic = true
		if runner.GPU == nil || runner.GPU.Name != "original-static" {
			t.Fatalf("expected static runner GPU to be cloned, got %+v", runner.GPU)
		}
	}
	if !foundStatic {
		t.Fatal("expected static runner in discovery")
	}
}

func TestParseStaticLiveRunnerConfigDefaultsHealthStatusAndNumericGPU(t *testing.T) {
	cfg, err := ParseStaticLiveRunnerConfig([]byte(`{"runners":[{"label":"app","runner_url":"https://runner.example.com","app":"live-video-to-video/scope","capacity":1,"health_url":"https://runner.example.com/health","gpu":-1}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Runners) != 1 {
		t.Fatalf("expected one runner, got %d", len(cfg.Runners))
	}
	if cfg.Runners[0].Route != LiveRunnerRoutingRunnerID {
		t.Fatalf("expected default routing %q, got %q", LiveRunnerRoutingRunnerID, cfg.Runners[0].Route)
	}
	if cfg.Runners[0].HealthyStatusCode != http.StatusOK {
		t.Fatalf("expected default health status 200, got %d", cfg.Runners[0].HealthyStatusCode)
	}
	if cfg.Runners[0].GPU == nil || cfg.Runners[0].GPU.ID != "-1" {
		t.Fatalf("expected numeric gpu fallback, got %+v", cfg.Runners[0].GPU)
	}
	if cfg.Runners[0].Mode != "" {
		t.Fatalf("expected raw static config mode to remain empty before registration, got %q", cfg.Runners[0].Mode)
	}
}

func TestParseStaticLiveRunnerConfigRoute(t *testing.T) {
	cfg, err := ParseStaticLiveRunnerConfig([]byte(`{"runners":[{"label":"app","routing":"label","runner_url":"https://runner.example.com","app":"live-video-to-video/scope","health_url":"https://runner.example.com/health"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Runners[0].Route != LiveRunnerRoutingLabel {
		t.Fatalf("expected routing %q, got %q", LiveRunnerRoutingLabel, cfg.Runners[0].Route)
	}
	if _, err := ParseStaticLiveRunnerConfig([]byte(`{"runners":[{"label":"app","routing":"bad","runner_url":"https://runner.example.com","app":"live-video-to-video/scope","health_url":"https://runner.example.com/health"}]}`)); err == nil || !strings.Contains(err.Error(), "routing") {
		t.Fatalf("expected invalid routing error, got %v", err)
	}
}

func TestLiveRunnerRegistry_RegisterStaticRunnersResolvesHealthPath(t *testing.T) {
	healthSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer healthSrv.Close()

	registry := newLiveRunnerTestRegistry()
	resp, err := registry.RegisterStaticRunners(StaticLiveRunnerConfig{Runners: []StaticLiveRunnerConfigEntry{{
		Label:     "path-health",
		RunnerURL: healthSrv.URL,
		App:       "live-video-to-video/scope",
		Capacity:  1,
		HealthURL: "/health",
	}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Runners) != 1 {
		t.Fatalf("expected one runner, got %d", len(resp.Runners))
	}
	if want := healthSrv.URL + "/health"; resp.Runners[0].HealthURL != want {
		t.Fatalf("expected health URL %q, got %q", want, resp.Runners[0].HealthURL)
	}
	if !resp.Runners[0].Healthy {
		t.Fatal("expected health path to be checked against runner URL")
	}
}

func TestLiveRunnerRegistry_RegisterStaticRunnersSingleShotMode(t *testing.T) {
	healthSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer healthSrv.Close()

	registry := newLiveRunnerTestRegistry()
	resp, err := registry.RegisterStaticRunners(StaticLiveRunnerConfig{Runners: []StaticLiveRunnerConfigEntry{{
		Label:     "static-single-shot",
		Route:     LiveRunnerRoutingLabel,
		RunnerURL: "https://runner.example.com",
		App:       "live-video-to-video/scope",
		Mode:      LiveRunnerModeSingleShot,
		Capacity:  1,
		HealthURL: healthSrv.URL,
	}}})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Runners[0].Mode != LiveRunnerModeSingleShot {
		t.Fatalf("expected registration mode %q, got %q", LiveRunnerModeSingleShot, resp.Runners[0].Mode)
	}
	mode, err := registry.RunnerMode("static-single-shot")
	if err != nil {
		t.Fatal(err)
	}
	if mode != LiveRunnerModeSingleShot {
		t.Fatalf("expected runner mode %q, got %q", LiveRunnerModeSingleShot, mode)
	}
	runners := registry.Runners()
	if len(runners) != 1 || runners[0].URL != "http://localhost:1234/apps/static-single-shot/app" {
		t.Fatalf("unexpected single-shot route discovery: %+v", runners)
	}
}

func TestLiveRunnerRegistry_RegisterStaticRunnersHealthAndNoHeartbeatExpiry(t *testing.T) {
	healthSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer healthSrv.Close()

	registry := newLiveRunnerTestRegistry()
	resp, err := registry.RegisterStaticRunners(StaticLiveRunnerConfig{Runners: []StaticLiveRunnerConfigEntry{{
		Label:     "static-app",
		RunnerURL: "https://runner.example.com",
		App:       "live-video-to-video/scope",
		Capacity:  1,
		HealthURL: healthSrv.URL,
	}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Runners) != 1 || resp.Runners[0].RunnerID == "" || !resp.Runners[0].Healthy {
		t.Fatalf("unexpected static registration response: %+v", resp)
	}
	runnerID := resp.Runners[0].RunnerID
	if _, _, err := registry.ReserveSession(runnerID); err != nil {
		t.Fatalf("expected healthy static runner to reserve session: %v", err)
	}

	registry.heartbeatTTL = time.Nanosecond
	time.Sleep(time.Millisecond)
	runners := registry.Runners()
	if len(runners) != 1 {
		t.Fatalf("expected static runner to skip heartbeat expiry, got %d", len(runners))
	}
}

func TestLiveRunnerRegistry_RegisterStaticRunnersAtomicValidation(t *testing.T) {
	healthSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer healthSrv.Close()

	registry := newLiveRunnerTestRegistry()
	valid := StaticLiveRunnerConfigEntry{
		Label:     "runner-a",
		RunnerURL: "https://runner.example.com",
		App:       "live-video-to-video/scope",
		HealthURL: healthSrv.URL,
	}
	if _, err := registry.RegisterStaticRunners(StaticLiveRunnerConfig{Runners: []StaticLiveRunnerConfigEntry{valid}}); err != nil {
		t.Fatal(err)
	}
	if len(registry.Runners()) != 1 {
		t.Fatal("expected initial static runner")
	}

	invalid := valid
	invalid.Label = "runner-b"
	invalid.RunnerURL = "bad url"
	if _, err := registry.RegisterStaticRunners(StaticLiveRunnerConfig{Runners: []StaticLiveRunnerConfigEntry{valid, invalid}}); !isRunnerErrorStatus(err, http.StatusBadRequest) {
		t.Fatalf("expected bad request for invalid batch, got %v", err)
	}
	if len(registry.Runners()) != 1 {
		t.Fatalf("expected invalid batch to leave existing runners unchanged, got %d", len(registry.Runners()))
	}
}

func TestLiveRunnerRegistry_RegisterStaticRunnersRequiresUniqueLabels(t *testing.T) {
	healthSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer healthSrv.Close()

	registry := newLiveRunnerTestRegistry()
	entry := StaticLiveRunnerConfigEntry{
		Label:     "runner-a",
		RunnerURL: "https://runner.example.com",
		App:       "live-video-to-video/scope",
		HealthURL: healthSrv.URL,
	}
	duplicate := entry
	duplicate.RunnerURL = "https://runner-two.example.com"
	if _, err := registry.RegisterStaticRunners(StaticLiveRunnerConfig{Runners: []StaticLiveRunnerConfigEntry{entry, duplicate}}); !isRunnerErrorStatus(err, http.StatusBadRequest) {
		t.Fatalf("expected duplicate label error, got %v", err)
	}

	missingLabel := entry
	missingLabel.Label = ""
	if _, err := registry.RegisterStaticRunners(StaticLiveRunnerConfig{Runners: []StaticLiveRunnerConfigEntry{missingLabel}}); !isRunnerErrorStatus(err, http.StatusBadRequest) || !strings.Contains(err.Error(), "label") {
		t.Fatalf("expected missing label error, got %v", err)
	}
}

func TestLiveRunnerRegistry_RegisterStaticRunnersUpsertsWithoutDroppingSession(t *testing.T) {
	healthSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer healthSrv.Close()

	registry := newLiveRunnerTestRegistry()
	entry := StaticLiveRunnerConfigEntry{
		Label:     "first",
		RunnerURL: "https://runner.example.com",
		App:       "live-video-to-video/scope",
		Capacity:  2,
		HealthURL: healthSrv.URL,
	}
	resp1, err := registry.RegisterStaticRunners(StaticLiveRunnerConfig{Runners: []StaticLiveRunnerConfigEntry{entry}})
	if err != nil {
		t.Fatal(err)
	}
	runnerID := resp1.Runners[0].RunnerID
	sessionID, _, err := registry.ReserveSession(runnerID)
	if err != nil {
		t.Fatal(err)
	}

	entry.RunnerURL = "https://runner-updated.example.com"
	resp2, err := registry.RegisterStaticRunners(StaticLiveRunnerConfig{Runners: []StaticLiveRunnerConfigEntry{entry}})
	if err != nil {
		t.Fatal(err)
	}
	if resp2.Runners[0].RunnerID != runnerID {
		t.Fatalf("expected static runner ID to be preserved, got %s want %s", resp2.Runners[0].RunnerID, runnerID)
	}
	if _, err := registry.RunnerEndpointForSession(runnerID, sessionID); err != nil {
		t.Fatalf("expected active session to survive healthy static upsert: %v", err)
	}
}

func TestLiveRunnerRegistry_RegisterStaticRunnersConcurrentUpserts(t *testing.T) {
	healthSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer healthSrv.Close()

	registry := newLiveRunnerTestRegistry()
	defer registry.Stop()

	entryA := StaticLiveRunnerConfigEntry{
		Label:     "runner-a",
		RunnerURL: "https://runner-a.example.com",
		App:       "live-video-to-video/scope",
		HealthURL: healthSrv.URL,
	}
	entryB := StaticLiveRunnerConfigEntry{
		Label:     "runner-b",
		RunnerURL: "https://runner-b.example.com",
		App:       "live-video-to-video/scope",
		HealthURL: healthSrv.URL,
	}

	const registrationWorkers = 100
	var wg sync.WaitGroup
	errCh := make(chan error, registrationWorkers)
	for i := 0; i < registrationWorkers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			entries := []StaticLiveRunnerConfigEntry{entryA, entryB}
			if i%2 == 1 {
				entries = []StaticLiveRunnerConfigEntry{entryB, entryA}
			}
			if _, err := registry.RegisterStaticRunners(StaticLiveRunnerConfig{Runners: entries}); err != nil {
				errCh <- err
			}
		}(i)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("concurrent static registrations timed out")
	}
	close(errCh)
	for err := range errCh {
		t.Fatal(err)
	}
	if runners := registry.Runners(); len(runners) != 2 {
		t.Fatalf("expected concurrent static upserts to settle on two runners, got %d", len(runners))
	}
}

func TestLiveRunnerRegistry_StaticRunnerRouteLabel(t *testing.T) {
	healthSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer healthSrv.Close()

	registry := newLiveRunnerTestRegistry()
	_, err := registry.RegisterStaticRunners(StaticLiveRunnerConfig{
		Runners: []StaticLiveRunnerConfigEntry{{
			Label:     "first",
			Route:     LiveRunnerRoutingLabel,
			RunnerURL: "https://runner.example.com",
			App:       "live-video-to-video/scope",
			Capacity:  2,
			HealthURL: healthSrv.URL,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	runners := registry.Runners()
	if len(runners) != 1 {
		t.Fatalf("expected one runner, got %d", len(runners))
	}
	if runners[0].URL != "http://localhost:1234/apps/first/session" {
		t.Fatalf("unexpected label route url: %s", runners[0].URL)
	}
	sessionID, _, err := registry.ReserveSession("first")
	if err != nil {
		t.Fatalf("expected label route to reserve: %v", err)
	}
	if _, err := registry.RunnerEndpointForSession("first", sessionID); err != nil {
		t.Fatalf("expected label route to resolve: %v", err)
	}
}

func TestLiveRunnerRegistry_StaticRunnerRouteChangesOnUpsert(t *testing.T) {
	healthSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer healthSrv.Close()

	registry := newLiveRunnerTestRegistry()
	entry := StaticLiveRunnerConfigEntry{
		Label:     "stable-route",
		Route:     LiveRunnerRoutingRunnerID,
		RunnerURL: "https://runner.example.com",
		App:       "live-video-to-video/scope",
		HealthURL: healthSrv.URL,
	}
	resp, err := registry.RegisterStaticRunners(StaticLiveRunnerConfig{Runners: []StaticLiveRunnerConfigEntry{entry}})
	if err != nil {
		t.Fatal(err)
	}
	runnerID := resp.Runners[0].RunnerID

	entry.Route = LiveRunnerRoutingLabel
	if _, err := registry.RegisterStaticRunners(StaticLiveRunnerConfig{Runners: []StaticLiveRunnerConfigEntry{entry}}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := registry.ReserveSession("stable-route"); err != nil {
		t.Fatalf("expected upsert to move static runner to label route: %v", err)
	}
	if _, _, err := registry.ReserveSession(runnerID); !isRunnerErrorStatus(err, http.StatusNotFound) {
		t.Fatalf("expected original runner-id route to be removed, got %v", err)
	}
}

func TestLiveRunnerRegistry_StaticRunnerCustomHealthStatusAndUnhealthyRelease(t *testing.T) {
	statusCode := http.StatusCreated
	healthSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(statusCode)
	}))
	defer healthSrv.Close()

	registry := newLiveRunnerTestRegistry()
	resp, err := registry.RegisterStaticRunners(StaticLiveRunnerConfig{Runners: []StaticLiveRunnerConfigEntry{{
		Label:             "custom-health",
		RunnerURL:         "https://runner.example.com",
		App:               "live-video-to-video/scope",
		Capacity:          1,
		HealthURL:         healthSrv.URL,
		HealthyStatusCode: http.StatusCreated,
	}}})
	if err != nil {
		t.Fatal(err)
	}
	runnerID := resp.Runners[0].RunnerID
	sessionID, _, err := registry.ReserveSession(runnerID)
	if err != nil {
		t.Fatal(err)
	}
	statusCode = http.StatusOK
	registry.checkStaticRunnerHealth()
	if _, err := registry.RunnerEndpointForSession(runnerID, sessionID); !isRunnerErrorStatus(err, http.StatusNotFound) {
		t.Fatalf("expected unhealthy static runner to release sessions, got %v", err)
	}
	if runners := registry.Runners(); len(runners) != 0 {
		data, _ := json.Marshal(runners)
		t.Fatalf("expected unhealthy static runner to be hidden, got %s", data)
	}
}

func TestLiveRunnerRegistry_RunnersDiscoveryShape(t *testing.T) {
	registry := newLiveRunnerTestRegistry()

	liveRunnerTestRegister(t, registry, liveRunnerTestHeartbeat("runner_discovery_1"))

	runners := registry.Runners()
	if len(runners) != 1 {
		t.Fatalf("expected one runner, got %d", len(runners))
	}
	runner := runners[0]
	if runner.URL != "http://localhost:1234/apps/runner_discovery_1/session" {
		t.Fatalf("unexpected url: %s", runner.URL)
	}
	if runner.GPU == nil || runner.GPU.Name != "NVIDIA L40S" {
		t.Fatalf("unexpected gpu: %+v", runner.GPU)
	}
	if runner.App != "new-ai-pipeline/model-a" {
		t.Fatalf("unexpected app: %s", runner.App)
	}
	if runner.PriceInfo != nil {
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

	registry := newOnchainLiveRunnerTestRegistry()
	req := liveRunnerTestHeartbeat("runner-1")
	req.PriceInfo = LiveRunnerPriceInfo{PricePerUnit: 10, PixelsPerUnit: 1, Unit: "USD"}
	liveRunnerTestRegister(t, registry, req)

	runners := registry.Runners()
	if len(runners) != 1 {
		t.Fatalf("expected one runner, got %d", len(runners))
	}
	if runners[0].PriceInfo == nil {
		t.Fatal("expected runner price info")
	}
	got := big.NewRat(runners[0].PriceInfo.PricePerUnit, runners[0].PriceInfo.PixelsPerUnit)
	// Per-second default: 10 USD/s at 2000 USD/ETH = 5e15 wei/s, published as wei per
	// nanosecond (PixelsPerUnit = 1e9).
	expected := big.NewRat(5_000_000_000_000_000, 1_000_000_000)
	if got.Cmp(expected) != 0 {
		t.Fatalf("unexpected converted price: got=%s want=%s", got, expected)
	}
	if runners[0].PriceInfo.Unit != "WEI" {
		t.Fatalf("unexpected converted unit: %s", runners[0].PriceInfo.Unit)
	}
}

func TestLiveRunnerRegistry_SharedURLAppKeepsPerRunnerPrices(t *testing.T) {
	registry := newOnchainLiveRunnerTestRegistry()

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
	if runners[0].PriceInfo == nil {
		t.Fatal("expected runner price info")
	}
	if got := runners[0].PriceInfo.PricePerUnit; got != 20 {
		t.Fatalf("expected remaining runner price to stay isolated, got %d", got)
	}
}

func TestLiveRunnerRegistry_ReserveSessionCapacity(t *testing.T) {
	registry := newLiveRunnerTestRegistry()
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

func TestLiveRunnerRegistry_ReserveSessionUsesProvidedID(t *testing.T) {
	registry := newLiveRunnerTestRegistry()
	liveRunnerTestRegister(t, registry, liveRunnerTestHeartbeat("runner-1"))

	sessionID, endpoint, err := registry.ReserveSession("runner-1", "manifest-id")
	if err != nil {
		t.Fatal(err)
	}
	if sessionID != "manifest-id" {
		t.Fatalf("expected provided session id, got %q", sessionID)
	}
	if endpoint != "https://runner.example.com" {
		t.Fatalf("unexpected endpoint: %s", endpoint)
	}
}

func TestLiveRunnerRegistry_ReleaseSessionFreesCapacity(t *testing.T) {
	registry := newLiveRunnerTestRegistry()
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
	registry := newLiveRunnerTestRegistry()
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

func TestLiveRunnerRegistry_ConcurrentReservationsAreRunnerScoped(t *testing.T) {
	registry := newLiveRunnerTestRegistry()
	const requestsPerRunner = 64
	for _, runnerID := range []string{"runner-a", "runner-b"} {
		req := liveRunnerTestHeartbeat(runnerID)
		req.RunnerURL = fmt.Sprintf("https://%s.example.com", runnerID)
		req.Capacity = requestsPerRunner
		liveRunnerTestRegister(t, registry, req)
	}

	registry.mu.Lock()
	runnerA := registry.runners["runner-a"]
	if runnerA == nil {
		registry.mu.Unlock()
		t.Fatal("expected runner-a to be registered")
	}
	runnerA.mu.Lock()

	var runnerAWG sync.WaitGroup
	runnerAErrCh := make(chan error, requestsPerRunner)
	startRunnerA := make(chan struct{})
	for i := 0; i < requestsPerRunner; i++ {
		runnerAWG.Add(1)
		go func(i int) {
			defer runnerAWG.Done()
			<-startRunnerA
			sessionID := fmt.Sprintf("runner-a-session-%d", i)
			gotSessionID, endpoint, err := registry.ReserveSession("runner-a", sessionID)
			if err != nil {
				runnerAErrCh <- fmt.Errorf("runner-a reserve %s: %w", sessionID, err)
				return
			}
			if gotSessionID != sessionID {
				runnerAErrCh <- fmt.Errorf("runner-a reserve got session %q want %q", gotSessionID, sessionID)
				return
			}
			if endpoint != "https://runner-a.example.com" {
				runnerAErrCh <- fmt.Errorf("runner-a endpoint got %q want %q", endpoint, "https://runner-a.example.com")
			}
		}(i)
	}
	close(startRunnerA)
	registry.mu.Unlock()

	time.Sleep(10 * time.Millisecond)

	var runnerBWG sync.WaitGroup
	runnerBErrCh := make(chan error, requestsPerRunner)
	runnerBDone := make(chan struct{})
	for i := 0; i < requestsPerRunner; i++ {
		runnerBWG.Add(1)
		go func(i int) {
			defer runnerBWG.Done()
			sessionID := fmt.Sprintf("runner-b-session-%d", i)
			gotSessionID, endpoint, err := registry.ReserveSession("runner-b", sessionID)
			if err != nil {
				runnerBErrCh <- fmt.Errorf("runner-b reserve %s: %w", sessionID, err)
				return
			}
			if gotSessionID != sessionID {
				runnerBErrCh <- fmt.Errorf("runner-b reserve got session %q want %q", gotSessionID, sessionID)
				return
			}
			if endpoint != "https://runner-b.example.com" {
				runnerBErrCh <- fmt.Errorf("runner-b endpoint got %q want %q", endpoint, "https://runner-b.example.com")
			}
		}(i)
	}
	go func() {
		runnerBWG.Wait()
		close(runnerBDone)
	}()

	select {
	case <-runnerBDone:
	case <-time.After(250 * time.Millisecond):
		runnerA.mu.Unlock()
		t.Fatal("runner-b reservations blocked while runner-a operations were waiting on runner-a.mu")
	}
	close(runnerBErrCh)
	for err := range runnerBErrCh {
		if err != nil {
			runnerA.mu.Unlock()
			t.Fatal(err)
		}
	}

	runnerADone := make(chan struct{})
	go func() {
		runnerAWG.Wait()
		close(runnerADone)
	}()
	runnerA.mu.Unlock()
	select {
	case <-runnerADone:
	case <-time.After(250 * time.Millisecond):
		t.Fatal("runner-a reservations did not complete after runner-a.mu was released")
	}
	close(runnerAErrCh)
	for err := range runnerAErrCh {
		if err != nil {
			t.Fatal(err)
		}
	}

	runners := registry.Runners()
	if len(runners) != 2 {
		t.Fatalf("expected two runners, got %+v", runners)
	}
	usedByURL := make(map[string]int, 2)
	availableByURL := make(map[string]int, 2)
	for _, runner := range runners {
		usedByURL[runner.URL] = runner.CapacityUsed
		availableByURL[runner.URL] = runner.CapacityAvailable
	}
	for _, runnerID := range []string{"runner-a", "runner-b"} {
		url := fmt.Sprintf("http://localhost:1234/apps/%s/session", runnerID)
		if usedByURL[url] != requestsPerRunner {
			t.Fatalf("%s used capacity got %d want %d", runnerID, usedByURL[url], requestsPerRunner)
		}
		if availableByURL[url] != 0 {
			t.Fatalf("%s available capacity got %d want %d", runnerID, availableByURL[url], 0)
		}
	}
}

func TestLiveRunnerRegistry_UnregisterRacesWithSessionActions(t *testing.T) {
	registry := newLiveRunnerTestRegistry()
	req := liveRunnerTestHeartbeat("runner-race")
	req.Capacity = 100
	resp := liveRunnerTestRegister(t, registry, req)
	sessionID, _, err := registry.ReserveSession("runner-race")
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	errCh := make(chan error, 101)
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			switch i % 3 {
			case 0:
				_, err := registry.RunnerEndpointForSession("runner-race", sessionID)
				if err != nil && !isRunnerErrorStatus(err, http.StatusNotFound) {
					errCh <- fmt.Errorf("endpoint action: %w", err)
				}
			case 1:
				err := registry.ValidSessionToken("runner-race", sessionID, "bad-token")
				if err != nil && !isRunnerErrorStatus(err, http.StatusUnauthorized) && !isRunnerErrorStatus(err, http.StatusNotFound) {
					errCh <- fmt.Errorf("token action: %w", err)
				}
			default:
				if err := registry.ReleaseSession("runner-race", sessionID); err != nil && !isRunnerErrorStatus(err, http.StatusNotFound) {
					errCh <- fmt.Errorf("release action: %w", err)
				}
			}
		}(i)
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := registry.Unregister("runner-race", resp.HeartbeatSecret); err != nil && !isRunnerErrorStatus(err, http.StatusNotFound) {
			errCh <- fmt.Errorf("unregister: %w", err)
		}
	}()
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatal(err)
		}
	}
	if _, err := registry.RunnerEndpointForSession("runner-race", sessionID); !isRunnerErrorStatus(err, http.StatusNotFound) {
		t.Fatalf("expected runner to be gone after unregister, got %v", err)
	}
}

func TestLiveRunnerRegistry_CreateTrickleChannelForSession(t *testing.T) {
	trickleSrv, channelBaseURL, _ := newLiveRunnerTestTrickleServer(t)
	registry := newLiveRunnerTestRegistry()
	registry.SetTrickleServer(trickleSrv, channelBaseURL, channelBaseURL)
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
	registry := newLiveRunnerTestRegistry()
	registry.SetTrickleServer(trickleSrv, channelBaseURL, channelBaseURL)
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
	registry := newLiveRunnerTestRegistry()
	registry.SetTrickleServer(trickleSrv, channelBaseURL, channelBaseURL)
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
	registry := newLiveRunnerTestRegistry()
	registry.SetTrickleServer(trickleSrv, channelBaseURL, channelBaseURL)
	resp1 := liveRunnerTestRegister(t, registry, liveRunnerTestHeartbeat("runner-1"))
	resp1Channels := liveRunnerTestBootstrapChannels(resp1)
	if len(resp1Channels) != 1 {
		t.Fatalf("expected bootstrap trickle channels, got %+v", resp1)
	}
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
	for _, channel := range resp1Channels {
		if status := channelStatus(channel.ChannelName); status != http.StatusNotFound {
			t.Fatalf("expected unregister to close bootstrap channel %q, got status=%d", channel.ChannelName, status)
		}
	}

	registry.heartbeatTTL = defaultLiveRunnerHeartbeatTTL
	resp2 := liveRunnerTestRegister(t, registry, liveRunnerTestHeartbeat("runner-2"))
	resp2Channels := liveRunnerTestBootstrapChannels(resp2)
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
		t.Fatalf("expected expired runner to be unusable, got %v", err)
	}
	if status := channelStatus(sessionID + "-cleanup"); status != http.StatusOK {
		t.Fatalf("expected expired runner cleanup to be asynchronous, got status=%d", status)
	}
	registry.removeExpiredRunners(time.Now())
	if status := channelStatus(sessionID + "-cleanup"); status != http.StatusNotFound {
		t.Fatalf("expected expiry to close channel, got status=%d", status)
	}
	for _, channel := range resp2Channels {
		if status := channelStatus(channel.ChannelName); status != http.StatusNotFound {
			t.Fatalf("expected expiry to close bootstrap channel %q, got status=%d", channel.ChannelName, status)
		}
	}
}

func TestLiveRunnerRegistry_ExpireAndUnregisterClearSessions(t *testing.T) {
	registry := newLiveRunnerTestRegistry()
	liveRunnerTestRegister(t, registry, liveRunnerTestHeartbeat("runner-1"))
	sessionID, _, err := registry.ReserveSession("runner-1")
	if err != nil {
		t.Fatal(err)
	}
	registry.heartbeatTTL = time.Nanosecond
	time.Sleep(time.Millisecond)
	if _, err := registry.RunnerEndpointForSession("runner-1", sessionID); !isRunnerErrorStatus(err, http.StatusNotFound) {
		t.Fatalf("expected expired runner to be unusable, got %v", err)
	}
	if runners := registry.Runners(); len(runners) != 0 {
		t.Fatalf("expected expired runner to be hidden from discovery, got %d", len(runners))
	}
	registry.removeExpiredRunners(time.Now())

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

func TestLiveRunnerRegistry_LockedRunnerDoesNotBlockUnrelatedRunnerLookup(t *testing.T) {
	registry := newLiveRunnerTestRegistry()
	defer registry.Stop()

	req1 := liveRunnerTestHeartbeat("runner-blocked")
	req1.Capacity = 100
	liveRunnerTestRegister(t, registry, req1)
	req2 := liveRunnerTestHeartbeat("runner-free")
	req2.Capacity = 100
	liveRunnerTestRegister(t, registry, req2)

	registry.mu.Lock()
	blockedRunner := registry.runners["runner-blocked"]
	registry.mu.Unlock()

	blockedRunner.mu.Lock()
	const blockedLookups = 50
	const freeLookups = 50
	startBlocked := make(chan struct{})
	blockedDone := make(chan error, blockedLookups)
	for i := 0; i < blockedLookups; i++ {
		go func() {
			<-startBlocked
			_, _, err := registry.ReserveSession("runner-blocked")
			blockedDone <- err
		}()
	}
	close(startBlocked)
	time.Sleep(10 * time.Millisecond)

	freeDone := make(chan error, freeLookups)
	for i := 0; i < freeLookups; i++ {
		go func(i int) {
			_, _, err := registry.ReserveSession("runner-free", fmt.Sprintf("free-%d", i))
			freeDone <- err
		}(i)
	}

	for i := 0; i < freeLookups; i++ {
		select {
		case err := <-freeDone:
			if err != nil {
				t.Fatalf("expected unrelated runner lookup to complete: %v", err)
			}
		case <-time.After(100 * time.Millisecond):
			t.Fatal("unrelated runner lookup blocked behind locked runner")
		}
	}

	blockedRunner.mu.Unlock()
	for i := 0; i < blockedLookups; i++ {
		if err := <-blockedDone; err != nil && !isRunnerErrorStatus(err, http.StatusServiceUnavailable) {
			t.Fatalf("expected blocked runner lookup to complete after unlock: %v", err)
		}
	}
}

func TestLiveRunnerRegistry_NotReadyCannotReserveSession(t *testing.T) {
	registry := newLiveRunnerTestRegistry()
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

func liveRunnerTestSetO2RKeepaliveInterval(t *testing.T, interval time.Duration) func() {
	t.Helper()
	previous := liveRunnerO2RKeepaliveInterval
	liveRunnerO2RKeepaliveInterval = interval
	return func() {
		liveRunnerO2RKeepaliveInterval = previous
	}
}

func liveRunnerTestReadTrickleMessage(t *testing.T, trickleSrv *trickle.Server, channelName string, match func(string) bool) string {
	t.Helper()
	sub := trickle.NewLocalSubscriber(trickleSrv, channelName)
	sub.SetSeq(0)
	deadline := time.Now().Add(time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		td, err := sub.Read()
		if err != nil {
			lastErr = err
			time.Sleep(10 * time.Millisecond)
			continue
		}
		data, err := io.ReadAll(td.Reader)
		if err != nil {
			t.Fatal(err)
		}
		msg := string(data)
		if match(msg) {
			return msg
		}
	}
	t.Fatalf("timed out reading matching trickle message from %q, lastErr=%v", channelName, lastErr)
	return ""
}

func liveRunnerTestReadSessionEvent(t *testing.T, trickleSrv *trickle.Server, channelName, event, sessionID string) {
	t.Helper()
	msg := liveRunnerTestReadTrickleMessage(t, trickleSrv, channelName, func(msg string) bool {
		var got struct {
			Event     string    `json:"event"`
			Session   string    `json:"session"`
			Timestamp time.Time `json:"timestamp"`
		}
		if err := json.Unmarshal([]byte(msg), &got); err != nil {
			return false
		}
		return got.Event == event && got.Session == sessionID && !got.Timestamp.IsZero()
	})
	var got struct {
		Event     string    `json:"event"`
		Session   string    `json:"session"`
		Timestamp time.Time `json:"timestamp"`
	}
	if err := json.Unmarshal([]byte(msg), &got); err != nil {
		t.Fatal(err)
	}
	if got.Event != event || got.Session != sessionID || got.Timestamp.IsZero() {
		t.Fatalf("unexpected session event: %+v", got)
	}
}
