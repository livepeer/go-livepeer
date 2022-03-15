package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/big"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/pm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	gonet "net"

	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/drivers"
	"github.com/livepeer/go-livepeer/net"
	lpmscore "github.com/livepeer/lpms/core"
	ffmpeg "github.com/livepeer/lpms/ffmpeg"
	"github.com/livepeer/lpms/stream"
)

var pushResetWg sync.WaitGroup // needed to synchronize exits from HTTP push
var port = 10000

// waitForTCP tries to establish TCP connection for a specified time
func waitForTCP(waitForTarget time.Duration, uri string) error {
	var u *url.URL
	var err error
	if u, err = url.Parse(uri); err != nil {
		return err
	}
	if u.Port() == "" {
		switch u.Scheme {
		case "rtmp":
			u.Host = u.Host + ":1935"
		}
	}
	dailer := gonet.Dialer{Timeout: 2 * time.Second}
	started := time.Now()
	var conn gonet.Conn
	for {
		if conn, err = dailer.Dial("tcp", u.Host); err != nil {
			time.Sleep(10 * time.Millisecond)
			if time.Since(started) > waitForTarget {
				return fmt.Errorf("Can't connect to '%s' for more than %s", uri, waitForTarget)
			}
			continue
		}
		conn.Close()
		break
	}
	return nil
}

func setupServerWithCancel() (*LivepeerServer, context.CancelFunc) {
	// wait for any earlier tests to complete
	wgWait(&pushResetWg)
	drivers.NodeStorage = drivers.NewMemoryDriver(nil)
	ctx, cancel := context.WithCancel(context.Background())
	var S *LivepeerServer
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
		// doesn't really starts server at 1938
		S, _ = NewLivepeerServer("127.0.0.1:1938", n, true, "")
		// rtmpurl := fmt.Sprintf("rtmp://127.0.0.1:%d", port)
		// S, _ = NewLivepeerServer(rtmpurl, n, true, "")
		// glog.Errorf("++> rtmp server with port %d", port)
		// port++
		// mediaUrl := fmt.Sprintf("http://127.0.0.1:%d", port)
		// doesn't really starts server at 8938
		go S.StartMediaServer(ctx, "127.0.0.1:8938")
		// port++
		// this one really starts server (without a way to shut it down)
		cliUrl := fmt.Sprintf("127.0.0.1:%d", port)
		go S.StartCliWebserver(cliUrl)
		port++
		// sometimes LivepeerServer needs time  to start
		// esp if this is the only test in the suite being run (eg, via `-run)
		// time.Sleep(10 * time.Millisecond)
		// if err := waitForTCP(2*time.Second, rtmpurl); err != nil {
		// 	panic(err)
		// }
		if err := waitForTCP(2*time.Second, "http://"+cliUrl); err != nil {
			panic(err)
		}
	}
	return S, cancel
}

func setupServerWithCancelAndPorts() (*LivepeerServer, context.CancelFunc) {
	drivers.NodeStorage = drivers.NewMemoryDriver(nil)
	ctx, cancel := context.WithCancel(context.Background())
	var S *LivepeerServer
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
		S, _ = NewLivepeerServer("127.0.0.1:2938", n, true, "")
		go S.StartMediaServer(ctx, "127.0.0.1:9080")
		go S.StartCliWebserver("127.0.0.1:9938")
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
		if cxn != nil {
			cxn.pl.Cleanup()
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

func (d *stubDiscovery) GetInfos() []common.OrchestratorLocalInfo {
	return nil
}

func (d *stubDiscovery) GetInfo(uri string) common.OrchestratorLocalInfo {
	var res common.OrchestratorLocalInfo
	return res
}

func (d *stubDiscovery) GetOrchestrators(ctx context.Context, num int, sus common.Suspender, caps common.CapabilityComparator,
	scorePred common.ScorePred) ([]*net.OrchestratorInfo, error) {

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

func (d *stubDiscovery) SizeWith(scorePred common.ScorePred) int {
	return len(d.infos)
}

func TestSelectOrchestrator(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	s, cancel := setupServerWithCancel()
	defer serverCleanup(s)
	defer cancel()

	defer func() {
		s.LivepeerNode.Sender = nil
		s.LivepeerNode.OrchestratorPool = nil
	}()

	// Empty discovery
	mid := core.RandomManifestID()
	storage := drivers.NodeStorage.NewSession(string(mid))
	sp := &core.StreamParameters{ManifestID: mid, Profiles: []ffmpeg.VideoProfile{ffmpeg.P360p30fps16x9}, OS: storage}
	if _, err := selectOrchestrator(context.TODO(), s.LivepeerNode, sp, 4, newSuspender(), common.ScoreAtLeast(0)); err != errDiscovery {
		t.Error("Expected error with discovery")
	}

	sd := &stubDiscovery{}
	// Discovery returned no orchestrators
	s.LivepeerNode.OrchestratorPool = sd
	if sess, err := selectOrchestrator(context.TODO(), s.LivepeerNode, sp, 4, newSuspender(), common.ScoreAtLeast(0)); sess != nil || err != errNoOrchs {
		t.Error("Expected nil session")
	}

	// populate stub discovery
	authToken0 := stubAuthToken
	authToken1 := &net.AuthToken{Token: stubAuthToken.Token, SessionId: "somethinglese", Expiration: stubAuthToken.Expiration}
	sd.infos = []*net.OrchestratorInfo{
		{PriceInfo: &net.PriceInfo{PricePerUnit: 1, PixelsPerUnit: 1}, TicketParams: &net.TicketParams{}, AuthToken: authToken0},
		{PriceInfo: &net.PriceInfo{PricePerUnit: 1, PixelsPerUnit: 1}, TicketParams: &net.TicketParams{}, AuthToken: authToken1},
	}
	sess, _ := selectOrchestrator(context.TODO(), s.LivepeerNode, sp, 4, newSuspender(), common.ScoreAtLeast(0))

	if len(sess) != len(sd.infos) {
		t.Error("Expected session length of 2")
	}

	if sess == nil || len(sess) < 1 {
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

	// Check broadcaster OS session initialization when OS is external
	drivers.NodeStorage, _ = drivers.ParseOSURL("s3://key:secret@us/livepeer", false)
	externalStorage := drivers.NodeStorage.NewSession(string(mid))
	sp.OS = externalStorage

	sess, err := selectOrchestrator(context.TODO(), s.LivepeerNode, sp, 4, newSuspender(), common.ScoreAtLeast(0))
	assert.Nil(err)

	// B should initialize new OS session using auth token sessionID
	assert.NotEqual(externalStorage, sess[0].BroadcasterOS)
	assert.Equal(fmt.Sprintf("%v/%v", sp.ManifestID, authToken0.SessionId), sess[0].BroadcasterOS.GetInfo().S3Info.Key)
	assert.NotEqual(externalStorage, sess[1].BroadcasterOS)
	assert.Equal(fmt.Sprintf("%v/%v", sp.ManifestID, authToken1.SessionId), sess[1].BroadcasterOS.GetInfo().S3Info.Key)

	drivers.NodeStorage = drivers.NewMemoryDriver(nil)
	sp.OS = storage

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
		{
			AuthToken:    &net.AuthToken{},
			TicketParams: protoParams,
			PriceInfo: &net.PriceInfo{
				PricePerUnit:  params.PricePerPixel.Num().Int64(),
				PixelsPerUnit: params.PricePerPixel.Denom().Int64(),
			},
		},
		{
			AuthToken:    &net.AuthToken{},
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

	sess, err = selectOrchestrator(context.TODO(), s.LivepeerNode, sp, 4, newSuspender(), common.ScoreAtLeast(0))
	require.Nil(err)

	assert.Len(sess, 2)
	assert.Equal(sender, sess[0].Sender)
	assert.Equal(expSessionID, sess[0].PMSessionID)
	assert.Equal(expSessionID2, sess[1].PMSessionID)
	assert.Equal(sess[0].OrchestratorInfo, &net.OrchestratorInfo{
		AuthToken:    &net.AuthToken{},
		TicketParams: protoParams,
		PriceInfo: &net.PriceInfo{
			PricePerUnit:  params.PricePerPixel.Num().Int64(),
			PixelsPerUnit: params.PricePerPixel.Denom().Int64(),
		},
	})
	assert.Equal(sess[1].OrchestratorInfo, &net.OrchestratorInfo{
		AuthToken:    &net.AuthToken{},
		TicketParams: protoParams2,
		PriceInfo: &net.PriceInfo{
			PricePerUnit:  params2.PricePerPixel.Num().Int64(),
			PixelsPerUnit: params2.PricePerPixel.Denom().Int64(),
		},
	})

	sender.On("StartSession", mock.Anything).Return("anything")

	// Skip orchestrator if missing auth token
	sd.infos[0].AuthToken = nil

	sess, err = selectOrchestrator(context.TODO(), s.LivepeerNode, sp, 4, newSuspender(), func(float32) bool { return true })
	require.Nil(err)

	assert.Len(sess, 1)
	assert.Equal(protoParams2.Recipient, sess[0].OrchestratorInfo.TicketParams.Recipient)

	// Skip orchestrator if missing ticket params
	sd.infos[0].AuthToken = &net.AuthToken{}
	sd.infos[0].TicketParams = nil

	sess, err = selectOrchestrator(context.TODO(), s.LivepeerNode, sp, 4, newSuspender(), func(float32) bool { return true })
	require.Nil(err)

	assert.Len(sess, 1)
	assert.Equal(protoParams2.Recipient, sess[0].OrchestratorInfo.TicketParams.Recipient)
}

func newStreamParams(mid core.ManifestID, rtmpKey string) *core.StreamParameters {
	return &core.StreamParameters{ManifestID: mid, RtmpKey: rtmpKey}
}

func TestCreateRTMPStreamHandlerCap(t *testing.T) {
	s := &LivepeerServer{
		connectionLock:  &sync.RWMutex{},
		rtmpConnections: make(map[core.ManifestID]*rtmpConnection),
	}
	createSid := createRTMPStreamIDHandler(context.TODO(), s)
	u := mustParseUrl(t, "http://hot/id1/secret")
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
	s, cancel := setupServerWithCancel()
	defer serverCleanup(s)
	defer cancel()
	createSid := createRTMPStreamIDHandler(context.TODO(), s)

	AuthWebhookURL = mustParseUrl(t, "http://localhost:8938/notexisting")
	u := mustParseUrl(t, "http://hot/something/id1")
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
	AuthWebhookURL = mustParseUrl(t, ts.URL)
	sid = createSid(u)
	assert.NotNil(sid, "On empty response with 200 code should pass")

	// local helper to reduce boilerplate
	makeServer := func(resp string) *httptest.Server {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(resp))
		}))
		AuthWebhookURL = mustParseUrl(t, ts.URL)
		return ts
	}
	defer func() { AuthWebhookURL = nil }()
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
		{"name": "passthru_fps", "bitrate": 890, "width": 789, "height": 654, "profile": "H264ConstrainedHigh", "gop":"123"},
		{"name": "gop0", "bitrate": 800, "width": 400, "height": 220, "profile": "H264ConstrainedHigh", "gop":"0.0"}]}`)
	defer ts7.Close()
	params = createSid(u).(*core.StreamParameters)
	assert.Len(params.Profiles, 4)

	expectedProfiles := []ffmpeg.VideoProfile{
		{
			Name:         "prof1",
			Bitrate:      "432",
			Framerate:    uint(560),
			FramerateDen: 0,
			Resolution:   "123x456",
			Profile:      ffmpeg.ProfileH264Baseline,
			GOP:          time.Duration(0),
		},
		{
			Name:         "prof2",
			Bitrate:      "765",
			Framerate:    uint(876),
			FramerateDen: uint(12),
			Resolution:   "456x987",
			Profile:      ffmpeg.ProfileNone,
			GOP:          ffmpeg.GOPIntraOnly,
		},
		{
			Name:         "passthru_fps",
			Bitrate:      "890",
			Resolution:   "789x654",
			Framerate:    0,
			FramerateDen: 0,
			Profile:      ffmpeg.ProfileH264ConstrainedHigh,
			GOP:          time.Duration(123) * time.Second,
		},
		{
			Name:         "gop0",
			Bitrate:      "800",
			Resolution:   "400x220",
			Framerate:    0,
			FramerateDen: 0,
			Profile:      ffmpeg.ProfileH264ConstrainedHigh,
			GOP:          time.Duration(0),
		},
	}

	assert.Len(params.Profiles, 4)
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
		{"name": "passthru_fps", "bitrate": 890, "width": 789, "height": 654, "profile": "H264ConstrainedHigh", "gop":"123"},
		{"name": "gop0", "bitrate": 800, "width": 400, "height": 220, "profile": "H264ConstrainedHigh", "gop":"0.0"}]}`)

	defer ts9.Close()
	params = createSid(u).(*core.StreamParameters)
	jointProfiles := append([]ffmpeg.VideoProfile{ffmpeg.P240p30fps16x9, ffmpeg.P720p30fps16x9}, expectedProfiles...)

	assert.Len(params.Profiles, 6)
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
	// we accept gop: 0 now
	//ts12 := makeServer(`{"manifestID":"a", "profiles": [ {"gop": "0"}]}`)
	//defer ts12.Close()
	//assert.Nil(createSid(u))
	ts13 := makeServer(`{"manifestID":"a", "profiles": [ {"gop": "-0,01"}]}`)
	defer ts13.Close()
	assert.Nil(createSid(u))

	// intra only gop
	ts14 := makeServer(`{"manifestID":"a", "profiles": [ {"gop": "intra" }]}`)
	defer ts14.Close()
	params = createSid(u).(*core.StreamParameters)
	assert.Len(params.Profiles, 1)
	assert.Equal(ffmpeg.GOPIntraOnly, params.Profiles[0].GOP)

	// do not create stream if ObjectStore URL is invalid
	ts15 := makeServer(`{"manifestID":"a2", "objectStore": "invalid://object.store", "recordObjectStore": ""}`)
	defer ts15.Close()
	sid = createSid(u)
	assert.Nil(sid)

	// do not create stream if RecordObjectStore URL is invalid
	ts16 := makeServer(`{"manifestID":"a2", "objectStore": "", "recordObjectStore": "invalid://object.store"}`)
	defer ts16.Close()
	sid = createSid(u)
	assert.Nil(sid)

	ts17 := makeServer(`{"manifestID":"a3", "objectStore": "s3+http://us:pass@object.store/path", "recordObjectStore": "s3+http://us:pass@record.store"}`)
	defer ts17.Close()
	params = createSid(u).(*core.StreamParameters)
	assert.Equal(core.ManifestID("a3"), params.ManifestID)
	assert.NotNil(params.OS)
	assert.True(params.OS.IsExternal())
	osinfo := params.OS.GetInfo()
	assert.Equal(net.OSInfo_S3, osinfo.GetStorageType())
	assert.Equal("http://object.store/path", osinfo.GetS3Info().Host)
	assert.NotNil(params.RecordOS)
	assert.True(params.RecordOS.IsExternal())
	osinfo = params.RecordOS.GetInfo()
	assert.Equal(net.OSInfo_S3, osinfo.GetStorageType())
	assert.Equal("http://record.store", osinfo.GetS3Info().Host)

	// set scene classification detector profiles
	ts18 := makeServer(`{"manifestID":"a", "detection": {"freq": 5, "sampleRate": 10, "sceneClassification": [{"name": "soccer"}]}}`)
	defer ts18.Close()
	params = createSid(u).(*core.StreamParameters)
	detectorProf := ffmpeg.DSceneAdultSoccer
	detectorProf.SampleRate = 10
	expectedDetection := core.DetectionConfig{
		Freq:               5,
		SelectedClassNames: []string{"soccer"},
		Profiles: []ffmpeg.DetectorProfile{
			&detectorProf,
		},
	}
	assert.Equal(expectedDetection, params.Detection, "Did not have matching detector config")

	// do not create stream if detector class is unknown
	ts19 := makeServer(`{"manifestID":"a", "detection": {"freq": 5, "sampleRate": 10, "sceneClassification": [{"name": "Unknown class"}]}}`)
	defer ts19.Close()
	sid = createSid(u)
	assert.Nil(sid)
}

func TestGetHLSMasterPlaylistHandler(t *testing.T) {
	s, cancel := setupServerWithCancel()
	defer serverCleanup(s)
	defer cancel()
	orc := lpmscore.RetryCount
	srw := lpmscore.SegmenterRetryWait
	lpmscore.RetryCount = 1
	lpmscore.SegmenterRetryWait = 0
	defer func() {
		lpmscore.RetryCount = orc
		lpmscore.SegmenterRetryWait = srw
	}()
	vProfile := ffmpeg.P720p30fps16x9
	hlsStrmID := core.MakeStreamID(core.RandomManifestID(), &vProfile)

	ts, mux := stubTLSServer()
	defer ts.Close()

	segPath := "/transcoded/segment.ts"
	tSegData := []*net.TranscodedSegmentData{{Url: ts.URL + segPath, Pixels: 100}}
	dummyRes := func(tSegData []*net.TranscodedSegmentData) *net.TranscodeResult {
		return &net.TranscodeResult{
			Result: &net.TranscodeResult_Data{
				Data: &net.TranscodeData{
					Segments: tSegData,
				},
			},
		}
	}
	buf, err := proto.Marshal(dummyRes(tSegData))
	require.Nil(t, err)

	mux.HandleFunc("/segment", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(buf)
	})
	mux.HandleFunc(segPath, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("transcoded binary data"))
	})

	sess := StubBroadcastSession(ts.URL)
	sess.Params.Profiles = []ffmpeg.VideoProfile{ffmpeg.P720p30fps16x9}
	sess.Params.ManifestID = hlsStrmID.ManifestID
	bsm := bsmWithSessList([]*BroadcastSession{sess})

	osd := drivers.NewMemoryDriver(mustParseUrl(t, "test://some.host"))
	osSession := osd.NewSession("testPath")

	s.rtmpConnections[hlsStrmID.ManifestID] = &rtmpConnection{
		mid:         hlsStrmID.ManifestID,
		nonce:       7,
		pl:          core.NewBasicPlaylistManager(hlsStrmID.ManifestID, osSession, nil),
		profile:     &ffmpeg.P720p30fps16x9,
		sessManager: bsm,
		params:      &core.StreamParameters{Profiles: []ffmpeg.VideoProfile{ffmpeg.P720p30fps16x9}},
	}
	s.lastManifestID = hlsStrmID.ManifestID

	reader := strings.NewReader("InsteadOf.mp4")
	req, err := http.NewRequest(http.MethodPost, "/live/"+string(hlsStrmID.ManifestID)+"/1.ts", reader)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	s.HandlePush(w, req)

	bytes, err := ioutil.ReadAll(w.Body)
	require.NoError(t, err)
	println(string(bytes))
	require.Equal(t, http.StatusOK, w.Code)

	mlHandler := getHLSMasterPlaylistHandler(s)
	url2 := mustParseUrl(t, fmt.Sprintf("http://localhost/stream/%s.m3u8", hlsStrmID.ManifestID))

	//Test get master playlist
	pl, err := mlHandler(url2)
	require.NoError(t, err)
	require.NotNil(t, pl)
	require.Equal(t, 1, len(pl.Variants), "Expected 1 variant")

	mediaPLName := fmt.Sprintf("%s.m3u8", hlsStrmID)
	require.Equal(t, mediaPLName, pl.Variants[0].URI)

	// need to wait until SegmentRTMPToHLS loop exits (we don't have a way to force it)
	time.Sleep(100 * time.Millisecond)
}

func TestRegisterConnection(t *testing.T) {
	assert := assert.New(t)
	s, cancel := setupServerWithCancel()
	defer serverCleanup(s)
	defer cancel()
	mid := core.SplitStreamIDString(t.Name()).ManifestID
	strm := stream.NewBasicRTMPVideoStream(&core.StreamParameters{ManifestID: mid, Nonce: 1})

	// Should return an error if missing node storage
	drivers.NodeStorage = nil
	_, err := s.registerConnection(context.TODO(), strm, nil)
	assert.Equal(err, errStorage)
	drivers.NodeStorage = drivers.NewMemoryDriver(nil)

	// normal success case
	rand.Seed(123)
	cxn, err := s.registerConnection(context.TODO(), strm, nil)
	assert.NotNil(cxn)
	assert.Nil(err)

	// Check some properties of the cxn / server
	assert.Equal(mid, cxn.mid)
	assert.Equal("source", cxn.profile.Name)
	assert.Equal(uint64(1), cxn.nonce)

	assert.Equal(mid, s.LastManifestID())
	assert.Equal(string(mid)+"/source", s.LastHLSStreamID().String())
	assert.NotNil(cxn.params.OS)
	assert.Nil(cxn.params.OS.GetInfo())
	assert.NotNil(cxn.params.Capabilities)

	// Should return an error if creating another cxn with the same mid
	_, err = s.registerConnection(context.TODO(), strm, nil)
	assert.Equal(err, errAlreadyExists)

	// Check for params with an existing OS assigned
	storage := drivers.NewS3Driver("", "", "", "", false).NewSession("")
	strm = stream.NewBasicRTMPVideoStream(&core.StreamParameters{ManifestID: core.RandomManifestID(), OS: storage})
	cxn, err = s.registerConnection(context.TODO(), strm, nil)
	assert.Nil(err)
	assert.Equal(storage, cxn.params.OS)
	assert.Equal(net.OSInfo_S3, cxn.params.OS.GetInfo().StorageType)
	assert.Equal(cxn.params.OS, cxn.pl.GetOSSession())

	// check for capabilities
	profiles := []ffmpeg.VideoProfile{ffmpeg.P144p30fps16x9}
	strm = stream.NewBasicRTMPVideoStream(&core.StreamParameters{ManifestID: core.RandomManifestID(), Profiles: profiles})
	cxn, err = s.registerConnection(context.TODO(), strm, nil)
	assert.Nil(err)
	job, err := core.JobCapabilities(streamParams(strm.AppData()))
	assert.Nil(err)
	assert.Equal(job, cxn.params.Capabilities)

	// check for capabilities with codec specified
	inCodec := ffmpeg.H264
	strm = stream.NewBasicRTMPVideoStream(&core.StreamParameters{ManifestID: core.RandomManifestID(), Profiles: profiles})
	cxn, err = s.registerConnection(context.TODO(), strm, &inCodec)
	assert.Nil(err)
	assert.True(core.NewCapabilities([]core.Capability{core.Capability_H264}, []core.Capability{}).CompatibleWith(cxn.params.Capabilities.ToNetCapabilities()))
	assert.False(core.NewCapabilities([]core.Capability{core.Capability_HEVC_Decode}, []core.Capability{}).CompatibleWith(cxn.params.Capabilities.ToNetCapabilities()))

	inCodec = ffmpeg.H265
	profiles[0].Encoder = ffmpeg.H265
	strm = stream.NewBasicRTMPVideoStream(&core.StreamParameters{ManifestID: core.RandomManifestID(), Profiles: profiles})
	cxn, err = s.registerConnection(context.TODO(), strm, &inCodec)
	assert.Nil(err)
	assert.True(core.NewCapabilities([]core.Capability{
		core.Capability_HEVC_Decode,
		core.Capability_HEVC_Encode,
	}, []core.Capability{}).CompatibleWith(cxn.params.Capabilities.ToNetCapabilities()))

	// check for capabilities: exit with an invalid cap
	profiles[0].Format = -1
	strm = stream.NewBasicRTMPVideoStream(&core.StreamParameters{ManifestID: core.RandomManifestID(), Profiles: profiles})
	cxn, err = s.registerConnection(context.TODO(), strm, nil)
	assert.Nil(cxn)
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
			cxn, err := s.registerConnection(context.TODO(), strm, nil)

			assert.Nil(err)
			assert.NotNil(cxn)
			assert.Equal(mid, cxn.mid)
		}(i)
	}
	wg.Wait()

}

func TestCleanStreamPrefix(t *testing.T) {
	u := mustParseUrl(t, "http://localhost/stream/1220c50f8bc4d2a807aace1e1376496a9d7f7c1408dec2512763c3ca16fe828f6631_01.ts")
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
	assert.Nil(err)

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
	assert.Nil(err)
	assert.Equal(time.Duration(0), p[0].GOP)

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

func mustParseUrl(t *testing.T, str string) *url.URL {
	u, err := url.Parse(str)
	require.NoError(t, err)
	return u
}
