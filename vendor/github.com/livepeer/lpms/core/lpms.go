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

	"github.com/ericxtang/m3u8"
	"github.com/golang/glog"
	"github.com/livepeer/lpms/ffmpeg"
	"github.com/livepeer/lpms/segmenter"
	"github.com/livepeer/lpms/stream"
	"github.com/livepeer/lpms/vidlistener"
	"github.com/livepeer/lpms/vidplayer"

	joy4rtmp "github.com/nareix/joy4/format/rtmp"
)

var RetryCount = 3
var SegmenterRetryWait = 500 * time.Millisecond

type LPMS struct {
	rtmpServer *joy4rtmp.Server
	vidPlayer  *vidplayer.VidPlayer
	vidListen  *vidlistener.VidListener
	workDir    string
	httpAddr   string
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
	return &LPMS{rtmpServer: rtmpServer, vidPlayer: player, vidListen: listener, workDir: opts.WorkDir, httpAddr: httpAddr}
}

//Start starts the rtmp and http servers, and initializes ffmpeg
func (l *LPMS) Start(ctx context.Context) error {
	ec := make(chan error, 1)
	ffmpeg.InitFFmpeg()
	if l.rtmpServer != nil {
		go func() {
			glog.V(4).Infof("LPMS Server listening on rtmp://%v", l.rtmpServer.Addr)
			ec <- l.rtmpServer.ListenAndServe()
		}()
	}
	startHTTP := l.httpAddr != ""
	if startHTTP {
		go func() {
			glog.V(4).Infof("HTTP Server listening on http://%v", l.httpAddr)
			ec <- http.ListenAndServe(l.httpAddr, nil)
		}()
	}

	if l.rtmpServer != nil || startHTTP {
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

//HandleRTMPPublish offload to the video listener
func (l *LPMS) HandleRTMPPublish(
	makeStreamID func(url *url.URL) (strmID string),
	gotStream func(url *url.URL, rtmpStrm stream.RTMPVideoStream) (err error),
	endStream func(url *url.URL, rtmpStrm stream.RTMPVideoStream) error) {

	l.vidListen.HandleRTMPPublish(makeStreamID, gotStream, endStream)
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
	rtmpAddr := l.rtmpServer.Addr
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

// //HandleTranscode kicks off a transcoding process, keeps a local HLS buffer, and returns the new stream ID.
// //stream is the video stream you want to be transcoded.  getNewStreamID gives you a way to name the transcoded stream.
// func (l *LPMS) HandleTranscode(getInStream func(ctx context.Context, streamID string) (stream.Stream, error), getOutStream func(ctx context.Context, streamID string) (stream.Stream, error)) {
// 	http.HandleFunc("/transcode", func(w http.ResponseWriter, r *http.Request) {
// 		ctx, _ := context.WithCancel(context.Background())

// 		//parse transcode request
// 		decoder := json.NewDecoder(r.Body)
// 		var tReq transcodeReq
// 		if r.Body == nil {
// 			http.Error(w, "Please send a request body", 400)
// 			return
// 		}
// 		err := decoder.Decode(&tReq)
// 		if err != nil {
// 			http.Error(w, err.Error(), 400)
// 			return
// 		}

// 		//Get the RTMP Stream
// 		inStream, err := getInStream(ctx, tReq.StreamID)
// 		if err != nil {
// 			http.Error(w, err.Error(), 400)
// 			return
// 		}

// 		//Get the HLS Stream
// 		newStream, err := getOutStream(ctx, tReq.StreamID)
// 		if err != nil {
// 			http.Error(w, err.Error(), 400)
// 		}

// 		ec := make(chan error, 1)
// 		go func() { ec <- l.doTranscoding(ctx, inStream, newStream) }()

// 		w.Write([]byte("New Stream: " + newStream.GetStreamID()))
// 	})
// }

// func (l *LPMS) doTranscoding(ctx context.Context, inStream stream.Stream, newStream stream.Stream) error {
// 	t := transcoder.New(l.srsRTMPPort, l.srsHTTPPort, newStream.GetStreamID())
// 	//Should kick off a goroutine for this, so we can return the new streamID rightaway.

// 	tranMux, err := t.LocalSRSUploadMux()
// 	if err != nil {
// 		return err
// 		// http.Error(w, "Cannot create a connection with local transcoder", 400)
// 	}

// 	uec := make(chan error, 1)
// 	go func() { uec <- t.StartUpload(ctx, tranMux, inStream) }()
// 	dec := make(chan error, 1)
// 	go func() { dec <- t.StartDownload(ctx, newStream) }()

// 	select {
// 	case err := <-uec:
// 		return err
// 		// http.Error(w, "Cannot upload stream to transcoder: "+err.Error(), 400)
// 	case err := <-dec:
// 		return err
// 		// http.Error(w, "Cannot download stream from transcoder: "+err.Error(), 400)
// 	}

// }
