package media

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestMediaWriterConcurrentReads(t *testing.T) {
	assert := assert.New(t)
	mw := NewMediaWriter()
	mr := mw.MakeReader()

	readerCount := 1000
	msgCount := 100
	msgs := []string{}
	for i := 0; i < msgCount; i++ {
		msgs = append(msgs, fmt.Sprintf("foo-%d", i))
	}
	concatMsg := strings.Join(msgs, "")

	var wg sync.WaitGroup
	for i := 0; i < readerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r := mr.Clone()
			b := new(bytes.Buffer)
			n, err := b.ReadFrom(r)
			assert.Nil(err)
			assert.Equal(len(concatMsg), int(n))
			assert.Equal(concatMsg, string(b.Bytes()))
		}()
	}

	for i := 0; i < msgCount; i++ {
		msg := []byte(msgs[i])
		n, err := mw.Write(msg)
		assert.Nil(err)
		assert.Equal(len(msg), n)
	}
	mw.Close()

	assert.True(wgWait(&wg), "timed out waiting for writes")
}

func TestMediaWriterReadersAfterClose(t *testing.T) {
	assert := assert.New(t)
	mw := NewMediaWriter()
	data := []byte("closing test")

	_, err := mw.Write(data)
	assert.Nil(err, "Write failed")
	mw.Close()

	// Writes after a read should error.
	n, err := mw.Write([]byte("should error"))
	assert.Equal(0, n)
	assert.Equal(io.ErrClosedPipe, err)

	// Any new readers created after close should read the data already present, then EOF
	r1 := mw.MakeReader()
	buf, err := io.ReadAll(r1)
	assert.True(err == nil || err == io.EOF, "ReadAll error: %v", err)
	assert.Equal(data, buf, "Data mismatch after close")

	// Reading again from the same reader yields EOF
	buf2 := make([]byte, 5)
	n2, err2 := r1.Read(buf2)
	assert.Equal(io.EOF, err2, "Expected EOF on second read")
	assert.Equal(0, n2, "Expected 0 bytes read")

	// Another new reader also only sees the data that was written before close
	r2 := mw.MakeReader()
	buf3, err3 := io.ReadAll(r2)
	assert.True(err3 == nil || err3 == io.EOF, "Unexpected error: %v", err3)
	assert.Equal(data, buf3, "Reader should see the same data after close")
}

func TestMediaWriterNoReads(t *testing.T) {
	// Write a large amount of data, ensuring nothing blocks
	assert := assert.New(t)
	mw := NewMediaWriter()

	totalWritten := 0
	for i := 0; i < 1000; i++ {
		d := make([]byte, 1024*10)
		n, err := mw.Write(d)
		assert.Equal(1024*10, n)
		assert.Nil(err)
		totalWritten += n
	}
	mw.Close()

	r := mw.MakeReader()
	b := new(bytes.Buffer)
	n, err := b.ReadFrom(r)
	assert.Nil(err)
	assert.Equal(totalWritten, int(n))
}

func TestMediaWriterNoWrites(t *testing.T) {
	mw := NewMediaWriter()

	r := mw.MakeReader()
	buffer := make([]byte, 10)

	// Reading from an empty MediaWriter should block until it's closed
	// Then it should return io.EOF with no data read
	doneCh := make(chan struct{})
	go func() {
		n, err := r.Read(buffer)
		assert.Equal(t, io.EOF, err, "Expected EOF")
		assert.Equal(t, 0, n, "Expected 0 bytes read")
		close(doneCh)
	}()

	// Give the reader a moment to block before closing
	time.Sleep(100 * time.Millisecond)
	mw.Close()
	select {
	case <-doneCh:
	case <-time.After(time.Second):
		t.Fatalf("Test timed out waiting for reader to finish on no writes")
	}
}

func wgWait(wg *sync.WaitGroup) bool {
	c := make(chan struct{})
	go func() { defer close(c); wg.Wait() }()
	select {
	case <-c:
		return true
	case <-time.After(1 * time.Second):
		return false
	}
}
