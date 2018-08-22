package eth

import (
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/golang/glog"
	lpcommon "github.com/livepeer/go-livepeer/common"
)

var (
	BlocksUntilFirstClaimDeadline = big.NewInt(200)
)

func VerifySig(addr common.Address, msg, sig []byte) bool {
	personalMsg := fmt.Sprintf("\x19Ethereum Signed Message:\n%d%s", 32, msg)
	personalHash := crypto.Keccak256([]byte(personalMsg))

	pubkey, err := crypto.SigToPub(personalHash, sig)
	if err != nil {
		return false
	}

	return crypto.PubkeyToAddress(*pubkey) == addr
}

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

func Wait(db *lpcommon.DB, blocks *big.Int) error {
	var (
		lastSeenBlock *big.Int
		err           error
	)

	lastSeenBlock, err = db.LastSeenBlock()
	if err != nil {
		return err
	}

	targetBlock := new(big.Int).Add(lastSeenBlock, blocks)
	tickCh := time.NewTicker(15 * time.Second).C

	glog.Infof("Waiting %v blocks...", blocks)

	for {
		select {
		case <-tickCh:
			if lastSeenBlock.Cmp(targetBlock) >= 0 {
				return nil
			}

			lastSeenBlock, err = db.LastSeenBlock()
			if err != nil {
				glog.Error("Error getting last seen block ", err)
				continue
			}
		}
	}

	return nil
}

func IsNullAddress(addr common.Address) bool {
	return addr == common.Address{}
}
