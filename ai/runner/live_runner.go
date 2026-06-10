package runner

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/jaypipes/ghw"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/trickle"
)

const (
	defaultLiveRunnerHeartbeatInterval = 5 * time.Second
	defaultLiveRunnerHeartbeatTTL      = 20 * time.Second
	defaultLiveRunnerHealthInterval    = 5 * time.Second
	defaultLiveRunnerHealthStatusCode  = http.StatusOK
	defaultLiveRunnerHealthTimeout     = 3 * time.Second
	liveRunnerO2RKeepaliveMessage      = `{"keep":"alive"}`
	liveRunnerIDRandomBytes            = 5
	liveRunnerSeedBytes                = 32
)

var liveRunnerO2RKeepaliveInterval = 10 * time.Second

const (
	LiveRunnerModePersistent      = "persistent"
	LiveRunnerModeSingleShot      = "single-shot"
	liveRunnerModeSingleShotAlias = "single_shot"
)

const (
	LiveRunnerRoutingRunnerID = "runner-id"
	LiveRunnerRoutingLabel    = "label"
)

const (
	// o2r gives the orchestrator an inbound signaling path to the runner without
	// requiring a new runner port or quietly extending the runner app API.
	// Runner-originated calls still use normal HTTP endpoints for heartbeats,
	// trickle channel creation, and related control-plane requests.
	LiveRunnerTrickleOrchestratorToRunner = "o2r"
)

var liveRunnerEncoding = base32.StdEncoding.WithPadding(base32.NoPadding)

type RunnerError struct {
	StatusCode int
	Message    string
}

func (e *RunnerError) Error() string {
	if e == nil {
		return "runner error"
	}
	return e.Message
}

type LiveRunnerGPU struct {
	ID     string `json:"id,omitempty"`
	Name   string `json:"name,omitempty"`
	VRAMMB int    `json:"vram_mb,omitempty"`
}

type LiveRunnerPriceInfo struct {
	PricePerUnit  int64  `json:"price_per_unit"`
	PixelsPerUnit int64  `json:"pixels_per_unit"`
	Unit          string `json:"unit,omitempty"`
}

type LiveRunnerHeartbeatRequest struct {
	RunnerID   string              `json:"runner_id,omitempty"`
	Label      string              `json:"label,omitempty"`
	RunnerURL  string              `json:"runner_url"`
	Version    string              `json:"version,omitempty"`
	Status     string              `json:"status,omitempty"`
	Mode       string              `json:"mode,omitempty"`
	GPU        *LiveRunnerGPU      `json:"gpu,omitempty"`
	App        string              `json:"app"`
	Capacity   int                 `json:"capacity"`
	PriceInfo  LiveRunnerPriceInfo `json:"price_info"`
	SessionIDs []string            `json:"session_ids,omitempty"`
}

type StaticLiveRunnerConfig struct {
	Runners []StaticLiveRunnerConfigEntry `json:"runners"`
}

type StaticLiveRunnerConfigEntry struct {
	Label             string              `json:"label,omitempty"`
	Route             string              `json:"routing,omitempty"`
	RunnerURL         string              `json:"runner_url"`
	Version           string              `json:"version,omitempty"`
	Mode              string              `json:"mode,omitempty"`
	GPU               *LiveRunnerGPU      `json:"gpu,omitempty"`
	App               string              `json:"app"`
	Capacity          int                 `json:"capacity"`
	PriceInfo         LiveRunnerPriceInfo `json:"price_info"`
	HealthURL         string              `json:"health_url"`
	HealthyStatusCode int                 `json:"healthy_status_code,omitempty"`
}

type StaticLiveRunnerRegistration struct {
	RunnerID          string `json:"runner_id"`
	Label             string `json:"label,omitempty"`
	App               string `json:"app"`
	RunnerURL         string `json:"runner_url"`
	Mode              string `json:"mode,omitempty"`
	HealthURL         string `json:"health_url"`
	HealthyStatusCode int    `json:"healthy_status_code"`
	Healthy           bool   `json:"healthy"`
}

type StaticLiveRunnerRegistrationResponse struct {
	Runners []StaticLiveRunnerRegistration `json:"runners"`
}

type LiveRunnerHeartbeatResponse struct {
	RunnerID          string                    `json:"runner_id"`
	Orchestrator      string                    `json:"orchestrator,omitempty"`
	HeartbeatInterval string                    `json:"heartbeat_interval"`
	HeartbeatTTL      string                    `json:"heartbeat_ttl"`
	HeartbeatSecret   string                    `json:"heartbeat_secret,omitempty"`
	O2R               *LiveRunnerTrickleChannel `json:"o2r,omitempty"`
	SessionIDs        []string                  `json:"session_ids"`
}

type LiveRunnerDiscoveryRunner struct {
	URL               string               `json:"url"`
	GPU               *LiveRunnerGPU       `json:"gpu,omitempty"`
	App               string               `json:"app"`
	Version           string               `json:"version,omitempty"`
	Mode              string               `json:"mode,omitempty"`
	Capacity          int                  `json:"capacity"`
	CapacityUsed      int                  `json:"capacity_used"`
	CapacityAvailable int                  `json:"capacity_available"`
	PriceInfo         *LiveRunnerPriceInfo `json:"price_info,omitempty"`
}

type LiveRunnerTrickleChannel struct {
	Name        string `json:"name"`
	ChannelName string `json:"channel_name"`
	URL         string `json:"url"`
	InternalURL string `json:"internal_url,omitempty"`
	MimeType    string `json:"mime_type"`
}

type liveRunner struct {
	seed            []byte
	heartbeatSecret string
	static          bool
	offchain        bool
	o2r             *liveRunnerTrickleChannel

	// all fields below are mutable and need to be protected by mu
	mu sync.Mutex
	LiveRunnerHeartbeatRequest
	LastHeartbeat time.Time
	removed       bool
	healthURL     string
	healthStatus  int
	route         string
	sessions      map[string]*liveRunnerSession
	priceSource   LiveRunnerPriceInfo
	converter     *core.AutoConvertedPrice
}

type liveRunnerSession struct {
	// Protected by the parent liveRunner.mu.
	createdAt time.Time
	channels  map[string]*liveRunnerTrickleChannel
}

type liveRunnerTrickleChannel struct {
	channel   LiveRunnerTrickleChannel
	publisher *trickle.TrickleLocalPublisher
	done      chan struct{}
	messages  chan string
	closeOnce sync.Once
}

type LiveRunnerRegistry struct {
	host              RunnerHost
	offchain          bool
	heartbeatInterval time.Duration
	heartbeatTTL      time.Duration
	healthClient      *http.Client
	healthInterval    time.Duration
	stopRegistry      chan struct{}
	stopRegistryOnce  sync.Once

	staticRegistrationMu sync.Mutex // serializes static runner config upserts

	// mu protects runner registration, route lookup, and trickle server routing.
	mu                     sync.Mutex
	runners                map[string]*liveRunner
	trickleSrv             *trickle.Server
	publicTrickleBaseURL   string
	internalTrickleBaseURL string
}

type RunnerHost interface {
	ServiceURI() *url.URL
	LiveRunnerURI() *url.URL
	RegistrationSecret() string
}

type LiveRunnerRegistryConfig struct {
	Host    RunnerHost
	Onchain bool
}

var liveRunnerChannelNamePattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

func NewLiveRunnerRegistry(config LiveRunnerRegistryConfig) *LiveRunnerRegistry {
	r := &LiveRunnerRegistry{
		host:              config.Host,
		offchain:          !config.Onchain,
		runners:           make(map[string]*liveRunner),
		heartbeatInterval: defaultLiveRunnerHeartbeatInterval,
		heartbeatTTL:      defaultLiveRunnerHeartbeatTTL,
		healthClient:      &http.Client{Timeout: defaultLiveRunnerHealthTimeout},
		healthInterval:    defaultLiveRunnerHealthInterval,
		stopRegistry:      make(chan struct{}),
	}
	go r.healthLoop()
	go r.expiryLoop()
	return r
}

func (r *LiveRunnerRegistry) SetTrickleServer(srv *trickle.Server, publicBaseURL, internalBaseURL string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.trickleSrv = srv
	r.publicTrickleBaseURL = publicBaseURL
	r.internalTrickleBaseURL = internalBaseURL
}

func (r *LiveRunnerRegistry) Stop() {
	if r == nil {
		return
	}
	r.stopRegistryOnce.Do(func() {
		close(r.stopRegistry)
	})
	r.mu.Lock()
	runners := make([]*liveRunner, 0, len(r.runners))
	for _, runner := range r.runners {
		runners = append(runners, runner)
	}
	r.mu.Unlock()
	for _, runner := range runners {
		runner.mu.Lock()
		runner.closeChannelsLocked()
		runner.stopPriceConverterLocked()
		runner.mu.Unlock()
	}
}

func (r *LiveRunnerRegistry) Heartbeat(req LiveRunnerHeartbeatRequest, auth string) (*LiveRunnerHeartbeatResponse, error) {
	req.RunnerID = strings.TrimSpace(req.RunnerID)
	if req.RunnerID == "" {
		req.RunnerID = "runner_" + randomStr(liveRunnerIDRandomBytes)
	}

	req, err := r.normalizeHeartbeat(req.RunnerID, req)
	if err != nil {
		return nil, &RunnerError{StatusCode: http.StatusBadRequest, Message: err.Error()}
	}

	// For now, only return the heartbeat secret on first registration.
	// Return another one later on if the secret needs to be rotated.
	var responseHeartbeatSecret string
	var runner *liveRunner

	r.mu.Lock()
	runner = r.runners[req.RunnerID]
	heartbeatInterval := r.heartbeatInterval
	heartbeatTTL := r.heartbeatTTL
	orchestrator := r.host.LiveRunnerURI().String()
	if runner == nil {
		trickleSrv := r.trickleSrv
		trickleBaseURL := r.internalTrickleBaseURL
		// registering a new runner
		registrationSecret := r.host.RegistrationSecret()
		if registrationSecret == "" {
			r.mu.Unlock()
			return nil, &RunnerError{StatusCode: http.StatusNotFound, Message: "dynamic live runner registration is disabled"}
		}
		if !validSecret(auth, registrationSecret) {
			r.mu.Unlock()
			return nil, &RunnerError{StatusCode: http.StatusUnauthorized, Message: "invalid registration authorization"}
		}
		seed := newRunnerSeed()
		heartbeatSecret := deriveSecret(seed, "heartbeat")
		runner = &liveRunner{
			seed:            seed,
			heartbeatSecret: heartbeatSecret,
			offchain:        r.offchain,
			route:           req.RunnerID, // No label-based routing for now
			sessions:        make(map[string]*liveRunnerSession),
		}
		if err := runner.setRunnerTrickleChannels(trickleSrv, trickleBaseURL, req.RunnerID); err != nil {
			runner.closeChannelsLocked()
			r.mu.Unlock()
			return nil, err
		}
		r.runners[req.RunnerID] = runner
		responseHeartbeatSecret = heartbeatSecret
		runner.mu.Lock()
		r.mu.Unlock()

		defer runner.mu.Unlock()
		runner.LiveRunnerHeartbeatRequest = req
		runner.LastHeartbeat = time.Now()
		runner.route = runner.RunnerID // not doing label based routing here for now
		runner.updatePriceConverterLocked()

		// SessionIDs in the response come from the orchestrator's authoritative
		// active sessions and may differ from session_ids reported in the request.
		return &LiveRunnerHeartbeatResponse{
			RunnerID:          runner.RunnerID,
			Orchestrator:      orchestrator,
			HeartbeatInterval: heartbeatInterval.String(),
			HeartbeatTTL:      heartbeatTTL.String(),
			HeartbeatSecret:   responseHeartbeatSecret,
			O2R:               &runner.o2r.channel,
			SessionIDs:        runner.sessionIDsLocked(),
		}, nil
	} else {
		r.mu.Unlock()
	}

	runner.mu.Lock()
	defer runner.mu.Unlock()
	if runner.removed || liveRunnerExpiredLocked(runner, time.Now(), heartbeatTTL) {
		return nil, &RunnerError{StatusCode: http.StatusNotFound, Message: "runner not found"}
	}
	if !validSecret(auth, runner.heartbeatSecret) {
		return nil, &RunnerError{StatusCode: http.StatusUnauthorized, Message: "invalid heartbeat authorization"}
	}

	if runner.static {
		return nil, &RunnerError{StatusCode: http.StatusBadRequest, Message: "static runner cannot heartbeat"}
	}
	runner.LiveRunnerHeartbeatRequest = req
	runner.LastHeartbeat = time.Now()
	runner.route = runner.RunnerID // not doing label based routing here for now
	runner.updatePriceConverterLocked()

	// SessionIDs in the response come from the orchestrator's authoritative
	// active sessions and may differ from session_ids reported in the request.
	return &LiveRunnerHeartbeatResponse{
		RunnerID:          runner.RunnerID,
		Orchestrator:      orchestrator,
		HeartbeatInterval: heartbeatInterval.String(),
		HeartbeatTTL:      heartbeatTTL.String(),
		HeartbeatSecret:   responseHeartbeatSecret,
		SessionIDs:        runner.sessionIDsLocked(),
	}, nil
}

func ParseStaticLiveRunnerConfig(data []byte) (StaticLiveRunnerConfig, error) {
	var cfg StaticLiveRunnerConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return StaticLiveRunnerConfig{}, err
	}
	return normalizeStaticLiveRunnerConfig(cfg)
}

func (r *LiveRunnerRegistry) RegisterStaticRunnersJSON(data []byte) (*StaticLiveRunnerRegistrationResponse, error) {
	cfg, err := ParseStaticLiveRunnerConfig(data)
	if err != nil {
		return nil, &RunnerError{StatusCode: http.StatusBadRequest, Message: err.Error()}
	}
	return r.RegisterStaticRunners(cfg)
}

func normalizeStaticLiveRunnerConfig(cfg StaticLiveRunnerConfig) (StaticLiveRunnerConfig, error) {
	for i := range cfg.Runners {
		cfg.Runners[i].Route = strings.TrimSpace(cfg.Runners[i].Route)
		if cfg.Runners[i].Route == "" {
			cfg.Runners[i].Route = LiveRunnerRoutingRunnerID
		}
		if cfg.Runners[i].Route != LiveRunnerRoutingRunnerID && cfg.Runners[i].Route != LiveRunnerRoutingLabel {
			return StaticLiveRunnerConfig{}, fmt.Errorf("runners[%d]: routing must be %q or %q", i, LiveRunnerRoutingRunnerID, LiveRunnerRoutingLabel)
		}
		if cfg.Runners[i].HealthyStatusCode == 0 {
			cfg.Runners[i].HealthyStatusCode = defaultLiveRunnerHealthStatusCode
		}
		healthURL, err := staticRunnerHealthURL(cfg.Runners[i].RunnerURL, cfg.Runners[i].HealthURL)
		if err != nil {
			return StaticLiveRunnerConfig{}, fmt.Errorf("runners[%d]: %v", i, err)
		}
		cfg.Runners[i].HealthURL = healthURL
	}
	return cfg, nil
}

func staticRunnerHealthURL(runnerURL, healthURL string) (string, error) {
	u, err := url.Parse(healthURL)
	if err != nil {
		return "", fmt.Errorf("invalid health_url")
	}
	if u.IsAbs() {
		return healthURL, nil
	}
	if u.Path == "" || u.Path[0] != '/' || u.Host != "" {
		return healthURL, nil
	}
	base, err := url.ParseRequestURI(runnerURL)
	if err != nil || base.Host == "" || (base.Scheme != "http" && base.Scheme != "https") {
		return "", fmt.Errorf("invalid runner_url")
	}
	return strings.TrimRight(runnerURL, "/") + healthURL, nil
}

func staticRunnerRoute(route, runnerID, label string) string {
	if route == LiveRunnerRoutingLabel {
		return label
	}
	return runnerID
}

func (g *LiveRunnerGPU) UnmarshalJSON(data []byte) error {
	var deviceIndex int
	if err := json.Unmarshal(data, &deviceIndex); err == nil {
		*g = liveRunnerGPUForIndex(deviceIndex)
		return nil
	}
	type gpuAlias LiveRunnerGPU
	var parsed gpuAlias
	if err := json.Unmarshal(data, &parsed); err != nil {
		return err
	}
	*g = LiveRunnerGPU(parsed)
	return nil
}

func liveRunnerGPUForIndex(deviceIndex int) LiveRunnerGPU {
	gpuInfo := LiveRunnerGPU{ID: fmt.Sprintf("%d", deviceIndex)}
	if deviceIndex < 0 {
		return gpuInfo
	}
	info, err := ghw.GPU()
	if err != nil || info == nil || deviceIndex >= len(info.GraphicsCards) {
		return gpuInfo
	}
	card := info.GraphicsCards[deviceIndex]
	if card == nil {
		return gpuInfo
	}
	if card.DeviceInfo != nil {
		gpuInfo.Name = strings.TrimSpace(card.DeviceInfo.Product.Name)
	}
	return gpuInfo
}

func (r *LiveRunnerRegistry) RegisterStaticRunners(cfg StaticLiveRunnerConfig) (*StaticLiveRunnerRegistrationResponse, error) {
	cfg, err := normalizeStaticLiveRunnerConfig(cfg)
	if err != nil {
		return nil, &RunnerError{StatusCode: http.StatusBadRequest, Message: err.Error()}
	}
	requests := make([]LiveRunnerHeartbeatRequest, 0, len(cfg.Runners))
	health := make([]bool, 0, len(cfg.Runners))
	seenLabels := make(map[string]struct{}, len(cfg.Runners))
	for i, entry := range cfg.Runners {
		req, err := r.buildStaticRunner(entry)
		if err != nil {
			return nil, &RunnerError{StatusCode: http.StatusBadRequest, Message: fmt.Sprintf("runners[%d]: %v", i, err)}
		}
		if _, duplicate := seenLabels[entry.Label]; duplicate {
			return nil, &RunnerError{StatusCode: http.StatusBadRequest, Message: fmt.Sprintf("runners[%d]: duplicate label %q", i, entry.Label)}
		}
		if entry.Route == LiveRunnerRoutingLabel && strings.Contains(entry.Label, "/") {
			return nil, &RunnerError{StatusCode: http.StatusBadRequest, Message: fmt.Sprintf("runners[%d]: label cannot contain / when routing is %q", i, LiveRunnerRoutingLabel)}
		}
		seenLabels[entry.Label] = struct{}{}
		requests = append(requests, req)
		health = append(health, r.staticRunnerHealthy(entry.HealthURL, entry.HealthyStatusCode))
	}

	r.staticRegistrationMu.Lock()
	defer r.staticRegistrationMu.Unlock()

	r.mu.Lock()
	snapshot := make([]*liveRunner, 0, len(r.runners))
	for _, runner := range r.runners {
		snapshot = append(snapshot, runner)
	}
	r.mu.Unlock()

	existingStatic := make(map[string]*liveRunner)
	for _, runner := range snapshot {
		runner.mu.Lock()
		if !runner.removed && runner.static && runner.Label != "" {
			existingStatic[runner.Label] = runner
		}
		runner.mu.Unlock()
	}

	staticRunnersByLabel := make(map[string]*liveRunner, len(cfg.Runners))
	for _, entry := range cfg.Runners {
		staticRunner := existingStatic[entry.Label]
		if staticRunner == nil {
			staticRunner = &liveRunner{
				seed:     newRunnerSeed(),
				static:   true,
				offchain: r.offchain,
				sessions: make(map[string]*liveRunnerSession),
			}
			staticRunner.RunnerID = "runner_" + randomStr(liveRunnerIDRandomBytes)
		}
		staticRunnersByLabel[entry.Label] = staticRunner
	}

	registrations := make([]StaticLiveRunnerRegistration, 0, len(cfg.Runners))
	for i, entry := range cfg.Runners {
		staticRunner := staticRunnersByLabel[entry.Label]
		staticRunner.mu.Lock()
		r.mu.Lock()
		if r.runners[staticRunner.RunnerID] != nil && r.runners[staticRunner.RunnerID] != staticRunner {
			for r.runners[staticRunner.RunnerID] != nil {
				// collision lol
				staticRunner.RunnerID = "runner_" + randomStr(liveRunnerIDRandomBytes)
			}
		}
		req := requests[i]
		req.RunnerID = staticRunner.RunnerID
		if health[i] {
			req.Status = "ready"
		} else {
			req.Status = "unavailable"
			for sessionID := range staticRunner.sessions {
				staticRunner.releaseSessionLocked(sessionID)
			}
		}
		staticRunner.removed = false
		staticRunner.LiveRunnerHeartbeatRequest = req
		staticRunner.LastHeartbeat = time.Now()
		staticRunner.healthURL = entry.HealthURL
		staticRunner.healthStatus = entry.HealthyStatusCode
		staticRunner.route = staticRunnerRoute(entry.Route, staticRunner.RunnerID, req.Label)
		for route, runner := range r.runners {
			if runner == staticRunner && route != staticRunner.route {
				// route changed so remove old entry and re-add
				delete(r.runners, route)
			}
		}
		r.runners[staticRunner.route] = staticRunner
		staticRunner.updatePriceConverterLocked()
		r.mu.Unlock()
		registrations = append(registrations, StaticLiveRunnerRegistration{
			RunnerID:          staticRunner.RunnerID,
			Label:             req.Label,
			App:               req.App,
			RunnerURL:         req.RunnerURL,
			Mode:              req.Mode,
			HealthURL:         entry.HealthURL,
			HealthyStatusCode: entry.HealthyStatusCode,
			Healthy:           health[i],
		})
		staticRunner.mu.Unlock()
	}
	return &StaticLiveRunnerRegistrationResponse{Runners: registrations}, nil
}

func (r *LiveRunnerRegistry) buildStaticRunner(entry StaticLiveRunnerConfigEntry) (LiveRunnerHeartbeatRequest, error) {
	if entry.Label == "" || entry.Label != strings.TrimSpace(entry.Label) {
		return LiveRunnerHeartbeatRequest{}, fmt.Errorf("label is required and must be trimmed")
	}
	if entry.HealthURL == "" {
		return LiveRunnerHeartbeatRequest{}, fmt.Errorf("health_url is required")
	}
	u, err := url.ParseRequestURI(entry.HealthURL)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return LiveRunnerHeartbeatRequest{}, fmt.Errorf("invalid health_url")
	}
	if entry.HealthyStatusCode < 100 || entry.HealthyStatusCode > 599 {
		return LiveRunnerHeartbeatRequest{}, fmt.Errorf("healthy_status_code must be a valid HTTP status code")
	}
	req := LiveRunnerHeartbeatRequest{
		Label:     entry.Label,
		RunnerURL: entry.RunnerURL,
		Version:   entry.Version,
		Mode:      entry.Mode,
		GPU:       entry.GPU,
		App:       entry.App,
		Capacity:  entry.Capacity,
		PriceInfo: entry.PriceInfo,
	}
	return r.normalizeHeartbeat("static", req)
}

func (r *LiveRunnerRegistry) normalizeHeartbeat(runnerID string, req LiveRunnerHeartbeatRequest) (LiveRunnerHeartbeatRequest, error) {
	u, err := url.ParseRequestURI(req.RunnerURL)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return LiveRunnerHeartbeatRequest{}, fmt.Errorf("invalid runner_url")
	}

	if req.App == "" || req.App != strings.TrimSpace(req.App) {
		return LiveRunnerHeartbeatRequest{}, fmt.Errorf("app must be trimmed")
	}
	if req.Status != strings.ToLower(strings.TrimSpace(req.Status)) {
		return LiveRunnerHeartbeatRequest{}, fmt.Errorf("status must be lowercase and trimmed")
	}
	if req.Mode == "" {
		req.Mode = LiveRunnerModePersistent
	}
	if req.Mode == liveRunnerModeSingleShotAlias {
		req.Mode = LiveRunnerModeSingleShot
	}
	if req.Mode != LiveRunnerModePersistent && req.Mode != LiveRunnerModeSingleShot {
		return LiveRunnerHeartbeatRequest{}, fmt.Errorf("mode must be %q or %q", LiveRunnerModePersistent, LiveRunnerModeSingleShot)
	}
	if req.Capacity < 0 {
		return LiveRunnerHeartbeatRequest{}, fmt.Errorf("capacity must be >= 0")
	}
	if req.Capacity == 0 {
		req.Capacity = 1
	}
	req.RunnerID = runnerID
	req.GPU = cloneLiveRunnerGPU(req.GPU)

	if r.offchain {
		return req, nil
	}

	if req.PriceInfo.PixelsPerUnit <= 0 || req.PriceInfo.PricePerUnit <= 0 {
		return LiveRunnerHeartbeatRequest{}, fmt.Errorf("runner must include positive price_info")
	}
	unit := strings.ToUpper(strings.TrimSpace(req.PriceInfo.Unit))
	if unit != "USD" && unit != "WEI" {
		return LiveRunnerHeartbeatRequest{}, fmt.Errorf("price_info.unit must be USD or WEI")
	}
	req.PriceInfo.Unit = unit

	return req, nil
}

func (r *LiveRunnerRegistry) Unregister(runnerID, auth string) error {
	r.mu.Lock()
	runner := r.runners[runnerID]
	heartbeatTTL := r.heartbeatTTL
	if runner == nil {
		r.mu.Unlock()
		return &RunnerError{StatusCode: http.StatusNotFound, Message: "runner not found"}
	}
	r.mu.Unlock()

	runner.mu.Lock()
	defer runner.mu.Unlock()
	if runner.removed || liveRunnerExpiredLocked(runner, time.Now(), heartbeatTTL) {
		return &RunnerError{StatusCode: http.StatusNotFound, Message: "runner not found"}
	}
	if runner.static {
		return &RunnerError{StatusCode: http.StatusBadRequest, Message: "static runner cannot unregister with heartbeat credentials"}
	}
	if !validSecret(auth, runner.heartbeatSecret) {
		return &RunnerError{StatusCode: http.StatusUnauthorized, Message: "invalid authorization"}
	}
	r.mu.Lock()
	if r.runners[runnerID] == runner && !runner.removed {
		r.removeRunnerWithRunnerLocked(runnerID, runner)
	}
	r.mu.Unlock()
	return nil
}

func (r *LiveRunnerRegistry) ReserveSession(runnerID string, optSessionID ...string) (string, string, error) {
	runner, unlock, err := r.lockLiveRunner(runnerID)
	if err != nil {
		return "", "", err
	}
	defer unlock()
	if !isReadyStatus(runner.Status) {
		return "", "", &RunnerError{StatusCode: http.StatusNotFound, Message: "runner not found"}
	}
	if len(runner.sessions) >= runner.Capacity {
		return "", "", &RunnerError{StatusCode: http.StatusServiceUnavailable, Message: "no capacity available for runner"}
	}
	id := ""
	if len(optSessionID) > 0 {
		// use supplied id if given - usually ticket param's auth token id / manifest id
		id = optSessionID[0]
	}
	if id != "" {
		if _, exists := runner.sessions[id]; exists {
			return "", "", &RunnerError{StatusCode: http.StatusConflict, Message: "session already exists"}
		}
	} else {
		id = "session_" + randomStr(liveRunnerIDRandomBytes)
		for {
			if _, exists := runner.sessions[id]; !exists {
				break
			}
			id = "session_" + randomStr(liveRunnerIDRandomBytes)
		}
	}
	runner.sessions[id] = &liveRunnerSession{
		createdAt: time.Now(),
		channels:  make(map[string]*liveRunnerTrickleChannel),
	}
	runner.sendSessionEvent("reserved", id)
	return id, runner.RunnerURL, nil
}

func (r *LiveRunnerRegistry) PaymentInfo(runnerID string) (*LiveRunnerPriceInfo, error) {
	runner, unlock, err := r.lockLiveRunner(runnerID)
	if err != nil {
		return nil, err
	}
	defer unlock()
	if !isReadyStatus(runner.Status) {
		return nil, &RunnerError{StatusCode: http.StatusNotFound, Message: "runner not found"}
	}
	if runner.offchain {
		return nil, nil
	}
	priceInfo := runner.PriceInfo
	if runner.converter != nil {
		converted, err := convertedPriceInfo(runner.converter)
		if err != nil {
			return nil, err
		}
		priceInfo = converted
	}
	if priceInfo == (LiveRunnerPriceInfo{}) || priceInfo.PricePerUnit <= 0 || priceInfo.PixelsPerUnit <= 0 {
		return nil, nil
	}
	return &priceInfo, nil
}

func (r *LiveRunnerRegistry) ReleaseSession(runnerID, sessionID string) error {
	runner, unlock, err := r.lockLiveRunner(runnerID)
	if err != nil {
		return err
	}
	defer unlock()
	runner.releaseSessionLocked(sessionID)
	return nil
}

func (r *LiveRunnerRegistry) RunnerMode(runnerID string) (string, error) {
	runner, unlock, err := r.lockLiveRunner(runnerID)
	if err != nil {
		return "", err
	}
	defer unlock()
	if !isReadyStatus(runner.Status) {
		return "", &RunnerError{StatusCode: http.StatusNotFound, Message: "runner not found"}
	}
	if runner.Mode == "" {
		return LiveRunnerModePersistent, nil
	}
	return runner.Mode, nil
}

func (r *LiveRunnerRegistry) RunnerEndpointForSession(runnerID, sessionID string) (string, error) {
	runner, unlock, err := r.lockLiveRunner(runnerID)
	if err != nil {
		return "", err
	}
	defer unlock()
	if !isReadyStatus(runner.Status) {
		return "", &RunnerError{StatusCode: http.StatusNotFound, Message: "runner not found"}
	}
	if _, exists := runner.sessions[sessionID]; !exists {
		return "", &RunnerError{StatusCode: http.StatusNotFound, Message: "runner session not found"}
	}
	return runner.RunnerURL, nil
}

func (r *LiveRunnerRegistry) SessionTokenForSession(runnerID, sessionID string) (string, error) {
	runner, unlock, err := r.lockLiveRunner(runnerID)
	if err != nil {
		return "", err
	}
	defer unlock()
	if !isReadyStatus(runner.Status) {
		return "", &RunnerError{StatusCode: http.StatusNotFound, Message: "runner not found"}
	}
	if _, exists := runner.sessions[sessionID]; !exists {
		return "", &RunnerError{StatusCode: http.StatusNotFound, Message: "runner session not found"}
	}
	return runner.sessionToken(sessionID), nil
}

func (r *LiveRunnerRegistry) ValidSessionToken(runnerID, sessionID, token string) error {
	runner, unlock, err := r.lockLiveRunner(runnerID)
	if err != nil {
		return err
	}
	defer unlock()
	if !isReadyStatus(runner.Status) {
		return &RunnerError{StatusCode: http.StatusNotFound, Message: "runner not found"}
	}
	if _, exists := runner.sessions[sessionID]; !exists {
		return &RunnerError{StatusCode: http.StatusNotFound, Message: "runner session not found"}
	}
	if !validSecret(token, runner.sessionToken(sessionID)) {
		return &RunnerError{StatusCode: http.StatusUnauthorized, Message: "invalid authorization"}
	}
	return nil
}

func (r *LiveRunnerRegistry) CreateTrickleChannel(runnerID, sessionID, name, mimeType string) (LiveRunnerTrickleChannel, error) {
	name, channelName, err := normalizeTrickleChannelName(sessionID, name)
	if err != nil {
		return LiveRunnerTrickleChannel{}, err
	}
	if strings.TrimSpace(mimeType) == "" {
		mimeType = "application/octet-stream"
	}

	r.mu.Lock()
	trickleSrv := r.trickleSrv
	publicTrickleBaseURL := r.publicTrickleBaseURL
	internalTrickleBaseURL := r.internalTrickleBaseURL
	r.mu.Unlock()

	if trickleSrv == nil {
		return LiveRunnerTrickleChannel{}, fmt.Errorf("trickle server is required")
	}
	channelURL, err := url.JoinPath(publicTrickleBaseURL, channelName)
	if err != nil {
		return LiveRunnerTrickleChannel{}, err
	}
	internalChannelURL, err := url.JoinPath(internalTrickleBaseURL, channelName)
	if err != nil {
		return LiveRunnerTrickleChannel{}, err
	}

	runner, unlock, err := r.lockLiveRunner(runnerID)
	if err != nil {
		return LiveRunnerTrickleChannel{}, err
	}
	defer unlock()
	session, err := runner.liveRunnerSessionLocked(sessionID)
	if err != nil {
		return LiveRunnerTrickleChannel{}, err
	}
	if channel := session.channels[name]; channel != nil {
		return channel.channel, nil
	}

	channel := newLiveRunnerTrickleChannel(trickleSrv, name, channelName, channelURL, internalChannelURL, mimeType)
	session.channels[name] = channel
	return channel.channel, nil
}

func (runner *liveRunner) setRunnerTrickleChannels(trickleSrv *trickle.Server, trickleBaseURL, runnerID string) error {
	o2r, err := runner.createBootstrapTrickleChannel(trickleSrv, trickleBaseURL, runnerID, LiveRunnerTrickleOrchestratorToRunner)
	if err != nil {
		return err
	}
	runner.o2r = o2r
	go writeO2RLoop(runner.o2r)
	return nil
}

func (runner *liveRunner) createBootstrapTrickleChannel(trickleSrv *trickle.Server, trickleBaseURL, runnerID, name string) (*liveRunnerTrickleChannel, error) {
	channelName := runnerID + "-" + deriveSecret(runner.seed, "trickle:"+name)[:16] + "-" + name
	channelURL, err := url.JoinPath(trickleBaseURL, channelName)
	if err != nil {
		return nil, err
	}
	channel := newLiveRunnerTrickleChannel(trickleSrv, name, channelName, channelURL, "", "application/octet-stream")
	return channel, nil
}

func newLiveRunnerTrickleChannel(trickleSrv *trickle.Server, name, channelName, channelURL, internalChannelURL, mimeType string) *liveRunnerTrickleChannel {
	publisher := trickle.NewLocalPublisher(trickleSrv, channelName, mimeType)
	publisher.CreateChannel()
	return &liveRunnerTrickleChannel{
		channel: LiveRunnerTrickleChannel{
			Name:        name,
			ChannelName: channelName,
			URL:         channelURL,
			InternalURL: internalChannelURL,
			MimeType:    mimeType,
		},
		publisher: publisher,
		done:      make(chan struct{}),
		messages:  make(chan string, 32),
	}
}

func (r *LiveRunnerRegistry) DeleteTrickleChannel(runnerID, sessionID, name string) error {
	name, _, err := normalizeTrickleChannelName(sessionID, name)
	if err != nil {
		return err
	}

	runner, unlock, err := r.lockLiveRunner(runnerID)
	if err != nil {
		return err
	}
	defer unlock()
	session, err := runner.liveRunnerSessionLocked(sessionID)
	if err != nil {
		return err
	}
	channel := session.channels[name]
	if channel == nil {
		return &RunnerError{StatusCode: http.StatusNotFound, Message: "trickle channel not found"}
	}
	if err := channel.close(); err != nil {
		return err
	}
	delete(session.channels, name)
	return nil
}

func (r *LiveRunnerRegistry) Runners() []LiveRunnerDiscoveryRunner {
	r.mu.Lock()
	heartbeatTTL := r.heartbeatTTL
	snapshot := make([]*liveRunner, 0, len(r.runners))
	for _, runner := range r.runners {
		snapshot = append(snapshot, runner)
	}
	r.mu.Unlock()

	runners := make([]LiveRunnerDiscoveryRunner, 0, len(snapshot))
	for _, runner := range snapshot {
		runner.mu.Lock()
		if runner.removed || liveRunnerExpiredLocked(runner, time.Now(), heartbeatTTL) || !isReadyStatus(runner.Status) {
			runner.mu.Unlock()
			continue
		}
		discoveryRunner := runner.discoveryRunner()
		discoveryRunner.URL = r.discoveryRunnerURL(runner)
		runner.mu.Unlock()
		runners = append(runners, discoveryRunner)
	}
	return runners
}

func (r *LiveRunnerRegistry) discoveryRunnerURL(runner *liveRunner) string {
	// Under no circumstances leak the runner URL through discovery.
	// Discovery must only expose orchestrator proxy URLs; the runner URL is an internal endpoint.
	// discoveryRunnerURL requires runner.mu.
	if runner.Mode == LiveRunnerModeSingleShot {
		return r.host.ServiceURI().JoinPath("apps", runner.route, "app").String()
	}
	return r.host.ServiceURI().JoinPath("apps", runner.route, "session").String()
}

func (r *LiveRunnerRegistry) lockLiveRunner(runnerID string) (*liveRunner, func(), error) {
	r.mu.Lock()
	runner := r.runners[runnerID]
	if runner == nil {
		r.mu.Unlock()
		return nil, nil, &RunnerError{StatusCode: http.StatusNotFound, Message: "runner not found"}
	}
	heartbeatTTL := r.heartbeatTTL
	r.mu.Unlock()

	// Do not wait on runner.mu while holding r.mu. A copied runner pointer stays
	// valid after map deletion; removal is observed through runner.removed once
	// runner.mu is acquired.
	runner.mu.Lock()
	if runner.removed || liveRunnerExpiredLocked(runner, time.Now(), heartbeatTTL) {
		runner.mu.Unlock()
		return nil, nil, &RunnerError{StatusCode: http.StatusNotFound, Message: "runner not found"}
	}
	return runner, runner.mu.Unlock, nil
}

func liveRunnerExpiredLocked(runner *liveRunner, now time.Time, heartbeatTTL time.Duration) bool {
	// liveRunnerExpiredLocked requires runner.mu.
	return !runner.static && now.Sub(runner.LastHeartbeat) > heartbeatTTL
}

func (r *LiveRunnerRegistry) expiryLoop() {
	ticker := time.NewTicker(r.heartbeatInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			r.removeExpiredRunners(time.Now())
		case <-r.stopRegistry:
			return
		}
	}
}

func (r *LiveRunnerRegistry) removeExpiredRunners(now time.Time) {
	type candidate struct {
		runnerID string
		runner   *liveRunner
	}
	r.mu.Lock()
	heartbeatTTL := r.heartbeatTTL
	candidates := make([]candidate, 0, len(r.runners))
	for runnerID, runner := range r.runners {
		candidates = append(candidates, candidate{runnerID: runnerID, runner: runner})
	}
	r.mu.Unlock()

	for _, c := range candidates {
		c.runner.mu.Lock()
		expired := !c.runner.removed && liveRunnerExpiredLocked(c.runner, now, heartbeatTTL)
		if expired {
			r.mu.Lock()
			if r.runners[c.runnerID] == c.runner && !c.runner.removed && liveRunnerExpiredLocked(c.runner, now, heartbeatTTL) {
				r.removeRunnerWithRunnerLocked(c.runnerID, c.runner)
			}
			r.mu.Unlock()
		}
		c.runner.mu.Unlock()
	}
}

func (r *LiveRunnerRegistry) healthLoop() {
	ticker := time.NewTicker(r.healthInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			r.checkStaticRunnerHealth()
		case <-r.stopRegistry:
			return
		}
	}
}

func (r *LiveRunnerRegistry) checkStaticRunnerHealth() {
	type check struct {
		runner       *liveRunner
		healthURL    string
		healthStatus int
	}
	r.mu.Lock()
	checks := []check{}
	for _, runner := range r.runners {
		checks = append(checks, check{runner: runner})
	}
	r.mu.Unlock()

	for i, c := range checks {
		runner := c.runner
		runner.mu.Lock()
		if !runner.removed && runner.static {
			checks[i].healthURL = runner.healthURL
			checks[i].healthStatus = runner.healthStatus
		} else {
			checks[i].runner = nil
		}
		runner.mu.Unlock()
	}

	for _, c := range checks {
		if c.runner == nil {
			continue
		}
		healthy := r.staticRunnerHealthy(c.healthURL, c.healthStatus)
		c.runner.mu.Lock()
		if !c.runner.removed && c.runner.static {
			if healthy {
				c.runner.Status = "ready"
			} else {
				c.runner.Status = "unavailable"
				for sessionID := range c.runner.sessions {
					c.runner.releaseSessionLocked(sessionID)
				}
			}
		}
		c.runner.mu.Unlock()
	}
}

func (r *LiveRunnerRegistry) staticRunnerHealthy(healthURL string, healthyStatus int) bool {
	resp, err := r.healthClient.Get(healthURL)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == healthyStatus
}

func (r *LiveRunnerRegistry) removeRunnerWithRunnerLocked(runnerID string, runner *liveRunner) {
	// removeRunnerWithRunnerLocked requires both r.mu and runner.mu. Callers
	// should already hold runner.mu before taking r.mu so the registry lock is
	// not held while waiting on a busy runner.
	// Set removed before deleting the map entry so any goroutine that already
	// copied the runner pointer can reject it after acquiring runner.mu.
	runner.removed = true
	runner.closeChannelsLocked()
	for sessionID := range runner.sessions {
		runner.releaseSessionLocked(sessionID)
	}
	runner.stopPriceConverterLocked()
	if r.runners[runnerID] == runner {
		delete(r.runners, runnerID)
	}
}

func (runner *liveRunner) releaseSessionLocked(sessionID string) {
	// releaseSessionLocked requires runner.mu.
	if session := runner.sessions[sessionID]; session != nil {
		runner.sendSessionEvent("released", sessionID)
		session.closeChannelsLocked()
	}
	delete(runner.sessions, sessionID)
}

func (runner *liveRunner) sendSessionEvent(event, sessionID string) {
	o2r := runner.o2r
	if o2r == nil {
		return
	}
	if err := o2r.queueJSON(struct {
		Event     string    `json:"event"`
		Session   string    `json:"session"`
		Timestamp time.Time `json:"timestamp"`
	}{
		Event:     event,
		Session:   sessionID,
		Timestamp: time.Now().UTC(),
	}); err != nil {
		slog.Warn("error queueing live runner session event", "runner_id", runner.RunnerID, "session_id", sessionID, "event", event, "err", err)
	}
}

func (runner *liveRunner) closeChannelsLocked() {
	for _, channel := range []*liveRunnerTrickleChannel{runner.o2r} {
		if channel == nil {
			continue
		}
		if err := channel.close(); err != nil {
			slog.Error("error closing live runner bootstrap trickle channel", "channel", channel.channel.ChannelName, "err", err)
		}
	}
}

func (runner *liveRunner) liveRunnerSessionLocked(sessionID string) (*liveRunnerSession, error) {
	// liveRunnerSessionLocked requires runner.mu.
	if runner.removed || !isReadyStatus(runner.Status) {
		return nil, &RunnerError{StatusCode: http.StatusNotFound, Message: "runner not found"}
	}
	session := runner.sessions[sessionID]
	if session == nil {
		return nil, &RunnerError{StatusCode: http.StatusNotFound, Message: "runner session not found"}
	}
	return session, nil
}

func normalizeTrickleChannelName(sessionID, name string) (string, string, error) {
	name = strings.ReplaceAll(strings.TrimSpace(name), "/", "-")
	if name == "" {
		return "", "", &RunnerError{StatusCode: http.StatusBadRequest, Message: "trickle channel name is required"}
	}
	if !liveRunnerChannelNamePattern.MatchString(name) {
		return "", "", &RunnerError{StatusCode: http.StatusBadRequest, Message: "trickle channel name may only contain alphanumerics, underscores, or hyphens"}
	}
	return name, sessionID + "-" + name, nil
}

func (session *liveRunnerSession) closeChannelsLocked() {
	for name, channel := range session.channels {
		if err := channel.close(); err != nil {
			slog.Error("error closing live runner trickle channel", "channel", channel.channel.ChannelName, "err", err)
		}
		delete(session.channels, name)
	}
}

func writeO2RLoop(channel *liveRunnerTrickleChannel) {
	ticker := time.NewTicker(liveRunnerO2RKeepaliveInterval)
	defer ticker.Stop()

	for {
		select {
		case <-channel.done:
			return
		case msg := <-channel.messages:
			if !channel.writeJSON(msg) {
				return
			}
		case <-ticker.C:
			if !channel.writeJSON(liveRunnerO2RKeepaliveMessage) {
				return
			}
		}
	}
}

func (channel *liveRunnerTrickleChannel) queueJSON(v any) error {
	if channel == nil {
		return fmt.Errorf("live runner trickle channel is closed")
	}
	msg, err := json.Marshal(v)
	if err != nil {
		return err
	}
	select {
	case <-channel.done:
		return fmt.Errorf("live runner trickle channel %q is closed", channel.channel.ChannelName)
	case channel.messages <- string(msg):
		return nil
	default:
		return fmt.Errorf("live runner trickle channel %q write queue is full", channel.channel.ChannelName)
	}
}

func (channel *liveRunnerTrickleChannel) writeJSON(msg string) bool {
	if err := channel.publisher.Write(strings.NewReader(msg)); err != nil {
		if errors.Is(err, trickle.StreamNotFoundErr) {
			channel.closeOnce.Do(func() {
				close(channel.done)
			})
			return false
		}
		slog.Warn("error writing live runner trickle message", "channel", channel.channel.ChannelName, "err", err)
	}
	return true
}

func (channel *liveRunnerTrickleChannel) close() error {
	if channel == nil {
		return nil
	}
	channel.closeOnce.Do(func() {
		close(channel.done)
	})
	return channel.publisher.Close()
}

func newRunnerSeed() []byte {
	return secureRandomBytes(liveRunnerSeedBytes)
}

func randomStr(n int) string {
	return strings.ToLower(liveRunnerEncoding.EncodeToString(secureRandomBytes(n)))
}

func secureRandomBytes(n int) []byte {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("generate secure random bytes: %v", err))
	}
	return b
}

func (runner *liveRunner) sessionToken(sessionID string) string {
	return deriveSecret(runner.seed, "session:"+sessionID)
}

func (runner *liveRunner) sessionIDsLocked() []string {
	// sessionIDsLocked requires runner.mu.
	type sessionEntry struct {
		id        string
		createdAt time.Time
	}
	entries := make([]sessionEntry, 0, len(runner.sessions))
	for id, session := range runner.sessions {
		entries = append(entries, sessionEntry{id: id, createdAt: session.createdAt})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].createdAt.Equal(entries[j].createdAt) {
			return entries[i].id < entries[j].id
		}
		return entries[i].createdAt.Before(entries[j].createdAt)
	})
	sessionIDs := make([]string, 0, len(entries))
	for _, entry := range entries {
		sessionIDs = append(sessionIDs, entry.id)
	}
	return sessionIDs
}

func deriveSecret(seed []byte, label string) string {
	return strings.ToLower(liveRunnerEncoding.EncodeToString(deriveSecretBytes(seed, label)))
}

func deriveSecretBytes(seed []byte, label string) []byte {
	mac := hmac.New(sha256.New, seed)
	mac.Write([]byte(label))
	return mac.Sum(nil)
}

func validSecret(got, want string) bool {
	return hmac.Equal([]byte(got), []byte(want))
}

func (runner *liveRunner) discoveryRunner() LiveRunnerDiscoveryRunner {
	// discoveryRunner requires runner.mu.
	priceInfo := runner.PriceInfo
	if runner.converter != nil {
		converted, err := convertedPriceInfo(runner.converter)
		if err != nil {
			slog.Error("error reading converted live runner price", "app", runner.App, "endpoint", runner.RunnerURL, "err", err)
		} else {
			priceInfo = converted
		}
	}
	var discoveryPriceInfo *LiveRunnerPriceInfo
	if !runner.offchain && priceInfo != (LiveRunnerPriceInfo{}) {
		discoveryPriceInfo = &priceInfo
	}
	return LiveRunnerDiscoveryRunner{
		GPU:               cloneLiveRunnerGPU(runner.GPU),
		App:               runner.App,
		Version:           runner.Version,
		Mode:              runner.Mode,
		Capacity:          runner.Capacity,
		CapacityUsed:      len(runner.sessions),
		CapacityAvailable: runner.Capacity - len(runner.sessions),
		PriceInfo:         discoveryPriceInfo,
	}
}

func (runner *liveRunner) updatePriceConverterLocked() {
	// updatePriceConverterLocked requires runner.mu.
	if runner.offchain {
		return
	}
	// Heartbeats call this each time, but converter rebuild work is only done when
	// price_info changes (or when a converter is missing after a prior failure).
	if runner.converter != nil && runner.priceSource == runner.PriceInfo {
		return
	}
	runner.stopPriceConverterLocked()
	converter, err := newConverterForRunner(runner.PriceInfo)
	if err != nil {
		slog.Error("error creating live runner price converter", "runner_id", runner.RunnerID, "app", runner.App, "endpoint", runner.RunnerURL, "err", err)
		return
	}
	runner.priceSource = runner.PriceInfo
	runner.converter = converter
}

func (runner *liveRunner) stopPriceConverterLocked() {
	// stopPriceConverterLocked requires runner.mu.
	if runner.converter == nil {
		return
	}
	runner.converter.Stop()
	runner.converter = nil
}

func newConverterForRunner(priceInfo LiveRunnerPriceInfo) (*core.AutoConvertedPrice, error) {
	if strings.ToUpper(priceInfo.Unit) != "USD" {
		return nil, nil
	}

	usdPerPixel := usdPerPixelFromUSDPerHour(priceInfo)
	return core.NewAutoConvertedPrice("USD", usdPerPixel, nil)
}

func usdPerPixelFromUSDPerHour(priceInfo LiveRunnerPriceInfo) *big.Rat {
	pixelsPerHour := 1280 * 720 * 30 * 3600 // 720p @ 30fps
	usdPerHour := new(big.Rat).SetFrac64(priceInfo.PricePerUnit, priceInfo.PixelsPerUnit)
	return new(big.Rat).Quo(usdPerHour, new(big.Rat).SetInt64(int64(pixelsPerHour)))
}

func convertedPriceInfo(converter *core.AutoConvertedPrice) (LiveRunnerPriceInfo, error) {
	if converter == nil {
		return LiveRunnerPriceInfo{}, nil
	}
	price, err := common.PriceToInt64(converter.Value())
	if err != nil {
		return LiveRunnerPriceInfo{}, err
	}
	return LiveRunnerPriceInfo{
		PricePerUnit:  price.Num().Int64(),
		PixelsPerUnit: price.Denom().Int64(),
		Unit:          "WEI",
	}, nil
}

func isReadyStatus(status string) bool {
	return status == "" || status == "ready"
}

func cloneLiveRunnerGPU(gpu *LiveRunnerGPU) *LiveRunnerGPU {
	if gpu == nil {
		return nil
	}
	copy := *gpu
	return &copy
}
