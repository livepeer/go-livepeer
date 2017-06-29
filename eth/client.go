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
	"github.com/livepeer/libp2p-livepeer/eth/contracts"
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
	case MaxHeap:
		addr, tx, _, err = contracts.DeployMaxHeap(transactOpts, backend, libraries)
	case MinHeap:
		addr, tx, _, err = contracts.DeployMinHeap(transactOpts, backend, libraries)
	case TranscoderPools:
		addr, tx, _, err = contracts.DeployTranscoderPools(transactOpts, backend, libraries)
	case TranscodeJobs:
		addr, tx, _, err = contracts.DeployTranscodeJobs(transactOpts, backend, libraries)
	case MerkleProof:
		addr, tx, _, err = contracts.DeployMerkleProof(transactOpts, backend, libraries)
	case ECVerify:
		addr, tx, _, err = contracts.DeployECVerify(transactOpts, backend, libraries)
	case SafeMath:
		addr, tx, _, err = contracts.DeploySafeMath(transactOpts, backend, libraries)
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

	if err != nil {
		glog.Errorf("Error deploying LivepeerProtocol: %v", err)
		return common.Address{}, nil, err
	}

	return addr, tx, nil
}

func (c *Client) SubscribeToEvent(contractAddr common.Address, eventHash common.Hash, logsCh chan<- types.Log) (ethereum.Subscription, error) {
	q := ethereum.FilterQuery{
		Addresses: []common.Address{contractAddr},
		Topics:    [][]common.Hash{[]common.Hash{eventHash}},
	}

	ctx, _ := context.WithTimeout(context.Background(), c.rpcTimeout)

	logsSub, err := c.backend.SubscribeFilterLogs(ctx, q, logsCh)

	if err != nil {
		glog.Errorf("Error subscribing to filter logs: %v", err)
		return nil, err
	}

	return logsSub, nil
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

func (c *Client) RoundInfo() (*big.Int, *big.Int, *big.Int, error) {
	cr, err := c.protocolSession.CurrentRound()

	if err != nil {
		glog.Errorf("Error getting current round: %v", err)
		return nil, nil, nil, err
	}

	cn, err := c.protocolSession.CycleNum()

	if err != nil {
		glog.Errorf("Error getting current cycle number: %v", err)
		return nil, nil, nil, err
	}

	crsb, err := c.protocolSession.CurrentRoundStartBlock()

	if err != nil {
		glog.Errorf("Error getting current round start block: %v", err)
		return nil, nil, nil, err
	}

	ctx, _ := context.WithTimeout(context.Background(), c.rpcTimeout)

	block, err := c.backend.BlockByNumber(ctx, nil)

	if err != nil {
		glog.Errorf("Error getting latest block number: %v", err)
		return nil, nil, nil, err
	}

	return cr, cn, new(big.Int).Sub(block.Number(), crsb), nil
}

func (c *Client) InitializeRound() (*types.Transaction, error) {
	return c.protocolSession.InitializeRound()
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
	return c.protocolSession.Transcoder(blockRewardCut, feeShare, pricePerSegment)
}

func (c *Client) Bond(amount *big.Int, toAddr common.Address) (*types.Transaction, error) {
	tokenJson, err := abi.JSON(strings.NewReader(contracts.LivepeerTokenABI))

	if err != nil {
		glog.Errorf("Error decoding ABI into JSON: %v", err)
		return nil, err
	}

	logsCh := make(chan types.Log)
	logsSub, err := c.SubscribeToEvent(c.tokenAddr, tokenJson.Events["Approval"].Id(), logsCh)

	defer logsSub.Unsubscribe()
	defer close(logsCh)

	_, err = c.tokenSession.Approve(c.protocolAddr, amount)

	if err != nil {
		glog.Errorf("Error approving token transfer: %v", err)
		return nil, err
	}

	_, err = c.WatchEvent(logsCh)

	if err != nil {
		glog.Errorf("Error watching event: %v", err)
		return nil, err
	}

	return c.protocolSession.Bond(amount, toAddr)
}

func (c *Client) ValidRewardTimeWindow() (bool, error) {
	return c.protocolSession.ValidRewardTimeWindow(c.addr)
}

func (c *Client) Reward() (*types.Transaction, error) {
	return c.protocolSession.Reward()
}

func (c *Client) Transfer(toAddr common.Address, amount *big.Int) (*types.Transaction, error) {
	return c.tokenSession.Transfer(toAddr, amount)
}
