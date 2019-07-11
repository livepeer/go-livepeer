package pm

import (
	"math/big"
	"sync"
	"sync/atomic"

	"github.com/pkg/errors"
)

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
	signer        Signer
	roundsManager RoundsManager
	senderManager SenderManager

	maxEV             *big.Rat
	depositMultiplier int

	sessions sync.Map
}

// NewSender creates a new Sender instance.
func NewSender(signer Signer, roundsManager RoundsManager, senderManager SenderManager, maxEV *big.Rat, depositMultiplier int) Sender {
	return &sender{
		signer:            signer,
		roundsManager:     roundsManager,
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

// CreateTicketBatch returns a ticket batch of the specified size
func (s *sender) CreateTicketBatch(sessionID string, size int) (*TicketBatch, error) {
	session, err := s.loadSession(sessionID)
	if err != nil {
		return nil, err
	}

	if err := s.ValidateTicketParams(&session.ticketParams); err != nil {
		return nil, err
	}

	expirationParams, err := s.expirationParams()
	if err != nil {
		return nil, err
	}

	batch := &TicketBatch{
		TicketParams:           &session.ticketParams,
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
	ev := ticketEV(ticketParams.FaceValue, ticketParams.WinProb)
	if ev.Cmp(s.maxEV) > 0 {
		return errors.Errorf("ticket EV higher than max EV")
	}

	info, err := s.senderManager.GetSenderInfo(s.signer.Account().Address)
	if err != nil {
		return err
	}

	maxFaceValue := new(big.Int).Div(info.Deposit, big.NewInt(int64(s.depositMultiplier)))
	if ticketParams.FaceValue.Cmp(maxFaceValue) > 0 {
		return errors.Errorf("ticket faceValue higher than max faceValue")
	}

	return nil
}

func (s *sender) expirationParams() (*TicketExpirationParams, error) {
	round, err := s.roundsManager.LastInitializedRound()
	if err != nil {
		return nil, err
	}

	blkHash, err := s.roundsManager.BlockHashForRound(round)
	if err != nil {
		return nil, err
	}

	return &TicketExpirationParams{
		CreationRound:          round.Int64(),
		CreationRoundBlockHash: blkHash,
	}, nil
}

func (s *sender) loadSession(sessionID string) (*session, error) {
	tempSession, ok := s.sessions.Load(sessionID)
	if !ok {
		return nil, errors.Errorf("error loading session: %x", sessionID)
	}

	return tempSession.(*session), nil
}
