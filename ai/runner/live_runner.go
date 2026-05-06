package runner

import (
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/livepeer/go-livepeer/common"
)

const (
	defaultLiveRunnerHeartbeatInterval = 5 * time.Second
	defaultLiveRunnerHeartbeatTTL      = 20 * time.Second
)

type Capacity struct {
	ContainersIdle  int
	ContainersInUse int
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
	InUse     []string            `json:"active,omitempty"`
	PriceInfo LiveRunnerPriceInfo `json:"price_info"`
}

type LiveRunnerHeartbeatResponse struct {
	RunnerID          string `json:"runner_id"`
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

type liveRunner struct {
	LiveRunnerHeartbeatRequest
	LastHeartbeat time.Time
}

type LiveRunnerRegistry struct {
	mu                sync.Mutex
	runners           map[string]*liveRunner
	heartbeatInterval time.Duration
	heartbeatTTL      time.Duration
}

func NewLiveRunnerRegistry() *LiveRunnerRegistry {
	return &LiveRunnerRegistry{
		runners:           make(map[string]*liveRunner),
		heartbeatInterval: defaultLiveRunnerHeartbeatInterval,
		heartbeatTTL:      defaultLiveRunnerHeartbeatTTL,
	}
}

func (r *LiveRunnerRegistry) Heartbeat(req LiveRunnerHeartbeatRequest) (*LiveRunnerHeartbeatResponse, error) {
	req.RunnerID = strings.TrimSpace(req.RunnerID)
	if req.RunnerID == "" {
		req.RunnerID = "runner_" + common.RandomIDGenerator(4)
	}

	runner, err := normalizeLiveRunner(req.RunnerID, req)
	if err != nil {
		return nil, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	r.expireLocked(time.Now())
	r.runners[runner.RunnerID] = runner

	return &LiveRunnerHeartbeatResponse{
		RunnerID:          runner.RunnerID,
		HeartbeatInterval: r.heartbeatInterval.String(),
		HeartbeatTTL:      r.heartbeatTTL.String(),
	}, nil
}

func normalizeLiveRunner(runnerID string, req LiveRunnerHeartbeatRequest) (*liveRunner, error) {
	u, err := url.ParseRequestURI(req.RunnerURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("invalid runner_url")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("runner_url must use http or https")
	}

	if req.PriceInfo.PixelsPerUnit <= 0 || req.PriceInfo.PricePerUnit <= 0 {
		return nil, fmt.Errorf("runner must include positive price_info")
	}
	if strings.TrimSpace(req.PriceInfo.Unit) == "" {
		return nil, fmt.Errorf("runner must include price_info.unit")
	}
	if req.PriceInfo.Unit != strings.ToUpper(strings.TrimSpace(req.PriceInfo.Unit)) {
		return nil, fmt.Errorf("price_info.unit must be uppercase and trimmed")
	}
	if req.App == "" {
		return nil, fmt.Errorf("runner must include app")
	}
	if req.App != strings.TrimSpace(req.App) {
		return nil, fmt.Errorf("app must be trimmed")
	}
	if req.Status != strings.ToLower(strings.TrimSpace(req.Status)) {
		return nil, fmt.Errorf("status must be lowercase and trimmed")
	}
	if req.Capacity < 0 {
		return nil, fmt.Errorf("capacity must be >= 0")
	}
	req.RunnerID = runnerID
	req.InUse = cloneStringSlice(req.InUse)

	return &liveRunner{
		LiveRunnerHeartbeatRequest: req,
		LastHeartbeat:              time.Now(),
	}, nil
}

func (r *LiveRunnerRegistry) Unregister(runnerID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.removeRunnerLocked(runnerID)
}

func (r *LiveRunnerRegistry) GetCapacity(pipeline, model string) Capacity {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.expireLocked(time.Now())

	appKey := pipeline + "/" + model
	var capacity Capacity
	for _, runner := range r.runners {
		if runner.App != appKey || !isReadyStatus(runner.Status) {
			continue
		}
		inUse := len(runner.InUse)
		capacity.ContainersInUse += inUse
		idle := runner.Capacity - inUse
		if idle > 0 {
			capacity.ContainersIdle += idle
		}
	}
	return capacity
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
	delete(r.runners, runnerID)
}

func (runner *liveRunner) discoveryRunner() LiveRunnerDiscoveryRunner {
	return LiveRunnerDiscoveryRunner{
		Endpoint:  runner.RunnerURL,
		GPU:       cloneLiveRunnerGPU(runner.GPU),
		App:       runner.App,
		Version:   runner.Version,
		PriceInfo: runner.PriceInfo,
	}
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

func cloneStringSlice(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	copied := make([]string, len(values))
	copy(copied, values)
	return copied
}
