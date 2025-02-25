package media

import (
	"io"
	"sync"
)

type SegmentHandler func(reader CloneableReader)

func NoopReader(reader CloneableReader) {
	// don't have to do anything here
}

type EOSReader struct{}

func (r *EOSReader) Read(p []byte) (n int, err error) {
	return 0, io.EOF
}
func (r *EOSReader) Clone() CloneableReader {
	return r
}

type SwitchableSegmentReader struct {
	mu     sync.RWMutex
	reader SegmentHandler
	seg    CloneableReader
}

func NewSwitchableSegmentReader() *SwitchableSegmentReader {
	return &SwitchableSegmentReader{
		reader: NoopReader,
	}
}

func (sr *SwitchableSegmentReader) SwitchReader(newReader SegmentHandler) {
	sr.mu.Lock()
	defer sr.mu.Unlock()
	sr.reader = newReader
	if sr.seg != nil {
		// immediately send the current segment instead of waiting for the next one
		// clone since current segment may have already been partially consumed
		sr.reader(sr.seg.Clone())
	}
}

func (sr *SwitchableSegmentReader) Read(reader CloneableReader) {
	sr.mu.Lock()
	defer sr.mu.Unlock()
	sr.reader(reader)
	sr.seg = reader
}

func (sr *SwitchableSegmentReader) Close() {
	sr.mu.RLock()
	defer sr.mu.RUnlock()
	sr.reader(&EOSReader{})
}
