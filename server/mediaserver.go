/*
Package server is the place we integrate the Livepeer node with the LPMS media server.
*/
package server

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/ericxtang/m3u8"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/core"
	ethTypes "github.com/livepeer/go-livepeer/eth/types"
	lpmscore "github.com/livepeer/lpms/core"
	"github.com/livepeer/lpms/segmenter"
	"github.com/livepeer/lpms/stream"
	"github.com/livepeer/lpms/vidplayer"
)

var ErrAlreadyExists = errors.New("StreamAlreadyExists")
var ErrRTMPPublish = errors.New("ErrRTMPPublish")
var ErrBroadcast = errors.New("ErrBroadcast")
var ErrHLSPlay = errors.New("ErrHLSPlay")
var ErrRTMPPlay = errors.New("ErrRTMPPlay")

const HLSWaitInterval = time.Second
const HLSBufferCap = uint(43200) //12 hrs assuming 1s segment
const HLSBufferWindow = uint(5)

var SegOptions = segmenter.SegmenterOptions{SegLength: 8 * time.Second}

const HLSUnsubWorkerFreq = time.Second * 5

const EthRpcTimeout = 5 * time.Second
const EthEventTimeout = 5 * time.Second
const EthMinedTxTimeout = 60 * time.Second

var HLSWaitTime = time.Second * 45
var BroadcastPrice = big.NewInt(1)
var BroadcastJobVideoProfiles = []lpmscore.VideoProfile{lpmscore.P240p30fps4x3, lpmscore.P360p30fps16x9}
var LastHLSStreamID core.StreamID
var LastManifestID core.ManifestID

type LivepeerServer struct {
	RTMPSegmenter lpmscore.RTMPSegmenter
	LPMS          *lpmscore.LPMS
	HttpPort      string
	RtmpPort      string
	FfmpegPath    string
	LivepeerNode  *core.LivepeerNode

	rtmpStreams                map[core.StreamID]stream.RTMPVideoStream
	hlsSubTimer                map[core.StreamID]time.Time
	hlsWorkerRunning           bool
	broadcastRtmpToHLSMap      map[string]string
	broadcastRtmpToManifestMap map[string]string
}

func NewLivepeerServer(rtmpPort string, httpPort string, ffmpegPath string, lpNode *core.LivepeerNode) *LivepeerServer {
	server := lpmscore.New(rtmpPort, httpPort, ffmpegPath, "", fmt.Sprintf("%v/.tmp", lpNode.WorkDir))
	return &LivepeerServer{RTMPSegmenter: server, LPMS: server, HttpPort: httpPort, RtmpPort: rtmpPort, FfmpegPath: ffmpegPath, LivepeerNode: lpNode, rtmpStreams: make(map[core.StreamID]stream.RTMPVideoStream), broadcastRtmpToHLSMap: make(map[string]string), broadcastRtmpToManifestMap: make(map[string]string)}
}

//StartServer starts the LPMS server
func (s *LivepeerServer) StartMediaServer(ctx context.Context, maxPricePerSegment *big.Int, transcodingOptions string) error {
	if s.LivepeerNode.Eth != nil {
		BroadcastPrice = maxPricePerSegment
		bProfiles := make([]lpmscore.VideoProfile, 0)
		for _, opt := range strings.Split(transcodingOptions, ",") {
			p, ok := lpmscore.VideoProfileLookup[strings.TrimSpace(opt)]
			if ok {
				bProfiles = append(bProfiles, p)
			}
		}
		BroadcastJobVideoProfiles = bProfiles
		glog.Infof("Transcode Job Price: %v, Transcode Job Type: %v", BroadcastPrice, BroadcastJobVideoProfiles)
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

	select {
	case err := <-ec:
		glog.Infof("LPMS Server Error: %v.  Quitting...", err)
		cancel()
		return err
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
		if s.LivepeerNode.Eth != nil {
			// Check deposit
			deposit, err := s.LivepeerNode.Eth.BroadcasterDeposit(s.LivepeerNode.Eth.Account().Address)
			if err != nil {
				glog.Errorf("Error getting deposit: %v", err)
				return ErrBroadcast
			}

			glog.Infof("Current deposit is: %v", deposit)

			if deposit.Cmp(BroadcastPrice) < 0 {
				glog.Errorf("Low deposit (%v) - cannot start broadcast session", deposit)
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
		var vProfile lpmscore.VideoProfile
		resolution := fmt.Sprintf("%vx%v", rtmpStrm.Width(), rtmpStrm.Height())
		for _, vp := range lpmscore.VideoProfileLookup {
			if vp.Resolution == resolution {
				vProfile = vp
				break
			}
		}
		if vProfile.Name == "" {
			vProfile = lpmscore.P720p30fps16x9
			glog.Infof("Cannot automatically detect the video profile - setting it to %v", vProfile)
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

		//Segment the stream, insert the segments into the broadcaster
		go func(broadcaster stream.Broadcaster, rtmpStrm stream.RTMPVideoStream) {
			hlsStrm := stream.NewBasicHLSVideoStream(string(hlsStrmID), stream.DefaultHLSStreamWin)
			hlsStrm.SetSubscriber(func(seg *stream.HLSSegment, eof bool) {
				if eof {
					broadcaster.Finish()
					return
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
			})

			err := s.RTMPSegmenter.SegmentRTMPToHLS(context.Background(), rtmpStrm, hlsStrm, SegOptions)
			if err != nil {
				// glog.Infof("Error in segmenter: %v, broadcasting finish message", err)
				if err := s.LivepeerNode.BroadcastFinishMsg(hlsStrmID.String()); err != nil {
					glog.Errorf("Error broadcaseting finish message: %v", err)
				}
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
		vParams := lpmscore.VideoProfileToVariantParams(lpmscore.VideoProfileLookup[vProfile.Name])
		manifest.Append(fmt.Sprintf("%v.m3u8", hlsStrmID), pl, vParams)

		if err := s.LivepeerNode.VideoNetwork.UpdateMasterPlaylist(string(mid), manifest); err != nil {
			glog.Errorf("Error broadasting manifest to network: %v", err)
		}

		//Set up the transcode response callback
		s.LivepeerNode.VideoNetwork.ReceivedTranscodeResponse(string(hlsStrmID), func(result map[string]string) {
			//Parse through the results
			for strmID, tProfile := range result {
				vParams := lpmscore.VideoProfileToVariantParams(lpmscore.VideoProfileLookup[tProfile])
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

		glog.Infof("\n\nManifestID: %v\n\n", mid)
		glog.V(common.SHORT).Infof("\n\nhlsStrmID: %v\n\n", hlsStrmID)

		//Remember HLS stream so we can remove later
		s.broadcastRtmpToHLSMap[rtmpStrm.GetStreamID()] = string(hlsStrmID)
		s.broadcastRtmpToManifestMap[rtmpStrm.GetStreamID()] = string(mid)

		if s.LivepeerNode.Eth != nil {
			//Create Transcode Job Onchain
			go s.LivepeerNode.CreateTranscodeJob(hlsStrmID, BroadcastJobVideoProfiles, BroadcastPrice)
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
		//Remove HLS stream from the network - only need to remove the original HLS stream because the other streams in the manifest are not on the current node (they are on the transcoding node)
		s.LivepeerNode.VideoCache.EvictHLSSubscriber(core.StreamID(hlsID))
		if b, err := s.LivepeerNode.VideoNetwork.GetBroadcaster(hlsID); err != nil {
			glog.Errorf("Error getting broadcaster from network: %v", err)
		} else {
			b.Finish()
		}
		//Remove Manifest
		s.LivepeerNode.VideoCache.EvictHLSMasterPlaylist(core.ManifestID(manifestID))
		//Remove the master playlist from the network
		s.LivepeerNode.VideoNetwork.UpdateMasterPlaylist(manifestID, nil)
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
		// glog.Infof("Got req: ", url.Path)
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
				glog.Infof("Inactive HLS Stream %v - unsubscribing", sid)
				s.LivepeerNode.VideoCache.EvictHLSSubscriber(sid)
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
