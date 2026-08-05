package pm

import (
	"math/big"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/pkg/errors"
)

var (
	errInvalidTicketRecipient        = errors.New("invalid ticket recipient")
	errInvalidTicketSender           = errors.New("invalid ticket sender")
	errInvalidTicketRecipientRand    = errors.New("invalid recipientRand for ticket recipientRandHash")
	errInvalidTicketSignature        = errors.New("invalid ticket signature")
	errInvalidCreationRound          = errors.New("invalid ticket creation round")
	errInvalidCreationRoundBlockHash = errors.New("invalid ticket creation round block hash")
	errIsUsedTicket                  = errors.New("ticket already used")

	// errInsufficientSenderFunds mirrors the TicketBroker precondition introduced in
	// livepeer/protocol#657: a redemption only succeeds if the sender's deposit and
	// reserve cover the full ticket face value. This is an expected, self-resolving
	// state for an underfunded sender rather than a failure, so it is retryable.
	errInsufficientSenderFunds = errors.New("sender deposit and reserve insufficient to cover ticket face value")
)

// unconsumedRedemptionErr wraps a redemption failure for which the broker reports that
// the ticket was not consumed on-chain. Since livepeer/protocol#657 a reverted redemption
// leaves the ticket unused and still redeemable, so it must not be dropped locally.
type unconsumedRedemptionErr struct {
	err error
}

func (e unconsumedRedemptionErr) Error() string { return e.err.Error() }

func (e unconsumedRedemptionErr) Unwrap() error { return e.err }

// Validator is an interface which describes an object capable
// of validating tickets
type Validator interface {
	// ValidateTicket checks if a ticket is valid
	ValidateTicket(recipient ethcommon.Address, ticket *Ticket, sig []byte, recipientRand *big.Int) error

	// IsWinningTicket checks if a ticket won
	// Note: This method does not check if a ticket is valid which is done using ValidateTicket
	IsWinningTicket(ticket *Ticket, sig []byte, recipientRand *big.Int) bool
}

// validator is an implementation of the Validator interface
type validator struct {
	sigVerifier SigVerifier
	tm          TimeManager
}

// NewValidator returns an instance of a validator
func NewValidator(sigVerifier SigVerifier, tm TimeManager) Validator {
	return &validator{
		sigVerifier: sigVerifier,
		tm:          tm,
	}
}

// ValidateTicket checks if a ticket is valid
func (v *validator) ValidateTicket(recipient ethcommon.Address, ticket *Ticket, sig []byte, recipientRand *big.Int) error {
	if ticket.Recipient != recipient {
		return errInvalidTicketRecipient
	}

	if (ticket.Sender == ethcommon.Address{}) {
		return errInvalidTicketSender
	}

	if crypto.Keccak256Hash(ethcommon.LeftPadBytes(recipientRand.Bytes(), uint256Size)) != ticket.RecipientRandHash {
		return errInvalidTicketRecipientRand
	}

	if !v.sigVerifier.Verify(ticket.Sender, ticket.Hash().Bytes(), sig) {
		return errInvalidTicketSignature
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
