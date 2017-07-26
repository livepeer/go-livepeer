package vidplayer

import (
	"context"
	"errors"
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

var ErrNotFound = errors.New("NotFound")
var ErrRTMP = errors.New("RTMP Error")
var PlaylistWaittime = 6 * time.Second

//VidPlayer is the module that handles playing video. For now we only support RTMP and HLS play.
type VidPlayer struct {
	RtmpServer *joy4rtmp.Server
	VodPath    string
}

//HandleRTMPPlay is the handler when there is a RTMP request for a video. The source should write
//into the MuxCloser. The easiest way is through avutil.Copy.
func (s *VidPlayer) HandleRTMPPlay(getStream func(url *url.URL) (stream.Stream, error)) error {
	s.RtmpServer.HandlePlay = func(conn *joy4rtmp.Conn) {
		glog.Infof("LPMS got RTMP request @ %v", conn.URL)

		src, err := getStream(conn.URL)
		if err != nil {
			glog.Errorf("Error getting stream: %v", err)
			return
		}

		err = src.ReadRTMPFromStream(context.Background(), conn)
		if err != nil {
			glog.Errorf("Error copying RTMP stream: %v", err)
			return
		}

	}
	return nil
}

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

	glog.Infof("LPMS got HTTP request @ %v", r.URL.Path)
	w.Header().Set("Content-Type", "application/x-mpegURL")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Cache-Control", "max-age=5")

	if !strings.HasSuffix(r.URL.Path, ".m3u8") && !strings.HasSuffix(r.URL.Path, ".ts") {
		http.Error(w, "LPMS only accepts HLS requests over HTTP (m3u8, ts).", 500)
	}

	if strings.HasSuffix(r.URL.Path, ".m3u8") {
		//First, assume it's the master playlist
		var masterPl *m3u8.MasterPlaylist
		var mediaPl *m3u8.MediaPlaylist
		masterPl, err := getMasterPlaylist(r.URL)
		if masterPl == nil || err != nil {
			//Now try the media playlist
			mediaPl, err = getMediaPlaylist(r.URL)
			if err != nil {
				http.Error(w, "Error getting HLS playlist", 500)
				return
			}
		}

		if masterPl != nil {
			_, err = w.Write(masterPl.Encode().Bytes())
		} else if mediaPl != nil {
			_, err = w.Write(mediaPl.Encode().Bytes())
		}
		if err != nil {
			glog.Errorf("Error writing playlist to ResponseWriter: %v", err)
			return
		}

		return
	}

	if strings.HasSuffix(r.URL.Path, ".ts") {
		seg, err := getSegment(r.URL)
		if err != nil {
			glog.Errorf("Error getting segment %v: %v", r.URL, err)
			return
		}
		w.Header().Set("Content-Type", mime.TypeByExtension(path.Ext(r.URL.Path)))
		w.Header().Set("Access-Control-Allow-Origin", "*")
		_, err = w.Write(seg)
		if err != nil {
			glog.Errorf("Error writting HLS segment %v: %v", r.URL, err)
			return
		}
		return
	}

	http.Error(w, "Cannot find HTTP video resource: "+r.URL.String(), 500)
}

func handleVOD(url *url.URL, vodPath string, w http.ResponseWriter) error {
	if strings.HasSuffix(url.Path, ".m3u8") {
		plName := filepath.Join(vodPath, strings.Replace(url.Path, "/vod/", "", -1))
		dat, err := ioutil.ReadFile(plName)
		if err != nil {
			glog.Errorf("Cannot find file: %v", plName)
			return ErrNotFound
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
			return ErrNotFound
		}
		w.Header().Set("Content-Type", mime.TypeByExtension(path.Ext(url.Path)))
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Write(dat)
	}

	return nil
}
