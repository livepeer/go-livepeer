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
	sm := NewSenderMonitor(claimant, b, smgr, tm, newStubTicketStore(), 5*time.Minute, 3600)
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
	sm := NewSenderMonitor(claimant, b, smgr, tm, newStubTicketStore(), 5*time.Minute, 3600)
	sm.Start()
	defer sm.Stop()

	assert := assert.New(t)
	require := require.New(t)

	reserve := new(big.Int).Add(smgr.info[addr].Reserve.FundsRemaining, smgr.info[addr].Reserve.ClaimedInCurrentRound)
	reserveAlloc := new(big.Int).Sub(new(big.Int).Div(reserve, tm.transcoderPoolSize), smgr.claimedReserve[addr])

	amount := big.NewInt(5)
	sm.(*senderMonitor).subFloat(addr, amount)
	mf, err := sm.MaxFloat(addr)
	require.Nil(err)
	assert.Equal(new(big.Int).Sub(reserveAlloc, amount), mf)

	sm.(*senderMonitor).subFloat(addr, amount)
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
	sm := NewSenderMonitor(claimant, b, smgr, tm, newStubTicketStore(), 5*time.Minute, 3600)
	sm.Start()
	defer sm.Stop()

	assert := assert.New(t)
	require := require.New(t)

	// Test value not cached and insufficient pendingAmount error
	smgr.err = nil
	reserve := new(big.Int).Add(smgr.info[addr].Reserve.FundsRemaining, smgr.info[addr].Reserve.ClaimedInCurrentRound)
	reserveAlloc := new(big.Int).Sub(new(big.Int).Div(reserve, tm.transcoderPoolSize), smgr.claimedReserve[addr])

	amount := big.NewInt(20)
	err := sm.(*senderMonitor).addFloat(addr, amount)
	assert.EqualError(err, "cannot subtract from insufficient pendingAmount")

	// Test value cached and no pendingAmount error
	sm.(*senderMonitor).subFloat(addr, amount)

	err = sm.(*senderMonitor).addFloat(addr, amount)
	assert.Nil(err)

	mf, err := sm.MaxFloat(addr)
	require.Nil(err)
	assert.Equal(mf, reserveAlloc)

	// Test cached value update
	smgr.info[addr].Reserve.FundsRemaining = big.NewInt(1000)
	reserve = new(big.Int).Add(smgr.info[addr].Reserve.FundsRemaining, smgr.info[addr].Reserve.ClaimedInCurrentRound)
	reserveAlloc = new(big.Int).Sub(new(big.Int).Div(reserve, tm.transcoderPoolSize), smgr.claimedReserve[addr])

	sm.(*senderMonitor).subFloat(addr, amount)

	err = sm.(*senderMonitor).addFloat(addr, amount)
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

	ts := newStubTicketStore()
	smgr.claimedReserve[addr] = big.NewInt(100)
	sm := NewSenderMonitor(claimant, b, smgr, tm, ts, 5*time.Minute, 3600)
	sm.Start()
	defer sm.Stop()

	assert := assert.New(t)

	// Test queue ticket
	// test fail
	ts.storeShouldFail = true
	assert.EqualError(sm.QueueTicket(defaultSignedTicket(addr, uint32(0))), "stub ticket store store error")
	ts.storeShouldFail = false

	signedT := defaultSignedTicket(addr, uint32(0))
	err := sm.QueueTicket(signedT)
	assert.Nil(err)
	time.Sleep(20 * time.Millisecond)
	qlen, err := sm.(*senderMonitor).senders[addr].queue.Length()
	assert.Nil(err)
	assert.Equal(qlen, 1)

	tm.blockNumSink <- big.NewInt(5)
	time.Sleep(20 * time.Millisecond)

	// check that ticket is now removed from queue
	qlen, err = sm.(*senderMonitor).senders[addr].queue.Length()
	assert.Nil(err)
	assert.Equal(qlen, 0)

	// check that ticket is used
	assert.True(b.IsUsedTicket(signedT.Ticket))

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

	signedT2 := defaultSignedTicket(addr, (2))
	sm.QueueTicket(signedT2)
	time.Sleep(20 * time.Millisecond)
	qlen, err = sm.(*senderMonitor).senders[addr].queue.Length()
	assert.Nil(err)
	assert.Equal(qlen, 1)
	tm.blockNumSink <- big.NewInt(5)
	time.Sleep(20 * time.Millisecond)

	signedT3 := defaultSignedTicket(addr2, uint32(3))
	sm.QueueTicket(signedT3)
	time.Sleep(20 * time.Millisecond)
	qlen, err = sm.(*senderMonitor).senders[addr2].queue.Length()
	assert.Nil(err)
	assert.Equal(qlen, 1)
	tm.blockNumSink <- big.NewInt(5)
	time.Sleep(20 * time.Millisecond)

	assert.True(b.IsUsedTicket(signedT2.Ticket))
	assert.True(b.IsUsedTicket(signedT3.Ticket))
}

func TestCleanup(t *testing.T) {
	claimant, b, smgr, tm := senderMonitorFixture()
	sm := NewSenderMonitor(claimant, b, smgr, tm, newStubTicketStore(), 5*time.Minute, 3600)
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
	// with updated lastAccess due to addFloat()

	// Update lastAccess for addr2
	increaseTime(4)
	err = sm.(*senderMonitor).addFloat(addr2, big.NewInt(0))
	require.Nil(err)

	increaseTime(1)

	// Change stub broker value
	// SenderMonitor should:
	// - Use new value for addr1 because it was cleaned up
	// - Use cached value for addr2 because it was accessed recently via addFloat()
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
	// with updated lastAccess due to subFloat()

	// Update lastAccess for addr1
	increaseTime(4)
	sm.(*senderMonitor).subFloat(addr1, big.NewInt(0))

	increaseTime(1)

	// Change stub broker value
	// SenderMonitor should:
	// - Use cached value for addr1 because it was accessed recently via subFloat()
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
	sm := NewSenderMonitor(claimant, b, smgr, tm, newStubTicketStore(), 5*time.Minute, 3600).(*senderMonitor)

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
	sm := NewSenderMonitor(claimant, b, smgr, tm, newStubTicketStore(), 5*time.Minute, 3600)
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

func TestRedeemWinningTicket_SingleTicket_ZeroMaxFloat(t *testing.T) {
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

	ts := newStubTicketStore()
	smgr.claimedReserve[addr] = big.NewInt(100)
	sm := NewSenderMonitor(claimant, b, smgr, tm, ts, 5*time.Minute, 3600)
	sm.Start()
	defer sm.Stop()

	assert := assert.New(t)

	signedT := defaultSignedTicket(addr, uint32(0))

	err := sm.(*senderMonitor).redeemWinningTicket(signedT)
	assert.EqualError(err, "max float is zero")
}

func TestRedeemWinningTicket_SingleTicket_RedeemError(t *testing.T) {
	claimant, b, smgr, tm := senderMonitorFixture()
	addr := RandAddress()
	smgr.info[addr] = &SenderInfo{
		Deposit:       big.NewInt(500),
		WithdrawRound: big.NewInt(0),
		Reserve: &ReserveInfo{
			FundsRemaining:        big.NewInt(1000),
			ClaimedInCurrentRound: big.NewInt(0),
		},
	}

	ts := newStubTicketStore()
	smgr.claimedReserve[addr] = big.NewInt(100)
	sm := NewSenderMonitor(claimant, b, smgr, tm, ts, 5*time.Minute, 3600)
	sm.Start()
	defer sm.Stop()

	assert := assert.New(t)

	signedT := defaultSignedTicket(addr, uint32(0))

	b.redeemShouldFail = true
	err := sm.(*senderMonitor).redeemWinningTicket(signedT)
	assert.EqualError(err, "stub broker redeem error")

	used, err := b.IsUsedTicket(signedT.Ticket)
	assert.NoError(err)
	assert.False(used)
}

func TestRedeemWinningTicket_SingleTicket_CheckTxError(t *testing.T) {
	claimant, b, smgr, tm := senderMonitorFixture()
	addr := RandAddress()
	smgr.info[addr] = &SenderInfo{
		Deposit:       big.NewInt(500),
		WithdrawRound: big.NewInt(0),
		Reserve: &ReserveInfo{
			FundsRemaining:        big.NewInt(1000),
			ClaimedInCurrentRound: big.NewInt(0),
		},
	}

	ts := newStubTicketStore()
	smgr.claimedReserve[addr] = big.NewInt(100)
	sm := NewSenderMonitor(claimant, b, smgr, tm, ts, 5*time.Minute, 3600)
	sm.Start()
	defer sm.Stop()
	b.checkTxErr = errors.New("checktx error")
	assert := assert.New(t)

	signedT := defaultSignedTicket(addr, uint32(0))

	err := sm.(*senderMonitor).redeemWinningTicket(signedT)
	assert.EqualError(err, b.checkTxErr.Error())
}

func TestRedeemWinningTicket_SingleTicket(t *testing.T) {
	claimant, b, smgr, tm := senderMonitorFixture()
	addr := RandAddress()
	smgr.info[addr] = &SenderInfo{
		Deposit:       big.NewInt(500),
		WithdrawRound: big.NewInt(0),
		Reserve: &ReserveInfo{
			FundsRemaining:        big.NewInt(1000),
			ClaimedInCurrentRound: big.NewInt(0),
		},
	}

	ts := newStubTicketStore()
	smgr.claimedReserve[addr] = big.NewInt(100)
	sm := NewSenderMonitor(claimant, b, smgr, tm, ts, 5*time.Minute, 3600)
	sm.Start()
	defer sm.Stop()
	assert := assert.New(t)

	signedT := defaultSignedTicket(addr, uint32(0))

	err := sm.(*senderMonitor).redeemWinningTicket(signedT)
	assert.Nil(err)

	ok, err := b.IsUsedTicket(signedT.Ticket)
	assert.Nil(err)
	assert.True(ok)
}

func TestRedeemWinningTicket_MaxFloatError(t *testing.T) {
	claimant, b, smgr, tm := senderMonitorFixture()
	addr := RandAddress()

	ts := newStubTicketStore()
	sm := NewSenderMonitor(claimant, b, smgr, tm, ts, 5*time.Minute, 3600)
	sm.Start()
	defer sm.Stop()

	assert := assert.New(t)

	signedT := defaultSignedTicket(addr, uint32(0))
	smgr.err = errors.New("maxfloat err")
	err := sm.(*senderMonitor).redeemWinningTicket(signedT)
	assert.EqualError(err, smgr.err.Error())
}

func TestRedeemWinningTicket_InsufficientMaxFloat_QueueTicket(t *testing.T) {
	claimant, b, smgr, tm := senderMonitorFixture()
	addr := RandAddress()
	smgr.info[addr] = &SenderInfo{
		Deposit:       big.NewInt(500),
		WithdrawRound: big.NewInt(0),
		Reserve: &ReserveInfo{
			FundsRemaining:        big.NewInt(525),
			ClaimedInCurrentRound: big.NewInt(0),
		},
	}

	ts := newStubTicketStore()
	smgr.claimedReserve[addr] = big.NewInt(100)
	sm := NewSenderMonitor(claimant, b, smgr, tm, ts, 5*time.Minute, 3600)
	sm.Start()
	defer sm.Stop()

	assert := assert.New(t)

	signedT := defaultSignedTicket(addr, uint32(0))

	err := sm.(*senderMonitor).redeemWinningTicket(signedT)
	assert.EqualError(err, fmt.Sprintf("insufficient max float sender=%v faceValue=%v maxFloat=%v", addr.Hex(), signedT.FaceValue, new(big.Int).Sub(new(big.Int).Div(smgr.info[addr].Reserve.FundsRemaining, tm.transcoderPoolSize), smgr.claimedReserve[addr])))
}

func TestRedeemWinningTicket_addFloatError(t *testing.T) {
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

	ts := newStubTicketStore()
	smgr.claimedReserve[addr] = big.NewInt(100)
	sm := NewSenderMonitor(claimant, b, smgr, tm, ts, 5*time.Minute, 3600)
	sm.Start()
	defer sm.Stop()

	assert := assert.New(t)

	signedT := defaultSignedTicket(addr, uint32(0))
	sm.(*senderMonitor).ensureCache(addr)
	sm.(*senderMonitor).senders[addr].pendingAmount = big.NewInt(-100)

	err := sm.(*senderMonitor).redeemWinningTicket(signedT)
	assert.EqualError(err, "cannot subtract from insufficient pendingAmount")
}

func TestMonitorMaxFloat(t *testing.T) {
	claimant, b, smgr, tm := senderMonitorFixture()
	addr := RandAddress()
	initialReserve := big.NewInt(500)
	smgr.info[addr] = &SenderInfo{
		Reserve: &ReserveInfo{
			FundsRemaining:        initialReserve,
			ClaimedInCurrentRound: big.NewInt(0),
		},
	}
	smgr.claimedReserve[addr] = big.NewInt(0)
	tm.transcoderPoolSize = big.NewInt(1)
	sm := NewSenderMonitor(claimant, b, smgr, tm, newStubTicketStore(), 5*time.Minute, 3600)
	sm.Start()
	defer sm.Stop()

	assert := assert.New(t)
	require := require.New(t)

	sink := make(chan *big.Int, 10)
	sub := sm.MonitorMaxFloat(addr, sink)
	defer sub.Unsubscribe()

	amount := big.NewInt(100)
	err := sm.(*senderMonitor).subFloat(addr, amount)
	require.Nil(err)
	mfu := <-sink
	newMaxFloat := new(big.Int).Sub(initialReserve, amount)
	assert.Equal(newMaxFloat, mfu)

	amount = big.NewInt(50)
	err = sm.(*senderMonitor).addFloat(addr, amount)
	require.Nil(err)
	mfu = <-sink
	newMaxFloat = new(big.Int).Add(newMaxFloat, amount)
	assert.Equal(newMaxFloat, mfu)
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
