package stream

import (
	"bytes"
	"context"
	"errors"
	"io"
	"reflect"
	"runtime/debug"

	"time"

	"github.com/ericxtang/m3u8"
	"github.com/golang/glog"
	"github.com/nareix/joy4/av"
)

var ErrBufferFull = errors.New("Stream Buffer Full")
var ErrBufferEmpty = errors.New("Stream Buffer Empty")
var ErrBufferItemType = errors.New("Buffer Item Type Not Recognized")
var ErrDroppedRTMPStream = errors.New("RTMP Stream Stopped Without EOF")
var ErrHttpReqFailed = errors.New("Http Request Failed")

type VideoFormat uint32

var (
	HLS  = MakeVideoFormatType(avFormatTypeMagic + 1)
	RTMP = MakeVideoFormatType(avFormatTypeMagic + 2)
)

func MakeVideoFormatType(base uint32) (c VideoFormat) {
	c = VideoFormat(base) << videoFormatOtherBits
	return
}

const avFormatTypeMagic = 577777
const videoFormatOtherBits = 1

type RTMPEOF struct{}

type streamBuffer struct {
	q *Queue
}

func newStreamBuffer() *streamBuffer {
	return &streamBuffer{q: NewQueue(1000)}
}

func (b *streamBuffer) push(in interface{}) error {
	b.q.Put(in)
	return nil
}

func (b *streamBuffer) poll(ctx context.Context, wait time.Duration) (interface{}, error) {
	results, err := b.q.Poll(ctx, 1, wait)
	if err != nil {
		return nil, err
	}
	result := results[0]
	return result, nil
}

func (b *streamBuffer) pop() (interface{}, error) {
	results, err := b.q.Get(1)
	if err != nil {
		return nil, err
	}
	result := results[0]
	return result, nil
}

func (b *streamBuffer) len() int64 {
	return b.q.Len()
}

//We couldn't just use the m3u8 definition
type HLSSegment struct {
	SeqNo    uint64
	Name     string
	Data     []byte
	Duration float64
	EOF      bool
}

type Stream interface {
	GetStreamID() string
	Len() int64
	// NewStream() Stream
	ReadRTMPFromStream(ctx context.Context, dst av.MuxCloser) error
	WriteRTMPToStream(ctx context.Context, src av.DemuxCloser) error
	WriteHLSPlaylistToStream(pl m3u8.MediaPlaylist) error
	WriteHLSSegmentToStream(seg HLSSegment) error
	ReadHLSFromStream(ctx context.Context, buffer HLSMuxer) error
	ReadHLSSegment() (HLSSegment, error)
}

type VideoStream struct {
	StreamID    string
	Format      VideoFormat
	RTMPTimeout time.Duration
	HLSTimeout  time.Duration
	buffer      *streamBuffer
}

func (s *VideoStream) Len() int64 {
	// glog.Infof("buffer.q: %v", s.buffer.q)
	return s.buffer.len()
}

func NewVideoStream(id string, format VideoFormat) *VideoStream {
	return &VideoStream{buffer: newStreamBuffer(), StreamID: id, Format: format}
}

func (s *VideoStream) GetStreamID() string {
	return s.StreamID
}

//ReadRTMPFromStream reads the content from the RTMP stream out into the dst.
func (s *VideoStream) ReadRTMPFromStream(ctx context.Context, dst av.MuxCloser) error {
	defer dst.Close()

	//TODO: Make sure to listen to ctx.Done()
	for {
		item, err := s.buffer.poll(ctx, s.RTMPTimeout)
		if err != nil {
			return err
		}

		switch item.(type) {
		case []av.CodecData:
			headers := item.([]av.CodecData)
			err = dst.WriteHeader(headers)
			if err != nil {
				glog.Infof("Error writing RTMP header from Stream %v to mux", s.StreamID)
				return err
			}
		case av.Packet:
			packet := item.(av.Packet)
			err = dst.WritePacket(packet)
			if err != nil {
				glog.Infof("Error writing RTMP packet from Stream %v to mux: %v", s.StreamID, err)
				return err
			}
		case RTMPEOF:
			err := dst.WriteTrailer()
			if err != nil {
				glog.Infof("Error writing RTMP trailer from Stream %v", s.StreamID)
				return err
			}
			return io.EOF
		default:
			glog.Infof("Cannot recognize buffer iteam type: ", reflect.TypeOf(item))
			debug.PrintStack()
			return ErrBufferItemType
		}
	}
}

func (s *VideoStream) WriteRTMPHeader(h []av.CodecData) {
	s.buffer.push(h)
}

func (s *VideoStream) WriteRTMPPacket(p av.Packet) {
	s.buffer.push(p)
}

func (s *VideoStream) WriteRTMPTrailer() {
	s.buffer.push(RTMPEOF{})
}

//WriteRTMPToStream writes a video stream from src into the stream.
func (s *VideoStream) WriteRTMPToStream(ctx context.Context, src av.DemuxCloser) error {
	defer src.Close()

	c := make(chan error, 1)
	go func() {
		c <- func() error {
			header, err := src.Streams()
			if err != nil {
				return err
			}
			err = s.buffer.push(header)
			if err != nil {
				return err
			}

			// var lastKeyframe av.Packet
			for {
				packet, err := src.ReadPacket()
				if err == io.EOF {
					s.buffer.push(RTMPEOF{})
					return err
				} else if err != nil {
					return err
				} else if len(packet.Data) == 0 { //TODO: Investigate if it's possible for packet to be nil (what happens when RTMP stopped publishing because of a dropped connection? Is it possible to have err and packet both nil?)
					return ErrDroppedRTMPStream
				}

				if packet.IsKeyFrame {
					// lastKeyframe = packet
				}

				err = s.buffer.push(packet)
				if err == ErrBufferFull {
					//TODO: Delete all packets until last keyframe, insert headers in front - trying to get rid of streaming artifacts.
				}
			}
		}()
	}()

	select {
	case <-ctx.Done():
		glog.Infof("Finished writing RTMP to Stream %v", s.StreamID)
		return ctx.Err()
	case err := <-c:
		return err
	}
}

func (s *VideoStream) WriteHLSPlaylistToStream(pl m3u8.MediaPlaylist) error {
	return s.buffer.push(pl)
}

func (s *VideoStream) WriteHLSSegmentToStream(seg HLSSegment) error {
	return s.buffer.push(seg)
}

//ReadHLSFromStream reads an HLS stream into an HLSBuffer
func (s *VideoStream) ReadHLSFromStream(ctx context.Context, mux HLSMuxer) error {
	ec := make(chan error, 1)
	go func() {
		ec <- func() error {
			for {
				// glog.Info("HLS Stream Buffer Len: %v\n", s.buffer.len())
				item, err := s.buffer.poll(ctx, s.HLSTimeout)
				if err != nil {
					return err
				}

				switch item.(type) {
				case m3u8.MediaPlaylist:
					//Do nothing for now - we are generating playlist from segments
					// mux.WritePlaylist(item.(m3u8.MediaPlaylist))
				case HLSSegment:
					mux.WriteSegment(item.(HLSSegment).SeqNo, item.(HLSSegment).Name, item.(HLSSegment).Duration, item.(HLSSegment).Data)
				default:
					return ErrBufferItemType
				}
			}
		}()
	}()

	select {
	case err := <-ec:
		glog.Errorf("Got error reading HLS: %v", err)
		return err
	}
}

func (s *VideoStream) ReadHLSSegment() (HLSSegment, error) {
	var firstPl m3u8.MediaPlaylist
	for {
		item, err := s.buffer.poll(context.Background(), s.HLSTimeout)
		if err != nil {
			return HLSSegment{}, err
		}

		switch item.(type) {
		case m3u8.MediaPlaylist:
			//Keep track of the playlist and re-insert it. If we see it again, it means we are done reading HLS segments.
			//Note this only works in a synchronous case.  Other threads can insert segments when we are reading.
			// glog.Info("Got playlist")
			s.buffer.push(item)
			pl := item.(m3u8.MediaPlaylist)
			if len(firstPl.Segments) == 0 && firstPl.SeqNo == 0 && firstPl.TargetDuration == 0 {
				firstPl = pl
			} else {
				if samePlaylist(firstPl, pl) {
					return HLSSegment{}, ErrBufferEmpty
				}
			}
		case HLSSegment:
			return item.(HLSSegment), nil
		default:
		}
	}
}

func (s *VideoStream) ReadHLSPlaylist() (m3u8.MediaPlaylist, error) {
	var firstSeg HLSSegment
	for {
		item, err := s.buffer.poll(context.Background(), s.HLSTimeout)
		if err != nil {
			return m3u8.MediaPlaylist{}, err
		}

		switch item.(type) {
		case m3u8.MediaPlaylist:
			return item.(m3u8.MediaPlaylist), nil
			//Keep track of the segment and re-insert it. If we see it again, it means we are done reading playlists.
			//Note this only works in a synchronous case.  Other threads can insert segments when we are reading.
		case HLSSegment:
			s.buffer.push(item)
			seg := item.(HLSSegment)
			if firstSeg.Name == "" {
				firstSeg = seg
			} else {
				if firstSeg.Name == seg.Name {
					return m3u8.MediaPlaylist{}, ErrNotFound
				}
			}
		default:
		}
	}
}

//Compare playlists by segments
func samePlaylist(p1, p2 m3u8.MediaPlaylist) bool {
	return bytes.Compare(p1.Encode().Bytes(), p2.Encode().Bytes()) == 0
	//TODO: Remove all the nils from s1 and s2
	// s1 := p1.Segments
	// s2 := p2.Segments

	// if p1.SeqNo != p2.SeqNo || len(s1) != len(s2) || p1.Iframe != p2.Iframe || p1.Key != p2.Key || p1.MediaType != p2.MediaType {
	// 	return false
	// }

	// //This is not the correct way to sort, but all we want is consistency here.
	// sort.Slice(s1, func(i, j int) bool {
	// 	if s1[i] == nil || s1[j] == nil {
	// 		return false
	// 	}
	// 	return s1[i].URI < s1[j].URI
	// })

	// sort.Slice(s2, func(i, j int) bool {
	// 	if s2[i] == nil || s2[j] == nil {
	// 		return false
	// 	}
	// 	return s2[i].URI < s2[j].URI
	// })

	// glog.Infof("s1: %v", s1)
	// glog.Infof("s2: %v", s2)
	// for i := 0; i < len(s1); i++ {
	// 	if (s1[i] == nil && s2[i] != nil) || (s1[i] != nil && s2[i] == nil) {
	// 		return false
	// 	}

	// 	if s1[i].URI != s2[i].URI {
	// 		return false
	// 	}
	// }

	// return true
}
