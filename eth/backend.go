package eth

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"strings"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
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
}

type backend struct {
	*ethclient.Client
	abiMap       map[string]*abi.ABI
	nonceManager *NonceManager
	signer       types.Signer
}

func NewBackend(client *ethclient.Client, signer types.Signer) (Backend, error) {
	abiMap, err := makeABIMap()
	if err != nil {
		return nil, err
	}

	return &backend{
		client,
		abiMap,
		NewNonceManager(client),
		signer,
	}, nil
}

func (b *backend) PendingNonceAt(ctx context.Context, account common.Address) (uint64, error) {
	b.nonceManager.Lock(account)
	defer b.nonceManager.Unlock(account)

	return b.nonceManager.Next(account)
}

func (b *backend) SendTransaction(ctx context.Context, tx *types.Transaction) error {
	sendErr := b.Client.SendTransaction(ctx, tx)

	// update local nonce
	msg, err := tx.AsMessage(b.signer)
	if err != nil {
		return err
	}
	sender := msg.From()
	b.nonceManager.Lock(sender)
	b.nonceManager.Update(sender, tx.Nonce())
	b.nonceManager.Unlock(sender)

	txLog, err := b.newTxLog(tx)
	if err != nil {
		txLog.method = "unknown"
	}

	if sendErr != nil {
		glog.Infof("\n%vEth Transaction%v\n\nInvoking transaction: \"%v\". Inputs: \"%v\"   \nTransaction Failed: %v\n\n%v\n", strings.Repeat("*", 30), strings.Repeat("*", 30), txLog.method, txLog.inputs, sendErr, strings.Repeat("*", 75))
		return sendErr
	}

	glog.Infof("\n%vEth Transaction%v\n\nInvoking transaction: \"%v\". Inputs: \"%v\"  Hash: \"%v\". \n\n%v\n", strings.Repeat("*", 30), strings.Repeat("*", 30), txLog.method, txLog.inputs, tx.Hash().String(), strings.Repeat("*", 75))

	return nil
}

type txLog struct {
	method string
	inputs string
}

func (b *backend) newTxLog(tx *types.Transaction) (txLog, error) {
	var txParamsString string
	data := tx.Data()
	if len(data) < 4 {
		return txLog{}, errors.New("no method signature")
	}
	methodSig := data[:4]
	abi, ok := b.abiMap[string(methodSig)]
	if !ok {
		return txLog{}, errors.New("unknown ABI")
	}
	method, err := abi.MethodById(methodSig)
	if err != nil {
		return txLog{}, err
	}
	txParams := make(map[string]interface{})
	if err := decodeTxParams(b.abiMap[string(methodSig)], txParams, data); err != nil {
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

func makeABIMap() (map[string]*abi.ABI, error) {
	abiMap := make(map[string]*abi.ABI)

	for _, ABI := range abis {
		parsedAbi, err := abi.JSON(strings.NewReader(ABI))
		if err != nil {
			return map[string]*abi.ABI{}, err
		}
		for _, m := range parsedAbi.Methods {
			abiMap[string(m.ID())] = &parsedAbi
		}
	}

	return abiMap, nil
}
