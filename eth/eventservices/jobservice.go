package eventservices

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/cenkalti/backoff"
	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/eth"
	"github.com/livepeer/go-livepeer/eth/contracts"
	lpTypes "github.com/livepeer/go-livepeer/eth/types"
)

var (
	ErrJobServiceStarted  = fmt.Errorf("job service already started")
	ErrJobServicedStopped = fmt.Errorf("job service already stopped")

	CheckFirstClaimPollingInterval = time.Second * 15
)

type JobService struct {
	node         *core.LivepeerNode
	working      bool
	cancelWorker context.CancelFunc
	resubscribe  bool
}

func NewJobService(node *core.LivepeerNode) *JobService {
	return &JobService{
		node:        node,
		resubscribe: true,
	}
}

func (s *JobService) RestartTranscoder() error {
	return eth.RecoverClaims(s.node.Eth, s.node.Ipfs, s.node.Database)
}

func (s *JobService) Start(ctx context.Context) error {
	if s.working {
		return ErrJobServiceStarted
	}

	ctx, cancel := context.WithCancel(ctx)
	s.cancelWorker = cancel

	startBlock, err := s.node.Database.LastSeenBlock()
	if err == nil {
		go s.processHistoricalEvents(ctx, startBlock)
	}

	go func() {
		var (
			newJobSink = make(chan *contracts.JobsManagerNewJob)
			newJobSub  ethereum.Subscription
			err        error
		)

		for {
			if s.resubscribe {
				newJobSub, err = s.node.Eth.WatchForNewJob(true, newJobSink)
				if err != nil {
					glog.Error(err)
				} else {
					s.resubscribe = false
				}
			}

			select {
			case newJob := <-newJobSink:
				if err = s.processNewJob(ctx, newJob); err != nil {
					glog.Error(err)
				}
			case newJobErr := <-newJobSub.Err():
				newJobSub.Unsubscribe()
				s.resubscribe = true

				glog.Error("Error with NewJob subscription ", newJobErr)
			case <-ctx.Done():
				newJobSub.Unsubscribe()

				glog.Error("Received cancellation from job service; stopping")

				return
			}
		}
	}()

	return nil
}

func (s *JobService) Stop() error {
	if !s.working {
		return ErrJobServicedStopped
	}

	s.cancelWorker()
	s.working = false

	return nil
}

func (s *JobService) IsWorking() bool {
	return s.working
}

func (s *JobService) firstClaim(ctx context.Context, job *lpTypes.Job) error {
	// We might want to check for the latest block
	// (from the blockchain, not the local DB)
	// and skip creating the claimmanager if the job is too old
	cm, err := s.node.GetClaimManager(job)
	if err != nil {
		return err
	}

	tickCh := time.NewTicker(CheckFirstClaimPollingInterval).C

	go func() {
		for {
			select {
			case <-tickCh:
				lastSeenBlock, err := s.node.Database.LastSeenBlock()
				if err != nil {
					glog.Error("Error getting last seen block ", err)
					continue
				}

				firstClaimBlock := new(big.Int).Add(job.CreationBlock, eth.BlocksUntilFirstClaimDeadline)

				// Check if we should submit the first claim
				if lastSeenBlock.Cmp(firstClaimBlock) >= 0 {
					canClaim, err := cm.CanClaim(lastSeenBlock, job)
					if err != nil {
						glog.Errorf("Error checking if the transcoder can claim ", err)
						continue
					}

					if canClaim {
						err := cm.ClaimVerifyAndDistributeFees()
						if err != nil {
							glog.Errorf("Error executing first claim process ", err)
							continue
						}

						glog.Infof("Executed first claim process for job %v", job.JobId)
					}

					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	return nil
}

func (s *JobService) processNewJob(ctx context.Context, newJob *contracts.JobsManagerNewJob) error {
	var job *lpTypes.Job

	getJob := func() error {
		j, err := s.node.Eth.GetJob(newJob.JobId)
		if err != nil || j == nil {
			return err
		}
		if j == nil {
			glog.Errorf("Unable to get job %v. Should try again.", newJob.JobId)
			return errors.New("ErrGetJob")
		}
		if j.StreamId == "" {
			glog.Errorf("Got job %v with empty stream ID. Should try again.", newJob.JobId)
			return errors.New("ErrGetJob")
		}

		job = j

		if (job.TranscoderAddress == common.Address{}) {
			transcoderAddress, err := s.node.Eth.AssignedTranscoder(job)
			if err != nil {
				return err
			}

			job.TranscoderAddress = transcoderAddress
		}

		return nil
	}

	if err := backoff.Retry(getJob, backoff.NewConstantBackOff(time.Second*2)); err != nil {
		glog.Errorf("Error getting job info: %v", err)
		return err
	}

	if job.TranscoderAddress == s.node.Eth.Account().Address {
		dbjob := eth.EthJobToDBJob(job)
		if err := s.node.Database.InsertJob(dbjob); err != nil {
			return err
		}

		if err := s.firstClaim(ctx, job); err != nil {
			return err
		}
	}

	return nil
}

func (s *JobService) processHistoricalEvents(ctx context.Context, startBlock *big.Int) error {
	//Exit early if LastSeenBlock is zero (starting with a new db)
	if startBlock.Cmp(big.NewInt(0)) == 0 {
		return nil
	}

	if err := s.node.Eth.ProcessHistoricalNewJob(startBlock, true, func(newJob *contracts.JobsManagerNewJob) error {
		if err := s.processNewJob(ctx, newJob); err != nil {
			return err
		}

		return nil
	}); err != nil {
		return err
	}

	return nil
}
