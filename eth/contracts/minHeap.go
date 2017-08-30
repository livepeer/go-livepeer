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

// MinHeapABI is the input ABI used to generate the binding from.
const MinHeapABI = "[{\"constant\":false,\"inputs\":[{\"name\":\"self\",\"type\":\"MinHeap.Heapstorage\"},{\"name\":\"_id\",\"type\":\"address\"}],\"name\":\"deleteId\",\"outputs\":[],\"payable\":false,\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"self\",\"type\":\"MinHeap.Heapstorage\"},{\"name\":\"_id\",\"type\":\"address\"},{\"name\":\"_amount\",\"type\":\"uint256\"}],\"name\":\"decreaseKey\",\"outputs\":[],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"self\",\"type\":\"MinHeap.Heapstorage\"}],\"name\":\"isEmpty\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"self\",\"type\":\"MinHeap.Heapstorage\"},{\"name\":\"_id\",\"type\":\"address\"},{\"name\":\"_amount\",\"type\":\"uint256\"}],\"name\":\"increaseKey\",\"outputs\":[],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"self\",\"type\":\"MinHeap.Heapstorage\"}],\"name\":\"size\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"self\",\"type\":\"MinHeap.Heapstorage\"},{\"name\":\"_pos\",\"type\":\"uint256\"}],\"name\":\"deletePos\",\"outputs\":[],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"self\",\"type\":\"MinHeap.Heapstorage\"},{\"name\":\"_id\",\"type\":\"address\"}],\"name\":\"getKey\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"self\",\"type\":\"MinHeap.Heapstorage\"},{\"name\":\"_size\",\"type\":\"uint256\"}],\"name\":\"init\",\"outputs\":[],\"payable\":false,\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"self\",\"type\":\"MinHeap.Heapstorage\"}],\"name\":\"extractMin\",\"outputs\":[],\"payable\":false,\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"self\",\"type\":\"MinHeap.Heapstorage\"},{\"name\":\"_id\",\"type\":\"address\"},{\"name\":\"_key\",\"type\":\"uint256\"}],\"name\":\"insert\",\"outputs\":[],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"self\",\"type\":\"MinHeap.Heapstorage\"}],\"name\":\"min\",\"outputs\":[{\"name\":\"\",\"type\":\"address\"},{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"self\",\"type\":\"MinHeap.Heapstorage\"}],\"name\":\"isFull\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"self\",\"type\":\"MinHeap.Heapstorage\"},{\"name\":\"_id\",\"type\":\"address\"}],\"name\":\"contains\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"type\":\"function\"}]"

// MinHeapBin is the compiled bytecode used for deploying new contracts.
const MinHeapBin = `0x6060604052341561000f57600080fd5b5b610e328061001f6000396000f300606060405236156100c25763ffffffff7c01000000000000000000000000000000000000000000000000000000006000350416630161e01281146100c75780630c10bda3146100e05780630fec2c17146100fc5780631a8255cd1461011b578063383b8403146101375780633caee0111461015457806360fbb88e1461016457806380f569d41461018d5780639c9435231461019d578063cffd134a146101aa578063e0e7c2a6146101c6578063f98b8071146101f3578063ff73bd3b14610212575b600080fd5b6100de600435600160a060020a036024351661023d565b005b6100de600435600160a060020a0360243516604435610291565b005b610107600435610341565b604051901515815260200160405180910390f35b6100de600435600160a060020a036024351660443561034a565b005b6101426004356103fa565b60405190815260200160405180910390f35b6100de600435602435610402565b005b610142600435600160a060020a03602435166105a1565b60405190815260200160405180910390f35b6100de600435602435610611565b005b6100de60043561063e565b005b6100de600435600160a060020a036024351660443561065c565b005b6101d1600435610771565b604051600160a060020a03909216825260208201526040908101905180910390f35b6101076004356107e1565b604051901515815260200160405180910390f35b610107600435600160a060020a03602435166107ef565b604051901515815260200160405180910390f35b600160a060020a038116600090815260028301602052604090205460ff16151561026657600080fd5b600160a060020a038116600090815260018301602052604090205461028c908390610402565b5b5050565b600160a060020a038216600090815260028401602052604081205460ff1615156102ba57600080fd5b50600160a060020a0382166000908152600184016020526040902054835461030c9083908690849081106102ea57fe5b906000526020600020906003020160005b50600101549063ffffffff61081416565b845485908390811061031a57fe5b906000526020600020906003020160005b506001015561033a848261082b565b5b50505050565b8054155b919050565b600160a060020a038216600090815260028401602052604081205460ff16151561037357600080fd5b50600160a060020a038216600090815260018401602052604090205483546103c59083908690849081106103a357fe5b906000526020600020906003020160005b50600101549063ffffffff610a3d16565b84548590839081106103d357fe5b906000526020600020906003020160005b506001015561033a8482610a57565b5b50505050565b80545b919050565b81548190101561041157600080fd5b6000826002016000846000018481548110151561042a57fe5b906000526020600020906003020160005b5054600160a060020a031681526020810191909152604001600020805460ff191691151591909117905581548290600019810190811061047757fe5b906000526020600020906003020160005b50825483908390811061049757fe5b906000526020600020906003020160005b5081548154600160a060020a031916600160a060020a039091161781556001808301548282015560029283015492909101805460ff909316151560ff19909316929092179091558254829184019060009085908490811061050557fe5b906000526020600020906003020160005b5054600160a060020a0316815260208101919091526040016000205581548290600019810190811061054457fe5b906000526020600020906003020160005b508054600160a060020a031916815560006001820155600201805460ff191690558154610586836000198301610d44565b5081548190111561028c5761028c8282610a57565b5b5b5050565b600160a060020a038116600090815260028301602052604081205460ff1615156105ca57600080fd5b600160a060020a0382166000908152600184016020526040902054835484919081106105f257fe5b906000526020600020906003020160005b506001015490505b92915050565b600482015460ff161561062357600080fd5b6003820181905560048201805460ff191660011790555b5050565b80546000901161064d57600080fd5b610658816000610402565b5b50565b60038301548354141561066e57600080fd5b600160a060020a038216600090815260028401602052604090205460ff161561069657600080fd5b82548390600181016106a88382610d44565b916000526020600020906003020160005b60606040519081016040908152600160a060020a03871682526020820186905260019082015291905081518154600160a060020a031916600160a060020a03919091161781556020820151816001015560408201516002918201805491151560ff199283161790558654600160a060020a03871660009081526001898101602090815260408084206000199586019055958b01905293902080549092169092179055855461076b93508692500161082b565b5b505050565b8054600090819081901161078457600080fd5b82548390600090811061079357fe5b906000526020600020906003020160005b50548354600160a060020a0390911690849060009081106107c157fe5b906000526020600020906003020160005b5060010154915091505b915091565b60038101548154145b919050565b600160a060020a038116600090815260028301602052604090205460ff165b92915050565b60008282111561082057fe5b508082035b92915050565b610833610da8565b825483908390811061084157fe5b906000526020600020906003020160005b50606060405190810160409081528254600160a060020a031682526001830154602083015260029092015460ff1615159181019190915290505b6000821180156108ca5750806020015183600260001985015b048154811015156108b257fe5b906000526020600020906003020160005b5060010154115b156109b05782600260001984015b048154811015156108e557fe5b906000526020600020906003020160005b50835484908490811061090557fe5b906000526020600020906003020160005b5081548154600160a060020a031916600160a060020a039091161781556001808301548282015560029283015492909101805460ff909316151560ff19909316929092179091558354839185019060009086908490811061097357fe5b906000526020600020906003020160005b5054600160a060020a03168152602081019190915260400160002055600260001983015b04915061088c565b825481908490849081106109c057fe5b906000526020600020906003020160005b5081518154600160a060020a031916600160a060020a0391909116178155602082015181600101556040820151600291909101805460ff191691151591909117905550816001840160008351600160a060020a031681526020810191909152604001600020555b505050565b600082820183811015610a4c57fe5b8091505b5092915050565b610a5f610da8565b6000808460000184815481101515610a7357fe5b906000526020600020906003020160005b50606060405190810160409081528254600160a060020a03168252600180840154602084015260029384015460ff16151591830191909152909450600093509085020190505b845481108015610ad8575081155b15610d3c57845460018201108015610b1657508454859060018301908110610afc57fe5b906000526020600020906003020160005b506002015460ff165b8015610b6d57508454859082908110610b2b57fe5b906000526020600020906003020160005b50600101548560000182600101815481101515610b5557fe5b906000526020600020906003020160005b5060010154105b15610b76576001015b8454859082908110610b8457fe5b906000526020600020906003020160005b506002015460ff168015610bcf575082602001518554869083908110610bb757fe5b906000526020600020906003020160005b5060010154105b15610cab578454859082908110610be257fe5b906000526020600020906003020160005b508554869086908110610c0257fe5b906000526020600020906003020160005b5081548154600160a060020a031916600160a060020a039091161781556001808301548282015560029283015492909101805460ff909316151560ff199093169290921790915585548591870190600090889084908110610c7057fe5b906000526020600020906003020160005b5054600160a060020a03168152602081019190915260400160002055925060028302600101610cb0565b600191505b84548390869086908110610cc057fe5b906000526020600020906003020160005b5081518154600160a060020a031916600160a060020a0391909116178155602082015181600101556040820151600291909101805460ff191691151591909117905550836001860160008551600160a060020a03168152602081019190915260400160002055610aca565b5b5050505050565b81548183558181151161076b5760030281600302836000526020600020918201910161076b9190610dc8565b5b505050565b81548183558181151161076b5760030281600302836000526020600020918201910161076b9190610dc8565b5b505050565b606060405190810160409081526000808352602083018190529082015290565b610e0391905b80821115610dff578054600160a060020a03191681556000600182015560028101805460ff19169055600301610dce565b5090565b905600a165627a7a723058209a298d0c607729a2f8c67663273e4e2275074e4aca6ef8d4b4191f87cfe255290029`

// DeployMinHeap deploys a new Ethereum contract, binding an instance of MinHeap to it.
func DeployMinHeap(auth *bind.TransactOpts, backend bind.ContractBackend, libraries map[string]common.Address) (common.Address, *types.Transaction, *MinHeap, error) {
	parsed, err := abi.JSON(strings.NewReader(MinHeapABI))
	if err != nil {
		return common.Address{}, nil, nil, err
	}

	linkedBin := MinHeapBin
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
	return address, tx, &MinHeap{MinHeapCaller: MinHeapCaller{contract: contract}, MinHeapTransactor: MinHeapTransactor{contract: contract}}, nil
}

// MinHeap is an auto generated Go binding around an Ethereum contract.
type MinHeap struct {
	MinHeapCaller     // Read-only binding to the contract
	MinHeapTransactor // Write-only binding to the contract
}

// MinHeapCaller is an auto generated read-only Go binding around an Ethereum contract.
type MinHeapCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// MinHeapTransactor is an auto generated write-only Go binding around an Ethereum contract.
type MinHeapTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// MinHeapSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type MinHeapSession struct {
	Contract     *MinHeap          // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// MinHeapCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type MinHeapCallerSession struct {
	Contract *MinHeapCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts  // Call options to use throughout this session
}

// MinHeapTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type MinHeapTransactorSession struct {
	Contract     *MinHeapTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts  // Transaction auth options to use throughout this session
}

// MinHeapRaw is an auto generated low-level Go binding around an Ethereum contract.
type MinHeapRaw struct {
	Contract *MinHeap // Generic contract binding to access the raw methods on
}

// MinHeapCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type MinHeapCallerRaw struct {
	Contract *MinHeapCaller // Generic read-only contract binding to access the raw methods on
}

// MinHeapTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type MinHeapTransactorRaw struct {
	Contract *MinHeapTransactor // Generic write-only contract binding to access the raw methods on
}

// NewMinHeap creates a new instance of MinHeap, bound to a specific deployed contract.
func NewMinHeap(address common.Address, backend bind.ContractBackend) (*MinHeap, error) {
	contract, err := bindMinHeap(address, backend, backend)
	if err != nil {
		return nil, err
	}
	return &MinHeap{MinHeapCaller: MinHeapCaller{contract: contract}, MinHeapTransactor: MinHeapTransactor{contract: contract}}, nil
}

// NewMinHeapCaller creates a new read-only instance of MinHeap, bound to a specific deployed contract.
func NewMinHeapCaller(address common.Address, caller bind.ContractCaller) (*MinHeapCaller, error) {
	contract, err := bindMinHeap(address, caller, nil)
	if err != nil {
		return nil, err
	}
	return &MinHeapCaller{contract: contract}, nil
}

// NewMinHeapTransactor creates a new write-only instance of MinHeap, bound to a specific deployed contract.
func NewMinHeapTransactor(address common.Address, transactor bind.ContractTransactor) (*MinHeapTransactor, error) {
	contract, err := bindMinHeap(address, nil, transactor)
	if err != nil {
		return nil, err
	}
	return &MinHeapTransactor{contract: contract}, nil
}

// bindMinHeap binds a generic wrapper to an already deployed contract.
func bindMinHeap(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor) (*bind.BoundContract, error) {
	parsed, err := abi.JSON(strings.NewReader(MinHeapABI))
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, parsed, caller, transactor), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_MinHeap *MinHeapRaw) Call(opts *bind.CallOpts, result interface{}, method string, params ...interface{}) error {
	return _MinHeap.Contract.MinHeapCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_MinHeap *MinHeapRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _MinHeap.Contract.MinHeapTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_MinHeap *MinHeapRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _MinHeap.Contract.MinHeapTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_MinHeap *MinHeapCallerRaw) Call(opts *bind.CallOpts, result interface{}, method string, params ...interface{}) error {
	return _MinHeap.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_MinHeap *MinHeapTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _MinHeap.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_MinHeap *MinHeapTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _MinHeap.Contract.contract.Transact(opts, method, params...)
}

// Contains is a free data retrieval call binding the contract method 0x84beebc5.
//
// Solidity: function contains(self MinHeap, _id address) constant returns(bool)
func (_MinHeap *MinHeapCaller) Contains(opts *bind.CallOpts, self MinHeap, _id common.Address) (bool, error) {
	var (
		ret0 = new(bool)
	)
	out := ret0
	err := _MinHeap.contract.Call(opts, out, "contains", self, _id)
	return *ret0, err
}

// Contains is a free data retrieval call binding the contract method 0x84beebc5.
//
// Solidity: function contains(self MinHeap, _id address) constant returns(bool)
func (_MinHeap *MinHeapSession) Contains(self MinHeap, _id common.Address) (bool, error) {
	return _MinHeap.Contract.Contains(&_MinHeap.CallOpts, self, _id)
}

// Contains is a free data retrieval call binding the contract method 0x84beebc5.
//
// Solidity: function contains(self MinHeap, _id address) constant returns(bool)
func (_MinHeap *MinHeapCallerSession) Contains(self MinHeap, _id common.Address) (bool, error) {
	return _MinHeap.Contract.Contains(&_MinHeap.CallOpts, self, _id)
}

// GetKey is a free data retrieval call binding the contract method 0x127acc76.
//
// Solidity: function getKey(self MinHeap, _id address) constant returns(uint256)
func (_MinHeap *MinHeapCaller) GetKey(opts *bind.CallOpts, self MinHeap, _id common.Address) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _MinHeap.contract.Call(opts, out, "getKey", self, _id)
	return *ret0, err
}

// GetKey is a free data retrieval call binding the contract method 0x127acc76.
//
// Solidity: function getKey(self MinHeap, _id address) constant returns(uint256)
func (_MinHeap *MinHeapSession) GetKey(self MinHeap, _id common.Address) (*big.Int, error) {
	return _MinHeap.Contract.GetKey(&_MinHeap.CallOpts, self, _id)
}

// GetKey is a free data retrieval call binding the contract method 0x127acc76.
//
// Solidity: function getKey(self MinHeap, _id address) constant returns(uint256)
func (_MinHeap *MinHeapCallerSession) GetKey(self MinHeap, _id common.Address) (*big.Int, error) {
	return _MinHeap.Contract.GetKey(&_MinHeap.CallOpts, self, _id)
}

// IsEmpty is a free data retrieval call binding the contract method 0x7c02bee0.
//
// Solidity: function isEmpty(self MinHeap) constant returns(bool)
func (_MinHeap *MinHeapCaller) IsEmpty(opts *bind.CallOpts, self MinHeap) (bool, error) {
	var (
		ret0 = new(bool)
	)
	out := ret0
	err := _MinHeap.contract.Call(opts, out, "isEmpty", self)
	return *ret0, err
}

// IsEmpty is a free data retrieval call binding the contract method 0x7c02bee0.
//
// Solidity: function isEmpty(self MinHeap) constant returns(bool)
func (_MinHeap *MinHeapSession) IsEmpty(self MinHeap) (bool, error) {
	return _MinHeap.Contract.IsEmpty(&_MinHeap.CallOpts, self)
}

// IsEmpty is a free data retrieval call binding the contract method 0x7c02bee0.
//
// Solidity: function isEmpty(self MinHeap) constant returns(bool)
func (_MinHeap *MinHeapCallerSession) IsEmpty(self MinHeap) (bool, error) {
	return _MinHeap.Contract.IsEmpty(&_MinHeap.CallOpts, self)
}

// IsFull is a free data retrieval call binding the contract method 0x86ac9d7b.
//
// Solidity: function isFull(self MinHeap) constant returns(bool)
func (_MinHeap *MinHeapCaller) IsFull(opts *bind.CallOpts, self MinHeap) (bool, error) {
	var (
		ret0 = new(bool)
	)
	out := ret0
	err := _MinHeap.contract.Call(opts, out, "isFull", self)
	return *ret0, err
}

// IsFull is a free data retrieval call binding the contract method 0x86ac9d7b.
//
// Solidity: function isFull(self MinHeap) constant returns(bool)
func (_MinHeap *MinHeapSession) IsFull(self MinHeap) (bool, error) {
	return _MinHeap.Contract.IsFull(&_MinHeap.CallOpts, self)
}

// IsFull is a free data retrieval call binding the contract method 0x86ac9d7b.
//
// Solidity: function isFull(self MinHeap) constant returns(bool)
func (_MinHeap *MinHeapCallerSession) IsFull(self MinHeap) (bool, error) {
	return _MinHeap.Contract.IsFull(&_MinHeap.CallOpts, self)
}

// Min is a free data retrieval call binding the contract method 0x8cd25c89.
//
// Solidity: function min(self MinHeap) constant returns(address, uint256)
func (_MinHeap *MinHeapCaller) Min(opts *bind.CallOpts, self MinHeap) (common.Address, *big.Int, error) {
	var (
		ret0 = new(common.Address)
		ret1 = new(*big.Int)
	)
	out := &[]interface{}{
		ret0,
		ret1,
	}
	err := _MinHeap.contract.Call(opts, out, "min", self)
	return *ret0, *ret1, err
}

// Min is a free data retrieval call binding the contract method 0x8cd25c89.
//
// Solidity: function min(self MinHeap) constant returns(address, uint256)
func (_MinHeap *MinHeapSession) Min(self MinHeap) (common.Address, *big.Int, error) {
	return _MinHeap.Contract.Min(&_MinHeap.CallOpts, self)
}

// Min is a free data retrieval call binding the contract method 0x8cd25c89.
//
// Solidity: function min(self MinHeap) constant returns(address, uint256)
func (_MinHeap *MinHeapCallerSession) Min(self MinHeap) (common.Address, *big.Int, error) {
	return _MinHeap.Contract.Min(&_MinHeap.CallOpts, self)
}

// Size is a free data retrieval call binding the contract method 0x9d2a412c.
//
// Solidity: function size(self MinHeap) constant returns(uint256)
func (_MinHeap *MinHeapCaller) Size(opts *bind.CallOpts, self MinHeap) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _MinHeap.contract.Call(opts, out, "size", self)
	return *ret0, err
}

// Size is a free data retrieval call binding the contract method 0x9d2a412c.
//
// Solidity: function size(self MinHeap) constant returns(uint256)
func (_MinHeap *MinHeapSession) Size(self MinHeap) (*big.Int, error) {
	return _MinHeap.Contract.Size(&_MinHeap.CallOpts, self)
}

// Size is a free data retrieval call binding the contract method 0x9d2a412c.
//
// Solidity: function size(self MinHeap) constant returns(uint256)
func (_MinHeap *MinHeapCallerSession) Size(self MinHeap) (*big.Int, error) {
	return _MinHeap.Contract.Size(&_MinHeap.CallOpts, self)
}

// DecreaseKey is a paid mutator transaction binding the contract method 0x38003c6d.
//
// Solidity: function decreaseKey(self MinHeap, _id address, _amount uint256) returns()
func (_MinHeap *MinHeapTransactor) DecreaseKey(opts *bind.TransactOpts, self MinHeap, _id common.Address, _amount *big.Int) (*types.Transaction, error) {
	return _MinHeap.contract.Transact(opts, "decreaseKey", self, _id, _amount)
}

// DecreaseKey is a paid mutator transaction binding the contract method 0x38003c6d.
//
// Solidity: function decreaseKey(self MinHeap, _id address, _amount uint256) returns()
func (_MinHeap *MinHeapSession) DecreaseKey(self MinHeap, _id common.Address, _amount *big.Int) (*types.Transaction, error) {
	return _MinHeap.Contract.DecreaseKey(&_MinHeap.TransactOpts, self, _id, _amount)
}

// DecreaseKey is a paid mutator transaction binding the contract method 0x38003c6d.
//
// Solidity: function decreaseKey(self MinHeap, _id address, _amount uint256) returns()
func (_MinHeap *MinHeapTransactorSession) DecreaseKey(self MinHeap, _id common.Address, _amount *big.Int) (*types.Transaction, error) {
	return _MinHeap.Contract.DecreaseKey(&_MinHeap.TransactOpts, self, _id, _amount)
}

// DeleteId is a paid mutator transaction binding the contract method 0x531d2b4a.
//
// Solidity: function deleteId(self MinHeap, _id address) returns()
func (_MinHeap *MinHeapTransactor) DeleteId(opts *bind.TransactOpts, self MinHeap, _id common.Address) (*types.Transaction, error) {
	return _MinHeap.contract.Transact(opts, "deleteId", self, _id)
}

// DeleteId is a paid mutator transaction binding the contract method 0x531d2b4a.
//
// Solidity: function deleteId(self MinHeap, _id address) returns()
func (_MinHeap *MinHeapSession) DeleteId(self MinHeap, _id common.Address) (*types.Transaction, error) {
	return _MinHeap.Contract.DeleteId(&_MinHeap.TransactOpts, self, _id)
}

// DeleteId is a paid mutator transaction binding the contract method 0x531d2b4a.
//
// Solidity: function deleteId(self MinHeap, _id address) returns()
func (_MinHeap *MinHeapTransactorSession) DeleteId(self MinHeap, _id common.Address) (*types.Transaction, error) {
	return _MinHeap.Contract.DeleteId(&_MinHeap.TransactOpts, self, _id)
}

// DeletePos is a paid mutator transaction binding the contract method 0x7dcfd2a2.
//
// Solidity: function deletePos(self MinHeap, _pos uint256) returns()
func (_MinHeap *MinHeapTransactor) DeletePos(opts *bind.TransactOpts, self MinHeap, _pos *big.Int) (*types.Transaction, error) {
	return _MinHeap.contract.Transact(opts, "deletePos", self, _pos)
}

// DeletePos is a paid mutator transaction binding the contract method 0x7dcfd2a2.
//
// Solidity: function deletePos(self MinHeap, _pos uint256) returns()
func (_MinHeap *MinHeapSession) DeletePos(self MinHeap, _pos *big.Int) (*types.Transaction, error) {
	return _MinHeap.Contract.DeletePos(&_MinHeap.TransactOpts, self, _pos)
}

// DeletePos is a paid mutator transaction binding the contract method 0x7dcfd2a2.
//
// Solidity: function deletePos(self MinHeap, _pos uint256) returns()
func (_MinHeap *MinHeapTransactorSession) DeletePos(self MinHeap, _pos *big.Int) (*types.Transaction, error) {
	return _MinHeap.Contract.DeletePos(&_MinHeap.TransactOpts, self, _pos)
}

// ExtractMin is a paid mutator transaction binding the contract method 0xbb5c0b7e.
//
// Solidity: function extractMin(self MinHeap) returns()
func (_MinHeap *MinHeapTransactor) ExtractMin(opts *bind.TransactOpts, self MinHeap) (*types.Transaction, error) {
	return _MinHeap.contract.Transact(opts, "extractMin", self)
}

// ExtractMin is a paid mutator transaction binding the contract method 0xbb5c0b7e.
//
// Solidity: function extractMin(self MinHeap) returns()
func (_MinHeap *MinHeapSession) ExtractMin(self MinHeap) (*types.Transaction, error) {
	return _MinHeap.Contract.ExtractMin(&_MinHeap.TransactOpts, self)
}

// ExtractMin is a paid mutator transaction binding the contract method 0xbb5c0b7e.
//
// Solidity: function extractMin(self MinHeap) returns()
func (_MinHeap *MinHeapTransactorSession) ExtractMin(self MinHeap) (*types.Transaction, error) {
	return _MinHeap.Contract.ExtractMin(&_MinHeap.TransactOpts, self)
}

// IncreaseKey is a paid mutator transaction binding the contract method 0xc4de36df.
//
// Solidity: function increaseKey(self MinHeap, _id address, _amount uint256) returns()
func (_MinHeap *MinHeapTransactor) IncreaseKey(opts *bind.TransactOpts, self MinHeap, _id common.Address, _amount *big.Int) (*types.Transaction, error) {
	return _MinHeap.contract.Transact(opts, "increaseKey", self, _id, _amount)
}

// IncreaseKey is a paid mutator transaction binding the contract method 0xc4de36df.
//
// Solidity: function increaseKey(self MinHeap, _id address, _amount uint256) returns()
func (_MinHeap *MinHeapSession) IncreaseKey(self MinHeap, _id common.Address, _amount *big.Int) (*types.Transaction, error) {
	return _MinHeap.Contract.IncreaseKey(&_MinHeap.TransactOpts, self, _id, _amount)
}

// IncreaseKey is a paid mutator transaction binding the contract method 0xc4de36df.
//
// Solidity: function increaseKey(self MinHeap, _id address, _amount uint256) returns()
func (_MinHeap *MinHeapTransactorSession) IncreaseKey(self MinHeap, _id common.Address, _amount *big.Int) (*types.Transaction, error) {
	return _MinHeap.Contract.IncreaseKey(&_MinHeap.TransactOpts, self, _id, _amount)
}

// Init is a paid mutator transaction binding the contract method 0xe9a4553b.
//
// Solidity: function init(self MinHeap, _size uint256) returns()
func (_MinHeap *MinHeapTransactor) Init(opts *bind.TransactOpts, self MinHeap, _size *big.Int) (*types.Transaction, error) {
	return _MinHeap.contract.Transact(opts, "init", self, _size)
}

// Init is a paid mutator transaction binding the contract method 0xe9a4553b.
//
// Solidity: function init(self MinHeap, _size uint256) returns()
func (_MinHeap *MinHeapSession) Init(self MinHeap, _size *big.Int) (*types.Transaction, error) {
	return _MinHeap.Contract.Init(&_MinHeap.TransactOpts, self, _size)
}

// Init is a paid mutator transaction binding the contract method 0xe9a4553b.
//
// Solidity: function init(self MinHeap, _size uint256) returns()
func (_MinHeap *MinHeapTransactorSession) Init(self MinHeap, _size *big.Int) (*types.Transaction, error) {
	return _MinHeap.Contract.Init(&_MinHeap.TransactOpts, self, _size)
}

// Insert is a paid mutator transaction binding the contract method 0x082396a6.
//
// Solidity: function insert(self MinHeap, _id address, _key uint256) returns()
func (_MinHeap *MinHeapTransactor) Insert(opts *bind.TransactOpts, self MinHeap, _id common.Address, _key *big.Int) (*types.Transaction, error) {
	return _MinHeap.contract.Transact(opts, "insert", self, _id, _key)
}

// Insert is a paid mutator transaction binding the contract method 0x082396a6.
//
// Solidity: function insert(self MinHeap, _id address, _key uint256) returns()
func (_MinHeap *MinHeapSession) Insert(self MinHeap, _id common.Address, _key *big.Int) (*types.Transaction, error) {
	return _MinHeap.Contract.Insert(&_MinHeap.TransactOpts, self, _id, _key)
}

// Insert is a paid mutator transaction binding the contract method 0x082396a6.
//
// Solidity: function insert(self MinHeap, _id address, _key uint256) returns()
func (_MinHeap *MinHeapTransactorSession) Insert(self MinHeap, _id common.Address, _key *big.Int) (*types.Transaction, error) {
	return _MinHeap.Contract.Insert(&_MinHeap.TransactOpts, self, _id, _key)
}
