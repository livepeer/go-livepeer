package runner

import (
	"errors"
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

var (
	ErrNoLiveRunnerCapacity = errors.New("no live runners have capacity")
)

type Capacity struct {
	ContainersIdle  int
	ContainersInUse int
}

type Version struct {
	Pipeline string `json:"pipeline,omitempty"`
	ModelId  string `json:"model_id,omitempty"`
	Version  string `json:"version,omitempty"`
}

type LiveRunnerModel struct {
	Pipeline string   `json:"pipeline"`
	Model    string   `json:"model"`
	Capacity int      `json:"capacity"`
	InUse    []string `json:"active,omitempty"`
}

type LiveRunnerGPU struct {
	ID     string `json:"id,omitempty"`
	Name   string `json:"name,omitempty"`
	VRAMMB int    `json:"vram_mb,omitempty"`
}

type LiveRunnerPriceInfo struct {
	PricePerUnit  int64 `json:"price_per_unit"`
	PixelsPerUnit int64 `json:"pixels_per_unit"`
}

type LiveRunnerHeartbeatRequest struct {
	RunnerID  string               `json:"runner_id,omitempty"`
	Label     string               `json:"label,omitempty"`
	RunnerURL string               `json:"runner_url"`
	Version   string               `json:"version,omitempty"`
	Status    string               `json:"status,omitempty"`
	GPUs      []LiveRunnerGPU      `json:"gpus,omitempty"`
	PriceInfo *LiveRunnerPriceInfo `json:"price_info,omitempty"`
	Models    []LiveRunnerModel    `json:"models,omitempty"`
}

type LiveRunnerHeartbeatResponse struct {
	RunnerID          string `json:"runner_id"`
	HeartbeatInterval string `json:"heartbeat_interval"`
	HeartbeatTTL      string `json:"heartbeat_ttl"`
}

type LiveRunnerDiscoveryCapability struct {
	Pipeline          string `json:"pipeline"`
	Model             string `json:"model"`
	Capacity          int    `json:"capacity"`
	CapacityAvailable int    `json:"capacity_available"`
	CapacityInUse     int    `json:"capacity_in_use"`
	Version           string `json:"version,omitempty"`
}

type LiveRunnerDiscoveryRunner struct {
	Endpoint     string                          `json:"endpoint"`
	GPU          *LiveRunnerGPU                  `json:"gpu,omitempty"`
	PriceInfo    *LiveRunnerPriceInfo            `json:"price_info,omitempty"`
	Capabilities []LiveRunnerDiscoveryCapability `json:"capabilities"`
}

type liveRunner struct {
	ID            string
	Label         string
	URL           string
	Version       string
	Status        string
	GPUs          []LiveRunnerGPU
	PriceInfo     *LiveRunnerPriceInfo
	Models        map[string]LiveRunnerModel
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

func liveRunnerModelKey(pipeline, model string) string {
	return pipeline + "/" + model
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
	r.runners[runner.ID] = runner

	return &LiveRunnerHeartbeatResponse{
		RunnerID:          runner.ID,
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

	models := make(map[string]LiveRunnerModel)
	for _, model := range req.Models {
		model = normalizeLiveRunnerModel(model)
		if model.Pipeline == "" || model.Model == "" {
			return nil, fmt.Errorf("runner model must include pipeline and model")
		}
		models[liveRunnerModelKey(model.Pipeline, model.Model)] = model
	}

	return &liveRunner{
		ID:            runnerID,
		Label:         req.Label,
		URL:           req.RunnerURL,
		Version:       req.Version,
		Status:        normalizeLiveRunnerStatus(req.Status),
		GPUs:          req.GPUs,
		PriceInfo:     req.PriceInfo,
		Models:        models,
		LastHeartbeat: time.Now(),
	}, nil
}

func (r *LiveRunnerRegistry) Unregister(runnerID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.removeRunnerLocked(runnerID)
}

func (r *LiveRunnerRegistry) HasCapacity(pipeline, model string) bool {
	return r.GetCapacity(pipeline, model).ContainersIdle > 0
}

func (r *LiveRunnerRegistry) GetCapacity(pipeline, model string) Capacity {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.expireLocked(time.Now())

	var capacity Capacity
	for _, runner := range r.runners {
		runnerModel, ok := runner.Models[liveRunnerModelKey(pipeline, model)]
		if !ok || runner.Status != "ready" {
			continue
		}
		inUse := runnerModel.inUseCount()
		capacity.ContainersInUse += inUse
		idle := runnerModel.Capacity - inUse
		if idle > 0 {
			capacity.ContainersIdle += idle
		}
	}
	return capacity
}

func (r *LiveRunnerRegistry) Version() []Version {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.expireLocked(time.Now())

	versions := make([]Version, 0, len(r.runners))
	for _, runner := range r.runners {
		if runner.Version == "" {
			continue
		}
		for _, model := range runner.Models {
			versions = append(versions, Version{
				Pipeline: model.Pipeline,
				ModelId:  model.Model,
				Version:  runner.Version,
			})
		}
	}
	return versions
}

func (r *LiveRunnerRegistry) Runners() []LiveRunnerDiscoveryRunner {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.expireLocked(time.Now())

	runners := make([]LiveRunnerDiscoveryRunner, 0, len(r.runners))
	for _, runner := range r.runners {
		if runner.Status != "ready" {
			continue
		}
		discoveryRunner := runner.discoveryRunner()
		if len(discoveryRunner.Capabilities) == 0 {
			continue
		}
		runners = append(runners, discoveryRunner)
	}
	return runners
}

func (r *LiveRunnerRegistry) SelectRunner(pipeline, model string) (*LiveRunnerDiscoveryRunner, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.expireLocked(time.Now())

	key := liveRunnerModelKey(pipeline, model)
	for _, runner := range r.runners {
		runnerModel, ok := runner.Models[key]
		if !ok || !runner.available(runnerModel) {
			continue
		}
		discoveryRunner := runner.discoveryRunnerForModel(runnerModel)
		return &discoveryRunner, nil
	}

	return nil, ErrNoLiveRunnerCapacity
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

func (runner *liveRunner) available(model LiveRunnerModel) bool {
	if runner.Status != "ready" {
		return false
	}
	return model.Capacity > model.inUseCount()
}

func (runner *liveRunner) discoveryRunner() LiveRunnerDiscoveryRunner {
	capabilities := make([]LiveRunnerDiscoveryCapability, 0, len(runner.Models))
	for _, model := range runner.Models {
		capabilities = append(capabilities, runner.discoveryCapability(model))
	}
	return LiveRunnerDiscoveryRunner{
		Endpoint:     runner.URL,
		GPU:          firstLiveRunnerGPU(runner.GPUs),
		PriceInfo:    cloneLiveRunnerPriceInfo(runner.PriceInfo),
		Capabilities: capabilities,
	}
}

func (runner *liveRunner) discoveryRunnerForModel(model LiveRunnerModel) LiveRunnerDiscoveryRunner {
	return LiveRunnerDiscoveryRunner{
		Endpoint:     runner.URL,
		GPU:          firstLiveRunnerGPU(runner.GPUs),
		PriceInfo:    cloneLiveRunnerPriceInfo(runner.PriceInfo),
		Capabilities: []LiveRunnerDiscoveryCapability{runner.discoveryCapability(model)},
	}
}

func (runner *liveRunner) discoveryCapability(model LiveRunnerModel) LiveRunnerDiscoveryCapability {
	inUse := model.inUseCount()
	available := model.Capacity - inUse
	if available < 0 {
		available = 0
	}
	return LiveRunnerDiscoveryCapability{
		Pipeline:          model.Pipeline,
		Model:             model.Model,
		Capacity:          model.Capacity,
		CapacityAvailable: available,
		CapacityInUse:     inUse,
		Version:           runner.Version,
	}
}

func normalizeLiveRunnerModel(model LiveRunnerModel) LiveRunnerModel {
	if model.Capacity < 0 {
		model.Capacity = 0
	}
	return model
}

func (model LiveRunnerModel) inUseCount() int {
	return len(model.InUse)
}

func normalizeLiveRunnerStatus(status string) string {
	if status == "" {
		return "ready"
	}
	return strings.ToLower(status)
}

func firstLiveRunnerGPU(gpus []LiveRunnerGPU) *LiveRunnerGPU {
	if len(gpus) == 0 {
		return nil
	}
	gpu := gpus[0]
	return &gpu
}

func cloneLiveRunnerPriceInfo(price *LiveRunnerPriceInfo) *LiveRunnerPriceInfo {
	if price == nil {
		return nil
	}
	copy := *price
	return &copy
}

type LiveRunnerManager interface {
	Heartbeat(req LiveRunnerHeartbeatRequest) (*LiveRunnerHeartbeatResponse, error)
	Unregister(runnerID string)
	Runners() []LiveRunnerDiscoveryRunner
	SelectRunner(pipeline, model string) (*LiveRunnerDiscoveryRunner, error)
}
