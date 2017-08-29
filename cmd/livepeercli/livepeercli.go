package main

import (
	"bufio"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/ethereum/go-ethereum/log"
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
		},
		cli.StringFlag{
			Name:  "rtmp",
			Usage: "local rtmp port",
		},
		cli.IntFlag{
			Name:  "loglevel",
			Value: 4,
			Usage: "log level to emit to the screen",
		},
	}
	app.Action = func(c *cli.Context) error {
		// Set up the logger to print everything and the random generator
		log.Root().SetHandler(log.LvlFilterHandler(log.Lvl(c.Int("loglevel")), log.StreamHandler(os.Stdout, log.TerminalFormat(true))))
		rand.Seed(time.Now().UnixNano())

		// Start the wizard and relinquish control
		w := &wizard{
			endpoint: fmt.Sprintf("http://localhost:%v/status", c.String("http")),
			rtmpPort: c.String("rtmp"),
			httpPort: c.String("http"),
			in:       bufio.NewReader(os.Stdin),
		}
		w.run()

		return nil
	}
	app.Run(os.Args)
}

type wizard struct {
	// conf     config        // Configurations from previous runs
	endpoint string // Local livepeer node
	rtmpPort string
	httpPort string
	in       *bufio.Reader // Wrapper around stdin to allow reading user input
}

func (w *wizard) run() {
	fmt.Println("+-----------------------------------------------------------+")
	fmt.Println("| Welcome to livepeer-cli, your Livepeer command line tool  |")
	fmt.Println("|                                                           |")
	fmt.Println("| This tool lets you interact with a local Livepeer node    |")
	fmt.Println("| and participate in the Livepeer protocol without the	    |")
	fmt.Println("| hassle that it would normally entail.                     |")
	fmt.Println("|                                                           |")
	fmt.Println("+-----------------------------------------------------------+")
	fmt.Println()

	// Make sure there is a local node running
	_, err := http.Get(w.endpoint)
	if err != nil {
		log.Error("Cannot find local node: ", err)
		return
	}

	w.stats(false)
	// Basics done, loop ad infinitum about what to do
	for {
		fmt.Println()
		fmt.Println("What would you like to do? (default = stats)")
		fmt.Println(" 1. Get node status")
		fmt.Println(" 2. Depoist token")
		fmt.Println(" 3. Broadcast video")
		fmt.Println(" 4. Stream video")
		fmt.Println(" 5. Set transcoder config")
		fmt.Println(" 6. Bond")
		fmt.Println(" 7. Unbond")
		fmt.Println(" 8. Become a transcoder")
		fmt.Println(" 9. Get test Livepeer Token")

		choice := w.read()
		switch {
		case choice == "" || choice == "1":
			w.stats(false)
		case choice == "2":
			//Deposit token
		case choice == "3":
			//Broadcast video
			w.broadcast()
		case choice == "4":
			//Stream video
		case choice == "5":
			//Print transcoder config
			//Set transcoder config
		case choice == "6":
			//Check balance
			//Bond
		case choice == "7":
			//Check Bond Amount
			//Unbond
		case choice == "8":
			//Activate Transcoder
		case choice == "9":
			//Get test Livepeer Token

		default:
			log.Error("That's not something I can do")
		}
	}
}

func (w *wizard) stats(tips bool) {
	// Observe how the b's and the d's, despite appearing in the
	// second cell of each line, belong to different columns.
	wtr := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', tabwriter.AlignRight|tabwriter.Debug)
	fmt.Fprintf(wtr, "Node ID: \t%s\n", w.getNodeID())
	fmt.Fprintf(wtr, "Node Addr: \t%s\n", w.getNodeAddr())
	fmt.Fprintf(wtr, "Protocol Contract Addr: \t%s\n", w.getProtAddr())
	fmt.Fprintf(wtr, "Token Contract Addr: \t%s\n", w.getTokenAddr())
	fmt.Fprintf(wtr, "Faucet Contract Addr: \t%s\n", w.getFaucetAddr())
	fmt.Fprintf(wtr, "Account Eth Addr: \t%s\n", w.getEthAddr())
	fmt.Fprintf(wtr, "Token balance: \t%s\n", w.getTokenBalance())
	fmt.Fprintf(wtr, "Deposit Amount: \t%s\n", w.getDeposit())
	fmt.Fprintf(wtr, "Broadcast Job Segment Price: \t%s\n", w.getJobPrice())
	fmt.Fprintf(wtr, "Is Active Transcoder: \t%s\n", w.getIsActiveTranscoder())
	fmt.Fprintf(wtr, "Transcoder Price: \t%s\n", w.getTranscoderPrice())
	wtr.Flush()
}

func (w *wizard) getNodeID() string {
	return "12209433a695c8bf34ef6a40863cfe7ed64266d876176aee13732293b63ba1637fd2"
}

func (w *wizard) getNodeAddr() string {
	return "[/ip4/127.0.0.1/tcp/15000 /ip6/::1/tcp/15000 /ip4/10.30.111.21/tcp/15000]"
}

func (w *wizard) getProtAddr() string {
	return ""
}

func (w *wizard) getTokenAddr() string {
	return ""
}

func (w *wizard) getFaucetAddr() string {
	return ""
}

func (w *wizard) getEthAddr() string {
	return ""
}

func (w *wizard) getTokenBalance() string {
	return ""
}

func (w *wizard) getDeposit() string {
	return ""
}

func (w *wizard) getJobPrice() string {
	return ""
}

func (w *wizard) getIsActiveTranscoder() string {
	return ""
}

func (w *wizard) getTranscoderPrice() string {
	return ""
}

// // read reads a single line from stdin, trimming if from spaces.
func (w *wizard) read() string {
	fmt.Printf("> ")
	text, err := w.in.ReadString('\n')
	if err != nil {
		log.Crit("Failed to read user input", "err", err)
	}
	return strings.TrimSpace(text)
}

// readString reads a single line from stdin, trimming if from spaces, enforcing
// non-emptyness.
func (w *wizard) readString() string {
	for {
		fmt.Printf("> ")
		text, err := w.in.ReadString('\n')
		if err != nil {
			log.Crit("Failed to read user input", "err", err)
		}
		if text = strings.TrimSpace(text); text != "" {
			return text
		}
	}
}
