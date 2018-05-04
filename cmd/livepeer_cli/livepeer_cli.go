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
			httpPort: c.String("http"),
			host:     c.String("host"),
			in:       bufio.NewReader(os.Stdin),
		}
		w.transcoder = w.isTranscoder()
		w.rinkeby = w.onRinkeby()
		w.run()

		return nil
	}
	app.Version = core.LivepeerVersion
	app.Run(os.Args)
}

type wizard struct {
	endpoint   string // Local livepeer node
	httpPort   string
	host       string
	transcoder bool
	rinkeby    bool
	in         *bufio.Reader // Wrapper around stdin to allow reading user input
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

	w.stats(w.transcoder)

	// Basics done, loop ad infinitum about what to do
	for {
		fmt.Println()
		fmt.Println("What would you like to do? (default = stats)")
		fmt.Println(" 1. Get node status")
		fmt.Println(" 2. View protocol parameters")
		fmt.Println(" 3. List registered transcoders")
		fmt.Println(" 4. Print latest jobs")
		fmt.Println(" 5. Invoke \"initialize round\"")
		fmt.Println(" 6. Invoke \"bond\"")
		fmt.Println(" 7. Invoke \"unbond\"")
		fmt.Println(" 8. Invoke \"withdraw stake\" (LPT)")
		fmt.Println(" 9. Invoke \"withdraw fees\" (ETH)")
		fmt.Println(" 10. Invoke \"claim\" (for rewards and fees)")
		fmt.Println(" 11. Invoke \"transfer\" (LPT)")
		if w.transcoder {
			fmt.Println(" 12. Invoke \"reward\"")
			fmt.Println(" 13. Invoke multi-step \"become a transcoder\"")
			fmt.Println(" 14. Set transcoder config")
		} else {
			fmt.Println(" 12. Invoke \"deposit\" (ETH)")
			fmt.Println(" 13. Invoke \"withdraw deposit\" (ETH)")
			fmt.Println(" 14. Set broadcast config")
		}
		if w.rinkeby {
			fmt.Println(" 15. Get test LPT")
			fmt.Println(" 16. Get test ETH")
		}
		w.doCLIOpt(w.read())
	}
}

func (w *wizard) doCLIOpt(choice string) {
	switch choice {
	case "1":
		w.stats(w.transcoder)
		return
	case "2":
		w.protocolStats()
		return
	case "3":
		w.registeredTranscoderStats()
		return
	case "4":
		w.printLast5Jobs()
		return
	case "5":
		w.initializeRound()
		return
	case "6":
		w.bond()
		return
	case "7":
		w.unbond()
		return
	case "8":
		w.withdrawStake()
		return
	case "9":
		w.withdrawFees()
		return
	case "10":
		w.claimRewardsAndFees()
		return
	case "11":
		w.transferTokens()
	}

	if w.transcoder {
		switch choice {
		case "12":
			w.callReward()
		case "13":
			w.activateTranscoder()
			return
		case "14":
			w.setTranscoderConfig()
			return
		}
	} else {
		switch choice {
		case "12":
			w.deposit()
			return
		case "13":
			w.withdraw()
			return
		case "14":
			w.setBroadcastConfig()
			return
		}
	}

	if w.rinkeby {
		switch choice {
		case "15":
			w.requestTokens()
			return
		case "16":
			fmt.Print("For Rinkeby Eth, go to the Rinkeby faucet (https://faucet.rinkeby.io/).")
			w.read()
			return
		}
	}

	log.Error("That's not something I can do")
}

func (w *wizard) onRinkeby() bool {
	nID := httpGet(fmt.Sprintf("http://%v:%v/EthNetworkID", w.host, w.httpPort))
	return nID == "4"
}
