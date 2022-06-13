package e2e

import (
	"context"
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/livepeer/go-livepeer/eth"
	"github.com/stretchr/testify/require"
)

type OrchestratorConfig struct {
	PricePerUnit   int
	PixelsPerUnit  int
	BlockRewardCut float64
	FeeShare       float64
	ServiceURI     string
}

func TestConfigureOrchestrator(t *testing.T) {
	// given
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	geth := setupGeth(t)
	defer terminateGeth(t, geth)

	// when
	o := startAndRegisterOrchestrator(t, geth, ctx)
	defer o.stop()

	waitForNextRound(t, o.dev.Client)
	o.dev.InitializeRound()

	requireOrchestratorRegisteredAndActivated(t, o.dev.Client)

	claimRewards(o)

	cfg := &OrchestratorConfig{
		PricePerUnit:   2,
		PixelsPerUnit:  12,
		BlockRewardCut: 25.0,
		FeeShare:       55.0,
		ServiceURI:     "127.0.0.1:18545",
	}

	configureOrchestrator(o, cfg)

	waitForNextRound(t, o.dev.Client)
	o.dev.InitializeRound()

	// then
	assertOrchestratorConfigured(t, o, cfg)
}

func startAndRegisterOrchestrator(t *testing.T, geth *gethContainer, ctx context.Context) *livepeer {
	const (
		pricePerUnit  = 1
		pixelsPerUnit = 10
		rewardCut     = 30.0
		feeShare      = 50.0
	)

	lpCfg := lpCfg()
	lpCfg.Orchestrator = boolPointer(true)
	lpCfg.Transcoder = boolPointer(true)
	o := startLivepeer(t, lpCfg, geth, ctx)

	<-o.ready

	val := url.Values{
		"pricePerUnit":   {fmt.Sprintf("%d", pricePerUnit)},
		"pixelsPerUnit":  {fmt.Sprintf("%d", pixelsPerUnit)},
		"blockRewardCut": {fmt.Sprintf("%v", rewardCut)},
		"feeShare":       {fmt.Sprintf("%v", feeShare)},
		"serviceURI":     {fmt.Sprintf("http://%v", &o.cfg.HttpAddr)},
		"amount":         {fmt.Sprintf("%d", lptStake)},
	}

	for {
		if _, ok := httpPostWithParams(fmt.Sprintf("http://%s/activateOrchestrator", *o.cfg.CliAddr), val); ok {
			return o
		}
		time.Sleep(200 * time.Millisecond)
	}
}

func claimRewards(o *livepeer) {
	val := url.Values{}

	for {
		if _, ok := httpPostWithParams(fmt.Sprintf("http://%s/reward", *o.cfg.CliAddr), val); ok {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
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
