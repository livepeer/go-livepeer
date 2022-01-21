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

// ControllerMetaData contains all meta data concerning the Controller contract.
var ControllerMetaData = &bind.MetaData{
	ABI: "[{\"constant\":false,\"inputs\":[],\"name\":\"unpause\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"paused\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"_id\",\"type\":\"bytes32\"}],\"name\":\"getContractInfo\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"},{\"internalType\":\"bytes20\",\"name\":\"\",\"type\":\"bytes20\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[],\"name\":\"pause\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"owner\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"_id\",\"type\":\"bytes32\"},{\"internalType\":\"address\",\"name\":\"_contractAddress\",\"type\":\"address\"},{\"internalType\":\"bytes20\",\"name\":\"_gitCommitHash\",\"type\":\"bytes20\"}],\"name\":\"setContractInfo\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"_id\",\"type\":\"bytes32\"}],\"name\":\"getContract\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"_id\",\"type\":\"bytes32\"},{\"internalType\":\"address\",\"name\":\"_controller\",\"type\":\"address\"}],\"name\":\"updateController\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"internalType\":\"address\",\"name\":\"newOwner\",\"type\":\"address\"}],\"name\":\"transferOwnership\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"constructor\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"bytes32\",\"name\":\"id\",\"type\":\"bytes32\"},{\"indexed\":false,\"internalType\":\"address\",\"name\":\"contractAddress\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"bytes20\",\"name\":\"gitCommitHash\",\"type\":\"bytes20\"}],\"name\":\"SetContractInfo\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[],\"name\":\"Pause\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[],\"name\":\"Unpause\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"previousOwner\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"newOwner\",\"type\":\"address\"}],\"name\":\"OwnershipTransferred\",\"type\":\"event\"}]",
}

// ControllerABI is the input ABI used to generate the binding from.
// Deprecated: Use ControllerMetaData.ABI instead.
var ControllerABI = ControllerMetaData.ABI

// Controller is an auto generated Go binding around an Ethereum contract.
type Controller struct {
	ControllerCaller     // Read-only binding to the contract
	ControllerTransactor // Write-only binding to the contract
	ControllerFilterer   // Log filterer for contract events
}

// ControllerCaller is an auto generated read-only Go binding around an Ethereum contract.
type ControllerCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// ControllerTransactor is an auto generated write-only Go binding around an Ethereum contract.
type ControllerTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// ControllerFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type ControllerFilterer struct {
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
	contract, err := bindController(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &Controller{ControllerCaller: ControllerCaller{contract: contract}, ControllerTransactor: ControllerTransactor{contract: contract}, ControllerFilterer: ControllerFilterer{contract: contract}}, nil
}

// NewControllerCaller creates a new read-only instance of Controller, bound to a specific deployed contract.
func NewControllerCaller(address common.Address, caller bind.ContractCaller) (*ControllerCaller, error) {
	contract, err := bindController(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &ControllerCaller{contract: contract}, nil
}

// NewControllerTransactor creates a new write-only instance of Controller, bound to a specific deployed contract.
func NewControllerTransactor(address common.Address, transactor bind.ContractTransactor) (*ControllerTransactor, error) {
	contract, err := bindController(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &ControllerTransactor{contract: contract}, nil
}

// NewControllerFilterer creates a new log filterer instance of Controller, bound to a specific deployed contract.
func NewControllerFilterer(address common.Address, filterer bind.ContractFilterer) (*ControllerFilterer, error) {
	contract, err := bindController(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &ControllerFilterer{contract: contract}, nil
}

// bindController binds a generic wrapper to an already deployed contract.
func bindController(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := abi.JSON(strings.NewReader(ControllerABI))
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_Controller *ControllerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
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
func (_Controller *ControllerCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
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
// Solidity: function getContract(bytes32 _id) view returns(address)
func (_Controller *ControllerCaller) GetContract(opts *bind.CallOpts, _id [32]byte) (common.Address, error) {
	var out []interface{}
	err := _Controller.contract.Call(opts, &out, "getContract", _id)

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// GetContract is a free data retrieval call binding the contract method 0xe16c7d98.
//
// Solidity: function getContract(bytes32 _id) view returns(address)
func (_Controller *ControllerSession) GetContract(_id [32]byte) (common.Address, error) {
	return _Controller.Contract.GetContract(&_Controller.CallOpts, _id)
}

// GetContract is a free data retrieval call binding the contract method 0xe16c7d98.
//
// Solidity: function getContract(bytes32 _id) view returns(address)
func (_Controller *ControllerCallerSession) GetContract(_id [32]byte) (common.Address, error) {
	return _Controller.Contract.GetContract(&_Controller.CallOpts, _id)
}

// GetContractInfo is a free data retrieval call binding the contract method 0x613e2de2.
//
// Solidity: function getContractInfo(bytes32 _id) view returns(address, bytes20)
func (_Controller *ControllerCaller) GetContractInfo(opts *bind.CallOpts, _id [32]byte) (common.Address, [20]byte, error) {
	var out []interface{}
	err := _Controller.contract.Call(opts, &out, "getContractInfo", _id)

	if err != nil {
		return *new(common.Address), *new([20]byte), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)
	out1 := *abi.ConvertType(out[1], new([20]byte)).(*[20]byte)

	return out0, out1, err

}

// GetContractInfo is a free data retrieval call binding the contract method 0x613e2de2.
//
// Solidity: function getContractInfo(bytes32 _id) view returns(address, bytes20)
func (_Controller *ControllerSession) GetContractInfo(_id [32]byte) (common.Address, [20]byte, error) {
	return _Controller.Contract.GetContractInfo(&_Controller.CallOpts, _id)
}

// GetContractInfo is a free data retrieval call binding the contract method 0x613e2de2.
//
// Solidity: function getContractInfo(bytes32 _id) view returns(address, bytes20)
func (_Controller *ControllerCallerSession) GetContractInfo(_id [32]byte) (common.Address, [20]byte, error) {
	return _Controller.Contract.GetContractInfo(&_Controller.CallOpts, _id)
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_Controller *ControllerCaller) Owner(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _Controller.contract.Call(opts, &out, "owner")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_Controller *ControllerSession) Owner() (common.Address, error) {
	return _Controller.Contract.Owner(&_Controller.CallOpts)
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_Controller *ControllerCallerSession) Owner() (common.Address, error) {
	return _Controller.Contract.Owner(&_Controller.CallOpts)
}

// Paused is a free data retrieval call binding the contract method 0x5c975abb.
//
// Solidity: function paused() view returns(bool)
func (_Controller *ControllerCaller) Paused(opts *bind.CallOpts) (bool, error) {
	var out []interface{}
	err := _Controller.contract.Call(opts, &out, "paused")

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// Paused is a free data retrieval call binding the contract method 0x5c975abb.
//
// Solidity: function paused() view returns(bool)
func (_Controller *ControllerSession) Paused() (bool, error) {
	return _Controller.Contract.Paused(&_Controller.CallOpts)
}

// Paused is a free data retrieval call binding the contract method 0x5c975abb.
//
// Solidity: function paused() view returns(bool)
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
// Solidity: function setContractInfo(bytes32 _id, address _contractAddress, bytes20 _gitCommitHash) returns()
func (_Controller *ControllerTransactor) SetContractInfo(opts *bind.TransactOpts, _id [32]byte, _contractAddress common.Address, _gitCommitHash [20]byte) (*types.Transaction, error) {
	return _Controller.contract.Transact(opts, "setContractInfo", _id, _contractAddress, _gitCommitHash)
}

// SetContractInfo is a paid mutator transaction binding the contract method 0xd737c2b0.
//
// Solidity: function setContractInfo(bytes32 _id, address _contractAddress, bytes20 _gitCommitHash) returns()
func (_Controller *ControllerSession) SetContractInfo(_id [32]byte, _contractAddress common.Address, _gitCommitHash [20]byte) (*types.Transaction, error) {
	return _Controller.Contract.SetContractInfo(&_Controller.TransactOpts, _id, _contractAddress, _gitCommitHash)
}

// SetContractInfo is a paid mutator transaction binding the contract method 0xd737c2b0.
//
// Solidity: function setContractInfo(bytes32 _id, address _contractAddress, bytes20 _gitCommitHash) returns()
func (_Controller *ControllerTransactorSession) SetContractInfo(_id [32]byte, _contractAddress common.Address, _gitCommitHash [20]byte) (*types.Transaction, error) {
	return _Controller.Contract.SetContractInfo(&_Controller.TransactOpts, _id, _contractAddress, _gitCommitHash)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address newOwner) returns()
func (_Controller *ControllerTransactor) TransferOwnership(opts *bind.TransactOpts, newOwner common.Address) (*types.Transaction, error) {
	return _Controller.contract.Transact(opts, "transferOwnership", newOwner)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address newOwner) returns()
func (_Controller *ControllerSession) TransferOwnership(newOwner common.Address) (*types.Transaction, error) {
	return _Controller.Contract.TransferOwnership(&_Controller.TransactOpts, newOwner)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address newOwner) returns()
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
// Solidity: function updateController(bytes32 _id, address _controller) returns()
func (_Controller *ControllerTransactor) UpdateController(opts *bind.TransactOpts, _id [32]byte, _controller common.Address) (*types.Transaction, error) {
	return _Controller.contract.Transact(opts, "updateController", _id, _controller)
}

// UpdateController is a paid mutator transaction binding the contract method 0xeb5dd94f.
//
// Solidity: function updateController(bytes32 _id, address _controller) returns()
func (_Controller *ControllerSession) UpdateController(_id [32]byte, _controller common.Address) (*types.Transaction, error) {
	return _Controller.Contract.UpdateController(&_Controller.TransactOpts, _id, _controller)
}

// UpdateController is a paid mutator transaction binding the contract method 0xeb5dd94f.
//
// Solidity: function updateController(bytes32 _id, address _controller) returns()
func (_Controller *ControllerTransactorSession) UpdateController(_id [32]byte, _controller common.Address) (*types.Transaction, error) {
	return _Controller.Contract.UpdateController(&_Controller.TransactOpts, _id, _controller)
}

// ControllerOwnershipTransferredIterator is returned from FilterOwnershipTransferred and is used to iterate over the raw logs and unpacked data for OwnershipTransferred events raised by the Controller contract.
type ControllerOwnershipTransferredIterator struct {
	Event *ControllerOwnershipTransferred // Event containing the contract specifics and raw log

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
func (it *ControllerOwnershipTransferredIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(ControllerOwnershipTransferred)
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
		it.Event = new(ControllerOwnershipTransferred)
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
func (it *ControllerOwnershipTransferredIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *ControllerOwnershipTransferredIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// ControllerOwnershipTransferred represents a OwnershipTransferred event raised by the Controller contract.
type ControllerOwnershipTransferred struct {
	PreviousOwner common.Address
	NewOwner      common.Address
	Raw           types.Log // Blockchain specific contextual infos
}

// FilterOwnershipTransferred is a free log retrieval operation binding the contract event 0x8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e0.
//
// Solidity: event OwnershipTransferred(address indexed previousOwner, address indexed newOwner)
func (_Controller *ControllerFilterer) FilterOwnershipTransferred(opts *bind.FilterOpts, previousOwner []common.Address, newOwner []common.Address) (*ControllerOwnershipTransferredIterator, error) {

	var previousOwnerRule []interface{}
	for _, previousOwnerItem := range previousOwner {
		previousOwnerRule = append(previousOwnerRule, previousOwnerItem)
	}
	var newOwnerRule []interface{}
	for _, newOwnerItem := range newOwner {
		newOwnerRule = append(newOwnerRule, newOwnerItem)
	}

	logs, sub, err := _Controller.contract.FilterLogs(opts, "OwnershipTransferred", previousOwnerRule, newOwnerRule)
	if err != nil {
		return nil, err
	}
	return &ControllerOwnershipTransferredIterator{contract: _Controller.contract, event: "OwnershipTransferred", logs: logs, sub: sub}, nil
}

// WatchOwnershipTransferred is a free log subscription operation binding the contract event 0x8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e0.
//
// Solidity: event OwnershipTransferred(address indexed previousOwner, address indexed newOwner)
func (_Controller *ControllerFilterer) WatchOwnershipTransferred(opts *bind.WatchOpts, sink chan<- *ControllerOwnershipTransferred, previousOwner []common.Address, newOwner []common.Address) (event.Subscription, error) {

	var previousOwnerRule []interface{}
	for _, previousOwnerItem := range previousOwner {
		previousOwnerRule = append(previousOwnerRule, previousOwnerItem)
	}
	var newOwnerRule []interface{}
	for _, newOwnerItem := range newOwner {
		newOwnerRule = append(newOwnerRule, newOwnerItem)
	}

	logs, sub, err := _Controller.contract.WatchLogs(opts, "OwnershipTransferred", previousOwnerRule, newOwnerRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(ControllerOwnershipTransferred)
				if err := _Controller.contract.UnpackLog(event, "OwnershipTransferred", log); err != nil {
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
func (_Controller *ControllerFilterer) ParseOwnershipTransferred(log types.Log) (*ControllerOwnershipTransferred, error) {
	event := new(ControllerOwnershipTransferred)
	if err := _Controller.contract.UnpackLog(event, "OwnershipTransferred", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// ControllerPauseIterator is returned from FilterPause and is used to iterate over the raw logs and unpacked data for Pause events raised by the Controller contract.
type ControllerPauseIterator struct {
	Event *ControllerPause // Event containing the contract specifics and raw log

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
func (it *ControllerPauseIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(ControllerPause)
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
		it.Event = new(ControllerPause)
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
func (it *ControllerPauseIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *ControllerPauseIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// ControllerPause represents a Pause event raised by the Controller contract.
type ControllerPause struct {
	Raw types.Log // Blockchain specific contextual infos
}

// FilterPause is a free log retrieval operation binding the contract event 0x6985a02210a168e66602d3235cb6db0e70f92b3ba4d376a33c0f3d9434bff625.
//
// Solidity: event Pause()
func (_Controller *ControllerFilterer) FilterPause(opts *bind.FilterOpts) (*ControllerPauseIterator, error) {

	logs, sub, err := _Controller.contract.FilterLogs(opts, "Pause")
	if err != nil {
		return nil, err
	}
	return &ControllerPauseIterator{contract: _Controller.contract, event: "Pause", logs: logs, sub: sub}, nil
}

// WatchPause is a free log subscription operation binding the contract event 0x6985a02210a168e66602d3235cb6db0e70f92b3ba4d376a33c0f3d9434bff625.
//
// Solidity: event Pause()
func (_Controller *ControllerFilterer) WatchPause(opts *bind.WatchOpts, sink chan<- *ControllerPause) (event.Subscription, error) {

	logs, sub, err := _Controller.contract.WatchLogs(opts, "Pause")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(ControllerPause)
				if err := _Controller.contract.UnpackLog(event, "Pause", log); err != nil {
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

// ParsePause is a log parse operation binding the contract event 0x6985a02210a168e66602d3235cb6db0e70f92b3ba4d376a33c0f3d9434bff625.
//
// Solidity: event Pause()
func (_Controller *ControllerFilterer) ParsePause(log types.Log) (*ControllerPause, error) {
	event := new(ControllerPause)
	if err := _Controller.contract.UnpackLog(event, "Pause", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// ControllerSetContractInfoIterator is returned from FilterSetContractInfo and is used to iterate over the raw logs and unpacked data for SetContractInfo events raised by the Controller contract.
type ControllerSetContractInfoIterator struct {
	Event *ControllerSetContractInfo // Event containing the contract specifics and raw log

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
func (it *ControllerSetContractInfoIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(ControllerSetContractInfo)
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
		it.Event = new(ControllerSetContractInfo)
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
func (it *ControllerSetContractInfoIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *ControllerSetContractInfoIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// ControllerSetContractInfo represents a SetContractInfo event raised by the Controller contract.
type ControllerSetContractInfo struct {
	Id              [32]byte
	ContractAddress common.Address
	GitCommitHash   [20]byte
	Raw             types.Log // Blockchain specific contextual infos
}

// FilterSetContractInfo is a free log retrieval operation binding the contract event 0xf9a6cf519167d81bc5cb3d26c60c0c9a19704aa908c148e82a861b570f4cd2d7.
//
// Solidity: event SetContractInfo(bytes32 id, address contractAddress, bytes20 gitCommitHash)
func (_Controller *ControllerFilterer) FilterSetContractInfo(opts *bind.FilterOpts) (*ControllerSetContractInfoIterator, error) {

	logs, sub, err := _Controller.contract.FilterLogs(opts, "SetContractInfo")
	if err != nil {
		return nil, err
	}
	return &ControllerSetContractInfoIterator{contract: _Controller.contract, event: "SetContractInfo", logs: logs, sub: sub}, nil
}

// WatchSetContractInfo is a free log subscription operation binding the contract event 0xf9a6cf519167d81bc5cb3d26c60c0c9a19704aa908c148e82a861b570f4cd2d7.
//
// Solidity: event SetContractInfo(bytes32 id, address contractAddress, bytes20 gitCommitHash)
func (_Controller *ControllerFilterer) WatchSetContractInfo(opts *bind.WatchOpts, sink chan<- *ControllerSetContractInfo) (event.Subscription, error) {

	logs, sub, err := _Controller.contract.WatchLogs(opts, "SetContractInfo")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(ControllerSetContractInfo)
				if err := _Controller.contract.UnpackLog(event, "SetContractInfo", log); err != nil {
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

// ParseSetContractInfo is a log parse operation binding the contract event 0xf9a6cf519167d81bc5cb3d26c60c0c9a19704aa908c148e82a861b570f4cd2d7.
//
// Solidity: event SetContractInfo(bytes32 id, address contractAddress, bytes20 gitCommitHash)
func (_Controller *ControllerFilterer) ParseSetContractInfo(log types.Log) (*ControllerSetContractInfo, error) {
	event := new(ControllerSetContractInfo)
	if err := _Controller.contract.UnpackLog(event, "SetContractInfo", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// ControllerUnpauseIterator is returned from FilterUnpause and is used to iterate over the raw logs and unpacked data for Unpause events raised by the Controller contract.
type ControllerUnpauseIterator struct {
	Event *ControllerUnpause // Event containing the contract specifics and raw log

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
func (it *ControllerUnpauseIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(ControllerUnpause)
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
		it.Event = new(ControllerUnpause)
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
func (it *ControllerUnpauseIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *ControllerUnpauseIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// ControllerUnpause represents a Unpause event raised by the Controller contract.
type ControllerUnpause struct {
	Raw types.Log // Blockchain specific contextual infos
}

// FilterUnpause is a free log retrieval operation binding the contract event 0x7805862f689e2f13df9f062ff482ad3ad112aca9e0847911ed832e158c525b33.
//
// Solidity: event Unpause()
func (_Controller *ControllerFilterer) FilterUnpause(opts *bind.FilterOpts) (*ControllerUnpauseIterator, error) {

	logs, sub, err := _Controller.contract.FilterLogs(opts, "Unpause")
	if err != nil {
		return nil, err
	}
	return &ControllerUnpauseIterator{contract: _Controller.contract, event: "Unpause", logs: logs, sub: sub}, nil
}

// WatchUnpause is a free log subscription operation binding the contract event 0x7805862f689e2f13df9f062ff482ad3ad112aca9e0847911ed832e158c525b33.
//
// Solidity: event Unpause()
func (_Controller *ControllerFilterer) WatchUnpause(opts *bind.WatchOpts, sink chan<- *ControllerUnpause) (event.Subscription, error) {

	logs, sub, err := _Controller.contract.WatchLogs(opts, "Unpause")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(ControllerUnpause)
				if err := _Controller.contract.UnpackLog(event, "Unpause", log); err != nil {
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

// ParseUnpause is a log parse operation binding the contract event 0x7805862f689e2f13df9f062ff482ad3ad112aca9e0847911ed832e158c525b33.
//
// Solidity: event Unpause()
func (_Controller *ControllerFilterer) ParseUnpause(log types.Log) (*ControllerUnpause, error) {
	event := new(ControllerUnpause)
	if err := _Controller.contract.UnpackLog(event, "Unpause", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}
