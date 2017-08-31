package main

import (
	"fmt"
	"net/url"
)

func (w *wizard) deposit() {
	fmt.Printf("Current deposit: %v\n", w.getDeposit())
	fmt.Printf("Current token balance: %v\n", w.getTokenBalance())
	fmt.Printf("Enter Deposit Amount - ")
	amount := w.readInt()

	val := url.Values{
		"amount": {fmt.Sprintf("%v", amount)},
	}

	httpPostWithParams(fmt.Sprintf("http://%v:%v/deposit", w.host, w.httpPort), val)
}
