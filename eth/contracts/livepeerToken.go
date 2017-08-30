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

// LivepeerTokenABI is the input ABI used to generate the binding from.
const LivepeerTokenABI = "[{\"constant\":true,\"inputs\":[],\"name\":\"mintingFinished\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"name\",\"outputs\":[{\"name\":\"\",\"type\":\"string\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_spender\",\"type\":\"address\"},{\"name\":\"_value\",\"type\":\"uint256\"}],\"name\":\"approve\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"totalSupply\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_from\",\"type\":\"address\"},{\"name\":\"_to\",\"type\":\"address\"},{\"name\":\"_value\",\"type\":\"uint256\"}],\"name\":\"transferFrom\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"decimals\",\"outputs\":[{\"name\":\"\",\"type\":\"uint8\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_to\",\"type\":\"address\"},{\"name\":\"_amount\",\"type\":\"uint256\"}],\"name\":\"mint\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"version\",\"outputs\":[{\"name\":\"\",\"type\":\"string\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"_owner\",\"type\":\"address\"}],\"name\":\"balanceOf\",\"outputs\":[{\"name\":\"balance\",\"type\":\"uint256\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":false,\"inputs\":[],\"name\":\"finishMinting\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"owner\",\"outputs\":[{\"name\":\"\",\"type\":\"address\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"symbol\",\"outputs\":[{\"name\":\"\",\"type\":\"string\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_to\",\"type\":\"address\"},{\"name\":\"_value\",\"type\":\"uint256\"}],\"name\":\"transfer\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"_owner\",\"type\":\"address\"},{\"name\":\"_spender\",\"type\":\"address\"}],\"name\":\"allowance\",\"outputs\":[{\"name\":\"remaining\",\"type\":\"uint256\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"newOwner\",\"type\":\"address\"}],\"name\":\"transferOwnership\",\"outputs\":[],\"payable\":false,\"type\":\"function\"},{\"inputs\":[],\"payable\":false,\"type\":\"constructor\"},{\"payable\":false,\"type\":\"fallback\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"name\":\"to\",\"type\":\"address\"},{\"indexed\":false,\"name\":\"amount\",\"type\":\"uint256\"}],\"name\":\"Mint\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[],\"name\":\"MintFinished\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"name\":\"owner\",\"type\":\"address\"},{\"indexed\":true,\"name\":\"spender\",\"type\":\"address\"},{\"indexed\":false,\"name\":\"value\",\"type\":\"uint256\"}],\"name\":\"Approval\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"name\":\"from\",\"type\":\"address\"},{\"indexed\":true,\"name\":\"to\",\"type\":\"address\"},{\"indexed\":false,\"name\":\"value\",\"type\":\"uint256\"}],\"name\":\"Transfer\",\"type\":\"event\"}]"

// LivepeerTokenBin is the compiled bytecode used for deploying new contracts.
const LivepeerTokenBin = `0x606060409081526003805460a060020a60ff02191690558051908101604052600e81527f4c6976657065657220546f6b656e0000000000000000000000000000000000006020820152600490805161005b929160200190610128565b506005805460ff1916601217905560408051908101604052600381527f4c50540000000000000000000000000000000000000000000000000000000000602082015260069080516100b0929160200190610128565b5060408051908101604052600381527f302e310000000000000000000000000000000000000000000000000000000000602082015260079080516100f8929160200190610128565b50341561010457600080fd5b5b5b60038054600160a060020a03191633600160a060020a03161790555b5b6101c8565b828054600181600116156101000203166002900490600052602060002090601f016020900481019282601f1061016957805160ff1916838001178555610196565b82800160010185558215610196579182015b8281111561019657825182559160200191906001019061017b565b5b506101a39291506101a7565b5090565b6101c591905b808211156101a357600081556001016101ad565b5090565b90565b610bbd806101d76000396000f300606060405236156100d85763ffffffff7c010000000000000000000000000000000000000000000000000000000060003504166305d2035b81146100eb57806306fdde0314610112578063095ea7b31461019d57806318160ddd146101d357806323b872dd146101f8578063313ce5671461023457806340c10f191461025d57806354fd4d501461029357806370a082311461031e5780637d64bcb41461034f5780638da5cb5b1461037657806395d89b41146103a5578063a9059cbb14610430578063dd62ed3e14610466578063f2fde38b1461049d575b34156100e357600080fd5b5b600080fd5b005b34156100f657600080fd5b6100fe6104be565b604051901515815260200160405180910390f35b341561011d57600080fd5b6101256104df565b60405160208082528190810183818151815260200191508051906020019080838360005b838110156101625780820151818401525b602001610149565b50505050905090810190601f16801561018f5780820380516001836020036101000a031916815260200191505b509250505060405180910390f35b34156101a857600080fd5b6100fe600160a060020a036004351660243561057d565b604051901515815260200160405180910390f35b34156101de57600080fd5b6101e6610624565b60405190815260200160405180910390f35b341561020357600080fd5b6100fe600160a060020a036004358116906024351660443561062a565b604051901515815260200160405180910390f35b341561023f57600080fd5b61024761073f565b60405160ff909116815260200160405180910390f35b341561026857600080fd5b6100fe600160a060020a0360043516602435610748565b604051901515815260200160405180910390f35b341561029e57600080fd5b61012561082a565b60405160208082528190810183818151815260200191508051906020019080838360005b838110156101625780820151818401525b602001610149565b50505050905090810190601f16801561018f5780820380516001836020036101000a031916815260200191505b509250505060405180910390f35b341561032957600080fd5b6101e6600160a060020a03600435166108c8565b60405190815260200160405180910390f35b341561035a57600080fd5b6100fe6108e7565b604051901515815260200160405180910390f35b341561038157600080fd5b61038961096e565b604051600160a060020a03909116815260200160405180910390f35b34156103b057600080fd5b61012561097d565b60405160208082528190810183818151815260200191508051906020019080838360005b838110156101625780820151818401525b602001610149565b50505050905090810190601f16801561018f5780820380516001836020036101000a031916815260200191505b509250505060405180910390f35b341561043b57600080fd5b6100fe600160a060020a0360043516602435610a1b565b604051901515815260200160405180910390f35b341561047157600080fd5b6101e6600160a060020a0360043581169060243516610adb565b60405190815260200160405180910390f35b34156104a857600080fd5b6100e9600160a060020a0360043516610b08565b005b60035474010000000000000000000000000000000000000000900460ff1681565b60048054600181600116156101000203166002900480601f0160208091040260200160405190810160405280929190818152602001828054600181600116156101000203166002900480156105755780601f1061054a57610100808354040283529160200191610575565b820191906000526020600020905b81548152906001019060200180831161055857829003601f168201915b505050505081565b60008115806105af5750600160a060020a03338116600090815260026020908152604080832093871683529290522054155b15156105ba57600080fd5b600160a060020a03338116600081815260026020908152604080832094881680845294909152908190208590557f8c5be1e5ebec7d5bd14f71427d1e84f3dd0314c0f7b2291e5b200ac8c7c3b9259085905190815260200160405180910390a35060015b92915050565b60005481565b600160a060020a038084166000908152600260209081526040808320338516845282528083205493861683526001909152812054909190610671908463ffffffff610b6016565b600160a060020a0380861660009081526001602052604080822093909355908716815220546106a6908463ffffffff610b7a16565b600160a060020a0386166000908152600160205260409020556106cf818463ffffffff610b7a16565b600160a060020a03808716600081815260026020908152604080832033861684529091529081902093909355908616917fddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef9086905190815260200160405180910390a3600191505b509392505050565b60055460ff1681565b60035460009033600160a060020a0390811691161461076657600080fd5b60035474010000000000000000000000000000000000000000900460ff161561078e57600080fd5b6000546107a1908363ffffffff610b6016565b6000908155600160a060020a0384168152600160205260409020546107cc908363ffffffff610b6016565b600160a060020a0384166000818152600160205260409081902092909255907f0f6798a560793a54c3bcfe86a93cde1e73087d944c0ea20544137d41213968859084905190815260200160405180910390a25060015b5b5b92915050565b60078054600181600116156101000203166002900480601f0160208091040260200160405190810160405280929190818152602001828054600181600116156101000203166002900480156105755780601f1061054a57610100808354040283529160200191610575565b820191906000526020600020905b81548152906001019060200180831161055857829003601f168201915b505050505081565b600160a060020a0381166000908152600160205260409020545b919050565b60035460009033600160a060020a0390811691161461090557600080fd5b6003805474ff00000000000000000000000000000000000000001916740100000000000000000000000000000000000000001790557fae5184fba832cb2b1f702aca6117b8d265eaf03ad33eb133f19dde0f5920fa0860405160405180910390a15060015b5b90565b600354600160a060020a031681565b60068054600181600116156101000203166002900480601f0160208091040260200160405190810160405280929190818152602001828054600181600116156101000203166002900480156105755780601f1061054a57610100808354040283529160200191610575565b820191906000526020600020905b81548152906001019060200180831161055857829003601f168201915b505050505081565b600160a060020a033316600090815260016020526040812054610a44908363ffffffff610b7a16565b600160a060020a033381166000908152600160205260408082209390935590851681522054610a79908363ffffffff610b6016565b600160a060020a0380851660008181526001602052604090819020939093559133909116907fddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef9085905190815260200160405180910390a35060015b92915050565b600160a060020a038083166000908152600260209081526040808320938516835292905220545b92915050565b60035433600160a060020a03908116911614610b2357600080fd5b600160a060020a03811615610b5b576003805473ffffffffffffffffffffffffffffffffffffffff1916600160a060020a0383161790555b5b5b50565b600082820183811015610b6f57fe5b8091505b5092915050565b600082821115610b8657fe5b508082035b929150505600a165627a7a72305820997c3045485c55ae6445c160e40940793f24f8a92f7e900371b40f97ebd4bdb60029`

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

		linkedBin = reg.ReplaceAllString(linkedBin, addr.Hex())
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
// Solidity: function approve(_spender address, _value uint256) returns(bool)
func (_LivepeerToken *LivepeerTokenTransactor) Approve(opts *bind.TransactOpts, _spender common.Address, _value *big.Int) (*types.Transaction, error) {
	return _LivepeerToken.contract.Transact(opts, "approve", _spender, _value)
}

// Approve is a paid mutator transaction binding the contract method 0x095ea7b3.
//
// Solidity: function approve(_spender address, _value uint256) returns(bool)
func (_LivepeerToken *LivepeerTokenSession) Approve(_spender common.Address, _value *big.Int) (*types.Transaction, error) {
	return _LivepeerToken.Contract.Approve(&_LivepeerToken.TransactOpts, _spender, _value)
}

// Approve is a paid mutator transaction binding the contract method 0x095ea7b3.
//
// Solidity: function approve(_spender address, _value uint256) returns(bool)
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
// Solidity: function transfer(_to address, _value uint256) returns(bool)
func (_LivepeerToken *LivepeerTokenTransactor) Transfer(opts *bind.TransactOpts, _to common.Address, _value *big.Int) (*types.Transaction, error) {
	return _LivepeerToken.contract.Transact(opts, "transfer", _to, _value)
}

// Transfer is a paid mutator transaction binding the contract method 0xa9059cbb.
//
// Solidity: function transfer(_to address, _value uint256) returns(bool)
func (_LivepeerToken *LivepeerTokenSession) Transfer(_to common.Address, _value *big.Int) (*types.Transaction, error) {
	return _LivepeerToken.Contract.Transfer(&_LivepeerToken.TransactOpts, _to, _value)
}

// Transfer is a paid mutator transaction binding the contract method 0xa9059cbb.
//
// Solidity: function transfer(_to address, _value uint256) returns(bool)
func (_LivepeerToken *LivepeerTokenTransactorSession) Transfer(_to common.Address, _value *big.Int) (*types.Transaction, error) {
	return _LivepeerToken.Contract.Transfer(&_LivepeerToken.TransactOpts, _to, _value)
}

// TransferFrom is a paid mutator transaction binding the contract method 0x23b872dd.
//
// Solidity: function transferFrom(_from address, _to address, _value uint256) returns(bool)
func (_LivepeerToken *LivepeerTokenTransactor) TransferFrom(opts *bind.TransactOpts, _from common.Address, _to common.Address, _value *big.Int) (*types.Transaction, error) {
	return _LivepeerToken.contract.Transact(opts, "transferFrom", _from, _to, _value)
}

// TransferFrom is a paid mutator transaction binding the contract method 0x23b872dd.
//
// Solidity: function transferFrom(_from address, _to address, _value uint256) returns(bool)
func (_LivepeerToken *LivepeerTokenSession) TransferFrom(_from common.Address, _to common.Address, _value *big.Int) (*types.Transaction, error) {
	return _LivepeerToken.Contract.TransferFrom(&_LivepeerToken.TransactOpts, _from, _to, _value)
}

// TransferFrom is a paid mutator transaction binding the contract method 0x23b872dd.
//
// Solidity: function transferFrom(_from address, _to address, _value uint256) returns(bool)
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
