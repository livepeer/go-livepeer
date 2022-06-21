package e2e

import (
	"context"
	"fmt"
	"math/big"
	"net/url"
	"testing"
	"time"

	"github.com/livepeer/go-livepeer/cmd/livepeer/starter"
	"github.com/livepeer/go-livepeer/eth"
	"github.com/stretchr/testify/require"
)

func TestConfigureOrchestrator(t *testing.T) {
	// given
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	geth := setupGeth(t, ctx)
	defer terminateGeth(t, geth, ctx)

	firstRunCtx, firstRunCancel := context.WithCancel(context.Background())
	o := startOrchestrator(t, geth, firstRunCtx, nil)

	lpEth := o.dev.Client
	<-o.ready

	initialCfg := OrchestratorConfig{
		PricePerUnit:   1,
		PixelsPerUnit:  10,
		BlockRewardCut: 30.0,
		FeeShare:       50.0,
		LptStake:       50,
	}

	// when
	registerOrchestrator(o, &initialCfg)
	waitForNextRound(t, lpEth)

	firstRunCancel()
	o.stop()

	sameAcct := starter.LivepeerConfig{EthAcctAddr: o.cfg.EthAcctAddr, Datadir: o.cfg.Datadir}
	o = startOrchestrator(t, geth, ctx, &sameAcct)
	defer o.stop()

	lpEth = o.dev.Client
	<-o.ready

	waitUntilOrchestratorIsConfigurable(t, lpEth)

	newCfg := &OrchestratorConfig{
		PricePerUnit:   2,
		PixelsPerUnit:  12,
		BlockRewardCut: 25.0,
		FeeShare:       55.0,
		ServiceURI:     "127.0.0.1:18545",
	}

	configureOrchestrator(o, newCfg)

	// then
	assertOrchestratorConfigured(t, o, newCfg)
}

func configureOrchestrator(o *livepeer, cfg *OrchestratorConfig) {
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
			time.Sleep(2 * time.Second)
			return
		}

		time.Sleep(2 * time.Second)
	}
}

func assertOrchestratorConfigured(t *testing.T, o *livepeer, cfg *OrchestratorConfig) {
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
