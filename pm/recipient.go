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

// Recipient is an interface which describes an object capable
// of receiving tickets
type Recipient interface {
	// ReceiveTicket validates and processes a received ticket
	ReceiveTicket(ticket *Ticket, sig []byte, seed *big.Int) (sessionID string, won bool, err error)

	// RedeemWinningTickets redeems all winning tickets with the broker
	// for a all sessionIDs
	RedeemWinningTickets(sessionIDs []string) error

	// TicketParams returns the recipient's currently accepted ticket parameters
	// for a provided sender ETH adddress
	TicketParams(sender ethcommon.Address) *TicketParams
}

// recipient is an implementation of the Recipient interface that
// receives tickets and redeems winning tickets
type recipient struct {
	val    Validator
	broker Broker
	store  TicketStore

	addr   ethcommon.Address
	secret [32]byte

	invalidRands sync.Map

	senderNonces     map[string]uint32
	senderNoncesLock sync.Mutex

	faceValue *big.Int
	winProb   *big.Int
}

// NewRecipient creates an instance of a recipient with an
// automatically generated random secret
func NewRecipient(addr ethcommon.Address, broker Broker, val Validator, store TicketStore, faceValue *big.Int, winProb *big.Int) (Recipient, error) {
	randBytes := make([]byte, 32)
	if _, err := rand.Read(randBytes); err != nil {
		return nil, err
	}

	var secret [32]byte
	copy(secret[:], randBytes[:32])

	return NewRecipientWithSecret(addr, broker, val, store, secret, faceValue, winProb), nil
}

// NewRecipientWithSecret creates an instance of a recipient with a user provided
// secret. In most cases, NewRecipient should be used instead which will
// automatically generate a random secret
func NewRecipientWithSecret(addr ethcommon.Address, broker Broker, val Validator, store TicketStore, secret [32]byte, faceValue *big.Int, winProb *big.Int) Recipient {
	return &recipient{
		broker:       broker,
		val:          val,
		store:        store,
		addr:         addr,
		secret:       secret,
		faceValue:    faceValue,
		senderNonces: make(map[string]uint32),
		winProb:      winProb,
	}
}

// ReceiveTicket validates and processes a received ticket
func (r *recipient) ReceiveTicket(ticket *Ticket, sig []byte, seed *big.Int) (string, bool, error) {
	recipientRand := r.rand(seed, ticket.Sender)

	if crypto.Keccak256Hash(ethcommon.LeftPadBytes(recipientRand.Bytes(), uint256Size)) != ticket.RecipientRandHash {
		return "", false, errors.Errorf("invalid recipientRand generated from seed %v", seed)
	}

	if ticket.FaceValue.Cmp(r.faceValue) != 0 {
		return "", false, errors.Errorf("invalid ticket faceValue %v", ticket.FaceValue)
	}

	if ticket.WinProb.Cmp(r.winProb) != 0 {
		return "", false, errors.Errorf("invalid ticket winProb %v", ticket.WinProb)
	}

	if err := r.val.ValidateTicket(r.addr, ticket, sig, recipientRand); err != nil {
		return "", false, err
	}

	if !r.validRand(recipientRand) {
		return "", false, errors.Errorf("invalid already revealed recipientRand %v", recipientRand)
	}

	if err := r.updateSenderNonce(recipientRand, ticket.SenderNonce); err != nil {
		return "", false, err
	}

	if r.val.IsWinningTicket(ticket, sig, recipientRand) {
		sessionID := ticket.RecipientRandHash.Hex()
		if err := r.store.StoreWinningTicket(sessionID, ticket, sig, recipientRand); err != nil {
			return "", true, err
		}

		return sessionID, true, nil
	}

	return "", false, nil
}

// RedeemWinningTicket redeems all winning tickets with the broker
// for a all sessionIDs
func (r *recipient) RedeemWinningTickets(sessionIDs []string) error {
	tickets, sigs, recipientRands, err := r.store.LoadWinningTickets(sessionIDs)
	if err != nil {
		return err
	}

	for i := 0; i < len(tickets); i++ {
		if err := r.redeemWinningTicket(tickets[i], sigs[i], recipientRands[i]); err != nil {
			return err
		}
	}

	return nil
}

// TicketParams returns the recipient's currently accepted ticket parameters
func (r *recipient) TicketParams(sender ethcommon.Address) *TicketParams {
	randBytes := RandBytes(32)

	seed := new(big.Int).SetBytes(randBytes)
	recipientRand := r.rand(seed, sender)
	recipientRandHash := crypto.Keccak256Hash(ethcommon.LeftPadBytes(recipientRand.Bytes(), uint256Size))

	return &TicketParams{
		Recipient:         r.addr,
		FaceValue:         r.faceValue,
		WinProb:           r.winProb,
		RecipientRandHash: recipientRandHash,
		Seed:              seed,
	}
}

func (r *recipient) redeemWinningTicket(ticket *Ticket, sig []byte, recipientRand *big.Int) error {
	info, err := r.broker.GetSenderInfo(ticket.Sender)
	if err != nil {
		return err
	}

	// TODO: Consider a smarter strategy here in the future
	// Ex. If deposit < transaction cost, do not try to redeem
	if info.Deposit.Cmp(big.NewInt(0)) == 0 && info.Reserve.Cmp(big.NewInt(0)) == 0 {
		return errors.Errorf("sender %v has zero deposit and reserve", ticket.Sender)
	}

	// Assume that that this call will return immediately if there
	// is an error in transaction submission. Else, the function will kick off
	// a goroutine and then return to the caller
	if _, err := r.broker.RedeemWinningTicket(ticket, sig, recipientRand); err != nil {
		return err
	}

	// If there is no error, the transaction has been submitted. As a result,
	// we assume that recipientRand has been revealed so we should invalidate it locally
	r.updateInvalidRands(recipientRand)

	// After we invalidate recipientRand we can clear the memory used to track
	// its latest senderNonce
	r.clearSenderNonce(recipientRand)

	return nil
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

func (r *recipient) updateInvalidRands(rand *big.Int) {
	r.invalidRands.Store(rand.String(), true)
}

func (r *recipient) updateSenderNonce(rand *big.Int, senderNonce uint32) error {
	r.senderNoncesLock.Lock()
	defer r.senderNoncesLock.Unlock()

	randStr := rand.String()
	nonce, ok := r.senderNonces[randStr]
	if ok && senderNonce <= nonce {
		return errors.Errorf("invalid ticket senderNonce %v - highest seen is %v", senderNonce, nonce)
	}

	r.senderNonces[randStr] = senderNonce

	return nil
}

func (r *recipient) clearSenderNonce(rand *big.Int) {
	r.senderNoncesLock.Lock()
	defer r.senderNoncesLock.Unlock()

	delete(r.senderNonces, rand.String())
}
