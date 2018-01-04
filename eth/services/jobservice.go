package services

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
	sub, err := s.eventMonitor.SubscribeNewJob(ctx, logsCh, common.Address{}, func(l types.Log) error {
		_, jid, _, _ := parseNewJobLog(l)

		// TODO: store broadcaster address to verify received signed segments

		job, err := s.node.Eth.GetJob(jid)
		if err != nil {
			glog.Errorf("Error getting job info: %v", err)
			return nil
		}

		assigned, err := s.node.Eth.IsAssignedTranscoder(jid, job.MaxPricePerSegment)
		if err != nil {
			glog.Errorf("Error checking for assignment: %v", err)
			return nil
		}

		if assigned {
			return s.doTranscode(job)
		} else {
			return nil
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

func (s *JobService) doTranscode(job *lpTypes.Job) error {
	//Check if broadcaster has enough funds
	bDeposit, err := s.node.Eth.BroadcasterDeposit(job.BroadcasterAddress)
	if err != nil {
		glog.Errorf("Error getting broadcaster deposit: %v", err)
		return err
	}

	if bDeposit.Cmp(big.NewInt(0)) == 0 {
		glog.Infof("Broadcaster does not have enough funds. Skipping job")
		return nil
	}

	tProfiles, err := txDataToVideoProfile(job.TranscodingOptions)
	if err != nil {
		glog.Errorf("Error processing transcoding options: %v", err)
		return err
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
		return err
	}

	// headersCh := make(chan *types.Header)
	// s.eventMonitor.SubscribeNewBlock(ctx, headersCh, func(h *types.Header) error {
	// 	// Check if current block is job creation block + 200
	// 	return s.doFirstClaim(cm)
	// })

	//Notify Broadcaster
	sid := core.StreamID(job.StreamId)
	vids := make(map[core.StreamID]lpmscore.VideoProfile)
	for i, vp := range tProfiles {
		vids[strmIDs[i]] = vp
	}
	if err = s.node.NotifyBroadcaster(sid.GetNodeID(), sid, vids); err != nil {
		glog.Errorf("Notify Broadcaster Error: %v", err)
	}

	return nil
}

// func (s *JobService) doFirstClaim(cm) {

// }

func txDataToVideoProfile(txData string) ([]lpmscore.VideoProfile, error) {
	profiles := make([]lpmscore.VideoProfile, 0)

	for i := 0; i+lpcommon.VideoProfileIDSize < len(txData); i += lpcommon.VideoProfileIDSize {
		txp := txData[i : i+lpcommon.VideoProfileIDSize]

		p, ok := lpmscore.VideoProfileLookup[lpcommon.VideoProfileNameLookup[txp]]
		if !ok {
			// glog.Errorf("Cannot find video profile for job: %v", txp)
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
