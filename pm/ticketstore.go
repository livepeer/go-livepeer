package pm

import (
	ethcommon "github.com/ethereum/go-ethereum/common"
)

// TicketStore is an interface which describes an object capable
// of persisting tickets
type TicketStore interface {
	// SelectEarliestWinningTicket selects the earliest stored winning ticket for a 'sender'
	// which is not yet redeemed
	SelectEarliestWinningTicket(sender ethcommon.Address, minCreationRound int64) (*SignedTicket, error)

	// RemoveWinningTicket removes a ticket
	RemoveWinningTicket(ticket *SignedTicket) error

	// StoreWinningTicket stores a signed ticket
	StoreWinningTicket(ticket *SignedTicket) error

	// MarkWinningTicketRedeemed stores the on-chain transaction hash and timestamp of redemption
	// This marks the ticket as being 'redeemed'
	MarkWinningTicketRedeemed(ticket *SignedTicket, txHash ethcommon.Hash) error

	// WinningTicketCount returns the amount of non-redeemed winning tickets for a sender in the TicketStore
	WinningTicketCount(sender ethcommon.Address, minCreationRound int64) (int, error)
}
