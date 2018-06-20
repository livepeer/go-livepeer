/*
Package eth client is the go client for the Livepeer Ethereum smart contract.  Contracts here are generated.
*/
package eth

//go:generate abigen --abi protocol/abi/Controller.abi --pkg contracts --type Controller --out contracts/controller.go
//go:generate abigen --abi protocol/abi/LivepeerToken.abi --pkg contracts --type LivepeerToken --out contracts/livepeerToken.go
//go:generate abigen --abi protocol/abi/ServiceRegistry.abi --pkg contracts --type ServiceRegistry --out contracts/serviceRegistry.go
//go:generate abigen --abi protocol/abi/BondingManager.abi --pkg contracts --type BondingManager --out contracts/bondingManager.go
//go:generate abigen --abi protocol/abi/JobsManager.abi --pkg contracts --type JobsManager --out contracts/jobsManager.go
//go:generate abigen --abi protocol/abi/RoundsManager.abi --pkg contracts --type RoundsManager --out contracts/roundsManager.go
//go:generate abigen --abi protocol/abi/Minter.abi --pkg contracts --type Minter --out contracts/minter.go
//go:generate abigen --abi protocol/abi/LivepeerVerifier.abi --pkg contracts --type LivepeerVerifier --out contracts/livepeerVerifier.go
//go:generate abigen --abi protocol/abi/LivepeerTokenFaucet.abi --pkg contracts --type LivepeerTokenFaucet --out contracts/livepeerTokenFaucet.go

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/cenkalti/backoff"
	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/eth/contracts"
	lpTypes "github.com/livepeer/go-livepeer/eth/types"
)

var (
	ErrReplacingMinedTx   = fmt.Errorf("trying to replace already mined tx")
	ErrCurrentRoundLocked = fmt.Errorf("current round locked")
	ErrMissingBackend     = fmt.Errorf("missing Ethereum client backend")
)

type LivepeerEthClient interface {
	Setup(password string, gasLimit uint64, gasPrice *big.Int) error
	Account() accounts.Account
	Backend() (*ethclient.Client, error)

	// Rounds
	InitializeRound() (*types.Transaction, error)
	CurrentRound() (*big.Int, error)
	LastInitializedRound() (*big.Int, error)
	CurrentRoundInitialized() (bool, error)
	CurrentRoundLocked() (bool, error)

	// Token
	Transfer(toAddr ethcommon.Address, amount *big.Int) (*types.Transaction, error)
	Request() (*types.Transaction, error)
	BalanceOf(ethcommon.Address) (*big.Int, error)
	TotalSupply() (*big.Int, error)

	// Service Registry
	SetServiceURI(serviceURI string) (*types.Transaction, error)
	GetServiceURI(addr ethcommon.Address) (string, error)

	// Staking
	Transcoder(blockRewardCut *big.Int, feeShare *big.Int, pricePerSegment *big.Int) (*types.Transaction, error)
	Reward() (*types.Transaction, error)
	Bond(amount *big.Int, toAddr ethcommon.Address) (*types.Transaction, error)
	Unbond() (*types.Transaction, error)
	WithdrawStake() (*types.Transaction, error)
	WithdrawFees() (*types.Transaction, error)
	ClaimEarnings(endRound *big.Int) error
	GetTranscoder(addr ethcommon.Address) (*lpTypes.Transcoder, error)
	GetDelegator(addr ethcommon.Address) (*lpTypes.Delegator, error)
	GetTranscoderEarningsPoolForRound(addr ethcommon.Address, round *big.Int) (*lpTypes.TokenPools, error)
	RegisteredTranscoders() ([]*lpTypes.Transcoder, error)
	IsActiveTranscoder() (bool, error)
	AssignedTranscoder(*lpTypes.Job) (ethcommon.Address, error)
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
	BroadcasterDeposit(broadcaster ethcommon.Address) (*big.Int, error)
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

	WatchForJob(string) (*lpTypes.Job, error)

	// Helpers
	ContractAddresses() map[string]ethcommon.Address
	CheckTx(*types.Transaction) error
	ReplaceTransaction(*types.Transaction, string, *big.Int) (*types.Transaction, error)
	Sign([]byte) ([]byte, error)
	LatestBlockNum() (*big.Int, error)
	GetGasInfo() (uint64, *big.Int)
	SetGasInfo(uint64, *big.Int) error
}

type client struct {
	accountManager *AccountManager
	backend        *ethclient.Client

	controllerAddr      ethcommon.Address
	tokenAddr           ethcommon.Address
	serviceRegistryAddr ethcommon.Address
	bondingManagerAddr  ethcommon.Address
	jobsManagerAddr     ethcommon.Address
	roundsManagerAddr   ethcommon.Address
	minterAddr          ethcommon.Address
	verifierAddr        ethcommon.Address
	faucetAddr          ethcommon.Address

	// Embedded contract sessions
	*contracts.ControllerSession
	*contracts.LivepeerTokenSession
	*contracts.ServiceRegistrySession
	*contracts.BondingManagerSession
	*contracts.JobsManagerSession
	*contracts.RoundsManagerSession
	*contracts.MinterSession
	*contracts.LivepeerVerifierSession
	*contracts.LivepeerTokenFaucetSession

	nonceInitialized bool
	nextNonce        uint64
	nonceLock        sync.Mutex
	gasLimit         uint64
	gasPrice         *big.Int

	txTimeout time.Duration
}

func NewClient(accountAddr ethcommon.Address, keystoreDir string, backend *ethclient.Client, controllerAddr ethcommon.Address, txTimeout time.Duration) (LivepeerEthClient, error) {
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

func (c *client) Setup(password string, gasLimit uint64, gasPrice *big.Int) error {
	err := c.accountManager.Unlock(password)
	if err != nil {
		return err
	}

	return c.SetGasInfo(gasLimit, gasPrice)
}

func (c *client) SetGasInfo(gasLimit uint64, gasPrice *big.Int) error {
	opts, err := c.accountManager.CreateTransactOpts(gasLimit, gasPrice)
	if err != nil {
		return err
	}

	opts.NonceFetcher = func() (uint64, error) {
		return c.getNonce()
	}

	if err := c.setContracts(opts); err != nil {
		return err
	} else {
		c.gasLimit = gasLimit
		c.gasPrice = gasPrice
		return nil
	}
}

func (c *client) GetGasInfo() (gasLimit uint64, gasPrice *big.Int) {
	return c.gasLimit, c.gasPrice
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

	glog.V(common.SHORT).Infof("Controller: %v", c.controllerAddr.Hex())

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

	glog.V(common.SHORT).Infof("LivepeerToken: %v", c.tokenAddr.Hex())

	serviceRegistryAddr, err := c.GetContract(crypto.Keccak256Hash([]byte("ServiceRegistry")))
	if err != nil {
		glog.Errorf("Error getting ServiceRegistry address: %v", err)
		return err
	}

	c.serviceRegistryAddr = serviceRegistryAddr

	serviceRegistry, err := contracts.NewServiceRegistry(serviceRegistryAddr, c.backend)
	if err != nil {
		glog.Errorf("Error creating ServiceRegistry binding: %v", err)
		return err
	}

	c.ServiceRegistrySession = &contracts.ServiceRegistrySession{
		Contract:     serviceRegistry,
		TransactOpts: *opts,
	}

	glog.V(common.SHORT).Infof("ServiceRegistry: %v", c.serviceRegistryAddr.Hex())

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

	glog.V(common.SHORT).Infof("BondingManager: %v", c.bondingManagerAddr.Hex())

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

	glog.V(common.SHORT).Infof("JobsManager: %v", c.jobsManagerAddr.Hex())

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

	glog.V(common.SHORT).Infof("RoundsManager: %v", c.roundsManagerAddr.Hex())

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

	glog.V(common.SHORT).Infof("Minter: %v", c.minterAddr.Hex())

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

	glog.V(common.SHORT).Infof("Verifier: %v", c.verifierAddr.Hex())

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

	glog.V(common.SHORT).Infof("LivepeerTokenFaucet: %v", c.faucetAddr.Hex())

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

// Rounds
func (c *client) InitializeRound() (*types.Transaction, error) {
	i, err := c.RoundsManagerSession.CurrentRoundInitialized()
	if err != nil {
		return nil, err
	}
	if i {
		glog.V(common.SHORT).Infof("Round already initialized")
		return nil, errors.New("ErrRoundInitialized")
	} else {
		return c.RoundsManagerSession.InitializeRound()
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

func (c *client) Bond(amount *big.Int, to ethcommon.Address) (*types.Transaction, error) {
	currentRound, err := c.CurrentRound()
	if err != nil {
		return nil, err
	}

	if err := c.autoClaimEarnings(currentRound, false); err != nil {
		return nil, err
	}

	allowance, err := c.Allowance(c.Account().Address, c.bondingManagerAddr)
	if err != nil {
		return nil, err
	}

	// If existing allowance set by account for BondingManager is
	// less than the bond amount, approve the necessary amount
	if allowance.Cmp(amount) == -1 {
		tx, err := c.Approve(c.bondingManagerAddr, amount)
		if err != nil {
			return nil, err
		}

		err = c.CheckTx(tx)
		if err != nil {
			return nil, err
		}
	}

	return c.BondingManagerSession.Bond(amount, to)
}

func (c *client) Unbond() (*types.Transaction, error) {
	currentRound, err := c.CurrentRound()
	if err != nil {
		return nil, err
	}

	if err := c.autoClaimEarnings(currentRound, false); err != nil {
		return nil, err
	}

	return c.BondingManagerSession.Unbond()
}

func (c *client) WithdrawStake() (*types.Transaction, error) {
	currentRound, err := c.CurrentRound()
	if err != nil {
		return nil, err
	}

	if err := c.autoClaimEarnings(currentRound, false); err != nil {
		return nil, err
	}

	return c.BondingManagerSession.WithdrawStake()
}

func (c *client) WithdrawFees() (*types.Transaction, error) {
	currentRound, err := c.CurrentRound()
	if err != nil {
		return nil, err
	}

	if err := c.autoClaimEarnings(currentRound, false); err != nil {
		return nil, err
	}

	return c.BondingManagerSession.WithdrawFees()
}

func (c *client) ClaimEarnings(endRound *big.Int) error {
	return c.autoClaimEarnings(endRound, true)
}

func (c *client) autoClaimEarnings(endRound *big.Int, allRounds bool) error {
	dStatus, err := c.DelegatorStatus(c.Account().Address)
	if err != nil {
		return err
	}

	maxRoundsPerClaim, err := c.MaxEarningsClaimsRounds()
	if err != nil {
		return err
	}

	if dStatus == 1 {
		dInfo, err := c.BondingManagerSession.GetDelegator(c.Account().Address)
		if err != nil {
			return err
		}

		lastClaimRound := dInfo.LastClaimRound

		var currentEndRound *big.Int

		// Keep claiming `maxRoundsPerClaim` at a time until there are <= `maxRoundsPerClaim` rounds
		// At this point, any subsequent bonding action will automatically claim through the rest of the rounds
		// so we do not need to submit an additional `claimEarnings()` transaction
		for new(big.Int).Sub(endRound, lastClaimRound).Cmp(maxRoundsPerClaim) == 1 {
			currentEndRound = new(big.Int).Add(lastClaimRound, maxRoundsPerClaim)

			tx, err := c.BondingManagerSession.ClaimEarnings(currentEndRound)
			if err != nil {
				return err
			}

			err = c.CheckTx(tx)
			if err != nil {
				return err
			}

			glog.V(common.SHORT).Infof("Claimed earnings from round %v through %v", lastClaimRound, currentEndRound)

			lastClaimRound = currentEndRound
		}

		// If `allRounds` is true and there are remaining rounds to be claimed through, claim earnings for them
		if allRounds && lastClaimRound.Cmp(endRound) == -1 {
			tx, err := c.BondingManagerSession.ClaimEarnings(endRound)
			if err != nil {
				return err
			}

			err = c.CheckTx(tx)
			if err != nil {
				return err
			}

			glog.V(common.SHORT).Infof("Finished claiming earnings through the end round %v", endRound)
		} else {
			glog.V(common.SHORT).Infof("Finished claiming earnings through round %v. Remaining rounds can be automatically claimed through a bonding action", lastClaimRound)
		}
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

func (c *client) BroadcasterDeposit(addr ethcommon.Address) (*big.Int, error) {
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

func (c *client) GetTranscoder(addr ethcommon.Address) (*lpTypes.Transcoder, error) {
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

	serviceURI, err := c.GetServiceURI(addr)
	if err != nil {
		return nil, err
	}

	return &lpTypes.Transcoder{
		Address:                addr,
		ServiceURI:             serviceURI,
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

func (c *client) GetTranscoderEarningsPoolForRound(addr ethcommon.Address, round *big.Int) (*lpTypes.TokenPools, error) {
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

func (c *client) GetDelegator(addr ethcommon.Address) (*lpTypes.Delegator, error) {
	dInfo, err := c.BondingManagerSession.GetDelegator(addr)
	if err != nil {
		glog.Errorf("Error getting delegator from bonding manager: %v", err)
		return nil, err
	}

	dStatus, err := c.DelegatorStatus(addr)
	if err != nil {
		glog.Errorf("Error getting status: %v", err)
		return nil, err
	}

	status, err := lpTypes.ParseDelegatorStatus(dStatus)
	if err != nil {
		return nil, err
	}
	currentRound, err := c.CurrentRound()
	if err != nil {
		glog.Errorf("Error getting current round: %v", err)
		return nil, err
	}

	pendingStake, err := c.PendingStake(addr, currentRound)
	if err != nil {
		if err.Error() == "abi: unmarshalling empty output" {
			pendingStake = big.NewInt(-1)
		} else {
			glog.Errorf("Error getting pending stake: %v", err)
			return nil, err
		}
	}

	pendingFees, err := c.PendingFees(addr, currentRound)
	if err != nil {
		if err.Error() == "abi: unmarshalling empty output" {
			pendingFees = big.NewInt(-1)
		} else {
			glog.Errorf("Error getting pending fees: %v", err)
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

	profiles, err := common.TxDataToVideoProfile(jInfo.TranscodingOptions)
	if err != nil {
		return nil, err
	}

	return &lpTypes.Job{
		JobId:              jobID,
		StreamId:           jInfo.StreamId,
		Profiles:           profiles,
		MaxPricePerSegment: jInfo.MaxPricePerSegment,
		BroadcasterAddress: jInfo.BroadcasterAddress,
		TranscoderAddress:  jInfo.TranscoderAddress,
		CreationRound:      jInfo.CreationRound,
		CreationBlock:      jInfo.CreationBlock,
		EndBlock:           jInfo.EndBlock,
		Escrow:             jInfo.Escrow,
		TotalClaims:        jInfo.TotalClaims,
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
		glog.V(common.SHORT).Infof("%v", cInfo)
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

func (c *client) AssignedTranscoder(jInfo *lpTypes.Job) (ethcommon.Address, error) {
	var blk *types.Block
	getBlock := func() error {
		var err error
		blk, err = c.backend.BlockByNumber(context.Background(), jInfo.CreationBlock)
		if err != nil {
			glog.Errorf("Error getting block by number %v: %v. retrying...", jInfo.CreationBlock.String(), err)
			return err
		}

		return nil
	}
	if err := backoff.Retry(getBlock, backoff.WithMaxRetries(backoff.NewConstantBackOff(time.Second), SubscribeRetry)); err != nil {
		glog.Errorf("BlockByNumber failed: %v", err)
		return ethcommon.Address{}, err
	}

	t, err := c.BondingManagerSession.ElectActiveTranscoder(jInfo.MaxPricePerSegment, blk.Hash(), jInfo.CreationRound)
	if err != nil {
		glog.Errorf("Error getting ElectActiveTranscoder: %v", err)
		return ethcommon.Address{}, err
	}

	return t, nil
}

// Helpers

func (c *client) ContractAddresses() map[string]ethcommon.Address {
	addrMap := make(map[string]ethcommon.Address)
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

func (c *client) ReplaceTransaction(tx *types.Transaction, method string, gasPrice *big.Int) (*types.Transaction, error) {
	_, pending, err := c.backend.TransactionByHash(context.Background(), tx.Hash())
	// Only return here if the error is not related to the tx not being found
	// Presumably the provided tx was already broadcasted at some point, so even if for some reason the
	// node being used cannot find it, the originally broadcasted tx is still valid and might be sitting somewhere
	if err != nil && err != ethereum.NotFound {
		return nil, err
	}
	// If tx was found
	// If `pending` is true, the tx was mined and included in a block
	if err == nil && !pending {
		return nil, ErrReplacingMinedTx
	}

	// Updated gas price must be at least 10% greater than the gas price used for the original transaction in order
	// to submit a replacement transaction with the same nonce. 10% is not defined by the protocol, but is the default required price bump
	// used by many clients: https://github.com/ethereum/go-ethereum/blob/01a7e267dc6d7bbef94882542bbd01bd712f5548/core/tx_pool.go#L148
	// We add a little extra in addition to the 10% price bump just to be sure
	minGasPrice := big.NewInt(0).Add(big.NewInt(0).Add(tx.GasPrice(), big.NewInt(0).Div(tx.GasPrice(), big.NewInt(10))), big.NewInt(10))

	// If gas price is not provided, use minimum gas price that satisfies the 10% required price bump
	if gasPrice == nil {
		gasPrice = minGasPrice

		suggestedGasPrice, err := c.backend.SuggestGasPrice(context.Background())
		if err != nil {
			return nil, err
		}

		// If the suggested gas price is higher than the bumped gas price, use the suggested gas price
		// This is to account for any wild market gas price increases between the time of the original tx submission and time
		// of replacement tx submission
		// Note: If the suggested gas price is lower than the bumped gas price because market gas prices have dropped
		// since the time of the original tx submission we cannot use the lower suggested gas price and we still need to use
		// the bumped gas price in order to properly replace a still pending tx
		if suggestedGasPrice.Cmp(gasPrice) == 1 {
			gasPrice = suggestedGasPrice
		}
	}

	// Check that gas price meets minimum price bump requirement
	if gasPrice.Cmp(minGasPrice) == -1 {
		return nil, fmt.Errorf("Provided gas price does not satisfy required price bump to replace transaction %v", tx.Hash())
	}

	// Replacement raw tx uses same fields as old tx (reusing the same nonce is crucial) except the gas price is updated
	newRawTx := types.NewTransaction(tx.Nonce(), *tx.To(), tx.Value(), tx.Gas(), gasPrice, tx.Data())

	newSignedTx, err := c.accountManager.SignTx(types.HomesteadSigner{}, newRawTx)
	if err != nil {
		return nil, err
	}

	err = c.backend.SendTransaction(context.Background(), newSignedTx)
	if err == nil {
		glog.Infof("\n%vEth Transaction%v\n\nReplacement transaction: \"%v\".  Hash: \"%v\".  Gas Price: %v \n\n%v\n", strings.Repeat("*", 30), strings.Repeat("*", 30), method, newSignedTx.Hash().String(), newSignedTx.GasPrice().String(), strings.Repeat("*", 75))
	} else {
		glog.Infof("\n%vEth Transaction%v\n\nReplacement transaction: \"%v\".  Gas Price: %v \nTransaction Failed: %v\n\n%v\n", strings.Repeat("*", 30), strings.Repeat("*", 30), method, newSignedTx.GasPrice().String(), err, strings.Repeat("*", 75))
	}

	return newSignedTx, err
}

// Watch for a new job matching the given streamId.
// Since this job will be fresh, not all fields will be populated!
// After receiving the job, validate the fields that are expected.
func (c *client) WatchForJob(streamId string) (*lpTypes.Job, error) {
	ctx, cancel := context.WithTimeout(context.Background(), c.txTimeout)
	defer cancel()
	var job *lpTypes.Job
	jobWatcher := func() error {
		sink := make(chan *contracts.JobsManagerNewJob)
		sub, err := c.JobsManagerSession.Contract.JobsManagerFilterer.WatchNewJob(nil, sink, []ethcommon.Address{c.Account().Address})
		if err != nil {
			glog.Error("Unable to start job watcher ", err)
			return err
		}
		select {
		case newJob := <-sink:
			sub.Unsubscribe()
			if newJob.StreamId == streamId {
				// might be faster to reconstruct the job locally?
				j, err := c.GetJob(newJob.JobId)
				if err != nil {
					glog.Error("Unable to fetch job after watching: ", err)
					// maybe perform/retry the job lookup outside this loop
					// but a retry may be unlikely to succeed, depending on the error
					// Manually create the job for now.
					// May have important fields missing!!!
					profiles, err := common.TxDataToVideoProfile(newJob.TranscodingOptions)
					if err != nil {
						glog.Error("Invalid transcoding options for job")
					}
					j = &lpTypes.Job{
						BroadcasterAddress: newJob.Broadcaster,
						StreamId:           newJob.StreamId,
						MaxPricePerSegment: newJob.MaxPricePerSegment,
						CreationBlock:      newJob.CreationBlock,
						Profiles:           profiles,
					}
				}
				job = j
				return nil
			}
			// mismatched streamid; maybe we had concurrent listeners so retry
			glog.Errorf("Watched for job; got mismatched stream Id %v; expecting %v",
				newJob.StreamId, streamId)
			return fmt.Errorf("MismatchedStreamId")
		case errChan := <-sub.Err():
			sub.Unsubscribe()
			glog.Errorf("Error subscribing to new job %v; retrying", errChan)
			return fmt.Errorf("SubscribeError")
		case <-ctx.Done():
			sub.Unsubscribe()
			glog.Errorf("Job watcher timeout exceeded; stopping")
			return fmt.Errorf("JobWatchTimeout")
		}
	}

	err := backoff.Retry(jobWatcher, backoff.NewConstantBackOff(time.Second*2))
	return job, err
}

func (c *client) getNonce() (uint64, error) {
	c.nonceLock.Lock()
	defer c.nonceLock.Unlock()

	if !c.nonceInitialized {
		nextNonce, err := c.backend.PendingNonceAt(context.Background(), c.Account().Address)
		if err != nil {
			return 0, err
		}

		c.nonceInitialized = true
		c.nextNonce = nextNonce

		return c.nextNonce, nil
	} else {
		c.nextNonce++

		nextNonce, err := c.backend.PendingNonceAt(context.Background(), c.Account().Address)
		if err != nil {
			return 0, err
		}

		if nextNonce > c.nextNonce {
			c.nextNonce = nextNonce
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
