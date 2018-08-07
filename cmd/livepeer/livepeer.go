/*
Livepeer is a peer-to-peer global video live streaming network.  The Golp project is a go implementation of the Livepeer protocol.  For more information, visit the project wiki.
*/
package main

import (
	"context"
	"encoding/json"
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
	peer "gx/ipfs/QmZoWKhxUmZ2seW4BzX6fJkNR8hh9PsGModr7q171yq2SS/go-libp2p-peer"
	crypto "gx/ipfs/QmaPbCnUMBohSGo3KnxEa2bHqyJVVeEEcwtqJAYxerieBo/go-libp2p-crypto"

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

func main() {
	flag.Set("logtostderr", "true")

	usr, err := user.Current()
	if err != nil {
		glog.Fatalf("Cannot find current user: %v", err)
	}

	httpPort := flag.String("http", "8935", "http port")
	rtmpPort := flag.String("rtmp", "1935", "rtmp port")
	datadir := flag.String("datadir", fmt.Sprintf("%v/.lpData", usr.HomeDir), "data directory")
	rtmpIP := flag.String("rtmpIP", "127.0.0.1", "IP to bind for HTTP RPC commands")
	httpIP := flag.String("httpIP", "127.0.0.1", "IP to bind for HTTP RPC commands")
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
	publicAddr := flag.String("publicAddr", "", "Public address that broadcasters can use to contact this node; may be an IP or hostname. If used, should match the on-chain ServiceURI set via livepeer_cli")
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

	//Take care of priv/pub keypair
	_, pub, err := getLPKeys(*datadir)
	if err != nil {
		glog.Errorf("Error getting keys: %v", err)
		return
	}

	nodeId, err := peer.IDFromPublicKey(pub) // TODO simplify this
	if err != nil {
		glog.Error("Error retrieving node ID ", err)
	}
	n, err := core.NewLivepeerNode(nil, nil, core.NodeID(nodeId), *datadir, dbh)
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

			if err := setupTranscoder(ctx, n, em, *ipfsPath, *initializeRound, *publicAddr); err != nil {
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

	//Create Livepeer Node
	if *monitor {
		glog.Info("Monitor is set to 'true' by default.  If you want to disable it, use -monitor=false when starting Livepeer.")
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
	s := server.NewLivepeerServer(*rtmpPort, *rtmpIP, *httpPort, *httpIP, n)
	ec := make(chan error)
	msCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bigMaxPricePerSegment, err := common.ParseBigInt(*maxPricePerSegment)
	if err != nil {
		glog.Errorf("Error setting max price per segment: %v", err)
		return
	}

	go func() {
		s.StartWebserver()
		ec <- s.StartMediaServer(msCtx, bigMaxPricePerSegment, *transcodingOptions)
	}()

	switch n.NodeType {
	case core.Transcoder:
		glog.Infof("***Livepeer Running in Transcoder Mode***")
	case core.Broadcaster:
		glog.Infof("***Livepeer Running in Broadcaster Mode***")
		glog.Infof("Video Ingest Endpoint - rtmp://%v:%v", *rtmpIP, *rtmpPort)
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
	case sig := <-c:
		glog.Infof("Exiting Livepeer: %v", sig)
		time.Sleep(time.Millisecond * 500) //Give time for other processes to shut down completely
		return
	}
}

type LPKeyFile struct {
	Pub  string
	Priv string
}

func getLPKeys(datadir string) (crypto.PrivKey, crypto.PubKey, error) {
	gen := false
	var priv crypto.PrivKey
	var pub crypto.PubKey
	var privb []byte
	var pubb []byte
	var err error

	if datadir != "" {
		f, e := ioutil.ReadFile(path.Join(datadir, "keys.json"))
		if e != nil {
			gen = true
		}

		var keyf LPKeyFile
		if gen == false {
			if err := json.Unmarshal(f, &keyf); err != nil {
				gen = true
			}
		}

		if gen == false {
			privb, err = crypto.ConfigDecodeKey(keyf.Priv)
			if err != nil {
				gen = true
			}
		}

		if gen == false {
			pubb, err = crypto.ConfigDecodeKey(keyf.Pub)
			if err != nil {
				gen = true
			}
		}

		if gen == false {
			priv, err = crypto.UnmarshalPrivateKey(privb)
			if err != nil {
				gen = true
			}

		}

		if gen == false {
			pub, err = crypto.UnmarshalPublicKey(pubb)
			if err != nil {
				gen = true
			}
		}
	}

	if gen == true || pub == nil || priv == nil {
		glog.Errorf("Cannot file keys in data dir %v, creating new keys", datadir)
		priv, pub, err := crypto.GenerateKeyPair(crypto.RSA, 2048)
		if err != nil {
			glog.Errorf("Error generating keypair: %v", err)
			return nil, nil, ErrKeygen
		}

		privb, _ := priv.Bytes()
		pubb, _ := pub.Bytes()

		//Write keys to datadir
		if datadir != "" {
			kf := LPKeyFile{Priv: crypto.ConfigEncodeKey(privb), Pub: crypto.ConfigEncodeKey(pubb)}
			kfb, err := json.Marshal(kf)
			if err != nil {
				glog.Errorf("Error writing keyfile to datadir: %v", err)
			} else {
				if err := ioutil.WriteFile(path.Join(datadir, "keys.json"), kfb, 0644); err != nil {
					glog.Errorf("Error writing keyfile to datadir: %v", err)
				}
			}
		}

		return priv, pub, nil
	}

	return priv, pub, nil
}

func setupTranscoder(ctx context.Context, n *core.LivepeerNode, em eth.EventMonitor, ipfsPath string, initializeRound bool, publicAddr string) error {
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

	if publicAddr == "" {
		// TODO probably should put this (along w wizard GETs) into common code
		resp, err := http.Get("https://api.ipify.org?format=text")
		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			glog.Error("Could not look up public IP address")
			return err
		}
		publicAddr = strings.TrimSpace(string(body))
	}
	uriStr, err := n.Eth.GetServiceURI(n.Eth.Account().Address)
	if err != nil {
		glog.Error("Could not get service URI")
		return err
	}
	uri, err := url.ParseRequestURI(uriStr)
	if err != nil {
		glog.Error("Could not parse service URI")
		uri, _ = url.ParseRequestURI("http://127.0.0.1")
	}
	if uri.Hostname() != publicAddr {
		glog.Errorf("Service address %v did not match discovered address %v; set the correct address in livepeer_cli or use -publicAddr", uri.Hostname(), publicAddr)
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
	js := eventservices.NewJobService(em, n)
	n.EthServices["JobService"] = js

	// Restart jobs as necessary
	err = js.RestartTranscoder()
	if err != nil {
		glog.Errorf("Unable to restart transcoder: %v", err)
		// non-fatal, so continue
	}

	return nil
}
