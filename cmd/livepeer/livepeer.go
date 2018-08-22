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
const RpcPort = "8935"

func main() {
	flag.Set("logtostderr", "true")

	usr, err := user.Current()
	if err != nil {
		glog.Fatalf("Cannot find current user: %v", err)
	}

	datadir := flag.String("datadir", fmt.Sprintf("%v/.lpData", usr.HomeDir), "data directory")
	rtmpAddr := flag.String("rtmpAddr", "127.0.0.1:1935", "IP to bind for RTMP commands")
	cliAddr := flag.String("cliAddr", "127.0.0.1:7935", "Address to bind for  CLI commands")
	httpAddr := flag.String("httpAddr", "", "Address to bind for HTTP commands")
	transcoder := flag.Bool("transcoder", false, "Set to true to be a transcoder")
	maxPricePerSegment := flag.String("maxPricePerSegment", "1", "Max price per segment for a broadcast job")
	transcodingOptions := flag.String("transcodingOptions", "P240p30fps16x9,P360p30fps16x9", "Transcoding options for broadcast job")
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
	serviceAddr := flag.String("serviceAddr", "", "Transcoder only. Public address:port that broadcasters can use to contact this node; may be an IP or hostname. If used, should match the on-chain ServiceURI set via livepeer_cli")
	initializeRound := flag.Bool("initializeRound", false, "Set to true if running as a transcoder and the node should automatically initialize new rounds")
	version := flag.Bool("version", false, "Print out the version")

	flag.Parse()

	if *version {
		fmt.Println("Livepeer Node Version: " + core.LivepeerVersion)
		return
	}

	if *rinkeby {
		if !*offchain {
			if *ethUrl == "" {
				*ethUrl = "wss://rinkeby.infura.io/ws"
			}
			if *controllerAddr == "" {
				*controllerAddr = RinkebyControllerAddr
			}
			glog.Infof("***Livepeer is running on the Rinkeby test network: %v***", RinkebyControllerAddr)
		}
		if *monitor && *monhost == "" {
			*monhost = "http://metrics-rinkeby.livepeer.org/api/events"
		}
	} else if *devenv {
	} else {
		if !*offchain {
			if *ethUrl == "" {
				*ethUrl = "wss://mainnet.infura.io/ws"
			}
			if *controllerAddr == "" {
				*controllerAddr = MainnetControllerAddr
			}
			glog.Infof("***Livepeer is running on the Ethereum main network: %v***", MainnetControllerAddr)
		}
		if *monitor && *monhost == "" {
			*monhost = "http://metrics-mainnet.livepeer.org/api/events"
		}
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
	if *transcoder {
		n.NodeType = core.Transcoder
	} else {
		n.NodeType = core.Broadcaster
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

		if *transcoder {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			if err := setupTranscoder(ctx, n, em, *ipfsPath, *initializeRound, *serviceAddr); err != nil {
				glog.Errorf("Error setting up transcoder: %v", err)
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

	if n.NodeType == core.Broadcaster {
		// default lpms listener for broadcaster; same as default rpc port
		// TODO provide an option to disable this?
		if "" == *httpAddr {
			*httpAddr = "127.0.0.1:" + RpcPort
		}
	} else if n.NodeType == core.Transcoder {
		// if http addr is not provided, listen to all ifaces
		// take the port to listen to from the service URI
		if "" == *httpAddr {
			*httpAddr = ":" + n.ServiceURI.Port()
		}
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

	go func() {
		s.StartWebserver(*cliAddr)
		close(wc)
	}()
	go func() {
		ec <- s.StartMediaServer(msCtx, bigMaxPricePerSegment, *transcodingOptions)
	}()
	go func() {
		if core.Transcoder != n.NodeType {
			return
		}
		orch := core.NewOrchestrator(s.LivepeerNode)
		server.StartTranscodeServer(orch, *httpAddr, s.HttpMux, n.WorkDir)
		tc <- struct{}{}
	}()

	switch n.NodeType {
	case core.Transcoder:
		glog.Infof("***Livepeer Running in Transcoder Mode***")
	case core.Broadcaster:
		glog.Infof("***Livepeer Running in Broadcaster Mode***")
		glog.Infof("Video Ingest Endpoint - rtmp:/%v", *rtmpAddr)
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
		glog.Infof("Transcoder server shut down")
	case <-wc:
		glog.Infof("CLI webserver shut down")
		return
	case sig := <-c:
		glog.Infof("Exiting Livepeer: %v", sig)
		time.Sleep(time.Millisecond * 500) //Give time for other processes to shut down completely
		return
	}
}

func setupTranscoder(ctx context.Context, n *core.LivepeerNode, em eth.EventMonitor, ipfsPath string, initializeRound bool, serviceUri string) error {
	//Check if transcoder is active
	active, err := n.Eth.IsActiveTranscoder()
	if err != nil {
		return err
	}

	if !active {
		glog.Infof("Transcoder %v is inactive", n.Eth.Account().Address.Hex())
	} else {
		glog.Infof("Transcoder %v is active", n.Eth.Account().Address.Hex())
	}

	if serviceUri == "" {
		// TODO probably should put this (along w wizard GETs) into common code
		resp, err := http.Get("https://api.ipify.org?format=text")
		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			glog.Error("Could not look up public IP address")
			return err
		}
		serviceUri = "https://" + strings.TrimSpace(string(body)) + ":" + RpcPort
	} else {
		serviceUri = "https://" + serviceUri
	}
	suri, err := url.ParseRequestURI(serviceUri)
	if err != nil {
		glog.Error("Could not parse local service URI ", err)
		return err // this is fatal; should at least infer a valid uri
	}
	uriStr, err := n.Eth.GetServiceURI(n.Eth.Account().Address)
	if err != nil {
		glog.Error("Could not get service URI; transcoder may be unreachable")
		return err
	}
	uri, err := url.ParseRequestURI(uriStr)
	if err != nil {
		glog.Error("Could not parse service URI; transcoder may be unreachable")
		uri, _ = url.ParseRequestURI("http://127.0.0.1:" + RpcPort)
	}
	if uri.Hostname() != suri.Hostname() || uri.Port() != suri.Port() {
		glog.Errorf("Service address %v did not match discovered address %v; set the correct address in livepeer_cli or use -serviceAddr", uri, suri)
		// TODO remove '&& false' after all transcoders have set a service URI
		if active && false {
			return fmt.Errorf("Mismatched service address")
		}
	}

	// Set up IPFS
	ipfsApi, err := ipfs.StartIpfs(ctx, ipfsPath)
	if err != nil {
		return err
	}

	n.Ipfs = ipfsApi
	n.EthEventMonitor = em
	n.ServiceURI = uri

	if initializeRound {
		glog.Infof("Transcoder %v will automatically initialize new rounds", n.Eth.Account().Address.Hex())

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
		glog.Errorf("Unable to restart transcoder: %v", err)
		// non-fatal, so continue
	}

	return nil
}
