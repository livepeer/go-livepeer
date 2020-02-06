package watchers

import (
	"errors"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/livepeer/go-livepeer/eth"
	"github.com/livepeer/go-livepeer/eth/blockwatch"
	"github.com/livepeer/go-livepeer/pm"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetAndSetSenderInfo(t *testing.T) {
	assert := assert.New(t)
	sw := &SenderWatcher{
		senders: make(map[ethcommon.Address]*pm.SenderInfo),
		lpEth: &eth.StubClient{
			SenderInfo: &pm.SenderInfo{
				Deposit: big.NewInt(10),
				Reserve: &pm.ReserveInfo{
					FundsRemaining:        big.NewInt(5),
					ClaimedInCurrentRound: big.NewInt(0),
				},
			},
		},
	}

	info := &pm.SenderInfo{
		Deposit: big.NewInt(1000000000000000000),
		Reserve: &pm.ReserveInfo{
			FundsRemaining:        big.NewInt(1000000000000000000),
			ClaimedInCurrentRound: big.NewInt(0),
		},
	}
	sender := pm.RandAddress()

	sw.setSenderInfo(sender, info)
	entry, err := sw.GetSenderInfo(sender)
	assert.Nil(err)
	assert.Equal(sw.senders[sender], info)
	assert.Equal(info, entry)

	// If map entry is nil and we call get, make RPC call and set the returned data
	sw.Clear(sender)
	info, err = sw.GetSenderInfo(sender)
	assert.Nil(err)
	assert.Zero(info.Deposit.Cmp(big.NewInt(10)))
	assert.Zero(info.Reserve.FundsRemaining.Cmp(big.NewInt(5)))
	info = sw.senders[sender]
	assert.Zero(info.Deposit.Cmp(big.NewInt(10)))
	assert.Zero(info.Reserve.FundsRemaining.Cmp(big.NewInt(5)))
}

func TestSenderWatcher_Clear(t *testing.T) {
	assert := assert.New(t)
	stubClientSenderInfo := &pm.SenderInfo{
		Deposit: big.NewInt(10),
		Reserve: &pm.ReserveInfo{
			FundsRemaining:        big.NewInt(5),
			ClaimedInCurrentRound: big.NewInt(0),
		},
	}
	sw := &SenderWatcher{
		senders:        make(map[ethcommon.Address]*pm.SenderInfo),
		claimedReserve: make(map[ethcommon.Address]*big.Int),
		lpEth: &eth.StubClient{
			SenderInfo: stubClientSenderInfo,
		},
	}
	info := &pm.SenderInfo{
		Deposit: big.NewInt(1000000000000000000),
		Reserve: &pm.ReserveInfo{
			FundsRemaining:        big.NewInt(1000000000000000000),
			ClaimedInCurrentRound: big.NewInt(0),
		},
	}
	sender := pm.RandAddress()

	sw.setSenderInfo(sender, info)
	claimed := big.NewInt(100)
	sw.claimedReserve[sender] = claimed

	getInfo, err := sw.GetSenderInfo(sender)
	assert.Nil(err)
	assert.Equal(info, getInfo)
	getClaimed, err := sw.ClaimedReserve(sender, ethcommon.Address{})
	assert.Nil(err)
	assert.Zero(claimed.Cmp(getClaimed))

	sw.Clear(sender)

	getInfo = sw.senders[sender]
	assert.Nil(getInfo)
	// on calling GetSenderInfo should be back to RPC stub values
	getInfo, err = sw.GetSenderInfo(sender)
	assert.Nil(err)
	assert.Equal(getInfo, stubClientSenderInfo)
	getClaimed, err = sw.ClaimedReserve(sender, ethcommon.Address{})
	assert.Nil(err)
	assert.Nil(getClaimed)

}

func TestSenderWatcher_ClaimedReserve(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	lpEth := &eth.StubClient{}
	watcher := &stubBlockWatcher{}

	tw := &stubTimeWatcher{}

	sw, err := NewSenderWatcher(stubTicketBrokerAddr, watcher, lpEth, tw)
	require.Nil(err)

	// if not existant on map; make RPC call and init the map
	_, ok := sw.claimedReserve[stubSender]
	require.False(ok)
	lpEth.ClaimedAmount = big.NewInt(5000)
	claimed, err := sw.ClaimedReserve(stubSender, stubClaimant)
	assert.Nil(err)
	assert.Zero(lpEth.ClaimedAmount.Cmp(claimed))

	// if existant on map return the map value
	sw.claimedReserve[stubSender] = big.NewInt(10000)
	claimed, err = sw.ClaimedReserve(stubSender, stubClaimant)
	assert.Nil(err)
	assert.Equal(big.NewInt(10000), claimed)

	// if RPC error return error
	lpEth.ClaimedAmount = nil
	sw.claimedReserve[stubSender] = nil
	expectedErr := errors.New("ClaimedReserve RPC error")
	lpEth.ClaimedReserveError = expectedErr
	claimed, err = sw.ClaimedReserve(stubSender, stubClaimant)
	assert.Nil(claimed)
	assert.Contains(err.Error(), expectedErr.Error())
}

func TestSenderWatcher_WatchAndStop(t *testing.T) {
	assert := assert.New(t)
	lpEth := &eth.StubClient{}
	watcher := &stubBlockWatcher{}

	tw := &stubTimeWatcher{}

	sw, err := NewSenderWatcher(stubTicketBrokerAddr, watcher, lpEth, tw)
	assert.Nil(err)

	go sw.Watch()
	time.Sleep(2 * time.Millisecond)
	sw.Stop()
	time.Sleep(2 * time.Millisecond)
	assert.True(watcher.sub.unsubscribed)
}

func TestSenderWatcher_HandleLog(t *testing.T) {
	assert := assert.New(t)
	lpEth := &eth.StubClient{}
	watcher := &stubBlockWatcher{}

	tw := &stubTimeWatcher{}

	sw, err := NewSenderWatcher(stubTicketBrokerAddr, watcher, lpEth, tw)
	assert.Nil(err)

	// Test unknown event
	log := newStubBaseLog()
	log.Topics = []ethcommon.Hash{ethcommon.BytesToHash([]byte("foo"))}

	err = sw.handleLog(log)
	assert.Nil(err)
}

func TestFundDepositEvent(t *testing.T) {
	assert := assert.New(t)
	startDeposit := big.NewInt(10)
	lpEth := &eth.StubClient{
		SenderInfo: &pm.SenderInfo{
			Deposit: big.NewInt(10),
			Reserve: &pm.ReserveInfo{
				FundsRemaining:        big.NewInt(5),
				ClaimedInCurrentRound: big.NewInt(0),
			},
		},
	}
	watcher := &stubBlockWatcher{}

	tw := &stubTimeWatcher{}

	sw, err := NewSenderWatcher(stubTicketBrokerAddr, watcher, lpEth, tw)
	assert.Nil(err)

	header := defaultMiniHeader()
	newDepositEvent := newStubDepositFundedLog()
	header.Logs = append(header.Logs, newDepositEvent)

	blockEvent := &blockwatch.Event{
		Type:        blockwatch.Added,
		BlockHeader: header,
	}

	go sw.Watch()
	defer sw.Stop()
	time.Sleep(2 * time.Millisecond)

	// If map entry is nil don't handle event
	watcher.sink <- []*blockwatch.Event{blockEvent}
	time.Sleep(2 * time.Millisecond)
	_, ok := sw.senders[stubSender]
	assert.False(ok)

	// if block removed, make RPC call
	blockEvent.Type = blockwatch.Removed
	sw.senders[stubSender] = &pm.SenderInfo{}
	watcher.sink <- []*blockwatch.Event{blockEvent}
	time.Sleep(2 * time.Millisecond)
	info, ok := sw.senders[stubSender]
	assert.True(ok)
	assert.Zero(info.Deposit.Cmp(startDeposit))

	// If map entry exists parse event log
	blockEvent.Type = blockwatch.Added
	expectedDeposit := new(big.Int).Add(startDeposit, new(big.Int).SetBytes(newDepositEvent.Data))
	watcher.sink <- []*blockwatch.Event{blockEvent}
	time.Sleep(2 * time.Millisecond)
	info, err = sw.GetSenderInfo(stubSender)
	assert.Nil(err)
	assert.Zero(info.Deposit.Cmp(expectedDeposit))

	// If we don't care about the address, don't handle the event
	s := pm.RandAddress()
	sender := ethcommon.LeftPadBytes(s.Bytes(), 32)
	var senderTopic ethcommon.Hash
	copy(senderTopic[:], sender[:])
	newDepositEvent.Topics[1] = senderTopic
	watcher.sink <- []*blockwatch.Event{blockEvent}
	time.Sleep(2 * time.Millisecond)
	_, ok = sw.senders[s]
	assert.False(ok)
}

func TestFundReserveEvent(t *testing.T) {
	assert := assert.New(t)
	startDeposit := big.NewInt(10)
	startReserve := big.NewInt(5)
	lpEth := &eth.StubClient{
		SenderInfo: &pm.SenderInfo{
			Deposit: big.NewInt(10),
			Reserve: &pm.ReserveInfo{
				FundsRemaining:        big.NewInt(5),
				ClaimedInCurrentRound: big.NewInt(0),
			},
		},
	}
	watcher := &stubBlockWatcher{}

	tw := &stubTimeWatcher{}

	sw, err := NewSenderWatcher(stubTicketBrokerAddr, watcher, lpEth, tw)
	assert.Nil(err)

	header := defaultMiniHeader()
	newReserveEvent := newStubReserveFundedLog()
	header.Logs = append(header.Logs, newReserveEvent)

	blockEvent := &blockwatch.Event{
		Type:        blockwatch.Added,
		BlockHeader: header,
	}

	go sw.Watch()
	defer sw.Stop()
	time.Sleep(2 * time.Millisecond)

	// If map entry is nil don't handle event
	watcher.sink <- []*blockwatch.Event{blockEvent}
	time.Sleep(2 * time.Millisecond)
	_, ok := sw.senders[stubSender]
	assert.False(ok)

	// if block removed, make RPC call
	blockEvent.Type = blockwatch.Removed
	sw.senders[stubSender] = &pm.SenderInfo{}
	watcher.sink <- []*blockwatch.Event{blockEvent}
	time.Sleep(2 * time.Millisecond)
	info, ok := sw.senders[stubSender]
	assert.True(ok)
	assert.Equal(info.Deposit, startDeposit)
	assert.Equal(info.Reserve.FundsRemaining, startReserve)

	// If map entry exists parse event log
	blockEvent.Type = blockwatch.Added
	expectedReserve := new(big.Int).Add(startReserve, new(big.Int).SetBytes(newReserveEvent.Data))
	watcher.sink <- []*blockwatch.Event{blockEvent}
	time.Sleep(2 * time.Millisecond)
	info, err = sw.GetSenderInfo(stubSender)
	assert.Nil(err)
	assert.Zero(info.Reserve.FundsRemaining.Cmp(expectedReserve))

	// If we don't care about the address, don't handle the event
	s := pm.RandAddress()
	sender := ethcommon.LeftPadBytes(s.Bytes(), 32)
	var senderTopic ethcommon.Hash
	copy(senderTopic[:], sender[:])
	newReserveEvent.Topics[1] = senderTopic
	watcher.sink <- []*blockwatch.Event{blockEvent}
	time.Sleep(2 * time.Millisecond)
	_, ok = sw.senders[s]
	assert.False(ok)
}

func TestWithdrawalEvent(t *testing.T) {
	assert := assert.New(t)
	startDeposit := big.NewInt(10)
	startReserve := big.NewInt(5)
	lpEth := &eth.StubClient{
		SenderInfo: &pm.SenderInfo{
			Deposit: big.NewInt(10),
			Reserve: &pm.ReserveInfo{
				FundsRemaining:        big.NewInt(5),
				ClaimedInCurrentRound: big.NewInt(0),
			},
		},
	}
	watcher := &stubBlockWatcher{}

	tw := &stubTimeWatcher{}

	sw, err := NewSenderWatcher(stubTicketBrokerAddr, watcher, lpEth, tw)
	assert.Nil(err)

	header := defaultMiniHeader()
	newWithdrawalEvent := newStubWithdrawalLog()
	header.Logs = append(header.Logs, newWithdrawalEvent)

	blockEvent := &blockwatch.Event{
		Type:        blockwatch.Added,
		BlockHeader: header,
	}

	go sw.Watch()
	defer sw.Stop()
	time.Sleep(2 * time.Millisecond)

	// If map entry is nil don't handle event
	watcher.sink <- []*blockwatch.Event{blockEvent}
	time.Sleep(2 * time.Millisecond)
	_, ok := sw.senders[stubSender]
	assert.False(ok)

	// if block removed, make RPC call
	blockEvent.Type = blockwatch.Removed
	sw.senders[stubSender] = &pm.SenderInfo{}
	watcher.sink <- []*blockwatch.Event{blockEvent}
	time.Sleep(2 * time.Millisecond)
	info, ok := sw.senders[stubSender]
	assert.True(ok)
	assert.Zero(info.Deposit.Cmp(startDeposit))
	assert.Zero(info.Reserve.FundsRemaining.Cmp(startReserve))

	// If map entry exists parse event log
	blockEvent.Type = blockwatch.Added
	time.Sleep(2 * time.Millisecond)
	watcher.sink <- []*blockwatch.Event{blockEvent}
	time.Sleep(2 * time.Millisecond)
	info, err = sw.GetSenderInfo(stubSender)
	assert.Nil(err)
	assert.Zero(info.Deposit.Cmp(big.NewInt(0)))
	assert.Zero(info.Reserve.FundsRemaining.Cmp(big.NewInt(0)))

	// If we don't care about the address, don't handle the event
	s := pm.RandAddress()
	sender := ethcommon.LeftPadBytes(s.Bytes(), 32)
	var senderTopic ethcommon.Hash
	copy(senderTopic[:], sender[:])
	newWithdrawalEvent.Topics[1] = senderTopic
	watcher.sink <- []*blockwatch.Event{blockEvent}
	time.Sleep(2 * time.Millisecond)
	_, ok = sw.senders[s]
	assert.False(ok)
}

func TestWinningTicketTransferEvent(t *testing.T) {
	assert := assert.New(t)
	deposit, _ := new(big.Int).SetString("10000000000000000000", 10)
	reserve, _ := new(big.Int).SetString("5000000000000000000", 10)
	startDeposit, _ := new(big.Int).SetString("10000000000000000000", 10)
	startReserve, _ := new(big.Int).SetString("5000000000000000000", 10)
	faceValue := big.NewInt(200000000000)

	senderInfo := &pm.SenderInfo{
		Deposit: deposit,
		Reserve: &pm.ReserveInfo{
			FundsRemaining:        reserve,
			ClaimedInCurrentRound: big.NewInt(0),
		},
	}
	lpEth := &eth.StubClient{
		SenderInfo:        senderInfo,
		TranscoderAddress: stubClaimant,
	}
	watcher := &stubBlockWatcher{}

	tw := &stubTimeWatcher{}

	sw, err := NewSenderWatcher(stubTicketBrokerAddr, watcher, lpEth, tw)
	assert.Nil(err)

	header := defaultMiniHeader()
	newWinningTicketEvent := newStubWinningTicketLog()
	header.Logs = append(header.Logs, newWinningTicketEvent)

	blockEvent := &blockwatch.Event{
		Type:        blockwatch.Added,
		BlockHeader: header,
	}

	go sw.Watch()
	defer sw.Stop()
	time.Sleep(2 * time.Millisecond)

	// If map entry is nil don't handle event
	watcher.sink <- []*blockwatch.Event{blockEvent}
	time.Sleep(2 * time.Millisecond)
	info, ok := sw.senders[stubSender]
	assert.False(ok)

	// if block removed, make RPC call
	blockEvent.Type = blockwatch.Removed
	sw.senders[stubSender] = &pm.SenderInfo{}
	watcher.sink <- []*blockwatch.Event{blockEvent}
	time.Sleep(2 * time.Millisecond)
	info, ok = sw.senders[stubSender]
	assert.True(ok)
	assert.Equal(info.Deposit, startDeposit)
	assert.Equal(info.Reserve.FundsRemaining, startReserve)

	// If map entry exists parse event log
	// ticket amount is 200 gwei
	// init the map entry
	blockEvent.Type = blockwatch.Added
	watcher.sink <- []*blockwatch.Event{blockEvent}
	time.Sleep(2 * time.Millisecond)
	info, err = sw.GetSenderInfo(stubSender)
	assert.Nil(err)
	assert.Zero(startDeposit.Sub(startDeposit, faceValue).Cmp(info.Deposit))
	assert.Zero(startReserve.Cmp(info.Reserve.FundsRemaining))

	// Test facevalue > deposit
	senderInfo.Deposit = big.NewInt(100000000000)
	info, err = sw.GetSenderInfo(stubSender)
	watcher.sink <- []*blockwatch.Event{blockEvent}
	time.Sleep(2 * time.Millisecond)
	info, err = sw.GetSenderInfo(stubSender)
	claimed, err := sw.ClaimedReserve(stubSender, stubClaimant)
	assert.Nil(err)
	assert.Zero(info.Deposit.Int64())
	diff := new(big.Int).Sub(faceValue, big.NewInt(100000000000))
	assert.Equal(claimed, diff)
	assert.Equal(diff, info.Reserve.ClaimedInCurrentRound)

	// If we don't care about the address, don't handle the event
	s := pm.RandAddress()
	sender := ethcommon.LeftPadBytes(s.Bytes(), 32)
	var senderTopic ethcommon.Hash
	copy(senderTopic[:], sender[:])
	newWinningTicketEvent.Topics[1] = senderTopic
	watcher.sink <- []*blockwatch.Event{blockEvent}
	time.Sleep(2 * time.Millisecond)
	_, ok = sw.senders[s]
	assert.False(ok)

	// If ticket recipient is not stubClaimant, don't alter ClaimedReserve for stubClaimant
	beforeClaimed, err := sw.ClaimedReserve(stubSender, stubClaimant)
	randRecipient := pm.RandAddress()
	var randRecipientTopic [32]byte
	copy(randRecipientTopic[:], common.LeftPadBytes(randRecipient.Bytes(), 32)[:])
	blockEvent.BlockHeader.Logs[1].Topics[2] = randRecipientTopic
	watcher.sink <- []*blockwatch.Event{blockEvent}
	time.Sleep(2 * time.Millisecond)
	afterClaimed, err := sw.ClaimedReserve(stubSender, stubClaimant)
	assert.Nil(err)
	assert.Equal(beforeClaimed, afterClaimed)

}

func TestUnlockEvent(t *testing.T) {
	assert := assert.New(t)
	startWithdrawRound := big.NewInt(5)
	lpEth := &eth.StubClient{
		SenderInfo: &pm.SenderInfo{
			WithdrawRound: big.NewInt(5),
		},
	}
	watcher := &stubBlockWatcher{}

	tw := &stubTimeWatcher{}

	sw, err := NewSenderWatcher(stubTicketBrokerAddr, watcher, lpEth, tw)
	assert.Nil(err)

	header := defaultMiniHeader()
	newUnlockEvent := newStubUnlockLog()
	header.Logs = append(header.Logs, newUnlockEvent)

	blockEvent := &blockwatch.Event{
		Type:        blockwatch.Added,
		BlockHeader: header,
	}

	go sw.Watch()
	defer sw.Stop()
	time.Sleep(2 * time.Millisecond)

	// If map entry is nil don't handle event
	watcher.sink <- []*blockwatch.Event{blockEvent}
	time.Sleep(2 * time.Millisecond)
	_, ok := sw.senders[stubSender]
	assert.False(ok)

	// if block removed, make RPC call
	blockEvent.Type = blockwatch.Removed
	sw.senders[stubSender] = &pm.SenderInfo{}
	watcher.sink <- []*blockwatch.Event{blockEvent}
	time.Sleep(2 * time.Millisecond)
	info, ok := sw.senders[stubSender]
	assert.True(ok)
	assert.Zero(info.WithdrawRound.Cmp(startWithdrawRound))

	// If map entry exists parse event log
	blockEvent.Type = blockwatch.Added
	watcher.sink <- []*blockwatch.Event{blockEvent}
	time.Sleep(2 * time.Millisecond)
	info, err = sw.GetSenderInfo(stubSender)
	assert.Nil(err)
	assert.Zero(info.WithdrawRound.Cmp(big.NewInt(150)))

	// If we don't care about the address, don't handle the event
	s := pm.RandAddress()
	sender := ethcommon.LeftPadBytes(s.Bytes(), 32)
	var senderTopic ethcommon.Hash
	copy(senderTopic[:], sender[:])
	newUnlockEvent.Topics[1] = senderTopic
	watcher.sink <- []*blockwatch.Event{blockEvent}
	time.Sleep(2 * time.Millisecond)
	_, ok = sw.senders[s]
	assert.False(ok)
}

func TestUnlockCancelledEvent(t *testing.T) {
	assert := assert.New(t)
	lpEth := &eth.StubClient{
		SenderInfo: &pm.SenderInfo{
			WithdrawRound: big.NewInt(10),
		},
	}
	watcher := &stubBlockWatcher{}

	tw := &stubTimeWatcher{}

	sw, err := NewSenderWatcher(stubTicketBrokerAddr, watcher, lpEth, tw)
	assert.Nil(err)

	header := defaultMiniHeader()
	newUnlockCancelledEvent := newStubUnlockCancelledLog()
	header.Logs = append(header.Logs, newUnlockCancelledEvent)

	blockEvent := &blockwatch.Event{
		Type:        blockwatch.Added,
		BlockHeader: header,
	}

	go sw.Watch()
	defer sw.Stop()
	time.Sleep(2 * time.Millisecond)

	// If map entry is nil don't handle event
	watcher.sink <- []*blockwatch.Event{blockEvent}
	time.Sleep(2 * time.Millisecond)
	_, ok := sw.senders[stubSender]
	assert.False(ok)

	// if block removed, make RPC call
	blockEvent.Type = blockwatch.Removed
	sw.senders[stubSender] = &pm.SenderInfo{}
	watcher.sink <- []*blockwatch.Event{blockEvent}
	time.Sleep(2 * time.Millisecond)
	info, ok := sw.senders[stubSender]
	assert.True(ok)
	assert.Zero(info.WithdrawRound.Cmp(big.NewInt(10)))

	// If map entry exists parse event log
	blockEvent.Type = blockwatch.Added
	watcher.sink <- []*blockwatch.Event{blockEvent}
	time.Sleep(2 * time.Millisecond)
	info, err = sw.GetSenderInfo(stubSender)
	assert.Nil(err)
	assert.Zero(info.WithdrawRound.Int64())

	// If we don't care about the address, don't handle the event
	s := pm.RandAddress()
	sender := ethcommon.LeftPadBytes(s.Bytes(), 32)
	var senderTopic ethcommon.Hash
	copy(senderTopic[:], sender[:])
	newUnlockCancelledEvent.Topics[1] = senderTopic
	watcher.sink <- []*blockwatch.Event{blockEvent}
	time.Sleep(2 * time.Millisecond)
	_, ok = sw.senders[s]
	assert.False(ok)
}

// Test RoundsWatcher Subscription log added and log removed
func TestNewRoundEvent_LogAdded(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	lpEth := &eth.StubClient{
		SenderInfo: &pm.SenderInfo{
			Reserve: &pm.ReserveInfo{
				ClaimedInCurrentRound: big.NewInt(1000),
			},
		},
		ClaimedAmount: big.NewInt(1000),
	}

	watcher := &stubBlockWatcher{}

	tw := &stubTimeWatcher{}

	sw, err := NewSenderWatcher(stubTicketBrokerAddr, watcher, lpEth, tw)
	assert.Nil(err)

	// set initial values
	info, err := sw.GetSenderInfo(stubSender)
	require.Nil(err)
	claimed, err := sw.ClaimedReserve(stubSender, stubClaimant)
	require.Nil(err)

	newRoundEvent := newStubNewRoundLog()

	go sw.Watch()
	defer sw.Stop()
	time.Sleep(2 * time.Millisecond)

	tw.sink <- newRoundEvent

	time.Sleep(2 * time.Millisecond)

	info, err = sw.GetSenderInfo(stubSender)
	assert.Nil(err)
	assert.Zero(info.Reserve.ClaimedInCurrentRound.Int64())
	claimed, err = sw.ClaimedReserve(stubSender, stubClaimant)
	assert.Nil(err)
	assert.Zero(claimed.Int64())
}

func TestNewRoundEvent_LogRemoved(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	lpEth := &eth.StubClient{
		SenderInfo: &pm.SenderInfo{
			Reserve: &pm.ReserveInfo{
				ClaimedInCurrentRound: big.NewInt(1000),
			},
		},
		ClaimedAmount: big.NewInt(1000),
	}

	watcher := &stubBlockWatcher{}

	tw := &stubTimeWatcher{}

	sw, err := NewSenderWatcher(stubTicketBrokerAddr, watcher, lpEth, tw)
	assert.Nil(err)

	newRoundEvent := newStubNewRoundLog()

	// set initial values
	info, err := sw.GetSenderInfo(stubSender)
	require.Nil(err)
	claimed, err := sw.ClaimedReserve(stubSender, stubClaimant)
	require.Nil(err)

	// change stub RPC call values
	expectedClaimedInCurrentRound := big.NewInt(500)
	expectedClaimedAmount := big.NewInt(2000)
	lpEth.SenderInfo.Reserve.ClaimedInCurrentRound = expectedClaimedInCurrentRound
	lpEth.ClaimedAmount = expectedClaimedAmount

	newRoundEvent.Removed = true

	go sw.Watch()
	defer sw.Stop()
	time.Sleep(2 * time.Millisecond)

	tw.sink <- newRoundEvent
	time.Sleep(2 * time.Millisecond)

	info, err = sw.GetSenderInfo(stubSender)
	assert.Nil(err)
	assert.Equal(info.Reserve.ClaimedInCurrentRound, expectedClaimedInCurrentRound)
	claimed, err = sw.ClaimedReserve(stubSender, stubClaimant)
	assert.Nil(err)
	assert.Equal(claimed, expectedClaimedAmount)
}
