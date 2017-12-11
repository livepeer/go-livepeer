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
	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/core"
	lpmon "github.com/livepeer/go-livepeer/monitor"
	lpmscore "github.com/livepeer/lpms/core"
	"github.com/livepeer/lpms/segmenter"
	"github.com/livepeer/lpms/stream"
	"github.com/livepeer/lpms/vidplayer"
)

var ErrNotFound = errors.New("ErrNotFound")
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
var BroadcastPrice = uint64(1)
var BroadcastJobVideoProfiles = []lpmscore.VideoProfile{lpmscore.P240p30fps4x3, lpmscore.P360p30fps16x9}
var TranscoderFeeCut = uint8(10)
var TranscoderRewardCut = uint8(10)
var TranscoderSegmentPrice = big.NewInt(150)
var LastHLSStreamID core.StreamID
var LastManifestID core.ManifestID

type LivepeerServer struct {
	RTMPSegmenter lpmscore.RTMPSegmenter
	LPMS          *lpmscore.LPMS
	HttpPort      string
	RtmpPort      string
	FfmpegPath    string
	LivepeerNode  *core.LivepeerNode

	hlsSubTimer                map[core.StreamID]time.Time
	hlsWorkerRunning           bool
	broadcastRtmpToHLSMap      map[string]string
	broadcastRtmpToManifestMap map[string]string
}

func NewLivepeerServer(rtmpPort string, httpPort string, ffmpegPath string, lpNode *core.LivepeerNode) *LivepeerServer {
	server := lpmscore.New(rtmpPort, httpPort, ffmpegPath, "", fmt.Sprintf("%v/.tmp", lpNode.WorkDir))
	return &LivepeerServer{RTMPSegmenter: server, LPMS: server, HttpPort: httpPort, RtmpPort: rtmpPort, FfmpegPath: ffmpegPath, LivepeerNode: lpNode, broadcastRtmpToHLSMap: make(map[string]string), broadcastRtmpToManifestMap: make(map[string]string)}
}

//StartServer starts the LPMS server
func (s *LivepeerServer) StartMediaServer(ctx context.Context, maxPricePerSegment int, transcodingOptions string) error {
	if s.LivepeerNode.Eth != nil {
		BroadcastPrice = uint64(maxPricePerSegment)
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
			//Check Token Balance
			b, err := s.LivepeerNode.Eth.TokenBalance()
			if err != nil {
				glog.Errorf("Error getting token balance:%v", err)
				return ErrBroadcast
			}
			glog.Infof("Current token balance for is: %v", b)

			if b.Cmp(big.NewInt(int64(BroadcastPrice))) < 0 {
				glog.Errorf("Low balance (%v) - cannot start broadcast session", b)
				return ErrBroadcast
			}
		}

		//Check if stream ID already exists
		if s.LivepeerNode.VideoDB.GetRTMPStream(core.StreamID(rtmpStrm.GetStreamID())) != nil {
			return ErrAlreadyExists
		}

		//Add stream to VideoDB
		if err := s.LivepeerNode.VideoDB.AddStream(core.StreamID(rtmpStrm.GetStreamID()), rtmpStrm); err != nil {
			glog.Errorf("Error adding stream to streamDB: %v", err)
			return ErrRTMPPublish
		}

		//We try to automatically determine the video profile from the RTMP stream.
		var vProfile lpmscore.VideoProfile
		resolution := fmt.Sprintf("%v:%v", rtmpStrm.Width(), rtmpStrm.Height())
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

		//Create a new HLS Stream.  If streamID is passed in, use that one.  Otherwise, generate a random ID.
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
		hlsStrm, err := s.LivepeerNode.VideoDB.AddNewHLSStream(hlsStrmID)
		if err != nil {
			glog.Errorf("Error creating HLS stream for segmentation: %v", err)
			return ErrRTMPPublish
		}
		LastHLSStreamID = hlsStrmID

		//Segment the stream (this populates the hls stream)
		go func() {
			err := s.RTMPSegmenter.SegmentRTMPToHLS(context.Background(), rtmpStrm, hlsStrm, SegOptions)
			if err != nil {
				// glog.Infof("Error in segmenter: %v, broadcasting finish message", err)
				if err := s.LivepeerNode.BroadcastFinishMsg(hlsStrmID.String()); err != nil {
					glog.Errorf("Error broadcaseting finish message: %v", err)
				}
			}
		}()

		go func() {
			//Broadcast the hls stream.
			err = s.LivepeerNode.BroadcastStreamToNetwork(hlsStrm)
			if err == core.ErrEOF {
				glog.Info("Broadcast Ended.")
				LastHLSStreamID = ""
			} else if err != nil {
				glog.Errorf("Error broadcasting to network: %v", err)
			}

		}()

		//Create the manifest and broadcast it (so the video can be consumed by itself without transcoding)
		mid, err := core.MakeManifestID(hlsStrmID.GetNodeID(), hlsStrmID.GetVideoID())
		if err != nil {
			glog.Errorf("Error creating manifest id: %v", err)
			return ErrRTMPPublish
		}
		LastManifestID = mid
		manifest, err := s.LivepeerNode.VideoDB.AddNewHLSManifest(mid)
		if err != nil {
			glog.Errorf("Error adding manifest %v - %v", mid, err)
			return ErrRTMPPublish
		}
		vParams := lpmscore.VideoProfileToVariantParams(lpmscore.VideoProfileLookup[vProfile.Name])
		variant := &m3u8.Variant{URI: fmt.Sprintf("%v.m3u8", hlsStrmID), Chunklist: pl, VariantParams: vParams}
		manifest.AddVideoStream(hlsStrm, variant)
		if err = s.LivepeerNode.BroadcastManifestToNetwork(manifest); err != nil {
			glog.Errorf("Error broadasting manifest to network: %v", err)
		}
		glog.Infof("\n\nManifestID: %v\n\n", mid)
		glog.V(common.SHORT).Infof("\n\nhlsStrmID: %v\n\n", mid)

		//Remember HLS stream so we can remove later
		s.broadcastRtmpToHLSMap[rtmpStrm.GetStreamID()] = hlsStrm.GetStreamID()
		s.broadcastRtmpToManifestMap[rtmpStrm.GetStreamID()] = manifest.GetManifestID()

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
		s.LivepeerNode.VideoDB.DeleteStream(core.StreamID(rtmpID))
		//Remove HLS stream associated with the RTMP stream
		s.LivepeerNode.VideoDB.DeleteStream(core.StreamID(hlsID))
		//Remove Manifest
		s.LivepeerNode.VideoDB.DeleteHLSManifest(core.ManifestID(manifestID))
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
			return nil, vidplayer.ErrNotMasterPlaylistID
		}

		//Do we already have the manifest?
		manifest := s.LivepeerNode.VideoDB.GetHLSManifest(manifestID)
		var mpl *m3u8.MasterPlaylist
		if manifest == nil {
			//Return nil if we don't have the playlist locally, and the nodeID is the local node. (No need to go to the network)
			if string(manifestID.GetNodeID()) == s.LivepeerNode.VideoNetwork.GetNodeID() {
				return nil, ErrHLSPlay
			}

			glog.V(common.DEBUG).Infof("Requesting master playlist from network")
			//Get master playlist from the network
			mpl = s.LivepeerNode.GetMasterPlaylistFromNetwork(manifestID)
			if mpl == nil {
				glog.Errorf("Cannot find master playlist")
				return nil, ErrHLSPlay
			}

			//Create local manifest and all of its streams
			manifest, err = s.LivepeerNode.VideoDB.AddNewHLSManifest(manifestID)
			if err != nil {
				glog.Errorf("Cannot create local stream for variant: %v", err)
				return nil, ErrNotFound
			}
			for _, v := range mpl.Variants {
				strmID := core.StreamID(strings.Split(v.URI, ".")[0])
				if !strmID.IsValid() {
					glog.Errorf("Invalid streamID %v from manifest: %v", strmID, manifestID)
					return nil, ErrHLSPlay
				}
				strm := stream.NewBasicHLSVideoStream(strmID.String(), stream.DefaultHLSStreamWin)
				if err := manifest.AddVideoStream(strm, v); err != nil {
					glog.Errorf("Error adding video stream: %v", err)
					return nil, ErrHLSPlay
				}
			}
		} else {
			mpl, err = manifest.GetManifest()
			if err != nil {
				glog.Errorf("Error getting master playlist from hlsStream: %v", err)
				return nil, ErrNotFound
			}
		}
		return mpl, nil
	}
}

func getHLSMediaPlaylistHandler(s *LivepeerServer) func(url *url.URL) (*m3u8.MediaPlaylist, error) {
	return func(url *url.URL) (*m3u8.MediaPlaylist, error) {
		strmID, err := parseStreamID(url.Path)
		if err != nil {
			glog.Errorf("Error parsing for stream id: %v", err)
			return nil, err
		}

		hlsStrm := s.LivepeerNode.VideoDB.GetHLSStream(strmID)
		//Already have it locally.  Just return it.
		if hlsStrm != nil {
			start := time.Now()
			for time.Since(start) < HLSWaitTime {
				pl, err := hlsStrm.GetStreamPlaylist()
				if err != nil {
					glog.Errorf("Error getting stream playlist: %v", err)
					return nil, ErrHLSPlay
				}
				if pl == nil || pl.Count() == 0 {
					time.Sleep(HLSWaitInterval)
				} else {
					return pl, nil
				}
			}
			return nil, ErrNotFound
		}

		//Create a stream, subscribe from the network, add to the videoDB if it actually gets populated
		manifest, err := s.LivepeerNode.VideoDB.GetHLSManifestFromStreamID(strmID)
		if err != nil {
			glog.Errorf("Error getting manifest from videoDB: %v", err)
			return nil, ErrHLSPlay
		}
		hlsStrm, err = manifest.GetVideoStream(strmID.String())
		if err != nil {
			glog.Errorf("Error getting stream from manifest: %v", err)
			return nil, ErrHLSPlay
		}
		if err := s.LivepeerNode.SubscribeFromNetwork(context.Background(), strmID, hlsStrm); err != nil {
			glog.Errorf("Error subscribing from network: %v", err)
			return nil, ErrHLSPlay
		}

		//Wait for the stream to get populated, get the playlist from the stream, and return it.
		start := time.Now()
		for time.Since(start) < HLSWaitTime {
			pl, err := hlsStrm.GetStreamPlaylist()
			if err != nil {
				glog.Errorf("Error getting stream playlist: %v", err)
				return nil, ErrHLSPlay
			}
			if pl == nil || pl.Count() == 0 {
				lpmon.Instance().LogBuffer(strmID.String())
				// glog.Infof("Waiting for playlist... err: %v", err)
				time.Sleep(HLSWaitInterval)
				continue
			} else {
				//Add stream to videoDB
				if err := s.LivepeerNode.VideoDB.AddStream(strmID, hlsStrm); err != nil {
					glog.Errorf("Error adding stream to videoDB: %v", err)
					return nil, ErrHLSPlay
				}
				return pl, nil
			}
		}

		return nil, ErrNotFound
	}
}

func getHLSSegmentHandler(s *LivepeerServer) func(url *url.URL) ([]byte, error) {
	return func(url *url.URL) ([]byte, error) {
		strmID, err := parseStreamID(url.Path)
		if err != nil {
			glog.Errorf("Error parsing streamID: %v", err)
		}

		//Look for stream in VideoDB, if not found return error (should already be there because of the mediaPlaylist request)
		hlsStrm := s.LivepeerNode.VideoDB.GetHLSStream(strmID)
		if hlsStrm == nil {
			return nil, ErrNotFound
		}

		segName := parseSegName(url.Path)
		if segName == "" {
			return nil, ErrNotFound
		}

		start := time.Now()
		for time.Since(start) < HLSWaitTime {
			seg, err := hlsStrm.GetHLSSegment(segName)
			if err != nil {
				if err == stream.ErrNotFound {
					time.Sleep(HLSWaitInterval)
					continue
				} else {
					glog.Errorf("Error getting segment from stream: %v", err)
					return nil, err
				}
			}
			s.hlsSubTimer[strmID] = time.Now()
			return seg.Data, nil
		}
		// glog.Infof("Return data for %v: %v", segName, len(seg.Data))
		return nil, ErrNotFound
	}
}

//End HLS Play Handlers

//Start RTMP Play Handlers
func getRTMPStreamHandler(s *LivepeerServer) func(url *url.URL) (stream.RTMPVideoStream, error) {
	return func(url *url.URL) (stream.RTMPVideoStream, error) {
		// glog.Infof("Got req: ", url.Path)
		//Look for stream in VideoDB
		strmID, err := parseStreamID(url.Path)
		if err != nil {
			glog.Errorf("Error parsing streamID with url %v - %v", url.Path, err)
			return nil, ErrRTMPPlay
		}
		strm := s.LivepeerNode.VideoDB.GetRTMPStream(strmID)
		if strm == nil {
			glog.Errorf("Cannot find RTMP stream")
			return nil, ErrNotFound
		}

		//Could use a subscriber, but not going to here because the RTMP stream doesn't need to be available for consumption by multiple views.  It's only for the segmenter.
		return strm, nil
	}
}

//End RTMP Handlers

//Helper Methods Begin

//HLS video streams need to be timed out.  When the viewer is no longer watching the stream, we send a Unsubscribe message to the network.
func (s *LivepeerServer) startHlsUnsubscribeWorker(limit time.Duration, freq time.Duration) {
	s.hlsWorkerRunning = true
	defer func() { s.hlsWorkerRunning = false }()
	for {
		time.Sleep(freq)
		for sid, t := range s.hlsSubTimer {
			if time.Since(t) > limit {
				glog.Infof("Inactive HLS Stream %v - unsubscribing", sid)
				glog.Infof("\n\nVideoDB before delete: %v", s.LivepeerNode.VideoDB)
				s.LivepeerNode.UnsubscribeFromNetwork(sid)
				s.LivepeerNode.VideoDB.DeleteStream(sid)
				delete(s.hlsSubTimer, sid)
				glog.Infof("\n\nVideoDB after delete: %v", s.LivepeerNode.VideoDB)
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
		return "", ErrNotFound
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
		return "", ErrNotFound
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

// func createBroadcastJob(s *LivepeerServer, hlsStrm stream.HLSVideoStream) {
// 	eth.CheckRoundAndInit(s.LivepeerNode.Eth)

// 	pNames := []string{}
// 	for _, prof := range BroadcastJobVideoProfiles {
// 		pNames = append(pNames, prof.Name)
// 	}
// 	resCh, errCh := s.LivepeerNode.Eth.Job(hlsStrm.GetStreamID(), strings.Join(pNames, ","), BroadcastPrice)
// 	select {
// 	case <-resCh:
// 		glog.Infof("Created broadcast job. Price: %v. Type: %v", BroadcastPrice, pNames)
// 	case err := <-errCh:
// 		glog.Errorf("Error creating broadcast job: %v", err)
// 	}
// }
