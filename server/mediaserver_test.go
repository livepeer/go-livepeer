package server

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"math/big"
	"net/url"
	"testing"
	"time"

	"github.com/ericxtang/m3u8"
	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/lpms/segmenter"
	"github.com/livepeer/lpms/stream"
)

var S *LivepeerServer

func setupServer() *LivepeerServer {
	if S == nil {
		n, _ := core.NewLivepeerNode(nil, "./tmp", nil)
		S = NewLivepeerServer("127.0.0.1:1938", "127.0.0.1:8080", n)
		go S.StartMediaServer(context.Background(), big.NewInt(0), "")
		go S.StartWebserver("127.0.0.1:8938")
	}
	return S
}

type StubSubscriber struct {
	working         bool
	subscribeCalled bool
}

func (s *StubSubscriber) IsLive() bool   { return s.working }
func (s *StubSubscriber) String() string { return "" }
func (s *StubSubscriber) Subscribe(ctx context.Context, f func(seqNo uint64, data []byte, eof bool)) error {
	// This does nothing right now; remove?
	glog.Infof("Calling StubSubscriber!!!")
	s.subscribeCalled = true
	s1 := core.SignedSegment{Seg: stream.HLSSegment{SeqNo: 0, Name: "strmID_01.ts", Data: []byte("test data"), Duration: 8.001}}
	s2 := core.SignedSegment{Seg: stream.HLSSegment{SeqNo: 1, Name: "strmID_02.ts", Data: []byte("test data"), Duration: 8.001}}
	s3 := core.SignedSegment{Seg: stream.HLSSegment{SeqNo: 2, Name: "strmID_03.ts", Data: []byte("test data"), Duration: 8.001}}
	for i, s := range []core.SignedSegment{s1, s2, s3} {
		var buf bytes.Buffer
		enc := gob.NewEncoder(&buf)
		err := enc.Encode(s)
		if err != nil {
			glog.Errorf("Error encoding segment to []byte: %v", err)
			continue
		}
		f(uint64(i), buf.Bytes(), false)
	}
	s.working = true
	return nil
}
func (s *StubSubscriber) Unsubscribe() error { return nil }

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

func (s *StubSegmenter) SubscribeToSegmenter(ctx context.Context, rs stream.RTMPVideoStream, segOptions segmenter.SegmenterOptions) (stream.Subscriber, error) {
	sub := &StubSubscriber{}
	return sub, nil
}

// Should publish RTMP stream, turn the RTMP stream into HLS, and broadcast the HLS stream.
func TestGotRTMPStreamHandler(t *testing.T) {
	s := setupServer()
	s.RTMPSegmenter = &StubSegmenter{}
	handler := gotRTMPStreamHandler(s)

	hlsStrmID := "10f6afa01868f11f5722434aa4a0769842e04fac75dfaccece208c5710fd52e0"
	url, _ := url.Parse(fmt.Sprintf("rtmp://localhost:1935/movie?hlsStrmID=%v", hlsStrmID))
	strm := stream.NewBasicRTMPVideoStream("strmID")

	//Stream already exists
	s.rtmpStreams["strmID"] = strm
	err := handler(url, strm)
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
		pl := s.LivepeerNode.VideoSource.GetHLSMediaPlaylist(sid)
		if pl == nil || len(pl.Segments) != 4 {
			time.Sleep(100 * time.Millisecond)
			continue
		} else {
			break
		}
	}

	pl := s.LivepeerNode.VideoSource.GetHLSMediaPlaylist(sid)
	if pl == nil {
		t.Error("Expected media playlist; got none")
	}

	if pl.Count() != 4 {
		t.Errorf("Should have recieved 4 data chunks, got: %v", pl.Count())
	}

	seg0 := pl.Segments[0]
	seg1 := pl.Segments[1]
	seg2 := pl.Segments[2]
	seg3 := pl.Segments[3]
	if seg0.URI != "seg0.ts" || seg1.URI != "seg1.ts" || seg2.URI != "seg2.ts" || seg3.URI != "seg3.ts" {
		t.Errorf("Wrong segments: %v, %v, %v, %v", seg0, seg1, seg2, seg3)
	}
}

func TestGetHLSMasterPlaylistHandler(t *testing.T) {
	glog.Infof("\n\nTestGetHLSMasterPlaylistHandler...\n")

	s := setupServer()

	//Set up the stubnet so it already has a manifest with a local stream
	mpl := m3u8.NewMasterPlaylist()
	mpl.Append("strm.m3u8", nil, m3u8.VariantParams{Bandwidth: 100})
	s.LivepeerNode.VideoSource.UpdateHLSMasterPlaylist("10f6afa01868f11f5722434aa4a0769842e04fac75dfaccece208c5710fd52e0", mpl)
	if len(mpl.Variants) != 1 {
		t.Errorf("Expecting 1 variant, but got %v", mpl)
	}

	handler := getHLSMasterPlaylistHandler(s)
	url, _ := url.Parse("http://localhost/stream/10f6afa01868f11f5722434aa4a0769842e04fac75dfaccece208c5710fd52e0.m3u8")

	//Test get master playlist
	pl, err := handler(url)
	if err != nil {
		t.Errorf("Error handling getHLSMasterPlaylist: %v", err)
	}

	if len(pl.Variants) != 1 {
		t.Errorf("Expecting 1 variant, but got %v", pl)
	}
	if pl.Variants[0].URI != "strm.m3u8" {
		t.Errorf("Expecting strm.m3u8, but got: %v", pl.Variants[0].URI)
	}
}

func TestParseSegname(t *testing.T) {
	u, _ := url.Parse("http://localhost/stream/1220c50f8bc4d2a807aace1e1376496a9d7f7c1408dec2512763c3ca16fe828f6631_01.ts")
	segName := parseSegName(u.Path)
	if segName != "1220c50f8bc4d2a807aace1e1376496a9d7f7c1408dec2512763c3ca16fe828f6631_01.ts" {
		t.Errorf("Expecting %v, but %v", "1220c50f8bc4d2a807aace1e1376496a9d7f7c1408dec2512763c3ca16fe828f6631_01.ts", segName)
	}
}
