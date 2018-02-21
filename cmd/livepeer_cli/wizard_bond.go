package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strconv"

	"github.com/golang/glog"

	"github.com/ethereum/go-ethereum/common"
	"github.com/livepeer/go-livepeer/eth"
	lpTypes "github.com/livepeer/go-livepeer/eth/types"
	"github.com/olekukonko/tablewriter"
)

func (w *wizard) registeredTranscoderStats() map[int]common.Address {
	transcoders, err := w.getRegisteredTranscoders()
	if err != nil {
		glog.Errorf("Error getting registered transcoders: %v", err)
		return nil
	}

	transcoderIDs := make(map[int]common.Address)
	nextId := 0

	fmt.Println("+----------------------+")
	fmt.Println("|REGISTERED TRANSCODERS|")
	fmt.Println("+----------------------+")

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"ID", "Address", "Active", "Delegated Stake", "Reward Cut (%)", "Fee Share (%)", "Price", "Pending Reward Cut (%)", "Pending Fee Share (%)", "Pending Price"})

	for _, t := range transcoders {
		table.Append([]string{
			strconv.FormatInt(int64(nextId), 10),
			t.Address.Hex(),
			strconv.FormatBool(t.Active),
			eth.FormatUnits(t.DelegatedStake, "LPT"),
			eth.FormatPerc(t.RewardCut),
			eth.FormatPerc(t.FeeShare),
			eth.FormatUnits(t.PricePerSegment, "ETH"),
			eth.FormatPerc(t.PendingRewardCut),
			eth.FormatPerc(t.PendingFeeShare),
			eth.FormatUnits(t.PendingPricePerSegment, "ETH"),
		})

		transcoderIDs[nextId] = t.Address
		nextId++
	}

	table.Render()

	return transcoderIDs
}

func (w *wizard) getRegisteredTranscoders() ([]lpTypes.Transcoder, error) {
	resp, err := http.Get(fmt.Sprintf("http://%v:%v/registeredTranscoders", w.host, w.httpPort))
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	result, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var transcoders []lpTypes.Transcoder
	err = json.Unmarshal(result, &transcoders)
	if err != nil {
		return nil, err
	}

	return transcoders, nil
}

func (w *wizard) bond() {
	transcoderIds := w.registeredTranscoderStats()
	var tAddr common.Address
	if transcoderIds == nil {
		fmt.Printf("Enter the address of the transcoder you would like to bond to - ")
		strAddr := w.readString()
		if err := tAddr.UnmarshalText([]byte(strAddr)); err != nil {
			fmt.Println(err)
			return
		}
	} else {
		fmt.Printf("Enter the identifier of the transcoder you would like to bond to - ")
		id := w.readInt()
		tAddr = transcoderIds[id]
	}

	fmt.Printf("Enter bond amount - ")
	amount := w.readBigInt()

	val := url.Values{
		"amount": {fmt.Sprintf("%v", amount.String())},
		"toAddr": {fmt.Sprintf("%v", tAddr.Hex())},
	}

	httpPostWithParams(fmt.Sprintf("http://%v:%v/bond", w.host, w.httpPort), val)
}

func (w *wizard) unbond() {
	httpPost(fmt.Sprintf("http://%v:%v/unbond", w.host, w.httpPort))
}

func (w *wizard) withdrawStake() {
	httpPost(fmt.Sprintf("http://%v:%v/withdrawStake", w.host, w.httpPort))
}

func (w *wizard) withdrawFees() {
	httpPost(fmt.Sprintf("http://%v:%v/withdrawFees", w.host, w.httpPort))
}

func (w *wizard) claimRewardsAndFees() {
	fmt.Printf("Current round: %v\n", w.currentRound())

	d, err := w.getDelegatorInfo()
	if err != nil {
		glog.Error(err)
		return
	}

	fmt.Printf("Last claim round: %v\n", d.LastClaimRound)

	fmt.Printf("Enter end round - ")
	endRound := w.readBigInt()

	val := url.Values{
		"endRound": {fmt.Sprintf("%v", endRound.String())},
	}

	httpPostWithParams(fmt.Sprintf("http://%v:%v/claimEarnings", w.host, w.httpPort), val)
}
