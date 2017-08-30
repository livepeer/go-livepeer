package main

import (
	"encoding/json"
	"fmt"
	"github.com/golang/glog"
	"io/ioutil"
	"net/http"
	"os"
	"text/tabwriter"

	"github.com/ethereum/go-ethereum/common"
	eth "github.com/livepeer/go-livepeer/eth"
)

func (w *wizard) allTranscoderStats() map[int]common.Address {
	fmt.Println("REGISTERED TRANSCODERS")
	fmt.Println("--------------------------")

	resp, err := http.Get(fmt.Sprintf("http://%v:%v/allTranscoderStats", w.host, w.httpPort))
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

	var transcoderStats []eth.TranscoderStats
	err = json.Unmarshal(result, &transcoderStats)
	if err != nil {
		glog.Errorf("Error unmarshalling transcoder stats: %v", err)
		return nil
	}

	transcoderIds := make(map[int]common.Address)

	wtr := tabwriter.NewWriter(os.Stdout, 0, 8, 1, '\t', 0)
	fmt.Fprintln(wtr, "Identifier\tAddress\tTotalStake\tBlockRewardCut\tFeeShare\tPricePerSegment\tPendingBlockRewardCut\tPendingFeeShare\tPendingPricePerSegment")
	for idx, stats := range transcoderStats {
		fmt.Fprintf(wtr, "%v\t%v\t%v\t%v\t%v\t%v\t%v\t%v\t%v\n", idx, stats.Address.Hex(), stats.TotalStake, stats.BlockRewardCut, stats.FeeShare, stats.PricePerSegment, stats.PendingBlockRewardCut, stats.PendingFeeShare, stats.PendingPricePerSegment)

		transcoderIds[idx] = stats.Address
	}

	wtr.Flush()

	return transcoderIds
}

func (w *wizard) bond() {
	transcoderIds := w.allTranscoderStats()
	if transcoderIds == nil {
		return
	}

	fmt.Printf("Enter the identifier of the transcoder you would like to bond to - ")
	id := w.readInt()

	fmt.Printf("Enter bond amount - ")
	amount := w.readInt()

	httpGet(fmt.Sprintf("http://%v:%v/bond?amount=%v&toAddr=%v", w.host, w.httpPort, amount, transcoderIds[id].Hex()))
}

func (w *wizard) unbond() {

}
