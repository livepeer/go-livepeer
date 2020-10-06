package main

import (
	"fmt"
	"net/url"
)

func (w *wizard) setGasPrice() {
	fmt.Printf("Current gas price: %v\n", w.getGasPrice())
	fmt.Printf("Enter new gas price in Wei (enter \"0\" for automatic)")
	amount := w.readBigInt()

	val := url.Values{
		"amount": {fmt.Sprintf("%v", amount.String())},
	}

	httpPostWithParams(fmt.Sprintf("http://%v:%v/setGasPrice", w.host, w.httpPort), val)
}

func (w *wizard) signMessage() {
	fmt.Printf("Enter or paste the message to sign: \n")
	msg := w.readMultilineString()
	val := url.Values{
		"message": {msg},
	}
	result, ok := httpPostWithParams(fmt.Sprintf("http://%v:%v/signMessage", w.host, w.httpPort), val)
	if !ok {
		fmt.Printf("Error signing message: %v\n", result)
	}
	fmt.Println(fmt.Sprintf("\n\nSignature:\n0x%x", result))
}
