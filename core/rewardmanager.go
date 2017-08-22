package core

import (
	"context"
	"time"

	"github.com/livepeer/go-livepeer/eth"
)

//RewardManager manages the transcoder's reward-calling cycle.
type RewardManager struct {
	checkFreq time.Duration
	eth       eth.LivepeerEthClient
}

//NewRewardManager creates a new reward manager with a given checkFrequency.
func NewRewardManager(checkFreq time.Duration, eth eth.LivepeerEthClient) *RewardManager {
	return &RewardManager{checkFreq: checkFreq, eth: eth}
}

//Start repeatedly calls reward.
func (r *RewardManager) Start(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(r.checkFreq):
			// r.callReward()
		}
	}
}

// func (r *RewardManager) callReward() {
// 	if err := eth.CheckRoundAndInit(r.eth, EthRpcTimeout, EthMinedTxTimeout); err != nil {
// 		glog.Errorf("%v", err)
// 		return
// 	}

// 	valid, err := r.eth.ValidRewardTimeWindow()
// 	if err != nil {
// 		glog.Errorf("Error getting reward time window info: %v", err)
// 		return
// 	}

// 	if valid {
// 		glog.Infof("It's our window. Calling reward()")
// 		tx, err := r.eth.Reward()
// 		if err != nil || tx == nil {
// 			glog.Errorf("Error calling reward: %v", err)
// 			return
// 		}
// 		r, err := eth.WaitForMinedTx(r.eth.Backend(), EthRpcTimeout, EthMinedTxTimeout, tx.Hash())
// 		if err != nil {
// 			glog.Errorf("Error waiting for mined tx: %v", err)
// 		}
// 		if tx.Gas().Cmp(r.GasUsed) == 0 {
// 			glog.Errorf("Call Reward Failed")
// 			return
// 		}
// 	}
// }
