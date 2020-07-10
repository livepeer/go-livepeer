package main

import (
	"fmt"
	"math/big"
	"net/url"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/golang/glog"
	lpcommon "github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/eth/types"
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

func (w *wizard) promptOrchestratorConfig() (float64, float64, int, int, string) {
	var (
		blockRewardCut float64
		feeShare       float64
	)

	fmt.Printf("Enter block reward cut percentage (default: 10) - ")
	blockRewardCut = w.readDefaultFloat(10.0)

	fmt.Printf("Enter fee share percentage (default: 5) - ")
	feeShare = w.readDefaultFloat(5.0)

	fmt.Println("Enter a transcoding base price in wei per pixels")
	fmt.Println("eg. 1 wei / 10 pixels = 0,1 wei per pixel")
	fmt.Println()
	fmt.Printf("Enter amount of pixels that make up a single unit (default: 1 pixel) ")
	pixelsPerUnit := w.readDefaultInt(1)
	fmt.Printf("Enter the price for %d pixels in Wei (required) ", pixelsPerUnit)
	pricePerUnit := w.readDefaultInt(0)

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

	return blockRewardCut, feeShare, pricePerUnit, pixelsPerUnit, serviceURI
}

func (w *wizard) activateOrchestrator() {
	d, err := w.getDelegatorInfo()
	if err != nil {
		glog.Errorf("Error getting delegator info: %v", err)
		return
	}

	fmt.Printf("Current token balance: %v\n", w.getTokenBalance())
	fmt.Printf("Current bonded amount: %v\n", d.BondedAmount.String())

	val := w.getOrchestratorConfigFormValues()

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

	val := w.getOrchestratorConfigFormValues()

	httpPostWithParams(fmt.Sprintf("http://%v:%v/setOrchestratorConfig", w.host, w.httpPort), val)
	// TODO we should confirm if the transaction was actually sent
	fmt.Println("\nTransaction sent. Once confirmed, please restart your node if the ServiceURI has been reset")
}

func (w *wizard) getOrchestratorConfigFormValues() url.Values {
	blockRewardCut, feeShare, pricePerUnit, pixelsPerUnit, serviceURI := w.promptOrchestratorConfig()

	return url.Values{
		"blockRewardCut": {fmt.Sprintf("%v", blockRewardCut)},
		"feeShare":       {fmt.Sprintf("%v", feeShare)},
		"pricePerUnit":   {fmt.Sprintf("%v", strconv.Itoa(pricePerUnit))},
		"pixelsPerUnit":  {fmt.Sprintf("%v", strconv.Itoa(pixelsPerUnit))},
		"serviceURI":     {fmt.Sprintf("%v", serviceURI)},
	}
}

func (w *wizard) callReward() {
	t, _, err := w.getOrchestratorInfo()
	if err != nil {
		fmt.Printf("Error getting orchestrator info: %v\n", err)
		return
	}
	c, err := w.currentRound()
	if err != nil {
		fmt.Printf("Error converting current round: %v\n", c)
	}

	if c.Cmp(t.LastRewardRound) == 0 {
		fmt.Printf("Reward for current round %v already called\n", c)
		return
	}

	fmt.Printf("Calling reward for round %v\n", c)
	httpGet(fmt.Sprintf("http://%v:%v/reward", w.host, w.httpPort))
}

func (w *wizard) vote() {
	if w.offchain {
		glog.Error("Can not vote in 'offchain' mode")
		return
	}

	fmt.Print("Enter the contract address for the poll you want to vote in -")
	poll := w.readStringAndValidate(func(in string) (string, error) {
		if !ethcommon.IsHexAddress(in) {
			return "", fmt.Errorf("invalid hex address address=%v", in)
		}
		return in, nil
	})

	var (
		confirm = "n"
		choice  = types.VoteChoice(-1)
	)

	for confirm == "n" {
		choice = types.VoteChoice(-1)
		w.showVoteChoices()

		for {
			fmt.Printf("Enter the ID of the choice you want to vote for -")
			choice = types.VoteChoice(w.readInt())
			if choice.IsValid() {
				break
			}
			fmt.Println("Must enter a valid ID")
		}

		fmt.Printf("Are you sure you want to vote \"%v\"? (y/n) -", choice.String())
		confirm = w.readStringYesOrNo()
	}

	data := url.Values{
		"poll":     {poll},
		"choiceID": {fmt.Sprintf("%v", int(choice))},
	}

	result := httpPostWithParams(fmt.Sprintf("http://%v:%v/vote", w.host, w.httpPort), data)

	if result == "" {
		fmt.Println("vote failed")
		return
	}

	fmt.Printf("\nVote success tx=0x%x\n", []byte(result))
}

func (w *wizard) showVoteChoices() {
	wtr := tabwriter.NewWriter(os.Stdout, 0, 8, 1, '\t', 0)
	fmt.Fprintln(wtr, "Identifier\tVoting Choices")
	for _, choice := range types.VoteChoices {
		fmt.Fprintf(wtr, "%v\t%v\n", int(choice), choice.String())
	}
	wtr.Flush()
}
