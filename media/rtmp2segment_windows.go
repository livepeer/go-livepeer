//go:build windows

package media

import "context"

type MediaSegmenter struct {
	Workdir        string
	MediaMTXClient *MediaMTXClient
	MediaMTXHost   string
}

func (ms *MediaSegmenter) RunSegmentation(ctx context.Context, in string, segmentHandler SegmentHandler, id, sourceType string) {
	// Not supported for Windows
}
