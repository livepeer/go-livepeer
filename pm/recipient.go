package pm

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"math/big"
	"sync"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/pkg/errors"
)

var (
	errInvalidRecipientRandFromSeed = errors.New("invalid recipientRand generated from seed")
	errInvalidRecipientRandRevealed = errors.New("invalid already revealed recipientRand")

	errInvalidTicketFaceValue   = errors.New("invalid ticket faceValue")
	errInvalidTicketWinProb     = errors.New("invalid ticket winProb")
	errInvalidTicketSenderNonce = errors.New("invalid ticket senderNonce")

	errZeroDepositAndPenaltyEscrow = errors.New("sender has zero deposit and penalty escrow")
)

// Recipient is an interface which describes an object capable
// of receiving tickets
type Recipient interface {
	// ReceiveTicket validates and processes a received ticket
	ReceiveTicket(ticket *Ticket, sig []byte, seed *big.Int) (won bool, err error)

	// RedeemWinningTicket redeems a winning ticket with the broker
	RedeemWinningTicket(ticketID ethcommon.Hash) error

	// TicketParams returns the recipient's currently accepted ticket parameters
	// for a provided sender ETH adddress
	TicketParams(sender ethcommon.Address) (*TicketParams, error)
}

// recipient is an implementation of the Recipient interface that
// receives tickets and redeems winning tickets
type recipient struct {
	val    Validator
	broker Broker
	store  TicketStore

	secret [32]byte

	invalidRands sync.Map

	senderNonces     map[string]uint64
	senderNoncesLock sync.Mutex

	faceValue *big.Int
	winProb   *big.Int
}

// NewRecipient creates an instance of a recipient
func NewRecipient(broker Broker, val Validator, store TicketStore, secret [32]byte, faceValue *big.Int, winProb *big.Int) Recipient {
	return &recipient{
		broker:       broker,
		val:          val,
		store:        store,
		secret:       secret,
		faceValue:    faceValue,
		senderNonces: make(map[string]uint64),
		winProb:      winProb,
	}
}

// ReceiveTicket validates and processes a received ticket
func (r *recipient) ReceiveTicket(ticket *Ticket, sig []byte, seed *big.Int) (bool, error) {
	recipientRand := r.rand(seed, ticket.Sender)

	if crypto.Keccak256Hash(ethcommon.LeftPadBytes(recipientRand.Bytes(), uint256Size)) != ticket.RecipientRandHash {
		return false, errInvalidRecipientRandFromSeed
	}

	if ticket.FaceValue.Cmp(r.faceValue) != 0 {
		return false, errInvalidTicketFaceValue
	}

	if ticket.WinProb.Cmp(r.winProb) != 0 {
		return false, errInvalidTicketWinProb
	}

	if err := r.val.ValidateTicket(ticket, sig, recipientRand); err != nil {
		return false, err
	}

	if !r.validRand(recipientRand) {
		return false, errInvalidRecipientRandRevealed
	}

	if err := r.updateSenderNonce(recipientRand, ticket.SenderNonce); err != nil {
		return false, err
	}

	if r.val.IsWinningTicket(ticket, sig, recipientRand) {
		if err := r.store.Store(ticket, sig, recipientRand); err != nil {
			return true, err
		}

		return true, nil
	}

	return false, nil
}

// RedeemWinningTicket redeems a winning ticket with the broker
func (r *recipient) RedeemWinningTicket(ticketID ethcommon.Hash) error {
	ticket, sig, recipientRand, err := r.store.Load(ticketID)
	if err != nil {
		return err
	}

	deposit, err := r.broker.GetDeposit(ticket.Sender)
	if err != nil {
		return err
	}

	penaltyEscrow, err := r.broker.GetPenaltyEscrow(ticket.Sender)
	if err != nil {
		return err
	}

	if deposit.Cmp(big.NewInt(0)) == 0 && penaltyEscrow.Cmp(big.NewInt(0)) == 0 {
		return errZeroDepositAndPenaltyEscrow
	}

	// We are about to reveal recipientRand on-chain so invalidate it locally
	if err := r.updateInvalidRands(recipientRand); err != nil {
		return err
	}

	// After we invalidate recipientRand we can clear the memory used to track
	// its latest senderNonce
	r.clearSenderNonce(recipientRand)

	if err := r.broker.RedeemWinningTicket(ticket, sig, recipientRand); err != nil {
		return err
	}

	return nil
}

// TicketParams returns the recipient's currently accepted ticket parameters
func (r *recipient) TicketParams(sender ethcommon.Address) (*TicketParams, error) {
	randBytes := make([]byte, 32)
	if _, err := rand.Read(randBytes); err != nil {
		return nil, err
	}

	seed := new(big.Int).SetBytes(randBytes)
	recipientRand := r.rand(seed, sender)
	recipientRandHash := crypto.Keccak256Hash(ethcommon.LeftPadBytes(recipientRand.Bytes(), uint256Size))

	return &TicketParams{
		FaceValue:         r.faceValue,
		WinProb:           r.winProb,
		RecipientRandHash: recipientRandHash,
		Seed:              seed,
	}, nil
}

func (r *recipient) rand(seed *big.Int, sender ethcommon.Address) *big.Int {
	h := hmac.New(sha256.New, r.secret[:])
	h.Write(append(seed.Bytes(), sender.Bytes()...))

	return new(big.Int).SetBytes(h.Sum(nil))
}

func (r *recipient) validRand(rand *big.Int) bool {
	_, ok := r.invalidRands.Load(rand.String())
	return !ok
}

func (r *recipient) updateInvalidRands(rand *big.Int) error {
	_, loaded := r.invalidRands.LoadOrStore(rand.String(), true)
	if loaded {
		return errInvalidRecipientRandRevealed
	}

	return nil
}

func (r *recipient) updateSenderNonce(rand *big.Int, senderNonce uint64) error {
	r.senderNoncesLock.Lock()
	defer r.senderNoncesLock.Unlock()

	randStr := rand.String()
	nonce, ok := r.senderNonces[randStr]
	if ok && senderNonce <= nonce {
		return errInvalidTicketSenderNonce
	}

	r.senderNonces[randStr] = senderNonce

	return nil
}

func (r *recipient) clearSenderNonce(rand *big.Int) {
	r.senderNoncesLock.Lock()
	defer r.senderNoncesLock.Unlock()

	delete(r.senderNonces, rand.String())
}
