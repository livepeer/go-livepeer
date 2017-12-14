package main

import (
	"fmt"
	"net/url"
	"strings"
)

func (w *wizard) promptTranscoderConfig() (int, int, int) {
	var (
		blockRewardCut  int
		feeShare        int
		pricePerSegment int
	)

	fmt.Printf("Enter block reward cut percentage (default: 10) - ")
	blockRewardCut = w.readDefaultInt(10)

	fmt.Printf("Enter fee share percentage (default: 5) - ")
	feeShare = w.readDefaultInt(5)

	fmt.Printf("Enter price per segment (default: 1) - ")
	pricePerSegment = w.readDefaultInt(1)

	return blockRewardCut, feeShare, pricePerSegment
}

func (w *wizard) activateTranscoder() {
	fmt.Printf("Current token balance: %v\n", w.getTokenBalance())

	blockRewardCut, feeShare, pricePerSegment := w.promptTranscoderConfig()

	fmt.Printf("Would you like to bond to yourself (your transaction will fail if no one has bonded to you, and you don't bond to yourself)? (y/n)")
	resp := w.read()

	var amount int
	if strings.Compare(strings.ToLower(resp), "y") == 0 {
		fmt.Printf("Enter bond amount - ")
		amount = w.readInt()
	}

	val := url.Values{
		"blockRewardCut":  {fmt.Sprintf("%v", blockRewardCut)},
		"feeShare":        {fmt.Sprintf("%v", feeShare)},
		"pricePerSegment": {fmt.Sprintf("%v", pricePerSegment)},
		"amount":          {fmt.Sprintf("%v", amount)},
	}

	httpPostWithParams(fmt.Sprintf("http://%v:%v/activateTranscoder", w.host, w.httpPort), val)
}

func (w *wizard) setTranscoderConfig() {
	fmt.Printf("Current token balance: %v\n", w.getTokenBalance())

	blockRewardCut, feeShare, pricePerSegment := w.promptTranscoderConfig()

	val := url.Values{
		"blockRewardCut":  {fmt.Sprintf("%v", blockRewardCut)},
		"feeShare":        {fmt.Sprintf("%v", feeShare)},
		"pricePerSegment": {fmt.Sprintf("%v", pricePerSegment)},
	}

	httpPostWithParams(fmt.Sprintf("http://%v:%v/setTranscoderConfig", w.host, w.httpPort), val)
}
