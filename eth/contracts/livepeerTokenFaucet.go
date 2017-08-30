// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package contracts

import (
	"fmt"
	"math/big"
	"regexp"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

// LivepeerTokenFaucetABI is the input ABI used to generate the binding from.
const LivepeerTokenFaucetABI = "[{\"constant\":true,\"inputs\":[],\"name\":\"requestWait\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"\",\"type\":\"address\"}],\"name\":\"nextValidRequest\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":false,\"inputs\":[],\"name\":\"request\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"\",\"type\":\"address\"}],\"name\":\"isWhitelisted\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_addr\",\"type\":\"address\"}],\"name\":\"removeFromWhitelist\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"owner\",\"outputs\":[{\"name\":\"\",\"type\":\"address\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_addr\",\"type\":\"address\"}],\"name\":\"addToWhitelist\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"newOwner\",\"type\":\"address\"}],\"name\":\"transferOwnership\",\"outputs\":[],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"requestAmount\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"token\",\"outputs\":[{\"name\":\"\",\"type\":\"address\"}],\"payable\":false,\"type\":\"function\"},{\"inputs\":[{\"name\":\"_token\",\"type\":\"address\"},{\"name\":\"_requestAmount\",\"type\":\"uint256\"},{\"name\":\"_requestWait\",\"type\":\"uint256\"}],\"payable\":false,\"type\":\"constructor\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"name\":\"to\",\"type\":\"address\"},{\"indexed\":false,\"name\":\"amount\",\"type\":\"uint256\"}],\"name\":\"Request\",\"type\":\"event\"}]"

// LivepeerTokenFaucetBin is the compiled bytecode used for deploying new contracts.
const LivepeerTokenFaucetBin = `0x6060604052341561000f57600080fd5b6040516060806105c38339810160405280805191906020018051919060200180519150505b5b60008054600160a060020a03191633600160a060020a03161790555b60018054600160a060020a031916600160a060020a038516179055600282905560038190555b5050505b6105398061008a6000396000f300606060405236156100a15763ffffffff7c01000000000000000000000000000000000000000000000000000000006000350416630d6c51b381146100a6578063207f5ce6146100cb578063338cdca1146100fc5780633af32abf146101235780638ab1d681146101565780638da5cb5b14610189578063e43252d7146101b8578063f2fde38b146101eb578063f52ec46c1461020c578063fc0c546a14610231575b600080fd5b34156100b157600080fd5b6100b9610260565b60405190815260200160405180910390f35b34156100d657600080fd5b6100b9600160a060020a0360043516610266565b60405190815260200160405180910390f35b341561010757600080fd5b61010f610278565b604051901515815260200160405180910390f35b341561012e57600080fd5b61010f600160a060020a03600435166103ec565b604051901515815260200160405180910390f35b341561016157600080fd5b61010f600160a060020a0360043516610401565b604051901515815260200160405180910390f35b341561019457600080fd5b61019c610447565b604051600160a060020a03909116815260200160405180910390f35b34156101c357600080fd5b61010f600160a060020a0360043516610456565b604051901515815260200160405180910390f35b34156101f657600080fd5b61020a600160a060020a03600435166104a0565b005b341561021757600080fd5b6100b96104f8565b60405190815260200160405180910390f35b341561023c57600080fd5b61019c6104fe565b604051600160a060020a03909116815260200160405180910390f35b60035481565b60046020526000908152604090205481565b600160a060020a03331660009081526005602052604081205460ff16806102b75750600160a060020a0333166000908152600460205260409020544210155b15156102c257600080fd5b600160a060020a03331660009081526005602052604090205460ff16151561030a5760035433600160a060020a03166000908152600460205260409020610e10909102420190555b600154600254600160a060020a039091169063a9059cbb9033906000604051602001526040517c010000000000000000000000000000000000000000000000000000000063ffffffff8516028152600160a060020a0390921660048301526024820152604401602060405180830381600087803b151561038957600080fd5b6102c65a03f1151561039a57600080fd5b505050604051805190505033600160a060020a03167fe31c60e37ab1301f69f01b436a1d13486e6c16cc22c888a08c0e64a39230b6ac60025460405190815260200160405180910390a25060015b5b90565b60056020526000908152604090205460ff1681565b6000805433600160a060020a0390811691161461041d57600080fd5b50600160a060020a0381166000908152600560205260409020805460ff1916905560015b5b919050565b600054600160a060020a031681565b6000805433600160a060020a0390811691161461047257600080fd5b50600160a060020a0381166000908152600560205260409020805460ff191660019081179091555b5b919050565b60005433600160a060020a039081169116146104bb57600080fd5b600160a060020a038116156104f3576000805473ffffffffffffffffffffffffffffffffffffffff1916600160a060020a0383161790555b5b5b50565b60025481565b600154600160a060020a0316815600a165627a7a723058200625d2f409b8be2b51aa37ff469482a1228120348bece8b1337fd28199e3e7ec0029`

// DeployLivepeerTokenFaucet deploys a new Ethereum contract, binding an instance of LivepeerTokenFaucet to it.
func DeployLivepeerTokenFaucet(auth *bind.TransactOpts, backend bind.ContractBackend, libraries map[string]common.Address, _token common.Address, _requestAmount *big.Int, _requestWait *big.Int) (common.Address, *types.Transaction, *LivepeerTokenFaucet, error) {
	parsed, err := abi.JSON(strings.NewReader(LivepeerTokenFaucetABI))
	if err != nil {
		return common.Address{}, nil, nil, err
	}

	linkedBin := LivepeerTokenFaucetBin
	for lib, addr := range libraries {
		reg, err := regexp.Compile(fmt.Sprintf("_+%s_+", lib))
		if err != nil {
			return common.Address{}, nil, nil, err
		}

		linkedBin = reg.ReplaceAllString(linkedBin, addr.Hex())
	}

	address, tx, contract, err := bind.DeployContract(auth, parsed, common.FromHex(linkedBin), backend, _token, _requestAmount, _requestWait)
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	return address, tx, &LivepeerTokenFaucet{LivepeerTokenFaucetCaller: LivepeerTokenFaucetCaller{contract: contract}, LivepeerTokenFaucetTransactor: LivepeerTokenFaucetTransactor{contract: contract}}, nil
}

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
