package server

import (
	"sync"
)

var (
	streamStatusStore = make(map[string]map[string]interface{})
	streamStatusMu    sync.RWMutex
)

// StoreStreamStatus updates the status for a stream
func StoreStreamStatus(streamID string, status map[string]interface{}) {
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
func GetStreamStatus(streamID string) (map[string]interface{}, bool) {
	streamStatusMu.RLock()
	defer streamStatusMu.RUnlock()
	status, exists := streamStatusStore[streamID]
	return status, exists
}
