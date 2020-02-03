package pm

import (
	"math/big"
)

// TicketStore is an interface which describes an object capable
// of persisting tickets
type TicketStore interface {
	// Store persists a ticket with its signature and recipientRand
	// for a session ID
	StoreWinningTicket(sessionID string, ticket *Ticket, sig []byte, recipientRand *big.Int) error

	// Load fetches all persisted tickets in the store with their signatures and recipientRands
	// for a session ID
	LoadWinningTickets(sessionIDs []string) (tickets []*Ticket, sigs [][]byte, recipientRands []*big.Int, err error)

	blockStore
}

type blockStore interface {
	LastSeenBlock() (*big.Int, error)
}
