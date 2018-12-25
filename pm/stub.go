package pm

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

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
	deposits        map[ethcommon.Address]*big.Int
	penaltyEscrows  map[ethcommon.Address]*big.Int
	usedTickets     map[ethcommon.Hash]bool
	approvedSigners map[ethcommon.Address]bool
}

func newStubBroker() *stubBroker {
	return &stubBroker{
		deposits:        make(map[ethcommon.Address]*big.Int),
		penaltyEscrows:  make(map[ethcommon.Address]*big.Int),
		usedTickets:     make(map[ethcommon.Hash]bool),
		approvedSigners: make(map[ethcommon.Address]bool),
	}
}

func (b *stubBroker) FundAndApproveSigners(depositAmount *big.Int, penaltyEscrowAmount *big.Int, signers []ethcommon.Address) error {
	return nil
}

func (b *stubBroker) FundDeposit(amount *big.Int) error {
	return nil
}

func (b *stubBroker) FundPenaltyEscrow(amount *big.Int) error {
	return nil
}

func (b *stubBroker) ApproveSigners(signers []ethcommon.Address) error {
	for i := 0; i < len(signers); i++ {
		b.approvedSigners[signers[i]] = true
	}

	return nil
}

func (b *stubBroker) RequestSignersRevocation(signers []ethcommon.Address) error {
	return nil
}

func (b *stubBroker) Unlock() error {
	return nil
}

func (b *stubBroker) CancelUnlock() error {
	return nil
}

func (b *stubBroker) Withdraw() error {
	return nil
}

func (b *stubBroker) RedeemWinningTicket(ticket *Ticket, sig []byte, recipientRand *big.Int) error {
	b.usedTickets[ticket.Hash()] = true

	return nil
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

func (b *stubBroker) GetDeposit(addr ethcommon.Address) (*big.Int, error) {
	deposit, ok := b.deposits[addr]
	if !ok {
		return nil, fmt.Errorf("no deposit for %x", addr)
	}

	return deposit, nil
}

func (b *stubBroker) SetPenaltyEscrow(addr ethcommon.Address, amount *big.Int) {
	b.penaltyEscrows[addr] = amount
}

func (b *stubBroker) GetPenaltyEscrow(addr ethcommon.Address) (*big.Int, error) {
	penaltyEscrow, ok := b.penaltyEscrows[addr]
	if !ok {
		return nil, fmt.Errorf("no penalty escrow for %x", addr)
	}

	return penaltyEscrow, nil
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
		return fmt.Errorf("invalid ticket")
	}

	return nil
}

func (v *stubValidator) IsWinningTicket(ticket *Ticket, sig []byte, recipientRand *big.Int) bool {
	return v.isWinningTicket
}

type stubAccountManager struct {
	account         accounts.Account
	saveSignRequest bool
	lastSignRequest []byte
	signResponse    []byte
	signShouldFail  bool
}

func (am *stubAccountManager) Unlock(passphrase string) error {
	return nil
}

func (am *stubAccountManager) Lock() error {
	return nil
}

func (am *stubAccountManager) CreateTransactOpts(gasLimit uint64, gasPrice *big.Int) (*bind.TransactOpts, error) {
	return nil, nil
}

func (am *stubAccountManager) SignTx(signer types.Signer, tx *types.Transaction) (*types.Transaction, error) {
	return nil, nil
}

func (am *stubAccountManager) Sign(msg []byte) ([]byte, error) {
	if am.saveSignRequest {
		am.lastSignRequest = msg
	}
	if am.signShouldFail {
		return nil, fmt.Errorf("stub returning error as requested")
	}
	return am.signResponse, nil
}

func (am *stubAccountManager) Account() accounts.Account {
	return am.account
}
