package eth

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/golang/glog"
)

func FormatLPTU(baseAmount *big.Int) string {
	lptAmount := FromLPTU(baseAmount)

	if lptAmount.Cmp(big.NewFloat(1)) == -1 {
		return fmt.Sprintf("%v LPTU", baseAmount)
	} else {
		return fmt.Sprintf("%.21f LPT", lptAmount)
	}
}

func ToLPTU(lptAmount *big.Float) *big.Int {
	decimals := new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)
	floatDecimals := new(big.Float).SetInt(decimals)
	floatBaseAmount := new(big.Float).Mul(lptAmount, floatDecimals)

	baseAmount := new(big.Int)
	floatBaseAmount.Int(baseAmount)

	return baseAmount
}

func FromLPTU(baseAmount *big.Int) *big.Float {
	decimals := new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)
	floatDecimals := new(big.Float).SetInt(decimals)
	floatBaseAmount := new(big.Float).SetInt(baseAmount)

	return new(big.Float).Quo(floatBaseAmount, floatDecimals)
}

func FormatPerc(value *big.Int) string {
	perc := ToPerc(value)

	return fmt.Sprintf("%.21f", perc)
}

func ToPerc(value *big.Int) float64 {
	divisor := big.NewFloat(10000)
	floatValue := new(big.Float).SetInt(value)

	perc, _ := new(big.Float).Quo(floatValue, divisor).Float64()
	return perc
}

func FromPerc(perc float64) *big.Int {
	divisor := big.NewFloat(10000)
	floatValue := new(big.Float).Mul(big.NewFloat(perc), divisor)

	value := new(big.Int)
	floatValue.Int(value)

	return value
}

func Wait(backend *ethclient.Client, rpcTimeout time.Duration, blocks *big.Int) error {
	ctx, _ := context.WithTimeout(context.Background(), rpcTimeout)

	block, err := backend.BlockByNumber(ctx, nil)
	if err != nil {
		return err
	}

	targetBlockNum := new(big.Int).Add(block.Number(), blocks)

	glog.Infof("Waiting %v blocks...", blocks)

	for block.Number().Cmp(targetBlockNum) == -1 {
		ctx, _ = context.WithTimeout(context.Background(), rpcTimeout)

		block, err = backend.BlockByNumber(ctx, nil)
		if err != nil {
			return err
		}
	}

	return nil
}

func IsNullAddress(addr common.Address) bool {
	return addr == common.Address{}
}
