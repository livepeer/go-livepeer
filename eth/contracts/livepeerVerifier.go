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

// LivepeerVerifierABI is the input ABI used to generate the binding from.
const LivepeerVerifierABI = "[{\"constant\":true,\"inputs\":[{\"name\":\"\",\"type\":\"address\"}],\"name\":\"isSolver\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"verificationCodeHash\",\"outputs\":[{\"name\":\"\",\"type\":\"string\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"requestCount\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"requests\",\"outputs\":[{\"name\":\"jobId\",\"type\":\"uint256\"},{\"name\":\"claimId\",\"type\":\"uint256\"},{\"name\":\"segmentNumber\",\"type\":\"uint256\"},{\"name\":\"commitHash\",\"type\":\"bytes32\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"solvers\",\"outputs\":[{\"name\":\"\",\"type\":\"address\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_controller\",\"type\":\"address\"}],\"name\":\"setController\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"controller\",\"outputs\":[{\"name\":\"\",\"type\":\"address\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"name\":\"_controller\",\"type\":\"address\"},{\"name\":\"_solvers\",\"type\":\"address[]\"},{\"name\":\"_verificationCodeHash\",\"type\":\"string\"}],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"constructor\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"name\":\"requestId\",\"type\":\"uint256\"},{\"indexed\":true,\"name\":\"jobId\",\"type\":\"uint256\"},{\"indexed\":true,\"name\":\"claimId\",\"type\":\"uint256\"},{\"indexed\":false,\"name\":\"segmentNumber\",\"type\":\"uint256\"},{\"indexed\":false,\"name\":\"transcodingOptions\",\"type\":\"string\"},{\"indexed\":false,\"name\":\"dataStorageHash\",\"type\":\"string\"},{\"indexed\":false,\"name\":\"dataHash\",\"type\":\"bytes32\"},{\"indexed\":false,\"name\":\"transcodedDataHash\",\"type\":\"bytes32\"}],\"name\":\"VerifyRequest\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"name\":\"requestId\",\"type\":\"uint256\"},{\"indexed\":true,\"name\":\"jobId\",\"type\":\"uint256\"},{\"indexed\":true,\"name\":\"claimId\",\"type\":\"uint256\"},{\"indexed\":false,\"name\":\"segmentNumber\",\"type\":\"uint256\"},{\"indexed\":false,\"name\":\"result\",\"type\":\"bool\"}],\"name\":\"Callback\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"name\":\"controller\",\"type\":\"address\"}],\"name\":\"SetController\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"name\":\"param\",\"type\":\"string\"}],\"name\":\"ParameterUpdate\",\"type\":\"event\"},{\"constant\":false,\"inputs\":[{\"name\":\"_verificationCodeHash\",\"type\":\"string\"}],\"name\":\"setVerificationCodeHash\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_solver\",\"type\":\"address\"}],\"name\":\"addSolver\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_jobId\",\"type\":\"uint256\"},{\"name\":\"_claimId\",\"type\":\"uint256\"},{\"name\":\"_segmentNumber\",\"type\":\"uint256\"},{\"name\":\"_transcodingOptions\",\"type\":\"string\"},{\"name\":\"_dataStorageHash\",\"type\":\"string\"},{\"name\":\"_dataHashes\",\"type\":\"bytes32[2]\"}],\"name\":\"verify\",\"outputs\":[],\"payable\":true,\"stateMutability\":\"payable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_requestId\",\"type\":\"uint256\"},{\"name\":\"_result\",\"type\":\"bytes32\"}],\"name\":\"__callback\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"getPrice\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"}]"

// LivepeerVerifier is an auto generated Go binding around an Ethereum contract.
type LivepeerVerifier struct {
	LivepeerVerifierCaller     // Read-only binding to the contract
	LivepeerVerifierTransactor // Write-only binding to the contract
}

// LivepeerVerifierCaller is an auto generated read-only Go binding around an Ethereum contract.
type LivepeerVerifierCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// LivepeerVerifierTransactor is an auto generated write-only Go binding around an Ethereum contract.
type LivepeerVerifierTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// LivepeerVerifierSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type LivepeerVerifierSession struct {
	Contract     *LivepeerVerifier // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// LivepeerVerifierCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type LivepeerVerifierCallerSession struct {
	Contract *LivepeerVerifierCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts           // Call options to use throughout this session
}

// LivepeerVerifierTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type LivepeerVerifierTransactorSession struct {
	Contract     *LivepeerVerifierTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts           // Transaction auth options to use throughout this session
}

// LivepeerVerifierRaw is an auto generated low-level Go binding around an Ethereum contract.
type LivepeerVerifierRaw struct {
	Contract *LivepeerVerifier // Generic contract binding to access the raw methods on
}

// LivepeerVerifierCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type LivepeerVerifierCallerRaw struct {
	Contract *LivepeerVerifierCaller // Generic read-only contract binding to access the raw methods on
}

// LivepeerVerifierTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type LivepeerVerifierTransactorRaw struct {
	Contract *LivepeerVerifierTransactor // Generic write-only contract binding to access the raw methods on
}

// NewLivepeerVerifier creates a new instance of LivepeerVerifier, bound to a specific deployed contract.
func NewLivepeerVerifier(address common.Address, backend bind.ContractBackend) (*LivepeerVerifier, error) {
	contract, err := bindLivepeerVerifier(address, backend, backend)
	if err != nil {
		return nil, err
	}
	return &LivepeerVerifier{LivepeerVerifierCaller: LivepeerVerifierCaller{contract: contract}, LivepeerVerifierTransactor: LivepeerVerifierTransactor{contract: contract}}, nil
}

// NewLivepeerVerifierCaller creates a new read-only instance of LivepeerVerifier, bound to a specific deployed contract.
func NewLivepeerVerifierCaller(address common.Address, caller bind.ContractCaller) (*LivepeerVerifierCaller, error) {
	contract, err := bindLivepeerVerifier(address, caller, nil)
	if err != nil {
		return nil, err
	}
	return &LivepeerVerifierCaller{contract: contract}, nil
}

// NewLivepeerVerifierTransactor creates a new write-only instance of LivepeerVerifier, bound to a specific deployed contract.
func NewLivepeerVerifierTransactor(address common.Address, transactor bind.ContractTransactor) (*LivepeerVerifierTransactor, error) {
	contract, err := bindLivepeerVerifier(address, nil, transactor)
	if err != nil {
		return nil, err
	}
	return &LivepeerVerifierTransactor{contract: contract}, nil
}

// bindLivepeerVerifier binds a generic wrapper to an already deployed contract.
func bindLivepeerVerifier(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor) (*bind.BoundContract, error) {
	parsed, err := abi.JSON(strings.NewReader(LivepeerVerifierABI))
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, parsed, caller, transactor), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_LivepeerVerifier *LivepeerVerifierRaw) Call(opts *bind.CallOpts, result interface{}, method string, params ...interface{}) error {
	return _LivepeerVerifier.Contract.LivepeerVerifierCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_LivepeerVerifier *LivepeerVerifierRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _LivepeerVerifier.Contract.LivepeerVerifierTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_LivepeerVerifier *LivepeerVerifierRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _LivepeerVerifier.Contract.LivepeerVerifierTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_LivepeerVerifier *LivepeerVerifierCallerRaw) Call(opts *bind.CallOpts, result interface{}, method string, params ...interface{}) error {
	return _LivepeerVerifier.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_LivepeerVerifier *LivepeerVerifierTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _LivepeerVerifier.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_LivepeerVerifier *LivepeerVerifierTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _LivepeerVerifier.Contract.contract.Transact(opts, method, params...)
}

// Controller is a free data retrieval call binding the contract method 0xf77c4791.
//
// Solidity: function controller() constant returns(address)
func (_LivepeerVerifier *LivepeerVerifierCaller) Controller(opts *bind.CallOpts) (common.Address, error) {
	var (
		ret0 = new(common.Address)
	)
	out := ret0
	err := _LivepeerVerifier.contract.Call(opts, out, "controller")
	return *ret0, err
}

// Controller is a free data retrieval call binding the contract method 0xf77c4791.
//
// Solidity: function controller() constant returns(address)
func (_LivepeerVerifier *LivepeerVerifierSession) Controller() (common.Address, error) {
	return _LivepeerVerifier.Contract.Controller(&_LivepeerVerifier.CallOpts)
}

// Controller is a free data retrieval call binding the contract method 0xf77c4791.
//
// Solidity: function controller() constant returns(address)
func (_LivepeerVerifier *LivepeerVerifierCallerSession) Controller() (common.Address, error) {
	return _LivepeerVerifier.Contract.Controller(&_LivepeerVerifier.CallOpts)
}

// GetPrice is a free data retrieval call binding the contract method 0x98d5fdca.
//
// Solidity: function getPrice() constant returns(uint256)
func (_LivepeerVerifier *LivepeerVerifierCaller) GetPrice(opts *bind.CallOpts) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _LivepeerVerifier.contract.Call(opts, out, "getPrice")
	return *ret0, err
}

// GetPrice is a free data retrieval call binding the contract method 0x98d5fdca.
//
// Solidity: function getPrice() constant returns(uint256)
func (_LivepeerVerifier *LivepeerVerifierSession) GetPrice() (*big.Int, error) {
	return _LivepeerVerifier.Contract.GetPrice(&_LivepeerVerifier.CallOpts)
}

// GetPrice is a free data retrieval call binding the contract method 0x98d5fdca.
//
// Solidity: function getPrice() constant returns(uint256)
func (_LivepeerVerifier *LivepeerVerifierCallerSession) GetPrice() (*big.Int, error) {
	return _LivepeerVerifier.Contract.GetPrice(&_LivepeerVerifier.CallOpts)
}

// IsSolver is a free data retrieval call binding the contract method 0x02cc250d.
//
// Solidity: function isSolver( address) constant returns(bool)
func (_LivepeerVerifier *LivepeerVerifierCaller) IsSolver(opts *bind.CallOpts, arg0 common.Address) (bool, error) {
	var (
		ret0 = new(bool)
	)
	out := ret0
	err := _LivepeerVerifier.contract.Call(opts, out, "isSolver", arg0)
	return *ret0, err
}

// IsSolver is a free data retrieval call binding the contract method 0x02cc250d.
//
// Solidity: function isSolver( address) constant returns(bool)
func (_LivepeerVerifier *LivepeerVerifierSession) IsSolver(arg0 common.Address) (bool, error) {
	return _LivepeerVerifier.Contract.IsSolver(&_LivepeerVerifier.CallOpts, arg0)
}

// IsSolver is a free data retrieval call binding the contract method 0x02cc250d.
//
// Solidity: function isSolver( address) constant returns(bool)
func (_LivepeerVerifier *LivepeerVerifierCallerSession) IsSolver(arg0 common.Address) (bool, error) {
	return _LivepeerVerifier.Contract.IsSolver(&_LivepeerVerifier.CallOpts, arg0)
}

// RequestCount is a free data retrieval call binding the contract method 0x5badbe4c.
//
// Solidity: function requestCount() constant returns(uint256)
func (_LivepeerVerifier *LivepeerVerifierCaller) RequestCount(opts *bind.CallOpts) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _LivepeerVerifier.contract.Call(opts, out, "requestCount")
	return *ret0, err
}

// RequestCount is a free data retrieval call binding the contract method 0x5badbe4c.
//
// Solidity: function requestCount() constant returns(uint256)
func (_LivepeerVerifier *LivepeerVerifierSession) RequestCount() (*big.Int, error) {
	return _LivepeerVerifier.Contract.RequestCount(&_LivepeerVerifier.CallOpts)
}

// RequestCount is a free data retrieval call binding the contract method 0x5badbe4c.
//
// Solidity: function requestCount() constant returns(uint256)
func (_LivepeerVerifier *LivepeerVerifierCallerSession) RequestCount() (*big.Int, error) {
	return _LivepeerVerifier.Contract.RequestCount(&_LivepeerVerifier.CallOpts)
}

// Requests is a free data retrieval call binding the contract method 0x81d12c58.
//
// Solidity: function requests( uint256) constant returns(jobId uint256, claimId uint256, segmentNumber uint256, commitHash bytes32)
func (_LivepeerVerifier *LivepeerVerifierCaller) Requests(opts *bind.CallOpts, arg0 *big.Int) (struct {
	JobId         *big.Int
	ClaimId       *big.Int
	SegmentNumber *big.Int
	CommitHash    [32]byte
}, error) {
	ret := new(struct {
		JobId         *big.Int
		ClaimId       *big.Int
		SegmentNumber *big.Int
		CommitHash    [32]byte
	})
	out := ret
	err := _LivepeerVerifier.contract.Call(opts, out, "requests", arg0)
	return *ret, err
}

// Requests is a free data retrieval call binding the contract method 0x81d12c58.
//
// Solidity: function requests( uint256) constant returns(jobId uint256, claimId uint256, segmentNumber uint256, commitHash bytes32)
func (_LivepeerVerifier *LivepeerVerifierSession) Requests(arg0 *big.Int) (struct {
	JobId         *big.Int
	ClaimId       *big.Int
	SegmentNumber *big.Int
	CommitHash    [32]byte
}, error) {
	return _LivepeerVerifier.Contract.Requests(&_LivepeerVerifier.CallOpts, arg0)
}

// Requests is a free data retrieval call binding the contract method 0x81d12c58.
//
// Solidity: function requests( uint256) constant returns(jobId uint256, claimId uint256, segmentNumber uint256, commitHash bytes32)
func (_LivepeerVerifier *LivepeerVerifierCallerSession) Requests(arg0 *big.Int) (struct {
	JobId         *big.Int
	ClaimId       *big.Int
	SegmentNumber *big.Int
	CommitHash    [32]byte
}, error) {
	return _LivepeerVerifier.Contract.Requests(&_LivepeerVerifier.CallOpts, arg0)
}

// Solvers is a free data retrieval call binding the contract method 0x92ce765e.
//
// Solidity: function solvers( uint256) constant returns(address)
func (_LivepeerVerifier *LivepeerVerifierCaller) Solvers(opts *bind.CallOpts, arg0 *big.Int) (common.Address, error) {
	var (
		ret0 = new(common.Address)
	)
	out := ret0
	err := _LivepeerVerifier.contract.Call(opts, out, "solvers", arg0)
	return *ret0, err
}

// Solvers is a free data retrieval call binding the contract method 0x92ce765e.
//
// Solidity: function solvers( uint256) constant returns(address)
func (_LivepeerVerifier *LivepeerVerifierSession) Solvers(arg0 *big.Int) (common.Address, error) {
	return _LivepeerVerifier.Contract.Solvers(&_LivepeerVerifier.CallOpts, arg0)
}

// Solvers is a free data retrieval call binding the contract method 0x92ce765e.
//
// Solidity: function solvers( uint256) constant returns(address)
func (_LivepeerVerifier *LivepeerVerifierCallerSession) Solvers(arg0 *big.Int) (common.Address, error) {
	return _LivepeerVerifier.Contract.Solvers(&_LivepeerVerifier.CallOpts, arg0)
}

// VerificationCodeHash is a free data retrieval call binding the contract method 0x41af1524.
//
// Solidity: function verificationCodeHash() constant returns(string)
func (_LivepeerVerifier *LivepeerVerifierCaller) VerificationCodeHash(opts *bind.CallOpts) (string, error) {
	var (
		ret0 = new(string)
	)
	out := ret0
	err := _LivepeerVerifier.contract.Call(opts, out, "verificationCodeHash")
	return *ret0, err
}

// VerificationCodeHash is a free data retrieval call binding the contract method 0x41af1524.
//
// Solidity: function verificationCodeHash() constant returns(string)
func (_LivepeerVerifier *LivepeerVerifierSession) VerificationCodeHash() (string, error) {
	return _LivepeerVerifier.Contract.VerificationCodeHash(&_LivepeerVerifier.CallOpts)
}

// VerificationCodeHash is a free data retrieval call binding the contract method 0x41af1524.
//
// Solidity: function verificationCodeHash() constant returns(string)
func (_LivepeerVerifier *LivepeerVerifierCallerSession) VerificationCodeHash() (string, error) {
	return _LivepeerVerifier.Contract.VerificationCodeHash(&_LivepeerVerifier.CallOpts)
}

// __callback is a paid mutator transaction binding the contract method 0x9842a37c.
//
// Solidity: function __callback(_requestId uint256, _result bytes32) returns()
func (_LivepeerVerifier *LivepeerVerifierTransactor) __callback(opts *bind.TransactOpts, _requestId *big.Int, _result [32]byte) (*types.Transaction, error) {
	return _LivepeerVerifier.contract.Transact(opts, "__callback", _requestId, _result)
}

// __callback is a paid mutator transaction binding the contract method 0x9842a37c.
//
// Solidity: function __callback(_requestId uint256, _result bytes32) returns()
func (_LivepeerVerifier *LivepeerVerifierSession) __callback(_requestId *big.Int, _result [32]byte) (*types.Transaction, error) {
	return _LivepeerVerifier.Contract.__callback(&_LivepeerVerifier.TransactOpts, _requestId, _result)
}

// __callback is a paid mutator transaction binding the contract method 0x9842a37c.
//
// Solidity: function __callback(_requestId uint256, _result bytes32) returns()
func (_LivepeerVerifier *LivepeerVerifierTransactorSession) __callback(_requestId *big.Int, _result [32]byte) (*types.Transaction, error) {
	return _LivepeerVerifier.Contract.__callback(&_LivepeerVerifier.TransactOpts, _requestId, _result)
}

// AddSolver is a paid mutator transaction binding the contract method 0xec58f4b8.
//
// Solidity: function addSolver(_solver address) returns()
func (_LivepeerVerifier *LivepeerVerifierTransactor) AddSolver(opts *bind.TransactOpts, _solver common.Address) (*types.Transaction, error) {
	return _LivepeerVerifier.contract.Transact(opts, "addSolver", _solver)
}

// AddSolver is a paid mutator transaction binding the contract method 0xec58f4b8.
//
// Solidity: function addSolver(_solver address) returns()
func (_LivepeerVerifier *LivepeerVerifierSession) AddSolver(_solver common.Address) (*types.Transaction, error) {
	return _LivepeerVerifier.Contract.AddSolver(&_LivepeerVerifier.TransactOpts, _solver)
}

// AddSolver is a paid mutator transaction binding the contract method 0xec58f4b8.
//
// Solidity: function addSolver(_solver address) returns()
func (_LivepeerVerifier *LivepeerVerifierTransactorSession) AddSolver(_solver common.Address) (*types.Transaction, error) {
	return _LivepeerVerifier.Contract.AddSolver(&_LivepeerVerifier.TransactOpts, _solver)
}

// SetController is a paid mutator transaction binding the contract method 0x92eefe9b.
//
// Solidity: function setController(_controller address) returns()
func (_LivepeerVerifier *LivepeerVerifierTransactor) SetController(opts *bind.TransactOpts, _controller common.Address) (*types.Transaction, error) {
	return _LivepeerVerifier.contract.Transact(opts, "setController", _controller)
}

// SetController is a paid mutator transaction binding the contract method 0x92eefe9b.
//
// Solidity: function setController(_controller address) returns()
func (_LivepeerVerifier *LivepeerVerifierSession) SetController(_controller common.Address) (*types.Transaction, error) {
	return _LivepeerVerifier.Contract.SetController(&_LivepeerVerifier.TransactOpts, _controller)
}

// SetController is a paid mutator transaction binding the contract method 0x92eefe9b.
//
// Solidity: function setController(_controller address) returns()
func (_LivepeerVerifier *LivepeerVerifierTransactorSession) SetController(_controller common.Address) (*types.Transaction, error) {
	return _LivepeerVerifier.Contract.SetController(&_LivepeerVerifier.TransactOpts, _controller)
}

// SetVerificationCodeHash is a paid mutator transaction binding the contract method 0x4862e650.
//
// Solidity: function setVerificationCodeHash(_verificationCodeHash string) returns()
func (_LivepeerVerifier *LivepeerVerifierTransactor) SetVerificationCodeHash(opts *bind.TransactOpts, _verificationCodeHash string) (*types.Transaction, error) {
	return _LivepeerVerifier.contract.Transact(opts, "setVerificationCodeHash", _verificationCodeHash)
}

// SetVerificationCodeHash is a paid mutator transaction binding the contract method 0x4862e650.
//
// Solidity: function setVerificationCodeHash(_verificationCodeHash string) returns()
func (_LivepeerVerifier *LivepeerVerifierSession) SetVerificationCodeHash(_verificationCodeHash string) (*types.Transaction, error) {
	return _LivepeerVerifier.Contract.SetVerificationCodeHash(&_LivepeerVerifier.TransactOpts, _verificationCodeHash)
}

// SetVerificationCodeHash is a paid mutator transaction binding the contract method 0x4862e650.
//
// Solidity: function setVerificationCodeHash(_verificationCodeHash string) returns()
func (_LivepeerVerifier *LivepeerVerifierTransactorSession) SetVerificationCodeHash(_verificationCodeHash string) (*types.Transaction, error) {
	return _LivepeerVerifier.Contract.SetVerificationCodeHash(&_LivepeerVerifier.TransactOpts, _verificationCodeHash)
}

// Verify is a paid mutator transaction binding the contract method 0x8c118cf1.
//
// Solidity: function verify(_jobId uint256, _claimId uint256, _segmentNumber uint256, _transcodingOptions string, _dataStorageHash string, _dataHashes bytes32[2]) returns()
func (_LivepeerVerifier *LivepeerVerifierTransactor) Verify(opts *bind.TransactOpts, _jobId *big.Int, _claimId *big.Int, _segmentNumber *big.Int, _transcodingOptions string, _dataStorageHash string, _dataHashes [2][32]byte) (*types.Transaction, error) {
	return _LivepeerVerifier.contract.Transact(opts, "verify", _jobId, _claimId, _segmentNumber, _transcodingOptions, _dataStorageHash, _dataHashes)
}

// Verify is a paid mutator transaction binding the contract method 0x8c118cf1.
//
// Solidity: function verify(_jobId uint256, _claimId uint256, _segmentNumber uint256, _transcodingOptions string, _dataStorageHash string, _dataHashes bytes32[2]) returns()
func (_LivepeerVerifier *LivepeerVerifierSession) Verify(_jobId *big.Int, _claimId *big.Int, _segmentNumber *big.Int, _transcodingOptions string, _dataStorageHash string, _dataHashes [2][32]byte) (*types.Transaction, error) {
	return _LivepeerVerifier.Contract.Verify(&_LivepeerVerifier.TransactOpts, _jobId, _claimId, _segmentNumber, _transcodingOptions, _dataStorageHash, _dataHashes)
}

// Verify is a paid mutator transaction binding the contract method 0x8c118cf1.
//
// Solidity: function verify(_jobId uint256, _claimId uint256, _segmentNumber uint256, _transcodingOptions string, _dataStorageHash string, _dataHashes bytes32[2]) returns()
func (_LivepeerVerifier *LivepeerVerifierTransactorSession) Verify(_jobId *big.Int, _claimId *big.Int, _segmentNumber *big.Int, _transcodingOptions string, _dataStorageHash string, _dataHashes [2][32]byte) (*types.Transaction, error) {
	return _LivepeerVerifier.Contract.Verify(&_LivepeerVerifier.TransactOpts, _jobId, _claimId, _segmentNumber, _transcodingOptions, _dataStorageHash, _dataHashes)
}
