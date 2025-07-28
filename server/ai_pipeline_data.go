package server

import (
	"sync"
)

// DataSegmentStore stores data segments for SSE streaming
type DataSegmentStore struct {
	streamID string
	segments chan []byte
	mu       sync.RWMutex
	closed   bool
}

func NewDataSegmentStore(streamID string) *DataSegmentStore {
	return &DataSegmentStore{
		streamID: streamID,
		segments: make(chan []byte, 100), // Buffer up to 100 segments
	}
}

func (d *DataSegmentStore) Store(data []byte) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if d.closed {
		return
	}
	select {
	case d.segments <- data:
	default:
		// Channel is full, drop oldest segment
		select {
		case <-d.segments:
		default:
		}
		select {
		case d.segments <- data:
		default:
		}
	}
}

func (d *DataSegmentStore) Subscribe() <-chan []byte {
	return d.segments
}

func (d *DataSegmentStore) Close() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.closed {
		d.closed = true
		close(d.segments)
	}
}

// Global store for data segments by stream ID
var dataStores = make(map[string]*DataSegmentStore)
var dataStoresMu sync.RWMutex

func getDataStore(stream string) *DataSegmentStore {
	dataStoresMu.RLock()
	store, exists := dataStores[stream]
	dataStoresMu.RUnlock()
	if exists {
		return store
	}

	dataStoresMu.Lock()
	defer dataStoresMu.Unlock()
	// Double-check after acquiring write lock
	if store, exists := dataStores[stream]; exists {
		return store
	}

	store = NewDataSegmentStore(stream)
	dataStores[stream] = store
	return store
}

func removeDataStore(stream string) {
	dataStoresMu.Lock()
	defer dataStoresMu.Unlock()
	if store, exists := dataStores[stream]; exists {
		store.Close()
		delete(dataStores, stream)
	}
}
