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

// MinterABI is the input ABI used to generate the binding from.
const MinterABI = "[{\"constant\":false,\"inputs\":[{\"name\":\"_newOwner\",\"type\":\"address\"}],\"name\":\"transferTokenOwnership\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"currentMintedTokens\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_to\",\"type\":\"address\"},{\"name\":\"_amount\",\"type\":\"uint256\"}],\"name\":\"withdrawETH\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_amount\",\"type\":\"uint256\"}],\"name\":\"burnTokens\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_targetBondingRate\",\"type\":\"uint256\"}],\"name\":\"setTargetBondingRate\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_fracNum\",\"type\":\"uint256\"},{\"name\":\"_fracDenom\",\"type\":\"uint256\"}],\"name\":\"createReward\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"targetBondingRate\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_controller\",\"type\":\"address\"}],\"name\":\"setController\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"currentMintableTokens\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"inflationChange\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"inflation\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_to\",\"type\":\"address\"},{\"name\":\"_amount\",\"type\":\"uint256\"}],\"name\":\"transferTokens\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[],\"name\":\"setCurrentRewardTokens\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[],\"name\":\"depositETH\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":true,\"stateMutability\":\"payable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"controller\",\"outputs\":[{\"name\":\"\",\"type\":\"address\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"name\":\"_controller\",\"type\":\"address\"},{\"name\":\"_inflation\",\"type\":\"uint256\"},{\"name\":\"_inflationChange\",\"type\":\"uint256\"},{\"name\":\"_targetBondingRate\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"constructor\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"name\":\"inflation\",\"type\":\"uint256\"}],\"name\":\"NewInflation\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"name\":\"mintableTokens\",\"type\":\"uint256\"}],\"name\":\"SetCurrentRewardTokens\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"name\":\"controller\",\"type\":\"address\"}],\"name\":\"SetController\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"name\":\"param\",\"type\":\"string\"}],\"name\":\"ParameterUpdate\",\"type\":\"event\"}]"

// Minter is an auto generated Go binding around an Ethereum contract.
type Minter struct {
	MinterCaller     // Read-only binding to the contract
	MinterTransactor // Write-only binding to the contract
}

// MinterCaller is an auto generated read-only Go binding around an Ethereum contract.
type MinterCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// MinterTransactor is an auto generated write-only Go binding around an Ethereum contract.
type MinterTransactor struct {
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
	contract, err := bindMinter(address, backend, backend)
	if err != nil {
		return nil, err
	}
	return &Minter{MinterCaller: MinterCaller{contract: contract}, MinterTransactor: MinterTransactor{contract: contract}}, nil
}

// NewMinterCaller creates a new read-only instance of Minter, bound to a specific deployed contract.
func NewMinterCaller(address common.Address, caller bind.ContractCaller) (*MinterCaller, error) {
	contract, err := bindMinter(address, caller, nil)
	if err != nil {
		return nil, err
	}
	return &MinterCaller{contract: contract}, nil
}

// NewMinterTransactor creates a new write-only instance of Minter, bound to a specific deployed contract.
func NewMinterTransactor(address common.Address, transactor bind.ContractTransactor) (*MinterTransactor, error) {
	contract, err := bindMinter(address, nil, transactor)
	if err != nil {
		return nil, err
	}
	return &MinterTransactor{contract: contract}, nil
}

// bindMinter binds a generic wrapper to an already deployed contract.
func bindMinter(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor) (*bind.BoundContract, error) {
	parsed, err := abi.JSON(strings.NewReader(MinterABI))
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, parsed, caller, transactor), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_Minter *MinterRaw) Call(opts *bind.CallOpts, result interface{}, method string, params ...interface{}) error {
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
func (_Minter *MinterCallerRaw) Call(opts *bind.CallOpts, result interface{}, method string, params ...interface{}) error {
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
// Solidity: function controller() constant returns(address)
func (_Minter *MinterCaller) Controller(opts *bind.CallOpts) (common.Address, error) {
	var (
		ret0 = new(common.Address)
	)
	out := ret0
	err := _Minter.contract.Call(opts, out, "controller")
	return *ret0, err
}

// Controller is a free data retrieval call binding the contract method 0xf77c4791.
//
// Solidity: function controller() constant returns(address)
func (_Minter *MinterSession) Controller() (common.Address, error) {
	return _Minter.Contract.Controller(&_Minter.CallOpts)
}

// Controller is a free data retrieval call binding the contract method 0xf77c4791.
//
// Solidity: function controller() constant returns(address)
func (_Minter *MinterCallerSession) Controller() (common.Address, error) {
	return _Minter.Contract.Controller(&_Minter.CallOpts)
}

// CurrentMintableTokens is a free data retrieval call binding the contract method 0x9ae6309a.
//
// Solidity: function currentMintableTokens() constant returns(uint256)
func (_Minter *MinterCaller) CurrentMintableTokens(opts *bind.CallOpts) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _Minter.contract.Call(opts, out, "currentMintableTokens")
	return *ret0, err
}

// CurrentMintableTokens is a free data retrieval call binding the contract method 0x9ae6309a.
//
// Solidity: function currentMintableTokens() constant returns(uint256)
func (_Minter *MinterSession) CurrentMintableTokens() (*big.Int, error) {
	return _Minter.Contract.CurrentMintableTokens(&_Minter.CallOpts)
}

// CurrentMintableTokens is a free data retrieval call binding the contract method 0x9ae6309a.
//
// Solidity: function currentMintableTokens() constant returns(uint256)
func (_Minter *MinterCallerSession) CurrentMintableTokens() (*big.Int, error) {
	return _Minter.Contract.CurrentMintableTokens(&_Minter.CallOpts)
}

// CurrentMintedTokens is a free data retrieval call binding the contract method 0x2de22cdb.
//
// Solidity: function currentMintedTokens() constant returns(uint256)
func (_Minter *MinterCaller) CurrentMintedTokens(opts *bind.CallOpts) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _Minter.contract.Call(opts, out, "currentMintedTokens")
	return *ret0, err
}

// CurrentMintedTokens is a free data retrieval call binding the contract method 0x2de22cdb.
//
// Solidity: function currentMintedTokens() constant returns(uint256)
func (_Minter *MinterSession) CurrentMintedTokens() (*big.Int, error) {
	return _Minter.Contract.CurrentMintedTokens(&_Minter.CallOpts)
}

// CurrentMintedTokens is a free data retrieval call binding the contract method 0x2de22cdb.
//
// Solidity: function currentMintedTokens() constant returns(uint256)
func (_Minter *MinterCallerSession) CurrentMintedTokens() (*big.Int, error) {
	return _Minter.Contract.CurrentMintedTokens(&_Minter.CallOpts)
}

// Inflation is a free data retrieval call binding the contract method 0xbe0522e0.
//
// Solidity: function inflation() constant returns(uint256)
func (_Minter *MinterCaller) Inflation(opts *bind.CallOpts) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _Minter.contract.Call(opts, out, "inflation")
	return *ret0, err
}

// Inflation is a free data retrieval call binding the contract method 0xbe0522e0.
//
// Solidity: function inflation() constant returns(uint256)
func (_Minter *MinterSession) Inflation() (*big.Int, error) {
	return _Minter.Contract.Inflation(&_Minter.CallOpts)
}

// Inflation is a free data retrieval call binding the contract method 0xbe0522e0.
//
// Solidity: function inflation() constant returns(uint256)
func (_Minter *MinterCallerSession) Inflation() (*big.Int, error) {
	return _Minter.Contract.Inflation(&_Minter.CallOpts)
}

// InflationChange is a free data retrieval call binding the contract method 0xa7c83514.
//
// Solidity: function inflationChange() constant returns(uint256)
func (_Minter *MinterCaller) InflationChange(opts *bind.CallOpts) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _Minter.contract.Call(opts, out, "inflationChange")
	return *ret0, err
}

// InflationChange is a free data retrieval call binding the contract method 0xa7c83514.
//
// Solidity: function inflationChange() constant returns(uint256)
func (_Minter *MinterSession) InflationChange() (*big.Int, error) {
	return _Minter.Contract.InflationChange(&_Minter.CallOpts)
}

// InflationChange is a free data retrieval call binding the contract method 0xa7c83514.
//
// Solidity: function inflationChange() constant returns(uint256)
func (_Minter *MinterCallerSession) InflationChange() (*big.Int, error) {
	return _Minter.Contract.InflationChange(&_Minter.CallOpts)
}

// TargetBondingRate is a free data retrieval call binding the contract method 0x821b771f.
//
// Solidity: function targetBondingRate() constant returns(uint256)
func (_Minter *MinterCaller) TargetBondingRate(opts *bind.CallOpts) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _Minter.contract.Call(opts, out, "targetBondingRate")
	return *ret0, err
}

// TargetBondingRate is a free data retrieval call binding the contract method 0x821b771f.
//
// Solidity: function targetBondingRate() constant returns(uint256)
func (_Minter *MinterSession) TargetBondingRate() (*big.Int, error) {
	return _Minter.Contract.TargetBondingRate(&_Minter.CallOpts)
}

// TargetBondingRate is a free data retrieval call binding the contract method 0x821b771f.
//
// Solidity: function targetBondingRate() constant returns(uint256)
func (_Minter *MinterCallerSession) TargetBondingRate() (*big.Int, error) {
	return _Minter.Contract.TargetBondingRate(&_Minter.CallOpts)
}

// BurnTokens is a paid mutator transaction binding the contract method 0x6d1b229d.
//
// Solidity: function burnTokens(_amount uint256) returns()
func (_Minter *MinterTransactor) BurnTokens(opts *bind.TransactOpts, _amount *big.Int) (*types.Transaction, error) {
	return _Minter.contract.Transact(opts, "burnTokens", _amount)
}

// BurnTokens is a paid mutator transaction binding the contract method 0x6d1b229d.
//
// Solidity: function burnTokens(_amount uint256) returns()
func (_Minter *MinterSession) BurnTokens(_amount *big.Int) (*types.Transaction, error) {
	return _Minter.Contract.BurnTokens(&_Minter.TransactOpts, _amount)
}

// BurnTokens is a paid mutator transaction binding the contract method 0x6d1b229d.
//
// Solidity: function burnTokens(_amount uint256) returns()
func (_Minter *MinterTransactorSession) BurnTokens(_amount *big.Int) (*types.Transaction, error) {
	return _Minter.Contract.BurnTokens(&_Minter.TransactOpts, _amount)
}

// CreateReward is a paid mutator transaction binding the contract method 0x7dbedad5.
//
// Solidity: function createReward(_fracNum uint256, _fracDenom uint256) returns(uint256)
func (_Minter *MinterTransactor) CreateReward(opts *bind.TransactOpts, _fracNum *big.Int, _fracDenom *big.Int) (*types.Transaction, error) {
	return _Minter.contract.Transact(opts, "createReward", _fracNum, _fracDenom)
}

// CreateReward is a paid mutator transaction binding the contract method 0x7dbedad5.
//
// Solidity: function createReward(_fracNum uint256, _fracDenom uint256) returns(uint256)
func (_Minter *MinterSession) CreateReward(_fracNum *big.Int, _fracDenom *big.Int) (*types.Transaction, error) {
	return _Minter.Contract.CreateReward(&_Minter.TransactOpts, _fracNum, _fracDenom)
}

// CreateReward is a paid mutator transaction binding the contract method 0x7dbedad5.
//
// Solidity: function createReward(_fracNum uint256, _fracDenom uint256) returns(uint256)
func (_Minter *MinterTransactorSession) CreateReward(_fracNum *big.Int, _fracDenom *big.Int) (*types.Transaction, error) {
	return _Minter.Contract.CreateReward(&_Minter.TransactOpts, _fracNum, _fracDenom)
}

// DepositETH is a paid mutator transaction binding the contract method 0xf6326fb3.
//
// Solidity: function depositETH() returns(bool)
func (_Minter *MinterTransactor) DepositETH(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Minter.contract.Transact(opts, "depositETH")
}

// DepositETH is a paid mutator transaction binding the contract method 0xf6326fb3.
//
// Solidity: function depositETH() returns(bool)
func (_Minter *MinterSession) DepositETH() (*types.Transaction, error) {
	return _Minter.Contract.DepositETH(&_Minter.TransactOpts)
}

// DepositETH is a paid mutator transaction binding the contract method 0xf6326fb3.
//
// Solidity: function depositETH() returns(bool)
func (_Minter *MinterTransactorSession) DepositETH() (*types.Transaction, error) {
	return _Minter.Contract.DepositETH(&_Minter.TransactOpts)
}

// SetController is a paid mutator transaction binding the contract method 0x92eefe9b.
//
// Solidity: function setController(_controller address) returns()
func (_Minter *MinterTransactor) SetController(opts *bind.TransactOpts, _controller common.Address) (*types.Transaction, error) {
	return _Minter.contract.Transact(opts, "setController", _controller)
}

// SetController is a paid mutator transaction binding the contract method 0x92eefe9b.
//
// Solidity: function setController(_controller address) returns()
func (_Minter *MinterSession) SetController(_controller common.Address) (*types.Transaction, error) {
	return _Minter.Contract.SetController(&_Minter.TransactOpts, _controller)
}

// SetController is a paid mutator transaction binding the contract method 0x92eefe9b.
//
// Solidity: function setController(_controller address) returns()
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

// SetTargetBondingRate is a paid mutator transaction binding the contract method 0x77bde142.
//
// Solidity: function setTargetBondingRate(_targetBondingRate uint256) returns()
func (_Minter *MinterTransactor) SetTargetBondingRate(opts *bind.TransactOpts, _targetBondingRate *big.Int) (*types.Transaction, error) {
	return _Minter.contract.Transact(opts, "setTargetBondingRate", _targetBondingRate)
}

// SetTargetBondingRate is a paid mutator transaction binding the contract method 0x77bde142.
//
// Solidity: function setTargetBondingRate(_targetBondingRate uint256) returns()
func (_Minter *MinterSession) SetTargetBondingRate(_targetBondingRate *big.Int) (*types.Transaction, error) {
	return _Minter.Contract.SetTargetBondingRate(&_Minter.TransactOpts, _targetBondingRate)
}

// SetTargetBondingRate is a paid mutator transaction binding the contract method 0x77bde142.
//
// Solidity: function setTargetBondingRate(_targetBondingRate uint256) returns()
func (_Minter *MinterTransactorSession) SetTargetBondingRate(_targetBondingRate *big.Int) (*types.Transaction, error) {
	return _Minter.Contract.SetTargetBondingRate(&_Minter.TransactOpts, _targetBondingRate)
}

// TransferTokenOwnership is a paid mutator transaction binding the contract method 0x21e6b53d.
//
// Solidity: function transferTokenOwnership(_newOwner address) returns()
func (_Minter *MinterTransactor) TransferTokenOwnership(opts *bind.TransactOpts, _newOwner common.Address) (*types.Transaction, error) {
	return _Minter.contract.Transact(opts, "transferTokenOwnership", _newOwner)
}

// TransferTokenOwnership is a paid mutator transaction binding the contract method 0x21e6b53d.
//
// Solidity: function transferTokenOwnership(_newOwner address) returns()
func (_Minter *MinterSession) TransferTokenOwnership(_newOwner common.Address) (*types.Transaction, error) {
	return _Minter.Contract.TransferTokenOwnership(&_Minter.TransactOpts, _newOwner)
}

// TransferTokenOwnership is a paid mutator transaction binding the contract method 0x21e6b53d.
//
// Solidity: function transferTokenOwnership(_newOwner address) returns()
func (_Minter *MinterTransactorSession) TransferTokenOwnership(_newOwner common.Address) (*types.Transaction, error) {
	return _Minter.Contract.TransferTokenOwnership(&_Minter.TransactOpts, _newOwner)
}

// TransferTokens is a paid mutator transaction binding the contract method 0xbec3fa17.
//
// Solidity: function transferTokens(_to address, _amount uint256) returns()
func (_Minter *MinterTransactor) TransferTokens(opts *bind.TransactOpts, _to common.Address, _amount *big.Int) (*types.Transaction, error) {
	return _Minter.contract.Transact(opts, "transferTokens", _to, _amount)
}

// TransferTokens is a paid mutator transaction binding the contract method 0xbec3fa17.
//
// Solidity: function transferTokens(_to address, _amount uint256) returns()
func (_Minter *MinterSession) TransferTokens(_to common.Address, _amount *big.Int) (*types.Transaction, error) {
	return _Minter.Contract.TransferTokens(&_Minter.TransactOpts, _to, _amount)
}

// TransferTokens is a paid mutator transaction binding the contract method 0xbec3fa17.
//
// Solidity: function transferTokens(_to address, _amount uint256) returns()
func (_Minter *MinterTransactorSession) TransferTokens(_to common.Address, _amount *big.Int) (*types.Transaction, error) {
	return _Minter.Contract.TransferTokens(&_Minter.TransactOpts, _to, _amount)
}

// WithdrawETH is a paid mutator transaction binding the contract method 0x4782f779.
//
// Solidity: function withdrawETH(_to address, _amount uint256) returns()
func (_Minter *MinterTransactor) WithdrawETH(opts *bind.TransactOpts, _to common.Address, _amount *big.Int) (*types.Transaction, error) {
	return _Minter.contract.Transact(opts, "withdrawETH", _to, _amount)
}

// WithdrawETH is a paid mutator transaction binding the contract method 0x4782f779.
//
// Solidity: function withdrawETH(_to address, _amount uint256) returns()
func (_Minter *MinterSession) WithdrawETH(_to common.Address, _amount *big.Int) (*types.Transaction, error) {
	return _Minter.Contract.WithdrawETH(&_Minter.TransactOpts, _to, _amount)
}

// WithdrawETH is a paid mutator transaction binding the contract method 0x4782f779.
//
// Solidity: function withdrawETH(_to address, _amount uint256) returns()
func (_Minter *MinterTransactorSession) WithdrawETH(_to common.Address, _amount *big.Int) (*types.Transaction, error) {
	return _Minter.Contract.WithdrawETH(&_Minter.TransactOpts, _to, _amount)
}
