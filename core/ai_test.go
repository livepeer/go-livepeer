package core

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	aiworker "github.com/livepeer/ai-worker/worker"
	"github.com/livepeer/go-livepeer/net"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

// Mock interfaces
type mockStream struct {
	net.Transcoder_RegisterAIWorkerServer
	sendFunc func(*net.NotifyAIJob) error
	ctx      context.Context
}

func (m *mockStream) Send(job *net.NotifyAIJob) error {
	return m.sendFunc(job)
}

func (m *mockStream) Context() context.Context {
	if m.ctx == nil {
		return context.Background()
	}
	return m.ctx
}

func TestNewRemoteAIWorkerManager(t *testing.T) {
	manager := NewRemoteAIWorkerManager()
	require.NotNil(t, manager, "Expected non-nil manager")
	assert.Empty(t, manager.remoteWorkers, "Expected empty remoteWorkers")
	assert.Empty(t, manager.liveWorkers, "Expected empty liveWorkers")
	assert.Empty(t, manager.taskChans, "Expected empty taskChans")
	assert.Zero(t, manager.taskCount, "Expected taskCount to be 0")
}

func TestManage(t *testing.T) {
	manager := NewRemoteAIWorkerManager()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stream := &mockStream{
		ctx: ctx,
		sendFunc: func(*net.NotifyAIJob) error {
			return nil
		},
	}
	capabilities := &Capabilities{
		capabilityConstraints: CapabilityConstraints{
			Capability_TextToImage: &PerCapabilityConstraints{
				Models: ModelConstraints{
					"model1": &ModelConstraint{},
				},
			},
		},
	}

	manageDone := make(chan struct{})
	go func() {
		manager.Manage(stream, capabilities.ToNetCapabilities())
		close(manageDone)
	}()

	time.Sleep(100 * time.Millisecond) // Give some time for goroutine to execute

	assert.Len(t, manager.liveWorkers, 1, "Expected 1 live worker")
	assert.Len(t, manager.remoteWorkers[Capability_TextToImage], 1, "Expected 1 remote worker for TextToImage")
	assert.Len(t, manager.remoteWorkers[Capability_TextToImage]["model1"], 1, "Expected 1 remote worker for model1")

	// Simulate stream closing
	cancel()

	select {
	case <-manageDone:
		// Manage function has finished
	case <-time.After(5 * time.Second):
		t.Fatal("Manage function did not finish within the expected time")
	}

	assert.Empty(t, manager.liveWorkers, "Expected 0 live workers after closing")
	assert.Empty(t, manager.remoteWorkers[Capability_TextToImage]["model1"], "Expected 0 remote workers after closing")
}

func TestProcessAIRequest(t *testing.T) {
	manager := NewRemoteAIWorkerManager()
	stream := &mockStream{}
	worker := NewRemoteAIWorker(manager, stream, "test-addr", nil)

	manager.remoteWorkers[Capability_TextToImage] = map[string][]*RemoteAIWorker{
		"model1": {worker},
	}

	req := aiworker.TextToImageParams{
		ModelId: stringPtr("model1"),
		Prompt:  "test prompt",
	}

	stream.sendFunc = func(job *net.NotifyAIJob) error {
		go func() {
			manager.aiResult(&RemoteAIWorkerResult{
				JobType: job.Type,
				TaskID:  job.TaskID,
				Bytes:   []byte(`{"images": [{"url": "test-image"}]}`),
			})
		}()
		return nil
	}

	res, err := manager.processAIRequest(context.Background(), Capability_TextToImage, req, net.AIRequestType_TextToImage)

	require.NoError(t, err, "Unexpected error")

	imgRes, ok := res.(*aiworker.ImageResponse)
	require.True(t, ok, "Expected *ImageResponse")

	assert.Len(t, imgRes.Images, 1, "Expected 1 image in response")
	assert.Equal(t, "test-image", imgRes.Images[0].Url, "Unexpected image URL")
}

func TestSelectWorker(t *testing.T) {
	manager := NewRemoteAIWorkerManager()
	cap1 := &Capabilities{
		capabilityConstraints: CapabilityConstraints{
			Capability_TextToImage: &PerCapabilityConstraints{
				Models: ModelConstraints{
					"model1": &ModelConstraint{},
				},
			},
		},
	}
	cap2 := &Capabilities{
		capabilityConstraints: CapabilityConstraints{
			Capability_TextToImage: &PerCapabilityConstraints{
				Models: ModelConstraints{
					"model2": &ModelConstraint{},
				},
			},
		},
	}

	worker1 := NewRemoteAIWorker(manager, nil, "addr1", cap1)
	worker2 := NewRemoteAIWorker(manager, nil, "addr2", cap2)
	worker3 := NewRemoteAIWorker(manager, nil, "addr3", cap1)

	manager.remoteWorkers[Capability_TextToImage] = map[string][]*RemoteAIWorker{
		"model1": {worker1, worker3},
		"model2": {worker2},
	}

	// First selection
	selected, err := manager.selectWorker(Capability_TextToImage, "model1")
	require.NoError(t, err, "Unexpected error")
	assert.Equal(t, "addr1", selected.addr, "Expected addr1")

	// Second selection (should rotate)
	selected, err = manager.selectWorker(Capability_TextToImage, "model1")
	require.NoError(t, err, "Unexpected error")
	assert.Equal(t, "addr3", selected.addr, "Expected addr3")

	// Third selection (should rotate back to first)
	selected, err = manager.selectWorker(Capability_TextToImage, "model1")
	require.NoError(t, err, "Unexpected error")
	assert.Equal(t, "addr1", selected.addr, "Expected addr1")

	// model2
	selected, err = manager.selectWorker(Capability_TextToImage, "model2")
	require.NoError(t, err, "Unexpected error")
	assert.Equal(t, "addr2", selected.addr, "Expected addr2")
}

func TestHasCapacity(t *testing.T) {
	manager := NewRemoteAIWorkerManager()
	worker := NewRemoteAIWorker(manager, nil, "test-addr", nil)

	manager.remoteWorkers[Capability_TextToImage] = map[string][]*RemoteAIWorker{
		"model1": {worker},
	}

	assert.True(t, manager.HasCapacity("text-to-image", "model1"), "Expected HasCapacity to return true for text-to-image model1")
	assert.False(t, manager.HasCapacity("text-to-image", "model2"), "Expected HasCapacity to return false for text-to-image model2")
	assert.False(t, manager.HasCapacity("image-to-image", "model1"), "Expected HasCapacity to return false for image-to-image model1")
}

func TestAddRemoveTaskChan(t *testing.T) {
	manager := NewRemoteAIWorkerManager()

	taskID, taskChan := manager.addTaskChan()
	assert.Equal(t, int64(0), taskID, "Expected taskID 0")
	assert.NotNil(t, taskChan, "Expected non-nil taskChan")

	retrievedChan, err := manager.getTaskChan(taskID)
	require.NoError(t, err, "Unexpected error")
	assert.Equal(t, taskChan, retrievedChan, "Retrieved channel does not match added channel")

	manager.removeTaskChan(taskID)

	_, err = manager.getTaskChan(taskID)
	assert.Error(t, err, "Expected error after removing task channel")
}

func TestTextToImage(t *testing.T) {
	manager := NewRemoteAIWorkerManager()
	stream := &mockStream{}
	worker := NewRemoteAIWorker(manager, stream, "test-addr", nil)

	manager.remoteWorkers[Capability_TextToImage] = map[string][]*RemoteAIWorker{
		"model1": {worker},
	}

	stream.sendFunc = func(job *net.NotifyAIJob) error {
		go func() {
			manager.aiResult(&RemoteAIWorkerResult{
				JobType: job.Type,
				TaskID:  job.TaskID,
				Bytes:   []byte(`{"images": [{"url": "test-image-url"}]}`),
			})
		}()
		return nil
	}

	req := aiworker.TextToImageJSONRequestBody{
		ModelId: stringPtr("model1"),
		Prompt:  "test prompt",
	}

	res, err := manager.TextToImage(context.Background(), req)

	require.NoError(t, err)
	require.NotNil(t, res)
	assert.Len(t, res.Images, 1)
	assert.Equal(t, "test-image-url", res.Images[0].Url)
}

func TestImageToImage(t *testing.T) {
	manager := NewRemoteAIWorkerManager()
	stream := &mockStream{}
	worker := NewRemoteAIWorker(manager, stream, "test-addr", nil)

	manager.remoteWorkers[Capability_ImageToImage] = map[string][]*RemoteAIWorker{
		"model1": {worker},
	}

	stream.sendFunc = func(job *net.NotifyAIJob) error {
		go func() {
			manager.aiResult(&RemoteAIWorkerResult{
				JobType: job.Type,
				TaskID:  job.TaskID,
				Bytes:   []byte(`{"images": [{"url": "transformed-image-url"}]}`),
			})
		}()
		return nil
	}
	file := &openapi_types.File{}
	file.InitFromBytes([]byte("fake image data"), "test-image.jpg")
	req := aiworker.ImageToImageMultipartRequestBody{
		ModelId: stringPtr("model1"),
		Image:   *file,
		Prompt:  "transform this image",
	}

	res, err := manager.ImageToImage(context.Background(), req)

	require.NoError(t, err)
	require.NotNil(t, res)
	assert.Len(t, res.Images, 1)
	assert.Equal(t, "transformed-image-url", res.Images[0].Url)
}

func TestAudioToText(t *testing.T) {
	manager := NewRemoteAIWorkerManager()
	stream := &mockStream{}
	worker := NewRemoteAIWorker(manager, stream, "test-addr", nil)

	manager.remoteWorkers[Capability_AudioToText] = map[string][]*RemoteAIWorker{
		"model1": {worker},
	}

	stream.sendFunc = func(job *net.NotifyAIJob) error {
		go func() {
			manager.aiResult(&RemoteAIWorkerResult{
				JobType: job.Type,
				TaskID:  job.TaskID,
				Bytes:   []byte(`{"text": "transcribed text"}`),
			})
		}()
		return nil
	}

	file := &openapi_types.File{}
	file.InitFromBytes([]byte("fake audio data"), "test-audio.wav")
	req := aiworker.AudioToTextMultipartRequestBody{
		ModelId: stringPtr("model1"),
		Audio:   *file,
	}

	res, err := manager.AudioToText(context.Background(), req)

	require.NoError(t, err)
	require.NotNil(t, res)
	assert.Equal(t, "transcribed text", res.Text)
}

func TestImageToVideo(t *testing.T) {
	manager := NewRemoteAIWorkerManager()
	stream := &mockStream{}
	worker := NewRemoteAIWorker(manager, stream, "test-addr", nil)

	manager.remoteWorkers[Capability_ImageToVideo] = map[string][]*RemoteAIWorker{
		"model1": {worker},
	}

	stream.sendFunc = func(job *net.NotifyAIJob) error {
		go func() {
			manager.aiResult(&RemoteAIWorkerResult{
				JobType: job.Type,
				TaskID:  job.TaskID,
				Bytes:   []byte(`{"frames": [[{"url": "generated-video-url"}]]}`),
			})
		}()
		return nil
	}

	file := &openapi_types.File{}
	file.InitFromBytes([]byte("fake image data"), "test-image.jpg")
	req := aiworker.ImageToVideoMultipartRequestBody{
		ModelId: stringPtr("model1"),
		Image:   *file,
	}

	res, err := manager.ImageToVideo(context.Background(), req)

	require.NoError(t, err)
	require.NotNil(t, res)
	assert.Len(t, res.Frames, 1)
	assert.Equal(t, "generated-video-url", res.Frames[0][0].Url)
}

func TestUpscale(t *testing.T) {
	manager := NewRemoteAIWorkerManager()
	stream := &mockStream{}
	worker := NewRemoteAIWorker(manager, stream, "test-addr", nil)

	manager.remoteWorkers[Capability_Upscale] = map[string][]*RemoteAIWorker{
		"model1": {worker},
	}

	stream.sendFunc = func(job *net.NotifyAIJob) error {
		go func() {
			manager.aiResult(&RemoteAIWorkerResult{
				JobType: job.Type,
				TaskID:  job.TaskID,
				Bytes:   []byte(`{"images": [{"url": "upscaled-image-url"}]}`),
			})
		}()
		return nil
	}

	file := &openapi_types.File{}
	file.InitFromBytes([]byte("fake image data"), "test-image.jpg")
	req := aiworker.UpscaleMultipartRequestBody{
		ModelId: stringPtr("model1"),
		Image:   *file,
		Prompt:  "upscale 2x",
	}

	res, err := manager.Upscale(context.Background(), req)

	require.NoError(t, err)
	require.NotNil(t, res)
	assert.Len(t, res.Images, 1)
	assert.Equal(t, "upscaled-image-url", res.Images[0].Url)
}

func intPtr(i int) *int {
	return &i
}

func stringPtr(s string) *string {
	return &s
}
