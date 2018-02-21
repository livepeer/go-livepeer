// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package contracts

import (
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

// ControllerABI is the input ABI used to generate the binding from.
const ControllerABI = "[{\"constant\":false,\"inputs\":[],\"name\":\"unpause\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"paused\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[],\"name\":\"pause\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"owner\",\"outputs\":[{\"name\":\"\",\"type\":\"address\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"newOwner\",\"type\":\"address\"}],\"name\":\"transferOwnership\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"constructor\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"name\":\"id\",\"type\":\"bytes32\"},{\"indexed\":false,\"name\":\"contractAddress\",\"type\":\"address\"},{\"indexed\":false,\"name\":\"gitCommitHash\",\"type\":\"bytes20\"}],\"name\":\"SetContractInfo\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[],\"name\":\"Pause\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[],\"name\":\"Unpause\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"name\":\"previousOwner\",\"type\":\"address\"},{\"indexed\":true,\"name\":\"newOwner\",\"type\":\"address\"}],\"name\":\"OwnershipTransferred\",\"type\":\"event\"},{\"constant\":false,\"inputs\":[{\"name\":\"_id\",\"type\":\"bytes32\"},{\"name\":\"_contractAddress\",\"type\":\"address\"},{\"name\":\"_gitCommitHash\",\"type\":\"bytes20\"}],\"name\":\"setContractInfo\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_id\",\"type\":\"bytes32\"},{\"name\":\"_controller\",\"type\":\"address\"}],\"name\":\"updateController\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"_id\",\"type\":\"bytes32\"}],\"name\":\"getContractInfo\",\"outputs\":[{\"name\":\"\",\"type\":\"address\"},{\"name\":\"\",\"type\":\"bytes20\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"_id\",\"type\":\"bytes32\"}],\"name\":\"getContract\",\"outputs\":[{\"name\":\"\",\"type\":\"address\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"}]"

// Controller is an auto generated Go binding around an Ethereum contract.
type Controller struct {
	ControllerCaller     // Read-only binding to the contract
	ControllerTransactor // Write-only binding to the contract
}

// ControllerCaller is an auto generated read-only Go binding around an Ethereum contract.
type ControllerCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// ControllerTransactor is an auto generated write-only Go binding around an Ethereum contract.
type ControllerTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// ControllerSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type ControllerSession struct {
	Contract     *Controller       // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// ControllerCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type ControllerCallerSession struct {
	Contract *ControllerCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts     // Call options to use throughout this session
}

// ControllerTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type ControllerTransactorSession struct {
	Contract     *ControllerTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts     // Transaction auth options to use throughout this session
}

// ControllerRaw is an auto generated low-level Go binding around an Ethereum contract.
type ControllerRaw struct {
	Contract *Controller // Generic contract binding to access the raw methods on
}

// ControllerCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type ControllerCallerRaw struct {
	Contract *ControllerCaller // Generic read-only contract binding to access the raw methods on
}

// ControllerTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type ControllerTransactorRaw struct {
	Contract *ControllerTransactor // Generic write-only contract binding to access the raw methods on
}

// NewController creates a new instance of Controller, bound to a specific deployed contract.
func NewController(address common.Address, backend bind.ContractBackend) (*Controller, error) {
	contract, err := bindController(address, backend, backend)
	if err != nil {
		return nil, err
	}
	return &Controller{ControllerCaller: ControllerCaller{contract: contract}, ControllerTransactor: ControllerTransactor{contract: contract}}, nil
}

// NewControllerCaller creates a new read-only instance of Controller, bound to a specific deployed contract.
func NewControllerCaller(address common.Address, caller bind.ContractCaller) (*ControllerCaller, error) {
	contract, err := bindController(address, caller, nil)
	if err != nil {
		return nil, err
	}
	return &ControllerCaller{contract: contract}, nil
}

// NewControllerTransactor creates a new write-only instance of Controller, bound to a specific deployed contract.
func NewControllerTransactor(address common.Address, transactor bind.ContractTransactor) (*ControllerTransactor, error) {
	contract, err := bindController(address, nil, transactor)
	if err != nil {
		return nil, err
	}
	return &ControllerTransactor{contract: contract}, nil
}

// bindController binds a generic wrapper to an already deployed contract.
func bindController(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor) (*bind.BoundContract, error) {
	parsed, err := abi.JSON(strings.NewReader(ControllerABI))
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, parsed, caller, transactor), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_Controller *ControllerRaw) Call(opts *bind.CallOpts, result interface{}, method string, params ...interface{}) error {
	return _Controller.Contract.ControllerCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_Controller *ControllerRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Controller.Contract.ControllerTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_Controller *ControllerRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _Controller.Contract.ControllerTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_Controller *ControllerCallerRaw) Call(opts *bind.CallOpts, result interface{}, method string, params ...interface{}) error {
	return _Controller.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_Controller *ControllerTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Controller.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_Controller *ControllerTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _Controller.Contract.contract.Transact(opts, method, params...)
}

// GetContract is a free data retrieval call binding the contract method 0xe16c7d98.
//
// Solidity: function getContract(_id bytes32) constant returns(address)
func (_Controller *ControllerCaller) GetContract(opts *bind.CallOpts, _id [32]byte) (common.Address, error) {
	var (
		ret0 = new(common.Address)
	)
	out := ret0
	err := _Controller.contract.Call(opts, out, "getContract", _id)
	return *ret0, err
}

// GetContract is a free data retrieval call binding the contract method 0xe16c7d98.
//
// Solidity: function getContract(_id bytes32) constant returns(address)
func (_Controller *ControllerSession) GetContract(_id [32]byte) (common.Address, error) {
	return _Controller.Contract.GetContract(&_Controller.CallOpts, _id)
}

// GetContract is a free data retrieval call binding the contract method 0xe16c7d98.
//
// Solidity: function getContract(_id bytes32) constant returns(address)
func (_Controller *ControllerCallerSession) GetContract(_id [32]byte) (common.Address, error) {
	return _Controller.Contract.GetContract(&_Controller.CallOpts, _id)
}

// GetContractInfo is a free data retrieval call binding the contract method 0x613e2de2.
//
// Solidity: function getContractInfo(_id bytes32) constant returns(address, bytes20)
func (_Controller *ControllerCaller) GetContractInfo(opts *bind.CallOpts, _id [32]byte) (common.Address, [20]byte, error) {
	var (
		ret0 = new(common.Address)
		ret1 = new([20]byte)
	)
	out := &[]interface{}{
		ret0,
		ret1,
	}
	err := _Controller.contract.Call(opts, out, "getContractInfo", _id)
	return *ret0, *ret1, err
}

// GetContractInfo is a free data retrieval call binding the contract method 0x613e2de2.
//
// Solidity: function getContractInfo(_id bytes32) constant returns(address, bytes20)
func (_Controller *ControllerSession) GetContractInfo(_id [32]byte) (common.Address, [20]byte, error) {
	return _Controller.Contract.GetContractInfo(&_Controller.CallOpts, _id)
}

// GetContractInfo is a free data retrieval call binding the contract method 0x613e2de2.
//
// Solidity: function getContractInfo(_id bytes32) constant returns(address, bytes20)
func (_Controller *ControllerCallerSession) GetContractInfo(_id [32]byte) (common.Address, [20]byte, error) {
	return _Controller.Contract.GetContractInfo(&_Controller.CallOpts, _id)
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() constant returns(address)
func (_Controller *ControllerCaller) Owner(opts *bind.CallOpts) (common.Address, error) {
	var (
		ret0 = new(common.Address)
	)
	out := ret0
	err := _Controller.contract.Call(opts, out, "owner")
	return *ret0, err
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() constant returns(address)
func (_Controller *ControllerSession) Owner() (common.Address, error) {
	return _Controller.Contract.Owner(&_Controller.CallOpts)
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() constant returns(address)
func (_Controller *ControllerCallerSession) Owner() (common.Address, error) {
	return _Controller.Contract.Owner(&_Controller.CallOpts)
}

// Paused is a free data retrieval call binding the contract method 0x5c975abb.
//
// Solidity: function paused() constant returns(bool)
func (_Controller *ControllerCaller) Paused(opts *bind.CallOpts) (bool, error) {
	var (
		ret0 = new(bool)
	)
	out := ret0
	err := _Controller.contract.Call(opts, out, "paused")
	return *ret0, err
}

// Paused is a free data retrieval call binding the contract method 0x5c975abb.
//
// Solidity: function paused() constant returns(bool)
func (_Controller *ControllerSession) Paused() (bool, error) {
	return _Controller.Contract.Paused(&_Controller.CallOpts)
}

// Paused is a free data retrieval call binding the contract method 0x5c975abb.
//
// Solidity: function paused() constant returns(bool)
func (_Controller *ControllerCallerSession) Paused() (bool, error) {
	return _Controller.Contract.Paused(&_Controller.CallOpts)
}

// Pause is a paid mutator transaction binding the contract method 0x8456cb59.
//
// Solidity: function pause() returns()
func (_Controller *ControllerTransactor) Pause(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Controller.contract.Transact(opts, "pause")
}

// Pause is a paid mutator transaction binding the contract method 0x8456cb59.
//
// Solidity: function pause() returns()
func (_Controller *ControllerSession) Pause() (*types.Transaction, error) {
	return _Controller.Contract.Pause(&_Controller.TransactOpts)
}

// Pause is a paid mutator transaction binding the contract method 0x8456cb59.
//
// Solidity: function pause() returns()
func (_Controller *ControllerTransactorSession) Pause() (*types.Transaction, error) {
	return _Controller.Contract.Pause(&_Controller.TransactOpts)
}

// SetContractInfo is a paid mutator transaction binding the contract method 0xd737c2b0.
//
// Solidity: function setContractInfo(_id bytes32, _contractAddress address, _gitCommitHash bytes20) returns()
func (_Controller *ControllerTransactor) SetContractInfo(opts *bind.TransactOpts, _id [32]byte, _contractAddress common.Address, _gitCommitHash [20]byte) (*types.Transaction, error) {
	return _Controller.contract.Transact(opts, "setContractInfo", _id, _contractAddress, _gitCommitHash)
}

// SetContractInfo is a paid mutator transaction binding the contract method 0xd737c2b0.
//
// Solidity: function setContractInfo(_id bytes32, _contractAddress address, _gitCommitHash bytes20) returns()
func (_Controller *ControllerSession) SetContractInfo(_id [32]byte, _contractAddress common.Address, _gitCommitHash [20]byte) (*types.Transaction, error) {
	return _Controller.Contract.SetContractInfo(&_Controller.TransactOpts, _id, _contractAddress, _gitCommitHash)
}

// SetContractInfo is a paid mutator transaction binding the contract method 0xd737c2b0.
//
// Solidity: function setContractInfo(_id bytes32, _contractAddress address, _gitCommitHash bytes20) returns()
func (_Controller *ControllerTransactorSession) SetContractInfo(_id [32]byte, _contractAddress common.Address, _gitCommitHash [20]byte) (*types.Transaction, error) {
	return _Controller.Contract.SetContractInfo(&_Controller.TransactOpts, _id, _contractAddress, _gitCommitHash)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(newOwner address) returns()
func (_Controller *ControllerTransactor) TransferOwnership(opts *bind.TransactOpts, newOwner common.Address) (*types.Transaction, error) {
	return _Controller.contract.Transact(opts, "transferOwnership", newOwner)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(newOwner address) returns()
func (_Controller *ControllerSession) TransferOwnership(newOwner common.Address) (*types.Transaction, error) {
	return _Controller.Contract.TransferOwnership(&_Controller.TransactOpts, newOwner)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(newOwner address) returns()
func (_Controller *ControllerTransactorSession) TransferOwnership(newOwner common.Address) (*types.Transaction, error) {
	return _Controller.Contract.TransferOwnership(&_Controller.TransactOpts, newOwner)
}

// Unpause is a paid mutator transaction binding the contract method 0x3f4ba83a.
//
// Solidity: function unpause() returns()
func (_Controller *ControllerTransactor) Unpause(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Controller.contract.Transact(opts, "unpause")
}

// Unpause is a paid mutator transaction binding the contract method 0x3f4ba83a.
//
// Solidity: function unpause() returns()
func (_Controller *ControllerSession) Unpause() (*types.Transaction, error) {
	return _Controller.Contract.Unpause(&_Controller.TransactOpts)
}

// Unpause is a paid mutator transaction binding the contract method 0x3f4ba83a.
//
// Solidity: function unpause() returns()
func (_Controller *ControllerTransactorSession) Unpause() (*types.Transaction, error) {
	return _Controller.Contract.Unpause(&_Controller.TransactOpts)
}

// UpdateController is a paid mutator transaction binding the contract method 0xeb5dd94f.
//
// Solidity: function updateController(_id bytes32, _controller address) returns()
func (_Controller *ControllerTransactor) UpdateController(opts *bind.TransactOpts, _id [32]byte, _controller common.Address) (*types.Transaction, error) {
	return _Controller.contract.Transact(opts, "updateController", _id, _controller)
}

// UpdateController is a paid mutator transaction binding the contract method 0xeb5dd94f.
//
// Solidity: function updateController(_id bytes32, _controller address) returns()
func (_Controller *ControllerSession) UpdateController(_id [32]byte, _controller common.Address) (*types.Transaction, error) {
	return _Controller.Contract.UpdateController(&_Controller.TransactOpts, _id, _controller)
}

// UpdateController is a paid mutator transaction binding the contract method 0xeb5dd94f.
//
// Solidity: function updateController(_id bytes32, _controller address) returns()
func (_Controller *ControllerTransactorSession) UpdateController(_id [32]byte, _controller common.Address) (*types.Transaction, error) {
	return _Controller.Contract.UpdateController(&_Controller.TransactOpts, _id, _controller)
}
