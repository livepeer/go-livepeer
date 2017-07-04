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

// TranscoderPoolsABI is the input ABI used to generate the binding from.
const TranscoderPoolsABI = "[{\"constant\":false,\"inputs\":[{\"name\":\"self\",\"type\":\"TranscoderPools.TranscoderPoolsstorage\"},{\"name\":\"_activePoolSize\",\"type\":\"uint256\"},{\"name\":\"_candidatePoolSize\",\"type\":\"uint256\"}],\"name\":\"init\",\"outputs\":[],\"payable\":false,\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"self\",\"type\":\"TranscoderPools.TranscoderPoolsstorage\"},{\"name\":\"_transcoder\",\"type\":\"address\"},{\"name\":\"_amount\",\"type\":\"uint256\"}],\"name\":\"increaseTranscoderStake\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"self\",\"type\":\"TranscoderPools.TranscoderPoolsstorage\"},{\"name\":\"_transcoder\",\"type\":\"address\"}],\"name\":\"isActiveTranscoder\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"self\",\"type\":\"TranscoderPools.TranscoderPoolsstorage\"},{\"name\":\"_transcoder\",\"type\":\"address\"},{\"name\":\"_amount\",\"type\":\"uint256\"}],\"name\":\"addTranscoder\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"self\",\"type\":\"TranscoderPools.TranscoderPoolsstorage\"},{\"name\":\"_transcoder\",\"type\":\"address\"}],\"name\":\"isCandidateTranscoder\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"self\",\"type\":\"TranscoderPools.TranscoderPoolsstorage\"},{\"name\":\"_transcoder\",\"type\":\"address\"}],\"name\":\"transcoderStake\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"self\",\"type\":\"TranscoderPools.TranscoderPoolsstorage\"},{\"name\":\"_transcoder\",\"type\":\"address\"},{\"name\":\"_amount\",\"type\":\"uint256\"}],\"name\":\"decreaseTranscoderStake\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"self\",\"type\":\"TranscoderPools.TranscoderPoolsstorage\"},{\"name\":\"_transcoder\",\"type\":\"address\"}],\"name\":\"isInPools\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"self\",\"type\":\"TranscoderPools.TranscoderPoolsstorage\"},{\"name\":\"_transcoder\",\"type\":\"address\"}],\"name\":\"removeTranscoder\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"type\":\"function\"}]"

// TranscoderPoolsBin is the compiled bytecode used for deploying new contracts.
const TranscoderPoolsBin = `0x6060604052341561000c57fe5b5b6116cf8061001c6000396000f3006060604052361561007d5763ffffffff60e060020a60003504166305f7d09c811461007f5780632267b1d81461009257806360859074146100c0578063a03761bf146100eb578063a1697d4814610119578063b98d2ac314610144578063bb84e5a71461016d578063ecffe6591461019b578063fd6d9e70146101c6575bfe5b6100906004356024356044356101f1565b005b6100ac600435600160a060020a03602435166044356102cc565b604080519115158252519081900360200190f35b6100ac600435600160a060020a03602435166106fc565b604080519115158252519081900360200190f35b6100ac600435600160a060020a0360243516604435610721565b604080519115158252519081900360200190f35b6100ac600435600160a060020a0360243516610d1d565b604080519115158252519081900360200190f35b61015b600435600160a060020a0360243516610d42565b60408051918252519081900360200190f35b6100ac600435600160a060020a0360243516604435610ea0565b604080519115158252519081900360200190f35b6100ac600435600160a060020a0360243516611343565b604080519115158252519081900360200190f35b6100ac600435600160a060020a036024351661138f565b604080519115158252519081900360200190f35b6040805160e260020a63203d5a750281526004810185905260248101849052905173__MinHeap_______________________________916380f569d4916044808301926000929190829003018186803b151561024957fe5b6102c65a03f4151561025757fe5b50506040805160e160020a635983ad2702815260058601600482015260248101849052905173__MaxHeap_______________________________925063b3075a4e91604480820192600092909190829003018186803b15156102b557fe5b6102c65a03f415156102c357fe5b5050505b505050565b600160a060020a0382166000908152600284016020526040812054819081908190819060ff1615610374576040805160e060020a631a8255cd028152600481018a9052600160a060020a038916602482015260448101889052905173__MinHeap_______________________________91631a8255cd916064808301926000929190829003018186803b151561035e57fe5b6102c65a03f4151561036c57fe5b5050506106eb565b600160a060020a038716600090815260078901602052604090205460ff16156106e5576040805160e060020a633d14c42702815260058a016004820152600160a060020a038916602482015260448101889052905173__MaxHeap_______________________________91633d14c427916064808301926000929190829003018186803b151561040057fe5b6102c65a03f4151561040e57fe5b505060408051600090820152805160e260020a633f374e4b02815260058b016004820152815173__MaxHeap_______________________________935063fcdd392c92602480840193919291829003018186803b151561046a57fe5b6102c65a03f4151561047857fe5b505050604051805190602001805190509350935083600160a060020a031687600160a060020a031614156106df5760408051600090820152805160e160020a637073e153028152600481018a9052815173__MinHeap_______________________________9263e0e7c2a69260248082019391829003018186803b15156104fb57fe5b6102c65a03f4151561050957fe5b50506040518051602090910151909350915050808311156106df576040805160e060020a639c943523028152600481018a9052905173__MinHeap_______________________________91639c943523916024808301926000929190829003018186803b151561057557fe5b6102c65a03f4151561058357fe5b50506040805160e160020a6367fe89a5028152600481018b9052600160a060020a038716602482015260448101869052905173__MinHeap_______________________________925063cffd134a91606480820192600092909190829003018186803b15156105ee57fe5b6102c65a03f415156105fc57fe5b5050508760050173__MaxHeap_______________________________63e87f49dc90916040518263ffffffff1660e060020a0281526004018082815260200191505060006040518083038186803b151561065257fe5b6102c65a03f4151561066057fe5b50506040805160e060020a632ace28e702815260058b016004820152600160a060020a038516602482015260448101849052905173__MaxHeap_______________________________9250632ace28e791606480820192600092909190829003018186803b151561035e57fe5b6102c65a03f4151561036c57fe5b5050505b5b6106eb565b60006000fd5b5b600194505b505050509392505050565b600160a060020a038116600090815260028301602052604090205460ff165b92915050565b600160a060020a038216600090815260028401602052604081205481908190819060ff168061076a5750600160a060020a038616600090815260078801602052604090205460ff165b156107755760006000fd5b604080516000602091820152815160e060020a63f98b8071028152600481018a9052915173__MinHeap_______________________________9263f98b8071926024808301939192829003018186803b15156107cd57fe5b6102c65a03f415156107db57fe5b50506040515115159050610866576040805160e160020a6367fe89a502815260048101899052600160a060020a038816602482015260448101879052905173__MinHeap_______________________________9163cffd134a916064808301926000929190829003018186803b151561085057fe5b6102c65a03f4151561085e57fe5b505050610d0c565b60408051600090820152805160e160020a637073e15302815260048101899052815173__MinHeap_______________________________9263e0e7c2a69260248082019391829003018186803b15156108bb57fe5b6102c65a03f415156108c957fe5b5050604051805160209091015190945092505081851115610c11576040805160e060020a639c94352302815260048101899052905173__MinHeap_______________________________91639c943523916024808301926000929190829003018186803b151561093557fe5b6102c65a03f4151561094357fe5b50506040805160e160020a6367fe89a5028152600481018a9052600160a060020a038916602482015260448101889052905173__MinHeap_______________________________925063cffd134a91606480820192600092909190829003018186803b15156109ae57fe5b6102c65a03f415156109bc57fe5b5050604080516000602091820152815160e160020a631a59204502815260058b016004820152915173__MaxHeap_______________________________93506334b2408a926024808201939291829003018186803b1515610a1957fe5b6102c65a03f41515610a2757fe5b505060405151159050610ab3576040805160e060020a632ace28e7028152600589016004820152600160a060020a038516602482015260448101849052905173__MaxHeap_______________________________91632ace28e7916064808301926000929190829003018186803b1515610a9d57fe5b6102c65a03f41515610aab57fe5b505050610c0b565b60408051600090820152805160e260020a633f374e4b028152600589016004820152815173__MaxHeap_______________________________9263fcdd392c9260248082019391829003018186803b1515610b0a57fe5b6102c65a03f41515610b1857fe5b505060405160200151915050808210610c0b576040805160e260020a633a1fd277028152600589016004820152905173__MaxHeap_______________________________9163e87f49dc916024808301926000929190829003018186803b1515610b7e57fe5b6102c65a03f41515610b8c57fe5b50506040805160e060020a632ace28e702815260058a016004820152600160a060020a038616602482015260448101859052905173__MaxHeap_______________________________9250632ace28e791606480820192600092909190829003018186803b151561085057fe5b6102c65a03f4151561085e57fe5b5050505b5b610d0c565b604080516000602091820152815160e260020a631267b25502815260058a016004820152915173__MaxHeap_______________________________9263499ec954926024808301939192829003018186803b1515610c6b57fe5b6102c65a03f41515610c7957fe5b505060405151151590506106e5576040805160e060020a632ace28e7028152600589016004820152600160a060020a038816602482015260448101879052905173__MaxHeap_______________________________91632ace28e7916064808301926000929190829003018186803b151561085057fe5b6102c65a03f4151561085e57fe5b505050610d0c565b60006000fd5b5b5b600193505b5050509392505050565b600160a060020a038116600090815260078301602052604090205460ff165b92915050565b600160a060020a038116600090815260028301602052604081205460ff1615610de957604080516000602091820152815160e160020a63307ddc4702815260048101869052600160a060020a0385166024820152915173__MinHeap_______________________________926360fbb88e926044808301939192829003018186803b1515610dcc57fe5b6102c65a03f41515610dda57fe5b505060405151915061071b9050565b600160a060020a038216600090815260078401602052604090205460ff16156106e557604080516000602091820152815160e260020a630375ff11028152600586016004820152600160a060020a0385166024820152915173__MaxHeap_______________________________92630dd7fc44926044808301939192829003018186803b1515610dcc57fe5b6102c65a03f41515610dda57fe5b505060405151915061071b9050565b60006000fd5b5b5b92915050565b600160a060020a0382166000908152600284016020526040812054819081908190819060ff161561128a576040805160e060020a630c10bda3028152600481018a9052600160a060020a038916602482015260448101889052905173__MinHeap_______________________________91630c10bda3916064808301926000929190829003018186803b1515610f3257fe5b6102c65a03f41515610f4057fe5b505060408051600090820152805160e160020a637073e153028152600481018b9052815173__MinHeap_______________________________935063e0e7c2a692602480840193919291829003018186803b1515610f9a57fe5b6102c65a03f41515610fa857fe5b5050604080518051602091820180516000909152835160e160020a631a59204502815260058e0160048201529351919850965073__MaxHeap_______________________________93506334b2408a926024808201939291829003018186803b151561101057fe5b6102c65a03f4151561101e57fe5b5050604051511590508015611044575083600160a060020a031687600160a060020a0316145b156106df5760408051600090820152805160e260020a633f374e4b02815260058a016004820152815173__MaxHeap_______________________________9263fcdd392c9260248082019391829003018186803b15156110a057fe5b6102c65a03f415156110ae57fe5b50506040518051602090910151909350915050808310156106df576040805160e060020a639c943523028152600481018a9052905173__MinHeap_______________________________91639c943523916024808301926000929190829003018186803b151561111a57fe5b6102c65a03f4151561112857fe5b50506040805160e160020a6367fe89a5028152600481018b9052600160a060020a038516602482015260448101849052905173__MinHeap_______________________________925063cffd134a91606480820192600092909190829003018186803b151561119357fe5b6102c65a03f415156111a157fe5b5050508760050173__MaxHeap_______________________________63e87f49dc90916040518263ffffffff1660e060020a0281526004018082815260200191505060006040518083038186803b15156111f757fe5b6102c65a03f4151561120557fe5b50506040805160e060020a632ace28e702815260058b016004820152600160a060020a038716602482015260448101869052905173__MaxHeap_______________________________9250632ace28e791606480820192600092909190829003018186803b151561035e57fe5b6102c65a03f4151561036c57fe5b5050505b5b6106eb565b600160a060020a038716600090815260078901602052604090205460ff16156106e5576040805160e360020a630c21958902815260058a016004820152600160a060020a038916602482015260448101889052905173__MaxHeap_______________________________9163610cac48916064808301926000929190829003018186803b151561035e57fe5b6102c65a03f4151561036c57fe5b5050506106eb565b60006000fd5b5b600194505b505050509392505050565b600160a060020a038116600090815260028301602052604081205460ff16806113865750600160a060020a038216600090815260078401602052604090205460ff165b90505b92915050565b600160a060020a03811660009081526002830160205260408120548190819060ff16156115f4576040805160e160020a62b0f00902815260048101879052600160a060020a0386166024820152905173__MinHeap_______________________________91630161e012916044808301926000929190829003018186803b151561141557fe5b6102c65a03f4151561142357fe5b5050604080516000602091820152815160e160020a631a592045028152600589016004820152915173__MaxHeap_______________________________93506334b2408a926024808201939291829003018186803b151561148057fe5b6102c65a03f4151561148e57fe5b505060405151151590506115ef5760408051600090820152805160e260020a633f374e4b028152600587016004820152815173__MaxHeap_______________________________9263fcdd392c9260248082019391829003018186803b15156114f357fe5b6102c65a03f4151561150157fe5b5050604080518051602082015160e260020a633a1fd27702835260058a016004840152925190955091935073__MaxHeap_______________________________925063e87f49dc916024808301926000929190829003018186803b151561156457fe5b6102c65a03f4151561157257fe5b50506040805160e160020a6367fe89a502815260048101889052600160a060020a038516602482015260448101849052905173__MinHeap_______________________________925063cffd134a91606480820192600092909190829003018186803b15156115dd57fe5b6102c65a03f415156115eb57fe5b5050505b611695565b600160a060020a038416600090815260078601602052604090205460ff16156106e5576040805160e560020a6306b7c203028152600587016004820152600160a060020a0386166024820152905173__MaxHeap_______________________________9163d6f84060916044808301926000929190829003018186803b15156115dd57fe5b6102c65a03f415156115eb57fe5b505050611695565b60006000fd5b5b600192505b5050929150505600a165627a7a72305820b2bd69b8ba0f4d2e1c2f0e05e360fdf4fbaca829a099cc193b1e45796e0f04210029`

// DeployTranscoderPools deploys a new Ethereum contract, binding an instance of TranscoderPools to it.
func DeployTranscoderPools(auth *bind.TransactOpts, backend bind.ContractBackend, libraries map[string]common.Address) (common.Address, *types.Transaction, *TranscoderPools, error) {
	parsed, err := abi.JSON(strings.NewReader(TranscoderPoolsABI))
	if err != nil {
		return common.Address{}, nil, nil, err
	}

	linkedBin := TranscoderPoolsBin
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
	return address, tx, &TranscoderPools{TranscoderPoolsCaller: TranscoderPoolsCaller{contract: contract}, TranscoderPoolsTransactor: TranscoderPoolsTransactor{contract: contract}}, nil
}

// TranscoderPools is an auto generated Go binding around an Ethereum contract.
type TranscoderPools struct {
	TranscoderPoolsCaller     // Read-only binding to the contract
	TranscoderPoolsTransactor // Write-only binding to the contract
}

// TranscoderPoolsCaller is an auto generated read-only Go binding around an Ethereum contract.
type TranscoderPoolsCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// TranscoderPoolsTransactor is an auto generated write-only Go binding around an Ethereum contract.
type TranscoderPoolsTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// TranscoderPoolsSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type TranscoderPoolsSession struct {
	Contract     *TranscoderPools  // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// TranscoderPoolsCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type TranscoderPoolsCallerSession struct {
	Contract *TranscoderPoolsCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts          // Call options to use throughout this session
}

// TranscoderPoolsTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type TranscoderPoolsTransactorSession struct {
	Contract     *TranscoderPoolsTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts          // Transaction auth options to use throughout this session
}

// TranscoderPoolsRaw is an auto generated low-level Go binding around an Ethereum contract.
type TranscoderPoolsRaw struct {
	Contract *TranscoderPools // Generic contract binding to access the raw methods on
}

// TranscoderPoolsCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type TranscoderPoolsCallerRaw struct {
	Contract *TranscoderPoolsCaller // Generic read-only contract binding to access the raw methods on
}

// TranscoderPoolsTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type TranscoderPoolsTransactorRaw struct {
	Contract *TranscoderPoolsTransactor // Generic write-only contract binding to access the raw methods on
}

// NewTranscoderPools creates a new instance of TranscoderPools, bound to a specific deployed contract.
func NewTranscoderPools(address common.Address, backend bind.ContractBackend) (*TranscoderPools, error) {
	contract, err := bindTranscoderPools(address, backend, backend)
	if err != nil {
		return nil, err
	}
	return &TranscoderPools{TranscoderPoolsCaller: TranscoderPoolsCaller{contract: contract}, TranscoderPoolsTransactor: TranscoderPoolsTransactor{contract: contract}}, nil
}

// NewTranscoderPoolsCaller creates a new read-only instance of TranscoderPools, bound to a specific deployed contract.
func NewTranscoderPoolsCaller(address common.Address, caller bind.ContractCaller) (*TranscoderPoolsCaller, error) {
	contract, err := bindTranscoderPools(address, caller, nil)
	if err != nil {
		return nil, err
	}
	return &TranscoderPoolsCaller{contract: contract}, nil
}

// NewTranscoderPoolsTransactor creates a new write-only instance of TranscoderPools, bound to a specific deployed contract.
func NewTranscoderPoolsTransactor(address common.Address, transactor bind.ContractTransactor) (*TranscoderPoolsTransactor, error) {
	contract, err := bindTranscoderPools(address, nil, transactor)
	if err != nil {
		return nil, err
	}
	return &TranscoderPoolsTransactor{contract: contract}, nil
}

// bindTranscoderPools binds a generic wrapper to an already deployed contract.
func bindTranscoderPools(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor) (*bind.BoundContract, error) {
	parsed, err := abi.JSON(strings.NewReader(TranscoderPoolsABI))
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, parsed, caller, transactor), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_TranscoderPools *TranscoderPoolsRaw) Call(opts *bind.CallOpts, result interface{}, method string, params ...interface{}) error {
	return _TranscoderPools.Contract.TranscoderPoolsCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_TranscoderPools *TranscoderPoolsRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _TranscoderPools.Contract.TranscoderPoolsTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_TranscoderPools *TranscoderPoolsRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _TranscoderPools.Contract.TranscoderPoolsTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_TranscoderPools *TranscoderPoolsCallerRaw) Call(opts *bind.CallOpts, result interface{}, method string, params ...interface{}) error {
	return _TranscoderPools.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_TranscoderPools *TranscoderPoolsTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _TranscoderPools.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_TranscoderPools *TranscoderPoolsTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _TranscoderPools.Contract.contract.Transact(opts, method, params...)
}

// IsActiveTranscoder is a free data retrieval call binding the contract method 0x9568a058.
//
// Solidity: function isActiveTranscoder(self TranscoderPools, _transcoder address) constant returns(bool)
func (_TranscoderPools *TranscoderPoolsCaller) IsActiveTranscoder(opts *bind.CallOpts, self TranscoderPools, _transcoder common.Address) (bool, error) {
	var (
		ret0 = new(bool)
	)
	out := ret0
	err := _TranscoderPools.contract.Call(opts, out, "isActiveTranscoder", self, _transcoder)
	return *ret0, err
}

// IsActiveTranscoder is a free data retrieval call binding the contract method 0x9568a058.
//
// Solidity: function isActiveTranscoder(self TranscoderPools, _transcoder address) constant returns(bool)
func (_TranscoderPools *TranscoderPoolsSession) IsActiveTranscoder(self TranscoderPools, _transcoder common.Address) (bool, error) {
	return _TranscoderPools.Contract.IsActiveTranscoder(&_TranscoderPools.CallOpts, self, _transcoder)
}

// IsActiveTranscoder is a free data retrieval call binding the contract method 0x9568a058.
//
// Solidity: function isActiveTranscoder(self TranscoderPools, _transcoder address) constant returns(bool)
func (_TranscoderPools *TranscoderPoolsCallerSession) IsActiveTranscoder(self TranscoderPools, _transcoder common.Address) (bool, error) {
	return _TranscoderPools.Contract.IsActiveTranscoder(&_TranscoderPools.CallOpts, self, _transcoder)
}

// IsCandidateTranscoder is a free data retrieval call binding the contract method 0x8f054154.
//
// Solidity: function isCandidateTranscoder(self TranscoderPools, _transcoder address) constant returns(bool)
func (_TranscoderPools *TranscoderPoolsCaller) IsCandidateTranscoder(opts *bind.CallOpts, self TranscoderPools, _transcoder common.Address) (bool, error) {
	var (
		ret0 = new(bool)
	)
	out := ret0
	err := _TranscoderPools.contract.Call(opts, out, "isCandidateTranscoder", self, _transcoder)
	return *ret0, err
}

// IsCandidateTranscoder is a free data retrieval call binding the contract method 0x8f054154.
//
// Solidity: function isCandidateTranscoder(self TranscoderPools, _transcoder address) constant returns(bool)
func (_TranscoderPools *TranscoderPoolsSession) IsCandidateTranscoder(self TranscoderPools, _transcoder common.Address) (bool, error) {
	return _TranscoderPools.Contract.IsCandidateTranscoder(&_TranscoderPools.CallOpts, self, _transcoder)
}

// IsCandidateTranscoder is a free data retrieval call binding the contract method 0x8f054154.
//
// Solidity: function isCandidateTranscoder(self TranscoderPools, _transcoder address) constant returns(bool)
func (_TranscoderPools *TranscoderPoolsCallerSession) IsCandidateTranscoder(self TranscoderPools, _transcoder common.Address) (bool, error) {
	return _TranscoderPools.Contract.IsCandidateTranscoder(&_TranscoderPools.CallOpts, self, _transcoder)
}

// IsInPools is a free data retrieval call binding the contract method 0xa5e0aaeb.
//
// Solidity: function isInPools(self TranscoderPools, _transcoder address) constant returns(bool)
func (_TranscoderPools *TranscoderPoolsCaller) IsInPools(opts *bind.CallOpts, self TranscoderPools, _transcoder common.Address) (bool, error) {
	var (
		ret0 = new(bool)
	)
	out := ret0
	err := _TranscoderPools.contract.Call(opts, out, "isInPools", self, _transcoder)
	return *ret0, err
}

// IsInPools is a free data retrieval call binding the contract method 0xa5e0aaeb.
//
// Solidity: function isInPools(self TranscoderPools, _transcoder address) constant returns(bool)
func (_TranscoderPools *TranscoderPoolsSession) IsInPools(self TranscoderPools, _transcoder common.Address) (bool, error) {
	return _TranscoderPools.Contract.IsInPools(&_TranscoderPools.CallOpts, self, _transcoder)
}

// IsInPools is a free data retrieval call binding the contract method 0xa5e0aaeb.
//
// Solidity: function isInPools(self TranscoderPools, _transcoder address) constant returns(bool)
func (_TranscoderPools *TranscoderPoolsCallerSession) IsInPools(self TranscoderPools, _transcoder common.Address) (bool, error) {
	return _TranscoderPools.Contract.IsInPools(&_TranscoderPools.CallOpts, self, _transcoder)
}

// TranscoderStake is a free data retrieval call binding the contract method 0x023be03c.
//
// Solidity: function transcoderStake(self TranscoderPools, _transcoder address) constant returns(uint256)
func (_TranscoderPools *TranscoderPoolsCaller) TranscoderStake(opts *bind.CallOpts, self TranscoderPools, _transcoder common.Address) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _TranscoderPools.contract.Call(opts, out, "transcoderStake", self, _transcoder)
	return *ret0, err
}

// TranscoderStake is a free data retrieval call binding the contract method 0x023be03c.
//
// Solidity: function transcoderStake(self TranscoderPools, _transcoder address) constant returns(uint256)
func (_TranscoderPools *TranscoderPoolsSession) TranscoderStake(self TranscoderPools, _transcoder common.Address) (*big.Int, error) {
	return _TranscoderPools.Contract.TranscoderStake(&_TranscoderPools.CallOpts, self, _transcoder)
}

// TranscoderStake is a free data retrieval call binding the contract method 0x023be03c.
//
// Solidity: function transcoderStake(self TranscoderPools, _transcoder address) constant returns(uint256)
func (_TranscoderPools *TranscoderPoolsCallerSession) TranscoderStake(self TranscoderPools, _transcoder common.Address) (*big.Int, error) {
	return _TranscoderPools.Contract.TranscoderStake(&_TranscoderPools.CallOpts, self, _transcoder)
}

// AddTranscoder is a paid mutator transaction binding the contract method 0x71a3c049.
//
// Solidity: function addTranscoder(self TranscoderPools, _transcoder address, _amount uint256) returns(bool)
func (_TranscoderPools *TranscoderPoolsTransactor) AddTranscoder(opts *bind.TransactOpts, self TranscoderPools, _transcoder common.Address, _amount *big.Int) (*types.Transaction, error) {
	return _TranscoderPools.contract.Transact(opts, "addTranscoder", self, _transcoder, _amount)
}

// AddTranscoder is a paid mutator transaction binding the contract method 0x71a3c049.
//
// Solidity: function addTranscoder(self TranscoderPools, _transcoder address, _amount uint256) returns(bool)
func (_TranscoderPools *TranscoderPoolsSession) AddTranscoder(self TranscoderPools, _transcoder common.Address, _amount *big.Int) (*types.Transaction, error) {
	return _TranscoderPools.Contract.AddTranscoder(&_TranscoderPools.TransactOpts, self, _transcoder, _amount)
}

// AddTranscoder is a paid mutator transaction binding the contract method 0x71a3c049.
//
// Solidity: function addTranscoder(self TranscoderPools, _transcoder address, _amount uint256) returns(bool)
func (_TranscoderPools *TranscoderPoolsTransactorSession) AddTranscoder(self TranscoderPools, _transcoder common.Address, _amount *big.Int) (*types.Transaction, error) {
	return _TranscoderPools.Contract.AddTranscoder(&_TranscoderPools.TransactOpts, self, _transcoder, _amount)
}

// DecreaseTranscoderStake is a paid mutator transaction binding the contract method 0x365da4bf.
//
// Solidity: function decreaseTranscoderStake(self TranscoderPools, _transcoder address, _amount uint256) returns(bool)
func (_TranscoderPools *TranscoderPoolsTransactor) DecreaseTranscoderStake(opts *bind.TransactOpts, self TranscoderPools, _transcoder common.Address, _amount *big.Int) (*types.Transaction, error) {
	return _TranscoderPools.contract.Transact(opts, "decreaseTranscoderStake", self, _transcoder, _amount)
}

// DecreaseTranscoderStake is a paid mutator transaction binding the contract method 0x365da4bf.
//
// Solidity: function decreaseTranscoderStake(self TranscoderPools, _transcoder address, _amount uint256) returns(bool)
func (_TranscoderPools *TranscoderPoolsSession) DecreaseTranscoderStake(self TranscoderPools, _transcoder common.Address, _amount *big.Int) (*types.Transaction, error) {
	return _TranscoderPools.Contract.DecreaseTranscoderStake(&_TranscoderPools.TransactOpts, self, _transcoder, _amount)
}

// DecreaseTranscoderStake is a paid mutator transaction binding the contract method 0x365da4bf.
//
// Solidity: function decreaseTranscoderStake(self TranscoderPools, _transcoder address, _amount uint256) returns(bool)
func (_TranscoderPools *TranscoderPoolsTransactorSession) DecreaseTranscoderStake(self TranscoderPools, _transcoder common.Address, _amount *big.Int) (*types.Transaction, error) {
	return _TranscoderPools.Contract.DecreaseTranscoderStake(&_TranscoderPools.TransactOpts, self, _transcoder, _amount)
}

// IncreaseTranscoderStake is a paid mutator transaction binding the contract method 0x826bf40b.
//
// Solidity: function increaseTranscoderStake(self TranscoderPools, _transcoder address, _amount uint256) returns(bool)
func (_TranscoderPools *TranscoderPoolsTransactor) IncreaseTranscoderStake(opts *bind.TransactOpts, self TranscoderPools, _transcoder common.Address, _amount *big.Int) (*types.Transaction, error) {
	return _TranscoderPools.contract.Transact(opts, "increaseTranscoderStake", self, _transcoder, _amount)
}

// IncreaseTranscoderStake is a paid mutator transaction binding the contract method 0x826bf40b.
//
// Solidity: function increaseTranscoderStake(self TranscoderPools, _transcoder address, _amount uint256) returns(bool)
func (_TranscoderPools *TranscoderPoolsSession) IncreaseTranscoderStake(self TranscoderPools, _transcoder common.Address, _amount *big.Int) (*types.Transaction, error) {
	return _TranscoderPools.Contract.IncreaseTranscoderStake(&_TranscoderPools.TransactOpts, self, _transcoder, _amount)
}

// IncreaseTranscoderStake is a paid mutator transaction binding the contract method 0x826bf40b.
//
// Solidity: function increaseTranscoderStake(self TranscoderPools, _transcoder address, _amount uint256) returns(bool)
func (_TranscoderPools *TranscoderPoolsTransactorSession) IncreaseTranscoderStake(self TranscoderPools, _transcoder common.Address, _amount *big.Int) (*types.Transaction, error) {
	return _TranscoderPools.Contract.IncreaseTranscoderStake(&_TranscoderPools.TransactOpts, self, _transcoder, _amount)
}

// Init is a paid mutator transaction binding the contract method 0x379e1e97.
//
// Solidity: function init(self TranscoderPools, _activePoolSize uint256, _candidatePoolSize uint256) returns()
func (_TranscoderPools *TranscoderPoolsTransactor) Init(opts *bind.TransactOpts, self TranscoderPools, _activePoolSize *big.Int, _candidatePoolSize *big.Int) (*types.Transaction, error) {
	return _TranscoderPools.contract.Transact(opts, "init", self, _activePoolSize, _candidatePoolSize)
}

// Init is a paid mutator transaction binding the contract method 0x379e1e97.
//
// Solidity: function init(self TranscoderPools, _activePoolSize uint256, _candidatePoolSize uint256) returns()
func (_TranscoderPools *TranscoderPoolsSession) Init(self TranscoderPools, _activePoolSize *big.Int, _candidatePoolSize *big.Int) (*types.Transaction, error) {
	return _TranscoderPools.Contract.Init(&_TranscoderPools.TransactOpts, self, _activePoolSize, _candidatePoolSize)
}

// Init is a paid mutator transaction binding the contract method 0x379e1e97.
//
// Solidity: function init(self TranscoderPools, _activePoolSize uint256, _candidatePoolSize uint256) returns()
func (_TranscoderPools *TranscoderPoolsTransactorSession) Init(self TranscoderPools, _activePoolSize *big.Int, _candidatePoolSize *big.Int) (*types.Transaction, error) {
	return _TranscoderPools.Contract.Init(&_TranscoderPools.TransactOpts, self, _activePoolSize, _candidatePoolSize)
}

// RemoveTranscoder is a paid mutator transaction binding the contract method 0xeaa68762.
//
// Solidity: function removeTranscoder(self TranscoderPools, _transcoder address) returns(bool)
func (_TranscoderPools *TranscoderPoolsTransactor) RemoveTranscoder(opts *bind.TransactOpts, self TranscoderPools, _transcoder common.Address) (*types.Transaction, error) {
	return _TranscoderPools.contract.Transact(opts, "removeTranscoder", self, _transcoder)
}

// RemoveTranscoder is a paid mutator transaction binding the contract method 0xeaa68762.
//
// Solidity: function removeTranscoder(self TranscoderPools, _transcoder address) returns(bool)
func (_TranscoderPools *TranscoderPoolsSession) RemoveTranscoder(self TranscoderPools, _transcoder common.Address) (*types.Transaction, error) {
	return _TranscoderPools.Contract.RemoveTranscoder(&_TranscoderPools.TransactOpts, self, _transcoder)
}

// RemoveTranscoder is a paid mutator transaction binding the contract method 0xeaa68762.
//
// Solidity: function removeTranscoder(self TranscoderPools, _transcoder address) returns(bool)
func (_TranscoderPools *TranscoderPoolsTransactorSession) RemoveTranscoder(self TranscoderPools, _transcoder common.Address) (*types.Transaction, error) {
	return _TranscoderPools.Contract.RemoveTranscoder(&_TranscoderPools.TransactOpts, self, _transcoder)
}
