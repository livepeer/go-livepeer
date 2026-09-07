package media

import (
	"bytes"
	"io"
	"log/slog"
	"sync"
)

type CloneableReader interface {
	io.Reader
	Clone() CloneableReader
}

type MediaWriter struct {
	mu     *sync.Mutex
	cond   *sync.Cond
	buffer *bytes.Buffer
	closed bool
}

type MediaReader struct {
	source  *MediaWriter
	readPos int
}

func NewMediaWriter() *MediaWriter {
	mu := &sync.Mutex{}
	return &MediaWriter{
		buffer: new(bytes.Buffer),
		cond:   sync.NewCond(mu),
		mu:     mu,
	}
}

func (mw *MediaWriter) Write(data []byte) (int, error) {
	mw.mu.Lock()
	defer mw.mu.Unlock()

	// Write to buffer
	n, err := mw.buffer.Write(data)

	// Signal waiting readers
	mw.cond.Broadcast()

	return n, err
}

func (mw *MediaWriter) readData(startPos int) ([]byte, bool) {
	mw.mu.Lock()
	defer mw.mu.Unlock()
	for {
		totalLen := mw.buffer.Len()
		if startPos < totalLen {
			data := mw.buffer.Bytes()[startPos:totalLen]
			return data, mw.closed
		}
		if startPos > totalLen {
			slog.Info("Invalid start pos, invoking eof")
			return nil, true
		}
		if mw.closed {
			return nil, true
		}
		// Wait for new data
		mw.cond.Wait()
	}
}

func (mw *MediaWriter) Close() error {
	if mw == nil {
		return nil // sometimes happens, weird
	}
	mw.mu.Lock()
	defer mw.mu.Unlock()
	if !mw.closed {
		mw.closed = true
		mw.cond.Broadcast()
	}
	return nil
}

func (mw *MediaWriter) MakeReader() CloneableReader {
	return &MediaReader{
		source: mw,
	}
}

func (mr *MediaReader) Read(p []byte) (int, error) {
	data, eof := mr.source.readData(mr.readPos)
	toRead := len(p)
	if len(data) <= toRead {
		toRead = len(data)
	} else {
		// there is more data to read
		eof = false
	}

	copy(p, data[:toRead])
	mr.readPos += toRead

	var err error = nil
	if eof {
		err = io.EOF
	}

	return toRead, err
}

func (mr *MediaReader) Clone() CloneableReader {
	return &MediaReader{
		source: mr.source,
	}
}
