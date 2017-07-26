package vidlistener

import (
	"context"
	"net/url"
	"os"
	"time"

	"github.com/golang/glog"
	"github.com/livepeer/lpms/segmenter"
	"github.com/livepeer/lpms/stream"
	joy4rtmp "github.com/nareix/joy4/format/rtmp"
)

var segOptions = segmenter.SegmenterOptions{SegLength: time.Second * 2}

type LocalStream struct {
	StreamID  string
	Timestamp int64
}

type VidListener struct {
	RtmpServer *joy4rtmp.Server
	FfmpegPath string
}

func (self *VidListener) HandleRTMPPublish(
	makeStreamID func(url *url.URL) (strmID string),
	gotStream func(url *url.URL, rtmpStrm *stream.VideoStream) error,
	endStream func(url *url.URL, rtmpStrm *stream.VideoStream) error) {

	self.RtmpServer.HandlePublish = func(conn *joy4rtmp.Conn) {
		glog.Infof("RTMP server got upstream: %v", conn.URL)

		s := stream.NewVideoStream(makeStreamID(conn.URL), stream.RTMP)
		ctx, cancel := context.WithCancel(context.Background())
		ec := make(chan error)
		go func() { ec <- s.WriteRTMPToStream(ctx, conn) }()
		glog.Infof("Listner rtmp addr: %v", &s)
		err := gotStream(conn.URL, s)
		if err != nil {
			glog.Errorf("Error RTMP gotStream handler: %v", err)
			cancel()
			return
		}

		select {
		case err := <-ec:
			endStream(conn.URL, s)
			glog.Errorf("Got error writing RTMP: %v", err)
			cancel()
		}
	}
}

//HandleRTMPPublish immediately turns the RTMP stream into segmented HLS, and writes it into a stream.  This code is only here as an example for segmenting a RTMP stream.
//It exposes getStreamID so the user can name the stream, and getStream so the user can keep track of all the streams.
func (self *VidListener) HandleRTMPPublish_OLD(
	getStreamID func(url *url.URL) (string, error),
	getStream func(url *url.URL) (rtmpStrm stream.Stream, hlsStrm stream.Stream, err error),
	endStream func(rtmpStrmID string, hlsStrmID string)) error {

	self.RtmpServer.HandlePublish = func(conn *joy4rtmp.Conn) {
		glog.Infof("RTMP server got upstream: %v", conn.URL)

		_, err := getStreamID(conn.URL)
		if err != nil {
			glog.Errorf("RTMP Stream Publish Error: %v", err)
			return
		}

		rs, hs, err := getStream(conn.URL)
		if err != nil {
			glog.Errorf("RTMP Publish couldn't get a destination stream for %v", conn.URL.Path)
			return
		}

		glog.Infof("Got RTMP Stream: %v", rs.GetStreamID())
		cew := make(chan error, 0)
		cs := make(chan error, 0)
		ctx, cancel := context.WithCancel(context.Background())
		glog.Infof("Writing RTMP to stream")
		go func() { cew <- rs.WriteRTMPToStream(ctx, conn) }()
		go func() { cs <- self.segmentStream(ctx, rs, hs) }()

		select {
		case err := <-cew:
			endStream(rs.GetStreamID(), hs.GetStreamID())
			glog.Infof("Final stream length: %v", rs.Len())
			glog.Error("Got error writing RTMP: ", err)
			cancel()
		case err := <-cs:
			glog.Errorf("Error segmenting, %v", err)
			cancel()
		}

	}
	return nil
}

//I think this should be done by the application instead the SDK.  It's unreasonable to expect the default LPMS behavior to segment the RTMP stream.
func (self *VidListener) segmentStream(ctx context.Context, rs stream.Stream, hs stream.Stream) error {
	// //Invoke Segmenter
	workDir, _ := os.Getwd()
	workDir = workDir + "/tmp"
	localRtmpUrl := "rtmp://localhost" + self.RtmpServer.Addr + "/stream/" + rs.GetStreamID()
	s := segmenter.NewFFMpegVideoSegmenter(workDir, hs.GetStreamID(), localRtmpUrl, segOptions.SegLength, self.FfmpegPath)
	c := make(chan error, 1)
	go func() { c <- s.RTMPToHLS(ctx, segOptions, true) }()

	go func() {
		c <- func() error {
			for {
				pl, err := s.PollPlaylist(ctx)
				if err != nil {
					glog.Errorf("Got error polling playlist: %v", err)
					return err
				}
				// glog.Infof("Writing pl: %v", pl)
				hs.WriteHLSPlaylistToStream(*pl.Data)
				select {
				case <-ctx.Done():
					return ctx.Err()
				default:
				}
			}
		}()
	}()

	go func() {
		c <- func() error {
			for {
				seg, err := s.PollSegment(ctx)
				if err != nil {
					return err
				}
				ss := stream.HLSSegment{SeqNo: seg.SeqNo, Data: seg.Data, Name: seg.Name, Duration: seg.Length.Seconds()}
				// glog.Infof("Writing stream: %v, duration:%v, len:%v", ss.Name, ss.Duration, len(seg.Data))
				hs.WriteHLSSegmentToStream(ss)
				select {
				case <-ctx.Done():
					return ctx.Err()
				default:
				}
			}
		}()
	}()

	select {
	case err := <-c:
		glog.Errorf("Error segmenting stream: %v", err)
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}
