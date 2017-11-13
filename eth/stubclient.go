package eth

import (
	"context"
	"math/big"
	"time"

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
	VeriRate      uint64
	DStorageHash  string
	DHash         [32]byte
	TDHash        [32]byte
	BSig          []byte
	Proof         []byte
	VerifyCounter int
	ClaimJid      []*big.Int
	ClaimStart    []*big.Int
	ClaimEnd      []*big.Int
	ClaimRoot     map[[32]byte]bool
	ClaimCounter  int
	SubLogsCh     chan types.Log
	JobsMap       map[string]*Job
}

func (e *StubClient) Backend() *ethclient.Client { return nil }
func (e *StubClient) Account() accounts.Account  { return accounts.Account{} }
func (e *StubClient) SubscribeToJobEvent(ctx context.Context, logsCh chan types.Log, broadcasterAddr, transcoderAddr common.Address) (ethereum.Subscription, error) {
	e.SubLogsCh = logsCh
	return &StubSubscription{}, nil
}
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
func (e *StubClient) Job(streamId string, transcodingOptions string, maxPricePerSegment *big.Int, endBlock *big.Int) (<-chan types.Receipt, <-chan error) {
	e.StrmID = streamId
	e.TOpts = transcodingOptions
	e.MaxPrice = maxPricePerSegment
	return nil, nil
}
func (e *StubClient) JobDetails(id *big.Int) (*big.Int, [32]byte, *big.Int, common.Address, common.Address, *big.Int, error) {
	return nil, [32]byte{}, nil, common.Address{}, common.Address{}, nil, nil
}
func (e *StubClient) SignSegmentHash(passphrase string, hash []byte) ([]byte, error) { return nil, nil }
func (e *StubClient) ClaimWork(jobId *big.Int, segmentRange [2]*big.Int, transcodeClaimsRoot [32]byte) (<-chan types.Receipt, <-chan error) {
	e.ClaimCounter++
	e.ClaimJid = append(e.ClaimJid, jobId)
	e.ClaimStart = append(e.ClaimStart, segmentRange[0])
	e.ClaimEnd = append(e.ClaimEnd, segmentRange[1])
	e.ClaimRoot[transcodeClaimsRoot] = true
	rc := make(chan types.Receipt)
	ec := make(chan error)
	go func() {
		rc <- types.Receipt{TxHash: common.StringToHash("ClaimWork")}
	}()
	return rc, ec
}
func (e *StubClient) Verify(jobId *big.Int, claimId *big.Int, segmentNumber *big.Int, dataStorageHash string, dataHashes [2][32]byte, broadcasterSig []byte, proof []byte) (<-chan types.Receipt, <-chan error) {
	e.Jid = jobId
	e.SegSeqNum = segmentNumber
	e.DStorageHash = dataStorageHash
	e.DHash = dataHashes[0]
	e.TDHash = dataHashes[1]
	e.BSig = broadcasterSig
	e.Proof = proof
	e.VerifyCounter++

	rc := make(chan types.Receipt)
	go func(rc chan types.Receipt) {
		time.Sleep(time.Millisecond * 50)
		rc <- types.Receipt{}
	}(rc)
	return rc, nil
}
func (e *StubClient) DistributeFees(jobId *big.Int, claimId *big.Int) (<-chan types.Receipt, <-chan error) {
	return nil, nil
}
func (e *StubClient) Transfer(toAddr common.Address, amount *big.Int) (<-chan types.Receipt, <-chan error) {
	return nil, nil
}
func (c *StubClient) Deposit(amount *big.Int) (<-chan types.Receipt, <-chan error) {
	return nil, nil
}
func (c *StubClient) GetBroadcasterDeposit(broadcaster common.Address) (*big.Int, error) {
	return nil, nil
}
func (e *StubClient) TokenBalance() (*big.Int, error) { return big.NewInt(100000), nil }
func (e *StubClient) WaitUntilNextRound() error       { return nil }
func (e *StubClient) GetJob(jobID *big.Int) (*Job, error) {
	return e.JobsMap[jobID.String()], nil
}
func (c *StubClient) GetClaim(jobID *big.Int, claimID *big.Int) (*Claim, error) {
	return nil, nil
}
func (c *StubClient) IsRegisteredTranscoder() (bool, error) {
	return false, nil
}
func (c *StubClient) RpcTimeout() time.Duration {
	return time.Millisecond
}
func (c *StubClient) TranscoderBond() (*big.Int, error) {
	return nil, nil
}
func (c *StubClient) WithdrawDeposit() (<-chan types.Receipt, <-chan error) {
	return nil, nil
}
func (e *StubClient) VerificationRate() (uint64, error) {
	return e.VeriRate, nil
}
func (e *StubClient) VerificationPeriod() (*big.Int, error) {
	return nil, nil
}
func (e *StubClient) SlashingPeriod() (*big.Int, error) {
	return nil, nil
}
func (e *StubClient) LastRewardRound() (*big.Int, error) {
	return nil, nil
}
func (e *StubClient) GetControllerAddr() string         { return "" }
func (e *StubClient) GetTokenAddr() string              { return "" }
func (e *StubClient) GetBondingManagerAddr() string     { return "" }
func (e *StubClient) GetJobsManagerAddr() string        { return "" }
func (e *StubClient) GetRoundsManagerAddr() string      { return "" }
func (e *StubClient) DelegatorStake() (*big.Int, error) { return nil, nil }
func (e *StubClient) DelegatorStatus() (string, error)  { return "", nil }
func (e *StubClient) GetCandidateTranscodersStats() ([]TranscoderStats, error) {
	return []TranscoderStats{}, nil
}
func (e *StubClient) GetReserveTranscodersStats() ([]TranscoderStats, error) {
	return []TranscoderStats{}, nil
}
func (e *StubClient) GetFaucetAddr() string { return "" }
func (e *StubClient) RequestTokens() (<-chan types.Receipt, <-chan error) {
	return nil, nil
}
func (e *StubClient) TranscoderPendingPricingInfo() (uint8, uint8, *big.Int, error) {
	return 0, 0, nil, nil
}
func (e *StubClient) TranscoderPricingInfo() (uint8, uint8, *big.Int, error) {
	return 0, 0, nil, nil
}
func (e *StubClient) TranscoderStatus() (string, error)                  { return "", nil }
func (e *StubClient) Unbond() (<-chan types.Receipt, <-chan error)       { return nil, nil }
func (e *StubClient) WithdrawBond() (<-chan types.Receipt, <-chan error) { return nil, nil }
func (e *StubClient) GetBlockInfoByTxHash(ctx context.Context, hash common.Hash) (blkNum *big.Int, blkHash common.Hash, err error) {
	return big.NewInt(0), hash, nil
}
