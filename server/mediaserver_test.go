package server

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"math/rand"
	"net/url"
	"sync"
	"testing"
	"time"

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

	// Empty discovery
	mid := core.ManifestID(core.RandomVideoID())
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
}

func TestCreateRTMPStreamHandler(t *testing.T) {
	s := setupServer()
	s.RTMPSegmenter = &StubSegmenter{skip: true}
	handler := gotRTMPStreamHandler(s)
	createSid := createRTMPStreamIDHandler(s)
	endHandler := endRTMPStreamHandler(s)

	// Test hlsStreamID query param
	rand.Seed(123)
	key := hex.EncodeToString(core.RandomIdGenerator(StreamKeyBytes))
	expectedSid, _ := core.MakeStreamID(core.RandomVideoID(), key)
	rand.Seed(123)
	u, _ := url.Parse("rtmp://localhost?hlsStrmID=" + expectedSid.String())
	if sid := createSid(u); sid != expectedSid.String() {
		t.Error("Unexpected streamid")
	}
	// Test normal case
	rand.Seed(123)
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
	rand.Seed(123)
	if sid := createSid(u); sid != "" {
		t.Error("Expected failure due to naming collision")
	}
	// Ensure the stream ID is reusable after the stream ends
	if err := endHandler(u, st); err != nil {
		t.Error("Could not clean up stream")
	}
	rand.Seed(123)
	if sid := createSid(u); sid != st.GetStreamID() {
		t.Error("Mismatched streamid during stream reuse")
	}
	// Test invalid ManifestID
	u, _ = url.Parse("rtmp://localhost?hlsStrmID=abc")
	if sid := createSid(u); sid != "" {
		t.Error("Failed to create streamid")
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
	// Stream has Invalid manifest ID
	if err := endHandler(u, stream.NewBasicRTMPVideoStream("abc")); err != core.ErrManifestID {
		t.Error("Expected invalid manifest")
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
	hlsStrmID, err := core.MakeStreamID(core.RandomVideoID(), vProfile.Name)
	if err != nil {
		t.Fatal(err)
	}
	url, _ := url.Parse(fmt.Sprintf("rtmp://localhost:1935/movie?hlsStrmID=%v", hlsStrmID))
	strm := stream.NewBasicRTMPVideoStream(hlsStrmID.String())

	// Check for invalid Stream ID
	badStream := stream.NewBasicRTMPVideoStream("strmID")
	if err := handler(url, badStream); err != core.ErrManifestID {
		t.Error("Expected invalid manifest ID ", err)
	}

	// Check for invalid node storage
	oldStorage := drivers.NodeStorage
	drivers.NodeStorage = nil
	if err := handler(url, strm); err != ErrStorage {
		t.Error("Expected storage error ", err)
	}
	drivers.NodeStorage = oldStorage

	//Try to handle test RTMP data.
	if err := handler(url, strm); err != nil {
		t.Errorf("Error: %v", err)
	}

	// Check assigned IDs
	mid, err := rtmpManifestID(strm)
	if err != nil {
		t.Error(err)
	}
	if s.LatestPlaylist().ManifestID() != mid || LastManifestID != mid {
		t.Error("Unexpected Manifest ID")
	}
	if LastHLSStreamID != hlsStrmID {
		t.Error("Unexpected Stream ID")
	}

	//Stream already exists
	err = handler(url, strm)
	if err != ErrAlreadyExists {
		t.Errorf("Expecting publish error because stream already exists, but got: %v", err)
	}

	sid := core.StreamID(hlsStrmID)

	start := time.Now()
	for time.Since(start) < time.Second*2 {
		pl := s.LatestPlaylist().GetHLSMediaPlaylist(sid)
		if pl == nil || len(pl.Segments) != 4 {
			time.Sleep(100 * time.Millisecond)
			continue
		} else {
			break
		}
	}
	pl := s.LatestPlaylist().GetHLSMediaPlaylist(sid)
	if pl == nil {
		t.Error("Expected media playlist; got none")
	}

	if pl.Count() != 4 {
		t.Errorf("Should have recieved 4 data chunks, got: %v", pl.Count())
	}

	rendition := hlsStrmID.GetRendition()
	for i := 0; i < 4; i++ {
		seg := pl.Segments[i]
		shouldSegName := fmt.Sprintf("%s/%s/%d.ts", mid, rendition, i)
		t.Log(shouldSegName)
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
	hlsStrmID, err := core.MakeStreamID(core.RandomVideoID(), vProfile.Name)
	if err != nil {
		t.Fatal(err)
	}
	url, _ := url.Parse(fmt.Sprintf("rtmp://localhost:1935/movie?hlsStrmID=%v", hlsStrmID))
	strm := stream.NewBasicRTMPVideoStream(hlsStrmID.String())

	if err := handler(url, strm); err != nil {
		t.Errorf("Error: %v", err)
	}

	segName := "test_seg/1.ts"
	err = s.LatestPlaylist().InsertHLSSegment(hlsStrmID, 1, segName, 12)
	if err != nil {
		t.Fatal(err)
	}
	mid, err := core.MakeManifestID(hlsStrmID.GetVideoID())
	if err != nil {
		t.Fatal(err)
	}

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

func TestParseSegname(t *testing.T) {
	u, _ := url.Parse("http://localhost/stream/1220c50f8bc4d2a807aace1e1376496a9d7f7c1408dec2512763c3ca16fe828f6631_01.ts")
	segName := parseSegName(u.Path)
	if segName != "1220c50f8bc4d2a807aace1e1376496a9d7f7c1408dec2512763c3ca16fe828f6631_01.ts" {
		t.Errorf("Expecting %v, but %v", "1220c50f8bc4d2a807aace1e1376496a9d7f7c1408dec2512763c3ca16fe828f6631_01.ts", segName)
	}
}

func TestShouldStopStream(t *testing.T) {
	if shouldStopStream(fmt.Errorf("some random error string")) {
		t.Error("Expected shouldStopStream=false for a random error")
	}
}
