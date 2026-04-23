package trickle

import (
	"bytes"
	"fmt"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type latencySegment interface {
	writeData([]byte)
	close()
	newReader() latencyReader
}

type latencyReader interface {
	readData() ([]byte, bool)
}

type baselineLatencySegment struct {
	mutex  *sync.Mutex
	cond   *sync.Cond
	buffer bytes.Buffer
	closed bool
}

type baselineLatencyReader struct {
	segment *baselineLatencySegment
	readPos int
}

func newBaselineLatencySegment() latencySegment {
	mu := &sync.Mutex{}
	return &baselineLatencySegment{
		mutex: mu,
		cond:  sync.NewCond(mu),
	}
}

func (s *baselineLatencySegment) writeData(data []byte) {
	s.mutex.Lock()
	s.buffer.Write(data)
	s.cond.Broadcast()
	s.mutex.Unlock()
}

func (s *baselineLatencySegment) close() {
	s.mutex.Lock()
	s.closed = true
	s.cond.Broadcast()
	s.mutex.Unlock()
}

func (s *baselineLatencySegment) newReader() latencyReader {
	return &baselineLatencyReader{segment: s}
}

func (r *baselineLatencyReader) readData() ([]byte, bool) {
	s := r.segment
	s.mutex.Lock()
	defer s.mutex.Unlock()
	for {
		totalLen := s.buffer.Len()
		if r.readPos < totalLen {
			data := s.buffer.Bytes()[r.readPos:totalLen]
			r.readPos = totalLen
			return data, s.closed
		}
		if s.closed {
			return nil, true
		}
		s.cond.Wait()
	}
}

type fixedPageLatencySegment struct {
	mutex    *sync.Mutex
	cond     *sync.Cond
	pages    [][]byte
	pageSize int
	total    int
	closed   bool
}

type fixedPageLatencyReader struct {
	segment *fixedPageLatencySegment
	readPos int
}

func newFixedPageLatencySegment(pageSize int) latencySegment {
	mu := &sync.Mutex{}
	return &fixedPageLatencySegment{
		mutex:    mu,
		cond:     sync.NewCond(mu),
		pages:    make([][]byte, 0, 4),
		pageSize: pageSize,
	}
}

func (s *fixedPageLatencySegment) writeData(data []byte) {
	s.mutex.Lock()
	for len(data) > 0 {
		if len(s.pages) == 0 || len(s.pages[len(s.pages)-1]) == cap(s.pages[len(s.pages)-1]) {
			s.pages = append(s.pages, make([]byte, 0, s.pageSize))
		}
		lastIdx := len(s.pages) - 1
		last := s.pages[lastIdx]
		remaining := cap(last) - len(last)
		if remaining > len(data) {
			remaining = len(data)
		}
		last = append(last, data[:remaining]...)
		s.pages[lastIdx] = last
		s.total += remaining
		data = data[remaining:]
	}
	s.cond.Broadcast()
	s.mutex.Unlock()
}

func (s *fixedPageLatencySegment) close() {
	s.mutex.Lock()
	s.closed = true
	s.cond.Broadcast()
	s.mutex.Unlock()
}

func (s *fixedPageLatencySegment) newReader() latencyReader {
	return &fixedPageLatencyReader{segment: s}
}

func (r *fixedPageLatencyReader) readData() ([]byte, bool) {
	s := r.segment
	s.mutex.Lock()
	defer s.mutex.Unlock()
	for {
		if r.readPos < s.total {
			pageIdx := r.readPos / s.pageSize
			offset := r.readPos % s.pageSize
			page := s.pages[pageIdx]
			data := page[offset:]
			r.readPos += len(data)
			return data, s.closed && r.readPos == s.total
		}
		if s.closed {
			return nil, true
		}
		s.cond.Wait()
	}
}

type currentLatencySegment struct {
	segment *Segment
}

type currentLatencyReader struct {
	reader *SegmentSubscriber
}

func newCurrentLatencySegment() latencySegment {
	return &currentLatencySegment{segment: newSegment(1)}
}

type atomicConfigLatencySegment struct {
	mutex  *sync.Mutex
	cond   *sync.Cond
	buffer *segmentBuffer
	closed bool
	done   atomic.Bool
}

type atomicConfigLatencyReader struct {
	segment *atomicConfigLatencySegment
	readPos int
}

func newAtomicConfigLatencySegment(initialCap, maxCap int) func() latencySegment {
	return func() latencySegment {
		mu := &sync.Mutex{}
		return &atomicConfigLatencySegment{
			mutex:  mu,
			cond:   sync.NewCond(mu),
			buffer: newSegmentBufferWithPageCaps(initialCap, maxCap),
		}
	}
}

func (s *atomicConfigLatencySegment) writeData(data []byte) {
	s.mutex.Lock()
	s.buffer.write(data)
	s.cond.Broadcast()
	s.mutex.Unlock()
}

func (s *atomicConfigLatencySegment) close() {
	s.mutex.Lock()
	s.closed = true
	s.done.Store(true)
	s.cond.Broadcast()
	s.mutex.Unlock()
}

func (s *atomicConfigLatencySegment) newReader() latencyReader {
	return &atomicConfigLatencyReader{segment: s}
}

func (r *atomicConfigLatencyReader) readData() ([]byte, bool) {
	s := r.segment
	data, nextPos, atTail, invalid, published := s.buffer.readChunk(r.readPos)
	if invalid {
		return nil, true
	}
	if len(data) > 0 {
		r.readPos = nextPos
		return data, s.done.Load() && atTail && nextPos == published
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()
	for {
		data, nextPos, atTail, invalid, published := s.buffer.readChunk(r.readPos)
		if invalid {
			return nil, true
		}
		if len(data) > 0 {
			r.readPos = nextPos
			return data, s.closed && atTail && nextPos == published
		}
		if s.closed {
			return nil, true
		}
		s.cond.Wait()
	}
}

func (s *currentLatencySegment) writeData(data []byte) {
	s.segment.writeData(data)
}

func (s *currentLatencySegment) close() {
	s.segment.close()
}

func (s *currentLatencySegment) newReader() latencyReader {
	return &currentLatencyReader{reader: &SegmentSubscriber{segment: s.segment}}
}

func (r *currentLatencyReader) readData() ([]byte, bool) {
	return r.reader.readData()
}

type latencySummary struct {
	p50 time.Duration
	p95 time.Duration
	p99 time.Duration
	max time.Duration
}

func TestSegmentSubscriberLatencyProfiles(t *testing.T) {
	type variant struct {
		name string
		new  func() latencySegment
	}

	variants := []variant{
		{name: "baseline-bytes-buffer", new: newBaselineLatencySegment},
		{name: "paged-cond-1MiB", new: func() latencySegment { return newFixedPageLatencySegment(1024 * 1024) }},
		{name: "current-atomic-paged", new: newCurrentLatencySegment},
		{name: "atomic-fixed-1MiB", new: newAtomicConfigLatencySegment(1024*1024, 1024*1024)},
		{name: "atomic-256KiB-to-1MiB", new: newAtomicConfigLatencySegment(256*1024, 1024*1024)},
	}

	for _, subscribers := range []int{1, 2, 5, 15, 40} {
		for _, variant := range variants {
			t.Run(fmt.Sprintf("%s/subscribers=%d", variant.name, subscribers), func(t *testing.T) {
				summary := measureSubscriberLatency(t, variant.new, subscribers)
				t.Logf(
					"p50=%s p95=%s p99=%s max=%s",
					summary.p50,
					summary.p95,
					summary.p99,
					summary.max,
				)
			})
		}
	}
}

func TestSegmentSubscriberLatencyProfilesSmallWrites(t *testing.T) {
	type variant struct {
		name string
		new  func() latencySegment
	}

	variants := []variant{
		{name: "baseline-bytes-buffer", new: newBaselineLatencySegment},
		{name: "paged-cond-1MiB", new: func() latencySegment { return newFixedPageLatencySegment(1024 * 1024) }},
		{name: "current-atomic-paged", new: newCurrentLatencySegment},
		{name: "atomic-fixed-1MiB", new: newAtomicConfigLatencySegment(1024*1024, 1024*1024)},
		{name: "atomic-256KiB-to-1MiB", new: newAtomicConfigLatencySegment(256*1024, 1024*1024)},
	}

	for _, subscribers := range []int{1, 2, 5, 15, 40} {
		for _, variant := range variants {
			t.Run(fmt.Sprintf("%s/subscribers=%d", variant.name, subscribers), func(t *testing.T) {
				summary := measureSubscriberLatencyWithConfig(t, variant.new, subscribers, 8*1024*1024, 1300)
				t.Logf(
					"p50=%s p95=%s p99=%s max=%s",
					summary.p50,
					summary.p95,
					summary.p99,
					summary.max,
				)
			})
		}
	}
}

func TestAtomicFixed1MiBConcurrentRead(t *testing.T) {
	for i := 0; i < 200; i++ {
		runConcurrentReadCheck(t, newAtomicConfigLatencySegment(1024*1024, 1024*1024), 15)
	}
}

func TestAtomic256KiBTo1MiBConcurrentRead(t *testing.T) {
	for i := 0; i < 200; i++ {
		runConcurrentReadCheck(t, newAtomicConfigLatencySegment(256*1024, 1024*1024), 15)
	}
}

// TestSegmentReadDataWriteCloseRace is a regression test for a race where
// Segment.readData's lock-free fast path could return a premature EOF.
//
// The bug: readChunk snapshots published (e.g. 0 on a fresh segment), returns
// no data, and then the fast path checked `done.Load() && readPos >= published`
// using the *stale* snapshot. If a write+close landed between the snapshot and
// the done check, the reader saw done=true, readPos(0) >= stale_published(0),
// and returned EOF — silently dropping all data.
//
// The fix: when the fast-path readChunk returns no data, always fall through to
// the locked slow path instead of short-circuiting on done.
func TestSegmentReadDataWriteCloseRace(t *testing.T) {
	const (
		iterations = 200_000
		readers    = 4
	)
	payload := []byte("x")

	var failures atomic.Int64
	var wg sync.WaitGroup
	for iter := 0; iter < iterations; iter++ {
		seg := newSegment(0)

		wg.Add(readers)
		for i := 0; i < readers; i++ {
			go func() {
				defer wg.Done()
				sub := &SegmentSubscriber{segment: seg}
				var total int
				for {
					data, eof := sub.readData()
					total += len(data)
					if eof {
						break
					}
				}
				if total != len(payload) {
					failures.Add(1)
				}
			}()
		}

		seg.writeData(payload)
		seg.close()
		wg.Wait()
	}

	if f := failures.Load(); f > 0 {
		t.Fatalf("%d / %d readers saw premature EOF (0 bytes)", f, int64(iterations)*readers)
	}
}

// TestSegmentReadDataPartialFastPathEOFRegression covers a second stale-snapshot
// case where the fast path returns some data and incorrectly marks it as EOF.
func TestSegmentReadDataPartialFastPathEOFRegression(t *testing.T) {
	const (
		iterations = 200_000
		readers    = 4
	)

	var failures atomic.Int64
	var wg sync.WaitGroup
	for iter := 0; iter < iterations; iter++ {
		seg := newSegment(0)
		seg.writeData([]byte("a"))

		wg.Add(readers)
		for i := 0; i < readers; i++ {
			go func() {
				defer wg.Done()
				sub := &SegmentSubscriber{segment: seg}
				var total int
				for {
					data, eof := sub.readData()
					total += len(data)
					if eof {
						break
					}
				}
				if total != 2 {
					failures.Add(1)
				}
			}()
		}

		seg.writeData([]byte("b"))
		seg.close()
		wg.Wait()
	}

	if f := failures.Load(); f > 0 {
		t.Fatalf("%d / %d readers saw premature EOF after a partial read", f, int64(iterations)*readers)
	}
}

func measureSubscriberLatency(t *testing.T, newSegment func() latencySegment, subscribers int) latencySummary {
	t.Helper()
	return measureSubscriberLatencyWithConfig(t, newSegment, subscribers, 8*1024*1024, 32*1024)
}

func measureSubscriberLatencyWithConfig(t *testing.T, newSegment func() latencySegment, subscribers int, totalBytes int, chunkSize int) latencySummary {
	t.Helper()

	writeSizes := buildWriteSizes(totalBytes, chunkSize)
	payloads := make([][]byte, len(writeSizes))
	for i, size := range writeSizes {
		payloads[i] = bytes.Repeat([]byte("segment-latency-payload-"), (size/24)+1)[:size]
	}
	publishedAt := make([]atomic.Int64, len(writeSizes))
	latencyCh := make(chan []int64, subscribers)
	segment := newSegment()

	var wg sync.WaitGroup
	wg.Add(subscribers)
	for subscriberIdx := 0; subscriberIdx < subscribers; subscriberIdx++ {
		go func() {
			defer wg.Done()
			reader := segment.newReader()
			writeCursor := 0
			totalRead := 0
			latencies := make([]int64, 0, len(writeSizes))
			pendingBytes := 0

			for {
				data, eof := reader.readData()
				if len(data) > 0 {
					now := time.Now().UnixNano()
					pendingBytes += len(data)
					for pendingBytes > 0 {
						if writeCursor >= len(writeSizes) {
							t.Errorf("subscriber read too many writes: %d", writeCursor)
							return
						}
						nextWriteSize := writeSizes[writeCursor]
						if pendingBytes < nextWriteSize {
							break
						}
						published := publishedAt[writeCursor].Load()
						if published == 0 {
							t.Errorf("missing publish timestamp for write %d", writeCursor)
							return
						}
						latencies = append(latencies, now-published)
						pendingBytes -= nextWriteSize
						writeCursor++
					}
					totalRead += len(data)
				}
				if eof {
					if totalRead != totalBytes {
						t.Errorf("subscriber read %d bytes, want %d", totalRead, totalBytes)
					}
					if pendingBytes != 0 || writeCursor != len(writeSizes) {
						t.Errorf("subscriber ended with pendingBytes=%d writeCursor=%d writes=%d", pendingBytes, writeCursor, len(writeSizes))
					}
					latencyCh <- latencies
					return
				}
			}
		}()
	}

	for writeIdx, payload := range payloads {
		publishedAt[writeIdx].Store(time.Now().UnixNano())
		segment.writeData(payload)
	}
	segment.close()

	wg.Wait()
	close(latencyCh)

	allLatencies := make([]int64, 0, len(writeSizes)*subscribers)
	for latencies := range latencyCh {
		allLatencies = append(allLatencies, latencies...)
	}
	if len(allLatencies) == 0 {
		t.Fatal("no latency samples collected")
	}

	slices.Sort(allLatencies)
	return latencySummary{
		p50: time.Duration(percentileSample(allLatencies, 50)),
		p95: time.Duration(percentileSample(allLatencies, 95)),
		p99: time.Duration(percentileSample(allLatencies, 99)),
		max: time.Duration(allLatencies[len(allLatencies)-1]),
	}
}

func runConcurrentReadCheck(t *testing.T, newSegment func() latencySegment, subscribers int) {
	t.Helper()

	const (
		totalBytes = 8 * 1024 * 1024
		chunkSize  = 32 * 1024
	)

	chunks := totalBytes / chunkSize
	payload := bytes.Repeat([]byte("segment-test-payload-"), (chunkSize/21)+1)[:chunkSize]
	want := bytes.Repeat(payload, chunks)
	segment := newSegment()

	results := make([][]byte, subscribers)
	var wg sync.WaitGroup
	wg.Add(subscribers)
	for subscriberIdx := 0; subscriberIdx < subscribers; subscriberIdx++ {
		go func(idx int) {
			defer wg.Done()
			reader := segment.newReader()
			var out bytes.Buffer
			for {
				data, eof := reader.readData()
				if len(data) > 0 {
					if _, err := out.Write(data); err != nil {
						t.Errorf("subscriber %d write error: %v", idx, err)
						return
					}
				}
				if eof {
					results[idx] = out.Bytes()
					return
				}
			}
		}(subscriberIdx)
	}

	for chunkIdx := 0; chunkIdx < chunks; chunkIdx++ {
		segment.writeData(payload)
	}
	segment.close()
	wg.Wait()

	for subscriberIdx, got := range results {
		if !bytes.Equal(got, want) {
			t.Fatalf("subscriber %d payload mismatch: got=%d want=%d", subscriberIdx, len(got), len(want))
		}
	}
}

func percentileSample(samples []int64, percentile int) int64 {
	if len(samples) == 0 {
		return 0
	}
	if percentile <= 0 {
		return samples[0]
	}
	if percentile >= 100 {
		return samples[len(samples)-1]
	}
	idx := (len(samples)*percentile + 99) / 100
	if idx <= 0 {
		idx = 1
	}
	return samples[idx-1]
}
