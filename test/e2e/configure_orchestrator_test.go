package e2e

import (
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/golang/glog"
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
	geth := setupGeth(t)
	defer terminateGeth(t, geth)

	// when
	o := startAndRegisterOrchestrator(t, geth)
	lpEth := o.dev.Client
	waitForNextRound(t, lpEth)
	initializeRound(o)

	// then
	//assertOrchestratorRegisteredAndActivated(t, lpEth)

	// when
	//deactivateOrchestrator(o)
	//waitForNextRound(t, lpEth)

	// then
	//assertOrchestratorDeactivated(t, o)

	//// when
	//cfg := &OrchestratorConfig{
	//	PricePerUnit:   2,
	//	PixelsPerUnit:  12,
	//	BlockRewardCut: 25.0,
	//	FeeShare:       55.0,
	//	ServiceURI:     "127.0.0.1:18545",
	//}
	//configureOrchestrator(o, cfg)
	//waitForNextRound(t, o.dev.Client)

	//// then
	//assertOrchestratorConfigured(t, o, cfg)
}

func startAndRegisterOrchestrator(t *testing.T, geth *gethContainer) *livepeer {
	const (
		pricePerUnit  = 1
		pixelsPerUnit = 10
		rewardCut     = 30.0
		feeShare      = 50.0
		lptStake      = 50
	)

	lpCfg := lpCfg()
	lpCfg.Orchestrator = boolPointer(true)
	lpCfg.Transcoder = boolPointer(true)
	o := startLivepeer(t, lpCfg, geth)

	defer o.stop()
	<-o.ready
	return o

	//val := url.Values{
	//	"pricePerUnit":   {fmt.Sprintf("%d", pricePerUnit)},
	//	"pixelsPerUnit":  {fmt.Sprintf("%d", pixelsPerUnit)},
	//	"blockRewardCut": {fmt.Sprintf("%v", rewardCut)},
	//	"feeShare":       {fmt.Sprintf("%v", feeShare)},
	//	"serviceURI":     {fmt.Sprintf("http://%v", o.cfg.HttpAddr)},
	//	"amount":         {fmt.Sprintf("%d", lptStake)},
	//}

	//for {
	//	if _, ok := httpPostWithParams(fmt.Sprintf("http://%s/activateOrchestrator", *o.cfg.CliAddr), val); ok {
	//		return o
	//	}
	//	time.Sleep(200 * time.Millisecond)
	//}
}

func deactivateOrchestrator(o *livepeer) {
	val := url.Values{
		"amount": {fmt.Sprintf("%d", lptStake)},
	}

	for {
		r, ok := httpPostWithParams(fmt.Sprintf("http://%s/unbond", *o.cfg.CliAddr), val)
		if ok {
			return
		}
		glog.Errorf("wating to unbind %v %v", r, ok)
		time.Sleep(200 * time.Millisecond)
	}
}

func configureOrchestrator(o *livepeer, cfg *OrchestratorConfig) {
	val := url.Values{
		"pricePerUnit":   {fmt.Sprintf("%d", cfg.PricePerUnit)},
		"pixelsPerUnit":  {fmt.Sprintf("%d", cfg.PixelsPerUnit)},
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

func initializeRound(o *livepeer) {
	//val := url.Values{}
	//httpPostWithParams(fmt.Sprintf("http://%s/initializeRound", *o.cfg.CliAddr), val)
	o.dev.InitializeRound()
}

func assertOrchestratorDeactivated(t *testing.T, o *livepeer) {
	require := require.New(t)

	transPool, err := o.dev.Client.TranscoderPool()
	trans, errTrans := o.dev.Client.GetTranscoder(o.dev.Client.Account().Address)

	require.NoError(err)
	require.NoError(errTrans)
	require.Len(transPool, 0)
	require.Equal(0, trans.Active)

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
	require.Equal(cfg.PricePerUnit, o.cfg.PricePerUnit)
	require.Equal(cfg.PixelsPerUnit, o.cfg.PixelsPerUnit)
	require.Equal(cfg.ServiceURI, uri)
}
