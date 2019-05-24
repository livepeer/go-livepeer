package stream

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"sync"
	"time"

	"github.com/golang/glog"
	"github.com/nareix/joy4/av"
)

type BasicRTMPVideoStream struct {
	streamID     string
	ch           chan *av.Packet
	listeners    map[string]av.MuxCloser
	listnersLock *sync.Mutex
	dirty        bool // set after listeners has been updated; reset after read
	header       []av.CodecData
	EOF          chan struct{}
	closed       bool
	closeLock    *sync.Mutex
	RTMPTimeout  time.Duration
}

//NewBasicRTMPVideoStream creates a new BasicRTMPVideoStream.  The default RTMPTimeout is set to 10 milliseconds because we assume all RTMP streams are local.
func NewBasicRTMPVideoStream(id string) *BasicRTMPVideoStream {
	ch := make(chan *av.Packet)
	eof := make(chan struct{})
	listeners := make(map[string]av.MuxCloser)
	lLock := &sync.Mutex{}
	cLock := &sync.Mutex{}

	s := &BasicRTMPVideoStream{streamID: id, listeners: listeners, listnersLock: lLock, ch: ch, EOF: eof, closeLock: cLock, closed: false}
	//Automatically start a worker that reads packets.  There is no buffering of the video packets.
	go func(strm *BasicRTMPVideoStream) {
		var cache map[string]av.MuxCloser
		for {
			select {
			case pkt := <-strm.ch:
				strm.listnersLock.Lock()
				if strm.dirty {
					cache = make(map[string]av.MuxCloser)
					for d, l := range strm.listeners {
						cache[d] = l
					}
					strm.dirty = false
				}
				strm.listnersLock.Unlock()
				for dstid, l := range cache {
					if err := l.WritePacket(*pkt); err != nil {
						glog.Infof("RTMP stream got error: %v", err)
						go strm.deleteListener(dstid)
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
func (s *BasicRTMPVideoStream) ReadRTMPFromStream(ctx context.Context, dst av.MuxCloser) (eof chan struct{}, err error) {

	// probably not the best named lock to use but lower risk of deadlock
	s.closeLock.Lock()
	hdr := s.header
	s.closeLock.Unlock()

	if err := dst.WriteHeader(hdr); err != nil {
		return nil, err
	}

	dstid := randString()
	s.listnersLock.Lock()
	s.listeners[dstid] = dst
	s.dirty = true
	s.listnersLock.Unlock()

	eof = make(chan struct{})
	go func(ctx context.Context, eof chan struct{}, dstid string, dst av.MuxCloser) {
		select {
		case <-s.EOF:
			dst.WriteTrailer()
			s.deleteListener(dstid)
			eof <- struct{}{}
			return
		case <-ctx.Done():
			dst.WriteTrailer()
			s.deleteListener(dstid)
			return
		}
	}(ctx, eof, dstid, dst)

	return eof, nil
}

//WriteRTMPToStream writes a video stream from src into the stream.
func (s *BasicRTMPVideoStream) WriteRTMPToStream(ctx context.Context, src av.DemuxCloser) (eof chan struct{}, err error) {
	//Set header in case we want to use it.
	h, err := src.Streams()
	if err != nil {
		return nil, err
	}
	// probably not the best named lock to use but lower risk of deadlock
	s.closeLock.Lock()
	s.header = h
	s.closeLock.Unlock()

	eof = make(chan struct{})
	go func(ch chan *av.Packet) {
		for {
			packet, err := src.ReadPacket()
			if err != nil {
				if err != io.EOF {
					glog.Errorf("Error reading packet from RTMP: %v", err)
				}
				s.Close()
				return
			}

			ch <- &packet
		}
	}(s.ch)

	go func(strmEOF chan struct{}, eof chan struct{}) {
		select {
		case <-strmEOF:
			src.Close()
			eof <- struct{}{}
		}
	}(s.EOF, eof)

	return eof, nil
}

func (s *BasicRTMPVideoStream) Close() {
	s.closeLock.Lock()
	defer s.closeLock.Unlock()
	if s.closed {
		return
	}
	s.closed = true
	glog.V(2).Infof("Closing RTMP %v", s.streamID)
	close(s.EOF)
}

func (s *BasicRTMPVideoStream) deleteListener(dstid string) {
	s.listnersLock.Lock()
	defer s.listnersLock.Unlock()
	delete(s.listeners, dstid)
	s.dirty = true
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

func randString() string {
	rand.Seed(time.Now().UnixNano())
	x := make([]byte, 10, 10)
	for i := 0; i < len(x); i++ {
		x[i] = byte(rand.Uint32())
	}
	return string(x)
}
