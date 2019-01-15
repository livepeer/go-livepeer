package server

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/livepeer/go-livepeer/pm"
	"github.com/stretchr/testify/assert"
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

func setupServer() *LivepeerServer {
	drivers.NodeStorage = drivers.NewMemoryDriver("")
	if S == nil {
		n, _ := core.NewLivepeerNode(nil, "./tmp", nil)
		S = NewLivepeerServer("127.0.0.1:1938", "127.0.0.1:8080", n)
		go S.StartMediaServer(context.Background(), big.NewInt(0), "")
		go S.StartWebserver("127.0.0.1:8938")
	}
	return S
}

type stubDiscovery struct {
	infos []*net.OrchestratorInfo
}

func (d *stubDiscovery) GetOrchestrators(num int) ([]*net.OrchestratorInfo, error) {
	return d.infos, nil
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

func TestStartBroadcast(t *testing.T) {
	s := setupServer()

	defer func() {
		s.LivepeerNode.Sender = nil
	}()

	// Empty discovery
	mid := core.RandomManifestID()
	storage := drivers.NodeStorage.NewSession(string(mid))
	pl := core.NewBasicPlaylistManager(mid, storage)
	if _, err := s.startBroadcast(pl); err != ErrDiscovery {
		t.Error("Expected error with discovery")
	}

	sd := &stubDiscovery{}
	// Discovery returned no orchestrators
	s.LivepeerNode.OrchestratorPool = sd
	if sess, _ := s.startBroadcast(pl); sess != nil {
		t.Error("Expected nil session")
	}

	// populate stub discovery
	sd.infos = []*net.OrchestratorInfo{
		&net.OrchestratorInfo{},
		&net.OrchestratorInfo{},
	}
	sess, _ := s.startBroadcast(pl)
	if sess == nil {
		t.Error("Expected nil session")
	}
	// Sanity check a few easy fields
	if sess.ManifestID != mid {
		t.Error("Expected manifest id")
	}
	if sess.BroadcasterOS != storage {
		t.Error("Unexpected broadcaster OS")
	}
	if sess.OrchestratorInfo != sd.infos[0] || sd.infos[0] == sd.infos[1] {
		t.Error("Unexpected orchestrator info")
	}
	if sess.Sender != nil {
		t.Error("Unexpected sender")
	}
	if sess.PMSessionID != "" {
		t.Error("Unexpected PM sessionID")
	}

	// Test start PM session
	sender := &pm.MockSender{}
	s.LivepeerNode.Sender = sender

	params := pm.TicketParams{
		Recipient:         pm.RandAddress(),
		FaceValue:         big.NewInt(1234),
		WinProb:           big.NewInt(5678),
		RecipientRandHash: pm.RandHash(),
		Seed:              big.NewInt(7777),
	}
	protoParams := &net.TicketParams{
		Recipient:         params.Recipient.Bytes(),
		FaceValue:         params.FaceValue.Bytes(),
		WinProb:           params.WinProb.Bytes(),
		RecipientRandHash: params.RecipientRandHash.Bytes(),
		Seed:              params.Seed.Bytes(),
	}

	sd.infos = []*net.OrchestratorInfo{
		&net.OrchestratorInfo{
			TicketParams: protoParams,
		},
	}

	expSessionID := "foo"
	sender.On("StartSession", params).Return(expSessionID)

	sess, err := s.startBroadcast(pl)
	require.Nil(t, err)

	assert := assert.New(t)
	assert.Equal(sender, sess.Sender)
	assert.Equal(expSessionID, sess.PMSessionID)
}

func TestCreateRTMPStreamHandler(t *testing.T) {

	// Monkey patch rng to avoid unpredictability even when seeding
	oldRandFunc := core.RandomIdGenerator
	core.RandomIdGenerator = func(length uint) []byte {
		return []byte("abcdef")
	}
	defer func() { core.RandomIdGenerator = oldRandFunc }()

	s := setupServer()
	s.RTMPSegmenter = &StubSegmenter{skip: true}
	handler := gotRTMPStreamHandler(s)
	createSid := createRTMPStreamIDHandler(s)
	endHandler := endRTMPStreamHandler(s)

	// Test hlsStreamID query param
	key := hex.EncodeToString(core.RandomIdGenerator(StreamKeyBytes))
	expectedSid := core.MakeStreamIDFromString("ghijkl", key)
	u, _ := url.Parse("rtmp://localhost?manifestID=" + expectedSid.String()) // with key
	if sid := createSid(u); sid != expectedSid.String() {
		t.Error("Unexpected streamid")
	}
	expectedMid := "mnopq"
	u, _ = url.Parse("rtmp://localhost?manifestID=" + string(expectedMid)) // without key
	if sid := createSid(u); sid != string(expectedMid)+"/"+key {
		t.Error("Unexpected streamid")
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
	if sid := createSid(u); sid != "" {
		t.Error("Expected failure due to naming collision")
	}
	// Ensure the stream ID is reusable after the stream ends
	if err := endHandler(u, st); err != nil {
		t.Error("Could not clean up stream")
	}
	if sid := createSid(u); sid != st.GetStreamID() {
		t.Error("Mismatched streamid during stream reuse")
	}

	// Test a couple of odd cases; subset of parseManifestID checks
	// (Would be nice to stub out parseManifestID to receive stronger
	//  transitive assurance via existing parseManifestID tests)
	testManifestIDQueryParam := func(inp string) {
		// This isn't a great test because if the query param ever changes,
		// this test will still pass
		u, _ := url.Parse("rtmp://localhost?manifestID=" + url.QueryEscape(inp))
		if sid := createSid(u); sid != st.GetStreamID() {
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
	s.RTMPSegmenter = &StubSegmenter{skip: true}
	createSid := createRTMPStreamIDHandler(s)
	handler := gotRTMPStreamHandler(s)
	endHandler := endRTMPStreamHandler(s)
	u, _ := url.Parse("rtmp://localhost")
	sid := createSid(u)
	st := stream.NewBasicRTMPVideoStream(sid)

	// Nonexistent stream
	if err := endHandler(u, st); err != ErrUnknownStream {
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
	if err := endHandler(u, st); err != ErrUnknownStream {
		t.Error("Stream was not cleaned up properly ", err)
	}
}

// Should publish RTMP stream, turn the RTMP stream into HLS, and broadcast the HLS stream.
func TestGotRTMPStreamHandler(t *testing.T) {
	s := setupServer()
	s.RTMPSegmenter = &StubSegmenter{}
	handler := gotRTMPStreamHandler(s)

	vProfile := ffmpeg.P720p30fps16x9
	hlsStrmID := core.MakeStreamID(core.ManifestID("ghijkl"), &vProfile)
	u, _ := url.Parse("rtmp://localhost:1935/movie")
	strm := stream.NewBasicRTMPVideoStream(hlsStrmID.String())
	expectedSid := core.MakeStreamIDFromString(string(hlsStrmID.ManifestID), "source")

	// Check for invalid node storage
	oldStorage := drivers.NodeStorage
	drivers.NodeStorage = nil
	if err := handler(u, strm); err != ErrStorage {
		t.Error("Expected storage error ", err)
	}
	drivers.NodeStorage = oldStorage

	//Try to handle test RTMP data.
	if err := handler(u, strm); err != nil {
		t.Errorf("Error: %v", err)
	}

	// Check assigned IDs
	mid := rtmpManifestID(strm)
	if s.LatestPlaylist().ManifestID() != mid || LastManifestID != mid {
		t.Error("Unexpected Manifest ID")
	}
	if LastHLSStreamID != expectedSid {
		t.Error("Unexpected Stream ID ", LastHLSStreamID, expectedSid)
	}

	//Stream already exists
	if err := handler(u, strm); err != ErrAlreadyExists {
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
		t.Errorf("Should have recieved 4 data chunks, got: %v", pl.Count())
	}

	for i := 0; i < 4; i++ {
		seg := pl.Segments[i]
		shouldSegName := fmt.Sprintf("/stream/%s/%s/%d.ts", mid, expectedSid.Rendition, i)
		if seg.URI != shouldSegName {
			t.Fatalf("Wrong segment, should have URI %s, has %s", shouldSegName, seg.URI)
		}
	}
}

func TestMultiStream(t *testing.T) {
	s := setupServer()
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
	mut := sync.Mutex{}
	completed := syncStreams
	ch := make(chan struct{})
	const asyncStreams = 500
	for i := completed; i < asyncStreams; i++ {
		go func(i int) {
			handleStream(i)
			mut.Lock()
			completed++
			mut.Unlock()
			if completed >= asyncStreams {
				ch <- struct{}{}
			}
		}(i)
	}
	// block until complete
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	select {
	case <-ch:
		close(ch)
		if len(s.rtmpConnections) < asyncStreams {
			t.Error("Did not have expected number of streams", len(s.rtmpConnections))
		}
	case <-ctx.Done():
		t.Error("Timed out at ", completed)
		close(ch)
	}

	// probably should test (concurrent) cleanups as well
}

func TestGetHLSMasterPlaylistHandler(t *testing.T) {
	glog.Infof("\n\nTestGetHLSMasterPlaylistHandler...\n")

	s := setupServer()
	handler := gotRTMPStreamHandler(s)

	vProfile := ffmpeg.P720p30fps16x9
	hlsStrmID := core.MakeStreamID(core.RandomManifestID(), &vProfile)
	url, _ := url.Parse("rtmp://localhost:1935/movie")
	strm := stream.NewBasicRTMPVideoStream(string(hlsStrmID.ManifestID) + "/source")

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
	if shouldStopStream(fmt.Errorf("some random error string")) {
		t.Error("Expected shouldStopStream=false for a random error")
	}
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
