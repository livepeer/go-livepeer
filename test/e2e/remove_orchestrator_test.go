package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/big"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/eth"
	"github.com/stretchr/testify/require"
)

func TestRemoveOrchestrator(t *testing.T) {
	//given
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	geth := setupGeth(t)
	defer terminateGeth(t, geth)

	o := startOrchestrator(t, geth, ctx, nil)
	defer o.stop()

	lpEth := o.dev.Client
	<-o.ready

	initialCfg := OrchestratorConfig{
		PricePerUnit:   1,
		PixelsPerUnit:  10,
		BlockRewardCut: 30.0,
		FeeShare:       50.0,
		LptStake:       50,
	}

	balance, _ := o.dev.Client.BalanceOf(o.dev.Client.Account().Address)

	// when
	registerOrchestrator(o, &initialCfg)
	waitForNextRound(t, lpEth)
	waitUntilRoundInitialized(t, lpEth)

	deactivateOrchestrator(o, big.NewInt(initialCfg.LptStake))

	lock := getUnbondingLock(o)

	waitUntilRound(lock.WithdrawRound, lpEth, t)
	// round is not initialized when o is not in the next active set, so we do it here
	lpEth.InitializeRound()
	waitUntilRoundInitialized(t, lpEth)

	withdrawStake(o, big.NewInt(lock.ID))

	// then
	assertOrchestratorRemoved(t, o, big.NewInt(lock.ID))
	assertStakeWithdrawn(t, o, balance)
}

func waitUntilRound(round int64, lpEth eth.LivepeerEthClient, t *testing.T) {
	for {
		current_round, err := lpEth.CurrentRound()
		require.NoError(t, err)

		if current_round.Cmp(big.NewInt(round)) == 0 {
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func deactivateOrchestrator(o *livepeer, initialStake *big.Int) {
	val := url.Values{
		"amount": {fmt.Sprintf("%d", initialStake)},
	}

	for {
		if _, ok := httpPostWithParams(fmt.Sprintf("http://%s/unbond", *o.cfg.CliAddr), val); ok {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
}

func getUnbondingLock(o *livepeer) common.DBUnbondingLock {
	response, _ := http.Get(fmt.Sprintf("http://%s/unbondingLocks", *o.cfg.CliAddr))
	defer response.Body.Close()

	payload, _ := ioutil.ReadAll(response.Body)

	var unbondingLocks = []common.DBUnbondingLock{}
	json.Unmarshal(payload, &unbondingLocks)
	return unbondingLocks[0]
}

func assertOrchestratorRemoved(t *testing.T, o *livepeer, lockId *big.Int) {
	require := require.New(t)

	lock, _ := o.dev.Client.GetDelegatorUnbondingLock(o.dev.Client.Account().Address, lockId)
	require.Equal(0, lock.Amount.Cmp(big.NewInt(0)))
}

func assertStakeWithdrawn(t *testing.T, o *livepeer, oldBalance *big.Int) {
	require := require.New(t)

	balance, _ := o.dev.Client.BalanceOf(o.dev.Client.Account().Address)
	require.Equal(0, balance.Cmp(oldBalance))
}

func requireOrchestratorDeactivated(t *testing.T, o *livepeer) {
	require := require.New(t)

	transPool, err := o.dev.Client.TranscoderPool()
	trans, errTrans := o.dev.Client.GetTranscoder(o.dev.Client.Account().Address)

	require.NoError(err)
	require.NoError(errTrans)
	require.Len(transPool, 0)
	require.Equal(false, trans.Active)
	require.Equal("Not Registered", trans.Status)
}

func withdrawStake(o *livepeer, lockId *big.Int) {
	val := url.Values{
		"unbondingLockId": {fmt.Sprintf("%s", lockId)},
	}

	for {
		_, ok := httpPostWithParams(fmt.Sprintf("http://%s/withdrawStake", *o.cfg.CliAddr), val)
		if ok {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
}
