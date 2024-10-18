package media

import (
	"io"
	"sync"
)

type SegmentHandler func(reader io.Reader)

func NoopReader(reader io.Reader) {
	go func() {
		io.Copy(io.Discard, reader)
	}()
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

func (sr *SwitchableSegmentReader) Read(reader io.Reader) {
	sr.mu.RLock()
	defer sr.mu.RUnlock()
	sr.reader(reader)
}
