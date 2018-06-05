package types

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	ffmpeg "github.com/livepeer/lpms/ffmpeg"
)

var (
	ErrUnknownTranscoderStatus = fmt.Errorf("unknown transcoder status")
	ErrUnknownDelegatorStatus  = fmt.Errorf("unknown delegator status")
	ErrUnknownJobStatus        = fmt.Errorf("unknown job status")
	ErrUnknownClaimStatus      = fmt.Errorf("unknown claim status")
)

type Transcoder struct {
	Address                common.Address
	ServiceURI             string
	LastRewardRound        *big.Int
	RewardCut              *big.Int
	FeeShare               *big.Int
	PricePerSegment        *big.Int
	PendingRewardCut       *big.Int
	PendingFeeShare        *big.Int
	PendingPricePerSegment *big.Int
	DelegatedStake         *big.Int
	Active                 bool
	Status                 string
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
	Address         common.Address
	BondedAmount    *big.Int
	Fees            *big.Int
	DelegateAddress common.Address
	DelegatedAmount *big.Int
	StartRound      *big.Int
	WithdrawRound   *big.Int
	LastClaimRound  *big.Int
	PendingStake    *big.Int
	PendingFees     *big.Int
	Status          string
}

func ParseDelegatorStatus(s uint8) (string, error) {
	switch s {
	case 0:
		return "Pending", nil
	case 1:
		return "Bonded", nil
	case 2:
		return "Unbonding", nil
	case 3:
		return "Unbonded", nil
	default:
		return "", ErrUnknownDelegatorStatus
	}
}

type TokenPools struct {
	RewardPool     *big.Int
	FeePool        *big.Int
	TotalStake     *big.Int
	ClaimableStake *big.Int
}

type Job struct {
	JobId              *big.Int
	StreamId           string
	Profiles           []ffmpeg.VideoProfile
	MaxPricePerSegment *big.Int
	BroadcasterAddress common.Address
	TranscoderAddress  common.Address
	CreationRound      *big.Int
	CreationBlock      *big.Int
	EndBlock           *big.Int
	Escrow             *big.Int
	TotalClaims        *big.Int
	Status             string
}

func ParseJobStatus(s uint8) (string, error) {
	switch s {
	case 0:
		return "Inactive", nil
	case 1:
		return "Active", nil
	default:
		return "", ErrUnknownJobStatus
	}
}

type Claim struct {
	ClaimId                      *big.Int
	SegmentRange                 [2]*big.Int
	ClaimRoot                    [32]byte
	ClaimBlock                   *big.Int
	EndVerificationBlock         *big.Int
	EndVerificationSlashingBlock *big.Int
	Status                       string
}

func ParseClaimStatus(s uint8) (string, error) {
	switch s {
	case 0:
		return "Pending", nil
	case 1:
		return "Slashed", nil
	case 2:
		return "Complete", nil
	default:
		return "", ErrUnknownClaimStatus
	}
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
