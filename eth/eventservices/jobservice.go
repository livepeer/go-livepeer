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
	"github.com/livepeer/go-livepeer/net"
	ffmpeg "github.com/livepeer/lpms/ffmpeg"
	"github.com/livepeer/lpms/transcoder"
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

func (s *JobService) doTranscode(job *lpTypes.Job) (bool, error) {
	//Check if broadcaster has enough funds
	bDeposit, err := s.node.Eth.BroadcasterDeposit(job.BroadcasterAddress)
	if err != nil {
		glog.Errorf("Error getting broadcaster deposit: %v", err)
		return false, err
	}

	if bDeposit.Cmp(job.MaxPricePerSegment) == -1 {
		glog.Infof("Broadcaster does not have enough funds. Skipping job")
		s.node.Database.SetStopReason(job.JobId, "Insufficient deposit")
		return true, nil
	}

	//Create transcode config, make sure the profiles are sorted
	config := net.TranscodeConfig{StrmID: job.StreamId, Profiles: job.Profiles, JobID: job.JobId, PerformOnchainClaim: true}
	glog.Infof("Transcoder got job %v - strmID: %v, tData: %v, config: %v", job.JobId, job.StreamId, job.Profiles, config)

	//Do The Transcoding
	cm, err := s.node.GetClaimManager(job)
	if err != nil {
		glog.Error("Could not get claim manager: ", err)
		return false, err
	}
	tr := transcoder.NewFFMpegSegmentTranscoder(job.Profiles, s.node.WorkDir)
	strmIDs, err := s.node.TranscodeAndBroadcast(config, cm, tr)
	if err != nil {
		reason := fmt.Sprintf("Transcode error: %v", err)
		glog.Errorf(reason)
		s.node.Database.SetStopReason(job.JobId, reason)
		return false, err
	}

	//Notify Broadcaster
	sid := core.StreamID(job.StreamId)
	vids := make(map[core.StreamID]ffmpeg.VideoProfile)
	for i, vp := range job.Profiles {
		vids[strmIDs[i]] = vp
	}
	if err = s.node.NotifyBroadcaster(sid.GetNodeID(), sid, vids); err != nil {
		glog.Errorf("Notify Broadcaster Error: %v", err)
		return true, nil
	}

	firstClaimBlock := new(big.Int).Add(job.CreationBlock, eth.BlocksUntilFirstClaimDeadline)
	headersCh := make(chan *types.Header)
	s.eventMonitor.SubscribeNewBlock(context.Background(), fmt.Sprintf("FirstClaimForJob%v", job.JobId), headersCh, func(h *types.Header) (bool, error) {
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

	return true, nil
}

func (s *JobService) RestartTranscoder() error {

	return eth.RecoverClaims(s.node.Eth, s.node.Ipfs, s.node.Database)
	// Intentionally skip all the below. Remove after some grace period.

	blknum, err := s.node.Eth.LatestBlockNum()
	if err != nil {
		return err
	}
	// fetch active jobs
	jobs, err := s.node.Database.ActiveJobs(blknum)
	if err != nil {
		glog.Error("Could not fetch active jobs ", err)
		return err
	}
	for _, j := range jobs {
		job, err := s.node.Eth.GetJob(big.NewInt(j.ID)) // benchmark; may be faster to reconstruct locally?
		if err != nil {
			glog.Error("Unable to get job for ", j.ID, err)
			continue
		}
		res, err := s.doTranscode(job)
		if !res || err != nil {
			glog.Error("Unable to restore transcoding of ", j.ID, err)
			continue
		}
	}
	return nil
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
