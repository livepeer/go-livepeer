/*
The Example Media Server.  It takes an RTMP stream, segments it into a HLS stream, and transcodes it so it's available for Adaptive Bitrate Streaming.
*/
package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/livepeer/lpms/transcoder"

	"github.com/ericxtang/m3u8"
	"github.com/golang/glog"
	"github.com/livepeer/lpms/core"
	"github.com/livepeer/lpms/ffmpeg"
	"github.com/livepeer/lpms/segmenter"
	"github.com/livepeer/lpms/stream"
)

var HLSWaitTime = time.Second * 10

func randString(n int) string {
	rand.Seed(time.Now().UnixNano())
	x := make([]byte, n, n)
	for i := 0; i < len(x); i++ {
		x[i] = byte(rand.Uint32())
	}
	return fmt.Sprintf("%x", x)
}

func parseStreamID(reqPath string) string {
	var strmID string
	regex, _ := regexp.Compile("\\/stream\\/([[:alpha:]]|\\d)*")
	match := regex.FindString(reqPath)
	if match != "" {
		strmID = strings.Replace(match, "/stream/", "", -1)
	}
	return strmID
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

	dir, err := os.Getwd()
	if err != nil {
		glog.Infof("Error getting work directory: %v", err)
	}
	glog.Infof("Settig working directory %v", fmt.Sprintf("%v/.tmp", dir))
	lpms := core.New(&core.LPMSOpts{WorkDir: fmt.Sprintf("%v/.tmp", dir)})

	//Streams needed for transcoding:
	var rtmpStrm stream.RTMPVideoStream
	var hlsStrm stream.HLSVideoStream
	var manifest stream.HLSVideoManifest
	var cancelSeg context.CancelFunc

	lpms.HandleRTMPPublish(
		//makeStreamID (give the stream an ID)
		func(url *url.URL) (strmID string) {
			return randString(10)
		},

		//gotStream
		func(url *url.URL, rs stream.RTMPVideoStream) (err error) {
			//Store the stream
			glog.Infof("Got RTMP stream: %v", rs.GetStreamID())
			rtmpStrm = rs

			// //Segment the video into HLS (If we need multiple outlets for the HLS stream, we'd need to create a buffer.  But here we only have one outlet for the transcoder)
			hlsStrm = stream.NewBasicHLSVideoStream(randString(10), 3)
			// var subscriber func(*stream.HLSSegment, bool)
			// subscriber, err = transcode(hlsStrm)
			// if err != nil {
			// 	glog.Errorf("Error transcoding: %v", err)
			// }
			// hlsStrm.SetSubscriber(subscriber)
			// glog.Infof("After set subscriber")
			opt := segmenter.SegmenterOptions{SegLength: 8 * time.Second}
			var ctx context.Context
			ctx, cancelSeg = context.WithCancel(context.Background())

			//Kick off FFMpeg to create segments
			go func() {
				if err := lpms.SegmentRTMPToHLS(ctx, rtmpStrm, hlsStrm, opt); err != nil {
					glog.Errorf("Error segmenting RTMP video stream: %v", err)
				}
			}()
			glog.Infof("HLS StreamID: %v", hlsStrm.GetStreamID())

			// mid := randString(10)
			// manifest = stream.NewBasicHLSVideoManifest(mid)
			// pl, _ := hlsStrm.GetStreamPlaylist()
			// variant := &m3u8.Variant{URI: fmt.Sprintf("%v.m3u8", mid), Chunklist: pl, VariantParams: m3u8.VariantParams{}}
			// manifest.AddVideoStream(hlsStrm, variant)
			return nil
		},
		//endStream
		func(url *url.URL, rtmpStrm stream.RTMPVideoStream) error {
			glog.Infof("Ending stream for %v", hlsStrm.GetStreamID())
			//Remove the stream
			cancelSeg()
			rtmpStrm = nil
			hlsStrm = nil
			return nil
		})

	lpms.HandleHLSPlay(
		//getMasterPlaylist
		func(url *url.URL) (*m3u8.MasterPlaylist, error) {
			if parseStreamID(url.Path) == "transcoded" && hlsStrm != nil {
				mpl, err := manifest.GetManifest()
				if err != nil {
					glog.Errorf("Error getting master playlist: %v", err)
					return nil, err
				}
				glog.Infof("Master Playlist: %v", mpl.String())
				return mpl, nil
			}
			return nil, nil
		},
		//getMediaPlaylist
		func(url *url.URL) (*m3u8.MediaPlaylist, error) {
			if nil == hlsStrm {
				return nil, fmt.Errorf("No stream available")
			}
			//Wait for the HLSBuffer gets populated, get the playlist from the buffer, and return it.
			start := time.Now()
			for time.Since(start) < HLSWaitTime {
				pl, err := hlsStrm.GetStreamPlaylist()
				if err != nil || pl == nil || pl.Segments == nil || len(pl.Segments) <= 0 || pl.Segments[0] == nil || pl.Segments[0].URI == "" {
					if err == stream.ErrEOF {
						return nil, err
					}

					time.Sleep(2 * time.Second)
					continue
				} else {
					return pl, nil
				}
			}
			return nil, fmt.Errorf("Error getting playlist")
		},
		//getSegment
		func(url *url.URL) ([]byte, error) {
			seg, err := hlsStrm.GetHLSSegment(getHLSSegmentName(url))
			if err != nil {
				glog.Errorf("Error getting segment: %v", err)
				return nil, err
			}
			return seg.Data, nil
		})

	lpms.HandleRTMPPlay(
		//getStream
		func(url *url.URL) (stream.RTMPVideoStream, error) {
			glog.Infof("Got req: ", url.Path)
			if rtmpStrm != nil {
				strmID := parseStreamID(url.Path)
				if strmID == rtmpStrm.GetStreamID() {
					return rtmpStrm, nil
				}
			}
			return nil, fmt.Errorf("Cannot find stream")
		})

	lpms.Start(context.Background())
}

func transcode(hlsStream stream.HLSVideoStream) (func(*stream.HLSSegment, bool), error) {
	//Create Transcoder
	profiles := []ffmpeg.VideoProfile{
		ffmpeg.P144p30fps16x9,
		ffmpeg.P240p30fps16x9,
		ffmpeg.P576p30fps16x9,
	}
	workDir := "./tmp"
	t := transcoder.NewFFMpegSegmentTranscoder(profiles, workDir)

	//Create variants in the stream
	strmIDs := make([]string, len(profiles), len(profiles))
	// for i, p := range profiles {
	// 	strmID := randString(10)
	// 	strmIDs[i] = strmID
	// 	pl, _ := m3u8.NewMediaPlaylist(100, 100)
	// 	// hlsStream.AddVariant(strmID, &m3u8.Variant{URI: fmt.Sprintf("%v.m3u8", strmID), Chunklist: pl, VariantParams: transcoder.TranscodeProfileToVariantParams(p)})
	// }

	subscriber := func(seg *stream.HLSSegment, eof bool) {
		//If we get a new video segment for the original HLS stream, do the transcoding.
		// glog.Infof("Got seg: %v", seg.Name)
		// if strmID == hlsStream.GetStreamID() {
		file, err := ioutil.TempFile(workDir, "example")
		if err != nil {
			glog.Errorf("Unable to get tempdir, %v", err)
		}
		defer os.Remove(file.Name())
		if _, err = file.Write(seg.Data); err != nil {
			glog.Errorf("Unable to write temp file %v", err)
		}
		if err = file.Close(); err != nil {
			glog.Errorf("Unable to close file: %v", err)
		}

		//Transcode stream
		tData, err := t.Transcode(file.Name())
		if err != nil {
			glog.Errorf("Error transcoding: %v", err)
		}

		//Insert into HLS stream
		for i, strmID := range strmIDs {
			glog.Infof("Inserting transcoded seg %v into strm: %v", len(tData[i]), strmID)
			if err := hlsStream.AddHLSSegment(&stream.HLSSegment{SeqNo: seg.SeqNo, Name: fmt.Sprintf("%v_%v.ts", strmID, seg.SeqNo), Data: tData[i], Duration: 8}); err != nil {
				glog.Errorf("Error writing transcoded seg: %v", err)
			}
		}
		// }
	}

	return subscriber, nil
}
