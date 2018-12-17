package pm

import (
	"math/big"

	ethcommon "github.com/ethereum/go-ethereum/common"
)

type stubSigVerifier struct {
	shouldVerify bool
}

func (sv *stubSigVerifier) Verify(addr ethcommon.Address, sig []byte, msg []byte) bool {
	return sv.shouldVerify
}

type stubBroker struct {
	usedTickets     map[ethcommon.Hash]bool
	approvedSigners map[ethcommon.Address]bool
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

func (b *stubBroker) GetDeposit(addr ethcommon.Address) (*big.Int, error) {
	return big.NewInt(0), nil
}

func (b *stubBroker) GetPenaltyEscrow(addr ethcommon.Address) (*big.Int, error) {
	return big.NewInt(0), nil
}
