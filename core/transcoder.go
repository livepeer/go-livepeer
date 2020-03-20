package core

import (
	"errors"
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
	Transcode(job string, fname string, profiles []ffmpeg.VideoProfile) (*TranscodeData, error)
}

type LocalTranscoder struct {
	workDir string
}

func (lt *LocalTranscoder) Transcode(job string, fname string, profiles []ffmpeg.VideoProfile) (*TranscodeData, error) {
	// Set up in / out config
	in := &ffmpeg.TranscodeOptionsIn{
		Fname: fname,
		Accel: ffmpeg.Software,
	}
	opts := profilesToTranscodeOptions(lt.workDir, ffmpeg.Software, profiles)

	_, seqNo, parseErr := parseURI(fname)
	start := time.Now()

	res, err := ffmpeg.Transcode3(in, opts)
	if err != nil {
		return nil, err
	}

	if monitor.Enabled && parseErr == nil {
		// This will run only when fname is actual URL and contains seqNo in it.
		// When orchestrator works as transcoder, `fname` will be relative path to file in local
		// filesystem and will not contain seqNo in it. For that case `SegmentTranscoded` will
		// be called in orchestrator.go
		monitor.SegmentTranscoded(0, seqNo, time.Since(start), common.ProfilesNames(profiles))
	}

	return resToTranscodeData(res, opts)
}

func NewLocalTranscoder(workDir string) Transcoder {
	return &LocalTranscoder{workDir: workDir}
}

type nvSegResult struct {
	*TranscodeData
	error
}

type nvSegData struct {
	session  *ffmpeg.Transcoder
	fname    string
	profiles []ffmpeg.VideoProfile
	res      chan *nvSegResult
}

type NvidiaTranscoder struct {
	device  *segStack
	session *ffmpeg.Transcoder
}

type segStack struct {
	segs []*nvSegData
	mu   *sync.Mutex
	cv   *sync.Cond
	gpu  string
}

func newSegStack(gpu string) *segStack {
	mu := &sync.Mutex{}
	return &segStack{
		segs: []*nvSegData{},
		mu:   mu,
		cv:   sync.NewCond(mu),
		gpu:  gpu,
	}
}

func (ss *segStack) push(seg *nvSegData) {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	ss.segs = append(ss.segs, seg)
	ss.cv.Broadcast()
	if monitor.Enabled {
		monitor.GPUBacklog(ss.gpu, len(ss.segs))
	}
}

func (ss *segStack) pop() *nvSegData {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	for len(ss.segs) <= 0 {
		ss.cv.Wait()
	}
	seg := ss.segs[len(ss.segs)-1]
	ss.segs = ss.segs[:len(ss.segs)-1]
	if monitor.Enabled {
		monitor.GPUBacklog(ss.gpu, len(ss.segs))
	}
	return seg
}

func (nv *NvidiaTranscoder) Transcode(job string, fname string, profiles []ffmpeg.VideoProfile) (*TranscodeData, error) {

	segData := &nvSegData{
		session:  nv.session,
		fname:    fname,
		profiles: profiles,
		res:      make(chan *nvSegResult, 1),
	}
	nv.device.push(segData)
	res := <-segData.res
	return res.TranscodeData, res.error
}

var nvidiaDevices map[string]*segStack

func StartNvidiaTranscoders(devices string, workDir string) {
	nvidiaDevices = make(map[string]*segStack)
	d := strings.Split(devices, ",")
	for _, n := range d {
		st := newSegStack(n)
		nvidiaDevices[n] = st
		go runTranscodeLoop(st, workDir)
	}
}

func NewNvidiaTranscoder(gpu string) TranscoderSession {
	return &NvidiaTranscoder{
		device:  nvidiaDevices[gpu],
		session: ffmpeg.NewTranscoder(),
	}
}

func (nv *NvidiaTranscoder) Stop() {
	nv.session.StopTranscoder()
}

func runTranscodeLoop(stack *segStack, workDir string) {
	for {
		seg := stack.pop()
		// Set up in / out config
		in := &ffmpeg.TranscodeOptionsIn{
			Fname:  seg.fname,
			Accel:  ffmpeg.Nvidia,
			Device: stack.gpu,
		}
		opts := profilesToTranscodeOptions(workDir, ffmpeg.Nvidia, seg.profiles)
		// Do the Transcoding
		res, err := seg.session.Transcode(in, opts)
		if err != nil {
			seg.res <- &nvSegResult{nil, err}
			continue
		}
		td, err := resToTranscodeData(res, opts)
		seg.res <- &nvSegResult{td, err}
	}
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

func resToTranscodeData(res *ffmpeg.TranscodeResults, opts []ffmpeg.TranscodeOptions) (*TranscodeData, error) {
	if len(res.Encoded) != len(opts) {
		return nil, errors.New("lengths of results and options different")
	}

	// Convert results into in-memory bytes following the expected API
	segments := make([]*TranscodedSegmentData, len(opts), len(opts))
	for i := range opts {
		oname := opts[i].Oname
		o, err := ioutil.ReadFile(oname)
		if err != nil {
			glog.Error("Cannot read transcoded output for ", oname)
			return nil, err
		}
		segments[i] = &TranscodedSegmentData{Data: o, Pixels: res.Encoded[i].Pixels}
		os.Remove(oname)
	}

	return &TranscodeData{
		Segments: segments,
		Pixels:   res.Decoded.Pixels,
	}, nil
}

func profilesToTranscodeOptions(workDir string, accel ffmpeg.Acceleration, profiles []ffmpeg.VideoProfile) []ffmpeg.TranscodeOptions {
	opts := make([]ffmpeg.TranscodeOptions, len(profiles), len(profiles))
	for i := range profiles {
		o := ffmpeg.TranscodeOptions{
			Oname:        fmt.Sprintf("%s/out_%s.tempfile", workDir, common.RandName()),
			Profile:      profiles[i],
			Accel:        accel,
			AudioEncoder: ffmpeg.ComponentOptions{Name: "copy"},
		}
		opts[i] = o
	}
	return opts
}
