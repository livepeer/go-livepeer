package core

import (
	"math/big"
	"sync"
	"time"
)

// Balances holds credit balances on a per-stream basis
type Balances struct {
	balances map[ManifestID]*balance
	mtx      sync.RWMutex
	ttl      time.Duration
	quit     chan struct{}
}

type balance struct {
	lastUpdate time.Time // Unix time since last update
	amount     *big.Rat  // Balance represented as a big.Rat
}

// NewBalances creates a Balances instance with the given ttl
func NewBalances(ttl time.Duration) *Balances {
	return &Balances{
		balances: make(map[ManifestID]*balance),
		ttl:      ttl,
		quit:     make(chan struct{}),
	}
}

// Credit adds an an amount to the balance for a ManifestID
func (b *Balances) Credit(id ManifestID, amount *big.Rat) {
	b.mtx.Lock()
	defer b.mtx.Unlock()
	if b.balances[id] == nil {
		b.balances[id] = &balance{amount: big.NewRat(0, 1)}
	}
	b.balances[id].amount.Add(b.balances[id].amount, amount)
	b.balances[id].lastUpdate = time.Now()
}

// Debit substracts an amount from the balance for a ManifestID
func (b *Balances) Debit(id ManifestID, amount *big.Rat) {
	b.mtx.Lock()
	defer b.mtx.Unlock()
	if b.balances[id] == nil {
		b.balances[id] = &balance{amount: big.NewRat(0, 1)}
	}
	b.balances[id].amount.Sub(b.balances[id].amount, amount)
	b.balances[id].lastUpdate = time.Now()
}

// Balance retrieves the current balance for a ManifestID
func (b *Balances) Balance(id ManifestID) *big.Rat {
	b.mtx.RLock()
	defer b.mtx.RUnlock()
	if b.balances[id] == nil {
		return nil
	}
	return b.balances[id].amount
}

func (b *Balances) cleanup() {
	for id, balance := range b.balances {
		b.mtx.Lock()
		if int64(time.Since(balance.lastUpdate)) > int64(b.ttl) {
			delete(b.balances, id)
		}
		b.mtx.Unlock()
	}
}

// StartCleanup is a state flushing method to clean up the balances mapping
func (b *Balances) StartCleanup() {
	ticker := time.NewTicker(b.ttl)
	for {
		select {
		case <-ticker.C:
			b.cleanup()
		case <-b.quit:
			return
		}
	}
}

// StopCleanup stops the cleanup loop for Balances
func (b *Balances) StopCleanup() {
	close(b.quit)
}
