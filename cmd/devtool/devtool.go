package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"math/big"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/accounts/keystore"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/console"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/livepeer/go-livepeer/eth"

	"github.com/golang/glog"
)

const (
	clientIdentifier = "geth" // Client identifier to advertise over the network
	passphrase       = ""
)

var (
	ethTxTimeout              = 600 * time.Second
	endpoint                  = "http://localhost:8545/"
	gethMiningAccount         = "87da6a8c6e9eff15d703fc2773e32f6af8dbe301"
	gethMiningAccountOverride = false
	ethController             = "0x04B9De88c81cda06165CF65a908e5f1EFBB9493B"
	ethControllerOverride     = false
	serviceHost               = "127.0.0.1"
	serviceURI                = "https://127.0.0.1:"
	cliPort                   = 7935
	mediaPort                 = 8935
	rtmpPort                  = 1935
)

func main() {
	flag.Set("logtostderr", "true")
	baseDataDir := flag.String("datadir", ".lpdev2", "default data directory")
	endpointAddr := flag.String("endpoint", "", "Geth endpoint to connect to")
	miningAccountFlag := flag.String("miningaccount", "", "Override geth mining account (usually not needed)")
	ethControllerFlag := flag.String("controller", "", "Override controller address (usually not needed)")
	svcHost := flag.String("svchost", "127.0.0.1", "default service host")

	flag.Parse()
	if *endpointAddr != "" {
		endpoint = *endpointAddr
	}
	if *miningAccountFlag != "" {
		gethMiningAccount = *miningAccountFlag
		gethMiningAccountOverride = true
	}
	if *ethControllerFlag != "" {
		ethController = *ethControllerFlag
		ethControllerOverride = true
	}
	if *svcHost != "" {
		serviceHost = *svcHost
		serviceURI = fmt.Sprintf("https://%s:", serviceHost)
	}
	args := flag.Args()
	goodToGo := false
	isBroadcaster := true
	if len(args) > 1 && args[0] == "setup" {
		switch args[1] {
		case "broadcaster":
			goodToGo = true
		case "transcoder":
			isBroadcaster = false
			goodToGo = true
		}
	}
	if !goodToGo {
		fmt.Println(`
    Usage: go run cmd/devtool/devtool.go setup broadcaster|transcoder [nodeIndex]
        It will create initilize eth account (on private testnet) to be used for broadcaster or transcoder
        and will create shell script (run_broadcaster_ETHACC.sh or run_transcoder_ETHACC.sh) to run it.
        Node index indicates how much to offset node's port. Orchestrator node's index by default is 1.
        For example:
        "devtool setup broadcaster" will create broadcaster with cli port 7935 and media port 8935
        "devtool setup broadcaster 2" will create broadcaster with cli port 7937 and media port 8937
        "devtool setup transcoder 3" will create transcoder with cli port 7938 and media port 8938`)
		return
	}
	nodeIndex := 0
	if args[1] == "transcoder" {
		nodeIndex = 1
	}
	if len(args) > 2 {
		if i, err := strconv.ParseInt(args[2], 10, 64); err == nil {
			nodeIndex = int(i)
		}
	}
	serviceURI += strconv.Itoa(mediaPort + nodeIndex)
	mediaPort += nodeIndex
	cliPort += nodeIndex
	rtmpPort += nodeIndex

	t := getNodeType(isBroadcaster)

	tmp, err := ioutil.TempDir("", "livepeer")
	if err != nil {
		glog.Fatalf("Can't create temporary directory: %v", err)
	}
	defer os.RemoveAll(tmp)

	tempKeystoreDir := filepath.Join(tmp, "keystore")
	acc := createKey(tempKeystoreDir)
	glog.Infof("Using account %s", acc)
	glog.Infof("Using svchost %s", serviceHost)
	dataDir := filepath.Join(*baseDataDir, t+"_"+acc)
	err = os.MkdirAll(dataDir, 0755)
	if err != nil {
		glog.Fatalf("Can't create directory %v", err)
	}

	keystoreDir := filepath.Join(dataDir, "keystore")
	err = moveDir(tempKeystoreDir, keystoreDir)
	if err != nil {
		glog.Fatal(err)
	}
	remoteConsole(acc)
	ethSetup(acc, keystoreDir, isBroadcaster)
	createRunScript(acc, dataDir, serviceHost, isBroadcaster)
	if !isBroadcaster {
		tDataDir := filepath.Join(*baseDataDir, "transcoder_"+acc)
		err = os.MkdirAll(tDataDir, 0755)
		if err != nil {
			glog.Fatalf("Can't create directory %v", err)
		}
		createTranscoderRunScript(acc, tDataDir, serviceHost)
	}
	glog.Info("Finished")
}

func getNodeType(isBroadcaster bool) string {
	t := "broadcaster"
	if !isBroadcaster {
		t = "orchestrator"
	}
	return t
}

func ethSetup(ethAcctAddr, keystoreDir string, isBroadcaster bool) {
	time.Sleep(3 * time.Second)
	//Set up eth client
	backend, err := ethclient.Dial(endpoint)
	if err != nil {
		glog.Errorf("Failed to connect to Ethereum client: %v", err)
		return
	}
	glog.Infof("Using controller address %s", ethController)

	gpm := eth.NewGasPriceMonitor(backend, 5*time.Second, big.NewInt(0), nil)

	// Start gas price monitor
	_, err = gpm.Start(context.Background())
	if err != nil {
		glog.Errorf("error starting gas price monitor: %v", err)
		return
	}
	defer gpm.Stop()

	chainID, err := backend.ChainID(context.Background())
	if err != nil {
		glog.Errorf("Failed to get chain ID from remote ethereum node: %v", err)
		return
	}

	am, err := eth.NewAccountManager(ethcommon.HexToAddress(ethAcctAddr), keystoreDir, chainID)
	if err != nil {
		glog.Errorf("Error creating Ethereum account manager: %v", err)
		return
	}

	if err := am.Unlock(passphrase); err != nil {
		glog.Errorf("Error unlocking Ethereum account: %v", err)
		return
	}

	tm := eth.NewTransactionManager(backend, gpm, am, 5*time.Minute, 0)
	go tm.Start()
	defer tm.Stop()

	ethCfg := eth.LivepeerEthClientConfig{
		AccountManager:     am,
		ControllerAddr:     ethcommon.HexToAddress(ethController),
		EthClient:          backend,
		GasPriceMonitor:    gpm,
		TransactionManager: tm,
		Signer:             types.LatestSignerForChainID(chainID),
	}

	client, err := eth.NewClient(ethCfg)
	if err != nil {
		glog.Errorf("Failed to create Livepeer Ethereum client: %v", err)
		return
	}

	if err := client.SetGasInfo(0); err != nil {
		glog.Errorf("Failed to set gas info on Livepeer Ethereum Client: %v", err)
		return
	}

	if isBroadcaster {
		amount := new(big.Int).Mul(big.NewInt(100), new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil))

		glog.Infof("Funding deposit with %v", amount)
		glog.Infof("Funding reserve with %v", amount)

		tx, err := client.FundDepositAndReserve(amount, amount)
		if err != nil {
			glog.Error(err)
			return
		}
		if err := client.CheckTx(tx); err != nil {
			glog.Error(err)
			return
		}

		glog.Info("Done funding deposit and reserve")
	} else {
		glog.Infof("Requesting tokens from faucet")

		tx, err := client.Request()
		if err != nil {
			glog.Errorf("Error requesting tokens from faucet: %v", err)
			return
		}

		err = client.CheckTx(tx)
		if err != nil {
			glog.Errorf("Error requesting tokens from faucet: %v", err)
			return
		}
		glog.Info("Done requesting tokens.")
		time.Sleep(4 * time.Second)

		// XXX TODO curl -X "POST" http://localhost:$transcoderCliPort/initializeRound
		time.Sleep(3 * time.Second)
		for {
			currentRound, err := client.CurrentRound()
			if err != nil {
				glog.Errorf("Error getting current round: %v", err)
				return
			}
			if currentRound.Int64() > 1 {
				break
			}
			// first round is initialized and locked, need to wait
			glog.Info("Waiting will first round ended.")
			time.Sleep(4 * time.Second)
		}
		tx, err = client.InitializeRound()
		// ErrRoundInitialized
		if err != nil {
			if err.Error() != "ErrRoundInitialized" {
				glog.Errorf("Error initializing round: %v", err)
				return
			}
		} else {
			err = client.CheckTx(tx)
			if err != nil {
				glog.Errorf("Error initializng round: %v", err)
				return
			}
		}
		glog.Info("Done initializing round.")
		glog.Info("Activating transcoder")
		// curl -d "blockRewardCut=10&feeShare=5&amount=500" --data-urlencode "serviceURI=https://$transcoderServiceAddr" \
		//   -H "Content-Type: application/x-www-form-urlencoded" \
		//   -X "POST" http://localhost:$transcoderCliPort/activateTranscoder\
		var amount *big.Int = big.NewInt(int64(500))
		glog.Infof("Bonding %v to %s", amount, ethAcctAddr)

		tx, err = client.Bond(amount, ethcommon.HexToAddress(ethAcctAddr))
		if err != nil {
			glog.Error(err)
			return
		}

		err = client.CheckTx(tx)
		if err != nil {
			glog.Error("=== Bonding failed")
			glog.Error(err)
			return
		}
		glog.Infof("Registering transcoder %v", ethAcctAddr)

		tx, err = client.Transcoder(eth.FromPerc(10), eth.FromPerc(5))
		if err == eth.ErrCurrentRoundLocked {
			// wait for next round and retry
		}
		if err != nil {
			glog.Error(err)
			return
		}

		err = client.CheckTx(tx)
		if err != nil {
			glog.Error(err)
			return
		}

		glog.Infof("Storing service URI %v in service registry...", serviceURI)

		tx, err = client.SetServiceURI(serviceURI)
		if err != nil {
			glog.Error(err)
			return
		}

		err = client.CheckTx(tx)
		if err != nil {
			glog.Error(err)
		}
	}
}

func createTranscoderRunScript(ethAcctAddr, dataDir, serviceHost string) {
	script := "#!/bin/bash\n"
	// script += fmt.Sprintf(`./livepeer -v 99 -datadir ./%s \
	script += fmt.Sprintf(`./livepeer -v 99 -datadir ./%s -orchSecret secre -orchAddr %s:%d -transcoder`,
		dataDir, serviceHost, mediaPort)
	fName := fmt.Sprintf("run_transcoder_%s.sh", ethAcctAddr)
	err := ioutil.WriteFile(fName, []byte(script), 0755)
	if err != nil {
		glog.Warningf("Error writing run script: %v", err)
	}
}

func createRunScript(ethAcctAddr, dataDir, serviceHost string, isBroadcaster bool) {
	script := "#!/bin/bash\n"
	script += fmt.Sprintf(`./livepeer -v 99 -ethController %s -datadir ./%s \
    -ethAcctAddr %s \
    -ethUrl %s \
    -ethPassword "" \
    -network=devenv \
    -blockPollingInterval 1 \
    -monitor=false -currentManifest=true -cliAddr %s:%d -httpAddr %s:%d `,
		ethController, dataDir, ethAcctAddr, endpoint, serviceHost, cliPort, serviceHost, mediaPort)

	if !isBroadcaster {
		script += fmt.Sprintf(` -initializeRound=true \
    -serviceAddr %s:%d  -transcoder=true -orchestrator=true \
    -orchSecret secre -pricePerUnit 1
    `, serviceHost, mediaPort)
	} else {
		script += fmt.Sprintf(` -broadcaster=true -rtmpAddr %s:%d`, serviceHost, rtmpPort)
	}

	glog.Info(script)
	fName := fmt.Sprintf("run_%s_%s.sh", getNodeType(isBroadcaster), ethAcctAddr)
	err := ioutil.WriteFile(fName, []byte(script), 0755)
	if err != nil {
		glog.Warningf("Error writing run script: %v", err)
	}
}

func createKey(keystoreDir string) string {
	keyStore := keystore.NewKeyStore(keystoreDir, keystore.StandardScryptN, keystore.StandardScryptP)

	account, err := keyStore.NewAccount(passphrase)
	if err != nil {
		glog.Fatal(err)
	}
	glog.Infof("Using ETH account: %v", account.Address.Hex())
	return account.Address.Hex()
}

func remoteConsole(destAccountAddr string) error {
	broadcasterGeth := "0161e041aad467a890839d5b08b138c1e6373072"
	if destAccountAddr != "" {
		broadcasterGeth = destAccountAddr
	}

	client, err := rpc.Dial(endpoint)
	if err != nil {
		glog.Fatalf("Unable to attach to remote geth: %v", err)
	}
	// get mining account
	if !gethMiningAccountOverride {
		var accounts []string
		err = client.Call(&accounts, "eth_accounts")
		if err != nil {
			glog.Fatalf("Error finding mining account: %v", err)
		}
		if len(accounts) == 0 {
			glog.Fatal("Can't find mining account")
		}
		gethMiningAccount = accounts[0]
		glog.Infof("Found mining account: %s", gethMiningAccount)
	}

	tmp, err := ioutil.TempDir("", "console")
	if err != nil {
		glog.Fatalf("Can't create temporary directory: %v", err)
	}
	defer os.RemoveAll(tmp)

	printer := new(bytes.Buffer)

	config := console.Config{
		DataDir: tmp,
		Client:  client,
		Printer: printer,
	}

	console, err := console.New(config)
	if err != nil {
		glog.Fatalf("Failed to start the JavaScript console: %v", err)
	}
	defer console.Stop(false)

	if !ethControllerOverride {
		// f9a6cf519167d81bc5cb3d26c60c0c9a19704aa908c148e82a861b570f4cd2d7 - SetContractInfo event
		getEthControllerScript := `
		var logs = [];
		var filter = web3.eth.filter({ fromBlock: 0, toBlock: "latest",
			topics: ["0xf9a6cf519167d81bc5cb3d26c60c0c9a19704aa908c148e82a861b570f4cd2d7"]});
		filter.get(function(error, result){
			logs.push(result);
		});
		console.log(logs[0][0].address);''
	`
		glog.Infof("Running eth script: %s", getEthControllerScript)
		console.Evaluate(getEthControllerScript)
		if printer.Len() == 0 {
			glog.Fatal("Can't find deployed controller")
		}
		ethController = strings.Split(printer.String(), "\n")[0]

		glog.Infof("Found controller address: %s", ethController)
	}

	script := fmt.Sprintf("eth.sendTransaction({from: \"%s\", to: \"%s\", value: web3.toWei(834, \"ether\")})",
		gethMiningAccount, broadcasterGeth)
	glog.Infof("Running eth script: %s", script)

	console.Evaluate(script)

	time.Sleep(3 * time.Second)

	return err
}

func moveDir(src, dst string) error {
	info, err := os.Lstat(src)
	if err != nil {
		return err
	}
	originalMode := info.Mode()

	if err := os.MkdirAll(dst, os.FileMode(0755)); err != nil {
		return err
	}
	defer os.Chmod(dst, originalMode)

	contents, err := ioutil.ReadDir(src)
	if err != nil {
		return err
	}

	for _, content := range contents {
		cs, cd := filepath.Join(src, content.Name()), filepath.Join(dst, content.Name())
		if err := moveFile(cs, cd, content); err != nil {
			return err
		}
	}

	err = os.Remove(src)
	if err != nil {
		return err
	}

	return nil
}

func moveFile(src, dst string, info os.FileInfo) error {
	if err := os.MkdirAll(filepath.Dir(dst), os.ModePerm); err != nil {
		return err
	}

	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer f.Close()

	if err = os.Chmod(f.Name(), info.Mode()); err != nil {
		return err
	}

	s, err := os.Open(src)
	if err != nil {
		return err
	}
	defer s.Close()

	_, err = io.Copy(f, s)
	if err != nil {
		return err
	}

	err = os.Remove(src)
	if err != nil {
		return err
	}

	return nil
}
