package media

import (
	"sync"
	"sync/atomic"
)

// OutputStatsData is an opaque struct that holds output statistics for a request.
// It is concurrency-safe and only ever increases per requestID.
type OutputStatsData struct {
	lastMpegTS atomic.Int64
}

var (
	outputStatsMu sync.RWMutex
	outputStats   = make(map[string]*OutputStatsData)
)

// Returns the output statistics for the given requestID.
// If no statistics exist, a new one is created and returned.
// Returns nil if requestID is empty.
func GetOutputStats(requestID string) *OutputStatsData {
	if requestID == "" {
		return nil
	}
	outputStatsMu.Lock()
	defer outputStatsMu.Unlock()
	entry, ok := outputStats[requestID]
	if !ok {
		entry = &OutputStatsData{}
		outputStats[requestID] = entry
	}
	return entry
}

// Remove any stored output statistics. Safe to call multiple times.
func ClearOutputStats(requestID string) {
	if requestID == "" {
		return
	}
	outputStatsMu.Lock()
	defer outputStatsMu.Unlock()
	delete(outputStats, requestID)
}

// Record the latest output timestamp in 90khz MPEG-TS timebase units
func (o *OutputStatsData) UpdateLastOutputTS(ts int64) {
	if o == nil {
		return
	}
	o.lastMpegTS.Store(ts)
}

// Return the latest recorded output timestampin 90khz MPEG-TS timebase units.
func (o *OutputStatsData) GetLastOutputTS() int64 {
	if o == nil {
		return 0
	}
	return o.lastMpegTS.Load()
}
