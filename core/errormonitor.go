package core

import (
	"sync"

	ethcommon "github.com/ethereum/go-ethereum/common"
)

type errorMonitor struct {
	mu             sync.Mutex
	maxErrCount    int
	errCount       map[ethcommon.Address]int
	gasPriceUpdate chan struct{}
}

// NewErrorMonitor returns a new errorMonitor instance
func NewErrorMonitor(maxErrCount int, gasPriceUpdate chan struct{}) *errorMonitor {
	return &errorMonitor{
		maxErrCount:    maxErrCount,
		errCount:       make(map[ethcommon.Address]int),
		gasPriceUpdate: gasPriceUpdate,
	}
}

// AcceptErr checks if a sender has reached the max error count
// returns false if no more errors can be accepted
// returns true and increments the error count when smaller than the max error count
func (em *errorMonitor) AcceptErr(sender ethcommon.Address) bool {
	em.mu.Lock()
	defer em.mu.Unlock()

	if em.errCount[sender] >= em.maxErrCount {
		return false
	}
	em.errCount[sender]++
	return true
}

// ClearErrCount zeroes the error count for a sender
func (em *errorMonitor) ClearErrCount(sender ethcommon.Address) {
	em.mu.Lock()
	defer em.mu.Unlock()
	em.errCount[sender] = 0
}

// ResetErrCounts clears error counts for all senders
func (em *errorMonitor) resetErrCounts() {
	em.mu.Lock()
	defer em.mu.Unlock()
	// Init a fresh map
	em.errCount = make(map[ethcommon.Address]int)
}

// StartGasPriceUpdateLoop initiates a loop that runs a worker
// to reset the errCount for senders every time a gas price change
// notification is received
func (em *errorMonitor) StartGasPriceUpdateLoop() {
	for range em.gasPriceUpdate {
		em.resetErrCounts()
	}
}

// AcceptableError is an interface that describes methods for a payment related error that
// may be acceptable depending on the type of underlying error
type AcceptableError interface {
	error

	// Acceptable returns whether the error is acceptable
	Acceptable() bool
}

type acceptableError struct {
	err        error
	acceptable bool
}

func newAcceptableError(err error, acceptable bool) *acceptableError {
	return &acceptableError{
		err:        err,
		acceptable: acceptable,
	}
}

// Error returns the underlying error as a string
func (re *acceptableError) Error() string {
	return re.err.Error()
}

// Acceptable returns whether the error is acceptable
func (re *acceptableError) Acceptable() bool {
	return re.acceptable
}
