// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package contracts

import (
	"strings"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
)

// ServiceRegistryABI is the input ABI used to generate the binding from.
const ServiceRegistryABI = "[{\"constant\":true,\"inputs\":[{\"name\":\"_addr\",\"type\":\"address\"}],\"name\":\"getServiceURI\",\"outputs\":[{\"name\":\"\",\"type\":\"string\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"targetContractId\",\"outputs\":[{\"name\":\"\",\"type\":\"bytes32\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_serviceURI\",\"type\":\"string\"}],\"name\":\"setServiceURI\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_controller\",\"type\":\"address\"}],\"name\":\"setController\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"controller\",\"outputs\":[{\"name\":\"\",\"type\":\"address\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"name\":\"_controller\",\"type\":\"address\"}],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"constructor\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"name\":\"addr\",\"type\":\"address\"},{\"indexed\":false,\"name\":\"serviceURI\",\"type\":\"string\"}],\"name\":\"ServiceURIUpdate\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"name\":\"controller\",\"type\":\"address\"}],\"name\":\"SetController\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"name\":\"param\",\"type\":\"string\"}],\"name\":\"ParameterUpdate\",\"type\":\"event\"}]"

// ServiceRegistry is an auto generated Go binding around an Ethereum contract.
type ServiceRegistry struct {
	ServiceRegistryCaller     // Read-only binding to the contract
	ServiceRegistryTransactor // Write-only binding to the contract
	ServiceRegistryFilterer   // Log filterer for contract events
}

// ServiceRegistryCaller is an auto generated read-only Go binding around an Ethereum contract.
type ServiceRegistryCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// ServiceRegistryTransactor is an auto generated write-only Go binding around an Ethereum contract.
type ServiceRegistryTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// ServiceRegistryFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type ServiceRegistryFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// ServiceRegistrySession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type ServiceRegistrySession struct {
	Contract     *ServiceRegistry  // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// ServiceRegistryCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type ServiceRegistryCallerSession struct {
	Contract *ServiceRegistryCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts          // Call options to use throughout this session
}

// ServiceRegistryTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type ServiceRegistryTransactorSession struct {
	Contract     *ServiceRegistryTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts          // Transaction auth options to use throughout this session
}

// ServiceRegistryRaw is an auto generated low-level Go binding around an Ethereum contract.
type ServiceRegistryRaw struct {
	Contract *ServiceRegistry // Generic contract binding to access the raw methods on
}

// ServiceRegistryCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type ServiceRegistryCallerRaw struct {
	Contract *ServiceRegistryCaller // Generic read-only contract binding to access the raw methods on
}

// ServiceRegistryTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type ServiceRegistryTransactorRaw struct {
	Contract *ServiceRegistryTransactor // Generic write-only contract binding to access the raw methods on
}

// NewServiceRegistry creates a new instance of ServiceRegistry, bound to a specific deployed contract.
func NewServiceRegistry(address common.Address, backend bind.ContractBackend) (*ServiceRegistry, error) {
	contract, err := bindServiceRegistry(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &ServiceRegistry{ServiceRegistryCaller: ServiceRegistryCaller{contract: contract}, ServiceRegistryTransactor: ServiceRegistryTransactor{contract: contract}, ServiceRegistryFilterer: ServiceRegistryFilterer{contract: contract}}, nil
}

// NewServiceRegistryCaller creates a new read-only instance of ServiceRegistry, bound to a specific deployed contract.
func NewServiceRegistryCaller(address common.Address, caller bind.ContractCaller) (*ServiceRegistryCaller, error) {
	contract, err := bindServiceRegistry(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &ServiceRegistryCaller{contract: contract}, nil
}

// NewServiceRegistryTransactor creates a new write-only instance of ServiceRegistry, bound to a specific deployed contract.
func NewServiceRegistryTransactor(address common.Address, transactor bind.ContractTransactor) (*ServiceRegistryTransactor, error) {
	contract, err := bindServiceRegistry(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &ServiceRegistryTransactor{contract: contract}, nil
}

// NewServiceRegistryFilterer creates a new log filterer instance of ServiceRegistry, bound to a specific deployed contract.
func NewServiceRegistryFilterer(address common.Address, filterer bind.ContractFilterer) (*ServiceRegistryFilterer, error) {
	contract, err := bindServiceRegistry(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &ServiceRegistryFilterer{contract: contract}, nil
}

// bindServiceRegistry binds a generic wrapper to an already deployed contract.
func bindServiceRegistry(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := abi.JSON(strings.NewReader(ServiceRegistryABI))
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_ServiceRegistry *ServiceRegistryRaw) Call(opts *bind.CallOpts, result interface{}, method string, params ...interface{}) error {
	return _ServiceRegistry.Contract.ServiceRegistryCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_ServiceRegistry *ServiceRegistryRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _ServiceRegistry.Contract.ServiceRegistryTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_ServiceRegistry *ServiceRegistryRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _ServiceRegistry.Contract.ServiceRegistryTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_ServiceRegistry *ServiceRegistryCallerRaw) Call(opts *bind.CallOpts, result interface{}, method string, params ...interface{}) error {
	return _ServiceRegistry.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_ServiceRegistry *ServiceRegistryTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _ServiceRegistry.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_ServiceRegistry *ServiceRegistryTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _ServiceRegistry.Contract.contract.Transact(opts, method, params...)
}

// Controller is a free data retrieval call binding the contract method 0xf77c4791.
//
// Solidity: function controller() constant returns(address)
func (_ServiceRegistry *ServiceRegistryCaller) Controller(opts *bind.CallOpts) (common.Address, error) {
	var (
		ret0 = new(common.Address)
	)
	out := ret0
	err := _ServiceRegistry.contract.Call(opts, out, "controller")
	return *ret0, err
}

// Controller is a free data retrieval call binding the contract method 0xf77c4791.
//
// Solidity: function controller() constant returns(address)
func (_ServiceRegistry *ServiceRegistrySession) Controller() (common.Address, error) {
	return _ServiceRegistry.Contract.Controller(&_ServiceRegistry.CallOpts)
}

// Controller is a free data retrieval call binding the contract method 0xf77c4791.
//
// Solidity: function controller() constant returns(address)
func (_ServiceRegistry *ServiceRegistryCallerSession) Controller() (common.Address, error) {
	return _ServiceRegistry.Contract.Controller(&_ServiceRegistry.CallOpts)
}

// GetServiceURI is a free data retrieval call binding the contract method 0x214c2a4b.
//
// Solidity: function getServiceURI(_addr address) constant returns(string)
func (_ServiceRegistry *ServiceRegistryCaller) GetServiceURI(opts *bind.CallOpts, _addr common.Address) (string, error) {
	var (
		ret0 = new(string)
	)
	out := ret0
	err := _ServiceRegistry.contract.Call(opts, out, "getServiceURI", _addr)
	return *ret0, err
}

// GetServiceURI is a free data retrieval call binding the contract method 0x214c2a4b.
//
// Solidity: function getServiceURI(_addr address) constant returns(string)
func (_ServiceRegistry *ServiceRegistrySession) GetServiceURI(_addr common.Address) (string, error) {
	return _ServiceRegistry.Contract.GetServiceURI(&_ServiceRegistry.CallOpts, _addr)
}

// GetServiceURI is a free data retrieval call binding the contract method 0x214c2a4b.
//
// Solidity: function getServiceURI(_addr address) constant returns(string)
func (_ServiceRegistry *ServiceRegistryCallerSession) GetServiceURI(_addr common.Address) (string, error) {
	return _ServiceRegistry.Contract.GetServiceURI(&_ServiceRegistry.CallOpts, _addr)
}

// TargetContractId is a free data retrieval call binding the contract method 0x51720b41.
//
// Solidity: function targetContractId() constant returns(bytes32)
func (_ServiceRegistry *ServiceRegistryCaller) TargetContractId(opts *bind.CallOpts) ([32]byte, error) {
	var (
		ret0 = new([32]byte)
	)
	out := ret0
	err := _ServiceRegistry.contract.Call(opts, out, "targetContractId")
	return *ret0, err
}

// TargetContractId is a free data retrieval call binding the contract method 0x51720b41.
//
// Solidity: function targetContractId() constant returns(bytes32)
func (_ServiceRegistry *ServiceRegistrySession) TargetContractId() ([32]byte, error) {
	return _ServiceRegistry.Contract.TargetContractId(&_ServiceRegistry.CallOpts)
}

// TargetContractId is a free data retrieval call binding the contract method 0x51720b41.
//
// Solidity: function targetContractId() constant returns(bytes32)
func (_ServiceRegistry *ServiceRegistryCallerSession) TargetContractId() ([32]byte, error) {
	return _ServiceRegistry.Contract.TargetContractId(&_ServiceRegistry.CallOpts)
}

// SetController is a paid mutator transaction binding the contract method 0x92eefe9b.
//
// Solidity: function setController(_controller address) returns()
func (_ServiceRegistry *ServiceRegistryTransactor) SetController(opts *bind.TransactOpts, _controller common.Address) (*types.Transaction, error) {
	return _ServiceRegistry.contract.Transact(opts, "setController", _controller)
}

// SetController is a paid mutator transaction binding the contract method 0x92eefe9b.
//
// Solidity: function setController(_controller address) returns()
func (_ServiceRegistry *ServiceRegistrySession) SetController(_controller common.Address) (*types.Transaction, error) {
	return _ServiceRegistry.Contract.SetController(&_ServiceRegistry.TransactOpts, _controller)
}

// SetController is a paid mutator transaction binding the contract method 0x92eefe9b.
//
// Solidity: function setController(_controller address) returns()
func (_ServiceRegistry *ServiceRegistryTransactorSession) SetController(_controller common.Address) (*types.Transaction, error) {
	return _ServiceRegistry.Contract.SetController(&_ServiceRegistry.TransactOpts, _controller)
}

// SetServiceURI is a paid mutator transaction binding the contract method 0x5f11301b.
//
// Solidity: function setServiceURI(_serviceURI string) returns()
func (_ServiceRegistry *ServiceRegistryTransactor) SetServiceURI(opts *bind.TransactOpts, _serviceURI string) (*types.Transaction, error) {
	return _ServiceRegistry.contract.Transact(opts, "setServiceURI", _serviceURI)
}

// SetServiceURI is a paid mutator transaction binding the contract method 0x5f11301b.
//
// Solidity: function setServiceURI(_serviceURI string) returns()
func (_ServiceRegistry *ServiceRegistrySession) SetServiceURI(_serviceURI string) (*types.Transaction, error) {
	return _ServiceRegistry.Contract.SetServiceURI(&_ServiceRegistry.TransactOpts, _serviceURI)
}

// SetServiceURI is a paid mutator transaction binding the contract method 0x5f11301b.
//
// Solidity: function setServiceURI(_serviceURI string) returns()
func (_ServiceRegistry *ServiceRegistryTransactorSession) SetServiceURI(_serviceURI string) (*types.Transaction, error) {
	return _ServiceRegistry.Contract.SetServiceURI(&_ServiceRegistry.TransactOpts, _serviceURI)
}

// ServiceRegistryParameterUpdateIterator is returned from FilterParameterUpdate and is used to iterate over the raw logs and unpacked data for ParameterUpdate events raised by the ServiceRegistry contract.
type ServiceRegistryParameterUpdateIterator struct {
	Event *ServiceRegistryParameterUpdate // Event containing the contract specifics and raw log

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
func (it *ServiceRegistryParameterUpdateIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(ServiceRegistryParameterUpdate)
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
		it.Event = new(ServiceRegistryParameterUpdate)
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
func (it *ServiceRegistryParameterUpdateIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *ServiceRegistryParameterUpdateIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// ServiceRegistryParameterUpdate represents a ParameterUpdate event raised by the ServiceRegistry contract.
type ServiceRegistryParameterUpdate struct {
	Param string
	Raw   types.Log // Blockchain specific contextual infos
}

// FilterParameterUpdate is a free log retrieval operation binding the contract event 0x9f5033568d78ae30f29f01e944f97b2216493bd19d1b46d429673acff3dcd674.
//
// Solidity: event ParameterUpdate(param string)
func (_ServiceRegistry *ServiceRegistryFilterer) FilterParameterUpdate(opts *bind.FilterOpts) (*ServiceRegistryParameterUpdateIterator, error) {

	logs, sub, err := _ServiceRegistry.contract.FilterLogs(opts, "ParameterUpdate")
	if err != nil {
		return nil, err
	}
	return &ServiceRegistryParameterUpdateIterator{contract: _ServiceRegistry.contract, event: "ParameterUpdate", logs: logs, sub: sub}, nil
}

// WatchParameterUpdate is a free log subscription operation binding the contract event 0x9f5033568d78ae30f29f01e944f97b2216493bd19d1b46d429673acff3dcd674.
//
// Solidity: event ParameterUpdate(param string)
func (_ServiceRegistry *ServiceRegistryFilterer) WatchParameterUpdate(opts *bind.WatchOpts, sink chan<- *ServiceRegistryParameterUpdate) (event.Subscription, error) {

	logs, sub, err := _ServiceRegistry.contract.WatchLogs(opts, "ParameterUpdate")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(ServiceRegistryParameterUpdate)
				if err := _ServiceRegistry.contract.UnpackLog(event, "ParameterUpdate", log); err != nil {
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

// ServiceRegistryServiceURIUpdateIterator is returned from FilterServiceURIUpdate and is used to iterate over the raw logs and unpacked data for ServiceURIUpdate events raised by the ServiceRegistry contract.
type ServiceRegistryServiceURIUpdateIterator struct {
	Event *ServiceRegistryServiceURIUpdate // Event containing the contract specifics and raw log

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
func (it *ServiceRegistryServiceURIUpdateIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(ServiceRegistryServiceURIUpdate)
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
		it.Event = new(ServiceRegistryServiceURIUpdate)
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
func (it *ServiceRegistryServiceURIUpdateIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *ServiceRegistryServiceURIUpdateIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// ServiceRegistryServiceURIUpdate represents a ServiceURIUpdate event raised by the ServiceRegistry contract.
type ServiceRegistryServiceURIUpdate struct {
	Addr       common.Address
	ServiceURI string
	Raw        types.Log // Blockchain specific contextual infos
}

// FilterServiceURIUpdate is a free log retrieval operation binding the contract event 0xfbb63068732c85741c9f8e61caffaabe038d577bfafd2d2dcc0e352a4f653a4c.
//
// Solidity: event ServiceURIUpdate(addr indexed address, serviceURI string)
func (_ServiceRegistry *ServiceRegistryFilterer) FilterServiceURIUpdate(opts *bind.FilterOpts, addr []common.Address) (*ServiceRegistryServiceURIUpdateIterator, error) {

	var addrRule []interface{}
	for _, addrItem := range addr {
		addrRule = append(addrRule, addrItem)
	}

	logs, sub, err := _ServiceRegistry.contract.FilterLogs(opts, "ServiceURIUpdate", addrRule)
	if err != nil {
		return nil, err
	}
	return &ServiceRegistryServiceURIUpdateIterator{contract: _ServiceRegistry.contract, event: "ServiceURIUpdate", logs: logs, sub: sub}, nil
}

// WatchServiceURIUpdate is a free log subscription operation binding the contract event 0xfbb63068732c85741c9f8e61caffaabe038d577bfafd2d2dcc0e352a4f653a4c.
//
// Solidity: event ServiceURIUpdate(addr indexed address, serviceURI string)
func (_ServiceRegistry *ServiceRegistryFilterer) WatchServiceURIUpdate(opts *bind.WatchOpts, sink chan<- *ServiceRegistryServiceURIUpdate, addr []common.Address) (event.Subscription, error) {

	var addrRule []interface{}
	for _, addrItem := range addr {
		addrRule = append(addrRule, addrItem)
	}

	logs, sub, err := _ServiceRegistry.contract.WatchLogs(opts, "ServiceURIUpdate", addrRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(ServiceRegistryServiceURIUpdate)
				if err := _ServiceRegistry.contract.UnpackLog(event, "ServiceURIUpdate", log); err != nil {
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

// ServiceRegistrySetControllerIterator is returned from FilterSetController and is used to iterate over the raw logs and unpacked data for SetController events raised by the ServiceRegistry contract.
type ServiceRegistrySetControllerIterator struct {
	Event *ServiceRegistrySetController // Event containing the contract specifics and raw log

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
func (it *ServiceRegistrySetControllerIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(ServiceRegistrySetController)
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
		it.Event = new(ServiceRegistrySetController)
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
func (it *ServiceRegistrySetControllerIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *ServiceRegistrySetControllerIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// ServiceRegistrySetController represents a SetController event raised by the ServiceRegistry contract.
type ServiceRegistrySetController struct {
	Controller common.Address
	Raw        types.Log // Blockchain specific contextual infos
}

// FilterSetController is a free log retrieval operation binding the contract event 0x4ff638452bbf33c012645d18ae6f05515ff5f2d1dfb0cece8cbf018c60903f70.
//
// Solidity: event SetController(controller address)
func (_ServiceRegistry *ServiceRegistryFilterer) FilterSetController(opts *bind.FilterOpts) (*ServiceRegistrySetControllerIterator, error) {

	logs, sub, err := _ServiceRegistry.contract.FilterLogs(opts, "SetController")
	if err != nil {
		return nil, err
	}
	return &ServiceRegistrySetControllerIterator{contract: _ServiceRegistry.contract, event: "SetController", logs: logs, sub: sub}, nil
}

// WatchSetController is a free log subscription operation binding the contract event 0x4ff638452bbf33c012645d18ae6f05515ff5f2d1dfb0cece8cbf018c60903f70.
//
// Solidity: event SetController(controller address)
func (_ServiceRegistry *ServiceRegistryFilterer) WatchSetController(opts *bind.WatchOpts, sink chan<- *ServiceRegistrySetController) (event.Subscription, error) {

	logs, sub, err := _ServiceRegistry.contract.WatchLogs(opts, "SetController")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(ServiceRegistrySetController)
				if err := _ServiceRegistry.contract.UnpackLog(event, "SetController", log); err != nil {
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
