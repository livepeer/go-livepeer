package main

import (
	"fmt"
	"math/big"
	"net/url"

	"github.com/livepeer/go-livepeer/eth"
)

func str2eth(v string) string {
	i, ok := big.NewInt(0).SetString(v, 10)
	if !ok {
		fmt.Printf("Could not convert %v to bigint", v)
		return ""
	}
	return eth.FormatUnits(i, "ETH")
}

func (w *wizard) deposit() {
	fmt.Printf("Current deposit: %v\n", str2eth(w.getDeposit()))
	fmt.Printf("Current balance: %v\n", str2eth(w.getEthBalance()))
	fmt.Printf("Enter Deposit Amount in Wei - ")
	amount := w.readBigInt()

	val := url.Values{
		"amount": {fmt.Sprintf("%v", amount.String())},
	}

	httpPostWithParams(fmt.Sprintf("http://%v:%v/deposit", w.host, w.httpPort), val)
}

func (w *wizard) withdraw() {
	// We don't run str2eth here to facilitate copy-pasting
	fmt.Printf("Current deposit in Wei: %v\n", w.getDeposit())

	fmt.Println(httpPost(fmt.Sprintf("http://%v:%v/withdrawDeposit", w.host, w.httpPort)))
}
