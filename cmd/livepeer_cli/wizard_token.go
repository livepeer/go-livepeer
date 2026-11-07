package main

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/livepeer/go-livepeer/eth"
)

func (w *wizard) transferTokens() {
	fmt.Printf("Current LPT balance: %v\n", w.getFormattedTokenBalance())

	fmt.Printf("Enter recipient address (in hex i.e. 0xfoo) - ")
	to := w.readString()

	fmt.Printf("Enter amount in LPT - ")
	amount := w.readPositiveBaseAmount()

	val := url.Values{
		"to":     {fmt.Sprintf("%v", to)},
		"amount": {fmt.Sprintf("%v", amount.String())},
	}

	var input string
	userAccepted := false
	for !userAccepted {
		fmt.Printf("Are you sure you want to send %s to \"%s\"? (y/n) - ", eth.FormatUnits(amount, "LPT"), val["to"][0])

		input = w.readString()

		switch strings.ToLower(input) {
		case "y":
			userAccepted = true
		case "n":
			fmt.Printf("Transaction cancelled: Interrupted by user.\n")
			return
		default:
			fmt.Printf("Enter (y)es or (n)o \n")
		}
	}

	httpPostWithParams(fmt.Sprintf("http://%v:%v/transferTokens", w.host, w.httpPort), val)
}

func (w *wizard) requestTokens() {
	httpPost(fmt.Sprintf("http://%v:%v/requestTokens", w.host, w.httpPort))
}
