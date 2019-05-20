package pm

import (
	"math/big"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

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

	// Senders returns a sender's information
	Senders(addr ethcommon.Address) (struct {
		Deposit       *big.Int
		WithdrawBlock *big.Int
	}, error)

	// RemainingReserve returns a sender's remaining reserve funds
	RemainingReserve(addr ethcommon.Address) (*big.Int, error)
}
