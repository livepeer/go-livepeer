//The RTMP server.  This will put up a RTMP endpoint when starting up Swarm.
//To integrate with LPMS means your code will become the source / destination of the media server.
//This RTMP endpoint is mainly used for video upload.  The expected url is rtmp://localhost:port/livepeer/stream
package core

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"os"

	"github.com/ericxtang/m3u8"
	"github.com/golang/glog"
	"github.com/livepeer/lpms/segmenter"
	"github.com/livepeer/lpms/stream"
	"github.com/livepeer/lpms/transcoder"
	"github.com/livepeer/lpms/vidlistener"
	"github.com/livepeer/lpms/vidplayer"

	joy4rtmp "github.com/nareix/joy4/format/rtmp"
)

type LPMS struct {
	rtmpServer  *joy4rtmp.Server
	vidPlayer   *vidplayer.VidPlayer
	vidListen   *vidlistener.VidListener
	httpPort    string
	srsRTMPPort string
	srsHTTPPort string
	ffmpegPath  string
}

type transcodeReq struct {
	Formats  []string
	Bitrates []string
	Codecin  string
	Codecout []string
	StreamID string
}

//New creates a new LPMS server object.  It really just brokers everything to the components.
func New(rtmpPort, httpPort, ffmpegPath, vodPath string) *LPMS {
	server := &joy4rtmp.Server{Addr: (":" + rtmpPort)}
	player := &vidplayer.VidPlayer{RtmpServer: server, VodPath: vodPath}
	listener := &vidlistener.VidListener{RtmpServer: server, FfmpegPath: ffmpegPath}
	return &LPMS{rtmpServer: server, vidPlayer: player, vidListen: listener, httpPort: httpPort, ffmpegPath: ffmpegPath}
}

//Start starts the rtmp and http server
func (l *LPMS) Start(ctx context.Context) error {
	ec := make(chan error, 1)
	go func() {
		glog.Infof("Starting LPMS Server at :%v", l.rtmpServer.Addr)
		ec <- l.rtmpServer.ListenAndServe()
	}()
	go func() {
		glog.Infof("Starting HTTP Server at :%v", l.httpPort)
		ec <- http.ListenAndServe(":"+l.httpPort, nil)
	}()

	select {
	case err := <-ec:
		glog.Infof("LPMS Server Error: %v.  Quitting...", err)
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

//HandleRTMPPublish offload to the video listener
func (l *LPMS) HandleRTMPPublish(
	makeStreamID func(url *url.URL) (strmID string),
	gotStream func(url *url.URL, rtmpStrm *stream.VideoStream) (err error),
	endStream func(url *url.URL, rtmpStrm *stream.VideoStream) error) {

	l.vidListen.HandleRTMPPublish(makeStreamID, gotStream, endStream)
}

//HandleRTMPPlay offload to the video player
func (l *LPMS) HandleRTMPPlay(getStream func(url *url.URL) (stream.Stream, error)) error {
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
func (l *LPMS) SegmentRTMPToHLS(ctx context.Context, rs stream.Stream, hs stream.Stream, segOptions segmenter.SegmenterOptions) error {
	//Invoke Segmenter
	workDir, _ := os.Getwd()
	workDir = workDir + "/tmp"
	localRtmpUrl := "rtmp://localhost" + l.rtmpServer.Addr + "/stream/" + rs.GetStreamID()
	glog.Infof("Segment RTMP Req: %v", localRtmpUrl)

	s := segmenter.NewFFMpegVideoSegmenter(workDir, hs.GetStreamID(), localRtmpUrl, segOptions.SegLength, l.ffmpegPath)
	c := make(chan error, 1)
	ffmpegCtx, ffmpegCancel := context.WithCancel(context.Background())
	go func() { c <- s.RTMPToHLS(ffmpegCtx, segOptions, true) }()

	//Kick off go routine to write HLS playlist
	plCtx, plCancel := context.WithCancel(context.Background())
	go func() {
		c <- func() error {
			for {
				pl, err := s.PollPlaylist(plCtx)
				if err != nil {
					glog.Errorf("Got error polling playlist: %v", err)
					return err
				}
				// glog.Infof("Writing pl: %v", pl)
				hs.WriteHLSPlaylistToStream(*pl.Data)
				select {
				case <-plCtx.Done():
					return plCtx.Err()
				default:
				}
			}
		}()
	}()

	//Kick off go routine to write HLS segments
	segCtx, segCancel := context.WithCancel(context.Background())
	go func() {
		c <- func() error {
			for {
				seg, err := s.PollSegment(segCtx)
				if err != nil {
					return err
				}
				ss := stream.HLSSegment{SeqNo: seg.SeqNo, Data: seg.Data, Name: seg.Name, Duration: seg.Length.Seconds()}
				// glog.Infof("Writing stream: %v, duration:%v, len:%v", ss.Name, ss.Duration, len(seg.Data))
				hs.WriteHLSSegmentToStream(ss)
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
		glog.Errorf("Error segmenting stream: %v", err)
		ffmpegCancel()
		plCancel()
		segCancel()
		return err
	case <-ctx.Done():
		ffmpegCancel()
		plCancel()
		segCancel()
		return ctx.Err()
	}
}

//HandleTranscode kicks off a transcoding process, keeps a local HLS buffer, and returns the new stream ID.
//stream is the video stream you want to be transcoded.  getNewStreamID gives you a way to name the transcoded stream.
func (l *LPMS) HandleTranscode(getInStream func(ctx context.Context, streamID string) (stream.Stream, error), getOutStream func(ctx context.Context, streamID string) (stream.Stream, error)) {
	http.HandleFunc("/transcode", func(w http.ResponseWriter, r *http.Request) {
		ctx, _ := context.WithCancel(context.Background())
		// defer cancel()

		//parse transcode request
		decoder := json.NewDecoder(r.Body)
		var tReq transcodeReq
		if r.Body == nil {
			http.Error(w, "Please send a request body", 400)
			return
		}
		err := decoder.Decode(&tReq)
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}

		//Get the RTMP Stream
		inStream, err := getInStream(ctx, tReq.StreamID)
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}

		//Get the HLS Stream
		newStream, err := getOutStream(ctx, tReq.StreamID)
		if err != nil {
			http.Error(w, err.Error(), 400)
		}

		ec := make(chan error, 1)
		go func() { ec <- l.doTranscoding(ctx, inStream, newStream) }()

		w.Write([]byte("New Stream: " + newStream.GetStreamID()))
	})
}

func (l *LPMS) doTranscoding(ctx context.Context, inStream stream.Stream, newStream stream.Stream) error {
	t := transcoder.New(l.srsRTMPPort, l.srsHTTPPort, newStream.GetStreamID())
	//Should kick off a goroutine for this, so we can return the new streamID rightaway.

	tranMux, err := t.LocalSRSUploadMux()
	if err != nil {
		return err
		// http.Error(w, "Cannot create a connection with local transcoder", 400)
	}

	uec := make(chan error, 1)
	go func() { uec <- t.StartUpload(ctx, tranMux, inStream) }()
	dec := make(chan error, 1)
	go func() { dec <- t.StartDownload(ctx, newStream) }()

	select {
	case err := <-uec:
		return err
		// http.Error(w, "Cannot upload stream to transcoder: "+err.Error(), 400)
	case err := <-dec:
		return err
		// http.Error(w, "Cannot download stream from transcoder: "+err.Error(), 400)
	}

}
