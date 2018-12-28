package eth

import (
	"math/big"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/livepeer/go-livepeer/pm"
)

// FundAndApproveSigners funds a sender's deposit and penalty escrow in addition
// to approving a set of ETH addresses to sign on behalf of the sender
// value to the sum of the provided deposit and penalty escrow amount
// This method wraps the underlying contract method in order to set the transaction options
// value to the sum of the provided deposit and penalty escrow amounts
func (c *client) FundAndApproveSigners(depositAmount, penaltyEscrowAmount *big.Int, signers []ethcommon.Address) (*types.Transaction, error) {
	opts := c.LivepeerETHTicketBrokerSession.TransactOpts
	opts.Value = new(big.Int).Add(depositAmount, penaltyEscrowAmount)

	return c.LivepeerETHTicketBrokerSession.Contract.FundAndApproveSigners(&opts, depositAmount, penaltyEscrowAmount, signers)
}

// FundDeposit funds a sender's deposit
// This method wraps the underlying contract method in order to set the transaction options
// value to the provided deposit amount
func (c *client) FundDeposit(amount *big.Int) (*types.Transaction, error) {
	opts := c.LivepeerETHTicketBrokerSession.TransactOpts
	opts.Value = amount

	return c.LivepeerETHTicketBrokerSession.Contract.FundDeposit(&opts)
}

// FundPenaltyEscrow funds a sender's penalty escrow
// This method wraps the underlying contract method in order to set the transaction options
// value to the provided penalty escrow amount
func (c *client) FundPenaltyEscrow(amount *big.Int) (*types.Transaction, error) {
	opts := c.LivepeerETHTicketBrokerSession.TransactOpts
	opts.Value = amount

	return c.LivepeerETHTicketBrokerSession.Contract.FundPenaltyEscrow(&opts)
}

// RedeemWinningTicket submits a ticket to be validated by the broker and if a valid winning ticket
// the broker pays the ticket's face value to the ticket's recipient
func (c *client) RedeemWinningTicket(ticket *pm.Ticket, sig []byte, recipientRand *big.Int) (*types.Transaction, error) {
	var recipientRandHash [32]byte
	copy(recipientRandHash[:], ticket.RecipientRandHash.Bytes()[:32])

	// Pass in destructured ticket fields in order to conform to the contract method signature
	return c.LivepeerETHTicketBrokerSession.RedeemWinningTicket(
		ticket.Recipient,
		ticket.Sender,
		ticket.FaceValue,
		ticket.WinProb,
		new(big.Int).SetUint64(uint64(ticket.SenderNonce)),
		recipientRandHash,
		[]byte{}, // Ignoring auxData right now since expiration is not implemented
		sig,
		recipientRand,
	)
}

// IsUsedTicket checks if a ticket has been used
// This method wraps the underlying contract method UsedTickets to allow callers to pass in
// a ticket object
func (c *client) IsUsedTicket(ticket *pm.Ticket) (bool, error) {
	var ticketHash [32]byte
	copy(ticketHash[:], ticket.Hash().Bytes()[:32])

	return c.LivepeerETHTicketBrokerSession.UsedTickets(ticketHash)
}

// ApproveSigners approves a set of ETH addresses to sign on behalf of the sender
func (c *client) ApproveSigners(signers []ethcommon.Address) (*types.Transaction, error) {
	return c.LivepeerETHTicketBrokerSession.ApproveSigners(signers)
}
