package transcoder

import (
	"strconv"
	"strings"

	"github.com/ericxtang/m3u8"
	"github.com/golang/glog"
)

type TranscodeProfile struct {
	Name       string
	Bitrate    string
	Framerate  uint
	Resolution string
}

var (
	P720p60fps16x9 = TranscodeProfile{Name: "P720p60fps16x9", Bitrate: "6000k", Framerate: 60, Resolution: "1280x720"}
	P720p30fps16x9 = TranscodeProfile{Name: "P720p30fps16x9", Bitrate: "4000k", Framerate: 30, Resolution: "1280x720"}
	P720p30fps4x3  = TranscodeProfile{Name: "P720p30fps4x3", Bitrate: "4000k", Framerate: 30, Resolution: "960x720"}
	P576p30fps16x9 = TranscodeProfile{Name: "P576p30fps16x9", Bitrate: "1000k", Framerate: 30, Resolution: "1024x576"}
	P360p30fps16x9 = TranscodeProfile{Name: "P360p30fps16x9", Bitrate: "1000k", Framerate: 30, Resolution: "640x360"}
	P360p30fps4x3  = TranscodeProfile{Name: "P360p30fps4x3", Bitrate: "1000k", Framerate: 30, Resolution: "480x360"}
	P240p30fps16x9 = TranscodeProfile{Name: "P240p30fps16x9", Bitrate: "700k", Framerate: 30, Resolution: "426x240"}
	P240p30fps4x3  = TranscodeProfile{Name: "P240p30fps4x3", Bitrate: "700k", Framerate: 30, Resolution: "320x240"}
	P144p30fps16x9 = TranscodeProfile{Name: "P144p30fps16x9", Bitrate: "400k", Framerate: 30, Resolution: "256x144"}
)

var TranscodeProfileLookup = map[string]TranscodeProfile{
	"P720p60fps16x9": P720p60fps16x9,
	"P720p30fps16x9": P720p30fps16x9,
	"P720p30fps4x3":  P720p30fps4x3,
	"P576p30fps16x9": P576p30fps16x9,
	"P360p30fps16x9": P360p30fps16x9,
	"P360p30fps4x3":  P360p30fps4x3,
	"P240p30fps16x9": P240p30fps16x9,
	"P240p30fps4x3":  P240p30fps4x3,
	"P144p30fps16x9": P144p30fps16x9,
}

func TranscodeProfileToVariantParams(t TranscodeProfile) m3u8.VariantParams {
	r := t.Resolution
	r = strings.Replace(r, ":", "x", 1)

	bw := t.Bitrate
	bw = strings.Replace(bw, "k", "000", 1)
	b, err := strconv.ParseUint(bw, 10, 32)
	if err != nil {
		glog.Errorf("Error converting %v to variant params: %v", bw, err)
	}
	return m3u8.VariantParams{Bandwidth: uint32(b), Resolution: r}
}
