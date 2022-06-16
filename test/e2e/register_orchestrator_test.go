package e2e

import (
	"context"
	"testing"
)

func TestRegisterOrchestrator(t *testing.T) {
	// given
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	geth := setupGeth(t)
	defer terminateGeth(t, geth)

	o := startOrchestrator(t, geth, ctx)
	defer o.stop()
	lpEth := o.dev.Client
	<-o.ready

	oParams := OrchestratorConfig{
		PricePerUnit:   1,
		PixelsPerUnit:  10,
		BlockRewardCut: 30.0,
		FeeShare:       50.0,
		LptStake:       50,
	}

	// when
	registerOrchestrator(o, &oParams)
	waitForNextRound(t, lpEth)

	// then
	requireOrchestratorRegisteredAndActivated(t, lpEth, &oParams)
}
