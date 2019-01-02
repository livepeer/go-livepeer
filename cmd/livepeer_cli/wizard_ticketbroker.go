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
	httpPostWithParams(fmt.Sprintf("http://%v:%v/fundAndApproveSigners", w.host, w.httpPort), form)
}

func (w *wizard) fundDeposit(currDeposit *big.Int) {
	fmt.Printf("Current Deposit: %v\n", currDeposit)

	fmt.Printf("Enter deposit amount in ETH - ")

	amount := w.readFloat()

	form := url.Values{
		"amount": {eth.ToBaseUnit(big.NewFloat(amount)).String()},
	}
	httpPostWithParams(fmt.Sprintf("http://%v:%v/fundDeposit", w.host, w.httpPort), form)
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
