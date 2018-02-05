package ffmpeg

import (
	"github.com/golang/glog"
	"unsafe"
)

// #cgo pkg-config: libavformat
// #include <stdlib.h>
// #include "lpms_ffmpeg.h"
import "C"

func RTMPToHLS(localRTMPUrl string, outM3U8 string, tmpl string, seglen_secs string) error {
	inp := C.CString(localRTMPUrl)
	outp := C.CString(outM3U8)
	ts_tmpl := C.CString(tmpl)
	seglen := C.CString(seglen_secs)
	ret := int(C.lpms_rtmp2hls(inp, outp, ts_tmpl, seglen))
	C.free(unsafe.Pointer(inp))
	C.free(unsafe.Pointer(outp))
	C.free(unsafe.Pointer(ts_tmpl))
	C.free(unsafe.Pointer(seglen))
	if 0 != ret {
		glog.Infof("RTMP2HLS Transmux Return : %v\n", Strerror(ret))
		return ErrorMap[ret]
	}
	return nil
}

func InitFFmpeg() {
	C.lpms_init()
}

func DeinitFFmpeg() {
	C.lpms_deinit()
}
