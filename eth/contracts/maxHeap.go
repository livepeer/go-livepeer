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
const MaxHeapBin = `0x6060604052341561000f57600080fd5b5b610dd18061001f6000396000f300606060405236156100c25763ffffffff7c010000000000000000000000000000000000000000000000000000000060003504166302db769781146100c75780630dd7fc44146100f25780632ace28e71461011b57806332b987fe1461013757806334b2408a146101475780633d14c42714610166578063499ec95414610182578063610cac48146101a1578063b3075a4e146101bd578063d495d1ad146101cd578063d6f84060146101ea578063e87f49dc14610203578063fcdd392c14610210575b600080fd5b6100de600435600160a060020a036024351661023d565b604051901515815260200160405180910390f35b610109600435600160a060020a0360243516610262565b60405190815260200160405180910390f35b610135600435600160a060020a03602435166044356102d2565b005b6101356004356024356103e7565b005b6100de600435610586565b604051901515815260200160405180910390f35b610135600435600160a060020a036024351660443561058f565b005b6100de60043561063f565b604051901515815260200160405180910390f35b610135600435600160a060020a036024351660443561064d565b005b6101356004356024356106fd565b005b61010960043561072a565b60405190815260200160405180910390f35b610135600435600160a060020a0360243516610732565b005b610135600435610786565b005b61021b6004356107a4565b604051600160a060020a03909216825260208201526040908101905180910390f35b600160a060020a038116600090815260028301602052604090205460ff165b92915050565b600160a060020a038116600090815260028301602052604081205460ff16151561028b57600080fd5b600160a060020a0382166000908152600184016020526040902054835484919081106102b357fe5b906000526020600020906003020160005b506001015490505b92915050565b6003830154835414156102e457600080fd5b600160a060020a038216600090815260028401602052604090205460ff161561030c57600080fd5b825483906001810161031e8382610ce3565b916000526020600020906003020160005b60606040519081016040908152600160a060020a03871682526020820186905260019082015291905081518154600160a060020a031916600160a060020a03919091161781556020820151816001015560408201516002918201805491151560ff199283161790558654600160a060020a03871660009081526001898101602090815260408084206000199586019055958b0190529390208054909216909217905585546103e1935086925001610814565b5b505050565b8154819010156103f657600080fd5b6000826002016000846000018481548110151561040f57fe5b906000526020600020906003020160005b5054600160a060020a031681526020810191909152604001600020805460ff191691151591909117905581548290600019810190811061045c57fe5b906000526020600020906003020160005b50825483908390811061047c57fe5b906000526020600020906003020160005b5081548154600160a060020a031916600160a060020a039091161781556001808301548282015560029283015492909101805460ff909316151560ff1990931692909217909155825482918401906000908590849081106104ea57fe5b906000526020600020906003020160005b5054600160a060020a0316815260208101919091526040016000205581548290600019810190811061052957fe5b906000526020600020906003020160005b508054600160a060020a031916815560006001820155600201805460ff19169055815461056b836000198301610ce3565b50815481901115610580576105808282610a26565b5b5b5050565b8054155b919050565b600160a060020a038216600090815260028401602052604081205460ff1615156105b857600080fd5b50600160a060020a0382166000908152600184016020526040902054835461060a9083908690849081106105e857fe5b906000526020600020906003020160005b50600101549063ffffffff610cb216565b845485908390811061061857fe5b906000526020600020906003020160005b50600101556106388482610814565b5b50505050565b60038101548154145b919050565b600160a060020a038216600090815260028401602052604081205460ff16151561067657600080fd5b50600160a060020a038216600090815260018401602052604090205483546106c89083908690849081106106a657fe5b906000526020600020906003020160005b50600101549063ffffffff610ccc16565b84548590839081106106d657fe5b906000526020600020906003020160005b50600101556106388482610a26565b5b50505050565b600482015460ff161561070f57600080fd5b6003820181905560048201805460ff191660011790555b5050565b80545b919050565b600160a060020a038116600090815260028301602052604090205460ff16151561075b57600080fd5b600160a060020a03811660009081526001830160205260409020546105809083906103e7565b5b5050565b80546000901161079557600080fd5b6107a08160006103e7565b5b50565b805460009081908190116107b757600080fd5b8254839060009081106107c657fe5b906000526020600020906003020160005b50548354600160a060020a0390911690849060009081106107f457fe5b906000526020600020906003020160005b5060010154915091505b915091565b61081c610d47565b825483908390811061082a57fe5b906000526020600020906003020160005b50606060405190810160409081528254600160a060020a031682526001830154602083015260029092015460ff1615159181019190915290505b6000821180156108b35750806020015183600260001985015b0481548110151561089b57fe5b906000526020600020906003020160005b5060010154105b156109995782600260001984015b048154811015156108ce57fe5b906000526020600020906003020160005b5083548490849081106108ee57fe5b906000526020600020906003020160005b5081548154600160a060020a031916600160a060020a039091161781556001808301548282015560029283015492909101805460ff909316151560ff19909316929092179091558354839185019060009086908490811061095c57fe5b906000526020600020906003020160005b5054600160a060020a03168152602081019190915260400160002055600260001983015b049150610875565b825481908490849081106109a957fe5b906000526020600020906003020160005b5081518154600160a060020a031916600160a060020a0391909116178155602082015181600101556040820151600291909101805460ff191691151591909117905550816001840160008351600160a060020a031681526020810191909152604001600020555b505050565b610a2e610d47565b6000808460000184815481101515610a4257fe5b906000526020600020906003020160005b50606060405190810160409081528254600160a060020a03168252600180840154602084015260029384015460ff16151591830191909152909450600093509085020190505b845481108015610aa7575081155b15610caa57845460018201108015610b0a57508454859082908110610ac857fe5b906000526020600020906003020160005b50600101548560000182600101815481101515610af257fe5b906000526020600020906003020160005b5060010154115b15610b13576001015b82602001518554869083908110610b2657fe5b906000526020600020906003020160005b50600101541115610c19578454859082908110610b5057fe5b906000526020600020906003020160005b508554869086908110610b7057fe5b906000526020600020906003020160005b5081548154600160a060020a031916600160a060020a039091161781556001808301548282015560029283015492909101805460ff909316151560ff199093169290921790915585548591870190600090889084908110610bde57fe5b906000526020600020906003020160005b5054600160a060020a03168152602081019190915260400160002055925060028302600101610c1e565b600191505b84548390869086908110610c2e57fe5b906000526020600020906003020160005b5081518154600160a060020a031916600160a060020a0391909116178155602082015181600101556040820151600291909101805460ff191691151591909117905550836001860160008551600160a060020a03168152602081019190915260400160002055610a99565b5b5050505050565b600082820183811015610cc157fe5b8091505b5092915050565b600082821115610cd857fe5b508082035b92915050565b8154818355818115116103e1576003028160030283600052602060002091820191016103e19190610d67565b5b505050565b8154818355818115116103e1576003028160030283600052602060002091820191016103e19190610d67565b5b505050565b606060405190810160409081526000808352602083018190529082015290565b610da291905b80821115610d9e578054600160a060020a03191681556000600182015560028101805460ff19169055600301610d6d565b5090565b905600a165627a7a72305820441f389dbd19eceaa72a1deaa8cbe93a3de80e0e2fd13aa8104046185ccd46c90029`

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
