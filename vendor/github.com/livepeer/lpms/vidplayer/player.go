package vidplayer

import (
	"context"
	"errors"
	"io"
	"io/ioutil"
	"mime"
	"net/http"
	"net/url"
	"path"
	"path/filepath"

	"strings"

	"time"

	"github.com/ericxtang/m3u8"
	"github.com/golang/glog"
	"github.com/livepeer/lpms/stream"
	joy4rtmp "github.com/nareix/joy4/format/rtmp"
)

var ErrNotFound = errors.New("ErrNotFound")
var ErrBadRequest = errors.New("ErrBadRequest")
var ErrTimeout = errors.New("ErrTimeout")
var ErrRTMP = errors.New("ErrRTMP")
var ErrHLS = errors.New("ErrHLS")
var PlaylistWaittime = 6 * time.Second

//VidPlayer is the module that handles playing video. For now we only support RTMP and HLS play.
type VidPlayer struct {
	RtmpServer      *joy4rtmp.Server
	rtmpPlayHandler func(url *url.URL) (stream.RTMPVideoStream, error)
	VodPath         string
}

func defaultRtmpPlayHandler(url *url.URL) (stream.RTMPVideoStream, error) { return nil, ErrRTMP }

//NewVidPlayer creates a new video player
func NewVidPlayer(rtmpS *joy4rtmp.Server, vodPath string) *VidPlayer {
	player := &VidPlayer{RtmpServer: rtmpS, VodPath: vodPath, rtmpPlayHandler: defaultRtmpPlayHandler}
	if rtmpS != nil {
		rtmpS.HandlePlay = player.rtmpServerHandlePlay()
	}
	return player
}

//HandleRTMPPlay is the handler when there is a RTMP request for a video. The source should write
//into the MuxCloser. The easiest way is through avutil.Copy.
func (s *VidPlayer) HandleRTMPPlay(getStream func(url *url.URL) (stream.RTMPVideoStream, error)) error {
	s.rtmpPlayHandler = getStream
	return nil
}

func (s *VidPlayer) rtmpServerHandlePlay() func(conn *joy4rtmp.Conn) {
	return func(conn *joy4rtmp.Conn) {
		glog.V(2).Infof("LPMS got RTMP request @ %v", conn.URL)

		src, err := s.rtmpPlayHandler(conn.URL)
		if err != nil {
			glog.Errorf("Error getting stream: %v", err)
			return
		}

		eof, err := src.ReadRTMPFromStream(context.Background(), conn)
		if err != nil {
			if err != io.EOF {
				glog.Errorf("Error copying RTMP stream: %v", err)
			}
		}
		select {
		case <-eof:
			conn.Close()
			return
		}
	}
}

//HandleHLSPlay is the handler when there is a HLS video request.  It supports both VOD and live streaming.
func (s *VidPlayer) HandleHLSPlay(
	getMasterPlaylist func(url *url.URL) (*m3u8.MasterPlaylist, error),
	getMediaPlaylist func(url *url.URL) (*m3u8.MediaPlaylist, error),
	getSegment func(url *url.URL) ([]byte, error)) {

	http.HandleFunc("/stream/", func(w http.ResponseWriter, r *http.Request) {
		handleLive(w, r, getMasterPlaylist, getMediaPlaylist, getSegment)
	})

	http.HandleFunc("/vod/", func(w http.ResponseWriter, r *http.Request) {
		handleVOD(r.URL, s.VodPath, w)
	})
}

func handleLive(w http.ResponseWriter, r *http.Request,
	getMasterPlaylist func(url *url.URL) (*m3u8.MasterPlaylist, error),
	getMediaPlaylist func(url *url.URL) (*m3u8.MediaPlaylist, error),
	getSegment func(url *url.URL) ([]byte, error)) {

	glog.V(4).Infof("LPMS got HTTP request @ %v", r.URL.Path)

	if !strings.HasSuffix(r.URL.Path, ".m3u8") && !strings.HasSuffix(r.URL.Path, ".ts") {
		http.Error(w, "LPMS only accepts HLS requests over HTTP (m3u8, ts).", 500)
	}
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Cache-Control", "max-age=5")

	if strings.HasSuffix(r.URL.Path, ".m3u8") {
		w.Header().Set("Content-Type", "application/x-mpegURL")

		//Could be a master playlist, or a media playlist
		var masterPl *m3u8.MasterPlaylist
		var mediaPl *m3u8.MediaPlaylist
		masterPl, err := getMasterPlaylist(r.URL) //Return ErrNotFound to indicate passing to mediaPlayList
		if err != nil {
			if err == ErrNotFound {
				//Do nothing here, because the call could be for mediaPlaylist
			} else if err == ErrTimeout {
				http.Error(w, "ErrTimeout", 408)
				return
			} else if err == ErrBadRequest {
				http.Error(w, "ErrBadRequest", 400)
				return
			} else {
				glog.Errorf("Error getting HLS master playlist: %v", err)
				http.Error(w, "Error getting master playlist", 500)
				return
			}
		}
		if masterPl != nil && len(masterPl.Variants) > 0 {
			w.Header().Set("Connection", "keep-alive")
			_, err = w.Write(masterPl.Encode().Bytes())
			// glog.Infof("%v", string(masterPl.Encode().Bytes()))
			return
		}

		mediaPl, err = getMediaPlaylist(r.URL)
		if err != nil {
			if err == ErrNotFound {
				http.Error(w, "ErrNotFound", 404)
			} else if err == ErrTimeout {
				http.Error(w, "ErrTimeout", 408)
			} else if err == ErrBadRequest {
				http.Error(w, "ErrBadRequest", 400)
			} else {
				http.Error(w, "Error getting HLS media playlist", 500)
			}
			return
		}

		w.Header().Set("Connection", "keep-alive")
		_, err = w.Write(mediaPl.Encode().Bytes())
		return
	}

	if strings.HasSuffix(r.URL.Path, ".ts") {
		seg, err := getSegment(r.URL)
		if err != nil {
			glog.Errorf("Error getting segment %v: %v", r.URL, err)
			http.Error(w, "Error getting segment", 500)
			return
		}
		w.Header().Set("Content-Type", mime.TypeByExtension(path.Ext(r.URL.Path)))
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Connection", "keep-alive")
		_, err = w.Write(seg)
		if err != nil {
			glog.Errorf("Error writting HLS segment %v: %v", r.URL, err)
			if err == ErrNotFound {
				http.Error(w, "ErrNotFound", 404)
			} else if err == ErrTimeout {
				http.Error(w, "ErrTimeout", 408)
			} else if err == ErrBadRequest {
				http.Error(w, "ErrBadRequest", 400)
			} else {
				http.Error(w, "Error writting segment", 500)
			}
			return
		}
		return
	}

	http.Error(w, "Cannot find HTTP video resource: "+r.URL.String(), 404)
}

func handleVOD(url *url.URL, vodPath string, w http.ResponseWriter) error {
	if strings.HasSuffix(url.Path, ".m3u8") {
		plName := filepath.Join(vodPath, strings.Replace(url.Path, "/vod/", "", -1))
		dat, err := ioutil.ReadFile(plName)
		if err != nil {
			glog.Errorf("Cannot find file: %v", plName)
			http.Error(w, "ErrNotFound", 404)
			return ErrHLS
		}
		w.Header().Set("Content-Type", mime.TypeByExtension(path.Ext(url.Path)))
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Cache-Control", "max-age=5")
		w.Write(dat)
	}

	if strings.Contains(url.Path, ".ts") {
		segName := filepath.Join(vodPath, strings.Replace(url.Path, "/vod/", "", -1))
		dat, err := ioutil.ReadFile(segName)
		if err != nil {
			glog.Errorf("Cannot find file: %v", segName)
			http.Error(w, "ErrNotFound", 404)
			return ErrHLS
		}
		w.Header().Set("Content-Type", mime.TypeByExtension(path.Ext(url.Path)))
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Write(dat)
	}

	http.Error(w, "Cannot find HTTP video resource: "+url.String(), 404)
	return nil
}
