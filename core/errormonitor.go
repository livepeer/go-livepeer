package core

import (
	"sync"

	ethcommon "github.com/ethereum/go-ethereum/common"
)

type errorMonitor struct {
	mu          sync.RWMutex
	maxErrCount int
	errCount    map[ethcommon.Address]int
}

func NewErrorMonitor(maxErrCount int) *errorMonitor {
	return &errorMonitor{
		maxErrCount: maxErrCount,
		errCount:    make(map[ethcommon.Address]int),
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
	for s := range em.errCount {
		em.ClearErrCount(s)
	}
}
