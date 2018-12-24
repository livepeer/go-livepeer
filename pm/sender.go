package pm

import (
	"math/big"
	"sync"
	"sync/atomic"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/livepeer/go-livepeer/eth"
	"github.com/pkg/errors"
)

// Sender enables starting multiple probabilistic micropayment sessions with multiple recipients
// and create tickets that adhere to each session's params and unique nonce requirements.
type Sender interface {
	StartSession(recipient ethcommon.Address, ticketParams TicketParams) string

	CreateTicket(sessionID string) (*Ticket, *big.Int, []byte, error)

	// Later: support receiving new recipientRandHash values mid-stream
}

type session struct {
	senderNonce uint64

	recipient ethcommon.Address

	ticketParams TicketParams
}

type defaultSender struct {
	accountManager eth.AccountManager

	sessions sync.Map
}

// NewSender creates a new Sender instance.
func NewSender(accountManager eth.AccountManager) Sender {
	return &defaultSender{
		accountManager: accountManager,
	}
}

func (s *defaultSender) StartSession(recipient ethcommon.Address, ticketParams TicketParams) string {
	sessionID := ticketParams.RecipientRandHash.Hex()

	s.sessions.Store(sessionID, &session{
		recipient:    recipient,
		ticketParams: ticketParams,
		senderNonce:  0,
	})

	return sessionID
}

func (s *defaultSender) CreateTicket(sessionID string) (*Ticket, *big.Int, []byte, error) {
	recipientRandHash := hexToHash(sessionID)

	tempSession, ok := s.sessions.Load(sessionID)
	if !ok {
		return nil, nil, nil, errors.Errorf("cannot create a ticket for an unknown session: %+v", sessionID)
	}
	session := tempSession.(*session)

	senderNonce := atomic.AddUint64(&session.senderNonce, 1)

	ticket := &Ticket{
		Recipient:         session.recipient,
		RecipientRandHash: recipientRandHash,
		Sender:            s.accountManager.Account().Address,
		SenderNonce:       senderNonce,
		FaceValue:         session.ticketParams.FaceValue,
		WinProb:           session.ticketParams.WinProb,
	}

	sig, err := s.accountManager.Sign(ticket.Hash().Bytes())
	if err != nil {
		return nil, nil, nil, errors.Wrapf(err, "error signing ticket for session: %v", sessionID)
	}

	return ticket, session.ticketParams.Seed, sig, nil
}
