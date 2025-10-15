package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"regexp"
	"runtime/debug"
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
	"github.com/livepeer/go-livepeer/clog"
	"github.com/livepeer/go-livepeer/monitor"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

const containerModelDir = "/models"
const containerPort = "8000/tcp"
const pollingInterval = 500 * time.Millisecond
const externalContainerTimeout = 2 * time.Minute
const optFlagsContainerTimeout = 5 * time.Minute
const containerStopTimeout = 8 * time.Second
const containerRemoveTimeout = 30 * time.Second
const containerCreatorLabel = "creator"
const containerCreator = "ai-worker"
const containerCreatorIDLabel = "creator_id"

var containerTimeout = 3 * time.Minute
var containerWatchInterval = 5 * time.Second
var healthcheckTimeout = 5 * time.Second
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
	"streamdiffusion": "livepeer/ai-runner:live-app-streamdiffusion",
	// streamdiffusion-sd15 is a utility image that uses an SD1.5 model on the default config of the pipeline. Optimizes startup time.
	"streamdiffusion-sd15": "livepeer/ai-runner:live-app-streamdiffusion-sd15",
	// streamdiffusion-sdxl is a utility image that uses an SDXL model on the default config of the pipeline. Optimizes startup time.
	"streamdiffusion-sdxl": "livepeer/ai-runner:live-app-streamdiffusion-sdxl",
	// streamdiffusion-sdxl-faceid is a utility image that uses an SDXL model with a FaceID IP Adapter on the default config of the pipeline. Optimizes startup time.
	"streamdiffusion-sdxl-faceid": "livepeer/ai-runner:live-app-streamdiffusion-sdxl-faceid",
	"comfyui":                     "livepeer/ai-runner:live-app-comfyui",
	"segment_anything_2":          "livepeer/ai-runner:live-app-segment_anything_2",
	"noop":                        "livepeer/ai-runner:live-app-noop",
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

func NewDefaultDockerClient() (DockerClient, error) {
	return docker.NewClientWithOpts(docker.FromEnv, docker.WithAPIVersionNegotiation())
}

type DockerManager struct {
	gpus               []string
	modelDir           string
	overrides          ImageOverrides
	verboseLogs        bool
	containerCreatorID string

	dockerClient DockerClient
	// gpu ID => container
	gpuContainers map[string]*RunnerContainer
	// Map of idle containers. container name => container
	containers map[string]*RunnerContainer
	mu         *sync.Mutex
}

func NewDockerManager(overrides ImageOverrides, verboseLogs bool, gpus []string, modelDir string, client DockerClient, containerCreatorID string) (*DockerManager, error) {
	if client == nil {
		var err error
		client, err = NewDefaultDockerClient()
		if err != nil {
			return nil, err
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), containerTimeout)
	if _, err := RemoveExistingContainers(ctx, client, containerCreatorID); err != nil {
		cancel()
		return nil, err
	}
	cancel()

	manager := &DockerManager{
		gpus:               gpus,
		modelDir:           modelDir,
		overrides:          overrides,
		verboseLogs:        verboseLogs,
		dockerClient:       client,
		containerCreatorID: containerCreatorID,
		gpuContainers:      make(map[string]*RunnerContainer),
		containers:         make(map[string]*RunnerContainer),
		mu:                 &sync.Mutex{},
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
	defer m.monitorInUse(pipeline, modelID)

	_, err := m.createContainer(ctx, pipeline, modelID, true, optimizationFlags)
	if err != nil {
		return err
	}

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
	defer m.monitorInUse(pipeline, modelID)

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

	m.borrowContainerLocked(ctx, rc)
	return rc, nil
}

func (m *DockerManager) borrowContainerLocked(ctx context.Context, rc *RunnerContainer) {
	// Remove container and set the BorrowCtx so it is unavailable until returnContainer() is called by watchContainer()
	delete(m.containers, rc.Name)
	rc.Lock()
	rc.BorrowCtx = ctx
	rc.Unlock()
}

// returnContainer returns a container to the pool so it can be reused. It is called automatically by watchContainer
// when the BorrowCtx of the container is done or the container is IDLE.
func (m *DockerManager) returnContainer(rc *RunnerContainer) {
	m.mu.Lock()
	defer m.mu.Unlock()
	defer m.monitorInUse(rc.Pipeline, rc.ModelID)

	rc.Lock()
	rc.BorrowCtx = nil
	rc.Unlock()

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
	if pipeline != "live-video-to-video" && !m.isImageAvailable(ctx, pipeline, modelID) {
		return false
	}

	// Check for available GPU to allocate for a new container for the requested model.
	_, err := m.allocGPU(ctx)
	return err == nil
}

func (m *DockerManager) Version() []Version {
	var version []Version
	for _, rc := range m.gpuContainers {
		if rc.Version != nil {
			version = append(version, *rc.Version)
		} else {
			version = append(version, Version{})
		}
	}
	return version
}

func (m *DockerManager) HardwareInformation() []HardwareInformation {
	var hardware []HardwareInformation
	for _, rc := range m.gpuContainers {
		if rc.Hardware != nil {
			hardware = append(hardware, *rc.Hardware)
		} else {
			hardware = append(hardware, HardwareInformation{})
		}
	}
	return hardware
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

	containerHostPort := containerHostPorts[pipeline][:2] + portOffset(gpu)
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
	if m.verboseLogs {
		envVars = append(envVars, "VERBOSE_LOGGING=1")
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
			containerCreatorLabel:   containerCreator,
			containerCreatorIDLabel: m.containerCreatorID,
		},
	}

	gpuOpts := opts.GpuOpts{}
	if !isEmulatedGPU(hwGPU(gpu)) {
		gpuOpts.Set("device=" + hwGPU(gpu))
	}

	restartPolicy := container.RestartPolicy{
		Name:              container.RestartPolicyOnFailure,
		MaximumRetryCount: 3,
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
					HostIP:   "127.0.0.1",
					HostPort: containerHostPort,
				},
			},
		},
		RestartPolicy: restartPolicy,
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
		ID:                resp.ID,
		GPU:               gpu,
		KeepWarm:          keepWarm,
		OptimizationFlags: optimizationFlags,
		containerTimeout:  runnerContainerTimeout,
	}

	rc, isLoading, err := NewRunnerContainer(ctx, cfg, containerName)
	if err != nil {
		dockerRemoveContainer(m.dockerClient, resp.ID)
		return nil, err
	}

	m.containers[containerName] = rc
	m.gpuContainers[gpu] = rc

	if keepWarm && isLoading {
		// If the container is only being warmed up, we only want to add it to the pool when it is past the loading state.
		// This will be done by the watchContainer() routine once the container returns IDLE on the healthcheck.
		slog.Info("Warm container started on loading state, removing from pool on startup", slog.String("container", rc.Name))
		m.borrowContainerLocked(context.Background(), rc)
	}
	go m.watchContainer(rc)

	return rc, nil
}

func hwGPU(gpu string) string {
	if !strings.HasPrefix(gpu, "colocated-") {
		return gpu
	}

	re := regexp.MustCompile(`colocated-\d+-(.*)`)
	matches := re.FindStringSubmatch(gpu)
	if len(matches) > 1 {
		return matches[1]
	}

	clog.Warningf(context.Background(), "Invalid GPU name: %s", gpu)
	return gpu
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
	for gpu, rc := range m.gpuContainers {
		// If the container exists in this map then it is idle and if it not marked as keep warm we remove it
		_, isIdle := m.containers[rc.Name]
		if isIdle && !rc.KeepWarm {
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
	defer m.monitorInUse(rc.Pipeline, rc.ModelID)
	delete(m.gpuContainers, rc.GPU)
	delete(m.containers, rc.Name)
	return nil
}

// watchContainer monitors a container's running state and automatically cleans
// up the internal state when the container stops. It will also monitor the
// borrowCtx to return the container to the pool when it is done.
func (m *DockerManager) watchContainer(rc *RunnerContainer) {
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
	var loadingStartTime time.Time
	failures := 0
	for {
		if failures >= maxHealthCheckFailures {
			slog.Error("Container health check failed too many times", slog.String("container", rc.Name))
			m.destroyContainer(rc, false)
			if rc.KeepWarm {
				slog.Info("Container was kept warm, restarting", slog.String("container", rc.Name))
				err := m.Warm(context.Background(), rc.Pipeline, rc.ModelID, rc.OptimizationFlags)
				if err != nil {
					slog.Error("Error restarting warm container", slog.String("container", rc.Name), slog.String("error", err.Error()))
				}
			}
			return
		}

		borrowCtx := func() context.Context {
			rc.RLock()
			defer rc.RUnlock()
			if rc.BorrowCtx == nil {
				return nil
			}
			return rc.BorrowCtx
		}

		var borrowDone <-chan struct{}
		// The BorrowCtx is set when the container has been borrowed for a request/stream. If it is not set (nil) it means
		// that it's not currently borrowed, so we don't need to wait for it to be done (hence keeping the nil channel).
		if bc := borrowCtx(); bc != nil {
			borrowDone = bc.Done()
		}

		select {
		case <-borrowDone:
			m.returnContainer(rc)
			continue
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), healthcheckTimeout)
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

			isBorrowed := borrowCtx() != nil
			status := health.JSON200.Status
			switch status {
			case IDLE:
				if isBorrowed {
					slog.Info("Container is idle, returning to pool", slog.String("container", rc.Name))
					m.returnContainer(rc)
					continue
				}
				fallthrough
			case OK:
				failures, loadingStartTime = 0, time.Time{}
				continue
			case LOADING:
				if loadingStartTime.IsZero() {
					failures, loadingStartTime = 0, time.Now()

					if !isBorrowed {
						slog.Info("Container is loading, removing from pool", slog.String("container", rc.Name))

						m.mu.Lock()
						m.borrowContainerLocked(context.Background(), rc)
						m.mu.Unlock()
					}
				}
				if loadingTime := time.Since(loadingStartTime); loadingTime > containerTimeout {
					failures++
					slog.Error("Container is loading for too long", slog.String("container", rc.Name), slog.Duration("duration", loadingTime))
				}
				continue
			case ERROR:
				failures = maxHealthCheckFailures
				slog.Error("Container returned ERROR state, restarting immediately",
					slog.String("container", rc.Name),
					slog.String("status", string(status)))
			default:
				slog.Error("Unknown container status",
					slog.String("container", rc.Name),
					slog.String("status", string(status)))
			}
		}
	}
}

func RemoveExistingContainers(ctx context.Context, client DockerClient, containerCreatorID string) (int, error) {
	if client == nil {
		var err error
		client, err = NewDefaultDockerClient()
		if err != nil {
			return 0, err
		}
	}

	filters := filters.NewArgs(filters.Arg("label", containerCreatorLabel+"="+containerCreator))
	containers, err := client.ContainerList(ctx, container.ListOptions{All: true, Filters: filters})
	if err != nil {
		return 0, fmt.Errorf("failed to list containers: %w", err)
	}

	removed := 0
	for _, c := range containers {
		creatorID, hasCreatorID := c.Labels[containerCreatorIDLabel]
		// We also remove containers without creator-id label as a migration from previous versions that didn't have it.
		shouldRemove := !hasCreatorID || creatorID == containerCreatorID
		if !shouldRemove {
			continue
		}
		slog.Info("Removing existing managed container", slog.String("name", c.Names[0]))
		if err := dockerRemoveContainer(client, c.ID); err != nil {
			return removed, err
		}
		removed++
	}

	return removed, nil
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

	timeoutSec := int(containerStopTimeout.Seconds())
	err := client.ContainerStop(ctx, containerID, container.StopOptions{Timeout: &timeoutSec})
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

			// If the container is running, we're done.
			if json.State.Running {
				break tickerLoop
			}

			// Fail fast on states that won't become running after startup.
			if json.State != nil {
				status := strings.ToLower(json.State.Status)
				// Consider exited/dead as terminal. "removing" will surface via
				// inspect error or transition to exited/dead shortly.
				if status == "exited" || status == "dead" {
					return fmt.Errorf("container entered terminal state before running: %s (exitCode=%d)", json.State.Status, json.State.ExitCode)
				}
				if !json.State.Restarting && json.State.ExitCode != 0 {
					return fmt.Errorf("container exited before running (status=%s, exitCode=%d)", json.State.Status, json.State.ExitCode)
				}
				if !json.State.Restarting && json.State.Error != "" {
					return fmt.Errorf("container error before running: %s", json.State.Error)
				}
			}
		}
	}

	return nil
}

type Capacity struct {
	ContainersInUse int
	ContainersIdle  int
}

// GetCapacity returns the current number of containers in use and idle
// It currently only supports a setup of a single model with the number of initial warm containers equalling max capacity.
// For example for Live AI we use this setup, we configure the number of warm containers to equal the max capacity we want
// to accept, all with the comfyui model.
func (m *DockerManager) GetCapacity(pipeline, modelID string) (Capacity, int) {
	if pipeline == "" && modelID == "" {
		return Capacity{
			ContainersInUse: len(m.gpuContainers) - len(m.containers),
			ContainersIdle:  len(m.containers),
		}, len(m.gpus) - len(m.gpuContainers)
	}
	gpuContainers := 0
	for _, container := range m.gpuContainers {
		if container.RunnerContainerConfig.Pipeline == pipeline && container.RunnerContainerConfig.ModelID == modelID {
			gpuContainers += 1
		}
	}
	containers := 0
	for _, container := range m.containers {
		if container.RunnerContainerConfig.Pipeline == pipeline && container.RunnerContainerConfig.ModelID == modelID {
			containers += 1
		}
	}
	return Capacity{
			ContainersInUse: gpuContainers - containers,
			ContainersIdle:  containers,
		},
		len(m.gpus) - gpuContainers

}

func (m *DockerManager) monitorInUse(pipeline string, modelID string) {
	if monitor.Enabled {
		capacity, gpusIdle := m.GetCapacity(pipeline, modelID)
		monitor.AIContainersInUse(capacity.ContainersInUse, pipeline, modelID)
		monitor.AIContainersIdle(capacity.ContainersIdle, pipeline, modelID, "")
		monitor.AIGPUsIdle(gpusIdle, pipeline, modelID) // Indicates a misconfiguration so we should alert on this
	}
}

func portOffset(gpu string) string {
	// last 2 digits of the port number
	res := "00"

	// If colocated, update the first digit of the port
	if strings.Contains(gpu, "colocated-") {
		re := regexp.MustCompile(`colocated-(\d+)-(.*)`)
		matches := re.FindStringSubmatch(gpu)
		if len(matches) > 1 {
			res = matches[1] + res[1:]
			gpu = matches[2]
		} else {
			clog.Warningf(context.Background(), "Invalid GPU name: %s", gpu)
		}
	}

	// If emulated, remove the prefix
	if isEmulatedGPU(gpu) {
		gpu = strings.Replace(gpu, "emulated-", "", 1)
	}

	// Update second digit with the gpu number
	res = res[:1] + gpu

	return res
}

func isEmulatedGPU(gpu string) bool {
	return strings.HasPrefix(gpu, "emulated-")
}
