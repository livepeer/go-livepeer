package eth

import (
	"math/big"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	lpTypes "github.com/livepeer/go-livepeer/eth/types"
)

type StubClient struct {
	StrmID            string
	TOpts             string
	MaxPrice          *big.Int
	Jid               *big.Int
	SegSeqNum         *big.Int
	VeriRate          uint64
	DStorageHash      string
	DHash             [32]byte
	TDHash            [32]byte
	BSig              []byte
	Proof             []byte
	VerifyCounter     int
	ClaimJid          []*big.Int
	ClaimStart        []*big.Int
	ClaimEnd          []*big.Int
	ClaimRoot         map[[32]byte]bool
	ClaimCounter      int
	SubLogsCh         chan types.Log
	JobsMap           map[string]*lpTypes.Job
	BlockNum          *big.Int
	BlockHashToReturn common.Hash
	Claims            map[int]*lpTypes.Claim
}

func (e *StubClient) Setup(password string, gasLimit uint64, gasPrice *big.Int) error { return nil }
func (e *StubClient) Account() accounts.Account                                       { return accounts.Account{} }
func (e *StubClient) Backend() (*ethclient.Client, error)                             { return nil, ErrMissingBackend }

// Rounds

func (e *StubClient) InitializeRound() (*types.Transaction, error) { return nil, nil }
func (e *StubClient) CurrentRound() (*big.Int, error)              { return big.NewInt(0), nil }
func (e *StubClient) LastInitializedRound() (*big.Int, error)      { return big.NewInt(0), nil }
func (e *StubClient) CurrentRoundInitialized() (bool, error)       { return false, nil }
func (e *StubClient) CurrentRoundLocked() (bool, error)            { return false, nil }
func (e *StubClient) Paused() (bool, error)                        { return false, nil }

// Token

func (e *StubClient) Transfer(toAddr common.Address, amount *big.Int) (*types.Transaction, error) {
	return nil, nil
}
func (e *StubClient) Request() (*types.Transaction, error)            { return nil, nil }
func (e *StubClient) BalanceOf(addr common.Address) (*big.Int, error) { return big.NewInt(0), nil }
func (e *StubClient) TotalSupply() (*big.Int, error)                  { return big.NewInt(0), nil }

// Service Registry

func (e *StubClient) SetServiceURI(serviceURI string) (*types.Transaction, error) { return nil, nil }
func (e *StubClient) GetServiceURI(addr common.Address) (string, error)           { return "", nil }

// Staking

func (e *StubClient) Transcoder(blockRewardCut *big.Int, feeShare *big.Int, pricePerSegment *big.Int) (*types.Transaction, error) {
	return nil, nil
}
func (e *StubClient) Reward() (*types.Transaction, error) { return nil, nil }
func (e *StubClient) Bond(amount *big.Int, toAddr common.Address) (*types.Transaction, error) {
	return nil, nil
}
func (e *StubClient) Unbond() (*types.Transaction, error)        { return nil, nil }
func (e *StubClient) WithdrawStake() (*types.Transaction, error) { return nil, nil }
func (e *StubClient) WithdrawFees() (*types.Transaction, error)  { return nil, nil }
func (e *StubClient) ClaimEarnings(endRound *big.Int) error {
	return nil
}
func (e *StubClient) GetTranscoder(addr common.Address) (*lpTypes.Transcoder, error) { return nil, nil }
func (e *StubClient) GetDelegator(addr common.Address) (*lpTypes.Delegator, error)   { return nil, nil }
func (e *StubClient) GetTranscoderEarningsPoolForRound(addr common.Address, round *big.Int) (*lpTypes.TokenPools, error) {
	return nil, nil
}
func (e *StubClient) RegisteredTranscoders() ([]*lpTypes.Transcoder, error) { return nil, nil }
func (e *StubClient) IsActiveTranscoder() (bool, error)                     { return false, nil }
func (e *StubClient) AssignedTranscoder(jobID *big.Int) (common.Address, error) {
	return common.Address{}, nil
}
func (e *StubClient) GetTotalBonded() (*big.Int, error) { return big.NewInt(0), nil }

// Jobs

func (e *StubClient) Job(streamId string, transcodingOptions string, maxPricePerSegment *big.Int, endBlock *big.Int) (*types.Transaction, error) {
	return nil, nil
}
func (e *StubClient) ClaimWork(jobId *big.Int, segmentRange [2]*big.Int, claimRoot [32]byte) (*types.Transaction, error) {
	e.ClaimCounter++
	e.ClaimJid = append(e.ClaimJid, jobId)
	e.ClaimStart = append(e.ClaimStart, segmentRange[0])
	e.ClaimEnd = append(e.ClaimEnd, segmentRange[1])
	e.ClaimRoot[claimRoot] = true
	e.Claims[e.ClaimCounter-1] = &lpTypes.Claim{
		SegmentRange: segmentRange,
		ClaimRoot:    claimRoot,
	}
	return nil, nil
}
func (e *StubClient) Verify(jobId *big.Int, claimId *big.Int, segmentNumber *big.Int, dataStorageHash string, dataHashes [2][32]byte, broadcasterSig []byte, proof []byte) (*types.Transaction, error) {
	e.Jid = jobId
	e.SegSeqNum = segmentNumber
	e.DStorageHash = dataStorageHash
	e.DHash = dataHashes[0]
	e.TDHash = dataHashes[1]
	e.BSig = broadcasterSig
	e.Proof = proof
	e.VerifyCounter++

	return nil, nil
}
func (e *StubClient) DistributeFees(jobId *big.Int, claimId *big.Int) (*types.Transaction, error) {
	return nil, nil
}
func (c *StubClient) Deposit(amount *big.Int) (*types.Transaction, error) {
	return nil, nil
}
func (c *StubClient) Withdraw() (*types.Transaction, error) { return nil, nil }
func (c *StubClient) BroadcasterDeposit(broadcaster common.Address) (*big.Int, error) {
	return big.NewInt(0), nil
}
func (e *StubClient) GetJob(jobID *big.Int) (*lpTypes.Job, error) {
	return nil, nil
}
func (c *StubClient) GetClaim(jobID *big.Int, claimID *big.Int) (*lpTypes.Claim, error) {
	return c.Claims[int(claimID.Int64())], nil
}
func (c *StubClient) NumJobs() (*big.Int, error) { return big.NewInt(0), nil }

// Parameters

func (c *StubClient) NumActiveTranscoders() (*big.Int, error)          { return big.NewInt(0), nil }
func (c *StubClient) RoundLength() (*big.Int, error)                   { return big.NewInt(0), nil }
func (c *StubClient) RoundLockAmount() (*big.Int, error)               { return big.NewInt(0), nil }
func (c *StubClient) UnbondingPeriod() (uint64, error)                 { return 0, nil }
func (c *StubClient) VerificationRate() (uint64, error)                { return 0, nil }
func (c *StubClient) VerificationPeriod() (*big.Int, error)            { return big.NewInt(0), nil }
func (c *StubClient) VerificationSlashingPeriod() (*big.Int, error)    { return big.NewInt(0), nil }
func (c *StubClient) FailedVerificationSlashAmount() (*big.Int, error) { return big.NewInt(0), nil }
func (c *StubClient) MissedVerificationSlashAmount() (*big.Int, error) { return big.NewInt(0), nil }
func (c *StubClient) DoubleClaimSegmentSlashAmount() (*big.Int, error) { return big.NewInt(0), nil }
func (c *StubClient) FinderFee() (*big.Int, error)                     { return big.NewInt(0), nil }
func (c *StubClient) Inflation() (*big.Int, error)                     { return big.NewInt(0), nil }
func (c *StubClient) InflationChange() (*big.Int, error)               { return big.NewInt(0), nil }
func (c *StubClient) TargetBondingRate() (*big.Int, error)             { return big.NewInt(0), nil }
func (c *StubClient) VerificationCodeHash() (string, error)            { return "", nil }

// Helpers

func (c *StubClient) ContractAddresses() map[string]common.Address { return nil }
func (c *StubClient) CheckTx(tx *types.Transaction) error          { return nil }
func (c *StubClient) ReplaceTransaction(tx *types.Transaction, method string, gasPrice *big.Int) (*types.Transaction, error) {
	return nil, nil
}
func (c *StubClient) Sign(msg []byte) ([]byte, error)   { return nil, nil }
func (c *StubClient) LatestBlockNum() (*big.Int, error) { return big.NewInt(0), nil }
func (c *StubClient) GetGasInfo() (uint64, *big.Int)    { return 0, nil }
func (c *StubClient) SetGasInfo(uint64, *big.Int) error { return nil }
