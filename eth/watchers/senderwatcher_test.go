package watchers

import (
	"math/big"
	"testing"
	"time"

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
				Reserve: big.NewInt(5),
			},
		},
	}

	info := &pm.SenderInfo{
		Deposit:      big.NewInt(1000000000000000000),
		Reserve:      big.NewInt(1000000000000000000),
		ReserveState: pm.NotFrozen,
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
	assert.Zero(info.Reserve.Cmp(big.NewInt(5)))
	info = sw.senders[sender]
	assert.Zero(info.Deposit.Cmp(big.NewInt(10)))
	assert.Zero(info.Reserve.Cmp(big.NewInt(5)))
}

func TestSenderWatcher_Clear(t *testing.T) {
	assert := assert.New(t)
	stubClientSenderInfo := &pm.SenderInfo{
		Deposit: big.NewInt(10),
		Reserve: big.NewInt(5),
	}
	sw := &SenderWatcher{
		senders: make(map[ethcommon.Address]*pm.SenderInfo),
		lpEth: &eth.StubClient{
			SenderInfo: stubClientSenderInfo,
		},
	}
	info := &pm.SenderInfo{
		Deposit:      big.NewInt(1000000000000000000),
		Reserve:      big.NewInt(1000000000000000000),
		ReserveState: pm.NotFrozen,
	}
	sender := pm.RandAddress()

	sw.setSenderInfo(sender, info)

	getInfo, err := sw.GetSenderInfo(sender)
	assert.Nil(err)
	assert.Equal(info, getInfo)

	sw.Clear(sender)

	getInfo = sw.senders[sender]
	assert.Nil(getInfo)
	// on calling GetSenderInfo should be back to RPC stub values
	getInfo, err = sw.GetSenderInfo(sender)
	assert.Nil(err)
	assert.Equal(getInfo, stubClientSenderInfo)
}

func TestSenderWatcher_WatchAndStop(t *testing.T) {
	assert := assert.New(t)
	lpEth := &eth.StubClient{}
	watcher := &stubBlockWatcher{}
	sw, err := NewSenderWatcher(stubTicketBrokerAddr, watcher, lpEth)
	assert.Nil(err)

	go sw.Watch()
	time.Sleep(2 * time.Millisecond)
	sw.Stop()
	time.Sleep(2 * time.Millisecond)
	assert.True(watcher.sub.unsubscribed)
}

func TestSenderWatcher_HandleLog(t *testing.T) {
	lpEth := &eth.StubClient{}
	watcher := &stubBlockWatcher{}
	rw, err := NewSenderWatcher(stubTicketBrokerAddr, watcher, lpEth)
	require.Nil(t, err)

	assert := assert.New(t)

	// Test unknown event
	log := newStubBaseLog()
	log.Topics = []ethcommon.Hash{ethcommon.BytesToHash([]byte("foo"))}

	err = rw.handleLog(log)
	assert.Nil(err)
}

func TestFundDepositEvent(t *testing.T) {
	assert := assert.New(t)
	startDeposit := big.NewInt(10)
	lpEth := &eth.StubClient{
		SenderInfo: &pm.SenderInfo{
			Deposit: big.NewInt(10),
			Reserve: big.NewInt(5),
		},
	}
	watcher := &stubBlockWatcher{}
	sw, err := NewSenderWatcher(stubTicketBrokerAddr, watcher, lpEth)
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
	startThawR := big.NewInt(1)
	lpEth := &eth.StubClient{
		SenderInfo: &pm.SenderInfo{
			Deposit:   big.NewInt(10),
			Reserve:   big.NewInt(5),
			ThawRound: big.NewInt(1),
		},
	}
	watcher := &stubBlockWatcher{}
	sw, err := NewSenderWatcher(stubTicketBrokerAddr, watcher, lpEth)
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
	assert.Equal(info.Reserve, startReserve)
	assert.Equal(info.ThawRound, startThawR)

	// If map entry exists parse event log
	blockEvent.Type = blockwatch.Added
	expectedReserve := new(big.Int).Add(startReserve, new(big.Int).SetBytes(newReserveEvent.Data))
	watcher.sink <- []*blockwatch.Event{blockEvent}
	time.Sleep(2 * time.Millisecond)
	info, err = sw.GetSenderInfo(stubSender)
	assert.Nil(err)
	assert.Zero(info.Reserve.Cmp(expectedReserve))

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
			Reserve: big.NewInt(5),
		},
	}
	watcher := &stubBlockWatcher{}
	sw, err := NewSenderWatcher(stubTicketBrokerAddr, watcher, lpEth)
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
	assert.Zero(info.Reserve.Cmp(startReserve))

	// If map entry exists parse event log
	blockEvent.Type = blockwatch.Added
	time.Sleep(2 * time.Millisecond)
	watcher.sink <- []*blockwatch.Event{blockEvent}
	time.Sleep(2 * time.Millisecond)
	info, err = sw.GetSenderInfo(stubSender)
	assert.Nil(err)
	assert.Zero(info.Deposit.Cmp(big.NewInt(0)))
	assert.Zero(info.Reserve.Cmp(big.NewInt(0)))

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
	require := require.New(t)
	deposit, _ := new(big.Int).SetString("10000000000000000000", 10)
	reserve, _ := new(big.Int).SetString("5000000000000000000", 10)
	startDeposit, _ := new(big.Int).SetString("10000000000000000000", 10)
	startReserve, _ := new(big.Int).SetString("5000000000000000000", 10)
	senderInfo := &pm.SenderInfo{
		Deposit: deposit,
		Reserve: reserve,
	}
	lpEth := &eth.StubClient{
		SenderInfo: senderInfo,
	}
	watcher := &stubBlockWatcher{}
	sw, err := NewSenderWatcher(stubTicketBrokerAddr, watcher, lpEth)
	require.Nil(err)

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
	assert.Equal(info.Reserve, startReserve)

	// If map entry exists parse event log
	// ticket amount is 200 gwei
	// init the map entry
	blockEvent.Type = blockwatch.Added
	faceValue := big.NewInt(200000000000)
	watcher.sink <- []*blockwatch.Event{blockEvent}
	time.Sleep(2 * time.Millisecond)
	info, err = sw.GetSenderInfo(stubSender)
	assert.Nil(err)
	assert.Zero(startDeposit.Sub(startDeposit, faceValue).Cmp(info.Deposit))
	assert.Zero(startReserve.Cmp(info.Reserve))

	// Test facevalue > deposit
	senderInfo.Deposit = big.NewInt(100000000000)
	info, err = sw.GetSenderInfo(stubSender)
	watcher.sink <- []*blockwatch.Event{blockEvent}
	time.Sleep(2 * time.Millisecond)
	info, err = sw.GetSenderInfo(stubSender)
	assert.Nil(err)
	assert.Zero(info.Deposit.Int64())
	diff := new(big.Int).Sub(faceValue, big.NewInt(100000000000))
	expectedReserve := new(big.Int).Sub(startReserve, diff)
	assert.Zero(expectedReserve.Cmp(info.Reserve))

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
}

func TestReserveFrozenEvent(t *testing.T) {
	assert := assert.New(t)
	startThawR := big.NewInt(10)
	lpEth := &eth.StubClient{
		SenderInfo: &pm.SenderInfo{
			ThawRound:    big.NewInt(10),
			ReserveState: pm.Frozen,
		},
	}
	watcher := &stubBlockWatcher{}
	sw, err := NewSenderWatcher(stubTicketBrokerAddr, watcher, lpEth)
	assert.Nil(err)

	header := defaultMiniHeader()
	newReserveFrozenEvent := newStubReserveFrozenLog()
	header.Logs = append(header.Logs, newReserveFrozenEvent)

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
	assert.Equal(info.ReserveState, pm.Frozen)
	assert.Zero(startThawR.Cmp(info.ThawRound))

	// If map entry exists parse event log
	blockEvent.Type = blockwatch.Added
	watcher.sink <- []*blockwatch.Event{blockEvent}
	time.Sleep(2 * time.Millisecond)
	info, err = sw.GetSenderInfo(stubSender)
	assert.Nil(err)
	assert.Equal(info.ReserveState, pm.Frozen)
	expectedThawRound := new(big.Int).Add(new(big.Int).SetBytes(newReserveFrozenEvent.Data[:32]), big.NewInt(2))
	assert.Zero(expectedThawRound.Cmp(info.ThawRound))

	// If we don't care about the address, don't handle the event
	s := pm.RandAddress()
	sender := ethcommon.LeftPadBytes(s.Bytes(), 32)
	var senderTopic ethcommon.Hash
	copy(senderTopic[:], sender[:])
	newReserveFrozenEvent.Topics[1] = senderTopic
	watcher.sink <- []*blockwatch.Event{blockEvent}
	time.Sleep(2 * time.Millisecond)
	_, ok = sw.senders[s]
	assert.False(ok)
}

func TestUnlockEvent(t *testing.T) {
	assert := assert.New(t)
	startWithdrawBlock := big.NewInt(5)
	lpEth := &eth.StubClient{
		SenderInfo: &pm.SenderInfo{
			WithdrawBlock: big.NewInt(5),
		},
	}
	watcher := &stubBlockWatcher{}
	sw, err := NewSenderWatcher(stubTicketBrokerAddr, watcher, lpEth)
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
	assert.Zero(info.WithdrawBlock.Cmp(startWithdrawBlock))

	// If map entry exists parse event log
	blockEvent.Type = blockwatch.Added
	watcher.sink <- []*blockwatch.Event{blockEvent}
	time.Sleep(2 * time.Millisecond)
	info, err = sw.GetSenderInfo(stubSender)
	assert.Nil(err)
	assert.Zero(info.WithdrawBlock.Cmp(big.NewInt(150)))

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
			WithdrawBlock: big.NewInt(10),
		},
	}
	watcher := &stubBlockWatcher{}
	sw, err := NewSenderWatcher(stubTicketBrokerAddr, watcher, lpEth)
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
	assert.Zero(info.WithdrawBlock.Cmp(big.NewInt(10)))

	// If map entry exists parse event log
	blockEvent.Type = blockwatch.Added
	watcher.sink <- []*blockwatch.Event{blockEvent}
	time.Sleep(2 * time.Millisecond)
	info, err = sw.GetSenderInfo(stubSender)
	assert.Nil(err)
	assert.Zero(info.WithdrawBlock.Int64())

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
