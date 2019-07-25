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
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"os/user"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/livepeer/go-livepeer/pm"

	ipfslogging "gx/ipfs/QmSpJByNKFX1sCsHBEp3R73FL4NF6FnQTEGyNAXHm2GS52/go-log"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/discovery"
	"github.com/livepeer/go-livepeer/drivers"
	"github.com/livepeer/go-livepeer/eth"
	"github.com/livepeer/go-livepeer/eth/eventservices"

	//"github.com/livepeer/go-livepeer/ipfs" until we re-enable IPFS
	lpmon "github.com/livepeer/go-livepeer/monitor"
	"github.com/livepeer/go-livepeer/server"
	lumberjack "gopkg.in/natefinch/lumberjack.v2"
)

var (
	ErrKeygen    = errors.New("ErrKeygen")
	EthTxTimeout = 600 * time.Second

	// The gas required to redeem a PM ticket
	redeemGas = 100000
	// The multiplier on the transaction cost to use for PM ticket faceValue
	txCostMultiplier = 100
	// The interval at which to poll for gas price updates
	gpmPollingInterval = 1 * time.Minute
	// The interval at which to clean up cached max float values for PM senders and balances per stream
	cleanupInterval = 1 * time.Minute
	// The time to live for cached max float values for PM senders (else they will be cleaned up)
	smTTL = 3600 // 1 minute
	// maxErrCount is the maximum number of acceptable errors tolerated by a payment recipient for a payment sender
	maxErrCount = 3
)

const RtmpPort = "1935"
const RpcPort = "8935"
const CliPort = "7935"

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

	// Transcoding:
	orchestrator := flag.Bool("orchestrator", false, "Set to true to be an orchestrator")
	transcoder := flag.Bool("transcoder", false, "Set to true to be a transcoder")
	broadcaster := flag.Bool("broadcaster", false, "Set to true to be a broadcaster")
	orchSecret := flag.String("orchSecret", "", "Shared secret with the orchestrator as a standalone transcoder")
	transcodingOptions := flag.String("transcodingOptions", "P240p30fps16x9,P360p30fps16x9", "Transcoding options for broadcast job")
	maxSessions := flag.Int("maxSessions", 10, "Maximum number of concurrent transcoding sessions for Orchestrator, maximum number or RTMP streams for Broadcaster, or maximum capacity for transcoder")
	currentManifest := flag.Bool("currentManifest", false, "Expose the currently active ManifestID as \"/stream/current.m3u8\"")
	nvidia := flag.String("nvidia", "", "Comma-separated list of Nvidia GPU device IDs to use for transcoding")

	// Onchain:
	ethAcctAddr := flag.String("ethAcctAddr", "", "Existing Eth account address")
	ethPassword := flag.String("ethPassword", "", "Password for existing Eth account address")
	ethKeystorePath := flag.String("ethKeystorePath", "", "Path for the Eth Key")
	ethUrl := flag.String("ethUrl", "", "geth/parity rpc or websocket url")
	ethController := flag.String("ethController", "", "Protocol smart contract address")
	gasLimit := flag.Int("gasLimit", 0, "Gas limit for ETH transactions")
	gasPrice := flag.Int("gasPrice", 0, "Gas price for ETH transactions")
	initializeRound := flag.Bool("initializeRound", false, "Set to true if running as a transcoder and the node should automatically initialize new rounds")
	ticketEV := flag.String("ticketEV", "1000000000", "The expected value for PM tickets")
	// Broadcaster max acceptable ticket EV
	maxTicketEV := flag.String("maxTicketEV", "10000000000", "The maximum acceptable expected value for PM tickets")
	// Broadcaster deposit multiplier to determine max acceptable ticket faceValue
	depositMultiplier := flag.Int("depositMultiplier", 1000, "The deposit multiplier used to determine max acceptable faceValue for PM tickets")

	// Orchestrator base pricing info
	pricePerUnit := flag.Int("pricePerUnit", 0, "The price per 'pixelsPerUnit' amount pixels")
	// Broadcaster max acceptable price
	maxPricePerUnit := flag.Int("maxPricePerUnit", 0, "The maximum transcoding price (in wei) per 'pixelsPerUnit' a broadcaster is willing to accept. If not set explicitly, broadcaster is willing to accept ANY price")
	// Unit of pixels for both O's basePriceInfo and B's MaxBroadcastPrice
	pixelsPerUnit := flag.Int("pixelsPerUnit", 1, "Amount of pixels per unit. Set to '> 1' to have smaller price granularity than 1 wei / pixel")

	// Metrics & logging:
	monitor := flag.Bool("monitor", false, "Set to true to send performance metrics")
	version := flag.Bool("version", false, "Print out the version")
	verbosity := flag.String("v", "", "Log verbosity.  {4|5|6}")
	logIPFS := flag.Bool("logIPFS", false, "Set to true if log files should not be generated") // unused until we re-enable IPFS

	// Storage:
	datadir := flag.String("datadir", "", "data directory")
	ipfsPath := flag.String("ipfsPath", fmt.Sprintf("%v/.ipfs", usr.HomeDir), "IPFS path") // unused until we re-enable IPFS
	s3bucket := flag.String("s3bucket", "", "S3 region/bucket (e.g. eu-central-1/testbucket)")
	s3creds := flag.String("s3creds", "", "S3 credentials (in form ACCESSKEYID/ACCESSKEY)")
	gsBucket := flag.String("gsbucket", "", "Google storage bucket")
	gsKey := flag.String("gskey", "", "Google Storage private key file name (in json format)")

	// API
	authWebhookURL := flag.String("authWebhookUrl", "", "RTMP authentication webhook URL")

	flag.Parse()
	vFlag.Value.Set(*verbosity)

	if *version {
		fmt.Println("Livepeer Node Version: " + core.LivepeerVersion)
		fmt.Printf("Golang runtime version: %s %s\n", runtime.Compiler, runtime.Version())
		fmt.Printf("Architecture: %s\n", runtime.GOARCH)
		fmt.Printf("Operating system: %s\n", runtime.GOOS)
		return
	}

	if *maxSessions <= 0 {
		glog.Error("-maxSessions must be greater than zero")
		return
	}

	type NetworkConfig struct {
		ethUrl        string
		ethController string
	}

	configOptions := map[string]*NetworkConfig{
		"rinkeby": {
			ethUrl:        "wss://rinkeby.infura.io/ws/v3/09642b98164d43eb890939eb9a7ec500",
			ethController: "0x37dc71366ec655093b9930bc816e16e6b587f968",
		},
		"mainnet": {
			ethUrl:        "wss://mainnet.infura.io/ws/v3/be11162798084102a3519541eded12f6",
			ethController: "0xf96d54e490317c557a967abfa5d6e33006be69b3",
		},
	}

	// If multiple orchAddresses specified, ensure other necessary flags present and clean up list
	var orchAddresses []string
	if len(*orchAddr) > 0 {
		orchAddresses = strings.Split(*orchAddr, ",")
		for i := range orchAddresses {
			orchAddresses[i] = strings.TrimSpace(orchAddresses[i])
			orchAddresses[i] = defaultAddr(orchAddresses[i], "127.0.0.1", RpcPort)
		}
	}

	// Setting config options based on specified network
	if netw, ok := configOptions[*network]; ok {
		if *ethUrl == "" {
			*ethUrl = netw.ethUrl
		}
		if *ethController == "" {
			*ethController = netw.ethController
		}
		glog.Infof("***Livepeer is running on the %v*** network: %v***", *network, *ethController)
	} else {
		glog.Infof("***Livepeer is running on the %v*** network", *network)
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
	dbh, err := common.InitDB(*datadir + "/lp.sqlite3")
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
		n.OrchSecret = *orchSecret
	}

	if *transcoder {
		if *nvidia != "" {
			n.Transcoder = core.NewNvidiaTranscoder(*nvidia, *datadir)
		} else {
			n.Transcoder = core.NewLocalTranscoder(*datadir)
		}
	}

	if *orchestrator {
		n.NodeType = core.OrchestratorNode
		if !*transcoder {
			n.TranscoderManager = core.NewRemoteTranscoderManager()
			n.Transcoder = n.TranscoderManager
		}
	} else if *transcoder {
		n.NodeType = core.TranscoderNode
	} else if *broadcaster {
		n.NodeType = core.BroadcasterNode
	} else {
		glog.Fatalf("Node type not set; must be one of -broadcaster, -transcoder or -orchestrator")
	}

	if *monitor {
		lpmon.Enabled = true
		nodeID := *ethAcctAddr
		if nodeID == "" {
			hn, _ := os.Hostname()
			nodeID = hn
		}
		nodeType := "bctr"
		switch n.NodeType {
		case core.OrchestratorNode:
			nodeType = "orch"
		case core.TranscoderNode:
			nodeType = "trcr"
		}
		lpmon.InitCensus(nodeType, nodeID, core.LivepeerVersion)
	}

	if n.NodeType == core.TranscoderNode {
		glog.Info("***Livepeer is in transcoder mode ***")
		if n.OrchSecret == "" {
			glog.Fatal("Missing -orchSecret")
		}
		if len(orchAddresses) > 0 {
			server.RunTranscoder(n, orchAddresses[0], *maxSessions)
		} else {
			glog.Fatal("Missing -orchAddr")
		}
		return
	}

	if *network == "offchain" {
		glog.Infof("***Livepeer is in off-chain mode***")
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
			glog.Error("Need to specify ethUrl")
			return
		}

		//Set up eth client
		backend, err := ethclient.Dial(*ethUrl)
		if err != nil {
			glog.Errorf("Failed to connect to Ethereum client: %v", err)
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

		defer n.StopEthServices()

		addrMap := n.Eth.ContractAddresses()
		em := eth.NewEventMonitor(backend, addrMap)

		// Setup block service to receive headers from the head of the chain
		n.EthServices["BlockService"] = eventservices.NewBlockService(em, dbh)
		// Setup unbonding service to manage unbonding locks
		n.EthServices["UnbondingService"] = eventservices.NewUnbondingService(n.Eth, dbh)

		n.Balances = core.NewBalances(cleanupInterval)

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

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			if err := setupOrchestrator(ctx, n, em, *ipfsPath, *initializeRound); err != nil {
				glog.Errorf("Error setting up orchestrator: %v", err)
				return
			}

			ev, _ := new(big.Int).SetString(*ticketEV, 10)
			if ev == nil {
				glog.Errorf("-ticketEV must be a valid integer, but %v provided. Restart the node with a different valid value for -ticketEV", *ticketEV)
				return
			}

			if ev.Cmp(big.NewInt(0)) < 0 {
				glog.Errorf("-ticketEV must be greater than 0, but %v provided. Restart the node with a different valid value for -ticketEV", *ticketEV)
				return
			}

			sigVerifier := &pm.DefaultSigVerifier{}
			// TODO: Initialize Validator with an implementation
			// of RoundsManager that reads from a cache
			validator := pm.NewValidator(sigVerifier, n.Eth)
			gpm := eth.NewGasPriceMonitor(backend, gpmPollingInterval)
			// Start gas price monitor
			gasPriceUpdate, err := gpm.Start(context.Background())
			if err != nil {
				glog.Errorf("error starting gas price monitor: %v", err)
				return
			}
			defer gpm.Stop()

			em := core.NewErrorMonitor(maxErrCount, gasPriceUpdate)
			n.ErrorMonitor = em
			go em.StartGasPriceUpdateLoop()

			sm := pm.NewSenderMonitor(n.Eth.Account().Address, n.Eth, cleanupInterval, smTTL, n.ErrorMonitor)
			// Start sender monitor
			sm.Start()
			defer sm.Stop()

			cfg := pm.TicketParamsConfig{
				EV:               ev,
				RedeemGas:        redeemGas,
				TxCostMultiplier: txCostMultiplier,
			}
			n.Recipient, err = pm.NewRecipient(
				n.Eth.Account().Address,
				n.Eth,
				validator,
				n.Database,
				gpm,
				sm,
				n.ErrorMonitor,
				cfg,
			)
			if err != nil {
				glog.Errorf("Error setting up PM recipient: %v", err)
				return
			}

			n.Recipient.Start()
			defer n.Recipient.Stop()

			// Run cleanup routine for stale balances
			go n.Balances.StartCleanup()
			// Stop the cleanup routine on program exit
			defer n.Balances.StopCleanup()
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

			// TODO: Initialize Sender with an implementation
			// of RoundsManager that reads from a cache
			// TODO: Initialize Sender with an implementation
			// of SenderManager that reads from a cache
			n.Sender = pm.NewSender(n.Eth, n.Eth, n.Eth, ev, *depositMultiplier)

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

		// Start services
		err = n.StartEthServices()
		if err != nil {
			glog.Errorf("Failed to start ETH services: %v", err)
			return
		}
	}

	if *s3bucket != "" && *s3creds == "" || *s3bucket == "" && *s3creds != "" {
		glog.Error("Should specify both s3bucket and s3creds")
		return
	}
	if *s3bucket != "" {
		s3bp := strings.Split(*s3bucket, "/")
		drivers.S3BUCKET = s3bp[1]
	}
	if *gsBucket != "" && *gsKey == "" || *gsBucket == "" && *gsKey != "" {
		glog.Error("Should specify both gsbucket and gskey")
		return
	}

	// XXX get s3 credentials from local env vars?
	if *s3bucket != "" && *s3creds != "" {
		br := strings.Split(*s3bucket, "/")
		cr := strings.Split(*s3creds, "/")
		drivers.NodeStorage = drivers.NewS3Driver(br[0], br[1], cr[0], cr[1])
	}

	if *gsBucket != "" && *gsKey != "" {
		drivers.GSBUCKET = *gsBucket
		drivers.NodeStorage, err = drivers.NewGoogleDriver(*gsBucket, *gsKey)
		if err != nil {
			glog.Error("Error creating Google Storage driver:", err)
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

		// Set up orchestrator discovery
		if len(orchAddresses) > 0 {
			n.OrchestratorPool = discovery.NewOrchestratorPool(n, orchAddresses)
		} else if *network != "offchain" {
			n.OrchestratorPool = discovery.NewDBOrchestratorPoolCache(n)
		}
		if n.OrchestratorPool == nil {
			// Not a fatal error; may continue operating in segment-only mode
			glog.Error("No orchestrator specified; transcoding will not happen")
		}
		var err error
		if server.AuthWebhookURL, err = getAuthWebhookURL(*authWebhookURL); err != nil {
			glog.Fatal("Error setting auth webhook URL ", err)
		}
	} else if n.NodeType == core.OrchestratorNode {
		suri, err := getServiceURI(n, *serviceAddr)
		if err != nil {
			glog.Fatal("Error getting service URI: ", err)
		}
		n.SetServiceURI(suri)
		// if http addr is not provided, listen to all ifaces
		// take the port to listen to from the service URI
		*httpAddr = defaultAddr(*httpAddr, "", n.GetServiceURI().Port())

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

	// Set up logging
	if *logIPFS {
		ipfslogging.LdJSONFormatter()
		logger := &lumberjack.Logger{
			Filename:   path.Join(*ipfsPath, "logs", "ipfs.log"),
			MaxSize:    10, // Megabytes
			MaxBackups: 3,
			MaxAge:     30, // Days
		}
		ipfslogging.LevelError()
		ipfslogging.Output(logger)()
	}

	//Set up the media server
	s := server.NewLivepeerServer(*rtmpAddr, *httpAddr, n)
	ec := make(chan error)
	tc := make(chan struct{})
	wc := make(chan struct{})
	msCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err != nil {
		glog.Errorf("Error setting max price per segment: %v", err)
		return
	}

	if *currentManifest {
		glog.Info("Current ManifestID will be available over ", *httpAddr)
		s.ExposeCurrentManifest = *currentManifest
	}

	go func() {
		s.StartCliWebserver(*cliAddr)
		close(wc)
	}()
	go func() {
		ec <- s.StartMediaServer(msCtx, *transcodingOptions)
	}()

	go func() {
		if core.OrchestratorNode != n.NodeType {
			return
		}

		orch := core.NewOrchestrator(s.LivepeerNode)

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
	}

	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt)
	select {
	case err := <-ec:
		glog.Infof("Error from media server: %v", err)
		return
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

func getAuthWebhookURL(u string) (string, error) {
	if u == "" {
		return "", nil
	}
	p, err := url.ParseRequestURI(u)
	if err != nil {
		return "", err
	}
	if p.Scheme != "http" && p.Scheme != "https" {
		return "", errors.New("Webhook URL should be HTTP or HTTP")
	}
	glog.Infof("Using webhook url %s", u)
	return u, nil
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

func setupOrchestrator(ctx context.Context, n *core.LivepeerNode, em eth.EventMonitor, ipfsPath string, initializeRound bool) error {
	//Check if orchestrator is active
	active, err := n.Eth.IsActiveTranscoder()
	if err != nil {
		return err
	}

	if !active {
		glog.Infof("Orchestrator %v is inactive", n.Eth.Account().Address.Hex())
	} else {
		glog.Infof("Orchestrator %v is active", n.Eth.Account().Address.Hex())
	}

	// Set up IPFS
	/*ipfsApi, err := ipfs.StartIpfs(ctx, ipfsPath)
	if err != nil {
		return err
	}
	drivers.SetIpfsAPI(ipfsApi)

	n.Ipfs = ipfsApi*/
	n.EthEventMonitor = em

	if initializeRound {
		glog.Infof("Orchestrator %v will automatically initialize new rounds", n.Eth.Account().Address.Hex())

		// Create rounds service to initialize round if it has not already been initialized
		rds := eventservices.NewRoundsService(em, n.Eth)
		n.EthServices["RoundsService"] = rds
	}

	// Create reward service to claim/distribute inflationary rewards every round
	rs := eventservices.NewRewardService(n.Eth)
	n.EthServices["RewardService"] = rs

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
