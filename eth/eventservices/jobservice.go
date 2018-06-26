package eventservices

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/golang/glog"
	lpcommon "github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/eth"
	lpTypes "github.com/livepeer/go-livepeer/eth/types"
)

var (
	ErrJobServiceStarted  = fmt.Errorf("job service already started")
	ErrJobServicedStopped = fmt.Errorf("job service already stopped")
)

type JobService struct {
	eventMonitor eth.EventMonitor
	node         *core.LivepeerNode
	sub          ethereum.Subscription
	logsCh       chan types.Log
}

func NewJobService(eventMonitor eth.EventMonitor, node *core.LivepeerNode) *JobService {
	return &JobService{
		eventMonitor: eventMonitor,
		node:         node,
	}
}

func (s *JobService) Start(ctx context.Context) error {
	if s.sub != nil {
		return ErrJobServiceStarted
	}

	logsCh := make(chan types.Log)
	sub, err := s.eventMonitor.SubscribeNewJob(ctx, "NewJob", logsCh, common.Address{}, func(l types.Log) (bool, error) {
		_, jid, _, _ := parseNewJobLog(l)

		var job *lpTypes.Job
		getJob := func() error {
			j, err := s.node.Eth.GetJob(jid)
			if err != nil || j == nil {
				glog.Errorf("Unable to get job %v, try again. Error: %v", jid, err)
				return err
			}
			if j.StreamId == "" {
				glog.Errorf("Got empty job for id:%v. Should try again.", jid.Int64())
				return errors.New("ErrGetJob")
			}
			job = j
			return err
		}
		if err := backoff.Retry(getJob, backoff.NewConstantBackOff(time.Second*2)); err != nil {
			glog.Errorf("Error getting job info: %v", err)
			return false, err
		}

		assignedAddr, err := s.node.Eth.AssignedTranscoder(job)
		if err != nil {
			glog.Errorf("Error checking for assignment: %v", err)
			return false, err
		}

		if assignedAddr == s.node.Eth.Account().Address {
			dbjob := lpcommon.NewDBJob(
				job.JobId, job.StreamId,
				job.MaxPricePerSegment, job.Profiles,
				job.BroadcasterAddress, s.node.Eth.Account().Address,
				job.CreationBlock, job.EndBlock)
			s.node.Database.InsertJob(dbjob)
			s.firstClaim(job)
		}
		return true, nil
	})

	if err != nil {
		return err
	}

	s.eventMonitor.SubscribeNewBlock(context.Background(), "BlockWatcher", make(chan *types.Header), func(h *types.Header) (bool, error) {
		s.node.Database.SetLastSeenBlock(h.Number)
		return true, nil
	})

	s.logsCh = logsCh
	s.sub = sub

	return nil
}

func (s *JobService) Stop() error {
	if s.sub == nil {
		return ErrJobServicedStopped
	}

	close(s.logsCh)
	s.sub.Unsubscribe()

	s.logsCh = nil
	s.sub = nil

	return nil
}

func (s *JobService) IsWorking() bool {
	return s.sub != nil
}

func (s *JobService) RestartTranscoder() error {

	return eth.RecoverClaims(s.node.Eth, s.node.Ipfs, s.node.Database)
}

func (s *JobService) firstClaim(job *lpTypes.Job) {
	cm, err := s.node.GetClaimManager(job)
	if err != nil {
		glog.Error("Could not get claim manager: ", err)
		return
	}
	firstClaimBlock := new(big.Int).Add(job.CreationBlock, eth.BlocksUntilFirstClaimDeadline)
	headersCh := make(chan *types.Header)
	sub, err := s.eventMonitor.SubscribeNewBlock(context.Background(), fmt.Sprintf("FirstClaimForJob%v", job.JobId), headersCh, func(h *types.Header) (bool, error) {
		if cm.DidFirstClaim() {
			// If the first claim has already been made then exit
			return false, nil
		}

		// Check if current block is job creation block + 230
		if h.Number.Cmp(firstClaimBlock) != -1 {
			glog.Infof("Making the first claim")

			canClaim, err := cm.CanClaim()
			if err != nil {
				return false, err
			}

			if canClaim {
				err := cm.ClaimVerifyAndDistributeFees()
				if err != nil {
					return false, err
				} else {
					// If this claim was successful then the first claim has been made - exit
					return false, nil
				}
			} else {
				glog.Infof("No segments to claim")
				// If there are no segments to claim at this point just stop watching
				return false, nil
			}
		} else {
			return true, nil
		}
	})
	if err == nil {
		sub.Unsubscribe()
	}
}

func parseNewJobLog(log types.Log) (broadcasterAddr common.Address, jid *big.Int, streamID string, transOptions string) {
	return common.BytesToAddress(log.Topics[1].Bytes()), new(big.Int).SetBytes(log.Data[0:32]), string(log.Data[192:338]), string(log.Data[338:])
}
