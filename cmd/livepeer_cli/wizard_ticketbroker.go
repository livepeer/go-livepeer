package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math/big"
	"net/http"
	"net/url"

	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/eth"
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

	amount := w.readFloatAndValidate(func(in float64) (float64, error) {
		amountWei := eth.ToBaseUnit(big.NewFloat(in))

		if amountWei.Cmp(reqPenaltyEscrowAmount) < 0 {
			return 0, fmt.Errorf("must deposit at least %v to cover the minimum penalty escrow", eth.FormatUnits(reqPenaltyEscrowAmount, "ETH"))
		}

		return in, nil
	})

	form := url.Values{
		"amount": {eth.ToBaseUnit(big.NewFloat(amount)).String()},
	}
	fmt.Println(httpPostWithParams(fmt.Sprintf("http://%v:%v/fundAndApproveSigners", w.host, w.httpPort), form))
}

func (w *wizard) fundDeposit(currDeposit *big.Int) {
	fmt.Printf("Current Deposit: %v\n", currDeposit)

	fmt.Printf("Enter deposit amount in ETH - ")

	amount := w.readFloat()

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

	if sender.WithdrawBlock.Cmp(big.NewInt(0)) > 0 {
		fmt.Println("Already in the unlock period")
		return
	}

	fmt.Printf("Current Deposit: %v\n", eth.FormatUnits(sender.Deposit, "ETH"))
	fmt.Printf("Current Penalty Escrow: %v\n", eth.FormatUnits(sender.PenaltyEscrow, "ETH"))

	fmt.Printf("Would you like initiate the unlock period? (y/n) - ")

	input := w.readStringAndValidate(func(in string) (string, error) {
		if in != "y" && in != "n" {
			return "", errors.New("Enter (y)es or (n)o")
		}

		return in, nil
	})
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

	if sender.WithdrawBlock.Cmp(big.NewInt(0)) <= 0 {
		fmt.Println("Unlock period has not been initiated")
		return
	}

	fmt.Printf("Withdraw Block: %v\n", sender.WithdrawBlock)

	fmt.Printf("Would you like to cancel the unlock period? (y/n) - ")

	input := w.readStringAndValidate(func(in string) (string, error) {
		if in != "y" && in != "n" {
			return "", errors.New("Enter (y)es or (n)o")
		}

		return in, nil
	})
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

	if sender.WithdrawBlock.Cmp(big.NewInt(0)) <= 0 {
		fmt.Println("Unlock period has not been initiated")
		return
	}

	fmt.Printf("Would you like to withdraw? (y/n) - ")

	input := w.readStringAndValidate(func(in string) (string, error) {
		if in != "y" && in != "n" {
			return "", errors.New("Enter (y)es or (n)o")
		}

		return in, nil
	})
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
