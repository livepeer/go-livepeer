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

func assertOrchestratorRemoved(t *testing.T, o *livepeer) {
	require := require.New(t)

	lock := getUnbondingLock(o)
	require.Equal(0, lock.Amount.Cmp(big.NewInt(initialCfg.LptStake)))
	require.Equal(o.dev.Client.Account().Address, lock.Delegator)
}

func getUnbondingLock(o *livepeer) common.DBUnbondingLock {
	response, _ := http.Get(fmt.Sprintf("http://%s/unbondingLocks", *o.cfg.CliAddr))
	defer response.Body.Close()

	payload, _ := ioutil.ReadAll(response.Body)

	var unbondingLocks = []common.DBUnbondingLock{}
	json.Unmarshal(payload, &unbondingLocks)
	return unbondingLocks[0]
}
