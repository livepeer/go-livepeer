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

// RoundsManagerMetaData contains all meta data concerning the RoundsManager contract.
var RoundsManagerMetaData = &bind.MetaData{
	ABI: "[{\"constant\":false,\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_roundLockAmount\",\"type\":\"uint256\"}],\"name\":\"setRoundLockAmount\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"lastRoundLengthUpdateRound\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_lip\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"_round\",\"type\":\"uint256\"}],\"name\":\"setLIPUpgradeRound\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"lipUpgradeRound\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"currentRoundInitialized\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_round\",\"type\":\"uint256\"}],\"name\":\"blockHashForRound\",\"outputs\":[{\"internalType\":\"bytes32\",\"name\":\"\",\"type\":\"bytes32\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"targetContractId\",\"outputs\":[{\"internalType\":\"bytes32\",\"name\":\"\",\"type\":\"bytes32\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"lastRoundLengthUpdateStartBlock\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_roundLength\",\"type\":\"uint256\"}],\"name\":\"setRoundLength\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"currentRoundLocked\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_block\",\"type\":\"uint256\"}],\"name\":\"blockHash\",\"outputs\":[{\"internalType\":\"bytes32\",\"name\":\"\",\"type\":\"bytes32\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"lastInitializedRound\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"currentRound\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"blockNum\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"roundLength\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"currentRoundStartBlock\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"internalType\":\"address\",\"name\":\"_controller\",\"type\":\"address\"}],\"name\":\"setController\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[],\"name\":\"initializeRound\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"roundLockAmount\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"controller\",\"outputs\":[{\"internalType\":\"contractIController\",\"name\":\"\",\"type\":\"address\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"_controller\",\"type\":\"address\"}],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"constructor\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"round\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"bytes32\",\"name\":\"blockHash\",\"type\":\"bytes32\"}],\"name\":\"NewRound\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"address\",\"name\":\"controller\",\"type\":\"address\"}],\"name\":\"SetController\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"string\",\"name\":\"param\",\"type\":\"string\"}],\"name\":\"ParameterUpdate\",\"type\":\"event\"}]",
}

// RoundsManagerABI is the input ABI used to generate the binding from.
// Deprecated: Use RoundsManagerMetaData.ABI instead.
var RoundsManagerABI = RoundsManagerMetaData.ABI

// RoundsManager is an auto generated Go binding around an Ethereum contract.
type RoundsManager struct {
	RoundsManagerCaller     // Read-only binding to the contract
	RoundsManagerTransactor // Write-only binding to the contract
	RoundsManagerFilterer   // Log filterer for contract events
}

// RoundsManagerCaller is an auto generated read-only Go binding around an Ethereum contract.
type RoundsManagerCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// RoundsManagerTransactor is an auto generated write-only Go binding around an Ethereum contract.
type RoundsManagerTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// RoundsManagerFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type RoundsManagerFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// RoundsManagerSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type RoundsManagerSession struct {
	Contract     *RoundsManager    // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// RoundsManagerCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type RoundsManagerCallerSession struct {
	Contract *RoundsManagerCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts        // Call options to use throughout this session
}

// RoundsManagerTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type RoundsManagerTransactorSession struct {
	Contract     *RoundsManagerTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts        // Transaction auth options to use throughout this session
}

// RoundsManagerRaw is an auto generated low-level Go binding around an Ethereum contract.
type RoundsManagerRaw struct {
	Contract *RoundsManager // Generic contract binding to access the raw methods on
}

// RoundsManagerCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type RoundsManagerCallerRaw struct {
	Contract *RoundsManagerCaller // Generic read-only contract binding to access the raw methods on
}

// RoundsManagerTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type RoundsManagerTransactorRaw struct {
	Contract *RoundsManagerTransactor // Generic write-only contract binding to access the raw methods on
}

// NewRoundsManager creates a new instance of RoundsManager, bound to a specific deployed contract.
func NewRoundsManager(address common.Address, backend bind.ContractBackend) (*RoundsManager, error) {
	contract, err := bindRoundsManager(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &RoundsManager{RoundsManagerCaller: RoundsManagerCaller{contract: contract}, RoundsManagerTransactor: RoundsManagerTransactor{contract: contract}, RoundsManagerFilterer: RoundsManagerFilterer{contract: contract}}, nil
}

// NewRoundsManagerCaller creates a new read-only instance of RoundsManager, bound to a specific deployed contract.
func NewRoundsManagerCaller(address common.Address, caller bind.ContractCaller) (*RoundsManagerCaller, error) {
	contract, err := bindRoundsManager(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &RoundsManagerCaller{contract: contract}, nil
}

// NewRoundsManagerTransactor creates a new write-only instance of RoundsManager, bound to a specific deployed contract.
func NewRoundsManagerTransactor(address common.Address, transactor bind.ContractTransactor) (*RoundsManagerTransactor, error) {
	contract, err := bindRoundsManager(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &RoundsManagerTransactor{contract: contract}, nil
}

// NewRoundsManagerFilterer creates a new log filterer instance of RoundsManager, bound to a specific deployed contract.
func NewRoundsManagerFilterer(address common.Address, filterer bind.ContractFilterer) (*RoundsManagerFilterer, error) {
	contract, err := bindRoundsManager(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &RoundsManagerFilterer{contract: contract}, nil
}

// bindRoundsManager binds a generic wrapper to an already deployed contract.
func bindRoundsManager(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := abi.JSON(strings.NewReader(RoundsManagerABI))
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_RoundsManager *RoundsManagerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _RoundsManager.Contract.RoundsManagerCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_RoundsManager *RoundsManagerRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _RoundsManager.Contract.RoundsManagerTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_RoundsManager *RoundsManagerRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _RoundsManager.Contract.RoundsManagerTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_RoundsManager *RoundsManagerCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _RoundsManager.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_RoundsManager *RoundsManagerTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _RoundsManager.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_RoundsManager *RoundsManagerTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _RoundsManager.Contract.contract.Transact(opts, method, params...)
}

// BlockHash is a free data retrieval call binding the contract method 0x85df51fd.
//
// Solidity: function blockHash(uint256 _block) view returns(bytes32)
func (_RoundsManager *RoundsManagerCaller) BlockHash(opts *bind.CallOpts, _block *big.Int) ([32]byte, error) {
	var out []interface{}
	err := _RoundsManager.contract.Call(opts, &out, "blockHash", _block)

	if err != nil {
		return *new([32]byte), err
	}

	out0 := *abi.ConvertType(out[0], new([32]byte)).(*[32]byte)

	return out0, err

}

// BlockHash is a free data retrieval call binding the contract method 0x85df51fd.
//
// Solidity: function blockHash(uint256 _block) view returns(bytes32)
func (_RoundsManager *RoundsManagerSession) BlockHash(_block *big.Int) ([32]byte, error) {
	return _RoundsManager.Contract.BlockHash(&_RoundsManager.CallOpts, _block)
}

// BlockHash is a free data retrieval call binding the contract method 0x85df51fd.
//
// Solidity: function blockHash(uint256 _block) view returns(bytes32)
func (_RoundsManager *RoundsManagerCallerSession) BlockHash(_block *big.Int) ([32]byte, error) {
	return _RoundsManager.Contract.BlockHash(&_RoundsManager.CallOpts, _block)
}

// BlockHashForRound is a free data retrieval call binding the contract method 0x3aa4add4.
//
// Solidity: function blockHashForRound(uint256 _round) view returns(bytes32)
func (_RoundsManager *RoundsManagerCaller) BlockHashForRound(opts *bind.CallOpts, _round *big.Int) ([32]byte, error) {
	var out []interface{}
	err := _RoundsManager.contract.Call(opts, &out, "blockHashForRound", _round)

	if err != nil {
		return *new([32]byte), err
	}

	out0 := *abi.ConvertType(out[0], new([32]byte)).(*[32]byte)

	return out0, err

}

// BlockHashForRound is a free data retrieval call binding the contract method 0x3aa4add4.
//
// Solidity: function blockHashForRound(uint256 _round) view returns(bytes32)
func (_RoundsManager *RoundsManagerSession) BlockHashForRound(_round *big.Int) ([32]byte, error) {
	return _RoundsManager.Contract.BlockHashForRound(&_RoundsManager.CallOpts, _round)
}

// BlockHashForRound is a free data retrieval call binding the contract method 0x3aa4add4.
//
// Solidity: function blockHashForRound(uint256 _round) view returns(bytes32)
func (_RoundsManager *RoundsManagerCallerSession) BlockHashForRound(_round *big.Int) ([32]byte, error) {
	return _RoundsManager.Contract.BlockHashForRound(&_RoundsManager.CallOpts, _round)
}

// BlockNum is a free data retrieval call binding the contract method 0x8ae63d6d.
//
// Solidity: function blockNum() view returns(uint256)
func (_RoundsManager *RoundsManagerCaller) BlockNum(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _RoundsManager.contract.Call(opts, &out, "blockNum")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// BlockNum is a free data retrieval call binding the contract method 0x8ae63d6d.
//
// Solidity: function blockNum() view returns(uint256)
func (_RoundsManager *RoundsManagerSession) BlockNum() (*big.Int, error) {
	return _RoundsManager.Contract.BlockNum(&_RoundsManager.CallOpts)
}

// BlockNum is a free data retrieval call binding the contract method 0x8ae63d6d.
//
// Solidity: function blockNum() view returns(uint256)
func (_RoundsManager *RoundsManagerCallerSession) BlockNum() (*big.Int, error) {
	return _RoundsManager.Contract.BlockNum(&_RoundsManager.CallOpts)
}

// Controller is a free data retrieval call binding the contract method 0xf77c4791.
//
// Solidity: function controller() view returns(address)
func (_RoundsManager *RoundsManagerCaller) Controller(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _RoundsManager.contract.Call(opts, &out, "controller")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// Controller is a free data retrieval call binding the contract method 0xf77c4791.
//
// Solidity: function controller() view returns(address)
func (_RoundsManager *RoundsManagerSession) Controller() (common.Address, error) {
	return _RoundsManager.Contract.Controller(&_RoundsManager.CallOpts)
}

// Controller is a free data retrieval call binding the contract method 0xf77c4791.
//
// Solidity: function controller() view returns(address)
func (_RoundsManager *RoundsManagerCallerSession) Controller() (common.Address, error) {
	return _RoundsManager.Contract.Controller(&_RoundsManager.CallOpts)
}

// CurrentRound is a free data retrieval call binding the contract method 0x8a19c8bc.
//
// Solidity: function currentRound() view returns(uint256)
func (_RoundsManager *RoundsManagerCaller) CurrentRound(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _RoundsManager.contract.Call(opts, &out, "currentRound")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// CurrentRound is a free data retrieval call binding the contract method 0x8a19c8bc.
//
// Solidity: function currentRound() view returns(uint256)
func (_RoundsManager *RoundsManagerSession) CurrentRound() (*big.Int, error) {
	return _RoundsManager.Contract.CurrentRound(&_RoundsManager.CallOpts)
}

// CurrentRound is a free data retrieval call binding the contract method 0x8a19c8bc.
//
// Solidity: function currentRound() view returns(uint256)
func (_RoundsManager *RoundsManagerCallerSession) CurrentRound() (*big.Int, error) {
	return _RoundsManager.Contract.CurrentRound(&_RoundsManager.CallOpts)
}

// CurrentRoundInitialized is a free data retrieval call binding the contract method 0x219bc76c.
//
// Solidity: function currentRoundInitialized() view returns(bool)
func (_RoundsManager *RoundsManagerCaller) CurrentRoundInitialized(opts *bind.CallOpts) (bool, error) {
	var out []interface{}
	err := _RoundsManager.contract.Call(opts, &out, "currentRoundInitialized")

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// CurrentRoundInitialized is a free data retrieval call binding the contract method 0x219bc76c.
//
// Solidity: function currentRoundInitialized() view returns(bool)
func (_RoundsManager *RoundsManagerSession) CurrentRoundInitialized() (bool, error) {
	return _RoundsManager.Contract.CurrentRoundInitialized(&_RoundsManager.CallOpts)
}

// CurrentRoundInitialized is a free data retrieval call binding the contract method 0x219bc76c.
//
// Solidity: function currentRoundInitialized() view returns(bool)
func (_RoundsManager *RoundsManagerCallerSession) CurrentRoundInitialized() (bool, error) {
	return _RoundsManager.Contract.CurrentRoundInitialized(&_RoundsManager.CallOpts)
}

// CurrentRoundLocked is a free data retrieval call binding the contract method 0x6841f253.
//
// Solidity: function currentRoundLocked() view returns(bool)
func (_RoundsManager *RoundsManagerCaller) CurrentRoundLocked(opts *bind.CallOpts) (bool, error) {
	var out []interface{}
	err := _RoundsManager.contract.Call(opts, &out, "currentRoundLocked")

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// CurrentRoundLocked is a free data retrieval call binding the contract method 0x6841f253.
//
// Solidity: function currentRoundLocked() view returns(bool)
func (_RoundsManager *RoundsManagerSession) CurrentRoundLocked() (bool, error) {
	return _RoundsManager.Contract.CurrentRoundLocked(&_RoundsManager.CallOpts)
}

// CurrentRoundLocked is a free data retrieval call binding the contract method 0x6841f253.
//
// Solidity: function currentRoundLocked() view returns(bool)
func (_RoundsManager *RoundsManagerCallerSession) CurrentRoundLocked() (bool, error) {
	return _RoundsManager.Contract.CurrentRoundLocked(&_RoundsManager.CallOpts)
}

// CurrentRoundStartBlock is a free data retrieval call binding the contract method 0x8fa148f2.
//
// Solidity: function currentRoundStartBlock() view returns(uint256)
func (_RoundsManager *RoundsManagerCaller) CurrentRoundStartBlock(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _RoundsManager.contract.Call(opts, &out, "currentRoundStartBlock")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// CurrentRoundStartBlock is a free data retrieval call binding the contract method 0x8fa148f2.
//
// Solidity: function currentRoundStartBlock() view returns(uint256)
func (_RoundsManager *RoundsManagerSession) CurrentRoundStartBlock() (*big.Int, error) {
	return _RoundsManager.Contract.CurrentRoundStartBlock(&_RoundsManager.CallOpts)
}

// CurrentRoundStartBlock is a free data retrieval call binding the contract method 0x8fa148f2.
//
// Solidity: function currentRoundStartBlock() view returns(uint256)
func (_RoundsManager *RoundsManagerCallerSession) CurrentRoundStartBlock() (*big.Int, error) {
	return _RoundsManager.Contract.CurrentRoundStartBlock(&_RoundsManager.CallOpts)
}

// LastInitializedRound is a free data retrieval call binding the contract method 0x8807f36e.
//
// Solidity: function lastInitializedRound() view returns(uint256)
func (_RoundsManager *RoundsManagerCaller) LastInitializedRound(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _RoundsManager.contract.Call(opts, &out, "lastInitializedRound")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// LastInitializedRound is a free data retrieval call binding the contract method 0x8807f36e.
//
// Solidity: function lastInitializedRound() view returns(uint256)
func (_RoundsManager *RoundsManagerSession) LastInitializedRound() (*big.Int, error) {
	return _RoundsManager.Contract.LastInitializedRound(&_RoundsManager.CallOpts)
}

// LastInitializedRound is a free data retrieval call binding the contract method 0x8807f36e.
//
// Solidity: function lastInitializedRound() view returns(uint256)
func (_RoundsManager *RoundsManagerCallerSession) LastInitializedRound() (*big.Int, error) {
	return _RoundsManager.Contract.LastInitializedRound(&_RoundsManager.CallOpts)
}

// LastRoundLengthUpdateRound is a free data retrieval call binding the contract method 0x0fe1dfa8.
//
// Solidity: function lastRoundLengthUpdateRound() view returns(uint256)
func (_RoundsManager *RoundsManagerCaller) LastRoundLengthUpdateRound(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _RoundsManager.contract.Call(opts, &out, "lastRoundLengthUpdateRound")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// LastRoundLengthUpdateRound is a free data retrieval call binding the contract method 0x0fe1dfa8.
//
// Solidity: function lastRoundLengthUpdateRound() view returns(uint256)
func (_RoundsManager *RoundsManagerSession) LastRoundLengthUpdateRound() (*big.Int, error) {
	return _RoundsManager.Contract.LastRoundLengthUpdateRound(&_RoundsManager.CallOpts)
}

// LastRoundLengthUpdateRound is a free data retrieval call binding the contract method 0x0fe1dfa8.
//
// Solidity: function lastRoundLengthUpdateRound() view returns(uint256)
func (_RoundsManager *RoundsManagerCallerSession) LastRoundLengthUpdateRound() (*big.Int, error) {
	return _RoundsManager.Contract.LastRoundLengthUpdateRound(&_RoundsManager.CallOpts)
}

// LastRoundLengthUpdateStartBlock is a free data retrieval call binding the contract method 0x668abff7.
//
// Solidity: function lastRoundLengthUpdateStartBlock() view returns(uint256)
func (_RoundsManager *RoundsManagerCaller) LastRoundLengthUpdateStartBlock(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _RoundsManager.contract.Call(opts, &out, "lastRoundLengthUpdateStartBlock")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// LastRoundLengthUpdateStartBlock is a free data retrieval call binding the contract method 0x668abff7.
//
// Solidity: function lastRoundLengthUpdateStartBlock() view returns(uint256)
func (_RoundsManager *RoundsManagerSession) LastRoundLengthUpdateStartBlock() (*big.Int, error) {
	return _RoundsManager.Contract.LastRoundLengthUpdateStartBlock(&_RoundsManager.CallOpts)
}

// LastRoundLengthUpdateStartBlock is a free data retrieval call binding the contract method 0x668abff7.
//
// Solidity: function lastRoundLengthUpdateStartBlock() view returns(uint256)
func (_RoundsManager *RoundsManagerCallerSession) LastRoundLengthUpdateStartBlock() (*big.Int, error) {
	return _RoundsManager.Contract.LastRoundLengthUpdateStartBlock(&_RoundsManager.CallOpts)
}

// LipUpgradeRound is a free data retrieval call binding the contract method 0x1e6d4c22.
//
// Solidity: function lipUpgradeRound(uint256 ) view returns(uint256)
func (_RoundsManager *RoundsManagerCaller) LipUpgradeRound(opts *bind.CallOpts, arg0 *big.Int) (*big.Int, error) {
	var out []interface{}
	err := _RoundsManager.contract.Call(opts, &out, "lipUpgradeRound", arg0)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// LipUpgradeRound is a free data retrieval call binding the contract method 0x1e6d4c22.
//
// Solidity: function lipUpgradeRound(uint256 ) view returns(uint256)
func (_RoundsManager *RoundsManagerSession) LipUpgradeRound(arg0 *big.Int) (*big.Int, error) {
	return _RoundsManager.Contract.LipUpgradeRound(&_RoundsManager.CallOpts, arg0)
}

// LipUpgradeRound is a free data retrieval call binding the contract method 0x1e6d4c22.
//
// Solidity: function lipUpgradeRound(uint256 ) view returns(uint256)
func (_RoundsManager *RoundsManagerCallerSession) LipUpgradeRound(arg0 *big.Int) (*big.Int, error) {
	return _RoundsManager.Contract.LipUpgradeRound(&_RoundsManager.CallOpts, arg0)
}

// RoundLength is a free data retrieval call binding the contract method 0x8b649b94.
//
// Solidity: function roundLength() view returns(uint256)
func (_RoundsManager *RoundsManagerCaller) RoundLength(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _RoundsManager.contract.Call(opts, &out, "roundLength")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// RoundLength is a free data retrieval call binding the contract method 0x8b649b94.
//
// Solidity: function roundLength() view returns(uint256)
func (_RoundsManager *RoundsManagerSession) RoundLength() (*big.Int, error) {
	return _RoundsManager.Contract.RoundLength(&_RoundsManager.CallOpts)
}

// RoundLength is a free data retrieval call binding the contract method 0x8b649b94.
//
// Solidity: function roundLength() view returns(uint256)
func (_RoundsManager *RoundsManagerCallerSession) RoundLength() (*big.Int, error) {
	return _RoundsManager.Contract.RoundLength(&_RoundsManager.CallOpts)
}

// RoundLockAmount is a free data retrieval call binding the contract method 0xf5b490d5.
//
// Solidity: function roundLockAmount() view returns(uint256)
func (_RoundsManager *RoundsManagerCaller) RoundLockAmount(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _RoundsManager.contract.Call(opts, &out, "roundLockAmount")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// RoundLockAmount is a free data retrieval call binding the contract method 0xf5b490d5.
//
// Solidity: function roundLockAmount() view returns(uint256)
func (_RoundsManager *RoundsManagerSession) RoundLockAmount() (*big.Int, error) {
	return _RoundsManager.Contract.RoundLockAmount(&_RoundsManager.CallOpts)
}

// RoundLockAmount is a free data retrieval call binding the contract method 0xf5b490d5.
//
// Solidity: function roundLockAmount() view returns(uint256)
func (_RoundsManager *RoundsManagerCallerSession) RoundLockAmount() (*big.Int, error) {
	return _RoundsManager.Contract.RoundLockAmount(&_RoundsManager.CallOpts)
}

// TargetContractId is a free data retrieval call binding the contract method 0x51720b41.
//
// Solidity: function targetContractId() view returns(bytes32)
func (_RoundsManager *RoundsManagerCaller) TargetContractId(opts *bind.CallOpts) ([32]byte, error) {
	var out []interface{}
	err := _RoundsManager.contract.Call(opts, &out, "targetContractId")

	if err != nil {
		return *new([32]byte), err
	}

	out0 := *abi.ConvertType(out[0], new([32]byte)).(*[32]byte)

	return out0, err

}

// TargetContractId is a free data retrieval call binding the contract method 0x51720b41.
//
// Solidity: function targetContractId() view returns(bytes32)
func (_RoundsManager *RoundsManagerSession) TargetContractId() ([32]byte, error) {
	return _RoundsManager.Contract.TargetContractId(&_RoundsManager.CallOpts)
}

// TargetContractId is a free data retrieval call binding the contract method 0x51720b41.
//
// Solidity: function targetContractId() view returns(bytes32)
func (_RoundsManager *RoundsManagerCallerSession) TargetContractId() ([32]byte, error) {
	return _RoundsManager.Contract.TargetContractId(&_RoundsManager.CallOpts)
}

// InitializeRound is a paid mutator transaction binding the contract method 0xd4807fb2.
//
// Solidity: function initializeRound() returns()
func (_RoundsManager *RoundsManagerTransactor) InitializeRound(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _RoundsManager.contract.Transact(opts, "initializeRound")
}

// InitializeRound is a paid mutator transaction binding the contract method 0xd4807fb2.
//
// Solidity: function initializeRound() returns()
func (_RoundsManager *RoundsManagerSession) InitializeRound() (*types.Transaction, error) {
	return _RoundsManager.Contract.InitializeRound(&_RoundsManager.TransactOpts)
}

// InitializeRound is a paid mutator transaction binding the contract method 0xd4807fb2.
//
// Solidity: function initializeRound() returns()
func (_RoundsManager *RoundsManagerTransactorSession) InitializeRound() (*types.Transaction, error) {
	return _RoundsManager.Contract.InitializeRound(&_RoundsManager.TransactOpts)
}

// SetController is a paid mutator transaction binding the contract method 0x92eefe9b.
//
// Solidity: function setController(address _controller) returns()
func (_RoundsManager *RoundsManagerTransactor) SetController(opts *bind.TransactOpts, _controller common.Address) (*types.Transaction, error) {
	return _RoundsManager.contract.Transact(opts, "setController", _controller)
}

// SetController is a paid mutator transaction binding the contract method 0x92eefe9b.
//
// Solidity: function setController(address _controller) returns()
func (_RoundsManager *RoundsManagerSession) SetController(_controller common.Address) (*types.Transaction, error) {
	return _RoundsManager.Contract.SetController(&_RoundsManager.TransactOpts, _controller)
}

// SetController is a paid mutator transaction binding the contract method 0x92eefe9b.
//
// Solidity: function setController(address _controller) returns()
func (_RoundsManager *RoundsManagerTransactorSession) SetController(_controller common.Address) (*types.Transaction, error) {
	return _RoundsManager.Contract.SetController(&_RoundsManager.TransactOpts, _controller)
}

// SetLIPUpgradeRound is a paid mutator transaction binding the contract method 0x13013e7a.
//
// Solidity: function setLIPUpgradeRound(uint256 _lip, uint256 _round) returns()
func (_RoundsManager *RoundsManagerTransactor) SetLIPUpgradeRound(opts *bind.TransactOpts, _lip *big.Int, _round *big.Int) (*types.Transaction, error) {
	return _RoundsManager.contract.Transact(opts, "setLIPUpgradeRound", _lip, _round)
}

// SetLIPUpgradeRound is a paid mutator transaction binding the contract method 0x13013e7a.
//
// Solidity: function setLIPUpgradeRound(uint256 _lip, uint256 _round) returns()
func (_RoundsManager *RoundsManagerSession) SetLIPUpgradeRound(_lip *big.Int, _round *big.Int) (*types.Transaction, error) {
	return _RoundsManager.Contract.SetLIPUpgradeRound(&_RoundsManager.TransactOpts, _lip, _round)
}

// SetLIPUpgradeRound is a paid mutator transaction binding the contract method 0x13013e7a.
//
// Solidity: function setLIPUpgradeRound(uint256 _lip, uint256 _round) returns()
func (_RoundsManager *RoundsManagerTransactorSession) SetLIPUpgradeRound(_lip *big.Int, _round *big.Int) (*types.Transaction, error) {
	return _RoundsManager.Contract.SetLIPUpgradeRound(&_RoundsManager.TransactOpts, _lip, _round)
}

// SetRoundLength is a paid mutator transaction binding the contract method 0x681312f5.
//
// Solidity: function setRoundLength(uint256 _roundLength) returns()
func (_RoundsManager *RoundsManagerTransactor) SetRoundLength(opts *bind.TransactOpts, _roundLength *big.Int) (*types.Transaction, error) {
	return _RoundsManager.contract.Transact(opts, "setRoundLength", _roundLength)
}

// SetRoundLength is a paid mutator transaction binding the contract method 0x681312f5.
//
// Solidity: function setRoundLength(uint256 _roundLength) returns()
func (_RoundsManager *RoundsManagerSession) SetRoundLength(_roundLength *big.Int) (*types.Transaction, error) {
	return _RoundsManager.Contract.SetRoundLength(&_RoundsManager.TransactOpts, _roundLength)
}

// SetRoundLength is a paid mutator transaction binding the contract method 0x681312f5.
//
// Solidity: function setRoundLength(uint256 _roundLength) returns()
func (_RoundsManager *RoundsManagerTransactorSession) SetRoundLength(_roundLength *big.Int) (*types.Transaction, error) {
	return _RoundsManager.Contract.SetRoundLength(&_RoundsManager.TransactOpts, _roundLength)
}

// SetRoundLockAmount is a paid mutator transaction binding the contract method 0x0b1573b8.
//
// Solidity: function setRoundLockAmount(uint256 _roundLockAmount) returns()
func (_RoundsManager *RoundsManagerTransactor) SetRoundLockAmount(opts *bind.TransactOpts, _roundLockAmount *big.Int) (*types.Transaction, error) {
	return _RoundsManager.contract.Transact(opts, "setRoundLockAmount", _roundLockAmount)
}

// SetRoundLockAmount is a paid mutator transaction binding the contract method 0x0b1573b8.
//
// Solidity: function setRoundLockAmount(uint256 _roundLockAmount) returns()
func (_RoundsManager *RoundsManagerSession) SetRoundLockAmount(_roundLockAmount *big.Int) (*types.Transaction, error) {
	return _RoundsManager.Contract.SetRoundLockAmount(&_RoundsManager.TransactOpts, _roundLockAmount)
}

// SetRoundLockAmount is a paid mutator transaction binding the contract method 0x0b1573b8.
//
// Solidity: function setRoundLockAmount(uint256 _roundLockAmount) returns()
func (_RoundsManager *RoundsManagerTransactorSession) SetRoundLockAmount(_roundLockAmount *big.Int) (*types.Transaction, error) {
	return _RoundsManager.Contract.SetRoundLockAmount(&_RoundsManager.TransactOpts, _roundLockAmount)
}

// RoundsManagerNewRoundIterator is returned from FilterNewRound and is used to iterate over the raw logs and unpacked data for NewRound events raised by the RoundsManager contract.
type RoundsManagerNewRoundIterator struct {
	Event *RoundsManagerNewRound // Event containing the contract specifics and raw log

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
func (it *RoundsManagerNewRoundIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RoundsManagerNewRound)
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
		it.Event = new(RoundsManagerNewRound)
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
func (it *RoundsManagerNewRoundIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RoundsManagerNewRoundIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RoundsManagerNewRound represents a NewRound event raised by the RoundsManager contract.
type RoundsManagerNewRound struct {
	Round     *big.Int
	BlockHash [32]byte
	Raw       types.Log // Blockchain specific contextual infos
}

// FilterNewRound is a free log retrieval operation binding the contract event 0x22f2fc17c5daf07db2379b3a03a8ef20a183f761097a58fce219c8a14619e786.
//
// Solidity: event NewRound(uint256 indexed round, bytes32 blockHash)
func (_RoundsManager *RoundsManagerFilterer) FilterNewRound(opts *bind.FilterOpts, round []*big.Int) (*RoundsManagerNewRoundIterator, error) {

	var roundRule []interface{}
	for _, roundItem := range round {
		roundRule = append(roundRule, roundItem)
	}

	logs, sub, err := _RoundsManager.contract.FilterLogs(opts, "NewRound", roundRule)
	if err != nil {
		return nil, err
	}
	return &RoundsManagerNewRoundIterator{contract: _RoundsManager.contract, event: "NewRound", logs: logs, sub: sub}, nil
}

// WatchNewRound is a free log subscription operation binding the contract event 0x22f2fc17c5daf07db2379b3a03a8ef20a183f761097a58fce219c8a14619e786.
//
// Solidity: event NewRound(uint256 indexed round, bytes32 blockHash)
func (_RoundsManager *RoundsManagerFilterer) WatchNewRound(opts *bind.WatchOpts, sink chan<- *RoundsManagerNewRound, round []*big.Int) (event.Subscription, error) {

	var roundRule []interface{}
	for _, roundItem := range round {
		roundRule = append(roundRule, roundItem)
	}

	logs, sub, err := _RoundsManager.contract.WatchLogs(opts, "NewRound", roundRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RoundsManagerNewRound)
				if err := _RoundsManager.contract.UnpackLog(event, "NewRound", log); err != nil {
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

// ParseNewRound is a log parse operation binding the contract event 0x22f2fc17c5daf07db2379b3a03a8ef20a183f761097a58fce219c8a14619e786.
//
// Solidity: event NewRound(uint256 indexed round, bytes32 blockHash)
func (_RoundsManager *RoundsManagerFilterer) ParseNewRound(log types.Log) (*RoundsManagerNewRound, error) {
	event := new(RoundsManagerNewRound)
	if err := _RoundsManager.contract.UnpackLog(event, "NewRound", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// RoundsManagerParameterUpdateIterator is returned from FilterParameterUpdate and is used to iterate over the raw logs and unpacked data for ParameterUpdate events raised by the RoundsManager contract.
type RoundsManagerParameterUpdateIterator struct {
	Event *RoundsManagerParameterUpdate // Event containing the contract specifics and raw log

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
func (it *RoundsManagerParameterUpdateIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RoundsManagerParameterUpdate)
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
		it.Event = new(RoundsManagerParameterUpdate)
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
func (it *RoundsManagerParameterUpdateIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RoundsManagerParameterUpdateIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RoundsManagerParameterUpdate represents a ParameterUpdate event raised by the RoundsManager contract.
type RoundsManagerParameterUpdate struct {
	Param string
	Raw   types.Log // Blockchain specific contextual infos
}

// FilterParameterUpdate is a free log retrieval operation binding the contract event 0x9f5033568d78ae30f29f01e944f97b2216493bd19d1b46d429673acff3dcd674.
//
// Solidity: event ParameterUpdate(string param)
func (_RoundsManager *RoundsManagerFilterer) FilterParameterUpdate(opts *bind.FilterOpts) (*RoundsManagerParameterUpdateIterator, error) {

	logs, sub, err := _RoundsManager.contract.FilterLogs(opts, "ParameterUpdate")
	if err != nil {
		return nil, err
	}
	return &RoundsManagerParameterUpdateIterator{contract: _RoundsManager.contract, event: "ParameterUpdate", logs: logs, sub: sub}, nil
}

// WatchParameterUpdate is a free log subscription operation binding the contract event 0x9f5033568d78ae30f29f01e944f97b2216493bd19d1b46d429673acff3dcd674.
//
// Solidity: event ParameterUpdate(string param)
func (_RoundsManager *RoundsManagerFilterer) WatchParameterUpdate(opts *bind.WatchOpts, sink chan<- *RoundsManagerParameterUpdate) (event.Subscription, error) {

	logs, sub, err := _RoundsManager.contract.WatchLogs(opts, "ParameterUpdate")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RoundsManagerParameterUpdate)
				if err := _RoundsManager.contract.UnpackLog(event, "ParameterUpdate", log); err != nil {
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
func (_RoundsManager *RoundsManagerFilterer) ParseParameterUpdate(log types.Log) (*RoundsManagerParameterUpdate, error) {
	event := new(RoundsManagerParameterUpdate)
	if err := _RoundsManager.contract.UnpackLog(event, "ParameterUpdate", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// RoundsManagerSetControllerIterator is returned from FilterSetController and is used to iterate over the raw logs and unpacked data for SetController events raised by the RoundsManager contract.
type RoundsManagerSetControllerIterator struct {
	Event *RoundsManagerSetController // Event containing the contract specifics and raw log

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
func (it *RoundsManagerSetControllerIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RoundsManagerSetController)
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
		it.Event = new(RoundsManagerSetController)
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
func (it *RoundsManagerSetControllerIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RoundsManagerSetControllerIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RoundsManagerSetController represents a SetController event raised by the RoundsManager contract.
type RoundsManagerSetController struct {
	Controller common.Address
	Raw        types.Log // Blockchain specific contextual infos
}

// FilterSetController is a free log retrieval operation binding the contract event 0x4ff638452bbf33c012645d18ae6f05515ff5f2d1dfb0cece8cbf018c60903f70.
//
// Solidity: event SetController(address controller)
func (_RoundsManager *RoundsManagerFilterer) FilterSetController(opts *bind.FilterOpts) (*RoundsManagerSetControllerIterator, error) {

	logs, sub, err := _RoundsManager.contract.FilterLogs(opts, "SetController")
	if err != nil {
		return nil, err
	}
	return &RoundsManagerSetControllerIterator{contract: _RoundsManager.contract, event: "SetController", logs: logs, sub: sub}, nil
}

// WatchSetController is a free log subscription operation binding the contract event 0x4ff638452bbf33c012645d18ae6f05515ff5f2d1dfb0cece8cbf018c60903f70.
//
// Solidity: event SetController(address controller)
func (_RoundsManager *RoundsManagerFilterer) WatchSetController(opts *bind.WatchOpts, sink chan<- *RoundsManagerSetController) (event.Subscription, error) {

	logs, sub, err := _RoundsManager.contract.WatchLogs(opts, "SetController")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RoundsManagerSetController)
				if err := _RoundsManager.contract.UnpackLog(event, "SetController", log); err != nil {
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
func (_RoundsManager *RoundsManagerFilterer) ParseSetController(log types.Log) (*RoundsManagerSetController, error) {
	event := new(RoundsManagerSetController)
	if err := _RoundsManager.contract.UnpackLog(event, "SetController", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}
