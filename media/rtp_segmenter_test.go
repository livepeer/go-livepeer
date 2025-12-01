package media

import (
	"fmt"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/bluenviron/mediacommon/v2/pkg/formats/mpegts"
	"github.com/pion/webrtc/v4"
	"github.com/stretchr/testify/require"
)

type mockTrackRemote struct {
	webrtc.TrackRemote
	codecType string
	channels  uint16
}

func (m *mockTrackRemote) Codec() webrtc.RTPCodecParameters {
	return webrtc.RTPCodecParameters{
		RTPCodecCapability: webrtc.RTPCodecCapability{
			MimeType:  m.codecType,
			Channels:  m.channels,
			ClockRate: 90000, // usually 48khz but say 90 to make understanding tests easier
		},
	}
}

func (m *mockTrackRemote) Kind() webrtc.RTPCodecType {
	switch m.codecType {
	case webrtc.MimeTypeH264:
		return webrtc.RTPCodecTypeVideo
	case webrtc.MimeTypeOpus:
		return webrtc.RTPCodecTypeAudio
	default:
		return webrtc.RTPCodecTypeVideo
	}
}

func (m *mockTrackRemote) SSRC() webrtc.SSRC {
	return 0
}

var (
	// Create mock tracks
	newVideoTrack = func() SegmenterTrack {
		return NewSegmenterTrack(&mockTrackRemote{codecType: webrtc.MimeTypeH264})
	}
	newAudioTrack = func() SegmenterTrack {
		return NewSegmenterTrack(&mockTrackRemote{codecType: webrtc.MimeTypeOpus, channels: 2})
	}
)

// Add this stub to capture writes
type stubWriter struct {
	writeVideo func(pts, dts int64, au [][]byte) error
	writeAudio func(pts int64, au [][]byte) error
}

func (s *stubWriter) WriteH264(track *mpegts.Track, pts, dts int64, au [][]byte) error {
	return s.writeVideo(pts, dts, au)
}

func (s *stubWriter) WriteOpus(track *mpegts.Track, pts int64, au [][]byte) error {
	return s.writeAudio(pts, au)
}

func newStubTSWriter(w io.Writer, t []*mpegts.Track) MpegtsWriter {
	return &stubWriter{
		writeVideo: func(pts, dts int64, au [][]byte) error {
			w.Write(fmt.Appendf(nil, "V%d ", pts))
			return nil
		},
		writeAudio: func(pts int64, au [][]byte) error {
			w.Write(fmt.Appendf(nil, "A%d ", pts))
			return nil
		},
	}
}

func TestRTPSegmenterQueueLimit(t *testing.T) {

	t.Skip("inapplicable for now")

	require := require.New(t)
	ssr := NewSwitchableSegmentReader()
	videoTrack, audioTrack := newVideoTrack(), newAudioTrack()
	seg := NewRTPSegmenter([]SegmenterTrack{videoTrack, audioTrack}, ssr, 0)
	seg.StartSegment(0)

	// Override maxQueueSize for testing
	seg.maxQueueSize = 3

	// Test video queue limit
	for i := 0; i < seg.maxQueueSize+5; i++ {
		require.NoError(seg.WriteVideo(videoTrack, int64(i*1000), int64(i*1000), [][]byte{[]byte{1, 2, 3}}))
		if i <= seg.maxQueueSize {
			require.Equal(int64(0), seg.tsWatermark)
		} else {
			require.Equal(int64(i-seg.maxQueueSize)*1000, seg.tsWatermark)
		}
	}

	// Check that oldest packets were flushed
	// Queue should be at or below max size
	require.Equal(len(seg.videoQueue), seg.maxQueueSize)
	// Watermark should be updated to the timestamp of the last flushed packet
	require.Equal(int64(4000), seg.tsWatermark)

	// Test audio queue limit
	for i := 0; i < seg.maxQueueSize+5; i++ {
		require.NoError(seg.WriteAudio(audioTrack, int64(i*10000), [][]byte{[]byte{4, 5, 6}}))
		if i == 0 {
			// first audio write should be dropped because zero ts
			require.Equal(int64(4000), seg.tsWatermark)
		} else if i <= seg.maxQueueSize {
			// old video should be flushed out all at once
			require.Equal(int64(7000), seg.tsWatermark)
		} else {
			require.Equal(int64(i-seg.maxQueueSize)*10000, seg.tsWatermark)
		}
	}

	// Check that oldest packets were flushed
	// Queue should be at or below max size
	require.Equal(len(seg.audioQueue), seg.maxQueueSize)
	require.Empty(seg.videoQueue)
	// Watermark should be updated to the timestamp of the last flushed packet
	require.Equal(int64(40000), seg.tsWatermark)
}

func TestRTPSegmenterVideoOnly(t *testing.T) {
	require := require.New(t)
	ssr := NewSwitchableSegmentReader()
	videoTrack, audioTrack := newVideoTrack(), newAudioTrack()
	seg := NewRTPSegmenter([]SegmenterTrack{videoTrack}, ssr, 0)
	seg.mpegtsInit = newStubTSWriter
	var segment CloneableReader
	ssr.AddReader(func(reader CloneableReader) {
		segment = reader
	})

	// Verify we have video but no audio
	require.True(seg.hasVideo)
	require.False(seg.hasAudio)

	// Start a segment
	seg.StartSegment(0)

	// Write packets in a loop and verify state after each write
	for i := 0; i < 3; i++ {
		ts := int64((i + 1) * 1000)
		require.NoError(seg.WriteVideo(videoTrack, ts, ts, [][]byte{[]byte{byte(i + 1), byte(i + 2), byte(i + 3)}}))
		require.Equal(ts, seg.tsWatermark, "Watermark should match timestamp after packet %d", i)
		require.Empty(seg.videoQueue, "Video queue should be empty after packet %d", i)
		// Check output
		expect := fmt.Sprintf("V%d ", ts)
		out := make([]byte, len(expect))
		n, err := io.ReadAtLeast(segment, out, len(out))
		require.Nil(err)
		require.Equal(len(expect), n)
		require.Equal(expect, string(out))
	}

	// Test that writing to non-existent audio track returns an error
	err := seg.WriteAudio(audioTrack, 1500, [][]byte{[]byte{10, 11, 12}})
	require.Error(err)
	require.Contains(err.Error(), "no matching audio track found")
}

func TestRTPSegmenterAudioOnly(t *testing.T) {
	require := require.New(t)
	ssr := NewSwitchableSegmentReader()
	videoTrack, audioTrack := newVideoTrack(), newAudioTrack()
	seg := NewRTPSegmenter([]SegmenterTrack{audioTrack}, ssr, 0)
	seg.mpegtsInit = newStubTSWriter
	var segment CloneableReader
	ssr.AddReader(func(reader CloneableReader) {
		segment = reader
	})

	// Verify we have audio but no video
	require.True(seg.hasAudio)
	require.False(seg.hasVideo)

	// Start a segment
	seg.StartSegment(0)

	// Write packets in a loop and verify state after each write
	for i := 0; i < 3; i++ {
		ts := int64((i + 1) * 1000)
		require.NoError(seg.WriteAudio(audioTrack, ts, [][]byte{[]byte{byte(1), byte(2), byte(3)}}))
		require.Equal(ts, seg.tsWatermark, "Watermark should match timestamp after packet %d", i)
		require.Empty(seg.audioQueue, "Audio queue should be empty after packet %d", i)
		// Check output
		expect := fmt.Sprintf("A%d ", ts)
		out := make([]byte, len(expect))
		n, err := io.ReadAtLeast(segment, out, len(out))
		require.Nil(err)
		require.Equal(len(expect), n)
		require.Equal(expect, string(out))
	}

	// Test that writing to non-existent video track returns an error
	err := seg.WriteVideo(videoTrack, 1500, 1500, [][]byte{[]byte{10, 11, 12}})
	require.Error(err)
	require.Contains(err.Error(), "no matching video track found")
}

func TestRTPSegmenterConcurrency(t *testing.T) {
	require := require.New(t)
	ssr := NewSwitchableSegmentReader()
	videoTrack, audioTrack := newVideoTrack(), newAudioTrack()
	seg := NewRTPSegmenter([]SegmenterTrack{videoTrack, audioTrack}, ssr, 0)

	// Start a segment
	seg.StartSegment(0)

	const (
		videoPackets = 500
		audioPackets = 1000
	)

	var wg sync.WaitGroup
	wg.Add(2)

	// Write video packets concurrently
	go func() {
		defer wg.Done()
		for i := 0; i < videoPackets; i++ {
			ts := int64(i * 100)
			require.NoError(seg.WriteVideo(videoTrack, ts, ts, [][]byte{[]byte{byte(i % 255)}}))
		}
	}()

	// Write audio packets concurrently
	go func() {
		defer wg.Done()
		for i := 0; i < audioPackets; i++ {
			ts := int64(i * 50)
			require.NoError(seg.WriteAudio(audioTrack, ts, [][]byte{[]byte{byte(i % 255)}}))
		}
	}()

	// Wait for all goroutines to complete
	wg.Wait()

	// Verify final state
	seg.CloseSegment()
	totalPackets := len(seg.audioQueue) + len(seg.videoQueue)
	require.Equal(0, totalPackets, "Still had queued packets")

	// Watermark should be set to some reasonable value
	require.Greater(seg.tsWatermark, int64(0), "Watermark should be greater than 0")
}

func TestRTPSegmenterLatePacketDropping(t *testing.T) {

	t.Skip("inapplicable for now")

	require := require.New(t)
	ssr := NewSwitchableSegmentReader()
	videoTrack, audioTrack := newVideoTrack(), newAudioTrack()
	seg := NewRTPSegmenter([]SegmenterTrack{videoTrack, audioTrack}, ssr, 0)
	seg.mpegtsInit = newStubTSWriter
	var segment CloneableReader
	ssr.AddReader(func(reader CloneableReader) {
		segment = reader
	})

	// Start a segment
	seg.StartSegment(0)

	// Write some packets to establish a low watermark
	require.NoError(seg.WriteVideo(videoTrack, 5000, 5000, [][]byte{[]byte{1, 2, 3}}))

	require.NoError(seg.WriteAudio(audioTrack, 2000, [][]byte{[]byte{4, 5, 6}}))

	require.Equal(int64(2000), seg.tsWatermark)
	require.Len(seg.videoQueue, 1)
	require.Empty(seg.audioQueue)

	// Try to write a late audio packets - should be dropped
	require.NoError(seg.WriteAudio(audioTrack, 1000, [][]byte{[]byte{4, 5, 6}}))
	require.Equal(int64(2000), seg.tsWatermark)
	require.Len(seg.videoQueue, 1)
	require.Equal(int64(5000), seg.videoQueue[0].pts)
	require.Empty(seg.audioQueue)

	// Try to write a late video packet - should be dropped
	require.NoError(seg.WriteVideo(videoTrack, 1500, 1500, [][]byte{[]byte{1, 2, 3}}))
	require.Equal(int64(2000), seg.tsWatermark)
	require.Len(seg.videoQueue, 1)
	require.Equal(int64(5000), seg.videoQueue[0].pts)
	require.Empty(seg.audioQueue)

	// Try to write a late audio packet - should be ok
	require.NoError(seg.WriteAudio(audioTrack, 6000, [][]byte{[]byte{4, 5, 6}}))
	require.Equal(int64(5000), seg.tsWatermark)
	require.Empty(seg.videoQueue)
	require.Len(seg.audioQueue, 1)
	require.Equal(int64(6000), seg.audioQueue[0].pts)

	// Write a new packet with higher timestamp
	require.NoError(seg.WriteAudio(audioTrack, 6500, [][]byte{[]byte{4, 5, 6}}))
	require.Empty(seg.videoQueue)
	require.Len(seg.audioQueue, 2)
	require.NoError(seg.WriteAudio(audioTrack, 6600, [][]byte{[]byte{4, 5, 6}}))
	require.Empty(seg.videoQueue)
	require.Len(seg.audioQueue, 3)
	require.Equal(int64(5000), seg.tsWatermark) // still awaiting video before flush
	require.NoError(seg.WriteVideo(videoTrack, 7000, 7000, [][]byte{[]byte{1, 2, 3}}))
	require.Len(seg.audioQueue, 0)
	require.Len(seg.videoQueue, 1)
	// Verify low watermark is updated
	require.Equal(int64(7000), seg.videoQueue[0].pts)

	// Check output
	seg.CloseSegment()
	expected := "A2000 V5000 A6000 A6500 A6600 V7000 "
	out, err := io.ReadAll(segment)
	require.Nil(err)
	require.Equal(expected, string(out))
}

func TestRTPSegmenterMinSegmentDurationWallClock(t *testing.T) {
	require := require.New(t)

	ssr := NewSwitchableSegmentReader()
	// Use a short minSegDur so we don't slow tests too much
	minSegDur := 100 * time.Millisecond
	videoTrack := newVideoTrack()
	seg := NewRTPSegmenter([]SegmenterTrack{videoTrack}, ssr, minSegDur)

	// Initially no segment
	require.False(seg.IsReady(), "No active segment yet")

	// Start an initial segment
	seg.StartSegment(0)
	require.True(seg.IsReady(), "Segment should be started")
	require.Nil(seg.WriteVideo(videoTrack, 0, 0, [][]byte{{0}}))

	// Immediately check if we should start a new segment
	// Not enough wall time has passed
	shouldStart := seg.ShouldStartSegment(1, 1) // ~1 second in 1hz timescale
	require.False(shouldStart, "Should not start new segment (wall-clock < minDur)")

	// Wait less than the minSegDur, ensure we still don't start
	time.Sleep(minSegDur / 2)
	shouldStart = seg.ShouldStartSegment(2, 1) // ~2 seconds
	require.False(shouldStart, "Still under the wall-clock limit")

	// Wait enough time, then we can start a new segment
	time.Sleep(minSegDur / 2)
	shouldStart = seg.ShouldStartSegment(3, 1) // ~3 seconds PTS
	require.True(shouldStart, "Segment should have started")
}

func TestRTPSegmenterMinSegmentDurationPTS(t *testing.T) {
	require := require.New(t)
	ssr := NewSwitchableSegmentReader()

	// 1 second min
	minSegDur := 10 * time.Millisecond
	videoTrack := newVideoTrack()
	seg := NewRTPSegmenter([]SegmenterTrack{videoTrack}, ssr, minSegDur)

	// Start initial segment at pts=0
	seg.StartSegment(0)
	require.True(seg.IsReady(), "Segment should be active")
	require.Nil(seg.WriteVideo(videoTrack, 0, 0, [][]byte{{0}}))

	// Even if we wait in real clock, if PTS is still less than 1s, no new segment
	time.Sleep(minSegDur)
	require.False(seg.ShouldStartSegment(9, 1000)) // 9ms < 10ms minSegDur
	require.True(seg.ShouldStartSegment(10, 1000)) // 10ms == minSegDur

	seg.StartSegment(12)
	require.Nil(seg.WriteVideo(videoTrack, 12, 12, [][]byte{{12}}))
	time.Sleep(minSegDur)                                  // because we also check the wall clock
	require.False(seg.ShouldStartSegment(20, 1000), "PTS") // 20ms < 22ms (12ms start + 10ms  minSegDur)
	require.True(seg.ShouldStartSegment(23, 1000), "PTS")  // 23ms > 22ms
}

func TestRTPSegmenterMixedOrder(t *testing.T) {
	require := require.New(t)
	// Create a new segmenter using both a video and an audio track.
	ssr := NewSwitchableSegmentReader()
	videoTrack, audioTrack := newVideoTrack(), newAudioTrack()
	seg := NewRTPSegmenter([]SegmenterTrack{videoTrack, audioTrack}, ssr, 0)

	seg.mpegtsInit = newStubTSWriter

	var segments []CloneableReader
	ssr.AddReader(func(reader CloneableReader) {
		segments = append(segments, reader)
	})
	writeVideo := func(ts int64) error { return seg.WriteVideo(videoTrack, ts, ts, [][]byte{{byte(ts)}}) }
	writeAudio := func(ts int64) error { return seg.WriteAudio(audioTrack, ts, [][]byte{{byte(ts)}}) }

	// Set an initial segment at timestamp 0 and override its writer with our stub.
	seg.StartSegment(0)

	// Write packets in the desired order.
	require.NoError(writeVideo(0))
	require.NoError(writeAudio(1))
	require.NoError(writeVideo(3))
	require.NoError(writeVideo(4))

	// Start a new segment at timestamp 6.
	seg.StartSegment(6)

	// Write additional packets.
	require.NoError(writeVideo(6))
	// This audio packet (with timestamp 2) is lower than the current watermark and should be dropped.
	require.NoError(writeAudio(2))
	require.NoError(writeAudio(5))

	// Flush any remaining queued packets.
	seg.CloseSegment()

	// Read first and second segments. add separator.
	require.Len(segments, 2)
	out1, err := io.ReadAll(segments[0])
	require.Nil(err)
	out2, err := io.ReadAll(segments[1])
	require.Nil(err)
	out := string(out1) + "/ " + string(out2)

	// Check results.
	expected := "V0 A1 V3 V4 / V6 A2 A5 "
	require.Equal(expected, out)
}

func TestRTPSegmenterDropKeyframe(t *testing.T) {
	require := require.New(t)
	// Create a new segmenter using both a video and an audio track.
	ssr := NewSwitchableSegmentReader()
	videoTrack, audioTrack := newVideoTrack(), newAudioTrack()
	seg := NewRTPSegmenter([]SegmenterTrack{videoTrack, audioTrack}, ssr, 0)

	seg.mpegtsInit = newStubTSWriter
	seg.maxQueueSize = 3

	var segments []CloneableReader
	ssr.AddReader(func(reader CloneableReader) {
		segments = append(segments, reader)
	})
	writeVideo := func(ts int64) error { return seg.WriteVideo(videoTrack, ts, ts, [][]byte{{byte(ts)}}) }
	writeAudio := func(ts int64) error { return seg.WriteAudio(audioTrack, ts, [][]byte{{byte(ts)}}) }

	// Set an initial segment at timestamp 0 and override its writer with our stub.
	seg.StartSegment(0)

	// Write packets in the desired order.
	require.NoError(writeVideo(0))
	require.NoError(writeAudio(1))
	require.NoError(writeAudio(3))
	require.NoError(writeAudio(4))
	require.NoError(writeAudio(5))
	require.NoError(writeAudio(6))

	seg.StartSegment(2)
	require.NoError(writeVideo(2)) // dropped
	require.NoError(writeAudio(8))
	require.NoError(writeAudio(9))
	require.NoError(writeAudio(10))
	require.NoError(writeAudio(12))
	require.NoError(writeAudio(13))
	require.NoError(writeVideo(7)) // droppd
	require.NoError(writeVideo(11))

	// Flush any remaining queued packets.
	seg.CloseSegment()

	// Read first and second segments. add separator.
	require.Len(segments, 2)
	out1, err := io.ReadAll(segments[0])
	require.Nil(err)
	out2, err := io.ReadAll(segments[1])
	require.Nil(err)
	out := string(out1) + "/ " + string(out2)

	// Check results.
	expected := "V0 A1 A3 A4 A5 A6 / V2 A8 A9 A10 A12 A13 V7 V11 "
	require.Equal(expected, out)
}

func TestRTPSegmenterLastMpegtsTS(t *testing.T) {
	require := require.New(t)

	ssr := NewSwitchableSegmentReader()
	videoTrack, audioTrack := newVideoTrack(), newAudioTrack()
	seg := NewRTPSegmenter([]SegmenterTrack{videoTrack, audioTrack}, ssr, 0)
	seg.mpegtsInit = newStubTSWriter

	// Start a segment so writes are accepted
	seg.StartSegment(0)

	// Video: we expect LastMpegtsTS to be set to the DTS we pass in.
	vPTS := int64(9000)
	vDTS := int64(8000)
	require.NoError(seg.WriteVideo(videoTrack, vPTS, vDTS, [][]byte{{0x01}}))
	require.Equal(vDTS, videoTrack.LastMpegtsTS(), "video LastMpegtsTS should match DTS")

	// Audio: clock rate in the mock is 90kHz, so rescaled PTS == input PTS.
	aPTS := int64(4500)
	require.NoError(seg.WriteAudio(audioTrack, aPTS, [][]byte{{0x02}}))
	require.Equal(aPTS, audioTrack.LastMpegtsTS(), "audio LastMpegtsTS should match rescaled PTS")
}

func TestRTPSegmenterConcurrentLastMpegtsTS(t *testing.T) {
	require := require.New(t)

	ssr := NewSwitchableSegmentReader()
	videoTrack := newVideoTrack()
	seg := NewRTPSegmenter([]SegmenterTrack{videoTrack}, ssr, 0)
	seg.mpegtsInit = newStubTSWriter
	seg.StartSegment(0)

	const (
		readers    = 8
		iterations = 500
	)

	var wg sync.WaitGroup
	start := make(chan struct{})

	// Writers: call WriteVideo with increasing DTS
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-start
		for j := 0; j < iterations; j++ {
			ts := int64(j) * 3000 // simulate ~30fps
			require.NoError(seg.WriteVideo(videoTrack, ts, ts, [][]byte{{0x01}}))
		}
	}()

	// Readers: sample LastMpegtsTS
	for i := 0; i < readers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			var last int64
			for j := 0; j < iterations; j++ {
				v := videoTrack.LastMpegtsTS()
				require.GreaterOrEqual(v, last)
				last = v
			}
		}()
	}

	close(start)
	wg.Wait()
	require.Greater(videoTrack.LastMpegtsTS(), int64(0))
}
