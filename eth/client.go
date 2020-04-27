/*
Package eth client is the go client for the Livepeer Ethereum smart contract.  Contracts here are generated.
*/
package eth

//go:generate abigen --abi protocol/abi/Controller.abi --pkg contracts --type Controller --out contracts/controller.go
//go:generate abigen --abi protocol/abi/LivepeerToken.abi --pkg contracts --type LivepeerToken --out contracts/livepeerToken.go
//go:generate abigen --abi protocol/abi/ServiceRegistry.abi --pkg contracts --type ServiceRegistry --out contracts/serviceRegistry.go
//go:generate abigen --abi protocol/abi/BondingManager.abi --pkg contracts --type BondingManager --out contracts/bondingManager.go
//go:generate abigen --abi protocol/abi/TicketBroker.abi --pkg contracts --type TicketBroker --out contracts/ticketBroker.go
//go:generate abigen --abi protocol/abi/RoundsManager.abi --pkg contracts --type RoundsManager --out contracts/roundsManager.go
//go:generate abigen --abi protocol/abi/Minter.abi --pkg contracts --type Minter --out contracts/minter.go
//go:generate abigen --abi protocol/abi/LivepeerTokenFaucet.abi --pkg contracts --type LivepeerTokenFaucet --out contracts/livepeerTokenFaucet.go
//go:generate abigen --abi protocol/abi/Poll.abi --pkg contracts --type Poll --out contracts/poll.go
import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

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
	"github.com/livepeer/go-livepeer/pm"
)

var (
	ErrReplacingMinedTx   = fmt.Errorf("trying to replace already mined tx")
	ErrCurrentRoundLocked = fmt.Errorf("current round locked")
	ErrMissingBackend     = fmt.Errorf("missing Ethereum client backend")
)

type LivepeerEthClient interface {
	Setup(password string, gasLimit uint64, gasPrice *big.Int) error
	Account() accounts.Account
	Backend() (Backend, error)

	// Rounds
	InitializeRound() (*types.Transaction, error)
	CurrentRound() (*big.Int, error)
	LastInitializedRound() (*big.Int, error)
	BlockHashForRound(round *big.Int) ([32]byte, error)
	CurrentRoundInitialized() (bool, error)
	CurrentRoundLocked() (bool, error)
	CurrentRoundStartBlock() (*big.Int, error)

	// Token
	Transfer(toAddr ethcommon.Address, amount *big.Int) (*types.Transaction, error)
	Request() (*types.Transaction, error)
	NextValidRequest(addr ethcommon.Address) (*big.Int, error)
	BalanceOf(ethcommon.Address) (*big.Int, error)
	TotalSupply() (*big.Int, error)

	// Service Registry
	SetServiceURI(serviceURI string) (*types.Transaction, error)
	GetServiceURI(addr ethcommon.Address) (string, error)

	// Staking
	Transcoder(blockRewardCut, feeShare *big.Int) (*types.Transaction, error)
	Reward() (*types.Transaction, error)
	Bond(amount *big.Int, toAddr ethcommon.Address) (*types.Transaction, error)
	Rebond(unbondingLockID *big.Int) (*types.Transaction, error)
	RebondFromUnbonded(toAddr ethcommon.Address, unbondingLockID *big.Int) (*types.Transaction, error)
	Unbond(amount *big.Int) (*types.Transaction, error)
	WithdrawStake(unbondingLockID *big.Int) (*types.Transaction, error)
	WithdrawFees() (*types.Transaction, error)
	ClaimEarnings(endRound *big.Int) error
	GetTranscoder(addr ethcommon.Address) (*lpTypes.Transcoder, error)
	GetDelegator(addr ethcommon.Address) (*lpTypes.Delegator, error)
	GetDelegatorUnbondingLock(addr ethcommon.Address, unbondingLockId *big.Int) (*lpTypes.UnbondingLock, error)
	GetTranscoderEarningsPoolForRound(addr ethcommon.Address, round *big.Int) (*lpTypes.TokenPools, error)
	TranscoderPool() ([]*lpTypes.Transcoder, error)
	IsActiveTranscoder() (bool, error)
	GetTotalBonded() (*big.Int, error)
	GetTranscoderPoolSize() (*big.Int, error)

	// TicketBroker
	FundDepositAndReserve(depositAmount, penaltyEscrowAmount *big.Int) (*types.Transaction, error)
	FundDeposit(amount *big.Int) (*types.Transaction, error)
	FundReserve(amount *big.Int) (*types.Transaction, error)
	Unlock() (*types.Transaction, error)
	CancelUnlock() (*types.Transaction, error)
	Withdraw() (*types.Transaction, error)
	RedeemWinningTicket(ticket *pm.Ticket, sig []byte, recipientRand *big.Int) (*types.Transaction, error)
	IsUsedTicket(ticket *pm.Ticket) (bool, error)
	GetSenderInfo(addr ethcommon.Address) (*pm.SenderInfo, error)
	UnlockPeriod() (*big.Int, error)
	ClaimedReserve(reserveHolder ethcommon.Address, claimant ethcommon.Address) (*big.Int, error)

	// Parameters
	GetTranscoderPoolMaxSize() (*big.Int, error)
	RoundLength() (*big.Int, error)
	RoundLockAmount() (*big.Int, error)
	UnbondingPeriod() (uint64, error)
	Inflation() (*big.Int, error)
	InflationChange() (*big.Int, error)
	TargetBondingRate() (*big.Int, error)
	Paused() (bool, error)

	// Governance
	Vote(ethcommon.Address, *big.Int) (*types.Transaction, error)

	// Helpers
	ContractAddresses() map[string]ethcommon.Address
	CheckTx(*types.Transaction) error
	ReplaceTransaction(*types.Transaction, string, *big.Int) (*types.Transaction, error)
	Sign([]byte) ([]byte, error)
	GetGasInfo() (uint64, *big.Int)
	SetGasInfo(uint64, *big.Int) error
}

type client struct {
	accountManager AccountManager
	backend        Backend

	controllerAddr      ethcommon.Address
	tokenAddr           ethcommon.Address
	serviceRegistryAddr ethcommon.Address
	bondingManagerAddr  ethcommon.Address
	ticketBrokerAddr    ethcommon.Address
	roundsManagerAddr   ethcommon.Address
	minterAddr          ethcommon.Address
	verifierAddr        ethcommon.Address
	faucetAddr          ethcommon.Address

	// Embedded contract sessions
	*contracts.ControllerSession
	*contracts.LivepeerTokenSession
	*contracts.ServiceRegistrySession
	*contracts.BondingManagerSession
	*contracts.TicketBrokerSession
	*contracts.RoundsManagerSession
	*contracts.MinterSession
	*contracts.LivepeerTokenFaucetSession

	gasLimit uint64
	gasPrice *big.Int

	txTimeout time.Duration
}

func NewClient(accountAddr ethcommon.Address, keystoreDir string, eth *ethclient.Client, controllerAddr ethcommon.Address, txTimeout time.Duration) (LivepeerEthClient, error) {
	chainID, err := eth.ChainID(context.Background())
	if err != nil {
		return nil, err
	}

	signer := types.NewEIP155Signer(chainID)

	backend, err := NewBackend(eth, signer)
	if err != nil {
		return nil, err
	}

	am, err := NewAccountManager(accountAddr, keystoreDir, signer)
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

	brokerAddr, err := c.GetContract(crypto.Keccak256Hash([]byte("TicketBroker")))
	if err != nil {
		glog.Errorf("Error getting TicketBroker address: %v", err)
		return err
	}

	c.ticketBrokerAddr = brokerAddr

	broker, err := contracts.NewTicketBroker(brokerAddr, c.backend)
	if err != nil {
		glog.Errorf("Error creating TicketBroker binding: %v", err)
		return err
	}

	c.TicketBrokerSession = &contracts.TicketBrokerSession{
		Contract:     broker,
		TransactOpts: *opts,
	}

	glog.V(common.SHORT).Infof("TicketBroker: %v", c.ticketBrokerAddr.Hex())

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
	return c.accountManager.Account()
}

func (c *client) Backend() (Backend, error) {
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

func (c *client) Transcoder(blockRewardCut, feeShare *big.Int) (*types.Transaction, error) {
	locked, err := c.CurrentRoundLocked()
	if err != nil {
		return nil, err
	}

	if locked {
		return nil, ErrCurrentRoundLocked
	} else {
		return c.BondingManagerSession.Transcoder(blockRewardCut, feeShare)
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

func (c *client) Rebond(unbondingLockID *big.Int) (*types.Transaction, error) {
	currentRound, err := c.CurrentRound()
	if err != nil {
		return nil, err
	}

	if err := c.autoClaimEarnings(currentRound, false); err != nil {
		return nil, err
	}

	return c.BondingManagerSession.Rebond(unbondingLockID)
}

func (c *client) RebondFromUnbonded(toAddr ethcommon.Address, unbondingLockID *big.Int) (*types.Transaction, error) {
	currentRound, err := c.CurrentRound()
	if err != nil {
		return nil, err
	}

	if err := c.autoClaimEarnings(currentRound, false); err != nil {
		return nil, err
	}

	return c.BondingManagerSession.RebondFromUnbonded(toAddr, unbondingLockID)
}

func (c *client) Unbond(amount *big.Int) (*types.Transaction, error) {
	currentRound, err := c.CurrentRound()
	if err != nil {
		return nil, err
	}

	if err := c.autoClaimEarnings(currentRound, false); err != nil {
		return nil, err
	}

	return c.BondingManagerSession.Unbond(amount)
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

func (c *client) IsActiveTranscoder() (bool, error) {
	return c.BondingManagerSession.IsActiveTranscoder(c.Account().Address)
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

	active, err := c.BondingManagerSession.IsActiveTranscoder(addr)
	if err != nil {
		return nil, err
	}

	serviceURI, err := c.GetServiceURI(addr)
	if err != nil {
		return nil, err
	}

	return &lpTypes.Transcoder{
		Address:           addr,
		ServiceURI:        serviceURI,
		LastRewardRound:   tInfo.LastRewardRound,
		RewardCut:         tInfo.RewardCut,
		FeeShare:          tInfo.FeeShare,
		DelegatedStake:    delegatedStake,
		ActivationRound:   tInfo.ActivationRound,
		DeactivationRound: tInfo.DeactivationRound,
		Active:            active,
		Status:            status,
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
		Address:             addr,
		BondedAmount:        dInfo.BondedAmount,
		Fees:                dInfo.Fees,
		DelegateAddress:     dInfo.DelegateAddress,
		DelegatedAmount:     dInfo.DelegatedAmount,
		StartRound:          dInfo.StartRound,
		LastClaimRound:      dInfo.LastClaimRound,
		NextUnbondingLockId: dInfo.NextUnbondingLockId,
		PendingStake:        pendingStake,
		PendingFees:         pendingFees,
		Status:              status,
	}, nil
}

func (c *client) GetDelegatorUnbondingLock(addr ethcommon.Address, unbondingLockId *big.Int) (*lpTypes.UnbondingLock, error) {
	lock, err := c.BondingManagerSession.GetDelegatorUnbondingLock(addr, unbondingLockId)
	if err != nil {
		return nil, err
	}

	return &lpTypes.UnbondingLock{
		ID:               unbondingLockId,
		DelegatorAddress: addr,
		Amount:           lock.Amount,
		WithdrawRound:    lock.WithdrawRound,
	}, nil
}

func (c *client) Paused() (bool, error) {
	return c.ControllerSession.Paused()
}

func (c *client) TranscoderPool() ([]*lpTypes.Transcoder, error) {
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

func (c *client) Vote(pollAddr ethcommon.Address, choiceID *big.Int) (*types.Transaction, error) {
	poll, err := contracts.NewPoll(pollAddr, c.backend)
	if err != nil {
		return nil, err
	}

	gl, gp := c.GetGasInfo()
	opts, err := c.accountManager.CreateTransactOpts(gl, gp)
	if err != nil {
		return nil, err
	}

	return poll.Vote(opts, choiceID)
}

// Helpers

func (c *client) ContractAddresses() map[string]ethcommon.Address {
	addrMap := make(map[string]ethcommon.Address)
	addrMap["Controller"] = c.controllerAddr
	addrMap["LivepeerToken"] = c.tokenAddr
	addrMap["LivepeerTokenFaucet"] = c.faucetAddr
	addrMap["TicketBroker"] = c.ticketBrokerAddr
	addrMap["RoundsManager"] = c.roundsManagerAddr
	addrMap["BondingManager"] = c.bondingManagerAddr
	addrMap["Minter"] = c.minterAddr

	return addrMap
}

func (c *client) CheckTx(tx *types.Transaction) error {
	ctx, cancel := context.WithTimeout(context.Background(), c.txTimeout)
	defer cancel()

	receipt, err := bind.WaitMined(ctx, c.backend, tx)
	if err != nil {
		return err
	}

	if receipt.Status == uint64(0) {
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

	newSignedTx, err := c.accountManager.SignTx(newRawTx)
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
