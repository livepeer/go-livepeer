package media

import (
	"io"
	"sync"
)

// segmentQueue is a thread-safe queue for CloneableReader segments.
// It encapsulates synchronization (mutex, condvar) and lifecycle state (closed).
type segmentQueue struct {
	mu     sync.Mutex
	cond   *sync.Cond
	items  []CloneableReader
	closed bool
}

func newSegmentQueue() *segmentQueue {
	q := &segmentQueue{}
	q.cond = sync.NewCond(&q.mu)
	return q
}

// Enqueue adds a segment to the queue and signals waiting readers.
func (q *segmentQueue) Enqueue(seg CloneableReader) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return
	}
	q.items = append(q.items, seg)
	q.cond.Broadcast()
}

// Close marks the queue as closed and wakes up any waiters.
func (q *segmentQueue) Close() {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return
	}
	q.closed = true
	q.cond.Broadcast()
}

// Front blocks until there is a front segment or the queue is closed and empty.
// It returns (seg, true) if a segment is available, or (nil, false) on EOF.
func (q *segmentQueue) Front() (CloneableReader, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	for len(q.items) == 0 && !q.closed {
		q.cond.Wait()
	}
	if len(q.items) == 0 && q.closed {
		return nil, false
	}
	return q.items[0], true
}

// Drop removes the current front segment from the queue.
func (q *segmentQueue) Drop() {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.items) == 0 {
		return
	}
	q.items[0] = nil // help GC
	q.items = q.items[1:]
}

// SegmentStreamReader provides a continuous io.ReadCloser that reads across
// multiple segments from a SwitchableSegmentReader. It registers as a handler
// and buffers segments internally to provide seamless reading.
type SegmentStreamReader struct {
	ssr   *SwitchableSegmentReader
	queue *segmentQueue
}

// NewSegmentStreamReader creates a new reader that continuously reads from
// the given SwitchableSegmentReader. The reader will block on Read() until
// data is available or the stream is closed.
func NewSegmentStreamReader(ssr *SwitchableSegmentReader) *SegmentStreamReader {
	q := newSegmentQueue()
	r := &SegmentStreamReader{
		ssr:   ssr,
		queue: q,
	}
	// Register ourselves to receive new segments
	ssr.AddReader(func(seg CloneableReader) {
		// EOSReader signals closure
		if _, isEOS := seg.(*EOSReader); isEOS {
			q.Close()
		} else {
			q.Enqueue(seg)
		}
	})
	return r
}

func (r *SegmentStreamReader) Read(p []byte) (int, error) {
	for {
		seg, ok := r.queue.Front()
		if !ok {
			return 0, io.EOF
		}

		n, err := seg.Read(p)
		if n > 0 {
			// Normal read
			return n, nil
		}
		if err == io.EOF {
			// No data and EOF: drop front and move to the next.
			r.queue.Drop()
			continue
		}

		// Some other error: propagate
		return n, err
	}
}

func (r *SegmentStreamReader) Close() error {
	r.queue.Close()
	return nil
}
