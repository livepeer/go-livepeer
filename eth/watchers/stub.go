package watchers

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/event"
	"github.com/livepeer/go-livepeer/eth/blockwatch"
	"github.com/livepeer/go-livepeer/pm"
)

var stubSender = pm.RandAddress()

var stubBondingManagerAddr = common.HexToAddress("0x511bc4556d823ae99630ae8de28b9b80df90ea2e")
var stubRoundsManagerAddr = common.HexToAddress("0xc1F9BB72216E5ecDc97e248F65E14df1fE46600a")

var stubTicketBrokerAddr = common.HexToAddress("0x9d6d492bD500DA5B33cf95A5d610a73360FcaAa0")

func newStubBaseLog() types.Log {
	return types.Log{
		BlockNumber: uint64(30),
		TxHash:      common.HexToHash("0xd9bb5f9e888ee6f74bedcda811c2461230f247c205849d6f83cb6c3925e54586"),
		TxIndex:     uint(0),
		BlockHash:   common.HexToHash("0x6bbf9b6e836207ab25379c20e517a89090cbbaf8877746f6ed7fb6820770816b"),
		Index:       uint(0),
		Removed:     false,
	}
}

func newStubUnbondLog() types.Log {
	log := newStubBaseLog()
	log.Address = stubBondingManagerAddr
	log.Topics = []common.Hash{
		common.HexToHash("0x2d5d98d189bee5496a08db2a5948cb7e5e786f09d17d0c3f228eb41776c24a06"),
		// delegate = 0x525419FF5707190389bfb5C87c375D710F5fCb0E
		common.HexToHash("0x000000000000000000000000525419ff5707190389bfb5c87c375d710f5fcb0e"),
		// delegator = 0xF75b78571F6563e8Acf1899F682Fb10A9248CCE8
		common.HexToHash("0x000000000000000000000000f75b78571f6563e8acf1899f682fb10a9248cce8"),
	}
	// unbondingLockId = 1
	// amount = 11111000000000000000
	// withdrawRound = 1457
	log.Data = common.Hex2Bytes("00000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000009a3233a1a35d800000000000000000000000000000000000000000000000000000000000000005b1")

	return log
}

func newStubRebondLog() types.Log {
	log := newStubBaseLog()
	log.Address = stubBondingManagerAddr
	log.Topics = []common.Hash{
		common.HexToHash("0x9f5b64cc71e1e26ff178caaa7877a04d8ce66fde989251870e80e6fbee690c17"),
		// delegate = 0x525419FF5707190389bfb5C87c375D710F5fCb0E
		common.HexToHash("0x000000000000000000000000525419ff5707190389bfb5c87c375d710f5fcb0e"),
		// delegator = 0xF75b78571F6563e8Acf1899F682Fb10A9248CCE8
		common.HexToHash("0x000000000000000000000000f75b78571f6563e8acf1899f682fb10a9248cce8"),
	}
	// unbondingLockId = 1
	// amount = 57000000000000000000
	log.Data = common.Hex2Bytes("00000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000031708ae0045440000")

	return log
}

func newStubWithdrawStakeLog() types.Log {
	log := newStubBaseLog()
	log.Address = stubBondingManagerAddr
	log.Topics = []common.Hash{
		common.HexToHash("0x1340f1a8f3d456a649e1a12071dfa15655e3d09252131d0f980c3b405cc8dd2e"),
		// delegator = 0xF75b78571F6563e8Acf1899F682Fb10A9248CCE8
		common.HexToHash("0x000000000000000000000000f75b78571f6563e8acf1899f682fb10a9248cce8"),
	}
	// unbondingLockId = 1
	// amount = 7343158980137288000
	// withdrawRound = 1450
	log.Data = common.Hex2Bytes("000000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000000065e8242fcc4d81b100000000000000000000000000000000000000000000000000000000000005aa")

	return log
}

func newStubNewRoundLog() types.Log {
	log := newStubBaseLog()
	log.Address = stubRoundsManagerAddr
	topic := crypto.Keccak256Hash([]byte("NewRound(uint256,bytes32)"))
	round := big.NewInt(8)
	roundBytes := common.BytesToHash(common.LeftPadBytes(round.Bytes(), 32))
	log.Topics = []common.Hash{topic, roundBytes}
	log.Data, _ = hexutil.Decode("0x15063b24c3dfd390370cd13eaf27fd0b079c60f31bf1414c574f865e906a8964")
	return log
}

func newStubDepositFundedLog() types.Log {
	log := newStubBaseLog()
	log.Address = stubTicketBrokerAddr
	amount, _ := new(big.Int).SetString("5000000000000000000", 10)
	amountData := common.LeftPadBytes(amount.Bytes(), 32)
	sender := common.LeftPadBytes(stubSender.Bytes(), 32)
	var senderTopic common.Hash
	copy(senderTopic[:], sender[:])
	log.Topics = []common.Hash{
		crypto.Keccak256Hash([]byte("DepositFunded(address,uint256)")),
		senderTopic,
	}
	log.Data = amountData
	return log
}

func newStubReserveFundedLog() types.Log {
	log := newStubBaseLog()
	log.Address = stubTicketBrokerAddr
	amount, _ := new(big.Int).SetString("5000000000000000000", 10)
	amountData := common.LeftPadBytes(amount.Bytes(), 32)
	sender := common.LeftPadBytes(stubSender.Bytes(), 32)
	var senderTopic common.Hash
	copy(senderTopic[:], sender[:])
	log.Topics = []common.Hash{
		crypto.Keccak256Hash([]byte("ReserveFunded(address,uint256)")),
		senderTopic,
	}
	log.Data = amountData
	return log
}

func newStubWithdrawalLog() types.Log {
	log := newStubBaseLog()
	log.Address = stubTicketBrokerAddr
	sender := common.LeftPadBytes(stubSender.Bytes(), 32)
	var senderTopic common.Hash
	copy(senderTopic[:], sender[:])
	log.Topics = []common.Hash{
		crypto.Keccak256Hash([]byte("Withdrawal(address,uint256,uint256)")),
		senderTopic,
	}

	deposit, _ := new(big.Int).SetString("5000000000000000000", 10)
	reserve, _ := new(big.Int).SetString("1000000000000000000", 10)
	depositData := common.LeftPadBytes(deposit.Bytes(), 32)
	reserveData := common.LeftPadBytes(reserve.Bytes(), 32)
	var data []byte
	data = append(data, depositData...)
	data = append(data, reserveData...)
	log.Data = data
	return log
}

func newStubWinningTicketLog() types.Log {
	log := newStubBaseLog()
	log.Address = stubTicketBrokerAddr
	sender := common.LeftPadBytes(stubSender.Bytes(), 32)
	var senderTopic [32]byte
	copy(senderTopic[:], sender[:])
	recipient := common.LeftPadBytes(pm.RandAddress().Bytes(), 32)
	var recipientTopic [32]byte
	copy(recipientTopic[:], recipient[:])
	log.Topics = []common.Hash{
		crypto.Keccak256Hash([]byte("WinningTicketTransfer(address,address,uint256)")),
		senderTopic,
		recipientTopic,
	}
	amount, _ := new(big.Int).SetString("200000000000", 10)
	amountData := common.LeftPadBytes(amount.Bytes(), 32)
	log.Data = amountData
	return log
}

func newStubReserveFrozenLog() types.Log {
	log := newStubBaseLog()
	log.Address = stubTicketBrokerAddr
	holder := common.LeftPadBytes(stubSender.Bytes(), 32)
	claimant := common.LeftPadBytes(pm.RandAddress().Bytes(), 32)
	var holderTopic [32]byte
	copy(holderTopic[:], holder[:])
	var claimantTopic [32]byte
	copy(claimantTopic[:], claimant[:])
	log.Topics = []common.Hash{
		crypto.Keccak256Hash([]byte("ReserveFrozen(address,address,uint256,uint256)")),
		holderTopic,
		claimantTopic,
	}
	var data []byte
	data = append(data, common.LeftPadBytes(big.NewInt(5).Bytes(), 32)...)
	data = append(data, common.LeftPadBytes(big.NewInt(25).Bytes(), 32)...)
	log.Data = data
	return log
}

func newStubUnlockLog() types.Log {
	log := newStubBaseLog()
	log.Address = stubTicketBrokerAddr
	sender := common.LeftPadBytes(stubSender.Bytes(), 32)
	var senderTopic common.Hash
	copy(senderTopic[:], sender[:])
	log.Topics = []common.Hash{
		crypto.Keccak256Hash([]byte("Unlock(address,uint256,uint256)")),
		senderTopic,
	}
	var data []byte
	data = append(data, common.LeftPadBytes(big.NewInt(100).Bytes(), 32)...)
	data = append(data, common.LeftPadBytes(big.NewInt(150).Bytes(), 32)...)
	log.Data = data
	return log
}

func newStubUnlockCancelledLog() types.Log {
	log := newStubBaseLog()
	log.Address = stubTicketBrokerAddr
	sender := common.LeftPadBytes(stubSender.Bytes(), 32)
	var senderTopic common.Hash
	copy(senderTopic[:], sender[:])
	log.Topics = []common.Hash{
		crypto.Keccak256Hash([]byte("UnlockCancelled(address)")),
		senderTopic,
	}
	log.Data = []byte{}
	return log
}

type stubSubscription struct {
	errCh        <-chan error
	unsubscribed bool
}

func (s *stubSubscription) Unsubscribe() {
	s.unsubscribed = true
}

func (s *stubSubscription) Err() <-chan error {
	return s.errCh
}

type stubBlockWatcher struct {
	sink chan<- []*blockwatch.Event
	sub  *stubSubscription
}

func (bw *stubBlockWatcher) Subscribe(sink chan<- []*blockwatch.Event) event.Subscription {
	bw.sink = sink
	bw.sub = &stubSubscription{errCh: make(<-chan error)}
	return bw.sub
}

type stubUnbondingLock struct {
	Delegator     common.Address
	Amount        *big.Int
	WithdrawRound *big.Int
	UsedBlock     *big.Int
}

type stubUnbondingLockStore struct {
	unbondingLocks map[int64]*stubUnbondingLock
	insertErr      error
	deleteErr      error
	useErr         error
}

func newStubUnbondingLockStore() *stubUnbondingLockStore {
	return &stubUnbondingLockStore{
		unbondingLocks: make(map[int64]*stubUnbondingLock),
	}
}

func (s *stubUnbondingLockStore) InsertUnbondingLock(id *big.Int, delegator common.Address, amount, withdrawRound *big.Int) error {
	if s.insertErr != nil {
		return s.insertErr
	}

	s.unbondingLocks[id.Int64()] = &stubUnbondingLock{
		Delegator:     delegator,
		Amount:        amount,
		WithdrawRound: withdrawRound,
	}

	return nil
}

func (s *stubUnbondingLockStore) DeleteUnbondingLock(id *big.Int, delegator common.Address) error {
	if s.deleteErr != nil {
		return s.deleteErr
	}

	delete(s.unbondingLocks, id.Int64())

	return nil
}

func (s *stubUnbondingLockStore) UseUnbondingLock(id *big.Int, delegator common.Address, usedBlock *big.Int) error {
	if s.useErr != nil {
		return s.useErr
	}

	s.unbondingLocks[id.Int64()].UsedBlock = usedBlock

	return nil
}

func (s *stubUnbondingLockStore) Get(id int64) *stubUnbondingLock {
	return s.unbondingLocks[id]
}

func defaultMiniHeader() *blockwatch.MiniHeader {
	block := &blockwatch.MiniHeader{
		Number: big.NewInt(450),
		Parent: pm.RandHash(),
		Hash:   pm.RandHash(),
	}
	log := types.Log{
		Topics:    []common.Hash{pm.RandHash(), pm.RandHash()},
		Data:      pm.RandBytes(32),
		BlockHash: block.Hash,
	}
	block.Logs = []types.Log{log}
	return block
}
