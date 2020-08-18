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

	table := tablewriter.NewWriter(os.Stdout)
	data := [][]string{
		[]string{"Node's version", status.Version},
		[]string{"Node's GO runtime version", status.GolangRuntimeVersion},
		[]string{"Node's architecture", status.GOArch},
		[]string{"Node's operating system", status.GOOS},
		[]string{"HTTP Port", w.httpPort},
		[]string{"Controller Address", addrMap["Controller"].Hex()},
		[]string{"LivepeerToken Address", addrMap["LivepeerToken"].Hex()},
		[]string{"LivepeerTokenFaucet Address", addrMap["LivepeerTokenFaucet"].Hex()},
		[]string{"ETH Account", w.getEthAddr()},
		[]string{"LPT Balance", eth.FormatUnits(lptBal, "LPT")},
		[]string{"ETH Balance", eth.FormatUnits(ethBal, "ETH")},
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
		[]string{"Protocol Paused", fmt.Sprintf("%t", params.Paused)},
		[]string{"Max # Active Orchestrators", params.NumActiveTranscoders.String()},
		[]string{"RoundLength (Blocks)", params.RoundLength.String()},
		[]string{"RoundLockAmount (%)", eth.FormatPerc(params.RoundLockAmount)},
		[]string{"UnbondingPeriod (Rounds)", strconv.Itoa(int(params.UnbondingPeriod))},
		[]string{"Inflation (%)", eth.FormatPercMinter(params.Inflation)},
		[]string{"InflationChange (%)", eth.FormatPercMinter(params.InflationChange)},
		[]string{"TargetBondingRate (%)", eth.FormatPercMinter(params.TargetBondingRate)},
		[]string{"Total Bonded", eth.FormatUnits(params.TotalBonded, "LPT")},
		[]string{"Total Supply", eth.FormatUnits(params.TotalSupply, "LPT")},
		[]string{"Current Participation Rate (%)", currentParticipationRate.String()},
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
		[]string{"Max Price Per Pixel", priceString},
		[]string{"Broadcast Transcoding Options", transcodingOptions},
		[]string{"Deposit", eth.FormatUnits(sender.Deposit, "ETH")},
		[]string{"Reserve", eth.FormatUnits(sender.Reserve.FundsRemaining, "ETH")},
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
		[]string{"Status", t.Status},
		[]string{"Active", strconv.FormatBool(t.Active)},
		[]string{"Service URI", t.ServiceURI},
		[]string{"Delegated Stake", eth.FormatUnits(t.DelegatedStake, "LPT")},
		[]string{"Reward Cut (%)", eth.FormatPerc(t.RewardCut)},
		[]string{"Fee Share (%)", eth.FormatPerc(t.FeeShare)},
		[]string{"Last Reward Round", t.LastRewardRound.String()},
		[]string{"Base price per pixel", fmt.Sprintf("%v wei / %v pixels", priceInfo.Num(), priceInfo.Denom())},
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
		[]string{"Status", d.Status},
		[]string{"Stake", eth.FormatUnits(d.BondedAmount, "LPT")},
		[]string{"Collected Fees", eth.FormatUnits(d.Fees, "ETH")},
		[]string{"Pending Stake", pendingStake},
		[]string{"Pending Fees", pendingFees},
		[]string{"Delegated Stake", eth.FormatUnits(d.DelegatedAmount, "LPT")},
		[]string{"Delegate Address", d.DelegateAddress.Hex()},
		[]string{"Last Claim Round", d.LastClaimRound.String()},
		[]string{"Start Round", d.StartRound.String()},
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

func (w *wizard) getGasPrice() string {
	g := httpGet(fmt.Sprintf("http://%v:%v/gasPrice", w.host, w.httpPort))
	if g == "" {
		g = "Unknown"
	} else if g == "0" {
		g = "automatic"
	}
	return g
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
