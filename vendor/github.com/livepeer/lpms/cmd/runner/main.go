package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/ericxtang/m3u8"
	"github.com/golang/glog"
	"github.com/livepeer/lpms/core"
	"github.com/livepeer/lpms/stream"
)

type StreamDB struct {
	db map[string]stream.Stream
}

type BufferDB struct {
	db map[string]*stream.HLSBuffer
}

//Trivial method for getting the id
func getStreamID(url *url.URL) string {
	if strings.HasSuffix(url.Path, "m3u8") {
		return "hlsStrmID"
	} else {
		return "rtmpStrmID"
	}
}

func getHLSSegmentName(url *url.URL) string {
	var segName string
	regex, _ := regexp.Compile("\\/stream\\/.*\\.ts")
	match := regex.FindString(url.Path)
	if match != "" {
		segName = strings.Replace(match, "/stream/", "", -1)
	}
	return segName
}

func main() {
	flag.Set("logtostderr", "true")
	flag.Parse()

	lpms := core.New("1935", "8000", "", "")
	streamDB := &StreamDB{db: make(map[string]stream.Stream)}
	bufferDB := &BufferDB{db: make(map[string]*stream.HLSBuffer)}

	lpms.HandleRTMPPublish(
		//makeStreamID
		func(url *url.URL) (strmID string) {
			//Give the stream a name
			return getStreamID(url)
		},
		//gotStream
		func(url *url.URL, rtmpStrm *stream.VideoStream) (err error) {
			//Store the stream
			streamDB.db[rtmpStrm.GetStreamID()] = rtmpStrm
			return nil
		},
		//endStream
		func(url *url.URL, rtmpStrm *stream.VideoStream) error {
			//Remove the stream
			delete(streamDB.db, rtmpStrm.GetStreamID())
			return nil
		})

	lpms.HandleHLSPlay(
		//getMasterPlaylist
		func(url *url.URL) (*m3u8.MasterPlaylist, error) {
			//No need to return a masterlist unless we are doing ABS
			return nil, nil
		},
		//getMediaPlaylist
		func(url *url.URL) (*m3u8.MediaPlaylist, error) {
			buf, ok := bufferDB.db[getStreamID(url)]
			if !ok {
				return nil, fmt.Errorf("Cannot find video")
			}
			return buf.LatestPlaylist()
		},
		//getSegment
		func(url *url.URL) ([]byte, error) {
			buf, ok := bufferDB.db[getStreamID(url)]
			if !ok {
				return nil, fmt.Errorf("Cannot find video")
			}
			return buf.WaitAndPopSegment(context.Background(), getHLSSegmentName(url))
		})

	lpms.HandleRTMPPlay(
		//getStream
		func(url *url.URL) (stream.Stream, error) {
			glog.Infof("Got req: ", url.Path)
			strmID := getStreamID(url)
			src := streamDB.db[strmID]

			return src, nil
		})

	//Helper function to print out all the streams
	http.HandleFunc("/streams", func(w http.ResponseWriter, r *http.Request) {
		streams := []string{}

		for k, _ := range streamDB.db {
			streams = append(streams, k)
		}

		if len(streams) == 0 {
			w.Write([]byte("no streams"))
			return
		}
		str := strings.Join(streams, ",")
		w.Write([]byte(str))
	})

	lpms.Start(context.Background())
}
