package server

import (
	"strings"

	"github.com/livepeer/go-livepeer/net"
)

// HardwareRequirements specifies hardware constraints for orchestrator
// selection, similar to chutes.ai's NodeSelector. These requirements are
// used gateway-side to filter orchestrators based on their reported GPU
// hardware before the selection algorithm runs.
type HardwareRequirements struct {
	// MinVRAMPerGPU is the minimum total VRAM per GPU in bytes.
	MinVRAMPerGPU int64

	// IncludeGPUs is a list of GPU name substrings to allow (e.g., "H100", "A100").
	// If non-empty, only orchestrators with at least one matching GPU are considered.
	IncludeGPUs []string

	// ExcludeGPUs is a list of GPU name substrings to reject (e.g., "K80", "V100").
	ExcludeGPUs []string

	// MinComputeMajor is the minimum required CUDA compute capability major version.
	MinComputeMajor uint32

	// MinComputeMinor is the minimum required CUDA compute capability minor version
	// (only compared when major version equals MinComputeMajor).
	MinComputeMinor uint32
}

// MatchesHardware checks whether an orchestrator's reported hardware satisfies
// the given requirements. Returns true if requirements is nil (no filtering).
// When no GPU info is reported by the orchestrator, the check passes to avoid
// rejecting orchestrators that haven't upgraded to report hardware yet.
func MatchesHardware(hardware []*net.HardwareInformation, req *HardwareRequirements) bool {
	if req == nil {
		return true
	}

	// Collect all unique GPUs across all pipelines/models.
	gpus := collectGPUs(hardware)

	// If the orchestrator reports no GPU info, pass the filter to avoid
	// excluding orchestrators that haven't upgraded to report hardware.
	if len(gpus) == 0 {
		return true
	}

	// At least one GPU must satisfy all requirements.
	for _, gpu := range gpus {
		if gpuMatchesRequirements(gpu, req) {
			return true
		}
	}
	return false
}

// gpuMatchesRequirements checks if a single GPU satisfies all requirements.
func gpuMatchesRequirements(gpu *net.GPUComputeInfo, req *HardwareRequirements) bool {
	// Check exclude list first.
	if len(req.ExcludeGPUs) > 0 && matchesAny(gpu.Name, req.ExcludeGPUs) {
		return false
	}

	// Check include list: if specified, GPU name must match at least one entry.
	if len(req.IncludeGPUs) > 0 && !matchesAny(gpu.Name, req.IncludeGPUs) {
		return false
	}

	// Check minimum VRAM.
	if req.MinVRAMPerGPU > 0 && gpu.MemoryTotal < req.MinVRAMPerGPU {
		return false
	}

	// Check minimum compute capability.
	if req.MinComputeMajor > 0 {
		if gpu.Major < req.MinComputeMajor {
			return false
		}
		if gpu.Major == req.MinComputeMajor && gpu.Minor < req.MinComputeMinor {
			return false
		}
	}

	return true
}

// collectGPUs deduplicates GPUs across all HardwareInformation entries by GPU ID.
func collectGPUs(hardware []*net.HardwareInformation) []*net.GPUComputeInfo {
	seen := make(map[string]bool)
	var gpus []*net.GPUComputeInfo
	for _, hw := range hardware {
		for id, gpu := range hw.GpuInfo {
			key := id
			if gpu.Id != "" {
				key = gpu.Id
			}
			if !seen[key] {
				seen[key] = true
				gpus = append(gpus, gpu)
			}
		}
	}
	return gpus
}

// matchesAny returns true if gpuName contains any of the given substrings
// (case-insensitive).
func matchesAny(gpuName string, patterns []string) bool {
	nameLower := strings.ToLower(gpuName)
	for _, p := range patterns {
		if strings.Contains(nameLower, strings.ToLower(p)) {
			return true
		}
	}
	return false
}

// ParseHardwareRequirements creates a HardwareRequirements from individual
// flag values. Returns nil if no requirements are specified.
func ParseHardwareRequirements(minVRAMGB int, includeGPUs, excludeGPUs string) *HardwareRequirements {
	include := splitAndTrim(includeGPUs)
	exclude := splitAndTrim(excludeGPUs)

	if minVRAMGB <= 0 && len(include) == 0 && len(exclude) == 0 {
		return nil
	}

	return &HardwareRequirements{
		MinVRAMPerGPU: int64(minVRAMGB) * 1024 * 1024 * 1024, // GB to bytes
		IncludeGPUs:   include,
		ExcludeGPUs:   exclude,
	}
}

func splitAndTrim(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	var result []string
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
