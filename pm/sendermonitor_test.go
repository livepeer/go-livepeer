package pm

import (
	"context"
	"fmt"
	"math/big"
	"testing"
	"time"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/golang/glog"
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
	cfg, b, smgr, tm := localSenderMonitorFixture()
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
	sm := NewSenderMonitor(cfg, b, smgr, tm, newStubTicketStore())
	sm.Start()
	defer sm.Stop()

	assert := assert.New(t)

	// Test ClaimedReserve() error
	smgr.claimedReserveErr = errors.New("ClaimedReserve error")

	_, err := sm.MaxFloat(RandAddress())
	assert.EqualError(err, "ClaimedReserve error")

	// Test value cached

	smgr.claimedReserveErr = nil
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

func TestMaxFloat_MinDepositPendingRatio(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	cfg, b, smgr, tm := localSenderMonitorFixture()
	addr := ethcommon.BytesToAddress([]byte("foo"))

	smgr.info[addr] = &SenderInfo{
		Reserve: &ReserveInfo{
			FundsRemaining:        big.NewInt(500),
			ClaimedInCurrentRound: big.NewInt(0),
		},
	}
	smgr.claimedReserve[addr] = big.NewInt(0)
	tm.transcoderPoolSize = big.NewInt(1)

	sm := NewSenderMonitor(cfg, b, smgr, tm, newStubTicketStore())

	reserveAlloc := smgr.info[addr].Reserve.FundsRemaining
	pendingAmount := big.NewInt(100)
	// Set pendingAmount
	sm.subFloat(addr, pendingAmount)
	// Set deposit pending ratio above the min ratio
	smgr.info[addr].Deposit = new(big.Int).Mul(pendingAmount, big.NewInt(int64(minDepositPendingRatio)+1))
	mf, err := sm.MaxFloat(addr)
	require.Nil(err)
	assert.Equal(reserveAlloc, mf)

	// Set deposit pending ratio equal to the min ratio
	smgr.info[addr].Deposit = new(big.Int).Mul(pendingAmount, big.NewInt(int64(minDepositPendingRatio)))
	mf, err = sm.MaxFloat(addr)
	require.Nil(err)
	assert.Equal(reserveAlloc, mf)

	// Set deposit pending ratio below the min ratio
	smgr.info[addr].Deposit = new(big.Int).Mul(pendingAmount, big.NewInt(int64(minDepositPendingRatio)-1))
	mf, err = sm.MaxFloat(addr)
	require.Nil(err)
	assert.Equal(new(big.Int).Sub(reserveAlloc, pendingAmount), mf)

	// Set deposit to 0
	smgr.info[addr].Deposit = big.NewInt(0)
	mf, err = sm.MaxFloat(addr)
	require.Nil(err)
	assert.Equal(new(big.Int).Sub(reserveAlloc, pendingAmount), mf)

	// Set pendingAmount to 0
	require.Nil(sm.addFloat(addr, pendingAmount))
	mf, err = sm.MaxFloat(addr)
	require.Nil(err)
	assert.Equal(reserveAlloc, mf)
}

func TestSubFloat(t *testing.T) {
	cfg, b, smgr, tm := localSenderMonitorFixture()
	addr := RandAddress()
	smgr.info[addr] = &SenderInfo{
		Deposit:       big.NewInt(0),
		WithdrawRound: big.NewInt(0),
		Reserve: &ReserveInfo{
			FundsRemaining:        big.NewInt(500),
			ClaimedInCurrentRound: big.NewInt(0),
		},
	}
	smgr.claimedReserve[addr] = big.NewInt(100)
	tm.transcoderPoolSize = big.NewInt(50)
	sm := NewSenderMonitor(cfg, b, smgr, tm, newStubTicketStore())
	sm.Start()
	defer sm.Stop()

	assert := assert.New(t)
	require := require.New(t)

	reserve := new(big.Int).Add(smgr.info[addr].Reserve.FundsRemaining, smgr.info[addr].Reserve.ClaimedInCurrentRound)
	reserveAlloc := new(big.Int).Sub(new(big.Int).Div(reserve, tm.transcoderPoolSize), smgr.claimedReserve[addr])

	amount := big.NewInt(5)
	sm.subFloat(addr, amount)
	mf, err := sm.MaxFloat(addr)
	require.Nil(err)
	assert.Equal(new(big.Int).Sub(reserveAlloc, amount), mf)

	sm.subFloat(addr, amount)
	assert.Nil(err)

	mf, err = sm.MaxFloat(addr)
	require.Nil(err)
	assert.Equal(
		new(big.Int).Sub(reserveAlloc, new(big.Int).Mul(amount, big.NewInt(2))),
		mf,
	)
}

func TestAddFloat(t *testing.T) {
	cfg, b, smgr, tm := localSenderMonitorFixture()
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
	sm := NewSenderMonitor(cfg, b, smgr, tm, newStubTicketStore())
	sm.Start()
	defer sm.Stop()

	assert := assert.New(t)
	require := require.New(t)

	// Test value not cached and insufficient pendingAmount error
	smgr.err = nil
	reserve := new(big.Int).Add(smgr.info[addr].Reserve.FundsRemaining, smgr.info[addr].Reserve.ClaimedInCurrentRound)
	reserveAlloc := new(big.Int).Sub(new(big.Int).Div(reserve, tm.transcoderPoolSize), smgr.claimedReserve[addr])

	amount := big.NewInt(20)
	err := sm.addFloat(addr, amount)
	assert.EqualError(err, "cannot subtract from insufficient pendingAmount")

	// Test value cached and no pendingAmount error
	sm.subFloat(addr, amount)

	err = sm.addFloat(addr, amount)
	assert.Nil(err)

	mf, err := sm.MaxFloat(addr)
	require.Nil(err)
	assert.Equal(mf, reserveAlloc)

	// Test cached value update
	smgr.info[addr].Reserve.FundsRemaining = big.NewInt(1000)
	reserve = new(big.Int).Add(smgr.info[addr].Reserve.FundsRemaining, smgr.info[addr].Reserve.ClaimedInCurrentRound)
	reserveAlloc = new(big.Int).Sub(new(big.Int).Div(reserve, tm.transcoderPoolSize), smgr.claimedReserve[addr])

	sm.subFloat(addr, amount)

	err = sm.addFloat(addr, amount)
	assert.Nil(err)

	mf, err = sm.MaxFloat(addr)
	require.Nil(err)
	assert.Equal(reserveAlloc, mf)
}

func TestQueueTicketAndSignalNewBlock(t *testing.T) {
	cfg, b, smgr, tm := localSenderMonitorFixture()
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
	sm := NewSenderMonitor(cfg, b, smgr, tm, ts)
	sm.Start()
	defer sm.Stop()

	assert := assert.New(t)

	// Test queue ticket
	// test fail
	ts.storeShouldFail = true
	assert.EqualError(sm.QueueTicket(defaultSignedTicket(addr, uint32(0))), "stub TicketStore store error")
	ts.storeShouldFail = false

	signedT := defaultSignedTicket(addr, uint32(0))
	err := sm.QueueTicket(signedT)
	assert.Nil(err)
	time.Sleep(20 * time.Millisecond)
	qlen, err := sm.senders[addr].queue.Length()
	assert.Nil(err)
	assert.Equal(qlen, 1)

	tm.blockNumSink <- big.NewInt(5)
	time.Sleep(20 * time.Millisecond)

	// check that ticket is now removed from queue
	qlen, err = sm.senders[addr].queue.Length()
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
	qlen, err = sm.senders[addr].queue.Length()
	assert.Nil(err)
	assert.Equal(qlen, 1)
	tm.blockNumSink <- big.NewInt(5)
	time.Sleep(20 * time.Millisecond)

	signedT3 := defaultSignedTicket(addr2, uint32(3))
	sm.QueueTicket(signedT3)
	time.Sleep(20 * time.Millisecond)
	qlen, err = sm.senders[addr2].queue.Length()
	assert.Nil(err)
	assert.Equal(qlen, 1)
	tm.blockNumSink <- big.NewInt(5)
	time.Sleep(20 * time.Millisecond)

	assert.True(b.IsUsedTicket(signedT2.Ticket))
	assert.True(b.IsUsedTicket(signedT3.Ticket))
}

func TestCleanup(t *testing.T) {
	cfg, b, smgr, tm := localSenderMonitorFixture()
	cfg.TTL = 5
	sm := NewSenderMonitor(cfg, b, smgr, tm, newStubTicketStore())
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
	sm.cleanup()
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

	sm.cleanup()

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
	err = sm.addFloat(addr2, big.NewInt(0))
	require.Nil(err)

	increaseTime(1)

	// Change stub broker value
	// SenderMonitor should:
	// - Use new value for addr1 because it was cleaned up
	// - Use cached value for addr2 because it was accessed recently via addFloat()
	reserve4 := big.NewInt(101)
	smgr.info[addr1].Reserve.FundsRemaining = reserve4

	sm.cleanup()

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
	sm.subFloat(addr1, big.NewInt(0))

	increaseTime(1)

	// Change stub broker value
	// SenderMonitor should:
	// - Use cached value for addr1 because it was accessed recently via subFloat()
	// - Use new value for addr2 because it was cleaned up
	reserve5 := big.NewInt(999)
	smgr.info[addr2].Reserve.FundsRemaining = reserve5

	sm.cleanup()

	mf1, err = sm.MaxFloat(addr1)
	require.Nil(err)
	mf2, err = sm.MaxFloat(addr2)
	require.Nil(err)

	expectedReserve4 := new(big.Int).Add(smgr.info[addr2].Reserve.FundsRemaining, smgr.info[addr2].Reserve.ClaimedInCurrentRound)
	expectedAlloc4 := new(big.Int).Sub(new(big.Int).Div(expectedReserve4, tm.transcoderPoolSize), smgr.claimedReserve[addr2])
	assert.Equal(expectedAlloc3, mf1)
	assert.Equal(expectedAlloc4, mf2)

	// Add a subscription for addr1
	// It should not be cleaned up
	sink := make(chan struct{})
	sub := sm.senders[addr1].subScope.Track(sm.senders[addr1].subFeed.Subscribe(sink))
	defer sub.Unsubscribe()
	increaseTime(10)
	sm.cleanup()
	assert.NotNil(sm.senders[addr1])
	assert.Nil(sm.senders[addr2])
}

func TestReserveAlloc(t *testing.T) {
	assert := assert.New(t)
	cfg, b, smgr, tm := localSenderMonitorFixture()
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
	sm := NewSenderMonitor(cfg, b, smgr, tm, newStubTicketStore())

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
	cfg, b, smgr, tm := localSenderMonitorFixture()
	addr := RandAddress()
	smgr.info[addr] = &SenderInfo{
		WithdrawRound: big.NewInt(10),
	}
	sm := NewSenderMonitor(cfg, b, smgr, tm, newStubTicketStore())
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

func TestAvailableFunds(t *testing.T) {
	assert := assert.New(t)

	cfg, b, smgr, tm := localSenderMonitorFixture()

	addr := ethcommon.BytesToAddress([]byte("foo"))
	deposit := big.NewInt(500)
	reserve := big.NewInt(1000)
	smgr.info[addr] = &SenderInfo{
		Deposit:       deposit,
		WithdrawRound: big.NewInt(0),
		Reserve: &ReserveInfo{
			FundsRemaining:        reserve,
			ClaimedInCurrentRound: big.NewInt(0),
		},
	}
	smgr.claimedReserve[addr] = big.NewInt(0)
	tm.transcoderPoolSize = big.NewInt(1)

	sm := NewSenderMonitor(cfg, b, smgr, tm, newStubTicketStore())

	// Trigger GetSenderInfo() error
	smgr.err = errors.New("GetSenderInfo error")
	_, err := sm.availableFunds(addr)
	assert.EqualError(err, smgr.err.Error())

	smgr.err = nil

	// When pendingAmount = 0
	expFunds := new(big.Int).Add(deposit, reserve)
	funds, err := sm.availableFunds(addr)
	assert.Nil(err)
	assert.Equal(expFunds, funds)

	// When pendingAmount > 0
	pendingAmount := big.NewInt(int64(100))
	sm.subFloat(addr, pendingAmount)
	expFunds.Sub(expFunds, pendingAmount)
	funds, err = sm.availableFunds(addr)
	assert.Nil(err)
	assert.Equal(expFunds, funds)
}

func TestRedeemWinningTicket_IsUsedTicket(t *testing.T) {
	claimant, b, smgr, tm := localSenderMonitorFixture()
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
	sm := NewSenderMonitor(claimant, b, smgr, tm, ts)
	sm.Start()
	defer sm.Stop()
	assert := assert.New(t)

	signedT := defaultSignedTicket(addr, uint32(0))

	// test error
	b.isUsedErr = errors.New("isUsed error")
	tx, err := sm.redeemWinningTicket(signedT)
	assert.Nil(tx)
	assert.EqualError(err, b.isUsedErr.Error())

	// test used
	b.isUsedErr = nil
	b.usedTickets[signedT.Hash()] = true
	tx, err = sm.redeemWinningTicket(signedT)
	assert.Nil(tx)
	assert.EqualError(err, errIsUsedTicket.Error())

	// test not used
	b.usedTickets[signedT.Hash()] = false
	tx, err = sm.redeemWinningTicket(signedT)
	assert.Nil(err)
	assert.NotNil(tx)
	assert.True(b.IsUsedTicket(signedT.Ticket))
}

func TestRedeemWinningTicket_CheckAvailableFundsAndFaceValue(t *testing.T) {
	assert := assert.New(t)

	cfg, b, smgr, tm := localSenderMonitorFixture()
	addr := ethcommon.BytesToAddress([]byte("foo"))
	smgr.info[addr] = &SenderInfo{
		Deposit:       big.NewInt(500),
		WithdrawRound: big.NewInt(0),
		Reserve: &ReserveInfo{
			FundsRemaining:        big.NewInt(1000),
			ClaimedInCurrentRound: big.NewInt(0),
		},
	}
	smgr.claimedReserve[addr] = big.NewInt(0)

	ts := newStubTicketStore()
	sm := NewSenderMonitor(cfg, b, smgr, tm, ts)

	signedT := defaultSignedTicket(addr, uint32(0))

	// Trigger availableFunds() error
	smgr.err = errors.New("GetSenderInfo() error")
	_, err := sm.redeemWinningTicket(signedT)
	assert.EqualError(err, smgr.err.Error())

	smgr.err = nil

	// Trigger SuggestGasPrice() error
	gasPriceErr := errors.New("SuggestGasPrice() error")
	cfg.SuggestGasPrice = func(ctx context.Context) (*big.Int, error) { return nil, gasPriceErr }
	sm = NewSenderMonitor(cfg, b, smgr, tm, ts)
	_, err = sm.redeemWinningTicket(signedT)
	assert.EqualError(err, gasPriceErr.Error())

	// Trigger SuggestGasPrice() timeout
	timeoutErr := errors.New("SuggestGasPrice() timeout")
	cfg.RPCTimeout = 20 * time.Millisecond
	cfg.SuggestGasPrice = func(ctx context.Context) (*big.Int, error) {
		timer := time.NewTimer(100 * time.Millisecond)
		select {
		case <-ctx.Done():
			return nil, timeoutErr
		case <-timer.C:
		}
		return nil, errors.New("incorrect timeout error")
	}
	sm = NewSenderMonitor(cfg, b, smgr, tm, ts)
	_, err = sm.redeemWinningTicket(signedT)
	assert.EqualError(err, timeoutErr.Error())

	// Trigger insufficient funds to cover redeem tx cost error when availableFunds < txCost
	cfg.RedeemGas = 1
	cfg.SuggestGasPrice = func(ctx context.Context) (*big.Int, error) { return big.NewInt(1000000000), nil }
	sm = NewSenderMonitor(cfg, b, smgr, tm, ts)
	_, err = sm.redeemWinningTicket(signedT)
	assert.Contains(err.Error(), "insufficient sender funds")

	// Trigger insufficient funds to cover redeem tx cost error when availableFunds = txCost
	funds, err := sm.availableFunds(addr)
	require.Nil(t, err)
	cfg.RedeemGas = 1
	cfg.SuggestGasPrice = func(ctx context.Context) (*big.Int, error) { return funds, nil }
	sm = NewSenderMonitor(cfg, b, smgr, tm, ts)
	_, err = sm.redeemWinningTicket(signedT)
	assert.Contains(err.Error(), "insufficient sender funds")

	// Trigger insufficient face value to cover redeem tx cost error when face value < txCost
	txCost := new(big.Int).Sub(funds, big.NewInt(1))
	cfg.RedeemGas = 1
	cfg.SuggestGasPrice = func(ctx context.Context) (*big.Int, error) { return txCost, nil }
	badSignedT := defaultSignedTicket(addr, uint32(0))
	badSignedT.FaceValue = new(big.Int).Sub(txCost, big.NewInt(1))
	sm = NewSenderMonitor(cfg, b, smgr, tm, ts)
	_, err = sm.redeemWinningTicket(signedT)
	assert.Contains(err.Error(), "insufficient ticket face value")

	// Trigger insufficient face value to cover redeem tx cost error when face value = txCost
	badSignedT.FaceValue = txCost
	sm = NewSenderMonitor(cfg, b, smgr, tm, ts)
	_, err = sm.redeemWinningTicket(signedT)
	assert.Contains(err.Error(), "insufficient ticket face value")

	// Pass available funds and face value check when availableFunds > txCost and face value > txCost
	cfg.RedeemGas = 0
	cfg.SuggestGasPrice = func(ctx context.Context) (*big.Int, error) { return big.NewInt(0), nil }
	sm = NewSenderMonitor(cfg, b, smgr, tm, ts)
	tx, err := sm.redeemWinningTicket(signedT)
	assert.Nil(err)
	assert.NotNil(tx)
}

func TestRedeemWinningTicket_SingleTicket_RedeemError(t *testing.T) {
	cfg, b, smgr, tm := localSenderMonitorFixture()
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
	sm := NewSenderMonitor(cfg, b, smgr, tm, ts)
	sm.Start()
	defer sm.Stop()

	assert := assert.New(t)

	signedT := defaultSignedTicket(addr, uint32(0))

	b.redeemShouldFail = true
	tx, err := sm.redeemWinningTicket(signedT)
	assert.EqualError(err, "stub broker redeem error")
	assert.Nil(tx)
	used, err := b.IsUsedTicket(signedT.Ticket)
	assert.NoError(err)
	assert.False(used)
}

func TestRedeemWinningTicket_SingleTicket_CheckTxError(t *testing.T) {
	cfg, b, smgr, tm := localSenderMonitorFixture()
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
	sm := NewSenderMonitor(cfg, b, smgr, tm, ts)
	sm.Start()
	defer sm.Stop()
	expErr := errors.New("checktx error")
	b.checkTxErr = expErr
	assert := assert.New(t)

	signedT := defaultSignedTicket(addr, uint32(0))

	tx, err := sm.redeemWinningTicket(signedT)
	assert.NotNil(tx)
	assert.Equal(expErr, err)
}

func TestRedeemWinningTicket_SingleTicket(t *testing.T) {
	cfg, b, smgr, tm := localSenderMonitorFixture()
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
	sm := NewSenderMonitor(cfg, b, smgr, tm, ts)
	sm.Start()
	defer sm.Stop()
	assert := assert.New(t)

	signedT := defaultSignedTicket(addr, uint32(0))

	tx, err := sm.redeemWinningTicket(signedT)
	assert.Nil(err)
	assert.NotNil(tx)

	ok, err := b.IsUsedTicket(signedT.Ticket)
	assert.Nil(err)
	assert.True(ok)
}

func TestRedeemWinningTicket_addFloatError(t *testing.T) {
	cfg, b, smgr, tm := localSenderMonitorFixture()
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
	sm := NewSenderMonitor(cfg, b, smgr, tm, ts)
	sm.Start()
	defer sm.Stop()

	assert := assert.New(t)

	signedT := defaultSignedTicket(addr, uint32(0))
	sm.ensureCache(addr)
	sm.senders[addr].pendingAmount = big.NewInt(-100)

	errLogsBefore := glog.Stats.Error.Lines()
	tx, err := sm.redeemWinningTicket(signedT)
	assert.NotNil(tx)
	errLogsAfter := glog.Stats.Error.Lines()
	assert.Nil(err)
	assert.Greater(errLogsAfter, errLogsBefore)
}

func TestSubscribeMaxFloatChange(t *testing.T) {
	cfg, b, smgr, tm := localSenderMonitorFixture()
	addr := RandAddress()
	initialReserve := big.NewInt(500)
	smgr.info[addr] = &SenderInfo{
		Deposit: big.NewInt(0),
		Reserve: &ReserveInfo{
			FundsRemaining:        initialReserve,
			ClaimedInCurrentRound: big.NewInt(0),
		},
	}
	smgr.claimedReserve[addr] = big.NewInt(0)
	tm.transcoderPoolSize = big.NewInt(1)
	sm := NewSenderMonitor(cfg, b, smgr, tm, newStubTicketStore())
	sm.Start()
	defer sm.Stop()

	assert := assert.New(t)
	require := require.New(t)

	sink := make(chan struct{}, 10)
	sub := sm.SubscribeMaxFloatChange(addr, sink)
	defer sub.Unsubscribe()

	amount := big.NewInt(100)
	sm.subFloat(addr, amount)
	<-sink
	newMaxFloat := new(big.Int).Sub(initialReserve, amount)
	mf, err := sm.MaxFloat(addr)
	assert.Nil(err)
	assert.Equal(newMaxFloat, mf)

	amount = big.NewInt(50)
	err = sm.addFloat(addr, amount)
	require.Nil(err)
	<-sink
	newMaxFloat = new(big.Int).Add(newMaxFloat, amount)
	mf, err = sm.MaxFloat(addr)
	assert.Nil(err)
	assert.Equal(newMaxFloat, mf)
}

func TestWatchReserveChange(t *testing.T) {
	assert := assert.New(t)
	cfg, b, smgr, tm := localSenderMonitorFixture()
	sm := NewSenderMonitor(cfg, b, smgr, tm, newStubTicketStore())
	go sm.watchReserveChange()
	defer sm.Stop()

	sender := RandAddress()

	time.Sleep(time.Second)

	sink := make(chan struct{})
	sub := sm.SubscribeMaxFloatChange(sender, sink)
	defer sub.Unsubscribe()
	time.Sleep(time.Second)

	smgr.reserveChangeSink <- sender

	_, ok := <-sink
	assert.True(ok)
}

func TestWatchPoolSizeChange(t *testing.T) {
	assert := assert.New(t)
	cfg, b, smgr, tm := localSenderMonitorFixture()
	sm := NewSenderMonitor(cfg, b, smgr, tm, newStubTicketStore())

	sender := RandAddress()
	tm.transcoderPoolSize = big.NewInt(10)

	go sm.watchPoolSizeChange()
	defer sm.Stop()
	time.Sleep(time.Second)

	tm.transcoderPoolSize = big.NewInt(20)

	sink := make(chan struct{})
	sub := sm.SubscribeMaxFloatChange(sender, sink)
	defer sub.Unsubscribe()
	time.Sleep(time.Second)

	tm.roundSink <- types.Log{}

	_, ok := <-sink
	assert.True(ok)
}

func localSenderMonitorFixture() (*LocalSenderMonitorConfig, *stubBroker, *stubSenderManager, *stubTimeManager) {
	cfg := stubLocalSenderMonitorCfg()
	b := newStubBroker()
	smgr := newStubSenderManager()
	tm := &stubTimeManager{
		transcoderPoolSize: big.NewInt(5),
		round:              big.NewInt(100),
	}
	return cfg, b, smgr, tm
}

func stubLocalSenderMonitorCfg() *LocalSenderMonitorConfig {
	return &LocalSenderMonitorConfig{
		Claimant:        RandAddress(),
		CleanupInterval: 5 * time.Minute,
		TTL:             3600,
		RedeemGas:       0,
		SuggestGasPrice: func(ctx context.Context) (*big.Int, error) {
			return big.NewInt(0), nil
		},
		RPCTimeout: 5 * time.Minute,
	}
}
