//go:build windows

package media

type MediaSegmenter struct {
	Workdir string
}

func (ms *MediaSegmenter) RunSegmentation(in string, segmentHandler SegmentHandler) {
	// Not supported for Windows
}
