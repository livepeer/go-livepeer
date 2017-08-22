package eth

import (
	"context"
	"math/big"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

type StubSubscription struct{}

func (s *StubSubscription) Unsubscribe()      {}
func (s *StubSubscription) Err() <-chan error { return make(chan error) }

type StubClient struct {
	StrmID        string
	TOpts         string
	MaxPrice      *big.Int
	Jid           *big.Int
	SegSeqNum     *big.Int
	DHash         string
	TDHash        string
	BSig          []byte
	Proof         []byte
	VerifyCounter int
	ClaimJid      []*big.Int
	ClaimStart    []*big.Int
	ClaimEnd      []*big.Int
	ClaimRoot     [][32]byte
	ClaimCounter  int
	SubLogsCh     chan types.Log
}

func (e *StubClient) Backend() *ethclient.Client { return nil }
func (e *StubClient) Account() accounts.Account  { return accounts.Account{} }
func (e *StubClient) SubscribeToJobEvent(ctx context.Context, logsCh chan types.Log) (ethereum.Subscription, error) {
	e.SubLogsCh = logsCh

	return &StubSubscription{}, nil
}

// func (e *StubClient) SubscribeToJobEvent(callback func(types.Log) error) (ethereum.Subscription, chan types.Log, error) {
// 	return nil, nil, nil
// }
func (e *StubClient) WatchEvent(logsCh <-chan types.Log) (types.Log, error) { return types.Log{}, nil }
func (e *StubClient) RoundInfo() (*big.Int, *big.Int, *big.Int, error) {
	return nil, nil, nil, nil
}
func (e *StubClient) InitializeRound() (<-chan types.Receipt, <-chan error) { return nil, nil }
func (e *StubClient) CurrentRoundInitialized() (bool, error)                { return false, nil }
func (e *StubClient) Transcoder(blockRewardCut uint8, feeShare uint8, pricePerSegment *big.Int) (<-chan types.Receipt, <-chan error) {
	return nil, nil
}
func (e *StubClient) IsActiveTranscoder() (bool, error)  { return false, nil }
func (e *StubClient) TranscoderStake() (*big.Int, error) { return nil, nil }
func (e *StubClient) Bond(amount *big.Int, toAddr common.Address) (<-chan types.Receipt, <-chan error) {
	return nil, nil
}
func (e *StubClient) ValidRewardTimeWindow() (bool, error)         { return false, nil }
func (e *StubClient) Reward() (<-chan types.Receipt, <-chan error) { return nil, nil }
func (e *StubClient) Job(streamId string, transcodingOptions string, maxPricePerSegment *big.Int) (<-chan types.Receipt, <-chan error) {
	e.StrmID = streamId
	e.TOpts = transcodingOptions
	e.MaxPrice = maxPricePerSegment
	return nil, nil
}
func (e *StubClient) JobDetails(id *big.Int) (*big.Int, [32]byte, *big.Int, common.Address, common.Address, *big.Int, error) {
	return nil, [32]byte{}, nil, common.Address{}, common.Address{}, nil, nil
}
func (e *StubClient) EndJob(id *big.Int) (*types.Transaction, error)                 { return nil, nil }
func (e *StubClient) SignSegmentHash(passphrase string, hash []byte) ([]byte, error) { return nil, nil }
func (e *StubClient) ClaimWork(jobId *big.Int, segmentRange [2]*big.Int, transcodeClaimsRoot [32]byte) (<-chan types.Receipt, <-chan error) {
	e.ClaimCounter++
	e.ClaimJid = append(e.ClaimJid, jobId)
	e.ClaimStart = append(e.ClaimStart, segmentRange[0])
	e.ClaimEnd = append(e.ClaimEnd, segmentRange[1])
	e.ClaimRoot = append(e.ClaimRoot, transcodeClaimsRoot)
	return nil, nil
}
func (e *StubClient) Verify(jobId *big.Int, claimId *big.Int, segmentNumber *big.Int, dataHash string, transcodedDataHash string, broadcasterSig []byte, proof []byte) (<-chan types.Receipt, <-chan error) {
	e.Jid = jobId
	e.SegSeqNum = segmentNumber
	e.DHash = dataHash
	e.TDHash = transcodedDataHash
	e.BSig = broadcasterSig
	e.Proof = proof
	e.VerifyCounter++
	return nil, nil
}
func (e *StubClient) DistributeFees(jobId *big.Int, claimId *big.Int) (<-chan types.Receipt, <-chan error) {
	return nil, nil
}
func (e *StubClient) Transfer(toAddr common.Address, amount *big.Int) (<-chan types.Receipt, <-chan error) {
	return nil, nil
}
func (e *StubClient) TokenBalance() (*big.Int, error) { return big.NewInt(100000), nil }
func (e *StubClient) WaitUntilNextRound() error       { return nil }
func (e *StubClient) GetJob(jobID *big.Int) (*Job, error) {
	return nil, nil
}
func (e *StubClient) GetClaim(jobID *big.Int) (*Claim, error) {
	return nil, nil
}
func (e *StubClient) VerificationRate() (uint64, error) {
	return 0, nil
}
func (e *StubClient) VerificationPeriod() (*big.Int, error) {
	return nil, nil
}
func (e *StubClient) SlashingPeriod() (*big.Int, error) {
	return nil, nil
}
