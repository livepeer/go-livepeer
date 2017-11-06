package eth

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/golang/glog"
)

// type LibraryType uint8

// const (
// 	Node LibraryType = iota
// 	MaxHeap
// 	MinHeap
// 	TranscoderPools
// 	JobLib
// 	MerkleProof
// 	ECRecovery
// 	SafeMath
// )

// func deployLibrary(transactOpts *bind.TransactOpts, backend *ethclient.Client, name LibraryType, libraries map[string]common.Address) (common.Address, *types.Transaction, error) {
// 	var (
// 		addr common.Address
// 		tx   *types.Transaction
// 		err  error
// 	)

// 	switch name {
// 	case Node:
// 		addr, tx, _, err = contracts.DeployNode(transactOpts, backend, libraries)
// 		glog.Infof("Deploying Node at %v", addr.Hex())
// 	case MaxHeap:
// 		addr, tx, _, err = contracts.DeployMaxHeap(transactOpts, backend, libraries)
// 		glog.Infof("Deploying MaxHeap at %v", addr.Hex())
// 	case MinHeap:
// 		addr, tx, _, err = contracts.DeployMinHeap(transactOpts, backend, libraries)
// 		glog.Infof("Deploying MinHeap at %v", addr.Hex())
// 	case TranscoderPools:
// 		addr, tx, _, err = contracts.DeployTranscoderPools(transactOpts, backend, libraries)
// 		glog.Infof("Deploying TranscoderPools at %v", addr.Hex())
// 	case JobLib:
// 		addr, tx, _, err = contracts.DeployJobLib(transactOpts, backend, libraries)
// 		glog.Infof("Deploying JobLib at %v", addr.Hex())
// 	case MerkleProof:
// 		addr, tx, _, err = contracts.DeployMerkleProof(transactOpts, backend, libraries)
// 		glog.Infof("Deploying MerkleProof at %v", addr.Hex())
// 	case ECRecovery:
// 		addr, tx, _, err = contracts.DeployECRecovery(transactOpts, backend, libraries)
// 		glog.Infof("Deploying ECRecovery at %v", addr.Hex())
// 	case SafeMath:
// 		addr, tx, _, err = contracts.DeploySafeMath(transactOpts, backend, libraries)
// 		glog.Infof("Deploying SafeMath at %v", addr.Hex())
// 	default:
// 		err = fmt.Errorf("Invalid library type: %v", name)

// 		glog.Errorf(err.Error())
// 		return common.Address{}, nil, err
// 	}

// 	if err != nil {
// 		glog.Errorf("Error deploying library: %v", err)
// 		return common.Address{}, nil, err
// 	}

// 	return addr, tx, nil
// }

// func deployIdentityVerifier(transactOpts *bind.TransactOpts, backend *ethclient.Client) (common.Address, *types.Transaction, error) {
// 	addr, tx, _, err := contracts.DeployIdentityVerifier(transactOpts, backend, nil)

// 	glog.Infof("Deploying IdentityVerifier at %v", addr.Hex())

// 	if err != nil {
// 		glog.Errorf("Error deploying IdentityVerifier: %v", err)
// 		return common.Address{}, nil, err
// 	}

// 	return addr, tx, nil
// }

// func deployLivepeerToken(transactOpts *bind.TransactOpts, backend *ethclient.Client) (common.Address, *types.Transaction, error) {
// 	addr, tx, _, err := contracts.DeployLivepeerToken(transactOpts, backend, nil)

// 	glog.Infof("Deploying LivepeerToken at %v", addr.Hex())

// 	if err != nil {
// 		glog.Errorf("Error deploying LivepeerToken: %v", err)
// 		return common.Address{}, nil, err
// 	}

// 	return addr, tx, nil
// }

// func deployLivepeerProtocol(transactOpts *bind.TransactOpts, backend *ethclient.Client) (common.Address, *types.Transaction, error) {
// 	addr, tx, _, err := contracts.DeployLivepeerProtocol(transactOpts, backend, nil)

// 	glog.Infof("Deploying LivepeerProtocol at %v", addr.Hex())

// 	if err != nil {
// 		glog.Errorf("Error deploying LivepeerProtocol: %v", err)
// 		return common.Address{}, nil, err
// 	}

// 	return addr, tx, nil
// }

// func deployBondingManager(transactOpts *bind.TransactOpts, backend *ethclient.Client, libraries map[string]common.Address, registry common.Address, token common.Address, numActiveTranscoders *big.Int, unbondingPeriod uint64) (common.Address, *types.Transaction, error) {
// 	addr, tx, _, err := contracts.DeployBondingManager(transactOpts, backend, libraries, registry, token, numActiveTranscoders, unbondingPeriod)

// 	glog.Infof("Deploying BondingManager at %v", addr.Hex())

// 	if err != nil {
// 		glog.Errorf("Error deploying BondingManager: %v", err)
// 		return common.Address{}, nil, err
// 	}

// 	return addr, tx, nil
// }

// func deployJobsManager(transactOpts *bind.TransactOpts, backend *ethclient.Client, libraries map[string]common.Address, registry common.Address, token common.Address, verifier common.Address, verificationRate uint64, jobEndingPeriod *big.Int, verificationPeriod *big.Int, slashingPeriod *big.Int, failedVerificationSlashAmount uint64, missedVerificationSlashAmount uint64, finderFee uint64) (common.Address, *types.Transaction, error) {
// 	addr, tx, _, err := contracts.DeployJobsManager(transactOpts, backend, libraries, registry, token, verifier, verificationRate, jobEndingPeriod, verificationPeriod, slashingPeriod, failedVerificationSlashAmount, missedVerificationSlashAmount, finderFee)

// 	glog.Infof("Deploying JobsManager at %v", addr.Hex())

// 	if err != nil {
// 		glog.Infof("Error deploying JobsManager: %v", err)
// 		return common.Address{}, nil, err
// 	}

// 	return addr, tx, nil
// }

// func deployRoundsManager(transactOpts *bind.TransactOpts, backend *ethclient.Client, libraries map[string]common.Address, registry common.Address, blockTime *big.Int, roundLength *big.Int) (common.Address, *types.Transaction, error) {
// 	addr, tx, _, err := contracts.DeployRoundsManager(transactOpts, backend, libraries, registry, blockTime, roundLength)

// 	glog.Infof("Deploying RoundsManager at %v", addr.Hex())

// 	if err != nil {
// 		glog.Errorf("Error deploying RoundsManager: %v", err)
// 		return common.Address{}, nil, err
// 	}

// 	return addr, tx, nil
// }

// func initProtocol(transactOpts *bind.TransactOpts, backend *ethclient.Client, rpcTimeout time.Duration, minedTxTimeout time.Duration, protocolAddr common.Address, tokenAddr common.Address, bondingManagerAddr common.Address, jobsManagerAddr common.Address, roundsManagerAddr common.Address) error {
// 	protocol, err := contracts.NewLivepeerProtocol(protocolAddr, backend)

// 	if err != nil {
// 		glog.Errorf("Error creating LivepeerProtocol: %v", err)
// 		return err
// 	}

// 	tx, err := protocol.SetContract(transactOpts, crypto.Keccak256Hash([]byte("BondingManager")), bondingManagerAddr)

// 	if err != nil {
// 		glog.Errorf("Error adding BondingManager to registry: %v", err)
// 		return err
// 	}

// 	_, err = WaitForMinedTx(backend, rpcTimeout, minedTxTimeout, tx.Hash(), tx.Gas())

// 	if err != nil {
// 		glog.Errorf("Error waiting for mined SetContract tx: %v", err)
// 		return err
// 	}

// 	tx, err = protocol.SetContract(transactOpts, crypto.Keccak256Hash([]byte("JobsManager")), jobsManagerAddr)

// 	if err != nil {
// 		glog.Errorf("Error adding JobsManager to registry: %v", err)
// 		return err
// 	}

// 	_, err = WaitForMinedTx(backend, rpcTimeout, minedTxTimeout, tx.Hash(), tx.Gas())

// 	if err != nil {
// 		glog.Errorf("Error waiting for mined SetContract tx: %v", err)
// 		return err
// 	}

// 	tx, err = protocol.SetContract(transactOpts, crypto.Keccak256Hash([]byte("RoundsManager")), roundsManagerAddr)

// 	if err != nil {
// 		glog.Errorf("Error adding RoundsManager to registry: %v", err)
// 		return err
// 	}

// 	_, err = WaitForMinedTx(backend, rpcTimeout, minedTxTimeout, tx.Hash(), tx.Gas())

// 	if err != nil {
// 		glog.Errorf("Error waiting for mined SetContract tx: %v", err)
// 		return err
// 	}

// 	token, err := contracts.NewLivepeerToken(tokenAddr, backend)

// 	if err != nil {
// 		glog.Errorf("Error creating LivepeerToken: %v", err)
// 		return err
// 	}

// 	tx, err = token.TransferOwnership(transactOpts, bondingManagerAddr)

// 	if err != nil {
// 		glog.Errorf("Error transfering ownership of LivepeerToken: %v", err)
// 		return err
// 	}

// 	_, err = WaitForMinedTx(backend, rpcTimeout, minedTxTimeout, tx.Hash(), tx.Gas())

// 	if err != nil {
// 		glog.Errorf("Error waiting for TransferOwnership tx: %v", err)
// 		return err
// 	}

// 	tx, err = protocol.Unpause(transactOpts)

// 	if err != nil {
// 		glog.Errorf("Error unpausing LivepeerProtocol: %v", err)
// 		return err
// 	}

// 	_, err = WaitForMinedTx(backend, rpcTimeout, minedTxTimeout, tx.Hash(), tx.Gas())

// 	if err != nil {
// 		glog.Errorf("Error waiting for Unapuse tx: %v", err)
// 		return err
// 	}

// 	return nil
// }

// // To be called by account that deploys LivepeerToken
// func distributeTokens(transactOpts *bind.TransactOpts, backend *ethclient.Client, rpcTimeout time.Duration, minedTxTimeout time.Duration, tokenAddr common.Address, mintAmount *big.Int, accounts []accounts.Account) error {
// 	token, err := contracts.NewLivepeerToken(tokenAddr, backend)

// 	if err != nil {
// 		glog.Errorf("Error creating LivepeerToken: %v", err)
// 	}

// 	var tx *types.Transaction

// 		tx, err = token.Mint(transactOpts, account.Address, mintAmount)
// 		if err != nil {
// 			glog.Errorf("Error minting tokens: %v", err)
// 			return err
// 		}

// 		_, err = WaitForMinedTx(backend, rpcTimeout, minedTxTimeout, tx.Hash(), tx.Gas())
// 		if err != nil {
// 			glog.Errorf("Error waiting for mined mint tx: %v", err)
// 			return err
// 		}
// 	}

// 	return nil
// }

func WaitUntilBlockMultiple(backend *ethclient.Client, rpcTimeout time.Duration, blockMultiple *big.Int) error {
	ctx, _ := context.WithTimeout(context.Background(), rpcTimeout)

	block, err := backend.BlockByNumber(ctx, nil)
	if err != nil {
		return err
	}

	targetBlockNum := NextBlockMultiple(block.Number(), blockMultiple)

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

func Wait(backend *ethclient.Client, rpcTimeout time.Duration, blocks *big.Int) error {
	ctx, _ := context.WithTimeout(context.Background(), rpcTimeout)

	block, err := backend.BlockByNumber(ctx, nil)
	if err != nil {
		return err
	}

	targetBlockNum := new(big.Int).Add(block.Number(), blocks)

	glog.Infof("Waiting %v blocks...", blocks)

	for block.Number().Cmp(targetBlockNum) == -1 {
		ctx, _ = context.WithTimeout(context.Background(), rpcTimeout)

		block, err = backend.BlockByNumber(ctx, nil)
		if err != nil {
			return err
		}
	}

	return nil
}

func CheckRoundAndInit(client LivepeerEthClient) error {
	ok, err := client.CurrentRoundInitialized()
	if err != nil {
		return fmt.Errorf("Client failed CurrentRoundInitialized: %v", err)
	}

	if !ok {
		receiptCh, errCh := client.InitializeRound()
		select {
		case <-receiptCh:
			return nil
		case err := <-errCh:
			return err
		}
	}

	return nil
}

func WaitForMinedTx(backend *ethclient.Client, rpcTimeout time.Duration, minedTxTimeout time.Duration, txHash common.Hash, gas *big.Int) (*types.Receipt, error) {
	var (
		receipt *types.Receipt
		ctx     context.Context
		err     error
	)
	h := common.Hash{}
	if txHash == h {
		return nil, nil
	}

	start := time.Now()
	for time.Since(start) < minedTxTimeout {
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

	if gas.Cmp(receipt.GasUsed) == 0 {
		return receipt, fmt.Errorf("Transaction %v threw", txHash.Hex())
	} else {
		return receipt, nil
	}
}

func NextBlockMultiple(blockNum *big.Int, blockMultiple *big.Int) *big.Int {
	if blockMultiple.Cmp(big.NewInt(0)) == 0 {
		return blockNum
	}

	remainder := new(big.Int).Mod(blockNum, blockMultiple)

	if remainder.Cmp(big.NewInt(0)) == 0 {
		return blockNum
	}

	return new(big.Int).Sub(blockNum.Add(blockNum, blockMultiple), remainder)
}

func IsNullAddress(addr common.Address) bool {
	return addr == common.Address{}
}
