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

// TranscodeJobsABI is the input ABI used to generate the binding from.
const TranscodeJobsABI = "[{\"constant\":false,\"inputs\":[{\"name\":\"self\",\"type\":\"TranscodeJobs.Jobsstorage\"},{\"name\":\"_streamId\",\"type\":\"uint256\"},{\"name\":\"_transcodingOptions\",\"type\":\"bytes32\"},{\"name\":\"_maxPricePerSegment\",\"type\":\"uint256\"},{\"name\":\"_electedTranscoder\",\"type\":\"address\"}],\"name\":\"newJob\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"self\",\"type\":\"TranscodeJobs.Jobsstorage\"},{\"name\":\"_jobId\",\"type\":\"uint256\"},{\"name\":\"_startSegmentSequenceNumber\",\"type\":\"uint256\"},{\"name\":\"_endSegmentSequenceNumber\",\"type\":\"uint256\"},{\"name\":\"_transcodeClaimsRoot\",\"type\":\"bytes32\"},{\"name\":\"_verificationPeriod\",\"type\":\"uint256\"}],\"name\":\"claimWork\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"self\",\"type\":\"TranscodeJobs.Jobsstorage\"},{\"name\":\"_jobId\",\"type\":\"uint256\"},{\"name\":\"_jobEndingPeriod\",\"type\":\"uint256\"}],\"name\":\"endJob\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"_segmentSequenceNumber\",\"type\":\"uint256\"},{\"name\":\"_startSegmentSequenceNumber\",\"type\":\"uint256\"},{\"name\":\"_endSegmentSequenceNumber\",\"type\":\"uint256\"},{\"name\":\"_lastClaimedWorkBlock\",\"type\":\"uint256\"},{\"name\":\"_lastClaimedWorkBlockHash\",\"type\":\"bytes32\"},{\"name\":\"_verificationRate\",\"type\":\"uint64\"}],\"name\":\"shouldVerifySegment\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"self\",\"type\":\"TranscodeJobs.Jobsstorage\"},{\"name\":\"_jobId\",\"type\":\"uint256\"},{\"name\":\"_segmentSequenceNumber\",\"type\":\"uint256\"},{\"name\":\"_dataHash\",\"type\":\"bytes32\"},{\"name\":\"_transcodedDataHash\",\"type\":\"bytes32\"},{\"name\":\"_broadcasterSig\",\"type\":\"bytes\"},{\"name\":\"_proof\",\"type\":\"bytes\"},{\"name\":\"_verificationRate\",\"type\":\"uint64\"}],\"name\":\"validateTranscoderClaim\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"self\",\"type\":\"TranscodeJobs.Jobsstorage\"},{\"name\":\"_jobId\",\"type\":\"uint256\"}],\"name\":\"getJobTranscodeClaimsDetails\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"},{\"name\":\"\",\"type\":\"uint256\"},{\"name\":\"\",\"type\":\"uint256\"},{\"name\":\"\",\"type\":\"uint256\"},{\"name\":\"\",\"type\":\"bytes32\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"self\",\"type\":\"TranscodeJobs.Jobsstorage\"},{\"name\":\"_jobId\",\"type\":\"uint256\"}],\"name\":\"getJob\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"},{\"name\":\"\",\"type\":\"uint256\"},{\"name\":\"\",\"type\":\"bytes32\"},{\"name\":\"\",\"type\":\"uint256\"},{\"name\":\"\",\"type\":\"address\"},{\"name\":\"\",\"type\":\"address\"},{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"type\":\"function\"}]"

// TranscodeJobsBin is the compiled bytecode used for deploying new contracts.
const TranscodeJobsBin = `0x6060604052341561000c57fe5b5b610a648061001c6000396000f300606060405236156100675763ffffffff60e060020a6000350416633300385881146100695780637f33aca31461009d5780638080f758146100cb578063ad6c0eca146100f0578063b9c2350b14610127578063c60c2881146101e2578063cce9f8dd1461021b575bfe5b610089600435602435604435606435600160a060020a036084351661026d565b604080519115158252519081900360200190f35b61008960043560243560443560643560843560a4356102f4565b604080519115158252519081900360200190f35b6100896004356024356044356103df565b604080519115158252519081900360200190f35b6100896004356024356044356064356084356001604060020a0360a4351661048a565b604080519115158252519081900360200190f35b604080516020601f60a4356004818101359283018490048402850184019095528184526100899480359460248035956044359560643595608435959460c494909391019190819084018382808284375050604080516020601f89358b01803591820183900483028401830190945280835297999881019791965091820194509250829150840183828082843750949650505092356001604060020a031692506104fb915050565b604080519115158252519081900360200190f35b6101f060043560243561082c565b6040805195865260208601949094528484019290925260608401526080830152519081900360a00190f35b61022960043560243561089d565b604080519788526020880196909652868601949094526060860192909252600160a060020a0390811660808601521660a084015260c0830152519081900360e00190f35b600180860180546000818152602089905260408082209283559184018890558254815281812060020187905582548152818120600301869055825481528181206004018054600160a060020a03338116600160a060020a03199283161790925584548352929091206005018054918616919092161790558054820190555b95945050505050565b600186015460009086106103085760006000fd5b60015b6103158888610919565b600181111561032057fe5b1461032b5760006000fd5b60008681526020889052604090206005015433600160a060020a039081169116146103565760006000fd5b6000868152602088905260409020600a01544310156103755760006000fd5b600086815260208890526040812086916007909101905b0160005b50556000868152602088905260409020849060070160015b0160005b505550600085815260208790526040902043600982018190558201600a820155600b0182905560015b9695505050505050565b600183015460009083106103f35760006000fd5b60008381526020859052604081206006015411156104115760006000fd5b60008381526020859052604090206004015433600160a060020a0390811691161480159061045d575060008381526020859052604090206005015433600160a060020a03908116911614155b156104685760006000fd5b50600082815260208490526040902043820160069091015560015b9392505050565b60008587108061049957508487115b156104a6575060006103d5565b604080518581526020810185905280820189905290519081900360600190206001604060020a038316908115156104d957fe5b0615156104e8575060016103d5565b5060006103d5565b5b9695505050505050565b6001880154600090881061050f5760006000fd5b60015b61051c8a8a610919565b600181111561052757fe5b1461053457506000610820565b600088815260208a9052604090206005015433600160a060020a0390811691161461056157506000610820565b600088815260208a9052604081206105b4918991600701905b0160005b505460008b815260208d90526040902060070160015b0160005b505460008c815260208e9052604090206009015480408761048a565b15156105c257506000610820565b600088815260208a9052604090206001015473__ECVerify______________________________906339cdde32906105fb908a8a610976565b60008b815260208d815260408083206004908101548251840194909452905160e060020a63ffffffff8716028152908101848152600160a060020a03909316604482018190526060602483019081528b5160648401528b518c95929492939192608490920191860190808383821561068e575b80518252602083111561068e57601f19909201916020918201910161066e565b505050905090810190601f1680156106ba5780820380516001836020036101000a031916815260200191505b5094505050505060206040518083038186803b15156106d557fe5b6102c65a03f415156106e357fe5b505060405151151590506106f957506000610820565b600088815260208a905260409020600b81015460019091015473__MerkleProof___________________________9163101f13e29186919061073e908c8c8c8c61099d565b604080516000602091820152905160e060020a63ffffffff8716028152602481018490526044810183905260606004820190815285516064830152855190928392608401919087019080838382156107b1575b8051825260208311156107b157601f199092019160209182019101610791565b505050905090810190601f1680156107dd5780820380516001836020036101000a031916815260200191505b5094505050505060206040518083038186803b15156107f857fe5b6102c65a03f4151561080657fe5b5050604051511515905061081c57506000610820565b5060015b98975050505050505050565b60006000600060006000600087600101548710151561084b5760006000fd5b5060008681526020889052604081206009810154600a820154919290919060078401905b0160005b50546007840160015b0160005b505484600b0154955095509550955095505b509295509295909350565b600060006000600060006000600060008960010154891015156108c05760006000fd5b50505060008681526020889052604090208054600182015460028301546003840154600485015460058601546006870154959b509399509197509550600160a060020a03908116945016915b5092959891949750929550565b6001820154600090821061092d5760006000fd5b60008281526020849052604081206006015411801561095e5750600082815260208490526040902060060154439011155b1561096b5750600061096f565b5060015b5b92915050565b604080518481526020810184905280820183905290519081900360600190205b9392505050565b60008585858585604051808681526020018581526020018460001916600019168152602001836000191660001916815260200182805190602001908083835b602083106109fb5780518252601f1990920191602091820191016109dc565b6001836020036101000a03801982511681845116808217855250505050505090500195505050505050604051809103902090505b959450505050505600a165627a7a72305820332bfca683b7aad4c6f88052a7071f90eda90c280646d9e54f7acff49bc364720029`

// DeployTranscodeJobs deploys a new Ethereum contract, binding an instance of TranscodeJobs to it.
func DeployTranscodeJobs(auth *bind.TransactOpts, backend bind.ContractBackend, libraries map[string]common.Address) (common.Address, *types.Transaction, *TranscodeJobs, error) {
	parsed, err := abi.JSON(strings.NewReader(TranscodeJobsABI))
	if err != nil {
		return common.Address{}, nil, nil, err
	}

	linkedBin := TranscodeJobsBin
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
	return address, tx, &TranscodeJobs{TranscodeJobsCaller: TranscodeJobsCaller{contract: contract}, TranscodeJobsTransactor: TranscodeJobsTransactor{contract: contract}}, nil
}

// TranscodeJobs is an auto generated Go binding around an Ethereum contract.
type TranscodeJobs struct {
	TranscodeJobsCaller     // Read-only binding to the contract
	TranscodeJobsTransactor // Write-only binding to the contract
}

// TranscodeJobsCaller is an auto generated read-only Go binding around an Ethereum contract.
type TranscodeJobsCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// TranscodeJobsTransactor is an auto generated write-only Go binding around an Ethereum contract.
type TranscodeJobsTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// TranscodeJobsSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type TranscodeJobsSession struct {
	Contract     *TranscodeJobs    // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// TranscodeJobsCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type TranscodeJobsCallerSession struct {
	Contract *TranscodeJobsCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts        // Call options to use throughout this session
}

// TranscodeJobsTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type TranscodeJobsTransactorSession struct {
	Contract     *TranscodeJobsTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts        // Transaction auth options to use throughout this session
}

// TranscodeJobsRaw is an auto generated low-level Go binding around an Ethereum contract.
type TranscodeJobsRaw struct {
	Contract *TranscodeJobs // Generic contract binding to access the raw methods on
}

// TranscodeJobsCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type TranscodeJobsCallerRaw struct {
	Contract *TranscodeJobsCaller // Generic read-only contract binding to access the raw methods on
}

// TranscodeJobsTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type TranscodeJobsTransactorRaw struct {
	Contract *TranscodeJobsTransactor // Generic write-only contract binding to access the raw methods on
}

// NewTranscodeJobs creates a new instance of TranscodeJobs, bound to a specific deployed contract.
func NewTranscodeJobs(address common.Address, backend bind.ContractBackend) (*TranscodeJobs, error) {
	contract, err := bindTranscodeJobs(address, backend, backend)
	if err != nil {
		return nil, err
	}
	return &TranscodeJobs{TranscodeJobsCaller: TranscodeJobsCaller{contract: contract}, TranscodeJobsTransactor: TranscodeJobsTransactor{contract: contract}}, nil
}

// NewTranscodeJobsCaller creates a new read-only instance of TranscodeJobs, bound to a specific deployed contract.
func NewTranscodeJobsCaller(address common.Address, caller bind.ContractCaller) (*TranscodeJobsCaller, error) {
	contract, err := bindTranscodeJobs(address, caller, nil)
	if err != nil {
		return nil, err
	}
	return &TranscodeJobsCaller{contract: contract}, nil
}

// NewTranscodeJobsTransactor creates a new write-only instance of TranscodeJobs, bound to a specific deployed contract.
func NewTranscodeJobsTransactor(address common.Address, transactor bind.ContractTransactor) (*TranscodeJobsTransactor, error) {
	contract, err := bindTranscodeJobs(address, nil, transactor)
	if err != nil {
		return nil, err
	}
	return &TranscodeJobsTransactor{contract: contract}, nil
}

// bindTranscodeJobs binds a generic wrapper to an already deployed contract.
func bindTranscodeJobs(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor) (*bind.BoundContract, error) {
	parsed, err := abi.JSON(strings.NewReader(TranscodeJobsABI))
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, parsed, caller, transactor), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_TranscodeJobs *TranscodeJobsRaw) Call(opts *bind.CallOpts, result interface{}, method string, params ...interface{}) error {
	return _TranscodeJobs.Contract.TranscodeJobsCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_TranscodeJobs *TranscodeJobsRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _TranscodeJobs.Contract.TranscodeJobsTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_TranscodeJobs *TranscodeJobsRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _TranscodeJobs.Contract.TranscodeJobsTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_TranscodeJobs *TranscodeJobsCallerRaw) Call(opts *bind.CallOpts, result interface{}, method string, params ...interface{}) error {
	return _TranscodeJobs.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_TranscodeJobs *TranscodeJobsTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _TranscodeJobs.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_TranscodeJobs *TranscodeJobsTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _TranscodeJobs.Contract.contract.Transact(opts, method, params...)
}

// GetJob is a free data retrieval call binding the contract method 0x91a8947d.
//
// Solidity: function getJob(self TranscodeJobs, _jobId uint256) constant returns(uint256, uint256, bytes32, uint256, address, address, uint256)
func (_TranscodeJobs *TranscodeJobsCaller) GetJob(opts *bind.CallOpts, self TranscodeJobs, _jobId *big.Int) (*big.Int, *big.Int, [32]byte, *big.Int, common.Address, common.Address, *big.Int, error) {
	var (
		ret0 = new(*big.Int)
		ret1 = new(*big.Int)
		ret2 = new([32]byte)
		ret3 = new(*big.Int)
		ret4 = new(common.Address)
		ret5 = new(common.Address)
		ret6 = new(*big.Int)
	)
	out := &[]interface{}{
		ret0,
		ret1,
		ret2,
		ret3,
		ret4,
		ret5,
		ret6,
	}
	err := _TranscodeJobs.contract.Call(opts, out, "getJob", self, _jobId)
	return *ret0, *ret1, *ret2, *ret3, *ret4, *ret5, *ret6, err
}

// GetJob is a free data retrieval call binding the contract method 0x91a8947d.
//
// Solidity: function getJob(self TranscodeJobs, _jobId uint256) constant returns(uint256, uint256, bytes32, uint256, address, address, uint256)
func (_TranscodeJobs *TranscodeJobsSession) GetJob(self TranscodeJobs, _jobId *big.Int) (*big.Int, *big.Int, [32]byte, *big.Int, common.Address, common.Address, *big.Int, error) {
	return _TranscodeJobs.Contract.GetJob(&_TranscodeJobs.CallOpts, self, _jobId)
}

// GetJob is a free data retrieval call binding the contract method 0x91a8947d.
//
// Solidity: function getJob(self TranscodeJobs, _jobId uint256) constant returns(uint256, uint256, bytes32, uint256, address, address, uint256)
func (_TranscodeJobs *TranscodeJobsCallerSession) GetJob(self TranscodeJobs, _jobId *big.Int) (*big.Int, *big.Int, [32]byte, *big.Int, common.Address, common.Address, *big.Int, error) {
	return _TranscodeJobs.Contract.GetJob(&_TranscodeJobs.CallOpts, self, _jobId)
}

// GetJobTranscodeClaimsDetails is a free data retrieval call binding the contract method 0x9a52386c.
//
// Solidity: function getJobTranscodeClaimsDetails(self TranscodeJobs, _jobId uint256) constant returns(uint256, uint256, uint256, uint256, bytes32)
func (_TranscodeJobs *TranscodeJobsCaller) GetJobTranscodeClaimsDetails(opts *bind.CallOpts, self TranscodeJobs, _jobId *big.Int) (*big.Int, *big.Int, *big.Int, *big.Int, [32]byte, error) {
	var (
		ret0 = new(*big.Int)
		ret1 = new(*big.Int)
		ret2 = new(*big.Int)
		ret3 = new(*big.Int)
		ret4 = new([32]byte)
	)
	out := &[]interface{}{
		ret0,
		ret1,
		ret2,
		ret3,
		ret4,
	}
	err := _TranscodeJobs.contract.Call(opts, out, "getJobTranscodeClaimsDetails", self, _jobId)
	return *ret0, *ret1, *ret2, *ret3, *ret4, err
}

// GetJobTranscodeClaimsDetails is a free data retrieval call binding the contract method 0x9a52386c.
//
// Solidity: function getJobTranscodeClaimsDetails(self TranscodeJobs, _jobId uint256) constant returns(uint256, uint256, uint256, uint256, bytes32)
func (_TranscodeJobs *TranscodeJobsSession) GetJobTranscodeClaimsDetails(self TranscodeJobs, _jobId *big.Int) (*big.Int, *big.Int, *big.Int, *big.Int, [32]byte, error) {
	return _TranscodeJobs.Contract.GetJobTranscodeClaimsDetails(&_TranscodeJobs.CallOpts, self, _jobId)
}

// GetJobTranscodeClaimsDetails is a free data retrieval call binding the contract method 0x9a52386c.
//
// Solidity: function getJobTranscodeClaimsDetails(self TranscodeJobs, _jobId uint256) constant returns(uint256, uint256, uint256, uint256, bytes32)
func (_TranscodeJobs *TranscodeJobsCallerSession) GetJobTranscodeClaimsDetails(self TranscodeJobs, _jobId *big.Int) (*big.Int, *big.Int, *big.Int, *big.Int, [32]byte, error) {
	return _TranscodeJobs.Contract.GetJobTranscodeClaimsDetails(&_TranscodeJobs.CallOpts, self, _jobId)
}

// ShouldVerifySegment is a free data retrieval call binding the contract method 0xad6c0eca.
//
// Solidity: function shouldVerifySegment(_segmentSequenceNumber uint256, _startSegmentSequenceNumber uint256, _endSegmentSequenceNumber uint256, _lastClaimedWorkBlock uint256, _lastClaimedWorkBlockHash bytes32, _verificationRate uint64) constant returns(bool)
func (_TranscodeJobs *TranscodeJobsCaller) ShouldVerifySegment(opts *bind.CallOpts, _segmentSequenceNumber *big.Int, _startSegmentSequenceNumber *big.Int, _endSegmentSequenceNumber *big.Int, _lastClaimedWorkBlock *big.Int, _lastClaimedWorkBlockHash [32]byte, _verificationRate uint64) (bool, error) {
	var (
		ret0 = new(bool)
	)
	out := ret0
	err := _TranscodeJobs.contract.Call(opts, out, "shouldVerifySegment", _segmentSequenceNumber, _startSegmentSequenceNumber, _endSegmentSequenceNumber, _lastClaimedWorkBlock, _lastClaimedWorkBlockHash, _verificationRate)
	return *ret0, err
}

// ShouldVerifySegment is a free data retrieval call binding the contract method 0xad6c0eca.
//
// Solidity: function shouldVerifySegment(_segmentSequenceNumber uint256, _startSegmentSequenceNumber uint256, _endSegmentSequenceNumber uint256, _lastClaimedWorkBlock uint256, _lastClaimedWorkBlockHash bytes32, _verificationRate uint64) constant returns(bool)
func (_TranscodeJobs *TranscodeJobsSession) ShouldVerifySegment(_segmentSequenceNumber *big.Int, _startSegmentSequenceNumber *big.Int, _endSegmentSequenceNumber *big.Int, _lastClaimedWorkBlock *big.Int, _lastClaimedWorkBlockHash [32]byte, _verificationRate uint64) (bool, error) {
	return _TranscodeJobs.Contract.ShouldVerifySegment(&_TranscodeJobs.CallOpts, _segmentSequenceNumber, _startSegmentSequenceNumber, _endSegmentSequenceNumber, _lastClaimedWorkBlock, _lastClaimedWorkBlockHash, _verificationRate)
}

// ShouldVerifySegment is a free data retrieval call binding the contract method 0xad6c0eca.
//
// Solidity: function shouldVerifySegment(_segmentSequenceNumber uint256, _startSegmentSequenceNumber uint256, _endSegmentSequenceNumber uint256, _lastClaimedWorkBlock uint256, _lastClaimedWorkBlockHash bytes32, _verificationRate uint64) constant returns(bool)
func (_TranscodeJobs *TranscodeJobsCallerSession) ShouldVerifySegment(_segmentSequenceNumber *big.Int, _startSegmentSequenceNumber *big.Int, _endSegmentSequenceNumber *big.Int, _lastClaimedWorkBlock *big.Int, _lastClaimedWorkBlockHash [32]byte, _verificationRate uint64) (bool, error) {
	return _TranscodeJobs.Contract.ShouldVerifySegment(&_TranscodeJobs.CallOpts, _segmentSequenceNumber, _startSegmentSequenceNumber, _endSegmentSequenceNumber, _lastClaimedWorkBlock, _lastClaimedWorkBlockHash, _verificationRate)
}

// ValidateTranscoderClaim is a free data retrieval call binding the contract method 0x2aec6e52.
//
// Solidity: function validateTranscoderClaim(self TranscodeJobs, _jobId uint256, _segmentSequenceNumber uint256, _dataHash bytes32, _transcodedDataHash bytes32, _broadcasterSig bytes, _proof bytes, _verificationRate uint64) constant returns(bool)
func (_TranscodeJobs *TranscodeJobsCaller) ValidateTranscoderClaim(opts *bind.CallOpts, self TranscodeJobs, _jobId *big.Int, _segmentSequenceNumber *big.Int, _dataHash [32]byte, _transcodedDataHash [32]byte, _broadcasterSig []byte, _proof []byte, _verificationRate uint64) (bool, error) {
	var (
		ret0 = new(bool)
	)
	out := ret0
	err := _TranscodeJobs.contract.Call(opts, out, "validateTranscoderClaim", self, _jobId, _segmentSequenceNumber, _dataHash, _transcodedDataHash, _broadcasterSig, _proof, _verificationRate)
	return *ret0, err
}

// ValidateTranscoderClaim is a free data retrieval call binding the contract method 0x2aec6e52.
//
// Solidity: function validateTranscoderClaim(self TranscodeJobs, _jobId uint256, _segmentSequenceNumber uint256, _dataHash bytes32, _transcodedDataHash bytes32, _broadcasterSig bytes, _proof bytes, _verificationRate uint64) constant returns(bool)
func (_TranscodeJobs *TranscodeJobsSession) ValidateTranscoderClaim(self TranscodeJobs, _jobId *big.Int, _segmentSequenceNumber *big.Int, _dataHash [32]byte, _transcodedDataHash [32]byte, _broadcasterSig []byte, _proof []byte, _verificationRate uint64) (bool, error) {
	return _TranscodeJobs.Contract.ValidateTranscoderClaim(&_TranscodeJobs.CallOpts, self, _jobId, _segmentSequenceNumber, _dataHash, _transcodedDataHash, _broadcasterSig, _proof, _verificationRate)
}

// ValidateTranscoderClaim is a free data retrieval call binding the contract method 0x2aec6e52.
//
// Solidity: function validateTranscoderClaim(self TranscodeJobs, _jobId uint256, _segmentSequenceNumber uint256, _dataHash bytes32, _transcodedDataHash bytes32, _broadcasterSig bytes, _proof bytes, _verificationRate uint64) constant returns(bool)
func (_TranscodeJobs *TranscodeJobsCallerSession) ValidateTranscoderClaim(self TranscodeJobs, _jobId *big.Int, _segmentSequenceNumber *big.Int, _dataHash [32]byte, _transcodedDataHash [32]byte, _broadcasterSig []byte, _proof []byte, _verificationRate uint64) (bool, error) {
	return _TranscodeJobs.Contract.ValidateTranscoderClaim(&_TranscodeJobs.CallOpts, self, _jobId, _segmentSequenceNumber, _dataHash, _transcodedDataHash, _broadcasterSig, _proof, _verificationRate)
}

// ClaimWork is a paid mutator transaction binding the contract method 0xb5eb6e87.
//
// Solidity: function claimWork(self TranscodeJobs, _jobId uint256, _startSegmentSequenceNumber uint256, _endSegmentSequenceNumber uint256, _transcodeClaimsRoot bytes32, _verificationPeriod uint256) returns(bool)
func (_TranscodeJobs *TranscodeJobsTransactor) ClaimWork(opts *bind.TransactOpts, self TranscodeJobs, _jobId *big.Int, _startSegmentSequenceNumber *big.Int, _endSegmentSequenceNumber *big.Int, _transcodeClaimsRoot [32]byte, _verificationPeriod *big.Int) (*types.Transaction, error) {
	return _TranscodeJobs.contract.Transact(opts, "claimWork", self, _jobId, _startSegmentSequenceNumber, _endSegmentSequenceNumber, _transcodeClaimsRoot, _verificationPeriod)
}

// ClaimWork is a paid mutator transaction binding the contract method 0xb5eb6e87.
//
// Solidity: function claimWork(self TranscodeJobs, _jobId uint256, _startSegmentSequenceNumber uint256, _endSegmentSequenceNumber uint256, _transcodeClaimsRoot bytes32, _verificationPeriod uint256) returns(bool)
func (_TranscodeJobs *TranscodeJobsSession) ClaimWork(self TranscodeJobs, _jobId *big.Int, _startSegmentSequenceNumber *big.Int, _endSegmentSequenceNumber *big.Int, _transcodeClaimsRoot [32]byte, _verificationPeriod *big.Int) (*types.Transaction, error) {
	return _TranscodeJobs.Contract.ClaimWork(&_TranscodeJobs.TransactOpts, self, _jobId, _startSegmentSequenceNumber, _endSegmentSequenceNumber, _transcodeClaimsRoot, _verificationPeriod)
}

// ClaimWork is a paid mutator transaction binding the contract method 0xb5eb6e87.
//
// Solidity: function claimWork(self TranscodeJobs, _jobId uint256, _startSegmentSequenceNumber uint256, _endSegmentSequenceNumber uint256, _transcodeClaimsRoot bytes32, _verificationPeriod uint256) returns(bool)
func (_TranscodeJobs *TranscodeJobsTransactorSession) ClaimWork(self TranscodeJobs, _jobId *big.Int, _startSegmentSequenceNumber *big.Int, _endSegmentSequenceNumber *big.Int, _transcodeClaimsRoot [32]byte, _verificationPeriod *big.Int) (*types.Transaction, error) {
	return _TranscodeJobs.Contract.ClaimWork(&_TranscodeJobs.TransactOpts, self, _jobId, _startSegmentSequenceNumber, _endSegmentSequenceNumber, _transcodeClaimsRoot, _verificationPeriod)
}

// EndJob is a paid mutator transaction binding the contract method 0xda261f68.
//
// Solidity: function endJob(self TranscodeJobs, _jobId uint256, _jobEndingPeriod uint256) returns(bool)
func (_TranscodeJobs *TranscodeJobsTransactor) EndJob(opts *bind.TransactOpts, self TranscodeJobs, _jobId *big.Int, _jobEndingPeriod *big.Int) (*types.Transaction, error) {
	return _TranscodeJobs.contract.Transact(opts, "endJob", self, _jobId, _jobEndingPeriod)
}

// EndJob is a paid mutator transaction binding the contract method 0xda261f68.
//
// Solidity: function endJob(self TranscodeJobs, _jobId uint256, _jobEndingPeriod uint256) returns(bool)
func (_TranscodeJobs *TranscodeJobsSession) EndJob(self TranscodeJobs, _jobId *big.Int, _jobEndingPeriod *big.Int) (*types.Transaction, error) {
	return _TranscodeJobs.Contract.EndJob(&_TranscodeJobs.TransactOpts, self, _jobId, _jobEndingPeriod)
}

// EndJob is a paid mutator transaction binding the contract method 0xda261f68.
//
// Solidity: function endJob(self TranscodeJobs, _jobId uint256, _jobEndingPeriod uint256) returns(bool)
func (_TranscodeJobs *TranscodeJobsTransactorSession) EndJob(self TranscodeJobs, _jobId *big.Int, _jobEndingPeriod *big.Int) (*types.Transaction, error) {
	return _TranscodeJobs.Contract.EndJob(&_TranscodeJobs.TransactOpts, self, _jobId, _jobEndingPeriod)
}

// NewJob is a paid mutator transaction binding the contract method 0xa0b235ba.
//
// Solidity: function newJob(self TranscodeJobs, _streamId uint256, _transcodingOptions bytes32, _maxPricePerSegment uint256, _electedTranscoder address) returns(bool)
func (_TranscodeJobs *TranscodeJobsTransactor) NewJob(opts *bind.TransactOpts, self TranscodeJobs, _streamId *big.Int, _transcodingOptions [32]byte, _maxPricePerSegment *big.Int, _electedTranscoder common.Address) (*types.Transaction, error) {
	return _TranscodeJobs.contract.Transact(opts, "newJob", self, _streamId, _transcodingOptions, _maxPricePerSegment, _electedTranscoder)
}

// NewJob is a paid mutator transaction binding the contract method 0xa0b235ba.
//
// Solidity: function newJob(self TranscodeJobs, _streamId uint256, _transcodingOptions bytes32, _maxPricePerSegment uint256, _electedTranscoder address) returns(bool)
func (_TranscodeJobs *TranscodeJobsSession) NewJob(self TranscodeJobs, _streamId *big.Int, _transcodingOptions [32]byte, _maxPricePerSegment *big.Int, _electedTranscoder common.Address) (*types.Transaction, error) {
	return _TranscodeJobs.Contract.NewJob(&_TranscodeJobs.TransactOpts, self, _streamId, _transcodingOptions, _maxPricePerSegment, _electedTranscoder)
}

// NewJob is a paid mutator transaction binding the contract method 0xa0b235ba.
//
// Solidity: function newJob(self TranscodeJobs, _streamId uint256, _transcodingOptions bytes32, _maxPricePerSegment uint256, _electedTranscoder address) returns(bool)
func (_TranscodeJobs *TranscodeJobsTransactorSession) NewJob(self TranscodeJobs, _streamId *big.Int, _transcodingOptions [32]byte, _maxPricePerSegment *big.Int, _electedTranscoder common.Address) (*types.Transaction, error) {
	return _TranscodeJobs.Contract.NewJob(&_TranscodeJobs.TransactOpts, self, _streamId, _transcodingOptions, _maxPricePerSegment, _electedTranscoder)
}
