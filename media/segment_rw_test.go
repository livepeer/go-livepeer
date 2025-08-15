package media

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// --- helpers ---

func payload(seq int, bodyLen int) []byte {
	var b bytes.Buffer
	fmt.Fprintf(&b, "SEG#%08d:", seq)
	if bodyLen > 0 {
		b.Write(bytes.Repeat([]byte{byte('A' + (seq % 26))}, bodyLen))
	}
	return b.Bytes()
}

func readAll(rc io.ReadCloser) ([]byte, error) {
	defer rc.Close()
	all, err := io.ReadAll(rc)
	return all, err
}

func writeSegment(t *testing.T, rw *SegmentWriter, seq, bodyLen int) []byte {
	t.Helper()
	w, err := rw.Next()
	require.Nil(t, err, "Next() failed")
	defer w.Close()
	data := payload(seq, bodyLen)
	n, err := w.Write(data)
	require.Nil(t, err, "write failed")
	require.Equal(t, len(data), n, "short write")
	return data
}

func TestSegmentWriter_BasicWriteRead(t *testing.T) {
	require := require.New(t)
	rw := NewSegmentWriter(3)
	rr := rw.MakeReader(SegmentReaderConfig{})
	rc, err := rr.Next()
	require.Nil(err)
	writeSegment(t, rw, 1, 1)
	got, err := readAll(rc)
	require.Nil(err)
	require.Equal(string(payload(1, 1)), string(got))
}

func TestSegmentWriter_MultipleWrites(t *testing.T) {

	// Test multiple concurrent writes.
	// Not sure if this is a good idea but here it is.
	// If this is a problem in practice, then close the writer on Next()

	require := require.New(t)
	rw := NewSegmentWriter(2)
	rr := rw.MakeReader(SegmentReaderConfig{})

	// First writer
	r1, err := rr.Next()
	require.Nil(err)
	w1, err := rw.Next()
	require.Nil(err, "Next() for w1 must succeed")
	r2, err := rr.Next()
	require.Nil(err)
	w2, err := rw.Next()
	require.Nil(err, "Next() for w2 must succeed")

	// write the new writer first
	_, err = w2.Write(payload(2, 2))
	require.Nil(err)
	w2.Close()
	got, err := readAll(r2)
	require.Equal(string(payload(2, 2)), string(got))

	// now write the old writer
	_, err = w1.Write(payload(1, 1))
	require.Nil(err)
	w1.Close()
	got, err = readAll(r1)
	require.Equal(string(payload(1, 1)), string(got))
}

func TestSegmentWriter_WraparoundReads(t *testing.T) {
	require := require.New(t)
	rw := NewSegmentWriter(3)

	// Write 5 segments; reader will start before the last two and keep up.
	writeSegment(t, rw, 1, 8)
	writeSegment(t, rw, 2, 9)

	rr := rw.MakeReader(SegmentReaderConfig{})

	rc, err := rr.Next()
	require.Nil(err, "Next")
	got, err := readAll(rc)
	require.Nil(err)
	require.Equal(string(payload(2, 9)), string(got), "mismatch")

	writeSegment(t, rw, 3, 10)
	writeSegment(t, rw, 4, 11)

	_, err = rr.Next()
	require.Nil(err, "Next")

	// Reader sees 5, then advancing without new writes should fail.
	rc, err = rr.Next()
	require.Nil(err, "Next")
	got, err = readAll(rc)
	require.Nil(err)
	require.Equal(string(payload(4, 11)), string(got), "mismatch")

	rc, err = rr.Next()
	require.Nil(err)

	// check OOB for good measure
	_, err = rr.Next()
	require.EqualError(err, "segment out of bounds")

	// Write a couple more, forcing reader wraparound.
	writeSegment(t, rw, 5, 12)
	writeSegment(t, rw, 6, 13)
	writeSegment(t, rw, 7, 14)

	got, err = readAll(rc)
	require.Nil(err)
	require.Equal(string(payload(5, 12)), string(got), "mismatch")

	rc, err = rr.Next()
	require.Nil(err, "Next")
	got, err = readAll(rc)
	require.Nil(err)
	require.Equal(string(payload(6, 13)), string(got), "mismatch")
}

func TestSegmentWriter_SlowReader(t *testing.T) {
	// slow readers should error out

	rw := NewSegmentWriter(2)
	rr := rw.MakeReader(SegmentReaderConfig{})

	writeSegment(t, rw, 0, 8) // fills slot 0
	writeSegment(t, rw, 1, 8) // fills slot 1
	writeSegment(t, rw, 2, 8) // overwrites slot 0 (segment 0 lost)

	// Reader's first Next() attempts to read segment 0, which is now overwritten.
	_, err := rr.Next()
	require.EqualError(t, err, "reader fell behind")
}

func TestSegmentWriter_FastReader(t *testing.T) {
	rw := NewSegmentWriter(3)
	rr := rw.MakeReader(SegmentReaderConfig{})

	_, err := rr.Next()
	require.Nil(t, err, "unexpected error on first Next")

	_, err = rr.Next()
	require.EqualError(t, err, "segment out of bounds")
}

func TestSegmentWriter_ZeroLengthSegment(t *testing.T) {
	rw := NewSegmentWriter(2)

	w, err := rw.Next()
	require.Nil(t, err, "Next() for zero‐length segment must succeed")
	w.Close()

	rr := rw.MakeReader(SegmentReaderConfig{})
	rc, err := rr.Next()
	require.Nil(t, err, "Next")
	data, err := readAll(rc)
	require.Nil(t, err)
	require.Empty(t, string(data))
}

func TestSegmentWriter_StartReadAfterWrap(t *testing.T) {
	rw := NewSegmentWriter(3)

	for i := 0; i < 7; i++ {
		writeSegment(t, rw, i, 5)
	}

	rr := rw.MakeReader(SegmentReaderConfig{})
	rc, err := rr.Next()
	require.Nil(t, err, "Next")
	got, err := readAll(rc)
	require.Nil(t, err)
	require.Equal(t, payload(6, 5), got, "wrap mismatch")
}

func TestSegmentWriter_Concurrency(t *testing.T) {

	const (
		numReaders = 30
		// set so it writes a segment approx every 100ms
		// in practice this should be several *seconds* at a time
		totalBytes = 64 * 1024 * 10     // 640 KB write
		rate       = 100 * 5_00_000 / 8 // 50 Mbps
	)

	require := require.New(t)
	rw := NewSegmentWriter(4)

	var wg sync.WaitGroup
	hashes := [][]byte{}
	// 2-way handshake for stop
	stop := make(chan struct{})
	stop2 := make(chan struct{})

	// Writer produces a segment every 100ms or so
	go func() {
		for {
			select {
			case <-stop:
				close(stop2)
				return
			default:
			}
			pub, err := rw.Next()
			require.Nil(err, "Next() in concurrency writer must succeed")
			writerHash := sha256.New()
			src := io.LimitReader(&patternReader{}, totalBytes) // only totalBytes
			tee := io.TeeReader(src, writerHash)                // also feed the hash
			tw := &throttledWriter{w: io.Writer(pub), rate: rate}
			n, err := io.Copy(tw, tee)
			require.Equal(int64(totalBytes), n)
			require.Nil(err)
			pub.Close()
			hashes = append(hashes, writerHash.Sum(nil))
		}
	}()

	// Let the writer get ahead a bit
	time.Sleep(30 * time.Millisecond)

	readerHashes := make([][]struct {
		seq int
		res []byte
	}, numReaders)

	// consume ~10 segments per reader
	wg.Add(numReaders)
	for j := 0; j < numReaders; j++ {
		go func(nb int) {
			defer wg.Done()
			rr := rw.MakeReader(SegmentReaderConfig{})
			hashes := readerHashes[nb]
			lastSeen := -1
			for i := 0; i < 10; i++ {
				hasher := sha256.New()
				rc, err := rr.Next()
				require.Nil(err)
				n, err := io.Copy(hasher, rc)
				require.Nil(err)
				seq := rc.Seq()
				require.Equal(int64(totalBytes), n, fmt.Sprintf("seq %d", seq))
				hashes = append(hashes, struct {
					seq int
					res []byte
				}{seq, hasher.Sum(nil)})
				if lastSeen != -1 {
					require.Equal(lastSeen+1, seq)
				}
				lastSeen = seq
			}
			readerHashes[nb] = hashes
		}(j)
	}

	wg.Wait()
	close(stop)
	<-stop2

	for i, rh := range readerHashes {
		for _, h := range rh {
			require.Equal(hashes[h.seq], h.res, fmt.Sprintf("mismatch %d/%d", i, h.seq))
		}
	}
}

func TestSegmentWriter_Close(t *testing.T) {
	require := require.New(t)

	sw := NewSegmentWriter(5)
	rr := sw.MakeReader(SegmentReaderConfig{})

	for i := 0; i < 3; i++ {
		writeSegment(t, sw, i, i)
	}

	// close the writer (second close is no-op)
	require.Nil(sw.Close())
	require.Nil(sw.Close())

	// writer.Next() now must EOF
	_, err := sw.Next()
	require.ErrorIs(err, io.EOF)

	// reader can still read up until last segment
	for seq := 0; seq < 3; seq++ {
		rc, err := rr.Next()
		require.NoError(err, "reader Next up to last seq")
		// consume and discard payload
		_, err = io.Copy(io.Discard, rc)
		require.Nil(err)
	}

	// any later reads must EOF
	_, err = rr.Next()
	require.ErrorIs(err, io.EOF, "reader Next past last seq after Close should ErrClosed")
}
