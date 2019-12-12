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
	"github.com/livepeer/go-livepeer/pm"
)

// SenderStatus represents a sender's current status
type SenderStatus int

const (
	// Empty is a sender's status when it has no deposit or reserve
	Empty SenderStatus = iota

	// Locked is a sender's status when it has a deposit or reserve and has not initiated
	// the unlock period
	Locked

	// Unlocking is a sender's status when it has initiated and is still in the unlock period
	Unlocking

	// Unlocked is a sender's status when it is no longer in the unlocking period, but it still
	// has a deposit or reserve
	Unlocked
)

const (
	senderEmptyStatusMsg     = "sender's deposit and reserve are zero"
	senderLockedStatusMsg    = "sender's deposit and reserve are locked"
	senderUnlockingStatusMsg = "sender is in the unlocking period"
	senderUnlockedStatusMsg  = "sender's deposit and reserve are unlocked"
)

func (w *wizard) deposit() {
	sender, err := w.senderInfo()
	if err != nil {
		glog.Errorf("Error getting sender info: %v", err)
		return
	}

	fmt.Printf("Current Deposit: %v\n", eth.FormatUnits(sender.Deposit, "ETH"))
	fmt.Printf("Current Reserve: %v\n", eth.FormatUnits(sender.Reserve.FundsRemaining, "ETH"))

	fmt.Printf("Enter deposit amount in ETH - ")
	depositAmount := w.readPositiveBaseAmount()

	fmt.Printf("Enter reserve amount in ETH - ")
	reserveAmount := w.readPositiveBaseAmount()

	form := url.Values{
		"depositAmount": {depositAmount.String()},
		"reserveAmount": {reserveAmount.String()},
	}
	fmt.Println(httpPostWithParams(fmt.Sprintf("http://%v:%v/fundDepositAndReserve", w.host, w.httpPort), form))

	return
}

func (w *wizard) unlock() {
	sender, err := w.senderInfo()
	if err != nil {
		glog.Errorf("Error getting sender info: %v", err)
		return
	}

	round, err := w.currentRound()
	if err != nil {
		glog.Errorf("Error getting current round: %v", err)
		return
	}

	fmt.Printf("Current Deposit: %v\n", eth.FormatUnits(sender.Deposit, "ETH"))
	fmt.Printf("Current Reserve: %v\n", eth.FormatUnits(sender.Reserve.FundsRemaining, "ETH"))

	fmt.Printf("Current Round: %v\n", round)

	ss := senderStatus(sender, round)
	if ss != Locked {
		printSenderStatus("Cannot unlock because sender's deposit and reserve are not locked", ss)
		return
	}

	params, err := w.ticketBrokerParams()
	if err != nil {
		glog.Errorf("Error getting TicketBroker params: %v", err)
		return
	}

	projWithdrawRound := new(big.Int).Add(round, params.UnlockPeriod)

	fmt.Printf("If you initiate the unlock period now, you will be able to withdraw at round %v\n", projWithdrawRound)
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

	round, err := w.currentRound()
	if err != nil {
		glog.Errorf("Error getting current round: %v", err)
		return
	}

	fmt.Printf("Current Deposit: %v\n", eth.FormatUnits(sender.Deposit, "ETH"))
	fmt.Printf("Current Reserve: %v\n", eth.FormatUnits(sender.Reserve.FundsRemaining, "ETH"))
	fmt.Printf("Current Round: %v\n", round)
	fmt.Printf("Withdraw Round: %v\n", sender.WithdrawRound)

	ss := senderStatus(sender, round)
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

	fmt.Printf("Current Deposit: %v\n", eth.FormatUnits(sender.Deposit, "ETH"))
	fmt.Printf("Current Reserve: %v\n", eth.FormatUnits(sender.Reserve.FundsRemaining, "ETH"))

	round, err := w.currentRound()
	if err != nil {
		glog.Errorf("Error getting current round: %v", err)
		return
	}

	fmt.Printf("Current Round: %v\n", round)
	fmt.Printf("Withdraw Round: %v\n", sender.WithdrawRound)

	ss := senderStatus(sender, round)
	if ss != Unlocked {
		printSenderStatus("Cannot withdraw because sender's deposit and reserve are not unlocked", ss)
		return
	}

	fmt.Printf("Would you like to withdraw? (y/n) - ")

	input := w.readStringYesOrNo()
	if input == "n" {
		return
	}

	fmt.Println(httpPost(fmt.Sprintf("http://%v:%v/withdraw", w.host, w.httpPort)))
}

func (w *wizard) senderInfo() (info pm.SenderInfo, err error) {
	var resp *http.Response
	resp, err = http.Get(fmt.Sprintf("http://%v:%v/senderInfo", w.host, w.httpPort))
	if err != nil {
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusInternalServerError {
		// node is in offchain mode
		info.Deposit = big.NewInt(0)
		info.WithdrawRound = big.NewInt(0)
		info.Reserve = &pm.ReserveInfo{
			FundsRemaining:        big.NewInt(0),
			ClaimedInCurrentRound: big.NewInt(0),
		}
		return
	}

	var res []byte
	res, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	err = json.Unmarshal(res, &info)
	if err != nil {
		return
	}

	return
}

func (w *wizard) ticketBrokerParams() (params struct {
	UnlockPeriod *big.Int
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

func senderStatus(sender pm.SenderInfo, currentRound *big.Int) SenderStatus {
	if sender.Deposit.Cmp(big.NewInt(0)) == 0 && sender.Reserve.FundsRemaining.Cmp(big.NewInt(0)) == 0 {
		return Empty
	}

	if sender.WithdrawRound.Cmp(big.NewInt(0)) > 0 {
		if sender.WithdrawRound.Cmp(currentRound) <= 0 {
			return Unlocked
		}

		return Unlocking
	}

	return Locked
}
