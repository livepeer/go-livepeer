package media

import (
	"errors"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bluenviron/mediacommon/v2/pkg/formats/mpegts"
	"github.com/pion/webrtc/v4"
)

// converts from rtp to segmented mpeg2ts

type RTPTrack interface {
	Codec() webrtc.RTPCodecParameters
	Kind() webrtc.RTPCodecType
	SSRC() webrtc.SSRC
}

type SegmenterTrack interface {
	RTPTrack
	LastMpegtsTS() int64
	SetLastMpegtsTS(ts int64)
}

type segmenterTrack struct {
	track  RTPTrack
	lastTS int64
}

func NewSegmenterTrack(tr RTPTrack) SegmenterTrack {
	return &segmenterTrack{track: tr}
}

func (t *segmenterTrack) Codec() webrtc.RTPCodecParameters {
	return t.track.Codec()
}

func (t *segmenterTrack) Kind() webrtc.RTPCodecType {
	return t.track.Kind()
}

func (t *segmenterTrack) SSRC() webrtc.SSRC {
	return t.track.SSRC()
}

func (t *segmenterTrack) LastMpegtsTS() int64 {
	return atomic.LoadInt64(&t.lastTS)
}

func (t *segmenterTrack) SetLastMpegtsTS(ts int64) {
	atomic.StoreInt64(&t.lastTS, ts)
}

type MpegtsWriter interface {
	WriteH264(*mpegts.Track, int64, int64, [][]byte) error
	WriteOpus(*mpegts.Track, int64, [][]byte) error
}

type RTPSegmenter struct {
	mu           sync.RWMutex
	mpegtsInit   func(bw io.Writer, tracks []*mpegts.Track) MpegtsWriter
	mediaWriter  *MediaWriter
	tracks       []*trackWriter
	mpegtsWriter MpegtsWriter
	ssr          *SwitchableSegmentReader
	audioQueue   []*audioPacket
	videoQueue   []*videoPacket
	hasAudio     bool
	hasVideo     bool
	maxQueueSize int   // Maximum number of packets to buffer per queue
	tsWatermark  int64 // Timestamp of the last written packet

	started      bool          // whether the first segment has been started
	nextStartTs  int64         // timestamp the next segment should start at
	nextStartSet bool          // wheher nextStartTs has been (re-)set
	segStartTs   int64         // timestamp of the current segment
	segStartTime time.Time     // wall clock of when the current segment started
	minSegDur    time.Duration // minimum segment duration
}

type audioPacket struct {
	track *trackWriter
	pts   int64
	data  [][]byte
}

type videoPacket struct {
	track    *trackWriter
	pts, dts int64
	data     [][]byte
}

type trackWriter struct {
	rtpTrack    SegmenterTrack
	mpegtsTrack *mpegts.Track
	writeAudio  func(pts int64, data [][]byte) error
	writeVideo  func(pts, dts int64, data [][]byte) error
}

func NewRTPSegmenter(tracks []SegmenterTrack, ssr *SwitchableSegmentReader, segDur time.Duration) *RTPSegmenter {
	s := &RTPSegmenter{
		ssr:          ssr,
		maxQueueSize: 64,
		minSegDur:    segDur,
		mpegtsInit:   func(w io.Writer, t []*mpegts.Track) MpegtsWriter { return mpegts.NewWriter(w, t) },
	}
	s.tracks = s.setupTracks(tracks)
	return s
}

func (s *RTPSegmenter) StartSegment(startTs int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.nextStartTs = startTs
	s.started = true
	s.nextStartSet = true
}

func (s *RTPSegmenter) resetSegment() {
	// Expects lock to already be held

	// close any previous segment
	if s.mediaWriter != nil {
		s.mediaWriter.Close()
	}
	// re-create mpegts tracks; we dont want to reuse them
	newTracks := resetMpegtsTracks(s.tracks)
	writer := NewMediaWriter()
	s.mpegtsWriter = s.mpegtsInit(writer, newTracks)
	s.ssr.Read(writer.MakeReader())
	s.mediaWriter = writer
	s.segStartTs = s.nextStartTs
	s.segStartTime = time.Now()
	s.nextStartSet = false
}

func (s *RTPSegmenter) ShouldStartSegment(pts int64, tb uint32) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.segStartTime.IsZero() {
		return true
	}
	// Enforce minimum real (wall-clock) time
	if time.Since(s.segStartTime) < s.minSegDur {
		return false
	}
	// Enforce minimum PTS time
	needed := int64(s.minSegDur.Seconds() * float64(tb))
	if (pts - s.segStartTs) < needed {
		return false
	}
	return true
}

func (s *RTPSegmenter) WriteVideo(source SegmenterTrack, pts, dts int64, au [][]byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, t := range s.tracks {
		if t.rtpTrack == source {
			// Check if packet is too old (below low watermark)
			if s.tsWatermark > 0 && dts < s.tsWatermark {
				// Packet is too old, discard it
				// TODO increment some metric for this connection?
				//return nil
			}

			// Add new packet
			s.videoQueue = append(s.videoQueue, &videoPacket{t, pts, dts, au})

			source.SetLastMpegtsTS(dts)

			return s.interleaveAndWrite()
		}
	}
	return errors.New("no matching video track found")
}
func (s *RTPSegmenter) WriteAudio(source SegmenterTrack, pts int64, au [][]byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, t := range s.tracks {
		if t.rtpTrack == source {
			rescaledPts := multiplyAndDivide(pts, 90000, int64(source.Codec().ClockRate))

			// Check if packet is too old (below low watermark)
			if s.tsWatermark > 0 && rescaledPts < s.tsWatermark {
				// Packet is too old, discard it
				// TODO increment some metric for this connection?
				//return nil
			}

			// Add new packet
			s.audioQueue = append(s.audioQueue, &audioPacket{t, rescaledPts, au})

			source.SetLastMpegtsTS(rescaledPts)

			return s.interleaveAndWrite()
		}
	}
	return errors.New("no matching audio track found")
}
func (s *RTPSegmenter) CloseSegment() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.flushQueues(0)
	if s.mediaWriter != nil {
		s.mediaWriter.Close()
	}
}
func (s *RTPSegmenter) IsReady() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.started
}

func (s *RTPSegmenter) setupTracks(rtpTracks []SegmenterTrack) []*trackWriter {
	tracks := []*trackWriter{}
	for _, t := range rtpTracks {
		codec := t.Codec()
		cw := &trackWriter{
			rtpTrack: t,
		}
		switch codec.MimeType {
		case webrtc.MimeTypeH264:
			cw.writeVideo = func(pts, dts int64, au [][]byte) error {
				if s.nextStartSet && dts >= s.nextStartTs {
					s.resetSegment()
				}
				return s.mpegtsWriter.WriteH264(cw.mpegtsTrack, pts, dts, au)
			}
			s.hasVideo = true
		case webrtc.MimeTypeOpus:
			cw.writeAudio = func(pts int64, data [][]byte) error {
				if s.nextStartSet && pts >= s.nextStartTs {
					s.resetSegment()
				}
				return s.mpegtsWriter.WriteOpus(cw.mpegtsTrack, pts, data)
			}
			s.hasAudio = true
		}
		tracks = append(tracks, cw)
	}
	return tracks
}

func (s *RTPSegmenter) interleaveAndWrite() error {
	s.flushQueues(0)
	if !s.hasAudio || !s.hasVideo {
		// only have one or the other, nothing to interleave
		// so flush immediately
		s.flushQueues(0)
	}

	for len(s.audioQueue) > 0 && len(s.videoQueue) > 0 {
		var timestamp int64
		if s.videoQueue[0].dts <= s.audioQueue[0].pts {
			vp := s.videoQueue[0]
			s.videoQueue = s.videoQueue[1:]
			timestamp = vp.dts
			if err := vp.track.writeVideo(vp.pts, vp.dts, vp.data); err != nil {
				return err
			}
		} else {
			ap := s.audioQueue[0]
			s.audioQueue = s.audioQueue[1:]
			timestamp = ap.pts
			if err := ap.track.writeAudio(ap.pts, ap.data); err != nil {
				return err
			}
		}

		// Update low watermark with the timestamp of the packet we just wrote
		s.tsWatermark = timestamp
	}

	// Check queue sizes and flush oldest packets if needed
	for len(s.videoQueue) > s.maxQueueSize {
		// Flush out the oldest packet
		vp := s.videoQueue[0]
		s.videoQueue = s.videoQueue[1:]
		if err := vp.track.writeVideo(vp.pts, vp.dts, vp.data); err != nil {
			return err
		}
		// Update watermark
		s.tsWatermark = vp.dts
	}

	for len(s.audioQueue) > s.maxQueueSize {
		// Flush out the oldest packet
		ap := s.audioQueue[0]
		s.audioQueue = s.audioQueue[1:]
		if err := ap.track.writeAudio(ap.pts, ap.data); err != nil {
			return err
		}
		// Update watermark
		s.tsWatermark = ap.pts
	}

	return nil
}

func (s *RTPSegmenter) flushQueues(stopTs int64) error {
	// NB interleaving should not be necessary at this point
	// only one queue should have any data
	for len(s.videoQueue) > 0 {
		vp := s.videoQueue[0]
		if stopTs > 0 && vp.dts >= stopTs {
			break
		}
		s.videoQueue = s.videoQueue[1:]
		if err := vp.track.writeVideo(vp.pts, vp.dts, vp.data); err != nil {
			return err
		}
		// Update low watermark
		s.tsWatermark = vp.dts
	}
	for len(s.audioQueue) > 0 {
		ap := s.audioQueue[0]
		if stopTs > 0 && ap.pts >= stopTs {
			break
		}
		s.audioQueue = s.audioQueue[1:]
		if err := ap.track.writeAudio(ap.pts, ap.data); err != nil {
			return err
		}
		// Update low watermark
		s.tsWatermark = ap.pts
	}
	return nil
}

func resetMpegtsTracks(tracks []*trackWriter) []*mpegts.Track {
	newTracks := []*mpegts.Track{}
	for _, t := range tracks {
		codec := t.rtpTrack.Codec()
		switch codec.MimeType {
		case webrtc.MimeTypeH264:
			newTrack := &mpegts.Track{Codec: &mpegts.CodecH264{}}
			t.mpegtsTrack = newTrack
			newTracks = append(newTracks, newTrack)
		case webrtc.MimeTypeOpus:
			newTrack := &mpegts.Track{Codec: &mpegts.CodecOpus{ChannelCount: int(codec.Channels)}}
			t.mpegtsTrack = newTrack
			newTracks = append(newTracks, newTrack)
		}
	}
	return newTracks
}

func multiplyAndDivide(v, m, d int64) int64 {
	secs := v / d
	dec := v % d
	return (secs*m + dec*m/d)
}
