package pm

import (
	"math/big"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

// Broker is an interface which serves as an abstraction over an on-chain
// smart contract that handles the administrative tasks in a probabilistic micropayment protocol
// including processing deposits and pay outs
type Broker interface {
	// FundAndApproveSigners funds a sender's deposit and penalty escrow in addition
	// to approving a set of ETH addresses to sign on behalf of the sender
	FundAndApproveSigners(depositAmount *big.Int, penaltyEscrowAmount *big.Int, signers []ethcommon.Address) (*types.Transaction, error)

	// FundDeposit funds a sender's deposit
	FundDeposit(amount *big.Int) (*types.Transaction, error)

	// FundPenaltyEscrow funds a sender's penalty escrow
	FundPenaltyEscrow(amount *big.Int) (*types.Transaction, error)

	// ApproveSigners approves a set of ETH addresses to sign on behalf of the sender
	ApproveSigners(signers []ethcommon.Address) (*types.Transaction, error)

	// RequestSignersRevocation requests the revocation of a set of approved ETH address signers
	RequestSignersRevocation(signers []ethcommon.Address) (*types.Transaction, error)

	// Unlock initiates the unlock period for a sender after which a sender can withdraw its
	// deposit and penalty escrow
	Unlock() (*types.Transaction, error)

	// CancelUnlock stops a sender's active unlock period
	CancelUnlock() (*types.Transaction, error)

	// Withdraw credits a sender with its deposit and penalty escrow after the sender
	// waits through the unlock period
	Withdraw() (*types.Transaction, error)

	// RedeemWinningTicket submits a ticket to be validated by the broker and if a valid winning ticket
	// the broker pays the ticket's face value to the ticket's recipient
	RedeemWinningTicket(ticket *Ticket, sig []byte, recipientRand *big.Int) (*types.Transaction, error)

	// IsUsedTicket checks if a ticket has been used
	IsUsedTicket(ticket *Ticket) (bool, error)

	// IsApprovedSigner checks if a ETH address signer is approved for a sender
	IsApprovedSigner(sender ethcommon.Address, signer ethcommon.Address) (bool, error)

	// Senders returns a sender's information
	Senders(addr ethcommon.Address) (struct {
		Deposit       *big.Int
		PenaltyEscrow *big.Int
		WithdrawBlock *big.Int
	}, error)
}
