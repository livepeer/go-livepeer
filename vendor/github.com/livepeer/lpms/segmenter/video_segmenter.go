package segmenter

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"path"

	"github.com/ericxtang/m3u8"
	"github.com/golang/glog"
	"github.com/livepeer/lpms/ffmpeg"
	"github.com/livepeer/lpms/stream"
	"github.com/nareix/joy4/av"
)

var ErrSegmenterTimeout = errors.New("SegmenterTimeout")
var ErrSegmenter = errors.New("SegmenterError")
var PlaylistRetryCount = 5
var PlaylistRetryWait = 500 * time.Millisecond

type SegmenterOptions struct {
	EnforceKeyframe bool //Enforce each segment starts with a keyframe
	SegLength       time.Duration
	StartSeq        int
}

type VideoSegment struct {
	Codec  av.CodecType
	Format stream.VideoFormat
	Length time.Duration
	Data   []byte
	Name   string
	SeqNo  uint64
}

type VideoPlaylist struct {
	Format stream.VideoFormat
	// Data   []byte
	Data *m3u8.MediaPlaylist
}

type VideoSegmenter interface {
	RTMPToHLS(ctx context.Context, cleanup bool) error
}

//FFMpegVideoSegmenter segments a RTMP stream by invoking FFMpeg and monitoring the file system.
type FFMpegVideoSegmenter struct {
	WorkDir        string
	LocalRtmpUrl   string
	StrmID         string
	curSegment     int
	curPlaylist    *m3u8.MediaPlaylist
	curPlWaitTime  time.Duration
	curSegWaitTime time.Duration
	SegLen         time.Duration
}

func NewFFMpegVideoSegmenter(workDir string, strmID string, localRtmpUrl string, opt SegmenterOptions) *FFMpegVideoSegmenter {
	if opt.SegLength == 0 {
		opt.SegLength = time.Second * 4
	}
	return &FFMpegVideoSegmenter{WorkDir: workDir, StrmID: strmID, LocalRtmpUrl: localRtmpUrl, SegLen: opt.SegLength, curSegment: opt.StartSeq}
}

//RTMPToHLS invokes FFMpeg to do the segmenting. This method blocks until the segmenter exits.
func (s *FFMpegVideoSegmenter) RTMPToHLS(ctx context.Context, cleanup bool) error {
	//Set up local workdir
	if _, err := os.Stat(s.WorkDir); os.IsNotExist(err) {
		err := os.Mkdir(s.WorkDir, 0700)
		if err != nil {
			return err
		}
	}

	outp := fmt.Sprintf("%s/%s.m3u8", s.WorkDir, s.StrmID)
	ts_tmpl := fmt.Sprintf("%s/%s", s.WorkDir, s.StrmID) + "_%d.ts"
	seglen := strconv.FormatFloat(s.SegLen.Seconds(), 'f', 6, 64)
	ret := ffmpeg.RTMPToHLS(s.LocalRtmpUrl, outp, ts_tmpl, seglen, s.curSegment)
	if cleanup {
		s.Cleanup()
	}
	return ret
}

//PollSegment monitors the filesystem and returns a new segment as it becomes available
func (s *FFMpegVideoSegmenter) PollSegment(ctx context.Context) (*VideoSegment, error) {
	var length time.Duration
	curTsfn := s.WorkDir + "/" + s.StrmID + "_" + strconv.Itoa(s.curSegment) + ".ts"
	nextTsfn := s.WorkDir + "/" + s.StrmID + "_" + strconv.Itoa(s.curSegment+1) + ".ts"
	seg, err := s.pollSegment(ctx, curTsfn, nextTsfn, time.Millisecond*100)
	if err != nil {
		return nil, err
	}

	name := s.StrmID + "_" + strconv.Itoa(s.curSegment) + ".ts"
	plfn := fmt.Sprintf("%s/%s.m3u8", s.WorkDir, s.StrmID)

	for i := 0; i < PlaylistRetryCount; i++ {
		pl, _ := m3u8.NewMediaPlaylist(uint(s.curSegment+1), uint(s.curSegment+1))
		content := readPlaylist(plfn)
		pl.DecodeFrom(bytes.NewReader(content), true)
		for _, plSeg := range pl.Segments {
			if plSeg != nil && plSeg.URI == name {
				length, err = time.ParseDuration(fmt.Sprintf("%vs", plSeg.Duration))
				break
			}
		}
		if length != 0 {
			break
		}
		if i < PlaylistRetryCount {
			glog.V(4).Infof("Waiting to load duration from playlist")
			time.Sleep(PlaylistRetryWait)
			continue
		} else {
			length, err = time.ParseDuration(fmt.Sprintf("%vs", pl.TargetDuration))
		}
	}

	s.curSegment = s.curSegment + 1
	// glog.Infof("Segment: %v, len:%v", name, len(seg))
	return &VideoSegment{Codec: av.H264, Format: stream.HLS, Length: length, Data: seg, Name: name, SeqNo: uint64(s.curSegment - 1)}, err
}

//PollPlaylist monitors the filesystem and returns a new playlist as it becomes available
func (s *FFMpegVideoSegmenter) PollPlaylist(ctx context.Context) (*VideoPlaylist, error) {
	plfn := fmt.Sprintf("%s/%s.m3u8", s.WorkDir, s.StrmID)
	var lastPl []byte
	if s.curPlaylist == nil {
		lastPl = nil
	} else {
		lastPl = s.curPlaylist.Encode().Bytes()
	}

	pl, err := s.pollPlaylist(ctx, plfn, time.Millisecond*100, lastPl)
	if err != nil {
		return nil, err
	}

	p, err := m3u8.NewMediaPlaylist(50000, 50000)
	err = p.DecodeFrom(bytes.NewReader(pl), true)
	if err != nil {
		return nil, err
	}

	s.curPlaylist = p
	return &VideoPlaylist{Format: stream.HLS, Data: p}, err
}

func readPlaylist(fn string) []byte {
	if _, err := os.Stat(fn); err == nil {
		content, err := ioutil.ReadFile(fn)
		if err != nil {
			return nil
		}
		return content
	}

	return nil
}

func (s *FFMpegVideoSegmenter) pollPlaylist(ctx context.Context, fn string, sleepTime time.Duration, lastFile []byte) (f []byte, err error) {
	for {
		if _, err := os.Stat(fn); err == nil {
			if err != nil {
				return nil, err
			}

			content, err := ioutil.ReadFile(fn)
			if err != nil {
				return nil, err
			}

			//The m3u8 package has some bugs, so the translation isn't 100% correct...
			p, err := m3u8.NewMediaPlaylist(50000, 50000)
			err = p.DecodeFrom(bytes.NewReader(content), true)
			if err != nil {
				return nil, err
			}
			curFile := p.Encode().Bytes()

			// fmt.Printf("p.Segments: %v\n", p.Segments[0])
			// fmt.Printf("lf: %s \ncf: %s \ncomp:%v\n\n", lastFile, curFile, bytes.Compare(lastFile, curFile))
			if lastFile == nil || bytes.Compare(lastFile, curFile) != 0 {
				s.curPlWaitTime = 0
				return content, nil
			}
		}

		select {
		case <-ctx.Done():
			glog.V(4).Infof("ctx.Done()!!!")
			return nil, ctx.Err()
		default:
		}

		if s.curPlWaitTime >= 10*s.SegLen {
			return nil, ErrSegmenterTimeout
		}
		time.Sleep(sleepTime)
		s.curPlWaitTime = s.curPlWaitTime + sleepTime
	}

}

func (s *FFMpegVideoSegmenter) pollSegment(ctx context.Context, curFn string, nextFn string, sleepTime time.Duration) (f []byte, err error) {
	var content []byte
	for {
		//Because FFMpeg keeps appending to the current segment until it's full before moving onto the next segment, we monitor the existance of
		//the next file as a signal for the completion of the current segment.
		if _, err := os.Stat(nextFn); err == nil {
			content, err = ioutil.ReadFile(curFn)
			if err != nil {
				return nil, err
			}
			s.curSegWaitTime = 0
			return content, err
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		if s.curSegWaitTime > 10*s.SegLen {
			return nil, ErrSegmenterTimeout
		}

		time.Sleep(sleepTime)
		s.curSegWaitTime = s.curSegWaitTime + sleepTime
	}
}

func (s *FFMpegVideoSegmenter) Cleanup() {
	glog.V(4).Infof("Cleaning up video segments.....")
	files, _ := filepath.Glob(path.Join(s.WorkDir, s.StrmID) + "*")
	for _, fn := range files {
		os.Remove(fn)
	}
}
