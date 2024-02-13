package core

import (
	"context"
	"encoding/json"
	"os"
	"strings"

	"github.com/livepeer/ai-worker/worker"
)

type AI interface {
	TextToImage(context.Context, worker.TextToImageJSONRequestBody) (*worker.ImageResponse, error)
	ImageToImage(context.Context, worker.ImageToImageMultipartRequestBody) (*worker.ImageResponse, error)
	ImageToVideo(context.Context, worker.ImageToVideoMultipartRequestBody) (*worker.VideoResponse, error)
	Warm(context.Context, string, string, worker.RunnerEndpoint) error
	Stop(context.Context) error
}

type AIModelConfig struct {
	Pipeline string `json:"pipeline"`
	ModelID  string `json:"model_id"`
	URL      string `json:"url,omitempty"`
	Token    string `json:"token,omitempty"`
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
		pipeline := parts[0]
		modelID := parts[1]

		configs = append(configs, AIModelConfig{Pipeline: pipeline, ModelID: modelID})
	}

	return configs, nil
}
