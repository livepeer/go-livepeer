package types

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

var (
	ErrUnknownTranscoderStatus = fmt.Errorf("unknown transcoder status")
	ErrUnknownDelegatorStatus  = fmt.Errorf("unknown delegator status")
)

type Transcoder struct {
	Address           common.Address
	ServiceURI        string
	LastRewardRound   *big.Int
	RewardCut         *big.Int
	FeeShare          *big.Int
	DelegatedStake    *big.Int
	ActivationRound   *big.Int
	DeactivationRound *big.Int
	Active            bool
	Status            string
	PricePerPixel     *big.Rat
}

func ParseTranscoderStatus(s uint8) (string, error) {
	switch s {
	case 0:
		return "Not Registered", nil
	case 1:
		return "Registered", nil
	default:
		return "", ErrUnknownTranscoderStatus
	}
}

type Delegator struct {
	Address             common.Address
	BondedAmount        *big.Int
	Fees                *big.Int
	DelegateAddress     common.Address
	DelegatedAmount     *big.Int
	StartRound          *big.Int
	LastClaimRound      *big.Int
	NextUnbondingLockId *big.Int
	PendingStake        *big.Int
	PendingFees         *big.Int
	Status              string
}

func ParseDelegatorStatus(s uint8) (string, error) {
	switch s {
	case 0:
		return "Pending", nil
	case 1:
		return "Bonded", nil
	case 2:
		return "Unbonded", nil
	default:
		return "", ErrUnknownDelegatorStatus
	}
}

type UnbondingLock struct {
	ID               *big.Int
	DelegatorAddress common.Address
	Amount           *big.Int
	WithdrawRound    *big.Int
}

type TokenPools struct {
	RewardPool     *big.Int
	FeePool        *big.Int
	TotalStake     *big.Int
	ClaimableStake *big.Int
}

type ProtocolParameters struct {
	NumActiveTranscoders          *big.Int
	RoundLength                   *big.Int
	RoundLockAmount               *big.Int
	UnbondingPeriod               uint64
	VerificationRate              uint64
	VerificationPeriod            *big.Int
	SlashingPeriod                *big.Int
	FailedVerificationSlashAmount *big.Int
	MissedVerificationSlashAmount *big.Int
	DoubleClaimSegmentSlashAmount *big.Int
	FinderFee                     *big.Int
	Inflation                     *big.Int
	InflationChange               *big.Int
	TargetBondingRate             *big.Int
	VerificationCodeHash          string
	TotalBonded                   *big.Int
	TotalSupply                   *big.Int
	Paused                        bool
}

type VoteChoice int

const (
	Yes VoteChoice = iota
	No
)

var VoteChoices = []VoteChoice{Yes, No}

func (v VoteChoice) String() string {
	switch v {
	case Yes:
		return "Yes"
	case No:
		return "No"
	default:
		return ""
	}
}

func (v VoteChoice) IsValid() bool {
	return v == Yes || v == No
}
