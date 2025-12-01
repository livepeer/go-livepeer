package media

import (
	"io"
	"sync"
)

type SegmentHandler func(reader CloneableReader)

type EOSReader struct{}

func (r *EOSReader) Read(p []byte) (n int, err error) {
	return 0, io.EOF
}
func (r *EOSReader) Clone() CloneableReader {
	return r
}

type SwitchableSegmentReader struct {
	mu      sync.RWMutex
	readers []SegmentHandler
	seg     CloneableReader
}

func NewSwitchableSegmentReader() *SwitchableSegmentReader {
	return &SwitchableSegmentReader{
		readers: []SegmentHandler{},
	}
}

func (sr *SwitchableSegmentReader) AddReader(newReader SegmentHandler) {
	sr.mu.Lock()
	defer sr.mu.Unlock()
	sr.readers = append(sr.readers, newReader)
	if sr.seg != nil {
		// immediately send the current segment instead of waiting for the next one
		// clone since current segment may have already been partially consumed
		newReader(sr.seg.Clone())
	}
}

func (sr *SwitchableSegmentReader) Read(reader CloneableReader) {
	sr.mu.Lock()
	readers := sr.readers
	sr.seg = reader
	sr.mu.Unlock()
	for _, r := range readers {
		r(reader.Clone())
	}
}

func (sr *SwitchableSegmentReader) Close() {
	sr.mu.RLock()
	readers := sr.readers
	sr.mu.RUnlock()
	for _, r := range readers {
		r(&EOSReader{})
	}
}
