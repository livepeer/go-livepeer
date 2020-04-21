package eth

import (
	"math/big"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	lpTypes "github.com/livepeer/go-livepeer/eth/types"
	"github.com/livepeer/go-livepeer/pm"
	"github.com/stretchr/testify/mock"
)

func mockTransaction(args mock.Arguments, idx int) *types.Transaction {
	arg := args.Get(idx)

	if arg == nil {
		return nil
	}

	return arg.(*types.Transaction)
}

func mockBigInt(args mock.Arguments, idx int) *big.Int {
	arg := args.Get(idx)

	if arg == nil {
		return nil
	}

	return arg.(*big.Int)
}

type MockClient struct {
	mock.Mock

	// Embed StubClient to call its methods with MockClient
	// as the receiver so that MockClient implements the LivepeerETHClient
	// interface
	*StubClient
}

// BondingManager

// TranscoderPool returns a list of registered transcoders
func (m *MockClient) TranscoderPool() ([]*lpTypes.Transcoder, error) {
	args := m.Called()

	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).([]*lpTypes.Transcoder), args.Error(1)
}

// GetTranscoderPoolMaxSize returns the max size of the active set
func (m *MockClient) GetTranscoderPoolMaxSize() (*big.Int, error) {
	args := m.Called()
	return mockBigInt(args, 0), args.Error(1)
}

// RoundsManager

// InitializeRound submits a round initialization transaction
func (m *MockClient) InitializeRound() (*types.Transaction, error) {
	args := m.Called()
	return mockTransaction(args, 0), args.Error(1)
}

// CurrentRound returns the current round number
func (m *MockClient) CurrentRound() (*big.Int, error) {
	args := m.Called()
	return mockBigInt(args, 0), args.Error(1)
}

// CurrentRoundInitialized returns whether the current round is initialized
func (m *MockClient) CurrentRoundInitialized() (bool, error) {
	args := m.Called()
	return args.Bool(0), args.Error(1)
}

// CurrentRoundStartBlock returns the block number that the current round started in
func (m *MockClient) CurrentRoundStartBlock() (*big.Int, error) {
	args := m.Called()
	return mockBigInt(args, 0), args.Error(1)
}

// TicketBroker

func (m *MockClient) FundDepositAndReserve(depositAmount, reserveAmount *big.Int) (*types.Transaction, error) {
	args := m.Called(depositAmount, reserveAmount)
	return mockTransaction(args, 0), args.Error(1)
}

func (m *MockClient) FundDeposit(amount *big.Int) (*types.Transaction, error) {
	args := m.Called(amount)
	return mockTransaction(args, 0), args.Error(1)
}

func (m *MockClient) Unlock() (*types.Transaction, error) {
	args := m.Called()
	return mockTransaction(args, 0), args.Error(1)
}

func (m *MockClient) CancelUnlock() (*types.Transaction, error) {
	args := m.Called()
	return mockTransaction(args, 0), args.Error(1)
}

func (m *MockClient) Withdraw() (*types.Transaction, error) {
	args := m.Called()
	return mockTransaction(args, 0), args.Error(1)
}

func (m *MockClient) Senders(addr common.Address) (sender struct {
	Deposit       *big.Int
	WithdrawRound *big.Int
}, err error) {
	args := m.Called(addr)
	sender.Deposit = mockBigInt(args, 0)
	sender.WithdrawRound = mockBigInt(args, 1)
	err = args.Error(2)

	return
}

func (m *MockClient) GetSenderInfo(addr common.Address) (*pm.SenderInfo, error) {
	args := m.Called(addr)
	infoArg := args.Get(0)
	err := args.Error(1)

	if infoArg == nil {
		return nil, err
	}

	return infoArg.(*pm.SenderInfo), err
}

func (m *MockClient) UnlockPeriod() (*big.Int, error) {
	args := m.Called()
	return mockBigInt(args, 0), args.Error(1)
}

func (m *MockClient) Account() accounts.Account {
	args := m.Called()

	arg0 := args.Get(0)
	if arg0 == nil {
		return accounts.Account{}
	}

	return arg0.(accounts.Account)
}

func (m *MockClient) CheckTx(tx *types.Transaction) error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockClient) Vote(pollAddr ethcommon.Address, choiceID *big.Int) (*types.Transaction, error) {
	args := m.Called()
	return mockTransaction(args, 0), args.Error(1)
}

type StubClient struct {
	SubLogsCh                    chan types.Log
	TranscoderAddress            common.Address
	BlockNum                     *big.Int
	BlockHashToReturn            common.Hash
	ProcessHistoricalUnbondError error
	Orchestrators                []*lpTypes.Transcoder
	Round                        *big.Int
	RoundsErr                    error
	SenderInfo                   *pm.SenderInfo
	PoolSize                     *big.Int
	ClaimedAmount                *big.Int
	ClaimedReserveError          error
	Orch                         *lpTypes.Transcoder
	Err                          error
	CheckTxErr                   error
	TotalStake                   *big.Int
	TranscoderPoolError          error
}

type stubTranscoder struct {
	ServiceURI string
}

func (e *StubClient) Setup(password string, gasLimit uint64, gasPrice *big.Int) error { return nil }
func (e *StubClient) Account() accounts.Account                                       { return accounts.Account{Address: e.TranscoderAddress} }
func (e *StubClient) Backend() (Backend, error)                                       { return nil, nil }

// Rounds

func (e *StubClient) InitializeRound() (*types.Transaction, error) { return nil, nil }
func (e *StubClient) CurrentRound() (*big.Int, error)              { return big.NewInt(0), e.RoundsErr }
func (e *StubClient) LastInitializedRound() (*big.Int, error)      { return e.Round, e.RoundsErr }
func (e *StubClient) BlockHashForRound(round *big.Int) ([32]byte, error) {
	return e.BlockHashToReturn, nil
}
func (e *StubClient) CurrentRoundInitialized() (bool, error)    { return false, nil }
func (e *StubClient) CurrentRoundLocked() (bool, error)         { return false, nil }
func (e *StubClient) CurrentRoundStartBlock() (*big.Int, error) { return nil, nil }
func (e *StubClient) Paused() (bool, error)                     { return false, nil }

// Token

func (e *StubClient) Transfer(toAddr common.Address, amount *big.Int) (*types.Transaction, error) {
	return nil, nil
}
func (e *StubClient) Request() (*types.Transaction, error)            { return nil, nil }
func (e *StubClient) BalanceOf(addr common.Address) (*big.Int, error) { return big.NewInt(0), nil }
func (e *StubClient) TotalSupply() (*big.Int, error)                  { return big.NewInt(0), nil }

// Service Registry

func (e *StubClient) SetServiceURI(serviceURI string) (*types.Transaction, error) { return nil, nil }
func (e *StubClient) GetServiceURI(addr common.Address) (string, error) {
	if e.Err != nil {
		return "", e.Err
	}
	return e.Orch.ServiceURI, nil
}

// Staking

func (e *StubClient) Transcoder(blockRewardCut, feeShare *big.Int) (*types.Transaction, error) {
	return nil, nil
}
func (e *StubClient) Reward() (*types.Transaction, error) { return nil, nil }
func (e *StubClient) Bond(amount *big.Int, toAddr common.Address) (*types.Transaction, error) {
	return nil, nil
}
func (e *StubClient) Rebond(*big.Int) (*types.Transaction, error) { return nil, nil }
func (e *StubClient) RebondFromUnbonded(common.Address, *big.Int) (*types.Transaction, error) {
	return nil, nil
}
func (e *StubClient) Unbond(*big.Int) (*types.Transaction, error) { return nil, nil }
func (e *StubClient) WithdrawStake(*big.Int) (*types.Transaction, error) {
	return nil, nil
}
func (e *StubClient) WithdrawFees() (*types.Transaction, error) { return nil, nil }
func (e *StubClient) ClaimEarnings(endRound *big.Int) error {
	return nil
}
func (e *StubClient) GetTranscoder(addr common.Address) (*lpTypes.Transcoder, error) {
	if e.Err != nil {
		return nil, e.Err
	}
	return e.Orch, nil
}
func (e *StubClient) GetDelegator(addr common.Address) (*lpTypes.Delegator, error) { return nil, nil }
func (e *StubClient) GetDelegatorUnbondingLock(addr common.Address, unbondingLockId *big.Int) (*lpTypes.UnbondingLock, error) {
	return nil, nil
}
func (e *StubClient) GetTranscoderEarningsPoolForRound(addr common.Address, round *big.Int) (*lpTypes.TokenPools, error) {
	if e.TranscoderPoolError != nil {
		return &lpTypes.TokenPools{}, e.TranscoderPoolError
	}

	totalStake := big.NewInt(0)
	if e.TotalStake != nil {
		totalStake = e.TotalStake
	}
	return &lpTypes.TokenPools{TotalStake: totalStake}, nil
}
func (e *StubClient) TranscoderPool() ([]*lpTypes.Transcoder, error) {
	return e.Orchestrators, e.TranscoderPoolError
}
func (e *StubClient) IsActiveTranscoder() (bool, error)        { return false, nil }
func (e *StubClient) GetTotalBonded() (*big.Int, error)        { return big.NewInt(0), nil }
func (e *StubClient) GetTranscoderPoolSize() (*big.Int, error) { return e.PoolSize, nil }
func (e *StubClient) ClaimedReserve(sender ethcommon.Address, claimant ethcommon.Address) (*big.Int, error) {
	return e.ClaimedAmount, e.ClaimedReserveError
}

// TicketBroker
func (e *StubClient) FundDepositAndReserve(depositAmount, reserveAmount *big.Int) (*types.Transaction, error) {
	return nil, nil

}
func (e *StubClient) FundDeposit(amount *big.Int) (*types.Transaction, error) {
	return nil, nil
}
func (e *StubClient) FundReserve(amount *big.Int) (*types.Transaction, error) {
	return nil, nil
}
func (e *StubClient) Unlock() (*types.Transaction, error) {
	return nil, nil
}
func (e *StubClient) CancelUnlock() (*types.Transaction, error) {
	return nil, nil
}
func (e *StubClient) Withdraw() (*types.Transaction, error) {
	return nil, nil
}
func (e *StubClient) RedeemWinningTicket(ticket *pm.Ticket, sig []byte, recipientRand *big.Int) (*types.Transaction, error) {
	return nil, nil
}
func (e *StubClient) IsUsedTicket(ticket *pm.Ticket) (bool, error) {
	return true, nil
}
func (e *StubClient) Senders(addr ethcommon.Address) (sender struct {
	Deposit       *big.Int
	WithdrawRound *big.Int
}, err error) {
	return
}
func (e *StubClient) GetSenderInfo(addr ethcommon.Address) (*pm.SenderInfo, error) {
	return e.SenderInfo, nil
}
func (e *StubClient) ClaimableReserve(reserveHolder, claimant ethcommon.Address) (*big.Int, error) {
	return nil, nil
}
func (e *StubClient) UnlockPeriod() (*big.Int, error) {
	return nil, nil
}

// Parameters
func (c *StubClient) GetTranscoderPoolMaxSize() (*big.Int, error) { return big.NewInt(0), nil }
func (c *StubClient) RoundLength() (*big.Int, error)              { return big.NewInt(0), nil }
func (c *StubClient) RoundLockAmount() (*big.Int, error)          { return big.NewInt(0), nil }
func (c *StubClient) UnbondingPeriod() (uint64, error)            { return 0, nil }
func (c *StubClient) Inflation() (*big.Int, error)                { return big.NewInt(0), nil }
func (c *StubClient) InflationChange() (*big.Int, error)          { return big.NewInt(0), nil }
func (c *StubClient) TargetBondingRate() (*big.Int, error)        { return big.NewInt(0), nil }

// Helpers

func (c *StubClient) ContractAddresses() map[string]common.Address { return nil }
func (c *StubClient) CheckTx(tx *types.Transaction) error {
	return c.CheckTxErr
}
func (c *StubClient) ReplaceTransaction(tx *types.Transaction, method string, gasPrice *big.Int) (*types.Transaction, error) {
	return nil, nil
}
func (c *StubClient) Sign(msg []byte) ([]byte, error)   { return msg, c.Err }
func (c *StubClient) GetGasInfo() (uint64, *big.Int)    { return 0, nil }
func (c *StubClient) SetGasInfo(uint64, *big.Int) error { return nil }

// Faucet
func (c *StubClient) NextValidRequest(common.Address) (*big.Int, error) { return nil, nil }

// Governance
func (c *StubClient) Vote(pollAddr ethcommon.Address, choiceID *big.Int) (*types.Transaction, error) {
	return nil, c.Err
}
