// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package contracts

import (
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
	_ = big.NewInt
	_ = strings.NewReader
	_ = ethereum.NotFound
	_ = abi.U256
	_ = bind.Bind
	_ = common.Big1
	_ = types.BloomLookup
	_ = event.NewSubscription
)

// LivepeerVerifierABI is the input ABI used to generate the binding from.
const LivepeerVerifierABI = "[{\"constant\":true,\"inputs\":[],\"name\":\"verificationCodeHash\",\"outputs\":[{\"name\":\"\",\"type\":\"string\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"solver\",\"outputs\":[{\"name\":\"\",\"type\":\"address\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"requestCount\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"requests\",\"outputs\":[{\"name\":\"jobId\",\"type\":\"uint256\"},{\"name\":\"claimId\",\"type\":\"uint256\"},{\"name\":\"segmentNumber\",\"type\":\"uint256\"},{\"name\":\"commitHash\",\"type\":\"bytes32\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_controller\",\"type\":\"address\"}],\"name\":\"setController\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"controller\",\"outputs\":[{\"name\":\"\",\"type\":\"address\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"name\":\"_controller\",\"type\":\"address\"},{\"name\":\"_solver\",\"type\":\"address\"},{\"name\":\"_verificationCodeHash\",\"type\":\"string\"}],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"constructor\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"name\":\"requestId\",\"type\":\"uint256\"},{\"indexed\":true,\"name\":\"jobId\",\"type\":\"uint256\"},{\"indexed\":true,\"name\":\"claimId\",\"type\":\"uint256\"},{\"indexed\":false,\"name\":\"segmentNumber\",\"type\":\"uint256\"},{\"indexed\":false,\"name\":\"transcodingOptions\",\"type\":\"string\"},{\"indexed\":false,\"name\":\"dataStorageHash\",\"type\":\"string\"},{\"indexed\":false,\"name\":\"dataHash\",\"type\":\"bytes32\"},{\"indexed\":false,\"name\":\"transcodedDataHash\",\"type\":\"bytes32\"}],\"name\":\"VerifyRequest\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"name\":\"requestId\",\"type\":\"uint256\"},{\"indexed\":true,\"name\":\"jobId\",\"type\":\"uint256\"},{\"indexed\":true,\"name\":\"claimId\",\"type\":\"uint256\"},{\"indexed\":false,\"name\":\"segmentNumber\",\"type\":\"uint256\"},{\"indexed\":false,\"name\":\"result\",\"type\":\"bool\"}],\"name\":\"Callback\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"name\":\"solver\",\"type\":\"address\"}],\"name\":\"SolverUpdate\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"name\":\"controller\",\"type\":\"address\"}],\"name\":\"SetController\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"name\":\"param\",\"type\":\"string\"}],\"name\":\"ParameterUpdate\",\"type\":\"event\"},{\"constant\":false,\"inputs\":[{\"name\":\"_verificationCodeHash\",\"type\":\"string\"}],\"name\":\"setVerificationCodeHash\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_solver\",\"type\":\"address\"}],\"name\":\"setSolver\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_jobId\",\"type\":\"uint256\"},{\"name\":\"_claimId\",\"type\":\"uint256\"},{\"name\":\"_segmentNumber\",\"type\":\"uint256\"},{\"name\":\"_transcodingOptions\",\"type\":\"string\"},{\"name\":\"_dataStorageHash\",\"type\":\"string\"},{\"name\":\"_dataHashes\",\"type\":\"bytes32[2]\"}],\"name\":\"verify\",\"outputs\":[],\"payable\":true,\"stateMutability\":\"payable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_requestId\",\"type\":\"uint256\"},{\"name\":\"_result\",\"type\":\"bytes32\"}],\"name\":\"__callback\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"getPrice\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"}]"

// LivepeerVerifier is an auto generated Go binding around an Ethereum contract.
type LivepeerVerifier struct {
	LivepeerVerifierCaller     // Read-only binding to the contract
	LivepeerVerifierTransactor // Write-only binding to the contract
	LivepeerVerifierFilterer   // Log filterer for contract events
}

// LivepeerVerifierCaller is an auto generated read-only Go binding around an Ethereum contract.
type LivepeerVerifierCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// LivepeerVerifierTransactor is an auto generated write-only Go binding around an Ethereum contract.
type LivepeerVerifierTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// LivepeerVerifierFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type LivepeerVerifierFilterer struct {
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
	contract, err := bindLivepeerVerifier(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &LivepeerVerifier{LivepeerVerifierCaller: LivepeerVerifierCaller{contract: contract}, LivepeerVerifierTransactor: LivepeerVerifierTransactor{contract: contract}, LivepeerVerifierFilterer: LivepeerVerifierFilterer{contract: contract}}, nil
}

// NewLivepeerVerifierCaller creates a new read-only instance of LivepeerVerifier, bound to a specific deployed contract.
func NewLivepeerVerifierCaller(address common.Address, caller bind.ContractCaller) (*LivepeerVerifierCaller, error) {
	contract, err := bindLivepeerVerifier(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &LivepeerVerifierCaller{contract: contract}, nil
}

// NewLivepeerVerifierTransactor creates a new write-only instance of LivepeerVerifier, bound to a specific deployed contract.
func NewLivepeerVerifierTransactor(address common.Address, transactor bind.ContractTransactor) (*LivepeerVerifierTransactor, error) {
	contract, err := bindLivepeerVerifier(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &LivepeerVerifierTransactor{contract: contract}, nil
}

// NewLivepeerVerifierFilterer creates a new log filterer instance of LivepeerVerifier, bound to a specific deployed contract.
func NewLivepeerVerifierFilterer(address common.Address, filterer bind.ContractFilterer) (*LivepeerVerifierFilterer, error) {
	contract, err := bindLivepeerVerifier(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &LivepeerVerifierFilterer{contract: contract}, nil
}

// bindLivepeerVerifier binds a generic wrapper to an already deployed contract.
func bindLivepeerVerifier(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := abi.JSON(strings.NewReader(LivepeerVerifierABI))
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, parsed, caller, transactor, filterer), nil
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
// Solidity: function requests(uint256 ) constant returns(uint256 jobId, uint256 claimId, uint256 segmentNumber, bytes32 commitHash)
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
// Solidity: function requests(uint256 ) constant returns(uint256 jobId, uint256 claimId, uint256 segmentNumber, bytes32 commitHash)
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
// Solidity: function requests(uint256 ) constant returns(uint256 jobId, uint256 claimId, uint256 segmentNumber, bytes32 commitHash)
func (_LivepeerVerifier *LivepeerVerifierCallerSession) Requests(arg0 *big.Int) (struct {
	JobId         *big.Int
	ClaimId       *big.Int
	SegmentNumber *big.Int
	CommitHash    [32]byte
}, error) {
	return _LivepeerVerifier.Contract.Requests(&_LivepeerVerifier.CallOpts, arg0)
}

// Solver is a free data retrieval call binding the contract method 0x49a7a26d.
//
// Solidity: function solver() constant returns(address)
func (_LivepeerVerifier *LivepeerVerifierCaller) Solver(opts *bind.CallOpts) (common.Address, error) {
	var (
		ret0 = new(common.Address)
	)
	out := ret0
	err := _LivepeerVerifier.contract.Call(opts, out, "solver")
	return *ret0, err
}

// Solver is a free data retrieval call binding the contract method 0x49a7a26d.
//
// Solidity: function solver() constant returns(address)
func (_LivepeerVerifier *LivepeerVerifierSession) Solver() (common.Address, error) {
	return _LivepeerVerifier.Contract.Solver(&_LivepeerVerifier.CallOpts)
}

// Solver is a free data retrieval call binding the contract method 0x49a7a26d.
//
// Solidity: function solver() constant returns(address)
func (_LivepeerVerifier *LivepeerVerifierCallerSession) Solver() (common.Address, error) {
	return _LivepeerVerifier.Contract.Solver(&_LivepeerVerifier.CallOpts)
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

// Callback is a paid mutator transaction binding the contract method 0x9842a37c.
//
// Solidity: function __callback(uint256 _requestId, bytes32 _result) returns()
func (_LivepeerVerifier *LivepeerVerifierTransactor) Callback(opts *bind.TransactOpts, _requestId *big.Int, _result [32]byte) (*types.Transaction, error) {
	return _LivepeerVerifier.contract.Transact(opts, "__callback", _requestId, _result)
}

// Callback is a paid mutator transaction binding the contract method 0x9842a37c.
//
// Solidity: function __callback(uint256 _requestId, bytes32 _result) returns()
func (_LivepeerVerifier *LivepeerVerifierSession) Callback(_requestId *big.Int, _result [32]byte) (*types.Transaction, error) {
	return _LivepeerVerifier.Contract.Callback(&_LivepeerVerifier.TransactOpts, _requestId, _result)
}

// Callback is a paid mutator transaction binding the contract method 0x9842a37c.
//
// Solidity: function __callback(uint256 _requestId, bytes32 _result) returns()
func (_LivepeerVerifier *LivepeerVerifierTransactorSession) Callback(_requestId *big.Int, _result [32]byte) (*types.Transaction, error) {
	return _LivepeerVerifier.Contract.Callback(&_LivepeerVerifier.TransactOpts, _requestId, _result)
}

// SetController is a paid mutator transaction binding the contract method 0x92eefe9b.
//
// Solidity: function setController(address _controller) returns()
func (_LivepeerVerifier *LivepeerVerifierTransactor) SetController(opts *bind.TransactOpts, _controller common.Address) (*types.Transaction, error) {
	return _LivepeerVerifier.contract.Transact(opts, "setController", _controller)
}

// SetController is a paid mutator transaction binding the contract method 0x92eefe9b.
//
// Solidity: function setController(address _controller) returns()
func (_LivepeerVerifier *LivepeerVerifierSession) SetController(_controller common.Address) (*types.Transaction, error) {
	return _LivepeerVerifier.Contract.SetController(&_LivepeerVerifier.TransactOpts, _controller)
}

// SetController is a paid mutator transaction binding the contract method 0x92eefe9b.
//
// Solidity: function setController(address _controller) returns()
func (_LivepeerVerifier *LivepeerVerifierTransactorSession) SetController(_controller common.Address) (*types.Transaction, error) {
	return _LivepeerVerifier.Contract.SetController(&_LivepeerVerifier.TransactOpts, _controller)
}

// SetSolver is a paid mutator transaction binding the contract method 0x1f879433.
//
// Solidity: function setSolver(address _solver) returns()
func (_LivepeerVerifier *LivepeerVerifierTransactor) SetSolver(opts *bind.TransactOpts, _solver common.Address) (*types.Transaction, error) {
	return _LivepeerVerifier.contract.Transact(opts, "setSolver", _solver)
}

// SetSolver is a paid mutator transaction binding the contract method 0x1f879433.
//
// Solidity: function setSolver(address _solver) returns()
func (_LivepeerVerifier *LivepeerVerifierSession) SetSolver(_solver common.Address) (*types.Transaction, error) {
	return _LivepeerVerifier.Contract.SetSolver(&_LivepeerVerifier.TransactOpts, _solver)
}

// SetSolver is a paid mutator transaction binding the contract method 0x1f879433.
//
// Solidity: function setSolver(address _solver) returns()
func (_LivepeerVerifier *LivepeerVerifierTransactorSession) SetSolver(_solver common.Address) (*types.Transaction, error) {
	return _LivepeerVerifier.Contract.SetSolver(&_LivepeerVerifier.TransactOpts, _solver)
}

// SetVerificationCodeHash is a paid mutator transaction binding the contract method 0x4862e650.
//
// Solidity: function setVerificationCodeHash(string _verificationCodeHash) returns()
func (_LivepeerVerifier *LivepeerVerifierTransactor) SetVerificationCodeHash(opts *bind.TransactOpts, _verificationCodeHash string) (*types.Transaction, error) {
	return _LivepeerVerifier.contract.Transact(opts, "setVerificationCodeHash", _verificationCodeHash)
}

// SetVerificationCodeHash is a paid mutator transaction binding the contract method 0x4862e650.
//
// Solidity: function setVerificationCodeHash(string _verificationCodeHash) returns()
func (_LivepeerVerifier *LivepeerVerifierSession) SetVerificationCodeHash(_verificationCodeHash string) (*types.Transaction, error) {
	return _LivepeerVerifier.Contract.SetVerificationCodeHash(&_LivepeerVerifier.TransactOpts, _verificationCodeHash)
}

// SetVerificationCodeHash is a paid mutator transaction binding the contract method 0x4862e650.
//
// Solidity: function setVerificationCodeHash(string _verificationCodeHash) returns()
func (_LivepeerVerifier *LivepeerVerifierTransactorSession) SetVerificationCodeHash(_verificationCodeHash string) (*types.Transaction, error) {
	return _LivepeerVerifier.Contract.SetVerificationCodeHash(&_LivepeerVerifier.TransactOpts, _verificationCodeHash)
}

// Verify is a paid mutator transaction binding the contract method 0x8c118cf1.
//
// Solidity: function verify(uint256 _jobId, uint256 _claimId, uint256 _segmentNumber, string _transcodingOptions, string _dataStorageHash, bytes32[2] _dataHashes) returns()
func (_LivepeerVerifier *LivepeerVerifierTransactor) Verify(opts *bind.TransactOpts, _jobId *big.Int, _claimId *big.Int, _segmentNumber *big.Int, _transcodingOptions string, _dataStorageHash string, _dataHashes [2][32]byte) (*types.Transaction, error) {
	return _LivepeerVerifier.contract.Transact(opts, "verify", _jobId, _claimId, _segmentNumber, _transcodingOptions, _dataStorageHash, _dataHashes)
}

// Verify is a paid mutator transaction binding the contract method 0x8c118cf1.
//
// Solidity: function verify(uint256 _jobId, uint256 _claimId, uint256 _segmentNumber, string _transcodingOptions, string _dataStorageHash, bytes32[2] _dataHashes) returns()
func (_LivepeerVerifier *LivepeerVerifierSession) Verify(_jobId *big.Int, _claimId *big.Int, _segmentNumber *big.Int, _transcodingOptions string, _dataStorageHash string, _dataHashes [2][32]byte) (*types.Transaction, error) {
	return _LivepeerVerifier.Contract.Verify(&_LivepeerVerifier.TransactOpts, _jobId, _claimId, _segmentNumber, _transcodingOptions, _dataStorageHash, _dataHashes)
}

// Verify is a paid mutator transaction binding the contract method 0x8c118cf1.
//
// Solidity: function verify(uint256 _jobId, uint256 _claimId, uint256 _segmentNumber, string _transcodingOptions, string _dataStorageHash, bytes32[2] _dataHashes) returns()
func (_LivepeerVerifier *LivepeerVerifierTransactorSession) Verify(_jobId *big.Int, _claimId *big.Int, _segmentNumber *big.Int, _transcodingOptions string, _dataStorageHash string, _dataHashes [2][32]byte) (*types.Transaction, error) {
	return _LivepeerVerifier.Contract.Verify(&_LivepeerVerifier.TransactOpts, _jobId, _claimId, _segmentNumber, _transcodingOptions, _dataStorageHash, _dataHashes)
}

// LivepeerVerifierCallbackIterator is returned from FilterCallback and is used to iterate over the raw logs and unpacked data for Callback events raised by the LivepeerVerifier contract.
type LivepeerVerifierCallbackIterator struct {
	Event *LivepeerVerifierCallback // Event containing the contract specifics and raw log

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
func (it *LivepeerVerifierCallbackIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(LivepeerVerifierCallback)
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
		it.Event = new(LivepeerVerifierCallback)
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
func (it *LivepeerVerifierCallbackIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *LivepeerVerifierCallbackIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// LivepeerVerifierCallback represents a Callback event raised by the LivepeerVerifier contract.
type LivepeerVerifierCallback struct {
	RequestId     *big.Int
	JobId         *big.Int
	ClaimId       *big.Int
	SegmentNumber *big.Int
	Result        bool
	Raw           types.Log // Blockchain specific contextual infos
}

// FilterCallback is a free log retrieval operation binding the contract event 0xaa22eba262859195ec25c1d3c94f98248add6d1374bd46df08c78470225df8d3.
//
// Solidity: event Callback(uint256 indexed requestId, uint256 indexed jobId, uint256 indexed claimId, uint256 segmentNumber, bool result)
func (_LivepeerVerifier *LivepeerVerifierFilterer) FilterCallback(opts *bind.FilterOpts, requestId []*big.Int, jobId []*big.Int, claimId []*big.Int) (*LivepeerVerifierCallbackIterator, error) {

	var requestIdRule []interface{}
	for _, requestIdItem := range requestId {
		requestIdRule = append(requestIdRule, requestIdItem)
	}
	var jobIdRule []interface{}
	for _, jobIdItem := range jobId {
		jobIdRule = append(jobIdRule, jobIdItem)
	}
	var claimIdRule []interface{}
	for _, claimIdItem := range claimId {
		claimIdRule = append(claimIdRule, claimIdItem)
	}

	logs, sub, err := _LivepeerVerifier.contract.FilterLogs(opts, "Callback", requestIdRule, jobIdRule, claimIdRule)
	if err != nil {
		return nil, err
	}
	return &LivepeerVerifierCallbackIterator{contract: _LivepeerVerifier.contract, event: "Callback", logs: logs, sub: sub}, nil
}

// WatchCallback is a free log subscription operation binding the contract event 0xaa22eba262859195ec25c1d3c94f98248add6d1374bd46df08c78470225df8d3.
//
// Solidity: event Callback(uint256 indexed requestId, uint256 indexed jobId, uint256 indexed claimId, uint256 segmentNumber, bool result)
func (_LivepeerVerifier *LivepeerVerifierFilterer) WatchCallback(opts *bind.WatchOpts, sink chan<- *LivepeerVerifierCallback, requestId []*big.Int, jobId []*big.Int, claimId []*big.Int) (event.Subscription, error) {

	var requestIdRule []interface{}
	for _, requestIdItem := range requestId {
		requestIdRule = append(requestIdRule, requestIdItem)
	}
	var jobIdRule []interface{}
	for _, jobIdItem := range jobId {
		jobIdRule = append(jobIdRule, jobIdItem)
	}
	var claimIdRule []interface{}
	for _, claimIdItem := range claimId {
		claimIdRule = append(claimIdRule, claimIdItem)
	}

	logs, sub, err := _LivepeerVerifier.contract.WatchLogs(opts, "Callback", requestIdRule, jobIdRule, claimIdRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(LivepeerVerifierCallback)
				if err := _LivepeerVerifier.contract.UnpackLog(event, "Callback", log); err != nil {
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

// LivepeerVerifierParameterUpdateIterator is returned from FilterParameterUpdate and is used to iterate over the raw logs and unpacked data for ParameterUpdate events raised by the LivepeerVerifier contract.
type LivepeerVerifierParameterUpdateIterator struct {
	Event *LivepeerVerifierParameterUpdate // Event containing the contract specifics and raw log

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
func (it *LivepeerVerifierParameterUpdateIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(LivepeerVerifierParameterUpdate)
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
		it.Event = new(LivepeerVerifierParameterUpdate)
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
func (it *LivepeerVerifierParameterUpdateIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *LivepeerVerifierParameterUpdateIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// LivepeerVerifierParameterUpdate represents a ParameterUpdate event raised by the LivepeerVerifier contract.
type LivepeerVerifierParameterUpdate struct {
	Param string
	Raw   types.Log // Blockchain specific contextual infos
}

// FilterParameterUpdate is a free log retrieval operation binding the contract event 0x9f5033568d78ae30f29f01e944f97b2216493bd19d1b46d429673acff3dcd674.
//
// Solidity: event ParameterUpdate(string param)
func (_LivepeerVerifier *LivepeerVerifierFilterer) FilterParameterUpdate(opts *bind.FilterOpts) (*LivepeerVerifierParameterUpdateIterator, error) {

	logs, sub, err := _LivepeerVerifier.contract.FilterLogs(opts, "ParameterUpdate")
	if err != nil {
		return nil, err
	}
	return &LivepeerVerifierParameterUpdateIterator{contract: _LivepeerVerifier.contract, event: "ParameterUpdate", logs: logs, sub: sub}, nil
}

// WatchParameterUpdate is a free log subscription operation binding the contract event 0x9f5033568d78ae30f29f01e944f97b2216493bd19d1b46d429673acff3dcd674.
//
// Solidity: event ParameterUpdate(string param)
func (_LivepeerVerifier *LivepeerVerifierFilterer) WatchParameterUpdate(opts *bind.WatchOpts, sink chan<- *LivepeerVerifierParameterUpdate) (event.Subscription, error) {

	logs, sub, err := _LivepeerVerifier.contract.WatchLogs(opts, "ParameterUpdate")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(LivepeerVerifierParameterUpdate)
				if err := _LivepeerVerifier.contract.UnpackLog(event, "ParameterUpdate", log); err != nil {
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

// LivepeerVerifierSetControllerIterator is returned from FilterSetController and is used to iterate over the raw logs and unpacked data for SetController events raised by the LivepeerVerifier contract.
type LivepeerVerifierSetControllerIterator struct {
	Event *LivepeerVerifierSetController // Event containing the contract specifics and raw log

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
func (it *LivepeerVerifierSetControllerIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(LivepeerVerifierSetController)
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
		it.Event = new(LivepeerVerifierSetController)
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
func (it *LivepeerVerifierSetControllerIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *LivepeerVerifierSetControllerIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// LivepeerVerifierSetController represents a SetController event raised by the LivepeerVerifier contract.
type LivepeerVerifierSetController struct {
	Controller common.Address
	Raw        types.Log // Blockchain specific contextual infos
}

// FilterSetController is a free log retrieval operation binding the contract event 0x4ff638452bbf33c012645d18ae6f05515ff5f2d1dfb0cece8cbf018c60903f70.
//
// Solidity: event SetController(address controller)
func (_LivepeerVerifier *LivepeerVerifierFilterer) FilterSetController(opts *bind.FilterOpts) (*LivepeerVerifierSetControllerIterator, error) {

	logs, sub, err := _LivepeerVerifier.contract.FilterLogs(opts, "SetController")
	if err != nil {
		return nil, err
	}
	return &LivepeerVerifierSetControllerIterator{contract: _LivepeerVerifier.contract, event: "SetController", logs: logs, sub: sub}, nil
}

// WatchSetController is a free log subscription operation binding the contract event 0x4ff638452bbf33c012645d18ae6f05515ff5f2d1dfb0cece8cbf018c60903f70.
//
// Solidity: event SetController(address controller)
func (_LivepeerVerifier *LivepeerVerifierFilterer) WatchSetController(opts *bind.WatchOpts, sink chan<- *LivepeerVerifierSetController) (event.Subscription, error) {

	logs, sub, err := _LivepeerVerifier.contract.WatchLogs(opts, "SetController")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(LivepeerVerifierSetController)
				if err := _LivepeerVerifier.contract.UnpackLog(event, "SetController", log); err != nil {
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

// LivepeerVerifierSolverUpdateIterator is returned from FilterSolverUpdate and is used to iterate over the raw logs and unpacked data for SolverUpdate events raised by the LivepeerVerifier contract.
type LivepeerVerifierSolverUpdateIterator struct {
	Event *LivepeerVerifierSolverUpdate // Event containing the contract specifics and raw log

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
func (it *LivepeerVerifierSolverUpdateIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(LivepeerVerifierSolverUpdate)
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
		it.Event = new(LivepeerVerifierSolverUpdate)
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
func (it *LivepeerVerifierSolverUpdateIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *LivepeerVerifierSolverUpdateIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// LivepeerVerifierSolverUpdate represents a SolverUpdate event raised by the LivepeerVerifier contract.
type LivepeerVerifierSolverUpdate struct {
	Solver common.Address
	Raw    types.Log // Blockchain specific contextual infos
}

// FilterSolverUpdate is a free log retrieval operation binding the contract event 0xace515c35c46c2bef1424779ab5938a69fd660a490ba0a4863392ee28000666f.
//
// Solidity: event SolverUpdate(address solver)
func (_LivepeerVerifier *LivepeerVerifierFilterer) FilterSolverUpdate(opts *bind.FilterOpts) (*LivepeerVerifierSolverUpdateIterator, error) {

	logs, sub, err := _LivepeerVerifier.contract.FilterLogs(opts, "SolverUpdate")
	if err != nil {
		return nil, err
	}
	return &LivepeerVerifierSolverUpdateIterator{contract: _LivepeerVerifier.contract, event: "SolverUpdate", logs: logs, sub: sub}, nil
}

// WatchSolverUpdate is a free log subscription operation binding the contract event 0xace515c35c46c2bef1424779ab5938a69fd660a490ba0a4863392ee28000666f.
//
// Solidity: event SolverUpdate(address solver)
func (_LivepeerVerifier *LivepeerVerifierFilterer) WatchSolverUpdate(opts *bind.WatchOpts, sink chan<- *LivepeerVerifierSolverUpdate) (event.Subscription, error) {

	logs, sub, err := _LivepeerVerifier.contract.WatchLogs(opts, "SolverUpdate")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(LivepeerVerifierSolverUpdate)
				if err := _LivepeerVerifier.contract.UnpackLog(event, "SolverUpdate", log); err != nil {
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

// LivepeerVerifierVerifyRequestIterator is returned from FilterVerifyRequest and is used to iterate over the raw logs and unpacked data for VerifyRequest events raised by the LivepeerVerifier contract.
type LivepeerVerifierVerifyRequestIterator struct {
	Event *LivepeerVerifierVerifyRequest // Event containing the contract specifics and raw log

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
func (it *LivepeerVerifierVerifyRequestIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(LivepeerVerifierVerifyRequest)
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
		it.Event = new(LivepeerVerifierVerifyRequest)
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
func (it *LivepeerVerifierVerifyRequestIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *LivepeerVerifierVerifyRequestIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// LivepeerVerifierVerifyRequest represents a VerifyRequest event raised by the LivepeerVerifier contract.
type LivepeerVerifierVerifyRequest struct {
	RequestId          *big.Int
	JobId              *big.Int
	ClaimId            *big.Int
	SegmentNumber      *big.Int
	TranscodingOptions string
	DataStorageHash    string
	DataHash           [32]byte
	TranscodedDataHash [32]byte
	Raw                types.Log // Blockchain specific contextual infos
}

// FilterVerifyRequest is a free log retrieval operation binding the contract event 0xf68da1a7e850796ae5473e78db07307108751eec3461dddf5ef610db7dfaaf56.
//
// Solidity: event VerifyRequest(uint256 indexed requestId, uint256 indexed jobId, uint256 indexed claimId, uint256 segmentNumber, string transcodingOptions, string dataStorageHash, bytes32 dataHash, bytes32 transcodedDataHash)
func (_LivepeerVerifier *LivepeerVerifierFilterer) FilterVerifyRequest(opts *bind.FilterOpts, requestId []*big.Int, jobId []*big.Int, claimId []*big.Int) (*LivepeerVerifierVerifyRequestIterator, error) {

	var requestIdRule []interface{}
	for _, requestIdItem := range requestId {
		requestIdRule = append(requestIdRule, requestIdItem)
	}
	var jobIdRule []interface{}
	for _, jobIdItem := range jobId {
		jobIdRule = append(jobIdRule, jobIdItem)
	}
	var claimIdRule []interface{}
	for _, claimIdItem := range claimId {
		claimIdRule = append(claimIdRule, claimIdItem)
	}

	logs, sub, err := _LivepeerVerifier.contract.FilterLogs(opts, "VerifyRequest", requestIdRule, jobIdRule, claimIdRule)
	if err != nil {
		return nil, err
	}
	return &LivepeerVerifierVerifyRequestIterator{contract: _LivepeerVerifier.contract, event: "VerifyRequest", logs: logs, sub: sub}, nil
}

// WatchVerifyRequest is a free log subscription operation binding the contract event 0xf68da1a7e850796ae5473e78db07307108751eec3461dddf5ef610db7dfaaf56.
//
// Solidity: event VerifyRequest(uint256 indexed requestId, uint256 indexed jobId, uint256 indexed claimId, uint256 segmentNumber, string transcodingOptions, string dataStorageHash, bytes32 dataHash, bytes32 transcodedDataHash)
func (_LivepeerVerifier *LivepeerVerifierFilterer) WatchVerifyRequest(opts *bind.WatchOpts, sink chan<- *LivepeerVerifierVerifyRequest, requestId []*big.Int, jobId []*big.Int, claimId []*big.Int) (event.Subscription, error) {

	var requestIdRule []interface{}
	for _, requestIdItem := range requestId {
		requestIdRule = append(requestIdRule, requestIdItem)
	}
	var jobIdRule []interface{}
	for _, jobIdItem := range jobId {
		jobIdRule = append(jobIdRule, jobIdItem)
	}
	var claimIdRule []interface{}
	for _, claimIdItem := range claimId {
		claimIdRule = append(claimIdRule, claimIdItem)
	}

	logs, sub, err := _LivepeerVerifier.contract.WatchLogs(opts, "VerifyRequest", requestIdRule, jobIdRule, claimIdRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(LivepeerVerifierVerifyRequest)
				if err := _LivepeerVerifier.contract.UnpackLog(event, "VerifyRequest", log); err != nil {
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
