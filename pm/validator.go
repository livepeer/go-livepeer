package pm

import (
	"bytes"
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
)

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
	sigVerifier   SigVerifier
	roundsManager RoundsManager
}

// NewValidator returns an instance of a validator
func NewValidator(sigVerifier SigVerifier, roundsManager RoundsManager) Validator {
	return &validator{
		sigVerifier:   sigVerifier,
		roundsManager: roundsManager,
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

	if err := v.validateCreationRound(ticket.CreationRound, ticket.CreationRoundBlockHash); err != nil {
		return err
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

func (v *validator) validateCreationRound(creationRound int64, creationRoundBlockHash ethcommon.Hash) error {
	round := v.roundsManager.LastInitializedRound()
	blkHash := v.roundsManager.LastInitializedBlockHash()
	// Check that creationRound matches last initialized round
	if big.NewInt(creationRound).Cmp(round) != 0 {
		return errInvalidCreationRound
	}

	// Check that creationRoundBlockHash is valid for creationRound
	if !bytes.Equal(creationRoundBlockHash.Bytes(), blkHash[:]) {
		return errInvalidCreationRoundBlockHash
	}

	return nil
}
