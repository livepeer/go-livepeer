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

// JobLibABI is the input ABI used to generate the binding from.
const JobLibABI = "[{\"constant\":true,\"inputs\":[{\"name\":\"_segmentNumber\",\"type\":\"uint256\"},{\"name\":\"_segmentRange\",\"type\":\"uint256[2]\"},{\"name\":\"_claimBlock\",\"type\":\"uint256\"},{\"name\":\"_verificationRate\",\"type\":\"uint64\"}],\"name\":\"shouldVerifySegment\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"_streamId\",\"type\":\"string\"},{\"name\":\"_segmentNumber\",\"type\":\"uint256\"},{\"name\":\"_dataHash\",\"type\":\"string\"},{\"name\":\"_transcodedDataHash\",\"type\":\"string\"},{\"name\":\"_broadcasterSig\",\"type\":\"bytes\"}],\"name\":\"transcodeReceiptHash\",\"outputs\":[{\"name\":\"\",\"type\":\"bytes32\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"_streamId\",\"type\":\"string\"},{\"name\":\"_segmentNumber\",\"type\":\"uint256\"},{\"name\":\"_dataHash\",\"type\":\"string\"}],\"name\":\"personalSegmentHash\",\"outputs\":[{\"name\":\"\",\"type\":\"bytes32\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"_streamId\",\"type\":\"string\"},{\"name\":\"_segmentNumber\",\"type\":\"uint256\"},{\"name\":\"_dataHash\",\"type\":\"string\"}],\"name\":\"segmentHash\",\"outputs\":[{\"name\":\"\",\"type\":\"bytes32\"}],\"payable\":false,\"type\":\"function\"}]"

// JobLibBin is the compiled bytecode used for deploying new contracts.
const JobLibBin = `0x6060604052341561000f57600080fd5b5b6106be8061001f6000396000f300606060405263ffffffff7c01000000000000000000000000000000000000000000000000000000006000350416634152b382811461005e57806347a6d1f3146100b8578063afcc9967146101df578063e52c921b14610282575b600080fd5b6100a4600480359060646024600260408051908101604052809291908260026020028082843750939550508335936020013567ffffffffffffffff169250610325915050565b604051901515815260200160405180910390f35b6101cd60046024813581810190830135806020601f8201819004810201604051908101604052818152929190602084018383808284378201915050505050509190803590602001909190803590602001908201803590602001908080601f01602080910402602001604051908101604052818152929190602084018383808284378201915050505050509190803590602001908201803590602001908080601f01602080910402602001604051908101604052818152929190602084018383808284378201915050505050509190803590602001908201803590602001908080601f0160208091040260200160405190810160405281815292919060208401838380828437509496506103a895505050505050565b60405190815260200160405180910390f35b6101cd60046024813581810190830135806020601f8201819004810201604051908101604052818152929190602084018383808284378201915050505050509190803590602001909190803590602001908201803590602001908080601f01602080910402602001604051908101604052818152929190602084018383808284375094965061050895505050505050565b60405190815260200160405180910390f35b6101cd60046024813581810190830135806020601f8201819004810201604051908101604052818152929190602084018383808284378201915050505050509190803590602001909190803590602001908201803590602001908080601f0160208091040260200160405190810160405281815292919060208401838380828437509496506105c095505050505050565b60405190815260200160405180910390f35b600083815b602002015185108061034357508360015b602002015185115b156103505750600061039f565b8167ffffffffffffffff168384408760405192835260208301919091526040808301919091526060909101905190819003902081151561038c57fe5b06151561039b5750600161039f565b5060005b5b949350505050565b600085858585856040518086805190602001908083835b602083106103df57805182525b601f1990920191602091820191016103bf565b6001836020036101000a038019825116818451161790925250505091909101868152602001905084805190602001908083835b6020831061043257805182525b601f199092019160209182019101610412565b6001836020036101000a038019825116818451161790925250505091909101905083805190602001908083835b6020831061047f57805182525b601f19909201916020918201910161045f565b6001836020036101000a038019825116818451161790925250505091909101905082805190602001908083835b602083106104cc57805182525b601f1990920191602091820191016104ac565b6001836020036101000a038019825116818451161790925250505091909101965060409550505050505051809103902090505b95945050505050565b6000610512610680565b60408051908101604052601c81527f19457468657265756d205369676e6564204d6573736167653a0a33320000000060208201529050806105548686866105c0565b6040518083805190602001908083835b6020831061058457805182525b601f199092019160209182019101610564565b6001836020036101000a03801982511681845116179092525050509190910192835250506020019050604051809103902091505b509392505050565b60008383836040518084805190602001908083835b602083106105f557805182525b601f1990920191602091820191016105d5565b6001836020036101000a038019825116818451161790925250505091909101848152602001905082805190602001908083835b6020831061064857805182525b601f199092019160209182019101610628565b6001836020036101000a03801982511681845116179092525050509190910194506040935050505051809103902090505b9392505050565b602060405190810160405260008152905600a165627a7a72305820bce481341915f3d0fe2fc4b2bca71e5a7c60d6da68632b189d4fe257cadce8110029`

// DeployJobLib deploys a new Ethereum contract, binding an instance of JobLib to it.
func DeployJobLib(auth *bind.TransactOpts, backend bind.ContractBackend, libraries map[string]common.Address) (common.Address, *types.Transaction, *JobLib, error) {
	parsed, err := abi.JSON(strings.NewReader(JobLibABI))
	if err != nil {
		return common.Address{}, nil, nil, err
	}

	linkedBin := JobLibBin
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
	return address, tx, &JobLib{JobLibCaller: JobLibCaller{contract: contract}, JobLibTransactor: JobLibTransactor{contract: contract}}, nil
}

// JobLib is an auto generated Go binding around an Ethereum contract.
type JobLib struct {
	JobLibCaller     // Read-only binding to the contract
	JobLibTransactor // Write-only binding to the contract
}

// JobLibCaller is an auto generated read-only Go binding around an Ethereum contract.
type JobLibCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// JobLibTransactor is an auto generated write-only Go binding around an Ethereum contract.
type JobLibTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// JobLibSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type JobLibSession struct {
	Contract     *JobLib           // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// JobLibCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type JobLibCallerSession struct {
	Contract *JobLibCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts // Call options to use throughout this session
}

// JobLibTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type JobLibTransactorSession struct {
	Contract     *JobLibTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// JobLibRaw is an auto generated low-level Go binding around an Ethereum contract.
type JobLibRaw struct {
	Contract *JobLib // Generic contract binding to access the raw methods on
}

// JobLibCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type JobLibCallerRaw struct {
	Contract *JobLibCaller // Generic read-only contract binding to access the raw methods on
}

// JobLibTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type JobLibTransactorRaw struct {
	Contract *JobLibTransactor // Generic write-only contract binding to access the raw methods on
}

// NewJobLib creates a new instance of JobLib, bound to a specific deployed contract.
func NewJobLib(address common.Address, backend bind.ContractBackend) (*JobLib, error) {
	contract, err := bindJobLib(address, backend, backend)
	if err != nil {
		return nil, err
	}
	return &JobLib{JobLibCaller: JobLibCaller{contract: contract}, JobLibTransactor: JobLibTransactor{contract: contract}}, nil
}

// NewJobLibCaller creates a new read-only instance of JobLib, bound to a specific deployed contract.
func NewJobLibCaller(address common.Address, caller bind.ContractCaller) (*JobLibCaller, error) {
	contract, err := bindJobLib(address, caller, nil)
	if err != nil {
		return nil, err
	}
	return &JobLibCaller{contract: contract}, nil
}

// NewJobLibTransactor creates a new write-only instance of JobLib, bound to a specific deployed contract.
func NewJobLibTransactor(address common.Address, transactor bind.ContractTransactor) (*JobLibTransactor, error) {
	contract, err := bindJobLib(address, nil, transactor)
	if err != nil {
		return nil, err
	}
	return &JobLibTransactor{contract: contract}, nil
}

// bindJobLib binds a generic wrapper to an already deployed contract.
func bindJobLib(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor) (*bind.BoundContract, error) {
	parsed, err := abi.JSON(strings.NewReader(JobLibABI))
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, parsed, caller, transactor), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_JobLib *JobLibRaw) Call(opts *bind.CallOpts, result interface{}, method string, params ...interface{}) error {
	return _JobLib.Contract.JobLibCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_JobLib *JobLibRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _JobLib.Contract.JobLibTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_JobLib *JobLibRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _JobLib.Contract.JobLibTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_JobLib *JobLibCallerRaw) Call(opts *bind.CallOpts, result interface{}, method string, params ...interface{}) error {
	return _JobLib.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_JobLib *JobLibTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _JobLib.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_JobLib *JobLibTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _JobLib.Contract.contract.Transact(opts, method, params...)
}

// PersonalSegmentHash is a free data retrieval call binding the contract method 0xafcc9967.
//
// Solidity: function personalSegmentHash(_streamId string, _segmentNumber uint256, _dataHash string) constant returns(bytes32)
func (_JobLib *JobLibCaller) PersonalSegmentHash(opts *bind.CallOpts, _streamId string, _segmentNumber *big.Int, _dataHash string) ([32]byte, error) {
	var (
		ret0 = new([32]byte)
	)
	out := ret0
	err := _JobLib.contract.Call(opts, out, "personalSegmentHash", _streamId, _segmentNumber, _dataHash)
	return *ret0, err
}

// PersonalSegmentHash is a free data retrieval call binding the contract method 0xafcc9967.
//
// Solidity: function personalSegmentHash(_streamId string, _segmentNumber uint256, _dataHash string) constant returns(bytes32)
func (_JobLib *JobLibSession) PersonalSegmentHash(_streamId string, _segmentNumber *big.Int, _dataHash string) ([32]byte, error) {
	return _JobLib.Contract.PersonalSegmentHash(&_JobLib.CallOpts, _streamId, _segmentNumber, _dataHash)
}

// PersonalSegmentHash is a free data retrieval call binding the contract method 0xafcc9967.
//
// Solidity: function personalSegmentHash(_streamId string, _segmentNumber uint256, _dataHash string) constant returns(bytes32)
func (_JobLib *JobLibCallerSession) PersonalSegmentHash(_streamId string, _segmentNumber *big.Int, _dataHash string) ([32]byte, error) {
	return _JobLib.Contract.PersonalSegmentHash(&_JobLib.CallOpts, _streamId, _segmentNumber, _dataHash)
}

// SegmentHash is a free data retrieval call binding the contract method 0xe52c921b.
//
// Solidity: function segmentHash(_streamId string, _segmentNumber uint256, _dataHash string) constant returns(bytes32)
func (_JobLib *JobLibCaller) SegmentHash(opts *bind.CallOpts, _streamId string, _segmentNumber *big.Int, _dataHash string) ([32]byte, error) {
	var (
		ret0 = new([32]byte)
	)
	out := ret0
	err := _JobLib.contract.Call(opts, out, "segmentHash", _streamId, _segmentNumber, _dataHash)
	return *ret0, err
}

// SegmentHash is a free data retrieval call binding the contract method 0xe52c921b.
//
// Solidity: function segmentHash(_streamId string, _segmentNumber uint256, _dataHash string) constant returns(bytes32)
func (_JobLib *JobLibSession) SegmentHash(_streamId string, _segmentNumber *big.Int, _dataHash string) ([32]byte, error) {
	return _JobLib.Contract.SegmentHash(&_JobLib.CallOpts, _streamId, _segmentNumber, _dataHash)
}

// SegmentHash is a free data retrieval call binding the contract method 0xe52c921b.
//
// Solidity: function segmentHash(_streamId string, _segmentNumber uint256, _dataHash string) constant returns(bytes32)
func (_JobLib *JobLibCallerSession) SegmentHash(_streamId string, _segmentNumber *big.Int, _dataHash string) ([32]byte, error) {
	return _JobLib.Contract.SegmentHash(&_JobLib.CallOpts, _streamId, _segmentNumber, _dataHash)
}

// ShouldVerifySegment is a free data retrieval call binding the contract method 0x4152b382.
//
// Solidity: function shouldVerifySegment(_segmentNumber uint256, _segmentRange uint256[2], _claimBlock uint256, _verificationRate uint64) constant returns(bool)
func (_JobLib *JobLibCaller) ShouldVerifySegment(opts *bind.CallOpts, _segmentNumber *big.Int, _segmentRange [2]*big.Int, _claimBlock *big.Int, _verificationRate uint64) (bool, error) {
	var (
		ret0 = new(bool)
	)
	out := ret0
	err := _JobLib.contract.Call(opts, out, "shouldVerifySegment", _segmentNumber, _segmentRange, _claimBlock, _verificationRate)
	return *ret0, err
}

// ShouldVerifySegment is a free data retrieval call binding the contract method 0x4152b382.
//
// Solidity: function shouldVerifySegment(_segmentNumber uint256, _segmentRange uint256[2], _claimBlock uint256, _verificationRate uint64) constant returns(bool)
func (_JobLib *JobLibSession) ShouldVerifySegment(_segmentNumber *big.Int, _segmentRange [2]*big.Int, _claimBlock *big.Int, _verificationRate uint64) (bool, error) {
	return _JobLib.Contract.ShouldVerifySegment(&_JobLib.CallOpts, _segmentNumber, _segmentRange, _claimBlock, _verificationRate)
}

// ShouldVerifySegment is a free data retrieval call binding the contract method 0x4152b382.
//
// Solidity: function shouldVerifySegment(_segmentNumber uint256, _segmentRange uint256[2], _claimBlock uint256, _verificationRate uint64) constant returns(bool)
func (_JobLib *JobLibCallerSession) ShouldVerifySegment(_segmentNumber *big.Int, _segmentRange [2]*big.Int, _claimBlock *big.Int, _verificationRate uint64) (bool, error) {
	return _JobLib.Contract.ShouldVerifySegment(&_JobLib.CallOpts, _segmentNumber, _segmentRange, _claimBlock, _verificationRate)
}

// TranscodeReceiptHash is a free data retrieval call binding the contract method 0x47a6d1f3.
//
// Solidity: function transcodeReceiptHash(_streamId string, _segmentNumber uint256, _dataHash string, _transcodedDataHash string, _broadcasterSig bytes) constant returns(bytes32)
func (_JobLib *JobLibCaller) TranscodeReceiptHash(opts *bind.CallOpts, _streamId string, _segmentNumber *big.Int, _dataHash string, _transcodedDataHash string, _broadcasterSig []byte) ([32]byte, error) {
	var (
		ret0 = new([32]byte)
	)
	out := ret0
	err := _JobLib.contract.Call(opts, out, "transcodeReceiptHash", _streamId, _segmentNumber, _dataHash, _transcodedDataHash, _broadcasterSig)
	return *ret0, err
}

// TranscodeReceiptHash is a free data retrieval call binding the contract method 0x47a6d1f3.
//
// Solidity: function transcodeReceiptHash(_streamId string, _segmentNumber uint256, _dataHash string, _transcodedDataHash string, _broadcasterSig bytes) constant returns(bytes32)
func (_JobLib *JobLibSession) TranscodeReceiptHash(_streamId string, _segmentNumber *big.Int, _dataHash string, _transcodedDataHash string, _broadcasterSig []byte) ([32]byte, error) {
	return _JobLib.Contract.TranscodeReceiptHash(&_JobLib.CallOpts, _streamId, _segmentNumber, _dataHash, _transcodedDataHash, _broadcasterSig)
}

// TranscodeReceiptHash is a free data retrieval call binding the contract method 0x47a6d1f3.
//
// Solidity: function transcodeReceiptHash(_streamId string, _segmentNumber uint256, _dataHash string, _transcodedDataHash string, _broadcasterSig bytes) constant returns(bytes32)
func (_JobLib *JobLibCallerSession) TranscodeReceiptHash(_streamId string, _segmentNumber *big.Int, _dataHash string, _transcodedDataHash string, _broadcasterSig []byte) ([32]byte, error) {
	return _JobLib.Contract.TranscodeReceiptHash(&_JobLib.CallOpts, _streamId, _segmentNumber, _dataHash, _transcodedDataHash, _broadcasterSig)
}
