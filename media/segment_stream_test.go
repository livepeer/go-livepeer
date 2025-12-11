package media

import (
	"crypto/rand"
	"crypto/sha256"

	"io"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSegmentStreamReader_Concurrent(t *testing.T) {
	const (
		numReaders  = 10
		numSegments = 25
		bufferSize  = 1 * 1024 * 1024 // 1 MB
	)

	// Write a bunch of segments to a bunch of readers.
	// Compare bytes written / read, hashes, etc

	ssr := NewSwitchableSegmentReader()
	readers := make([]*SegmentStreamReader, numReaders)
	var wg sync.WaitGroup

	// Reader goroutines: read and verify data
	// Each reader should read all segments continuously
	readerHashes := make([][32]byte, numReaders)
	for i := 0; i < numReaders; i++ {
		readers[i] = NewSegmentStreamReader(ssr)
		wg.Go(func() {
			reader := readers[i]
			hasher := sha256.New()

			// Read continuously until EOF (when ssr closes)
			n, err := io.Copy(hasher, reader)
			if err != nil && err != io.EOF {
				require.NoError(t, err, "Read should not error")
			}
			assert.Equal(t, int64(numSegments*bufferSize), n, "Should read all bytes")

			// Store hash
			hash := hasher.Sum(nil)
			copy(readerHashes[i][:], hash)
		})
	}

	writerHasher := sha256.New()
	for i := 0; i < numSegments; i++ {
		mw := NewMediaWriter()
		ssr.Read(mw.MakeReader())

		// Use TeeReader to populate writer and hash simultaneously
		src := io.LimitReader(rand.Reader, bufferSize)
		tee := io.TeeReader(src, writerHasher)
		n, err := io.Copy(mw, tee)
		require.NoError(t, err, "Copy should succeed")
		assert.Equal(t, int64(bufferSize), n, "Should copy all bytes")

		err = mw.Close()
		require.NoError(t, err, "Close should succeed")
	}

	// Close the switchable reader to signal EOF to all readers
	ssr.Close()

	// Wait for all readers to complete
	require.True(t, wgWait(&wg), "Test should complete quickly")

	// Verify all readers got the correct total data
	expectedHash := writerHasher.Sum(nil)
	for readerIdx := 0; readerIdx < numReaders; readerIdx++ {
		assert.Equal(t, expectedHash, readerHashes[readerIdx][:],
			"Reader %d hash mismatch", readerIdx)
	}

	// Verify all readers get EOF after close
	for i := 0; i < numReaders; i++ {
		buf := make([]byte, 1024)
		n, err := readers[i].Read(buf)
		assert.Equal(t, 0, n, "Should read 0 bytes after close")
		assert.Equal(t, io.EOF, err, "Should get EOF after close")
	}
}
