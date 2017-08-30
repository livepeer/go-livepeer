package main

import "fmt"

func (w *wizard) deposit() {
	fmt.Printf("Current deposit: %v\n", w.getDeposit())
	fmt.Printf("Current token balance: %v\n", w.getTokenBalance())
	fmt.Printf("Enter Deposit Amount - ")
	amount := w.readInt()
	httpGet(fmt.Sprintf("http://%v:%v/deposit?amount=%v", w.host, w.httpPort, amount))
}
