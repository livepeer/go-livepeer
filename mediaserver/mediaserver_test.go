package mediaserver

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"net/url"
	"testing"

	"time"

	crypto "gx/ipfs/QmaPbCnUMBohSGo3KnxEa2bHqyJVVeEEcwtqJAYxerieBo/go-libp2p-crypto"

	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/net"
	"github.com/livepeer/lpms/segmenter"
	"github.com/livepeer/lpms/stream"
)

var S *LivepeerMediaServer

func setupServer() *LivepeerMediaServer {
	if S == nil {
		priv, pub, _ := crypto.GenerateKeyPair(crypto.RSA, 2048)
		node, err := net.NewNode(15000, priv, pub)
		if err != nil {
			glog.Errorf("Error creating a new node: %v", err)
			return nil
		}
		nw, err := net.NewBasicVideoNetwork(node)
		if err != nil {
			glog.Errorf("Cannot create network node: %v", err)
			return nil
		}
		n, _ := core.NewLivepeerNode(nil, nw)
		S = NewLivepeerMediaServer("1935", "8080", "", n)
		go S.StartMediaServer(context.Background())
	}
	return S
}

// func TestStartMediaServer(t *testing.T) {
// 	s := setupServer()
// 	time.Sleep(time.Millisecond * 100)

// 	if s.hlsSubTimer == nil {
// 		t.Errorf("hlsSubTimer should be initiated")
// 	}

// 	if s.hlsWorkerRunning == false {
// 		t.Errorf("Should have kicked off hlsUnsubscribeWorker go routine")
// 	}
// }

type StubNetwork struct {
	B *StubBroadcaster
	S *StubSubscriber
}

func (n *StubNetwork) GetNodeID() string {
	return "122011e494a06b20bf7a80f40e80d538675cc0b168c21912d33e0179617d5d4fe4e0"
}

func (n *StubNetwork) GetBroadcaster(strmID string) (net.Broadcaster, error) {
	n.B = &StubBroadcaster{Data: make(map[uint64][]byte)}
	return n.B, nil
}

func (n *StubNetwork) GetSubscriber(strmID string) (net.Subscriber, error) {
	n.S = &StubSubscriber{}
	return n.S, nil
}

func (n *StubNetwork) Connect(nodeID, nodeAddr string) error { return nil }
func (n *StubNetwork) SetupProtocol() error                  { return nil }
func (b *StubNetwork) SendTranscodeResult(nodeID string, strmID string, transcodeResult map[string]string) error {
	return nil
}

type StubBroadcaster struct {
	Data map[uint64][]byte
}

func (b *StubBroadcaster) Broadcast(seqNo uint64, data []byte) error {
	b.Data[seqNo] = data
	return nil
}
func (b *StubBroadcaster) Finish() error { return nil }

type StubSubscriber struct{}

func (s *StubSubscriber) Subscribe(ctx context.Context, f func(seqNo uint64, data []byte, eof bool)) error {
	glog.Infof("Calling StubSubscriber!!!")
	s1 := core.SignedSegment{Seg: stream.HLSSegment{SeqNo: 0, Name: "strmID_01.ts", Data: []byte("test data"), Duration: 8.001}}
	s2 := core.SignedSegment{Seg: stream.HLSSegment{SeqNo: 1, Name: "strmID_02.ts", Data: []byte("test data"), Duration: 8.001}}
	s3 := core.SignedSegment{Seg: stream.HLSSegment{SeqNo: 2, Name: "strmID_03.ts", Data: []byte("test data"), Duration: 8.001}}
	s4 := core.SignedSegment{Seg: stream.HLSSegment{SeqNo: 3, Name: "strmID_04.ts", Data: []byte("test data"), Duration: 8.001}}
	for i, s := range []core.SignedSegment{s1, s2, s3, s4} {
		var buf bytes.Buffer
		enc := gob.NewEncoder(&buf)
		err := enc.Encode(s)
		if err != nil {
			glog.Errorf("Error encoding segment to []byte: %v", err)
			continue
		}
		f(uint64(i), buf.Bytes(), false)
	}
	return nil
}
func (s *StubSubscriber) Unsubscribe() error { return nil }

type StubSegmenter struct{}

func (s *StubSegmenter) SegmentRTMPToHLS(ctx context.Context, rs stream.Stream, hs stream.Stream, segOptions segmenter.SegmenterOptions) error {
	glog.Infof("Calling StubSegmenter")
	hs.WriteHLSSegmentToStream(stream.HLSSegment{SeqNo: 0, Name: "seg0.ts"})
	hs.WriteHLSSegmentToStream(stream.HLSSegment{SeqNo: 1, Name: "seg1.ts"})
	hs.WriteHLSSegmentToStream(stream.HLSSegment{SeqNo: 2, Name: "seg2.ts"})
	return fmt.Errorf("EOF")
}

// Should publish RTMP stream, turn the RTMP stream into HLS, and broadcast the HLS stream.
func TestGotRTMPStreamHandler(t *testing.T) {
	s := setupServer()
	s.LivepeerNode.VideoNetwork = &StubNetwork{}
	s.RTMPSegmenter = &StubSegmenter{}
	handler := gotRTMPStreamHandler(s)

	url, _ := url.Parse("http://localhost/stream/test")
	strm := stream.NewVideoStream("strmID", stream.RTMP)

	//Stream already exists
	s.LivepeerNode.StreamDB.AddStream("strmID", stream.NewVideoStream("strmID", stream.RTMP))
	err := handler(url, strm)
	if err != ErrAlreadyExists {
		t.Errorf("Expecting publish error because stream already exists, but got: %v", err)
	}
	s.LivepeerNode.StreamDB.DeleteStream(core.StreamID("strmID"))

	//Try to handle test RTMP data
	handler(url, strm)

	hlsStrmID := s.broadcastRtmpToHLSMap["strmID"]
	if hlsStrmID == "" {
		t.Errorf("HLS stream ID should exist")
	}
	hlsStrm := s.LivepeerNode.StreamDB.GetStream(core.StreamID(hlsStrmID))
	if hlsStrm == nil {
		t.Errorf("HLS stream should exist")
	}

	//Wait for the video to be segmented and broadcasted
	sn, ok := s.LivepeerNode.VideoNetwork.(*StubNetwork)
	if !ok {
		t.Errorf("VideoNetwork not assigned correctly.")
	}
	start := time.Now()
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
	if seg0.Seg.Name != "seg0.ts" || seg1.Seg.Name != "seg1.ts" || seg2.Seg.Name != "seg2.ts" || seg3.Seg.EOF != true {
		t.Errorf("Wrong segments: %v, %v, %v, %v", seg0, seg1, seg2, seg3)
	}
}

func TestGetHLSPlaylistHandler(t *testing.T) {
	s := setupServer()
	handler := getHLSMediaPlaylistHandler(s)
	url, _ := url.Parse("http://localhost/stream/strmID.m3u8")

	pl, err := handler(url)
	if err != nil {
		t.Errorf("Error handling getHLSPlaylist: %v", err)
	}

	if pl.Segments[0].URI == "" {
		t.Errorf("playlist is empty")
	}

	buf := s.LivepeerNode.StreamDB.GetHLSBuffer("strmID")
	if buf == nil {
		t.Errorf("Cannot find the stream")
	}

	//Load 4 segments
	for i := 0; i < 4; i++ {
		ctx, _ := context.WithTimeout(context.Background(), time.Second)
		d, err := buf.WaitAndPopSegment(ctx, fmt.Sprintf("strmID_0%d.ts", i+1))
		if err != nil {
			t.Errorf("Error getting segment from buffer: %v", err)
		}
		if string(d) != "test data" {
			t.Errorf("Expecting 'test data', but got %v", d)
		}
	}
}

func TestParseSegname(t *testing.T) {
	u, _ := url.Parse("http://localhost/stream/1220c50f8bc4d2a807aace1e1376496a9d7f7c1408dec2512763c3ca16fe828f6631_01.ts")
	segName := parseSegName(u.Path)
	if segName != "1220c50f8bc4d2a807aace1e1376496a9d7f7c1408dec2512763c3ca16fe828f6631_01.ts" {
		t.Errorf("Expecting %v, but %v", "1220c50f8bc4d2a807aace1e1376496a9d7f7c1408dec2512763c3ca16fe828f6631_01.ts", segName)
	}
}
