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

	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/eth"
	"github.com/stretchr/testify/require"
)

func TestRemoveOrchestrator(t *testing.T) {
	//given
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	geth := setupGeth(t, ctx)
	defer terminateGeth(t, ctx, geth)

	o := startOrchestratorWithNewAccount(t, ctx, geth)
	defer o.stop()

	registerOrchestrator(t, o)
	waitUntilRoundInitialized(t, o.dev.Client)

	// when
	deactivateOrchestrator(o, big.NewInt(initialCfg.LptStake))

	// then
	assertOrchestratorRemoved(t, o)

	//balance, _ := o.dev.Client.BalanceOf(o.dev.Client.Account().Address)
	//waitUntilRound(lock.WithdrawRound, o.dev.Client, t)
	//// round is not initialized when o is not in the next active set, so we do it here
	//o.dev.Client.InitializeRound()
	//waitUntilRoundInitialized(t, o.dev.Client)

	//withdrawStake(o, big.NewInt(lock.ID))

	//assertStakeWithdrawn(t, o, balance)
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

	glog.Errorf("deactivate orchestrator")

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

func assertOrchestratorRemoved(t *testing.T, o *livepeer) {
	require := require.New(t)

	lock := getUnbondingLock(o)
	require.Equal(0, lock.Amount.Cmp(big.NewInt(initialCfg.LptStake)))
	require.Equal(o.dev.Client.Account().Address, lock.Delegator)
}

func assertStakeWithdrawn(t *testing.T, o *livepeer, oldBalance *big.Int) {
	require := require.New(t)

	balance, _ := o.dev.Client.BalanceOf(o.dev.Client.Account().Address)
	require.Equal(0, balance.Cmp(oldBalance))
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
