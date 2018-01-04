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
	"context"
	"fmt"
	"math/big"
	"time"

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
	RoundsPerTokenSharesClaim = big.NewInt(20)

	ErrCurrentRoundLocked = fmt.Errorf("current round locked")
)

type LivepeerEthClient interface {
	Setup(gasLimit, gasPrice *big.Int) error
	Account() accounts.Account
	Backend() *ethclient.Client

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

	// Staking
	Transcoder(blockRewardCut *big.Int, feeShare *big.Int, pricePerSegment *big.Int) (*types.Transaction, error)
	Reward() (*types.Transaction, error)
	Bond(amount *big.Int, toAddr common.Address) (*types.Transaction, error)
	Unbond() (*types.Transaction, error)
	WithdrawStake() (*types.Transaction, error)
	WithdrawFees() (*types.Transaction, error)
	ClaimTokenPoolsShares(endRound *big.Int) (*types.Transaction, error)
	GetTranscoder(addr common.Address) (*lpTypes.Transcoder, error)
	GetDelegator(addr common.Address) (*lpTypes.Delegator, error)
	GetTranscoderTokenPoolsForRound(addr common.Address, round *big.Int) (*lpTypes.TokenPools, error)
	RegisteredTranscoders() ([]*lpTypes.Transcoder, error)
	IsActiveTranscoder() (bool, error)
	IsAssignedTranscoder(jobID, maxPricePerSegment *big.Int) (bool, error)

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

	// Parameters
	RoundLength() (*big.Int, error)
	UnbondingPeriod() (uint64, error)
	VerificationRate() (uint64, error)
	VerificationPeriod() (*big.Int, error)
	SlashingPeriod() (*big.Int, error)

	// Helpers
	ContractAddresses() map[string]common.Address
	CheckTx(*types.Transaction) error
	Sign([]byte) ([]byte, error)
}

type client struct {
	accountManager *AccountManager
	backend        *ethclient.Client

	controllerAddr     common.Address
	tokenAddr          common.Address
	bondingManagerAddr common.Address
	jobsManagerAddr    common.Address
	roundsManagerAddr  common.Address
	faucetAddr         common.Address

	// Embedded contract sessions
	*contracts.ControllerSession
	*contracts.LivepeerTokenSession
	*contracts.BondingManagerSession
	*contracts.JobsManagerSession
	*contracts.RoundsManagerSession
	*contracts.LivepeerTokenFaucetSession

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

func (c *client) Setup(gasLimit, gasPrice *big.Int) error {
	err := c.accountManager.Unlock()
	if err != nil {
		return err
	}

	opts, err := c.accountManager.CreateTransactOpts(gasLimit, gasPrice)
	if err != nil {
		return err
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

func (c *client) Backend() *ethclient.Client {
	return c.backend
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
	if err := c.autoClaimTokenPoolsShares(); err != nil {
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
	if err := c.autoClaimTokenPoolsShares(); err != nil {
		return nil, err
	}

	return c.BondingManagerSession.Unbond()
}

func (c *client) WithdrawStake() (*types.Transaction, error) {
	if err := c.autoClaimTokenPoolsShares(); err != nil {
		return nil, err
	}

	return c.BondingManagerSession.WithdrawStake()
}

func (c *client) WithdrawFees() (*types.Transaction, error) {
	if err := c.autoClaimTokenPoolsShares(); err != nil {
		return nil, err
	}

	return c.BondingManagerSession.WithdrawFees()
}

func (c *client) autoClaimTokenPoolsShares() error {
	dStatus, err := c.DelegatorStatus(c.Account().Address)
	if err != nil {
		return err
	}

	if dStatus != 0 {
		// If already registered, auto claim token pools shares
		currentRound, err := c.CurrentRound()
		if err != nil {
			return err
		}

		dInfo, err := c.BondingManagerSession.GetDelegator(c.Account().Address)
		if err != nil {
			return err
		}

		lastClaimTokenPoolsSharesRound := dInfo.LastClaimTokenPoolsSharesRound

		var endRound *big.Int
		for new(big.Int).Sub(currentRound, lastClaimTokenPoolsSharesRound).Cmp(RoundsPerTokenSharesClaim) == 1 {
			endRound = new(big.Int).Add(lastClaimTokenPoolsSharesRound, RoundsPerTokenSharesClaim)

			tx, err := c.ClaimTokenPoolsShares(endRound)
			if err != nil {
				return err
			}

			err = c.CheckTx(tx)
			if err != nil {
				return err
			}

			lastClaimTokenPoolsSharesRound = endRound
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
		BlockRewardCut:         tInfo.BlockRewardCut,
		FeeShare:               tInfo.FeeShare,
		PricePerSegment:        tInfo.PricePerSegment,
		PendingBlockRewardCut:  tInfo.PendingBlockRewardCut,
		PendingFeeShare:        tInfo.PendingFeeShare,
		PendingPricePerSegment: tInfo.PendingPricePerSegment,
		DelegatedStake:         delegatedStake,
		Active:                 active,
		Status:                 status,
	}, nil
}

func (c *client) GetTranscoderTokenPoolsForRound(addr common.Address, round *big.Int) (*lpTypes.TokenPools, error) {
	tp, err := c.BondingManagerSession.GetTranscoderTokenPoolsForRound(addr, round)
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
		return nil, err
	}

	dStatus, err := c.DelegatorStatus(addr)
	if err != nil {
		return nil, err
	}

	status, err := lpTypes.ParseDelegatorStatus(dStatus)
	if err != nil {
		return nil, err
	}

	pendingStake, err := c.PendingStake(addr)
	if err != nil {
		return nil, err
	}

	pendingFees, err := c.PendingFees(addr)
	if err != nil {
		return nil, err
	}

	return &lpTypes.Delegator{
		Address:                        addr,
		BondedAmount:                   dInfo.BondedAmount,
		Fees:                           dInfo.Fees,
		DelegateAddress:                dInfo.DelegateAddress,
		DelegatedAmount:                dInfo.DelegatedAmount,
		StartRound:                     dInfo.StartRound,
		WithdrawRound:                  dInfo.WithdrawRound,
		LastClaimTokenPoolsSharesRound: dInfo.LastClaimTokenPoolsSharesRound,
		PendingStake:                   pendingStake,
		PendingFees:                    pendingFees,
		Status:                         status,
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
		ClaimId:              claimID,
		SegmentRange:         cInfo.SegmentRange,
		ClaimRoot:            cInfo.ClaimRoot,
		ClaimBlock:           cInfo.ClaimBlock,
		EndVerificationBlock: cInfo.EndVerificationBlock,
		EndSlashingBlock:     cInfo.EndSlashingBlock,
		Status:               status,
	}, nil
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

func (c *client) IsAssignedTranscoder(jobID, maxPricePerSegment *big.Int) (bool, error) {
	jInfo, err := c.JobsManagerSession.GetJob(jobID)
	if err != nil {
		return false, err
	}

	currentRound, err := c.CurrentRound()
	if err != nil {
		return false, err
	}

	t, err := c.BondingManagerSession.ElectActiveTranscoder(maxPricePerSegment, jInfo.CreationBlock, currentRound)
	if err != nil {
		return false, err
	}

	return c.Account().Address == t, nil
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
