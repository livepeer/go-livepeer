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
const BondingManagerABI = "[{\"constant\":true,\"inputs\":[{\"name\":\"_transcoder\",\"type\":\"address\"}],\"name\":\"activeTranscoderTotalStake\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"\",\"type\":\"address\"}],\"name\":\"isActiveTranscoder\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"_delegator\",\"type\":\"address\"}],\"name\":\"delegatorStake\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"_transcoder\",\"type\":\"address\"}],\"name\":\"getTranscoderDelegatorWithdrawRound\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"_position\",\"type\":\"uint256\"}],\"name\":\"getCandidateTranscoderAtPosition\",\"outputs\":[{\"name\":\"\",\"type\":\"address\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"_maxPricePerSegment\",\"type\":\"uint256\"}],\"name\":\"electActiveTranscoder\",\"outputs\":[{\"name\":\"\",\"type\":\"address\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"_delegator\",\"type\":\"address\"}],\"name\":\"delegatorStatus\",\"outputs\":[{\"name\":\"\",\"type\":\"uint8\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":false,\"inputs\":[],\"name\":\"reward\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":false,\"inputs\":[],\"name\":\"setActiveTranscoders\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_unbondingPeriod\",\"type\":\"uint64\"},{\"name\":\"_numActiveTranscoders\",\"type\":\"uint256\"}],\"name\":\"initialize\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"getCandidatePoolSize\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":false,\"inputs\":[],\"name\":\"resignAsTranscoder\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"_transcoder\",\"type\":\"address\"}],\"name\":\"getTranscoderFeeShare\",\"outputs\":[{\"name\":\"\",\"type\":\"uint8\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"_delegator\",\"type\":\"address\"}],\"name\":\"getDelegatorStartRound\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"_transcoder\",\"type\":\"address\"}],\"name\":\"getTranscoderBlockRewardCut\",\"outputs\":[{\"name\":\"\",\"type\":\"uint8\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"isInitialized\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"_position\",\"type\":\"uint256\"}],\"name\":\"getReserveTranscoderAtPosition\",\"outputs\":[{\"name\":\"\",\"type\":\"address\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":false,\"inputs\":[],\"name\":\"withdraw\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"_transcoder\",\"type\":\"address\"}],\"name\":\"getTranscoderLastRewardRound\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"targetContractId\",\"outputs\":[{\"name\":\"\",\"type\":\"bytes32\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"_delegator\",\"type\":\"address\"}],\"name\":\"getDelegatorDelegateBlock\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":false,\"inputs\":[],\"name\":\"unbond\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"unbondingPeriod\",\"outputs\":[{\"name\":\"\",\"type\":\"uint64\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"_delegator\",\"type\":\"address\"}],\"name\":\"getDelegatorLastStakeUpdateRound\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"getReservePoolSize\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"_delegator\",\"type\":\"address\"}],\"name\":\"getDelegatorDelegatedAmount\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"_transcoder\",\"type\":\"address\"}],\"name\":\"transcoderStatus\",\"outputs\":[{\"name\":\"\",\"type\":\"uint8\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"_delegator\",\"type\":\"address\"}],\"name\":\"getDelegatorWithdrawRound\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"totalActiveTranscoderStake\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_controller\",\"type\":\"address\"}],\"name\":\"setController\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"_delegator\",\"type\":\"address\"}],\"name\":\"getDelegatorDelegateAddress\",\"outputs\":[{\"name\":\"\",\"type\":\"address\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"_transcoder\",\"type\":\"address\"}],\"name\":\"getTranscoderPendingFeeShare\",\"outputs\":[{\"name\":\"\",\"type\":\"uint8\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"_transcoder\",\"type\":\"address\"}],\"name\":\"transcoderTotalStake\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_amount\",\"type\":\"uint256\"},{\"name\":\"_to\",\"type\":\"address\"}],\"name\":\"bond\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"_delegator\",\"type\":\"address\"}],\"name\":\"getDelegatorBondedAmount\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"_transcoder\",\"type\":\"address\"}],\"name\":\"getTranscoderPendingBlockRewardCut\",\"outputs\":[{\"name\":\"\",\"type\":\"uint8\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_transcoder\",\"type\":\"address\"},{\"name\":\"_fees\",\"type\":\"uint256\"},{\"name\":\"_claimBlock\",\"type\":\"uint256\"},{\"name\":\"_transcoderTotalStake\",\"type\":\"uint256\"}],\"name\":\"updateTranscoderFeePool\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_blockRewardCut\",\"type\":\"uint8\"},{\"name\":\"_feeShare\",\"type\":\"uint8\"},{\"name\":\"_pricePerSegment\",\"type\":\"uint256\"}],\"name\":\"transcoder\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"_transcoder\",\"type\":\"address\"}],\"name\":\"getTranscoderPricePerSegment\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"_transcoder\",\"type\":\"address\"}],\"name\":\"getTranscoderPendingPricePerSegment\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"\",\"type\":\"address\"}],\"name\":\"activeTranscoderPositions\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"controller\",\"outputs\":[{\"name\":\"\",\"type\":\"address\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_transcoder\",\"type\":\"address\"},{\"name\":\"_finder\",\"type\":\"address\"},{\"name\":\"_slashAmount\",\"type\":\"uint64\"},{\"name\":\"_finderFee\",\"type\":\"uint64\"}],\"name\":\"slashTranscoder\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"type\":\"function\"},{\"inputs\":[{\"name\":\"_controller\",\"type\":\"address\"}],\"payable\":false,\"type\":\"constructor\"}]"

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

// ActiveTranscoderPositions is a free data retrieval call binding the contract method 0xf56044ed.
//
// Solidity: function activeTranscoderPositions( address) constant returns(uint256)
func (_BondingManager *BondingManagerCaller) ActiveTranscoderPositions(opts *bind.CallOpts, arg0 common.Address) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _BondingManager.contract.Call(opts, out, "activeTranscoderPositions", arg0)
	return *ret0, err
}

// ActiveTranscoderPositions is a free data retrieval call binding the contract method 0xf56044ed.
//
// Solidity: function activeTranscoderPositions( address) constant returns(uint256)
func (_BondingManager *BondingManagerSession) ActiveTranscoderPositions(arg0 common.Address) (*big.Int, error) {
	return _BondingManager.Contract.ActiveTranscoderPositions(&_BondingManager.CallOpts, arg0)
}

// ActiveTranscoderPositions is a free data retrieval call binding the contract method 0xf56044ed.
//
// Solidity: function activeTranscoderPositions( address) constant returns(uint256)
func (_BondingManager *BondingManagerCallerSession) ActiveTranscoderPositions(arg0 common.Address) (*big.Int, error) {
	return _BondingManager.Contract.ActiveTranscoderPositions(&_BondingManager.CallOpts, arg0)
}

// ActiveTranscoderTotalStake is a free data retrieval call binding the contract method 0x00944f32.
//
// Solidity: function activeTranscoderTotalStake(_transcoder address) constant returns(uint256)
func (_BondingManager *BondingManagerCaller) ActiveTranscoderTotalStake(opts *bind.CallOpts, _transcoder common.Address) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _BondingManager.contract.Call(opts, out, "activeTranscoderTotalStake", _transcoder)
	return *ret0, err
}

// ActiveTranscoderTotalStake is a free data retrieval call binding the contract method 0x00944f32.
//
// Solidity: function activeTranscoderTotalStake(_transcoder address) constant returns(uint256)
func (_BondingManager *BondingManagerSession) ActiveTranscoderTotalStake(_transcoder common.Address) (*big.Int, error) {
	return _BondingManager.Contract.ActiveTranscoderTotalStake(&_BondingManager.CallOpts, _transcoder)
}

// ActiveTranscoderTotalStake is a free data retrieval call binding the contract method 0x00944f32.
//
// Solidity: function activeTranscoderTotalStake(_transcoder address) constant returns(uint256)
func (_BondingManager *BondingManagerCallerSession) ActiveTranscoderTotalStake(_transcoder common.Address) (*big.Int, error) {
	return _BondingManager.Contract.ActiveTranscoderTotalStake(&_BondingManager.CallOpts, _transcoder)
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

// DelegatorStake is a free data retrieval call binding the contract method 0x0906a8ce.
//
// Solidity: function delegatorStake(_delegator address) constant returns(uint256)
func (_BondingManager *BondingManagerCaller) DelegatorStake(opts *bind.CallOpts, _delegator common.Address) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _BondingManager.contract.Call(opts, out, "delegatorStake", _delegator)
	return *ret0, err
}

// DelegatorStake is a free data retrieval call binding the contract method 0x0906a8ce.
//
// Solidity: function delegatorStake(_delegator address) constant returns(uint256)
func (_BondingManager *BondingManagerSession) DelegatorStake(_delegator common.Address) (*big.Int, error) {
	return _BondingManager.Contract.DelegatorStake(&_BondingManager.CallOpts, _delegator)
}

// DelegatorStake is a free data retrieval call binding the contract method 0x0906a8ce.
//
// Solidity: function delegatorStake(_delegator address) constant returns(uint256)
func (_BondingManager *BondingManagerCallerSession) DelegatorStake(_delegator common.Address) (*big.Int, error) {
	return _BondingManager.Contract.DelegatorStake(&_BondingManager.CallOpts, _delegator)
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

// ElectActiveTranscoder is a free data retrieval call binding the contract method 0x105d772f.
//
// Solidity: function electActiveTranscoder(_maxPricePerSegment uint256) constant returns(address)
func (_BondingManager *BondingManagerCaller) ElectActiveTranscoder(opts *bind.CallOpts, _maxPricePerSegment *big.Int) (common.Address, error) {
	var (
		ret0 = new(common.Address)
	)
	out := ret0
	err := _BondingManager.contract.Call(opts, out, "electActiveTranscoder", _maxPricePerSegment)
	return *ret0, err
}

// ElectActiveTranscoder is a free data retrieval call binding the contract method 0x105d772f.
//
// Solidity: function electActiveTranscoder(_maxPricePerSegment uint256) constant returns(address)
func (_BondingManager *BondingManagerSession) ElectActiveTranscoder(_maxPricePerSegment *big.Int) (common.Address, error) {
	return _BondingManager.Contract.ElectActiveTranscoder(&_BondingManager.CallOpts, _maxPricePerSegment)
}

// ElectActiveTranscoder is a free data retrieval call binding the contract method 0x105d772f.
//
// Solidity: function electActiveTranscoder(_maxPricePerSegment uint256) constant returns(address)
func (_BondingManager *BondingManagerCallerSession) ElectActiveTranscoder(_maxPricePerSegment *big.Int) (common.Address, error) {
	return _BondingManager.Contract.ElectActiveTranscoder(&_BondingManager.CallOpts, _maxPricePerSegment)
}

// GetCandidatePoolSize is a free data retrieval call binding the contract method 0x2a64a8fc.
//
// Solidity: function getCandidatePoolSize() constant returns(uint256)
func (_BondingManager *BondingManagerCaller) GetCandidatePoolSize(opts *bind.CallOpts) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _BondingManager.contract.Call(opts, out, "getCandidatePoolSize")
	return *ret0, err
}

// GetCandidatePoolSize is a free data retrieval call binding the contract method 0x2a64a8fc.
//
// Solidity: function getCandidatePoolSize() constant returns(uint256)
func (_BondingManager *BondingManagerSession) GetCandidatePoolSize() (*big.Int, error) {
	return _BondingManager.Contract.GetCandidatePoolSize(&_BondingManager.CallOpts)
}

// GetCandidatePoolSize is a free data retrieval call binding the contract method 0x2a64a8fc.
//
// Solidity: function getCandidatePoolSize() constant returns(uint256)
func (_BondingManager *BondingManagerCallerSession) GetCandidatePoolSize() (*big.Int, error) {
	return _BondingManager.Contract.GetCandidatePoolSize(&_BondingManager.CallOpts)
}

// GetCandidateTranscoderAtPosition is a free data retrieval call binding the contract method 0x0feb78d1.
//
// Solidity: function getCandidateTranscoderAtPosition(_position uint256) constant returns(address)
func (_BondingManager *BondingManagerCaller) GetCandidateTranscoderAtPosition(opts *bind.CallOpts, _position *big.Int) (common.Address, error) {
	var (
		ret0 = new(common.Address)
	)
	out := ret0
	err := _BondingManager.contract.Call(opts, out, "getCandidateTranscoderAtPosition", _position)
	return *ret0, err
}

// GetCandidateTranscoderAtPosition is a free data retrieval call binding the contract method 0x0feb78d1.
//
// Solidity: function getCandidateTranscoderAtPosition(_position uint256) constant returns(address)
func (_BondingManager *BondingManagerSession) GetCandidateTranscoderAtPosition(_position *big.Int) (common.Address, error) {
	return _BondingManager.Contract.GetCandidateTranscoderAtPosition(&_BondingManager.CallOpts, _position)
}

// GetCandidateTranscoderAtPosition is a free data retrieval call binding the contract method 0x0feb78d1.
//
// Solidity: function getCandidateTranscoderAtPosition(_position uint256) constant returns(address)
func (_BondingManager *BondingManagerCallerSession) GetCandidateTranscoderAtPosition(_position *big.Int) (common.Address, error) {
	return _BondingManager.Contract.GetCandidateTranscoderAtPosition(&_BondingManager.CallOpts, _position)
}

// GetDelegatorBondedAmount is a free data retrieval call binding the contract method 0xb8cb529d.
//
// Solidity: function getDelegatorBondedAmount(_delegator address) constant returns(uint256)
func (_BondingManager *BondingManagerCaller) GetDelegatorBondedAmount(opts *bind.CallOpts, _delegator common.Address) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _BondingManager.contract.Call(opts, out, "getDelegatorBondedAmount", _delegator)
	return *ret0, err
}

// GetDelegatorBondedAmount is a free data retrieval call binding the contract method 0xb8cb529d.
//
// Solidity: function getDelegatorBondedAmount(_delegator address) constant returns(uint256)
func (_BondingManager *BondingManagerSession) GetDelegatorBondedAmount(_delegator common.Address) (*big.Int, error) {
	return _BondingManager.Contract.GetDelegatorBondedAmount(&_BondingManager.CallOpts, _delegator)
}

// GetDelegatorBondedAmount is a free data retrieval call binding the contract method 0xb8cb529d.
//
// Solidity: function getDelegatorBondedAmount(_delegator address) constant returns(uint256)
func (_BondingManager *BondingManagerCallerSession) GetDelegatorBondedAmount(_delegator common.Address) (*big.Int, error) {
	return _BondingManager.Contract.GetDelegatorBondedAmount(&_BondingManager.CallOpts, _delegator)
}

// GetDelegatorDelegateAddress is a free data retrieval call binding the contract method 0x982e4b05.
//
// Solidity: function getDelegatorDelegateAddress(_delegator address) constant returns(address)
func (_BondingManager *BondingManagerCaller) GetDelegatorDelegateAddress(opts *bind.CallOpts, _delegator common.Address) (common.Address, error) {
	var (
		ret0 = new(common.Address)
	)
	out := ret0
	err := _BondingManager.contract.Call(opts, out, "getDelegatorDelegateAddress", _delegator)
	return *ret0, err
}

// GetDelegatorDelegateAddress is a free data retrieval call binding the contract method 0x982e4b05.
//
// Solidity: function getDelegatorDelegateAddress(_delegator address) constant returns(address)
func (_BondingManager *BondingManagerSession) GetDelegatorDelegateAddress(_delegator common.Address) (common.Address, error) {
	return _BondingManager.Contract.GetDelegatorDelegateAddress(&_BondingManager.CallOpts, _delegator)
}

// GetDelegatorDelegateAddress is a free data retrieval call binding the contract method 0x982e4b05.
//
// Solidity: function getDelegatorDelegateAddress(_delegator address) constant returns(address)
func (_BondingManager *BondingManagerCallerSession) GetDelegatorDelegateAddress(_delegator common.Address) (common.Address, error) {
	return _BondingManager.Contract.GetDelegatorDelegateAddress(&_BondingManager.CallOpts, _delegator)
}

// GetDelegatorDelegateBlock is a free data retrieval call binding the contract method 0x5bcbb7a8.
//
// Solidity: function getDelegatorDelegateBlock(_delegator address) constant returns(uint256)
func (_BondingManager *BondingManagerCaller) GetDelegatorDelegateBlock(opts *bind.CallOpts, _delegator common.Address) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _BondingManager.contract.Call(opts, out, "getDelegatorDelegateBlock", _delegator)
	return *ret0, err
}

// GetDelegatorDelegateBlock is a free data retrieval call binding the contract method 0x5bcbb7a8.
//
// Solidity: function getDelegatorDelegateBlock(_delegator address) constant returns(uint256)
func (_BondingManager *BondingManagerSession) GetDelegatorDelegateBlock(_delegator common.Address) (*big.Int, error) {
	return _BondingManager.Contract.GetDelegatorDelegateBlock(&_BondingManager.CallOpts, _delegator)
}

// GetDelegatorDelegateBlock is a free data retrieval call binding the contract method 0x5bcbb7a8.
//
// Solidity: function getDelegatorDelegateBlock(_delegator address) constant returns(uint256)
func (_BondingManager *BondingManagerCallerSession) GetDelegatorDelegateBlock(_delegator common.Address) (*big.Int, error) {
	return _BondingManager.Contract.GetDelegatorDelegateBlock(&_BondingManager.CallOpts, _delegator)
}

// GetDelegatorDelegatedAmount is a free data retrieval call binding the contract method 0x8679342d.
//
// Solidity: function getDelegatorDelegatedAmount(_delegator address) constant returns(uint256)
func (_BondingManager *BondingManagerCaller) GetDelegatorDelegatedAmount(opts *bind.CallOpts, _delegator common.Address) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _BondingManager.contract.Call(opts, out, "getDelegatorDelegatedAmount", _delegator)
	return *ret0, err
}

// GetDelegatorDelegatedAmount is a free data retrieval call binding the contract method 0x8679342d.
//
// Solidity: function getDelegatorDelegatedAmount(_delegator address) constant returns(uint256)
func (_BondingManager *BondingManagerSession) GetDelegatorDelegatedAmount(_delegator common.Address) (*big.Int, error) {
	return _BondingManager.Contract.GetDelegatorDelegatedAmount(&_BondingManager.CallOpts, _delegator)
}

// GetDelegatorDelegatedAmount is a free data retrieval call binding the contract method 0x8679342d.
//
// Solidity: function getDelegatorDelegatedAmount(_delegator address) constant returns(uint256)
func (_BondingManager *BondingManagerCallerSession) GetDelegatorDelegatedAmount(_delegator common.Address) (*big.Int, error) {
	return _BondingManager.Contract.GetDelegatorDelegatedAmount(&_BondingManager.CallOpts, _delegator)
}

// GetDelegatorLastStakeUpdateRound is a free data retrieval call binding the contract method 0x7350cb32.
//
// Solidity: function getDelegatorLastStakeUpdateRound(_delegator address) constant returns(uint256)
func (_BondingManager *BondingManagerCaller) GetDelegatorLastStakeUpdateRound(opts *bind.CallOpts, _delegator common.Address) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _BondingManager.contract.Call(opts, out, "getDelegatorLastStakeUpdateRound", _delegator)
	return *ret0, err
}

// GetDelegatorLastStakeUpdateRound is a free data retrieval call binding the contract method 0x7350cb32.
//
// Solidity: function getDelegatorLastStakeUpdateRound(_delegator address) constant returns(uint256)
func (_BondingManager *BondingManagerSession) GetDelegatorLastStakeUpdateRound(_delegator common.Address) (*big.Int, error) {
	return _BondingManager.Contract.GetDelegatorLastStakeUpdateRound(&_BondingManager.CallOpts, _delegator)
}

// GetDelegatorLastStakeUpdateRound is a free data retrieval call binding the contract method 0x7350cb32.
//
// Solidity: function getDelegatorLastStakeUpdateRound(_delegator address) constant returns(uint256)
func (_BondingManager *BondingManagerCallerSession) GetDelegatorLastStakeUpdateRound(_delegator common.Address) (*big.Int, error) {
	return _BondingManager.Contract.GetDelegatorLastStakeUpdateRound(&_BondingManager.CallOpts, _delegator)
}

// GetDelegatorStartRound is a free data retrieval call binding the contract method 0x358fbaed.
//
// Solidity: function getDelegatorStartRound(_delegator address) constant returns(uint256)
func (_BondingManager *BondingManagerCaller) GetDelegatorStartRound(opts *bind.CallOpts, _delegator common.Address) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _BondingManager.contract.Call(opts, out, "getDelegatorStartRound", _delegator)
	return *ret0, err
}

// GetDelegatorStartRound is a free data retrieval call binding the contract method 0x358fbaed.
//
// Solidity: function getDelegatorStartRound(_delegator address) constant returns(uint256)
func (_BondingManager *BondingManagerSession) GetDelegatorStartRound(_delegator common.Address) (*big.Int, error) {
	return _BondingManager.Contract.GetDelegatorStartRound(&_BondingManager.CallOpts, _delegator)
}

// GetDelegatorStartRound is a free data retrieval call binding the contract method 0x358fbaed.
//
// Solidity: function getDelegatorStartRound(_delegator address) constant returns(uint256)
func (_BondingManager *BondingManagerCallerSession) GetDelegatorStartRound(_delegator common.Address) (*big.Int, error) {
	return _BondingManager.Contract.GetDelegatorStartRound(&_BondingManager.CallOpts, _delegator)
}

// GetDelegatorWithdrawRound is a free data retrieval call binding the contract method 0x8d507b38.
//
// Solidity: function getDelegatorWithdrawRound(_delegator address) constant returns(uint256)
func (_BondingManager *BondingManagerCaller) GetDelegatorWithdrawRound(opts *bind.CallOpts, _delegator common.Address) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _BondingManager.contract.Call(opts, out, "getDelegatorWithdrawRound", _delegator)
	return *ret0, err
}

// GetDelegatorWithdrawRound is a free data retrieval call binding the contract method 0x8d507b38.
//
// Solidity: function getDelegatorWithdrawRound(_delegator address) constant returns(uint256)
func (_BondingManager *BondingManagerSession) GetDelegatorWithdrawRound(_delegator common.Address) (*big.Int, error) {
	return _BondingManager.Contract.GetDelegatorWithdrawRound(&_BondingManager.CallOpts, _delegator)
}

// GetDelegatorWithdrawRound is a free data retrieval call binding the contract method 0x8d507b38.
//
// Solidity: function getDelegatorWithdrawRound(_delegator address) constant returns(uint256)
func (_BondingManager *BondingManagerCallerSession) GetDelegatorWithdrawRound(_delegator common.Address) (*big.Int, error) {
	return _BondingManager.Contract.GetDelegatorWithdrawRound(&_BondingManager.CallOpts, _delegator)
}

// GetReservePoolSize is a free data retrieval call binding the contract method 0x77446bde.
//
// Solidity: function getReservePoolSize() constant returns(uint256)
func (_BondingManager *BondingManagerCaller) GetReservePoolSize(opts *bind.CallOpts) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _BondingManager.contract.Call(opts, out, "getReservePoolSize")
	return *ret0, err
}

// GetReservePoolSize is a free data retrieval call binding the contract method 0x77446bde.
//
// Solidity: function getReservePoolSize() constant returns(uint256)
func (_BondingManager *BondingManagerSession) GetReservePoolSize() (*big.Int, error) {
	return _BondingManager.Contract.GetReservePoolSize(&_BondingManager.CallOpts)
}

// GetReservePoolSize is a free data retrieval call binding the contract method 0x77446bde.
//
// Solidity: function getReservePoolSize() constant returns(uint256)
func (_BondingManager *BondingManagerCallerSession) GetReservePoolSize() (*big.Int, error) {
	return _BondingManager.Contract.GetReservePoolSize(&_BondingManager.CallOpts)
}

// GetReserveTranscoderAtPosition is a free data retrieval call binding the contract method 0x3a80e283.
//
// Solidity: function getReserveTranscoderAtPosition(_position uint256) constant returns(address)
func (_BondingManager *BondingManagerCaller) GetReserveTranscoderAtPosition(opts *bind.CallOpts, _position *big.Int) (common.Address, error) {
	var (
		ret0 = new(common.Address)
	)
	out := ret0
	err := _BondingManager.contract.Call(opts, out, "getReserveTranscoderAtPosition", _position)
	return *ret0, err
}

// GetReserveTranscoderAtPosition is a free data retrieval call binding the contract method 0x3a80e283.
//
// Solidity: function getReserveTranscoderAtPosition(_position uint256) constant returns(address)
func (_BondingManager *BondingManagerSession) GetReserveTranscoderAtPosition(_position *big.Int) (common.Address, error) {
	return _BondingManager.Contract.GetReserveTranscoderAtPosition(&_BondingManager.CallOpts, _position)
}

// GetReserveTranscoderAtPosition is a free data retrieval call binding the contract method 0x3a80e283.
//
// Solidity: function getReserveTranscoderAtPosition(_position uint256) constant returns(address)
func (_BondingManager *BondingManagerCallerSession) GetReserveTranscoderAtPosition(_position *big.Int) (common.Address, error) {
	return _BondingManager.Contract.GetReserveTranscoderAtPosition(&_BondingManager.CallOpts, _position)
}

// GetTranscoderBlockRewardCut is a free data retrieval call binding the contract method 0x35f4d5b3.
//
// Solidity: function getTranscoderBlockRewardCut(_transcoder address) constant returns(uint8)
func (_BondingManager *BondingManagerCaller) GetTranscoderBlockRewardCut(opts *bind.CallOpts, _transcoder common.Address) (uint8, error) {
	var (
		ret0 = new(uint8)
	)
	out := ret0
	err := _BondingManager.contract.Call(opts, out, "getTranscoderBlockRewardCut", _transcoder)
	return *ret0, err
}

// GetTranscoderBlockRewardCut is a free data retrieval call binding the contract method 0x35f4d5b3.
//
// Solidity: function getTranscoderBlockRewardCut(_transcoder address) constant returns(uint8)
func (_BondingManager *BondingManagerSession) GetTranscoderBlockRewardCut(_transcoder common.Address) (uint8, error) {
	return _BondingManager.Contract.GetTranscoderBlockRewardCut(&_BondingManager.CallOpts, _transcoder)
}

// GetTranscoderBlockRewardCut is a free data retrieval call binding the contract method 0x35f4d5b3.
//
// Solidity: function getTranscoderBlockRewardCut(_transcoder address) constant returns(uint8)
func (_BondingManager *BondingManagerCallerSession) GetTranscoderBlockRewardCut(_transcoder common.Address) (uint8, error) {
	return _BondingManager.Contract.GetTranscoderBlockRewardCut(&_BondingManager.CallOpts, _transcoder)
}

// GetTranscoderDelegatorWithdrawRound is a free data retrieval call binding the contract method 0x0b8d4b8e.
//
// Solidity: function getTranscoderDelegatorWithdrawRound(_transcoder address) constant returns(uint256)
func (_BondingManager *BondingManagerCaller) GetTranscoderDelegatorWithdrawRound(opts *bind.CallOpts, _transcoder common.Address) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _BondingManager.contract.Call(opts, out, "getTranscoderDelegatorWithdrawRound", _transcoder)
	return *ret0, err
}

// GetTranscoderDelegatorWithdrawRound is a free data retrieval call binding the contract method 0x0b8d4b8e.
//
// Solidity: function getTranscoderDelegatorWithdrawRound(_transcoder address) constant returns(uint256)
func (_BondingManager *BondingManagerSession) GetTranscoderDelegatorWithdrawRound(_transcoder common.Address) (*big.Int, error) {
	return _BondingManager.Contract.GetTranscoderDelegatorWithdrawRound(&_BondingManager.CallOpts, _transcoder)
}

// GetTranscoderDelegatorWithdrawRound is a free data retrieval call binding the contract method 0x0b8d4b8e.
//
// Solidity: function getTranscoderDelegatorWithdrawRound(_transcoder address) constant returns(uint256)
func (_BondingManager *BondingManagerCallerSession) GetTranscoderDelegatorWithdrawRound(_transcoder common.Address) (*big.Int, error) {
	return _BondingManager.Contract.GetTranscoderDelegatorWithdrawRound(&_BondingManager.CallOpts, _transcoder)
}

// GetTranscoderFeeShare is a free data retrieval call binding the contract method 0x32b55608.
//
// Solidity: function getTranscoderFeeShare(_transcoder address) constant returns(uint8)
func (_BondingManager *BondingManagerCaller) GetTranscoderFeeShare(opts *bind.CallOpts, _transcoder common.Address) (uint8, error) {
	var (
		ret0 = new(uint8)
	)
	out := ret0
	err := _BondingManager.contract.Call(opts, out, "getTranscoderFeeShare", _transcoder)
	return *ret0, err
}

// GetTranscoderFeeShare is a free data retrieval call binding the contract method 0x32b55608.
//
// Solidity: function getTranscoderFeeShare(_transcoder address) constant returns(uint8)
func (_BondingManager *BondingManagerSession) GetTranscoderFeeShare(_transcoder common.Address) (uint8, error) {
	return _BondingManager.Contract.GetTranscoderFeeShare(&_BondingManager.CallOpts, _transcoder)
}

// GetTranscoderFeeShare is a free data retrieval call binding the contract method 0x32b55608.
//
// Solidity: function getTranscoderFeeShare(_transcoder address) constant returns(uint8)
func (_BondingManager *BondingManagerCallerSession) GetTranscoderFeeShare(_transcoder common.Address) (uint8, error) {
	return _BondingManager.Contract.GetTranscoderFeeShare(&_BondingManager.CallOpts, _transcoder)
}

// GetTranscoderLastRewardRound is a free data retrieval call binding the contract method 0x3f8a1f87.
//
// Solidity: function getTranscoderLastRewardRound(_transcoder address) constant returns(uint256)
func (_BondingManager *BondingManagerCaller) GetTranscoderLastRewardRound(opts *bind.CallOpts, _transcoder common.Address) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _BondingManager.contract.Call(opts, out, "getTranscoderLastRewardRound", _transcoder)
	return *ret0, err
}

// GetTranscoderLastRewardRound is a free data retrieval call binding the contract method 0x3f8a1f87.
//
// Solidity: function getTranscoderLastRewardRound(_transcoder address) constant returns(uint256)
func (_BondingManager *BondingManagerSession) GetTranscoderLastRewardRound(_transcoder common.Address) (*big.Int, error) {
	return _BondingManager.Contract.GetTranscoderLastRewardRound(&_BondingManager.CallOpts, _transcoder)
}

// GetTranscoderLastRewardRound is a free data retrieval call binding the contract method 0x3f8a1f87.
//
// Solidity: function getTranscoderLastRewardRound(_transcoder address) constant returns(uint256)
func (_BondingManager *BondingManagerCallerSession) GetTranscoderLastRewardRound(_transcoder common.Address) (*big.Int, error) {
	return _BondingManager.Contract.GetTranscoderLastRewardRound(&_BondingManager.CallOpts, _transcoder)
}

// GetTranscoderPendingBlockRewardCut is a free data retrieval call binding the contract method 0xbb2770ed.
//
// Solidity: function getTranscoderPendingBlockRewardCut(_transcoder address) constant returns(uint8)
func (_BondingManager *BondingManagerCaller) GetTranscoderPendingBlockRewardCut(opts *bind.CallOpts, _transcoder common.Address) (uint8, error) {
	var (
		ret0 = new(uint8)
	)
	out := ret0
	err := _BondingManager.contract.Call(opts, out, "getTranscoderPendingBlockRewardCut", _transcoder)
	return *ret0, err
}

// GetTranscoderPendingBlockRewardCut is a free data retrieval call binding the contract method 0xbb2770ed.
//
// Solidity: function getTranscoderPendingBlockRewardCut(_transcoder address) constant returns(uint8)
func (_BondingManager *BondingManagerSession) GetTranscoderPendingBlockRewardCut(_transcoder common.Address) (uint8, error) {
	return _BondingManager.Contract.GetTranscoderPendingBlockRewardCut(&_BondingManager.CallOpts, _transcoder)
}

// GetTranscoderPendingBlockRewardCut is a free data retrieval call binding the contract method 0xbb2770ed.
//
// Solidity: function getTranscoderPendingBlockRewardCut(_transcoder address) constant returns(uint8)
func (_BondingManager *BondingManagerCallerSession) GetTranscoderPendingBlockRewardCut(_transcoder common.Address) (uint8, error) {
	return _BondingManager.Contract.GetTranscoderPendingBlockRewardCut(&_BondingManager.CallOpts, _transcoder)
}

// GetTranscoderPendingFeeShare is a free data retrieval call binding the contract method 0x9d95d25b.
//
// Solidity: function getTranscoderPendingFeeShare(_transcoder address) constant returns(uint8)
func (_BondingManager *BondingManagerCaller) GetTranscoderPendingFeeShare(opts *bind.CallOpts, _transcoder common.Address) (uint8, error) {
	var (
		ret0 = new(uint8)
	)
	out := ret0
	err := _BondingManager.contract.Call(opts, out, "getTranscoderPendingFeeShare", _transcoder)
	return *ret0, err
}

// GetTranscoderPendingFeeShare is a free data retrieval call binding the contract method 0x9d95d25b.
//
// Solidity: function getTranscoderPendingFeeShare(_transcoder address) constant returns(uint8)
func (_BondingManager *BondingManagerSession) GetTranscoderPendingFeeShare(_transcoder common.Address) (uint8, error) {
	return _BondingManager.Contract.GetTranscoderPendingFeeShare(&_BondingManager.CallOpts, _transcoder)
}

// GetTranscoderPendingFeeShare is a free data retrieval call binding the contract method 0x9d95d25b.
//
// Solidity: function getTranscoderPendingFeeShare(_transcoder address) constant returns(uint8)
func (_BondingManager *BondingManagerCallerSession) GetTranscoderPendingFeeShare(_transcoder common.Address) (uint8, error) {
	return _BondingManager.Contract.GetTranscoderPendingFeeShare(&_BondingManager.CallOpts, _transcoder)
}

// GetTranscoderPendingPricePerSegment is a free data retrieval call binding the contract method 0xd6cc0798.
//
// Solidity: function getTranscoderPendingPricePerSegment(_transcoder address) constant returns(uint256)
func (_BondingManager *BondingManagerCaller) GetTranscoderPendingPricePerSegment(opts *bind.CallOpts, _transcoder common.Address) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _BondingManager.contract.Call(opts, out, "getTranscoderPendingPricePerSegment", _transcoder)
	return *ret0, err
}

// GetTranscoderPendingPricePerSegment is a free data retrieval call binding the contract method 0xd6cc0798.
//
// Solidity: function getTranscoderPendingPricePerSegment(_transcoder address) constant returns(uint256)
func (_BondingManager *BondingManagerSession) GetTranscoderPendingPricePerSegment(_transcoder common.Address) (*big.Int, error) {
	return _BondingManager.Contract.GetTranscoderPendingPricePerSegment(&_BondingManager.CallOpts, _transcoder)
}

// GetTranscoderPendingPricePerSegment is a free data retrieval call binding the contract method 0xd6cc0798.
//
// Solidity: function getTranscoderPendingPricePerSegment(_transcoder address) constant returns(uint256)
func (_BondingManager *BondingManagerCallerSession) GetTranscoderPendingPricePerSegment(_transcoder common.Address) (*big.Int, error) {
	return _BondingManager.Contract.GetTranscoderPendingPricePerSegment(&_BondingManager.CallOpts, _transcoder)
}

// GetTranscoderPricePerSegment is a free data retrieval call binding the contract method 0xd2ecc477.
//
// Solidity: function getTranscoderPricePerSegment(_transcoder address) constant returns(uint256)
func (_BondingManager *BondingManagerCaller) GetTranscoderPricePerSegment(opts *bind.CallOpts, _transcoder common.Address) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _BondingManager.contract.Call(opts, out, "getTranscoderPricePerSegment", _transcoder)
	return *ret0, err
}

// GetTranscoderPricePerSegment is a free data retrieval call binding the contract method 0xd2ecc477.
//
// Solidity: function getTranscoderPricePerSegment(_transcoder address) constant returns(uint256)
func (_BondingManager *BondingManagerSession) GetTranscoderPricePerSegment(_transcoder common.Address) (*big.Int, error) {
	return _BondingManager.Contract.GetTranscoderPricePerSegment(&_BondingManager.CallOpts, _transcoder)
}

// GetTranscoderPricePerSegment is a free data retrieval call binding the contract method 0xd2ecc477.
//
// Solidity: function getTranscoderPricePerSegment(_transcoder address) constant returns(uint256)
func (_BondingManager *BondingManagerCallerSession) GetTranscoderPricePerSegment(_transcoder common.Address) (*big.Int, error) {
	return _BondingManager.Contract.GetTranscoderPricePerSegment(&_BondingManager.CallOpts, _transcoder)
}

// IsActiveTranscoder is a free data retrieval call binding the contract method 0x08802374.
//
// Solidity: function isActiveTranscoder( address) constant returns(bool)
func (_BondingManager *BondingManagerCaller) IsActiveTranscoder(opts *bind.CallOpts, arg0 common.Address) (bool, error) {
	var (
		ret0 = new(bool)
	)
	out := ret0
	err := _BondingManager.contract.Call(opts, out, "isActiveTranscoder", arg0)
	return *ret0, err
}

// IsActiveTranscoder is a free data retrieval call binding the contract method 0x08802374.
//
// Solidity: function isActiveTranscoder( address) constant returns(bool)
func (_BondingManager *BondingManagerSession) IsActiveTranscoder(arg0 common.Address) (bool, error) {
	return _BondingManager.Contract.IsActiveTranscoder(&_BondingManager.CallOpts, arg0)
}

// IsActiveTranscoder is a free data retrieval call binding the contract method 0x08802374.
//
// Solidity: function isActiveTranscoder( address) constant returns(bool)
func (_BondingManager *BondingManagerCallerSession) IsActiveTranscoder(arg0 common.Address) (bool, error) {
	return _BondingManager.Contract.IsActiveTranscoder(&_BondingManager.CallOpts, arg0)
}

// IsInitialized is a free data retrieval call binding the contract method 0x392e53cd.
//
// Solidity: function isInitialized() constant returns(bool)
func (_BondingManager *BondingManagerCaller) IsInitialized(opts *bind.CallOpts) (bool, error) {
	var (
		ret0 = new(bool)
	)
	out := ret0
	err := _BondingManager.contract.Call(opts, out, "isInitialized")
	return *ret0, err
}

// IsInitialized is a free data retrieval call binding the contract method 0x392e53cd.
//
// Solidity: function isInitialized() constant returns(bool)
func (_BondingManager *BondingManagerSession) IsInitialized() (bool, error) {
	return _BondingManager.Contract.IsInitialized(&_BondingManager.CallOpts)
}

// IsInitialized is a free data retrieval call binding the contract method 0x392e53cd.
//
// Solidity: function isInitialized() constant returns(bool)
func (_BondingManager *BondingManagerCallerSession) IsInitialized() (bool, error) {
	return _BondingManager.Contract.IsInitialized(&_BondingManager.CallOpts)
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

// TotalActiveTranscoderStake is a free data retrieval call binding the contract method 0x8d985601.
//
// Solidity: function totalActiveTranscoderStake() constant returns(uint256)
func (_BondingManager *BondingManagerCaller) TotalActiveTranscoderStake(opts *bind.CallOpts) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _BondingManager.contract.Call(opts, out, "totalActiveTranscoderStake")
	return *ret0, err
}

// TotalActiveTranscoderStake is a free data retrieval call binding the contract method 0x8d985601.
//
// Solidity: function totalActiveTranscoderStake() constant returns(uint256)
func (_BondingManager *BondingManagerSession) TotalActiveTranscoderStake() (*big.Int, error) {
	return _BondingManager.Contract.TotalActiveTranscoderStake(&_BondingManager.CallOpts)
}

// TotalActiveTranscoderStake is a free data retrieval call binding the contract method 0x8d985601.
//
// Solidity: function totalActiveTranscoderStake() constant returns(uint256)
func (_BondingManager *BondingManagerCallerSession) TotalActiveTranscoderStake() (*big.Int, error) {
	return _BondingManager.Contract.TotalActiveTranscoderStake(&_BondingManager.CallOpts)
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
// Solidity: function bond(_amount uint256, _to address) returns(bool)
func (_BondingManager *BondingManagerTransactor) Bond(opts *bind.TransactOpts, _amount *big.Int, _to common.Address) (*types.Transaction, error) {
	return _BondingManager.contract.Transact(opts, "bond", _amount, _to)
}

// Bond is a paid mutator transaction binding the contract method 0xb78d27dc.
//
// Solidity: function bond(_amount uint256, _to address) returns(bool)
func (_BondingManager *BondingManagerSession) Bond(_amount *big.Int, _to common.Address) (*types.Transaction, error) {
	return _BondingManager.Contract.Bond(&_BondingManager.TransactOpts, _amount, _to)
}

// Bond is a paid mutator transaction binding the contract method 0xb78d27dc.
//
// Solidity: function bond(_amount uint256, _to address) returns(bool)
func (_BondingManager *BondingManagerTransactorSession) Bond(_amount *big.Int, _to common.Address) (*types.Transaction, error) {
	return _BondingManager.Contract.Bond(&_BondingManager.TransactOpts, _amount, _to)
}

// Initialize is a paid mutator transaction binding the contract method 0x294c865a.
//
// Solidity: function initialize(_unbondingPeriod uint64, _numActiveTranscoders uint256) returns(bool)
func (_BondingManager *BondingManagerTransactor) Initialize(opts *bind.TransactOpts, _unbondingPeriod uint64, _numActiveTranscoders *big.Int) (*types.Transaction, error) {
	return _BondingManager.contract.Transact(opts, "initialize", _unbondingPeriod, _numActiveTranscoders)
}

// Initialize is a paid mutator transaction binding the contract method 0x294c865a.
//
// Solidity: function initialize(_unbondingPeriod uint64, _numActiveTranscoders uint256) returns(bool)
func (_BondingManager *BondingManagerSession) Initialize(_unbondingPeriod uint64, _numActiveTranscoders *big.Int) (*types.Transaction, error) {
	return _BondingManager.Contract.Initialize(&_BondingManager.TransactOpts, _unbondingPeriod, _numActiveTranscoders)
}

// Initialize is a paid mutator transaction binding the contract method 0x294c865a.
//
// Solidity: function initialize(_unbondingPeriod uint64, _numActiveTranscoders uint256) returns(bool)
func (_BondingManager *BondingManagerTransactorSession) Initialize(_unbondingPeriod uint64, _numActiveTranscoders *big.Int) (*types.Transaction, error) {
	return _BondingManager.Contract.Initialize(&_BondingManager.TransactOpts, _unbondingPeriod, _numActiveTranscoders)
}

// ResignAsTranscoder is a paid mutator transaction binding the contract method 0x311f9e18.
//
// Solidity: function resignAsTranscoder() returns(bool)
func (_BondingManager *BondingManagerTransactor) ResignAsTranscoder(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _BondingManager.contract.Transact(opts, "resignAsTranscoder")
}

// ResignAsTranscoder is a paid mutator transaction binding the contract method 0x311f9e18.
//
// Solidity: function resignAsTranscoder() returns(bool)
func (_BondingManager *BondingManagerSession) ResignAsTranscoder() (*types.Transaction, error) {
	return _BondingManager.Contract.ResignAsTranscoder(&_BondingManager.TransactOpts)
}

// ResignAsTranscoder is a paid mutator transaction binding the contract method 0x311f9e18.
//
// Solidity: function resignAsTranscoder() returns(bool)
func (_BondingManager *BondingManagerTransactorSession) ResignAsTranscoder() (*types.Transaction, error) {
	return _BondingManager.Contract.ResignAsTranscoder(&_BondingManager.TransactOpts)
}

// Reward is a paid mutator transaction binding the contract method 0x228cb733.
//
// Solidity: function reward() returns(bool)
func (_BondingManager *BondingManagerTransactor) Reward(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _BondingManager.contract.Transact(opts, "reward")
}

// Reward is a paid mutator transaction binding the contract method 0x228cb733.
//
// Solidity: function reward() returns(bool)
func (_BondingManager *BondingManagerSession) Reward() (*types.Transaction, error) {
	return _BondingManager.Contract.Reward(&_BondingManager.TransactOpts)
}

// Reward is a paid mutator transaction binding the contract method 0x228cb733.
//
// Solidity: function reward() returns(bool)
func (_BondingManager *BondingManagerTransactorSession) Reward() (*types.Transaction, error) {
	return _BondingManager.Contract.Reward(&_BondingManager.TransactOpts)
}

// SetActiveTranscoders is a paid mutator transaction binding the contract method 0x242ed69f.
//
// Solidity: function setActiveTranscoders() returns(bool)
func (_BondingManager *BondingManagerTransactor) SetActiveTranscoders(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _BondingManager.contract.Transact(opts, "setActiveTranscoders")
}

// SetActiveTranscoders is a paid mutator transaction binding the contract method 0x242ed69f.
//
// Solidity: function setActiveTranscoders() returns(bool)
func (_BondingManager *BondingManagerSession) SetActiveTranscoders() (*types.Transaction, error) {
	return _BondingManager.Contract.SetActiveTranscoders(&_BondingManager.TransactOpts)
}

// SetActiveTranscoders is a paid mutator transaction binding the contract method 0x242ed69f.
//
// Solidity: function setActiveTranscoders() returns(bool)
func (_BondingManager *BondingManagerTransactorSession) SetActiveTranscoders() (*types.Transaction, error) {
	return _BondingManager.Contract.SetActiveTranscoders(&_BondingManager.TransactOpts)
}

// SetController is a paid mutator transaction binding the contract method 0x92eefe9b.
//
// Solidity: function setController(_controller address) returns(bool)
func (_BondingManager *BondingManagerTransactor) SetController(opts *bind.TransactOpts, _controller common.Address) (*types.Transaction, error) {
	return _BondingManager.contract.Transact(opts, "setController", _controller)
}

// SetController is a paid mutator transaction binding the contract method 0x92eefe9b.
//
// Solidity: function setController(_controller address) returns(bool)
func (_BondingManager *BondingManagerSession) SetController(_controller common.Address) (*types.Transaction, error) {
	return _BondingManager.Contract.SetController(&_BondingManager.TransactOpts, _controller)
}

// SetController is a paid mutator transaction binding the contract method 0x92eefe9b.
//
// Solidity: function setController(_controller address) returns(bool)
func (_BondingManager *BondingManagerTransactorSession) SetController(_controller common.Address) (*types.Transaction, error) {
	return _BondingManager.Contract.SetController(&_BondingManager.TransactOpts, _controller)
}

// SlashTranscoder is a paid mutator transaction binding the contract method 0xfa474e95.
//
// Solidity: function slashTranscoder(_transcoder address, _finder address, _slashAmount uint64, _finderFee uint64) returns(bool)
func (_BondingManager *BondingManagerTransactor) SlashTranscoder(opts *bind.TransactOpts, _transcoder common.Address, _finder common.Address, _slashAmount uint64, _finderFee uint64) (*types.Transaction, error) {
	return _BondingManager.contract.Transact(opts, "slashTranscoder", _transcoder, _finder, _slashAmount, _finderFee)
}

// SlashTranscoder is a paid mutator transaction binding the contract method 0xfa474e95.
//
// Solidity: function slashTranscoder(_transcoder address, _finder address, _slashAmount uint64, _finderFee uint64) returns(bool)
func (_BondingManager *BondingManagerSession) SlashTranscoder(_transcoder common.Address, _finder common.Address, _slashAmount uint64, _finderFee uint64) (*types.Transaction, error) {
	return _BondingManager.Contract.SlashTranscoder(&_BondingManager.TransactOpts, _transcoder, _finder, _slashAmount, _finderFee)
}

// SlashTranscoder is a paid mutator transaction binding the contract method 0xfa474e95.
//
// Solidity: function slashTranscoder(_transcoder address, _finder address, _slashAmount uint64, _finderFee uint64) returns(bool)
func (_BondingManager *BondingManagerTransactorSession) SlashTranscoder(_transcoder common.Address, _finder common.Address, _slashAmount uint64, _finderFee uint64) (*types.Transaction, error) {
	return _BondingManager.Contract.SlashTranscoder(&_BondingManager.TransactOpts, _transcoder, _finder, _slashAmount, _finderFee)
}

// Transcoder is a paid mutator transaction binding the contract method 0xca1031d1.
//
// Solidity: function transcoder(_blockRewardCut uint8, _feeShare uint8, _pricePerSegment uint256) returns(bool)
func (_BondingManager *BondingManagerTransactor) Transcoder(opts *bind.TransactOpts, _blockRewardCut uint8, _feeShare uint8, _pricePerSegment *big.Int) (*types.Transaction, error) {
	return _BondingManager.contract.Transact(opts, "transcoder", _blockRewardCut, _feeShare, _pricePerSegment)
}

// Transcoder is a paid mutator transaction binding the contract method 0xca1031d1.
//
// Solidity: function transcoder(_blockRewardCut uint8, _feeShare uint8, _pricePerSegment uint256) returns(bool)
func (_BondingManager *BondingManagerSession) Transcoder(_blockRewardCut uint8, _feeShare uint8, _pricePerSegment *big.Int) (*types.Transaction, error) {
	return _BondingManager.Contract.Transcoder(&_BondingManager.TransactOpts, _blockRewardCut, _feeShare, _pricePerSegment)
}

// Transcoder is a paid mutator transaction binding the contract method 0xca1031d1.
//
// Solidity: function transcoder(_blockRewardCut uint8, _feeShare uint8, _pricePerSegment uint256) returns(bool)
func (_BondingManager *BondingManagerTransactorSession) Transcoder(_blockRewardCut uint8, _feeShare uint8, _pricePerSegment *big.Int) (*types.Transaction, error) {
	return _BondingManager.Contract.Transcoder(&_BondingManager.TransactOpts, _blockRewardCut, _feeShare, _pricePerSegment)
}

// Unbond is a paid mutator transaction binding the contract method 0x5df6a6bc.
//
// Solidity: function unbond() returns(bool)
func (_BondingManager *BondingManagerTransactor) Unbond(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _BondingManager.contract.Transact(opts, "unbond")
}

// Unbond is a paid mutator transaction binding the contract method 0x5df6a6bc.
//
// Solidity: function unbond() returns(bool)
func (_BondingManager *BondingManagerSession) Unbond() (*types.Transaction, error) {
	return _BondingManager.Contract.Unbond(&_BondingManager.TransactOpts)
}

// Unbond is a paid mutator transaction binding the contract method 0x5df6a6bc.
//
// Solidity: function unbond() returns(bool)
func (_BondingManager *BondingManagerTransactorSession) Unbond() (*types.Transaction, error) {
	return _BondingManager.Contract.Unbond(&_BondingManager.TransactOpts)
}

// UpdateTranscoderFeePool is a paid mutator transaction binding the contract method 0xc3d20295.
//
// Solidity: function updateTranscoderFeePool(_transcoder address, _fees uint256, _claimBlock uint256, _transcoderTotalStake uint256) returns(bool)
func (_BondingManager *BondingManagerTransactor) UpdateTranscoderFeePool(opts *bind.TransactOpts, _transcoder common.Address, _fees *big.Int, _claimBlock *big.Int, _transcoderTotalStake *big.Int) (*types.Transaction, error) {
	return _BondingManager.contract.Transact(opts, "updateTranscoderFeePool", _transcoder, _fees, _claimBlock, _transcoderTotalStake)
}

// UpdateTranscoderFeePool is a paid mutator transaction binding the contract method 0xc3d20295.
//
// Solidity: function updateTranscoderFeePool(_transcoder address, _fees uint256, _claimBlock uint256, _transcoderTotalStake uint256) returns(bool)
func (_BondingManager *BondingManagerSession) UpdateTranscoderFeePool(_transcoder common.Address, _fees *big.Int, _claimBlock *big.Int, _transcoderTotalStake *big.Int) (*types.Transaction, error) {
	return _BondingManager.Contract.UpdateTranscoderFeePool(&_BondingManager.TransactOpts, _transcoder, _fees, _claimBlock, _transcoderTotalStake)
}

// UpdateTranscoderFeePool is a paid mutator transaction binding the contract method 0xc3d20295.
//
// Solidity: function updateTranscoderFeePool(_transcoder address, _fees uint256, _claimBlock uint256, _transcoderTotalStake uint256) returns(bool)
func (_BondingManager *BondingManagerTransactorSession) UpdateTranscoderFeePool(_transcoder common.Address, _fees *big.Int, _claimBlock *big.Int, _transcoderTotalStake *big.Int) (*types.Transaction, error) {
	return _BondingManager.Contract.UpdateTranscoderFeePool(&_BondingManager.TransactOpts, _transcoder, _fees, _claimBlock, _transcoderTotalStake)
}

// Withdraw is a paid mutator transaction binding the contract method 0x3ccfd60b.
//
// Solidity: function withdraw() returns(bool)
func (_BondingManager *BondingManagerTransactor) Withdraw(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _BondingManager.contract.Transact(opts, "withdraw")
}

// Withdraw is a paid mutator transaction binding the contract method 0x3ccfd60b.
//
// Solidity: function withdraw() returns(bool)
func (_BondingManager *BondingManagerSession) Withdraw() (*types.Transaction, error) {
	return _BondingManager.Contract.Withdraw(&_BondingManager.TransactOpts)
}

// Withdraw is a paid mutator transaction binding the contract method 0x3ccfd60b.
//
// Solidity: function withdraw() returns(bool)
func (_BondingManager *BondingManagerTransactorSession) Withdraw() (*types.Transaction, error) {
	return _BondingManager.Contract.Withdraw(&_BondingManager.TransactOpts)
}
