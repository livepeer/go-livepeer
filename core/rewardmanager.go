package core

import (
	"context"
	"math/big"
	"time"

	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/eth"
)

//RewardManager manages the transcoder's reward-calling cycle.
type RewardManager struct {
	checkFreq       time.Duration
	lastRewardRound *big.Int
	client          eth.LivepeerEthClient
}

//NewRewardManager creates a new reward manager with a given checkFrequency.
func NewRewardManager(checkFreq time.Duration, client eth.LivepeerEthClient) *RewardManager {
	return &RewardManager{checkFreq: checkFreq, client: client}
}

//Start repeatedly calls reward.
func (r *RewardManager) Start(ctx context.Context) {
	lastRewardRound, err := r.client.LastRewardRound()
	if err != nil {
		glog.Errorf("Error getting last reward round: %v", err)
	}

	r.lastRewardRound = lastRewardRound

	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(r.checkFreq):
			r.callReward()
		}
	}
}

func (r *RewardManager) callReward() {
	currentRound, _, _, err := r.client.RoundInfo()
	if err != nil {
		glog.Errorf("Error getting round info: %v", err)
		return
	}

	if r.lastRewardRound.Cmp(currentRound) == -1 {
		if err := eth.CheckRoundAndInit(r.client); err != nil {
			glog.Errorf("%v", err)
			return
		}

		resCh, errCh := r.client.Reward()
		select {
		case <-resCh:
			r.lastRewardRound = currentRound
		case err := <-errCh:
			glog.Errorf("Error calling reward: %v", err)
		}
	}
}
