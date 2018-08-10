package core

import (
	"context"
	"fmt"
	"io/ioutil"
	"math/big"
	"math/rand"
	"net/url"
	"os"
	"path"
	"time"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/golang/glog"

	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/eth"
	ethTypes "github.com/livepeer/go-livepeer/eth/types"
	"github.com/livepeer/go-livepeer/net"
	ffmpeg "github.com/livepeer/lpms/ffmpeg"
	"github.com/livepeer/lpms/stream"
	"github.com/livepeer/lpms/transcoder"
)

const TranscodeLoopTimeout = 10 * time.Minute

// Transcoder / orchestrator RPC interface implementation
type orchestrator struct {
	serviceUri *url.URL
	address    ethcommon.Address
	node       *LivepeerNode
}

func (orch *orchestrator) ServiceURI() *url.URL {
	return orch.serviceUri
}

func (orch *orchestrator) CurrentBlock() *big.Int {
	if orch.node == nil || orch.node.Eth == nil {
		return nil
	}
	block, _ := orch.node.Eth.LatestBlockNum()
	return block
}

func (orch *orchestrator) GetJob(jid int64) (*ethTypes.Job, error) {
	if orch.node == nil || orch.node.Eth == nil {
		return nil, fmt.Errorf("Cannot get job; missing eth client")
	}
	job, err := orch.node.Eth.GetJob(big.NewInt(jid))
	if err != nil {
		return nil, err
	}
	if (job.TranscoderAddress == ethcommon.Address{}) {
		ta, err := orch.node.Eth.AssignedTranscoder(job)
		if err != nil {
			glog.Errorf("Could not get assigned transcoder for job %v, err: %s", jid, err.Error())
			// continue here without a valid transcoder address
		} else {
			job.TranscoderAddress = ta
		}
	}
	return job, nil
}

func (orch *orchestrator) Sign(msg []byte) ([]byte, error) {
	if orch.node == nil || orch.node.Eth == nil {
		return []byte{}, fmt.Errorf("Cannot sign; missing eth client")
	}
	return orch.node.Eth.Sign(crypto.Keccak256(msg))
}

func (orch *orchestrator) Address() ethcommon.Address {
	return orch.address
}

func (orch *orchestrator) StreamIDs(job *ethTypes.Job) ([]StreamID, error) {
	streamIds := make([]StreamID, len(job.Profiles))
	sid := StreamID(job.StreamId)
	vid := sid.GetVideoID()
	for i, p := range job.Profiles {
		strmId, err := MakeStreamID(orch.node.Identity, vid, p.Name)
		if err != nil {
			glog.Error("Error making stream ID: ", err)
			return []StreamID{}, err
		}
		streamIds[i] = strmId
	}
	return streamIds, nil
}

func (orch *orchestrator) TranscodeSeg(job *ethTypes.Job, ss *SignedSegment) error {
	return orch.node.TranscodeSegment(job, ss)
}

func NewOrchestrator(n *LivepeerNode) *orchestrator {
	var addr ethcommon.Address
	uri, _ := url.Parse("https://localhost:4433") // set via cli args; differentiate between listening iface and blockchain service uri
	if n.Eth != nil {
		var err error
		addr = n.Eth.Account().Address
		stringUri, err := n.Eth.GetServiceURI(addr)
		if err != nil || stringUri == "" {
			glog.Error("Transcoder does not have a service URI set; may be unreachable")
		}
		uri, err = url.Parse(stringUri)
		if err != nil {
			glog.Error("Service URI invalid; transcoder may be unreachable. ", err)
		}
	}
	return &orchestrator{
		node:       n,
		address:    addr,
		serviceUri: uri,
	}
}

// LivepeerNode transcode methods

type SegChanData struct {
	seg *SignedSegment
	res chan error
}

type SegmentChan chan *SegChanData

func (n *LivepeerNode) getSegmentChan(job *ethTypes.Job) (SegmentChan, error) {
	// concurrency concerns here? what if a chan is added mid-call?
	n.segmentMutex.Lock()
	defer n.segmentMutex.Unlock()
	jobId := job.JobId.Int64()
	if sc, ok := n.SegmentChans[jobId]; ok {
		return sc, nil
	}
	sc := make(SegmentChan, 1)
	glog.V(common.DEBUG).Info("Creating new segment chan for job ", jobId)
	n.SegmentChans[jobId] = sc
	if err := n.transcodeSegmentLoop(job, sc); err != nil {
		return nil, err
	}
	return sc, nil
}

func (n *LivepeerNode) TranscodeSegment(job *ethTypes.Job, ss *SignedSegment) error {
	glog.V(common.DEBUG).Infof("Starting to transcode segment %v", ss.Seg.SeqNo)
	ch, err := n.getSegmentChan(job)
	if err != nil {
		glog.Error("Could not find segment chan ", err)
		return err
	}
	segChan := &SegChanData{seg: ss, res: make(chan error, 1)}
	select {
	case ch <- segChan:
		glog.V(common.DEBUG).Info("Submitted segment to transcode loop")
	default:
		// sending segChan should not block; if it does, the channel is busy
		glog.Error("Transcoder was busy with a previous segment!")
		return fmt.Errorf("TranscoderBusy")
	}
	select {
	case err := <-segChan.res:
		if err != nil {
			return err
		}
	}
	return nil
}

func (n *LivepeerNode) transcodeAndBroadcastSeg(seg *stream.HLSSegment, sig []byte, cm eth.ClaimManager, t transcoder.Transcoder, resultStrmIDs []StreamID, config net.TranscodeConfig) error {

	// Prevent unnecessary work, check for replayed sequence numbers.
	// NOTE: If we ever process segments from the same job concurrently,
	// we may still end up doing work multiple times. But this is OK for now.
	hasReceipt, err := n.Database.ReceiptExists(config.JobID, seg.SeqNo)
	if err != nil || hasReceipt {
		glog.Errorf("Got a DB error (%v) or receipt exists (%v)", err, hasReceipt)
		if err == nil {
			err = fmt.Errorf("DuplicateSequence")
		}
		return err
	}

	// Check deposit
	if cm != nil && config.PerformOnchainClaim {
		sufficient, err := cm.SufficientBroadcasterDeposit()
		if err != nil {
			glog.Errorf("Error checking broadcaster deposit for job %v: %v", config.JobID, err)
			// Give the benefit of doubt in case of an unrelated issue
			sufficient = true
		}
		if !sufficient {
			glog.Error("Insufficient deposit for job ", config.JobID)
			return fmt.Errorf("Insufficient deposit")
		}
	}

	//Assume d is in the right format, write it to disk
	inName := randName()
	if _, err := os.Stat(n.WorkDir); os.IsNotExist(err) {
		err := os.Mkdir(n.WorkDir, 0700)
		if err != nil {
			glog.Errorf("Transcoder cannot create workdir: %v", err)
			return err
		}
	}
	// Create input file from segment. Removed after claiming complete or error
	fname := path.Join(n.WorkDir, inName)
	if err := ioutil.WriteFile(fname, seg.Data, 0644); err != nil {
		glog.Errorf("Transcoder cannot write file: %v", err)
		return err
	}

	transcodeStart := time.Now().UTC()
	// Ensure length matches expectations. 4 second + 25% wiggle factor, 60fps
	if err := ffmpeg.CheckMediaLen(fname, 4*1.25*1000, 60*4*1.25); err != nil {
		glog.Errorf("Media length check failed: %v", err)
		os.Remove(fname)
		return err
	}
	//Do the transcoding
	start := time.Now()
	tData, err := t.Transcode(fname)
	if err != nil {
		glog.Errorf("Error transcoding seg: %v - %v", seg.Name, err)
		os.Remove(fname)
		return err
	}
	transcodeEnd := time.Now().UTC()
	tProfileData := make(map[ffmpeg.VideoProfile][]byte, 0)
	glog.V(common.DEBUG).Infof("Transcoding of segment %v took %v", seg.SeqNo, time.Since(start))

	//Encode and broadcast the segment
	start = time.Now()
	for i, r := range resultStrmIDs {
		//Insert the transcoded segments into the streams (streams are already broadcasted to the network)
		if tData[i] == nil {
			glog.Errorf("Cannot find transcoded segment for %v", seg.SeqNo)
			continue
		}
		tProfileData[config.Profiles[i]] = tData[i]
		newSeg := &stream.HLSSegment{SeqNo: seg.SeqNo, Name: fmt.Sprintf("%v_%d.ts", r, seg.SeqNo), Data: tData[i], Duration: seg.Duration}
		n.VideoCache.InsertHLSSegment(r, newSeg)
	}
	//Don't do the onchain stuff unless specified
	if cm != nil && config.PerformOnchainClaim {
		err = cm.AddReceipt(int64(seg.SeqNo), fname, seg.Data, sig, tProfileData, transcodeStart, transcodeEnd)
		if err != nil {
			os.Remove(fname)
			return err
		}
	} else {
		// We aren't going through the claim process so remove input immediately
		os.Remove(fname)
	}
	return nil
}

func (n *LivepeerNode) transcodeSegmentLoop(job *ethTypes.Job, segChan SegmentChan) error {
	glog.V(common.DEBUG).Info("Starting transcode segment loop for ", job.StreamId)
	cm, err := n.GetClaimManager(job)
	if err != nil {
		return err
	}
	resultStrmIDs := make([]StreamID, len(job.Profiles), len(job.Profiles))
	sid := StreamID(job.StreamId)
	for i, vp := range job.Profiles {
		strmID, err := MakeStreamID(n.Identity, sid.GetVideoID(), vp.Name)
		if err != nil {
			glog.Error("Error making stream ID: ", err)
			return err
		}
		resultStrmIDs[i] = strmID
	}
	tr := transcoder.NewFFMpegSegmentTranscoder(job.Profiles, n.WorkDir)
	config := net.TranscodeConfig{
		StrmID:              job.StreamId,
		Profiles:            job.Profiles,
		JobID:               job.JobId,
		PerformOnchainClaim: cm != nil,
	}
	go func() {
		for {
			// XXX make context timeout configurable
			ctx, cancel := context.WithTimeout(context.Background(), TranscodeLoopTimeout)
			select {
			case <-ctx.Done():
				// timeout; clean up goroutine here
				jid := job.JobId.Int64()
				glog.V(common.DEBUG).Info("Segment loop timed out; closing ", jid)
				n.segmentMutex.Lock()
				if _, ok := n.SegmentChans[jid]; ok {
					close(n.SegmentChans[jid])
					delete(n.SegmentChans, jid)
				}
				n.segmentMutex.Unlock()
				n.claimMutex.Lock()
				if cm, ok := n.ClaimManagers[jid]; ok {
					go func() {
						err := cm.ClaimVerifyAndDistributeFees()
						if err != nil {
							glog.Errorf("Error claiming work for job %v: %v", jid, err)
						}
					}()
					delete(n.ClaimManagers, jid)
				}
				n.claimMutex.Unlock()
				return
			case chanData := <-segChan:
				chanData.res <- n.transcodeAndBroadcastSeg(&chanData.seg.Seg, chanData.seg.Sig, cm, tr, resultStrmIDs, config)
			}
			cancel()
		}
	}()
	return nil
}

func randName() string {
	rand.Seed(time.Now().UnixNano())
	x := make([]byte, 10, 10)
	for i := 0; i < len(x); i++ {
		x[i] = byte(rand.Uint32())
	}
	return fmt.Sprintf("%x.ts", x)
}
