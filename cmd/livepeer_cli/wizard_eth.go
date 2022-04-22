package main

import (
	"fmt"
	"net/url"
)

func (w *wizard) setMaxGasPrice() {
	fmt.Printf("Current maximum gas price: %v\n", w.maxGasPrice())
	amount := w.readBigInt("Enter new maximum gas price in Wei (enter \"0\" for no maximum gas price)")

	val := url.Values{
		"amount": {fmt.Sprintf("%v", amount.String())},
	}

	httpPostWithParams(fmt.Sprintf("http://%v:%v/setMaxGasPrice", w.host, w.httpPort), val)
}

func (w *wizard) setMinGasPrice() {
	fmt.Printf("Current minimum gas price: %v\n", w.minGasPrice())
	minGasPrice := w.readBigInt("Enter new minimum gas price in Wei")

	val := url.Values{
		"minGasPrice": {fmt.Sprintf("%v", minGasPrice.String())},
	}

	httpPostWithParams(fmt.Sprintf("http://%v:%v/setMinGasPrice", w.host, w.httpPort), val)
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
		return
	}
	fmt.Println(fmt.Sprintf("\n\nSignature:\n0x%x", result))
}

func (w *wizard) signTypedData() {
	fmt.Printf("Enter or paste the typed data to sign: \n")
	msg := w.readMultilineString()
	val := url.Values{
		"message": {msg},
	}
	headers := map[string]string{
		"SigFormat": "data/typed",
	}
	result, ok := httpPostWithParamsHeaders(fmt.Sprintf("http://%v:%v/signMessage", w.host, w.httpPort), val, headers)
	if !ok {
		fmt.Printf("Error signing typed data: %v\n", result)
		return
	}
	fmt.Println(fmt.Sprintf("\n\nSignature:\n0x%x", result))
}
