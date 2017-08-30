package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"text/tabwriter"

	"github.com/ethereum/go-ethereum/log"
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
	fmt.Fprintf(wtr, "Deposit Amount: \t%s\n", w.getDeposit())
	fmt.Fprintf(wtr, "Broadcast Job Segment Price: \t%s\n", w.getJobPrice())
	fmt.Fprintf(wtr, "Is Active Transcoder: \t%s\n", w.getIsActiveTranscoder())
	fmt.Fprintf(wtr, "Transcoder Pending Block Reward Cut: \t%s\n", w.getPendingTranscoderBlockRewardCut())
	fmt.Fprintf(wtr, "Transcoder Pending Fee Share: \t%s\n", w.getPendingTranscoderFeeShare())
	fmt.Fprintf(wtr, "Transcoder Pending Price: \t%s\n", w.getPendingTranscoderPrice())
	fmt.Fprintf(wtr, "Transcoder Block Reward Cut: \t%s\n", w.getTranscoderBlockRewardCut())
	fmt.Fprintf(wtr, "Transcoder Fee Share: \t%s\n", w.getTranscoderFeeShare())
	fmt.Fprintf(wtr, "Transcoder Price: \t%s\n", w.getTranscoderPrice())
	fmt.Fprintf(wtr, "Transcoder Bond: \t%s\n", w.getTranscoderBond())
	fmt.Fprintf(wtr, "Transcoder Stake: \t%s\n", w.getTranscoderStake())
	fmt.Fprintf(wtr, "Delegator Stake: \t%s\n", w.getDelegatorStake())
	wtr.Flush()
}

func httpGet(url string) string {
	resp, err := http.Get(url)
	if err != nil {
		log.Error("Error getting node ID: %v")
		return ""
	}

	defer resp.Body.Close()
	result, err := ioutil.ReadAll(resp.Body)
	if err != nil || string(result) == "" {
		// log.Error(fmt.Sprintf("Error reading from: %v - %v", url, err))
		return ""
	}
	return string(result)

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

func (w *wizard) getDelegatorStake() string {
	e := httpGet(fmt.Sprintf("http://%v:%v/delegatorStake", w.host, w.httpPort))
	if e == "" {
		e = "Unknown"
	}
	return e
}

func (w *wizard) getJobPrice() string {
	return "TODO"
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
