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
	"time"

	"github.com/livepeer/lpms/transcoder"

	crypto "gx/ipfs/QmaPbCnUMBohSGo3KnxEa2bHqyJVVeEEcwtqJAYxerieBo/go-libp2p-crypto"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/cmd/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/console"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/golang/glog"
	bnet "github.com/livepeer/go-livepeer-basicnet"
	lpcommon "github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/eth"
	"github.com/livepeer/go-livepeer/ipfs"
	lpmon "github.com/livepeer/go-livepeer/monitor"
	"github.com/livepeer/go-livepeer/net"
	"github.com/livepeer/go-livepeer/server"
	lpmscore "github.com/livepeer/lpms/core"
)

var ErrKeygen = errors.New("ErrKeygen")
var EthRpcTimeout = 10 * time.Second
var EthEventTimeout = 120 * time.Second
var ErrIpfs = errors.New("ErrIpfs")

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
	defaultEthDatadir := fmt.Sprintf("%v/.lpGeth", usr.HomeDir)

	port := flag.Int("p", 15000, "port")
	httpPort := flag.String("http", "8935", "http port")
	rtmpPort := flag.String("rtmp", "1935", "rtmp port")
	datadir := flag.String("datadir", fmt.Sprintf("%v/.lpData", usr.HomeDir), "data directory")
	bootID := flag.String("bootID", "", "Bootstrap node ID")
	bootAddr := flag.String("bootAddr", "", "Bootstrap node addr")
	bootnode := flag.Bool("bootnode", false, "Set to true if starting bootstrap node")
	transcoder := flag.Bool("transcoder", false, "Set to true to be a transcoder")
	maxPricePerSegment := flag.Int("maxPricePerSegment", 1, "Max price per segment for a broadcast job")
	transcodingOptions := flag.String("transcodingOptions", "P240p30fps16x9,P360p30fps16x9", "Transcoding options for broadcast job")
	ethAcctAddr := flag.String("ethAcctAddr", "", "Existing Eth account address")
	ethKeyPath := flag.String("ethKeyPath", "", "Path for the Eth Key")
	ethPassword := flag.String("ethPassword", "", "Eth account password")
	ethIpcPath := flag.String("ethIpcPath", "", "Path for eth IPC file")
	ethWsUrl := flag.String("ethWsUrl", "", "geth websocket url")
	testnet := flag.Bool("testnet", false, "Set to true to connect to testnet")
	controllerAddr := flag.String("controllerAddr", "", "Protocol smart contract address")
	gasPrice := flag.Int("gasPrice", 4000000000, "Gas price for ETH transactions")
	monitor := flag.Bool("monitor", true, "Set to true to send performance metrics")
	monhost := flag.String("monitorhost", "http://viz.livepeer.org:8081/metrics", "host name for the metrics data collector")
	ipfsPath := flag.String("ipfsPath", fmt.Sprintf("%v/.ipfs", usr.HomeDir), "IPFS path")
	offchain := flag.Bool("offchain", false, "Set to true to start the node in offchain mode")
	version := flag.Bool("version", false, "Print out the version")

	flag.Parse()

	if *version {
		fmt.Println("Livepeer Node Version: 0.1.7-unstable")
		return
	}

	if *testnet {
		*bootID = "12208a4eb428aa57a74ef0593612adb88077c75c71ad07c3c26e4e7a8d4860083b01"
		*bootAddr = "/ip4/52.15.174.204/tcp/15000"

		if !*offchain {
			*ethWsUrl = "ws://ethws-testnet.livepeer.org:8546"
			*controllerAddr = "0x0875085bc9a970055ddb213330c027cd9d26e20e"
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

	if *bootnode {
		glog.Infof("\n\nSetting up bootnode")
		//Setup boostrap node
		if err := n.VideoNetwork.SetupProtocol(); err != nil {
			glog.Errorf("Cannot set up protocol:%v", err)
			return
		}
		lpmon.Instance().SetBootNode()
	} else {
		if err := n.Start(context.Background(), *bootID, *bootAddr); err != nil {
			glog.Errorf("Cannot connect to bootstrap node: %v", err)
			return
		}
	}

	var gethCmd *exec.Cmd
	if *offchain {
		glog.Infof("***Livepeer is in off-chain mode***")
	} else {
		var acct accounts.Account
		var keystoreDir string
		if _, err := os.Stat(*ethKeyPath); !os.IsNotExist(err) {
			//Try loading eth key from ethKeyPath
			data, err := ioutil.ReadFile(*ethKeyPath)
			if err != nil {
				glog.Errorf("Cannot read key from %v", *ethKeyPath)
				return
			}

			var objmap map[string]*json.RawMessage
			if err := json.Unmarshal(data, &objmap); err != nil {
				glog.Errorf("Cannot parse key from %v", *ethKeyPath)
				return
			}
			var addr string
			if err := json.Unmarshal(*objmap["address"], &addr); err != nil {
				glog.Errorf("Cannot find address in %v", *ethKeyPath)
				return
			}
			keystoreDir, _ = filepath.Split(*ethKeyPath)
			acct, err = getEthAccount(keystoreDir, addr)
			if err != nil {
				glog.Errorf("Cannot get account %v in %v", addr, keystoreDir)
				return
			}
		} else {
			keystoreDir = filepath.Join(*datadir, "keystore")
			//Try loading eth key from datadir
			if _, err := os.Stat(keystoreDir); !os.IsNotExist(err) {
				acct, err = getEthAccount(keystoreDir, *ethAcctAddr)
				if err != nil {
					glog.Errorf("Cannot get account %v from %v", *ethAcctAddr, *datadir)
					if acct, *ethPassword, err = createEthAccount(keystoreDir); err != nil {
						glog.Errorf("Cannot create Eth account.")
						return
					}
				}
			} else {
				//Try to create a new Eth key
				if acct, *ethPassword, err = createEthAccount(keystoreDir); err != nil {
					glog.Errorf("Cannot create Eth account.")
					return
				}
			}
		}

		if acct.Address.Hex() == "0x0000000000000000000000000000000000000000" {
			glog.Errorf("Cannot find eth account")
			return
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
			//Default behavior - start a Geth node and connect to its IPC
			if *testnet {
				gethCmd = exec.Command("geth", "--rpc", "--datadir", defaultEthDatadir, "--networkid=858585", "--bootnodes=enode://a1bd18a737acef008f94857654cfb2470124d1dc826b6248cea0331a7ca82b36d2389566e3aa0a1bc9a5c3c34a61f47601a6cff5279d829fcc60cb632ee88bad@13.58.149.151:30303") //timeout in 3 mins
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
				gethipc := fmt.Sprintf("%v/geth.ipc", defaultEthDatadir)
				//Wait for gethipc
				if _, err := os.Stat(gethipc); os.IsNotExist(err) {
					start := time.Now()
					glog.V(0).Infof("Waiting to start go-ethereum")
					for time.Since(start) < time.Second*5 {
						if _, err := os.Stat(gethipc); os.IsNotExist(err) {
							time.Sleep(time.Millisecond * 500)
							continue
						} else {
							break
						}
					}
				}
				gethUrl = gethipc

			} else {
				glog.Errorf("Cannot connect to production network yet.")
				return
			}
		}
		glog.Infof("Using Eth account: %v", acct.Address.Hex())

		//Set up eth client
		backend, err := ethclient.Dial(gethUrl)
		if err != nil {
			glog.Errorf("Failed to connect to Ethereum client: %v", err)
			return
		}

		var client *eth.Client
		for firstTime := true; ; {
			client, err = eth.NewClient(acct, *ethPassword, keystoreDir, backend, big.NewInt(int64(*gasPrice)), common.HexToAddress(*controllerAddr), EthRpcTimeout, EthEventTimeout)
			if err != nil {
				if err == keystore.ErrDecrypt {
					if !firstTime {
						glog.Infof("Error decrypting using passphrase. Please provide the passphrase again.")
					} else {
						glog.Infof("Please provide the passphrase.")
						firstTime = false
					}
					*ethPassword = getPassphrase(false)
					continue
				}
				glog.Errorf("Error creating Eth client: %v", err)
				return
			}
			break
		}
		n.Eth = client
		n.EthAccount = acct.Address.String()
		n.EthPassword = *ethPassword

		//Create LogMonitor, the addresses act as filters.
		var logMonitor *eth.LogMonitor
		if *transcoder {
			logMonitor = eth.NewLogMonitor(client, common.Address{})
		} else {
			logMonitor = eth.NewLogMonitor(client, client.Account().Address)
		}
		logMonitor.SubscribeToJobEvents(func(j *eth.Job) {
			n.VideoDB.AddJid(core.StreamID(j.StreamId), j.JobId)
		})

		if *transcoder {
			if err := setupTranscoder(n, logMonitor); err != nil {
				glog.Errorf("Error setting up transcoder: %v", err)
				return
			}
		}
	}

	if *transcoder {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		ipfsApi, err := ipfs.StartIpfs(ctx, *ipfsPath)
		if err != nil {
			glog.Errorf("Error starting ipfs: %v", err)
			return
		}

		n.Ipfs = ipfsApi
	}

	//Set up the media server
	s := server.NewLivepeerServer(*rtmpPort, *httpPort, "", n)
	ec := make(chan error)
	msCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		s.StartWebserver()
		ec <- s.StartMediaServer(msCtx, *maxPricePerSegment, *transcodingOptions)
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

func getEthAccount(keystoreDir string, addr string) (accounts.Account, error) {
	keyStore := keystore.NewKeyStore(keystoreDir, keystore.StandardScryptN, keystore.StandardScryptP)
	accts := keyStore.Accounts()
	if len(accts) == 0 {
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

func setupTranscoder(n *core.LivepeerNode, lm *eth.LogMonitor) error {
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

	//Set up callback for when a job is assigned to us (via monitoring the eth log)
	lm.SubscribeToJobEvents(func(job *eth.Job) {
		//Check if broadcaster has enough funds
		bDeposit, err := n.Eth.GetBroadcasterDeposit(job.BroadcasterAddress)
		if err != nil {
			glog.Errorf("Error getting broadcaster deposit: %v", err)
			return
		}
		if bDeposit.Cmp(big.NewInt(0)) == 0 {
			glog.Errorf("Broadcaster does not have enough funds. Skipping job")
			return
		}

		//Create transcode config, make sure the profiles are sorted
		tProfiles, err := txDataToVideoProfile(job.TranscodingOptions)
		if err != nil {
			glog.Errorf("Error processing job transcoding options: %v", err)
			return
		}

		config := net.TranscodeConfig{StrmID: job.StreamId, Profiles: tProfiles, JobID: job.JobId, PerformOnchainClaim: true}
		glog.Infof("Transcoder got job %v - strmID: %v, tData: %v, config: %v", job.JobId, job.StreamId, job.TranscodingOptions, config)

		//Do The Transcoding
		cm := core.NewBasicClaimManager(job.StreamId, job.JobId, job.BroadcasterAddress, job.MaxPricePerSegment, tProfiles, n.Eth, n.Ipfs)
		tr := transcoder.NewFFMpegSegmentTranscoder(tProfiles, "", n.WorkDir)
		strmIDs, err := n.TranscodeAndBroadcast(config, cm, tr)
		if err != nil {
			glog.Errorf("Transcode Error: %v", err)
			return
		}

		//Notify Broadcaster
		sid := core.StreamID(job.StreamId)
		vids := make(map[core.StreamID]lpmscore.VideoProfile)
		for i, vp := range tProfiles {
			vids[strmIDs[i]] = vp
		}
		if err = n.NotifyBroadcaster(sid.GetNodeID(), sid, vids); err != nil {
			glog.Errorf("Notify Broadcaster Error: %v", err)
		}
	})

	return nil
}

func txDataToVideoProfile(txData string) ([]lpmscore.VideoProfile, error) {
	profiles := make([]lpmscore.VideoProfile, 0)

	for i := 0; i+lpcommon.VideoProfileIDSize < len(txData); i += lpcommon.VideoProfileIDSize {
		txp := txData[i : i+lpcommon.VideoProfileIDSize]

		p, ok := lpmscore.VideoProfileLookup[lpcommon.VideoProfileNameLookup[txp]]
		if !ok {
			// glog.Errorf("Cannot find video profile for job: %v", txp)
			// return nil, core.ErrTranscode
		} else {
			profiles = append(profiles, p)
		}
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

func createEthAccount(keystoreDir string) (acct accounts.Account, passphrase string, err error) {
	if err := os.Mkdir(keystoreDir, 0755); err != nil {
		glog.Errorf("Error creating datadir: %v", err)
	}

	glog.Infoln("Creating a new Ethereum account.  Your new account is locked with a password. Please give a password. Do not forget this password.")
	passphrase = getPassphrase(true)
	keyStore := keystore.NewKeyStore(keystoreDir, keystore.StandardScryptN, keystore.StandardScryptP)
	acct, err = keyStore.NewAccount(passphrase)
	return
}

func getPassphrase(confirmation bool) string {
	password, err := console.Stdin.PromptPassword("Passphrase: ")
	if err != nil {
		utils.Fatalf("Failed to read passphrase: %v", err)
	}
	if confirmation {
		confirm, err := console.Stdin.PromptPassword("Repeat passphrase: ")
		if err != nil {
			utils.Fatalf("Failed to read passphrase confirmation: %v", err)
		}
		if password != confirm {
			utils.Fatalf("Passphrases do not match")
		}
	}
	return password
}
