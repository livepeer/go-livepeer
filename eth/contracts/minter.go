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

// MinterMetaData contains all meta data concerning the Minter contract.
var MinterMetaData = &bind.MetaData{
	ABI: "[{\"constant\":false,\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_inflationChange\",\"type\":\"uint256\"}],\"name\":\"setInflationChange\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"internalType\":\"contractIMinter\",\"name\":\"_newMinter\",\"type\":\"address\"}],\"name\":\"migrateToNewMinter\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"internalType\":\"addresspayable\",\"name\":\"_to\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"_amount\",\"type\":\"uint256\"}],\"name\":\"trustedWithdrawETH\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"currentMintedTokens\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"getController\",\"outputs\":[{\"internalType\":\"contractIController\",\"name\":\"\",\"type\":\"address\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"getGlobalTotalSupply\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_targetBondingRate\",\"type\":\"uint256\"}],\"name\":\"setTargetBondingRate\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_fracNum\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"_fracDenom\",\"type\":\"uint256\"}],\"name\":\"createReward\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"targetBondingRate\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"internalType\":\"address\",\"name\":\"_controller\",\"type\":\"address\"}],\"name\":\"setController\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"currentMintableTokens\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"inflationChange\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"inflation\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_amount\",\"type\":\"uint256\"}],\"name\":\"trustedBurnTokens\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"internalType\":\"address\",\"name\":\"_to\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"_amount\",\"type\":\"uint256\"}],\"name\":\"trustedTransferTokens\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[],\"name\":\"setCurrentRewardTokens\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[],\"name\":\"depositETH\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"payable\":true,\"stateMutability\":\"payable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"controller\",\"outputs\":[{\"internalType\":\"contractIController\",\"name\":\"\",\"type\":\"address\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"_controller\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"_inflation\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"_inflationChange\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"_targetBondingRate\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"constructor\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"currentMintableTokens\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"currentInflation\",\"type\":\"uint256\"}],\"name\":\"SetCurrentRewardTokens\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"address\",\"name\":\"controller\",\"type\":\"address\"}],\"name\":\"SetController\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"string\",\"name\":\"param\",\"type\":\"string\"}],\"name\":\"ParameterUpdate\",\"type\":\"event\"}]",
}

// MinterABI is the input ABI used to generate the binding from.
// Deprecated: Use MinterMetaData.ABI instead.
var MinterABI = MinterMetaData.ABI

// Minter is an auto generated Go binding around an Ethereum contract.
type Minter struct {
	MinterCaller     // Read-only binding to the contract
	MinterTransactor // Write-only binding to the contract
	MinterFilterer   // Log filterer for contract events
}

// MinterCaller is an auto generated read-only Go binding around an Ethereum contract.
type MinterCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// MinterTransactor is an auto generated write-only Go binding around an Ethereum contract.
type MinterTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// MinterFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type MinterFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// MinterSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type MinterSession struct {
	Contract     *Minter           // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// MinterCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type MinterCallerSession struct {
	Contract *MinterCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts // Call options to use throughout this session
}

// MinterTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type MinterTransactorSession struct {
	Contract     *MinterTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// MinterRaw is an auto generated low-level Go binding around an Ethereum contract.
type MinterRaw struct {
	Contract *Minter // Generic contract binding to access the raw methods on
}

// MinterCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type MinterCallerRaw struct {
	Contract *MinterCaller // Generic read-only contract binding to access the raw methods on
}

// MinterTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type MinterTransactorRaw struct {
	Contract *MinterTransactor // Generic write-only contract binding to access the raw methods on
}

// NewMinter creates a new instance of Minter, bound to a specific deployed contract.
func NewMinter(address common.Address, backend bind.ContractBackend) (*Minter, error) {
	contract, err := bindMinter(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &Minter{MinterCaller: MinterCaller{contract: contract}, MinterTransactor: MinterTransactor{contract: contract}, MinterFilterer: MinterFilterer{contract: contract}}, nil
}

// NewMinterCaller creates a new read-only instance of Minter, bound to a specific deployed contract.
func NewMinterCaller(address common.Address, caller bind.ContractCaller) (*MinterCaller, error) {
	contract, err := bindMinter(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &MinterCaller{contract: contract}, nil
}

// NewMinterTransactor creates a new write-only instance of Minter, bound to a specific deployed contract.
func NewMinterTransactor(address common.Address, transactor bind.ContractTransactor) (*MinterTransactor, error) {
	contract, err := bindMinter(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &MinterTransactor{contract: contract}, nil
}

// NewMinterFilterer creates a new log filterer instance of Minter, bound to a specific deployed contract.
func NewMinterFilterer(address common.Address, filterer bind.ContractFilterer) (*MinterFilterer, error) {
	contract, err := bindMinter(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &MinterFilterer{contract: contract}, nil
}

// bindMinter binds a generic wrapper to an already deployed contract.
func bindMinter(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := abi.JSON(strings.NewReader(MinterABI))
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_Minter *MinterRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _Minter.Contract.MinterCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_Minter *MinterRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Minter.Contract.MinterTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_Minter *MinterRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _Minter.Contract.MinterTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_Minter *MinterCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _Minter.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_Minter *MinterTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Minter.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_Minter *MinterTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _Minter.Contract.contract.Transact(opts, method, params...)
}

// Controller is a free data retrieval call binding the contract method 0xf77c4791.
//
// Solidity: function controller() view returns(address)
func (_Minter *MinterCaller) Controller(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _Minter.contract.Call(opts, &out, "controller")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// Controller is a free data retrieval call binding the contract method 0xf77c4791.
//
// Solidity: function controller() view returns(address)
func (_Minter *MinterSession) Controller() (common.Address, error) {
	return _Minter.Contract.Controller(&_Minter.CallOpts)
}

// Controller is a free data retrieval call binding the contract method 0xf77c4791.
//
// Solidity: function controller() view returns(address)
func (_Minter *MinterCallerSession) Controller() (common.Address, error) {
	return _Minter.Contract.Controller(&_Minter.CallOpts)
}

// CurrentMintableTokens is a free data retrieval call binding the contract method 0x9ae6309a.
//
// Solidity: function currentMintableTokens() view returns(uint256)
func (_Minter *MinterCaller) CurrentMintableTokens(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _Minter.contract.Call(opts, &out, "currentMintableTokens")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// CurrentMintableTokens is a free data retrieval call binding the contract method 0x9ae6309a.
//
// Solidity: function currentMintableTokens() view returns(uint256)
func (_Minter *MinterSession) CurrentMintableTokens() (*big.Int, error) {
	return _Minter.Contract.CurrentMintableTokens(&_Minter.CallOpts)
}

// CurrentMintableTokens is a free data retrieval call binding the contract method 0x9ae6309a.
//
// Solidity: function currentMintableTokens() view returns(uint256)
func (_Minter *MinterCallerSession) CurrentMintableTokens() (*big.Int, error) {
	return _Minter.Contract.CurrentMintableTokens(&_Minter.CallOpts)
}

// CurrentMintedTokens is a free data retrieval call binding the contract method 0x2de22cdb.
//
// Solidity: function currentMintedTokens() view returns(uint256)
func (_Minter *MinterCaller) CurrentMintedTokens(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _Minter.contract.Call(opts, &out, "currentMintedTokens")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// CurrentMintedTokens is a free data retrieval call binding the contract method 0x2de22cdb.
//
// Solidity: function currentMintedTokens() view returns(uint256)
func (_Minter *MinterSession) CurrentMintedTokens() (*big.Int, error) {
	return _Minter.Contract.CurrentMintedTokens(&_Minter.CallOpts)
}

// CurrentMintedTokens is a free data retrieval call binding the contract method 0x2de22cdb.
//
// Solidity: function currentMintedTokens() view returns(uint256)
func (_Minter *MinterCallerSession) CurrentMintedTokens() (*big.Int, error) {
	return _Minter.Contract.CurrentMintedTokens(&_Minter.CallOpts)
}

// GetController is a free data retrieval call binding the contract method 0x3018205f.
//
// Solidity: function getController() view returns(address)
func (_Minter *MinterCaller) GetController(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _Minter.contract.Call(opts, &out, "getController")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// GetController is a free data retrieval call binding the contract method 0x3018205f.
//
// Solidity: function getController() view returns(address)
func (_Minter *MinterSession) GetController() (common.Address, error) {
	return _Minter.Contract.GetController(&_Minter.CallOpts)
}

// GetController is a free data retrieval call binding the contract method 0x3018205f.
//
// Solidity: function getController() view returns(address)
func (_Minter *MinterCallerSession) GetController() (common.Address, error) {
	return _Minter.Contract.GetController(&_Minter.CallOpts)
}

// GetGlobalTotalSupply is a free data retrieval call binding the contract method 0x5507442d.
//
// Solidity: function getGlobalTotalSupply() view returns(uint256)
func (_Minter *MinterCaller) GetGlobalTotalSupply(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _Minter.contract.Call(opts, &out, "getGlobalTotalSupply")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// GetGlobalTotalSupply is a free data retrieval call binding the contract method 0x5507442d.
//
// Solidity: function getGlobalTotalSupply() view returns(uint256)
func (_Minter *MinterSession) GetGlobalTotalSupply() (*big.Int, error) {
	return _Minter.Contract.GetGlobalTotalSupply(&_Minter.CallOpts)
}

// GetGlobalTotalSupply is a free data retrieval call binding the contract method 0x5507442d.
//
// Solidity: function getGlobalTotalSupply() view returns(uint256)
func (_Minter *MinterCallerSession) GetGlobalTotalSupply() (*big.Int, error) {
	return _Minter.Contract.GetGlobalTotalSupply(&_Minter.CallOpts)
}

// Inflation is a free data retrieval call binding the contract method 0xbe0522e0.
//
// Solidity: function inflation() view returns(uint256)
func (_Minter *MinterCaller) Inflation(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _Minter.contract.Call(opts, &out, "inflation")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// Inflation is a free data retrieval call binding the contract method 0xbe0522e0.
//
// Solidity: function inflation() view returns(uint256)
func (_Minter *MinterSession) Inflation() (*big.Int, error) {
	return _Minter.Contract.Inflation(&_Minter.CallOpts)
}

// Inflation is a free data retrieval call binding the contract method 0xbe0522e0.
//
// Solidity: function inflation() view returns(uint256)
func (_Minter *MinterCallerSession) Inflation() (*big.Int, error) {
	return _Minter.Contract.Inflation(&_Minter.CallOpts)
}

// InflationChange is a free data retrieval call binding the contract method 0xa7c83514.
//
// Solidity: function inflationChange() view returns(uint256)
func (_Minter *MinterCaller) InflationChange(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _Minter.contract.Call(opts, &out, "inflationChange")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// InflationChange is a free data retrieval call binding the contract method 0xa7c83514.
//
// Solidity: function inflationChange() view returns(uint256)
func (_Minter *MinterSession) InflationChange() (*big.Int, error) {
	return _Minter.Contract.InflationChange(&_Minter.CallOpts)
}

// InflationChange is a free data retrieval call binding the contract method 0xa7c83514.
//
// Solidity: function inflationChange() view returns(uint256)
func (_Minter *MinterCallerSession) InflationChange() (*big.Int, error) {
	return _Minter.Contract.InflationChange(&_Minter.CallOpts)
}

// TargetBondingRate is a free data retrieval call binding the contract method 0x821b771f.
//
// Solidity: function targetBondingRate() view returns(uint256)
func (_Minter *MinterCaller) TargetBondingRate(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _Minter.contract.Call(opts, &out, "targetBondingRate")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// TargetBondingRate is a free data retrieval call binding the contract method 0x821b771f.
//
// Solidity: function targetBondingRate() view returns(uint256)
func (_Minter *MinterSession) TargetBondingRate() (*big.Int, error) {
	return _Minter.Contract.TargetBondingRate(&_Minter.CallOpts)
}

// TargetBondingRate is a free data retrieval call binding the contract method 0x821b771f.
//
// Solidity: function targetBondingRate() view returns(uint256)
func (_Minter *MinterCallerSession) TargetBondingRate() (*big.Int, error) {
	return _Minter.Contract.TargetBondingRate(&_Minter.CallOpts)
}

// CreateReward is a paid mutator transaction binding the contract method 0x7dbedad5.
//
// Solidity: function createReward(uint256 _fracNum, uint256 _fracDenom) returns(uint256)
func (_Minter *MinterTransactor) CreateReward(opts *bind.TransactOpts, _fracNum *big.Int, _fracDenom *big.Int) (*types.Transaction, error) {
	return _Minter.contract.Transact(opts, "createReward", _fracNum, _fracDenom)
}

// CreateReward is a paid mutator transaction binding the contract method 0x7dbedad5.
//
// Solidity: function createReward(uint256 _fracNum, uint256 _fracDenom) returns(uint256)
func (_Minter *MinterSession) CreateReward(_fracNum *big.Int, _fracDenom *big.Int) (*types.Transaction, error) {
	return _Minter.Contract.CreateReward(&_Minter.TransactOpts, _fracNum, _fracDenom)
}

// CreateReward is a paid mutator transaction binding the contract method 0x7dbedad5.
//
// Solidity: function createReward(uint256 _fracNum, uint256 _fracDenom) returns(uint256)
func (_Minter *MinterTransactorSession) CreateReward(_fracNum *big.Int, _fracDenom *big.Int) (*types.Transaction, error) {
	return _Minter.Contract.CreateReward(&_Minter.TransactOpts, _fracNum, _fracDenom)
}

// DepositETH is a paid mutator transaction binding the contract method 0xf6326fb3.
//
// Solidity: function depositETH() payable returns(bool)
func (_Minter *MinterTransactor) DepositETH(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Minter.contract.Transact(opts, "depositETH")
}

// DepositETH is a paid mutator transaction binding the contract method 0xf6326fb3.
//
// Solidity: function depositETH() payable returns(bool)
func (_Minter *MinterSession) DepositETH() (*types.Transaction, error) {
	return _Minter.Contract.DepositETH(&_Minter.TransactOpts)
}

// DepositETH is a paid mutator transaction binding the contract method 0xf6326fb3.
//
// Solidity: function depositETH() payable returns(bool)
func (_Minter *MinterTransactorSession) DepositETH() (*types.Transaction, error) {
	return _Minter.Contract.DepositETH(&_Minter.TransactOpts)
}

// MigrateToNewMinter is a paid mutator transaction binding the contract method 0x18d217ad.
//
// Solidity: function migrateToNewMinter(address _newMinter) returns()
func (_Minter *MinterTransactor) MigrateToNewMinter(opts *bind.TransactOpts, _newMinter common.Address) (*types.Transaction, error) {
	return _Minter.contract.Transact(opts, "migrateToNewMinter", _newMinter)
}

// MigrateToNewMinter is a paid mutator transaction binding the contract method 0x18d217ad.
//
// Solidity: function migrateToNewMinter(address _newMinter) returns()
func (_Minter *MinterSession) MigrateToNewMinter(_newMinter common.Address) (*types.Transaction, error) {
	return _Minter.Contract.MigrateToNewMinter(&_Minter.TransactOpts, _newMinter)
}

// MigrateToNewMinter is a paid mutator transaction binding the contract method 0x18d217ad.
//
// Solidity: function migrateToNewMinter(address _newMinter) returns()
func (_Minter *MinterTransactorSession) MigrateToNewMinter(_newMinter common.Address) (*types.Transaction, error) {
	return _Minter.Contract.MigrateToNewMinter(&_Minter.TransactOpts, _newMinter)
}

// SetController is a paid mutator transaction binding the contract method 0x92eefe9b.
//
// Solidity: function setController(address _controller) returns()
func (_Minter *MinterTransactor) SetController(opts *bind.TransactOpts, _controller common.Address) (*types.Transaction, error) {
	return _Minter.contract.Transact(opts, "setController", _controller)
}

// SetController is a paid mutator transaction binding the contract method 0x92eefe9b.
//
// Solidity: function setController(address _controller) returns()
func (_Minter *MinterSession) SetController(_controller common.Address) (*types.Transaction, error) {
	return _Minter.Contract.SetController(&_Minter.TransactOpts, _controller)
}

// SetController is a paid mutator transaction binding the contract method 0x92eefe9b.
//
// Solidity: function setController(address _controller) returns()
func (_Minter *MinterTransactorSession) SetController(_controller common.Address) (*types.Transaction, error) {
	return _Minter.Contract.SetController(&_Minter.TransactOpts, _controller)
}

// SetCurrentRewardTokens is a paid mutator transaction binding the contract method 0xece2064c.
//
// Solidity: function setCurrentRewardTokens() returns()
func (_Minter *MinterTransactor) SetCurrentRewardTokens(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Minter.contract.Transact(opts, "setCurrentRewardTokens")
}

// SetCurrentRewardTokens is a paid mutator transaction binding the contract method 0xece2064c.
//
// Solidity: function setCurrentRewardTokens() returns()
func (_Minter *MinterSession) SetCurrentRewardTokens() (*types.Transaction, error) {
	return _Minter.Contract.SetCurrentRewardTokens(&_Minter.TransactOpts)
}

// SetCurrentRewardTokens is a paid mutator transaction binding the contract method 0xece2064c.
//
// Solidity: function setCurrentRewardTokens() returns()
func (_Minter *MinterTransactorSession) SetCurrentRewardTokens() (*types.Transaction, error) {
	return _Minter.Contract.SetCurrentRewardTokens(&_Minter.TransactOpts)
}

// SetInflationChange is a paid mutator transaction binding the contract method 0x010e3c1c.
//
// Solidity: function setInflationChange(uint256 _inflationChange) returns()
func (_Minter *MinterTransactor) SetInflationChange(opts *bind.TransactOpts, _inflationChange *big.Int) (*types.Transaction, error) {
	return _Minter.contract.Transact(opts, "setInflationChange", _inflationChange)
}

// SetInflationChange is a paid mutator transaction binding the contract method 0x010e3c1c.
//
// Solidity: function setInflationChange(uint256 _inflationChange) returns()
func (_Minter *MinterSession) SetInflationChange(_inflationChange *big.Int) (*types.Transaction, error) {
	return _Minter.Contract.SetInflationChange(&_Minter.TransactOpts, _inflationChange)
}

// SetInflationChange is a paid mutator transaction binding the contract method 0x010e3c1c.
//
// Solidity: function setInflationChange(uint256 _inflationChange) returns()
func (_Minter *MinterTransactorSession) SetInflationChange(_inflationChange *big.Int) (*types.Transaction, error) {
	return _Minter.Contract.SetInflationChange(&_Minter.TransactOpts, _inflationChange)
}

// SetTargetBondingRate is a paid mutator transaction binding the contract method 0x77bde142.
//
// Solidity: function setTargetBondingRate(uint256 _targetBondingRate) returns()
func (_Minter *MinterTransactor) SetTargetBondingRate(opts *bind.TransactOpts, _targetBondingRate *big.Int) (*types.Transaction, error) {
	return _Minter.contract.Transact(opts, "setTargetBondingRate", _targetBondingRate)
}

// SetTargetBondingRate is a paid mutator transaction binding the contract method 0x77bde142.
//
// Solidity: function setTargetBondingRate(uint256 _targetBondingRate) returns()
func (_Minter *MinterSession) SetTargetBondingRate(_targetBondingRate *big.Int) (*types.Transaction, error) {
	return _Minter.Contract.SetTargetBondingRate(&_Minter.TransactOpts, _targetBondingRate)
}

// SetTargetBondingRate is a paid mutator transaction binding the contract method 0x77bde142.
//
// Solidity: function setTargetBondingRate(uint256 _targetBondingRate) returns()
func (_Minter *MinterTransactorSession) SetTargetBondingRate(_targetBondingRate *big.Int) (*types.Transaction, error) {
	return _Minter.Contract.SetTargetBondingRate(&_Minter.TransactOpts, _targetBondingRate)
}

// TrustedBurnTokens is a paid mutator transaction binding the contract method 0xc7ee98c2.
//
// Solidity: function trustedBurnTokens(uint256 _amount) returns()
func (_Minter *MinterTransactor) TrustedBurnTokens(opts *bind.TransactOpts, _amount *big.Int) (*types.Transaction, error) {
	return _Minter.contract.Transact(opts, "trustedBurnTokens", _amount)
}

// TrustedBurnTokens is a paid mutator transaction binding the contract method 0xc7ee98c2.
//
// Solidity: function trustedBurnTokens(uint256 _amount) returns()
func (_Minter *MinterSession) TrustedBurnTokens(_amount *big.Int) (*types.Transaction, error) {
	return _Minter.Contract.TrustedBurnTokens(&_Minter.TransactOpts, _amount)
}

// TrustedBurnTokens is a paid mutator transaction binding the contract method 0xc7ee98c2.
//
// Solidity: function trustedBurnTokens(uint256 _amount) returns()
func (_Minter *MinterTransactorSession) TrustedBurnTokens(_amount *big.Int) (*types.Transaction, error) {
	return _Minter.Contract.TrustedBurnTokens(&_Minter.TransactOpts, _amount)
}

// TrustedTransferTokens is a paid mutator transaction binding the contract method 0xe7a49c2b.
//
// Solidity: function trustedTransferTokens(address _to, uint256 _amount) returns()
func (_Minter *MinterTransactor) TrustedTransferTokens(opts *bind.TransactOpts, _to common.Address, _amount *big.Int) (*types.Transaction, error) {
	return _Minter.contract.Transact(opts, "trustedTransferTokens", _to, _amount)
}

// TrustedTransferTokens is a paid mutator transaction binding the contract method 0xe7a49c2b.
//
// Solidity: function trustedTransferTokens(address _to, uint256 _amount) returns()
func (_Minter *MinterSession) TrustedTransferTokens(_to common.Address, _amount *big.Int) (*types.Transaction, error) {
	return _Minter.Contract.TrustedTransferTokens(&_Minter.TransactOpts, _to, _amount)
}

// TrustedTransferTokens is a paid mutator transaction binding the contract method 0xe7a49c2b.
//
// Solidity: function trustedTransferTokens(address _to, uint256 _amount) returns()
func (_Minter *MinterTransactorSession) TrustedTransferTokens(_to common.Address, _amount *big.Int) (*types.Transaction, error) {
	return _Minter.Contract.TrustedTransferTokens(&_Minter.TransactOpts, _to, _amount)
}

// TrustedWithdrawETH is a paid mutator transaction binding the contract method 0x20283da9.
//
// Solidity: function trustedWithdrawETH(address _to, uint256 _amount) returns()
func (_Minter *MinterTransactor) TrustedWithdrawETH(opts *bind.TransactOpts, _to common.Address, _amount *big.Int) (*types.Transaction, error) {
	return _Minter.contract.Transact(opts, "trustedWithdrawETH", _to, _amount)
}

// TrustedWithdrawETH is a paid mutator transaction binding the contract method 0x20283da9.
//
// Solidity: function trustedWithdrawETH(address _to, uint256 _amount) returns()
func (_Minter *MinterSession) TrustedWithdrawETH(_to common.Address, _amount *big.Int) (*types.Transaction, error) {
	return _Minter.Contract.TrustedWithdrawETH(&_Minter.TransactOpts, _to, _amount)
}

// TrustedWithdrawETH is a paid mutator transaction binding the contract method 0x20283da9.
//
// Solidity: function trustedWithdrawETH(address _to, uint256 _amount) returns()
func (_Minter *MinterTransactorSession) TrustedWithdrawETH(_to common.Address, _amount *big.Int) (*types.Transaction, error) {
	return _Minter.Contract.TrustedWithdrawETH(&_Minter.TransactOpts, _to, _amount)
}

// MinterParameterUpdateIterator is returned from FilterParameterUpdate and is used to iterate over the raw logs and unpacked data for ParameterUpdate events raised by the Minter contract.
type MinterParameterUpdateIterator struct {
	Event *MinterParameterUpdate // Event containing the contract specifics and raw log

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
func (it *MinterParameterUpdateIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(MinterParameterUpdate)
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
		it.Event = new(MinterParameterUpdate)
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
func (it *MinterParameterUpdateIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *MinterParameterUpdateIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// MinterParameterUpdate represents a ParameterUpdate event raised by the Minter contract.
type MinterParameterUpdate struct {
	Param string
	Raw   types.Log // Blockchain specific contextual infos
}

// FilterParameterUpdate is a free log retrieval operation binding the contract event 0x9f5033568d78ae30f29f01e944f97b2216493bd19d1b46d429673acff3dcd674.
//
// Solidity: event ParameterUpdate(string param)
func (_Minter *MinterFilterer) FilterParameterUpdate(opts *bind.FilterOpts) (*MinterParameterUpdateIterator, error) {

	logs, sub, err := _Minter.contract.FilterLogs(opts, "ParameterUpdate")
	if err != nil {
		return nil, err
	}
	return &MinterParameterUpdateIterator{contract: _Minter.contract, event: "ParameterUpdate", logs: logs, sub: sub}, nil
}

// WatchParameterUpdate is a free log subscription operation binding the contract event 0x9f5033568d78ae30f29f01e944f97b2216493bd19d1b46d429673acff3dcd674.
//
// Solidity: event ParameterUpdate(string param)
func (_Minter *MinterFilterer) WatchParameterUpdate(opts *bind.WatchOpts, sink chan<- *MinterParameterUpdate) (event.Subscription, error) {

	logs, sub, err := _Minter.contract.WatchLogs(opts, "ParameterUpdate")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(MinterParameterUpdate)
				if err := _Minter.contract.UnpackLog(event, "ParameterUpdate", log); err != nil {
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

// ParseParameterUpdate is a log parse operation binding the contract event 0x9f5033568d78ae30f29f01e944f97b2216493bd19d1b46d429673acff3dcd674.
//
// Solidity: event ParameterUpdate(string param)
func (_Minter *MinterFilterer) ParseParameterUpdate(log types.Log) (*MinterParameterUpdate, error) {
	event := new(MinterParameterUpdate)
	if err := _Minter.contract.UnpackLog(event, "ParameterUpdate", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// MinterSetControllerIterator is returned from FilterSetController and is used to iterate over the raw logs and unpacked data for SetController events raised by the Minter contract.
type MinterSetControllerIterator struct {
	Event *MinterSetController // Event containing the contract specifics and raw log

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
func (it *MinterSetControllerIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(MinterSetController)
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
		it.Event = new(MinterSetController)
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
func (it *MinterSetControllerIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *MinterSetControllerIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// MinterSetController represents a SetController event raised by the Minter contract.
type MinterSetController struct {
	Controller common.Address
	Raw        types.Log // Blockchain specific contextual infos
}

// FilterSetController is a free log retrieval operation binding the contract event 0x4ff638452bbf33c012645d18ae6f05515ff5f2d1dfb0cece8cbf018c60903f70.
//
// Solidity: event SetController(address controller)
func (_Minter *MinterFilterer) FilterSetController(opts *bind.FilterOpts) (*MinterSetControllerIterator, error) {

	logs, sub, err := _Minter.contract.FilterLogs(opts, "SetController")
	if err != nil {
		return nil, err
	}
	return &MinterSetControllerIterator{contract: _Minter.contract, event: "SetController", logs: logs, sub: sub}, nil
}

// WatchSetController is a free log subscription operation binding the contract event 0x4ff638452bbf33c012645d18ae6f05515ff5f2d1dfb0cece8cbf018c60903f70.
//
// Solidity: event SetController(address controller)
func (_Minter *MinterFilterer) WatchSetController(opts *bind.WatchOpts, sink chan<- *MinterSetController) (event.Subscription, error) {

	logs, sub, err := _Minter.contract.WatchLogs(opts, "SetController")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(MinterSetController)
				if err := _Minter.contract.UnpackLog(event, "SetController", log); err != nil {
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

// ParseSetController is a log parse operation binding the contract event 0x4ff638452bbf33c012645d18ae6f05515ff5f2d1dfb0cece8cbf018c60903f70.
//
// Solidity: event SetController(address controller)
func (_Minter *MinterFilterer) ParseSetController(log types.Log) (*MinterSetController, error) {
	event := new(MinterSetController)
	if err := _Minter.contract.UnpackLog(event, "SetController", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// MinterSetCurrentRewardTokensIterator is returned from FilterSetCurrentRewardTokens and is used to iterate over the raw logs and unpacked data for SetCurrentRewardTokens events raised by the Minter contract.
type MinterSetCurrentRewardTokensIterator struct {
	Event *MinterSetCurrentRewardTokens // Event containing the contract specifics and raw log

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
func (it *MinterSetCurrentRewardTokensIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(MinterSetCurrentRewardTokens)
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
		it.Event = new(MinterSetCurrentRewardTokens)
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
func (it *MinterSetCurrentRewardTokensIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *MinterSetCurrentRewardTokensIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// MinterSetCurrentRewardTokens represents a SetCurrentRewardTokens event raised by the Minter contract.
type MinterSetCurrentRewardTokens struct {
	CurrentMintableTokens *big.Int
	CurrentInflation      *big.Int
	Raw                   types.Log // Blockchain specific contextual infos
}

// FilterSetCurrentRewardTokens is a free log retrieval operation binding the contract event 0x39567a366345edf17f50c1967a31b597745186c4632f34c4f8ebe06b6890784d.
//
// Solidity: event SetCurrentRewardTokens(uint256 currentMintableTokens, uint256 currentInflation)
func (_Minter *MinterFilterer) FilterSetCurrentRewardTokens(opts *bind.FilterOpts) (*MinterSetCurrentRewardTokensIterator, error) {

	logs, sub, err := _Minter.contract.FilterLogs(opts, "SetCurrentRewardTokens")
	if err != nil {
		return nil, err
	}
	return &MinterSetCurrentRewardTokensIterator{contract: _Minter.contract, event: "SetCurrentRewardTokens", logs: logs, sub: sub}, nil
}

// WatchSetCurrentRewardTokens is a free log subscription operation binding the contract event 0x39567a366345edf17f50c1967a31b597745186c4632f34c4f8ebe06b6890784d.
//
// Solidity: event SetCurrentRewardTokens(uint256 currentMintableTokens, uint256 currentInflation)
func (_Minter *MinterFilterer) WatchSetCurrentRewardTokens(opts *bind.WatchOpts, sink chan<- *MinterSetCurrentRewardTokens) (event.Subscription, error) {

	logs, sub, err := _Minter.contract.WatchLogs(opts, "SetCurrentRewardTokens")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(MinterSetCurrentRewardTokens)
				if err := _Minter.contract.UnpackLog(event, "SetCurrentRewardTokens", log); err != nil {
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

// ParseSetCurrentRewardTokens is a log parse operation binding the contract event 0x39567a366345edf17f50c1967a31b597745186c4632f34c4f8ebe06b6890784d.
//
// Solidity: event SetCurrentRewardTokens(uint256 currentMintableTokens, uint256 currentInflation)
func (_Minter *MinterFilterer) ParseSetCurrentRewardTokens(log types.Log) (*MinterSetCurrentRewardTokens, error) {
	event := new(MinterSetCurrentRewardTokens)
	if err := _Minter.contract.UnpackLog(event, "SetCurrentRewardTokens", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}
