package stream

import (
	"context"
	"errors"

	"github.com/livepeer/m3u8"
)

var ErrNotFound = errors.New("Not Found")
var ErrBadHLSBuffer = errors.New("BadHLSBuffer")
var ErrEOF = errors.New("ErrEOF")

type HLSDemuxer interface {
	PollPlaylist(ctx context.Context) (m3u8.MediaPlaylist, error)
	WaitAndPopSegment(ctx context.Context, name string) ([]byte, error)
	WaitAndGetSegment(ctx context.Context, name string) ([]byte, error)
}

type HLSMuxer interface {
	WriteSegment(seqNo uint64, name string, duration float64, s []byte) error
}
