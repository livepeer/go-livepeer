package server

import (
	"testing"

	"github.com/livepeer/go-livepeer/net"
	"github.com/stretchr/testify/assert"
)

func makeGPU(name string, memoryTotal int64, major, minor uint32) *net.GPUComputeInfo {
	return &net.GPUComputeInfo{
		Id:          name + "-0",
		Name:        name,
		MemoryTotal: memoryTotal,
		Major:       major,
		Minor:       minor,
	}
}

func makeHardware(gpus ...*net.GPUComputeInfo) []*net.HardwareInformation {
	gpuMap := make(map[string]*net.GPUComputeInfo)
	for i, gpu := range gpus {
		key := gpu.Id
		if key == "" {
			key = string(rune('0' + i))
		}
		gpuMap[key] = gpu
	}
	return []*net.HardwareInformation{
		{
			Pipeline: "text-to-image",
			ModelId:  "test-model",
			GpuInfo:  gpuMap,
		},
	}
}

const gb = 1024 * 1024 * 1024

func TestMatchesHardware_NilRequirements(t *testing.T) {
	assert.True(t, MatchesHardware(nil, nil))
	assert.True(t, MatchesHardware(makeHardware(makeGPU("H100", 80*gb, 9, 0)), nil))
}

func TestMatchesHardware_NoGPUInfo(t *testing.T) {
	// Orchestrators with no GPU info should pass (backwards compatibility).
	req := &HardwareRequirements{MinVRAMPerGPU: 40 * gb}
	assert.True(t, MatchesHardware(nil, req))
	assert.True(t, MatchesHardware([]*net.HardwareInformation{}, req))
	assert.True(t, MatchesHardware([]*net.HardwareInformation{
		{Pipeline: "test", GpuInfo: map[string]*net.GPUComputeInfo{}},
	}, req))
}

func TestMatchesHardware_MinVRAM(t *testing.T) {
	h100 := makeHardware(makeGPU("NVIDIA H100", 80*gb, 9, 0))
	rtx4090 := makeHardware(makeGPU("NVIDIA RTX 4090", 24*gb, 8, 9))

	// 40GB min - H100 passes, RTX 4090 fails.
	req := &HardwareRequirements{MinVRAMPerGPU: 40 * gb}
	assert.True(t, MatchesHardware(h100, req))
	assert.False(t, MatchesHardware(rtx4090, req))

	// 16GB min - both pass.
	req.MinVRAMPerGPU = 16 * gb
	assert.True(t, MatchesHardware(h100, req))
	assert.True(t, MatchesHardware(rtx4090, req))
}

func TestMatchesHardware_IncludeGPUs(t *testing.T) {
	h100 := makeHardware(makeGPU("NVIDIA H100", 80*gb, 9, 0))
	a100 := makeHardware(makeGPU("NVIDIA A100", 40*gb, 8, 0))
	rtx4090 := makeHardware(makeGPU("NVIDIA RTX 4090", 24*gb, 8, 9))

	req := &HardwareRequirements{
		IncludeGPUs: []string{"H100", "A100"},
	}
	assert.True(t, MatchesHardware(h100, req))
	assert.True(t, MatchesHardware(a100, req))
	assert.False(t, MatchesHardware(rtx4090, req))
}

func TestMatchesHardware_ExcludeGPUs(t *testing.T) {
	h100 := makeHardware(makeGPU("NVIDIA H100", 80*gb, 9, 0))
	v100 := makeHardware(makeGPU("NVIDIA V100", 16*gb, 7, 0))
	k80 := makeHardware(makeGPU("NVIDIA K80", 12*gb, 3, 7))

	req := &HardwareRequirements{
		ExcludeGPUs: []string{"K80", "V100"},
	}
	assert.True(t, MatchesHardware(h100, req))
	assert.False(t, MatchesHardware(v100, req))
	assert.False(t, MatchesHardware(k80, req))
}

func TestMatchesHardware_IncludeAndExclude(t *testing.T) {
	// Include A100 but exclude A100 40GB variant (by name).
	a100_80 := makeHardware(makeGPU("NVIDIA A100 80GB", 80*gb, 8, 0))
	a100_40 := makeHardware(makeGPU("NVIDIA A100 40GB", 40*gb, 8, 0))

	req := &HardwareRequirements{
		IncludeGPUs: []string{"A100"},
		ExcludeGPUs: []string{"40GB"},
	}
	assert.True(t, MatchesHardware(a100_80, req))
	assert.False(t, MatchesHardware(a100_40, req))
}

func TestMatchesHardware_CaseInsensitive(t *testing.T) {
	h100 := makeHardware(makeGPU("NVIDIA H100 SXM", 80*gb, 9, 0))

	req := &HardwareRequirements{
		IncludeGPUs: []string{"h100"},
	}
	assert.True(t, MatchesHardware(h100, req))

	req = &HardwareRequirements{
		ExcludeGPUs: []string{"h100"},
	}
	assert.False(t, MatchesHardware(h100, req))
}

func TestMatchesHardware_ComputeCapability(t *testing.T) {
	h100 := makeHardware(makeGPU("NVIDIA H100", 80*gb, 9, 0))
	a100 := makeHardware(makeGPU("NVIDIA A100", 80*gb, 8, 0))
	v100 := makeHardware(makeGPU("NVIDIA V100", 16*gb, 7, 0))

	// Require compute 8.0+
	req := &HardwareRequirements{
		MinComputeMajor: 8,
		MinComputeMinor: 0,
	}
	assert.True(t, MatchesHardware(h100, req))
	assert.True(t, MatchesHardware(a100, req))
	assert.False(t, MatchesHardware(v100, req))

	// Require compute 8.5+ (A100 is 8.0, should fail)
	req.MinComputeMinor = 5
	assert.True(t, MatchesHardware(h100, req))
	assert.False(t, MatchesHardware(a100, req))
}

func TestMatchesHardware_MultipleGPUs(t *testing.T) {
	// Orchestrator has both H100 and V100. If we include H100, it should pass
	// because at least one GPU matches.
	hw := []*net.HardwareInformation{
		{
			Pipeline: "pipeline-a",
			GpuInfo: map[string]*net.GPUComputeInfo{
				"gpu0": makeGPU("NVIDIA V100", 16*gb, 7, 0),
			},
		},
		{
			Pipeline: "pipeline-b",
			GpuInfo: map[string]*net.GPUComputeInfo{
				"gpu1": makeGPU("NVIDIA H100", 80*gb, 9, 0),
			},
		},
	}

	req := &HardwareRequirements{IncludeGPUs: []string{"H100"}}
	assert.True(t, MatchesHardware(hw, req))

	req = &HardwareRequirements{IncludeGPUs: []string{"A100"}}
	assert.False(t, MatchesHardware(hw, req))
}

func TestMatchesHardware_CombinedRequirements(t *testing.T) {
	h100 := makeHardware(makeGPU("NVIDIA H100", 80*gb, 9, 0))
	a100_40 := makeHardware(makeGPU("NVIDIA A100", 40*gb, 8, 0))

	// Require H100 or A100, min 48GB VRAM, compute 8.0+
	req := &HardwareRequirements{
		MinVRAMPerGPU:   48 * gb,
		IncludeGPUs:     []string{"H100", "A100"},
		MinComputeMajor: 8,
	}
	assert.True(t, MatchesHardware(h100, req))
	assert.False(t, MatchesHardware(a100_40, req)) // A100 40GB < 48GB
}

func TestParseHardwareRequirements(t *testing.T) {
	// No requirements.
	assert.Nil(t, ParseHardwareRequirements(0, "", ""))

	// Only min VRAM.
	req := ParseHardwareRequirements(40, "", "")
	assert.NotNil(t, req)
	assert.Equal(t, int64(40*gb), req.MinVRAMPerGPU)
	assert.Empty(t, req.IncludeGPUs)
	assert.Empty(t, req.ExcludeGPUs)

	// Include/exclude with trimming.
	req = ParseHardwareRequirements(0, " H100 , A100 ", " K80 , V100 ")
	assert.NotNil(t, req)
	assert.Equal(t, []string{"H100", "A100"}, req.IncludeGPUs)
	assert.Equal(t, []string{"K80", "V100"}, req.ExcludeGPUs)
	assert.Equal(t, int64(0), req.MinVRAMPerGPU)

	// All combined.
	req = ParseHardwareRequirements(80, "H100", "K80")
	assert.NotNil(t, req)
	assert.Equal(t, int64(80*gb), req.MinVRAMPerGPU)
	assert.Equal(t, []string{"H100"}, req.IncludeGPUs)
	assert.Equal(t, []string{"K80"}, req.ExcludeGPUs)
}

func TestMatchesHardware_GPUDeduplication(t *testing.T) {
	// Same GPU ID reported across multiple pipelines should be counted once.
	hw := []*net.HardwareInformation{
		{
			Pipeline: "pipeline-a",
			GpuInfo: map[string]*net.GPUComputeInfo{
				"gpu0": {Id: "gpu0", Name: "NVIDIA H100", MemoryTotal: 80 * gb, Major: 9},
			},
		},
		{
			Pipeline: "pipeline-b",
			GpuInfo: map[string]*net.GPUComputeInfo{
				"gpu0": {Id: "gpu0", Name: "NVIDIA H100", MemoryTotal: 80 * gb, Major: 9},
			},
		},
	}

	req := &HardwareRequirements{IncludeGPUs: []string{"H100"}}
	assert.True(t, MatchesHardware(hw, req))

	gpus := collectGPUs(hw)
	assert.Len(t, gpus, 1, "Duplicate GPU IDs should be deduplicated")
}
