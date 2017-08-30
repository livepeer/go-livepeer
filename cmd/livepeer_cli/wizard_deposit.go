package main

import (
	"bytes"
	"fmt"
	"net/http"
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

	body := bytes.NewBufferString(val.Encode())
	rsp, err := http.Post(fmt.Sprintf("http://%v:%v/deposit", w.host, w.httpPort), "application/x-www-form-urlencoded", body)
	if err != nil {
		panic(err)
	}
	defer rsp.Body.Close()
}
