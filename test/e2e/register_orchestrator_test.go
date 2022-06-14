package e2e

import (
	"fmt"
	"github.com/livepeer/go-livepeer/eth"
	"github.com/stretchr/testify/require"
	"math/big"
	"net/url"
	"testing"
	"time"
)

func TestRegisterOrchestrator(t *testing.T) {
	// given
	geth := setupGeth(t)
	defer terminateGeth(t, geth)

	o := startOrchestrator(t, geth)
	lpEth := o.dev.Client
	defer o.stop()
	<-o.ready

	// when
	registerOrchestrator(o)
	waitForNextRound(t, lpEth)

	// then
	assertOrchestratorRegisteredAndActivated(t, lpEth)
}

func startOrchestrator(t *testing.T, geth *gethContainer) *livepeer {
	lpCfg := lpCfg()
	MaxFaceValue := big.NewInt(1000000).String()
	TicketEV := big.NewInt(1000000).String()
	lpCfg.Orchestrator = boolPointer(true)
	lpCfg.Transcoder = boolPointer(true)
	lpCfg.Redeemer = boolPointer(true)
	lpCfg.MaxFaceValue = &MaxFaceValue
	lpCfg.TicketEV = &TicketEV
	return startLivepeer(t, lpCfg, geth)
}

const (
	pricePerUnit  = 1
	pixelsPerUnit = 10
	rewardCut     = 30.0
	feeShare      = 50.0
	lptStake      = 50
)

func registerOrchestrator(o *livepeer) {
	val := url.Values{
		"pricePerUnit":   {fmt.Sprintf("%d", pricePerUnit)},
		"pixelsPerUnit":  {fmt.Sprintf("%d", pixelsPerUnit)},
		"blockRewardCut": {fmt.Sprintf("%v", rewardCut)},
		"feeShare":       {fmt.Sprintf("%v", feeShare)},
		"serviceURI":     {fmt.Sprintf("http://%s", *o.cfg.HttpAddr)},
		"amount":         {fmt.Sprintf("%d", lptStake)},
	}

	for {
		if _, ok := httpPostWithParams(fmt.Sprintf("http://%s/activateOrchestrator", *o.cfg.CliAddr), val); ok {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
}

func assertOrchestratorRegisteredAndActivated(t *testing.T, lpEth eth.LivepeerEthClient) {
	require := require.New(t)

	transPool, err := lpEth.TranscoderPool()

	require.NoError(err)
	require.Len(transPool, 1)
	trans := transPool[0]
	require.True(trans.Active)
	require.Equal("Registered", trans.Status)
	require.Equal(big.NewInt(lptStake), trans.DelegatedStake)
	require.Equal(eth.FromPerc(feeShare), trans.FeeShare)
	require.Equal(eth.FromPerc(rewardCut), trans.RewardCut)
}
