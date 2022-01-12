// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package contracts

import (
	"errors"
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
	_ = errors.New
	_ = big.NewInt
	_ = strings.NewReader
	_ = ethereum.NotFound
	_ = bind.Bind
	_ = common.Big1
	_ = types.BloomLookup
	_ = event.NewSubscription
)

// MReserveReserveInfo is an auto generated low-level Go binding around an user-defined struct.
type MReserveReserveInfo struct {
	FundsRemaining        *big.Int
	ClaimedInCurrentRound *big.Int
}

// MTicketBrokerCoreTicket is an auto generated low-level Go binding around an user-defined struct.
type MTicketBrokerCoreTicket struct {
	Recipient         common.Address
	Sender            common.Address
	FaceValue         *big.Int
	WinProb           *big.Int
	SenderNonce       *big.Int
	RecipientRandHash [32]byte
	AuxData           []byte
}

// MixinTicketBrokerCoreSender is an auto generated low-level Go binding around an user-defined struct.
type MixinTicketBrokerCoreSender struct {
	Deposit       *big.Int
	WithdrawRound *big.Int
}

// TicketBrokerMetaData contains all meta data concerning the TicketBroker contract.
var TicketBrokerMetaData = &bind.MetaData{
	ABI: "[{\"constant\":true,\"inputs\":[{\"internalType\":\"address\",\"name\":\"_sender\",\"type\":\"address\"}],\"name\":\"isUnlockInProgress\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"unlockPeriod\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[],\"name\":\"withdraw\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_unlockPeriod\",\"type\":\"uint256\"}],\"name\":\"setUnlockPeriod\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"internalType\":\"address\",\"name\":\"_reserveHolder\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"_claimant\",\"type\":\"address\"}],\"name\":\"claimedReserve\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_depositAmount\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"_reserveAmount\",\"type\":\"uint256\"}],\"name\":\"fundDepositAndReserve\",\"outputs\":[],\"payable\":true,\"stateMutability\":\"payable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"targetContractId\",\"outputs\":[{\"internalType\":\"bytes32\",\"name\":\"\",\"type\":\"bytes32\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"\",\"type\":\"bytes32\"}],\"name\":\"usedTickets\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"internalType\":\"address\",\"name\":\"_reserveHolder\",\"type\":\"address\"}],\"name\":\"getReserveInfo\",\"outputs\":[{\"components\":[{\"internalType\":\"uint256\",\"name\":\"fundsRemaining\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"claimedInCurrentRound\",\"type\":\"uint256\"}],\"internalType\":\"structMReserve.ReserveInfo\",\"name\":\"info\",\"type\":\"tuple\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[],\"name\":\"fundDeposit\",\"outputs\":[],\"payable\":true,\"stateMutability\":\"payable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[],\"name\":\"fundReserve\",\"outputs\":[],\"payable\":true,\"stateMutability\":\"payable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"internalType\":\"address\",\"name\":\"_reserveHolder\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"_claimant\",\"type\":\"address\"}],\"name\":\"claimableReserve\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"ticketValidityPeriod\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"internalType\":\"address\",\"name\":\"_controller\",\"type\":\"address\"}],\"name\":\"setController\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"internalType\":\"address\",\"name\":\"_addr\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"_depositAmount\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"_reserveAmount\",\"type\":\"uint256\"}],\"name\":\"fundDepositAndReserveFor\",\"outputs\":[],\"payable\":true,\"stateMutability\":\"payable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[],\"name\":\"unlock\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"components\":[{\"internalType\":\"address\",\"name\":\"recipient\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"sender\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"faceValue\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"winProb\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"senderNonce\",\"type\":\"uint256\"},{\"internalType\":\"bytes32\",\"name\":\"recipientRandHash\",\"type\":\"bytes32\"},{\"internalType\":\"bytes\",\"name\":\"auxData\",\"type\":\"bytes\"}],\"internalType\":\"structMTicketBrokerCore.Ticket\",\"name\":\"_ticket\",\"type\":\"tuple\"}],\"name\":\"getTicketHash\",\"outputs\":[{\"internalType\":\"bytes32\",\"name\":\"\",\"type\":\"bytes32\"}],\"payable\":false,\"stateMutability\":\"pure\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[],\"name\":\"cancelUnlock\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_ticketValidityPeriod\",\"type\":\"uint256\"}],\"name\":\"setTicketValidityPeriod\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"components\":[{\"internalType\":\"address\",\"name\":\"recipient\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"sender\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"faceValue\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"winProb\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"senderNonce\",\"type\":\"uint256\"},{\"internalType\":\"bytes32\",\"name\":\"recipientRandHash\",\"type\":\"bytes32\"},{\"internalType\":\"bytes\",\"name\":\"auxData\",\"type\":\"bytes\"}],\"internalType\":\"structMTicketBrokerCore.Ticket[]\",\"name\":\"_tickets\",\"type\":\"tuple[]\"},{\"internalType\":\"bytes[]\",\"name\":\"_sigs\",\"type\":\"bytes[]\"},{\"internalType\":\"uint256[]\",\"name\":\"_recipientRands\",\"type\":\"uint256[]\"}],\"name\":\"batchRedeemWinningTickets\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"internalType\":\"address\",\"name\":\"_sender\",\"type\":\"address\"}],\"name\":\"getSenderInfo\",\"outputs\":[{\"components\":[{\"internalType\":\"uint256\",\"name\":\"deposit\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"withdrawRound\",\"type\":\"uint256\"}],\"internalType\":\"structMixinTicketBrokerCore.Sender\",\"name\":\"sender\",\"type\":\"tuple\"},{\"components\":[{\"internalType\":\"uint256\",\"name\":\"fundsRemaining\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"claimedInCurrentRound\",\"type\":\"uint256\"}],\"internalType\":\"structMReserve.ReserveInfo\",\"name\":\"reserve\",\"type\":\"tuple\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"components\":[{\"internalType\":\"address\",\"name\":\"recipient\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"sender\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"faceValue\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"winProb\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"senderNonce\",\"type\":\"uint256\"},{\"internalType\":\"bytes32\",\"name\":\"recipientRandHash\",\"type\":\"bytes32\"},{\"internalType\":\"bytes\",\"name\":\"auxData\",\"type\":\"bytes\"}],\"internalType\":\"structMTicketBrokerCore.Ticket\",\"name\":\"_ticket\",\"type\":\"tuple\"},{\"internalType\":\"bytes\",\"name\":\"_sig\",\"type\":\"bytes\"},{\"internalType\":\"uint256\",\"name\":\"_recipientRand\",\"type\":\"uint256\"}],\"name\":\"redeemWinningTicket\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"controller\",\"outputs\":[{\"internalType\":\"contractIController\",\"name\":\"\",\"type\":\"address\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"_controller\",\"type\":\"address\"}],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"constructor\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"sender\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"}],\"name\":\"DepositFunded\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"sender\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"recipient\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"faceValue\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"winProb\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"senderNonce\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"recipientRand\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"bytes\",\"name\":\"auxData\",\"type\":\"bytes\"}],\"name\":\"WinningTicketRedeemed\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"sender\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"recipient\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"}],\"name\":\"WinningTicketTransfer\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"sender\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"startRound\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"endRound\",\"type\":\"uint256\"}],\"name\":\"Unlock\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"sender\",\"type\":\"address\"}],\"name\":\"UnlockCancelled\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"sender\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"deposit\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"reserve\",\"type\":\"uint256\"}],\"name\":\"Withdrawal\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"reserveHolder\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"}],\"name\":\"ReserveFunded\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"reserveHolder\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"address\",\"name\":\"claimant\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"}],\"name\":\"ReserveClaimed\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"address\",\"name\":\"controller\",\"type\":\"address\"}],\"name\":\"SetController\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"string\",\"name\":\"param\",\"type\":\"string\"}],\"name\":\"ParameterUpdate\",\"type\":\"event\"}]",
}

// TicketBrokerABI is the input ABI used to generate the binding from.
// Deprecated: Use TicketBrokerMetaData.ABI instead.
var TicketBrokerABI = TicketBrokerMetaData.ABI

// TicketBroker is an auto generated Go binding around an Ethereum contract.
type TicketBroker struct {
	TicketBrokerCaller     // Read-only binding to the contract
	TicketBrokerTransactor // Write-only binding to the contract
	TicketBrokerFilterer   // Log filterer for contract events
}

// TicketBrokerCaller is an auto generated read-only Go binding around an Ethereum contract.
type TicketBrokerCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// TicketBrokerTransactor is an auto generated write-only Go binding around an Ethereum contract.
type TicketBrokerTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// TicketBrokerFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type TicketBrokerFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// TicketBrokerSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type TicketBrokerSession struct {
	Contract     *TicketBroker     // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// TicketBrokerCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type TicketBrokerCallerSession struct {
	Contract *TicketBrokerCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts       // Call options to use throughout this session
}

// TicketBrokerTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type TicketBrokerTransactorSession struct {
	Contract     *TicketBrokerTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts       // Transaction auth options to use throughout this session
}

// TicketBrokerRaw is an auto generated low-level Go binding around an Ethereum contract.
type TicketBrokerRaw struct {
	Contract *TicketBroker // Generic contract binding to access the raw methods on
}

// TicketBrokerCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type TicketBrokerCallerRaw struct {
	Contract *TicketBrokerCaller // Generic read-only contract binding to access the raw methods on
}

// TicketBrokerTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type TicketBrokerTransactorRaw struct {
	Contract *TicketBrokerTransactor // Generic write-only contract binding to access the raw methods on
}

// NewTicketBroker creates a new instance of TicketBroker, bound to a specific deployed contract.
func NewTicketBroker(address common.Address, backend bind.ContractBackend) (*TicketBroker, error) {
	contract, err := bindTicketBroker(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &TicketBroker{TicketBrokerCaller: TicketBrokerCaller{contract: contract}, TicketBrokerTransactor: TicketBrokerTransactor{contract: contract}, TicketBrokerFilterer: TicketBrokerFilterer{contract: contract}}, nil
}

// NewTicketBrokerCaller creates a new read-only instance of TicketBroker, bound to a specific deployed contract.
func NewTicketBrokerCaller(address common.Address, caller bind.ContractCaller) (*TicketBrokerCaller, error) {
	contract, err := bindTicketBroker(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &TicketBrokerCaller{contract: contract}, nil
}

// NewTicketBrokerTransactor creates a new write-only instance of TicketBroker, bound to a specific deployed contract.
func NewTicketBrokerTransactor(address common.Address, transactor bind.ContractTransactor) (*TicketBrokerTransactor, error) {
	contract, err := bindTicketBroker(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &TicketBrokerTransactor{contract: contract}, nil
}

// NewTicketBrokerFilterer creates a new log filterer instance of TicketBroker, bound to a specific deployed contract.
func NewTicketBrokerFilterer(address common.Address, filterer bind.ContractFilterer) (*TicketBrokerFilterer, error) {
	contract, err := bindTicketBroker(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &TicketBrokerFilterer{contract: contract}, nil
}

// bindTicketBroker binds a generic wrapper to an already deployed contract.
func bindTicketBroker(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := abi.JSON(strings.NewReader(TicketBrokerABI))
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_TicketBroker *TicketBrokerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _TicketBroker.Contract.TicketBrokerCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_TicketBroker *TicketBrokerRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _TicketBroker.Contract.TicketBrokerTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_TicketBroker *TicketBrokerRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _TicketBroker.Contract.TicketBrokerTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_TicketBroker *TicketBrokerCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _TicketBroker.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_TicketBroker *TicketBrokerTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _TicketBroker.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_TicketBroker *TicketBrokerTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _TicketBroker.Contract.contract.Transact(opts, method, params...)
}

// ClaimableReserve is a free data retrieval call binding the contract method 0x81779f38.
//
// Solidity: function claimableReserve(address _reserveHolder, address _claimant) view returns(uint256)
func (_TicketBroker *TicketBrokerCaller) ClaimableReserve(opts *bind.CallOpts, _reserveHolder common.Address, _claimant common.Address) (*big.Int, error) {
	var out []interface{}
	err := _TicketBroker.contract.Call(opts, &out, "claimableReserve", _reserveHolder, _claimant)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// ClaimableReserve is a free data retrieval call binding the contract method 0x81779f38.
//
// Solidity: function claimableReserve(address _reserveHolder, address _claimant) view returns(uint256)
func (_TicketBroker *TicketBrokerSession) ClaimableReserve(_reserveHolder common.Address, _claimant common.Address) (*big.Int, error) {
	return _TicketBroker.Contract.ClaimableReserve(&_TicketBroker.CallOpts, _reserveHolder, _claimant)
}

// ClaimableReserve is a free data retrieval call binding the contract method 0x81779f38.
//
// Solidity: function claimableReserve(address _reserveHolder, address _claimant) view returns(uint256)
func (_TicketBroker *TicketBrokerCallerSession) ClaimableReserve(_reserveHolder common.Address, _claimant common.Address) (*big.Int, error) {
	return _TicketBroker.Contract.ClaimableReserve(&_TicketBroker.CallOpts, _reserveHolder, _claimant)
}

// ClaimedReserve is a free data retrieval call binding the contract method 0x4ac826da.
//
// Solidity: function claimedReserve(address _reserveHolder, address _claimant) view returns(uint256)
func (_TicketBroker *TicketBrokerCaller) ClaimedReserve(opts *bind.CallOpts, _reserveHolder common.Address, _claimant common.Address) (*big.Int, error) {
	var out []interface{}
	err := _TicketBroker.contract.Call(opts, &out, "claimedReserve", _reserveHolder, _claimant)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// ClaimedReserve is a free data retrieval call binding the contract method 0x4ac826da.
//
// Solidity: function claimedReserve(address _reserveHolder, address _claimant) view returns(uint256)
func (_TicketBroker *TicketBrokerSession) ClaimedReserve(_reserveHolder common.Address, _claimant common.Address) (*big.Int, error) {
	return _TicketBroker.Contract.ClaimedReserve(&_TicketBroker.CallOpts, _reserveHolder, _claimant)
}

// ClaimedReserve is a free data retrieval call binding the contract method 0x4ac826da.
//
// Solidity: function claimedReserve(address _reserveHolder, address _claimant) view returns(uint256)
func (_TicketBroker *TicketBrokerCallerSession) ClaimedReserve(_reserveHolder common.Address, _claimant common.Address) (*big.Int, error) {
	return _TicketBroker.Contract.ClaimedReserve(&_TicketBroker.CallOpts, _reserveHolder, _claimant)
}

// Controller is a free data retrieval call binding the contract method 0xf77c4791.
//
// Solidity: function controller() view returns(address)
func (_TicketBroker *TicketBrokerCaller) Controller(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _TicketBroker.contract.Call(opts, &out, "controller")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// Controller is a free data retrieval call binding the contract method 0xf77c4791.
//
// Solidity: function controller() view returns(address)
func (_TicketBroker *TicketBrokerSession) Controller() (common.Address, error) {
	return _TicketBroker.Contract.Controller(&_TicketBroker.CallOpts)
}

// Controller is a free data retrieval call binding the contract method 0xf77c4791.
//
// Solidity: function controller() view returns(address)
func (_TicketBroker *TicketBrokerCallerSession) Controller() (common.Address, error) {
	return _TicketBroker.Contract.Controller(&_TicketBroker.CallOpts)
}

// GetReserveInfo is a free data retrieval call binding the contract method 0x5b6333eb.
//
// Solidity: function getReserveInfo(address _reserveHolder) view returns((uint256,uint256) info)
func (_TicketBroker *TicketBrokerCaller) GetReserveInfo(opts *bind.CallOpts, _reserveHolder common.Address) (MReserveReserveInfo, error) {
	var out []interface{}
	err := _TicketBroker.contract.Call(opts, &out, "getReserveInfo", _reserveHolder)

	if err != nil {
		return *new(MReserveReserveInfo), err
	}

	out0 := *abi.ConvertType(out[0], new(MReserveReserveInfo)).(*MReserveReserveInfo)

	return out0, err

}

// GetReserveInfo is a free data retrieval call binding the contract method 0x5b6333eb.
//
// Solidity: function getReserveInfo(address _reserveHolder) view returns((uint256,uint256) info)
func (_TicketBroker *TicketBrokerSession) GetReserveInfo(_reserveHolder common.Address) (MReserveReserveInfo, error) {
	return _TicketBroker.Contract.GetReserveInfo(&_TicketBroker.CallOpts, _reserveHolder)
}

// GetReserveInfo is a free data retrieval call binding the contract method 0x5b6333eb.
//
// Solidity: function getReserveInfo(address _reserveHolder) view returns((uint256,uint256) info)
func (_TicketBroker *TicketBrokerCallerSession) GetReserveInfo(_reserveHolder common.Address) (MReserveReserveInfo, error) {
	return _TicketBroker.Contract.GetReserveInfo(&_TicketBroker.CallOpts, _reserveHolder)
}

// GetSenderInfo is a free data retrieval call binding the contract method 0xe1a589da.
//
// Solidity: function getSenderInfo(address _sender) view returns((uint256,uint256) sender, (uint256,uint256) reserve)
func (_TicketBroker *TicketBrokerCaller) GetSenderInfo(opts *bind.CallOpts, _sender common.Address) (struct {
	Sender  MixinTicketBrokerCoreSender
	Reserve MReserveReserveInfo
}, error) {
	var out []interface{}
	err := _TicketBroker.contract.Call(opts, &out, "getSenderInfo", _sender)

	outstruct := new(struct {
		Sender  MixinTicketBrokerCoreSender
		Reserve MReserveReserveInfo
	})
	if err != nil {
		return *outstruct, err
	}

	outstruct.Sender = *abi.ConvertType(out[0], new(MixinTicketBrokerCoreSender)).(*MixinTicketBrokerCoreSender)
	outstruct.Reserve = *abi.ConvertType(out[1], new(MReserveReserveInfo)).(*MReserveReserveInfo)

	return *outstruct, err

}

// GetSenderInfo is a free data retrieval call binding the contract method 0xe1a589da.
//
// Solidity: function getSenderInfo(address _sender) view returns((uint256,uint256) sender, (uint256,uint256) reserve)
func (_TicketBroker *TicketBrokerSession) GetSenderInfo(_sender common.Address) (struct {
	Sender  MixinTicketBrokerCoreSender
	Reserve MReserveReserveInfo
}, error) {
	return _TicketBroker.Contract.GetSenderInfo(&_TicketBroker.CallOpts, _sender)
}

// GetSenderInfo is a free data retrieval call binding the contract method 0xe1a589da.
//
// Solidity: function getSenderInfo(address _sender) view returns((uint256,uint256) sender, (uint256,uint256) reserve)
func (_TicketBroker *TicketBrokerCallerSession) GetSenderInfo(_sender common.Address) (struct {
	Sender  MixinTicketBrokerCoreSender
	Reserve MReserveReserveInfo
}, error) {
	return _TicketBroker.Contract.GetSenderInfo(&_TicketBroker.CallOpts, _sender)
}

// GetTicketHash is a free data retrieval call binding the contract method 0xb03fa864.
//
// Solidity: function getTicketHash((address,address,uint256,uint256,uint256,bytes32,bytes) _ticket) pure returns(bytes32)
func (_TicketBroker *TicketBrokerCaller) GetTicketHash(opts *bind.CallOpts, _ticket MTicketBrokerCoreTicket) ([32]byte, error) {
	var out []interface{}
	err := _TicketBroker.contract.Call(opts, &out, "getTicketHash", _ticket)

	if err != nil {
		return *new([32]byte), err
	}

	out0 := *abi.ConvertType(out[0], new([32]byte)).(*[32]byte)

	return out0, err

}

// GetTicketHash is a free data retrieval call binding the contract method 0xb03fa864.
//
// Solidity: function getTicketHash((address,address,uint256,uint256,uint256,bytes32,bytes) _ticket) pure returns(bytes32)
func (_TicketBroker *TicketBrokerSession) GetTicketHash(_ticket MTicketBrokerCoreTicket) ([32]byte, error) {
	return _TicketBroker.Contract.GetTicketHash(&_TicketBroker.CallOpts, _ticket)
}

// GetTicketHash is a free data retrieval call binding the contract method 0xb03fa864.
//
// Solidity: function getTicketHash((address,address,uint256,uint256,uint256,bytes32,bytes) _ticket) pure returns(bytes32)
func (_TicketBroker *TicketBrokerCallerSession) GetTicketHash(_ticket MTicketBrokerCoreTicket) ([32]byte, error) {
	return _TicketBroker.Contract.GetTicketHash(&_TicketBroker.CallOpts, _ticket)
}

// IsUnlockInProgress is a free data retrieval call binding the contract method 0x121cdcc2.
//
// Solidity: function isUnlockInProgress(address _sender) view returns(bool)
func (_TicketBroker *TicketBrokerCaller) IsUnlockInProgress(opts *bind.CallOpts, _sender common.Address) (bool, error) {
	var out []interface{}
	err := _TicketBroker.contract.Call(opts, &out, "isUnlockInProgress", _sender)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// IsUnlockInProgress is a free data retrieval call binding the contract method 0x121cdcc2.
//
// Solidity: function isUnlockInProgress(address _sender) view returns(bool)
func (_TicketBroker *TicketBrokerSession) IsUnlockInProgress(_sender common.Address) (bool, error) {
	return _TicketBroker.Contract.IsUnlockInProgress(&_TicketBroker.CallOpts, _sender)
}

// IsUnlockInProgress is a free data retrieval call binding the contract method 0x121cdcc2.
//
// Solidity: function isUnlockInProgress(address _sender) view returns(bool)
func (_TicketBroker *TicketBrokerCallerSession) IsUnlockInProgress(_sender common.Address) (bool, error) {
	return _TicketBroker.Contract.IsUnlockInProgress(&_TicketBroker.CallOpts, _sender)
}

// TargetContractId is a free data retrieval call binding the contract method 0x51720b41.
//
// Solidity: function targetContractId() view returns(bytes32)
func (_TicketBroker *TicketBrokerCaller) TargetContractId(opts *bind.CallOpts) ([32]byte, error) {
	var out []interface{}
	err := _TicketBroker.contract.Call(opts, &out, "targetContractId")

	if err != nil {
		return *new([32]byte), err
	}

	out0 := *abi.ConvertType(out[0], new([32]byte)).(*[32]byte)

	return out0, err

}

// TargetContractId is a free data retrieval call binding the contract method 0x51720b41.
//
// Solidity: function targetContractId() view returns(bytes32)
func (_TicketBroker *TicketBrokerSession) TargetContractId() ([32]byte, error) {
	return _TicketBroker.Contract.TargetContractId(&_TicketBroker.CallOpts)
}

// TargetContractId is a free data retrieval call binding the contract method 0x51720b41.
//
// Solidity: function targetContractId() view returns(bytes32)
func (_TicketBroker *TicketBrokerCallerSession) TargetContractId() ([32]byte, error) {
	return _TicketBroker.Contract.TargetContractId(&_TicketBroker.CallOpts)
}

// TicketValidityPeriod is a free data retrieval call binding the contract method 0x856a2cf8.
//
// Solidity: function ticketValidityPeriod() view returns(uint256)
func (_TicketBroker *TicketBrokerCaller) TicketValidityPeriod(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _TicketBroker.contract.Call(opts, &out, "ticketValidityPeriod")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// TicketValidityPeriod is a free data retrieval call binding the contract method 0x856a2cf8.
//
// Solidity: function ticketValidityPeriod() view returns(uint256)
func (_TicketBroker *TicketBrokerSession) TicketValidityPeriod() (*big.Int, error) {
	return _TicketBroker.Contract.TicketValidityPeriod(&_TicketBroker.CallOpts)
}

// TicketValidityPeriod is a free data retrieval call binding the contract method 0x856a2cf8.
//
// Solidity: function ticketValidityPeriod() view returns(uint256)
func (_TicketBroker *TicketBrokerCallerSession) TicketValidityPeriod() (*big.Int, error) {
	return _TicketBroker.Contract.TicketValidityPeriod(&_TicketBroker.CallOpts)
}

// UnlockPeriod is a free data retrieval call binding the contract method 0x20d3a0b4.
//
// Solidity: function unlockPeriod() view returns(uint256)
func (_TicketBroker *TicketBrokerCaller) UnlockPeriod(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _TicketBroker.contract.Call(opts, &out, "unlockPeriod")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// UnlockPeriod is a free data retrieval call binding the contract method 0x20d3a0b4.
//
// Solidity: function unlockPeriod() view returns(uint256)
func (_TicketBroker *TicketBrokerSession) UnlockPeriod() (*big.Int, error) {
	return _TicketBroker.Contract.UnlockPeriod(&_TicketBroker.CallOpts)
}

// UnlockPeriod is a free data retrieval call binding the contract method 0x20d3a0b4.
//
// Solidity: function unlockPeriod() view returns(uint256)
func (_TicketBroker *TicketBrokerCallerSession) UnlockPeriod() (*big.Int, error) {
	return _TicketBroker.Contract.UnlockPeriod(&_TicketBroker.CallOpts)
}

// UsedTickets is a free data retrieval call binding the contract method 0x59a515ba.
//
// Solidity: function usedTickets(bytes32 ) view returns(bool)
func (_TicketBroker *TicketBrokerCaller) UsedTickets(opts *bind.CallOpts, arg0 [32]byte) (bool, error) {
	var out []interface{}
	err := _TicketBroker.contract.Call(opts, &out, "usedTickets", arg0)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// UsedTickets is a free data retrieval call binding the contract method 0x59a515ba.
//
// Solidity: function usedTickets(bytes32 ) view returns(bool)
func (_TicketBroker *TicketBrokerSession) UsedTickets(arg0 [32]byte) (bool, error) {
	return _TicketBroker.Contract.UsedTickets(&_TicketBroker.CallOpts, arg0)
}

// UsedTickets is a free data retrieval call binding the contract method 0x59a515ba.
//
// Solidity: function usedTickets(bytes32 ) view returns(bool)
func (_TicketBroker *TicketBrokerCallerSession) UsedTickets(arg0 [32]byte) (bool, error) {
	return _TicketBroker.Contract.UsedTickets(&_TicketBroker.CallOpts, arg0)
}

// BatchRedeemWinningTickets is a paid mutator transaction binding the contract method 0xd01b808e.
//
// Solidity: function batchRedeemWinningTickets((address,address,uint256,uint256,uint256,bytes32,bytes)[] _tickets, bytes[] _sigs, uint256[] _recipientRands) returns()
func (_TicketBroker *TicketBrokerTransactor) BatchRedeemWinningTickets(opts *bind.TransactOpts, _tickets []MTicketBrokerCoreTicket, _sigs [][]byte, _recipientRands []*big.Int) (*types.Transaction, error) {
	return _TicketBroker.contract.Transact(opts, "batchRedeemWinningTickets", _tickets, _sigs, _recipientRands)
}

// BatchRedeemWinningTickets is a paid mutator transaction binding the contract method 0xd01b808e.
//
// Solidity: function batchRedeemWinningTickets((address,address,uint256,uint256,uint256,bytes32,bytes)[] _tickets, bytes[] _sigs, uint256[] _recipientRands) returns()
func (_TicketBroker *TicketBrokerSession) BatchRedeemWinningTickets(_tickets []MTicketBrokerCoreTicket, _sigs [][]byte, _recipientRands []*big.Int) (*types.Transaction, error) {
	return _TicketBroker.Contract.BatchRedeemWinningTickets(&_TicketBroker.TransactOpts, _tickets, _sigs, _recipientRands)
}

// BatchRedeemWinningTickets is a paid mutator transaction binding the contract method 0xd01b808e.
//
// Solidity: function batchRedeemWinningTickets((address,address,uint256,uint256,uint256,bytes32,bytes)[] _tickets, bytes[] _sigs, uint256[] _recipientRands) returns()
func (_TicketBroker *TicketBrokerTransactorSession) BatchRedeemWinningTickets(_tickets []MTicketBrokerCoreTicket, _sigs [][]byte, _recipientRands []*big.Int) (*types.Transaction, error) {
	return _TicketBroker.Contract.BatchRedeemWinningTickets(&_TicketBroker.TransactOpts, _tickets, _sigs, _recipientRands)
}

// CancelUnlock is a paid mutator transaction binding the contract method 0xc2c4c2c8.
//
// Solidity: function cancelUnlock() returns()
func (_TicketBroker *TicketBrokerTransactor) CancelUnlock(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _TicketBroker.contract.Transact(opts, "cancelUnlock")
}

// CancelUnlock is a paid mutator transaction binding the contract method 0xc2c4c2c8.
//
// Solidity: function cancelUnlock() returns()
func (_TicketBroker *TicketBrokerSession) CancelUnlock() (*types.Transaction, error) {
	return _TicketBroker.Contract.CancelUnlock(&_TicketBroker.TransactOpts)
}

// CancelUnlock is a paid mutator transaction binding the contract method 0xc2c4c2c8.
//
// Solidity: function cancelUnlock() returns()
func (_TicketBroker *TicketBrokerTransactorSession) CancelUnlock() (*types.Transaction, error) {
	return _TicketBroker.Contract.CancelUnlock(&_TicketBroker.TransactOpts)
}

// FundDeposit is a paid mutator transaction binding the contract method 0x6caa736b.
//
// Solidity: function fundDeposit() payable returns()
func (_TicketBroker *TicketBrokerTransactor) FundDeposit(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _TicketBroker.contract.Transact(opts, "fundDeposit")
}

// FundDeposit is a paid mutator transaction binding the contract method 0x6caa736b.
//
// Solidity: function fundDeposit() payable returns()
func (_TicketBroker *TicketBrokerSession) FundDeposit() (*types.Transaction, error) {
	return _TicketBroker.Contract.FundDeposit(&_TicketBroker.TransactOpts)
}

// FundDeposit is a paid mutator transaction binding the contract method 0x6caa736b.
//
// Solidity: function fundDeposit() payable returns()
func (_TicketBroker *TicketBrokerTransactorSession) FundDeposit() (*types.Transaction, error) {
	return _TicketBroker.Contract.FundDeposit(&_TicketBroker.TransactOpts)
}

// FundDepositAndReserve is a paid mutator transaction binding the contract method 0x511f4073.
//
// Solidity: function fundDepositAndReserve(uint256 _depositAmount, uint256 _reserveAmount) payable returns()
func (_TicketBroker *TicketBrokerTransactor) FundDepositAndReserve(opts *bind.TransactOpts, _depositAmount *big.Int, _reserveAmount *big.Int) (*types.Transaction, error) {
	return _TicketBroker.contract.Transact(opts, "fundDepositAndReserve", _depositAmount, _reserveAmount)
}

// FundDepositAndReserve is a paid mutator transaction binding the contract method 0x511f4073.
//
// Solidity: function fundDepositAndReserve(uint256 _depositAmount, uint256 _reserveAmount) payable returns()
func (_TicketBroker *TicketBrokerSession) FundDepositAndReserve(_depositAmount *big.Int, _reserveAmount *big.Int) (*types.Transaction, error) {
	return _TicketBroker.Contract.FundDepositAndReserve(&_TicketBroker.TransactOpts, _depositAmount, _reserveAmount)
}

// FundDepositAndReserve is a paid mutator transaction binding the contract method 0x511f4073.
//
// Solidity: function fundDepositAndReserve(uint256 _depositAmount, uint256 _reserveAmount) payable returns()
func (_TicketBroker *TicketBrokerTransactorSession) FundDepositAndReserve(_depositAmount *big.Int, _reserveAmount *big.Int) (*types.Transaction, error) {
	return _TicketBroker.Contract.FundDepositAndReserve(&_TicketBroker.TransactOpts, _depositAmount, _reserveAmount)
}

// FundDepositAndReserveFor is a paid mutator transaction binding the contract method 0x989f789c.
//
// Solidity: function fundDepositAndReserveFor(address _addr, uint256 _depositAmount, uint256 _reserveAmount) payable returns()
func (_TicketBroker *TicketBrokerTransactor) FundDepositAndReserveFor(opts *bind.TransactOpts, _addr common.Address, _depositAmount *big.Int, _reserveAmount *big.Int) (*types.Transaction, error) {
	return _TicketBroker.contract.Transact(opts, "fundDepositAndReserveFor", _addr, _depositAmount, _reserveAmount)
}

// FundDepositAndReserveFor is a paid mutator transaction binding the contract method 0x989f789c.
//
// Solidity: function fundDepositAndReserveFor(address _addr, uint256 _depositAmount, uint256 _reserveAmount) payable returns()
func (_TicketBroker *TicketBrokerSession) FundDepositAndReserveFor(_addr common.Address, _depositAmount *big.Int, _reserveAmount *big.Int) (*types.Transaction, error) {
	return _TicketBroker.Contract.FundDepositAndReserveFor(&_TicketBroker.TransactOpts, _addr, _depositAmount, _reserveAmount)
}

// FundDepositAndReserveFor is a paid mutator transaction binding the contract method 0x989f789c.
//
// Solidity: function fundDepositAndReserveFor(address _addr, uint256 _depositAmount, uint256 _reserveAmount) payable returns()
func (_TicketBroker *TicketBrokerTransactorSession) FundDepositAndReserveFor(_addr common.Address, _depositAmount *big.Int, _reserveAmount *big.Int) (*types.Transaction, error) {
	return _TicketBroker.Contract.FundDepositAndReserveFor(&_TicketBroker.TransactOpts, _addr, _depositAmount, _reserveAmount)
}

// FundReserve is a paid mutator transaction binding the contract method 0x6f9c3c8f.
//
// Solidity: function fundReserve() payable returns()
func (_TicketBroker *TicketBrokerTransactor) FundReserve(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _TicketBroker.contract.Transact(opts, "fundReserve")
}

// FundReserve is a paid mutator transaction binding the contract method 0x6f9c3c8f.
//
// Solidity: function fundReserve() payable returns()
func (_TicketBroker *TicketBrokerSession) FundReserve() (*types.Transaction, error) {
	return _TicketBroker.Contract.FundReserve(&_TicketBroker.TransactOpts)
}

// FundReserve is a paid mutator transaction binding the contract method 0x6f9c3c8f.
//
// Solidity: function fundReserve() payable returns()
func (_TicketBroker *TicketBrokerTransactorSession) FundReserve() (*types.Transaction, error) {
	return _TicketBroker.Contract.FundReserve(&_TicketBroker.TransactOpts)
}

// RedeemWinningTicket is a paid mutator transaction binding the contract method 0xec8b3cb6.
//
// Solidity: function redeemWinningTicket((address,address,uint256,uint256,uint256,bytes32,bytes) _ticket, bytes _sig, uint256 _recipientRand) returns()
func (_TicketBroker *TicketBrokerTransactor) RedeemWinningTicket(opts *bind.TransactOpts, _ticket MTicketBrokerCoreTicket, _sig []byte, _recipientRand *big.Int) (*types.Transaction, error) {
	return _TicketBroker.contract.Transact(opts, "redeemWinningTicket", _ticket, _sig, _recipientRand)
}

// RedeemWinningTicket is a paid mutator transaction binding the contract method 0xec8b3cb6.
//
// Solidity: function redeemWinningTicket((address,address,uint256,uint256,uint256,bytes32,bytes) _ticket, bytes _sig, uint256 _recipientRand) returns()
func (_TicketBroker *TicketBrokerSession) RedeemWinningTicket(_ticket MTicketBrokerCoreTicket, _sig []byte, _recipientRand *big.Int) (*types.Transaction, error) {
	return _TicketBroker.Contract.RedeemWinningTicket(&_TicketBroker.TransactOpts, _ticket, _sig, _recipientRand)
}

// RedeemWinningTicket is a paid mutator transaction binding the contract method 0xec8b3cb6.
//
// Solidity: function redeemWinningTicket((address,address,uint256,uint256,uint256,bytes32,bytes) _ticket, bytes _sig, uint256 _recipientRand) returns()
func (_TicketBroker *TicketBrokerTransactorSession) RedeemWinningTicket(_ticket MTicketBrokerCoreTicket, _sig []byte, _recipientRand *big.Int) (*types.Transaction, error) {
	return _TicketBroker.Contract.RedeemWinningTicket(&_TicketBroker.TransactOpts, _ticket, _sig, _recipientRand)
}

// SetController is a paid mutator transaction binding the contract method 0x92eefe9b.
//
// Solidity: function setController(address _controller) returns()
func (_TicketBroker *TicketBrokerTransactor) SetController(opts *bind.TransactOpts, _controller common.Address) (*types.Transaction, error) {
	return _TicketBroker.contract.Transact(opts, "setController", _controller)
}

// SetController is a paid mutator transaction binding the contract method 0x92eefe9b.
//
// Solidity: function setController(address _controller) returns()
func (_TicketBroker *TicketBrokerSession) SetController(_controller common.Address) (*types.Transaction, error) {
	return _TicketBroker.Contract.SetController(&_TicketBroker.TransactOpts, _controller)
}

// SetController is a paid mutator transaction binding the contract method 0x92eefe9b.
//
// Solidity: function setController(address _controller) returns()
func (_TicketBroker *TicketBrokerTransactorSession) SetController(_controller common.Address) (*types.Transaction, error) {
	return _TicketBroker.Contract.SetController(&_TicketBroker.TransactOpts, _controller)
}

// SetTicketValidityPeriod is a paid mutator transaction binding the contract method 0xc9297808.
//
// Solidity: function setTicketValidityPeriod(uint256 _ticketValidityPeriod) returns()
func (_TicketBroker *TicketBrokerTransactor) SetTicketValidityPeriod(opts *bind.TransactOpts, _ticketValidityPeriod *big.Int) (*types.Transaction, error) {
	return _TicketBroker.contract.Transact(opts, "setTicketValidityPeriod", _ticketValidityPeriod)
}

// SetTicketValidityPeriod is a paid mutator transaction binding the contract method 0xc9297808.
//
// Solidity: function setTicketValidityPeriod(uint256 _ticketValidityPeriod) returns()
func (_TicketBroker *TicketBrokerSession) SetTicketValidityPeriod(_ticketValidityPeriod *big.Int) (*types.Transaction, error) {
	return _TicketBroker.Contract.SetTicketValidityPeriod(&_TicketBroker.TransactOpts, _ticketValidityPeriod)
}

// SetTicketValidityPeriod is a paid mutator transaction binding the contract method 0xc9297808.
//
// Solidity: function setTicketValidityPeriod(uint256 _ticketValidityPeriod) returns()
func (_TicketBroker *TicketBrokerTransactorSession) SetTicketValidityPeriod(_ticketValidityPeriod *big.Int) (*types.Transaction, error) {
	return _TicketBroker.Contract.SetTicketValidityPeriod(&_TicketBroker.TransactOpts, _ticketValidityPeriod)
}

// SetUnlockPeriod is a paid mutator transaction binding the contract method 0x3d0ddf84.
//
// Solidity: function setUnlockPeriod(uint256 _unlockPeriod) returns()
func (_TicketBroker *TicketBrokerTransactor) SetUnlockPeriod(opts *bind.TransactOpts, _unlockPeriod *big.Int) (*types.Transaction, error) {
	return _TicketBroker.contract.Transact(opts, "setUnlockPeriod", _unlockPeriod)
}

// SetUnlockPeriod is a paid mutator transaction binding the contract method 0x3d0ddf84.
//
// Solidity: function setUnlockPeriod(uint256 _unlockPeriod) returns()
func (_TicketBroker *TicketBrokerSession) SetUnlockPeriod(_unlockPeriod *big.Int) (*types.Transaction, error) {
	return _TicketBroker.Contract.SetUnlockPeriod(&_TicketBroker.TransactOpts, _unlockPeriod)
}

// SetUnlockPeriod is a paid mutator transaction binding the contract method 0x3d0ddf84.
//
// Solidity: function setUnlockPeriod(uint256 _unlockPeriod) returns()
func (_TicketBroker *TicketBrokerTransactorSession) SetUnlockPeriod(_unlockPeriod *big.Int) (*types.Transaction, error) {
	return _TicketBroker.Contract.SetUnlockPeriod(&_TicketBroker.TransactOpts, _unlockPeriod)
}

// Unlock is a paid mutator transaction binding the contract method 0xa69df4b5.
//
// Solidity: function unlock() returns()
func (_TicketBroker *TicketBrokerTransactor) Unlock(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _TicketBroker.contract.Transact(opts, "unlock")
}

// Unlock is a paid mutator transaction binding the contract method 0xa69df4b5.
//
// Solidity: function unlock() returns()
func (_TicketBroker *TicketBrokerSession) Unlock() (*types.Transaction, error) {
	return _TicketBroker.Contract.Unlock(&_TicketBroker.TransactOpts)
}

// Unlock is a paid mutator transaction binding the contract method 0xa69df4b5.
//
// Solidity: function unlock() returns()
func (_TicketBroker *TicketBrokerTransactorSession) Unlock() (*types.Transaction, error) {
	return _TicketBroker.Contract.Unlock(&_TicketBroker.TransactOpts)
}

// Withdraw is a paid mutator transaction binding the contract method 0x3ccfd60b.
//
// Solidity: function withdraw() returns()
func (_TicketBroker *TicketBrokerTransactor) Withdraw(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _TicketBroker.contract.Transact(opts, "withdraw")
}

// Withdraw is a paid mutator transaction binding the contract method 0x3ccfd60b.
//
// Solidity: function withdraw() returns()
func (_TicketBroker *TicketBrokerSession) Withdraw() (*types.Transaction, error) {
	return _TicketBroker.Contract.Withdraw(&_TicketBroker.TransactOpts)
}

// Withdraw is a paid mutator transaction binding the contract method 0x3ccfd60b.
//
// Solidity: function withdraw() returns()
func (_TicketBroker *TicketBrokerTransactorSession) Withdraw() (*types.Transaction, error) {
	return _TicketBroker.Contract.Withdraw(&_TicketBroker.TransactOpts)
}

// TicketBrokerDepositFundedIterator is returned from FilterDepositFunded and is used to iterate over the raw logs and unpacked data for DepositFunded events raised by the TicketBroker contract.
type TicketBrokerDepositFundedIterator struct {
	Event *TicketBrokerDepositFunded // Event containing the contract specifics and raw log

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
func (it *TicketBrokerDepositFundedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(TicketBrokerDepositFunded)
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
		it.Event = new(TicketBrokerDepositFunded)
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
func (it *TicketBrokerDepositFundedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *TicketBrokerDepositFundedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// TicketBrokerDepositFunded represents a DepositFunded event raised by the TicketBroker contract.
type TicketBrokerDepositFunded struct {
	Sender common.Address
	Amount *big.Int
	Raw    types.Log // Blockchain specific contextual infos
}

// FilterDepositFunded is a free log retrieval operation binding the contract event 0x5159e237d952190e68d5215430f305831be7c9c8776d1377c76679ae4773413f.
//
// Solidity: event DepositFunded(address indexed sender, uint256 amount)
func (_TicketBroker *TicketBrokerFilterer) FilterDepositFunded(opts *bind.FilterOpts, sender []common.Address) (*TicketBrokerDepositFundedIterator, error) {

	var senderRule []interface{}
	for _, senderItem := range sender {
		senderRule = append(senderRule, senderItem)
	}

	logs, sub, err := _TicketBroker.contract.FilterLogs(opts, "DepositFunded", senderRule)
	if err != nil {
		return nil, err
	}
	return &TicketBrokerDepositFundedIterator{contract: _TicketBroker.contract, event: "DepositFunded", logs: logs, sub: sub}, nil
}

// WatchDepositFunded is a free log subscription operation binding the contract event 0x5159e237d952190e68d5215430f305831be7c9c8776d1377c76679ae4773413f.
//
// Solidity: event DepositFunded(address indexed sender, uint256 amount)
func (_TicketBroker *TicketBrokerFilterer) WatchDepositFunded(opts *bind.WatchOpts, sink chan<- *TicketBrokerDepositFunded, sender []common.Address) (event.Subscription, error) {

	var senderRule []interface{}
	for _, senderItem := range sender {
		senderRule = append(senderRule, senderItem)
	}

	logs, sub, err := _TicketBroker.contract.WatchLogs(opts, "DepositFunded", senderRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(TicketBrokerDepositFunded)
				if err := _TicketBroker.contract.UnpackLog(event, "DepositFunded", log); err != nil {
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

// ParseDepositFunded is a log parse operation binding the contract event 0x5159e237d952190e68d5215430f305831be7c9c8776d1377c76679ae4773413f.
//
// Solidity: event DepositFunded(address indexed sender, uint256 amount)
func (_TicketBroker *TicketBrokerFilterer) ParseDepositFunded(log types.Log) (*TicketBrokerDepositFunded, error) {
	event := new(TicketBrokerDepositFunded)
	if err := _TicketBroker.contract.UnpackLog(event, "DepositFunded", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// TicketBrokerParameterUpdateIterator is returned from FilterParameterUpdate and is used to iterate over the raw logs and unpacked data for ParameterUpdate events raised by the TicketBroker contract.
type TicketBrokerParameterUpdateIterator struct {
	Event *TicketBrokerParameterUpdate // Event containing the contract specifics and raw log

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
func (it *TicketBrokerParameterUpdateIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(TicketBrokerParameterUpdate)
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
		it.Event = new(TicketBrokerParameterUpdate)
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
func (it *TicketBrokerParameterUpdateIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *TicketBrokerParameterUpdateIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// TicketBrokerParameterUpdate represents a ParameterUpdate event raised by the TicketBroker contract.
type TicketBrokerParameterUpdate struct {
	Param string
	Raw   types.Log // Blockchain specific contextual infos
}

// FilterParameterUpdate is a free log retrieval operation binding the contract event 0x9f5033568d78ae30f29f01e944f97b2216493bd19d1b46d429673acff3dcd674.
//
// Solidity: event ParameterUpdate(string param)
func (_TicketBroker *TicketBrokerFilterer) FilterParameterUpdate(opts *bind.FilterOpts) (*TicketBrokerParameterUpdateIterator, error) {

	logs, sub, err := _TicketBroker.contract.FilterLogs(opts, "ParameterUpdate")
	if err != nil {
		return nil, err
	}
	return &TicketBrokerParameterUpdateIterator{contract: _TicketBroker.contract, event: "ParameterUpdate", logs: logs, sub: sub}, nil
}

// WatchParameterUpdate is a free log subscription operation binding the contract event 0x9f5033568d78ae30f29f01e944f97b2216493bd19d1b46d429673acff3dcd674.
//
// Solidity: event ParameterUpdate(string param)
func (_TicketBroker *TicketBrokerFilterer) WatchParameterUpdate(opts *bind.WatchOpts, sink chan<- *TicketBrokerParameterUpdate) (event.Subscription, error) {

	logs, sub, err := _TicketBroker.contract.WatchLogs(opts, "ParameterUpdate")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(TicketBrokerParameterUpdate)
				if err := _TicketBroker.contract.UnpackLog(event, "ParameterUpdate", log); err != nil {
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

// ParseParameterUpdate is a log parse operation binding the contract event 0x9f5033568d78ae30f29f01e944f97b2216493bd19d1b46d429673acff3dcd674.
//
// Solidity: event ParameterUpdate(string param)
func (_TicketBroker *TicketBrokerFilterer) ParseParameterUpdate(log types.Log) (*TicketBrokerParameterUpdate, error) {
	event := new(TicketBrokerParameterUpdate)
	if err := _TicketBroker.contract.UnpackLog(event, "ParameterUpdate", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// TicketBrokerReserveClaimedIterator is returned from FilterReserveClaimed and is used to iterate over the raw logs and unpacked data for ReserveClaimed events raised by the TicketBroker contract.
type TicketBrokerReserveClaimedIterator struct {
	Event *TicketBrokerReserveClaimed // Event containing the contract specifics and raw log

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
func (it *TicketBrokerReserveClaimedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(TicketBrokerReserveClaimed)
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
		it.Event = new(TicketBrokerReserveClaimed)
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
func (it *TicketBrokerReserveClaimedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *TicketBrokerReserveClaimedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// TicketBrokerReserveClaimed represents a ReserveClaimed event raised by the TicketBroker contract.
type TicketBrokerReserveClaimed struct {
	ReserveHolder common.Address
	Claimant      common.Address
	Amount        *big.Int
	Raw           types.Log // Blockchain specific contextual infos
}

// FilterReserveClaimed is a free log retrieval operation binding the contract event 0x5c2b394723f408a40a60335e24b71829642e35f350cebe2036a96a66e895ea98.
//
// Solidity: event ReserveClaimed(address indexed reserveHolder, address claimant, uint256 amount)
func (_TicketBroker *TicketBrokerFilterer) FilterReserveClaimed(opts *bind.FilterOpts, reserveHolder []common.Address) (*TicketBrokerReserveClaimedIterator, error) {

	var reserveHolderRule []interface{}
	for _, reserveHolderItem := range reserveHolder {
		reserveHolderRule = append(reserveHolderRule, reserveHolderItem)
	}

	logs, sub, err := _TicketBroker.contract.FilterLogs(opts, "ReserveClaimed", reserveHolderRule)
	if err != nil {
		return nil, err
	}
	return &TicketBrokerReserveClaimedIterator{contract: _TicketBroker.contract, event: "ReserveClaimed", logs: logs, sub: sub}, nil
}

// WatchReserveClaimed is a free log subscription operation binding the contract event 0x5c2b394723f408a40a60335e24b71829642e35f350cebe2036a96a66e895ea98.
//
// Solidity: event ReserveClaimed(address indexed reserveHolder, address claimant, uint256 amount)
func (_TicketBroker *TicketBrokerFilterer) WatchReserveClaimed(opts *bind.WatchOpts, sink chan<- *TicketBrokerReserveClaimed, reserveHolder []common.Address) (event.Subscription, error) {

	var reserveHolderRule []interface{}
	for _, reserveHolderItem := range reserveHolder {
		reserveHolderRule = append(reserveHolderRule, reserveHolderItem)
	}

	logs, sub, err := _TicketBroker.contract.WatchLogs(opts, "ReserveClaimed", reserveHolderRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(TicketBrokerReserveClaimed)
				if err := _TicketBroker.contract.UnpackLog(event, "ReserveClaimed", log); err != nil {
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

// ParseReserveClaimed is a log parse operation binding the contract event 0x5c2b394723f408a40a60335e24b71829642e35f350cebe2036a96a66e895ea98.
//
// Solidity: event ReserveClaimed(address indexed reserveHolder, address claimant, uint256 amount)
func (_TicketBroker *TicketBrokerFilterer) ParseReserveClaimed(log types.Log) (*TicketBrokerReserveClaimed, error) {
	event := new(TicketBrokerReserveClaimed)
	if err := _TicketBroker.contract.UnpackLog(event, "ReserveClaimed", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// TicketBrokerReserveFundedIterator is returned from FilterReserveFunded and is used to iterate over the raw logs and unpacked data for ReserveFunded events raised by the TicketBroker contract.
type TicketBrokerReserveFundedIterator struct {
	Event *TicketBrokerReserveFunded // Event containing the contract specifics and raw log

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
func (it *TicketBrokerReserveFundedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(TicketBrokerReserveFunded)
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
		it.Event = new(TicketBrokerReserveFunded)
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
func (it *TicketBrokerReserveFundedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *TicketBrokerReserveFundedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// TicketBrokerReserveFunded represents a ReserveFunded event raised by the TicketBroker contract.
type TicketBrokerReserveFunded struct {
	ReserveHolder common.Address
	Amount        *big.Int
	Raw           types.Log // Blockchain specific contextual infos
}

// FilterReserveFunded is a free log retrieval operation binding the contract event 0xb52b99b9e83551fcbd069b559cc3e823e2a1a3bad8ece46561ea77524394c850.
//
// Solidity: event ReserveFunded(address indexed reserveHolder, uint256 amount)
func (_TicketBroker *TicketBrokerFilterer) FilterReserveFunded(opts *bind.FilterOpts, reserveHolder []common.Address) (*TicketBrokerReserveFundedIterator, error) {

	var reserveHolderRule []interface{}
	for _, reserveHolderItem := range reserveHolder {
		reserveHolderRule = append(reserveHolderRule, reserveHolderItem)
	}

	logs, sub, err := _TicketBroker.contract.FilterLogs(opts, "ReserveFunded", reserveHolderRule)
	if err != nil {
		return nil, err
	}
	return &TicketBrokerReserveFundedIterator{contract: _TicketBroker.contract, event: "ReserveFunded", logs: logs, sub: sub}, nil
}

// WatchReserveFunded is a free log subscription operation binding the contract event 0xb52b99b9e83551fcbd069b559cc3e823e2a1a3bad8ece46561ea77524394c850.
//
// Solidity: event ReserveFunded(address indexed reserveHolder, uint256 amount)
func (_TicketBroker *TicketBrokerFilterer) WatchReserveFunded(opts *bind.WatchOpts, sink chan<- *TicketBrokerReserveFunded, reserveHolder []common.Address) (event.Subscription, error) {

	var reserveHolderRule []interface{}
	for _, reserveHolderItem := range reserveHolder {
		reserveHolderRule = append(reserveHolderRule, reserveHolderItem)
	}

	logs, sub, err := _TicketBroker.contract.WatchLogs(opts, "ReserveFunded", reserveHolderRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(TicketBrokerReserveFunded)
				if err := _TicketBroker.contract.UnpackLog(event, "ReserveFunded", log); err != nil {
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

// ParseReserveFunded is a log parse operation binding the contract event 0xb52b99b9e83551fcbd069b559cc3e823e2a1a3bad8ece46561ea77524394c850.
//
// Solidity: event ReserveFunded(address indexed reserveHolder, uint256 amount)
func (_TicketBroker *TicketBrokerFilterer) ParseReserveFunded(log types.Log) (*TicketBrokerReserveFunded, error) {
	event := new(TicketBrokerReserveFunded)
	if err := _TicketBroker.contract.UnpackLog(event, "ReserveFunded", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// TicketBrokerSetControllerIterator is returned from FilterSetController and is used to iterate over the raw logs and unpacked data for SetController events raised by the TicketBroker contract.
type TicketBrokerSetControllerIterator struct {
	Event *TicketBrokerSetController // Event containing the contract specifics and raw log

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
func (it *TicketBrokerSetControllerIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(TicketBrokerSetController)
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
		it.Event = new(TicketBrokerSetController)
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
func (it *TicketBrokerSetControllerIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *TicketBrokerSetControllerIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// TicketBrokerSetController represents a SetController event raised by the TicketBroker contract.
type TicketBrokerSetController struct {
	Controller common.Address
	Raw        types.Log // Blockchain specific contextual infos
}

// FilterSetController is a free log retrieval operation binding the contract event 0x4ff638452bbf33c012645d18ae6f05515ff5f2d1dfb0cece8cbf018c60903f70.
//
// Solidity: event SetController(address controller)
func (_TicketBroker *TicketBrokerFilterer) FilterSetController(opts *bind.FilterOpts) (*TicketBrokerSetControllerIterator, error) {

	logs, sub, err := _TicketBroker.contract.FilterLogs(opts, "SetController")
	if err != nil {
		return nil, err
	}
	return &TicketBrokerSetControllerIterator{contract: _TicketBroker.contract, event: "SetController", logs: logs, sub: sub}, nil
}

// WatchSetController is a free log subscription operation binding the contract event 0x4ff638452bbf33c012645d18ae6f05515ff5f2d1dfb0cece8cbf018c60903f70.
//
// Solidity: event SetController(address controller)
func (_TicketBroker *TicketBrokerFilterer) WatchSetController(opts *bind.WatchOpts, sink chan<- *TicketBrokerSetController) (event.Subscription, error) {

	logs, sub, err := _TicketBroker.contract.WatchLogs(opts, "SetController")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(TicketBrokerSetController)
				if err := _TicketBroker.contract.UnpackLog(event, "SetController", log); err != nil {
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

// ParseSetController is a log parse operation binding the contract event 0x4ff638452bbf33c012645d18ae6f05515ff5f2d1dfb0cece8cbf018c60903f70.
//
// Solidity: event SetController(address controller)
func (_TicketBroker *TicketBrokerFilterer) ParseSetController(log types.Log) (*TicketBrokerSetController, error) {
	event := new(TicketBrokerSetController)
	if err := _TicketBroker.contract.UnpackLog(event, "SetController", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// TicketBrokerUnlockIterator is returned from FilterUnlock and is used to iterate over the raw logs and unpacked data for Unlock events raised by the TicketBroker contract.
type TicketBrokerUnlockIterator struct {
	Event *TicketBrokerUnlock // Event containing the contract specifics and raw log

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
func (it *TicketBrokerUnlockIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(TicketBrokerUnlock)
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
		it.Event = new(TicketBrokerUnlock)
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
func (it *TicketBrokerUnlockIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *TicketBrokerUnlockIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// TicketBrokerUnlock represents a Unlock event raised by the TicketBroker contract.
type TicketBrokerUnlock struct {
	Sender     common.Address
	StartRound *big.Int
	EndRound   *big.Int
	Raw        types.Log // Blockchain specific contextual infos
}

// FilterUnlock is a free log retrieval operation binding the contract event 0xf7870c5b224cbc19873599e46ccfc7103934650509b1af0c3ce90138377c2004.
//
// Solidity: event Unlock(address indexed sender, uint256 startRound, uint256 endRound)
func (_TicketBroker *TicketBrokerFilterer) FilterUnlock(opts *bind.FilterOpts, sender []common.Address) (*TicketBrokerUnlockIterator, error) {

	var senderRule []interface{}
	for _, senderItem := range sender {
		senderRule = append(senderRule, senderItem)
	}

	logs, sub, err := _TicketBroker.contract.FilterLogs(opts, "Unlock", senderRule)
	if err != nil {
		return nil, err
	}
	return &TicketBrokerUnlockIterator{contract: _TicketBroker.contract, event: "Unlock", logs: logs, sub: sub}, nil
}

// WatchUnlock is a free log subscription operation binding the contract event 0xf7870c5b224cbc19873599e46ccfc7103934650509b1af0c3ce90138377c2004.
//
// Solidity: event Unlock(address indexed sender, uint256 startRound, uint256 endRound)
func (_TicketBroker *TicketBrokerFilterer) WatchUnlock(opts *bind.WatchOpts, sink chan<- *TicketBrokerUnlock, sender []common.Address) (event.Subscription, error) {

	var senderRule []interface{}
	for _, senderItem := range sender {
		senderRule = append(senderRule, senderItem)
	}

	logs, sub, err := _TicketBroker.contract.WatchLogs(opts, "Unlock", senderRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(TicketBrokerUnlock)
				if err := _TicketBroker.contract.UnpackLog(event, "Unlock", log); err != nil {
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

// ParseUnlock is a log parse operation binding the contract event 0xf7870c5b224cbc19873599e46ccfc7103934650509b1af0c3ce90138377c2004.
//
// Solidity: event Unlock(address indexed sender, uint256 startRound, uint256 endRound)
func (_TicketBroker *TicketBrokerFilterer) ParseUnlock(log types.Log) (*TicketBrokerUnlock, error) {
	event := new(TicketBrokerUnlock)
	if err := _TicketBroker.contract.UnpackLog(event, "Unlock", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// TicketBrokerUnlockCancelledIterator is returned from FilterUnlockCancelled and is used to iterate over the raw logs and unpacked data for UnlockCancelled events raised by the TicketBroker contract.
type TicketBrokerUnlockCancelledIterator struct {
	Event *TicketBrokerUnlockCancelled // Event containing the contract specifics and raw log

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
func (it *TicketBrokerUnlockCancelledIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(TicketBrokerUnlockCancelled)
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
		it.Event = new(TicketBrokerUnlockCancelled)
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
func (it *TicketBrokerUnlockCancelledIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *TicketBrokerUnlockCancelledIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// TicketBrokerUnlockCancelled represents a UnlockCancelled event raised by the TicketBroker contract.
type TicketBrokerUnlockCancelled struct {
	Sender common.Address
	Raw    types.Log // Blockchain specific contextual infos
}

// FilterUnlockCancelled is a free log retrieval operation binding the contract event 0xfa044b7b93a40365dc68049797c2eb06918523d694e5d56e406cac3eb35578e5.
//
// Solidity: event UnlockCancelled(address indexed sender)
func (_TicketBroker *TicketBrokerFilterer) FilterUnlockCancelled(opts *bind.FilterOpts, sender []common.Address) (*TicketBrokerUnlockCancelledIterator, error) {

	var senderRule []interface{}
	for _, senderItem := range sender {
		senderRule = append(senderRule, senderItem)
	}

	logs, sub, err := _TicketBroker.contract.FilterLogs(opts, "UnlockCancelled", senderRule)
	if err != nil {
		return nil, err
	}
	return &TicketBrokerUnlockCancelledIterator{contract: _TicketBroker.contract, event: "UnlockCancelled", logs: logs, sub: sub}, nil
}

// WatchUnlockCancelled is a free log subscription operation binding the contract event 0xfa044b7b93a40365dc68049797c2eb06918523d694e5d56e406cac3eb35578e5.
//
// Solidity: event UnlockCancelled(address indexed sender)
func (_TicketBroker *TicketBrokerFilterer) WatchUnlockCancelled(opts *bind.WatchOpts, sink chan<- *TicketBrokerUnlockCancelled, sender []common.Address) (event.Subscription, error) {

	var senderRule []interface{}
	for _, senderItem := range sender {
		senderRule = append(senderRule, senderItem)
	}

	logs, sub, err := _TicketBroker.contract.WatchLogs(opts, "UnlockCancelled", senderRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(TicketBrokerUnlockCancelled)
				if err := _TicketBroker.contract.UnpackLog(event, "UnlockCancelled", log); err != nil {
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

// ParseUnlockCancelled is a log parse operation binding the contract event 0xfa044b7b93a40365dc68049797c2eb06918523d694e5d56e406cac3eb35578e5.
//
// Solidity: event UnlockCancelled(address indexed sender)
func (_TicketBroker *TicketBrokerFilterer) ParseUnlockCancelled(log types.Log) (*TicketBrokerUnlockCancelled, error) {
	event := new(TicketBrokerUnlockCancelled)
	if err := _TicketBroker.contract.UnpackLog(event, "UnlockCancelled", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// TicketBrokerWinningTicketRedeemedIterator is returned from FilterWinningTicketRedeemed and is used to iterate over the raw logs and unpacked data for WinningTicketRedeemed events raised by the TicketBroker contract.
type TicketBrokerWinningTicketRedeemedIterator struct {
	Event *TicketBrokerWinningTicketRedeemed // Event containing the contract specifics and raw log

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
func (it *TicketBrokerWinningTicketRedeemedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(TicketBrokerWinningTicketRedeemed)
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
		it.Event = new(TicketBrokerWinningTicketRedeemed)
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
func (it *TicketBrokerWinningTicketRedeemedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *TicketBrokerWinningTicketRedeemedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// TicketBrokerWinningTicketRedeemed represents a WinningTicketRedeemed event raised by the TicketBroker contract.
type TicketBrokerWinningTicketRedeemed struct {
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
func (_TicketBroker *TicketBrokerFilterer) FilterWinningTicketRedeemed(opts *bind.FilterOpts, sender []common.Address, recipient []common.Address) (*TicketBrokerWinningTicketRedeemedIterator, error) {

	var senderRule []interface{}
	for _, senderItem := range sender {
		senderRule = append(senderRule, senderItem)
	}
	var recipientRule []interface{}
	for _, recipientItem := range recipient {
		recipientRule = append(recipientRule, recipientItem)
	}

	logs, sub, err := _TicketBroker.contract.FilterLogs(opts, "WinningTicketRedeemed", senderRule, recipientRule)
	if err != nil {
		return nil, err
	}
	return &TicketBrokerWinningTicketRedeemedIterator{contract: _TicketBroker.contract, event: "WinningTicketRedeemed", logs: logs, sub: sub}, nil
}

// WatchWinningTicketRedeemed is a free log subscription operation binding the contract event 0xc389eb51ed006dbf2528507f010efdf5225ea596e1e1741d74f550dab1925ee7.
//
// Solidity: event WinningTicketRedeemed(address indexed sender, address indexed recipient, uint256 faceValue, uint256 winProb, uint256 senderNonce, uint256 recipientRand, bytes auxData)
func (_TicketBroker *TicketBrokerFilterer) WatchWinningTicketRedeemed(opts *bind.WatchOpts, sink chan<- *TicketBrokerWinningTicketRedeemed, sender []common.Address, recipient []common.Address) (event.Subscription, error) {

	var senderRule []interface{}
	for _, senderItem := range sender {
		senderRule = append(senderRule, senderItem)
	}
	var recipientRule []interface{}
	for _, recipientItem := range recipient {
		recipientRule = append(recipientRule, recipientItem)
	}

	logs, sub, err := _TicketBroker.contract.WatchLogs(opts, "WinningTicketRedeemed", senderRule, recipientRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(TicketBrokerWinningTicketRedeemed)
				if err := _TicketBroker.contract.UnpackLog(event, "WinningTicketRedeemed", log); err != nil {
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

// ParseWinningTicketRedeemed is a log parse operation binding the contract event 0xc389eb51ed006dbf2528507f010efdf5225ea596e1e1741d74f550dab1925ee7.
//
// Solidity: event WinningTicketRedeemed(address indexed sender, address indexed recipient, uint256 faceValue, uint256 winProb, uint256 senderNonce, uint256 recipientRand, bytes auxData)
func (_TicketBroker *TicketBrokerFilterer) ParseWinningTicketRedeemed(log types.Log) (*TicketBrokerWinningTicketRedeemed, error) {
	event := new(TicketBrokerWinningTicketRedeemed)
	if err := _TicketBroker.contract.UnpackLog(event, "WinningTicketRedeemed", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// TicketBrokerWinningTicketTransferIterator is returned from FilterWinningTicketTransfer and is used to iterate over the raw logs and unpacked data for WinningTicketTransfer events raised by the TicketBroker contract.
type TicketBrokerWinningTicketTransferIterator struct {
	Event *TicketBrokerWinningTicketTransfer // Event containing the contract specifics and raw log

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
func (it *TicketBrokerWinningTicketTransferIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(TicketBrokerWinningTicketTransfer)
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
		it.Event = new(TicketBrokerWinningTicketTransfer)
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
func (it *TicketBrokerWinningTicketTransferIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *TicketBrokerWinningTicketTransferIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// TicketBrokerWinningTicketTransfer represents a WinningTicketTransfer event raised by the TicketBroker contract.
type TicketBrokerWinningTicketTransfer struct {
	Sender    common.Address
	Recipient common.Address
	Amount    *big.Int
	Raw       types.Log // Blockchain specific contextual infos
}

// FilterWinningTicketTransfer is a free log retrieval operation binding the contract event 0x8b87351a208c06e3ceee59d80725fd77a23b4129e1b51ca231fc89b40712649c.
//
// Solidity: event WinningTicketTransfer(address indexed sender, address indexed recipient, uint256 amount)
func (_TicketBroker *TicketBrokerFilterer) FilterWinningTicketTransfer(opts *bind.FilterOpts, sender []common.Address, recipient []common.Address) (*TicketBrokerWinningTicketTransferIterator, error) {

	var senderRule []interface{}
	for _, senderItem := range sender {
		senderRule = append(senderRule, senderItem)
	}
	var recipientRule []interface{}
	for _, recipientItem := range recipient {
		recipientRule = append(recipientRule, recipientItem)
	}

	logs, sub, err := _TicketBroker.contract.FilterLogs(opts, "WinningTicketTransfer", senderRule, recipientRule)
	if err != nil {
		return nil, err
	}
	return &TicketBrokerWinningTicketTransferIterator{contract: _TicketBroker.contract, event: "WinningTicketTransfer", logs: logs, sub: sub}, nil
}

// WatchWinningTicketTransfer is a free log subscription operation binding the contract event 0x8b87351a208c06e3ceee59d80725fd77a23b4129e1b51ca231fc89b40712649c.
//
// Solidity: event WinningTicketTransfer(address indexed sender, address indexed recipient, uint256 amount)
func (_TicketBroker *TicketBrokerFilterer) WatchWinningTicketTransfer(opts *bind.WatchOpts, sink chan<- *TicketBrokerWinningTicketTransfer, sender []common.Address, recipient []common.Address) (event.Subscription, error) {

	var senderRule []interface{}
	for _, senderItem := range sender {
		senderRule = append(senderRule, senderItem)
	}
	var recipientRule []interface{}
	for _, recipientItem := range recipient {
		recipientRule = append(recipientRule, recipientItem)
	}

	logs, sub, err := _TicketBroker.contract.WatchLogs(opts, "WinningTicketTransfer", senderRule, recipientRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(TicketBrokerWinningTicketTransfer)
				if err := _TicketBroker.contract.UnpackLog(event, "WinningTicketTransfer", log); err != nil {
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

// ParseWinningTicketTransfer is a log parse operation binding the contract event 0x8b87351a208c06e3ceee59d80725fd77a23b4129e1b51ca231fc89b40712649c.
//
// Solidity: event WinningTicketTransfer(address indexed sender, address indexed recipient, uint256 amount)
func (_TicketBroker *TicketBrokerFilterer) ParseWinningTicketTransfer(log types.Log) (*TicketBrokerWinningTicketTransfer, error) {
	event := new(TicketBrokerWinningTicketTransfer)
	if err := _TicketBroker.contract.UnpackLog(event, "WinningTicketTransfer", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// TicketBrokerWithdrawalIterator is returned from FilterWithdrawal and is used to iterate over the raw logs and unpacked data for Withdrawal events raised by the TicketBroker contract.
type TicketBrokerWithdrawalIterator struct {
	Event *TicketBrokerWithdrawal // Event containing the contract specifics and raw log

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
func (it *TicketBrokerWithdrawalIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(TicketBrokerWithdrawal)
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
		it.Event = new(TicketBrokerWithdrawal)
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
func (it *TicketBrokerWithdrawalIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *TicketBrokerWithdrawalIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// TicketBrokerWithdrawal represents a Withdrawal event raised by the TicketBroker contract.
type TicketBrokerWithdrawal struct {
	Sender  common.Address
	Deposit *big.Int
	Reserve *big.Int
	Raw     types.Log // Blockchain specific contextual infos
}

// FilterWithdrawal is a free log retrieval operation binding the contract event 0xdf273cb619d95419a9cd0ec88123a0538c85064229baa6363788f743fff90deb.
//
// Solidity: event Withdrawal(address indexed sender, uint256 deposit, uint256 reserve)
func (_TicketBroker *TicketBrokerFilterer) FilterWithdrawal(opts *bind.FilterOpts, sender []common.Address) (*TicketBrokerWithdrawalIterator, error) {

	var senderRule []interface{}
	for _, senderItem := range sender {
		senderRule = append(senderRule, senderItem)
	}

	logs, sub, err := _TicketBroker.contract.FilterLogs(opts, "Withdrawal", senderRule)
	if err != nil {
		return nil, err
	}
	return &TicketBrokerWithdrawalIterator{contract: _TicketBroker.contract, event: "Withdrawal", logs: logs, sub: sub}, nil
}

// WatchWithdrawal is a free log subscription operation binding the contract event 0xdf273cb619d95419a9cd0ec88123a0538c85064229baa6363788f743fff90deb.
//
// Solidity: event Withdrawal(address indexed sender, uint256 deposit, uint256 reserve)
func (_TicketBroker *TicketBrokerFilterer) WatchWithdrawal(opts *bind.WatchOpts, sink chan<- *TicketBrokerWithdrawal, sender []common.Address) (event.Subscription, error) {

	var senderRule []interface{}
	for _, senderItem := range sender {
		senderRule = append(senderRule, senderItem)
	}

	logs, sub, err := _TicketBroker.contract.WatchLogs(opts, "Withdrawal", senderRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(TicketBrokerWithdrawal)
				if err := _TicketBroker.contract.UnpackLog(event, "Withdrawal", log); err != nil {
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

// ParseWithdrawal is a log parse operation binding the contract event 0xdf273cb619d95419a9cd0ec88123a0538c85064229baa6363788f743fff90deb.
//
// Solidity: event Withdrawal(address indexed sender, uint256 deposit, uint256 reserve)
func (_TicketBroker *TicketBrokerFilterer) ParseWithdrawal(log types.Log) (*TicketBrokerWithdrawal, error) {
	event := new(TicketBrokerWithdrawal)
	if err := _TicketBroker.contract.UnpackLog(event, "Withdrawal", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}
