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

// PollABI is the input ABI used to generate the binding from.
const PollABI = "[{\"constant\":true,\"inputs\":[],\"name\":\"endBlock\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_endBlock\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"constructor\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"voter\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"choiceID\",\"type\":\"uint256\"}],\"name\":\"Vote\",\"type\":\"event\"},{\"constant\":false,\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_choiceID\",\"type\":\"uint256\"}],\"name\":\"vote\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[],\"name\":\"destroy\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"}]"

// Poll is an auto generated Go binding around an Ethereum contract.
type Poll struct {
	PollCaller     // Read-only binding to the contract
	PollTransactor // Write-only binding to the contract
	PollFilterer   // Log filterer for contract events
}

// PollCaller is an auto generated read-only Go binding around an Ethereum contract.
type PollCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// PollTransactor is an auto generated write-only Go binding around an Ethereum contract.
type PollTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// PollFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type PollFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// PollSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type PollSession struct {
	Contract     *Poll             // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// PollCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type PollCallerSession struct {
	Contract *PollCaller   // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts // Call options to use throughout this session
}

// PollTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type PollTransactorSession struct {
	Contract     *PollTransactor   // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// PollRaw is an auto generated low-level Go binding around an Ethereum contract.
type PollRaw struct {
	Contract *Poll // Generic contract binding to access the raw methods on
}

// PollCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type PollCallerRaw struct {
	Contract *PollCaller // Generic read-only contract binding to access the raw methods on
}

// PollTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type PollTransactorRaw struct {
	Contract *PollTransactor // Generic write-only contract binding to access the raw methods on
}

// NewPoll creates a new instance of Poll, bound to a specific deployed contract.
func NewPoll(address common.Address, backend bind.ContractBackend) (*Poll, error) {
	contract, err := bindPoll(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &Poll{PollCaller: PollCaller{contract: contract}, PollTransactor: PollTransactor{contract: contract}, PollFilterer: PollFilterer{contract: contract}}, nil
}

// NewPollCaller creates a new read-only instance of Poll, bound to a specific deployed contract.
func NewPollCaller(address common.Address, caller bind.ContractCaller) (*PollCaller, error) {
	contract, err := bindPoll(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &PollCaller{contract: contract}, nil
}

// NewPollTransactor creates a new write-only instance of Poll, bound to a specific deployed contract.
func NewPollTransactor(address common.Address, transactor bind.ContractTransactor) (*PollTransactor, error) {
	contract, err := bindPoll(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &PollTransactor{contract: contract}, nil
}

// NewPollFilterer creates a new log filterer instance of Poll, bound to a specific deployed contract.
func NewPollFilterer(address common.Address, filterer bind.ContractFilterer) (*PollFilterer, error) {
	contract, err := bindPoll(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &PollFilterer{contract: contract}, nil
}

// bindPoll binds a generic wrapper to an already deployed contract.
func bindPoll(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := abi.JSON(strings.NewReader(PollABI))
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_Poll *PollRaw) Call(opts *bind.CallOpts, result interface{}, method string, params ...interface{}) error {
	return _Poll.Contract.PollCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_Poll *PollRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Poll.Contract.PollTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_Poll *PollRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _Poll.Contract.PollTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_Poll *PollCallerRaw) Call(opts *bind.CallOpts, result interface{}, method string, params ...interface{}) error {
	return _Poll.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_Poll *PollTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Poll.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_Poll *PollTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _Poll.Contract.contract.Transact(opts, method, params...)
}

// EndBlock is a free data retrieval call binding the contract method 0x083c6323.
//
// Solidity: function endBlock() constant returns(uint256)
func (_Poll *PollCaller) EndBlock(opts *bind.CallOpts) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _Poll.contract.Call(opts, out, "endBlock")
	return *ret0, err
}

// EndBlock is a free data retrieval call binding the contract method 0x083c6323.
//
// Solidity: function endBlock() constant returns(uint256)
func (_Poll *PollSession) EndBlock() (*big.Int, error) {
	return _Poll.Contract.EndBlock(&_Poll.CallOpts)
}

// EndBlock is a free data retrieval call binding the contract method 0x083c6323.
//
// Solidity: function endBlock() constant returns(uint256)
func (_Poll *PollCallerSession) EndBlock() (*big.Int, error) {
	return _Poll.Contract.EndBlock(&_Poll.CallOpts)
}

// Destroy is a paid mutator transaction binding the contract method 0x83197ef0.
//
// Solidity: function destroy() returns()
func (_Poll *PollTransactor) Destroy(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Poll.contract.Transact(opts, "destroy")
}

// Destroy is a paid mutator transaction binding the contract method 0x83197ef0.
//
// Solidity: function destroy() returns()
func (_Poll *PollSession) Destroy() (*types.Transaction, error) {
	return _Poll.Contract.Destroy(&_Poll.TransactOpts)
}

// Destroy is a paid mutator transaction binding the contract method 0x83197ef0.
//
// Solidity: function destroy() returns()
func (_Poll *PollTransactorSession) Destroy() (*types.Transaction, error) {
	return _Poll.Contract.Destroy(&_Poll.TransactOpts)
}

// Vote is a paid mutator transaction binding the contract method 0x0121b93f.
//
// Solidity: function vote(uint256 _choiceID) returns()
func (_Poll *PollTransactor) Vote(opts *bind.TransactOpts, _choiceID *big.Int) (*types.Transaction, error) {
	return _Poll.contract.Transact(opts, "vote", _choiceID)
}

// Vote is a paid mutator transaction binding the contract method 0x0121b93f.
//
// Solidity: function vote(uint256 _choiceID) returns()
func (_Poll *PollSession) Vote(_choiceID *big.Int) (*types.Transaction, error) {
	return _Poll.Contract.Vote(&_Poll.TransactOpts, _choiceID)
}

// Vote is a paid mutator transaction binding the contract method 0x0121b93f.
//
// Solidity: function vote(uint256 _choiceID) returns()
func (_Poll *PollTransactorSession) Vote(_choiceID *big.Int) (*types.Transaction, error) {
	return _Poll.Contract.Vote(&_Poll.TransactOpts, _choiceID)
}

// PollVoteIterator is returned from FilterVote and is used to iterate over the raw logs and unpacked data for Vote events raised by the Poll contract.
type PollVoteIterator struct {
	Event *PollVote // Event containing the contract specifics and raw log

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
func (it *PollVoteIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(PollVote)
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
		it.Event = new(PollVote)
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
func (it *PollVoteIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *PollVoteIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// PollVote represents a Vote event raised by the Poll contract.
type PollVote struct {
	Voter    common.Address
	ChoiceID *big.Int
	Raw      types.Log // Blockchain specific contextual infos
}

// FilterVote is a free log retrieval operation binding the contract event 0xf668ead05c744b9178e571d2edb452e72baf6529c8d72160e64e59b50d865bd0.
//
// Solidity: event Vote(address indexed voter, uint256 choiceID)
func (_Poll *PollFilterer) FilterVote(opts *bind.FilterOpts, voter []common.Address) (*PollVoteIterator, error) {

	var voterRule []interface{}
	for _, voterItem := range voter {
		voterRule = append(voterRule, voterItem)
	}

	logs, sub, err := _Poll.contract.FilterLogs(opts, "Vote", voterRule)
	if err != nil {
		return nil, err
	}
	return &PollVoteIterator{contract: _Poll.contract, event: "Vote", logs: logs, sub: sub}, nil
}

// WatchVote is a free log subscription operation binding the contract event 0xf668ead05c744b9178e571d2edb452e72baf6529c8d72160e64e59b50d865bd0.
//
// Solidity: event Vote(address indexed voter, uint256 choiceID)
func (_Poll *PollFilterer) WatchVote(opts *bind.WatchOpts, sink chan<- *PollVote, voter []common.Address) (event.Subscription, error) {

	var voterRule []interface{}
	for _, voterItem := range voter {
		voterRule = append(voterRule, voterItem)
	}

	logs, sub, err := _Poll.contract.WatchLogs(opts, "Vote", voterRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(PollVote)
				if err := _Poll.contract.UnpackLog(event, "Vote", log); err != nil {
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

// ParseVote is a log parse operation binding the contract event 0xf668ead05c744b9178e571d2edb452e72baf6529c8d72160e64e59b50d865bd0.
//
// Solidity: event Vote(address indexed voter, uint256 choiceID)
func (_Poll *PollFilterer) ParseVote(log types.Log) (*PollVote, error) {
	event := new(PollVote)
	if err := _Poll.contract.UnpackLog(event, "Vote", log); err != nil {
		return nil, err
	}
	return event, nil
}
