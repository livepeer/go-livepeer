package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"sync"
)

// EnvValue unmarshals JSON booleans as strings for compatibility with env variables.
type EnvValue string

// UnmarshalJSON converts JSON booleans to strings for EnvValue.
func (sb *EnvValue) UnmarshalJSON(b []byte) error {
	var boolVal bool
	err := json.Unmarshal(b, &boolVal)
	if err == nil {
		*sb = EnvValue(strconv.FormatBool(boolVal))
		return nil
	}

	var strVal string
	err = json.Unmarshal(b, &strVal)
	if err == nil {
		*sb = EnvValue(strVal)
	}

	return err
}

// String returns the string representation of the EnvValue.
func (sb EnvValue) String() string {
	return string(sb)
}

// OptimizationFlags is a map of optimization flags to be passed to the pipeline.
type OptimizationFlags map[string]EnvValue

type Worker struct {
	manager            *DockerManager
	externalContainers map[string]*RunnerContainer
	mu                 *sync.Mutex
}

func NewWorker(imageOverrides ImageOverrides, verboseLogs bool, gpus []string, modelDir string, containerCreatorID string) (*Worker, error) {
	manager, err := NewDockerManager(imageOverrides, verboseLogs, gpus, modelDir, nil, containerCreatorID)
	if err != nil {
		return nil, fmt.Errorf("error creating docker manager: %w", err)
	}

	return &Worker{
		manager:            manager,
		externalContainers: make(map[string]*RunnerContainer),
		mu:                 &sync.Mutex{},
	}, nil
}

func (w *Worker) HardwareInformation() []HardwareInformation {
	var hardware []HardwareInformation
	for _, rc := range w.externalContainers {
		if rc.Hardware != nil {
			hardware = append(hardware, *rc.Hardware)
		} else {
			hardware = append(hardware, HardwareInformation{})
		}
	}
	return append(hardware, w.manager.HardwareInformation()...)
}

func (w *Worker) GetLiveAICapacity(pipeline, modelID string) Capacity {
	capacity, _ := w.manager.GetCapacity(pipeline, modelID)
	return capacity
}

func (w *Worker) Version() []Version {
	var version []Version
	for _, rc := range w.externalContainers {
		if rc.Version != nil {
			version = append(version, *rc.Version)
		} else {
			version = append(version, Version{})
		}
	}

	return append(version, w.manager.Version()...)
}

func (w *Worker) LiveVideoToVideo(ctx context.Context, req GenLiveVideoToVideoJSONRequestBody) (*LiveVideoToVideoResponse, error) {
	// Live video containers keep running after the initial request, so we use a background context to borrow the container.
	c, err := w.borrowContainer(context.Background(), "live-video-to-video", *req.ModelId)
	if err != nil {
		return nil, err
	}

	resp, err := c.Client.GenLiveVideoToVideoWithResponse(ctx, req)
	if err != nil {
		return nil, err
	}

	if resp.JSON400 != nil {
		val, err := json.Marshal(resp.JSON400)
		if err != nil {
			return nil, err
		}
		slog.Error("live-video-to-video container returned 400", slog.String("err", string(val)))
		return nil, errors.New("live-video-to-video container returned 400: " + resp.JSON400.Detail.Msg)
	}

	if resp.JSON401 != nil {
		val, err := json.Marshal(resp.JSON401)
		if err != nil {
			return nil, err
		}
		slog.Error("live-video-to-video container returned 401", slog.String("err", string(val)))
		return nil, errors.New("live-video-to-video container returned 401: " + resp.JSON401.Detail.Msg)
	}

	if resp.JSON422 != nil {
		val, err := json.Marshal(resp.JSON422)
		if err != nil {
			return nil, err
		}
		slog.Error("live-video-to-video container returned 422", slog.String("err", string(val)))
		return nil, errors.New("live-video-to-video container returned 422: " + string(val))
	}

	if resp.JSON500 != nil {
		val, err := json.Marshal(resp.JSON500)
		if err != nil {
			return nil, err
		}
		slog.Error("live-video-to-video container returned 500", slog.String("err", string(val)))
		return nil, errors.New("live-video-to-video container returned 500: " + resp.JSON500.Detail.Msg)
	}

	return resp.JSON200, nil
}

func (w *Worker) EnsureImageAvailable(ctx context.Context, pipeline string, modelID string) error {
	return w.manager.EnsureImageAvailable(ctx, pipeline, modelID)
}

func (w *Worker) Warm(ctx context.Context, pipeline string, modelID string, endpoint RunnerEndpoint, optimizationFlags OptimizationFlags) error {
	if endpoint.URL == "" {
		return w.manager.Warm(ctx, pipeline, modelID, optimizationFlags)
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	cfg := RunnerContainerConfig{
		Type:             External,
		Pipeline:         pipeline,
		ModelID:          modelID,
		Endpoint:         endpoint,
		containerTimeout: externalContainerTimeout,
	}
	rc, _, err := NewRunnerContainer(ctx, cfg, endpoint.URL)
	if err != nil {
		return err
	}

	name := dockerContainerName(pipeline, modelID, endpoint.URL)
	slog.Info("Starting external container", slog.String("name", name), slog.String("modelID", modelID))
	w.externalContainers[name] = rc

	return nil
}

func (w *Worker) Stop(ctx context.Context) error {
	if err := w.manager.Stop(ctx); err != nil {
		return err
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	for name := range w.externalContainers {
		delete(w.externalContainers, name)
	}

	return nil
}

// HasCapacity returns true if the worker has capacity for the given pipeline and model ID.
func (w *Worker) HasCapacity(pipeline, modelID string) bool {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Check if we have capacity for external containers.
	for _, rc := range w.externalContainers {
		if rc.Pipeline == pipeline && rc.ModelID == modelID {
			return true
		}
	}

	// Check if we have capacity for managed containers.
	return w.manager.HasCapacity(context.Background(), pipeline, modelID)
}

func (w *Worker) borrowContainer(ctx context.Context, pipeline, modelID string) (*RunnerContainer, error) {
	w.mu.Lock()

	for _, rc := range w.externalContainers {
		if rc.Pipeline == pipeline && rc.ModelID == modelID {
			w.mu.Unlock()
			// Assume external containers can handle concurrent in-flight requests.
			return rc, nil
		}
	}

	w.mu.Unlock()

	return w.manager.Borrow(ctx, pipeline, modelID)
}
