package pm

import (
	"fmt"
	"math/big"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

// Validator is an interface which describes an object capable
// of validating tickets
type Validator interface {
	// ValidateTicket checks if a ticket is valid
	ValidateTicket(ticket *Ticket, sig []byte, recipientRand *big.Int) error

	// IsWinningTicket checks if a ticket won
	// Note: This method does not check if a ticket is valid which is done using ValidateTicket
	IsWinningTicket(ticket *Ticket, sig []byte, recipientRand *big.Int) bool
}

// validator is an implementation of the Validator interface
// that relies on an implementation of the Broker interface to provide
// a set of already used tickets
type validator struct {
	addr        ethcommon.Address
	broker      Broker
	sigVerifier SigVerifier
}

// NewValidator returns an instance of a validator
func NewValidator(addr ethcommon.Address, broker Broker, sigVerifier SigVerifier) Validator {
	return &validator{
		addr:        addr,
		broker:      broker,
		sigVerifier: sigVerifier,
	}
}

// ValidateTicket checks if a ticket is valid
func (v *validator) ValidateTicket(ticket *Ticket, sig []byte, recipientRand *big.Int) error {
	if ticket.Recipient != v.addr {
		return fmt.Errorf("invalid ticket recipient")
	}

	if (ticket.Sender == ethcommon.Address{}) {
		return fmt.Errorf("invalid ticket sender")
	}

	if crypto.Keccak256Hash(ethcommon.LeftPadBytes(recipientRand.Bytes(), uint256Size)) != ticket.RecipientRandHash {
		return fmt.Errorf("invalid preimage provided for hash commitment recipientRandHash")
	}

	used, err := v.broker.IsUsedTicket(ticket)
	if err != nil {
		return err
	}

	if used {
		return fmt.Errorf("ticket has already been used")
	}

	if !v.sigVerifier.Verify(ticket.Sender, sig, ticket.Hash().Bytes()) {
		return fmt.Errorf("invalid sender signature over ticket hash")
	}

	return nil
}

// IsWinningTicket checks if a ticket won
// Note: This method does not check if a ticket is valid which is done using IsValidTicket
// A ticket wins if:
// H(SIG(H(T)), T.RecipientRand) < T.WinProb
func (v *validator) IsWinningTicket(ticket *Ticket, sig []byte, recipientRand *big.Int) bool {
	recipientRandBytes := ethcommon.LeftPadBytes(recipientRand.Bytes(), bytes32Size)
	res := new(big.Int).SetBytes(crypto.Keccak256(sig, recipientRandBytes))

	return res.Cmp(ticket.WinProb) < 0
}
