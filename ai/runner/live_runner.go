package runner

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/big"
	"net/http"
	"net/url"
	"regexp"
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
	liveRunnerIDRandomBytes            = 5
	liveRunnerSeedBytes                = 32
)

const (
	LiveRunnerModePersistent = "persistent"
	LiveRunnerModeSingleShot = "single_shot"
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
	RunnerID  string              `json:"runner_id,omitempty"`
	Label     string              `json:"label,omitempty"`
	RunnerURL string              `json:"runner_url"`
	Version   string              `json:"version,omitempty"`
	Status    string              `json:"status,omitempty"`
	Mode      string              `json:"mode,omitempty"`
	GPU       *LiveRunnerGPU      `json:"gpu,omitempty"`
	App       string              `json:"app"`
	Capacity  int                 `json:"capacity"`
	PriceInfo LiveRunnerPriceInfo `json:"price_info"`
}

type StaticLiveRunnerConfig struct {
	Runners []StaticLiveRunnerConfigEntry `json:"runners"`
}

type StaticLiveRunnerConfigEntry struct {
	Label             string              `json:"label,omitempty"`
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
	RunnerID          string `json:"runner_id"`
	Orchestrator      string `json:"orchestrator,omitempty"`
	HeartbeatInterval string `json:"heartbeat_interval"`
	HeartbeatTTL      string `json:"heartbeat_ttl"`
	HeartbeatSecret   string `json:"heartbeat_secret,omitempty"`
}

type LiveRunnerDiscoveryRunner struct {
	Endpoint  string              `json:"endpoint"`
	GPU       *LiveRunnerGPU      `json:"gpu,omitempty"`
	App       string              `json:"app"`
	Version   string              `json:"version,omitempty"`
	Mode      string              `json:"mode,omitempty"`
	PriceInfo LiveRunnerPriceInfo `json:"price_info"`
}

type LiveRunnerTrickleChannel struct {
	Name        string `json:"name"`
	ChannelName string `json:"channel_name"`
	URL         string `json:"url"`
	MimeType    string `json:"mime_type"`
}

type liveRunner struct {
	LiveRunnerHeartbeatRequest
	LastHeartbeat   time.Time
	seed            []byte
	heartbeatSecret string
	static          bool
	healthURL       string
	healthStatus    int
	sessions        map[string]*liveRunnerSession
	priceSource     LiveRunnerPriceInfo
	converter       *core.AutoConvertedPrice
}

type liveRunnerSession struct {
	channels map[string]*liveRunnerTrickleChannel
}

type liveRunnerTrickleChannel struct {
	info      LiveRunnerTrickleChannel
	publisher *trickle.TrickleLocalPublisher
}

type LiveRunnerRegistry struct {
	mu                sync.Mutex
	runners           map[string]*liveRunner
	host              RunnerHost
	heartbeatInterval time.Duration
	heartbeatTTL      time.Duration
	trickleSrv        *trickle.Server
	trickleBaseURL    string
	healthClient      *http.Client
	healthInterval    time.Duration
	stopHealth        chan struct{}
}

type RunnerHost interface {
	ServiceURI() *url.URL
	RegistrationSecret() string
}

type LiveRunnerRegistryConfig struct {
	Host RunnerHost
}

var liveRunnerChannelNamePattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

func NewLiveRunnerRegistry(config LiveRunnerRegistryConfig) *LiveRunnerRegistry {
	r := &LiveRunnerRegistry{
		host:              config.Host,
		runners:           make(map[string]*liveRunner),
		heartbeatInterval: defaultLiveRunnerHeartbeatInterval,
		heartbeatTTL:      defaultLiveRunnerHeartbeatTTL,
		healthClient:      &http.Client{Timeout: defaultLiveRunnerHealthTimeout},
		healthInterval:    defaultLiveRunnerHealthInterval,
		stopHealth:        make(chan struct{}),
	}
	go r.healthLoop()
	return r
}

func (r *LiveRunnerRegistry) SetTrickleServer(srv *trickle.Server, baseURL string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.trickleSrv = srv
	r.trickleBaseURL = baseURL
}

func (r *LiveRunnerRegistry) Heartbeat(req LiveRunnerHeartbeatRequest, auth string) (*LiveRunnerHeartbeatResponse, error) {
	req.RunnerID = strings.TrimSpace(req.RunnerID)
	if req.RunnerID == "" {
		req.RunnerID = "runner_" + randomStr(liveRunnerIDRandomBytes)
	}

	req, err := normalizeLiveRunnerRequest(req.RunnerID, req)
	if err != nil {
		return nil, &RunnerError{StatusCode: http.StatusBadRequest, Message: err.Error()}
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	r.expireLocked(time.Now())

	runner := r.runners[req.RunnerID]
	// For now, only return the heartbeat secret on first registration.
	// Return another one later on if the secret needs to be rotated.
	var responseHeartbeatSecret string
	if runner == nil {
		registrationSecret := ""
		if r.host != nil {
			registrationSecret = r.host.RegistrationSecret()
		}
		if registrationSecret == "" {
			return nil, &RunnerError{StatusCode: http.StatusNotFound, Message: "dynamic live runner registration is disabled"}
		}
		if !validSecret(auth, registrationSecret) {
			return nil, &RunnerError{StatusCode: http.StatusUnauthorized, Message: "invalid authorization"}
		}
		seed := newRunnerSeed()
		heartbeatSecret := deriveSecret(seed, "heartbeat")
		runner = &liveRunner{
			seed:            seed,
			heartbeatSecret: heartbeatSecret,
			sessions:        make(map[string]*liveRunnerSession),
		}
		r.runners[req.RunnerID] = runner
		responseHeartbeatSecret = heartbeatSecret
	} else if runner.static {
		return nil, &RunnerError{StatusCode: http.StatusBadRequest, Message: "static runner cannot heartbeat"}
	} else if !validSecret(auth, runner.heartbeatSecret) {
		return nil, &RunnerError{StatusCode: http.StatusUnauthorized, Message: "invalid authorization"}
	}
	runner.LiveRunnerHeartbeatRequest = req
	runner.LastHeartbeat = time.Now()
	runner.updatePriceConverterLocked()

	return &LiveRunnerHeartbeatResponse{
		RunnerID:          runner.RunnerID,
		Orchestrator:      r.host.ServiceURI().String(),
		HeartbeatInterval: r.heartbeatInterval.String(),
		HeartbeatTTL:      r.heartbeatTTL.String(),
		HeartbeatSecret:   responseHeartbeatSecret,
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
		req, err := staticRunnerRequest(entry)
		if err != nil {
			return nil, &RunnerError{StatusCode: http.StatusBadRequest, Message: fmt.Sprintf("runners[%d]: %v", i, err)}
		}
		if _, duplicate := seenLabels[entry.Label]; duplicate {
			return nil, &RunnerError{StatusCode: http.StatusBadRequest, Message: fmt.Sprintf("runners[%d]: duplicate label %q", i, entry.Label)}
		}
		seenLabels[entry.Label] = struct{}{}
		requests = append(requests, req)
		health = append(health, r.staticRunnerHealthy(entry.HealthURL, entry.HealthyStatusCode))
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	existingStatic := make(map[string]*liveRunner)
	for _, runner := range r.runners {
		if runner.static && runner.Label != "" {
			existingStatic[runner.Label] = runner
		}
	}

	registrations := make([]StaticLiveRunnerRegistration, 0, len(cfg.Runners))
	for i, entry := range cfg.Runners {
		staticRunner := existingStatic[entry.Label]
		if staticRunner == nil {
			runnerID := "runner_" + randomStr(liveRunnerIDRandomBytes)
			for r.runners[runnerID] != nil {
				// collision lol
				runnerID = "runner_" + randomStr(liveRunnerIDRandomBytes)
			}
			staticRunner = &liveRunner{
				seed:     newRunnerSeed(),
				static:   true,
				sessions: make(map[string]*liveRunnerSession),
			}
			staticRunner.RunnerID = runnerID
			r.runners[runnerID] = staticRunner
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
		staticRunner.LiveRunnerHeartbeatRequest = req
		staticRunner.LastHeartbeat = time.Now()
		staticRunner.static = true
		staticRunner.healthURL = entry.HealthURL
		staticRunner.healthStatus = entry.HealthyStatusCode
		staticRunner.updatePriceConverterLocked()
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
	}
	return &StaticLiveRunnerRegistrationResponse{Runners: registrations}, nil
}

func staticRunnerRequest(entry StaticLiveRunnerConfigEntry) (LiveRunnerHeartbeatRequest, error) {
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
	return normalizeLiveRunnerRequest("static", req)
}

func normalizeLiveRunnerRequest(runnerID string, req LiveRunnerHeartbeatRequest) (LiveRunnerHeartbeatRequest, error) {
	u, err := url.ParseRequestURI(req.RunnerURL)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return LiveRunnerHeartbeatRequest{}, fmt.Errorf("invalid runner_url")
	}

	if req.PriceInfo.PixelsPerUnit <= 0 || req.PriceInfo.PricePerUnit <= 0 {
		return LiveRunnerHeartbeatRequest{}, fmt.Errorf("runner must include positive price_info")
	}
	unit := strings.ToUpper(strings.TrimSpace(req.PriceInfo.Unit))
	if unit != "USD" && unit != "WEI" {
		return LiveRunnerHeartbeatRequest{}, fmt.Errorf("price_info.unit must be USD or WEI")
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
	req.PriceInfo.Unit = unit

	return req, nil
}

func (r *LiveRunnerRegistry) Unregister(runnerID, auth string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.expireLocked(time.Now())
	runner := r.runners[runnerID]
	if runner == nil {
		return &RunnerError{StatusCode: http.StatusNotFound, Message: "runner not found"}
	}
	if runner.static {
		return &RunnerError{StatusCode: http.StatusBadRequest, Message: "static runner cannot unregister with heartbeat credentials"}
	}
	if !validSecret(auth, runner.heartbeatSecret) {
		return &RunnerError{StatusCode: http.StatusUnauthorized, Message: "invalid authorization"}
	}
	r.removeRunnerLocked(runnerID)
	return nil
}

func (r *LiveRunnerRegistry) ReserveSession(runnerID string) (string, string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.expireLocked(time.Now())

	runner := r.runners[runnerID]
	if runner == nil || !isReadyStatus(runner.Status) {
		return "", "", &RunnerError{StatusCode: http.StatusNotFound, Message: "runner not found"}
	}
	if len(runner.sessions) >= runner.Capacity {
		return "", "", &RunnerError{StatusCode: http.StatusServiceUnavailable, Message: "no capacity available for runner"}
	}

	sessionID := "session_" + randomStr(liveRunnerIDRandomBytes)
	for {
		if _, exists := runner.sessions[sessionID]; !exists {
			break
		}
		sessionID = "session_" + randomStr(liveRunnerIDRandomBytes)
	}
	runner.sessions[sessionID] = &liveRunnerSession{
		channels: make(map[string]*liveRunnerTrickleChannel),
	}
	return sessionID, runner.RunnerURL, nil
}

func (r *LiveRunnerRegistry) ReleaseSession(runnerID, sessionID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.expireLocked(time.Now())

	runner := r.runners[runnerID]
	if runner == nil {
		return &RunnerError{StatusCode: http.StatusNotFound, Message: "runner not found"}
	}
	runner.releaseSessionLocked(sessionID)
	return nil
}

func (r *LiveRunnerRegistry) RunnerMode(runnerID string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.expireLocked(time.Now())

	runner := r.runners[runnerID]
	if runner == nil || !isReadyStatus(runner.Status) {
		return "", &RunnerError{StatusCode: http.StatusNotFound, Message: "runner not found"}
	}
	if runner.Mode == "" {
		return LiveRunnerModePersistent, nil
	}
	return runner.Mode, nil
}

func (r *LiveRunnerRegistry) RunnerEndpointForSession(runnerID, sessionID string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.expireLocked(time.Now())

	runner := r.runners[runnerID]
	if runner == nil || !isReadyStatus(runner.Status) {
		return "", &RunnerError{StatusCode: http.StatusNotFound, Message: "runner not found"}
	}
	if _, exists := runner.sessions[sessionID]; !exists {
		return "", &RunnerError{StatusCode: http.StatusNotFound, Message: "runner session not found"}
	}
	return runner.RunnerURL, nil
}

func (r *LiveRunnerRegistry) SessionTokenForSession(runnerID, sessionID string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.expireLocked(time.Now())

	runner := r.runners[runnerID]
	if runner == nil || !isReadyStatus(runner.Status) {
		return "", &RunnerError{StatusCode: http.StatusNotFound, Message: "runner not found"}
	}
	if _, exists := runner.sessions[sessionID]; !exists {
		return "", &RunnerError{StatusCode: http.StatusNotFound, Message: "runner session not found"}
	}
	return runner.sessionToken(sessionID), nil
}

func (r *LiveRunnerRegistry) ValidSessionToken(runnerID, sessionID, token string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.expireLocked(time.Now())

	runner := r.runners[runnerID]
	if runner == nil || !isReadyStatus(runner.Status) {
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
	defer r.mu.Unlock()
	r.expireLocked(time.Now())

	if r.trickleSrv == nil {
		return LiveRunnerTrickleChannel{}, fmt.Errorf("trickle server is required")
	}
	channelURL, err := url.JoinPath(r.trickleBaseURL, channelName)
	if err != nil {
		return LiveRunnerTrickleChannel{}, err
	}

	session, err := r.liveRunnerSessionLocked(runnerID, sessionID)
	if err != nil {
		return LiveRunnerTrickleChannel{}, err
	}
	if channel := session.channels[name]; channel != nil {
		return channel.info, nil
	}

	publisher := trickle.NewLocalPublisher(r.trickleSrv, channelName, mimeType)
	publisher.CreateChannel()
	channel := &liveRunnerTrickleChannel{
		info: LiveRunnerTrickleChannel{
			Name:        name,
			ChannelName: channelName,
			URL:         channelURL,
			MimeType:    mimeType,
		},
		publisher: publisher,
	}
	session.channels[name] = channel
	return channel.info, nil
}

func (r *LiveRunnerRegistry) DeleteTrickleChannel(runnerID, sessionID, name string) error {
	name, _, err := normalizeTrickleChannelName(sessionID, name)
	if err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	r.expireLocked(time.Now())

	session, err := r.liveRunnerSessionLocked(runnerID, sessionID)
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
	defer r.mu.Unlock()
	r.expireLocked(time.Now())

	runners := make([]LiveRunnerDiscoveryRunner, 0, len(r.runners))
	for _, runner := range r.runners {
		if !isReadyStatus(runner.Status) {
			continue
		}
		discoveryRunner := runner.discoveryRunner()
		runners = append(runners, discoveryRunner)
	}
	return runners
}

func (r *LiveRunnerRegistry) expireLocked(now time.Time) {
	for runnerID, runner := range r.runners {
		if runner.static {
			continue
		}
		if now.Sub(runner.LastHeartbeat) > r.heartbeatTTL {
			r.removeRunnerLocked(runnerID)
		}
	}
}

func (r *LiveRunnerRegistry) healthLoop() {
	ticker := time.NewTicker(r.healthInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			r.checkStaticRunnerHealth()
		case <-r.stopHealth:
			return
		}
	}
}

func (r *LiveRunnerRegistry) checkStaticRunnerHealth() {
	type check struct {
		runnerID     string
		healthURL    string
		healthStatus int
	}
	r.mu.Lock()
	checks := []check{}
	for runnerID, runner := range r.runners {
		if runner.static {
			checks = append(checks, check{runnerID: runnerID, healthURL: runner.healthURL, healthStatus: runner.healthStatus})
		}
	}
	r.mu.Unlock()

	for _, c := range checks {
		healthy := r.staticRunnerHealthy(c.healthURL, c.healthStatus)
		r.mu.Lock()
		runner := r.runners[c.runnerID]
		if runner != nil && runner.static {
			if healthy {
				runner.Status = "ready"
			} else {
				runner.Status = "unavailable"
				for sessionID := range runner.sessions {
					runner.releaseSessionLocked(sessionID)
				}
			}
		}
		r.mu.Unlock()
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

func (r *LiveRunnerRegistry) removeRunnerLocked(runnerID string) {
	if runner := r.runners[runnerID]; runner != nil {
		for sessionID := range runner.sessions {
			runner.releaseSessionLocked(sessionID)
		}
		runner.stopPriceConverterLocked()
	}
	delete(r.runners, runnerID)
}

func (runner *liveRunner) releaseSessionLocked(sessionID string) {
	if session := runner.sessions[sessionID]; session != nil {
		session.closeChannelsLocked()
	}
	delete(runner.sessions, sessionID)
}

func (r *LiveRunnerRegistry) liveRunnerSessionLocked(runnerID, sessionID string) (*liveRunnerSession, error) {
	runner := r.runners[runnerID]
	if runner == nil || !isReadyStatus(runner.Status) {
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
			slog.Error("error closing live runner trickle channel", "channel", channel.info.ChannelName, "err", err)
		}
		delete(session.channels, name)
	}
}

func (channel *liveRunnerTrickleChannel) close() error {
	if channel == nil || channel.publisher == nil {
		return nil
	}
	err := channel.publisher.Close()
	channel.publisher = nil
	return err
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
	priceInfo := runner.PriceInfo
	if runner.converter != nil {
		converted, err := convertedPriceInfo(runner.converter)
		if err != nil {
			slog.Error("error reading converted live runner price", "app", runner.App, "endpoint", runner.RunnerURL, "err", err)
		} else {
			priceInfo = converted
		}
	}
	return LiveRunnerDiscoveryRunner{
		Endpoint:  runner.RunnerURL,
		GPU:       cloneLiveRunnerGPU(runner.GPU),
		App:       runner.App,
		Version:   runner.Version,
		Mode:      runner.Mode,
		PriceInfo: priceInfo,
	}
}

func (runner *liveRunner) updatePriceConverterLocked() {
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
	priceFixed, err := common.PriceToFixed(converter.Value())
	if err != nil {
		return LiveRunnerPriceInfo{}, err
	}
	return LiveRunnerPriceInfo{
		PricePerUnit:  priceFixed,
		PixelsPerUnit: 1,
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
