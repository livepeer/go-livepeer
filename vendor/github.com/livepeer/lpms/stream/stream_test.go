package stream

import (
	"context"
	"fmt"
	"runtime"
	"testing"

	"time"

	"github.com/ericxtang/m3u8"
)

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

	if buffer.mediaPlCache.SeqNo != 10 { //From the first HLSSegment
		t.Errorf("Should have gotten SeqNo 10, but got %v", buffer.mediaPlCache.SeqNo)
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
