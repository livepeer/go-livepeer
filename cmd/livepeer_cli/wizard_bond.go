package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"text/tabwriter"

	"github.com/golang/glog"

	"github.com/ethereum/go-ethereum/common"
	eth "github.com/livepeer/go-livepeer/eth"
)

func (w *wizard) allTranscoderStats() map[int]common.Address {
	transcoderIds := make(map[int]common.Address)
	nextId := 0

	fmt.Println("REGISTERED CANDIDATE TRANSCODERS")
	fmt.Println("--------------------------------")

	resp, err := http.Get(fmt.Sprintf("http://%v:%v/candidateTranscodersStats", w.host, w.httpPort))
	if err != nil {
		glog.Errorf("Error getting node ID: %v", err)
		return nil
	}

	defer resp.Body.Close()
	result, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		glog.Errorf("Error reading response: %v", err)
		return nil
	}

	var candidateTranscoderStats []eth.TranscoderStats
	err = json.Unmarshal(result, &candidateTranscoderStats)
	if err != nil {
		glog.Errorf("Error unmarshalling transcoder stats: %v", err)
		return nil
	}

	wtr := tabwriter.NewWriter(os.Stdout, 0, 8, 1, '\t', 0)
	fmt.Fprintln(wtr, "Identifier\tAddress\tTotalStake\tBlockRewardCut\tFeeShare\tPricePerSegment\tPendingBlockRewardCut\tPendingFeeShare\tPendingPricePerSegment")
	for _, stats := range candidateTranscoderStats {
		fmt.Fprintf(wtr, "%v\t%v\t%v\t%v\t%v\t%v\t%v\t%v\t%v\n", nextId, stats.Address.Hex(), stats.TotalStake, stats.BlockRewardCut, stats.FeeShare, stats.PricePerSegment, stats.PendingBlockRewardCut, stats.PendingFeeShare, stats.PendingPricePerSegment)

		transcoderIds[nextId] = stats.Address
		nextId++
	}

	fmt.Fprintln(wtr, "REGISTERED RESERVE TRANSCODERS")
	fmt.Fprintln(wtr, "-------------------------------")

	resp, err = http.Get(fmt.Sprintf("http://%v:%v/reserveTranscodersStats", w.host, w.httpPort))
	if err != nil {
		glog.Errorf("Error getting node ID: %v", err)
		return nil
	}

	defer resp.Body.Close()
	result, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		glog.Errorf("Error reading response: %v", err)
		return nil
	}

	var reserveTranscoderStats []eth.TranscoderStats
	err = json.Unmarshal(result, &reserveTranscoderStats)
	if err != nil {
		glog.Errorf("Error unmarshalling transcoder stats: %v", err)
		return nil
	}

	fmt.Fprintln(wtr, "Identifier\tAddress\tTotalStake\tBlockRewardCut\tFeeShare\tPricePerSegment\tPendingBlockRewardCut\tPendingFeeShare\tPendingPricePerSegment")
	for _, stats := range reserveTranscoderStats {
		fmt.Fprintf(wtr, "%v\t%v\t%v\t%v\t%v\t%v\t%v\t%v\t%v\n", nextId, stats.Address.Hex(), stats.TotalStake, stats.BlockRewardCut, stats.FeeShare, stats.PricePerSegment, stats.PendingBlockRewardCut, stats.PendingFeeShare, stats.PendingPricePerSegment)

		transcoderIds[nextId] = stats.Address
		nextId++
	}

	wtr.Flush()

	return transcoderIds
}

func (w *wizard) bond() {
	transcoderIds := w.allTranscoderStats()
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
	amount := w.readInt()

	val := url.Values{
		"amount": {fmt.Sprintf("%v", amount)},
		"toAddr": {fmt.Sprintf("%v", tAddr.Hex())},
	}

	httpPostWithParams(fmt.Sprintf("http://%v:%v/bond", w.host, w.httpPort), val)
}

func (w *wizard) unbond() {
	httpPost(fmt.Sprintf("http://%v:%v/unbond", w.host, w.httpPort))
}

func (w *wizard) withdrawBond() {
	httpPost(fmt.Sprintf("http://%v:%v/withdrawBond", w.host, w.httpPort))
}
