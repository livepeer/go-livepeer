package pm

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"math/big"
	"sync"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/monitor"
	"github.com/pkg/errors"
)

// ErrTicketParamsExpired is returned when ticket params have expired
var ErrTicketParamsExpired = errors.New("TicketParams expired")

var errInsufficientSenderReserve = errors.New("insufficient sender reserve")

// maxWinProb = 2^256 - 1
var maxWinProb = new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1))

var paramsExpirationBlock = big.NewInt(5)

// Recipient is an interface which describes an object capable
// of receiving tickets
type Recipient interface {
	// Start initiates the helper goroutines for the recipient
	Start()

	// Stop signals the recipient to exit gracefully
	Stop()

	// ReceiveTicket validates and processes a received ticket
	ReceiveTicket(ticket *Ticket, sig []byte, seed *big.Int) (sessionID string, won bool, err error)

	// RedeemWinningTickets redeems all winning tickets with the broker
	// for a all sessionIDs
	RedeemWinningTickets(sessionIDs []string) error

	// RedeemWinningTicket redeems a single winning ticket
	RedeemWinningTicket(ticket *Ticket, sig []byte, seed *big.Int) error

	// TicketParams returns the recipient's currently accepted ticket parameters
	// for a provided sender ETH adddress
	TicketParams(sender ethcommon.Address, price *big.Rat) (*TicketParams, error)

	// TxCostMultiplier returns the multiplier -
	TxCostMultiplier(sender ethcommon.Address) (*big.Rat, error)

	// EV returns the recipients EV requirement for a ticket as configured on startup
	EV() *big.Rat
}

// TicketParamsConfig contains config information for a recipient to determine
// the parameters to use for tickets
type TicketParamsConfig struct {
	// EV is the desired expected value of tickets
	EV *big.Int

	// RedeemGas is the expected gas required to redeem a ticket
	RedeemGas int

	// TxCostMultiplier is the desired multiplier of the transaction
	// cost for redemption
	TxCostMultiplier int
}

// GasPriceMonitor defines methods for monitoring gas prices
type GasPriceMonitor interface {
	GasPrice() *big.Int
}

// recipient is an implementation of the Recipient interface that
// receives tickets and redeems winning tickets
type recipient struct {
	val    Validator
	broker Broker
	store  TicketStore
	gpm    GasPriceMonitor
	sm     SenderMonitor
	tm     TimeManager

	addr   ethcommon.Address
	secret [32]byte

	invalidRands sync.Map

	senderNonces     map[string]uint32
	senderNoncesLock sync.Mutex

	cfg TicketParamsConfig

	quit chan struct{}
}

// NewRecipient creates an instance of a recipient with an
// automatically generated random secret
func NewRecipient(addr ethcommon.Address, broker Broker, val Validator, store TicketStore, gpm GasPriceMonitor, sm SenderMonitor, tm TimeManager, cfg TicketParamsConfig) (Recipient, error) {
	randBytes := make([]byte, 32)
	if _, err := rand.Read(randBytes); err != nil {
		return nil, err
	}

	var secret [32]byte
	copy(secret[:], randBytes[:32])

	return NewRecipientWithSecret(addr, broker, val, store, gpm, sm, tm, secret, cfg), nil
}

// NewRecipientWithSecret creates an instance of a recipient with a user provided
// secret. In most cases, NewRecipient should be used instead which will
// automatically generate a random secret
func NewRecipientWithSecret(addr ethcommon.Address, broker Broker, val Validator, store TicketStore, gpm GasPriceMonitor, sm SenderMonitor, tm TimeManager, secret [32]byte, cfg TicketParamsConfig) Recipient {
	return &recipient{
		broker:       broker,
		val:          val,
		store:        store,
		gpm:          gpm,
		sm:           sm,
		tm:           tm,
		addr:         addr,
		secret:       secret,
		senderNonces: make(map[string]uint32),
		cfg:          cfg,
		quit:         make(chan struct{}),
	}
}

// Start initiates the helper goroutines for the recipient
func (r *recipient) Start() {
	go r.redeemManager()
}

// Stop signals the recipient to exit gracefully
func (r *recipient) Stop() {
	close(r.quit)
}

// ReceiveTicket validates and processes a received ticket
func (r *recipient) ReceiveTicket(ticket *Ticket, sig []byte, seed *big.Int) (string, bool, error) {
	recipientRand := r.rand(seed, ticket.Sender, ticket.FaceValue, ticket.WinProb, ticket.ParamsExpirationBlock, ticket.PricePerPixel, ticket.expirationParams())

	// If sender validation check fails, abort
	if err := r.sm.ValidateSender(ticket.Sender); err != nil {
		return "", false, &FatalReceiveErr{err}
	}

	// If any of the basic ticket validity checks fail, abort
	if err := r.val.ValidateTicket(r.addr, ticket, sig, recipientRand); err != nil {
		if err.Error() == errInvalidTicketSignature.Error() {
			return "", false, err
		}
		return "", false, &FatalReceiveErr{err}
	}

	var sessionID string
	var won bool

	if r.val.IsWinningTicket(ticket, sig, recipientRand) {
		sessionID = ticket.RecipientRandHash.Hex()
		won = true
		if err := r.store.StoreWinningTicket(sessionID, ticket, sig, recipientRand); err != nil {
			glog.Errorf("error storing ticket sender=%x recipientRandHash=%x senderNonce=%v", ticket.Sender, ticket.RecipientRandHash, ticket.SenderNonce)
		}
	}

	if !r.validRand(recipientRand) {
		return sessionID, won, fmt.Errorf("recipientRand already revealed recipientRand=%v", recipientRand)
	}

	if err := r.updateSenderNonce(recipientRand, ticket.SenderNonce); err != nil {
		return sessionID, won, err
	}

	// check advertised params aren't expired
	latestBlock := r.tm.LastSeenBlock()
	if ticket.ParamsExpirationBlock.Cmp(latestBlock) <= 0 {
		return sessionID, won, ErrTicketParamsExpired
	}

	return sessionID, won, nil
}

// RedeemWinningTicket redeems all winning tickets with the broker for the provided session IDs
func (r *recipient) RedeemWinningTickets(sessionIDs []string) error {
	tickets, sigs, recipientRands, err := r.store.LoadWinningTickets(sessionIDs)
	if err != nil {
		return err
	}

	for i := 0; i < len(tickets); i++ {
		r.sm.QueueTicket(tickets[i].Sender, &SignedTicket{tickets[i], sigs[i], recipientRands[i]})
	}

	return nil
}

// RedeemWinningTicket redeems a single winning ticket
func (r *recipient) RedeemWinningTicket(ticket *Ticket, sig []byte, seed *big.Int) error {
	recipientRand := r.rand(seed, ticket.Sender, ticket.FaceValue, ticket.WinProb, ticket.ParamsExpirationBlock, ticket.PricePerPixel, ticket.expirationParams())
	r.sm.QueueTicket(ticket.Sender, &SignedTicket{ticket, sig, recipientRand})
	return nil
}

// TicketParams returns the recipient's currently accepted ticket parameters
func (r *recipient) TicketParams(sender ethcommon.Address, price *big.Rat) (*TicketParams, error) {
	randBytes := RandBytes(32)

	seed := new(big.Int).SetBytes(randBytes)

	faceValue, err := r.faceValue(sender)
	if err != nil {
		return nil, err
	}

	lastBlock := r.tm.LastSeenBlock()
	expirationBlock := new(big.Int).Add(lastBlock, paramsExpirationBlock)

	winProb := r.winProb(faceValue)

	ticketExpirationParams := &TicketExpirationParams{
		CreationRound:          r.tm.LastInitializedRound().Int64(),
		CreationRoundBlockHash: r.tm.LastInitializedBlockHash(),
	}

	recipientRand := r.rand(seed, sender, faceValue, winProb, expirationBlock, price, ticketExpirationParams)
	recipientRandHash := crypto.Keccak256Hash(ethcommon.LeftPadBytes(recipientRand.Bytes(), uint256Size))

	return &TicketParams{
		Recipient:         r.addr,
		FaceValue:         faceValue,
		WinProb:           winProb,
		RecipientRandHash: recipientRandHash,
		Seed:              seed,
		ExpirationBlock:   expirationBlock,
		PricePerPixel:     price,
		ExpirationParams:  ticketExpirationParams,
	}, nil
}

func (r *recipient) txCost() *big.Int {
	// Fetch current gasprice from cache through gasPrice monitor
	gasPrice := r.gpm.GasPrice()
	// Return txCost = redeemGas * gasPrice
	return new(big.Int).Mul(big.NewInt(int64(r.cfg.RedeemGas)), gasPrice)
}

func (r *recipient) faceValue(sender ethcommon.Address) (*big.Int, error) {
	// faceValue = txCost * txCostMultiplier
	faceValue := new(big.Int).Mul(r.txCost(), big.NewInt(int64(r.cfg.TxCostMultiplier)))

	// TODO: Consider setting faceValue to some value higher than
	// EV in this case where the default faceValue < the desired EV.
	// At the moment, for simplicity we just adjust faceValue to the
	// desired EV in this case (which would result in winProb = 100%).
	// In practice, EV should be smaller than the default faceValue
	// so this shouldn't be a problem in most cases
	if faceValue.Cmp(r.cfg.EV) < 0 {
		faceValue = r.cfg.EV
	}

	// Fetch current max float for sender
	maxFloat, err := r.sm.MaxFloat(sender)
	if err != nil {
		return nil, err
	}

	if faceValue.Cmp(maxFloat) > 0 {
		if maxFloat.Cmp(r.cfg.EV) < 0 {
			// If maxFloat < EV, then there is no
			// acceptable faceValue
			return nil, errInsufficientSenderReserve
		}

		// If faceValue > maxFloat
		// Set faceValue = maxFloat
		faceValue = maxFloat
	}

	return faceValue, nil
}

func (r *recipient) winProb(faceValue *big.Int) *big.Int {
	// Return 0 if faceValue happens to be 0
	if faceValue.Cmp(big.NewInt(0)) == 0 {
		return big.NewInt(0)
	}
	// Return maxWinProb if faceValue = EV
	if faceValue.Cmp(r.cfg.EV) == 0 {
		return maxWinProb
	}

	m := new(big.Int)
	x, m := new(big.Int).DivMod(maxWinProb, faceValue, m)
	if m.Int64() != 0 {
		return new(big.Int).Mul(r.cfg.EV, x.Add(x, big.NewInt(1)))
	}
	// Compute winProb as the numerator of a fraction over maxWinProb
	return new(big.Int).Mul(r.cfg.EV, x)
}

func (r *recipient) TxCostMultiplier(sender ethcommon.Address) (*big.Rat, error) {
	// 'r.faceValue(sender)' will return min(defaultFaceValue, MaxFloat(sender))
	faceValue, err := r.faceValue(sender)

	if err != nil {
		return nil, err
	}

	// defaultTxCostMultiplier = defaultFaceValue / txCost
	// Replacing defaultFaceValue with min(defaultFaceValue, MaxFloat(sender))
	// Will scale the TxCostMultiplier according to the effective faceValue
	return new(big.Rat).SetFrac(faceValue, r.txCost()), nil
}

func (r *recipient) redeemWinningTicket(ticket *Ticket, sig []byte, recipientRand *big.Int) error {
	maxFloat, err := r.sm.MaxFloat(ticket.Sender)
	if err != nil {
		return err
	}

	// if max float is zero, there is no claimable reserve left or reserve is 0
	if maxFloat.Cmp(big.NewInt(0)) == 0 {
		r.sm.QueueTicket(ticket.Sender, &SignedTicket{ticket, sig, recipientRand})
		return errors.Errorf("max float is zero")
	}

	// If max float is insufficient to cover the ticket face value, queue
	// the ticket to be retried later
	if maxFloat.Cmp(ticket.FaceValue) < 0 {
		r.sm.QueueTicket(ticket.Sender, &SignedTicket{ticket, sig, recipientRand})
		return fmt.Errorf("insufficient max float - faceValue=%v maxFloat=%v", ticket.FaceValue, maxFloat)
	}

	// Subtract the ticket face value from the sender's current max float
	// This amount will be considered pending until the ticket redemption
	// transaction confirms on-chain
	r.sm.SubFloat(ticket.Sender, ticket.FaceValue)

	defer func() {
		// Add the ticket face value back to the sender's current max float
		// This amount is no longer considered pending since the ticket
		// redemption transaction either confirmed on-chain or was not
		// submitted at all
		//
		// TODO(yondonfu): Should ultimately add back only the amount that
		// was actually successfully redeemed in order to take into account
		// the case where the ticket was not redeemd for its full face value
		// because the reserve was insufficient
		if err := r.sm.AddFloat(ticket.Sender, ticket.FaceValue); err != nil {
			glog.Errorf("error updating sender %x max float: %v", ticket.Sender, err)
		}
	}()

	// Assume that that this call will return immediately if there
	// is an error in transaction submission
	tx, err := r.broker.RedeemWinningTicket(ticket, sig, recipientRand)
	if err != nil {
		if monitor.Enabled {
			monitor.TicketRedemptionError(ticket.Sender.String())
		}

		return err
	}

	// If there is no error, the transaction has been submitted. As a result,
	// we assume that recipientRand has been revealed so we should invalidate it locally
	r.updateInvalidRands(recipientRand)

	// After we invalidate recipientRand we can clear the memory used to track
	// its latest senderNonce
	r.clearSenderNonce(recipientRand)

	// Wait for transaction to confirm
	if err := r.broker.CheckTx(tx); err != nil {
		if monitor.Enabled {
			monitor.TicketRedemptionError(ticket.Sender.String())
		}

		return err
	}

	if monitor.Enabled {
		// TODO(yondonfu): Handle case where < ticket.FaceValue is actually
		// redeemed i.e. if sender reserve cannot cover the full ticket.FaceValue
		monitor.ValueRedeemed(ticket.Sender.String(), ticket.FaceValue)
	}

	return nil
}

func (r *recipient) rand(seed *big.Int, sender ethcommon.Address, faceValue *big.Int, winProb *big.Int, expirationBlock *big.Int, price *big.Rat, ticketExpirationParams *TicketExpirationParams) *big.Int {
	h := hmac.New(sha256.New, r.secret[:])
	msg := append(seed.Bytes(), sender.Bytes()...)
	msg = append(msg, faceValue.Bytes()...)
	msg = append(msg, winProb.Bytes()...)
	msg = append(msg, expirationBlock.Bytes()...)
	msg = append(msg, price.Num().Bytes()...)
	msg = append(msg, price.Denom().Bytes()...)
	msg = append(msg, ticketExpirationParams.AuxData()...)

	h.Write(msg)
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

func (r *recipient) redeemManager() {
	// Listen for redeemable tickets that should be retried
	for {
		select {
		case ticket := <-r.sm.Redeemable():
			if err := r.redeemWinningTicket(ticket.Ticket, ticket.Sig, ticket.RecipientRand); err != nil {
				glog.Errorf("error redeeming ticket - sender=%x recipientRandHash=%x senderNonce=%v err=%v", ticket.Sender, ticket.RecipientRandHash, ticket.SenderNonce, err)
			}
		case <-r.quit:
			return
		}
	}
}

// EV Returns the required ticket EV for a recipient
func (r *recipient) EV() *big.Rat {
	return new(big.Rat).SetFrac(r.cfg.EV, big.NewInt(1))
}

type FatalReceiveErr struct {
	error
}

func NewFatalReceiveErr(err error) *FatalReceiveErr {
	return &FatalReceiveErr{
		err,
	}
}
