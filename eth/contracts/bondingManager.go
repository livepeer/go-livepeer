// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package contracts

import (
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

// BondingManagerABI is the input ABI used to generate the binding from.
const BondingManagerABI = "[{\"constant\":true,\"inputs\":[],\"name\":\"maxEarningsClaimsRounds\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"activeTranscoderSet\",\"outputs\":[{\"name\":\"totalStake\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"targetContractId\",\"outputs\":[{\"name\":\"\",\"type\":\"bytes32\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"numActiveTranscoders\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"unbondingPeriod\",\"outputs\":[{\"name\":\"\",\"type\":\"uint64\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_controller\",\"type\":\"address\"}],\"name\":\"setController\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"controller\",\"outputs\":[{\"name\":\"\",\"type\":\"address\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"name\":\"_controller\",\"type\":\"address\"}],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"constructor\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"name\":\"transcoder\",\"type\":\"address\"},{\"indexed\":false,\"name\":\"pendingRewardCut\",\"type\":\"uint256\"},{\"indexed\":false,\"name\":\"pendingFeeShare\",\"type\":\"uint256\"},{\"indexed\":false,\"name\":\"pendingPricePerSegment\",\"type\":\"uint256\"},{\"indexed\":false,\"name\":\"registered\",\"type\":\"bool\"}],\"name\":\"TranscoderUpdate\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"name\":\"transcoder\",\"type\":\"address\"}],\"name\":\"TranscoderEvicted\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"name\":\"transcoder\",\"type\":\"address\"}],\"name\":\"TranscoderResigned\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"name\":\"transcoder\",\"type\":\"address\"},{\"indexed\":false,\"name\":\"finder\",\"type\":\"address\"},{\"indexed\":false,\"name\":\"penalty\",\"type\":\"uint256\"},{\"indexed\":false,\"name\":\"finderReward\",\"type\":\"uint256\"}],\"name\":\"TranscoderSlashed\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"name\":\"transcoder\",\"type\":\"address\"},{\"indexed\":false,\"name\":\"amount\",\"type\":\"uint256\"}],\"name\":\"Reward\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"name\":\"delegate\",\"type\":\"address\"},{\"indexed\":true,\"name\":\"delegator\",\"type\":\"address\"}],\"name\":\"Bond\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"name\":\"delegate\",\"type\":\"address\"},{\"indexed\":true,\"name\":\"delegator\",\"type\":\"address\"}],\"name\":\"Unbond\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"name\":\"delegator\",\"type\":\"address\"}],\"name\":\"WithdrawStake\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"name\":\"delegator\",\"type\":\"address\"}],\"name\":\"WithdrawFees\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"name\":\"controller\",\"type\":\"address\"}],\"name\":\"SetController\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"name\":\"param\",\"type\":\"string\"}],\"name\":\"ParameterUpdate\",\"type\":\"event\"},{\"constant\":false,\"inputs\":[{\"name\":\"_unbondingPeriod\",\"type\":\"uint64\"}],\"name\":\"setUnbondingPeriod\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_numTranscoders\",\"type\":\"uint256\"}],\"name\":\"setNumTranscoders\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_numActiveTranscoders\",\"type\":\"uint256\"}],\"name\":\"setNumActiveTranscoders\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_maxEarningsClaimsRounds\",\"type\":\"uint256\"}],\"name\":\"setMaxEarningsClaimsRounds\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_rewardCut\",\"type\":\"uint256\"},{\"name\":\"_feeShare\",\"type\":\"uint256\"},{\"name\":\"_pricePerSegment\",\"type\":\"uint256\"}],\"name\":\"transcoder\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_amount\",\"type\":\"uint256\"},{\"name\":\"_to\",\"type\":\"address\"}],\"name\":\"bond\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[],\"name\":\"unbond\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[],\"name\":\"withdrawStake\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[],\"name\":\"withdrawFees\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[],\"name\":\"setActiveTranscoders\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[],\"name\":\"reward\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_transcoder\",\"type\":\"address\"},{\"name\":\"_fees\",\"type\":\"uint256\"},{\"name\":\"_round\",\"type\":\"uint256\"}],\"name\":\"updateTranscoderWithFees\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_transcoder\",\"type\":\"address\"},{\"name\":\"_finder\",\"type\":\"address\"},{\"name\":\"_slashAmount\",\"type\":\"uint256\"},{\"name\":\"_finderFee\",\"type\":\"uint256\"}],\"name\":\"slashTranscoder\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"_maxPricePerSegment\",\"type\":\"uint256\"},{\"name\":\"_blockHash\",\"type\":\"bytes32\"},{\"name\":\"_round\",\"type\":\"uint256\"}],\"name\":\"electActiveTranscoder\",\"outputs\":[{\"name\":\"\",\"type\":\"address\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_endRound\",\"type\":\"uint256\"}],\"name\":\"claimEarnings\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"_delegator\",\"type\":\"address\"},{\"name\":\"_endRound\",\"type\":\"uint256\"}],\"name\":\"pendingStake\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"_delegator\",\"type\":\"address\"},{\"name\":\"_endRound\",\"type\":\"uint256\"}],\"name\":\"pendingFees\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"_transcoder\",\"type\":\"address\"},{\"name\":\"_round\",\"type\":\"uint256\"}],\"name\":\"activeTranscoderTotalStake\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"_transcoder\",\"type\":\"address\"}],\"name\":\"transcoderTotalStake\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"_transcoder\",\"type\":\"address\"}],\"name\":\"transcoderStatus\",\"outputs\":[{\"name\":\"\",\"type\":\"uint8\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"_delegator\",\"type\":\"address\"}],\"name\":\"delegatorStatus\",\"outputs\":[{\"name\":\"\",\"type\":\"uint8\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"_transcoder\",\"type\":\"address\"}],\"name\":\"getTranscoder\",\"outputs\":[{\"name\":\"lastRewardRound\",\"type\":\"uint256\"},{\"name\":\"rewardCut\",\"type\":\"uint256\"},{\"name\":\"feeShare\",\"type\":\"uint256\"},{\"name\":\"pricePerSegment\",\"type\":\"uint256\"},{\"name\":\"pendingRewardCut\",\"type\":\"uint256\"},{\"name\":\"pendingFeeShare\",\"type\":\"uint256\"},{\"name\":\"pendingPricePerSegment\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"_transcoder\",\"type\":\"address\"},{\"name\":\"_round\",\"type\":\"uint256\"}],\"name\":\"getTranscoderEarningsPoolForRound\",\"outputs\":[{\"name\":\"rewardPool\",\"type\":\"uint256\"},{\"name\":\"feePool\",\"type\":\"uint256\"},{\"name\":\"totalStake\",\"type\":\"uint256\"},{\"name\":\"claimableStake\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"_delegator\",\"type\":\"address\"}],\"name\":\"getDelegator\",\"outputs\":[{\"name\":\"bondedAmount\",\"type\":\"uint256\"},{\"name\":\"fees\",\"type\":\"uint256\"},{\"name\":\"delegateAddress\",\"type\":\"address\"},{\"name\":\"delegatedAmount\",\"type\":\"uint256\"},{\"name\":\"startRound\",\"type\":\"uint256\"},{\"name\":\"withdrawRound\",\"type\":\"uint256\"},{\"name\":\"lastClaimRound\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"getTranscoderPoolMaxSize\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"getTranscoderPoolSize\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"getFirstTranscoderInPool\",\"outputs\":[{\"name\":\"\",\"type\":\"address\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"_transcoder\",\"type\":\"address\"}],\"name\":\"getNextTranscoderInPool\",\"outputs\":[{\"name\":\"\",\"type\":\"address\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"getTotalBonded\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"_round\",\"type\":\"uint256\"}],\"name\":\"getTotalActiveStake\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"_transcoder\",\"type\":\"address\"},{\"name\":\"_round\",\"type\":\"uint256\"}],\"name\":\"isActiveTranscoder\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"_transcoder\",\"type\":\"address\"}],\"name\":\"isRegisteredTranscoder\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"}]"

// BondingManager is an auto generated Go binding around an Ethereum contract.
type BondingManager struct {
	BondingManagerCaller     // Read-only binding to the contract
	BondingManagerTransactor // Write-only binding to the contract
}

// BondingManagerCaller is an auto generated read-only Go binding around an Ethereum contract.
type BondingManagerCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// BondingManagerTransactor is an auto generated write-only Go binding around an Ethereum contract.
type BondingManagerTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// BondingManagerSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type BondingManagerSession struct {
	Contract     *BondingManager   // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// BondingManagerCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type BondingManagerCallerSession struct {
	Contract *BondingManagerCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts         // Call options to use throughout this session
}

// BondingManagerTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type BondingManagerTransactorSession struct {
	Contract     *BondingManagerTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts         // Transaction auth options to use throughout this session
}

// BondingManagerRaw is an auto generated low-level Go binding around an Ethereum contract.
type BondingManagerRaw struct {
	Contract *BondingManager // Generic contract binding to access the raw methods on
}

// BondingManagerCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type BondingManagerCallerRaw struct {
	Contract *BondingManagerCaller // Generic read-only contract binding to access the raw methods on
}

// BondingManagerTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type BondingManagerTransactorRaw struct {
	Contract *BondingManagerTransactor // Generic write-only contract binding to access the raw methods on
}

// NewBondingManager creates a new instance of BondingManager, bound to a specific deployed contract.
func NewBondingManager(address common.Address, backend bind.ContractBackend) (*BondingManager, error) {
	contract, err := bindBondingManager(address, backend, backend)
	if err != nil {
		return nil, err
	}
	return &BondingManager{BondingManagerCaller: BondingManagerCaller{contract: contract}, BondingManagerTransactor: BondingManagerTransactor{contract: contract}}, nil
}

// NewBondingManagerCaller creates a new read-only instance of BondingManager, bound to a specific deployed contract.
func NewBondingManagerCaller(address common.Address, caller bind.ContractCaller) (*BondingManagerCaller, error) {
	contract, err := bindBondingManager(address, caller, nil)
	if err != nil {
		return nil, err
	}
	return &BondingManagerCaller{contract: contract}, nil
}

// NewBondingManagerTransactor creates a new write-only instance of BondingManager, bound to a specific deployed contract.
func NewBondingManagerTransactor(address common.Address, transactor bind.ContractTransactor) (*BondingManagerTransactor, error) {
	contract, err := bindBondingManager(address, nil, transactor)
	if err != nil {
		return nil, err
	}
	return &BondingManagerTransactor{contract: contract}, nil
}

// bindBondingManager binds a generic wrapper to an already deployed contract.
func bindBondingManager(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor) (*bind.BoundContract, error) {
	parsed, err := abi.JSON(strings.NewReader(BondingManagerABI))
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, parsed, caller, transactor), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_BondingManager *BondingManagerRaw) Call(opts *bind.CallOpts, result interface{}, method string, params ...interface{}) error {
	return _BondingManager.Contract.BondingManagerCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_BondingManager *BondingManagerRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _BondingManager.Contract.BondingManagerTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_BondingManager *BondingManagerRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _BondingManager.Contract.BondingManagerTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_BondingManager *BondingManagerCallerRaw) Call(opts *bind.CallOpts, result interface{}, method string, params ...interface{}) error {
	return _BondingManager.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_BondingManager *BondingManagerTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _BondingManager.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_BondingManager *BondingManagerTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _BondingManager.Contract.contract.Transact(opts, method, params...)
}

// ActiveTranscoderSet is a free data retrieval call binding the contract method 0x3da1c2f5.
//
// Solidity: function activeTranscoderSet( uint256) constant returns(totalStake uint256)
func (_BondingManager *BondingManagerCaller) ActiveTranscoderSet(opts *bind.CallOpts, arg0 *big.Int) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _BondingManager.contract.Call(opts, out, "activeTranscoderSet", arg0)
	return *ret0, err
}

// ActiveTranscoderSet is a free data retrieval call binding the contract method 0x3da1c2f5.
//
// Solidity: function activeTranscoderSet( uint256) constant returns(totalStake uint256)
func (_BondingManager *BondingManagerSession) ActiveTranscoderSet(arg0 *big.Int) (*big.Int, error) {
	return _BondingManager.Contract.ActiveTranscoderSet(&_BondingManager.CallOpts, arg0)
}

// ActiveTranscoderSet is a free data retrieval call binding the contract method 0x3da1c2f5.
//
// Solidity: function activeTranscoderSet( uint256) constant returns(totalStake uint256)
func (_BondingManager *BondingManagerCallerSession) ActiveTranscoderSet(arg0 *big.Int) (*big.Int, error) {
	return _BondingManager.Contract.ActiveTranscoderSet(&_BondingManager.CallOpts, arg0)
}

// ActiveTranscoderTotalStake is a free data retrieval call binding the contract method 0xf2083220.
//
// Solidity: function activeTranscoderTotalStake(_transcoder address, _round uint256) constant returns(uint256)
func (_BondingManager *BondingManagerCaller) ActiveTranscoderTotalStake(opts *bind.CallOpts, _transcoder common.Address, _round *big.Int) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _BondingManager.contract.Call(opts, out, "activeTranscoderTotalStake", _transcoder, _round)
	return *ret0, err
}

// ActiveTranscoderTotalStake is a free data retrieval call binding the contract method 0xf2083220.
//
// Solidity: function activeTranscoderTotalStake(_transcoder address, _round uint256) constant returns(uint256)
func (_BondingManager *BondingManagerSession) ActiveTranscoderTotalStake(_transcoder common.Address, _round *big.Int) (*big.Int, error) {
	return _BondingManager.Contract.ActiveTranscoderTotalStake(&_BondingManager.CallOpts, _transcoder, _round)
}

// ActiveTranscoderTotalStake is a free data retrieval call binding the contract method 0xf2083220.
//
// Solidity: function activeTranscoderTotalStake(_transcoder address, _round uint256) constant returns(uint256)
func (_BondingManager *BondingManagerCallerSession) ActiveTranscoderTotalStake(_transcoder common.Address, _round *big.Int) (*big.Int, error) {
	return _BondingManager.Contract.ActiveTranscoderTotalStake(&_BondingManager.CallOpts, _transcoder, _round)
}

// Controller is a free data retrieval call binding the contract method 0xf77c4791.
//
// Solidity: function controller() constant returns(address)
func (_BondingManager *BondingManagerCaller) Controller(opts *bind.CallOpts) (common.Address, error) {
	var (
		ret0 = new(common.Address)
	)
	out := ret0
	err := _BondingManager.contract.Call(opts, out, "controller")
	return *ret0, err
}

// Controller is a free data retrieval call binding the contract method 0xf77c4791.
//
// Solidity: function controller() constant returns(address)
func (_BondingManager *BondingManagerSession) Controller() (common.Address, error) {
	return _BondingManager.Contract.Controller(&_BondingManager.CallOpts)
}

// Controller is a free data retrieval call binding the contract method 0xf77c4791.
//
// Solidity: function controller() constant returns(address)
func (_BondingManager *BondingManagerCallerSession) Controller() (common.Address, error) {
	return _BondingManager.Contract.Controller(&_BondingManager.CallOpts)
}

// DelegatorStatus is a free data retrieval call binding the contract method 0x1544fc67.
//
// Solidity: function delegatorStatus(_delegator address) constant returns(uint8)
func (_BondingManager *BondingManagerCaller) DelegatorStatus(opts *bind.CallOpts, _delegator common.Address) (uint8, error) {
	var (
		ret0 = new(uint8)
	)
	out := ret0
	err := _BondingManager.contract.Call(opts, out, "delegatorStatus", _delegator)
	return *ret0, err
}

// DelegatorStatus is a free data retrieval call binding the contract method 0x1544fc67.
//
// Solidity: function delegatorStatus(_delegator address) constant returns(uint8)
func (_BondingManager *BondingManagerSession) DelegatorStatus(_delegator common.Address) (uint8, error) {
	return _BondingManager.Contract.DelegatorStatus(&_BondingManager.CallOpts, _delegator)
}

// DelegatorStatus is a free data retrieval call binding the contract method 0x1544fc67.
//
// Solidity: function delegatorStatus(_delegator address) constant returns(uint8)
func (_BondingManager *BondingManagerCallerSession) DelegatorStatus(_delegator common.Address) (uint8, error) {
	return _BondingManager.Contract.DelegatorStatus(&_BondingManager.CallOpts, _delegator)
}

// ElectActiveTranscoder is a free data retrieval call binding the contract method 0x91fdf6b1.
//
// Solidity: function electActiveTranscoder(_maxPricePerSegment uint256, _blockHash bytes32, _round uint256) constant returns(address)
func (_BondingManager *BondingManagerCaller) ElectActiveTranscoder(opts *bind.CallOpts, _maxPricePerSegment *big.Int, _blockHash [32]byte, _round *big.Int) (common.Address, error) {
	var (
		ret0 = new(common.Address)
	)
	out := ret0
	err := _BondingManager.contract.Call(opts, out, "electActiveTranscoder", _maxPricePerSegment, _blockHash, _round)
	return *ret0, err
}

// ElectActiveTranscoder is a free data retrieval call binding the contract method 0x91fdf6b1.
//
// Solidity: function electActiveTranscoder(_maxPricePerSegment uint256, _blockHash bytes32, _round uint256) constant returns(address)
func (_BondingManager *BondingManagerSession) ElectActiveTranscoder(_maxPricePerSegment *big.Int, _blockHash [32]byte, _round *big.Int) (common.Address, error) {
	return _BondingManager.Contract.ElectActiveTranscoder(&_BondingManager.CallOpts, _maxPricePerSegment, _blockHash, _round)
}

// ElectActiveTranscoder is a free data retrieval call binding the contract method 0x91fdf6b1.
//
// Solidity: function electActiveTranscoder(_maxPricePerSegment uint256, _blockHash bytes32, _round uint256) constant returns(address)
func (_BondingManager *BondingManagerCallerSession) ElectActiveTranscoder(_maxPricePerSegment *big.Int, _blockHash [32]byte, _round *big.Int) (common.Address, error) {
	return _BondingManager.Contract.ElectActiveTranscoder(&_BondingManager.CallOpts, _maxPricePerSegment, _blockHash, _round)
}

// GetDelegator is a free data retrieval call binding the contract method 0xa64ad595.
//
// Solidity: function getDelegator(_delegator address) constant returns(bondedAmount uint256, fees uint256, delegateAddress address, delegatedAmount uint256, startRound uint256, withdrawRound uint256, lastClaimRound uint256)
func (_BondingManager *BondingManagerCaller) GetDelegator(opts *bind.CallOpts, _delegator common.Address) (struct {
	BondedAmount    *big.Int
	Fees            *big.Int
	DelegateAddress common.Address
	DelegatedAmount *big.Int
	StartRound      *big.Int
	WithdrawRound   *big.Int
	LastClaimRound  *big.Int
}, error) {
	ret := new(struct {
		BondedAmount    *big.Int
		Fees            *big.Int
		DelegateAddress common.Address
		DelegatedAmount *big.Int
		StartRound      *big.Int
		WithdrawRound   *big.Int
		LastClaimRound  *big.Int
	})
	out := ret
	err := _BondingManager.contract.Call(opts, out, "getDelegator", _delegator)
	return *ret, err
}

// GetDelegator is a free data retrieval call binding the contract method 0xa64ad595.
//
// Solidity: function getDelegator(_delegator address) constant returns(bondedAmount uint256, fees uint256, delegateAddress address, delegatedAmount uint256, startRound uint256, withdrawRound uint256, lastClaimRound uint256)
func (_BondingManager *BondingManagerSession) GetDelegator(_delegator common.Address) (struct {
	BondedAmount    *big.Int
	Fees            *big.Int
	DelegateAddress common.Address
	DelegatedAmount *big.Int
	StartRound      *big.Int
	WithdrawRound   *big.Int
	LastClaimRound  *big.Int
}, error) {
	return _BondingManager.Contract.GetDelegator(&_BondingManager.CallOpts, _delegator)
}

// GetDelegator is a free data retrieval call binding the contract method 0xa64ad595.
//
// Solidity: function getDelegator(_delegator address) constant returns(bondedAmount uint256, fees uint256, delegateAddress address, delegatedAmount uint256, startRound uint256, withdrawRound uint256, lastClaimRound uint256)
func (_BondingManager *BondingManagerCallerSession) GetDelegator(_delegator common.Address) (struct {
	BondedAmount    *big.Int
	Fees            *big.Int
	DelegateAddress common.Address
	DelegatedAmount *big.Int
	StartRound      *big.Int
	WithdrawRound   *big.Int
	LastClaimRound  *big.Int
}, error) {
	return _BondingManager.Contract.GetDelegator(&_BondingManager.CallOpts, _delegator)
}

// GetFirstTranscoderInPool is a free data retrieval call binding the contract method 0x88a6c749.
//
// Solidity: function getFirstTranscoderInPool() constant returns(address)
func (_BondingManager *BondingManagerCaller) GetFirstTranscoderInPool(opts *bind.CallOpts) (common.Address, error) {
	var (
		ret0 = new(common.Address)
	)
	out := ret0
	err := _BondingManager.contract.Call(opts, out, "getFirstTranscoderInPool")
	return *ret0, err
}

// GetFirstTranscoderInPool is a free data retrieval call binding the contract method 0x88a6c749.
//
// Solidity: function getFirstTranscoderInPool() constant returns(address)
func (_BondingManager *BondingManagerSession) GetFirstTranscoderInPool() (common.Address, error) {
	return _BondingManager.Contract.GetFirstTranscoderInPool(&_BondingManager.CallOpts)
}

// GetFirstTranscoderInPool is a free data retrieval call binding the contract method 0x88a6c749.
//
// Solidity: function getFirstTranscoderInPool() constant returns(address)
func (_BondingManager *BondingManagerCallerSession) GetFirstTranscoderInPool() (common.Address, error) {
	return _BondingManager.Contract.GetFirstTranscoderInPool(&_BondingManager.CallOpts)
}

// GetNextTranscoderInPool is a free data retrieval call binding the contract method 0x235c9603.
//
// Solidity: function getNextTranscoderInPool(_transcoder address) constant returns(address)
func (_BondingManager *BondingManagerCaller) GetNextTranscoderInPool(opts *bind.CallOpts, _transcoder common.Address) (common.Address, error) {
	var (
		ret0 = new(common.Address)
	)
	out := ret0
	err := _BondingManager.contract.Call(opts, out, "getNextTranscoderInPool", _transcoder)
	return *ret0, err
}

// GetNextTranscoderInPool is a free data retrieval call binding the contract method 0x235c9603.
//
// Solidity: function getNextTranscoderInPool(_transcoder address) constant returns(address)
func (_BondingManager *BondingManagerSession) GetNextTranscoderInPool(_transcoder common.Address) (common.Address, error) {
	return _BondingManager.Contract.GetNextTranscoderInPool(&_BondingManager.CallOpts, _transcoder)
}

// GetNextTranscoderInPool is a free data retrieval call binding the contract method 0x235c9603.
//
// Solidity: function getNextTranscoderInPool(_transcoder address) constant returns(address)
func (_BondingManager *BondingManagerCallerSession) GetNextTranscoderInPool(_transcoder common.Address) (common.Address, error) {
	return _BondingManager.Contract.GetNextTranscoderInPool(&_BondingManager.CallOpts, _transcoder)
}

// GetTotalActiveStake is a free data retrieval call binding the contract method 0x77517765.
//
// Solidity: function getTotalActiveStake(_round uint256) constant returns(uint256)
func (_BondingManager *BondingManagerCaller) GetTotalActiveStake(opts *bind.CallOpts, _round *big.Int) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _BondingManager.contract.Call(opts, out, "getTotalActiveStake", _round)
	return *ret0, err
}

// GetTotalActiveStake is a free data retrieval call binding the contract method 0x77517765.
//
// Solidity: function getTotalActiveStake(_round uint256) constant returns(uint256)
func (_BondingManager *BondingManagerSession) GetTotalActiveStake(_round *big.Int) (*big.Int, error) {
	return _BondingManager.Contract.GetTotalActiveStake(&_BondingManager.CallOpts, _round)
}

// GetTotalActiveStake is a free data retrieval call binding the contract method 0x77517765.
//
// Solidity: function getTotalActiveStake(_round uint256) constant returns(uint256)
func (_BondingManager *BondingManagerCallerSession) GetTotalActiveStake(_round *big.Int) (*big.Int, error) {
	return _BondingManager.Contract.GetTotalActiveStake(&_BondingManager.CallOpts, _round)
}

// GetTotalBonded is a free data retrieval call binding the contract method 0x5c50c356.
//
// Solidity: function getTotalBonded() constant returns(uint256)
func (_BondingManager *BondingManagerCaller) GetTotalBonded(opts *bind.CallOpts) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _BondingManager.contract.Call(opts, out, "getTotalBonded")
	return *ret0, err
}

// GetTotalBonded is a free data retrieval call binding the contract method 0x5c50c356.
//
// Solidity: function getTotalBonded() constant returns(uint256)
func (_BondingManager *BondingManagerSession) GetTotalBonded() (*big.Int, error) {
	return _BondingManager.Contract.GetTotalBonded(&_BondingManager.CallOpts)
}

// GetTotalBonded is a free data retrieval call binding the contract method 0x5c50c356.
//
// Solidity: function getTotalBonded() constant returns(uint256)
func (_BondingManager *BondingManagerCallerSession) GetTotalBonded() (*big.Int, error) {
	return _BondingManager.Contract.GetTotalBonded(&_BondingManager.CallOpts)
}

// GetTranscoder is a free data retrieval call binding the contract method 0x5dce9948.
//
// Solidity: function getTranscoder(_transcoder address) constant returns(lastRewardRound uint256, rewardCut uint256, feeShare uint256, pricePerSegment uint256, pendingRewardCut uint256, pendingFeeShare uint256, pendingPricePerSegment uint256)
func (_BondingManager *BondingManagerCaller) GetTranscoder(opts *bind.CallOpts, _transcoder common.Address) (struct {
	LastRewardRound        *big.Int
	RewardCut              *big.Int
	FeeShare               *big.Int
	PricePerSegment        *big.Int
	PendingRewardCut       *big.Int
	PendingFeeShare        *big.Int
	PendingPricePerSegment *big.Int
}, error) {
	ret := new(struct {
		LastRewardRound        *big.Int
		RewardCut              *big.Int
		FeeShare               *big.Int
		PricePerSegment        *big.Int
		PendingRewardCut       *big.Int
		PendingFeeShare        *big.Int
		PendingPricePerSegment *big.Int
	})
	out := ret
	err := _BondingManager.contract.Call(opts, out, "getTranscoder", _transcoder)
	return *ret, err
}

// GetTranscoder is a free data retrieval call binding the contract method 0x5dce9948.
//
// Solidity: function getTranscoder(_transcoder address) constant returns(lastRewardRound uint256, rewardCut uint256, feeShare uint256, pricePerSegment uint256, pendingRewardCut uint256, pendingFeeShare uint256, pendingPricePerSegment uint256)
func (_BondingManager *BondingManagerSession) GetTranscoder(_transcoder common.Address) (struct {
	LastRewardRound        *big.Int
	RewardCut              *big.Int
	FeeShare               *big.Int
	PricePerSegment        *big.Int
	PendingRewardCut       *big.Int
	PendingFeeShare        *big.Int
	PendingPricePerSegment *big.Int
}, error) {
	return _BondingManager.Contract.GetTranscoder(&_BondingManager.CallOpts, _transcoder)
}

// GetTranscoder is a free data retrieval call binding the contract method 0x5dce9948.
//
// Solidity: function getTranscoder(_transcoder address) constant returns(lastRewardRound uint256, rewardCut uint256, feeShare uint256, pricePerSegment uint256, pendingRewardCut uint256, pendingFeeShare uint256, pendingPricePerSegment uint256)
func (_BondingManager *BondingManagerCallerSession) GetTranscoder(_transcoder common.Address) (struct {
	LastRewardRound        *big.Int
	RewardCut              *big.Int
	FeeShare               *big.Int
	PricePerSegment        *big.Int
	PendingRewardCut       *big.Int
	PendingFeeShare        *big.Int
	PendingPricePerSegment *big.Int
}, error) {
	return _BondingManager.Contract.GetTranscoder(&_BondingManager.CallOpts, _transcoder)
}

// GetTranscoderEarningsPoolForRound is a free data retrieval call binding the contract method 0x24454fc4.
//
// Solidity: function getTranscoderEarningsPoolForRound(_transcoder address, _round uint256) constant returns(rewardPool uint256, feePool uint256, totalStake uint256, claimableStake uint256)
func (_BondingManager *BondingManagerCaller) GetTranscoderEarningsPoolForRound(opts *bind.CallOpts, _transcoder common.Address, _round *big.Int) (struct {
	RewardPool     *big.Int
	FeePool        *big.Int
	TotalStake     *big.Int
	ClaimableStake *big.Int
}, error) {
	ret := new(struct {
		RewardPool     *big.Int
		FeePool        *big.Int
		TotalStake     *big.Int
		ClaimableStake *big.Int
	})
	out := ret
	err := _BondingManager.contract.Call(opts, out, "getTranscoderEarningsPoolForRound", _transcoder, _round)
	return *ret, err
}

// GetTranscoderEarningsPoolForRound is a free data retrieval call binding the contract method 0x24454fc4.
//
// Solidity: function getTranscoderEarningsPoolForRound(_transcoder address, _round uint256) constant returns(rewardPool uint256, feePool uint256, totalStake uint256, claimableStake uint256)
func (_BondingManager *BondingManagerSession) GetTranscoderEarningsPoolForRound(_transcoder common.Address, _round *big.Int) (struct {
	RewardPool     *big.Int
	FeePool        *big.Int
	TotalStake     *big.Int
	ClaimableStake *big.Int
}, error) {
	return _BondingManager.Contract.GetTranscoderEarningsPoolForRound(&_BondingManager.CallOpts, _transcoder, _round)
}

// GetTranscoderEarningsPoolForRound is a free data retrieval call binding the contract method 0x24454fc4.
//
// Solidity: function getTranscoderEarningsPoolForRound(_transcoder address, _round uint256) constant returns(rewardPool uint256, feePool uint256, totalStake uint256, claimableStake uint256)
func (_BondingManager *BondingManagerCallerSession) GetTranscoderEarningsPoolForRound(_transcoder common.Address, _round *big.Int) (struct {
	RewardPool     *big.Int
	FeePool        *big.Int
	TotalStake     *big.Int
	ClaimableStake *big.Int
}, error) {
	return _BondingManager.Contract.GetTranscoderEarningsPoolForRound(&_BondingManager.CallOpts, _transcoder, _round)
}

// GetTranscoderPoolMaxSize is a free data retrieval call binding the contract method 0x5a2a75a9.
//
// Solidity: function getTranscoderPoolMaxSize() constant returns(uint256)
func (_BondingManager *BondingManagerCaller) GetTranscoderPoolMaxSize(opts *bind.CallOpts) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _BondingManager.contract.Call(opts, out, "getTranscoderPoolMaxSize")
	return *ret0, err
}

// GetTranscoderPoolMaxSize is a free data retrieval call binding the contract method 0x5a2a75a9.
//
// Solidity: function getTranscoderPoolMaxSize() constant returns(uint256)
func (_BondingManager *BondingManagerSession) GetTranscoderPoolMaxSize() (*big.Int, error) {
	return _BondingManager.Contract.GetTranscoderPoolMaxSize(&_BondingManager.CallOpts)
}

// GetTranscoderPoolMaxSize is a free data retrieval call binding the contract method 0x5a2a75a9.
//
// Solidity: function getTranscoderPoolMaxSize() constant returns(uint256)
func (_BondingManager *BondingManagerCallerSession) GetTranscoderPoolMaxSize() (*big.Int, error) {
	return _BondingManager.Contract.GetTranscoderPoolMaxSize(&_BondingManager.CallOpts)
}

// GetTranscoderPoolSize is a free data retrieval call binding the contract method 0x2a4e0d55.
//
// Solidity: function getTranscoderPoolSize() constant returns(uint256)
func (_BondingManager *BondingManagerCaller) GetTranscoderPoolSize(opts *bind.CallOpts) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _BondingManager.contract.Call(opts, out, "getTranscoderPoolSize")
	return *ret0, err
}

// GetTranscoderPoolSize is a free data retrieval call binding the contract method 0x2a4e0d55.
//
// Solidity: function getTranscoderPoolSize() constant returns(uint256)
func (_BondingManager *BondingManagerSession) GetTranscoderPoolSize() (*big.Int, error) {
	return _BondingManager.Contract.GetTranscoderPoolSize(&_BondingManager.CallOpts)
}

// GetTranscoderPoolSize is a free data retrieval call binding the contract method 0x2a4e0d55.
//
// Solidity: function getTranscoderPoolSize() constant returns(uint256)
func (_BondingManager *BondingManagerCallerSession) GetTranscoderPoolSize() (*big.Int, error) {
	return _BondingManager.Contract.GetTranscoderPoolSize(&_BondingManager.CallOpts)
}

// IsActiveTranscoder is a free data retrieval call binding the contract method 0x7c0207cb.
//
// Solidity: function isActiveTranscoder(_transcoder address, _round uint256) constant returns(bool)
func (_BondingManager *BondingManagerCaller) IsActiveTranscoder(opts *bind.CallOpts, _transcoder common.Address, _round *big.Int) (bool, error) {
	var (
		ret0 = new(bool)
	)
	out := ret0
	err := _BondingManager.contract.Call(opts, out, "isActiveTranscoder", _transcoder, _round)
	return *ret0, err
}

// IsActiveTranscoder is a free data retrieval call binding the contract method 0x7c0207cb.
//
// Solidity: function isActiveTranscoder(_transcoder address, _round uint256) constant returns(bool)
func (_BondingManager *BondingManagerSession) IsActiveTranscoder(_transcoder common.Address, _round *big.Int) (bool, error) {
	return _BondingManager.Contract.IsActiveTranscoder(&_BondingManager.CallOpts, _transcoder, _round)
}

// IsActiveTranscoder is a free data retrieval call binding the contract method 0x7c0207cb.
//
// Solidity: function isActiveTranscoder(_transcoder address, _round uint256) constant returns(bool)
func (_BondingManager *BondingManagerCallerSession) IsActiveTranscoder(_transcoder common.Address, _round *big.Int) (bool, error) {
	return _BondingManager.Contract.IsActiveTranscoder(&_BondingManager.CallOpts, _transcoder, _round)
}

// IsRegisteredTranscoder is a free data retrieval call binding the contract method 0x68ba170c.
//
// Solidity: function isRegisteredTranscoder(_transcoder address) constant returns(bool)
func (_BondingManager *BondingManagerCaller) IsRegisteredTranscoder(opts *bind.CallOpts, _transcoder common.Address) (bool, error) {
	var (
		ret0 = new(bool)
	)
	out := ret0
	err := _BondingManager.contract.Call(opts, out, "isRegisteredTranscoder", _transcoder)
	return *ret0, err
}

// IsRegisteredTranscoder is a free data retrieval call binding the contract method 0x68ba170c.
//
// Solidity: function isRegisteredTranscoder(_transcoder address) constant returns(bool)
func (_BondingManager *BondingManagerSession) IsRegisteredTranscoder(_transcoder common.Address) (bool, error) {
	return _BondingManager.Contract.IsRegisteredTranscoder(&_BondingManager.CallOpts, _transcoder)
}

// IsRegisteredTranscoder is a free data retrieval call binding the contract method 0x68ba170c.
//
// Solidity: function isRegisteredTranscoder(_transcoder address) constant returns(bool)
func (_BondingManager *BondingManagerCallerSession) IsRegisteredTranscoder(_transcoder common.Address) (bool, error) {
	return _BondingManager.Contract.IsRegisteredTranscoder(&_BondingManager.CallOpts, _transcoder)
}

// MaxEarningsClaimsRounds is a free data retrieval call binding the contract method 0x038424c3.
//
// Solidity: function maxEarningsClaimsRounds() constant returns(uint256)
func (_BondingManager *BondingManagerCaller) MaxEarningsClaimsRounds(opts *bind.CallOpts) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _BondingManager.contract.Call(opts, out, "maxEarningsClaimsRounds")
	return *ret0, err
}

// MaxEarningsClaimsRounds is a free data retrieval call binding the contract method 0x038424c3.
//
// Solidity: function maxEarningsClaimsRounds() constant returns(uint256)
func (_BondingManager *BondingManagerSession) MaxEarningsClaimsRounds() (*big.Int, error) {
	return _BondingManager.Contract.MaxEarningsClaimsRounds(&_BondingManager.CallOpts)
}

// MaxEarningsClaimsRounds is a free data retrieval call binding the contract method 0x038424c3.
//
// Solidity: function maxEarningsClaimsRounds() constant returns(uint256)
func (_BondingManager *BondingManagerCallerSession) MaxEarningsClaimsRounds() (*big.Int, error) {
	return _BondingManager.Contract.MaxEarningsClaimsRounds(&_BondingManager.CallOpts)
}

// NumActiveTranscoders is a free data retrieval call binding the contract method 0x61e25d23.
//
// Solidity: function numActiveTranscoders() constant returns(uint256)
func (_BondingManager *BondingManagerCaller) NumActiveTranscoders(opts *bind.CallOpts) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _BondingManager.contract.Call(opts, out, "numActiveTranscoders")
	return *ret0, err
}

// NumActiveTranscoders is a free data retrieval call binding the contract method 0x61e25d23.
//
// Solidity: function numActiveTranscoders() constant returns(uint256)
func (_BondingManager *BondingManagerSession) NumActiveTranscoders() (*big.Int, error) {
	return _BondingManager.Contract.NumActiveTranscoders(&_BondingManager.CallOpts)
}

// NumActiveTranscoders is a free data retrieval call binding the contract method 0x61e25d23.
//
// Solidity: function numActiveTranscoders() constant returns(uint256)
func (_BondingManager *BondingManagerCallerSession) NumActiveTranscoders() (*big.Int, error) {
	return _BondingManager.Contract.NumActiveTranscoders(&_BondingManager.CallOpts)
}

// PendingFees is a free data retrieval call binding the contract method 0xf595f1cc.
//
// Solidity: function pendingFees(_delegator address, _endRound uint256) constant returns(uint256)
func (_BondingManager *BondingManagerCaller) PendingFees(opts *bind.CallOpts, _delegator common.Address, _endRound *big.Int) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _BondingManager.contract.Call(opts, out, "pendingFees", _delegator, _endRound)
	return *ret0, err
}

// PendingFees is a free data retrieval call binding the contract method 0xf595f1cc.
//
// Solidity: function pendingFees(_delegator address, _endRound uint256) constant returns(uint256)
func (_BondingManager *BondingManagerSession) PendingFees(_delegator common.Address, _endRound *big.Int) (*big.Int, error) {
	return _BondingManager.Contract.PendingFees(&_BondingManager.CallOpts, _delegator, _endRound)
}

// PendingFees is a free data retrieval call binding the contract method 0xf595f1cc.
//
// Solidity: function pendingFees(_delegator address, _endRound uint256) constant returns(uint256)
func (_BondingManager *BondingManagerCallerSession) PendingFees(_delegator common.Address, _endRound *big.Int) (*big.Int, error) {
	return _BondingManager.Contract.PendingFees(&_BondingManager.CallOpts, _delegator, _endRound)
}

// PendingStake is a free data retrieval call binding the contract method 0x9d0b2c7a.
//
// Solidity: function pendingStake(_delegator address, _endRound uint256) constant returns(uint256)
func (_BondingManager *BondingManagerCaller) PendingStake(opts *bind.CallOpts, _delegator common.Address, _endRound *big.Int) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _BondingManager.contract.Call(opts, out, "pendingStake", _delegator, _endRound)
	return *ret0, err
}

// PendingStake is a free data retrieval call binding the contract method 0x9d0b2c7a.
//
// Solidity: function pendingStake(_delegator address, _endRound uint256) constant returns(uint256)
func (_BondingManager *BondingManagerSession) PendingStake(_delegator common.Address, _endRound *big.Int) (*big.Int, error) {
	return _BondingManager.Contract.PendingStake(&_BondingManager.CallOpts, _delegator, _endRound)
}

// PendingStake is a free data retrieval call binding the contract method 0x9d0b2c7a.
//
// Solidity: function pendingStake(_delegator address, _endRound uint256) constant returns(uint256)
func (_BondingManager *BondingManagerCallerSession) PendingStake(_delegator common.Address, _endRound *big.Int) (*big.Int, error) {
	return _BondingManager.Contract.PendingStake(&_BondingManager.CallOpts, _delegator, _endRound)
}

// TargetContractId is a free data retrieval call binding the contract method 0x51720b41.
//
// Solidity: function targetContractId() constant returns(bytes32)
func (_BondingManager *BondingManagerCaller) TargetContractId(opts *bind.CallOpts) ([32]byte, error) {
	var (
		ret0 = new([32]byte)
	)
	out := ret0
	err := _BondingManager.contract.Call(opts, out, "targetContractId")
	return *ret0, err
}

// TargetContractId is a free data retrieval call binding the contract method 0x51720b41.
//
// Solidity: function targetContractId() constant returns(bytes32)
func (_BondingManager *BondingManagerSession) TargetContractId() ([32]byte, error) {
	return _BondingManager.Contract.TargetContractId(&_BondingManager.CallOpts)
}

// TargetContractId is a free data retrieval call binding the contract method 0x51720b41.
//
// Solidity: function targetContractId() constant returns(bytes32)
func (_BondingManager *BondingManagerCallerSession) TargetContractId() ([32]byte, error) {
	return _BondingManager.Contract.TargetContractId(&_BondingManager.CallOpts)
}

// TranscoderStatus is a free data retrieval call binding the contract method 0x8b2f1652.
//
// Solidity: function transcoderStatus(_transcoder address) constant returns(uint8)
func (_BondingManager *BondingManagerCaller) TranscoderStatus(opts *bind.CallOpts, _transcoder common.Address) (uint8, error) {
	var (
		ret0 = new(uint8)
	)
	out := ret0
	err := _BondingManager.contract.Call(opts, out, "transcoderStatus", _transcoder)
	return *ret0, err
}

// TranscoderStatus is a free data retrieval call binding the contract method 0x8b2f1652.
//
// Solidity: function transcoderStatus(_transcoder address) constant returns(uint8)
func (_BondingManager *BondingManagerSession) TranscoderStatus(_transcoder common.Address) (uint8, error) {
	return _BondingManager.Contract.TranscoderStatus(&_BondingManager.CallOpts, _transcoder)
}

// TranscoderStatus is a free data retrieval call binding the contract method 0x8b2f1652.
//
// Solidity: function transcoderStatus(_transcoder address) constant returns(uint8)
func (_BondingManager *BondingManagerCallerSession) TranscoderStatus(_transcoder common.Address) (uint8, error) {
	return _BondingManager.Contract.TranscoderStatus(&_BondingManager.CallOpts, _transcoder)
}

// TranscoderTotalStake is a free data retrieval call binding the contract method 0x9ef9df94.
//
// Solidity: function transcoderTotalStake(_transcoder address) constant returns(uint256)
func (_BondingManager *BondingManagerCaller) TranscoderTotalStake(opts *bind.CallOpts, _transcoder common.Address) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _BondingManager.contract.Call(opts, out, "transcoderTotalStake", _transcoder)
	return *ret0, err
}

// TranscoderTotalStake is a free data retrieval call binding the contract method 0x9ef9df94.
//
// Solidity: function transcoderTotalStake(_transcoder address) constant returns(uint256)
func (_BondingManager *BondingManagerSession) TranscoderTotalStake(_transcoder common.Address) (*big.Int, error) {
	return _BondingManager.Contract.TranscoderTotalStake(&_BondingManager.CallOpts, _transcoder)
}

// TranscoderTotalStake is a free data retrieval call binding the contract method 0x9ef9df94.
//
// Solidity: function transcoderTotalStake(_transcoder address) constant returns(uint256)
func (_BondingManager *BondingManagerCallerSession) TranscoderTotalStake(_transcoder common.Address) (*big.Int, error) {
	return _BondingManager.Contract.TranscoderTotalStake(&_BondingManager.CallOpts, _transcoder)
}

// UnbondingPeriod is a free data retrieval call binding the contract method 0x6cf6d675.
//
// Solidity: function unbondingPeriod() constant returns(uint64)
func (_BondingManager *BondingManagerCaller) UnbondingPeriod(opts *bind.CallOpts) (uint64, error) {
	var (
		ret0 = new(uint64)
	)
	out := ret0
	err := _BondingManager.contract.Call(opts, out, "unbondingPeriod")
	return *ret0, err
}

// UnbondingPeriod is a free data retrieval call binding the contract method 0x6cf6d675.
//
// Solidity: function unbondingPeriod() constant returns(uint64)
func (_BondingManager *BondingManagerSession) UnbondingPeriod() (uint64, error) {
	return _BondingManager.Contract.UnbondingPeriod(&_BondingManager.CallOpts)
}

// UnbondingPeriod is a free data retrieval call binding the contract method 0x6cf6d675.
//
// Solidity: function unbondingPeriod() constant returns(uint64)
func (_BondingManager *BondingManagerCallerSession) UnbondingPeriod() (uint64, error) {
	return _BondingManager.Contract.UnbondingPeriod(&_BondingManager.CallOpts)
}

// Bond is a paid mutator transaction binding the contract method 0xb78d27dc.
//
// Solidity: function bond(_amount uint256, _to address) returns()
func (_BondingManager *BondingManagerTransactor) Bond(opts *bind.TransactOpts, _amount *big.Int, _to common.Address) (*types.Transaction, error) {
	return _BondingManager.contract.Transact(opts, "bond", _amount, _to)
}

// Bond is a paid mutator transaction binding the contract method 0xb78d27dc.
//
// Solidity: function bond(_amount uint256, _to address) returns()
func (_BondingManager *BondingManagerSession) Bond(_amount *big.Int, _to common.Address) (*types.Transaction, error) {
	return _BondingManager.Contract.Bond(&_BondingManager.TransactOpts, _amount, _to)
}

// Bond is a paid mutator transaction binding the contract method 0xb78d27dc.
//
// Solidity: function bond(_amount uint256, _to address) returns()
func (_BondingManager *BondingManagerTransactorSession) Bond(_amount *big.Int, _to common.Address) (*types.Transaction, error) {
	return _BondingManager.Contract.Bond(&_BondingManager.TransactOpts, _amount, _to)
}

// ClaimEarnings is a paid mutator transaction binding the contract method 0x24b1babf.
//
// Solidity: function claimEarnings(_endRound uint256) returns()
func (_BondingManager *BondingManagerTransactor) ClaimEarnings(opts *bind.TransactOpts, _endRound *big.Int) (*types.Transaction, error) {
	return _BondingManager.contract.Transact(opts, "claimEarnings", _endRound)
}

// ClaimEarnings is a paid mutator transaction binding the contract method 0x24b1babf.
//
// Solidity: function claimEarnings(_endRound uint256) returns()
func (_BondingManager *BondingManagerSession) ClaimEarnings(_endRound *big.Int) (*types.Transaction, error) {
	return _BondingManager.Contract.ClaimEarnings(&_BondingManager.TransactOpts, _endRound)
}

// ClaimEarnings is a paid mutator transaction binding the contract method 0x24b1babf.
//
// Solidity: function claimEarnings(_endRound uint256) returns()
func (_BondingManager *BondingManagerTransactorSession) ClaimEarnings(_endRound *big.Int) (*types.Transaction, error) {
	return _BondingManager.Contract.ClaimEarnings(&_BondingManager.TransactOpts, _endRound)
}

// Reward is a paid mutator transaction binding the contract method 0x228cb733.
//
// Solidity: function reward() returns()
func (_BondingManager *BondingManagerTransactor) Reward(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _BondingManager.contract.Transact(opts, "reward")
}

// Reward is a paid mutator transaction binding the contract method 0x228cb733.
//
// Solidity: function reward() returns()
func (_BondingManager *BondingManagerSession) Reward() (*types.Transaction, error) {
	return _BondingManager.Contract.Reward(&_BondingManager.TransactOpts)
}

// Reward is a paid mutator transaction binding the contract method 0x228cb733.
//
// Solidity: function reward() returns()
func (_BondingManager *BondingManagerTransactorSession) Reward() (*types.Transaction, error) {
	return _BondingManager.Contract.Reward(&_BondingManager.TransactOpts)
}

// SetActiveTranscoders is a paid mutator transaction binding the contract method 0x242ed69f.
//
// Solidity: function setActiveTranscoders() returns()
func (_BondingManager *BondingManagerTransactor) SetActiveTranscoders(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _BondingManager.contract.Transact(opts, "setActiveTranscoders")
}

// SetActiveTranscoders is a paid mutator transaction binding the contract method 0x242ed69f.
//
// Solidity: function setActiveTranscoders() returns()
func (_BondingManager *BondingManagerSession) SetActiveTranscoders() (*types.Transaction, error) {
	return _BondingManager.Contract.SetActiveTranscoders(&_BondingManager.TransactOpts)
}

// SetActiveTranscoders is a paid mutator transaction binding the contract method 0x242ed69f.
//
// Solidity: function setActiveTranscoders() returns()
func (_BondingManager *BondingManagerTransactorSession) SetActiveTranscoders() (*types.Transaction, error) {
	return _BondingManager.Contract.SetActiveTranscoders(&_BondingManager.TransactOpts)
}

// SetController is a paid mutator transaction binding the contract method 0x92eefe9b.
//
// Solidity: function setController(_controller address) returns()
func (_BondingManager *BondingManagerTransactor) SetController(opts *bind.TransactOpts, _controller common.Address) (*types.Transaction, error) {
	return _BondingManager.contract.Transact(opts, "setController", _controller)
}

// SetController is a paid mutator transaction binding the contract method 0x92eefe9b.
//
// Solidity: function setController(_controller address) returns()
func (_BondingManager *BondingManagerSession) SetController(_controller common.Address) (*types.Transaction, error) {
	return _BondingManager.Contract.SetController(&_BondingManager.TransactOpts, _controller)
}

// SetController is a paid mutator transaction binding the contract method 0x92eefe9b.
//
// Solidity: function setController(_controller address) returns()
func (_BondingManager *BondingManagerTransactorSession) SetController(_controller common.Address) (*types.Transaction, error) {
	return _BondingManager.Contract.SetController(&_BondingManager.TransactOpts, _controller)
}

// SetMaxEarningsClaimsRounds is a paid mutator transaction binding the contract method 0x72d9f13d.
//
// Solidity: function setMaxEarningsClaimsRounds(_maxEarningsClaimsRounds uint256) returns()
func (_BondingManager *BondingManagerTransactor) SetMaxEarningsClaimsRounds(opts *bind.TransactOpts, _maxEarningsClaimsRounds *big.Int) (*types.Transaction, error) {
	return _BondingManager.contract.Transact(opts, "setMaxEarningsClaimsRounds", _maxEarningsClaimsRounds)
}

// SetMaxEarningsClaimsRounds is a paid mutator transaction binding the contract method 0x72d9f13d.
//
// Solidity: function setMaxEarningsClaimsRounds(_maxEarningsClaimsRounds uint256) returns()
func (_BondingManager *BondingManagerSession) SetMaxEarningsClaimsRounds(_maxEarningsClaimsRounds *big.Int) (*types.Transaction, error) {
	return _BondingManager.Contract.SetMaxEarningsClaimsRounds(&_BondingManager.TransactOpts, _maxEarningsClaimsRounds)
}

// SetMaxEarningsClaimsRounds is a paid mutator transaction binding the contract method 0x72d9f13d.
//
// Solidity: function setMaxEarningsClaimsRounds(_maxEarningsClaimsRounds uint256) returns()
func (_BondingManager *BondingManagerTransactorSession) SetMaxEarningsClaimsRounds(_maxEarningsClaimsRounds *big.Int) (*types.Transaction, error) {
	return _BondingManager.Contract.SetMaxEarningsClaimsRounds(&_BondingManager.TransactOpts, _maxEarningsClaimsRounds)
}

// SetNumActiveTranscoders is a paid mutator transaction binding the contract method 0x673a456b.
//
// Solidity: function setNumActiveTranscoders(_numActiveTranscoders uint256) returns()
func (_BondingManager *BondingManagerTransactor) SetNumActiveTranscoders(opts *bind.TransactOpts, _numActiveTranscoders *big.Int) (*types.Transaction, error) {
	return _BondingManager.contract.Transact(opts, "setNumActiveTranscoders", _numActiveTranscoders)
}

// SetNumActiveTranscoders is a paid mutator transaction binding the contract method 0x673a456b.
//
// Solidity: function setNumActiveTranscoders(_numActiveTranscoders uint256) returns()
func (_BondingManager *BondingManagerSession) SetNumActiveTranscoders(_numActiveTranscoders *big.Int) (*types.Transaction, error) {
	return _BondingManager.Contract.SetNumActiveTranscoders(&_BondingManager.TransactOpts, _numActiveTranscoders)
}

// SetNumActiveTranscoders is a paid mutator transaction binding the contract method 0x673a456b.
//
// Solidity: function setNumActiveTranscoders(_numActiveTranscoders uint256) returns()
func (_BondingManager *BondingManagerTransactorSession) SetNumActiveTranscoders(_numActiveTranscoders *big.Int) (*types.Transaction, error) {
	return _BondingManager.Contract.SetNumActiveTranscoders(&_BondingManager.TransactOpts, _numActiveTranscoders)
}

// SetNumTranscoders is a paid mutator transaction binding the contract method 0x60c79d00.
//
// Solidity: function setNumTranscoders(_numTranscoders uint256) returns()
func (_BondingManager *BondingManagerTransactor) SetNumTranscoders(opts *bind.TransactOpts, _numTranscoders *big.Int) (*types.Transaction, error) {
	return _BondingManager.contract.Transact(opts, "setNumTranscoders", _numTranscoders)
}

// SetNumTranscoders is a paid mutator transaction binding the contract method 0x60c79d00.
//
// Solidity: function setNumTranscoders(_numTranscoders uint256) returns()
func (_BondingManager *BondingManagerSession) SetNumTranscoders(_numTranscoders *big.Int) (*types.Transaction, error) {
	return _BondingManager.Contract.SetNumTranscoders(&_BondingManager.TransactOpts, _numTranscoders)
}

// SetNumTranscoders is a paid mutator transaction binding the contract method 0x60c79d00.
//
// Solidity: function setNumTranscoders(_numTranscoders uint256) returns()
func (_BondingManager *BondingManagerTransactorSession) SetNumTranscoders(_numTranscoders *big.Int) (*types.Transaction, error) {
	return _BondingManager.Contract.SetNumTranscoders(&_BondingManager.TransactOpts, _numTranscoders)
}

// SetUnbondingPeriod is a paid mutator transaction binding the contract method 0xf10d1de1.
//
// Solidity: function setUnbondingPeriod(_unbondingPeriod uint64) returns()
func (_BondingManager *BondingManagerTransactor) SetUnbondingPeriod(opts *bind.TransactOpts, _unbondingPeriod uint64) (*types.Transaction, error) {
	return _BondingManager.contract.Transact(opts, "setUnbondingPeriod", _unbondingPeriod)
}

// SetUnbondingPeriod is a paid mutator transaction binding the contract method 0xf10d1de1.
//
// Solidity: function setUnbondingPeriod(_unbondingPeriod uint64) returns()
func (_BondingManager *BondingManagerSession) SetUnbondingPeriod(_unbondingPeriod uint64) (*types.Transaction, error) {
	return _BondingManager.Contract.SetUnbondingPeriod(&_BondingManager.TransactOpts, _unbondingPeriod)
}

// SetUnbondingPeriod is a paid mutator transaction binding the contract method 0xf10d1de1.
//
// Solidity: function setUnbondingPeriod(_unbondingPeriod uint64) returns()
func (_BondingManager *BondingManagerTransactorSession) SetUnbondingPeriod(_unbondingPeriod uint64) (*types.Transaction, error) {
	return _BondingManager.Contract.SetUnbondingPeriod(&_BondingManager.TransactOpts, _unbondingPeriod)
}

// SlashTranscoder is a paid mutator transaction binding the contract method 0x22bf9d7c.
//
// Solidity: function slashTranscoder(_transcoder address, _finder address, _slashAmount uint256, _finderFee uint256) returns()
func (_BondingManager *BondingManagerTransactor) SlashTranscoder(opts *bind.TransactOpts, _transcoder common.Address, _finder common.Address, _slashAmount *big.Int, _finderFee *big.Int) (*types.Transaction, error) {
	return _BondingManager.contract.Transact(opts, "slashTranscoder", _transcoder, _finder, _slashAmount, _finderFee)
}

// SlashTranscoder is a paid mutator transaction binding the contract method 0x22bf9d7c.
//
// Solidity: function slashTranscoder(_transcoder address, _finder address, _slashAmount uint256, _finderFee uint256) returns()
func (_BondingManager *BondingManagerSession) SlashTranscoder(_transcoder common.Address, _finder common.Address, _slashAmount *big.Int, _finderFee *big.Int) (*types.Transaction, error) {
	return _BondingManager.Contract.SlashTranscoder(&_BondingManager.TransactOpts, _transcoder, _finder, _slashAmount, _finderFee)
}

// SlashTranscoder is a paid mutator transaction binding the contract method 0x22bf9d7c.
//
// Solidity: function slashTranscoder(_transcoder address, _finder address, _slashAmount uint256, _finderFee uint256) returns()
func (_BondingManager *BondingManagerTransactorSession) SlashTranscoder(_transcoder common.Address, _finder common.Address, _slashAmount *big.Int, _finderFee *big.Int) (*types.Transaction, error) {
	return _BondingManager.Contract.SlashTranscoder(&_BondingManager.TransactOpts, _transcoder, _finder, _slashAmount, _finderFee)
}

// Transcoder is a paid mutator transaction binding the contract method 0x85aaff62.
//
// Solidity: function transcoder(_rewardCut uint256, _feeShare uint256, _pricePerSegment uint256) returns()
func (_BondingManager *BondingManagerTransactor) Transcoder(opts *bind.TransactOpts, _rewardCut *big.Int, _feeShare *big.Int, _pricePerSegment *big.Int) (*types.Transaction, error) {
	return _BondingManager.contract.Transact(opts, "transcoder", _rewardCut, _feeShare, _pricePerSegment)
}

// Transcoder is a paid mutator transaction binding the contract method 0x85aaff62.
//
// Solidity: function transcoder(_rewardCut uint256, _feeShare uint256, _pricePerSegment uint256) returns()
func (_BondingManager *BondingManagerSession) Transcoder(_rewardCut *big.Int, _feeShare *big.Int, _pricePerSegment *big.Int) (*types.Transaction, error) {
	return _BondingManager.Contract.Transcoder(&_BondingManager.TransactOpts, _rewardCut, _feeShare, _pricePerSegment)
}

// Transcoder is a paid mutator transaction binding the contract method 0x85aaff62.
//
// Solidity: function transcoder(_rewardCut uint256, _feeShare uint256, _pricePerSegment uint256) returns()
func (_BondingManager *BondingManagerTransactorSession) Transcoder(_rewardCut *big.Int, _feeShare *big.Int, _pricePerSegment *big.Int) (*types.Transaction, error) {
	return _BondingManager.Contract.Transcoder(&_BondingManager.TransactOpts, _rewardCut, _feeShare, _pricePerSegment)
}

// Unbond is a paid mutator transaction binding the contract method 0x5df6a6bc.
//
// Solidity: function unbond() returns()
func (_BondingManager *BondingManagerTransactor) Unbond(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _BondingManager.contract.Transact(opts, "unbond")
}

// Unbond is a paid mutator transaction binding the contract method 0x5df6a6bc.
//
// Solidity: function unbond() returns()
func (_BondingManager *BondingManagerSession) Unbond() (*types.Transaction, error) {
	return _BondingManager.Contract.Unbond(&_BondingManager.TransactOpts)
}

// Unbond is a paid mutator transaction binding the contract method 0x5df6a6bc.
//
// Solidity: function unbond() returns()
func (_BondingManager *BondingManagerTransactorSession) Unbond() (*types.Transaction, error) {
	return _BondingManager.Contract.Unbond(&_BondingManager.TransactOpts)
}

// UpdateTranscoderWithFees is a paid mutator transaction binding the contract method 0x3aeb512c.
//
// Solidity: function updateTranscoderWithFees(_transcoder address, _fees uint256, _round uint256) returns()
func (_BondingManager *BondingManagerTransactor) UpdateTranscoderWithFees(opts *bind.TransactOpts, _transcoder common.Address, _fees *big.Int, _round *big.Int) (*types.Transaction, error) {
	return _BondingManager.contract.Transact(opts, "updateTranscoderWithFees", _transcoder, _fees, _round)
}

// UpdateTranscoderWithFees is a paid mutator transaction binding the contract method 0x3aeb512c.
//
// Solidity: function updateTranscoderWithFees(_transcoder address, _fees uint256, _round uint256) returns()
func (_BondingManager *BondingManagerSession) UpdateTranscoderWithFees(_transcoder common.Address, _fees *big.Int, _round *big.Int) (*types.Transaction, error) {
	return _BondingManager.Contract.UpdateTranscoderWithFees(&_BondingManager.TransactOpts, _transcoder, _fees, _round)
}

// UpdateTranscoderWithFees is a paid mutator transaction binding the contract method 0x3aeb512c.
//
// Solidity: function updateTranscoderWithFees(_transcoder address, _fees uint256, _round uint256) returns()
func (_BondingManager *BondingManagerTransactorSession) UpdateTranscoderWithFees(_transcoder common.Address, _fees *big.Int, _round *big.Int) (*types.Transaction, error) {
	return _BondingManager.Contract.UpdateTranscoderWithFees(&_BondingManager.TransactOpts, _transcoder, _fees, _round)
}

// WithdrawFees is a paid mutator transaction binding the contract method 0x476343ee.
//
// Solidity: function withdrawFees() returns()
func (_BondingManager *BondingManagerTransactor) WithdrawFees(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _BondingManager.contract.Transact(opts, "withdrawFees")
}

// WithdrawFees is a paid mutator transaction binding the contract method 0x476343ee.
//
// Solidity: function withdrawFees() returns()
func (_BondingManager *BondingManagerSession) WithdrawFees() (*types.Transaction, error) {
	return _BondingManager.Contract.WithdrawFees(&_BondingManager.TransactOpts)
}

// WithdrawFees is a paid mutator transaction binding the contract method 0x476343ee.
//
// Solidity: function withdrawFees() returns()
func (_BondingManager *BondingManagerTransactorSession) WithdrawFees() (*types.Transaction, error) {
	return _BondingManager.Contract.WithdrawFees(&_BondingManager.TransactOpts)
}

// WithdrawStake is a paid mutator transaction binding the contract method 0xbed9d861.
//
// Solidity: function withdrawStake() returns()
func (_BondingManager *BondingManagerTransactor) WithdrawStake(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _BondingManager.contract.Transact(opts, "withdrawStake")
}

// WithdrawStake is a paid mutator transaction binding the contract method 0xbed9d861.
//
// Solidity: function withdrawStake() returns()
func (_BondingManager *BondingManagerSession) WithdrawStake() (*types.Transaction, error) {
	return _BondingManager.Contract.WithdrawStake(&_BondingManager.TransactOpts)
}

// WithdrawStake is a paid mutator transaction binding the contract method 0xbed9d861.
//
// Solidity: function withdrawStake() returns()
func (_BondingManager *BondingManagerTransactorSession) WithdrawStake() (*types.Transaction, error) {
	return _BondingManager.Contract.WithdrawStake(&_BondingManager.TransactOpts)
}
