package stream

import (
	"context"
	"errors"
	"fmt"
	"io"
	"runtime"
	"testing"

	"time"

	"github.com/ericxtang/m3u8"
	"github.com/nareix/joy4/av"
)

//Testing WriteRTMP errors
var ErrPacketRead = errors.New("packet read error")
var ErrStreams = errors.New("streams error")

type BadStreamsDemuxer struct{}

func (d BadStreamsDemuxer) Close() error                     { return nil }
func (d BadStreamsDemuxer) Streams() ([]av.CodecData, error) { return nil, ErrStreams }
func (d BadStreamsDemuxer) ReadPacket() (av.Packet, error)   { return av.Packet{Data: []byte{0, 0}}, nil }

type BadPacketsDemuxer struct{}

func (d BadPacketsDemuxer) Close() error                     { return nil }
func (d BadPacketsDemuxer) Streams() ([]av.CodecData, error) { return nil, nil }
func (d BadPacketsDemuxer) ReadPacket() (av.Packet, error) {
	return av.Packet{Data: []byte{0, 0}}, ErrPacketRead
}

type NoEOFDemuxer struct {
	c *Counter
}

type Counter struct {
	Count int
}

func (d NoEOFDemuxer) Close() error                     { return nil }
func (d NoEOFDemuxer) Streams() ([]av.CodecData, error) { return nil, nil }
func (d NoEOFDemuxer) ReadPacket() (av.Packet, error) {
	if d.c.Count == 10 {
		return av.Packet{}, nil
	}

	d.c.Count = d.c.Count + 1
	return av.Packet{Data: []byte{0}}, nil
}

func TestWriteRTMPErrors(t *testing.T) {
	// stream := Stream{Buffer: &StreamBuffer{}, StreamID: "test"}
	stream := NewVideoStream("test", RTMP)
	err := stream.WriteRTMPToStream(context.Background(), BadStreamsDemuxer{})
	if err != ErrStreams {
		t.Error("Expecting Streams Error, but got: ", err)
	}

	err = stream.WriteRTMPToStream(context.Background(), BadPacketsDemuxer{})
	if err != ErrPacketRead {
		t.Error("Expecting Packet Read Error, but got: ", err)
	}

	err = stream.WriteRTMPToStream(context.Background(), NoEOFDemuxer{c: &Counter{Count: 0}})
	if err != ErrDroppedRTMPStream {
		t.Error("Expecting RTMP Dropped Error, but got: ", err)
	}
}

//Testing WriteRTMP
type PacketsDemuxer struct {
	c *Counter
}

func (d PacketsDemuxer) Close() error                     { return nil }
func (d PacketsDemuxer) Streams() ([]av.CodecData, error) { return nil, nil }
func (d PacketsDemuxer) ReadPacket() (av.Packet, error) {
	if d.c.Count == 10 {
		return av.Packet{Data: []byte{0, 0}}, io.EOF
	}

	d.c.Count = d.c.Count + 1
	return av.Packet{Data: []byte{0, 0}}, nil
}

func TestWriteRTMP(t *testing.T) {
	// stream := Stream{Buffer: NewStreamBuffer(), StreamID: "test"}
	stream := NewVideoStream("test", RTMP)
	err := stream.WriteRTMPToStream(context.Background(), PacketsDemuxer{c: &Counter{Count: 0}})

	if err != io.EOF {
		t.Error("Expecting EOF, but got: ", err)
	}

	if stream.Len() != 12 { //10 packets, 1 header, 1 trailer
		t.Error("Expecting buffer length to be 12, but got: ", stream.Len())
	}

	// fmt.Println(stream.buffer.q.Get(12))

	//TODO: Test what happens when the buffer is full (should evict everything before the last keyframe)
}

var ErrBadHeader = errors.New("BadHeader")
var ErrBadPacket = errors.New("BadPacket")

type BadHeaderMuxer struct{}

func (d BadHeaderMuxer) Close() error                     { return nil }
func (d BadHeaderMuxer) WriteHeader([]av.CodecData) error { return ErrBadHeader }
func (d BadHeaderMuxer) WriteTrailer() error              { return nil }
func (d BadHeaderMuxer) WritePacket(av.Packet) error      { return nil }

type BadPacketMuxer struct{}

func (d BadPacketMuxer) Close() error                     { return nil }
func (d BadPacketMuxer) WriteHeader([]av.CodecData) error { return nil }
func (d BadPacketMuxer) WriteTrailer() error              { return nil }
func (d BadPacketMuxer) WritePacket(av.Packet) error      { return ErrBadPacket }

func TestReadRTMPError(t *testing.T) {
	stream := NewVideoStream("test", RTMP)
	err := stream.WriteRTMPToStream(context.Background(), PacketsDemuxer{c: &Counter{Count: 0}})
	if err != io.EOF {
		t.Error("Error setting up the test - while inserting packet.")
	}
	err = stream.ReadRTMPFromStream(context.Background(), BadHeaderMuxer{})

	if err != ErrBadHeader {
		t.Error("Expecting bad header error, but got ", err)
	}

	err = stream.ReadRTMPFromStream(context.Background(), BadPacketMuxer{})
	if err != ErrBadPacket {
		t.Error("Expecting bad packet error, but got ", err)
	}
}

//Test ReadRTMP
type PacketsMuxer struct{}

func (d PacketsMuxer) Close() error                     { return nil }
func (d PacketsMuxer) WriteHeader([]av.CodecData) error { return nil }
func (d PacketsMuxer) WriteTrailer() error              { return nil }
func (d PacketsMuxer) WritePacket(av.Packet) error      { return nil }

func TestReadRTMP(t *testing.T) {
	stream := NewVideoStream("test", RTMP)
	err := stream.WriteRTMPToStream(context.Background(), PacketsDemuxer{c: &Counter{Count: 0}})
	if err != io.EOF {
		t.Error("Error setting up the test - while inserting packet.")
	}
	readErr := stream.ReadRTMPFromStream(context.Background(), PacketsMuxer{})

	if readErr != io.EOF {
		t.Error("Expecting buffer to be empty, but got ", err)
	}

	if stream.Len() != 0 {
		t.Error("Expecting buffer length to be 0, but got ", stream.Len())
	}

	stream2 := NewVideoStream("test2", RTMP)
	stream2.RTMPTimeout = time.Millisecond * 50
	err2 := stream.WriteRTMPToStream(context.Background(), NoEOFDemuxer{c: &Counter{Count: 0}})
	if err2 != ErrDroppedRTMPStream {
		t.Error("Error setting up the test - while inserting packet.")
	}
	err2 = stream2.ReadRTMPFromStream(context.Background(), PacketsMuxer{})
	if err2 != ErrTimeout {
		t.Error("Expecting timeout, but got", err2)
	}
}

func TestWriteHLS(t *testing.T) {
	stream := NewVideoStream("test", HLS)
	err1 := stream.WriteHLSPlaylistToStream(m3u8.MediaPlaylist{})
	err2 := stream.WriteHLSSegmentToStream(HLSSegment{})
	if err1 != nil {
		t.Error("Shouldn't be error writing playlist, but got:", err1)
	}
	if err2 != nil {
		t.Error("Shouldn't be error writing segment, but got:", err2)
	}
	if stream.buffer.len() != 2 {
		t.Error("Should have 2 packet, but got:", stream.buffer.len())
	}
}

func TestReadHLSAsync(t *testing.T) {
	stream := NewVideoStream("test", HLS)
	stream.HLSTimeout = time.Millisecond * 100
	buffer := NewHLSBuffer(10, 10)
	grBefore := runtime.NumGoroutine()
	for i := 0; i < 9; i++ {
		stream.WriteHLSSegmentToStream(HLSSegment{SeqNo: uint64(i + 10), Name: "test" + string(i), Data: []byte{0}})
	}

	ec := make(chan error, 1)
	go func() { ec <- stream.ReadHLSFromStream(context.Background(), buffer) }()

	time.Sleep(time.Millisecond * 100)
	if buffer.sq.Count() != 9 {
		t.Errorf("Should have 9 packets in the buffer, but got: %v", buffer.sq.Count())
	}

	if buffer.plCache.SeqNo != 10 { //From the first HLSSegment
		t.Errorf("Should have gotten SeqNo 10, but got %v", buffer.plCache.SeqNo)
	}

	time.Sleep(time.Millisecond * 100)
	grAfter := runtime.NumGoroutine()
	if grBefore != grAfter {
		t.Errorf("Should have %v Go routines, but have %v", grBefore, grAfter)
	}
}

func TestReadHLSSync(t *testing.T) {
	stream := NewVideoStream("test", HLS)
	stream.HLSTimeout = time.Millisecond * 100

	for i := 0; i < 5; i++ {
		stream.WriteHLSSegmentToStream(HLSSegment{Name: fmt.Sprintf("test_%v.ts", i), Data: []byte{0}})
	}

	//Insert playlist in the middle - this shouldn't be discarded
	stream.WriteHLSPlaylistToStream(m3u8.MediaPlaylist{SeqNo: 100})

	for i := 5; i < 10; i++ {
		stream.WriteHLSSegmentToStream(HLSSegment{Name: fmt.Sprintf("test_%v.ts", i), Data: []byte{0}})
	}

	for i := 0; i < 10; i++ {
		seg, err := stream.ReadHLSSegment()
		if err != nil {
			t.Errorf("Error when reading HLS Segment: %v", err)
		}
		if seg.Name != fmt.Sprintf("test_%v.ts", i) {
			t.Errorf("Expecting %v, got %v", fmt.Sprintf("test_%v.ts", i), seg.Name)
		}
	}

	if stream.buffer.len() == 0 {
		t.Errorf("Should still have playlist")
	}

	_, err := stream.ReadHLSSegment()
	if err != ErrBufferEmpty {
		t.Errorf("Expeting ErrBufferEmpty, got %v", err)
	}

	if stream.buffer.len() == 0 {
		t.Errorf("Should still have playlist")
	}

	// item, err := stream.buffer.pop()
	// t.Errorf("stream buffer: %v, %v", item, err)
	pl, err := stream.ReadHLSPlaylist()

	if err != nil {
		t.Errorf("Error when reading playlist: %v", err)
	}

	if pl.SeqNo != 100 {
		t.Errorf("Expecting pl segNo to be 100 (inserted pl), but got %v", pl.SeqNo)
	}

	_, err = stream.ReadHLSPlaylist()
	if err != ErrTimeout {
		t.Errorf("Expeting ErrTimeout, got %v", err)
	}
}

func TestReadHLSCancel(t *testing.T) {
	stream := NewVideoStream("test", HLS)
	stream.HLSTimeout = time.Millisecond * 100
	buffer := NewHLSBuffer(5, 1000)
	grBefore := runtime.NumGoroutine()

	for i := 0; i < 9; i++ {
		stream.WriteHLSSegmentToStream(HLSSegment{SeqNo: uint64(i), Name: "test" + string(i), Data: []byte{0}})
	}

	ec := make(chan error, 1)
	ctx, cancel := context.WithCancel(context.Background())
	go func() { ec <- stream.ReadHLSFromStream(ctx, buffer) }()

	// time.Sleep(time.Millisecond * 100)
	cancel()

	err := <-ec

	if err != context.Canceled {
		t.Errorf("Expecting canceled, got %v", err)
	}

	time.Sleep(time.Millisecond * 100)
	grAfter := runtime.NumGoroutine()
	if grBefore != grAfter {
		t.Errorf("Should have %v Go routines, but have %v", grBefore, grAfter)
	}
}

//////////////////////////////////////////////////////////////////////
// type GoodHLSDemux struct{}

// func (d GoodHLSDemux) WaitAndPopPlaylist(ctx context.Context) (m3u8.MediaPlaylist, error) {
// 	return m3u8.MediaPlaylist{}, nil
// for i := 0; i < 2; i++ {
// 	pc <- m3u8.MediaPlaylist{}
// 	time.Sleep(time.Millisecond * 50)
// }

// select {
// case <-ctx.Done():
// 	return ctx.Err()
// }
// }
// func (d GoodHLSDemux) WaitAndGetSegment(ctx context.Context, name string) ([]byte, error) {
// 	return nil, nil
// }

// func (d GoodHLSDemux) PollPlaylist(ctx context.Context, pc chan m3u8.MediaPlaylist) error {
// 	for i := 0; i < 2; i++ {
// 		pc <- m3u8.MediaPlaylist{}
// 		time.Sleep(time.Millisecond * 50)
// 	}

// 	select {
// 	case <-ctx.Done():
// 		return ctx.Err()
// 	}
// }

// func (d GoodHLSDemux) PollSegment(ctx context.Context, sc chan m3u8.MediaSegment) error {
// 	for i := 0; i < 4; i++ {
// 		sc <- m3u8.MediaSegment{}
// 		time.Sleep(time.Millisecond * 50)
// 	}

// 	return io.EOF
// }

// func TestWriteHLS(t *testing.T) {
// 	stream := NewVideoStream("test")
// 	numGR := runtime.NumGoroutine()
// 	ctx, cancel := context.WithCancel(context.Background())
// 	err := stream.WriteHLSToStream(ctx, GoodHLSDemux{})
// 	cancel()

// 	if err != io.EOF {
// 		t.Error("Expecting EOF, but got:", err)
// 	}

// 	if stream.buffer.len() != 6 {
// 		t.Error("Expecting 6 packets in buffer, but got:", stream.buffer.len())
// 	}

// 	time.Sleep(time.Millisecond * 100)
// 	if numGR != runtime.NumGoroutine() {
// 		t.Errorf("NumGoroutine not equal. Before:%v, After:%v", numGR, runtime.NumGoroutine())
// 	}
// }

// type TimeoutHLSDemux struct{}

// func (d TimeoutHLSDemux) PollPlaylist(ctx context.Context, pc chan m3u8.MediaPlaylist) error {
// 	select {
// 	case <-ctx.Done():
// 		return ctx.Err()
// 	}
// }

// func (d TimeoutHLSDemux) PollSegment(ctx context.Context, sc chan m3u8.MediaSegment) error {
// 	select {
// 	case <-ctx.Done():
// 		return ctx.Err()
// 	}
// }

// //This test is more for documentation - this is how timeout works here.
// func TestWriteHLSTimeout(t *testing.T) {
// 	stream := NewVideoStream("test")
// 	numGR := runtime.NumGoroutine()
// 	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*100)
// 	defer cancel()
// 	err := stream.WriteHLSToStream(ctx, TimeoutHLSDemux{})

// 	if err != context.DeadlineExceeded {
// 		t.Error("Expecting EOF, but got:", err)
// 	}

// 	if stream.buffer.len() != 0 {
// 		t.Error("Expecting 0 packets in buffer, but got:", stream.buffer.len())
// 	}

// 	if numGR != runtime.NumGoroutine() {
// 		t.Errorf("NumGoroutine not equal. Before:%v, After:%v", numGR, runtime.NumGoroutine())
// 	}
// }

// //Test ReadRTMP Errors
// type FakeStreamBuffer struct {
// 	c *Counter
// }

// func (b *FakeStreamBuffer) Push(in interface{}) error { return nil }
// func (b *FakeStreamBuffer) Pop() (interface{}, error) {
// 	// fmt.Println("pop, count:", b.c.Count)
// 	switch b.c.Count {
// 	case 10:
// 		b.c.Count = b.c.Count - 1
// 		// i := &BufferItem{Type: RTMPHeader, Data: []av.CodecData{}}
// 		// h, _ := Serialize(i)
// 		// return h, nil
// 		return []av.CodecData{}, nil
// 	case 0:
// 		return nil, ErrBufferEmpty
// 	default:
// 		b.c.Count = b.c.Count - 1
// 		// i := &BufferItem{Type: RTMPPacket, Data: av.Packet{}}
// 		// p, _ := Serialize(i)
// 		// return p, nil
// 		return av.Packet{}, nil
// 	}
// }
