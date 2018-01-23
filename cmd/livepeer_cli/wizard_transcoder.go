package main

import (
	"fmt"
	"math/big"
	"net/url"
)

func (w *wizard) promptTranscoderConfig() (float64, float64, *big.Int) {
	var (
		blockRewardCut  float64
		feeShare        float64
		pricePerSegment *big.Int
	)

	fmt.Printf("Enter block reward cut percentage (default: 10) - ")
	blockRewardCut = w.readDefaultFloat(10.0)

	fmt.Printf("Enter fee share percentage (default: 5) - ")
	feeShare = w.readDefaultFloat(5.0)

	fmt.Printf("Enter price per segment (default: 1) - ")
	pricePerSegment = w.readDefaultBigInt(big.NewInt(1))

	return blockRewardCut, feeShare, pricePerSegment
}

func (w *wizard) activateTranscoder() {
	fmt.Printf("Current token balance: %v\n", w.getTokenBalance())

	blockRewardCut, feeShare, pricePerSegment := w.promptTranscoderConfig()

	fmt.Printf("You must bond to yourself in order to become a transcoder\n")

	amount := big.NewInt(0)
	fmt.Printf("Enter bond amount - ")
	amount = w.readBigInt()

	val := url.Values{
		"blockRewardCut":  {fmt.Sprintf("%v", blockRewardCut)},
		"feeShare":        {fmt.Sprintf("%v", feeShare)},
		"pricePerSegment": {fmt.Sprintf("%v", pricePerSegment.String())},
		"amount":          {fmt.Sprintf("%v", amount.String())},
	}

	httpPostWithParams(fmt.Sprintf("http://%v:%v/activateTranscoder", w.host, w.httpPort), val)
}

func (w *wizard) setTranscoderConfig() {
	fmt.Printf("Current token balance: %v\n", w.getTokenBalance())

	blockRewardCut, feeShare, pricePerSegment := w.promptTranscoderConfig()

	val := url.Values{
		"blockRewardCut":  {fmt.Sprintf("%v", blockRewardCut)},
		"feeShare":        {fmt.Sprintf("%v", feeShare)},
		"pricePerSegment": {fmt.Sprintf("%v", pricePerSegment.String())},
	}

	httpPostWithParams(fmt.Sprintf("http://%v:%v/setTranscoderConfig", w.host, w.httpPort), val)
}
