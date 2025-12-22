package worker

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var stopTimeout = 8
var expectedContainerStopOptions = container.StopOptions{Timeout: &stopTimeout}

type MockDockerClient struct {
	mock.Mock
}

func (m *MockDockerClient) ImagePull(ctx context.Context, ref string, options image.PullOptions) (io.ReadCloser, error) {
	args := m.Called(ctx, ref, options)
	return args.Get(0).(io.ReadCloser), args.Error(1)
}

func (m *MockDockerClient) ImageInspectWithRaw(ctx context.Context, imageID string) (types.ImageInspect, []byte, error) {
	args := m.Called(ctx, imageID)
	return args.Get(0).(types.ImageInspect), args.Get(1).([]byte), args.Error(2)
}

func (m *MockDockerClient) ContainerCreate(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *ocispec.Platform, containerName string) (container.CreateResponse, error) {
	args := m.Called(ctx, config, hostConfig, networkingConfig, platform, containerName)
	return args.Get(0).(container.CreateResponse), args.Error(1)
}

func (m *MockDockerClient) ContainerStart(ctx context.Context, containerID string, options container.StartOptions) error {
	args := m.Called(ctx, containerID, options)
	return args.Error(0)
}

func (m *MockDockerClient) ContainerInspect(ctx context.Context, containerID string) (types.ContainerJSON, error) {
	args := m.Called(ctx, containerID)
	return args.Get(0).(types.ContainerJSON), args.Error(1)
}

func (m *MockDockerClient) ContainerList(ctx context.Context, options container.ListOptions) ([]types.Container, error) {
	args := m.Called(ctx, options)
	return args.Get(0).([]types.Container), args.Error(1)
}

func (m *MockDockerClient) ContainerStop(ctx context.Context, containerID string, options container.StopOptions) error {
	args := m.Called(ctx, containerID, options)
	return args.Error(0)
}

func (m *MockDockerClient) ContainerRemove(ctx context.Context, containerID string, options container.RemoveOptions) error {
	args := m.Called(ctx, containerID, options)
	return args.Error(0)
}

type MockServer struct {
	mock.Mock
	*httptest.Server
}

func (m *MockServer) ServeHTTP(method, path, reqBody string) (status int, contentType string, respBody string) {
	args := m.Called(method, path, reqBody)
	return args.Int(0), args.String(1), args.String(2)
}

func NewMockServer() *MockServer {
	mockServer := new(MockServer)
	mockServer.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method, path := r.Method, r.URL.Path
		reqBody, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		status, contentType, respBody := mockServer.ServeHTTP(method, path, string(reqBody))

		w.Header().Set("Content-Type", contentType)
		w.WriteHeader(status)
		w.Write([]byte(respBody))
	}))
	return mockServer
}

// createDockerManager creates a DockerManager with a mock DockerClient.
func createDockerManager(mockDockerClient *MockDockerClient) *DockerManager {
	ctx, cancel := context.WithCancel(context.Background())
	return &DockerManager{
		gpus:               []string{"gpu0"},
		modelDir:           "/models",
		overrides:          ImageOverrides{Default: "default-image"},
		dockerClient:       mockDockerClient,
		containerCreatorID: "instance-1",
		gpuContainers:      make(map[string]*RunnerContainer),
		containers:         make(map[string]*RunnerContainer),
		mu:                 &sync.Mutex{},
		ctx:                ctx,
		stop:               cancel,
	}
}

func TestNewDockerManager(t *testing.T) {
	mockDockerClient := new(MockDockerClient)

	createAndVerifyManager := func() *DockerManager {
		manager, err := NewDockerManager(ImageOverrides{Default: "default-image"}, false, []string{"gpu0"}, "/models", mockDockerClient, "instance-1")
		require.NoError(t, err)
		require.NotNil(t, manager)
		require.Equal(t, "default-image", manager.overrides.Default)
		require.Equal(t, []string{"gpu0"}, manager.gpus)
		require.Equal(t, "/models", manager.modelDir)
		require.Equal(t, mockDockerClient, manager.dockerClient)
		return manager
	}

	t.Run("NoExistingContainers", func(t *testing.T) {
		mockDockerClient.On("ContainerList", mock.Anything, mock.Anything).Return([]types.Container{}, nil).Once()
		createAndVerifyManager()
		mockDockerClient.AssertNotCalled(t, "ContainerStop", mock.Anything, mock.Anything, mock.Anything)
		mockDockerClient.AssertNotCalled(t, "ContainerRemove", mock.Anything, mock.Anything, mock.Anything)
		mockDockerClient.AssertExpectations(t)
	})

	t.Run("ExistingContainers", func(t *testing.T) {
		// Mock client methods to simulate the removal of existing containers.
		existingContainers := []types.Container{
			{ID: "container1", Names: []string{"/container1"}, Labels: map[string]string{containerCreatorLabel: containerCreator}},
			{ID: "container2", Names: []string{"/container2"}, Labels: map[string]string{containerCreatorLabel: containerCreator}},
		}
		mockDockerClient.On("ContainerList", mock.Anything, mock.Anything).Return(existingContainers, nil)
		mockDockerClient.On("ContainerStop", mock.Anything, "container1", expectedContainerStopOptions).Return(nil)
		mockDockerClient.On("ContainerStop", mock.Anything, "container2", expectedContainerStopOptions).Return(nil)
		mockDockerClient.On("ContainerRemove", mock.Anything, "container1", mock.Anything).Return(nil)
		mockDockerClient.On("ContainerRemove", mock.Anything, "container2", mock.Anything).Return(nil)

		// Verify that existing containers were stopped and removed.
		createAndVerifyManager()
		mockDockerClient.AssertCalled(t, "ContainerStop", mock.Anything, "container1", expectedContainerStopOptions)
		mockDockerClient.AssertCalled(t, "ContainerStop", mock.Anything, "container2", expectedContainerStopOptions)
		mockDockerClient.AssertCalled(t, "ContainerRemove", mock.Anything, "container1", mock.Anything)
		mockDockerClient.AssertCalled(t, "ContainerRemove", mock.Anything, "container2", mock.Anything)
		mockDockerClient.AssertExpectations(t)
	})
}

func TestDockerManager_EnsureImageAvailable(t *testing.T) {
	mockDockerClient := new(MockDockerClient)
	dockerManager := createDockerManager(mockDockerClient)

	ctx := context.Background()
	pipeline := "text-to-image"
	modelID := "test-model"

	tests := []struct {
		name         string
		setup        func(*DockerManager, *MockDockerClient)
		expectedPull bool
	}{
		{
			name: "ImageAvailable",
			setup: func(dockerManager *DockerManager, mockDockerClient *MockDockerClient) {
				// Mock client methods to simulate the image being available locally.
				mockDockerClient.On("ImageInspectWithRaw", mock.Anything, "default-image").Return(types.ImageInspect{}, []byte{}, nil).Once()
			},
			expectedPull: false,
		},
		{
			name: "ImageNotAvailable",
			setup: func(dockerManager *DockerManager, mockDockerClient *MockDockerClient) {
				// Mock client methods to simulate the image not being available locally.
				mockDockerClient.On("ImageInspectWithRaw", mock.Anything, "default-image").Return(types.ImageInspect{}, []byte{}, fmt.Errorf("image not found")).Once()
			},
			expectedPull: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup(dockerManager, mockDockerClient)

			if tt.expectedPull {
				mockDockerClient.On("ImagePull", mock.Anything, "default-image", mock.Anything).Return(io.NopCloser(strings.NewReader("")), nil).Once()
			}

			err := dockerManager.EnsureImageAvailable(ctx, pipeline, modelID)
			require.NoError(t, err)

			mockDockerClient.AssertExpectations(t)
		})
	}
}

func TestDockerManager_Warm(t *testing.T) {
	mockDockerClient := new(MockDockerClient)
	dockerManager := createDockerManager(mockDockerClient)

	ctx := context.Background()
	pipeline := "text-to-image"
	modelID := "test-model"
	containerID := "container1"
	optimizationFlags := OptimizationFlags{}

	// Mock nested functions.
	defer updateDuringTest(&dockerWaitUntilRunningFunc, func(ctx context.Context, client DockerClient, containerID string, pollingInterval time.Duration) error {
		return nil
	})()
	defer updateDuringTest(&runnerWaitUntilReadyFunc, func(ctx context.Context, client *ClientWithResponses, pollingInterval time.Duration) (bool, error) {
		return false, nil
	})()

	mockDockerClient.On("ContainerCreate", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(container.CreateResponse{ID: containerID}, nil)
	mockDockerClient.On("ContainerStart", mock.Anything, containerID, mock.Anything).Return(nil)
	err := dockerManager.Warm(ctx, pipeline, modelID, optimizationFlags)
	require.NoError(t, err)
	mockDockerClient.AssertExpectations(t)
}

func TestDockerManager_Stop(t *testing.T) {
	MockDockerClient := new(MockDockerClient)
	dockerManager := createDockerManager(MockDockerClient)

	// Create a mock server for health checks
	mockServer := NewMockServer()
	defer mockServer.Close()
	mockClient, err := NewClientWithResponses(mockServer.URL)
	require.NoError(t, err)

	// Mock health check to keep the container running until Stop is called
	mockServer.On("ServeHTTP", "GET", "/health", mock.Anything).
		Return(200, "application/json", `{"status":"OK"}`).Maybe()

	ctx, cancel := context.WithTimeout(context.Background(), containerRemoveTimeout)
	defer cancel()
	containerID := "container1"
	rc := &RunnerContainer{
		RunnerContainerConfig: RunnerContainerConfig{
			ID: containerID,
		},
		Name:   containerID,
		Client: mockClient,
	}
	dockerManager.containers[containerID] = rc

	// Start the watchContainer goroutine
	dockerManager.watchGroup.Go(func() { dockerManager.watchContainer(rc) })

	MockDockerClient.On("ContainerStop", mock.Anything, containerID, expectedContainerStopOptions).Return(nil)
	MockDockerClient.On("ContainerRemove", mock.Anything, containerID, container.RemoveOptions{}).Return(nil)
	err = dockerManager.Stop(ctx)
	require.NoError(t, err)
	MockDockerClient.AssertExpectations(t)
}

func TestDockerManager_Borrow(t *testing.T) {
	mockDockerClient := new(MockDockerClient)
	dockerManager := createDockerManager(mockDockerClient)

	ctx := context.Background()
	pipeline := "text-to-image"
	modelID := "model"
	containerID, _ := dockerManager.getContainerImageName(pipeline, modelID)

	// Mock nested functions.
	defer updateDuringTest(&dockerWaitUntilRunningFunc, func(ctx context.Context, client DockerClient, containerID string, pollingInterval time.Duration) error {
		return nil
	})()
	defer updateDuringTest(&runnerWaitUntilReadyFunc, func(ctx context.Context, client *ClientWithResponses, pollingInterval time.Duration) (bool, error) {
		return false, nil
	})()

	mockDockerClient.On("ContainerCreate", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(container.CreateResponse{ID: containerID}, nil)
	mockDockerClient.On("ContainerStart", mock.Anything, containerID, mock.Anything).Return(nil)
	rc, err := dockerManager.Borrow(ctx, pipeline, modelID)
	require.NoError(t, err)
	require.NotNil(t, rc)
	require.Empty(t, dockerManager.containers, "containers map should be empty")
	mockDockerClient.AssertExpectations(t)
}

func TestDockerManager_returnContainer(t *testing.T) {
	mockDockerClient := new(MockDockerClient)
	dockerManager := createDockerManager(mockDockerClient)

	// Create a RunnerContainer to return to the pool
	rc := &RunnerContainer{
		Name:                  "container1",
		RunnerContainerConfig: RunnerContainerConfig{},
	}

	// Ensure the container is not in the pool initially.
	_, exists := dockerManager.containers[rc.Name]
	require.False(t, exists)

	// Return the container to the pool.
	dockerManager.returnContainer(rc)

	// Verify the container is now in the pool.
	returnedContainer, exists := dockerManager.containers[rc.Name]
	require.True(t, exists)
	require.Equal(t, rc, returnedContainer)
}

func TestDockerManager_getContainerImageName(t *testing.T) {
	mockDockerClient := new(MockDockerClient)
	dockerManager := createDockerManager(mockDockerClient)

	tests := []struct {
		name          string
		setup         func(*DockerManager, *MockDockerClient)
		pipeline      string
		modelID       string
		expectedImage string
		expectError   bool
	}{
		{
			name:          "live-video-to-video with valid modelID",
			setup:         func(dockerManager *DockerManager, mockDockerClient *MockDockerClient) {},
			pipeline:      "live-video-to-video",
			modelID:       "streamdiffusion",
			expectedImage: "livepeer/ai-runner:live-app-streamdiffusion",
			expectError:   false,
		},
		{
			name:          "live-video-to-video with cached attention sd15 modelID",
			setup:         func(dockerManager *DockerManager, mockDockerClient *MockDockerClient) {},
			pipeline:      "live-video-to-video",
			modelID:       "streamdiffusion-sd15-v2v",
			expectedImage: "livepeer/ai-runner:live-app-streamdiffusion-sd15-v2v",
			expectError:   false,
		},
		{
			name:          "live-video-to-video with cached attention sdxl modelID",
			setup:         func(dockerManager *DockerManager, mockDockerClient *MockDockerClient) {},
			pipeline:      "live-video-to-video",
			modelID:       "streamdiffusion-sdxl-v2v",
			expectedImage: "livepeer/ai-runner:live-app-streamdiffusion-sdxl-v2v",
			expectError:   false,
		},
		{
			name:        "live-video-to-video with invalid modelID",
			setup:       func(dockerManager *DockerManager, mockDockerClient *MockDockerClient) {},
			pipeline:    "live-video-to-video",
			modelID:     "invalid-model",
			expectError: true,
		},
		{
			name:          "valid pipeline",
			setup:         func(dockerManager *DockerManager, mockDockerClient *MockDockerClient) {},
			pipeline:      "text-to-speech",
			modelID:       "",
			expectedImage: "livepeer/ai-runner:text-to-speech",
			expectError:   false,
		},
		{
			name:          "invalid pipeline",
			setup:         func(dockerManager *DockerManager, mockDockerClient *MockDockerClient) {},
			pipeline:      "invalid-pipeline",
			modelID:       "",
			expectedImage: "default-image",
			expectError:   false,
		},
		{
			name: "override default image",
			setup: func(dockerManager *DockerManager, mockDockerClient *MockDockerClient) {
				dockerManager.overrides = ImageOverrides{
					Default: "custom-image",
				}
			},
			pipeline:      "",
			modelID:       "",
			expectedImage: "custom-image",
			expectError:   false,
		},
		{
			name: "override batch image",
			setup: func(dockerManager *DockerManager, mockDockerClient *MockDockerClient) {
				dockerManager.overrides = ImageOverrides{
					Batch: map[string]string{
						"text-to-speech": "custom-image",
					},
				}
			},
			pipeline:      "text-to-speech",
			modelID:       "",
			expectedImage: "custom-image",
			expectError:   false,
		},
		{
			name: "override live image",
			setup: func(dockerManager *DockerManager, mockDockerClient *MockDockerClient) {
				dockerManager.overrides = ImageOverrides{
					Live: map[string]string{
						"streamdiffusion": "custom-image",
					},
				}
			},
			pipeline:      "live-video-to-video",
			modelID:       "streamdiffusion",
			expectedImage: "custom-image",
			expectError:   false,
		},
		{
			name: "non-overridden batch image",
			setup: func(dockerManager *DockerManager, mockDockerClient *MockDockerClient) {
				dockerManager.overrides = ImageOverrides{
					Default: "default-image",
					Batch: map[string]string{
						"text-to-speech": "custom-batch-image",
					},
					Live: map[string]string{
						"streamdiffusion": "custom-live-image",
					},
				}
			},
			pipeline:      "audio-to-text",
			modelID:       "",
			expectedImage: "livepeer/ai-runner:audio-to-text",
			expectError:   false,
		},
		{
			name: "non-overridden live image",
			setup: func(dockerManager *DockerManager, mockDockerClient *MockDockerClient) {
				dockerManager.overrides = ImageOverrides{
					Default: "default-image",
					Batch: map[string]string{
						"text-to-speech": "custom-batch-image",
					},
					Live: map[string]string{
						"streamdiffusion": "custom-live-image",
					},
				}
			},
			pipeline:      "live-video-to-video",
			modelID:       "comfyui",
			expectedImage: "livepeer/ai-runner:live-app-comfyui",
			expectError:   false,
		},
		{
			name: "scope live image",
			setup: func(dockerManager *DockerManager, mockDockerClient *MockDockerClient) {
			},
			pipeline:      "live-video-to-video",
			modelID:       "scope",
			expectedImage: "livepeer/ai-runner:live-app-scope",
			expectError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup(dockerManager, mockDockerClient)

			image, err := dockerManager.getContainerImageName(tt.pipeline, tt.modelID)
			if tt.expectError {
				require.Error(t, err)
				require.Equal(t, fmt.Sprintf("no container image found for live pipeline %s", tt.modelID), err.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedImage, image)
			}
		})
	}
}

func TestDockerManager_HasCapacity(t *testing.T) {
	ctx := context.Background()
	pipeline := "text-to-image"
	modelID := "test-model"

	tests := []struct {
		name                string
		setup               func(*DockerManager, *MockDockerClient)
		expectedHasCapacity bool
	}{
		{
			name: "UnusedManagedContainerExists",
			setup: func(dockerManager *DockerManager, mockDockerClient *MockDockerClient) {
				// Add an unused managed container.
				dockerManager.containers["container1"] = &RunnerContainer{
					RunnerContainerConfig: RunnerContainerConfig{
						Pipeline: pipeline,
						ModelID:  modelID,
					}}
			},
			expectedHasCapacity: true,
		},
		{
			name: "ImageNotAvailable",
			setup: func(dockerManager *DockerManager, mockDockerClient *MockDockerClient) {
				// Mock client methods to simulate the image not being available locally.
				mockDockerClient.On("ImageInspectWithRaw", mock.Anything, "default-image").Return(types.ImageInspect{}, []byte{}, fmt.Errorf("image not found"))
			},
			expectedHasCapacity: false,
		},
		{
			name: "GPUAvailable",
			setup: func(dockerManager *DockerManager, mockDockerClient *MockDockerClient) {
				// Mock client methods to simulate the image being available locally.
				mockDockerClient.On("ImageInspectWithRaw", mock.Anything, "default-image").Return(types.ImageInspect{}, []byte{}, nil)
				// Ensure that the GPU is available by not setting any container for the GPU.
				dockerManager.gpuContainers = make(map[string]*RunnerContainer)
			},
			expectedHasCapacity: true,
		},
		{
			name: "GPUUnavailable",
			setup: func(dockerManager *DockerManager, mockDockerClient *MockDockerClient) {
				// Mock client methods to simulate the image being available locally.
				mockDockerClient.On("ImageInspectWithRaw", mock.Anything, "default-image").Return(types.ImageInspect{}, []byte{}, nil)
				// Ensure that the GPU is not available by setting a container for the GPU.
				dockerManager.gpuContainers["gpu0"] = &RunnerContainer{
					RunnerContainerConfig: RunnerContainerConfig{
						Pipeline: pipeline,
						ModelID:  modelID,
					}}
			},
			expectedHasCapacity: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockDockerClient := new(MockDockerClient)
			dockerManager := createDockerManager(mockDockerClient)

			tt.setup(dockerManager, mockDockerClient)

			hasCapacity := dockerManager.HasCapacity(ctx, pipeline, modelID)
			require.Equal(t, tt.expectedHasCapacity, hasCapacity)

			mockDockerClient.AssertExpectations(t)
		})
	}
}

func TestDockerManager_isImageAvailable(t *testing.T) {
	mockDockerClient := new(MockDockerClient)
	dockerManager := createDockerManager(mockDockerClient)

	ctx := context.Background()
	pipeline := "text-to-image"
	modelID := "test-model"

	t.Run("ImageNotFound", func(t *testing.T) {
		mockDockerClient.On("ImageInspectWithRaw", mock.Anything, "default-image").Return(types.ImageInspect{}, []byte{}, fmt.Errorf("image not found")).Once()

		isAvailable := dockerManager.isImageAvailable(ctx, pipeline, modelID)
		require.False(t, isAvailable)
		mockDockerClient.AssertExpectations(t)
	})

	t.Run("ImageFound", func(t *testing.T) {
		mockDockerClient.On("ImageInspectWithRaw", mock.Anything, "default-image").Return(types.ImageInspect{}, []byte{}, nil).Once()

		isAvailable := dockerManager.isImageAvailable(ctx, pipeline, modelID)
		require.True(t, isAvailable)
		mockDockerClient.AssertExpectations(t)
	})
}

func TestDockerManager_pullImage(t *testing.T) {
	mockDockerClient := new(MockDockerClient)
	dockerManager := createDockerManager(mockDockerClient)

	ctx := context.Background()
	imageName := "default-image"

	t.Run("ImagePullError", func(t *testing.T) {
		mockDockerClient.On("ImagePull", mock.Anything, imageName, mock.Anything).Return(io.NopCloser(strings.NewReader("")), fmt.Errorf("failed to pull image: pull error")).Once()

		err := dockerManager.pullImage(ctx, imageName)
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to pull image: pull error")
		mockDockerClient.AssertExpectations(t)
	})

	t.Run("ImagePullSuccess", func(t *testing.T) {
		mockDockerClient.On("ImagePull", mock.Anything, imageName, mock.Anything).Return(io.NopCloser(strings.NewReader("")), nil).Once()

		err := dockerManager.pullImage(ctx, imageName)
		require.NoError(t, err)
		mockDockerClient.AssertExpectations(t)
	})
}

func TestDockerManager_createContainer(t *testing.T) {
	mockDockerClient := new(MockDockerClient)
	dockerManager := createDockerManager(mockDockerClient)

	ctx := context.Background()
	pipeline := "text-to-image"
	modelID := "test-model"
	containerID := "container1"
	gpu := "0"
	containerHostPort := "8000"
	containerName := dockerContainerName(pipeline, modelID, containerHostPort)
	containerImage := "default-image"
	optimizationFlags := OptimizationFlags{}

	// Mock nested functions.
	defer updateDuringTest(&dockerWaitUntilRunningFunc, func(ctx context.Context, client DockerClient, containerID string, pollingInterval time.Duration) error {
		return nil
	})()
	defer updateDuringTest(&runnerWaitUntilReadyFunc, func(ctx context.Context, client *ClientWithResponses, pollingInterval time.Duration) (bool, error) {
		return false, nil
	})()

	// Mock allocGPU and getContainerImageName methods.
	dockerManager.gpus = []string{gpu}
	dockerManager.gpuContainers = make(map[string]*RunnerContainer)
	dockerManager.containers = make(map[string]*RunnerContainer)
	dockerManager.overrides.Default = containerImage

	mockDockerClient.On("ContainerCreate", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(container.CreateResponse{ID: containerID}, nil)
	mockDockerClient.On("ContainerStart", mock.Anything, containerID, mock.Anything).Return(nil)

	rc, err := dockerManager.createContainer(ctx, pipeline, modelID, false, optimizationFlags)
	require.NoError(t, err)
	require.NotNil(t, rc)
	require.Equal(t, containerID, rc.ID)
	require.Equal(t, gpu, rc.GPU)
	require.Equal(t, pipeline, rc.Pipeline)
	require.Equal(t, modelID, rc.ModelID)
	require.Equal(t, containerName, rc.Name)

	mockDockerClient.AssertExpectations(t)
}

func TestDockerManager_allocGPU(t *testing.T) {
	ctx := context.Background()

	container1 := func(keepWarm bool) *RunnerContainer {
		return &RunnerContainer{
			Name: "container1",
			RunnerContainerConfig: RunnerContainerConfig{
				ID:       "container1",
				KeepWarm: keepWarm,
				GPU:      "gpu0",
			},
		}
	}
	tests := []struct {
		name                 string
		setup                func(*DockerManager, *MockDockerClient)
		expectedAllocatedGPU string
		errorMessage         string
	}{
		{
			name: "GPUAvailable",
			setup: func(dockerManager *DockerManager, mockDockerClient *MockDockerClient) {
				// Ensure that the GPU is available by not setting any container for the GPU.
				dockerManager.gpuContainers = make(map[string]*RunnerContainer)
			},
			expectedAllocatedGPU: "gpu0",
			errorMessage:         "",
		},
		{
			name: "GPUUnavailable",
			setup: func(dockerManager *DockerManager, mockDockerClient *MockDockerClient) {
				// Ensure that the GPU is not available by setting a container for the GPU.
				rc := container1(true)
				dockerManager.gpuContainers[rc.GPU] = rc
			},
			expectedAllocatedGPU: "",
			errorMessage:         "insufficient capacity",
		},
		{
			name: "GPUUnavailableAndWarm",
			setup: func(dockerManager *DockerManager, mockDockerClient *MockDockerClient) {
				// Ensure that the GPU is not available by setting a container for the GPU.
				rc := container1(true)
				dockerManager.gpuContainers[rc.GPU] = rc
				dockerManager.containers[rc.Name] = rc
			},
			expectedAllocatedGPU: "",
			errorMessage:         "insufficient capacity",
		},
		{
			name: "GPUUnavailableButCold",
			setup: func(dockerManager *DockerManager, mockDockerClient *MockDockerClient) {
				// Ensure that the GPU is not available by setting a container for the GPU.
				rc := container1(false)
				dockerManager.gpuContainers[rc.GPU] = rc
				dockerManager.containers[rc.Name] = rc
				// Mock client methods to simulate the removal of the warm container.
				mockDockerClient.On("ContainerStop", mock.Anything, "container1", expectedContainerStopOptions).Return(nil)
				mockDockerClient.On("ContainerRemove", mock.Anything, "container1", container.RemoveOptions{}).Return(nil)
			},
			expectedAllocatedGPU: "gpu0",
			errorMessage:         "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockDockerClient := new(MockDockerClient)
			dockerManager := createDockerManager(mockDockerClient)

			tt.setup(dockerManager, mockDockerClient)

			gpu, err := dockerManager.allocGPU(ctx)
			if tt.errorMessage != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.errorMessage)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedAllocatedGPU, gpu)
			}
		})
	}
}

func TestDockerManager_destroyContainer(t *testing.T) {
	mockDockerClient := new(MockDockerClient)
	dockerManager := createDockerManager(mockDockerClient)

	containerID := "container1"
	gpu := "gpu0"

	rc := &RunnerContainer{
		Name: containerID,
		RunnerContainerConfig: RunnerContainerConfig{
			ID:  containerID,
			GPU: gpu,
		},
	}
	dockerManager.gpuContainers[gpu] = rc
	dockerManager.containers[containerID] = rc

	mockDockerClient.On("ContainerStop", mock.Anything, containerID, expectedContainerStopOptions).Return(nil)
	mockDockerClient.On("ContainerRemove", mock.Anything, containerID, container.RemoveOptions{}).Return(nil)

	err := dockerManager.destroyContainer(rc, true)
	require.NoError(t, err)
	require.Empty(t, dockerManager.gpuContainers, "gpuContainers map should be empty")
	require.Empty(t, dockerManager.containers, "containers map should be empty")
	mockDockerClient.AssertExpectations(t)
}

func TestDockerManager_watchContainer(t *testing.T) {
	// Override the containerWatchInterval for testing purposes.
	defer updateDuringTest(&containerWatchInterval, 10*time.Millisecond)()
	defer updateDuringTest(&containerTimeout, 100*time.Millisecond)()

	setup := func() (*MockDockerClient, *DockerManager, *MockServer, *RunnerContainer) {
		mockDockerClient := new(MockDockerClient)
		dockerManager := createDockerManager(mockDockerClient)

		mockServer := NewMockServer()
		mockClient, err := NewClientWithResponses(mockServer.URL)
		require.NoError(t, err)

		containerID := "container1"
		rc := &RunnerContainer{
			Name: containerID,
			RunnerContainerConfig: RunnerContainerConfig{
				ID: containerID,
			},
			Client: mockClient,
		}

		return mockDockerClient, dockerManager, mockServer, rc
	}

	t.Run("ReturnContainerOnContextDone", func(t *testing.T) {
		mockDockerClient, dockerManager, mockServer, rc := setup()
		defer mockServer.Close()
		defer mockServer.AssertExpectations(t)
		defer mockDockerClient.AssertNotCalled(t, "ContainerRemove", mock.Anything, rc.Name, mock.Anything)

		mockServer.On("ServeHTTP", "GET", "/health", mock.Anything).
			Return(200, "application/json", `{"status":"OK"}`)

		borrowCtx, cancel := context.WithCancel(context.Background())
		rc.BorrowCtx = borrowCtx

		watchingCtx, stopWatching := context.WithCancel(context.Background())
		go func() {
			dockerManager.watchContainer(rc)
			stopWatching()
		}()
		cancel()                          // Cancel the context.
		time.Sleep(50 * time.Millisecond) // Ensure the ticker triggers.

		// Verify that the container was returned.
		_, exists := dockerManager.containers[rc.Name]
		require.True(t, exists)

		// Watch should keep running
		require.Nil(t, watchingCtx.Err())

		// Borrow the container again
		borrowCtx, cancel = context.WithCancel(context.Background())
		delete(dockerManager.containers, rc.Name)
		rc.Lock()
		rc.BorrowCtx = borrowCtx
		rc.Unlock()

		cancel()                          // Cancel the borrow context
		time.Sleep(50 * time.Millisecond) // Ensure the ticker triggers.

		// Verify that the container was returned.
		_, exists = dockerManager.containers[rc.Name]
		require.True(t, exists)

		// Watch should still keep running
		require.Nil(t, watchingCtx.Err())
	})

	notHealthyTestCases := []struct {
		name            string
		mockServerSetup func(*MockServer)
	}{
		{
			name: "ErrorStatus",
			mockServerSetup: func(mockServer *MockServer) {
				mockServer.On("ServeHTTP", "GET", "/health", mock.Anything).
					Return(200, "application/json", `{"status":"ERROR"}`).
					Times(1) // should restart immediately with an ERROR
			},
		},
		{
			name: "UnknownStatus",
			mockServerSetup: func(mockServer *MockServer) {
				mockServer.On("ServeHTTP", "GET", "/health", mock.Anything).
					Return(200, "application/json", `{"status":"HAPPY"}`).
					Times(2)
			},
		},
		{
			name: "BadSchema",
			mockServerSetup: func(mockServer *MockServer) {
				mockServer.On("ServeHTTP", "GET", "/health", mock.Anything).
					Return(200, "application/json", `{"status":1}`).
					Times(2)
			},
		},
		{
			name: "HTTPError",
			mockServerSetup: func(mockServer *MockServer) {
				mockServer.On("ServeHTTP", "GET", "/health", mock.Anything).
					Return(500, "text/plain", "Error").
					Times(2)
			},
		},
		{
			name: "Timeout",
			mockServerSetup: func(mockServer *MockServer) {
				done := make(chan time.Time)
				go func() {
					time.Sleep(100 * time.Millisecond)
					close(done)
				}()
				mockServer.On("ServeHTTP", "GET", "/health", mock.Anything).
					WaitUntil(done).
					Return(200, "application/json", `{"status":"OK"}`).
					Times(2)
			},
		},
		{
			name: "ConnectionRefused",
			mockServerSetup: func(mockServer *MockServer) {
				mockServer.Close()
			},
		},
	}
	for _, tt := range notHealthyTestCases {
		t.Run("DestroyContainerOnNotHealthy_"+tt.name, func(t *testing.T) {
			// tight timeout for Timeout error case
			defer updateDuringTest(&healthcheckTimeout, 10*time.Millisecond)()

			mockDockerClient, dockerManager, mockServer, rc := setup()
			defer mockServer.Close()
			defer mockDockerClient.AssertExpectations(t)
			defer mockServer.AssertExpectations(t)

			tt.mockServerSetup(mockServer)

			// Mock destroyContainer to verify it is called.
			mockDockerClient.On("ContainerStop", mock.Anything, rc.Name, expectedContainerStopOptions).Return(nil).Once()
			mockDockerClient.On("ContainerRemove", mock.Anything, rc.Name, mock.Anything).Return(nil).Once()

			done := make(chan struct{})
			go func() {
				defer close(done)
				dockerManager.watchContainer(rc)
			}()
			select {
			case <-done:
			case <-time.After(500 * time.Millisecond):
				t.Fatal("watchContainer did not return")
			}

			// Verify that the container was destroyed.
			_, exists := dockerManager.containers[rc.Name]
			require.False(t, exists)
		})
	}

	t.Run("RespectLoadingStateGracePeriod", func(t *testing.T) {
		// Slightly below the time for 5 healthchecks, to avoid races
		defer updateDuringTest(&containerTimeout, 45*time.Millisecond)()

		mockDockerClient, dockerManager, mockServer, rc := setup()
		defer mockServer.Close()
		defer mockDockerClient.AssertExpectations(t)
		defer mockServer.AssertExpectations(t)
		dockerManager.gpuContainers[rc.GPU] = rc

		// Must fail twice before the watch routine gives up on it
		healthchecks := make(chan bool, 10)
		mockServer.On("ServeHTTP", "GET", "/health", mock.Anything).
			Run(func(args mock.Arguments) { healthchecks <- true }).
			Return(200, "application/json", `{"status":"LOADING"}`)

		timeout := time.After(500 * time.Millisecond)
		go dockerManager.watchContainer(rc)

		// We expect at least 6 calls (first call sets start time, then 4 calls until timeout, then 1 call for the first failure)
		var startTime time.Time
	graceLoop:
		for i := 1; i <= 6; i++ {
			select {
			case <-healthchecks:
				if i == 1 {
					startTime = time.Now()
				}
				if time.Since(startTime) > containerTimeout && i < 6 {
					// this is fine as there might be further delays in the healthcheck logic, just log it
					t.Logf("healthcheck was called %d times after %s, expected 6", i, time.Since(startTime))
					break graceLoop
				}
			case <-timeout:
				t.Fatal("healthcheck was not called at least 6 times within the timeout")
			}
		}

		// Make sure container wasn't destroyed yet
		mockDockerClient.AssertNotCalled(t, "ContainerRemove", mock.Anything, rc.Name, mock.Anything)

		// Mock destroyContainer to verify it is called.
		mockDockerClient.On("ContainerStop", mock.Anything, rc.Name, expectedContainerStopOptions).Return(nil).Once()
		mockDockerClient.On("ContainerRemove", mock.Anything, rc.Name, mock.Anything).Return(nil).Once()

		// after the first failure, there should only 1 more healthcheck for the container to be stopped
		select {
		case <-healthchecks:
			time.Sleep(10 * time.Millisecond) // give time for container to actually be stopped
		case <-timeout:
			// this is ok, we will check if the container was stopped below
		}

		// Verify that the container was destroyed.
		_, exists := dockerManager.containers[rc.Name]
		require.False(t, exists)
		_, exists = dockerManager.gpuContainers[rc.GPU]
		require.False(t, exists)
	})

	t.Run("ReturnContainerWhenIdle", func(t *testing.T) {
		mockDockerClient, dockerManager, mockServer, rc := setup()
		defer mockServer.Close()
		defer mockServer.AssertExpectations(t)
		defer mockDockerClient.AssertNotCalled(t, "ContainerRemove", mock.Anything, rc.Name, mock.Anything)

		// Schedule:
		// - Return LOADING for the first 3 times (should phantom borrow container)
		// - Then OK for the next 5 times (stayin' alive)
		// - Then back to IDLE (should return)
		idle := make(chan bool, 10)
		mockServer.On("ServeHTTP", "GET", "/health", mock.Anything).
			Return(200, "application/json", `{"status":"LOADING"}`).
			Times(3).
			On("ServeHTTP", "GET", "/health", mock.Anything).
			Return(200, "application/json", `{"status":"OK"}`).
			Times(5).
			On("ServeHTTP", "GET", "/health", mock.Anything).
			Run(func(args mock.Arguments) { idle <- true }).
			Return(200, "application/json", `{"status":"IDLE"}`) // Return only IDLE after that

		startTime := time.Now()
		timeUntil := func(sinceStart time.Duration) time.Duration {
			dur := sinceStart - time.Since(startTime)
			if dur <= 0 {
				return 0
			}
			return dur
		}
		rc.BorrowCtx = context.Background() // Simulate a borrow of the container
		go dockerManager.watchContainer(rc)
		time.Sleep(timeUntil(30 * time.Millisecond)) // Almost the entire grace period

		// Verify that the container was not returned yet.
		_, exists := dockerManager.containers[rc.Name]
		require.False(t, exists)

		time.Sleep(timeUntil(60 * time.Millisecond)) // Ensure we pass the grace period and container is OK

		// Verify that the container was not returned yet.
		_, exists = dockerManager.containers[rc.Name]
		require.False(t, exists)

		select {
		case <-idle:
			time.Sleep(10 * time.Millisecond) // give time for container to actually be returned
		case <-time.After(timeUntil(500 * time.Millisecond)):
			// this is ok, we will check if the container was returned below
		}

		// Verify that the container was returned.
		_, exists = dockerManager.containers[rc.Name]
		require.True(t, exists)
	})
}

// Watch container

func TestRemoveExistingContainers(t *testing.T) {
	mockDockerClient := new(MockDockerClient)

	ctx := context.Background()

	// Mock client methods to simulate the removal of existing containers.
	existingContainers := []types.Container{
		{ID: "container1", Names: []string{"/container1"}, Labels: map[string]string{containerCreatorLabel: containerCreator}},
		{ID: "container2", Names: []string{"/container2"}, Labels: map[string]string{containerCreatorLabel: containerCreator}},
	}
	mockDockerClient.On("ContainerList", mock.Anything, mock.Anything).Return(existingContainers, nil)
	mockDockerClient.On("ContainerStop", mock.Anything, "container1", expectedContainerStopOptions).Return(nil)
	mockDockerClient.On("ContainerStop", mock.Anything, "container2", expectedContainerStopOptions).Return(nil)
	mockDockerClient.On("ContainerRemove", mock.Anything, "container1", mock.Anything).Return(nil)
	mockDockerClient.On("ContainerRemove", mock.Anything, "container2", mock.Anything).Return(nil)

	RemoveExistingContainers(ctx, mockDockerClient, "instance-1")
	mockDockerClient.AssertExpectations(t)
}

func TestRemoveExistingContainers_InMemoryFilterLegacyAndOwnerID(t *testing.T) {
	mockDockerClient := new(MockDockerClient)

	ctx := context.Background()

	// Expect a single list call filtered only by creator label
	mockDockerClient.
		On("ContainerList", mock.Anything, mock.MatchedBy(func(opts container.ListOptions) bool {
			if !opts.All {
				return false
			}
			labels := opts.Filters.Get("label")
			if len(labels) == 0 {
				return false
			}
			// ensure we only filter by creator label; no owner id filter
			hasCreator := false
			for _, l := range labels {
				if l == containerCreatorLabel+"="+containerCreator {
					hasCreator = true
				}
				if strings.HasPrefix(l, containerCreatorIDLabel+"=") {
					return false
				}
			}
			return hasCreator
		})).
		Return([]types.Container{
			{ID: "legacy-1", Names: []string{"/legacy-1"}, Labels: map[string]string{containerCreatorLabel: containerCreator}},                                   // no creator_id -> remove
			{ID: "other-1", Names: []string{"/other-1"}, Labels: map[string]string{containerCreatorLabel: containerCreator, containerCreatorIDLabel: "owner-B"}}, // mismatched -> keep
			{ID: "other-2-empty", Names: []string{"/other-2"}, Labels: map[string]string{containerCreatorLabel: containerCreator, containerCreatorIDLabel: ""}},  // empty -> keep (new version but empty label)
			{ID: "mine-1", Names: []string{"/mine-1"}, Labels: map[string]string{containerCreatorLabel: containerCreator, containerCreatorIDLabel: "owner-A"}},   // match -> remove
		}, nil).
		Once()
	mockDockerClient.On("ContainerStop", mock.Anything, "legacy-1", expectedContainerStopOptions).Return(nil).Once()
	mockDockerClient.On("ContainerRemove", mock.Anything, "legacy-1", mock.Anything).Return(nil).Once()
	mockDockerClient.On("ContainerStop", mock.Anything, "mine-1", expectedContainerStopOptions).Return(nil).Once()
	mockDockerClient.On("ContainerRemove", mock.Anything, "mine-1", mock.Anything).Return(nil).Once()

	removed, err := RemoveExistingContainers(ctx, mockDockerClient, "owner-A")
	require.NoError(t, err)
	require.Equal(t, 2, removed)

	mockDockerClient.AssertExpectations(t)
}

func TestDockerContainerName(t *testing.T) {
	tests := []struct {
		name         string
		pipeline     string
		modelID      string
		suffix       []string
		expectedName string
	}{
		{
			name:         "with suffix",
			pipeline:     "text-to-speech",
			modelID:      "model1",
			suffix:       []string{"suffix1"},
			expectedName: "text-to-speech_model1_suffix1",
		},
		{
			name:         "without suffix",
			pipeline:     "text-to-speech",
			modelID:      "model1",
			expectedName: "text-to-speech_model1",
		},
		{
			name:         "modelID with special characters",
			pipeline:     "text-to-speech",
			modelID:      "model/1_2",
			suffix:       []string{"suffix1"},
			expectedName: "text-to-speech_model-1-2_suffix1",
		},
		{
			name:         "modelID with special characters without suffix",
			pipeline:     "text-to-speech",
			modelID:      "model/1_2",
			expectedName: "text-to-speech_model-1-2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name := dockerContainerName(tt.pipeline, tt.modelID, tt.suffix...)
			require.Equal(t, tt.expectedName, name)
		})
	}
}

func TestDockerRemoveContainer(t *testing.T) {
	mockDockerClient := new(MockDockerClient)

	mockDockerClient.On("ContainerStop", mock.Anything, "container1", expectedContainerStopOptions).Return(nil)
	mockDockerClient.On("ContainerRemove", mock.Anything, "container1", container.RemoveOptions{}).Return(nil)

	err := dockerRemoveContainer(mockDockerClient, "container1")
	require.NoError(t, err)
	mockDockerClient.AssertExpectations(t)
}

func TestDockerWaitUntilRunning(t *testing.T) {
	mockDockerClient := new(MockDockerClient)
	containerID := "container1"
	pollingInterval := 10 * time.Millisecond
	ctx := context.Background()

	t.Run("ContainerRunning", func(t *testing.T) {
		// Mock ContainerInspect to return a running container state.
		mockDockerClient.On("ContainerInspect", mock.Anything, containerID).Return(types.ContainerJSON{
			ContainerJSONBase: &types.ContainerJSONBase{
				State: &types.ContainerState{
					Running: true,
				},
			},
		}, nil).Once()

		err := dockerWaitUntilRunning(ctx, mockDockerClient, containerID, pollingInterval)
		require.NoError(t, err)
		mockDockerClient.AssertExpectations(t)
	})

	t.Run("ContainerNotRunningInitially", func(t *testing.T) {
		// Mock ContainerInspect to return a non-running state initially, then a running state.
		mockDockerClient.On("ContainerInspect", mock.Anything, containerID).Return(types.ContainerJSON{
			ContainerJSONBase: &types.ContainerJSONBase{
				State: &types.ContainerState{
					Running: false,
				},
			},
		}, nil).Once()
		mockDockerClient.On("ContainerInspect", mock.Anything, containerID).Return(types.ContainerJSON{
			ContainerJSONBase: &types.ContainerJSONBase{
				State: &types.ContainerState{
					Running: true,
				},
			},
		}, nil).Once()

		err := dockerWaitUntilRunning(ctx, mockDockerClient, containerID, pollingInterval)
		require.NoError(t, err)
		mockDockerClient.AssertExpectations(t)
	})

	t.Run("ContextTimeout", func(t *testing.T) {
		// Create a context that will timeout.
		timeoutCtx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		// Mock ContainerInspect to always return a non-running state.
		mockDockerClient.On("ContainerInspect", mock.Anything, containerID).Return(types.ContainerJSON{
			ContainerJSONBase: &types.ContainerJSONBase{
				State: &types.ContainerState{
					Running: false,
				},
			},
		}, nil)

		err := dockerWaitUntilRunning(timeoutCtx, mockDockerClient, containerID, pollingInterval)
		require.Error(t, err)
		require.Contains(t, err.Error(), "timed out waiting for managed container")
		mockDockerClient.AssertExpectations(t)
	})

	t.Run("FailFastOnExited", func(t *testing.T) {
		// If the container is immediately exited, we should fail fast instead of waiting.
		mockDockerClient := new(MockDockerClient)
		// Always return non-running, exited state
		mockDockerClient.On("ContainerInspect", mock.Anything, containerID).Return(types.ContainerJSON{
			ContainerJSONBase: &types.ContainerJSONBase{
				State: &types.ContainerState{
					Status:   "exited",
					Running:  false,
					ExitCode: 137,
				},
			},
		}, nil)

		err := dockerWaitUntilRunning(ctx, mockDockerClient, containerID, pollingInterval)
		require.Error(t, err)
		require.Contains(t, err.Error(), "terminal state")
		mockDockerClient.AssertExpectations(t)
	})

	t.Run("FailFastOnDead", func(t *testing.T) {
		mockDockerClient := new(MockDockerClient)
		mockDockerClient.On("ContainerInspect", mock.Anything, containerID).Return(types.ContainerJSON{
			ContainerJSONBase: &types.ContainerJSONBase{
				State: &types.ContainerState{
					Status:  "dead",
					Running: false,
					Error:   "killed",
				},
			},
		}, nil)

		err := dockerWaitUntilRunning(ctx, mockDockerClient, containerID, pollingInterval)
		require.Error(t, err)
		require.Contains(t, err.Error(), "container entered terminal state")
		mockDockerClient.AssertExpectations(t)
	})

	t.Run("FailFastOnExitCodeNonZeroWithoutRestarting", func(t *testing.T) {
		mockDockerClient := new(MockDockerClient)
		mockDockerClient.On("ContainerInspect", mock.Anything, containerID).Return(types.ContainerJSON{
			ContainerJSONBase: &types.ContainerJSONBase{
				State: &types.ContainerState{
					Status:     "created",
					Running:    false,
					Restarting: false,
					ExitCode:   1,
				},
			},
		}, nil)

		err := dockerWaitUntilRunning(ctx, mockDockerClient, containerID, pollingInterval)
		require.Error(t, err)
		require.Contains(t, err.Error(), "exited before running")
		mockDockerClient.AssertExpectations(t)
	})
}

func TestHwGPU(t *testing.T) {
	tests := []struct {
		name          string
		gpu           string
		expectedHwGPU string
	}{
		{
			name:          "Standard GPU",
			gpu:           "0",
			expectedHwGPU: "0",
		},
		{
			name:          "Emulated GPU",
			gpu:           "emulated-0",
			expectedHwGPU: "emulated-0",
		},
		{
			name:          "Colocated GPU",
			gpu:           "colocated-2-1",
			expectedHwGPU: "1",
		},
		{
			name:          "Colocated Emulated GPU",
			gpu:           "colocated-1-emulated-0",
			expectedHwGPU: "emulated-0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := hwGPU(tt.gpu)
			require.Equal(t, tt.expectedHwGPU, res)
		})
	}
}

func TestPortOffset(t *testing.T) {
	tests := []struct {
		name               string
		gpu                string
		expectedPortOffset string
	}{
		{
			name:               "Standard GPU",
			gpu:                "0",
			expectedPortOffset: "00",
		},
		{
			name:               "Emulated GPU",
			gpu:                "emulated-1",
			expectedPortOffset: "01",
		},
		{
			name:               "Colocated GPU",
			gpu:                "colocated-2-1",
			expectedPortOffset: "21",
		},
		{
			name:               "Colocated Emulated GPU",
			gpu:                "colocated-1-emulated-0",
			expectedPortOffset: "10",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := portOffset(tt.gpu)
			require.Equal(t, tt.expectedPortOffset, res)
		})
	}
}

func updateDuringTest[T any](variable *T, testValue T) func() {
	originalValue := *variable
	*variable = testValue
	return func() { *variable = originalValue }
}
