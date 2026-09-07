package media

import (
	"errors"
	"io"
	"sync"
	"sync/atomic"
)

/*
	A multi-reader ring buffer implementation.
	Writers are non-blocking.
	Readers block waiting for writes.

	New readers start at the next write.

	NB: Readers are not guaranteed to get
	a time slice, and a busy CPU could
	lead to starvation. The buffer should
	be able to hold at least a couple seconds
	at the expected data rate.
*/

type RingBufferConfig struct {
	// Ringbuffer size, in bytes
	BufferLen int
}

type RingBuffer struct {

	// the buffer
	buffer []byte

	// buffer size
	bufferLen int

	// current write position within buffer
	pos int

	// total bytes written
	nb int64

	// whether the ringbuffer is accepting writes
	closed bool

	// all accesses must be protected
	mu   *sync.Mutex
	cond *sync.Cond
}

func NewRingBuffer(config *RingBufferConfig) (*RingBuffer, error) {
	if config.BufferLen <= 0 {
		return nil, errors.New("ringbuffer: BufferLen must be more than zero")
	}
	mu := &sync.Mutex{}
	return &RingBuffer{
		bufferLen: config.BufferLen,
		buffer:    make([]byte, config.BufferLen),
		mu:        mu,
		cond:      sync.NewCond(mu),
	}, nil
}

func (rb *RingBuffer) Close() {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	rb.closed = true
	rb.cond.Broadcast()
}

func (rb *RingBuffer) Write(data []byte) (int, error) {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	if rb.closed {
		return 0, io.EOF
	}
	dataLen := len(data)
	if dataLen <= 0 {
		return 0, nil
	}
	if dataLen > rb.bufferLen {
		return 0, errors.New("data exceeds ringbuffer size")
	}
	start := rb.pos
	end := start + dataLen
	if end < rb.bufferLen {
		// contiguous write
		copy(rb.buffer[start:end], data)
	} else {
		// split write
		end = end % rb.bufferLen
		copy(rb.buffer[start:rb.bufferLen], data[:rb.bufferLen-start])
		copy(rb.buffer[:end], data[rb.bufferLen-start:])
	}
	rb.pos = end
	rb.nb += int64(dataLen)
	rb.cond.Broadcast()
	return dataLen, nil
}

func (rb *RingBuffer) readFrom(p []byte, head int64) (int, error) {

	// starting position for the reader within the buffer
	start := int(head % int64(rb.bufferLen))

	rb.mu.Lock()
	defer rb.mu.Unlock()

	for rb.nb == head {
		if rb.closed {
			return 0, io.EOF
		}
		// writer and reader in the same position
		// both waiting for more data
		//
		// |------------------------------|
		//         ^
		//       head
		//      rb.nb
		//
		rb.cond.Wait()
		continue
	}

	if head > rb.nb {
		// reader is somehow ahead of writer
		//
		// |------------------------------|
		//          ^          ^
		//        rb.nb       head
		//
		return 0, errors.New("ringbuffer: reader outpaced writer")
	}

	if head < rb.nb-int64(rb.bufferLen) {
		// writer has lapped reader
		//
		// |------------------------------|
		//      ^               ^
		//   head, wrap=N     rb.nb, wrap>=N+1
		//
		return 0, errors.New("ringbuffer: truncated data")
	}

	// current writer position within the buffer
	pos := rb.pos

	// bytes available in p
	pAvail := len(p)

	if pos > start {
		// contiguous read - no wraparound
		//
		// |------------------------------|
		//    ^                  ^
		//   start, wrap=N      pos, wrap=N
		//
		if pos-start < pAvail {
			pAvail = pos - start
		}
		n := copy(p, rb.buffer[start:start+pAvail])

		return n, nil
	}

	// wraparound read
	//
	// |------------------------------|
	//    ^                  ^
	//   pos, wrap=N+1     start, wrap=N
	//

	// distance from start to buffer edge
	edge := rb.bufferLen - start

	// copy as much as possible until
	// the end of buffer or p is full
	n := copy(p, rb.buffer[start:])

	if edge > pAvail {
		// p full, haven't wrapped around yet
		// |------------------------------|
		//    ^              ^         ^
		//   rb.pos        start      full
		//
		return n, nil
	}
	pAvail -= n

	// we've read up to the edge at this point
	// now wrap around to zero and retrieve remainder

	remainder := pos
	if remainder > pAvail {
		// p isn't large enough for remainder
		// |------------------------------|
		//     ^       ^            ^
		//   full     pos         start
		//
		remainder = pAvail
	}

	n += copy(p[n:], rb.buffer[:remainder])
	return n, nil
}

func (rb *RingBuffer) MakeReader() *RingBufferReader {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	return &RingBufferReader{
		rb: rb,
		nb: rb.nb,
	}
}

type RingBufferReader struct {
	rb     *RingBuffer
	nb     int64
	closed atomic.Bool
}

func (rbr *RingBufferReader) Read(p []byte) (int, error) {
	if closed := rbr.closed.Load(); closed {
		return 0, io.EOF
	}
	n, err := rbr.rb.readFrom(p, rbr.nb)
	rbr.nb += int64(n)
	return n, err
}

func (rbr *RingBufferReader) Close() error {
	// NB: If already blocking in Read(),
	// EOF occurs on the *next* Read() call
	// This does not wake up blocked readers.
	rbr.closed.Store(true)
	return nil
}
