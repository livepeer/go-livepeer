package main

import (
	"fmt"
	"strings"
)

func (w *wizard) activateTranscoder() {
	var (
		blockRewardCut  int
		feeShare        int
		pricePerSegment int
		amount          int
	)

	fmt.Printf("Current token balance: %v\n", w.getTokenBalance())
	fmt.Printf("Enter block reward cut percentage (default: 10) - ")
	blockRewardCut = w.readDefaultInt(10)

	fmt.Printf("Enter fee share percentage (default: 5) - ")
	feeShare = w.readDefaultInt(5)

	fmt.Printf("Enter price per segment (default: 1) - ")
	pricePerSegment = w.readDefaultInt(1)

	fmt.Printf("Would you like to bond to yourself (you will not be active until someone bonds to you)? (y/n)")
	resp := w.read()
	if strings.Compare(strings.ToLower(resp), "y") == 0 {
		fmt.Printf("Enter bond amount - ")
		amount = w.readInt()
	}

	httpGet(fmt.Sprintf("http://%v:%v/activateTranscoder?blockRewardCut=%v&feeShare=%v&pricePerSegment=%v&amount=%v", w.host, w.httpPort, blockRewardCut, feeShare, pricePerSegment, amount))
}
