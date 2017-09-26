package core

import (
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/golang/glog"

	"github.com/livepeer/go-livepeer/common"
	lpmon "github.com/livepeer/go-livepeer/monitor"
	"github.com/livepeer/lpms/stream"
)

var ErrNotFound = errors.New("NotFound")
var ErrVideoDB = errors.New("ErrVideoDB")

const HLSWaitTime = time.Second * 10
const HLSStreamWinSize = uint(3)

//VideoDB stores the video streams, the video buffers, and related information in memory. Note that HLS streams may have many streamIDs, each representing a
//ABS rendition, so there may be multiple streamIDs that map to the same HLS stream in the streams map.
type VideoDB struct {
	streams      map[StreamID]stream.VideoStream
	manifests    map[ManifestID]stream.HLSVideoManifest
	streamJobMap map[StreamID]*big.Int
	SelfNodeID   string
}

//NewVideo Create a new VideoDB with the node's network address
func NewVideoDB(selfNodeID string) *VideoDB {
	return &VideoDB{
		streams:    make(map[StreamID]stream.VideoStream),
		manifests:  make(map[ManifestID]stream.HLSVideoManifest),
		SelfNodeID: selfNodeID}
}

//GetHLSStream gets a HLS video stream stored in the VideoDB
func (s *VideoDB) GetHLSStream(id StreamID) stream.HLSVideoStream {
	// glog.Infof("Getting stream with %v, %v", id, s.streams[id])
	strm, ok := s.streams[id]
	if !ok {
		return nil
	}
	if strm.GetStreamFormat() != stream.HLS {
		return nil
	}
	return strm.(stream.HLSVideoStream)
}

//GetRTMPStream gets a RTMP video stream stored in the VideoDB
func (s *VideoDB) GetRTMPStream(id StreamID) stream.RTMPVideoStream {
	// glog.Infof("Getting stream with %v, %v", id, s.streams[id])
	strm, ok := s.streams[id]
	if !ok {
		return nil
	}
	if strm.GetStreamFormat() != stream.RTMP {
		return nil
	}
	return strm.(stream.RTMPVideoStream)
}

//AddNewHLSStream creates a new HLS video stream in the VideoDB
func (s *VideoDB) AddNewHLSStream(strmID StreamID) (strm stream.HLSVideoStream, err error) {
	_, ok := s.streams[strmID]
	if ok {
		return nil, ErrVideoDB
	}
	strm = stream.NewBasicHLSVideoStream(strmID.String(), HLSStreamWinSize)
	s.streams[strmID] = strm
	lpmon.Instance().LogStream(strmID.String(), 0, 0)

	// glog.Infof("Adding new video stream with ID: %v", strmID)
	return strm, nil
}

//AddNewRTMPStream creates a new RTMP video stream in the VideoDB
func (s *VideoDB) AddNewRTMPStream(strmID StreamID) (strm stream.RTMPVideoStream, err error) {
	_, ok := s.streams[strmID]
	if ok {
		return nil, ErrVideoDB
	}
	strm = stream.NewBasicRTMPVideoStream(strmID.String())
	s.streams[strmID] = strm
	lpmon.Instance().LogStream(strmID.String(), 0, 0)

	// glog.Infof("Adding new video stream with ID: %v", strmID)
	return strm, nil
}

//AddStream adds an existing video stream.
func (s *VideoDB) AddStream(strmID StreamID, strm stream.VideoStream) (err error) {
	if _, ok := s.streams[strmID]; ok {
		return ErrVideoDB
	}

	glog.V(common.VERBOSE).Infof("Adding stream: %v", strmID)
	s.streams[strmID] = strm
	lpmon.Instance().LogStream(strmID.String(), 0, 0)
	return nil
}

func (s *VideoDB) DeleteStream(strmID StreamID) {
	if _, ok := s.streams[strmID]; !ok {
		return
	}

	delete(s.streams, strmID)
}

func (s *VideoDB) GetStreamIDs(format stream.VideoFormat) []StreamID {
	result := make([]StreamID, 0)
	for id, strm := range s.streams {
		if strm.GetStreamFormat() == format {
			result = append(result, id)
		}
	}
	return result
}

func (s *VideoDB) AddNewHLSManifest(manifestID ManifestID) (stream.HLSVideoManifest, error) {
	_, ok := s.manifests[manifestID]
	if ok {
		return nil, ErrVideoDB
	}
	m := stream.NewBasicHLSVideoManifest(manifestID.String())
	s.manifests[manifestID] = m
	return m, nil
}

func (s *VideoDB) GetHLSManifest(mid ManifestID) stream.HLSVideoManifest {
	m, ok := s.manifests[mid]
	if !ok {
		return nil
	}

	if m.GetVideoFormat() != stream.HLS {
		return nil
	}
	return m.(stream.HLSVideoManifest)
}

func (s *VideoDB) GetHLSManifestFromStreamID(strmID StreamID) (stream.HLSVideoManifest, error) {
	for _, m := range s.manifests {
		mpl, err := m.GetManifest()
		if err != nil {
			glog.Errorf("Error getting manifest: %v", err)
			return nil, ErrVideoDB
		}
		for _, v := range mpl.Variants {
			vsid := strings.Split(v.URI, ".")[0]
			if strmID.String() == vsid {
				return m, nil
			}
		}
		// for _, strm := range m.GetVideoStreams() {
		// 	if strm.GetStreamID() == strmID.String() {
		// 		return m, nil
		// 	}
		// }
	}
	return nil, ErrNotFound
}

func (s *VideoDB) DeleteHLSManifest(mid ManifestID) {
	delete(s.manifests, mid)
}

func (s *VideoDB) GetJidByStreamID(strmID StreamID) *big.Int {
	jid, ok := s.streamJobMap[strmID]
	if ok {
		return jid
	}
	return nil
}

func (s *VideoDB) AddJid(strmID StreamID, jid *big.Int) {
	s.streamJobMap[strmID] = jid
}

func (s VideoDB) String() string {
	streams := ""
	for sid, s := range s.streams {
		streams = streams + fmt.Sprintf("\nAddr:%p, streamID:%v, %v", s, sid, s)
	}

	manifests := ""
	for mid, m := range s.manifests {
		manifests = manifests + fmt.Sprintf("\nManifestID: %v, %v", mid, m)
	}
	return fmt.Sprintf("\nManifests:%v\n\nStreams:%v\n\n", manifests, streams)
}
