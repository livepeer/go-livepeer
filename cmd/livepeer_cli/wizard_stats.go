package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math/big"
	"net/http"
	"os"
	"strconv"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/params"
	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/eth"
	lpTypes "github.com/livepeer/go-livepeer/eth/types"
	"github.com/livepeer/go-livepeer/net"
	"github.com/olekukonko/tablewriter"
)

func (w *wizard) status() *net.NodeStatus {
	status := &net.NodeStatus{}
	statusJSON := httpGet(w.endpoint)
	if len(statusJSON) > 0 {
		err := json.Unmarshal([]byte(statusJSON), status)
		if err != nil {
			glog.Error("Error getting status:", err)
		}
	}
	return status
}

func (w *wizard) stats(showOrchestrator bool) {
	addrMap, err := w.getContractAddresses()
	if err != nil {
		glog.Errorf("Error getting contract addresses: %v", err)
		return
	}
	status := w.status()

	fmt.Println("+-----------+")
	fmt.Println("|NODE STATS|")
	fmt.Println("+-----------+")

	lptBal, _ := new(big.Int).SetString(w.getTokenBalance(), 10)
	ethBal, _ := new(big.Int).SetString(w.getEthBalance(), 10)
	maxGasPriceStr := "n/a"
	maxGasPrice, _ := new(big.Int).SetString(w.maxGasPrice(), 10)
	if maxGasPrice != nil && maxGasPrice.Cmp(big.NewInt(0)) > 0 {
		maxGasPriceStr = fmt.Sprintf("%v GWei", eth.FromWei(maxGasPrice, params.GWei))
	}

	table := tablewriter.NewWriter(os.Stdout)
	data := [][]string{
		{"Node's version", status.Version},
		{"Node's GO runtime version", status.GolangRuntimeVersion},
		{"Node's architecture", status.GOArch},
		{"Node's operating system", status.GOOS},
		{"HTTP Port", w.httpPort},
		{"Controller Address", addrMap["Controller"].Hex()},
		{"LivepeerToken Address", addrMap["LivepeerToken"].Hex()},
		{"LivepeerTokenFaucet Address", addrMap["LivepeerTokenFaucet"].Hex()},
		{"ETH Account", w.getEthAddr()},
		{"LPT Balance", eth.FormatUnits(lptBal, "LPT")},
		{"ETH Balance", eth.FormatUnits(ethBal, "ETH")},
		{"Max Gas Price", maxGasPriceStr},
	}

	for _, v := range data {
		table.Append(v)
	}

	table.SetAlignment(tablewriter.ALIGN_RIGHT)
	table.SetCenterSeparator("*")
	table.SetRowLine(true)
	table.SetColumnSeparator("|")
	table.Render()

	if showOrchestrator {
		w.orchestratorStats()
		w.delegatorStats()
	} else {
		w.broadcastStats()
		w.delegatorStats()
	}

	currentRound, err := w.currentRound()
	if err != nil {
		glog.Errorf("Error getting current round: %v", err)
		return
	}

	fmt.Printf("CURRENT ROUND: %v\n", currentRound)
}

func (w *wizard) protocolStats() {
	fmt.Println("+-------------------+}")
	fmt.Println("|PROTOCOL PARAMETERS|")
	fmt.Println("+-------------------+")

	params, err := w.getProtocolParameters()
	if err != nil {
		glog.Errorf("Error getting protocol parameters: %v", err)
		return
	}

	floatTotalBonded := new(big.Float)
	floatTotalBonded.SetInt(params.TotalBonded)
	floatTotalSupply := new(big.Float)
	floatTotalSupply.SetInt(params.TotalSupply)

	var currentParticipationRate *big.Float
	if floatTotalSupply.Cmp(big.NewFloat(0.0)) == 0 {
		currentParticipationRate = big.NewFloat(0.0)
	} else {
		participationRateRatio := new(big.Float).Quo(floatTotalBonded, floatTotalSupply)
		currentParticipationRate = new(big.Float).Mul(participationRateRatio, big.NewFloat(100.0))
	}

	table := tablewriter.NewWriter(os.Stdout)
	data := [][]string{
		{"Protocol Paused", fmt.Sprintf("%t", params.Paused)},
		{"Max # Active Orchestrators", params.NumActiveTranscoders.String()},
		{"RoundLength (Blocks)", params.RoundLength.String()},
		{"RoundLockAmount (%)", eth.FormatPerc(params.RoundLockAmount)},
		{"UnbondingPeriod (Rounds)", strconv.Itoa(int(params.UnbondingPeriod))},
		{"Inflation (%)", eth.FormatPercMinter(params.Inflation)},
		{"InflationChange (%)", eth.FormatPercMinter(params.InflationChange)},
		{"TargetBondingRate (%)", eth.FormatPercMinter(params.TargetBondingRate)},
		{"Total Bonded", eth.FormatUnits(params.TotalBonded, "LPT")},
		{"Total Supply", eth.FormatUnits(params.TotalSupply, "LPT")},
		{"Current Participation Rate (%)", currentParticipationRate.String()},
	}

	for _, v := range data {
		table.Append(v)
	}

	table.SetAlignment(tablewriter.ALIGN_RIGHT)
	table.SetCenterSeparator("*")
	table.SetRowLine(true)
	table.SetColumnSeparator("|")
	table.Render()
}

func (w *wizard) broadcastStats() {
	fmt.Println("+-----------------+")
	fmt.Println("|BROADCASTER STATS|")
	fmt.Println("+-----------------+")

	sender, err := w.senderInfo()
	if err != nil {
		glog.Errorf("Error getting sender info: %v", err)
		return
	}

	price, transcodingOptions := w.getBroadcastConfig()
	priceString := "n/a"
	if price != nil {
		priceString = fmt.Sprintf("%v wei / %v pixels", price.Num().Int64(), price.Denom().Int64())
	}

	table := tablewriter.NewWriter(os.Stdout)
	data := [][]string{
		{"Max Price Per Pixel", priceString},
		{"Broadcast Transcoding Options", transcodingOptions},
		{"Deposit", eth.FormatUnits(sender.Deposit, "ETH")},
		{"Reserve", eth.FormatUnits(sender.Reserve.FundsRemaining, "ETH")},
	}

	for _, v := range data {
		table.Append(v)
	}

	if sender.WithdrawRound.Cmp(big.NewInt(0)) > 0 && (sender.Deposit.Cmp(big.NewInt(0)) > 0 || sender.Reserve.FundsRemaining.Cmp(big.NewInt(0)) > 0) {
		table.Append([]string{"Withdraw Round", sender.WithdrawRound.String()})
	}

	table.SetAlignment(tablewriter.ALIGN_RIGHT)
	table.SetCenterSeparator("*")
	table.SetRowLine(true)
	table.SetColumnSeparator("|")
	table.Render()
}

func (w *wizard) orchestratorStats() {
	if w.offchain {
		return
	}
	t, priceInfo, err := w.getOrchestratorInfo()
	if err != nil {
		glog.Errorf("Error getting orchestrator info: %v", err)
		return
	}

	fmt.Println("+------------------+")
	fmt.Println("|ORCHESTRATOR STATS|")
	fmt.Println("+------------------+")

	table := tablewriter.NewWriter(os.Stdout)
	data := [][]string{
		{"Status", t.Status},
		{"Active", strconv.FormatBool(t.Active)},
		{"Service URI", t.ServiceURI},
		{"Delegated Stake", eth.FormatUnits(t.DelegatedStake, "LPT")},
		{"Reward Cut (%)", eth.FormatPerc(t.RewardCut)},
		{"Fee Share (%)", eth.FormatPerc(t.FeeShare)},
		{"Last Reward Round", t.LastRewardRound.String()},
		{"Base price per pixel", fmt.Sprintf("%v wei / %v pixels", priceInfo.Num(), priceInfo.Denom())},
	}

	for _, v := range data {
		table.Append(v)
	}

	table.SetAlignment(tablewriter.ALIGN_RIGHT)
	table.SetCenterSeparator("*")
	table.SetRowLine(true)
	table.SetColumnSeparator("|")
	table.Render()
}

func (w *wizard) delegatorStats() {
	if w.offchain {
		return
	}
	d, err := w.getDelegatorInfo()
	if err != nil {
		glog.Errorf("Error getting delegator info: %v", err)
		return
	}

	pendingStake := ""
	if d.PendingStake.Int64() == -1 {
		pendingStake = "Please fetch pending stake separately"
	} else {
		pendingStake = eth.FormatUnits(d.PendingStake, "LPT")
	}

	pendingFees := ""
	if d.PendingFees.Int64() == -1 {
		pendingFees = "Please fetch pending fees separately"
	} else {
		pendingFees = eth.FormatUnits(d.PendingFees, "ETH")
	}

	fmt.Println("+---------------+")
	fmt.Println("|DELEGATOR STATS|")
	fmt.Println("+---------------+")

	table := tablewriter.NewWriter(os.Stdout)
	data := [][]string{
		{"Status", d.Status},
		{"Stake", eth.FormatUnits(d.BondedAmount, "LPT")},
		{"Collected Fees", eth.FormatUnits(d.Fees, "ETH")},
		{"Pending Stake", pendingStake},
		{"Pending Fees", pendingFees},
		{"Delegated Stake", eth.FormatUnits(d.DelegatedAmount, "LPT")},
		{"Delegate Address", d.DelegateAddress.Hex()},
		{"Last Claim Round", d.LastClaimRound.String()},
		{"Start Round", d.StartRound.String()},
	}

	for _, v := range data {
		table.Append(v)
	}

	table.SetAlignment(tablewriter.ALIGN_RIGHT)
	table.SetCenterSeparator("*")
	table.SetRowLine(true)
	table.SetColumnSeparator("|")
	table.Render()
}

func (w *wizard) getProtocolParameters() (lpTypes.ProtocolParameters, error) {
	resp, err := http.Get(fmt.Sprintf("http://%v:%v/protocolParameters", w.host, w.httpPort))
	if err != nil {
		return lpTypes.ProtocolParameters{}, err
	}

	defer resp.Body.Close()

	result, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return lpTypes.ProtocolParameters{}, err
	}

	var params lpTypes.ProtocolParameters
	err = json.Unmarshal(result, &params)
	if err != nil {
		return lpTypes.ProtocolParameters{}, err
	}

	return params, nil
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

func (w *wizard) getBroadcastConfig() (*big.Rat, string) {
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
		MaxPrice           *big.Rat
		TranscodingOptions string
	}
	err = json.Unmarshal(result, &config)
	if err != nil {
		glog.Errorf("Error unmarshalling broadcast config: %v", err)
		return nil, ""
	}

	return config.MaxPrice, config.TranscodingOptions
}

func (w *wizard) getOrchestratorInfo() (*lpTypes.Transcoder, *big.Rat, error) {
	resp, err := http.Get(fmt.Sprintf("http://%v:%v/orchestratorInfo", w.host, w.httpPort))
	if err != nil {
		return nil, nil, err
	}

	defer resp.Body.Close()

	result, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, err
	}

	var config struct {
		Transcoder *lpTypes.Transcoder
		PriceInfo  *big.Rat
	}
	err = json.Unmarshal(result, &config)
	if err != nil {
		return nil, nil, err
	}

	return config.Transcoder, config.PriceInfo, nil
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

func (w *wizard) maxGasPrice() string {
	max := httpGet(fmt.Sprintf("http://%v:%v/maxGasPrice", w.host, w.httpPort))
	if max == "" {
		max = "n/a"
	}
	return max
}

func (w *wizard) currentBlock() (*big.Int, error) {
	resp, err := http.Get(fmt.Sprintf("http://%v:%v/currentBlock", w.host, w.httpPort))
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("http response status not ok")
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return new(big.Int).SetBytes(body), nil
}
