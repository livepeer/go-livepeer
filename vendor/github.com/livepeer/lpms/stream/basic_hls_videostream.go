package stream

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/livepeer/m3u8"
)

const DefaultHLSStreamCap = uint(500)
const DefaultHLSStreamWin = uint(3)

// const DefaultMediaWinLen = uint(5)
const DefaultSegWaitTime = time.Second * 10
const SegWaitInterval = time.Second

var ErrAddHLSSegment = errors.New("ErrAddHLSSegment")

//BasicHLSVideoStream is a basic implementation of HLSVideoStream
type BasicHLSVideoStream struct {
	plCache    *m3u8.MediaPlaylist //StrmID -> MediaPlaylist
	segMap     map[string]*HLSSegment
	segNames   []string
	lock       sync.Locker
	strmID     string
	subscriber func(*HLSSegment, bool)
	winSize    uint
}

func NewBasicHLSVideoStream(strmID string, wSize uint) *BasicHLSVideoStream {
	pl, err := m3u8.NewMediaPlaylist(wSize, DefaultHLSStreamCap)
	if err != nil {
		return nil
	}

	return &BasicHLSVideoStream{
		plCache:  pl,
		segMap:   make(map[string]*HLSSegment),
		segNames: make([]string, 0),
		lock:     &sync.Mutex{},
		strmID:   strmID,
		winSize:  wSize,
	}
}

//SetSubscriber sets the callback function that will be called when a new hls segment is inserted
func (s *BasicHLSVideoStream) SetSubscriber(f func(seg *HLSSegment, eof bool)) {
	s.subscriber = f
}

//GetStreamID returns the streamID
func (s *BasicHLSVideoStream) GetStreamID() string { return s.strmID }

func (s *BasicHLSVideoStream) AppData() AppData { return nil }

//GetStreamFormat always returns HLS
func (s *BasicHLSVideoStream) GetStreamFormat() VideoFormat { return HLS }

//GetStreamPlaylist returns the media playlist represented by the streamID
func (s *BasicHLSVideoStream) GetStreamPlaylist() (*m3u8.MediaPlaylist, error) {
	if s.plCache.Count() < s.winSize {
		return nil, nil
	}

	return s.plCache, nil
}

//GetHLSSegment gets the HLS segment.  It blocks until something is found, or timeout happens.
func (s *BasicHLSVideoStream) GetHLSSegment(segName string) (*HLSSegment, error) {
	seg, ok := s.segMap[segName]
	if !ok {
		return nil, ErrNotFound
	}
	return seg, nil
}

//AddHLSSegment adds the hls segment to the right stream
func (s *BasicHLSVideoStream) AddHLSSegment(seg *HLSSegment) error {
	if _, ok := s.segMap[seg.Name]; ok {
		return nil //Already have the seg.
	}
	// glog.V(common.VERBOSE).Infof("Adding segment: %v", seg.Name)

	s.lock.Lock()
	defer s.lock.Unlock()

	//Add segment to media playlist and buffer
	s.plCache.AppendSegment(&m3u8.MediaSegment{SeqId: seg.SeqNo, Duration: seg.Duration, URI: seg.Name})
	s.segNames = append(s.segNames, seg.Name)
	s.segMap[seg.Name] = seg
	if s.plCache.Count() > s.winSize {
		s.plCache.Remove()
		toRemove := s.segNames[0]
		delete(s.segMap, toRemove)
		s.segNames = s.segNames[1:]
	}

	//Call subscriber
	if s.subscriber != nil {
		s.subscriber(seg, false)
	}

	return nil
}

func (s *BasicHLSVideoStream) End() {
	if s.subscriber != nil {
		s.subscriber(nil, true)
	}
}

func (s BasicHLSVideoStream) String() string {
	return fmt.Sprintf("StreamID: %v, Type: %v, len: %v", s.GetStreamID(), s.GetStreamFormat(), len(s.segMap))
}
