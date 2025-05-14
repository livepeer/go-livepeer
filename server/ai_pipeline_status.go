package server

import (
	"sync"
)

type streamStatusStore struct {
	store map[string]map[string]interface{}
	mu    sync.RWMutex
}

var StreamStatusStore = streamStatusStore{store: make(map[string]map[string]interface{})}
var GatewayStatus = streamStatusStore{store: make(map[string]map[string]interface{})}

// StoreStreamStatus updates the status for a stream
func (s *streamStatusStore) Store(streamID string, status map[string]interface{}) {
	s.mu.Lock()
	s.store[streamID] = status
	s.mu.Unlock()
}

// ClearStreamStatus removes a stream's status from the store
func (s *streamStatusStore) Clear(streamID string) {
	s.mu.Lock()
	delete(s.store, streamID)
	s.mu.Unlock()
}

// GetStreamStatus returns the current status for a stream
func (s *streamStatusStore) Get(streamID string) (map[string]interface{}, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	status, exists := s.store[streamID]
	return status, exists
}

// StoreIfNotExists stores a status only if the streamID doesn't already exist or keyToCheck does not exist on the status
func (s *streamStatusStore) StoreIfNotExists(streamID string, keyToCheck string, status map[string]interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	status, exists := s.store[streamID]
	if !exists {
		s.store[streamID] = status
		return
	}
	if _, ok := status[keyToCheck]; !ok {
		s.store[streamID] = status
	}
}
