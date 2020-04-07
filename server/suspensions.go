package server

import (
	"sync"
)

// suspender is a list that keep track of suspender orchestrators
// and the count until which they are suspended
type suspender struct {
	mu    sync.Mutex
	list  map[string]int // list of orchestrator => refresh count at which the orchestrator is no longer suspended
	count int
}

// newSuspender returns the pointer to a new Suspender instance
func newSuspender() *suspender {
	return &suspender{
		list: make(map[string]int),
	}
}

// suspend an orchestrator for 'penalty' refreshes
func (s *suspender) suspend(orch string, penalty int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.list[orch] += penalty
}

// Suspended returns a non-zero value if the orchestrator is suspended
// 'orch' is the service URI of the orchestrator
// The value returned is the suspension penalty associated with the orchestrator whereby lower is better
func (s *suspender) Suspended(orch string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.list[orch] < s.count {
		delete(s.list, orch)
	}
	return s.list[orch]
}

// signalRefresh increases Suspender.count
func (s *suspender) signalRefresh() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.count++
}
