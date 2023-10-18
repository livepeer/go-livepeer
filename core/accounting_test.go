package core

import (
	"math/big"
	"testing"
	"time"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
)

func TestBalance_Credit(t *testing.T) {
	addr := ethcommon.BytesToAddress([]byte("foo"))
	mid := ManifestID("some manifestID")
	balances := NewAddressBalances(5 * time.Second)
	defer balances.StopCleanup()

	b := NewBalance(addr, mid, balances)

	assert := assert.New(t)

	otherAddr := ethcommon.BytesToAddress([]byte("bar"))

	b.Credit(big.NewRat(5, 1))
	assert.Zero(big.NewRat(5, 1).Cmp(balances.Balance(addr, mid)))
	assert.Nil(balances.Balance(otherAddr, mid))

	b.Credit(big.NewRat(-5, 1))
	assert.Zero(big.NewRat(0, 1).Cmp(balances.Balance(addr, mid)))
	assert.Nil(balances.Balance(otherAddr, mid))

	b.Credit(big.NewRat(0, 1))
	assert.Zero(big.NewRat(0, 1).Cmp(balances.Balance(addr, mid)))
	assert.Nil(balances.Balance(otherAddr, mid))
}

func TestBalance_StageUpdate(t *testing.T) {
	addr := ethcommon.BytesToAddress([]byte("foo"))
	mid := ManifestID("some manifestID")
	balances := NewAddressBalances(5 * time.Second)
	defer balances.StopCleanup()

	b := NewBalance(addr, mid, balances)

	assert := assert.New(t)

	// Credit otherAddr and the same manifestID
	// The balance for otherAddr should not change during these tests
	otherAddr := ethcommon.BytesToAddress([]byte("bar"))
	balances.Credit(otherAddr, mid, big.NewRat(5, 1))

	// Test existing credit > minimum credit
	b.Credit(big.NewRat(2, 1))
	numTickets, newCredit, existingCredit := b.StageUpdate(big.NewRat(1, 1), nil)
	assert.Equal(0, numTickets)
	assert.Zero(big.NewRat(0, 1).Cmp(newCredit))
	assert.Zero(big.NewRat(2, 1).Cmp(existingCredit))
	assert.Zero(big.NewRat(0, 1).Cmp(balances.Balance(addr, mid)))
	assert.Zero(big.NewRat(5, 1).Cmp(balances.Balance(otherAddr, mid)))

	// Test existing credit = minimum credit
	b.Credit(big.NewRat(2, 1))
	numTickets, newCredit, existingCredit = b.StageUpdate(big.NewRat(2, 1), nil)
	assert.Equal(0, numTickets)
	assert.Zero(big.NewRat(0, 1).Cmp(newCredit))
	assert.Zero(big.NewRat(2, 1).Cmp(existingCredit))
	assert.Zero(big.NewRat(0, 1).Cmp(balances.Balance(addr, mid)))
	assert.Zero(big.NewRat(5, 1).Cmp(balances.Balance(otherAddr, mid)))

	// Test exact number of tickets covers new credit
	b.Credit(big.NewRat(1, 1))
	numTickets, newCredit, existingCredit = b.StageUpdate(big.NewRat(5, 1), big.NewRat(1, 1))
	assert.Equal(4, numTickets)
	assert.Zero(big.NewRat(4, 1).Cmp(newCredit))
	assert.Zero(big.NewRat(1, 1).Cmp(existingCredit))
	assert.Zero(big.NewRat(0, 1).Cmp(balances.Balance(addr, mid)))
	assert.Zero(big.NewRat(5, 1).Cmp(balances.Balance(otherAddr, mid)))

	// Test non-exact number of tickets covers new credit
	b.Credit(big.NewRat(1, 4))
	numTickets, newCredit, existingCredit = b.StageUpdate(big.NewRat(2, 1), big.NewRat(1, 1))
	assert.Equal(2, numTickets)
	assert.Zero(big.NewRat(2, 1).Cmp(newCredit))
	assert.Zero(big.NewRat(1, 4).Cmp(existingCredit))
	assert.Zero(big.NewRat(0, 1).Cmp(balances.Balance(addr, mid)))
	assert.Zero(big.NewRat(5, 1).Cmp(balances.Balance(otherAddr, mid)))

	// Test negative existing credit
	b.Credit(big.NewRat(-5, 1))
	numTickets, newCredit, existingCredit = b.StageUpdate(big.NewRat(2, 1), big.NewRat(1, 1))
	assert.Equal(7, numTickets)
	assert.Zero(big.NewRat(7, 1).Cmp(newCredit))
	assert.Zero(big.NewRat(-5, 1).Cmp(existingCredit))
	assert.Zero(big.NewRat(0, 1).Cmp(balances.Balance(addr, mid)))
	assert.Zero(big.NewRat(5, 1).Cmp(balances.Balance(otherAddr, mid)))

	// Test no existing credit
	numTickets, newCredit, existingCredit = b.StageUpdate(big.NewRat(2, 1), big.NewRat(1, 1))
	assert.Equal(2, numTickets)
	assert.Zero(big.NewRat(2, 1).Cmp(newCredit))
	assert.Zero(big.NewRat(0, 1).Cmp(existingCredit))
	assert.Zero(big.NewRat(0, 1).Cmp(balances.Balance(addr, mid)))
	assert.Zero(big.NewRat(5, 1).Cmp(balances.Balance(otherAddr, mid)))
}

func TestAddressBalances(t *testing.T) {
	addr1 := ethcommon.BytesToAddress([]byte("foo"))
	addr2 := ethcommon.BytesToAddress([]byte("bar"))
	mid := ManifestID("some manifestID")

	balances := NewAddressBalances(500 * time.Millisecond)

	assert := assert.New(t)

	assert.Nil(balances.Balance(addr1, mid))

	balances.Credit(addr1, mid, big.NewRat(5, 1))
	assert.Zero(big.NewRat(5, 1).Cmp(balances.Balance(addr1, mid)))

	balances.Debit(addr1, mid, big.NewRat(2, 1))
	assert.Zero(big.NewRat(3, 1).Cmp(balances.Balance(addr1, mid)))

	balances.Credit(addr2, mid, big.NewRat(5, 1))
	assert.Zero(big.NewRat(5, 1).Cmp(balances.Balance(addr2, mid)))

	reserved := balances.Reserve(addr2, mid)
	assert.Zero(big.NewRat(5, 1).Cmp(reserved))
	assert.Zero(big.NewRat(0, 1).Cmp(balances.Balance(addr2, mid)))

	// Check that we are cleaning up balances after the TTL
	time.Sleep(700 * time.Millisecond)

	assert.Nil(balances.Balance(addr1, mid))
	assert.Nil(balances.Balance(addr2, mid))

	balances.Credit(addr1, mid, big.NewRat(5, 1))
	balances.Credit(addr2, mid, big.NewRat(5, 1))

	// Check that we are not cleaning up balances after StopCleanup() is called
	balances.StopCleanup()

	time.Sleep(700 * time.Millisecond)

	assert.Zero(big.NewRat(5, 1).Cmp(balances.Balance(addr1, mid)))
	assert.Zero(big.NewRat(5, 1).Cmp(balances.Balance(addr2, mid)))
}

func TestEmptyBalances_ReturnsZeroedValues(t *testing.T) {
	mid := ManifestID("some manifest id")
	b := NewBalances(5 * time.Second)
	assert := assert.New(t)

	assert.Nil(b.Balance(mid))
	assert.Nil(b.balances[mid])
}

func TestCredit_ReturnsNewCreditBalance(t *testing.T) {
	mid := ManifestID("some manifest id")
	b := NewBalances(5 * time.Second)
	assert := assert.New(t)
	amount := big.NewRat(100, 1)

	b.Credit(mid, amount)
	assert.Zero(b.Balance(mid).Cmp(amount))
}

func TestDebitAfterCredit_SameAmount_ReturnsZero(t *testing.T) {
	mid := ManifestID("some manifest id")
	b := NewBalances(5 * time.Second)
	assert := assert.New(t)
	amount := big.NewRat(100, 1)

	b.Credit(mid, amount)
	assert.Zero(b.Balance(mid).Cmp(amount))

	b.Debit(mid, amount)
	assert.Zero(b.Balance(mid).Cmp(big.NewRat(0, 1)))
}

func TestDebitHalfOfCredit_ReturnsHalfOfCredit(t *testing.T) {
	mid := ManifestID("some manifest id")
	b := NewBalances(5 * time.Second)
	assert := assert.New(t)
	credit := big.NewRat(100, 1)
	debit := big.NewRat(50, 1)
	b.Credit(mid, credit)
	assert.Zero(b.Balance(mid).Cmp(credit))

	b.Debit(mid, debit)
	assert.Zero(b.Balance(mid).Cmp(debit))
}

func TestReserve(t *testing.T) {
	assert := assert.New(t)

	mid := ManifestID("some manifest id")
	b := NewBalances(5 * time.Second)

	// Test when entry is nil
	assert.Zero(big.NewRat(0, 1).Cmp(b.Reserve(mid)))
	assert.Zero(big.NewRat(0, 1).Cmp(b.Balance(mid)))

	// Test when entry is non-nil
	b.Credit(mid, big.NewRat(5, 1))
	assert.Zero(big.NewRat(5, 1).Cmp(b.Reserve(mid)))
	assert.Zero(big.NewRat(0, 1).Cmp(b.Balance(mid)))

	// Test when amount is negative
	b.Debit(mid, big.NewRat(5, 1))
	assert.Zero(big.NewRat(-5, 1).Cmp(b.Reserve(mid)))
	assert.Zero(big.NewRat(0, 1).Cmp(b.Balance(mid)))
}

func TestFixedPrice(t *testing.T) {
	b := NewBalances(5 * time.Second)
	id1 := ManifestID("12345")
	id2 := ManifestID("abcdef")

	// No fixed price set yet
	assert.Nil(t, b.FixedPrice(id1))

	// Set fixed price
	p := big.NewRat(1, 5)
	b.SetFixedPrice(id1, p)
	assert.Equal(t, p, b.FixedPrice(id1))

	// No fixed price for a different manifest ID
	assert.Nil(t, b.FixedPrice(id2))
}

func TestBalancesCleanup(t *testing.T) {
	b := NewBalances(5 * time.Second)
	assert := assert.New(t)

	// Set up two mids
	// One we will update after 2*time.Seconds
	// The other one we will not update before timeout
	// This should run clean only the second
	mid1 := ManifestID("First MID")
	mid2 := ManifestID("Second MID")
	// Start cleanup loop
	go b.StartCleanup()
	defer b.StopCleanup()

	// Fund balances
	credit := big.NewRat(100, 1)
	b.Credit(mid1, credit)
	b.Credit(mid2, credit)
	assert.Zero(b.Balance(mid1).Cmp(credit))
	assert.Zero(b.Balance(mid2).Cmp(credit))

	time.Sleep(2 * time.Second)
	b.Credit(mid1, credit)
	assert.Zero(b.Balance(mid1).Cmp(big.NewRat(200, 1)))

	time.Sleep(4 * time.Second)

	// Balance for mid1 should still be 200/1
	assert.NotNil(b.Balance(mid1))
	assert.Zero(b.Balance(mid1).Cmp(big.NewRat(200, 1)))
	// Balance for mid2 should be cleaned
	assert.Nil(b.Balance(mid2))

	time.Sleep(5 * time.Second)
	// Now balance for mid1 should be cleaned as well
	assert.Nil(b.Balance(mid1))
}
