//go:build windows

package media

type MediaSegmenter struct {
	Workdir string
}

func (ms *MediaSegmenter) RunSegmentation(ctx context.Context, in string, segmentHandler SegmentHandler, id, sourceType string) {
	// Not supported for Windows
}
