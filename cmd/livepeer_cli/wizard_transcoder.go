package main

import (
	"fmt"
	"math/big"
	"net/url"

	lpcommon "github.com/livepeer/go-livepeer/common"
)

func (w *wizard) isTranscoder() bool {
	isT := httpGet(fmt.Sprintf("http://%v:%v/IsTranscoder", w.host, w.httpPort))
	return isT == "true"
}

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
	d, err := w.getDelegatorInfo()
	fmt.Printf("Current token balance: %v\n", w.getTokenBalance())
	fmt.Printf("Current bonded amount: %v\n", d.BondedAmount.String())

	blockRewardCut, feeShare, pricePerSegment := w.promptTranscoderConfig()

	fmt.Printf("You must bond to yourself in order to become a transcoder\n")

	balBigInt, err := lpcommon.ParseBigInt(w.getTokenBalance())
	if err != nil {
		fmt.Printf("Cannot read token balance: %v", w.getTokenBalance())
		return
	}

	amount := big.NewInt(0)
	for amount.Cmp(big.NewInt(0)) == 0 || balBigInt.Cmp(amount) < 0 {
		fmt.Printf("Enter bond amount - ")
		amount = w.readBigInt()
		if balBigInt.Cmp(amount) < 0 {
			fmt.Printf("Must enter an amount smaller than the current balance. ")
		}
		if amount.Cmp(big.NewInt(0)) == 0 && d.BondedAmount.Cmp(big.NewInt(0)) > 0 {
			break
		}
	}

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
