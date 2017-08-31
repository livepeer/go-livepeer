package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/big"
	"net/http"
	"os"
	"text/tabwriter"

	"github.com/golang/glog"
)

func (w *wizard) stats(tips bool) {
	// Observe how the b's and the d's, despite appearing in the
	// second cell of each line, belong to different columns.
	// wtr := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', tabwriter.AlignRight|tabwriter.Debug)
	wtr := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', tabwriter.AlignRight)
	fmt.Fprintf(wtr, "Node ID: \t%s\n", w.getNodeID())
	fmt.Fprintf(wtr, "Node Addr: \t%s\n", w.getNodeAddr())
	fmt.Fprintf(wtr, "RTMP Port: \t%s\n", w.rtmpPort)
	fmt.Fprintf(wtr, "HTTP Port: \t%s\n", w.httpPort)
	fmt.Fprintf(wtr, "Protocol Contract Addr: \t%s\n", w.getProtAddr())
	fmt.Fprintf(wtr, "Token Contract Addr: \t%s\n", w.getTokenAddr())
	fmt.Fprintf(wtr, "Faucet Contract Addr: \t%s\n", w.getFaucetAddr())
	fmt.Fprintf(wtr, "Account Eth Addr: \t%s\n", w.getEthAddr())
	fmt.Fprintf(wtr, "Token balance: \t%s\n", w.getTokenBalance())
	fmt.Fprintf(wtr, "Eth balance: \t%s\n", w.getEthBalance())
	wtr.Flush()

	w.broadcastStats()
	w.transcoderStats()
	w.delegatorStats()
}

func (w *wizard) broadcastStats() {
	wtr := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', tabwriter.AlignRight)
	fmt.Fprintln(wtr, "+---------------+")
	fmt.Fprintln(wtr, "|BROADCAST STATS|")
	fmt.Fprintln(wtr, "+---------------+")
	fmt.Fprintf(wtr, "Deposit Amount: \t%s\n", w.getDeposit())

	price, transcodingOptions := w.getBroadcastConfig()
	fmt.Fprintf(wtr, "Broadcast Job Segment Price: \t%s\n", price)
	fmt.Fprintf(wtr, "Broadcast Transcoding Options: \t%s\n", transcodingOptions)
	wtr.Flush()
}

func (w *wizard) transcoderStats() {
	wtr := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', tabwriter.AlignRight)
	fmt.Fprintln(wtr, "+----------------+")
	fmt.Fprintln(wtr, "|TRANSCODER STATS|")
	fmt.Fprintln(wtr, "+----------------+")
	fmt.Fprintf(wtr, "Transcoder Status: \t%s\n", w.getTranscoderStatus())
	fmt.Fprintf(wtr, "Is Active Transcoder: \t%s\n", w.getIsActiveTranscoder())
	fmt.Fprintf(wtr, "Pending Block Reward Cut: \t%s\n", w.getPendingTranscoderBlockRewardCut())
	fmt.Fprintf(wtr, "Pending Fee Share: \t%s\n", w.getPendingTranscoderFeeShare())
	fmt.Fprintf(wtr, "Pending Price: \t%s\n", w.getPendingTranscoderPrice())
	fmt.Fprintf(wtr, "Block Reward Cut: \t%s\n", w.getTranscoderBlockRewardCut())
	fmt.Fprintf(wtr, "Fee Share: \t%s\n", w.getTranscoderFeeShare())
	fmt.Fprintf(wtr, "Price: \t%s\n", w.getTranscoderPrice())
	fmt.Fprintf(wtr, "Bond: \t%s\n", w.getTranscoderBond())
	fmt.Fprintf(wtr, "Total Stake: \t%s\n", w.getTranscoderStake())
	wtr.Flush()
}

func (w *wizard) delegatorStats() {
	wtr := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', tabwriter.AlignRight)
	fmt.Fprintln(wtr, "+---------------+")
	fmt.Fprintln(wtr, "|DELEGATOR STATS|")
	fmt.Fprintln(wtr, "+---------------+")
	fmt.Fprintf(wtr, "Delegator Status: \t%s\n", w.getDelegatorStatus())
	fmt.Fprintf(wtr, "Total Stake: \t%s\n", w.getDelegatorStake())
	wtr.Flush()
}

func (w *wizard) getNodeID() string {
	return httpGet(fmt.Sprintf("http://%v:%v/nodeID", w.host, w.httpPort))
}

func (w *wizard) getNodeAddr() string {
	return httpGet(fmt.Sprintf("http://%v:%v/nodeAddrs", w.host, w.httpPort))
}

func (w *wizard) getProtAddr() string {
	addr := httpGet(fmt.Sprintf("http://%v:%v/protocolContractAddr", w.host, w.httpPort))
	if addr == "" {
		addr = "Unknown"
	}
	return addr
}

func (w *wizard) getTokenAddr() string {
	addr := httpGet(fmt.Sprintf("http://%v:%v/tokenContractAddr", w.host, w.httpPort))
	if addr == "" {
		addr = "Unknown"
	}
	return addr
}

func (w *wizard) getFaucetAddr() string {
	addr := httpGet(fmt.Sprintf("http://%v:%v/faucetContractAddr", w.host, w.httpPort))
	if addr == "" {
		addr = "Unknown"
	}
	return addr
}

func (w *wizard) getEthAddr() string {
	addr := httpGet(fmt.Sprintf("http://%v:%v/ethAddr", w.host, w.httpPort))
	if addr == "" {
		addr = "Unknown"
	}
	return addr
}

func (w *wizard) getTokenBalance() string {
	b := httpGet(fmt.Sprintf("http://%v:%v/tokenBalance", w.host, w.httpPort))
	if b == "" {
		b = "Unknown"
	}
	return b
}

func (w *wizard) getEthBalance() string {
	e := httpGet(fmt.Sprintf("http://%v:%v/ethBalance", w.host, w.httpPort))
	if e == "" {
		e = "Unknown"
	}
	return e
}

func (w *wizard) getDeposit() string {
	e := httpGet(fmt.Sprintf("http://%v:%v/broadcasterDeposit", w.host, w.httpPort))
	if e == "" {
		e = "Unknown"
	}
	return e
}

func (w *wizard) getTranscoderStatus() string {
	return httpGet(fmt.Sprintf("http://%v:%v/transcoderStatus", w.host, w.httpPort))
}

func (w *wizard) getTranscoderBond() string {
	e := httpGet(fmt.Sprintf("http://%v:%v/transcoderBond", w.host, w.httpPort))
	if e == "" {
		e = "Unknown"
	}
	return e
}

func (w *wizard) getTranscoderStake() string {
	e := httpGet(fmt.Sprintf("http://%v:%v/transcoderStake", w.host, w.httpPort))
	if e == "" {
		e = "Unknown"
	}
	return e
}

func (w *wizard) getDelegatorStatus() string {
	return httpGet(fmt.Sprintf("http://%v:%v/delegatorStatus", w.host, w.httpPort))
}

func (w *wizard) getDelegatorStake() string {
	e := httpGet(fmt.Sprintf("http://%v:%v/delegatorStake", w.host, w.httpPort))
	if e == "" {
		e = "Unknown"
	}
	return e
}

func (w *wizard) getBroadcastConfig() (*big.Int, string) {
	resp, err := http.Get(fmt.Sprintf("http://%v:%v/getBroadcastConfig", w.host, w.httpPort))
	if err != nil {
		glog.Errorf("Error getting broadcast config: %v", err)
		return nil, ""
	}

	defer resp.Body.Close()
	result, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		glog.Errorf("Error reading response: %v", err)
		return nil, ""
	}

	var config struct {
		MaxPricePerSegment *big.Int
		TranscodingOptions string
	}
	err = json.Unmarshal(result, &config)
	if err != nil {
		glog.Errorf("Error unmarshalling broadcast config: %v", err)
		return nil, ""
	}

	return config.MaxPricePerSegment, config.TranscodingOptions
}

func (w *wizard) getIsActiveTranscoder() string {
	return httpGet(fmt.Sprintf("http://%v:%v/isActiveTranscoder", w.host, w.httpPort))
}

func (w *wizard) getTranscoderBlockRewardCut() string {
	return httpGet(fmt.Sprintf("http://%v:%v/transcoderBlockRewardCut", w.host, w.httpPort))
}

func (w *wizard) getTranscoderFeeShare() string {
	return httpGet(fmt.Sprintf("http://%v:%v/transcoderFeeShare", w.host, w.httpPort))
}

func (w *wizard) getTranscoderPrice() string {
	return httpGet(fmt.Sprintf("http://%v:%v/transcoderPrice", w.host, w.httpPort))
}

func (w *wizard) getPendingTranscoderBlockRewardCut() string {
	return httpGet(fmt.Sprintf("http://%v:%v/transcoderPendingBlockRewardCut", w.host, w.httpPort))
}

func (w *wizard) getPendingTranscoderFeeShare() string {
	return httpGet(fmt.Sprintf("http://%v:%v/transcoderPendingFeeShare", w.host, w.httpPort))
}

func (w *wizard) getPendingTranscoderPrice() string {
	return httpGet(fmt.Sprintf("http://%v:%v/transcoderPendingPrice", w.host, w.httpPort))
}
