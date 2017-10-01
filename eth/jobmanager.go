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
	c                  LivepeerEthClient
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

func (j *JobManager) Start() error {
	ctx := context.Background()
	logsCh := make(chan types.Log)
	sub, err := j.c.SubscribeToJobEvent(ctx, logsCh, common.Address{}, j.c.Account().Address)
	if err != nil {
		return err
	}

	//Monitor every job event
	go func() {
		for {
			select {
			case l := <-logsCh:
				tAddr, _, jid := ParseNewJobLog(l)

				job, err := j.c.GetJob(jid)
				if err != nil {
					glog.Errorf("Error getting job info: %v", err)
				}

				strmID := job.StreamId

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

			case err := <-ctx.Done():
				glog.Errorf("Subscription to job done: %v", err)
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

func ParseNewJobLog(log types.Log) (transcoderAddr common.Address, broadcasterAddr common.Address, jid *big.Int) {
	return common.BytesToAddress(log.Topics[0].Bytes()), common.BytesToAddress(log.Topics[1].Bytes()), new(big.Int).SetBytes(log.Data[0:32])
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
