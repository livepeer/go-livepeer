/*
Package eth client is the go client for the Livepeer Ethereum smart contract.  Contracts here are generated.
*/
package eth

//go:generate abigen --abi protocol/abi/Controller.abi --pkg contracts --type Controller --out contracts/controller.go
//go:generate abigen --abi protocol/abi/LivepeerToken.abi --pkg contracts --type LivepeerToken --out contracts/livepeerToken.go
//go:generate abigen --abi protocol/abi/BondingManager.abi --pkg contracts --type BondingManager --out contracts/bondingManager.go
//go:generate abigen --abi protocol/abi/JobsManager.abi --pkg contracts --type JobsManager --out contracts/jobsManager.go
//go:generate abigen --abi protocol/abi/RoundsManager.abi --pkg contracts --type RoundsManager --out contracts/roundsManager.go
//go:generate abigen --abi protocol/abi/LivepeerTokenFaucet.abi --pkg contracts --type LivepeerTokenFaucet --out contracts/livepeerTokenFaucet.go

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
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/eth/contracts"
)

var ProtocolCyclesPerRound = 2
var ProtocolBlockPerRound = big.NewInt(20)

type LivepeerEthClient interface {
	Backend() *ethclient.Client
	Account() accounts.Account
	RpcTimeout() time.Duration
	SubscribeToJobEvent(ctx context.Context, logsCh chan types.Log, broadcasterAddr, transcoderAddr common.Address) (ethereum.Subscription, error)
	RoundInfo() (*big.Int, *big.Int, *big.Int, error)
	InitializeRound() (<-chan types.Receipt, <-chan error)
	Transcoder(blockRewardCut uint8, feeShare uint8, pricePerSegment *big.Int) (<-chan types.Receipt, <-chan error)
	Bond(amount *big.Int, toAddr common.Address) (<-chan types.Receipt, <-chan error)
	Unbond() (<-chan types.Receipt, <-chan error)
	WithdrawBond() (<-chan types.Receipt, <-chan error)
	Reward() (<-chan types.Receipt, <-chan error)
	Deposit(amount *big.Int) (<-chan types.Receipt, <-chan error)
	GetBroadcasterDeposit(broadcaster common.Address) (*big.Int, error)
	WithdrawDeposit() (<-chan types.Receipt, <-chan error)
	Job(streamId string, transcodingOptions string, maxPricePerSegment *big.Int) (<-chan types.Receipt, <-chan error)
	EndJob(jobID *big.Int) (<-chan types.Receipt, <-chan error)
	ClaimWork(jobId *big.Int, segmentRange [2]*big.Int, claimRoot [32]byte) (<-chan types.Receipt, <-chan error)
	Verify(jobId *big.Int, claimId *big.Int, segmentNumber *big.Int, dataStorageHash string, dataHashes [2][32]byte, broadcasterSig []byte, proof []byte) (<-chan types.Receipt, <-chan error)
	DistributeFees(jobId *big.Int, claimId *big.Int) (<-chan types.Receipt, <-chan error)
	Transfer(toAddr common.Address, amount *big.Int) (<-chan types.Receipt, <-chan error)
	RequestTokens() (<-chan types.Receipt, <-chan error)
	CurrentRoundInitialized() (bool, error)
	IsActiveTranscoder() (bool, error)
	TranscoderStatus() (string, error)
	TranscoderStake() (*big.Int, error)
	TranscoderPendingPricingInfo() (uint8, uint8, *big.Int, error)
	TranscoderPricingInfo() (uint8, uint8, *big.Int, error)
	DelegatorStatus() (string, error)
	DelegatorStake() (*big.Int, error)
	TokenBalance() (*big.Int, error)
	GetJob(jobID *big.Int) (*Job, error)
	GetClaim(jobID *big.Int, claimID *big.Int) (*Claim, error)
	VerificationRate() (uint64, error)
	VerificationPeriod() (*big.Int, error)
	SlashingPeriod() (*big.Int, error)
	LastRewardRound() (*big.Int, error)
	IsRegisteredTranscoder() (bool, error)
	TranscoderBond() (*big.Int, error)
	GetCandidateTranscodersStats() ([]TranscoderStats, error)
	GetReserveTranscodersStats() ([]TranscoderStats, error)
	GetControllerAddr() string
	GetTokenAddr() string
	GetFaucetAddr() string
	GetBondingManagerAddr() string
	GetJobsManagerAddr() string
	GetRoundsManagerAddr() string
	GetBlockInfoByTxHash(ctx context.Context, hash common.Hash) (blkNum *big.Int, blkHash common.Hash, err error)
}

type Client struct {
	account               accounts.Account
	keyStore              *keystore.KeyStore
	transactOpts          bind.TransactOpts
	backend               *ethclient.Client
	controllerAddr        common.Address
	tokenAddr             common.Address
	bondingManagerAddr    common.Address
	jobsManagerAddr       common.Address
	roundsManagerAddr     common.Address
	faucetAddr            common.Address
	controllerSession     *contracts.ControllerSession
	tokenSession          *contracts.LivepeerTokenSession
	bondingManagerSession *contracts.BondingManagerSession
	jobsManagerSession    *contracts.JobsManagerSession
	roundsManagerSession  *contracts.RoundsManagerSession
	faucetSession         *contracts.LivepeerTokenFaucetSession

	rpcTimeout   time.Duration
	eventTimeout time.Duration
}

type TranscoderStats struct {
	Address                common.Address
	TotalStake             *big.Int
	PendingBlockRewardCut  uint8
	PendingFeeShare        uint8
	PendingPricePerSegment *big.Int
	BlockRewardCut         uint8
	FeeShare               uint8
	PricePerSegment        *big.Int
}

type Job struct {
	JobId              *big.Int
	StreamId           string
	TranscodingOptions string
	MaxPricePerSegment *big.Int
	BroadcasterAddress common.Address
	TranscoderAddress  common.Address
	EndBlock           *big.Int
	Escrow             *big.Int
}

type Claim struct {
	SegmentRange         [2]*big.Int
	ClaimRoot            [32]byte
	ClaimBlock           *big.Int
	EndVerificationBlock *big.Int
	EndSlashingBlock     *big.Int
	TranscoderTotalStake *big.Int
	Status               uint8
}

func NewClient(account accounts.Account, passphrase string, datadir string, backend *ethclient.Client, gasPrice *big.Int, controllerAddr common.Address, rpcTimeout time.Duration, eventTimeout time.Duration) (*Client, error) {
	keyStore := keystore.NewKeyStore(filepath.Join(datadir, "keystore"), keystore.StandardScryptN, keystore.StandardScryptP)

	transactOpts, err := NewTransactOptsForAccount(account, passphrase, keyStore)
	if err != nil {
		return nil, err
	}

	transactOpts.GasPrice = gasPrice

	controller, err := contracts.NewController(controllerAddr, backend)
	if err != nil {
		glog.Errorf("Error creating Controller: %v", err)
		return nil, err
	}

	client := &Client{
		account:        account,
		keyStore:       keyStore,
		transactOpts:   *transactOpts,
		backend:        backend,
		controllerAddr: controllerAddr,
		controllerSession: &contracts.ControllerSession{
			Contract:     controller,
			TransactOpts: *transactOpts,
		},
		rpcTimeout:   rpcTimeout,
		eventTimeout: eventTimeout,
	}

	glog.Infof("Creating client for account %v", transactOpts.From.Hex())

	client.SetManagers()

	return client, nil
}

func (c *Client) SetManagers() error {
	tokenAddr, err := c.controllerSession.GetContract(crypto.Keccak256Hash([]byte("LivepeerToken")))
	if err != nil {
		glog.Errorf("Error getting LivepeerToken address: %v", err)
		return err
	}

	c.tokenAddr = tokenAddr

	token, err := contracts.NewLivepeerToken(tokenAddr, c.backend)
	if err != nil {
		glog.Errorf("Error creating LivpeeerToken: %v", err)
		return err
	}

	c.tokenSession = &contracts.LivepeerTokenSession{
		Contract:     token,
		TransactOpts: c.transactOpts,
	}

	glog.Infof("LivepeerToken: %v", c.tokenAddr.Hex())

	bondingManagerAddr, err := c.controllerSession.GetContract(crypto.Keccak256Hash([]byte("BondingManager")))
	if err != nil {
		glog.Errorf("Error getting BondingManager address: %v", err)
		return err
	}

	c.bondingManagerAddr = bondingManagerAddr

	bondingManager, err := contracts.NewBondingManager(bondingManagerAddr, c.backend)
	if err != nil {
		glog.Errorf("Error creating BondingManager: %v", err)
		return err
	}

	c.bondingManagerSession = &contracts.BondingManagerSession{
		Contract:     bondingManager,
		TransactOpts: c.transactOpts,
	}

	glog.Infof("BondingManager: %v", c.bondingManagerAddr.Hex())

	jobsManagerAddr, err := c.controllerSession.GetContract(crypto.Keccak256Hash([]byte("JobsManager")))
	if err != nil {
		glog.Errorf("Error getting JobsManager address: %v", err)
		return err
	}

	c.jobsManagerAddr = jobsManagerAddr

	jobsManager, err := contracts.NewJobsManager(jobsManagerAddr, c.backend)
	if err != nil {
		glog.Errorf("Error creating JobsManager: %v", err)
		return err
	}

	c.jobsManagerSession = &contracts.JobsManagerSession{
		Contract:     jobsManager,
		TransactOpts: c.transactOpts,
	}

	glog.Infof("JobsManager: %v", c.jobsManagerAddr.Hex())

	roundsManagerAddr, err := c.controllerSession.GetContract(crypto.Keccak256Hash([]byte("RoundsManager")))
	if err != nil {
		glog.Errorf("Error getting RoundsManager address: %v", err)
		return err
	}

	c.roundsManagerAddr = roundsManagerAddr

	roundsManager, err := contracts.NewRoundsManager(roundsManagerAddr, c.backend)
	if err != nil {
		glog.Errorf("Error creating RoundsManager: %v", err)
		return err
	}

	c.roundsManagerSession = &contracts.RoundsManagerSession{
		Contract:     roundsManager,
		TransactOpts: c.transactOpts,
	}

	glog.Infof("RoundsManager: %v", c.roundsManagerAddr.Hex())

	faucetAddr, err := c.controllerSession.GetContract(crypto.Keccak256Hash([]byte("LivepeerTokenFaucet")))
	if err != nil {
		glog.Errorf("Error getting LivepeerTokenFacuet address: %v", err)
		return err
	}

	c.faucetAddr = faucetAddr

	faucet, err := contracts.NewLivepeerTokenFaucet(faucetAddr, c.backend)
	if err != nil {
		glog.Errorf("Error creating LivepeerTokenFacuet: %v", err)
		return err
	}

	c.faucetSession = &contracts.LivepeerTokenFaucetSession{
		Contract:     faucet,
		TransactOpts: c.transactOpts,
	}

	glog.Infof("LivepeerTokenFaucet: %v", c.faucetAddr.Hex())

	return nil
}

func (c *Client) Backend() *ethclient.Client {
	return c.backend
}

func (c *Client) Account() accounts.Account {
	return c.account
}

func (c *Client) RpcTimeout() time.Duration {
	return c.rpcTimeout
}

func NewTransactOptsForAccount(account accounts.Account, passphrase string, keyStore *keystore.KeyStore) (*bind.TransactOpts, error) {
	keyjson, err := keyStore.Export(account, passphrase, passphrase)

	if err != nil {
		return nil, err
	}

	transactOpts, err := bind.NewTransactor(bytes.NewReader(keyjson), passphrase)
	transactOpts.GasLimit = big.NewInt(4000000)

	if err != nil {
		return nil, err
	}

	return transactOpts, err
}

// TRANSACTIONS

func (c *Client) InitializeRound() (<-chan types.Receipt, <-chan error) {
	return c.WaitForReceipt(func() (*types.Transaction, error) {
		if tx, err := c.roundsManagerSession.InitializeRound(); err != nil {
			return nil, err
		} else {
			glog.Infof("[%v] Submitted tx %v. Initialize round", c.account.Address.Hex(), tx.Hash().Hex())
			return tx, nil
		}
	})
}

func (c *Client) Transcoder(blockRewardCut uint8, feeShare uint8, pricePerSegment *big.Int) (<-chan types.Receipt, <-chan error) {
	return c.WaitForReceipt(func() (*types.Transaction, error) {
		if tx, err := c.bondingManagerSession.Transcoder(blockRewardCut, feeShare, pricePerSegment); err != nil {
			return nil, err
		} else {
			glog.Infof("[%v] Submitted tx %v. Calling transcoder", c.account.Address.Hex(), tx.Hash().Hex())
			return tx, nil
		}
	})
}

func (c *Client) Bond(amount *big.Int, toAddr common.Address) (<-chan types.Receipt, <-chan error) {
	return c.ApproveAndTransact(c.bondingManagerAddr, amount, func() (*types.Transaction, error) {
		if tx, err := c.bondingManagerSession.Bond(amount, toAddr); err != nil {
			return nil, err
		} else {
			glog.Infof("[%v] Submitted tx %v. Bond %v LPTU to %v", c.account.Address.Hex(), tx.Hash().Hex(), amount, toAddr.Hex())
			return tx, nil
		}
	})
}

func (c *Client) Unbond() (<-chan types.Receipt, <-chan error) {
	return c.WaitForReceipt(func() (*types.Transaction, error) {
		if tx, err := c.bondingManagerSession.Unbond(); err != nil {
			return nil, err
		} else {
			glog.Infof("[%v] Submitted tx %v. Unbond", c.account.Address.Hex(), tx.Hash().Hex())
			return tx, nil
		}
	})
}

func (c *Client) WithdrawBond() (<-chan types.Receipt, <-chan error) {
	return c.WaitForReceipt(func() (*types.Transaction, error) {
		if tx, err := c.bondingManagerSession.Withdraw(); err != nil {
			return nil, err
		} else {
			glog.Infof("[%v] Submitted tx %v. Withdraw bond", c.account.Address.Hex(), tx.Hash().Hex())
			return tx, nil
		}
	})
}

func (c *Client) Reward() (<-chan types.Receipt, <-chan error) {
	return c.WaitForReceipt(func() (*types.Transaction, error) {
		if tx, err := c.bondingManagerSession.Reward(); err != nil {
			return nil, err
		} else {
			glog.Infof("[%v] Submitted tx %v. Called reward", c.account.Address.Hex(), tx.Hash().Hex())
			return tx, nil
		}
	})
}

func (c *Client) Deposit(amount *big.Int) (<-chan types.Receipt, <-chan error) {
	return c.ApproveAndTransact(c.jobsManagerAddr, amount, func() (*types.Transaction, error) {
		if tx, err := c.jobsManagerSession.Deposit(amount); err != nil {
			return nil, err
		} else {
			glog.Infof("[%v] Submitted tx %v. Deposit %v LPTU", c.account.Address.Hex(), tx.Hash().Hex(), amount)
			return tx, nil
		}
	})
}

func (c *Client) GetBroadcasterDeposit(broadcaster common.Address) (*big.Int, error) {
	return c.jobsManagerSession.BroadcasterDeposits(broadcaster)
}

func (c *Client) WithdrawDeposit() (<-chan types.Receipt, <-chan error) {
	return c.WaitForReceipt(func() (*types.Transaction, error) {
		if tx, err := c.jobsManagerSession.Withdraw(); err != nil {
			return nil, err
		} else {
			glog.Infof("[%v] Submitted tx %v. Withdraw deposit", c.account.Address.Hex(), tx.Hash().Hex())
			return tx, nil
		}
	})
}

func (c *Client) Job(streamId string, transcodingOptions string, maxPricePerSegment *big.Int) (<-chan types.Receipt, <-chan error) {
	return c.WaitForReceipt(func() (*types.Transaction, error) {
		if tx, err := c.jobsManagerSession.Job(streamId, transcodingOptions, maxPricePerSegment); err != nil {
			return nil, err
		} else {
			glog.Infof("[%v] Submitted tx %v. Creating job for stream id %v", c.account.Address.Hex(), tx.Hash().Hex(), streamId)
			return tx, nil
		}
	})
}

func (c *Client) EndJob(jobId *big.Int) (<-chan types.Receipt, <-chan error) {
	return c.WaitForReceipt(func() (*types.Transaction, error) {
		if tx, err := c.jobsManagerSession.EndJob(jobId); err != nil {
			return nil, err
		} else {
			glog.Infof("[%v] Submitted tx %v. Ending job %v", c.account.Address.Hex(), tx.Hash().Hex(), jobId)
			return tx, nil
		}
	})
}

func (c *Client) ClaimWork(jobId *big.Int, segmentRange [2]*big.Int, claimRoot [32]byte) (<-chan types.Receipt, <-chan error) {
	return c.WaitForReceipt(func() (*types.Transaction, error) {
		if tx, err := c.jobsManagerSession.ClaimWork(jobId, segmentRange, claimRoot); err != nil {
			return nil, err
		} else {
			glog.Infof("[%v] Submitted tx %v. Claimed work for segments %v - %v", c.account.Address.Hex(), tx.Hash().Hex(), segmentRange[0], segmentRange[1])
			return tx, nil
		}
	})
}

func (c *Client) Verify(jobId *big.Int, claimId *big.Int, segmentNumber *big.Int, dataStorageHash string, dataHashes [2][32]byte, broadcasterSig []byte, proof []byte) (<-chan types.Receipt, <-chan error) {
	return c.WaitForReceipt(func() (*types.Transaction, error) {
		if tx, err := c.jobsManagerSession.Verify(jobId, claimId, segmentNumber, dataStorageHash, dataHashes, broadcasterSig, proof); err != nil {
			return nil, err
		} else {
			glog.Infof("[%v] Submitted tx %v. Verify segment %v in claim %v", c.account.Address.Hex(), tx.Hash().Hex(), segmentNumber, claimId)
			return tx, nil
		}
	})
}

func (c *Client) DistributeFees(jobId *big.Int, claimId *big.Int) (<-chan types.Receipt, <-chan error) {
	return c.WaitForReceipt(func() (*types.Transaction, error) {
		if tx, err := c.jobsManagerSession.DistributeFees(jobId, claimId); err != nil {
			return nil, err
		} else {
			glog.Infof("[%v] Submitted tx %v. Distributed fees for job %v claim %v", c.account.Address.Hex(), tx.Hash().Hex(), jobId, claimId)
			return tx, nil
		}
	})
}

func (c *Client) BatchDistributeFees(jobId *big.Int, claimIds []*big.Int) (<-chan types.Receipt, <-chan error) {
	return c.WaitForReceipt(func() (*types.Transaction, error) {
		if tx, err := c.jobsManagerSession.BatchDistributeFees(jobId, claimIds); err != nil {
			return nil, err
		} else {
			glog.Infof("[%v] Submitted tx %v. Distributed fee for job %v claims %v", c.account.Address.Hex(), tx.Hash().Hex(), jobId, claimIds)
			return tx, nil
		}
	})
}

func (c *Client) Transfer(toAddr common.Address, amount *big.Int) (<-chan types.Receipt, <-chan error) {
	return c.WaitForReceipt(func() (*types.Transaction, error) {
		if tx, err := c.tokenSession.Transfer(toAddr, amount); err != nil {
			return nil, err
		} else {
			glog.Infof("[%v] Submitted tx %v. Transfer %v LPTU to %v", c.account.Address.Hex(), tx.Hash().Hex(), amount, toAddr.Hex())
			return tx, nil
		}
	})
}

func (c *Client) Approve(toAddr common.Address, amount *big.Int) (<-chan types.Receipt, <-chan error) {
	return c.WaitForReceipt(func() (*types.Transaction, error) {
		if tx, err := c.tokenSession.Approve(toAddr, amount); err != nil {
			return nil, err
		} else {
			glog.Infof("[%v], Submitted tx %v. Approve %v LPTU to %v", c.account.Address.Hex(), tx.Hash().Hex(), amount, toAddr.Hex())
			return tx, nil
		}
	})
}

func (c *Client) RequestTokens() (<-chan types.Receipt, <-chan error) {
	return c.WaitForReceipt(func() (*types.Transaction, error) {
		if tx, err := c.faucetSession.Request(); err != nil {
			return nil, err
		} else {
			glog.Infof("[%v], Submitted tx %v. Requested tokens from faucet", c.account.Address.Hex(), tx.Hash().Hex())
			return tx, nil
		}
	})
}

func (c *Client) SubscribeToApproval() (chan types.Log, ethereum.Subscription, error) {
	logCh := make(chan types.Log)

	abiJSON, err := abi.JSON(strings.NewReader(contracts.LivepeerTokenABI))
	if err != nil {
		return nil, nil, err
	}

	q := ethereum.FilterQuery{
		Addresses: []common.Address{c.tokenAddr},
		Topics:    [][]common.Hash{[]common.Hash{abiJSON.Events["Approval"].Id()}, []common.Hash{common.BytesToHash(common.LeftPadBytes(c.account.Address[:], 32))}},
	}

	ctx, _ := context.WithTimeout(context.Background(), c.rpcTimeout)

	sub, err := c.backend.SubscribeFilterLogs(ctx, q, logCh)
	if err != nil {
		return nil, nil, err
	}

	return logCh, sub, nil
}

func (c *Client) SubscribeToJobEvent(ctx context.Context, logsCh chan types.Log, broadcasterAddr, transcoderAddr common.Address) (ethereum.Subscription, error) {
	abiJSON, err := abi.JSON(strings.NewReader(contracts.JobsManagerABI))
	if err != nil {
		glog.Errorf("Error decoding ABI into JSON: %v", err)
		return nil, err
	}

	var q ethereum.FilterQuery
	if !IsNullAddress(broadcasterAddr) {
		q = ethereum.FilterQuery{
			Addresses: []common.Address{c.jobsManagerAddr},
			Topics:    [][]common.Hash{[]common.Hash{abiJSON.Events["NewJob"].Id()}, []common.Hash{}, []common.Hash{common.BytesToHash(common.LeftPadBytes(broadcasterAddr[:], 32))}},
		}
	} else if !IsNullAddress(transcoderAddr) {
		q = ethereum.FilterQuery{
			Addresses: []common.Address{c.jobsManagerAddr},
			Topics:    [][]common.Hash{[]common.Hash{abiJSON.Events["NewJob"].Id()}, []common.Hash{common.BytesToHash(common.LeftPadBytes(transcoderAddr[:], 32))}},
		}
	} else {
		q = ethereum.FilterQuery{
			Addresses: []common.Address{c.jobsManagerAddr},
			Topics: [][]common.Hash{
				[]common.Hash{abiJSON.Events["NewJob"].Id()},
				[]common.Hash{common.BytesToHash(common.LeftPadBytes(transcoderAddr[:], 32))},
				[]common.Hash{common.BytesToHash(common.LeftPadBytes(broadcasterAddr[:], 32))},
			},
		}
	}

	return c.backend.SubscribeFilterLogs(ctx, q, logsCh)
}

// CONSTANT FUNCTIONS

//RoundInfo returns the current round, start block of current round and current block of the protocol
func (c *Client) RoundInfo() (*big.Int, *big.Int, *big.Int, error) {
	cr, err := c.roundsManagerSession.CurrentRound()
	if err != nil {
		glog.Errorf("Error getting current round: %v", err)
		return nil, nil, nil, err
	}

	crsb, err := c.roundsManagerSession.CurrentRoundStartBlock()
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

	return cr, crsb, block.Number(), nil
}

func (c *Client) CurrentRoundInitialized() (bool, error) {
	initialized, err := c.roundsManagerSession.CurrentRoundInitialized()
	if err != nil {
		glog.Errorf("Error checking if current round initialized: %v", err)
		return false, err
	}

	return initialized, nil
}

func (c *Client) IsRegisteredTranscoder() (bool, error) {
	status, err := c.bondingManagerSession.TranscoderStatus(c.account.Address)
	if err != nil {
		return false, err
	}

	// TODO: Use enum here for transcoder status?
	return status == 1, nil
}

func (c *Client) IsActiveTranscoder() (bool, error) {
	return c.bondingManagerSession.IsActiveTranscoder(c.account.Address)
}

func (c *Client) TranscoderBond() (*big.Int, error) {
	return c.bondingManagerSession.GetDelegatorBondedAmount(c.account.Address)
}

func (c *Client) TranscoderStake() (*big.Int, error) {
	return c.bondingManagerSession.TranscoderTotalStake(c.account.Address)
}

func (c *Client) TranscoderStatus() (string, error) {
	status, err := c.bondingManagerSession.TranscoderStatus(c.account.Address)
	if err != nil {
		return "", err
	}

	switch status {
	case 0:
		return "NotRegistered", nil
	case 1:
		return "Registered", nil
	case 2:
		return "Unbonding", nil
	case 3:
		return "Unbonded", nil
	default:
		return "", fmt.Errorf("Unknown transcoder status")
	}
}

func (c *Client) TranscoderPendingPricingInfo() (uint8, uint8, *big.Int, error) {
	return c.GetTranscoderPendingPricingInfo(c.account.Address)
}

func (c *Client) GetTranscoderPendingPricingInfo(addr common.Address) (uint8, uint8, *big.Int, error) {
	pBlockRewardCut, err := c.bondingManagerSession.GetTranscoderPendingBlockRewardCut(addr)
	if err != nil {
		return 0, 0, nil, err
	}

	pFeeShare, err := c.bondingManagerSession.GetTranscoderPendingFeeShare(addr)
	if err != nil {
		return 0, 0, nil, err
	}

	pPricePerSegment, err := c.bondingManagerSession.GetTranscoderPendingPricePerSegment(addr)
	if err != nil {
		return 0, 0, nil, err
	}

	return pBlockRewardCut, pFeeShare, pPricePerSegment, nil
}

func (c *Client) TranscoderPricingInfo() (uint8, uint8, *big.Int, error) {
	return c.GetTranscoderPricingInfo(c.account.Address)
}

func (c *Client) GetTranscoderPricingInfo(addr common.Address) (uint8, uint8, *big.Int, error) {
	blockRewardCut, err := c.bondingManagerSession.GetTranscoderBlockRewardCut(addr)
	if err != nil {
		return 0, 0, nil, err
	}

	feeShare, err := c.bondingManagerSession.GetTranscoderFeeShare(addr)
	if err != nil {
		return 0, 0, nil, err
	}

	pricePerSegment, err := c.bondingManagerSession.GetTranscoderPricePerSegment(addr)
	if err != nil {
		return 0, 0, nil, err
	}

	return blockRewardCut, feeShare, pricePerSegment, nil
}

func (c *Client) DelegatorStatus() (string, error) {
	status, err := c.bondingManagerSession.DelegatorStatus(c.account.Address)
	if err != nil {
		return "", err
	}

	switch status {
	case 0:
		return "NotRegistered", nil
	case 1:
		return "Pending", nil
	case 2:
		return "Bonded", nil
	case 3:
		return "Unbonding", nil
	case 4:
		return "Unbonded", nil
	default:
		return "", fmt.Errorf("Unknown delegator status")
	}
}

func (c *Client) DelegatorStake() (*big.Int, error) {
	return c.bondingManagerSession.DelegatorStake(c.account.Address)
}

func (c *Client) LastRewardRound() (*big.Int, error) {
	return c.bondingManagerSession.GetTranscoderLastRewardRound(c.account.Address)
}

func (c *Client) RoundLength() (*big.Int, error) {
	return c.roundsManagerSession.RoundLength()
}

func (c *Client) VerificationRate() (uint64, error) {
	return c.jobsManagerSession.VerificationRate()
}

func (c *Client) VerificationPeriod() (*big.Int, error) {
	return c.jobsManagerSession.VerificationPeriod()
}

func (c *Client) SlashingPeriod() (*big.Int, error) {
	return c.jobsManagerSession.SlashingPeriod()
}

func (c *Client) GetJob(jobID *big.Int) (*Job, error) {
	maxPricePerSegment, err := c.jobsManagerSession.GetJobMaxPricePerSegment(jobID)
	if err != nil {
		return nil, err
	}

	broadcaster, err := c.jobsManagerSession.GetJobBroadcasterAddress(jobID)
	if err != nil {
		return nil, err
	}

	transcoder, err := c.jobsManagerSession.GetJobTranscoderAddress(jobID)
	if err != nil {
		return nil, err
	}

	endBlock, err := c.jobsManagerSession.GetJobEndBlock(jobID)
	if err != nil {
		return nil, err
	}

	escrow, err := c.jobsManagerSession.GetJobEscrow(jobID)
	if err != nil {
		return nil, err
	}

	// If we are using the ManagerProxy contract address
	// we cannot directly retrieve dynamic length data at the moment
	// We have to retrieve streamId and transcodingOptions via logs
	return &Job{
		JobId:              jobID,
		MaxPricePerSegment: maxPricePerSegment,
		BroadcasterAddress: broadcaster,
		TranscoderAddress:  transcoder,
		EndBlock:           endBlock,
		Escrow:             escrow,
	}, nil
}

//TODO: Go binding has an issue returning [32]byte...
func (c *Client) GetClaim(jobID *big.Int, claimID *big.Int) (*Claim, error) {
	startSegment, err := c.jobsManagerSession.GetClaimStartSegment(jobID, claimID)
	if err != nil {
		return nil, err
	}

	endSegment, err := c.jobsManagerSession.GetClaimEndSegment(jobID, claimID)
	if err != nil {
		return nil, err
	}

	claimRoot, err := c.jobsManagerSession.GetClaimRoot(jobID, claimID)
	if err != nil {
		return nil, err
	}

	claimBlock, err := c.jobsManagerSession.GetClaimBlock(jobID, claimID)
	if err != nil {
		return nil, err
	}

	endVerificationBlock, err := c.jobsManagerSession.GetClaimEndVerificationBlock(jobID, claimID)
	if err != nil {
		return nil, err
	}

	endSlashingBlock, err := c.jobsManagerSession.GetClaimEndSlashingBlock(jobID, claimID)
	if err != nil {
		return nil, err
	}

	transcoderTotalStake, err := c.jobsManagerSession.GetClaimTranscoderTotalStake(jobID, claimID)
	if err != nil {
		return nil, err
	}

	status, err := c.jobsManagerSession.GetClaimStatus(jobID, claimID)
	if err != nil {
		return nil, err
	}

	return &Claim{
		SegmentRange:         [2]*big.Int{startSegment, endSegment},
		ClaimRoot:            claimRoot,
		ClaimBlock:           claimBlock,
		EndVerificationBlock: endVerificationBlock,
		EndSlashingBlock:     endSlashingBlock,
		TranscoderTotalStake: transcoderTotalStake,
		Status:               status,
	}, err
}

func (c *Client) GetCandidateTranscodersStats() ([]TranscoderStats, error) {
	poolSize, err := c.bondingManagerSession.GetCandidatePoolSize()
	if err != nil {
		return nil, err
	}

	var candidateTranscodersStats []TranscoderStats
	for i := 0; i < int(poolSize.Int64()); i++ {
		transcoder, err := c.bondingManagerSession.GetCandidateTranscoderAtPosition(big.NewInt(int64(i)))
		if err != nil {
			return nil, err
		}

		blockRewardCut, feeShare, pricePerSegment, err := c.GetTranscoderPricingInfo(transcoder)
		if err != nil {
			return nil, err

		}

		pBlockRewardCut, pFeeShare, pPricePerSegment, err := c.GetTranscoderPendingPricingInfo(transcoder)
		if err != nil {
			return nil, err
		}

		transcoderTotalStake, err := c.bondingManagerSession.TranscoderTotalStake(transcoder)
		if err != nil {
			return nil, err
		}

		stats := TranscoderStats{
			Address:                transcoder,
			TotalStake:             transcoderTotalStake,
			PendingBlockRewardCut:  pBlockRewardCut,
			PendingFeeShare:        pFeeShare,
			PendingPricePerSegment: pPricePerSegment,
			BlockRewardCut:         blockRewardCut,
			FeeShare:               feeShare,
			PricePerSegment:        pricePerSegment,
		}

		candidateTranscodersStats = append(candidateTranscodersStats, stats)
	}

	return candidateTranscodersStats, nil
}

func (c *Client) GetReserveTranscodersStats() ([]TranscoderStats, error) {
	poolSize, err := c.bondingManagerSession.GetReservePoolSize()
	if err != nil {
		return nil, err
	}

	if poolSize.Cmp(big.NewInt(0)) == 0 {
		return nil, nil
	}

	var reserveTranscodersStats []TranscoderStats
	for i := 0; i < int(poolSize.Int64()); i++ {
		transcoder, err := c.bondingManagerSession.GetReserveTranscoderAtPosition(big.NewInt(int64(i)))
		if err != nil {
			return nil, err
		}

		blockRewardCut, feeShare, pricePerSegment, err := c.GetTranscoderPricingInfo(transcoder)
		if err != nil {
			return nil, err

		}

		pBlockRewardCut, pFeeShare, pPricePerSegment, err := c.GetTranscoderPendingPricingInfo(transcoder)
		if err != nil {
			return nil, err
		}

		transcoderTotalStake, err := c.bondingManagerSession.TranscoderTotalStake(transcoder)
		if err != nil {
			return nil, err
		}

		stats := TranscoderStats{
			Address:                transcoder,
			TotalStake:             transcoderTotalStake,
			PendingBlockRewardCut:  pBlockRewardCut,
			PendingFeeShare:        pFeeShare,
			PendingPricePerSegment: pPricePerSegment,
			BlockRewardCut:         blockRewardCut,
			FeeShare:               feeShare,
			PricePerSegment:        pricePerSegment,
		}

		reserveTranscodersStats = append(reserveTranscodersStats, stats)
	}

	return reserveTranscodersStats, nil
}

func (c *Client) TokenBalance() (*big.Int, error) {
	return c.tokenSession.BalanceOf(c.account.Address)
}

// HELPERS

func (c *Client) SignSegmentHash(passphrase string, hash []byte) ([]byte, error) {
	msg := fmt.Sprintf("\x19Ethereum Signed Message:\n%d%s", 32, hash)
	signHash := crypto.Keccak256([]byte(msg))

	sig, err := c.keyStore.SignHashWithPassphrase(c.account, passphrase, signHash)
	if err != nil {
		glog.Errorf("Error signing segment hash: %v", err)
		return nil, err
	}

	return sig, nil
}

func (c *Client) GetReceipt(tx *types.Transaction) (*types.Receipt, error) {
	start := time.Now()
	for time.Since(start) < c.eventTimeout {
		ctx, _ := context.WithTimeout(context.Background(), c.rpcTimeout)

		receipt, err := c.backend.TransactionReceipt(ctx, tx.Hash())
		if err != nil && err != ethereum.NotFound {
			return nil, err
		}

		if receipt != nil {
			if tx.Gas().Cmp(receipt.GasUsed) == 0 {
				return nil, fmt.Errorf("Tx %v threw", tx.Hash().Hex())
			} else {
				return receipt, nil
			}
		}

		time.Sleep(time.Second)
	}

	return nil, fmt.Errorf("Tx %v timed out", tx.Hash().Hex())
}

func (c *Client) WaitForReceipt(txFunc func() (*types.Transaction, error)) (<-chan types.Receipt, <-chan error) {
	outRes := make(chan types.Receipt)
	outErr := make(chan error)

	go func() {
		defer close(outRes)
		defer close(outErr)

		tx, err := txFunc()
		if err != nil {
			outErr <- err
			return
		}

		receipt, err := c.GetReceipt(tx)
		if err != nil {
			outErr <- err
		} else {
			outRes <- *receipt
		}

		return
	}()

	return outRes, outErr
}

func (c *Client) ApproveAndTransact(toAddr common.Address, amount *big.Int, txFunc func() (*types.Transaction, error)) (<-chan types.Receipt, <-chan error) {
	timer := time.NewTimer(c.eventTimeout)
	outRes := make(chan types.Receipt)
	outErr := make(chan error)

	go func() {
		defer close(outRes)
		defer close(outErr)

		logCh, sub, err := c.SubscribeToApproval()
		if err != nil {
			outErr <- err
			return
		}

		defer close(logCh)
		defer sub.Unsubscribe()

		tx, err := c.tokenSession.Approve(toAddr, amount)
		if err != nil {
			outErr <- err
			return
		}

		glog.Infof("[%v] Submitted tx %v. Approved %v LPTU for %v", c.account.Address.Hex(), tx.Hash().Hex(), amount, toAddr.Hex())

		select {
		case log := <-logCh:
			if !log.Removed {
				tx, err := txFunc()
				if err != nil {
					// Set approval amount to 0
					_, approveErr := c.tokenSession.Approve(toAddr, big.NewInt(0))
					if approveErr != nil {
						outErr <- approveErr
					} else {
						outErr <- err
					}

					return
				}

				receipt, err := c.GetReceipt(tx)
				if err != nil {
					// Set approval amount to 0
					_, approveErr := c.tokenSession.Approve(toAddr, big.NewInt(0))
					if approveErr != nil {
						outErr <- approveErr
					} else {
						outErr <- err
					}
				} else {
					outRes <- *receipt
				}

				return
			}
		case <-timer.C:
			outErr <- fmt.Errorf("Event subscription timed out")
			return
		}
	}()

	return outRes, outErr
}

func (c *Client) GetControllerAddr() string {
	return c.controllerAddr.Hex()
}

func (c *Client) GetTokenAddr() string {
	return c.tokenAddr.Hex()
}

func (c *Client) GetFaucetAddr() string {
	return c.faucetAddr.Hex()
}

func (c *Client) GetJobsManagerAddr() string {
	return c.jobsManagerAddr.Hex()
}

func (c *Client) GetRoundsManagerAddr() string {
	return c.roundsManagerAddr.Hex()
}

func (c *Client) GetBondingManagerAddr() string {
	return c.bondingManagerAddr.Hex()
}

func (c *Client) GetBlockInfoByTxHash(ctx context.Context, hash common.Hash) (blkNum *big.Int, blkHash common.Hash, err error) {
	blk, _, err := c.Backend().BlockByTxHash(ctx, hash)
	if err != nil {
		return nil, common.Hash{}, err
	}
	return blk.Number(), blk.Hash(), nil
}
