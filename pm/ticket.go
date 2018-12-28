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

// TicketParams represents the parameters defined by a receiver that a sender must adhere to when
// sending tickets to receiver.
type TicketParams struct {
	FaceValue *big.Int

	WinProb *big.Int

	RecipientRandHash ethcommon.Hash

	Seed *big.Int
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
	SenderNonce uint32

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
	i += copy(buf[i:], ethcommon.LeftPadBytes(new(big.Int).SetUint64(uint64(t.SenderNonce)).Bytes(), uint256Size))
	i += copy(buf[i:], t.RecipientRandHash.Bytes())

	return buf
}
