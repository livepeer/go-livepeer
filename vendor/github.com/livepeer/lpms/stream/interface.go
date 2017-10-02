package stream

import (
	"context"

	"github.com/ericxtang/m3u8"
	"github.com/nareix/joy4/av"
)

type VideoStream interface {
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
	ReadRTMPFromStream(ctx context.Context, dst av.MuxCloser) error
	WriteRTMPToStream(ctx context.Context, src av.DemuxCloser) error
	Height() int
	Width() int
}
