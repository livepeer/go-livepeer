package core

import (
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/monitor"
	"github.com/livepeer/lpms/ffmpeg"

	"github.com/golang/glog"
)

type Transcoder interface {
	Transcode(fname string, profiles []ffmpeg.VideoProfile) ([][]byte, error)
}

type LocalTranscoder struct {
	workDir string
}

func (lt *LocalTranscoder) Transcode(fname string, profiles []ffmpeg.VideoProfile) ([][]byte, error) {
	// Set up in / out config
	in := &ffmpeg.TranscodeOptionsIn{
		Fname: fname,
		Accel: ffmpeg.Software,
	}
	opts := profilesToTranscodeOptions(lt.workDir, ffmpeg.Software, profiles)

	_, seqNo, parseErr := parseURI(fname)
	start := time.Now()

	_, err := ffmpeg.Transcode3(in, opts)
	if err != nil {
		return [][]byte{}, err
	}

	if monitor.Enabled && parseErr == nil {
		// This will run only when fname is actual URL and contains seqNo in it.
		// When orchestrator works as transcoder, `fname` will be relative path to file in local
		// filesystem and will not contain seqNo in it. For that case `SegmentTranscoded` will
		// be called in orchestrator.go
		monitor.SegmentTranscoded(0, seqNo, time.Since(start), common.ProfilesNames(profiles))
	}

	return resToTranscodeData(opts)
}

func NewLocalTranscoder(workDir string) Transcoder {
	return &LocalTranscoder{workDir: workDir}
}

type NvidiaTranscoder struct {
	workDir string
	devices []string

	// The following fields need to be protected by the mutex `mu`
	mu     *sync.Mutex
	devIdx int // current index within the devices list
}

func (nv *NvidiaTranscoder) getDevice() string {
	nv.mu.Lock()
	defer nv.mu.Unlock()
	nv.devIdx = (nv.devIdx + 1) % len(nv.devices)
	return nv.devices[nv.devIdx]
}

func (nv *NvidiaTranscoder) Transcode(fname string, profiles []ffmpeg.VideoProfile) ([][]byte, error) {
	// Set up in / out config
	in := &ffmpeg.TranscodeOptionsIn{
		Fname:  fname,
		Accel:  ffmpeg.Nvidia,
		Device: nv.getDevice(),
	}
	opts := profilesToTranscodeOptions(nv.workDir, ffmpeg.Nvidia, profiles)

	// Do the Transcoding
	_, err := ffmpeg.Transcode3(in, opts)
	if err != nil {
		return [][]byte{}, err
	}

	return resToTranscodeData(opts)
}

func NewNvidiaTranscoder(devices string, workDir string) Transcoder {
	d := strings.Split(devices, ",")
	return &NvidiaTranscoder{devices: d, workDir: workDir, mu: &sync.Mutex{}}
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

func resToTranscodeData(opts []ffmpeg.TranscodeOptions) ([][]byte, error) {
	// Convert results into in-memory bytes following the expected API
	out := make([][]byte, len(opts), len(opts))
	for i := range opts {
		oname := opts[i].Oname
		o, err := ioutil.ReadFile(oname)
		if err != nil {
			glog.Error("Cannot read transcoded output for ", oname)
			return [][]byte{}, err
		}
		out[i] = o
		os.Remove(oname)
	}
	return out, nil
}

func profilesToTranscodeOptions(workDir string, accel ffmpeg.Acceleration, profiles []ffmpeg.VideoProfile) []ffmpeg.TranscodeOptions {
	opts := make([]ffmpeg.TranscodeOptions, len(profiles), len(profiles))
	for i := range profiles {
		o := ffmpeg.TranscodeOptions{
			Oname:   fmt.Sprintf("%s/out_%s.ts", workDir, common.RandName()),
			Profile: profiles[i],
			Accel:   accel,
		}
		opts[i] = o
	}
	return opts
}
