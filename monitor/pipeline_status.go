package monitor

import (
	"sync"
	"time"
)

// StreamStatus stores the latest status for each stream
type StreamStatus struct {
	Status          map[string]interface{}
	LastRestartLogs interface{}
	LastParams      interface{}
	UpdatedAt       time.Time
}

var (
	streamStatusStore = make(map[string]*StreamStatus)
	streamStatusMu    sync.RWMutex
)

// StoreStreamStatus updates the status for a stream
func StoreStreamStatus(streamID string, status *StreamStatus) {
	streamStatusMu.Lock()
	streamStatusStore[streamID] = status
	streamStatusMu.Unlock()
}

// ClearStreamStatus removes a stream's status from the store
func ClearStreamStatus(streamID string) {
	streamStatusMu.Lock()
	delete(streamStatusStore, streamID)
	streamStatusMu.Unlock()
}

// GetStreamStatus returns the current status for a stream
func GetStreamStatus(streamID string) (*StreamStatus, bool) {
	streamStatusMu.RLock()
	defer streamStatusMu.RUnlock()
	status, exists := streamStatusStore[streamID]
	return status, exists
}
