package stream

import (
	"context"
	"errors"
	"fmt"
	"io"
	"reflect"
	"time"

	"sync"

	"github.com/golang/glog"
	"github.com/nareix/joy4/av"
	"github.com/nareix/joy4/av/pubsub"
)

var ErrWrongFormat = errors.New("WrongVideoFormat")
var ErrStreamSubscriber = errors.New("StreamSubscriberError")
var HLSWorkerSleepTime = time.Millisecond * 500

type StreamSubscriber struct {
	stream          Stream
	lock            sync.Mutex
	rtmpSubscribers map[string]av.Muxer
	rtmpHeader      []av.CodecData
	hlsSubscribers  map[string]HLSMuxer
}

func NewStreamSubscriber(s Stream) *StreamSubscriber {
	return &StreamSubscriber{stream: s, rtmpSubscribers: make(map[string]av.Muxer), hlsSubscribers: make(map[string]HLSMuxer)}
}

func (s *StreamSubscriber) SubscribeRTMP(muxID string, mux av.Muxer) error {
	if len(s.hlsSubscribers) != 0 {
		glog.Errorf("Cannot add RTMP subscriber.  Already have HLS subscribers.")
		return ErrWrongFormat
	}

	if s.rtmpHeader != nil {
		mux.WriteHeader(s.rtmpHeader)
	}

	s.lock.Lock()
	s.rtmpSubscribers[muxID] = mux
	s.lock.Unlock()
	// glog.Infof("subscriber length: %v", len(s.rtmpSubscribers))
	return nil
}

func (s *StreamSubscriber) UnsubscribeRTMP(muxID string) error {
	if s.rtmpSubscribers[muxID] == nil {
		return ErrNotFound
	}
	delete(s.rtmpSubscribers, muxID)
	return nil
}

func (s *StreamSubscriber) HasSubscribers() bool {
	rs := len(s.rtmpSubscribers)
	hs := len(s.hlsSubscribers)

	return rs+hs > 0
}

func (s *StreamSubscriber) StartRTMPWorker(ctx context.Context) error {
	// glog.Infof("Starting RTMP worker")
	q := pubsub.NewQueue()
	go s.stream.ReadRTMPFromStream(ctx, q)

	m := q.Oldest()
	// glog.Infof("Waiting for rtmp header in worker")
	headers, _ := m.Streams()
	// glog.Infof("StartRTMPWorker: rtmp headers: %v", headers)
	s.rtmpHeader = headers
	for _, rtmpMux := range s.rtmpSubscribers {
		rtmpMux.WriteHeader(headers)
	}

	for {
		pkt, err := m.ReadPacket()

		// glog.Infof("Writing packet %v", pkt.Data)
		if err != nil {
			if err == io.EOF {
				// glog.Info("Got EOF, stopping RTMP subscribers now.")
				for _, rtmpMux := range s.rtmpSubscribers {
					rtmpMux.WriteTrailer()
				}
				return err
			}
			glog.Errorf("Error while reading RTMP in subscriber worker: %v", err)
			return err
		}

		// glog.Infof("subsciber len: %v", len(s.rtmpSubscribers))
		for _, rtmpMux := range s.rtmpSubscribers {
			rtmpMux.WritePacket(pkt)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
	}
}

func (s *StreamSubscriber) SubscribeHLS(muxID string, mux HLSMuxer) error {
	if len(s.rtmpSubscribers) != 0 {
		glog.Errorf("Cannot add HLS subscriber.  Already have RTMP subscribers.")
		return ErrWrongFormat
	}

	// fmt.Println("adding mux to subscribers")
	if s.hlsSubscribers[muxID] != nil {
		glog.Errorf("Subscription already exists for %v: %v", muxID, reflect.TypeOf(s.hlsSubscribers))
		return ErrStreamSubscriber
	}

	s.hlsSubscribers[muxID] = mux
	return nil
}

func (s *StreamSubscriber) UnsubscribeHLS(muxID string) error {
	if s.hlsSubscribers[muxID] == nil {
		return ErrNotFound
	}

	delete(s.hlsSubscribers, muxID)
	return nil
}

func (s *StreamSubscriber) UnsubscribeAll() error {
	if s.hlsSubscribers != nil {
		for k := range s.hlsSubscribers {
			delete(s.hlsSubscribers, k)
		}
	}

	if s.rtmpSubscribers != nil {
		for k := range s.rtmpSubscribers {
			delete(s.rtmpSubscribers, k)
		}
	}

	return nil
}

func (s *StreamSubscriber) StartHLSWorker(ctx context.Context, segWaitTime time.Duration) error {
	lastSegTimer := time.Now()
	for {
		seg, err := s.stream.ReadHLSSegment()
		if err != nil {
			if err == ErrBufferEmpty && lastSegTimer.Add(segWaitTime).After(time.Now()) {
				//Read failed because we have exhausted the list.  Wait and try again.
				time.Sleep(HLSWorkerSleepTime)
				continue
			} else {
				glog.Errorf("Error reading segment in HLS subscribe worker: %v", err)
				return err
			}
		}

		lastSegTimer = time.Now()

		for _, hlsmux := range s.hlsSubscribers {
			// glog.Infof("Writing segment %v to muxes", strings.Split(seg.Name, "_")[1])
			hlsmux.WriteSegment(seg.SeqNo, seg.Name, seg.Duration, seg.Data)
		}

		select {
		case <-ctx.Done():
			glog.Errorf("Canceling HLS Worker.")
			return ctx.Err()
		default:
		}
	}
}

func (s *StreamSubscriber) GetHLSMuxer(subID string) HLSMuxer {
	return s.hlsSubscribers[subID]
}

func (s *StreamSubscriber) GetRTMPBuffer(subID string) av.Muxer {
	return s.rtmpSubscribers[subID]
}

func (s *StreamSubscriber) HLSSubscribersReport() []string {
	var res []string
	for sub, v := range s.hlsSubscribers {
		res = append(res, fmt.Sprintf("%v: %v", reflect.TypeOf(v), sub))
	}
	return res
}

func (s *StreamSubscriber) RTMPSubscribersReport() []string {
	var res []string
	for sub, v := range s.rtmpSubscribers {
		res = append(res, fmt.Sprintf("%v: %v", reflect.TypeOf(v), sub))
	}
	return res
}
