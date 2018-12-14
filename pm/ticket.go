package pm

import (
	"math/big"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

// Constants for byte sizes of Solidity types
const (
	addressSize = 20
	uint256Size = 32
	bytes32Size = 32
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

// Ticket is lottery ticket payment in a probabilistic micropayment protocol
// The expected value of the ticket constitutes the payment and can be
// calculated using the ticket's face value and winning probability
type Ticket struct {
	// Recipient is the ETH address of recipient
	Recipient ethcommon.Address

	// Sender is the ETH address of sender
	Sender ethcommon.Address

	// FaceValue represents the pay out to
	// the recipient if the ticket wins
	FaceValue *big.Int

	// WinProb represents how likely a ticket will win
	WinProb *big.Int

	// SenderNonce is the monotonically increasing counter that makes
	// each ticket unique given a particular recipientRand value
	SenderNonce uint64

	// RecipientRandHash is the 32 byte keccak-256 hash commitment to a random number
	// provided by the recipient. In order for the recipient to redeem
	// a winning ticket, it must reveal the preimage to this hash
	RecipientRandHash ethcommon.Hash
}

// Hash returns the keccak-256 hash of the ticket's fields as tightly packed
// arguments as described in the Solidity documentation
// See: https://solidity.readthedocs.io/en/v0.4.25/units-and-global-variables.html#mathematical-and-cryptographic-functions
func (t *Ticket) Hash() ethcommon.Hash {
	return crypto.Keccak256Hash(t.flatten())
}

func (t *Ticket) flatten() []byte {
	buf := make([]byte, addressSize+addressSize+uint256Size+uint256Size+uint256Size+bytes32Size)
	i := copy(buf[0:], t.Recipient.Bytes())
	i += copy(buf[i:], t.Sender.Bytes())
	i += copy(buf[i:], ethcommon.LeftPadBytes(t.FaceValue.Bytes(), uint256Size))
	i += copy(buf[i:], ethcommon.LeftPadBytes(t.WinProb.Bytes(), uint256Size))
	i += copy(buf[i:], ethcommon.LeftPadBytes(new(big.Int).SetUint64(t.SenderNonce).Bytes(), uint256Size))
	i += copy(buf[i:], t.RecipientRandHash.Bytes())

	return buf
}
