package media

import (
	"errors"
	"fmt"
	"io"
	"sync"
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

type RingBuffer struct {
	// Public API

	// Ringbuffer size, in bytes
	BufferLen int

	// Private API

	// the buffer
	buffer []byte
	// current write position within buffer
	pos int
	// total bytes written
	nb int64
	// wraparound count
	wraparounds int
	closed      bool
	mu          *sync.Mutex
	cond        *sync.Cond
}

func (rb *RingBuffer) Initialize() error {
	rb.buffer = make([]byte, rb.BufferLen)
	rb.mu = &sync.Mutex{}
	rb.cond = sync.NewCond(rb.mu)
	return nil
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
	if dataLen > rb.BufferLen {
		return 0, errors.New("data exceeds ringbuffer size")
	}
	start := rb.pos
	end := start + dataLen
	if end < rb.BufferLen {
		// contiguous write
		copy(rb.buffer[start:end], data)
	} else {
		// split write
		end = end % rb.BufferLen
		copy(rb.buffer[start:rb.BufferLen], data[0:rb.BufferLen-start])
		copy(rb.buffer[0:end], data[rb.BufferLen-start:])
	}
	rb.pos = end
	rb.nb += int64(dataLen)
	rb.wraparounds = int(rb.nb / int64(rb.BufferLen))
	rb.cond.Broadcast()
	return dataLen, nil
}

func LostDataErr(err error) error {
	return fmt.Errorf("ringbuffer: truncated data: %w", err)
}

func (rb *RingBuffer) readFrom(p []byte, head int) (int, error) {
	start := head % rb.BufferLen
	wraparounds := head / rb.BufferLen

	rb.mu.Lock()
	defer rb.mu.Unlock()

	for rb.pos == start {
		if rb.wraparounds == wraparounds {
			if rb.closed {
				return 0, io.EOF
			}
			// writer and reader in the same position
			// both waiting for more data
			//
			// |------------------------------|
			//         ^
			//       start, wrap=N
			//      rb.pos, wrap=N
			//
			rb.cond.Wait()
			continue
		}
		if rb.wraparounds != wraparounds+1 {
			// writer has lapped reader
			// |------------------------------|
			//         ^
			//       start, wrap=N
			//      rb.pos, wrap>=N+2
			//
			return 0, LostDataErr(errors.New("pos == start"))
		}
		// writer is a full lap ahead of reader
		// |------------------------------|
		//         ^
		//       start, wrap=N
		//      rb.pos, wrap=N+1
		//
		break
	}

	if rb.pos > start && rb.wraparounds != wraparounds {
		// writer has lapped reader
		//
		// |------------------------------|
		//    ^                  ^
		//   start, wrap=N    rb.pos, wrap=N+1
		//
		return 0, LostDataErr(errors.New("pos > start"))
	}
	if rb.pos < start && rb.wraparounds != wraparounds+1 {
		// writer has lapped reader (or reader is in a weird state somehow)
		//
		// |------------------------------|
		//    ^                  ^
		//   rb.pos, wrap=N+2   start, wrap=N
		//
		return 0, LostDataErr(errors.New("pos < start"))
	}

	pos := rb.pos

	// bytes available in p
	pAvail := len(p)

	// TODO test rb.pos == start, should block

	if pos > start {
		// contiguous read - on the same wraparound
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
	edge := rb.BufferLen - start

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

	n += copy(p[n:], rb.buffer[0:remainder])
	return n, nil
}

func (rb *RingBuffer) MakeReader() *RingBufferReader {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	return &RingBufferReader{
		rb:  rb,
		pos: rb.pos,
	}
}

type RingBufferReader struct {
	rb  *RingBuffer
	pos int
}

func (rbr *RingBufferReader) Read(p []byte) (int, error) {
	n, err := rbr.rb.readFrom(p, rbr.pos)
	rbr.pos += n
	return n, err
}
