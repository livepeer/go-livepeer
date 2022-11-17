package e2e

import (
	"context"
	"math/big"
	"testing"
)

func TestDepositBroadcaster(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	geth := setupGeth(t, ctx)
	defer terminateGeth(t, ctx, geth)

	b := startBroadcasterWithNewAccount(t, ctx, geth)
	defer b.stop()

	// 100 ETH
	amount := new(big.Int).Mul(big.NewInt(100), new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil))
	depositBroadcaster(t, b, amount)
}
