package ai

import (
	"context"
	"encoding/json"
	"errors"
	"os"

	"github.com/livepeer/ai-worker/worker"
)

type FileWorker struct {
	files map[string]string
}

func NewFileWorker(files map[string]string) *FileWorker {
	return &FileWorker{files: files}
}

func (w *FileWorker) TextToImage(ctx context.Context, req worker.GenTextToImageJSONRequestBody) (*worker.ImageResponse, error) {
	fname, ok := w.files["text-to-image"]
	if !ok {
		return nil, errors.New("text-to-image response file not found")
	}

	data, err := os.ReadFile(fname)
	if err != nil {
		return nil, err
	}

	var resp worker.ImageResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}

	return &resp, nil
}

func (w *FileWorker) ImageToImage(ctx context.Context, req worker.GenImageToImageMultipartRequestBody) (*worker.ImageResponse, error) {
	fname, ok := w.files["image-to-image"]
	if !ok {
		return nil, errors.New("image-to-image response file not found")
	}

	data, err := os.ReadFile(fname)
	if err != nil {
		return nil, err
	}

	var resp worker.ImageResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}

	return &resp, nil
}

func (w *FileWorker) ImageToVideo(ctx context.Context, req worker.GenImageToVideoMultipartRequestBody) (*worker.VideoResponse, error) {
	fname, ok := w.files["image-to-video"]
	if !ok {
		return nil, errors.New("image-to-video response file not found")
	}

	data, err := os.ReadFile(fname)
	if err != nil {
		return nil, err
	}

	var resp worker.VideoResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}

	return &resp, nil
}

func (w *FileWorker) Upscale(ctx context.Context, req worker.GenUpscaleMultipartRequestBody) (*worker.ImageResponse, error) {
	fname, ok := w.files["upscale"]
	if !ok {
		return nil, errors.New("upscale response file not found")
	}

	data, err := os.ReadFile(fname)
	if err != nil {
		return nil, err
	}

	var resp worker.ImageResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}

	return &resp, nil
}

func (w *FileWorker) Warm(ctx context.Context, containerName, modelID string) error {
	return nil
}

func (w *FileWorker) Stop(ctx context.Context, containerName string) error {
	return nil
}
