package e2e

import (
	"context"
	"fmt"
	"math/big"
	"net/url"
	"testing"
	"time"

	"github.com/livepeer/go-livepeer/eth"
	"github.com/stretchr/testify/require"
)

func TestConfigureOrchestrator(t *testing.T) {
	// given
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	geth := setupGeth(t, ctx)
	defer terminateGeth(t, ctx, geth)

	oCtx, oCancel := context.WithCancel(context.Background())
	o := startOrchestratorWithNewAccount(t, oCtx, geth)
	registerOrchestrator(t, o)

	// orchestrator needs to restart in order to be able to process rewards, so configuration is unlocked
	o = restartOrchestrator(t, ctx, oCancel, geth, o)
	defer o.stop()

	waitUntilOrchestratorIsConfigurable(t, o.dev.Client)

	// when
	configureOrchestrator(o, newCfg)

	// then
	assertOrchestratorConfigured(t, o, newCfg)
}

func restartOrchestrator(t *testing.T, ctx context.Context, cancel context.CancelFunc, geth *gethContainer, o *livepeer) *livepeer {
	o.stop()
	cancel()
	return startOrchestratorWithExistingAccount(t, ctx, geth, o.cfg.EthAcctAddr, o.cfg.Datadir)
}

func waitUntilOrchestratorIsConfigurable(t *testing.T, lpEth eth.LivepeerEthClient) {
	require := require.New(t)

	for {
		active, err := lpEth.IsActiveTranscoder()
		require.NoError(err)

		initialized, err := lpEth.CurrentRoundInitialized()
		require.NoError(err)

		t, err := lpEth.GetTranscoder(lpEth.Account().Address)
		require.NoError(err)
		rewardCalled := t.LastRewardRound.Cmp(big.NewInt(0)) > 0

		if active && initialized && rewardCalled {
			return
		}

		time.Sleep(2 * time.Second)
	}
}

func configureOrchestrator(o *livepeer, cfg *orchestratorConfig) {
	val := url.Values{
		"blockRewardCut": {fmt.Sprintf("%v", cfg.BlockRewardCut)},
		"feeShare":       {fmt.Sprintf("%v", cfg.FeeShare)},
		"serviceURI":     {fmt.Sprintf("http://%v", cfg.ServiceURI)},
	}

	for {
		if _, ok := httpPostWithParams(fmt.Sprintf("http://%s/setOrchestratorConfig", *o.cfg.CliAddr), val); ok {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
}

func assertOrchestratorConfigured(t *testing.T, o *livepeer, cfg *orchestratorConfig) {
	require := require.New(t)

	transPool, err := o.dev.Client.TranscoderPool()
	uri, errURI := o.dev.Client.GetServiceURI(o.dev.Client.Account().Address)

	require.NoError(err)
	require.NoError(errURI)
	require.Len(transPool, 1)
	trans := transPool[0]
	require.Equal(eth.FromPerc(cfg.FeeShare), trans.FeeShare)
	require.Equal(eth.FromPerc(cfg.BlockRewardCut), trans.RewardCut)
	require.Equal(fmt.Sprintf("http://%v", cfg.ServiceURI), uri)
}
