package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"path"
	"path/filepath"
	"time"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/golang/glog"
	crypto "github.com/libp2p/go-libp2p-crypto"
	"github.com/livepeer/golp/core"
	"github.com/livepeer/golp/eth"
	"github.com/livepeer/golp/mediaserver"
	"github.com/livepeer/golp/net"
)

var ErrKeygen = errors.New("ErrKeygen")
var EthRpcTimeout = 10 * time.Second
var EthEventTimeout = 30 * time.Second
var EthMinedTxTimeout = 60 * time.Second

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

func getEthAccount(datadir string) (accounts.Account, error) {
	keyStore := keystore.NewKeyStore(filepath.Join(datadir, "keystore"), keystore.StandardScryptN, keystore.StandardScryptP)
	accounts := keyStore.Accounts()
	if len(accounts) == 0 {
		glog.Errorf("Cannot find geth account, creating a new one")
		return accounts[0], fmt.Errorf("ErrGeth")
	}

	return accounts[0], nil
}

func main() {
	port := flag.Int("p", 0, "port")
	httpPort := flag.String("http", "", "http port")
	rtmpPort := flag.String("rtmp", "", "rtmp port")
	datadir := flag.String("datadir", "", "data directory")
	bootID := flag.String("bootID", "", "Bootstrap node ID")
	bootAddr := flag.String("bootAddr", "", "Bootstrap node addr")
	bootnode := flag.Bool("bootnode", false, "Set to true if starting bootstrap node")
	transcoder := flag.Bool("transcoder", false, "Set to true to be a transcoder")
	newEthAccount := flag.Bool("newEthAccount", false, "Create an eth account")
	ethPassword := flag.String("ethPassword", "", "New Eth account password")
	gethipc := flag.String("gethipc", "", "Geth ipc file location")
	protocolAddr := flag.String("protocolAddr", "", "Protocol smart contract address")

	flag.Parse()

	if *port == 0 {
		glog.Fatalf("Please provide port")
	}
	if *httpPort == "" {
		glog.Fatalf("Please provide http port")
	}
	if *rtmpPort == "" {
		glog.Fatalf("Please provide rtmp port")
	}

	priv, pub, err := getLPKeys(*datadir)
	if err != nil {
		glog.Errorf("Error getting keys: %v", err)
		return
	}

	n, err := core.NewLivepeerNode(*port, priv, pub, nil)
	if err != nil {
		glog.Errorf("Error creating livepeer node: %v", err)
	}

	if *bootnode {
		glog.Infof("Setting up boostrap node")
		//Setup boostrap node
		if err := n.VideoNetwork.SetupProtocol(); err != nil {
			glog.Errorf("Cannot set up protocol:%v", err)
			return
		}
	} else {
		if err := n.Start(*bootID, *bootAddr); err != nil {
			glog.Errorf("Cannot connect to bootstrap node: %v", err)
			return
		}
	}

	var acct accounts.Account

	if *newEthAccount {
		keyStore := keystore.NewKeyStore(filepath.Join(*datadir, "keystore"), keystore.StandardScryptN, keystore.StandardScryptP)
		acct, err = keyStore.NewAccount(*ethPassword)
		if err != nil {
			glog.Errorf("Error creating new eth account: %v", err)
			return
		}
	} else {
		acct, err = getEthAccount(*datadir)
		if err != nil {
			glog.Errorf("Error getting Eth account: %v", err)
			return
		}
	}

	var backend *ethclient.Client
	if *gethipc != "" {
		glog.Infof("Connecting to geth @ %v", *gethipc)
		backend, err = ethclient.Dial(*gethipc)
		if err != nil {
			glog.Errorf("Failed to connect to Ethereum client: %v", err)
			return
		}

		client, err := eth.NewClient(acct, *ethPassword, *datadir, backend, common.HexToAddress(*protocolAddr), EthRpcTimeout, EthEventTimeout)
		if err != nil {
			glog.Errorf("Error creating Eth client: %v", err)
			return
		}
		n.Eth = client
		n.EthPassword = *ethPassword

		if *transcoder {
			logsSub, err := setupTranscoder(n, acct)

			if err != nil {
				glog.Errorf("Error subscribing to job event: %v", err)
			}
			defer logsSub.Unsubscribe()
			// defer close(logsChan)
		}
	} else {
		glog.Infof("Not creating Eth client...")
	}

	s := mediaserver.NewLivepeerMediaServer(*rtmpPort, *httpPort, "", n)
	s.StartLPMS(context.Background())

	select {}
}

func setupTranscoder(n *core.LivepeerNode, acct accounts.Account) (ethereum.Subscription, error) {
	//Check if transcoder is active
	active, err := n.Eth.IsActiveTranscoder()
	if err != nil {
		glog.Errorf("Error getting transcoder state: %v", err)
	}

	if !active {
		glog.Infof("Transcoder %v is inactive", acct.Address.Hex())
	} else {
		s, err := n.Eth.TranscoderStake()
		if err != nil {
			glog.Errorf("Error getting transcoder stake: %v", err)
		}
		glog.Infof("Transcoder Active. Total Stake: %v", s)
	}

	//Check to call reward periodically
	go func() {
		for {
			time.Sleep(time.Second * 5)
			n.CallReward()
		}
	}()

	//Subscribe to when a job is assigned to us
	logsCh := make(chan types.Log)
	sub, err := n.Eth.SubscribeToJobEvent(context.Background(), logsCh)
	if err != nil {
		glog.Errorf("Error subscribing to job event: %v", err)
	}
	go func() error {
		select {
		case l := <-logsCh:
			tx, _, err := n.Eth.Backend().TransactionByHash(context.Background(), l.TxHash)
			if err != nil {
				glog.Errorf("Error getting transaction data: %v", err)
			}
			strmId, tData, err := eth.ParseJobTxData(tx.Data())
			if err != nil {
				glog.Errorf("Error parsing job tx data: %v", err)
			}

			jid, _, _, _, err := eth.GetInfoFromJobEvent(l, n.Eth)
			if err != nil {
				glog.Errorf("Error getting info from job event: %v", err)
			}

			//Create Transcode Config
			//TODO: profile should contain multiple video profiles.  Waiting for a protocol change.
			profile, ok := net.VideoProfileLookup[tData]
			if !ok {
				glog.Errorf("Cannot find video profile for job: %v", tData)
				return core.ErrTranscode
			}

			tProfiles := []net.VideoProfile{profile}
			config := net.TranscodeConfig{StrmID: strmId, Profiles: tProfiles, JobID: jid, PerformOnchainClaim: true}
			glog.Infof("Transcoder got job %v - strmID: %v, tData: %v, config: %v", tx.Hash(), strmId, tData, config)

			//Do The Transcoding
			cm := core.NewClaimManager(strmId, jid, tProfiles, n.Eth)
			strmIDs, err := n.Transcode(config, cm)
			if err != nil {
				glog.Errorf("Transcode Error: %v", err)
			}

			//Notify Broadcaster
			sid := core.StreamID(strmId)
			err = n.NotifyBroadcaster(sid.GetNodeID(), sid, map[core.StreamID]net.VideoProfile{strmIDs[0]: net.VideoProfileLookup[tData]})
			if err != nil {
				glog.Errorf("Notify Broadcaster Error: %v", err)
			}

			return nil

		}
	}()

	return sub, nil
}
