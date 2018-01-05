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

func FormatUnits(baseAmount *big.Int, name string) string {
	amount := FromBaseUnit(baseAmount)

	if amount.Cmp(big.NewFloat(1)) == -1 {
		switch name {
		case "ETH":
			return fmt.Sprintf("%v WEI", baseAmount)
		default:
			return fmt.Sprintf("%v LPTU", baseAmount)
		}
	} else {
		switch name {
		case "ETH":
			return fmt.Sprintf("%v ETH", amount)
		default:
			return fmt.Sprintf("%v LPT", amount)
		}
	}
}

func ToBaseUnit(lptAmount *big.Float) *big.Int {
	decimals := new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)
	floatDecimals := new(big.Float).SetInt(decimals)
	floatBaseAmount := new(big.Float).Mul(lptAmount, floatDecimals)

	baseAmount := new(big.Int)
	floatBaseAmount.Int(baseAmount)

	return baseAmount
}

func FromBaseUnit(baseAmount *big.Int) *big.Float {
	decimals := new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)
	floatDecimals := new(big.Float).SetInt(decimals)
	floatBaseAmount := new(big.Float).SetInt(baseAmount)

	return new(big.Float).Quo(floatBaseAmount, floatDecimals)
}

func FormatPerc(value *big.Int) string {
	perc := ToPerc(value)

	return fmt.Sprintf("%v", perc)
}

func ToPerc(value *big.Int) float64 {
	pMultiplier := 10000.0

	return float64(value.Int64()) / pMultiplier
}

func FromPerc(perc float64) *big.Int {
	pMultiplier := 10000.0
	value := perc * pMultiplier

	return big.NewInt(int64(value))
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
