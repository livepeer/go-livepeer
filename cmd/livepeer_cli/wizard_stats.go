package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"strconv"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/params"
	"github.com/golang/glog"
	lcommon "github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/eth"
	lpTypes "github.com/livepeer/go-livepeer/eth/types"
	"github.com/olekukonko/tablewriter"
)

func (w *wizard) status() *lcommon.NodeStatus {
	status := &lcommon.NodeStatus{}
	statusJSON := httpGet(w.endpoint)
	if len(statusJSON) > 0 {
		err := json.Unmarshal([]byte(statusJSON), status)
		if err != nil {
			glog.Error("Error getting status:", err)
		}
	}
	return status
}

func (w *wizard) stats(isOrchestrator bool) {
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

	minGasPriceStr := "n/a"
	minGasPrice, _ := new(big.Int).SetString(w.minGasPrice(), 10)
	if minGasPrice != nil {
		minGasPriceStr = fmt.Sprintf("%v GWei", eth.FromWei(minGasPrice, params.GWei))
	}

	table := tablewriter.NewWriter(os.Stdout)
	table.SetAutoWrapText(false)
	data := [][]string{
		{"Node's version", status.Version},
		{"Node's GO runtime version", status.GolangRuntimeVersion},
		{"Node's architecture", status.GOArch},
		{"Node's operating system", status.GOOS},
		{"HTTP Port", w.httpPort},
		{"Controller Address\nFOR REFERENCE - DO NOT SEND TOKENS", addrMap["Controller"].Hex()},
		{"LivepeerToken Address\nFOR REFERENCE - DO NOT SEND TOKENS", addrMap["LivepeerToken"].Hex()},
		{"LivepeerTokenFaucet Address\nFOR REFERENCE - DO NOT SEND TOKENS", addrMap["LivepeerTokenFaucet"].Hex()},
		{account(isOrchestrator) + " Account\nYOUR WALLET FOR ETH & LPT", w.getEthAddr()},
		{"LPT Balance", eth.FormatUnits(lptBal, "LPT")},
		{"ETH Balance", eth.FormatUnits(ethBal, "ETH")},
		{"Max Gas Price", maxGasPriceStr},
		{"Min Gas Price", minGasPriceStr},
	}

	for _, v := range data {
		table.Append(v)
	}

	table.SetAlignment(tablewriter.ALIGN_RIGHT)
	table.SetCenterSeparator("*")
	table.SetRowLine(true)
	table.SetColumnSeparator("|")
	table.Render()

	if isOrchestrator {
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

func account(isOrchestrator bool) string {
	if isOrchestrator {
		return "Orchestrator"
	}
	return "Broadcaster"
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

	table := tablewriter.NewWriter(os.Stdout)
	data := [][]string{
		{"Max Price Per Pixel", formatPricePerPixel(price)},
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

	b_prices, err := w.getBroadcasterPrices()
	if err != nil {
		glog.Errorf("Error getting broadcaster prices: %v", err)
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
		{"Fee Cut (%)", eth.FormatPerc(flipPerc(t.FeeShare))},
		{"Last Reward Round", t.LastRewardRound.String()},
		{"Base price per pixel", formatPricePerPixel(priceInfo)},
		{"Base price for broadcasters", b_prices},
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

	if resp.StatusCode != http.StatusOK {
		return lpTypes.ProtocolParameters{}, fmt.Errorf("http error: %d", resp.StatusCode)
	}

	defer resp.Body.Close()

	result, err := io.ReadAll(resp.Body)
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

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http error: %d", resp.StatusCode)
	}

	defer resp.Body.Close()
	result, err := io.ReadAll(resp.Body)
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
	result, err := io.ReadAll(resp.Body)
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

	result, err := io.ReadAll(resp.Body)
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

	result, err := io.ReadAll(resp.Body)
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

func (w *wizard) minGasPrice() string {
	return httpGet(fmt.Sprintf("http://%v:%v/minGasPrice", w.host, w.httpPort))
}

func (w *wizard) currentBlock() (*big.Int, error) {
	resp, err := http.Get(fmt.Sprintf("http://%v:%v/currentBlock", w.host, w.httpPort))
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http response status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return new(big.Int).SetBytes(body), nil
}

func (w *wizard) getBroadcasterPrices() (string, error) {
	resp, err := http.Get(fmt.Sprintf("http://%v:%v/status", w.host, w.httpPort))

	if err != nil {
		return "", err
	}

	defer resp.Body.Close()
	result, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var status struct {
		BroadcasterPrices map[string]*big.Rat `json:"BroadcasterPrices"`
	}
	err = json.Unmarshal(result, &status)
	if err != nil {
		return "", err
	}

	prices := new(bytes.Buffer)

	for b, p := range status.BroadcasterPrices {
		if b != "default" {
			fmt.Fprintf(prices, "%s: %s\n", b, formatPricePerPixel(p))
		}
	}

	return prices.String(), nil
}

func formatPricePerPixel(price *big.Rat) string {
	if price == nil {
		return "n/a"
	}
	if price.IsInt() {
		return fmt.Sprintf("%v wei/pixel", price.RatString())
	}
	return fmt.Sprintf("%v wei/pixel (%v/%v)", price.FloatString(3), price.Num(), price.Denom())
}
