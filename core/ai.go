package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/golang/glog"
	"github.com/livepeer/ai-worker/worker"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/net"
)

type AI interface {
	TextToImage(context.Context, worker.TextToImageJSONRequestBody) (*worker.ImageResponse, error)
	ImageToImage(context.Context, worker.ImageToImageMultipartRequestBody) (*worker.ImageResponse, error)
	ImageToVideo(context.Context, worker.ImageToVideoMultipartRequestBody) (*worker.VideoResponse, error)
	Upscale(context.Context, worker.UpscaleMultipartRequestBody) (*worker.ImageResponse, error)
	AudioToText(context.Context, worker.AudioToTextMultipartRequestBody) (*worker.TextResponse, error)
	Warm(context.Context, string, string, worker.RunnerEndpoint, worker.OptimizationFlags) error
	Stop(context.Context) error
	HasCapacity(pipeline, modelID string) bool
}

type RemoteAIResultChan chan *RemoteAIWorkerResult

type RemoteAIWorkerManager struct {
	// workers mapped by Pipeline(Capability) + ModelID
	remoteWorkers map[Capability]map[string][]*RemoteAIWorker
	liveWorkers   map[net.Transcoder_RegisterAIWorkerServer]*RemoteAIWorker
	workersMutex  sync.Mutex

	// tasks
	taskChans map[int64]RemoteAIResultChan
	taskMutex sync.RWMutex
	taskCount int64
}
type RemoteAIWorkerResult struct {
	JobType net.AIRequestType
	TaskID  int64
	Bytes   []byte
	Err     string
}

func NewRemoteAIWorkerManager() *RemoteAIWorkerManager {
	return &RemoteAIWorkerManager{
		remoteWorkers: map[Capability]map[string][]*RemoteAIWorker{},
		liveWorkers:   map[net.Transcoder_RegisterAIWorkerServer]*RemoteAIWorker{},
		workersMutex:  sync.Mutex{},

		taskChans: map[int64]RemoteAIResultChan{},
		taskMutex: sync.RWMutex{},
		taskCount: 0,
	}
}

func (m *RemoteAIWorkerManager) Manage(stream net.Transcoder_RegisterAIWorkerServer, capabilities *net.Capabilities) {
	from := common.GetConnectionAddr(stream.Context())
	worker := NewRemoteAIWorker(m, stream, from, CapabilitiesFromNetCapabilities(capabilities))
	go func() {
		ctx := stream.Context()
		<-ctx.Done()
		err := ctx.Err()
		glog.Errorf("Stream closed for remote AI worker=%s err=%q", from, err)
		worker.done()
	}()

	m.workersMutex.Lock()
	for cap, constraints := range capabilities.Constraints {
		c := Capability(cap)
		// Initialize the inner map if it's nil
		if m.remoteWorkers[c] == nil {
			m.remoteWorkers[c] = make(map[string][]*RemoteAIWorker)
		}
		for modelID, _ := range constraints.Models {
			// Initialize the remoteWorkers slice if it's nil
			if m.remoteWorkers[Capability(cap)][modelID] == nil {
				m.remoteWorkers[Capability(cap)][modelID] = []*RemoteAIWorker{}
			}
			m.remoteWorkers[c][modelID] = append(m.remoteWorkers[c][modelID], worker)
		}
	}
	m.liveWorkers[stream] = worker
	m.workersMutex.Unlock()

	<-worker.eof
	glog.Infof("Remote AI worker=%s done, removing from live AI workers map", from)

	m.workersMutex.Lock()
	delete(m.liveWorkers, stream)

	// Remove worker from remoteWorkers
	for cap, modelMap := range m.remoteWorkers {
		for modelID, workers := range modelMap {
			for i, w := range workers {
				if w == worker {
					// Remove the worker from the slice
					m.remoteWorkers[cap][modelID] = append(workers[:i], workers[i+1:]...)
					break
				}
			}
			// If the worker list is empty, remove the modelID entry
			if len(m.remoteWorkers[cap][modelID]) == 0 {
				delete(m.remoteWorkers[cap], modelID)
			}
		}
		// If the capability map is empty, remove the capability entry
		if len(m.remoteWorkers[cap]) == 0 {
			delete(m.remoteWorkers, cap)
		}
	}

	m.workersMutex.Unlock()
}

func (m *RemoteAIWorkerManager) processAIRequest(ctx context.Context, capability Capability, req interface{}, aiRequestType net.AIRequestType) (interface{}, error) {
	taskID, taskChan := m.addTaskChan()
	defer m.removeTaskChan(taskID)

	modelID := getModelID(req)

	w, err := m.selectWorker(capability, modelID)
	if err != nil {
		return nil, err
	}

	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	remoteReq := &net.NotifyAIJob{
		Type:   aiRequestType,
		TaskID: taskID,
		Data:   jsonData,
	}

	if err := w.stream.Send(remoteReq); err != nil {
		return nil, err
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case chanData := <-taskChan:
		if chanData.Err != "" {
			return nil, fmt.Errorf("%v", chanData.Err)
		}
		glog.Infof("Received AI result for task %d", chanData.TaskID)
		var res interface{}
		switch aiRequestType {
		case net.AIRequestType_ImageToVideo:
			var videoRes worker.VideoResponse
			if err := json.Unmarshal(chanData.Bytes, &videoRes); err != nil {
				return nil, err
			}
			res = &videoRes
		default:
			var imgRes worker.ImageResponse
			if err := json.Unmarshal(chanData.Bytes, &imgRes); err != nil {
				return nil, err
			}
			res = &imgRes
		}
		return res, nil
	}
}

func (m *RemoteAIWorkerManager) selectWorker(capability Capability, modelID string) (*RemoteAIWorker, error) {
	m.workersMutex.Lock()
	defer m.workersMutex.Unlock()
	workers := m.remoteWorkers[capability][modelID]

	if len(workers) == 0 {
		return nil, ErrOrchCap
	}

	w := workers[0]
	glog.Infof("Selected worker %s for model %s; Total worker count: %v", w.addr, modelID, len(workers))

	if len(workers) > 1 {
		m.remoteWorkers[capability][modelID] = append(workers[1:], workers[0])
	}

	return w, nil
}

func (m *RemoteAIWorkerManager) TextToImage(ctx context.Context, req worker.TextToImageJSONRequestBody) (*worker.ImageResponse, error) {
	res, err := m.processAIRequest(ctx, Capability_TextToImage, req, net.AIRequestType_TextToImage)
	if err != nil {
		return nil, err
	}
	return res.(*worker.ImageResponse), nil
}

func (m *RemoteAIWorkerManager) ImageToImage(ctx context.Context, req worker.ImageToImageMultipartRequestBody) (*worker.ImageResponse, error) {
	res, err := m.processAIRequest(ctx, Capability_ImageToImage, req, net.AIRequestType_ImageToImage)
	if err != nil {
		return nil, err
	}
	return res.(*worker.ImageResponse), nil
}

func (m *RemoteAIWorkerManager) Upscale(ctx context.Context, req worker.UpscaleMultipartRequestBody) (*worker.ImageResponse, error) {
	res, err := m.processAIRequest(ctx, Capability_Upscale, req, net.AIRequestType_Upscale)
	if err != nil {
		return nil, err
	}
	return res.(*worker.ImageResponse), nil
}

// Helper function to get ModelId from different request types
func getModelID(req interface{}) string {
	switch r := req.(type) {
	case worker.TextToImageJSONRequestBody:
		return *r.ModelId
	case worker.ImageToImageMultipartRequestBody:
		return *r.ModelId
	case worker.UpscaleMultipartRequestBody:
		return *r.ModelId
	case worker.ImageToVideoMultipartRequestBody:
		return *r.ModelId
	default:
		return ""
	}
}

func (m *RemoteAIWorkerManager) ImageToVideo(ctx context.Context, req worker.ImageToVideoMultipartRequestBody) (*worker.VideoResponse, error) {
	res, err := m.processAIRequest(ctx, Capability_ImageToVideo, req, net.AIRequestType_ImageToVideo)
	if err != nil {
		return nil, err
	}
	return res.(*worker.VideoResponse), nil
}

func (m *RemoteAIWorkerManager) Warm(ctx context.Context, pipeline, modelID string, endpoint worker.RunnerEndpoint, flags worker.OptimizationFlags) error {
	return nil
}

func (m *RemoteAIWorkerManager) Stop(ctx context.Context) error {
	return nil
}

func (m *RemoteAIWorkerManager) HasCapacity(pipeline, modelID string) bool {
	m.workersMutex.Lock()
	defer m.workersMutex.Unlock()
	var cap Capability
	switch pipeline {
	case "text-to-image":
		cap = Capability_TextToImage
	case "image-to-image":
		cap = Capability_ImageToImage
	case "upscale":
		cap = Capability_Upscale
	case "image-to-video":
		cap = Capability_ImageToVideo
	default:
		return false
	}
	return len(m.remoteWorkers[cap][modelID]) > 0

}

func (m *RemoteAIWorkerManager) aiResult(res *RemoteAIWorkerResult) {
	tc, err := m.getTaskChan(res.TaskID)
	if err != nil {
		glog.V(common.DEBUG).Info("No AI job channel for ", res.TaskID)
		return
	}
	tc <- res
}

func (m *RemoteAIWorkerManager) getTaskChan(taskID int64) (RemoteAIResultChan, error) {
	m.taskMutex.RLock()
	defer m.taskMutex.RUnlock()
	if tc, ok := m.taskChans[taskID]; ok {
		return tc, nil
	}
	return nil, fmt.Errorf("No AI job channel")
}

func (m *RemoteAIWorkerManager) addTaskChan() (int64, RemoteAIResultChan) {
	m.taskMutex.Lock()
	defer m.taskMutex.Unlock()
	taskID := m.taskCount
	m.taskCount++
	if tc, ok := m.taskChans[taskID]; ok {
		// should really never happen
		glog.V(common.DEBUG).Info("AI job channel already exists for ", taskID)
		return taskID, tc
	}
	m.taskChans[taskID] = make(RemoteAIResultChan, 1)
	return taskID, m.taskChans[taskID]
}

func (m *RemoteAIWorkerManager) removeTaskChan(taskID int64) {
	m.taskMutex.Lock()
	defer m.taskMutex.Unlock()
	if _, ok := m.taskChans[taskID]; !ok {
		glog.V(common.DEBUG).Info("Transcoder channel nonexistent for job ", taskID)
		return
	}
	delete(m.taskChans, taskID)
}

type RemoteAIWorker struct {
	manager      *RemoteAIWorkerManager
	stream       net.Transcoder_RegisterAIWorkerServer
	addr         string
	capabilities *Capabilities // TODO: AI capabilities only
	eof          chan struct{}
}

func NewRemoteAIWorker(manager *RemoteAIWorkerManager, stream net.Transcoder_RegisterAIWorkerServer, addr string, capabilities *Capabilities) *RemoteAIWorker {
	return &RemoteAIWorker{
		manager:      manager,
		stream:       stream,
		addr:         addr,
		capabilities: capabilities,
		eof:          make(chan struct{}),
	}
}

func (w *RemoteAIWorker) done() {
	// select so we don't block indefinitely if there's no listener
	select {
	case w.eof <- struct{}{}:
	default:
	}
}

type AIModelConfig struct {
	Pipeline          string                   `json:"pipeline"`
	ModelID           string                   `json:"model_id"`
	URL               string                   `json:"url,omitempty"`
	Token             string                   `json:"token,omitempty"`
	Warm              bool                     `json:"warm,omitempty"`
	PricePerUnit      int64                    `json:"price_per_unit,omitempty"`
	PixelsPerUnit     int64                    `json:"pixels_per_unit,omitempty"`
	OptimizationFlags worker.OptimizationFlags `json:"optimization_flags,omitempty"`
}

func (config *AIModelConfig) UnmarshalJSON(data []byte) error {
	// Custom type to avoid recursive calls to UnmarshalJSON
	type AIModelConfigAlias AIModelConfig
	// Set default values for fields
	defaultConfig := &AIModelConfigAlias{
		PixelsPerUnit: 1,
	}

	if err := json.Unmarshal(data, defaultConfig); err != nil {
		return err
	}

	*config = AIModelConfig(*defaultConfig)

	return nil
}

func ParseAIModelConfigs(config string) ([]AIModelConfig, error) {
	var configs []AIModelConfig

	info, err := os.Stat(config)
	if err == nil && !info.IsDir() {
		data, err := os.ReadFile(config)
		if err != nil {
			return nil, err
		}

		if err := json.Unmarshal(data, &configs); err != nil {
			return nil, err
		}

		return configs, nil
	}

	models := strings.Split(config, ",")
	for _, m := range models {
		parts := strings.Split(m, ":")
		if len(parts) < 3 {
			return nil, errors.New("invalid AI model config expected <pipeline>:<model_id>:<warm>")
		}

		pipeline := parts[0]
		modelID := parts[1]
		warm, err := strconv.ParseBool(parts[2])
		if err != nil {
			return nil, err
		}

		configs = append(configs, AIModelConfig{Pipeline: pipeline, ModelID: modelID, Warm: warm})
	}

	return configs, nil
}

// parseStepsFromModelID parses the number of inference steps from the model ID suffix.
func ParseStepsFromModelID(modelID *string, defaultSteps float64) float64 {
	numInferenceSteps := defaultSteps

	// Regular expression to find "_<number>step" pattern anywhere in the model ID.
	stepPattern := regexp.MustCompile(`_(\d+)step`)
	matches := stepPattern.FindStringSubmatch(*modelID)
	if len(matches) == 2 {
		if parsedSteps, err := strconv.Atoi(matches[1]); err == nil {
			numInferenceSteps = float64(parsedSteps)
		}
	}

	return numInferenceSteps
}
