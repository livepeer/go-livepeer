/*
Package server is the place we integrate the Livepeer node with the LPMS media server.
*/
package server

import (
	"context"
	"errors"
	// "fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"math/big"

	"github.com/ericxtang/m3u8"
	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/eth"
	lpmon "github.com/livepeer/go-livepeer/monitor"
	"github.com/livepeer/go-livepeer/types"
	lpmscore "github.com/livepeer/lpms/core"
	"github.com/livepeer/lpms/segmenter"
	"github.com/livepeer/lpms/stream"
)

var ErrNotFound = errors.New("ErrNotFound")
var ErrAlreadyExists = errors.New("StreamAlreadyExists")
var ErrRTMPPublish = errors.New("ErrRTMPPublish")
var ErrBroadcast = errors.New("ErrBroadcast")

const HLSWaitTime = time.Second * 45
const HLSBufferCap = uint(43200) //12 hrs assuming 1s segment
const HLSBufferWindow = uint(5)

var SegOptions = segmenter.SegmenterOptions{SegLength: 8 * time.Second}

const HLSUnsubWorkerFreq = time.Second * 5

const EthRpcTimeout = 5 * time.Second
const EthEventTimeout = 5 * time.Second
const EthMinedTxTimeout = 60 * time.Second

var BroadcastPrice = big.NewInt(150)
var BroadcastJobVideoProfile = types.P240p30fps4x3
var TranscoderFeeCut = uint8(10)
var TranscoderRewardCut = uint8(10)
var TranscoderSegmentPrice = big.NewInt(150)
var LastHLSStreamID core.StreamID

type LivepeerServer struct {
	RTMPSegmenter lpmscore.RTMPSegmenter
	LPMS          *lpmscore.LPMS
	HttpPort      string
	RtmpPort      string
	FfmpegPath    string
	LivepeerNode  *core.LivepeerNode

	hlsSubTimer           map[core.StreamID]time.Time
	hlsWorkerRunning      bool
	broadcastRtmpToHLSMap map[string]string
}

func NewLivepeerServer(rtmpPort string, httpPort string, ffmpegPath string, lpNode *core.LivepeerNode) *LivepeerServer {
	server := lpmscore.New(rtmpPort, httpPort, ffmpegPath, "")
	return &LivepeerServer{RTMPSegmenter: server, LPMS: server, HttpPort: httpPort, RtmpPort: rtmpPort, FfmpegPath: ffmpegPath, LivepeerNode: lpNode}
}

//StartServer starts the LPMS server
func (s *LivepeerServer) StartMediaServer(ctx context.Context, maxPricePerSegment int, transcodingOptions string) error {
	if s.LivepeerNode.Eth != nil {
		glog.Infof("Transcode Job Price: %v, Transcode Job Type: %v", maxPricePerSegment, transcodingOptions)
	}

	//Start HLS unsubscribe worker
	s.hlsSubTimer = make(map[core.StreamID]time.Time)
	go s.startHlsUnsubscribeWorker(time.Second*30, HLSUnsubWorkerFreq)

	s.broadcastRtmpToHLSMap = make(map[string]string)

	//LPMS handlers for handling RTMP video
	s.LPMS.HandleRTMPPublish(createRTMPStreamIDHandler(s), gotRTMPStreamHandler(s, maxPricePerSegment, transcodingOptions), endRTMPStreamHandler(s))
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
		id, err := core.MakeStreamID(s.LivepeerNode.Identity, core.RandomVideoID(), "")
		if err != nil {
			glog.Errorf("Error making stream ID")
			return ""
		}
		return id.String()
	}
}

func gotRTMPStreamHandler(s *LivepeerServer, maxPricePerSegment int, transcodingOptions string) func(url *url.URL, rtmpStrm stream.RTMPVideoStream) (err error) {
	return func(url *url.URL, rtmpStrm stream.RTMPVideoStream) (err error) {
		if s.LivepeerNode.Eth != nil {
			//Check Token Balance
			b, err := s.LivepeerNode.Eth.TokenBalance()
			if err != nil {
				glog.Errorf("Error getting token balance:%v", err)
				return ErrBroadcast
			}
			glog.Infof("Current token balance for is: %v", b)

			if b.Cmp(BroadcastPrice) < 0 {
				glog.Errorf("Low balance (%v) - cannot start broadcast session", b)
				return ErrBroadcast
			}
		}

		//Check if stream ID already exists
		if s.LivepeerNode.StreamDB.GetRTMPStream(core.StreamID(rtmpStrm.GetStreamID())) != nil {
			return ErrAlreadyExists
		}

		//Add stream to StreamDB
		if err := s.LivepeerNode.StreamDB.AddStream(core.StreamID(rtmpStrm.GetStreamID()), rtmpStrm); err != nil {
			glog.Errorf("Error adding stream to streamDB: %v", err)
			return ErrRTMPPublish
		}

		//Create a new HLS Stream.  If streamID is passed in, use that one.  Otherwise, generate a random ID.
		hlsStrmID := core.StreamID(url.Query().Get("hlsStrmID"))
		if hlsStrmID == "" {
			hlsStrmID, err = core.MakeStreamID(s.LivepeerNode.Identity, core.RandomVideoID(), "")
			if err != nil {
				glog.Errorf("Error making stream ID")
				return ErrRTMPPublish
			}
		}
		if hlsStrmID.GetNodeID() != s.LivepeerNode.Identity {
			glog.Errorf("Cannot create a HLS stream with Node ID: %v", hlsStrmID.GetNodeID())
			return ErrRTMPPublish
		}
		hlsStrm, err := s.LivepeerNode.StreamDB.AddNewHLSStream(hlsStrmID)

		if err != nil {
			glog.Errorf("Error creating HLS stream for segmentation: %v", err)
		}
		LastHLSStreamID = hlsStrmID

		//Create Segmenter
		glog.Infof("\n\nSegmenting rtmp stream:\n%v \nto hls stream:\n%v\n\n", rtmpStrm.GetStreamID(), hlsStrm.GetStreamID())
		go func() {
			err := s.RTMPSegmenter.SegmentRTMPToHLS(context.Background(), rtmpStrm, hlsStrm, SegOptions) //TODO: do we need to cancel this thread when the stream finishes?
			if err != nil {
				glog.Infof("Error in segmenter: %v, broadcasting finish message", err)
				if err := s.LivepeerNode.BroadcastFinishMsg(hlsStrmID.String()); err != nil {
					glog.Errorf("Error broadcaseting finish message: %v", err)
				}
			}
		}()

		//Kick off go routine to broadcast the hls stream.
		go func() {
			// glog.Infof("Kicking off broadcaster")
			mpl, err := hlsStrm.GetMasterPlaylist()
			if err != nil {
				glog.Errorf("Error getting master playlist: %v", err)
			}
			if err = s.LivepeerNode.VideoNetwork.UpdateMasterPlaylist(hlsStrm.GetStreamID(), mpl); err != nil {
				glog.Errorf("Error updating master playlist: %v", err)
			}

			err = s.LivepeerNode.BroadcastToNetwork(hlsStrm)
			if err == core.ErrEOF {
				glog.Info("Broadcast Ended.")
				LastHLSStreamID = ""
			} else if err != nil {
				glog.Errorf("Error broadcasting to network: %v", err)
			}
		}()

		//Store HLS Stream into StreamDB, remember HLS stream so we can remove later
		if err = s.LivepeerNode.StreamDB.AddStream(core.StreamID(hlsStrm.GetStreamID()), hlsStrm); err != nil {
			glog.Errorf("Error adding stream to streamDB: %v", err)
		}
		s.broadcastRtmpToHLSMap[rtmpStrm.GetStreamID()] = hlsStrm.GetStreamID()

		if s.LivepeerNode.Eth != nil {
			//Create Transcode Job Onchain
			go createBroadcastJob(s, hlsStrm, maxPricePerSegment, transcodingOptions)
		}
		return nil
	}
}

func endRTMPStreamHandler(s *LivepeerServer) func(url *url.URL, rtmpStrm stream.RTMPVideoStream) error {
	return func(url *url.URL, rtmpStrm stream.RTMPVideoStream) error {
		//Remove RTMP stream
		s.LivepeerNode.StreamDB.DeleteStream(core.StreamID(rtmpStrm.GetStreamID()))
		//Remove HLS stream associated with the RTMP stream
		s.LivepeerNode.StreamDB.DeleteStream(core.StreamID(s.broadcastRtmpToHLSMap[rtmpStrm.GetStreamID()]))

		return nil
	}
}

//End RTMP Publish Handlers

//HLS Play Handlers
func getHLSMasterPlaylistHandler(s *LivepeerServer) func(url *url.URL) (*m3u8.MasterPlaylist, error) {
	return func(url *url.URL) (*m3u8.MasterPlaylist, error) {
		strmID := parseStreamID(url.Path)

		//Get master playlist from broadcaster
		hlsStrm := s.LivepeerNode.StreamDB.GetHLSStream(strmID)
		var mpl *m3u8.MasterPlaylist
		var err error
		if hlsStrm == nil {
			//Get master playlist from the network
			mpl = s.LivepeerNode.GetMasterPlaylistFromNetwork(strmID)
			if mpl == nil {
				glog.Errorf("Cannot find master playlist")
				return nil, nil //not returning an error here because it could be a media playlist request
			}

			//Create local stream and all of its variants
			hlsStrm, err = s.LivepeerNode.StreamDB.AddNewHLSStream(strmID)
			if err != nil {
				glog.Errorf("Cannot create local stream: %v", err)
				return nil, ErrNotFound
			}
		} else {
			mpl, err = hlsStrm.GetMasterPlaylist()
			if err != nil {
				glog.Errorf("Error getting master playlist from hlsStream: %v", err)
				return nil, ErrNotFound
			}
		}

		//Make sure the variants are in the streamDB
		for _, v := range mpl.Variants {
			vName := strings.Split(v.URI, ".")[0]
			//Need to create local media playlist because it has local state.
			pl, _ := m3u8.NewMediaPlaylist(stream.DefaultMediaPlLen, stream.DefaultMediaPlLen)
			v.Chunklist = pl
			// glog.Infof("Adding variant %v to %v", v, hlsStrm.GetStreamID())
			if s.LivepeerNode.StreamDB.GetHLSStream(core.StreamID(vName)) == nil {
				if err = hlsStrm.AddVariant(strings.Split(v.URI, ".")[0], v); err != nil {
					glog.Errorf("Error adding variant: %v", err)
				}
				if err = s.LivepeerNode.StreamDB.AddStream(core.StreamID(vName), hlsStrm); err != nil {
					glog.Errorf("Error adding variant stream to StreamDB: %v", vName)
				}
			}
		}
		// glog.Infof("hlsStrm: %v", hlsStrm)

		//If the strmID is not the hls streamID, it means it's a variant stream ID.  Return nil.
		if hlsStrm.GetStreamID() != strmID.String() {
			return nil, nil
		}
		return hlsStrm.GetMasterPlaylist()
	}
}

func getHLSMediaPlaylistHandler(s *LivepeerServer) func(url *url.URL) (*m3u8.MediaPlaylist, error) {
	return func(url *url.URL) (*m3u8.MediaPlaylist, error) {
		strmID := parseStreamID(url.Path)

		hlsStrm := s.LivepeerNode.StreamDB.GetHLSStream(strmID)
		if hlsStrm == nil {
			glog.Errorf("Cannot find stream: %v", strmID)
			return nil, ErrNotFound
		}

		//Subscribe from the network if we can't find a subscriber
		sub, err := s.LivepeerNode.VideoNetwork.GetSubscriber(strmID.String())
		if err != nil {
			glog.Errorf("Error getting subscriber for %v: %v", strmID, err)
			return nil, err
		}
		if !sub.IsWorking() {
			glog.Infof("Making new subscriber for %v", strmID)
			if _, err := s.LivepeerNode.SubscribeFromNetwork(context.Background(), strmID, hlsStrm); err != nil {
				glog.Errorf("Error subscribing from network: %v", err)
				return nil, err
			}
		}

		//Wait for the stream to get populated, get the playlist from the stream, and return it.
		//Also update the hlsSubTimer.
		start := time.Now()
		for time.Since(start) < HLSWaitTime {
			pl, err := hlsStrm.GetVariantPlaylist(strmID.String())
			// glog.Infof("pl: %v", pl)
			if err != nil || pl == nil || pl.Segments[0] == nil || pl.Segments[0].URI == "" {
				if err == stream.ErrEOF {
					return nil, err
				}

				lpmon.Instance().LogBuffer(strmID.String())
				// glog.Infof("Waiting for playlist... err: %v", err)
				time.Sleep(2 * time.Second)

				continue
			} else {
				// glog.Infof("Found playlist. Returning")
				s.hlsSubTimer[strmID] = time.Now()
				return pl, err
			}
		}

		return nil, ErrNotFound
	}
}

func getHLSSegmentHandler(s *LivepeerServer) func(url *url.URL) ([]byte, error) {
	return func(url *url.URL) ([]byte, error) {
		strmID := parseStreamID(url.Path)

		//Look for stream in StreamDB, if not found return error (should already be there because of the mediaPlaylist request)
		hlsStrm := s.LivepeerNode.StreamDB.GetHLSStream(strmID)
		if hlsStrm == nil {
			return nil, ErrNotFound
		}

		segName := parseSegName(url.Path)
		if segName == "" {
			return nil, ErrNotFound
		}

		seg, err := hlsStrm.GetHLSSegment(strmID.String(), segName)
		// glog.Infof("Return data for %v: %v", segName, len(seg.Data))
		if err != nil {
			glog.Errorf("Error getting segment from stream: %v", err)
			return nil, err
		}
		return seg.Data, nil
	}
}

//End HLS Play Handlers

//Start RTMP Play Handlers
func getRTMPStreamHandler(s *LivepeerServer) func(url *url.URL) (stream.RTMPVideoStream, error) {
	return func(url *url.URL) (stream.RTMPVideoStream, error) {
		// glog.Infof("Got req: ", url.Path)
		//Look for stream in StreamDB
		strmID := parseStreamID(url.Path)
		strm := s.LivepeerNode.StreamDB.GetRTMPStream(strmID)
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
				glog.Infof("HLS Stream %v inactive - unsubscribing", sid)
				s.LivepeerNode.UnsubscribeFromNetwork(sid)

				// s.LivepeerNode.StreamDB.DeleteSubscriber(sid)
				// s.LivepeerNode.StreamDB.DeleteStream(sid) - don't delete stream because we still need it in case other renditions are being used.
				delete(s.hlsSubTimer, sid)
			}
		}
	}
}

func parseStreamID(reqPath string) core.StreamID {
	var strmID string
	regex, _ := regexp.Compile("\\/stream\\/([[:alpha:]]|\\d)*")
	match := regex.FindString(reqPath)
	if match != "" {
		strmID = strings.Replace(match, "/stream/", "", -1)
	}
	return core.StreamID(strmID)
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

func createBroadcastJob(s *LivepeerServer, hlsStrm stream.HLSVideoStream, maxPricePerSegment int, transcodingOptions string) {
	eth.CheckRoundAndInit(s.LivepeerNode.Eth)

	resCh, errCh := s.LivepeerNode.Eth.Job(hlsStrm.GetStreamID(), transcodingOptions, big.NewInt(int64(maxPricePerSegment)))
	select {
	case <-resCh:
		glog.Infof("Created broadcast job. Price: %v. Type: %v", maxPricePerSegment, transcodingOptions)
	case err := <-errCh:
		glog.Errorf("Error creating broadcast job: %v", err)
	}
}
