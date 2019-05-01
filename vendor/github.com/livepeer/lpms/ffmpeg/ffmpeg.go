package ffmpeg

import (
	"errors"
	"fmt"
	"github.com/golang/glog"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"unsafe"
)

// #cgo pkg-config: libavformat libavfilter libavcodec libavutil libswscale gnutls
// #include <stdlib.h>
// #include "lpms_ffmpeg.h"
import "C"

var ErrTranscoderRes = errors.New("TranscoderInvalidResolution")
var ErrTranscoderHw = errors.New("TranscoderInvalidHardware")
var ErrTranscoderInp = errors.New("TranscoderInvalidInput")

type Acceleration int

const (
	Software Acceleration = iota
	Nvidia
	Amd
)

type TranscodeOptionsIn struct {
	Fname  string
	Accel  Acceleration
	Device string
}

type TranscodeOptions struct {
	Oname   string
	Profile VideoProfile
	Accel   Acceleration
	Device  string
}

func RTMPToHLS(localRTMPUrl string, outM3U8 string, tmpl string, seglen_secs string, seg_start int) error {
	inp := C.CString(localRTMPUrl)
	outp := C.CString(outM3U8)
	ts_tmpl := C.CString(tmpl)
	seglen := C.CString(seglen_secs)
	segstart := C.CString(fmt.Sprintf("%v", seg_start))
	ret := int(C.lpms_rtmp2hls(inp, outp, ts_tmpl, seglen, segstart))
	C.free(unsafe.Pointer(inp))
	C.free(unsafe.Pointer(outp))
	C.free(unsafe.Pointer(ts_tmpl))
	C.free(unsafe.Pointer(seglen))
	C.free(unsafe.Pointer(segstart))
	if 0 != ret {
		glog.Infof("RTMP2HLS Transmux Return : %v\n", Strerror(ret))
		return ErrorMap[ret]
	}
	return nil
}

func Transcode(input string, workDir string, ps []VideoProfile) error {

	opts := make([]TranscodeOptions, len(ps))
	for i, param := range ps {
		oname := path.Join(workDir, fmt.Sprintf("out%v%v", i, filepath.Base(input)))
		opt := TranscodeOptions{
			Oname:   oname,
			Profile: param,
			Accel:   Software,
		}
		opts[i] = opt
	}
	inopts := &TranscodeOptionsIn{
		Fname: input,
		Accel: Software,
	}
	return Transcode2(inopts, opts)
}

// return encoding specific options for the given accel
func configAccel(inAcc, outAcc Acceleration, inDev, outDev string) (string, string, error) {
	switch inAcc {
	case Software:
		switch outAcc {
		case Software:
			return "libx264", "scale", nil
		case Nvidia:
			upload := "hwupload_cuda"
			if outDev != "" {
				upload = upload + "=device=" + outDev
			}
			return "h264_nvenc", upload + ",scale_cuda", nil
		}
	case Nvidia:
		switch outAcc {
		case Software:
			return "libx264", "scale_cuda", nil
		case Nvidia:
			// If we encode on a different device from decode then need to transfer
			if outDev != "" && outDev != inDev {
				return "", "", ErrTranscoderInp // XXX not allowed
			}
			return "h264_nvenc", "scale_cuda", nil
		}
	}
	return "", "", ErrTranscoderHw
}
func accelDeviceType(accel Acceleration) (C.enum_AVHWDeviceType, error) {
	switch accel {
	case Software:
		return C.AV_HWDEVICE_TYPE_NONE, nil
	case Nvidia:
		return C.AV_HWDEVICE_TYPE_CUDA, nil

	}
	return C.AV_HWDEVICE_TYPE_NONE, ErrTranscoderHw
}

func Transcode2(input *TranscodeOptionsIn, ps []TranscodeOptions) error {

	if input == nil {
		return ErrTranscoderInp
	}
	if len(ps) <= 0 {
		return nil
	}
	hw_type, err := accelDeviceType(input.Accel)
	if err != nil {
		return err
	}
	fname := C.CString(input.Fname)
	defer C.free(unsafe.Pointer(fname))
	params := make([]C.output_params, len(ps))
	for i, p := range ps {
		oname := C.CString(p.Oname)
		defer C.free(unsafe.Pointer(oname))

		param := p.Profile
		res := strings.Split(param.Resolution, "x")
		if len(res) < 2 {
			return ErrTranscoderRes
		}
		w, err := strconv.Atoi(res[0])
		if err != nil {
			return err
		}
		h, err := strconv.Atoi(res[1])
		if err != nil {
			return err
		}
		br := strings.Replace(param.Bitrate, "k", "000", 1)
		bitrate, err := strconv.Atoi(br)
		if err != nil {
			return err
		}
		encoder, scale_filter, err := configAccel(input.Accel, p.Accel, input.Device, p.Device)
		if err != nil {
			return err
		}
		// preserve aspect ratio along the larger dimension when rescaling
		filters := fmt.Sprintf("fps=%d/%d,%s='w=if(gte(iw,ih),%d,-2):h=if(lt(iw,ih),%d,-2)'", param.Framerate, 1, scale_filter, w, h)
		if input.Accel != Software && p.Accel == Software {
			// needed for hw dec -> hw rescale -> sw enc
			filters = filters + ":format=yuv420p,hwdownload"
		}
		venc := C.CString(encoder)
		vfilt := C.CString(filters)
		defer C.free(unsafe.Pointer(venc))
		defer C.free(unsafe.Pointer(vfilt))
		fps := C.AVRational{num: C.int(param.Framerate), den: 1}
		params[i] = C.output_params{fname: oname, fps: fps,
			w: C.int(w), h: C.int(h), bitrate: C.int(bitrate),
			vencoder: venc, vfilters: vfilt}
	}
	var device *C.char
	if input.Device != "" {
		device = C.CString(input.Device)
		defer C.free(unsafe.Pointer(device))
	}
	inp := &C.input_params{fname: fname, hw_type: hw_type, device: device}
	ret := int(C.lpms_transcode(inp, (*C.output_params)(&params[0]), C.int(len(params))))
	if 0 != ret {
		glog.Infof("Transcoder Return : %v\n", Strerror(ret))
		return ErrorMap[ret]
	}
	return nil
}

func InitFFmpeg() {
	C.lpms_init()
}
