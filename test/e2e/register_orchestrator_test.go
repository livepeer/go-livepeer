package e2e

import (
	"context"
	"testing"
)

func TestRegisterOrchestrator(t *testing.T) {
	// given
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	geth := setupGeth(t, ctx)
	defer terminateGeth(t, ctx, geth)

	o := startOrchestratorWithNewAccount(t, ctx, geth)
	defer o.stop()

	// when
	registerOrchestrator(t, o)

	// then
	requireOrchestratorRegisteredAndActivated(t, o)
}
