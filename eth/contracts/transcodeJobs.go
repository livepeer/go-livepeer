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
const TranscodeJobsABI = "[{\"constant\":false,\"inputs\":[{\"name\":\"self\",\"type\":\"TranscodeJobs.Jobsstorage\"},{\"name\":\"_jobId\",\"type\":\"uint256\"},{\"name\":\"_startSegmentSequenceNumber\",\"type\":\"uint256\"},{\"name\":\"_endSegmentSequenceNumber\",\"type\":\"uint256\"},{\"name\":\"_transcodeClaimsRoot\",\"type\":\"bytes32\"},{\"name\":\"_verificationPeriod\",\"type\":\"uint256\"}],\"name\":\"claimWork\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"self\",\"type\":\"TranscodeJobs.Jobsstorage\"},{\"name\":\"_jobId\",\"type\":\"uint256\"},{\"name\":\"_jobEndingPeriod\",\"type\":\"uint256\"}],\"name\":\"endJob\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"_segmentSequenceNumber\",\"type\":\"uint256\"},{\"name\":\"_startSegmentSequenceNumber\",\"type\":\"uint256\"},{\"name\":\"_endSegmentSequenceNumber\",\"type\":\"uint256\"},{\"name\":\"_lastClaimedWorkBlock\",\"type\":\"uint256\"},{\"name\":\"_lastClaimedWorkBlockHash\",\"type\":\"bytes32\"},{\"name\":\"_verificationRate\",\"type\":\"uint64\"}],\"name\":\"shouldVerifySegment\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"self\",\"type\":\"TranscodeJobs.Jobsstorage\"},{\"name\":\"_jobId\",\"type\":\"uint256\"},{\"name\":\"_segmentSequenceNumber\",\"type\":\"uint256\"},{\"name\":\"_dataHash\",\"type\":\"bytes32\"},{\"name\":\"_transcodedDataHash\",\"type\":\"bytes32\"},{\"name\":\"_broadcasterSig\",\"type\":\"bytes\"},{\"name\":\"_proof\",\"type\":\"bytes\"},{\"name\":\"_verificationRate\",\"type\":\"uint64\"}],\"name\":\"validateTranscoderClaim\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"self\",\"type\":\"TranscodeJobs.Jobsstorage\"},{\"name\":\"_jobId\",\"type\":\"uint256\"}],\"name\":\"getJobTranscodeClaimsDetails\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"},{\"name\":\"\",\"type\":\"uint256\"},{\"name\":\"\",\"type\":\"uint256\"},{\"name\":\"\",\"type\":\"uint256\"},{\"name\":\"\",\"type\":\"bytes32\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"self\",\"type\":\"TranscodeJobs.Jobsstorage\"},{\"name\":\"_streamId\",\"type\":\"string\"},{\"name\":\"_transcodingOptions\",\"type\":\"bytes32\"},{\"name\":\"_maxPricePerSegment\",\"type\":\"uint256\"},{\"name\":\"_electedTranscoder\",\"type\":\"address\"}],\"name\":\"newJob\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"self\",\"type\":\"TranscodeJobs.Jobsstorage\"},{\"name\":\"_jobId\",\"type\":\"uint256\"}],\"name\":\"getJob\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"},{\"name\":\"\",\"type\":\"bytes32\"},{\"name\":\"\",\"type\":\"uint256\"},{\"name\":\"\",\"type\":\"address\"},{\"name\":\"\",\"type\":\"address\"},{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"type\":\"function\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"name\":\"transcoder\",\"type\":\"address\"},{\"indexed\":true,\"name\":\"broadcaster\",\"type\":\"address\"}],\"name\":\"NewJob\",\"type\":\"event\"}]"

// TranscodeJobsBin is the compiled bytecode used for deploying new contracts.
const TranscodeJobsBin = `0x6060604052341561000c57fe5b5b610d0a8061001c6000396000f300606060405236156100675763ffffffff60e060020a6000350416637f33aca381146100695780638080f75814610097578063ad6c0eca146100bc578063b9c2350b146100f3578063c60c2881146101ae578063ca13292d146101e7578063cce9f8dd14610260575bfe5b61008360043560243560443560643560843560a4356102aa565b604080519115158252519081900360200190f35b610083600435602435604435610395565b604080519115158252519081900360200190f35b6100836004356024356044356064356084356001604060020a0360a43516610440565b604080519115158252519081900360200190f35b604080516020601f60a4356004818101359283018490048402850184019095528184526100839480359460248035956044359560643595608435959460c494909391019190819084018382808284375050604080516020601f89358b01803591820183900483028401830190945280835297999881019791965091820194509250829150840183828082843750949650505092356001604060020a031692506104b1915050565b604080519115158252519081900360200190f35b6101bc6004356024356108f0565b6040805195865260208601949094528484019290925260608401526080830152519081900360a00190f35b60408051602060046024803582810135601f81018590048502860185019096528585526100839583359593946044949392909201918190840183828082843750949650508435946020810135945060400135600160a060020a03169250610961915050565b604080519115158252519081900360200190f35b61026e600435602435610a31565b60408051968752602087019590955285850193909352600160a060020a03918216606086015216608084015260a0830152519081900360c00190f35b600186015460009086106102be5760006000fd5b60015b6102cb8888610aa2565b60018111156102d657fe5b146102e15760006000fd5b60008681526020889052604090206005015433600160a060020a0390811691161461030c5760006000fd5b6000868152602088905260409020600a015443101561032b5760006000fd5b600086815260208890526040812086916007909101905b0160005b50556000868152602088905260409020849060070160015b0160005b505550600085815260208790526040902043600982018190558201600a820155600b0182905560015b9695505050505050565b600183015460009083106103a95760006000fd5b60008381526020859052604081206006015411156103c75760006000fd5b60008381526020859052604090206004015433600160a060020a03908116911614801590610413575060008381526020859052604090206005015433600160a060020a03908116911614155b1561041e5760006000fd5b50600082815260208490526040902043820160069091015560015b9392505050565b60008587108061044f57508487115b1561045c5750600061038b565b604080518581526020810185905280820189905290519081900360600190206001604060020a0383169081151561048f57fe5b06151561049e5750600161038b565b50600061038b565b5b9695505050505050565b600188015460009088106104c55760006000fd5b60015b6104d28a8a610aa2565b60018111156104dd57fe5b146104ea575060006108e4565b600088815260208a9052604090206005015433600160a060020a03908116911614610517575060006108e4565b600088815260208a90526040812061056a918991600701905b0160005b505460008b815260208d90526040902060070160015b0160005b505460008c815260208e90526040902060090154804087610440565b1515610578575060006108e4565b600088815260208a8152604091829020600190810180548451600293821615610100026000190190911692909204601f810184900484028301840190945283825273__ECVerify______________________________936339cdde32936106399392919083018282801561062d5780601f106106025761010080835404028352916020019161062d565b820191906000526020600020905b81548152906001019060200180831161061057829003601f168201915b50505050508a8a610aff565b60008b815260208d815260408083206004908101548251840194909452905160e060020a63ffffffff8716028152908101848152600160a060020a03909316604482018190526060602483019081528b5160648401528b518c9592949293919260849092019186019080838382156106cc575b8051825260208311156106cc57601f1990920191602091820191016106ac565b505050905090810190601f1680156106f85780820380516001836020036101000a031916815260200191505b5094505050505060206040518083038186803b151561071357fe5b6102c65a03f4151561072157fe5b50506040515115159050610737575060006108e4565b600088815260208a8152604091829020600b810154600191820180548551600294821615610100026000190190911693909304601f810185900485028401850190955284835273__MerkleProof___________________________9463101f13e294899461080293909290918301828280156107f45780601f106107c9576101008083540402835291602001916107f4565b820191906000526020600020905b8154815290600101906020018083116107d757829003601f168201915b50505050508c8c8c8c610b70565b604080516000602091820152905160e060020a63ffffffff871602815260248101849052604481018390526060600482019081528551606483015285519092839260840191908701908083838215610875575b80518252602083111561087557601f199092019160209182019101610855565b505050905090810190601f1680156108a15780820380516001836020036101000a031916815260200191505b5094505050505060206040518083038186803b15156108bc57fe5b6102c65a03f415156108ca57fe5b505060405151151590506108e0575060006108e4565b5060015b98975050505050505050565b60006000600060006000600087600101548710151561090f5760006000fd5b5060008681526020889052604081206009810154600a820154919290919060078401905b0160005b50546007840160015b0160005b505484600b0154955095509550955095505b509295509295909350565b60018086015460008181526020888152604082209283558751919361098b93019190880190610c3e565b506001868101805460009081526020899052604080822060020188905582548252808220600301879055825482528082206004018054600160a060020a031990811633600160a060020a039081169182179093558554855283852060050180549092169289169283179091558454909501909355517f90742bd3e5961d457f589126dcc2b7350ddd0ab331e81b6844e2bde6172464909190a35060015b95945050505050565b6000600060006000600060006000886001015488101515610a525760006000fd5b5050506000858152602087905260409020805460028201546003830154600484015460058501546006860154949950929750909550600160a060020a03908116945016915b509295509295509295565b60018201546000908210610ab65760006000fd5b600082815260208490526040812060060154118015610ae75750600082815260208490526040902060060154439011155b15610af457506000610af8565b5060015b5b92915050565b60008383836040518084805190602001908083835b60208310610b335780518252601f199092019160209182019101610b14565b51815160209384036101000a60001901801990921691161790529201948552508301919091525060408051918290030190209150505b9392505050565b600085858585856040518086805190602001908083835b60208310610ba65780518252601f199092019160209182019101610b87565b51815160209384036101000a60001901801990921691161790529201878152808301879052604081018690528451606090910192850191508083835b60208310610c015780518252601f199092019160209182019101610be2565b6001836020036101000a03801982511681845116808217855250505050505090500195505050505050604051809103902090505b95945050505050565b828054600181600116156101000203166002900490600052602060002090601f016020900481019282601f10610c7f57805160ff1916838001178555610cac565b82800160010185558215610cac579182015b82811115610cac578251825591602001919060010190610c91565b5b50610cb9929150610cbd565b5090565b610cdb91905b80821115610cb95760008155600101610cc3565b5090565b905600a165627a7a7230582070b29de8d728fbd254d97debb08fd1a339b895069e5715927c6806630448af510029`

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
// Solidity: function getJob(self TranscodeJobs, _jobId uint256) constant returns(uint256, bytes32, uint256, address, address, uint256)
func (_TranscodeJobs *TranscodeJobsCaller) GetJob(opts *bind.CallOpts, self TranscodeJobs, _jobId *big.Int) (*big.Int, [32]byte, *big.Int, common.Address, common.Address, *big.Int, error) {
	var (
		ret0 = new(*big.Int)
		ret1 = new([32]byte)
		ret2 = new(*big.Int)
		ret3 = new(common.Address)
		ret4 = new(common.Address)
		ret5 = new(*big.Int)
	)
	out := &[]interface{}{
		ret0,
		ret1,
		ret2,
		ret3,
		ret4,
		ret5,
	}
	err := _TranscodeJobs.contract.Call(opts, out, "getJob", self, _jobId)
	return *ret0, *ret1, *ret2, *ret3, *ret4, *ret5, err
}

// GetJob is a free data retrieval call binding the contract method 0x91a8947d.
//
// Solidity: function getJob(self TranscodeJobs, _jobId uint256) constant returns(uint256, bytes32, uint256, address, address, uint256)
func (_TranscodeJobs *TranscodeJobsSession) GetJob(self TranscodeJobs, _jobId *big.Int) (*big.Int, [32]byte, *big.Int, common.Address, common.Address, *big.Int, error) {
	return _TranscodeJobs.Contract.GetJob(&_TranscodeJobs.CallOpts, self, _jobId)
}

// GetJob is a free data retrieval call binding the contract method 0x91a8947d.
//
// Solidity: function getJob(self TranscodeJobs, _jobId uint256) constant returns(uint256, bytes32, uint256, address, address, uint256)
func (_TranscodeJobs *TranscodeJobsCallerSession) GetJob(self TranscodeJobs, _jobId *big.Int) (*big.Int, [32]byte, *big.Int, common.Address, common.Address, *big.Int, error) {
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

// NewJob is a paid mutator transaction binding the contract method 0xbcbb1294.
//
// Solidity: function newJob(self TranscodeJobs, _streamId string, _transcodingOptions bytes32, _maxPricePerSegment uint256, _electedTranscoder address) returns(bool)
func (_TranscodeJobs *TranscodeJobsTransactor) NewJob(opts *bind.TransactOpts, self TranscodeJobs, _streamId string, _transcodingOptions [32]byte, _maxPricePerSegment *big.Int, _electedTranscoder common.Address) (*types.Transaction, error) {
	return _TranscodeJobs.contract.Transact(opts, "newJob", self, _streamId, _transcodingOptions, _maxPricePerSegment, _electedTranscoder)
}

// NewJob is a paid mutator transaction binding the contract method 0xbcbb1294.
//
// Solidity: function newJob(self TranscodeJobs, _streamId string, _transcodingOptions bytes32, _maxPricePerSegment uint256, _electedTranscoder address) returns(bool)
func (_TranscodeJobs *TranscodeJobsSession) NewJob(self TranscodeJobs, _streamId string, _transcodingOptions [32]byte, _maxPricePerSegment *big.Int, _electedTranscoder common.Address) (*types.Transaction, error) {
	return _TranscodeJobs.Contract.NewJob(&_TranscodeJobs.TransactOpts, self, _streamId, _transcodingOptions, _maxPricePerSegment, _electedTranscoder)
}

// NewJob is a paid mutator transaction binding the contract method 0xbcbb1294.
//
// Solidity: function newJob(self TranscodeJobs, _streamId string, _transcodingOptions bytes32, _maxPricePerSegment uint256, _electedTranscoder address) returns(bool)
func (_TranscodeJobs *TranscodeJobsTransactorSession) NewJob(self TranscodeJobs, _streamId string, _transcodingOptions [32]byte, _maxPricePerSegment *big.Int, _electedTranscoder common.Address) (*types.Transaction, error) {
	return _TranscodeJobs.Contract.NewJob(&_TranscodeJobs.TransactOpts, self, _streamId, _transcodingOptions, _maxPricePerSegment, _electedTranscoder)
}
