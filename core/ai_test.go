package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/livepeer/go-livepeer/ai/worker"

	"github.com/stretchr/testify/assert"
)

func TestPipelineToCapability(t *testing.T) {
	cap, err := PipelineToCapability("live-video-to-video")
	assert.Nil(t, err)
	assert.Equal(t, cap, Capability_LiveVideoToVideo)

	cap, err = PipelineToCapability("i-love-tests")
	assert.Error(t, err)
	assert.Equal(t, cap, Capability_Unused)

	// removed batch pipelines no longer resolve to a capability
	cap, err = PipelineToCapability("text-to-image")
	assert.Error(t, err)
	assert.Equal(t, cap, Capability_Unused)
}

func TestCheckAICapacity(t *testing.T) {
	n, _ := NewLivepeerNode(nil, "", nil)
	o := NewOrchestrator(n, nil)
	wkr := stubAIWorker{}
	n.Capabilities = createAIWorkerCapabilities()
	n.AIWorker = &wkr
	// Test when local AI worker has capacity: live-video-to-video defers to the worker
	hasCapacity, releaseCapacity := o.CheckAICapacity("live-video-to-video", "livepeer/model1")
	assert.True(t, hasCapacity)
	assert.Nil(t, releaseCapacity)

	// Test when no local AI worker is configured
	o.node.AIWorker = nil
	hasCapacity, releaseCapacity = o.CheckAICapacity("live-video-to-video", "livepeer/model1")
	assert.False(t, hasCapacity)
	assert.Nil(t, releaseCapacity)
}

func TestReserveAICapability(t *testing.T) {
	n, _ := NewLivepeerNode(nil, "", nil)
	n.Capabilities = createAIWorkerCapabilities()

	pipeline := "live-video-to-video"
	modelID := "livepeer/model2"

	// Add AI capability and model
	caps := NewCapabilities(DefaultCapabilities(), nil)
	caps.SetPerCapabilityConstraints(PerCapabilityConstraints{
		Capability_LiveVideoToVideo: {
			Models: ModelConstraints{
				modelID: {Warm: true, Capacity: 2},
			},
		},
	})
	n.AddAICapabilities(caps)

	// Reserve AI capability
	err := n.ReserveAICapability(pipeline, modelID)
	assert.Nil(t, err)

	// Check capacity is reduced
	cap := n.Capabilities.constraints.perCapability[Capability_LiveVideoToVideo]
	assert.Equal(t, 1, cap.Models[modelID].Capacity)

	// Reserve AI capability again
	err = n.ReserveAICapability(pipeline, modelID)
	assert.Nil(t, err)

	// Check capacity is further reduced
	cap = n.Capabilities.constraints.perCapability[Capability_LiveVideoToVideo]
	assert.Equal(t, 0, cap.Models[modelID].Capacity)

	// Reserve AI capability when capacity is already zero
	err = n.ReserveAICapability(pipeline, modelID)
	assert.NotNil(t, err)
	assert.EqualError(t, err, fmt.Sprintf("failed to reserve AI capability capacity, model capacity is 0 pipeline=%v modelID=%v", pipeline, modelID))

	// Reserve AI capability for non-existent pipeline
	err = n.ReserveAICapability("invalid-pipeline", modelID)
	assert.NotNil(t, err)
	assert.EqualError(t, err, "pipeline not available")

	// Reserve AI capability for non-existent model
	err = n.ReserveAICapability(pipeline, "invalid-model")
	assert.NotNil(t, err)
	assert.EqualError(t, err, fmt.Sprintf("failed to reserve AI capability capacity, model does not exist pipeline=%v modelID=invalid-model", pipeline))
}

func createAIWorkerCapabilities() *Capabilities {
	//create capabilities and constraints the ai worker sends to orch
	constraints := make(PerCapabilityConstraints)
	constraints[Capability_LiveVideoToVideo] = &CapabilityConstraints{Models: make(ModelConstraints)}
	constraints[Capability_LiveVideoToVideo].Models["livepeer/model1"] = &ModelConstraint{Warm: true, Capacity: 2}
	caps := NewCapabilities(DefaultCapabilities(), MandatoryOCapabilities())
	caps.SetPerCapabilityConstraints(constraints)
	caps.version = "1.0"
	return caps
}

type stubAIWorker struct{}

func (a *stubAIWorker) GetLiveAICapacity(pipeline, modelID string) worker.Capacity {
	return worker.Capacity{}
}

func (a *stubAIWorker) LiveVideoToVideo(ctx context.Context, req worker.GenLiveVideoToVideoJSONRequestBody) (*worker.LiveVideoToVideoResponse, error) {
	return &worker.LiveVideoToVideoResponse{}, nil
}

func (a *stubAIWorker) Warm(ctx context.Context, arg1, arg2 string, endpoint worker.RunnerEndpoint, flags worker.OptimizationFlags) error {
	return nil
}

func (a *stubAIWorker) Stop(ctx context.Context) error {
	return nil
}

func (a *stubAIWorker) HasCapacity(pipeline, modelID string) bool {
	return true
}

func (a *stubAIWorker) EnsureImageAvailable(ctx context.Context, pipeline string, modelID string) error {
	return nil
}

func (a *stubAIWorker) HardwareInformation() []worker.HardwareInformation {
	return nil
}

func (a *stubAIWorker) Version() []worker.Version {
	return nil
}

// Utility function to create a temporary file for file-based configurations
func mockFile(t *testing.T, content string) string {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "config.json")
	err := os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to write mock file: %v", err)
	}
	return filePath
}

func TestParseAIModelConfigs(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		fileData    string
		expected    []AIModelConfig
		expectedErr string
	}{{
		name:  "Valid Inline String Config",
		input: "pipeline1:model1:true,pipeline2:model2:false",
		expected: []AIModelConfig{
			{Pipeline: "pipeline1", ModelID: "model1", Warm: true},
			{Pipeline: "pipeline2", ModelID: "model2", Warm: false},
		},
	},
		{
			name:        "Invalid Inline String Config Missing Parts",
			input:       "pipeline1:model1",
			expectedErr: "invalid AI model config expected <pipeline>:<model_id>:<warm>",
		},
		{
			name:     "Valid File-Based Config",
			fileData: `[{"pipeline": "pipeline1", "model_id": "model1", "warm": true}, {"pipeline": "pipeline2", "model_id": "model2", "warm": false}]`,
			expected: []AIModelConfig{
				{Pipeline: "pipeline1", ModelID: "model1", Warm: true},
				{Pipeline: "pipeline2", ModelID: "model2", Warm: false},
			},
		},
		{
			name:        "Invalid File Config Corrupted JSON",
			fileData:    `[{"pipeline": "pipeline1", "model_id": "model1", "warm": true`,
			expectedErr: "unexpected end of JSON input",
		},
		{
			name:        "File Not Found",
			input:       "nonexistent.json",
			expectedErr: "invalid AI model config expected <pipeline>:<model_id>:<warm>",
		},
		{
			name:        "Invalid Boolean Value in Inline String Config",
			input:       "pipeline1:model1:invalid_bool",
			expectedErr: "strconv.ParseBool: parsing \"invalid_bool\": invalid syntax",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result []AIModelConfig
			var err error

			// Mock file handling if fileData is provided
			if tt.fileData != "" {
				mockFilePath := mockFile(t, tt.fileData)
				result, err = ParseAIModelConfigs(mockFilePath)
			} else {
				result, err = ParseAIModelConfigs(tt.input)
			}

			// Verify error messages match
			assert := assert.New(t)
			if tt.expectedErr != "" {
				assert.Equal(err.Error(), tt.expectedErr)
				assert.Empty(result, err)
			} else {
				assert.Empty(err)
				assert.Equal(tt.expected, result)
			}
		})
	}
}
