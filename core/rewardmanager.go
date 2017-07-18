package core

import (
	"time"

	"github.com/golang/glog"
	"github.com/livepeer/golp/eth"
)

type RewardManager struct {
	checkFreq time.Duration
	eth       eth.LivepeerEthClient
}

func NewRewardManager(checkFreq time.Duration, eth eth.LivepeerEthClient) *RewardManager {
	return &RewardManager{checkFreq: checkFreq, eth: eth}
}

func (r *RewardManager) Start() {
	time.Sleep(r.checkFreq)
	r.callReward()
}

func (r *RewardManager) callReward() {
	if err := eth.CheckRoundAndInit(r.eth, EthRpcTimeout, EthMinedTxTimeout); err != nil {
		glog.Errorf("%v", err)
		return
	}

	valid, err := r.eth.ValidRewardTimeWindow()
	if err != nil {
		glog.Errorf("Error getting reward time window info: %v", err)
		return
	}

	if valid {
		glog.Infof("It's our window. Calling reward()")
		tx, err := r.eth.Reward()
		if err != nil || tx == nil {
			glog.Errorf("Error calling reward: %v", err)
			return
		}
		r, err := eth.WaitForMinedTx(r.eth.Backend(), EthRpcTimeout, EthMinedTxTimeout, tx.Hash())
		if err != nil {
			glog.Errorf("Error waiting for mined tx: %v", err)
		}
		if tx.Gas().Cmp(r.GasUsed) == 0 {
			glog.Errorf("Call Reward Failed")
			return
		}
	}
}
