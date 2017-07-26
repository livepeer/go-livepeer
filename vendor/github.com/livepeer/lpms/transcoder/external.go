package transcoder

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/ericxtang/m3u8"
	"github.com/golang/glog"
	"github.com/livepeer/lpms/stream"
	"github.com/nareix/joy4/av"
	joy4rtmp "github.com/nareix/joy4/format/rtmp"
	cmap "github.com/orcaman/concurrent-map"
)

var ErrTranscoderConnRefused = errors.New("Connection Refused for Local External Transcoder")
var ErrHLSDownloadTimeout = errors.New("HLS Download Timeout")
var ErrUnsupportFormat = errors.New("Unsupported Format")
var ErrNotFound = errors.New("Not Found")

type ExternalTranscoder struct {
	localSRSRTMPPort string
	localSRSHTTPPort string
	streamID         string
	downloader       HLSDownloader

	//TODO: Keep track of local SRS instance
}

func New(rtmpPort string, srsHTTPPort string, streamID string) *ExternalTranscoder {
	m := cmap.New()
	d := SRSHLSDownloader{cache: &m, localEndpoint: "http://localhost:" + srsHTTPPort + "/stream/", streamID: streamID, startDownloadWaitTime: time.Second * 20, hlsIntervalWaitTime: time.Second}
	return &ExternalTranscoder{localSRSRTMPPort: rtmpPort, localSRSHTTPPort: srsHTTPPort, streamID: streamID, downloader: d}
}

func (et *ExternalTranscoder) StartService() {
	//Start SRS
}

//LocalSRSUploadMux Convenience method to get a mux
func (et *ExternalTranscoder) LocalSRSUploadMux() (av.MuxCloser, error) {
	url := "rtmp://localhost:" + et.localSRSRTMPPort + "/stream/" + et.streamID
	glog.Infof("SRS Upload path: %v", url)
	rtmpMux, err := joy4rtmp.Dial(url)
	if err != nil {
		glog.Errorf("Transcoder RTMP Stream Publish Error: %v.  Make sure you have started your local SRS instance correctly.", err)
		return nil, err
	}
	return rtmpMux, nil
}

//StartUpload takes a io.Stream of RTMP stream, and loads it into a local RTMP endpoint.  The streamID will be used as the streaming endpoint.
//So if you want to create a new stream, make sure to do that before passing in the stream.
func (et *ExternalTranscoder) StartUpload(ctx context.Context, rtmpMux av.MuxCloser, src stream.Stream) error {
	upErrC := make(chan error, 1)

	go func() { upErrC <- src.ReadRTMPFromStream(ctx, rtmpMux) }()

	select {
	case err := <-upErrC:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

//StartDownload pushes hls playlists and segments into the stream as they become available from the transcoder.
func (et *ExternalTranscoder) StartDownload(ctx context.Context, hlsMux stream.Stream) error {
	pc := make(chan *m3u8.MediaPlaylist)
	sc := make(chan *stream.HLSSegment)
	ec := make(chan error)
	go func() { ec <- et.downloader.Download(pc, sc) }()
	for {
		select {
		case pl := <-pc:
			err := hlsMux.WriteHLSPlaylistToStream(*pl)
			if err != nil {
				return err
			}
		case seg := <-sc:
			err := hlsMux.WriteHLSSegmentToStream(*seg)
			if err != nil {
				return err
			}
		case err := <-ec:
			glog.Errorf("HLS Download Error: %v", err)
			return err
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

//HLSDownloader doesn't take m3u8.MediaSegment because it doesn't contain the actual data
type HLSDownloader interface {
	Download(pc chan *m3u8.MediaPlaylist, sc chan *stream.HLSSegment) error
}

type SRSHLSDownloader struct {
	cache                 *cmap.ConcurrentMap
	localEndpoint         string
	streamID              string
	startDownloadWaitTime time.Duration
	hlsIntervalWaitTime   time.Duration
}

//Download only pushes a playlist onto the channel when there is a new segment in it.
func (d SRSHLSDownloader) Download(pc chan *m3u8.MediaPlaylist, sc chan *stream.HLSSegment) error {
	before := time.Now()
	plURL := d.localEndpoint + d.streamID + ".m3u8"
	glog.Infof("SRS Playlist Download Path: ", plURL)

	for {
		pl, errp := DownloadPlaylist(plURL)
		if errp == ErrNotFound && time.Since(before) < d.startDownloadWaitTime { //only sleep wait for until the start download time
			time.Sleep(time.Second * 5)
			continue
		} else if errp != nil {
			glog.Errorf("Transcoder HLS Playlist Download Error: %v", errp)
			return errp
		}

		sendpl := false
		for _, seginfo := range pl.Segments {
			if seginfo == nil {
				continue
			}
			if _, found := d.cache.Get(seginfo.URI); found == false {
				seg, errs := DownloadSegment(d.localEndpoint, seginfo)
				if errs != nil {
					glog.Errorf("Transcoder HLS Segment Download Error: %v", errp)
					return errs
				}
				sc <- &stream.HLSSegment{Name: seginfo.URI, Data: seg}

				d.cache.Set(seginfo.URI, true)
				sendpl = true
			}
		}

		if sendpl {
			pc <- pl
		}

		time.Sleep(d.hlsIntervalWaitTime)
	}
}

func DownloadSegment(endpoint string, seginfo *m3u8.MediaSegment) ([]byte, error) {
	req, err := http.NewRequest("GET", endpoint+seginfo.URI, nil)
	if err != nil {
		glog.Errorf("Transcoder HLS Segment Download Error: %v", err)
		return nil, err
	}
	client := http.Client{}

	resp, err := client.Do(req)
	if err != nil {
		glog.Errorf("Transcoder HLS Segment Download Error: %v", err)
		return nil, err
	}

	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, resp.Body)
	if err != nil {
		glog.Errorf("Segment Download Error: %v", err)
		return nil, err
	}

	return buf.Bytes(), nil
}

func DownloadPlaylist(endpointUrl string) (*m3u8.MediaPlaylist, error) {
	req, err := http.NewRequest("GET", endpointUrl, nil)
	if err != nil {
		glog.Errorf("Transcoder HLS Download Error: %v", err)
		return nil, err
	}
	client := http.Client{}

	resp, err := client.Do(req)
	if err != nil {
		glog.Errorf("Transcoder HLS Download Error: %v", err)
		return nil, err
	}

	playlist, listType, err := m3u8.DecodeFrom(resp.Body, true)

	if playlist == nil {
		return nil, ErrNotFound
	}

	if listType == m3u8.MEDIA {
		mpl := playlist.(*m3u8.MediaPlaylist)
		return mpl, nil
	}

	return nil, ErrUnsupportFormat
}
