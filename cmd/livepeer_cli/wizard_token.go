package main

import (
	"fmt"
	"log"
	"net/url"
	"strings"
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

	var answer string
	userAccepted := false
	for !userAccepted {
		fmt.Printf("Are you sure you want to send %s LPT to \"%s\"? [y/n]: ", val["amount"], val["to"])

		_, err := fmt.Scanln(&answer)
		if err != nil {
			log.Fatal(err)
		}

		switch strings.ToLower(answer) {
		case "y", "yes":
			userAccepted = true
		case "n", "no":
			fmt.Println("Transaction cancelled: Interrupted by user.")
			return
		default:
			fmt.Println("Wrong input!")
		}
	}

	httpPostWithParams(fmt.Sprintf("http://%v:%v/transferTokens", w.host, w.httpPort), val)
}

func (w *wizard) requestTokens() {
	httpPost(fmt.Sprintf("http://%v:%v/requestTokens", w.host, w.httpPort))
}
