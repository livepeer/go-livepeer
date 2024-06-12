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

// Custom type to handle both string and int but store as string.
type StringInt string

// UnmarshalJSON method to handle both string and int.
func (s *StringInt) UnmarshalJSON(data []byte) error {
	// Try to unmarshal as int.
	var intValue int64
	if err := json.Unmarshal(data, &intValue); err == nil {
		*s = StringInt(strconv.FormatInt(intValue, 10))
		return nil
	}

	var strValue string
	if err := json.Unmarshal(data, &strValue); err == nil {
		*s = StringInt(strValue)
		return nil
	}

	return fmt.Errorf("invalid value for StringInt: %s", data)
}

// String converts the StringInt type to a string.
func (s StringInt) String() string {
	return string(s)
}

type RemoteAIWorkerManager struct {
	// TODO Mapping by pipeline
	remoteWorkers []*RemoteAIWorker
	liveWorkers   map[net.Transcoder_RegisterAIWorkerServer]*RemoteAIWorker
	workersMutex  sync.Mutex

	// tasks
	// TODO: how to id tasks/sessions ?
	taskMutex sync.Mutex
}

func NewRemoteAIWorkerManager() *RemoteAIWorkerManager {
	return &RemoteAIWorkerManager{
		remoteWorkers: []*RemoteAIWorker{},
		liveWorkers:   map[net.Transcoder_RegisterAIWorkerServer]*RemoteAIWorker{},
		workersMutex:  sync.Mutex{},
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
	m.remoteWorkers = append(m.remoteWorkers, worker)
	m.liveWorkers[stream] = worker
	m.workersMutex.Unlock()

	<-worker.eof
	glog.Infof("Remote AI worker=%s done, removing from live AI workers map", from)

	m.workersMutex.Lock()
	delete(m.liveWorkers, stream)
	// TODO: remove from remoteWorkers
	m.workersMutex.Unlock()
}

func (m *RemoteAIWorkerManager) handleAIRequest()

func (m *RemoteAIWorkerManager) TextToImage(ctx context.Context, req worker.TextToImageJSONRequestBody) (*worker.ImageResponse, error) {
	return nil, nil

}

func (m *RemoteAIWorkerManager) ImageToImage(ctx context.Context, req worker.ImageToImageMultipartRequestBody) (*worker.ImageResponse, error) {
	return nil, nil
}

func (m *RemoteAIWorkerManager) ImageToVideo(ctx context.Context, req worker.ImageToVideoMultipartRequestBody) (*worker.VideoResponse, error) {
	return nil, nil
}

func (m *RemoteAIWorkerManager) Upscale(ctx context.Context, req worker.UpscaleMultipartRequestBody) (*worker.ImageResponse, error) {
	return nil, nil
}

func (m *RemoteAIWorkerManager) Warm(ctx context.Context, pipeline, modelID string, endpoint worker.RunnerEndpoint, flags worker.OptimizationFlags) error {
	return nil
}

func (m *RemoteAIWorkerManager) Stop(ctx context.Context) error {
	return nil
}

func (m *RemoteAIWorkerManager) HasCapacity(pipeline, modelID string) bool {
	return false
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
	PricePerUnit      StringInt                `json:"price_per_unit,omitempty"`
	PixelsPerUnit     StringInt                `json:"pixels_per_unit,omitempty"`
	OptimizationFlags worker.OptimizationFlags `json:"optimization_flags,omitempty"`
}

func (config *AIModelConfig) UnmarshalJSON(data []byte) error {
	// Custom type to avoid recursive calls to UnmarshalJSON
	type AIModelConfigAlias AIModelConfig
	// Set default values for fields
	defaultConfig := &AIModelConfigAlias{
		PixelsPerUnit: "1",
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
		warm, err := strconv.ParseBool(parts[3])
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
