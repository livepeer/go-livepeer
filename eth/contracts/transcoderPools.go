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
const TranscoderPoolsABI = "[{\"constant\":false,\"inputs\":[{\"name\":\"self\",\"type\":\"TranscoderPools.TranscoderPoolsstorage\"},{\"name\":\"_candidatePoolSize\",\"type\":\"uint256\"},{\"name\":\"_reservePoolSize\",\"type\":\"uint256\"}],\"name\":\"init\",\"outputs\":[],\"payable\":false,\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"self\",\"type\":\"TranscoderPools.TranscoderPoolsstorage\"},{\"name\":\"_transcoder\",\"type\":\"address\"},{\"name\":\"_amount\",\"type\":\"uint256\"}],\"name\":\"increaseTranscoderStake\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"self\",\"type\":\"TranscoderPools.TranscoderPoolsstorage\"},{\"name\":\"_transcoder\",\"type\":\"address\"},{\"name\":\"_amount\",\"type\":\"uint256\"}],\"name\":\"addTranscoder\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"self\",\"type\":\"TranscoderPools.TranscoderPoolsstorage\"},{\"name\":\"_transcoder\",\"type\":\"address\"}],\"name\":\"isCandidateTranscoder\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"self\",\"type\":\"TranscoderPools.TranscoderPoolsstorage\"},{\"name\":\"_transcoder\",\"type\":\"address\"}],\"name\":\"transcoderStake\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"self\",\"type\":\"TranscoderPools.TranscoderPoolsstorage\"},{\"name\":\"_transcoder\",\"type\":\"address\"},{\"name\":\"_amount\",\"type\":\"uint256\"}],\"name\":\"decreaseTranscoderStake\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"self\",\"type\":\"TranscoderPools.TranscoderPoolsstorage\"},{\"name\":\"_transcoder\",\"type\":\"address\"}],\"name\":\"isReserveTranscoder\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"self\",\"type\":\"TranscoderPools.TranscoderPoolsstorage\"},{\"name\":\"_transcoder\",\"type\":\"address\"}],\"name\":\"isInPools\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"self\",\"type\":\"TranscoderPools.TranscoderPoolsstorage\"},{\"name\":\"_transcoder\",\"type\":\"address\"}],\"name\":\"removeTranscoder\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"type\":\"function\"}]"

// TranscoderPoolsBin is the compiled bytecode used for deploying new contracts.
const TranscoderPoolsBin = `0x6060604052341561000f57600080fd5b5b6117798061001f6000396000f3006060604052361561007d5763ffffffff60e060020a60003504166305f7d09c81146100825780632267b1d814610095578063a03761bf146100c3578063a1697d48146100f1578063b98d2ac31461011c578063bb84e5a714610145578063c61563d414610173578063ecffe6591461019e578063fd6d9e70146101c9575b600080fd5b6100936004356024356044356101f4565b005b6100af600435600160a060020a03602435166044356102d1565b604051901515815260200160405180910390f35b6100af600435600160a060020a036024351660443561071d565b604051901515815260200160405180910390f35b6100af600435600160a060020a0360243516610d52565b604051901515815260200160405180910390f35b610133600435600160a060020a0360243516610d77565b60405190815260200160405180910390f35b6100af600435600160a060020a0360243516604435610edf565b604051901515815260200160405180910390f35b6100af600435600160a060020a03602435166113ac565b604051901515815260200160405180910390f35b6100af600435600160a060020a03602435166113d1565b604051901515815260200160405180910390f35b6100af600435600160a060020a036024351661141d565b604051901515815260200160405180910390f35b73__MinHeap_______________________________6380f569d4848460405160e060020a63ffffffff85160281526004810192909252602482015260440160006040518083038186803b151561024957600080fd5b6102c65a03f4151561025a57600080fd5b5050508260050173__MaxHeap_______________________________63b3075a4e90918360405160e060020a63ffffffff85160281526004810192909252602482015260440160006040518083038186803b15156102b757600080fd5b6102c65a03f415156102c857600080fd5b5050505b505050565b600160a060020a0382166000908152600284016020526040812054819081908190819060ff161561037b5773__MinHeap_______________________________631a8255cd89898960405160e060020a63ffffffff86160281526004810193909352600160a060020a039091166024830152604482015260640160006040518083038186803b151561036257600080fd5b6102c65a03f4151561037357600080fd5b50505061070c565b600160a060020a038716600090815260078901602052604090205460ff161561007d5773__MaxHeap_______________________________633d14c42760058a01898960405160e060020a63ffffffff86160281526004810193909352600160a060020a039091166024830152604482015260640160006040518083038186803b151561040757600080fd5b6102c65a03f4151561041857600080fd5b5050508760050173__MaxHeap_______________________________63fcdd392c909160006040516040015260405160e060020a63ffffffff84160281526004810191909152602401604080518083038186803b151561047757600080fd5b6102c65a03f4151561048857600080fd5b505050604051805190602001805190509350935083600160a060020a031687600160a060020a031614156107015773__MinHeap_______________________________63e0e7c2a68960006040516040015260405160e060020a63ffffffff84160281526004810191909152602401604080518083038186803b151561050d57600080fd5b6102c65a03f4151561051e57600080fd5b5050506040518051906020018051905091509150808311156107015773__MinHeap_______________________________639c9435238960405160e060020a63ffffffff8416028152600481019190915260240160006040518083038186803b151561058957600080fd5b6102c65a03f4151561059a57600080fd5b5073__MinHeap_______________________________915063cffd134a905089868660405160e060020a63ffffffff86160281526004810193909352600160a060020a039091166024830152604482015260640160006040518083038186803b151561060557600080fd5b6102c65a03f4151561061657600080fd5b5050508760050173__MaxHeap_______________________________63e87f49dc909160405160e060020a63ffffffff8416028152600481019190915260240160006040518083038186803b151561066d57600080fd5b6102c65a03f4151561067e57600080fd5b5050508760050173__MaxHeap_______________________________632ace28e79091848460405160e060020a63ffffffff86160281526004810193909352600160a060020a039091166024830152604482015260640160006040518083038186803b151561036257600080fd5b6102c65a03f4151561037357600080fd5b5050505b5b61070c565b600080fd5b5b600194505b505050509392505050565b600160a060020a038216600090815260028401602052604081205481908190819060ff161580156107695750600160a060020a038616600090815260078801602052604090205460ff16155b151561077457600080fd5b73__MinHeap_______________________________63f98b80718860006040516020015260405160e060020a63ffffffff8416028152600481019190915260240160206040518083038186803b15156107cc57600080fd5b6102c65a03f415156107dd57600080fd5b50505060405180519050151561086c5773__MinHeap_______________________________63cffd134a88888860405160e060020a63ffffffff86160281526004810193909352600160a060020a039091166024830152604482015260640160006040518083038186803b151561085357600080fd5b6102c65a03f4151561086457600080fd5b505050610d41565b73__MinHeap_______________________________63e0e7c2a68860006040516040015260405160e060020a63ffffffff84160281526004810191909152602401604080518083038186803b15156108c357600080fd5b6102c65a03f415156108d457600080fd5b505050604051805190602001805190509250925081851115610c3e5773__MinHeap_______________________________639c9435238860405160e060020a63ffffffff8416028152600481019190915260240160006040518083038186803b151561093f57600080fd5b6102c65a03f4151561095057600080fd5b5073__MinHeap_______________________________915063cffd134a905088888860405160e060020a63ffffffff86160281526004810193909352600160a060020a039091166024830152604482015260640160006040518083038186803b15156109bb57600080fd5b6102c65a03f415156109cc57600080fd5b5050508660050173__MaxHeap_______________________________6334b2408a909160006040516020015260405160e060020a63ffffffff8416028152600481019190915260240160206040518083038186803b1515610a2c57600080fd5b6102c65a03f41515610a3d57600080fd5b5050506040518051905015610ace5773__MaxHeap_______________________________632ace28e760058901858560405160e060020a63ffffffff86160281526004810193909352600160a060020a039091166024830152604482015260640160006040518083038186803b1515610ab557600080fd5b6102c65a03f41515610ac657600080fd5b505050610c38565b73__MaxHeap_______________________________63fcdd392c6005890160006040516040015260405160e060020a63ffffffff84160281526004810191909152602401604080518083038186803b1515610b2857600080fd5b6102c65a03f41515610b3957600080fd5b505050604051805190602001805192505050808210610c385773__MaxHeap_______________________________63e87f49dc6005890160405160e060020a63ffffffff8416028152600481019190915260240160006040518083038186803b1515610ba457600080fd5b6102c65a03f41515610bb557600080fd5b5050508660050173__MaxHeap_______________________________632ace28e79091858560405160e060020a63ffffffff86160281526004810193909352600160a060020a039091166024830152604482015260640160006040518083038186803b151561085357600080fd5b6102c65a03f4151561086457600080fd5b5050505b5b610d41565b73__MaxHeap_______________________________63499ec9546005890160006040516020015260405160e060020a63ffffffff8416028152600481019190915260240160206040518083038186803b1515610c9957600080fd5b6102c65a03f41515610caa57600080fd5b50505060405180519050151561007d5773__MaxHeap_______________________________632ace28e760058901888860405160e060020a63ffffffff86160281526004810193909352600160a060020a039091166024830152604482015260640160006040518083038186803b151561085357600080fd5b6102c65a03f4151561086457600080fd5b505050610d41565b600080fd5b5b5b600193505b5050509392505050565b600160a060020a038116600090815260028301602052604090205460ff165b92915050565b600160a060020a038116600090815260028301602052604081205460ff1615610e235773__MinHeap_______________________________6360fbb88e848460006040516020015260405160e060020a63ffffffff85160281526004810192909252600160a060020a0316602482015260440160206040518083038186803b1515610e0157600080fd5b6102c65a03f41515610e1257600080fd5b505050604051805190509050610d71565b600160a060020a038216600090815260078401602052604090205460ff161561007d5773__MaxHeap_______________________________630dd7fc44600585018460006040516020015260405160e060020a63ffffffff85160281526004810192909252600160a060020a0316602482015260440160206040518083038186803b1515610e0157600080fd5b6102c65a03f41515610e1257600080fd5b505050604051805190509050610d71565b600080fd5b5b5b92915050565b600160a060020a0382166000908152600284016020526040812054819081908190819060ff16156112f15773__MinHeap_______________________________630c10bda389898960405160e060020a63ffffffff86160281526004810193909352600160a060020a039091166024830152604482015260640160006040518083038186803b1515610f7057600080fd5b6102c65a03f41515610f8157600080fd5b5073__MinHeap_______________________________915063e0e7c2a690508960006040516040015260405160e060020a63ffffffff84160281526004810191909152602401604080518083038186803b1515610fdd57600080fd5b6102c65a03f41515610fee57600080fd5b50505060405180519060200180519050935093508760050173__MaxHeap_______________________________6334b2408a909160006040516020015260405160e060020a63ffffffff8416028152600481019190915260240160206040518083038186803b151561105f57600080fd5b6102c65a03f4151561107057600080fd5b50505060405180519050158015611098575083600160a060020a031687600160a060020a0316145b156107015773__MaxHeap_______________________________63fcdd392c60058a0160006040516040015260405160e060020a63ffffffff84160281526004810191909152602401604080518083038186803b15156110f757600080fd5b6102c65a03f4151561110857600080fd5b5050506040518051906020018051905091509150808310156107015773__MinHeap_______________________________639c9435238960405160e060020a63ffffffff8416028152600481019190915260240160006040518083038186803b151561117357600080fd5b6102c65a03f4151561118457600080fd5b5073__MinHeap_______________________________915063cffd134a905089848460405160e060020a63ffffffff86160281526004810193909352600160a060020a039091166024830152604482015260640160006040518083038186803b15156111ef57600080fd5b6102c65a03f4151561120057600080fd5b5050508760050173__MaxHeap_______________________________63e87f49dc909160405160e060020a63ffffffff8416028152600481019190915260240160006040518083038186803b151561125757600080fd5b6102c65a03f4151561126857600080fd5b5050508760050173__MaxHeap_______________________________632ace28e79091868660405160e060020a63ffffffff86160281526004810193909352600160a060020a039091166024830152604482015260640160006040518083038186803b151561036257600080fd5b6102c65a03f4151561037357600080fd5b5050505b5b61070c565b600160a060020a038716600090815260078901602052604090205460ff161561007d5773__MaxHeap_______________________________63610cac4860058a01898960405160e060020a63ffffffff86160281526004810193909352600160a060020a039091166024830152604482015260640160006040518083038186803b151561036257600080fd5b6102c65a03f4151561037357600080fd5b50505061070c565b600080fd5b5b600194505b505050509392505050565b600160a060020a038116600090815260078301602052604090205460ff165b92915050565b600160a060020a038116600090815260028301602052604081205460ff16806114145750600160a060020a038216600090815260078401602052604090205460ff165b90505b92915050565b600160a060020a03811660009081526002830160205260408120548190819060ff161561169d5773__MinHeap_______________________________630161e012868660405160e060020a63ffffffff85160281526004810192909252600160a060020a0316602482015260440160006040518083038186803b15156114a257600080fd5b6102c65a03f415156114b357600080fd5b5050508460050173__MaxHeap_______________________________6334b2408a909160006040516020015260405160e060020a63ffffffff8416028152600481019190915260240160206040518083038186803b151561151357600080fd5b6102c65a03f4151561152457600080fd5b5050506040518051905015156116985773__MaxHeap_______________________________63fcdd392c6005870160006040516040015260405160e060020a63ffffffff84160281526004810191909152602401604080518083038186803b151561158e57600080fd5b6102c65a03f4151561159f57600080fd5b50505060405180519060200180519050915091508460050173__MaxHeap_______________________________63e87f49dc909160405160e060020a63ffffffff8416028152600481019190915260240160006040518083038186803b151561160757600080fd5b6102c65a03f4151561161857600080fd5b5073__MinHeap_______________________________915063cffd134a905086848460405160e060020a63ffffffff86160281526004810193909352600160a060020a039091166024830152604482015260640160006040518083038186803b151561168357600080fd5b6102c65a03f4151561169457600080fd5b5050505b61173f565b600160a060020a038416600090815260078601602052604090205460ff161561007d5773__MaxHeap_______________________________63d6f84060600587018660405160e060020a63ffffffff85160281526004810192909252600160a060020a0316602482015260440160006040518083038186803b151561168357600080fd5b6102c65a03f4151561169457600080fd5b50505061173f565b600080fd5b5b600192505b5050929150505600a165627a7a72305820e9a43cf429c09a89f30e4968afab602b8bc656b4cab01087d2ea72bfc47978700029`

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

// IsReserveTranscoder is a free data retrieval call binding the contract method 0xdf7712e3.
//
// Solidity: function isReserveTranscoder(self TranscoderPools, _transcoder address) constant returns(bool)
func (_TranscoderPools *TranscoderPoolsCaller) IsReserveTranscoder(opts *bind.CallOpts, self TranscoderPools, _transcoder common.Address) (bool, error) {
	var (
		ret0 = new(bool)
	)
	out := ret0
	err := _TranscoderPools.contract.Call(opts, out, "isReserveTranscoder", self, _transcoder)
	return *ret0, err
}

// IsReserveTranscoder is a free data retrieval call binding the contract method 0xdf7712e3.
//
// Solidity: function isReserveTranscoder(self TranscoderPools, _transcoder address) constant returns(bool)
func (_TranscoderPools *TranscoderPoolsSession) IsReserveTranscoder(self TranscoderPools, _transcoder common.Address) (bool, error) {
	return _TranscoderPools.Contract.IsReserveTranscoder(&_TranscoderPools.CallOpts, self, _transcoder)
}

// IsReserveTranscoder is a free data retrieval call binding the contract method 0xdf7712e3.
//
// Solidity: function isReserveTranscoder(self TranscoderPools, _transcoder address) constant returns(bool)
func (_TranscoderPools *TranscoderPoolsCallerSession) IsReserveTranscoder(self TranscoderPools, _transcoder common.Address) (bool, error) {
	return _TranscoderPools.Contract.IsReserveTranscoder(&_TranscoderPools.CallOpts, self, _transcoder)
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
// Solidity: function init(self TranscoderPools, _candidatePoolSize uint256, _reservePoolSize uint256) returns()
func (_TranscoderPools *TranscoderPoolsTransactor) Init(opts *bind.TransactOpts, self TranscoderPools, _candidatePoolSize *big.Int, _reservePoolSize *big.Int) (*types.Transaction, error) {
	return _TranscoderPools.contract.Transact(opts, "init", self, _candidatePoolSize, _reservePoolSize)
}

// Init is a paid mutator transaction binding the contract method 0x379e1e97.
//
// Solidity: function init(self TranscoderPools, _candidatePoolSize uint256, _reservePoolSize uint256) returns()
func (_TranscoderPools *TranscoderPoolsSession) Init(self TranscoderPools, _candidatePoolSize *big.Int, _reservePoolSize *big.Int) (*types.Transaction, error) {
	return _TranscoderPools.Contract.Init(&_TranscoderPools.TransactOpts, self, _candidatePoolSize, _reservePoolSize)
}

// Init is a paid mutator transaction binding the contract method 0x379e1e97.
//
// Solidity: function init(self TranscoderPools, _candidatePoolSize uint256, _reservePoolSize uint256) returns()
func (_TranscoderPools *TranscoderPoolsTransactorSession) Init(self TranscoderPools, _candidatePoolSize *big.Int, _reservePoolSize *big.Int) (*types.Transaction, error) {
	return _TranscoderPools.Contract.Init(&_TranscoderPools.TransactOpts, self, _candidatePoolSize, _reservePoolSize)
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
