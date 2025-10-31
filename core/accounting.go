package core

import (
	"context"
	"math/big"
	"sync"
	"time"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/livepeer/go-livepeer/clog"
)

// Balance holds the credit balance for a broadcast session
type Balance struct {
	addr       ethcommon.Address
	manifestID ManifestID
	balances   *AddressBalances
}

// NewBalance returns a Balance instance
func NewBalance(addr ethcommon.Address, manifestID ManifestID, balances *AddressBalances) *Balance {
	return &Balance{
		addr:       addr,
		manifestID: manifestID,
		balances:   balances,
	}
}

// Credit adds an amount to the balance
func (b *Balance) Credit(amount *big.Rat) {
	b.balances.Credit(b.addr, b.manifestID, amount)
}

// StageUpdate prepares a balance update by reserving the current balance and returning the number of tickets
// to send with a payment, the new credit represented by the payment and the existing credit (i.e reserved balance)
func (b *Balance) StageUpdate(minCredit, ev *big.Rat) (int, *big.Rat, *big.Rat) {
	existingCredit := b.balances.Reserve(b.addr, b.manifestID)

	// If the existing credit exceeds the minimum credit then no tickets are required
	// and the total payment value is 0
	if existingCredit.Cmp(minCredit) >= 0 {
		return 0, big.NewRat(0, 1), existingCredit
	}

	creditGap := new(big.Rat).Sub(minCredit, existingCredit)
	if ev == nil || ev.Cmp(big.NewRat(0, 1)) == 0 {
		clog.Warningf(context.Background(), "Error calculating tickets: ev is nil or zero")
		return 0, big.NewRat(0, 1), existingCredit
	}
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

func (b *Balance) Balance() *big.Rat {
	return b.balances.balancesForAddr(b.addr).Balance(b.manifestID)
}

// AddressBalances holds credit balances for ETH addresses
type AddressBalances struct {
	balances    map[ethcommon.Address]*Balances
	mtx         sync.Mutex
	sharedBaltx sync.Mutex
	ttl         time.Duration
}

// NewAddressBalances creates a new AddressBalances instance
func NewAddressBalances(ttl time.Duration) *AddressBalances {
	return &AddressBalances{
		balances: make(map[ethcommon.Address]*Balances),
		ttl:      ttl,
	}
}

// Credit adds an amount to the balance for an address' ManifestID
func (a *AddressBalances) Credit(addr ethcommon.Address, id ManifestID, amount *big.Rat) {
	a.balancesForAddr(addr).Credit(id, amount)
}

// Debit subtracts an amount from the balance for an address' ManifestID
func (a *AddressBalances) Debit(addr ethcommon.Address, id ManifestID, amount *big.Rat) {
	a.balancesForAddr(addr).Debit(id, amount)
}

// Reserve zeros the balance for an address' ManifestID and returns the current balance
func (a *AddressBalances) Reserve(addr ethcommon.Address, id ManifestID) *big.Rat {
	return a.balancesForAddr(addr).Reserve(id)
}

// Balance retrieves the current balance for an address' ManifestID
func (a *AddressBalances) Balance(addr ethcommon.Address, id ManifestID) *big.Rat {
	return a.balancesForAddr(addr).Balance(id)
}

// compares expected balance with current balance and updates accordingly with the expected balance being the target
// returns the difference and if minimum balance was covered
// also returns if balance was reset to zero because expected was zero
func (a *AddressBalances) CompareAndUpdateBalance(addr ethcommon.Address, id ManifestID, expected *big.Rat, minimumBal *big.Rat) (*big.Rat, *big.Rat, bool, bool) {
	a.sharedBaltx.Lock()
	defer a.sharedBaltx.Unlock()
	current := a.balancesForAddr(addr).Balance(id)
	if current == nil {
		//create a balance of 1 to start tracking
		a.Debit(addr, id, big.NewRat(0, 1))
		current = a.balancesForAddr(addr).Balance(id)
	}
	if expected == nil {
		expected = big.NewRat(0, 1)
	}
	diff := new(big.Rat).Sub(expected, current)

	if diff.Sign() > 0 {
		a.Credit(addr, id, diff)
	} else {
		a.Debit(addr, id, new(big.Rat).Abs(diff))
	}

	var resetToZero bool
	if expected.Sign() == 0 {
		a.Debit(addr, id, current)

		resetToZero = true
	}

	//get updated balance after changes
	current = a.balancesForAddr(addr).Balance(id)

	var minimumBalCovered bool
	if current.Cmp(minimumBal) >= 0 {
		minimumBalCovered = true
	}

	return current, diff, minimumBalCovered, resetToZero
}

// StopCleanup stops the cleanup loop for all balances
func (a *AddressBalances) StopCleanup() {
	a.mtx.Lock()
	defer a.mtx.Unlock()

	for _, b := range a.balances {
		b.StopCleanup()
	}
}

func (a *AddressBalances) balancesForAddr(addr ethcommon.Address) *Balances {
	a.mtx.Lock()
	defer a.mtx.Unlock()

	if _, ok := a.balances[addr]; !ok {
		b := NewBalances(a.ttl)
		go b.StartCleanup()

		a.balances[addr] = b
	}

	return a.balances[addr]
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
	fixedPrice *big.Rat  // Fixed price for the session
}

// NewBalances creates a Balances instance with the given ttl
func NewBalances(ttl time.Duration) *Balances {
	return &Balances{
		balances: make(map[ManifestID]*balance),
		ttl:      ttl,
		quit:     make(chan struct{}),
	}
}

// Credit adds an amount to the balance for a ManifestID
func (b *Balances) Credit(id ManifestID, amount *big.Rat) {
	b.mtx.Lock()
	defer b.mtx.Unlock()
	if b.balances[id] == nil {
		b.balances[id] = &balance{amount: big.NewRat(0, 1)}
	}
	b.balances[id].amount.Add(b.balances[id].amount, amount)
	b.balances[id].lastUpdate = time.Now()
}

// Debit subtracts an amount from the balance for a ManifestID
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

// FixedPrice retrieves the price fixed the given session
func (b *Balances) FixedPrice(id ManifestID) *big.Rat {
	b.mtx.RLock()
	defer b.mtx.RUnlock()
	if b.balances[id] == nil {
		return nil
	}
	return b.balances[id].fixedPrice
}

// SetFixedPrice sets fixed price for the given session
func (b *Balances) SetFixedPrice(id ManifestID, fixedPrice *big.Rat) {
	b.mtx.Lock()
	defer b.mtx.Unlock()
	if b.balances[id] == nil {
		b.balances[id] = &balance{amount: big.NewRat(0, 1)}
	}
	b.balances[id].fixedPrice = fixedPrice
	b.balances[id].lastUpdate = time.Now()
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
