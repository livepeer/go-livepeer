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
	kb "gx/ipfs/QmTH6VLu3WXfbH3nuLdmscgPWuiPZv3GMJ2YCdzBS5z91T/go-libp2p-kbucket"
	ma "gx/ipfs/QmWWQ2Txc2c6tqjsBpzg5Ar652cHPGNsQQp2SejkNmkUMb/go-multiaddr"
	peer "gx/ipfs/QmZoWKhxUmZ2seW4BzX6fJkNR8hh9PsGModr7q171yq2SS/go-libp2p-peer"
	crypto "gx/ipfs/QmaPbCnUMBohSGo3KnxEa2bHqyJVVeEEcwtqJAYxerieBo/go-libp2p-crypto"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/golang/glog"
	bnet "github.com/livepeer/go-livepeer-basicnet"
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
	ErrKeygen       = errors.New("ErrKeygen")
	EthRpcTimeout   = 10 * time.Second
	EthEventTimeout = 120 * time.Second
	EthTxTimeout    = 600 * time.Second
	ErrIpfs         = errors.New("ErrIpfs")
)

const RinkebyBootNodeIDs = "122019c1a1f0d9fa2296dccb972e7478c5163415cd55722dcf0123553f397c45df7e,1220abded0103eb4e8e46616881e74e88617409ac4012c6958f4086c649f44eacf89,1220afca402fae8dbb0f980ea2d7e873bc07da36d5463516a862c6199bb6383e9e1e"
const RinkebyBootNodeAddrs = "/ip4/18.217.129.34/tcp/15000,/ip4/52.15.174.204/tcp/15000,/ip4/13.59.47.56/tcp/15000"
const RinkebyControllerAddr = "0x37dc71366ec655093b9930bc816e16e6b587f968"
const MainnetBootNodeIDs = "122018ef62657724948b236e9523d928d892965e0fdd679beb643f2353556d0aef78,122051f483e4ae0477773540a4ac0e845eac5a51e0883338970270b8af2c02084a66,12208d9eaed4e66382e86fea8956bd7cf0fbd6fbd0146375aa2fa4aaa43467a5dfac,1220bf3b27668b43ce76ca3ca1878ac3aab5009ce64e2305703c9120c90d0f381e1f,1220c079d0cd80d3b82b6ee8b1d91ec09c3172fee8468765ff5dbd43a6c1d5be4443"
const MainnetBootNodeAddrs = "/ip4/18.188.233.37/tcp/15000,/ip4/18.219.112.164/tcp/15000,/ip4/18.222.5.113/tcp/15000,/ip4/18.221.147.73/tcp/15000,/ip4/18.216.88.163/tcp/15000"
const MainnetControllerAddr = "0xf96d54e490317c557a967abfa5d6e33006be69b3"

func main() {
	flag.Set("logtostderr", "true")

	usr, err := user.Current()
	if err != nil {
		glog.Fatalf("Cannot find current user: %v", err)
	}

	port := flag.Int("p", 15000, "port")
	httpPort := flag.String("http", "8935", "http port")
	rtmpPort := flag.String("rtmp", "1935", "rtmp port")
	datadir := flag.String("datadir", fmt.Sprintf("%v/.lpData", usr.HomeDir), "data directory")
	rtmpIP := flag.String("rtmpIP", "127.0.0.1", "IP to bind for HTTP RPC commands")
	httpIP := flag.String("httpIP", "127.0.0.1", "IP to bind for HTTP RPC commands")
	bindIPs := flag.String("bindIPs", "", "Comma-separated list of IPs/ports to bind to")
	bootIDs := flag.String("bootIDs", "", "Comma-separated bootstrap node IDs")
	bootAddrs := flag.String("bootAddrs", "", "Comma-separated bootstrap node addresses")
	bootnode := flag.Bool("bootnode", false, "Set to true if starting bootstrap node")
	transcoder := flag.Bool("transcoder", false, "Set to true to be a transcoder")
	gateway := flag.Bool("gateway", false, "Set to true to be a gateway node")
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
	monhost := flag.String("monitorhost", "http://viz.livepeer.org:8081/metrics", "host name for the metrics data collector")
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
		if *bootIDs == "" {
			*bootIDs = RinkebyBootNodeIDs
		}
		if *bootAddrs == "" {
			*bootAddrs = RinkebyBootNodeAddrs
		}
		if !*offchain {
			if *ethUrl == "" {
				if *transcoder {
					*ethUrl = "wss://rinkeby.infura.io/ws"
				} else {
					*ethUrl = "https://rinkeby.infura.io/cFwU3koCZdTqiH6VE4fj"
				}
			}
			if *controllerAddr == "" {
				*controllerAddr = RinkebyControllerAddr
			}
			glog.Infof("***Livepeer is running on the Rinkeby test network: %v***", RinkebyControllerAddr)
		}
	} else if *devenv {
	} else {
		if *bootIDs == "" {
			*bootIDs = MainnetBootNodeIDs
		}
		if *bootAddrs == "" {
			*bootAddrs = MainnetBootNodeAddrs
		}
		if !*offchain {
			if *ethUrl == "" {
				if *transcoder {
					*ethUrl = "wss://mainnet.infura.io/ws"
				} else {
					*ethUrl = "https://mainnet.infura.io/cFwU3koCZdTqiH6VE4fj"
				}
			}
			if *controllerAddr == "" {
				*controllerAddr = MainnetControllerAddr
			}
			glog.Infof("***Livepeer is running on the Ethereum main network: %v***", MainnetControllerAddr)
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
	priv, pub, err := getLPKeys(*datadir)
	if err != nil {
		glog.Errorf("Error getting keys: %v", err)
		return
	}
	notifiee := bnet.NewBasicNotifiee(lpmon.Instance())
	var maddrs []ma.Multiaddr
	if *bindIPs != "" {
		maddrs = make([]ma.Multiaddr, 0)
		mas := strings.Split(*bindIPs, ",")
		i := 0
		for _, m := range mas {
			addr, err := ma.NewMultiaddr(m)
			if err != nil {
				glog.Errorf("Error creating bindIP %v to multiaddr: %v", m, err)
				continue // nonfatal
			}
			maddrs = append(maddrs, addr)
			i++
		}
	} else {
		sourceMultiAddr, _ := ma.NewMultiaddr(fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", *port))
		maddrs = []ma.Multiaddr{sourceMultiAddr}
	}
	node, err := bnet.NewNode(maddrs, priv, pub, notifiee)
	if err != nil {
		glog.Errorf("Error creating a new node: %v", err)
		return
	}
	nw, err := bnet.NewBasicVideoNetwork(node, "127.0.0.1", *port)
	if err != nil {
		glog.Errorf("Cannot create network node: %v", err)
		return
	}

	n, err := core.NewLivepeerNode(nil, nw, core.NodeID(nw.GetNodeID()), *datadir, dbh)
	if err != nil {
		glog.Errorf("Error creating livepeer node: %v", err)
	}
	if *transcoder {
		n.NodeType = core.Transcoder
	} else if *bootnode {
		n.NodeType = core.Bootnode
	} else if *gateway {
		n.NodeType = core.Gateway
	} else {
		n.NodeType = core.Broadcaster
	}

	if *bootnode || *transcoder || *gateway {
		//Bootnodes, transcoders and gateway nodes connect to all the bootnodes
		if err := n.VideoNetwork.SetupProtocol(); err != nil {
			glog.Errorf("Cannot set up protocol:%v", err)
			return
		}
		if err := n.Start(context.Background(), strings.Split(*bootIDs, ","), strings.Split(*bootAddrs, ",")); err != nil {
			glog.Errorf("Cannot connect to bootstrap node: %v", err)
			return
		}
		n.BootIDs = strings.Split(*bootIDs, ",")
		n.BootAddrs = strings.Split(*bootAddrs, ",")
	} else {
		//Connect to the closest bootnode
		localNID, err := peer.IDHexDecode(n.VideoNetwork.GetNodeID())
		if err != nil {
			glog.Errorf("Cannot load local node ID: %v", n.VideoNetwork.GetNodeID())
			return
		}

		if *bootIDs != "" && *bootAddrs != "" {
			indexLookup := make(map[peer.ID]int)
			bIDs := make([]peer.ID, 0)
			for i, bootID := range strings.Split(*bootIDs, ",") {
				id, err := peer.IDHexDecode(bootID)
				if err != nil {
					continue
				}
				bIDs = append(bIDs, id)
				indexLookup[id] = i
			}
			closestNodeID := kb.SortClosestPeers(bIDs, kb.ConvertPeerID(localNID))[0]
			closestNodeAddr := strings.Split(*bootAddrs, ",")[indexLookup[closestNodeID]]
			if err := n.Start(context.Background(), []string{peer.IDHexEncode(closestNodeID)}, []string{closestNodeAddr}); err != nil {
				glog.Errorf("Cannot connect to bootstrap node: %v", err)
				return
			}

			n.BootIDs = []string{peer.IDHexEncode(closestNodeID)}
			n.BootAddrs = []string{closestNodeAddr}
		} else {
			if err := n.Start(context.Background(), []string{}, []string{}); err != nil {
				glog.Errorf("Cannot start node: %v", err)
				return
			}
		}
	}

	if *offchain || *bootnode || *gateway {
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
		node.SetSignFun(client.Sign)
		node.SetVerifyTranscoderSig(func(data []byte, sig []byte, strmID string) bool {
			// look up job by stream id, verify from there
			return true
		})

		if *transcoder {
			addrMap := n.Eth.ContractAddresses()
			em := eth.NewEventMonitor(backend, addrMap)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			if err := setupTranscoder(ctx, n, em, *ipfsPath, *initializeRound, *publicAddr); err != nil {
				glog.Errorf("Error setting up transcoder: %v", err)
				return
			}

			defer n.StopEthServices()
		}
	}

	//Create Livepeer Node
	if *monitor {
		glog.Info("Monitor is set to 'true' by default.  If you want to disable it, use -monitor=false when starting Livepeer.")
		lpmon.Endpoint = *monhost
		n.MonitorMetrics = true
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
	case core.Gateway:
		glog.Infof("***Livepeer Running in Gateway Mode***")
	case core.Bootnode:
		glog.Infof("***Livepeer Running in Bootnode Mode***")
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

	// Start services
	err = n.StartEthServices()
	if err != nil {
		return err
	}

	// Restart jobs as necessary
	err = js.RestartTranscoder()
	if err != nil {
		glog.Errorf("Unable to restart transcoder: %v", err)
		// non-fatal, so continue
	}

	return nil
}
