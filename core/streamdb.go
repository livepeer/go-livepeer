package core

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ericxtang/m3u8"
	"github.com/golang/glog"

	lpmon "github.com/livepeer/go-livepeer/monitor"
	"github.com/livepeer/lpms/stream"
)

var ErrNotFound = errors.New("NotFound")

const HLSWaitTime = time.Second * 10

//StreamDB stores the video streams, the video buffers, and related information in memory. Note that HLS streams may have many streamIDs, each representing a
//ABS rendition, so there may be multiple streamIDs that map to the same HLS stream in the streams map.
type StreamDB struct {
	streams    map[StreamID]stream.VideoStream_
	SelfNodeID string
}

//NewStreamDB Create a new StreamDB with the node's network address
func NewStreamDB(selfNodeID string) *StreamDB {
	return &StreamDB{
		streams:    make(map[StreamID]stream.VideoStream_),
		SelfNodeID: selfNodeID}
}

//GetHLSStream gets a HLS video stream stored in the StreamDB
func (s *StreamDB) GetHLSStream(id StreamID) stream.HLSVideoStream {
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

//GetRTMPStream gets a RTMP video stream stored in the StreamDB
func (s *StreamDB) GetRTMPStream(id StreamID) stream.RTMPVideoStream {
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

//AddNewHLSStream creates a new HLS video stream in the StreamDB
func (s *StreamDB) AddNewHLSStream(strmID StreamID) (strm stream.HLSVideoStream, err error) {
	strm = stream.NewBasicHLSVideoStream(strmID.String(), stream.DefaultSegWaitTime)
	s.streams[strmID] = strm
	lpmon.Instance().LogStream(strmID.String(), 0, 0)

	// glog.Infof("Adding new video stream with ID: %v", strmID)
	return strm, nil
}

//AddNewRTMPStream creates a new RTMP video stream in the StreamDB
func (s *StreamDB) AddNewRTMPStream(strmID StreamID) (strm stream.RTMPVideoStream, err error) {
	strm = stream.NewBasicRTMPVideoStream(strmID.String())
	s.streams[strmID] = strm
	lpmon.Instance().LogStream(strmID.String(), 0, 0)

	// glog.Infof("Adding new video stream with ID: %v", strmID)
	return strm, nil
}

//AddHLSVariant adds a new variant for the HLS video stream
func (s *StreamDB) AddHLSVariant(hlsStrmID StreamID, varStrmID StreamID, variant *m3u8.Variant) error {
	strm, ok := s.streams[hlsStrmID]
	if !ok {
		return ErrNotFound
	}

	hlsStrm, ok := strm.(stream.HLSVideoStream)
	if !ok {
		return ErrNotFound
	}

	if err := hlsStrm.AddVariant(varStrmID.String(), variant); err != nil {
		return ErrNotFound
	}

	//Add variant streamID to streams map so we can keep track
	s.streams[varStrmID] = strm
	lpmon.Instance().LogStream(varStrmID.String(), 0, 0)
	return nil
}

//AddStream adds an existing video stream.
func (s *StreamDB) AddStream(strmID StreamID, strm stream.VideoStream_) (err error) {
	s.streams[strmID] = strm
	lpmon.Instance().LogStream(strmID.String(), 0, 0)
	return nil
}

//DeleteStream removes a video stream from the StreamDB
func (s *StreamDB) DeleteStream(strmID StreamID) {
	strm, ok := s.streams[strmID]
	if !ok {
		return
	}

	if strm.GetStreamFormat() == stream.HLS {
		//Remove all the variant lookups too
		hlsStrm := strm.(stream.HLSVideoStream)
		mpl, err := hlsStrm.GetMasterPlaylist()
		if err != nil {
			glog.Errorf("Error getting master playlist: %v", err)
		}
		for _, v := range mpl.Variants {
			vName := strings.Split(v.URI, ".")[0]
			delete(s.streams, StreamID(vName))
		}
	}
	delete(s.streams, strmID)
	// glog.Infof("streams len after delete: %v", len(s.streams))
}

func (s *StreamDB) GetStreamIDs(format stream.VideoFormat) []StreamID {
	result := make([]StreamID, 0)
	for id, strm := range s.streams {
		if strm.GetStreamFormat() == format {
			result = append(result, id)
		}
	}
	return result
}

func (s StreamDB) String() string {
	streams := ""
	for vid, s := range s.streams {
		streams = streams + fmt.Sprintf("\nVariantID:%v, %v", vid, s)
	}

	return fmt.Sprintf("\nStreams:%v\n\n", streams)
}
