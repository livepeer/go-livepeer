/*
Package server is the place we integrate the Livepeer node with the LPMS media server.
*/
package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math/big"
	"math/rand"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"path"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/livepeer/go-livepeer/clog"
	"github.com/livepeer/go-livepeer/monitor"
	"github.com/livepeer/go-livepeer/pm"
	"github.com/livepeer/go-tools/drivers"

	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/core"
	lpmscore "github.com/livepeer/lpms/core"
	ffmpeg "github.com/livepeer/lpms/ffmpeg"
	"github.com/livepeer/lpms/segmenter"
	"github.com/livepeer/lpms/stream"
	"github.com/livepeer/lpms/vidplayer"
	"github.com/livepeer/m3u8"
	"github.com/patrickmn/go-cache"
)

var (
	errAlreadyExists    = errors.New("StreamAlreadyExists")
	errStorage          = errors.New("ErrStorage")
	errDiscovery        = errors.New("ErrDiscovery")
	errNoOrchs          = errors.New("ErrNoOrchs")
	errUnknownStream    = errors.New("ErrUnknownStream")
	errMismatchedParams = errors.New("Mismatched type for stream params")
	errForbidden        = errors.New("authentication denied")
)

const HLSWaitInterval = time.Second
const HLSBufferCap = uint(43200) //12 hrs assuming 1s segment
const HLSBufferWindow = uint(5)
const StreamKeyBytes = 6

const SegLen = 2 * time.Second
const BroadcastRetry = 15 * time.Second

var BroadcastJobVideoProfiles = []ffmpeg.VideoProfile{ffmpeg.P240p30fps4x3, ffmpeg.P360p30fps16x9}

var AuthWebhookURL *url.URL

func PixelFormatNone() ffmpeg.PixelFormat {
	return ffmpeg.PixelFormat{RawValue: ffmpeg.PixelFormatNone}
}

// For HTTP push watchdog
var httpPushTimeout = 60 * time.Second
var httpPushResetTimer = func() (context.Context, context.CancelFunc) {
	sleepDur := time.Duration(int64(float64(httpPushTimeout) * 0.9))
	return context.WithTimeout(context.Background(), sleepDur)
}

type rtmpConnection struct {
	initializing    chan struct{}
	mid             core.ManifestID
	nonce           uint64
	stream          stream.RTMPVideoStream
	pl              core.PlaylistManager
	profile         *ffmpeg.VideoProfile
	params          *core.StreamParameters
	sessManager     *BroadcastSessionsManager
	lastUsed        time.Time
	sourceBytes     uint64
	transcodedBytes uint64
	mu              sync.Mutex
	mediaFormat     ffmpeg.MediaFormatInfo
}

func (s *LivepeerServer) getActiveRtmpConnectionUnsafe(mid core.ManifestID) (*rtmpConnection, bool) {
	cxn, exists := s.rtmpConnections[mid]
	if exists {
		if cxn.initializing != nil {
			<-cxn.initializing
		}
	}
	return cxn, exists
}

type LivepeerServer struct {
	RTMPSegmenter           lpmscore.RTMPSegmenter
	LPMS                    *lpmscore.LPMS
	LivepeerNode            *core.LivepeerNode
	HTTPMux                 *http.ServeMux
	ExposeCurrentManifest   bool
	recordingsAuthResponses *cache.Cache

	// Thread sensitive fields. All accesses to the
	// following fields should be protected by `connectionLock`
	rtmpConnections   map[core.ManifestID]*rtmpConnection
	internalManifests map[core.ManifestID]core.ManifestID
	lastHLSStreamID   core.StreamID
	lastManifestID    core.ManifestID
	context           context.Context
	connectionLock    *sync.RWMutex
	serverLock        *sync.RWMutex
}

func (s *LivepeerServer) SetContextFromUnitTest(c context.Context) {
	s.context = c
}

type authWebhookResponse struct {
	ManifestID           string   `json:"manifestID"`
	StreamID             string   `json:"streamID"`
	SessionID            string   `json:"sessionID"`
	StreamKey            string   `json:"streamKey"`
	Presets              []string `json:"presets"`
	ObjectStore          string   `json:"objectStore"`
	RecordObjectStore    string   `json:"recordObjectStore"`
	RecordObjectStoreURL string   `json:"recordObjectStoreUrl"`
	// Same json structure is used in lpms to decode profile from
	// files, while here we decode from HTTP
	Profiles           []ffmpeg.JsonProfile `json:"profiles"`
	PreviousSessions   []string             `json:"previousSessions"`
	VerificationFreq   uint                 `json:"verificationFreq"`
	TimeoutMultiplier  int                  `json:"timeoutMultiplier"`
	ForceSessionReinit bool                 `json:"forceSessionReinit"`
}

func NewLivepeerServer(rtmpAddr string, lpNode *core.LivepeerNode, httpIngest bool, transcodingOptions string) (*LivepeerServer, error) {
	opts := lpmscore.LPMSOpts{
		RtmpAddr:     rtmpAddr,
		RtmpDisabled: true,
		WorkDir:      lpNode.WorkDir,
		HttpMux:      http.NewServeMux(),
	}
	switch lpNode.NodeType {
	case core.BroadcasterNode:
		opts.RtmpDisabled = false

		if transcodingOptions != "" {
			var profiles []ffmpeg.VideoProfile
			content, err := ioutil.ReadFile(transcodingOptions)
			if err == nil && len(content) > 0 {
				stubResp := &authWebhookResponse{}
				err = json.Unmarshal(content, &stubResp.Profiles)
				if err != nil {
					return nil, err
				}
				profiles, err = ffmpeg.ParseProfilesFromJsonProfileArray(stubResp.Profiles)
				if err != nil {
					return nil, err
				}
			} else {
				// check the built-in profiles
				profiles = parsePresets(strings.Split(transcodingOptions, ","))
			}
			if len(profiles) <= 0 {
				return nil, fmt.Errorf("No transcoding profiles found")
			}
			BroadcastJobVideoProfiles = profiles
		}
	}
	server := lpmscore.New(&opts)
	ls := &LivepeerServer{RTMPSegmenter: server, LPMS: server, LivepeerNode: lpNode, HTTPMux: opts.HttpMux, connectionLock: &sync.RWMutex{},
		serverLock:              &sync.RWMutex{},
		rtmpConnections:         make(map[core.ManifestID]*rtmpConnection),
		internalManifests:       make(map[core.ManifestID]core.ManifestID),
		recordingsAuthResponses: cache.New(time.Hour, 2*time.Hour),
	}
	if lpNode.NodeType == core.BroadcasterNode && httpIngest {
		opts.HttpMux.HandleFunc("/live/", ls.HandlePush)
	}
	opts.HttpMux.HandleFunc("/recordings/", ls.HandleRecordings)
	return ls, nil
}

// StartMediaServer starts the LPMS server
func (s *LivepeerServer) StartMediaServer(ctx context.Context, httpAddr string) error {
	glog.V(common.SHORT).Infof("Transcode Job Type: %v", BroadcastJobVideoProfiles)

	// Store ctx to later use as cancel signal for watchdog goroutine
	s.context = ctx

	// health endpoint
	s.HTTPMux.Handle("/healthz", s.healthzHandler())

	//LPMS handlers for handling RTMP video
	s.LPMS.HandleRTMPPublish(createRTMPStreamIDHandler(ctx, s, nil), gotRTMPStreamHandler(s), endRTMPStreamHandler(s))
	s.LPMS.HandleRTMPPlay(getRTMPStreamHandler(s))

	//LPMS handler for handling HLS video play
	s.LPMS.HandleHLSPlay(getHLSMasterPlaylistHandler(s), getHLSMediaPlaylistHandler(s), getHLSSegmentHandler(s))

	//Start the LPMS server
	lpmsCtx, cancel := context.WithCancel(ctx)

	ec := make(chan error, 2)
	go func() {
		if err := s.LPMS.Start(lpmsCtx); err != nil {
			// typically triggered if there's an error with broadcaster LPMS
			// transcoder LPMS should return without an error
			ec <- s.LPMS.Start(lpmsCtx)
		}
	}()
	if s.LivepeerNode.NodeType == core.BroadcasterNode {
		go func() {
			glog.V(4).Infof("HTTP Server listening on http://%v", httpAddr)
			ec <- http.ListenAndServe(httpAddr, s.HTTPMux)
		}()
	}

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

// RTMP Publish Handlers
func createRTMPStreamIDHandler(_ctx context.Context, s *LivepeerServer, webhookResponseOverride *authWebhookResponse) func(url *url.URL) (strmID stream.AppData, e error) {
	return func(url *url.URL) (strmID stream.AppData, e error) {
		//Check HTTP header for ManifestID
		//If ManifestID is passed in HTTP header, use that one
		//Else check webhook for ManifestID
		//If ManifestID is returned from webhook, use it
		//Else check URL for ManifestID
		//If ManifestID is passed in URL, use that one
		//Else create one
		var resp *authWebhookResponse
		var mid core.ManifestID
		var extStreamID, sessionID string
		var err error
		var key string
		var os, ros drivers.OSDriver
		var oss, ross drivers.OSSession
		profiles := []ffmpeg.VideoProfile{}
		var VerificationFreq uint
		nonce := rand.Uint64()

		// do not replace captured _ctx variable
		ctx := clog.AddNonce(_ctx, nonce)
		if resp, err = authenticateStream(AuthWebhookURL, url.String()); err != nil {
			clog.Errorf(ctx, fmt.Sprintf("Forbidden: Authentication denied for streamID url=%s err=%q", url.String(), err))
			return nil, errForbidden
		}

		// If we've received auth in header AND callback URL forms then for now, we reject cases where they're
		// trying to give us different profiles
		if resp != nil && webhookResponseOverride != nil {
			if !resp.areProfilesEqual(*webhookResponseOverride) {
				clog.Errorf(ctx, "Received auth header with profiles that don't match those in callback URL response")
				return nil, fmt.Errorf("Received auth header with profiles that don't match those in callback URL response")
			}
		}

		// If we've received a header containing auth values then let those override any from a callback URL
		if webhookResponseOverride != nil {
			resp = webhookResponseOverride
		}

		if resp != nil {
			mid, key = parseManifestID(resp.ManifestID), resp.StreamKey
			extStreamID, sessionID = resp.StreamID, resp.SessionID
			if sessionID != "" && extStreamID != "" && sessionID != extStreamID {
				ctx = clog.AddSessionID(ctx, sessionID)
			}
			// Process transcoding options presets
			if len(resp.Presets) > 0 {
				profiles = parsePresets(resp.Presets)
			}

			parsedProfiles, err := ffmpeg.ParseProfilesFromJsonProfileArray(resp.Profiles)
			if err != nil {
				errMsg := fmt.Sprintf("Failed to parse JSON video profile for streamID url=%s err=%q", url.String(), err)
				clog.Errorf(ctx, errMsg)
				return nil, fmt.Errorf(errMsg)
			}
			profiles = append(profiles, parsedProfiles...)

			// Only set defaults if user did not specify a preset/profile
			if resp.Profiles == nil && len(resp.Presets) <= 0 {
				profiles = BroadcastJobVideoProfiles
			}

			// set OS if it was provided
			if resp.ObjectStore != "" {
				os, err = drivers.ParseOSURL(resp.ObjectStore, false)
				if err != nil {
					errMsg := fmt.Sprintf("Failed to parse object store url for streamID url=%s err=%q", url.String(), err)
					clog.Errorf(ctx, errMsg)
					return nil, fmt.Errorf(errMsg)
				}
			}
			// set Recording OS if it was provided
			if resp.RecordObjectStore != "" {
				ros, err = drivers.ParseOSURL(resp.RecordObjectStore, true)
				if err != nil {
					errMsg := fmt.Sprintf("Failed to parse recording object store url for streamID url=%s err=%q", url.String(), err)
					clog.Errorf(ctx, errMsg)
					return nil, fmt.Errorf(errMsg)
				}
			}

			VerificationFreq = resp.VerificationFreq
		} else {
			profiles = BroadcastJobVideoProfiles
		}
		sid := parseStreamID(url.Path)
		extmid := sid.ManifestID
		if mid == "" {
			mid, key = sid.ManifestID, sid.Rendition
		}
		if mid == "" {
			mid = core.RandomManifestID()
		}
		// Generate RTMP part of StreamID
		if key == "" {
			key = common.RandomIDGenerator(StreamKeyBytes)
		}
		ctx = clog.AddManifestID(ctx, string(mid))

		if os != nil {
			oss = os.NewSession(string(mid))
		}

		recordPath := fmt.Sprintf("%s/%s", extmid, monitor.NodeID)
		if ros != nil {
			ross = ros.NewSession(recordPath)
		} else if drivers.RecordStorage != nil {
			ross = drivers.RecordStorage.NewSession(recordPath)
		}
		// Ensure there's no concurrent StreamID with the same name
		s.connectionLock.RLock()
		defer s.connectionLock.RUnlock()
		if core.MaxSessions > 0 && len(s.rtmpConnections) >= core.MaxSessions {
			errMsg := fmt.Sprintf("Too many connections for streamID url=%s err=%q", url.String(), err)
			clog.Errorf(ctx, errMsg)
			return nil, fmt.Errorf(errMsg)

		}
		return &core.StreamParameters{
			ManifestID:       mid,
			ExternalStreamID: extStreamID,
			SessionID:        sessionID,
			RtmpKey:          key,
			// HTTP push mutates `profiles` so make a copy of it
			Profiles:         append([]ffmpeg.VideoProfile(nil), profiles...),
			OS:               oss,
			RecordOS:         ross,
			VerificationFreq: VerificationFreq,
			Nonce:            nonce,
		}, nil
	}
}

func streamParams(d stream.AppData) *core.StreamParameters {
	p, ok := d.(*core.StreamParameters)
	if !ok {
		glog.Error("Mismatched type for RTMP app data")
		return nil
	}
	return p
}

func gotRTMPStreamHandler(s *LivepeerServer) func(url *url.URL, rtmpStrm stream.RTMPVideoStream) (err error) {
	return func(url *url.URL, rtmpStrm stream.RTMPVideoStream) (err error) {

		cxn, err := s.registerConnection(context.Background(), rtmpStrm, nil, PixelFormatNone(), nil)
		if err != nil {
			return err
		}

		mid := cxn.mid
		nonce := cxn.nonce
		startSeq := 0

		streamStarted := false
		//Segment the stream, insert the segments into the broadcaster
		go func(rtmpStrm stream.RTMPVideoStream) {
			hid := string(core.RandomManifestID()) // ffmpeg m3u8 output name
			hlsStrm := stream.NewBasicHLSVideoStream(hid, stream.DefaultHLSStreamWin)
			hlsStrm.SetSubscriber(func(seg *stream.HLSSegment, eof bool) {
				if eof {
					// XXX update HLS manifest
					return
				}
				if !streamStarted {
					streamStarted = true
					if monitor.Enabled {
						monitor.StreamStarted(nonce)
					}
				}
				go processSegment(context.Background(), cxn, seg, nil)
			})

			segOptions := segmenter.SegmenterOptions{
				StartSeq:  startSeq,
				SegLength: SegLen,
			}
			err := s.RTMPSegmenter.SegmentRTMPToHLS(context.Background(), rtmpStrm, hlsStrm, segOptions)
			if err != nil {
				// Stop the incoming RTMP connection.
				if err == segmenter.ErrSegmenterTimeout {
					glog.Info("RTMP Timeout: Ensure keyframe interval is less than 8 seconds")
				}
				// TODO retry segmentation if err != SegmenterTimeout; may be recoverable
				rtmpStrm.Close()
			}

		}(rtmpStrm)

		if monitor.Enabled {
			monitor.StreamCreated(string(mid), nonce)
		}

		glog.Infof("\n\nVideo Created With ManifestID: %v\n\n", mid)

		return nil
	}
}

func endRTMPStreamHandler(s *LivepeerServer) func(url *url.URL, rtmpStrm stream.RTMPVideoStream) error {
	return func(url *url.URL, rtmpStrm stream.RTMPVideoStream) error {
		params := streamParams(rtmpStrm.AppData())
		params.Capabilities.SetMinVersionConstraint(s.LivepeerNode.Capabilities.MinVersionConstraint())
		if params == nil {
			return errMismatchedParams
		}

		//Remove RTMP stream
		err := removeRTMPStream(context.Background(), s, params.ManifestID)
		if err != nil {
			return err
		}
		return nil
	}
}

func (s *LivepeerServer) registerConnection(ctx context.Context, rtmpStrm stream.RTMPVideoStream, actualStreamCodec *ffmpeg.VideoCodec, pixelFormat ffmpeg.PixelFormat, segPar *core.SegmentParameters) (*rtmpConnection, error) {
	ctx = clog.Clone(context.Background(), ctx)
	// Set up the connection tracking
	params := streamParams(rtmpStrm.AppData())
	if params == nil {
		return nil, errMismatchedParams
	}
	mid := params.ManifestID
	if drivers.NodeStorage == nil {
		clog.Errorf(ctx, "Missing node storage")
		return nil, errStorage
	}
	// Build the source video profile from the RTMP stream.
	if params.Resolution == "" {
		params.Resolution = fmt.Sprintf("%vx%v", rtmpStrm.Width(), rtmpStrm.Height())
	}
	if params.OS == nil {
		params.OS = drivers.NodeStorage.NewSession(string(mid))
	}
	storage := params.OS

	// Generate and set capabilities
	if actualStreamCodec != nil {
		params.Codec = *actualStreamCodec
	}
	params.PixelFormat = pixelFormat

	caps, err := core.JobCapabilities(params, segPar)
	if err != nil {
		return nil, err
	}
	params.Capabilities = caps

	recordStorage := params.RecordOS
	vProfile := ffmpeg.VideoProfile{
		Name:       "source",
		Resolution: params.Resolution,
		Bitrate:    "4000k", // Fix this
		Format:     params.Format,
	}
	hlsStrmID := core.MakeStreamID(mid, &vProfile)
	playlist := core.NewBasicPlaylistManager(mid, storage, recordStorage)

	// first, initialize connection without SessionManager, which creates O and T sessions, and may leave
	// connectionLock locked for significant amount of time
	cxn := &rtmpConnection{
		mid:          mid,
		initializing: make(chan struct{}),
		nonce:        params.Nonce,
		stream:       rtmpStrm,
		pl:           playlist,
		profile:      &vProfile,
		params:       params,
		lastUsed:     time.Now(),
	}
	s.connectionLock.Lock()
	oldCxn, exists := s.getActiveRtmpConnectionUnsafe(mid)
	if exists {
		// We can only have one concurrent stream per ManifestID
		s.connectionLock.Unlock()
		return oldCxn, errAlreadyExists
	}
	s.rtmpConnections[mid] = cxn
	// do not obtain this lock again while initializing channel is open, it will cause deadlock if other goroutine already obtained the lock and called getActiveRtmpConnectionUnsafe()
	s.connectionLock.Unlock()

	// safe, because other goroutines should be waiting on initializing channel
	cxn.sessManager = NewSessionManager(ctx, s.LivepeerNode, params)

	// populate fields and signal initializing channel
	s.serverLock.Lock()
	s.lastManifestID = mid
	s.lastHLSStreamID = hlsStrmID
	s.serverLock.Unlock()

	// connection is ready, only monitoring below
	close(cxn.initializing)

	// need lock to access rtmpConnections
	s.connectionLock.RLock()
	defer s.connectionLock.RUnlock()
	sessionsNumber := len(s.rtmpConnections)
	fastVerificationEnabled, fastVerificationUsing := countStreamsWithFastVerificationEnabled(s.rtmpConnections)

	if monitor.Enabled {
		monitor.CurrentSessions(sessionsNumber)
		monitor.FastVerificationEnabledAndUsingCurrentSessions(fastVerificationEnabled, fastVerificationUsing)
	}

	return cxn, nil
}

func countStreamsWithFastVerificationEnabled(rtmpConnections map[core.ManifestID]*rtmpConnection) (int, int) {
	var enabled, using int
	for _, cxn := range rtmpConnections {
		if cxn.params.VerificationFreq > 0 {
			enabled++
			if cxn.sessManager != nil && cxn.sessManager.usingVerified() {
				using++
			}
		}
	}
	return enabled, using
}

func removeRTMPStream(ctx context.Context, s *LivepeerServer, extmid core.ManifestID) error {
	s.connectionLock.Lock()
	defer s.connectionLock.Unlock()
	intmid := extmid
	if _intmid, exists := s.internalManifests[extmid]; exists {
		// Use the internal manifestID that was stored for the provided manifestID
		// to index into rtmpConnections
		intmid = _intmid
	}
	cxn, ok := s.getActiveRtmpConnectionUnsafe(intmid)
	if !ok || cxn.pl == nil {
		clog.Warningf(ctx, "Attempted to end unknown stream with manifestID=%s", extmid)
		return errUnknownStream
	}
	cxn.stream.Close()
	cxn.sessManager.cleanup(ctx)
	cxn.pl.Cleanup()
	clog.Infof(ctx, "Ended stream with manifestID=%s external manifestID=%s", intmid, extmid)
	delete(s.rtmpConnections, intmid)
	delete(s.internalManifests, extmid)

	if monitor.Enabled {
		monitor.StreamEnded(ctx, cxn.nonce)
		monitor.CurrentSessions(len(s.rtmpConnections))
		monitor.FastVerificationEnabledAndUsingCurrentSessions(countStreamsWithFastVerificationEnabled(s.rtmpConnections))
	}

	return nil
}

//End RTMP Publish Handlers

// HLS Play Handlers
func getHLSMasterPlaylistHandler(s *LivepeerServer) func(url *url.URL) (*m3u8.MasterPlaylist, error) {
	return func(url *url.URL) (*m3u8.MasterPlaylist, error) {
		var manifestID core.ManifestID
		if s.ExposeCurrentManifest && strings.ToLower(url.Path) == "/stream/current.m3u8" {
			manifestID = s.LastManifestID()
		} else {
			sid := parseStreamID(url.Path)
			if sid.Rendition != "" {
				// requesting a media PL, not master PL
				return nil, vidplayer.ErrNotFound
			}
			manifestID = sid.ManifestID
		}

		s.connectionLock.RLock()
		defer s.connectionLock.RUnlock()
		cxn, ok := s.getActiveRtmpConnectionUnsafe(manifestID)
		if !ok || cxn.pl == nil {
			return nil, vidplayer.ErrNotFound
		}
		cpl := cxn.pl

		if cpl.ManifestID() != manifestID {
			return nil, vidplayer.ErrNotFound
		}
		return cpl.GetHLSMasterPlaylist(), nil
	}
}

func getHLSMediaPlaylistHandler(s *LivepeerServer) func(url *url.URL) (*m3u8.MediaPlaylist, error) {
	return func(url *url.URL) (*m3u8.MediaPlaylist, error) {
		strmID := parseStreamID(url.Path)
		mid := strmID.ManifestID
		s.connectionLock.RLock()
		defer s.connectionLock.RUnlock()
		cxn, ok := s.getActiveRtmpConnectionUnsafe(mid)
		if !ok || cxn.pl == nil {
			return nil, vidplayer.ErrNotFound
		}

		//Get the hls playlist
		pl := cxn.pl.GetHLSMediaPlaylist(strmID.Rendition)
		if pl == nil {
			return nil, vidplayer.ErrNotFound
		}
		return pl, nil
	}
}

func getHLSSegmentHandler(s *LivepeerServer) func(url *url.URL) ([]byte, error) {
	return func(url *url.URL) ([]byte, error) {
		// Strip the /stream/ prefix
		segName := cleanStreamPrefix(url.Path)
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

// Start RTMP Play Handlers
func getRTMPStreamHandler(s *LivepeerServer) func(url *url.URL) (stream.RTMPVideoStream, error) {
	return func(url *url.URL) (stream.RTMPVideoStream, error) {
		mid := parseManifestID(url.Path)
		s.connectionLock.RLock()
		cxn, ok := s.getActiveRtmpConnectionUnsafe(mid)
		defer s.connectionLock.RUnlock()
		if !ok {
			glog.Error("Cannot find RTMP stream for ManifestID ", mid)
			return nil, vidplayer.ErrNotFound
		}

		//Could use a subscriber, but not going to here because the RTMP stream doesn't need to be available for consumption by multiple views.  It's only for the segmenter.
		return cxn.stream, nil
	}
}

//End RTMP Handlers

type BreakOperation bool

// HandlePush processes request for HTTP ingest
func (s *LivepeerServer) HandlePush(w http.ResponseWriter, r *http.Request) {
	errorOut := func(status int, s string, params ...interface{}) {
		httpErr := fmt.Sprintf(s, params...)
		statusErr := fmt.Sprintf(" statusCode=%d", status)
		glog.Error(httpErr + statusErr)
		http.Error(w, httpErr, status)
	}

	start := time.Now()
	if r.Method != "POST" && r.Method != "PUT" {
		errorOut(http.StatusMethodNotAllowed, `http push request wrong method=%s url=%s host=%s`, r.Method, r.URL, r.Host)
		return
	}

	authHeaderConfig, err := getTranscodeConfiguration(r)
	if err != nil {
		httpErr := fmt.Sprintf(`failed to parse transcode config header: %q`, err)
		glog.Error(httpErr)
		http.Error(w, httpErr, http.StatusBadRequest)
		return
	}

	body, err := common.ReadAtMost(r.Body, common.MaxSegSize)
	if err != nil {
		errorOut(http.StatusInternalServerError, `Error reading http request body: %s`, err.Error())
		return
	}
	r.Body.Close()
	r.URL = &url.URL{Scheme: "http", Host: r.Host, Path: r.URL.Path}

	// Determine the input format the request is claiming to have
	ext := path.Ext(r.URL.Path)
	format := common.ProfileExtensionFormat(ext)
	if ffmpeg.FormatNone == format {
		// ffmpeg sends us a m3u8 as well, so ignore
		// Alternatively, reject m3u8s explicitly and take any other type
		// TODO also look at use content-type
		errorOut(http.StatusBadRequest, `ignoring file extension: %s`, ext)
		return
	}
	ctx := r.Context()

	mid := parseManifestID(r.URL.Path)
	if mid != "" {
		ctx = clog.AddManifestID(ctx, string(mid))
	}
	remoteAddr := getRemoteAddr(r)
	ctx = clog.AddVal(ctx, clog.ClientIP, remoteAddr)

	sliceFromStr := r.Header.Get("Content-Slice-From")
	sliceToStr := r.Header.Get("Content-Slice-To")

	clog.Infof(ctx, "Got push request at url=%s ua=%s addr=%s bytes=%d dur=%s resolution=%s slice-from=%s slice-to=%s", r.URL.String(), r.UserAgent(),
		remoteAddr, len(body), r.Header.Get("Content-Duration"), r.Header.Get("Content-Resolution"), sliceFromStr, sliceToStr)
	var sliceFromDur time.Duration
	if valMs, err := strconv.ParseUint(sliceFromStr, 10, 64); err == nil {
		sliceFromDur = time.Duration(valMs) * time.Millisecond
	}
	var sliceToDur time.Duration
	if valMs, err := strconv.ParseUint(sliceToStr, 10, 64); err == nil {
		sliceToDur = time.Duration(valMs) * time.Millisecond
	}
	var segPar core.SegmentParameters
	if sliceFromDur > 0 || sliceToDur > 0 {
		if sliceFromDur > 0 && sliceToDur > 0 && sliceFromDur > sliceToDur {
			httpErr := fmt.Sprintf(`Invalid slice config from=%s to=%s`, sliceFromDur, sliceToDur)
			clog.Errorf(ctx, httpErr)
			http.Error(w, httpErr, http.StatusBadRequest)
			return
		}
		segPar.Clip = &core.SegmentClip{
			From: sliceFromDur,
			To:   sliceToDur,
		}
	}
	if authHeaderConfig != nil {
		segPar.ForceSessionReinit = authHeaderConfig.ForceSessionReinit
	}

	now := time.Now()
	if mid == "" {
		errorOut(http.StatusBadRequest, "Bad URL url=%s", r.URL)
		return
	}
	s.connectionLock.RLock()
	if intmid, exists := s.internalManifests[mid]; exists {
		mid = intmid
	}
	cxn, exists := s.getActiveRtmpConnectionUnsafe(mid)
	if monitor.Enabled {
		fastVerificationEnabled, fastVerificationUsing := countStreamsWithFastVerificationEnabled(s.rtmpConnections)
		monitor.FastVerificationEnabledAndUsingCurrentSessions(fastVerificationEnabled, fastVerificationUsing)
	}
	s.connectionLock.RUnlock()
	ctx = clog.AddManifestID(ctx, string(mid))
	if exists && cxn != nil {
		s.connectionLock.Lock()
		cxn.lastUsed = now
		s.connectionLock.Unlock()
		ctx = clog.AddNonce(ctx, cxn.nonce)
	}

	status, mediaFormat, err := ffmpeg.GetCodecInfoBytes(body)
	isZeroFrame := status == ffmpeg.CodecStatusNeedsBypass
	if err != nil {
		errorOut(http.StatusUnprocessableEntity, "Error getting codec info url=%s", r.URL)
		return
	}

	var vcodec *ffmpeg.VideoCodec
	if len(mediaFormat.Vcodec) == 0 {
		clog.Warningf(ctx, "Couldn't detect input video stream codec")
	} else {
		vcodecVal, ok := ffmpeg.FfmpegNameToVideoCodec[mediaFormat.Vcodec]
		vcodec = &vcodecVal
		if !ok {
			errorOut(http.StatusUnprocessableEntity, "Unknown input stream codec=%s", mediaFormat.Vcodec)
			return
		}
	}

	// Check for presence and register if a fresh cxn
	if !exists {
		appData, err := (createRTMPStreamIDHandler(ctx, s, authHeaderConfig))(r.URL)
		if err != nil {
			if errors.Is(err, errForbidden) {
				errorOut(http.StatusForbidden, "Could not create stream ID: url=%s", r.URL)
				return
			} else {
				errorOut(http.StatusInternalServerError, "Could not create stream ID: url=%s", r.URL)
				return
			}
		}
		params := streamParams(appData)
		if authHeaderConfig != nil {
			params.TimeoutMultiplier = authHeaderConfig.TimeoutMultiplier
		}
		params.Resolution = r.Header.Get("Content-Resolution")
		params.Format = format
		s.connectionLock.RLock()
		_, cxnExists := s.getActiveRtmpConnectionUnsafe(params.ManifestID)
		if mid != params.ManifestID && cxnExists && s.internalManifests[mid] == "" {
			// Pre-existing connection found for this new stream with the same underlying manifestID
			var oldStreamID core.ManifestID
			for k, v := range s.internalManifests {
				if v == params.ManifestID {
					oldStreamID = k
					break
				}
			}
			s.connectionLock.RUnlock()
			if oldStreamID != "" && mid != oldStreamID {
				// Close the old connection, and open a new one
				// TODO try to re-use old HLS playlist?
				clog.Warningf(ctx, "Ending streamID=%v as new streamID=%s with same manifestID=%s has arrived",
					oldStreamID, mid, params.ManifestID)
				removeRTMPStream(context.TODO(), s, oldStreamID)
			}
		} else {
			s.connectionLock.RUnlock()
		}
		st := stream.NewBasicRTMPVideoStream(appData)
		// Set output formats if not explicitly specified
		for i, v := range params.Profiles {
			if ffmpeg.FormatNone == v.Format {
				params.Profiles[i].Format = format
			}
		}

		cxn, err = s.registerConnection(ctx, st, vcodec, mediaFormat.PixFormat, &segPar)
		if err != nil {
			st.Close()
			if err != errAlreadyExists {
				errorOut(http.StatusInternalServerError, "http push error url=%s err=%q", r.URL, err)
				return
			} // else we continue with the old cxn
		} else {
			// Start a watchdog to remove session after a period of inactivity
			ticker := time.NewTicker(httpPushTimeout)
			// print stack trace here:
			go func(s *LivepeerServer, intmid, extmid core.ManifestID) {
				runCheck := func() BreakOperation {
					var lastUsed time.Time
					s.connectionLock.RLock()
					if cxn, exists := s.getActiveRtmpConnectionUnsafe(intmid); exists {
						lastUsed = cxn.lastUsed
					}
					if _, exists := s.internalManifests[extmid]; !exists && intmid != extmid {
						s.connectionLock.RUnlock()
						clog.Warningf(ctx, "Watchdog tried closing session for streamID=%s, which was already closed", extmid)
						return true
					}
					s.connectionLock.RUnlock()
					if time.Since(lastUsed) > httpPushTimeout {
						_ = removeRTMPStream(context.TODO(), s, extmid)
						return true
					}
					return false
				}
				defer ticker.Stop()
				if s.context == nil {
					for range ticker.C {
						if runCheck() {
							return
						}
					}
				}
				for {
					select {
					case <-ticker.C:
						if runCheck() {
							return
						}
					case <-s.context.Done():
						return
					}
				}
			}(s, cxn.mid, mid)
		}
		// Regardless of old/new cxn returned by registerConnection, we make sure
		// our internalManifests mapping is OK before moving on
		if cxn.mid != mid {
			// AuthWebhook provided different ManifestID
			s.connectionLock.Lock()
			s.internalManifests[mid] = cxn.mid
			s.connectionLock.Unlock()
			mid = cxn.mid
		}
	}
	ctx = clog.AddManifestID(ctx, string(mid))
	defer func(now time.Time) {
		clog.Infof(ctx, "Finished push request at url=%s ua=%s addr=%s bytes=%d dur=%s resolution=%s took=%s", r.URL.String(), r.UserAgent(), r.RemoteAddr, len(body),
			r.Header.Get("Content-Duration"), r.Header.Get("Content-Resolution"), time.Since(now))
	}(now)

	fname := path.Base(r.URL.Path)
	seq, err := strconv.ParseUint(strings.TrimSuffix(fname, ext), 10, 64)
	if err != nil {
		seq = 0
	}
	ctx = clog.AddSeqNo(ctx, seq)

	duration, err := strconv.Atoi(r.Header.Get("Content-Duration"))
	if err != nil {
		duration = 2000
		glog.Info("Missing duration; filling in a default of 2000ms")
	}

	seg := &stream.HLSSegment{
		Data:        body,
		Name:        fname,
		SeqNo:       seq,
		Duration:    float64(duration) / 1000.0,
		IsZeroFrame: isZeroFrame,
	}

	// Kick watchdog periodically so session doesn't time out during long transcodes
	requestEnded := make(chan struct{}, 1)
	defer func() { requestEnded <- struct{}{} }()
	go func() {
		for {
			tick, cancel := httpPushResetTimer()
			select {
			case <-requestEnded:
				cancel()
				return
			case <-tick.Done():
				clog.V(common.VERBOSE).Infof(ctx, "watchdog reset seq=%d dur=%v started=%v", seq, duration, now)
				s.connectionLock.RLock()
				if cxn, exists := s.getActiveRtmpConnectionUnsafe(mid); exists {
					cxn.lastUsed = time.Now()
				}
				s.connectionLock.RUnlock()
			}
		}
	}()

	// Reinitialize HW Session if video segment resolution has changed
	cxn.mu.Lock()
	if cxn.mediaFormat == (ffmpeg.MediaFormatInfo{}) {
		cxn.mediaFormat = mediaFormat
	} else if !mediaCompatible(cxn.mediaFormat, mediaFormat) {
		cxn.mediaFormat = mediaFormat
		segPar.ForceSessionReinit = true
	}
	cxn.mu.Unlock()

	// Do the transcoding!
	urls, err := processSegment(ctx, cxn, seg, &segPar)
	if err != nil {
		status := http.StatusInternalServerError
		if isNonRetryableError(err) {
			status = http.StatusUnprocessableEntity
		}
		errorOut(status, "http push error processing segment url=%s manifestID=%s err=%q", r.URL, mid, err)
		return
	}
	select {
	case <-r.Context().Done():
		// HTTP request already timed out
		if monitor.Enabled {
			monitor.HTTPClientTimedOut1(ctx)
		}
		return
	default:
	}
	if len(urls) == 0 {
		if len(cxn.params.Profiles) > 0 {
			clog.Errorf(ctx, "No sessions available name=%s url=%s statusCode=%d", fname, r.URL, http.StatusServiceUnavailable)
			http.Error(w, "No sessions available", http.StatusServiceUnavailable)
		}
		return
	}
	renditionData := make([][]byte, len(urls))
	// find data in local storage
	memOS, ok := cxn.pl.GetOSSession().(*drivers.MemorySession)
	if ok {
		for i, fname := range urls {
			data := memOS.GetData(fname)
			if data != nil {
				renditionData[i] = data
			}
		}
	}
	clog.Infof(ctx, "Finished transcoding push request at url=%s took=%s", r.URL.String(), time.Since(now))

	boundary := common.RandName()
	accept := r.Header.Get("Accept")
	if accept == "multipart/mixed" {
		contentType := "multipart/mixed; boundary=" + boundary
		w.Header().Set("Content-Type", contentType)
	}
	w.WriteHeader(http.StatusOK)
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
	if accept != "multipart/mixed" {
		return
	}
	mw := multipart.NewWriter(w)
	var fw io.Writer
	for i, url := range urls {
		mw.SetBoundary(boundary)
		var typ, ext string
		length := len(renditionData[i])
		if length == 0 {
			typ, ext, length = "application/vnd+livepeer.uri", ".txt", len(url)
		} else {
			format := cxn.params.Profiles[i].Format
			ext, err = common.ProfileFormatExtension(format)
			if err != nil {
				clog.Errorf(ctx, "Unknown extension for format err=%q", err)
				break
			}
			typ, err = common.ProfileFormatMimeType(format)
			if err != nil {
				clog.Errorf(ctx, "Unknown mime type for format url=%s err=%q ", r.URL, err)
			}
		}
		profile := cxn.params.Profiles[i].Name
		fname := fmt.Sprintf(`"%s_%d%s"`, profile, seq, ext)
		hdrs := textproto.MIMEHeader{
			"Content-Type":        {typ + "; name=" + fname},
			"Content-Length":      {strconv.Itoa(length)},
			"Content-Disposition": {"attachment; filename=" + fname},
			"Rendition-Name":      {profile},
		}
		fw, err = mw.CreatePart(hdrs)
		if err != nil {
			clog.Errorf(ctx, "Could not create multipart part err=%q", err)
			break
		}
		if len(renditionData[i]) > 0 {
			_, err = io.Copy(fw, bytes.NewBuffer(renditionData[i]))
			if err != nil {
				break
			}
		} else {
			_, err = fw.Write([]byte(url))
			if err != nil {
				break
			}
		}
	}
	if err == nil {
		err = mw.Close()
	}
	if err != nil {
		clog.Errorf(ctx, "Error sending transcoded response url=%s err=%q", r.URL.String(), err)
		if monitor.Enabled {
			monitor.HTTPClientTimedOut2(ctx)
		}
		return
	}
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
	roundtripTime := time.Since(start)
	select {
	case <-r.Context().Done():
		// HTTP request already timed out
		if monitor.Enabled {
			monitor.HTTPClientTimedOut2(ctx)
		}
		return
	default:
	}
	if monitor.Enabled {
		monitor.SegmentFullyProcessed(ctx, seg.Duration, roundtripTime.Seconds())
	}
}

// getPlaylistsFromStore finds all the json playlist files belonging to the provided manifests
// returns:
// - a map of manifestID -> a list of indices pointing to JSON files in the returned list of JSON files
// - a list of JSON files for all manifestIDs provided
// - the latest playlist time
func getPlaylistsFromStore(ctx context.Context, sess drivers.OSSession, manifests []string) (map[string][]int, []string, time.Time, error) {
	var latestPlaylistTime time.Time
	var jsonFiles []string
	filesMap := make(map[string][]int)
	for _, manifestID := range manifests {
		filesMap[manifestID] = nil
		start := time.Now()
		filesPage, err := sess.ListFiles(ctx, manifestID+"/", "/")
		if err != nil {
			return nil, nil, latestPlaylistTime, err
		}
		glog.V(common.VERBOSE).Infof("Listing directories for manifestID=%s took=%s", manifestID, time.Since(start))
		dirs := filesPage.Directories()
		if len(dirs) == 0 {
			continue
		}
		for _, dirName := range dirs {
			start = time.Now()
			dirOnePage, err := sess.ListFiles(ctx, dirName+"playlist_", "")
			glog.V(common.VERBOSE).Infof("Listing playlist files for manifestID=%s took=%s", manifestID, time.Since(start))
			if err != nil {
				return nil, nil, latestPlaylistTime, err
			}
			for {
				playlistsNames := dirOnePage.Files()
				for _, plf := range playlistsNames {
					if plf.LastModified.After(latestPlaylistTime) {
						latestPlaylistTime = plf.LastModified
					}
					filesMap[manifestID] = append(filesMap[manifestID], len(jsonFiles))
					jsonFiles = append(jsonFiles, plf.Name)
				}
				if !dirOnePage.HasNextPage() {
					break
				}
				dirOnePage, err = dirOnePage.NextPage()
				if err != nil {
					return nil, nil, latestPlaylistTime, err
				}
			}
		}
	}
	return filesMap, jsonFiles, latestPlaylistTime, nil
}

func (s *LivepeerServer) streamMP4(w http.ResponseWriter, r *http.Request, jpl *core.JsonPlaylist, manifestID, track, fileName string) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	contentType, _ := common.TypeByExtension(".mp4")
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename=%s;`, fileName))
	var sourceBytesSent, resultBytesSent int64

	or, ow, err := os.Pipe()
	if err != nil {
		glog.Errorf("Error creating pipe manifestID=%s err=%q", manifestID, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	tc := ffmpeg.NewTranscoder()
	done := make(chan struct{})
	go func() {
		var err2 error
		resultBytesSent, err2 = io.Copy(w, or)
		if err2 != nil {
			glog.Errorf("Error transmuxing to mp4 request=%s manifestID=%s err=%q", r.URL.String(), manifestID, err2)
		}
		or.Close()
		done <- struct{}{}
	}()
	defer func() {
		tc.StopTranscoder()
		ow.Close()
		<-done
		glog.Infof("Completed mp4 request=%s manifestID=%s sourceBytes=%d destBytes=%d", r.URL.String(),
			manifestID, atomic.LoadInt64(&sourceBytesSent), resultBytesSent)
	}()
	oname := fmt.Sprintf("pipe:%d", ow.Fd())
	out := []ffmpeg.TranscodeOptions{
		{
			Oname: oname,
			VideoEncoder: ffmpeg.ComponentOptions{
				Name: "copy",
			},
			AudioEncoder: ffmpeg.ComponentOptions{
				Name: "copy",
			},
			Profile: ffmpeg.VideoProfile{Format: ffmpeg.FormatNone},
			Muxer: ffmpeg.ComponentOptions{
				Name: "mp4",
				// main option is 'frag_keyframe' which tells ffmpeg to create fragmented MP4 (which we need to be able to stream generatd file)
				// other options is not mandatory but they will slightly improve generated MP4 file
				Opts: map[string]string{"movflags": "frag_keyframe+negative_cts_offsets+omit_tfhd_offset+disable_chpl+default_base_moof"},
			},
		},
	}
	for _, seg := range jpl.Segments[track] {
		if seg.GetDiscontinuity() {
			tc.Discontinuity()
		}

		ir, iw, err := os.Pipe()
		if err != nil {
			glog.Errorf("Error creating pipe manifestID=%s err=%q", manifestID, err)
			return
		}
		fname := fmt.Sprintf("pipe:%d", ir.Fd())

		in := &ffmpeg.TranscodeOptionsIn{Fname: fname, Transmuxing: true}
		go func(segUri string, iw *os.File) {
			defer iw.Close()
			glog.V(common.VERBOSE).Infof("Adding manifestID=%s track=%s uri=%s to mp4", manifestID, track, segUri)
			resp, err := http.Get(segUri)
			if err != nil {
				glog.Errorf("Error getting HTTP uri=%s manifestID=%s err=%q", segUri, manifestID, err)
				return
			}
			defer resp.Body.Close()
			if resp.StatusCode != 200 {
				glog.Errorf("Non-200 response for status=%v uri=%s manifestID=%s request=%s", resp.Status, segUri, manifestID, r.URL.String())
				return
			}
			wn, err := io.Copy(iw, resp.Body)
			if err != nil {
				glog.Errorf("Error transmuxing to mp4 request=%s uri=%s manifestID=%s err=%q", r.URL.String(), segUri, manifestID, err)
			}
			atomic.AddInt64(&sourceBytesSent, wn)
		}(seg.URI, iw)

		_, err = tc.Transcode(in, out)
		ir.Close()
		if err != nil {
			glog.Errorf("Error transmuxing to mp4 request=%s uri=%s manifestID=%s err=%q", r.URL.String(), seg.URI, manifestID, err)
			return
		}
	}
}

// HandleRecordings handle requests to /recordings/ endpoint
func (s *LivepeerServer) HandleRecordings(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		glog.Errorf(`/recordings request wrong method=%s url=%s host=%s`, r.Method, r.URL, r.Host)
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	ext := path.Ext(r.URL.Path)
	if ext != ".m3u8" && ext != ".ts" && ext != ".mp4" {
		glog.Errorf(`/recordings request wrong extension=%s url=%s host=%s`, ext, r.URL, r.Host)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	r.URL.Host = r.Host
	if r.URL.Scheme == "" {
		r.URL.Scheme = "http"
	}
	pp := strings.Split(r.URL.Path, "/")
	finalize := r.URL.Query().Get("finalize") == "true"
	_, finalizeSet := r.URL.Query()["finalize"]
	if len(pp) < 4 {
		glog.Errorf(`/recordings request wrong url structure url=%s host=%s`, r.URL, r.Host)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	glog.V(common.VERBOSE).Infof("/recordings request=%s", r.URL.String())
	now := time.Now()
	defer func() {
		glog.V(common.VERBOSE).Infof("request=%s took=%s headers=%+v", r.URL.String(), time.Since(now), w.Header())
	}()
	returnMasterPlaylist := pp[3] == "index.m3u8"
	var track string
	if !returnMasterPlaylist {
		tp := strings.Split(pp[3], ".")
		track = tp[0]
	}
	manifestID := pp[2]
	requestFileName := strings.Join(pp[2:], "/")
	var fromCache bool
	var err error
	var resp *authWebhookResponse
	if cresp, has := s.recordingsAuthResponses.Get(manifestID); has {
		resp = cresp.(*authWebhookResponse)
		fromCache = true
	} else if resp, err = authenticateStream(AuthWebhookURL, r.URL.String()); err != nil {
		glog.Errorf("Authentication denied for url=%s err=%q", r.URL.String(), err)
		if strings.Contains(err.Error(), "not found") {
			w.WriteHeader(http.StatusNotFound)
		} else {
			w.WriteHeader(http.StatusForbidden)
		}
		return
	}
	var sess drivers.OSSession
	ctx := r.Context()
	if resp != nil && !fromCache {
		s.recordingsAuthResponses.SetDefault(manifestID, resp)
	}
	ctx = clog.AddManifestID(ctx, manifestID)

	if resp != nil && resp.RecordObjectStore != "" {
		os, err := drivers.ParseOSURL(resp.RecordObjectStore, true)
		if err != nil {
			clog.Errorf(ctx, "Error parsing OS URL err=%q request url=%s", err, r.URL)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		sess = os.NewSession(manifestID)
	} else if drivers.RecordStorage != nil {
		sess = drivers.RecordStorage.NewSession(manifestID)
	} else {
		clog.Errorf(ctx, "No record object store defined for request url=%s", r.URL)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	startRead := time.Now()
	fi, err := sess.ReadData(ctx, requestFileName)
	if err == context.Canceled {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if err == nil && fi != nil && fi.Body != nil {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Expose-Headers", "Content-Length")
		if ext == ".ts" {
			contentType, _ := common.TypeByExtension(".ts")
			w.Header().Set("Content-Type", contentType)
		} else {
			w.Header().Set("Cache-Control", "max-age=5")
			w.Header().Set("Content-Type", "application/x-mpegURL")
		}
		w.Header().Set("Connection", "keep-alive")
		startWrite := time.Now()
		io.Copy(w, fi.Body)
		fi.Body.Close()
		clog.V(common.VERBOSE).Infof(ctx, "request url=%s streaming filename=%s took=%s from_read_took=%s", r.URL.String(), requestFileName, time.Since(startWrite), time.Since(startRead))
		return
	}
	var manifests []string
	if resp != nil && len(resp.PreviousSessions) > 0 {
		manifests = append(resp.PreviousSessions, manifestID)
	} else {
		manifests = []string{manifestID}
	}
	jsonFilesMap, jsonFiles, latestPlaylistTime, err := getPlaylistsFromStore(ctx, sess, manifests)
	if err != nil {
		clog.Errorf(ctx, "Error getting playlist from store err=%q", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	clog.V(common.VERBOSE).Infof(ctx, "request url=%s found json files: %+v", r.URL, jsonFiles)

	if len(jsonFiles) == 0 {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if time.Since(latestPlaylistTime) > 24*time.Hour && !finalizeSet && ext == ".m3u8" {
		finalize = true
	}

	now1 := time.Now()
	_, datas, err := drivers.ParallelReadFiles(ctx, sess, jsonFiles, 16)
	if err != nil {
		clog.Errorf(ctx, "Error reading files from store err=%q", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	clog.V(common.VERBOSE).Infof(ctx, "Finished reading num=%d playlist files for took=%s", len(jsonFiles), time.Since(now1))

	var jsonPlaylists []*core.JsonPlaylist
	for _, manifestID := range manifests {
		if len(jsonFilesMap[manifestID]) == 0 {
			continue
		}
		// reconstruct sessions
		manifestMainJspl := core.NewJSONPlaylist()
		jsonPlaylists = append(jsonPlaylists, manifestMainJspl)
		for _, i := range jsonFilesMap[manifestID] {
			jspl := &core.JsonPlaylist{}
			err = json.Unmarshal(datas[i], jspl)
			if err != nil {
				clog.Errorf(ctx, "Error unmarshalling json playlist err=%q", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			manifestMainJspl.AddMaster(jspl)
			if finalize {
				for trackName := range jspl.Segments {
					manifestMainJspl.AddTrack(jspl, trackName)
				}
			} else if track != "" {
				manifestMainJspl.AddTrack(jspl, track)
			}
		}
	}
	var mainJspl *core.JsonPlaylist
	if len(jsonPlaylists) == 1 {
		mainJspl = jsonPlaylists[0]
	} else {
		mainJspl = core.NewJSONPlaylist()
		// join sessions
		for _, jspl := range jsonPlaylists {
			mainJspl.AddMaster(jspl)
			if finalize {
				for trackName := range jspl.Segments {
					mainJspl.AddDiscontinuedTrack(jspl, trackName)
				}
			} else if track != "" {
				mainJspl.AddDiscontinuedTrack(jspl, track)
			}
		}
	}
	if ext == ".mp4" {
		if segs, has := mainJspl.Segments[track]; !has || len(segs) == 0 {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		s.streamMP4(w, r, mainJspl, manifestID, track, pp[len(pp)-1])
		return
	}

	masterPList := m3u8.NewMasterPlaylist()
	mediaLists := make(map[string]*m3u8.MediaPlaylist)

	for _, track := range mainJspl.Tracks {
		segments := mainJspl.Segments[track.Name]
		mpl, err := m3u8.NewMediaPlaylist(uint(len(segments)), uint(len(segments)))
		if err != nil {
			clog.Errorf(ctx, "err=%q", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		url := fmt.Sprintf("%s.m3u8", track.Name)
		vParams := m3u8.VariantParams{Bandwidth: track.Bandwidth, Resolution: track.Resolution}
		masterPList.Append(url, mpl, vParams)
		mpl.Live = false
		mediaLists[track.Name] = mpl
	}
	select {
	case <-ctx.Done():
		w.WriteHeader(http.StatusBadRequest)
		return
	default:
	}
	clog.V(common.VERBOSE).Infof(ctx, "Playlist generation for took=%s", time.Since(now1))
	if finalize {
		for trackName := range mainJspl.Segments {
			mpl := mediaLists[trackName]
			mainJspl.AddSegmentsToMPL(manifests, trackName, mpl, resp.RecordObjectStoreURL)
			fileName := trackName + ".m3u8"
			nows := time.Now()
			_, err = sess.SaveData(ctx, fileName, mpl.Encode(), nil, 0)
			clog.V(common.VERBOSE).Infof(ctx, "Saving playlist fileName=%s took=%s", fileName, time.Since(nows))
			if err != nil {
				clog.Errorf(ctx, "Error saving finalized json playlist to store err=%q", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		}
		nows := time.Now()
		_, err = sess.SaveData(ctx, "index.m3u8", masterPList.Encode(), nil, 0)
		clog.V(common.VERBOSE).Infof(ctx, "Saving playlist fileName=%s took=%s", "index.m3u8", time.Since(nows))
		if err != nil {
			clog.Errorf(ctx, "Error saving playlist to store err=%q", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	} else if !returnMasterPlaylist {
		mpl := mediaLists[track]
		if mpl != nil {
			osUrl := ""
			if resp != nil {
				osUrl = resp.RecordObjectStoreURL
			}
			mainJspl.AddSegmentsToMPL(manifests, track, mpl, osUrl)
			// check (debug code)
			startSeq := mpl.Segments[0].SeqId
			for _, seg := range mpl.Segments[1:] {
				if seg.SeqId != startSeq+1 {
					clog.Infof(ctx, "prev seq is %d but next is %d", startSeq, seg.SeqId)
				}
				startSeq = seg.SeqId
			}
		}
	}
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Expose-Headers", "Content-Length")
	w.Header().Set("Cache-Control", "max-age=5")
	w.Header().Set("Content-Type", "application/x-mpegURL")
	if returnMasterPlaylist {
		w.Header().Set("Connection", "keep-alive")
		_, err = w.Write(masterPList.Encode().Bytes())
	} else if track != "" {
		mediaPl := mediaLists[track]
		if mediaPl != nil {
			w.Header().Set("Connection", "keep-alive")
			_, err = w.Write(mediaPl.Encode().Bytes())
		} else {
			w.WriteHeader(http.StatusNotFound)
			return
		}
	} else {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
}

//Helper Methods Begin

// StreamPrefix match all leading spaces, slashes and optionally `stream/`
var StreamPrefix = regexp.MustCompile(`^[ /]*(stream/)?|(live/)?`) // test carefully!

func cleanStreamPrefix(reqPath string) string {
	return StreamPrefix.ReplaceAllString(reqPath, "")
}

func parseStreamID(reqPath string) core.StreamID {
	// remove extension and create streamid
	p := strings.TrimSuffix(reqPath, path.Ext(reqPath))
	return core.SplitStreamIDString(cleanStreamPrefix(p))
}

func parseManifestID(reqPath string) core.ManifestID {
	return parseStreamID(reqPath).ManifestID
}

func parsePresets(presets []string) []ffmpeg.VideoProfile {
	profs := make([]ffmpeg.VideoProfile, 0)
	for _, v := range presets {
		if p, ok := ffmpeg.VideoProfileLookup[strings.TrimSpace(v)]; ok {
			profs = append(profs, p)
		}
	}
	return profs
}

func (s *LivepeerServer) LastManifestID() core.ManifestID {
	s.connectionLock.RLock()
	defer s.connectionLock.RUnlock()
	return s.lastManifestID
}

func (s *LivepeerServer) LastHLSStreamID() core.StreamID {
	s.connectionLock.RLock()
	defer s.connectionLock.RUnlock()
	return s.lastHLSStreamID
}

func (s *LivepeerServer) GetNodeStatus() *common.NodeStatus {
	// not threadsafe; need to deep copy the playlist
	m := make(map[string]*m3u8.MasterPlaylist)

	s.connectionLock.RLock()
	defer s.connectionLock.RUnlock()
	streamInfo := make(map[string]common.StreamInfo)
	for _, cxn := range s.rtmpConnections {
		if cxn.pl == nil {
			continue
		}
		cpl := cxn.pl
		m[string(cpl.ManifestID())] = cpl.GetHLSMasterPlaylist()
		sb := atomic.LoadUint64(&cxn.sourceBytes)
		tb := atomic.LoadUint64(&cxn.transcodedBytes)
		streamInfo[string(cpl.ManifestID())] = common.StreamInfo{
			SourceBytes:     sb,
			TranscodedBytes: tb,
		}
	}
	res := &common.NodeStatus{
		Manifests:             m,
		InternalManifests:     make(map[string]string),
		StreamInfo:            streamInfo,
		Version:               core.LivepeerVersion,
		GolangRuntimeVersion:  runtime.Version(),
		GOArch:                runtime.GOARCH,
		GOOS:                  runtime.GOOS,
		OrchestratorPool:      []string{},
		RegisteredTranscoders: []common.RemoteTranscoderInfo{},
		LocalTranscoding:      s.LivepeerNode.TranscoderManager == nil,
		BroadcasterPrices:     make(map[string]*big.Rat),
	}
	for k, v := range s.internalManifests {
		res.InternalManifests[string(k)] = string(v)
	}
	if s.LivepeerNode.TranscoderManager != nil {
		res.RegisteredTranscodersNumber = s.LivepeerNode.TranscoderManager.RegisteredTranscodersCount()
		res.RegisteredTranscoders = s.LivepeerNode.TranscoderManager.RegisteredTranscodersInfo()
	}
	if s.LivepeerNode.OrchestratorPool != nil {
		infos := s.LivepeerNode.OrchestratorPool.GetInfos()
		res.OrchestratorPoolInfos = infos
		for _, info := range infos {
			res.OrchestratorPool = append(res.OrchestratorPool, info.URL.String())
		}
	}

	res.BroadcasterPrices = s.LivepeerNode.GetBasePrices()

	return res
}

// Debug helpers
func (s *LivepeerServer) LatestPlaylist() core.PlaylistManager {
	s.connectionLock.RLock()
	defer s.connectionLock.RUnlock()
	cxn, ok := s.getActiveRtmpConnectionUnsafe(s.lastManifestID)
	if !ok || cxn.pl == nil {
		return nil
	}
	return cxn.pl
}

func shouldStopStream(err error) bool {
	_, ok := err.(pm.ErrSenderValidation)
	return ok
}

func getRemoteAddr(r *http.Request) string {
	addr := r.RemoteAddr
	if proxiedAddr := r.Header.Get("X-Forwarded-For"); proxiedAddr != "" {
		addr = strings.Split(proxiedAddr, ",")[0]
	}
	return strings.Split(addr, ":")[0]
}

func mediaCompatible(a, b ffmpeg.MediaFormatInfo) bool {
	return a.Acodec == b.Acodec &&
		a.Vcodec == b.Vcodec &&
		a.PixFormat == b.PixFormat &&
		a.Width == b.Width &&
		a.Height == b.Height

	// NB: there is also a Format field but that does
	// not need to match since transcoder will reopen
	// a new demuxer each time
}
