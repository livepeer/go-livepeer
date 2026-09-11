package eth

import (
	"context"
	"errors"
	"math/big"
	"testing"
	"time"

	ethcommon "github.com/ethereum/go-ethereum/common"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/golang/glog"
	lpTypes "github.com/livepeer/go-livepeer/eth/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRewardService_Start(t *testing.T) {
	assert := assert.New(t)
	rs := RewardService{
		working: true,
	}
	assert.EqualError(rs.Start(context.Background()), ErrRewardServiceStarted.Error())

	ctx, cancel := context.WithCancel(context.Background())
	rs = RewardService{
		tw:           &stubTimeWatcher{},
		cancelWorker: cancel,
	}
	errC := make(chan error)
	go func() { errC <- rs.Start(ctx) }()
	time.Sleep(1 * time.Second)
	assert.True(rs.working)
	cancel()
	err := <-errC
	assert.Nil(err)
}

func TestRewardService_Stop(t *testing.T) {
	assert := assert.New(t)
	rs := RewardService{
		working: false,
	}
	assert.EqualError(rs.Stop(), ErrRewardServiceStopped.Error())

	ctx, cancel := context.WithCancel(context.Background())
	rs = RewardService{
		tw:           &stubTimeWatcher{},
		cancelWorker: cancel,
	}
	go rs.Start(ctx)
	time.Sleep(1 * time.Second)
	require.True(t, rs.working)
	rs.Stop()
	assert.False(rs.working)
}

func TestRewardService_IsWorking(t *testing.T) {
	assert := assert.New(t)
	rs := RewardService{
		working: false,
	}
	assert.False(rs.IsWorking())
	rs.working = true
	assert.True(rs.IsWorking())
}

func TestRewardService_ReceiveRoundEvent_TryReward(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	eth := &MockClient{}
	tw := &stubTimeWatcher{
		lastInitializedRound: big.NewInt(100),
	}
	ctx := context.Background()
	rs := RewardService{
		client: eth,
		tw:     tw,
	}

	go rs.Start(ctx)
	defer rs.Stop()
	time.Sleep(1 * time.Second)
	require.True(rs.IsWorking())

	// Happy case , check that reward was called
	// Assert that no error was logged
	addr := ethcommon.Address{}
	eth.On("Account").Return(accounts.Account{Address: addr})
	eth.On("GetTranscoder", addr).Return(&lpTypes.Transcoder{
		LastRewardRound: big.NewInt(1),
		Active:          true,
	}, nil)
	eth.On("Reward").Return(&types.Transaction{}, nil).Times(1)
	eth.On("CheckTx").Return(nil).Times(1)
	eth.On("GetTranscoderEarningsPoolForRound").Return(&lpTypes.TokenPools{}, nil)

	errorLogsBefore := glog.Stats.Error.Lines()
	infoLogsBefore := glog.Stats.Info.Lines()

	tw.roundSink <- types.Log{}
	time.Sleep(1 * time.Second)

	eth.AssertNumberOfCalls(t, "Reward", 1)
	eth.AssertNumberOfCalls(t, "CheckTx", 1)

	errorLogsAfter := glog.Stats.Error.Lines()
	infoLogsAfter := glog.Stats.Info.Lines()
	assert.Equal(int64(0), errorLogsAfter-errorLogsBefore)
	assert.Equal(int64(1), infoLogsAfter-infoLogsBefore)

	// Test for transaction time out error
	eth.On("Reward").Return(&types.Transaction{}, nil).Once()
	eth.On("CheckTx").Return(context.DeadlineExceeded).Once()

	errorLogsBefore = glog.Stats.Error.Lines()
	infoLogsBefore = glog.Stats.Info.Lines()

	tw.roundSink <- types.Log{}
	time.Sleep(1 * time.Second)

	eth.AssertNumberOfCalls(t, "Reward", 2)
	eth.AssertNumberOfCalls(t, "CheckTx", 2)

	errorLogsAfter = glog.Stats.Error.Lines()
	infoLogsAfter = glog.Stats.Info.Lines()
	assert.Equal(int64(1), errorLogsAfter-errorLogsBefore)
	assert.Equal(int64(0), infoLogsAfter-infoLogsBefore)
}

func TestRewardService_TryReward_RewardCaller(t *testing.T) {
	var (
		account = ethcommon.HexToAddress("0x1111111111111111111111111111111111111111")
		orch    = ethcommon.HexToAddress("0x2222222222222222222222222222222222222222")
		other   = ethcommon.HexToAddress("0x3333333333333333333333333333333333333333")

		eligible   = &lpTypes.Transcoder{LastRewardRound: big.NewInt(1), Active: true}
		rewarded   = &lpTypes.Transcoder{LastRewardRound: big.NewInt(100), Active: true}
		inactive   = &lpTypes.Transcoder{LastRewardRound: big.NewInt(1), Active: false}
		currentRnd = big.NewInt(100)
	)

	tests := []struct {
		name          string
		orchAddr      ethcommon.Address
		accountRecord *lpTypes.Transcoder
		orchRecord    *lpTypes.Transcoder
		// on-chain reward caller for orch, or the error looking it up
		rewardCaller    ethcommon.Address
		rewardCallerErr error

		wantReward              bool
		wantRewardForTranscoder bool
		wantErr                 string
	}{
		{
			name:          "not delegated, zero orchAddr, calls reward directly",
			orchAddr:      ethcommon.Address{},
			accountRecord: eligible,
			wantReward:    true,
		},
		{
			name:          "not delegated, orchAddr equals account, calls reward directly",
			orchAddr:      account,
			accountRecord: eligible,
			wantReward:    true,
		},
		{
			name:                    "delegated and authorized, calls rewardForTranscoder",
			orchAddr:                orch,
			orchRecord:              eligible,
			rewardCaller:            account,
			wantRewardForTranscoder: true,
		},
		{
			name:         "delegated but not authorized, errors without sending a tx",
			orchAddr:     orch,
			orchRecord:   eligible,
			rewardCaller: other,
			wantErr:      "is not the reward caller",
		},
		{
			name:         "delegated and revoked, errors without sending a tx",
			orchAddr:     orch,
			orchRecord:   eligible,
			rewardCaller: ethcommon.Address{},
			wantErr:      "is not the reward caller",
		},
		{
			name:            "delegated, reward caller lookup fails, errors without sending a tx",
			orchAddr:        orch,
			orchRecord:      eligible,
			rewardCallerErr: errors.New("rpc boom"),
			wantErr:         "could not look up reward caller",
		},
		{
			// Reading the caller's record here would reward twice a round.
			name:          "delegated, eligibility read from orchestrator not caller",
			orchAddr:      orch,
			accountRecord: eligible,
			orchRecord:    rewarded,
			rewardCaller:  account,
		},
		{
			name:          "delegated, orchestrator inactive",
			orchAddr:      orch,
			accountRecord: eligible,
			orchRecord:    inactive,
			rewardCaller:  account,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert := assert.New(t)
			client := &MockClient{}
			client.On("Account").Return(accounts.Account{Address: account})
			rs := NewRewardService(client, &stubTimeWatcher{lastInitializedRound: currentRnd}, tt.orchAddr)

			if tt.accountRecord != nil {
				client.On("GetTranscoder", account).Return(tt.accountRecord, nil)
			}
			if tt.orchRecord != nil {
				client.On("GetTranscoder", orch).Return(tt.orchRecord, nil)
			}
			client.On("GetRewardCaller", orch).Return(tt.rewardCaller, tt.rewardCallerErr)
			client.On("Reward").Return(&types.Transaction{}, nil)
			client.On("RewardForTranscoder", orch).Return(&types.Transaction{}, nil)
			client.On("CheckTx").Return(nil)
			client.On("GetTranscoderEarningsPoolForRound").Return(&lpTypes.TokenPools{}, nil)

			err := rs.tryReward()

			if tt.wantErr != "" {
				assert.ErrorContains(err, tt.wantErr)
			} else {
				assert.NoError(err)
			}

			if tt.wantReward {
				client.AssertNumberOfCalls(t, "Reward", 1)
			} else {
				client.AssertNumberOfCalls(t, "Reward", 0)
			}
			if tt.wantRewardForTranscoder {
				client.AssertCalled(t, "RewardForTranscoder", orch)
			} else {
				client.AssertNumberOfCalls(t, "RewardForTranscoder", 0)
			}
		})
	}
}
