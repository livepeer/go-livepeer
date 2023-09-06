package main

import (
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"strconv"

	"github.com/golang/glog"

	"github.com/ethereum/go-ethereum/common"
	lpcommon "github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/eth"
	lpTypes "github.com/livepeer/go-livepeer/eth/types"
	"github.com/olekukonko/tablewriter"
)

func (w *wizard) registeredOrchestratorStats() map[int]common.Address {
	orchestrators, err := w.getRegisteredOrchestrators()
	if err != nil {
		glog.Errorf("Error getting registered orchestrators: %v", err)
		return nil
	}

	orchestratorIDs := make(map[int]common.Address)
	nextId := 0

	fmt.Println("+------------------------+")
	fmt.Println("|REGISTERED ORCHESTRATORS|")
	fmt.Println("+------------------------+")

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"ID", "Address", "Active", "Delegated Stake", "Reward Cut (%)", "Fee Cut (%)", "Service URI", "Price Per Pixel"})
	for _, t := range orchestrators {
		table.Append([]string{
			strconv.FormatInt(int64(nextId), 10),
			t.Address.Hex(),
			strconv.FormatBool(t.Active),
			eth.FormatUnits(t.DelegatedStake, "LPT"),
			eth.FormatPerc(t.RewardCut),
			eth.FormatPerc(flipPerc(t.FeeShare)),
			t.ServiceURI,
			t.PricePerPixel.FloatString(3),
		})

		orchestratorIDs[nextId] = t.Address
		nextId++
	}

	table.Render()

	return orchestratorIDs
}

func (w *wizard) getRegisteredOrchestrators() ([]lpTypes.Transcoder, error) {
	resp, err := http.Get(fmt.Sprintf("http://%v:%v/registeredOrchestrators", w.host, w.httpPort))
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

	var orchestrators []lpTypes.Transcoder
	err = json.Unmarshal(result, &orchestrators)
	if err != nil {
		return nil, err
	}

	return orchestrators, nil
}

func (w *wizard) unbondingLockStats(withdrawable bool) map[int64]bool {
	unbondingLocks, err := w.getUnbondingLocks(withdrawable)
	if err != nil {
		glog.Errorf("Error getting unbonding locks: %v", err)
		return nil
	}

	unbondingLockIDs := make(map[int64]bool)

	if len(unbondingLocks) == 0 {
		return unbondingLockIDs
	}

	fmt.Println("+---------------+")
	fmt.Println("|UNBONDING LOCKS|")
	fmt.Println("+---------------+")

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"ID", "Amount", "Withdraw Round"})

	for _, u := range unbondingLocks {
		table.Append([]string{
			strconv.FormatInt(u.ID, 10),
			eth.FormatUnits(u.Amount, "LPT"),
			strconv.FormatInt(u.WithdrawRound, 10),
		})

		unbondingLockIDs[u.ID] = true
	}

	table.Render()

	return unbondingLockIDs
}

func (w *wizard) getUnbondingLocks(withdrawable bool) ([]lpcommon.DBUnbondingLock, error) {
	var url string
	if withdrawable {
		url = fmt.Sprintf("http://%v:%v/unbondingLocks?withdrawable=true", w.host, w.httpPort)
	} else {
		url = fmt.Sprintf("http://%v:%v/unbondingLocks", w.host, w.httpPort)
	}
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	result, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var unbondingLocks []lpcommon.DBUnbondingLock
	err = json.Unmarshal(result, &unbondingLocks)
	if err != nil {
		return nil, err
	}

	return unbondingLocks, nil
}

func (w *wizard) bond() {
	orchestratorIds := w.registeredOrchestratorStats()
	var tAddr common.Address
	if orchestratorIds == nil {
		fmt.Printf("Enter the address of the orchestrator you would like to bond to - ")
		strAddr := w.readString()
		if err := tAddr.UnmarshalText([]byte(strAddr)); err != nil {
			fmt.Println(err)
			return
		}
	} else {
		fmt.Printf("Enter the identifier of the orchestrator you would like to bond to - ")
		id := w.readInt()
		tAddr = orchestratorIds[id]
	}

	balBigInt, err := lpcommon.ParseBigInt(w.getTokenBalance())
	if err != nil {
		fmt.Printf("Cannot read token balance: %v", w.getTokenBalance())
		return
	}

	amount := big.NewInt(0)
	for amount.Cmp(big.NewInt(0)) == 0 || balBigInt.Cmp(amount) < 0 {
		amount = w.readBigInt("Enter bond amount")
		if amount.Cmp(big.NewInt(0)) == 0 {
			break
		}
		if balBigInt.Cmp(amount) < 0 {
			fmt.Printf("Must enter an amount smaller than the current balance. ")
		}
	}

	val := url.Values{
		"amount": {fmt.Sprintf("%v", amount.String())},
		"toAddr": {fmt.Sprintf("%v", tAddr.Hex())},
	}

	httpPostWithParams(fmt.Sprintf("http://%v:%v/bond", w.host, w.httpPort), val)
}

func (w *wizard) rebond() {
	dInfo, err := w.getDelegatorInfo()
	if err != nil {
		glog.Errorf("Error getting delegator info: %v", err)
		return
	}

	fmt.Printf("Current Bonded Amount: %v\n", eth.FormatUnits(dInfo.BondedAmount, "LPT"))
	if (dInfo.DelegateAddress != common.Address{}) {
		fmt.Printf("Current Delegate: %v\n", dInfo.DelegateAddress.Hex())
	}

	unbondingLockIDs := w.unbondingLockStats(false)

	if unbondingLockIDs == nil || len(unbondingLockIDs) == 0 {
		fmt.Printf("No unbonding locks")
		return
	}

	unbondingLockID := int64(-1)

	for {
		fmt.Printf("Enter the identifier of the unbonding lock you would like to rebond with - ")
		unbondingLockID = int64(w.readInt())
		if _, ok := unbondingLockIDs[unbondingLockID]; ok {
			break
		}
		fmt.Printf("Must enter a valid unbonding lock ID\n")
	}

	val := url.Values{
		"unbondingLockId": {fmt.Sprintf("%v", strconv.FormatInt(unbondingLockID, 10))},
	}

	if dInfo.Status == "Unbonded" {
		fmt.Printf("You are unbonded - you will need to choose an address to rebond to.\n")

		var toAddr common.Address

		orchestratorIds := w.registeredOrchestratorStats()

		if orchestratorIds == nil {
			fmt.Printf("Enter the address of the orchestrator you would like to rebond to - ")
			strAddr := w.readString()
			if err := toAddr.UnmarshalText([]byte(strAddr)); err != nil {
				fmt.Println(err)
				return
			}
		} else {
			fmt.Printf("Enter the identifier of the orchestrator you would like to rebond to - ")
			orchestratorID := w.readInt()
			toAddr = orchestratorIds[orchestratorID]
		}

		val["toAddr"] = []string{fmt.Sprintf("%v", toAddr.Hex())}
	}

	httpPostWithParams(fmt.Sprintf("http://%v:%v/rebond", w.host, w.httpPort), val)
}

func (w *wizard) unbond() {
	dInfo, err := w.getDelegatorInfo()
	if err != nil {
		glog.Errorf("Error getting delegator info: %v", err)
		return
	}

	if dInfo.BondedAmount.Cmp(big.NewInt(0)) < 0 {
		fmt.Printf("You are not bonded\n")
		return
	}

	fmt.Printf("Current Bonded Amount: %v\n", eth.FormatUnits(dInfo.BondedAmount, "LPT"))
	fmt.Printf("Current Delegate: %v\n", dInfo.DelegateAddress.Hex())

	fmt.Printf("Would you like to fully unbond? (y/n) - ")

	input := ""
	for {
		input = w.readString()
		if input == "y" || input == "n" {
			break
		}
		fmt.Printf("Enter (y)es or (n)o \n")
	}

	var amount *big.Int
	if input == "y" {
		amount = dInfo.BondedAmount
	} else {
		amount = big.NewInt(0)
	}

	for amount.Cmp(big.NewInt(0)) == 0 || dInfo.BondedAmount.Cmp(amount) < 0 {
		amount = w.readBigInt("Enter unbond amount")
		if dInfo.BondedAmount.Cmp(amount) < 0 {
			fmt.Printf("Must enter an amount less than or equal to the current bonded amount.")
		}
	}

	val := url.Values{
		"amount": {fmt.Sprintf("%v", amount.String())},
	}

	httpPostWithParams(fmt.Sprintf("http://%v:%v/unbond", w.host, w.httpPort), val)
}

func (w *wizard) withdrawStake() {
	dInfo, err := w.getDelegatorInfo()
	if err != nil {
		glog.Errorf("Error getting delegator info: %v", err)
		return
	}

	fmt.Printf("Current Bonded Amount: %v\n", eth.FormatUnits(dInfo.BondedAmount, "LPT"))
	if (dInfo.DelegateAddress != common.Address{}) {
		fmt.Printf("Current Delegate: %v\n", dInfo.DelegateAddress.Hex())
	}

	unbondingLockIDs := w.unbondingLockStats(true)

	if unbondingLockIDs == nil || len(unbondingLockIDs) == 0 {
		fmt.Printf("No withdrawable unbonding locks")
		return
	}

	unbondingLockID := int64(-1)

	for {
		fmt.Printf("Enter the identifier of the unbonding lock you would like to withdraw with - ")
		unbondingLockID = int64(w.readInt())
		if _, ok := unbondingLockIDs[unbondingLockID]; ok {
			break
		}
		fmt.Printf("Must enter a valid unbonding lock ID\n")
	}

	val := url.Values{
		"unbondingLockId": {fmt.Sprintf("%v", strconv.FormatInt(unbondingLockID, 10))},
	}

	httpPostWithParams(fmt.Sprintf("http://%v:%v/withdrawStake", w.host, w.httpPort), val)
}

func (w *wizard) withdrawFees() {
	dInfo, err := w.getDelegatorInfo()
	if err != nil {
		glog.Errorf("Error getting delegator info: %v", err)
		return
	}

	val := url.Values{
		"amount": {fmt.Sprintf("%v", dInfo.PendingFees.String())},
	}

	httpPostWithParams(fmt.Sprintf("http://%v:%v/withdrawFees", w.host, w.httpPort), val)
}
