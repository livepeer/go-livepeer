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
	"github.com/livepeer/go-livepeer/drivers"
	"github.com/livepeer/go-livepeer/eth"
	ethTypes "github.com/livepeer/go-livepeer/eth/types"
	"github.com/livepeer/go-livepeer/net"

	ffmpeg "github.com/livepeer/lpms/ffmpeg"
	"github.com/livepeer/lpms/stream"
	"github.com/livepeer/lpms/transcoder"
)

const TranscodeLoopTimeout = 10 * time.Minute

var profiles = []ffmpeg.VideoProfile{ffmpeg.P144p30fps16x9, ffmpeg.P240p30fps16x9} // ANGIE - MUST REMOVE THIS ONCE PROFILES IN JOB

// Transcoder / orchestrator RPC interface implementation
type orchestrator struct {
	address ethcommon.Address
	node    *LivepeerNode
}

func (orch *orchestrator) ServiceURI() *url.URL {
	return orch.node.ServiceURI
}

func (orch *orchestrator) CurrentBlock() *big.Int {
	if orch.node == nil || orch.node.Database == nil {
		return nil
	}
	block, _ := orch.node.Database.LastSeenBlock()
	return block
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

func (orch *orchestrator) TranscoderSecret() string {
	return orch.node.OrchSecret
}

func (orch *orchestrator) StreamIDs(jobId string) ([]StreamID, error) {
	streamIds := make([]StreamID, len(profiles))
	sid := StreamID(jobId)
	vid := sid.GetVideoID()
	for i, p := range profiles {
		strmId, err := MakeStreamID(vid, p.Name)
		if err != nil {
			glog.Error("Error making stream ID: ", err)
			return []StreamID{}, err
		}
		streamIds[i] = strmId
	}
	return streamIds, nil
}

func (orch *orchestrator) TranscodeSeg(jobId int64, md *SegTranscodingMetadata, seg *stream.HLSSegment) (*TranscodeResult, error) {
	return orch.node.sendToTranscodeLoop(jobId, md, seg)
}

func (orch *orchestrator) ServeTranscoder(stream net.Transcoder_RegisterTranscoderServer) {
	orch.node.serveTranscoder(stream)
}

func (orch *orchestrator) TranscoderResults(tcId int64, res *RemoteTranscoderResult) {
	orch.node.transcoderResults(tcId, res)
}

func NewOrchestrator(n *LivepeerNode) *orchestrator {
	var addr ethcommon.Address
	if n.Eth != nil {
		addr = n.Eth.Account().Address
	}
	return &orchestrator{
		node:    n,
		address: addr,
	}
}

// LivepeerNode transcode methods

type TranscodeResult struct {
	Err  error
	Sig  []byte
	Data [][]byte
	OS   drivers.OSSession
}

type SegChanData struct {
	seg *stream.HLSSegment
	md  *SegTranscodingMetadata
	res chan *TranscodeResult
}

type RemoteTranscoderResult struct {
	Segments [][]byte
	Err      error
}

type SegmentChan chan *SegChanData
type TranscoderChan chan *RemoteTranscoderResult

type transcodeConfig struct {
	ResultStrmIDs []StreamID
	JobID         string // ANGIE - WE'LL HAVE TO DELETE EITHER STREAMIDS OR JOBID
	Transcoder    transcoder.Transcoder
	OS            drivers.OSSession
	LocalOS       drivers.OSSession
}

func (n *LivepeerNode) getTaskChan(taskId int64) (TranscoderChan, error) {
	n.taskMutex.RLock()
	defer n.taskMutex.RUnlock()
	if tc, ok := n.taskChans[taskId]; ok {
		return tc, nil
	}
	return nil, fmt.Errorf("No transcoder channel")
}
func (n *LivepeerNode) addTaskChan() (int64, TranscoderChan) {
	n.taskMutex.Lock()
	defer n.taskMutex.Unlock()
	taskId := n.taskCount
	n.taskCount++
	if tc, ok := n.taskChans[taskId]; ok {
		// should really never happen
		glog.V(common.DEBUG).Info("Transcoder channel already exists for ", taskId)
		return taskId, tc
	}
	n.taskChans[taskId] = make(TranscoderChan, 1)
	return taskId, n.taskChans[taskId]
}
func (n *LivepeerNode) removeTaskChan(taskId int64) {
	n.taskMutex.Lock()
	defer n.taskMutex.Unlock()
	if _, ok := n.taskChans[taskId]; !ok {
		glog.V(common.DEBUG).Info("Transcoder channel nonexistent for job ", taskId)
		return
	}
	delete(n.taskChans, taskId)
}

func (n *LivepeerNode) getSegmentChan(jobId int64, os *net.OSInfo) (SegmentChan, error) {
	// concurrency concerns here? what if a chan is added mid-call?
	n.segmentMutex.Lock()
	defer n.segmentMutex.Unlock()
	if sc, ok := n.SegmentChans[jobId]; ok {
		return sc, nil
	}
	sc := make(SegmentChan, 1)
	glog.V(common.DEBUG).Info("Creating new segment chan for job ", jobId)
	n.SegmentChans[jobId] = sc
	if err := n.transcodeSegmentLoop(jobId, os, sc); err != nil {
		return nil, err
	}
	return sc, nil
}

func (n *LivepeerNode) sendToTranscodeLoop(jobId int64, md *SegTranscodingMetadata, seg *stream.HLSSegment) (*TranscodeResult, error) {
	glog.V(common.DEBUG).Infof("Starting to transcode segment %v", md.Seq)
	ch, err := n.getSegmentChan(jobId, md.OS)
	if err != nil {
		glog.Error("Could not find segment chan ", err)
		return nil, err
	}
	segChanData := &SegChanData{seg: seg, md: md, res: make(chan *TranscodeResult, 1)}
	select {
	case ch <- segChanData:
		glog.V(common.DEBUG).Info("Submitted segment to transcode loop ", md.Seq)
	default:
		// sending segChan should not block; if it does, the channel is busy
		glog.Error("Transcoder was busy with a previous segment!")
		return nil, fmt.Errorf("TranscoderBusy")
	}
	res := <-segChanData.res
	return res, res.Err
}

func (n *LivepeerNode) transcodeSeg(config transcodeConfig, seg *stream.HLSSegment, md *SegTranscodingMetadata) *TranscodeResult {
	var fnamep *string
	terr := func(err error) *TranscodeResult {
		if fnamep != nil {
			os.Remove(*fnamep)
		}
		return &TranscodeResult{Err: err}
	}

	// Prevent unnecessary work, check for replayed sequence numbers.
	// NOTE: If we ever process segments from the same job concurrently,
	// we may still end up doing work multiple times. But this is OK for now.

	//Assume d is in the right format, write it to disk
	inName := randName()
	if _, err := os.Stat(n.WorkDir); os.IsNotExist(err) {
		err := os.Mkdir(n.WorkDir, 0700)
		if err != nil {
			glog.Errorf("Transcoder cannot create workdir: %v", err)
			return terr(err)
		}
	}
	// Create input file from segment. Removed after claiming complete or error
	fname := path.Join(n.WorkDir, inName)
	fnamep = &fname
	if err := ioutil.WriteFile(fname, seg.Data, 0644); err != nil {
		glog.Errorf("Transcoder cannot write file: %v", err)
		return terr(err)
	}

	// Check if there's a transcoder available
	var transcoder Transcoder
	n.tcoderMutex.RLock()
	if n.Transcoder == nil {
		n.tcoderMutex.RUnlock()
		return terr(fmt.Errorf("No transcoders available on orchestrator"))
	}
	transcoder = n.Transcoder
	n.tcoderMutex.RUnlock()

	var url string
	// Small optimization: serve from disk for local transcoding
	if _, ok := transcoder.(*LocalTranscoder); ok {
		url = fname
	} else if drivers.IsOwnExternal(seg.Name) {
		// We're using a remote TC and segment is already in our own OS
		// Incurs an additional download for topologies with T on local network!
		url = seg.Name
	} else {
		// Need to store segment in our local OS
		var err error
		sid := StreamID(config.JobID)
		name := fmt.Sprintf("%s/%d.ts", sid.GetRendition(), seg.SeqNo)
		url, err = config.LocalOS.SaveData(name, seg.Data)
		if err != nil {
			return terr(err)
		}
		seg.Name = url
	}

	//Do the transcoding
	start := time.Now()
	tData, err := transcoder.Transcode(url, profiles)
	if err != nil {
		glog.Errorf("Error transcoding seg: %v - %v", seg.Name, err)
		return terr(err)
	}
	// transcodeEnd := time.Now().UTC()
	if len(tData) != len(config.ResultStrmIDs) {
		glog.Errorf("Did not receive the correct number of transcoded segments; got %v expected %v", len(tData), len(config.ResultStrmIDs))
		return terr(fmt.Errorf("MismatchedSegments"))
	}
	tProfileData := make(map[ffmpeg.VideoProfile][]byte, 0)
	glog.V(common.DEBUG).Infof("Transcoding of segment %v took %v", seg.SeqNo, time.Since(start))

	// Prepare the result object
	var tr TranscodeResult
	for i, _ := range config.ResultStrmIDs {
		if tData[i] == nil {
			glog.Errorf("Cannot find transcoded segment for %v", seg.SeqNo)
			continue
		}
		tProfileData[profiles[i]] = tData[i] // ANGIE - PROFILES MUST BE REPLACED HERE
		tr.Data = append(tr.Data, tData[i])
	}
	os.Remove(fname)
	tr.OS = config.OS
	return &tr
}

func (n *LivepeerNode) transcodeSegmentLoop(jobId int64, osInfo *net.OSInfo, segChan SegmentChan) error {
	glog.V(common.DEBUG).Info("Starting transcode segment loop for ", jobId)        // ANGIE - NO STREAMID ?
	profiles := []ffmpeg.VideoProfile{ffmpeg.P144p30fps16x9, ffmpeg.P240p30fps16x9} // ANGIE - NEED TO FIGURE OUT WHAT TO USE INSTEAD OF PROFILES, AND WHETHER TO USE JOBID HERE
	resultStrmIDs := make([]StreamID, len(profiles), len(profiles))

	sid := StreamID(jobId)
	for i, vp := range profiles {
		strmID, err := MakeStreamID(sid.GetVideoID(), vp.Name)
		if err != nil {
			glog.Error("Error making stream ID: ", err)
			return err
		}
		resultStrmIDs[i] = strmID
	}

	// Set up local OS for any remote transcoders to use if necessary
	if drivers.NodeStorage == nil {
		return fmt.Errorf("Missing local storage")
	}
	mid, err := sid.ManifestIDFromStreamID()
	if err != nil {
		return err
	}
	los := drivers.NodeStorage.NewSession(string(mid))

	// determine appropriate OS to use
	os := drivers.NewSession(osInfo)
	if os == nil {
		// no preference (or unknown pref), so use our own
		os = los
	}

	tr := transcoder.NewFFMpegSegmentTranscoder(profiles, n.WorkDir) // ANGIE - FIX PROFILES
	config := transcodeConfig{
		ResultStrmIDs: resultStrmIDs,
		JobID:         string(jobId),
		Transcoder:    tr,
		OS:            os,
		LocalOS:       los,
	}
	go func() {
		for {
			// XXX make context timeout configurable
			ctx, cancel := context.WithTimeout(context.Background(), TranscodeLoopTimeout)
			select {
			case <-ctx.Done():
				// timeout; clean up goroutine here
				jid := jobId
				os.EndSession()
				los.EndSession()
				glog.V(common.DEBUG).Info("Segment loop timed out; closing ", jid)
				n.segmentMutex.Lock()
				if _, ok := n.SegmentChans[jid]; ok {
					close(n.SegmentChans[jid])
					delete(n.SegmentChans, jid)
				}
				n.segmentMutex.Unlock()
				return
			case chanData := <-segChan:
				chanData.res <- n.transcodeSeg(config, chanData.seg, chanData.md)
			}
			cancel()
		}
	}()
	return nil
}

func (n *LivepeerNode) GetClaimManager(job *ethTypes.Job) (eth.ClaimManager, error) {
	n.claimMutex.Lock()
	defer n.claimMutex.Unlock()
	if job == nil {
		glog.Error("Nil job")
		return nil, fmt.Errorf("Nil job")
	}
	jobId := job.JobId.Int64()
	// XXX we should clear entries after some period of inactivity
	if cm, ok := n.ClaimManagers[jobId]; ok {
		return cm, nil
	}
	// no claimmanager exists yet; check if we're assigned the job
	if n.Eth == nil {
		return nil, nil
	}
	glog.Infof("Creating new claim manager for job %v", jobId)
	cm := eth.NewBasicClaimManager(job, n.Eth, n.Ipfs, n.Database)
	n.ClaimManagers[jobId] = cm
	return cm, nil
}

func (n *LivepeerNode) serveTranscoder(stream net.Transcoder_RegisterTranscoderServer) {
	transcoder := NewRemoteTranscoder(n, stream)

	n.tcoderMutex.Lock()
	n.Transcoder = transcoder
	n.tcoderMutex.Unlock()

	select {
	case <-transcoder.eof:
		glog.V(common.DEBUG).Info("Closing transcoder channel") // XXX cxn info
		n.tcoderMutex.Lock()
		n.Transcoder = nil
		n.tcoderMutex.Unlock()
		return
	}
}

func (n *LivepeerNode) transcoderResults(tcId int64, res *RemoteTranscoderResult) {
	remoteChan, err := n.getTaskChan(tcId)
	if err != nil {
		return // do we need to return anything?
	}
	remoteChan <- res
}

type RemoteTranscoder struct {
	node   *LivepeerNode
	stream net.Transcoder_RegisterTranscoderServer
	eof    chan struct{}
}

var RemoteTranscoderTimeout = 8 * time.Second

func (rt *RemoteTranscoder) Transcode(fname string, profiles []ffmpeg.VideoProfile) ([][]byte, error) {
	taskId, taskChan := rt.node.addTaskChan()
	defer rt.node.removeTaskChan(taskId)
	msg := &net.NotifySegment{
		Url:      fname,
		TaskId:   taskId,
		Profiles: common.ProfilesToTranscodeOpts(profiles),
	}
	err := rt.stream.Send(msg)
	if err != nil {
		glog.Error("Error sending message to remote transcoder ", err)
		rt.eof <- struct{}{}
		return [][]byte{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), RemoteTranscoderTimeout)
	defer cancel()
	select {
	case <-ctx.Done():
		// XXX remove transcoder from streams
		rt.eof <- struct{}{}
		return [][]byte{}, fmt.Errorf("Remote transcoder took too long")
	case chanData := <-taskChan:
		glog.Info("Successfully received results from remote transcoder ", len(chanData.Segments))
		return chanData.Segments, chanData.Err
	}
	return [][]byte{}, fmt.Errorf("Unknown error")
}

func NewRemoteTranscoder(n *LivepeerNode, stream net.Transcoder_RegisterTranscoderServer) *RemoteTranscoder {
	return &RemoteTranscoder{
		node:   n,
		stream: stream,
		eof:    make(chan struct{}, 1),
	}
}

func randName() string {
	rand.Seed(time.Now().UnixNano())
	x := make([]byte, 10, 10)
	for i := 0; i < len(x); i++ {
		x[i] = byte(rand.Uint32())
	}
	return fmt.Sprintf("%x.ts", x)
}
