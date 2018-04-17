/*
Livepeer is a peer-to-peer global video live streaming network.  The Golp project is a go implementation of the Livepeer protocol.  For more information, visit the project wiki.
*/
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"math/big"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"os/user"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	kb "gx/ipfs/QmSAFA8v42u4gpJNy1tb7vW3JiiXiaYDC2b845c2RnNSJL/go-libp2p-kbucket"
	ipfslogging "gx/ipfs/QmSpJByNKFX1sCsHBEp3R73FL4NF6FnQTEGyNAXHm2GS52/go-log"
	peer "gx/ipfs/QmXYjuNuxVzXKJCfWasQk1RqkhVLDM9jtUKhqc2WPQmFSB/go-libp2p-peer"
	crypto "gx/ipfs/QmaPbCnUMBohSGo3KnxEa2bHqyJVVeEEcwtqJAYxerieBo/go-libp2p-crypto"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/golang/glog"
	bnet "github.com/livepeer/go-livepeer-basicnet"
	lpcommon "github.com/livepeer/go-livepeer/common"
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
	EthTxTimeout    = 120 * time.Second
	ErrIpfs         = errors.New("ErrIpfs")
)

const RinkebyBootNodeIDs = "122019c1a1f0d9fa2296dccb972e7478c5163415cd55722dcf0123553f397c45df7e,1220abded0103eb4e8e46616881e74e88617409ac4012c6958f4086c649f44eacf89,1220afca402fae8dbb0f980ea2d7e873bc07da36d5463516a862c6199bb6383e9e1e"
const RinkebyBootNodeAddrs = "/ip4/18.217.129.34/tcp/15000,/ip4/52.15.174.204/tcp/15000,/ip4/13.59.47.56/tcp/15000"

func main() {
	flag.Set("logtostderr", "true")

	//Stream Command
	streamCmd := flag.NewFlagSet("stream", flag.ExitOnError)
	streamID := streamCmd.String("id", "", "Stream ID")
	srPort := streamCmd.String("http", "8935", "http port for the video")

	//Broadcast Command
	broadcastCmd := flag.NewFlagSet("broadcast", flag.ExitOnError)
	brtmp := broadcastCmd.Int("rtmp", 1935, "RTMP port for broadcasting.")
	bhttp := broadcastCmd.Int("http", 8935, "HTTP port for getting broadcast streamID.")

	if len(os.Args) > 1 {
		if os.Args[1] == "stream" {
			streamCmd.Parse(os.Args[2:])
			stream(*srPort, *streamID)
			return
		} else if os.Args[1] == "broadcast" {
			broadcastCmd.Parse(os.Args[2:])
			broadcast(*brtmp, *bhttp)
			return
		}
	}

	usr, err := user.Current()
	if err != nil {
		glog.Fatalf("Cannot find current user: %v", err)
	}

	port := flag.Int("p", 15000, "port")
	httpPort := flag.String("http", "8935", "http port")
	rtmpPort := flag.String("rtmp", "1935", "rtmp port")
	datadir := flag.String("datadir", fmt.Sprintf("%v/.lpData", usr.HomeDir), "data directory")
	bootIDs := flag.String("bootID", "", "Comma-separated bootstrap node IDs")
	bootAddrs := flag.String("bootAddr", "", "Comma-separated bootstrap node addresses")
	bootnode := flag.Bool("bootnode", false, "Set to true if starting bootstrap node")
	transcoder := flag.Bool("transcoder", false, "Set to true to be a transcoder")
	gateway := flag.Bool("gateway", false, "Set to true to be a gateway node")
	maxPricePerSegment := flag.String("maxPricePerSegment", "1", "Max price per segment for a broadcast job")
	transcodingOptions := flag.String("transcodingOptions", "P240p30fps16x9,P360p30fps16x9", "Transcoding options for broadcast job")
	ethAcctAddr := flag.String("ethAcctAddr", "", "Existing Eth account address")
	ethPassword := flag.String("ethPassword", "", "Password for existing Eth account address")
	ethKeystorePath := flag.String("ethKeystorePath", "", "Path for the Eth Key")
	ethIpcPath := flag.String("ethIpcPath", "", "Path for eth IPC file")
	ethWsUrl := flag.String("ethWsUrl", "", "geth websocket url")
	rinkeby := flag.Bool("rinkeby", false, "Set to true to connect to rinkeby")
	controllerAddr := flag.String("controllerAddr", "", "Protocol smart contract address")
	gasLimit := flag.Int("gasLimit", 0, "Gas limit for ETH transactions")
	gasPrice := flag.Int("gasPrice", 4000000000, "Gas price for ETH transactions")
	monitor := flag.Bool("monitor", false, "Set to true to send performance metrics")
	monhost := flag.String("monitorhost", "http://viz.livepeer.org:8081/metrics", "host name for the metrics data collector")
	ipfsPath := flag.String("ipfsPath", fmt.Sprintf("%v/.ipfs", usr.HomeDir), "IPFS path")
	noIPFSLogFiles := flag.Bool("noIPFSLogFiles", false, "Set to true if log files should not be generated")
	offchain := flag.Bool("offchain", false, "Set to true to start the node in offchain mode")
	version := flag.Bool("version", false, "Print out the version")

	flag.Parse()

	if *version {
		fmt.Println("Livepeer Node Version: " + core.LivepeerVersion)
		return
	}

	if *rinkeby {
		*bootIDs = RinkebyBootNodeIDs
		*bootAddrs = RinkebyBootNodeAddrs
		if !*offchain {
			*ethWsUrl = "wss://rinkeby.infura.io/ws"
			*controllerAddr = "0x37dc71366ec655093b9930bc816e16e6b587f968"
		}
	}

	//Make sure datadir is present
	if _, err := os.Stat(*datadir); os.IsNotExist(err) {
		glog.Infof("Creating data dir: %v", *datadir)
		if err = os.Mkdir(*datadir, 0755); err != nil {
			glog.Errorf("Error creating datadir: %v", err)
		}
	}

	//Take care of priv/pub keypair
	priv, pub, err := getLPKeys(*datadir)
	if err != nil {
		glog.Errorf("Error getting keys: %v", err)
		return
	}

	//Create Livepeer Node
	if *monitor {
		glog.Info("Monitor is set to 'true' by default.  If you want to disable it, use -monitor=false when starting Livepeer.")
		lpmon.Endpoint = *monhost
	}
	notifiee := bnet.NewBasicNotifiee(lpmon.Instance())
	node, err := bnet.NewNode(*port, priv, pub, notifiee)
	if err != nil {
		glog.Errorf("Error creating a new node: %v", err)
		return
	}
	addrs := make([]string, 0)
	for _, addr := range node.PeerHost.Addrs() {
		addrs = append(addrs, addr.String())
	}
	nw, err := bnet.NewBasicVideoNetwork(node, *datadir)
	if err != nil {
		glog.Errorf("Cannot create network node: %v", err)
		return
	}

	n, err := core.NewLivepeerNode(nil, nw, core.NodeID(nw.GetNodeID()), addrs, *datadir)
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
		if *bootnode {
			glog.Infof("\n\nSetting up bootnode")
		} else if *transcoder {
			glog.Infof("\n\nSetting up transcoder")
		} else if *gateway {
			glog.Infof("\n\nSetting up gateway node")
		}

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
		} else if *ethWsUrl != "" {
			//Connect to specified websocket
			gethUrl = *ethWsUrl
		} else {
			glog.Errorf("Cannot connect to production network yet.")
			return
		}

		glog.Infof("Setting up client...")

		//Set up eth client
		backend, err := ethclient.Dial(gethUrl)
		if err != nil {
			glog.Errorf("Failed to connect to Ethereum client: %v", err)
			return
		}

		client, err := eth.NewClient(common.HexToAddress(*ethAcctAddr), keystoreDir, backend, common.HexToAddress(*controllerAddr), EthTxTimeout)
		if err != nil {
			glog.Errorf("Failed to create client: %v", err)
			return
		}

		var bigGasLimit *big.Int
		var bigGasPrice *big.Int

		if *gasLimit > 0 {
			bigGasLimit = big.NewInt(int64(*gasLimit))
		}

		if *gasPrice > 0 {
			bigGasPrice = big.NewInt(int64(*gasPrice))
		}

		err = client.Setup(*ethPassword, bigGasLimit, bigGasPrice)
		if err != nil {
			glog.Errorf("Failed to setup client: %v", err)
			return
		}

		n.Eth = client

		if *transcoder {
			addrMap := n.Eth.ContractAddresses()
			em := eth.NewEventMonitor(backend, addrMap)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			if err := setupTranscoder(ctx, n, em, *ipfsPath); err != nil {
				glog.Errorf("Error setting up transcoder: %v", err)
				return
			}

			defer n.StopEthServices()
		}
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
	s := server.NewLivepeerServer(*rtmpPort, *httpPort, "", n)
	ec := make(chan error)
	msCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bigMaxPricePerSegment, err := lpcommon.ParseBigInt(*maxPricePerSegment)
	if err != nil {
		glog.Errorf("Error setting max price per segment: %v", err)
		return
	}

	go func() {
		s.StartWebserver()
		ec <- s.StartMediaServer(msCtx, bigMaxPricePerSegment, *transcodingOptions)
	}()

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

func setupTranscoder(ctx context.Context, n *core.LivepeerNode, em eth.EventMonitor, ipfsPath string) error {
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

	// Set up IPFS
	ipfsApi, err := ipfs.StartIpfs(ctx, ipfsPath)
	if err != nil {
		return err
	}

	n.Ipfs = ipfsApi
	n.EthEventMonitor = em

	// Create rounds service to initialize round if it has not already been initialized
	rds := eventservices.NewRoundsService(em, n.Eth)
	n.EthServices = append(n.EthServices, rds)

	// Create reward service to claim/distribute inflationary rewards every round
	rs := eventservices.NewRewardService(em, n.Eth)
	n.EthServices = append(n.EthServices, rs)

	// Create job service to listen for new jobs and transcode if assigned to the job
	js := eventservices.NewJobService(em, n)
	n.EthServices = append(n.EthServices, js)

	// Start services
	err = n.StartEthServices()
	if err != nil {
		return err
	}

	return nil
}

func stream(port string, streamID string) {
	start := time.Now()
	if streamID == "" {
		glog.Errorf("Need to specify streamID via -id")
		return
	}

	//Fetch local stream playlist - this will request from the network so we know the stream is here
	url := fmt.Sprintf("http://localhost:%v/stream/%v.m3u8", port, streamID)
	res, err := http.Get(url)
	if err != nil {
		glog.Fatal(err)
	}
	_, err = ioutil.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		glog.Fatal(err)
	}

	cmd := exec.Command("ffplay", "-timeout", "180000000", url) //timeout in 3 mins
	glog.Infof("url: %v", url)
	err = cmd.Start()
	if err != nil {
		glog.Infof("Couldn't start the stream.  Make sure a local Livepeer node is running on port %v", port)
		os.Exit(1)
	}
	glog.Infof("Now streaming")
	err = cmd.Wait()
	if err != nil {
		glog.Infof("Couldn't start the stream.  Make sure a local Livepeer node is running on port %v", port)
		os.Exit(1)
	}

	if time.Since(start) < time.Second {
		glog.Infof("Error: Make sure local Livepeer node is running on port %v", port)
	} else {
		glog.Infof("Finished the stream")
	}
	return
}

//Run ffmpeg - only works on OSX
func broadcast(rtmpPort int, httpPort int) {
	if runtime.GOOS == "darwin" {
		cmd := exec.Command("ffmpeg", "-f", "avfoundation", "-framerate", "30", "-pixel_format", "uyvy422", "-i", "0:0", "-vcodec", "libx264", "-tune", "zerolatency", "-b", "1000k", "-x264-params", "keyint=60:min-keyint=60", "-acodec", "aac", "-ac", "1", "-b:a", "96k", "-f", "flv", fmt.Sprintf("rtmp://localhost:%v/movie", rtmpPort))

		var out bytes.Buffer
		var stderr bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &stderr
		err := cmd.Start()
		if err != nil {
			glog.Infof("Couldn't broadcast the stream: %v %v", err, stderr.String())
			os.Exit(1)
		}

		glog.Infof("Now broadcasting - %v%v", out.String(), stderr.String())

		time.Sleep(3 * time.Second)
		resp, err := http.Get(fmt.Sprintf("http://localhost:%v/streamID", httpPort))
		if err != nil {
			glog.Errorf("Error getting stream ID: %v", err)
		} else {
			defer resp.Body.Close()
			id, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				glog.Errorf("Error reading stream ID: %v", err)
			}
			glog.Infof("StreamID: %v", string(id))
		}

		if err = cmd.Wait(); err != nil {
			glog.Errorf("Error running broadcast: %v\n%v", err, stderr.String())
			return
		}
	} else {
		glog.Errorf("The broadcast command only support darwin for now.  Please download OBS to broadcast.")
	}
}
