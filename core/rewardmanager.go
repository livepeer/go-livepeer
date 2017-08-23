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
	if err := eth.CheckRoundAndInit(r.client); err != nil {
		glog.Errorf("%v", err)
		return
	}

	currentRound, _, _, err := r.client.RoundInfo()
	if err != nil {
		glog.Errorf("Error getting round info: %v", err)
		return
	}

	active, err := r.client.IsActiveTranscoder()
	if err != nil {
		glog.Errorf("Error checking for active transcoder: %v", err)
		return
	}

	if r.lastRewardRound.Cmp(currentRound) == -1 && active {
		resCh, errCh := r.client.Reward()
		select {
		case <-resCh:
			r.lastRewardRound = currentRound

			bond, err := r.client.TranscoderBond()
			if err != nil {
				glog.Errorf("Error getting transcoder bond: %v", err)
			} else {
				glog.Infof("Transcoder bond: %v", bond)
			}
		case err := <-errCh:
			glog.Errorf("Error calling reward: %v", err)
		}
	}
}
