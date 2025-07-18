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
		client:           eth,
		tw:               tw,
		rewardRetryTimes: 3, // Set max retries to 3 for testing
		retryInterval:    10 * time.Millisecond,
		maxElapsedTime:   100 * time.Millisecond,
	}

	go rs.Start(ctx)
	defer rs.Stop()
	time.Sleep(1 * time.Second)
	require.True(rs.IsWorking())

	// Happy case, check that reward was called
	// Assert that no error was logged
	addr := ethcommon.Address{}
	eth.On("Account").Return(accounts.Account{Address: addr})
	eth.On("GetTranscoder", addr).Return(&lpTypes.Transcoder{
		LastRewardRound: big.NewInt(1),
		Active:          true,
	}, nil)
	eth.On("Reward").Return(&types.Transaction{}, nil).Once()
	eth.On("CheckTx").Return(nil).Once()
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

	// Test for transaction timeout error with retries
	eth.On("Reward").Return(&types.Transaction{}, errors.New("network error")).Times(2)
	eth.On("Reward").Return(&types.Transaction{}, nil).Once()
	eth.On("CheckTx").Return(context.DeadlineExceeded).Once()

	errorLogsBefore = glog.Stats.Error.Lines()
	infoLogsBefore = glog.Stats.Info.Lines()

	tw.roundSink <- types.Log{}
	time.Sleep(1 * time.Second)

	eth.AssertNumberOfCalls(t, "Reward", 4)  // 1 initial call + 2 retries + 1 final success
	eth.AssertNumberOfCalls(t, "CheckTx", 2) // 1 initial call + 1 final failure

	errorLogsAfter = glog.Stats.Error.Lines()
	infoLogsAfter = glog.Stats.Info.Lines()
	assert.Equal(int64(4), errorLogsAfter-errorLogsBefore)
	assert.Equal(int64(0), infoLogsAfter-infoLogsBefore)
}
