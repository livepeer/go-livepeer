package pm

import (
	"fmt"
	"math/big"
	"sync"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
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

func (ts *stubTicketStore) LoadWinningTickets(sessionID string) ([]*Ticket, [][]byte, []*big.Int, error) {
	ts.lock.RLock()
	defer ts.lock.RUnlock()

	if ts.loadShouldFail {
		return nil, nil, nil, fmt.Errorf("stub ticket store load error")
	}

	return ts.tickets[sessionID], ts.sigs[sessionID], ts.recipientRands[sessionID], nil
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
	deposits          map[ethcommon.Address]*big.Int
	penaltyEscrows    map[ethcommon.Address]*big.Int
	usedTickets       map[ethcommon.Hash]bool
	approvedSigners   map[ethcommon.Address]bool
	redeemShouldFail  bool
	sendersShouldFail bool
}

func newStubBroker() *stubBroker {
	return &stubBroker{
		deposits:        make(map[ethcommon.Address]*big.Int),
		penaltyEscrows:  make(map[ethcommon.Address]*big.Int),
		usedTickets:     make(map[ethcommon.Hash]bool),
		approvedSigners: make(map[ethcommon.Address]bool),
	}
}

func (b *stubBroker) FundAndApproveSigners(depositAmount *big.Int, penaltyEscrowAmount *big.Int, signers []ethcommon.Address) (*types.Transaction, error) {
	return nil, nil
}

func (b *stubBroker) FundDeposit(amount *big.Int) (*types.Transaction, error) {
	return nil, nil
}

func (b *stubBroker) FundPenaltyEscrow(amount *big.Int) (*types.Transaction, error) {
	return nil, nil
}

func (b *stubBroker) ApproveSigners(signers []ethcommon.Address) (*types.Transaction, error) {
	for i := 0; i < len(signers); i++ {
		b.approvedSigners[signers[i]] = true
	}

	return nil, nil
}

func (b *stubBroker) RequestSignersRevocation(signers []ethcommon.Address) (*types.Transaction, error) {
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

func (b *stubBroker) IsApprovedSigner(sender ethcommon.Address, signer ethcommon.Address) (bool, error) {
	return b.approvedSigners[signer], nil
}

func (b *stubBroker) SetDeposit(addr ethcommon.Address, amount *big.Int) {
	b.deposits[addr] = amount
}

func (b *stubBroker) SetPenaltyEscrow(addr ethcommon.Address, amount *big.Int) {
	b.penaltyEscrows[addr] = amount
}

func (b *stubBroker) Senders(addr ethcommon.Address) (sender struct {
	Deposit       *big.Int
	PenaltyEscrow *big.Int
	WithdrawBlock *big.Int
}, err error) {
	if b.sendersShouldFail {
		err = fmt.Errorf("stub broker senders error")
		return
	}

	sender.Deposit = b.deposits[addr]
	sender.PenaltyEscrow = b.penaltyEscrows[addr]
	sender.WithdrawBlock = big.NewInt(0)

	return
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

func (v *stubValidator) ValidateTicket(ticket *Ticket, sig []byte, recipientRand *big.Int) error {
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
