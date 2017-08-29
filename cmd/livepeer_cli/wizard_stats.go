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
	fmt.Fprintf(wtr, "Deposit Amount: \t%s\n", w.getDeposit())
	fmt.Fprintf(wtr, "Broadcast Job Segment Price: \t%s\n", w.getJobPrice())
	fmt.Fprintf(wtr, "Is Active Transcoder: \t%s\n", w.getIsActiveTranscoder())
	fmt.Fprintf(wtr, "Transcoder Price: \t%s\n", w.getTranscoderPrice())
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
	return httpGet(fmt.Sprintf("http://localhost:%v/nodeID", w.httpPort))
}

func (w *wizard) getNodeAddr() string {
	return httpGet(fmt.Sprintf("http://localhost:%v/nodeAddrs", w.httpPort))
}

func (w *wizard) getProtAddr() string {
	addr := httpGet(fmt.Sprintf("http://localhost:%v/protocolContractAddr", w.httpPort))
	if addr == "" {
		addr = "Unknown"
	}
	return addr
}

func (w *wizard) getTokenAddr() string {
	addr := httpGet(fmt.Sprintf("http://localhost:%v/tokenContractAddr", w.httpPort))
	if addr == "" {
		addr = "Unknown"
	}
	return addr
}

func (w *wizard) getFaucetAddr() string {
	return "TODO"
}

func (w *wizard) getEthAddr() string {
	addr := httpGet(fmt.Sprintf("http://localhost:%v/ethAddr", w.httpPort))
	if addr == "" {
		addr = "Unknown"
	}
	return addr
}

func (w *wizard) getTokenBalance() string {
	return "TODO"
}

func (w *wizard) getDeposit() string {
	return "TODO"
}

func (w *wizard) getJobPrice() string {
	return "TODO"
}

func (w *wizard) getIsActiveTranscoder() string {
	return httpGet(fmt.Sprintf("http://localhost:%v/isActiveTranscoder", w.httpPort))
}

func (w *wizard) getTranscoderPrice() string {
	return "TODO"
}
