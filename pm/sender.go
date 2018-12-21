package pm

import (
	"math/big"
	"sync"
	"sync/atomic"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"
)

type Sender interface {
	StartSession(recipient ethcommon.Address, ticketParams TicketParams) string

	CreateTicket(sessionId string) (*Ticket, *big.Int, []byte, error)

	// Later: support receiving new recipientRandHash values mid-stream
}

type Session struct {
	senderNonce uint64

	recipient ethcommon.Address

	ticketParams TicketParams
}

type DefaultSender struct {
	address ethcommon.Address

	sessions sync.Map
}

func NewSender(address ethcommon.Address) Sender {
	return &DefaultSender{
		address: address,
	}
}

func (s *DefaultSender) StartSession(recipient ethcommon.Address, ticketParams TicketParams) string {
	sessionId := hashToHex(ticketParams.RecipientRandHash)

	s.sessions.Store(sessionId, &Session{
		recipient:    recipient,
		ticketParams: ticketParams,
		senderNonce:  0,
	})

	return sessionId
}

func (s *DefaultSender) CreateTicket(sessionId string) (*Ticket, *big.Int, []byte, error) {
	recipientRandHash := hexToHash(sessionId)

	tempSession, ok := s.sessions.Load(sessionId)
	if !ok {
		return nil, nil, nil, errors.Errorf("cannot create a ticket for an unknown session: %+v", sessionId)
	}
	session := tempSession.(*Session)

	senderNonce := atomic.AddUint64(&session.senderNonce, 1)

	ticket := &Ticket{
		Recipient:         session.recipient,
		RecipientRandHash: recipientRandHash,
		Sender:            s.address,
		SenderNonce:       senderNonce,
		FaceValue:         session.ticketParams.FaceValue,
		WinProb:           session.ticketParams.WinProb,
	}

	// TODO sign!

	return ticket, session.ticketParams.Seed, nil, nil
}
