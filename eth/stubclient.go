package eth

import (
	"context"
	"errors"
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
	TOpts         [32]byte
	MaxPrice      *big.Int
	Jid           *big.Int
	SegSeqNum     *big.Int
	DHash         [32]byte
	TDHash        [32]byte
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
func (e *StubClient) RoundInfo() (*big.Int, *big.Int, *big.Int, *big.Int, error) {
	return nil, nil, nil, nil, nil
}
func (e *StubClient) InitializeRound() (*types.Transaction, error) { return nil, nil }
func (e *StubClient) CurrentRoundInitialized() (bool, error)       { return false, nil }
func (e *StubClient) Transcoder(blockRewardCut uint8, feeShare uint8, pricePerSegment *big.Int) (*types.Transaction, error) {
	return nil, nil
}
func (e *StubClient) IsActiveTranscoder() (bool, error)  { return false, nil }
func (e *StubClient) TranscoderStake() (*big.Int, error) { return nil, nil }
func (e *StubClient) Bond(amount *big.Int, toAddr common.Address) (*types.Transaction, error) {
	return nil, nil
}
func (e *StubClient) ValidRewardTimeWindow() (bool, error) { return false, nil }
func (e *StubClient) Reward() (*types.Transaction, error)  { return nil, nil }
func (e *StubClient) Job(streamId string, transcodingOptions [32]byte, maxPricePerSegment *big.Int) (*types.Transaction, error) {
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
func (e *StubClient) ClaimWork(jobId *big.Int, startSegmentSequenceNumber *big.Int, endSegmentSequenceNumber *big.Int, transcodeClaimsRoot [32]byte) (*types.Transaction, error) {
	e.ClaimCounter++
	e.ClaimJid = append(e.ClaimJid, jobId)
	e.ClaimStart = append(e.ClaimStart, startSegmentSequenceNumber)
	e.ClaimEnd = append(e.ClaimEnd, endSegmentSequenceNumber)
	e.ClaimRoot = append(e.ClaimRoot, transcodeClaimsRoot)
	return nil, errors.New("ClaimWorkError")
}
func (e *StubClient) Verify(jobId *big.Int, segmentSequenceNumber *big.Int, dataHash [32]byte, transcodedDataHash [32]byte, broadcasterSig []byte, proof []byte) (*types.Transaction, error) {
	e.Jid = jobId
	e.SegSeqNum = segmentSequenceNumber
	e.DHash = dataHash
	e.TDHash = transcodedDataHash
	e.BSig = broadcasterSig
	e.Proof = proof
	e.VerifyCounter++
	return nil, nil
}
func (e *StubClient) Transfer(toAddr common.Address, amount *big.Int) (*types.Transaction, error) {
	return nil, nil
}
func (e *StubClient) TokenBalance() (*big.Int, error)   { return big.NewInt(100000), nil }
func (e *StubClient) WaitUntilNextRound(*big.Int) error { return nil }
