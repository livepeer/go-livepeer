package stream

import (
	"context"
	"fmt"
	"io"
	"reflect"
	"runtime/debug"
	"time"

	"github.com/golang/glog"
	"github.com/nareix/joy4/av"
)

type BasicRTMPVideoStream struct {
	streamID    string
	buffer      *streamBuffer
	RTMPTimeout time.Duration
}

//NewBasicRTMPVideoStream creates a new BasicRTMPVideoStream.  The default RTMPTimeout is set to 10 milliseconds because we assume all RTMP streams are local.
func NewBasicRTMPVideoStream(id string) *BasicRTMPVideoStream {
	// return &BasicRTMPVideoStream{buffer: newStreamBuffer(), streamID: id, RTMPTimeout: 10 * time.Millisecond}
	return &BasicRTMPVideoStream{buffer: newStreamBuffer(), streamID: id}
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
				glog.Errorf("Error writing RTMP header from Stream %v to mux", s.streamID)
				return err
			}
		case av.Packet:
			packet := item.(av.Packet)
			err = dst.WritePacket(packet)
			if err != nil {
				glog.Errorf("Error writing RTMP packet from Stream %v to mux: %v", s.streamID, err)
				return err
			}
		case RTMPEOF:
			err := dst.WriteTrailer()
			if err != nil {
				glog.Errorf("Error writing RTMP trailer from Stream %v", s.streamID)
				return err
			}
			return io.EOF
		default:
			glog.Errorf("Cannot recognize buffer iteam type: ", reflect.TypeOf(item))
			debug.PrintStack()
			return ErrBufferItemType
		}
	}
}

//WriteRTMPToStream writes a video stream from src into the stream.
func (s *BasicRTMPVideoStream) WriteRTMPToStream(ctx context.Context, src av.DemuxCloser) error {
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
		glog.V(2).Infof("Finished writing RTMP to Stream %v", s.streamID)
		return ctx.Err()
	case err := <-c:
		return err
	}
}

func (s BasicRTMPVideoStream) String() string {
	return fmt.Sprintf("StreamID: %v, Type: %v", s.GetStreamID(), s.GetStreamFormat())
}
