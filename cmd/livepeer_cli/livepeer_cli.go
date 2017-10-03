package main

import (
	"bufio"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"time"

	"github.com/ethereum/go-ethereum/log"
	"gopkg.in/urfave/cli.v1"
)

const (
	NodeStatus                = "1"
	DepositToken              = "2"
	BroadcastVideo            = "3"
	StreamVideo               = "4"
	SetTranscoderConfig       = "5"
	SetBroadcastConfig        = "6"
	Bond                      = "7"
	UnBond                    = "8"
	WithdrawBond              = "9"
	BecomeTranscoder          = "10"
	GetTestToken              = "11"
	GetTestEther              = "12"
	ListRegisteredTranscoders = "13"
	InvalidOption             = "20"
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
		cli.BoolFlag{
			Name:  "transcoder",
			Usage: "transcoder on off flag",
		},
	}
	app.Action = func(c *cli.Context) error {
		// Set up the logger to print everything and the random generator
		log.Root().SetHandler(log.LvlFilterHandler(log.Lvl(c.Int("loglevel")), log.StreamHandler(os.Stdout, log.TerminalFormat(true))))
		rand.Seed(time.Now().UnixNano())

		// Start the wizard and relinquish control
		w := &wizard{
			endpoint:   fmt.Sprintf("http://%v:%v/status", c.String("host"), c.String("http")),
			rtmpPort:   c.String("rtmp"),
			httpPort:   c.String("http"),
			host:       c.String("host"),
			transcoder: c.Bool("transcoder"),
			in:         bufio.NewReader(os.Stdin),
		}
		w.run()

		return nil
	}
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

	w.stats(false)
	// Basics done, loop ad infinitum about what to do
	for {

		fmt.Println()
		fmt.Println("What would you like to do? (default = stats)")
		fmt.Println(" 1. Get node status")
		var choice string

		if w.transcoder {
			fmt.Println(" 2. Set transcoder config")
			fmt.Println(" 3. Become a transcoder")
			fmt.Println(" 4. Get test Livepeer Token")
			fmt.Println(" 5. Get test Ether")
			fmt.Println(" 6. List registered transcoders")
			choice = w.read()
			switch {
			case choice == "2":
				choice = SetTranscoderConfig
				break
			case choice == "3":
				choice = BecomeTranscoder
				break
			case choice == "4":
				choice = GetTestToken
				break
			case choice == "5":
				choice = GetTestEther
				break
			case choice == "6":
				choice = ListRegisteredTranscoders
				break
			default:
				choice = InvalidOption

			}

		} else {

			fmt.Println(" 2. Deposit token")
			fmt.Println(" 3. Broadcast video")
			fmt.Println(" 4. Stream video")
			fmt.Println(" 5. Set broadcast config")
			fmt.Println(" 6. Bond")
			fmt.Println(" 7. Unbond")
			fmt.Println(" 8. Withdraw bond")
			fmt.Println(" 9. Get test Livepeer Token")
			fmt.Println(" 10. Get test Ether")
			fmt.Println(" 11. List registered transcoders")

			choice = w.read()
			switch {
			case choice == "2":
				choice = DepositToken
				break
			case choice == "3":
				choice = BroadcastVideo
				break
			case choice == "4":
				choice = StreamVideo
				break
			case choice == "5":
				choice = SetBroadcastConfig
				break
			case choice == "6":
				choice = Bond
				break
			case choice == "7":
				choice = UnBond
				break
			case choice == "8":
				choice = WithdrawBond
				break
			case choice == "9":
				choice = GetTestToken
				break
			case choice == "10":
				choice = GetTestEther
				break
			case choice == "11":
				choice = ListRegisteredTranscoders
				break
			default:
				choice = InvalidOption
			}

		}
		switch {
		case choice == "" || choice == NodeStatus:
			w.stats(false)
		case choice == DepositToken:
			w.deposit()
		case choice == BroadcastVideo:
			w.broadcast()
		case choice == StreamVideo:
			w.stream()
		case choice == SetTranscoderConfig:
			w.setTranscoderConfig()
		case choice == SetBroadcastConfig:
			w.setBroadcastConfig()
		case choice == Bond:
			w.bond()
		case choice == UnBond:
			w.unbond()
		case choice == WithdrawBond:
			w.withdrawBond()
		case choice == BecomeTranscoder:
			w.activateTranscoder()
		case choice == GetTestToken:
			w.requestTokens()
		case choice == GetTestEther:
			fmt.Print("Go to eth-testnet.livepeer.org and use the faucet. (enter to continue)")
			w.read()
		case choice == ListRegisteredTranscoders:
			w.allTranscoderStats()
		default:
			log.Error("That's not something I can do")
		}
	}
}
