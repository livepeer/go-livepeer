package core

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/livepeer/go-livepeer/clog"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/monitor"
	"github.com/livepeer/lpms/ffmpeg"
)

type Transcoder interface {
	Transcode(ctx context.Context, md *SegTranscodingMetadata) (*TranscodeData, error)
}

type LocalTranscoder struct {
	workDir string
}

type UnrecoverableError struct {
	error
}

func NewUnrecoverableError(err error) UnrecoverableError {
	return UnrecoverableError{err}
}

var WorkDir string

func (lt *LocalTranscoder) Transcode(ctx context.Context, md *SegTranscodingMetadata) (td *TranscodeData, retErr error) {
	// Returns UnrecoverableError instead of panicking to gracefully notify orchestrator about transcoder's failure
	defer recoverFromPanic(&retErr)

	// Set up in / out config
	in := &ffmpeg.TranscodeOptionsIn{
		Fname: md.Fname,
		Accel: ffmpeg.Software,
	}
	profiles := md.Profiles
	opts := profilesToTranscodeOptions(lt.workDir, ffmpeg.Software, profiles, md.CalcPerceptualHash)
	if md.DetectorEnabled {
		opts = append(opts, detectorsToTranscodeOptions(lt.workDir, ffmpeg.Software, md.DetectorProfiles)...)
	}

	_, seqNo, parseErr := parseURI(md.Fname)
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
		monitor.SegmentTranscoded(0, seqNo, md.Duration, time.Since(start), common.ProfilesNames(profiles), true, true)
	}

	return resToTranscodeData(ctx, res, opts)
}

func NewLocalTranscoder(workDir string) Transcoder {
	return &LocalTranscoder{workDir: workDir}
}

type NvidiaTranscoder struct {
	device  string
	session *ffmpeg.Transcoder
}

func (nv *NvidiaTranscoder) Transcode(ctx context.Context, md *SegTranscodingMetadata) (td *TranscodeData, retErr error) {
	// Returns UnrecoverableError instead of panicking to gracefully notify orchestrator about transcoder's failure
	defer recoverFromPanic(&retErr)

	in := &ffmpeg.TranscodeOptionsIn{
		Fname:  md.Fname,
		Accel:  ffmpeg.Nvidia,
		Device: nv.device,
	}
	profiles := md.Profiles
	out := profilesToTranscodeOptions(WorkDir, ffmpeg.Nvidia, profiles, md.CalcPerceptualHash)
	if md.DetectorEnabled {
		out = append(out, detectorsToTranscodeOptions(WorkDir, ffmpeg.Nvidia, md.DetectorProfiles)...)
	}

	_, seqNo, parseErr := parseURI(md.Fname)
	start := time.Now()

	res, err := nv.session.Transcode(in, out)
	if err != nil {
		return nil, err
	}

	if monitor.Enabled && parseErr == nil {
		// This will run only when fname is actual URL and contains seqNo in it.
		// When orchestrator works as transcoder, `fname` will be relative path to file in local
		// filesystem and will not contain seqNo in it. For that case `SegmentTranscoded` will
		// be called in orchestrator.go
		monitor.SegmentTranscoded(0, seqNo, md.Duration, time.Since(start), common.ProfilesNames(profiles), true, true)
	}

	return resToTranscodeData(ctx, res, out)
}

// TestNvidiaTranscoder tries to transcode test segment on all the devices
func TestNvidiaTranscoder(devices []string) error {
	b := bytes.NewReader(testSegment)
	z, err := gzip.NewReader(b)
	if err != nil {
		return err
	}
	mp4testSeg, err := ioutil.ReadAll(z)
	z.Close()
	if err != nil {
		return err
	}
	fname := filepath.Join(WorkDir, "testseg.tempfile")
	err = ioutil.WriteFile(fname, mp4testSeg, 0644)
	if err != nil {
		return err
	}
	defer os.Remove(fname)
	for _, device := range devices {
		t1 := NewNvidiaTranscoder(device)
		// "145x1" is the minimal resolution that succeeds on Windows, so use "145x145"
		p := ffmpeg.VideoProfile{Resolution: "145x145", Bitrate: "1k", Format: ffmpeg.FormatMP4}
		md := &SegTranscodingMetadata{Fname: fname, Profiles: []ffmpeg.VideoProfile{p, p, p, p}}
		td, err := t1.Transcode(context.Background(), md)

		t1.Stop()
		if err != nil {
			return err
		}
		if len(td.Segments) == 0 || td.Pixels == 0 {
			return errors.New("Empty transcoded segment")
		}
	}

	return nil
}

func NewNvidiaTranscoder(gpu string) TranscoderSession {
	return &NvidiaTranscoder{
		device:  gpu,
		session: ffmpeg.NewTranscoder(),
	}
}

func NewNvidiaTranscoderWithDetector(detector ffmpeg.DetectorProfile, gpu string) (TranscoderSession, error) {
	// Hardcode detection to device 0 for now
	// Transcoding can still run on a separate GPU as we copy frames to CPU before detection
	session, err := ffmpeg.NewTranscoderWithDetector(detector, gpu)
	return &NvidiaTranscoder{
		device:  gpu,
		session: session,
	}, err
}

func (nv *NvidiaTranscoder) Stop() {
	nv.session.StopTranscoder()
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

func resToTranscodeData(logCtx context.Context, res *ffmpeg.TranscodeResults, opts []ffmpeg.TranscodeOptions) (*TranscodeData, error) {
	if len(res.Encoded) != len(opts) {
		return nil, errors.New("lengths of results and options different")
	}

	// Convert results into in-memory bytes following the expected API
	segments := []*TranscodedSegmentData{}
	// Extract detection data from detector outputs
	detections := []ffmpeg.DetectData{}
	for i := range opts {
		if opts[i].Detector == nil {
			oname := opts[i].Oname
			o, err := ioutil.ReadFile(oname)
			if err != nil {
				clog.Errorf(logCtx, "Cannot read transcoded output for name=%s", oname)
				return nil, err
			}
			// Extract perceptual hash if calculated
			var s []byte = nil
			if opts[i].CalcSign {
				sigfile := oname + ".bin"
				s, err = ioutil.ReadFile(sigfile)
				if err != nil {
					clog.Errorf(logCtx, "Cannot read perceptual hash at name=%s", sigfile)
					return nil, err
				}
				err = os.Remove(sigfile)
				if err != nil {
					clog.Errorf(logCtx, "Cannot delete perceptual hash after reading name=%s", sigfile)
				}
			}
			segments = append(segments, &TranscodedSegmentData{Data: o, Pixels: res.Encoded[i].Pixels, PHash: s})
			os.Remove(oname)
		} else {
			detections = append(detections, res.Encoded[i].DetectData)
		}
	}

	return &TranscodeData{
		Segments:   segments,
		Pixels:     res.Decoded.Pixels,
		Detections: detections,
	}, nil
}

func profilesToTranscodeOptions(workDir string, accel ffmpeg.Acceleration, profiles []ffmpeg.VideoProfile, calcPHash bool) []ffmpeg.TranscodeOptions {
	opts := make([]ffmpeg.TranscodeOptions, len(profiles))
	for i := range profiles {
		o := ffmpeg.TranscodeOptions{
			Oname:        fmt.Sprintf("%s/out_%s.tempfile", workDir, common.RandName()),
			Profile:      profiles[i],
			Accel:        accel,
			AudioEncoder: ffmpeg.ComponentOptions{Name: "copy"},
			CalcSign:     calcPHash,
		}
		opts[i] = o
	}
	return opts
}

func detectorsToTranscodeOptions(workDir string, accel ffmpeg.Acceleration, profiles []ffmpeg.DetectorProfile) []ffmpeg.TranscodeOptions {
	opts := make([]ffmpeg.TranscodeOptions, len(profiles))
	for i := range profiles {
		var o ffmpeg.TranscodeOptions
		switch profiles[i].Type() {
		case ffmpeg.SceneClassification:
			classifier := profiles[i].(*ffmpeg.SceneClassificationProfile)
			classifier.ModelPath = ffmpeg.DSceneAdultSoccer.ModelPath
			classifier.Input = ffmpeg.DSceneAdultSoccer.Input
			classifier.Output = ffmpeg.DSceneAdultSoccer.Output
			o = ffmpeg.TranscodeOptions{
				Detector: classifier,
				Accel:    accel,
			}
		}
		opts[i] = o
	}
	return opts
}

func recoverFromPanic(retErr *error) {
	if r := recover(); r != nil {
		err, ok := r.(error)
		if !ok {
			err = errors.New("unrecoverable transcoding failure")
		}
		*retErr = NewUnrecoverableError(err)
	}
}
