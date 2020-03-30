package watchers

import (
	"math/big"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/event"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/eth/blockwatch"
	"github.com/livepeer/go-livepeer/pm"
)

var stubSender = pm.RandAddress()
var stubClaimant = pm.RandAddress()
var stubTranscoder = pm.RandAddress()

var stubActivationRound = big.NewInt(99)
var stubDeactivationRound = big.NewInt(77)
var stubUpdatedServiceURI = "http://127.0.0.1:9999"

var stubBondingManagerAddr = ethcommon.HexToAddress("0x511bc4556d823ae99630ae8de28b9b80df90ea2e")
var stubRoundsManagerAddr = ethcommon.HexToAddress("0xc1F9BB72216E5ecDc97e248F65E14df1fE46600a")
var stubTicketBrokerAddr = ethcommon.HexToAddress("0x9d6d492bD500DA5B33cf95A5d610a73360FcaAa0")
var stubServiceRegistryAddr = ethcommon.HexToAddress("0xF55C0B3c82be82EEBa1557d3Db1bdb55FDF46a4b")

func newStubBaseLog() types.Log {
	return types.Log{
		BlockNumber: uint64(30),
		TxHash:      ethcommon.HexToHash("0xd9bb5f9e888ee6f74bedcda811c2461230f247c205849d6f83cb6c3925e54586"),
		TxIndex:     uint(0),
		BlockHash:   ethcommon.HexToHash("0x6bbf9b6e836207ab25379c20e517a89090cbbaf8877746f6ed7fb6820770816b"),
		Index:       uint(0),
		Removed:     false,
	}
}

func newStubUnbondLog() types.Log {
	log := newStubBaseLog()
	log.Address = stubBondingManagerAddr
	log.Topics = []ethcommon.Hash{
		ethcommon.HexToHash("0x2d5d98d189bee5496a08db2a5948cb7e5e786f09d17d0c3f228eb41776c24a06"),
		// delegate = 0x525419FF5707190389bfb5C87c375D710F5fCb0E
		ethcommon.HexToHash("0x000000000000000000000000525419ff5707190389bfb5c87c375d710f5fcb0e"),
		// delegator = 0xF75b78571F6563e8Acf1899F682Fb10A9248CCE8
		ethcommon.HexToHash("0x000000000000000000000000f75b78571f6563e8acf1899f682fb10a9248cce8"),
	}
	// unbondingLockId = 1
	// amount = 11111000000000000000
	// withdrawRound = 1457
	log.Data = ethcommon.Hex2Bytes("00000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000009a3233a1a35d800000000000000000000000000000000000000000000000000000000000000005b1")

	return log
}

func newStubRebondLog() types.Log {
	log := newStubBaseLog()
	log.Address = stubBondingManagerAddr
	log.Topics = []ethcommon.Hash{
		ethcommon.HexToHash("0x9f5b64cc71e1e26ff178caaa7877a04d8ce66fde989251870e80e6fbee690c17"),
		// delegate = 0x525419FF5707190389bfb5C87c375D710F5fCb0E
		ethcommon.HexToHash("0x000000000000000000000000525419ff5707190389bfb5c87c375d710f5fcb0e"),
		// delegator = 0xF75b78571F6563e8Acf1899F682Fb10A9248CCE8
		ethcommon.HexToHash("0x000000000000000000000000f75b78571f6563e8acf1899f682fb10a9248cce8"),
	}
	// unbondingLockId = 1
	// amount = 57000000000000000000
	log.Data = ethcommon.Hex2Bytes("00000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000031708ae0045440000")

	return log
}

func newStubWithdrawStakeLog() types.Log {
	log := newStubBaseLog()
	log.Address = stubBondingManagerAddr
	log.Topics = []ethcommon.Hash{
		ethcommon.HexToHash("0x1340f1a8f3d456a649e1a12071dfa15655e3d09252131d0f980c3b405cc8dd2e"),
		// delegator = 0xF75b78571F6563e8Acf1899F682Fb10A9248CCE8
		ethcommon.HexToHash("0x000000000000000000000000f75b78571f6563e8acf1899f682fb10a9248cce8"),
	}
	// unbondingLockId = 1
	// amount = 7343158980137288000
	// withdrawRound = 1450
	log.Data = ethcommon.Hex2Bytes("000000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000000065e8242fcc4d81b100000000000000000000000000000000000000000000000000000000000005aa")

	return log
}

func newStubNewRoundLog() types.Log {
	log := newStubBaseLog()
	log.Address = stubRoundsManagerAddr
	topic := crypto.Keccak256Hash([]byte("NewRound(uint256,bytes32)"))
	round := big.NewInt(8)
	roundBytes := ethcommon.BytesToHash(ethcommon.LeftPadBytes(round.Bytes(), 32))
	log.Topics = []ethcommon.Hash{topic, roundBytes}
	log.Data, _ = hexutil.Decode("0x15063b24c3dfd390370cd13eaf27fd0b079c60f31bf1414c574f865e906a8964")
	return log
}

func newStubDepositFundedLog() types.Log {
	log := newStubBaseLog()
	log.Address = stubTicketBrokerAddr
	amount, _ := new(big.Int).SetString("5000000000000000000", 10)
	amountData := ethcommon.LeftPadBytes(amount.Bytes(), 32)
	sender := ethcommon.LeftPadBytes(stubSender.Bytes(), 32)
	var senderTopic ethcommon.Hash
	copy(senderTopic[:], sender[:])
	log.Topics = []ethcommon.Hash{
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
	amountData := ethcommon.LeftPadBytes(amount.Bytes(), 32)
	sender := ethcommon.LeftPadBytes(stubSender.Bytes(), 32)
	var senderTopic ethcommon.Hash
	copy(senderTopic[:], sender[:])
	log.Topics = []ethcommon.Hash{
		crypto.Keccak256Hash([]byte("ReserveFunded(address,uint256)")),
		senderTopic,
	}
	log.Data = amountData
	return log
}

func newStubWithdrawalLog() types.Log {
	log := newStubBaseLog()
	log.Address = stubTicketBrokerAddr
	sender := ethcommon.LeftPadBytes(stubSender.Bytes(), 32)
	var senderTopic ethcommon.Hash
	copy(senderTopic[:], sender[:])
	log.Topics = []ethcommon.Hash{
		crypto.Keccak256Hash([]byte("Withdrawal(address,uint256,uint256)")),
		senderTopic,
	}

	deposit, _ := new(big.Int).SetString("5000000000000000000", 10)
	reserve, _ := new(big.Int).SetString("1000000000000000000", 10)
	depositData := ethcommon.LeftPadBytes(deposit.Bytes(), 32)
	reserveData := ethcommon.LeftPadBytes(reserve.Bytes(), 32)
	var data []byte
	data = append(data, depositData...)
	data = append(data, reserveData...)
	log.Data = data
	return log
}

func newStubWinningTicketLog() types.Log {
	log := newStubBaseLog()
	log.Address = stubTicketBrokerAddr
	sender := ethcommon.LeftPadBytes(stubSender.Bytes(), 32)
	var senderTopic [32]byte
	copy(senderTopic[:], sender[:])
	recipient := ethcommon.LeftPadBytes(stubClaimant.Bytes(), 32)
	var recipientTopic [32]byte
	copy(recipientTopic[:], recipient[:])
	log.Topics = []ethcommon.Hash{
		crypto.Keccak256Hash([]byte("WinningTicketTransfer(address,address,uint256)")),
		senderTopic,
		recipientTopic,
	}
	amount, _ := new(big.Int).SetString("200000000000", 10)
	amountData := ethcommon.LeftPadBytes(amount.Bytes(), 32)
	log.Data = amountData
	return log
}

func newStubUnlockLog() types.Log {
	log := newStubBaseLog()
	log.Address = stubTicketBrokerAddr
	sender := ethcommon.LeftPadBytes(stubSender.Bytes(), 32)
	var senderTopic ethcommon.Hash
	copy(senderTopic[:], sender[:])
	log.Topics = []ethcommon.Hash{
		crypto.Keccak256Hash([]byte("Unlock(address,uint256,uint256)")),
		senderTopic,
	}
	var data []byte
	data = append(data, ethcommon.LeftPadBytes(big.NewInt(100).Bytes(), 32)...)
	data = append(data, ethcommon.LeftPadBytes(big.NewInt(150).Bytes(), 32)...)
	log.Data = data
	return log
}

func newStubUnlockCancelledLog() types.Log {
	log := newStubBaseLog()
	log.Address = stubTicketBrokerAddr
	sender := ethcommon.LeftPadBytes(stubSender.Bytes(), 32)
	var senderTopic ethcommon.Hash
	copy(senderTopic[:], sender[:])
	log.Topics = []ethcommon.Hash{
		crypto.Keccak256Hash([]byte("UnlockCancelled(address)")),
		senderTopic,
	}
	log.Data = []byte{}
	return log
}

func newStubTranscoderActivatedLog() types.Log {
	log := newStubBaseLog()
	log.Address = stubBondingManagerAddr
	transcoder := ethcommon.LeftPadBytes(stubTranscoder.Bytes(), 32)
	var transcoderTopic ethcommon.Hash
	copy(transcoderTopic[:], transcoder[:])
	log.Topics = []ethcommon.Hash{
		crypto.Keccak256Hash([]byte("TranscoderActivated(address,uint256)")),
		transcoderTopic,
	}
	log.Data = ethcommon.LeftPadBytes(stubActivationRound.Bytes(), 32)
	return log
}

func newStubTranscoderDeactivatedLog() types.Log {
	log := newStubBaseLog()
	log.Address = stubBondingManagerAddr
	transcoder := ethcommon.LeftPadBytes(stubTranscoder.Bytes(), 32)
	var transcoderTopic ethcommon.Hash
	copy(transcoderTopic[:], transcoder[:])
	log.Topics = []ethcommon.Hash{
		crypto.Keccak256Hash([]byte("TranscoderDeactivated(address,uint256)")),
		transcoderTopic,
	}
	log.Data = ethcommon.LeftPadBytes(stubDeactivationRound.Bytes(), 32)
	return log
}

func newStubServiceURIUpdateLog() types.Log {
	log := newStubBaseLog()
	log.Address = stubServiceRegistryAddr
	transcoder := ethcommon.LeftPadBytes(stubTranscoder.Bytes(), 32)
	var transcoderTopic ethcommon.Hash
	copy(transcoderTopic[:], transcoder[:])
	log.Topics = []ethcommon.Hash{
		crypto.Keccak256Hash([]byte("ServiceURIUpdate(address,string)")),
		transcoderTopic,
	}

	var data []byte
	serviceURI := []byte(stubUpdatedServiceURI)
	data = append(data, ethcommon.LeftPadBytes(big.NewInt(32).Bytes(), 32)...)
	data = append(data, ethcommon.LeftPadBytes(big.NewInt(int64(len(serviceURI))).Bytes(), 32)...)
	data = append(data, serviceURI...)
	log.Data = data
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
	sink         chan<- []*blockwatch.Event
	sub          *stubSubscription
	latestHeader *blockwatch.MiniHeader
	err          error
}

func (bw *stubBlockWatcher) Subscribe(sink chan<- []*blockwatch.Event) event.Subscription {
	bw.sink = sink
	bw.sub = &stubSubscription{errCh: make(<-chan error)}
	return bw.sub
}

func (bw *stubBlockWatcher) GetLatestBlock() (*blockwatch.MiniHeader, error) {
	return bw.latestHeader, bw.err
}

type stubUnbondingLock struct {
	Delegator     ethcommon.Address
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

func (s *stubUnbondingLockStore) InsertUnbondingLock(id *big.Int, delegator ethcommon.Address, amount, withdrawRound *big.Int) error {
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

func (s *stubUnbondingLockStore) DeleteUnbondingLock(id *big.Int, delegator ethcommon.Address) error {
	if s.deleteErr != nil {
		return s.deleteErr
	}

	delete(s.unbondingLocks, id.Int64())

	return nil
}

func (s *stubUnbondingLockStore) UseUnbondingLock(id *big.Int, delegator ethcommon.Address, usedBlock *big.Int) error {
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
		Topics:    []ethcommon.Hash{pm.RandHash(), pm.RandHash()},
		Data:      pm.RandBytes(32),
		BlockHash: block.Hash,
	}
	block.Logs = []types.Log{log}
	return block
}

type stubTimeWatcher struct {
	sink chan<- types.Log
	sub  *stubSubscription
}

func (tw *stubTimeWatcher) SubscribeRounds(sink chan<- types.Log) event.Subscription {
	tw.sink = sink
	tw.sub = &stubSubscription{errCh: make(<-chan error)}
	return tw.sub
}

type stubOrchestratorStore struct {
	activationRound   int64
	deactivationRound int64
	serviceURI        string
	ethereumAddr      string
	stake             int64
	selectErr         error
	updateErr         error
}

func (s *stubOrchestratorStore) UpdateOrch(orch *common.DBOrch) error {
	if s.updateErr != nil {
		return s.updateErr
	}
	s.activationRound = orch.ActivationRound
	s.deactivationRound = orch.DeactivationRound
	s.ethereumAddr = orch.EthereumAddr
	s.serviceURI = orch.ServiceURI
	s.stake = orch.Stake
	return nil
}

func (s *stubOrchestratorStore) OrchCount(filter *common.DBOrchFilter) (int, error) { return 0, nil }
func (s *stubOrchestratorStore) SelectOrchs(filter *common.DBOrchFilter) ([]*common.DBOrch, error) {
	if s.selectErr != nil {
		return []*common.DBOrch{}, s.selectErr
	}
	return []*common.DBOrch{
		common.NewDBOrch(s.ethereumAddr, s.serviceURI, 0, s.activationRound, s.deactivationRound, s.stake),
	}, nil
}
