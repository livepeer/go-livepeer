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

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/livepeer/go-livepeer/build"
	"github.com/livepeer/go-livepeer/pm"
	"github.com/livepeer/go-livepeer/server"
	"github.com/livepeer/livepeer-data/pkg/event"
	"github.com/livepeer/livepeer-data/pkg/mistconnector"
	"github.com/peterbourgon/ff/v3"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/discovery"
	"github.com/livepeer/go-livepeer/drivers"
	"github.com/livepeer/go-livepeer/eth"
	"github.com/livepeer/go-livepeer/eth/blockwatch"
	"github.com/livepeer/go-livepeer/eth/watchers"
	"github.com/livepeer/go-livepeer/verification"
	"github.com/livepeer/lpms/ffmpeg"

	lpmon "github.com/livepeer/go-livepeer/monitor"
)

var (
	ErrKeygen = errors.New("ErrKeygen")

	// The timeout for ETH RPC calls
	ethRPCTimeout = 20 * time.Second
	// The maximum blocks for the block watcher to retain
	blockWatcherRetentionLimit = 20

	// Estimate of the gas required to redeem a PM ticket on L1 Ethereum
	redeemGasL1 = 350000
	// Estimate of the gas required to redeem a PM ticket on L2 Arbitrum
	redeemGasL2 = 1200000
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

type LivepeerConfig struct {
	network                      *string
	rtmpAddr                     *string
	cliAddr                      *string
	httpAddr                     *string
	serviceAddr                  *string
	orchAddr                     *string
	verifierURL                  *string
	ethController                *string
	verifierPath                 *string
	localVerify                  *bool
	httpIngest                   *bool
	orchestrator                 *bool
	transcoder                   *bool
	broadcaster                  *bool
	orchSecret                   *string
	transcodingOptions           *string
	maxAttempts                  *int
	selectRandFreq               *float64
	maxSessions                  *int
	currentManifest              *bool
	nvidia                       *string
	testTranscoder               *bool
	sceneClassificationModelPath *string
	ethAcctAddr                  *string
	ethPassword                  *string
	ethKeystorePath              *string
	ethOrchAddr                  *string
	ethUrl                       *string
	txTimeout                    *time.Duration
	maxTxReplacements            *int
	gasLimit                     *int
	minGasPrice                  *int64
	maxGasPrice                  *int
	initializeRound              *bool
	ticketEV                     *string
	maxTicketEV                  *string
	depositMultiplier            *int
	pricePerUnit                 *int
	maxPricePerUnit              *int
	pixelsPerUnit                *int
	autoAdjustPrice              *bool
	blockPollingInterval         *int
	redeemer                     *bool
	redeemerAddr                 *string
	reward                       *bool
	monitor                      *bool
	metricsPerStream             *bool
	metricsExposeClientIP        *bool
	metadataQueueUri             *string
	metadataAmqpExchange         *string
	metadataPublishTimeout       *time.Duration
	datadir                      *string
	objectstore                  *string
	recordstore                  *string
	authWebhookURL               *string
	orchWebhookURL               *string
	detectionWebhookURL          *string
}

func main() {
	// Override the default flag set since there are dependencies that
	// incorrectly add their own flags (specifically, due to the 'testing'
	// package being linked)
	flag.Set("logtostderr", "true")
	vFlag := flag.Lookup("v")
	//We preserve this flag before resetting all the flags.  Not a scalable approach, but it'll do for now.  More discussions here - https://github.com/livepeer/go-livepeer/pull/617
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

	// Help & Log
	mistJson := flag.Bool("j", false, "Print application info as json")
	version := flag.Bool("version", false, "Print out the version")
	verbosity := flag.String("v", "", "Log verbosity.  {4|5|6}")

	cfg := parseLivepeerConfig()

	// Config file
	_ = flag.String("config", "", "Config file in the format 'key value', flags and env vars take precedence over the config file")
	err := ff.Parse(flag.CommandLine, os.Args[1:],
		ff.WithConfigFileFlag("config"),
		ff.WithEnvVarPrefix("LP"),
		ff.WithConfigFileParser(ff.PlainParser),
	)
	if err != nil {
		glog.Fatal("Error parsing config: ", err)
	}

	vFlag.Value.Set(*verbosity)

	cfg = updateNilsForUnsetFlags(cfg)

	if *mistJson {
		mistconnector.PrintMistConfigJson(
			"livepeer",
			"Official implementation of the Livepeer video processing protocol. Can play all roles in the network.",
			"Livepeer",
			core.LivepeerVersion,
			flag.CommandLine,
		)
		return
	}

	if *version {
		fmt.Println("Livepeer Node Version: " + core.LivepeerVersion)
		fmt.Printf("Golang runtime version: %s %s\n", runtime.Compiler, runtime.Version())
		fmt.Printf("Architecture: %s\n", runtime.GOARCH)
		fmt.Printf("Operating system: %s\n", runtime.GOOS)
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	lc := make(chan struct{})

	go func() {
		StartLivepeer(ctx, cfg)
		lc <- struct{}{}
	}()

	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt)
	select {
	case sig := <-c:
		glog.Infof("Exiting Livepeer: %v", sig)
		cancel()
		time.Sleep(time.Millisecond * 500) //Give time for other processes to shut down completely
	case <-lc:
	}
}

// DefaultLivepeerConfig creates LivepeerConfig exactly the same as when no flags are passed to the livepeer process.
func DefaultLivepeerConfig() LivepeerConfig {
	// Network & Addresses:
	defaultNetwork := "offchain"
	defaultRtmpAddr := "127.0.0.1:" + RtmpPort
	defaultCliAddr := "127.0.0.1:" + CliPort
	defaultHttpAddr := ""
	defaultServiceAddr := ""
	defaultOrchAddr := ""
	defaultVerifierURL := ""
	defaultVerifierPath := ""

	// Transcoding:
	defaultOrchestrator := false
	defaultTranscoder := false
	defaultBroadcaster := false
	defaultOrchSecret := ""
	defaultTranscodingOptions := "P240p30fps16x9,P360p30fps16x9"
	defaultMaxAttempts := 3
	defaultSelectRandFreq := 0.3
	defaultMaxSessions := 10
	defaultCurrentManifest := false
	defaultNvidia := ""
	defaultTestTranscoder := true
	defaultSceneClassificationModelPath := ""

	// Onchain:
	defaultEthAcctAddr := ""
	defaultEthPassword := ""
	defaultEthKeystorePath := ""
	defaultEthOrchAddr := ""
	defaultEthUrl := ""
	defaultTxTimeout := 5 * time.Minute
	defaultMaxTxReplacements := 1
	defaultGasLimit := 0
	defaultMaxGasPrice := 0
	defaultEthController := ""
	defaultInitializeRound := false
	defaultTicketEV := "1000000000000"
	defaultMaxTicketEV := "3000000000000"
	defaultDepositMultiplier := 1
	defaultMaxPricePerUnit := 0
	defaultPixelsPerUnit := 1
	defaultAutoAdjustPrice := true
	defaultBlockPollingInterval := 5
	defaultRedeemer := false
	defaultRedeemerAddr := ""
	defaultMonitor := false
	defaultMetricsPerStream := false
	defaultMetricsExposeClientIP := false
	defaultMetadataQueueUri := ""
	defaultMetadataAmqpExchange := "lp_golivepeer_metadata"
	defaultMetadataPublishTimeout := 1 * time.Second

	// Storage:
	defaultDatadir := ""
	defaultObjectstore := ""
	defaultRecordstore := ""

	// API
	defaultAuthWebhookURL := ""
	defaultOrchWebhookURL := ""
	defaultDetectionWebhookURL := ""

	return LivepeerConfig{
		// Network & Addresses:
		network:      &defaultNetwork,
		rtmpAddr:     &defaultRtmpAddr,
		cliAddr:      &defaultCliAddr,
		httpAddr:     &defaultHttpAddr,
		serviceAddr:  &defaultServiceAddr,
		orchAddr:     &defaultOrchAddr,
		verifierURL:  &defaultVerifierURL,
		verifierPath: &defaultVerifierPath,

		// Transcoding:
		orchestrator:                 &defaultOrchestrator,
		transcoder:                   &defaultTranscoder,
		broadcaster:                  &defaultBroadcaster,
		orchSecret:                   &defaultOrchSecret,
		transcodingOptions:           &defaultTranscodingOptions,
		maxAttempts:                  &defaultMaxAttempts,
		selectRandFreq:               &defaultSelectRandFreq,
		maxSessions:                  &defaultMaxSessions,
		currentManifest:              &defaultCurrentManifest,
		nvidia:                       &defaultNvidia,
		testTranscoder:               &defaultTestTranscoder,
		sceneClassificationModelPath: &defaultSceneClassificationModelPath,

		// Onchain:
		ethAcctAddr:            &defaultEthAcctAddr,
		ethPassword:            &defaultEthPassword,
		ethKeystorePath:        &defaultEthKeystorePath,
		ethOrchAddr:            &defaultEthOrchAddr,
		ethUrl:                 &defaultEthUrl,
		txTimeout:              &defaultTxTimeout,
		maxTxReplacements:      &defaultMaxTxReplacements,
		gasLimit:               &defaultGasLimit,
		maxGasPrice:            &defaultMaxGasPrice,
		ethController:          &defaultEthController,
		initializeRound:        &defaultInitializeRound,
		ticketEV:               &defaultTicketEV,
		maxTicketEV:            &defaultMaxTicketEV,
		depositMultiplier:      &defaultDepositMultiplier,
		maxPricePerUnit:        &defaultMaxPricePerUnit,
		pixelsPerUnit:          &defaultPixelsPerUnit,
		autoAdjustPrice:        &defaultAutoAdjustPrice,
		blockPollingInterval:   &defaultBlockPollingInterval,
		redeemer:               &defaultRedeemer,
		redeemerAddr:           &defaultRedeemerAddr,
		monitor:                &defaultMonitor,
		metricsPerStream:       &defaultMetricsPerStream,
		metricsExposeClientIP:  &defaultMetricsExposeClientIP,
		metadataQueueUri:       &defaultMetadataQueueUri,
		metadataAmqpExchange:   &defaultMetadataAmqpExchange,
		metadataPublishTimeout: &defaultMetadataPublishTimeout,

		// Storage:
		datadir:     &defaultDatadir,
		objectstore: &defaultObjectstore,
		recordstore: &defaultRecordstore,

		// API
		authWebhookURL:      &defaultAuthWebhookURL,
		orchWebhookURL:      &defaultOrchWebhookURL,
		detectionWebhookURL: &defaultDetectionWebhookURL,
	}
}

func parseLivepeerConfig() LivepeerConfig {
	cfg := DefaultLivepeerConfig()

	// Network & Addresses:
	cfg.network = flag.String("network", *cfg.network, "Network to connect to")
	cfg.rtmpAddr = flag.String("rtmpAddr", *cfg.rtmpAddr, "Address to bind for RTMP commands")
	cfg.cliAddr = flag.String("cliAddr", *cfg.cliAddr, "Address to bind for  CLI commands")
	cfg.httpAddr = flag.String("httpAddr", *cfg.httpAddr, "Address to bind for HTTP commands")
	cfg.serviceAddr = flag.String("serviceAddr", *cfg.serviceAddr, "Orchestrator only. Overrides the on-chain serviceURI that broadcasters can use to contact this node; may be an IP or hostname.")
	cfg.orchAddr = flag.String("orchAddr", *cfg.orchAddr, "Comma-separated list of orchestrators to connect to")
	cfg.verifierURL = flag.String("verifierUrl", *cfg.verifierURL, "URL of the verifier to use")
	cfg.verifierPath = flag.String("verifierPath", *cfg.verifierPath, "Path to verifier shared volume")
	cfg.localVerify = flag.Bool("localVerify", true, "Set to true to enable local verification i.e. pixel count and signature verification.")
	cfg.httpIngest = flag.Bool("httpIngest", true, "Set to true to enable HTTP ingest")

	// Transcoding:
	cfg.orchestrator = flag.Bool("orchestrator", *cfg.orchestrator, "Set to true to be an orchestrator")
	cfg.transcoder = flag.Bool("transcoder", *cfg.transcoder, "Set to true to be a transcoder")
	cfg.broadcaster = flag.Bool("broadcaster", *cfg.broadcaster, "Set to true to be a broadcaster")
	cfg.orchSecret = flag.String("orchSecret", *cfg.orchSecret, "Shared secret with the orchestrator as a standalone transcoder")
	cfg.transcodingOptions = flag.String("transcodingOptions", *cfg.transcodingOptions, "Transcoding options for broadcast job, or path to json config")
	cfg.maxAttempts = flag.Int("maxAttempts", *cfg.maxAttempts, "Maximum transcode attempts")
	cfg.selectRandFreq = flag.Float64("selectRandFreq", *cfg.selectRandFreq, "Frequency to randomly select unknown orchestrators (on-chain mode only)")
	cfg.maxSessions = flag.Int("maxSessions", *cfg.maxSessions, "Maximum number of concurrent transcoding sessions for Orchestrator, maximum number or RTMP streams for Broadcaster, or maximum capacity for transcoder")
	cfg.currentManifest = flag.Bool("currentManifest", *cfg.currentManifest, "Expose the currently active ManifestID as \"/stream/current.m3u8\"")
	cfg.nvidia = flag.String("nvidia", *cfg.nvidia, "Comma-separated list of Nvidia GPU device IDs (or \"all\" for all available devices)")
	cfg.testTranscoder = flag.Bool("testTranscoder", *cfg.testTranscoder, "Test Nvidia GPU transcoding at startup")
	cfg.sceneClassificationModelPath = flag.String("sceneClassificationModelPath", *cfg.sceneClassificationModelPath, "Path to scene classification model")

	// Onchain:
	cfg.ethAcctAddr = flag.String("ethAcctAddr", *cfg.ethAcctAddr, "Existing Eth account address")
	cfg.ethPassword = flag.String("ethPassword", *cfg.ethPassword, "Password for existing Eth account address")
	cfg.ethKeystorePath = flag.String("ethKeystorePath", *cfg.ethKeystorePath, "Path for the Eth Key")
	cfg.ethOrchAddr = flag.String("ethOrchAddr", *cfg.ethOrchAddr, "ETH address of an on-chain registered orchestrator")
	cfg.ethUrl = flag.String("ethUrl", *cfg.ethUrl, "Ethereum node JSON-RPC URL")
	cfg.txTimeout = flag.Duration("transactionTimeout", *cfg.txTimeout, "Amount of time to wait for an Ethereum transaction to confirm before timing out")
	cfg.maxTxReplacements = flag.Int("maxTransactionReplacements", *cfg.maxTxReplacements, "Number of times to automatically replace pending Ethereum transactions")
	cfg.gasLimit = flag.Int("gasLimit", *cfg.gasLimit, "Gas limit for ETH transactions")
	cfg.minGasPrice = flag.Int64("minGasPrice", 0, "Minimum gas price (priority fee + base fee) for ETH transactions in wei, 10 Gwei = 10000000000")
	cfg.maxGasPrice = flag.Int("maxGasPrice", *cfg.maxGasPrice, "Maximum gas price (priority fee + base fee) for ETH transactions in wei, 40 Gwei = 40000000000")
	cfg.ethController = flag.String("ethController", *cfg.ethController, "Protocol smart contract address")
	cfg.initializeRound = flag.Bool("initializeRound", *cfg.initializeRound, "Set to true if running as a transcoder and the node should automatically initialize new rounds")
	cfg.ticketEV = flag.String("ticketEV", *cfg.ticketEV, "The expected value for PM tickets")
	// Broadcaster max acceptable ticket EV
	cfg.maxTicketEV = flag.String("maxTicketEV", *cfg.maxTicketEV, "The maximum acceptable expected value for PM tickets")
	// Broadcaster deposit multiplier to determine max acceptable ticket faceValue
	cfg.depositMultiplier = flag.Int("depositMultiplier", *cfg.depositMultiplier, "The deposit multiplier used to determine max acceptable faceValue for PM tickets")
	// Orchestrator base pricing info
	cfg.pricePerUnit = flag.Int("pricePerUnit", 0, "The price per 'pixelsPerUnit' amount pixels")
	// Broadcaster max acceptable price
	cfg.maxPricePerUnit = flag.Int("maxPricePerUnit", *cfg.maxPricePerUnit, "The maximum transcoding price (in wei) per 'pixelsPerUnit' a broadcaster is willing to accept. If not set explicitly, broadcaster is willing to accept ANY price")
	// Unit of pixels for both O's basePriceInfo and B's MaxBroadcastPrice
	cfg.pixelsPerUnit = flag.Int("pixelsPerUnit", *cfg.pixelsPerUnit, "Amount of pixels per unit. Set to '> 1' to have smaller price granularity than 1 wei / pixel")
	cfg.autoAdjustPrice = flag.Bool("autoAdjustPrice", *cfg.autoAdjustPrice, "Enable/disable automatic price adjustments based on the overhead for redeeming tickets")
	// Interval to poll for blocks
	cfg.blockPollingInterval = flag.Int("blockPollingInterval", *cfg.blockPollingInterval, "Interval in seconds at which different blockchain event services poll for blocks")
	// Redemption service
	cfg.redeemer = flag.Bool("redeemer", *cfg.redeemer, "Set to true to run a ticket redemption service")
	cfg.redeemerAddr = flag.String("redeemerAddr", *cfg.redeemerAddr, "URL of the ticket redemption service to use")
	// Reward service
	cfg.reward = flag.Bool("reward", false, "Set to true to run a reward service")
	// Metrics & logging:
	cfg.monitor = flag.Bool("monitor", *cfg.monitor, "Set to true to send performance metrics")
	cfg.metricsPerStream = flag.Bool("metricsPerStream", *cfg.metricsPerStream, "Set to true to group performance metrics per stream")
	cfg.metricsExposeClientIP = flag.Bool("metricsClientIP", *cfg.metricsExposeClientIP, "Set to true to expose client's IP in metrics")
	cfg.metadataQueueUri = flag.String("metadataQueueUri", *cfg.metadataQueueUri, "URI for message broker to send operation metadata")
	cfg.metadataAmqpExchange = flag.String("metadataAmqpExchange", *cfg.metadataAmqpExchange, "Name of AMQP exchange to send operation metadata")
	cfg.metadataPublishTimeout = flag.Duration("metadataPublishTimeout", *cfg.metadataPublishTimeout, "Max time to wait in background for publishing operation metadata events")

	// Storage:
	cfg.datadir = flag.String("datadir", *cfg.datadir, "Directory that data is stored in")
	cfg.objectstore = flag.String("objectStore", *cfg.objectstore, "url of primary object store")
	cfg.recordstore = flag.String("recordStore", *cfg.recordstore, "url of object store for recordings")

	// API
	cfg.authWebhookURL = flag.String("authWebhookUrl", *cfg.authWebhookURL, "RTMP authentication webhook URL")
	cfg.orchWebhookURL = flag.String("orchWebhookUrl", *cfg.orchWebhookURL, "Orchestrator discovery callback URL")
	cfg.detectionWebhookURL = flag.String("detectionWebhookUrl", *cfg.detectionWebhookURL, "(Experimental) Detection results callback URL")

	return cfg
}

// updateNilsForUnsetFlags changes some cfg fields to nil if they were not explicitly set with flags.
// For some flags, the behavior is different whether the value is default or not set by the user at all.
func updateNilsForUnsetFlags(cfg LivepeerConfig) LivepeerConfig {
	res := cfg

	isFlagSet := make(map[string]bool)
	flag.Visit(func(f *flag.Flag) { isFlagSet[f.Name] = true })

	if !isFlagSet["minGasPrice"] {
		res.minGasPrice = nil
	}
	if !isFlagSet["pricePerUnit"] {
		res.pricePerUnit = nil
	}
	if !isFlagSet["reward"] {
		res.reward = nil
	}
	if !isFlagSet["httpIngest"] {
		res.httpIngest = nil
	}
	if !isFlagSet["localVerify"] {
		res.localVerify = nil
	}

	return res
}

func StartLivepeer(ctx context.Context, cfg LivepeerConfig) {
	if *cfg.maxSessions <= 0 {
		glog.Fatal("-maxSessions must be greater than zero")
		return
	}

	blockPollingTime := time.Duration(*cfg.blockPollingInterval) * time.Second

	type NetworkConfig struct {
		ethController string
		minGasPrice   int64
		redeemGas     int
	}

	configOptions := map[string]*NetworkConfig{
		"rinkeby": {
			ethController: "0x9a9827455911a858E55f07911904fACC0D66027E",
			redeemGas:     redeemGasL1,
		},
		"arbitrum-one-rinkeby": {
			ethController: "0x9ceC649179e2C7Ab91688271bcD09fb707b3E574",
			redeemGas:     redeemGasL2,
		},
		"mainnet": {
			ethController: "0xf96d54e490317c557a967abfa5d6e33006be69b3",
			minGasPrice:   int64(params.GWei),
			redeemGas:     redeemGasL1,
		},
		"arbitrum-one-mainnet": {
			ethController: "0xD8E8328501E9645d16Cf49539efC04f734606ee4",
			redeemGas:     redeemGasL2,
		},
	}

	// If multiple orchAddr specified, ensure other necessary flags present and clean up list
	orchURLs := parseOrchAddrs(*cfg.orchAddr)

	// Setting config options based on specified network
	var redeemGas int
	minGasPrice := int64(0)
	if cfg.minGasPrice != nil {
		minGasPrice = *cfg.minGasPrice
	}
	if netw, ok := configOptions[*cfg.network]; ok {
		if *cfg.ethController == "" {
			*cfg.ethController = netw.ethController
		}

		if cfg.minGasPrice == nil {
			minGasPrice = netw.minGasPrice
		}

		redeemGas = netw.redeemGas

		glog.Infof("***Livepeer is running on the %v network: %v***", *cfg.network, *cfg.ethController)
	} else {
		redeemGas = redeemGasL1
		glog.Infof("***Livepeer is running on the %v network***", *cfg.network)
	}

	if *cfg.datadir == "" {
		homedir := os.Getenv("HOME")
		if homedir == "" {
			usr, err := user.Current()
			if err != nil {
				glog.Fatalf("Cannot find current user: %v", err)
			}
			homedir = usr.HomeDir
		}
		*cfg.datadir = filepath.Join(homedir, ".lpData", *cfg.network)
	}

	//Make sure datadir is present
	if _, err := os.Stat(*cfg.datadir); os.IsNotExist(err) {
		glog.Infof("Creating data dir: %v", *cfg.datadir)
		if err = os.MkdirAll(*cfg.datadir, 0755); err != nil {
			glog.Errorf("Error creating datadir: %v", err)
		}
	}

	//Set up DB
	dbh, err := common.InitDB(*cfg.datadir + "/lpdb.sqlite3")
	if err != nil {
		glog.Errorf("Error opening DB: %v", err)
		return
	}
	defer dbh.Close()

	n, err := core.NewLivepeerNode(nil, *cfg.datadir, dbh)
	if err != nil {
		glog.Errorf("Error creating livepeer node: %v", err)
	}

	if *cfg.orchSecret != "" {
		n.OrchSecret, _ = common.GetPass(*cfg.orchSecret)
	}

	var transcoderCaps []core.Capability
	if *cfg.transcoder {
		core.WorkDir = *cfg.datadir
		if *cfg.nvidia != "" {
			// Get a list of device ids
			devices, err := common.ParseNvidiaDevices(*cfg.nvidia)
			if err != nil {
				glog.Fatalf("Error while parsing '-nvidia %v' flag: %v", *cfg.nvidia, err)
			}
			glog.Infof("Transcoding on these Nvidia GPUs: %v", devices)
			// Test transcoding with nvidia
			if *cfg.testTranscoder {
				transcoderCaps, err = core.TestTranscoderCapabilities(devices)
				if err != nil {
					glog.Fatal(err)
				}
			}
			// FIXME: Short-term hack to pre-load the detection models on every device
			if *cfg.sceneClassificationModelPath != "" {
				detectorProfile := ffmpeg.DSceneAdultSoccer
				detectorProfile.ModelPath = *cfg.sceneClassificationModelPath
				core.DetectorProfile = &detectorProfile
				for _, d := range devices {
					tc, err := core.NewNvidiaTranscoderWithDetector(&detectorProfile, d)
					if err != nil {
						glog.Fatalf("Could not initialize detector")
					}
					defer tc.Stop()
				}
			}
			// Initialize LB transcoder
			n.Transcoder = core.NewLoadBalancingTranscoder(devices, core.NewNvidiaTranscoder, core.NewNvidiaTranscoderWithDetector)
		} else {
			// for local software mode, enable all capabilities
			transcoderCaps = append(core.DefaultCapabilities(), core.OptionalCapabilities()...)
			n.Transcoder = core.NewLocalTranscoder(*cfg.datadir)
		}
	}

	if *cfg.redeemer {
		n.NodeType = core.RedeemerNode
	} else if *cfg.orchestrator {
		n.NodeType = core.OrchestratorNode
		if !*cfg.transcoder {
			n.TranscoderManager = core.NewRemoteTranscoderManager()
			n.Transcoder = n.TranscoderManager
		}
	} else if *cfg.transcoder {
		n.NodeType = core.TranscoderNode
	} else if *cfg.broadcaster {
		n.NodeType = core.BroadcasterNode
	} else if (cfg.reward == nil || !*cfg.reward) && !*cfg.initializeRound {
		glog.Fatalf("No services enabled; must be at least one of -broadcaster, -transcoder, -orchestrator, -redeemer, -reward or -initializeRound")
	}

	lpmon.NodeID = *cfg.ethAcctAddr
	if lpmon.NodeID != "" {
		lpmon.NodeID += "-"
	}
	hn, _ := os.Hostname()
	lpmon.NodeID += hn

	if *cfg.monitor {
		if *cfg.metricsExposeClientIP {
			*cfg.metricsPerStream = true
		}
		lpmon.Enabled = true
		lpmon.PerStreamMetrics = *cfg.metricsPerStream
		lpmon.ExposeClientIP = *cfg.metricsExposeClientIP
		nodeType := lpmon.Default
		switch n.NodeType {
		case core.BroadcasterNode:
			nodeType = lpmon.Broadcaster
		case core.OrchestratorNode:
			nodeType = lpmon.Orchestrator
		case core.TranscoderNode:
			nodeType = lpmon.Transcoder
		case core.RedeemerNode:
			nodeType = lpmon.Redeemer
		}
		lpmon.InitCensus(nodeType, core.LivepeerVersion)
	}

	watcherErr := make(chan error)
	serviceErr := make(chan error)
	var timeWatcher *watchers.TimeWatcher
	if *cfg.network == "offchain" {
		glog.Infof("***Livepeer is in off-chain mode***")

		if err := checkOrStoreChainID(dbh, big.NewInt(0)); err != nil {
			glog.Error(err)
			return
		}

	} else {
		var keystoreDir string
		if _, err := os.Stat(*cfg.ethKeystorePath); !os.IsNotExist(err) {
			keystoreDir, _ = filepath.Split(*cfg.ethKeystorePath)
		} else {
			keystoreDir = filepath.Join(*cfg.datadir, "keystore")
		}

		if keystoreDir == "" {
			glog.Errorf("Cannot find keystore directory")
			return
		}

		//Get the Eth client connection information
		if *cfg.ethUrl == "" {
			glog.Fatal("Need to specify an Ethereum node JSON-RPC URL using -ethUrl")
		}

		//Set up eth client
		backend, err := ethclient.Dial(*cfg.ethUrl)
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

		var bigMaxGasPrice *big.Int
		if *cfg.maxGasPrice > 0 {
			bigMaxGasPrice = big.NewInt(int64(*cfg.maxGasPrice))
		}

		gpm := eth.NewGasPriceMonitor(backend, blockPollingTime, big.NewInt(minGasPrice), bigMaxGasPrice)
		// Start gas price monitor
		_, err = gpm.Start(ctx)
		if err != nil {
			glog.Errorf("Error starting gas price monitor: %v", err)
			return
		}
		defer gpm.Stop()

		am, err := eth.NewAccountManager(ethcommon.HexToAddress(*cfg.ethAcctAddr), keystoreDir, chainID)
		if err != nil {
			glog.Errorf("Error creating Ethereum account manager: %v", err)
			return
		}

		if err := am.Unlock(*cfg.ethPassword); err != nil {
			glog.Errorf("Error unlocking Ethereum account: %v", err)
			return
		}

		tm := eth.NewTransactionManager(backend, gpm, am, *cfg.txTimeout, *cfg.maxTxReplacements)
		go tm.Start()
		defer tm.Stop()

		ethCfg := eth.LivepeerEthClientConfig{
			AccountManager:     am,
			ControllerAddr:     ethcommon.HexToAddress(*cfg.ethController),
			EthClient:          backend,
			GasPriceMonitor:    gpm,
			TransactionManager: tm,
			Signer:             types.LatestSignerForChainID(chainID),
			CheckTxTimeout:     time.Duration(int64(*cfg.txTimeout) * int64(*cfg.maxTxReplacements+1)),
		}

		client, err := eth.NewClient(ethCfg)
		if err != nil {
			glog.Errorf("Failed to create Livepeer Ethereum client: %v", err)
			return
		}

		if err := client.SetGasInfo(uint64(*cfg.gasLimit)); err != nil {
			glog.Errorf("Failed to set gas info on Livepeer Ethereum Client: %v", err)
			return
		}
		if err := client.SetMaxGasPrice(bigMaxGasPrice); err != nil {
			glog.Errorf("Failed to set max gas price: %v", err)
			return
		}

		n.Eth = client

		addrMap := n.Eth.ContractAddresses()

		// Initialize block watcher that will emit logs used by event watchers
		blockWatcherClient, err := blockwatch.NewRPCClient(*cfg.ethUrl, ethRPCTimeout)
		if err != nil {
			glog.Errorf("Failed to setup blockwatch client: %v", err)
			return
		}
		topics := watchers.FilterTopics()

		blockWatcherCfg := blockwatch.Config{
			Store:               n.Database,
			PollingInterval:     blockPollingTime,
			StartBlockDepth:     rpc.LatestBlockNumber,
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
		if *cfg.ethOrchAddr != "" {
			recipientAddr = ethcommon.HexToAddress(*cfg.ethOrchAddr)
		}

		smCfg := &pm.LocalSenderMonitorConfig{
			Claimant:        recipientAddr,
			CleanupInterval: cleanupInterval,
			TTL:             smTTL,
			RedeemGas:       redeemGas,
			SuggestGasPrice: client.Backend().SuggestGasPrice,
			RPCTimeout:      ethRPCTimeout,
		}

		if *cfg.orchestrator {
			// Set price per pixel base info
			if *cfg.pixelsPerUnit <= 0 {
				// Can't divide by 0
				panic(fmt.Errorf("-pixelsPerUnit must be > 0, provided %d", *cfg.pixelsPerUnit))
			}
			if cfg.pricePerUnit == nil {
				// Prevent orchestrators from unknowingly providing free transcoding
				panic(fmt.Errorf("-pricePerUnit must be set"))
			}
			if *cfg.pricePerUnit < 0 {
				panic(fmt.Errorf("-pricePerUnit must be >= 0, provided %d", *cfg.pricePerUnit))
			}
			n.SetBasePrice(big.NewRat(int64(*cfg.pricePerUnit), int64(*cfg.pixelsPerUnit)))
			glog.Infof("Price: %d wei for %d pixels\n ", *cfg.pricePerUnit, *cfg.pixelsPerUnit)

			n.AutoAdjustPrice = *cfg.autoAdjustPrice

			ev, _ := new(big.Int).SetString(*cfg.ticketEV, 10)
			if ev == nil {
				glog.Errorf("-ticketEV must be a valid integer, but %v provided. Restart the node with a different valid value for -ticketEV", *cfg.ticketEV)
				return
			}

			if ev.Cmp(big.NewInt(0)) < 0 {
				glog.Errorf("-ticketEV must be greater than 0, but %v provided. Restart the node with a different valid value for -ticketEV", *cfg.ticketEV)
				return
			}

			if err := setupOrchestrator(n, recipientAddr); err != nil {
				glog.Errorf("Error setting up orchestrator: %v", err)
				return
			}

			sigVerifier := &pm.DefaultSigVerifier{}
			validator := pm.NewValidator(sigVerifier, timeWatcher)

			var sm pm.SenderMonitor
			if *cfg.redeemerAddr != "" {
				*cfg.redeemerAddr = defaultAddr(*cfg.redeemerAddr, "127.0.0.1", RpcPort)
				rc, err := server.NewRedeemerClient(*cfg.redeemerAddr, senderWatcher, timeWatcher)
				if err != nil {
					glog.Error("Unable to start redeemer client: ", err)
					return
				}
				sm = rc
			} else {
				sm = pm.NewSenderMonitor(smCfg, n.Eth, senderWatcher, timeWatcher, n.Database)
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
			ev, _ := new(big.Rat).SetString(*cfg.maxTicketEV)
			if ev == nil {
				panic(fmt.Errorf("-maxTicketEV must be a valid rational number, but %v provided. Restart the node with a valid value for -maxTicketEV", *cfg.maxTicketEV))
			}

			if ev.Cmp(big.NewRat(0, 1)) < 0 {
				panic(fmt.Errorf("-maxTicketEV must not be negative, but %v provided. Restart the node with a valid value for -maxTicketEV", *cfg.maxTicketEV))
			}

			if *cfg.depositMultiplier <= 0 {
				panic(fmt.Errorf("-depositMultiplier must be greater than 0, but %v provided. Restart the node with a valid value for -depositMultiplier", *cfg.depositMultiplier))
			}

			// Fetch and cache broadcaster on-chain info
			info, err := senderWatcher.GetSenderInfo(n.Eth.Account().Address)
			if err != nil {
				glog.Error("Failed to get broadcaster on-chain info: ", err)
				return
			}
			glog.Info("Broadcaster Deposit: ", eth.FormatUnits(info.Deposit, "ETH"))
			glog.Info("Broadcaster Reserve: ", eth.FormatUnits(info.Reserve.FundsRemaining, "ETH"))

			n.Sender = pm.NewSender(n.Eth, timeWatcher, senderWatcher, ev, *cfg.depositMultiplier)

			if *cfg.pixelsPerUnit <= 0 {
				// Can't divide by 0
				panic(fmt.Errorf("The amount of pixels per unit must be greater than 0, provided %d instead\n", *cfg.pixelsPerUnit))
			}
			if *cfg.maxPricePerUnit > 0 {
				server.BroadcastCfg.SetMaxPrice(big.NewRat(int64(*cfg.maxPricePerUnit), int64(*cfg.pixelsPerUnit)))
			} else {
				glog.Infof("Maximum transcoding price per pixel is not greater than 0: %v, broadcaster is currently set to accept ANY price.\n", *cfg.maxPricePerUnit)
				glog.Infoln("To update the broadcaster's maximum acceptable transcoding price per pixel, use the CLI or restart the broadcaster with the appropriate 'maxPricePerUnit' and 'pixelsPerUnit' values")
			}
		}

		if n.NodeType == core.RedeemerNode {
			if err := setupOrchestrator(n, recipientAddr); err != nil {
				glog.Errorf("Error setting up orchestrator: %v", err)
				return
			}

			r, err := server.NewRedeemer(
				recipientAddr,
				n.Eth,
				pm.NewSenderMonitor(smCfg, n.Eth, senderWatcher, timeWatcher, n.Database),
			)
			if err != nil {
				glog.Errorf("Unable to create redeemer: %v", err)
				return
			}

			*cfg.httpAddr = defaultAddr(*cfg.httpAddr, "127.0.0.1", RpcPort)
			url, err := url.ParseRequestURI("https://" + *cfg.httpAddr)
			if err != nil {
				glog.Error("Could not parse redeemer URI: ", err)
				return
			}

			go func() {
				if err := r.Start(url, n.WorkDir); err != nil {
					serviceErr <- err
					return
				}
			}()
			defer r.Stop()
			glog.Infof("Redeemer started on %v", *cfg.httpAddr)
		}

		var reward bool
		if cfg.reward == nil {
			// If the node address is an on-chain registered address, start the reward service
			t, err := n.Eth.GetTranscoder(n.Eth.Account().Address)
			if err != nil {
				glog.Error(err)
				return
			}
			if t.Status == "Registered" {
				reward = true
			} else {
				reward = false
			}
		}

		if reward {
			// Start reward service
			// The node will only call reward if it is active in the current round
			rs := eth.NewRewardService(n.Eth, timeWatcher)
			go func() {
				if err := rs.Start(ctx); err != nil {
					serviceErr <- err
				}
				return
			}()
			defer rs.Stop()
		}

		if *cfg.initializeRound {
			// Start round initializer
			// The node will only initialize rounds if it in the upcoming active set for the round
			initializer := eth.NewRoundInitializer(n.Eth, timeWatcher)
			go func() {
				if err := initializer.Start(); err != nil {
					serviceErr <- err
				}
				return
			}()
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

	if *cfg.objectstore != "" {
		prepared, err := drivers.PrepareOSURL(*cfg.objectstore)
		if err != nil {
			glog.Error("Error creating object store driver: ", err)
			return
		}
		drivers.NodeStorage, err = drivers.ParseOSURL(prepared, false)
		if err != nil {
			glog.Error("Error creating object store driver: ", err)
			return
		}
	}

	if *cfg.recordstore != "" {
		prepared, err := drivers.PrepareOSURL(*cfg.recordstore)
		if err != nil {
			glog.Error("Error creating recordings object store driver: ", err)
			return
		}
		drivers.RecordStorage, err = drivers.ParseOSURL(prepared, true)
		if err != nil {
			glog.Error("Error creating recordings object store driver: ", err)
			return
		}
	}

	core.MaxSessions = *cfg.maxSessions
	if lpmon.Enabled {
		lpmon.MaxSessions(core.MaxSessions)
	}

	if *cfg.authWebhookURL != "" {
		parsedUrl, err := validateURL(*cfg.authWebhookURL)
		if err != nil {
			glog.Fatal("Error setting auth webhook URL ", err)
		}
		glog.Info("Using auth webhook URL ", parsedUrl.Redacted())
		server.AuthWebhookURL = parsedUrl
	}

	if *cfg.detectionWebhookURL != "" {
		parsedUrl, err := validateURL(*cfg.detectionWebhookURL)
		if err != nil {
			glog.Fatal("Error setting detection webhook URL ", err)
		}
		glog.Info("Using detection webhook URL ", parsedUrl.Redacted())
		server.DetectionWebhookURL = parsedUrl
	}
	httpIngest := true

	if n.NodeType == core.BroadcasterNode {
		// default lpms listener for broadcaster; same as default rpc port
		// TODO provide an option to disable this?
		*cfg.rtmpAddr = defaultAddr(*cfg.rtmpAddr, "127.0.0.1", RtmpPort)
		*cfg.httpAddr = defaultAddr(*cfg.httpAddr, "127.0.0.1", RpcPort)

		bcast := core.NewBroadcaster(n)

		// When the node is on-chain mode always cache the on-chain orchestrators and poll for updates
		// Right now we rely on the DBOrchestratorPoolCache constructor to do this. Consider separating the logic
		// caching/polling from the logic for fetching orchestrators during discovery
		if *cfg.network != "offchain" {
			ctx, cancel := context.WithCancel(ctx)
			defer cancel()
			dbOrchPoolCache, err := discovery.NewDBOrchestratorPoolCache(ctx, n, timeWatcher)
			if err != nil {
				glog.Errorf("Could not create orchestrator pool with DB cache: %v", err)
			}

			n.OrchestratorPool = dbOrchPoolCache
		}

		// Set up orchestrator discovery
		if *cfg.orchWebhookURL != "" {
			whurl, err := validateURL(*cfg.orchWebhookURL)
			if err != nil {
				glog.Fatal("Error setting orch webhook URL ", err)
			}
			glog.Info("Using orchestrator webhook URL ", whurl)
			n.OrchestratorPool = discovery.NewWebhookPool(bcast, whurl)
		} else if len(orchURLs) > 0 {
			n.OrchestratorPool = discovery.NewOrchestratorPool(bcast, orchURLs, common.Score_Trusted)
		}

		if n.OrchestratorPool == nil {
			// Not a fatal error; may continue operating in segment-only mode
			glog.Error("No orchestrator specified; transcoding will not happen")
		}

		isLocalHTTP, err := isLocalURL("https://" + *cfg.httpAddr)
		if err != nil {
			glog.Errorf("Error checking for local -httpAddr: %v", err)
			return
		}
		if cfg.httpIngest != nil {
			httpIngest = *cfg.httpIngest
		}
		if cfg.httpIngest == nil && !isLocalHTTP && server.AuthWebhookURL == nil {
			glog.Warning("HTTP ingest is disabled because -httpAddr is publicly accessible. To enable, configure -authWebhookUrl or use the -httpIngest flag")
			httpIngest = false
		}

		// Disable local verification when running in off-chain mode
		// To enable, set -localVerify or -verifierURL
		localVerify := true
		if cfg.localVerify != nil {
			localVerify = *cfg.localVerify
		}
		if cfg.localVerify == nil && *cfg.network == "offchain" {
			localVerify = false
		}

		if *cfg.verifierURL != "" {
			_, err := validateURL(*cfg.verifierURL)
			if err != nil {
				glog.Fatal("Error setting verifier URL ", err)
			}
			glog.Info("Using the Epic Labs classifier for verification at ", *cfg.verifierURL)
			server.Policy = &verification.Policy{Retries: 2, Verifier: &verification.EpicClassifier{Addr: *cfg.verifierURL}}

			// Set the verifier path. Remove once [1] is implemented!
			// [1] https://github.com/livepeer/verification-classifier/issues/64
			if drivers.NodeStorage == nil && *cfg.verifierPath == "" {
				glog.Fatal("Requires a path to the verifier shared volume when local storage is in use; use -verifierPath or -objectStore")
			}
			verification.VerifierPath = *cfg.verifierPath
		} else if localVerify {
			glog.Info("Local verification enabled")
			server.Policy = &verification.Policy{Retries: 2}
		}

		// Set max transcode attempts. <=0 is OK; it just means "don't transcode"
		server.MaxAttempts = *cfg.maxAttempts
		server.SelectRandFreq = *cfg.selectRandFreq

	} else if n.NodeType == core.OrchestratorNode {
		suri, err := getServiceURI(n, *cfg.serviceAddr)
		if err != nil {
			glog.Fatal("Error getting service URI: ", err)
		}
		n.SetServiceURI(suri)
		// if http addr is not provided, listen to all ifaces
		// take the port to listen to from the service URI
		*cfg.httpAddr = defaultAddr(*cfg.httpAddr, "", n.GetServiceURI().Port())

		if *cfg.sceneClassificationModelPath != "" {
			// Only enable experimental capabilities if scene classification model is actually loaded
			transcoderCaps = append(transcoderCaps, core.ExperimentalCapabilities()...)
		}
		if !*cfg.transcoder && n.OrchSecret == "" {
			glog.Fatal("Running an orchestrator requires an -orchSecret for standalone mode or -transcoder for orchestrator+transcoder mode")
		}
	}
	n.Capabilities = core.NewCapabilities(transcoderCaps, core.MandatoryOCapabilities())
	*cfg.cliAddr = defaultAddr(*cfg.cliAddr, "127.0.0.1", CliPort)

	if drivers.NodeStorage == nil {
		// base URI will be empty for broadcasters; that's OK
		drivers.NodeStorage = drivers.NewMemoryDriver(n.GetServiceURI())
	}

	if *cfg.metadataPublishTimeout > 0 {
		server.MetadataPublishTimeout = *cfg.metadataPublishTimeout
	}
	if *cfg.metadataQueueUri != "" {
		uri, err := url.ParseRequestURI(*cfg.metadataQueueUri)
		if err != nil {
			glog.Fatalf("Error parsing -metadataQueueUri: err=%q", err)
		}
		switch uri.Scheme {
		case "amqp", "amqps":
			uriStr, exchange, keyNs := *cfg.metadataQueueUri, *cfg.metadataAmqpExchange, n.NodeType.String()
			server.MetadataQueue, err = event.NewAMQPExchangeProducer(context.Background(), uriStr, exchange, keyNs)
			if err != nil {
				glog.Fatalf("Error establishing AMQP connection: err=%q", err)
			}
		default:
			glog.Fatalf("Unsupported scheme in -metadataUri: %s", uri.Scheme)
		}
	}

	//Create Livepeer Node

	//Set up the media server
	s, err := server.NewLivepeerServer(*cfg.rtmpAddr, n, httpIngest, *cfg.transcodingOptions)
	if err != nil {
		glog.Fatal("Error creating Livepeer server err=", err)
	}

	ec := make(chan error)
	tc := make(chan struct{})
	wc := make(chan struct{})
	msCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	if *cfg.currentManifest {
		glog.Info("Current ManifestID will be available over ", *cfg.httpAddr)
		s.ExposeCurrentManifest = *cfg.currentManifest
	}

	go func() {
		s.StartCliWebserver(*cfg.cliAddr)
		close(wc)
	}()
	if n.NodeType != core.RedeemerNode {
		go func() {
			ec <- s.StartMediaServer(msCtx, *cfg.httpAddr)
		}()
	}

	go func() {
		if core.OrchestratorNode != n.NodeType {
			return
		}

		orch := core.NewOrchestrator(s.LivepeerNode, timeWatcher)

		go func() {
			server.StartTranscodeServer(orch, *cfg.httpAddr, s.HTTPMux, n.WorkDir, n.TranscoderManager != nil)
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

	if n.NodeType == core.TranscoderNode {
		if n.OrchSecret == "" {
			glog.Fatal("Missing -orchSecret")
		}
		if len(orchURLs) <= 0 {
			glog.Fatal("Missing -orchAddr")
		}

		go server.RunTranscoder(n, orchURLs[0].Host, *cfg.maxSessions, transcoderCaps)
	}

	switch n.NodeType {
	case core.OrchestratorNode:
		glog.Infof("***Livepeer Running in Orchestrator Mode***")
	case core.BroadcasterNode:
		glog.Infof("***Livepeer Running in Broadcaster Mode***")
		glog.Infof("Video Ingest Endpoint - rtmp://%v", *cfg.rtmpAddr)
	case core.TranscoderNode:
		glog.Infof("**Liveepeer Running in Transcoder Mode***")
	case core.RedeemerNode:
		glog.Infof("**Livepeer Running in Redeemer Mode**")
	}

	select {
	case err := <-watcherErr:
		glog.Error(err)
		return
	case err := <-ec:
		glog.Infof("Error from media server: %v", err)
		return
	case err := <-serviceErr:
		if err != nil {
			glog.Fatalf("Error starting service: %v", err)
		}
	case <-tc:
		glog.Infof("Orchestrator server shut down")
	case <-wc:
		glog.Infof("CLI webserver shut down")
		return
	case <-msCtx.Done():
		glog.Infof("MediaServer Done()")
		return
	case <-ctx.Done():
		return
	}
}

func parseOrchAddrs(addrs string) []*url.URL {
	var res []*url.URL
	if len(addrs) > 0 {
		for _, addr := range strings.Split(addrs, ",") {
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
			res = append(res, uri)
		}
	}
	return res
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
		glog.Errorf("Could not look up public IP err=%q", err)
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		glog.Errorf("Could not look up public IP err=%q", err)
		return nil, err
	}
	addr := "https://" + strings.TrimSpace(string(body)) + ":" + RpcPort
	inferredUri, err := url.ParseRequestURI(addr)
	if err != nil {
		glog.Errorf("Could not look up public IP err=%q", err)
		return nil, err
	}
	if n.Eth == nil {
		// we won't be looking up onchain sURI so use the inferred one
		return inferredUri, err
	}

	// On-chain lookup and matching with inferred public address
	addr, err = n.Eth.GetServiceURI(n.Eth.Account().Address)
	if err != nil {
		glog.Errorf("Could not get service URI; orchestrator may be unreachable err=%q", err)
		return nil, err
	}
	ethUri, err := url.ParseRequestURI(addr)
	if err != nil {
		glog.Errorf("Could not parse service URI; orchestrator may be unreachable err=%q", err)
		ethUri, _ = url.ParseRequestURI("http://127.0.0.1:" + RpcPort)
	}
	if ethUri.Hostname() != inferredUri.Hostname() || ethUri.Port() != inferredUri.Port() {
		glog.Errorf("Service address %v did not match discovered address %v; set the correct address in livepeer_cli or use -serviceAddr", ethUri, inferredUri)
	}
	return ethUri, nil
}

func setupOrchestrator(n *core.LivepeerNode, ethOrchAddr ethcommon.Address) error {
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
