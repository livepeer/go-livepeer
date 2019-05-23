/*
Package server is the place we integrate the Livepeer node with the LPMS media server.
*/
package server

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math/big"
	"math/rand"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/swarm/api/client"
	"github.com/ethereum/go-ethereum/swarm/storage/feed"
	"github.com/ethereum/go-ethereum/swarm/storage/feed/lookup"
	"github.com/livepeer/go-livepeer/drivers"
	"github.com/livepeer/go-livepeer/eth"
	"github.com/livepeer/go-livepeer/monitor"
	"github.com/livepeer/go-livepeer/net"

	"github.com/ericxtang/m3u8"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/core"
	lpmscore "github.com/livepeer/lpms/core"
	"github.com/livepeer/lpms/ffmpeg"
	"github.com/livepeer/lpms/segmenter"
	"github.com/livepeer/lpms/stream"
	"github.com/livepeer/lpms/vidplayer"
)

var errAlreadyExists = errors.New("StreamAlreadyExists")
var errBroadcast = errors.New("ErrBroadcast")
var errLowDeposit = errors.New("ErrLowDeposit")
var errStorage = errors.New("ErrStorage")
var errDiscovery = errors.New("ErrDiscovery")
var errNoOrchs = errors.New("ErrNoOrchs")
var errUnknownStream = errors.New("ErrUnknownStream")
var errPMCheckFailed = errors.New("PM Check Failed")

const HLSWaitInterval = time.Second
const HLSBufferCap = uint(43200) //12 hrs assuming 1s segment
const HLSBufferWindow = uint(5)
const StreamKeyBytes = 6

const SegLen = 4 * time.Second
const BroadcastRetry = 15 * time.Second

var BroadcastPrice = big.NewInt(1)
var BroadcastJobVideoProfiles = []ffmpeg.VideoProfile{ffmpeg.P240p30fps4x3, ffmpeg.P360p30fps16x9}

var AuthWebhookURL string
var SwarmPlaylists = map[string]core.PlaylistManager{}

type rtmpConnection struct {
	mid         core.ManifestID
	nonce       uint64
	stream      stream.RTMPVideoStream
	pl          core.PlaylistManager
	profile     *ffmpeg.VideoProfile
	sessManager *BroadcastSessionsManager
}

type LivepeerServer struct {
	RTMPSegmenter lpmscore.RTMPSegmenter
	LPMS          *lpmscore.LPMS
	LivepeerNode  *core.LivepeerNode
	HTTPMux       *http.ServeMux

	ExposeCurrentManifest bool

	// Thread sensitive fields. All accesses to the
	// following fields should be protected by `connectionLock`
	rtmpConnections map[core.ManifestID]*rtmpConnection
	lastHLSStreamID core.StreamID
	lastManifestID  core.ManifestID
	connectionLock  *sync.RWMutex

	//Swarm Integration
	bzzApi         string
	publishTopic   string
	swarmEthClient eth.LivepeerEthClient
}

type authWebhookResponse struct {
	ManifestID string `json:"manifestID"`
	StreamKey  string `json:"streamKey"`
}

type swarmFeedEntry struct {
	PlData string `json:"plData"`
	SegLoc string `json:"segLoc"`
	SeqNo  uint64 `json:"seqNo"`
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
	return &LivepeerServer{RTMPSegmenter: server, LPMS: server, LivepeerNode: lpNode, HTTPMux: opts.HttpMux, connectionLock: &sync.RWMutex{}, rtmpConnections: make(map[core.ManifestID]*rtmpConnection)}
}

func (s *LivepeerServer) ConfigSwarm(bzzApi string, publishTopic string, client eth.LivepeerEthClient) {
	s.bzzApi = bzzApi
	s.publishTopic = publishTopic
	s.swarmEthClient = client
}

//StartServer starts the LPMS server
func (s *LivepeerServer) StartMediaServer(ctx context.Context, transcodingOptions string) error {
	bProfiles := make([]ffmpeg.VideoProfile, 0)
	for _, opt := range strings.Split(transcodingOptions, ",") {
		p, ok := ffmpeg.VideoProfileLookup[strings.TrimSpace(opt)]
		if ok {
			bProfiles = append(bProfiles, p)
		}
	}
	BroadcastJobVideoProfiles = bProfiles

	glog.V(common.SHORT).Infof("Transcode Job Type: %v", BroadcastJobVideoProfiles)

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
		//Check webhook for ManifestID
		//If ManifestID is returned from webhook, use it
		//Else check URL for ManifestID
		//If ManifestID is passed in URL, use that one
		//Else create one
		var resp *authWebhookResponse
		var mid core.ManifestID
		var err error
		var key string
		if resp, err = authenticateStream(url.String()); err != nil {
			glog.Error("Authentication denied for ", err)
			return ""
		}
		if resp != nil {
			mid, key = parseStreamID(resp.ManifestID).ManifestID, resp.StreamKey
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
			return ""
		}
		if _, exists := s.rtmpConnections[mid]; exists {
			glog.Error("Manifest already exists ", mid)
			return ""
		}

		// Generate RTMP part of StreamID
		if key == "" {
			key = hex.EncodeToString(core.RandomIdGenerator(StreamKeyBytes))
		}
		return core.MakeStreamIDFromString(string(mid), key).String()
	}
}

func authenticateStream(url string) (*authWebhookResponse, error) {
	if AuthWebhookURL == "" {
		return nil, nil
	}

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
	return &authResp, nil
}

func rtmpManifestID(rtmpStrm stream.RTMPVideoStream) core.ManifestID {
	return parseManifestID(rtmpStrm.GetStreamID())
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
				processSegment(cxn, seg)
				if s.bzzApi != "" && s.publishTopic != "" {
					userAcct := s.swarmEthClient.Account().Address
					signer, _ := s.swarmEthClient.GetSigner()
					glog.Infof("Publish %v to Swarm: user: %v, topic: %v", seg.Name, userAcct.String(), s.publishTopic)
					go publishToSwarmFeed(s.bzzApi, userAcct, s.publishTopic, signer, seg.SeqNo, seg.Name)
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

		if monitor.Enabled {
			monitor.StreamCreated(string(mid), nonce)
		}

		glog.Infof("\n\nVideo Created With ManifestID: %v\n\n", mid)

		return nil
	}
}

func endRTMPStreamHandler(s *LivepeerServer) func(url *url.URL, rtmpStrm stream.RTMPVideoStream) error {
	return func(url *url.URL, rtmpStrm stream.RTMPVideoStream) error {
		mid := rtmpManifestID(rtmpStrm)
		//Remove RTMP stream
		s.connectionLock.Lock()
		defer s.connectionLock.Unlock()
		cxn, ok := s.rtmpConnections[mid]
		if !ok || cxn.pl == nil {
			glog.Error("Attempted to end unknown stream with manifest ID ", mid)
			return errUnknownStream
		}
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
}

func (s *LivepeerServer) registerConnection(rtmpStrm stream.RTMPVideoStream) (*rtmpConnection, error) {
	nonce := rand.Uint64()

	// If running in on-chain mode, check for a reasonable deposit
	if s.LivepeerNode.Eth != nil {
		//Comment this out for now - ticket broker isn't deployed on a testnet anywhere.
		// sender, err := s.LivepeerNode.Eth.Senders(s.LivepeerNode.Eth.Account().Address)
		// if err != nil {
		// 	return nil, err
		// }

		// if sender.Deposit.Cmp(big.NewInt(0)) <= 0 {
		// 	glog.Errorf("No deposit - cannot start broadcast session")

		// 	if monitor.Enabled {
		// 		monitor.StreamCreateFailed(nonce, "LowDeposit")
		// 	}

		// 	return nil, errLowDeposit
		// }
	}

	// Set up the connection tracking
	mid := rtmpManifestID(rtmpStrm)
	if drivers.NodeStorage == nil {
		glog.Error("Missing node storage")
		return nil, errStorage
	}
	storage := drivers.NodeStorage.NewSession(string(mid))
	// Build the source video profile from the RTMP stream.
	resolution := fmt.Sprintf("%vx%v", rtmpStrm.Width(), rtmpStrm.Height())
	vProfile := ffmpeg.VideoProfile{
		Name:       "source",
		Resolution: resolution,
		Bitrate:    "4000k", // Fix this
	}
	hlsStrmID := core.MakeStreamID(mid, &vProfile)
	s.connectionLock.Lock()
	_, exists := s.rtmpConnections[mid]
	s.connectionLock.Unlock()
	if exists {
		// We can only have one concurrent stream per ManifestID
		return nil, errAlreadyExists
	}

	playlist := core.NewBasicPlaylistManager(mid, storage)
	cxn := &rtmpConnection{
		mid:         mid,
		nonce:       nonce,
		stream:      rtmpStrm,
		pl:          playlist,
		profile:     &vProfile,
		sessManager: NewSessionManager(s.LivepeerNode, playlist),
	}
	s.connectionLock.Lock()
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

//End RTMP Publish Handlers

//HLS Play Handlers
func getHLSMasterPlaylistHandler(s *LivepeerServer) func(url *url.URL) (*m3u8.MasterPlaylist, error) {
	return func(url *url.URL) (*m3u8.MasterPlaylist, error) {
		var manifestID core.ManifestID
		if s.ExposeCurrentManifest && "/stream/current.m3u8" == strings.ToLower(url.Path) {
			manifestID = s.LastManifestID()
		} else if isSwarmRequest(url.Path) {
			user, topic, swarmManifestID, manifestID, rendition := parseSwarmParams(url.String())
			if rendition == "" { //is master playlist if rendition is empty
				return loadMPLFromSwarm(s.bzzApi, user, topic, swarmManifestID, manifestID)
			} else {
				return nil, vidplayer.ErrNotFound
			}
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
		if isSwarmRequest(url.Path) {
			user, topic, swarmManifestID, _, rendition := parseSwarmParams(url.String())
			if rendition != "" {
				return loadPlFromSwarm(s.bzzApi, user, topic, swarmManifestID, rendition)
			} else {
				return nil, vidplayer.ErrNotFound
			}
		} else {
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

//Helper Methods Begin

// Match all leading spaces, slashes and optionally `stream/`
var StreamPrefix = regexp.MustCompile(`^[ /]*(stream/)?`)
var SwarmPrefix = regexp.MustCompile(`^[/]*(stream/swarm.m3u8\?)?`)

func cleanStreamPrefix(reqPath string) string {
	return StreamPrefix.ReplaceAllString(reqPath, "")
}

func isSwarmRequest(reqPath string) bool {
	return SwarmPrefix.MatchString(reqPath)
}

func parseSwarmParams(reqUrl string) (user string, topic string, swarmManifestID string, manifestID string, rendition string) {
	truncated := strings.Replace(reqUrl, ".m3u8", "", 1)
	split := strings.Split(truncated, "/")
	isKey := func(s string) bool {
		if s == "user" || s == "topic" || s == "manifestID" {
			return true
		}
		return false
	}

	result := map[string]string{}
	key := ""
	for _, s := range split {
		if isKey(s) {
			key = s
		} else if key != "" {
			result[key] = s
			key = ""
		} else {
			//do nothing
		}

		lookupSuccess := ffmpeg.VideoProfileLookup[s] != ffmpeg.VideoProfile{}
		if lookupSuccess {
			rendition = s
		}
	}
	user = result["user"]
	topic = result["topic"]
	swarmManifestID = result["manifestID"]
	manifestID = strings.Replace(truncated, "/stream/", "", 1) //used to create playlist

	// p, _ := url.ParseQuery(qStr)
	// if p != nil {
	// 	user = p.Get("user")
	// 	topic = p.Get("topic")
	// 	manifestID = p.Get("manifestID")
	// }
	return
}

func loadMPLFromSwarm(bzzApi, user, topic, swarmManifestID, manifestID string) (*m3u8.MasterPlaylist, error) {
	c := client.NewClient(bzzApi)
	f := new(feed.Feed)
	f.User = ethcommon.HexToAddress(user)
	var err error
	f.Topic, err = feed.NewTopic(topic, nil)
	if err != nil {
		glog.Infof("Error creating new swarm topic: %v", err)
	}
	query := feed.NewQueryLatest(f, lookup.NoClue)
	_, err = c.QueryFeed(query, "")
	if err != nil {
		fmt.Printf("Cannot query feed: %v", err)
		return nil, err
	}

	fetchFromSwarm := func(pl core.PlaylistManager, query *feed.Query) {
		lastEntry := ""
		for {
			reader, err := c.QueryFeed(query, "")
			if err != nil {
				fmt.Printf("Cannot query feed: %v", err)
				return
			}
			feed, err := ioutil.ReadAll(reader)
			if err != nil {
				fmt.Printf("Cannot read feed: %v", err)
				return
			}

			if lastEntry != string(feed) {
				//Insert into playlist
				entry := swarmFeedEntry{}
				json.Unmarshal(feed, &entry)
				vProfile := ffmpeg.VideoProfileLookup[entry.PlData]
				pl.InsertHLSSegment(&vProfile, entry.SeqNo, entry.SegLoc, SegLen.Seconds())
			} else {
				//Don't do anything
			}
			lastEntry = string(feed)
		}
	}

	os := drivers.NodeStorage.NewSession("./")
	lookupKey := fmt.Sprintf("%v,%v,%v", user, topic, swarmManifestID)
	if pl, ok := SwarmPlaylists[lookupKey]; !ok {
		pl = core.NewBasicPlaylistManager(core.ManifestID(manifestID), os)
		SwarmPlaylists[lookupKey] = pl
		go fetchFromSwarm(pl, query)
		time.Sleep(2 * time.Second)
		return pl.GetHLSMasterPlaylist(), nil
	} else {
		return pl.GetHLSMasterPlaylist(), nil
	}
}

func loadPlFromSwarm(bzzApi, user, topic, swarmManifestID, rendition string) (*m3u8.MediaPlaylist, error) {
	lookupKey := fmt.Sprintf("%v,%v,%v", user, topic, swarmManifestID)
	if pl, ok := SwarmPlaylists[lookupKey]; !ok {
		return nil, fmt.Errorf("Cannot find playlist from swarm for %v, %v, %v", user, topic, swarmManifestID)
	} else {
		return pl.GetHLSMediaPlaylist(rendition), nil
	}
}

func parseStreamID(reqPath string) core.StreamID {
	// remove extension and create streamid
	p := strings.TrimSuffix(reqPath, path.Ext(reqPath))
	return core.SplitStreamIDString(cleanStreamPrefix(p))
}

func parseManifestID(reqPath string) core.ManifestID {
	return parseStreamID(reqPath).ManifestID
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
	return false
}
