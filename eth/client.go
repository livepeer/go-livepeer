package eth

//go:generate abigen --abi protocol/abi/LivepeerProtocol.abi --pkg contracts --type LivepeerProtocol --out contracts/livepeerProtocol.go --bin protocol/bin/LivepeerProtocol.bin
//go:generate abigen --abi protocol/abi/LivepeerToken.abi --pkg contracts --type LivepeerToken --out contracts/livepeerToken.go --bin protocol/bin/LivepeerToken.bin
//go:generate abigen --abi protocol/abi/TranscoderPools.abi --pkg contracts --type TranscoderPools --out contracts/transcoderPools.go --bin protocol/bin/TranscoderPools.bin
//go:generate abigen --abi protocol/abi/TranscodeJobs.abi --pkg contracts --type TranscodeJobs --out contracts/transcodeJobs.go --bin protocol/bin/TranscodeJobs.bin
//go:generate abigen --abi protocol/abi/MaxHeap.abi --pkg contracts --type MaxHeap --out contracts/maxHeap.go --bin protocol/bin/MaxHeap.bin
//go:generate abigen --abi protocol/abi/MinHeap.abi --pkg contracts --type MinHeap --out contracts/minHeap.go --bin protocol/bin/MinHeap.bin
//go:generate abigen --abi protocol/abi/Node.abi --pkg contracts --type Node --out contracts/node.go --bin protocol/bin/Node.bin
//go:generate abigen --abi protocol/abi/SafeMath.abi --pkg contracts --type SafeMath --out contracts/safeMath.go --bin protocol/bin/SafeMath.bin
//go:generate abigen --abi protocol/abi/ECVerify.abi --pkg contracts --type ECVerify --out contracts/ecVerify.go --bin protocol/bin/ECVerify.bin
//go:generate abigen --abi protocol/abi/MerkleProof.abi --pkg contracts --type MerkleProof --out contracts/merkleProof.go --bin protocol/bin/MerkleProof.bin

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/golang/glog"
	"github.com/livepeer/golp/eth/contracts"
)

type Client struct {
	protocolSession *contracts.LivepeerProtocolSession
	tokenSession    *contracts.LivepeerTokenSession
	backend         *ethclient.Client

	addr         common.Address
	protocolAddr common.Address
	tokenAddr    common.Address
	rpcTimeout   time.Duration
	eventTimeout time.Duration
}

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

func NewClient(transactOpts *bind.TransactOpts, backend *ethclient.Client, protocolAddr common.Address, rpcTimeout time.Duration, eventTimeout time.Duration) (*Client, error) {
	protocol, err := contracts.NewLivepeerProtocol(protocolAddr, backend)

	if err != nil {
		glog.Error("Error creating LivepeerProtocol: %v", err)
		return nil, err
	}

	tokenAddr, err := protocol.Token(nil)

	if err != nil {
		glog.Errorf("Error retrieving token address: %v", err)
		return nil, err
	}

	token, err := contracts.NewLivepeerToken(tokenAddr, backend)

	if err != nil {
		glog.Errorf("Error creating LivepeerToken: %v", err)
		return nil, err
	}

	glog.Infof("Creating client for account %v", transactOpts.From.Hex())

	return &Client{
		&contracts.LivepeerProtocolSession{
			Contract:     protocol,
			TransactOpts: *transactOpts,
		},
		&contracts.LivepeerTokenSession{
			Contract:     token,
			TransactOpts: *transactOpts,
		},
		backend,
		transactOpts.From,
		protocolAddr,
		tokenAddr,
		rpcTimeout,
		eventTimeout,
	}, nil
}

func DeployLibrary(transactOpts *bind.TransactOpts, backend *ethclient.Client, name LibraryType, libraries map[string]common.Address) (common.Address, *types.Transaction, error) {
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

func DeployLivepeerProtocol(transactOpts *bind.TransactOpts, backend *ethclient.Client, libraries map[string]common.Address, n uint64, roundLength *big.Int, cyclesPerRound *big.Int) (common.Address, *types.Transaction, error) {
	addr, tx, _, err := contracts.DeployLivepeerProtocol(transactOpts, backend, libraries, n, roundLength, cyclesPerRound)

	glog.Infof("Deploying LivepeerProtocol at %v", addr.Hex())

	if err != nil {
		glog.Errorf("Error deploying LivepeerProtocol: %v", err)
		return common.Address{}, nil, err
	}

	return addr, tx, nil
}

func (c *Client) WatchEvent(logsCh <-chan types.Log) (types.Log, error) {
	var (
		timer = time.NewTimer(c.eventTimeout)
	)

	for {
		select {
		case log := <-logsCh:
			if !log.Removed {
				return log, nil
			}
		case <-timer.C:
			err := fmt.Errorf("watchEvent timed out")

			glog.Errorf(err.Error())
			return types.Log{}, err
		}
	}
}

func (c *Client) RoundInfo() (*big.Int, *big.Int, *big.Int, *big.Int, error) {
	cr, err := c.protocolSession.CurrentRound()

	if err != nil {
		glog.Errorf("Error getting current round: %v", err)
		return nil, nil, nil, nil, err
	}

	cn, err := c.protocolSession.CycleNum()

	if err != nil {
		glog.Errorf("Error getting current cycle number: %v", err)
		return nil, nil, nil, nil, err
	}

	crsb, err := c.protocolSession.CurrentRoundStartBlock()

	if err != nil {
		glog.Errorf("Error getting current round start block: %v", err)
		return nil, nil, nil, nil, err
	}

	ctx, _ := context.WithTimeout(context.Background(), c.rpcTimeout)

	block, err := c.backend.BlockByNumber(ctx, nil)

	if err != nil {
		glog.Errorf("Error getting latest block number: %v", err)
		return nil, nil, nil, nil, err
	}

	return cr, cn, crsb, block.Number(), nil
}

func (c *Client) InitializeRound() (*types.Transaction, error) {
	tx, err := c.protocolSession.InitializeRound()

	if err != nil {
		glog.Errorf("Error initializing round: %v", err)
		return nil, err
	}

	glog.Infof("[%v] Submitted transaction %v. Initialize round", c.addr.Hex(), tx.Hash().Hex())
	return tx, nil
}

func (c *Client) CurrentRoundInitialized() (bool, error) {
	lir, err := c.protocolSession.LastInitializedRound()

	if err != nil {
		glog.Errorf("Error getting last initialized round: %v", err)
		return false, err
	}

	cr, err := c.protocolSession.CurrentRound()

	if err != nil {
		glog.Errorf("Error getting current round: %v", err)
		return false, err
	}

	if lir.Cmp(cr) == -1 {
		return false, nil
	} else {
		return true, nil
	}
}

func (c *Client) Transcoder(blockRewardCut uint8, feeShare uint8, pricePerSegment *big.Int) (*types.Transaction, error) {
	tx, err := c.protocolSession.Transcoder(blockRewardCut, feeShare, pricePerSegment)

	if err != nil {
		glog.Errorf("Error registering as a transcoder: %v", err)
		return nil, err
	}

	glog.Infof("[%v] Submitted transaction %v. Register as a transcoder", c.addr.Hex(), tx.Hash().Hex())
	return tx, nil
}

func (c *Client) Bond(amount *big.Int, toAddr common.Address) (*types.Transaction, error) {
	var (
		logsCh = make(chan types.Log)
	)

	tokenJson, err := abi.JSON(strings.NewReader(contracts.LivepeerTokenABI))

	if err != nil {
		glog.Errorf("Error decoding ABI into JSON: %v", err)
		return nil, err
	}

	q := ethereum.FilterQuery{
		Addresses: []common.Address{c.tokenAddr},
		Topics:    [][]common.Hash{[]common.Hash{tokenJson.Events["Approval"].Id()}, []common.Hash{common.BytesToHash(c.addr[:])}},
	}

	ctx, _ := context.WithTimeout(context.Background(), c.rpcTimeout)

	logsSub, err := c.backend.SubscribeFilterLogs(ctx, q, logsCh)

	defer logsSub.Unsubscribe()
	defer close(logsCh)

	tx, err := c.tokenSession.Approve(c.protocolAddr, amount)

	if err != nil {
		glog.Errorf("Error approving token transfer: %v", err)
		return nil, err
	}

	glog.Infof("[%v] Submitted transaction %v. Approve %v LPTU transfer by LivepeerProtocol", c.addr.Hex(), tx.Hash().Hex(), amount)

	_, err = c.WatchEvent(logsCh)

	if err != nil {
		glog.Errorf("Error watching event: %v", err)
		return nil, err
	}

	tx, err = c.protocolSession.Bond(amount, toAddr)

	if err != nil {
		glog.Errorf("Error bonding: %v", err)
		return nil, err
	}

	cr, cn, crsb, cb, err := c.RoundInfo()

	if err != nil {
		glog.Errorf("Error getting round info: %v", err)
		return nil, err
	}

	glog.Infof("[%v] Submitted transaction %v. Bond %v LPTU to %v at CR %v CN %v CRSB %v CB %v", c.addr.Hex(), tx.Hash().Hex(), amount, toAddr.Hex(), cr, cn, crsb, cb)
	return tx, nil
}

func (c *Client) ValidRewardTimeWindow() (bool, error) {
	return c.protocolSession.ValidRewardTimeWindow(c.addr)
}

func (c *Client) Reward() (*types.Transaction, error) {
	tx, err := c.protocolSession.Reward()

	if err != nil {
		glog.Errorf("Error calling reward: %v", err)
		return nil, err
	}

	cr, cn, crsb, cb, err := c.RoundInfo()

	if err != nil {
		glog.Errorf("Error getting round info: %v", err)
		return nil, err
	}

	glog.Infof("[%v] Submitted transaction %v. Reward at CR %v CN %v CRSB %v CB %v", c.addr.Hex(), tx.Hash().Hex(), cr, cn, crsb, cb)
	return tx, nil
}

func (c *Client) Transfer(toAddr common.Address, amount *big.Int) (*types.Transaction, error) {
	tx, err := c.tokenSession.Transfer(toAddr, amount)

	if err != nil {
		glog.Errorf("Error transferring tokens: %v", err)
		return nil, err
	}

	glog.Infof("[%v] Submitted transaction %v. Transfer %v LPTU to %v", c.addr.Hex(), tx.Hash().Hex(), amount, toAddr.Hex())
	return tx, nil
}
