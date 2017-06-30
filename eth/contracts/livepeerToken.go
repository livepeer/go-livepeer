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

// LivepeerTokenABI is the input ABI used to generate the binding from.
const LivepeerTokenABI = "[{\"constant\":true,\"inputs\":[],\"name\":\"mintingFinished\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"name\",\"outputs\":[{\"name\":\"\",\"type\":\"string\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_spender\",\"type\":\"address\"},{\"name\":\"_value\",\"type\":\"uint256\"}],\"name\":\"approve\",\"outputs\":[],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"totalSupply\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_from\",\"type\":\"address\"},{\"name\":\"_to\",\"type\":\"address\"},{\"name\":\"_value\",\"type\":\"uint256\"}],\"name\":\"transferFrom\",\"outputs\":[],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"decimals\",\"outputs\":[{\"name\":\"\",\"type\":\"uint8\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_to\",\"type\":\"address\"},{\"name\":\"_amount\",\"type\":\"uint256\"}],\"name\":\"mint\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"version\",\"outputs\":[{\"name\":\"\",\"type\":\"string\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"_owner\",\"type\":\"address\"}],\"name\":\"balanceOf\",\"outputs\":[{\"name\":\"balance\",\"type\":\"uint256\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":false,\"inputs\":[],\"name\":\"finishMinting\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"owner\",\"outputs\":[{\"name\":\"\",\"type\":\"address\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"symbol\",\"outputs\":[{\"name\":\"\",\"type\":\"string\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_to\",\"type\":\"address\"},{\"name\":\"_value\",\"type\":\"uint256\"}],\"name\":\"transfer\",\"outputs\":[],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"_owner\",\"type\":\"address\"},{\"name\":\"_spender\",\"type\":\"address\"}],\"name\":\"allowance\",\"outputs\":[{\"name\":\"remaining\",\"type\":\"uint256\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"newOwner\",\"type\":\"address\"}],\"name\":\"transferOwnership\",\"outputs\":[],\"payable\":false,\"type\":\"function\"},{\"inputs\":[],\"payable\":false,\"type\":\"constructor\"},{\"payable\":false,\"type\":\"fallback\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"name\":\"to\",\"type\":\"address\"},{\"indexed\":false,\"name\":\"value\",\"type\":\"uint256\"}],\"name\":\"Mint\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[],\"name\":\"MintFinished\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"name\":\"owner\",\"type\":\"address\"},{\"indexed\":true,\"name\":\"spender\",\"type\":\"address\"},{\"indexed\":false,\"name\":\"value\",\"type\":\"uint256\"}],\"name\":\"Approval\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"name\":\"from\",\"type\":\"address\"},{\"indexed\":true,\"name\":\"to\",\"type\":\"address\"},{\"indexed\":false,\"name\":\"value\",\"type\":\"uint256\"}],\"name\":\"Transfer\",\"type\":\"event\"}]"

// LivepeerTokenBin is the compiled bytecode used for deploying new contracts.
const LivepeerTokenBin = `0x6003805460a060020a60ff0219169055600060045560a0604052600e60608190527f4c6976657065657220546f6b656e00000000000000000000000000000000000060809081526100539160059190610117565b506006805460ff191660121790556040805180820190915260038082527f4c5054000000000000000000000000000000000000000000000000000000000060209092019182526100a591600791610117565b506040805180820190915260038082527f302e31000000000000000000000000000000000000000000000000000000000060209092019182526100ea91600891610117565b5034156100f357fe5b5b5b60038054600160a060020a03191633600160a060020a03161790555b5b6101b7565b828054600181600116156101000203166002900490600052602060002090601f016020900481019282601f1061015857805160ff1916838001178555610185565b82800160010185558215610185579182015b8281111561018557825182559160200191906001019061016a565b5b50610192929150610196565b5090565b6101b491905b80821115610192576000815560010161019c565b5090565b90565b610ab1806101c66000396000f300606060405236156100bf5763ffffffff60e060020a60003504166305d2035b81146100d557806306fdde03146100f9578063095ea7b31461018957806318160ddd146101aa57806323b872dd146101cc578063313ce567146101f357806340c10f191461021957806354fd4d501461024c57806370a08231146102dc5780637d64bcb41461030a5780638da5cb5b1461032e57806395d89b411461035a578063a9059cbb146103ea578063dd62ed3e1461040b578063f2fde38b1461043f575b34156100c757fe5b6100d35b60006000fd5b565b005b34156100dd57fe5b6100e561045d565b604080519115158252519081900360200190f35b341561010157fe5b61010961046d565b60408051602080825283518183015283519192839290830191850190808383821561014f575b80518252602083111561014f57601f19909201916020918201910161012f565b505050905090810190601f16801561017b5780820380516001836020036101000a031916815260200191505b509250505060405180910390f35b341561019157fe5b6100d3600160a060020a03600435166024356104fb565b005b34156101b257fe5b6101ba61055d565b60408051918252519081900360200190f35b34156101d457fe5b6100d3600160a060020a0360043581169060243516604435610563565b005b34156101fb57fe5b61020361065d565b6040805160ff9092168252519081900360200190f35b341561022157fe5b6100e5600160a060020a0360043516602435610666565b604080519115158252519081900360200190f35b341561025457fe5b61010961073a565b60408051602080825283518183015283519192839290830191850190808383821561014f575b80518252602083111561014f57601f19909201916020918201910161012f565b505050905090810190601f16801561017b5780820380516001836020036101000a031916815260200191505b509250505060405180910390f35b34156102e457fe5b6101ba600160a060020a03600435166107c8565b60408051918252519081900360200190f35b341561031257fe5b6100e56107e7565b604080519115158252519081900360200190f35b341561033657fe5b61033e61084d565b60408051600160a060020a039092168252519081900360200190f35b341561036257fe5b61010961085c565b60408051602080825283518183015283519192839290830191850190808383821561014f575b80518252602083111561014f57601f19909201916020918201910161012f565b505050905090810190601f16801561017b5780820380516001836020036101000a031916815260200191505b509250505060405180910390f35b34156103f257fe5b6100d3600160a060020a03600435166024356108ea565b005b341561041357fe5b6101ba600160a060020a03600435811690602435166109a6565b60408051918252519081900360200190f35b341561044757fe5b6100d3600160a060020a03600435166109d3565b005b60035460a060020a900460ff1681565b6005805460408051602060026001851615610100026000190190941693909304601f810184900484028201840190925281815292918301828280156104f35780601f106104c8576101008083540402835291602001916104f3565b820191906000526020600020905b8154815290600101906020018083116104d657829003601f168201915b505050505081565b600160a060020a03338116600081815260026020908152604080832094871680845294825291829020859055815185815291517f8c5be1e5ebec7d5bd14f71427d1e84f3dd0314c0f7b2291e5b200ac8c7c3b9259281900390910190a35b5050565b60045481565b600160a060020a0380841660009081526002602090815260408083203385168452825280832054938616835260019091529020546105a7908363ffffffff610a1f16565b600160a060020a0380851660009081526001602052604080822093909355908616815220546105dc908363ffffffff610a3b16565b600160a060020a038516600090815260016020526040902055610605818363ffffffff610a3b16565b600160a060020a03808616600081815260026020908152604080832033861684528252918290209490945580518681529051928716939192600080516020610a66833981519152929181900390910190a35b50505050565b60065460ff1681565b60035460009033600160a060020a039081169116146106855760006000fd5b60035460a060020a900460ff161561069d5760006000fd5b6004546106b0908363ffffffff610a1f16565b600455600160a060020a0383166000908152600160205260409020546106dc908363ffffffff610a1f16565b600160a060020a038416600081815260016020908152604091829020939093558051858152905191927f0f6798a560793a54c3bcfe86a93cde1e73087d944c0ea20544137d412139688592918290030190a25060015b5b5b92915050565b6008805460408051602060026001851615610100026000190190941693909304601f810184900484028201840190925281815292918301828280156104f35780601f106104c8576101008083540402835291602001916104f3565b820191906000526020600020905b8154815290600101906020018083116104d657829003601f168201915b505050505081565b600160a060020a0381166000908152600160205260409020545b919050565b60035460009033600160a060020a039081169116146108065760006000fd5b6003805460a060020a60ff02191660a060020a1790556040517fae5184fba832cb2b1f702aca6117b8d265eaf03ad33eb133f19dde0f5920fa0890600090a15060015b5b90565b600354600160a060020a031681565b6007805460408051602060026001851615610100026000190190941693909304601f810184900484028201840190925281815292918301828280156104f35780601f106104c8576101008083540402835291602001916104f3565b820191906000526020600020905b8154815290600101906020018083116104d657829003601f168201915b505050505081565b604060443610156108fb5760006000fd5b600160a060020a033316600090815260016020526040902054610924908363ffffffff610a3b16565b600160a060020a033381166000908152600160205260408082209390935590851681522054610959908363ffffffff610a1f16565b600160a060020a03808516600081815260016020908152604091829020949094558051868152905191933390931692600080516020610a6683398151915292918290030190a35b5b505050565b600160a060020a038083166000908152600260209081526040808320938516835292905220545b92915050565b60035433600160a060020a039081169116146109ef5760006000fd5b600160a060020a03811615610a1a5760038054600160a060020a031916600160a060020a0383161790555b5b5b50565b6000828201610a3084821015610a54565b8091505b5092915050565b6000610a4983831115610a54565b508082035b92915050565b801515610a1a5760006000fd5b5b505600ddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3efa165627a7a72305820d5f57bc1774b67ca4770c8221e36fae5ab47100903f409cf29037aa15409ee340029`

// DeployLivepeerToken deploys a new Ethereum contract, binding an instance of LivepeerToken to it.
func DeployLivepeerToken(auth *bind.TransactOpts, backend bind.ContractBackend, libraries map[string]common.Address) (common.Address, *types.Transaction, *LivepeerToken, error) {
	parsed, err := abi.JSON(strings.NewReader(LivepeerTokenABI))
	if err != nil {
		return common.Address{}, nil, nil, err
	}

	linkedBin := LivepeerTokenBin
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
	return address, tx, &LivepeerToken{LivepeerTokenCaller: LivepeerTokenCaller{contract: contract}, LivepeerTokenTransactor: LivepeerTokenTransactor{contract: contract}}, nil
}

// LivepeerToken is an auto generated Go binding around an Ethereum contract.
type LivepeerToken struct {
	LivepeerTokenCaller     // Read-only binding to the contract
	LivepeerTokenTransactor // Write-only binding to the contract
}

// LivepeerTokenCaller is an auto generated read-only Go binding around an Ethereum contract.
type LivepeerTokenCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// LivepeerTokenTransactor is an auto generated write-only Go binding around an Ethereum contract.
type LivepeerTokenTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// LivepeerTokenSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type LivepeerTokenSession struct {
	Contract     *LivepeerToken    // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// LivepeerTokenCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type LivepeerTokenCallerSession struct {
	Contract *LivepeerTokenCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts        // Call options to use throughout this session
}

// LivepeerTokenTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type LivepeerTokenTransactorSession struct {
	Contract     *LivepeerTokenTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts        // Transaction auth options to use throughout this session
}

// LivepeerTokenRaw is an auto generated low-level Go binding around an Ethereum contract.
type LivepeerTokenRaw struct {
	Contract *LivepeerToken // Generic contract binding to access the raw methods on
}

// LivepeerTokenCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type LivepeerTokenCallerRaw struct {
	Contract *LivepeerTokenCaller // Generic read-only contract binding to access the raw methods on
}

// LivepeerTokenTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type LivepeerTokenTransactorRaw struct {
	Contract *LivepeerTokenTransactor // Generic write-only contract binding to access the raw methods on
}

// NewLivepeerToken creates a new instance of LivepeerToken, bound to a specific deployed contract.
func NewLivepeerToken(address common.Address, backend bind.ContractBackend) (*LivepeerToken, error) {
	contract, err := bindLivepeerToken(address, backend, backend)
	if err != nil {
		return nil, err
	}
	return &LivepeerToken{LivepeerTokenCaller: LivepeerTokenCaller{contract: contract}, LivepeerTokenTransactor: LivepeerTokenTransactor{contract: contract}}, nil
}

// NewLivepeerTokenCaller creates a new read-only instance of LivepeerToken, bound to a specific deployed contract.
func NewLivepeerTokenCaller(address common.Address, caller bind.ContractCaller) (*LivepeerTokenCaller, error) {
	contract, err := bindLivepeerToken(address, caller, nil)
	if err != nil {
		return nil, err
	}
	return &LivepeerTokenCaller{contract: contract}, nil
}

// NewLivepeerTokenTransactor creates a new write-only instance of LivepeerToken, bound to a specific deployed contract.
func NewLivepeerTokenTransactor(address common.Address, transactor bind.ContractTransactor) (*LivepeerTokenTransactor, error) {
	contract, err := bindLivepeerToken(address, nil, transactor)
	if err != nil {
		return nil, err
	}
	return &LivepeerTokenTransactor{contract: contract}, nil
}

// bindLivepeerToken binds a generic wrapper to an already deployed contract.
func bindLivepeerToken(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor) (*bind.BoundContract, error) {
	parsed, err := abi.JSON(strings.NewReader(LivepeerTokenABI))
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, parsed, caller, transactor), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_LivepeerToken *LivepeerTokenRaw) Call(opts *bind.CallOpts, result interface{}, method string, params ...interface{}) error {
	return _LivepeerToken.Contract.LivepeerTokenCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_LivepeerToken *LivepeerTokenRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _LivepeerToken.Contract.LivepeerTokenTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_LivepeerToken *LivepeerTokenRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _LivepeerToken.Contract.LivepeerTokenTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_LivepeerToken *LivepeerTokenCallerRaw) Call(opts *bind.CallOpts, result interface{}, method string, params ...interface{}) error {
	return _LivepeerToken.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_LivepeerToken *LivepeerTokenTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _LivepeerToken.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_LivepeerToken *LivepeerTokenTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _LivepeerToken.Contract.contract.Transact(opts, method, params...)
}

// Allowance is a free data retrieval call binding the contract method 0xdd62ed3e.
//
// Solidity: function allowance(_owner address, _spender address) constant returns(remaining uint256)
func (_LivepeerToken *LivepeerTokenCaller) Allowance(opts *bind.CallOpts, _owner common.Address, _spender common.Address) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _LivepeerToken.contract.Call(opts, out, "allowance", _owner, _spender)
	return *ret0, err
}

// Allowance is a free data retrieval call binding the contract method 0xdd62ed3e.
//
// Solidity: function allowance(_owner address, _spender address) constant returns(remaining uint256)
func (_LivepeerToken *LivepeerTokenSession) Allowance(_owner common.Address, _spender common.Address) (*big.Int, error) {
	return _LivepeerToken.Contract.Allowance(&_LivepeerToken.CallOpts, _owner, _spender)
}

// Allowance is a free data retrieval call binding the contract method 0xdd62ed3e.
//
// Solidity: function allowance(_owner address, _spender address) constant returns(remaining uint256)
func (_LivepeerToken *LivepeerTokenCallerSession) Allowance(_owner common.Address, _spender common.Address) (*big.Int, error) {
	return _LivepeerToken.Contract.Allowance(&_LivepeerToken.CallOpts, _owner, _spender)
}

// BalanceOf is a free data retrieval call binding the contract method 0x70a08231.
//
// Solidity: function balanceOf(_owner address) constant returns(balance uint256)
func (_LivepeerToken *LivepeerTokenCaller) BalanceOf(opts *bind.CallOpts, _owner common.Address) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _LivepeerToken.contract.Call(opts, out, "balanceOf", _owner)
	return *ret0, err
}

// BalanceOf is a free data retrieval call binding the contract method 0x70a08231.
//
// Solidity: function balanceOf(_owner address) constant returns(balance uint256)
func (_LivepeerToken *LivepeerTokenSession) BalanceOf(_owner common.Address) (*big.Int, error) {
	return _LivepeerToken.Contract.BalanceOf(&_LivepeerToken.CallOpts, _owner)
}

// BalanceOf is a free data retrieval call binding the contract method 0x70a08231.
//
// Solidity: function balanceOf(_owner address) constant returns(balance uint256)
func (_LivepeerToken *LivepeerTokenCallerSession) BalanceOf(_owner common.Address) (*big.Int, error) {
	return _LivepeerToken.Contract.BalanceOf(&_LivepeerToken.CallOpts, _owner)
}

// Decimals is a free data retrieval call binding the contract method 0x313ce567.
//
// Solidity: function decimals() constant returns(uint8)
func (_LivepeerToken *LivepeerTokenCaller) Decimals(opts *bind.CallOpts) (uint8, error) {
	var (
		ret0 = new(uint8)
	)
	out := ret0
	err := _LivepeerToken.contract.Call(opts, out, "decimals")
	return *ret0, err
}

// Decimals is a free data retrieval call binding the contract method 0x313ce567.
//
// Solidity: function decimals() constant returns(uint8)
func (_LivepeerToken *LivepeerTokenSession) Decimals() (uint8, error) {
	return _LivepeerToken.Contract.Decimals(&_LivepeerToken.CallOpts)
}

// Decimals is a free data retrieval call binding the contract method 0x313ce567.
//
// Solidity: function decimals() constant returns(uint8)
func (_LivepeerToken *LivepeerTokenCallerSession) Decimals() (uint8, error) {
	return _LivepeerToken.Contract.Decimals(&_LivepeerToken.CallOpts)
}

// MintingFinished is a free data retrieval call binding the contract method 0x05d2035b.
//
// Solidity: function mintingFinished() constant returns(bool)
func (_LivepeerToken *LivepeerTokenCaller) MintingFinished(opts *bind.CallOpts) (bool, error) {
	var (
		ret0 = new(bool)
	)
	out := ret0
	err := _LivepeerToken.contract.Call(opts, out, "mintingFinished")
	return *ret0, err
}

// MintingFinished is a free data retrieval call binding the contract method 0x05d2035b.
//
// Solidity: function mintingFinished() constant returns(bool)
func (_LivepeerToken *LivepeerTokenSession) MintingFinished() (bool, error) {
	return _LivepeerToken.Contract.MintingFinished(&_LivepeerToken.CallOpts)
}

// MintingFinished is a free data retrieval call binding the contract method 0x05d2035b.
//
// Solidity: function mintingFinished() constant returns(bool)
func (_LivepeerToken *LivepeerTokenCallerSession) MintingFinished() (bool, error) {
	return _LivepeerToken.Contract.MintingFinished(&_LivepeerToken.CallOpts)
}

// Name is a free data retrieval call binding the contract method 0x06fdde03.
//
// Solidity: function name() constant returns(string)
func (_LivepeerToken *LivepeerTokenCaller) Name(opts *bind.CallOpts) (string, error) {
	var (
		ret0 = new(string)
	)
	out := ret0
	err := _LivepeerToken.contract.Call(opts, out, "name")
	return *ret0, err
}

// Name is a free data retrieval call binding the contract method 0x06fdde03.
//
// Solidity: function name() constant returns(string)
func (_LivepeerToken *LivepeerTokenSession) Name() (string, error) {
	return _LivepeerToken.Contract.Name(&_LivepeerToken.CallOpts)
}

// Name is a free data retrieval call binding the contract method 0x06fdde03.
//
// Solidity: function name() constant returns(string)
func (_LivepeerToken *LivepeerTokenCallerSession) Name() (string, error) {
	return _LivepeerToken.Contract.Name(&_LivepeerToken.CallOpts)
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() constant returns(address)
func (_LivepeerToken *LivepeerTokenCaller) Owner(opts *bind.CallOpts) (common.Address, error) {
	var (
		ret0 = new(common.Address)
	)
	out := ret0
	err := _LivepeerToken.contract.Call(opts, out, "owner")
	return *ret0, err
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() constant returns(address)
func (_LivepeerToken *LivepeerTokenSession) Owner() (common.Address, error) {
	return _LivepeerToken.Contract.Owner(&_LivepeerToken.CallOpts)
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() constant returns(address)
func (_LivepeerToken *LivepeerTokenCallerSession) Owner() (common.Address, error) {
	return _LivepeerToken.Contract.Owner(&_LivepeerToken.CallOpts)
}

// Symbol is a free data retrieval call binding the contract method 0x95d89b41.
//
// Solidity: function symbol() constant returns(string)
func (_LivepeerToken *LivepeerTokenCaller) Symbol(opts *bind.CallOpts) (string, error) {
	var (
		ret0 = new(string)
	)
	out := ret0
	err := _LivepeerToken.contract.Call(opts, out, "symbol")
	return *ret0, err
}

// Symbol is a free data retrieval call binding the contract method 0x95d89b41.
//
// Solidity: function symbol() constant returns(string)
func (_LivepeerToken *LivepeerTokenSession) Symbol() (string, error) {
	return _LivepeerToken.Contract.Symbol(&_LivepeerToken.CallOpts)
}

// Symbol is a free data retrieval call binding the contract method 0x95d89b41.
//
// Solidity: function symbol() constant returns(string)
func (_LivepeerToken *LivepeerTokenCallerSession) Symbol() (string, error) {
	return _LivepeerToken.Contract.Symbol(&_LivepeerToken.CallOpts)
}

// TotalSupply is a free data retrieval call binding the contract method 0x18160ddd.
//
// Solidity: function totalSupply() constant returns(uint256)
func (_LivepeerToken *LivepeerTokenCaller) TotalSupply(opts *bind.CallOpts) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _LivepeerToken.contract.Call(opts, out, "totalSupply")
	return *ret0, err
}

// TotalSupply is a free data retrieval call binding the contract method 0x18160ddd.
//
// Solidity: function totalSupply() constant returns(uint256)
func (_LivepeerToken *LivepeerTokenSession) TotalSupply() (*big.Int, error) {
	return _LivepeerToken.Contract.TotalSupply(&_LivepeerToken.CallOpts)
}

// TotalSupply is a free data retrieval call binding the contract method 0x18160ddd.
//
// Solidity: function totalSupply() constant returns(uint256)
func (_LivepeerToken *LivepeerTokenCallerSession) TotalSupply() (*big.Int, error) {
	return _LivepeerToken.Contract.TotalSupply(&_LivepeerToken.CallOpts)
}

// Version is a free data retrieval call binding the contract method 0x54fd4d50.
//
// Solidity: function version() constant returns(string)
func (_LivepeerToken *LivepeerTokenCaller) Version(opts *bind.CallOpts) (string, error) {
	var (
		ret0 = new(string)
	)
	out := ret0
	err := _LivepeerToken.contract.Call(opts, out, "version")
	return *ret0, err
}

// Version is a free data retrieval call binding the contract method 0x54fd4d50.
//
// Solidity: function version() constant returns(string)
func (_LivepeerToken *LivepeerTokenSession) Version() (string, error) {
	return _LivepeerToken.Contract.Version(&_LivepeerToken.CallOpts)
}

// Version is a free data retrieval call binding the contract method 0x54fd4d50.
//
// Solidity: function version() constant returns(string)
func (_LivepeerToken *LivepeerTokenCallerSession) Version() (string, error) {
	return _LivepeerToken.Contract.Version(&_LivepeerToken.CallOpts)
}

// Approve is a paid mutator transaction binding the contract method 0x095ea7b3.
//
// Solidity: function approve(_spender address, _value uint256) returns()
func (_LivepeerToken *LivepeerTokenTransactor) Approve(opts *bind.TransactOpts, _spender common.Address, _value *big.Int) (*types.Transaction, error) {
	return _LivepeerToken.contract.Transact(opts, "approve", _spender, _value)
}

// Approve is a paid mutator transaction binding the contract method 0x095ea7b3.
//
// Solidity: function approve(_spender address, _value uint256) returns()
func (_LivepeerToken *LivepeerTokenSession) Approve(_spender common.Address, _value *big.Int) (*types.Transaction, error) {
	return _LivepeerToken.Contract.Approve(&_LivepeerToken.TransactOpts, _spender, _value)
}

// Approve is a paid mutator transaction binding the contract method 0x095ea7b3.
//
// Solidity: function approve(_spender address, _value uint256) returns()
func (_LivepeerToken *LivepeerTokenTransactorSession) Approve(_spender common.Address, _value *big.Int) (*types.Transaction, error) {
	return _LivepeerToken.Contract.Approve(&_LivepeerToken.TransactOpts, _spender, _value)
}

// FinishMinting is a paid mutator transaction binding the contract method 0x7d64bcb4.
//
// Solidity: function finishMinting() returns(bool)
func (_LivepeerToken *LivepeerTokenTransactor) FinishMinting(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _LivepeerToken.contract.Transact(opts, "finishMinting")
}

// FinishMinting is a paid mutator transaction binding the contract method 0x7d64bcb4.
//
// Solidity: function finishMinting() returns(bool)
func (_LivepeerToken *LivepeerTokenSession) FinishMinting() (*types.Transaction, error) {
	return _LivepeerToken.Contract.FinishMinting(&_LivepeerToken.TransactOpts)
}

// FinishMinting is a paid mutator transaction binding the contract method 0x7d64bcb4.
//
// Solidity: function finishMinting() returns(bool)
func (_LivepeerToken *LivepeerTokenTransactorSession) FinishMinting() (*types.Transaction, error) {
	return _LivepeerToken.Contract.FinishMinting(&_LivepeerToken.TransactOpts)
}

// Mint is a paid mutator transaction binding the contract method 0x40c10f19.
//
// Solidity: function mint(_to address, _amount uint256) returns(bool)
func (_LivepeerToken *LivepeerTokenTransactor) Mint(opts *bind.TransactOpts, _to common.Address, _amount *big.Int) (*types.Transaction, error) {
	return _LivepeerToken.contract.Transact(opts, "mint", _to, _amount)
}

// Mint is a paid mutator transaction binding the contract method 0x40c10f19.
//
// Solidity: function mint(_to address, _amount uint256) returns(bool)
func (_LivepeerToken *LivepeerTokenSession) Mint(_to common.Address, _amount *big.Int) (*types.Transaction, error) {
	return _LivepeerToken.Contract.Mint(&_LivepeerToken.TransactOpts, _to, _amount)
}

// Mint is a paid mutator transaction binding the contract method 0x40c10f19.
//
// Solidity: function mint(_to address, _amount uint256) returns(bool)
func (_LivepeerToken *LivepeerTokenTransactorSession) Mint(_to common.Address, _amount *big.Int) (*types.Transaction, error) {
	return _LivepeerToken.Contract.Mint(&_LivepeerToken.TransactOpts, _to, _amount)
}

// Transfer is a paid mutator transaction binding the contract method 0xa9059cbb.
//
// Solidity: function transfer(_to address, _value uint256) returns()
func (_LivepeerToken *LivepeerTokenTransactor) Transfer(opts *bind.TransactOpts, _to common.Address, _value *big.Int) (*types.Transaction, error) {
	return _LivepeerToken.contract.Transact(opts, "transfer", _to, _value)
}

// Transfer is a paid mutator transaction binding the contract method 0xa9059cbb.
//
// Solidity: function transfer(_to address, _value uint256) returns()
func (_LivepeerToken *LivepeerTokenSession) Transfer(_to common.Address, _value *big.Int) (*types.Transaction, error) {
	return _LivepeerToken.Contract.Transfer(&_LivepeerToken.TransactOpts, _to, _value)
}

// Transfer is a paid mutator transaction binding the contract method 0xa9059cbb.
//
// Solidity: function transfer(_to address, _value uint256) returns()
func (_LivepeerToken *LivepeerTokenTransactorSession) Transfer(_to common.Address, _value *big.Int) (*types.Transaction, error) {
	return _LivepeerToken.Contract.Transfer(&_LivepeerToken.TransactOpts, _to, _value)
}

// TransferFrom is a paid mutator transaction binding the contract method 0x23b872dd.
//
// Solidity: function transferFrom(_from address, _to address, _value uint256) returns()
func (_LivepeerToken *LivepeerTokenTransactor) TransferFrom(opts *bind.TransactOpts, _from common.Address, _to common.Address, _value *big.Int) (*types.Transaction, error) {
	return _LivepeerToken.contract.Transact(opts, "transferFrom", _from, _to, _value)
}

// TransferFrom is a paid mutator transaction binding the contract method 0x23b872dd.
//
// Solidity: function transferFrom(_from address, _to address, _value uint256) returns()
func (_LivepeerToken *LivepeerTokenSession) TransferFrom(_from common.Address, _to common.Address, _value *big.Int) (*types.Transaction, error) {
	return _LivepeerToken.Contract.TransferFrom(&_LivepeerToken.TransactOpts, _from, _to, _value)
}

// TransferFrom is a paid mutator transaction binding the contract method 0x23b872dd.
//
// Solidity: function transferFrom(_from address, _to address, _value uint256) returns()
func (_LivepeerToken *LivepeerTokenTransactorSession) TransferFrom(_from common.Address, _to common.Address, _value *big.Int) (*types.Transaction, error) {
	return _LivepeerToken.Contract.TransferFrom(&_LivepeerToken.TransactOpts, _from, _to, _value)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(newOwner address) returns()
func (_LivepeerToken *LivepeerTokenTransactor) TransferOwnership(opts *bind.TransactOpts, newOwner common.Address) (*types.Transaction, error) {
	return _LivepeerToken.contract.Transact(opts, "transferOwnership", newOwner)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(newOwner address) returns()
func (_LivepeerToken *LivepeerTokenSession) TransferOwnership(newOwner common.Address) (*types.Transaction, error) {
	return _LivepeerToken.Contract.TransferOwnership(&_LivepeerToken.TransactOpts, newOwner)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(newOwner address) returns()
func (_LivepeerToken *LivepeerTokenTransactorSession) TransferOwnership(newOwner common.Address) (*types.Transaction, error) {
	return _LivepeerToken.Contract.TransferOwnership(&_LivepeerToken.TransactOpts, newOwner)
}
