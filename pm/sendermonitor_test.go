package pm

import (
	"fmt"
	"math/big"
	"testing"
	"time"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"
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
	claimant, b, smgr, tm := senderMonitorFixture()
	addr := RandAddress()
	smgr.info[addr] = &SenderInfo{
		Deposit:       big.NewInt(500),
		WithdrawRound: big.NewInt(0),
		Reserve: &ReserveInfo{
			FundsRemaining:        big.NewInt(500),
			ClaimedInCurrentRound: big.NewInt(0),
		},
	}
	smgr.claimedReserve[addr] = big.NewInt(100)
	tm.transcoderPoolSize = big.NewInt(50)
	sm := NewSenderMonitor(claimant, b, smgr, tm, 5*time.Minute, 3600)
	sm.Start()
	defer sm.Stop()

	assert := assert.New(t)

	// Test ClaimedReserve() error
	smgr.err = errors.New("ClaimedReserve error")

	_, err := sm.MaxFloat(RandAddress())
	assert.EqualError(err, "ClaimedReserve error")

	// Test value cached

	smgr.err = nil
	reserve := new(big.Int).Add(smgr.info[addr].Reserve.FundsRemaining, smgr.info[addr].Reserve.ClaimedInCurrentRound)
	reserveAlloc := new(big.Int).Sub(new(big.Int).Div(reserve, tm.transcoderPoolSize), smgr.claimedReserve[addr])

	mf, err := sm.MaxFloat(addr)
	assert.Equal(reserveAlloc, mf)

	// test race conditions
	addrs := make([]ethcommon.Address, 5)
	for i := range addrs {
		addr := RandAddress()
		addrs[i] = addr
		smgr.info[addr] = &SenderInfo{
			Deposit:       big.NewInt(500),
			WithdrawRound: big.NewInt(0),
			Reserve: &ReserveInfo{
				FundsRemaining:        big.NewInt(500),
				ClaimedInCurrentRound: big.NewInt(0),
			},
		}
		smgr.claimedReserve[addr] = big.NewInt(100)
	}

	for _, addr := range addrs {
		go sm.MaxFloat(addr)
	}
}

func TestSubFloat(t *testing.T) {
	claimant, b, smgr, tm := senderMonitorFixture()
	addr := RandAddress()
	smgr.info[addr] = &SenderInfo{
		Deposit:       big.NewInt(500),
		WithdrawRound: big.NewInt(0),
		Reserve: &ReserveInfo{
			FundsRemaining:        big.NewInt(500),
			ClaimedInCurrentRound: big.NewInt(0),
		},
	}
	smgr.claimedReserve[addr] = big.NewInt(100)
	tm.transcoderPoolSize = big.NewInt(50)
	sm := NewSenderMonitor(claimant, b, smgr, tm, 5*time.Minute, 3600)
	sm.Start()
	defer sm.Stop()

	assert := assert.New(t)
	require := require.New(t)

	reserve := new(big.Int).Add(smgr.info[addr].Reserve.FundsRemaining, smgr.info[addr].Reserve.ClaimedInCurrentRound)
	reserveAlloc := new(big.Int).Sub(new(big.Int).Div(reserve, tm.transcoderPoolSize), smgr.claimedReserve[addr])

	amount := big.NewInt(5)
	sm.SubFloat(addr, amount)
	mf, err := sm.MaxFloat(addr)
	require.Nil(err)
	assert.Equal(new(big.Int).Sub(reserveAlloc, amount), mf)

	sm.SubFloat(addr, amount)
	assert.Nil(err)

	mf, err = sm.MaxFloat(addr)
	require.Nil(err)
	assert.Equal(
		new(big.Int).Sub(reserveAlloc, new(big.Int).Mul(amount, big.NewInt(2))),
		mf,
	)
}

func TestAddFloat(t *testing.T) {
	claimant, b, smgr, tm := senderMonitorFixture()
	addr := RandAddress()
	smgr.info[addr] = &SenderInfo{
		Deposit:       big.NewInt(500),
		WithdrawRound: big.NewInt(0),
		Reserve: &ReserveInfo{
			FundsRemaining:        big.NewInt(500),
			ClaimedInCurrentRound: big.NewInt(0),
		},
	}
	smgr.claimedReserve[addr] = big.NewInt(100)
	tm.transcoderPoolSize = big.NewInt(1)
	sm := NewSenderMonitor(claimant, b, smgr, tm, 5*time.Minute, 3600)
	sm.Start()
	defer sm.Stop()

	assert := assert.New(t)
	require := require.New(t)

	// Test value not cached and insufficient pendingAmount error
	smgr.err = nil
	reserve := new(big.Int).Add(smgr.info[addr].Reserve.FundsRemaining, smgr.info[addr].Reserve.ClaimedInCurrentRound)
	reserveAlloc := new(big.Int).Sub(new(big.Int).Div(reserve, tm.transcoderPoolSize), smgr.claimedReserve[addr])

	amount := big.NewInt(20)
	err := sm.AddFloat(addr, amount)
	assert.EqualError(err, "cannot subtract from insufficient pendingAmount")

	// Test value cached and no pendingAmount error
	sm.SubFloat(addr, amount)

	err = sm.AddFloat(addr, amount)
	assert.Nil(err)

	mf, err := sm.MaxFloat(addr)
	require.Nil(err)
	assert.Equal(mf, reserveAlloc)

	// Test cached value update
	smgr.info[addr].Reserve.FundsRemaining = big.NewInt(1000)
	reserve = new(big.Int).Add(smgr.info[addr].Reserve.FundsRemaining, smgr.info[addr].Reserve.ClaimedInCurrentRound)
	reserveAlloc = new(big.Int).Sub(new(big.Int).Div(reserve, tm.transcoderPoolSize), smgr.claimedReserve[addr])

	sm.SubFloat(addr, amount)

	err = sm.AddFloat(addr, amount)
	assert.Nil(err)

	mf, err = sm.MaxFloat(addr)
	require.Nil(err)
	assert.Equal(reserveAlloc, mf)
}

func TestQueueTicketAndSignalNewBlock(t *testing.T) {
	claimant, b, smgr, tm := senderMonitorFixture()
	addr := RandAddress()
	smgr.info[addr] = &SenderInfo{
		Deposit:       big.NewInt(500),
		WithdrawRound: big.NewInt(0),
		Reserve: &ReserveInfo{
			FundsRemaining:        big.NewInt(5000),
			ClaimedInCurrentRound: big.NewInt(0),
		},
	}
	smgr.claimedReserve[addr] = big.NewInt(100)
	sm := NewSenderMonitor(claimant, b, smgr, tm, 5*time.Minute, 3600)
	sm.Start()
	defer sm.Stop()

	assert := assert.New(t)

	// Test queue ticket

	sm.QueueTicket(addr, defaultSignedTicket(uint32(0)))
	time.Sleep(20 * time.Millisecond)

	assert.Equal(sm.(*senderMonitor).senders[addr].queue.Length(), int32(1))

	qc := &queueConsumer{}
	go qc.Wait(1, sm)

	tm.blockNumSink <- big.NewInt(5)
	time.Sleep(20 * time.Millisecond)

	// check that ticket is now removed from queue
	assert.Equal(sm.(*senderMonitor).senders[addr].queue.Length(), int32(0))

	tickets := qc.Redeemable()
	assert.Equal(1, len(tickets))
	assert.Equal(uint32(0), tickets[0].SenderNonce)

	// Test queue tickets from multiple senders

	addr2 := RandAddress()
	smgr.info[addr2] = &SenderInfo{
		Deposit:       big.NewInt(500),
		WithdrawRound: big.NewInt(0),
		Reserve: &ReserveInfo{
			FundsRemaining:        big.NewInt(5000),
			ClaimedInCurrentRound: big.NewInt(0),
		},
	}
	smgr.claimedReserve[addr2] = big.NewInt(100)

	qc = &queueConsumer{}
	go qc.Wait(2, sm)

	sm.QueueTicket(addr, defaultSignedTicket(uint32(2)))
	time.Sleep(20 * time.Millisecond)
	assert.Equal(sm.(*senderMonitor).senders[addr].queue.Length(), int32(1))
	tm.blockNumSink <- big.NewInt(5)
	time.Sleep(20 * time.Millisecond)

	sm.QueueTicket(addr2, defaultSignedTicket(uint32(3)))
	time.Sleep(20 * time.Millisecond)
	assert.Equal(sm.(*senderMonitor).senders[addr2].queue.Length(), int32(1))
	tm.blockNumSink <- big.NewInt(5)
	time.Sleep(20 * time.Millisecond)

	tickets = qc.Redeemable()
	assert.Equal(2, len(tickets))
	assert.Equal(uint32(2), tickets[0].Ticket.SenderNonce)
	assert.Equal(uint32(3), tickets[1].Ticket.SenderNonce)
}

func TestCleanup(t *testing.T) {
	claimant, b, smgr, tm := senderMonitorFixture()
	sm := NewSenderMonitor(claimant, b, smgr, tm, 5*time.Minute, 3600)
	sm.Start()
	defer sm.Stop()

	assert := assert.New(t)
	require := require.New(t)

	setTime(0)

	// TODO: Test ticker?

	// Test clean up
	addr1 := RandAddress()
	addr2 := RandAddress()
	smgr.info[addr1] = &SenderInfo{
		Deposit:       big.NewInt(500),
		WithdrawRound: big.NewInt(0),
		Reserve: &ReserveInfo{
			FundsRemaining:        big.NewInt(500),
			ClaimedInCurrentRound: big.NewInt(0),
		},
	}
	smgr.claimedReserve[addr1] = big.NewInt(100)
	smgr.info[addr2] = &SenderInfo{
		Deposit:       big.NewInt(500),
		WithdrawRound: big.NewInt(0),
		Reserve: &ReserveInfo{
			FundsRemaining:        big.NewInt(500),
			ClaimedInCurrentRound: big.NewInt(0),
		},
	}
	smgr.claimedReserve[addr2] = big.NewInt(100)

	// Set lastAccess
	_, err := sm.MaxFloat(addr1)
	require.Nil(err)
	_, err = sm.MaxFloat(addr2)
	require.Nil(err)

	increaseTime(10)

	// Change stub SenderManager values
	// SenderMonitor should no longer use cached values
	// since they have been cleaned up
	sm.(*senderMonitor).cleanup()
	smgr.Clear(addr1)
	smgr.Clear(addr2)
	assert.Nil(smgr.info[addr1])
	assert.Nil(smgr.claimedReserve[addr1])
	assert.Nil(smgr.info[addr2])
	assert.Nil(smgr.claimedReserve[addr2])

	reserve2 := big.NewInt(1000)
	smgr.info[addr1] = &SenderInfo{
		Deposit:       big.NewInt(500),
		WithdrawRound: big.NewInt(0),
		Reserve: &ReserveInfo{
			FundsRemaining:        reserve2,
			ClaimedInCurrentRound: big.NewInt(0),
		},
	}
	smgr.claimedReserve[addr1] = big.NewInt(100)
	smgr.info[addr2] = &SenderInfo{
		Deposit:       big.NewInt(500),
		WithdrawRound: big.NewInt(0),
		Reserve: &ReserveInfo{
			FundsRemaining:        reserve2,
			ClaimedInCurrentRound: big.NewInt(0),
		},
	}
	smgr.claimedReserve[addr2] = big.NewInt(100)

	mf1, err := sm.MaxFloat(addr1)
	require.Nil(err)
	mf2, err := sm.MaxFloat(addr2)
	require.Nil(err)

	expectedReserve := new(big.Int).Add(smgr.info[addr1].Reserve.FundsRemaining, smgr.info[addr1].Reserve.ClaimedInCurrentRound)
	expectedAlloc := new(big.Int).Sub(new(big.Int).Div(expectedReserve, tm.transcoderPoolSize), smgr.claimedReserve[addr1])

	assert.Equal(expectedAlloc, mf1)
	assert.Equal(expectedAlloc, mf2)

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
	smgr.info[addr2].Reserve.FundsRemaining = reserve3

	sm.(*senderMonitor).cleanup()

	mf1, err = sm.MaxFloat(addr1)
	require.Nil(err)
	mf2, err = sm.MaxFloat(addr2)
	require.Nil(err)

	expectedReserve2 := new(big.Int).Add(smgr.info[addr2].Reserve.FundsRemaining, smgr.info[addr2].Reserve.ClaimedInCurrentRound)
	expectedAlloc2 := new(big.Int).Sub(new(big.Int).Div(expectedReserve2, tm.transcoderPoolSize), smgr.claimedReserve[addr2])
	assert.Equal(expectedAlloc, mf1)
	assert.Equal(expectedAlloc2, mf2)

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
	smgr.info[addr1].Reserve.FundsRemaining = reserve4

	sm.(*senderMonitor).cleanup()

	mf1, err = sm.MaxFloat(addr1)
	require.Nil(err)
	mf2, err = sm.MaxFloat(addr2)
	require.Nil(err)

	expectedReserve3 := new(big.Int).Add(smgr.info[addr1].Reserve.FundsRemaining, smgr.info[addr1].Reserve.ClaimedInCurrentRound)
	expectedAlloc3 := new(big.Int).Sub(new(big.Int).Div(expectedReserve3, tm.transcoderPoolSize), smgr.claimedReserve[addr1])

	assert.Equal(expectedAlloc3, mf1)
	assert.Equal(expectedAlloc2, mf2)

	// Test clean up excluding items
	// with updated lastAccess due to SubFloat()

	// Update lastAccess for addr1
	increaseTime(4)
	sm.SubFloat(addr1, big.NewInt(0))

	increaseTime(1)

	// Change stub broker value
	// SenderMonitor should:
	// - Use cached value for addr1 because it was accessed recently via SubFloat()
	// - Use new value for addr2 because it was cleaned up
	reserve5 := big.NewInt(999)
	smgr.info[addr2].Reserve.FundsRemaining = reserve5

	sm.(*senderMonitor).cleanup()

	mf1, err = sm.MaxFloat(addr1)
	require.Nil(err)
	mf2, err = sm.MaxFloat(addr2)
	require.Nil(err)

	expectedReserve4 := new(big.Int).Add(smgr.info[addr2].Reserve.FundsRemaining, smgr.info[addr2].Reserve.ClaimedInCurrentRound)
	expectedAlloc4 := new(big.Int).Sub(new(big.Int).Div(expectedReserve4, tm.transcoderPoolSize), smgr.claimedReserve[addr2])
	assert.Equal(expectedAlloc3, mf1)
	assert.Equal(expectedAlloc4, mf2)
}

func TestReserveAlloc(t *testing.T) {
	assert := assert.New(t)
	claimant, b, smgr, tm := senderMonitorFixture()
	addr := RandAddress()
	smgr.info[addr] = &SenderInfo{
		Deposit:       big.NewInt(500),
		WithdrawRound: big.NewInt(0),
		Reserve: &ReserveInfo{
			FundsRemaining:        big.NewInt(5000),
			ClaimedInCurrentRound: big.NewInt(0),
		},
	}
	smgr.claimedReserve[addr] = big.NewInt(100)
	sm := NewSenderMonitor(claimant, b, smgr, tm, 5*time.Minute, 3600).(*senderMonitor)

	// test GetSenderInfo error
	smgr.err = errors.New("GetSenderInfo error")
	_, err := sm.reserveAlloc(addr)
	assert.EqualError(err, smgr.err.Error())
	// test reserveAlloc correctly calculated
	smgr.err = nil
	expectedReserve := new(big.Int).Add(smgr.info[addr].Reserve.FundsRemaining, smgr.info[addr].Reserve.ClaimedInCurrentRound)
	expectedAlloc := new(big.Int).Sub(new(big.Int).Div(expectedReserve, tm.transcoderPoolSize), smgr.claimedReserve[addr])
	alloc, err := sm.reserveAlloc(addr)
	assert.Nil(err)
	assert.Zero(expectedAlloc.Cmp(alloc))
}

func TestSenderMonitor_ValidateSender(t *testing.T) {
	claimant, b, smgr, tm := senderMonitorFixture()
	addr := RandAddress()
	smgr.info[addr] = &SenderInfo{
		WithdrawRound: big.NewInt(10),
	}
	sm := NewSenderMonitor(claimant, b, smgr, tm, 5*time.Minute, 3600)
	sm.Start()
	defer sm.Stop()

	assert := assert.New(t)

	// SenderManager.GetSenderInfo error
	smgr.err = errors.New("GetSenderInfo error")
	expErr := fmt.Sprintf("could not get sender info for %v: %v", addr.Hex(), smgr.err.Error())
	err := sm.ValidateSender(addr)
	assert.EqualError(err, expErr)
	smgr.err = nil

	// WithdrawRound = 0 (not unlocked) -> No error
	tm.round = big.NewInt(2)
	smgr.info[addr].WithdrawRound = big.NewInt(0)
	err = sm.ValidateSender(addr)
	assert.NoError(err)
	smgr.info[addr].WithdrawRound = big.NewInt(10)

	// currentRound + 1 < WithdrawRound -> No error
	tm.round = big.NewInt(1)
	err = sm.ValidateSender(addr)
	assert.Nil(err)

	// currentRound + 1 == WithdrawRound -> Error
	expErr = fmt.Sprintf("deposit and reserve for sender %v is set to unlock soon", addr.Hex())
	tm.round = big.NewInt(9)
	err = sm.ValidateSender(addr)
	assert.EqualError(err, expErr)

	// currentRound +1 > WithdrawRound -> Error
	tm.round = big.NewInt(10)
	err = sm.ValidateSender(addr)
	assert.EqualError(err, expErr)
}

func senderMonitorFixture() (ethcommon.Address, *stubBroker, *stubSenderManager, *stubTimeManager) {
	claimant := RandAddress()
	b := newStubBroker()
	smgr := newStubSenderManager()
	tm := &stubTimeManager{
		transcoderPoolSize: big.NewInt(5),
	}
	return claimant, b, smgr, tm
}
