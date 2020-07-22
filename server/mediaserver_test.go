package server

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"math/big"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/pm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/drivers"
	"github.com/livepeer/go-livepeer/net"
	ffmpeg "github.com/livepeer/lpms/ffmpeg"
	"github.com/livepeer/lpms/segmenter"
	"github.com/livepeer/lpms/stream"
)

var S *LivepeerServer

var pushResetWg sync.WaitGroup // needed to synchronize exits from HTTP push

func setupServer() *LivepeerServer {
	s, _ := setupServerWithCancel()
	return s
}

func setupServerWithCancel() (*LivepeerServer, context.CancelFunc) {
	drivers.NodeStorage = drivers.NewMemoryDriver(nil)
	ctx, cancel := context.WithCancel(context.Background())
	if S == nil {
		httpPushResetTimer = func() (context.Context, context.CancelFunc) {
			ctx, cancel := context.WithCancel(context.Background())
			pushResetWg.Add(1)
			wrapCancel := func() {
				cancel()
				pushResetWg.Done()
			}
			return ctx, wrapCancel
		}
		n, _ := core.NewLivepeerNode(nil, "./tmp", nil)
		S, _ = NewLivepeerServer("127.0.0.1:1938", n, true, "")
		go S.StartMediaServer(ctx, "127.0.0.1:8080")
		go S.StartCliWebserver("127.0.0.1:8938")
	}
	return S, cancel
}

// since we have test that checks that there is no goroutine
// left running after using RTMP connection - we have to properly
// close connections in all the tests that are using them
func serverCleanup(s *LivepeerServer) {
	s.connectionLock.Lock()
	for _, cxn := range s.rtmpConnections {
		if cxn != nil && cxn.stream != nil {
			cxn.stream.Close()
		}
	}
	s.connectionLock.Unlock()
}

type stubDiscovery struct {
	infos       []*net.OrchestratorInfo
	waitGetOrch chan struct{}

	// typically the following fields have to be wrapped by `lock`
	// makes the race detector happy
	lock         *sync.Mutex
	getOrchCalls int
	getOrchError error
}

func (d *stubDiscovery) GetURLs() []*url.URL {
	return nil
}

func (d *stubDiscovery) GetOrchestrators(num int, sus common.Suspender, caps common.CapabilityComparator) ([]*net.OrchestratorInfo, error) {
	if d.waitGetOrch != nil {
		<-d.waitGetOrch
	}
	var err error
	if d.lock != nil {
		d.lock.Lock()
		d.getOrchCalls++
		err = d.getOrchError
		d.lock.Unlock()
	}
	return d.infos, err
}

func (d *stubDiscovery) Size() int {
	return len(d.infos)
}

type StubSegmenter struct {
	skip bool
}

func (s *StubSegmenter) SegmentRTMPToHLS(ctx context.Context, rs stream.RTMPVideoStream, hs stream.HLSVideoStream, segOptions segmenter.SegmenterOptions) error {
	if s.skip {
		// prevents spamming the console w error logging
		// when segment can't be submitted successfully
		return nil
	}
	glog.Infof("Calling StubSegmenter")
	if err := hs.AddHLSSegment(&stream.HLSSegment{SeqNo: 0, Name: "seg0.ts"}); err != nil {
		glog.Errorf("Error adding hls seg0")
	}
	if err := hs.AddHLSSegment(&stream.HLSSegment{SeqNo: 1, Name: "seg1.ts"}); err != nil {
		glog.Errorf("Error adding hls seg1")
	}
	if err := hs.AddHLSSegment(&stream.HLSSegment{SeqNo: 2, Name: "seg2.ts"}); err != nil {
		glog.Errorf("Error adding hls seg2")
	}
	if err := hs.AddHLSSegment(&stream.HLSSegment{SeqNo: 3, Name: "seg3.ts"}); err != nil {
		glog.Errorf("Error adding hls seg3")
	}
	return nil
}

func TestSelectOrchestrator(t *testing.T) {
	require := require.New(t)
	s := setupServer()
	defer serverCleanup(s)

	defer func() {
		s.LivepeerNode.Sender = nil
	}()

	// Empty discovery
	mid := core.RandomManifestID()
	storage := drivers.NodeStorage.NewSession(string(mid))
	sp := &core.StreamParameters{ManifestID: mid, Profiles: []ffmpeg.VideoProfile{ffmpeg.P360p30fps16x9}, OS: storage}
	if _, err := selectOrchestrator(s.LivepeerNode, sp, 4, newSuspender()); err != errDiscovery {
		t.Error("Expected error with discovery")
	}

	sd := &stubDiscovery{}
	// Discovery returned no orchestrators
	s.LivepeerNode.OrchestratorPool = sd
	if sess, err := selectOrchestrator(s.LivepeerNode, sp, 4, newSuspender()); sess != nil || err != errNoOrchs {
		t.Error("Expected nil session")
	}

	// populate stub discovery
	sd.infos = []*net.OrchestratorInfo{
		&net.OrchestratorInfo{PriceInfo: &net.PriceInfo{PricePerUnit: 1, PixelsPerUnit: 1}, TicketParams: &net.TicketParams{}},
		&net.OrchestratorInfo{PriceInfo: &net.PriceInfo{PricePerUnit: 1, PixelsPerUnit: 1}, TicketParams: &net.TicketParams{}},
	}
	sess, _ := selectOrchestrator(s.LivepeerNode, sp, 4, newSuspender())

	if len(sess) != len(sd.infos) {
		t.Error("Expected session length of 2")
	}

	if sess == nil {
		t.Error("Expected non-nil session")
	}

	// Sanity check a few easy fields
	if sess[0].Params.ManifestID != mid {
		t.Error("Expected manifest id")
	}
	if sess[0].BroadcasterOS != storage {
		t.Error("Unexpected broadcaster OS")
	}
	if sess[0].OrchestratorInfo != sd.infos[0] || sd.infos[0] == sd.infos[1] {
		t.Error("Unexpected orchestrator info")
	}
	if len(sess[0].Params.Profiles) != 1 || sess[0].Params.Profiles[0] != sp.Profiles[0] {
		t.Error("Unexpected profiles")
	}
	if sess[0].Sender != nil {
		t.Error("Unexpected sender")
	}
	if sess[0].PMSessionID != "" {
		t.Error("Unexpected PM sessionID")
	}

	// Test start PM session
	sender := &pm.MockSender{}
	s.LivepeerNode.Sender = sender

	price := &net.PriceInfo{
		PixelsPerUnit: 1,
		PricePerUnit:  1,
	}
	ratPrice, err := common.RatPriceInfo(price)
	require.Nil(err)
	params := pm.TicketParams{
		Recipient:         pm.RandAddress(),
		FaceValue:         big.NewInt(1234),
		WinProb:           big.NewInt(5678),
		RecipientRandHash: pm.RandHash(),
		Seed:              big.NewInt(7777),
		ExpirationBlock:   big.NewInt(100),
		PricePerPixel:     ratPrice,
	}
	protoParams := &net.TicketParams{
		Recipient:         params.Recipient.Bytes(),
		FaceValue:         params.FaceValue.Bytes(),
		WinProb:           params.WinProb.Bytes(),
		RecipientRandHash: params.RecipientRandHash.Bytes(),
		Seed:              params.Seed.Bytes(),
	}

	price2 := &net.PriceInfo{
		PixelsPerUnit: 2,
		PricePerUnit:  1,
	}
	ratPrice2, err := common.RatPriceInfo(price2)
	require.Nil(err)
	params2 := pm.TicketParams{
		Recipient:         pm.RandAddress(),
		FaceValue:         big.NewInt(1234),
		WinProb:           big.NewInt(5678),
		RecipientRandHash: pm.RandHash(),
		Seed:              big.NewInt(7777),
		ExpirationBlock:   big.NewInt(200),
		PricePerPixel:     ratPrice2,
	}
	protoParams2 := &net.TicketParams{
		Recipient:         params2.Recipient.Bytes(),
		FaceValue:         params2.FaceValue.Bytes(),
		WinProb:           params2.WinProb.Bytes(),
		RecipientRandHash: params2.RecipientRandHash.Bytes(),
		Seed:              params2.Seed.Bytes(),
		ExpirationBlock:   params.ExpirationBlock.Bytes(),
	}

	sd.infos = []*net.OrchestratorInfo{
		&net.OrchestratorInfo{
			TicketParams: protoParams,
			PriceInfo: &net.PriceInfo{
				PricePerUnit:  params.PricePerPixel.Num().Int64(),
				PixelsPerUnit: params.PricePerPixel.Denom().Int64(),
			},
		},
		&net.OrchestratorInfo{
			TicketParams: protoParams2,
			PriceInfo: &net.PriceInfo{
				PricePerUnit:  params2.PricePerPixel.Num().Int64(),
				PixelsPerUnit: params2.PricePerPixel.Denom().Int64(),
			},
		},
	}

	expSessionID := "foo"
	sender.On("StartSession", mock.Anything).Return(expSessionID).Once()

	expSessionID2 := "bar"
	sender.On("StartSession", mock.Anything).Return(expSessionID2).Once()

	sess, err = selectOrchestrator(s.LivepeerNode, sp, 4, newSuspender())
	require.Nil(err)

	assert := assert.New(t)
	assert.Len(sess, 2)
	assert.Equal(sender, sess[0].Sender)
	assert.Equal(expSessionID, sess[0].PMSessionID)
	assert.Equal(expSessionID2, sess[1].PMSessionID)
	assert.Equal(sess[0].OrchestratorInfo, &net.OrchestratorInfo{
		TicketParams: protoParams,
		PriceInfo: &net.PriceInfo{
			PricePerUnit:  params.PricePerPixel.Num().Int64(),
			PixelsPerUnit: params.PricePerPixel.Denom().Int64(),
		},
	})
	assert.Equal(sess[1].OrchestratorInfo, &net.OrchestratorInfo{
		TicketParams: protoParams2,
		PriceInfo: &net.PriceInfo{
			PricePerUnit:  params2.PricePerPixel.Num().Int64(),
			PixelsPerUnit: params2.PricePerPixel.Denom().Int64(),
		},
	})
}

func newStreamParams(mid core.ManifestID, rtmpKey string) *core.StreamParameters {
	return &core.StreamParameters{ManifestID: mid, RtmpKey: rtmpKey}
}

func TestCreateRTMPStreamHandlerCap(t *testing.T) {
	s := &LivepeerServer{
		connectionLock:  &sync.RWMutex{},
		rtmpConnections: make(map[core.ManifestID]*rtmpConnection),
	}
	createSid := createRTMPStreamIDHandler(s)
	u, _ := url.Parse("http://hot/id1/secret")
	oldMaxSessions := core.MaxSessions
	core.MaxSessions = 1
	// happy case
	sid := createSid(u).(*core.StreamParameters)
	mid := sid.ManifestID
	if mid != "id1" {
		t.Error("Stream should be allowd", sid)
	}
	if sid.StreamID() != "id1/secret" {
		t.Error("Stream id/key did not match ", sid)
	}
	s.rtmpConnections[core.ManifestID("id1")] = nil
	// capped case
	params := createSid(u)
	if params != nil {
		t.Error("Stream should be denied because of capacity cap")
	}
	core.MaxSessions = oldMaxSessions
}

type authWebhookReq struct {
	URL string `json:"url"`
}

func TestCreateRTMPStreamHandlerWebhook(t *testing.T) {
	assert := assert.New(t)
	s := setupServer()
	defer serverCleanup(s)
	s.RTMPSegmenter = &StubSegmenter{skip: true}
	createSid := createRTMPStreamIDHandler(s)

	AuthWebhookURL = "http://localhost:8938/notexisting"
	u, _ := url.Parse("http://hot/something/id1")
	sid := createSid(u)
	assert.Nil(sid, "Webhook auth failed")

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		out, _ := ioutil.ReadAll(r.Body)
		var req authWebhookReq
		err := json.Unmarshal(out, &req)
		if err != nil {
			fmt.Printf("Error parsing URL: %v\n", err)
			w.WriteHeader(http.StatusForbidden)
			return
		}

		w.Write(nil)
	}))
	defer ts.Close()
	AuthWebhookURL = ts.URL
	sid = createSid(u)
	assert.NotNil(sid, "On empty response with 200 code should pass")

	// local helper to reduce boilerplate
	makeServer := func(resp string) *httptest.Server {
		t := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(resp))
		}))
		AuthWebhookURL = t.URL
		return t
	}
	defer func() { AuthWebhookURL = "" }()
	BroadcastJobVideoProfiles = []ffmpeg.VideoProfile{ffmpeg.P360p30fps16x9}

	// empty manifestID
	ts2 := makeServer(`{"manifestID":""}`)
	defer ts2.Close()
	sid = createSid(u)
	assert.Nil(sid, "Should not pass if returned manifest id is empty")

	// invalid json
	ts3 := makeServer(`{manifestID:"XX"}`)
	defer ts3.Close()
	sid = createSid(u)
	assert.Nil(sid, "Should not pass if returned json is invalid")

	// set manifestID
	ts4 := makeServer(`{"manifestID":"xy"}`)
	defer ts4.Close()
	params := createSid(u).(*core.StreamParameters)
	mid := params.ManifestID
	assert.Equal(core.ManifestID("xy"), mid, "Should set manifest id to one provided by webhook")

	// ensure the presets match defaults
	assert.Len(params.Profiles, 1)
	assert.Equal(params.Profiles, BroadcastJobVideoProfiles, "Default presets did not match")

	// set manifestID + streamKey
	ts5 := makeServer(`{"manifestID":"xyz", "streamKey":"zyx"}`)
	defer ts5.Close()
	params = createSid(u).(*core.StreamParameters)
	mid = params.ManifestID
	assert.Equal(core.ManifestID("xyz"), mid, "Should set manifest to one provided by webhook")
	assert.Equal("xyz/zyx", params.StreamID(), "Should set streamkey to one provided by webhook")
	assert.Equal("zyx", params.RtmpKey, "Should set rtmp key to one provided by webhook")

	// set presets (with some invalid)
	ts6 := makeServer(`{"manifestID":"a", "presets":["P240p30fps16x9", "unknown", "P720p30fps16x9"]}`)
	defer ts6.Close()
	params = createSid(u).(*core.StreamParameters)
	assert.Len(params.Profiles, 2)
	assert.Equal(params.Profiles, []ffmpeg.VideoProfile{ffmpeg.P240p30fps16x9,
		ffmpeg.P720p30fps16x9}, "Did not have matching presets")

	// set profiles with valid values, presets empty
	ts7 := makeServer(`{"manifestID":"a", "profiles": [
		{"name": "prof1", "bitrate": 432, "fps": 560, "width": 123, "height": 456, "profile": "H264Baseline"},
		{"name": "prof2", "bitrate": 765, "fps": 876, "fpsDen": 12, "width": 456, "height": 987, "gop": "intra"},
		{"name": "passthru_fps", "bitrate": 890, "width": 789, "height": 654, "profile": "H264ConstrainedHigh", "gop":"123"}]}`)
	defer ts7.Close()
	params = createSid(u).(*core.StreamParameters)
	assert.Len(params.Profiles, 3)

	expectedProfiles := []ffmpeg.VideoProfile{
		ffmpeg.VideoProfile{
			Name:         "prof1",
			Bitrate:      "432",
			Framerate:    uint(560),
			FramerateDen: 0,
			Resolution:   "123x456",
			Profile:      ffmpeg.ProfileH264Baseline,
			GOP:          time.Duration(0),
		},
		ffmpeg.VideoProfile{
			Name:         "prof2",
			Bitrate:      "765",
			Framerate:    uint(876),
			FramerateDen: uint(12),
			Resolution:   "456x987",
			Profile:      ffmpeg.ProfileNone,
			GOP:          ffmpeg.GOPIntraOnly,
		},
		ffmpeg.VideoProfile{
			Name:         "passthru_fps",
			Bitrate:      "890",
			Resolution:   "789x654",
			Framerate:    0,
			FramerateDen: 0,
			Profile:      ffmpeg.ProfileH264ConstrainedHigh,
			GOP:          time.Duration(123) * time.Second,
		},
	}

	assert.Len(params.Profiles, 3)
	assert.Equal(expectedProfiles, params.Profiles, "Did not have matching profiles")

	// set profiles with invalid values, presets empty
	ts8 := makeServer(`{"manifestID":"a", "profiles": [
		{"name": "prof1", "bitrate": 432, "fps": 560, "width": 123, "height": 456},
		{"name": "prof2", "bitrate": 765, "fps": 876, "width": 456, "height": "hello"}]}`)
	defer ts8.Close()
	params, ok := createSid(u).(*core.StreamParameters)
	assert.False(ok)
	assert.Nil(params)

	// set profiles and presets
	ts9 := makeServer(`{"manifestID":"a", "presets":["P240p30fps16x9", "P720p30fps16x9"], "profiles": [
		{"name": "prof1", "bitrate": 432, "fps": 560, "width": 123, "height": 456, "profile": "H264Baseline"},
		{"name": "prof2", "bitrate": 765, "fps": 876, "fpsDen": 12, "width": 456, "height": 987, "gop":"intra"},
		{"name": "passthru_fps", "bitrate": 890, "width": 789, "height": 654, "profile": "H264ConstrainedHigh", "gop":"123"}]}`)

	defer ts9.Close()
	params = createSid(u).(*core.StreamParameters)
	jointProfiles := append([]ffmpeg.VideoProfile{ffmpeg.P240p30fps16x9, ffmpeg.P720p30fps16x9}, expectedProfiles...)

	assert.Len(params.Profiles, 5)
	assert.Equal(jointProfiles, params.Profiles, "Did not have matching profiles")

	// all invalid presets in webhook should lead to empty set
	ts10 := makeServer(`{"manifestID":"a", "presets":["very", "unknown"]}`)
	defer ts10.Close()
	params = createSid(u).(*core.StreamParameters)
	assert.Len(params.Profiles, 0, "Unexpected value in presets")

	// invalid gops
	ts11 := makeServer(`{"manifestID":"a", "profiles": [ {"gop": " 1 "}]}`)
	defer ts11.Close()
	assert.Nil(createSid(u))
	ts12 := makeServer(`{"manifestID":"a", "profiles": [ {"gop": "0"}]}`)
	defer ts12.Close()
	assert.Nil(createSid(u))
	ts13 := makeServer(`{"manifestID":"a", "profiles": [ {"gop": "-0,01"}]}`)
	defer ts13.Close()
	assert.Nil(createSid(u))

	// intra only gop
	ts14 := makeServer(`{"manifestID":"a", "profiles": [ {"gop": "intra" }]}`)
	defer ts14.Close()
	params = createSid(u).(*core.StreamParameters)
	assert.Len(params.Profiles, 1)
	assert.Equal(ffmpeg.GOPIntraOnly, params.Profiles[0].GOP)
}

func TestCreateRTMPStreamHandler(t *testing.T) {

	// Monkey patch rng to avoid unpredictability even when seeding
	oldRandFunc := common.RandomIDGenerator
	common.RandomIDGenerator = func(length uint) string {
		return "abcdef"
	}
	defer func() { common.RandomIDGenerator = oldRandFunc }()

	s := setupServer()
	defer serverCleanup(s)
	s.RTMPSegmenter = &StubSegmenter{skip: true}
	handler := gotRTMPStreamHandler(s)
	createSid := createRTMPStreamIDHandler(s)
	endHandler := endRTMPStreamHandler(s)

	// Test default path structure
	expectedSid := core.MakeStreamIDFromString("ghijkl", "secretkey")
	u, _ := url.Parse("rtmp://localhost/" + expectedSid.String()) // with key
	if sid := createSid(u); sid.StreamID() != expectedSid.String() {
		t.Error("Unexpected streamid", sid.StreamID())
	}
	u, _ = url.Parse("rtmp://localhost/stream/" + expectedSid.String()) // with stream
	if sid := createSid(u); sid.StreamID() != expectedSid.String() {
		t.Error("Unexpected streamid")
	}
	expectedMid := "mnopq"
	key := common.RandomIDGenerator(StreamKeyBytes)
	u, _ = url.Parse("rtmp://localhost/" + string(expectedMid)) // without key
	if sid := createSid(u); sid.StreamID() != string(expectedMid)+"/"+key {
		t.Error("Unexpected streamid", sid.StreamID())
	}
	u, _ = url.Parse("rtmp://localhost/stream/" + string(expectedMid)) // with stream, without key
	if sid := createSid(u); sid.StreamID() != string(expectedMid)+"/"+key {
		t.Error("Unexpected streamid", sid.StreamID())
	}
	// Test normal case
	u, _ = url.Parse("rtmp://localhost")
	st := stream.NewBasicRTMPVideoStream(createSid(u))
	if st.GetStreamID() == "" {
		t.Error("Empty streamid")
	}
	// Populate internal state with s1
	if err := handler(u, st); err != nil {
		t.Error("Handler failed ", err)
	}
	// Test collisions via stream reuse
	if sid := createSid(u); sid != nil {
		t.Error("Expected failure due to naming collision")
	}
	// Ensure the stream ID is reusable after the stream ends
	if err := endHandler(u, st); err != nil {
		t.Error("Could not clean up stream")
	}
	if sid := createSid(u); sid.StreamID() != st.GetStreamID() {
		t.Error("Mismatched streamid during stream reuse", sid.StreamID(), st.GetStreamID())
	}

	// Test a couple of odd cases; subset of parseManifestID checks
	// (Would be nice to stub out parseManifestID to receive stronger
	//  transitive assurance via existing parseManifestID tests)
	testManifestIDQueryParam := func(inp string) {
		// This isn't a great test because if the query param ever changes,
		// this test will still pass
		u, _ := url.Parse("rtmp://localhost/" + inp)
		if sid := createSid(u); sid.StreamID() != st.GetStreamID() {
			t.Errorf("Unexpected StreamID for '%v' ; expected '%v' for input '%v'", sid, st.GetStreamID(), inp)
		}
	}
	inputs := []string{"  /  ", ".m3u8", "/stream/", "stream/.m3u8"}
	for _, v := range inputs {
		testManifestIDQueryParam(v)
	}
}

func TestEndRTMPStreamHandler(t *testing.T) {
	s := setupServer()
	defer serverCleanup(s)
	s.RTMPSegmenter = &StubSegmenter{skip: true}
	createSid := createRTMPStreamIDHandler(s)
	handler := gotRTMPStreamHandler(s)
	endHandler := endRTMPStreamHandler(s)
	u, _ := url.Parse("rtmp://localhost")
	sid := createSid(u)
	st := stream.NewBasicRTMPVideoStream(sid)

	// Nonexistent stream
	if err := endHandler(u, st); err != errUnknownStream {
		t.Error("Expected unknown stream ", err)
	}
	// Normal case: clean up existing stream
	if err := handler(u, st); err != nil {
		t.Error("Handler failed ", err)
	}
	if err := endHandler(u, st); err != nil {
		t.Error("Did  not end stream ", err)
	}
	// Check behavior on calling `endHandler` twice
	if err := endHandler(u, st); err != errUnknownStream {
		t.Error("Stream was not cleaned up properly ", err)
	}
}

// Should publish RTMP stream, turn the RTMP stream into HLS, and broadcast the HLS stream.
func TestGotRTMPStreamHandler(t *testing.T) {
	s := setupServer()
	defer serverCleanup(s)
	s.RTMPSegmenter = &StubSegmenter{}
	handler := gotRTMPStreamHandler(s)

	vProfile := ffmpeg.P720p30fps16x9
	hlsStrmID := core.MakeStreamID(core.ManifestID("ghijkl"), &vProfile)
	u, _ := url.Parse("rtmp://localhost:1935/movie")
	strm := stream.NewBasicRTMPVideoStream(&core.StreamParameters{ManifestID: hlsStrmID.ManifestID})
	expectedSid := core.MakeStreamIDFromString(string(hlsStrmID.ManifestID), "source")

	// Check for invalid node storage
	oldStorage := drivers.NodeStorage
	drivers.NodeStorage = nil
	if err := handler(u, strm); err != errStorage {
		t.Error("Expected storage error ", err)
	}
	drivers.NodeStorage = oldStorage

	//Try to handle test RTMP data.
	if err := handler(u, strm); err != nil {
		t.Errorf("Error: %v", err)
	}

	// Check assigned IDs
	mid := streamParams(strm).ManifestID
	if s.LatestPlaylist().ManifestID() != mid {
		t.Error("Unexpected Manifest ID")
	}
	if s.LastHLSStreamID() != expectedSid {
		t.Error("Unexpected Stream ID ", s.LastHLSStreamID(), expectedSid)
	}

	//Stream already exists
	if err := handler(u, strm); err != errAlreadyExists {
		t.Errorf("Expecting publish error because stream already exists, but got: %v", err)
	}

	start := time.Now()
	for time.Since(start) < time.Second*2 {
		pl := s.LatestPlaylist().GetHLSMediaPlaylist(expectedSid.Rendition)
		if pl == nil || len(pl.Segments) != 4 {
			time.Sleep(100 * time.Millisecond)
			continue
		} else {
			break
		}
	}
	pl := s.LatestPlaylist().GetHLSMediaPlaylist(expectedSid.Rendition)
	if pl == nil {
		t.Error("Expected media playlist; got none ", expectedSid)
	}

	if pl.Count() != 4 {
		t.Errorf("Should have received 4 data chunks, got: %v", pl.Count())
	}

	for i := 0; i < 4; i++ {
		// XXX we shouldn't do this. Need threadsafe accessors for playlist
		seg := pl.Segments[i]
		shouldSegName := fmt.Sprintf("/stream/%s/%s/%d.ts", mid, expectedSid.Rendition, i)
		if seg.URI != shouldSegName {
			t.Fatalf("Wrong segment, should have URI %s, has %s", shouldSegName, seg.URI)
		}
	}
}

func TestMultiStream(t *testing.T) {
	// set unlimited sessions because this tests creates 500 streams
	core.MaxSessions = 0
	//Turning off logging to stderr because this test prints ALOT of logs.
	//Ideally we would record the flag value and set it back instead of hardcoding the value,
	// but the `flag` doesn't allow easy access to existing flag value.
	flag.Set("logtostderr", "false")
	defer func() {
		flag.Set("logtostderr", "true")
	}()
	s := setupServer()
	defer serverCleanup(s)
	s.RTMPSegmenter = &StubSegmenter{skip: true}
	handler := gotRTMPStreamHandler(s)
	createSid := createRTMPStreamIDHandler(s)
	u, _ := url.Parse("rtmp://localhost")

	handleStream := func(i int) {
		st := stream.NewBasicRTMPVideoStream(createSid(u))
		if err := handler(u, st); err != nil {
			t.Error("Could not handle stream ", i, err)
		}
	}

	// test synchronous
	const syncStreams = 10
	for i := 0; i < syncStreams; i++ {
		handleStream(i)
	}

	// test more streams, somewhat concurrently
	var wg sync.WaitGroup
	const asyncStreams = 500
	for i := syncStreams; i < asyncStreams; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			handleStream(i)
		}(i)
	}

	// block until complete
	ch := make(chan struct{})
	go func() {
		wg.Wait()
		ch <- struct{}{}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	select {
	case <-ch:
		close(ch)
		if len(s.rtmpConnections) < asyncStreams {
			t.Error("Did not have expected number of streams", len(s.rtmpConnections))
		}
	case <-ctx.Done():
		t.Error("Timed out")
		close(ch)
	}

	// probably should test (concurrent) cleanups as well
}

func TestGetHLSMasterPlaylistHandler(t *testing.T) {
	glog.Infof("\n\nTestGetHLSMasterPlaylistHandler...\n")

	s := setupServer()
	defer serverCleanup(s)
	handler := gotRTMPStreamHandler(s)

	vProfile := ffmpeg.P720p30fps16x9
	hlsStrmID := core.MakeStreamID(core.RandomManifestID(), &vProfile)
	url, _ := url.Parse("rtmp://localhost:1935/movie")
	strm := stream.NewBasicRTMPVideoStream(newStreamParams(hlsStrmID.ManifestID, "source"))

	if err := handler(url, strm); err != nil {
		t.Errorf("Error: %v", err)
	}

	segName := "test_seg/1.ts"
	err := s.LatestPlaylist().InsertHLSSegment(&vProfile, 1, segName, 12)
	if err != nil {
		t.Fatal(err)
	}
	mid := hlsStrmID.ManifestID

	mlHandler := getHLSMasterPlaylistHandler(s)
	url2, _ := url.Parse(fmt.Sprintf("http://localhost/stream/%s.m3u8", mid))

	//Test get master playlist
	pl, err := mlHandler(url2)
	if err != nil {
		t.Errorf("Error handling getHLSMasterPlaylist: %v", err)
	}
	if pl == nil {
		t.Fatal("Expected playlist; got none")
	}

	if len(pl.Variants) != 1 {
		t.Errorf("Expecting 1 variant, but got %v", pl)
	}
	mediaPLName := fmt.Sprintf("%s.m3u8", hlsStrmID)
	if pl.Variants[0].URI != mediaPLName {
		t.Errorf("Expecting %s, but got: %s", mediaPLName, pl.Variants[0].URI)
	}
}

func TestRegisterConnection(t *testing.T) {
	assert := assert.New(t)
	s := setupServer()
	defer serverCleanup(s)
	mid := core.SplitStreamIDString(t.Name()).ManifestID
	strm := stream.NewBasicRTMPVideoStream(&core.StreamParameters{ManifestID: mid})

	// Should return an error if missing node storage
	drivers.NodeStorage = nil
	_, err := s.registerConnection(strm)
	assert.Equal(err, errStorage)
	drivers.NodeStorage = drivers.NewMemoryDriver(nil)

	// normal success case
	rand.Seed(123)
	cxn, err := s.registerConnection(strm)
	assert.NotNil(cxn)
	assert.Nil(err)

	// Check some properties of the cxn / server
	assert.Equal(mid, cxn.mid)
	assert.Equal("source", cxn.profile.Name)
	assert.Equal(uint64(0x4a68998bed5c40f1), cxn.nonce)

	assert.Equal(mid, s.LastManifestID())
	assert.Equal(string(mid)+"/source", s.LastHLSStreamID().String())
	assert.NotNil(cxn.params.OS)
	assert.Nil(cxn.params.OS.GetInfo())
	assert.NotNil(cxn.params.Capabilities)

	// Should return an error if creating another cxn with the same mid
	_, err = s.registerConnection(strm)
	assert.Equal(err, errAlreadyExists)

	// Check for params with an existing OS assigned
	storage := drivers.NewS3Driver("", "", "", "", false).NewSession("")
	strm = stream.NewBasicRTMPVideoStream(&core.StreamParameters{ManifestID: core.RandomManifestID(), OS: storage})
	cxn, err = s.registerConnection(strm)
	assert.Nil(err)
	assert.Equal(storage, cxn.params.OS)
	assert.Equal(net.OSInfo_S3, cxn.params.OS.GetInfo().StorageType)
	assert.Equal(cxn.params.OS, cxn.pl.GetOSSession())

	// check for capabilities
	profiles := []ffmpeg.VideoProfile{ffmpeg.P144p30fps16x9}
	strm = stream.NewBasicRTMPVideoStream(&core.StreamParameters{ManifestID: core.RandomManifestID(), Profiles: profiles})
	cxn, err = s.registerConnection(strm)
	assert.Nil(err)
	job, err := core.JobCapabilities(streamParams(strm))
	assert.Nil(err)
	assert.Equal(job, cxn.params.Capabilities)
	// check for capabilities: exit with an invalid cap
	profiles[0].Format = -1
	strm = stream.NewBasicRTMPVideoStream(&core.StreamParameters{ManifestID: core.RandomManifestID(), Profiles: profiles})
	cxn, err = s.registerConnection(strm)
	assert.Equal("capability: unknown format", err.Error())
	// TODO test with non-legacy capabilities once we have some
	// Should result in a non-nil `cxn.params.Capabilities`.

	// Ensure thread-safety under -race
	var wg sync.WaitGroup
	for i := 0; i < 500; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			name := fmt.Sprintf("%v_%v", t.Name(), i)
			mid := core.SplitStreamIDString(name).ManifestID
			strm := stream.NewBasicRTMPVideoStream(&core.StreamParameters{ManifestID: mid})
			cxn, err := s.registerConnection(strm)

			assert.Nil(err)
			assert.NotNil(cxn)
			assert.Equal(mid, cxn.mid)
		}(i)
	}
	wg.Wait()

}

func TestBroadcastSessionManagerWithStreamStartStop(t *testing.T) {
	assert := assert.New(t)

	s := setupServer()
	defer serverCleanup(s)
	// populate stub discovery
	sd := &stubDiscovery{}
	sd.infos = []*net.OrchestratorInfo{
		&net.OrchestratorInfo{Transcoder: "transcoder1", TicketParams: &net.TicketParams{}, PriceInfo: &net.PriceInfo{PricePerUnit: 1, PixelsPerUnit: 1}},
		&net.OrchestratorInfo{Transcoder: "transcoder2", TicketParams: &net.TicketParams{}, PriceInfo: &net.PriceInfo{PricePerUnit: 1, PixelsPerUnit: 1}},
	}
	s.LivepeerNode.OrchestratorPool = sd

	// create RTMPStream handler methods
	s.RTMPSegmenter = &StubSegmenter{skip: true}
	createSid := createRTMPStreamIDHandler(s)
	handler := gotRTMPStreamHandler(s)
	endHandler := endRTMPStreamHandler(s)

	// create BasicRTMPVideoStream and extract ManifestID
	u, _ := url.Parse("rtmp://localhost")
	sid := createSid(u)
	st := stream.NewBasicRTMPVideoStream(sid)
	mid := streamParams(st).ManifestID

	// assert that ManifestID has not been added to rtmpConnections yet
	_, exists := s.rtmpConnections[mid]
	assert.Equal(exists, false)

	// assert stream starts successfully
	err := handler(u, st)
	assert.Nil(err)

	// assert sessManager is running and has right number of sessions
	cxn, exists := s.rtmpConnections[mid]
	assert.Equal(exists, true)
	assert.Equal(cxn.sessManager.finished, false)
	assert.Equal(cxn.sessManager.sel.Size(), 2)
	assert.Len(cxn.sessManager.sessMap, 2)

	// assert stream ends successfully
	err = endHandler(u, st)
	assert.Nil(err)

	// assert sessManager is not running and has no sessions
	_, exists = s.rtmpConnections[mid]
	assert.Equal(exists, false)
	assert.Equal(cxn.sessManager.finished, true)
	assert.Equal(cxn.sessManager.sel.Size(), 0)
	assert.Len(cxn.sessManager.sessMap, 0)

	// assert stream starts successfully again
	err = handler(u, st)
	assert.Nil(err)

	// assert sessManager is running and has right number of sessions
	cxn, exists = s.rtmpConnections[mid]
	assert.Equal(exists, true)
	assert.Equal(cxn.sessManager.finished, false)
	assert.Equal(cxn.sessManager.sel.Size(), 2)
	assert.Len(cxn.sessManager.sessMap, 2)
}

func TestCleanStreamPrefix(t *testing.T) {
	u, _ := url.Parse("http://localhost/stream/1220c50f8bc4d2a807aace1e1376496a9d7f7c1408dec2512763c3ca16fe828f6631_01.ts")
	segName := cleanStreamPrefix(u.Path)
	if segName != "1220c50f8bc4d2a807aace1e1376496a9d7f7c1408dec2512763c3ca16fe828f6631_01.ts" {
		t.Errorf("Expecting %v, but %v", "1220c50f8bc4d2a807aace1e1376496a9d7f7c1408dec2512763c3ca16fe828f6631_01.ts", segName)
	}

	str := cleanStreamPrefix("")
	if str != "" {
		t.Error("Expected empty stream prefix; got ", str)
	}
	str = cleanStreamPrefix("  /  //  ///  / abc def")
	if str != "abc def" {
		t.Error("Unexpected value after prefix cleaning; got ", str)
	}

	str = cleanStreamPrefix("  /  //  ///  / stream/abc def")
	if str != "abc def" {
		t.Error("Unexpected value after prefix cleaning; got ", str)
	}
}

func TestShouldStopStream(t *testing.T) {
	assert := assert.New(t)
	ok := shouldStopStream(fmt.Errorf("some random error string"))
	assert.False(ok)
	ok = shouldStopStream(pm.ErrSenderValidation{})
	assert.True(ok)
}

func TestParseManifestID(t *testing.T) {
	checkMid := func(inp string, exp string) {
		mid := parseManifestID(inp)
		if mid != core.ManifestID(exp) {
			t.Errorf("Unexpected ManifestID; expected '%v' got '%v' with input '%v'", exp, mid, inp)
		}
	}

	emptyExpects := []string{"", "/", "///", "  /  ", "/stream/", "stream/", "/stream///", " stream/", "stream/.m3u8", "stream//.m3u8"}
	for _, v := range emptyExpects {
		checkMid(v, "")
	}

	abcExpects := []string{"/stream/abc.m3u8", "/stream/abc/def.m3u8", "stream/abc/def.m3u8", "/abc/def/m3u8", "abc/def.m3u8", "abc/def", "abc/", "abc//", "abc", "  abc", "  /abc", "/abc.m3u8"}
	for _, v := range abcExpects {
		checkMid(v, "abc")
	}

	checkMid("/stream/stream/stream", "stream")
}

func TestParseStreamID(t *testing.T) {
	checkSid := func(inp string, exp core.StreamID) {
		sid := parseStreamID(inp)
		if sid.ManifestID != exp.ManifestID || sid.Rendition != exp.Rendition {
			t.Errorf("Unexpected StreamID; expected '%v' got '%v' with input '%v'", exp, sid, inp)
		}
	}

	checkSid("stream/", core.StreamID{})
	checkSid("stream/.m3u8", core.StreamID{})
	checkSid("stream/abc  .m3u8", core.StreamID{ManifestID: "abc  "})
	checkSid("/stream/abc/def  .m3u8", core.StreamID{ManifestID: "abc", Rendition: "def  "})
	checkSid("/stream/stream/stream/stream", core.StreamID{ManifestID: "stream", Rendition: "stream/stream"})
	checkSid("/abc", core.StreamID{ManifestID: "abc"})
	checkSid("/abc.m3u8", core.StreamID{ManifestID: "abc"})
	checkSid("//abc", core.StreamID{ManifestID: "abc"})
	checkSid("abc/def//ghi", core.StreamID{ManifestID: "abc", Rendition: "def//ghi"})
	checkSid("abc/def.m3u8/ghi.ts", core.StreamID{ManifestID: "abc", Rendition: "def.m3u8/ghi"})
}

func TestParsePresets(t *testing.T) {
	assert := assert.New(t)
	presets := []string{"P240p30fps16x9", "unknown", "P720p30fps16x9"}

	p := parsePresets([]string{})
	assert.Equal([]ffmpeg.VideoProfile{}, p)

	p = parsePresets(nil)
	assert.Equal([]ffmpeg.VideoProfile{}, p)

	p = parsePresets([]string{"bad", "example"})
	assert.Equal([]ffmpeg.VideoProfile{}, p)

	p = parsePresets(presets)
	assert.Equal([]ffmpeg.VideoProfile{ffmpeg.P240p30fps16x9, ffmpeg.P720p30fps16x9}, p)

}

func TestJsonProfileToVideoProfiles(t *testing.T) {
	assert := assert.New(t)
	initialValue := []byte(`[{"Width":1,"Height":2}]`)
	resp := &authWebhookResponse{}

	// test empty case
	p, err := jsonProfileToVideoProfile(resp)
	assert.Nil(err)
	assert.Len(p, 0)

	err = json.Unmarshal(initialValue, &resp.Profiles)

	// test default name
	p, err = jsonProfileToVideoProfile(resp)
	assert.Nil(err)
	assert.Equal("webhook_1x2_0", p[0].Name)

	// test provided name
	resp.Profiles[0].Name = "abc"
	p, err = jsonProfileToVideoProfile(resp)
	assert.Nil(err)
	assert.Equal("abc", p[0].Name)

	// test gop empty
	assert.Equal("", resp.Profiles[0].GOP)
	assert.Equal(time.Duration(0), p[0].GOP)

	// test gop intra
	resp.Profiles[0].GOP = "intra"
	p, err = jsonProfileToVideoProfile(resp)
	assert.Nil(err)
	assert.Equal(ffmpeg.GOPIntraOnly, p[0].GOP)

	// test gop float
	resp.Profiles[0].GOP = "1.2"
	p, err = jsonProfileToVideoProfile(resp)
	assert.Nil(err)
	assert.Equal(time.Duration(1200)*time.Millisecond, p[0].GOP)

	// test gop integer
	resp.Profiles[0].GOP = "60"
	p, err = jsonProfileToVideoProfile(resp)
	assert.Nil(err)
	assert.Equal(time.Minute, p[0].GOP)

	// test gop 0
	resp.Profiles[0].GOP = "0"
	p, err = jsonProfileToVideoProfile(resp)
	assert.Nil(p)
	assert.NotNil(err)
	assert.Equal("invalid gop value", err.Error())

	// test gop <0
	resp.Profiles[0].GOP = "-0.001"
	p, err = jsonProfileToVideoProfile(resp)
	assert.Nil(p)
	assert.NotNil(err)
	assert.Equal("invalid gop value", err.Error())

	// test gop non-numeric
	resp.Profiles[0].GOP = " 1 "
	p, err = jsonProfileToVideoProfile(resp)
	assert.Nil(p)
	assert.NotNil(err)
	assert.Contains(err.Error(), "strconv.ParseFloat: parsing")
	resp.Profiles[0].GOP = ""

	// test default encoding profile
	p, err = jsonProfileToVideoProfile(resp)
	assert.Nil(err)
	assert.Equal(ffmpeg.ProfileNone, p[0].Profile)

	// test encoding profile
	resp.Profiles[0].Profile = "h264baseline"
	p, err = jsonProfileToVideoProfile(resp)
	assert.Nil(err)
	assert.Equal(ffmpeg.ProfileH264Baseline, p[0].Profile)

	// test invalid encoding profile
	resp.Profiles[0].Profile = "invalid"
	p, err = jsonProfileToVideoProfile(resp)
	assert.Nil(p)
	assert.Equal(common.ErrProfName, err)
}
