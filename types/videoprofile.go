package types

//Standard Profiles:
//1080p60fps: 9000kbps
//1080p30fps: 6000kbps
//720p60fps: 6000kbps
//720p30fps: 4000kbps
//480p30fps: 2000kbps
//360p30fps: 1000kbps
//240p30fps: 700kbps
//144p30fps: 400kbps
type VideoProfile struct {
	Name        string
	Bitrate     string
	Framerate   uint
	AspectRatio string
	Resolution  string
}

//Some sample video profiles
var (
	P720p60fps16x9 = VideoProfile{Name: "P720p60fps16x9", Bitrate: "6000k", Framerate: 60, AspectRatio: "16:9", Resolution: "1280:720"}
	P720p30fps16x9 = VideoProfile{Name: "P720p30fps16x9", Bitrate: "4000k", Framerate: 30, AspectRatio: "16:9", Resolution: "1280:720"}
	P720p30fps4x3  = VideoProfile{Name: "P720p30fps4x3", Bitrate: "4000k", Framerate: 30, AspectRatio: "4:3", Resolution: "960:720"}
	P360p30fps16x9 = VideoProfile{Name: "P360p30fps16x9", Bitrate: "1000k", Framerate: 30, AspectRatio: "16:9", Resolution: "640:360"}
	P360p30fps4x3  = VideoProfile{Name: "P360p30fps4x3", Bitrate: "1000k", Framerate: 30, AspectRatio: "4:3", Resolution: "480:360"}
	P240p30fps16x9 = VideoProfile{Name: "P240p30fps16x9", Bitrate: "700k", Framerate: 30, AspectRatio: "16:9", Resolution: "426:240"}
	P240p30fps4x3  = VideoProfile{Name: "P240p30fps4x3", Bitrate: "700k", Framerate: 30, AspectRatio: "4:3", Resolution: "320:240"}
	P144p30fps16x9 = VideoProfile{Name: "P144p30fps16x9", Bitrate: "400k", Framerate: 30, AspectRatio: "16:9", Resolution: "256:144"}
)

var VideoProfileLookup = map[string]VideoProfile{
	"P720p60fps16x9": P720p60fps16x9,
	"P720p30fps16x9": P720p30fps16x9,
	"P720p30fps4x3":  P720p30fps4x3,
	"P360p30fps16x9": P360p30fps16x9,
	"P360p30fps4x3":  P360p30fps4x3,
	"P240p30fps16x9": P240p30fps16x9,
	"P240p30fps4x3":  P240p30fps4x3,
	"P144p30fps16x9": P144p30fps16x9,
}
