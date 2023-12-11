package devtool

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/ethereum/go-ethereum/accounts/keystore"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/console"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/eth"
	"io/ioutil"
	"math/big"
	"os"
	"strings"
	"time"
)

const (
	passphrase   = ""
	ethTxTimeout = 5 * time.Minute
)

type DevtoolConfig struct {
	Endpoint                  string
	GethMiningAccount         string
	GethMiningAccountOverride bool
	EthController             string
	EthControllerOverride     bool
	ServiceURI                string
	Account                   string
	KeystoreDir               string
	IsBroadcaster             bool
}

func NewDevtoolConfig() DevtoolConfig {
	return DevtoolConfig{
		Endpoint:          "http://localhost:8545/",
		GethMiningAccount: "87da6a8c6e9eff15d703fc2773e32f6af8dbe301",
		EthController:     "0x04B9De88c81cda06165CF65a908e5f1EFBB9493B",
		ServiceURI:        "https://127.0.0.1:",
	}
}

type Devtool struct {
	EthController string
	Client        eth.LivepeerEthClient
	gpm           *eth.GasPriceMonitor
	tm            *eth.TransactionManager
}

func Init(cfg DevtoolConfig) (Devtool, error) {
	ethController, err := RemoteConsole(cfg)
	if err != nil {
		return Devtool{}, err
	}
	cfg.EthController = ethController
	client, gpm, tm, err := EthSetup(cfg)
	if err != nil {
		return Devtool{}, err
	}
	return Devtool{EthController: ethController, Client: client, gpm: gpm, tm: tm}, nil
}

func RemoteConsole(cfg DevtoolConfig) (string, error) {
	broadcasterGeth := "0161e041aad467a890839d5b08b138c1e6373072"
	if cfg.Account != "" {
		broadcasterGeth = cfg.Account
	}

	client, err := rpc.Dial(cfg.Endpoint)
	if err != nil {
		glog.Exitf("Unable to attach to remote geth: %v", err)
		return "", err
	}
	// get mining account
	gethMiningAccount := cfg.GethMiningAccount
	if !cfg.GethMiningAccountOverride {
		var accounts []string
		err = client.Call(&accounts, "eth_accounts")
		if err != nil {
			glog.Exitf("Error finding mining account: %v", err)
			return "", err
		}
		if len(accounts) == 0 {
			glog.Exit("Can't find mining account")
			return "", errors.New("can't find mining account")
		}
		gethMiningAccount = accounts[0]
		glog.Infof("Found mining account: %s", gethMiningAccount)
	}

	tmp, err := ioutil.TempDir("", "console")
	if err != nil {
		glog.Exitf("Can't create temporary directory: %v", err)
		return "", err
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
		glog.Exitf("Failed to start the JavaScript console: %v", err)
		return "", err
	}
	defer console.Stop(false)

	ethController := cfg.EthController
	if !cfg.EthControllerOverride {
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
			glog.Exit("Can't find deployed controller")
			return "", errors.New("can't find deployed controller")
		}
		ethController = strings.Split(printer.String(), "\n")[0]

		glog.Infof("Found controller address: %s", ethController)
	}

	script := fmt.Sprintf("eth.sendTransaction({from: \"%s\", to: \"%s\", value: web3.toWei(834, \"ether\")})",
		gethMiningAccount, broadcasterGeth)
	glog.Infof("Running eth script: %s", script)

	console.Evaluate(script)

	time.Sleep(3 * time.Second)

	return ethController, nil
}

func EthSetup(cfg DevtoolConfig) (eth.LivepeerEthClient, *eth.GasPriceMonitor, *eth.TransactionManager, error) {
	//Set up eth client
	backend, err := ethclient.Dial(cfg.Endpoint)
	if err != nil {
		glog.Errorf("Failed to connect to Ethereum client: %v", err)
		return nil, nil, nil, err
	}
	glog.Infof("Using controller address %s", cfg.EthController)

	gpm := eth.NewGasPriceMonitor(backend, 5*time.Second, big.NewInt(0), nil)

	// Start gas price monitor
	_, err = gpm.Start(context.Background())
	if err != nil {
		glog.Errorf("error starting gas price monitor: %v", err)
		return nil, nil, nil, err
	}

	chainID, err := backend.ChainID(context.Background())
	if err != nil {
		glog.Errorf("Failed to get chain ID from remote ethereum node: %v", err)
		return nil, nil, nil, err
	}

	am, err := eth.NewAccountManager(ethcommon.HexToAddress(cfg.Account), cfg.KeystoreDir, chainID, passphrase)
	if err != nil {
		glog.Errorf("Error creating Ethereum account manager: %v", err)
		return nil, nil, nil, err
	}

	if err := am.Unlock(passphrase); err != nil {
		glog.Errorf("Error unlocking Ethereum account: %v", err)
		return nil, nil, nil, err
	}

	maxTxReplacements := 0
	tm := eth.NewTransactionManager(backend, gpm, am, ethTxTimeout, maxTxReplacements)
	go tm.Start()

	ethCfg := eth.LivepeerEthClientConfig{
		AccountManager:     am,
		ControllerAddr:     ethcommon.HexToAddress(cfg.EthController),
		EthClient:          backend,
		GasPriceMonitor:    gpm,
		TransactionManager: tm,
		Signer:             types.LatestSignerForChainID(chainID),
		CheckTxTimeout:     time.Duration(int64(ethTxTimeout) * int64(maxTxReplacements+1)),
	}

	lpEth, err := eth.NewClient(ethCfg)
	if err != nil {
		glog.Errorf("Failed to create Livepeer Ethereum client: %v", err)
		return nil, nil, nil, err
	}

	if err := lpEth.SetGasInfo(0); err != nil {
		glog.Errorf("Failed to set gas info on Livepeer Ethereum Client: %v", err)
		return nil, nil, nil, err
	}

	return lpEth, gpm, tm, nil
}

func (d *Devtool) FundBroadcaster() error {
	amount := new(big.Int).Mul(big.NewInt(100), new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil))

	glog.Infof("Funding deposit with %v", amount)
	glog.Infof("Funding reserve with %v", amount)

	tx, err := d.Client.FundDepositAndReserve(amount, amount)
	if err != nil {
		glog.Error(err)
		return err
	}
	if err := d.Client.CheckTx(tx); err != nil {
		glog.Error(err)
		return err
	}

	glog.Info("Done funding deposit and reserve")
	return nil
}

func (d *Devtool) InitializeOrchestrator(cfg DevtoolConfig) error {
	if err := d.RequestTokens(); err != nil {
		return err
	}
	if err := d.InitializeRound(); err != nil {
		return err
	}
	return d.RegisterOrchestrator(cfg)
}

func (d *Devtool) RequestTokens() error {
	glog.Infof("Requesting tokens from faucet")

	tx, err := d.Client.Request()
	if err != nil {
		glog.Errorf("Error requesting tokens from faucet: %v", err)
		return err
	}

	err = d.Client.CheckTx(tx)
	if err != nil {
		glog.Errorf("Error requesting tokens from faucet: %v", err)
		return err
	}
	glog.Info("Done requesting tokens.")
	return nil
}

func (d *Devtool) InitializeRound() error {
	// XXX TODO curl -X "POST" http://localhost:$transcoderCliPort/initializeRound
	time.Sleep(7 * time.Second)
	for {
		currentRound, err := d.Client.CurrentRound()
		if err != nil {
			glog.Errorf("Error getting current round: %v", err)
			return err
		}
		if currentRound.Int64() > 1 {
			break
		}
		// first round is initialized and locked, need to wait
		glog.Info("Waiting will first round ended.")
		time.Sleep(4 * time.Second)
	}
	tx, err := d.Client.InitializeRound()
	// ErrRoundInitialized
	if err != nil {
		if err.Error() != "ErrRoundInitialized" {
			glog.Errorf("Error initializing round: %v", err)
			return err
		}
	} else {
		err = d.Client.CheckTx(tx)
		if err != nil {
			glog.Errorf("Error initializng round: %v", err)
			return err
		}
	}
	glog.Info("Done initializing round.")
	return nil
}

func (d *Devtool) RegisterOrchestrator(cfg DevtoolConfig) error {
	glog.Info("Activating orchestrator")
	// curl -d "blockRewardCut=10&feeShare=5&amount=500" --data-urlencode "serviceURI=https://$transcoderServiceAddr" \
	//   -H "Content-Type: application/x-www-form-urlencoded" \
	//   -X "POST" http://localhost:$transcoderCliPort/activateTranscoder\
	var amount *big.Int = big.NewInt(int64(500))
	glog.Infof("Bonding %v to %s", amount, cfg.Account)

	tx, err := d.Client.Bond(amount, ethcommon.HexToAddress(cfg.Account))
	if err != nil {
		glog.Error(err)
		return err
	}

	err = d.Client.CheckTx(tx)
	if err != nil {
		glog.Error("=== Bonding failed")
		glog.Error(err)
		return err
	}
	glog.Infof("Registering transcoder %v", cfg.Account)

	tx, err = d.Client.Transcoder(eth.FromPerc(10), eth.FromPerc(5))
	if err == eth.ErrCurrentRoundLocked {
		// TODO: wait for next round and retry
		fmt.Println("unimplemented: wait for next round and retry")
	}
	if err != nil {
		glog.Error(err)
		return err
	}

	err = d.Client.CheckTx(tx)
	if err != nil {
		glog.Error(err)
		return err
	}

	glog.Infof("Storing service URI %v in service registry...", cfg.ServiceURI)

	tx, err = d.Client.SetServiceURI(cfg.ServiceURI)
	if err != nil {
		glog.Error(err)
		return err
	}

	err = d.Client.CheckTx(tx)
	if err != nil {
		glog.Error(err)
		return err
	}

	glog.Info("Done activating orchestrator.")
	return nil
}

func (d *Devtool) Close() {
	d.gpm.Stop()
	d.tm.Stop()
}

func CreateKey(keystoreDir string) string {
	keyStore := keystore.NewKeyStore(keystoreDir, keystore.StandardScryptN, keystore.StandardScryptP)

	account, err := keyStore.NewAccount(passphrase)
	if err != nil {
		glog.Exit(err)
	}
	glog.Infof("Using ETH account: %v", account.Address.Hex())
	return account.Address.Hex()
}
