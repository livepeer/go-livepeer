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
	return &DockerManager{
		defaultImage:  "default-image",
		gpus:          []string{"gpu0"},
		modelDir:      "/models",
		dockerClient:  mockDockerClient,
		gpuContainers: make(map[string]string),
		containers:    make(map[string]*RunnerContainer),
		mu:            &sync.Mutex{},
	}
}

func TestNewDockerManager(t *testing.T) {
	mockDockerClient := new(MockDockerClient)

	createAndVerifyManager := func() *DockerManager {
		manager, err := NewDockerManager("default-image", []string{"gpu0"}, "/models", mockDockerClient)
		require.NoError(t, err)
		require.NotNil(t, manager)
		require.Equal(t, "default-image", manager.defaultImage)
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
			{ID: "container1", Names: []string{"/container1"}},
			{ID: "container2", Names: []string{"/container2"}},
		}
		mockDockerClient.On("ContainerList", mock.Anything, mock.Anything).Return(existingContainers, nil)
		mockDockerClient.On("ContainerStop", mock.Anything, "container1", mock.Anything).Return(nil)
		mockDockerClient.On("ContainerStop", mock.Anything, "container2", mock.Anything).Return(nil)
		mockDockerClient.On("ContainerRemove", mock.Anything, "container1", mock.Anything).Return(nil)
		mockDockerClient.On("ContainerRemove", mock.Anything, "container2", mock.Anything).Return(nil)

		// Verify that existing containers were stopped and removed.
		createAndVerifyManager()
		mockDockerClient.AssertCalled(t, "ContainerStop", mock.Anything, "container1", mock.Anything)
		mockDockerClient.AssertCalled(t, "ContainerStop", mock.Anything, "container2", mock.Anything)
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
	originalFunc := dockerWaitUntilRunningFunc
	dockerWaitUntilRunningFunc = func(ctx context.Context, client DockerClient, containerID string, pollingInterval time.Duration) error {
		return nil
	}
	defer func() { dockerWaitUntilRunningFunc = originalFunc }()
	originalFunc2 := runnerWaitUntilReadyFunc
	runnerWaitUntilReadyFunc = func(ctx context.Context, client *ClientWithResponses, pollingInterval time.Duration) error {
		return nil
	}
	defer func() { runnerWaitUntilReadyFunc = originalFunc2 }()

	mockDockerClient.On("ContainerCreate", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(container.CreateResponse{ID: containerID}, nil)
	mockDockerClient.On("ContainerStart", mock.Anything, containerID, mock.Anything).Return(nil)
	err := dockerManager.Warm(ctx, pipeline, modelID, optimizationFlags)
	require.NoError(t, err)
	mockDockerClient.AssertExpectations(t)
}

func TestDockerManager_Stop(t *testing.T) {
	MockDockerClient := new(MockDockerClient)
	dockerManager := createDockerManager(MockDockerClient)

	ctx, cancel := context.WithTimeout(context.Background(), containerRemoveTimeout)
	defer cancel()
	containerID := "container1"
	dockerManager.containers[containerID] = &RunnerContainer{
		RunnerContainerConfig: RunnerContainerConfig{
			ID: containerID,
		},
	}

	MockDockerClient.On("ContainerStop", mock.Anything, containerID, container.StopOptions{Timeout: nil}).Return(nil)
	MockDockerClient.On("ContainerRemove", mock.Anything, containerID, container.RemoveOptions{}).Return(nil)
	err := dockerManager.Stop(ctx)
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
	originalFunc := dockerWaitUntilRunningFunc
	dockerWaitUntilRunningFunc = func(ctx context.Context, client DockerClient, containerID string, pollingInterval time.Duration) error {
		return nil
	}
	defer func() { dockerWaitUntilRunningFunc = originalFunc }()
	originalFunc2 := runnerWaitUntilReadyFunc
	runnerWaitUntilReadyFunc = func(ctx context.Context, client *ClientWithResponses, pollingInterval time.Duration) error {
		return nil
	}
	defer func() { runnerWaitUntilReadyFunc = originalFunc2 }()

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
	manager := createDockerManager(mockDockerClient)

	tests := []struct {
		name          string
		pipeline      string
		modelID       string
		expectedImage string
		expectError   bool
	}{
		{
			name:          "live-video-to-video with valid modelID",
			pipeline:      "live-video-to-video",
			modelID:       "streamdiffusion",
			expectedImage: "livepeer/ai-runner:live-app-streamdiffusion",
			expectError:   false,
		},
		{
			name:        "live-video-to-video with invalid modelID",
			pipeline:    "live-video-to-video",
			modelID:     "invalid-model",
			expectError: true,
		},
		{
			name:          "valid pipeline",
			pipeline:      "text-to-speech",
			modelID:       "",
			expectedImage: "livepeer/ai-runner:text-to-speech",
			expectError:   false,
		},
		{
			name:          "invalid pipeline",
			pipeline:      "invalid-pipeline",
			modelID:       "",
			expectedImage: "default-image",
			expectError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			image, err := manager.getContainerImageName(tt.pipeline, tt.modelID)
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
				dockerManager.gpuContainers = make(map[string]string)
			},
			expectedHasCapacity: true,
		},
		{
			name: "GPUUnavailable",
			setup: func(dockerManager *DockerManager, mockDockerClient *MockDockerClient) {
				// Mock client methods to simulate the image being available locally.
				mockDockerClient.On("ImageInspectWithRaw", mock.Anything, "default-image").Return(types.ImageInspect{}, []byte{}, nil)
				// Ensure that the GPU is not available by setting a container for the GPU.
				dockerManager.gpuContainers["gpu0"] = "container1"
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
	originalFunc := dockerWaitUntilRunningFunc
	dockerWaitUntilRunningFunc = func(ctx context.Context, client DockerClient, containerID string, pollingInterval time.Duration) error {
		return nil
	}
	defer func() { dockerWaitUntilRunningFunc = originalFunc }()
	originalFunc2 := runnerWaitUntilReadyFunc
	runnerWaitUntilReadyFunc = func(ctx context.Context, client *ClientWithResponses, pollingInterval time.Duration) error {
		return nil
	}
	defer func() { runnerWaitUntilReadyFunc = originalFunc2 }()

	// Mock allocGPU and getContainerImageName methods.
	dockerManager.gpus = []string{gpu}
	dockerManager.gpuContainers = make(map[string]string)
	dockerManager.containers = make(map[string]*RunnerContainer)
	dockerManager.defaultImage = containerImage

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
				dockerManager.gpuContainers = make(map[string]string)
			},
			expectedAllocatedGPU: "gpu0",
			errorMessage:         "",
		},
		{
			name: "GPUUnavailable",
			setup: func(dockerManager *DockerManager, mockDockerClient *MockDockerClient) {
				// Ensure that the GPU is not available by setting a container for the GPU.
				dockerManager.gpuContainers["gpu0"] = "container1"
			},
			expectedAllocatedGPU: "",
			errorMessage:         "insufficient capacity",
		},
		{
			name: "GPUUnavailableAndWarm",
			setup: func(dockerManager *DockerManager, mockDockerClient *MockDockerClient) {
				// Ensure that the GPU is not available by setting a container for the GPU.
				dockerManager.gpuContainers["gpu0"] = "container1"
				dockerManager.containers["container1"] = &RunnerContainer{
					RunnerContainerConfig: RunnerContainerConfig{
						ID:       "container1",
						KeepWarm: true,
					},
				}
			},
			expectedAllocatedGPU: "",
			errorMessage:         "insufficient capacity",
		},
		{
			name: "GPUUnavailableButCold",
			setup: func(dockerManager *DockerManager, mockDockerClient *MockDockerClient) {
				// Ensure that the GPU is not available by setting a container for the GPU.
				dockerManager.gpuContainers["gpu0"] = "container1"
				dockerManager.containers["container1"] = &RunnerContainer{
					RunnerContainerConfig: RunnerContainerConfig{
						ID:       "container1",
						KeepWarm: false,
					},
				}
				// Mock client methods to simulate the removal of the warm container.
				mockDockerClient.On("ContainerStop", mock.Anything, "container1", container.StopOptions{}).Return(nil)
				mockDockerClient.On("ContainerRemove", mock.Anything, "container1", container.RemoveOptions{}).Return(nil)
			},
			expectedAllocatedGPU: "gpu0",
			errorMessage:         "",
		},
	}

	for _, tt := range tests {
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
	dockerManager.gpuContainers[gpu] = containerID
	dockerManager.containers[containerID] = rc

	mockDockerClient.On("ContainerStop", mock.Anything, containerID, container.StopOptions{}).Return(nil)
	mockDockerClient.On("ContainerRemove", mock.Anything, containerID, container.RemoveOptions{}).Return(nil)

	err := dockerManager.destroyContainer(rc, true)
	require.NoError(t, err)
	require.Empty(t, dockerManager.gpuContainers, "gpuContainers map should be empty")
	require.Empty(t, dockerManager.containers, "containers map should be empty")
	mockDockerClient.AssertExpectations(t)
}

func TestDockerManager_watchContainer(t *testing.T) {
	// Override the containerWatchInterval for testing purposes.
	originalContainerWatchInterval, originalPipelineStartGracePeriod := containerWatchInterval, pipelineStartGracePeriod
	containerWatchInterval = 10 * time.Millisecond
	pipelineStartGracePeriod = 0
	defer func() {
		containerWatchInterval, pipelineStartGracePeriod = originalContainerWatchInterval, originalPipelineStartGracePeriod
	}()

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

		borrowCtx, cancel := context.WithCancel(context.Background())

		go dockerManager.watchContainer(rc, borrowCtx)
		cancel()                          // Cancel the context.
		time.Sleep(50 * time.Millisecond) // Ensure the ticker triggers.

		// Verify that the container was returned.
		_, exists := dockerManager.containers[rc.Name]
		require.True(t, exists)

		mockServer.AssertNotCalled(t, "ServeHTTP", "GET", "/health", mock.Anything)
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
					Times(2)
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
			mockDockerClient, dockerManager, mockServer, rc := setup()
			defer mockServer.Close()
			defer mockDockerClient.AssertExpectations(t)
			defer mockServer.AssertExpectations(t)

			borrowCtx := context.Background()

			tt.mockServerSetup(mockServer)

			// Mock destroyContainer to verify it is called.
			mockDockerClient.On("ContainerStop", mock.Anything, rc.Name, mock.Anything).Return(nil).Once()
			mockDockerClient.On("ContainerRemove", mock.Anything, rc.Name, mock.Anything).Return(nil).Once()

			done := make(chan struct{})
			go func() {
				defer close(done)
				dockerManager.watchContainer(rc, borrowCtx)
			}()
			select {
			case <-done:
			case <-time.After(50 * time.Millisecond):
				t.Fatal("watchContainer did not return")
			}

			// Verify that the container was destroyed.
			_, exists := dockerManager.containers[rc.Name]
			require.False(t, exists)
		})
	}

	t.Run("RespectStartupGracePeriod", func(t *testing.T) {
		pipelineStartGracePeriod = 50 * time.Millisecond
		defer func() { pipelineStartGracePeriod = 0 }()

		mockDockerClient, dockerManager, mockServer, rc := setup()
		defer mockServer.Close()
		defer mockDockerClient.AssertExpectations(t)
		defer mockServer.AssertExpectations(t)

		borrowCtx := context.Background()

		// Must fail twice before the watch routine gives up on it
		mockServer.On("ServeHTTP", "GET", "/health", mock.Anything).
			Return(200, "application/json", `{"status":"ERROR"}`).
			Times(4) // 4 calls during the grace period (first call only after 10ms)

		go dockerManager.watchContainer(rc, borrowCtx)
		time.Sleep(40 * time.Millisecond) // Almost the entire grace period

		// Make sure container wasn't destroyed yet
		mockDockerClient.AssertNotCalled(t, "ContainerRemove", mock.Anything, rc.Name, mock.Anything)

		// Mock destroyContainer to verify it is called.
		mockDockerClient.On("ContainerStop", mock.Anything, rc.Name, mock.Anything).Return(nil).Once()
		mockDockerClient.On("ContainerRemove", mock.Anything, rc.Name, mock.Anything).Return(nil).Once()

		time.Sleep(30 * time.Millisecond) // Ensure we pass the grace period

		// Verify that the container was destroyed.
		_, exists := dockerManager.containers[rc.Name]
		require.False(t, exists)
	})

	t.Run("ReturnContainerWhenIdle", func(t *testing.T) {
		pipelineStartGracePeriod = 50 * time.Millisecond
		defer func() { pipelineStartGracePeriod = 0 }()

		mockDockerClient, dockerManager, mockServer, rc := setup()
		defer mockServer.Close()
		defer mockServer.AssertExpectations(t)
		defer mockDockerClient.AssertNotCalled(t, "ContainerRemove", mock.Anything, rc.Name, mock.Anything)

		borrowCtx := context.Background()

		// Schedule:
		// - Return IDLE for the first 3 times during grace period (should not return)
		// - Then OK for the next 5 times (stayin' alive)
		// - Then back to IDLE (should return)
		mockServer.On("ServeHTTP", "GET", "/health", mock.Anything).
			Return(200, "application/json", `{"status":"IDLE"}`).
			Times(3).
			On("ServeHTTP", "GET", "/health", mock.Anything).
			Return(200, "application/json", `{"status":"OK"}`).
			Times(5).
			On("ServeHTTP", "GET", "/health", mock.Anything).
			Return(200, "application/json", `{"status":"IDLE"}`).
			Once() // once is enough and container should be returned immediately

		startTime := time.Now()
		sleepUntil := func(sinceStart time.Duration) {
			dur := sinceStart - time.Since(startTime)
			if dur > 0 {
				time.Sleep(dur)
			}
		}
		go dockerManager.watchContainer(rc, borrowCtx)
		sleepUntil(30 * time.Millisecond) // Almost the entire grace period

		// Verify that the container was not returned yet.
		_, exists := dockerManager.containers[rc.Name]
		require.False(t, exists)

		sleepUntil(60 * time.Millisecond) // Ensure we pass the grace period and container is OK

		// Verify that the container was not returned yet.
		_, exists = dockerManager.containers[rc.Name]
		require.False(t, exists)

		sleepUntil(100 * time.Millisecond) // Ensure we pass the grace period and container is now IDLE and returned

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
		{ID: "container1", Names: []string{"/container1"}},
		{ID: "container2", Names: []string{"/container2"}},
	}
	mockDockerClient.On("ContainerList", mock.Anything, mock.Anything).Return(existingContainers, nil)
	mockDockerClient.On("ContainerStop", mock.Anything, "container1", mock.Anything).Return(nil)
	mockDockerClient.On("ContainerStop", mock.Anything, "container2", mock.Anything).Return(nil)
	mockDockerClient.On("ContainerRemove", mock.Anything, "container1", mock.Anything).Return(nil)
	mockDockerClient.On("ContainerRemove", mock.Anything, "container2", mock.Anything).Return(nil)

	removeExistingContainers(ctx, mockDockerClient)
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

	mockDockerClient.On("ContainerStop", mock.Anything, "container1", container.StopOptions{}).Return(nil)
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
}
