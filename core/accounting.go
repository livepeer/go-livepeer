package core

import (
	"math/big"
	"sync"
	"time"
)

// Balance holds the credit balance for a broadcast session
type Balance struct {
	manifestID ManifestID
	balances   *Balances
}

// NewBalance returns a Balance instance
func NewBalance(manifestID ManifestID, balances *Balances) *Balance {
	return &Balance{
		manifestID: manifestID,
		balances:   balances,
	}
}

// Credit adds an amount to the balance
func (b *Balance) Credit(amount *big.Rat) {
	b.balances.Credit(b.manifestID, amount)
}

// StageUpdate prepares a balance update by reserving the current balance and returning the number of tickets
// to send with a payment, the new credit represented by the payment and the existing credit (i.e reserved balance)
func (b *Balance) StageUpdate(minCredit, ev *big.Rat) (int, *big.Rat, *big.Rat) {
	existingCredit := b.balances.Reserve(b.manifestID)

	// If the existing credit exceeds the minimum credit then no tickets are required
	// and the total payment value is 0
	if existingCredit.Cmp(minCredit) >= 0 {
		return 0, big.NewRat(0, 1), existingCredit
	}

	creditGap := new(big.Rat).Sub(minCredit, existingCredit)
	sizeRat := creditGap.Quo(creditGap, ev)
	res := sizeRat.Num()
	if !sizeRat.IsInt() {
		// If sizeRat is not an integer take the ceiling of the result of division to ensure
		// that the batch of tickets will cover the entire creditGap
		res = res.Div(res, sizeRat.Denom()).Add(res, big.NewInt(1))
	}

	size := res.Int64()

	return int(size), new(big.Rat).Mul(new(big.Rat).SetInt64(size), ev), existingCredit
}

// Clear zeros the balance
func (b *Balance) Clear() {
	delete(b.balances.balances, b.manifestID)
}

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

// Reserve zeros the balance for a ManifestID and returns the current balance
func (b *Balances) Reserve(id ManifestID) *big.Rat {
	b.mtx.Lock()
	defer b.mtx.Unlock()
	if b.balances[id] == nil {
		b.balances[id] = &balance{amount: big.NewRat(0, 1)}
	}

	amount := b.balances[id].amount
	b.balances[id].amount = big.NewRat(0, 1)

	return amount
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
