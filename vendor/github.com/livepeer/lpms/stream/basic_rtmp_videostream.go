package stream

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/golang/glog"
	"github.com/nareix/joy4/av"
)

type BasicRTMPVideoStream struct {
	streamID     string
	ch           chan *av.Packet
	src          av.DemuxCloser
	listeners    []av.MuxCloser
	listnersLock *sync.Mutex
	header       []av.CodecData
	exitWorker   chan struct{}
	EOF          chan struct{}
	RTMPTimeout  time.Duration
}

//NewBasicRTMPVideoStream creates a new BasicRTMPVideoStream.  The default RTMPTimeout is set to 10 milliseconds because we assume all RTMP streams are local.
func NewBasicRTMPVideoStream(id string) *BasicRTMPVideoStream {
	ch := make(chan *av.Packet)
	eof := make(chan struct{})
	listeners := make([]av.MuxCloser, 0)
	lLock := &sync.Mutex{}

	s := &BasicRTMPVideoStream{streamID: id, listeners: listeners, listnersLock: lLock, ch: ch, EOF: eof}
	//Automatically start a worker that reads packets.  There is no buffering of the video packets.
	go func(strm *BasicRTMPVideoStream) {
		for {
			select {
			case pkt := <-strm.ch:
				for i, l := range strm.listeners {
					if err := l.WritePacket(*pkt); err != nil {
						lLock.Lock()
						strm.listeners = append(strm.listeners[:i], strm.listeners[i+1:]...)
						lLock.Unlock()
					}
				}
			case <-strm.EOF:
				return
			}
		}
	}(s)
	return s
}

func (s *BasicRTMPVideoStream) GetStreamID() string {
	return s.streamID
}

func (s *BasicRTMPVideoStream) GetStreamFormat() VideoFormat {
	return RTMP
}

//ReadRTMPFromStream reads the content from the RTMP stream out into the dst.
func (s *BasicRTMPVideoStream) ReadRTMPFromStream(ctx context.Context, dst av.MuxCloser) error {
	defer dst.Close()

	if err := dst.WriteHeader(s.header); err != nil {
		return err
	}

	s.listnersLock.Lock()
	s.listeners = append(s.listeners, dst)
	s.listnersLock.Unlock()

	select {
	case <-s.EOF:
		if err := dst.WriteTrailer(); err != nil {
			return err
		}
	case <-ctx.Done():
		return ctx.Err()
	}

	return nil
}

//WriteRTMPToStream writes a video stream from src into the stream.
func (s *BasicRTMPVideoStream) WriteRTMPToStream(ctx context.Context, src av.DemuxCloser) (eof chan struct{}, err error) {
	//Set header in case we want to use it.
	h, err := src.Streams()
	if err != nil {
		return nil, err
	}
	s.header = h

	go func(eof chan struct{}, ch chan *av.Packet) {
		for {
			packet, err := src.ReadPacket()
			if err == io.EOF {
				glog.Infof("EOF...")
				close(eof)
				src.Close()
				return
			} else if err != nil {
				glog.Errorf("Error reading packet from RTMP: %v", err)
				close(eof)
				src.Close()
				return
			}

			ch <- &packet
		}
	}(s.EOF, s.ch)

	return s.EOF, nil
}

func (s BasicRTMPVideoStream) String() string {
	return fmt.Sprintf("StreamID: %v, Type: %v", s.GetStreamID(), s.GetStreamFormat())
}

func (s BasicRTMPVideoStream) Height() int {
	for _, cd := range s.header {
		if cd.Type().IsVideo() {
			return cd.(av.VideoCodecData).Height()
		}
	}

	return 0
}

func (s BasicRTMPVideoStream) Width() int {
	for _, cd := range s.header {
		if cd.Type().IsVideo() {
			return cd.(av.VideoCodecData).Width()
		}
	}

	return 0
}
