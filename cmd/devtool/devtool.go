package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/accounts/keystore"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/console"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/livepeer/go-livepeer/eth"

	"github.com/golang/glog"
)

const (
	clientIdentifier = "geth" // Client identifier to advertise over the network
	passphrase       = ""
	serviceURI       = "https://127.0.0.1:8936"
)

var (
	ethTxTimeout              = 600 * time.Second
	endpoint                  = "ws://localhost:8546/"
	gethMiningAccount         = "87da6a8c6e9eff15d703fc2773e32f6af8dbe301"
	gethMiningAccountOverride = false
	ethController             = "0x04B9De88c81cda06165CF65a908e5f1EFBB9493B"
	ethControllerOverride     = false
)

func main() {
	flag.Set("logtostderr", "true")
	baseDataDir := flag.String("datadir", ".lpdev2", "default data directory")
	endpointAddr := flag.String("endpoint", "", "Geth endpoint to connect to")
	miningAccountFlag := flag.String("miningaccount", "", "Override geth mining account (usually not needed)")
	ethControllerFlag := flag.String("controller", "", "Override controller address (usually not needed)")

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
    Usage: go run cmd/devtool/devtool.go setup broadcaster|transcoder
        It will create initilize eth account (on private testnet) to be used for broadcaster or transcoder
        and will create shell script (run_broadcaster_ETHACC.sh or run_transcoder_ETHACC.sh) to run it.`)
		return
	}

	t := getNodeType(isBroadcaster)

	tmp, err := ioutil.TempDir("", "livepeer")
	if err != nil {
		glog.Fatalf("Can't create temporary directory: %v", err)
	}
	defer os.RemoveAll(tmp)

	tempKeystoreDir := filepath.Join(tmp, "keystore")
	acc := createKey(tempKeystoreDir)
	glog.Infof("Using account %s", acc)
	dataDir := filepath.Join(*baseDataDir, t+"_"+acc)
	dataDirToCreate := filepath.Join(dataDir, "devenv")
	err = os.MkdirAll(dataDirToCreate, 0755)
	if err != nil {
		glog.Fatalf("Can't create directory %v", err)
	}

	keystoreDir := filepath.Join(dataDirToCreate, "keystore")
	err = os.Rename(tempKeystoreDir, keystoreDir)
	if err != nil {
		glog.Fatal(err)
	}
	remoteConsole(acc)
	ethSetup(acc, keystoreDir, isBroadcaster)
	createRunScript(acc, dataDir, isBroadcaster)
	glog.Info("Finished")
}

func getNodeType(isBroadcaster bool) string {
	t := "broadcaster"
	if !isBroadcaster {
		t = "transcoder"
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

	client, err := eth.NewClient(ethcommon.HexToAddress(ethAcctAddr), keystoreDir, backend,
		ethcommon.HexToAddress(ethController), ethTxTimeout)
	if err != nil {
		glog.Errorf("Failed to create client: %v", err)
		return
	}

	var bigGasPrice *big.Int = big.NewInt(int64(200))
	// var bigGasPrice *big.Int = big.NewInt(int64(00000))

	err = client.Setup(passphrase, 1000000, bigGasPrice)
	if err != nil {
		glog.Fatalf("Failed to setup client: %v", err)
		return
	}
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

	var depositAmount *big.Int = big.NewInt(int64(5000))

	glog.Infof("Depositing: %v", depositAmount)

	tx, err = client.FundDeposit(depositAmount)
	if err != nil {
		glog.Error(err)
		return
	}
	err = client.CheckTx(tx)
	if err != nil {
		glog.Error(err)
		return
	}
	glog.Info("Done depositing")

	if !isBroadcaster {
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
		tx, err := client.InitializeRound()
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
		// curl -d "blockRewardCut=10&feeShare=5&pricePerSegment=1&amount=500" --data-urlencode "serviceURI=https://$transcoderServiceAddr" \
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
			glog.Error(err)
			return
		}
		glog.Infof("Registering transcoder %v", ethAcctAddr)
		price := big.NewInt(1)

		tx, err = client.Transcoder(eth.FromPerc(10), eth.FromPerc(5), price)
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

func createRunScript(ethAcctAddr, dataDir string, isBroadcaster bool) {
	script := "#!/bin/bash\n"
	script += fmt.Sprintf(`./livepeer -v 99 -ethController %s -datadir ./%s \
    -ethAcctAddr %s \
    -ethUrl %s \
    -ethPassword "" \
    -gasPrice 200 -gasLimit 2000000 -network=devenv \
    -monitor=false -currentManifest=true `,
		ethController, dataDir, ethAcctAddr, endpoint)

	if !isBroadcaster {
		script += fmt.Sprintf(` -initializeRound=true \
    -serviceAddr 127.0.0.1:8936 -httpAddr 127.0.0.1:8936  -transcoder \
    -cliAddr 127.0.0.1:7936 -ipfsPath ./%s/trans
    `, dataDir)
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
		getethControllerScript := `
		var logs = [];
		var filter = web3.eth.filter({ fromBlock: 0, toBlock: "latest",
			topics: ["0xf9a6cf519167d81bc5cb3d26c60c0c9a19704aa908c148e82a861b570f4cd2d7"]});
		filter.get(function(error, result){
			logs.push(result);
		});
		console.log(logs[0][0].address);''
	`
		glog.Infof("Running eth script: %s", getethControllerScript)
		err = console.Evaluate(getethControllerScript)
		if err != nil {
			glog.Error(err)
		}
		if printer.Len() == 0 {
			glog.Fatal("Can't find deployed controller")
		}
		ethController = strings.Split(printer.String(), "\n")[0]

		glog.Infof("Found controller address: %s", ethController)
	}

	script := fmt.Sprintf("eth.sendTransaction({from: \"%s\", to: \"%s\", value: web3.toWei(834, \"ether\")})",
		gethMiningAccount, broadcasterGeth)
	glog.Infof("Running eth script: %s", script)

	err = console.Evaluate(script)
	if err != nil {
		glog.Error(err)
	}

	time.Sleep(3 * time.Second)

	return err
}
