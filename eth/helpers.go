package eth

import (
	"context"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
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
