package main

import (
	"fmt"
	"math/big"
	"net/url"
	"strconv"
	"strings"

	"github.com/golang/glog"
	lpcommon "github.com/livepeer/go-livepeer/common"
)

const defaultRPCPort = "8935"

func (w *wizard) isOrchestrator() bool {
	isT := httpGet(fmt.Sprintf("http://%v:%v/IsOrchestrator", w.host, w.httpPort))
	return isT == "true"
}

func myHostPort() string {
	// TODO Fall back to try other services if this one fails. Ask a peer?
	// 	http://myexternalip.com
	// 	http://api.ident.me
	// 	http://whatismyipaddress.com/api
	// 	http://ipinfo.io/ip
	ip := strings.TrimSpace(httpGet("https://api.ipify.org/?format=text"))
	return ip + ":" + defaultRPCPort
}

func (w *wizard) promptOrchestratorConfig() (float64, float64, *big.Int, string) {
	var (
		blockRewardCut  float64
		feeShare        float64
		pricePerSegment *big.Int
	)

	fmt.Printf("Enter block reward cut percentage (default: 10) - ")
	blockRewardCut = w.readDefaultFloat(10.0)

	fmt.Printf("Enter fee share percentage (default: 5) - ")
	feeShare = w.readDefaultFloat(5.0)

	fmt.Printf("Enter price per segment in Wei (default: 1) - ")
	pricePerSegment = w.readDefaultBigInt(big.NewInt(1))

	addr := myHostPort()
	fmt.Printf("Enter the public host:port of node (default: %v)", addr)
	serviceURI := w.readStringAndValidate(func(in string) (string, error) {
		if "" == in {
			in = addr
		}
		in = "https://" + in
		uri, err := url.ParseRequestURI(in)
		if err != nil {
			return "", err
		}
		if uri.Port() == "" {
			return "", fmt.Errorf("Missing Port")
		}
		return in, nil
	})

	return blockRewardCut, feeShare, pricePerSegment, serviceURI
}

func (w *wizard) activateOrchestrator() {
	d, err := w.getDelegatorInfo()
	if err != nil {
		glog.Errorf("Error getting delegator info: %v", err)
		return
	}

	fmt.Printf("Current token balance: %v\n", w.getTokenBalance())
	fmt.Printf("Current bonded amount: %v\n", d.BondedAmount.String())

	blockRewardCut, feeShare, pricePerSegment, serviceURI := w.promptOrchestratorConfig()

	val := url.Values{
		"blockRewardCut":  {fmt.Sprintf("%v", blockRewardCut)},
		"feeShare":        {fmt.Sprintf("%v", feeShare)},
		"pricePerSegment": {fmt.Sprintf("%v", pricePerSegment.String())},
		"serviceURI":      {fmt.Sprintf("%v", serviceURI)},
	}

	if d.BondedAmount.Cmp(big.NewInt(0)) <= 0 || d.DelegateAddress != d.Address {
		fmt.Printf("You must bond to yourself in order to become a orchestrator\n")

		rebond := false

		unbondingLockIDs := w.unbondingLockStats(false)
		if unbondingLockIDs != nil && len(unbondingLockIDs) > 0 {
			fmt.Printf("You have some unbonding locks. Would you like to use one to rebond to yourself? (y/n) - ")

			input := ""
			for {
				input = w.readString()
				if input == "y" || input == "n" {
					break
				}
				fmt.Printf("Enter (y)es or (n)o\n")
			}

			if input == "y" {
				rebond = true

				unbondingLockID := int64(-1)

				for {
					fmt.Printf("Enter the identifier of the unbonding lock you would like to rebond to yourself with - ")
					unbondingLockID = int64(w.readInt())
					if _, ok := unbondingLockIDs[unbondingLockID]; ok {
						break
					}
					fmt.Printf("Must enter a valid unbonding lock ID\n")
				}

				val["unbondingLockId"] = []string{fmt.Sprintf("%v", strconv.FormatInt(unbondingLockID, 10))}
			}
		}

		if !rebond {
			balBigInt, err := lpcommon.ParseBigInt(w.getTokenBalance())
			if err != nil {
				fmt.Printf("Cannot read token balance: %v", w.getTokenBalance())
				return
			}

			amount := big.NewInt(0)
			for amount.Cmp(big.NewInt(0)) == 0 || balBigInt.Cmp(amount) < 0 {
				fmt.Printf("Enter bond amount - ")
				amount = w.readBigInt()
				if balBigInt.Cmp(amount) < 0 {
					fmt.Printf("Must enter an amount smaller than the current balance. ")
				}
				if amount.Cmp(big.NewInt(0)) == 0 && d.BondedAmount.Cmp(big.NewInt(0)) > 0 {
					break
				}
			}

			val["amount"] = []string{fmt.Sprintf("%v", amount.String())}
		}
	}

	httpPostWithParams(fmt.Sprintf("http://%v:%v/activateOrchestrator", w.host, w.httpPort), val)
	// TODO we should confirm if the transaction was actually sent
	fmt.Println("\nTransaction sent. Once confirmed, please restart your node.")
}

func (w *wizard) setOrchestratorConfig() {
	fmt.Printf("Current token balance: %v\n", w.getTokenBalance())

	blockRewardCut, feeShare, pricePerSegment, serviceURI := w.promptOrchestratorConfig()

	val := url.Values{
		"blockRewardCut":  {fmt.Sprintf("%v", blockRewardCut)},
		"feeShare":        {fmt.Sprintf("%v", feeShare)},
		"pricePerSegment": {fmt.Sprintf("%v", pricePerSegment.String())},
		"serviceURI":      {fmt.Sprintf("%v", serviceURI)},
	}

	httpPostWithParams(fmt.Sprintf("http://%v:%v/setOrchestratorConfig", w.host, w.httpPort), val)
	// TODO we should confirm if the transaction was actually sent
	fmt.Println("\nTransaction sent. Once confirmed, please restart your node if the ServiceURI has been reset")
}

func (w *wizard) callReward() {
	t, err := w.getOrchestratorInfo()
	if err != nil {
		fmt.Printf("Error getting orchestrator info: %v\n", err)
		return
	}
	c, err := strconv.ParseInt(w.currentRound(), 10, 64)
	if err != nil {
		fmt.Printf("Error converting current round: %v\n", c)
	}

	if c == t.LastRewardRound.Int64() {
		fmt.Printf("Reward for current round %v already called\n", c)
		return
	}

	fmt.Printf("Calling reward for round %v\n", c)
	httpGet(fmt.Sprintf("http://%v:%v/reward", w.host, w.httpPort))
}
