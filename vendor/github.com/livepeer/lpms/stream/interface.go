package stream

import (
	"context"

	"github.com/livepeer/m3u8"
	"github.com/livepeer/joy4/av"
)

type AppData interface {
	StreamID() string
}

type VideoStream interface {
	AppData() AppData
	GetStreamID() string
	GetStreamFormat() VideoFormat
	String() string
}

type VideoManifest interface {
	GetManifestID() string
	GetVideoFormat() VideoFormat
	String() string
}

type HLSVideoManifest interface {
	VideoManifest
	GetManifest() (*m3u8.MasterPlaylist, error)
	GetVideoStream(strmID string) (HLSVideoStream, error)
	// AddVideoStream(strmID string, variant *m3u8.Variant) (HLSVideoStream, error)
	AddVideoStream(strm HLSVideoStream, variant *m3u8.Variant) error
	GetStreamVariant(strmID string) (*m3u8.Variant, error)
	GetVideoStreams() []HLSVideoStream
	DeleteVideoStream(strmID string) error
}

//HLSVideoStream contains the master playlist, media playlists in it, and the segments in them.  Each media playlist also has a streamID.
//You can only add media playlists to the stream.
type HLSVideoStream interface {
	VideoStream
	GetStreamPlaylist() (*m3u8.MediaPlaylist, error)
	// GetStreamVariant() *m3u8.Variant
	GetHLSSegment(segName string) (*HLSSegment, error)
	AddHLSSegment(seg *HLSSegment) error
	SetSubscriber(f func(seg *HLSSegment, eof bool))
	End()
}

type RTMPVideoStream interface {
	VideoStream
	ReadRTMPFromStream(ctx context.Context, dst av.MuxCloser) (eof chan struct{}, err error)
	WriteRTMPToStream(ctx context.Context, src av.DemuxCloser) (eof chan struct{}, err error)
	Close()
	Height() int
	Width() int
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
	IsLive() bool
	Finish() error
	String() string
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
	IsLive() bool
	Unsubscribe() error
	String() string
}
