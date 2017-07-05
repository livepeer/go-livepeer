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
	"bytes"
	"context"
	"fmt"
	"math/big"
	"path/filepath"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/golang/glog"
	"github.com/livepeer/golp/eth/contracts"
)

type Client struct {
	account         accounts.Account
	keyStore        *keystore.KeyStore
	backend         *ethclient.Client
	protocolAddr    common.Address
	tokenAddr       common.Address
	protocolSession *contracts.LivepeerProtocolSession
	tokenSession    *contracts.LivepeerTokenSession

	rpcTimeout   time.Duration
	eventTimeout time.Duration
}

func NewClient(account accounts.Account, passphrase string, datadir string, backend *ethclient.Client, protocolAddr common.Address, rpcTimeout time.Duration, eventTimeout time.Duration) (*Client, error) {
	keyStore = keystore.NewKeyStore(filepath.Join(dir, datadir, "keystore"), keystore.StandardScryptN, keystore.StandardScryptP)

	transactOpts, err := NewTransactOptsForAccount(account, passphrase, keyStore)

	if err != nil {
		return nil, err
	}

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
		account:      account,
		keyStore:     keyStore,
		backend:      backend,
		protocolAddr: protocolAddr,
		tokenAddr:    tokenAddr,
		protocolSession: &contracts.LivepeerProtocolSession{
			Contract:     protocol,
			TransactOpts: *transactOpts,
		},
		tokenSession: &contracts.LivepeerTokenSession{
			Contract:     token,
			TransactOpts: *transactOpts,
		},
		rpcTimeout:   rpcTimeout,
		eventTimeout: eventTimeout,
	}, nil
}

func NewTransactOptsForAccount(account accounts.Account, passphrase string, keyStore *keystore.KeyStore) (*bind.TransactOpts, error) {
	keyjson, err := keyStore.Export(account, passphrase, passphrase)

	if err != nil {
		return nil, err
	}

	transactOpts, err := bind.NewTransactor(bytes.NewReader(keyjson), passphrase)

	if err != nil {
		return nil, err
	}

	return transactOpts, err
}

func (c *Client) SubscribeToJobEvent() (ethereum.Subscription, chan types.Log, error) {
	var (
		logsCh = make(chan types.Log)
	)

	protocolJson, err := abi.JSON(strings.NewReader(contracts.LivepeerProtocolABI))

	if err != nil {
		glog.Errorf("Error decoding ABI into JSON: %v", err)
		return nil, nil, err
	}

	q := ethereum.FilterQuery{
		Addresses: []common.Address{c.protocolAddr},
		Topics:    [][]common.Hash{[]common.Hash{protocolJson.Events["NewJob"].Id()}, []common.Hash{common.BytesToHash(common.LeftPadBytes(c.account.Address[:], 32))}},
	}

	ctx, _ := context.WithTimeout(context.Background(), c.rpcTimeout)

	logsSub, err := c.backend.SubscribeFilterLogs(ctx, q, logsCh)

	go monitorJobEvent(logsCh)

	return logsSub, logsCh, nil
}

func monitorJobEvent(logsCh <-chan types.Log) {
	for {
		select {
		case log := <-logsCh:
			if !log.Removed && log.BlockNumber != 0 {
				// TODO: Perform action in response to job event
				glog.Infof("Received a job at block %v with tx %v by address %v", log.BlockNumber, log.TxHash.Hex(), log.Address.Hex())
			}
		}
	}
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

	glog.Infof("[%v] Submitted transaction %v. Initialize round", c.account.Address.Hex(), tx.Hash().Hex())
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

	glog.Infof("[%v] Submitted transaction %v. Register as a transcoder", c.account.Address.Hex(), tx.Hash().Hex())
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
		Topics:    [][]common.Hash{[]common.Hash{tokenJson.Events["Approval"].Id()}, []common.Hash{common.BytesToHash(common.LeftPadBytes(c.account.Address[:], 32))}},
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

	glog.Infof("[%v] Submitted transaction %v. Approve %v LPTU transfer by LivepeerProtocol", c.account.Address.Hex(), tx.Hash().Hex(), amount)

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

	glog.Infof("[%v] Submitted transaction %v. Bond %v LPTU to %v at CR %v CN %v CRSB %v CB %v", c.account.Address.Hex(), tx.Hash().Hex(), amount, toAddr.Hex(), cr, cn, crsb, cb)
	return tx, nil
}

func (c *Client) ValidRewardTimeWindow() (bool, error) {
	return c.protocolSession.ValidRewardTimeWindow(c.account.Address)
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

	glog.Infof("[%v] Submitted transaction %v. Reward at CR %v CN %v CRSB %v CB %v", c.account.Address.Hex(), tx.Hash().Hex(), cr, cn, crsb, cb)
	return tx, nil
}

func (c *Client) Job(streamId string, transcodingOptions [32]byte, maxPricePerSegment *big.Int) (*types.Transaction, error) {
	tx, err := c.protocolSession.Job(streamId, transcodingOptions, maxPricePerSegment)

	if err != nil {
		glog.Errorf("Error creating job: %v", err)
		return nil, err
	}

	glog.Infof("[%v] Submitted transaction %v. Creating a job for stream id %v", c.account.Address.Hex(), tx.Hash().Hex(), streamId)
	return tx, nil
}

func (c *Client) SignSegmentHash(passphrase string, hash []byte) ([]byte, error) {
	sig, err := c.keyStore.SignHashWithPassphrase(c.account, passphrase, hash)

	if err != nil {
		glog.Errorf("Error signing segment: %v", err)
		return nil, err
	}

	glog.Infof("[%v] Created signed segment hash %v", c.account.Address.Hex(), common.ToHex(sig))
	return sig, nil
}

func (c *Client) ClaimWork(jobId *big.Int, startSegmentSequenceNumber *big.Int, endSegmentSequenceNumber *big.Int, transcodeClaimsRoot [32]byte) (*types.Transaction, error) {
	tx, err := c.protocolSession.ClaimWork(jobId, startSegmentSequenceNumber, endSegmentSequenceNumber, transcodeClaimsRoot)

	if err != nil {
		glog.Errorf("Error claiming work: %v", err)
		return nil, err
	}

	glog.Infof("[%v] Submitted transaction %v. Claimed work for segments %v - %v", c.account.Address.Hex(), tx.Hash().Hex(), startSegmentSequenceNumber, endSegmentSequenceNumber)
	return tx, nil
}

func (c *Client) Verify(jobId *big.Int, segmentSequenceNumber *big.Int, dataHash [32]byte, transcodedDataHash [32]byte, broadcasterSig []byte, proof []byte) (*types.Transaction, error) {
	tx, err := c.protocolSession.Verify(jobId, segmentSequenceNumber, dataHash, transcodedDataHash, broadcasterSig, proof)

	if err != nil {
		glog.Errorf("Error invoking verify %v", err)
		return nil, err
	}

	glog.Infof("[%v] Submitted transaction %v, Invoked verify for segment %v", c.account.Address.Hex(), tx.Hash().Hex(), segmentSequenceNumber)
	return tx, nil
}

// Token methods

func (c *Client) Transfer(toAddr common.Address, amount *big.Int) (*types.Transaction, error) {
	tx, err := c.tokenSession.Transfer(toAddr, amount)

	if err != nil {
		glog.Errorf("Error transferring tokens: %v", err)
		return nil, err
	}

	glog.Infof("[%v] Submitted transaction %v. Transfer %v LPTU to %v", c.account.Address.Hex(), tx.Hash().Hex(), amount, toAddr.Hex())
	return tx, nil
}
