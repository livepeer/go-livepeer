package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/big"
	"net/http"
	"os"
	"strconv"
	"text/tabwriter"

	"github.com/ethereum/go-ethereum/common"
	"github.com/golang/glog"
	lpTypes "github.com/livepeer/go-livepeer/eth/types"
)

func (w *wizard) stats(showTranscoder bool) {
	addrMap, err := w.getContractAddresses()
	if err != nil {
		glog.Errorf("Error getting contract addresses: %v", err)
		return
	}

	// Observe how the b's and the d's, despite appearing in the
	// second cell of each line, belong to different columns.
	// wtr := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', tabwriter.AlignRight|tabwriter.Debug)
	wtr := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', tabwriter.AlignRight)
	fmt.Fprintf(wtr, "Node ID: \t%s\n", w.getNodeID())
	fmt.Fprintf(wtr, "Node Addr: \t%s\n", w.getNodeAddr())
	fmt.Fprintf(wtr, "RTMP Port: \t%s\n", w.rtmpPort)
	fmt.Fprintf(wtr, "HTTP Port: \t%s\n", w.httpPort)
	fmt.Fprintf(wtr, "Controller Address: \t%s\n", addrMap["Controller"].Hex())
	fmt.Fprintf(wtr, "LivepeerToken Address: \t%s\n", addrMap["LivepeerToken"].Hex())
	fmt.Fprintf(wtr, "LivepeerTokenFaucet Address: \t%s\n", addrMap["LivepeerTokenFaucet"].Hex())
	fmt.Fprintf(wtr, "ETH Account: \t%s\n", w.getEthAddr())
	fmt.Fprintf(wtr, "LPT Balance: \t%s\n", w.getTokenBalance())
	fmt.Fprintf(wtr, "ETH Balance: \t%s\n", w.getEthBalance())
	wtr.Flush()

	if showTranscoder {
		w.transcoderStats()
		w.delegatorStats()
	} else {
		w.broadcastStats()
		w.delegatorStats()
	}
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
	t, err := w.getTranscoderInfo()
	if err != nil {
		glog.Errorf("Error getting transcoder info: %v", err)
		return
	}

	wtr := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', tabwriter.AlignRight)
	fmt.Fprintln(wtr, "+----------------+")
	fmt.Fprintln(wtr, "|TRANSCODER STATS|")
	fmt.Fprintln(wtr, "+----------------+")
	fmt.Fprintf(wtr, "Status: \t%s\n", t.Status)
	fmt.Fprintf(wtr, "Active: \t%s\n", strconv.FormatBool(t.Active))
	fmt.Fprintf(wtr, "Delegated Stake: \t%s\n", t.DelegatedStake)
	fmt.Fprintf(wtr, "Block Reward Cut: \t%s\n", t.BlockRewardCut)
	fmt.Fprintf(wtr, "Fee Share: \t%s\n", t.FeeShare)
	fmt.Fprintf(wtr, "Price Per Segment: \t%s\n", t.PricePerSegment)
	fmt.Fprintf(wtr, "Pending Block Reward Cut: \t%s\n", t.PendingBlockRewardCut)
	fmt.Fprintf(wtr, "Pending Fee Share: \t%s\n", t.PendingFeeShare)
	fmt.Fprintf(wtr, "Pending Price Per Segment: \t%s\n", t.PendingPricePerSegment)
	fmt.Fprintf(wtr, "Last Reward Round: \t%s\n", t.LastRewardRound)
	wtr.Flush()
}

func (w *wizard) delegatorStats() {
	d, err := w.getDelegatorInfo()
	if err != nil {
		glog.Errorf("Error getting delegator info: %v", err)
		return
	}

	wtr := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', tabwriter.AlignRight)
	fmt.Fprintln(wtr, "+---------------+")
	fmt.Fprintln(wtr, "|DELEGATOR STATS|")
	fmt.Fprintln(wtr, "+---------------+")
	fmt.Fprintf(wtr, "Status: \t%s\n", d.Status)
	fmt.Fprintf(wtr, "Stake: \t%s\n", d.BondedAmount)
	fmt.Fprintf(wtr, "Fees: \t%s\n", d.Fees)
	fmt.Fprintf(wtr, "Pending Stake: \t%s\n", d.PendingStake)
	fmt.Fprintf(wtr, "Pending Fees: \t%s\n", d.PendingFees)
	fmt.Fprintf(wtr, "Delegated Stake: \t%s\n", d.DelegatedAmount)
	fmt.Fprintf(wtr, "Delegate Address: \t%s\n", d.DelegateAddress.Hex())
	fmt.Fprintf(wtr, "Last Claim Token Pools Shares Round: \t%s\n", d.LastClaimTokenPoolsSharesRound)
	fmt.Fprintf(wtr, "Start Round: \t%s\n", d.StartRound)
	fmt.Fprintf(wtr, "Withdraw Round: \t%s\n", d.WithdrawRound)
	wtr.Flush()
}

func (w *wizard) getNodeID() string {
	return httpGet(fmt.Sprintf("http://%v:%v/nodeID", w.host, w.httpPort))
}

func (w *wizard) getNodeAddr() string {
	return httpGet(fmt.Sprintf("http://%v:%v/nodeAddrs", w.host, w.httpPort))
}

func (w *wizard) getContractAddresses() (map[string]common.Address, error) {
	resp, err := http.Get(fmt.Sprintf("http://%v:%v/contractAddresses", w.host, w.httpPort))
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	result, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var addrMap map[string]common.Address
	err = json.Unmarshal(result, &addrMap)
	if err != nil {
		return nil, err
	}

	return addrMap, nil
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

func (w *wizard) getTranscoderInfo() (lpTypes.Transcoder, error) {
	resp, err := http.Get(fmt.Sprintf("http://%v:%v/transcoderInfo", w.host, w.httpPort))
	if err != nil {
		return lpTypes.Transcoder{}, err
	}

	defer resp.Body.Close()

	result, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return lpTypes.Transcoder{}, err
	}

	var tInfo lpTypes.Transcoder
	err = json.Unmarshal(result, &tInfo)
	if err != nil {
		return lpTypes.Transcoder{}, err
	}

	return tInfo, nil
}

func (w *wizard) getDelegatorInfo() (lpTypes.Delegator, error) {
	resp, err := http.Get(fmt.Sprintf("http://%v:%v/delegatorInfo", w.host, w.httpPort))
	if err != nil {
		return lpTypes.Delegator{}, err
	}

	defer resp.Body.Close()

	result, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return lpTypes.Delegator{}, err
	}

	var dInfo lpTypes.Delegator
	err = json.Unmarshal(result, &dInfo)
	if err != nil {
		return lpTypes.Delegator{}, err
	}

	return dInfo, nil
}
