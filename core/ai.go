package core

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/livepeer/ai-worker/worker"
)

type AI interface {
	TextToImage(context.Context, worker.TextToImageJSONRequestBody) (*worker.ImageResponse, error)
	ImageToImage(context.Context, worker.ImageToImageMultipartRequestBody) (*worker.ImageResponse, error)
	ImageToVideo(context.Context, worker.ImageToVideoMultipartRequestBody) (*worker.VideoResponse, error)
	Upscale(context.Context, worker.UpscaleMultipartRequestBody) (*worker.ImageResponse, error)
	AudioToText(context.Context, worker.AudioToTextMultipartRequestBody) (*worker.TextResponse, error)
	Warm(context.Context, string, string, worker.RunnerEndpoint, worker.OptimizationFlags, []int) error
	Stop(context.Context) error
	HasCapacity(pipeline, modelID string) bool
}

type AIModelConfig struct {
	Pipeline          string                   `json:"pipeline"`
	ModelID           string                   `json:"model_id"`
	Gpus              []int                    `json:"gpus,omitempty"`
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
		warm, err := strconv.ParseBool(parts[3])
		if err != nil {
			return nil, err
		}

		gpuStrings := strings.Split(parts[4], ",")
		gpus := make([]int, len(gpuStrings))
		for i, gpuString := range gpuStrings {
			gpu, err := strconv.Atoi(gpuString)
			if err != nil {
				return nil, err
			}
			gpus[i] = gpu
		}

		configs = append(configs, AIModelConfig{Pipeline: pipeline, ModelID: modelID, Warm: warm, Gpus: gpus})
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
