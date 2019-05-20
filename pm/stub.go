package pm

import (
	"fmt"
	"math/big"
	"sync"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/mock"
)

type stubTicketStore struct {
	tickets         map[string][]*Ticket
	sigs            map[string][][]byte
	recipientRands  map[string][]*big.Int
	storeShouldFail bool
	loadShouldFail  bool
	lock            sync.RWMutex
}

func newStubTicketStore() *stubTicketStore {
	return &stubTicketStore{
		tickets:        make(map[string][]*Ticket),
		sigs:           make(map[string][][]byte),
		recipientRands: make(map[string][]*big.Int),
	}
}

func (ts *stubTicketStore) StoreWinningTicket(sessionID string, ticket *Ticket, sig []byte, recipientRand *big.Int) error {
	ts.lock.Lock()
	defer ts.lock.Unlock()

	if ts.storeShouldFail {
		return fmt.Errorf("stub ticket store store error")
	}

	ts.tickets[sessionID] = append(ts.tickets[sessionID], ticket)
	ts.sigs[sessionID] = append(ts.sigs[sessionID], sig)
	ts.recipientRands[sessionID] = append(ts.recipientRands[sessionID], recipientRand)

	return nil
}

func (ts *stubTicketStore) LoadWinningTickets(sessionIDs []string) ([]*Ticket, [][]byte, []*big.Int, error) {
	ts.lock.RLock()
	defer ts.lock.RUnlock()

	if ts.loadShouldFail {
		return nil, nil, nil, fmt.Errorf("stub ticket store load error")
	}

	allTix := make([]*Ticket, 0)
	allSigs := make([][]byte, 0)
	allRecipientRands := make([]*big.Int, 0)

	for _, sessionID := range sessionIDs {
		tickets := ts.tickets[sessionID]
		if tickets != nil && len(tickets) > 0 {
			allTix = append(allTix, tickets...)
		}
		sigs := ts.sigs[sessionID]
		if sigs != nil && len(sigs) > 0 {
			allSigs = append(allSigs, sigs...)
		}
		recipientRands := ts.recipientRands[sessionID]
		if recipientRands != nil && len(recipientRands) > 0 {
			allRecipientRands = append(allRecipientRands, recipientRands...)
		}
	}

	return allTix, allSigs, allRecipientRands, nil
}

type stubSigVerifier struct {
	verifyResult bool
}

func (sv *stubSigVerifier) SetVerifyResult(verifyResult bool) {
	sv.verifyResult = verifyResult
}

func (sv *stubSigVerifier) Verify(addr ethcommon.Address, msg, sig []byte) bool {
	return sv.verifyResult
}

type stubBroker struct {
	deposits                   map[ethcommon.Address]*big.Int
	reserves                   map[ethcommon.Address]*big.Int
	usedTickets                map[ethcommon.Hash]bool
	approvedSigners            map[ethcommon.Address]bool
	redeemShouldFail           bool
	getSenderInfoShouldFail    bool
	remainingReserveShouldFail bool
}

func newStubBroker() *stubBroker {
	return &stubBroker{
		deposits:        make(map[ethcommon.Address]*big.Int),
		reserves:        make(map[ethcommon.Address]*big.Int),
		usedTickets:     make(map[ethcommon.Hash]bool),
		approvedSigners: make(map[ethcommon.Address]bool),
	}
}

func (b *stubBroker) FundDepositAndReserve(depositAmount, reserveAmount *big.Int) (*types.Transaction, error) {
	return nil, nil
}

func (b *stubBroker) FundDeposit(amount *big.Int) (*types.Transaction, error) {
	return nil, nil
}

func (b *stubBroker) FundReserve(amount *big.Int) (*types.Transaction, error) {
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

func (b *stubBroker) RedeemWinningTicket(ticket *Ticket, sig []byte, recipientRand *big.Int) (*types.Transaction, error) {
	if b.redeemShouldFail {
		return nil, fmt.Errorf("stub broker redeem error")
	}

	b.usedTickets[ticket.Hash()] = true

	return nil, nil
}

func (b *stubBroker) IsUsedTicket(ticket *Ticket) (bool, error) {
	return b.usedTickets[ticket.Hash()], nil
}

func (b *stubBroker) SetDeposit(addr ethcommon.Address, amount *big.Int) {
	b.deposits[addr] = amount
}

func (b *stubBroker) SetReserve(addr ethcommon.Address, amount *big.Int) {
	b.reserves[addr] = amount
}

func (b *stubBroker) GetSenderInfo(addr ethcommon.Address) (info *SenderInfo, err error) {
	if b.getSenderInfoShouldFail {
		return nil, fmt.Errorf("stub broker GetSenderInfo error")
	}

	return &SenderInfo{
		Deposit:       b.deposits[addr],
		WithdrawBlock: big.NewInt(0),
		Reserve:       b.reserves[addr],
		ReserveState:  ReserveState(0),
		ThawRound:     big.NewInt(0),
	}, nil
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

func (v *stubValidator) ValidateTicket(recipient ethcommon.Address, ticket *Ticket, sig []byte, recipientRand *big.Int) error {
	if !v.isValidTicket {
		return fmt.Errorf("stub validator invalid ticket error")
	}

	return nil
}

func (v *stubValidator) IsWinningTicket(ticket *Ticket, sig []byte, recipientRand *big.Int) bool {
	return v.isWinningTicket
}

type stubSigner struct {
	account         accounts.Account
	saveSignRequest bool
	lastSignRequest []byte
	signResponse    []byte
	signShouldFail  bool
}

// TODO remove this function
// NOTE: Keeping this function for now because removing it causes the tests to fail when run with the
// logtostderr flag.
func (s *stubSigner) CreateTransactOpts(gasLimit uint64, gasPrice *big.Int) (*bind.TransactOpts, error) {
	return nil, nil
}

func (s *stubSigner) Sign(msg []byte) ([]byte, error) {
	if s.saveSignRequest {
		s.lastSignRequest = msg
	}
	if s.signShouldFail {
		return nil, fmt.Errorf("stub returning error as requested")
	}
	return s.signResponse, nil
}

func (s *stubSigner) Account() accounts.Account {
	return s.account
}

// MockRecipient is useful for testing components that depend on pm.Recipient
type MockRecipient struct {
	mock.Mock
}

// ReceiveTicket validates and processes a received ticket
func (m *MockRecipient) ReceiveTicket(ticket *Ticket, sig []byte, seed *big.Int) (sessionID string, won bool, err error) {
	args := m.Called(ticket, sig, seed)
	return args.String(0), args.Bool(1), args.Error(2)
}

// RedeemWinningTickets redeems all winning tickets with the broker
// for a all sessionIDs
func (m *MockRecipient) RedeemWinningTickets(sessionIDs []string) error {
	args := m.Called(sessionIDs)
	return args.Error(0)
}

// TicketParams returns the recipient's currently accepted ticket parameters
// for a provided sender ETH adddress
func (m *MockRecipient) TicketParams(sender ethcommon.Address) *TicketParams {
	args := m.Called(sender)

	var params *TicketParams
	if args.Get(0) != nil {
		params = args.Get(0).(*TicketParams)
	}

	return params
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

// CreateTicket returns a new ticket, seed (which the recipient can use to derive its random number),
// and signature over the new ticket for a given session ID
func (m *MockSender) CreateTicket(sessionID string) (*Ticket, *big.Int, []byte, error) {
	args := m.Called(sessionID)

	var ticket *Ticket
	var seed *big.Int
	var sig []byte

	if args.Get(0) != nil {
		ticket = args.Get(0).(*Ticket)
	}

	if args.Get(1) != nil {
		seed = args.Get(1).(*big.Int)
	}

	if args.Get(2) != nil {
		sig = args.Get(2).([]byte)
	}

	return ticket, seed, sig, args.Error(3)
}
