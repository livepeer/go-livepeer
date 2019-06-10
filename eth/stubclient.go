package eth

import (
	"math/big"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/livepeer/go-livepeer/eth/contracts"
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
	WithdrawBlock *big.Int
}, err error) {
	args := m.Called(addr)
	sender.Deposit = mockBigInt(args, 0)
	sender.WithdrawBlock = mockBigInt(args, 1)
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

func (m *MockClient) LatestBlockNum() (*big.Int, error) {
	args := m.Called()
	return mockBigInt(args, 0), args.Error(1)
}

type StubClient struct {
	SubLogsCh                    chan types.Log
	TranscoderAddress            common.Address
	BlockNum                     *big.Int
	BlockHashToReturn            common.Hash
	LatestBlockError             error
	ProcessHistoricalUnbondError error
	Orchestrators                []*lpTypes.Transcoder
}

type stubTranscoder struct {
	ServiceURI string
}

func (e *StubClient) Setup(password string, gasLimit uint64, gasPrice *big.Int) error { return nil }
func (e *StubClient) Account() accounts.Account                                       { return accounts.Account{} }
func (e *StubClient) Backend() (*ethclient.Client, error)                             { return nil, ErrMissingBackend }

// Rounds

func (e *StubClient) InitializeRound() (*types.Transaction, error)       { return nil, nil }
func (e *StubClient) CurrentRound() (*big.Int, error)                    { return big.NewInt(0), nil }
func (e *StubClient) LastInitializedRound() (*big.Int, error)            { return big.NewInt(0), nil }
func (e *StubClient) BlockHashForRound(round *big.Int) ([32]byte, error) { return [32]byte{}, nil }
func (e *StubClient) CurrentRoundInitialized() (bool, error)             { return false, nil }
func (e *StubClient) CurrentRoundLocked() (bool, error)                  { return false, nil }
func (e *StubClient) Paused() (bool, error)                              { return false, nil }

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
func (e *StubClient) GetTranscoder(addr common.Address) (*lpTypes.Transcoder, error) { return nil, nil }
func (e *StubClient) GetDelegator(addr common.Address) (*lpTypes.Delegator, error)   { return nil, nil }
func (e *StubClient) GetDelegatorUnbondingLock(addr common.Address, unbondingLockId *big.Int) (*lpTypes.UnbondingLock, error) {
	return nil, nil
}
func (e *StubClient) GetTranscoderEarningsPoolForRound(addr common.Address, round *big.Int) (*lpTypes.TokenPools, error) {
	return nil, nil
}
func (e *StubClient) RegisteredTranscoders() ([]*lpTypes.Transcoder, error) {
	return e.Orchestrators, nil
}
func (e *StubClient) IsActiveTranscoder() (bool, error) { return false, nil }
func (e *StubClient) GetTotalBonded() (*big.Int, error) { return big.NewInt(0), nil }

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
	WithdrawBlock *big.Int
}, err error) {
	return
}
func (e *StubClient) GetSenderInfo(addr ethcommon.Address) (*pm.SenderInfo, error) {
	return nil, nil
}
func (e *StubClient) UnlockPeriod() (*big.Int, error) {
	return nil, nil
}

// Parameters

func (c *StubClient) NumActiveTranscoders() (*big.Int, error) { return big.NewInt(0), nil }
func (c *StubClient) RoundLength() (*big.Int, error)          { return big.NewInt(0), nil }
func (c *StubClient) RoundLockAmount() (*big.Int, error)      { return big.NewInt(0), nil }
func (c *StubClient) UnbondingPeriod() (uint64, error)        { return 0, nil }
func (c *StubClient) Inflation() (*big.Int, error)            { return big.NewInt(0), nil }
func (c *StubClient) InflationChange() (*big.Int, error)      { return big.NewInt(0), nil }
func (c *StubClient) TargetBondingRate() (*big.Int, error)    { return big.NewInt(0), nil }

// Helpers

func (c *StubClient) ContractAddresses() map[string]common.Address { return nil }
func (c *StubClient) CheckTx(tx *types.Transaction) error {
	return nil
}
func (c *StubClient) ReplaceTransaction(tx *types.Transaction, method string, gasPrice *big.Int) (*types.Transaction, error) {
	return nil, nil
}
func (c *StubClient) Sign(msg []byte) ([]byte, error)   { return msg, nil }
func (c *StubClient) LatestBlockNum() (*big.Int, error) { return big.NewInt(0), c.LatestBlockError }
func (c *StubClient) GetGasInfo() (uint64, *big.Int)    { return 0, nil }
func (c *StubClient) SetGasInfo(uint64, *big.Int) error { return nil }
func (c *StubClient) ProcessHistoricalUnbond(*big.Int, func(*contracts.BondingManagerUnbond) error) error {
	return c.ProcessHistoricalUnbondError
}
func (c *StubClient) WatchForUnbond(chan *contracts.BondingManagerUnbond) (ethereum.Subscription, error) {
	return nil, nil
}
func (c *StubClient) ProcessHistoricalRebond(*big.Int, func(*contracts.BondingManagerRebond) error) error {
	return nil
}
func (c *StubClient) WatchForRebond(chan *contracts.BondingManagerRebond) (ethereum.Subscription, error) {
	return nil, nil
}
func (c *StubClient) ProcessHistoricalWithdrawStake(*big.Int, func(*contracts.BondingManagerWithdrawStake) error) error {
	return nil
}
func (c *StubClient) WatchForWithdrawStake(chan *contracts.BondingManagerWithdrawStake) (ethereum.Subscription, error) {
	return nil, nil
}
