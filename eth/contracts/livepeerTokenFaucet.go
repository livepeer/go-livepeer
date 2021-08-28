// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package contracts

import (
	"errors"
	"math/big"
	"strings"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
)

// Reference imports to suppress errors if they are not otherwise used.
var (
	_ = errors.New
	_ = big.NewInt
	_ = strings.NewReader
	_ = ethereum.NotFound
	_ = bind.Bind
	_ = common.Big1
	_ = types.BloomLookup
	_ = event.NewSubscription
)

// LivepeerTokenFaucetMetaData contains all meta data concerning the LivepeerTokenFaucet contract.
var LivepeerTokenFaucetMetaData = &bind.MetaData{
	ABI: "[{\"constant\":true,\"inputs\":[],\"name\":\"requestWait\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"name\":\"nextValidRequest\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[],\"name\":\"request\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"name\":\"isWhitelisted\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"internalType\":\"address\",\"name\":\"_addr\",\"type\":\"address\"}],\"name\":\"removeFromWhitelist\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"owner\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"internalType\":\"address\",\"name\":\"_addr\",\"type\":\"address\"}],\"name\":\"addToWhitelist\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"internalType\":\"address\",\"name\":\"newOwner\",\"type\":\"address\"}],\"name\":\"transferOwnership\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"requestAmount\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"token\",\"outputs\":[{\"internalType\":\"contractILivepeerToken\",\"name\":\"\",\"type\":\"address\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"_token\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"_requestAmount\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"_requestWait\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"constructor\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"to\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"}],\"name\":\"Request\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"previousOwner\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"newOwner\",\"type\":\"address\"}],\"name\":\"OwnershipTransferred\",\"type\":\"event\"}]",
}

// LivepeerTokenFaucetABI is the input ABI used to generate the binding from.
// Deprecated: Use LivepeerTokenFaucetMetaData.ABI instead.
var LivepeerTokenFaucetABI = LivepeerTokenFaucetMetaData.ABI

// LivepeerTokenFaucet is an auto generated Go binding around an Ethereum contract.
type LivepeerTokenFaucet struct {
	LivepeerTokenFaucetCaller     // Read-only binding to the contract
	LivepeerTokenFaucetTransactor // Write-only binding to the contract
	LivepeerTokenFaucetFilterer   // Log filterer for contract events
}

// LivepeerTokenFaucetCaller is an auto generated read-only Go binding around an Ethereum contract.
type LivepeerTokenFaucetCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// LivepeerTokenFaucetTransactor is an auto generated write-only Go binding around an Ethereum contract.
type LivepeerTokenFaucetTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// LivepeerTokenFaucetFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type LivepeerTokenFaucetFilterer struct {
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
	contract, err := bindLivepeerTokenFaucet(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &LivepeerTokenFaucet{LivepeerTokenFaucetCaller: LivepeerTokenFaucetCaller{contract: contract}, LivepeerTokenFaucetTransactor: LivepeerTokenFaucetTransactor{contract: contract}, LivepeerTokenFaucetFilterer: LivepeerTokenFaucetFilterer{contract: contract}}, nil
}

// NewLivepeerTokenFaucetCaller creates a new read-only instance of LivepeerTokenFaucet, bound to a specific deployed contract.
func NewLivepeerTokenFaucetCaller(address common.Address, caller bind.ContractCaller) (*LivepeerTokenFaucetCaller, error) {
	contract, err := bindLivepeerTokenFaucet(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &LivepeerTokenFaucetCaller{contract: contract}, nil
}

// NewLivepeerTokenFaucetTransactor creates a new write-only instance of LivepeerTokenFaucet, bound to a specific deployed contract.
func NewLivepeerTokenFaucetTransactor(address common.Address, transactor bind.ContractTransactor) (*LivepeerTokenFaucetTransactor, error) {
	contract, err := bindLivepeerTokenFaucet(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &LivepeerTokenFaucetTransactor{contract: contract}, nil
}

// NewLivepeerTokenFaucetFilterer creates a new log filterer instance of LivepeerTokenFaucet, bound to a specific deployed contract.
func NewLivepeerTokenFaucetFilterer(address common.Address, filterer bind.ContractFilterer) (*LivepeerTokenFaucetFilterer, error) {
	contract, err := bindLivepeerTokenFaucet(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &LivepeerTokenFaucetFilterer{contract: contract}, nil
}

// bindLivepeerTokenFaucet binds a generic wrapper to an already deployed contract.
func bindLivepeerTokenFaucet(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := abi.JSON(strings.NewReader(LivepeerTokenFaucetABI))
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_LivepeerTokenFaucet *LivepeerTokenFaucetRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
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
func (_LivepeerTokenFaucet *LivepeerTokenFaucetCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
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
// Solidity: function isWhitelisted(address ) view returns(bool)
func (_LivepeerTokenFaucet *LivepeerTokenFaucetCaller) IsWhitelisted(opts *bind.CallOpts, arg0 common.Address) (bool, error) {
	var out []interface{}
	err := _LivepeerTokenFaucet.contract.Call(opts, &out, "isWhitelisted", arg0)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// IsWhitelisted is a free data retrieval call binding the contract method 0x3af32abf.
//
// Solidity: function isWhitelisted(address ) view returns(bool)
func (_LivepeerTokenFaucet *LivepeerTokenFaucetSession) IsWhitelisted(arg0 common.Address) (bool, error) {
	return _LivepeerTokenFaucet.Contract.IsWhitelisted(&_LivepeerTokenFaucet.CallOpts, arg0)
}

// IsWhitelisted is a free data retrieval call binding the contract method 0x3af32abf.
//
// Solidity: function isWhitelisted(address ) view returns(bool)
func (_LivepeerTokenFaucet *LivepeerTokenFaucetCallerSession) IsWhitelisted(arg0 common.Address) (bool, error) {
	return _LivepeerTokenFaucet.Contract.IsWhitelisted(&_LivepeerTokenFaucet.CallOpts, arg0)
}

// NextValidRequest is a free data retrieval call binding the contract method 0x207f5ce6.
//
// Solidity: function nextValidRequest(address ) view returns(uint256)
func (_LivepeerTokenFaucet *LivepeerTokenFaucetCaller) NextValidRequest(opts *bind.CallOpts, arg0 common.Address) (*big.Int, error) {
	var out []interface{}
	err := _LivepeerTokenFaucet.contract.Call(opts, &out, "nextValidRequest", arg0)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// NextValidRequest is a free data retrieval call binding the contract method 0x207f5ce6.
//
// Solidity: function nextValidRequest(address ) view returns(uint256)
func (_LivepeerTokenFaucet *LivepeerTokenFaucetSession) NextValidRequest(arg0 common.Address) (*big.Int, error) {
	return _LivepeerTokenFaucet.Contract.NextValidRequest(&_LivepeerTokenFaucet.CallOpts, arg0)
}

// NextValidRequest is a free data retrieval call binding the contract method 0x207f5ce6.
//
// Solidity: function nextValidRequest(address ) view returns(uint256)
func (_LivepeerTokenFaucet *LivepeerTokenFaucetCallerSession) NextValidRequest(arg0 common.Address) (*big.Int, error) {
	return _LivepeerTokenFaucet.Contract.NextValidRequest(&_LivepeerTokenFaucet.CallOpts, arg0)
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_LivepeerTokenFaucet *LivepeerTokenFaucetCaller) Owner(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _LivepeerTokenFaucet.contract.Call(opts, &out, "owner")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_LivepeerTokenFaucet *LivepeerTokenFaucetSession) Owner() (common.Address, error) {
	return _LivepeerTokenFaucet.Contract.Owner(&_LivepeerTokenFaucet.CallOpts)
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_LivepeerTokenFaucet *LivepeerTokenFaucetCallerSession) Owner() (common.Address, error) {
	return _LivepeerTokenFaucet.Contract.Owner(&_LivepeerTokenFaucet.CallOpts)
}

// RequestAmount is a free data retrieval call binding the contract method 0xf52ec46c.
//
// Solidity: function requestAmount() view returns(uint256)
func (_LivepeerTokenFaucet *LivepeerTokenFaucetCaller) RequestAmount(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _LivepeerTokenFaucet.contract.Call(opts, &out, "requestAmount")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// RequestAmount is a free data retrieval call binding the contract method 0xf52ec46c.
//
// Solidity: function requestAmount() view returns(uint256)
func (_LivepeerTokenFaucet *LivepeerTokenFaucetSession) RequestAmount() (*big.Int, error) {
	return _LivepeerTokenFaucet.Contract.RequestAmount(&_LivepeerTokenFaucet.CallOpts)
}

// RequestAmount is a free data retrieval call binding the contract method 0xf52ec46c.
//
// Solidity: function requestAmount() view returns(uint256)
func (_LivepeerTokenFaucet *LivepeerTokenFaucetCallerSession) RequestAmount() (*big.Int, error) {
	return _LivepeerTokenFaucet.Contract.RequestAmount(&_LivepeerTokenFaucet.CallOpts)
}

// RequestWait is a free data retrieval call binding the contract method 0x0d6c51b3.
//
// Solidity: function requestWait() view returns(uint256)
func (_LivepeerTokenFaucet *LivepeerTokenFaucetCaller) RequestWait(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _LivepeerTokenFaucet.contract.Call(opts, &out, "requestWait")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// RequestWait is a free data retrieval call binding the contract method 0x0d6c51b3.
//
// Solidity: function requestWait() view returns(uint256)
func (_LivepeerTokenFaucet *LivepeerTokenFaucetSession) RequestWait() (*big.Int, error) {
	return _LivepeerTokenFaucet.Contract.RequestWait(&_LivepeerTokenFaucet.CallOpts)
}

// RequestWait is a free data retrieval call binding the contract method 0x0d6c51b3.
//
// Solidity: function requestWait() view returns(uint256)
func (_LivepeerTokenFaucet *LivepeerTokenFaucetCallerSession) RequestWait() (*big.Int, error) {
	return _LivepeerTokenFaucet.Contract.RequestWait(&_LivepeerTokenFaucet.CallOpts)
}

// Token is a free data retrieval call binding the contract method 0xfc0c546a.
//
// Solidity: function token() view returns(address)
func (_LivepeerTokenFaucet *LivepeerTokenFaucetCaller) Token(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _LivepeerTokenFaucet.contract.Call(opts, &out, "token")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// Token is a free data retrieval call binding the contract method 0xfc0c546a.
//
// Solidity: function token() view returns(address)
func (_LivepeerTokenFaucet *LivepeerTokenFaucetSession) Token() (common.Address, error) {
	return _LivepeerTokenFaucet.Contract.Token(&_LivepeerTokenFaucet.CallOpts)
}

// Token is a free data retrieval call binding the contract method 0xfc0c546a.
//
// Solidity: function token() view returns(address)
func (_LivepeerTokenFaucet *LivepeerTokenFaucetCallerSession) Token() (common.Address, error) {
	return _LivepeerTokenFaucet.Contract.Token(&_LivepeerTokenFaucet.CallOpts)
}

// AddToWhitelist is a paid mutator transaction binding the contract method 0xe43252d7.
//
// Solidity: function addToWhitelist(address _addr) returns()
func (_LivepeerTokenFaucet *LivepeerTokenFaucetTransactor) AddToWhitelist(opts *bind.TransactOpts, _addr common.Address) (*types.Transaction, error) {
	return _LivepeerTokenFaucet.contract.Transact(opts, "addToWhitelist", _addr)
}

// AddToWhitelist is a paid mutator transaction binding the contract method 0xe43252d7.
//
// Solidity: function addToWhitelist(address _addr) returns()
func (_LivepeerTokenFaucet *LivepeerTokenFaucetSession) AddToWhitelist(_addr common.Address) (*types.Transaction, error) {
	return _LivepeerTokenFaucet.Contract.AddToWhitelist(&_LivepeerTokenFaucet.TransactOpts, _addr)
}

// AddToWhitelist is a paid mutator transaction binding the contract method 0xe43252d7.
//
// Solidity: function addToWhitelist(address _addr) returns()
func (_LivepeerTokenFaucet *LivepeerTokenFaucetTransactorSession) AddToWhitelist(_addr common.Address) (*types.Transaction, error) {
	return _LivepeerTokenFaucet.Contract.AddToWhitelist(&_LivepeerTokenFaucet.TransactOpts, _addr)
}

// RemoveFromWhitelist is a paid mutator transaction binding the contract method 0x8ab1d681.
//
// Solidity: function removeFromWhitelist(address _addr) returns()
func (_LivepeerTokenFaucet *LivepeerTokenFaucetTransactor) RemoveFromWhitelist(opts *bind.TransactOpts, _addr common.Address) (*types.Transaction, error) {
	return _LivepeerTokenFaucet.contract.Transact(opts, "removeFromWhitelist", _addr)
}

// RemoveFromWhitelist is a paid mutator transaction binding the contract method 0x8ab1d681.
//
// Solidity: function removeFromWhitelist(address _addr) returns()
func (_LivepeerTokenFaucet *LivepeerTokenFaucetSession) RemoveFromWhitelist(_addr common.Address) (*types.Transaction, error) {
	return _LivepeerTokenFaucet.Contract.RemoveFromWhitelist(&_LivepeerTokenFaucet.TransactOpts, _addr)
}

// RemoveFromWhitelist is a paid mutator transaction binding the contract method 0x8ab1d681.
//
// Solidity: function removeFromWhitelist(address _addr) returns()
func (_LivepeerTokenFaucet *LivepeerTokenFaucetTransactorSession) RemoveFromWhitelist(_addr common.Address) (*types.Transaction, error) {
	return _LivepeerTokenFaucet.Contract.RemoveFromWhitelist(&_LivepeerTokenFaucet.TransactOpts, _addr)
}

// Request is a paid mutator transaction binding the contract method 0x338cdca1.
//
// Solidity: function request() returns()
func (_LivepeerTokenFaucet *LivepeerTokenFaucetTransactor) Request(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _LivepeerTokenFaucet.contract.Transact(opts, "request")
}

// Request is a paid mutator transaction binding the contract method 0x338cdca1.
//
// Solidity: function request() returns()
func (_LivepeerTokenFaucet *LivepeerTokenFaucetSession) Request() (*types.Transaction, error) {
	return _LivepeerTokenFaucet.Contract.Request(&_LivepeerTokenFaucet.TransactOpts)
}

// Request is a paid mutator transaction binding the contract method 0x338cdca1.
//
// Solidity: function request() returns()
func (_LivepeerTokenFaucet *LivepeerTokenFaucetTransactorSession) Request() (*types.Transaction, error) {
	return _LivepeerTokenFaucet.Contract.Request(&_LivepeerTokenFaucet.TransactOpts)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address newOwner) returns()
func (_LivepeerTokenFaucet *LivepeerTokenFaucetTransactor) TransferOwnership(opts *bind.TransactOpts, newOwner common.Address) (*types.Transaction, error) {
	return _LivepeerTokenFaucet.contract.Transact(opts, "transferOwnership", newOwner)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address newOwner) returns()
func (_LivepeerTokenFaucet *LivepeerTokenFaucetSession) TransferOwnership(newOwner common.Address) (*types.Transaction, error) {
	return _LivepeerTokenFaucet.Contract.TransferOwnership(&_LivepeerTokenFaucet.TransactOpts, newOwner)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address newOwner) returns()
func (_LivepeerTokenFaucet *LivepeerTokenFaucetTransactorSession) TransferOwnership(newOwner common.Address) (*types.Transaction, error) {
	return _LivepeerTokenFaucet.Contract.TransferOwnership(&_LivepeerTokenFaucet.TransactOpts, newOwner)
}

// LivepeerTokenFaucetOwnershipTransferredIterator is returned from FilterOwnershipTransferred and is used to iterate over the raw logs and unpacked data for OwnershipTransferred events raised by the LivepeerTokenFaucet contract.
type LivepeerTokenFaucetOwnershipTransferredIterator struct {
	Event *LivepeerTokenFaucetOwnershipTransferred // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *LivepeerTokenFaucetOwnershipTransferredIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(LivepeerTokenFaucetOwnershipTransferred)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(LivepeerTokenFaucetOwnershipTransferred)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *LivepeerTokenFaucetOwnershipTransferredIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *LivepeerTokenFaucetOwnershipTransferredIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// LivepeerTokenFaucetOwnershipTransferred represents a OwnershipTransferred event raised by the LivepeerTokenFaucet contract.
type LivepeerTokenFaucetOwnershipTransferred struct {
	PreviousOwner common.Address
	NewOwner      common.Address
	Raw           types.Log // Blockchain specific contextual infos
}

// FilterOwnershipTransferred is a free log retrieval operation binding the contract event 0x8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e0.
//
// Solidity: event OwnershipTransferred(address indexed previousOwner, address indexed newOwner)
func (_LivepeerTokenFaucet *LivepeerTokenFaucetFilterer) FilterOwnershipTransferred(opts *bind.FilterOpts, previousOwner []common.Address, newOwner []common.Address) (*LivepeerTokenFaucetOwnershipTransferredIterator, error) {

	var previousOwnerRule []interface{}
	for _, previousOwnerItem := range previousOwner {
		previousOwnerRule = append(previousOwnerRule, previousOwnerItem)
	}
	var newOwnerRule []interface{}
	for _, newOwnerItem := range newOwner {
		newOwnerRule = append(newOwnerRule, newOwnerItem)
	}

	logs, sub, err := _LivepeerTokenFaucet.contract.FilterLogs(opts, "OwnershipTransferred", previousOwnerRule, newOwnerRule)
	if err != nil {
		return nil, err
	}
	return &LivepeerTokenFaucetOwnershipTransferredIterator{contract: _LivepeerTokenFaucet.contract, event: "OwnershipTransferred", logs: logs, sub: sub}, nil
}

// WatchOwnershipTransferred is a free log subscription operation binding the contract event 0x8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e0.
//
// Solidity: event OwnershipTransferred(address indexed previousOwner, address indexed newOwner)
func (_LivepeerTokenFaucet *LivepeerTokenFaucetFilterer) WatchOwnershipTransferred(opts *bind.WatchOpts, sink chan<- *LivepeerTokenFaucetOwnershipTransferred, previousOwner []common.Address, newOwner []common.Address) (event.Subscription, error) {

	var previousOwnerRule []interface{}
	for _, previousOwnerItem := range previousOwner {
		previousOwnerRule = append(previousOwnerRule, previousOwnerItem)
	}
	var newOwnerRule []interface{}
	for _, newOwnerItem := range newOwner {
		newOwnerRule = append(newOwnerRule, newOwnerItem)
	}

	logs, sub, err := _LivepeerTokenFaucet.contract.WatchLogs(opts, "OwnershipTransferred", previousOwnerRule, newOwnerRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(LivepeerTokenFaucetOwnershipTransferred)
				if err := _LivepeerTokenFaucet.contract.UnpackLog(event, "OwnershipTransferred", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseOwnershipTransferred is a log parse operation binding the contract event 0x8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e0.
//
// Solidity: event OwnershipTransferred(address indexed previousOwner, address indexed newOwner)
func (_LivepeerTokenFaucet *LivepeerTokenFaucetFilterer) ParseOwnershipTransferred(log types.Log) (*LivepeerTokenFaucetOwnershipTransferred, error) {
	event := new(LivepeerTokenFaucetOwnershipTransferred)
	if err := _LivepeerTokenFaucet.contract.UnpackLog(event, "OwnershipTransferred", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// LivepeerTokenFaucetRequestIterator is returned from FilterRequest and is used to iterate over the raw logs and unpacked data for Request events raised by the LivepeerTokenFaucet contract.
type LivepeerTokenFaucetRequestIterator struct {
	Event *LivepeerTokenFaucetRequest // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *LivepeerTokenFaucetRequestIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(LivepeerTokenFaucetRequest)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(LivepeerTokenFaucetRequest)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *LivepeerTokenFaucetRequestIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *LivepeerTokenFaucetRequestIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// LivepeerTokenFaucetRequest represents a Request event raised by the LivepeerTokenFaucet contract.
type LivepeerTokenFaucetRequest struct {
	To     common.Address
	Amount *big.Int
	Raw    types.Log // Blockchain specific contextual infos
}

// FilterRequest is a free log retrieval operation binding the contract event 0xe31c60e37ab1301f69f01b436a1d13486e6c16cc22c888a08c0e64a39230b6ac.
//
// Solidity: event Request(address indexed to, uint256 amount)
func (_LivepeerTokenFaucet *LivepeerTokenFaucetFilterer) FilterRequest(opts *bind.FilterOpts, to []common.Address) (*LivepeerTokenFaucetRequestIterator, error) {

	var toRule []interface{}
	for _, toItem := range to {
		toRule = append(toRule, toItem)
	}

	logs, sub, err := _LivepeerTokenFaucet.contract.FilterLogs(opts, "Request", toRule)
	if err != nil {
		return nil, err
	}
	return &LivepeerTokenFaucetRequestIterator{contract: _LivepeerTokenFaucet.contract, event: "Request", logs: logs, sub: sub}, nil
}

// WatchRequest is a free log subscription operation binding the contract event 0xe31c60e37ab1301f69f01b436a1d13486e6c16cc22c888a08c0e64a39230b6ac.
//
// Solidity: event Request(address indexed to, uint256 amount)
func (_LivepeerTokenFaucet *LivepeerTokenFaucetFilterer) WatchRequest(opts *bind.WatchOpts, sink chan<- *LivepeerTokenFaucetRequest, to []common.Address) (event.Subscription, error) {

	var toRule []interface{}
	for _, toItem := range to {
		toRule = append(toRule, toItem)
	}

	logs, sub, err := _LivepeerTokenFaucet.contract.WatchLogs(opts, "Request", toRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(LivepeerTokenFaucetRequest)
				if err := _LivepeerTokenFaucet.contract.UnpackLog(event, "Request", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseRequest is a log parse operation binding the contract event 0xe31c60e37ab1301f69f01b436a1d13486e6c16cc22c888a08c0e64a39230b6ac.
//
// Solidity: event Request(address indexed to, uint256 amount)
func (_LivepeerTokenFaucet *LivepeerTokenFaucetFilterer) ParseRequest(log types.Log) (*LivepeerTokenFaucetRequest, error) {
	event := new(LivepeerTokenFaucetRequest)
	if err := _LivepeerTokenFaucet.contract.UnpackLog(event, "Request", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}
