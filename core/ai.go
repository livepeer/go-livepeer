package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/ai/worker"
)

var errPipelineNotAvailable = errors.New("pipeline not available")

type AI interface {
	TextToImage(context.Context, worker.GenTextToImageJSONRequestBody) (*worker.ImageResponse, error)
	ImageToImage(context.Context, worker.GenImageToImageMultipartRequestBody) (*worker.ImageResponse, error)
	ImageToVideo(context.Context, worker.GenImageToVideoMultipartRequestBody) (*worker.VideoResponse, error)
	Upscale(context.Context, worker.GenUpscaleMultipartRequestBody) (*worker.ImageResponse, error)
	AudioToText(context.Context, worker.GenAudioToTextMultipartRequestBody) (*worker.TextResponse, error)
	LLM(context.Context, worker.GenLLMJSONRequestBody) (interface{}, error)
	SegmentAnything2(context.Context, worker.GenSegmentAnything2MultipartRequestBody) (*worker.MasksResponse, error)
	ImageToText(context.Context, worker.GenImageToTextMultipartRequestBody) (*worker.ImageToTextResponse, error)
	TextToSpeech(context.Context, worker.GenTextToSpeechJSONRequestBody) (*worker.AudioResponse, error)
	LiveVideoToVideo(context.Context, worker.GenLiveVideoToVideoJSONRequestBody) (*worker.LiveVideoToVideoResponse, error)
	Warm(context.Context, string, string, worker.RunnerEndpoint, worker.OptimizationFlags) error
	Stop(context.Context) error
	HasCapacity(string, string) bool
	EnsureImageAvailable(context.Context, string, string) error
	HardwareInformation() []worker.HardwareInformation
	GetLiveAICapacity(pipeline, modelID string) worker.Capacity
	Version() []worker.Version
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

// PipelineToCapability converts a pipeline name to a capability enum.
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
	Pipeline string `json:"pipeline"`
	ModelID  string `json:"model_id"`
	// used by worker
	URL               string                   `json:"url,omitempty"`
	Token             string                   `json:"token,omitempty"`
	Warm              bool                     `json:"warm,omitempty"`
	Capacity          int                      `json:"capacity,omitempty"`
	OptimizationFlags worker.OptimizationFlags `json:"optimization_flags,omitempty"`
	// used by orchestrator
	Gateway       string  `json:"gateway"`
	PricePerUnit  JSONRat `json:"price_per_unit,omitempty"`
	PixelsPerUnit JSONRat `json:"pixels_per_unit,omitempty"`
	Currency      string  `json:"currency,omitempty"`
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

// ParseStepsFromModelID parses the number of inference steps from the model ID suffix.
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

// AddAICapabilities adds AI capabilities to the node.
func (n *LivepeerNode) AddAICapabilities(caps *Capabilities) {
	aiConstraints := caps.PerCapability()
	if aiConstraints == nil {
		return
	}

	n.Capabilities.mutex.Lock()
	defer n.Capabilities.mutex.Unlock()
	for aiCapability, aiConstraint := range aiConstraints {
		_, capExists := n.Capabilities.constraints.perCapability[aiCapability]
		if !capExists {
			n.Capabilities.constraints.perCapability[aiCapability] = &CapabilityConstraints{
				Models: make(ModelConstraints),
			}
		}

		for modelId, modelConstraint := range aiConstraint.Models {
			_, modelExists := n.Capabilities.constraints.perCapability[aiCapability].Models[modelId]
			if modelExists {
				n.Capabilities.constraints.perCapability[aiCapability].Models[modelId].Capacity += modelConstraint.Capacity
			} else {
				n.Capabilities.constraints.perCapability[aiCapability].Models[modelId] = &ModelConstraint{Warm: modelConstraint.Warm, Capacity: modelConstraint.Capacity}
			}
		}
	}
}

// RemoveAICapabilities removes AI capabilities from the node.
func (n *LivepeerNode) RemoveAICapabilities(caps *Capabilities) {
	aiConstraints := caps.PerCapability()
	if aiConstraints == nil {
		return
	}

	n.Capabilities.mutex.Lock()
	defer n.Capabilities.mutex.Unlock()
	for capability, constraint := range aiConstraints {
		_, ok := n.Capabilities.constraints.perCapability[capability]
		if ok {
			for modelId, modelConstraint := range constraint.Models {
				_, modelExists := n.Capabilities.constraints.perCapability[capability].Models[modelId]
				if modelExists {
					n.Capabilities.constraints.perCapability[capability].Models[modelId].Capacity -= modelConstraint.Capacity
					if n.Capabilities.constraints.perCapability[capability].Models[modelId].Capacity <= 0 {
						delete(n.Capabilities.constraints.perCapability[capability].Models, modelId)
					}
				} else {
					glog.Errorf("failed to remove AI capability capacity, model does not exist pipeline=%v modelID=%v", capability, modelId)
				}
			}
		}
	}
}

func (n *LivepeerNode) ReserveAICapability(pipeline string, modelID string) error {
	cap, err := PipelineToCapability(pipeline)
	if err != nil {
		return err
	}

	_, hasCap := n.Capabilities.constraints.perCapability[cap]
	if hasCap {
		_, hasModel := n.Capabilities.constraints.perCapability[cap].Models[modelID]
		if hasModel {
			n.Capabilities.mutex.Lock()
			defer n.Capabilities.mutex.Unlock()
			if n.Capabilities.constraints.perCapability[cap].Models[modelID].Capacity > 0 {
				n.Capabilities.constraints.perCapability[cap].Models[modelID].Capacity -= 1
			} else {
				return fmt.Errorf("failed to reserve AI capability capacity, model capacity is 0 pipeline=%v modelID=%v", pipeline, modelID)
			}
			return nil
		}
		return fmt.Errorf("failed to reserve AI capability capacity, model does not exist pipeline=%v modelID=%v", pipeline, modelID)
	}
	return fmt.Errorf("failed to reserve AI capability capacity, pipeline does not exist pipeline=%v modelID=%v", pipeline, modelID)
}

func (n *LivepeerNode) ReleaseAICapability(pipeline string, modelID string) error {
	cap, err := PipelineToCapability(pipeline)
	if err != nil {
		return err
	}
	_, hasCap := n.Capabilities.constraints.perCapability[cap]
	if hasCap {
		_, hasModel := n.Capabilities.constraints.perCapability[cap].Models[modelID]
		if hasModel {
			n.Capabilities.mutex.Lock()
			defer n.Capabilities.mutex.Unlock()
			n.Capabilities.constraints.perCapability[cap].Models[modelID].Capacity += 1

			return nil
		}
		return fmt.Errorf("failed to release AI capability capacity, model does not exist pipeline=%v modelID=%v", pipeline, modelID)
	}
	return fmt.Errorf("failed to release AI capability capacity, pipeline does not exist pipeline=%v modelID=%v", pipeline, modelID)
}
