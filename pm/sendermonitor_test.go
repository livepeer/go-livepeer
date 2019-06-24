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
	sm := NewSenderMonitor(claimant, b, 5*time.Minute, 3600, 3)
	sm.Start()
	defer sm.Stop()

	assert := assert.New(t)

	// Test ClaimableReserve() error

	b.claimableReserveShouldFail = true

	_, err := sm.MaxFloat(RandAddress())
	assert.EqualError(err, "stub broker ClaimableReserve error")

	// Test value not cached

	b.claimableReserveShouldFail = false
	addr := RandAddress()
	reserve := big.NewInt(10)
	b.SetReserve(addr, reserve)

	mf, err := sm.MaxFloat(addr)
	assert.Equal(reserve, mf)

	// Test value cached

	// Change stub broker value
	// SenderMonitor should still use cached value which
	// is different
	b.SetReserve(addr, big.NewInt(99))

	mf, err = sm.MaxFloat(addr)
	assert.Equal(reserve, mf)
}

func TestSubFloat(t *testing.T) {
	claimant := RandAddress()
	b := newStubBroker()
	sm := NewSenderMonitor(claimant, b, 5*time.Minute, 3600, 3)
	sm.Start()
	defer sm.Stop()

	assert := assert.New(t)
	require := require.New(t)

	// Test ClaimableReserve() error

	b.claimableReserveShouldFail = true

	err := sm.SubFloat(RandAddress(), big.NewInt(10))
	assert.EqualError(err, "stub broker ClaimableReserve error")

	// Test value not cached

	b.claimableReserveShouldFail = false
	addr := RandAddress()
	reserve := big.NewInt(20)
	b.SetReserve(addr, reserve)

	amount := big.NewInt(5)
	err = sm.SubFloat(addr, amount)
	assert.Nil(err)

	mf, err := sm.MaxFloat(addr)
	require.Nil(err)
	assert.Equal(new(big.Int).Sub(reserve, amount), mf)

	// Test value cached

	// Change stub broker value
	// SenderMonitor should still use cached value which
	// is different
	b.SetReserve(addr, big.NewInt(99))

	err = sm.SubFloat(addr, amount)
	assert.Nil(err)

	mf, err = sm.MaxFloat(addr)
	require.Nil(err)
	assert.Equal(
		new(big.Int).Sub(reserve, new(big.Int).Mul(amount, big.NewInt(2))),
		mf,
	)
}
func TestAddFloat(t *testing.T) {
	claimant := RandAddress()
	b := newStubBroker()
	sm := NewSenderMonitor(claimant, b, 5*time.Minute, 3600, 3)
	sm.Start()
	defer sm.Stop()

	assert := assert.New(t)
	require := require.New(t)

	// Test ClaimableReserve() error

	b.claimableReserveShouldFail = true

	err := sm.AddFloat(RandAddress(), big.NewInt(10))
	assert.EqualError(err, "stub broker ClaimableReserve error")

	// Test value not cached and insufficient pendingAmount error

	b.claimableReserveShouldFail = false
	addr := RandAddress()
	reserve := big.NewInt(100)
	b.SetReserve(addr, reserve)

	amount := big.NewInt(20)
	err = sm.AddFloat(addr, amount)
	assert.EqualError(err, "cannot subtract from insufficient pendingAmount")

	// Test value cached and no pendingAmount error

	err = sm.SubFloat(addr, amount)
	require.Nil(err)

	err = sm.AddFloat(addr, amount)
	assert.Nil(err)

	mf, err := sm.MaxFloat(addr)
	require.Nil(err)
	assert.Equal(mf, reserve)

	// Test cached value update
	// Change stub broker value
	// SenderMonitor should still use cached value which
	// is different
	newReserve := big.NewInt(99)
	b.SetReserve(addr, newReserve)

	err = sm.SubFloat(addr, amount)
	require.Nil(err)

	err = sm.AddFloat(addr, amount)
	assert.Nil(err)

	mf, err = sm.MaxFloat(addr)
	require.Nil(err)
	assert.Equal(reserve, mf)
}

func TestQueueTicketAndSignalMaxFloat(t *testing.T) {
	claimant := RandAddress()
	b := newStubBroker()
	sm := NewSenderMonitor(claimant, b, 5*time.Minute, 3600, 3)
	sm.Start()
	defer sm.Stop()

	assert := assert.New(t)
	require := require.New(t)

	// Test ClaimableReserve() error

	b.claimableReserveShouldFail = true
	addr := RandAddress()

	err := sm.QueueTicket(addr, defaultSignedTicket(uint32(0)))
	assert.EqualError(err, "stub broker ClaimableReserve error")

	// Test queue ticket

	b.claimableReserveShouldFail = false
	reserve := big.NewInt(100)
	b.SetReserve(addr, reserve)

	err = sm.QueueTicket(addr, defaultSignedTicket(uint32(0)))
	assert.Nil(err)

	err = sm.SubFloat(addr, big.NewInt(5))
	require.Nil(err)

	qc := &queueConsumer{}
	go qc.Wait(1, sm)

	err = sm.AddFloat(addr, big.NewInt(5))
	require.Nil(err)

	time.Sleep(time.Millisecond * 20)
	tickets := qc.Redeemable()
	assert.Equal(1, len(tickets))
	assert.Equal(uint32(0), tickets[0].SenderNonce)

	// Test queue tickets from multiple senders

	addr2 := RandAddress()
	b.SetReserve(addr2, reserve)

	err = sm.QueueTicket(addr, defaultSignedTicket(uint32(2)))
	assert.Nil(err)
	err = sm.QueueTicket(addr2, defaultSignedTicket(uint32(3)))
	assert.Nil(err)

	err = sm.SubFloat(addr, big.NewInt(5))
	require.Nil(err)
	err = sm.SubFloat(addr2, big.NewInt(5))
	require.Nil(err)

	qc = &queueConsumer{}
	go qc.Wait(2, sm)

	err = sm.AddFloat(addr2, big.NewInt(5))
	require.Nil(err)
	err = sm.AddFloat(addr, big.NewInt(5))
	require.Nil(err)

	time.Sleep(time.Millisecond * 20)
	// Order of tickets should reflect order that AddFloat()
	// was called
	tickets = qc.Redeemable()
	assert.Equal(2, len(tickets))
	assert.Equal(uint32(3), tickets[0].SenderNonce)
	assert.Equal(uint32(2), tickets[1].SenderNonce)
}

func TestCleanup(t *testing.T) {
	claimant := RandAddress()
	b := newStubBroker()
	sm := NewSenderMonitor(claimant, b, 5*time.Minute, 2, 3)
	sm.Start()
	defer sm.Stop()

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
	_, err := sm.MaxFloat(addr1)
	require.Nil(err)
	_, err = sm.MaxFloat(addr2)
	require.Nil(err)

	increaseTime(5)

	// Change stub broker value
	// SenderMonitor should no longer use cached values
	// since they have been cleaned up
	reserve2 := big.NewInt(99)
	b.SetReserve(addr1, reserve2)
	b.SetReserve(addr2, reserve2)

	sm.(*senderMonitor).cleanup()

	mf1, err := sm.MaxFloat(addr1)
	require.Nil(err)
	mf2, err := sm.MaxFloat(addr2)
	require.Nil(err)

	assert.Equal(reserve2, mf1)
	assert.Equal(reserve2, mf2)

	// Test clean up after excluding items
	// with updated lastAccess due to MaxFloat()

	// Update lastAccess for addr1
	increaseTime(4)
	_, err = sm.MaxFloat(addr1)
	require.Nil(err)

	increaseTime(1)

	// Change stub broker value
	// SenderMonitor should:
	// - Use cached value for addr1 because it was accessed recently via MaxFloat()
	// - Use new value for addr2 because it was cleaned up
	reserve3 := big.NewInt(100)
	b.SetReserve(addr1, reserve3)
	b.SetReserve(addr2, reserve3)

	sm.(*senderMonitor).cleanup()

	mf1, err = sm.MaxFloat(addr1)
	require.Nil(err)
	mf2, err = sm.MaxFloat(addr2)
	require.Nil(err)

	assert.Equal(reserve2, mf1)
	assert.Equal(reserve3, mf2)

	// Test clean up excluding items
	// with updated lastAccess due to AddFloat()

	// Update lastAccess for addr2
	increaseTime(4)
	err = sm.AddFloat(addr2, big.NewInt(0))
	require.Nil(err)

	increaseTime(1)

	// Change stub broker value
	// SenderMonitor should:
	// - Use new value for addr1 because it was cleaned up
	// - Use cached value for addr2 because it was accessed recently via AddFloat()
	reserve4 := big.NewInt(101)
	b.SetReserve(addr1, reserve4)
	b.SetReserve(addr2, reserve4)

	sm.(*senderMonitor).cleanup()

	mf1, err = sm.MaxFloat(addr1)
	require.Nil(err)
	mf2, err = sm.MaxFloat(addr2)
	require.Nil(err)

	assert.Equal(reserve4, mf1)
	assert.Equal(reserve3, mf2)

	// Test clean up excluding items
	// with updated lastAccess due to SubFloat()

	// Update lastAccess for addr1
	increaseTime(4)
	err = sm.SubFloat(addr1, big.NewInt(0))
	require.Nil(err)

	increaseTime(1)

	// Change stub broker value
	// SenderMonitor should:
	// - Use cached value for addr1 because it was accessed recently via SubFloat()
	// - Use new value for addr2 because it was cleaned up
	reserve5 := big.NewInt(999)
	b.SetReserve(addr1, reserve5)
	b.SetReserve(addr2, reserve5)

	sm.(*senderMonitor).cleanup()

	mf1, err = sm.MaxFloat(addr1)
	require.Nil(err)
	mf2, err = sm.MaxFloat(addr2)
	require.Nil(err)

	assert.Equal(reserve4, mf1)
	assert.Equal(reserve5, mf2)
}

func TestAcceptErr(t *testing.T) {
	claimant := RandAddress()
	addr := RandAddress()
	b := newStubBroker()
	sm := NewSenderMonitor(claimant, b, 5*time.Minute, 2, 1)

	assert := assert.New(t)

	// Cache remote sender
	reserve := big.NewInt(10)
	b.SetReserve(addr, reserve)
	sm.MaxFloat(addr)

	// Test errCount for addr below maxErrCount
	ok := sm.AcceptErr(addr)
	assert.True(ok)

	// Test errCount for addr equal to maxErrCount
	ok = sm.AcceptErr(addr)
	assert.False(ok)

	// Test errCount did not increase when errCount = maxErrCount
	ok = sm.AcceptErr(addr)
	assert.False(ok)
}
