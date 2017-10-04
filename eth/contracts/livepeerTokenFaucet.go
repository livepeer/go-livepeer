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

// LivepeerTokenFaucetABI is the input ABI used to generate the binding from.
const LivepeerTokenFaucetABI = "[{\"constant\":true,\"inputs\":[],\"name\":\"requestWait\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"\",\"type\":\"address\"}],\"name\":\"nextValidRequest\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":false,\"inputs\":[],\"name\":\"request\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"\",\"type\":\"address\"}],\"name\":\"isWhitelisted\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_addr\",\"type\":\"address\"}],\"name\":\"removeFromWhitelist\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"owner\",\"outputs\":[{\"name\":\"\",\"type\":\"address\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_addr\",\"type\":\"address\"}],\"name\":\"addToWhitelist\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"newOwner\",\"type\":\"address\"}],\"name\":\"transferOwnership\",\"outputs\":[],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"requestAmount\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"token\",\"outputs\":[{\"name\":\"\",\"type\":\"address\"}],\"payable\":false,\"type\":\"function\"},{\"inputs\":[{\"name\":\"_token\",\"type\":\"address\"},{\"name\":\"_requestAmount\",\"type\":\"uint256\"},{\"name\":\"_requestWait\",\"type\":\"uint256\"}],\"payable\":false,\"type\":\"constructor\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"name\":\"to\",\"type\":\"address\"},{\"indexed\":false,\"name\":\"amount\",\"type\":\"uint256\"}],\"name\":\"Request\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"name\":\"previousOwner\",\"type\":\"address\"},{\"indexed\":true,\"name\":\"newOwner\",\"type\":\"address\"}],\"name\":\"OwnershipTransferred\",\"type\":\"event\"}]"

// LivepeerTokenFaucet is an auto generated Go binding around an Ethereum contract.
type LivepeerTokenFaucet struct {
	LivepeerTokenFaucetCaller     // Read-only binding to the contract
	LivepeerTokenFaucetTransactor // Write-only binding to the contract
}

// LivepeerTokenFaucetCaller is an auto generated read-only Go binding around an Ethereum contract.
type LivepeerTokenFaucetCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// LivepeerTokenFaucetTransactor is an auto generated write-only Go binding around an Ethereum contract.
type LivepeerTokenFaucetTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// LivepeerTokenFaucetSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type LivepeerTokenFaucetSession struct {
	Contract     *LivepeerTokenFaucet // Generic contract binding to set the session for
	CallOpts     bind.CallOpts        // Call options to use throughout this session
	TransactOpts bind.TransactOpts    // Transaction auth options to use throughout this session
}

// LivepeerTokenFaucetCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type LivepeerTokenFaucetCallerSession struct {
	Contract *LivepeerTokenFaucetCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts              // Call options to use throughout this session
}

// LivepeerTokenFaucetTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type LivepeerTokenFaucetTransactorSession struct {
	Contract     *LivepeerTokenFaucetTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts              // Transaction auth options to use throughout this session
}

// LivepeerTokenFaucetRaw is an auto generated low-level Go binding around an Ethereum contract.
type LivepeerTokenFaucetRaw struct {
	Contract *LivepeerTokenFaucet // Generic contract binding to access the raw methods on
}

// LivepeerTokenFaucetCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type LivepeerTokenFaucetCallerRaw struct {
	Contract *LivepeerTokenFaucetCaller // Generic read-only contract binding to access the raw methods on
}

// LivepeerTokenFaucetTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type LivepeerTokenFaucetTransactorRaw struct {
	Contract *LivepeerTokenFaucetTransactor // Generic write-only contract binding to access the raw methods on
}

// NewLivepeerTokenFaucet creates a new instance of LivepeerTokenFaucet, bound to a specific deployed contract.
func NewLivepeerTokenFaucet(address common.Address, backend bind.ContractBackend) (*LivepeerTokenFaucet, error) {
	contract, err := bindLivepeerTokenFaucet(address, backend, backend)
	if err != nil {
		return nil, err
	}
	return &LivepeerTokenFaucet{LivepeerTokenFaucetCaller: LivepeerTokenFaucetCaller{contract: contract}, LivepeerTokenFaucetTransactor: LivepeerTokenFaucetTransactor{contract: contract}}, nil
}

// NewLivepeerTokenFaucetCaller creates a new read-only instance of LivepeerTokenFaucet, bound to a specific deployed contract.
func NewLivepeerTokenFaucetCaller(address common.Address, caller bind.ContractCaller) (*LivepeerTokenFaucetCaller, error) {
	contract, err := bindLivepeerTokenFaucet(address, caller, nil)
	if err != nil {
		return nil, err
	}
	return &LivepeerTokenFaucetCaller{contract: contract}, nil
}

// NewLivepeerTokenFaucetTransactor creates a new write-only instance of LivepeerTokenFaucet, bound to a specific deployed contract.
func NewLivepeerTokenFaucetTransactor(address common.Address, transactor bind.ContractTransactor) (*LivepeerTokenFaucetTransactor, error) {
	contract, err := bindLivepeerTokenFaucet(address, nil, transactor)
	if err != nil {
		return nil, err
	}
	return &LivepeerTokenFaucetTransactor{contract: contract}, nil
}

// bindLivepeerTokenFaucet binds a generic wrapper to an already deployed contract.
func bindLivepeerTokenFaucet(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor) (*bind.BoundContract, error) {
	parsed, err := abi.JSON(strings.NewReader(LivepeerTokenFaucetABI))
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, parsed, caller, transactor), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_LivepeerTokenFaucet *LivepeerTokenFaucetRaw) Call(opts *bind.CallOpts, result interface{}, method string, params ...interface{}) error {
	return _LivepeerTokenFaucet.Contract.LivepeerTokenFaucetCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_LivepeerTokenFaucet *LivepeerTokenFaucetRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _LivepeerTokenFaucet.Contract.LivepeerTokenFaucetTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_LivepeerTokenFaucet *LivepeerTokenFaucetRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _LivepeerTokenFaucet.Contract.LivepeerTokenFaucetTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_LivepeerTokenFaucet *LivepeerTokenFaucetCallerRaw) Call(opts *bind.CallOpts, result interface{}, method string, params ...interface{}) error {
	return _LivepeerTokenFaucet.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_LivepeerTokenFaucet *LivepeerTokenFaucetTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _LivepeerTokenFaucet.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_LivepeerTokenFaucet *LivepeerTokenFaucetTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _LivepeerTokenFaucet.Contract.contract.Transact(opts, method, params...)
}

// IsWhitelisted is a free data retrieval call binding the contract method 0x3af32abf.
//
// Solidity: function isWhitelisted( address) constant returns(bool)
func (_LivepeerTokenFaucet *LivepeerTokenFaucetCaller) IsWhitelisted(opts *bind.CallOpts, arg0 common.Address) (bool, error) {
	var (
		ret0 = new(bool)
	)
	out := ret0
	err := _LivepeerTokenFaucet.contract.Call(opts, out, "isWhitelisted", arg0)
	return *ret0, err
}

// IsWhitelisted is a free data retrieval call binding the contract method 0x3af32abf.
//
// Solidity: function isWhitelisted( address) constant returns(bool)
func (_LivepeerTokenFaucet *LivepeerTokenFaucetSession) IsWhitelisted(arg0 common.Address) (bool, error) {
	return _LivepeerTokenFaucet.Contract.IsWhitelisted(&_LivepeerTokenFaucet.CallOpts, arg0)
}

// IsWhitelisted is a free data retrieval call binding the contract method 0x3af32abf.
//
// Solidity: function isWhitelisted( address) constant returns(bool)
func (_LivepeerTokenFaucet *LivepeerTokenFaucetCallerSession) IsWhitelisted(arg0 common.Address) (bool, error) {
	return _LivepeerTokenFaucet.Contract.IsWhitelisted(&_LivepeerTokenFaucet.CallOpts, arg0)
}

// NextValidRequest is a free data retrieval call binding the contract method 0x207f5ce6.
//
// Solidity: function nextValidRequest( address) constant returns(uint256)
func (_LivepeerTokenFaucet *LivepeerTokenFaucetCaller) NextValidRequest(opts *bind.CallOpts, arg0 common.Address) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _LivepeerTokenFaucet.contract.Call(opts, out, "nextValidRequest", arg0)
	return *ret0, err
}

// NextValidRequest is a free data retrieval call binding the contract method 0x207f5ce6.
//
// Solidity: function nextValidRequest( address) constant returns(uint256)
func (_LivepeerTokenFaucet *LivepeerTokenFaucetSession) NextValidRequest(arg0 common.Address) (*big.Int, error) {
	return _LivepeerTokenFaucet.Contract.NextValidRequest(&_LivepeerTokenFaucet.CallOpts, arg0)
}

// NextValidRequest is a free data retrieval call binding the contract method 0x207f5ce6.
//
// Solidity: function nextValidRequest( address) constant returns(uint256)
func (_LivepeerTokenFaucet *LivepeerTokenFaucetCallerSession) NextValidRequest(arg0 common.Address) (*big.Int, error) {
	return _LivepeerTokenFaucet.Contract.NextValidRequest(&_LivepeerTokenFaucet.CallOpts, arg0)
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() constant returns(address)
func (_LivepeerTokenFaucet *LivepeerTokenFaucetCaller) Owner(opts *bind.CallOpts) (common.Address, error) {
	var (
		ret0 = new(common.Address)
	)
	out := ret0
	err := _LivepeerTokenFaucet.contract.Call(opts, out, "owner")
	return *ret0, err
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() constant returns(address)
func (_LivepeerTokenFaucet *LivepeerTokenFaucetSession) Owner() (common.Address, error) {
	return _LivepeerTokenFaucet.Contract.Owner(&_LivepeerTokenFaucet.CallOpts)
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() constant returns(address)
func (_LivepeerTokenFaucet *LivepeerTokenFaucetCallerSession) Owner() (common.Address, error) {
	return _LivepeerTokenFaucet.Contract.Owner(&_LivepeerTokenFaucet.CallOpts)
}

// RequestAmount is a free data retrieval call binding the contract method 0xf52ec46c.
//
// Solidity: function requestAmount() constant returns(uint256)
func (_LivepeerTokenFaucet *LivepeerTokenFaucetCaller) RequestAmount(opts *bind.CallOpts) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _LivepeerTokenFaucet.contract.Call(opts, out, "requestAmount")
	return *ret0, err
}

// RequestAmount is a free data retrieval call binding the contract method 0xf52ec46c.
//
// Solidity: function requestAmount() constant returns(uint256)
func (_LivepeerTokenFaucet *LivepeerTokenFaucetSession) RequestAmount() (*big.Int, error) {
	return _LivepeerTokenFaucet.Contract.RequestAmount(&_LivepeerTokenFaucet.CallOpts)
}

// RequestAmount is a free data retrieval call binding the contract method 0xf52ec46c.
//
// Solidity: function requestAmount() constant returns(uint256)
func (_LivepeerTokenFaucet *LivepeerTokenFaucetCallerSession) RequestAmount() (*big.Int, error) {
	return _LivepeerTokenFaucet.Contract.RequestAmount(&_LivepeerTokenFaucet.CallOpts)
}

// RequestWait is a free data retrieval call binding the contract method 0x0d6c51b3.
//
// Solidity: function requestWait() constant returns(uint256)
func (_LivepeerTokenFaucet *LivepeerTokenFaucetCaller) RequestWait(opts *bind.CallOpts) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _LivepeerTokenFaucet.contract.Call(opts, out, "requestWait")
	return *ret0, err
}

// RequestWait is a free data retrieval call binding the contract method 0x0d6c51b3.
//
// Solidity: function requestWait() constant returns(uint256)
func (_LivepeerTokenFaucet *LivepeerTokenFaucetSession) RequestWait() (*big.Int, error) {
	return _LivepeerTokenFaucet.Contract.RequestWait(&_LivepeerTokenFaucet.CallOpts)
}

// RequestWait is a free data retrieval call binding the contract method 0x0d6c51b3.
//
// Solidity: function requestWait() constant returns(uint256)
func (_LivepeerTokenFaucet *LivepeerTokenFaucetCallerSession) RequestWait() (*big.Int, error) {
	return _LivepeerTokenFaucet.Contract.RequestWait(&_LivepeerTokenFaucet.CallOpts)
}

// Token is a free data retrieval call binding the contract method 0xfc0c546a.
//
// Solidity: function token() constant returns(address)
func (_LivepeerTokenFaucet *LivepeerTokenFaucetCaller) Token(opts *bind.CallOpts) (common.Address, error) {
	var (
		ret0 = new(common.Address)
	)
	out := ret0
	err := _LivepeerTokenFaucet.contract.Call(opts, out, "token")
	return *ret0, err
}

// Token is a free data retrieval call binding the contract method 0xfc0c546a.
//
// Solidity: function token() constant returns(address)
func (_LivepeerTokenFaucet *LivepeerTokenFaucetSession) Token() (common.Address, error) {
	return _LivepeerTokenFaucet.Contract.Token(&_LivepeerTokenFaucet.CallOpts)
}

// Token is a free data retrieval call binding the contract method 0xfc0c546a.
//
// Solidity: function token() constant returns(address)
func (_LivepeerTokenFaucet *LivepeerTokenFaucetCallerSession) Token() (common.Address, error) {
	return _LivepeerTokenFaucet.Contract.Token(&_LivepeerTokenFaucet.CallOpts)
}

// AddToWhitelist is a paid mutator transaction binding the contract method 0xe43252d7.
//
// Solidity: function addToWhitelist(_addr address) returns(bool)
func (_LivepeerTokenFaucet *LivepeerTokenFaucetTransactor) AddToWhitelist(opts *bind.TransactOpts, _addr common.Address) (*types.Transaction, error) {
	return _LivepeerTokenFaucet.contract.Transact(opts, "addToWhitelist", _addr)
}

// AddToWhitelist is a paid mutator transaction binding the contract method 0xe43252d7.
//
// Solidity: function addToWhitelist(_addr address) returns(bool)
func (_LivepeerTokenFaucet *LivepeerTokenFaucetSession) AddToWhitelist(_addr common.Address) (*types.Transaction, error) {
	return _LivepeerTokenFaucet.Contract.AddToWhitelist(&_LivepeerTokenFaucet.TransactOpts, _addr)
}

// AddToWhitelist is a paid mutator transaction binding the contract method 0xe43252d7.
//
// Solidity: function addToWhitelist(_addr address) returns(bool)
func (_LivepeerTokenFaucet *LivepeerTokenFaucetTransactorSession) AddToWhitelist(_addr common.Address) (*types.Transaction, error) {
	return _LivepeerTokenFaucet.Contract.AddToWhitelist(&_LivepeerTokenFaucet.TransactOpts, _addr)
}

// RemoveFromWhitelist is a paid mutator transaction binding the contract method 0x8ab1d681.
//
// Solidity: function removeFromWhitelist(_addr address) returns(bool)
func (_LivepeerTokenFaucet *LivepeerTokenFaucetTransactor) RemoveFromWhitelist(opts *bind.TransactOpts, _addr common.Address) (*types.Transaction, error) {
	return _LivepeerTokenFaucet.contract.Transact(opts, "removeFromWhitelist", _addr)
}

// RemoveFromWhitelist is a paid mutator transaction binding the contract method 0x8ab1d681.
//
// Solidity: function removeFromWhitelist(_addr address) returns(bool)
func (_LivepeerTokenFaucet *LivepeerTokenFaucetSession) RemoveFromWhitelist(_addr common.Address) (*types.Transaction, error) {
	return _LivepeerTokenFaucet.Contract.RemoveFromWhitelist(&_LivepeerTokenFaucet.TransactOpts, _addr)
}

// RemoveFromWhitelist is a paid mutator transaction binding the contract method 0x8ab1d681.
//
// Solidity: function removeFromWhitelist(_addr address) returns(bool)
func (_LivepeerTokenFaucet *LivepeerTokenFaucetTransactorSession) RemoveFromWhitelist(_addr common.Address) (*types.Transaction, error) {
	return _LivepeerTokenFaucet.Contract.RemoveFromWhitelist(&_LivepeerTokenFaucet.TransactOpts, _addr)
}

// Request is a paid mutator transaction binding the contract method 0x338cdca1.
//
// Solidity: function request() returns(bool)
func (_LivepeerTokenFaucet *LivepeerTokenFaucetTransactor) Request(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _LivepeerTokenFaucet.contract.Transact(opts, "request")
}

// Request is a paid mutator transaction binding the contract method 0x338cdca1.
//
// Solidity: function request() returns(bool)
func (_LivepeerTokenFaucet *LivepeerTokenFaucetSession) Request() (*types.Transaction, error) {
	return _LivepeerTokenFaucet.Contract.Request(&_LivepeerTokenFaucet.TransactOpts)
}

// Request is a paid mutator transaction binding the contract method 0x338cdca1.
//
// Solidity: function request() returns(bool)
func (_LivepeerTokenFaucet *LivepeerTokenFaucetTransactorSession) Request() (*types.Transaction, error) {
	return _LivepeerTokenFaucet.Contract.Request(&_LivepeerTokenFaucet.TransactOpts)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(newOwner address) returns()
func (_LivepeerTokenFaucet *LivepeerTokenFaucetTransactor) TransferOwnership(opts *bind.TransactOpts, newOwner common.Address) (*types.Transaction, error) {
	return _LivepeerTokenFaucet.contract.Transact(opts, "transferOwnership", newOwner)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(newOwner address) returns()
func (_LivepeerTokenFaucet *LivepeerTokenFaucetSession) TransferOwnership(newOwner common.Address) (*types.Transaction, error) {
	return _LivepeerTokenFaucet.Contract.TransferOwnership(&_LivepeerTokenFaucet.TransactOpts, newOwner)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(newOwner address) returns()
func (_LivepeerTokenFaucet *LivepeerTokenFaucetTransactorSession) TransferOwnership(newOwner common.Address) (*types.Transaction, error) {
	return _LivepeerTokenFaucet.Contract.TransferOwnership(&_LivepeerTokenFaucet.TransactOpts, newOwner)
}
