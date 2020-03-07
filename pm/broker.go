package pm

import (
	"math/big"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
)

// SenderInfo contains information about a sender tracked by a Broker
type SenderInfo struct {
	// Deposit is the amount of funds the sender has in its deposit
	Deposit *big.Int

	// WithdrawRound is the round that the sender can withdraw its deposit and reserve
	WithdrawRound *big.Int

	// ReserveInfo is a struct containing details about a sender's reserve
	Reserve *ReserveInfo
}

// ReserveInfo holds information about a sender's reserve
type ReserveInfo struct {
	// FundsRemaining is the amount of funds the sender has left in its reserve
	FundsRemaining *big.Int

	// ClaimedInCurrentRound is the total amount of funds claimed from the sender's reserve in the current round
	ClaimedInCurrentRound *big.Int
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

	// CheckTx waits for a transaction to confirm on-chain and returns an error
	// if the transaction failed
	CheckTx(tx *types.Transaction) error
}

// TimeManager defines the methods for fetching the last
// initialized round and associated block hash of the Livepeer protocol
type TimeManager interface {
	// LastInitializedRound returns the last initialized round of the Livepeer protocol
	LastInitializedRound() *big.Int
	// LastInitializedBlockHash returns the blockhash of the block the last round was initiated in
	LastInitializedBlockHash() [32]byte
	// GetTranscoderPoolSize returns the size of the active transcoder set for a round
	GetTranscoderPoolSize() *big.Int
	// LastSeenBlock returns the last seen block number
	LastSeenBlock() *big.Int
	// SubscribeRounds allows one to subscribe to new round events
	SubscribeRounds(sink chan<- types.Log) event.Subscription
	// SubscribeBlocks allows one to subscribe to newly seen block numbers
	SubscribeBlocks(sink chan<- *big.Int) event.Subscription
}

// SenderManager defines the methods for fetching sender information
type SenderManager interface {
	// GetSenderInfo returns a sender's information
	GetSenderInfo(addr ethcommon.Address) (*SenderInfo, error)
	// ClaimedReserve returns the amount claimed from a sender's reserve
	ClaimedReserve(reserveHolder ethcommon.Address, claimant ethcommon.Address) (*big.Int, error)
	// Clear clears the cached values for a sender
	Clear(addr ethcommon.Address)
}
