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

// LivepeerETHTicketBrokerABI is the input ABI used to generate the binding from.
const LivepeerETHTicketBrokerABI = "[{\"constant\":true,\"inputs\":[{\"name\":\"_sender\",\"type\":\"address\"}],\"name\":\"isUnlockInProgress\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"unlockPeriod\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[],\"name\":\"withdraw\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[],\"name\":\"fundPenaltyEscrow\",\"outputs\":[],\"payable\":true,\"stateMutability\":\"payable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"targetContractId\",\"outputs\":[{\"name\":\"\",\"type\":\"bytes32\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"\",\"type\":\"bytes32\"}],\"name\":\"usedTickets\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_signers\",\"type\":\"address[]\"}],\"name\":\"approveSigners\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_signers\",\"type\":\"address[]\"}],\"name\":\"requestSignersRevocation\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_depositAmount\",\"type\":\"uint256\"},{\"name\":\"_penaltyEscrowAmount\",\"type\":\"uint256\"},{\"name\":\"_signers\",\"type\":\"address[]\"}],\"name\":\"fundAndApproveSigners\",\"outputs\":[],\"payable\":true,\"stateMutability\":\"payable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_recipient\",\"type\":\"address\"},{\"name\":\"_sender\",\"type\":\"address\"},{\"name\":\"_faceValue\",\"type\":\"uint256\"},{\"name\":\"_winProb\",\"type\":\"uint256\"},{\"name\":\"_senderNonce\",\"type\":\"uint256\"},{\"name\":\"_recipientRandHash\",\"type\":\"bytes32\"},{\"name\":\"_auxData\",\"type\":\"bytes\"},{\"name\":\"_sig\",\"type\":\"bytes\"},{\"name\":\"_recipientRand\",\"type\":\"uint256\"}],\"name\":\"redeemWinningTicket\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[],\"name\":\"fundDeposit\",\"outputs\":[],\"payable\":true,\"stateMutability\":\"payable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"signerRevocationPeriod\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"minPenaltyEscrow\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_controller\",\"type\":\"address\"}],\"name\":\"setController\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"\",\"type\":\"address\"}],\"name\":\"senders\",\"outputs\":[{\"name\":\"deposit\",\"type\":\"uint256\"},{\"name\":\"penaltyEscrow\",\"type\":\"uint256\"},{\"name\":\"withdrawBlock\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[],\"name\":\"unlock\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[],\"name\":\"cancelUnlock\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"_sender\",\"type\":\"address\"},{\"name\":\"_signer\",\"type\":\"address\"}],\"name\":\"isApprovedSigner\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"controller\",\"outputs\":[{\"name\":\"\",\"type\":\"address\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"name\":\"_controller\",\"type\":\"address\"},{\"name\":\"_minPenaltyEscrow\",\"type\":\"uint256\"},{\"name\":\"_unlockPeriod\",\"type\":\"uint256\"},{\"name\":\"_signerRevocationPeriod\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"constructor\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"name\":\"sender\",\"type\":\"address\"},{\"indexed\":false,\"name\":\"amount\",\"type\":\"uint256\"}],\"name\":\"DepositFunded\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"name\":\"sender\",\"type\":\"address\"},{\"indexed\":false,\"name\":\"amount\",\"type\":\"uint256\"}],\"name\":\"PenaltyEscrowFunded\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"name\":\"sender\",\"type\":\"address\"},{\"indexed\":false,\"name\":\"approvedSigners\",\"type\":\"address[]\"}],\"name\":\"SignersApproved\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"name\":\"sender\",\"type\":\"address\"},{\"indexed\":false,\"name\":\"signers\",\"type\":\"address[]\"},{\"indexed\":false,\"name\":\"revocationBlock\",\"type\":\"uint256\"}],\"name\":\"SignersRevocationRequested\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"name\":\"sender\",\"type\":\"address\"},{\"indexed\":true,\"name\":\"recipient\",\"type\":\"address\"},{\"indexed\":false,\"name\":\"faceValue\",\"type\":\"uint256\"},{\"indexed\":false,\"name\":\"winProb\",\"type\":\"uint256\"},{\"indexed\":false,\"name\":\"senderNonce\",\"type\":\"uint256\"},{\"indexed\":false,\"name\":\"recipientRand\",\"type\":\"uint256\"},{\"indexed\":false,\"name\":\"auxData\",\"type\":\"bytes\"}],\"name\":\"WinningTicketRedeemed\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"name\":\"sender\",\"type\":\"address\"},{\"indexed\":true,\"name\":\"recipient\",\"type\":\"address\"},{\"indexed\":false,\"name\":\"amount\",\"type\":\"uint256\"}],\"name\":\"WinningTicketTransfer\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"name\":\"sender\",\"type\":\"address\"},{\"indexed\":true,\"name\":\"recipient\",\"type\":\"address\"},{\"indexed\":false,\"name\":\"amount\",\"type\":\"uint256\"}],\"name\":\"PenaltyEscrowSlashed\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"name\":\"sender\",\"type\":\"address\"},{\"indexed\":false,\"name\":\"startBlock\",\"type\":\"uint256\"},{\"indexed\":false,\"name\":\"endBlock\",\"type\":\"uint256\"}],\"name\":\"Unlock\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"name\":\"sender\",\"type\":\"address\"}],\"name\":\"UnlockCancelled\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"name\":\"sender\",\"type\":\"address\"},{\"indexed\":false,\"name\":\"deposit\",\"type\":\"uint256\"},{\"indexed\":false,\"name\":\"penaltyEscrow\",\"type\":\"uint256\"}],\"name\":\"Withdrawal\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"name\":\"controller\",\"type\":\"address\"}],\"name\":\"SetController\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"name\":\"param\",\"type\":\"string\"}],\"name\":\"ParameterUpdate\",\"type\":\"event\"},{\"constant\":false,\"inputs\":[{\"name\":\"_minPenaltyEscrow\",\"type\":\"uint256\"}],\"name\":\"setMinPenaltyEscrow\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_unlockPeriod\",\"type\":\"uint256\"}],\"name\":\"setUnlockPeriod\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_signerRevocationPeriod\",\"type\":\"uint256\"}],\"name\":\"setSignerRevocationPeriod\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"}]"

// LivepeerETHTicketBroker is an auto generated Go binding around an Ethereum contract.
type LivepeerETHTicketBroker struct {
	LivepeerETHTicketBrokerCaller     // Read-only binding to the contract
	LivepeerETHTicketBrokerTransactor // Write-only binding to the contract
	LivepeerETHTicketBrokerFilterer   // Log filterer for contract events
}

// LivepeerETHTicketBrokerCaller is an auto generated read-only Go binding around an Ethereum contract.
type LivepeerETHTicketBrokerCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// LivepeerETHTicketBrokerTransactor is an auto generated write-only Go binding around an Ethereum contract.
type LivepeerETHTicketBrokerTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// LivepeerETHTicketBrokerFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type LivepeerETHTicketBrokerFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// LivepeerETHTicketBrokerSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type LivepeerETHTicketBrokerSession struct {
	Contract     *LivepeerETHTicketBroker // Generic contract binding to set the session for
	CallOpts     bind.CallOpts            // Call options to use throughout this session
	TransactOpts bind.TransactOpts        // Transaction auth options to use throughout this session
}

// LivepeerETHTicketBrokerCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type LivepeerETHTicketBrokerCallerSession struct {
	Contract *LivepeerETHTicketBrokerCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts                  // Call options to use throughout this session
}

// LivepeerETHTicketBrokerTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type LivepeerETHTicketBrokerTransactorSession struct {
	Contract     *LivepeerETHTicketBrokerTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts                  // Transaction auth options to use throughout this session
}

// LivepeerETHTicketBrokerRaw is an auto generated low-level Go binding around an Ethereum contract.
type LivepeerETHTicketBrokerRaw struct {
	Contract *LivepeerETHTicketBroker // Generic contract binding to access the raw methods on
}

// LivepeerETHTicketBrokerCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type LivepeerETHTicketBrokerCallerRaw struct {
	Contract *LivepeerETHTicketBrokerCaller // Generic read-only contract binding to access the raw methods on
}

// LivepeerETHTicketBrokerTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type LivepeerETHTicketBrokerTransactorRaw struct {
	Contract *LivepeerETHTicketBrokerTransactor // Generic write-only contract binding to access the raw methods on
}

// NewLivepeerETHTicketBroker creates a new instance of LivepeerETHTicketBroker, bound to a specific deployed contract.
func NewLivepeerETHTicketBroker(address common.Address, backend bind.ContractBackend) (*LivepeerETHTicketBroker, error) {
	contract, err := bindLivepeerETHTicketBroker(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &LivepeerETHTicketBroker{LivepeerETHTicketBrokerCaller: LivepeerETHTicketBrokerCaller{contract: contract}, LivepeerETHTicketBrokerTransactor: LivepeerETHTicketBrokerTransactor{contract: contract}, LivepeerETHTicketBrokerFilterer: LivepeerETHTicketBrokerFilterer{contract: contract}}, nil
}

// NewLivepeerETHTicketBrokerCaller creates a new read-only instance of LivepeerETHTicketBroker, bound to a specific deployed contract.
func NewLivepeerETHTicketBrokerCaller(address common.Address, caller bind.ContractCaller) (*LivepeerETHTicketBrokerCaller, error) {
	contract, err := bindLivepeerETHTicketBroker(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &LivepeerETHTicketBrokerCaller{contract: contract}, nil
}

// NewLivepeerETHTicketBrokerTransactor creates a new write-only instance of LivepeerETHTicketBroker, bound to a specific deployed contract.
func NewLivepeerETHTicketBrokerTransactor(address common.Address, transactor bind.ContractTransactor) (*LivepeerETHTicketBrokerTransactor, error) {
	contract, err := bindLivepeerETHTicketBroker(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &LivepeerETHTicketBrokerTransactor{contract: contract}, nil
}

// NewLivepeerETHTicketBrokerFilterer creates a new log filterer instance of LivepeerETHTicketBroker, bound to a specific deployed contract.
func NewLivepeerETHTicketBrokerFilterer(address common.Address, filterer bind.ContractFilterer) (*LivepeerETHTicketBrokerFilterer, error) {
	contract, err := bindLivepeerETHTicketBroker(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &LivepeerETHTicketBrokerFilterer{contract: contract}, nil
}

// bindLivepeerETHTicketBroker binds a generic wrapper to an already deployed contract.
func bindLivepeerETHTicketBroker(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := abi.JSON(strings.NewReader(LivepeerETHTicketBrokerABI))
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_LivepeerETHTicketBroker *LivepeerETHTicketBrokerRaw) Call(opts *bind.CallOpts, result interface{}, method string, params ...interface{}) error {
	return _LivepeerETHTicketBroker.Contract.LivepeerETHTicketBrokerCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_LivepeerETHTicketBroker *LivepeerETHTicketBrokerRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _LivepeerETHTicketBroker.Contract.LivepeerETHTicketBrokerTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_LivepeerETHTicketBroker *LivepeerETHTicketBrokerRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _LivepeerETHTicketBroker.Contract.LivepeerETHTicketBrokerTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_LivepeerETHTicketBroker *LivepeerETHTicketBrokerCallerRaw) Call(opts *bind.CallOpts, result interface{}, method string, params ...interface{}) error {
	return _LivepeerETHTicketBroker.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_LivepeerETHTicketBroker *LivepeerETHTicketBrokerTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _LivepeerETHTicketBroker.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_LivepeerETHTicketBroker *LivepeerETHTicketBrokerTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _LivepeerETHTicketBroker.Contract.contract.Transact(opts, method, params...)
}

// Controller is a free data retrieval call binding the contract method 0xf77c4791.
//
// Solidity: function controller() constant returns(address)
func (_LivepeerETHTicketBroker *LivepeerETHTicketBrokerCaller) Controller(opts *bind.CallOpts) (common.Address, error) {
	var (
		ret0 = new(common.Address)
	)
	out := ret0
	err := _LivepeerETHTicketBroker.contract.Call(opts, out, "controller")
	return *ret0, err
}

// Controller is a free data retrieval call binding the contract method 0xf77c4791.
//
// Solidity: function controller() constant returns(address)
func (_LivepeerETHTicketBroker *LivepeerETHTicketBrokerSession) Controller() (common.Address, error) {
	return _LivepeerETHTicketBroker.Contract.Controller(&_LivepeerETHTicketBroker.CallOpts)
}

// Controller is a free data retrieval call binding the contract method 0xf77c4791.
//
// Solidity: function controller() constant returns(address)
func (_LivepeerETHTicketBroker *LivepeerETHTicketBrokerCallerSession) Controller() (common.Address, error) {
	return _LivepeerETHTicketBroker.Contract.Controller(&_LivepeerETHTicketBroker.CallOpts)
}

// IsApprovedSigner is a free data retrieval call binding the contract method 0xd6904109.
//
// Solidity: function isApprovedSigner(address _sender, address _signer) constant returns(bool)
func (_LivepeerETHTicketBroker *LivepeerETHTicketBrokerCaller) IsApprovedSigner(opts *bind.CallOpts, _sender common.Address, _signer common.Address) (bool, error) {
	var (
		ret0 = new(bool)
	)
	out := ret0
	err := _LivepeerETHTicketBroker.contract.Call(opts, out, "isApprovedSigner", _sender, _signer)
	return *ret0, err
}

// IsApprovedSigner is a free data retrieval call binding the contract method 0xd6904109.
//
// Solidity: function isApprovedSigner(address _sender, address _signer) constant returns(bool)
func (_LivepeerETHTicketBroker *LivepeerETHTicketBrokerSession) IsApprovedSigner(_sender common.Address, _signer common.Address) (bool, error) {
	return _LivepeerETHTicketBroker.Contract.IsApprovedSigner(&_LivepeerETHTicketBroker.CallOpts, _sender, _signer)
}

// IsApprovedSigner is a free data retrieval call binding the contract method 0xd6904109.
//
// Solidity: function isApprovedSigner(address _sender, address _signer) constant returns(bool)
func (_LivepeerETHTicketBroker *LivepeerETHTicketBrokerCallerSession) IsApprovedSigner(_sender common.Address, _signer common.Address) (bool, error) {
	return _LivepeerETHTicketBroker.Contract.IsApprovedSigner(&_LivepeerETHTicketBroker.CallOpts, _sender, _signer)
}

// IsUnlockInProgress is a free data retrieval call binding the contract method 0x121cdcc2.
//
// Solidity: function isUnlockInProgress(address _sender) constant returns(bool)
func (_LivepeerETHTicketBroker *LivepeerETHTicketBrokerCaller) IsUnlockInProgress(opts *bind.CallOpts, _sender common.Address) (bool, error) {
	var (
		ret0 = new(bool)
	)
	out := ret0
	err := _LivepeerETHTicketBroker.contract.Call(opts, out, "isUnlockInProgress", _sender)
	return *ret0, err
}

// IsUnlockInProgress is a free data retrieval call binding the contract method 0x121cdcc2.
//
// Solidity: function isUnlockInProgress(address _sender) constant returns(bool)
func (_LivepeerETHTicketBroker *LivepeerETHTicketBrokerSession) IsUnlockInProgress(_sender common.Address) (bool, error) {
	return _LivepeerETHTicketBroker.Contract.IsUnlockInProgress(&_LivepeerETHTicketBroker.CallOpts, _sender)
}

// IsUnlockInProgress is a free data retrieval call binding the contract method 0x121cdcc2.
//
// Solidity: function isUnlockInProgress(address _sender) constant returns(bool)
func (_LivepeerETHTicketBroker *LivepeerETHTicketBrokerCallerSession) IsUnlockInProgress(_sender common.Address) (bool, error) {
	return _LivepeerETHTicketBroker.Contract.IsUnlockInProgress(&_LivepeerETHTicketBroker.CallOpts, _sender)
}

// MinPenaltyEscrow is a free data retrieval call binding the contract method 0x713a5cc2.
//
// Solidity: function minPenaltyEscrow() constant returns(uint256)
func (_LivepeerETHTicketBroker *LivepeerETHTicketBrokerCaller) MinPenaltyEscrow(opts *bind.CallOpts) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _LivepeerETHTicketBroker.contract.Call(opts, out, "minPenaltyEscrow")
	return *ret0, err
}

// MinPenaltyEscrow is a free data retrieval call binding the contract method 0x713a5cc2.
//
// Solidity: function minPenaltyEscrow() constant returns(uint256)
func (_LivepeerETHTicketBroker *LivepeerETHTicketBrokerSession) MinPenaltyEscrow() (*big.Int, error) {
	return _LivepeerETHTicketBroker.Contract.MinPenaltyEscrow(&_LivepeerETHTicketBroker.CallOpts)
}

// MinPenaltyEscrow is a free data retrieval call binding the contract method 0x713a5cc2.
//
// Solidity: function minPenaltyEscrow() constant returns(uint256)
func (_LivepeerETHTicketBroker *LivepeerETHTicketBrokerCallerSession) MinPenaltyEscrow() (*big.Int, error) {
	return _LivepeerETHTicketBroker.Contract.MinPenaltyEscrow(&_LivepeerETHTicketBroker.CallOpts)
}

// Senders is a free data retrieval call binding the contract method 0x982fb9d8.
//
// Solidity: function senders(address ) constant returns(uint256 deposit, uint256 penaltyEscrow, uint256 withdrawBlock)
func (_LivepeerETHTicketBroker *LivepeerETHTicketBrokerCaller) Senders(opts *bind.CallOpts, arg0 common.Address) (struct {
	Deposit       *big.Int
	PenaltyEscrow *big.Int
	WithdrawBlock *big.Int
}, error) {
	ret := new(struct {
		Deposit       *big.Int
		PenaltyEscrow *big.Int
		WithdrawBlock *big.Int
	})
	out := ret
	err := _LivepeerETHTicketBroker.contract.Call(opts, out, "senders", arg0)
	return *ret, err
}

// Senders is a free data retrieval call binding the contract method 0x982fb9d8.
//
// Solidity: function senders(address ) constant returns(uint256 deposit, uint256 penaltyEscrow, uint256 withdrawBlock)
func (_LivepeerETHTicketBroker *LivepeerETHTicketBrokerSession) Senders(arg0 common.Address) (struct {
	Deposit       *big.Int
	PenaltyEscrow *big.Int
	WithdrawBlock *big.Int
}, error) {
	return _LivepeerETHTicketBroker.Contract.Senders(&_LivepeerETHTicketBroker.CallOpts, arg0)
}

// Senders is a free data retrieval call binding the contract method 0x982fb9d8.
//
// Solidity: function senders(address ) constant returns(uint256 deposit, uint256 penaltyEscrow, uint256 withdrawBlock)
func (_LivepeerETHTicketBroker *LivepeerETHTicketBrokerCallerSession) Senders(arg0 common.Address) (struct {
	Deposit       *big.Int
	PenaltyEscrow *big.Int
	WithdrawBlock *big.Int
}, error) {
	return _LivepeerETHTicketBroker.Contract.Senders(&_LivepeerETHTicketBroker.CallOpts, arg0)
}

// SignerRevocationPeriod is a free data retrieval call binding the contract method 0x6e2cff2b.
//
// Solidity: function signerRevocationPeriod() constant returns(uint256)
func (_LivepeerETHTicketBroker *LivepeerETHTicketBrokerCaller) SignerRevocationPeriod(opts *bind.CallOpts) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _LivepeerETHTicketBroker.contract.Call(opts, out, "signerRevocationPeriod")
	return *ret0, err
}

// SignerRevocationPeriod is a free data retrieval call binding the contract method 0x6e2cff2b.
//
// Solidity: function signerRevocationPeriod() constant returns(uint256)
func (_LivepeerETHTicketBroker *LivepeerETHTicketBrokerSession) SignerRevocationPeriod() (*big.Int, error) {
	return _LivepeerETHTicketBroker.Contract.SignerRevocationPeriod(&_LivepeerETHTicketBroker.CallOpts)
}

// SignerRevocationPeriod is a free data retrieval call binding the contract method 0x6e2cff2b.
//
// Solidity: function signerRevocationPeriod() constant returns(uint256)
func (_LivepeerETHTicketBroker *LivepeerETHTicketBrokerCallerSession) SignerRevocationPeriod() (*big.Int, error) {
	return _LivepeerETHTicketBroker.Contract.SignerRevocationPeriod(&_LivepeerETHTicketBroker.CallOpts)
}

// TargetContractId is a free data retrieval call binding the contract method 0x51720b41.
//
// Solidity: function targetContractId() constant returns(bytes32)
func (_LivepeerETHTicketBroker *LivepeerETHTicketBrokerCaller) TargetContractId(opts *bind.CallOpts) ([32]byte, error) {
	var (
		ret0 = new([32]byte)
	)
	out := ret0
	err := _LivepeerETHTicketBroker.contract.Call(opts, out, "targetContractId")
	return *ret0, err
}

// TargetContractId is a free data retrieval call binding the contract method 0x51720b41.
//
// Solidity: function targetContractId() constant returns(bytes32)
func (_LivepeerETHTicketBroker *LivepeerETHTicketBrokerSession) TargetContractId() ([32]byte, error) {
	return _LivepeerETHTicketBroker.Contract.TargetContractId(&_LivepeerETHTicketBroker.CallOpts)
}

// TargetContractId is a free data retrieval call binding the contract method 0x51720b41.
//
// Solidity: function targetContractId() constant returns(bytes32)
func (_LivepeerETHTicketBroker *LivepeerETHTicketBrokerCallerSession) TargetContractId() ([32]byte, error) {
	return _LivepeerETHTicketBroker.Contract.TargetContractId(&_LivepeerETHTicketBroker.CallOpts)
}

// UnlockPeriod is a free data retrieval call binding the contract method 0x20d3a0b4.
//
// Solidity: function unlockPeriod() constant returns(uint256)
func (_LivepeerETHTicketBroker *LivepeerETHTicketBrokerCaller) UnlockPeriod(opts *bind.CallOpts) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _LivepeerETHTicketBroker.contract.Call(opts, out, "unlockPeriod")
	return *ret0, err
}

// UnlockPeriod is a free data retrieval call binding the contract method 0x20d3a0b4.
//
// Solidity: function unlockPeriod() constant returns(uint256)
func (_LivepeerETHTicketBroker *LivepeerETHTicketBrokerSession) UnlockPeriod() (*big.Int, error) {
	return _LivepeerETHTicketBroker.Contract.UnlockPeriod(&_LivepeerETHTicketBroker.CallOpts)
}

// UnlockPeriod is a free data retrieval call binding the contract method 0x20d3a0b4.
//
// Solidity: function unlockPeriod() constant returns(uint256)
func (_LivepeerETHTicketBroker *LivepeerETHTicketBrokerCallerSession) UnlockPeriod() (*big.Int, error) {
	return _LivepeerETHTicketBroker.Contract.UnlockPeriod(&_LivepeerETHTicketBroker.CallOpts)
}

// UsedTickets is a free data retrieval call binding the contract method 0x59a515ba.
//
// Solidity: function usedTickets(bytes32 ) constant returns(bool)
func (_LivepeerETHTicketBroker *LivepeerETHTicketBrokerCaller) UsedTickets(opts *bind.CallOpts, arg0 [32]byte) (bool, error) {
	var (
		ret0 = new(bool)
	)
	out := ret0
	err := _LivepeerETHTicketBroker.contract.Call(opts, out, "usedTickets", arg0)
	return *ret0, err
}

// UsedTickets is a free data retrieval call binding the contract method 0x59a515ba.
//
// Solidity: function usedTickets(bytes32 ) constant returns(bool)
func (_LivepeerETHTicketBroker *LivepeerETHTicketBrokerSession) UsedTickets(arg0 [32]byte) (bool, error) {
	return _LivepeerETHTicketBroker.Contract.UsedTickets(&_LivepeerETHTicketBroker.CallOpts, arg0)
}

// UsedTickets is a free data retrieval call binding the contract method 0x59a515ba.
//
// Solidity: function usedTickets(bytes32 ) constant returns(bool)
func (_LivepeerETHTicketBroker *LivepeerETHTicketBrokerCallerSession) UsedTickets(arg0 [32]byte) (bool, error) {
	return _LivepeerETHTicketBroker.Contract.UsedTickets(&_LivepeerETHTicketBroker.CallOpts, arg0)
}

// ApproveSigners is a paid mutator transaction binding the contract method 0x64b57754.
//
// Solidity: function approveSigners(address[] _signers) returns()
func (_LivepeerETHTicketBroker *LivepeerETHTicketBrokerTransactor) ApproveSigners(opts *bind.TransactOpts, _signers []common.Address) (*types.Transaction, error) {
	return _LivepeerETHTicketBroker.contract.Transact(opts, "approveSigners", _signers)
}

// ApproveSigners is a paid mutator transaction binding the contract method 0x64b57754.
//
// Solidity: function approveSigners(address[] _signers) returns()
func (_LivepeerETHTicketBroker *LivepeerETHTicketBrokerSession) ApproveSigners(_signers []common.Address) (*types.Transaction, error) {
	return _LivepeerETHTicketBroker.Contract.ApproveSigners(&_LivepeerETHTicketBroker.TransactOpts, _signers)
}

// ApproveSigners is a paid mutator transaction binding the contract method 0x64b57754.
//
// Solidity: function approveSigners(address[] _signers) returns()
func (_LivepeerETHTicketBroker *LivepeerETHTicketBrokerTransactorSession) ApproveSigners(_signers []common.Address) (*types.Transaction, error) {
	return _LivepeerETHTicketBroker.Contract.ApproveSigners(&_LivepeerETHTicketBroker.TransactOpts, _signers)
}

// CancelUnlock is a paid mutator transaction binding the contract method 0xc2c4c2c8.
//
// Solidity: function cancelUnlock() returns()
func (_LivepeerETHTicketBroker *LivepeerETHTicketBrokerTransactor) CancelUnlock(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _LivepeerETHTicketBroker.contract.Transact(opts, "cancelUnlock")
}

// CancelUnlock is a paid mutator transaction binding the contract method 0xc2c4c2c8.
//
// Solidity: function cancelUnlock() returns()
func (_LivepeerETHTicketBroker *LivepeerETHTicketBrokerSession) CancelUnlock() (*types.Transaction, error) {
	return _LivepeerETHTicketBroker.Contract.CancelUnlock(&_LivepeerETHTicketBroker.TransactOpts)
}

// CancelUnlock is a paid mutator transaction binding the contract method 0xc2c4c2c8.
//
// Solidity: function cancelUnlock() returns()
func (_LivepeerETHTicketBroker *LivepeerETHTicketBrokerTransactorSession) CancelUnlock() (*types.Transaction, error) {
	return _LivepeerETHTicketBroker.Contract.CancelUnlock(&_LivepeerETHTicketBroker.TransactOpts)
}

// FundAndApproveSigners is a paid mutator transaction binding the contract method 0x68e62fa7.
//
// Solidity: function fundAndApproveSigners(uint256 _depositAmount, uint256 _penaltyEscrowAmount, address[] _signers) returns()
func (_LivepeerETHTicketBroker *LivepeerETHTicketBrokerTransactor) FundAndApproveSigners(opts *bind.TransactOpts, _depositAmount *big.Int, _penaltyEscrowAmount *big.Int, _signers []common.Address) (*types.Transaction, error) {
	return _LivepeerETHTicketBroker.contract.Transact(opts, "fundAndApproveSigners", _depositAmount, _penaltyEscrowAmount, _signers)
}

// FundAndApproveSigners is a paid mutator transaction binding the contract method 0x68e62fa7.
//
// Solidity: function fundAndApproveSigners(uint256 _depositAmount, uint256 _penaltyEscrowAmount, address[] _signers) returns()
func (_LivepeerETHTicketBroker *LivepeerETHTicketBrokerSession) FundAndApproveSigners(_depositAmount *big.Int, _penaltyEscrowAmount *big.Int, _signers []common.Address) (*types.Transaction, error) {
	return _LivepeerETHTicketBroker.Contract.FundAndApproveSigners(&_LivepeerETHTicketBroker.TransactOpts, _depositAmount, _penaltyEscrowAmount, _signers)
}

// FundAndApproveSigners is a paid mutator transaction binding the contract method 0x68e62fa7.
//
// Solidity: function fundAndApproveSigners(uint256 _depositAmount, uint256 _penaltyEscrowAmount, address[] _signers) returns()
func (_LivepeerETHTicketBroker *LivepeerETHTicketBrokerTransactorSession) FundAndApproveSigners(_depositAmount *big.Int, _penaltyEscrowAmount *big.Int, _signers []common.Address) (*types.Transaction, error) {
	return _LivepeerETHTicketBroker.Contract.FundAndApproveSigners(&_LivepeerETHTicketBroker.TransactOpts, _depositAmount, _penaltyEscrowAmount, _signers)
}

// FundDeposit is a paid mutator transaction binding the contract method 0x6caa736b.
//
// Solidity: function fundDeposit() returns()
func (_LivepeerETHTicketBroker *LivepeerETHTicketBrokerTransactor) FundDeposit(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _LivepeerETHTicketBroker.contract.Transact(opts, "fundDeposit")
}

// FundDeposit is a paid mutator transaction binding the contract method 0x6caa736b.
//
// Solidity: function fundDeposit() returns()
func (_LivepeerETHTicketBroker *LivepeerETHTicketBrokerSession) FundDeposit() (*types.Transaction, error) {
	return _LivepeerETHTicketBroker.Contract.FundDeposit(&_LivepeerETHTicketBroker.TransactOpts)
}

// FundDeposit is a paid mutator transaction binding the contract method 0x6caa736b.
//
// Solidity: function fundDeposit() returns()
func (_LivepeerETHTicketBroker *LivepeerETHTicketBrokerTransactorSession) FundDeposit() (*types.Transaction, error) {
	return _LivepeerETHTicketBroker.Contract.FundDeposit(&_LivepeerETHTicketBroker.TransactOpts)
}

// FundPenaltyEscrow is a paid mutator transaction binding the contract method 0x45286456.
//
// Solidity: function fundPenaltyEscrow() returns()
func (_LivepeerETHTicketBroker *LivepeerETHTicketBrokerTransactor) FundPenaltyEscrow(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _LivepeerETHTicketBroker.contract.Transact(opts, "fundPenaltyEscrow")
}

// FundPenaltyEscrow is a paid mutator transaction binding the contract method 0x45286456.
//
// Solidity: function fundPenaltyEscrow() returns()
func (_LivepeerETHTicketBroker *LivepeerETHTicketBrokerSession) FundPenaltyEscrow() (*types.Transaction, error) {
	return _LivepeerETHTicketBroker.Contract.FundPenaltyEscrow(&_LivepeerETHTicketBroker.TransactOpts)
}

// FundPenaltyEscrow is a paid mutator transaction binding the contract method 0x45286456.
//
// Solidity: function fundPenaltyEscrow() returns()
func (_LivepeerETHTicketBroker *LivepeerETHTicketBrokerTransactorSession) FundPenaltyEscrow() (*types.Transaction, error) {
	return _LivepeerETHTicketBroker.Contract.FundPenaltyEscrow(&_LivepeerETHTicketBroker.TransactOpts)
}

// RedeemWinningTicket is a paid mutator transaction binding the contract method 0x69ada57a.
//
// Solidity: function redeemWinningTicket(address _recipient, address _sender, uint256 _faceValue, uint256 _winProb, uint256 _senderNonce, bytes32 _recipientRandHash, bytes _auxData, bytes _sig, uint256 _recipientRand) returns()
func (_LivepeerETHTicketBroker *LivepeerETHTicketBrokerTransactor) RedeemWinningTicket(opts *bind.TransactOpts, _recipient common.Address, _sender common.Address, _faceValue *big.Int, _winProb *big.Int, _senderNonce *big.Int, _recipientRandHash [32]byte, _auxData []byte, _sig []byte, _recipientRand *big.Int) (*types.Transaction, error) {
	return _LivepeerETHTicketBroker.contract.Transact(opts, "redeemWinningTicket", _recipient, _sender, _faceValue, _winProb, _senderNonce, _recipientRandHash, _auxData, _sig, _recipientRand)
}

// RedeemWinningTicket is a paid mutator transaction binding the contract method 0x69ada57a.
//
// Solidity: function redeemWinningTicket(address _recipient, address _sender, uint256 _faceValue, uint256 _winProb, uint256 _senderNonce, bytes32 _recipientRandHash, bytes _auxData, bytes _sig, uint256 _recipientRand) returns()
func (_LivepeerETHTicketBroker *LivepeerETHTicketBrokerSession) RedeemWinningTicket(_recipient common.Address, _sender common.Address, _faceValue *big.Int, _winProb *big.Int, _senderNonce *big.Int, _recipientRandHash [32]byte, _auxData []byte, _sig []byte, _recipientRand *big.Int) (*types.Transaction, error) {
	return _LivepeerETHTicketBroker.Contract.RedeemWinningTicket(&_LivepeerETHTicketBroker.TransactOpts, _recipient, _sender, _faceValue, _winProb, _senderNonce, _recipientRandHash, _auxData, _sig, _recipientRand)
}

// RedeemWinningTicket is a paid mutator transaction binding the contract method 0x69ada57a.
//
// Solidity: function redeemWinningTicket(address _recipient, address _sender, uint256 _faceValue, uint256 _winProb, uint256 _senderNonce, bytes32 _recipientRandHash, bytes _auxData, bytes _sig, uint256 _recipientRand) returns()
func (_LivepeerETHTicketBroker *LivepeerETHTicketBrokerTransactorSession) RedeemWinningTicket(_recipient common.Address, _sender common.Address, _faceValue *big.Int, _winProb *big.Int, _senderNonce *big.Int, _recipientRandHash [32]byte, _auxData []byte, _sig []byte, _recipientRand *big.Int) (*types.Transaction, error) {
	return _LivepeerETHTicketBroker.Contract.RedeemWinningTicket(&_LivepeerETHTicketBroker.TransactOpts, _recipient, _sender, _faceValue, _winProb, _senderNonce, _recipientRandHash, _auxData, _sig, _recipientRand)
}

// RequestSignersRevocation is a paid mutator transaction binding the contract method 0x66f6a7a2.
//
// Solidity: function requestSignersRevocation(address[] _signers) returns()
func (_LivepeerETHTicketBroker *LivepeerETHTicketBrokerTransactor) RequestSignersRevocation(opts *bind.TransactOpts, _signers []common.Address) (*types.Transaction, error) {
	return _LivepeerETHTicketBroker.contract.Transact(opts, "requestSignersRevocation", _signers)
}

// RequestSignersRevocation is a paid mutator transaction binding the contract method 0x66f6a7a2.
//
// Solidity: function requestSignersRevocation(address[] _signers) returns()
func (_LivepeerETHTicketBroker *LivepeerETHTicketBrokerSession) RequestSignersRevocation(_signers []common.Address) (*types.Transaction, error) {
	return _LivepeerETHTicketBroker.Contract.RequestSignersRevocation(&_LivepeerETHTicketBroker.TransactOpts, _signers)
}

// RequestSignersRevocation is a paid mutator transaction binding the contract method 0x66f6a7a2.
//
// Solidity: function requestSignersRevocation(address[] _signers) returns()
func (_LivepeerETHTicketBroker *LivepeerETHTicketBrokerTransactorSession) RequestSignersRevocation(_signers []common.Address) (*types.Transaction, error) {
	return _LivepeerETHTicketBroker.Contract.RequestSignersRevocation(&_LivepeerETHTicketBroker.TransactOpts, _signers)
}

// SetController is a paid mutator transaction binding the contract method 0x92eefe9b.
//
// Solidity: function setController(address _controller) returns()
func (_LivepeerETHTicketBroker *LivepeerETHTicketBrokerTransactor) SetController(opts *bind.TransactOpts, _controller common.Address) (*types.Transaction, error) {
	return _LivepeerETHTicketBroker.contract.Transact(opts, "setController", _controller)
}

// SetController is a paid mutator transaction binding the contract method 0x92eefe9b.
//
// Solidity: function setController(address _controller) returns()
func (_LivepeerETHTicketBroker *LivepeerETHTicketBrokerSession) SetController(_controller common.Address) (*types.Transaction, error) {
	return _LivepeerETHTicketBroker.Contract.SetController(&_LivepeerETHTicketBroker.TransactOpts, _controller)
}

// SetController is a paid mutator transaction binding the contract method 0x92eefe9b.
//
// Solidity: function setController(address _controller) returns()
func (_LivepeerETHTicketBroker *LivepeerETHTicketBrokerTransactorSession) SetController(_controller common.Address) (*types.Transaction, error) {
	return _LivepeerETHTicketBroker.Contract.SetController(&_LivepeerETHTicketBroker.TransactOpts, _controller)
}

// SetMinPenaltyEscrow is a paid mutator transaction binding the contract method 0x3fcbd299.
//
// Solidity: function setMinPenaltyEscrow(uint256 _minPenaltyEscrow) returns()
func (_LivepeerETHTicketBroker *LivepeerETHTicketBrokerTransactor) SetMinPenaltyEscrow(opts *bind.TransactOpts, _minPenaltyEscrow *big.Int) (*types.Transaction, error) {
	return _LivepeerETHTicketBroker.contract.Transact(opts, "setMinPenaltyEscrow", _minPenaltyEscrow)
}

// SetMinPenaltyEscrow is a paid mutator transaction binding the contract method 0x3fcbd299.
//
// Solidity: function setMinPenaltyEscrow(uint256 _minPenaltyEscrow) returns()
func (_LivepeerETHTicketBroker *LivepeerETHTicketBrokerSession) SetMinPenaltyEscrow(_minPenaltyEscrow *big.Int) (*types.Transaction, error) {
	return _LivepeerETHTicketBroker.Contract.SetMinPenaltyEscrow(&_LivepeerETHTicketBroker.TransactOpts, _minPenaltyEscrow)
}

// SetMinPenaltyEscrow is a paid mutator transaction binding the contract method 0x3fcbd299.
//
// Solidity: function setMinPenaltyEscrow(uint256 _minPenaltyEscrow) returns()
func (_LivepeerETHTicketBroker *LivepeerETHTicketBrokerTransactorSession) SetMinPenaltyEscrow(_minPenaltyEscrow *big.Int) (*types.Transaction, error) {
	return _LivepeerETHTicketBroker.Contract.SetMinPenaltyEscrow(&_LivepeerETHTicketBroker.TransactOpts, _minPenaltyEscrow)
}

// SetSignerRevocationPeriod is a paid mutator transaction binding the contract method 0xec18a8af.
//
// Solidity: function setSignerRevocationPeriod(uint256 _signerRevocationPeriod) returns()
func (_LivepeerETHTicketBroker *LivepeerETHTicketBrokerTransactor) SetSignerRevocationPeriod(opts *bind.TransactOpts, _signerRevocationPeriod *big.Int) (*types.Transaction, error) {
	return _LivepeerETHTicketBroker.contract.Transact(opts, "setSignerRevocationPeriod", _signerRevocationPeriod)
}

// SetSignerRevocationPeriod is a paid mutator transaction binding the contract method 0xec18a8af.
//
// Solidity: function setSignerRevocationPeriod(uint256 _signerRevocationPeriod) returns()
func (_LivepeerETHTicketBroker *LivepeerETHTicketBrokerSession) SetSignerRevocationPeriod(_signerRevocationPeriod *big.Int) (*types.Transaction, error) {
	return _LivepeerETHTicketBroker.Contract.SetSignerRevocationPeriod(&_LivepeerETHTicketBroker.TransactOpts, _signerRevocationPeriod)
}

// SetSignerRevocationPeriod is a paid mutator transaction binding the contract method 0xec18a8af.
//
// Solidity: function setSignerRevocationPeriod(uint256 _signerRevocationPeriod) returns()
func (_LivepeerETHTicketBroker *LivepeerETHTicketBrokerTransactorSession) SetSignerRevocationPeriod(_signerRevocationPeriod *big.Int) (*types.Transaction, error) {
	return _LivepeerETHTicketBroker.Contract.SetSignerRevocationPeriod(&_LivepeerETHTicketBroker.TransactOpts, _signerRevocationPeriod)
}

// SetUnlockPeriod is a paid mutator transaction binding the contract method 0x3d0ddf84.
//
// Solidity: function setUnlockPeriod(uint256 _unlockPeriod) returns()
func (_LivepeerETHTicketBroker *LivepeerETHTicketBrokerTransactor) SetUnlockPeriod(opts *bind.TransactOpts, _unlockPeriod *big.Int) (*types.Transaction, error) {
	return _LivepeerETHTicketBroker.contract.Transact(opts, "setUnlockPeriod", _unlockPeriod)
}

// SetUnlockPeriod is a paid mutator transaction binding the contract method 0x3d0ddf84.
//
// Solidity: function setUnlockPeriod(uint256 _unlockPeriod) returns()
func (_LivepeerETHTicketBroker *LivepeerETHTicketBrokerSession) SetUnlockPeriod(_unlockPeriod *big.Int) (*types.Transaction, error) {
	return _LivepeerETHTicketBroker.Contract.SetUnlockPeriod(&_LivepeerETHTicketBroker.TransactOpts, _unlockPeriod)
}

// SetUnlockPeriod is a paid mutator transaction binding the contract method 0x3d0ddf84.
//
// Solidity: function setUnlockPeriod(uint256 _unlockPeriod) returns()
func (_LivepeerETHTicketBroker *LivepeerETHTicketBrokerTransactorSession) SetUnlockPeriod(_unlockPeriod *big.Int) (*types.Transaction, error) {
	return _LivepeerETHTicketBroker.Contract.SetUnlockPeriod(&_LivepeerETHTicketBroker.TransactOpts, _unlockPeriod)
}

// Unlock is a paid mutator transaction binding the contract method 0xa69df4b5.
//
// Solidity: function unlock() returns()
func (_LivepeerETHTicketBroker *LivepeerETHTicketBrokerTransactor) Unlock(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _LivepeerETHTicketBroker.contract.Transact(opts, "unlock")
}

// Unlock is a paid mutator transaction binding the contract method 0xa69df4b5.
//
// Solidity: function unlock() returns()
func (_LivepeerETHTicketBroker *LivepeerETHTicketBrokerSession) Unlock() (*types.Transaction, error) {
	return _LivepeerETHTicketBroker.Contract.Unlock(&_LivepeerETHTicketBroker.TransactOpts)
}

// Unlock is a paid mutator transaction binding the contract method 0xa69df4b5.
//
// Solidity: function unlock() returns()
func (_LivepeerETHTicketBroker *LivepeerETHTicketBrokerTransactorSession) Unlock() (*types.Transaction, error) {
	return _LivepeerETHTicketBroker.Contract.Unlock(&_LivepeerETHTicketBroker.TransactOpts)
}

// Withdraw is a paid mutator transaction binding the contract method 0x3ccfd60b.
//
// Solidity: function withdraw() returns()
func (_LivepeerETHTicketBroker *LivepeerETHTicketBrokerTransactor) Withdraw(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _LivepeerETHTicketBroker.contract.Transact(opts, "withdraw")
}

// Withdraw is a paid mutator transaction binding the contract method 0x3ccfd60b.
//
// Solidity: function withdraw() returns()
func (_LivepeerETHTicketBroker *LivepeerETHTicketBrokerSession) Withdraw() (*types.Transaction, error) {
	return _LivepeerETHTicketBroker.Contract.Withdraw(&_LivepeerETHTicketBroker.TransactOpts)
}

// Withdraw is a paid mutator transaction binding the contract method 0x3ccfd60b.
//
// Solidity: function withdraw() returns()
func (_LivepeerETHTicketBroker *LivepeerETHTicketBrokerTransactorSession) Withdraw() (*types.Transaction, error) {
	return _LivepeerETHTicketBroker.Contract.Withdraw(&_LivepeerETHTicketBroker.TransactOpts)
}

// LivepeerETHTicketBrokerDepositFundedIterator is returned from FilterDepositFunded and is used to iterate over the raw logs and unpacked data for DepositFunded events raised by the LivepeerETHTicketBroker contract.
type LivepeerETHTicketBrokerDepositFundedIterator struct {
	Event *LivepeerETHTicketBrokerDepositFunded // Event containing the contract specifics and raw log

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
func (it *LivepeerETHTicketBrokerDepositFundedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(LivepeerETHTicketBrokerDepositFunded)
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
		it.Event = new(LivepeerETHTicketBrokerDepositFunded)
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
func (it *LivepeerETHTicketBrokerDepositFundedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *LivepeerETHTicketBrokerDepositFundedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// LivepeerETHTicketBrokerDepositFunded represents a DepositFunded event raised by the LivepeerETHTicketBroker contract.
type LivepeerETHTicketBrokerDepositFunded struct {
	Sender common.Address
	Amount *big.Int
	Raw    types.Log // Blockchain specific contextual infos
}

// FilterDepositFunded is a free log retrieval operation binding the contract event 0x5159e237d952190e68d5215430f305831be7c9c8776d1377c76679ae4773413f.
//
// Solidity: event DepositFunded(address indexed sender, uint256 amount)
func (_LivepeerETHTicketBroker *LivepeerETHTicketBrokerFilterer) FilterDepositFunded(opts *bind.FilterOpts, sender []common.Address) (*LivepeerETHTicketBrokerDepositFundedIterator, error) {

	var senderRule []interface{}
	for _, senderItem := range sender {
		senderRule = append(senderRule, senderItem)
	}

	logs, sub, err := _LivepeerETHTicketBroker.contract.FilterLogs(opts, "DepositFunded", senderRule)
	if err != nil {
		return nil, err
	}
	return &LivepeerETHTicketBrokerDepositFundedIterator{contract: _LivepeerETHTicketBroker.contract, event: "DepositFunded", logs: logs, sub: sub}, nil
}

// WatchDepositFunded is a free log subscription operation binding the contract event 0x5159e237d952190e68d5215430f305831be7c9c8776d1377c76679ae4773413f.
//
// Solidity: event DepositFunded(address indexed sender, uint256 amount)
func (_LivepeerETHTicketBroker *LivepeerETHTicketBrokerFilterer) WatchDepositFunded(opts *bind.WatchOpts, sink chan<- *LivepeerETHTicketBrokerDepositFunded, sender []common.Address) (event.Subscription, error) {

	var senderRule []interface{}
	for _, senderItem := range sender {
		senderRule = append(senderRule, senderItem)
	}

	logs, sub, err := _LivepeerETHTicketBroker.contract.WatchLogs(opts, "DepositFunded", senderRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(LivepeerETHTicketBrokerDepositFunded)
				if err := _LivepeerETHTicketBroker.contract.UnpackLog(event, "DepositFunded", log); err != nil {
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

// LivepeerETHTicketBrokerParameterUpdateIterator is returned from FilterParameterUpdate and is used to iterate over the raw logs and unpacked data for ParameterUpdate events raised by the LivepeerETHTicketBroker contract.
type LivepeerETHTicketBrokerParameterUpdateIterator struct {
	Event *LivepeerETHTicketBrokerParameterUpdate // Event containing the contract specifics and raw log

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
func (it *LivepeerETHTicketBrokerParameterUpdateIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(LivepeerETHTicketBrokerParameterUpdate)
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
		it.Event = new(LivepeerETHTicketBrokerParameterUpdate)
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
func (it *LivepeerETHTicketBrokerParameterUpdateIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *LivepeerETHTicketBrokerParameterUpdateIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// LivepeerETHTicketBrokerParameterUpdate represents a ParameterUpdate event raised by the LivepeerETHTicketBroker contract.
type LivepeerETHTicketBrokerParameterUpdate struct {
	Param string
	Raw   types.Log // Blockchain specific contextual infos
}

// FilterParameterUpdate is a free log retrieval operation binding the contract event 0x9f5033568d78ae30f29f01e944f97b2216493bd19d1b46d429673acff3dcd674.
//
// Solidity: event ParameterUpdate(string param)
func (_LivepeerETHTicketBroker *LivepeerETHTicketBrokerFilterer) FilterParameterUpdate(opts *bind.FilterOpts) (*LivepeerETHTicketBrokerParameterUpdateIterator, error) {

	logs, sub, err := _LivepeerETHTicketBroker.contract.FilterLogs(opts, "ParameterUpdate")
	if err != nil {
		return nil, err
	}
	return &LivepeerETHTicketBrokerParameterUpdateIterator{contract: _LivepeerETHTicketBroker.contract, event: "ParameterUpdate", logs: logs, sub: sub}, nil
}

// WatchParameterUpdate is a free log subscription operation binding the contract event 0x9f5033568d78ae30f29f01e944f97b2216493bd19d1b46d429673acff3dcd674.
//
// Solidity: event ParameterUpdate(string param)
func (_LivepeerETHTicketBroker *LivepeerETHTicketBrokerFilterer) WatchParameterUpdate(opts *bind.WatchOpts, sink chan<- *LivepeerETHTicketBrokerParameterUpdate) (event.Subscription, error) {

	logs, sub, err := _LivepeerETHTicketBroker.contract.WatchLogs(opts, "ParameterUpdate")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(LivepeerETHTicketBrokerParameterUpdate)
				if err := _LivepeerETHTicketBroker.contract.UnpackLog(event, "ParameterUpdate", log); err != nil {
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

// LivepeerETHTicketBrokerPenaltyEscrowFundedIterator is returned from FilterPenaltyEscrowFunded and is used to iterate over the raw logs and unpacked data for PenaltyEscrowFunded events raised by the LivepeerETHTicketBroker contract.
type LivepeerETHTicketBrokerPenaltyEscrowFundedIterator struct {
	Event *LivepeerETHTicketBrokerPenaltyEscrowFunded // Event containing the contract specifics and raw log

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
func (it *LivepeerETHTicketBrokerPenaltyEscrowFundedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(LivepeerETHTicketBrokerPenaltyEscrowFunded)
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
		it.Event = new(LivepeerETHTicketBrokerPenaltyEscrowFunded)
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
func (it *LivepeerETHTicketBrokerPenaltyEscrowFundedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *LivepeerETHTicketBrokerPenaltyEscrowFundedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// LivepeerETHTicketBrokerPenaltyEscrowFunded represents a PenaltyEscrowFunded event raised by the LivepeerETHTicketBroker contract.
type LivepeerETHTicketBrokerPenaltyEscrowFunded struct {
	Sender common.Address
	Amount *big.Int
	Raw    types.Log // Blockchain specific contextual infos
}

// FilterPenaltyEscrowFunded is a free log retrieval operation binding the contract event 0xd679017ec79e2a2859033228f61da78d4793646aa3833e0b2fab88bae82f9ad3.
//
// Solidity: event PenaltyEscrowFunded(address indexed sender, uint256 amount)
func (_LivepeerETHTicketBroker *LivepeerETHTicketBrokerFilterer) FilterPenaltyEscrowFunded(opts *bind.FilterOpts, sender []common.Address) (*LivepeerETHTicketBrokerPenaltyEscrowFundedIterator, error) {

	var senderRule []interface{}
	for _, senderItem := range sender {
		senderRule = append(senderRule, senderItem)
	}

	logs, sub, err := _LivepeerETHTicketBroker.contract.FilterLogs(opts, "PenaltyEscrowFunded", senderRule)
	if err != nil {
		return nil, err
	}
	return &LivepeerETHTicketBrokerPenaltyEscrowFundedIterator{contract: _LivepeerETHTicketBroker.contract, event: "PenaltyEscrowFunded", logs: logs, sub: sub}, nil
}

// WatchPenaltyEscrowFunded is a free log subscription operation binding the contract event 0xd679017ec79e2a2859033228f61da78d4793646aa3833e0b2fab88bae82f9ad3.
//
// Solidity: event PenaltyEscrowFunded(address indexed sender, uint256 amount)
func (_LivepeerETHTicketBroker *LivepeerETHTicketBrokerFilterer) WatchPenaltyEscrowFunded(opts *bind.WatchOpts, sink chan<- *LivepeerETHTicketBrokerPenaltyEscrowFunded, sender []common.Address) (event.Subscription, error) {

	var senderRule []interface{}
	for _, senderItem := range sender {
		senderRule = append(senderRule, senderItem)
	}

	logs, sub, err := _LivepeerETHTicketBroker.contract.WatchLogs(opts, "PenaltyEscrowFunded", senderRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(LivepeerETHTicketBrokerPenaltyEscrowFunded)
				if err := _LivepeerETHTicketBroker.contract.UnpackLog(event, "PenaltyEscrowFunded", log); err != nil {
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

// LivepeerETHTicketBrokerPenaltyEscrowSlashedIterator is returned from FilterPenaltyEscrowSlashed and is used to iterate over the raw logs and unpacked data for PenaltyEscrowSlashed events raised by the LivepeerETHTicketBroker contract.
type LivepeerETHTicketBrokerPenaltyEscrowSlashedIterator struct {
	Event *LivepeerETHTicketBrokerPenaltyEscrowSlashed // Event containing the contract specifics and raw log

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
func (it *LivepeerETHTicketBrokerPenaltyEscrowSlashedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(LivepeerETHTicketBrokerPenaltyEscrowSlashed)
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
		it.Event = new(LivepeerETHTicketBrokerPenaltyEscrowSlashed)
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
func (it *LivepeerETHTicketBrokerPenaltyEscrowSlashedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *LivepeerETHTicketBrokerPenaltyEscrowSlashedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// LivepeerETHTicketBrokerPenaltyEscrowSlashed represents a PenaltyEscrowSlashed event raised by the LivepeerETHTicketBroker contract.
type LivepeerETHTicketBrokerPenaltyEscrowSlashed struct {
	Sender    common.Address
	Recipient common.Address
	Amount    *big.Int
	Raw       types.Log // Blockchain specific contextual infos
}

// FilterPenaltyEscrowSlashed is a free log retrieval operation binding the contract event 0xebf82fdc1b74684102a88965407eff69eeef94329b0d5c935ab383efc81b971e.
//
// Solidity: event PenaltyEscrowSlashed(address indexed sender, address indexed recipient, uint256 amount)
func (_LivepeerETHTicketBroker *LivepeerETHTicketBrokerFilterer) FilterPenaltyEscrowSlashed(opts *bind.FilterOpts, sender []common.Address, recipient []common.Address) (*LivepeerETHTicketBrokerPenaltyEscrowSlashedIterator, error) {

	var senderRule []interface{}
	for _, senderItem := range sender {
		senderRule = append(senderRule, senderItem)
	}
	var recipientRule []interface{}
	for _, recipientItem := range recipient {
		recipientRule = append(recipientRule, recipientItem)
	}

	logs, sub, err := _LivepeerETHTicketBroker.contract.FilterLogs(opts, "PenaltyEscrowSlashed", senderRule, recipientRule)
	if err != nil {
		return nil, err
	}
	return &LivepeerETHTicketBrokerPenaltyEscrowSlashedIterator{contract: _LivepeerETHTicketBroker.contract, event: "PenaltyEscrowSlashed", logs: logs, sub: sub}, nil
}

// WatchPenaltyEscrowSlashed is a free log subscription operation binding the contract event 0xebf82fdc1b74684102a88965407eff69eeef94329b0d5c935ab383efc81b971e.
//
// Solidity: event PenaltyEscrowSlashed(address indexed sender, address indexed recipient, uint256 amount)
func (_LivepeerETHTicketBroker *LivepeerETHTicketBrokerFilterer) WatchPenaltyEscrowSlashed(opts *bind.WatchOpts, sink chan<- *LivepeerETHTicketBrokerPenaltyEscrowSlashed, sender []common.Address, recipient []common.Address) (event.Subscription, error) {

	var senderRule []interface{}
	for _, senderItem := range sender {
		senderRule = append(senderRule, senderItem)
	}
	var recipientRule []interface{}
	for _, recipientItem := range recipient {
		recipientRule = append(recipientRule, recipientItem)
	}

	logs, sub, err := _LivepeerETHTicketBroker.contract.WatchLogs(opts, "PenaltyEscrowSlashed", senderRule, recipientRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(LivepeerETHTicketBrokerPenaltyEscrowSlashed)
				if err := _LivepeerETHTicketBroker.contract.UnpackLog(event, "PenaltyEscrowSlashed", log); err != nil {
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

// LivepeerETHTicketBrokerSetControllerIterator is returned from FilterSetController and is used to iterate over the raw logs and unpacked data for SetController events raised by the LivepeerETHTicketBroker contract.
type LivepeerETHTicketBrokerSetControllerIterator struct {
	Event *LivepeerETHTicketBrokerSetController // Event containing the contract specifics and raw log

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
func (it *LivepeerETHTicketBrokerSetControllerIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(LivepeerETHTicketBrokerSetController)
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
		it.Event = new(LivepeerETHTicketBrokerSetController)
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
func (it *LivepeerETHTicketBrokerSetControllerIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *LivepeerETHTicketBrokerSetControllerIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// LivepeerETHTicketBrokerSetController represents a SetController event raised by the LivepeerETHTicketBroker contract.
type LivepeerETHTicketBrokerSetController struct {
	Controller common.Address
	Raw        types.Log // Blockchain specific contextual infos
}

// FilterSetController is a free log retrieval operation binding the contract event 0x4ff638452bbf33c012645d18ae6f05515ff5f2d1dfb0cece8cbf018c60903f70.
//
// Solidity: event SetController(address controller)
func (_LivepeerETHTicketBroker *LivepeerETHTicketBrokerFilterer) FilterSetController(opts *bind.FilterOpts) (*LivepeerETHTicketBrokerSetControllerIterator, error) {

	logs, sub, err := _LivepeerETHTicketBroker.contract.FilterLogs(opts, "SetController")
	if err != nil {
		return nil, err
	}
	return &LivepeerETHTicketBrokerSetControllerIterator{contract: _LivepeerETHTicketBroker.contract, event: "SetController", logs: logs, sub: sub}, nil
}

// WatchSetController is a free log subscription operation binding the contract event 0x4ff638452bbf33c012645d18ae6f05515ff5f2d1dfb0cece8cbf018c60903f70.
//
// Solidity: event SetController(address controller)
func (_LivepeerETHTicketBroker *LivepeerETHTicketBrokerFilterer) WatchSetController(opts *bind.WatchOpts, sink chan<- *LivepeerETHTicketBrokerSetController) (event.Subscription, error) {

	logs, sub, err := _LivepeerETHTicketBroker.contract.WatchLogs(opts, "SetController")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(LivepeerETHTicketBrokerSetController)
				if err := _LivepeerETHTicketBroker.contract.UnpackLog(event, "SetController", log); err != nil {
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

// LivepeerETHTicketBrokerSignersApprovedIterator is returned from FilterSignersApproved and is used to iterate over the raw logs and unpacked data for SignersApproved events raised by the LivepeerETHTicketBroker contract.
type LivepeerETHTicketBrokerSignersApprovedIterator struct {
	Event *LivepeerETHTicketBrokerSignersApproved // Event containing the contract specifics and raw log

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
func (it *LivepeerETHTicketBrokerSignersApprovedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(LivepeerETHTicketBrokerSignersApproved)
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
		it.Event = new(LivepeerETHTicketBrokerSignersApproved)
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
func (it *LivepeerETHTicketBrokerSignersApprovedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *LivepeerETHTicketBrokerSignersApprovedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// LivepeerETHTicketBrokerSignersApproved represents a SignersApproved event raised by the LivepeerETHTicketBroker contract.
type LivepeerETHTicketBrokerSignersApproved struct {
	Sender          common.Address
	ApprovedSigners []common.Address
	Raw             types.Log // Blockchain specific contextual infos
}

// FilterSignersApproved is a free log retrieval operation binding the contract event 0x8612106bdb1778879d2c28794f6e78939e298025e0afd8fb02c7f3aa4ba23e55.
//
// Solidity: event SignersApproved(address indexed sender, address[] approvedSigners)
func (_LivepeerETHTicketBroker *LivepeerETHTicketBrokerFilterer) FilterSignersApproved(opts *bind.FilterOpts, sender []common.Address) (*LivepeerETHTicketBrokerSignersApprovedIterator, error) {

	var senderRule []interface{}
	for _, senderItem := range sender {
		senderRule = append(senderRule, senderItem)
	}

	logs, sub, err := _LivepeerETHTicketBroker.contract.FilterLogs(opts, "SignersApproved", senderRule)
	if err != nil {
		return nil, err
	}
	return &LivepeerETHTicketBrokerSignersApprovedIterator{contract: _LivepeerETHTicketBroker.contract, event: "SignersApproved", logs: logs, sub: sub}, nil
}

// WatchSignersApproved is a free log subscription operation binding the contract event 0x8612106bdb1778879d2c28794f6e78939e298025e0afd8fb02c7f3aa4ba23e55.
//
// Solidity: event SignersApproved(address indexed sender, address[] approvedSigners)
func (_LivepeerETHTicketBroker *LivepeerETHTicketBrokerFilterer) WatchSignersApproved(opts *bind.WatchOpts, sink chan<- *LivepeerETHTicketBrokerSignersApproved, sender []common.Address) (event.Subscription, error) {

	var senderRule []interface{}
	for _, senderItem := range sender {
		senderRule = append(senderRule, senderItem)
	}

	logs, sub, err := _LivepeerETHTicketBroker.contract.WatchLogs(opts, "SignersApproved", senderRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(LivepeerETHTicketBrokerSignersApproved)
				if err := _LivepeerETHTicketBroker.contract.UnpackLog(event, "SignersApproved", log); err != nil {
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

// LivepeerETHTicketBrokerSignersRevocationRequestedIterator is returned from FilterSignersRevocationRequested and is used to iterate over the raw logs and unpacked data for SignersRevocationRequested events raised by the LivepeerETHTicketBroker contract.
type LivepeerETHTicketBrokerSignersRevocationRequestedIterator struct {
	Event *LivepeerETHTicketBrokerSignersRevocationRequested // Event containing the contract specifics and raw log

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
func (it *LivepeerETHTicketBrokerSignersRevocationRequestedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(LivepeerETHTicketBrokerSignersRevocationRequested)
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
		it.Event = new(LivepeerETHTicketBrokerSignersRevocationRequested)
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
func (it *LivepeerETHTicketBrokerSignersRevocationRequestedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *LivepeerETHTicketBrokerSignersRevocationRequestedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// LivepeerETHTicketBrokerSignersRevocationRequested represents a SignersRevocationRequested event raised by the LivepeerETHTicketBroker contract.
type LivepeerETHTicketBrokerSignersRevocationRequested struct {
	Sender          common.Address
	Signers         []common.Address
	RevocationBlock *big.Int
	Raw             types.Log // Blockchain specific contextual infos
}

// FilterSignersRevocationRequested is a free log retrieval operation binding the contract event 0x866265c73ee21dac2b20249a3684fcb0378e11be6e813b0c594eb1b21471d8c5.
//
// Solidity: event SignersRevocationRequested(address indexed sender, address[] signers, uint256 revocationBlock)
func (_LivepeerETHTicketBroker *LivepeerETHTicketBrokerFilterer) FilterSignersRevocationRequested(opts *bind.FilterOpts, sender []common.Address) (*LivepeerETHTicketBrokerSignersRevocationRequestedIterator, error) {

	var senderRule []interface{}
	for _, senderItem := range sender {
		senderRule = append(senderRule, senderItem)
	}

	logs, sub, err := _LivepeerETHTicketBroker.contract.FilterLogs(opts, "SignersRevocationRequested", senderRule)
	if err != nil {
		return nil, err
	}
	return &LivepeerETHTicketBrokerSignersRevocationRequestedIterator{contract: _LivepeerETHTicketBroker.contract, event: "SignersRevocationRequested", logs: logs, sub: sub}, nil
}

// WatchSignersRevocationRequested is a free log subscription operation binding the contract event 0x866265c73ee21dac2b20249a3684fcb0378e11be6e813b0c594eb1b21471d8c5.
//
// Solidity: event SignersRevocationRequested(address indexed sender, address[] signers, uint256 revocationBlock)
func (_LivepeerETHTicketBroker *LivepeerETHTicketBrokerFilterer) WatchSignersRevocationRequested(opts *bind.WatchOpts, sink chan<- *LivepeerETHTicketBrokerSignersRevocationRequested, sender []common.Address) (event.Subscription, error) {

	var senderRule []interface{}
	for _, senderItem := range sender {
		senderRule = append(senderRule, senderItem)
	}

	logs, sub, err := _LivepeerETHTicketBroker.contract.WatchLogs(opts, "SignersRevocationRequested", senderRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(LivepeerETHTicketBrokerSignersRevocationRequested)
				if err := _LivepeerETHTicketBroker.contract.UnpackLog(event, "SignersRevocationRequested", log); err != nil {
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

// LivepeerETHTicketBrokerUnlockIterator is returned from FilterUnlock and is used to iterate over the raw logs and unpacked data for Unlock events raised by the LivepeerETHTicketBroker contract.
type LivepeerETHTicketBrokerUnlockIterator struct {
	Event *LivepeerETHTicketBrokerUnlock // Event containing the contract specifics and raw log

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
func (it *LivepeerETHTicketBrokerUnlockIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(LivepeerETHTicketBrokerUnlock)
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
		it.Event = new(LivepeerETHTicketBrokerUnlock)
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
func (it *LivepeerETHTicketBrokerUnlockIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *LivepeerETHTicketBrokerUnlockIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// LivepeerETHTicketBrokerUnlock represents a Unlock event raised by the LivepeerETHTicketBroker contract.
type LivepeerETHTicketBrokerUnlock struct {
	Sender     common.Address
	StartBlock *big.Int
	EndBlock   *big.Int
	Raw        types.Log // Blockchain specific contextual infos
}

// FilterUnlock is a free log retrieval operation binding the contract event 0xf7870c5b224cbc19873599e46ccfc7103934650509b1af0c3ce90138377c2004.
//
// Solidity: event Unlock(address indexed sender, uint256 startBlock, uint256 endBlock)
func (_LivepeerETHTicketBroker *LivepeerETHTicketBrokerFilterer) FilterUnlock(opts *bind.FilterOpts, sender []common.Address) (*LivepeerETHTicketBrokerUnlockIterator, error) {

	var senderRule []interface{}
	for _, senderItem := range sender {
		senderRule = append(senderRule, senderItem)
	}

	logs, sub, err := _LivepeerETHTicketBroker.contract.FilterLogs(opts, "Unlock", senderRule)
	if err != nil {
		return nil, err
	}
	return &LivepeerETHTicketBrokerUnlockIterator{contract: _LivepeerETHTicketBroker.contract, event: "Unlock", logs: logs, sub: sub}, nil
}

// WatchUnlock is a free log subscription operation binding the contract event 0xf7870c5b224cbc19873599e46ccfc7103934650509b1af0c3ce90138377c2004.
//
// Solidity: event Unlock(address indexed sender, uint256 startBlock, uint256 endBlock)
func (_LivepeerETHTicketBroker *LivepeerETHTicketBrokerFilterer) WatchUnlock(opts *bind.WatchOpts, sink chan<- *LivepeerETHTicketBrokerUnlock, sender []common.Address) (event.Subscription, error) {

	var senderRule []interface{}
	for _, senderItem := range sender {
		senderRule = append(senderRule, senderItem)
	}

	logs, sub, err := _LivepeerETHTicketBroker.contract.WatchLogs(opts, "Unlock", senderRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(LivepeerETHTicketBrokerUnlock)
				if err := _LivepeerETHTicketBroker.contract.UnpackLog(event, "Unlock", log); err != nil {
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

// LivepeerETHTicketBrokerUnlockCancelledIterator is returned from FilterUnlockCancelled and is used to iterate over the raw logs and unpacked data for UnlockCancelled events raised by the LivepeerETHTicketBroker contract.
type LivepeerETHTicketBrokerUnlockCancelledIterator struct {
	Event *LivepeerETHTicketBrokerUnlockCancelled // Event containing the contract specifics and raw log

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
func (it *LivepeerETHTicketBrokerUnlockCancelledIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(LivepeerETHTicketBrokerUnlockCancelled)
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
		it.Event = new(LivepeerETHTicketBrokerUnlockCancelled)
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
func (it *LivepeerETHTicketBrokerUnlockCancelledIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *LivepeerETHTicketBrokerUnlockCancelledIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// LivepeerETHTicketBrokerUnlockCancelled represents a UnlockCancelled event raised by the LivepeerETHTicketBroker contract.
type LivepeerETHTicketBrokerUnlockCancelled struct {
	Sender common.Address
	Raw    types.Log // Blockchain specific contextual infos
}

// FilterUnlockCancelled is a free log retrieval operation binding the contract event 0xfa044b7b93a40365dc68049797c2eb06918523d694e5d56e406cac3eb35578e5.
//
// Solidity: event UnlockCancelled(address indexed sender)
func (_LivepeerETHTicketBroker *LivepeerETHTicketBrokerFilterer) FilterUnlockCancelled(opts *bind.FilterOpts, sender []common.Address) (*LivepeerETHTicketBrokerUnlockCancelledIterator, error) {

	var senderRule []interface{}
	for _, senderItem := range sender {
		senderRule = append(senderRule, senderItem)
	}

	logs, sub, err := _LivepeerETHTicketBroker.contract.FilterLogs(opts, "UnlockCancelled", senderRule)
	if err != nil {
		return nil, err
	}
	return &LivepeerETHTicketBrokerUnlockCancelledIterator{contract: _LivepeerETHTicketBroker.contract, event: "UnlockCancelled", logs: logs, sub: sub}, nil
}

// WatchUnlockCancelled is a free log subscription operation binding the contract event 0xfa044b7b93a40365dc68049797c2eb06918523d694e5d56e406cac3eb35578e5.
//
// Solidity: event UnlockCancelled(address indexed sender)
func (_LivepeerETHTicketBroker *LivepeerETHTicketBrokerFilterer) WatchUnlockCancelled(opts *bind.WatchOpts, sink chan<- *LivepeerETHTicketBrokerUnlockCancelled, sender []common.Address) (event.Subscription, error) {

	var senderRule []interface{}
	for _, senderItem := range sender {
		senderRule = append(senderRule, senderItem)
	}

	logs, sub, err := _LivepeerETHTicketBroker.contract.WatchLogs(opts, "UnlockCancelled", senderRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(LivepeerETHTicketBrokerUnlockCancelled)
				if err := _LivepeerETHTicketBroker.contract.UnpackLog(event, "UnlockCancelled", log); err != nil {
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

// LivepeerETHTicketBrokerWinningTicketRedeemedIterator is returned from FilterWinningTicketRedeemed and is used to iterate over the raw logs and unpacked data for WinningTicketRedeemed events raised by the LivepeerETHTicketBroker contract.
type LivepeerETHTicketBrokerWinningTicketRedeemedIterator struct {
	Event *LivepeerETHTicketBrokerWinningTicketRedeemed // Event containing the contract specifics and raw log

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
func (it *LivepeerETHTicketBrokerWinningTicketRedeemedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(LivepeerETHTicketBrokerWinningTicketRedeemed)
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
		it.Event = new(LivepeerETHTicketBrokerWinningTicketRedeemed)
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
func (it *LivepeerETHTicketBrokerWinningTicketRedeemedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *LivepeerETHTicketBrokerWinningTicketRedeemedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// LivepeerETHTicketBrokerWinningTicketRedeemed represents a WinningTicketRedeemed event raised by the LivepeerETHTicketBroker contract.
type LivepeerETHTicketBrokerWinningTicketRedeemed struct {
	Sender        common.Address
	Recipient     common.Address
	FaceValue     *big.Int
	WinProb       *big.Int
	SenderNonce   *big.Int
	RecipientRand *big.Int
	AuxData       []byte
	Raw           types.Log // Blockchain specific contextual infos
}

// FilterWinningTicketRedeemed is a free log retrieval operation binding the contract event 0xc389eb51ed006dbf2528507f010efdf5225ea596e1e1741d74f550dab1925ee7.
//
// Solidity: event WinningTicketRedeemed(address indexed sender, address indexed recipient, uint256 faceValue, uint256 winProb, uint256 senderNonce, uint256 recipientRand, bytes auxData)
func (_LivepeerETHTicketBroker *LivepeerETHTicketBrokerFilterer) FilterWinningTicketRedeemed(opts *bind.FilterOpts, sender []common.Address, recipient []common.Address) (*LivepeerETHTicketBrokerWinningTicketRedeemedIterator, error) {

	var senderRule []interface{}
	for _, senderItem := range sender {
		senderRule = append(senderRule, senderItem)
	}
	var recipientRule []interface{}
	for _, recipientItem := range recipient {
		recipientRule = append(recipientRule, recipientItem)
	}

	logs, sub, err := _LivepeerETHTicketBroker.contract.FilterLogs(opts, "WinningTicketRedeemed", senderRule, recipientRule)
	if err != nil {
		return nil, err
	}
	return &LivepeerETHTicketBrokerWinningTicketRedeemedIterator{contract: _LivepeerETHTicketBroker.contract, event: "WinningTicketRedeemed", logs: logs, sub: sub}, nil
}

// WatchWinningTicketRedeemed is a free log subscription operation binding the contract event 0xc389eb51ed006dbf2528507f010efdf5225ea596e1e1741d74f550dab1925ee7.
//
// Solidity: event WinningTicketRedeemed(address indexed sender, address indexed recipient, uint256 faceValue, uint256 winProb, uint256 senderNonce, uint256 recipientRand, bytes auxData)
func (_LivepeerETHTicketBroker *LivepeerETHTicketBrokerFilterer) WatchWinningTicketRedeemed(opts *bind.WatchOpts, sink chan<- *LivepeerETHTicketBrokerWinningTicketRedeemed, sender []common.Address, recipient []common.Address) (event.Subscription, error) {

	var senderRule []interface{}
	for _, senderItem := range sender {
		senderRule = append(senderRule, senderItem)
	}
	var recipientRule []interface{}
	for _, recipientItem := range recipient {
		recipientRule = append(recipientRule, recipientItem)
	}

	logs, sub, err := _LivepeerETHTicketBroker.contract.WatchLogs(opts, "WinningTicketRedeemed", senderRule, recipientRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(LivepeerETHTicketBrokerWinningTicketRedeemed)
				if err := _LivepeerETHTicketBroker.contract.UnpackLog(event, "WinningTicketRedeemed", log); err != nil {
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

// LivepeerETHTicketBrokerWinningTicketTransferIterator is returned from FilterWinningTicketTransfer and is used to iterate over the raw logs and unpacked data for WinningTicketTransfer events raised by the LivepeerETHTicketBroker contract.
type LivepeerETHTicketBrokerWinningTicketTransferIterator struct {
	Event *LivepeerETHTicketBrokerWinningTicketTransfer // Event containing the contract specifics and raw log

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
func (it *LivepeerETHTicketBrokerWinningTicketTransferIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(LivepeerETHTicketBrokerWinningTicketTransfer)
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
		it.Event = new(LivepeerETHTicketBrokerWinningTicketTransfer)
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
func (it *LivepeerETHTicketBrokerWinningTicketTransferIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *LivepeerETHTicketBrokerWinningTicketTransferIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// LivepeerETHTicketBrokerWinningTicketTransfer represents a WinningTicketTransfer event raised by the LivepeerETHTicketBroker contract.
type LivepeerETHTicketBrokerWinningTicketTransfer struct {
	Sender    common.Address
	Recipient common.Address
	Amount    *big.Int
	Raw       types.Log // Blockchain specific contextual infos
}

// FilterWinningTicketTransfer is a free log retrieval operation binding the contract event 0x8b87351a208c06e3ceee59d80725fd77a23b4129e1b51ca231fc89b40712649c.
//
// Solidity: event WinningTicketTransfer(address indexed sender, address indexed recipient, uint256 amount)
func (_LivepeerETHTicketBroker *LivepeerETHTicketBrokerFilterer) FilterWinningTicketTransfer(opts *bind.FilterOpts, sender []common.Address, recipient []common.Address) (*LivepeerETHTicketBrokerWinningTicketTransferIterator, error) {

	var senderRule []interface{}
	for _, senderItem := range sender {
		senderRule = append(senderRule, senderItem)
	}
	var recipientRule []interface{}
	for _, recipientItem := range recipient {
		recipientRule = append(recipientRule, recipientItem)
	}

	logs, sub, err := _LivepeerETHTicketBroker.contract.FilterLogs(opts, "WinningTicketTransfer", senderRule, recipientRule)
	if err != nil {
		return nil, err
	}
	return &LivepeerETHTicketBrokerWinningTicketTransferIterator{contract: _LivepeerETHTicketBroker.contract, event: "WinningTicketTransfer", logs: logs, sub: sub}, nil
}

// WatchWinningTicketTransfer is a free log subscription operation binding the contract event 0x8b87351a208c06e3ceee59d80725fd77a23b4129e1b51ca231fc89b40712649c.
//
// Solidity: event WinningTicketTransfer(address indexed sender, address indexed recipient, uint256 amount)
func (_LivepeerETHTicketBroker *LivepeerETHTicketBrokerFilterer) WatchWinningTicketTransfer(opts *bind.WatchOpts, sink chan<- *LivepeerETHTicketBrokerWinningTicketTransfer, sender []common.Address, recipient []common.Address) (event.Subscription, error) {

	var senderRule []interface{}
	for _, senderItem := range sender {
		senderRule = append(senderRule, senderItem)
	}
	var recipientRule []interface{}
	for _, recipientItem := range recipient {
		recipientRule = append(recipientRule, recipientItem)
	}

	logs, sub, err := _LivepeerETHTicketBroker.contract.WatchLogs(opts, "WinningTicketTransfer", senderRule, recipientRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(LivepeerETHTicketBrokerWinningTicketTransfer)
				if err := _LivepeerETHTicketBroker.contract.UnpackLog(event, "WinningTicketTransfer", log); err != nil {
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

// LivepeerETHTicketBrokerWithdrawalIterator is returned from FilterWithdrawal and is used to iterate over the raw logs and unpacked data for Withdrawal events raised by the LivepeerETHTicketBroker contract.
type LivepeerETHTicketBrokerWithdrawalIterator struct {
	Event *LivepeerETHTicketBrokerWithdrawal // Event containing the contract specifics and raw log

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
func (it *LivepeerETHTicketBrokerWithdrawalIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(LivepeerETHTicketBrokerWithdrawal)
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
		it.Event = new(LivepeerETHTicketBrokerWithdrawal)
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
func (it *LivepeerETHTicketBrokerWithdrawalIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *LivepeerETHTicketBrokerWithdrawalIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// LivepeerETHTicketBrokerWithdrawal represents a Withdrawal event raised by the LivepeerETHTicketBroker contract.
type LivepeerETHTicketBrokerWithdrawal struct {
	Sender        common.Address
	Deposit       *big.Int
	PenaltyEscrow *big.Int
	Raw           types.Log // Blockchain specific contextual infos
}

// FilterWithdrawal is a free log retrieval operation binding the contract event 0xdf273cb619d95419a9cd0ec88123a0538c85064229baa6363788f743fff90deb.
//
// Solidity: event Withdrawal(address indexed sender, uint256 deposit, uint256 penaltyEscrow)
func (_LivepeerETHTicketBroker *LivepeerETHTicketBrokerFilterer) FilterWithdrawal(opts *bind.FilterOpts, sender []common.Address) (*LivepeerETHTicketBrokerWithdrawalIterator, error) {

	var senderRule []interface{}
	for _, senderItem := range sender {
		senderRule = append(senderRule, senderItem)
	}

	logs, sub, err := _LivepeerETHTicketBroker.contract.FilterLogs(opts, "Withdrawal", senderRule)
	if err != nil {
		return nil, err
	}
	return &LivepeerETHTicketBrokerWithdrawalIterator{contract: _LivepeerETHTicketBroker.contract, event: "Withdrawal", logs: logs, sub: sub}, nil
}

// WatchWithdrawal is a free log subscription operation binding the contract event 0xdf273cb619d95419a9cd0ec88123a0538c85064229baa6363788f743fff90deb.
//
// Solidity: event Withdrawal(address indexed sender, uint256 deposit, uint256 penaltyEscrow)
func (_LivepeerETHTicketBroker *LivepeerETHTicketBrokerFilterer) WatchWithdrawal(opts *bind.WatchOpts, sink chan<- *LivepeerETHTicketBrokerWithdrawal, sender []common.Address) (event.Subscription, error) {

	var senderRule []interface{}
	for _, senderItem := range sender {
		senderRule = append(senderRule, senderItem)
	}

	logs, sub, err := _LivepeerETHTicketBroker.contract.WatchLogs(opts, "Withdrawal", senderRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(LivepeerETHTicketBrokerWithdrawal)
				if err := _LivepeerETHTicketBroker.contract.UnpackLog(event, "Withdrawal", log); err != nil {
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
