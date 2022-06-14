package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/big"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/livepeer/go-livepeer/eth"
	lpTypes "github.com/livepeer/go-livepeer/eth/types"
	"github.com/livepeer/go-livepeer/pm"
	"github.com/stretchr/testify/require"
)

func TestE2EStreamingWorkflow(t *testing.T) {
	geth := setupGeth(t)
	defer terminateGeth(t, geth)

	// 1. Start and register Orchestrator
	o := startOrchestrator(t, geth)
	// 2. Start Broadcaster
	b := startBroadcaster(t, geth)
	oEth := o.dev.Client
	bEth := b.dev.Client
	defer o.stop()
	defer b.stop()
	<-o.ready
	<-b.ready

	// when
	// 1. Start and register Orchestrator
	registerOrchestrator(o)
	waitForNextRound(t, oEth)

	// then
	fmt.Println("b.Account() orch ", oEth.Account())
	fmt.Println("b.Account() broadcaster ", bEth.Account())
	assertOrchestratorRegisteredAndActivated(t, oEth)

	// when
	// 3. Fund Deposit/Reserve
	fundBroadcasterDepositAndReserve(b)

	// then
	// 4. Check if broadcaster's funds have been correctly deposited
	assertBroadcasterFundsDeposited(t, b, bEth)
	// // waitForNextRound(t, bEth)

	// when
	// 5. Start a stream
	startStream(b)

	// then
	// 6. Check if the stream was transcoded
	assertSuccessfulStreamTranscoding(t, b)
	waitForNextRound(t, oEth)

	// 6. Check that a ticket was received by Orchestrator
	// 7. Redeem the ticket
	// 8. Check that ticket was redeemed and ETH was paid
	assertEarnings(t, o)
	// 9. Withdraw Fees and check that ETH is in the O's acount
	assertWithdrawal(t, o)

}

func startBroadcaster(t *testing.T, geth *gethContainer) *livepeer {
	lpCfg := lpCfg()
	lpCfg.Broadcaster = boolPointer(true)
	CurrentManifest := true
	DepositMultiplier := 1
	MaxTicketEV := "0"
	lpCfg.CurrentManifest = &CurrentManifest
	lpCfg.DepositMultiplier = &DepositMultiplier
	lpCfg.MaxTicketEV = &MaxTicketEV
	return startLivepeer(t, lpCfg, geth)
}

func fundBroadcasterDepositAndReserve(b *livepeer) {

	val := url.Values{
		"depositAmount": {fmt.Sprintf("%d", big.NewInt(1000000))},
		"reserveAmount": {fmt.Sprintf("%d", big.NewInt(1000000))},
	}

	if _, ok := httpPostWithParams(fmt.Sprintf("http://%s/fundDepositAndReserve", *b.cfg.CliAddr), val); ok {
		return
	}
	time.Sleep(200 * time.Millisecond)

}

func getBroadcasterFunds(urlAddr string, b eth.LivepeerEthClient) pm.SenderInfo {
	response, _ := http.Get(fmt.Sprintf("http://%s/senderInfo", urlAddr))
	result, _ := ioutil.ReadAll(response.Body)
	var broadcasterFunds pm.SenderInfo
	json.Unmarshal(result, &broadcasterFunds)
	return broadcasterFunds

}

func assertBroadcasterFundsDeposited(t *testing.T, bNode *livepeer, b eth.LivepeerEthClient) {
	require := require.New(t)

	broadcasterFunds := getBroadcasterFunds(*bNode.cfg.CliAddr, b)

	require.Equal(broadcasterFunds.Deposit, big.NewInt(1000000))
	require.Equal(broadcasterFunds.Reserve.FundsRemaining, big.NewInt(1000000))
}

func startStream(b *livepeer) {
	fakeSegment, _ := ioutil.ReadFile("../../server/test.ts")
	reader := bytes.NewReader(fakeSegment)

	response, _ := http.Post(fmt.Sprintf("http://%s/live/name/12.ts", *b.cfg.ServiceAddr), "application/x-www-form-urlencoded", reader)

	defer response.Body.Close()

}

func assertSuccessfulStreamTranscoding(t *testing.T, b *livepeer) {
	require := require.New(t)
	response, _ := http.Get(fmt.Sprintf("http://%s/stream/current.m3u8", *b.cfg.ServiceAddr))
	defer response.Body.Close()
	body, _ := ioutil.ReadAll(response.Body)

	require.Equal(200, response.StatusCode)
	require.Equal("#EXTM3U\n#EXT-X-VERSION:3\n#EXT-X-STREAM-INF:PROGRAM-ID=0,BANDWIDTH=4000000,RESOLUTION=0x0\nname/source.m3u8\n", string(body))

}

func assertEarnings(t *testing.T, o *livepeer) {
	require := require.New(t)
	var delegatorInfoBefore lpTypes.Delegator
	var delegatorInfoAfter lpTypes.Delegator

	delegateInfoResponseBefore, _ := http.Get(fmt.Sprintf("http://%s/delegatorInfo", *o.cfg.CliAddr))
	delegateInfoResponseBody, _ := ioutil.ReadAll(delegateInfoResponseBefore.Body)
	json.Unmarshal(delegateInfoResponseBody, &delegatorInfoBefore)

	httpPostWithParams(fmt.Sprintf("http://%s/claimEarnings", *o.cfg.CliAddr), nil)

	delegateInfoResponseAfter, _ := http.Get(fmt.Sprintf("http://%s/delegatorInfo", *o.cfg.CliAddr))
	delegateInfoBodyAfter, _ := ioutil.ReadAll(delegateInfoResponseAfter.Body)
	json.Unmarshal(delegateInfoBodyAfter, &delegatorInfoAfter)
	require.GreaterOrEqual(delegatorInfoAfter.Fees.Uint64(), delegatorInfoBefore.Fees.Uint64())
}

func assertWithdrawal(t *testing.T, o *livepeer) {
	require := require.New(t)
	var delegatorInfoBefore lpTypes.Delegator
	var delegatorInfoAfter lpTypes.Delegator
	var oEthBalanceBefore big.Int
	var oEthBalanceAfter big.Int

	delegateInfoResponseBefore, _ := http.Get(fmt.Sprintf("http://%s/delegatorInfo", *o.cfg.CliAddr))
	delegateInfoResponseBody, _ := ioutil.ReadAll(delegateInfoResponseBefore.Body)
	json.Unmarshal(delegateInfoResponseBody, &delegatorInfoBefore)

	oEthBalanceResponseBefore, _ := http.Get(fmt.Sprintf("http://%s/ethBalance", *o.cfg.CliAddr))
	oEthBalanceResponseBodyBefore, _ := ioutil.ReadAll(oEthBalanceResponseBefore.Body)
	json.Unmarshal(oEthBalanceResponseBodyBefore, &oEthBalanceBefore)
	val := url.Values{
		"amount": {fmt.Sprintf("%v", delegatorInfoBefore.PendingFees.String())},
	}

	httpPostWithParams(fmt.Sprintf("http://%s/withdrawFees", *o.cfg.CliAddr), val)

	delegateInfoResponseAfter, _ := http.Get(fmt.Sprintf("http://%s/delegatorInfo", *o.cfg.CliAddr))
	delegateInfoBodyAfter, _ := ioutil.ReadAll(delegateInfoResponseAfter.Body)
	json.Unmarshal(delegateInfoBodyAfter, &delegatorInfoAfter)

	oEthBalanceResponseAfter, _ := http.Get(fmt.Sprintf("http://%s/ethBalance", *o.cfg.CliAddr))
	oEthBalanceResponseBodyAfter, _ := ioutil.ReadAll(oEthBalanceResponseAfter.Body)
	json.Unmarshal(oEthBalanceResponseBodyAfter, &oEthBalanceAfter)

	require.Equal(delegatorInfoAfter.Fees, big.NewInt(0))
	require.Greater(oEthBalanceAfter.Uint64(), oEthBalanceBefore.Uint64())
}
