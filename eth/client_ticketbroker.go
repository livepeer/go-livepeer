package eth

import (
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/livepeer/go-livepeer/eth/contracts"
	"github.com/livepeer/go-livepeer/pm"
)

// FundDepositAndReserve funds a sender's deposit and reserve
// This method wraps the underlying contract method in order to set the transaction options
// value to the sum of the provided deposit and penalty escrow amounts
func (c *client) FundDepositAndReserve(depositAmount, reserveAmount *big.Int) (*types.Transaction, error) {
	opts := c.TicketBrokerSession.TransactOpts
	opts.Value = new(big.Int).Add(depositAmount, reserveAmount)

	return c.TicketBrokerSession.Contract.FundDepositAndReserve(&opts, depositAmount, reserveAmount)
}

// FundDeposit funds a sender's deposit
// This method wraps the underlying contract method in order to set the transaction options
// value to the provided deposit amount
func (c *client) FundDeposit(amount *big.Int) (*types.Transaction, error) {
	opts := c.TicketBrokerSession.TransactOpts
	opts.Value = amount

	return c.TicketBrokerSession.Contract.FundDeposit(&opts)
}

// FundReserve funds a sender's reserve
// This method wraps the underlying contract method in order to set the transaction options
// value to the provided reserve amount
func (c *client) FundReserve(amount *big.Int) (*types.Transaction, error) {
	opts := c.TicketBrokerSession.TransactOpts
	opts.Value = amount

	return c.TicketBrokerSession.Contract.FundReserve(&opts)
}

// RedeemWinningTicket submits a ticket to be validated by the broker and if a valid winning ticket
// the broker pays the ticket's face value to the ticket's recipient
func (c *client) RedeemWinningTicket(ticket *pm.Ticket, sig []byte, recipientRand *big.Int) (*types.Transaction, error) {
	var recipientRandHash [32]byte
	copy(recipientRandHash[:], ticket.RecipientRandHash.Bytes()[:32])

	return c.TicketBrokerSession.RedeemWinningTicket(
		contracts.Struct1{
			Recipient:         ticket.Recipient,
			Sender:            ticket.Sender,
			FaceValue:         ticket.FaceValue,
			WinProb:           ticket.WinProb,
			SenderNonce:       new(big.Int).SetUint64(uint64(ticket.SenderNonce)),
			RecipientRandHash: recipientRandHash,
			AuxData:           ticket.AuxData(),
		},
		sig,
		recipientRand,
	)
}

// GetSenderInfo returns the info for a sender
func (c *client) GetSenderInfo(addr ethcommon.Address) (*pm.SenderInfo, error) {
	info := new(struct {
		Sender struct {
			Deposit       *big.Int
			WithdrawRound *big.Int
		}
		Reserve pm.ReserveInfo
	})

	abi, err := abi.JSON(strings.NewReader(contracts.TicketBrokerABI))
	if err != nil {
		return nil, err
	}

	contract := bind.NewBoundContract(c.ticketBrokerAddr, abi, c.backend, c.backend, c.backend)
	if err := contract.Call(&c.TicketBrokerSession.CallOpts, info, "getSenderInfo", addr); err != nil {
		return nil, err
	}

	return &pm.SenderInfo{
		Deposit:       info.Sender.Deposit,
		WithdrawRound: info.Sender.WithdrawRound,
		Reserve: &pm.ReserveInfo{
			FundsRemaining:        info.Reserve.FundsRemaining,
			ClaimedInCurrentRound: info.Reserve.ClaimedInCurrentRound,
		},
	}, nil
}

// IsUsedTicket checks if a ticket has been used
// This method wraps the underlying contract method UsedTickets to allow callers to pass in
// a ticket object
func (c *client) IsUsedTicket(ticket *pm.Ticket) (bool, error) {
	var ticketHash [32]byte
	copy(ticketHash[:], ticket.Hash().Bytes()[:32])

	return c.TicketBrokerSession.UsedTickets(ticketHash)
}
