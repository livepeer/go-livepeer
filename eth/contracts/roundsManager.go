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

// RoundsManagerABI is the input ABI used to generate the binding from.
const RoundsManagerABI = "[{\"constant\":true,\"inputs\":[],\"name\":\"rewardCallsPerYear\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"currentRoundInitialized\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"blockTime\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"numActiveTranscoders\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"registry\",\"outputs\":[{\"name\":\"\",\"type\":\"address\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"lastInitializedRound\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"currentRound\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"roundLength\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"currentRoundStartBlock\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_registry\",\"type\":\"address\"}],\"name\":\"setRegistry\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":false,\"inputs\":[],\"name\":\"initializeRound\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"type\":\"function\"},{\"inputs\":[{\"name\":\"_registry\",\"type\":\"address\"}],\"payable\":false,\"type\":\"constructor\"}]"

// RoundsManagerBin is the compiled bytecode used for deploying new contracts.
const RoundsManagerBin = `0x6060604052341561000f57600080fd5b6040516020806106a8833981016040528080519150505b805b60008054600160a060020a031916600160a060020a0383161790555b50600180805560326002556004556100676401000000006102ca61007182021704565b6003555b506100b1565b60025460009061008f9043906401000000006104cf61009582021704565b90505b90565b60008082848115156100a357fe5b0490508091505b5092915050565b6105e8806100c06000396000f300606060405236156100935763ffffffff60e060020a6000350416630a71fde48114610098578063219bc76c146100bd57806348b15166146100e457806361e25d23146101095780637b1039991461012e5780638807f36e1461015d5780638a19c8bc146101825780638b649b94146101a75780638fa148f2146101cc578063a91ee0dc146101f1578063d4807fb214610224575b600080fd5b34156100a357600080fd5b6100ab61024b565b60405190815260200160405180910390f35b34156100c857600080fd5b6100d0610295565b604051901515815260200160405180910390f35b34156100ef57600080fd5b6100ab6102a9565b60405190815260200160405180910390f35b341561011457600080fd5b6100ab6102af565b60405190815260200160405180910390f35b341561013957600080fd5b6101416102b5565b604051600160a060020a03909116815260200160405180910390f35b341561016857600080fd5b6100ab6102c4565b60405190815260200160405180910390f35b341561018d57600080fd5b6100ab6102ca565b60405190815260200160405180910390f35b34156101b257600080fd5b6100ab6102e7565b60405190815260200160405180910390f35b34156101d757600080fd5b6100ab6102ed565b60405190815260200160405180910390f35b34156101fc57600080fd5b6100d0600160a060020a036004351661030f565b604051901515815260200160405180910390f35b341561022f57600080fd5b6100d06103cd565b604051901515815260200160405180910390f35b6000806301e13380905061028e600454610282600254610276600154866104cf90919063ffffffff16565b9063ffffffff6104cf16565b9063ffffffff6104eb16565b91505b5090565b600061029f6102ca565b6003541490505b90565b60015481565b60045481565b600054600160a060020a031681565b60035481565b60006102e1600254436104cf90919063ffffffff16565b90505b90565b60025481565b60006102e16002546102826102ca565b9063ffffffff6104eb16565b90505b90565b6000805433600160a060020a0390811691161461032b57600080fd5b60008054600160a060020a031690635c975abb90604051602001526040518163ffffffff1660e060020a028152600401602060405180830381600087803b151561037457600080fd5b6102c65a03f1151561038557600080fd5b50505060405180519050151561039a57600080fd5b506000805473ffffffffffffffffffffffffffffffffffffffff1916600160a060020a03831617905560015b5b5b919050565b60008054600160a060020a0316635c975abb82604051602001526040518163ffffffff1660e060020a028152600401602060405180830381600087803b151561041557600080fd5b6102c65a03f1151561042657600080fd5b505050604051805115905061043a57600080fd5b6104426102ca565b6003541415610453575060006102a6565b61045b6102ca565b60035561046661051a565b600160a060020a031663242ed69f6000604051602001526040518163ffffffff1660e060020a028152600401602060405180830381600087803b15156104ab57600080fd5b6102c65a03f115156104bc57600080fd5b50505060405180515060019150505b5b90565b60008082848115156104dd57fe5b0490508091505b5092915050565b6000828202831580610507575082848281151561050457fe5b04145b151561050f57fe5b8091505b5092915050565b60008054600160a060020a0316637ef502986040517f426f6e64696e674d616e616765720000000000000000000000000000000000008152600e01604051809103902060006040516020015260405160e060020a63ffffffff84160281526004810191909152602401602060405180830381600087803b151561059c57600080fd5b6102c65a03f115156105ad57600080fd5b50505060405180519150505b905600a165627a7a72305820e447f58ac91b03c899cdec9ff7a1404c647294084f9c2b63fab09e9a953a81920029`

// DeployRoundsManager deploys a new Ethereum contract, binding an instance of RoundsManager to it.
func DeployRoundsManager(auth *bind.TransactOpts, backend bind.ContractBackend, libraries map[string]common.Address, _registry common.Address) (common.Address, *types.Transaction, *RoundsManager, error) {
	parsed, err := abi.JSON(strings.NewReader(RoundsManagerABI))
	if err != nil {
		return common.Address{}, nil, nil, err
	}

	linkedBin := RoundsManagerBin
	for lib, addr := range libraries {
		reg, err := regexp.Compile(fmt.Sprintf("_+%s_+", lib))
		if err != nil {
			return common.Address{}, nil, nil, err
		}

		linkedBin = reg.ReplaceAllString(linkedBin, addr.Hex()[2:])
	}

	address, tx, contract, err := bind.DeployContract(auth, parsed, common.FromHex(linkedBin), backend, _registry)
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	return address, tx, &RoundsManager{RoundsManagerCaller: RoundsManagerCaller{contract: contract}, RoundsManagerTransactor: RoundsManagerTransactor{contract: contract}}, nil
}

// RoundsManager is an auto generated Go binding around an Ethereum contract.
type RoundsManager struct {
	RoundsManagerCaller     // Read-only binding to the contract
	RoundsManagerTransactor // Write-only binding to the contract
}

// RoundsManagerCaller is an auto generated read-only Go binding around an Ethereum contract.
type RoundsManagerCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// RoundsManagerTransactor is an auto generated write-only Go binding around an Ethereum contract.
type RoundsManagerTransactor struct {
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
	contract, err := bindRoundsManager(address, backend, backend)
	if err != nil {
		return nil, err
	}
	return &RoundsManager{RoundsManagerCaller: RoundsManagerCaller{contract: contract}, RoundsManagerTransactor: RoundsManagerTransactor{contract: contract}}, nil
}

// NewRoundsManagerCaller creates a new read-only instance of RoundsManager, bound to a specific deployed contract.
func NewRoundsManagerCaller(address common.Address, caller bind.ContractCaller) (*RoundsManagerCaller, error) {
	contract, err := bindRoundsManager(address, caller, nil)
	if err != nil {
		return nil, err
	}
	return &RoundsManagerCaller{contract: contract}, nil
}

// NewRoundsManagerTransactor creates a new write-only instance of RoundsManager, bound to a specific deployed contract.
func NewRoundsManagerTransactor(address common.Address, transactor bind.ContractTransactor) (*RoundsManagerTransactor, error) {
	contract, err := bindRoundsManager(address, nil, transactor)
	if err != nil {
		return nil, err
	}
	return &RoundsManagerTransactor{contract: contract}, nil
}

// bindRoundsManager binds a generic wrapper to an already deployed contract.
func bindRoundsManager(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor) (*bind.BoundContract, error) {
	parsed, err := abi.JSON(strings.NewReader(RoundsManagerABI))
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, parsed, caller, transactor), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_RoundsManager *RoundsManagerRaw) Call(opts *bind.CallOpts, result interface{}, method string, params ...interface{}) error {
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
func (_RoundsManager *RoundsManagerCallerRaw) Call(opts *bind.CallOpts, result interface{}, method string, params ...interface{}) error {
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

// BlockTime is a free data retrieval call binding the contract method 0x48b15166.
//
// Solidity: function blockTime() constant returns(uint256)
func (_RoundsManager *RoundsManagerCaller) BlockTime(opts *bind.CallOpts) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _RoundsManager.contract.Call(opts, out, "blockTime")
	return *ret0, err
}

// BlockTime is a free data retrieval call binding the contract method 0x48b15166.
//
// Solidity: function blockTime() constant returns(uint256)
func (_RoundsManager *RoundsManagerSession) BlockTime() (*big.Int, error) {
	return _RoundsManager.Contract.BlockTime(&_RoundsManager.CallOpts)
}

// BlockTime is a free data retrieval call binding the contract method 0x48b15166.
//
// Solidity: function blockTime() constant returns(uint256)
func (_RoundsManager *RoundsManagerCallerSession) BlockTime() (*big.Int, error) {
	return _RoundsManager.Contract.BlockTime(&_RoundsManager.CallOpts)
}

// CurrentRound is a free data retrieval call binding the contract method 0x8a19c8bc.
//
// Solidity: function currentRound() constant returns(uint256)
func (_RoundsManager *RoundsManagerCaller) CurrentRound(opts *bind.CallOpts) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _RoundsManager.contract.Call(opts, out, "currentRound")
	return *ret0, err
}

// CurrentRound is a free data retrieval call binding the contract method 0x8a19c8bc.
//
// Solidity: function currentRound() constant returns(uint256)
func (_RoundsManager *RoundsManagerSession) CurrentRound() (*big.Int, error) {
	return _RoundsManager.Contract.CurrentRound(&_RoundsManager.CallOpts)
}

// CurrentRound is a free data retrieval call binding the contract method 0x8a19c8bc.
//
// Solidity: function currentRound() constant returns(uint256)
func (_RoundsManager *RoundsManagerCallerSession) CurrentRound() (*big.Int, error) {
	return _RoundsManager.Contract.CurrentRound(&_RoundsManager.CallOpts)
}

// CurrentRoundInitialized is a free data retrieval call binding the contract method 0x219bc76c.
//
// Solidity: function currentRoundInitialized() constant returns(bool)
func (_RoundsManager *RoundsManagerCaller) CurrentRoundInitialized(opts *bind.CallOpts) (bool, error) {
	var (
		ret0 = new(bool)
	)
	out := ret0
	err := _RoundsManager.contract.Call(opts, out, "currentRoundInitialized")
	return *ret0, err
}

// CurrentRoundInitialized is a free data retrieval call binding the contract method 0x219bc76c.
//
// Solidity: function currentRoundInitialized() constant returns(bool)
func (_RoundsManager *RoundsManagerSession) CurrentRoundInitialized() (bool, error) {
	return _RoundsManager.Contract.CurrentRoundInitialized(&_RoundsManager.CallOpts)
}

// CurrentRoundInitialized is a free data retrieval call binding the contract method 0x219bc76c.
//
// Solidity: function currentRoundInitialized() constant returns(bool)
func (_RoundsManager *RoundsManagerCallerSession) CurrentRoundInitialized() (bool, error) {
	return _RoundsManager.Contract.CurrentRoundInitialized(&_RoundsManager.CallOpts)
}

// CurrentRoundStartBlock is a free data retrieval call binding the contract method 0x8fa148f2.
//
// Solidity: function currentRoundStartBlock() constant returns(uint256)
func (_RoundsManager *RoundsManagerCaller) CurrentRoundStartBlock(opts *bind.CallOpts) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _RoundsManager.contract.Call(opts, out, "currentRoundStartBlock")
	return *ret0, err
}

// CurrentRoundStartBlock is a free data retrieval call binding the contract method 0x8fa148f2.
//
// Solidity: function currentRoundStartBlock() constant returns(uint256)
func (_RoundsManager *RoundsManagerSession) CurrentRoundStartBlock() (*big.Int, error) {
	return _RoundsManager.Contract.CurrentRoundStartBlock(&_RoundsManager.CallOpts)
}

// CurrentRoundStartBlock is a free data retrieval call binding the contract method 0x8fa148f2.
//
// Solidity: function currentRoundStartBlock() constant returns(uint256)
func (_RoundsManager *RoundsManagerCallerSession) CurrentRoundStartBlock() (*big.Int, error) {
	return _RoundsManager.Contract.CurrentRoundStartBlock(&_RoundsManager.CallOpts)
}

// LastInitializedRound is a free data retrieval call binding the contract method 0x8807f36e.
//
// Solidity: function lastInitializedRound() constant returns(uint256)
func (_RoundsManager *RoundsManagerCaller) LastInitializedRound(opts *bind.CallOpts) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _RoundsManager.contract.Call(opts, out, "lastInitializedRound")
	return *ret0, err
}

// LastInitializedRound is a free data retrieval call binding the contract method 0x8807f36e.
//
// Solidity: function lastInitializedRound() constant returns(uint256)
func (_RoundsManager *RoundsManagerSession) LastInitializedRound() (*big.Int, error) {
	return _RoundsManager.Contract.LastInitializedRound(&_RoundsManager.CallOpts)
}

// LastInitializedRound is a free data retrieval call binding the contract method 0x8807f36e.
//
// Solidity: function lastInitializedRound() constant returns(uint256)
func (_RoundsManager *RoundsManagerCallerSession) LastInitializedRound() (*big.Int, error) {
	return _RoundsManager.Contract.LastInitializedRound(&_RoundsManager.CallOpts)
}

// NumActiveTranscoders is a free data retrieval call binding the contract method 0x61e25d23.
//
// Solidity: function numActiveTranscoders() constant returns(uint256)
func (_RoundsManager *RoundsManagerCaller) NumActiveTranscoders(opts *bind.CallOpts) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _RoundsManager.contract.Call(opts, out, "numActiveTranscoders")
	return *ret0, err
}

// NumActiveTranscoders is a free data retrieval call binding the contract method 0x61e25d23.
//
// Solidity: function numActiveTranscoders() constant returns(uint256)
func (_RoundsManager *RoundsManagerSession) NumActiveTranscoders() (*big.Int, error) {
	return _RoundsManager.Contract.NumActiveTranscoders(&_RoundsManager.CallOpts)
}

// NumActiveTranscoders is a free data retrieval call binding the contract method 0x61e25d23.
//
// Solidity: function numActiveTranscoders() constant returns(uint256)
func (_RoundsManager *RoundsManagerCallerSession) NumActiveTranscoders() (*big.Int, error) {
	return _RoundsManager.Contract.NumActiveTranscoders(&_RoundsManager.CallOpts)
}

// Registry is a free data retrieval call binding the contract method 0x7b103999.
//
// Solidity: function registry() constant returns(address)
func (_RoundsManager *RoundsManagerCaller) Registry(opts *bind.CallOpts) (common.Address, error) {
	var (
		ret0 = new(common.Address)
	)
	out := ret0
	err := _RoundsManager.contract.Call(opts, out, "registry")
	return *ret0, err
}

// Registry is a free data retrieval call binding the contract method 0x7b103999.
//
// Solidity: function registry() constant returns(address)
func (_RoundsManager *RoundsManagerSession) Registry() (common.Address, error) {
	return _RoundsManager.Contract.Registry(&_RoundsManager.CallOpts)
}

// Registry is a free data retrieval call binding the contract method 0x7b103999.
//
// Solidity: function registry() constant returns(address)
func (_RoundsManager *RoundsManagerCallerSession) Registry() (common.Address, error) {
	return _RoundsManager.Contract.Registry(&_RoundsManager.CallOpts)
}

// RewardCallsPerYear is a free data retrieval call binding the contract method 0x0a71fde4.
//
// Solidity: function rewardCallsPerYear() constant returns(uint256)
func (_RoundsManager *RoundsManagerCaller) RewardCallsPerYear(opts *bind.CallOpts) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _RoundsManager.contract.Call(opts, out, "rewardCallsPerYear")
	return *ret0, err
}

// RewardCallsPerYear is a free data retrieval call binding the contract method 0x0a71fde4.
//
// Solidity: function rewardCallsPerYear() constant returns(uint256)
func (_RoundsManager *RoundsManagerSession) RewardCallsPerYear() (*big.Int, error) {
	return _RoundsManager.Contract.RewardCallsPerYear(&_RoundsManager.CallOpts)
}

// RewardCallsPerYear is a free data retrieval call binding the contract method 0x0a71fde4.
//
// Solidity: function rewardCallsPerYear() constant returns(uint256)
func (_RoundsManager *RoundsManagerCallerSession) RewardCallsPerYear() (*big.Int, error) {
	return _RoundsManager.Contract.RewardCallsPerYear(&_RoundsManager.CallOpts)
}

// RoundLength is a free data retrieval call binding the contract method 0x8b649b94.
//
// Solidity: function roundLength() constant returns(uint256)
func (_RoundsManager *RoundsManagerCaller) RoundLength(opts *bind.CallOpts) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _RoundsManager.contract.Call(opts, out, "roundLength")
	return *ret0, err
}

// RoundLength is a free data retrieval call binding the contract method 0x8b649b94.
//
// Solidity: function roundLength() constant returns(uint256)
func (_RoundsManager *RoundsManagerSession) RoundLength() (*big.Int, error) {
	return _RoundsManager.Contract.RoundLength(&_RoundsManager.CallOpts)
}

// RoundLength is a free data retrieval call binding the contract method 0x8b649b94.
//
// Solidity: function roundLength() constant returns(uint256)
func (_RoundsManager *RoundsManagerCallerSession) RoundLength() (*big.Int, error) {
	return _RoundsManager.Contract.RoundLength(&_RoundsManager.CallOpts)
}

// InitializeRound is a paid mutator transaction binding the contract method 0xd4807fb2.
//
// Solidity: function initializeRound() returns(bool)
func (_RoundsManager *RoundsManagerTransactor) InitializeRound(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _RoundsManager.contract.Transact(opts, "initializeRound")
}

// InitializeRound is a paid mutator transaction binding the contract method 0xd4807fb2.
//
// Solidity: function initializeRound() returns(bool)
func (_RoundsManager *RoundsManagerSession) InitializeRound() (*types.Transaction, error) {
	return _RoundsManager.Contract.InitializeRound(&_RoundsManager.TransactOpts)
}

// InitializeRound is a paid mutator transaction binding the contract method 0xd4807fb2.
//
// Solidity: function initializeRound() returns(bool)
func (_RoundsManager *RoundsManagerTransactorSession) InitializeRound() (*types.Transaction, error) {
	return _RoundsManager.Contract.InitializeRound(&_RoundsManager.TransactOpts)
}

// SetRegistry is a paid mutator transaction binding the contract method 0xa91ee0dc.
//
// Solidity: function setRegistry(_registry address) returns(bool)
func (_RoundsManager *RoundsManagerTransactor) SetRegistry(opts *bind.TransactOpts, _registry common.Address) (*types.Transaction, error) {
	return _RoundsManager.contract.Transact(opts, "setRegistry", _registry)
}

// SetRegistry is a paid mutator transaction binding the contract method 0xa91ee0dc.
//
// Solidity: function setRegistry(_registry address) returns(bool)
func (_RoundsManager *RoundsManagerSession) SetRegistry(_registry common.Address) (*types.Transaction, error) {
	return _RoundsManager.Contract.SetRegistry(&_RoundsManager.TransactOpts, _registry)
}

// SetRegistry is a paid mutator transaction binding the contract method 0xa91ee0dc.
//
// Solidity: function setRegistry(_registry address) returns(bool)
func (_RoundsManager *RoundsManagerTransactorSession) SetRegistry(_registry common.Address) (*types.Transaction, error) {
	return _RoundsManager.Contract.SetRegistry(&_RoundsManager.TransactOpts, _registry)
}
