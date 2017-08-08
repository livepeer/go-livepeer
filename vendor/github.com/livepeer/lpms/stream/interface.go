package stream

import (
	"context"

	"github.com/ericxtang/m3u8"
	"github.com/nareix/joy4/av"
)

type VideoStream_ interface {
	GetStreamID() string
	GetStreamFormat() VideoFormat
}

//HLSVideoStream contains the master playlist, media playlists in it, and the segments in them.  Each media playlist also has a streamID.
//You can only add media playlists to the stream.
type HLSVideoStream interface {
	VideoStream_
	GetMasterPlaylist() (*m3u8.MasterPlaylist, error)
	GetVariantPlaylist(strmID string) (*m3u8.MediaPlaylist, error)
	GetHLSSegment(strmID string, segName string) (*HLSSegment, error)
	AddVariant(strmID string, variant *m3u8.Variant) error
	AddHLSSegment(strmID string, seg *HLSSegment) error
	SetSubscriber(f func(strm HLSVideoStream, strmID string, seg *HLSSegment))
}

type RTMPVideoStream interface {
	VideoStream_
	ReadRTMPFromStream(ctx context.Context, dst av.MuxCloser) error
	WriteRTMPToStream(ctx context.Context, src av.DemuxCloser) error
}
