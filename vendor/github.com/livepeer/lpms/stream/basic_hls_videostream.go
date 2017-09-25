package stream

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/ericxtang/m3u8"
	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/common"
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
	sqMap      map[string]*HLSSegment
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
		plCache: pl,
		// variant: variant,
		sqMap:   make(map[string]*HLSSegment),
		lock:    &sync.Mutex{},
		strmID:  strmID,
		winSize: wSize,
	}
}

//SetSubscriber sets the callback function that will be called when a new hls segment is inserted
func (s *BasicHLSVideoStream) SetSubscriber(f func(seg *HLSSegment, eof bool)) {
	s.subscriber = f
}

//GetStreamID returns the streamID
func (s *BasicHLSVideoStream) GetStreamID() string { return s.strmID }

//GetStreamFormat always returns HLS
func (s *BasicHLSVideoStream) GetStreamFormat() VideoFormat { return HLS }

//GetStreamPlaylist returns the media playlist represented by the streamID
func (s *BasicHLSVideoStream) GetStreamPlaylist() (*m3u8.MediaPlaylist, error) {
	if s.plCache.Count() < s.winSize {
		return nil, nil
	}

	return s.plCache, nil
}

// func (s *BasicHLSVideoStream) GetStreamVariant() *m3u8.Variant {
// 	return s.variant
// }

//GetHLSSegment gets the HLS segment.  It blocks until something is found, or timeout happens.
func (s *BasicHLSVideoStream) GetHLSSegment(segName string) (*HLSSegment, error) {
	seg, ok := s.sqMap[segName]
	if !ok {
		return nil, ErrNotFound
	}
	return seg, nil
}

//AddHLSSegment adds the hls segment to the right stream
func (s *BasicHLSVideoStream) AddHLSSegment(seg *HLSSegment) error {
	if _, ok := s.sqMap[seg.Name]; ok {
		return nil //Already have the seg.
	}
	glog.V(common.VERBOSE).Infof("Adding segment: %v", seg.Name)

	s.lock.Lock()
	defer s.lock.Unlock()

	//Add segment to media playlist
	s.plCache.AppendSegment(&m3u8.MediaSegment{SeqId: seg.SeqNo, Duration: seg.Duration, URI: seg.Name})
	if s.plCache.Count() > s.winSize {
		s.plCache.Remove()
	}

	//Add to buffer
	s.sqMap[seg.Name] = seg

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
	return fmt.Sprintf("StreamID: %v, Type: %v, len: %v", s.GetStreamID(), s.GetStreamFormat(), len(s.sqMap))
}

// //AddVariant adds a new variant playlist (and therefore, a new HLS video stream) to the master playlist.
// func (s *BasicHLSVideoStream) AddVariant(strmID string, variant *m3u8.Variant) error {
// 	if variant == nil {
// 		glog.Errorf("Cannot add nil variant")
// 		return ErrAddVariant
// 	}

// 	_, ok := s.variantMediaPlCache[strmID]
// 	if ok {
// 		glog.Errorf("Variant %v already exists", strmID)
// 		return ErrAddVariant
// 	}

// 	for _, v := range s.masterPlCache.Variants {
// 		if v.Bandwidth == variant.Bandwidth && v.Resolution == variant.Resolution {
// 			glog.Errorf("Variant with Bandwidth %v and Resolution %v already exists", v.Bandwidth, v.Resolution)
// 			return ErrAddVariant
// 		}
// 	}

// 	//Append to master playlist
// 	s.masterPlCache.Append(variant.URI, variant.Chunklist, variant.VariantParams)

// 	//Add to mediaPLCache
// 	s.variantMediaPlCache[strmID] = variant.Chunklist

// 	//Create the "media playlist specific" lock
// 	s.lockMap[strmID] = &sync.Mutex{}

// 	return nil
// }
