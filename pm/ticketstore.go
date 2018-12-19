package pm

import (
	"math/big"

	ethcommon "github.com/ethereum/go-ethereum/common"
)

// TicketStore is an interface which describes an object capable
// of persisting tickets
type TicketStore interface {
	// Store persists a ticket with its signature and recipientRand
	Store(ticket *Ticket, sig []byte, recipientRand *big.Int) error

	// Load fetches a persisted ticket in the store with its signature and recipientRand
	Load(ticketID ethcommon.Hash) (ticket *Ticket, sig []byte, recipientRand *big.Int, err error)
}
