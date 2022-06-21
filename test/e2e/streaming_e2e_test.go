package e2e

import (
	"bytes"
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
	lpTypes "github.com/livepeer/go-livepeer/eth/types"
	"github.com/livepeer/go-livepeer/pm"
	"github.com/stretchr/testify/require"
)

func TestE2EStreamingWorkflow(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// given
	geth := setupGeth(t, ctx)
	defer terminateGeth(t, geth, ctx)

	// 1. Start and register Orchestrator
	o := startOrchestrator(t, geth, ctx)
	// 2. Start Broadcaster
	b := startBroadcaster(t, geth, ctx)
	oEth := o.dev.Client
	bEth := b.dev.Client
	defer o.stop()
	defer b.stop()
	<-o.ready
	<-b.ready
	fmt.Println("b.Account() orch ", oEth.Account())
	fmt.Println("b.Account() broadcaster ", bEth.Account())
	fmt.Println("o.cfg.MaxFaceValue")
	fmt.Println("o.cfg.TicketEV")
	fmt.Println(*o.cfg.MaxFaceValue)
	fmt.Println(*o.cfg.TicketEV)

	// when
	// 1. Start and register Orchestrator
	registerOrchestrator(o)
	waitForNextRound(t, oEth)

	// then
	assertOrchestratorRegisteredAndActivated(t, oEth)

	// when
	// 3. Fund Deposit/Reserve
	fundBroadcasterDepositAndReserve(b)

	// then
	// 4. Check if broadcaster's funds have been correctly deposited
	assertBroadcasterFundsDeposited(t, b, bEth)

	// when
	// 5. Start a stream
	startStream(b)

	// then
	// 6. Check if the stream was transcoded
	assertSuccessfulStreamTranscoding(t, b)
	assertSuccessfulStreamTranscoding(t, b)
	assertSuccessfulStreamTranscoding(t, b)
	assertSuccessfulStreamTranscoding(t, b)
	waitForNextRound(t, oEth)

	// 6. Check that a ticket was received by Orchestrator
	// 7. Redeem the ticket
	// 8. Check that ticket was redeemed and ETH was paid
	assertEarnings(t, o)
	// 9. Withdraw Fees and check that ETH is in the O's acount
	assertWithdrawal(t, o)
}

func startBroadcaster(t *testing.T, geth *gethContainer, ctx context.Context) *livepeer {
	lpCfg := lpCfg()
	lpCfg.Broadcaster = boolPointer(true)
	CurrentManifest := boolPointer(true)
	DepositMultiplier := 1
	MaxTicketEV := "0"
	lpCfg.CurrentManifest = CurrentManifest
	lpCfg.DepositMultiplier = &DepositMultiplier
	lpCfg.MaxTicketEV = &MaxTicketEV
	return startLivepeer(t, lpCfg, geth, ctx)
}

func fundBroadcasterDepositAndReserve(b *livepeer) {

	val := url.Values{
		"depositAmount": {fmt.Sprintf("%d", big.NewInt(4000000000000))},
		"reserveAmount": {fmt.Sprintf("%d", big.NewInt(4000000000000))},
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

	require.Equal(broadcasterFunds.Deposit, big.NewInt(4000000000000))
	require.Equal(broadcasterFunds.Reserve.FundsRemaining, big.NewInt(4000000000000))
}

func startStream(b *livepeer) {
	fakeSegment, _ := ioutil.ReadFile("./test.ts")
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
	fmt.Println("before orch info")
	res, err := http.Get(fmt.Sprintf("http://%s/registeredOrchestrators", *b.cfg.CliAddr))
	fmt.Println(res)
	fmt.Println(err)
	defer res.Body.Close()
	bbody, _ := ioutil.ReadAll(res.Body)
	var orchestrators []lpTypes.Transcoder
	json.Unmarshal(bbody, &orchestrators)
	fmt.Println(bbody)
	fmt.Println(orchestrators)

	dbh, dbraw, err := common.TempDB(t)
	defer dbh.Close()
	defer dbraw.Close()
	r, _ := b.dev.Client.CurrentRound()
	fmt.Println("round")
	fmt.Println(r)
	count, err3 := dbh.WinningTicketCount(b.dev.Client.Account().Address, int64(r.Uint64()))
	fmt.Println("count")
	fmt.Println(count)
	fmt.Println(err3)
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

	resp1, err1 := httpPostWithParams(fmt.Sprintf("http://%s/unlock", *o.cfg.CliAddr), nil)
	fmt.Println(resp1)
	fmt.Println("err1")
	fmt.Println(err1)
	waitForNextRound(t, o.dev.Client)
	resp, err := httpPostWithParams(fmt.Sprintf("http://%s/withdrawFees", *o.cfg.CliAddr), val)
	fmt.Println(resp)
	fmt.Println("err")
	fmt.Println(err)
	delegateInfoResponseAfter, _ := http.Get(fmt.Sprintf("http://%s/delegatorInfo", *o.cfg.CliAddr))
	delegateInfoBodyAfter, _ := ioutil.ReadAll(delegateInfoResponseAfter.Body)
	json.Unmarshal(delegateInfoBodyAfter, &delegatorInfoAfter)

	oEthBalanceResponseAfter, _ := http.Get(fmt.Sprintf("http://%s/ethBalance", *o.cfg.CliAddr))
	oEthBalanceResponseBodyAfter, _ := ioutil.ReadAll(oEthBalanceResponseAfter.Body)
	json.Unmarshal(oEthBalanceResponseBodyAfter, &oEthBalanceAfter)
	fmt.Println("delegatorInfoBefore.Fees.String()", delegatorInfoBefore.Fees.String())
	fmt.Println("delegatorInfoAfter.Fees.String()", delegatorInfoAfter.Fees.String())
	fmt.Println("delegatorInfoBefore.PendingFees.String()", delegatorInfoBefore.PendingFees.String())
	fmt.Println("delegatorInfoAfter.PendingFees.String()", delegatorInfoAfter.PendingFees.String())
	fmt.Println("oEthBalanceBefore.Fees.String()", oEthBalanceBefore.String())
	fmt.Println("oEthBalanceAfter.Fees.String()", oEthBalanceAfter.String())
	// require.Equal(delegatorInfoAfter.Fees, big.NewInt(0))
	require.Greater(oEthBalanceAfter.Uint64(), oEthBalanceBefore.Uint64())
}
