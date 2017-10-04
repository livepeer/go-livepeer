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

// RoundsManagerABI is the input ABI used to generate the binding from.
const RoundsManagerABI = "[{\"constant\":true,\"inputs\":[],\"name\":\"roundsPerYear\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"currentRoundInitialized\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"isInitialized\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"blockTime\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"targetContractId\",\"outputs\":[{\"name\":\"\",\"type\":\"bytes32\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"lastInitializedRound\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"currentRound\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"roundLength\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"currentRoundStartBlock\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_controller\",\"type\":\"address\"}],\"name\":\"setController\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":false,\"inputs\":[],\"name\":\"initializeRound\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_blockTime\",\"type\":\"uint256\"},{\"name\":\"_roundLength\",\"type\":\"uint256\"}],\"name\":\"initialize\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"controller\",\"outputs\":[{\"name\":\"\",\"type\":\"address\"}],\"payable\":false,\"type\":\"function\"},{\"inputs\":[{\"name\":\"_controller\",\"type\":\"address\"}],\"payable\":false,\"type\":\"constructor\"}]"

// RoundsManager is an auto generated Go binding around an Ethereum contract.
type RoundsManager struct {
	RoundsManagerCaller     // Read-only binding to the contract
	RoundsManagerTransactor // Write-only binding to the contract
}

// RoundsManagerCaller is an auto generated read-only Go binding around an Ethereum contract.
type RoundsManagerCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// RoundsManagerTransactor is an auto generated write-only Go binding around an Ethereum contract.
type RoundsManagerTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// RoundsManagerSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type RoundsManagerSession struct {
	Contract     *RoundsManager    // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// RoundsManagerCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type RoundsManagerCallerSession struct {
	Contract *RoundsManagerCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts        // Call options to use throughout this session
}

// RoundsManagerTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type RoundsManagerTransactorSession struct {
	Contract     *RoundsManagerTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts        // Transaction auth options to use throughout this session
}

// RoundsManagerRaw is an auto generated low-level Go binding around an Ethereum contract.
type RoundsManagerRaw struct {
	Contract *RoundsManager // Generic contract binding to access the raw methods on
}

// RoundsManagerCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type RoundsManagerCallerRaw struct {
	Contract *RoundsManagerCaller // Generic read-only contract binding to access the raw methods on
}

// RoundsManagerTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type RoundsManagerTransactorRaw struct {
	Contract *RoundsManagerTransactor // Generic write-only contract binding to access the raw methods on
}

// NewRoundsManager creates a new instance of RoundsManager, bound to a specific deployed contract.
func NewRoundsManager(address common.Address, backend bind.ContractBackend) (*RoundsManager, error) {
	contract, err := bindRoundsManager(address, backend, backend)
	if err != nil {
		return nil, err
	}
	return &RoundsManager{RoundsManagerCaller: RoundsManagerCaller{contract: contract}, RoundsManagerTransactor: RoundsManagerTransactor{contract: contract}}, nil
}

// NewRoundsManagerCaller creates a new read-only instance of RoundsManager, bound to a specific deployed contract.
func NewRoundsManagerCaller(address common.Address, caller bind.ContractCaller) (*RoundsManagerCaller, error) {
	contract, err := bindRoundsManager(address, caller, nil)
	if err != nil {
		return nil, err
	}
	return &RoundsManagerCaller{contract: contract}, nil
}

// NewRoundsManagerTransactor creates a new write-only instance of RoundsManager, bound to a specific deployed contract.
func NewRoundsManagerTransactor(address common.Address, transactor bind.ContractTransactor) (*RoundsManagerTransactor, error) {
	contract, err := bindRoundsManager(address, nil, transactor)
	if err != nil {
		return nil, err
	}
	return &RoundsManagerTransactor{contract: contract}, nil
}

// bindRoundsManager binds a generic wrapper to an already deployed contract.
func bindRoundsManager(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor) (*bind.BoundContract, error) {
	parsed, err := abi.JSON(strings.NewReader(RoundsManagerABI))
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, parsed, caller, transactor), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_RoundsManager *RoundsManagerRaw) Call(opts *bind.CallOpts, result interface{}, method string, params ...interface{}) error {
	return _RoundsManager.Contract.RoundsManagerCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_RoundsManager *RoundsManagerRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _RoundsManager.Contract.RoundsManagerTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_RoundsManager *RoundsManagerRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _RoundsManager.Contract.RoundsManagerTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_RoundsManager *RoundsManagerCallerRaw) Call(opts *bind.CallOpts, result interface{}, method string, params ...interface{}) error {
	return _RoundsManager.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_RoundsManager *RoundsManagerTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _RoundsManager.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_RoundsManager *RoundsManagerTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _RoundsManager.Contract.contract.Transact(opts, method, params...)
}

// BlockTime is a free data retrieval call binding the contract method 0x48b15166.
//
// Solidity: function blockTime() constant returns(uint256)
func (_RoundsManager *RoundsManagerCaller) BlockTime(opts *bind.CallOpts) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _RoundsManager.contract.Call(opts, out, "blockTime")
	return *ret0, err
}

// BlockTime is a free data retrieval call binding the contract method 0x48b15166.
//
// Solidity: function blockTime() constant returns(uint256)
func (_RoundsManager *RoundsManagerSession) BlockTime() (*big.Int, error) {
	return _RoundsManager.Contract.BlockTime(&_RoundsManager.CallOpts)
}

// BlockTime is a free data retrieval call binding the contract method 0x48b15166.
//
// Solidity: function blockTime() constant returns(uint256)
func (_RoundsManager *RoundsManagerCallerSession) BlockTime() (*big.Int, error) {
	return _RoundsManager.Contract.BlockTime(&_RoundsManager.CallOpts)
}

// Controller is a free data retrieval call binding the contract method 0xf77c4791.
//
// Solidity: function controller() constant returns(address)
func (_RoundsManager *RoundsManagerCaller) Controller(opts *bind.CallOpts) (common.Address, error) {
	var (
		ret0 = new(common.Address)
	)
	out := ret0
	err := _RoundsManager.contract.Call(opts, out, "controller")
	return *ret0, err
}

// Controller is a free data retrieval call binding the contract method 0xf77c4791.
//
// Solidity: function controller() constant returns(address)
func (_RoundsManager *RoundsManagerSession) Controller() (common.Address, error) {
	return _RoundsManager.Contract.Controller(&_RoundsManager.CallOpts)
}

// Controller is a free data retrieval call binding the contract method 0xf77c4791.
//
// Solidity: function controller() constant returns(address)
func (_RoundsManager *RoundsManagerCallerSession) Controller() (common.Address, error) {
	return _RoundsManager.Contract.Controller(&_RoundsManager.CallOpts)
}

// CurrentRound is a free data retrieval call binding the contract method 0x8a19c8bc.
//
// Solidity: function currentRound() constant returns(uint256)
func (_RoundsManager *RoundsManagerCaller) CurrentRound(opts *bind.CallOpts) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _RoundsManager.contract.Call(opts, out, "currentRound")
	return *ret0, err
}

// CurrentRound is a free data retrieval call binding the contract method 0x8a19c8bc.
//
// Solidity: function currentRound() constant returns(uint256)
func (_RoundsManager *RoundsManagerSession) CurrentRound() (*big.Int, error) {
	return _RoundsManager.Contract.CurrentRound(&_RoundsManager.CallOpts)
}

// CurrentRound is a free data retrieval call binding the contract method 0x8a19c8bc.
//
// Solidity: function currentRound() constant returns(uint256)
func (_RoundsManager *RoundsManagerCallerSession) CurrentRound() (*big.Int, error) {
	return _RoundsManager.Contract.CurrentRound(&_RoundsManager.CallOpts)
}

// CurrentRoundInitialized is a free data retrieval call binding the contract method 0x219bc76c.
//
// Solidity: function currentRoundInitialized() constant returns(bool)
func (_RoundsManager *RoundsManagerCaller) CurrentRoundInitialized(opts *bind.CallOpts) (bool, error) {
	var (
		ret0 = new(bool)
	)
	out := ret0
	err := _RoundsManager.contract.Call(opts, out, "currentRoundInitialized")
	return *ret0, err
}

// CurrentRoundInitialized is a free data retrieval call binding the contract method 0x219bc76c.
//
// Solidity: function currentRoundInitialized() constant returns(bool)
func (_RoundsManager *RoundsManagerSession) CurrentRoundInitialized() (bool, error) {
	return _RoundsManager.Contract.CurrentRoundInitialized(&_RoundsManager.CallOpts)
}

// CurrentRoundInitialized is a free data retrieval call binding the contract method 0x219bc76c.
//
// Solidity: function currentRoundInitialized() constant returns(bool)
func (_RoundsManager *RoundsManagerCallerSession) CurrentRoundInitialized() (bool, error) {
	return _RoundsManager.Contract.CurrentRoundInitialized(&_RoundsManager.CallOpts)
}

// CurrentRoundStartBlock is a free data retrieval call binding the contract method 0x8fa148f2.
//
// Solidity: function currentRoundStartBlock() constant returns(uint256)
func (_RoundsManager *RoundsManagerCaller) CurrentRoundStartBlock(opts *bind.CallOpts) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _RoundsManager.contract.Call(opts, out, "currentRoundStartBlock")
	return *ret0, err
}

// CurrentRoundStartBlock is a free data retrieval call binding the contract method 0x8fa148f2.
//
// Solidity: function currentRoundStartBlock() constant returns(uint256)
func (_RoundsManager *RoundsManagerSession) CurrentRoundStartBlock() (*big.Int, error) {
	return _RoundsManager.Contract.CurrentRoundStartBlock(&_RoundsManager.CallOpts)
}

// CurrentRoundStartBlock is a free data retrieval call binding the contract method 0x8fa148f2.
//
// Solidity: function currentRoundStartBlock() constant returns(uint256)
func (_RoundsManager *RoundsManagerCallerSession) CurrentRoundStartBlock() (*big.Int, error) {
	return _RoundsManager.Contract.CurrentRoundStartBlock(&_RoundsManager.CallOpts)
}

// IsInitialized is a free data retrieval call binding the contract method 0x392e53cd.
//
// Solidity: function isInitialized() constant returns(bool)
func (_RoundsManager *RoundsManagerCaller) IsInitialized(opts *bind.CallOpts) (bool, error) {
	var (
		ret0 = new(bool)
	)
	out := ret0
	err := _RoundsManager.contract.Call(opts, out, "isInitialized")
	return *ret0, err
}

// IsInitialized is a free data retrieval call binding the contract method 0x392e53cd.
//
// Solidity: function isInitialized() constant returns(bool)
func (_RoundsManager *RoundsManagerSession) IsInitialized() (bool, error) {
	return _RoundsManager.Contract.IsInitialized(&_RoundsManager.CallOpts)
}

// IsInitialized is a free data retrieval call binding the contract method 0x392e53cd.
//
// Solidity: function isInitialized() constant returns(bool)
func (_RoundsManager *RoundsManagerCallerSession) IsInitialized() (bool, error) {
	return _RoundsManager.Contract.IsInitialized(&_RoundsManager.CallOpts)
}

// LastInitializedRound is a free data retrieval call binding the contract method 0x8807f36e.
//
// Solidity: function lastInitializedRound() constant returns(uint256)
func (_RoundsManager *RoundsManagerCaller) LastInitializedRound(opts *bind.CallOpts) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _RoundsManager.contract.Call(opts, out, "lastInitializedRound")
	return *ret0, err
}

// LastInitializedRound is a free data retrieval call binding the contract method 0x8807f36e.
//
// Solidity: function lastInitializedRound() constant returns(uint256)
func (_RoundsManager *RoundsManagerSession) LastInitializedRound() (*big.Int, error) {
	return _RoundsManager.Contract.LastInitializedRound(&_RoundsManager.CallOpts)
}

// LastInitializedRound is a free data retrieval call binding the contract method 0x8807f36e.
//
// Solidity: function lastInitializedRound() constant returns(uint256)
func (_RoundsManager *RoundsManagerCallerSession) LastInitializedRound() (*big.Int, error) {
	return _RoundsManager.Contract.LastInitializedRound(&_RoundsManager.CallOpts)
}

// RoundLength is a free data retrieval call binding the contract method 0x8b649b94.
//
// Solidity: function roundLength() constant returns(uint256)
func (_RoundsManager *RoundsManagerCaller) RoundLength(opts *bind.CallOpts) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _RoundsManager.contract.Call(opts, out, "roundLength")
	return *ret0, err
}

// RoundLength is a free data retrieval call binding the contract method 0x8b649b94.
//
// Solidity: function roundLength() constant returns(uint256)
func (_RoundsManager *RoundsManagerSession) RoundLength() (*big.Int, error) {
	return _RoundsManager.Contract.RoundLength(&_RoundsManager.CallOpts)
}

// RoundLength is a free data retrieval call binding the contract method 0x8b649b94.
//
// Solidity: function roundLength() constant returns(uint256)
func (_RoundsManager *RoundsManagerCallerSession) RoundLength() (*big.Int, error) {
	return _RoundsManager.Contract.RoundLength(&_RoundsManager.CallOpts)
}

// RoundsPerYear is a free data retrieval call binding the contract method 0x17f60ec7.
//
// Solidity: function roundsPerYear() constant returns(uint256)
func (_RoundsManager *RoundsManagerCaller) RoundsPerYear(opts *bind.CallOpts) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _RoundsManager.contract.Call(opts, out, "roundsPerYear")
	return *ret0, err
}

// RoundsPerYear is a free data retrieval call binding the contract method 0x17f60ec7.
//
// Solidity: function roundsPerYear() constant returns(uint256)
func (_RoundsManager *RoundsManagerSession) RoundsPerYear() (*big.Int, error) {
	return _RoundsManager.Contract.RoundsPerYear(&_RoundsManager.CallOpts)
}

// RoundsPerYear is a free data retrieval call binding the contract method 0x17f60ec7.
//
// Solidity: function roundsPerYear() constant returns(uint256)
func (_RoundsManager *RoundsManagerCallerSession) RoundsPerYear() (*big.Int, error) {
	return _RoundsManager.Contract.RoundsPerYear(&_RoundsManager.CallOpts)
}

// TargetContractId is a free data retrieval call binding the contract method 0x51720b41.
//
// Solidity: function targetContractId() constant returns(bytes32)
func (_RoundsManager *RoundsManagerCaller) TargetContractId(opts *bind.CallOpts) ([32]byte, error) {
	var (
		ret0 = new([32]byte)
	)
	out := ret0
	err := _RoundsManager.contract.Call(opts, out, "targetContractId")
	return *ret0, err
}

// TargetContractId is a free data retrieval call binding the contract method 0x51720b41.
//
// Solidity: function targetContractId() constant returns(bytes32)
func (_RoundsManager *RoundsManagerSession) TargetContractId() ([32]byte, error) {
	return _RoundsManager.Contract.TargetContractId(&_RoundsManager.CallOpts)
}

// TargetContractId is a free data retrieval call binding the contract method 0x51720b41.
//
// Solidity: function targetContractId() constant returns(bytes32)
func (_RoundsManager *RoundsManagerCallerSession) TargetContractId() ([32]byte, error) {
	return _RoundsManager.Contract.TargetContractId(&_RoundsManager.CallOpts)
}

// Initialize is a paid mutator transaction binding the contract method 0xe4a30116.
//
// Solidity: function initialize(_blockTime uint256, _roundLength uint256) returns(bool)
func (_RoundsManager *RoundsManagerTransactor) Initialize(opts *bind.TransactOpts, _blockTime *big.Int, _roundLength *big.Int) (*types.Transaction, error) {
	return _RoundsManager.contract.Transact(opts, "initialize", _blockTime, _roundLength)
}

// Initialize is a paid mutator transaction binding the contract method 0xe4a30116.
//
// Solidity: function initialize(_blockTime uint256, _roundLength uint256) returns(bool)
func (_RoundsManager *RoundsManagerSession) Initialize(_blockTime *big.Int, _roundLength *big.Int) (*types.Transaction, error) {
	return _RoundsManager.Contract.Initialize(&_RoundsManager.TransactOpts, _blockTime, _roundLength)
}

// Initialize is a paid mutator transaction binding the contract method 0xe4a30116.
//
// Solidity: function initialize(_blockTime uint256, _roundLength uint256) returns(bool)
func (_RoundsManager *RoundsManagerTransactorSession) Initialize(_blockTime *big.Int, _roundLength *big.Int) (*types.Transaction, error) {
	return _RoundsManager.Contract.Initialize(&_RoundsManager.TransactOpts, _blockTime, _roundLength)
}

// InitializeRound is a paid mutator transaction binding the contract method 0xd4807fb2.
//
// Solidity: function initializeRound() returns(bool)
func (_RoundsManager *RoundsManagerTransactor) InitializeRound(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _RoundsManager.contract.Transact(opts, "initializeRound")
}

// InitializeRound is a paid mutator transaction binding the contract method 0xd4807fb2.
//
// Solidity: function initializeRound() returns(bool)
func (_RoundsManager *RoundsManagerSession) InitializeRound() (*types.Transaction, error) {
	return _RoundsManager.Contract.InitializeRound(&_RoundsManager.TransactOpts)
}

// InitializeRound is a paid mutator transaction binding the contract method 0xd4807fb2.
//
// Solidity: function initializeRound() returns(bool)
func (_RoundsManager *RoundsManagerTransactorSession) InitializeRound() (*types.Transaction, error) {
	return _RoundsManager.Contract.InitializeRound(&_RoundsManager.TransactOpts)
}

// SetController is a paid mutator transaction binding the contract method 0x92eefe9b.
//
// Solidity: function setController(_controller address) returns(bool)
func (_RoundsManager *RoundsManagerTransactor) SetController(opts *bind.TransactOpts, _controller common.Address) (*types.Transaction, error) {
	return _RoundsManager.contract.Transact(opts, "setController", _controller)
}

// SetController is a paid mutator transaction binding the contract method 0x92eefe9b.
//
// Solidity: function setController(_controller address) returns(bool)
func (_RoundsManager *RoundsManagerSession) SetController(_controller common.Address) (*types.Transaction, error) {
	return _RoundsManager.Contract.SetController(&_RoundsManager.TransactOpts, _controller)
}

// SetController is a paid mutator transaction binding the contract method 0x92eefe9b.
//
// Solidity: function setController(_controller address) returns(bool)
func (_RoundsManager *RoundsManagerTransactorSession) SetController(_controller common.Address) (*types.Transaction, error) {
	return _RoundsManager.Contract.SetController(&_RoundsManager.TransactOpts, _controller)
}
