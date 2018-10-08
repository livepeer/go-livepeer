package main

import (
	"encoding/json"
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
	"github.com/olekukonko/tablewriter"
)

func (w *wizard) stats(showOrchestrator bool) {
	addrMap, err := w.getContractAddresses()
	if err != nil {
		glog.Errorf("Error getting contract addresses: %v", err)
		return
	}

	fmt.Println("+-----------+")
	fmt.Println("|NODE STATS|")
	fmt.Println("+-----------+")

	table := tablewriter.NewWriter(os.Stdout)
	data := [][]string{
		[]string{"HTTP Port", w.httpPort},
		[]string{"Controller Address", addrMap["Controller"].Hex()},
		[]string{"LivepeerToken Address", addrMap["LivepeerToken"].Hex()},
		[]string{"LivepeerTokenFaucet Address", addrMap["LivepeerTokenFaucet"].Hex()},
		[]string{"ETH Account", w.getEthAddr()},
		[]string{"LPT Balance", w.getTokenBalance()},
		[]string{"ETH Balance", w.getEthBalance()},
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
		w.orchestratorEventSubscriptions()
		w.delegatorStats()
	} else {
		w.broadcastStats()
		w.delegatorStats()
	}

	currentRound := w.currentRound()

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
		[]string{"VerificationRate (1 / # of segments)", strconv.Itoa(int(params.VerificationRate))},
		[]string{"VerificationPeriod (Blocks)", params.VerificationPeriod.String()},
		[]string{"SlashingPeriod (Blocks)", params.SlashingPeriod.String()},
		[]string{"FailedVerificationSlashAmount (%)", eth.FormatPerc(params.FailedVerificationSlashAmount)},
		[]string{"MissedVerificationSlashAmount (%)", eth.FormatPerc(params.MissedVerificationSlashAmount)},
		[]string{"DoubleClaimSegmentSlashAmount (%)", eth.FormatPerc(params.DoubleClaimSegmentSlashAmount)},
		[]string{"FinderFee (%)", eth.FormatPerc(params.FinderFee)},
		[]string{"Inflation (%)", eth.FormatPerc(params.Inflation)},
		[]string{"InflationChange (%)", eth.FormatPerc(params.InflationChange)},
		[]string{"TargetBondingRate (%)", eth.FormatPerc(params.TargetBondingRate)},
		[]string{"VerificationCodeHash", params.VerificationCodeHash},
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

	price, transcodingOptions := w.getBroadcastConfig()

	table := tablewriter.NewWriter(os.Stdout)
	data := [][]string{
		[]string{"Deposit", w.getDeposit()},
		[]string{"Broadcast Price Per Segment in Wei", price.String()},
		[]string{"Broadcast Transcoding Options", transcodingOptions},
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

func (w *wizard) orchestratorStats() {
	t, err := w.getOrchestratorInfo()
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
		[]string{"Price Per Segment", eth.FormatUnits(t.PricePerSegment, "ETH")},
		[]string{"Pending Reward Cut (%)", eth.FormatPerc(t.PendingRewardCut)},
		[]string{"Pending Fee Share (%)", eth.FormatPerc(t.PendingFeeShare)},
		[]string{"Pending Price Per Segment", eth.FormatUnits(t.PendingPricePerSegment, "ETH")},
		[]string{"Last Reward Round", t.LastRewardRound.String()},
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

func (w *wizard) orchestratorEventSubscriptions() {
	subMap, err := w.getOrchestratorEventSubscriptions()
	if err != nil {
		glog.Errorf("Error getting orchestrator event subscriptions: %v", err)
		return
	}

	fmt.Println("+--------------------------------+")
	fmt.Println("|ORCHESTRATOR EVENT SUBSCRIPTIONS|")
	fmt.Println("+--------------------------------+")

	table := tablewriter.NewWriter(os.Stdout)
	data := [][]string{
		[]string{"Watching for new rounds to initialize", strconv.FormatBool(subMap["RoundService"])},
		[]string{"Watching for initialized rounds to call reward", strconv.FormatBool(subMap["RewardService"])},
		[]string{"Watching for new jobs", strconv.FormatBool(subMap["JobService"])},
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
	d, err := w.getDelegatorInfo()
	if err != nil {
		glog.Errorf("Error getting delegator info: %v", err)
		return
	}

	fmt.Println("+---------------+")
	fmt.Println("|DELEGATOR STATS|")
	fmt.Println("+---------------+")

	pendingStake := ""
	if d.PendingStake.Int64() == -1 {
		pendingStake = "Please fetch pending stake separately"
	} else {
		pendingStake = d.PendingStake.String()
	}
	pendingFees := ""
	if d.PendingFees.Int64() == -1 {
		pendingFees = "Please fetch pending fees separately"
	} else {
		pendingFees = d.PendingFees.String()
	}
	table := tablewriter.NewWriter(os.Stdout)
	data := [][]string{
		[]string{"Status", d.Status},
		[]string{"Stake", d.BondedAmount.String()},
		[]string{"Collected Fees", d.Fees.String()},
		[]string{"Pending Stake", pendingStake},
		[]string{"Pending Fees", pendingFees},
		[]string{"Delegated Stake", d.DelegatedAmount.String()},
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

func (w *wizard) getOrchestratorInfo() (lpTypes.Transcoder, error) {
	resp, err := http.Get(fmt.Sprintf("http://%v:%v/orchestratorInfo", w.host, w.httpPort))
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

func (w *wizard) getOrchestratorEventSubscriptions() (map[string]bool, error) {
	resp, err := http.Get(fmt.Sprintf("http://%v:%v/orchestratorEventSubscriptions", w.host, w.httpPort))
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	result, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var subMap map[string]bool
	err = json.Unmarshal(result, &subMap)
	if err != nil {
		return nil, err
	}

	return subMap, nil
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
