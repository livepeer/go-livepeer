package ffmpeg

// #cgo pkg-config: libavformat
//#include "ffmpeg_errors.h"
//#include "lpms_ffmpeg.h"
import "C"
import (
	"encoding/binary"
	"errors"
	"unsafe"
)

func error_map() map[int]error {
	// errs is a []byte , we really need an []int so need to convert
	errs := C.GoBytes(unsafe.Pointer(&C.ffmpeg_errors), C.sizeof_ffmpeg_errors)
	m := make(map[int]error)
	for i := 0; i < len(errs)/C.sizeof_int; i++ {
		// unsigned -> C 4-byte signed int -> golang nativeint
		// golang nativeint is usually 8 bytes on 64bit, so intermediate cast is
		// needed to preserve sign
		v := int(int32(binary.LittleEndian.Uint32(errs[i*C.sizeof_int : (i+1)*C.sizeof_int])))
		m[v] = errors.New(Strerror(v))
	}
	for i := -255; i < 0; i++ {
		v := Strerror(i)
		if "UNKNOWN_ERROR" != v {
			m[i] = errors.New(v)
		}
	}

	// Add in LPMS specific errors
	lpmsErrors := []struct {
		code C.int
		desc string
	}{
		{code: C.lpms_ERR_INPUT_PIXFMT, desc: "Unsupported input pixel format"},
		{code: C.lpms_ERR_FILTERS, desc: "Error initializing filtergraph"},
	}
	for _, v := range lpmsErrors {
		m[int(v.code)] = errors.New(v.desc)
	}

	return m
}

var ErrorMap = error_map()

// Use of this source code is governed by a MIT license that can be found in the LICENSE file.
// Corbatto (luca@corbatto.de)

// Strerror returns a descriptive string of the given return code.
//
// C-Function: av_strerror
func Strerror(errnum int) string {
	buf := make([]C.char, C.ffmpeg_AV_ERROR_MAX_STRING_SIZE)
	if C.av_strerror(C.int(errnum), (*C.char)(unsafe.Pointer(&buf[0])), C.size_t(len(buf))) != 0 {
		return "UNKNOWN_ERROR"
	}
	return C.GoString((*C.char)(unsafe.Pointer(&buf[0])))
}
