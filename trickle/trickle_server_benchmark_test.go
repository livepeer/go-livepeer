package trickle

import (
	"bytes"
	"fmt"
	"io"
	"sync"
	"testing"
)

func BenchmarkSegmentFanout(b *testing.B) {
	for _, subscribers := range []int{1, 2, 5, 15, 40} {
		b.Run(fmt.Sprintf("subscribers=%d", subscribers), func(b *testing.B) {
			benchmarkSegmentFanout(b, subscribers)
		})
	}
}

func BenchmarkSegmentFanoutSmallWrites(b *testing.B) {
	for _, subscribers := range []int{1, 2, 5, 15, 40} {
		b.Run(fmt.Sprintf("subscribers=%d", subscribers), func(b *testing.B) {
			benchmarkSegmentFanoutWithWriteSize(b, subscribers, newCurrentLatencySegment, 8*1024*1024, 1300)
		})
	}
}

func BenchmarkSegmentFanoutVariants(b *testing.B) {
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
			b.Run(fmt.Sprintf("%s/subscribers=%d", variant.name, subscribers), func(b *testing.B) {
				benchmarkSegmentFanoutVariant(b, subscribers, variant.new)
			})
		}
	}
}

func BenchmarkSegmentFanoutVariantsSmallWrites(b *testing.B) {
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
			b.Run(fmt.Sprintf("%s/subscribers=%d", variant.name, subscribers), func(b *testing.B) {
				benchmarkSegmentFanoutWithWriteSize(b, subscribers, variant.new, 8*1024*1024, 1300)
			})
		}
	}
}

func benchmarkSegmentFanout(b *testing.B, subscribers int) {
	benchmarkSegmentFanoutWithWriteSize(b, subscribers, newCurrentLatencySegment, 8*1024*1024, 32*1024)
}

func benchmarkSegmentFanoutVariant(b *testing.B, subscribers int, newSegment func() latencySegment) {
	benchmarkSegmentFanoutWithWriteSize(b, subscribers, newSegment, 8*1024*1024, 32*1024)
}

func benchmarkSegmentFanoutWithWriteSize(b *testing.B, subscribers int, newSegment func() latencySegment, totalBytes int, writeSize int) {
	writeSizes := buildWriteSizes(totalBytes, writeSize)
	payloads := make([][]byte, len(writeSizes))
	for i, size := range writeSizes {
		payloads[i] = bytes.Repeat([]byte("segment-benchmark-payload-"), (size/26)+1)[:size]
	}

	b.SetBytes(int64(totalBytes * subscribers))
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		segment := newSegment()

		var (
			wg      sync.WaitGroup
			results = make([]int, subscribers)
		)

		wg.Add(subscribers)
		for subscriberIdx := 0; subscriberIdx < subscribers; subscriberIdx++ {
			go func(idx int) {
				defer wg.Done()
				reader := segment.newReader()
				totalRead := 0
				for {
					data, eof := reader.readData()
					totalRead += len(data)
					if eof {
						results[idx] = totalRead
						return
					}
				}
			}(subscriberIdx)
		}

		for _, payload := range payloads {
			segment.writeData(payload)
		}
		segment.close()
		wg.Wait()

		for subscriberIdx, totalRead := range results {
			if totalRead != totalBytes {
				b.Fatalf("subscriber %d read %d bytes, want %d", subscriberIdx, totalRead, totalBytes)
			}
		}
	}
}

func buildWriteSizes(totalBytes int, writeSize int) []int {
	if totalBytes <= 0 || writeSize <= 0 {
		return nil
	}

	writes := make([]int, 0, (totalBytes/writeSize)+1)
	remaining := totalBytes
	for remaining > 0 {
		size := writeSize
		if remaining < size {
			size = remaining
		}
		writes = append(writes, size)
		remaining -= size
	}
	return writes
}

func TestSegmentSubscriberConcurrentRead(t *testing.T) {
	testSegmentSubscriberConcurrentRead(t, newCurrentLatencySegment, 2*1024*1024, 32*1024, 15)
}

func TestSegmentSubscriberConcurrentReadSmallWrites(t *testing.T) {
	testSegmentSubscriberConcurrentRead(t, newCurrentLatencySegment, 2*1024*1024, 1300, 15)
}

func testSegmentSubscriberConcurrentRead(t *testing.T, newSegment func() latencySegment, totalBytes int, writeSize int, subscribers int) {
	const (
		payloadPattern = "segment-test-payload-"
	)

	writeSizes := buildWriteSizes(totalBytes, writeSize)
	payloads := make([][]byte, len(writeSizes))
	want := make([]byte, 0, totalBytes)
	for i, size := range writeSizes {
		payloads[i] = bytes.Repeat([]byte(payloadPattern), (size/len(payloadPattern))+1)[:size]
		want = append(want, payloads[i]...)
	}
	segment := newSegment()

	var (
		wg      sync.WaitGroup
		results = make([][]byte, subscribers)
	)

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

	for _, payload := range payloads {
		segment.writeData(payload)
	}
	segment.close()
	wg.Wait()

	for subscriberIdx, got := range results {
		if !bytes.Equal(got, want) {
			t.Fatalf("subscriber %d payload mismatch: got=%d want=%d", subscriberIdx, len(got), len(want))
		}
	}

	if _, err := io.Copy(io.Discard, bytes.NewReader(want)); err != nil {
		t.Fatalf("discard copy failed: %v", err)
	}
}
