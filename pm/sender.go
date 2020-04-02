package pm

import (
	"fmt"
	"math/big"
	"sync"
	"sync/atomic"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"
)

// ErrSenderValidation is returned when the sender cannot send tickets
type ErrSenderValidation struct {
	error
}

// Sender enables starting multiple probabilistic micropayment sessions with multiple recipients
// and create tickets that adhere to each session's params and unique nonce requirements.
type Sender interface {
	// StartSession creates a session for a given set of ticket params which tracks information
	// for creating new tickets
	StartSession(ticketParams TicketParams) string

	// CreateTicketBatch returns a ticket batch of the specified size
	CreateTicketBatch(sessionID string, size int) (*TicketBatch, error)

	// ValidateTicketParams checks if ticket params are acceptable
	ValidateTicketParams(ticketParams *TicketParams) error

	// EV returns the ticket EV for a session
	EV(sessionID string) (*big.Rat, error)
}

type session struct {
	senderNonce uint32

	ticketParams TicketParams
}

type sender struct {
	signer            Signer
	timeManager       TimeManager
	senderManager     SenderManager
	maxEV             *big.Rat
	depositMultiplier int

	sessions sync.Map
}

// NewSender creates a new Sender instance.
func NewSender(signer Signer, timeManager TimeManager, senderManager SenderManager, maxEV *big.Rat, depositMultiplier int) Sender {
	return &sender{
		signer:            signer,
		timeManager:       timeManager,
		senderManager:     senderManager,
		maxEV:             maxEV,
		depositMultiplier: depositMultiplier,
	}
}

func (s *sender) StartSession(ticketParams TicketParams) string {
	sessionID := ticketParams.RecipientRandHash.Hex()

	s.sessions.Store(sessionID, &session{
		ticketParams: ticketParams,
		senderNonce:  0,
	})

	return sessionID
}

// EV returns the ticket EV for a session
func (s *sender) EV(sessionID string) (*big.Rat, error) {
	session, err := s.loadSession(sessionID)
	if err != nil {
		return nil, err
	}

	return ticketEV(session.ticketParams.FaceValue, session.ticketParams.WinProb), nil
}

func (s *sender) validateSender(info *SenderInfo) error {
	maxWithdrawRound := new(big.Int).Add(s.timeManager.LastInitializedRound(), big.NewInt(1))
	if info.WithdrawRound.Int64() != 0 && info.WithdrawRound.Cmp(maxWithdrawRound) != 1 {
		return ErrSenderValidation{fmt.Errorf("unable to validate sender: deposit and reserve is set to unlock soon")}
	}

	if info.Reserve.FundsRemaining.Cmp(big.NewInt(0)) == 0 {
		return ErrSenderValidation{errors.New("no sender reserve")}
	}

	if info.Deposit.Cmp(big.NewInt(0)) == 0 {
		return ErrSenderValidation{errors.New("no sender deposit")}
	}

	return nil
}

// CreateTicketBatch returns a ticket batch of the specified size
func (s *sender) CreateTicketBatch(sessionID string, size int) (*TicketBatch, error) {
	session, err := s.loadSession(sessionID)
	if err != nil {
		return nil, err
	}

	if err := s.validateTicketParams(&session.ticketParams, size); err != nil {
		return nil, err
	}

	ticketParams := &session.ticketParams

	expirationParams := ticketParams.ExpirationParams
	// Ensure backwards compatbility
	// If no expirationParams are included by O
	// B sets the values based upon its last seen round
	if expirationParams == nil || expirationParams.CreationRound == 0 || expirationParams.CreationRoundBlockHash == (ethcommon.Hash{}) {
		expirationParams = s.expirationParams()
	}

	batch := &TicketBatch{
		TicketParams:           ticketParams,
		TicketExpirationParams: expirationParams,
		Sender:                 s.signer.Account().Address,
	}

	for i := 0; i < size; i++ {
		senderNonce := atomic.AddUint32(&session.senderNonce, 1)
		ticket := NewTicket(&session.ticketParams, expirationParams, s.signer.Account().Address, senderNonce)
		sig, err := s.signer.Sign(ticket.Hash().Bytes())
		if err != nil {
			return nil, errors.Wrapf(err, "error signing ticket for session: %v", sessionID)
		}

		batch.SenderParams = append(batch.SenderParams, &TicketSenderParams{SenderNonce: senderNonce, Sig: sig})
	}

	return batch, nil
}

// ValidateTicketParams checks if ticket params are acceptable
func (s *sender) ValidateTicketParams(ticketParams *TicketParams) error {
	// Check for sending a single ticket
	return s.validateTicketParams(ticketParams, 1)
}

// validateTicketParams checks if ticket params are acceptable for a specific number of tickets
func (s *sender) validateTicketParams(ticketParams *TicketParams, numTickets int) error {
	info, err := s.senderManager.GetSenderInfo(s.signer.Account().Address)
	if err != nil {
		return err
	}

	// validate sender
	if err := s.validateSender(info); err != nil {
		return err
	}

	ev := ticketEV(ticketParams.FaceValue, ticketParams.WinProb)
	totalEV := ev.Mul(ev, new(big.Rat).SetInt64(int64(numTickets)))
	if totalEV.Cmp(s.maxEV) > 0 {
		return fmt.Errorf("total ticket EV %v for %v tickets > max total ticket EV %v", totalEV.FloatString(5), numTickets, s.maxEV.FloatString(5))
	}

	maxFaceValue := new(big.Int).Div(info.Deposit, big.NewInt(int64(s.depositMultiplier)))
	if ticketParams.FaceValue.Cmp(maxFaceValue) > 0 {
		return fmt.Errorf("ticket faceValue %v > max faceValue %v", ticketParams.FaceValue, maxFaceValue)
	}

	if ticketParams.ExpirationBlock.Int64() == 0 {
		return nil
	}

	latestBlock := s.timeManager.LastSeenBlock()
	if ticketParams.ExpirationBlock.Cmp(latestBlock) <= 0 {
		return ErrTicketParamsExpired
	}

	return nil
}

func (s *sender) expirationParams() *TicketExpirationParams {
	round := s.timeManager.LastInitializedRound()
	blkHash := s.timeManager.LastInitializedBlockHash()

	return &TicketExpirationParams{
		CreationRound:          round.Int64(),
		CreationRoundBlockHash: blkHash,
	}
}

func (s *sender) loadSession(sessionID string) (*session, error) {
	tempSession, ok := s.sessions.Load(sessionID)
	if !ok {
		return nil, errors.Errorf("error loading session: %x", sessionID)
	}

	return tempSession.(*session), nil
}
