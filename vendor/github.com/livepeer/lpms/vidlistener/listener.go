package vidlistener

import (
	"context"
	"net/url"
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
}

func (self *VidListener) HandleRTMPPublish(
	makeStreamID func(url *url.URL) (strmID string),
	gotStream func(url *url.URL, rtmpStrm stream.RTMPVideoStream) error,
	endStream func(url *url.URL, rtmpStrm stream.RTMPVideoStream) error) {

	if self.RtmpServer != nil {
		self.RtmpServer.HandlePublish = func(conn *joy4rtmp.Conn) {
			glog.V(2).Infof("RTMP server got upstream: %v", conn.URL)

			strmID := makeStreamID(conn.URL)
			if strmID == "" {
				return
			}
			s := stream.NewBasicRTMPVideoStream(strmID)
			ctx, cancel := context.WithCancel(context.Background())
			eof, err := s.WriteRTMPToStream(ctx, conn)
			if err != nil {
				cancel()
				return
			}

			err = gotStream(conn.URL, s)
			if err != nil {
				glog.Errorf("Error RTMP gotStream handler: %v", err)
				endStream(conn.URL, s)
				conn.Close()
				cancel()
				return
			}

			select {
			case <-eof:
				endStream(conn.URL, s)
				cancel()
			}
		}

	}
}
