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
}

func (sr *SwitchableSegmentReader) Read(reader CloneableReader) {
	sr.mu.RLock()
	defer sr.mu.RUnlock()
	sr.reader(reader)
}

func (sr *SwitchableSegmentReader) Close() {
	sr.mu.RLock()
	defer sr.mu.RUnlock()
	sr.reader(&EOSReader{})
}
