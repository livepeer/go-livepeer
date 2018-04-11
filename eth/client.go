/*
Package eth client is the go client for the Livepeer Ethereum smart contract.  Contracts here are generated.
*/
package eth

//go:generate abigen --abi protocol/abi/Controller.abi --pkg contracts --type Controller --out contracts/controller.go
//go:generate abigen --abi protocol/abi/LivepeerToken.abi --pkg contracts --type LivepeerToken --out contracts/livepeerToken.go
//go:generate abigen --abi protocol/abi/BondingManager.abi --pkg contracts --type BondingManager --out contracts/bondingManager.go
//go:generate abigen --abi protocol/abi/JobsManager.abi --pkg contracts --type JobsManager --out contracts/jobsManager.go
//go:generate abigen --abi protocol/abi/RoundsManager.abi --pkg contracts --type RoundsManager --out contracts/roundsManager.go
//go:generate abigen --abi protocol/abi/Minter.abi --pkg contracts --type Minter --out contracts/minter.go
//go:generate abigen --abi protocol/abi/LivepeerVerifier.abi --pkg contracts --type LivepeerVerifier --out contracts/livepeerVerifier.go
//go:generate abigen --abi protocol/abi/LivepeerTokenFaucet.abi --pkg contracts --type LivepeerTokenFaucet --out contracts/livepeerTokenFaucet.go

import (
	"context"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/eth/contracts"
	lpTypes "github.com/livepeer/go-livepeer/eth/types"
)

var (
	RoundsPerEarningsClaim = big.NewInt(20)

	ErrCurrentRoundLocked = fmt.Errorf("current round locked")
	ErrMissingBackend     = fmt.Errorf("missing Ethereum client backend")
)

type LivepeerEthClient interface {
	Setup(password string, gasLimit, gasPrice *big.Int) error
	Account() accounts.Account
	Backend() (*ethclient.Client, error)

	// Rounds
	InitializeRound() (*types.Transaction, error)
	CurrentRound() (*big.Int, error)
	LastInitializedRound() (*big.Int, error)
	CurrentRoundInitialized() (bool, error)
	CurrentRoundLocked() (bool, error)

	// Token
	Transfer(toAddr common.Address, amount *big.Int) (*types.Transaction, error)
	Request() (*types.Transaction, error)
	BalanceOf(common.Address) (*big.Int, error)
	TotalSupply() (*big.Int, error)

	// Staking
	Transcoder(blockRewardCut *big.Int, feeShare *big.Int, pricePerSegment *big.Int) (*types.Transaction, error)
	Reward() (*types.Transaction, error)
	Bond(amount *big.Int, toAddr common.Address) (*types.Transaction, error)
	Unbond() (*types.Transaction, error)
	WithdrawStake() (*types.Transaction, error)
	WithdrawFees() (*types.Transaction, error)
	ClaimEarnings(endRound *big.Int) error
	GetTranscoder(addr common.Address) (*lpTypes.Transcoder, error)
	GetDelegator(addr common.Address) (*lpTypes.Delegator, error)
	GetTranscoderEarningsPoolForRound(addr common.Address, round *big.Int) (*lpTypes.TokenPools, error)
	RegisteredTranscoders() ([]*lpTypes.Transcoder, error)
	IsActiveTranscoder() (bool, error)
	AssignedTranscoder(jobID *big.Int) (common.Address, error)
	GetTotalBonded() (*big.Int, error)

	// Jobs
	Job(streamId string, transcodingOptions string, maxPricePerSegment *big.Int, endBlock *big.Int) (*types.Transaction, error)
	ClaimWork(jobId *big.Int, segmentRange [2]*big.Int, claimRoot [32]byte) (*types.Transaction, error)
	Verify(jobId *big.Int, claimId *big.Int, segmentNumber *big.Int, dataStorageHash string, dataHashes [2][32]byte, broadcasterSig []byte, proof []byte) (*types.Transaction, error)
	DistributeFees(jobId *big.Int, claimId *big.Int) (*types.Transaction, error)
	Deposit(amount *big.Int) (*types.Transaction, error)
	Withdraw() (*types.Transaction, error)
	GetJob(jobID *big.Int) (*lpTypes.Job, error)
	GetClaim(jobID *big.Int, claimID *big.Int) (*lpTypes.Claim, error)
	BroadcasterDeposit(broadcaster common.Address) (*big.Int, error)
	NumJobs() (*big.Int, error)

	// Parameters
	NumActiveTranscoders() (*big.Int, error)
	RoundLength() (*big.Int, error)
	RoundLockAmount() (*big.Int, error)
	UnbondingPeriod() (uint64, error)
	VerificationRate() (uint64, error)
	VerificationPeriod() (*big.Int, error)
	VerificationSlashingPeriod() (*big.Int, error)
	FailedVerificationSlashAmount() (*big.Int, error)
	MissedVerificationSlashAmount() (*big.Int, error)
	DoubleClaimSegmentSlashAmount() (*big.Int, error)
	FinderFee() (*big.Int, error)
	Inflation() (*big.Int, error)
	InflationChange() (*big.Int, error)
	TargetBondingRate() (*big.Int, error)
	VerificationCodeHash() (string, error)
	Paused() (bool, error)

	// Helpers
	ContractAddresses() map[string]common.Address
	CheckTx(*types.Transaction) error
	Sign([]byte) ([]byte, error)
	LatestBlockNum() (*big.Int, error)
}

type client struct {
	accountManager *AccountManager
	backend        *ethclient.Client

	controllerAddr     common.Address
	tokenAddr          common.Address
	bondingManagerAddr common.Address
	jobsManagerAddr    common.Address
	roundsManagerAddr  common.Address
	minterAddr         common.Address
	verifierAddr       common.Address
	faucetAddr         common.Address

	// Embedded contract sessions
	*contracts.ControllerSession
	*contracts.LivepeerTokenSession
	*contracts.BondingManagerSession
	*contracts.JobsManagerSession
	*contracts.RoundsManagerSession
	*contracts.MinterSession
	*contracts.LivepeerVerifierSession
	*contracts.LivepeerTokenFaucetSession

	nonceInitialized bool
	nextNonce        uint64
	nonceLock        sync.Mutex

	txTimeout time.Duration
}

func NewClient(accountAddr common.Address, keystoreDir string, backend *ethclient.Client, controllerAddr common.Address, txTimeout time.Duration) (LivepeerEthClient, error) {
	am, err := NewAccountManager(accountAddr, keystoreDir)
	if err != nil {
		return nil, err
	}

	return &client{
		accountManager: am,
		backend:        backend,
		controllerAddr: controllerAddr,
		txTimeout:      txTimeout,
	}, nil
}

func (c *client) Setup(password string, gasLimit, gasPrice *big.Int) error {
	err := c.accountManager.Unlock(password)
	if err != nil {
		return err
	}

	opts, err := c.accountManager.CreateTransactOpts(gasLimit, gasPrice)
	if err != nil {
		return err
	}

	opts.NonceFetcher = func() (uint64, error) {
		return c.getNonce()
	}

	return c.setContracts(opts)
}

func (c *client) setContracts(opts *bind.TransactOpts) error {
	controller, err := contracts.NewController(c.controllerAddr, c.backend)
	if err != nil {
		glog.Errorf("Error creating Controller binding: %v", err)
		return err
	}

	c.ControllerSession = &contracts.ControllerSession{
		Contract:     controller,
		TransactOpts: *opts,
	}

	glog.Infof("Controller: %v", c.controllerAddr.Hex())

	tokenAddr, err := c.GetContract(crypto.Keccak256Hash([]byte("LivepeerToken")))
	if err != nil {
		glog.Errorf("Error getting LivepeerToken address: %v", err)
		return err
	}

	c.tokenAddr = tokenAddr

	token, err := contracts.NewLivepeerToken(tokenAddr, c.backend)
	if err != nil {
		glog.Errorf("Error creating LivpeerToken binding: %v", err)
		return err
	}

	c.LivepeerTokenSession = &contracts.LivepeerTokenSession{
		Contract:     token,
		TransactOpts: *opts,
	}

	glog.Infof("LivepeerToken: %v", c.tokenAddr.Hex())

	bondingManagerAddr, err := c.GetContract(crypto.Keccak256Hash([]byte("BondingManager")))
	if err != nil {
		glog.Errorf("Error getting BondingManager address: %v", err)
		return err
	}

	c.bondingManagerAddr = bondingManagerAddr

	bondingManager, err := contracts.NewBondingManager(bondingManagerAddr, c.backend)
	if err != nil {
		glog.Errorf("Error creating BondingManager binding: %v", err)
		return err
	}

	c.BondingManagerSession = &contracts.BondingManagerSession{
		Contract:     bondingManager,
		TransactOpts: *opts,
	}

	glog.Infof("BondingManager: %v", c.bondingManagerAddr.Hex())

	jobsManagerAddr, err := c.GetContract(crypto.Keccak256Hash([]byte("JobsManager")))
	if err != nil {
		glog.Errorf("Error getting JobsManager address: %v", err)
		return err
	}

	c.jobsManagerAddr = jobsManagerAddr

	jobsManager, err := contracts.NewJobsManager(jobsManagerAddr, c.backend)
	if err != nil {
		glog.Errorf("Error creating JobsManager binding: %v", err)
		return err
	}

	c.JobsManagerSession = &contracts.JobsManagerSession{
		Contract:     jobsManager,
		TransactOpts: *opts,
	}

	glog.Infof("JobsManager: %v", c.jobsManagerAddr.Hex())

	roundsManagerAddr, err := c.GetContract(crypto.Keccak256Hash([]byte("RoundsManager")))
	if err != nil {
		glog.Errorf("Error getting RoundsManager address: %v", err)
		return err
	}

	c.roundsManagerAddr = roundsManagerAddr

	roundsManager, err := contracts.NewRoundsManager(roundsManagerAddr, c.backend)
	if err != nil {
		glog.Errorf("Error creating RoundsManager binding: %v", err)
		return err
	}

	c.RoundsManagerSession = &contracts.RoundsManagerSession{
		Contract:     roundsManager,
		TransactOpts: *opts,
	}

	glog.Infof("RoundsManager: %v", c.roundsManagerAddr.Hex())

	minterAddr, err := c.GetContract(crypto.Keccak256Hash([]byte("Minter")))
	if err != nil {
		glog.Errorf("Error getting Minter address: %v", err)
		return err
	}

	c.minterAddr = minterAddr

	minter, err := contracts.NewMinter(minterAddr, c.backend)
	if err != nil {
		glog.Errorf("Error creating Minter binding: %v", err)
		return err
	}

	// Client should never transact with the Minter directly so we don't include transact opts
	c.MinterSession = &contracts.MinterSession{
		Contract: minter,
	}

	glog.Infof("Minter: %v", c.minterAddr.Hex())

	verifierAddr, err := c.GetContract(crypto.Keccak256Hash([]byte("Verifier")))
	if err != nil {
		glog.Errorf("Error getting Verifier address: %v", err)
		return err
	}

	c.verifierAddr = verifierAddr

	verifier, err := contracts.NewLivepeerVerifier(verifierAddr, c.backend)
	if err != nil {
		glog.Errorf("Error creating LivepeerVerifier binding: %v", err)
		return err
	}

	// Client should never transact with the Verifier directly so we don't include transact opts
	c.LivepeerVerifierSession = &contracts.LivepeerVerifierSession{
		Contract: verifier,
	}

	glog.Infof("Verifier: %v", c.verifierAddr.Hex())

	faucetAddr, err := c.GetContract(crypto.Keccak256Hash([]byte("LivepeerTokenFaucet")))
	if err != nil {
		glog.Errorf("Error getting LivepeerTokenFaucet address: %v", err)
		return err
	}

	c.faucetAddr = faucetAddr

	faucet, err := contracts.NewLivepeerTokenFaucet(faucetAddr, c.backend)
	if err != nil {
		glog.Errorf("Error creating LivepeerTokenFaucet binding: %v", err)
		return err
	}

	c.LivepeerTokenFaucetSession = &contracts.LivepeerTokenFaucetSession{
		Contract:     faucet,
		TransactOpts: *opts,
	}

	glog.Infof("LivepeerTokenFaucet: %v", c.faucetAddr.Hex())

	return nil
}

func (c *client) Account() accounts.Account {
	return c.accountManager.Account
}

func (c *client) Backend() (*ethclient.Client, error) {
	if c.backend == nil {
		return nil, ErrMissingBackend
	} else {
		return c.backend, nil
	}
}

// Staking

func (c *client) Transcoder(blockRewardCut, feeShare, pricePerSegment *big.Int) (*types.Transaction, error) {
	locked, err := c.CurrentRoundLocked()
	if err != nil {
		return nil, err
	}

	if locked {
		return nil, ErrCurrentRoundLocked
	} else {
		return c.BondingManagerSession.Transcoder(blockRewardCut, feeShare, pricePerSegment)
	}
}

func (c *client) Bond(amount *big.Int, to common.Address) (*types.Transaction, error) {
	currentRound, err := c.CurrentRound()
	if err != nil {
		return nil, err
	}

	if err := c.autoClaimEarnings(currentRound); err != nil {
		return nil, err
	}

	tx, err := c.Approve(c.bondingManagerAddr, amount)
	if err != nil {
		return nil, err
	}

	err = c.CheckTx(tx)
	if err != nil {
		return nil, err
	}

	return c.BondingManagerSession.Bond(amount, to)
}

func (c *client) Unbond() (*types.Transaction, error) {
	currentRound, err := c.CurrentRound()
	if err != nil {
		return nil, err
	}

	if err := c.autoClaimEarnings(currentRound); err != nil {
		return nil, err
	}

	return c.BondingManagerSession.Unbond()
}

func (c *client) WithdrawStake() (*types.Transaction, error) {
	currentRound, err := c.CurrentRound()
	if err != nil {
		return nil, err
	}

	if err := c.autoClaimEarnings(currentRound); err != nil {
		return nil, err
	}

	return c.BondingManagerSession.WithdrawStake()
}

func (c *client) WithdrawFees() (*types.Transaction, error) {
	currentRound, err := c.CurrentRound()
	if err != nil {
		return nil, err
	}

	if err := c.autoClaimEarnings(currentRound); err != nil {
		return nil, err
	}

	return c.BondingManagerSession.WithdrawFees()
}

func (c *client) ClaimEarnings(endRound *big.Int) error {
	return c.autoClaimEarnings(endRound)
}

func (c *client) autoClaimEarnings(endRound *big.Int) error {
	dStatus, err := c.DelegatorStatus(c.Account().Address)
	if err != nil {
		return err
	}

	if dStatus == 1 {
		// If already bonded, auto claim token pools shares
		dInfo, err := c.BondingManagerSession.GetDelegator(c.Account().Address)
		if err != nil {
			return err
		}

		lastClaimRound := dInfo.LastClaimRound

		var currentEndRound *big.Int
		for new(big.Int).Sub(endRound, lastClaimRound).Cmp(RoundsPerEarningsClaim) == 1 {
			currentEndRound = new(big.Int).Add(lastClaimRound, RoundsPerEarningsClaim)

			tx, err := c.BondingManagerSession.ClaimEarnings(currentEndRound)
			if err != nil {
				return err
			}

			err = c.CheckTx(tx)
			if err != nil {
				return err
			}

			glog.Infof("Claimed rewards and fees from round %v through %v", lastClaimRound, currentEndRound)

			lastClaimRound = currentEndRound
		}

		// Claim for any remaining rounds s.t. the number of rounds < RoundsPerEarningsClaim
		if lastClaimRound.Cmp(endRound) == -1 {
			tx, err := c.BondingManagerSession.ClaimEarnings(endRound)
			if err != nil {
				return err
			}

			err = c.CheckTx(tx)
			if err != nil {
				return err
			}
		}

		glog.Infof("Finished claiming rewards and fees through the end round %v", endRound)
	}

	return nil
}

func (c *client) Deposit(amount *big.Int) (*types.Transaction, error) {
	c.JobsManagerSession.TransactOpts.Value = amount

	tx, err := c.JobsManagerSession.Deposit()
	c.JobsManagerSession.TransactOpts.Value = nil
	return tx, err
}

// Disambiguate between the Verifiy method in JobsManager and in Verifier
func (c *client) Verify(jobId *big.Int, claimId *big.Int, segmentNumber *big.Int, dataStorageHash string, dataHashes [2][32]byte, broadcasterSig []byte, proof []byte) (*types.Transaction, error) {
	return c.JobsManagerSession.Verify(jobId, claimId, segmentNumber, dataStorageHash, dataHashes, broadcasterSig, proof)
}

func (c *client) BroadcasterDeposit(addr common.Address) (*big.Int, error) {
	b, err := c.Broadcasters(addr)
	if err != nil {
		return nil, err
	}

	return b.Deposit, nil
}

func (c *client) IsActiveTranscoder() (bool, error) {
	r, err := c.CurrentRound()
	if err != nil {
		return false, err
	}

	return c.BondingManagerSession.IsActiveTranscoder(c.Account().Address, r)
}

func (c *client) GetTranscoder(addr common.Address) (*lpTypes.Transcoder, error) {
	tInfo, err := c.BondingManagerSession.GetTranscoder(addr)
	if err != nil {
		return nil, err
	}

	tStatus, err := c.TranscoderStatus(addr)
	if err != nil {
		return nil, err
	}

	status, err := lpTypes.ParseTranscoderStatus(tStatus)
	if err != nil {
		return nil, err
	}

	delegatedStake, err := c.TranscoderTotalStake(addr)
	if err != nil {
		return nil, err
	}

	currentRound, err := c.CurrentRound()
	if err != nil {
		return nil, err
	}

	active, err := c.BondingManagerSession.IsActiveTranscoder(addr, currentRound)
	if err != nil {
		return nil, err
	}

	return &lpTypes.Transcoder{
		Address:                addr,
		LastRewardRound:        tInfo.LastRewardRound,
		RewardCut:              tInfo.RewardCut,
		FeeShare:               tInfo.FeeShare,
		PricePerSegment:        tInfo.PricePerSegment,
		PendingRewardCut:       tInfo.PendingRewardCut,
		PendingFeeShare:        tInfo.PendingFeeShare,
		PendingPricePerSegment: tInfo.PendingPricePerSegment,
		DelegatedStake:         delegatedStake,
		Active:                 active,
		Status:                 status,
	}, nil
}

func (c *client) GetTranscoderEarningsPoolForRound(addr common.Address, round *big.Int) (*lpTypes.TokenPools, error) {
	tp, err := c.BondingManagerSession.GetTranscoderEarningsPoolForRound(addr, round)
	if err != nil {
		return nil, err
	}

	return &lpTypes.TokenPools{
		RewardPool:     tp.RewardPool,
		FeePool:        tp.FeePool,
		TotalStake:     tp.TotalStake,
		ClaimableStake: tp.ClaimableStake,
	}, nil
}

func (c *client) GetDelegator(addr common.Address) (*lpTypes.Delegator, error) {
	dInfo, err := c.BondingManagerSession.GetDelegator(addr)
	if err != nil {
		glog.Infof("Error getting delegator from bonding manager: %v", err)
		return nil, err
	}

	dStatus, err := c.DelegatorStatus(addr)
	if err != nil {
		glog.Infof("Error getting status: %v", err)
		return nil, err
	}

	status, err := lpTypes.ParseDelegatorStatus(dStatus)
	if err != nil {
		return nil, err
	}
	currentRound, err := c.CurrentRound()
	if err != nil {
		glog.Infof("Error getting current round: %v", err)
		return nil, err
	}

	pendingStake, err := c.PendingStake(addr, currentRound)
	if err != nil {
		if err.Error() == "abi: unmarshalling empty output" {
			pendingStake = big.NewInt(-1)
		} else {
			glog.Infof("Error getting pending stake: %v", err)
			return nil, err
		}
	}

	pendingFees, err := c.PendingFees(addr, currentRound)
	if err != nil {
		if err.Error() == "abi: unmarshalling empty output" {
			pendingFees = big.NewInt(-1)
		} else {
			glog.Infof("Error getting pending fees: %v", err)
			return nil, err
		}
	}

	return &lpTypes.Delegator{
		Address:         addr,
		BondedAmount:    dInfo.BondedAmount,
		Fees:            dInfo.Fees,
		DelegateAddress: dInfo.DelegateAddress,
		DelegatedAmount: dInfo.DelegatedAmount,
		StartRound:      dInfo.StartRound,
		WithdrawRound:   dInfo.WithdrawRound,
		LastClaimRound:  dInfo.LastClaimRound,
		PendingStake:    pendingStake,
		PendingFees:     pendingFees,
		Status:          status,
	}, nil
}

func (c *client) GetJob(jobID *big.Int) (*lpTypes.Job, error) {
	jInfo, err := c.JobsManagerSession.GetJob(jobID)
	if err != nil {
		return nil, err
	}

	jStatus, err := c.JobStatus(jobID)
	if err != nil {
		return nil, err
	}

	status, err := lpTypes.ParseJobStatus(jStatus)
	if err != nil {
		return nil, err
	}

	return &lpTypes.Job{
		JobId:              jobID,
		StreamId:           jInfo.StreamId,
		MaxPricePerSegment: jInfo.MaxPricePerSegment,
		BroadcasterAddress: jInfo.BroadcasterAddress,
		TranscodingOptions: jInfo.TranscodingOptions,
		TranscoderAddress:  jInfo.TranscoderAddress,
		CreationRound:      jInfo.CreationRound,
		CreationBlock:      jInfo.CreationBlock,
		EndBlock:           jInfo.EndBlock,
		Escrow:             jInfo.Escrow,
		Status:             status,
	}, nil
}

func (c *client) GetClaim(jobID *big.Int, claimID *big.Int) (*lpTypes.Claim, error) {
	cInfo, err := c.JobsManagerSession.GetClaim(jobID, claimID)
	if err != nil {
		return nil, err
	}

	status, err := lpTypes.ParseClaimStatus(cInfo.Status)
	if err != nil {
		glog.Infof("%v", cInfo)
		return nil, err
	}

	return &lpTypes.Claim{
		ClaimId:                      claimID,
		SegmentRange:                 cInfo.SegmentRange,
		ClaimRoot:                    cInfo.ClaimRoot,
		ClaimBlock:                   cInfo.ClaimBlock,
		EndVerificationBlock:         cInfo.EndVerificationBlock,
		EndVerificationSlashingBlock: cInfo.EndVerificationSlashingBlock,
		Status: status,
	}, nil
}

func (c *client) Paused() (bool, error) {
	return c.ControllerSession.Paused()
}

func (c *client) RegisteredTranscoders() ([]*lpTypes.Transcoder, error) {
	var transcoders []*lpTypes.Transcoder

	tAddr, err := c.GetFirstTranscoderInPool()
	if err != nil {
		return nil, err
	}

	for !IsNullAddress(tAddr) {
		t, err := c.GetTranscoder(tAddr)
		if err != nil {
			return nil, err
		}

		transcoders = append(transcoders, t)

		tAddr, err = c.GetNextTranscoderInPool(tAddr)
		if err != nil {
			return nil, err
		}
	}

	return transcoders, nil
}

func (c *client) AssignedTranscoder(jobID *big.Int) (common.Address, error) {
	jInfo, err := c.JobsManagerSession.GetJob(jobID)
	if err != nil {
		glog.Errorf("Error getting job: %v", err)
		return common.Address{}, err
	}

	var blk *types.Block
	getBlock := func() error {
		blk, err = c.backend.BlockByNumber(context.Background(), jInfo.CreationBlock)
		if err != nil {
			glog.Errorf("Error getting block by number %v: %v. retrying...", jInfo.CreationBlock.String(), err)
			return err
		}

		return nil
	}
	if err := backoff.Retry(getBlock, backoff.WithMaxRetries(backoff.NewConstantBackOff(time.Second), SubscribeRetry)); err != nil {
		glog.Errorf("BlockByNumber failed: %v", err)
		return common.Address{}, err
	}

	t, err := c.BondingManagerSession.ElectActiveTranscoder(jInfo.MaxPricePerSegment, blk.Hash(), jInfo.CreationRound)
	if err != nil {
		glog.Errorf("Error getting ElectActiveTranscoder: %v", err)
		return common.Address{}, err
	}

	return t, nil
}

// Helpers

func (c *client) ContractAddresses() map[string]common.Address {
	addrMap := make(map[string]common.Address)
	addrMap["Controller"] = c.controllerAddr
	addrMap["LivepeerToken"] = c.tokenAddr
	addrMap["LivepeerTokenFaucet"] = c.faucetAddr
	addrMap["JobsManager"] = c.jobsManagerAddr
	addrMap["RoundsManager"] = c.roundsManagerAddr
	addrMap["BondingManager"] = c.bondingManagerAddr
	addrMap["Minter"] = c.minterAddr
	addrMap["Verifier"] = c.verifierAddr

	return addrMap
}

func (c *client) CheckTx(tx *types.Transaction) error {
	ctx, cancel := context.WithTimeout(context.Background(), c.txTimeout)
	defer cancel()

	receipt, err := bind.WaitMined(ctx, c.backend, tx)
	if err != nil {
		return err
	}

	if receipt.Status == uint(0) {
		return fmt.Errorf("tx %v failed", tx.Hash().Hex())
	} else {
		return nil
	}
}

func (c *client) Sign(msg []byte) ([]byte, error) {
	return c.accountManager.Sign(msg)
}

func (c *client) getNonce() (uint64, error) {
	c.nonceLock.Lock()
	defer c.nonceLock.Unlock()

	if !c.nonceInitialized {
		netNonce, err := c.backend.PendingNonceAt(context.Background(), c.Account().Address)
		if err != nil {
			return 0, err
		}

		c.nonceInitialized = true
		c.nextNonce = netNonce

		return c.nextNonce, nil
	} else {
		c.nextNonce++

		netNonce, err := c.backend.PendingNonceAt(context.Background(), c.Account().Address)
		if err != nil {
			return 0, err
		}

		if netNonce > c.nextNonce {
			c.nextNonce = netNonce
		}

		return c.nextNonce, nil
	}
}

func (c *client) LatestBlockNum() (*big.Int, error) {
	var blk *types.Header
	var err error
	getBlock := func() error {
		blk, err = c.backend.HeaderByNumber(context.Background(), nil)
		if err != nil {
			return err
		}
		return nil
	}
	if err := backoff.Retry(getBlock, backoff.NewConstantBackOff(time.Second*2)); err != nil {
		glog.Errorf("Cannot get current block number: %v", err)
		return nil, err
	}
	return blk.Number, nil
}
