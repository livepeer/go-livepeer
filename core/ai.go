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

	"github.com/livepeer/ai-worker/worker"
)

var errPipelineNotAvailable = errors.New("pipeline not available")

type AI interface {
	TextToImage(context.Context, worker.TextToImageJSONRequestBody) (*worker.ImageResponse, error)
	ImageToImage(context.Context, worker.ImageToImageMultipartRequestBody) (*worker.ImageResponse, error)
	ImageToVideo(context.Context, worker.ImageToVideoMultipartRequestBody) (*worker.VideoResponse, error)
	Upscale(context.Context, worker.UpscaleMultipartRequestBody) (*worker.ImageResponse, error)
	AudioToText(context.Context, worker.AudioToTextMultipartRequestBody) (*worker.TextResponse, error)
	SegmentAnything2(context.Context, worker.SegmentAnything2MultipartRequestBody) (*worker.MasksResponse, error)
	Warm(context.Context, string, string, worker.RunnerEndpoint, worker.OptimizationFlags) error
	Stop(context.Context) error
	HasCapacity(pipeline, modelID string) bool
}

// Mapping for per pipeline custom container images.
var PipelineToImage = map[string]string{
	"segment-anything-2": "livepeer/ai-runner:segment-anything-2",
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
