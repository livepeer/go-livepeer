/*
Livepeer is a peer-to-peer global video live streaming network.  The Golp project is a go implementation of the Livepeer protocol.  For more information, visit the project wiki.
*/
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"os/user"

	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/rpc"
	"github.com/livepeer/go-livepeer/build"
	"github.com/livepeer/go-livepeer/pm"
	"github.com/livepeer/go-livepeer/server"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/discovery"
	"github.com/livepeer/go-livepeer/drivers"
	"github.com/livepeer/go-livepeer/eth"
	"github.com/livepeer/go-livepeer/eth/blockwatch"
	"github.com/livepeer/go-livepeer/eth/eventservices"
	"github.com/livepeer/go-livepeer/eth/watchers"
	"github.com/livepeer/go-livepeer/verification"

	lpmon "github.com/livepeer/go-livepeer/monitor"
)

var (
	ErrKeygen    = errors.New("ErrKeygen")
	EthTxTimeout = 600 * time.Second

	// The timeout for ETH RPC calls
	ethRPCTimeout = 20 * time.Second
	// The maximum blocks for the block watcher to retain
	blockWatcherRetentionLimit = 20

	// Estimate of the gas required to redeem a PM ticket
	redeemGas = 250000
	// The multiplier on the transaction cost to use for PM ticket faceValue
	txCostMultiplier = 100

	// The interval at which to clean up cached max float values for PM senders and balances per stream
	cleanupInterval = 1 * time.Minute
	// The time to live for cached max float values for PM senders (else they will be cleaned up) in seconds
	smTTL = 60 // 1 minute
)

const RtmpPort = "1935"
const RpcPort = "8935"
const CliPort = "7935"

// Add to this list as new features are added. Orchestrator only
var defaultCapabilities = []core.Capability{
	core.Capability_H264,
	core.Capability_MPEGTS,
	core.Capability_MP4,
	core.Capability_FractionalFramerates,
	core.Capability_StorageDirect,
	core.Capability_StorageS3,
	core.Capability_StorageGCS,
	core.Capability_ProfileH264Baseline,
	core.Capability_ProfileH264Main,
	core.Capability_ProfileH264High,
	core.Capability_ProfileH264ConstrainedHigh,
	core.Capability_GOP,
}

// Add to this list as certain features become mandatory. Orchestrator only
// Use sparingly, as adding to this is a hard break with older nodes
var mandatoryCapabilities = []core.Capability{}

func main() {
	// Override the default flag set since there are dependencies that
	// incorrectly add their own flags (specifically, due to the 'testing'
	// package being linked)
	flag.Set("logtostderr", "true")
	usr, err := user.Current()
	if err != nil {
		glog.Fatalf("Cannot find current user: %v", err)
	}
	vFlag := flag.Lookup("v")
	//We preserve this flag before resetting all the flags.  Not a scalable approach, but it'll do for now.  More discussions here - https://github.com/livepeer/go-livepeer/pull/617
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

	// Network & Addresses:
	network := flag.String("network", "offchain", "Network to connect to")
	rtmpAddr := flag.String("rtmpAddr", "127.0.0.1:"+RtmpPort, "Address to bind for RTMP commands")
	cliAddr := flag.String("cliAddr", "127.0.0.1:"+CliPort, "Address to bind for  CLI commands")
	httpAddr := flag.String("httpAddr", "", "Address to bind for HTTP commands")
	serviceAddr := flag.String("serviceAddr", "", "Orchestrator only. Overrides the on-chain serviceURI that broadcasters can use to contact this node; may be an IP or hostname.")
	orchAddr := flag.String("orchAddr", "", "Orchestrator to connect to as a standalone transcoder")
	verifierURL := flag.String("verifierUrl", "", "URL of the verifier to use")

	verifierPath := flag.String("verifierPath", "", "Path to verifier shared volume")
	httpIngest := flag.Bool("httpIngest", true, "Set to true to enable HTTP ingest")

	// Transcoding:
	orchestrator := flag.Bool("orchestrator", false, "Set to true to be an orchestrator")
	transcoder := flag.Bool("transcoder", false, "Set to true to be a transcoder")
	broadcaster := flag.Bool("broadcaster", false, "Set to true to be a broadcaster")
	orchSecret := flag.String("orchSecret", "", "Shared secret with the orchestrator as a standalone transcoder")
	transcodingOptions := flag.String("transcodingOptions", "P240p30fps16x9,P360p30fps16x9", "Transcoding options for broadcast job, or path to json config")
	maxAttempts := flag.Int("maxAttempts", 3, "Maximum transcode attempts")
	maxSessions := flag.Int("maxSessions", 10, "Maximum number of concurrent transcoding sessions for Orchestrator, maximum number or RTMP streams for Broadcaster, or maximum capacity for transcoder")
	currentManifest := flag.Bool("currentManifest", false, "Expose the currently active ManifestID as \"/stream/current.m3u8\"")
	nvidia := flag.String("nvidia", "", "Comma-separated list of Nvidia GPU device IDs to use for transcoding")
	testTranscoder := flag.Bool("testTranscoder", true, "Test Nvidia GPU transcoding at startup")

	// Onchain:
	ethAcctAddr := flag.String("ethAcctAddr", "", "Existing Eth account address")
	ethPassword := flag.String("ethPassword", "", "Password for existing Eth account address")
	ethKeystorePath := flag.String("ethKeystorePath", "", "Path for the Eth Key")
	ethOrchAddr := flag.String("ethOrchAddr", "", "ETH address of an on-chain registered orchestrator")
	ethUrl := flag.String("ethUrl", "", "Ethereum node JSON-RPC URL")
	ethController := flag.String("ethController", "", "Protocol smart contract address")
	gasLimit := flag.Int("gasLimit", 0, "Gas limit for ETH transactions")
	gasPrice := flag.Int("gasPrice", 0, "Gas price for ETH transactions")
	initializeRound := flag.Bool("initializeRound", false, "Set to true if running as a transcoder and the node should automatically initialize new rounds")
	ticketEV := flag.String("ticketEV", "1000000000000", "The expected value for PM tickets")
	// Broadcaster max acceptable ticket EV
	maxTicketEV := flag.String("maxTicketEV", "100000000000000", "The maximum acceptable expected value for PM tickets")
	// Broadcaster deposit multiplier to determine max acceptable ticket faceValue
	depositMultiplier := flag.Int("depositMultiplier", 1, "The deposit multiplier used to determine max acceptable faceValue for PM tickets")
	// Orchestrator base pricing info
	pricePerUnit := flag.Int("pricePerUnit", 0, "The price per 'pixelsPerUnit' amount pixels")
	// Broadcaster max acceptable price
	maxPricePerUnit := flag.Int("maxPricePerUnit", 0, "The maximum transcoding price (in wei) per 'pixelsPerUnit' a broadcaster is willing to accept. If not set explicitly, broadcaster is willing to accept ANY price")
	// Unit of pixels for both O's basePriceInfo and B's MaxBroadcastPrice
	pixelsPerUnit := flag.Int("pixelsPerUnit", 1, "Amount of pixels per unit. Set to '> 1' to have smaller price granularity than 1 wei / pixel")
	// Interval to poll for blocks
	blockPollingInterval := flag.Int("blockPollingInterval", 5, "Interval in seconds at which different blockchain event services poll for blocks")
	// Redemption service
	redeemer := flag.Bool("redeemer", false, "Set to true to run a ticket redemption service")
	redeemerAddr := flag.String("redeemerAddr", "", "URL of the ticket redemption service to use")
	// Reward service
	reward := flag.Bool("reward", false, "Set to true to run a reward service")
	// Metrics & logging:
	monitor := flag.Bool("monitor", false, "Set to true to send performance metrics")
	version := flag.Bool("version", false, "Print out the version")
	verbosity := flag.String("v", "", "Log verbosity.  {4|5|6}")

	// Storage:
	datadir := flag.String("datadir", "", "data directory")
	objectstore := flag.String("objectStore", "", "url of primary object store")

	// All deprecated
	s3bucket := flag.String("s3bucket", "", "S3 region/bucket (e.g. eu-central-1/testbucket)")
	s3creds := flag.String("s3creds", "", "S3 credentials (in form ACCESSKEYID/ACCESSKEY)")
	gsBucket := flag.String("gsbucket", "", "Google storage bucket")
	gsKey := flag.String("gskey", "", "Google Storage private key file name (in json format)")

	// API
	authWebhookURL := flag.String("authWebhookUrl", "", "RTMP authentication webhook URL")
	orchWebhookURL := flag.String("orchWebhookUrl", "", "Orchestrator discovery callback URL")

	flag.Parse()
	vFlag.Value.Set(*verbosity)

	isFlagSet := make(map[string]bool)
	flag.Visit(func(f *flag.Flag) { isFlagSet[f.Name] = true })

	blockPollingTime := time.Duration(*blockPollingInterval) * time.Second

	if *version {
		fmt.Println("Livepeer Node Version: " + core.LivepeerVersion)
		fmt.Printf("Golang runtime version: %s %s\n", runtime.Compiler, runtime.Version())
		fmt.Printf("Architecture: %s\n", runtime.GOARCH)
		fmt.Printf("Operating system: %s\n", runtime.GOOS)
		return
	}

	if *maxSessions <= 0 {
		glog.Fatal("-maxSessions must be greater than zero")
		return
	}

	type NetworkConfig struct {
		ethController string
	}

	ctx := context.Background()

	configOptions := map[string]*NetworkConfig{
		"rinkeby": {
			ethController: "0xA268AEa9D048F8d3A592dD7f1821297972D4C8Ea",
		},
		"mainnet": {
			ethController: "0xf96d54e490317c557a967abfa5d6e33006be69b3",
		},
	}

	// If multiple orchAddr specified, ensure other necessary flags present and clean up list
	var orchURLs []*url.URL
	if len(*orchAddr) > 0 {
		for _, addr := range strings.Split(*orchAddr, ",") {
			addr = strings.TrimSpace(addr)
			addr = defaultAddr(addr, "127.0.0.1", RpcPort)
			if !strings.HasPrefix(addr, "http") {
				addr = "https://" + addr
			}
			uri, err := url.ParseRequestURI(addr)
			if err != nil {
				glog.Error("Could not parse orchestrator URI: ", err)
				continue
			}
			orchURLs = append(orchURLs, uri)
		}
	}

	// Setting config options based on specified network
	if netw, ok := configOptions[*network]; ok {
		if *ethController == "" {
			*ethController = netw.ethController
		}
		glog.Infof("***Livepeer is running on the %v network: %v***", *network, *ethController)
	} else {
		glog.Infof("***Livepeer is running on the %v network***", *network)
	}

	if *datadir == "" {
		homedir := os.Getenv("HOME")
		if homedir == "" {
			homedir = usr.HomeDir
		}
		*datadir = filepath.Join(homedir, ".lpData", *network)
	}

	//Make sure datadir is present
	if _, err := os.Stat(*datadir); os.IsNotExist(err) {
		glog.Infof("Creating data dir: %v", *datadir)
		if err = os.MkdirAll(*datadir, 0755); err != nil {
			glog.Errorf("Error creating datadir: %v", err)
		}
	}

	//Set up DB
	dbh, err := common.InitDB(*datadir + "/lpdb.sqlite3")
	if err != nil {
		glog.Errorf("Error opening DB: %v", err)
		return
	}
	defer dbh.Close()

	n, err := core.NewLivepeerNode(nil, *datadir, dbh)
	if err != nil {
		glog.Errorf("Error creating livepeer node: %v", err)
	}

	if *orchSecret != "" {
		n.OrchSecret, _ = common.GetPass(*orchSecret)
	}

	if *transcoder {
		core.WorkDir = *datadir
		if *nvidia != "" {
			if *testTranscoder {
				err := core.TestNvidiaTranscoder(*nvidia)
				if err != nil {
					glog.Fatalf("Unable to transcode using Nvidia gpu=%s err=%v", *nvidia, err)
				}
			}
			n.Transcoder = core.NewLoadBalancingTranscoder(*nvidia, core.NewNvidiaTranscoder)
		} else {
			n.Transcoder = core.NewLocalTranscoder(*datadir)
		}
	}

	if *redeemer {
		n.NodeType = core.RedeemerNode
	} else if *orchestrator {
		n.NodeType = core.OrchestratorNode
		if !*transcoder {
			n.TranscoderManager = core.NewRemoteTranscoderManager()
			n.Transcoder = n.TranscoderManager
		}
	} else if *transcoder {
		n.NodeType = core.TranscoderNode
	} else if *broadcaster {
		n.NodeType = core.BroadcasterNode
	} else if !*reward && !*initializeRound {
		glog.Fatalf("No services enabled; must be at least one of -broadcaster, -transcoder, -orchestrator, -redeemer, -reward or -initializeRound")
	}

	if *monitor {
		lpmon.Enabled = true
		nodeID := *ethAcctAddr
		if nodeID == "" {
			hn, _ := os.Hostname()
			nodeID = hn
		}
		nodeType := "dflt"
		switch n.NodeType {
		case core.BroadcasterNode:
			nodeType = "bctr"
		case core.OrchestratorNode:
			nodeType = "orch"
		case core.TranscoderNode:
			nodeType = "trcr"
		case core.RedeemerNode:
			nodeType = "rdmr"
		}
		lpmon.InitCensus(nodeType, nodeID, core.LivepeerVersion)
	}

	if n.NodeType == core.TranscoderNode {
		glog.Info("***Livepeer is in transcoder mode ***")
		if n.OrchSecret == "" {
			glog.Fatal("Missing -orchSecret")
		}
		if len(orchURLs) > 0 {
			server.RunTranscoder(n, orchURLs[0].Host, *maxSessions)
		} else {
			glog.Fatal("Missing -orchAddr")
		}
		return
	}

	watcherErr := make(chan error)
	redeemerErr := make(chan error)
	var timeWatcher *watchers.TimeWatcher
	if *network == "offchain" {
		glog.Infof("***Livepeer is in off-chain mode***")

		if err := checkOrStoreChainID(dbh, big.NewInt(0)); err != nil {
			glog.Error(err)
			return
		}

	} else {
		var keystoreDir string
		if _, err := os.Stat(*ethKeystorePath); !os.IsNotExist(err) {
			keystoreDir, _ = filepath.Split(*ethKeystorePath)
		} else {
			keystoreDir = filepath.Join(*datadir, "keystore")
		}

		if keystoreDir == "" {
			glog.Errorf("Cannot find keystore directory")
			return
		}

		//Get the Eth client connection information
		if *ethUrl == "" {
			glog.Fatal("Need to specify an Ethereum node JSON-RPC URL using -ethUrl")
		}

		//Set up eth client
		backend, err := ethclient.Dial(*ethUrl)
		if err != nil {
			glog.Errorf("Failed to connect to Ethereum client: %v", err)
			return
		}

		chainID, err := backend.ChainID(ctx)
		if err != nil {
			glog.Errorf("failed to get chain ID from remote ethereum node: %v", err)
			return
		}

		if !build.ChainSupported(chainID.Int64()) {
			glog.Errorf("node does not support chainID = %v right now", chainID)
			return
		}

		if err := checkOrStoreChainID(dbh, chainID); err != nil {
			glog.Error(err)
			return
		}

		client, err := eth.NewClient(ethcommon.HexToAddress(*ethAcctAddr), keystoreDir, backend, ethcommon.HexToAddress(*ethController), EthTxTimeout)
		if err != nil {
			glog.Errorf("Failed to create client: %v", err)
			return
		}

		var bigGasPrice *big.Int
		if *gasPrice > 0 {
			bigGasPrice = big.NewInt(int64(*gasPrice))
		}

		err = client.Setup(*ethPassword, uint64(*gasLimit), bigGasPrice)
		if err != nil {
			glog.Errorf("Failed to setup client: %v", err)
			return
		}

		n.Eth = client

		addrMap := n.Eth.ContractAddresses()

		// Initialize block watcher that will emit logs used by event watchers
		blockWatcherClient, err := blockwatch.NewRPCClient(*ethUrl, ethRPCTimeout)
		if err != nil {
			glog.Errorf("Failed to setup blockwatch client: %v", err)
			return
		}
		topics := watchers.FilterTopics()

		// Determine backfilling start block
		originalLastSeenBlock, err := dbh.LastSeenBlock()
		if err != nil {
			glog.Errorf("db: failed to retrieve latest retained block: %v", err)
			return
		}
		currentRoundStartBlock, err := client.CurrentRoundStartBlock()
		if err != nil {
			glog.Errorf("eth: failed to retrieve current round start block: %v", err)
			return
		}

		var blockWatcherBackfillStartBlock *big.Int
		if originalLastSeenBlock == nil || originalLastSeenBlock.Cmp(currentRoundStartBlock) < 0 {
			blockWatcherBackfillStartBlock = currentRoundStartBlock
		}

		blockWatcherCfg := blockwatch.Config{
			Store:               n.Database,
			PollingInterval:     blockPollingTime,
			StartBlockDepth:     rpc.LatestBlockNumber,
			BackfillStartBlock:  blockWatcherBackfillStartBlock,
			BlockRetentionLimit: blockWatcherRetentionLimit,
			WithLogs:            true,
			Topics:              topics,
			Client:              blockWatcherClient,
		}
		// Wait until all event watchers have been initialized before starting the block watcher
		blockWatcher := blockwatch.New(blockWatcherCfg)

		timeWatcher, err = watchers.NewTimeWatcher(addrMap["RoundsManager"], blockWatcher, n.Eth)
		if err != nil {
			glog.Errorf("Failed to setup roundswatcher: %v", err)
			return
		}

		timeWatcherErr := make(chan error, 1)
		go func() {
			if err := timeWatcher.Watch(); err != nil {
				timeWatcherErr <- fmt.Errorf("roundswatcher failed to start watching for events: %v", err)
			}
		}()
		defer timeWatcher.Stop()

		// Initialize unbonding watcher to update the DB with latest state of the node's unbonding locks
		unbondingWatcher, err := watchers.NewUnbondingWatcher(n.Eth.Account().Address, addrMap["BondingManager"], blockWatcher, n.Database)
		if err != nil {
			glog.Errorf("Failed to setup unbonding watcher: %v", err)
			return
		}
		// Start unbonding watcher (logs will not be received until the block watcher is started)
		go unbondingWatcher.Watch()
		defer unbondingWatcher.Stop()

		senderWatcher, err := watchers.NewSenderWatcher(addrMap["TicketBroker"], blockWatcher, n.Eth, timeWatcher)
		if err != nil {
			glog.Errorf("Failed to setup senderwatcher: %v", err)
			return
		}
		go senderWatcher.Watch()
		defer senderWatcher.Stop()

		orchWatcher, err := watchers.NewOrchestratorWatcher(addrMap["BondingManager"], blockWatcher, dbh, n.Eth, timeWatcher)
		if err != nil {
			glog.Errorf("Failed to setup orchestrator watcher: %v", err)
			return
		}
		go orchWatcher.Watch()
		defer orchWatcher.Stop()

		serviceRegistryWatcher, err := watchers.NewServiceRegistryWatcher(addrMap["ServiceRegistry"], blockWatcher, dbh, n.Eth)
		if err != nil {
			glog.Errorf("Failed to set up service registry watcher: %v", err)
			return
		}
		go serviceRegistryWatcher.Watch()
		defer serviceRegistryWatcher.Stop()

		n.Balances = core.NewAddressBalances(cleanupInterval)
		defer n.Balances.StopCleanup()

		// By default the ticket recipient is the node's address
		// If the address of an on-chain registered orchestrator is provided, then it should be specified as the ticket recipient
		recipientAddr := n.Eth.Account().Address
		if *ethOrchAddr != "" {
			recipientAddr = ethcommon.HexToAddress(*ethOrchAddr)
		}

		if *orchestrator {
			// Set price per pixel base info
			if *pixelsPerUnit <= 0 {
				// Can't divide by 0
				panic(fmt.Errorf("The amount of pixels per unit must be greater than 0, provided %d instead\n", *pixelsPerUnit))
			}
			if *pricePerUnit <= 0 {
				// Prevent orchestrator from unknowingly provide free transcoding
				panic(fmt.Errorf("Price per unit of pixels must be greater than 0, provided %d instead\n", *pricePerUnit))
			}
			n.SetBasePrice(big.NewRat(int64(*pricePerUnit), int64(*pixelsPerUnit)))
			glog.Infof("Price: %d wei for %d pixels\n ", *pricePerUnit, *pixelsPerUnit)

			ev, _ := new(big.Int).SetString(*ticketEV, 10)
			if ev == nil {
				glog.Errorf("-ticketEV must be a valid integer, but %v provided. Restart the node with a different valid value for -ticketEV", *ticketEV)
				return
			}

			if ev.Cmp(big.NewInt(0)) < 0 {
				glog.Errorf("-ticketEV must be greater than 0, but %v provided. Restart the node with a different valid value for -ticketEV", *ticketEV)
				return
			}

			orchSetupCtx, cancel := context.WithCancel(ctx)
			defer cancel()

			if err := setupOrchestrator(orchSetupCtx, n, recipientAddr); err != nil {
				glog.Errorf("Error setting up orchestrator: %v", err)
				return
			}

			sigVerifier := &pm.DefaultSigVerifier{}
			validator := pm.NewValidator(sigVerifier, timeWatcher)
			gpm := eth.NewGasPriceMonitor(backend, blockPollingTime)
			// Start gas price monitor
			_, err := gpm.Start(ctx)
			if err != nil {
				glog.Errorf("error starting gas price monitor: %v", err)
				return
			}
			defer gpm.Stop()

			var sm pm.SenderMonitor
			if *redeemerAddr != "" {
				*redeemerAddr = defaultAddr(*redeemerAddr, "127.0.0.1", RpcPort)
				rc, err := server.NewRedeemerClient(*redeemerAddr, senderWatcher, timeWatcher)
				if err != nil {
					glog.Error("Unable to start redeemer client: ", err)
					return
				}
				sm = rc
			} else {
				sm = pm.NewSenderMonitor(recipientAddr, n.Eth, senderWatcher, timeWatcher, n.Database, cleanupInterval, smTTL)
			}

			// Start sender monitor
			sm.Start()
			defer sm.Stop()

			cfg := pm.TicketParamsConfig{
				EV:               ev,
				RedeemGas:        redeemGas,
				TxCostMultiplier: txCostMultiplier,
			}
			n.Recipient, err = pm.NewRecipient(
				recipientAddr,
				n.Eth,
				validator,
				gpm,
				sm,
				timeWatcher,
				cfg,
			)
			if err != nil {
				glog.Errorf("Error setting up PM recipient: %v", err)
				return
			}
		}

		if n.NodeType == core.BroadcasterNode {
			ev, _ := new(big.Rat).SetString(*maxTicketEV)
			if ev == nil {
				panic(fmt.Errorf("-maxTicketEV must be a valid rational number, but %v provided. Restart the node with a valid value for -maxTicketEV", *maxTicketEV))
			}

			if ev.Cmp(big.NewRat(0, 1)) < 0 {
				panic(fmt.Errorf("-maxTicketEV must not be negative, but %v provided. Restart the node with a valid value for -maxTicketEV", *maxTicketEV))
			}

			if *depositMultiplier <= 0 {
				panic(fmt.Errorf("-depositMultiplier must be greater than 0, but %v provided. Restart the node with a valid value for -depositMultiplier", *depositMultiplier))
			}

			// Fetch and cache broadcaster on-chain info
			info, err := senderWatcher.GetSenderInfo(n.Eth.Account().Address)
			if err != nil {
				glog.Error("Failed to get broadcaster on-chain info: ", err)
				return
			}
			glog.Info("Broadcaster Deposit: ", eth.FormatUnits(info.Deposit, "ETH"))
			glog.Info("Broadcaster Reserve: ", eth.FormatUnits(info.Reserve.FundsRemaining, "ETH"))

			n.Sender = pm.NewSender(n.Eth, timeWatcher, senderWatcher, ev, *depositMultiplier)

			if *pixelsPerUnit <= 0 {
				// Can't divide by 0
				panic(fmt.Errorf("The amount of pixels per unit must be greater than 0, provided %d instead\n", *pixelsPerUnit))
			}
			if *maxPricePerUnit > 0 {
				server.BroadcastCfg.SetMaxPrice(big.NewRat(int64(*maxPricePerUnit), int64(*pixelsPerUnit)))
			} else {
				glog.Infof("Maximum transcoding price per pixel is not greater than 0: %v, broadcaster is currently set to accept ANY price.\n", *maxPricePerUnit)
				glog.Infoln("To update the broadcaster's maximum acceptable transcoding price per pixel, use the CLI or restart the broadcaster with the appropriate 'maxPricePerUnit' and 'pixelsPerUnit' values")
			}
		}

		if n.NodeType == core.RedeemerNode {
			r, err := server.NewRedeemer(
				recipientAddr,
				n.Eth,
				pm.NewSenderMonitor(recipientAddr, n.Eth, senderWatcher, timeWatcher, n.Database, cleanupInterval, smTTL),
			)
			if err != nil {
				glog.Errorf("Unable to create redeemer: %v", err)
				return
			}

			*httpAddr = defaultAddr(*httpAddr, "127.0.0.1", RpcPort)
			url, err := url.ParseRequestURI("https://" + *httpAddr)
			if err != nil {
				glog.Error("Could not parse redeemer URI: ", err)
				return
			}

			go func() {
				if err := r.Start(url, n.WorkDir); err != nil {
					redeemerErr <- err
					return
				}
			}()
			defer r.Stop()
			glog.Infof("Redeemer started on %v", *httpAddr)
		}

		if !isFlagSet["reward"] {
			// If the node address is an on-chain registered address, start the reward service
			t, err := n.Eth.GetTranscoder(n.Eth.Account().Address)
			if err != nil {
				glog.Error(err)
				return
			}
			if t.Status == "Registered" {
				*reward = true
			}
		}

		if *reward {
			// Start reward service
			// The node will only call reward if it is active in the current round
			rs := eventservices.NewRewardService(n.Eth, blockPollingTime)
			rs.Start(ctx)
			defer rs.Stop()
		}

		if *initializeRound {
			// Start round initializer
			// The node will only initialize rounds if it in the upcoming active set for the round
			initializer := eth.NewRoundInitializer(n.Eth, n.Database, timeWatcher, blockPollingTime)
			go initializer.Start()
			defer initializer.Stop()
		}

		blockWatchCtx, cancel := context.WithCancel(ctx)
		defer cancel()

		// Backfill events that the node has missed since its last seen block. This method will block
		// and the node will not continue setup until it finishes
		if err := blockWatcher.BackfillEventsIfNeeded(blockWatchCtx); err != nil {
			glog.Errorf("Failed to backfill events: %v", err)
			return
		}

		blockWatcherErr := make(chan error, 1)
		go func() {
			if err := blockWatcher.Watch(blockWatchCtx); err != nil {
				blockWatcherErr <- fmt.Errorf("block watcher error: %v", err)
			}
		}()

		go func() {
			var err error
			select {
			case err = <-timeWatcherErr:
			case err = <-blockWatcherErr:
			}

			watcherErr <- err
		}()
	}

	if *s3bucket != "" && *s3creds == "" || *s3bucket == "" && *s3creds != "" {
		glog.Error("Should specify both s3bucket and s3creds")
		return
	}
	if *gsBucket != "" && *gsKey == "" || *gsBucket == "" && *gsKey != "" {
		glog.Error("Should specify both gsbucket and gskey")
		return
	}

	// XXX get s3 credentials from local env vars?
	if *s3bucket != "" && *s3creds != "" {
		br := strings.Split(*s3bucket, "/")
		cr := strings.Split(*s3creds, "/")
		u := url.URL{
			Scheme: "s3",
			Host:   br[0],
			Path:   fmt.Sprintf("/%s", br[1]),
			User:   url.UserPassword(cr[0], cr[1]),
		}
		glog.Warningf("-s3bucket and -s3creds are deprecated. Instead, you can use -objectStore %s", u.String())
		ustr := u.String()
		objectstore = &ustr
	}

	if *gsBucket != "" && *gsKey != "" {
		u := url.URL{
			Scheme:   "gs",
			Host:     *gsBucket,
			RawQuery: fmt.Sprintf("keyfile=%s", *gsKey),
		}
		glog.Warningf("-gsbucket and -gskey are deprecated. Instead, you can use -objectStore '%s'", u.String())
		ustr := u.String()
		objectstore = &ustr
	}

	if *objectstore != "" {
		prepared, err := drivers.PrepareOSURL(*objectstore)
		if err != nil {
			glog.Error("Error creating object store driver: ", err)
			return
		}
		drivers.NodeStorage, err = drivers.ParseOSURL(prepared, true)
		if err != nil {
			glog.Error("Error creating object store driver: ", err)
			return
		}
	}

	core.MaxSessions = *maxSessions
	if lpmon.Enabled {
		lpmon.MaxSessions(core.MaxSessions)
	}

	if n.NodeType == core.BroadcasterNode {
		// default lpms listener for broadcaster; same as default rpc port
		// TODO provide an option to disable this?
		*rtmpAddr = defaultAddr(*rtmpAddr, "127.0.0.1", RtmpPort)
		*httpAddr = defaultAddr(*httpAddr, "127.0.0.1", RpcPort)

		bcast := core.NewBroadcaster(n)

		// When the node is on-chain mode always cache the on-chain orchestrators and poll for updates
		// Right now we rely on the DBOrchestratorPoolCache constructor to do this. Consider separating the logic
		// caching/polling from the logic for fetching orchestrators during discovery
		if *network != "offchain" {
			ctx, cancel := context.WithCancel(ctx)
			defer cancel()
			dbOrchPoolCache, err := discovery.NewDBOrchestratorPoolCache(ctx, n, timeWatcher)
			if err != nil {
				glog.Errorf("Could not create orchestrator pool with DB cache: %v", err)
			}

			n.OrchestratorPool = dbOrchPoolCache
		}

		// Set up orchestrator discovery
		if *orchWebhookURL != "" {
			whurl, err := validateURL(*orchWebhookURL)
			if err != nil {
				glog.Fatal("Error setting orch webhook URL ", err)
			}
			glog.Info("Using orchestrator webhook URL ", whurl)
			n.OrchestratorPool = discovery.NewWebhookPool(bcast, whurl)
		} else if len(orchURLs) > 0 {
			n.OrchestratorPool = discovery.NewOrchestratorPool(bcast, orchURLs)
		}

		if n.OrchestratorPool == nil {
			// Not a fatal error; may continue operating in segment-only mode
			glog.Error("No orchestrator specified; transcoding will not happen")
		}
		if *authWebhookURL != "" {
			_, err := validateURL(*authWebhookURL)
			if err != nil {
				glog.Fatal("Error setting auth webhook URL ", err)
			}
			glog.Info("Using auth webhook URL ", *authWebhookURL)
			server.AuthWebhookURL = *authWebhookURL
		}

		isLocalHTTP, err := isLocalURL("https://" + *httpAddr)
		if err != nil {
			glog.Errorf("Error checking for local -httpAddr: %v", err)
			return
		}
		if !isFlagSet["httpIngest"] && !isLocalHTTP && server.AuthWebhookURL == "" {
			glog.Warning("HTTP ingest is disabled because -httpAddr is publicly accessible. To enable, configure -authWebhookUrl or use the -httpIngest flag")
			*httpIngest = false
		}

		// Set up verifier
		if *verifierURL != "" {
			_, err := validateURL(*verifierURL)
			if err != nil {
				glog.Fatal("Error setting verifier URL ", err)
			}
			glog.Info("Using the Epic Labs classifier for verification at ", *verifierURL)
			server.Policy = &verification.Policy{Retries: 2, Verifier: &verification.EpicClassifier{Addr: *verifierURL}}
			// TODO Set up a default "empty" verifier-less policy for onchain
			//      that only checks sigs and pixels?

			// Set the verifier path. Remove once [1] is implemented!
			// [1] https://github.com/livepeer/verification-classifier/issues/64
			if drivers.NodeStorage == nil && *verifierPath == "" {
				glog.Fatal("Requires a path to the verifier shared volume when local storage is in use; use -verifierPath, S3 or GCS")
			}
			verification.VerifierPath = *verifierPath
		} else if *network != "offchain" {
			server.Policy = &verification.Policy{Retries: 2}
		}

		// Set max transcode attempts. <=0 is OK; it just means "don't transcode"
		server.MaxAttempts = *maxAttempts

	} else if n.NodeType == core.OrchestratorNode {
		suri, err := getServiceURI(n, *serviceAddr)
		if err != nil {
			glog.Fatal("Error getting service URI: ", err)
		}
		n.SetServiceURI(suri)
		// if http addr is not provided, listen to all ifaces
		// take the port to listen to from the service URI
		*httpAddr = defaultAddr(*httpAddr, "", n.GetServiceURI().Port())

		n.Capabilities = core.NewCapabilities(defaultCapabilities, mandatoryCapabilities)

		if !*transcoder && n.OrchSecret == "" {
			glog.Fatal("Running an orchestrator requires an -orchSecret for standalone mode or -transcoder for orchestrator+transcoder mode")
		}
	}
	*cliAddr = defaultAddr(*cliAddr, "127.0.0.1", CliPort)

	if drivers.NodeStorage == nil {
		// base URI will be empty for broadcasters; that's OK
		drivers.NodeStorage = drivers.NewMemoryDriver(n.GetServiceURI())
	}

	//Create Livepeer Node

	//Set up the media server
	s, err := server.NewLivepeerServer(*rtmpAddr, n, *httpIngest, *transcodingOptions)
	if err != nil {
		glog.Fatal("Error creating Livepeer server err=", err)
	}

	ec := make(chan error)
	tc := make(chan struct{})
	wc := make(chan struct{})
	msCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	if *currentManifest {
		glog.Info("Current ManifestID will be available over ", *httpAddr)
		s.ExposeCurrentManifest = *currentManifest
	}

	go func() {
		s.StartCliWebserver(*cliAddr)
		close(wc)
	}()
	if n.NodeType != core.RedeemerNode {
		go func() {
			ec <- s.StartMediaServer(msCtx, *httpAddr)
		}()
	}

	go func() {
		if core.OrchestratorNode != n.NodeType {
			return
		}

		orch := core.NewOrchestrator(s.LivepeerNode, timeWatcher)

		go func() {
			server.StartTranscodeServer(orch, *httpAddr, s.HTTPMux, n.WorkDir, n.TranscoderManager != nil)
			tc <- struct{}{}
		}()

		// check whether or not the orchestrator is available
		time.Sleep(2 * time.Second)
		orchAvail := server.CheckOrchestratorAvailability(orch)
		if !orchAvail {
			// shut down orchestrator
			glog.Infof("Orchestrator not available at %v; shutting down", orch.ServiceURI())
			tc <- struct{}{}
		}

	}()

	switch n.NodeType {
	case core.OrchestratorNode:
		glog.Infof("***Livepeer Running in Orchestrator Mode***")
	case core.BroadcasterNode:
		glog.Infof("***Livepeer Running in Broadcaster Mode***")
		glog.Infof("Video Ingest Endpoint - rtmp://%v", *rtmpAddr)
	case core.TranscoderNode:
		glog.Infof("**Liveepeer Running in Transcoder Mode***")
	case core.RedeemerNode:
		glog.Infof("**Livepeer Running in Redeemer Mode**")
	}

	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt)
	select {
	case err := <-watcherErr:
		glog.Error(err)
		return
	case err := <-ec:
		glog.Infof("Error from media server: %v", err)
		return
	case err := <-redeemerErr:
		if err != nil {
			glog.Fatalf("Error starting redemption service: %v", err)
		}
	case <-msCtx.Done():
		glog.Infof("MediaServer Done()")
		return
	case <-tc:
		glog.Infof("Orchestrator server shut down")
	case <-wc:
		glog.Infof("CLI webserver shut down")
		return
	case sig := <-c:
		glog.Infof("Exiting Livepeer: %v", sig)
		time.Sleep(time.Millisecond * 500) //Give time for other processes to shut down completely
		return
	}
}

func validateURL(u string) (*url.URL, error) {
	if u == "" {
		return nil, nil
	}
	p, err := url.ParseRequestURI(u)
	if err != nil {
		return nil, err
	}
	if p.Scheme != "http" && p.Scheme != "https" {
		return nil, errors.New("URL should be HTTP or HTTPS")
	}
	return p, nil
}

func isLocalURL(u string) (bool, error) {
	uri, err := url.ParseRequestURI(u)
	if err != nil {
		return false, err
	}

	hostname := uri.Hostname()
	if net.ParseIP(hostname).IsLoopback() || hostname == "localhost" {
		return true, nil
	}

	return false, nil
}

// ServiceURI checking steps:
// If passed in via -serviceAddr: return that
// Else: get inferred address.
// If offchain: return inferred address
// Else: get on-chain sURI
// If on-chain sURI mismatches inferred address: print warning
// Return on-chain sURI
func getServiceURI(n *core.LivepeerNode, serviceAddr string) (*url.URL, error) {
	// Passed in via CLI
	if serviceAddr != "" {
		return url.ParseRequestURI("https://" + serviceAddr)
	}

	// Infer address
	// TODO probably should put this (along w wizard GETs) into common code
	resp, err := http.Get("https://api.ipify.org?format=text")
	if err != nil {
		glog.Error("Could not look up public IP address")
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		glog.Error("Could not look up public IP address")
		return nil, err
	}
	addr := "https://" + strings.TrimSpace(string(body)) + ":" + RpcPort
	inferredUri, err := url.ParseRequestURI(addr)
	if n.Eth == nil {
		// we won't be looking up onchain sURI so use the inferred one
		return inferredUri, err
	}

	// On-chain lookup and matching with inferred public address
	addr, err = n.Eth.GetServiceURI(n.Eth.Account().Address)
	if err != nil {
		glog.Error("Could not get service URI; orchestrator may be unreachable")
		return nil, err
	}
	ethUri, err := url.ParseRequestURI(addr)
	if err != nil {
		glog.Error("Could not parse service URI; orchestrator may be unreachable")
		ethUri, _ = url.ParseRequestURI("http://127.0.0.1:" + RpcPort)
	}
	if ethUri.Hostname() != inferredUri.Hostname() || ethUri.Port() != inferredUri.Port() {
		glog.Errorf("Service address %v did not match discovered address %v; set the correct address in livepeer_cli or use -serviceAddr", ethUri, inferredUri)
	}
	return ethUri, nil
}

func setupOrchestrator(ctx context.Context, n *core.LivepeerNode, ethOrchAddr ethcommon.Address) error {
	// add orchestrator to DB
	orch, err := n.Eth.GetTranscoder(ethOrchAddr)
	if err != nil {
		return err
	}

	err = n.Database.UpdateOrch(&common.DBOrch{
		EthereumAddr:      ethOrchAddr.Hex(),
		ActivationRound:   common.ToInt64(orch.ActivationRound),
		DeactivationRound: common.ToInt64(orch.DeactivationRound),
	})
	if err != nil {
		return err
	}

	if !orch.Active {
		glog.Infof("Orchestrator %v is inactive", ethOrchAddr.Hex())
	} else {
		glog.Infof("Orchestrator %v is active", ethOrchAddr.Hex())
	}

	return nil
}

func defaultAddr(addr, defaultHost, defaultPort string) string {
	if addr == "" {
		return defaultHost + ":" + defaultPort
	}
	if addr[0] == ':' {
		return defaultHost + addr
	}
	// not IPv6 safe
	if !strings.Contains(addr, ":") {
		return addr + ":" + defaultPort
	}
	return addr
}

func checkOrStoreChainID(dbh *common.DB, chainID *big.Int) error {
	expectedChainID, err := dbh.ChainID()
	if err != nil {
		return err
	}

	if expectedChainID == nil {
		// No chainID stored yet
		// Store the provided chainID and skip the check
		if err := dbh.SetChainID(chainID); err != nil {
			return err
		}

		return nil
	}

	if expectedChainID.Cmp(chainID) != 0 {
		return fmt.Errorf("expecting chainID of %v, but got %v. Did you change networks without changing network name or datadir?", expectedChainID, chainID)
	}

	return nil
}
