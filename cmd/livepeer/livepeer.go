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

	crypto "gx/ipfs/QmaPbCnUMBohSGo3KnxEa2bHqyJVVeEEcwtqJAYxerieBo/go-libp2p-crypto"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/golang/glog"
	bnet "github.com/livepeer/go-livepeer-basicnet"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/eth"
	lpmon "github.com/livepeer/go-livepeer/monitor"
	"github.com/livepeer/go-livepeer/net"
	"github.com/livepeer/go-livepeer/server"
	"github.com/livepeer/go-livepeer/types"
)

var ErrKeygen = errors.New("ErrKeygen")
var EthRpcTimeout = 10 * time.Second
var EthEventTimeout = 120 * time.Second

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
	bootID := flag.String("bootID", "", "Bootstrap node ID")
	bootAddr := flag.String("bootAddr", "", "Bootstrap node addr")
	bootnode := flag.Bool("bootnode", false, "Set to true if starting bootstrap node")
	transcoder := flag.Bool("transcoder", false, "Set to true to be a transcoder")
	maxPricePerSegment := flag.Int("maxPricePerSegment", 1, "Max price per segment for a broadcast job")
	transcodingOptions := flag.String("transcodingOptions", "P360p30fps16x9", "Transcoding options for broadcast job")
	ethPassword := flag.String("ethPassword", "", "New Eth account password")
	ethAccountAddr := flag.String("ethAccountAddr", "", "Existing Eth account address")
	ethDatadir := flag.String("ethDatadir", "", "geth data directory")
	testnet := flag.Bool("testnet", false, "Set to true to connect to testnet")
	protocolAddr := flag.String("protocolAddr", "0xc7ff57decee68ab792a31eb99af132fa2e6889b0", "Protocol smart contract address")
	tokenAddr := flag.String("tokenAddr", "0x2c7d6359ad65c6ba619d0297974c4763991f2eec", "Token smart contract address")
	faucetAddr := flag.String("faucetAddr", "0x6ae1af1d97625c5d443da7c243d196c30d26354a", "Token faucet smart contract address")
	gasPrice := flag.Int("gasPrice", 4000000000, "Gas price for ETH transactions")
	monitor := flag.Bool("monitor", true, "Set to true to send performance metrics")
	monhost := flag.String("monitorhost", "http://viz.livepeer.org:8081/metrics", "host name for the metrics data collector")
	offchain := flag.Bool("offchain", false, "Set to true to start the node in offchain mode")
	version := flag.Bool("version", false, "Print out the version")

	flag.Parse()

	if *version {
		fmt.Println("Livepeer Node Version: 0.1.0")
		return
	}

	if *testnet {
		*bootID = "12208a4eb428aa57a74ef0593612adb88077c75c71ad07c3c26e4e7a8d4860083b01"
		*bootAddr = "/ip4/52.15.174.204/tcp/15000"
		if *ethDatadir == "" && !*offchain {
			*ethDatadir = fmt.Sprintf("%v/.lpGeth", usr.HomeDir)
		}
	}

	//Make sure datadir is present
	if _, err := os.Stat(*datadir); os.IsNotExist(err) {
		glog.Infof("Creating data dir: %v", *datadir)
		if err = os.Mkdir(*datadir, 0755); err != nil {
			glog.Errorf("Error creating datadir: %v", err)
		}
	}

	// //Set pidfile
	// if _, err = os.Stat(fmt.Sprintf("%v/livepeer.pid", *datadir)); !os.IsNotExist(err) {
	// 	glog.Errorf("Node already running with datadir: %v", *datadir)
	// 	return
	// }
	// pidfile.SetPidfilePath(fmt.Sprintf("%v/livepeer.pid", *datadir))
	// if err = pidfile.Write(); err != nil {
	// 	glog.Errorf("Error writing pidfile: %v", err)
	// 	return
	// }
	// defer os.Remove(fmt.Sprintf("%v/livepeer.pid", *datadir))

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
	nw, err := bnet.NewBasicVideoNetwork(node)
	if err != nil {
		glog.Errorf("Cannot create network node: %v", err)
		return
	}

	n, err := core.NewLivepeerNode(nil, nw, core.NodeID(nw.GetNodeID()), addrs, fmt.Sprintf("%v/.tmp", *datadir))
	if err != nil {
		glog.Errorf("Error creating livepeer node: %v", err)
	}

	if *bootnode {
		glog.Infof("\n\nSetting up bootnode")
		//Setup boostrap node
		if err := n.VideoNetwork.SetupProtocol(); err != nil {
			glog.Errorf("Cannot set up protocol:%v", err)
			return
		}
		lpmon.Instance().SetBootNode()
	} else {
		if err := n.Start(*bootID, *bootAddr); err != nil {
			glog.Errorf("Cannot connect to bootstrap node: %v", err)
			return
		}
	}

	var gethCmd *exec.Cmd
	//Set up ethereum-related stuff
	if *ethDatadir != "" && !*offchain {
		gethipc := filepath.Join(filepath.Join(*ethDatadir, "geth.ipc"))
		//If getipc file is not there, start the geth node
		if _, err := os.Stat(gethipc); os.IsNotExist(err) {
			//Set up geth params depending on network, invoke geth
			if *testnet {
				gethCmd = exec.Command("geth", "--rpc", "--datadir", *ethDatadir, "--networkid=858585", "--bootnodes=enode://a1bd18a737acef008f94857654cfb2470124d1dc826b6248cea0331a7ca82b36d2389566e3aa0a1bc9a5c3c34a61f47601a6cff5279d829fcc60cb632ee88bad@13.58.149.151:30303") //timeout in 3 mins
				err = gethCmd.Start()
				if err != nil {
					glog.Infof("Couldn't start geth: %v", err)
					return
				}
				defer gethCmd.Process.Kill()
				go func() {
					err = gethCmd.Wait()
					if err != nil {
						glog.Infof("Couldn't start geth: %v", err)
						os.Exit(1)
					}
				}()
			} else {
				glog.Errorf("Cannot connect to product network yet.")
				return
			}
		}

		var backend *ethclient.Client
		var acct accounts.Account

		acct, err = getEthAccount(*ethDatadir, *ethAccountAddr)
		if err != nil {
			glog.Infof("Cannot find eth account - creating new account")
			keyStore := keystore.NewKeyStore(filepath.Join(*ethDatadir, "keystore"), keystore.StandardScryptN, keystore.StandardScryptP)
			acct, err = keyStore.NewAccount(*ethPassword)
			if err != nil {
				glog.Errorf("Error creating new eth account: %v", err)
				return
			}
		}
		glog.Infof("Using Eth account: %v", acct.Address.Hex())

		//Wait for gethipc
		if _, err := os.Stat(gethipc); os.IsNotExist(err) {
			start := time.Now()
			glog.V(0).Infof("Waiting to start go-ethereum")
			for time.Since(start) < time.Second*5 {
				if _, err := os.Stat(gethipc); os.IsNotExist(err) {
					time.Sleep(time.Millisecond * 500)
				} else {
					continue
				}
			}
		}

		backend, err = ethclient.Dial(gethipc)
		if err != nil {
			glog.Errorf("Failed to connect to Ethereum client: %v", err)
			return
		}

		client, err := eth.NewClient(acct, *ethPassword, *ethDatadir, backend, big.NewInt(int64(*gasPrice)), common.HexToAddress(*protocolAddr), common.HexToAddress(*tokenAddr), common.HexToAddress(*faucetAddr), EthRpcTimeout, EthEventTimeout)
		if err != nil {
			glog.Errorf("Error creating Eth client: %v", err)
			return
		}
		n.Eth = client
		n.EthAccount = acct.Address.String()
		n.EthPassword = *ethPassword

		if *transcoder {
			logsCh := make(chan ethtypes.Log)
			logsSub, err := n.Eth.SubscribeToJobEvent(context.Background(), logsCh)
			if err != nil {
				glog.Errorf("Error subscribing to job event: %v", err)
			}

			defer close(logsCh)
			defer logsSub.Unsubscribe()

			if err := setupTranscoder(n, logsCh); err != nil {
				glog.Errorf("Error setting up transcoder: %v", err)
				return
			}
		}
	} else {
		glog.Infof("***Livepeer is in off-chain mode***")
	}

	//Set up the media server
	glog.Infof("\n\nSetting up Media Server")
	s := server.NewLivepeerServer(*rtmpPort, *httpPort, "", n)
	ec := make(chan error)
	msCtx, cancel := context.WithCancel(context.Background())
	go func() {
		s.StartWebserver()
		ec <- s.StartMediaServer(msCtx, *maxPricePerSegment, *transcodingOptions)
	}()

	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt)
	select {
	case err := <-ec:
		glog.Infof("Error from media server: %v", err)
		cancel()
		return
	case <-msCtx.Done():
		glog.Infof("MediaServer Done()")
		cancel()
		return
	case sig := <-c:
		glog.Infof("Exiting Livepeer: %v", sig)
		time.Sleep(time.Millisecond * 500) //Give time for other processes to shut down completely
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

func getEthAccount(datadir string, addr string) (accounts.Account, error) {
	keyStore := keystore.NewKeyStore(filepath.Join(datadir, "keystore"), keystore.StandardScryptN, keystore.StandardScryptP)
	accts := keyStore.Accounts()
	if len(accts) == 0 {
		// glog.Errorf("Cannot find geth account.  Make sure the data directory contains keys, or use -newEthAccount to create a new account.")
		return accounts.Account{}, fmt.Errorf("ErrGeth")
	}

	if addr != "" {
		for _, acct := range accts {
			if acct.Address == common.HexToAddress(addr) {
				return acct, nil
			}
		}

		glog.Errorf("Cannot find geth account")
		return accounts.Account{}, fmt.Errorf("ErrGeth")
	}

	return accts[0], nil
}

func setupTranscoder(n *core.LivepeerNode, logsCh chan ethtypes.Log) error {
	//Check if transcoder is active
	active, err := n.Eth.IsActiveTranscoder()
	if err != nil {
		glog.Errorf("Error getting transcoder state: %v", err)
		return err
	}

	if !active {
		glog.Infof("Transcoder %v is inactive", n.Eth.Account().Address.Hex())
	} else {
		s, err := n.Eth.TranscoderStake()
		if err != nil {
			glog.Errorf("Error getting transcoder stake: %v", err)
		}
		glog.Infof("Transcoder Active. Total Stake: %v", s)
	}

	rm := core.NewRewardManager(time.Second*5, n.Eth)
	go rm.Start(context.Background())

	go func() {
		for {
			select {
			case l, ok := <-logsCh:
				if !ok {
					// Logs channel closed, exit loop
					return
				}

				_, _, jid := eth.ParseNewJobLog(l)

				job, err := n.Eth.GetJob(jid)
				if err != nil {
					glog.Errorf("Error getting job info: %v", err)
					continue
				}

				//Check if broadcaster has enough funds
				bDeposit, err := n.Eth.GetBroadcasterDeposit(job.BroadcasterAddress)
				if err != nil {
					glog.Errorf("Error getting broadcaster deposit: %v", err)
					continue
				}
				if bDeposit.Cmp(big.NewInt(0)) == 0 {
					glog.Errorf("Broadcaster does not have enough funds. Skipping job")
					continue
				}

				//Create Transcode Config
				tProfiles, err := txDataToVideoProfile(job.TranscodingOptions)
				if err != nil {
					glog.Errorf("Error processing job transcoding options: %v", err)
					continue
				}

				config := net.TranscodeConfig{StrmID: job.StreamId, Profiles: tProfiles, JobID: jid, PerformOnchainClaim: true}
				glog.Infof("Transcoder got job %v - strmID: %v, tData: %v, config: %v", jid, job.StreamId, job.TranscodingOptions, config)

				//Do The Transcoding
				cm := core.NewClaimManager(job.StreamId, jid, job.BroadcasterAddress, job.PricePerSegment, tProfiles, n.Eth)
				strmIDs, err := n.TranscodeAndBroadcast(config, cm)
				if err != nil {
					glog.Errorf("Transcode Error: %v", err)
					continue
				}

				//Notify Broadcaster
				sid := core.StreamID(job.StreamId)
				vids := make(map[core.StreamID]types.VideoProfile)
				for i, vp := range tProfiles {
					vids[strmIDs[i]] = vp
				}
				if err = n.NotifyBroadcaster(sid.GetNodeID(), sid, vids); err != nil {
					glog.Errorf("Notify Broadcaster Error: %v", err)
				}
			}
		}
	}()

	return nil
}

func txDataToVideoProfile(txData string) ([]types.VideoProfile, error) {
	profiles := make([]types.VideoProfile, 0)
	for _, txp := range strings.Split(txData, "|") {
		p, ok := types.VideoProfileLookup[txp]
		if !ok {
			glog.Errorf("Cannot find video profile for job: %v", txp)
			return nil, core.ErrTranscode
		}
		profiles = append(profiles, p)
	}
	return profiles, nil
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
