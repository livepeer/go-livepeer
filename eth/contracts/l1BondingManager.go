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

// L1BondingManagerMetaData contains all meta data concerning the L1BondingManager contract.
var L1BondingManagerMetaData = &bind.MetaData{
	ABI: "[{\"constant\":true,\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"activeTranscoderSetDEPRECATED\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"totalStake\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"maxEarningsClaimsRounds\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"internalType\":\"address\",\"name\":\"_to\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"_unbondingLockId\",\"type\":\"uint256\"},{\"internalType\":\"address\",\"name\":\"_newPosPrev\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"_newPosNext\",\"type\":\"address\"}],\"name\":\"rebondFromUnbondedWithHint\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"internalType\":\"address\",\"name\":\"_transcoder\",\"type\":\"address\"}],\"name\":\"isActiveTranscoder\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"internalType\":\"address\",\"name\":\"_delegator\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"_unbondingLockId\",\"type\":\"uint256\"}],\"name\":\"isValidUnbondingLock\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"internalType\":\"address\",\"name\":\"_delegator\",\"type\":\"address\"}],\"name\":\"delegatorStatus\",\"outputs\":[{\"internalType\":\"enumBondingManager.DelegatorStatus\",\"name\":\"\",\"type\":\"uint8\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[],\"name\":\"reward\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"internalType\":\"address\",\"name\":\"_transcoder\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"_finder\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"_slashAmount\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"_finderFee\",\"type\":\"uint256\"}],\"name\":\"slashTranscoder\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"internalType\":\"address\",\"name\":\"_transcoder\",\"type\":\"address\"}],\"name\":\"getNextTranscoderInPool\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"internalType\":\"address\",\"name\":\"_transcoder\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"_round\",\"type\":\"uint256\"}],\"name\":\"getTranscoderEarningsPoolForRound\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"rewardPool\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"feePool\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"totalStake\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"claimableStake\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"transcoderRewardCut\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"transcoderFeeShare\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"transcoderRewardPool\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"transcoderFeePool\",\"type\":\"uint256\"},{\"internalType\":\"bool\",\"name\":\"hasTranscoderRewardFeePool\",\"type\":\"bool\"},{\"internalType\":\"uint256\",\"name\":\"cumulativeRewardFactor\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"cumulativeFeeFactor\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_endRound\",\"type\":\"uint256\"}],\"name\":\"claimEarnings\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_unbondingLockId\",\"type\":\"uint256\"}],\"name\":\"withdrawStake\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_amount\",\"type\":\"uint256\"}],\"name\":\"unbond\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"getTranscoderPoolSize\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_bondedAmount\",\"type\":\"uint256\"}],\"name\":\"executeLIP77\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_rewardCut\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"_feeShare\",\"type\":\"uint256\"},{\"internalType\":\"address\",\"name\":\"_newPosPrev\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"_newPosNext\",\"type\":\"address\"}],\"name\":\"transcoderWithHint\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"internalType\":\"address\",\"name\":\"_to\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"_unbondingLockId\",\"type\":\"uint256\"}],\"name\":\"rebondFromUnbonded\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"internalType\":\"address\",\"name\":\"_transcoder\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"_fees\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"_round\",\"type\":\"uint256\"}],\"name\":\"updateTranscoderWithFees\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"numActiveTranscodersDEPRECATED\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"internalType\":\"address\",\"name\":\"_delegator\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"_unbondingLockId\",\"type\":\"uint256\"}],\"name\":\"getDelegatorUnbondingLock\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"withdrawRound\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"currentRoundTotalActiveStake\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_rewardCut\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"_feeShare\",\"type\":\"uint256\"}],\"name\":\"transcoder\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"nextRoundTotalActiveStake\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[],\"name\":\"withdrawFees\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"targetContractId\",\"outputs\":[{\"internalType\":\"bytes32\",\"name\":\"\",\"type\":\"bytes32\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"getTranscoderPoolMaxSize\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"getTotalBonded\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"internalType\":\"address\",\"name\":\"_transcoder\",\"type\":\"address\"}],\"name\":\"getTranscoder\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"lastRewardRound\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"rewardCut\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"feeShare\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"lastActiveStakeUpdateRound\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"activationRound\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"deactivationRound\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"activeCumulativeRewards\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"cumulativeRewards\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"cumulativeFees\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"lastFeeRound\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_numActiveTranscoders\",\"type\":\"uint256\"}],\"name\":\"setNumActiveTranscoders\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"internalType\":\"address\",\"name\":\"_transcoder\",\"type\":\"address\"}],\"name\":\"isRegisteredTranscoder\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_amount\",\"type\":\"uint256\"},{\"internalType\":\"address\",\"name\":\"_to\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"_oldDelegateNewPosPrev\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"_oldDelegateNewPosNext\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"_currDelegateNewPosPrev\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"_currDelegateNewPosNext\",\"type\":\"address\"}],\"name\":\"bondWithHint\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"unbondingPeriod\",\"outputs\":[{\"internalType\":\"uint64\",\"name\":\"\",\"type\":\"uint64\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[],\"name\":\"setCurrentRoundTotalActiveStake\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_maxEarningsClaimsRounds\",\"type\":\"uint256\"}],\"name\":\"setMaxEarningsClaimsRounds\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_unbondingLockId\",\"type\":\"uint256\"},{\"internalType\":\"address\",\"name\":\"_newPosPrev\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"_newPosNext\",\"type\":\"address\"}],\"name\":\"rebondWithHint\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"internalType\":\"address\",\"name\":\"_newPosPrev\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"_newPosNext\",\"type\":\"address\"}],\"name\":\"rewardWithHint\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"getFirstTranscoderInPool\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"internalType\":\"address\",\"name\":\"_transcoder\",\"type\":\"address\"}],\"name\":\"transcoderStatus\",\"outputs\":[{\"internalType\":\"enumBondingManager.TranscoderStatus\",\"name\":\"\",\"type\":\"uint8\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"internalType\":\"address\",\"name\":\"_controller\",\"type\":\"address\"}],\"name\":\"setController\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_amount\",\"type\":\"uint256\"},{\"internalType\":\"address\",\"name\":\"_newPosPrev\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"_newPosNext\",\"type\":\"address\"}],\"name\":\"unbondWithHint\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"internalType\":\"address\",\"name\":\"_delegator\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"_endRound\",\"type\":\"uint256\"}],\"name\":\"pendingStake\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"internalType\":\"address\",\"name\":\"_transcoder\",\"type\":\"address\"}],\"name\":\"transcoderTotalStake\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"internalType\":\"address\",\"name\":\"_delegator\",\"type\":\"address\"}],\"name\":\"getDelegator\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"bondedAmount\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"fees\",\"type\":\"uint256\"},{\"internalType\":\"address\",\"name\":\"delegateAddress\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"delegatedAmount\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"startRound\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"lastClaimRound\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"nextUnbondingLockId\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_amount\",\"type\":\"uint256\"},{\"internalType\":\"address\",\"name\":\"_to\",\"type\":\"address\"}],\"name\":\"bond\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_pendingStake\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"_pendingFees\",\"type\":\"uint256\"},{\"internalType\":\"bytes32[]\",\"name\":\"_earningsProof\",\"type\":\"bytes32[]\"},{\"internalType\":\"bytes\",\"name\":\"_data\",\"type\":\"bytes\"}],\"name\":\"claimSnapshotEarnings\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_unbondingLockId\",\"type\":\"uint256\"}],\"name\":\"rebond\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"internalType\":\"uint64\",\"name\":\"_unbondingPeriod\",\"type\":\"uint64\"}],\"name\":\"setUnbondingPeriod\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"internalType\":\"address\",\"name\":\"_delegator\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"_endRound\",\"type\":\"uint256\"}],\"name\":\"pendingFees\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"controller\",\"outputs\":[{\"internalType\":\"contractIController\",\"name\":\"\",\"type\":\"address\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"_controller\",\"type\":\"address\"}],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"constructor\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"transcoder\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"rewardCut\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"feeShare\",\"type\":\"uint256\"}],\"name\":\"TranscoderUpdate\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"transcoder\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"activationRound\",\"type\":\"uint256\"}],\"name\":\"TranscoderActivated\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"transcoder\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"deactivationRound\",\"type\":\"uint256\"}],\"name\":\"TranscoderDeactivated\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"transcoder\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"address\",\"name\":\"finder\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"penalty\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"finderReward\",\"type\":\"uint256\"}],\"name\":\"TranscoderSlashed\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"transcoder\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"}],\"name\":\"Reward\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"newDelegate\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"oldDelegate\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"delegator\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"additionalAmount\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"bondedAmount\",\"type\":\"uint256\"}],\"name\":\"Bond\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"delegate\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"delegator\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"unbondingLockId\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"withdrawRound\",\"type\":\"uint256\"}],\"name\":\"Unbond\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"delegate\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"delegator\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"unbondingLockId\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"}],\"name\":\"Rebond\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"delegator\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"unbondingLockId\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"withdrawRound\",\"type\":\"uint256\"}],\"name\":\"WithdrawStake\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"delegator\",\"type\":\"address\"}],\"name\":\"WithdrawFees\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"delegate\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"delegator\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"rewards\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"fees\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"startRound\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"endRound\",\"type\":\"uint256\"}],\"name\":\"EarningsClaimed\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"address\",\"name\":\"controller\",\"type\":\"address\"}],\"name\":\"SetController\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"string\",\"name\":\"param\",\"type\":\"string\"}],\"name\":\"ParameterUpdate\",\"type\":\"event\"}]",
}

// L1BondingManagerABI is the input ABI used to generate the binding from.
// Deprecated: Use L1BondingManagerMetaData.ABI instead.
var L1BondingManagerABI = L1BondingManagerMetaData.ABI

// L1BondingManager is an auto generated Go binding around an Ethereum contract.
type L1BondingManager struct {
	L1BondingManagerCaller     // Read-only binding to the contract
	L1BondingManagerTransactor // Write-only binding to the contract
	L1BondingManagerFilterer   // Log filterer for contract events
}

// L1BondingManagerCaller is an auto generated read-only Go binding around an Ethereum contract.
type L1BondingManagerCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// L1BondingManagerTransactor is an auto generated write-only Go binding around an Ethereum contract.
type L1BondingManagerTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// L1BondingManagerFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type L1BondingManagerFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// L1BondingManagerSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type L1BondingManagerSession struct {
	Contract     *L1BondingManager // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// L1BondingManagerCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type L1BondingManagerCallerSession struct {
	Contract *L1BondingManagerCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts           // Call options to use throughout this session
}

// L1BondingManagerTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type L1BondingManagerTransactorSession struct {
	Contract     *L1BondingManagerTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts           // Transaction auth options to use throughout this session
}

// L1BondingManagerRaw is an auto generated low-level Go binding around an Ethereum contract.
type L1BondingManagerRaw struct {
	Contract *L1BondingManager // Generic contract binding to access the raw methods on
}

// L1BondingManagerCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type L1BondingManagerCallerRaw struct {
	Contract *L1BondingManagerCaller // Generic read-only contract binding to access the raw methods on
}

// L1BondingManagerTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type L1BondingManagerTransactorRaw struct {
	Contract *L1BondingManagerTransactor // Generic write-only contract binding to access the raw methods on
}

// NewL1BondingManager creates a new instance of L1BondingManager, bound to a specific deployed contract.
func NewL1BondingManager(address common.Address, backend bind.ContractBackend) (*L1BondingManager, error) {
	contract, err := bindL1BondingManager(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &L1BondingManager{L1BondingManagerCaller: L1BondingManagerCaller{contract: contract}, L1BondingManagerTransactor: L1BondingManagerTransactor{contract: contract}, L1BondingManagerFilterer: L1BondingManagerFilterer{contract: contract}}, nil
}

// NewL1BondingManagerCaller creates a new read-only instance of L1BondingManager, bound to a specific deployed contract.
func NewL1BondingManagerCaller(address common.Address, caller bind.ContractCaller) (*L1BondingManagerCaller, error) {
	contract, err := bindL1BondingManager(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &L1BondingManagerCaller{contract: contract}, nil
}

// NewL1BondingManagerTransactor creates a new write-only instance of L1BondingManager, bound to a specific deployed contract.
func NewL1BondingManagerTransactor(address common.Address, transactor bind.ContractTransactor) (*L1BondingManagerTransactor, error) {
	contract, err := bindL1BondingManager(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &L1BondingManagerTransactor{contract: contract}, nil
}

// NewL1BondingManagerFilterer creates a new log filterer instance of L1BondingManager, bound to a specific deployed contract.
func NewL1BondingManagerFilterer(address common.Address, filterer bind.ContractFilterer) (*L1BondingManagerFilterer, error) {
	contract, err := bindL1BondingManager(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &L1BondingManagerFilterer{contract: contract}, nil
}

// bindL1BondingManager binds a generic wrapper to an already deployed contract.
func bindL1BondingManager(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := abi.JSON(strings.NewReader(L1BondingManagerABI))
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_L1BondingManager *L1BondingManagerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _L1BondingManager.Contract.L1BondingManagerCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_L1BondingManager *L1BondingManagerRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _L1BondingManager.Contract.L1BondingManagerTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_L1BondingManager *L1BondingManagerRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _L1BondingManager.Contract.L1BondingManagerTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_L1BondingManager *L1BondingManagerCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _L1BondingManager.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_L1BondingManager *L1BondingManagerTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _L1BondingManager.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_L1BondingManager *L1BondingManagerTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _L1BondingManager.Contract.contract.Transact(opts, method, params...)
}

// ActiveTranscoderSetDEPRECATED is a free data retrieval call binding the contract method 0x014ee259.
//
// Solidity: function activeTranscoderSetDEPRECATED(uint256 ) view returns(uint256 totalStake)
func (_L1BondingManager *L1BondingManagerCaller) ActiveTranscoderSetDEPRECATED(opts *bind.CallOpts, arg0 *big.Int) (*big.Int, error) {
	var out []interface{}
	err := _L1BondingManager.contract.Call(opts, &out, "activeTranscoderSetDEPRECATED", arg0)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// ActiveTranscoderSetDEPRECATED is a free data retrieval call binding the contract method 0x014ee259.
//
// Solidity: function activeTranscoderSetDEPRECATED(uint256 ) view returns(uint256 totalStake)
func (_L1BondingManager *L1BondingManagerSession) ActiveTranscoderSetDEPRECATED(arg0 *big.Int) (*big.Int, error) {
	return _L1BondingManager.Contract.ActiveTranscoderSetDEPRECATED(&_L1BondingManager.CallOpts, arg0)
}

// ActiveTranscoderSetDEPRECATED is a free data retrieval call binding the contract method 0x014ee259.
//
// Solidity: function activeTranscoderSetDEPRECATED(uint256 ) view returns(uint256 totalStake)
func (_L1BondingManager *L1BondingManagerCallerSession) ActiveTranscoderSetDEPRECATED(arg0 *big.Int) (*big.Int, error) {
	return _L1BondingManager.Contract.ActiveTranscoderSetDEPRECATED(&_L1BondingManager.CallOpts, arg0)
}

// Controller is a free data retrieval call binding the contract method 0xf77c4791.
//
// Solidity: function controller() view returns(address)
func (_L1BondingManager *L1BondingManagerCaller) Controller(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _L1BondingManager.contract.Call(opts, &out, "controller")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// Controller is a free data retrieval call binding the contract method 0xf77c4791.
//
// Solidity: function controller() view returns(address)
func (_L1BondingManager *L1BondingManagerSession) Controller() (common.Address, error) {
	return _L1BondingManager.Contract.Controller(&_L1BondingManager.CallOpts)
}

// Controller is a free data retrieval call binding the contract method 0xf77c4791.
//
// Solidity: function controller() view returns(address)
func (_L1BondingManager *L1BondingManagerCallerSession) Controller() (common.Address, error) {
	return _L1BondingManager.Contract.Controller(&_L1BondingManager.CallOpts)
}

// CurrentRoundTotalActiveStake is a free data retrieval call binding the contract method 0x4196ee75.
//
// Solidity: function currentRoundTotalActiveStake() view returns(uint256)
func (_L1BondingManager *L1BondingManagerCaller) CurrentRoundTotalActiveStake(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _L1BondingManager.contract.Call(opts, &out, "currentRoundTotalActiveStake")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// CurrentRoundTotalActiveStake is a free data retrieval call binding the contract method 0x4196ee75.
//
// Solidity: function currentRoundTotalActiveStake() view returns(uint256)
func (_L1BondingManager *L1BondingManagerSession) CurrentRoundTotalActiveStake() (*big.Int, error) {
	return _L1BondingManager.Contract.CurrentRoundTotalActiveStake(&_L1BondingManager.CallOpts)
}

// CurrentRoundTotalActiveStake is a free data retrieval call binding the contract method 0x4196ee75.
//
// Solidity: function currentRoundTotalActiveStake() view returns(uint256)
func (_L1BondingManager *L1BondingManagerCallerSession) CurrentRoundTotalActiveStake() (*big.Int, error) {
	return _L1BondingManager.Contract.CurrentRoundTotalActiveStake(&_L1BondingManager.CallOpts)
}

// DelegatorStatus is a free data retrieval call binding the contract method 0x1544fc67.
//
// Solidity: function delegatorStatus(address _delegator) view returns(uint8)
func (_L1BondingManager *L1BondingManagerCaller) DelegatorStatus(opts *bind.CallOpts, _delegator common.Address) (uint8, error) {
	var out []interface{}
	err := _L1BondingManager.contract.Call(opts, &out, "delegatorStatus", _delegator)

	if err != nil {
		return *new(uint8), err
	}

	out0 := *abi.ConvertType(out[0], new(uint8)).(*uint8)

	return out0, err

}

// DelegatorStatus is a free data retrieval call binding the contract method 0x1544fc67.
//
// Solidity: function delegatorStatus(address _delegator) view returns(uint8)
func (_L1BondingManager *L1BondingManagerSession) DelegatorStatus(_delegator common.Address) (uint8, error) {
	return _L1BondingManager.Contract.DelegatorStatus(&_L1BondingManager.CallOpts, _delegator)
}

// DelegatorStatus is a free data retrieval call binding the contract method 0x1544fc67.
//
// Solidity: function delegatorStatus(address _delegator) view returns(uint8)
func (_L1BondingManager *L1BondingManagerCallerSession) DelegatorStatus(_delegator common.Address) (uint8, error) {
	return _L1BondingManager.Contract.DelegatorStatus(&_L1BondingManager.CallOpts, _delegator)
}

// GetDelegator is a free data retrieval call binding the contract method 0xa64ad595.
//
// Solidity: function getDelegator(address _delegator) view returns(uint256 bondedAmount, uint256 fees, address delegateAddress, uint256 delegatedAmount, uint256 startRound, uint256 lastClaimRound, uint256 nextUnbondingLockId)
func (_L1BondingManager *L1BondingManagerCaller) GetDelegator(opts *bind.CallOpts, _delegator common.Address) (struct {
	BondedAmount        *big.Int
	Fees                *big.Int
	DelegateAddress     common.Address
	DelegatedAmount     *big.Int
	StartRound          *big.Int
	LastClaimRound      *big.Int
	NextUnbondingLockId *big.Int
}, error) {
	var out []interface{}
	err := _L1BondingManager.contract.Call(opts, &out, "getDelegator", _delegator)

	outstruct := new(struct {
		BondedAmount        *big.Int
		Fees                *big.Int
		DelegateAddress     common.Address
		DelegatedAmount     *big.Int
		StartRound          *big.Int
		LastClaimRound      *big.Int
		NextUnbondingLockId *big.Int
	})
	if err != nil {
		return *outstruct, err
	}

	outstruct.BondedAmount = *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)
	outstruct.Fees = *abi.ConvertType(out[1], new(*big.Int)).(**big.Int)
	outstruct.DelegateAddress = *abi.ConvertType(out[2], new(common.Address)).(*common.Address)
	outstruct.DelegatedAmount = *abi.ConvertType(out[3], new(*big.Int)).(**big.Int)
	outstruct.StartRound = *abi.ConvertType(out[4], new(*big.Int)).(**big.Int)
	outstruct.LastClaimRound = *abi.ConvertType(out[5], new(*big.Int)).(**big.Int)
	outstruct.NextUnbondingLockId = *abi.ConvertType(out[6], new(*big.Int)).(**big.Int)

	return *outstruct, err

}

// GetDelegator is a free data retrieval call binding the contract method 0xa64ad595.
//
// Solidity: function getDelegator(address _delegator) view returns(uint256 bondedAmount, uint256 fees, address delegateAddress, uint256 delegatedAmount, uint256 startRound, uint256 lastClaimRound, uint256 nextUnbondingLockId)
func (_L1BondingManager *L1BondingManagerSession) GetDelegator(_delegator common.Address) (struct {
	BondedAmount        *big.Int
	Fees                *big.Int
	DelegateAddress     common.Address
	DelegatedAmount     *big.Int
	StartRound          *big.Int
	LastClaimRound      *big.Int
	NextUnbondingLockId *big.Int
}, error) {
	return _L1BondingManager.Contract.GetDelegator(&_L1BondingManager.CallOpts, _delegator)
}

// GetDelegator is a free data retrieval call binding the contract method 0xa64ad595.
//
// Solidity: function getDelegator(address _delegator) view returns(uint256 bondedAmount, uint256 fees, address delegateAddress, uint256 delegatedAmount, uint256 startRound, uint256 lastClaimRound, uint256 nextUnbondingLockId)
func (_L1BondingManager *L1BondingManagerCallerSession) GetDelegator(_delegator common.Address) (struct {
	BondedAmount        *big.Int
	Fees                *big.Int
	DelegateAddress     common.Address
	DelegatedAmount     *big.Int
	StartRound          *big.Int
	LastClaimRound      *big.Int
	NextUnbondingLockId *big.Int
}, error) {
	return _L1BondingManager.Contract.GetDelegator(&_L1BondingManager.CallOpts, _delegator)
}

// GetDelegatorUnbondingLock is a free data retrieval call binding the contract method 0x412f83b6.
//
// Solidity: function getDelegatorUnbondingLock(address _delegator, uint256 _unbondingLockId) view returns(uint256 amount, uint256 withdrawRound)
func (_L1BondingManager *L1BondingManagerCaller) GetDelegatorUnbondingLock(opts *bind.CallOpts, _delegator common.Address, _unbondingLockId *big.Int) (struct {
	Amount        *big.Int
	WithdrawRound *big.Int
}, error) {
	var out []interface{}
	err := _L1BondingManager.contract.Call(opts, &out, "getDelegatorUnbondingLock", _delegator, _unbondingLockId)

	outstruct := new(struct {
		Amount        *big.Int
		WithdrawRound *big.Int
	})
	if err != nil {
		return *outstruct, err
	}

	outstruct.Amount = *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)
	outstruct.WithdrawRound = *abi.ConvertType(out[1], new(*big.Int)).(**big.Int)

	return *outstruct, err

}

// GetDelegatorUnbondingLock is a free data retrieval call binding the contract method 0x412f83b6.
//
// Solidity: function getDelegatorUnbondingLock(address _delegator, uint256 _unbondingLockId) view returns(uint256 amount, uint256 withdrawRound)
func (_L1BondingManager *L1BondingManagerSession) GetDelegatorUnbondingLock(_delegator common.Address, _unbondingLockId *big.Int) (struct {
	Amount        *big.Int
	WithdrawRound *big.Int
}, error) {
	return _L1BondingManager.Contract.GetDelegatorUnbondingLock(&_L1BondingManager.CallOpts, _delegator, _unbondingLockId)
}

// GetDelegatorUnbondingLock is a free data retrieval call binding the contract method 0x412f83b6.
//
// Solidity: function getDelegatorUnbondingLock(address _delegator, uint256 _unbondingLockId) view returns(uint256 amount, uint256 withdrawRound)
func (_L1BondingManager *L1BondingManagerCallerSession) GetDelegatorUnbondingLock(_delegator common.Address, _unbondingLockId *big.Int) (struct {
	Amount        *big.Int
	WithdrawRound *big.Int
}, error) {
	return _L1BondingManager.Contract.GetDelegatorUnbondingLock(&_L1BondingManager.CallOpts, _delegator, _unbondingLockId)
}

// GetFirstTranscoderInPool is a free data retrieval call binding the contract method 0x88a6c749.
//
// Solidity: function getFirstTranscoderInPool() view returns(address)
func (_L1BondingManager *L1BondingManagerCaller) GetFirstTranscoderInPool(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _L1BondingManager.contract.Call(opts, &out, "getFirstTranscoderInPool")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// GetFirstTranscoderInPool is a free data retrieval call binding the contract method 0x88a6c749.
//
// Solidity: function getFirstTranscoderInPool() view returns(address)
func (_L1BondingManager *L1BondingManagerSession) GetFirstTranscoderInPool() (common.Address, error) {
	return _L1BondingManager.Contract.GetFirstTranscoderInPool(&_L1BondingManager.CallOpts)
}

// GetFirstTranscoderInPool is a free data retrieval call binding the contract method 0x88a6c749.
//
// Solidity: function getFirstTranscoderInPool() view returns(address)
func (_L1BondingManager *L1BondingManagerCallerSession) GetFirstTranscoderInPool() (common.Address, error) {
	return _L1BondingManager.Contract.GetFirstTranscoderInPool(&_L1BondingManager.CallOpts)
}

// GetNextTranscoderInPool is a free data retrieval call binding the contract method 0x235c9603.
//
// Solidity: function getNextTranscoderInPool(address _transcoder) view returns(address)
func (_L1BondingManager *L1BondingManagerCaller) GetNextTranscoderInPool(opts *bind.CallOpts, _transcoder common.Address) (common.Address, error) {
	var out []interface{}
	err := _L1BondingManager.contract.Call(opts, &out, "getNextTranscoderInPool", _transcoder)

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// GetNextTranscoderInPool is a free data retrieval call binding the contract method 0x235c9603.
//
// Solidity: function getNextTranscoderInPool(address _transcoder) view returns(address)
func (_L1BondingManager *L1BondingManagerSession) GetNextTranscoderInPool(_transcoder common.Address) (common.Address, error) {
	return _L1BondingManager.Contract.GetNextTranscoderInPool(&_L1BondingManager.CallOpts, _transcoder)
}

// GetNextTranscoderInPool is a free data retrieval call binding the contract method 0x235c9603.
//
// Solidity: function getNextTranscoderInPool(address _transcoder) view returns(address)
func (_L1BondingManager *L1BondingManagerCallerSession) GetNextTranscoderInPool(_transcoder common.Address) (common.Address, error) {
	return _L1BondingManager.Contract.GetNextTranscoderInPool(&_L1BondingManager.CallOpts, _transcoder)
}

// GetTotalBonded is a free data retrieval call binding the contract method 0x5c50c356.
//
// Solidity: function getTotalBonded() view returns(uint256)
func (_L1BondingManager *L1BondingManagerCaller) GetTotalBonded(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _L1BondingManager.contract.Call(opts, &out, "getTotalBonded")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// GetTotalBonded is a free data retrieval call binding the contract method 0x5c50c356.
//
// Solidity: function getTotalBonded() view returns(uint256)
func (_L1BondingManager *L1BondingManagerSession) GetTotalBonded() (*big.Int, error) {
	return _L1BondingManager.Contract.GetTotalBonded(&_L1BondingManager.CallOpts)
}

// GetTotalBonded is a free data retrieval call binding the contract method 0x5c50c356.
//
// Solidity: function getTotalBonded() view returns(uint256)
func (_L1BondingManager *L1BondingManagerCallerSession) GetTotalBonded() (*big.Int, error) {
	return _L1BondingManager.Contract.GetTotalBonded(&_L1BondingManager.CallOpts)
}

// GetTranscoder is a free data retrieval call binding the contract method 0x5dce9948.
//
// Solidity: function getTranscoder(address _transcoder) view returns(uint256 lastRewardRound, uint256 rewardCut, uint256 feeShare, uint256 lastActiveStakeUpdateRound, uint256 activationRound, uint256 deactivationRound, uint256 activeCumulativeRewards, uint256 cumulativeRewards, uint256 cumulativeFees, uint256 lastFeeRound)
func (_L1BondingManager *L1BondingManagerCaller) GetTranscoder(opts *bind.CallOpts, _transcoder common.Address) (struct {
	LastRewardRound            *big.Int
	RewardCut                  *big.Int
	FeeShare                   *big.Int
	LastActiveStakeUpdateRound *big.Int
	ActivationRound            *big.Int
	DeactivationRound          *big.Int
	ActiveCumulativeRewards    *big.Int
	CumulativeRewards          *big.Int
	CumulativeFees             *big.Int
	LastFeeRound               *big.Int
}, error) {
	var out []interface{}
	err := _L1BondingManager.contract.Call(opts, &out, "getTranscoder", _transcoder)

	outstruct := new(struct {
		LastRewardRound            *big.Int
		RewardCut                  *big.Int
		FeeShare                   *big.Int
		LastActiveStakeUpdateRound *big.Int
		ActivationRound            *big.Int
		DeactivationRound          *big.Int
		ActiveCumulativeRewards    *big.Int
		CumulativeRewards          *big.Int
		CumulativeFees             *big.Int
		LastFeeRound               *big.Int
	})
	if err != nil {
		return *outstruct, err
	}

	outstruct.LastRewardRound = *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)
	outstruct.RewardCut = *abi.ConvertType(out[1], new(*big.Int)).(**big.Int)
	outstruct.FeeShare = *abi.ConvertType(out[2], new(*big.Int)).(**big.Int)
	outstruct.LastActiveStakeUpdateRound = *abi.ConvertType(out[3], new(*big.Int)).(**big.Int)
	outstruct.ActivationRound = *abi.ConvertType(out[4], new(*big.Int)).(**big.Int)
	outstruct.DeactivationRound = *abi.ConvertType(out[5], new(*big.Int)).(**big.Int)
	outstruct.ActiveCumulativeRewards = *abi.ConvertType(out[6], new(*big.Int)).(**big.Int)
	outstruct.CumulativeRewards = *abi.ConvertType(out[7], new(*big.Int)).(**big.Int)
	outstruct.CumulativeFees = *abi.ConvertType(out[8], new(*big.Int)).(**big.Int)
	outstruct.LastFeeRound = *abi.ConvertType(out[9], new(*big.Int)).(**big.Int)

	return *outstruct, err

}

// GetTranscoder is a free data retrieval call binding the contract method 0x5dce9948.
//
// Solidity: function getTranscoder(address _transcoder) view returns(uint256 lastRewardRound, uint256 rewardCut, uint256 feeShare, uint256 lastActiveStakeUpdateRound, uint256 activationRound, uint256 deactivationRound, uint256 activeCumulativeRewards, uint256 cumulativeRewards, uint256 cumulativeFees, uint256 lastFeeRound)
func (_L1BondingManager *L1BondingManagerSession) GetTranscoder(_transcoder common.Address) (struct {
	LastRewardRound            *big.Int
	RewardCut                  *big.Int
	FeeShare                   *big.Int
	LastActiveStakeUpdateRound *big.Int
	ActivationRound            *big.Int
	DeactivationRound          *big.Int
	ActiveCumulativeRewards    *big.Int
	CumulativeRewards          *big.Int
	CumulativeFees             *big.Int
	LastFeeRound               *big.Int
}, error) {
	return _L1BondingManager.Contract.GetTranscoder(&_L1BondingManager.CallOpts, _transcoder)
}

// GetTranscoder is a free data retrieval call binding the contract method 0x5dce9948.
//
// Solidity: function getTranscoder(address _transcoder) view returns(uint256 lastRewardRound, uint256 rewardCut, uint256 feeShare, uint256 lastActiveStakeUpdateRound, uint256 activationRound, uint256 deactivationRound, uint256 activeCumulativeRewards, uint256 cumulativeRewards, uint256 cumulativeFees, uint256 lastFeeRound)
func (_L1BondingManager *L1BondingManagerCallerSession) GetTranscoder(_transcoder common.Address) (struct {
	LastRewardRound            *big.Int
	RewardCut                  *big.Int
	FeeShare                   *big.Int
	LastActiveStakeUpdateRound *big.Int
	ActivationRound            *big.Int
	DeactivationRound          *big.Int
	ActiveCumulativeRewards    *big.Int
	CumulativeRewards          *big.Int
	CumulativeFees             *big.Int
	LastFeeRound               *big.Int
}, error) {
	return _L1BondingManager.Contract.GetTranscoder(&_L1BondingManager.CallOpts, _transcoder)
}

// GetTranscoderEarningsPoolForRound is a free data retrieval call binding the contract method 0x24454fc4.
//
// Solidity: function getTranscoderEarningsPoolForRound(address _transcoder, uint256 _round) view returns(uint256 rewardPool, uint256 feePool, uint256 totalStake, uint256 claimableStake, uint256 transcoderRewardCut, uint256 transcoderFeeShare, uint256 transcoderRewardPool, uint256 transcoderFeePool, bool hasTranscoderRewardFeePool, uint256 cumulativeRewardFactor, uint256 cumulativeFeeFactor)
func (_L1BondingManager *L1BondingManagerCaller) GetTranscoderEarningsPoolForRound(opts *bind.CallOpts, _transcoder common.Address, _round *big.Int) (struct {
	RewardPool                 *big.Int
	FeePool                    *big.Int
	TotalStake                 *big.Int
	ClaimableStake             *big.Int
	TranscoderRewardCut        *big.Int
	TranscoderFeeShare         *big.Int
	TranscoderRewardPool       *big.Int
	TranscoderFeePool          *big.Int
	HasTranscoderRewardFeePool bool
	CumulativeRewardFactor     *big.Int
	CumulativeFeeFactor        *big.Int
}, error) {
	var out []interface{}
	err := _L1BondingManager.contract.Call(opts, &out, "getTranscoderEarningsPoolForRound", _transcoder, _round)

	outstruct := new(struct {
		RewardPool                 *big.Int
		FeePool                    *big.Int
		TotalStake                 *big.Int
		ClaimableStake             *big.Int
		TranscoderRewardCut        *big.Int
		TranscoderFeeShare         *big.Int
		TranscoderRewardPool       *big.Int
		TranscoderFeePool          *big.Int
		HasTranscoderRewardFeePool bool
		CumulativeRewardFactor     *big.Int
		CumulativeFeeFactor        *big.Int
	})
	if err != nil {
		return *outstruct, err
	}

	outstruct.RewardPool = *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)
	outstruct.FeePool = *abi.ConvertType(out[1], new(*big.Int)).(**big.Int)
	outstruct.TotalStake = *abi.ConvertType(out[2], new(*big.Int)).(**big.Int)
	outstruct.ClaimableStake = *abi.ConvertType(out[3], new(*big.Int)).(**big.Int)
	outstruct.TranscoderRewardCut = *abi.ConvertType(out[4], new(*big.Int)).(**big.Int)
	outstruct.TranscoderFeeShare = *abi.ConvertType(out[5], new(*big.Int)).(**big.Int)
	outstruct.TranscoderRewardPool = *abi.ConvertType(out[6], new(*big.Int)).(**big.Int)
	outstruct.TranscoderFeePool = *abi.ConvertType(out[7], new(*big.Int)).(**big.Int)
	outstruct.HasTranscoderRewardFeePool = *abi.ConvertType(out[8], new(bool)).(*bool)
	outstruct.CumulativeRewardFactor = *abi.ConvertType(out[9], new(*big.Int)).(**big.Int)
	outstruct.CumulativeFeeFactor = *abi.ConvertType(out[10], new(*big.Int)).(**big.Int)

	return *outstruct, err

}

// GetTranscoderEarningsPoolForRound is a free data retrieval call binding the contract method 0x24454fc4.
//
// Solidity: function getTranscoderEarningsPoolForRound(address _transcoder, uint256 _round) view returns(uint256 rewardPool, uint256 feePool, uint256 totalStake, uint256 claimableStake, uint256 transcoderRewardCut, uint256 transcoderFeeShare, uint256 transcoderRewardPool, uint256 transcoderFeePool, bool hasTranscoderRewardFeePool, uint256 cumulativeRewardFactor, uint256 cumulativeFeeFactor)
func (_L1BondingManager *L1BondingManagerSession) GetTranscoderEarningsPoolForRound(_transcoder common.Address, _round *big.Int) (struct {
	RewardPool                 *big.Int
	FeePool                    *big.Int
	TotalStake                 *big.Int
	ClaimableStake             *big.Int
	TranscoderRewardCut        *big.Int
	TranscoderFeeShare         *big.Int
	TranscoderRewardPool       *big.Int
	TranscoderFeePool          *big.Int
	HasTranscoderRewardFeePool bool
	CumulativeRewardFactor     *big.Int
	CumulativeFeeFactor        *big.Int
}, error) {
	return _L1BondingManager.Contract.GetTranscoderEarningsPoolForRound(&_L1BondingManager.CallOpts, _transcoder, _round)
}

// GetTranscoderEarningsPoolForRound is a free data retrieval call binding the contract method 0x24454fc4.
//
// Solidity: function getTranscoderEarningsPoolForRound(address _transcoder, uint256 _round) view returns(uint256 rewardPool, uint256 feePool, uint256 totalStake, uint256 claimableStake, uint256 transcoderRewardCut, uint256 transcoderFeeShare, uint256 transcoderRewardPool, uint256 transcoderFeePool, bool hasTranscoderRewardFeePool, uint256 cumulativeRewardFactor, uint256 cumulativeFeeFactor)
func (_L1BondingManager *L1BondingManagerCallerSession) GetTranscoderEarningsPoolForRound(_transcoder common.Address, _round *big.Int) (struct {
	RewardPool                 *big.Int
	FeePool                    *big.Int
	TotalStake                 *big.Int
	ClaimableStake             *big.Int
	TranscoderRewardCut        *big.Int
	TranscoderFeeShare         *big.Int
	TranscoderRewardPool       *big.Int
	TranscoderFeePool          *big.Int
	HasTranscoderRewardFeePool bool
	CumulativeRewardFactor     *big.Int
	CumulativeFeeFactor        *big.Int
}, error) {
	return _L1BondingManager.Contract.GetTranscoderEarningsPoolForRound(&_L1BondingManager.CallOpts, _transcoder, _round)
}

// GetTranscoderPoolMaxSize is a free data retrieval call binding the contract method 0x5a2a75a9.
//
// Solidity: function getTranscoderPoolMaxSize() view returns(uint256)
func (_L1BondingManager *L1BondingManagerCaller) GetTranscoderPoolMaxSize(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _L1BondingManager.contract.Call(opts, &out, "getTranscoderPoolMaxSize")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// GetTranscoderPoolMaxSize is a free data retrieval call binding the contract method 0x5a2a75a9.
//
// Solidity: function getTranscoderPoolMaxSize() view returns(uint256)
func (_L1BondingManager *L1BondingManagerSession) GetTranscoderPoolMaxSize() (*big.Int, error) {
	return _L1BondingManager.Contract.GetTranscoderPoolMaxSize(&_L1BondingManager.CallOpts)
}

// GetTranscoderPoolMaxSize is a free data retrieval call binding the contract method 0x5a2a75a9.
//
// Solidity: function getTranscoderPoolMaxSize() view returns(uint256)
func (_L1BondingManager *L1BondingManagerCallerSession) GetTranscoderPoolMaxSize() (*big.Int, error) {
	return _L1BondingManager.Contract.GetTranscoderPoolMaxSize(&_L1BondingManager.CallOpts)
}

// GetTranscoderPoolSize is a free data retrieval call binding the contract method 0x2a4e0d55.
//
// Solidity: function getTranscoderPoolSize() view returns(uint256)
func (_L1BondingManager *L1BondingManagerCaller) GetTranscoderPoolSize(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _L1BondingManager.contract.Call(opts, &out, "getTranscoderPoolSize")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// GetTranscoderPoolSize is a free data retrieval call binding the contract method 0x2a4e0d55.
//
// Solidity: function getTranscoderPoolSize() view returns(uint256)
func (_L1BondingManager *L1BondingManagerSession) GetTranscoderPoolSize() (*big.Int, error) {
	return _L1BondingManager.Contract.GetTranscoderPoolSize(&_L1BondingManager.CallOpts)
}

// GetTranscoderPoolSize is a free data retrieval call binding the contract method 0x2a4e0d55.
//
// Solidity: function getTranscoderPoolSize() view returns(uint256)
func (_L1BondingManager *L1BondingManagerCallerSession) GetTranscoderPoolSize() (*big.Int, error) {
	return _L1BondingManager.Contract.GetTranscoderPoolSize(&_L1BondingManager.CallOpts)
}

// IsActiveTranscoder is a free data retrieval call binding the contract method 0x08802374.
//
// Solidity: function isActiveTranscoder(address _transcoder) view returns(bool)
func (_L1BondingManager *L1BondingManagerCaller) IsActiveTranscoder(opts *bind.CallOpts, _transcoder common.Address) (bool, error) {
	var out []interface{}
	err := _L1BondingManager.contract.Call(opts, &out, "isActiveTranscoder", _transcoder)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// IsActiveTranscoder is a free data retrieval call binding the contract method 0x08802374.
//
// Solidity: function isActiveTranscoder(address _transcoder) view returns(bool)
func (_L1BondingManager *L1BondingManagerSession) IsActiveTranscoder(_transcoder common.Address) (bool, error) {
	return _L1BondingManager.Contract.IsActiveTranscoder(&_L1BondingManager.CallOpts, _transcoder)
}

// IsActiveTranscoder is a free data retrieval call binding the contract method 0x08802374.
//
// Solidity: function isActiveTranscoder(address _transcoder) view returns(bool)
func (_L1BondingManager *L1BondingManagerCallerSession) IsActiveTranscoder(_transcoder common.Address) (bool, error) {
	return _L1BondingManager.Contract.IsActiveTranscoder(&_L1BondingManager.CallOpts, _transcoder)
}

// IsRegisteredTranscoder is a free data retrieval call binding the contract method 0x68ba170c.
//
// Solidity: function isRegisteredTranscoder(address _transcoder) view returns(bool)
func (_L1BondingManager *L1BondingManagerCaller) IsRegisteredTranscoder(opts *bind.CallOpts, _transcoder common.Address) (bool, error) {
	var out []interface{}
	err := _L1BondingManager.contract.Call(opts, &out, "isRegisteredTranscoder", _transcoder)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// IsRegisteredTranscoder is a free data retrieval call binding the contract method 0x68ba170c.
//
// Solidity: function isRegisteredTranscoder(address _transcoder) view returns(bool)
func (_L1BondingManager *L1BondingManagerSession) IsRegisteredTranscoder(_transcoder common.Address) (bool, error) {
	return _L1BondingManager.Contract.IsRegisteredTranscoder(&_L1BondingManager.CallOpts, _transcoder)
}

// IsRegisteredTranscoder is a free data retrieval call binding the contract method 0x68ba170c.
//
// Solidity: function isRegisteredTranscoder(address _transcoder) view returns(bool)
func (_L1BondingManager *L1BondingManagerCallerSession) IsRegisteredTranscoder(_transcoder common.Address) (bool, error) {
	return _L1BondingManager.Contract.IsRegisteredTranscoder(&_L1BondingManager.CallOpts, _transcoder)
}

// IsValidUnbondingLock is a free data retrieval call binding the contract method 0x0fd02fc1.
//
// Solidity: function isValidUnbondingLock(address _delegator, uint256 _unbondingLockId) view returns(bool)
func (_L1BondingManager *L1BondingManagerCaller) IsValidUnbondingLock(opts *bind.CallOpts, _delegator common.Address, _unbondingLockId *big.Int) (bool, error) {
	var out []interface{}
	err := _L1BondingManager.contract.Call(opts, &out, "isValidUnbondingLock", _delegator, _unbondingLockId)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// IsValidUnbondingLock is a free data retrieval call binding the contract method 0x0fd02fc1.
//
// Solidity: function isValidUnbondingLock(address _delegator, uint256 _unbondingLockId) view returns(bool)
func (_L1BondingManager *L1BondingManagerSession) IsValidUnbondingLock(_delegator common.Address, _unbondingLockId *big.Int) (bool, error) {
	return _L1BondingManager.Contract.IsValidUnbondingLock(&_L1BondingManager.CallOpts, _delegator, _unbondingLockId)
}

// IsValidUnbondingLock is a free data retrieval call binding the contract method 0x0fd02fc1.
//
// Solidity: function isValidUnbondingLock(address _delegator, uint256 _unbondingLockId) view returns(bool)
func (_L1BondingManager *L1BondingManagerCallerSession) IsValidUnbondingLock(_delegator common.Address, _unbondingLockId *big.Int) (bool, error) {
	return _L1BondingManager.Contract.IsValidUnbondingLock(&_L1BondingManager.CallOpts, _delegator, _unbondingLockId)
}

// MaxEarningsClaimsRounds is a free data retrieval call binding the contract method 0x038424c3.
//
// Solidity: function maxEarningsClaimsRounds() view returns(uint256)
func (_L1BondingManager *L1BondingManagerCaller) MaxEarningsClaimsRounds(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _L1BondingManager.contract.Call(opts, &out, "maxEarningsClaimsRounds")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// MaxEarningsClaimsRounds is a free data retrieval call binding the contract method 0x038424c3.
//
// Solidity: function maxEarningsClaimsRounds() view returns(uint256)
func (_L1BondingManager *L1BondingManagerSession) MaxEarningsClaimsRounds() (*big.Int, error) {
	return _L1BondingManager.Contract.MaxEarningsClaimsRounds(&_L1BondingManager.CallOpts)
}

// MaxEarningsClaimsRounds is a free data retrieval call binding the contract method 0x038424c3.
//
// Solidity: function maxEarningsClaimsRounds() view returns(uint256)
func (_L1BondingManager *L1BondingManagerCallerSession) MaxEarningsClaimsRounds() (*big.Int, error) {
	return _L1BondingManager.Contract.MaxEarningsClaimsRounds(&_L1BondingManager.CallOpts)
}

// NextRoundTotalActiveStake is a free data retrieval call binding the contract method 0x465501d3.
//
// Solidity: function nextRoundTotalActiveStake() view returns(uint256)
func (_L1BondingManager *L1BondingManagerCaller) NextRoundTotalActiveStake(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _L1BondingManager.contract.Call(opts, &out, "nextRoundTotalActiveStake")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// NextRoundTotalActiveStake is a free data retrieval call binding the contract method 0x465501d3.
//
// Solidity: function nextRoundTotalActiveStake() view returns(uint256)
func (_L1BondingManager *L1BondingManagerSession) NextRoundTotalActiveStake() (*big.Int, error) {
	return _L1BondingManager.Contract.NextRoundTotalActiveStake(&_L1BondingManager.CallOpts)
}

// NextRoundTotalActiveStake is a free data retrieval call binding the contract method 0x465501d3.
//
// Solidity: function nextRoundTotalActiveStake() view returns(uint256)
func (_L1BondingManager *L1BondingManagerCallerSession) NextRoundTotalActiveStake() (*big.Int, error) {
	return _L1BondingManager.Contract.NextRoundTotalActiveStake(&_L1BondingManager.CallOpts)
}

// NumActiveTranscodersDEPRECATED is a free data retrieval call binding the contract method 0x3c725cbb.
//
// Solidity: function numActiveTranscodersDEPRECATED() view returns(uint256)
func (_L1BondingManager *L1BondingManagerCaller) NumActiveTranscodersDEPRECATED(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _L1BondingManager.contract.Call(opts, &out, "numActiveTranscodersDEPRECATED")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// NumActiveTranscodersDEPRECATED is a free data retrieval call binding the contract method 0x3c725cbb.
//
// Solidity: function numActiveTranscodersDEPRECATED() view returns(uint256)
func (_L1BondingManager *L1BondingManagerSession) NumActiveTranscodersDEPRECATED() (*big.Int, error) {
	return _L1BondingManager.Contract.NumActiveTranscodersDEPRECATED(&_L1BondingManager.CallOpts)
}

// NumActiveTranscodersDEPRECATED is a free data retrieval call binding the contract method 0x3c725cbb.
//
// Solidity: function numActiveTranscodersDEPRECATED() view returns(uint256)
func (_L1BondingManager *L1BondingManagerCallerSession) NumActiveTranscodersDEPRECATED() (*big.Int, error) {
	return _L1BondingManager.Contract.NumActiveTranscodersDEPRECATED(&_L1BondingManager.CallOpts)
}

// PendingFees is a free data retrieval call binding the contract method 0xf595f1cc.
//
// Solidity: function pendingFees(address _delegator, uint256 _endRound) view returns(uint256)
func (_L1BondingManager *L1BondingManagerCaller) PendingFees(opts *bind.CallOpts, _delegator common.Address, _endRound *big.Int) (*big.Int, error) {
	var out []interface{}
	err := _L1BondingManager.contract.Call(opts, &out, "pendingFees", _delegator, _endRound)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// PendingFees is a free data retrieval call binding the contract method 0xf595f1cc.
//
// Solidity: function pendingFees(address _delegator, uint256 _endRound) view returns(uint256)
func (_L1BondingManager *L1BondingManagerSession) PendingFees(_delegator common.Address, _endRound *big.Int) (*big.Int, error) {
	return _L1BondingManager.Contract.PendingFees(&_L1BondingManager.CallOpts, _delegator, _endRound)
}

// PendingFees is a free data retrieval call binding the contract method 0xf595f1cc.
//
// Solidity: function pendingFees(address _delegator, uint256 _endRound) view returns(uint256)
func (_L1BondingManager *L1BondingManagerCallerSession) PendingFees(_delegator common.Address, _endRound *big.Int) (*big.Int, error) {
	return _L1BondingManager.Contract.PendingFees(&_L1BondingManager.CallOpts, _delegator, _endRound)
}

// PendingStake is a free data retrieval call binding the contract method 0x9d0b2c7a.
//
// Solidity: function pendingStake(address _delegator, uint256 _endRound) view returns(uint256)
func (_L1BondingManager *L1BondingManagerCaller) PendingStake(opts *bind.CallOpts, _delegator common.Address, _endRound *big.Int) (*big.Int, error) {
	var out []interface{}
	err := _L1BondingManager.contract.Call(opts, &out, "pendingStake", _delegator, _endRound)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// PendingStake is a free data retrieval call binding the contract method 0x9d0b2c7a.
//
// Solidity: function pendingStake(address _delegator, uint256 _endRound) view returns(uint256)
func (_L1BondingManager *L1BondingManagerSession) PendingStake(_delegator common.Address, _endRound *big.Int) (*big.Int, error) {
	return _L1BondingManager.Contract.PendingStake(&_L1BondingManager.CallOpts, _delegator, _endRound)
}

// PendingStake is a free data retrieval call binding the contract method 0x9d0b2c7a.
//
// Solidity: function pendingStake(address _delegator, uint256 _endRound) view returns(uint256)
func (_L1BondingManager *L1BondingManagerCallerSession) PendingStake(_delegator common.Address, _endRound *big.Int) (*big.Int, error) {
	return _L1BondingManager.Contract.PendingStake(&_L1BondingManager.CallOpts, _delegator, _endRound)
}

// TargetContractId is a free data retrieval call binding the contract method 0x51720b41.
//
// Solidity: function targetContractId() view returns(bytes32)
func (_L1BondingManager *L1BondingManagerCaller) TargetContractId(opts *bind.CallOpts) ([32]byte, error) {
	var out []interface{}
	err := _L1BondingManager.contract.Call(opts, &out, "targetContractId")

	if err != nil {
		return *new([32]byte), err
	}

	out0 := *abi.ConvertType(out[0], new([32]byte)).(*[32]byte)

	return out0, err

}

// TargetContractId is a free data retrieval call binding the contract method 0x51720b41.
//
// Solidity: function targetContractId() view returns(bytes32)
func (_L1BondingManager *L1BondingManagerSession) TargetContractId() ([32]byte, error) {
	return _L1BondingManager.Contract.TargetContractId(&_L1BondingManager.CallOpts)
}

// TargetContractId is a free data retrieval call binding the contract method 0x51720b41.
//
// Solidity: function targetContractId() view returns(bytes32)
func (_L1BondingManager *L1BondingManagerCallerSession) TargetContractId() ([32]byte, error) {
	return _L1BondingManager.Contract.TargetContractId(&_L1BondingManager.CallOpts)
}

// TranscoderStatus is a free data retrieval call binding the contract method 0x8b2f1652.
//
// Solidity: function transcoderStatus(address _transcoder) view returns(uint8)
func (_L1BondingManager *L1BondingManagerCaller) TranscoderStatus(opts *bind.CallOpts, _transcoder common.Address) (uint8, error) {
	var out []interface{}
	err := _L1BondingManager.contract.Call(opts, &out, "transcoderStatus", _transcoder)

	if err != nil {
		return *new(uint8), err
	}

	out0 := *abi.ConvertType(out[0], new(uint8)).(*uint8)

	return out0, err

}

// TranscoderStatus is a free data retrieval call binding the contract method 0x8b2f1652.
//
// Solidity: function transcoderStatus(address _transcoder) view returns(uint8)
func (_L1BondingManager *L1BondingManagerSession) TranscoderStatus(_transcoder common.Address) (uint8, error) {
	return _L1BondingManager.Contract.TranscoderStatus(&_L1BondingManager.CallOpts, _transcoder)
}

// TranscoderStatus is a free data retrieval call binding the contract method 0x8b2f1652.
//
// Solidity: function transcoderStatus(address _transcoder) view returns(uint8)
func (_L1BondingManager *L1BondingManagerCallerSession) TranscoderStatus(_transcoder common.Address) (uint8, error) {
	return _L1BondingManager.Contract.TranscoderStatus(&_L1BondingManager.CallOpts, _transcoder)
}

// TranscoderTotalStake is a free data retrieval call binding the contract method 0x9ef9df94.
//
// Solidity: function transcoderTotalStake(address _transcoder) view returns(uint256)
func (_L1BondingManager *L1BondingManagerCaller) TranscoderTotalStake(opts *bind.CallOpts, _transcoder common.Address) (*big.Int, error) {
	var out []interface{}
	err := _L1BondingManager.contract.Call(opts, &out, "transcoderTotalStake", _transcoder)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// TranscoderTotalStake is a free data retrieval call binding the contract method 0x9ef9df94.
//
// Solidity: function transcoderTotalStake(address _transcoder) view returns(uint256)
func (_L1BondingManager *L1BondingManagerSession) TranscoderTotalStake(_transcoder common.Address) (*big.Int, error) {
	return _L1BondingManager.Contract.TranscoderTotalStake(&_L1BondingManager.CallOpts, _transcoder)
}

// TranscoderTotalStake is a free data retrieval call binding the contract method 0x9ef9df94.
//
// Solidity: function transcoderTotalStake(address _transcoder) view returns(uint256)
func (_L1BondingManager *L1BondingManagerCallerSession) TranscoderTotalStake(_transcoder common.Address) (*big.Int, error) {
	return _L1BondingManager.Contract.TranscoderTotalStake(&_L1BondingManager.CallOpts, _transcoder)
}

// UnbondingPeriod is a free data retrieval call binding the contract method 0x6cf6d675.
//
// Solidity: function unbondingPeriod() view returns(uint64)
func (_L1BondingManager *L1BondingManagerCaller) UnbondingPeriod(opts *bind.CallOpts) (uint64, error) {
	var out []interface{}
	err := _L1BondingManager.contract.Call(opts, &out, "unbondingPeriod")

	if err != nil {
		return *new(uint64), err
	}

	out0 := *abi.ConvertType(out[0], new(uint64)).(*uint64)

	return out0, err

}

// UnbondingPeriod is a free data retrieval call binding the contract method 0x6cf6d675.
//
// Solidity: function unbondingPeriod() view returns(uint64)
func (_L1BondingManager *L1BondingManagerSession) UnbondingPeriod() (uint64, error) {
	return _L1BondingManager.Contract.UnbondingPeriod(&_L1BondingManager.CallOpts)
}

// UnbondingPeriod is a free data retrieval call binding the contract method 0x6cf6d675.
//
// Solidity: function unbondingPeriod() view returns(uint64)
func (_L1BondingManager *L1BondingManagerCallerSession) UnbondingPeriod() (uint64, error) {
	return _L1BondingManager.Contract.UnbondingPeriod(&_L1BondingManager.CallOpts)
}

// Bond is a paid mutator transaction binding the contract method 0xb78d27dc.
//
// Solidity: function bond(uint256 _amount, address _to) returns()
func (_L1BondingManager *L1BondingManagerTransactor) Bond(opts *bind.TransactOpts, _amount *big.Int, _to common.Address) (*types.Transaction, error) {
	return _L1BondingManager.contract.Transact(opts, "bond", _amount, _to)
}

// Bond is a paid mutator transaction binding the contract method 0xb78d27dc.
//
// Solidity: function bond(uint256 _amount, address _to) returns()
func (_L1BondingManager *L1BondingManagerSession) Bond(_amount *big.Int, _to common.Address) (*types.Transaction, error) {
	return _L1BondingManager.Contract.Bond(&_L1BondingManager.TransactOpts, _amount, _to)
}

// Bond is a paid mutator transaction binding the contract method 0xb78d27dc.
//
// Solidity: function bond(uint256 _amount, address _to) returns()
func (_L1BondingManager *L1BondingManagerTransactorSession) Bond(_amount *big.Int, _to common.Address) (*types.Transaction, error) {
	return _L1BondingManager.Contract.Bond(&_L1BondingManager.TransactOpts, _amount, _to)
}

// BondWithHint is a paid mutator transaction binding the contract method 0x6bd9add4.
//
// Solidity: function bondWithHint(uint256 _amount, address _to, address _oldDelegateNewPosPrev, address _oldDelegateNewPosNext, address _currDelegateNewPosPrev, address _currDelegateNewPosNext) returns()
func (_L1BondingManager *L1BondingManagerTransactor) BondWithHint(opts *bind.TransactOpts, _amount *big.Int, _to common.Address, _oldDelegateNewPosPrev common.Address, _oldDelegateNewPosNext common.Address, _currDelegateNewPosPrev common.Address, _currDelegateNewPosNext common.Address) (*types.Transaction, error) {
	return _L1BondingManager.contract.Transact(opts, "bondWithHint", _amount, _to, _oldDelegateNewPosPrev, _oldDelegateNewPosNext, _currDelegateNewPosPrev, _currDelegateNewPosNext)
}

// BondWithHint is a paid mutator transaction binding the contract method 0x6bd9add4.
//
// Solidity: function bondWithHint(uint256 _amount, address _to, address _oldDelegateNewPosPrev, address _oldDelegateNewPosNext, address _currDelegateNewPosPrev, address _currDelegateNewPosNext) returns()
func (_L1BondingManager *L1BondingManagerSession) BondWithHint(_amount *big.Int, _to common.Address, _oldDelegateNewPosPrev common.Address, _oldDelegateNewPosNext common.Address, _currDelegateNewPosPrev common.Address, _currDelegateNewPosNext common.Address) (*types.Transaction, error) {
	return _L1BondingManager.Contract.BondWithHint(&_L1BondingManager.TransactOpts, _amount, _to, _oldDelegateNewPosPrev, _oldDelegateNewPosNext, _currDelegateNewPosPrev, _currDelegateNewPosNext)
}

// BondWithHint is a paid mutator transaction binding the contract method 0x6bd9add4.
//
// Solidity: function bondWithHint(uint256 _amount, address _to, address _oldDelegateNewPosPrev, address _oldDelegateNewPosNext, address _currDelegateNewPosPrev, address _currDelegateNewPosNext) returns()
func (_L1BondingManager *L1BondingManagerTransactorSession) BondWithHint(_amount *big.Int, _to common.Address, _oldDelegateNewPosPrev common.Address, _oldDelegateNewPosNext common.Address, _currDelegateNewPosPrev common.Address, _currDelegateNewPosNext common.Address) (*types.Transaction, error) {
	return _L1BondingManager.Contract.BondWithHint(&_L1BondingManager.TransactOpts, _amount, _to, _oldDelegateNewPosPrev, _oldDelegateNewPosNext, _currDelegateNewPosPrev, _currDelegateNewPosNext)
}

// ClaimEarnings is a paid mutator transaction binding the contract method 0x24b1babf.
//
// Solidity: function claimEarnings(uint256 _endRound) returns()
func (_L1BondingManager *L1BondingManagerTransactor) ClaimEarnings(opts *bind.TransactOpts, _endRound *big.Int) (*types.Transaction, error) {
	return _L1BondingManager.contract.Transact(opts, "claimEarnings", _endRound)
}

// ClaimEarnings is a paid mutator transaction binding the contract method 0x24b1babf.
//
// Solidity: function claimEarnings(uint256 _endRound) returns()
func (_L1BondingManager *L1BondingManagerSession) ClaimEarnings(_endRound *big.Int) (*types.Transaction, error) {
	return _L1BondingManager.Contract.ClaimEarnings(&_L1BondingManager.TransactOpts, _endRound)
}

// ClaimEarnings is a paid mutator transaction binding the contract method 0x24b1babf.
//
// Solidity: function claimEarnings(uint256 _endRound) returns()
func (_L1BondingManager *L1BondingManagerTransactorSession) ClaimEarnings(_endRound *big.Int) (*types.Transaction, error) {
	return _L1BondingManager.Contract.ClaimEarnings(&_L1BondingManager.TransactOpts, _endRound)
}

// ClaimSnapshotEarnings is a paid mutator transaction binding the contract method 0xc6d63d8c.
//
// Solidity: function claimSnapshotEarnings(uint256 _pendingStake, uint256 _pendingFees, bytes32[] _earningsProof, bytes _data) returns()
func (_L1BondingManager *L1BondingManagerTransactor) ClaimSnapshotEarnings(opts *bind.TransactOpts, _pendingStake *big.Int, _pendingFees *big.Int, _earningsProof [][32]byte, _data []byte) (*types.Transaction, error) {
	return _L1BondingManager.contract.Transact(opts, "claimSnapshotEarnings", _pendingStake, _pendingFees, _earningsProof, _data)
}

// ClaimSnapshotEarnings is a paid mutator transaction binding the contract method 0xc6d63d8c.
//
// Solidity: function claimSnapshotEarnings(uint256 _pendingStake, uint256 _pendingFees, bytes32[] _earningsProof, bytes _data) returns()
func (_L1BondingManager *L1BondingManagerSession) ClaimSnapshotEarnings(_pendingStake *big.Int, _pendingFees *big.Int, _earningsProof [][32]byte, _data []byte) (*types.Transaction, error) {
	return _L1BondingManager.Contract.ClaimSnapshotEarnings(&_L1BondingManager.TransactOpts, _pendingStake, _pendingFees, _earningsProof, _data)
}

// ClaimSnapshotEarnings is a paid mutator transaction binding the contract method 0xc6d63d8c.
//
// Solidity: function claimSnapshotEarnings(uint256 _pendingStake, uint256 _pendingFees, bytes32[] _earningsProof, bytes _data) returns()
func (_L1BondingManager *L1BondingManagerTransactorSession) ClaimSnapshotEarnings(_pendingStake *big.Int, _pendingFees *big.Int, _earningsProof [][32]byte, _data []byte) (*types.Transaction, error) {
	return _L1BondingManager.Contract.ClaimSnapshotEarnings(&_L1BondingManager.TransactOpts, _pendingStake, _pendingFees, _earningsProof, _data)
}

// ExecuteLIP77 is a paid mutator transaction binding the contract method 0x34aba214.
//
// Solidity: function executeLIP77(uint256 _bondedAmount) returns()
func (_L1BondingManager *L1BondingManagerTransactor) ExecuteLIP77(opts *bind.TransactOpts, _bondedAmount *big.Int) (*types.Transaction, error) {
	return _L1BondingManager.contract.Transact(opts, "executeLIP77", _bondedAmount)
}

// ExecuteLIP77 is a paid mutator transaction binding the contract method 0x34aba214.
//
// Solidity: function executeLIP77(uint256 _bondedAmount) returns()
func (_L1BondingManager *L1BondingManagerSession) ExecuteLIP77(_bondedAmount *big.Int) (*types.Transaction, error) {
	return _L1BondingManager.Contract.ExecuteLIP77(&_L1BondingManager.TransactOpts, _bondedAmount)
}

// ExecuteLIP77 is a paid mutator transaction binding the contract method 0x34aba214.
//
// Solidity: function executeLIP77(uint256 _bondedAmount) returns()
func (_L1BondingManager *L1BondingManagerTransactorSession) ExecuteLIP77(_bondedAmount *big.Int) (*types.Transaction, error) {
	return _L1BondingManager.Contract.ExecuteLIP77(&_L1BondingManager.TransactOpts, _bondedAmount)
}

// Rebond is a paid mutator transaction binding the contract method 0xeaffb3f9.
//
// Solidity: function rebond(uint256 _unbondingLockId) returns()
func (_L1BondingManager *L1BondingManagerTransactor) Rebond(opts *bind.TransactOpts, _unbondingLockId *big.Int) (*types.Transaction, error) {
	return _L1BondingManager.contract.Transact(opts, "rebond", _unbondingLockId)
}

// Rebond is a paid mutator transaction binding the contract method 0xeaffb3f9.
//
// Solidity: function rebond(uint256 _unbondingLockId) returns()
func (_L1BondingManager *L1BondingManagerSession) Rebond(_unbondingLockId *big.Int) (*types.Transaction, error) {
	return _L1BondingManager.Contract.Rebond(&_L1BondingManager.TransactOpts, _unbondingLockId)
}

// Rebond is a paid mutator transaction binding the contract method 0xeaffb3f9.
//
// Solidity: function rebond(uint256 _unbondingLockId) returns()
func (_L1BondingManager *L1BondingManagerTransactorSession) Rebond(_unbondingLockId *big.Int) (*types.Transaction, error) {
	return _L1BondingManager.Contract.Rebond(&_L1BondingManager.TransactOpts, _unbondingLockId)
}

// RebondFromUnbonded is a paid mutator transaction binding the contract method 0x3a080e93.
//
// Solidity: function rebondFromUnbonded(address _to, uint256 _unbondingLockId) returns()
func (_L1BondingManager *L1BondingManagerTransactor) RebondFromUnbonded(opts *bind.TransactOpts, _to common.Address, _unbondingLockId *big.Int) (*types.Transaction, error) {
	return _L1BondingManager.contract.Transact(opts, "rebondFromUnbonded", _to, _unbondingLockId)
}

// RebondFromUnbonded is a paid mutator transaction binding the contract method 0x3a080e93.
//
// Solidity: function rebondFromUnbonded(address _to, uint256 _unbondingLockId) returns()
func (_L1BondingManager *L1BondingManagerSession) RebondFromUnbonded(_to common.Address, _unbondingLockId *big.Int) (*types.Transaction, error) {
	return _L1BondingManager.Contract.RebondFromUnbonded(&_L1BondingManager.TransactOpts, _to, _unbondingLockId)
}

// RebondFromUnbonded is a paid mutator transaction binding the contract method 0x3a080e93.
//
// Solidity: function rebondFromUnbonded(address _to, uint256 _unbondingLockId) returns()
func (_L1BondingManager *L1BondingManagerTransactorSession) RebondFromUnbonded(_to common.Address, _unbondingLockId *big.Int) (*types.Transaction, error) {
	return _L1BondingManager.Contract.RebondFromUnbonded(&_L1BondingManager.TransactOpts, _to, _unbondingLockId)
}

// RebondFromUnbondedWithHint is a paid mutator transaction binding the contract method 0x0584a373.
//
// Solidity: function rebondFromUnbondedWithHint(address _to, uint256 _unbondingLockId, address _newPosPrev, address _newPosNext) returns()
func (_L1BondingManager *L1BondingManagerTransactor) RebondFromUnbondedWithHint(opts *bind.TransactOpts, _to common.Address, _unbondingLockId *big.Int, _newPosPrev common.Address, _newPosNext common.Address) (*types.Transaction, error) {
	return _L1BondingManager.contract.Transact(opts, "rebondFromUnbondedWithHint", _to, _unbondingLockId, _newPosPrev, _newPosNext)
}

// RebondFromUnbondedWithHint is a paid mutator transaction binding the contract method 0x0584a373.
//
// Solidity: function rebondFromUnbondedWithHint(address _to, uint256 _unbondingLockId, address _newPosPrev, address _newPosNext) returns()
func (_L1BondingManager *L1BondingManagerSession) RebondFromUnbondedWithHint(_to common.Address, _unbondingLockId *big.Int, _newPosPrev common.Address, _newPosNext common.Address) (*types.Transaction, error) {
	return _L1BondingManager.Contract.RebondFromUnbondedWithHint(&_L1BondingManager.TransactOpts, _to, _unbondingLockId, _newPosPrev, _newPosNext)
}

// RebondFromUnbondedWithHint is a paid mutator transaction binding the contract method 0x0584a373.
//
// Solidity: function rebondFromUnbondedWithHint(address _to, uint256 _unbondingLockId, address _newPosPrev, address _newPosNext) returns()
func (_L1BondingManager *L1BondingManagerTransactorSession) RebondFromUnbondedWithHint(_to common.Address, _unbondingLockId *big.Int, _newPosPrev common.Address, _newPosNext common.Address) (*types.Transaction, error) {
	return _L1BondingManager.Contract.RebondFromUnbondedWithHint(&_L1BondingManager.TransactOpts, _to, _unbondingLockId, _newPosPrev, _newPosNext)
}

// RebondWithHint is a paid mutator transaction binding the contract method 0x7fc4606f.
//
// Solidity: function rebondWithHint(uint256 _unbondingLockId, address _newPosPrev, address _newPosNext) returns()
func (_L1BondingManager *L1BondingManagerTransactor) RebondWithHint(opts *bind.TransactOpts, _unbondingLockId *big.Int, _newPosPrev common.Address, _newPosNext common.Address) (*types.Transaction, error) {
	return _L1BondingManager.contract.Transact(opts, "rebondWithHint", _unbondingLockId, _newPosPrev, _newPosNext)
}

// RebondWithHint is a paid mutator transaction binding the contract method 0x7fc4606f.
//
// Solidity: function rebondWithHint(uint256 _unbondingLockId, address _newPosPrev, address _newPosNext) returns()
func (_L1BondingManager *L1BondingManagerSession) RebondWithHint(_unbondingLockId *big.Int, _newPosPrev common.Address, _newPosNext common.Address) (*types.Transaction, error) {
	return _L1BondingManager.Contract.RebondWithHint(&_L1BondingManager.TransactOpts, _unbondingLockId, _newPosPrev, _newPosNext)
}

// RebondWithHint is a paid mutator transaction binding the contract method 0x7fc4606f.
//
// Solidity: function rebondWithHint(uint256 _unbondingLockId, address _newPosPrev, address _newPosNext) returns()
func (_L1BondingManager *L1BondingManagerTransactorSession) RebondWithHint(_unbondingLockId *big.Int, _newPosPrev common.Address, _newPosNext common.Address) (*types.Transaction, error) {
	return _L1BondingManager.Contract.RebondWithHint(&_L1BondingManager.TransactOpts, _unbondingLockId, _newPosPrev, _newPosNext)
}

// Reward is a paid mutator transaction binding the contract method 0x228cb733.
//
// Solidity: function reward() returns()
func (_L1BondingManager *L1BondingManagerTransactor) Reward(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _L1BondingManager.contract.Transact(opts, "reward")
}

// Reward is a paid mutator transaction binding the contract method 0x228cb733.
//
// Solidity: function reward() returns()
func (_L1BondingManager *L1BondingManagerSession) Reward() (*types.Transaction, error) {
	return _L1BondingManager.Contract.Reward(&_L1BondingManager.TransactOpts)
}

// Reward is a paid mutator transaction binding the contract method 0x228cb733.
//
// Solidity: function reward() returns()
func (_L1BondingManager *L1BondingManagerTransactorSession) Reward() (*types.Transaction, error) {
	return _L1BondingManager.Contract.Reward(&_L1BondingManager.TransactOpts)
}

// RewardWithHint is a paid mutator transaction binding the contract method 0x81871056.
//
// Solidity: function rewardWithHint(address _newPosPrev, address _newPosNext) returns()
func (_L1BondingManager *L1BondingManagerTransactor) RewardWithHint(opts *bind.TransactOpts, _newPosPrev common.Address, _newPosNext common.Address) (*types.Transaction, error) {
	return _L1BondingManager.contract.Transact(opts, "rewardWithHint", _newPosPrev, _newPosNext)
}

// RewardWithHint is a paid mutator transaction binding the contract method 0x81871056.
//
// Solidity: function rewardWithHint(address _newPosPrev, address _newPosNext) returns()
func (_L1BondingManager *L1BondingManagerSession) RewardWithHint(_newPosPrev common.Address, _newPosNext common.Address) (*types.Transaction, error) {
	return _L1BondingManager.Contract.RewardWithHint(&_L1BondingManager.TransactOpts, _newPosPrev, _newPosNext)
}

// RewardWithHint is a paid mutator transaction binding the contract method 0x81871056.
//
// Solidity: function rewardWithHint(address _newPosPrev, address _newPosNext) returns()
func (_L1BondingManager *L1BondingManagerTransactorSession) RewardWithHint(_newPosPrev common.Address, _newPosNext common.Address) (*types.Transaction, error) {
	return _L1BondingManager.Contract.RewardWithHint(&_L1BondingManager.TransactOpts, _newPosPrev, _newPosNext)
}

// SetController is a paid mutator transaction binding the contract method 0x92eefe9b.
//
// Solidity: function setController(address _controller) returns()
func (_L1BondingManager *L1BondingManagerTransactor) SetController(opts *bind.TransactOpts, _controller common.Address) (*types.Transaction, error) {
	return _L1BondingManager.contract.Transact(opts, "setController", _controller)
}

// SetController is a paid mutator transaction binding the contract method 0x92eefe9b.
//
// Solidity: function setController(address _controller) returns()
func (_L1BondingManager *L1BondingManagerSession) SetController(_controller common.Address) (*types.Transaction, error) {
	return _L1BondingManager.Contract.SetController(&_L1BondingManager.TransactOpts, _controller)
}

// SetController is a paid mutator transaction binding the contract method 0x92eefe9b.
//
// Solidity: function setController(address _controller) returns()
func (_L1BondingManager *L1BondingManagerTransactorSession) SetController(_controller common.Address) (*types.Transaction, error) {
	return _L1BondingManager.Contract.SetController(&_L1BondingManager.TransactOpts, _controller)
}

// SetCurrentRoundTotalActiveStake is a paid mutator transaction binding the contract method 0x713f2216.
//
// Solidity: function setCurrentRoundTotalActiveStake() returns()
func (_L1BondingManager *L1BondingManagerTransactor) SetCurrentRoundTotalActiveStake(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _L1BondingManager.contract.Transact(opts, "setCurrentRoundTotalActiveStake")
}

// SetCurrentRoundTotalActiveStake is a paid mutator transaction binding the contract method 0x713f2216.
//
// Solidity: function setCurrentRoundTotalActiveStake() returns()
func (_L1BondingManager *L1BondingManagerSession) SetCurrentRoundTotalActiveStake() (*types.Transaction, error) {
	return _L1BondingManager.Contract.SetCurrentRoundTotalActiveStake(&_L1BondingManager.TransactOpts)
}

// SetCurrentRoundTotalActiveStake is a paid mutator transaction binding the contract method 0x713f2216.
//
// Solidity: function setCurrentRoundTotalActiveStake() returns()
func (_L1BondingManager *L1BondingManagerTransactorSession) SetCurrentRoundTotalActiveStake() (*types.Transaction, error) {
	return _L1BondingManager.Contract.SetCurrentRoundTotalActiveStake(&_L1BondingManager.TransactOpts)
}

// SetMaxEarningsClaimsRounds is a paid mutator transaction binding the contract method 0x72d9f13d.
//
// Solidity: function setMaxEarningsClaimsRounds(uint256 _maxEarningsClaimsRounds) returns()
func (_L1BondingManager *L1BondingManagerTransactor) SetMaxEarningsClaimsRounds(opts *bind.TransactOpts, _maxEarningsClaimsRounds *big.Int) (*types.Transaction, error) {
	return _L1BondingManager.contract.Transact(opts, "setMaxEarningsClaimsRounds", _maxEarningsClaimsRounds)
}

// SetMaxEarningsClaimsRounds is a paid mutator transaction binding the contract method 0x72d9f13d.
//
// Solidity: function setMaxEarningsClaimsRounds(uint256 _maxEarningsClaimsRounds) returns()
func (_L1BondingManager *L1BondingManagerSession) SetMaxEarningsClaimsRounds(_maxEarningsClaimsRounds *big.Int) (*types.Transaction, error) {
	return _L1BondingManager.Contract.SetMaxEarningsClaimsRounds(&_L1BondingManager.TransactOpts, _maxEarningsClaimsRounds)
}

// SetMaxEarningsClaimsRounds is a paid mutator transaction binding the contract method 0x72d9f13d.
//
// Solidity: function setMaxEarningsClaimsRounds(uint256 _maxEarningsClaimsRounds) returns()
func (_L1BondingManager *L1BondingManagerTransactorSession) SetMaxEarningsClaimsRounds(_maxEarningsClaimsRounds *big.Int) (*types.Transaction, error) {
	return _L1BondingManager.Contract.SetMaxEarningsClaimsRounds(&_L1BondingManager.TransactOpts, _maxEarningsClaimsRounds)
}

// SetNumActiveTranscoders is a paid mutator transaction binding the contract method 0x673a456b.
//
// Solidity: function setNumActiveTranscoders(uint256 _numActiveTranscoders) returns()
func (_L1BondingManager *L1BondingManagerTransactor) SetNumActiveTranscoders(opts *bind.TransactOpts, _numActiveTranscoders *big.Int) (*types.Transaction, error) {
	return _L1BondingManager.contract.Transact(opts, "setNumActiveTranscoders", _numActiveTranscoders)
}

// SetNumActiveTranscoders is a paid mutator transaction binding the contract method 0x673a456b.
//
// Solidity: function setNumActiveTranscoders(uint256 _numActiveTranscoders) returns()
func (_L1BondingManager *L1BondingManagerSession) SetNumActiveTranscoders(_numActiveTranscoders *big.Int) (*types.Transaction, error) {
	return _L1BondingManager.Contract.SetNumActiveTranscoders(&_L1BondingManager.TransactOpts, _numActiveTranscoders)
}

// SetNumActiveTranscoders is a paid mutator transaction binding the contract method 0x673a456b.
//
// Solidity: function setNumActiveTranscoders(uint256 _numActiveTranscoders) returns()
func (_L1BondingManager *L1BondingManagerTransactorSession) SetNumActiveTranscoders(_numActiveTranscoders *big.Int) (*types.Transaction, error) {
	return _L1BondingManager.Contract.SetNumActiveTranscoders(&_L1BondingManager.TransactOpts, _numActiveTranscoders)
}

// SetUnbondingPeriod is a paid mutator transaction binding the contract method 0xf10d1de1.
//
// Solidity: function setUnbondingPeriod(uint64 _unbondingPeriod) returns()
func (_L1BondingManager *L1BondingManagerTransactor) SetUnbondingPeriod(opts *bind.TransactOpts, _unbondingPeriod uint64) (*types.Transaction, error) {
	return _L1BondingManager.contract.Transact(opts, "setUnbondingPeriod", _unbondingPeriod)
}

// SetUnbondingPeriod is a paid mutator transaction binding the contract method 0xf10d1de1.
//
// Solidity: function setUnbondingPeriod(uint64 _unbondingPeriod) returns()
func (_L1BondingManager *L1BondingManagerSession) SetUnbondingPeriod(_unbondingPeriod uint64) (*types.Transaction, error) {
	return _L1BondingManager.Contract.SetUnbondingPeriod(&_L1BondingManager.TransactOpts, _unbondingPeriod)
}

// SetUnbondingPeriod is a paid mutator transaction binding the contract method 0xf10d1de1.
//
// Solidity: function setUnbondingPeriod(uint64 _unbondingPeriod) returns()
func (_L1BondingManager *L1BondingManagerTransactorSession) SetUnbondingPeriod(_unbondingPeriod uint64) (*types.Transaction, error) {
	return _L1BondingManager.Contract.SetUnbondingPeriod(&_L1BondingManager.TransactOpts, _unbondingPeriod)
}

// SlashTranscoder is a paid mutator transaction binding the contract method 0x22bf9d7c.
//
// Solidity: function slashTranscoder(address _transcoder, address _finder, uint256 _slashAmount, uint256 _finderFee) returns()
func (_L1BondingManager *L1BondingManagerTransactor) SlashTranscoder(opts *bind.TransactOpts, _transcoder common.Address, _finder common.Address, _slashAmount *big.Int, _finderFee *big.Int) (*types.Transaction, error) {
	return _L1BondingManager.contract.Transact(opts, "slashTranscoder", _transcoder, _finder, _slashAmount, _finderFee)
}

// SlashTranscoder is a paid mutator transaction binding the contract method 0x22bf9d7c.
//
// Solidity: function slashTranscoder(address _transcoder, address _finder, uint256 _slashAmount, uint256 _finderFee) returns()
func (_L1BondingManager *L1BondingManagerSession) SlashTranscoder(_transcoder common.Address, _finder common.Address, _slashAmount *big.Int, _finderFee *big.Int) (*types.Transaction, error) {
	return _L1BondingManager.Contract.SlashTranscoder(&_L1BondingManager.TransactOpts, _transcoder, _finder, _slashAmount, _finderFee)
}

// SlashTranscoder is a paid mutator transaction binding the contract method 0x22bf9d7c.
//
// Solidity: function slashTranscoder(address _transcoder, address _finder, uint256 _slashAmount, uint256 _finderFee) returns()
func (_L1BondingManager *L1BondingManagerTransactorSession) SlashTranscoder(_transcoder common.Address, _finder common.Address, _slashAmount *big.Int, _finderFee *big.Int) (*types.Transaction, error) {
	return _L1BondingManager.Contract.SlashTranscoder(&_L1BondingManager.TransactOpts, _transcoder, _finder, _slashAmount, _finderFee)
}

// Transcoder is a paid mutator transaction binding the contract method 0x43d3461a.
//
// Solidity: function transcoder(uint256 _rewardCut, uint256 _feeShare) returns()
func (_L1BondingManager *L1BondingManagerTransactor) Transcoder(opts *bind.TransactOpts, _rewardCut *big.Int, _feeShare *big.Int) (*types.Transaction, error) {
	return _L1BondingManager.contract.Transact(opts, "transcoder", _rewardCut, _feeShare)
}

// Transcoder is a paid mutator transaction binding the contract method 0x43d3461a.
//
// Solidity: function transcoder(uint256 _rewardCut, uint256 _feeShare) returns()
func (_L1BondingManager *L1BondingManagerSession) Transcoder(_rewardCut *big.Int, _feeShare *big.Int) (*types.Transaction, error) {
	return _L1BondingManager.Contract.Transcoder(&_L1BondingManager.TransactOpts, _rewardCut, _feeShare)
}

// Transcoder is a paid mutator transaction binding the contract method 0x43d3461a.
//
// Solidity: function transcoder(uint256 _rewardCut, uint256 _feeShare) returns()
func (_L1BondingManager *L1BondingManagerTransactorSession) Transcoder(_rewardCut *big.Int, _feeShare *big.Int) (*types.Transaction, error) {
	return _L1BondingManager.Contract.Transcoder(&_L1BondingManager.TransactOpts, _rewardCut, _feeShare)
}

// TranscoderWithHint is a paid mutator transaction binding the contract method 0x3550aa10.
//
// Solidity: function transcoderWithHint(uint256 _rewardCut, uint256 _feeShare, address _newPosPrev, address _newPosNext) returns()
func (_L1BondingManager *L1BondingManagerTransactor) TranscoderWithHint(opts *bind.TransactOpts, _rewardCut *big.Int, _feeShare *big.Int, _newPosPrev common.Address, _newPosNext common.Address) (*types.Transaction, error) {
	return _L1BondingManager.contract.Transact(opts, "transcoderWithHint", _rewardCut, _feeShare, _newPosPrev, _newPosNext)
}

// TranscoderWithHint is a paid mutator transaction binding the contract method 0x3550aa10.
//
// Solidity: function transcoderWithHint(uint256 _rewardCut, uint256 _feeShare, address _newPosPrev, address _newPosNext) returns()
func (_L1BondingManager *L1BondingManagerSession) TranscoderWithHint(_rewardCut *big.Int, _feeShare *big.Int, _newPosPrev common.Address, _newPosNext common.Address) (*types.Transaction, error) {
	return _L1BondingManager.Contract.TranscoderWithHint(&_L1BondingManager.TransactOpts, _rewardCut, _feeShare, _newPosPrev, _newPosNext)
}

// TranscoderWithHint is a paid mutator transaction binding the contract method 0x3550aa10.
//
// Solidity: function transcoderWithHint(uint256 _rewardCut, uint256 _feeShare, address _newPosPrev, address _newPosNext) returns()
func (_L1BondingManager *L1BondingManagerTransactorSession) TranscoderWithHint(_rewardCut *big.Int, _feeShare *big.Int, _newPosPrev common.Address, _newPosNext common.Address) (*types.Transaction, error) {
	return _L1BondingManager.Contract.TranscoderWithHint(&_L1BondingManager.TransactOpts, _rewardCut, _feeShare, _newPosPrev, _newPosNext)
}

// Unbond is a paid mutator transaction binding the contract method 0x27de9e32.
//
// Solidity: function unbond(uint256 _amount) returns()
func (_L1BondingManager *L1BondingManagerTransactor) Unbond(opts *bind.TransactOpts, _amount *big.Int) (*types.Transaction, error) {
	return _L1BondingManager.contract.Transact(opts, "unbond", _amount)
}

// Unbond is a paid mutator transaction binding the contract method 0x27de9e32.
//
// Solidity: function unbond(uint256 _amount) returns()
func (_L1BondingManager *L1BondingManagerSession) Unbond(_amount *big.Int) (*types.Transaction, error) {
	return _L1BondingManager.Contract.Unbond(&_L1BondingManager.TransactOpts, _amount)
}

// Unbond is a paid mutator transaction binding the contract method 0x27de9e32.
//
// Solidity: function unbond(uint256 _amount) returns()
func (_L1BondingManager *L1BondingManagerTransactorSession) Unbond(_amount *big.Int) (*types.Transaction, error) {
	return _L1BondingManager.Contract.Unbond(&_L1BondingManager.TransactOpts, _amount)
}

// UnbondWithHint is a paid mutator transaction binding the contract method 0x9500ed9b.
//
// Solidity: function unbondWithHint(uint256 _amount, address _newPosPrev, address _newPosNext) returns()
func (_L1BondingManager *L1BondingManagerTransactor) UnbondWithHint(opts *bind.TransactOpts, _amount *big.Int, _newPosPrev common.Address, _newPosNext common.Address) (*types.Transaction, error) {
	return _L1BondingManager.contract.Transact(opts, "unbondWithHint", _amount, _newPosPrev, _newPosNext)
}

// UnbondWithHint is a paid mutator transaction binding the contract method 0x9500ed9b.
//
// Solidity: function unbondWithHint(uint256 _amount, address _newPosPrev, address _newPosNext) returns()
func (_L1BondingManager *L1BondingManagerSession) UnbondWithHint(_amount *big.Int, _newPosPrev common.Address, _newPosNext common.Address) (*types.Transaction, error) {
	return _L1BondingManager.Contract.UnbondWithHint(&_L1BondingManager.TransactOpts, _amount, _newPosPrev, _newPosNext)
}

// UnbondWithHint is a paid mutator transaction binding the contract method 0x9500ed9b.
//
// Solidity: function unbondWithHint(uint256 _amount, address _newPosPrev, address _newPosNext) returns()
func (_L1BondingManager *L1BondingManagerTransactorSession) UnbondWithHint(_amount *big.Int, _newPosPrev common.Address, _newPosNext common.Address) (*types.Transaction, error) {
	return _L1BondingManager.Contract.UnbondWithHint(&_L1BondingManager.TransactOpts, _amount, _newPosPrev, _newPosNext)
}

// UpdateTranscoderWithFees is a paid mutator transaction binding the contract method 0x3aeb512c.
//
// Solidity: function updateTranscoderWithFees(address _transcoder, uint256 _fees, uint256 _round) returns()
func (_L1BondingManager *L1BondingManagerTransactor) UpdateTranscoderWithFees(opts *bind.TransactOpts, _transcoder common.Address, _fees *big.Int, _round *big.Int) (*types.Transaction, error) {
	return _L1BondingManager.contract.Transact(opts, "updateTranscoderWithFees", _transcoder, _fees, _round)
}

// UpdateTranscoderWithFees is a paid mutator transaction binding the contract method 0x3aeb512c.
//
// Solidity: function updateTranscoderWithFees(address _transcoder, uint256 _fees, uint256 _round) returns()
func (_L1BondingManager *L1BondingManagerSession) UpdateTranscoderWithFees(_transcoder common.Address, _fees *big.Int, _round *big.Int) (*types.Transaction, error) {
	return _L1BondingManager.Contract.UpdateTranscoderWithFees(&_L1BondingManager.TransactOpts, _transcoder, _fees, _round)
}

// UpdateTranscoderWithFees is a paid mutator transaction binding the contract method 0x3aeb512c.
//
// Solidity: function updateTranscoderWithFees(address _transcoder, uint256 _fees, uint256 _round) returns()
func (_L1BondingManager *L1BondingManagerTransactorSession) UpdateTranscoderWithFees(_transcoder common.Address, _fees *big.Int, _round *big.Int) (*types.Transaction, error) {
	return _L1BondingManager.Contract.UpdateTranscoderWithFees(&_L1BondingManager.TransactOpts, _transcoder, _fees, _round)
}

// WithdrawFees is a paid mutator transaction binding the contract method 0x476343ee.
//
// Solidity: function withdrawFees() returns()
func (_L1BondingManager *L1BondingManagerTransactor) WithdrawFees(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _L1BondingManager.contract.Transact(opts, "withdrawFees")
}

// WithdrawFees is a paid mutator transaction binding the contract method 0x476343ee.
//
// Solidity: function withdrawFees() returns()
func (_L1BondingManager *L1BondingManagerSession) WithdrawFees() (*types.Transaction, error) {
	return _L1BondingManager.Contract.WithdrawFees(&_L1BondingManager.TransactOpts)
}

// WithdrawFees is a paid mutator transaction binding the contract method 0x476343ee.
//
// Solidity: function withdrawFees() returns()
func (_L1BondingManager *L1BondingManagerTransactorSession) WithdrawFees() (*types.Transaction, error) {
	return _L1BondingManager.Contract.WithdrawFees(&_L1BondingManager.TransactOpts)
}

// WithdrawStake is a paid mutator transaction binding the contract method 0x25d5971f.
//
// Solidity: function withdrawStake(uint256 _unbondingLockId) returns()
func (_L1BondingManager *L1BondingManagerTransactor) WithdrawStake(opts *bind.TransactOpts, _unbondingLockId *big.Int) (*types.Transaction, error) {
	return _L1BondingManager.contract.Transact(opts, "withdrawStake", _unbondingLockId)
}

// WithdrawStake is a paid mutator transaction binding the contract method 0x25d5971f.
//
// Solidity: function withdrawStake(uint256 _unbondingLockId) returns()
func (_L1BondingManager *L1BondingManagerSession) WithdrawStake(_unbondingLockId *big.Int) (*types.Transaction, error) {
	return _L1BondingManager.Contract.WithdrawStake(&_L1BondingManager.TransactOpts, _unbondingLockId)
}

// WithdrawStake is a paid mutator transaction binding the contract method 0x25d5971f.
//
// Solidity: function withdrawStake(uint256 _unbondingLockId) returns()
func (_L1BondingManager *L1BondingManagerTransactorSession) WithdrawStake(_unbondingLockId *big.Int) (*types.Transaction, error) {
	return _L1BondingManager.Contract.WithdrawStake(&_L1BondingManager.TransactOpts, _unbondingLockId)
}

// L1BondingManagerBondIterator is returned from FilterBond and is used to iterate over the raw logs and unpacked data for Bond events raised by the L1BondingManager contract.
type L1BondingManagerBondIterator struct {
	Event *L1BondingManagerBond // Event containing the contract specifics and raw log

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
func (it *L1BondingManagerBondIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(L1BondingManagerBond)
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
		it.Event = new(L1BondingManagerBond)
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
func (it *L1BondingManagerBondIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *L1BondingManagerBondIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// L1BondingManagerBond represents a Bond event raised by the L1BondingManager contract.
type L1BondingManagerBond struct {
	NewDelegate      common.Address
	OldDelegate      common.Address
	Delegator        common.Address
	AdditionalAmount *big.Int
	BondedAmount     *big.Int
	Raw              types.Log // Blockchain specific contextual infos
}

// FilterBond is a free log retrieval operation binding the contract event 0xe5917769f276ddca9f2ee7c6b0b33e1d1e1b61008010ce622c632dd20d168a23.
//
// Solidity: event Bond(address indexed newDelegate, address indexed oldDelegate, address indexed delegator, uint256 additionalAmount, uint256 bondedAmount)
func (_L1BondingManager *L1BondingManagerFilterer) FilterBond(opts *bind.FilterOpts, newDelegate []common.Address, oldDelegate []common.Address, delegator []common.Address) (*L1BondingManagerBondIterator, error) {

	var newDelegateRule []interface{}
	for _, newDelegateItem := range newDelegate {
		newDelegateRule = append(newDelegateRule, newDelegateItem)
	}
	var oldDelegateRule []interface{}
	for _, oldDelegateItem := range oldDelegate {
		oldDelegateRule = append(oldDelegateRule, oldDelegateItem)
	}
	var delegatorRule []interface{}
	for _, delegatorItem := range delegator {
		delegatorRule = append(delegatorRule, delegatorItem)
	}

	logs, sub, err := _L1BondingManager.contract.FilterLogs(opts, "Bond", newDelegateRule, oldDelegateRule, delegatorRule)
	if err != nil {
		return nil, err
	}
	return &L1BondingManagerBondIterator{contract: _L1BondingManager.contract, event: "Bond", logs: logs, sub: sub}, nil
}

// WatchBond is a free log subscription operation binding the contract event 0xe5917769f276ddca9f2ee7c6b0b33e1d1e1b61008010ce622c632dd20d168a23.
//
// Solidity: event Bond(address indexed newDelegate, address indexed oldDelegate, address indexed delegator, uint256 additionalAmount, uint256 bondedAmount)
func (_L1BondingManager *L1BondingManagerFilterer) WatchBond(opts *bind.WatchOpts, sink chan<- *L1BondingManagerBond, newDelegate []common.Address, oldDelegate []common.Address, delegator []common.Address) (event.Subscription, error) {

	var newDelegateRule []interface{}
	for _, newDelegateItem := range newDelegate {
		newDelegateRule = append(newDelegateRule, newDelegateItem)
	}
	var oldDelegateRule []interface{}
	for _, oldDelegateItem := range oldDelegate {
		oldDelegateRule = append(oldDelegateRule, oldDelegateItem)
	}
	var delegatorRule []interface{}
	for _, delegatorItem := range delegator {
		delegatorRule = append(delegatorRule, delegatorItem)
	}

	logs, sub, err := _L1BondingManager.contract.WatchLogs(opts, "Bond", newDelegateRule, oldDelegateRule, delegatorRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(L1BondingManagerBond)
				if err := _L1BondingManager.contract.UnpackLog(event, "Bond", log); err != nil {
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

// ParseBond is a log parse operation binding the contract event 0xe5917769f276ddca9f2ee7c6b0b33e1d1e1b61008010ce622c632dd20d168a23.
//
// Solidity: event Bond(address indexed newDelegate, address indexed oldDelegate, address indexed delegator, uint256 additionalAmount, uint256 bondedAmount)
func (_L1BondingManager *L1BondingManagerFilterer) ParseBond(log types.Log) (*L1BondingManagerBond, error) {
	event := new(L1BondingManagerBond)
	if err := _L1BondingManager.contract.UnpackLog(event, "Bond", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// L1BondingManagerEarningsClaimedIterator is returned from FilterEarningsClaimed and is used to iterate over the raw logs and unpacked data for EarningsClaimed events raised by the L1BondingManager contract.
type L1BondingManagerEarningsClaimedIterator struct {
	Event *L1BondingManagerEarningsClaimed // Event containing the contract specifics and raw log

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
func (it *L1BondingManagerEarningsClaimedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(L1BondingManagerEarningsClaimed)
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
		it.Event = new(L1BondingManagerEarningsClaimed)
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
func (it *L1BondingManagerEarningsClaimedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *L1BondingManagerEarningsClaimedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// L1BondingManagerEarningsClaimed represents a EarningsClaimed event raised by the L1BondingManager contract.
type L1BondingManagerEarningsClaimed struct {
	Delegate   common.Address
	Delegator  common.Address
	Rewards    *big.Int
	Fees       *big.Int
	StartRound *big.Int
	EndRound   *big.Int
	Raw        types.Log // Blockchain specific contextual infos
}

// FilterEarningsClaimed is a free log retrieval operation binding the contract event 0xd7eab0765b772ea6ea859d5633baf737502198012e930f257f90013d9b211094.
//
// Solidity: event EarningsClaimed(address indexed delegate, address indexed delegator, uint256 rewards, uint256 fees, uint256 startRound, uint256 endRound)
func (_L1BondingManager *L1BondingManagerFilterer) FilterEarningsClaimed(opts *bind.FilterOpts, delegate []common.Address, delegator []common.Address) (*L1BondingManagerEarningsClaimedIterator, error) {

	var delegateRule []interface{}
	for _, delegateItem := range delegate {
		delegateRule = append(delegateRule, delegateItem)
	}
	var delegatorRule []interface{}
	for _, delegatorItem := range delegator {
		delegatorRule = append(delegatorRule, delegatorItem)
	}

	logs, sub, err := _L1BondingManager.contract.FilterLogs(opts, "EarningsClaimed", delegateRule, delegatorRule)
	if err != nil {
		return nil, err
	}
	return &L1BondingManagerEarningsClaimedIterator{contract: _L1BondingManager.contract, event: "EarningsClaimed", logs: logs, sub: sub}, nil
}

// WatchEarningsClaimed is a free log subscription operation binding the contract event 0xd7eab0765b772ea6ea859d5633baf737502198012e930f257f90013d9b211094.
//
// Solidity: event EarningsClaimed(address indexed delegate, address indexed delegator, uint256 rewards, uint256 fees, uint256 startRound, uint256 endRound)
func (_L1BondingManager *L1BondingManagerFilterer) WatchEarningsClaimed(opts *bind.WatchOpts, sink chan<- *L1BondingManagerEarningsClaimed, delegate []common.Address, delegator []common.Address) (event.Subscription, error) {

	var delegateRule []interface{}
	for _, delegateItem := range delegate {
		delegateRule = append(delegateRule, delegateItem)
	}
	var delegatorRule []interface{}
	for _, delegatorItem := range delegator {
		delegatorRule = append(delegatorRule, delegatorItem)
	}

	logs, sub, err := _L1BondingManager.contract.WatchLogs(opts, "EarningsClaimed", delegateRule, delegatorRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(L1BondingManagerEarningsClaimed)
				if err := _L1BondingManager.contract.UnpackLog(event, "EarningsClaimed", log); err != nil {
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

// ParseEarningsClaimed is a log parse operation binding the contract event 0xd7eab0765b772ea6ea859d5633baf737502198012e930f257f90013d9b211094.
//
// Solidity: event EarningsClaimed(address indexed delegate, address indexed delegator, uint256 rewards, uint256 fees, uint256 startRound, uint256 endRound)
func (_L1BondingManager *L1BondingManagerFilterer) ParseEarningsClaimed(log types.Log) (*L1BondingManagerEarningsClaimed, error) {
	event := new(L1BondingManagerEarningsClaimed)
	if err := _L1BondingManager.contract.UnpackLog(event, "EarningsClaimed", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// L1BondingManagerParameterUpdateIterator is returned from FilterParameterUpdate and is used to iterate over the raw logs and unpacked data for ParameterUpdate events raised by the L1BondingManager contract.
type L1BondingManagerParameterUpdateIterator struct {
	Event *L1BondingManagerParameterUpdate // Event containing the contract specifics and raw log

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
func (it *L1BondingManagerParameterUpdateIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(L1BondingManagerParameterUpdate)
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
		it.Event = new(L1BondingManagerParameterUpdate)
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
func (it *L1BondingManagerParameterUpdateIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *L1BondingManagerParameterUpdateIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// L1BondingManagerParameterUpdate represents a ParameterUpdate event raised by the L1BondingManager contract.
type L1BondingManagerParameterUpdate struct {
	Param string
	Raw   types.Log // Blockchain specific contextual infos
}

// FilterParameterUpdate is a free log retrieval operation binding the contract event 0x9f5033568d78ae30f29f01e944f97b2216493bd19d1b46d429673acff3dcd674.
//
// Solidity: event ParameterUpdate(string param)
func (_L1BondingManager *L1BondingManagerFilterer) FilterParameterUpdate(opts *bind.FilterOpts) (*L1BondingManagerParameterUpdateIterator, error) {

	logs, sub, err := _L1BondingManager.contract.FilterLogs(opts, "ParameterUpdate")
	if err != nil {
		return nil, err
	}
	return &L1BondingManagerParameterUpdateIterator{contract: _L1BondingManager.contract, event: "ParameterUpdate", logs: logs, sub: sub}, nil
}

// WatchParameterUpdate is a free log subscription operation binding the contract event 0x9f5033568d78ae30f29f01e944f97b2216493bd19d1b46d429673acff3dcd674.
//
// Solidity: event ParameterUpdate(string param)
func (_L1BondingManager *L1BondingManagerFilterer) WatchParameterUpdate(opts *bind.WatchOpts, sink chan<- *L1BondingManagerParameterUpdate) (event.Subscription, error) {

	logs, sub, err := _L1BondingManager.contract.WatchLogs(opts, "ParameterUpdate")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(L1BondingManagerParameterUpdate)
				if err := _L1BondingManager.contract.UnpackLog(event, "ParameterUpdate", log); err != nil {
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
func (_L1BondingManager *L1BondingManagerFilterer) ParseParameterUpdate(log types.Log) (*L1BondingManagerParameterUpdate, error) {
	event := new(L1BondingManagerParameterUpdate)
	if err := _L1BondingManager.contract.UnpackLog(event, "ParameterUpdate", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// L1BondingManagerRebondIterator is returned from FilterRebond and is used to iterate over the raw logs and unpacked data for Rebond events raised by the L1BondingManager contract.
type L1BondingManagerRebondIterator struct {
	Event *L1BondingManagerRebond // Event containing the contract specifics and raw log

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
func (it *L1BondingManagerRebondIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(L1BondingManagerRebond)
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
		it.Event = new(L1BondingManagerRebond)
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
func (it *L1BondingManagerRebondIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *L1BondingManagerRebondIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// L1BondingManagerRebond represents a Rebond event raised by the L1BondingManager contract.
type L1BondingManagerRebond struct {
	Delegate        common.Address
	Delegator       common.Address
	UnbondingLockId *big.Int
	Amount          *big.Int
	Raw             types.Log // Blockchain specific contextual infos
}

// FilterRebond is a free log retrieval operation binding the contract event 0x9f5b64cc71e1e26ff178caaa7877a04d8ce66fde989251870e80e6fbee690c17.
//
// Solidity: event Rebond(address indexed delegate, address indexed delegator, uint256 unbondingLockId, uint256 amount)
func (_L1BondingManager *L1BondingManagerFilterer) FilterRebond(opts *bind.FilterOpts, delegate []common.Address, delegator []common.Address) (*L1BondingManagerRebondIterator, error) {

	var delegateRule []interface{}
	for _, delegateItem := range delegate {
		delegateRule = append(delegateRule, delegateItem)
	}
	var delegatorRule []interface{}
	for _, delegatorItem := range delegator {
		delegatorRule = append(delegatorRule, delegatorItem)
	}

	logs, sub, err := _L1BondingManager.contract.FilterLogs(opts, "Rebond", delegateRule, delegatorRule)
	if err != nil {
		return nil, err
	}
	return &L1BondingManagerRebondIterator{contract: _L1BondingManager.contract, event: "Rebond", logs: logs, sub: sub}, nil
}

// WatchRebond is a free log subscription operation binding the contract event 0x9f5b64cc71e1e26ff178caaa7877a04d8ce66fde989251870e80e6fbee690c17.
//
// Solidity: event Rebond(address indexed delegate, address indexed delegator, uint256 unbondingLockId, uint256 amount)
func (_L1BondingManager *L1BondingManagerFilterer) WatchRebond(opts *bind.WatchOpts, sink chan<- *L1BondingManagerRebond, delegate []common.Address, delegator []common.Address) (event.Subscription, error) {

	var delegateRule []interface{}
	for _, delegateItem := range delegate {
		delegateRule = append(delegateRule, delegateItem)
	}
	var delegatorRule []interface{}
	for _, delegatorItem := range delegator {
		delegatorRule = append(delegatorRule, delegatorItem)
	}

	logs, sub, err := _L1BondingManager.contract.WatchLogs(opts, "Rebond", delegateRule, delegatorRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(L1BondingManagerRebond)
				if err := _L1BondingManager.contract.UnpackLog(event, "Rebond", log); err != nil {
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

// ParseRebond is a log parse operation binding the contract event 0x9f5b64cc71e1e26ff178caaa7877a04d8ce66fde989251870e80e6fbee690c17.
//
// Solidity: event Rebond(address indexed delegate, address indexed delegator, uint256 unbondingLockId, uint256 amount)
func (_L1BondingManager *L1BondingManagerFilterer) ParseRebond(log types.Log) (*L1BondingManagerRebond, error) {
	event := new(L1BondingManagerRebond)
	if err := _L1BondingManager.contract.UnpackLog(event, "Rebond", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// L1BondingManagerRewardIterator is returned from FilterReward and is used to iterate over the raw logs and unpacked data for Reward events raised by the L1BondingManager contract.
type L1BondingManagerRewardIterator struct {
	Event *L1BondingManagerReward // Event containing the contract specifics and raw log

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
func (it *L1BondingManagerRewardIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(L1BondingManagerReward)
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
		it.Event = new(L1BondingManagerReward)
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
func (it *L1BondingManagerRewardIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *L1BondingManagerRewardIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// L1BondingManagerReward represents a Reward event raised by the L1BondingManager contract.
type L1BondingManagerReward struct {
	Transcoder common.Address
	Amount     *big.Int
	Raw        types.Log // Blockchain specific contextual infos
}

// FilterReward is a free log retrieval operation binding the contract event 0x619caafabdd75649b302ba8419e48cccf64f37f1983ac4727cfb38b57703ffc9.
//
// Solidity: event Reward(address indexed transcoder, uint256 amount)
func (_L1BondingManager *L1BondingManagerFilterer) FilterReward(opts *bind.FilterOpts, transcoder []common.Address) (*L1BondingManagerRewardIterator, error) {

	var transcoderRule []interface{}
	for _, transcoderItem := range transcoder {
		transcoderRule = append(transcoderRule, transcoderItem)
	}

	logs, sub, err := _L1BondingManager.contract.FilterLogs(opts, "Reward", transcoderRule)
	if err != nil {
		return nil, err
	}
	return &L1BondingManagerRewardIterator{contract: _L1BondingManager.contract, event: "Reward", logs: logs, sub: sub}, nil
}

// WatchReward is a free log subscription operation binding the contract event 0x619caafabdd75649b302ba8419e48cccf64f37f1983ac4727cfb38b57703ffc9.
//
// Solidity: event Reward(address indexed transcoder, uint256 amount)
func (_L1BondingManager *L1BondingManagerFilterer) WatchReward(opts *bind.WatchOpts, sink chan<- *L1BondingManagerReward, transcoder []common.Address) (event.Subscription, error) {

	var transcoderRule []interface{}
	for _, transcoderItem := range transcoder {
		transcoderRule = append(transcoderRule, transcoderItem)
	}

	logs, sub, err := _L1BondingManager.contract.WatchLogs(opts, "Reward", transcoderRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(L1BondingManagerReward)
				if err := _L1BondingManager.contract.UnpackLog(event, "Reward", log); err != nil {
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

// ParseReward is a log parse operation binding the contract event 0x619caafabdd75649b302ba8419e48cccf64f37f1983ac4727cfb38b57703ffc9.
//
// Solidity: event Reward(address indexed transcoder, uint256 amount)
func (_L1BondingManager *L1BondingManagerFilterer) ParseReward(log types.Log) (*L1BondingManagerReward, error) {
	event := new(L1BondingManagerReward)
	if err := _L1BondingManager.contract.UnpackLog(event, "Reward", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// L1BondingManagerSetControllerIterator is returned from FilterSetController and is used to iterate over the raw logs and unpacked data for SetController events raised by the L1BondingManager contract.
type L1BondingManagerSetControllerIterator struct {
	Event *L1BondingManagerSetController // Event containing the contract specifics and raw log

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
func (it *L1BondingManagerSetControllerIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(L1BondingManagerSetController)
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
		it.Event = new(L1BondingManagerSetController)
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
func (it *L1BondingManagerSetControllerIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *L1BondingManagerSetControllerIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// L1BondingManagerSetController represents a SetController event raised by the L1BondingManager contract.
type L1BondingManagerSetController struct {
	Controller common.Address
	Raw        types.Log // Blockchain specific contextual infos
}

// FilterSetController is a free log retrieval operation binding the contract event 0x4ff638452bbf33c012645d18ae6f05515ff5f2d1dfb0cece8cbf018c60903f70.
//
// Solidity: event SetController(address controller)
func (_L1BondingManager *L1BondingManagerFilterer) FilterSetController(opts *bind.FilterOpts) (*L1BondingManagerSetControllerIterator, error) {

	logs, sub, err := _L1BondingManager.contract.FilterLogs(opts, "SetController")
	if err != nil {
		return nil, err
	}
	return &L1BondingManagerSetControllerIterator{contract: _L1BondingManager.contract, event: "SetController", logs: logs, sub: sub}, nil
}

// WatchSetController is a free log subscription operation binding the contract event 0x4ff638452bbf33c012645d18ae6f05515ff5f2d1dfb0cece8cbf018c60903f70.
//
// Solidity: event SetController(address controller)
func (_L1BondingManager *L1BondingManagerFilterer) WatchSetController(opts *bind.WatchOpts, sink chan<- *L1BondingManagerSetController) (event.Subscription, error) {

	logs, sub, err := _L1BondingManager.contract.WatchLogs(opts, "SetController")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(L1BondingManagerSetController)
				if err := _L1BondingManager.contract.UnpackLog(event, "SetController", log); err != nil {
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
func (_L1BondingManager *L1BondingManagerFilterer) ParseSetController(log types.Log) (*L1BondingManagerSetController, error) {
	event := new(L1BondingManagerSetController)
	if err := _L1BondingManager.contract.UnpackLog(event, "SetController", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// L1BondingManagerTranscoderActivatedIterator is returned from FilterTranscoderActivated and is used to iterate over the raw logs and unpacked data for TranscoderActivated events raised by the L1BondingManager contract.
type L1BondingManagerTranscoderActivatedIterator struct {
	Event *L1BondingManagerTranscoderActivated // Event containing the contract specifics and raw log

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
func (it *L1BondingManagerTranscoderActivatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(L1BondingManagerTranscoderActivated)
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
		it.Event = new(L1BondingManagerTranscoderActivated)
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
func (it *L1BondingManagerTranscoderActivatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *L1BondingManagerTranscoderActivatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// L1BondingManagerTranscoderActivated represents a TranscoderActivated event raised by the L1BondingManager contract.
type L1BondingManagerTranscoderActivated struct {
	Transcoder      common.Address
	ActivationRound *big.Int
	Raw             types.Log // Blockchain specific contextual infos
}

// FilterTranscoderActivated is a free log retrieval operation binding the contract event 0x65d72d782835d64c3287844a829608d5abdc7e864cc9affe96d910ab3db665e9.
//
// Solidity: event TranscoderActivated(address indexed transcoder, uint256 activationRound)
func (_L1BondingManager *L1BondingManagerFilterer) FilterTranscoderActivated(opts *bind.FilterOpts, transcoder []common.Address) (*L1BondingManagerTranscoderActivatedIterator, error) {

	var transcoderRule []interface{}
	for _, transcoderItem := range transcoder {
		transcoderRule = append(transcoderRule, transcoderItem)
	}

	logs, sub, err := _L1BondingManager.contract.FilterLogs(opts, "TranscoderActivated", transcoderRule)
	if err != nil {
		return nil, err
	}
	return &L1BondingManagerTranscoderActivatedIterator{contract: _L1BondingManager.contract, event: "TranscoderActivated", logs: logs, sub: sub}, nil
}

// WatchTranscoderActivated is a free log subscription operation binding the contract event 0x65d72d782835d64c3287844a829608d5abdc7e864cc9affe96d910ab3db665e9.
//
// Solidity: event TranscoderActivated(address indexed transcoder, uint256 activationRound)
func (_L1BondingManager *L1BondingManagerFilterer) WatchTranscoderActivated(opts *bind.WatchOpts, sink chan<- *L1BondingManagerTranscoderActivated, transcoder []common.Address) (event.Subscription, error) {

	var transcoderRule []interface{}
	for _, transcoderItem := range transcoder {
		transcoderRule = append(transcoderRule, transcoderItem)
	}

	logs, sub, err := _L1BondingManager.contract.WatchLogs(opts, "TranscoderActivated", transcoderRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(L1BondingManagerTranscoderActivated)
				if err := _L1BondingManager.contract.UnpackLog(event, "TranscoderActivated", log); err != nil {
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

// ParseTranscoderActivated is a log parse operation binding the contract event 0x65d72d782835d64c3287844a829608d5abdc7e864cc9affe96d910ab3db665e9.
//
// Solidity: event TranscoderActivated(address indexed transcoder, uint256 activationRound)
func (_L1BondingManager *L1BondingManagerFilterer) ParseTranscoderActivated(log types.Log) (*L1BondingManagerTranscoderActivated, error) {
	event := new(L1BondingManagerTranscoderActivated)
	if err := _L1BondingManager.contract.UnpackLog(event, "TranscoderActivated", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// L1BondingManagerTranscoderDeactivatedIterator is returned from FilterTranscoderDeactivated and is used to iterate over the raw logs and unpacked data for TranscoderDeactivated events raised by the L1BondingManager contract.
type L1BondingManagerTranscoderDeactivatedIterator struct {
	Event *L1BondingManagerTranscoderDeactivated // Event containing the contract specifics and raw log

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
func (it *L1BondingManagerTranscoderDeactivatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(L1BondingManagerTranscoderDeactivated)
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
		it.Event = new(L1BondingManagerTranscoderDeactivated)
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
func (it *L1BondingManagerTranscoderDeactivatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *L1BondingManagerTranscoderDeactivatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// L1BondingManagerTranscoderDeactivated represents a TranscoderDeactivated event raised by the L1BondingManager contract.
type L1BondingManagerTranscoderDeactivated struct {
	Transcoder        common.Address
	DeactivationRound *big.Int
	Raw               types.Log // Blockchain specific contextual infos
}

// FilterTranscoderDeactivated is a free log retrieval operation binding the contract event 0xfee3e693fc72d0a0a673805f3e606c551f4c677b9072444b90dd2d0406bc995c.
//
// Solidity: event TranscoderDeactivated(address indexed transcoder, uint256 deactivationRound)
func (_L1BondingManager *L1BondingManagerFilterer) FilterTranscoderDeactivated(opts *bind.FilterOpts, transcoder []common.Address) (*L1BondingManagerTranscoderDeactivatedIterator, error) {

	var transcoderRule []interface{}
	for _, transcoderItem := range transcoder {
		transcoderRule = append(transcoderRule, transcoderItem)
	}

	logs, sub, err := _L1BondingManager.contract.FilterLogs(opts, "TranscoderDeactivated", transcoderRule)
	if err != nil {
		return nil, err
	}
	return &L1BondingManagerTranscoderDeactivatedIterator{contract: _L1BondingManager.contract, event: "TranscoderDeactivated", logs: logs, sub: sub}, nil
}

// WatchTranscoderDeactivated is a free log subscription operation binding the contract event 0xfee3e693fc72d0a0a673805f3e606c551f4c677b9072444b90dd2d0406bc995c.
//
// Solidity: event TranscoderDeactivated(address indexed transcoder, uint256 deactivationRound)
func (_L1BondingManager *L1BondingManagerFilterer) WatchTranscoderDeactivated(opts *bind.WatchOpts, sink chan<- *L1BondingManagerTranscoderDeactivated, transcoder []common.Address) (event.Subscription, error) {

	var transcoderRule []interface{}
	for _, transcoderItem := range transcoder {
		transcoderRule = append(transcoderRule, transcoderItem)
	}

	logs, sub, err := _L1BondingManager.contract.WatchLogs(opts, "TranscoderDeactivated", transcoderRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(L1BondingManagerTranscoderDeactivated)
				if err := _L1BondingManager.contract.UnpackLog(event, "TranscoderDeactivated", log); err != nil {
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

// ParseTranscoderDeactivated is a log parse operation binding the contract event 0xfee3e693fc72d0a0a673805f3e606c551f4c677b9072444b90dd2d0406bc995c.
//
// Solidity: event TranscoderDeactivated(address indexed transcoder, uint256 deactivationRound)
func (_L1BondingManager *L1BondingManagerFilterer) ParseTranscoderDeactivated(log types.Log) (*L1BondingManagerTranscoderDeactivated, error) {
	event := new(L1BondingManagerTranscoderDeactivated)
	if err := _L1BondingManager.contract.UnpackLog(event, "TranscoderDeactivated", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// L1BondingManagerTranscoderSlashedIterator is returned from FilterTranscoderSlashed and is used to iterate over the raw logs and unpacked data for TranscoderSlashed events raised by the L1BondingManager contract.
type L1BondingManagerTranscoderSlashedIterator struct {
	Event *L1BondingManagerTranscoderSlashed // Event containing the contract specifics and raw log

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
func (it *L1BondingManagerTranscoderSlashedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(L1BondingManagerTranscoderSlashed)
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
		it.Event = new(L1BondingManagerTranscoderSlashed)
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
func (it *L1BondingManagerTranscoderSlashedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *L1BondingManagerTranscoderSlashedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// L1BondingManagerTranscoderSlashed represents a TranscoderSlashed event raised by the L1BondingManager contract.
type L1BondingManagerTranscoderSlashed struct {
	Transcoder   common.Address
	Finder       common.Address
	Penalty      *big.Int
	FinderReward *big.Int
	Raw          types.Log // Blockchain specific contextual infos
}

// FilterTranscoderSlashed is a free log retrieval operation binding the contract event 0xf4b71fed8e2c9a8c67c388bc6d35ad20b9368a24eed6d565459f2b277b6c0c22.
//
// Solidity: event TranscoderSlashed(address indexed transcoder, address finder, uint256 penalty, uint256 finderReward)
func (_L1BondingManager *L1BondingManagerFilterer) FilterTranscoderSlashed(opts *bind.FilterOpts, transcoder []common.Address) (*L1BondingManagerTranscoderSlashedIterator, error) {

	var transcoderRule []interface{}
	for _, transcoderItem := range transcoder {
		transcoderRule = append(transcoderRule, transcoderItem)
	}

	logs, sub, err := _L1BondingManager.contract.FilterLogs(opts, "TranscoderSlashed", transcoderRule)
	if err != nil {
		return nil, err
	}
	return &L1BondingManagerTranscoderSlashedIterator{contract: _L1BondingManager.contract, event: "TranscoderSlashed", logs: logs, sub: sub}, nil
}

// WatchTranscoderSlashed is a free log subscription operation binding the contract event 0xf4b71fed8e2c9a8c67c388bc6d35ad20b9368a24eed6d565459f2b277b6c0c22.
//
// Solidity: event TranscoderSlashed(address indexed transcoder, address finder, uint256 penalty, uint256 finderReward)
func (_L1BondingManager *L1BondingManagerFilterer) WatchTranscoderSlashed(opts *bind.WatchOpts, sink chan<- *L1BondingManagerTranscoderSlashed, transcoder []common.Address) (event.Subscription, error) {

	var transcoderRule []interface{}
	for _, transcoderItem := range transcoder {
		transcoderRule = append(transcoderRule, transcoderItem)
	}

	logs, sub, err := _L1BondingManager.contract.WatchLogs(opts, "TranscoderSlashed", transcoderRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(L1BondingManagerTranscoderSlashed)
				if err := _L1BondingManager.contract.UnpackLog(event, "TranscoderSlashed", log); err != nil {
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

// ParseTranscoderSlashed is a log parse operation binding the contract event 0xf4b71fed8e2c9a8c67c388bc6d35ad20b9368a24eed6d565459f2b277b6c0c22.
//
// Solidity: event TranscoderSlashed(address indexed transcoder, address finder, uint256 penalty, uint256 finderReward)
func (_L1BondingManager *L1BondingManagerFilterer) ParseTranscoderSlashed(log types.Log) (*L1BondingManagerTranscoderSlashed, error) {
	event := new(L1BondingManagerTranscoderSlashed)
	if err := _L1BondingManager.contract.UnpackLog(event, "TranscoderSlashed", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// L1BondingManagerTranscoderUpdateIterator is returned from FilterTranscoderUpdate and is used to iterate over the raw logs and unpacked data for TranscoderUpdate events raised by the L1BondingManager contract.
type L1BondingManagerTranscoderUpdateIterator struct {
	Event *L1BondingManagerTranscoderUpdate // Event containing the contract specifics and raw log

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
func (it *L1BondingManagerTranscoderUpdateIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(L1BondingManagerTranscoderUpdate)
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
		it.Event = new(L1BondingManagerTranscoderUpdate)
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
func (it *L1BondingManagerTranscoderUpdateIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *L1BondingManagerTranscoderUpdateIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// L1BondingManagerTranscoderUpdate represents a TranscoderUpdate event raised by the L1BondingManager contract.
type L1BondingManagerTranscoderUpdate struct {
	Transcoder common.Address
	RewardCut  *big.Int
	FeeShare   *big.Int
	Raw        types.Log // Blockchain specific contextual infos
}

// FilterTranscoderUpdate is a free log retrieval operation binding the contract event 0x7346854431dbb3eb8e373c604abf89e90f4865b8447e1e2834d7b3e4677bf544.
//
// Solidity: event TranscoderUpdate(address indexed transcoder, uint256 rewardCut, uint256 feeShare)
func (_L1BondingManager *L1BondingManagerFilterer) FilterTranscoderUpdate(opts *bind.FilterOpts, transcoder []common.Address) (*L1BondingManagerTranscoderUpdateIterator, error) {

	var transcoderRule []interface{}
	for _, transcoderItem := range transcoder {
		transcoderRule = append(transcoderRule, transcoderItem)
	}

	logs, sub, err := _L1BondingManager.contract.FilterLogs(opts, "TranscoderUpdate", transcoderRule)
	if err != nil {
		return nil, err
	}
	return &L1BondingManagerTranscoderUpdateIterator{contract: _L1BondingManager.contract, event: "TranscoderUpdate", logs: logs, sub: sub}, nil
}

// WatchTranscoderUpdate is a free log subscription operation binding the contract event 0x7346854431dbb3eb8e373c604abf89e90f4865b8447e1e2834d7b3e4677bf544.
//
// Solidity: event TranscoderUpdate(address indexed transcoder, uint256 rewardCut, uint256 feeShare)
func (_L1BondingManager *L1BondingManagerFilterer) WatchTranscoderUpdate(opts *bind.WatchOpts, sink chan<- *L1BondingManagerTranscoderUpdate, transcoder []common.Address) (event.Subscription, error) {

	var transcoderRule []interface{}
	for _, transcoderItem := range transcoder {
		transcoderRule = append(transcoderRule, transcoderItem)
	}

	logs, sub, err := _L1BondingManager.contract.WatchLogs(opts, "TranscoderUpdate", transcoderRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(L1BondingManagerTranscoderUpdate)
				if err := _L1BondingManager.contract.UnpackLog(event, "TranscoderUpdate", log); err != nil {
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

// ParseTranscoderUpdate is a log parse operation binding the contract event 0x7346854431dbb3eb8e373c604abf89e90f4865b8447e1e2834d7b3e4677bf544.
//
// Solidity: event TranscoderUpdate(address indexed transcoder, uint256 rewardCut, uint256 feeShare)
func (_L1BondingManager *L1BondingManagerFilterer) ParseTranscoderUpdate(log types.Log) (*L1BondingManagerTranscoderUpdate, error) {
	event := new(L1BondingManagerTranscoderUpdate)
	if err := _L1BondingManager.contract.UnpackLog(event, "TranscoderUpdate", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// L1BondingManagerUnbondIterator is returned from FilterUnbond and is used to iterate over the raw logs and unpacked data for Unbond events raised by the L1BondingManager contract.
type L1BondingManagerUnbondIterator struct {
	Event *L1BondingManagerUnbond // Event containing the contract specifics and raw log

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
func (it *L1BondingManagerUnbondIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(L1BondingManagerUnbond)
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
		it.Event = new(L1BondingManagerUnbond)
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
func (it *L1BondingManagerUnbondIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *L1BondingManagerUnbondIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// L1BondingManagerUnbond represents a Unbond event raised by the L1BondingManager contract.
type L1BondingManagerUnbond struct {
	Delegate        common.Address
	Delegator       common.Address
	UnbondingLockId *big.Int
	Amount          *big.Int
	WithdrawRound   *big.Int
	Raw             types.Log // Blockchain specific contextual infos
}

// FilterUnbond is a free log retrieval operation binding the contract event 0x2d5d98d189bee5496a08db2a5948cb7e5e786f09d17d0c3f228eb41776c24a06.
//
// Solidity: event Unbond(address indexed delegate, address indexed delegator, uint256 unbondingLockId, uint256 amount, uint256 withdrawRound)
func (_L1BondingManager *L1BondingManagerFilterer) FilterUnbond(opts *bind.FilterOpts, delegate []common.Address, delegator []common.Address) (*L1BondingManagerUnbondIterator, error) {

	var delegateRule []interface{}
	for _, delegateItem := range delegate {
		delegateRule = append(delegateRule, delegateItem)
	}
	var delegatorRule []interface{}
	for _, delegatorItem := range delegator {
		delegatorRule = append(delegatorRule, delegatorItem)
	}

	logs, sub, err := _L1BondingManager.contract.FilterLogs(opts, "Unbond", delegateRule, delegatorRule)
	if err != nil {
		return nil, err
	}
	return &L1BondingManagerUnbondIterator{contract: _L1BondingManager.contract, event: "Unbond", logs: logs, sub: sub}, nil
}

// WatchUnbond is a free log subscription operation binding the contract event 0x2d5d98d189bee5496a08db2a5948cb7e5e786f09d17d0c3f228eb41776c24a06.
//
// Solidity: event Unbond(address indexed delegate, address indexed delegator, uint256 unbondingLockId, uint256 amount, uint256 withdrawRound)
func (_L1BondingManager *L1BondingManagerFilterer) WatchUnbond(opts *bind.WatchOpts, sink chan<- *L1BondingManagerUnbond, delegate []common.Address, delegator []common.Address) (event.Subscription, error) {

	var delegateRule []interface{}
	for _, delegateItem := range delegate {
		delegateRule = append(delegateRule, delegateItem)
	}
	var delegatorRule []interface{}
	for _, delegatorItem := range delegator {
		delegatorRule = append(delegatorRule, delegatorItem)
	}

	logs, sub, err := _L1BondingManager.contract.WatchLogs(opts, "Unbond", delegateRule, delegatorRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(L1BondingManagerUnbond)
				if err := _L1BondingManager.contract.UnpackLog(event, "Unbond", log); err != nil {
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

// ParseUnbond is a log parse operation binding the contract event 0x2d5d98d189bee5496a08db2a5948cb7e5e786f09d17d0c3f228eb41776c24a06.
//
// Solidity: event Unbond(address indexed delegate, address indexed delegator, uint256 unbondingLockId, uint256 amount, uint256 withdrawRound)
func (_L1BondingManager *L1BondingManagerFilterer) ParseUnbond(log types.Log) (*L1BondingManagerUnbond, error) {
	event := new(L1BondingManagerUnbond)
	if err := _L1BondingManager.contract.UnpackLog(event, "Unbond", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// L1BondingManagerWithdrawFeesIterator is returned from FilterWithdrawFees and is used to iterate over the raw logs and unpacked data for WithdrawFees events raised by the L1BondingManager contract.
type L1BondingManagerWithdrawFeesIterator struct {
	Event *L1BondingManagerWithdrawFees // Event containing the contract specifics and raw log

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
func (it *L1BondingManagerWithdrawFeesIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(L1BondingManagerWithdrawFees)
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
		it.Event = new(L1BondingManagerWithdrawFees)
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
func (it *L1BondingManagerWithdrawFeesIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *L1BondingManagerWithdrawFeesIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// L1BondingManagerWithdrawFees represents a WithdrawFees event raised by the L1BondingManager contract.
type L1BondingManagerWithdrawFees struct {
	Delegator common.Address
	Raw       types.Log // Blockchain specific contextual infos
}

// FilterWithdrawFees is a free log retrieval operation binding the contract event 0xd3719f04262b628e1d01a6ed24707f542cda51f144b5271149c7d0419436d00c.
//
// Solidity: event WithdrawFees(address indexed delegator)
func (_L1BondingManager *L1BondingManagerFilterer) FilterWithdrawFees(opts *bind.FilterOpts, delegator []common.Address) (*L1BondingManagerWithdrawFeesIterator, error) {

	var delegatorRule []interface{}
	for _, delegatorItem := range delegator {
		delegatorRule = append(delegatorRule, delegatorItem)
	}

	logs, sub, err := _L1BondingManager.contract.FilterLogs(opts, "WithdrawFees", delegatorRule)
	if err != nil {
		return nil, err
	}
	return &L1BondingManagerWithdrawFeesIterator{contract: _L1BondingManager.contract, event: "WithdrawFees", logs: logs, sub: sub}, nil
}

// WatchWithdrawFees is a free log subscription operation binding the contract event 0xd3719f04262b628e1d01a6ed24707f542cda51f144b5271149c7d0419436d00c.
//
// Solidity: event WithdrawFees(address indexed delegator)
func (_L1BondingManager *L1BondingManagerFilterer) WatchWithdrawFees(opts *bind.WatchOpts, sink chan<- *L1BondingManagerWithdrawFees, delegator []common.Address) (event.Subscription, error) {

	var delegatorRule []interface{}
	for _, delegatorItem := range delegator {
		delegatorRule = append(delegatorRule, delegatorItem)
	}

	logs, sub, err := _L1BondingManager.contract.WatchLogs(opts, "WithdrawFees", delegatorRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(L1BondingManagerWithdrawFees)
				if err := _L1BondingManager.contract.UnpackLog(event, "WithdrawFees", log); err != nil {
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

// ParseWithdrawFees is a log parse operation binding the contract event 0xd3719f04262b628e1d01a6ed24707f542cda51f144b5271149c7d0419436d00c.
//
// Solidity: event WithdrawFees(address indexed delegator)
func (_L1BondingManager *L1BondingManagerFilterer) ParseWithdrawFees(log types.Log) (*L1BondingManagerWithdrawFees, error) {
	event := new(L1BondingManagerWithdrawFees)
	if err := _L1BondingManager.contract.UnpackLog(event, "WithdrawFees", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// L1BondingManagerWithdrawStakeIterator is returned from FilterWithdrawStake and is used to iterate over the raw logs and unpacked data for WithdrawStake events raised by the L1BondingManager contract.
type L1BondingManagerWithdrawStakeIterator struct {
	Event *L1BondingManagerWithdrawStake // Event containing the contract specifics and raw log

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
func (it *L1BondingManagerWithdrawStakeIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(L1BondingManagerWithdrawStake)
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
		it.Event = new(L1BondingManagerWithdrawStake)
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
func (it *L1BondingManagerWithdrawStakeIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *L1BondingManagerWithdrawStakeIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// L1BondingManagerWithdrawStake represents a WithdrawStake event raised by the L1BondingManager contract.
type L1BondingManagerWithdrawStake struct {
	Delegator       common.Address
	UnbondingLockId *big.Int
	Amount          *big.Int
	WithdrawRound   *big.Int
	Raw             types.Log // Blockchain specific contextual infos
}

// FilterWithdrawStake is a free log retrieval operation binding the contract event 0x1340f1a8f3d456a649e1a12071dfa15655e3d09252131d0f980c3b405cc8dd2e.
//
// Solidity: event WithdrawStake(address indexed delegator, uint256 unbondingLockId, uint256 amount, uint256 withdrawRound)
func (_L1BondingManager *L1BondingManagerFilterer) FilterWithdrawStake(opts *bind.FilterOpts, delegator []common.Address) (*L1BondingManagerWithdrawStakeIterator, error) {

	var delegatorRule []interface{}
	for _, delegatorItem := range delegator {
		delegatorRule = append(delegatorRule, delegatorItem)
	}

	logs, sub, err := _L1BondingManager.contract.FilterLogs(opts, "WithdrawStake", delegatorRule)
	if err != nil {
		return nil, err
	}
	return &L1BondingManagerWithdrawStakeIterator{contract: _L1BondingManager.contract, event: "WithdrawStake", logs: logs, sub: sub}, nil
}

// WatchWithdrawStake is a free log subscription operation binding the contract event 0x1340f1a8f3d456a649e1a12071dfa15655e3d09252131d0f980c3b405cc8dd2e.
//
// Solidity: event WithdrawStake(address indexed delegator, uint256 unbondingLockId, uint256 amount, uint256 withdrawRound)
func (_L1BondingManager *L1BondingManagerFilterer) WatchWithdrawStake(opts *bind.WatchOpts, sink chan<- *L1BondingManagerWithdrawStake, delegator []common.Address) (event.Subscription, error) {

	var delegatorRule []interface{}
	for _, delegatorItem := range delegator {
		delegatorRule = append(delegatorRule, delegatorItem)
	}

	logs, sub, err := _L1BondingManager.contract.WatchLogs(opts, "WithdrawStake", delegatorRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(L1BondingManagerWithdrawStake)
				if err := _L1BondingManager.contract.UnpackLog(event, "WithdrawStake", log); err != nil {
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

// ParseWithdrawStake is a log parse operation binding the contract event 0x1340f1a8f3d456a649e1a12071dfa15655e3d09252131d0f980c3b405cc8dd2e.
//
// Solidity: event WithdrawStake(address indexed delegator, uint256 unbondingLockId, uint256 amount, uint256 withdrawRound)
func (_L1BondingManager *L1BondingManagerFilterer) ParseWithdrawStake(log types.Log) (*L1BondingManagerWithdrawStake, error) {
	event := new(L1BondingManagerWithdrawStake)
	if err := _L1BondingManager.contract.UnpackLog(event, "WithdrawStake", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}
