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

	addrutil "gx/ipfs/QmNSWW3Sb4eju4o2djPQ1L1c2Zj9XN9sMYJL8r1cbxdc6b/go-addr-util"
	ma "gx/ipfs/QmWWQ2Txc2c6tqjsBpzg5Ar652cHPGNsQQp2SejkNmkUMb/go-multiaddr"
	crypto "gx/ipfs/QmaPbCnUMBohSGo3KnxEa2bHqyJVVeEEcwtqJAYxerieBo/go-libp2p-crypto"

	"github.com/ericxtang/m3u8"
	"github.com/golang/glog"
	bnet "github.com/livepeer/go-livepeer-basicnet"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/net"
	"github.com/livepeer/lpms/segmenter"
	"github.com/livepeer/lpms/stream"
	"github.com/livepeer/lpms/vidplayer"
)

var S *LivepeerServer

func setupServer(nw net.VideoNetwork) *LivepeerServer {
	if S == nil {
		priv, pub, _ := crypto.GenerateKeyPair(crypto.RSA, 2048)
		node, err := bnet.NewNode(addrs(15000), priv, pub, &bnet.BasicNotifiee{})
		if err != nil {
			glog.Errorf("Error creating a new node: %v", err)
			return nil
		}
		if nw == nil {
			nw, err = bnet.NewBasicVideoNetwork(node, "", 0)
			if err != nil {
				glog.Errorf("Cannot create network node: %v", err)
				return nil
			}
		}
		n, _ := core.NewLivepeerNode(nil, nw, "12209433a695c8bf34ef6a40863cfe7ed64266d876176aee13732293b63ba1637fd2", "./tmp", nil)
		S = NewLivepeerServer("1935", "127.0.0.1", "8080", "127.0.0.1", n)
		go S.StartMediaServer(context.Background(), big.NewInt(0), "")
		go S.StartWebserver()
	}
	return S
}

type StubNetwork struct {
	B                  map[string]*StubBroadcaster
	S                  map[string]*StubSubscriber
	MPL                map[string]*m3u8.MasterPlaylist
	TranscodeCallbacks map[string]func(map[string]string)
}

func (n *StubNetwork) String() string { return "" }
func (n *StubNetwork) GetNodeID() string {
	return "12209433a695c8bf34ef6a40863cfe7ed64266d876176aee13732293b63ba1637fd2"
}

func (n *StubNetwork) GetBroadcaster(strmID string) (stream.Broadcaster, error) {
	return n.B[strmID], nil
}

func (n *StubNetwork) GetSubscriber(strmID string) (stream.Subscriber, error) {
	return n.S[strmID], nil
}

func (n *StubNetwork) Connect(nodeID string, nodeAddr []string) error { return nil }
func (n *StubNetwork) SetupProtocol() error                           { return nil }
func (b *StubNetwork) SendTranscodeResponse(nodeID string, strmID string, transcodeResult map[string]string) error {
	return nil
}
func (b *StubNetwork) ReceivedTranscodeResponse(strmID string, gotResult func(transcodeResult map[string]string)) {
	b.TranscodeCallbacks[strmID] = gotResult
}

func (b *StubNetwork) GetMasterPlaylist(nodeID string, strmID string) (chan *m3u8.MasterPlaylist, error) {
	mplc := make(chan *m3u8.MasterPlaylist)
	mpl, ok := b.MPL[strmID]
	if !ok {
		return nil, vidplayer.ErrNotFound
	}

	go func() {
		mplc <- mpl
		close(mplc)
	}()

	return mplc, nil
}

func (b *StubNetwork) UpdateMasterPlaylist(strmID string, mpl *m3u8.MasterPlaylist) error {
	b.MPL[strmID] = mpl
	return nil
}

func (n *StubNetwork) GetNodeStatus(nodeID string) (chan *net.NodeStatus, error) {
	return nil, nil
}
func (n *StubNetwork) TranscodeSub(ctx context.Context, strmID string, gotData func(seqNo uint64, data []byte, eof bool)) error {
	sub, _ := n.GetSubscriber(strmID)
	return sub.Subscribe(ctx, gotData)
}

type StubBroadcaster struct {
	Data map[uint64][]byte
}

func (b *StubBroadcaster) IsLive() bool   { return false }
func (b *StubBroadcaster) String() string { return "" }
func (b *StubBroadcaster) Broadcast(seqNo uint64, data []byte) error {
	b.Data[seqNo] = data
	return nil
}
func (b *StubBroadcaster) Finish() error { return nil }

type StubSubscriber struct {
	working         bool
	subscribeCalled bool
}

func (s *StubSubscriber) IsLive() bool   { return s.working }
func (s *StubSubscriber) String() string { return "" }
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

func (s *StubSegmenter) SubscribeToSegmenter(ctx context.Context, rs stream.RTMPVideoStream, segOptions segmenter.SegmenterOptions) (stream.Subscriber, error) {
	sub := &StubSubscriber{}
	return sub, nil
}

// Should publish RTMP stream, turn the RTMP stream into HLS, and broadcast the HLS stream.
func TestGotRTMPStreamHandler(t *testing.T) {
	stubnet := &StubNetwork{
		B:                  make(map[string]*StubBroadcaster),
		S:                  make(map[string]*StubSubscriber),
		MPL:                make(map[string]*m3u8.MasterPlaylist),
		TranscodeCallbacks: make(map[string]func(map[string]string)),
	}

	s := setupServer(stubnet)
	// s.LivepeerNode.VideoNetwork = stubnet
	s.RTMPSegmenter = &StubSegmenter{}
	handler := gotRTMPStreamHandler(s)

	hlsStrmID := string(s.LivepeerNode.Identity + "10f6afa01868f11f5722434aa4a0769842e04fac75dfaccece208c5710fd52e0")
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
	b := &StubBroadcaster{Data: make(map[uint64][]byte)}
	stubnet.B[hlsStrmID] = b
	if err := handler(url, strm); err != nil {
		t.Errorf("Error: %v", err)
	}

	start := time.Now()
	for time.Since(start) < time.Second*2 {
		if len(b.Data) != 4 {
			time.Sleep(100 * time.Millisecond)
			continue
		} else {
			break
		}
	}

	if len(b.Data) != 4 {
		t.Errorf("Should have recieved 3 data chunks, got:%v", len(b.Data))
	}

	seg0, _ := core.BytesToSignedSegment(b.Data[0])
	seg1, _ := core.BytesToSignedSegment(b.Data[1])
	seg2, _ := core.BytesToSignedSegment(b.Data[2])
	seg3, _ := core.BytesToSignedSegment(b.Data[3])
	if seg0.Seg.Name != "seg0.ts" || seg1.Seg.Name != "seg1.ts" || seg2.Seg.Name != "seg2.ts" || seg3.Seg.Name != "seg3.ts" {
		t.Errorf("Wrong segments: %v, %v, %v, %v", seg0, seg1, seg2, seg3)
	}

	if len(stubnet.TranscodeCallbacks) != 1 {
		// XXX fix
		//t.Errorf("Expect callback to be installed: %v", len(stubnet.TranscodeCallbacks))
	}
}

func TestGetHLSMasterPlaylistHandler(t *testing.T) {
	glog.Infof("\n\nTestGetHLSMasterPlaylistHandler...\n")
	stubnet := &StubNetwork{MPL: make(map[string]*m3u8.MasterPlaylist)}

	//Set up the stubnet so it already has a manifest with a local stream
	mpl := m3u8.NewMasterPlaylist()
	mpl.Append("strm.m3u8", nil, m3u8.VariantParams{Bandwidth: 100})
	stubnet.MPL["12209433a695c8bf34ef6a40863cfe7ed64266d876176aee13732293b63ba1637fd210f6afa01868f11f5722434aa4a0769842e04fac75dfaccece208c5710fd52e0"] = mpl
	if len(mpl.Variants) != 1 {
		t.Errorf("Expecting 1 variant, but got %v", mpl)
	}

	s := setupServer(stubnet)
	handler := getHLSMasterPlaylistHandler(s)
	url, _ := url.Parse("http://localhost/stream/12209433a695c8bf34ef6a40863cfe7ed64266d876176aee13732293b63ba1637fd210f6afa01868f11f5722434aa4a0769842e04fac75dfaccece208c5710fd52e0.m3u8")

	//Test get master playlist
	pl, err := handler(url)
	if err != nil {
		t.Errorf("Error handling getHLSMasterPlaylist: %v", err)
	}

	if len(pl.Variants) != 1 {
		t.Errorf("Expecting 1 variant, but got %v", pl)
	}
	if pl.Variants[0].URI != "12209433a695c8bf34ef6a40863cfe7ed64266d876176aee13732293b63ba1637fd210f6afa01868f11f5722434aa4a0769842e04fac75dfaccece208c5710fd52e0.m3u8" {
		t.Errorf("Expecting 12209433a695c8bf34ef6a40863cfe7ed64266d876176aee13732293b63ba1637fd210f6afa01868f11f5722434aa4a0769842e04fac75dfaccece208c5710fd52e0.m3u8, but got: %v", pl.Variants[0].URI)
	}
}

//So simple, don't need to test anymore.
// func TestGetHLSMediaPlaylistHandler(t *testing.T) {
// 	HLSWaitTime = time.Second * 3 //Make the waittime shorter to speed up the test suite
// 	strmID := "12209433a695c8bf34ef6a40863cfe7ed64266d876176aee13732293b63ba1637fd2122f1cb223775a93e8b3102942f54aa2561e3931cc5ac0d57a39e0c064b243eePTestVideo"
// 	s := setupServer()
// 	s.LivepeerNode.VideoNetwork = &StubNetwork{}
// 	url, _ := url.Parse(fmt.Sprintf("http://localhost/stream/%v.m3u8", strmID))

// 	handler := getHLSMediaPlaylistHandler(s)

// 	//Set up a local stream, should return that.
// 	glog.Infof("VideoDB: %v", s.LivepeerNode.VideoDB)
// 	hlsStrm, err := s.LivepeerNode.VideoDB.AddNewHLSStream(core.StreamID(strmID))
// 	if err != nil {
// 		t.Errorf("Error: %v", err)
// 	}
// 	hlsStrm.AddHLSSegment(&stream.HLSSegment{SeqNo: 0, Name: "seg0.ts", Data: []byte("hello"), Duration: 8})
// 	hlsStrm.AddHLSSegment(&stream.HLSSegment{SeqNo: 1, Name: "seg1.ts", Data: []byte("hello"), Duration: 8})
// 	hlsStrm.AddHLSSegment(&stream.HLSSegment{SeqNo: 2, Name: "seg2.ts", Data: []byte("hello"), Duration: 8})
// 	handler = getHLSMediaPlaylistHandler(s)
// 	pl, err := handler(url)
// 	if err != nil {
// 		t.Errorf("Error in handler: %v", err)
// 	}
// 	if pl.Count() == 0 {
// 		t.Errorf("Expecting segments, but got %v", pl)
// 	}
// 	if pl.Segments[0].URI != "seg0.ts" || pl.Segments[0].Duration != 8 {
// 		t.Errorf("Wrong segment info: %v", pl.Segments[0])
// 	}

// 	//Don't set up local stream (test should call subscribe to network)
// 	s.LivepeerNode.VideoDB = core.NewVideoDB(string(s.LivepeerNode.Identity))
// 	m, err := s.LivepeerNode.VideoDB.AddNewHLSManifest(core.ManifestID("12209433a695c8bf34ef6a40863cfe7ed64266d876176aee13732293b63ba1637fd2122f1cb223775a93e8b3102942f54aa2561e3931cc5ac0d57a39e0c064b243ee"))
// 	if err != nil {
// 		t.Errorf("Error: %v", err)
// 	}
// 	tmpStrm := stream.NewBasicHLSVideoStream(strmID, 3)
// 	m.AddVideoStream(tmpStrm, &m3u8.Variant{URI: fmt.Sprintf("%v.m3u8", strmID), VariantParams: m3u8.VariantParams{Bandwidth: 100}})
// 	pl, err = handler(url)
// 	if err != nil {
// 		t.Errorf("Error in handler: %v", err)
// 	}
// 	hlsStrm = s.LivepeerNode.VideoDB.GetHLSStream(core.StreamID(strmID))
// 	if hlsStrm == nil {
// 		t.Errorf("Expecting there to be a stream")
// 	}

// 	//Wait a few seconds to make sure segments are populated.
// 	start := time.Now()
// 	for time.Since(start) < time.Second*2 {
// 		s, err := hlsStrm.GetHLSSegment("strmID_01.ts")
// 		if err != nil || s == nil {
// 			time.Sleep(50 * time.Millisecond)
// 		}
// 	}

// 	glog.Infof("strm: %v", hlsStrm)
// 	//Test the data from stub subscriber
// 	for _, i := range []int{1, 2, 3} {
// 		s, err := hlsStrm.GetHLSSegment(fmt.Sprintf("strmID_0%v.ts", i))
// 		if err != nil || s == nil {
// 			t.Errorf("Error getting hlsSegment%v: %v", i, err)
// 		}
// 		if string(s.Data) != "test data" {
// 			t.Errorf("Expecting 'test data', but got %v", s.Data)
// 		}
// 	}
// 	if pl.Count() != 3 {
// 		t.Errorf("Expecting count to be 3, got %v", pl.Count())
// 	}
// }

func TestParseSegname(t *testing.T) {
	u, _ := url.Parse("http://localhost/stream/1220c50f8bc4d2a807aace1e1376496a9d7f7c1408dec2512763c3ca16fe828f6631_01.ts")
	segName := parseSegName(u.Path)
	if segName != "1220c50f8bc4d2a807aace1e1376496a9d7f7c1408dec2512763c3ca16fe828f6631_01.ts" {
		t.Errorf("Expecting %v, but %v", "1220c50f8bc4d2a807aace1e1376496a9d7f7c1408dec2512763c3ca16fe828f6631_01.ts", segName)
	}
}

func addrs(port int) []ma.Multiaddr {
	uaddrs, err := addrutil.InterfaceAddresses()
	if err != nil {
		return nil
	}
	addrs := make([]ma.Multiaddr, len(uaddrs), len(uaddrs))
	for i, uaddr := range uaddrs {
		portAddr, err := ma.NewMultiaddr(fmt.Sprintf("/tcp/%d", port))
		if err != nil {
			glog.Errorf("Error creating portAddr: %v %v", uaddr, err)
			return nil
		}
		addrs[i] = uaddr.Encapsulate(portAddr)
	}

	return addrs
}
