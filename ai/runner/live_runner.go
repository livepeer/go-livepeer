package runner

import (
	"fmt"
	"log/slog"
	"math/big"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/trickle"
)

const (
	defaultLiveRunnerHeartbeatInterval = 5 * time.Second
	defaultLiveRunnerHeartbeatTTL      = 20 * time.Second
)

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
	GPU       *LiveRunnerGPU      `json:"gpu,omitempty"`
	App       string              `json:"app"`
	Capacity  int                 `json:"capacity"`
	PriceInfo LiveRunnerPriceInfo `json:"price_info"`
}

type LiveRunnerHeartbeatResponse struct {
	RunnerID          string `json:"runner_id"`
	Orchestrator      string `json:"orchestrator,omitempty"`
	HeartbeatInterval string `json:"heartbeat_interval"`
	HeartbeatTTL      string `json:"heartbeat_ttl"`
}

type LiveRunnerDiscoveryRunner struct {
	Endpoint  string              `json:"endpoint"`
	GPU       *LiveRunnerGPU      `json:"gpu,omitempty"`
	App       string              `json:"app"`
	Version   string              `json:"version,omitempty"`
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
	LastHeartbeat time.Time
	sessions      map[string]*liveRunnerSession
	priceSource   LiveRunnerPriceInfo
	converter     *core.AutoConvertedPrice
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
	heartbeatInterval time.Duration
	heartbeatTTL      time.Duration
	trickleSrv        *trickle.Server
	trickleBaseURL    string
}

var liveRunnerChannelNamePattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

func NewLiveRunnerRegistry() *LiveRunnerRegistry {
	return &LiveRunnerRegistry{
		runners:           make(map[string]*liveRunner),
		heartbeatInterval: defaultLiveRunnerHeartbeatInterval,
		heartbeatTTL:      defaultLiveRunnerHeartbeatTTL,
	}
}

func (r *LiveRunnerRegistry) SetTrickleServer(srv *trickle.Server, baseURL string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.trickleSrv = srv
	r.trickleBaseURL = baseURL
}

func (r *LiveRunnerRegistry) Heartbeat(req LiveRunnerHeartbeatRequest) (*LiveRunnerHeartbeatResponse, error) {
	req.RunnerID = strings.TrimSpace(req.RunnerID)
	if req.RunnerID == "" {
		req.RunnerID = "runner_" + common.RandomIDGenerator(4)
	}

	req, err := normalizeLiveRunnerRequest(req.RunnerID, req)
	if err != nil {
		return nil, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	r.expireLocked(time.Now())

	runner := r.runners[req.RunnerID]
	if runner == nil {
		runner = &liveRunner{
			sessions: make(map[string]*liveRunnerSession),
		}
		r.runners[req.RunnerID] = runner
	}
	runner.LiveRunnerHeartbeatRequest = req
	runner.LastHeartbeat = time.Now()
	runner.updatePriceConverterLocked()

	return &LiveRunnerHeartbeatResponse{
		RunnerID:          runner.RunnerID,
		HeartbeatInterval: r.heartbeatInterval.String(),
		HeartbeatTTL:      r.heartbeatTTL.String(),
	}, nil
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

func (r *LiveRunnerRegistry) Unregister(runnerID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.removeRunnerLocked(runnerID)
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

	sessionID := "session_" + common.RandomIDGenerator(4)
	for {
		if _, exists := runner.sessions[sessionID]; !exists {
			break
		}
		sessionID = "session_" + common.RandomIDGenerator(4)
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
		if now.Sub(runner.LastHeartbeat) > r.heartbeatTTL {
			r.removeRunnerLocked(runnerID)
		}
	}
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
