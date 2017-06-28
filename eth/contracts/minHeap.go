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

// MinHeapABI is the input ABI used to generate the binding from.
const MinHeapABI = "[{\"constant\":false,\"inputs\":[{\"name\":\"self\",\"type\":\"MinHeap.Heapstorage\"},{\"name\":\"_id\",\"type\":\"address\"}],\"name\":\"deleteId\",\"outputs\":[],\"payable\":false,\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"self\",\"type\":\"MinHeap.Heapstorage\"},{\"name\":\"_id\",\"type\":\"address\"},{\"name\":\"_amount\",\"type\":\"uint256\"}],\"name\":\"decreaseKey\",\"outputs\":[],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"self\",\"type\":\"MinHeap.Heapstorage\"}],\"name\":\"isEmpty\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"self\",\"type\":\"MinHeap.Heapstorage\"},{\"name\":\"_id\",\"type\":\"address\"},{\"name\":\"_amount\",\"type\":\"uint256\"}],\"name\":\"increaseKey\",\"outputs\":[],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"self\",\"type\":\"MinHeap.Heapstorage\"}],\"name\":\"size\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"self\",\"type\":\"MinHeap.Heapstorage\"},{\"name\":\"_pos\",\"type\":\"uint256\"}],\"name\":\"deletePos\",\"outputs\":[],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"self\",\"type\":\"MinHeap.Heapstorage\"},{\"name\":\"_id\",\"type\":\"address\"}],\"name\":\"getKey\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"self\",\"type\":\"MinHeap.Heapstorage\"},{\"name\":\"_size\",\"type\":\"uint256\"}],\"name\":\"init\",\"outputs\":[],\"payable\":false,\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"self\",\"type\":\"MinHeap.Heapstorage\"}],\"name\":\"extractMin\",\"outputs\":[],\"payable\":false,\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"self\",\"type\":\"MinHeap.Heapstorage\"},{\"name\":\"_id\",\"type\":\"address\"},{\"name\":\"_key\",\"type\":\"uint256\"}],\"name\":\"insert\",\"outputs\":[],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"self\",\"type\":\"MinHeap.Heapstorage\"}],\"name\":\"min\",\"outputs\":[{\"name\":\"\",\"type\":\"address\"},{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"self\",\"type\":\"MinHeap.Heapstorage\"}],\"name\":\"isFull\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"self\",\"type\":\"MinHeap.Heapstorage\"},{\"name\":\"_id\",\"type\":\"address\"}],\"name\":\"contains\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"type\":\"function\"}]"

// MinHeapBin is the compiled bytecode used for deploying new contracts.
const MinHeapBin = `0x6060604052341561000c57fe5b5b610df18061001c6000396000f300606060405236156100a95763ffffffff60e060020a6000350416630161e01281146100ab5780630c10bda3146100c45780630fec2c17146100e05780631a8255cd146100ff578063383b84031461011b5780633caee0111461013857806360fbb88e1461014857806380f569d4146101715780639c94352314610181578063cffd134a1461018e578063e0e7c2a6146101aa578063f98b8071146101d8578063ff73bd3b146101f7575bfe5b6100c2600435600160a060020a0360243516610222565b005b6100c2600435600160a060020a0360243516604435610277565b005b6100eb600435610328565b604080519115158252519081900360200190f35b6100c2600435600160a060020a0360243516604435610331565b005b6101266004356103e2565b60408051918252519081900360200190f35b6100c26004356024356103ea565b005b610126600435600160a060020a036024351661058a565b60408051918252519081900360200190f35b6100c26004356024356105fb565b005b6100c260043561062e565b005b6100c2600435600160a060020a036024351660443561064b565b005b6101b560043561073c565b60408051600160a060020a03909316835260208301919091528051918290030190f35b6100eb6004356107ac565b604080519115158252519081900360200190f35b6100eb600435600160a060020a03602435166107ba565b604080519115158252519081900360200190f35b600160a060020a038116600090815260028301602052604090205460ff16151561024c5760006000fd5b600160a060020a03811660009081526001830160205260409020546102729083906103ea565b5b5050565b600160a060020a038216600090815260028401602052604081205460ff1615156102a15760006000fd5b50600160a060020a038216600090815260018401602052604090205483546102f39083908690849081106102d157fe5b906000526020600020906003020160005b50600101549063ffffffff6107df16565b845485908390811061030157fe5b906000526020600020906003020160005b506001015561032184826107f8565b5b50505050565b8054155b919050565b600160a060020a038216600090815260028401602052604081205460ff16151561035b5760006000fd5b50600160a060020a038216600090815260018401602052604090205483546103ad90839086908490811061038b57fe5b906000526020600020906003020160005b50600101549063ffffffff6109f816565b84548590839081106103bb57fe5b906000526020600020906003020160005b50600101556103218482610a14565b5b50505050565b80545b919050565b8154819010156103fa5760006000fd5b6000826002016000846000018481548110151561041357fe5b906000526020600020906003020160005b5054600160a060020a031681526020810191909152604001600020805460ff191691151591909117905581548290600019810190811061046057fe5b906000526020600020906003020160005b50825483908390811061048057fe5b906000526020600020906003020160005b5081548154600160a060020a031916600160a060020a039091161781556001808301548282015560029283015492909101805460ff909316151560ff1990931692909217909155825482918401906000908590849081106104ee57fe5b906000526020600020906003020160005b5054600160a060020a0316815260208101919091526040016000205581548290600019810190811061052d57fe5b906000526020600020906003020160005b508054600160a060020a031916815560006001820155600201805460ff19169055815461056f836000198301610d03565b50815481901115610272576102728282610a14565b5b5b5050565b600160a060020a038116600090815260028301602052604081205460ff1615156105b45760006000fd5b600160a060020a0382166000908152600184016020526040902054835484919081106105dc57fe5b906000526020600020906003020160005b506001015490505b92915050565b600482015460ff161515600114156106135760006000fd5b6003820181905560048201805460ff191660011790555b5050565b8054151561063c5760006000fd5b6106478160006103ea565b5b50565b60038301548354141561065e5760006000fd5b600160a060020a038216600090815260028401602052604090205460ff1615156001141561068c5760006000fd5b825483906001810161069e8382610d03565b916000526020600020906003020160005b5060408051606081018252600160a060020a038616808252602080830187905260019284018390528454600160a060020a031916821785558483018790556002948501805460ff199081168517909155895460009384528a850183528584206000199182019055958a019091529290208054909216179055845461073692508591016107f8565b5b505050565b80546000908190151561074f5760006000fd5b82548390600090811061075e57fe5b906000526020600020906003020160005b50548354600160a060020a03909116908490600090811061078c57fe5b906000526020600020906003020160005b5060010154915091505b915091565b60038101548154145b919050565b600160a060020a038116600090815260028301602052604090205460ff165b92915050565b60006107ed83831115610cf2565b508082035b92915050565b610800610d67565b825483908390811061080e57fe5b906000526020600020906003020160005b50604080516060810182528254600160a060020a031681526001830154602082015260029092015460ff1615159082015290505b6000821180156108915750602081015183600260001985015b0481548110151561087957fe5b906000526020600020906003020160005b5060010154115b156109775782600260001984015b048154811015156108ac57fe5b906000526020600020906003020160005b5083548490849081106108cc57fe5b906000526020600020906003020160005b5081548154600160a060020a031916600160a060020a039091161781556001808301548282015560029283015492909101805460ff909316151560ff19909316929092179091558354839185019060009086908490811061093a57fe5b906000526020600020906003020160005b5054600160a060020a03168152602081019190915260400160002055600260001983015b049150610853565b8254819084908490811061098757fe5b906000526020600020906003020160005b5081518154600160a060020a031916600160a060020a039182161782556020808401516001808501919091556040948501516002909401805460ff191694151594909417909355845190911660009081529186019052208290555b505050565b6000828201610a0984821015610cf2565b8091505b5092915050565b610a1c610d67565b600060008460000184815481101515610a3157fe5b906000526020600020906003020160005b50604080516060810182528254600160a060020a03168152600180840154602083015260029384015460ff161515928201929092529450600093509085020190505b845481108015610a92575081155b15610cea57845460018201108015610ad057508454859060018301908110610ab657fe5b906000526020600020906003020160005b506002015460ff165b8015610b2757508454859082908110610ae557fe5b906000526020600020906003020160005b50600101548560000182600101815481101515610b0f57fe5b906000526020600020906003020160005b5060010154105b15610b30576001015b8454859082908110610b3e57fe5b906000526020600020906003020160005b506002015460ff168015610b89575060208301518554869083908110610b7157fe5b906000526020600020906003020160005b5060010154105b15610c65578454859082908110610b9c57fe5b906000526020600020906003020160005b508554869086908110610bbc57fe5b906000526020600020906003020160005b5081548154600160a060020a031916600160a060020a039091161781556001808301548282015560029283015492909101805460ff909316151560ff199093169290921790915585548591870190600090889084908110610c2a57fe5b906000526020600020906003020160005b5054600160a060020a03168152602081019190915260400160002055925060028302600101610c6a565b600191505b84548390869086908110610c7a57fe5b906000526020600020906003020160005b5081518154600160a060020a031916600160a060020a039182161782556020808401516001808501919091556040948501516002909401805460ff19169415159490941790935586519091166000908152918801905220849055610a84565b5b5050505050565b8015156106475760006000fd5b5b50565b815481835581811511610736576003028160030283600052602060002091820191016107369190610d87565b5b505050565b815481835581811511610736576003028160030283600052602060002091820191016107369190610d87565b5b505050565b604080516060810182526000808252602082018190529181019190915290565b610dc291905b80821115610dbe578054600160a060020a03191681556000600182015560028101805460ff19169055600301610d8d565b5090565b905600a165627a7a723058209232a5e282f6b7e9719361cec7a1e1dfb4c04e37b46d6f25b109e866f0dcd51d0029`

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

		linkedBin = reg.ReplaceAllString(linkedBin, addr.Hex()[2:])
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
