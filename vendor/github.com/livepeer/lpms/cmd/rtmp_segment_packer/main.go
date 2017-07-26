package main

import (
	"context"
	"flag"
	"io"
	"net/url"
	"strings"

	"time"

	"github.com/ericxtang/m3u8"
	"github.com/golang/glog"
	"github.com/livepeer/lpms/core"
	"github.com/livepeer/lpms/stream"
	"github.com/nareix/joy4/av"
)

//This is basically a test for segmenting videos.  After much research, the current best approach is to use the segment function inside FFMpeg.
//However, this test serves as a document so we can revisit later.

var StagedStream SegmentStream
var transcodeCount int

type StreamDB struct {
	db map[string]stream.Stream
}

type Segment struct {
	availSpace time.Duration
	packets    []av.Packet
}

func NewSegment() *Segment {
	return &Segment{time.Second * 2, make([]av.Packet, 0, 100)}
}

type SegmentStream struct {
	StreamID    string
	Headers     []av.CodecData
	Segments    []*Segment
	RTMPTimeout time.Duration
}

func NewSegmentStream(streamID string) *SegmentStream {
	return &SegmentStream{streamID, nil, make([]*Segment, 0, 100), time.Second * 10}
}

func (s *SegmentStream) GetStreamID() string {
	return s.StreamID
}

func (s *SegmentStream) Len() int64 {
	return int64(len(s.Segments))
}

func (s *SegmentStream) ReadRTMPFromStream(ctx context.Context, dst av.MuxCloser) error {
	defer dst.Close()

	dst.WriteHeader(s.Headers)
	transcodeCount = transcodeCount + 1

	if transcodeCount%2 == 0 {
		glog.Infof("Writing seg 2")
		for _, p := range s.Segments[2].packets {
			err := dst.WritePacket(p)
			time.Sleep(time.Millisecond * 5)

			if err != nil {
				glog.Infof("Error writing RTMP packet from Stream %v to mux", s.StreamID)
				return err
			}
		}
		glog.Infof("Writing seg 3")
		for _, p := range s.Segments[3].packets {
			err := dst.WritePacket(p)
			time.Sleep(time.Millisecond * 5)

			if err != nil {
				glog.Infof("Error writing RTMP packet from Stream %v to mux", s.StreamID)
				dst.WriteTrailer()
				return err
			}
		}

		glog.Infof("Writing seg 6")
		for _, p := range s.Segments[6].packets {
			err := dst.WritePacket(p)
			time.Sleep(time.Millisecond * 5)

			if err != nil {
				glog.Infof("Error writing RTMP packet from Stream %v to mux", s.StreamID)
				return err
			}
		}
		glog.Infof("Writing seg 7")
		for _, p := range s.Segments[7].packets {
			err := dst.WritePacket(p)
			time.Sleep(time.Millisecond * 5)

			if err != nil {
				glog.Infof("Error writing RTMP packet from Stream %v to mux", s.StreamID)
				dst.WriteTrailer()
				return err
			}
		}
	} else {
		// s.Segments = s.Segments[0:len(s.Segments)]
		for i, seg := range s.Segments {
			glog.Infof("Writing seg %v", i)
			glog.Infof("Packet Keyframe: %v, %v", seg.packets[0].IsKeyFrame, len(seg.packets[0].Data))
			for _, p := range seg.packets {
				// glog.Infof("%v", j)
				err := dst.WritePacket(p)

				if err != nil {
					glog.Infof("Error writing RTMP packet from Stream %v to mux", s.StreamID)
					dst.WriteTrailer()
					return err
				}
			}
			time.Sleep(time.Second * 1)
		}
	}

	return nil
}

func (s *SegmentStream) WriteRTMPToStream(ctx context.Context, src av.DemuxCloser) error {
	defer src.Close()

	c := make(chan error, 1)
	go func() {
		c <- func() error {
			header, err := src.Streams()
			if err != nil {
				return err
			}
			s.Headers = header

			// packet, err := src.ReadPacket()
			// tag, _ := flv.PacketToTag(packet, header[packet.Idx])
			// glog.Infof("Tag Type: %v", tag.Type)
			// glog.Infof("Tag: %v", tag)
			// var lastKeyframe av.Packet
			for {
				packet, err := src.ReadPacket()
				if packet.IsKeyFrame {
					glog.Infof("IsKeyFrame: %v\n", packet.IsKeyFrame)
					// glog.Infof("Composition Time: %v\n", packet.CompositionTime)
					glog.Infof("Time: %v\n", packet.Time)

				}
				if err == io.EOF {
					StagedStream = *s
					glog.Infof("Segments Len: %v", len(s.Segments))
					for i, seg := range s.Segments {
						glog.Infof("seg[%v] Len: %v", i, len(seg.packets))
					}
					// glog.Infof("%v", s.Segments[0].packets[0])
					// glog.Infof("%v", s.Segments[1].packets[0])
					// s.buffer.push(RTMPEOF{})
					return err
				} else if err != nil {
					return err
				} else if len(packet.Data) == 0 { //TODO: Investigate if it's possible for packet to be nil (what happens when RTMP stopped publishing because of a dropped connection? Is it possible to have err and packet both nil?)
					return stream.ErrDroppedRTMPStream
				}

				insertPacket(s, packet)
				// err = s.buffer.push(packet)
				// if err == ErrBufferFull {
				//TODO: Delete all packets until last keyframe, insert headers in front - trying to get rid of streaming artifacts.
				// }
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

func insertPacket(s *SegmentStream, pkt av.Packet) {
	var lastSeg *Segment
	if s.Len() == 0 || pkt.IsKeyFrame {
		lastSeg = NewSegment()
		s.Segments = append(s.Segments, lastSeg)
	} else {
		// glog.Infof("seg length: %v", s.Len())
		lastSeg = s.Segments[s.Len()-1]
		// if lastSeg.availSpace == 0 {
		// 	lastSeg = NewSegment()
		// 	s.Segments = append(s.Segments, lastSeg)
		// }
	}

	// glog.Infof("Appending packet: %v to %v", len(pkt.Data), lastSeg.availSpace)
	lastSeg.packets = append(lastSeg.packets, pkt)
	lastSeg.availSpace = lastSeg.availSpace - pkt.CompositionTime
}

func (s *SegmentStream) GetPacket() (pkt av.Packet) {
	if len(s.Segments) == 0 {
		pkt = av.Packet{}
		return
	}

	seg := s.Segments[0]

	if len(seg.packets) > 1 {
		pkt, seg.packets = seg.packets[0], seg.packets[1:len(seg.packets)]
	} else {
		glog.Infof("Seg len %v", len(seg.packets))
		pkt = seg.packets[0]
		if len(s.Segments) > 1 {
			s.Segments = s.Segments[1:len(s.Segments)]
		} else {
			s.Segments = []*Segment{}
		}
	}
	return
}

func (s *SegmentStream) WriteHLSPlaylistToStream(pl m3u8.MediaPlaylist) error {
	glog.Infof("Writing HLS Playlist")
	return nil
}

func (s *SegmentStream) WriteHLSSegmentToStream(seg stream.HLSSegment) error {
	glog.Infof("Writing HLS Segment")
	return nil
}

func (s *SegmentStream) ReadHLSFromStream(ctx context.Context, buffer stream.HLSMuxer) error {
	glog.Info("Reading HLS")
	return nil
}

func (s *SegmentStream) ReadHLSSegment() (stream.HLSSegment, error) {
	glog.Info("Read HLS Segment")
	return stream.HLSSegment{}, nil
}

func main() {
	flag.Set("logtostderr", "true")
	flag.Parse()

	lpms := core.New("1935", "8000", "", "")
	streamDB := &StreamDB{db: make(map[string]stream.Stream)}

	lpms.HandleRTMPPublish(
		//makeStreamID
		func(url *url.URL) (strmID string) {
			return getStreamIDFromPath(url.Path)
		},
		//gotStream
		func(url *url.URL, rtmpStrm *stream.VideoStream) error {
			streamID := getStreamIDFromPath(url.Path)
			stream1 := NewSegmentStream(streamID)
			// stream2 := NewSegmentStream(streamID)
			streamDB.db[streamID] = stream1
			return nil
		},
		//endStream
		func(url *url.URL, rtmpStrm *stream.VideoStream) error {
			delete(streamDB.db, rtmpStrm.GetStreamID())
			return nil
		})

	lpms.HandleRTMPPlay(
		//getStream
		func(url *url.URL) (stream.Stream, error) {
			src := copyStream(&StagedStream)
			return src, nil
		})

	lpms.HandleTranscode(
		//getInStream
		func(ctx context.Context, streamID string) (stream.Stream, error) {
			s := copyStream(&StagedStream)
			return s, nil
		},
		//getOutStream
		func(ctx context.Context, streamID string) (stream.Stream, error) {
			//For this example, we'll name the transcoded stream "{streamID}_tran"
			// newStream := stream.NewVideoStream(streamID + "_tran")
			// streamDB.db[newStream.GetStreamID()] = newStream
			// return newStream, nil

			glog.Infof("Making File Stream")
			fileStream := stream.NewFileStream(streamID + "_file")
			return fileStream, nil
		})
	lpms.Start(context.Background())
}

func getStreamIDFromPath(reqPath string) string {
	return "test"
}

func getHLSStreamIDFromPath(reqPath string) string {
	if strings.HasSuffix(reqPath, ".m3u8") {
		return "test_tran"
	} else {
		return "test_tran"
	}
}

func copyStream(s *SegmentStream) *SegmentStream {
	c := &SegmentStream{Headers: s.Headers, Segments: make([]*Segment, len(s.Segments))}
	for i := range c.Segments {
		seg := &Segment{packets: make([]av.Packet, len(s.Segments[i].packets))}
		seg.packets = s.Segments[i].packets
		c.Segments[i] = seg
	}
	return c
}
