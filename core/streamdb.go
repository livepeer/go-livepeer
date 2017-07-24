package core

import (
	"context"
	"errors"
	"time"

	"github.com/livepeer/lpms/stream"
)

var ErrNotFound = errors.New("NotFound")

const HLSWaitTime = time.Second * 10

type StreamDB struct {
	streams      map[StreamID]*stream.VideoStream
	subscribers  map[StreamID]*stream.StreamSubscriber
	hlsBuffers   map[StreamID]*stream.HLSBuffer
	cancellation map[StreamID]context.CancelFunc
	SelfAddress  string
}

func NewStreamDB(selfAddr string) *StreamDB {
	return &StreamDB{
		streams:      make(map[StreamID]*stream.VideoStream),
		subscribers:  make(map[StreamID]*stream.StreamSubscriber),
		hlsBuffers:   make(map[StreamID]*stream.HLSBuffer),
		cancellation: make(map[StreamID]context.CancelFunc),
		SelfAddress:  selfAddr}
}

func (s *StreamDB) GetStream(id StreamID) *stream.VideoStream {
	// glog.Infof("Getting stream with %v, %v", id, s.streams[id])
	strm, ok := s.streams[id]
	if !ok {
		return nil
	}
	return strm
}

func (s *StreamDB) AddNewStream(strmID StreamID, format stream.VideoFormat) (strm *stream.VideoStream, err error) {
	strm = stream.NewVideoStream(strmID.String(), format)
	s.streams[strmID] = strm

	// glog.Infof("Adding new video stream with ID: %v", strmID)
	return strm, nil
}

func (s *StreamDB) AddStream(strmID StreamID, strm *stream.VideoStream) (err error) {
	s.streams[strmID] = strm
	return nil
}

func (s *StreamDB) DeleteStream(strmID StreamID) {
	delete(s.streams, strmID)
}

func (s *StreamDB) GetHLSBuffer(strmID StreamID) *stream.HLSBuffer {
	buf, ok := s.hlsBuffers[strmID]
	if !ok {
		return nil
	}
	return buf
}

func (s *StreamDB) AddNewHLSBuffer(strmID StreamID) *stream.HLSBuffer {
	buf := stream.NewHLSBuffer(5, 1000) //TODO: Need to fix the static cap
	s.hlsBuffers[strmID] = buf
	return buf
}

func (s *StreamDB) DeleteHLSBuffer(strmID StreamID) {
	delete(s.hlsBuffers, strmID)
}

func (self *StreamDB) SubscribeToHLSStream(strmID string, subID string, mux stream.HLSMuxer) error {
	strm := self.streams[StreamID(strmID)]
	if strm == nil {
		return ErrNotFound
	}

	sub := self.subscribers[StreamID(strmID)]
	if sub == nil {
		sub = stream.NewStreamSubscriber(strm)
		self.subscribers[StreamID(strmID)] = sub
		ctx, cancel := context.WithCancel(context.Background())
		go sub.StartHLSWorker(ctx, HLSWaitTime)
		self.cancellation[StreamID(strmID)] = cancel
	}

	return sub.SubscribeHLS(subID, mux)
}

func (self *StreamDB) UnsubscribeToHLSStream(strmID string, subID string) {
	sub := self.subscribers[StreamID(strmID)]
	if sub != nil {
		sub.UnsubscribeHLS(subID)
	} else {
		return
	}

	if !sub.HasSubscribers() {
		self.cancellation[StreamID(strmID)]() //Call cancel on hls worker
		delete(self.subscribers, StreamID(strmID))
		sid := StreamID(strmID)
		nid := string(sid.GetNodeID())
		if self.SelfAddress != nid { //Only delete the networkStream if you are a relay node
			delete(self.streams, StreamID(strmID))
		}
	}
}

// func (self *StreamDB) GetHLSMuxer(strmID string, subID string) (mux stream.HLSMuxer) {
// 	sub := self.subscribers[StreamID(strmID)]
// 	if sub != nil {
// 		return sub.GetHLSMuxer(subID)
// 	}
// 	return nil
// }
