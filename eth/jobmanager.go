package eth

import (
	"context"
	"errors"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/golang/glog"
)

var ErrJobManager = errors.New("ErrJobManager")

type JobManager struct {
	c LivepeerEthClient
	// jobDB              TranscodeJob
	transcoderAddrSubs map[common.Address]map[chan *big.Int]*JobSubscription //Addresses can have multiple subscriptions
	strmIDSubs         map[string]map[chan *big.Int]*JobSubscription         //streamIDs can have multiple subscriptions
}

func NewJobManager(c LivepeerEthClient) *JobManager {
	return &JobManager{
		c:                  c,
		transcoderAddrSubs: make(map[common.Address]map[chan *big.Int]*JobSubscription),
		strmIDSubs:         make(map[string]map[chan *big.Int]*JobSubscription),
	}
}

// type TranscodeJob struct {
// 	ID              *big.Int
// 	StrmID          string
// 	BroadcasterAddr common.Address
// }
func (j *JobManager) Start() error {
	ctx := context.Background()
	logsCh := make(chan types.Log)
	// jidChan := make(chan *big.Int)
	sub, err := j.c.SubscribeToJobEvent(ctx, logsCh)
	if err != nil {
		// cancel()
		return err
	}

	//Monitor every job event
	go func() {
		for {
			select {
			case l := <-logsCh:
				jid, strmID, _, tAddr, err := GetInfoFromJobEvent(l, j.c)
				if err != nil {
					glog.Errorf("Error getting info from job event: %v", err)
				}

				if subs, ok := j.transcoderAddrSubs[tAddr]; ok {
					for sub := range subs {
						sub <- jid
					}
				}
				if subs, ok := j.strmIDSubs[strmID]; ok {
					for sub := range subs {
						sub <- jid
					}
				}
			case err := <-sub.Err():
				glog.Errorf("Error from job event subscription: %v", err)
				// cancel()
				// return err
			case err := <-ctx.Done():
				glog.Errorf("Subscription to job done: %v", err)
				// cancel()
				// return ctx.Err()
			}
		}
	}()

	return nil
}

func (j *JobManager) SubscribeJobIDByStrmID(strmID string, jidChan chan *big.Int) (*JobSubscription, error) {
	if j.strmIDSubs[strmID] == nil {
		j.strmIDSubs[strmID] = make(map[chan *big.Int]*JobSubscription)
	}
	subs := j.strmIDSubs[strmID]

	if sub, ok := subs[jidChan]; !ok {
		subs[jidChan] = &JobSubscription{isStrmIDSub: true, strmID: strmID, jidChan: jidChan, j: j}
		return subs[jidChan], nil
	} else {
		glog.Infof("JobID chan already registered: %v", sub)
		return nil, ErrJobManager
	}
}

func (j *JobManager) SubscribeJobIDByTranscoderAddr(ctx context.Context, addr common.Address, callback func(jobID *big.Int)) (*JobSubscription, error) {
	return nil, nil
}

type JobSubscription struct {
	isTAddrSub  bool
	tAddr       common.Address
	isStrmIDSub bool
	strmID      string
	jidChan     chan *big.Int
	j           *JobManager
}

func (js *JobSubscription) Unsubscribe() error {
	if js.isStrmIDSub {
		subs := js.j.strmIDSubs[js.strmID]
		delete(subs, js.jidChan)
		return nil
	}

	if js.isTAddrSub {
		subs := js.j.transcoderAddrSubs[js.tAddr]
		delete(subs, js.jidChan)
		return nil
	}

	return fmt.Errorf("Subscription Not Found")
}
