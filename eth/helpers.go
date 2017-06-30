package eth

import (
	"context"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/golang/glog"
)

func waitForMinedTx(backend *ethclient.Client, rpcTimeout time.Duration, minedTxTimeout int, txHash common.Hash) (*types.Receipt, error) {
	var (
		receipt *types.Receipt
		ctx     context.Context
		err     error
	)

	for i := 0; i < minedTxTimeout; i++ {
		ctx, _ = context.WithTimeout(context.Background(), rpcTimeout)

		receipt, err = backend.TransactionReceipt(ctx, txHash)

		if err != nil && err != ethereum.NotFound {
			return nil, err
		}

		if receipt != nil {
			break
		}

		time.Sleep(time.Second)
	}

	return receipt, nil
}

func nextBlockMultiple(blockNum *big.Int, blockMultiple *big.Int) *big.Int {
	if blockMultiple.Cmp(big.NewInt(0)) == 0 {
		return blockNum
	}

	remainder := new(big.Int).Mod(blockNum, blockMultiple)

	if remainder.Cmp(big.NewInt(0)) == 0 {
		return blockNum
	}

	return new(big.Int).Sub(blockNum.Add(blockNum, blockMultiple), remainder)
}

func waitUntilNextRound(backend *ethclient.Client, rpcTimeout time.Duration, roundLength *big.Int) error {
	ctx, _ := context.WithTimeout(context.Background(), rpcTimeout)
	block, err := backend.BlockByNumber(ctx, nil)

	if err != nil {
		return err
	}

	targetBlockNum := nextBlockMultiple(block.Number(), roundLength)

	glog.Infof("Waiting until next round at block %v...", targetBlockNum)

	for block.Number().Cmp(targetBlockNum) == -1 {
		ctx, _ = context.WithTimeout(context.Background(), rpcTimeout)
		block, err = backend.BlockByNumber(ctx, nil)

		if err != nil {
			return err
		}
	}

	return nil
}
