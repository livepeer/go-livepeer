package server

import (
	"context"
	"fmt"
	"math/big"
	"net/url"
	"testing"
	"time"

	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/drivers"
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

type StubSegmenter struct{}

func (s *StubSegmenter) SegmentRTMPToHLS(ctx context.Context, rs stream.RTMPVideoStream, hs stream.HLSVideoStream, segOptions segmenter.SegmenterOptions) error {
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
	strm := stream.NewBasicRTMPVideoStream("strmID")

	//Stream already exists
	s.rtmpStreams["strmID"] = strm
	err = handler(url, strm)
	if err != ErrAlreadyExists {
		t.Errorf("Expecting publish error because stream already exists, but got: %v", err)
	}
	delete(s.rtmpStreams, "strmID")
	delete(s.VideoNonce, "strmID")

	//Try to handle test RTMP data.
	if err := handler(url, strm); err != nil {
		t.Errorf("Error: %v", err)
	}

	sid := core.StreamID(hlsStrmID)

	start := time.Now()
	for time.Since(start) < time.Second*2 {
		pl := s.CurrentPlaylist.GetHLSMediaPlaylist(sid)
		if pl == nil || len(pl.Segments) != 4 {
			time.Sleep(100 * time.Millisecond)
			continue
		} else {
			break
		}
	}
	pl := s.CurrentPlaylist.GetHLSMediaPlaylist(sid)
	if pl == nil {
		t.Error("Expected media playlist; got none")
	}

	if pl.Count() != 4 {
		t.Errorf("Should have recieved 4 data chunks, got: %v", pl.Count())
	}

	mid, err := core.MakeManifestID(hlsStrmID.GetVideoID())
	if err != nil {
		t.Fatal(err)
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
	delete(s.rtmpStreams, "strmID")
	delete(s.VideoNonce, "strmID")
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
	strm := stream.NewBasicRTMPVideoStream("strmID")

	if err := handler(url, strm); err != nil {
		t.Errorf("Error: %v", err)
	}

	segName := "test_seg/1.ts"
	err = s.CurrentPlaylist.InsertHLSSegment(hlsStrmID, 1, segName, 12)
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

	if !shouldStopStream(fmt.Errorf(JobOutOfRangeError)) {
		t.Errorf("Expected shouldStopStream=true for %v", JobOutOfRangeError)
	}
}
