package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/monitor"
	"github.com/livepeer/lpms/ffmpeg"
	"github.com/livepeer/lpms/transcoder"
)

type Transcoder interface {
	Transcode(fname string, profiles []ffmpeg.VideoProfile) ([][]byte, error)
}

type LocalTranscoder struct {
	workDir string
}

func (lt *LocalTranscoder) Transcode(fname string, profiles []ffmpeg.VideoProfile) ([][]byte, error) {
	dirName := randName()
	fullDirName := filepath.Join(lt.workDir, dirName)
	err := os.MkdirAll(fullDirName, 0755)
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(fullDirName)
	tr := transcoder.NewFFMpegSegmentTranscoder(profiles, fullDirName)
	mid, seqNo, parseErr := parseURI(fname)
	start := time.Now()
	if monitor.Enabled && parseErr == nil {
		monitor.LogSegmentTranscodeStarting(seqNo, mid)
	}
	data, err := tr.Transcode(fname)
	if monitor.Enabled && parseErr == nil {
		monitor.LogSegmentTranscodeEnded(seqNo, mid, time.Since(start), common.ProfilesNames(profiles))
	}
	return data, err
}

func NewLocalTranscoder(workDir string) Transcoder {
	return &LocalTranscoder{workDir: workDir}
}

func parseURI(uri string) (string, uint64, error) {
	var mid string
	var seqNo uint64
	parts := strings.Split(uri, "/")
	if len(parts) < 3 {
		return mid, seqNo, fmt.Errorf("BadURI")
	}
	mid = parts[len(parts)-2]
	parts = strings.Split(parts[len(parts)-1], ".")
	seqNo, err := strconv.ParseUint(parts[0], 10, 64)
	return mid, seqNo, err
}
