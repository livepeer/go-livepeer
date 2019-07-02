package core

import (
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

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
