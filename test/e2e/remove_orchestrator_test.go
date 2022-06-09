package e2e

import (
	"context"
	"fmt"
	"math/big"
	"net/url"
	"testing"
	"time"

	ethcommon "github.com/ethereum/go-ethereum/common"

	"github.com/livepeer/go-livepeer/eth/contracts"
	"github.com/stretchr/testify/require"
)

func TestRemoveOrchestrator(t *testing.T) {
	// given
	geth := setupGeth(t)
	defer terminateGeth(t, geth)

	// when
	o := startAndRegisterOrchestrator(t, geth)
	balance, _ := o.dev.Client.BalanceOf(o.dev.Client.Account().Address)
	defer o.stop()

	waitForNextRound(t, o.dev.Client)
	o.dev.InitializeRound()

	lockId := deactivateOrchestrator(o)

	waitForNextRound(t, o.dev.Client)
	o.dev.InitializeRound()

	assertOrchestratorDeactivated(t, o)

	lock, _ := o.dev.Client.GetDelegatorUnbondingLock(o.dev.Client.Account().Address, lockId)

	waitUntilRound(lock.WithdrawRound, o, t)

	withdrawStake(o, lockId)

	// then
	assertOrchestratorRemoved(t, o, balance)
}

func waitUntilRound(round *big.Int, o *livepeer, t *testing.T) {
	for {
		current, _ := o.dev.Client.CurrentRound()
		if current.Cmp(round) == 0 {
			return
		}

		waitForNextRound(t, o.dev.Client)
		o.dev.InitializeRound()
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

func assertOrchestratorRemoved(t *testing.T, o *livepeer, oldBalance *big.Int) {
	require := require.New(t)

	var diff big.Int
	balance, _ := o.dev.Client.BalanceOf(o.dev.Client.Account().Address)
	require.Equal(diff.Sub(balance, oldBalance), big.NewInt(lptStake))
}

func assertOrchestratorDeactivated(t *testing.T, o *livepeer) {
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
