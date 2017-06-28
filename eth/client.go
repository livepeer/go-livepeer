package eth

// This is meant to be the integration point with the Ethereum smart contract.  It's currently stubbed for now.
//
// We can do the implementation following this link:
// https://github.com/ethereum/go-ethereum/wiki/Native-DApps:-Go-bindings-to-Ethereum-contracts

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
	"github.com/livepeer/libp2p-livepeer/eth/contracts"
)

const (
	waitForMinedTxTimeout = 60
	watchEventTimeout     = 60
)

type Client struct {
	protocolSession *contracts.LivepeerProtocolSession
	tokenSession    *contracts.LivepeerTokenSession
	backend         *ethclient.Client

	addr         common.Address
	protocolAddr common.Address
	tokenAddr    common.Address
}

func NewClient(transactOpts *bind.TransactOpts, backend *ethclient.Client, protocolAddr common.Address) (*Client, error) {
	protocol, err := contracts.NewLivepeerProtocol(protocolAddr, backend)

	if err != nil {
		return nil, err
	}

	tokenAddr, err := protocol.Token(nil)

	if err != nil {
		return nil, err
	}

	token, err := contracts.NewLivepeerToken(tokenAddr, backend)

	if err != nil {
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
	}, nil
}

func DeployLibrary(transactOpts *bind.TransactOpts, backend *ethclient.Client, name string, libraries map[string]common.Address) (common.Address, *types.Transaction, error) {
	var (
		addr common.Address
		tx   *types.Transaction
		err  error
	)

	switch name {
	case "Node":
		addr, tx, _, err = contracts.DeployNode(transactOpts, backend, libraries)
	case "MaxHeap":
		addr, tx, _, err = contracts.DeployMaxHeap(transactOpts, backend, libraries)
	case "MinHeap":
		addr, tx, _, err = contracts.DeployMinHeap(transactOpts, backend, libraries)
	case "TranscoderPools":
		addr, tx, _, err = contracts.DeployTranscoderPools(transactOpts, backend, libraries)
	case "TranscodeJobs":
		addr, tx, _, err = contracts.DeployTranscodeJobs(transactOpts, backend, libraries)
	case "MerkleProof":
		addr, tx, _, err = contracts.DeployMerkleProof(transactOpts, backend, libraries)
	case "ECVerify":
		addr, tx, _, err = contracts.DeployECVerify(transactOpts, backend, libraries)
	case "SafeMath":
		addr, tx, _, err = contracts.DeploySafeMath(transactOpts, backend, libraries)
	default:
		return common.Address{}, nil, fmt.Errorf("Invalid contract name: %v", name)
	}

	if err != nil {
		return common.Address{}, nil, err
	}

	return addr, tx, nil
}

func DeployLivepeerProtocol(transactOpts *bind.TransactOpts, backend *ethclient.Client, libraries map[string]common.Address, n uint64, roundLength *big.Int, cyclesPerRound *big.Int) (common.Address, *types.Transaction, error) {
	addr, tx, _, err := contracts.DeployLivepeerProtocol(transactOpts, backend, libraries, n, roundLength, cyclesPerRound)

	if err != nil {
		return common.Address{}, nil, err
	}

	return addr, tx, nil
}

func waitForMinedTx(ctx context.Context, backend *ethclient.Client, txHash common.Hash) (*types.Receipt, error) {
	var (
		receipt *types.Receipt
		err     error
	)

	for i := 0; i < waitForMinedTxTimeout; i++ {
		receipt, err = backend.TransactionReceipt(ctx, txHash)

		if err != nil && err != ethereum.NotFound {
			return nil, err
		}

		if receipt != nil {
			break
		}

		time.Sleep(time.Second)
	}

	if receipt == nil {
		return nil, ethereum.NotFound
	} else {
		return receipt, nil
	}
}

func (c *Client) subscribeToEvent(ctx context.Context, contractAddr common.Address, eventHash common.Hash) (ethereum.Subscription, <-chan types.Log, error) {
	var (
		logsSub ethereum.Subscription
		logsCh  = make(chan types.Log)
	)

	q := ethereum.FilterQuery{
		Addresses: []common.Address{contractAddr},
		Topics:    [][]common.Hash{[]common.Hash{eventHash}},
	}

	logsSub, err := c.backend.SubscribeFilterLogs(ctx, q, logsCh)

	if err != nil {
		return nil, nil, err
	}

	return logsSub, logsCh, nil
}

func (c *Client) watchEvent(logsCh <-chan types.Log) (types.Log, error) {
	var (
		receivedLog *types.Log
		timeout     = time.NewTimer(watchEventTimeout * time.Second)
	)

	for {
		select {
		case log := <-logsCh:
			if !log.Removed {
				receivedLog = &log
			}
		case <-timeout.C:
			return types.Log{}, fmt.Errorf("watchEvent timed out")
		}

		if receivedLog != nil {
			break
		}
	}

	return *receivedLog, nil
}

func (c *Client) RoundInfo(ctx context.Context) (*big.Int, *big.Int, *big.Int, error) {
	cr, err := c.protocolSession.CurrentRound()

	if err != nil {
		return nil, nil, nil, err
	}

	cn, err := c.protocolSession.CycleNum()

	if err != nil {
		return nil, nil, nil, err
	}

	crsb, err := c.protocolSession.CurrentRoundStartBlock()

	if err != nil {
		return nil, nil, nil, err
	}

	block, err := c.backend.BlockByNumber(ctx, nil)

	if err != nil {
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
		return false, err
	}

	cr, err := c.protocolSession.CurrentRound()

	if err != nil {
		return false, err
	}

	if lir.Cmp(cr) == -1 {
		return true, nil
	} else {
		return false, nil
	}
}

func (c *Client) Transcoder(blockRewardCut uint8, feeShare uint8, pricePerSegment *big.Int) (*types.Transaction, error) {
	return c.protocolSession.Transcoder(blockRewardCut, feeShare, pricePerSegment)
}

func (c *Client) Bond(ctx context.Context, amount *big.Int, toAddr common.Address) (*types.Transaction, error) {
	tokenJson, _ := abi.JSON(strings.NewReader(contracts.LivepeerTokenABI))

	logsSub, logsCh, err := c.subscribeToEvent(ctx, c.tokenAddr, tokenJson.Events["Approval"].Id())

	defer logsSub.Unsubscribe()

	_, err = c.tokenSession.Approve(c.protocolAddr, amount)

	if err != nil {
		return nil, err
	}

	_, err = c.watchEvent(logsCh)

	if err != nil {
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
