package server

// Keeps track of spawned goroutines we want to wait for completion
// Uses channels that are closed on goroutine side
type SpawnedGoroutines struct {
	signals []chan int
}

// Create new signal to give to new goroutine
func (s *SpawnedGoroutines) Signal() chan int {
	signal := make(chan int)
	s.signals = append(s.signals, signal)
	return signal
}

// Wait on all signals/channels
func (s *SpawnedGoroutines) WaitAll() {
	for i := 0; i < len(s.signals); i++ {
		<-(s.signals[i])
	}
}
