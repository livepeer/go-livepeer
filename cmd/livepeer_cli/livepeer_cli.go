package main

import (
	"bufio"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/ethereum/go-ethereum/log"
	"github.com/livepeer/go-livepeer/core"
	"github.com/urfave/cli"
)

func main() {
	app := cli.NewApp()
	app.Name = "livepeer-cli"
	app.Usage = "interact with local Livepeer node"
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "http",
			Usage: "local cli port",
			Value: "7935",
		},
		cli.StringFlag{
			Name:  "host",
			Usage: "host for the Livepeer node",
			Value: "localhost",
		},
		cli.IntFlag{
			Name:  "loglevel",
			Value: 4,
			Usage: "log level to emit to the screen",
		},
	}
	app.Action = func(c *cli.Context) error {
		if c.Bool("version") {
			fmt.Println("Version: " + core.LivepeerVersion)
		}

		// Set up the logger to print everything and the random generator
		log.Root().SetHandler(log.LvlFilterHandler(log.Lvl(c.Int("loglevel")), log.StreamHandler(os.Stdout, log.TerminalFormat(true))))
		rand.Seed(time.Now().UnixNano())

		// Start the wizard and relinquish control
		w := &wizard{
			endpoint: fmt.Sprintf("http://%v:%v/status", c.String("host"), c.String("http")),
			httpPort: c.String("http"),
			host:     c.String("host"),
			in:       bufio.NewReader(os.Stdin),
		}
		w.orchestrator = w.isOrchestrator()
		w.redeemer = w.isRedeemer()
		w.checkNet()
		w.run()

		return nil
	}
	app.Version = core.LivepeerVersion
	// flag.Parse()
	app.Run(os.Args)
}

type wizard struct {
	endpoint     string // Local livepeer node
	httpPort     string
	host         string
	orchestrator bool
	redeemer     bool
	testnet      bool
	offchain     bool
	in           *bufio.Reader // Wrapper around stdin to allow reading user input
}

type wizardOpt struct {
	desc            string
	invoke          func()
	testnet         bool
	orchestrator    bool
	notOrchestrator bool
}

func (w *wizard) initializeOptions() []wizardOpt {
	options := []wizardOpt{
		{desc: "Get node status", invoke: func() { w.stats(w.orchestrator) }},
		{desc: "View protocol parameters", invoke: w.protocolStats},
		{desc: "List registered orchestrators", invoke: func() { w.registeredOrchestratorStats() }},
		{desc: "Invoke \"initialize round\"", invoke: w.initializeRound},
		{desc: "Invoke \"bond\"", invoke: w.bond},
		{desc: "Invoke \"unbond\"", invoke: w.unbond},
		{desc: "Invoke \"rebond\"", invoke: w.rebond},
		{desc: "Invoke \"withdraw stake\" (LPT)", invoke: w.withdrawStake},
		{desc: "Invoke \"withdraw fees\" (ETH)", invoke: w.withdrawFees},
		{desc: "Invoke \"transfer\" (LPT)", invoke: w.transferTokens},
		{desc: "Invoke \"reward\"", invoke: w.callReward, orchestrator: true},
		{desc: "Invoke multi-step \"become an orchestrator\"", invoke: w.activateOrchestrator, orchestrator: true},
		{desc: "Set orchestrator config", invoke: w.setOrchestratorConfig, orchestrator: true},
		{desc: "Invoke \"deposit broadcasting funds\" (ETH)", invoke: w.deposit, notOrchestrator: true},
		{desc: "Invoke \"unlock broadcasting funds\"", invoke: w.unlock, notOrchestrator: true},
		{desc: "Invoke \"cancel unlock of broadcasting funds\"", invoke: w.cancelUnlock, notOrchestrator: true},
		{desc: "Invoke \"withdraw broadcasting funds\"", invoke: w.withdraw, notOrchestrator: true},
		{desc: "Set broadcast config", invoke: w.setBroadcastConfig, notOrchestrator: true},
		{desc: "Set maximum Ethereum gas price", invoke: w.setMaxGasPrice},
		{desc: "Set minimum Ethereum gas price", invoke: w.setMinGasPrice},
		{desc: "Get test LPT", invoke: w.requestTokens, testnet: true},
		{desc: "Get test ETH", invoke: func() {
			fmt.Print("For Rinkeby Eth, go to the Rinkeby faucet (https://faucet.rinkeby.io/).")
			w.read()
		}, testnet: true},
		{desc: "Sign a message", invoke: w.signMessage},
		{desc: "Sign typed data", invoke: w.signTypedData},
		{desc: "Vote in a poll", invoke: w.vote, orchestrator: true},
		{desc: "Set max ticket face value", invoke: w.setMaxFaceValue, orchestrator: true},
		{desc: "Set price for broadcaster", invoke: w.setPriceForBroadcaster, orchestrator: true},
		{desc: "Set maximum sessions", invoke: w.setMaxSessions, orchestrator: true, notOrchestrator: false},
		{desc: "Exit", invoke: func() {
			fmt.Println("Goodbye, my friend")
			os.Exit(0)
		}},
	}
	return options
}

func (w *wizard) filterOptions(options []wizardOpt) []wizardOpt {
	isOrchestratorOrRedeemer := w.orchestrator || w.redeemer
	filtered := make([]wizardOpt, 0, len(options))
	for _, opt := range options {
		if opt.testnet && !w.testnet {
			continue
		}
		if !opt.orchestrator && !opt.notOrchestrator || isOrchestratorOrRedeemer && opt.orchestrator || !isOrchestratorOrRedeemer && opt.notOrchestrator {
			filtered = append(filtered, opt)
		}
	}
	return filtered
}

func (w *wizard) run() {
	// Make sure there is a local node running
	_, err := http.Get(w.endpoint)
	if err != nil {
		log.Error(fmt.Sprintf("Cannot find local node. Is your node running on http:%v?", w.httpPort))
		return
	}

	fmt.Println("+-----------------------------------------------------------+")
	fmt.Println("| Welcome to livepeer-cli, your Livepeer command line tool  |")
	fmt.Println("|                                                           |")
	fmt.Println("| This tool lets you interact with a local Livepeer node    |")
	fmt.Println("| and participate in the Livepeer protocol without the	    |")
	fmt.Println("| hassle that it would normally entail.                     |")
	fmt.Println("|                                                           |")
	fmt.Println("+-----------------------------------------------------------+")
	fmt.Println()

	w.stats(w.orchestrator)
	options := w.filterOptions(w.initializeOptions())

	// Basics done, loop ad infinitum about what to do
	for {
		fmt.Println()
		fmt.Println("What would you like to do? (default = stats)")
		for i, opt := range options {
			fmt.Printf("%d. %s\n", i+1, opt.desc)
		}
		w.doCLIOpt(w.read(), options)
	}
}

func (w *wizard) doCLIOpt(choice string, options []wizardOpt) {
	index, err := strconv.ParseInt(choice, 10, 64)
	index--
	if err == nil && index >= 0 && index < int64(len(options)) {
		options[index].invoke()
		return
	}
	log.Error("That's not something I can do")
}

var RinkebyChainID = "4"
var DevenvChainID = "54321"
var ArbitrumTestnetChainID = "421611"

func (w *wizard) checkNet() {
	nID := httpGet(fmt.Sprintf("http://%v:%v/EthChainID", w.host, w.httpPort))
	w.testnet = nID == RinkebyChainID || nID == DevenvChainID || nID == ArbitrumTestnetChainID
	w.offchain = nID == "0"
}
