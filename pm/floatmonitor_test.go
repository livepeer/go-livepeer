package pm

import (
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setTime is a helper to set the time during tests
var setTime = func(time int64) {
	unixNow = func() int64 {
		return time
	}
}

// increaseTime is a helper to increase the time during tests
var increaseTime = func(sec int64) {
	time := unixNow()
	setTime(time + sec)
}

func TestMaxFloat(t *testing.T) {
	claimant := RandAddress()
	b := newStubBroker()
	fm := NewFloatMonitor(claimant, b, 5*time.Minute, 3600)

	assert := assert.New(t)

	// Test ClaimableReserve() error

	b.claimableReserveShouldFail = true

	_, err := fm.MaxFloat(RandAddress())
	assert.EqualError(err, "stub broker ClaimableReserve error")

	// Test value not cached

	b.claimableReserveShouldFail = false
	addr := RandAddress()
	reserve := big.NewInt(10)
	b.SetReserve(addr, reserve)

	mf, err := fm.MaxFloat(addr)
	assert.Equal(reserve, mf)

	// Test value cached

	// Change stub broker value
	// FloatMonitor should still use cached value which
	// is different
	b.SetReserve(addr, big.NewInt(99))

	mf, err = fm.MaxFloat(addr)
	assert.Equal(reserve, mf)
}

func TestAdd(t *testing.T) {
	claimant := RandAddress()
	b := newStubBroker()
	fm := NewFloatMonitor(claimant, b, 5*time.Minute, 3600)

	assert := assert.New(t)
	require := require.New(t)

	// Test ClaimableReserve() error

	b.claimableReserveShouldFail = true

	err := fm.Add(RandAddress(), big.NewInt(10))
	assert.EqualError(err, "stub broker ClaimableReserve error")

	// Test value not cached

	b.claimableReserveShouldFail = false
	addr := RandAddress()
	reserve := big.NewInt(10)
	b.SetReserve(addr, reserve)

	amount := big.NewInt(20)
	err = fm.Add(addr, amount)
	assert.Nil(err)

	mf, err := fm.MaxFloat(addr)
	require.Nil(err)
	assert.Equal(new(big.Int).Add(reserve, amount), mf)

	// Test value cached

	// Change stub broker value
	// FloatMonitor should still use cached value which
	// is different
	b.SetReserve(addr, big.NewInt(99))

	err = fm.Add(addr, amount)
	assert.Nil(err)

	mf, err = fm.MaxFloat(addr)
	require.Nil(err)
	assert.Equal(
		new(big.Int).Add(reserve, new(big.Int).Mul(amount, big.NewInt(2))),
		mf,
	)
}

func TestSub(t *testing.T) {
	claimant := RandAddress()
	b := newStubBroker()
	fm := NewFloatMonitor(claimant, b, 5*time.Minute, 3600)

	assert := assert.New(t)
	require := require.New(t)

	// Test ClaimableReserve() error

	b.claimableReserveShouldFail = true

	err := fm.Sub(RandAddress(), big.NewInt(10))
	assert.EqualError(err, "stub broker ClaimableReserve error")

	// Test value not cached

	b.claimableReserveShouldFail = false
	addr := RandAddress()
	reserve := big.NewInt(20)
	b.SetReserve(addr, reserve)

	amount := big.NewInt(5)
	err = fm.Sub(addr, amount)
	assert.Nil(err)

	mf, err := fm.MaxFloat(addr)
	require.Nil(err)
	assert.Equal(new(big.Int).Sub(reserve, amount), mf)

	// Test value cached

	// Change stub broker value
	// FloatMonitor should still use cached value which
	// is different
	b.SetReserve(addr, big.NewInt(99))

	err = fm.Sub(addr, amount)
	assert.Nil(err)

	mf, err = fm.MaxFloat(addr)
	require.Nil(err)
	assert.Equal(
		new(big.Int).Sub(reserve, new(big.Int).Mul(amount, big.NewInt(2))),
		mf,
	)
}

func TestCleanup(t *testing.T) {
	claimant := RandAddress()
	b := newStubBroker()
	fm := NewFloatMonitor(claimant, b, 5*time.Minute, 2)

	assert := assert.New(t)
	require := require.New(t)

	setTime(0)

	// TODO: Test ticker?

	// Test clean up

	addr1 := RandAddress()
	addr2 := RandAddress()
	reserve := big.NewInt(20)
	b.SetReserve(addr1, reserve)
	b.SetReserve(addr2, reserve)

	// Cache values
	_, err := fm.MaxFloat(addr1)
	require.Nil(err)
	_, err = fm.MaxFloat(addr2)
	require.Nil(err)

	increaseTime(5)

	// Change stub broker value
	// FloatMonitor should no longer use cached values
	// since they have been cleaned up
	reserve2 := big.NewInt(99)
	b.SetReserve(addr1, reserve2)
	b.SetReserve(addr2, reserve2)

	fm.(*floatMonitor).cleanup()

	mf1, err := fm.MaxFloat(addr1)
	require.Nil(err)
	mf2, err := fm.MaxFloat(addr2)
	require.Nil(err)

	assert.Equal(reserve2, mf1)
	assert.Equal(reserve2, mf2)

	// Test clean up after excluding items
	// with updated lastAccess due to MaxFloat()

	// Update lastAccess for addr1
	increaseTime(4)
	_, err = fm.MaxFloat(addr1)
	require.Nil(err)

	increaseTime(1)

	// Change stub broker value
	// FloatMonitor should:
	// - Use cached value for addr1 because it was accessed recently via MaxFloat()
	// - Use new value for addr2 because it was cleaned up
	reserve3 := big.NewInt(99)
	b.SetReserve(addr1, reserve3)
	b.SetReserve(addr2, reserve3)

	fm.(*floatMonitor).cleanup()

	mf1, err = fm.MaxFloat(addr1)
	require.Nil(err)
	mf2, err = fm.MaxFloat(addr2)
	require.Nil(err)

	assert.Equal(reserve2, mf1)
	assert.Equal(reserve3, mf2)

	// Test clean up excluding items
	// with updated lastAccess due to Add()

	// Update lastAccess for addr2
	increaseTime(4)
	err = fm.Add(addr2, big.NewInt(0))
	require.Nil(err)

	increaseTime(1)

	// FloatMonitor should:
	// - Use new value for addr1 because it was cleaned up
	// - Use cached value for addr2 because it was accessed recently via Add()

	mf1, err = fm.MaxFloat(addr1)
	require.Nil(err)
	mf2, err = fm.MaxFloat(addr2)
	require.Nil(err)

	assert.Equal(reserve3, mf1)
	assert.Equal(reserve3, mf2)

	// Test clean up excluding items
	// with updated lastAccess due to Sub()

	// Update lastAccess for addr1
	increaseTime(4)
	err = fm.Sub(addr1, big.NewInt(0))
	require.Nil(err)

	increaseTime(1)

	// Change stub broker value
	// FloatMonitor should:
	// - Use cached value for addr1 because it was accessed recently via Sub()
	// - Use new value for addr2 because it was cleaned up
	reserve4 := big.NewInt(999)
	b.SetReserve(addr1, reserve4)
	b.SetReserve(addr2, reserve4)

	fm.(*floatMonitor).cleanup()

	mf1, err = fm.MaxFloat(addr1)
	require.Nil(err)
	mf2, err = fm.MaxFloat(addr2)
	require.Nil(err)

	assert.Equal(reserve3, mf1)
	assert.Equal(reserve4, mf2)
}
