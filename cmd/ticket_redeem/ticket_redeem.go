package main

import (
	"context"
	"flag"
	"fmt"
	"math/big"
	"os"
	"os/user"
	"path/filepath"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/eth"
	"github.com/livepeer/go-livepeer/eth/contracts"
	"github.com/livepeer/go-livepeer/pm"
)

const ticketValidityPeriod = 2
const txTimeout = 10 * time.Minute

func main() {
	flag.Set("logtostderr", "true")
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

	datadir := flag.String("datadir", "", "Data directory for the node")

	ethUrl := flag.String("ethUrl", "", "Ethereum node JSON-RPC URL")
	ethAcctAddr := flag.String("ethAcctAddr", "", "Existing Eth account address")
	ethPassword := flag.String("ethPassword", "", "Password for existing Eth account address")

	brokerAddr := flag.String("brokerAddr", "", "ETH address of TicketBroker contract")
	broadcaster := flag.String("broadcaster", "", "ETH address of broadcaster")
	currentRound := flag.Int64("currentRound", 0, "Current round of the Livepeer protocol")

	flag.Parse()

	keystoreDir := filepath.Join(*datadir, "keystore")

	usr, err := user.Current()
	if err != nil {
		glog.Fatalf("Cannot find current user: %v", err)
	}

	if *datadir == "" {
		homedir := os.Getenv("HOME")
		if homedir == "" {
			homedir = usr.HomeDir
		}
		*datadir = filepath.Join(homedir, ".lpData", "mainnet")
	}

	if *ethUrl == "" {
		glog.Fatal("Need to specify an Ethereum node JSON-RPC URL using -ethUrl")
	}

	backend, err := ethclient.Dial(*ethUrl)
	if err != nil {
		glog.Fatalf("Failed to connect to Ethereum client: %v", err)
	}

	chainID, err := backend.ChainID(context.Background())
	if err != nil {
		glog.Fatalf("Failed to get chain ID: %v", err)
	}

	am, err := eth.NewAccountManager(ethcommon.HexToAddress(*ethAcctAddr), keystoreDir, chainID)
	if err != nil {
		glog.Fatalf("Error creating Ethereum account manager: %v", err)
	}

	if err := am.Unlock(*ethPassword); err != nil {
		glog.Fatalf("Error unlocking Ethereum account: %v", err)
	}

	broker, err := contracts.NewTicketBroker(ethcommon.HexToAddress(*brokerAddr), backend)
	if err != nil {
		glog.Fatalf("Error creating TicketBroker binding: %v", err)
	}

	opts, err := am.CreateTransactOpts(0)
	if err != nil {
		glog.Fatalf("Error creating transact opts: %v", err)
	}

	ticketBrokerSession := &contracts.TicketBrokerSession{
		Contract:     broker,
		TransactOpts: *opts,
	}

	db, err := common.InitDB(*datadir + "/lpdb.sqlite3")
	if err != nil {
		glog.Fatalf("Error opening DB: %v", err)
	}
	defer db.Close()

	minCreationRound := *currentRound - ticketValidityPeriod
	ticket, err := db.SelectEarliestWinningTicket(ethcommon.HexToAddress(*broadcaster), minCreationRound)
	if err != nil {
		glog.Fatalf("Error selecting earliest winning ticket: %v", err)
	}

	if ticket == nil {
		glog.Fatalf("No tickets to redeem")
	}

	// 1. Create tx without sending
	ticketBrokerSession.TransactOpts.NoSend = true
	tx, err := redeemWinningTicket(ticketBrokerSession, ticket.Ticket, ticket.Sig, ticket.RecipientRand)
	if err != nil {
		glog.Fatalf("Error creating ticket redemption tx: %v", err)
	}

	// 2. Log tx data
	fmt.Printf("maxPriorityFeePerGas = %v\n", tx.GasTipCap())
	fmt.Printf("maxFeePerGas = %v\n", tx.GasFeeCap())
	fmt.Printf("This tx will cost at most %v\n", tx.Cost())

	fmt.Println("Enter 'y' if you would like to send this tx")

	var resp string
	fmt.Scanln(&resp)

	if resp != "y" {
		glog.Infof("Tx not sent")
		return
	}

	// 3. Send tx
	if err := backend.SendTransaction(context.Background(), tx); err != nil {
		glog.Fatalf("Error sending tx: %v", err)
	}

	glog.Infof("Submitted ticket redemption tx %v", tx.Hash())
	glog.Infof("Waiting for tx to be mined...")

	ctx, cancel := context.WithTimeout(context.Background(), txTimeout)
	defer cancel()
	receipt, err := bind.WaitMined(ctx, backend, tx)
	if err != nil {
		glog.Fatalf("Error waiting for tx to be mined %v", err)
	}

	defer func() {
		if err := db.MarkWinningTicketRedeemed(ticket, tx.Hash()); err != nil {
			glog.Fatalf("Error marking ticket as redeemed for tx %v", tx.Hash())
		}
	}()

	if receipt.Status == uint64(0) {
		glog.Fatalf("Tx %v failed", tx.Hash())
	}

	glog.Infof("Tx %v succeeded", tx.Hash())
}

func redeemWinningTicket(ticketBrokerSession *contracts.TicketBrokerSession, ticket *pm.Ticket, sig []byte, recipientRand *big.Int) (*types.Transaction, error) {
	var recipientRandHash [32]byte
	copy(recipientRandHash[:], ticket.RecipientRandHash.Bytes()[:32])

	return ticketBrokerSession.RedeemWinningTicket(
		contracts.MTicketBrokerCoreTicket{
			Recipient:         ticket.Recipient,
			Sender:            ticket.Sender,
			FaceValue:         ticket.FaceValue,
			WinProb:           ticket.WinProb,
			SenderNonce:       new(big.Int).SetUint64(uint64(ticket.SenderNonce)),
			RecipientRandHash: recipientRandHash,
			AuxData:           ticket.AuxData(),
		},
		sig,
		recipientRand,
	)
}
