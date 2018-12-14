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
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/livepeer/go-livepeer/drivers"
	"github.com/livepeer/go-livepeer/monitor"
	"github.com/livepeer/go-livepeer/net"

	"github.com/cenkalti/backoff"
	"github.com/ericxtang/m3u8"
	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/core"
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
var ErrStorage = errors.New("ErrStorage")
var ErrDiscovery = errors.New("ErrDiscovery")

const HLSWaitInterval = time.Second
const HLSBufferCap = uint(43200) //12 hrs assuming 1s segment
const HLSBufferWindow = uint(5)

const SegLen = 4 * time.Second
const HLSUnsubWorkerFreq = time.Second * 5
const BroadcastRetry = 15 * time.Second

var HLSWaitTime = time.Second * 45
var BroadcastPrice = big.NewInt(1)
var BroadcastJobVideoProfiles = []ffmpeg.VideoProfile{ffmpeg.P240p30fps4x3, ffmpeg.P360p30fps16x9}
var MinDepositSegmentCount = int64(75)     // 5 mins assuming 4s segments
var MinJobBlocksRemaining = big.NewInt(40) // 10 mins assuming 15s blocks
var LastHLSStreamID core.StreamID
var LastManifestID core.ManifestID

type LivepeerServer struct {
	RTMPSegmenter   lpmscore.RTMPSegmenter
	LPMS            *lpmscore.LPMS
	LivepeerNode    *core.LivepeerNode
	VideoNonce      map[string]uint64
	VideoNonceLock  *sync.Mutex
	HttpMux         *http.ServeMux
	CurrentPlaylist core.PlaylistManager

	ExposeCurrentManifest bool

	rtmpStreams                map[core.StreamID]stream.RTMPVideoStream
	broadcastRtmpToManifestMap map[string]string
}

func NewLivepeerServer(rtmpAddr string, httpAddr string, lpNode *core.LivepeerNode) *LivepeerServer {
	opts := lpmscore.LPMSOpts{
		RtmpAddr: rtmpAddr, RtmpDisabled: true,
		HttpAddr: httpAddr,
		WorkDir:  lpNode.WorkDir,
	}
	switch lpNode.NodeType {
	case core.BroadcasterNode:
		opts.RtmpDisabled = false
	case core.OrchestratorNode:
		opts.HttpMux = http.NewServeMux()
	}
	server := lpmscore.New(&opts)
	return &LivepeerServer{RTMPSegmenter: server, LPMS: server, LivepeerNode: lpNode, VideoNonce: map[string]uint64{}, VideoNonceLock: &sync.Mutex{}, HttpMux: opts.HttpMux, rtmpStreams: make(map[core.StreamID]stream.RTMPVideoStream), broadcastRtmpToManifestMap: make(map[string]string)}
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

	//LPMS handlers for handling RTMP video
	s.LPMS.HandleRTMPPublish(createRTMPStreamIDHandler(s), gotRTMPStreamHandler(s), endRTMPStreamHandler(s))
	s.LPMS.HandleRTMPPlay(getRTMPStreamHandler(s))

	//LPMS hanlder for handling HLS video play
	s.LPMS.HandleHLSPlay(getHLSMasterPlaylistHandler(s), getHLSMediaPlaylistHandler(s), getHLSSegmentHandler(s))

	//Start the LPMS server
	lpmsCtx, cancel := context.WithCancel(context.Background())
	ec := make(chan error, 1)
	go func() {
		if err := s.LPMS.Start(lpmsCtx); err != nil {
			// typically triggered if there's an error with broadcaster LPMS
			// transcoder LPMS should return without an error
			ec <- s.LPMS.Start(lpmsCtx)
		}
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
		id, err := core.MakeStreamID(core.RandomVideoID(), "RTMP")
		if err != nil {
			glog.Errorf("Error making stream ID")
			return ""
		}
		return id.String()
	}
}

func (s *LivepeerServer) startBroadcast(cpl core.PlaylistManager) (*BroadcastSession, error) {

	if s.LivepeerNode.OrchestratorPool == nil {
		glog.Info("No orchestrators specified; not transcoding")
		return nil, ErrDiscovery
	}

	rpcBcast := core.NewBroadcaster(s.LivepeerNode)

	tinfos, err := s.LivepeerNode.OrchestratorPool.GetOrchestrators(1)
	if len(tinfos) <= 0 || err != nil {
		return nil, err
	}
	tinfo := tinfos[0]

	// set OSes
	var orchOS drivers.OSSession
	if len(tinfo.Storage) > 0 {
		orchOS = drivers.NewSession(tinfo.Storage[0])
	}

	return &BroadcastSession{
		Broadcaster:      rpcBcast,
		ManifestID:       cpl.ManifestID(),
		Profiles:         BroadcastJobVideoProfiles,
		OrchestratorInfo: tinfo,
		OrchestratorOS:   orchOS,
		BroadcasterOS:    cpl.GetOSSession(),
	}, nil
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

		startSeq := 0
		var sess *BroadcastSession
		var cpl core.PlaylistManager

		if s.LivepeerNode.Eth != nil {

			//Check if round is initialized
			initialized, err := s.LivepeerNode.Eth.CurrentRoundInitialized()
			if err != nil {
				glog.Errorf("Could not check whether round was initialized: %v", err)
				return err
			}
			if !initialized {
				glog.Infof("Round was uninitialized. Please try again in a few blocks.")
				// todo send to metrics ?
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
				glog.Errorf("Low deposit (%v) - cannot start broadcast session.  Need at least %v", deposit, minDeposit)
				if monitor.Enabled {
					monitor.LogStreamCreateFailed(nonce, "LowDeposit")
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

		//Create a HLS StreamID
		//If streamID is passed in, use that one
		//Else generate a random ID
		hlsStrmID := core.StreamID(url.Query().Get("hlsStrmID"))
		if hlsStrmID == "" {
			hlsStrmID, err = core.MakeStreamID(core.RandomVideoID(), vProfile.Name)
			if err != nil {
				glog.Errorf("Error making stream ID")
				return ErrRTMPPublish
			}
		}

		LastHLSStreamID = hlsStrmID

		streamStarted := false
		//Segment the stream, insert the segments into the broadcaster
		go func(rtmpStrm stream.RTMPVideoStream) {
			hlsStrm := stream.NewBasicHLSVideoStream(string(hlsStrmID), stream.DefaultHLSStreamWin)
			hlsStrm.SetSubscriber(func(seg *stream.HLSSegment, eof bool) {
				if eof {
					// XXX update HLS manifest
					return
				}

				if cpl == nil {
					return
				}

				if streamStarted == false {
					streamStarted = true
					if monitor.Enabled {
						monitor.LogStreamStartedEvent(nonce)
					}
				}
				if monitor.Enabled {
					monitor.LogSegmentEmerged(nonce, seg.SeqNo)
				}

				seg.Name = "" // hijack seg.Name to convey the uploaded URI
				name := fmt.Sprintf("%s/%d.ts", vProfile.Name, seg.SeqNo)
				uri, err := cpl.GetOSSession().SaveData(name, seg.Data)
				if err != nil {
					glog.Errorf("Error saving segment %d: %v", seg.SeqNo, err)
					if monitor.Enabled {
						monitor.LogSegmentUploadFailed(nonce, seg.SeqNo, err.Error())
					}
					return
				}
				if cpl.GetOSSession().IsExternal() {
					seg.Name = uri // hijack seg.Name to convey the uploaded URI
				}
				err = cpl.InsertHLSSegment(hlsStrmID, seg.SeqNo, uri, seg.Duration)
				if err != nil {
					glog.Errorf("Error inserting segment %d: %v", seg.SeqNo, err)
					if monitor.Enabled {
						monitor.LogSegmentUploadFailed(nonce, seg.SeqNo, err.Error())
					}
				}

				if sess != nil {
					go func() {
						// storage the orchestrator prefers
						if ios := sess.OrchestratorOS; ios != nil {
							// XXX handle case when orch expects direct upload
							uri, err := ios.SaveData(name, seg.Data)
							if err != nil {
								glog.Error("Error saving segment to OS ", err)
								if monitor.Enabled {
									monitor.LogSegmentUploadFailed(nonce, seg.SeqNo, err.Error())
								}
								return
							}
							seg.Name = uri // hijack seg.Name to convey the uploaded URI
						}

						// send segment to the orchestrator
						glog.V(common.DEBUG).Infof("Submitting segment %d", seg.SeqNo)

						res, err := SubmitSegment(sess, seg, nonce)
						if err != nil {
							if shouldStopStream(err) {
								glog.Warningf("Stopping current stream due to: %v", err)
								rtmpStrm.Close()
							}
							return
						}

						// download transcoded segments from the transcoder
						gotErr := false // only send one error msg per segment list
						errFunc := func(subType, url string, err error) {
							glog.Errorf("%v error with segment %v: %v (URL: %v)", subType, seg.SeqNo, err, url)
							if monitor.Enabled && !gotErr {
								monitor.LogSegmentTranscodeFailed(subType, nonce, seg.SeqNo, err)
								gotErr = true
							}
						}

						segHashes := make([][]byte, len(res.Segments))
						n := len(res.Segments)
						SegHashLock := &sync.Mutex{}
						cond := sync.NewCond(SegHashLock)

						dlFunc := func(url string, i int) {
							defer func() {
								cond.L.Lock()
								n--
								if n == 0 {
									cond.Signal()
								}
								cond.L.Unlock()
							}()

							if bos := sess.BroadcasterOS; bos != nil && !drivers.IsOwnExternal(url) {
								data, err := drivers.GetSegmentData(url)
								if err != nil {
									errFunc("Download", url, err)
									return
								}
								name := fmt.Sprintf("%s/%d.ts", sess.Profiles[i].Name, seg.SeqNo)
								newUrl, err := bos.SaveData(name, data)
								if err != nil {
									errFunc("SaveData", url, err)
									return
								}
								url = newUrl

								hash := crypto.Keccak256(data)
								SegHashLock.Lock()
								segHashes[i] = hash
								SegHashLock.Unlock()
							}

							// hoist this out so we only do it once
							sid, _ := core.MakeStreamID(hlsStrmID.GetVideoID(), sess.Profiles[i].Name)
							err = cpl.InsertHLSSegment(sid, seg.SeqNo, url, seg.Duration)
							if err != nil {
								errFunc("Playlist", url, err)
								return
							}
						}

						for i, v := range res.Segments {
							go dlFunc(v.Url, i)
						}

						cond.L.Lock()
						for n != 0 {
							cond.Wait()
						}
						cond.L.Unlock()

						// if !eth.VerifySig(transcoderAddress, crypto.Keccak256(segHashes...), res.Sig) { // need transcoder address here
						// 	glog.Error("Sig check failed for segment ", seg.SeqNo)
						// 	return
						// }

						glog.V(common.DEBUG).Info("Successfully validated segment ", seg.SeqNo)
					}()
				}
			})

			segOptions := segmenter.SegmenterOptions{
				StartSeq:  startSeq,
				SegLength: SegLen,
			}
			err := s.RTMPSegmenter.SegmentRTMPToHLS(context.Background(), rtmpStrm, hlsStrm, segOptions)
			if err != nil {
				// Stop the incoming RTMP connection.
				// TODO retry segmentation if err != SegmenterTimeout; may be recoverable
				rtmpStrm.Close()
			}

		}(rtmpStrm)

		//Create the manifest and broadcast it (so the video can be consumed by itself without transcoding)
		mid, err := core.MakeManifestID(hlsStrmID.GetVideoID())
		if err != nil {
			glog.Errorf("Error creating manifest id: %v", err)
			return ErrRTMPPublish
		}
		LastManifestID = mid

		if drivers.NodeStorage == nil {
			glog.Error("Missing node storage")
			return ErrStorage
		}
		storage := drivers.NodeStorage.NewSession(string(mid))
		if s.CurrentPlaylist != nil {
			s.CurrentPlaylist.Cleanup()
		}
		s.CurrentPlaylist = core.NewBasicPlaylistManager(mid, storage)
		cpl = s.CurrentPlaylist

		if monitor.Enabled {
			monitor.LogStreamCreatedEvent(hlsStrmID.String(), nonce)
		}

		glog.Infof("\n\nVideo Created With ManifestID: %v\n\n", mid)
		glog.V(common.SHORT).Infof("\n\nhlsStrmID: %v\n\n", hlsStrmID)

		s.broadcastRtmpToManifestMap[rtmpStrm.GetStreamID()] = string(mid)

		//Create Transcode Job Onchain
		go func() {
			// Connect to the orchestrator. If it fails, retry for as long
			// as the RTMP stream is alive; maybe the orchestrator hasn't
			// received the block containing the job yet
			broadcastFunc := func() error {
				sess, err = s.startBroadcast(cpl)
				if err == ErrDiscovery {
					return nil // discovery disabled, don't retry
				} else if err != nil {
					// Should be logged upstream
				}
				s.VideoNonceLock.Lock()
				_, active := s.VideoNonce[rtmpStrm.GetStreamID()]
				s.VideoNonceLock.Unlock()
				if active {
					return err
				}
				return nil // stop if inactive
			}
			expb := backoff.NewExponentialBackOff()
			expb.MaxInterval = BroadcastRetry
			expb.MaxElapsedTime = 0
			backoff.Retry(broadcastFunc, expb)
		}()
		return nil
	}
}

func endRTMPStreamHandler(s *LivepeerServer) func(url *url.URL, rtmpStrm stream.RTMPVideoStream) error {
	return func(url *url.URL, rtmpStrm stream.RTMPVideoStream) error {
		rtmpID := rtmpStrm.GetStreamID()
		//Remove RTMP stream
		delete(s.rtmpStreams, core.StreamID(rtmpID))
		if s.CurrentPlaylist != nil {
			s.CurrentPlaylist.Cleanup()
			s.CurrentPlaylist = nil
		}

		s.VideoNonceLock.Lock()
		if _, ok := s.VideoNonce[rtmpStrm.GetStreamID()]; ok {
			if monitor.Enabled {
				monitor.LogStreamEndedEvent(s.VideoNonce[rtmpStrm.GetStreamID()])
			}
			delete(s.VideoNonce, rtmpStrm.GetStreamID())
		}
		s.VideoNonceLock.Unlock()
		return nil
	}
}

//End RTMP Publish Handlers

//HLS Play Handlers
func getHLSMasterPlaylistHandler(s *LivepeerServer) func(url *url.URL) (*m3u8.MasterPlaylist, error) {
	return func(url *url.URL) (*m3u8.MasterPlaylist, error) {
		cpl := s.CurrentPlaylist
		if cpl == nil {
			return nil, vidplayer.ErrNotFound
		}
		var manifestID core.ManifestID
		if s.ExposeCurrentManifest && "/stream/current.m3u8" == strings.ToLower(url.Path) {
			manifestID = LastManifestID
		} else {
			var err error
			if manifestID, err = parseManifestID(url.Path); err != nil {
				return nil, vidplayer.ErrNotFound
			}
		}

		if cpl.ManifestID() != manifestID {
			return nil, vidplayer.ErrNotFound
		}
		return cpl.GetHLSMasterPlaylist(), nil
	}
}

func getHLSMediaPlaylistHandler(s *LivepeerServer) func(url *url.URL) (*m3u8.MediaPlaylist, error) {
	return func(url *url.URL) (*m3u8.MediaPlaylist, error) {
		strmID, err := parseStreamID(url.Path)
		if err != nil {
			glog.Errorf("Error parsing for stream id: %v", err)
			return nil, err
		}

		cpl := s.CurrentPlaylist
		if cpl == nil {
			return nil, vidplayer.ErrNotFound
		}
		//Get the hls playlist
		pl := cpl.GetHLSMediaPlaylist(strmID)
		if pl == nil {
			return nil, vidplayer.ErrNotFound
		}
		return pl, nil
	}
}

func getHLSSegmentHandler(s *LivepeerServer) func(url *url.URL) ([]byte, error) {
	return func(url *url.URL) ([]byte, error) {
		// Strip the /stream/ prefix
		segName := parseSegName(url.Path)
		if segName == "" || drivers.NodeStorage == nil {
			glog.Error("SegName not found or storage nil")
			return nil, vidplayer.ErrNotFound
		}
		parts := strings.SplitN(segName, "/", 2)
		if len(parts) <= 0 {
			glog.Error("Unexpected path structure")
			return nil, vidplayer.ErrNotFound
		}
		memoryOS, ok := drivers.NodeStorage.(*drivers.MemoryOS)
		if !ok {
			return nil, vidplayer.ErrNotFound
		}
		// We index the session by the first entry of the path, eg
		// <session>/<more-path>/<data>
		os := memoryOS.GetSession(parts[0])
		if os == nil {
			return nil, vidplayer.ErrNotFound
		}
		data := os.GetData(segName)
		if len(data) > 0 {
			return data, nil
		}
		return nil, vidplayer.ErrNotFound
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

func (s *LivepeerServer) GetNodeStatus() *net.NodeStatus {
	// not threadsafe; need to deep copy the playlist
	m := make(map[string]*m3u8.MasterPlaylist, 0)
	cpl := s.CurrentPlaylist
	if cpl != nil {
		m[string(cpl.ManifestID())] = cpl.GetHLSMasterPlaylist()
	}
	return &net.NodeStatus{Manifests: m}
}

func shouldStopStream(err error) bool {
	return false
}
