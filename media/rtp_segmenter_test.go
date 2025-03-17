package media

import (
	"sync"
	"testing"

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

var (
	// Create mock tracks
	videoTrack = &mockTrackRemote{codecType: webrtc.MimeTypeH264}
	audioTrack = &mockTrackRemote{codecType: webrtc.MimeTypeOpus, channels: 2}
)

func TestRTPSegmenterQueueLimit(t *testing.T) {

	require := require.New(t)
	ssr := NewSwitchableSegmentReader()
	seg := NewRTPSegmenter([]RTPTrack{videoTrack, audioTrack}, ssr)
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
	seg := NewRTPSegmenter([]RTPTrack{videoTrack}, ssr)

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
	}

	// Test that writing to non-existent audio track returns an error
	err := seg.WriteAudio(audioTrack, 1500, [][]byte{[]byte{10, 11, 12}})
	require.Error(err)
	require.Contains(err.Error(), "no matching audio track found")
}

func TestRTPSegmenterAudioOnly(t *testing.T) {
	require := require.New(t)
	ssr := NewSwitchableSegmentReader()
	seg := NewRTPSegmenter([]RTPTrack{audioTrack}, ssr)

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
	}

	// Test that writing to non-existent video track returns an error
	err := seg.WriteVideo(videoTrack, 1500, 1500, [][]byte{[]byte{10, 11, 12}})
	require.Error(err)
	require.Contains(err.Error(), "no matching video track found")
}

func TestRTPSegmenterConcurrency(t *testing.T) {
	require := require.New(t)
	ssr := NewSwitchableSegmentReader()
	seg := NewRTPSegmenter([]RTPTrack{videoTrack, audioTrack}, ssr)

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
	require := require.New(t)
	ssr := NewSwitchableSegmentReader()
	seg := NewRTPSegmenter([]RTPTrack{videoTrack, audioTrack}, ssr)

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

	// Try to write a late audio packet - should be dropped
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
}
