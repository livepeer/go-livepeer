package pm

import (
	"math/big"

	ethcommon "github.com/ethereum/go-ethereum/common"
)

// Broker is an interface which serves as an abstraction over an on-chain
// smart contract that handles the administrative tasks in a probabilistic micropayment protocol
// including processing deposits and pay outs
type Broker interface {
	// FundAndApproveSigners funds a sender's deposit and penalty escrow in addition
	// to approving a set of ETH addresses to sign on behalf of the sender
	FundAndApproveSigners(depositAmount *big.Int, penaltyEscrowAmount *big.Int, signers []ethcommon.Address) error

	// FundDeposit funds a sender's deposit
	FundDeposit(amount *big.Int) error

	// FundPenaltyEscrow funds a sender's penalty escrow
	FundPenaltyEscrow(amount *big.Int) error

	// ApproveSigners approves a set of ETH addresses to sign on behalf of the sender
	ApproveSigners(signers []ethcommon.Address) error

	// RequestSignersRevocation requests the revocation of a set of approved ETH address signers
	RequestSignersRevocation(signers []ethcommon.Address) error

	// Unlock initiates the unlock period for a sender after which a sender can withdraw its
	// deposit and penalty escrow
	Unlock() error

	// CancelUnlock stops a sender's active unlock period
	CancelUnlock() error

	// Withdraw credits a sender with its deposit and penalty escrow after the sender
	// waits through the unlock period
	Withdraw() error

	// RedeemWinningTicket submits a ticket to be validated by the broker and if a valid winning ticket
	// the broker pays the ticket's face value to the ticket's recipient
	RedeemWinningTicket(ticket *Ticket, sig []byte, recipientRand *big.Int) error

	// IsUsedTicket checks if a ticket has been used
	IsUsedTicket(ticket *Ticket) (bool, error)

	// IsApprovedSigner checks if a ETH address signer is approved for a sender
	IsApprovedSigner(sender ethcommon.Address, signer ethcommon.Address) (bool, error)

	// GetDeposit returns the deposit value managed by the broker for a specified user
	GetDeposit(addr ethcommon.Address) (*big.Int, error)

	// GetPenaltyEscrow returns the penalty escrow value managed by the broker for a specified user
	GetPenaltyEscrow(addr ethcommon.Address) (*big.Int, error)
}
