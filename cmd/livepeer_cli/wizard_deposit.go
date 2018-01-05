package main

import (
	"fmt"
	"net/url"
)

func (w *wizard) deposit() {
	fmt.Printf("Current deposit: %v\n", w.getDeposit())
	fmt.Printf("Current token balance: %v\n", w.getTokenBalance())
	fmt.Printf("Enter Deposit Amount - ")
	amount := w.readBigInt()

	val := url.Values{
		"amount": {fmt.Sprintf("%v", amount.String())},
	}

	httpPostWithParams(fmt.Sprintf("http://%v:%v/deposit", w.host, w.httpPort), val)
}

func (w *wizard) withdraw() {
	fmt.Printf("Current deposit: %v\n", w.getDeposit())

	httpPost(fmt.Sprintf("http://%v:%v/withdraw", w.host, w.httpPort))
}
