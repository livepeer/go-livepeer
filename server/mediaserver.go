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
	"math/rand"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"path"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/livepeer/go-livepeer/drivers"
	"github.com/livepeer/go-livepeer/monitor"
	"github.com/livepeer/go-livepeer/net"
	"github.com/livepeer/go-livepeer/pm"

	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/core"
	lpmscore "github.com/livepeer/lpms/core"
	ffmpeg "github.com/livepeer/lpms/ffmpeg"
	"github.com/livepeer/lpms/segmenter"
	"github.com/livepeer/lpms/stream"
	"github.com/livepeer/lpms/vidplayer"
	"github.com/livepeer/m3u8"
)

var errAlreadyExists = errors.New("StreamAlreadyExists")
var errBroadcast = errors.New("ErrBroadcast")
var errLowDeposit = errors.New("ErrLowDeposit")
var errStorage = errors.New("ErrStorage")
var errDiscovery = errors.New("ErrDiscovery")
var errNoOrchs = errors.New("ErrNoOrchs")
var errUnknownStream = errors.New("ErrUnknownStream")
var errMismatchedParams = errors.New("Mismatched type for stream params")

const HLSWaitInterval = time.Second
const HLSBufferCap = uint(43200) //12 hrs assuming 1s segment
const HLSBufferWindow = uint(5)
const StreamKeyBytes = 6

const SegLen = 2 * time.Second
const BroadcastRetry = 15 * time.Second

var BroadcastJobVideoProfiles = []ffmpeg.VideoProfile{ffmpeg.P240p30fps4x3, ffmpeg.P360p30fps16x9}

var AuthWebhookURL string

var refreshIntervalHttpPush = 1 * time.Minute

type streamParameters struct {
	mid        core.ManifestID
	rtmpKey    string
	profiles   []ffmpeg.VideoProfile
	resolution string
	format     ffmpeg.Format
}

func (s *streamParameters) StreamID() string {
	return string(s.mid) + "/" + s.rtmpKey
}

type rtmpConnection struct {
	mid         core.ManifestID
	nonce       uint64
	stream      stream.RTMPVideoStream
	pl          core.PlaylistManager
	profile     *ffmpeg.VideoProfile
	params      *streamParameters
	sessManager *BroadcastSessionsManager
	lastUsed    time.Time
}

type LivepeerServer struct {
	RTMPSegmenter         lpmscore.RTMPSegmenter
	LPMS                  *lpmscore.LPMS
	LivepeerNode          *core.LivepeerNode
	HTTPMux               *http.ServeMux
	ExposeCurrentManifest bool

	// Thread sensitive fields. All accesses to the
	// following fields should be protected by `connectionLock`
	rtmpConnections map[core.ManifestID]*rtmpConnection
	lastHLSStreamID core.StreamID
	lastManifestID  core.ManifestID
	connectionLock  *sync.RWMutex
}

type authWebhookResponse struct {
	ManifestID string   `json:"manifestID"`
	StreamKey  string   `json:"streamKey"`
	Presets    []string `json:"presets"`
	Profiles   []struct {
		Name    string `json:"name"`
		Width   int    `json:"width"`
		Height  int    `json:"height"`
		Bitrate int    `json:"bitrate"`
		FPS     uint   `json:"fps"`
	} `json:"profiles"`
}

func NewLivepeerServer(rtmpAddr string, lpNode *core.LivepeerNode, httpIngest bool) *LivepeerServer {
	opts := lpmscore.LPMSOpts{
		RtmpAddr:     rtmpAddr,
		RtmpDisabled: true,
		WorkDir:      lpNode.WorkDir,
		HttpMux:      http.NewServeMux(),
	}
	switch lpNode.NodeType {
	case core.BroadcasterNode:
		opts.RtmpDisabled = false
	}
	server := lpmscore.New(&opts)
	ls := &LivepeerServer{RTMPSegmenter: server, LPMS: server, LivepeerNode: lpNode, HTTPMux: opts.HttpMux, connectionLock: &sync.RWMutex{},
		rtmpConnections: make(map[core.ManifestID]*rtmpConnection),
	}
	if lpNode.NodeType == core.BroadcasterNode && httpIngest {
		opts.HttpMux.HandleFunc("/live/", ls.HandlePush)
	}
	return ls
}

//StartMediaServer starts the LPMS server
func (s *LivepeerServer) StartMediaServer(ctx context.Context, transcodingOptions string, httpAddr string) error {
	BroadcastJobVideoProfiles = parsePresets(strings.Split(transcodingOptions, ","))

	glog.V(common.SHORT).Infof("Transcode Job Type: %v", BroadcastJobVideoProfiles)

	//LPMS handlers for handling RTMP video
	s.LPMS.HandleRTMPPublish(createRTMPStreamIDHandler(s), gotRTMPStreamHandler(s), endRTMPStreamHandler(s))
	s.LPMS.HandleRTMPPlay(getRTMPStreamHandler(s))

	//LPMS hanlder for handling HLS video play
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

//RTMP Publish Handlers
func createRTMPStreamIDHandler(s *LivepeerServer) func(url *url.URL) (strmID stream.AppData) {
	return func(url *url.URL) (strmID stream.AppData) {
		//Check webhook for ManifestID
		//If ManifestID is returned from webhook, use it
		//Else check URL for ManifestID
		//If ManifestID is passed in URL, use that one
		//Else create one
		var resp *authWebhookResponse
		var mid core.ManifestID
		var err error
		var key string
		profiles := []ffmpeg.VideoProfile{}
		if resp, err = authenticateStream(url.String()); err != nil {
			glog.Error("Authentication denied for ", err)
			return nil
		}
		if resp != nil {
			mid, key = parseManifestID(resp.ManifestID), resp.StreamKey
			// Process transcoding options presets
			if len(resp.Presets) > 0 {
				profiles = parsePresets(resp.Presets)
			}

			for _, profile := range resp.Profiles {
				name := profile.Name
				if name == "" {
					name = "webhook_" + common.DefaultProfileName(
						profile.Width,
						profile.Height,
						profile.Bitrate)
				}
				prof := ffmpeg.VideoProfile{
					Name:       name,
					Bitrate:    fmt.Sprint(profile.Bitrate),
					Framerate:  profile.FPS,
					Resolution: fmt.Sprintf("%dx%d", profile.Width, profile.Height),
				}
				profiles = append(profiles, prof)
			}

			// Only set defaults if user did not specify a preset/profile
			if len(resp.Profiles) <= 0 && len(resp.Presets) <= 0 {
				profiles = BroadcastJobVideoProfiles
			}
		} else {
			profiles = BroadcastJobVideoProfiles
		}

		if mid == "" {
			sid := parseStreamID(url.Path)
			mid, key = sid.ManifestID, sid.Rendition
		}
		if mid == "" {
			mid = core.RandomManifestID()
		}

		// Ensure there's no concurrent StreamID with the same name
		s.connectionLock.RLock()
		defer s.connectionLock.RUnlock()
		if core.MaxSessions > 0 && len(s.rtmpConnections) >= core.MaxSessions {
			glog.Error("Too many connections")
			return nil
		}
		if _, exists := s.rtmpConnections[mid]; exists {
			glog.Error("Manifest already exists ", mid)
			return nil
		}

		// Generate RTMP part of StreamID
		if key == "" {
			key = common.RandomIDGenerator(StreamKeyBytes)
		}
		return &streamParameters{
			mid:     mid,
			rtmpKey: key,
			// HTTP push mutates `profiles` so make a copy of it
			profiles: append([]ffmpeg.VideoProfile(nil), profiles...),
		}
	}
}

func authenticateStream(url string) (*authWebhookResponse, error) {
	if AuthWebhookURL == "" {
		return nil, nil
	}
	started := time.Now()
	values := map[string]string{"url": url}
	jsonValue, err := json.Marshal(values)
	if err != nil {
		return nil, err
	}
	resp, err := http.Post(AuthWebhookURL, "application/json", bytes.NewBuffer(jsonValue))

	if err != nil {
		return nil, err
	}
	rbody, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, errors.New(resp.Status)
	}
	if len(rbody) == 0 {
		return nil, nil
	}
	var authResp authWebhookResponse
	err = json.Unmarshal(rbody, &authResp)
	if err != nil {
		return nil, err
	}
	if authResp.ManifestID == "" {
		return nil, errors.New("Empty manifest id not allowed")
	}
	took := time.Since(started)
	glog.Infof("Stream authentication for url=%s dur=%s", url, took)
	if monitor.Enabled {
		monitor.AuthWebhookFinished(took)
	}
	return &authResp, nil
}

func streamParams(rtmpStrm stream.RTMPVideoStream) *streamParameters {
	d := rtmpStrm.AppData()
	p, ok := d.(*streamParameters)
	if !ok {
		glog.Error("Mismatched type for RTMP app data")
		return nil
	}
	return p
}

func gotRTMPStreamHandler(s *LivepeerServer) func(url *url.URL, rtmpStrm stream.RTMPVideoStream) (err error) {
	return func(url *url.URL, rtmpStrm stream.RTMPVideoStream) (err error) {

		cxn, err := s.registerConnection(rtmpStrm)
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
				if streamStarted == false {
					streamStarted = true
					if monitor.Enabled {
						monitor.StreamStarted(nonce)
					}
				}
				go processSegment(cxn, seg)
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

		if monitor.Enabled {
			monitor.StreamCreated(string(mid), nonce)
		}

		glog.Infof("\n\nVideo Created With ManifestID: %v\n\n", mid)

		return nil
	}
}

func endRTMPStreamHandler(s *LivepeerServer) func(url *url.URL, rtmpStrm stream.RTMPVideoStream) error {
	return func(url *url.URL, rtmpStrm stream.RTMPVideoStream) error {
		params := streamParams(rtmpStrm)
		if params == nil {
			return errMismatchedParams
		}

		//Remove RTMP stream
		err := removeRTMPStream(s, params.mid)
		if err != nil {
			return err
		}
		return nil
	}
}

func (s *LivepeerServer) registerConnection(rtmpStrm stream.RTMPVideoStream) (*rtmpConnection, error) {
	nonce := rand.Uint64()

	// Set up the connection tracking
	params := streamParams(rtmpStrm)
	if params == nil {
		return nil, errMismatchedParams
	}
	mid := params.mid
	if drivers.NodeStorage == nil {
		glog.Error("Missing node storage")
		return nil, errStorage
	}
	storage := drivers.NodeStorage.NewSession(string(mid))
	// Build the source video profile from the RTMP stream.
	if params.resolution == "" {
		params.resolution = fmt.Sprintf("%vx%v", rtmpStrm.Width(), rtmpStrm.Height())
	}
	vProfile := ffmpeg.VideoProfile{
		Name:       "source",
		Resolution: params.resolution,
		Bitrate:    "4000k", // Fix this
		Format:     params.format,
	}
	hlsStrmID := core.MakeStreamID(mid, &vProfile)
	s.connectionLock.RLock()
	// Fast path - check early if session exists - creating new session can take time
	_, exists := s.rtmpConnections[mid]
	s.connectionLock.RUnlock()
	if exists {
		// We can only have one concurrent stream per ManifestID
		return nil, errAlreadyExists
	}

	playlist := core.NewBasicPlaylistManager(mid, storage)
	var stakeRdr stakeReader
	if s.LivepeerNode.Eth != nil {
		stakeRdr = &storeStakeReader{store: s.LivepeerNode.Database}
	}
	cxn := &rtmpConnection{
		mid:         mid,
		nonce:       nonce,
		stream:      rtmpStrm,
		pl:          playlist,
		profile:     &vProfile,
		params:      params,
		sessManager: NewSessionManager(s.LivepeerNode, params, playlist, NewMinLSSelector(stakeRdr, 1.0)),
		lastUsed:    time.Now(),
	}

	s.connectionLock.Lock()
	_, exists = s.rtmpConnections[mid]
	// Check if session exist again - potentially two sessions can be created simultaneously,
	// so we don't want to overwrite one that was already created
	if exists {
		// We can only have one concurrent stream per ManifestID
		s.connectionLock.Unlock()
		return nil, errAlreadyExists
	}
	s.rtmpConnections[mid] = cxn
	s.lastManifestID = mid
	s.lastHLSStreamID = hlsStrmID
	sessionsNumber := len(s.rtmpConnections)
	s.connectionLock.Unlock()

	if monitor.Enabled {
		monitor.CurrentSessions(sessionsNumber)
	}

	return cxn, nil
}

func removeRTMPStream(s *LivepeerServer, mid core.ManifestID) error {
	s.connectionLock.Lock()
	defer s.connectionLock.Unlock()
	cxn, ok := s.rtmpConnections[mid]
	if !ok || cxn.pl == nil {
		glog.Error("Attempted to end unknown stream with manifest ID ", mid)
		return errUnknownStream
	}
	cxn.stream.Close()
	cxn.sessManager.cleanup()
	cxn.pl.Cleanup()
	glog.Infof("Ended stream with id=%s", mid)
	delete(s.rtmpConnections, mid)

	if monitor.Enabled {
		monitor.StreamEnded(cxn.nonce)
		monitor.CurrentSessions(len(s.rtmpConnections))
	}

	return nil
}

//End RTMP Publish Handlers

//HLS Play Handlers
func getHLSMasterPlaylistHandler(s *LivepeerServer) func(url *url.URL) (*m3u8.MasterPlaylist, error) {
	return func(url *url.URL) (*m3u8.MasterPlaylist, error) {
		var manifestID core.ManifestID
		if s.ExposeCurrentManifest && "/stream/current.m3u8" == strings.ToLower(url.Path) {
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
		cxn, ok := s.rtmpConnections[manifestID]
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
		cxn, ok := s.rtmpConnections[mid]
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

//Start RTMP Play Handlers
func getRTMPStreamHandler(s *LivepeerServer) func(url *url.URL) (stream.RTMPVideoStream, error) {
	return func(url *url.URL) (stream.RTMPVideoStream, error) {
		mid := parseManifestID(url.Path)
		s.connectionLock.RLock()
		cxn, ok := s.rtmpConnections[mid]
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

// HandlePush processes request for HTTP ingest
func (s *LivepeerServer) HandlePush(w http.ResponseWriter, r *http.Request) {
	// we read this unconditionally, mostly for ffmpeg
	body, err := ioutil.ReadAll(r.Body)

	if err != nil {
		httpErr := fmt.Sprintf(`Error reading http request body: %s`, err.Error())
		glog.Error(httpErr)
		http.Error(w, httpErr, http.StatusInternalServerError)
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
		http.Error(w, fmt.Sprintf(`ignoring file extension: %s`, ext), http.StatusBadRequest)
		return
	}
	glog.Infof("Got push request at url=%s ua=%s addr=%s len=%d", r.URL.String(), r.UserAgent(), r.RemoteAddr, len(body))

	now := time.Now()
	mid := parseManifestID(r.URL.Path)
	if mid == "" {
		http.Error(w, `Bad URL`, http.StatusBadRequest)
		return
	}
	s.connectionLock.Lock()
	cxn, exists := s.rtmpConnections[mid]
	if exists && cxn != nil {
		cxn.lastUsed = now
	}
	s.connectionLock.Unlock()

	// Check for presence and register if a fresh cxn
	if !exists {
		appData := (createRTMPStreamIDHandler(s))(r.URL)
		if appData == nil {
			http.Error(w, "Could not create stream ID: ", http.StatusInternalServerError)
			return
		}
		st := stream.NewBasicRTMPVideoStream(appData)
		params := streamParams(st)
		params.resolution = r.Header.Get("Content-Resolution")
		params.format = format
		// Set output formats if not explicitly specified
		for i, v := range params.profiles {
			if ffmpeg.FormatNone == v.Format {
				params.profiles[i].Format = format
			}
		}

		cxn, err = s.registerConnection(st)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		ticker := time.NewTicker(refreshIntervalHttpPush)

		go func(s *LivepeerServer, mid core.ManifestID) {
			defer ticker.Stop()
			for range ticker.C {
				var lastUsed time.Time
				s.connectionLock.RLock()
				if cxn, exists := s.rtmpConnections[mid]; exists {
					lastUsed = cxn.lastUsed
				}
				s.connectionLock.RUnlock()
				if time.Since(lastUsed) > refreshIntervalHttpPush {
					_ = removeRTMPStream(s, mid)
					return
				}
			}
		}(s, mid)
	}

	fname := path.Base(r.URL.Path)
	seq, err := strconv.ParseUint(strings.TrimSuffix(fname, ext), 10, 64)
	if err != nil {
		seq = 0
	}

	duration, err := strconv.Atoi(r.Header.Get("Content-Duration"))
	if err != nil {
		duration = 2000
		glog.Info("Missing duration; filling in a default of 2000ms")
	}

	seg := &stream.HLSSegment{
		Data:     body,
		Name:     fname,
		SeqNo:    seq,
		Duration: float64(duration) / 1000.0,
	}

	// Do the transcoding!
	urls, err := processSegment(cxn, seg)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if len(urls) == 0 {
		http.Error(w, "No sessions available", http.StatusServiceUnavailable)
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

	boundary := common.RandName()
	accept := r.Header.Get("Accept")
	if accept == "multipart/mixed" {
		contentType := "multipart/mixed; boundary=" + boundary
		w.Header().Set("Content-Type", contentType)
	}
	w.WriteHeader(http.StatusOK)
	w.(http.Flusher).Flush()
	if accept != "multipart/mixed" {
		return
	}
	mw := multipart.NewWriter(w)
	for i, url := range urls {
		mw.SetBoundary(boundary)
		var typ, ext string
		length := len(renditionData[i])
		if length == 0 {
			typ, ext, length = "application/vnd+livepeer.uri", ".txt", len(url)
		} else {
			format := cxn.params.profiles[i].Format
			ext, err = common.ProfileFormatExtension(format)
			if err != nil {
				glog.Error("Unknown extension for format: ", err)
				break
			}
			typ, err = common.ProfileFormatMimeType(format)
			if err != nil {
				glog.Error("Unknown mime type for format: ", err)
			}
		}
		profile := cxn.params.profiles[i].Name
		fname := fmt.Sprintf(`"%s_%d%s"`, profile, seq, ext)
		hdrs := textproto.MIMEHeader{
			"Content-Type":        {typ + "; name=" + fname},
			"Content-Length":      {strconv.Itoa(length)},
			"Content-Disposition": {"attachment; filename=" + fname},
			"Rendition-Name":      {profile},
		}
		fw, err := mw.CreatePart(hdrs)
		if err != nil {
			glog.Error("Could not create multipart part ", err)
			break
		}
		if len(renditionData[i]) > 0 {
			io.Copy(fw, bytes.NewBuffer(renditionData[i]))
		} else {
			fw.Write([]byte(url))
		}
	}
	mw.Close()
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

func (s *LivepeerServer) GetNodeStatus() *net.NodeStatus {
	// not threadsafe; need to deep copy the playlist
	m := make(map[string]*m3u8.MasterPlaylist, 0)

	s.connectionLock.RLock()
	defer s.connectionLock.RUnlock()
	for _, cxn := range s.rtmpConnections {
		if cxn.pl == nil {
			continue
		}
		cpl := cxn.pl
		m[string(cpl.ManifestID())] = cpl.GetHLSMasterPlaylist()
	}
	res := &net.NodeStatus{
		Manifests:             m,
		Version:               core.LivepeerVersion,
		GolangRuntimeVersion:  runtime.Version(),
		GOArch:                runtime.GOARCH,
		GOOS:                  runtime.GOOS,
		OrchestratorPool:      []string{},
		RegisteredTranscoders: []net.RemoteTranscoderInfo{},
		LocalTranscoding:      s.LivepeerNode.TranscoderManager == nil,
	}
	if s.LivepeerNode.TranscoderManager != nil {
		res.RegisteredTranscodersNumber = s.LivepeerNode.TranscoderManager.RegisteredTranscodersCount()
		res.RegisteredTranscoders = s.LivepeerNode.TranscoderManager.RegisteredTranscodersInfo()
	}
	if s.LivepeerNode.OrchestratorPool != nil {
		urls := s.LivepeerNode.OrchestratorPool.GetURLs()
		for _, url := range urls {
			res.OrchestratorPool = append(res.OrchestratorPool, url.String())
		}
	}
	return res
}

// Debug helpers
func (s *LivepeerServer) LatestPlaylist() core.PlaylistManager {
	s.connectionLock.RLock()
	defer s.connectionLock.RUnlock()
	cxn, ok := s.rtmpConnections[s.lastManifestID]
	if !ok || cxn.pl == nil {
		return nil
	}
	return cxn.pl
}

func shouldStopStream(err error) bool {
	_, ok := err.(pm.ErrSenderValidation)
	return ok
}
