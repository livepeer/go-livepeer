package core

import (
	"sync"

	ethcommon "github.com/ethereum/go-ethereum/common"
)

type errorMonitor struct {
	mu             sync.RWMutex
	maxErrCount    int
	errCount       map[ethcommon.Address]int
	gasPriceUpdate chan struct{}
}

func NewErrorMonitor(maxErrCount int, gasPriceUpdate chan struct{}) *errorMonitor {
	return &errorMonitor{
		maxErrCount:    maxErrCount,
		errCount:       make(map[ethcommon.Address]int),
		gasPriceUpdate: gasPriceUpdate,
	}
}

func (em *errorMonitor) AcceptErr(sender ethcommon.Address) bool {
	em.mu.Lock()
	defer em.mu.Unlock()

	if em.errCount[sender] >= em.maxErrCount {
		return false
	}
	em.errCount[sender]++
	return true
}

func (em *errorMonitor) ClearErrCount(sender ethcommon.Address) {
	em.mu.Lock()
	defer em.mu.Unlock()
	em.errCount[sender] = 0
}

func (em *errorMonitor) ResetErrCounts() {
	em.mu.Lock()
	defer em.mu.Unlock()
	// Init a fresh map
	em.errCount = make(map[ethcommon.Address]int)
}

// startGasPriceUpdateLoop initiates a loop that runs a worker
// to reset the errCount for senders every time a gas price change
// notification is received
func (em *errorMonitor) startGasPriceUpdateLoop() {
	defer em.ResetErrCounts()
	for range em.gasPriceUpdate {
		em.ResetErrCounts()
	}
}

func (em *errorMonitor) Start() {
	go em.startGasPriceUpdateLoop()
}

func (em *errorMonitor) Stop() {
	close(em.gasPriceUpdate)
}
