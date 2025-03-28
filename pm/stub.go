package pm

import (
	"fmt"
	"math/big"
	"sync"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
	"github.com/stretchr/testify/mock"
)

type stubBlockStore struct {
	lastBlock *big.Int
	err       error
	isActive  bool
}

type stubTicketStore struct {
	stubBlockStore
	tickets          map[ethcommon.Address][]*SignedTicket
	submitted        map[string]bool
	storeShouldFail  bool
	loadShouldFail   bool
	removeShouldFail bool
	lock             sync.RWMutex
}

func newStubTicketStore() *stubTicketStore {
	return &stubTicketStore{
		tickets:   make(map[ethcommon.Address][]*SignedTicket),
		submitted: make(map[string]bool),
		stubBlockStore: stubBlockStore{
			isActive: true,
		},
	}
}

func (ts *stubTicketStore) StoreWinningTicket(ticket *SignedTicket) error {
	ts.lock.Lock()
	defer ts.lock.Unlock()

	if ts.storeShouldFail {
		return fmt.Errorf("stub TicketStore store error")
	}

	// if ticket exists don't insert it
	for _, t := range ts.tickets[ticket.Sender] {
		if fmt.Sprintf("%x", t.Sig) == fmt.Sprintf("%x", ticket.Sig) {
			return nil
		}
	}

	ts.tickets[ticket.Sender] = append(ts.tickets[ticket.Sender], ticket)
	return nil
}

func (ts *stubTicketStore) SelectEarliestWinningTicket(sender ethcommon.Address, _ int64) (*SignedTicket, error) {
	ts.lock.Lock()
	defer ts.lock.Unlock()
	if ts.loadShouldFail {
		return nil, fmt.Errorf("stub TicketStore load error")
	}
	for _, t := range ts.tickets[sender] {
		if !ts.submitted[fmt.Sprintf("%x", t.Sig)] {
			return t, nil
		}
	}
	return nil, nil
}

func (ts *stubTicketStore) MarkWinningTicketRedeemed(ticket *SignedTicket, _ ethcommon.Hash) error {
	ts.lock.Lock()
	defer ts.lock.Unlock()
	ts.submitted[fmt.Sprintf("%x", ticket.Sig)] = true
	return nil
}

func (ts *stubTicketStore) RemoveWinningTicket(ticket *SignedTicket) error {
	ts.lock.Lock()
	defer ts.lock.Unlock()
	if ts.removeShouldFail {
		return fmt.Errorf("stub TicketStore remove error")
	}
	for i, t := range ts.tickets[ticket.Sender] {
		if ethcommon.Bytes2Hex(t.Sig) != ethcommon.Bytes2Hex(ticket.Sig) {
			continue
		}
		tickets := ts.tickets[ticket.Sender][:i]
		if i != len(ts.tickets[ticket.Sender])-1 {
			tickets = append(tickets, ts.tickets[ticket.Sender][i+1:]...)
		}
		ts.tickets[ticket.Sender] = tickets
		break
	}
	return nil
}

func (ts *stubTicketStore) WinningTicketCount(sender ethcommon.Address, _ int64) (int, error) {
	ts.lock.Lock()
	defer ts.lock.Unlock()
	if ts.loadShouldFail {
		return 0, fmt.Errorf("stub TicketStore load error")
	}
	count := 0
	for _, t := range ts.tickets[sender] {
		if !ts.submitted[fmt.Sprintf("%x", t.Sig)] {
			count++
		}
	}
	return count, nil
}

func (ts *stubTicketStore) IsOrchActive(_ ethcommon.Address, _ *big.Int) (bool, error) {
	return ts.isActive, ts.err
}

func (ts *stubBlockStore) LastSeenBlock() (*big.Int, error) {
	return ts.lastBlock, ts.err
}

type stubSigVerifier struct {
	verifyResult bool
}

func (sv *stubSigVerifier) SetVerifyResult(verifyResult bool) {
	sv.verifyResult = verifyResult
}

func (sv *stubSigVerifier) Verify(_ ethcommon.Address, _, _ []byte) bool {
	return sv.verifyResult
}

type stubBroker struct {
	deposits        map[ethcommon.Address]*big.Int
	reserves        map[ethcommon.Address]*big.Int
	usedTickets     map[ethcommon.Hash]bool
	approvedSigners map[ethcommon.Address]bool
	mu              sync.Mutex

	redeemShouldFail           bool
	getSenderInfoShouldFail    bool
	claimableReserveShouldFail bool

	checkTxErr error
	isUsedErr  error
}

func newStubBroker() *stubBroker {
	return &stubBroker{
		usedTickets:     make(map[ethcommon.Hash]bool),
		approvedSigners: make(map[ethcommon.Address]bool),
	}
}

func (b *stubBroker) FundDepositAndReserve(_, _ *big.Int) (*types.Transaction, error) {
	return nil, nil
}

func (b *stubBroker) FundDeposit(_ *big.Int) (*types.Transaction, error) {
	return nil, nil
}

func (b *stubBroker) FundReserve(_ *big.Int) (*types.Transaction, error) {
	return nil, nil
}

func (b *stubBroker) Unlock() (*types.Transaction, error) {
	return nil, nil
}

func (b *stubBroker) CancelUnlock() (*types.Transaction, error) {
	return nil, nil
}

func (b *stubBroker) Withdraw() (*types.Transaction, error) {
	return nil, nil
}

func (b *stubBroker) RedeemWinningTicket(ticket *Ticket, _ []byte, _ *big.Int) (*types.Transaction, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.redeemShouldFail {
		return nil, fmt.Errorf("stub broker redeem error")
	}

	b.usedTickets[ticket.Hash()] = true

	return types.NewTx(&types.DynamicFeeTx{}), nil
}

func (b *stubBroker) IsUsedTicket(ticket *Ticket) (bool, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.isUsedErr != nil {
		return false, b.isUsedErr
	}

	return b.usedTickets[ticket.Hash()], nil
}

func (b *stubBroker) ClaimableReserve(reserveHolder ethcommon.Address, _ ethcommon.Address) (*big.Int, error) {
	if b.claimableReserveShouldFail {
		return nil, fmt.Errorf("stub broker ClaimableReserve error")
	}

	return b.reserves[reserveHolder], nil
}

func (b *stubBroker) CheckTx(_ *types.Transaction) error {
	return b.checkTxErr
}

type stubValidator struct {
	isValidTicket   bool
	isWinningTicket bool
}

func (v *stubValidator) SetIsValidTicket(isValidTicket bool) {
	v.isValidTicket = isValidTicket
}

func (v *stubValidator) SetIsWinningTicket(isWinningTicket bool) {
	v.isWinningTicket = isWinningTicket
}

func (v *stubValidator) ValidateTicket(_ ethcommon.Address, _ *Ticket, _ []byte, _ *big.Int) error {
	if !v.isValidTicket {
		return fmt.Errorf("stub validator invalid ticket error")
	}

	return nil
}

func (v *stubValidator) IsWinningTicket(_ *Ticket, _ []byte, _ *big.Int) bool {
	return v.isWinningTicket
}

type stubSigner struct {
	account         accounts.Account
	saveSignRequest bool
	signRequests    [][]byte
	signResponse    []byte
	signShouldFail  bool
}

// TODO remove this function
// NOTE: Keeping this function for now because removing it causes the tests to fail when run with the
// logtostderr flag.
func (s *stubSigner) CreateTransactOpts(_ uint64, _ *big.Int) (*bind.TransactOpts, error) {
	return nil, nil
}

func (s *stubSigner) Sign(msg []byte) ([]byte, error) {
	if s.saveSignRequest {
		s.signRequests = append(s.signRequests, msg)
	}
	if s.signShouldFail {
		return nil, fmt.Errorf("stub returning error as requested")
	}
	return s.signResponse, nil
}

func (s *stubSigner) Account() accounts.Account {
	return s.account
}

type stubTimeManager struct {
	round              *big.Int
	blkHash            [32]byte
	preBlkHash         [32]byte
	transcoderPoolSize *big.Int
	lastSeenBlock      *big.Int

	blockNumSink chan<- *big.Int
	blockNumSub  event.Subscription

	roundSink chan<- types.Log
	roundSub  event.Subscription
}

func (m *stubTimeManager) LastInitializedRound() *big.Int {
	return m.round
}

func (m *stubTimeManager) LastInitializedL1BlockHash() [32]byte {
	return m.blkHash
}

func (m *stubTimeManager) PreLastInitializedL1BlockHash() [32]byte {
	return m.preBlkHash
}

func (m *stubTimeManager) GetTranscoderPoolSize() *big.Int {
	return m.transcoderPoolSize
}

func (m *stubTimeManager) LastSeenL1Block() *big.Int {
	return m.lastSeenBlock
}

func (m *stubTimeManager) SubscribeRounds(sink chan<- types.Log) event.Subscription {
	m.roundSink = sink
	m.roundSub = &stubSubscription{errCh: make(<-chan error)}
	return m.roundSub
}

func (m *stubTimeManager) SubscribeL1Blocks(sink chan<- *big.Int) event.Subscription {
	m.blockNumSink = sink
	m.blockNumSub = &stubSubscription{errCh: make(<-chan error)}
	return m.blockNumSub
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

type stubSenderManager struct {
	info              map[ethcommon.Address]*SenderInfo
	claimedReserve    map[ethcommon.Address]*big.Int
	err               error
	claimedReserveErr error
	reserveChangeSub  event.Subscription
	reserveChangeSink chan<- ethcommon.Address
}

func newStubSenderManager() *stubSenderManager {
	return &stubSenderManager{
		info:           make(map[ethcommon.Address]*SenderInfo),
		claimedReserve: make(map[ethcommon.Address]*big.Int),
	}
}

func (s *stubSenderManager) GetSenderInfo(addr ethcommon.Address) (*SenderInfo, error) {
	if s.err != nil {
		return nil, s.err
	}

	return s.info[addr], nil
}

func (s *stubSenderManager) ClaimedReserve(reserveHolder ethcommon.Address, _ ethcommon.Address) (*big.Int, error) {
	if s.claimedReserveErr != nil {
		return nil, s.claimedReserveErr
	}
	return s.claimedReserve[reserveHolder], nil
}

func (s *stubSenderManager) Clear(addr ethcommon.Address) {
	delete(s.info, addr)
	delete(s.claimedReserve, addr)
}

func (s *stubSenderManager) SubscribeReserveChange(sink chan<- ethcommon.Address) event.Subscription {
	s.reserveChangeSink = sink
	s.reserveChangeSub = &stubSubscription{errCh: make(<-chan error)}
	return s.reserveChangeSub
}

type stubGasPriceMonitor struct {
	gasPrice *big.Int
}

func (s *stubGasPriceMonitor) GasPrice() *big.Int {
	return s.gasPrice
}

type stubSenderMonitor struct {
	maxFloat          *big.Int
	redeemable        chan *redemption
	queued            []*SignedTicket
	acceptable        bool
	addFloatErr       error
	maxFloatErr       error
	validateSenderErr error
	shouldFail        error
}

func newStubSenderMonitor() *stubSenderMonitor {
	return &stubSenderMonitor{
		maxFloat:   big.NewInt(0),
		redeemable: make(chan *redemption),
	}
}

func (s *stubSenderMonitor) Start() {}

func (s *stubSenderMonitor) Stop() {}

func (s *stubSenderMonitor) Redeemable() chan *redemption {
	return s.redeemable
}

func (s *stubSenderMonitor) QueueTicket(ticket *SignedTicket) error {
	if s.shouldFail != nil {
		return s.shouldFail
	}
	s.queued = append(s.queued, ticket)
	return nil
}

func (s *stubSenderMonitor) AddFloat(_ ethcommon.Address, _ *big.Int) error {
	if s.addFloatErr != nil {
		return s.addFloatErr
	}

	return nil
}

func (s *stubSenderMonitor) SubFloat(_ ethcommon.Address, amount *big.Int) {
	s.maxFloat.Sub(s.maxFloat, amount)
}

func (s *stubSenderMonitor) MaxFloat(_ ethcommon.Address) (*big.Int, error) {
	if s.maxFloatErr != nil {
		return nil, s.maxFloatErr
	}

	return s.maxFloat, nil
}

func (s *stubSenderMonitor) ValidateSender(_ ethcommon.Address) error { return s.validateSenderErr }

// MockRecipient is useful for testing components that depend on pm.Recipient
type MockRecipient struct {
	mock.Mock
}

// Start initiates the helper goroutines for the recipient
func (m *MockRecipient) Start() {}

// Stop signals the recipient to exit gracefully
func (m *MockRecipient) Stop() {}

// ReceiveTicket validates and processes a received ticket
func (m *MockRecipient) ReceiveTicket(ticket *Ticket, sig []byte, seed *big.Int) (sessionID string, won bool, err error) {
	args := m.Called(ticket, sig, seed)
	return args.String(0), args.Bool(1), args.Error(2)
}

// RedeemWinningTickets redeems all winning tickets with the broker
// for all sessionIDs
func (m *MockRecipient) RedeemWinningTickets(sessionIDs []string) error {
	args := m.Called(sessionIDs)
	return args.Error(0)
}

// RedeemWinningTicket redeems a single winning ticket
func (m *MockRecipient) RedeemWinningTicket(ticket *Ticket, sig []byte, seed *big.Int) error {
	args := m.Called(ticket, sig, seed)
	return args.Error(0)
}

// TicketParams returns the recipient's currently accepted ticket parameters
// for a provided sender ETH address
func (m *MockRecipient) TicketParams(sender ethcommon.Address, price *big.Rat) (*TicketParams, error) {
	args := m.Called(sender, price)

	var params *TicketParams
	if args.Get(0) != nil {
		params = args.Get(0).(*TicketParams)
	}

	return params, args.Error(1)
}

// TxCostMultiplier returns the transaction cost multiplier for a sender based on sender's MaxFloat
func (m *MockRecipient) TxCostMultiplier(sender ethcommon.Address) (*big.Rat, error) {
	args := m.Called(sender)
	var multiplier *big.Rat
	if args.Get(0) != nil {
		multiplier = args.Get(0).(*big.Rat)
	}
	return multiplier, args.Error(1)
}

// EV Returns the recipient's request ticket EV
func (m *MockRecipient) EV() *big.Rat {
	args := m.Called()
	return args.Get(0).(*big.Rat)
}

// Sets the max ticket facevalue for the orchestrator
func (m *MockRecipient) SetMaxFaceValue(_ *big.Int) {

}

// MockSender is useful for testing components that depend on pm.Sender
type MockSender struct {
	mock.Mock
}

// StartSession creates a session for a given set of ticket params which tracks information
// for creating new tickets
func (m *MockSender) StartSession(ticketParams TicketParams) string {
	args := m.Called(ticketParams)
	return args.String(0)
}

// CleanupSession deletes session from the internal ma
func (m *MockSender) CleanupSession(sessionID string) {
	m.Called(sessionID)
}

// EV returns the ticket EV for a session
func (m *MockSender) EV(sessionID string) (*big.Rat, error) {
	args := m.Called(sessionID)

	var ev *big.Rat
	if args.Get(0) != nil {
		ev = args.Get(0).(*big.Rat)
	}

	return ev, args.Error(1)
}

// CreateTicketBatch returns a ticket batch of the specified size
func (m *MockSender) CreateTicketBatch(sessionID string, size int) (*TicketBatch, error) {
	args := m.Called(sessionID, size)

	var batch *TicketBatch
	if args.Get(0) != nil {
		batch = args.Get(0).(*TicketBatch)
	}

	return batch, args.Error(1)
}

// ValidateTicketParams checks if ticket params are acceptable
func (m *MockSender) ValidateTicketParams(ticketParams *TicketParams) error {
	args := m.Called(ticketParams)
	return args.Error(0)
}
