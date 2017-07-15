package net

import (
	"context"
	"math/big"
)

type VideoNetwork interface {
	GetBroadcaster(strmID string) (Broadcaster, error)
	GetSubscriber(strmID string) (Subscriber, error)
	Connect(nodeID, nodeAddr string) error
	SetupProtocol() error
	SendTranscodResult(nodeID string, strmID string, transcodeResult map[string]string) error
}

//Broadcaster takes a streamID and a reader, and broadcasts the data to whatever underlining network.
//Note the data param doesn't have to be the raw data.  The implementation can choose to encode any struct.
//Example:
// 	s := GetStream("StrmID")
// 	b := ppspp.NewBroadcaster("StrmID", s.Metadata())
// 	for seqNo, data := range s.Segments() {
// 		b.Broadcast(seqNo, data)
// 	}
//	b.Finish()
type Broadcaster interface {
	Broadcast(seqNo uint64, data []byte) error
	Finish() error
}

//Subscriber subscribes to a stream defined by strmID.  It returns a reader that contains the stream.
//Example 1:
//	sub, metadata := ppspp.NewSubscriber("StrmID")
//	stream := NewStream("StrmID", metadata)
//	ctx, cancel := context.WithCancel(context.Background()
//	err := sub.Subscribe(ctx, func(seqNo uint64, data []byte){
//		stream.WriteSeg(seqNo, data)
//	})
//	time.Sleep(time.Second * 5)
//	cancel()
//
//Example 2:
//	sub.Unsubscribe() //This is the same with calling cancel()
type Subscriber interface {
	Subscribe(ctx context.Context, gotData func(seqNo uint64, data []byte, eof bool)) error
	Unsubscribe() error
}

//Standard Profiles:
//1080p_60fps: 9000kbps
//1080p_30fps: 6000kbps
//720p_60fps: 6000kbps
//720p_30fps: 4000kbps
//480p_30fps: 2000kbps
//360p_30fps: 1000kbps
//240p_30fps: 700kbps
//144p_30fps: 400kbps
type VideoProfile struct {
	Name        string
	Bitrate     string
	Framerate   uint
	AspectRatio string
	Resolution  string
}

type TranscodeConfig struct {
	StrmID              string
	Profiles            []VideoProfile
	PerformOnchainClaim bool
	JobID               *big.Int
}

type Transcoder interface {
	Transcode(strmID string, config TranscodeConfig, gotPlaylist func(masterPlaylist []byte)) error
}

//Some sample video profiles
var (
	P_720P_60FPS_16_9 = VideoProfile{Name: "P720P60FPS169", Bitrate: "6000k", Framerate: 60, AspectRatio: "16:9", Resolution: "1280:720"}
	P_720P_30FPS_16_9 = VideoProfile{Name: "P720P30FPS169", Bitrate: "4000k", Framerate: 30, AspectRatio: "16:9", Resolution: "1280:720"}
	P_720P_30FPS_4_3  = VideoProfile{Name: "P720P30FPS43", Bitrate: "4000k", Framerate: 30, AspectRatio: "4:3", Resolution: "960:720"}
	P_360P_30FPS_16_9 = VideoProfile{Name: "P360P30FPS169", Bitrate: "1000k", Framerate: 30, AspectRatio: "16:9", Resolution: "640:360"}
	P_360P_30FPS_4_3  = VideoProfile{Name: "P360P30FPS43", Bitrate: "1000k", Framerate: 30, AspectRatio: "4:3", Resolution: "480:360"}
	P_240P_30FPS_16_9 = VideoProfile{Name: "P240P30FPS169", Bitrate: "700k", Framerate: 30, AspectRatio: "16:9", Resolution: "426:240"}
	P_240P_30FPS_4_3  = VideoProfile{Name: "P240P30FPS43", Bitrate: "700k", Framerate: 30, AspectRatio: "4:3", Resolution: "320:240"}
	P_144P_30FPS_16_9 = VideoProfile{Name: "P144P30FPS169", Bitrate: "400k", Framerate: 30, AspectRatio: "16:9", Resolution: "256:144"}
)

var VideoProfileLookup = map[string]VideoProfile{
	"P720P60FPS169": P_720P_60FPS_16_9,
	"P720P30FPS169": P_720P_30FPS_16_9,
	"P720P30FPS43":  P_720P_30FPS_4_3,
	"P360P30FPS169": P_360P_30FPS_16_9,
	"P360P30FPS43":  P_360P_30FPS_4_3,
	"P240P30FPS169": P_240P_30FPS_16_9,
	"P240P30FPS43":  P_240P_30FPS_4_3,
	"P144P30FPS169": P_144P_30FPS_16_9,
}
