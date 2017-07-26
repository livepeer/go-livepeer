package eth

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"time"

	"strings"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/golang/glog"
	"github.com/livepeer/golp/eth/contracts"
)

var ErrParseJobTxData = errors.New("ErrParseJobTxData")

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

func CheckRoundAndInit(client LivepeerEthClient, rpcTimeout time.Duration, minedTxTimeout time.Duration) error {
	ok, err := client.CurrentRoundInitialized()

	if err != nil {
		return fmt.Errorf("Client failed CurrentRoundInitialized: %v", err)
	}

	if !ok {
		tx, err := client.InitializeRound()

		if err != nil {
			return fmt.Errorf("Client failed InitializeRound: %v", err)
		}

		_, err = WaitForMinedTx(client.Backend(), rpcTimeout, minedTxTimeout, tx.Hash())

		if err != nil {
			return fmt.Errorf("%v", err)
		}
	}
	return nil
}

func WaitForMinedTx(backend *ethclient.Client, rpcTimeout time.Duration, minedTxTimeout time.Duration, txHash common.Hash) (*types.Receipt, error) {
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

	return receipt, nil
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

func SignSegmentHash(c *Client, passphrase string, hash []byte) ([]byte, error) {
	sig, err := c.keyStore.SignHashWithPassphrase(c.account, passphrase, hash)

	if err != nil {
		glog.Errorf("Error signing segment: %v", err)
		return nil, err
	}

	// glog.Infof("[%v] Created signed segment hash %v", c.account.Address.Hex(), common.ToHex(sig))
	return sig, nil
}

func ParseJobTxData(d []byte) (strmID string, data string, err error) {
	data = strings.Trim(string(d[56:88]), "\x00")
	strmID = string(d[132:264])
	if data == "" || strmID == "" {
		return "", "", ErrParseJobTxData
	}
	return strmID, data, nil
}

//XXX: This is a total hack.  Should be getting the info from the event log.
func GetInfoFromJobEvent(l types.Log, c LivepeerEthClient) (jobID *big.Int, strmID string, bAddr common.Address, tAddr common.Address, err error) {
	var jidR *big.Int
	var tAddrR common.Address
	var bAddrR common.Address
	for i := 0; i < 1000; i++ {
		jid, _, _, bAddr, tAddr, _, err := c.JobDetails(big.NewInt(int64(i)))

		if err != nil {
			break
		} else {
			jidR = jid
			bAddrR = bAddr
			tAddrR = tAddr
		}
	}

	tx, _, err := c.Backend().TransactionByHash(context.Background(), l.TxHash)
	if err != nil {
		glog.Errorf("Error getting transaction data: %v", err)
		return nil, "", common.Address{}, common.Address{}, fmt.Errorf("JobEventError")
	}

	strmID, _, err = ParseJobTxData(tx.Data())
	if err != nil {
		glog.Errorf("Error parsing transaction data: %v", err)
		return nil, "", common.Address{}, common.Address{}, fmt.Errorf("JobEventError")
	}

	return jidR, strmID, bAddrR, tAddrR, nil
}
