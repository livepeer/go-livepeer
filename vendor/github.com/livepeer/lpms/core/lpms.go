//The RTMP server.  This will put up a RTMP endpoint when starting up Swarm.
//To integrate with LPMS means your code will become the source / destination of the media server.
//This RTMP endpoint is mainly used for video upload.  The expected url is rtmp://localhost:port/livepeer/stream
package core

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/golang/glog"
	"github.com/livepeer/lpms/ffmpeg"
	"github.com/livepeer/lpms/segmenter"
	"github.com/livepeer/lpms/stream"
	"github.com/livepeer/lpms/vidlistener"
	"github.com/livepeer/lpms/vidplayer"
	"github.com/livepeer/m3u8"

	joy4rtmp "github.com/livepeer/joy4/format/rtmp"
)

var RetryCount = 3
var SegmenterRetryWait = 500 * time.Millisecond

// LPMS struct is the container of all the LPMS server related tools.
type LPMS struct {
	// rtmpServer *joy4rtmp.Server
	vidPlayer   *vidplayer.VidPlayer
	vidListener *vidlistener.VidListener
	workDir     string
	httpAddr    string
	rtmpAddr    string
}

type transcodeReq struct {
	Formats  []string
	Bitrates []string
	Codecin  string
	Codecout []string
	StreamID string
}

type LPMSOpts struct {
	RtmpAddr     string
	RtmpDisabled bool
	HttpAddr     string
	HttpDisabled bool
	VodPath      string
	WorkDir      string

	// Pass in a custom HTTP mux.
	// Useful if a non-default mux needs to be used.
	// The caller is responsible for setting up the listener
	// on the mux; LPMS won't initialize it.
	// If set, HttpPort and HttpDisabled are ignored.
	HttpMux *http.ServeMux
}

func defaultLPMSOpts(opts *LPMSOpts) {
	if opts.RtmpAddr == "" {
		opts.RtmpAddr = "127.0.0.1:1935"
	}
	if opts.HttpAddr == "" {
		opts.HttpAddr = "127.0.0.1:7935"
	}
}

//New creates a new LPMS server object.  It really just brokers everything to the components.
func New(opts *LPMSOpts) *LPMS {
	defaultLPMSOpts(opts)
	var rtmpServer *joy4rtmp.Server
	if !opts.RtmpDisabled {
		rtmpServer = &joy4rtmp.Server{Addr: opts.RtmpAddr}
	}
	var httpAddr string
	if !opts.HttpDisabled && opts.HttpMux == nil {
		httpAddr = opts.HttpAddr
	}
	player := vidplayer.NewVidPlayer(rtmpServer, opts.VodPath, opts.HttpMux)
	listener := &vidlistener.VidListener{RtmpServer: rtmpServer}
	return &LPMS{vidPlayer: player, vidListener: listener, workDir: opts.WorkDir, rtmpAddr: opts.RtmpAddr, httpAddr: httpAddr}
}

//Start starts the rtmp and http servers, and initializes ffmpeg
func (l *LPMS) Start(ctx context.Context) error {
	ec := make(chan error, 1)
	ffmpeg.InitFFmpeg()
	if l.vidListener.RtmpServer != nil {
		go func() {
			glog.V(4).Infof("LPMS Server listening on rtmp://%v", l.vidListener.RtmpServer.Addr)
			ec <- l.vidListener.RtmpServer.ListenAndServe()
		}()
	}
	startHTTP := l.httpAddr != ""
	if startHTTP {
		go func() {
			glog.V(4).Infof("HTTP Server listening on http://%v", l.httpAddr)
			ec <- http.ListenAndServe(l.httpAddr, nil)
		}()
	}

	if l.vidListener.RtmpServer != nil || startHTTP {
		select {
		case err := <-ec:
			glog.Errorf("LPMS Server Error: %v.  Quitting...", err)
			return err
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return nil
}

//HandleRTMPPublish offload to the video listener.  To understand how it works, look at videoListener.HandleRTMPPublish.
func (l *LPMS) HandleRTMPPublish(
	makeStreamID func(url *url.URL) (strmID stream.AppData),
	gotStream func(url *url.URL, rtmpStrm stream.RTMPVideoStream) (err error),
	endStream func(url *url.URL, rtmpStrm stream.RTMPVideoStream) error) {

	l.vidListener.HandleRTMPPublish(makeStreamID, gotStream, endStream)
}

//HandleRTMPPlay offload to the video player
func (l *LPMS) HandleRTMPPlay(getStream func(url *url.URL) (stream.RTMPVideoStream, error)) error {
	return l.vidPlayer.HandleRTMPPlay(getStream)
}

//HandleHLSPlay offload to the video player
func (l *LPMS) HandleHLSPlay(
	getMasterPlaylist func(url *url.URL) (*m3u8.MasterPlaylist, error),
	getMediaPlaylist func(url *url.URL) (*m3u8.MediaPlaylist, error),
	getSegment func(url *url.URL) ([]byte, error)) {

	l.vidPlayer.HandleHLSPlay(getMasterPlaylist, getMediaPlaylist, getSegment)
}

//SegmentRTMPToHLS takes a rtmp stream and re-packages it into a HLS stream with the specified segmenter options
func (l *LPMS) SegmentRTMPToHLS(ctx context.Context, rs stream.RTMPVideoStream, hs stream.HLSVideoStream, segOptions segmenter.SegmenterOptions) error {
	// set localhost if necessary. Check more problematic addrs? [::] ?
	rtmpAddr := l.rtmpAddr
	if strings.HasPrefix(rtmpAddr, "0.0.0.0") {
		rtmpAddr = "127.0.0.1" + rtmpAddr[len("0.0.0.0"):]
	}
	localRtmpUrl := "rtmp://" + rtmpAddr + "/stream/" + rs.GetStreamID()

	glog.V(4).Infof("Segment RTMP Req: %v", localRtmpUrl)

	//Invoke Segmenter
	s := segmenter.NewFFMpegVideoSegmenter(l.workDir, hs.GetStreamID(), localRtmpUrl, segOptions)
	c := make(chan error, 1)
	ffmpegCtx, ffmpegCancel := context.WithCancel(context.Background())
	go func() { c <- rtmpToHLS(s, ffmpegCtx, true) }()

	//Kick off go routine to write HLS segments
	segCtx, segCancel := context.WithCancel(context.Background())
	go func() {
		c <- func() error {
			var seg *segmenter.VideoSegment
			var err error
			for {
				if hs == nil {
					glog.Errorf("HLS Stream is nil")
					return segmenter.ErrSegmenter
				}

				for i := 1; i <= RetryCount; i++ {
					seg, err = s.PollSegment(segCtx)
					if err == nil || err == context.Canceled || err == context.DeadlineExceeded {
						break
					} else if i < RetryCount {
						glog.Errorf("Error polling Segment: %v, Retrying", err)
						time.Sleep(SegmenterRetryWait)
					}
				}

				if err != nil {
					return err
				}

				ss := stream.HLSSegment{SeqNo: seg.SeqNo, Data: seg.Data, Name: seg.Name, Duration: seg.Length.Seconds()}
				// glog.Infof("Writing stream: %v, duration:%v, len:%v", ss.Name, ss.Duration, len(seg.Data))
				if err = hs.AddHLSSegment(&ss); err != nil {
					glog.Errorf("Error adding segment: %v", err)
				}
				select {
				case <-segCtx.Done():
					return segCtx.Err()
				default:
				}
			}
		}()
	}()

	select {
	case err := <-c:
		if err != nil && err != context.Canceled {
			glog.Errorf("Error segmenting stream: %v", err)
		}
		ffmpegCancel()
		segCancel()
		return err
	case <-ctx.Done():
		ffmpegCancel()
		segCancel()
		return nil
	}
}

func rtmpToHLS(s segmenter.VideoSegmenter, ctx context.Context, cleanup bool) error {
	var err error
	for i := 1; i <= RetryCount; i++ {
		err = s.RTMPToHLS(ctx, cleanup)
		if err == nil {
			break
		} else if i < RetryCount {
			glog.Errorf("Error Invoking Segmenter: %v, Retrying", err)
			time.Sleep(SegmenterRetryWait)
		}
	}
	return err
}
