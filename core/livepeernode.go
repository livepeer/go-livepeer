/*
Core contains the main functionality of the Livepeer node.
*/
package core

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"math/big"
	"math/rand"
	"os"
	"path"
	"sync"
	"time"

	"github.com/ericxtang/m3u8"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/eth"
	ethTypes "github.com/livepeer/go-livepeer/eth/types"
	"github.com/livepeer/go-livepeer/ipfs"
	"github.com/livepeer/go-livepeer/net"
	ffmpeg "github.com/livepeer/lpms/ffmpeg"
	"github.com/livepeer/lpms/stream"
	"github.com/livepeer/lpms/transcoder"
)

var ErrLivepeerNode = errors.New("ErrLivepeerNode")
var ErrTranscode = errors.New("ErrTranscode")
var ErrBroadcastTimeout = errors.New("ErrBroadcastTimeout")
var ErrBroadcastJob = errors.New("ErrBroadcastJob")
var ErrBroadcast = errors.New("ErrBroadcast")
var ErrEOF = errors.New("ErrEOF")
var ErrNotFound = errors.New("ErrNotFound")
var BroadcastTimeout = time.Second * 30
var EthRpcTimeout = 5 * time.Second
var EthEventTimeout = 5 * time.Second
var EthMinedTxTimeout = 60 * time.Second
var DefaultMasterPlaylistWaitTime = 60 * time.Second
var DefaultJobLength = int64(5760) //Avg 1 day in 15 sec blocks
var ConnFileWriteFreq = time.Duration(60) * time.Second
var LivepeerVersion = "0.2.4-unstable"
var SubscribeRetry = uint64(3)

//NodeID can be converted from libp2p PeerID.
type NodeID string

type PeerConn struct {
	NodeID   string
	NodeAddr string
}

type NodeType int

const (
	Broadcaster NodeType = iota
	Transcoder
	Gateway
	Bootnode
)

const TranscodeLoopTimeout = 10 * time.Minute

type SegmentChan chan *SegChanData

type SegChanData struct {
	seg *SignedSegment
	res chan error
}

//LivepeerNode handles videos going in and coming out of the Livepeer network.
type LivepeerNode struct {
	Identity        NodeID
	BootIDs         []string
	BootAddrs       []string
	VideoNetwork    net.VideoNetwork
	VideoCache      VideoCache
	Eth             eth.LivepeerEthClient
	EthEventMonitor eth.EventMonitor
	EthServices     map[string]eth.EventService
	ClaimManagers   map[int64]eth.ClaimManager
	SegmentChans    map[int64]SegmentChan
	Ipfs            ipfs.IpfsApi
	WorkDir         string
	NodeType        NodeType
	Database        *common.DB
	MonitorMetrics  bool

	claimMutex   *sync.Mutex
	segmentMutex *sync.Mutex
}

//NewLivepeerNode creates a new Livepeer Node. Eth can be nil.
func NewLivepeerNode(e eth.LivepeerEthClient, vn net.VideoNetwork, nodeId NodeID, wd string, dbh *common.DB) (*LivepeerNode, error) {
	if vn == nil {
		glog.Errorf("Cannot create a LivepeerNode without a VideoNetwork")
		return nil, ErrLivepeerNode
	}

	return &LivepeerNode{VideoCache: NewBasicVideoCache(vn), VideoNetwork: vn, Identity: nodeId, Eth: e, WorkDir: wd, Database: dbh, EthServices: make(map[string]eth.EventService), ClaimManagers: make(map[int64]eth.ClaimManager), SegmentChans: make(map[int64]SegmentChan), claimMutex: &sync.Mutex{}, segmentMutex: &sync.Mutex{}}, nil
}

//Start sets up the Livepeer protocol and connects the node to the network
func (n *LivepeerNode) Start(ctx context.Context, bootIDs, bootAddrs []string) error {
	if len(bootIDs) != len(bootAddrs) {
		return errors.New("BootIDs and BootAddrs do not match.")
	}
	//Set up protocol (to handle incoming streams)
	if err := n.VideoNetwork.SetupProtocol(); err != nil {
		glog.Errorf("Error setting up protocol: %v", err)
		return err
	}

	//Connect to bootstrap node.
	//TODO: Kick off a bootstrap process, which periodically checks for new peers and connect to them.
	errCount := 0
	if len(bootIDs) > 0 && len(bootAddrs) > 0 {
		for i, bootID := range bootIDs {
			bootAddr := bootAddrs[i]
			glog.V(common.DEBUG).Infof("Connecting to %v %v", bootID, bootAddr)
			if err := n.VideoNetwork.Connect(bootID, []string{bootAddr}); err != nil {
				glog.V(common.SHORT).Infof("Cannot connect to boot node %v: %v", bootID, err)
				errCount++
			}
		}
	}
	if errCount == len(bootIDs) {
		glog.Errorf("Current node cannot connect to any neighbors")
	}
	//TODO:Kick off process to periodically monitor peer connection by pinging them
	return nil
}

//CreateTranscodeJob creates the on-chain transcode job.
func (n *LivepeerNode) CreateTranscodeJob(strmID StreamID, profiles []ffmpeg.VideoProfile, price *big.Int) (*ethTypes.Job, error) {
	if n.Eth == nil {
		glog.Errorf("Cannot create transcode job, no eth client found")
		return nil, ErrNotFound
	}

	transOpts := common.ProfilesToTranscodeOpts(profiles)

	//Call eth client to create the job
	blknum, err := n.Eth.LatestBlockNum()
	if err != nil {
		return nil, ErrNotFound
	}

	_, err = n.Eth.Job(strmID.String(), ethcommon.ToHex(transOpts)[2:], price, big.NewInt(0).Add(blknum, big.NewInt(DefaultJobLength)))
	if err != nil {
		glog.Errorf("Error creating transcode job: %v", err)
		return nil, err
	}

	job, err := n.Eth.WatchForJob(strmID.String())
	if err != nil {
		glog.Error("Unable to monitor for job ", err)
		return nil, err
	}
	glog.V(common.DEBUG).Info("Got a new job from the blockchain: ", job.JobId)

	glog.Infof("Created broadcast job. Id: %v Price: %v. Type: %v", job.JobId, price, ethcommon.ToHex(transOpts)[2:])

	return job, nil
}

func (n *LivepeerNode) transcodeSegmentLoop(job *ethTypes.Job, segChan SegmentChan) error {
	glog.V(common.DEBUG).Info("Starting transcode segment loop for ", job.StreamId)
	cm, err := n.GetClaimManager(job)
	if err != nil {
		return err
	}
	resultStrmIDs := make([]StreamID, len(job.Profiles), len(job.Profiles))
	broadcasters := make(map[StreamID]stream.Broadcaster)
	sid := StreamID(job.StreamId)
	for i, vp := range job.Profiles {
		strmID, err := MakeStreamID(n.Identity, sid.GetVideoID(), vp.Name)
		if err != nil {
			glog.Error("Error making stream ID: ", err)
			return err
		}
		resultStrmIDs[i] = strmID
		broadcaster, err := n.VideoNetwork.GetBroadcaster(string(strmID))
		if err != nil {
			glog.Errorf("Error making new stream: %v", err)
			return err
		}
		broadcasters[strmID] = broadcaster
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
				chanData.res <- n.transcodeAndBroadcastSeg(&chanData.seg.Seg, chanData.seg.Sig, cm, tr, resultStrmIDs, broadcasters, config)
			}
			cancel()
		}
	}()
	return nil
}

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

//TranscodeAndBroadcast transcodes one stream into multiple streams (specified by TranscodeConfig), broadcasts the streams, and returns a list of streamIDs.
// Deprecated. Remove after some grace period.
func (n *LivepeerNode) TranscodeAndBroadcast(config net.TranscodeConfig, cm eth.ClaimManager, t transcoder.Transcoder) ([]StreamID, error) {
	//Create the broadcasters
	tProfiles := make([]ffmpeg.VideoProfile, len(config.Profiles), len(config.Profiles))
	resultStrmIDs := make([]StreamID, len(config.Profiles), len(config.Profiles))
	broadcasters := make(map[StreamID]stream.Broadcaster)
	sid := StreamID(config.StrmID)
	for i, vp := range config.Profiles {
		strmID, err := MakeStreamID(n.Identity, sid.GetVideoID(), vp.Name)
		if err != nil {
			glog.Errorf("Error making stream ID: %v", err)
			return nil, ErrTranscode
		}
		resultStrmIDs[i] = strmID
		tProfiles[i] = ffmpeg.VideoProfileLookup[vp.Name]

		broadcaster, err := n.VideoNetwork.GetBroadcaster(string(strmID))
		if err != nil {
			glog.Errorf("Error making new stream: %v", err)
			return nil, ErrTranscode
		}
		broadcasters[strmID] = broadcaster
	}

	//Subscribe to broadcast video, do the transcoding, broadcast the transcoded video, do the on-chain claim / verify
	err := n.VideoNetwork.TranscodeSub(context.Background(), config.StrmID, func(seqNo uint64, data []byte, eof bool) {
		glog.V(common.DEBUG).Infof("Starting to transcode segment %v", seqNo)
		totalStart := time.Now()
		if eof {
			n.Database.SetStopReason(config.JobID, "Stream finished")
			if cm != nil && config.PerformOnchainClaim {
				glog.V(common.SHORT).Infof("Stream finished. Claiming work.")

				canClaim, err := cm.CanClaim()
				if err != nil {
					glog.Error(err)
				}

				if canClaim {
					if err := cm.ClaimVerifyAndDistributeFees(); err != nil {
						glog.Errorf("Error claiming work: %v", err)
					}
				} else {
					glog.Infof("No segments to claim")
				}
			}
			return
		}

		if cm != nil && config.PerformOnchainClaim {
			sufficient, err := cm.SufficientBroadcasterDeposit()
			if err != nil {
				glog.Errorf("Error checking broadcaster funds: %v", err)
			}

			if !sufficient {
				glog.V(common.SHORT).Infof("Broadcaster does not have enough funds. Claiming work.")

				canClaim, err := cm.CanClaim()
				if err != nil {
					glog.Error(err)
				}

				if canClaim {
					if err := cm.ClaimVerifyAndDistributeFees(); err != nil {
						glog.Errorf("Error claiming work: %v", err)
					}
				} else {
					glog.Infof("No segments to claim")
				}
				return
			}
		}

		//Decode the segment
		start := time.Now()
		ss, err := BytesToSignedSegment(data)
		if err != nil {
			glog.Errorf("Error decoding byte array into segment: %v", err)
		}
		glog.V(common.DEBUG).Infof("Decoding of segment took %v", time.Since(start))

		//If running in on-chain mode, check that segment was signed by broadcaster ETH address
		segHash := (&ethTypes.Segment{StreamID: config.StrmID, SegmentSequenceNumber: big.NewInt(int64(seqNo)), DataHash: crypto.Keccak256Hash(ss.Seg.Data)}).Hash()
		if cm == nil || (cm.BroadcasterAddr() == ethcommon.Address{}) || eth.VerifySig(cm.BroadcasterAddr(), segHash.Bytes(), ss.Sig) {
			glog.V(common.DEBUG).Infof("Verified segment received from stream broadcaster")

			n.transcodeAndBroadcastSeg(&ss.Seg, ss.Sig, cm, t, resultStrmIDs, broadcasters, config)
			glog.V(common.DEBUG).Infof("Encoding and broadcasting of segment %v took %v", ss.Seg.SeqNo, time.Since(start))
			glog.V(common.SHORT).Infof("Finished transcoding segment %v, overall took %v\n\n\n", seqNo, time.Since(totalStart))
		} else {
			glog.Errorf("Invalid broadcaster signature for received segment for stream. Dropping segment")
		}
	})
	if err != nil {
		return nil, err
	}
	return resultStrmIDs, nil
}

func (n *LivepeerNode) transcodeAndBroadcastSeg(seg *stream.HLSSegment, sig []byte, cm eth.ClaimManager, t transcoder.Transcoder, resultStrmIDs []StreamID, broadcasters map[StreamID]stream.Broadcaster, config net.TranscodeConfig) error {

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
	for i, resultStrmID := range resultStrmIDs {
		//Insert the transcoded segments into the streams (streams are already broadcasted to the network)
		if tData[i] == nil {
			glog.Errorf("Cannot find transcoded segment for %v", seg.SeqNo)
			continue
		}

		newSeg := &stream.HLSSegment{SeqNo: seg.SeqNo, Name: fmt.Sprintf("%v_%d.ts", resultStrmID, seg.SeqNo), Data: tData[i], Duration: seg.Duration}
		broadcaster, ok := broadcasters[resultStrmID]
		if !ok {
			// glog.Errorf("Cannot find stream for %v", tranStrms)
			glog.Errorf("Cannot find broadcaster for %v", resultStrmID)
			continue
		}
		if err := n.BroadcastHLSSegToNetwork(string(resultStrmID), newSeg, broadcaster); err != nil {
			glog.Errorf("Error inserting transcoded segment into network: %v", err)
		}

		tProfileData[config.Profiles[i]] = tData[i]
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

func (n *LivepeerNode) BroadcastFinishMsg(strmID string) error {
	b, err := n.VideoNetwork.GetBroadcaster(strmID)
	if err != nil {
		glog.Errorf("Error getting broadcaster from network: %v", err)
		return err
	}

	return b.Finish()
}

func (n *LivepeerNode) BroadcastHLSSegToNetwork(strmID string, seg *stream.HLSSegment, b stream.Broadcaster) error {
	var sig []byte
	var err error
	if n.Eth != nil {
		segHash := (&ethTypes.Segment{StreamID: strmID, SegmentSequenceNumber: big.NewInt(int64(seg.SeqNo)), DataHash: crypto.Keccak256Hash(seg.Data)}).Hash()

		sig, err = n.Eth.Sign(segHash.Bytes())
		if err != nil {
			glog.Errorf("Error signing segment %v-%v: %v", strmID, seg.SeqNo, err)
			return err
		}
	}

	//Encode segment into []byte, broadcast it
	if ssb, err := SignedSegmentToBytes(SignedSegment{Seg: *seg, Sig: sig}); err == nil {
		if err = b.Broadcast(seg.SeqNo, ssb); err != nil {
			glog.Errorf("Error broadcasting segment to network: %v", err)
		}
	} else {
		glog.Errorf("Error encoding segment to []byte: %v", err)
		return err
	}

	return nil
}

//SubscribeFromNetwork subscribes to a stream on the network.  Returns the stream as a reference.
func (n *LivepeerNode) SubscribeFromNetwork(ctx context.Context, strmID StreamID, strm stream.HLSVideoStream) error {
	glog.V(common.DEBUG).Infof("Subscribe from network: %v", strmID)
	sub, err := n.VideoNetwork.GetSubscriber(strmID.String())
	if err != nil {
		glog.Errorf("Error getting subscriber: %v", err)
		return err
	}

	return sub.Subscribe(context.Background(), func(seqNo uint64, data []byte, eof bool) {
		//1 - the subscriber quits
		if eof {
			glog.Infof("Got EOF, unsubscribing to %v", strmID)
			if err := sub.Unsubscribe(); err != nil {
				glog.Errorf("Unsubscribe error: %v", err)
				return
			}
			strm.End()
			return
		}

		//Decode data into HLSSegment
		ss, err := BytesToSignedSegment(data)
		if err != nil {
			glog.Errorf("Error decoding byte array into segment: %v", err)
			return
		}

		//Add segment into a HLS buffer in VideoDB
		// glog.Infof("Inserting seg %v into stream %v", ss.Seg.Name, strmID)
		if err = strm.AddHLSSegment(&ss.Seg); err != nil {
			glog.Errorf("Error adding segment: %v", err)
		}
	})
}

//UnsubscribeFromNetwork unsubscribes to a stream on the network.
func (n *LivepeerNode) UnsubscribeFromNetwork(strmID StreamID) error {
	s, err := n.VideoNetwork.GetSubscriber(strmID.String())
	if err != nil {
		glog.Errorf("Error getting subscriber when unsubscribing from network: %v", err)
		return ErrNotFound
	}

	err = s.Unsubscribe()
	if err != nil {
		glog.Errorf("Error unsubscribing from network: %v", err)
		return err
	}

	return nil
}

//GetMasterPlaylistFromNetwork blocks until it gets the playlist, or it times out.
func (n *LivepeerNode) GetMasterPlaylistFromNetwork(mid ManifestID) *m3u8.MasterPlaylist {
	timer := time.NewTimer(DefaultMasterPlaylistWaitTime)
	plChan, err := n.VideoNetwork.GetMasterPlaylist(string(mid.GetNodeID()), mid.String())
	if err != nil {
		glog.Errorf("Error getting master playlist: %v", err)
		return nil
	}
	select {
	case pl := <-plChan:
		//Got pl
		return pl
	case <-timer.C:
		//timed out
		return nil
	}

}

//NotifyBroadcaster sends a messages to the broadcaster of the video stream, containing the new streamIDs of the transcoded video streams.
func (n *LivepeerNode) NotifyBroadcaster(nid NodeID, strmID StreamID, transcodeStrmIDs map[StreamID]ffmpeg.VideoProfile) error {
	ids := make(map[string]string)
	for sid, p := range transcodeStrmIDs {
		ids[sid.String()] = p.Name
	}
	if nid == n.Identity {
		return nil
	}
	return n.VideoNetwork.SendTranscodeResponse(string(nid), strmID.String(), ids)
}

func (n *LivepeerNode) StartEthServices() error {
	var err error
	for _, s := range n.EthServices {
		err = s.Start(context.Background())
		if err != nil {
			return err
		}
	}

	return nil
}

func (n *LivepeerNode) StopEthServices() error {
	var err error
	for _, s := range n.EthServices {
		err = s.Stop()
		if err != nil {
			return err
		}
	}

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

func randName() string {
	rand.Seed(time.Now().UnixNano())
	x := make([]byte, 10, 10)
	for i := 0; i < len(x); i++ {
		x[i] = byte(rand.Uint32())
	}
	return fmt.Sprintf("%x.ts", x)
}
