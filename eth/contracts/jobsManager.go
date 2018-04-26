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

// JobsManagerABI is the input ABI used to generate the binding from.
const JobsManagerABI = "[{\"constant\":true,\"inputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"jobs\",\"outputs\":[{\"name\":\"jobId\",\"type\":\"uint256\"},{\"name\":\"streamId\",\"type\":\"string\"},{\"name\":\"transcodingOptions\",\"type\":\"string\"},{\"name\":\"maxPricePerSegment\",\"type\":\"uint256\"},{\"name\":\"broadcasterAddress\",\"type\":\"address\"},{\"name\":\"transcoderAddress\",\"type\":\"address\"},{\"name\":\"creationRound\",\"type\":\"uint256\"},{\"name\":\"creationBlock\",\"type\":\"uint256\"},{\"name\":\"endBlock\",\"type\":\"uint256\"},{\"name\":\"escrow\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"finderFee\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"missedVerificationSlashAmount\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"targetContractId\",\"outputs\":[{\"name\":\"\",\"type\":\"bytes32\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"doubleClaimSegmentSlashAmount\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"verificationRate\",\"outputs\":[{\"name\":\"\",\"type\":\"uint64\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"\",\"type\":\"address\"}],\"name\":\"broadcasters\",\"outputs\":[{\"name\":\"deposit\",\"type\":\"uint256\"},{\"name\":\"withdrawBlock\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"numJobs\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_controller\",\"type\":\"address\"}],\"name\":\"setController\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"verificationSlashingPeriod\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"verificationPeriod\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"failedVerificationSlashAmount\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"controller\",\"outputs\":[{\"name\":\"\",\"type\":\"address\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"name\":\"_controller\",\"type\":\"address\"}],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"constructor\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"name\":\"broadcaster\",\"type\":\"address\"},{\"indexed\":false,\"name\":\"amount\",\"type\":\"uint256\"}],\"name\":\"Deposit\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"name\":\"broadcaster\",\"type\":\"address\"}],\"name\":\"Withdraw\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"name\":\"broadcaster\",\"type\":\"address\"},{\"indexed\":false,\"name\":\"jobId\",\"type\":\"uint256\"},{\"indexed\":false,\"name\":\"streamId\",\"type\":\"string\"},{\"indexed\":false,\"name\":\"transcodingOptions\",\"type\":\"string\"},{\"indexed\":false,\"name\":\"maxPricePerSegment\",\"type\":\"uint256\"},{\"indexed\":false,\"name\":\"creationBlock\",\"type\":\"uint256\"}],\"name\":\"NewJob\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"name\":\"transcoder\",\"type\":\"address\"},{\"indexed\":true,\"name\":\"jobId\",\"type\":\"uint256\"},{\"indexed\":false,\"name\":\"claimId\",\"type\":\"uint256\"}],\"name\":\"NewClaim\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"name\":\"transcoder\",\"type\":\"address\"},{\"indexed\":true,\"name\":\"jobId\",\"type\":\"uint256\"},{\"indexed\":true,\"name\":\"claimId\",\"type\":\"uint256\"},{\"indexed\":false,\"name\":\"segmentNumber\",\"type\":\"uint256\"}],\"name\":\"Verify\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"name\":\"transcoder\",\"type\":\"address\"},{\"indexed\":true,\"name\":\"jobId\",\"type\":\"uint256\"},{\"indexed\":true,\"name\":\"claimId\",\"type\":\"uint256\"},{\"indexed\":false,\"name\":\"fees\",\"type\":\"uint256\"}],\"name\":\"DistributeFees\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"name\":\"transcoder\",\"type\":\"address\"},{\"indexed\":true,\"name\":\"jobId\",\"type\":\"uint256\"},{\"indexed\":true,\"name\":\"claimId\",\"type\":\"uint256\"},{\"indexed\":false,\"name\":\"segmentNumber\",\"type\":\"uint256\"}],\"name\":\"PassedVerification\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"name\":\"transcoder\",\"type\":\"address\"},{\"indexed\":true,\"name\":\"jobId\",\"type\":\"uint256\"},{\"indexed\":true,\"name\":\"claimId\",\"type\":\"uint256\"},{\"indexed\":false,\"name\":\"segmentNumber\",\"type\":\"uint256\"}],\"name\":\"FailedVerification\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"name\":\"controller\",\"type\":\"address\"}],\"name\":\"SetController\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"name\":\"param\",\"type\":\"string\"}],\"name\":\"ParameterUpdate\",\"type\":\"event\"},{\"constant\":false,\"inputs\":[{\"name\":\"_verificationRate\",\"type\":\"uint64\"}],\"name\":\"setVerificationRate\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_verificationPeriod\",\"type\":\"uint256\"}],\"name\":\"setVerificationPeriod\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_verificationSlashingPeriod\",\"type\":\"uint256\"}],\"name\":\"setVerificationSlashingPeriod\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_failedVerificationSlashAmount\",\"type\":\"uint256\"}],\"name\":\"setFailedVerificationSlashAmount\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_missedVerificationSlashAmount\",\"type\":\"uint256\"}],\"name\":\"setMissedVerificationSlashAmount\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_doubleClaimSegmentSlashAmount\",\"type\":\"uint256\"}],\"name\":\"setDoubleClaimSegmentSlashAmount\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_finderFee\",\"type\":\"uint256\"}],\"name\":\"setFinderFee\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[],\"name\":\"deposit\",\"outputs\":[],\"payable\":true,\"stateMutability\":\"payable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[],\"name\":\"withdraw\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_streamId\",\"type\":\"string\"},{\"name\":\"_transcodingOptions\",\"type\":\"string\"},{\"name\":\"_maxPricePerSegment\",\"type\":\"uint256\"},{\"name\":\"_endBlock\",\"type\":\"uint256\"}],\"name\":\"job\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_jobId\",\"type\":\"uint256\"},{\"name\":\"_segmentRange\",\"type\":\"uint256[2]\"},{\"name\":\"_claimRoot\",\"type\":\"bytes32\"}],\"name\":\"claimWork\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_jobId\",\"type\":\"uint256\"},{\"name\":\"_claimId\",\"type\":\"uint256\"},{\"name\":\"_segmentNumber\",\"type\":\"uint256\"},{\"name\":\"_dataStorageHash\",\"type\":\"string\"},{\"name\":\"_dataHashes\",\"type\":\"bytes32[2]\"},{\"name\":\"_broadcasterSig\",\"type\":\"bytes\"},{\"name\":\"_proof\",\"type\":\"bytes\"}],\"name\":\"verify\",\"outputs\":[],\"payable\":true,\"stateMutability\":\"payable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_jobId\",\"type\":\"uint256\"},{\"name\":\"_claimId\",\"type\":\"uint256\"},{\"name\":\"_segmentNumber\",\"type\":\"uint256\"},{\"name\":\"_result\",\"type\":\"bool\"}],\"name\":\"receiveVerification\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_jobId\",\"type\":\"uint256\"},{\"name\":\"_claimIds\",\"type\":\"uint256[]\"}],\"name\":\"batchDistributeFees\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_jobId\",\"type\":\"uint256\"},{\"name\":\"_claimId\",\"type\":\"uint256\"},{\"name\":\"_segmentNumber\",\"type\":\"uint256\"}],\"name\":\"missedVerificationSlash\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_jobId\",\"type\":\"uint256\"},{\"name\":\"_claimId1\",\"type\":\"uint256\"},{\"name\":\"_claimId2\",\"type\":\"uint256\"},{\"name\":\"_segmentNumber\",\"type\":\"uint256\"}],\"name\":\"doubleClaimSegmentSlash\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_jobId\",\"type\":\"uint256\"},{\"name\":\"_claimId\",\"type\":\"uint256\"}],\"name\":\"distributeFees\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"_jobId\",\"type\":\"uint256\"}],\"name\":\"jobStatus\",\"outputs\":[{\"name\":\"\",\"type\":\"uint8\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"_jobId\",\"type\":\"uint256\"}],\"name\":\"getJob\",\"outputs\":[{\"name\":\"streamId\",\"type\":\"string\"},{\"name\":\"transcodingOptions\",\"type\":\"string\"},{\"name\":\"maxPricePerSegment\",\"type\":\"uint256\"},{\"name\":\"broadcasterAddress\",\"type\":\"address\"},{\"name\":\"transcoderAddress\",\"type\":\"address\"},{\"name\":\"creationRound\",\"type\":\"uint256\"},{\"name\":\"creationBlock\",\"type\":\"uint256\"},{\"name\":\"endBlock\",\"type\":\"uint256\"},{\"name\":\"escrow\",\"type\":\"uint256\"},{\"name\":\"totalClaims\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"_jobId\",\"type\":\"uint256\"},{\"name\":\"_claimId\",\"type\":\"uint256\"}],\"name\":\"getClaim\",\"outputs\":[{\"name\":\"segmentRange\",\"type\":\"uint256[2]\"},{\"name\":\"claimRoot\",\"type\":\"bytes32\"},{\"name\":\"claimBlock\",\"type\":\"uint256\"},{\"name\":\"endVerificationBlock\",\"type\":\"uint256\"},{\"name\":\"endVerificationSlashingBlock\",\"type\":\"uint256\"},{\"name\":\"status\",\"type\":\"uint8\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"_jobId\",\"type\":\"uint256\"},{\"name\":\"_claimId\",\"type\":\"uint256\"},{\"name\":\"_segmentNumber\",\"type\":\"uint256\"}],\"name\":\"isClaimSegmentVerified\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"}]"

// JobsManager is an auto generated Go binding around an Ethereum contract.
type JobsManager struct {
	JobsManagerCaller     // Read-only binding to the contract
	JobsManagerTransactor // Write-only binding to the contract
	JobsManagerFilterer   // Log filterer for contract events
}

// JobsManagerCaller is an auto generated read-only Go binding around an Ethereum contract.
type JobsManagerCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// JobsManagerTransactor is an auto generated write-only Go binding around an Ethereum contract.
type JobsManagerTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// JobsManagerFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type JobsManagerFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// JobsManagerSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type JobsManagerSession struct {
	Contract     *JobsManager      // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// JobsManagerCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type JobsManagerCallerSession struct {
	Contract *JobsManagerCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts      // Call options to use throughout this session
}

// JobsManagerTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type JobsManagerTransactorSession struct {
	Contract     *JobsManagerTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts      // Transaction auth options to use throughout this session
}

// JobsManagerRaw is an auto generated low-level Go binding around an Ethereum contract.
type JobsManagerRaw struct {
	Contract *JobsManager // Generic contract binding to access the raw methods on
}

// JobsManagerCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type JobsManagerCallerRaw struct {
	Contract *JobsManagerCaller // Generic read-only contract binding to access the raw methods on
}

// JobsManagerTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type JobsManagerTransactorRaw struct {
	Contract *JobsManagerTransactor // Generic write-only contract binding to access the raw methods on
}

// NewJobsManager creates a new instance of JobsManager, bound to a specific deployed contract.
func NewJobsManager(address common.Address, backend bind.ContractBackend) (*JobsManager, error) {
	contract, err := bindJobsManager(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &JobsManager{JobsManagerCaller: JobsManagerCaller{contract: contract}, JobsManagerTransactor: JobsManagerTransactor{contract: contract}, JobsManagerFilterer: JobsManagerFilterer{contract: contract}}, nil
}

// NewJobsManagerCaller creates a new read-only instance of JobsManager, bound to a specific deployed contract.
func NewJobsManagerCaller(address common.Address, caller bind.ContractCaller) (*JobsManagerCaller, error) {
	contract, err := bindJobsManager(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &JobsManagerCaller{contract: contract}, nil
}

// NewJobsManagerTransactor creates a new write-only instance of JobsManager, bound to a specific deployed contract.
func NewJobsManagerTransactor(address common.Address, transactor bind.ContractTransactor) (*JobsManagerTransactor, error) {
	contract, err := bindJobsManager(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &JobsManagerTransactor{contract: contract}, nil
}

// NewJobsManagerFilterer creates a new log filterer instance of JobsManager, bound to a specific deployed contract.
func NewJobsManagerFilterer(address common.Address, filterer bind.ContractFilterer) (*JobsManagerFilterer, error) {
	contract, err := bindJobsManager(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &JobsManagerFilterer{contract: contract}, nil
}

// bindJobsManager binds a generic wrapper to an already deployed contract.
func bindJobsManager(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := abi.JSON(strings.NewReader(JobsManagerABI))
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_JobsManager *JobsManagerRaw) Call(opts *bind.CallOpts, result interface{}, method string, params ...interface{}) error {
	return _JobsManager.Contract.JobsManagerCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_JobsManager *JobsManagerRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _JobsManager.Contract.JobsManagerTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_JobsManager *JobsManagerRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _JobsManager.Contract.JobsManagerTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_JobsManager *JobsManagerCallerRaw) Call(opts *bind.CallOpts, result interface{}, method string, params ...interface{}) error {
	return _JobsManager.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_JobsManager *JobsManagerTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _JobsManager.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_JobsManager *JobsManagerTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _JobsManager.Contract.contract.Transact(opts, method, params...)
}

// Broadcasters is a free data retrieval call binding the contract method 0x88cc1093.
//
// Solidity: function broadcasters( address) constant returns(deposit uint256, withdrawBlock uint256)
func (_JobsManager *JobsManagerCaller) Broadcasters(opts *bind.CallOpts, arg0 common.Address) (struct {
	Deposit       *big.Int
	WithdrawBlock *big.Int
}, error) {
	ret := new(struct {
		Deposit       *big.Int
		WithdrawBlock *big.Int
	})
	out := ret
	err := _JobsManager.contract.Call(opts, out, "broadcasters", arg0)
	return *ret, err
}

// Broadcasters is a free data retrieval call binding the contract method 0x88cc1093.
//
// Solidity: function broadcasters( address) constant returns(deposit uint256, withdrawBlock uint256)
func (_JobsManager *JobsManagerSession) Broadcasters(arg0 common.Address) (struct {
	Deposit       *big.Int
	WithdrawBlock *big.Int
}, error) {
	return _JobsManager.Contract.Broadcasters(&_JobsManager.CallOpts, arg0)
}

// Broadcasters is a free data retrieval call binding the contract method 0x88cc1093.
//
// Solidity: function broadcasters( address) constant returns(deposit uint256, withdrawBlock uint256)
func (_JobsManager *JobsManagerCallerSession) Broadcasters(arg0 common.Address) (struct {
	Deposit       *big.Int
	WithdrawBlock *big.Int
}, error) {
	return _JobsManager.Contract.Broadcasters(&_JobsManager.CallOpts, arg0)
}

// Controller is a free data retrieval call binding the contract method 0xf77c4791.
//
// Solidity: function controller() constant returns(address)
func (_JobsManager *JobsManagerCaller) Controller(opts *bind.CallOpts) (common.Address, error) {
	var (
		ret0 = new(common.Address)
	)
	out := ret0
	err := _JobsManager.contract.Call(opts, out, "controller")
	return *ret0, err
}

// Controller is a free data retrieval call binding the contract method 0xf77c4791.
//
// Solidity: function controller() constant returns(address)
func (_JobsManager *JobsManagerSession) Controller() (common.Address, error) {
	return _JobsManager.Contract.Controller(&_JobsManager.CallOpts)
}

// Controller is a free data retrieval call binding the contract method 0xf77c4791.
//
// Solidity: function controller() constant returns(address)
func (_JobsManager *JobsManagerCallerSession) Controller() (common.Address, error) {
	return _JobsManager.Contract.Controller(&_JobsManager.CallOpts)
}

// DoubleClaimSegmentSlashAmount is a free data retrieval call binding the contract method 0x6d7221d5.
//
// Solidity: function doubleClaimSegmentSlashAmount() constant returns(uint256)
func (_JobsManager *JobsManagerCaller) DoubleClaimSegmentSlashAmount(opts *bind.CallOpts) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _JobsManager.contract.Call(opts, out, "doubleClaimSegmentSlashAmount")
	return *ret0, err
}

// DoubleClaimSegmentSlashAmount is a free data retrieval call binding the contract method 0x6d7221d5.
//
// Solidity: function doubleClaimSegmentSlashAmount() constant returns(uint256)
func (_JobsManager *JobsManagerSession) DoubleClaimSegmentSlashAmount() (*big.Int, error) {
	return _JobsManager.Contract.DoubleClaimSegmentSlashAmount(&_JobsManager.CallOpts)
}

// DoubleClaimSegmentSlashAmount is a free data retrieval call binding the contract method 0x6d7221d5.
//
// Solidity: function doubleClaimSegmentSlashAmount() constant returns(uint256)
func (_JobsManager *JobsManagerCallerSession) DoubleClaimSegmentSlashAmount() (*big.Int, error) {
	return _JobsManager.Contract.DoubleClaimSegmentSlashAmount(&_JobsManager.CallOpts)
}

// FailedVerificationSlashAmount is a free data retrieval call binding the contract method 0xbe5c2423.
//
// Solidity: function failedVerificationSlashAmount() constant returns(uint256)
func (_JobsManager *JobsManagerCaller) FailedVerificationSlashAmount(opts *bind.CallOpts) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _JobsManager.contract.Call(opts, out, "failedVerificationSlashAmount")
	return *ret0, err
}

// FailedVerificationSlashAmount is a free data retrieval call binding the contract method 0xbe5c2423.
//
// Solidity: function failedVerificationSlashAmount() constant returns(uint256)
func (_JobsManager *JobsManagerSession) FailedVerificationSlashAmount() (*big.Int, error) {
	return _JobsManager.Contract.FailedVerificationSlashAmount(&_JobsManager.CallOpts)
}

// FailedVerificationSlashAmount is a free data retrieval call binding the contract method 0xbe5c2423.
//
// Solidity: function failedVerificationSlashAmount() constant returns(uint256)
func (_JobsManager *JobsManagerCallerSession) FailedVerificationSlashAmount() (*big.Int, error) {
	return _JobsManager.Contract.FailedVerificationSlashAmount(&_JobsManager.CallOpts)
}

// FinderFee is a free data retrieval call binding the contract method 0x1e6b0e44.
//
// Solidity: function finderFee() constant returns(uint256)
func (_JobsManager *JobsManagerCaller) FinderFee(opts *bind.CallOpts) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _JobsManager.contract.Call(opts, out, "finderFee")
	return *ret0, err
}

// FinderFee is a free data retrieval call binding the contract method 0x1e6b0e44.
//
// Solidity: function finderFee() constant returns(uint256)
func (_JobsManager *JobsManagerSession) FinderFee() (*big.Int, error) {
	return _JobsManager.Contract.FinderFee(&_JobsManager.CallOpts)
}

// FinderFee is a free data retrieval call binding the contract method 0x1e6b0e44.
//
// Solidity: function finderFee() constant returns(uint256)
func (_JobsManager *JobsManagerCallerSession) FinderFee() (*big.Int, error) {
	return _JobsManager.Contract.FinderFee(&_JobsManager.CallOpts)
}

// GetClaim is a free data retrieval call binding the contract method 0x427a2fc2.
//
// Solidity: function getClaim(_jobId uint256, _claimId uint256) constant returns(segmentRange uint256[2], claimRoot bytes32, claimBlock uint256, endVerificationBlock uint256, endVerificationSlashingBlock uint256, status uint8)
func (_JobsManager *JobsManagerCaller) GetClaim(opts *bind.CallOpts, _jobId *big.Int, _claimId *big.Int) (struct {
	SegmentRange                 [2]*big.Int
	ClaimRoot                    [32]byte
	ClaimBlock                   *big.Int
	EndVerificationBlock         *big.Int
	EndVerificationSlashingBlock *big.Int
	Status                       uint8
}, error) {
	ret := new(struct {
		SegmentRange                 [2]*big.Int
		ClaimRoot                    [32]byte
		ClaimBlock                   *big.Int
		EndVerificationBlock         *big.Int
		EndVerificationSlashingBlock *big.Int
		Status                       uint8
	})
	out := ret
	err := _JobsManager.contract.Call(opts, out, "getClaim", _jobId, _claimId)
	return *ret, err
}

// GetClaim is a free data retrieval call binding the contract method 0x427a2fc2.
//
// Solidity: function getClaim(_jobId uint256, _claimId uint256) constant returns(segmentRange uint256[2], claimRoot bytes32, claimBlock uint256, endVerificationBlock uint256, endVerificationSlashingBlock uint256, status uint8)
func (_JobsManager *JobsManagerSession) GetClaim(_jobId *big.Int, _claimId *big.Int) (struct {
	SegmentRange                 [2]*big.Int
	ClaimRoot                    [32]byte
	ClaimBlock                   *big.Int
	EndVerificationBlock         *big.Int
	EndVerificationSlashingBlock *big.Int
	Status                       uint8
}, error) {
	return _JobsManager.Contract.GetClaim(&_JobsManager.CallOpts, _jobId, _claimId)
}

// GetClaim is a free data retrieval call binding the contract method 0x427a2fc2.
//
// Solidity: function getClaim(_jobId uint256, _claimId uint256) constant returns(segmentRange uint256[2], claimRoot bytes32, claimBlock uint256, endVerificationBlock uint256, endVerificationSlashingBlock uint256, status uint8)
func (_JobsManager *JobsManagerCallerSession) GetClaim(_jobId *big.Int, _claimId *big.Int) (struct {
	SegmentRange                 [2]*big.Int
	ClaimRoot                    [32]byte
	ClaimBlock                   *big.Int
	EndVerificationBlock         *big.Int
	EndVerificationSlashingBlock *big.Int
	Status                       uint8
}, error) {
	return _JobsManager.Contract.GetClaim(&_JobsManager.CallOpts, _jobId, _claimId)
}

// GetJob is a free data retrieval call binding the contract method 0xbf22c457.
//
// Solidity: function getJob(_jobId uint256) constant returns(streamId string, transcodingOptions string, maxPricePerSegment uint256, broadcasterAddress address, transcoderAddress address, creationRound uint256, creationBlock uint256, endBlock uint256, escrow uint256, totalClaims uint256)
func (_JobsManager *JobsManagerCaller) GetJob(opts *bind.CallOpts, _jobId *big.Int) (struct {
	StreamId           string
	TranscodingOptions string
	MaxPricePerSegment *big.Int
	BroadcasterAddress common.Address
	TranscoderAddress  common.Address
	CreationRound      *big.Int
	CreationBlock      *big.Int
	EndBlock           *big.Int
	Escrow             *big.Int
	TotalClaims        *big.Int
}, error) {
	ret := new(struct {
		StreamId           string
		TranscodingOptions string
		MaxPricePerSegment *big.Int
		BroadcasterAddress common.Address
		TranscoderAddress  common.Address
		CreationRound      *big.Int
		CreationBlock      *big.Int
		EndBlock           *big.Int
		Escrow             *big.Int
		TotalClaims        *big.Int
	})
	out := ret
	err := _JobsManager.contract.Call(opts, out, "getJob", _jobId)
	return *ret, err
}

// GetJob is a free data retrieval call binding the contract method 0xbf22c457.
//
// Solidity: function getJob(_jobId uint256) constant returns(streamId string, transcodingOptions string, maxPricePerSegment uint256, broadcasterAddress address, transcoderAddress address, creationRound uint256, creationBlock uint256, endBlock uint256, escrow uint256, totalClaims uint256)
func (_JobsManager *JobsManagerSession) GetJob(_jobId *big.Int) (struct {
	StreamId           string
	TranscodingOptions string
	MaxPricePerSegment *big.Int
	BroadcasterAddress common.Address
	TranscoderAddress  common.Address
	CreationRound      *big.Int
	CreationBlock      *big.Int
	EndBlock           *big.Int
	Escrow             *big.Int
	TotalClaims        *big.Int
}, error) {
	return _JobsManager.Contract.GetJob(&_JobsManager.CallOpts, _jobId)
}

// GetJob is a free data retrieval call binding the contract method 0xbf22c457.
//
// Solidity: function getJob(_jobId uint256) constant returns(streamId string, transcodingOptions string, maxPricePerSegment uint256, broadcasterAddress address, transcoderAddress address, creationRound uint256, creationBlock uint256, endBlock uint256, escrow uint256, totalClaims uint256)
func (_JobsManager *JobsManagerCallerSession) GetJob(_jobId *big.Int) (struct {
	StreamId           string
	TranscodingOptions string
	MaxPricePerSegment *big.Int
	BroadcasterAddress common.Address
	TranscoderAddress  common.Address
	CreationRound      *big.Int
	CreationBlock      *big.Int
	EndBlock           *big.Int
	Escrow             *big.Int
	TotalClaims        *big.Int
}, error) {
	return _JobsManager.Contract.GetJob(&_JobsManager.CallOpts, _jobId)
}

// IsClaimSegmentVerified is a free data retrieval call binding the contract method 0x71d6dbe1.
//
// Solidity: function isClaimSegmentVerified(_jobId uint256, _claimId uint256, _segmentNumber uint256) constant returns(bool)
func (_JobsManager *JobsManagerCaller) IsClaimSegmentVerified(opts *bind.CallOpts, _jobId *big.Int, _claimId *big.Int, _segmentNumber *big.Int) (bool, error) {
	var (
		ret0 = new(bool)
	)
	out := ret0
	err := _JobsManager.contract.Call(opts, out, "isClaimSegmentVerified", _jobId, _claimId, _segmentNumber)
	return *ret0, err
}

// IsClaimSegmentVerified is a free data retrieval call binding the contract method 0x71d6dbe1.
//
// Solidity: function isClaimSegmentVerified(_jobId uint256, _claimId uint256, _segmentNumber uint256) constant returns(bool)
func (_JobsManager *JobsManagerSession) IsClaimSegmentVerified(_jobId *big.Int, _claimId *big.Int, _segmentNumber *big.Int) (bool, error) {
	return _JobsManager.Contract.IsClaimSegmentVerified(&_JobsManager.CallOpts, _jobId, _claimId, _segmentNumber)
}

// IsClaimSegmentVerified is a free data retrieval call binding the contract method 0x71d6dbe1.
//
// Solidity: function isClaimSegmentVerified(_jobId uint256, _claimId uint256, _segmentNumber uint256) constant returns(bool)
func (_JobsManager *JobsManagerCallerSession) IsClaimSegmentVerified(_jobId *big.Int, _claimId *big.Int, _segmentNumber *big.Int) (bool, error) {
	return _JobsManager.Contract.IsClaimSegmentVerified(&_JobsManager.CallOpts, _jobId, _claimId, _segmentNumber)
}

// JobStatus is a free data retrieval call binding the contract method 0xa8e5e219.
//
// Solidity: function jobStatus(_jobId uint256) constant returns(uint8)
func (_JobsManager *JobsManagerCaller) JobStatus(opts *bind.CallOpts, _jobId *big.Int) (uint8, error) {
	var (
		ret0 = new(uint8)
	)
	out := ret0
	err := _JobsManager.contract.Call(opts, out, "jobStatus", _jobId)
	return *ret0, err
}

// JobStatus is a free data retrieval call binding the contract method 0xa8e5e219.
//
// Solidity: function jobStatus(_jobId uint256) constant returns(uint8)
func (_JobsManager *JobsManagerSession) JobStatus(_jobId *big.Int) (uint8, error) {
	return _JobsManager.Contract.JobStatus(&_JobsManager.CallOpts, _jobId)
}

// JobStatus is a free data retrieval call binding the contract method 0xa8e5e219.
//
// Solidity: function jobStatus(_jobId uint256) constant returns(uint8)
func (_JobsManager *JobsManagerCallerSession) JobStatus(_jobId *big.Int) (uint8, error) {
	return _JobsManager.Contract.JobStatus(&_JobsManager.CallOpts, _jobId)
}

// Jobs is a free data retrieval call binding the contract method 0x180aedf3.
//
// Solidity: function jobs( uint256) constant returns(jobId uint256, streamId string, transcodingOptions string, maxPricePerSegment uint256, broadcasterAddress address, transcoderAddress address, creationRound uint256, creationBlock uint256, endBlock uint256, escrow uint256)
func (_JobsManager *JobsManagerCaller) Jobs(opts *bind.CallOpts, arg0 *big.Int) (struct {
	JobId              *big.Int
	StreamId           string
	TranscodingOptions string
	MaxPricePerSegment *big.Int
	BroadcasterAddress common.Address
	TranscoderAddress  common.Address
	CreationRound      *big.Int
	CreationBlock      *big.Int
	EndBlock           *big.Int
	Escrow             *big.Int
}, error) {
	ret := new(struct {
		JobId              *big.Int
		StreamId           string
		TranscodingOptions string
		MaxPricePerSegment *big.Int
		BroadcasterAddress common.Address
		TranscoderAddress  common.Address
		CreationRound      *big.Int
		CreationBlock      *big.Int
		EndBlock           *big.Int
		Escrow             *big.Int
	})
	out := ret
	err := _JobsManager.contract.Call(opts, out, "jobs", arg0)
	return *ret, err
}

// Jobs is a free data retrieval call binding the contract method 0x180aedf3.
//
// Solidity: function jobs( uint256) constant returns(jobId uint256, streamId string, transcodingOptions string, maxPricePerSegment uint256, broadcasterAddress address, transcoderAddress address, creationRound uint256, creationBlock uint256, endBlock uint256, escrow uint256)
func (_JobsManager *JobsManagerSession) Jobs(arg0 *big.Int) (struct {
	JobId              *big.Int
	StreamId           string
	TranscodingOptions string
	MaxPricePerSegment *big.Int
	BroadcasterAddress common.Address
	TranscoderAddress  common.Address
	CreationRound      *big.Int
	CreationBlock      *big.Int
	EndBlock           *big.Int
	Escrow             *big.Int
}, error) {
	return _JobsManager.Contract.Jobs(&_JobsManager.CallOpts, arg0)
}

// Jobs is a free data retrieval call binding the contract method 0x180aedf3.
//
// Solidity: function jobs( uint256) constant returns(jobId uint256, streamId string, transcodingOptions string, maxPricePerSegment uint256, broadcasterAddress address, transcoderAddress address, creationRound uint256, creationBlock uint256, endBlock uint256, escrow uint256)
func (_JobsManager *JobsManagerCallerSession) Jobs(arg0 *big.Int) (struct {
	JobId              *big.Int
	StreamId           string
	TranscodingOptions string
	MaxPricePerSegment *big.Int
	BroadcasterAddress common.Address
	TranscoderAddress  common.Address
	CreationRound      *big.Int
	CreationBlock      *big.Int
	EndBlock           *big.Int
	Escrow             *big.Int
}, error) {
	return _JobsManager.Contract.Jobs(&_JobsManager.CallOpts, arg0)
}

// MissedVerificationSlashAmount is a free data retrieval call binding the contract method 0x32b5b2d1.
//
// Solidity: function missedVerificationSlashAmount() constant returns(uint256)
func (_JobsManager *JobsManagerCaller) MissedVerificationSlashAmount(opts *bind.CallOpts) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _JobsManager.contract.Call(opts, out, "missedVerificationSlashAmount")
	return *ret0, err
}

// MissedVerificationSlashAmount is a free data retrieval call binding the contract method 0x32b5b2d1.
//
// Solidity: function missedVerificationSlashAmount() constant returns(uint256)
func (_JobsManager *JobsManagerSession) MissedVerificationSlashAmount() (*big.Int, error) {
	return _JobsManager.Contract.MissedVerificationSlashAmount(&_JobsManager.CallOpts)
}

// MissedVerificationSlashAmount is a free data retrieval call binding the contract method 0x32b5b2d1.
//
// Solidity: function missedVerificationSlashAmount() constant returns(uint256)
func (_JobsManager *JobsManagerCallerSession) MissedVerificationSlashAmount() (*big.Int, error) {
	return _JobsManager.Contract.MissedVerificationSlashAmount(&_JobsManager.CallOpts)
}

// NumJobs is a free data retrieval call binding the contract method 0x9212051c.
//
// Solidity: function numJobs() constant returns(uint256)
func (_JobsManager *JobsManagerCaller) NumJobs(opts *bind.CallOpts) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _JobsManager.contract.Call(opts, out, "numJobs")
	return *ret0, err
}

// NumJobs is a free data retrieval call binding the contract method 0x9212051c.
//
// Solidity: function numJobs() constant returns(uint256)
func (_JobsManager *JobsManagerSession) NumJobs() (*big.Int, error) {
	return _JobsManager.Contract.NumJobs(&_JobsManager.CallOpts)
}

// NumJobs is a free data retrieval call binding the contract method 0x9212051c.
//
// Solidity: function numJobs() constant returns(uint256)
func (_JobsManager *JobsManagerCallerSession) NumJobs() (*big.Int, error) {
	return _JobsManager.Contract.NumJobs(&_JobsManager.CallOpts)
}

// TargetContractId is a free data retrieval call binding the contract method 0x51720b41.
//
// Solidity: function targetContractId() constant returns(bytes32)
func (_JobsManager *JobsManagerCaller) TargetContractId(opts *bind.CallOpts) ([32]byte, error) {
	var (
		ret0 = new([32]byte)
	)
	out := ret0
	err := _JobsManager.contract.Call(opts, out, "targetContractId")
	return *ret0, err
}

// TargetContractId is a free data retrieval call binding the contract method 0x51720b41.
//
// Solidity: function targetContractId() constant returns(bytes32)
func (_JobsManager *JobsManagerSession) TargetContractId() ([32]byte, error) {
	return _JobsManager.Contract.TargetContractId(&_JobsManager.CallOpts)
}

// TargetContractId is a free data retrieval call binding the contract method 0x51720b41.
//
// Solidity: function targetContractId() constant returns(bytes32)
func (_JobsManager *JobsManagerCallerSession) TargetContractId() ([32]byte, error) {
	return _JobsManager.Contract.TargetContractId(&_JobsManager.CallOpts)
}

// VerificationPeriod is a free data retrieval call binding the contract method 0xb1bb7e0f.
//
// Solidity: function verificationPeriod() constant returns(uint256)
func (_JobsManager *JobsManagerCaller) VerificationPeriod(opts *bind.CallOpts) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _JobsManager.contract.Call(opts, out, "verificationPeriod")
	return *ret0, err
}

// VerificationPeriod is a free data retrieval call binding the contract method 0xb1bb7e0f.
//
// Solidity: function verificationPeriod() constant returns(uint256)
func (_JobsManager *JobsManagerSession) VerificationPeriod() (*big.Int, error) {
	return _JobsManager.Contract.VerificationPeriod(&_JobsManager.CallOpts)
}

// VerificationPeriod is a free data retrieval call binding the contract method 0xb1bb7e0f.
//
// Solidity: function verificationPeriod() constant returns(uint256)
func (_JobsManager *JobsManagerCallerSession) VerificationPeriod() (*big.Int, error) {
	return _JobsManager.Contract.VerificationPeriod(&_JobsManager.CallOpts)
}

// VerificationRate is a free data retrieval call binding the contract method 0x7af8b87d.
//
// Solidity: function verificationRate() constant returns(uint64)
func (_JobsManager *JobsManagerCaller) VerificationRate(opts *bind.CallOpts) (uint64, error) {
	var (
		ret0 = new(uint64)
	)
	out := ret0
	err := _JobsManager.contract.Call(opts, out, "verificationRate")
	return *ret0, err
}

// VerificationRate is a free data retrieval call binding the contract method 0x7af8b87d.
//
// Solidity: function verificationRate() constant returns(uint64)
func (_JobsManager *JobsManagerSession) VerificationRate() (uint64, error) {
	return _JobsManager.Contract.VerificationRate(&_JobsManager.CallOpts)
}

// VerificationRate is a free data retrieval call binding the contract method 0x7af8b87d.
//
// Solidity: function verificationRate() constant returns(uint64)
func (_JobsManager *JobsManagerCallerSession) VerificationRate() (uint64, error) {
	return _JobsManager.Contract.VerificationRate(&_JobsManager.CallOpts)
}

// VerificationSlashingPeriod is a free data retrieval call binding the contract method 0x9f37b53f.
//
// Solidity: function verificationSlashingPeriod() constant returns(uint256)
func (_JobsManager *JobsManagerCaller) VerificationSlashingPeriod(opts *bind.CallOpts) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _JobsManager.contract.Call(opts, out, "verificationSlashingPeriod")
	return *ret0, err
}

// VerificationSlashingPeriod is a free data retrieval call binding the contract method 0x9f37b53f.
//
// Solidity: function verificationSlashingPeriod() constant returns(uint256)
func (_JobsManager *JobsManagerSession) VerificationSlashingPeriod() (*big.Int, error) {
	return _JobsManager.Contract.VerificationSlashingPeriod(&_JobsManager.CallOpts)
}

// VerificationSlashingPeriod is a free data retrieval call binding the contract method 0x9f37b53f.
//
// Solidity: function verificationSlashingPeriod() constant returns(uint256)
func (_JobsManager *JobsManagerCallerSession) VerificationSlashingPeriod() (*big.Int, error) {
	return _JobsManager.Contract.VerificationSlashingPeriod(&_JobsManager.CallOpts)
}

// BatchDistributeFees is a paid mutator transaction binding the contract method 0x8978fc79.
//
// Solidity: function batchDistributeFees(_jobId uint256, _claimIds uint256[]) returns()
func (_JobsManager *JobsManagerTransactor) BatchDistributeFees(opts *bind.TransactOpts, _jobId *big.Int, _claimIds []*big.Int) (*types.Transaction, error) {
	return _JobsManager.contract.Transact(opts, "batchDistributeFees", _jobId, _claimIds)
}

// BatchDistributeFees is a paid mutator transaction binding the contract method 0x8978fc79.
//
// Solidity: function batchDistributeFees(_jobId uint256, _claimIds uint256[]) returns()
func (_JobsManager *JobsManagerSession) BatchDistributeFees(_jobId *big.Int, _claimIds []*big.Int) (*types.Transaction, error) {
	return _JobsManager.Contract.BatchDistributeFees(&_JobsManager.TransactOpts, _jobId, _claimIds)
}

// BatchDistributeFees is a paid mutator transaction binding the contract method 0x8978fc79.
//
// Solidity: function batchDistributeFees(_jobId uint256, _claimIds uint256[]) returns()
func (_JobsManager *JobsManagerTransactorSession) BatchDistributeFees(_jobId *big.Int, _claimIds []*big.Int) (*types.Transaction, error) {
	return _JobsManager.Contract.BatchDistributeFees(&_JobsManager.TransactOpts, _jobId, _claimIds)
}

// ClaimWork is a paid mutator transaction binding the contract method 0x3ffe5eb7.
//
// Solidity: function claimWork(_jobId uint256, _segmentRange uint256[2], _claimRoot bytes32) returns()
func (_JobsManager *JobsManagerTransactor) ClaimWork(opts *bind.TransactOpts, _jobId *big.Int, _segmentRange [2]*big.Int, _claimRoot [32]byte) (*types.Transaction, error) {
	return _JobsManager.contract.Transact(opts, "claimWork", _jobId, _segmentRange, _claimRoot)
}

// ClaimWork is a paid mutator transaction binding the contract method 0x3ffe5eb7.
//
// Solidity: function claimWork(_jobId uint256, _segmentRange uint256[2], _claimRoot bytes32) returns()
func (_JobsManager *JobsManagerSession) ClaimWork(_jobId *big.Int, _segmentRange [2]*big.Int, _claimRoot [32]byte) (*types.Transaction, error) {
	return _JobsManager.Contract.ClaimWork(&_JobsManager.TransactOpts, _jobId, _segmentRange, _claimRoot)
}

// ClaimWork is a paid mutator transaction binding the contract method 0x3ffe5eb7.
//
// Solidity: function claimWork(_jobId uint256, _segmentRange uint256[2], _claimRoot bytes32) returns()
func (_JobsManager *JobsManagerTransactorSession) ClaimWork(_jobId *big.Int, _segmentRange [2]*big.Int, _claimRoot [32]byte) (*types.Transaction, error) {
	return _JobsManager.Contract.ClaimWork(&_JobsManager.TransactOpts, _jobId, _segmentRange, _claimRoot)
}

// Deposit is a paid mutator transaction binding the contract method 0xd0e30db0.
//
// Solidity: function deposit() returns()
func (_JobsManager *JobsManagerTransactor) Deposit(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _JobsManager.contract.Transact(opts, "deposit")
}

// Deposit is a paid mutator transaction binding the contract method 0xd0e30db0.
//
// Solidity: function deposit() returns()
func (_JobsManager *JobsManagerSession) Deposit() (*types.Transaction, error) {
	return _JobsManager.Contract.Deposit(&_JobsManager.TransactOpts)
}

// Deposit is a paid mutator transaction binding the contract method 0xd0e30db0.
//
// Solidity: function deposit() returns()
func (_JobsManager *JobsManagerTransactorSession) Deposit() (*types.Transaction, error) {
	return _JobsManager.Contract.Deposit(&_JobsManager.TransactOpts)
}

// DistributeFees is a paid mutator transaction binding the contract method 0x7e69671a.
//
// Solidity: function distributeFees(_jobId uint256, _claimId uint256) returns()
func (_JobsManager *JobsManagerTransactor) DistributeFees(opts *bind.TransactOpts, _jobId *big.Int, _claimId *big.Int) (*types.Transaction, error) {
	return _JobsManager.contract.Transact(opts, "distributeFees", _jobId, _claimId)
}

// DistributeFees is a paid mutator transaction binding the contract method 0x7e69671a.
//
// Solidity: function distributeFees(_jobId uint256, _claimId uint256) returns()
func (_JobsManager *JobsManagerSession) DistributeFees(_jobId *big.Int, _claimId *big.Int) (*types.Transaction, error) {
	return _JobsManager.Contract.DistributeFees(&_JobsManager.TransactOpts, _jobId, _claimId)
}

// DistributeFees is a paid mutator transaction binding the contract method 0x7e69671a.
//
// Solidity: function distributeFees(_jobId uint256, _claimId uint256) returns()
func (_JobsManager *JobsManagerTransactorSession) DistributeFees(_jobId *big.Int, _claimId *big.Int) (*types.Transaction, error) {
	return _JobsManager.Contract.DistributeFees(&_JobsManager.TransactOpts, _jobId, _claimId)
}

// DoubleClaimSegmentSlash is a paid mutator transaction binding the contract method 0x64d563f1.
//
// Solidity: function doubleClaimSegmentSlash(_jobId uint256, _claimId1 uint256, _claimId2 uint256, _segmentNumber uint256) returns()
func (_JobsManager *JobsManagerTransactor) DoubleClaimSegmentSlash(opts *bind.TransactOpts, _jobId *big.Int, _claimId1 *big.Int, _claimId2 *big.Int, _segmentNumber *big.Int) (*types.Transaction, error) {
	return _JobsManager.contract.Transact(opts, "doubleClaimSegmentSlash", _jobId, _claimId1, _claimId2, _segmentNumber)
}

// DoubleClaimSegmentSlash is a paid mutator transaction binding the contract method 0x64d563f1.
//
// Solidity: function doubleClaimSegmentSlash(_jobId uint256, _claimId1 uint256, _claimId2 uint256, _segmentNumber uint256) returns()
func (_JobsManager *JobsManagerSession) DoubleClaimSegmentSlash(_jobId *big.Int, _claimId1 *big.Int, _claimId2 *big.Int, _segmentNumber *big.Int) (*types.Transaction, error) {
	return _JobsManager.Contract.DoubleClaimSegmentSlash(&_JobsManager.TransactOpts, _jobId, _claimId1, _claimId2, _segmentNumber)
}

// DoubleClaimSegmentSlash is a paid mutator transaction binding the contract method 0x64d563f1.
//
// Solidity: function doubleClaimSegmentSlash(_jobId uint256, _claimId1 uint256, _claimId2 uint256, _segmentNumber uint256) returns()
func (_JobsManager *JobsManagerTransactorSession) DoubleClaimSegmentSlash(_jobId *big.Int, _claimId1 *big.Int, _claimId2 *big.Int, _segmentNumber *big.Int) (*types.Transaction, error) {
	return _JobsManager.Contract.DoubleClaimSegmentSlash(&_JobsManager.TransactOpts, _jobId, _claimId1, _claimId2, _segmentNumber)
}

// Job is a paid mutator transaction binding the contract method 0x307c6f8e.
//
// Solidity: function job(_streamId string, _transcodingOptions string, _maxPricePerSegment uint256, _endBlock uint256) returns()
func (_JobsManager *JobsManagerTransactor) Job(opts *bind.TransactOpts, _streamId string, _transcodingOptions string, _maxPricePerSegment *big.Int, _endBlock *big.Int) (*types.Transaction, error) {
	return _JobsManager.contract.Transact(opts, "job", _streamId, _transcodingOptions, _maxPricePerSegment, _endBlock)
}

// Job is a paid mutator transaction binding the contract method 0x307c6f8e.
//
// Solidity: function job(_streamId string, _transcodingOptions string, _maxPricePerSegment uint256, _endBlock uint256) returns()
func (_JobsManager *JobsManagerSession) Job(_streamId string, _transcodingOptions string, _maxPricePerSegment *big.Int, _endBlock *big.Int) (*types.Transaction, error) {
	return _JobsManager.Contract.Job(&_JobsManager.TransactOpts, _streamId, _transcodingOptions, _maxPricePerSegment, _endBlock)
}

// Job is a paid mutator transaction binding the contract method 0x307c6f8e.
//
// Solidity: function job(_streamId string, _transcodingOptions string, _maxPricePerSegment uint256, _endBlock uint256) returns()
func (_JobsManager *JobsManagerTransactorSession) Job(_streamId string, _transcodingOptions string, _maxPricePerSegment *big.Int, _endBlock *big.Int) (*types.Transaction, error) {
	return _JobsManager.Contract.Job(&_JobsManager.TransactOpts, _streamId, _transcodingOptions, _maxPricePerSegment, _endBlock)
}

// MissedVerificationSlash is a paid mutator transaction binding the contract method 0xc8e8f487.
//
// Solidity: function missedVerificationSlash(_jobId uint256, _claimId uint256, _segmentNumber uint256) returns()
func (_JobsManager *JobsManagerTransactor) MissedVerificationSlash(opts *bind.TransactOpts, _jobId *big.Int, _claimId *big.Int, _segmentNumber *big.Int) (*types.Transaction, error) {
	return _JobsManager.contract.Transact(opts, "missedVerificationSlash", _jobId, _claimId, _segmentNumber)
}

// MissedVerificationSlash is a paid mutator transaction binding the contract method 0xc8e8f487.
//
// Solidity: function missedVerificationSlash(_jobId uint256, _claimId uint256, _segmentNumber uint256) returns()
func (_JobsManager *JobsManagerSession) MissedVerificationSlash(_jobId *big.Int, _claimId *big.Int, _segmentNumber *big.Int) (*types.Transaction, error) {
	return _JobsManager.Contract.MissedVerificationSlash(&_JobsManager.TransactOpts, _jobId, _claimId, _segmentNumber)
}

// MissedVerificationSlash is a paid mutator transaction binding the contract method 0xc8e8f487.
//
// Solidity: function missedVerificationSlash(_jobId uint256, _claimId uint256, _segmentNumber uint256) returns()
func (_JobsManager *JobsManagerTransactorSession) MissedVerificationSlash(_jobId *big.Int, _claimId *big.Int, _segmentNumber *big.Int) (*types.Transaction, error) {
	return _JobsManager.Contract.MissedVerificationSlash(&_JobsManager.TransactOpts, _jobId, _claimId, _segmentNumber)
}

// ReceiveVerification is a paid mutator transaction binding the contract method 0x1e0976f3.
//
// Solidity: function receiveVerification(_jobId uint256, _claimId uint256, _segmentNumber uint256, _result bool) returns()
func (_JobsManager *JobsManagerTransactor) ReceiveVerification(opts *bind.TransactOpts, _jobId *big.Int, _claimId *big.Int, _segmentNumber *big.Int, _result bool) (*types.Transaction, error) {
	return _JobsManager.contract.Transact(opts, "receiveVerification", _jobId, _claimId, _segmentNumber, _result)
}

// ReceiveVerification is a paid mutator transaction binding the contract method 0x1e0976f3.
//
// Solidity: function receiveVerification(_jobId uint256, _claimId uint256, _segmentNumber uint256, _result bool) returns()
func (_JobsManager *JobsManagerSession) ReceiveVerification(_jobId *big.Int, _claimId *big.Int, _segmentNumber *big.Int, _result bool) (*types.Transaction, error) {
	return _JobsManager.Contract.ReceiveVerification(&_JobsManager.TransactOpts, _jobId, _claimId, _segmentNumber, _result)
}

// ReceiveVerification is a paid mutator transaction binding the contract method 0x1e0976f3.
//
// Solidity: function receiveVerification(_jobId uint256, _claimId uint256, _segmentNumber uint256, _result bool) returns()
func (_JobsManager *JobsManagerTransactorSession) ReceiveVerification(_jobId *big.Int, _claimId *big.Int, _segmentNumber *big.Int, _result bool) (*types.Transaction, error) {
	return _JobsManager.Contract.ReceiveVerification(&_JobsManager.TransactOpts, _jobId, _claimId, _segmentNumber, _result)
}

// SetController is a paid mutator transaction binding the contract method 0x92eefe9b.
//
// Solidity: function setController(_controller address) returns()
func (_JobsManager *JobsManagerTransactor) SetController(opts *bind.TransactOpts, _controller common.Address) (*types.Transaction, error) {
	return _JobsManager.contract.Transact(opts, "setController", _controller)
}

// SetController is a paid mutator transaction binding the contract method 0x92eefe9b.
//
// Solidity: function setController(_controller address) returns()
func (_JobsManager *JobsManagerSession) SetController(_controller common.Address) (*types.Transaction, error) {
	return _JobsManager.Contract.SetController(&_JobsManager.TransactOpts, _controller)
}

// SetController is a paid mutator transaction binding the contract method 0x92eefe9b.
//
// Solidity: function setController(_controller address) returns()
func (_JobsManager *JobsManagerTransactorSession) SetController(_controller common.Address) (*types.Transaction, error) {
	return _JobsManager.Contract.SetController(&_JobsManager.TransactOpts, _controller)
}

// SetDoubleClaimSegmentSlashAmount is a paid mutator transaction binding the contract method 0x7d6ebe94.
//
// Solidity: function setDoubleClaimSegmentSlashAmount(_doubleClaimSegmentSlashAmount uint256) returns()
func (_JobsManager *JobsManagerTransactor) SetDoubleClaimSegmentSlashAmount(opts *bind.TransactOpts, _doubleClaimSegmentSlashAmount *big.Int) (*types.Transaction, error) {
	return _JobsManager.contract.Transact(opts, "setDoubleClaimSegmentSlashAmount", _doubleClaimSegmentSlashAmount)
}

// SetDoubleClaimSegmentSlashAmount is a paid mutator transaction binding the contract method 0x7d6ebe94.
//
// Solidity: function setDoubleClaimSegmentSlashAmount(_doubleClaimSegmentSlashAmount uint256) returns()
func (_JobsManager *JobsManagerSession) SetDoubleClaimSegmentSlashAmount(_doubleClaimSegmentSlashAmount *big.Int) (*types.Transaction, error) {
	return _JobsManager.Contract.SetDoubleClaimSegmentSlashAmount(&_JobsManager.TransactOpts, _doubleClaimSegmentSlashAmount)
}

// SetDoubleClaimSegmentSlashAmount is a paid mutator transaction binding the contract method 0x7d6ebe94.
//
// Solidity: function setDoubleClaimSegmentSlashAmount(_doubleClaimSegmentSlashAmount uint256) returns()
func (_JobsManager *JobsManagerTransactorSession) SetDoubleClaimSegmentSlashAmount(_doubleClaimSegmentSlashAmount *big.Int) (*types.Transaction, error) {
	return _JobsManager.Contract.SetDoubleClaimSegmentSlashAmount(&_JobsManager.TransactOpts, _doubleClaimSegmentSlashAmount)
}

// SetFailedVerificationSlashAmount is a paid mutator transaction binding the contract method 0x71af5d0e.
//
// Solidity: function setFailedVerificationSlashAmount(_failedVerificationSlashAmount uint256) returns()
func (_JobsManager *JobsManagerTransactor) SetFailedVerificationSlashAmount(opts *bind.TransactOpts, _failedVerificationSlashAmount *big.Int) (*types.Transaction, error) {
	return _JobsManager.contract.Transact(opts, "setFailedVerificationSlashAmount", _failedVerificationSlashAmount)
}

// SetFailedVerificationSlashAmount is a paid mutator transaction binding the contract method 0x71af5d0e.
//
// Solidity: function setFailedVerificationSlashAmount(_failedVerificationSlashAmount uint256) returns()
func (_JobsManager *JobsManagerSession) SetFailedVerificationSlashAmount(_failedVerificationSlashAmount *big.Int) (*types.Transaction, error) {
	return _JobsManager.Contract.SetFailedVerificationSlashAmount(&_JobsManager.TransactOpts, _failedVerificationSlashAmount)
}

// SetFailedVerificationSlashAmount is a paid mutator transaction binding the contract method 0x71af5d0e.
//
// Solidity: function setFailedVerificationSlashAmount(_failedVerificationSlashAmount uint256) returns()
func (_JobsManager *JobsManagerTransactorSession) SetFailedVerificationSlashAmount(_failedVerificationSlashAmount *big.Int) (*types.Transaction, error) {
	return _JobsManager.Contract.SetFailedVerificationSlashAmount(&_JobsManager.TransactOpts, _failedVerificationSlashAmount)
}

// SetFinderFee is a paid mutator transaction binding the contract method 0xbe427b1c.
//
// Solidity: function setFinderFee(_finderFee uint256) returns()
func (_JobsManager *JobsManagerTransactor) SetFinderFee(opts *bind.TransactOpts, _finderFee *big.Int) (*types.Transaction, error) {
	return _JobsManager.contract.Transact(opts, "setFinderFee", _finderFee)
}

// SetFinderFee is a paid mutator transaction binding the contract method 0xbe427b1c.
//
// Solidity: function setFinderFee(_finderFee uint256) returns()
func (_JobsManager *JobsManagerSession) SetFinderFee(_finderFee *big.Int) (*types.Transaction, error) {
	return _JobsManager.Contract.SetFinderFee(&_JobsManager.TransactOpts, _finderFee)
}

// SetFinderFee is a paid mutator transaction binding the contract method 0xbe427b1c.
//
// Solidity: function setFinderFee(_finderFee uint256) returns()
func (_JobsManager *JobsManagerTransactorSession) SetFinderFee(_finderFee *big.Int) (*types.Transaction, error) {
	return _JobsManager.Contract.SetFinderFee(&_JobsManager.TransactOpts, _finderFee)
}

// SetMissedVerificationSlashAmount is a paid mutator transaction binding the contract method 0x0cb335c4.
//
// Solidity: function setMissedVerificationSlashAmount(_missedVerificationSlashAmount uint256) returns()
func (_JobsManager *JobsManagerTransactor) SetMissedVerificationSlashAmount(opts *bind.TransactOpts, _missedVerificationSlashAmount *big.Int) (*types.Transaction, error) {
	return _JobsManager.contract.Transact(opts, "setMissedVerificationSlashAmount", _missedVerificationSlashAmount)
}

// SetMissedVerificationSlashAmount is a paid mutator transaction binding the contract method 0x0cb335c4.
//
// Solidity: function setMissedVerificationSlashAmount(_missedVerificationSlashAmount uint256) returns()
func (_JobsManager *JobsManagerSession) SetMissedVerificationSlashAmount(_missedVerificationSlashAmount *big.Int) (*types.Transaction, error) {
	return _JobsManager.Contract.SetMissedVerificationSlashAmount(&_JobsManager.TransactOpts, _missedVerificationSlashAmount)
}

// SetMissedVerificationSlashAmount is a paid mutator transaction binding the contract method 0x0cb335c4.
//
// Solidity: function setMissedVerificationSlashAmount(_missedVerificationSlashAmount uint256) returns()
func (_JobsManager *JobsManagerTransactorSession) SetMissedVerificationSlashAmount(_missedVerificationSlashAmount *big.Int) (*types.Transaction, error) {
	return _JobsManager.Contract.SetMissedVerificationSlashAmount(&_JobsManager.TransactOpts, _missedVerificationSlashAmount)
}

// SetVerificationPeriod is a paid mutator transaction binding the contract method 0x09bc1812.
//
// Solidity: function setVerificationPeriod(_verificationPeriod uint256) returns()
func (_JobsManager *JobsManagerTransactor) SetVerificationPeriod(opts *bind.TransactOpts, _verificationPeriod *big.Int) (*types.Transaction, error) {
	return _JobsManager.contract.Transact(opts, "setVerificationPeriod", _verificationPeriod)
}

// SetVerificationPeriod is a paid mutator transaction binding the contract method 0x09bc1812.
//
// Solidity: function setVerificationPeriod(_verificationPeriod uint256) returns()
func (_JobsManager *JobsManagerSession) SetVerificationPeriod(_verificationPeriod *big.Int) (*types.Transaction, error) {
	return _JobsManager.Contract.SetVerificationPeriod(&_JobsManager.TransactOpts, _verificationPeriod)
}

// SetVerificationPeriod is a paid mutator transaction binding the contract method 0x09bc1812.
//
// Solidity: function setVerificationPeriod(_verificationPeriod uint256) returns()
func (_JobsManager *JobsManagerTransactorSession) SetVerificationPeriod(_verificationPeriod *big.Int) (*types.Transaction, error) {
	return _JobsManager.Contract.SetVerificationPeriod(&_JobsManager.TransactOpts, _verificationPeriod)
}

// SetVerificationRate is a paid mutator transaction binding the contract method 0x15fa168a.
//
// Solidity: function setVerificationRate(_verificationRate uint64) returns()
func (_JobsManager *JobsManagerTransactor) SetVerificationRate(opts *bind.TransactOpts, _verificationRate uint64) (*types.Transaction, error) {
	return _JobsManager.contract.Transact(opts, "setVerificationRate", _verificationRate)
}

// SetVerificationRate is a paid mutator transaction binding the contract method 0x15fa168a.
//
// Solidity: function setVerificationRate(_verificationRate uint64) returns()
func (_JobsManager *JobsManagerSession) SetVerificationRate(_verificationRate uint64) (*types.Transaction, error) {
	return _JobsManager.Contract.SetVerificationRate(&_JobsManager.TransactOpts, _verificationRate)
}

// SetVerificationRate is a paid mutator transaction binding the contract method 0x15fa168a.
//
// Solidity: function setVerificationRate(_verificationRate uint64) returns()
func (_JobsManager *JobsManagerTransactorSession) SetVerificationRate(_verificationRate uint64) (*types.Transaction, error) {
	return _JobsManager.Contract.SetVerificationRate(&_JobsManager.TransactOpts, _verificationRate)
}

// SetVerificationSlashingPeriod is a paid mutator transaction binding the contract method 0x4e78e0c2.
//
// Solidity: function setVerificationSlashingPeriod(_verificationSlashingPeriod uint256) returns()
func (_JobsManager *JobsManagerTransactor) SetVerificationSlashingPeriod(opts *bind.TransactOpts, _verificationSlashingPeriod *big.Int) (*types.Transaction, error) {
	return _JobsManager.contract.Transact(opts, "setVerificationSlashingPeriod", _verificationSlashingPeriod)
}

// SetVerificationSlashingPeriod is a paid mutator transaction binding the contract method 0x4e78e0c2.
//
// Solidity: function setVerificationSlashingPeriod(_verificationSlashingPeriod uint256) returns()
func (_JobsManager *JobsManagerSession) SetVerificationSlashingPeriod(_verificationSlashingPeriod *big.Int) (*types.Transaction, error) {
	return _JobsManager.Contract.SetVerificationSlashingPeriod(&_JobsManager.TransactOpts, _verificationSlashingPeriod)
}

// SetVerificationSlashingPeriod is a paid mutator transaction binding the contract method 0x4e78e0c2.
//
// Solidity: function setVerificationSlashingPeriod(_verificationSlashingPeriod uint256) returns()
func (_JobsManager *JobsManagerTransactorSession) SetVerificationSlashingPeriod(_verificationSlashingPeriod *big.Int) (*types.Transaction, error) {
	return _JobsManager.Contract.SetVerificationSlashingPeriod(&_JobsManager.TransactOpts, _verificationSlashingPeriod)
}

// Verify is a paid mutator transaction binding the contract method 0x5a40ec7e.
//
// Solidity: function verify(_jobId uint256, _claimId uint256, _segmentNumber uint256, _dataStorageHash string, _dataHashes bytes32[2], _broadcasterSig bytes, _proof bytes) returns()
func (_JobsManager *JobsManagerTransactor) Verify(opts *bind.TransactOpts, _jobId *big.Int, _claimId *big.Int, _segmentNumber *big.Int, _dataStorageHash string, _dataHashes [2][32]byte, _broadcasterSig []byte, _proof []byte) (*types.Transaction, error) {
	return _JobsManager.contract.Transact(opts, "verify", _jobId, _claimId, _segmentNumber, _dataStorageHash, _dataHashes, _broadcasterSig, _proof)
}

// Verify is a paid mutator transaction binding the contract method 0x5a40ec7e.
//
// Solidity: function verify(_jobId uint256, _claimId uint256, _segmentNumber uint256, _dataStorageHash string, _dataHashes bytes32[2], _broadcasterSig bytes, _proof bytes) returns()
func (_JobsManager *JobsManagerSession) Verify(_jobId *big.Int, _claimId *big.Int, _segmentNumber *big.Int, _dataStorageHash string, _dataHashes [2][32]byte, _broadcasterSig []byte, _proof []byte) (*types.Transaction, error) {
	return _JobsManager.Contract.Verify(&_JobsManager.TransactOpts, _jobId, _claimId, _segmentNumber, _dataStorageHash, _dataHashes, _broadcasterSig, _proof)
}

// Verify is a paid mutator transaction binding the contract method 0x5a40ec7e.
//
// Solidity: function verify(_jobId uint256, _claimId uint256, _segmentNumber uint256, _dataStorageHash string, _dataHashes bytes32[2], _broadcasterSig bytes, _proof bytes) returns()
func (_JobsManager *JobsManagerTransactorSession) Verify(_jobId *big.Int, _claimId *big.Int, _segmentNumber *big.Int, _dataStorageHash string, _dataHashes [2][32]byte, _broadcasterSig []byte, _proof []byte) (*types.Transaction, error) {
	return _JobsManager.Contract.Verify(&_JobsManager.TransactOpts, _jobId, _claimId, _segmentNumber, _dataStorageHash, _dataHashes, _broadcasterSig, _proof)
}

// Withdraw is a paid mutator transaction binding the contract method 0x3ccfd60b.
//
// Solidity: function withdraw() returns()
func (_JobsManager *JobsManagerTransactor) Withdraw(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _JobsManager.contract.Transact(opts, "withdraw")
}

// Withdraw is a paid mutator transaction binding the contract method 0x3ccfd60b.
//
// Solidity: function withdraw() returns()
func (_JobsManager *JobsManagerSession) Withdraw() (*types.Transaction, error) {
	return _JobsManager.Contract.Withdraw(&_JobsManager.TransactOpts)
}

// Withdraw is a paid mutator transaction binding the contract method 0x3ccfd60b.
//
// Solidity: function withdraw() returns()
func (_JobsManager *JobsManagerTransactorSession) Withdraw() (*types.Transaction, error) {
	return _JobsManager.Contract.Withdraw(&_JobsManager.TransactOpts)
}

// JobsManagerDepositIterator is returned from FilterDeposit and is used to iterate over the raw logs and unpacked data for Deposit events raised by the JobsManager contract.
type JobsManagerDepositIterator struct {
	Event *JobsManagerDeposit // Event containing the contract specifics and raw log

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
func (it *JobsManagerDepositIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(JobsManagerDeposit)
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
		it.Event = new(JobsManagerDeposit)
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
func (it *JobsManagerDepositIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *JobsManagerDepositIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// JobsManagerDeposit represents a Deposit event raised by the JobsManager contract.
type JobsManagerDeposit struct {
	Broadcaster common.Address
	Amount      *big.Int
	Raw         types.Log // Blockchain specific contextual infos
}

// FilterDeposit is a free log retrieval operation binding the contract event 0xe1fffcc4923d04b559f4d29a8bfc6cda04eb5b0d3c460751c2402c5c5cc9109c.
//
// Solidity: event Deposit(broadcaster indexed address, amount uint256)
func (_JobsManager *JobsManagerFilterer) FilterDeposit(opts *bind.FilterOpts, broadcaster []common.Address) (*JobsManagerDepositIterator, error) {

	var broadcasterRule []interface{}
	for _, broadcasterItem := range broadcaster {
		broadcasterRule = append(broadcasterRule, broadcasterItem)
	}

	logs, sub, err := _JobsManager.contract.FilterLogs(opts, "Deposit", broadcasterRule)
	if err != nil {
		return nil, err
	}
	return &JobsManagerDepositIterator{contract: _JobsManager.contract, event: "Deposit", logs: logs, sub: sub}, nil
}

// WatchDeposit is a free log subscription operation binding the contract event 0xe1fffcc4923d04b559f4d29a8bfc6cda04eb5b0d3c460751c2402c5c5cc9109c.
//
// Solidity: event Deposit(broadcaster indexed address, amount uint256)
func (_JobsManager *JobsManagerFilterer) WatchDeposit(opts *bind.WatchOpts, sink chan<- *JobsManagerDeposit, broadcaster []common.Address) (event.Subscription, error) {

	var broadcasterRule []interface{}
	for _, broadcasterItem := range broadcaster {
		broadcasterRule = append(broadcasterRule, broadcasterItem)
	}

	logs, sub, err := _JobsManager.contract.WatchLogs(opts, "Deposit", broadcasterRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(JobsManagerDeposit)
				if err := _JobsManager.contract.UnpackLog(event, "Deposit", log); err != nil {
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

// JobsManagerDistributeFeesIterator is returned from FilterDistributeFees and is used to iterate over the raw logs and unpacked data for DistributeFees events raised by the JobsManager contract.
type JobsManagerDistributeFeesIterator struct {
	Event *JobsManagerDistributeFees // Event containing the contract specifics and raw log

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
func (it *JobsManagerDistributeFeesIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(JobsManagerDistributeFees)
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
		it.Event = new(JobsManagerDistributeFees)
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
func (it *JobsManagerDistributeFeesIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *JobsManagerDistributeFeesIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// JobsManagerDistributeFees represents a DistributeFees event raised by the JobsManager contract.
type JobsManagerDistributeFees struct {
	Transcoder common.Address
	JobId      *big.Int
	ClaimId    *big.Int
	Fees       *big.Int
	Raw        types.Log // Blockchain specific contextual infos
}

// FilterDistributeFees is a free log retrieval operation binding the contract event 0xa9fda9546b61eac5990fddef170f356f0f70c0f75dc7a6821b430218f3d04264.
//
// Solidity: event DistributeFees(transcoder indexed address, jobId indexed uint256, claimId indexed uint256, fees uint256)
func (_JobsManager *JobsManagerFilterer) FilterDistributeFees(opts *bind.FilterOpts, transcoder []common.Address, jobId []*big.Int, claimId []*big.Int) (*JobsManagerDistributeFeesIterator, error) {

	var transcoderRule []interface{}
	for _, transcoderItem := range transcoder {
		transcoderRule = append(transcoderRule, transcoderItem)
	}
	var jobIdRule []interface{}
	for _, jobIdItem := range jobId {
		jobIdRule = append(jobIdRule, jobIdItem)
	}
	var claimIdRule []interface{}
	for _, claimIdItem := range claimId {
		claimIdRule = append(claimIdRule, claimIdItem)
	}

	logs, sub, err := _JobsManager.contract.FilterLogs(opts, "DistributeFees", transcoderRule, jobIdRule, claimIdRule)
	if err != nil {
		return nil, err
	}
	return &JobsManagerDistributeFeesIterator{contract: _JobsManager.contract, event: "DistributeFees", logs: logs, sub: sub}, nil
}

// WatchDistributeFees is a free log subscription operation binding the contract event 0xa9fda9546b61eac5990fddef170f356f0f70c0f75dc7a6821b430218f3d04264.
//
// Solidity: event DistributeFees(transcoder indexed address, jobId indexed uint256, claimId indexed uint256, fees uint256)
func (_JobsManager *JobsManagerFilterer) WatchDistributeFees(opts *bind.WatchOpts, sink chan<- *JobsManagerDistributeFees, transcoder []common.Address, jobId []*big.Int, claimId []*big.Int) (event.Subscription, error) {

	var transcoderRule []interface{}
	for _, transcoderItem := range transcoder {
		transcoderRule = append(transcoderRule, transcoderItem)
	}
	var jobIdRule []interface{}
	for _, jobIdItem := range jobId {
		jobIdRule = append(jobIdRule, jobIdItem)
	}
	var claimIdRule []interface{}
	for _, claimIdItem := range claimId {
		claimIdRule = append(claimIdRule, claimIdItem)
	}

	logs, sub, err := _JobsManager.contract.WatchLogs(opts, "DistributeFees", transcoderRule, jobIdRule, claimIdRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(JobsManagerDistributeFees)
				if err := _JobsManager.contract.UnpackLog(event, "DistributeFees", log); err != nil {
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

// JobsManagerFailedVerificationIterator is returned from FilterFailedVerification and is used to iterate over the raw logs and unpacked data for FailedVerification events raised by the JobsManager contract.
type JobsManagerFailedVerificationIterator struct {
	Event *JobsManagerFailedVerification // Event containing the contract specifics and raw log

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
func (it *JobsManagerFailedVerificationIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(JobsManagerFailedVerification)
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
		it.Event = new(JobsManagerFailedVerification)
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
func (it *JobsManagerFailedVerificationIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *JobsManagerFailedVerificationIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// JobsManagerFailedVerification represents a FailedVerification event raised by the JobsManager contract.
type JobsManagerFailedVerification struct {
	Transcoder    common.Address
	JobId         *big.Int
	ClaimId       *big.Int
	SegmentNumber *big.Int
	Raw           types.Log // Blockchain specific contextual infos
}

// FilterFailedVerification is a free log retrieval operation binding the contract event 0x325eefe220fe85167c5d95dfbfc58fd8c17a709a9dda3df44784d7ba83669816.
//
// Solidity: event FailedVerification(transcoder indexed address, jobId indexed uint256, claimId indexed uint256, segmentNumber uint256)
func (_JobsManager *JobsManagerFilterer) FilterFailedVerification(opts *bind.FilterOpts, transcoder []common.Address, jobId []*big.Int, claimId []*big.Int) (*JobsManagerFailedVerificationIterator, error) {

	var transcoderRule []interface{}
	for _, transcoderItem := range transcoder {
		transcoderRule = append(transcoderRule, transcoderItem)
	}
	var jobIdRule []interface{}
	for _, jobIdItem := range jobId {
		jobIdRule = append(jobIdRule, jobIdItem)
	}
	var claimIdRule []interface{}
	for _, claimIdItem := range claimId {
		claimIdRule = append(claimIdRule, claimIdItem)
	}

	logs, sub, err := _JobsManager.contract.FilterLogs(opts, "FailedVerification", transcoderRule, jobIdRule, claimIdRule)
	if err != nil {
		return nil, err
	}
	return &JobsManagerFailedVerificationIterator{contract: _JobsManager.contract, event: "FailedVerification", logs: logs, sub: sub}, nil
}

// WatchFailedVerification is a free log subscription operation binding the contract event 0x325eefe220fe85167c5d95dfbfc58fd8c17a709a9dda3df44784d7ba83669816.
//
// Solidity: event FailedVerification(transcoder indexed address, jobId indexed uint256, claimId indexed uint256, segmentNumber uint256)
func (_JobsManager *JobsManagerFilterer) WatchFailedVerification(opts *bind.WatchOpts, sink chan<- *JobsManagerFailedVerification, transcoder []common.Address, jobId []*big.Int, claimId []*big.Int) (event.Subscription, error) {

	var transcoderRule []interface{}
	for _, transcoderItem := range transcoder {
		transcoderRule = append(transcoderRule, transcoderItem)
	}
	var jobIdRule []interface{}
	for _, jobIdItem := range jobId {
		jobIdRule = append(jobIdRule, jobIdItem)
	}
	var claimIdRule []interface{}
	for _, claimIdItem := range claimId {
		claimIdRule = append(claimIdRule, claimIdItem)
	}

	logs, sub, err := _JobsManager.contract.WatchLogs(opts, "FailedVerification", transcoderRule, jobIdRule, claimIdRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(JobsManagerFailedVerification)
				if err := _JobsManager.contract.UnpackLog(event, "FailedVerification", log); err != nil {
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

// JobsManagerNewClaimIterator is returned from FilterNewClaim and is used to iterate over the raw logs and unpacked data for NewClaim events raised by the JobsManager contract.
type JobsManagerNewClaimIterator struct {
	Event *JobsManagerNewClaim // Event containing the contract specifics and raw log

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
func (it *JobsManagerNewClaimIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(JobsManagerNewClaim)
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
		it.Event = new(JobsManagerNewClaim)
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
func (it *JobsManagerNewClaimIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *JobsManagerNewClaimIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// JobsManagerNewClaim represents a NewClaim event raised by the JobsManager contract.
type JobsManagerNewClaim struct {
	Transcoder common.Address
	JobId      *big.Int
	ClaimId    *big.Int
	Raw        types.Log // Blockchain specific contextual infos
}

// FilterNewClaim is a free log retrieval operation binding the contract event 0x83bd61fdfc2d435598c85b87527fb51d01971ab4904c8d41410f6d7b2ffb29de.
//
// Solidity: event NewClaim(transcoder indexed address, jobId indexed uint256, claimId uint256)
func (_JobsManager *JobsManagerFilterer) FilterNewClaim(opts *bind.FilterOpts, transcoder []common.Address, jobId []*big.Int) (*JobsManagerNewClaimIterator, error) {

	var transcoderRule []interface{}
	for _, transcoderItem := range transcoder {
		transcoderRule = append(transcoderRule, transcoderItem)
	}
	var jobIdRule []interface{}
	for _, jobIdItem := range jobId {
		jobIdRule = append(jobIdRule, jobIdItem)
	}

	logs, sub, err := _JobsManager.contract.FilterLogs(opts, "NewClaim", transcoderRule, jobIdRule)
	if err != nil {
		return nil, err
	}
	return &JobsManagerNewClaimIterator{contract: _JobsManager.contract, event: "NewClaim", logs: logs, sub: sub}, nil
}

// WatchNewClaim is a free log subscription operation binding the contract event 0x83bd61fdfc2d435598c85b87527fb51d01971ab4904c8d41410f6d7b2ffb29de.
//
// Solidity: event NewClaim(transcoder indexed address, jobId indexed uint256, claimId uint256)
func (_JobsManager *JobsManagerFilterer) WatchNewClaim(opts *bind.WatchOpts, sink chan<- *JobsManagerNewClaim, transcoder []common.Address, jobId []*big.Int) (event.Subscription, error) {

	var transcoderRule []interface{}
	for _, transcoderItem := range transcoder {
		transcoderRule = append(transcoderRule, transcoderItem)
	}
	var jobIdRule []interface{}
	for _, jobIdItem := range jobId {
		jobIdRule = append(jobIdRule, jobIdItem)
	}

	logs, sub, err := _JobsManager.contract.WatchLogs(opts, "NewClaim", transcoderRule, jobIdRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(JobsManagerNewClaim)
				if err := _JobsManager.contract.UnpackLog(event, "NewClaim", log); err != nil {
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

// JobsManagerNewJobIterator is returned from FilterNewJob and is used to iterate over the raw logs and unpacked data for NewJob events raised by the JobsManager contract.
type JobsManagerNewJobIterator struct {
	Event *JobsManagerNewJob // Event containing the contract specifics and raw log

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
func (it *JobsManagerNewJobIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(JobsManagerNewJob)
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
		it.Event = new(JobsManagerNewJob)
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
func (it *JobsManagerNewJobIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *JobsManagerNewJobIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// JobsManagerNewJob represents a NewJob event raised by the JobsManager contract.
type JobsManagerNewJob struct {
	Broadcaster        common.Address
	JobId              *big.Int
	StreamId           string
	TranscodingOptions string
	MaxPricePerSegment *big.Int
	CreationBlock      *big.Int
	Raw                types.Log // Blockchain specific contextual infos
}

// FilterNewJob is a free log retrieval operation binding the contract event 0x167f465188f71efa9880c291714e13242987d056e8148687871fae51457ae6e2.
//
// Solidity: event NewJob(broadcaster indexed address, jobId uint256, streamId string, transcodingOptions string, maxPricePerSegment uint256, creationBlock uint256)
func (_JobsManager *JobsManagerFilterer) FilterNewJob(opts *bind.FilterOpts, broadcaster []common.Address) (*JobsManagerNewJobIterator, error) {

	var broadcasterRule []interface{}
	for _, broadcasterItem := range broadcaster {
		broadcasterRule = append(broadcasterRule, broadcasterItem)
	}

	logs, sub, err := _JobsManager.contract.FilterLogs(opts, "NewJob", broadcasterRule)
	if err != nil {
		return nil, err
	}
	return &JobsManagerNewJobIterator{contract: _JobsManager.contract, event: "NewJob", logs: logs, sub: sub}, nil
}

// WatchNewJob is a free log subscription operation binding the contract event 0x167f465188f71efa9880c291714e13242987d056e8148687871fae51457ae6e2.
//
// Solidity: event NewJob(broadcaster indexed address, jobId uint256, streamId string, transcodingOptions string, maxPricePerSegment uint256, creationBlock uint256)
func (_JobsManager *JobsManagerFilterer) WatchNewJob(opts *bind.WatchOpts, sink chan<- *JobsManagerNewJob, broadcaster []common.Address) (event.Subscription, error) {

	var broadcasterRule []interface{}
	for _, broadcasterItem := range broadcaster {
		broadcasterRule = append(broadcasterRule, broadcasterItem)
	}

	logs, sub, err := _JobsManager.contract.WatchLogs(opts, "NewJob", broadcasterRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(JobsManagerNewJob)
				if err := _JobsManager.contract.UnpackLog(event, "NewJob", log); err != nil {
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

// JobsManagerParameterUpdateIterator is returned from FilterParameterUpdate and is used to iterate over the raw logs and unpacked data for ParameterUpdate events raised by the JobsManager contract.
type JobsManagerParameterUpdateIterator struct {
	Event *JobsManagerParameterUpdate // Event containing the contract specifics and raw log

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
func (it *JobsManagerParameterUpdateIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(JobsManagerParameterUpdate)
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
		it.Event = new(JobsManagerParameterUpdate)
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
func (it *JobsManagerParameterUpdateIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *JobsManagerParameterUpdateIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// JobsManagerParameterUpdate represents a ParameterUpdate event raised by the JobsManager contract.
type JobsManagerParameterUpdate struct {
	Param string
	Raw   types.Log // Blockchain specific contextual infos
}

// FilterParameterUpdate is a free log retrieval operation binding the contract event 0x9f5033568d78ae30f29f01e944f97b2216493bd19d1b46d429673acff3dcd674.
//
// Solidity: event ParameterUpdate(param string)
func (_JobsManager *JobsManagerFilterer) FilterParameterUpdate(opts *bind.FilterOpts) (*JobsManagerParameterUpdateIterator, error) {

	logs, sub, err := _JobsManager.contract.FilterLogs(opts, "ParameterUpdate")
	if err != nil {
		return nil, err
	}
	return &JobsManagerParameterUpdateIterator{contract: _JobsManager.contract, event: "ParameterUpdate", logs: logs, sub: sub}, nil
}

// WatchParameterUpdate is a free log subscription operation binding the contract event 0x9f5033568d78ae30f29f01e944f97b2216493bd19d1b46d429673acff3dcd674.
//
// Solidity: event ParameterUpdate(param string)
func (_JobsManager *JobsManagerFilterer) WatchParameterUpdate(opts *bind.WatchOpts, sink chan<- *JobsManagerParameterUpdate) (event.Subscription, error) {

	logs, sub, err := _JobsManager.contract.WatchLogs(opts, "ParameterUpdate")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(JobsManagerParameterUpdate)
				if err := _JobsManager.contract.UnpackLog(event, "ParameterUpdate", log); err != nil {
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

// JobsManagerPassedVerificationIterator is returned from FilterPassedVerification and is used to iterate over the raw logs and unpacked data for PassedVerification events raised by the JobsManager contract.
type JobsManagerPassedVerificationIterator struct {
	Event *JobsManagerPassedVerification // Event containing the contract specifics and raw log

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
func (it *JobsManagerPassedVerificationIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(JobsManagerPassedVerification)
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
		it.Event = new(JobsManagerPassedVerification)
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
func (it *JobsManagerPassedVerificationIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *JobsManagerPassedVerificationIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// JobsManagerPassedVerification represents a PassedVerification event raised by the JobsManager contract.
type JobsManagerPassedVerification struct {
	Transcoder    common.Address
	JobId         *big.Int
	ClaimId       *big.Int
	SegmentNumber *big.Int
	Raw           types.Log // Blockchain specific contextual infos
}

// FilterPassedVerification is a free log retrieval operation binding the contract event 0x18d2d655e3f8d4b44ce95ed671c3f12339b2863d065ef91e970ac87826f45d8e.
//
// Solidity: event PassedVerification(transcoder indexed address, jobId indexed uint256, claimId indexed uint256, segmentNumber uint256)
func (_JobsManager *JobsManagerFilterer) FilterPassedVerification(opts *bind.FilterOpts, transcoder []common.Address, jobId []*big.Int, claimId []*big.Int) (*JobsManagerPassedVerificationIterator, error) {

	var transcoderRule []interface{}
	for _, transcoderItem := range transcoder {
		transcoderRule = append(transcoderRule, transcoderItem)
	}
	var jobIdRule []interface{}
	for _, jobIdItem := range jobId {
		jobIdRule = append(jobIdRule, jobIdItem)
	}
	var claimIdRule []interface{}
	for _, claimIdItem := range claimId {
		claimIdRule = append(claimIdRule, claimIdItem)
	}

	logs, sub, err := _JobsManager.contract.FilterLogs(opts, "PassedVerification", transcoderRule, jobIdRule, claimIdRule)
	if err != nil {
		return nil, err
	}
	return &JobsManagerPassedVerificationIterator{contract: _JobsManager.contract, event: "PassedVerification", logs: logs, sub: sub}, nil
}

// WatchPassedVerification is a free log subscription operation binding the contract event 0x18d2d655e3f8d4b44ce95ed671c3f12339b2863d065ef91e970ac87826f45d8e.
//
// Solidity: event PassedVerification(transcoder indexed address, jobId indexed uint256, claimId indexed uint256, segmentNumber uint256)
func (_JobsManager *JobsManagerFilterer) WatchPassedVerification(opts *bind.WatchOpts, sink chan<- *JobsManagerPassedVerification, transcoder []common.Address, jobId []*big.Int, claimId []*big.Int) (event.Subscription, error) {

	var transcoderRule []interface{}
	for _, transcoderItem := range transcoder {
		transcoderRule = append(transcoderRule, transcoderItem)
	}
	var jobIdRule []interface{}
	for _, jobIdItem := range jobId {
		jobIdRule = append(jobIdRule, jobIdItem)
	}
	var claimIdRule []interface{}
	for _, claimIdItem := range claimId {
		claimIdRule = append(claimIdRule, claimIdItem)
	}

	logs, sub, err := _JobsManager.contract.WatchLogs(opts, "PassedVerification", transcoderRule, jobIdRule, claimIdRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(JobsManagerPassedVerification)
				if err := _JobsManager.contract.UnpackLog(event, "PassedVerification", log); err != nil {
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

// JobsManagerSetControllerIterator is returned from FilterSetController and is used to iterate over the raw logs and unpacked data for SetController events raised by the JobsManager contract.
type JobsManagerSetControllerIterator struct {
	Event *JobsManagerSetController // Event containing the contract specifics and raw log

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
func (it *JobsManagerSetControllerIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(JobsManagerSetController)
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
		it.Event = new(JobsManagerSetController)
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
func (it *JobsManagerSetControllerIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *JobsManagerSetControllerIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// JobsManagerSetController represents a SetController event raised by the JobsManager contract.
type JobsManagerSetController struct {
	Controller common.Address
	Raw        types.Log // Blockchain specific contextual infos
}

// FilterSetController is a free log retrieval operation binding the contract event 0x4ff638452bbf33c012645d18ae6f05515ff5f2d1dfb0cece8cbf018c60903f70.
//
// Solidity: event SetController(controller address)
func (_JobsManager *JobsManagerFilterer) FilterSetController(opts *bind.FilterOpts) (*JobsManagerSetControllerIterator, error) {

	logs, sub, err := _JobsManager.contract.FilterLogs(opts, "SetController")
	if err != nil {
		return nil, err
	}
	return &JobsManagerSetControllerIterator{contract: _JobsManager.contract, event: "SetController", logs: logs, sub: sub}, nil
}

// WatchSetController is a free log subscription operation binding the contract event 0x4ff638452bbf33c012645d18ae6f05515ff5f2d1dfb0cece8cbf018c60903f70.
//
// Solidity: event SetController(controller address)
func (_JobsManager *JobsManagerFilterer) WatchSetController(opts *bind.WatchOpts, sink chan<- *JobsManagerSetController) (event.Subscription, error) {

	logs, sub, err := _JobsManager.contract.WatchLogs(opts, "SetController")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(JobsManagerSetController)
				if err := _JobsManager.contract.UnpackLog(event, "SetController", log); err != nil {
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

// JobsManagerVerifyIterator is returned from FilterVerify and is used to iterate over the raw logs and unpacked data for Verify events raised by the JobsManager contract.
type JobsManagerVerifyIterator struct {
	Event *JobsManagerVerify // Event containing the contract specifics and raw log

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
func (it *JobsManagerVerifyIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(JobsManagerVerify)
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
		it.Event = new(JobsManagerVerify)
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
func (it *JobsManagerVerifyIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *JobsManagerVerifyIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// JobsManagerVerify represents a Verify event raised by the JobsManager contract.
type JobsManagerVerify struct {
	Transcoder    common.Address
	JobId         *big.Int
	ClaimId       *big.Int
	SegmentNumber *big.Int
	Raw           types.Log // Blockchain specific contextual infos
}

// FilterVerify is a free log retrieval operation binding the contract event 0x31e97f52032376a943c45aefa03fa9c7467b54ba1d66b85e54686424108209e3.
//
// Solidity: event Verify(transcoder indexed address, jobId indexed uint256, claimId indexed uint256, segmentNumber uint256)
func (_JobsManager *JobsManagerFilterer) FilterVerify(opts *bind.FilterOpts, transcoder []common.Address, jobId []*big.Int, claimId []*big.Int) (*JobsManagerVerifyIterator, error) {

	var transcoderRule []interface{}
	for _, transcoderItem := range transcoder {
		transcoderRule = append(transcoderRule, transcoderItem)
	}
	var jobIdRule []interface{}
	for _, jobIdItem := range jobId {
		jobIdRule = append(jobIdRule, jobIdItem)
	}
	var claimIdRule []interface{}
	for _, claimIdItem := range claimId {
		claimIdRule = append(claimIdRule, claimIdItem)
	}

	logs, sub, err := _JobsManager.contract.FilterLogs(opts, "Verify", transcoderRule, jobIdRule, claimIdRule)
	if err != nil {
		return nil, err
	}
	return &JobsManagerVerifyIterator{contract: _JobsManager.contract, event: "Verify", logs: logs, sub: sub}, nil
}

// WatchVerify is a free log subscription operation binding the contract event 0x31e97f52032376a943c45aefa03fa9c7467b54ba1d66b85e54686424108209e3.
//
// Solidity: event Verify(transcoder indexed address, jobId indexed uint256, claimId indexed uint256, segmentNumber uint256)
func (_JobsManager *JobsManagerFilterer) WatchVerify(opts *bind.WatchOpts, sink chan<- *JobsManagerVerify, transcoder []common.Address, jobId []*big.Int, claimId []*big.Int) (event.Subscription, error) {

	var transcoderRule []interface{}
	for _, transcoderItem := range transcoder {
		transcoderRule = append(transcoderRule, transcoderItem)
	}
	var jobIdRule []interface{}
	for _, jobIdItem := range jobId {
		jobIdRule = append(jobIdRule, jobIdItem)
	}
	var claimIdRule []interface{}
	for _, claimIdItem := range claimId {
		claimIdRule = append(claimIdRule, claimIdItem)
	}

	logs, sub, err := _JobsManager.contract.WatchLogs(opts, "Verify", transcoderRule, jobIdRule, claimIdRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(JobsManagerVerify)
				if err := _JobsManager.contract.UnpackLog(event, "Verify", log); err != nil {
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

// JobsManagerWithdrawIterator is returned from FilterWithdraw and is used to iterate over the raw logs and unpacked data for Withdraw events raised by the JobsManager contract.
type JobsManagerWithdrawIterator struct {
	Event *JobsManagerWithdraw // Event containing the contract specifics and raw log

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
func (it *JobsManagerWithdrawIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(JobsManagerWithdraw)
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
		it.Event = new(JobsManagerWithdraw)
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
func (it *JobsManagerWithdrawIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *JobsManagerWithdrawIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// JobsManagerWithdraw represents a Withdraw event raised by the JobsManager contract.
type JobsManagerWithdraw struct {
	Broadcaster common.Address
	Raw         types.Log // Blockchain specific contextual infos
}

// FilterWithdraw is a free log retrieval operation binding the contract event 0xf67611512e0a2d90c96fd3f08dca4971bc45fba9dc679eabe839a32abbe58a8e.
//
// Solidity: event Withdraw(broadcaster indexed address)
func (_JobsManager *JobsManagerFilterer) FilterWithdraw(opts *bind.FilterOpts, broadcaster []common.Address) (*JobsManagerWithdrawIterator, error) {

	var broadcasterRule []interface{}
	for _, broadcasterItem := range broadcaster {
		broadcasterRule = append(broadcasterRule, broadcasterItem)
	}

	logs, sub, err := _JobsManager.contract.FilterLogs(opts, "Withdraw", broadcasterRule)
	if err != nil {
		return nil, err
	}
	return &JobsManagerWithdrawIterator{contract: _JobsManager.contract, event: "Withdraw", logs: logs, sub: sub}, nil
}

// WatchWithdraw is a free log subscription operation binding the contract event 0xf67611512e0a2d90c96fd3f08dca4971bc45fba9dc679eabe839a32abbe58a8e.
//
// Solidity: event Withdraw(broadcaster indexed address)
func (_JobsManager *JobsManagerFilterer) WatchWithdraw(opts *bind.WatchOpts, sink chan<- *JobsManagerWithdraw, broadcaster []common.Address) (event.Subscription, error) {

	var broadcasterRule []interface{}
	for _, broadcasterItem := range broadcaster {
		broadcasterRule = append(broadcasterRule, broadcasterItem)
	}

	logs, sub, err := _JobsManager.contract.WatchLogs(opts, "Withdraw", broadcasterRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(JobsManagerWithdraw)
				if err := _JobsManager.contract.UnpackLog(event, "Withdraw", log); err != nil {
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
