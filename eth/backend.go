package eth

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"sync"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/params"
	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/eth/contracts"
)

var abis = []string{
	contracts.BondingManagerABI,
	contracts.ControllerABI,
	contracts.LivepeerTokenABI,
	contracts.LivepeerTokenFaucetABI,
	contracts.MinterABI,
	contracts.RoundsManagerABI,
	contracts.ServiceRegistryABI,
	contracts.TicketBrokerABI,
	contracts.PollABI,
}

var abiMap = makeABIMap()

type Backend interface {
	ethereum.ChainStateReader
	ethereum.TransactionReader
	ethereum.TransactionSender
	ethereum.ContractCaller
	ethereum.PendingContractCaller
	ethereum.PendingStateReader
	ethereum.GasEstimator
	ethereum.GasPricer
	ethereum.LogFilterer
	ethereum.ChainReader
	ChainID(ctx context.Context) (*big.Int, error)
	GasPriceMonitor() *GasPriceMonitor
	SuggestGasTipCap(context.Context) (*big.Int, error)
}

type backend struct {
	*ethclient.Client
	nonceManager *NonceManager
	signer       types.Signer
	gpm          *GasPriceMonitor
	tm           *TransactionManager

	sync.RWMutex
}

func NewBackend(client *ethclient.Client, signer types.Signer, gpm *GasPriceMonitor, tm *TransactionManager) Backend {
	return &backend{
		Client:       client,
		nonceManager: NewNonceManager(client),
		signer:       signer,
		gpm:          gpm,
		tm:           tm,
	}
}

func (b *backend) PendingNonceAt(ctx context.Context, account common.Address) (uint64, error) {
	b.nonceManager.Lock(account)
	defer b.nonceManager.Unlock(account)

	return b.nonceManager.Next(account)
}

func (b *backend) SendTransaction(ctx context.Context, tx *types.Transaction) error {
	// Use the transaction manager instead of the ethereum client
	if err := b.tm.SendTransaction(ctx, tx); err != nil {
		return err
	}

	sender, err := types.Sender(b.signer, tx)
	if err != nil {
		return err
	}

	// update local nonce
	b.nonceManager.Lock(sender)
	b.nonceManager.Update(sender, tx.Nonce())
	b.nonceManager.Unlock(sender)

	return nil
}

func (b *backend) SuggestGasPrice(ctx context.Context) (*big.Int, error) {

	// Use the gas price monitor instead of the ethereum client to suggest a gas price
	gp := b.gpm.GasPrice()
	maxGp := b.gpm.MaxGasPrice()

	if maxGp != nil && gp.Cmp(maxGp) > 0 {
		return nil, fmt.Errorf("current gas price exceeds maximum gas price max=%v GWei current=%v GWei",
			FromWei(maxGp, params.GWei),
			FromWei(gp, params.GWei),
		)
	}

	return gp, nil
}

func (b *backend) SuggestGasTipCap(ctx context.Context) (*big.Int, error) {
	// This runs the max gas price check against the value returned by eth_gasPrice which should be priority fee + base fee.
	// We may use the returned value later on to derive the priority fee if the eth_maxPriorityFeePerGas method is not supported.
	gasPrice, err := b.SuggestGasPrice(ctx)
	if err != nil {
		return nil, err
	}

	tip, err := b.Client.SuggestGasTipCap(ctx)
	if err != nil {
		// SuggestGasTipCap() uses the eth_maxPriorityFeePerGas RPC call under the hood which
		// is not a part of the ETH JSON-RPC spec.
		// In the future, eth_maxPriorityFeePerGas can be replaced with an eth_feeHistory based algorithm (see https://github.com/ethereum/go-ethereum/issues/23479).
		// For now, if the provider does not support eth_maxPriorityFeePerGas (i.e. not geth), then we calculate the priority fee as
		// eth_gasPrice - baseFee.
		head, err := b.HeaderByNumber(ctx, nil)
		if err != nil {
			return nil, err
		}
		if head.BaseFee == nil {
			return nil, errors.New("missing base fee")
		}

		tip = new(big.Int).Sub(gasPrice, head.BaseFee)
	}

	return tip, nil
}

func (b *backend) GasPriceMonitor() *GasPriceMonitor {
	return b.gpm
}

type txLog struct {
	method string
	inputs string
}

func (b *backend) CallContract(ctx context.Context, msg ethereum.CallMsg, blockNumber *big.Int) ([]byte, error) {
	return b.retryRemoteCall(func() ([]byte, error) {
		return b.Client.CallContract(ctx, msg, blockNumber)
	})
}

func (b *backend) PendingCallContract(ctx context.Context, msg ethereum.CallMsg) ([]byte, error) {
	return b.retryRemoteCall(func() ([]byte, error) {
		return b.Client.PendingCallContract(ctx, msg)
	})
}

func (b *backend) retryRemoteCall(remoteCall func() ([]byte, error)) (out []byte, err error) {
	count := 3    // consider making this a package-level global constant
	retry := true // consider making this a package-level global constant

	for i := 0; i < count && retry; i++ {
		out, err = remoteCall()
		if err != nil && (err.Error() == "EOF" || err.Error() == "tls: use of closed connection") {
			glog.V(4).Infof("Retrying call to remote ethereum node")
		} else {
			retry = false
		}
	}

	return out, err
}

func makeABIMap() map[string]*abi.ABI {
	abiMap := make(map[string]*abi.ABI)

	for _, ABI := range abis {
		parsedAbi, err := abi.JSON(strings.NewReader(ABI))
		if err != nil {
			glog.Errorf("Error creating ABI map err=%q", err)
			return map[string]*abi.ABI{}
		}
		for _, m := range parsedAbi.Methods {
			abiMap[string(m.ID)] = &parsedAbi
		}
	}

	return abiMap
}

func newTxLog(tx *types.Transaction) (txLog, error) {
	var txParamsString string
	data := tx.Data()
	if len(data) < 4 {
		return txLog{}, errors.New("no method signature")
	}
	methodSig := data[:4]
	abi, ok := abiMap[string(methodSig)]
	if !ok {
		return txLog{}, errors.New("unknown ABI")
	}
	method, err := abi.MethodById(methodSig)
	if err != nil {
		return txLog{}, err
	}
	txParams := make(map[string]interface{})
	if err := decodeTxParams(abiMap[string(methodSig)], txParams, data); err != nil {
		return txLog{}, err
	}

	for _, arg := range method.Inputs {
		txParamsString += fmt.Sprintf("%v: %v  ", arg.Name, txParams[arg.Name])
	}
	return txLog{
		method: method.Name,
		inputs: strings.TrimSpace(txParamsString),
	}, nil
}
