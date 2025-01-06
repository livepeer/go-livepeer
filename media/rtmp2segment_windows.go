//go:build windows

package media

import "context"

type MediaSegmenter struct {
	Workdir        string
	MediaMTXClient *MediaMTXClient
}

func (ms *MediaSegmenter) RunSegmentation(ctx context.Context, in string, segmentHandler SegmentHandler) {
	// Not supported for Windows
}
