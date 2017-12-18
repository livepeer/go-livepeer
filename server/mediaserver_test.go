package server

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"net/url"
	"testing"

	"time"

	crypto "gx/ipfs/QmaPbCnUMBohSGo3KnxEa2bHqyJVVeEEcwtqJAYxerieBo/go-libp2p-crypto"

	"github.com/ericxtang/m3u8"
	"github.com/golang/glog"
	bnet "github.com/livepeer/go-livepeer-basicnet"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/net"
	"github.com/livepeer/lpms/segmenter"
	"github.com/livepeer/lpms/stream"
)

var S *LivepeerServer

func setupServer() *LivepeerServer {
	if S == nil {
		priv, pub, _ := crypto.GenerateKeyPair(crypto.RSA, 2048)
		node, err := bnet.NewNode(15000, priv, pub, &bnet.BasicNotifiee{})
		if err != nil {
			glog.Errorf("Error creating a new node: %v", err)
			return nil
		}
		nw, err := bnet.NewBasicVideoNetwork(node, "")
		if err != nil {
			glog.Errorf("Cannot create network node: %v", err)
			return nil
		}
		n, _ := core.NewLivepeerNode(nil, nw, "12209433a695c8bf34ef6a40863cfe7ed64266d876176aee13732293b63ba1637fd2", []string{"test"}, "./tmp")
		S = NewLivepeerServer("1935", "8080", "", n)
		go S.StartMediaServer(context.Background(), 0, "")
		go S.StartWebserver()
	}
	return S
}

var StubMasterPLStreamID = "12209433a695c8bf34ef6a40863cfe7ed64266d876176aee13732293b63ba1637fd2122f1cb223775a93e8b3102942f54aa2561e3931cc5ac0d57a39e0c064b243efPvideotest"

type StubNetwork struct {
	B *StubBroadcaster
	S *StubSubscriber
}

func (n *StubNetwork) String() string { return "" }
func (n *StubNetwork) GetNodeID() string {
	return "122011e494a06b20bf7a80f40e80d538675cc0b168c21912d33e0179617d5d4fe4e0"
}

func (n *StubNetwork) GetBroadcaster(strmID string) (net.Broadcaster, error) {
	n.B = &StubBroadcaster{Data: make(map[uint64][]byte)}
	return n.B, nil
}

func (n *StubNetwork) GetSubscriber(strmID string) (net.Subscriber, error) {
	if n.S == nil {
		n.S = &StubSubscriber{}
	}
	return n.S, nil
}

func (n *StubNetwork) Connect(nodeID string, nodeAddr []string) error { return nil }
func (n *StubNetwork) SetupProtocol() error                           { return nil }
func (b *StubNetwork) SendTranscodeResponse(nodeID string, strmID string, transcodeResult map[string]string) error {
	return nil
}
func (b *StubNetwork) ReceivedTranscodeResponse(strmID string, gotResult func(transcodeResult map[string]string)) {
}
func (b *StubNetwork) GetMasterPlaylist(nodeID string, strmID string) (chan *m3u8.MasterPlaylist, error) {
	mplc := make(chan *m3u8.MasterPlaylist)
	mpl := m3u8.NewMasterPlaylist()
	pl, _ := m3u8.NewMediaPlaylist(100, 100)
	mpl.Append(fmt.Sprintf("%v.m3u8", StubMasterPLStreamID), pl, m3u8.VariantParams{Bandwidth: 100})
	// glog.Infof("StubNetwork GetMasterPlaylist. mpl: %v", mpl)

	go func() {
		mplc <- mpl
		close(mplc)
	}()

	return mplc, nil
}

func (b *StubNetwork) UpdateMasterPlaylist(strmID string, mpl *m3u8.MasterPlaylist) error {
	return nil
}

func (n *StubNetwork) GetNodeStatus(nodeID string) (chan *net.NodeStatus, error) {
	return nil, nil
}

type StubBroadcaster struct {
	Data map[uint64][]byte
}

func (b *StubBroadcaster) IsWorking() bool { return false }
func (b *StubBroadcaster) String() string  { return "" }
func (b *StubBroadcaster) Broadcast(seqNo uint64, data []byte) error {
	b.Data[seqNo] = data
	return nil
}
func (b *StubBroadcaster) Finish() error { return nil }

type StubSubscriber struct {
	working         bool
	subscribeCalled bool
}

func (s *StubSubscriber) IsWorking() bool { return s.working }
func (s *StubSubscriber) String() string  { return "" }
func (s *StubSubscriber) Subscribe(ctx context.Context, f func(seqNo uint64, data []byte, eof bool)) error {
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

// Should publish RTMP stream, turn the RTMP stream into HLS, and broadcast the HLS stream.
func TestGotRTMPStreamHandler(t *testing.T) {
	s := setupServer()
	s.LivepeerNode.VideoNetwork = &StubNetwork{}
	s.RTMPSegmenter = &StubSegmenter{}
	handler := gotRTMPStreamHandler(s)

	url, _ := url.Parse("http://localhost/stream/test")
	strm := stream.NewBasicRTMPVideoStream("strmID")

	//Stream already exists
	s.LivepeerNode.VideoDB.AddNewRTMPStream("strmID")
	err := handler(url, strm)
	if err != ErrAlreadyExists {
		t.Errorf("Expecting publish error because stream already exists, but got: %v", err)
	}
	s.LivepeerNode.VideoDB.DeleteStream(core.StreamID("strmID"))

	//Try to handle test RTMP data.  There is a race condition somewhere, sleeping seems to make it go away...
	// time.Sleep(time.Millisecond * 500)
	if err := handler(url, strm); err != nil {
		t.Errorf("Error: %v", err)
	}

	hlsStrmID := s.broadcastRtmpToHLSMap["strmID"]
	if hlsStrmID == "" {
		t.Errorf("HLS stream ID should exist")
	}
	hlsStrm := s.LivepeerNode.VideoDB.GetHLSStream(core.StreamID(hlsStrmID))
	if hlsStrm == nil {
		t.Errorf("HLS stream should exist")
	}
	start := time.Now()
	s1, err := hlsStrm.GetHLSSegment("seg1.ts")
	for time.Since(start) < time.Second*2 {
		if err == stream.ErrNotFound {
			time.Sleep(100 * time.Millisecond)
			s1, err = hlsStrm.GetHLSSegment("seg1.ts")
			continue
		} else {
			break
		}
	}

	if err != nil {
		t.Errorf("Error getting segment: %v", err)
	}
	if s1.SeqNo != 1 {
		t.Errorf("Expecting seqno 1, but got %v", s1.SeqNo)
	}

	//Wait for the video to be segmented and broadcasted
	sn, ok := s.LivepeerNode.VideoNetwork.(*StubNetwork)
	if !ok {
		t.Errorf("VideoNetwork not assigned correctly.")
	}
	start = time.Now()
	for time.Since(start) < time.Second*5 {
		if sn.B == nil || len(sn.B.Data) < 2 {
			time.Sleep(100 * time.Millisecond)
		} else {
			break
		}
	}

	if len(sn.B.Data) != 4 {
		t.Errorf("Should have recieved 4 data chunks, got:%v", len(sn.B.Data))
	}

	seg0, _ := core.BytesToSignedSegment(sn.B.Data[0])
	seg1, _ := core.BytesToSignedSegment(sn.B.Data[1])
	seg2, _ := core.BytesToSignedSegment(sn.B.Data[2])
	seg3, _ := core.BytesToSignedSegment(sn.B.Data[3])
	if seg0.Seg.Name != "seg0.ts" || seg1.Seg.Name != "seg1.ts" || seg2.Seg.Name != "seg2.ts" {
		t.Errorf("Wrong segments: %v, %v, %v, %v", seg0, seg1, seg2, seg3)
	}
}

func TestGetHLSMasterPlaylistHandler(t *testing.T) {
	s := setupServer()
	s.LivepeerNode.VideoNetwork = &StubNetwork{}
	url, _ := url.Parse("http://localhost/stream/12209433a695c8bf34ef6a40863cfe7ed64266d876176aee13732293b63ba1637fd2122f1cb223775a93e8b3102942f54aa2561e3931cc5ac0d57a39e0c064b243ee.m3u8")

	//Set up the VideoDB so it already has a manifest with a local stream
	manifest, err := s.LivepeerNode.VideoDB.AddNewHLSManifest("12209433a695c8bf34ef6a40863cfe7ed64266d876176aee13732293b63ba1637fd2122f1cb223775a93e8b3102942f54aa2561e3931cc5ac0d57a39e0c064b243ee")
	if err != nil {
		t.Errorf("Error creating hls stream: %v", err)
	}

	strm := stream.NewBasicHLSVideoStream("strm", 3)
	manifest.AddVideoStream(strm, &m3u8.Variant{URI: "strm.m3u8", VariantParams: m3u8.VariantParams{Bandwidth: 100}})
	// if err = s.LivepeerNode.VideoDB.AddHLSVariant("1220e3fd52491fc1691d6a5b45b7f21244640bd3b5cfbe2a59b3f5a8f6f1eb9e39a8strmID", "strm", &m3u8.Variant{URI: "strm.m3u8", VariantParams: m3u8.VariantParams{Bandwidth: 100}}); err != nil {
	// 	t.Errorf("Error adding variant: %v", err)
	// }

	mpl, _ := manifest.GetManifest()
	if len(mpl.Variants) != 1 {
		t.Errorf("Expecting 1 variant, but got %v", mpl)
	}

	//Test master playlist
	handler := getHLSMasterPlaylistHandler(s)
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

	//Remove the local stream to test getting stream from the network
	s.LivepeerNode.VideoDB.DeleteHLSManifest("12209433a695c8bf34ef6a40863cfe7ed64266d876176aee13732293b63ba1637fd2122f1cb223775a93e8b3102942f54aa2561e3931cc5ac0d57a39e0c064b243ee")
	pl, err = handler(url)
	if err != nil {
		t.Errorf("Error handling getHLSMasterPlaylist", err)
	}

	if len(pl.Variants) != 1 {
		t.Errorf("Expecting 1 variant, but got %v", pl)
	}
	if pl.Variants[0].URI != fmt.Sprintf("%v.m3u8", StubMasterPLStreamID) {
		t.Errorf("Expecting %v.m3u8, but got: %v", StubMasterPLStreamID, pl.Variants[0].URI)
	}

	mfst := s.LivepeerNode.VideoDB.GetHLSManifest("12209433a695c8bf34ef6a40863cfe7ed64266d876176aee13732293b63ba1637fd2122f1cb223775a93e8b3102942f54aa2561e3931cc5ac0d57a39e0c064b243ee")
	if mfst == nil {
		t.Errorf("Expecting to be able to look up via manifest names, but got nil")
	}
}

func TestGetHLSMediaPlaylistHandler(t *testing.T) {
	HLSWaitTime = time.Second * 3 //Make the waittime shorter to speed up the test suite
	strmID := "12209433a695c8bf34ef6a40863cfe7ed64266d876176aee13732293b63ba1637fd2122f1cb223775a93e8b3102942f54aa2561e3931cc5ac0d57a39e0c064b243eePTestVideo"
	s := setupServer()
	s.LivepeerNode.VideoNetwork = &StubNetwork{}
	url, _ := url.Parse(fmt.Sprintf("http://localhost/stream/%v.m3u8", strmID))

	handler := getHLSMediaPlaylistHandler(s)

	//Set up a local stream, should return that.
	glog.Infof("VideoDB: %v", s.LivepeerNode.VideoDB)
	hlsStrm, err := s.LivepeerNode.VideoDB.AddNewHLSStream(core.StreamID(strmID))
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	hlsStrm.AddHLSSegment(&stream.HLSSegment{SeqNo: 0, Name: "seg0.ts", Data: []byte("hello"), Duration: 8})
	hlsStrm.AddHLSSegment(&stream.HLSSegment{SeqNo: 1, Name: "seg1.ts", Data: []byte("hello"), Duration: 8})
	hlsStrm.AddHLSSegment(&stream.HLSSegment{SeqNo: 2, Name: "seg2.ts", Data: []byte("hello"), Duration: 8})
	handler = getHLSMediaPlaylistHandler(s)
	pl, err := handler(url)
	if err != nil {
		t.Errorf("Error in handler: %v", err)
	}
	if pl.Count() == 0 {
		t.Errorf("Expecting segments, but got %v", pl)
	}
	if pl.Segments[0].URI != "seg0.ts" || pl.Segments[0].Duration != 8 {
		t.Errorf("Wrong segment info: %v", pl.Segments[0])
	}

	//Don't set up local stream (test should call subscribe to network)
	s.LivepeerNode.VideoDB = core.NewVideoDB(string(s.LivepeerNode.Identity))
	m, err := s.LivepeerNode.VideoDB.AddNewHLSManifest(core.ManifestID("12209433a695c8bf34ef6a40863cfe7ed64266d876176aee13732293b63ba1637fd2122f1cb223775a93e8b3102942f54aa2561e3931cc5ac0d57a39e0c064b243ee"))
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	tmpStrm := stream.NewBasicHLSVideoStream(strmID, 3)
	m.AddVideoStream(tmpStrm, &m3u8.Variant{URI: fmt.Sprintf("%v.m3u8", strmID), VariantParams: m3u8.VariantParams{Bandwidth: 100}})
	pl, err = handler(url)
	if err != nil {
		t.Errorf("Error in handler: %v", err)
	}
	hlsStrm = s.LivepeerNode.VideoDB.GetHLSStream(core.StreamID(strmID))
	if hlsStrm == nil {
		t.Errorf("Expecting there to be a stream")
	}

	//Wait a few seconds to make sure segments are populated.
	start := time.Now()
	for time.Since(start) < time.Second*2 {
		s, err := hlsStrm.GetHLSSegment("strmID_01.ts")
		if err != nil || s == nil {
			time.Sleep(50 * time.Millisecond)
		}
	}

	glog.Infof("strm: %v", hlsStrm)
	//Test the data from stub subscriber
	for _, i := range []int{1, 2, 3} {
		s, err := hlsStrm.GetHLSSegment(fmt.Sprintf("strmID_0%v.ts", i))
		if err != nil || s == nil {
			t.Errorf("Error getting hlsSegment%v: %v", i, err)
		}
		if string(s.Data) != "test data" {
			t.Errorf("Expecting 'test data', but got %v", s.Data)
		}
	}
	if pl.Count() != 3 {
		t.Errorf("Expecting count to be 3, got %v", pl.Count())
	}
}

func TestParseSegname(t *testing.T) {
	u, _ := url.Parse("http://localhost/stream/1220c50f8bc4d2a807aace1e1376496a9d7f7c1408dec2512763c3ca16fe828f6631_01.ts")
	segName := parseSegName(u.Path)
	if segName != "1220c50f8bc4d2a807aace1e1376496a9d7f7c1408dec2512763c3ca16fe828f6631_01.ts" {
		t.Errorf("Expecting %v, but %v", "1220c50f8bc4d2a807aace1e1376496a9d7f7c1408dec2512763c3ca16fe828f6631_01.ts", segName)
	}
}
