package e2e

import (
	"context"
	"math/big"
	"testing"
)

func TestHTTPPushBroadcaster(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	geth := setupGeth(t, ctx)
	defer terminateGeth(t, ctx, geth)

	o := startOrchestratorWithNewAccount(t, ctx, geth)
	defer o.stop()

	registerOrchestrator(t, o)
	requireOrchestratorRegisteredAndActivated(t, o)

	b := startBroadcasterWithNewAccount(t, ctx, geth)
	defer b.stop()

	amount := new(big.Int).Mul(big.NewInt(100), new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil))
	depositBroadcaster(t, b, amount)

	// Sequential requests
	pushSegmentsBroadcaster(t, b, 3)
}
