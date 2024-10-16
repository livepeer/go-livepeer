package core

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang/glog"
	"github.com/livepeer/ai-worker/worker"
)

var errPipelineNotAvailable = errors.New("pipeline not available")

type AI interface {
	TextToImage(context.Context, worker.GenTextToImageJSONRequestBody) (*worker.ImageResponse, error)
	ImageToImage(context.Context, worker.GenImageToImageMultipartRequestBody) (*worker.ImageResponse, error)
	ImageToVideo(context.Context, worker.GenImageToVideoMultipartRequestBody) (*worker.VideoResponse, error)
	Upscale(context.Context, worker.GenUpscaleMultipartRequestBody) (*worker.ImageResponse, error)
	AudioToText(context.Context, worker.GenAudioToTextMultipartRequestBody) (*worker.TextResponse, error)
	LLM(context.Context, worker.GenLLMFormdataRequestBody) (interface{}, error)
	SegmentAnything2(context.Context, worker.GenSegmentAnything2MultipartRequestBody) (*worker.MasksResponse, error)
	Warm(context.Context, string, string, worker.RunnerEndpoint, worker.OptimizationFlags) error
	Stop(context.Context) error
	HasCapacity(pipeline, modelID string) bool
}

// Custom type to parse a big.Rat from a JSON number.
type JSONRat struct{ *big.Rat }

func (s *JSONRat) UnmarshalJSON(data []byte) error {
	rat, ok := new(big.Rat).SetString(string(data))
	if !ok {
		return fmt.Errorf("value is not a number: %s", data)
	}
	*s = JSONRat{rat}
	return nil
}

func (s JSONRat) String() string {
	return s.FloatString(2)
}

// parsePipelineFromModelID converts a pipeline name to a capability enum.
func PipelineToCapability(pipeline string) (Capability, error) {
	if pipeline == "" {
		return Capability_Unused, errPipelineNotAvailable
	}

	pipelineName := strings.ToUpper(pipeline[:1]) + strings.ReplaceAll(pipeline[1:], "-", " ")

	for cap, desc := range CapabilityNameLookup {
		if pipelineName == desc {
			return cap, nil
		}
	}

	// No capability description matches name.
	return Capability_Unused, errPipelineNotAvailable
}

type AIModelConfig struct {
	Pipeline          string                   `json:"pipeline"`
	ModelID           string                   `json:"model_id"`
	URL               string                   `json:"url,omitempty"`
	Token             string                   `json:"token,omitempty"`
	Warm              bool                     `json:"warm,omitempty"`
	PricePerUnit      JSONRat                  `json:"price_per_unit,omitempty"`
	PixelsPerUnit     JSONRat                  `json:"pixels_per_unit,omitempty"`
	Currency          string                   `json:"currency,omitempty"`
	OptimizationFlags worker.OptimizationFlags `json:"optimization_flags,omitempty"`
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

type ModelsWebhook struct {
	configs    []AIModelConfig
	source     string
	mu         sync.RWMutex
	lastHash   string
	refreshInt time.Duration
	stopChan   chan struct{}
}

func NewAIModelWebhook(source string, refreshInterval time.Duration) (*ModelsWebhook, error) {
	webhook := &ModelsWebhook{
		source:     source,
		refreshInt: refreshInterval,
		stopChan:   make(chan struct{}),
	}
	err := webhook.refreshConfigs()
	if err != nil {
		return nil, err
	}
	go webhook.startRefreshing()
	return webhook, nil
}

func FetchAIModelConfigs(source string) ([]AIModelConfig, error) {
	var content string
	if strings.HasPrefix(source, "file://") {
		filePath := strings.TrimPrefix(source, "file://")
		data, err := os.ReadFile(filePath)
		if err != nil {
			return nil, err
		}
		content = string(data)
	} else {
		resp, err := http.Get(source)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		content = string(body)
	}
	return ParseAIModelConfigs(content)
}

func (w *ModelsWebhook) startRefreshing() {
	ticker := time.NewTicker(w.refreshInt)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			err := w.refreshConfigs()
			if err != nil {
				glog.Errorf("Error refreshing AI model configs: %v", err)
			}
		case <-w.stopChan:
			return
		}
	}
}

func (w *ModelsWebhook) refreshConfigs() error {
	content, err := w.fetchContent()
	if err != nil {
		return err
	}
	hash := hashContent(content)
	if hash != w.lastHash {
		configs, err := ParseAIModelConfigs(content)
		if err != nil {
			return err
		}
		w.mu.Lock()
		w.configs = configs
		w.lastHash = hash
		w.mu.Unlock()
		glog.V(6).Info("AI Model configurations have been updated.")
	}
	return nil
}

func (w *ModelsWebhook) fetchContent() (string, error) {
	if strings.HasPrefix(w.source, "file://") {
		filePath := strings.TrimPrefix(w.source, "file://")
		data, err := os.ReadFile(filePath)
		if err != nil {
			return "", err
		}
		return string(data), nil
	} else {
		resp, err := http.Get(w.source)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", err
		}
		return string(body), nil
	}
}

func hashContent(content string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(content)))
}

func (w *ModelsWebhook) GetConfigs() []AIModelConfig {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.configs
}

func (w *ModelsWebhook) Stop() {
	close(w.stopChan)
}
