package monitor

import (
	"sync"
)

var (
	// pipelineStatusMap stores the latest pipeline status for each stream
	pipelineStatusMap = make(map[string]PipelineStatus)
	pipelineStatusMu  sync.RWMutex
)

func UpdatePipelineStatus(stream string, status PipelineStatus) {
	pipelineStatusMu.Lock()
	defer pipelineStatusMu.Unlock()
	pipelineStatusMap[stream] = status
}

func GetPipelineStatus(stream string) (PipelineStatus, bool) {
	pipelineStatusMu.RLock()
	defer pipelineStatusMu.RUnlock()
	status, exists := pipelineStatusMap[stream]
	return status, exists
}

func DeletePipelineStatus(stream string) {
	pipelineStatusMu.Lock()
	defer pipelineStatusMu.Unlock()
	delete(pipelineStatusMap, stream)
}
