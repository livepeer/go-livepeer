package pm

import (
	"math/big"
	"sync"
	"sync/atomic"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"
)

// Sender enables starting multiple probabilistic micropayment sessions with multiple recipients
// and create tickets that adhere to each session's params and unique nonce requirements.
type Sender interface {
	// StartSession creates a session for a given set of ticket params which tracks information
	// for creating new tickets
	StartSession(ticketParams TicketParams) string

	// CreateTicket returns a new ticket, seed (which the recipient can use to derive its random number),
	// and signature over the new ticket for a given session ID
	CreateTicket(sessionID string) (*Ticket, *big.Int, []byte, error)

	// Later: support receiving new recipientRandHash values mid-stream
}

type session struct {
	senderNonce uint32

	ticketParams TicketParams
}

type sender struct {
	signer Signer

	sessions sync.Map
}

// NewSender creates a new Sender instance.
func NewSender(signer Signer) Sender {
	return &sender{
		signer: signer,
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

func (s *sender) CreateTicket(sessionID string) (*Ticket, *big.Int, []byte, error) {
	recipientRandHash := ethcommon.HexToHash(sessionID)

	tempSession, ok := s.sessions.Load(sessionID)
	if !ok {
		return nil, nil, nil, errors.Errorf("cannot create a ticket for an unknown session: %v", sessionID)
	}
	session := tempSession.(*session)

	senderNonce := atomic.AddUint32(&session.senderNonce, 1)

	ticket := &Ticket{
		Recipient:         session.ticketParams.Recipient,
		RecipientRandHash: recipientRandHash,
		Sender:            s.signer.Account().Address,
		SenderNonce:       senderNonce,
		FaceValue:         session.ticketParams.FaceValue,
		WinProb:           session.ticketParams.WinProb,
	}

	sig, err := s.signer.Sign(ticket.Hash().Bytes())
	if err != nil {
		return nil, nil, nil, errors.Wrapf(err, "error signing ticket for session: %v", sessionID)
	}

	return ticket, session.ticketParams.Seed, sig, nil
}
