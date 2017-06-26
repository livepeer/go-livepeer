// This file is an automatically generated Go binding. Do not modify as any
// change will likely be lost upon the next re-generation!

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

// ECVerifyABI is the input ABI used to generate the binding from.
const ECVerifyABI = "[{\"constant\":true,\"inputs\":[{\"name\":\"hash\",\"type\":\"bytes32\"},{\"name\":\"sig\",\"type\":\"bytes\"},{\"name\":\"signer\",\"type\":\"address\"}],\"name\":\"ecverify\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"hash\",\"type\":\"bytes32\"},{\"name\":\"sig\",\"type\":\"bytes\"}],\"name\":\"ecrecovery\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"},{\"name\":\"\",\"type\":\"address\"}],\"payable\":false,\"type\":\"function\"}]"

// ECVerifyBin is the compiled bytecode used for deploying new contracts.
const ECVerifyBin = `0x6060604052341561000c57fe5b5b61024d8061001c6000396000f300606060405263ffffffff60e060020a60003504166339cdde32811461002c57806377d32e941461009a575bfe5b60408051602060046024803582810135601f8101859004850286018501909652858552610086958335959394604494939290920191819084018382808284375094965050509235600160a060020a0316925061010c915050565b604080519115158252519081900360200190f35b60408051602060046024803582810135601f81018590048502860185019096528585526100e9958335959394604494939290920191819084018382808284375094965061015095505050505050565b604080519215158352600160a060020a0390911660208301528051918290030190f35b60006000600061011c8686610150565b90925090506001821515148015610144575083600160a060020a031681600160a060020a0316145b92505b50509392505050565b600060006000600060008551604114151561017157600094508493506101d0565b50505060208301516040840151606085015160001a601b60ff8216101561019657601b015b8060ff16601b141580156101ae57508060ff16601c14155b156101bf57600094508493506101d0565b6101cb878285856101da565b945094505b5050509250929050565b600060006000600060405188815287602082015286604082015285606082015260208160808360006001610bb8f1925080519150508181935093505b5050945094925050505600a165627a7a72305820f0466684a92ce61864598bcc3ad3a7ec7c832a2957760c43dff77e8f5e3ebd010029`

// DeployECVerify deploys a new Ethereum contract, binding an instance of ECVerify to it.
func DeployECVerify(auth *bind.TransactOpts, backend bind.ContractBackend, libraries map[string]common.Address) (common.Address, *types.Transaction, *ECVerify, error) {
	parsed, err := abi.JSON(strings.NewReader(ECVerifyABI))
	if err != nil {
		return common.Address{}, nil, nil, err
	}

	linkedBin := ECVerifyBin
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
	return address, tx, &ECVerify{ECVerifyCaller: ECVerifyCaller{contract: contract}, ECVerifyTransactor: ECVerifyTransactor{contract: contract}}, nil
}

// ECVerify is an auto generated Go binding around an Ethereum contract.
type ECVerify struct {
	ECVerifyCaller     // Read-only binding to the contract
	ECVerifyTransactor // Write-only binding to the contract
}

// ECVerifyCaller is an auto generated read-only Go binding around an Ethereum contract.
type ECVerifyCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// ECVerifyTransactor is an auto generated write-only Go binding around an Ethereum contract.
type ECVerifyTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// ECVerifySession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type ECVerifySession struct {
	Contract     *ECVerify         // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// ECVerifyCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type ECVerifyCallerSession struct {
	Contract *ECVerifyCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts   // Call options to use throughout this session
}

// ECVerifyTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type ECVerifyTransactorSession struct {
	Contract     *ECVerifyTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts   // Transaction auth options to use throughout this session
}

// ECVerifyRaw is an auto generated low-level Go binding around an Ethereum contract.
type ECVerifyRaw struct {
	Contract *ECVerify // Generic contract binding to access the raw methods on
}

// ECVerifyCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type ECVerifyCallerRaw struct {
	Contract *ECVerifyCaller // Generic read-only contract binding to access the raw methods on
}

// ECVerifyTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type ECVerifyTransactorRaw struct {
	Contract *ECVerifyTransactor // Generic write-only contract binding to access the raw methods on
}

// NewECVerify creates a new instance of ECVerify, bound to a specific deployed contract.
func NewECVerify(address common.Address, backend bind.ContractBackend) (*ECVerify, error) {
	contract, err := bindECVerify(address, backend, backend)
	if err != nil {
		return nil, err
	}
	return &ECVerify{ECVerifyCaller: ECVerifyCaller{contract: contract}, ECVerifyTransactor: ECVerifyTransactor{contract: contract}}, nil
}

// NewECVerifyCaller creates a new read-only instance of ECVerify, bound to a specific deployed contract.
func NewECVerifyCaller(address common.Address, caller bind.ContractCaller) (*ECVerifyCaller, error) {
	contract, err := bindECVerify(address, caller, nil)
	if err != nil {
		return nil, err
	}
	return &ECVerifyCaller{contract: contract}, nil
}

// NewECVerifyTransactor creates a new write-only instance of ECVerify, bound to a specific deployed contract.
func NewECVerifyTransactor(address common.Address, transactor bind.ContractTransactor) (*ECVerifyTransactor, error) {
	contract, err := bindECVerify(address, nil, transactor)
	if err != nil {
		return nil, err
	}
	return &ECVerifyTransactor{contract: contract}, nil
}

// bindECVerify binds a generic wrapper to an already deployed contract.
func bindECVerify(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor) (*bind.BoundContract, error) {
	parsed, err := abi.JSON(strings.NewReader(ECVerifyABI))
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, parsed, caller, transactor), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_ECVerify *ECVerifyRaw) Call(opts *bind.CallOpts, result interface{}, method string, params ...interface{}) error {
	return _ECVerify.Contract.ECVerifyCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_ECVerify *ECVerifyRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _ECVerify.Contract.ECVerifyTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_ECVerify *ECVerifyRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _ECVerify.Contract.ECVerifyTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_ECVerify *ECVerifyCallerRaw) Call(opts *bind.CallOpts, result interface{}, method string, params ...interface{}) error {
	return _ECVerify.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_ECVerify *ECVerifyTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _ECVerify.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_ECVerify *ECVerifyTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _ECVerify.Contract.contract.Transact(opts, method, params...)
}

// Ecverify is a free data retrieval call binding the contract method 0x39cdde32.
//
// Solidity: function ecverify(hash bytes32, sig bytes, signer address) constant returns(bool)
func (_ECVerify *ECVerifyCaller) Ecverify(opts *bind.CallOpts, hash [32]byte, sig []byte, signer common.Address) (bool, error) {
	var (
		ret0 = new(bool)
	)
	out := ret0
	err := _ECVerify.contract.Call(opts, out, "ecverify", hash, sig, signer)
	return *ret0, err
}

// Ecverify is a free data retrieval call binding the contract method 0x39cdde32.
//
// Solidity: function ecverify(hash bytes32, sig bytes, signer address) constant returns(bool)
func (_ECVerify *ECVerifySession) Ecverify(hash [32]byte, sig []byte, signer common.Address) (bool, error) {
	return _ECVerify.Contract.Ecverify(&_ECVerify.CallOpts, hash, sig, signer)
}

// Ecverify is a free data retrieval call binding the contract method 0x39cdde32.
//
// Solidity: function ecverify(hash bytes32, sig bytes, signer address) constant returns(bool)
func (_ECVerify *ECVerifyCallerSession) Ecverify(hash [32]byte, sig []byte, signer common.Address) (bool, error) {
	return _ECVerify.Contract.Ecverify(&_ECVerify.CallOpts, hash, sig, signer)
}

// Ecrecovery is a paid mutator transaction binding the contract method 0x77d32e94.
//
// Solidity: function ecrecovery(hash bytes32, sig bytes) returns(bool, address)
func (_ECVerify *ECVerifyTransactor) Ecrecovery(opts *bind.TransactOpts, hash [32]byte, sig []byte) (*types.Transaction, error) {
	return _ECVerify.contract.Transact(opts, "ecrecovery", hash, sig)
}

// Ecrecovery is a paid mutator transaction binding the contract method 0x77d32e94.
//
// Solidity: function ecrecovery(hash bytes32, sig bytes) returns(bool, address)
func (_ECVerify *ECVerifySession) Ecrecovery(hash [32]byte, sig []byte) (*types.Transaction, error) {
	return _ECVerify.Contract.Ecrecovery(&_ECVerify.TransactOpts, hash, sig)
}

// Ecrecovery is a paid mutator transaction binding the contract method 0x77d32e94.
//
// Solidity: function ecrecovery(hash bytes32, sig bytes) returns(bool, address)
func (_ECVerify *ECVerifyTransactorSession) Ecrecovery(hash [32]byte, sig []byte) (*types.Transaction, error) {
	return _ECVerify.Contract.Ecrecovery(&_ECVerify.TransactOpts, hash, sig)
}
