package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/big"
	"net/http"
	"net/url"

	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/eth"
)

// SenderStatus represents a sender's current status
type SenderStatus int

const (
	// Empty is a sender's status when it has no deposit or penalty escrow
	Empty SenderStatus = iota

	// Locked is a sender's status when it has a deposit or penalty escrow and has not initiated
	// the unlock period
	Locked

	// Unlocking is a sender's status when it has initiated and is still in the unlock period
	Unlocking

	// Unlocked is a sender's status when it is no longer in the unlocking period, but it still
	// has a deposit or penalty escrow
	Unlocked
)

const (
	senderEmptyStatusMsg     = "sender's deposit and penalty escrow are zero"
	senderLockedStatusMsg    = "sender's deposit and penalty escrow are locked"
	senderUnlockingStatusMsg = "sender is in the unlocking period"
	senderUnlockedStatusMsg  = "sender's deposit and penalty escrow are unlocked"
)

func (w *wizard) deposit() {
	sender, err := w.senderInfo()
	if err != nil {
		glog.Errorf("Error getting sender info: %v", err)
		return
	}

	params, err := w.ticketBrokerParams()
	if err != nil {
		glog.Errorf("Error getting TicketBroker params: %v", err)
		return
	}

	reqPenaltyEscrowAmount := big.NewInt(0)
	if params.MinPenaltyEscrow.Cmp(sender.PenaltyEscrow) > 0 {
		reqPenaltyEscrowAmount = new(big.Int).Sub(params.MinPenaltyEscrow, sender.PenaltyEscrow)
	}

	if reqPenaltyEscrowAmount.Cmp(big.NewInt(0)) > 0 {
		w.fundAndApproveSigners(sender.Deposit, sender.PenaltyEscrow, reqPenaltyEscrowAmount)
		return
	}

	w.fundDeposit(sender.Deposit)

	return
}

func (w *wizard) fundAndApproveSigners(currDeposit *big.Int, currPenaltyEscrow *big.Int, reqPenaltyEscrowAmount *big.Int) {
	fmt.Printf("Current Deposit: %v\n", currDeposit)
	fmt.Printf("Current Penalty Escrow: %v\n", currPenaltyEscrow)

	fmt.Printf("Enter deposit amount in ETH (%v will be used cover the minimum penalty escrow) - ", eth.FormatUnits(reqPenaltyEscrowAmount, "ETH"))

	amount := w.readPositiveFloatAndValidate(func(in float64) (float64, error) {
		amountWei := eth.ToBaseUnit(big.NewFloat(in))

		if amountWei.Cmp(reqPenaltyEscrowAmount) < 0 {
			return 0, fmt.Errorf("must deposit at least %v to cover the minimum penalty escrow", eth.FormatUnits(reqPenaltyEscrowAmount, "ETH"))
		}

		return in, nil
	})

	amountWei := eth.ToBaseUnit(big.NewFloat(amount))
	depositAmount := new(big.Int).Sub(amountWei, reqPenaltyEscrowAmount)

	form := url.Values{
		"depositAmount":       {depositAmount.String()},
		"penaltyEscrowAmount": {reqPenaltyEscrowAmount.String()},
	}
	fmt.Println(httpPostWithParams(fmt.Sprintf("http://%v:%v/fundAndApproveSigners", w.host, w.httpPort), form))
}

func (w *wizard) fundDeposit(currDeposit *big.Int) {
	fmt.Printf("Current Deposit: %v\n", eth.FormatUnits(currDeposit, "ETH"))

	fmt.Printf("Enter deposit amount in ETH - ")

	amount := w.readPositiveFloat()

	form := url.Values{
		"amount": {eth.ToBaseUnit(big.NewFloat(amount)).String()},
	}
	fmt.Println(httpPostWithParams(fmt.Sprintf("http://%v:%v/fundDeposit", w.host, w.httpPort), form))
}

func (w *wizard) unlock() {
	sender, err := w.senderInfo()
	if err != nil {
		glog.Errorf("Error getting sender info: %v", err)
		return
	}

	blk, err := w.currentBlock()
	if err != nil {
		glog.Errorf("Error getting current block: %v", err)
		return
	}

	fmt.Printf("Current Deposit: %v\n", eth.FormatUnits(sender.Deposit, "ETH"))
	fmt.Printf("Current Penalty Escrow: %v\n", eth.FormatUnits(sender.PenaltyEscrow, "ETH"))
	fmt.Printf("Current Block: %v\n", blk)

	ss := senderStatus(sender, blk)
	if ss != Locked {
		printSenderStatus("Cannot unlock because sender's deposit and penalty escrow are not locked", ss)
		return
	}

	params, err := w.ticketBrokerParams()
	if err != nil {
		glog.Errorf("Error getting TicketBroker params: %v", err)
		return
	}

	projWithdrawBlock := new(big.Int).Add(blk, params.UnlockPeriod)

	fmt.Printf("If you initiate the unlock period now, you will be able to withdraw at block %v\n", projWithdrawBlock)
	fmt.Printf("Would you like initiate the unlock period? (y/n) - ")

	input := w.readStringYesOrNo()
	if input == "n" {
		return
	}

	fmt.Println(httpPost(fmt.Sprintf("http://%v:%v/unlock", w.host, w.httpPort)))
}

func (w *wizard) cancelUnlock() {
	sender, err := w.senderInfo()
	if err != nil {
		glog.Errorf("Error getting sender info: %v", err)
		return
	}

	blk, err := w.currentBlock()
	if err != nil {
		glog.Errorf("Error getting current block: %v", err)
		return
	}

	fmt.Printf("Current Deposit: %v\n", eth.FormatUnits(sender.Deposit, "ETH"))
	fmt.Printf("Current Penalty Escrow: %v\n", eth.FormatUnits(sender.PenaltyEscrow, "ETH"))
	fmt.Printf("Current Block: %v\n", blk)
	fmt.Printf("Withdraw Block: %v\n", sender.WithdrawBlock)

	ss := senderStatus(sender, blk)
	if ss != Unlocking {
		printSenderStatus("Cannot cancel unlock because sender is not in the unlock period", ss)
		return
	}

	fmt.Printf("Would you like to cancel the unlock period? (y/n) - ")

	input := w.readStringYesOrNo()
	if input == "n" {
		return
	}

	fmt.Println(httpPost(fmt.Sprintf("http://%v:%v/cancelUnlock", w.host, w.httpPort)))
}

func (w *wizard) withdraw() {
	sender, err := w.senderInfo()
	if err != nil {
		glog.Errorf("Error getting sender info: %v", err)
		return
	}

	blk, err := w.currentBlock()
	if err != nil {
		glog.Errorf("Error getting current block: %v", err)
		return
	}

	fmt.Printf("Current Deposit: %v\n", eth.FormatUnits(sender.Deposit, "ETH"))
	fmt.Printf("Current Penalty Escrow: %v\n", eth.FormatUnits(sender.PenaltyEscrow, "ETH"))
	fmt.Printf("Current Block: %v\n", blk)
	fmt.Printf("Withdraw Block: %v\n", sender.WithdrawBlock)

	ss := senderStatus(sender, blk)
	if ss != Unlocked {
		printSenderStatus("Cannot withdraw because sender's deposit and penalty escrow are not unlocked", ss)
		return
	}

	fmt.Printf("Would you like to withdraw? (y/n) - ")

	input := w.readStringYesOrNo()
	if input == "n" {
		return
	}

	fmt.Println(httpPost(fmt.Sprintf("http://%v:%v/withdraw", w.host, w.httpPort)))
}

func (w *wizard) senderInfo() (sender struct {
	Deposit       *big.Int
	PenaltyEscrow *big.Int
	WithdrawBlock *big.Int
}, err error) {
	var resp *http.Response
	resp, err = http.Get(fmt.Sprintf("http://%v:%v/senderInfo", w.host, w.httpPort))
	if err != nil {
		return
	}
	defer resp.Body.Close()

	var res []byte
	res, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	err = json.Unmarshal(res, &sender)
	if err != nil {
		return
	}

	return
}

func (w *wizard) ticketBrokerParams() (params struct {
	MinPenaltyEscrow *big.Int
	UnlockPeriod     *big.Int
}, err error) {
	var resp *http.Response
	resp, err = http.Get(fmt.Sprintf("http://%v:%v/ticketBrokerParams", w.host, w.httpPort))
	if err != nil {
		return
	}
	defer resp.Body.Close()

	var res []byte
	res, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	err = json.Unmarshal(res, &params)
	if err != nil {
		return
	}

	return
}

func printSenderStatus(msg string, status SenderStatus) {
	var statusMsg string
	if status == Empty {
		statusMsg = senderEmptyStatusMsg
	}
	if status == Locked {
		statusMsg = senderLockedStatusMsg
	}
	if status == Unlocking {
		statusMsg = senderUnlockingStatusMsg
	}
	if status == Unlocked {
		statusMsg = senderUnlockedStatusMsg
	}

	fmt.Printf("%v: %v\n", msg, statusMsg)
}

func senderStatus(
	sender struct {
		Deposit       *big.Int
		PenaltyEscrow *big.Int
		WithdrawBlock *big.Int
	}, currentBlock *big.Int,
) SenderStatus {
	if sender.Deposit.Cmp(big.NewInt(0)) == 0 && sender.PenaltyEscrow.Cmp(big.NewInt(0)) == 0 {
		return Empty
	}

	if sender.WithdrawBlock.Cmp(big.NewInt(0)) > 0 {
		if sender.WithdrawBlock.Cmp(currentBlock) <= 0 {
			return Unlocked
		}

		return Unlocking
	}

	return Locked
}
