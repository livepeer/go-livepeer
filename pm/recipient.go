package pm

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"math/big"
	"sync"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/golang/glog"
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
	// ReceiveTicket validates and processes a received ticket
	ReceiveTicket(ticket *Ticket, sig []byte, seed *big.Int) (sessionID string, won bool, err error)

	// RedeemWinningTicket redeems a single winning ticket
	RedeemWinningTicket(ticket *Ticket, sig []byte, seed *big.Int) error

	// TicketParams returns the recipient's currently accepted ticket parameters
	// for a provided sender ETH adddress
	TicketParams(sender ethcommon.Address, price *big.Rat) (*TicketParams, error)

	// TxCostMultiplier returns the tx cost multiplier for an address
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
	gpm    GasPriceMonitor
	sm     SenderMonitor
	tm     TimeManager

	addr   ethcommon.Address
	secret [32]byte

	senderNonces map[string]*struct {
		nonce           uint32
		expirationBlock *big.Int
	}
	senderNoncesLock sync.Mutex

	cfg TicketParamsConfig

	quit chan struct{}
}

// NewRecipient creates an instance of a recipient with an
// automatically generated random secret
func NewRecipient(addr ethcommon.Address, broker Broker, val Validator, gpm GasPriceMonitor, sm SenderMonitor, tm TimeManager, cfg TicketParamsConfig) (Recipient, error) {
	randBytes := make([]byte, 32)
	if _, err := rand.Read(randBytes); err != nil {
		return nil, err
	}

	var secret [32]byte
	copy(secret[:], randBytes[:32])

	return NewRecipientWithSecret(addr, broker, val, gpm, sm, tm, secret, cfg), nil
}

// NewRecipientWithSecret creates an instance of a recipient with a user provided
// secret. In most cases, NewRecipient should be used instead which will
// automatically generate a random secret
func NewRecipientWithSecret(addr ethcommon.Address, broker Broker, val Validator, gpm GasPriceMonitor, sm SenderMonitor, tm TimeManager, secret [32]byte, cfg TicketParamsConfig) Recipient {
	return &recipient{
		broker: broker,
		val:    val,
		gpm:    gpm,
		sm:     sm,
		tm:     tm,
		addr:   addr,
		secret: secret,
		senderNonces: make(map[string]*struct {
			nonce           uint32
			expirationBlock *big.Int
		}),
		cfg:  cfg,
		quit: make(chan struct{}),
	}
}

// Start initiates the helper goroutines for the recipient
func (r *recipient) Start() {
	go r.senderNoncesCleanupLoop()
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
	}

	if err := r.updateSenderNonce(recipientRand, ticket); err != nil {
		return sessionID, won, err
	}

	// check advertised params aren't expired
	latestBlock := r.tm.LastSeenBlock()
	if ticket.ParamsExpirationBlock.Cmp(latestBlock) <= 0 {
		return sessionID, won, ErrTicketParamsExpired
	}

	return sessionID, won, nil
}

// RedeemWinningTicket redeems a single winning ticket
func (r *recipient) RedeemWinningTicket(ticket *Ticket, sig []byte, seed *big.Int) error {
	recipientRand := r.rand(seed, ticket.Sender, ticket.FaceValue, ticket.WinProb, ticket.ParamsExpirationBlock, ticket.PricePerPixel, ticket.expirationParams())
	return r.sm.QueueTicket(&SignedTicket{ticket, sig, recipientRand})
}

// TicketParams returns the recipient's currently accepted ticket parameters
func (r *recipient) TicketParams(sender ethcommon.Address, price *big.Rat) (*TicketParams, error) {
	randBytes := RandBytes(32)

	seed := new(big.Int).SetBytes(randBytes)

	faceValue := big.NewInt(0)
	// If price is 0 face value, win prob and EV are 0 because no payments are required
	if price.Num().Cmp(big.NewInt(0)) > 0 {
		var err error
		faceValue, err = r.faceValue(sender)
		if err != nil {
			return nil, err
		}
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

func (r *recipient) updateSenderNonce(rand *big.Int, ticket *Ticket) error {
	r.senderNoncesLock.Lock()
	defer r.senderNoncesLock.Unlock()

	randStr := rand.String()
	sn, ok := r.senderNonces[randStr]
	if ok && ticket.SenderNonce <= sn.nonce {
		return errors.Errorf("invalid ticket senderNonce sender=%v nonce=%v highest=%v", ticket.Sender.Hex(), ticket.SenderNonce, sn.nonce)
	}

	r.senderNonces[randStr] = &struct {
		nonce           uint32
		expirationBlock *big.Int
	}{ticket.SenderNonce, ticket.ParamsExpirationBlock}

	return nil
}

// EV Returns the required ticket EV for a recipient
func (r *recipient) EV() *big.Rat {
	return new(big.Rat).SetFrac(r.cfg.EV, big.NewInt(1))
}

func (r *recipient) senderNoncesCleanupLoop() {
	sink := make(chan *big.Int, 10)
	sub := r.tm.SubscribeBlocks(sink)
	defer sub.Unsubscribe()
	for {
		select {
		case <-r.quit:
			return
		case err := <-sub.Err():
			glog.Error(err)
		case latestBlock := <-sink:
			r.senderNoncesLock.Lock()
			for recipientRand, sn := range r.senderNonces {
				if sn.expirationBlock.Cmp(latestBlock) <= 0 {
					delete(r.senderNonces, recipientRand)
				}
			}
			r.senderNoncesLock.Unlock()
		}
	}
}

type FatalReceiveErr struct {
	error
}

func NewFatalReceiveErr(err error) *FatalReceiveErr {
	return &FatalReceiveErr{
		err,
	}
}
