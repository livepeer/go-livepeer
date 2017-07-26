package core

import (
	"context"
	"errors"
	"time"

	"github.com/livepeer/lpms/stream"
)

var ErrNotFound = errors.New("NotFound")

const HLSWaitTime = time.Second * 10

//StreamDB stores the video streams, the video buffers, and related information in memory.
type StreamDB struct {
	streams      map[StreamID]*stream.VideoStream
	subscribers  map[StreamID]*stream.StreamSubscriber
	hlsBuffers   map[StreamID]*stream.HLSBuffer
	cancellation map[StreamID]context.CancelFunc
	SelfNodeID   string
}

//Create a new StreamDB with the node's network address
func NewStreamDB(selfNodeID string) *StreamDB {
	return &StreamDB{
		streams:      make(map[StreamID]*stream.VideoStream),
		subscribers:  make(map[StreamID]*stream.StreamSubscriber),
		hlsBuffers:   make(map[StreamID]*stream.HLSBuffer),
		cancellation: make(map[StreamID]context.CancelFunc),
		SelfNodeID:   selfNodeID}
}

//GetStream gets a video stream stored in the StreamDB
func (s *StreamDB) GetStream(id StreamID) *stream.VideoStream {
	// glog.Infof("Getting stream with %v, %v", id, s.streams[id])
	strm, ok := s.streams[id]
	if !ok {
		return nil
	}
	return strm
}

//AddNewStream creates a new video stream in the StreamDB
func (s *StreamDB) AddNewStream(strmID StreamID, format stream.VideoFormat) (strm *stream.VideoStream, err error) {
	strm = stream.NewVideoStream(strmID.String(), format)
	s.streams[strmID] = strm

	// glog.Infof("Adding new video stream with ID: %v", strmID)
	return strm, nil
}

//AddStream adds an existing video stream.
func (s *StreamDB) AddStream(strmID StreamID, strm *stream.VideoStream) (err error) {
	s.streams[strmID] = strm
	return nil
}

//DeleteStream removes a video stream from the StreamDB
func (s *StreamDB) DeleteStream(strmID StreamID) {
	delete(s.streams, strmID)
}

//GetHLSBuffer gets a HLS video buffer from the StreamDB
func (s *StreamDB) GetHLSBuffer(strmID StreamID) *stream.HLSBuffer {
	buf, ok := s.hlsBuffers[strmID]
	if !ok {
		return nil
	}
	return buf
}

//AddNewHLSBuffer creates a new HLS video buffer and adds it to the StreamDB
func (s *StreamDB) AddNewHLSBuffer(strmID StreamID) *stream.HLSBuffer {
	buf := stream.NewHLSBuffer(5, 1000) //TODO: Need to fix the static cap
	s.hlsBuffers[strmID] = buf
	return buf
}

//DeleteHLSBuffer removes the HLS video buffer from the StreamDB
func (s *StreamDB) DeleteHLSBuffer(strmID StreamID) {
	delete(s.hlsBuffers, strmID)
}

//SubscribeToHLSStream takes a HLSMuxer and puts any new HLS segment from the stream into the muxer.
func (s *StreamDB) SubscribeToHLSStream(strmID string, subID string, mux stream.HLSMuxer) error {
	strm := s.streams[StreamID(strmID)]
	if strm == nil {
		return ErrNotFound
	}

	sub := s.subscribers[StreamID(strmID)]
	if sub == nil {
		sub = stream.NewStreamSubscriber(strm)
		s.subscribers[StreamID(strmID)] = sub
		ctx, cancel := context.WithCancel(context.Background())
		go sub.StartHLSWorker(ctx, HLSWaitTime)
		s.cancellation[StreamID(strmID)] = cancel
	}

	return sub.SubscribeHLS(subID, mux)
}

//UnsubscribeToHLSStream unsubscribes a HLSMuxer from a stream
func (s *StreamDB) UnsubscribeToHLSStream(strmID string, subID string) {
	sub := s.subscribers[StreamID(strmID)]
	if sub != nil {
		sub.UnsubscribeHLS(subID)
	} else {
		return
	}

	if !sub.HasSubscribers() {
		s.cancellation[StreamID(strmID)]() //Call cancel on hls worker
		delete(s.subscribers, StreamID(strmID))
		sid := StreamID(strmID)
		nid := string(sid.GetNodeID())
		if s.SelfNodeID != nid { //Only delete the networkStream if you are a relay node
			delete(s.streams, StreamID(strmID))
		}
	}
}
