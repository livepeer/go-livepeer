package pm

import (
	"math/big"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

// ReserveState represents the state of a reserve
type ReserveState uint8

const (
	// NotFrozen is the state when the reserve is not frozen
	NotFrozen ReserveState = iota

	// Frozen is the state when the reserve has been frozen but not yet thawed
	Frozen

	// Thawed is the state when the reserve was frozen and is now thawed (i.e. the freeze period is over)
	Thawed
)

// SenderInfo contains information about a sender tracked by a Broker
type SenderInfo struct {
	// Deposit is the amount of funds the sender has in its deposit
	Deposit *big.Int

	// WithdrawBlock is the block that the sender can withdraw its deposit and reserve if
	// the reserve has not been frozen
	WithdrawBlock *big.Int

	// Reserve is the amount of funds the sender has in its reserve
	Reserve *big.Int

	// ReserveState is the state of the sender's reserve
	ReserveState ReserveState

	// ThawRound is the round that the sender can withdraw its deposit and reserve if
	// the reserve has been frozen
	ThawRound *big.Int
}

// Broker is an interface which serves as an abstraction over an on-chain
// smart contract that handles the administrative tasks in a probabilistic micropayment protocol
// including processing deposits and pay outs
type Broker interface {
	// FundDepositAndReserve funds a sender's deposit and reserve
	FundDepositAndReserve(depositAmount, reserveAmount *big.Int) (*types.Transaction, error)

	// FundDeposit funds a sender's deposit
	FundDeposit(amount *big.Int) (*types.Transaction, error)

	// FundReserve funds a sender's reserve
	FundReserve(amount *big.Int) (*types.Transaction, error)

	// Unlock initiates the unlock period for a sender after which a sender can withdraw its
	// deposit and penalty escrow
	Unlock() (*types.Transaction, error)

	// CancelUnlock stops a sender's active unlock period
	CancelUnlock() (*types.Transaction, error)

	// Withdraw credits a sender with its deposit and penalty escrow after the sender
	// waits through the unlock period
	Withdraw() (*types.Transaction, error)

	// RedeemWinningTicket submits a ticket to be validated by the broker and if a valid winning ticket
	// the broker pays the ticket's face value to the ticket's recipient
	RedeemWinningTicket(ticket *Ticket, sig []byte, recipientRand *big.Int) (*types.Transaction, error)

	// IsUsedTicket checks if a ticket has been used
	IsUsedTicket(ticket *Ticket) (bool, error)

	// GetSenderInfo returns a sender's information
	GetSenderInfo(addr ethcommon.Address) (*SenderInfo, error)
}
