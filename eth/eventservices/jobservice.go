package eventservices

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/golang/glog"
	lpcommon "github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/eth"
	lpTypes "github.com/livepeer/go-livepeer/eth/types"
	"github.com/livepeer/go-livepeer/net"
	lpmscore "github.com/livepeer/lpms/core"
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
	sub, err := s.eventMonitor.SubscribeNewJob(ctx, logsCh, common.Address{}, func(l types.Log) (bool, error) {
		_, jid, _, _ := parseNewJobLog(l)

		job, err := s.node.Eth.GetJob(jid)
		if err != nil {
			glog.Errorf("Error getting job info: %v", err)
			return false, err
		}

		assigned, err := s.node.Eth.IsAssignedTranscoder(jid)
		if err != nil {
			glog.Errorf("Error checking for assignment: %v", err)
			return false, err
		}

		if assigned {
			return s.doTranscode(job)
		} else {
			return true, nil
		}
	})

	if err != nil {
		return err
	}

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

func (s *JobService) doTranscode(job *lpTypes.Job) (bool, error) {
	//Check if broadcaster has enough funds
	bDeposit, err := s.node.Eth.BroadcasterDeposit(job.BroadcasterAddress)
	if err != nil {
		glog.Errorf("Error getting broadcaster deposit: %v", err)
		return false, err
	}

	if bDeposit.Cmp(job.MaxPricePerSegment) == -1 {
		glog.Infof("Broadcaster does not have enough funds. Skipping job")
		return true, nil
	}

	tProfiles, err := txDataToVideoProfile(job.TranscodingOptions)
	if err != nil {
		glog.Errorf("Error processing transcoding options: %v", err)
		return false, err
	}

	//Create transcode config, make sure the profiles are sorted
	config := net.TranscodeConfig{StrmID: job.StreamId, Profiles: tProfiles, JobID: job.JobId, PerformOnchainClaim: true}
	glog.Infof("Transcoder got job %v - strmID: %v, tData: %v, config: %v", job.JobId, job.StreamId, job.TranscodingOptions, config)

	//Do The Transcoding
	cm := eth.NewBasicClaimManager(job.StreamId, job.JobId, job.BroadcasterAddress, job.MaxPricePerSegment, tProfiles, s.node.Eth, s.node.Ipfs)
	tr := transcoder.NewFFMpegSegmentTranscoder(tProfiles, "", s.node.WorkDir)
	strmIDs, err := s.node.TranscodeAndBroadcast(config, cm, tr)
	if err != nil {
		glog.Errorf("Transcode Error: %v", err)
		return false, err
	}

	//Notify Broadcaster
	sid := core.StreamID(job.StreamId)
	vids := make(map[core.StreamID]lpmscore.VideoProfile)
	for i, vp := range tProfiles {
		vids[strmIDs[i]] = vp
	}
	if err = s.node.NotifyBroadcaster(sid.GetNodeID(), sid, vids); err != nil {
		glog.Errorf("Notify Broadcaster Error: %v", err)
		return true, nil
	}

	firstClaimBlock := new(big.Int).Add(job.CreationBlock, big.NewInt(200))
	headersCh := make(chan *types.Header)
	s.eventMonitor.SubscribeNewBlock(context.Background(), headersCh, func(h *types.Header) (bool, error) {
		if cm.DidFirstClaim() {
			// If the first claim has already been made then exit
			return false, nil
		}

		// Check if current block is job creation block + 200
		if h.Number.Cmp(firstClaimBlock) != -1 {
			glog.Infof("Making the first claim")

			if cm.CanClaim() {
				err := cm.ClaimVerifyAndDistributeFees()
				if err != nil {
					return false, err
				} else {
					// If this claim was successful then the first claim has been made - exit
					return false, nil
				}
			} else {
				glog.Infof("No segments to claim")
				// Keep watching
				return true, nil
			}
		} else {
			return true, nil
		}
	})

	return true, nil
}

func txDataToVideoProfile(txData string) ([]lpmscore.VideoProfile, error) {
	profiles := make([]lpmscore.VideoProfile, 0)

	for i := 0; i+lpcommon.VideoProfileIDSize <= len(txData); i += lpcommon.VideoProfileIDSize {
		txp := txData[i : i+lpcommon.VideoProfileIDSize]

		p, ok := lpmscore.VideoProfileLookup[lpcommon.VideoProfileNameLookup[txp]]
		if !ok {
			glog.Errorf("Cannot find video profile for job: %v", txp)
			// return nil, core.ErrTranscode
		} else {
			profiles = append(profiles, p)
		}
	}

	return profiles, nil
}

func parseNewJobLog(log types.Log) (broadcasterAddr common.Address, jid *big.Int, streamID string, transOptions string) {
	return common.BytesToAddress(log.Topics[1].Bytes()), new(big.Int).SetBytes(log.Data[0:32]), string(log.Data[192:338]), string(log.Data[338:])
}
