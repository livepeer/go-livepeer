package pm

import (
	"math/big"
	"sync"
	"time"

	ethcommon "github.com/ethereum/go-ethereum/common"
)

// unixNow returns the current unix time
// This is a wrapper function that can be stubbed in tests
var unixNow = func() int64 {
	return time.Now().Unix()
}

// FloatMonitor is an interface that describes methods used to
// monitor a sender's max float which is based on the sender's available reserve
type FloatMonitor interface {
	// Add adds amount to the max float for addr
	Add(addr ethcommon.Address, amount *big.Int) error
	// Sub subtracts amount from the max float for addr
	Sub(addr ethcommon.Address, amount *big.Int) error
	// MaxFloat returns the current max float for addr
	MaxFloat(addr ethcommon.Address) (*big.Int, error)
}

type item struct {
	val        *big.Int
	lastAccess int64
}

type floatMonitor struct {
	claimant        ethcommon.Address
	cleanupInterval time.Duration
	ttl             int

	mu        sync.RWMutex
	maxFloats map[ethcommon.Address]*item

	broker Broker
}

// NewFloatMonitor returns a new FloatMonitor
func NewFloatMonitor(claimant ethcommon.Address, broker Broker, cleanupInterval time.Duration, ttl int) FloatMonitor {
	fm := &floatMonitor{
		claimant:        claimant,
		cleanupInterval: cleanupInterval,
		ttl:             ttl,
		broker:          broker,
		maxFloats:       make(map[ethcommon.Address]*item),
	}

	ticker := time.NewTicker(fm.cleanupInterval)

	// Start cleanup worker
	// Every cleanupInterval the worker will remove itmes
	// that have exceeded their ttl
	go func() {
		for {
			<-ticker.C
			fm.cleanup()
		}
	}()

	return fm
}

func (fm *floatMonitor) Add(addr ethcommon.Address, amount *big.Int) error {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	maxFloat, err := fm.maxFloat(addr)
	if err != nil {
		return err
	}

	fm.maxFloats[addr].val = new(big.Int).Add(maxFloat, amount)

	return nil
}

func (fm *floatMonitor) Sub(addr ethcommon.Address, amount *big.Int) error {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	maxFloat, err := fm.maxFloat(addr)
	if err != nil {
		return err
	}

	fm.maxFloats[addr].val = new(big.Int).Sub(maxFloat, amount)

	return nil
}

func (fm *floatMonitor) MaxFloat(addr ethcommon.Address) (*big.Int, error) {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	return fm.maxFloat(addr)
}

func (fm *floatMonitor) maxFloat(addr ethcommon.Address) (*big.Int, error) {
	// Request max float and cache it for addr if it
	// is not already cached
	if fm.maxFloats[addr] == nil {
		maxFloat, err := fm.broker.ClaimableReserve(addr, fm.claimant)
		if err != nil {
			return nil, err
		}

		fm.maxFloats[addr] = &item{
			val: maxFloat,
		}
	}

	fm.maxFloats[addr].lastAccess = unixNow()

	return fm.maxFloats[addr].val, nil
}

func (fm *floatMonitor) cleanup() {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	for k, v := range fm.maxFloats {
		if unixNow()-v.lastAccess > int64(fm.ttl) {
			delete(fm.maxFloats, k)
		}
	}
}
