package mediaserver

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"io"
	"net/url"
	"os"
	"testing"

	"time"

	"github.com/golang/glog"
	crypto "github.com/libp2p/go-libp2p-crypto"
	"github.com/livepeer/libp2p-livepeer/core"
	"github.com/livepeer/libp2p-livepeer/net"
	"github.com/livepeer/lpms/stream"
	"github.com/nareix/joy4/av/avutil"
	"github.com/nareix/joy4/format"
)

var S *LivepeerMediaServer

func setupServer() *LivepeerMediaServer {
	if S == nil {
		priv1, pub1, _ := crypto.GenerateKeyPair(crypto.RSA, 2048)
		n, _ := core.NewLivepeerNode(15000, priv1, pub1)
		S = NewLivepeerMediaServer("1935", "8080", "", n)
		S.StartLPMS(context.Background())
	}
	return S
}

func TestStartMediaServer(t *testing.T) {
	s := setupServer()
	time.Sleep(time.Millisecond * 100)

	if s.hlsSubTimer == nil {
		t.Errorf("hlsSubTimer should be initiated")
	}

	if s.hlsWorkerRunning == false {
		t.Errorf("Should have kicked off hlsUnsubscribeWorker go routine")
	}
}

type StubNetwork struct {
	B *StubBroadcaster
	S *StubSubscriber
}

func (n *StubNetwork) NewBroadcaster(strmID string) net.Broadcaster {
	n.B = &StubBroadcaster{Data: make(map[uint64][]byte)}
	return n.B
}

func (n *StubNetwork) GetBroadcaster(strmID string) net.Broadcaster {
	n.B = &StubBroadcaster{Data: make(map[uint64][]byte)}
	return n.B
}

func (n *StubNetwork) NewSubscriber(strmID string) net.Subscriber {
	n.S = &StubSubscriber{}
	return n.S
}

func (n *StubNetwork) GetSubscriber(strmID string) net.Subscriber {
	n.S = &StubSubscriber{}
	return n.S
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

func (s *StubSubscriber) Subscribe(ctx context.Context, f func(seqNo uint64, data []byte)) error {
	glog.Infof("Calling StubSubscriber!!!")
	s1 := stream.HLSSegment{SeqNo: 0, Name: "strmID_01.ts", Data: []byte("test data"), Duration: 8.001}
	s2 := stream.HLSSegment{SeqNo: 1, Name: "strmID_02.ts", Data: []byte("test data"), Duration: 8.001}
	s3 := stream.HLSSegment{SeqNo: 2, Name: "strmID_03.ts", Data: []byte("test data"), Duration: 8.001}
	s4 := stream.HLSSegment{SeqNo: 3, Name: "strmID_04.ts", Data: []byte("test data"), Duration: 8.001}
	for i, s := range []stream.HLSSegment{s1, s2, s3, s4} {
		var buf bytes.Buffer
		enc := gob.NewEncoder(&buf)
		err := enc.Encode(s)
		if err != nil {
			glog.Errorf("Error encoding segment to []byte: %v", err)
			continue
		}
		f(uint64(i), buf.Bytes())
	}
	return nil
}
func (s *StubSubscriber) Unsubscribe() error { return nil }

// Should publish RTMP stream, turn the RTMP stream into HLS, and broadcast the HLS stream.
func TestGotRTMPStreamHandler(t *testing.T) {
	s := setupServer()
	s.LivepeerNode.VideoNetwork = &StubNetwork{}
	handler := s.makeGotRTMPStreamHandler()

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
	wd, err := os.Getwd()
	if err != nil {
		t.Errorf("Cannot get working dir: %v", err)
	}

	//Send test video
	format.RegisterAll()
	file, err := avutil.Open(wd + "/test.flv")
	if err != nil {
		t.Errorf("Error opening file: %v", err)
	}
	header, _ := file.Streams()
	strm.WriteRTMPHeader(header)
	for {
		pkt, err := file.ReadPacket()
		if err == io.EOF {
			strm.WriteRTMPTrailer()
			break
		}
		strm.WriteRTMPPacket(pkt)
	}

	hlsStrmID := s.broadcastRtmpToHLSMap["strmID"]
	if hlsStrmID == "" {
		t.Errorf("HLS stream ID should exist")
	}
	hlsStrm := s.LivepeerNode.StreamDB.GetStream(core.StreamID(hlsStrmID))
	if hlsStrm == nil {
		t.Errorf("HLS stream should exist")
	}

	//Wait for the video to be segmented and broadcasted
	sn, _ := s.LivepeerNode.VideoNetwork.(*StubNetwork)
	start := time.Now()
	for time.Since(start) < time.Second*5 {
		if len(sn.B.Data) < 2 {
			time.Sleep(100 * time.Millisecond)
		} else {
			break
		}
	}

	if len(sn.B.Data) != 2 {
		t.Errorf("Should have recieved 2 data chunks, got:%v", len(sn.B.Data))
	}
}

func TestGetHLSPlaylistHandler(t *testing.T) {
	s := setupServer()
	handler := s.makeGetHLSMediaPlaylistHandler()
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
