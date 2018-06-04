/*
Package server is the place we integrate the Livepeer node with the LPMS media server.
*/
package server

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"math/rand"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/livepeer/go-livepeer/monitor"

	"github.com/ericxtang/m3u8"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/core"
	ethTypes "github.com/livepeer/go-livepeer/eth/types"
	lpmscore "github.com/livepeer/lpms/core"
	ffmpeg "github.com/livepeer/lpms/ffmpeg"
	"github.com/livepeer/lpms/segmenter"
	"github.com/livepeer/lpms/stream"
	"github.com/livepeer/lpms/vidplayer"
)

var ErrAlreadyExists = errors.New("StreamAlreadyExists")
var ErrRTMPPublish = errors.New("ErrRTMPPublish")
var ErrBroadcast = errors.New("ErrBroadcast")
var ErrHLSPlay = errors.New("ErrHLSPlay")
var ErrRTMPPlay = errors.New("ErrRTMPPlay")
var ErrRoundInit = errors.New("ErrRoundInit")

const HLSWaitInterval = time.Second
const HLSBufferCap = uint(43200) //12 hrs assuming 1s segment
const HLSBufferWindow = uint(5)

var SegOptions = segmenter.SegmenterOptions{SegLength: 4 * time.Second}

const HLSUnsubWorkerFreq = time.Second * 5

const EthRpcTimeout = 5 * time.Second
const EthEventTimeout = 5 * time.Second
const EthMinedTxTimeout = 60 * time.Second

var HLSWaitTime = time.Second * 45
var BroadcastPrice = big.NewInt(1)
var BroadcastJobVideoProfiles = []ffmpeg.VideoProfile{ffmpeg.P240p30fps4x3, ffmpeg.P360p30fps16x9}
var MinDepositSegmentCount = int64(75) //5 mins assuming 4s segments
var LastHLSStreamID core.StreamID
var LastManifestID core.ManifestID

type LivepeerServer struct {
	RTMPSegmenter  lpmscore.RTMPSegmenter
	LPMS           *lpmscore.LPMS
	HttpPort       string
	RtmpPort       string
	LivepeerNode   *core.LivepeerNode
	VideoNonce     map[string]uint64
	VideoNonceLock *sync.Mutex

	rtmpStreams                map[core.StreamID]stream.RTMPVideoStream
	hlsSubTimer                map[core.StreamID]time.Time
	hlsWorkerRunning           bool
	broadcastRtmpToHLSMap      map[string]string
	broadcastRtmpToManifestMap map[string]string
}

func NewLivepeerServer(rtmpPort string, rtmpIP string, httpPort string, httpIP string, lpNode *core.LivepeerNode) *LivepeerServer {
	opts := lpmscore.LPMSOpts{
		RtmpPort: rtmpPort, RtmpHost: rtmpIP, RtmpDisabled: true,
		HttpPort: httpPort, HttpHost: httpIP,
		WorkDir: lpNode.WorkDir,
	}
	switch lpNode.NodeType {
	case core.Broadcaster:
		opts.RtmpDisabled = false
	}
	server := lpmscore.New(&opts)
	return &LivepeerServer{RTMPSegmenter: server, LPMS: server, HttpPort: httpPort, RtmpPort: rtmpPort, LivepeerNode: lpNode, VideoNonce: map[string]uint64{}, VideoNonceLock: &sync.Mutex{}, rtmpStreams: make(map[core.StreamID]stream.RTMPVideoStream), broadcastRtmpToHLSMap: make(map[string]string), broadcastRtmpToManifestMap: make(map[string]string)}
}

//StartServer starts the LPMS server
func (s *LivepeerServer) StartMediaServer(ctx context.Context, maxPricePerSegment *big.Int, transcodingOptions string) error {
	if s.LivepeerNode.Eth != nil {
		BroadcastPrice = maxPricePerSegment
		bProfiles := make([]ffmpeg.VideoProfile, 0)
		for _, opt := range strings.Split(transcodingOptions, ",") {
			p, ok := ffmpeg.VideoProfileLookup[strings.TrimSpace(opt)]
			if ok {
				bProfiles = append(bProfiles, p)
			}
		}
		BroadcastJobVideoProfiles = bProfiles
		glog.V(common.SHORT).Infof("Transcode Job Price: %v, Transcode Job Type: %v", BroadcastPrice, BroadcastJobVideoProfiles)
	}

	//Start HLS unsubscribe worker
	s.hlsSubTimer = make(map[core.StreamID]time.Time)
	go s.startHlsUnsubscribeWorker(time.Second*30, HLSUnsubWorkerFreq)

	//LPMS handlers for handling RTMP video
	s.LPMS.HandleRTMPPublish(createRTMPStreamIDHandler(s), gotRTMPStreamHandler(s), endRTMPStreamHandler(s))
	s.LPMS.HandleRTMPPlay(getRTMPStreamHandler(s))

	//LPMS hanlder for handling HLS video play
	s.LPMS.HandleHLSPlay(getHLSMasterPlaylistHandler(s), getHLSMediaPlaylistHandler(s), getHLSSegmentHandler(s))

	//Start the LPMS server
	lpmsCtx, cancel := context.WithCancel(context.Background())
	ec := make(chan error, 1)
	go func() {
		ec <- s.LPMS.Start(lpmsCtx)
	}()

	ts := make(chan struct{}, 1)
	if s.LivepeerNode.NodeType == core.Transcoder {
		go func() {
			port := ":4433" // default port; should it be 443 instead?
			// fetch service URI and extract port, if any.
			serviceURI := "https://localhost" + port // XXX do better; cmdarg?
			if s.LivepeerNode.Eth != nil {
				var err error
				addr := s.LivepeerNode.Eth.Account().Address
				serviceURI, err = s.LivepeerNode.Eth.GetServiceURI(addr)
				if err != nil || serviceURI == "" {
					glog.Error("Transcoder does not have a service URI set; may be unreachable")
				}
			}
			uri, err := url.Parse(serviceURI) // should be publicly accessible
			if err == nil && uri.Port() != "" {
				port = ":" + uri.Port() // bind to all interfaces by default
			}
			if err != nil {
				glog.Error("Service URI invalid; transcoder may be unreachable")
			}
			StartTranscodeServer(port, uri, s.LivepeerNode)
			ts <- struct{}{}
		}()
	}

	select {
	case err := <-ec:
		glog.Infof("LPMS Server Error: %v.  Quitting...", err)
		cancel()
		return err
	case <-ts:
		glog.Info("Transcoder server down. Quitting...")
		cancel()
		return errors.New("TranscoderServerDown")
	case <-ctx.Done():
		cancel()
		return ctx.Err()
	}
}

//RTMP Publish Handlers
func createRTMPStreamIDHandler(s *LivepeerServer) func(url *url.URL) (strmID string) {
	return func(url *url.URL) (strmID string) {
		id, err := core.MakeStreamID(s.LivepeerNode.Identity, core.RandomVideoID(), "RTMP")
		if err != nil {
			glog.Errorf("Error making stream ID")
			return ""
		}
		return id.String()
	}
}

func gotRTMPStreamHandler(s *LivepeerServer) func(url *url.URL, rtmpStrm stream.RTMPVideoStream) (err error) {
	return func(url *url.URL, rtmpStrm stream.RTMPVideoStream) (err error) {
		// For now, only allow a single stream
		if len(s.rtmpStreams) > 0 {
			return ErrAlreadyExists
		}

		s.VideoNonceLock.Lock()
		nonce, ok := s.VideoNonce[rtmpStrm.GetStreamID()]
		if !ok {
			nonce = rand.Uint64()
			s.VideoNonce[rtmpStrm.GetStreamID()] = nonce
		} else {
			// We can only have one concurrent stream for now
			return ErrAlreadyExists
		}
		s.VideoNonceLock.Unlock()

		var rpcBcast *broadcaster
		if s.LivepeerNode.Eth != nil {
			//Check if round is initialized
			initialized, err := s.LivepeerNode.Eth.CurrentRoundInitialized()
			if err != nil {
				glog.Errorf("Could not check whether round was initialized: %v", err)
				return err
			}
			if !initialized {
				glog.Infof("Round was uninitialized, can't create job. Please try again in a few blocks.")
				return ErrRoundInit
			}
			// Check deposit
			deposit, err := s.LivepeerNode.Eth.BroadcasterDeposit(s.LivepeerNode.Eth.Account().Address)
			if err != nil {
				glog.Errorf("Error getting deposit: %v", err)
				return ErrBroadcast
			}

			glog.V(common.SHORT).Infof("Current deposit is: %v", deposit)

			minDeposit := big.NewInt(0).Mul(BroadcastPrice, big.NewInt(MinDepositSegmentCount))
			if deposit.Cmp(minDeposit) < 0 {
				glog.Errorf("Low deposit (%v) - cannot start broadcast session", deposit)
				if s.LivepeerNode.MonitorMetrics {
					monitor.LogStreamCreateFailed(rtmpStrm.GetStreamID(), nonce, "LowDeposit")
				}
				return ErrBroadcast
			}
		}

		//Check if stream ID already exists
		if _, ok := s.rtmpStreams[core.StreamID(rtmpStrm.GetStreamID())]; ok {
			return ErrAlreadyExists
		}

		//Add stream to stream store
		s.rtmpStreams[core.StreamID(rtmpStrm.GetStreamID())] = rtmpStrm

		//We try to automatically determine the video profile from the RTMP stream.
		var vProfile ffmpeg.VideoProfile
		resolution := fmt.Sprintf("%vx%v", rtmpStrm.Width(), rtmpStrm.Height())
		for _, vp := range ffmpeg.VideoProfileLookup {
			if vp.Resolution == resolution {
				vProfile = vp
				break
			}
		}
		if vProfile.Name == "" {
			vProfile = ffmpeg.P720p30fps16x9
			glog.V(common.SHORT).Infof("Cannot automatically detect the video profile - setting it to %v", vProfile)
		}

		//Create a new HLS StreamID.  If streamID is passed in, use that one.  Otherwise, generate a random ID.
		hlsStrmID := core.StreamID(url.Query().Get("hlsStrmID"))
		if hlsStrmID == "" {
			hlsStrmID, err = core.MakeStreamID(s.LivepeerNode.Identity, core.RandomVideoID(), vProfile.Name)
			if err != nil {
				glog.Errorf("Error making stream ID")
				return ErrRTMPPublish
			}
		}
		if hlsStrmID.GetNodeID() != s.LivepeerNode.Identity {
			glog.Errorf("Cannot create a HLS stream with Node ID: %v", hlsStrmID.GetNodeID())
			return ErrRTMPPublish
		}

		pl, err := m3u8.NewMediaPlaylist(stream.DefaultHLSStreamWin, stream.DefaultHLSStreamCap)
		if err != nil {
			glog.Errorf("Error creating playlist: %v", err)
			return ErrRTMPPublish
		}

		//Get a broadcaster from the network
		broadcaster, err := s.LivepeerNode.VideoNetwork.GetBroadcaster(string(hlsStrmID))
		if err != nil {
			glog.Errorf("Error creating HLS stream for segmentation: %v", err)
			return ErrRTMPPublish
		}
		LastHLSStreamID = hlsStrmID

		streamStarted := false
		//Segment the stream, insert the segments into the broadcaster
		go func(broadcaster stream.Broadcaster, rtmpStrm stream.RTMPVideoStream) {
			hlsStrm := stream.NewBasicHLSVideoStream(string(hlsStrmID), stream.DefaultHLSStreamWin)
			hlsStrm.SetSubscriber(func(seg *stream.HLSSegment, eof bool) {
				if eof {
					broadcaster.Finish()
					return
				}

				if streamStarted == false {
					streamStarted = true
					if s.LivepeerNode.MonitorMetrics {
						monitor.LogStreamStartedEvent(hlsStrmID.String(), nonce)
					}
				}
				var sig []byte
				if s.LivepeerNode.Eth != nil {
					segHash := (&ethTypes.Segment{StreamID: hlsStrm.GetStreamID(), SegmentSequenceNumber: big.NewInt(int64(seg.SeqNo)), DataHash: crypto.Keccak256Hash(seg.Data)}).Hash()
					sig, err = s.LivepeerNode.Eth.Sign(segHash.Bytes())
					if err != nil {
						glog.Errorf("Error signing segment %v-%v: %v", hlsStrm.GetStreamID(), seg.SeqNo, err)
						return
					}
				}

				//Encode segment into []byte, broadcast it
				if ssb, err := core.SignedSegmentToBytes(core.SignedSegment{Seg: *seg, Sig: sig}); err != nil {
					glog.Errorf("Error signing segment: %v", seg.SeqNo)
				} else {
					if err := broadcaster.Broadcast(seg.SeqNo, ssb); err != nil {
						glog.Errorf("Error broadcasting to network: %v", err)
					}
				}

				if rpcBcast != nil {
					SubmitSegment(rpcBcast, seg)
				}
			})

			err := s.RTMPSegmenter.SegmentRTMPToHLS(context.Background(), rtmpStrm, hlsStrm, SegOptions)
			if err != nil {
				if err := s.LivepeerNode.BroadcastFinishMsg(hlsStrmID.String()); err != nil {
					glog.Errorf("Error broadcaseting finish message: %v", err)
				}
				// Stop the incoming RTMP connection.
				// TODO retry segmentation if err != SegmenterTimeout; may be recoverable
				rtmpStrm.Close()
			}

		}(broadcaster, rtmpStrm)

		//Create the manifest and broadcast it (so the video can be consumed by itself without transcoding)
		mid, err := core.MakeManifestID(hlsStrmID.GetNodeID(), hlsStrmID.GetVideoID())
		if err != nil {
			glog.Errorf("Error creating manifest id: %v", err)
			return ErrRTMPPublish
		}
		LastManifestID = mid
		manifest := m3u8.NewMasterPlaylist()
		vParams := ffmpeg.VideoProfileToVariantParams(ffmpeg.VideoProfileLookup[vProfile.Name])
		manifest.Append(fmt.Sprintf("%v.m3u8", hlsStrmID), pl, vParams)

		if err := s.LivepeerNode.VideoNetwork.UpdateMasterPlaylist(string(mid), manifest); err != nil {
			glog.Errorf("Error broadasting manifest to network: %v", err)
		}
		if s.LivepeerNode.MonitorMetrics {
			monitor.LogStreamCreatedEvent(mid.String(), nonce)
		}

		//Set up the transcode response callback
		s.LivepeerNode.VideoNetwork.ReceivedTranscodeResponse(string(hlsStrmID), func(result map[string]string) {
			//Parse through the results
			for strmID, tProfile := range result {
				vParams := ffmpeg.VideoProfileToVariantParams(ffmpeg.VideoProfileLookup[tProfile])
				pl, _ := m3u8.NewMediaPlaylist(stream.DefaultHLSStreamWin, stream.DefaultHLSStreamCap)
				variant := &m3u8.Variant{URI: fmt.Sprintf("%v.m3u8", strmID), Chunklist: pl, VariantParams: vParams}
				manifest.Append(variant.URI, variant.Chunklist, variant.VariantParams)
			}

			//Update the master playlist on the network
			if err = s.LivepeerNode.VideoNetwork.UpdateMasterPlaylist(string(mid), manifest); err != nil {
				glog.Errorf("Error updating master playlist on network: %v", err)
				return
			}
		})

		glog.Infof("\n\nVideo Created With ManifestID: %v\n\n", mid)
		glog.V(common.SHORT).Infof("\n\nhlsStrmID: %v\n\n", hlsStrmID)

		//Remember HLS stream so we can remove later
		s.broadcastRtmpToHLSMap[rtmpStrm.GetStreamID()] = string(hlsStrmID)
		s.broadcastRtmpToManifestMap[rtmpStrm.GetStreamID()] = string(mid)

		if s.LivepeerNode.Eth != nil {
			//Create Transcode Job Onchain
			go func() {
				job, err := s.LivepeerNode.CreateTranscodeJob(hlsStrmID, BroadcastJobVideoProfiles, BroadcastPrice)
				if err != nil {
					return // XXX feed back error?
				}
				tca, err := s.LivepeerNode.Eth.AssignedTranscoder(job.JobId)
				if err != nil {
					return // XXX feed back error?
				}
				if (tca == ethcommon.Address{}) {
					glog.Error("A transcoder was not assigned! Ensure the broadcast price meets the minimum for the transcoder pool")
					return // XXX feed back error?
				}
				serviceUri, err := s.LivepeerNode.Eth.GetServiceURI(tca)
				if err != nil || serviceUri == "" {
					glog.Error("Unable to retrieve Service URI for %v: %v", tca, err)
					return // XXX feed back error?
				}
				rpcBcast, err = StartBroadcastClient(serviceUri, s.LivepeerNode, job)
				if err != nil {
					return // XXX feed back error?
				}
			}()
		}
		return nil
	}
}

func endRTMPStreamHandler(s *LivepeerServer) func(url *url.URL, rtmpStrm stream.RTMPVideoStream) error {
	return func(url *url.URL, rtmpStrm stream.RTMPVideoStream) error {
		rtmpID := rtmpStrm.GetStreamID()
		hlsID := s.broadcastRtmpToHLSMap[rtmpID]
		manifestID := s.broadcastRtmpToManifestMap[rtmpID]
		//Remove RTMP stream
		delete(s.rtmpStreams, core.StreamID(rtmpID))
		if b, err := s.LivepeerNode.VideoNetwork.GetBroadcaster(hlsID); err != nil {
			glog.Errorf("Error getting broadcaster from network: %v", err)
		} else {
			b.Finish()
		}
		//Remove Manifest
		s.LivepeerNode.VideoCache.EvictHLSMasterPlaylist(core.ManifestID(manifestID))
		//Remove the master playlist from the network
		s.LivepeerNode.VideoNetwork.UpdateMasterPlaylist(manifestID, nil)
		//Remove the stream from cache
		s.LivepeerNode.VideoCache.EvictHLSStream(core.StreamID(hlsID))

		if s.LivepeerNode.MonitorMetrics {
			s.VideoNonceLock.Lock()
			if _, ok := s.VideoNonce[rtmpStrm.GetStreamID()]; ok {
				monitor.LogStreamEndedEvent(manifestID, s.VideoNonce[rtmpStrm.GetStreamID()])
				delete(s.VideoNonce, rtmpStrm.GetStreamID())
			}
			s.VideoNonceLock.Unlock()
		}
		return nil
	}
}

//End RTMP Publish Handlers

//HLS Play Handlers
func getHLSMasterPlaylistHandler(s *LivepeerServer) func(url *url.URL) (*m3u8.MasterPlaylist, error) {
	return func(url *url.URL) (*m3u8.MasterPlaylist, error) {
		manifestID, err := parseManifestID(url.Path)
		if err != nil {
			return nil, vidplayer.ErrNotFound
		}

		//Just load it from the cache (it's already hooked up to the network)
		manifest := s.LivepeerNode.VideoCache.GetHLSMasterPlaylist(core.ManifestID(manifestID))
		if manifest == nil {
			return nil, vidplayer.ErrNotFound
		}
		return manifest, nil
	}
}

func getHLSMediaPlaylistHandler(s *LivepeerServer) func(url *url.URL) (*m3u8.MediaPlaylist, error) {
	return func(url *url.URL) (*m3u8.MediaPlaylist, error) {
		strmID, err := parseStreamID(url.Path)
		if err != nil {
			glog.Errorf("Error parsing for stream id: %v", err)
			return nil, err
		}

		//Get the hls playlist, update the timeout timer
		pl := s.LivepeerNode.VideoCache.GetHLSMediaPlaylist(strmID)
		if pl == nil {
			return nil, vidplayer.ErrNotFound
		} else {
			s.hlsSubTimer[strmID] = time.Now()
		}
		return pl, nil
	}
}

func getHLSSegmentHandler(s *LivepeerServer) func(url *url.URL) ([]byte, error) {
	return func(url *url.URL) ([]byte, error) {
		strmID, err := parseStreamID(url.Path)
		if err != nil {
			glog.Errorf("Error parsing streamID: %v", err)
		}

		segName := parseSegName(url.Path)
		if segName == "" {
			return nil, vidplayer.ErrNotFound
		}

		seg := s.LivepeerNode.VideoCache.GetHLSSegment(strmID, segName)
		if seg == nil {
			return nil, vidplayer.ErrNotFound
		} else {
			return seg.Data, nil
		}
	}
}

//End HLS Play Handlers

//Start RTMP Play Handlers
func getRTMPStreamHandler(s *LivepeerServer) func(url *url.URL) (stream.RTMPVideoStream, error) {
	return func(url *url.URL) (stream.RTMPVideoStream, error) {
		strmID, err := parseStreamID(url.Path)
		if err != nil {
			glog.Errorf("Error parsing streamID with url %v - %v", url.Path, err)
			return nil, ErrRTMPPlay
		}
		strm, ok := s.rtmpStreams[core.StreamID(strmID)]
		if !ok {
			glog.Errorf("Cannot find RTMP stream")
			return nil, vidplayer.ErrNotFound
		}

		//Could use a subscriber, but not going to here because the RTMP stream doesn't need to be available for consumption by multiple views.  It's only for the segmenter.
		return strm, nil
	}
}

//End RTMP Handlers

//Helper Methods Begin

// //HLS video streams need to be timed out.  When the viewer is no longer watching the stream, we send a Unsubscribe message to the network.
func (s *LivepeerServer) startHlsUnsubscribeWorker(limit time.Duration, freq time.Duration) {
	s.hlsWorkerRunning = true
	defer func() { s.hlsWorkerRunning = false }()
	for {
		time.Sleep(freq)
		for sid, t := range s.hlsSubTimer {
			if time.Since(t) > limit {
				glog.V(common.SHORT).Infof("Inactive HLS Stream %v - unsubscribing", sid)
				s.LivepeerNode.UnsubscribeFromNetwork(sid)
				delete(s.hlsSubTimer, sid)
			}
		}
	}
}

func parseManifestID(reqPath string) (core.ManifestID, error) {
	var strmID string
	regex, _ := regexp.Compile("\\/stream\\/([[:alpha:]]|\\d)*")
	match := regex.FindString(reqPath)
	if match != "" {
		strmID = strings.Replace(match, "/stream/", "", -1)
	}
	mid := core.ManifestID(strmID)
	if mid.IsValid() {
		return mid, nil
	} else {
		return "", vidplayer.ErrNotFound
	}
}

func parseStreamID(reqPath string) (core.StreamID, error) {
	var strmID string
	regex, _ := regexp.Compile("\\/stream\\/([[:alpha:]]|\\d)*")
	match := regex.FindString(reqPath)
	if match != "" {
		strmID = strings.Replace(match, "/stream/", "", -1)
	}
	sid := core.StreamID(strmID)
	if sid.IsValid() {
		return sid, nil
	} else {
		return "", vidplayer.ErrNotFound
	}
}

func parseSegName(reqPath string) string {
	var segName string
	regex, _ := regexp.Compile("\\/stream\\/.*\\.ts")
	match := regex.FindString(reqPath)
	if match != "" {
		segName = strings.Replace(match, "/stream/", "", -1)
	}
	return segName
}
