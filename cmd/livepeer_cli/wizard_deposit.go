package main

import (
	"fmt"
	"math/big"

	"github.com/livepeer/go-livepeer/eth"
)

func str2eth(v string) string {
	i, ok := big.NewInt(0).SetString(v, 10)
	if !ok {
		fmt.Printf("Could not convert %v to bigint", v)
		return ""
	}
	return eth.FormatUnits(i, "ETH")
}
