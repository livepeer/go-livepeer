package pm

import (
	"math/big"
	"sync"
	"sync/atomic"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/livepeer/go-livepeer/eth"
	"github.com/pkg/errors"
)

type Sender interface {
	StartSession(recipient ethcommon.Address, ticketParams TicketParams) string

	CreateTicket(sessionID string) (*Ticket, *big.Int, []byte, error)

	// Later: support receiving new recipientRandHash values mid-stream
}

type Session struct {
	senderNonce uint64

	recipient ethcommon.Address

	ticketParams TicketParams
}

type DefaultSender struct {
	accountManager eth.AccountManager

	sessions sync.Map
}

func NewSender(accountManager eth.AccountManager) Sender {
	return &DefaultSender{
		accountManager: accountManager,
	}
}

func (s *DefaultSender) StartSession(recipient ethcommon.Address, ticketParams TicketParams) string {
	sessionID := hashToHex(ticketParams.RecipientRandHash)

	s.sessions.Store(sessionID, &Session{
		recipient:    recipient,
		ticketParams: ticketParams,
		senderNonce:  0,
	})

	return sessionID
}

func (s *DefaultSender) CreateTicket(sessionID string) (*Ticket, *big.Int, []byte, error) {
	recipientRandHash := hexToHash(sessionID)

	tempSession, ok := s.sessions.Load(sessionID)
	if !ok {
		return nil, nil, nil, errors.Errorf("cannot create a ticket for an unknown session: %+v", sessionID)
	}
	session := tempSession.(*Session)

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
