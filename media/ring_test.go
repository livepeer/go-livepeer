package media

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
)

// patternReader emits an endless sequence of bytes 0,1,2…255,0,1…
type patternReader struct {
	next byte
}

func (pr *patternReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = pr.next
		pr.next++
	}
	return len(p), nil
}

// throttledWriter sleeps after every Write to simulate pacing.
type throttledWriter struct {
	w    io.Writer
	rate int64

	start   time.Time // window start time
	written int64     // bytes written in current 1ms window
}

func (tw *throttledWriter) Write(p []byte) (int, error) {
	now := time.Now()
	if tw.start.IsZero() {
		tw.start = now
	}
	// reset every 1 ms
	elapsed := now.Sub(tw.start)
	if elapsed >= time.Millisecond {
		tw.start = now
		tw.written = 0
		elapsed = 0
	}

	// projected bytes if we write all of p in this window
	projected := tw.written + int64(len(p))

	// ideal elapsed time for projected bytes:
	//   expectedDur = projected * (1 sec / rate)
	expectedDur := time.Duration(projected) * time.Second / time.Duration(tw.rate)

	// if we're ahead of schedule, sleep just enough
	if wait := expectedDur - elapsed; wait > 0 {
		time.Sleep(wait)
	}

	// perform the write
	n, err := tw.w.Write(p)
	tw.written += int64(n)
	return n, err
}

func TestRingbuffer_RW(t *testing.T) {
	assert := assert.New(t)

	// invalid configuration
	rb, err := NewRingBuffer(&RingBufferConfig{})
	assert.Nil(rb)
	assert.Equal(errors.New("ringbuffer: BufferLen must be more than zero"), err)

	// --- initialize ring buffer ---
	rbc := &RingBufferConfig{BufferLen: 100}
	rb, err = NewRingBuffer(rbc)
	assert.Nil(err)
	reader := rb.MakeReader()
	lateReader := rb.MakeReader()

	n, err := rb.Write([]byte("hello, world"))
	assert.Nil(err)
	assert.Equal(12, n)

	out := make([]byte, 25)
	n, err = reader.Read(out)
	assert.Nil(err)
	assert.Equal(12, n)
	assert.Equal("hello, world", string(out[:n]))

	// creating a reader after a writer should start at edge
	reader2 := rb.MakeReader()

	n, err = rb.Write([]byte(" and how are you today?"))
	assert.Nil(err)
	assert.Equal(23, n)

	// first reader
	n, err = reader.Read(out)
	assert.Nil(err)
	assert.Equal(23, n)
	assert.Equal(" and how are you today?", string(out[:n]))

	// edgy reader
	n, err = reader2.Read(out)
	assert.Nil(err)
	assert.Equal(23, n)
	assert.Equal(" and how are you today?", string(out[:n]))

	// created early but hasn't read yet so should start from beginning
	n, err = lateReader.Read(out)
	assert.Nil(err)
	assert.Equal(25, n)
	assert.Equal("hello, world and how are ", string(out[:n]))

	// check lapping on a boundary
	rbc = &RingBufferConfig{BufferLen: 4}
	rb, err = NewRingBuffer(rbc)
	assert.Nil(err)
	reader = rb.MakeReader()
	n, err = rb.Write([]byte("yyyy"))
	assert.Equal(4, n)
	assert.Nil(err)
	n, err = reader.Read(out)
	assert.Equal(4, n)
	assert.Nil(err)
	assert.Equal("yyyy", string(out[:n]))

	// check zero length writes
	n, err = rb.Write([]byte{})
	assert.Zero(n)
	assert.Nil(err)

	// check excessive writes
	n, err = rb.Write([]byte("12345"))
	assert.Zero(n)
	assert.Equal(errors.New("data exceeds ringbuffer size"), err)

	// check writes to a closed ringbuffer
	n, err = rb.Write([]byte("zz"))
	assert.Equal(2, n)
	assert.Nil(err)
	rb.Close()
	n, err = rb.Write([]byte("!!"))
	assert.Equal(0, n)
	assert.Equal(io.EOF, err)
	// Read should drain remainder
	n, err = reader.Read(out)
	assert.Equal(2, n)
	assert.Nil(err)
	assert.Equal("zz", string(out[:n]))
	// Next read should yield EOF
	n, err = reader.Read(out)
	assert.Zero(n)
	assert.Equal(io.EOF, err)
}

func TestRingBuffer_RW_Block(t *testing.T) {
	assert := assert.New(t)
	rbc := &RingBufferConfig{BufferLen: 4}
	rb, err := NewRingBuffer(rbc)
	assert.Nil(err)
	reader := rb.MakeReader()
	//next := make(chan bool)
	//next2 := make(chan bool)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		n, err := rb.Write([]byte("1234"))
		assert.Equal(4, n)
		assert.Nil(err)
		time.Sleep(5 * time.Millisecond)
		n, err = rb.Write([]byte("5678"))
		assert.Equal(4, n)
		assert.Nil(err)
		rb.Close()
	}()
	buf := make([]byte, 10)
	n, err := reader.Read(buf)
	assert.Equal(4, n)
	assert.Nil(err)
	assert.Equal("1234", string(buf[:n]))
	n, err = reader.Read(buf)
	assert.Equal(4, n)
	assert.Equal("5678", string(buf[:n]))
	assert.Nil(err)
	n, err = reader.Read(buf)
	assert.Equal(0, n)
	assert.Equal(io.EOF, err)
	assert.True(wgWait(&wg))
}

func TestRingbuffer_ConcurrentRW(t *testing.T) {
	const (
		numReaders = 30

		// set ringbuffer to approx 100ms
		// in practice there should be several *seconds* of data in it
		bufferLen  = 64 * 1024 * 1      // 64 KB ring buffer
		totalBytes = bufferLen * 100    // 6.4 MB write
		rate       = 100 * 5_00_000 / 8 // 50 Mbps
	)

	assert := assert.New(t)

	// --- initialize ring buffer ---
	rbc := &RingBufferConfig{BufferLen: bufferLen}
	rb, err := NewRingBuffer(rbc)
	assert.Nil(err)

	// prepare reader hashes
	readerHashes := make([][]byte, numReaders)
	var wg sync.WaitGroup

	// --- start readers ---
	for i := 0; i < numReaders; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			reader := rb.MakeReader()
			hasher := sha256.New()

			n, err := io.Copy(hasher, reader)
			if err != nil {
				assert.Equal(io.EOF, err, fmt.Sprintf("idx %d", idx))
			}
			assert.Equal(int64(totalBytes), n, fmt.Sprintf("idx %d", idx))
			readerHashes[idx] = hasher.Sum(nil)
		}(i)
	}

	// --- writer: pattern → hash → ringbuffer (throttled) ---
	writerHash := sha256.New()
	src := io.LimitReader(&patternReader{}, totalBytes) // only totalBytes
	tee := io.TeeReader(src, writerHash)                // also feed the hash
	tw := &throttledWriter{w: io.Writer(rb), rate: rate}

	n, err := io.Copy(tw, tee)
	assert.Equal(int64(totalBytes), n)
	assert.Nil(err)

	// signal EOF to readers
	rb.Close()

	// wait for readers to finish
	wg.Wait()

	// compare hashes
	want := writerHash.Sum(nil)
	for i, got := range readerHashes {
		assert.Equal(want, got, fmt.Sprintf("reader hash mismatch for %d", i))
	}
}

// 1) writer too far ahead of reader → ErrLostData
func TestRingbuffer_LostData(t *testing.T) {
	assert := assert.New(t)

	rbc := &RingBufferConfig{BufferLen: 4}
	rb, err := NewRingBuffer(rbc)
	assert.Nil(err)
	reader := rb.MakeReader()
	// write 3 bytes
	n, err := rb.Write([]byte("abc"))
	assert.Equal(3, n)
	assert.Nil(err)
	// write 4 bytes to force a wrap and lapping
	n, err = rb.Write([]byte("xxxx"))
	assert.Equal(4, n)
	assert.Nil(err)
	buf3 := make([]byte, 3)
	n, err = reader.Read(buf3)
	assert.Zero(n)
	assert.ErrorContains(err, "truncated data")

	// check lapped 2 times where pos < read head
	rb, err = NewRingBuffer(rbc)
	assert.Nil(err)
	reader = rb.MakeReader()
	// need to split the writes
	_, _ = rb.Write([]byte("zzzz"))
	n, err = reader.Read(buf3)
	assert.Equal(3, n)
	assert.Nil(err)
	assert.Equal("zzz", string(buf3))

	n, err = rb.Write([]byte("zzzz"))
	assert.Equal(4, n)
	assert.Nil(err)
	n, err = reader.Read(buf3)
	assert.Zero(n)
	assert.ErrorContains(err, "truncated data")

	// check pos == read head but lapped >2 times
	rb, err = NewRingBuffer(rbc)
	assert.Nil(err)
	reader = rb.MakeReader()
	n, err = rb.Write([]byte{1})
	assert.Equal(1, n)
	assert.Nil(err)
	n, err = reader.Read(buf3)
	assert.Equal(1, n)
	assert.Nil(err)
	assert.Equal([]byte{1}, buf3[:n])
	n, err = rb.Write([]byte{2, 3, 4})
	assert.Equal(3, n)
	assert.Nil(err)
	n, err = reader.Read(buf3)
	assert.Equal(3, n)
	assert.Nil(err)
	assert.Equal([]byte{2, 3, 4}, buf3[:n])
	n, err = rb.Write([]byte{5, 6, 7})
	assert.Equal(3, n)
	assert.Nil(err)
	n, err = rb.Write([]byte{8, 9, 10})
	assert.Equal(3, n)
	assert.Nil(err)
	n, err = reader.Read(buf3)
	assert.Zero(n)
	assert.ErrorContains(err, "truncated data")

	// simulate reader outpacing writer (should never happen in practice)
	reader.nb = rb.nb + 1
	n, err = reader.Read(buf3)
	assert.Zero(n)
	assert.ErrorContains(err, "reader outpaced writer")
}

func TestRingbuffer_ContiguousReads(t *testing.T) {
	assert := assert.New(t)
	rbc := &RingBufferConfig{BufferLen: 8}
	rb, err := NewRingBuffer(rbc)
	assert.Nil(err)
	r := rb.MakeReader()

	// write 5 bytes
	_, _ = rb.Write([]byte("HELLO"))

	// p < available (5): read 2 bytes
	small := make([]byte, 2)
	n, err := r.Read(small)
	assert.NoError(err)
	assert.Equal(2, n)
	assert.Equal("HE", string(small))

	// p == remaining (5-2=3): read 3 bytes
	exact := make([]byte, 3)
	n, err = r.Read(exact)
	assert.NoError(err)
	assert.Equal(3, n)
	assert.Equal("LLO", string(exact)) // "LLO"

	// p > remaining: read only 0 bytes if no new data yet
	big := make([]byte, 10)
	// close to signal EOF
	rb.Close()
	n, err = r.Read(big)
	assert.Zero(n)
	assert.Equal(io.EOF, err)
}

func TestRingbuffer_WraparoundReads(t *testing.T) {
	assert := assert.New(t)

	rbc := &RingBufferConfig{BufferLen: 8}
	rb, err := NewRingBuffer(rbc)
	assert.Nil(err)

	// prepare to read first 5 bytes
	buf5 := make([]byte, 5)
	reader := rb.MakeReader()

	// write 7 bytes → "ABCDEFG"
	n, err := rb.Write([]byte("ABCDEFG"))
	assert.Equal(7, n)
	assert.Nil(err)

	n, err = reader.Read(buf5)
	assert.Equal(5, n)
	assert.Nil(err)
	assert.Equal("ABCDE", string(buf5))

	// write 4 bytes → wraps: "XYZDEFGW"
	n, err = rb.Write([]byte("WXYZ"))
	assert.Equal(4, n)
	assert.Nil(err)

	// reader wraps but won't read all available data
	n, err = reader.Read(buf5)
	assert.Equal(5, n)
	assert.Nil(err)
	assert.Equal("FGWXY", string(buf5))

	// read remainder for completeness
	n, err = reader.Read(buf5)
	assert.Equal(1, n)
	assert.Nil(err)
	assert.Equal("Z", string(buf5[:n]))

	// check reader wraparound + read all available data
	rb, err = NewRingBuffer(rbc)
	assert.Nil(err)
	n, err = rb.Write([]byte("12345"))
	assert.Equal(5, n)
	assert.Nil(err)
	reader = rb.MakeReader() // start mid stream
	n, err = rb.Write([]byte("67890"))
	assert.Equal(5, n)
	assert.Nil(err)
	n, err = reader.Read(buf5)
	assert.Equal(5, n)
	assert.Nil(err)
	assert.Equal("67890", string(buf5))
}

func sync_TestRingbuffer_ReadClose(t *testing.T) {
	assert := assert.New(t)

	rbc := &RingBufferConfig{BufferLen: 8}
	rb, err := NewRingBuffer(rbc)
	assert.Nil(err)

	reader := rb.MakeReader()
	reader2 := rb.MakeReader()
	var wg sync.WaitGroup

	wg.Go(func() {
		buf := make([]byte, 5)
		n, err := reader.Read(buf)
		assert.Equal(4, n)
		assert.Nil(err)
		assert.Equal([]byte{1, 2, 3, 4, 0}, buf)
		n, err = reader.Read(buf)
		assert.Equal(0, n)
		assert.Equal(io.EOF, err)
	})

	wg.Go(func() {
		// reader should be drained
		buf := make([]byte, 5)
		n, err := reader2.Read(buf)
		assert.Equal(4, n)
		assert.Nil(err)
		assert.Equal([]byte{1, 2, 3, 4, 0}, buf)
		n, err = reader2.Read(buf)
		assert.Equal([]byte{5, 6, 7, 8, 0}, buf)
		assert.Equal(4, n)
		n, err = reader2.Read(buf)
		assert.Equal(0, n)
		assert.Equal(io.EOF, err)
	})

	// give the reader goroutine a moment to block waiting for data
	time.Sleep(5 * time.Millisecond)

	// close the reader concurrently
	err = reader.Close()
	assert.Nil(err)

	rb.Write([]byte{1, 2, 3, 4})
	time.Sleep(1 * time.Millisecond)
	rb.Write([]byte{5, 6, 7, 8})

	rb.Close()

	wgWait(&wg)
}

func TestRingbuffer_ReadClose(t *testing.T) {
	synctest.Test(t, sync_TestRingbuffer_ReadClose)
}
