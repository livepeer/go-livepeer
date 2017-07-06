package eth

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/golang/glog"
	"github.com/livepeer/golp/eth/contracts"
)

type LibraryType uint8

const (
	Node LibraryType = iota
	MaxHeap
	MinHeap
	TranscoderPools
	TranscodeJobs
	MerkleProof
	ECVerify
	SafeMath
)

func deployLibrary(transactOpts *bind.TransactOpts, backend *ethclient.Client, name LibraryType, libraries map[string]common.Address) (common.Address, *types.Transaction, error) {
	var (
		addr common.Address
		tx   *types.Transaction
		err  error
	)

	switch name {
	case Node:
		addr, tx, _, err = contracts.DeployNode(transactOpts, backend, libraries)
		glog.Infof("Deploying Node at %v", addr.Hex())
	case MaxHeap:
		addr, tx, _, err = contracts.DeployMaxHeap(transactOpts, backend, libraries)
		glog.Infof("Deploying MaxHeap at %v", addr.Hex())
	case MinHeap:
		addr, tx, _, err = contracts.DeployMinHeap(transactOpts, backend, libraries)
		glog.Infof("Deploying MinHeap at %v", addr.Hex())
	case TranscoderPools:
		addr, tx, _, err = contracts.DeployTranscoderPools(transactOpts, backend, libraries)
		glog.Infof("Deploying TranscoderPools at %v", addr.Hex())
	case TranscodeJobs:
		addr, tx, _, err = contracts.DeployTranscodeJobs(transactOpts, backend, libraries)
		glog.Infof("Deploying TranscodeJobs at %v", addr.Hex())
	case MerkleProof:
		addr, tx, _, err = contracts.DeployMerkleProof(transactOpts, backend, libraries)
		glog.Infof("Deploying MerkleProof at %v", addr.Hex())
	case ECVerify:
		addr, tx, _, err = contracts.DeployECVerify(transactOpts, backend, libraries)
		glog.Infof("Deploying ECVerify at %v", addr.Hex())
	case SafeMath:
		addr, tx, _, err = contracts.DeploySafeMath(transactOpts, backend, libraries)
		glog.Infof("Deploying SafeMath at %v", addr.Hex())
	default:
		err = fmt.Errorf("Invalid library type: %v", name)

		glog.Errorf(err.Error())
		return common.Address{}, nil, err
	}

	if err != nil {
		glog.Errorf("Error deploying library: %v", err)
		return common.Address{}, nil, err
	}

	return addr, tx, nil
}

func deployLivepeerProtocol(transactOpts *bind.TransactOpts, backend *ethclient.Client, libraries map[string]common.Address, n uint64, roundLength *big.Int, cyclesPerRound *big.Int) (common.Address, *types.Transaction, error) {
	addr, tx, _, err := contracts.DeployLivepeerProtocol(transactOpts, backend, libraries, n, roundLength, cyclesPerRound)

	glog.Infof("Deploying LivepeerProtocol at %v", addr.Hex())

	if err != nil {
		glog.Errorf("Error deploying LivepeerProtocol: %v", err)
		return common.Address{}, nil, err
	}

	return addr, tx, nil
}

func waitForMinedTx(backend *ethclient.Client, rpcTimeout time.Duration, minedTxTimeout int, txHash common.Hash) (*types.Receipt, error) {
	var (
		receipt *types.Receipt
		ctx     context.Context
		err     error
	)

	for i := 0; i < minedTxTimeout; i++ {
		ctx, _ = context.WithTimeout(context.Background(), rpcTimeout)

		receipt, err = backend.TransactionReceipt(ctx, txHash)

		if err != nil && err != ethereum.NotFound {
			return nil, err
		}

		if receipt != nil {
			break
		}

		time.Sleep(time.Second)
	}

	return receipt, nil
}

func nextBlockMultiple(blockNum *big.Int, blockMultiple *big.Int) *big.Int {
	if blockMultiple.Cmp(big.NewInt(0)) == 0 {
		return blockNum
	}

	remainder := new(big.Int).Mod(blockNum, blockMultiple)

	if remainder.Cmp(big.NewInt(0)) == 0 {
		return blockNum
	}

	return new(big.Int).Sub(blockNum.Add(blockNum, blockMultiple), remainder)
}

func waitUntilNextRound(backend *ethclient.Client, rpcTimeout time.Duration, roundLength *big.Int) error {
	ctx, _ := context.WithTimeout(context.Background(), rpcTimeout)
	block, err := backend.BlockByNumber(ctx, nil)

	if err != nil {
		return err
	}

	targetBlockNum := nextBlockMultiple(block.Number(), roundLength)

	glog.Infof("Waiting until next round at block %v...", targetBlockNum)

	for block.Number().Cmp(targetBlockNum) == -1 {
		ctx, _ = context.WithTimeout(context.Background(), rpcTimeout)
		block, err = backend.BlockByNumber(ctx, nil)

		if err != nil {
			return err
		}
	}

	return nil
}
