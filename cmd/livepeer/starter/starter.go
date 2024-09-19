package starter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/build"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/discovery"
	"github.com/livepeer/go-livepeer/eth"
	"github.com/livepeer/go-livepeer/eth/blockwatch"
	"github.com/livepeer/go-livepeer/eth/watchers"
	"github.com/livepeer/go-livepeer/monitor"
	lpmon "github.com/livepeer/go-livepeer/monitor"
	"github.com/livepeer/go-livepeer/pm"
	"github.com/livepeer/go-livepeer/server"
	"github.com/livepeer/go-livepeer/verification"
	"github.com/livepeer/go-tools/drivers"
	"github.com/livepeer/livepeer-data/pkg/event"
	"github.com/livepeer/lpms/ffmpeg"
)

var (
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
	cleanupInterval = 10 * time.Minute
	// The time to live for cached max float values for PM senders (else they will be cleaned up) in seconds
	smTTL = 172800 // 2 days
)

const (
	BroadcasterRpcPort  = "9935"
	BroadcasterCliPort  = "5935"
	BroadcasterRtmpPort = "1935"
	OrchestratorRpcPort = "8935"
	OrchestratorCliPort = "7935"
	TranscoderCliPort   = "6935"

	RefreshPerfScoreInterval = 10 * time.Minute
)

type LivepeerConfig struct {
	Network                 *string
	RtmpAddr                *string
	CliAddr                 *string
	HttpAddr                *string
	ServiceAddr             *string
	OrchAddr                *string
	VerifierURL             *string
	EthController           *string
	VerifierPath            *string
	LocalVerify             *bool
	HttpIngest              *bool
	Orchestrator            *bool
	Transcoder              *bool
	Gateway                 *bool
	Broadcaster             *bool
	OrchSecret              *string
	TranscodingOptions      *string
	MaxAttempts             *int
	SelectRandWeight        *float64
	SelectStakeWeight       *float64
	SelectPriceWeight       *float64
	SelectPriceExpFactor    *float64
	OrchPerfStatsURL        *string
	Region                  *string
	MaxPricePerUnit         *string
	IgnoreMaxPriceIfNeeded  *bool
	MinPerfScore            *float64
	DiscoveryTimeout        *time.Duration
	MaxSessions             *string
	CurrentManifest         *bool
	Nvidia                  *string
	Netint                  *string
	HevcDecoding            *bool
	TestTranscoder          *bool
	EthAcctAddr             *string
	EthPassword             *string
	EthKeystorePath         *string
	EthOrchAddr             *string
	EthUrl                  *string
	TxTimeout               *time.Duration
	MaxTxReplacements       *int
	GasLimit                *int
	MinGasPrice             *int64
	MaxGasPrice             *int
	InitializeRound         *bool
	InitializeRoundMaxDelay *time.Duration
	TicketEV                *string
	MaxFaceValue            *string
	MaxTicketEV             *string
	MaxTotalEV              *string
	DepositMultiplier       *int
	PricePerUnit            *string
	PixelsPerUnit           *string
	PriceFeedAddr           *string
	AutoAdjustPrice         *bool
	PricePerGateway         *string
	PricePerBroadcaster     *string
	BlockPollingInterval    *int
	Redeemer                *bool
	RedeemerAddr            *string
	Reward                  *bool
	Monitor                 *bool
	MetricsPerStream        *bool
	MetricsExposeClientIP   *bool
	MetadataQueueUri        *string
	MetadataAmqpExchange    *string
	MetadataPublishTimeout  *time.Duration
	Datadir                 *string
	Objectstore             *string
	Recordstore             *string
	FVfailGsBucket          *string
	FVfailGsKey             *string
	AuthWebhookURL          *string
	OrchWebhookURL          *string
	OrchBlacklist           *string
	OrchMinLivepeerVersion  *string
	TestOrchAvail           *bool
}

// DefaultLivepeerConfig creates LivepeerConfig exactly the same as when no flags are passed to the livepeer process.
func DefaultLivepeerConfig() LivepeerConfig {
	// Network & Addresses:
	defaultNetwork := "offchain"
	defaultRtmpAddr := ""
	defaultCliAddr := ""
	defaultHttpAddr := ""
	defaultServiceAddr := ""
	defaultOrchAddr := ""
	defaultVerifierURL := ""
	defaultVerifierPath := ""

	// Transcoding:
	defaultOrchestrator := false
	defaultTranscoder := false
	defaultBroadcaster := false
	defaultGateway := false
	defaultOrchSecret := ""
	defaultTranscodingOptions := "P240p30fps16x9,P360p30fps16x9"
	defaultMaxAttempts := 3
	defaultSelectRandWeight := 0.3
	defaultSelectStakeWeight := 0.7
	defaultSelectPriceWeight := 0.0
	defaultSelectPriceExpFactor := 100.0
	defaultMaxSessions := strconv.Itoa(10)
	defaultOrchPerfStatsURL := ""
	defaultRegion := ""
	defaultMinPerfScore := 0.0
	defaultDiscoveryTimeout := 500 * time.Millisecond
	defaultCurrentManifest := false
	defaultNvidia := ""
	defaultNetint := ""
	defaultHevcDecoding := false
	defaultTestTranscoder := true

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
	defaultInitializeRoundMaxDelay := 30 * time.Second
	defaultTicketEV := "8000000000"
	defaultMaxFaceValue := "0"
	defaultMaxTicketEV := "3000000000000"
	defaultMaxTotalEV := "20000000000000"
	defaultDepositMultiplier := 1
	defaultMaxPricePerUnit := "0"
	defaultIgnoreMaxPriceIfNeeded := false
	defaultPixelsPerUnit := "1"
	defaultPriceFeedAddr := "0x639Fe6ab55C921f74e7fac1ee960C0B6293ba612" // ETH / USD price feed address on Arbitrum Mainnet
	defaultAutoAdjustPrice := true
	defaultPricePerGateway := ""
	defaultPricePerBroadcaster := ""
	defaultBlockPollingInterval := 5
	defaultRedeemer := false
	defaultRedeemerAddr := ""
	defaultMonitor := false
	defaultMetricsPerStream := false
	defaultMetricsExposeClientIP := false
	defaultMetadataQueueUri := ""
	defaultMetadataAmqpExchange := "lp_golivepeer_metadata"
	defaultMetadataPublishTimeout := 1 * time.Second

	// Ingest:
	defaultHttpIngest := true

	// Verification:
	defaultLocalVerify := true

	// Storage:
	defaultDatadir := ""
	defaultObjectstore := ""
	defaultRecordstore := ""

	// Fast Verification GS bucket:
	defaultFVfailGsBucket := ""
	defaultFVfailGsKey := ""

	// API
	defaultAuthWebhookURL := ""
	defaultOrchWebhookURL := ""
	defaultMinLivepeerVersion := ""

	// Flags
	defaultTestOrchAvail := true

	return LivepeerConfig{
		// Network & Addresses:
		Network:      &defaultNetwork,
		RtmpAddr:     &defaultRtmpAddr,
		CliAddr:      &defaultCliAddr,
		HttpAddr:     &defaultHttpAddr,
		ServiceAddr:  &defaultServiceAddr,
		OrchAddr:     &defaultOrchAddr,
		VerifierURL:  &defaultVerifierURL,
		VerifierPath: &defaultVerifierPath,

		// Transcoding:
		Orchestrator:         &defaultOrchestrator,
		Transcoder:           &defaultTranscoder,
		Gateway:              &defaultGateway,
		Broadcaster:          &defaultBroadcaster,
		OrchSecret:           &defaultOrchSecret,
		TranscodingOptions:   &defaultTranscodingOptions,
		MaxAttempts:          &defaultMaxAttempts,
		SelectRandWeight:     &defaultSelectRandWeight,
		SelectStakeWeight:    &defaultSelectStakeWeight,
		SelectPriceWeight:    &defaultSelectPriceWeight,
		SelectPriceExpFactor: &defaultSelectPriceExpFactor,
		MaxSessions:          &defaultMaxSessions,
		OrchPerfStatsURL:     &defaultOrchPerfStatsURL,
		Region:               &defaultRegion,
		MinPerfScore:         &defaultMinPerfScore,
		DiscoveryTimeout:     &defaultDiscoveryTimeout,
		CurrentManifest:      &defaultCurrentManifest,
		Nvidia:               &defaultNvidia,
		Netint:               &defaultNetint,
		HevcDecoding:         &defaultHevcDecoding,
		TestTranscoder:       &defaultTestTranscoder,

		// Onchain:
		EthAcctAddr:             &defaultEthAcctAddr,
		EthPassword:             &defaultEthPassword,
		EthKeystorePath:         &defaultEthKeystorePath,
		EthOrchAddr:             &defaultEthOrchAddr,
		EthUrl:                  &defaultEthUrl,
		TxTimeout:               &defaultTxTimeout,
		MaxTxReplacements:       &defaultMaxTxReplacements,
		GasLimit:                &defaultGasLimit,
		MaxGasPrice:             &defaultMaxGasPrice,
		EthController:           &defaultEthController,
		InitializeRound:         &defaultInitializeRound,
		InitializeRoundMaxDelay: &defaultInitializeRoundMaxDelay,
		TicketEV:                &defaultTicketEV,
		MaxFaceValue:            &defaultMaxFaceValue,
		MaxTicketEV:             &defaultMaxTicketEV,
		MaxTotalEV:              &defaultMaxTotalEV,
		DepositMultiplier:       &defaultDepositMultiplier,
		MaxPricePerUnit:         &defaultMaxPricePerUnit,
		IgnoreMaxPriceIfNeeded:  &defaultIgnoreMaxPriceIfNeeded,
		PixelsPerUnit:           &defaultPixelsPerUnit,
		PriceFeedAddr:           &defaultPriceFeedAddr,
		AutoAdjustPrice:         &defaultAutoAdjustPrice,
		PricePerGateway:         &defaultPricePerGateway,
		PricePerBroadcaster:     &defaultPricePerBroadcaster,
		BlockPollingInterval:    &defaultBlockPollingInterval,
		Redeemer:                &defaultRedeemer,
		RedeemerAddr:            &defaultRedeemerAddr,
		Monitor:                 &defaultMonitor,
		MetricsPerStream:        &defaultMetricsPerStream,
		MetricsExposeClientIP:   &defaultMetricsExposeClientIP,
		MetadataQueueUri:        &defaultMetadataQueueUri,
		MetadataAmqpExchange:    &defaultMetadataAmqpExchange,
		MetadataPublishTimeout:  &defaultMetadataPublishTimeout,

		// Ingest:
		HttpIngest: &defaultHttpIngest,

		// Verification:
		LocalVerify: &defaultLocalVerify,

		// Storage:
		Datadir:     &defaultDatadir,
		Objectstore: &defaultObjectstore,
		Recordstore: &defaultRecordstore,

		// Fast Verification GS bucket:
		FVfailGsBucket: &defaultFVfailGsBucket,
		FVfailGsKey:    &defaultFVfailGsKey,

		// API
		AuthWebhookURL:         &defaultAuthWebhookURL,
		OrchWebhookURL:         &defaultOrchWebhookURL,
		OrchMinLivepeerVersion: &defaultMinLivepeerVersion,

		// Flags
		TestOrchAvail: &defaultTestOrchAvail,
	}
}

func StartLivepeer(ctx context.Context, cfg LivepeerConfig) {
	if *cfg.MaxSessions == "auto" && *cfg.Orchestrator {
		if *cfg.Transcoder {
			glog.Exit("-maxSessions 'auto' cannot be used when both -orchestrator and -transcoder are specified")
		}
		core.MaxSessions = 0
	} else {
		intMaxSessions, err := strconv.Atoi(*cfg.MaxSessions)
		if err != nil || intMaxSessions <= 0 {
			glog.Exit("-maxSessions must be 'auto' or greater than zero")
		}

		core.MaxSessions = intMaxSessions
	}

	if *cfg.Netint != "" && *cfg.Nvidia != "" {
		glog.Exit("both -netint and -nvidia arguments specified, this is not supported")
	}

	blockPollingTime := time.Duration(*cfg.BlockPollingInterval) * time.Second

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

	if *cfg.Network == "rinkeby" || *cfg.Network == "arbitrum-one-rinkeby" {
		glog.Warning("The Rinkeby/ArbRinkeby networks are deprecated in favor of the Goerli/ArbGoerli networks which will be launched in January 2023.")
	}

	// If multiple orchAddr specified, ensure other necessary flags present and clean up list
	orchURLs := parseOrchAddrs(*cfg.OrchAddr)

	// Setting config options based on specified network
	var redeemGas int
	minGasPrice := int64(0)
	if cfg.MinGasPrice != nil {
		minGasPrice = *cfg.MinGasPrice
	}
	if netw, ok := configOptions[*cfg.Network]; ok {
		if *cfg.EthController == "" {
			*cfg.EthController = netw.ethController
		}

		if cfg.MinGasPrice == nil {
			minGasPrice = netw.minGasPrice
		}

		redeemGas = netw.redeemGas

		glog.Infof("***Livepeer is running on the %v network: %v***", *cfg.Network, *cfg.EthController)
	} else {
		redeemGas = redeemGasL1
		glog.Infof("***Livepeer is running on the %v network***", *cfg.Network)
	}

	if *cfg.Datadir == "" {
		homedir := os.Getenv("HOME")
		if homedir == "" {
			usr, err := user.Current()
			if err != nil {
				exit("Cannot find current user: %v", err)
			}
			homedir = usr.HomeDir
		}
		*cfg.Datadir = filepath.Join(homedir, ".lpData", *cfg.Network)
	}

	//Make sure datadir is present
	if _, err := os.Stat(*cfg.Datadir); os.IsNotExist(err) {
		glog.Infof("Creating data dir: %v", *cfg.Datadir)
		if err = os.MkdirAll(*cfg.Datadir, 0755); err != nil {
			glog.Errorf("Error creating datadir: %v", err)
		}
	}

	//Set Gs bucket for fast verification fail case
	if *cfg.FVfailGsBucket != "" && *cfg.FVfailGsKey != "" {
		drivers.SetCreds(*cfg.FVfailGsBucket, *cfg.FVfailGsKey)
	}

	//Set up DB
	dbh, err := common.InitDB(*cfg.Datadir + "/lpdb.sqlite3")
	if err != nil {
		glog.Errorf("Error opening DB: %v", err)
		return
	}
	defer dbh.Close()

	n, err := core.NewLivepeerNode(nil, *cfg.Datadir, dbh)
	if err != nil {
		glog.Errorf("Error creating livepeer node: %v", err)
	}

	if *cfg.OrchSecret != "" {
		n.OrchSecret, _ = common.ReadFromFile(*cfg.OrchSecret)
	}

	var transcoderCaps []core.Capability
	if *cfg.Transcoder {
		core.WorkDir = *cfg.Datadir
		accel := ffmpeg.Software
		var devicesStr string
		if *cfg.Nvidia != "" {
			accel = ffmpeg.Nvidia
			devicesStr = *cfg.Nvidia
		}
		if *cfg.Netint != "" {
			accel = ffmpeg.Netint
			devicesStr = *cfg.Netint
		}
		if accel != ffmpeg.Software {
			accelName := ffmpeg.AccelerationNameLookup[accel]
			tf, err := core.GetTranscoderFactoryByAccel(accel)
			if err != nil {
				exit("Error unsupported acceleration: %v", err)
			}
			// Get a list of device ids
			devices, err := common.ParseAccelDevices(devicesStr, accel)
			glog.Infof("%v devices: %v", accelName, devices)
			if err != nil {
				exit("Error while parsing '-%v %v' flag: %v", strings.ToLower(accelName), devices, err)
			}
			glog.Infof("Transcoding on these %v devices: %v", accelName, devices)
			// Test transcoding with specified device
			if *cfg.TestTranscoder {
				transcoderCaps, err = core.TestTranscoderCapabilities(devices, tf)
				if err != nil {
					glog.Exit(err)
				}
			} else {
				// no capability test was run, assume default capabilities
				transcoderCaps = append(transcoderCaps, core.DefaultCapabilities()...)
			}
			// Initialize LB transcoder
			n.Transcoder = core.NewLoadBalancingTranscoder(devices, tf)
		} else {
			// for local software mode, enable most capabilities but remove expensive decoders and non-H264 encoders
			capsToRemove := []core.Capability{core.Capability_HEVC_Decode, core.Capability_HEVC_Encode, core.Capability_VP8_Encode, core.Capability_VP9_Decode, core.Capability_VP9_Encode}
			caps := core.OptionalCapabilities()
			for _, c := range capsToRemove {
				caps = core.RemoveCapability(caps, c)
			}
			transcoderCaps = append(core.DefaultCapabilities(), caps...)
			n.Transcoder = core.NewLocalTranscoder(*cfg.Datadir)
		}

		if cfg.HevcDecoding == nil {
			// do nothing; keep defaults
		} else if *cfg.HevcDecoding {
			if !core.HasCapability(transcoderCaps, core.Capability_HEVC_Decode) {
				if accel != ffmpeg.Software {
					glog.Info("Enabling HEVC decoding when the hardware does not support it")
				} else {
					glog.Info("Enabling HEVC decoding on CPU, may be slow")
				}
				transcoderCaps = core.AddCapability(transcoderCaps, core.Capability_HEVC_Decode)
			}
		} else if !*cfg.HevcDecoding {
			transcoderCaps = core.RemoveCapability(transcoderCaps, core.Capability_HEVC_Decode)
		}
	}

	if *cfg.Redeemer {
		n.NodeType = core.RedeemerNode
	} else if *cfg.Orchestrator {
		n.NodeType = core.OrchestratorNode
		if !*cfg.Transcoder {
			n.TranscoderManager = core.NewRemoteTranscoderManager()
			n.Transcoder = n.TranscoderManager
		}
	} else if *cfg.Transcoder {
		n.NodeType = core.TranscoderNode
	} else if *cfg.Broadcaster {
		n.NodeType = core.BroadcasterNode
		glog.Warning("-broadcaster flag is deprecated and will be removed in a future release. Please use -gateway instead")
	} else if *cfg.Gateway {
		n.NodeType = core.BroadcasterNode
	} else if (cfg.Reward == nil || !*cfg.Reward) && !*cfg.InitializeRound {
		exit("No services enabled; must be at least one of -gateway, -transcoder, -orchestrator, -redeemer, -reward or -initializeRound")
	}

	lpmon.NodeID = *cfg.EthAcctAddr
	if lpmon.NodeID != "" {
		lpmon.NodeID += "-"
	}
	hn, _ := os.Hostname()
	lpmon.NodeID += hn

	if *cfg.Monitor {
		if *cfg.MetricsExposeClientIP {
			*cfg.MetricsPerStream = true
		}
		lpmon.Enabled = true
		lpmon.PerStreamMetrics = *cfg.MetricsPerStream
		lpmon.ExposeClientIP = *cfg.MetricsExposeClientIP
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
	if *cfg.Network == "offchain" {
		glog.Infof("***Livepeer is in off-chain mode***")

		if err := checkOrStoreChainID(dbh, big.NewInt(0)); err != nil {
			glog.Error(err)
			return
		}

	} else {
		n.SelectionAlgorithm, err = createSelectionAlgorithm(cfg)
		if err != nil {
			exit("Incorrect parameters for selection algorithm, err=%v", err)
		}

		var keystoreDir = filepath.Join(*cfg.Datadir, "keystore")
		keystoreInfo, err := parseEthKeystorePath(*cfg.EthKeystorePath)
		if err == nil {
			if keystoreInfo.path != "" {
				keystoreDir = keystoreInfo.path
			} else if (keystoreInfo.address != ethcommon.Address{}) {
				ethKeystoreAddr := keystoreInfo.address.Hex()
				ethAcctAddr := ethcommon.HexToAddress(*cfg.EthAcctAddr).Hex()

				if (ethAcctAddr == ethcommon.Address{}.Hex()) || ethKeystoreAddr == ethAcctAddr {
					*cfg.EthAcctAddr = ethKeystoreAddr
				} else {
					glog.Exit("-ethKeystorePath and -ethAcctAddr were both provided, but ethAcctAddr does not match the address found in keystore")
				}
			}
		} else {
			glog.Exit(fmt.Errorf(err.Error()))
		}

		//Get the Eth client connection information
		if *cfg.EthUrl == "" {
			glog.Exit("Need to specify an Ethereum node JSON-RPC URL using -ethUrl")
		}

		//Set up eth client
		backend, err := ethclient.Dial(*cfg.EthUrl)
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
		if *cfg.MaxGasPrice > 0 {
			bigMaxGasPrice = big.NewInt(int64(*cfg.MaxGasPrice))
		}

		gpm := eth.NewGasPriceMonitor(backend, blockPollingTime, big.NewInt(minGasPrice), bigMaxGasPrice)
		// Start gas price monitor
		_, err = gpm.Start(ctx)
		if err != nil {
			glog.Errorf("Error starting gas price monitor: %v", err)
			return
		}
		defer gpm.Stop()

		am, err := eth.NewAccountManager(ethcommon.HexToAddress(*cfg.EthAcctAddr), keystoreDir, chainID, *cfg.EthPassword)
		if err != nil {
			glog.Errorf("Error creating Ethereum account manager: %v", err)
			return
		}

		if err := am.Unlock(*cfg.EthPassword); err != nil {
			glog.Errorf("Error unlocking Ethereum account: %v", err)
			return
		}

		tm := eth.NewTransactionManager(backend, gpm, am, *cfg.TxTimeout, *cfg.MaxTxReplacements)
		go tm.Start()
		defer tm.Stop()

		ethCfg := eth.LivepeerEthClientConfig{
			AccountManager:     am,
			ControllerAddr:     ethcommon.HexToAddress(*cfg.EthController),
			EthClient:          backend,
			GasPriceMonitor:    gpm,
			TransactionManager: tm,
			Signer:             types.LatestSignerForChainID(chainID),
			CheckTxTimeout:     time.Duration(int64(*cfg.TxTimeout) * int64(*cfg.MaxTxReplacements+1)),
		}

		client, err := eth.NewClient(ethCfg)
		if err != nil {
			glog.Errorf("Failed to create Livepeer Ethereum client: %v", err)
			return
		}

		if err := client.SetGasInfo(uint64(*cfg.GasLimit)); err != nil {
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
		blockWatcherClient, err := blockwatch.NewRPCClient(*cfg.EthUrl, ethRPCTimeout)
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

		core.PriceFeedWatcher, err = watchers.NewPriceFeedWatcher(backend, *cfg.PriceFeedAddr)
		// The price feed watch loop is started on demand on first subscribe.
		if err != nil {
			glog.Errorf("Failed to set up price feed watcher: %v", err)
			return
		}

		n.Balances = core.NewAddressBalances(cleanupInterval)
		defer n.Balances.StopCleanup()

		// By default the ticket recipient is the node's address
		// If the address of an on-chain registered orchestrator is provided, then it should be specified as the ticket recipient
		recipientAddr := n.Eth.Account().Address
		if *cfg.EthOrchAddr != "" {
			recipientAddr = ethcommon.HexToAddress(*cfg.EthOrchAddr)
		}

		smCfg := &pm.LocalSenderMonitorConfig{
			Claimant:        recipientAddr,
			CleanupInterval: cleanupInterval,
			TTL:             smTTL,
			RedeemGas:       redeemGas,
			SuggestGasPrice: client.Backend().SuggestGasPrice,
			RPCTimeout:      ethRPCTimeout,
		}

		if *cfg.Orchestrator {
			// Set price per pixel base info
			pixelsPerUnit, ok := new(big.Rat).SetString(*cfg.PixelsPerUnit)
			if !ok || !pixelsPerUnit.IsInt() {
				panic(fmt.Errorf("-pixelsPerUnit must be a valid integer, provided %v", *cfg.PixelsPerUnit))
			}
			if pixelsPerUnit.Sign() <= 0 {
				// Can't divide by 0
				panic(fmt.Errorf("-pixelsPerUnit must be > 0, provided %v", *cfg.PixelsPerUnit))
			}
			if cfg.PricePerUnit == nil {
				// Prevent orchestrators from unknowingly providing free transcoding
				panic(fmt.Errorf("-pricePerUnit must be set"))
			}
			pricePerUnit, currency, err := parsePricePerUnit(*cfg.PricePerUnit)
			if err != nil {
				panic(fmt.Errorf("-pricePerUnit must be a valid integer with an optional currency, provided %v", *cfg.PricePerUnit))
			} else if pricePerUnit.Sign() < 0 {
				panic(fmt.Errorf("-pricePerUnit must be >= 0, provided %s", pricePerUnit))
			}
			pricePerPixel := new(big.Rat).Quo(pricePerUnit, pixelsPerUnit)
			autoPrice, err := core.NewAutoConvertedPrice(currency, pricePerPixel, func(price *big.Rat) {
				glog.Infof("Price: %v wei per pixel\n ", price.FloatString(3))
			})
			if err != nil {
				panic(fmt.Errorf("Error converting price: %v", err))
			}
			n.SetBasePrice("default", autoPrice)

			if *cfg.PricePerBroadcaster != "" {
				glog.Warning("-PricePerBroadcaster flag is deprecated and will be removed in a future release. Please use -PricePerGateway instead")
				cfg.PricePerGateway = cfg.PricePerBroadcaster
			}
			gatewayPrices := getGatewayPrices(*cfg.PricePerGateway)
			for _, p := range gatewayPrices {
				p := p
				pricePerPixel := new(big.Rat).Quo(p.PricePerUnit, p.PixelsPerUnit)
				autoPrice, err := core.NewAutoConvertedPrice(p.Currency, pricePerPixel, func(price *big.Rat) {
					glog.Infof("Price: %v wei per pixel for gateway %v", price.FloatString(3), p.EthAddress)
				})
				if err != nil {
					panic(fmt.Errorf("Error converting price for gateway %s: %v", p.EthAddress, err))
				}
				n.SetBasePrice(p.EthAddress, autoPrice)
			}

			n.AutoSessionLimit = *cfg.MaxSessions == "auto"
			n.AutoAdjustPrice = *cfg.AutoAdjustPrice

			ev, _ := new(big.Int).SetString(*cfg.TicketEV, 10)
			if ev == nil {
				glog.Errorf("-ticketEV must be a valid integer, but %v provided. Restart the node with a different valid value for -ticketEV", *cfg.TicketEV)
				return
			}

			if ev.Cmp(big.NewInt(0)) < 0 {
				glog.Errorf("-ticketEV must be greater than 0, but %v provided. Restart the node with a different valid value for -ticketEV", *cfg.TicketEV)
				return
			}

			if err := setupOrchestrator(n, recipientAddr); err != nil {
				glog.Errorf("Error setting up orchestrator: %v", err)
				return
			}
			n.RecipientAddr = recipientAddr.Hex()

			sigVerifier := &pm.DefaultSigVerifier{}
			validator := pm.NewValidator(sigVerifier, timeWatcher)

			var sm pm.SenderMonitor
			if *cfg.RedeemerAddr != "" {
				*cfg.RedeemerAddr = defaultAddr(*cfg.RedeemerAddr, "127.0.0.1", OrchestratorRpcPort)
				rc, err := server.NewRedeemerClient(*cfg.RedeemerAddr, senderWatcher, timeWatcher)
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

			tcfg := pm.TicketParamsConfig{
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
				tcfg,
			)
			if err != nil {
				glog.Errorf("Error setting up PM recipient: %v", err)
				return
			}
			mfv, _ := new(big.Int).SetString(*cfg.MaxFaceValue, 10)
			if mfv == nil {
				panic(fmt.Errorf("-maxFaceValue must be a valid integer, but %v provided. Restart the node with a different valid value for -maxFaceValue", *cfg.MaxFaceValue))
				return
			} else {
				n.SetMaxFaceValue(mfv)
			}

		}
		if n.NodeType == core.BroadcasterNode {
			maxEV, _ := new(big.Rat).SetString(*cfg.MaxTicketEV)
			if maxEV == nil {
				panic(fmt.Errorf("-maxTicketEV must be a valid rational number, but %v provided. Restart the node with a valid value for -maxTicketEV", *cfg.MaxTicketEV))
			}
			if maxEV.Cmp(big.NewRat(0, 1)) < 0 {
				panic(fmt.Errorf("-maxTicketEV must not be negative, but %v provided. Restart the node with a valid value for -maxTicketEV", *cfg.MaxTicketEV))
			}
			maxTotalEV, _ := new(big.Rat).SetString(*cfg.MaxTotalEV)
			if maxTotalEV.Cmp(big.NewRat(0, 1)) < 0 {
				panic(fmt.Errorf("-maxTotalEV must not be negative, but %v provided. Restart the node with a valid value for -maxTotalEV", *cfg.MaxTotalEV))
			}

			if *cfg.DepositMultiplier <= 0 {
				panic(fmt.Errorf("-depositMultiplier must be greater than 0, but %v provided. Restart the node with a valid value for -depositMultiplier", *cfg.DepositMultiplier))
			}

			// Fetch and cache broadcaster on-chain info
			info, err := senderWatcher.GetSenderInfo(n.Eth.Account().Address)
			if err != nil {
				glog.Error("Failed to get broadcaster on-chain info: ", err)
				return
			}
			glog.Info("Broadcaster Deposit: ", eth.FormatUnits(info.Deposit, "ETH"))
			glog.Info("Broadcaster Reserve: ", eth.FormatUnits(info.Reserve.FundsRemaining, "ETH"))

			n.Sender = pm.NewSender(n.Eth, timeWatcher, senderWatcher, maxEV, maxTotalEV, *cfg.DepositMultiplier)

			pixelsPerUnit, ok := new(big.Rat).SetString(*cfg.PixelsPerUnit)
			if !ok || !pixelsPerUnit.IsInt() {
				panic(fmt.Errorf("-pixelsPerUnit must be a valid integer, provided %v", *cfg.PixelsPerUnit))
			}
			if pixelsPerUnit.Sign() <= 0 {
				// Can't divide by 0
				panic(fmt.Errorf("-pixelsPerUnit must be > 0, provided %v", *cfg.PixelsPerUnit))
			}
			maxPricePerUnit, currency, err := parsePricePerUnit(*cfg.MaxPricePerUnit)
			if err != nil {
				panic(fmt.Errorf("The maximum price per unit must be a valid integer with an optional currency, provided %v instead\n", *cfg.MaxPricePerUnit))
			}
			if maxPricePerUnit.Sign() > 0 {
				pricePerPixel := new(big.Rat).Quo(maxPricePerUnit, pixelsPerUnit)
				autoPrice, err := core.NewAutoConvertedPrice(currency, pricePerPixel, func(price *big.Rat) {
					if monitor.Enabled {
						monitor.MaxTranscodingPrice(price)
					}
					glog.Infof("Maximum transcoding price: %v wei per pixel\n ", price.FloatString(3))
				})
				if err != nil {
					panic(fmt.Errorf("Error converting price: %v", err))
				}
				server.BroadcastCfg.SetMaxPrice(autoPrice)
			} else {
				glog.Infof("Maximum transcoding price per pixel is not greater than 0: %v, broadcaster is currently set to accept ANY price.\n", *cfg.MaxPricePerUnit)
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

			*cfg.HttpAddr = defaultAddr(*cfg.HttpAddr, "127.0.0.1", OrchestratorRpcPort)
			url, err := url.ParseRequestURI("https://" + *cfg.HttpAddr)
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
			glog.Infof("Redeemer started on %v", *cfg.HttpAddr)
		}

		var reward bool
		if cfg.Reward == nil {
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
		} else {
			reward = *cfg.Reward
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

		if *cfg.InitializeRound {
			// Start round initializer
			// The node will only initialize rounds if it in the upcoming active set for the round
			initializer := eth.NewRoundInitializer(n.Eth, timeWatcher, *cfg.InitializeRoundMaxDelay)
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
		glog.Infof("Backfilling block events (this can take a while)...\n")
		if err := blockWatcher.BackfillEvents(blockWatchCtx, nil); err != nil {
			glog.Errorf("Failed to backfill events: %v", err)
			return
		}
		glog.Info("Done backfilling block events")

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

	if *cfg.Objectstore != "" {
		prepared, err := drivers.PrepareOSURL(*cfg.Objectstore)
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

	if *cfg.Recordstore != "" {
		prepared, err := drivers.PrepareOSURL(*cfg.Recordstore)
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

	if lpmon.Enabled {
		lpmon.MaxSessions(core.MaxSessions)
	}

	if *cfg.AuthWebhookURL != "" {
		parsedUrl, err := validateURL(*cfg.AuthWebhookURL)
		if err != nil {
			glog.Exit("Error setting auth webhook URL ", err)
		}
		glog.Info("Using auth webhook URL ", parsedUrl.Redacted())
		server.AuthWebhookURL = parsedUrl
	}

	httpIngest := true

	if n.NodeType == core.BroadcasterNode {
		// default lpms listener for broadcaster; same as default rpc port
		// TODO provide an option to disable this?
		*cfg.RtmpAddr = defaultAddr(*cfg.RtmpAddr, "127.0.0.1", BroadcasterRtmpPort)
		*cfg.HttpAddr = defaultAddr(*cfg.HttpAddr, "127.0.0.1", BroadcasterRpcPort)
		*cfg.CliAddr = defaultAddr(*cfg.CliAddr, "127.0.0.1", BroadcasterCliPort)

		bcast := core.NewBroadcaster(n)
		orchBlacklist := parseOrchBlacklist(cfg.OrchBlacklist)
		if *cfg.OrchPerfStatsURL != "" && *cfg.Region != "" {
			glog.Infof("Using Performance Stats, region=%s, URL=%s, minPerfScore=%v", *cfg.Region, *cfg.OrchPerfStatsURL, *cfg.MinPerfScore)
			n.OrchPerfScore = &common.PerfScore{Scores: make(map[ethcommon.Address]float64)}
			go refreshOrchPerfScoreLoop(ctx, strings.ToUpper(*cfg.Region), *cfg.OrchPerfStatsURL, n.OrchPerfScore)
		}

		// When the node is on-chain mode always cache the on-chain orchestrators and poll for updates
		// Right now we rely on the DBOrchestratorPoolCache constructor to do this. Consider separating the logic
		// caching/polling from the logic for fetching orchestrators during discovery
		if *cfg.Network != "offchain" {
			ctx, cancel := context.WithCancel(ctx)
			defer cancel()
			dbOrchPoolCache, err := discovery.NewDBOrchestratorPoolCache(ctx, n, timeWatcher, orchBlacklist, *cfg.DiscoveryTimeout)
			if err != nil {
				exit("Could not create orchestrator pool with DB cache: %v", err)
			}

			n.OrchestratorPool = dbOrchPoolCache
		}

		// Set up orchestrator discovery
		if *cfg.OrchWebhookURL != "" {
			whurl, err := validateURL(*cfg.OrchWebhookURL)
			if err != nil {
				glog.Exit("Error setting orch webhook URL ", err)
			}
			glog.Info("Using orchestrator webhook URL ", whurl)
			n.OrchestratorPool = discovery.NewWebhookPool(bcast, whurl, *cfg.DiscoveryTimeout)
		} else if len(orchURLs) > 0 {
			n.OrchestratorPool = discovery.NewOrchestratorPool(bcast, orchURLs, common.Score_Trusted, orchBlacklist, *cfg.DiscoveryTimeout)
		}

		if n.OrchestratorPool == nil {
			// Not a fatal error; may continue operating in segment-only mode
			glog.Error("No orchestrator specified; transcoding will not happen")
		}

		isLocalHTTP, err := isLocalURL("https://" + *cfg.HttpAddr)
		if err != nil {
			glog.Errorf("Error checking for local -httpAddr: %v", err)
			return
		}
		if cfg.HttpIngest != nil {
			httpIngest = *cfg.HttpIngest
		}
		if cfg.HttpIngest == nil && !isLocalHTTP && server.AuthWebhookURL == nil {
			glog.Warning("HTTP ingest is disabled because -httpAddr is publicly accessible. To enable, configure -authWebhookUrl or use the -httpIngest flag")
			httpIngest = false
		}

		// Disable local verification when running in off-chain mode
		// To enable, set -localVerify or -verifierURL
		localVerify := true
		if cfg.LocalVerify != nil {
			localVerify = *cfg.LocalVerify
		}
		if cfg.LocalVerify == nil && *cfg.Network == "offchain" {
			localVerify = false
		}

		if *cfg.VerifierURL != "" {
			_, err := validateURL(*cfg.VerifierURL)
			if err != nil {
				glog.Exit("Error setting verifier URL ", err)
			}
			glog.Info("Using the Epic Labs classifier for verification at ", *cfg.VerifierURL)
			server.Policy = &verification.Policy{Retries: 2, Verifier: &verification.EpicClassifier{Addr: *cfg.VerifierURL}}

			// Set the verifier path. Remove once [1] is implemented!
			// [1] https://github.com/livepeer/verification-classifier/issues/64
			if drivers.NodeStorage == nil && *cfg.VerifierPath == "" {
				glog.Exit("Requires a path to the verifier shared volume when local storage is in use; use -verifierPath or -objectStore")
			}
			verification.VerifierPath = *cfg.VerifierPath
		} else if localVerify {
			glog.Info("Local verification enabled")
			server.Policy = &verification.Policy{Retries: 2}
		}

		// Set max transcode attempts. <=0 is OK; it just means "don't transcode"
		server.MaxAttempts = *cfg.MaxAttempts

	} else if n.NodeType == core.OrchestratorNode {
		*cfg.CliAddr = defaultAddr(*cfg.CliAddr, "127.0.0.1", OrchestratorCliPort)

		suri, err := getServiceURI(n, *cfg.ServiceAddr)
		if err != nil {
			glog.Exit("Error getting service URI: ", err)
		}

		if *cfg.Network != "offchain" && !common.ValidateServiceURI(suri) {
			glog.Warning("**Warning -serviceAddr is a not a public address or hostname; this is not recommended for onchain networks**")
		}

		n.SetServiceURI(suri)
		// if http addr is not provided, listen to all ifaces
		// take the port to listen to from the service URI
		*cfg.HttpAddr = defaultAddr(*cfg.HttpAddr, "", n.GetServiceURI().Port())
		if !*cfg.Transcoder && n.OrchSecret == "" {
			glog.Exit("Running an orchestrator requires an -orchSecret for standalone mode or -transcoder for orchestrator+transcoder mode")
		}
	} else if n.NodeType == core.TranscoderNode {
		*cfg.CliAddr = defaultAddr(*cfg.CliAddr, "127.0.0.1", TranscoderCliPort)
	}

	n.Capabilities = core.NewCapabilities(transcoderCaps, core.MandatoryOCapabilities())
	if cfg.OrchMinLivepeerVersion != nil {
		n.Capabilities.SetMinVersionConstraint(*cfg.OrchMinLivepeerVersion)
	}

	if drivers.NodeStorage == nil {
		// base URI will be empty for broadcasters; that's OK
		drivers.NodeStorage = drivers.NewMemoryDriver(n.GetServiceURI())
	}

	if *cfg.MetadataPublishTimeout > 0 {
		server.MetadataPublishTimeout = *cfg.MetadataPublishTimeout
	}
	if *cfg.MetadataQueueUri != "" {
		uri, err := url.ParseRequestURI(*cfg.MetadataQueueUri)
		if err != nil {
			exit("Error parsing -metadataQueueUri: err=%q", err)
		}
		switch uri.Scheme {
		case "amqp", "amqps":
			uriStr, exchange, keyNs := *cfg.MetadataQueueUri, *cfg.MetadataAmqpExchange, n.NodeType.String()
			server.MetadataQueue, err = event.NewAMQPExchangeProducer(context.Background(), uriStr, exchange, keyNs)
			if err != nil {
				exit("Error establishing AMQP connection: err=%q", err)
			}
		default:
			exit("Unsupported scheme in -metadataUri: %s", uri.Scheme)
		}
	}

	//Create Livepeer Node

	//Set up the media server
	s, err := server.NewLivepeerServer(*cfg.RtmpAddr, n, httpIngest, *cfg.TranscodingOptions)
	if err != nil {
		exit("Error creating Livepeer server: err=%q", err)
	}

	ec := make(chan error)
	tc := make(chan struct{})
	wc := make(chan struct{})
	msCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	if *cfg.CurrentManifest {
		glog.Info("Current ManifestID will be available over ", *cfg.HttpAddr)
		s.ExposeCurrentManifest = *cfg.CurrentManifest
	}
	srv := &http.Server{Addr: *cfg.CliAddr}
	go func() {
		s.StartCliWebserver(srv)
		close(wc)
	}()
	if n.NodeType != core.RedeemerNode {
		go func() {
			ec <- s.StartMediaServer(msCtx, *cfg.HttpAddr)
		}()
	}

	go func() {
		if core.OrchestratorNode != n.NodeType {
			return
		}

		orch := core.NewOrchestrator(s.LivepeerNode, timeWatcher)

		go func() {
			err = server.StartTranscodeServer(orch, *cfg.HttpAddr, s.HTTPMux, n.WorkDir, n.TranscoderManager != nil, n)
			if err != nil {
				exit("Error starting Transcoder node: err=%q", err)
			}
			tc <- struct{}{}
		}()

		// check whether or not the orchestrator is available
		if *cfg.TestOrchAvail {
			time.Sleep(2 * time.Second)
			orchAvail := server.CheckOrchestratorAvailability(orch)
			if !orchAvail {
				// shut down orchestrator
				glog.Infof("Orchestrator not available at %v; shutting down", orch.ServiceURI())
				tc <- struct{}{}
			}
		}

	}()

	if n.NodeType == core.TranscoderNode {
		if n.OrchSecret == "" {
			glog.Exit("Missing -orchSecret")
		}
		if len(orchURLs) <= 0 {
			glog.Exit("Missing -orchAddr")
		}

		go server.RunTranscoder(n, orchURLs[0].Host, core.MaxSessions, transcoderCaps)
	}

	switch n.NodeType {
	case core.OrchestratorNode:
		glog.Infof("***Livepeer Running in Orchestrator Mode***")
	case core.BroadcasterNode:
		glog.Infof("***Livepeer Running in Gateway Mode***")
		glog.Infof("Video Ingest Endpoint - rtmp://%v", *cfg.RtmpAddr)
	case core.TranscoderNode:
		glog.Infof("**Liveepeer Running in Transcoder Mode***")
	case core.RedeemerNode:
		glog.Infof("**Livepeer Running in Redeemer Mode**")
	}

	glog.Infof("Livepeer Node version: %v", core.LivepeerVersion)

	select {
	case err := <-watcherErr:
		glog.Error(err)
		return
	case err := <-ec:
		glog.Infof("Error from media server: %v", err)
		return
	case err := <-serviceErr:
		if err != nil {
			exit("Error starting service: %v", err)
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
		srv.Shutdown(ctx)
		return
	}
}

func parseOrchAddrs(addrs string) []*url.URL {
	var res []*url.URL
	if len(addrs) > 0 {
		for _, addr := range strings.Split(addrs, ",") {
			addr = strings.TrimSpace(addr)
			addr = defaultAddr(addr, "127.0.0.1", OrchestratorRpcPort)
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

func parseOrchBlacklist(b *string) []string {
	if b == nil {
		return []string{}
	}
	return strings.Split(strings.ToLower(*b), ",")
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
	addr := "https://" + strings.TrimSpace(string(body)) + ":" + OrchestratorRpcPort
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
		ethUri, _ = url.ParseRequestURI("http://127.0.0.1:" + OrchestratorRpcPort)
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
		return dbh.SetChainID(chainID)
	}

	if expectedChainID.Cmp(chainID) != 0 {
		return fmt.Errorf("expecting chainID of %v, but got %v. Did you change networks without changing network name or datadir?", expectedChainID, chainID)
	}

	return nil
}

type GatewayPrice struct {
	EthAddress    string
	PricePerUnit  *big.Rat
	Currency      string
	PixelsPerUnit *big.Rat
}

func getGatewayPrices(gatewayPrices string) []GatewayPrice {
	if gatewayPrices == "" {
		return nil
	}

	// Format of gatewayPrices json
	// {"gateways":[{"ethaddress":"address1","priceperunit":0.5,"currency":"USD","pixelsperunit":1}, {"ethaddress":"address2","priceperunit":0.3,"currency":"USD","pixelsperunit":3}]}
	var pricesSet struct {
		Gateways []struct {
			EthAddress    string          `json:"ethaddress"`
			PixelsPerUnit json.RawMessage `json:"pixelsperunit"`
			PricePerUnit  json.RawMessage `json:"priceperunit"`
			Currency      string          `json:"currency"`
		} `json:"gateways"`
		// TODO: Keep the old name for backwards compatibility, remove in the future
		Broadcasters []struct {
			EthAddress    string          `json:"ethaddress"`
			PixelsPerUnit json.RawMessage `json:"pixelsperunit"`
			PricePerUnit  json.RawMessage `json:"priceperunit"`
			Currency      string          `json:"currency"`
		} `json:"broadcasters"`
	}
	pricesFileContent, _ := common.ReadFromFile(gatewayPrices)

	err := json.Unmarshal([]byte(pricesFileContent), &pricesSet)
	if err != nil {
		glog.Errorf("gateway prices could not be parsed: %s", err)
		return nil
	}

	// Check if broadcasters field is used and display a warning
	if len(pricesSet.Broadcasters) > 0 {
		glog.Warning("The 'broadcaster' property in the 'pricePerGateway' config is deprecated and will be removed in a future release. Please use 'gateways' instead.")
	}

	// Combine broadcasters and gateways into a single slice
	allGateways := append(pricesSet.Broadcasters, pricesSet.Gateways...)

	prices := make([]GatewayPrice, len(allGateways))
	for i, p := range allGateways {
		pixelsPerUnit, ok := new(big.Rat).SetString(string(p.PixelsPerUnit))
		if !ok {
			glog.Errorf("Pixels per unit could not be parsed for gateway %v. must be a valid number, provided %s", p.EthAddress, p.PixelsPerUnit)
			continue
		}
		pricePerUnit, ok := new(big.Rat).SetString(string(p.PricePerUnit))
		if !ok {
			glog.Errorf("Price per unit could not be parsed for gateway %v. must be a valid number, provided %s", p.EthAddress, p.PricePerUnit)
			continue
		}
		prices[i] = GatewayPrice{
			EthAddress:    p.EthAddress,
			Currency:      p.Currency,
			PricePerUnit:  pricePerUnit,
			PixelsPerUnit: pixelsPerUnit,
		}
	}

	return prices
}

func createSelectionAlgorithm(cfg LivepeerConfig) (common.SelectionAlgorithm, error) {
	sumWeight := *cfg.SelectStakeWeight + *cfg.SelectPriceWeight + *cfg.SelectRandWeight
	if math.Abs(sumWeight-1.0) > 0.0001 {
		return nil, fmt.Errorf(
			"sum of selection algorithm weights must be 1.0, stakeWeight=%v, priceWeight=%v, randWeight=%v",
			*cfg.SelectStakeWeight, *cfg.SelectPriceWeight, *cfg.SelectRandWeight)
	}
	return server.ProbabilitySelectionAlgorithm{
		MinPerfScore:           *cfg.MinPerfScore,
		StakeWeight:            *cfg.SelectStakeWeight,
		PriceWeight:            *cfg.SelectPriceWeight,
		RandWeight:             *cfg.SelectRandWeight,
		PriceExpFactor:         *cfg.SelectPriceExpFactor,
		IgnoreMaxPriceIfNeeded: *cfg.IgnoreMaxPriceIfNeeded,
	}, nil
}

type keystorePath struct {
	path    string
	address ethcommon.Address
}

func parseEthKeystorePath(ethKeystorePath string) (keystorePath, error) {
	var keystore = keystorePath{"", ethcommon.Address{}}
	if ethKeystorePath == "" {
		return keystore, nil
	}

	ethKeystorePath = strings.TrimSuffix(ethKeystorePath, "/")
	fileInfo, err := os.Stat(ethKeystorePath)
	if err != nil {
		return keystore, errors.New("provided -ethKeystorePath was not found")
	}

	if fileInfo.IsDir() {
		keystore.path = ethKeystorePath
	} else {
		if keyText, err := common.ReadFromFile(ethKeystorePath); err == nil {
			if address, err := common.ParseEthAddr(keyText); err == nil {
				keystore.address = ethcommon.BytesToAddress(ethcommon.FromHex(address))
			} else {
				return keystore, errors.New("error parsing address from keyfile")
			}
		} else {
			return keystore, errors.New("error opening keystore")
		}
	}
	return keystore, nil
}

func parsePricePerUnit(pricePerUnitStr string) (*big.Rat, string, error) {
	pricePerUnitRex := regexp.MustCompile(`^(\d+(\.\d+)?)([A-z][A-z0-9]*)?$`)
	match := pricePerUnitRex.FindStringSubmatch(pricePerUnitStr)
	if match == nil {
		return nil, "", fmt.Errorf("price must be in the format of <price><currency>, provided %v", pricePerUnitStr)
	}
	price, currency := match[1], match[3]

	pricePerUnit, ok := new(big.Rat).SetString(price)
	if !ok {
		return nil, "", fmt.Errorf("price must be a valid number, provided %v", match[1])
	}

	return pricePerUnit, currency, nil
}

func refreshOrchPerfScoreLoop(ctx context.Context, region string, orchPerfScoreURL string, score *common.PerfScore) {
	for {
		refreshOrchPerfScore(region, orchPerfScoreURL, score)

		select {
		case <-ctx.Done():
			return
		case <-time.After(RefreshPerfScoreInterval):
		}
	}
}

func refreshOrchPerfScore(region string, scoreURL string, score *common.PerfScore) {
	resp, err := http.Get(scoreURL)
	if err != nil {
		glog.Warning("Cannot fetch Orchestrator Performance Stats from URL: %s", scoreURL)
		return
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		glog.Warning("Cannot fetch Orchestrator Performance Stats from URL: %s", scoreURL)
		return
	}
	updatePerfScore(region, body, score)
}

func updatePerfScore(region string, respBody []byte, score *common.PerfScore) {
	respMap := map[ethcommon.Address]map[string]map[string]float64{}
	if err := json.Unmarshal(respBody, &respMap); err != nil {
		glog.Warning("Cannot unmarshal response from Orchestrator Performance Stats URL, err=%v", err)
		return
	}

	score.Mu.Lock()
	defer score.Mu.Unlock()
	for orchAddr, regions := range respMap {
		if stats, ok := regions[region]; ok {
			if sc, ok := stats["score"]; ok {
				score.Scores[orchAddr] = sc
			}
		}
	}
}

func exit(msg string, args ...any) {
	glog.Errorf(msg, args...)
	os.Exit(2)
}
