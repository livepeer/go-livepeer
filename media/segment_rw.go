package media

import (
	"errors"
	"io"
	"sync"
)

type SegmentReaderConfig struct{}

// NewSegmentWriter makes a ring of n MediaWriters.
func NewSegmentWriter(n int) *SegmentWriter {
	size := n + 1 // make space for 'next'
	rw := &SegmentWriter{
		writers: make([]*MediaWriter, size),
		size:    size,
		seq:     -1, // -1 makes logic simpler in first Next()
	}
	rw.writers[0] = NewMediaWriter() // precreate first segment
	return rw
}

type SegmentWriter struct {
	mu      sync.Mutex
	writers []*MediaWriter
	size    int
	seq     int
	closed  bool
}

type writerWrapper struct{ mw *MediaWriter }

func (w *writerWrapper) Write(p []byte) (int, error) { return w.mw.Write(p) }
func (w *writerWrapper) Close() error                { return w.mw.Close() }

// Return a fresh writer, pre-creating the next writer.
func (rb *SegmentWriter) Next() (io.WriteCloser, error) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	if rb.closed {
		return nil, io.EOF
	}

	rb.seq += 1
	idx := rb.seq % rb.size
	mw := rb.writers[idx]

	// close existing writer at next before replacing
	nextIdx := (rb.seq + 1) % rb.size
	if old := rb.writers[nextIdx]; old != nil {
		old.Close()
	}
	rb.writers[nextIdx] = NewMediaWriter()
	return &writerWrapper{mw}, nil
}

// MakeReader returns a new reader positioned at the currently active segment.
func (rb *SegmentWriter) MakeReader(_ SegmentReaderConfig) *SegmentReader {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	idx := max(rb.seq-1, -1)
	return &SegmentReader{
		rb:   rb,
		seq:  idx,
		size: rb.size,
	}
}

// Close shuts the SegmentWriter and all its underlying MediaWriters.
// After Close, all future Next() calls on writer or readers will error.
func (rb *SegmentWriter) Close() error {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	if rb.closed {
		return nil
	}
	// close any open MediaWriters
	for _, w := range rb.writers {
		if w != nil {
			w.Close()
		}
	}
	rb.closed = true
	return nil
}

type SegmentReader struct {
	rb   *SegmentWriter
	seq  int
	size int
}

type readerWrapper struct {
	cr  CloneableReader
	seq int
}

func (r *readerWrapper) Read(p []byte) (int, error) { return r.cr.Read(p) }
func (r *readerWrapper) Close() error               { return nil }
func (r *readerWrapper) Seq() int                   { return r.seq }

// Return a reader for the next segment (per‐reader cursor).
func (rr *SegmentReader) Next() (*readerWrapper, error) {
	rr.rb.mu.Lock()
	defer rr.rb.mu.Unlock()

	nextSeq := rr.seq + 1
	// if the writer has been closed, disallow stepping past its last seq
	if rr.rb.closed && nextSeq > rr.rb.seq {
		return nil, io.EOF

	}
	idx := rr.seq + 1
	if idx > (rr.rb.seq + 1) {
		return nil, errors.New("segment out of bounds")
	}
	// +1 to account for the precreate
	if idx <= (rr.rb.seq+1)-rr.size {
		return nil, errors.New("reader fell behind")
	}
	rr.seq = idx
	idx = idx % rr.size
	mw := rr.rb.writers[idx]
	if mw == nil {
		return nil, errors.New("no writer")
	}
	return &readerWrapper{cr: mw.MakeReader(), seq: rr.seq}, nil
}
