package e2e

import (
	"context"
	"fmt"
	"math/big"
	"net/url"
	"testing"
	"time"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/eth"
	contracts "github.com/livepeer/go-livepeer/eth/contracts"
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
	defer o.stop()
	lpEth := o.dev.Client
	waitForNextRound(t, lpEth)

	// then
	assertOrchestratorRegisteredAndActivated(t, lpEth)

	// when
	o.dev.InitializeRound()
	lockId := deactivateOrchestrator(o)
	waitForNextRound(t, lpEth)

	// then
	assertOrchestratorDeactivated(t, o)

	// when
	o.dev.InitializeRound()
	waitForNextRound(t, o.dev.Client)
	o.dev.InitializeRound()
	waitForNextRound(t, o.dev.Client)
	o.dev.InitializeRound()

	{
		round, _ := o.dev.Client.CurrentRound()
		tc, _ := o.dev.Client.GetTranscoder(o.dev.Client.Account().Address)
		del, _ := o.dev.Client.GetDelegator(o.dev.Client.Account().Address)

		glog.Errorf("bond %v, start %v, status %v", del.BondedAmount, del.StartRound, del.Status)
		glog.Errorf("current round %v", round)
		glog.Errorf("transcoder active round %v", tc.ActivationRound)
		glog.Errorf("transcoder status %v", tc.Status)
		glog.Errorf("transcoder active %v", tc.Active)
		glog.Errorf("transcoder stake %v", tc.DelegatedStake)
	}

	rebond(o, lockId)
	waitForNextRound(t, o.dev.Client)
	o.dev.InitializeRound()

	{
		round, _ := o.dev.Client.CurrentRound()
		tc, _ := o.dev.Client.GetTranscoder(o.dev.Client.Account().Address)
		del, _ := o.dev.Client.GetDelegator(o.dev.Client.Account().Address)

		glog.Errorf("bond %v, start %v, status %v", del.BondedAmount, del.StartRound, del.Status)
		glog.Errorf("current round %v", round)
		glog.Errorf("transcoder active round %v", tc.ActivationRound)
		glog.Errorf("transcoder status %v", tc.Status)
		glog.Errorf("transcoder active %v", tc.Active)
		glog.Errorf("transcoder stake %v", tc.DelegatedStake)
	}

	cfg := &OrchestratorConfig{
		PricePerUnit:   2,
		PixelsPerUnit:  12,
		BlockRewardCut: 25.0,
		FeeShare:       55.0,
		ServiceURI:     "127.0.0.1:18545",
	}

	o.dev.InitializeRound()
	configureOrchestrator(o, cfg)
	waitForNextRound(t, o.dev.Client)
	o.dev.InitializeRound()

	// then
	assertOrchestratorConfigured(t, o, cfg)
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

	<-o.ready

	val := url.Values{
		"pricePerUnit":   {fmt.Sprintf("%d", pricePerUnit)},
		"pixelsPerUnit":  {fmt.Sprintf("%d", pixelsPerUnit)},
		"blockRewardCut": {fmt.Sprintf("%v", rewardCut)},
		"feeShare":       {fmt.Sprintf("%v", feeShare)},
		"serviceURI":     {fmt.Sprintf("http://%v", o.cfg.HttpAddr)},
		"amount":         {fmt.Sprintf("%d", lptStake)},
	}

	for {
		if _, ok := httpPostWithParams(fmt.Sprintf("http://%s/activateOrchestrator", *o.cfg.CliAddr), val); ok {
			return o
		}
		time.Sleep(200 * time.Millisecond)
	}
}

func deactivateOrchestrator(o *livepeer) *big.Int {
	val := url.Values{
		"amount": {fmt.Sprintf("%d", lptStake)},
	}

	for {
		txHash, ok := httpPostWithParams(fmt.Sprintf("http://%s/unbond", *o.cfg.CliAddr), val)
		if ok {
			receipt, err := o.dev.Client.Backend().TransactionReceipt(context.TODO(), ethcommon.BytesToHash([]byte(txHash)))
			if err == nil {
				abi, err := contracts.BondingManagerMetaData.GetAbi()
				if err == nil {
					event := new(contracts.BondingManagerUnbond)
					log := receipt.Logs[2]
					abi.UnpackIntoInterface(event, "Unbond", log.Data)
					return event.UnbondingLockId
				}
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
}

func rebond(o *livepeer, lockId *big.Int) {
	val := url.Values{
		"unbondingLockId": {fmt.Sprintf("%d", lockId)},
		"toAddr":          {fmt.Sprintf("%v", o.dev.Client.Account())},
	}

	for {
		if _, ok := httpPostWithParams(fmt.Sprintf("http://%s/rebond", *o.cfg.CliAddr), val); ok {
			return
		}
		time.Sleep(2000 * time.Microsecond)
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

func assertOrchestratorDeactivated(t *testing.T, o *livepeer) {
	require := require.New(t)

	transPool, err := o.dev.Client.TranscoderPool()
	trans, errTrans := o.dev.Client.GetTranscoder(o.dev.Client.Account().Address)

	require.NoError(err)
	require.NoError(errTrans)
	require.Len(transPool, 0)
	require.Equal(false, trans.Active)
}

func assertOrchestratorConfigured(t *testing.T, o *livepeer, cfg *OrchestratorConfig) {
	require := require.New(t)

	transPool, err := o.dev.Client.TranscoderPool()
	uri, errURI := o.dev.Client.GetServiceURI(o.dev.Client.Account().Address)

	glog.Errorf("len %v, uri %v", len(transPool), uri)
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
