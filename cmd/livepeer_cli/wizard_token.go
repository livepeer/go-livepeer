package main

import (
	"fmt"
	"net/url"
)

func (w *wizard) transferTokens() {
	fmt.Printf("Current LPT balance: %v\n", w.getTokenBalance())

	fmt.Printf("Enter receipient address (in hex i.e. 0xfoo) - ")
	to := w.readString()

	fmt.Printf("Enter amount - ")
	amount := w.readBigInt()

	val := url.Values{
		"to":     {fmt.Sprintf("%v", to)},
		"amount": {fmt.Sprintf("%v", amount.String())},
	}

	httpPostWithParams(fmt.Sprintf("http://%v:%v/transferTokens", w.host, w.httpPort), val)
}

func (w *wizard) requestTokens() {
	httpPost(fmt.Sprintf("http://%v:%v/requestTokens", w.host, w.httpPort))
}
