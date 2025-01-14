/*
Package eth client is the go client for the Livepeer Ethereum smart contract.  Contracts here are generated.
*/
package eth

//go:generate abigen --abi protocol/abi/Controller.json --pkg contracts --type Controller --out contracts/controller.go
//go:generate abigen --abi protocol/abi/LivepeerToken.json --pkg contracts --type LivepeerToken --out contracts/livepeerToken.go
//go:generate abigen --abi protocol/abi/ServiceRegistry.json --pkg contracts --type ServiceRegistry --out contracts/serviceRegistry.go
//go:generate abigen --abi protocol/abi/BondingManager.json --pkg contracts --type BondingManager --out contracts/bondingManager.go
//go:generate abigen --abi protocol/abi/TicketBroker.json --pkg contracts --type TicketBroker --out contracts/ticketBroker.go
//go:generate abigen --abi protocol/abi/RoundsManager.json --pkg contracts --type RoundsManager --out contracts/roundsManager.go
//go:generate abigen --abi protocol/abi/Minter.json --pkg contracts --type Minter --out contracts/minter.go
//go:generate abigen --abi protocol/abi/LivepeerTokenFaucet.json --pkg contracts --type LivepeerTokenFaucet --out contracts/livepeerTokenFaucet.go
//go:generate abigen --abi protocol/abi/Poll.json --pkg contracts --type Poll --out contracts/poll.go
import (
	"context"
	"fmt"
	"math/big"
	"sort"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/signer/core/apitypes"
	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/eth/contracts"
	lpTypes "github.com/livepeer/go-livepeer/eth/types"
	"github.com/livepeer/go-livepeer/pm"
	"github.com/pkg/errors"
)

var (
	ErrReplacingMinedTx   = fmt.Errorf("trying to replace already mined tx")
	ErrCurrentRoundLocked = fmt.Errorf("current round locked")
	ErrMissingBackend     = fmt.Errorf("missing Ethereum client backend")
	ethRpcTimeout         = 2 * time.Minute
)

type LivepeerEthClient interface {
	Account() accounts.Account
	Backend() Backend

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
	WithdrawFees(addr ethcommon.Address, amount *big.Int) (*types.Transaction, error)
	// for L1 contracts backwards-compatibility
	L1WithdrawFees() (*types.Transaction, error)
	ClaimEarnings(endRound *big.Int) (*types.Transaction, error)
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
	GetGlobalTotalSupply() (*big.Int, error)
	Paused() (bool, error)

	// Governance
	Vote(ethcommon.Address, *big.Int) (*types.Transaction, error)

	// Helpers
	ContractAddresses() map[string]ethcommon.Address
	CheckTx(*types.Transaction) error
	Sign([]byte) ([]byte, error)
	SignTypedData(apitypes.TypedData) ([]byte, error)
	SetGasInfo(uint64) error
	SetMaxGasPrice(*big.Int) error
}

type client struct {
	accountManager AccountManager
	backend        Backend
	tm             *TransactionManager
	transOpts      bind.TransactOpts
	transOptsMu    sync.RWMutex

	controllerAddr      ethcommon.Address
	tokenAddr           ethcommon.Address
	serviceRegistryAddr ethcommon.Address
	bondingManagerAddr  ethcommon.Address
	ticketBrokerAddr    ethcommon.Address
	roundsManagerAddr   ethcommon.Address
	minterAddr          ethcommon.Address
	verifierAddr        ethcommon.Address
	faucetAddr          ethcommon.Address

	// Contracts
	controller          *contracts.Controller
	livepeerToken       *contracts.LivepeerToken
	serviceRegistry     *contracts.ServiceRegistry
	bondingManager      *contracts.BondingManager
	ticketBroker        *contracts.TicketBroker
	roundsManager       *contracts.RoundsManager
	minter              *contracts.Minter
	livepeerTokenFaucet *contracts.LivepeerTokenFaucet

	// for L1 contracts backwards-compatibility
	l1BondingManager *contracts.L1BondingManager

	gasLimit uint64
	gasPrice *big.Int

	checkTxTimeout time.Duration
}

type LivepeerEthClientConfig struct {
	AccountManager     AccountManager
	GasPriceMonitor    *GasPriceMonitor
	EthClient          *ethclient.Client
	TransactionManager *TransactionManager
	Signer             types.Signer
	ControllerAddr     ethcommon.Address
	CheckTxTimeout     time.Duration

	// For the time-being Livepeer AI Subnet uses its own ServiceRegistry, so we define it here
	ServiceRegistryAddr ethcommon.Address
}

func NewClient(cfg LivepeerEthClientConfig) (LivepeerEthClient, error) {

	backend := NewBackend(cfg.EthClient, cfg.Signer, cfg.GasPriceMonitor, cfg.TransactionManager)

	return &client{
		accountManager:      cfg.AccountManager,
		backend:             backend,
		tm:                  cfg.TransactionManager,
		controllerAddr:      cfg.ControllerAddr,
		checkTxTimeout:      cfg.CheckTxTimeout,
		serviceRegistryAddr: cfg.ServiceRegistryAddr,
	}, nil
}

func (c *client) setContracts(opts *bind.TransactOpts) error {
	c.setTransactOpts(*opts)

	controller, err := contracts.NewController(c.controllerAddr, c.backend)
	if err != nil {
		glog.Errorf("Error creating Controller binding: %v", err)
		return err
	}
	c.controller = controller

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
	c.livepeerToken = token

	glog.V(common.SHORT).Infof("LivepeerToken: %v", c.tokenAddr.Hex())

	if c.serviceRegistryAddr == (ethcommon.Address{}) {
		c.serviceRegistryAddr, err = c.GetContract(crypto.Keccak256Hash([]byte("ServiceRegistry")))
		if err != nil {
			glog.Errorf("Error getting ServiceRegistry address: %v", err)
			return err
		}
	}

	serviceRegistry, err := contracts.NewServiceRegistry(c.serviceRegistryAddr, c.backend)
	if err != nil {
		glog.Errorf("Error creating ServiceRegistry binding: %v", err)
		return err
	}
	c.serviceRegistry = serviceRegistry

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
	c.bondingManager = bondingManager

	// for L1 contracts backwards-compatibility
	l1BondingManager, err := contracts.NewL1BondingManager(bondingManagerAddr, c.backend)
	if err != nil {
		glog.Errorf("Error creating L1BondingManager binding: %v", err)
		return err
	}
	c.l1BondingManager = l1BondingManager

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
	c.ticketBroker = broker

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
	c.roundsManager = roundsManager

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
	c.minter = minter

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
	c.livepeerTokenFaucet = faucet

	glog.V(common.SHORT).Infof("LivepeerTokenFaucet: %v", c.faucetAddr.Hex())

	return nil
}

func (c *client) SetGasInfo(gasLimit uint64) error {
	opts, err := c.accountManager.CreateTransactOpts(gasLimit)
	if err != nil {
		return err
	}

	if err := c.setContracts(opts); err != nil {
		return err
	} else {
		c.gasLimit = gasLimit
		return nil
	}
}

func (c *client) SetMaxGasPrice(maxGasPrice *big.Int) error {
	head, err := c.backend.HeaderByNumber(context.Background(), nil)
	if err != nil {
		return err
	}

	c.transOptsMu.Lock()
	if head.BaseFee == nil {
		// legacy tx, not London ready
		c.transOpts.GasPrice = maxGasPrice
	} else {
		// dynamic tx
		c.transOpts.GasFeeCap = maxGasPrice
	}
	c.transOptsMu.Unlock()

	return nil
}

func (c *client) setTransactOpts(opts bind.TransactOpts) {
	c.transOptsMu.Lock()
	c.transOpts = opts
	c.transOptsMu.Unlock()
}

func (c *client) transactOpts() *bind.TransactOpts {
	c.transOptsMu.RLock()
	opts := c.transOpts
	c.transOptsMu.RUnlock()

	opts.Context = newEthRpcContext()

	// If GasFeeCap is nil then one of the following will be true:
	// - A dynamic tx will be created by BoundContract and GasFeeCap will automatically be set.
	// - A legacy tx will be created by BoundContract in which case the GasFeeCap field is not used.
	if opts.GasFeeCap == nil {
		return &opts
	}

	// If GasFeeCap is non-nil ensure that we adjust it to be min(gasPriceEstimate, current GasFeeCap).
	gasPriceEstimate := opts.GasFeeCap
	head, err := c.backend.HeaderByNumber(context.Background(), nil)
	if err != nil {
		glog.Errorf("failed to calculate gas price estimate - defaulting to using GasFeeCap = maxGasPrice")
	} else {
		gasPriceEstimate = new(big.Int).Mul(head.BaseFee, big.NewInt(2))
	}
	// Setting GasFeeCap > gasPriceEstimate is detrimental to the user because the user account will need at least gas * GasFeeCap in their balance
	// to pay for the tx when they only need gas * gasPriceEstimate. GasFeeCap is initially going to be set to the maxGasPrice specified by the user which
	// they expect to be the maximum gas price they will ever pay. So, a user might set maxGasPrice to 100 gwei as a safety precaution even if the average gas price
	// is 0.1 gwei. In this case, the gasPriceEstimate would be 0.1 * 2 = 0.2 gwei (multiplied by 2 to add a buffer). So, the user account would need 500x more funds
	// in its balance if its GasFeeCap is set to the maxGasPrice.
	if opts.GasFeeCap.Cmp(gasPriceEstimate) > 0 {
		opts.GasFeeCap = gasPriceEstimate
	}

	return &opts
}

func (c *client) callOpts() *bind.CallOpts {
	return &bind.CallOpts{
		Context: newEthRpcContext(),
	}
}

func newEthRpcContext() context.Context {
	ctx, _ := context.WithTimeout(context.Background(), ethRpcTimeout)
	return ctx
}

func (c *client) Account() accounts.Account {
	return c.accountManager.Account()
}

func (c *client) Backend() Backend {
	return c.backend
}

// Controller
func (c *client) GetContract(hash ethcommon.Hash) (ethcommon.Address, error) {
	return c.controller.GetContract(c.callOpts(), hash)
}

func (c *client) Paused() (bool, error) {
	return c.controller.Paused(c.callOpts())
}

// Rounds
func (c *client) InitializeRound() (*types.Transaction, error) {
	i, err := c.roundsManager.CurrentRoundInitialized(c.callOpts())
	if err != nil {
		return nil, err
	}
	if i {
		glog.V(common.SHORT).Infof("Round already initialized")
		return nil, errors.New("ErrRoundInitialized")
	} else {
		return c.roundsManager.InitializeRound(c.transactOpts())
	}
}

func (c *client) CurrentRound() (*big.Int, error) {
	return c.roundsManager.CurrentRound(c.callOpts())
}

func (c *client) CurrentRoundLocked() (bool, error) {
	return c.roundsManager.CurrentRoundLocked(c.callOpts())
}

func (c *client) LastInitializedRound() (*big.Int, error) {
	return c.roundsManager.LastInitializedRound(c.callOpts())
}

func (c *client) BlockHashForRound(round *big.Int) ([32]byte, error) {
	return c.roundsManager.BlockHashForRound(c.callOpts(), round)
}

func (c *client) CurrentRoundInitialized() (bool, error) {
	return c.roundsManager.CurrentRoundInitialized(c.callOpts())
}

func (c *client) CurrentRoundStartBlock() (*big.Int, error) {
	return c.roundsManager.CurrentRoundStartBlock(c.callOpts())
}

func (c *client) RoundLength() (*big.Int, error) {
	return c.roundsManager.RoundLength(c.callOpts())
}

func (c *client) RoundLockAmount() (*big.Int, error) {
	return c.roundsManager.RoundLockAmount(c.callOpts())
}

// Minter
func (c *client) Inflation() (*big.Int, error) {
	return c.minter.Inflation(c.callOpts())
}

func (c *client) InflationChange() (*big.Int, error) {
	return c.minter.InflationChange(c.callOpts())
}

func (c *client) TargetBondingRate() (*big.Int, error) {
	return c.minter.TargetBondingRate(c.callOpts())
}

func (c *client) GetGlobalTotalSupply() (*big.Int, error) {
	return c.minter.GetGlobalTotalSupply(c.callOpts())
}

func (c *client) CurrentMintableTokens() (*big.Int, error) {
	return c.minter.CurrentMintableTokens(c.callOpts())
}

// Token
func (c *client) Transfer(toAddr ethcommon.Address, amount *big.Int) (*types.Transaction, error) {
	return c.livepeerToken.Transfer(c.transactOpts(), toAddr, amount)
}

func (c *client) Allowance(owner ethcommon.Address, spender ethcommon.Address) (*big.Int, error) {
	return c.livepeerToken.Allowance(c.callOpts(), owner, spender)
}

func (c *client) Request() (*types.Transaction, error) {
	return c.livepeerTokenFaucet.Request(c.transactOpts())
}

func (c *client) BalanceOf(address ethcommon.Address) (*big.Int, error) {
	return c.livepeerToken.BalanceOf(c.callOpts(), address)
}

func (c *client) TotalSupply() (*big.Int, error) {
	return c.livepeerToken.TotalSupply(c.callOpts())
}

func (c *client) NextValidRequest(addr ethcommon.Address) (*big.Int, error) {
	return c.livepeerTokenFaucet.NextValidRequest(c.callOpts(), addr)
}

// Service Registry
func (c *client) SetServiceURI(serviceURI string) (*types.Transaction, error) {
	return c.serviceRegistry.SetServiceURI(c.transactOpts(), serviceURI)
}

func (c *client) GetServiceURI(addr ethcommon.Address) (string, error) {
	return c.serviceRegistry.GetServiceURI(c.callOpts(), addr)
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
		return c.bondingManager.Transcoder(c.transactOpts(), blockRewardCut, feeShare)
	}
}

func (c *client) Bond(amount *big.Int, to ethcommon.Address) (*types.Transaction, error) {
	sender := c.Account().Address
	allowance, err := c.Allowance(sender, c.bondingManagerAddr)
	if err != nil {
		return nil, err
	}

	// If existing allowance set by account for BondingManager is
	// less than the bond amount, approve the necessary amount
	if allowance.Cmp(amount) == -1 {
		tx, err := c.livepeerToken.Approve(c.transactOpts(), c.bondingManagerAddr, amount)
		if err != nil {
			return nil, err
		}

		err = c.CheckTx(tx)
		if err != nil {
			return nil, err
		}
	}

	// Get transcoder pool
	transcoders, err := c.TranscoderPool()
	if err != nil {
		return nil, errors.Wrapf(err, "unable to get transcoder pool")
	}

	// Get max pool size
	maxSize, err := c.GetTranscoderPoolMaxSize()
	if err != nil {
		return nil, errors.Wrapf(err, "unable to get transcoder pool max size")
	}

	// Get delegator
	delegator, err := c.GetDelegator(sender)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to get delegator")
	}

	isFull := int64(len(transcoders)) == maxSize.Int64()

	// Switching delegate's calculate old delegate positions
	var oldHints lpTypes.TranscoderPoolHints
	if delegator.DelegateAddress != to && delegator.DelegateAddress != (ethcommon.Address{}) {
		currentRound, err := c.CurrentRound()
		if err != nil {
			return nil, err
		}

		delegatorTotalStake, err := c.PendingStake(sender, currentRound)
		if err != nil {
			return nil, err
		}

		// If the caller is switching delegate's with additional stake the new amount becomes
		// the delegator's current pending stake plus the amount
		amount = new(big.Int).Add(delegatorTotalStake, amount)
		// Get total bonded
		totalBonded, err := c.TranscoderTotalStake(delegator.DelegateAddress)
		if err != nil {
			return nil, err
		}

		// Only substract the delegator's pending stake from the old delegate since 'amount' is newly added stake
		oldHints = simulateTranscoderPoolUpdate(delegator.DelegateAddress, new(big.Int).Sub(totalBonded, delegatorTotalStake), transcoders, isFull)
	}

	// Get total bonded
	totalBonded, err := c.TranscoderTotalStake(to)
	if err != nil {
		return nil, err
	}
	newStake := totalBonded.Add(totalBonded, amount)

	newHints := simulateTranscoderPoolUpdate(to, newStake, transcoders, isFull)

	return c.bondingManager.BondWithHint(
		c.transactOpts(),
		amount,
		to,
		oldHints.PosPrev,
		oldHints.PosNext,
		newHints.PosPrev,
		newHints.PosNext,
	)
}

func (c *client) Unbond(amount *big.Int) (*types.Transaction, error) {
	sender := c.Account().Address

	// Get delegator
	delegator, err := c.GetDelegator(sender)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to get delegator")
	}

	// Get transcoder pool
	transcoders, err := c.TranscoderPool()
	if err != nil {
		return nil, errors.Wrapf(err, "unable to get transcoder pool")
	}

	// Get max pool size
	maxSize, err := c.GetTranscoderPoolMaxSize()
	if err != nil {
		return nil, errors.Wrapf(err, "unable to get transcoder pool max size")
	}

	// Get total bonded
	totalBonded, err := c.TranscoderTotalStake(delegator.DelegateAddress)
	if err != nil {
		return nil, err
	}

	newStake := totalBonded.Sub(totalBonded, amount)

	isFull := int64(len(transcoders)) == maxSize.Int64()

	hints := simulateTranscoderPoolUpdate(delegator.DelegateAddress, newStake, transcoders, isFull)

	return c.bondingManager.UnbondWithHint(c.transactOpts(), amount, hints.PosPrev, hints.PosNext)
}

func (c *client) RebondFromUnbonded(to ethcommon.Address, unbondingLockID *big.Int) (*types.Transaction, error) {
	sender := c.Account().Address

	// Get transcoder pool
	transcoders, err := c.TranscoderPool()
	if err != nil {
		return nil, errors.Wrapf(err, "unable to get transcoder pool")
	}

	// Get max pool size
	maxSize, err := c.GetTranscoderPoolMaxSize()
	if err != nil {
		return nil, errors.Wrapf(err, "unable to get transcoder pool max size")
	}

	totalBonded, err := c.TranscoderTotalStake(to)
	if err != nil {
		return nil, err
	}

	lock, err := c.GetDelegatorUnbondingLock(sender, unbondingLockID)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to get unbonding lock")
	}

	isFull := int64(len(transcoders)) == maxSize.Int64()

	newStake := totalBonded.Add(totalBonded, lock.Amount)

	hints := simulateTranscoderPoolUpdate(to, newStake, transcoders, isFull)

	return c.bondingManager.RebondFromUnbondedWithHint(c.transactOpts(), to, unbondingLockID, hints.PosPrev, hints.PosNext)
}

func (c *client) Rebond(unbondingLockID *big.Int) (*types.Transaction, error) {
	sender := c.Account().Address

	// Get delegator
	delegator, err := c.GetDelegator(sender)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to get delegator")
	}

	// Get transcoder pool
	transcoders, err := c.TranscoderPool()
	if err != nil {
		return nil, errors.Wrapf(err, "unable to get transcoder pool")
	}

	// Get max pool size
	maxSize, err := c.GetTranscoderPoolMaxSize()
	if err != nil {
		return nil, errors.Wrapf(err, "unable to get transcoder pool max size")
	}

	lock, err := c.GetDelegatorUnbondingLock(sender, unbondingLockID)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to get unbonding lock")
	}

	transcoderStake, err := c.TranscoderTotalStake(delegator.DelegateAddress)
	if err != nil {
		return nil, err
	}

	isFull := int64(len(transcoders)) == maxSize.Int64()

	newStake := transcoderStake.Add(transcoderStake, lock.Amount)

	hints := simulateTranscoderPoolUpdate(delegator.DelegateAddress, newStake, transcoders, isFull)

	return c.bondingManager.RebondWithHint(c.transactOpts(), unbondingLockID, hints.PosPrev, hints.PosNext)
}

func (c *client) WithdrawStake(unbondingLockID *big.Int) (*types.Transaction, error) {
	return c.bondingManager.WithdrawStake(c.transactOpts(), unbondingLockID)
}

func (c *client) L1WithdrawFees() (*types.Transaction, error) {
	return c.l1BondingManager.WithdrawFees(c.transactOpts())
}

func (c *client) ClaimEarnings(endRound *big.Int) (*types.Transaction, error) {
	return c.bondingManager.ClaimEarnings(c.transactOpts(), endRound)
}

func (c *client) GetTranscoderPoolMaxSize() (*big.Int, error) {
	return c.bondingManager.GetTranscoderPoolMaxSize(c.callOpts())
}

func (c *client) TranscoderTotalStake(to ethcommon.Address) (*big.Int, error) {
	return c.bondingManager.TranscoderTotalStake(c.callOpts(), to)
}

func (c *client) GetTotalBonded() (*big.Int, error) {
	return c.bondingManager.GetTotalBonded(c.callOpts())
}

func (c *client) PendingStake(delegator ethcommon.Address, endRound *big.Int) (*big.Int, error) {
	return c.bondingManager.PendingStake(c.callOpts(), delegator, endRound)
}

func (c *client) TranscoderStatus(transcoder ethcommon.Address) (uint8, error) {
	return c.bondingManager.TranscoderStatus(c.callOpts(), transcoder)
}

func (c *client) DelegatorStatus(delegator ethcommon.Address) (uint8, error) {
	return c.bondingManager.DelegatorStatus(c.callOpts(), delegator)
}

func (c *client) GetFirstTranscoderInPool() (ethcommon.Address, error) {
	return c.bondingManager.GetFirstTranscoderInPool(c.callOpts())
}

func (c *client) PendingFees(delegator ethcommon.Address, endRound *big.Int) (*big.Int, error) {
	return c.bondingManager.PendingFees(c.callOpts(), delegator, endRound)
}

func (c *client) GetNextTranscoderInPool(transcoder ethcommon.Address) (ethcommon.Address, error) {
	return c.bondingManager.GetNextTranscoderInPool(c.callOpts(), transcoder)
}

func (c *client) GetTranscoderPoolSize() (*big.Int, error) {
	return c.bondingManager.GetTranscoderPoolSize(c.callOpts())
}

func (c *client) UnbondingPeriod() (uint64, error) {
	return c.bondingManager.UnbondingPeriod(c.callOpts())
}

func (c *client) IsActiveTranscoder() (bool, error) {
	return c.bondingManager.IsActiveTranscoder(c.callOpts(), c.Account().Address)
}

func (c *client) GetTranscoder(addr ethcommon.Address) (*lpTypes.Transcoder, error) {
	tInfo, err := c.bondingManager.GetTranscoder(c.callOpts(), addr)
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

	active, err := c.bondingManager.IsActiveTranscoder(c.callOpts(), addr)
	if err != nil {
		return nil, err
	}

	serviceURI, err := c.GetServiceURI(addr)
	if err != nil {
		return nil, err
	}

	return &lpTypes.Transcoder{
		Address:                    addr,
		ServiceURI:                 serviceURI,
		LastRewardRound:            tInfo.LastRewardRound,
		RewardCut:                  tInfo.RewardCut,
		FeeShare:                   tInfo.FeeShare,
		DelegatedStake:             delegatedStake,
		ActivationRound:            tInfo.ActivationRound,
		DeactivationRound:          tInfo.DeactivationRound,
		LastActiveStakeUpdateRound: tInfo.LastActiveStakeUpdateRound,
		Active:                     active,
		Status:                     status,
	}, nil
}

func (c *client) GetTranscoderEarningsPoolForRound(addr ethcommon.Address, round *big.Int) (*lpTypes.TokenPools, error) {
	tp, err := c.bondingManager.GetTranscoderEarningsPoolForRound(c.callOpts(), addr, round)
	if err != nil {
		return nil, err
	}

	return &lpTypes.TokenPools{
		TotalStake:             tp.TotalStake,
		TranscoderRewardCut:    tp.TranscoderRewardCut,
		TranscoderFeeShare:     tp.TranscoderFeeShare,
		CumulativeRewardFactor: tp.CumulativeRewardFactor,
		CumulativeFeeFactor:    tp.CumulativeFeeFactor,
	}, nil
}

func (c *client) GetDelegator(addr ethcommon.Address) (*lpTypes.Delegator, error) {
	dInfo, err := c.bondingManager.GetDelegator(c.callOpts(), addr)
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
	lock, err := c.bondingManager.GetDelegatorUnbondingLock(c.callOpts(), addr, unbondingLockId)
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

// TicketBroker
func (c *client) Unlock() (*types.Transaction, error) {
	return c.ticketBroker.Unlock(c.transactOpts())
}

func (c *client) CancelUnlock() (*types.Transaction, error) {
	return c.ticketBroker.CancelUnlock(c.transactOpts())
}

func (c *client) Withdraw() (*types.Transaction, error) {
	return c.ticketBroker.Withdraw(c.transactOpts())
}

func (c *client) UnlockPeriod() (*big.Int, error) {
	return c.ticketBroker.UnlockPeriod(c.callOpts())
}

func (c *client) ClaimedReserve(reserveHolder ethcommon.Address, claimant ethcommon.Address) (*big.Int, error) {
	return c.ticketBroker.ClaimedReserve(c.callOpts(), reserveHolder, claimant)
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

	opts := c.transactOpts()
	return poll.Vote(opts, choiceID)
}

func (c *client) Reward() (*types.Transaction, error) {
	addr := c.accountManager.Account().Address

	tr, err := c.GetTranscoder(addr)
	if err != nil {
		return nil, err
	}

	ep, err := c.GetTranscoderEarningsPoolForRound(addr, tr.LastActiveStakeUpdateRound)
	if err != nil {
		return nil, err
	}
	activeTotalStake := ep.TotalStake

	mintable, err := c.CurrentMintableTokens()
	if err != nil {
		return nil, errors.Wrapf(err, "unable to get current mintable tokens")
	}

	totalBonded, err := c.GetTotalBonded()
	if err != nil {
		return nil, errors.Wrapf(err, "unable to get total bonded")
	}

	if totalBonded.Cmp(big.NewInt(0)) == 0 {
		return nil, errors.New("no rewards to be minted")
	}

	// reward = (current mintable tokens for the round * active transcoder stake) / total active stake
	reward := new(big.Int).Div(new(big.Int).Mul(mintable, activeTotalStake), totalBonded)

	// get the transcoder pool
	transcoders, err := c.TranscoderPool()
	if err != nil {
		return nil, errors.Wrapf(err, "unable to get transcoder pool")
	}

	// get max pool size
	maxSize, err := c.GetTranscoderPoolMaxSize()
	if err != nil {
		return nil, errors.Wrapf(err, "unable to get transcoder pool max size")
	}

	hints := simulateTranscoderPoolUpdate(addr, reward.Add(reward, tr.DelegatedStake), transcoders, len(transcoders) == int(maxSize.Int64()))

	return c.bondingManager.RewardWithHint(c.transactOpts(), hints.PosPrev, hints.PosNext)
}

func (c *client) WithdrawFees(addr ethcommon.Address, amount *big.Int) (*types.Transaction, error) {
	return c.bondingManager.WithdrawFees(c.transactOpts(), addr, amount)
}

// Helpers

// simulateTranscoderPoolUpdate simulates an update to the transcoder pool and returns the positional hints for a transcoder accordingly.
// if the transcoder will not be in the updated set no hints will be returned
func simulateTranscoderPoolUpdate(del ethcommon.Address, newStake *big.Int, transcoders []*lpTypes.Transcoder, isFull bool) lpTypes.TranscoderPoolHints {
	for i, t := range transcoders {
		if t.Address == del {
			// I don't think an out-of-bounds panic is an issue here when i == len(transcoders) - 1
			// because transcoders[len(transcoders):] is valid
			transcoders = append(transcoders[:i], transcoders[i+1:]...)
			break
		}
	}

	// insert 'del' into the pool
	transcoders = append(transcoders, &lpTypes.Transcoder{
		Address:        del,
		DelegatedStake: newStake,
	})

	// re-sort the list
	sort.SliceStable(transcoders, func(i, j int) bool {
		return transcoders[i].DelegatedStake.Cmp(transcoders[j].DelegatedStake) > 0
	})

	// if the list was full evict the last transcoder
	if isFull {
		transcoders = transcoders[:len(transcoders)-1]
	}

	return findTranscoderHints(del, transcoders)
}

func findTranscoderHints(del ethcommon.Address, transcoders []*lpTypes.Transcoder) lpTypes.TranscoderPoolHints {
	hints := lpTypes.TranscoderPoolHints{}

	// do a linear search to get the previous and next transcoder relative to 'del'
	for i, t := range transcoders {
		if t.Address == del && len(transcoders) > 1 {
			if i == 0 {
				// 'del' is head
				hints.PosNext = transcoders[i+1].Address
			} else if i == len(transcoders)-1 {
				// 'del' is tail
				hints.PosPrev = transcoders[i-1].Address
			} else {
				hints.PosNext = transcoders[i+1].Address
				hints.PosPrev = transcoders[i-1].Address
			}
		}
	}

	return hints
}

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
	receipts := make(chan *transactionReceipt, 10)
	txSub := c.tm.Subscribe(receipts)
	defer txSub.Unsubscribe()

	timer := time.NewTimer(c.checkTxTimeout)
	defer timer.Stop()

	for {
		select {
		case <-timer.C:
			return fmt.Errorf("timed out waiting for transaction receipt txHash=%v", tx.Hash().Hex())
		case err := <-txSub.Err():
			return err
		case receipt := <-receipts:
			if tx.Hash() == receipt.originTxHash {
				if receipt.err != nil {
					return receipt.err
				}
				if receipt.Status == uint64(0) {
					return fmt.Errorf("transaction failed txHash=%v", receipt.TxHash.Hex())
				}
				return nil
			}
		}
	}
}

func (c *client) Sign(msg []byte) ([]byte, error) {
	return c.accountManager.Sign(msg)
}

func (c *client) SignTypedData(typedData apitypes.TypedData) ([]byte, error) {
	return c.accountManager.SignTypedData(typedData)
}
