// This file is an automatically generated Go binding. Do not modify as any
// change will likely be lost upon the next re-generation!

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

// IdentityVerifierABI is the input ABI used to generate the binding from.
const IdentityVerifierABI = "[{\"constant\":false,\"inputs\":[{\"name\":\"_jobId\",\"type\":\"uint256\"},{\"name\":\"_claimId\",\"type\":\"uint256\"},{\"name\":\"_segmentNumber\",\"type\":\"uint256\"},{\"name\":\"_dataHash\",\"type\":\"string\"},{\"name\":\"_transcodedDataHash\",\"type\":\"string\"},{\"name\":\"_callbackContract\",\"type\":\"address\"}],\"name\":\"verify\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":true,\"type\":\"function\"}]"

// IdentityVerifierBin is the compiled bytecode used for deploying new contracts.
const IdentityVerifierBin = `0x6060604052341561000f57600080fd5b5b61017c8061001f6000396000f300606060405263ffffffff7c0100000000000000000000000000000000000000000000000000000000600035041663d3251d63811461003d575b600080fd5b610081600480359060248035916044359160643580820192908101359160843590810191013573ffffffffffffffffffffffffffffffffffffffff60a43516610095565b604051901515815260200160405180910390f35b60008173ffffffffffffffffffffffffffffffffffffffff16631e0976f38a8a8a60016000604051602001526040517c010000000000000000000000000000000000000000000000000000000063ffffffff871602815260048101949094526024840192909252604483015215156064820152608401602060405180830381600087803b151561012457600080fd5b6102c65a03f1151561013557600080fd5b50505060405180515060019150505b989750505050505050505600a165627a7a723058200c479f1e819afe3ee167b33f1a9b15fc314da1c696d7eb308e94d49473fc74550029`

// DeployIdentityVerifier deploys a new Ethereum contract, binding an instance of IdentityVerifier to it.
func DeployIdentityVerifier(auth *bind.TransactOpts, backend bind.ContractBackend, libraries map[string]common.Address) (common.Address, *types.Transaction, *IdentityVerifier, error) {
	parsed, err := abi.JSON(strings.NewReader(IdentityVerifierABI))
	if err != nil {
		return common.Address{}, nil, nil, err
	}

	linkedBin := IdentityVerifierBin
	for lib, addr := range libraries {
		reg, err := regexp.Compile(fmt.Sprintf("_+%s_+", lib))
		if err != nil {
			return common.Address{}, nil, nil, err
		}

		linkedBin = reg.ReplaceAllString(linkedBin, addr.Hex()[2:])
	}

	address, tx, contract, err := bind.DeployContract(auth, parsed, common.FromHex(linkedBin), backend)
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	return address, tx, &IdentityVerifier{IdentityVerifierCaller: IdentityVerifierCaller{contract: contract}, IdentityVerifierTransactor: IdentityVerifierTransactor{contract: contract}}, nil
}

// IdentityVerifier is an auto generated Go binding around an Ethereum contract.
type IdentityVerifier struct {
	IdentityVerifierCaller     // Read-only binding to the contract
	IdentityVerifierTransactor // Write-only binding to the contract
}

// IdentityVerifierCaller is an auto generated read-only Go binding around an Ethereum contract.
type IdentityVerifierCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// IdentityVerifierTransactor is an auto generated write-only Go binding around an Ethereum contract.
type IdentityVerifierTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// IdentityVerifierSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type IdentityVerifierSession struct {
	Contract     *IdentityVerifier // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// IdentityVerifierCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type IdentityVerifierCallerSession struct {
	Contract *IdentityVerifierCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts           // Call options to use throughout this session
}

// IdentityVerifierTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type IdentityVerifierTransactorSession struct {
	Contract     *IdentityVerifierTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts           // Transaction auth options to use throughout this session
}

// IdentityVerifierRaw is an auto generated low-level Go binding around an Ethereum contract.
type IdentityVerifierRaw struct {
	Contract *IdentityVerifier // Generic contract binding to access the raw methods on
}

// IdentityVerifierCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type IdentityVerifierCallerRaw struct {
	Contract *IdentityVerifierCaller // Generic read-only contract binding to access the raw methods on
}

// IdentityVerifierTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type IdentityVerifierTransactorRaw struct {
	Contract *IdentityVerifierTransactor // Generic write-only contract binding to access the raw methods on
}

// NewIdentityVerifier creates a new instance of IdentityVerifier, bound to a specific deployed contract.
func NewIdentityVerifier(address common.Address, backend bind.ContractBackend) (*IdentityVerifier, error) {
	contract, err := bindIdentityVerifier(address, backend, backend)
	if err != nil {
		return nil, err
	}
	return &IdentityVerifier{IdentityVerifierCaller: IdentityVerifierCaller{contract: contract}, IdentityVerifierTransactor: IdentityVerifierTransactor{contract: contract}}, nil
}

// NewIdentityVerifierCaller creates a new read-only instance of IdentityVerifier, bound to a specific deployed contract.
func NewIdentityVerifierCaller(address common.Address, caller bind.ContractCaller) (*IdentityVerifierCaller, error) {
	contract, err := bindIdentityVerifier(address, caller, nil)
	if err != nil {
		return nil, err
	}
	return &IdentityVerifierCaller{contract: contract}, nil
}

// NewIdentityVerifierTransactor creates a new write-only instance of IdentityVerifier, bound to a specific deployed contract.
func NewIdentityVerifierTransactor(address common.Address, transactor bind.ContractTransactor) (*IdentityVerifierTransactor, error) {
	contract, err := bindIdentityVerifier(address, nil, transactor)
	if err != nil {
		return nil, err
	}
	return &IdentityVerifierTransactor{contract: contract}, nil
}

// bindIdentityVerifier binds a generic wrapper to an already deployed contract.
func bindIdentityVerifier(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor) (*bind.BoundContract, error) {
	parsed, err := abi.JSON(strings.NewReader(IdentityVerifierABI))
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, parsed, caller, transactor), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_IdentityVerifier *IdentityVerifierRaw) Call(opts *bind.CallOpts, result interface{}, method string, params ...interface{}) error {
	return _IdentityVerifier.Contract.IdentityVerifierCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_IdentityVerifier *IdentityVerifierRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _IdentityVerifier.Contract.IdentityVerifierTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_IdentityVerifier *IdentityVerifierRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _IdentityVerifier.Contract.IdentityVerifierTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_IdentityVerifier *IdentityVerifierCallerRaw) Call(opts *bind.CallOpts, result interface{}, method string, params ...interface{}) error {
	return _IdentityVerifier.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_IdentityVerifier *IdentityVerifierTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _IdentityVerifier.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_IdentityVerifier *IdentityVerifierTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _IdentityVerifier.Contract.contract.Transact(opts, method, params...)
}

// Verify is a paid mutator transaction binding the contract method 0xd3251d63.
//
// Solidity: function verify(_jobId uint256, _claimId uint256, _segmentNumber uint256, _dataHash string, _transcodedDataHash string, _callbackContract address) returns(bool)
func (_IdentityVerifier *IdentityVerifierTransactor) Verify(opts *bind.TransactOpts, _jobId *big.Int, _claimId *big.Int, _segmentNumber *big.Int, _dataHash string, _transcodedDataHash string, _callbackContract common.Address) (*types.Transaction, error) {
	return _IdentityVerifier.contract.Transact(opts, "verify", _jobId, _claimId, _segmentNumber, _dataHash, _transcodedDataHash, _callbackContract)
}

// Verify is a paid mutator transaction binding the contract method 0xd3251d63.
//
// Solidity: function verify(_jobId uint256, _claimId uint256, _segmentNumber uint256, _dataHash string, _transcodedDataHash string, _callbackContract address) returns(bool)
func (_IdentityVerifier *IdentityVerifierSession) Verify(_jobId *big.Int, _claimId *big.Int, _segmentNumber *big.Int, _dataHash string, _transcodedDataHash string, _callbackContract common.Address) (*types.Transaction, error) {
	return _IdentityVerifier.Contract.Verify(&_IdentityVerifier.TransactOpts, _jobId, _claimId, _segmentNumber, _dataHash, _transcodedDataHash, _callbackContract)
}

// Verify is a paid mutator transaction binding the contract method 0xd3251d63.
//
// Solidity: function verify(_jobId uint256, _claimId uint256, _segmentNumber uint256, _dataHash string, _transcodedDataHash string, _callbackContract address) returns(bool)
func (_IdentityVerifier *IdentityVerifierTransactorSession) Verify(_jobId *big.Int, _claimId *big.Int, _segmentNumber *big.Int, _dataHash string, _transcodedDataHash string, _callbackContract common.Address) (*types.Transaction, error) {
	return _IdentityVerifier.Contract.Verify(&_IdentityVerifier.TransactOpts, _jobId, _claimId, _segmentNumber, _dataHash, _transcodedDataHash, _callbackContract)
}
