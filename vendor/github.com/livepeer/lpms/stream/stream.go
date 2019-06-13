package stream

import (
	"bytes"
	"context"
	"errors"

	"time"

	"github.com/livepeer/m3u8"
)

var ErrBufferFull = errors.New("Stream Buffer Full")
var ErrBufferEmpty = errors.New("Stream Buffer Empty")
var ErrBufferItemType = errors.New("Buffer Item Type Not Recognized")
var ErrDroppedRTMPStream = errors.New("RTMP Stream Stopped Without EOF")
var ErrHttpReqFailed = errors.New("Http Request Failed")

type VideoFormat uint32

var (
	HLS  = MakeVideoFormatType(avFormatTypeMagic + 1)
	RTMP = MakeVideoFormatType(avFormatTypeMagic + 2)
	DASH = MakeVideoFormatType(avFormatTypeMagic + 3)
)

func MakeVideoFormatType(base uint32) (c VideoFormat) {
	c = VideoFormat(base) << videoFormatOtherBits
	return
}

const avFormatTypeMagic = 577777
const videoFormatOtherBits = 1

type RTMPEOF struct{}

type streamBuffer struct {
	q *Queue
}

func newStreamBuffer() *streamBuffer {
	return &streamBuffer{q: NewQueue(1000)}
}

func (b *streamBuffer) push(in interface{}) error {
	b.q.Put(in)
	return nil
}

func (b *streamBuffer) poll(ctx context.Context, wait time.Duration) (interface{}, error) {
	results, err := b.q.Poll(ctx, 1, wait)
	if err != nil {
		return nil, err
	}
	result := results[0]
	return result, nil
}

func (b *streamBuffer) pop() (interface{}, error) {
	results, err := b.q.Get(1)
	if err != nil {
		return nil, err
	}
	result := results[0]
	return result, nil
}

func (b *streamBuffer) len() int64 {
	return b.q.Len()
}

//We couldn't just use the m3u8 definition
type HLSSegment struct {
	SeqNo    uint64
	Name     string
	Data     []byte
	Duration float64
}

//Compare playlists by segments
func samePlaylist(p1, p2 m3u8.MediaPlaylist) bool {
	return bytes.Compare(p1.Encode().Bytes(), p2.Encode().Bytes()) == 0
}
