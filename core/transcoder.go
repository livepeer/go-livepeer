package core

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/golang/glog"

	"github.com/livepeer/go-livepeer/clog"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/monitor"
	"github.com/livepeer/lpms/ffmpeg"
)

type Transcoder interface {
	Transcode(ctx context.Context, md *SegTranscodingMetadata) (*TranscodeData, error)
	EndTranscodingSession(sessionId string)
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
	opts := profilesToTranscodeOptions(lt.workDir, ffmpeg.Software, md)

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
		monitor.SegmentTranscoded(ctx, 0, seqNo, md.Duration, time.Since(start), common.ProfilesNames(profiles), true, true)
	}

	return resToTranscodeData(ctx, res, opts)
}

func (lt *LocalTranscoder) EndTranscodingSession(sessionId string) {
	// no-op for software transcoder
}

func NewLocalTranscoder(workDir string) Transcoder {
	return &LocalTranscoder{workDir: workDir}
}

type NvidiaTranscoder struct {
	device  string
	session *ffmpeg.Transcoder
}

type NetintTranscoder struct {
	device  string
	session *ffmpeg.Transcoder
}

func (nv *NetintTranscoder) Transcode(ctx context.Context, md *SegTranscodingMetadata) (td *TranscodeData, retErr error) {
	// Returns UnrecoverableError instead of panicking to gracefully notify orchestrator about transcoder's failure
	defer recoverFromPanic(&retErr)

	in := &ffmpeg.TranscodeOptionsIn{
		Fname:  md.Fname,
		Accel:  ffmpeg.Netint,
		Device: nv.device,
	}
	profiles := md.Profiles
	out := profilesToTranscodeOptions(WorkDir, ffmpeg.Netint, md)

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
		monitor.SegmentTranscoded(ctx, 0, seqNo, md.Duration, time.Since(start), common.ProfilesNames(profiles), true, true)
	}

	return resToTranscodeData(ctx, res, out)
}

func (lt *LocalTranscoder) Stop() {
	//no-op for software transcoder
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
	out := profilesToTranscodeOptions(WorkDir, ffmpeg.Nvidia, md)

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
		monitor.SegmentTranscoded(ctx, 0, seqNo, md.Duration, time.Since(start), common.ProfilesNames(profiles), true, true)
	}

	return resToTranscodeData(ctx, res, out)
}

func (nv *NvidiaTranscoder) EndTranscodingSession(sessionId string) {
	nv.Stop()
}

func (nt *NetintTranscoder) EndTranscodingSession(sessionId string) {
	nt.Stop()
}

type transcodeTestParams struct {
	TestAvailable bool
	Cap           Capability
	OutProfile    ffmpeg.VideoProfile
	SegmentPath   string
}

func (params transcodeTestParams) IsRequired() bool {
	return HasCapability(DefaultCapabilities(), params.Cap)
}

func (params transcodeTestParams) Kind() string {
	if params.IsRequired() {
		return "required capability"
	}
	return "optional capability"
}

func (params transcodeTestParams) Name() string {
	name, err := CapabilityToName(params.Cap)
	if err == nil {
		return name
	}
	return "unknown"
}

type continueLoop bool

func transcodeWithSample(handler func(*transcodeTestParams) continueLoop) {
	// default capabilities
	allCaps := append(DefaultCapabilities(), OptionalCapabilities()...)
	handlerParams := transcodeTestParams{SegmentPath: filepath.Join(WorkDir, "testseg.tempfile")}
	defer os.Remove(handlerParams.SegmentPath)
	for _, handlerParams.Cap = range allCaps {
		var capTest CapabilityTest
		capTest, handlerParams.TestAvailable = CapabilityTestLookup[handlerParams.Cap]
		if handlerParams.TestAvailable {
			handlerParams.OutProfile = capTest.outProfile
			b := bytes.NewReader(capTest.inVideoData)
			z, err := gzip.NewReader(b)
			if err != nil {
				continue
			}
			mp4testSeg, err := ioutil.ReadAll(z)
			z.Close()
			if err != nil {
				glog.Errorf("error reading test segment for capability %d: %s", handlerParams.Cap, err)
				continue
			}
			err = ioutil.WriteFile(handlerParams.SegmentPath, mp4testSeg, 0644)
			if err != nil {
				glog.Errorf("error writing test segment for capability %d: %s", handlerParams.Cap, err)
				continue
			}
		}
		if !handler(&handlerParams) {
			return
		}
	}
}

func testAccelTranscode(device string, tf func(device string) TranscoderSession, fname string, profile ffmpeg.VideoProfile, renditionCount int) (outputProduced, outputValid bool, err error) {
	transcoder := tf(device)
	outputProfiles := make([]ffmpeg.VideoProfile, 0, renditionCount)
	for i := 0; i < renditionCount; i++ {
		outputProfiles = append(outputProfiles, profile)
	}
	metadata := &SegTranscodingMetadata{Fname: fname, Profiles: outputProfiles}
	td, err := transcoder.Transcode(context.Background(), metadata)
	transcoder.Stop()
	if err != nil {
		return false, false, err
	}
	outputProduced = len(td.Segments) > 0
	outputValid = td.Pixels > 0
	return outputProduced, outputValid, err
}

// Test which capabilities transcoder supports
func TestTranscoderCapabilities(devices []string, tf func(device string) TranscoderSession) (caps []Capability, fatalError error) {
	// disable logging, unless verbosity is set
	vFlag := flag.Lookup("v").Value.String()
	detailsMsg := ""
	if vFlag == "" {
		detailsMsg = ", set verbosity level to see more details"
		logLevel := ffmpeg.FfmpegGetLogLevel()
		defer ffmpeg.FfmpegSetLogLevel(logLevel)
		ffmpeg.FfmpegSetLogLevel(0)
		ffmpeg.LogTranscodeErrors = false
		defer func() { ffmpeg.LogTranscodeErrors = true }()
	}
	fatalError = nil
	transcodeWithSample(func(params *transcodeTestParams) continueLoop {
		if !params.TestAvailable {
			// Assume capability is supported if we do not have test for it
			caps = append(caps, params.Cap)
			return true
		}
		runRestrictedSessionTest := true
		transcodingFailed := func() {
			// check GeForce limit
			if runRestrictedSessionTest {
				// do it only once
				runRestrictedSessionTest = false
				// if 4 renditions didn't succeed, try 3 renditions on first device to check if it could be session limit
				outputProduced, outputValid, err := testAccelTranscode(devices[0], tf, params.SegmentPath, params.OutProfile, 3)
				if err != nil && outputProduced && outputValid {
					glog.Error("Maximum number of simultaneous NVENC video encoding sessions is restricted by driver")
					fatalError = fmt.Errorf("maximum number of simultaneous NVENC video encoding sessions is restricted by driver")
				}
			}
			if params.IsRequired() {
				// All devices need to support this capability, stop further testing
				fatalError = fmt.Errorf("%s %q is not supported on hardware", params.Kind(), params.Name())
			}
		}
		// check that capability is supported on all devices
		for _, device := range devices {
			outputProduced, outputValid, err := testAccelTranscode(device, tf, params.SegmentPath, params.OutProfile, 4)
			if err != nil {
				glog.Infof("%s %q is not supported on device %s%s", params.Kind(), params.Name(), device, detailsMsg)
				// likely means capability is not supported, don't check on other devices
				transcodingFailed()
				return fatalError == nil
			}
			if !outputProduced || !outputValid {
				// abnormal behavior
				glog.Errorf("Empty result segment when testing for %s %q", params.Kind(), params.Name())
				transcodingFailed()
				return fatalError == nil
			}
			// no error creating 4 renditions - disable 3 renditions test, as restriction is on driver level, not device
			runRestrictedSessionTest = false
		}
		caps = append(caps, params.Cap)
		return true
	})
	return caps, fatalError
}

func testSoftwareTranscode(tmpdir string, fname string, profile ffmpeg.VideoProfile, renditionCount int) (outputProduced, outputValid bool, err error) {
	transcoder := NewLocalTranscoder(tmpdir)
	outputProfiles := make([]ffmpeg.VideoProfile, 0, renditionCount)
	for i := 0; i < renditionCount; i++ {
		outputProfiles = append(outputProfiles, profile)
	}
	metadata := &SegTranscodingMetadata{Fname: fname, Profiles: outputProfiles}
	td, err := transcoder.Transcode(context.Background(), metadata)
	if err != nil {
		return false, false, err
	}
	outputProduced = len(td.Segments) > 0
	outputValid = td.Pixels > 0
	return outputProduced, outputValid, err
}

func TestSoftwareTranscoderCapabilities(tmpdir string) (caps []Capability, fatalError error) {
	// iterate all capabilities and test ones which has test data
	fatalError = nil
	transcodeWithSample(func(params *transcodeTestParams) continueLoop {
		if !params.TestAvailable {
			caps = append(caps, params.Cap)
			return true
		}
		// check that capability is supported on all devices
		outputProduced, outputValid, err := testSoftwareTranscode(tmpdir, params.SegmentPath, params.OutProfile, 4)
		if err != nil {
			// likely means capability is not supported
			return true
		}
		if !outputProduced || !outputValid {
			// abnormal behavior
			fatalError = fmt.Errorf("empty result segment when testing for capability %d", params.Cap)
			return false
		}
		caps = append(caps, params.Cap)
		return true
	})
	return caps, fatalError
}

func GetTranscoderFactoryByAccel(acceleration ffmpeg.Acceleration) (func(device string) TranscoderSession, error) {
	switch acceleration {
	case ffmpeg.Nvidia:
		return NewNvidiaTranscoder, nil
	case ffmpeg.Netint:
		return NewNetintTranscoder, nil
	default:
		return nil, ffmpeg.ErrTranscoderHw
	}
}

func NewNvidiaTranscoder(gpu string) TranscoderSession {
	return &NvidiaTranscoder{
		device:  gpu,
		session: ffmpeg.NewTranscoder(),
	}
}

func NewNetintTranscoder(gpu string) TranscoderSession {
	return &NetintTranscoder{
		device:  gpu,
		session: ffmpeg.NewTranscoder(),
	}
}

func (nv *NvidiaTranscoder) Stop() {
	nv.session.StopTranscoder()
}

func (nv *NetintTranscoder) Stop() {
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

func resToTranscodeData(ctx context.Context, res *ffmpeg.TranscodeResults, opts []ffmpeg.TranscodeOptions) (*TranscodeData, error) {
	if len(res.Encoded) != len(opts) {
		return nil, errors.New("lengths of results and options different")
	}

	// Convert results into in-memory bytes following the expected API
	segments := []*TranscodedSegmentData{}
	for i := range opts {
		oname := opts[i].Oname
		o, err := ioutil.ReadFile(oname)
		if err != nil {
			clog.Errorf(ctx, "Cannot read transcoded output for name=%s", oname)
			return nil, err
		}
		// Extract perceptual hash if calculated
		var s []byte = nil
		if opts[i].CalcSign {
			sigfile := oname + ".bin"
			s, err = ioutil.ReadFile(sigfile)
			if err != nil {
				clog.Errorf(ctx, "Cannot read perceptual hash at name=%s", sigfile)
				return nil, err
			}
			err = os.Remove(sigfile)
			if err != nil {
				clog.Errorf(ctx, "Cannot delete perceptual hash after reading name=%s", sigfile)
			}
		}
		segments = append(segments, &TranscodedSegmentData{Data: o, Pixels: res.Encoded[i].Pixels, PHash: s})
		os.Remove(oname)
	}

	return &TranscodeData{
		Segments: segments,
		Pixels:   res.Decoded.Pixels,
	}, nil
}

func profilesToTranscodeOptions(workDir string, accel ffmpeg.Acceleration, md *SegTranscodingMetadata) []ffmpeg.TranscodeOptions {
	var (
		profiles  []ffmpeg.VideoProfile = md.Profiles
		calcPHash bool                  = md.CalcPerceptualHash
		segPar    *SegmentParameters    = md.SegmentParameters
		metadata  map[string]string     = md.Metadata
	)

	opts := make([]ffmpeg.TranscodeOptions, len(profiles))
	for i := range profiles {
		o := ffmpeg.TranscodeOptions{
			Oname:        fmt.Sprintf("%s/out_%s.tempfile", workDir, common.RandName()),
			Profile:      profiles[i],
			Accel:        accel,
			AudioEncoder: ffmpeg.ComponentOptions{Name: "copy"},
			CalcSign:     calcPHash,
			Metadata:     metadata,
		}
		if segPar != nil && segPar.Clip != nil {
			o.From = segPar.Clip.From
			o.To = segPar.Clip.To
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
