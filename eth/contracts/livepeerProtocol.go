// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package contracts

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

// LivepeerProtocolABI is the input ABI used to generate the binding from.
const LivepeerProtocolABI = "[{\"constant\":false,\"inputs\":[],\"name\":\"unpause\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"paused\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_key\",\"type\":\"bytes32\"},{\"name\":\"_registry\",\"type\":\"address\"}],\"name\":\"updateManagerRegistry\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_key\",\"type\":\"bytes32\"},{\"name\":\"_contract\",\"type\":\"address\"}],\"name\":\"setContract\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"\",\"type\":\"bytes32\"}],\"name\":\"registry\",\"outputs\":[{\"name\":\"\",\"type\":\"address\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":false,\"inputs\":[],\"name\":\"pause\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"owner\",\"outputs\":[{\"name\":\"\",\"type\":\"address\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"newOwner\",\"type\":\"address\"}],\"name\":\"transferOwnership\",\"outputs\":[],\"payable\":false,\"type\":\"function\"},{\"inputs\":[],\"payable\":false,\"type\":\"constructor\"},{\"anonymous\":false,\"inputs\":[],\"name\":\"Pause\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[],\"name\":\"Unpause\",\"type\":\"event\"}]"

// LivepeerProtocolBin is the compiled bytecode used for deploying new contracts.
const LivepeerProtocolBin = `0x60606040526000805460a060020a60ff0219169055341561001f57600080fd5b5b5b5b60008054600160a060020a03191633600160a060020a03161790555b6100536401000000006103f261005b82021704565b505b5b6100fb565b6000805433600160a060020a0390811691161461007757600080fd5b60005474010000000000000000000000000000000000000000900460ff161561009f57600080fd5b6000805460a060020a60ff021916740100000000000000000000000000000000000000001790557f6985a02210a168e66602d3235cb6db0e70f92b3ba4d376a33c0f3d9434bff62560405160405180910390a15060015b5b5b90565b6105118061010a6000396000f3006060604052361561008b5763ffffffff7c01000000000000000000000000000000000000000000000000000000006000350416633f4ba83a81146100905780635c975abb146100b75780636627c2bd146100de5780637ed77c9c146101145780637ef502981461014a5780638456cb591461017c5780638da5cb5b146101a3578063f2fde38b146101d2575b600080fd5b341561009b57600080fd5b6100a36101f3565b604051901515815260200160405180910390f35b34156100c257600080fd5b6100a361027a565b604051901515815260200160405180910390f35b34156100e957600080fd5b6100a3600435600160a060020a036024351661028a565b604051901515815260200160405180910390f35b341561011f57600080fd5b6100a3600435600160a060020a0360243516610362565b604051901515815260200160405180910390f35b341561015557600080fd5b6101606004356103d7565b604051600160a060020a03909116815260200160405180910390f35b341561018757600080fd5b6100a36103f2565b604051901515815260200160405180910390f35b34156101ae57600080fd5b61016061047e565b604051600160a060020a03909116815260200160405180910390f35b34156101dd57600080fd5b6101f1600160a060020a036004351661048d565b005b6000805433600160a060020a0390811691161461020f57600080fd5b60005460a060020a900460ff16151561022757600080fd5b6000805474ff0000000000000000000000000000000000000000191690557f7805862f689e2f13df9f062ff482ad3ad112aca9e0847911ed832e158c525b3360405160405180910390a15060015b5b5b90565b60005460a060020a900460ff1681565b6000805433600160a060020a039081169116146102a657600080fd5b60005460a060020a900460ff1615156102be57600080fd5b60008381526001602052604080822054600160a060020a03169163a91ee0dc9185919051602001526040517c010000000000000000000000000000000000000000000000000000000063ffffffff8416028152600160a060020a039091166004820152602401602060405180830381600087803b151561033d57600080fd5b6102c65a03f1151561034e57600080fd5b50505060405180519150505b5b5b92915050565b6000805433600160a060020a0390811691161461037e57600080fd5b60005460a060020a900460ff16151561039657600080fd5b506000828152600160208190526040909120805473ffffffffffffffffffffffffffffffffffffffff1916600160a060020a0384161790555b5b5b92915050565b600160205260009081526040902054600160a060020a031681565b6000805433600160a060020a0390811691161461040e57600080fd5b60005460a060020a900460ff161561042557600080fd5b6000805474ff0000000000000000000000000000000000000000191660a060020a1790557f6985a02210a168e66602d3235cb6db0e70f92b3ba4d376a33c0f3d9434bff62560405160405180910390a15060015b5b5b90565b600054600160a060020a031681565b60005433600160a060020a039081169116146104a857600080fd5b600160a060020a038116156104e0576000805473ffffffffffffffffffffffffffffffffffffffff1916600160a060020a0383161790555b5b5b505600a165627a7a72305820b87a1b064cd072758e5f5ff80e4b3e7737105adb86860908dbeb54fd21c5903a0029`

// DeployLivepeerProtocol deploys a new Ethereum contract, binding an instance of LivepeerProtocol to it.
func DeployLivepeerProtocol(auth *bind.TransactOpts, backend bind.ContractBackend, libraries map[string]common.Address) (common.Address, *types.Transaction, *LivepeerProtocol, error) {
	parsed, err := abi.JSON(strings.NewReader(LivepeerProtocolABI))
	if err != nil {
		return common.Address{}, nil, nil, err
	}

	linkedBin := LivepeerProtocolBin
	for lib, addr := range libraries {
		reg, err := regexp.Compile(fmt.Sprintf("_+%s_+", lib))
		if err != nil {
			return common.Address{}, nil, nil, err
		}

		linkedBin = reg.ReplaceAllString(linkedBin, addr.Hex())
	}

	address, tx, contract, err := bind.DeployContract(auth, parsed, common.FromHex(linkedBin), backend)
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	return address, tx, &LivepeerProtocol{LivepeerProtocolCaller: LivepeerProtocolCaller{contract: contract}, LivepeerProtocolTransactor: LivepeerProtocolTransactor{contract: contract}}, nil
}

// LivepeerProtocol is an auto generated Go binding around an Ethereum contract.
type LivepeerProtocol struct {
	LivepeerProtocolCaller     // Read-only binding to the contract
	LivepeerProtocolTransactor // Write-only binding to the contract
}

// LivepeerProtocolCaller is an auto generated read-only Go binding around an Ethereum contract.
type LivepeerProtocolCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// LivepeerProtocolTransactor is an auto generated write-only Go binding around an Ethereum contract.
type LivepeerProtocolTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// LivepeerProtocolSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type LivepeerProtocolSession struct {
	Contract     *LivepeerProtocol // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// LivepeerProtocolCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type LivepeerProtocolCallerSession struct {
	Contract *LivepeerProtocolCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts           // Call options to use throughout this session
}

// LivepeerProtocolTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type LivepeerProtocolTransactorSession struct {
	Contract     *LivepeerProtocolTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts           // Transaction auth options to use throughout this session
}

// LivepeerProtocolRaw is an auto generated low-level Go binding around an Ethereum contract.
type LivepeerProtocolRaw struct {
	Contract *LivepeerProtocol // Generic contract binding to access the raw methods on
}

// LivepeerProtocolCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type LivepeerProtocolCallerRaw struct {
	Contract *LivepeerProtocolCaller // Generic read-only contract binding to access the raw methods on
}

// LivepeerProtocolTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type LivepeerProtocolTransactorRaw struct {
	Contract *LivepeerProtocolTransactor // Generic write-only contract binding to access the raw methods on
}

// NewLivepeerProtocol creates a new instance of LivepeerProtocol, bound to a specific deployed contract.
func NewLivepeerProtocol(address common.Address, backend bind.ContractBackend) (*LivepeerProtocol, error) {
	contract, err := bindLivepeerProtocol(address, backend, backend)
	if err != nil {
		return nil, err
	}
	return &LivepeerProtocol{LivepeerProtocolCaller: LivepeerProtocolCaller{contract: contract}, LivepeerProtocolTransactor: LivepeerProtocolTransactor{contract: contract}}, nil
}

// NewLivepeerProtocolCaller creates a new read-only instance of LivepeerProtocol, bound to a specific deployed contract.
func NewLivepeerProtocolCaller(address common.Address, caller bind.ContractCaller) (*LivepeerProtocolCaller, error) {
	contract, err := bindLivepeerProtocol(address, caller, nil)
	if err != nil {
		return nil, err
	}
	return &LivepeerProtocolCaller{contract: contract}, nil
}

// NewLivepeerProtocolTransactor creates a new write-only instance of LivepeerProtocol, bound to a specific deployed contract.
func NewLivepeerProtocolTransactor(address common.Address, transactor bind.ContractTransactor) (*LivepeerProtocolTransactor, error) {
	contract, err := bindLivepeerProtocol(address, nil, transactor)
	if err != nil {
		return nil, err
	}
	return &LivepeerProtocolTransactor{contract: contract}, nil
}

// bindLivepeerProtocol binds a generic wrapper to an already deployed contract.
func bindLivepeerProtocol(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor) (*bind.BoundContract, error) {
	parsed, err := abi.JSON(strings.NewReader(LivepeerProtocolABI))
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, parsed, caller, transactor), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_LivepeerProtocol *LivepeerProtocolRaw) Call(opts *bind.CallOpts, result interface{}, method string, params ...interface{}) error {
	return _LivepeerProtocol.Contract.LivepeerProtocolCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_LivepeerProtocol *LivepeerProtocolRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _LivepeerProtocol.Contract.LivepeerProtocolTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_LivepeerProtocol *LivepeerProtocolRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _LivepeerProtocol.Contract.LivepeerProtocolTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_LivepeerProtocol *LivepeerProtocolCallerRaw) Call(opts *bind.CallOpts, result interface{}, method string, params ...interface{}) error {
	return _LivepeerProtocol.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_LivepeerProtocol *LivepeerProtocolTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _LivepeerProtocol.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_LivepeerProtocol *LivepeerProtocolTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _LivepeerProtocol.Contract.contract.Transact(opts, method, params...)
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() constant returns(address)
func (_LivepeerProtocol *LivepeerProtocolCaller) Owner(opts *bind.CallOpts) (common.Address, error) {
	var (
		ret0 = new(common.Address)
	)
	out := ret0
	err := _LivepeerProtocol.contract.Call(opts, out, "owner")
	return *ret0, err
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() constant returns(address)
func (_LivepeerProtocol *LivepeerProtocolSession) Owner() (common.Address, error) {
	return _LivepeerProtocol.Contract.Owner(&_LivepeerProtocol.CallOpts)
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() constant returns(address)
func (_LivepeerProtocol *LivepeerProtocolCallerSession) Owner() (common.Address, error) {
	return _LivepeerProtocol.Contract.Owner(&_LivepeerProtocol.CallOpts)
}

// Paused is a free data retrieval call binding the contract method 0x5c975abb.
//
// Solidity: function paused() constant returns(bool)
func (_LivepeerProtocol *LivepeerProtocolCaller) Paused(opts *bind.CallOpts) (bool, error) {
	var (
		ret0 = new(bool)
	)
	out := ret0
	err := _LivepeerProtocol.contract.Call(opts, out, "paused")
	return *ret0, err
}

// Paused is a free data retrieval call binding the contract method 0x5c975abb.
//
// Solidity: function paused() constant returns(bool)
func (_LivepeerProtocol *LivepeerProtocolSession) Paused() (bool, error) {
	return _LivepeerProtocol.Contract.Paused(&_LivepeerProtocol.CallOpts)
}

// Paused is a free data retrieval call binding the contract method 0x5c975abb.
//
// Solidity: function paused() constant returns(bool)
func (_LivepeerProtocol *LivepeerProtocolCallerSession) Paused() (bool, error) {
	return _LivepeerProtocol.Contract.Paused(&_LivepeerProtocol.CallOpts)
}

// Registry is a free data retrieval call binding the contract method 0x7ef50298.
//
// Solidity: function registry( bytes32) constant returns(address)
func (_LivepeerProtocol *LivepeerProtocolCaller) Registry(opts *bind.CallOpts, arg0 [32]byte) (common.Address, error) {
	var (
		ret0 = new(common.Address)
	)
	out := ret0
	err := _LivepeerProtocol.contract.Call(opts, out, "registry", arg0)
	return *ret0, err
}

// Registry is a free data retrieval call binding the contract method 0x7ef50298.
//
// Solidity: function registry( bytes32) constant returns(address)
func (_LivepeerProtocol *LivepeerProtocolSession) Registry(arg0 [32]byte) (common.Address, error) {
	return _LivepeerProtocol.Contract.Registry(&_LivepeerProtocol.CallOpts, arg0)
}

// Registry is a free data retrieval call binding the contract method 0x7ef50298.
//
// Solidity: function registry( bytes32) constant returns(address)
func (_LivepeerProtocol *LivepeerProtocolCallerSession) Registry(arg0 [32]byte) (common.Address, error) {
	return _LivepeerProtocol.Contract.Registry(&_LivepeerProtocol.CallOpts, arg0)
}

// Pause is a paid mutator transaction binding the contract method 0x8456cb59.
//
// Solidity: function pause() returns(bool)
func (_LivepeerProtocol *LivepeerProtocolTransactor) Pause(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _LivepeerProtocol.contract.Transact(opts, "pause")
}

// Pause is a paid mutator transaction binding the contract method 0x8456cb59.
//
// Solidity: function pause() returns(bool)
func (_LivepeerProtocol *LivepeerProtocolSession) Pause() (*types.Transaction, error) {
	return _LivepeerProtocol.Contract.Pause(&_LivepeerProtocol.TransactOpts)
}

// Pause is a paid mutator transaction binding the contract method 0x8456cb59.
//
// Solidity: function pause() returns(bool)
func (_LivepeerProtocol *LivepeerProtocolTransactorSession) Pause() (*types.Transaction, error) {
	return _LivepeerProtocol.Contract.Pause(&_LivepeerProtocol.TransactOpts)
}

// SetContract is a paid mutator transaction binding the contract method 0x7ed77c9c.
//
// Solidity: function setContract(_key bytes32, _contract address) returns(bool)
func (_LivepeerProtocol *LivepeerProtocolTransactor) SetContract(opts *bind.TransactOpts, _key [32]byte, _contract common.Address) (*types.Transaction, error) {
	return _LivepeerProtocol.contract.Transact(opts, "setContract", _key, _contract)
}

// SetContract is a paid mutator transaction binding the contract method 0x7ed77c9c.
//
// Solidity: function setContract(_key bytes32, _contract address) returns(bool)
func (_LivepeerProtocol *LivepeerProtocolSession) SetContract(_key [32]byte, _contract common.Address) (*types.Transaction, error) {
	return _LivepeerProtocol.Contract.SetContract(&_LivepeerProtocol.TransactOpts, _key, _contract)
}

// SetContract is a paid mutator transaction binding the contract method 0x7ed77c9c.
//
// Solidity: function setContract(_key bytes32, _contract address) returns(bool)
func (_LivepeerProtocol *LivepeerProtocolTransactorSession) SetContract(_key [32]byte, _contract common.Address) (*types.Transaction, error) {
	return _LivepeerProtocol.Contract.SetContract(&_LivepeerProtocol.TransactOpts, _key, _contract)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(newOwner address) returns()
func (_LivepeerProtocol *LivepeerProtocolTransactor) TransferOwnership(opts *bind.TransactOpts, newOwner common.Address) (*types.Transaction, error) {
	return _LivepeerProtocol.contract.Transact(opts, "transferOwnership", newOwner)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(newOwner address) returns()
func (_LivepeerProtocol *LivepeerProtocolSession) TransferOwnership(newOwner common.Address) (*types.Transaction, error) {
	return _LivepeerProtocol.Contract.TransferOwnership(&_LivepeerProtocol.TransactOpts, newOwner)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(newOwner address) returns()
func (_LivepeerProtocol *LivepeerProtocolTransactorSession) TransferOwnership(newOwner common.Address) (*types.Transaction, error) {
	return _LivepeerProtocol.Contract.TransferOwnership(&_LivepeerProtocol.TransactOpts, newOwner)
}

// Unpause is a paid mutator transaction binding the contract method 0x3f4ba83a.
//
// Solidity: function unpause() returns(bool)
func (_LivepeerProtocol *LivepeerProtocolTransactor) Unpause(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _LivepeerProtocol.contract.Transact(opts, "unpause")
}

// Unpause is a paid mutator transaction binding the contract method 0x3f4ba83a.
//
// Solidity: function unpause() returns(bool)
func (_LivepeerProtocol *LivepeerProtocolSession) Unpause() (*types.Transaction, error) {
	return _LivepeerProtocol.Contract.Unpause(&_LivepeerProtocol.TransactOpts)
}

// Unpause is a paid mutator transaction binding the contract method 0x3f4ba83a.
//
// Solidity: function unpause() returns(bool)
func (_LivepeerProtocol *LivepeerProtocolTransactorSession) Unpause() (*types.Transaction, error) {
	return _LivepeerProtocol.Contract.Unpause(&_LivepeerProtocol.TransactOpts)
}

// UpdateManagerRegistry is a paid mutator transaction binding the contract method 0x6627c2bd.
//
// Solidity: function updateManagerRegistry(_key bytes32, _registry address) returns(bool)
func (_LivepeerProtocol *LivepeerProtocolTransactor) UpdateManagerRegistry(opts *bind.TransactOpts, _key [32]byte, _registry common.Address) (*types.Transaction, error) {
	return _LivepeerProtocol.contract.Transact(opts, "updateManagerRegistry", _key, _registry)
}

// UpdateManagerRegistry is a paid mutator transaction binding the contract method 0x6627c2bd.
//
// Solidity: function updateManagerRegistry(_key bytes32, _registry address) returns(bool)
func (_LivepeerProtocol *LivepeerProtocolSession) UpdateManagerRegistry(_key [32]byte, _registry common.Address) (*types.Transaction, error) {
	return _LivepeerProtocol.Contract.UpdateManagerRegistry(&_LivepeerProtocol.TransactOpts, _key, _registry)
}

// UpdateManagerRegistry is a paid mutator transaction binding the contract method 0x6627c2bd.
//
// Solidity: function updateManagerRegistry(_key bytes32, _registry address) returns(bool)
func (_LivepeerProtocol *LivepeerProtocolTransactorSession) UpdateManagerRegistry(_key [32]byte, _registry common.Address) (*types.Transaction, error) {
	return _LivepeerProtocol.Contract.UpdateManagerRegistry(&_LivepeerProtocol.TransactOpts, _key, _registry)
}
