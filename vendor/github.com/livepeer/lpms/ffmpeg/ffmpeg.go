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
	if len(ps) <= 0 {
		return nil
	}
	inp := C.CString(input)
	params := make([]C.output_params, len(ps))
	for i, param := range ps {
		oname := C.CString(path.Join(workDir, fmt.Sprintf("out%v%v", i, filepath.Base(input))))
		defer C.free(unsafe.Pointer(oname))
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
		fps := C.AVRational{num: C.int(param.Framerate), den: 1}
		params[i] = C.output_params{fname: oname, fps: fps,
			w: C.int(w), h: C.int(h), bitrate: C.int(bitrate)}
	}
	ret := int(C.lpms_transcode(inp, (*C.output_params)(&params[0]), C.int(len(params))))
	C.free(unsafe.Pointer(inp))
	if 0 != ret {
		glog.Infof("Transcoder Return : %v\n", Strerror(ret))
		return ErrorMap[ret]
	}
	return nil
}

// Check media length up to given limits for timestamp (ms) and packet count.
// XXX someday return some actual stats, if limits aren't hit
func CheckMediaLen(fname string, ts_max int, packet_max int) error {
	f := C.CString(fname)
	defer C.free(unsafe.Pointer(f))
	tm := C.int(ts_max)
	pm := C.int(packet_max)
	ret := int(C.lpms_length(f, tm, pm))
	if 0 != ret {
		if nil == ErrorMap[ret] {
			return errors.New("MediaStats Failure")
		}
		return ErrorMap[ret]
	}
	return nil
}

func InitFFmpeg() {
	C.lpms_init()
}
