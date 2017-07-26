package core

import (
	"context"

	"github.com/livepeer/lpms/segmenter"
	"github.com/livepeer/lpms/stream"
)

type RTMPSegmenter interface {
	SegmentRTMPToHLS(ctx context.Context, rs stream.Stream, hs stream.Stream, segOptions segmenter.SegmenterOptions) error
}
