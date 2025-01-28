package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/docker/cli/opts"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	docker "github.com/docker/docker/client"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/docker/go-connections/nat"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

const containerModelDir = "/models"
const containerPort = "8000/tcp"
const pollingInterval = 500 * time.Millisecond
const containerTimeout = 2 * time.Minute
const externalContainerTimeout = 2 * time.Minute
const optFlagsContainerTimeout = 5 * time.Minute
const containerRemoveTimeout = 30 * time.Second
const containerCreatorLabel = "creator"
const containerCreator = "ai-worker"

var containerWatchInterval = 5 * time.Second
var pipelineStartGracePeriod = 60 * time.Second
var maxHealthCheckFailures = 2

// This only works right now on a single GPU because if there is another container
// using the GPU we stop it so we don't have to worry about having enough ports
var containerHostPorts = map[string]string{
	"text-to-image":       "8000",
	"image-to-image":      "8100",
	"image-to-video":      "8200",
	"upscale":             "8300",
	"audio-to-text":       "8400",
	"llm":                 "8500",
	"segment-anything-2":  "8600",
	"image-to-text":       "8700",
	"text-to-speech":      "8800",
	"live-video-to-video": "8900",
}

// Default pipeline container image mapping to use if no overrides are provided.
var defaultBaseImage = "livepeer/ai-runner:latest"
var pipelineToImage = map[string]string{
	"segment-anything-2": "livepeer/ai-runner:segment-anything-2",
	"text-to-speech":     "livepeer/ai-runner:text-to-speech",
	"audio-to-text":      "livepeer/ai-runner:audio-to-text",
	"llm":                "livepeer/ai-runner:llm",
}
var livePipelineToImage = map[string]string{
	"streamdiffusion":    "livepeer/ai-runner:live-app-streamdiffusion",
	"comfyui":            "livepeer/ai-runner:live-app-comfyui",
	"segment_anything_2": "livepeer/ai-runner:live-app-segment_anything_2",
	"noop":               "livepeer/ai-runner:live-app-noop",
}

type ImageOverrides struct {
	Default string            `json:"default"`
	Batch   map[string]string `json:"batch"`
	Live    map[string]string `json:"live"`
}

// DockerClient is an interface for the Docker client, allowing for mocking in tests.
// NOTE: ensure any docker.Client methods used in this package are added.
type DockerClient interface {
	ContainerCreate(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *ocispec.Platform, containerName string) (container.CreateResponse, error)
	ContainerInspect(ctx context.Context, containerID string) (types.ContainerJSON, error)
	ContainerList(ctx context.Context, options container.ListOptions) ([]types.Container, error)
	ContainerRemove(ctx context.Context, containerID string, options container.RemoveOptions) error
	ContainerStart(ctx context.Context, containerID string, options container.StartOptions) error
	ContainerStop(ctx context.Context, containerID string, options container.StopOptions) error
	ImageInspectWithRaw(ctx context.Context, imageID string) (types.ImageInspect, []byte, error)
	ImagePull(ctx context.Context, ref string, options image.PullOptions) (io.ReadCloser, error)
}

// Compile-time assertion to ensure docker.Client implements DockerClient.
var _ DockerClient = (*docker.Client)(nil)

// Create global references to functions to allow for mocking in tests.
var dockerWaitUntilRunningFunc = dockerWaitUntilRunning

type DockerManager struct {
	gpus      []string
	modelDir  string
	overrides ImageOverrides

	dockerClient DockerClient
	// gpu ID => container name
	gpuContainers map[string]string
	// container name => container
	containers map[string]*RunnerContainer
	mu         *sync.Mutex
}

func NewDockerManager(overrides ImageOverrides, gpus []string, modelDir string, client DockerClient) (*DockerManager, error) {
	ctx, cancel := context.WithTimeout(context.Background(), containerTimeout)
	if err := removeExistingContainers(ctx, client); err != nil {
		cancel()
		return nil, err
	}
	cancel()

	manager := &DockerManager{
		gpus:          gpus,
		modelDir:      modelDir,
		overrides:     overrides,
		dockerClient:  client,
		gpuContainers: make(map[string]string),
		containers:    make(map[string]*RunnerContainer),
		mu:            &sync.Mutex{},
	}

	return manager, nil
}

// EnsureImageAvailable ensures the container image is available locally for the given pipeline and model ID.
func (m *DockerManager) EnsureImageAvailable(ctx context.Context, pipeline string, modelID string) error {
	imageName, err := m.getContainerImageName(pipeline, modelID)
	if err != nil {
		return err
	}

	// Pull the image if it is not available locally.
	if !m.isImageAvailable(ctx, pipeline, modelID) {
		slog.Info(fmt.Sprintf("Pulling image for pipeline %s and modelID %s: %s", pipeline, modelID, imageName))
		err = m.pullImage(ctx, imageName)
		if err != nil {
			return err
		}
	}

	return nil
}

func (m *DockerManager) Warm(ctx context.Context, pipeline string, modelID string, optimizationFlags OptimizationFlags) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	rc, err := m.createContainer(ctx, pipeline, modelID, true, optimizationFlags)
	if err != nil {
		return err
	}

	// Watch with a background context since we're not borrowing the container.
	go m.watchContainer(rc, context.Background())

	return nil
}

func (m *DockerManager) Stop(ctx context.Context) error {
	var stopContainerWg sync.WaitGroup
	for _, rc := range m.containers {
		stopContainerWg.Add(1)
		go func(container *RunnerContainer) {
			defer stopContainerWg.Done()
			m.destroyContainer(container, false)
		}(rc)
	}

	stopContainerWg.Wait()
	return nil
}

func (m *DockerManager) Borrow(ctx context.Context, pipeline, modelID string) (*RunnerContainer, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var rc *RunnerContainer
	var err error

	for _, runner := range m.containers {
		if runner.Pipeline == pipeline && runner.ModelID == modelID {
			rc = runner
			break
		}
	}

	// The container does not exist so try to create it
	if rc == nil {
		// TODO: Optimization flags for dynamically loaded (borrowed) containers are not currently supported due to startup delays.
		rc, err = m.createContainer(ctx, pipeline, modelID, false, map[string]EnvValue{})
		if err != nil {
			return nil, err
		}
	}

	// Remove container so it is unavailable until Return() is called
	delete(m.containers, rc.Name)
	// watch container to return when request completed
	go m.watchContainer(rc, ctx)

	return rc, nil
}

// returnContainer returns a container to the pool so it can be reused. It is called automatically by watchContainer
// when the context used to borrow the container is done.
func (m *DockerManager) returnContainer(rc *RunnerContainer) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.containers[rc.Name] = rc
}

// getContainerImageName returns the image name for the given pipeline and model ID.
// Returns an error if the image is not found for "live-video-to-video".
func (m *DockerManager) getContainerImageName(pipeline, modelID string) (string, error) {
	if pipeline == "live-video-to-video" {
		// We currently use the model ID as the live pipeline name for legacy reasons.
		if image, ok := m.overrides.Live[modelID]; ok {
			return image, nil
		} else if image, ok := livePipelineToImage[modelID]; ok {
			return image, nil
		}
		return "", fmt.Errorf("no container image found for live pipeline %s", modelID)
	}

	if image, ok := m.overrides.Batch[pipeline]; ok {
		return image, nil
	} else if image, ok := pipelineToImage[pipeline]; ok {
		return image, nil
	}

	if m.overrides.Default != "" {
		return m.overrides.Default, nil
	}
	return defaultBaseImage, nil
}

// HasCapacity checks if an unused managed container exists or if a GPU is available for a new container.
func (m *DockerManager) HasCapacity(ctx context.Context, pipeline, modelID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if unused managed container exists for the requested model.
	for _, rc := range m.containers {
		if rc.Pipeline == pipeline && rc.ModelID == modelID {
			return true
		}
	}

	// TODO: This can be removed if we optimize the selection algorithm.
	// Currently, using CreateContainer errors only can cause orchestrator reselection.
	if !m.isImageAvailable(ctx, pipeline, modelID) {
		return false
	}

	// Check for available GPU to allocate for a new container for the requested model.
	_, err := m.allocGPU(ctx)
	return err == nil
}

// isImageAvailable checks if the specified image is available locally.
func (m *DockerManager) isImageAvailable(ctx context.Context, pipeline string, modelID string) bool {
	imageName, err := m.getContainerImageName(pipeline, modelID)
	if err != nil {
		slog.Error(err.Error())
		return false
	}

	_, _, err = m.dockerClient.ImageInspectWithRaw(ctx, imageName)
	if err != nil {
		slog.Error(fmt.Sprintf("Image for pipeline %s and modelID %s is not available locally: %s", pipeline, modelID, imageName))
	}
	return err == nil
}

// pullImage pulls the specified image from the registry.
func (m *DockerManager) pullImage(ctx context.Context, imageName string) error {
	reader, err := m.dockerClient.ImagePull(ctx, imageName, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("failed to pull image: %w", err)
	}
	defer reader.Close()

	// Display progress messages from ImagePull reader.
	decoder := json.NewDecoder(reader)
	for {
		var progress jsonmessage.JSONMessage
		if err := decoder.Decode(&progress); err == io.EOF {
			break
		} else if err != nil {
			return fmt.Errorf("error decoding progress message: %w", err)
		}
		if progress.Status != "" && progress.Progress != nil {
			slog.Info(fmt.Sprintf("%s: %s", progress.Status, progress.Progress.String()))
		}
	}

	return nil
}

func (m *DockerManager) createContainer(ctx context.Context, pipeline string, modelID string, keepWarm bool, optimizationFlags OptimizationFlags) (*RunnerContainer, error) {
	gpu, err := m.allocGPU(ctx)
	if err != nil {
		return nil, err
	}

	// NOTE: We currently allow only one container per GPU for each pipeline.
	containerHostPort := containerHostPorts[pipeline][:3] + portOffset(gpu)
	containerName := dockerContainerName(pipeline, modelID, containerHostPort)
	containerImage, err := m.getContainerImageName(pipeline, modelID)
	if err != nil {
		return nil, err
	}

	slog.Info("Starting managed container", slog.String("gpu", gpu), slog.String("name", containerName), slog.String("modelID", modelID), slog.String("containerImage", containerImage))

	// Add optimization flags as environment variables.
	envVars := []string{
		"PIPELINE=" + pipeline,
		"MODEL_ID=" + modelID,
	}
	for key, value := range optimizationFlags {
		envVars = append(envVars, key+"="+value.String())
	}

	containerConfig := &container.Config{
		Image: containerImage,
		Env:   envVars,
		Volumes: map[string]struct{}{
			containerModelDir: {},
		},
		ExposedPorts: nat.PortSet{
			containerPort: struct{}{},
		},
		Labels: map[string]string{
			containerCreatorLabel: containerCreator,
		},
	}

	gpuOpts := opts.GpuOpts{}
	if !isEmulatedGPU(gpu) {
		gpuOpts.Set("device=" + gpu)
	}

	hostConfig := &container.HostConfig{
		Resources: container.Resources{
			DeviceRequests: gpuOpts.Value(),
		},
		Mounts: []mount.Mount{
			{
				Type:   mount.TypeBind,
				Source: m.modelDir,
				Target: containerModelDir,
			},
		},
		PortBindings: nat.PortMap{
			containerPort: []nat.PortBinding{
				{
					HostIP:   "0.0.0.0",
					HostPort: containerHostPort,
				},
			},
		},
		AutoRemove: true,
	}

	resp, err := m.dockerClient.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, containerName)
	if err != nil {
		return nil, err
	}

	cctx, cancel := context.WithTimeout(ctx, containerTimeout)
	if err := m.dockerClient.ContainerStart(cctx, resp.ID, container.StartOptions{}); err != nil {
		cancel()
		dockerRemoveContainer(m.dockerClient, resp.ID)
		return nil, err
	}
	cancel()

	cctx, cancel = context.WithTimeout(ctx, containerTimeout)
	if err := dockerWaitUntilRunningFunc(cctx, m.dockerClient, resp.ID, pollingInterval); err != nil {
		cancel()
		dockerRemoveContainer(m.dockerClient, resp.ID)
		return nil, err
	}
	cancel()

	// Extend runner container timeout when optimization flags are used, as these
	// pipelines may require more startup time.
	runnerContainerTimeout := containerTimeout
	if len(optimizationFlags) > 0 {
		runnerContainerTimeout = optFlagsContainerTimeout
	}

	cfg := RunnerContainerConfig{
		Type:     Managed,
		Pipeline: pipeline,
		ModelID:  modelID,
		Endpoint: RunnerEndpoint{
			URL: "http://localhost:" + containerHostPort,
		},
		ID:               resp.ID,
		GPU:              gpu,
		KeepWarm:         keepWarm,
		containerTimeout: runnerContainerTimeout,
	}

	rc, err := NewRunnerContainer(ctx, cfg, containerName)
	if err != nil {
		dockerRemoveContainer(m.dockerClient, resp.ID)
		return nil, err
	}

	m.containers[containerName] = rc
	m.gpuContainers[gpu] = containerName

	return rc, nil
}

func (m *DockerManager) allocGPU(ctx context.Context) (string, error) {
	// Is there a GPU available?
	for _, gpu := range m.gpus {
		_, ok := m.gpuContainers[gpu]
		if !ok {
			return gpu, nil
		}
	}

	// Is there a GPU with an idle container?
	for _, gpu := range m.gpus {
		containerName := m.gpuContainers[gpu]
		// If the container exists in this map then it is idle and if it not marked as keep warm we remove it
		rc, ok := m.containers[containerName]
		if ok && !rc.KeepWarm {
			if err := m.destroyContainer(rc, true); err != nil {
				return "", err
			}
			return gpu, nil
		}
	}

	return "", errors.New("insufficient capacity")
}

// destroyContainer stops the container on docker and removes it from the
// internal state. If locked is true then the mutex is not re-locked, otherwise
// it is done automatically only when updating the internal state.
func (m *DockerManager) destroyContainer(rc *RunnerContainer, locked bool) error {
	slog.Info("Removing managed container",
		slog.String("gpu", rc.GPU),
		slog.String("name", rc.Name),
		slog.String("modelID", rc.ModelID))

	if err := dockerRemoveContainer(m.dockerClient, rc.ID); err != nil {
		slog.Error("Error removing managed container",
			slog.String("gpu", rc.GPU),
			slog.String("name", rc.Name),
			slog.String("modelID", rc.ModelID),
			slog.String("error", err.Error()))
		return fmt.Errorf("failed to remove container %s: %w", rc.Name, err)
	}

	if !locked {
		m.mu.Lock()
		defer m.mu.Unlock()
	}
	delete(m.gpuContainers, rc.GPU)
	delete(m.containers, rc.Name)
	return nil
}

// watchContainer monitors a container's running state and automatically cleans
// up the internal state when the container stops. It will also monitor the
// borrowCtx to return the container to the pool when it is done.
func (m *DockerManager) watchContainer(rc *RunnerContainer, borrowCtx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("Panic in container watch routine",
				slog.String("container", rc.Name),
				slog.Any("panic", r),
				slog.String("stack", string(debug.Stack())))
		}
	}()

	ticker := time.NewTicker(containerWatchInterval)
	defer ticker.Stop()

	slog.Info("Watching container", slog.String("container", rc.Name))
	failures := 0
	startTime := time.Now()
	for {
		if failures >= maxHealthCheckFailures && time.Since(startTime) > pipelineStartGracePeriod {
			slog.Error("Container health check failed too many times", slog.String("container", rc.Name))
			m.destroyContainer(rc, false)
			return
		}

		select {
		case <-borrowCtx.Done():
			m.returnContainer(rc)
			return
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), containerWatchInterval)
			health, err := rc.Client.HealthWithResponse(ctx)
			cancel()

			if err != nil {
				failures++
				slog.Error("Error getting health for runner",
					slog.String("container", rc.Name),
					slog.String("error", err.Error()))
				continue
			} else if health.StatusCode() != 200 {
				failures++
				slog.Error("Container health check failed with HTTP status code",
					slog.String("container", rc.Name),
					slog.Int("status_code", health.StatusCode()),
					slog.String("body", string(health.Body)))
				continue
			}
			slog.Debug("Health check response",
				slog.String("status", health.Status()),
				slog.Any("JSON200", health.JSON200),
				slog.String("body", string(health.Body)))

			status := health.JSON200.Status
			switch status {
			case IDLE:
				if time.Since(startTime) > pipelineStartGracePeriod {
					slog.Info("Container is idle, returning to pool", slog.String("container", rc.Name))
					m.returnContainer(rc)
					return
				}
				fallthrough
			case OK:
				failures = 0
				continue
			default:
				failures++
				slog.Error("Container not healthy",
					slog.String("container", rc.Name),
					slog.String("status", string(status)),
					slog.String("failures", strconv.Itoa(failures)))
			}
		}
	}
}

func removeExistingContainers(ctx context.Context, client DockerClient) error {
	filters := filters.NewArgs(filters.Arg("label", containerCreatorLabel+"="+containerCreator))
	containers, err := client.ContainerList(ctx, container.ListOptions{All: true, Filters: filters})
	if err != nil {
		return err
	}

	for _, c := range containers {
		slog.Info("Removing existing managed container", slog.String("name", c.Names[0]))
		if err := dockerRemoveContainer(client, c.ID); err != nil {
			return err
		}
	}

	return nil
}

// dockerContainerName generates a unique container name based on the pipeline, model ID, and an optional suffix.
func dockerContainerName(pipeline string, modelID string, suffix ...string) string {
	sanitizedModelID := strings.NewReplacer("/", "-", "_", "-").Replace(modelID)
	if len(suffix) > 0 {
		return fmt.Sprintf("%s_%s_%s", pipeline, sanitizedModelID, suffix[0])
	}
	return fmt.Sprintf("%s_%s", pipeline, sanitizedModelID)
}

func dockerRemoveContainer(client DockerClient, containerID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), containerRemoveTimeout)
	defer cancel()

	err := client.ContainerStop(ctx, containerID, container.StopOptions{})
	// Ignore "not found" or "already stopped" errors
	if err != nil && !docker.IsErrNotFound(err) && !errdefs.IsNotModified(err) {
		return err
	}

	err = client.ContainerRemove(ctx, containerID, container.RemoveOptions{})
	if err == nil || docker.IsErrNotFound(err) {
		return nil
	} else if err != nil && !strings.Contains(err.Error(), "is already in progress") {
		return err
	}
	// The container is being removed asynchronously, wait until it is actually gone
	ticker := time.NewTicker(pollingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timed out waiting for container removal to complete")
		case <-ticker.C:
			_, err := client.ContainerInspect(ctx, containerID)
			if docker.IsErrNotFound(err) {
				return nil
			}
		}
	}
}

func dockerWaitUntilRunning(ctx context.Context, client DockerClient, containerID string, pollingInterval time.Duration) error {
	ticker := time.NewTicker(pollingInterval)
	defer ticker.Stop()

tickerLoop:
	for range ticker.C {
		select {
		case <-ctx.Done():
			return errors.New("timed out waiting for managed container")
		default:
			json, err := client.ContainerInspect(ctx, containerID)
			if err != nil {
				return err
			}

			if json.State.Running {
				break tickerLoop
			}
		}
	}

	return nil
}

func portOffset(gpu string) string {
	if isEmulatedGPU(gpu) {
		return strings.Replace(gpu, "emulated-", "", 1)
	}
	return gpu
}

func isEmulatedGPU(gpu string) bool {
	return strings.HasPrefix(gpu, "emulated-")
}
