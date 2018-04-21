package main

import (
	"bufio"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"time"

	"github.com/ethereum/go-ethereum/log"
	"github.com/livepeer/go-livepeer/core"
	"gopkg.in/urfave/cli.v1"
)

func main() {
	app := cli.NewApp()
	app.Name = "livepeer-cli"
	app.Usage = "interact with local Livepeer node"
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "http",
			Usage: "local http port",
			Value: "8935",
		},
		cli.StringFlag{
			Name:  "rtmp",
			Usage: "local rtmp port",
			Value: "1935",
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
			rtmpPort: c.String("rtmp"),
			httpPort: c.String("http"),
			host:     c.String("host"),
			in:       bufio.NewReader(os.Stdin),
		}
		w.transcoder = w.isTranscoder()
		w.run()

		return nil
	}
	app.Version = core.LivepeerVersion
	app.Run(os.Args)
}

type wizard struct {
	endpoint   string // Local livepeer node
	rtmpPort   string
	httpPort   string
	host       string
	transcoder bool
	in         *bufio.Reader // Wrapper around stdin to allow reading user input
}

func (w *wizard) run() {
	// Make sure there is a local node running
	_, err := http.Get(w.endpoint)
	if err != nil {
		log.Error(fmt.Sprintf("Cannot find local node. Is your node running on http:%v and rtmp:%v?", w.httpPort, w.rtmpPort))
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

	w.stats(w.transcoder)

	// Basics done, loop ad infinitum about what to do
	for {
		fmt.Println()
		fmt.Println("What would you like to do? (default = stats)")
		fmt.Println(" 1. Get node status")
		fmt.Println(" 2. View protocol parameters")
		fmt.Println(" 3. Initialize round")
		fmt.Println(" 4. Bond")
		fmt.Println(" 5. Unbond")
		fmt.Println(" 6. Withdraw stake (LPT)")
		fmt.Println(" 7. Withdraw fees (ETH)")
		fmt.Println(" 8. Claim rewards and fees")
		fmt.Println(" 9. Transfer LPT")
		fmt.Println(" 10. Get test LPT")
		fmt.Println(" 11. Get test ETH")
		fmt.Println(" 12. List registered transcoders")
		fmt.Println(" 13. Print latest jobs")

		if w.transcoder {
			fmt.Println(" 14. Manually invoke \"reward\"")
			fmt.Println(" 15. Become a transcoder")
			fmt.Println(" 16. Set transcoder config")

			w.doCLIOpt(w.read(), true)
		} else {
			fmt.Println(" 14. Deposit (ETH)")
			fmt.Println(" 15. Withdraw deposit (ETH)")
			fmt.Println(" 16. Broadcast video")
			fmt.Println(" 17. Stream video")
			fmt.Println(" 18. Set broadcast config")

			w.doCLIOpt(w.read(), false)
		}
	}
}

func (w *wizard) doCLIOpt(choice string, transcoder bool) {
	switch choice {
	case "1":
		w.stats(w.transcoder)
		return
	case "2":
		w.protocolStats()
		return
	case "3":
		w.initializeRound()
		return
	case "4":
		w.bond()
		return
	case "5":
		w.unbond()
		return
	case "6":
		w.withdrawStake()
		return
	case "7":
		w.withdrawFees()
		return
	case "8":
		w.claimRewardsAndFees()
		return
	case "9":
		w.transferTokens()
		return
	case "10":
		w.requestTokens()
		return
	case "11":
		fmt.Print("For Rinkeby, go to the Rinkeby faucet (https://faucet.rinkeby.io/).  For the Livepeer testnet, go to the Livepeer faucet (eth-testnet.livepeer.org). (enter to continue)")
		w.read()
		return
	case "12":
		w.registeredTranscoderStats()
		return
	case "13":
		w.printLast5Jobs()
		return
	}

	if transcoder {
		switch choice {
		case "14":
			w.callReward()
		case "15":
			w.activateTranscoder()
			return
		case "16":
			w.setTranscoderConfig()
			return
		}
	} else {
		switch choice {
		case "14":
			w.deposit()
			return
		case "15":
			w.withdraw()
			return
		case "16":
			w.broadcast()
			return
		case "17":
			w.stream()
			return
		case "18":
			w.setBroadcastConfig()
			return
		}
	}

	log.Error("That's not something I can do")
}
