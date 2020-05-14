package pm

import ethcommon "github.com/ethereum/go-ethereum/common"

// TicketStore is an interface which describes an object capable
// of persisting tickets
type TicketStore interface {
	SelectEarliestWinningTicket(sender ethcommon.Address) (*SignedTicket, error)

	// RemoveWinningTicket removes a ticket from the TicketStore
	RemoveWinningTicket(ticket *SignedTicket) error

	// Store persists a signed winning ticket
	StoreWinningTicket(ticket *SignedTicket) error

	// MarkWinningTicketRedeemed stores the submission time and transaction hash
	MarkWinningTicketRedeemed(ticket *SignedTicket, txHash ethcommon.Hash) error

	WinningTicketCount(sender ethcommon.Address) (int, error)
}
