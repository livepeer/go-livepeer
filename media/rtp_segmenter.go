package media

import (
	"errors"
	"sync"

	"github.com/bluenviron/mediacommon/v2/pkg/formats/mpegts"
	"github.com/pion/webrtc/v4"
)

// converts from rtp to segmented mpeg2ts

type RTPSegmenter struct {
	mu           sync.RWMutex
	mediaWriter  *MediaWriter
	tracks       []*trackWriter
	mpegtsWriter *mpegts.Writer
	ssr          *SwitchableSegmentReader
	audioQueue   []*audioPacket
	videoQueue   []*videoPacket
	hasAudio     bool
	hasVideo     bool
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
	rtpTrack    *webrtc.TrackRemote
	mpegtsTrack *mpegts.Track
	writeAudio  func(pts int64, data [][]byte) error
	writeVideo  func(pts, dts int64, data [][]byte) error
}

func NewRTPSegmenter(tracks []*webrtc.TrackRemote, ssr *SwitchableSegmentReader) *RTPSegmenter {
	s := &RTPSegmenter{
		ssr: ssr,
	}
	s.tracks = s.setupTracks(tracks)
	return s
}

func (s *RTPSegmenter) StartSegment(startTs int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Flush any pending packets < startTs
	s.flushQueues(startTs)

	// close any previous segment
	if s.mediaWriter != nil {
		s.mediaWriter.Close()
	}
	// re-create mpegts tracks; we dont want to reuse them
	newTracks := resetMpegtsTracks(s.tracks)
	writer := NewMediaWriter()
	s.mpegtsWriter = mpegts.NewWriter(writer, newTracks)
	s.ssr.Read(writer.MakeReader())
	s.mediaWriter = writer
}
func (s *RTPSegmenter) WriteVideo(source *webrtc.TrackRemote, pts, dts int64, au [][]byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, t := range s.tracks {
		if t.rtpTrack == source {
			s.videoQueue = append(s.videoQueue, &videoPacket{t, pts, dts, au})
			return s.interleaveAndWrite()
		}
	}
	return errors.New("no matching video track found")
}
func (s *RTPSegmenter) WriteAudio(source *webrtc.TrackRemote, pts int64, au [][]byte) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, t := range s.tracks {
		if t.rtpTrack == source {
			rescaledPts := multiplyAndDivide(pts, 90000, int64(source.Codec().ClockRate))
			s.audioQueue = append(s.audioQueue, &audioPacket{t, rescaledPts, au})
			return s.interleaveAndWrite()
		}
	}
	return errors.New("no matching audio track found")
}
func (s *RTPSegmenter) CloseSegment() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.mediaWriter != nil {
		s.mediaWriter.Close()
	}
}
func (s *RTPSegmenter) IsReady() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.mediaWriter != nil
}

func (s *RTPSegmenter) setupTracks(rtpTracks []*webrtc.TrackRemote) []*trackWriter {
	tracks := []*trackWriter{}
	for _, t := range rtpTracks {
		codec := t.Codec()
		cw := &trackWriter{
			rtpTrack: t,
		}
		switch codec.MimeType {
		case webrtc.MimeTypeH264:
			cw.writeVideo = func(pts, dts int64, au [][]byte) error {
				return s.mpegtsWriter.WriteH264(cw.mpegtsTrack, pts, dts, au)
			}
			s.hasVideo = true
		case webrtc.MimeTypeOpus:
			cw.writeAudio = func(pts int64, data [][]byte) error {
				return s.mpegtsWriter.WriteOpus(cw.mpegtsTrack, pts, data)
			}
			s.hasAudio = true
		}
		tracks = append(tracks, cw)
	}
	return tracks
}

func (s *RTPSegmenter) interleaveAndWrite() error {
	if !s.hasAudio || !s.hasVideo {
		// only have one or the other, nothing to interleave
		// so flush immediately
		s.flushQueues(0)
	}
	for len(s.audioQueue) > 0 && len(s.videoQueue) > 0 {
		if s.videoQueue[0].dts <= s.audioQueue[0].pts {
			vp := s.videoQueue[0]
			s.videoQueue = s.videoQueue[1:]
			if err := vp.track.writeVideo(vp.pts, vp.dts, vp.data); err != nil {
				return err
			}
		} else {
			ap := s.audioQueue[0]
			s.audioQueue = s.audioQueue[1:]
			if err := ap.track.writeAudio(ap.pts, ap.data); err != nil {
				return err
			}
		}
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
