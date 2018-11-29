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
	"strings"
	"time"

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
	"github.com/livepeer/go-livepeer/ipfs"
	lpmon "github.com/livepeer/go-livepeer/monitor"
	"github.com/livepeer/go-livepeer/server"
	lumberjack "gopkg.in/natefinch/lumberjack.v2"
)

var (
	ErrKeygen    = errors.New("ErrKeygen")
	EthTxTimeout = 600 * time.Second
)

const RinkebyControllerAddr = "0x37dc71366ec655093b9930bc816e16e6b587f968"
const MainnetControllerAddr = "0xf96d54e490317c557a967abfa5d6e33006be69b3"
const RtmpPort = "1935"
const RpcPort = "8935"
const CliPort = "7935"

func main() {
	flag.Set("logtostderr", "true")

	usr, err := user.Current()
	if err != nil {
		glog.Fatalf("Cannot find current user: %v", err)
	}

	datadir := flag.String("datadir", fmt.Sprintf("%v/.lpData", usr.HomeDir), "data directory")
	rtmpAddr := flag.String("rtmpAddr", "127.0.0.1:"+RtmpPort, "Address to bind for RTMP commands")
	cliAddr := flag.String("cliAddr", "127.0.0.1:"+CliPort, "Address to bind for  CLI commands")
	httpAddr := flag.String("httpAddr", "", "Address to bind for HTTP commands")
	orchAndTrans := flag.Bool("transcoder", false, "Set to true to be a orchestrator+transcoder")
	orchestrator := flag.Bool("orchestrator", false, "Set to true to be a standalone orchestrator")
	maxPricePerSegment := flag.String("maxPricePerSegment", "1", "Max price per segment for a broadcast job")
	transcodingOptions := flag.String("transcodingOptions", "P240p30fps16x9,P360p30fps16x9", "Transcoding options for broadcast job")
	currentManifest := flag.Bool("currentManifest", false, "Expose the currently active ManifestID as \"/stream/current.m3u8\"")
	ethAcctAddr := flag.String("ethAcctAddr", "", "Existing Eth account address")
	ethPassword := flag.String("ethPassword", "", "Password for existing Eth account address")
	ethKeystorePath := flag.String("ethKeystorePath", "", "Path for the Eth Key")
	ethIpcPath := flag.String("ethIpcPath", "", "Path for eth IPC file")
	ethUrl := flag.String("ethUrl", "", "geth/parity rpc or websocket url")
	rinkeby := flag.Bool("rinkeby", false, "Set to true to connect to rinkeby")
	devenv := flag.Bool("devenv", false, "Set to true to enable devenv")
	controllerAddr := flag.String("controllerAddr", "", "Protocol smart contract address")
	gasLimit := flag.Int("gasLimit", 0, "Gas limit for ETH transactions")
	gasPrice := flag.Int("gasPrice", 0, "Gas price for ETH transactions")
	monitor := flag.Bool("monitor", false, "Set to true to send performance metrics")
	monhost := flag.String("monitorhost", "", "host name for the metrics data collector")
	ipfsPath := flag.String("ipfsPath", fmt.Sprintf("%v/.ipfs", usr.HomeDir), "IPFS path")
	noIPFSLogFiles := flag.Bool("noIPFSLogFiles", false, "Set to true if log files should not be generated")
	offchain := flag.Bool("offchain", false, "Set to true to start the node in offchain mode")
	serviceAddr := flag.String("serviceAddr", "", "Orchestrator only. Overrides the on-chain serviceURI that broadcasters can use to contact this node; may be an IP or hostname.")
	initializeRound := flag.Bool("initializeRound", false, "Set to true if running as a transcoder and the node should automatically initialize new rounds")
	s3bucket := flag.String("s3bucket", "", "S3 region/bucket (e.g. eu-central-1/testbucket)")
	s3creds := flag.String("s3creds", "", "S3 credentials (in form ACCESSKEYID/ACCESSKEY)")
	version := flag.Bool("version", false, "Print out the version")
	orchAddr := flag.String("orchAddr", "", "Orchestrator to connect to as a standalone transcoder")
	orchSecret := flag.String("orchSecret", "", "Shared secret with the orchestrator as a standalone transcoder")
	standaloneTranscoder := flag.Bool("standaloneTranscoder", false, "Set to true to be a standalone transcoder")
	var transcoder bool

	flag.Parse()

	if *version {
		fmt.Println("Livepeer Node Version: " + core.LivepeerVersion)
		return
	}

	if *orchAddr != "" {
		if *standaloneTranscoder && *orchSecret == "" {
			glog.Error("Running a standalone transcoder requires both -orchAddr and -orchSecret")
			return
		} else if *standaloneTranscoder {
			transcoder = true
		}
		*orchAddr = defaultAddr(*orchAddr, "127.0.0.1", RpcPort)
	}

	if *rinkeby {
		if !*offchain && !transcoder {
			if *ethUrl == "" {
				*ethUrl = "wss://rinkeby.infura.io/ws"
			}
			if *controllerAddr == "" {
				*controllerAddr = RinkebyControllerAddr
			}
			glog.Infof("***Livepeer is running on the Rinkeby test network: %v***", RinkebyControllerAddr)
			*datadir = *datadir + "/rinkeby"
		} else {
			*datadir = *datadir + "/offchain"
		}

		if *monitor && *monhost == "" {
			*monhost = "http://metrics-rinkeby.livepeer.org/api/events"
		}
	} else if *devenv {
		*datadir = *datadir + "/devenv"
	} else {
		if !*offchain && !transcoder {
			if *ethUrl == "" {
				*ethUrl = "wss://mainnet.infura.io/ws"
			}
			if *controllerAddr == "" {
				*controllerAddr = MainnetControllerAddr
			}
			glog.Infof("***Livepeer is running on the Ethereum main network: %v***", MainnetControllerAddr)
			*datadir = *datadir + "/mainnet"
		} else {
			*datadir = *datadir + "/offchain"
		}
		if *monitor && *monhost == "" {
			*monhost = "http://metrics-mainnet.livepeer.org/api/events"
		}
	}

	if *orchAndTrans {
		transcoder = true
		*orchestrator = true
	}

	//Make sure datadir is present
	if _, err := os.Stat(*datadir); os.IsNotExist(err) {
		glog.Infof("Creating data dir: %v", *datadir)
		if err = os.Mkdir(*datadir, 0755); err != nil {
			glog.Errorf("Error creating datadir: %v", err)
		}
	}

	//Set up DB
	dbh, err := common.InitDB(*datadir + "/lp.sqlite3")
	if err != nil {
		glog.Errorf("Error opening DB", err)
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

	if transcoder {
		n.Transcoder = core.NewLocalTranscoder(*datadir)
	}

	if *orchestrator {
		n.NodeType = core.OrchestratorNode
	} else if transcoder {
		n.NodeType = core.TranscoderNode
		glog.Info("***Livepeer is in transcoder mode ***")
		server.RunTranscoder(n, *orchAddr)
		return
	} else {
		n.NodeType = core.BroadcasterNode
	}

	if n.NodeType == core.BroadcasterNode {
		if *orchAddr == "" {
			glog.Info("No orchestrator specified; transcoding will not happen")
		} else {
			n.OrchestratorSelector = discovery.NewOffchainOrchestrator(n, *orchAddr)
			if n.OrchestratorSelector == nil {
				return
			}
		}
	}

	if *offchain {
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
		gethUrl := ""
		if *ethIpcPath != "" {
			//Connect to specified IPC file
			gethUrl = *ethIpcPath
		} else if *ethUrl != "" {
			//Connect to specified websocket
			gethUrl = *ethUrl
		} else {
			glog.Errorf("Need to specify ethIpcPath or ethWsUrl")
			return
		}

		//Set up eth client
		backend, err := ethclient.Dial(gethUrl)
		if err != nil {
			glog.Errorf("Failed to connect to Ethereum client: %v", err)
			return
		}

		client, err := eth.NewClient(ethcommon.HexToAddress(*ethAcctAddr), keystoreDir, backend, ethcommon.HexToAddress(*controllerAddr), EthTxTimeout)
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

		if *orchestrator {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			if err := setupOrchestrator(ctx, n, em, *ipfsPath, *initializeRound); err != nil {
				glog.Errorf("Error setting up orchestrator: %v", err)
				return
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
		drivers.S3REGION = s3bp[0]
		drivers.S3BUCKET = s3bp[1]
	}

	// XXX get s3 credentials from local env vars?
	if *s3bucket != "" && *s3creds != "" {
		br := strings.Split(*s3bucket, "/")
		cr := strings.Split(*s3creds, "/")
		drivers.NodeStorage = drivers.NewS3Driver(br[0], br[1], cr[0], cr[1])
	}

	if n.NodeType == core.BroadcasterNode {
		// default lpms listener for broadcaster; same as default rpc port
		// TODO provide an option to disable this?
		*rtmpAddr = defaultAddr(*rtmpAddr, "127.0.0.1", RtmpPort)
		*httpAddr = defaultAddr(*httpAddr, "127.0.0.1", RpcPort)
	} else if n.NodeType == core.OrchestratorNode {
		n.ServiceURI, err = getServiceURI(n, *serviceAddr)
		if err != nil {
			glog.Error("Error getting service URI: ", err)
			return
		}
		// if http addr is not provided, listen to all ifaces
		// take the port to listen to from the service URI
		*httpAddr = defaultAddr(*httpAddr, "", n.ServiceURI.Port())
	}
	*cliAddr = defaultAddr(*cliAddr, "127.0.0.1", CliPort)

	if drivers.NodeStorage == nil {
		// base URI will be empty for broadcasters; that's OK
		base := ""
		if n.ServiceURI != nil {
			base = n.ServiceURI.String()
		}
		drivers.NodeStorage = drivers.NewMemoryDriver(base)
	}

	//Create Livepeer Node
	if *monitor {
		glog.Infof("Monitoring endpoint: %s", *monhost)
		lpmon.Enabled = true
		lpmon.SetURL(*monhost)
	}

	// Set up logging
	if !*noIPFSLogFiles {
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

	bigMaxPricePerSegment, err := common.ParseBigInt(*maxPricePerSegment)
	if err != nil {
		glog.Errorf("Error setting max price per segment: %v", err)
		return
	}

	if *currentManifest {
		glog.Info("Current ManifestID will be available over ", *httpAddr)
		s.ExposeCurrentManifest = *currentManifest
	}

	go func() {
		s.StartWebserver(*cliAddr)
		close(wc)
	}()
	go func() {
		ec <- s.StartMediaServer(msCtx, bigMaxPricePerSegment, *transcodingOptions)
	}()

	go func() {
		if core.OrchestratorNode != n.NodeType {
			return
		}
		orch := core.NewOrchestrator(s.LivepeerNode)

		go func() {
			server.StartTranscodeServer(orch, *httpAddr, s.HttpMux, n.WorkDir)
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
	ipfsApi, err := ipfs.StartIpfs(ctx, ipfsPath)
	if err != nil {
		return err
	}
	drivers.SetIpfsAPI(ipfsApi)

	n.Ipfs = ipfsApi
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

	// Create job service to listen for new jobs and transcode if assigned to the job
	js := eventservices.NewJobService(n)
	n.EthServices["JobService"] = js

	// Restart jobs as necessary
	err = js.RestartTranscoder()
	if err != nil {
		glog.Errorf("Unable to restart orchestrator: %v", err)
		// non-fatal, so continue
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
