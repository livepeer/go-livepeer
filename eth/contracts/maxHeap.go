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

// MaxHeapABI is the input ABI used to generate the binding from.
const MaxHeapABI = "[{\"constant\":true,\"inputs\":[{\"name\":\"self\",\"type\":\"MaxHeap.Heapstorage\"},{\"name\":\"_id\",\"type\":\"address\"}],\"name\":\"contains\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"self\",\"type\":\"MaxHeap.Heapstorage\"},{\"name\":\"_id\",\"type\":\"address\"}],\"name\":\"getKey\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"self\",\"type\":\"MaxHeap.Heapstorage\"},{\"name\":\"_id\",\"type\":\"address\"},{\"name\":\"_key\",\"type\":\"uint256\"}],\"name\":\"insert\",\"outputs\":[],\"payable\":false,\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"self\",\"type\":\"MaxHeap.Heapstorage\"},{\"name\":\"_pos\",\"type\":\"uint256\"}],\"name\":\"deletePos\",\"outputs\":[],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"self\",\"type\":\"MaxHeap.Heapstorage\"}],\"name\":\"isEmpty\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"self\",\"type\":\"MaxHeap.Heapstorage\"},{\"name\":\"_id\",\"type\":\"address\"},{\"name\":\"_amount\",\"type\":\"uint256\"}],\"name\":\"increaseKey\",\"outputs\":[],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"self\",\"type\":\"MaxHeap.Heapstorage\"}],\"name\":\"isFull\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"self\",\"type\":\"MaxHeap.Heapstorage\"},{\"name\":\"_id\",\"type\":\"address\"},{\"name\":\"_amount\",\"type\":\"uint256\"}],\"name\":\"decreaseKey\",\"outputs\":[],\"payable\":false,\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"self\",\"type\":\"MaxHeap.Heapstorage\"},{\"name\":\"_size\",\"type\":\"uint256\"}],\"name\":\"init\",\"outputs\":[],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"self\",\"type\":\"MaxHeap.Heapstorage\"}],\"name\":\"size\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"self\",\"type\":\"MaxHeap.Heapstorage\"},{\"name\":\"_id\",\"type\":\"address\"}],\"name\":\"deleteId\",\"outputs\":[],\"payable\":false,\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"self\",\"type\":\"MaxHeap.Heapstorage\"}],\"name\":\"extractMax\",\"outputs\":[],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"self\",\"type\":\"MaxHeap.Heapstorage\"}],\"name\":\"max\",\"outputs\":[{\"name\":\"\",\"type\":\"address\"},{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"type\":\"function\"}]"

// MaxHeapBin is the compiled bytecode used for deploying new contracts.
const MaxHeapBin = `0x6060604052341561000c57fe5b5b610d908061001c6000396000f300606060405236156100a95763ffffffff60e060020a60003504166302db769781146100ab5780630dd7fc44146100d65780632ace28e7146100ff57806332b987fe1461011b57806334b2408a1461012b5780633d14c4271461014a578063499ec95414610166578063610cac4814610185578063b3075a4e146101a1578063d495d1ad146101b1578063d6f84060146101ce578063e87f49dc146101e7578063fcdd392c146101f4575bfe5b6100c2600435600160a060020a0360243516610222565b604080519115158252519081900360200190f35b6100ed600435600160a060020a0360243516610247565b60408051918252519081900360200190f35b610119600435600160a060020a03602435166044356102b8565b005b6101196004356024356103a9565b005b6100c2600435610549565b604080519115158252519081900360200190f35b610119600435600160a060020a0360243516604435610552565b005b6100c2600435610603565b604080519115158252519081900360200190f35b610119600435600160a060020a0360243516604435610611565b005b6101196004356024356106c2565b005b6100ed6004356106f5565b60408051918252519081900360200190f35b610119600435600160a060020a03602435166106fd565b005b610119600435610752565b005b6101ff60043561076f565b60408051600160a060020a03909316835260208301919091528051918290030190f35b600160a060020a038116600090815260028301602052604090205460ff165b92915050565b600160a060020a038116600090815260028301602052604081205460ff1615156102715760006000fd5b600160a060020a03821660009081526001840160205260409020548354849190811061029957fe5b906000526020600020906003020160005b506001015490505b92915050565b6003830154835414156102cb5760006000fd5b600160a060020a038216600090815260028401602052604090205460ff161515600114156102f95760006000fd5b825483906001810161030b8382610ca2565b916000526020600020906003020160005b5060408051606081018252600160a060020a038616808252602080830187905260019284018390528454600160a060020a031916821785558483018790556002948501805460ff199081168517909155895460009384528a850183528584206000199182019055958a01909152929020805490921617905584546103a392508591016107df565b5b505050565b8154819010156103b95760006000fd5b600082600201600084600001848154811015156103d257fe5b906000526020600020906003020160005b5054600160a060020a031681526020810191909152604001600020805460ff191691151591909117905581548290600019810190811061041f57fe5b906000526020600020906003020160005b50825483908390811061043f57fe5b906000526020600020906003020160005b5081548154600160a060020a031916600160a060020a039091161781556001808301548282015560029283015492909101805460ff909316151560ff1990931692909217909155825482918401906000908590849081106104ad57fe5b906000526020600020906003020160005b5054600160a060020a031681526020810191909152604001600020558154829060001981019081106104ec57fe5b906000526020600020906003020160005b508054600160a060020a031916815560006001820155600201805460ff19169055815461052e836000198301610ca2565b508154819011156105435761054382826109df565b5b5b5050565b8054155b919050565b600160a060020a038216600090815260028401602052604081205460ff16151561057c5760006000fd5b50600160a060020a038216600090815260018401602052604090205483546105ce9083908690849081106105ac57fe5b906000526020600020906003020160005b50600101549063ffffffff610c5c16565b84548590839081106105dc57fe5b906000526020600020906003020160005b50600101556105fc84826107df565b5b50505050565b60038101548154145b919050565b600160a060020a038216600090815260028401602052604081205460ff16151561063b5760006000fd5b50600160a060020a0382166000908152600184016020526040902054835461068d90839086908490811061066b57fe5b906000526020600020906003020160005b50600101549063ffffffff610c7816565b845485908390811061069b57fe5b906000526020600020906003020160005b50600101556105fc84826109df565b5b50505050565b600482015460ff161515600114156106da5760006000fd5b6003820181905560048201805460ff191660011790555b5050565b80545b919050565b600160a060020a038116600090815260028301602052604090205460ff1615156107275760006000fd5b600160a060020a03811660009081526001830160205260409020546105439083906103a9565b5b5050565b805415156107605760006000fd5b61076b8160006103a9565b5b50565b8054600090819015156107825760006000fd5b82548390600090811061079157fe5b906000526020600020906003020160005b50548354600160a060020a0390911690849060009081106107bf57fe5b906000526020600020906003020160005b5060010154915091505b915091565b6107e7610d06565b82548390839081106107f557fe5b906000526020600020906003020160005b50604080516060810182528254600160a060020a031681526001830154602082015260029092015460ff1615159082015290505b6000821180156108785750602081015183600260001985015b0481548110151561086057fe5b906000526020600020906003020160005b5060010154105b1561095e5782600260001984015b0481548110151561089357fe5b906000526020600020906003020160005b5083548490849081106108b357fe5b906000526020600020906003020160005b5081548154600160a060020a031916600160a060020a039091161781556001808301548282015560029283015492909101805460ff909316151560ff19909316929092179091558354839185019060009086908490811061092157fe5b906000526020600020906003020160005b5054600160a060020a03168152602081019190915260400160002055600260001983015b04915061083a565b8254819084908490811061096e57fe5b906000526020600020906003020160005b5081518154600160a060020a031916600160a060020a039182161782556020808401516001808501919091556040948501516002909401805460ff191694151594909417909355845190911660009081529186019052208290555b505050565b6109e7610d06565b6000600084600001848154811015156109fc57fe5b906000526020600020906003020160005b50604080516060810182528254600160a060020a03168152600180840154602083015260029384015460ff161515928201929092529450600093509085020190505b845481108015610a5d575081155b15610c5457845460018201108015610ac057508454859082908110610a7e57fe5b906000526020600020906003020160005b50600101548560000182600101815481101515610aa857fe5b906000526020600020906003020160005b5060010154115b15610ac9576001015b60208301518554869083908110610adc57fe5b906000526020600020906003020160005b50600101541115610bcf578454859082908110610b0657fe5b906000526020600020906003020160005b508554869086908110610b2657fe5b906000526020600020906003020160005b5081548154600160a060020a031916600160a060020a039091161781556001808301548282015560029283015492909101805460ff909316151560ff199093169290921790915585548591870190600090889084908110610b9457fe5b906000526020600020906003020160005b5054600160a060020a03168152602081019190915260400160002055925060028302600101610bd4565b600191505b84548390869086908110610be457fe5b906000526020600020906003020160005b5081518154600160a060020a031916600160a060020a039182161782556020808401516001808501919091556040948501516002909401805460ff19169415159490941790935586519091166000908152918801905220849055610a4f565b5b5050505050565b6000828201610c6d84821015610c91565b8091505b5092915050565b6000610c8683831115610c91565b508082035b92915050565b80151561076b5760006000fd5b5b50565b8154818355818115116103a3576003028160030283600052602060002091820191016103a39190610d26565b5b505050565b8154818355818115116103a3576003028160030283600052602060002091820191016103a39190610d26565b5b505050565b604080516060810182526000808252602082018190529181019190915290565b610d6191905b80821115610d5d578054600160a060020a03191681556000600182015560028101805460ff19169055600301610d2c565b5090565b905600a165627a7a72305820e573dd3e820b3365e9e31ad5e57831be26e92a418bd7996887c9030499c64f320029`

// DeployMaxHeap deploys a new Ethereum contract, binding an instance of MaxHeap to it.
func DeployMaxHeap(auth *bind.TransactOpts, backend bind.ContractBackend, libraries map[string]common.Address) (common.Address, *types.Transaction, *MaxHeap, error) {
	parsed, err := abi.JSON(strings.NewReader(MaxHeapABI))
	if err != nil {
		return common.Address{}, nil, nil, err
	}

	linkedBin := MaxHeapBin
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
	return address, tx, &MaxHeap{MaxHeapCaller: MaxHeapCaller{contract: contract}, MaxHeapTransactor: MaxHeapTransactor{contract: contract}}, nil
}

// MaxHeap is an auto generated Go binding around an Ethereum contract.
type MaxHeap struct {
	MaxHeapCaller     // Read-only binding to the contract
	MaxHeapTransactor // Write-only binding to the contract
}

// MaxHeapCaller is an auto generated read-only Go binding around an Ethereum contract.
type MaxHeapCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// MaxHeapTransactor is an auto generated write-only Go binding around an Ethereum contract.
type MaxHeapTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// MaxHeapSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type MaxHeapSession struct {
	Contract     *MaxHeap          // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// MaxHeapCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type MaxHeapCallerSession struct {
	Contract *MaxHeapCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts  // Call options to use throughout this session
}

// MaxHeapTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type MaxHeapTransactorSession struct {
	Contract     *MaxHeapTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts  // Transaction auth options to use throughout this session
}

// MaxHeapRaw is an auto generated low-level Go binding around an Ethereum contract.
type MaxHeapRaw struct {
	Contract *MaxHeap // Generic contract binding to access the raw methods on
}

// MaxHeapCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type MaxHeapCallerRaw struct {
	Contract *MaxHeapCaller // Generic read-only contract binding to access the raw methods on
}

// MaxHeapTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type MaxHeapTransactorRaw struct {
	Contract *MaxHeapTransactor // Generic write-only contract binding to access the raw methods on
}

// NewMaxHeap creates a new instance of MaxHeap, bound to a specific deployed contract.
func NewMaxHeap(address common.Address, backend bind.ContractBackend) (*MaxHeap, error) {
	contract, err := bindMaxHeap(address, backend, backend)
	if err != nil {
		return nil, err
	}
	return &MaxHeap{MaxHeapCaller: MaxHeapCaller{contract: contract}, MaxHeapTransactor: MaxHeapTransactor{contract: contract}}, nil
}

// NewMaxHeapCaller creates a new read-only instance of MaxHeap, bound to a specific deployed contract.
func NewMaxHeapCaller(address common.Address, caller bind.ContractCaller) (*MaxHeapCaller, error) {
	contract, err := bindMaxHeap(address, caller, nil)
	if err != nil {
		return nil, err
	}
	return &MaxHeapCaller{contract: contract}, nil
}

// NewMaxHeapTransactor creates a new write-only instance of MaxHeap, bound to a specific deployed contract.
func NewMaxHeapTransactor(address common.Address, transactor bind.ContractTransactor) (*MaxHeapTransactor, error) {
	contract, err := bindMaxHeap(address, nil, transactor)
	if err != nil {
		return nil, err
	}
	return &MaxHeapTransactor{contract: contract}, nil
}

// bindMaxHeap binds a generic wrapper to an already deployed contract.
func bindMaxHeap(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor) (*bind.BoundContract, error) {
	parsed, err := abi.JSON(strings.NewReader(MaxHeapABI))
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, parsed, caller, transactor), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_MaxHeap *MaxHeapRaw) Call(opts *bind.CallOpts, result interface{}, method string, params ...interface{}) error {
	return _MaxHeap.Contract.MaxHeapCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_MaxHeap *MaxHeapRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _MaxHeap.Contract.MaxHeapTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_MaxHeap *MaxHeapRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _MaxHeap.Contract.MaxHeapTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_MaxHeap *MaxHeapCallerRaw) Call(opts *bind.CallOpts, result interface{}, method string, params ...interface{}) error {
	return _MaxHeap.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_MaxHeap *MaxHeapTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _MaxHeap.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_MaxHeap *MaxHeapTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _MaxHeap.Contract.contract.Transact(opts, method, params...)
}

// Contains is a free data retrieval call binding the contract method 0xb22f79a0.
//
// Solidity: function contains(self MaxHeap, _id address) constant returns(bool)
func (_MaxHeap *MaxHeapCaller) Contains(opts *bind.CallOpts, self MaxHeap, _id common.Address) (bool, error) {
	var (
		ret0 = new(bool)
	)
	out := ret0
	err := _MaxHeap.contract.Call(opts, out, "contains", self, _id)
	return *ret0, err
}

// Contains is a free data retrieval call binding the contract method 0xb22f79a0.
//
// Solidity: function contains(self MaxHeap, _id address) constant returns(bool)
func (_MaxHeap *MaxHeapSession) Contains(self MaxHeap, _id common.Address) (bool, error) {
	return _MaxHeap.Contract.Contains(&_MaxHeap.CallOpts, self, _id)
}

// Contains is a free data retrieval call binding the contract method 0xb22f79a0.
//
// Solidity: function contains(self MaxHeap, _id address) constant returns(bool)
func (_MaxHeap *MaxHeapCallerSession) Contains(self MaxHeap, _id common.Address) (bool, error) {
	return _MaxHeap.Contract.Contains(&_MaxHeap.CallOpts, self, _id)
}

// GetKey is a free data retrieval call binding the contract method 0x97ae9665.
//
// Solidity: function getKey(self MaxHeap, _id address) constant returns(uint256)
func (_MaxHeap *MaxHeapCaller) GetKey(opts *bind.CallOpts, self MaxHeap, _id common.Address) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _MaxHeap.contract.Call(opts, out, "getKey", self, _id)
	return *ret0, err
}

// GetKey is a free data retrieval call binding the contract method 0x97ae9665.
//
// Solidity: function getKey(self MaxHeap, _id address) constant returns(uint256)
func (_MaxHeap *MaxHeapSession) GetKey(self MaxHeap, _id common.Address) (*big.Int, error) {
	return _MaxHeap.Contract.GetKey(&_MaxHeap.CallOpts, self, _id)
}

// GetKey is a free data retrieval call binding the contract method 0x97ae9665.
//
// Solidity: function getKey(self MaxHeap, _id address) constant returns(uint256)
func (_MaxHeap *MaxHeapCallerSession) GetKey(self MaxHeap, _id common.Address) (*big.Int, error) {
	return _MaxHeap.Contract.GetKey(&_MaxHeap.CallOpts, self, _id)
}

// IsEmpty is a free data retrieval call binding the contract method 0x09aa112d.
//
// Solidity: function isEmpty(self MaxHeap) constant returns(bool)
func (_MaxHeap *MaxHeapCaller) IsEmpty(opts *bind.CallOpts, self MaxHeap) (bool, error) {
	var (
		ret0 = new(bool)
	)
	out := ret0
	err := _MaxHeap.contract.Call(opts, out, "isEmpty", self)
	return *ret0, err
}

// IsEmpty is a free data retrieval call binding the contract method 0x09aa112d.
//
// Solidity: function isEmpty(self MaxHeap) constant returns(bool)
func (_MaxHeap *MaxHeapSession) IsEmpty(self MaxHeap) (bool, error) {
	return _MaxHeap.Contract.IsEmpty(&_MaxHeap.CallOpts, self)
}

// IsEmpty is a free data retrieval call binding the contract method 0x09aa112d.
//
// Solidity: function isEmpty(self MaxHeap) constant returns(bool)
func (_MaxHeap *MaxHeapCallerSession) IsEmpty(self MaxHeap) (bool, error) {
	return _MaxHeap.Contract.IsEmpty(&_MaxHeap.CallOpts, self)
}

// IsFull is a free data retrieval call binding the contract method 0x0a546dd0.
//
// Solidity: function isFull(self MaxHeap) constant returns(bool)
func (_MaxHeap *MaxHeapCaller) IsFull(opts *bind.CallOpts, self MaxHeap) (bool, error) {
	var (
		ret0 = new(bool)
	)
	out := ret0
	err := _MaxHeap.contract.Call(opts, out, "isFull", self)
	return *ret0, err
}

// IsFull is a free data retrieval call binding the contract method 0x0a546dd0.
//
// Solidity: function isFull(self MaxHeap) constant returns(bool)
func (_MaxHeap *MaxHeapSession) IsFull(self MaxHeap) (bool, error) {
	return _MaxHeap.Contract.IsFull(&_MaxHeap.CallOpts, self)
}

// IsFull is a free data retrieval call binding the contract method 0x0a546dd0.
//
// Solidity: function isFull(self MaxHeap) constant returns(bool)
func (_MaxHeap *MaxHeapCallerSession) IsFull(self MaxHeap) (bool, error) {
	return _MaxHeap.Contract.IsFull(&_MaxHeap.CallOpts, self)
}

// Max is a free data retrieval call binding the contract method 0xd4458d19.
//
// Solidity: function max(self MaxHeap) constant returns(address, uint256)
func (_MaxHeap *MaxHeapCaller) Max(opts *bind.CallOpts, self MaxHeap) (common.Address, *big.Int, error) {
	var (
		ret0 = new(common.Address)
		ret1 = new(*big.Int)
	)
	out := &[]interface{}{
		ret0,
		ret1,
	}
	err := _MaxHeap.contract.Call(opts, out, "max", self)
	return *ret0, *ret1, err
}

// Max is a free data retrieval call binding the contract method 0xd4458d19.
//
// Solidity: function max(self MaxHeap) constant returns(address, uint256)
func (_MaxHeap *MaxHeapSession) Max(self MaxHeap) (common.Address, *big.Int, error) {
	return _MaxHeap.Contract.Max(&_MaxHeap.CallOpts, self)
}

// Max is a free data retrieval call binding the contract method 0xd4458d19.
//
// Solidity: function max(self MaxHeap) constant returns(address, uint256)
func (_MaxHeap *MaxHeapCallerSession) Max(self MaxHeap) (common.Address, *big.Int, error) {
	return _MaxHeap.Contract.Max(&_MaxHeap.CallOpts, self)
}

// Size is a free data retrieval call binding the contract method 0x3aaf7415.
//
// Solidity: function size(self MaxHeap) constant returns(uint256)
func (_MaxHeap *MaxHeapCaller) Size(opts *bind.CallOpts, self MaxHeap) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _MaxHeap.contract.Call(opts, out, "size", self)
	return *ret0, err
}

// Size is a free data retrieval call binding the contract method 0x3aaf7415.
//
// Solidity: function size(self MaxHeap) constant returns(uint256)
func (_MaxHeap *MaxHeapSession) Size(self MaxHeap) (*big.Int, error) {
	return _MaxHeap.Contract.Size(&_MaxHeap.CallOpts, self)
}

// Size is a free data retrieval call binding the contract method 0x3aaf7415.
//
// Solidity: function size(self MaxHeap) constant returns(uint256)
func (_MaxHeap *MaxHeapCallerSession) Size(self MaxHeap) (*big.Int, error) {
	return _MaxHeap.Contract.Size(&_MaxHeap.CallOpts, self)
}

// DecreaseKey is a paid mutator transaction binding the contract method 0xd9d99bd8.
//
// Solidity: function decreaseKey(self MaxHeap, _id address, _amount uint256) returns()
func (_MaxHeap *MaxHeapTransactor) DecreaseKey(opts *bind.TransactOpts, self MaxHeap, _id common.Address, _amount *big.Int) (*types.Transaction, error) {
	return _MaxHeap.contract.Transact(opts, "decreaseKey", self, _id, _amount)
}

// DecreaseKey is a paid mutator transaction binding the contract method 0xd9d99bd8.
//
// Solidity: function decreaseKey(self MaxHeap, _id address, _amount uint256) returns()
func (_MaxHeap *MaxHeapSession) DecreaseKey(self MaxHeap, _id common.Address, _amount *big.Int) (*types.Transaction, error) {
	return _MaxHeap.Contract.DecreaseKey(&_MaxHeap.TransactOpts, self, _id, _amount)
}

// DecreaseKey is a paid mutator transaction binding the contract method 0xd9d99bd8.
//
// Solidity: function decreaseKey(self MaxHeap, _id address, _amount uint256) returns()
func (_MaxHeap *MaxHeapTransactorSession) DecreaseKey(self MaxHeap, _id common.Address, _amount *big.Int) (*types.Transaction, error) {
	return _MaxHeap.Contract.DecreaseKey(&_MaxHeap.TransactOpts, self, _id, _amount)
}

// DeleteId is a paid mutator transaction binding the contract method 0x7ee840b9.
//
// Solidity: function deleteId(self MaxHeap, _id address) returns()
func (_MaxHeap *MaxHeapTransactor) DeleteId(opts *bind.TransactOpts, self MaxHeap, _id common.Address) (*types.Transaction, error) {
	return _MaxHeap.contract.Transact(opts, "deleteId", self, _id)
}

// DeleteId is a paid mutator transaction binding the contract method 0x7ee840b9.
//
// Solidity: function deleteId(self MaxHeap, _id address) returns()
func (_MaxHeap *MaxHeapSession) DeleteId(self MaxHeap, _id common.Address) (*types.Transaction, error) {
	return _MaxHeap.Contract.DeleteId(&_MaxHeap.TransactOpts, self, _id)
}

// DeleteId is a paid mutator transaction binding the contract method 0x7ee840b9.
//
// Solidity: function deleteId(self MaxHeap, _id address) returns()
func (_MaxHeap *MaxHeapTransactorSession) DeleteId(self MaxHeap, _id common.Address) (*types.Transaction, error) {
	return _MaxHeap.Contract.DeleteId(&_MaxHeap.TransactOpts, self, _id)
}

// DeletePos is a paid mutator transaction binding the contract method 0x3e33afe4.
//
// Solidity: function deletePos(self MaxHeap, _pos uint256) returns()
func (_MaxHeap *MaxHeapTransactor) DeletePos(opts *bind.TransactOpts, self MaxHeap, _pos *big.Int) (*types.Transaction, error) {
	return _MaxHeap.contract.Transact(opts, "deletePos", self, _pos)
}

// DeletePos is a paid mutator transaction binding the contract method 0x3e33afe4.
//
// Solidity: function deletePos(self MaxHeap, _pos uint256) returns()
func (_MaxHeap *MaxHeapSession) DeletePos(self MaxHeap, _pos *big.Int) (*types.Transaction, error) {
	return _MaxHeap.Contract.DeletePos(&_MaxHeap.TransactOpts, self, _pos)
}

// DeletePos is a paid mutator transaction binding the contract method 0x3e33afe4.
//
// Solidity: function deletePos(self MaxHeap, _pos uint256) returns()
func (_MaxHeap *MaxHeapTransactorSession) DeletePos(self MaxHeap, _pos *big.Int) (*types.Transaction, error) {
	return _MaxHeap.Contract.DeletePos(&_MaxHeap.TransactOpts, self, _pos)
}

// ExtractMax is a paid mutator transaction binding the contract method 0xf2da2241.
//
// Solidity: function extractMax(self MaxHeap) returns()
func (_MaxHeap *MaxHeapTransactor) ExtractMax(opts *bind.TransactOpts, self MaxHeap) (*types.Transaction, error) {
	return _MaxHeap.contract.Transact(opts, "extractMax", self)
}

// ExtractMax is a paid mutator transaction binding the contract method 0xf2da2241.
//
// Solidity: function extractMax(self MaxHeap) returns()
func (_MaxHeap *MaxHeapSession) ExtractMax(self MaxHeap) (*types.Transaction, error) {
	return _MaxHeap.Contract.ExtractMax(&_MaxHeap.TransactOpts, self)
}

// ExtractMax is a paid mutator transaction binding the contract method 0xf2da2241.
//
// Solidity: function extractMax(self MaxHeap) returns()
func (_MaxHeap *MaxHeapTransactorSession) ExtractMax(self MaxHeap) (*types.Transaction, error) {
	return _MaxHeap.Contract.ExtractMax(&_MaxHeap.TransactOpts, self)
}

// IncreaseKey is a paid mutator transaction binding the contract method 0xe9c94987.
//
// Solidity: function increaseKey(self MaxHeap, _id address, _amount uint256) returns()
func (_MaxHeap *MaxHeapTransactor) IncreaseKey(opts *bind.TransactOpts, self MaxHeap, _id common.Address, _amount *big.Int) (*types.Transaction, error) {
	return _MaxHeap.contract.Transact(opts, "increaseKey", self, _id, _amount)
}

// IncreaseKey is a paid mutator transaction binding the contract method 0xe9c94987.
//
// Solidity: function increaseKey(self MaxHeap, _id address, _amount uint256) returns()
func (_MaxHeap *MaxHeapSession) IncreaseKey(self MaxHeap, _id common.Address, _amount *big.Int) (*types.Transaction, error) {
	return _MaxHeap.Contract.IncreaseKey(&_MaxHeap.TransactOpts, self, _id, _amount)
}

// IncreaseKey is a paid mutator transaction binding the contract method 0xe9c94987.
//
// Solidity: function increaseKey(self MaxHeap, _id address, _amount uint256) returns()
func (_MaxHeap *MaxHeapTransactorSession) IncreaseKey(self MaxHeap, _id common.Address, _amount *big.Int) (*types.Transaction, error) {
	return _MaxHeap.Contract.IncreaseKey(&_MaxHeap.TransactOpts, self, _id, _amount)
}

// Init is a paid mutator transaction binding the contract method 0x069c5652.
//
// Solidity: function init(self MaxHeap, _size uint256) returns()
func (_MaxHeap *MaxHeapTransactor) Init(opts *bind.TransactOpts, self MaxHeap, _size *big.Int) (*types.Transaction, error) {
	return _MaxHeap.contract.Transact(opts, "init", self, _size)
}

// Init is a paid mutator transaction binding the contract method 0x069c5652.
//
// Solidity: function init(self MaxHeap, _size uint256) returns()
func (_MaxHeap *MaxHeapSession) Init(self MaxHeap, _size *big.Int) (*types.Transaction, error) {
	return _MaxHeap.Contract.Init(&_MaxHeap.TransactOpts, self, _size)
}

// Init is a paid mutator transaction binding the contract method 0x069c5652.
//
// Solidity: function init(self MaxHeap, _size uint256) returns()
func (_MaxHeap *MaxHeapTransactorSession) Init(self MaxHeap, _size *big.Int) (*types.Transaction, error) {
	return _MaxHeap.Contract.Init(&_MaxHeap.TransactOpts, self, _size)
}

// Insert is a paid mutator transaction binding the contract method 0x2bfc6299.
//
// Solidity: function insert(self MaxHeap, _id address, _key uint256) returns()
func (_MaxHeap *MaxHeapTransactor) Insert(opts *bind.TransactOpts, self MaxHeap, _id common.Address, _key *big.Int) (*types.Transaction, error) {
	return _MaxHeap.contract.Transact(opts, "insert", self, _id, _key)
}

// Insert is a paid mutator transaction binding the contract method 0x2bfc6299.
//
// Solidity: function insert(self MaxHeap, _id address, _key uint256) returns()
func (_MaxHeap *MaxHeapSession) Insert(self MaxHeap, _id common.Address, _key *big.Int) (*types.Transaction, error) {
	return _MaxHeap.Contract.Insert(&_MaxHeap.TransactOpts, self, _id, _key)
}

// Insert is a paid mutator transaction binding the contract method 0x2bfc6299.
//
// Solidity: function insert(self MaxHeap, _id address, _key uint256) returns()
func (_MaxHeap *MaxHeapTransactorSession) Insert(self MaxHeap, _id common.Address, _key *big.Int) (*types.Transaction, error) {
	return _MaxHeap.Contract.Insert(&_MaxHeap.TransactOpts, self, _id, _key)
}
