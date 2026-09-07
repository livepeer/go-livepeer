package media

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/pion/interceptor/pkg/stats"
	"github.com/pion/webrtc/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockPC mocks a webrtc.PeerConnection for testing
type MockPC struct {
	closeCalled bool
	closeErr    error
}

type mockRTPTrack struct {
	ssrc int
	kind webrtc.RTPCodecType
}

func (t *mockRTPTrack) SSRC() webrtc.SSRC {
	return webrtc.SSRC(t.ssrc)
}

func (t *mockRTPTrack) Kind() webrtc.RTPCodecType {
	return t.kind
}

func (t *mockRTPTrack) Codec() webrtc.RTPCodecParameters {
	return webrtc.RTPCodecParameters{}
}

type mockStatsGetter struct {
	statsMap map[uint32]*stats.Stats
}

func (m *mockStatsGetter) Get(ssrc uint32) *stats.Stats {
	return m.statsMap[ssrc]
}

func TestMediaStateStats(t *testing.T) {
	t.Run("ReturnsErrorWhenClosedOrNilPC", func(t *testing.T) {
		state := NewMediaState(nil)
		// Should fail because pc == nil
		_, err := state.Stats()
		assert.Error(t, err, "Expected error when pc is nil")

		mockPC := NewMockPC()
		stateWithPC := NewMediaState(mockPC)
		// Manually close
		stateWithPC.Close()
		_, err = stateWithPC.Stats()
		assert.Error(t, err, "Expected error when state is closed")
	})

	t.Run("ReturnsOnlyPeerConnectionStatsWhenGetterIsNil", func(t *testing.T) {
		mockPC := NewMockPC()
		mockState := NewMediaState(mockPC)

		// We can provide any custom logic if we need a mocked PC.GetStats()
		// For example, we don't actually have a valid StatsReport so let's assume nil is fine
		statsResult, err := mockState.Stats()
		require.NoError(t, err, "Did not expect an error from Stats() call")
		assert.NotNil(t, statsResult, "Expected a valid MediaStats result")
		assert.Nil(t, statsResult.TrackStats, "Expected no TrackStats when getter is nil")
	})

	t.Run("ReturnsTrackStatsWhenGetterIsPopulated", func(t *testing.T) {
		mockPC := NewMockPC()
		mockState := NewMediaState(mockPC)

		// Create a mock stats getter with two track stats
		msGetter := &mockStatsGetter{
			statsMap: map[uint32]*stats.Stats{
				123: {}, // a dummy stats object
				456: {},
			},
		}
		tracks := []SegmenterTrack{
			NewSegmenterTrack(&mockRTPTrack{ssrc: 123, kind: webrtc.RTPCodecTypeVideo}),
			NewSegmenterTrack(&mockRTPTrack{ssrc: 999, kind: webrtc.RTPCodecTypeAudio}), // will not be found
			NewSegmenterTrack(&mockRTPTrack{ssrc: 456, kind: webrtc.RTPCodecTypeVideo}),
		}
		mockState.SetTracks(msGetter, tracks)
		tracks[0].SetLastMpegtsTS(90_000)
		tracks[2].SetLastMpegtsTS(180_000)

		statsResult, err := mockState.Stats()
		require.NoError(t, err)
		assert.NotNil(t, statsResult)
		assert.NotNil(t, statsResult.TrackStats, "Expected TrackStats slice to be non-nil")
		assert.Equal(t, 2, len(statsResult.TrackStats), "Only two tracks should have been found in statsMap")
		assert.Equal(t, 1.0, statsResult.TrackStats[0].LastInputTS)
		assert.Equal(t, 2.0, statsResult.TrackStats[1].LastInputTS)
	})

	t.Run("ReturnsConnectionWarnings", func(t *testing.T) {
		mockPC := NewMockPC()
		mockState := NewMediaState(mockPC)

		// Create a mock stats getter with two track stats
		msGetter := &mockStatsGetter{
			statsMap: map[uint32]*stats.Stats{
				123: {
					InboundRTPStreamStats: stats.InboundRTPStreamStats{
						ReceivedRTPStreamStats: stats.ReceivedRTPStreamStats{
							PacketsLost:     1, // 1 packet lost and 2 received = 33.33% loss
							PacketsReceived: 2,
						},
					},
				},
				456: {},
			},
		}
		tracks := []SegmenterTrack{
			NewSegmenterTrack(&mockRTPTrack{ssrc: 123, kind: webrtc.RTPCodecTypeVideo}),
			NewSegmenterTrack(&mockRTPTrack{ssrc: 456, kind: webrtc.RTPCodecTypeVideo}),
		}
		mockState.SetTracks(msGetter, tracks)

		statsResult, err := mockState.Stats()
		require.NoError(t, err)

		require.Len(t, statsResult.TrackStats, 2)
		require.Len(t, statsResult.TrackStats[0].Warnings, 1)
		require.Equal(t, ConnQualityBad, statsResult.ConnQuality)
	})

	t.Run("HandlesNoStatsInGetter", func(t *testing.T) {
		mockPC := NewMockPC()
		mockState := NewMediaState(mockPC)

		msGetter := &mockStatsGetter{
			statsMap: map[uint32]*stats.Stats{},
		}
		tracks := []SegmenterTrack{
			NewSegmenterTrack(&mockRTPTrack{ssrc: 111, kind: webrtc.RTPCodecTypeVideo}),
		}
		mockState.SetTracks(msGetter, tracks)

		statsResult, err := mockState.Stats()
		require.NoError(t, err, "Expected no error even if tracks aren't found in getter")
		assert.NotNil(t, statsResult)
		assert.Empty(t, statsResult.TrackStats, "No track stats should be returned if they're not in getter")
	})

	t.Run("ConcurrentStatsCalls", func(t *testing.T) {
		mockPC := NewMockPC()
		mockState := NewMediaState(mockPC)

		// Set up a basic stats getter and some tracks
		msGetter := &mockStatsGetter{
			statsMap: map[uint32]*stats.Stats{
				111: {},
				222: {},
			},
		}
		tracks := []SegmenterTrack{
			NewSegmenterTrack(&mockRTPTrack{ssrc: 111, kind: webrtc.RTPCodecTypeVideo}),
			NewSegmenterTrack(&mockRTPTrack{ssrc: 222, kind: webrtc.RTPCodecTypeAudio}),
		}
		mockState.SetTracks(msGetter, tracks)

		// Call Stats concurrently while also closing
		var wg sync.WaitGroup
		const goroutines = 10
		wg.Add(goroutines)

		for i := 0; i < goroutines; i++ {
			go func() {
				defer wg.Done()
				// If the state isn't closed yet, Stats() should succeed
				// If it has just been closed, Stats() can return an error
				statsResult, err := mockState.Stats()
				if mockState.IsClosed() {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
					assert.NotNil(t, statsResult)
				}
			}()
		}

		// Let some Stats() calls proceed, then close the state
		time.Sleep(50 * time.Millisecond)
		mockState.Close()

		wg.Wait()
	})
}

func NewMockPC() *MockPC {
	return &MockPC{}
}

func (m *MockPC) Close() error {
	m.closeCalled = true
	return m.closeErr
}

func (m *MockPC) GetStats() webrtc.StatsReport {
	return nil
}

func (m *MockPC) WasCloseCalled() bool {
	return m.closeCalled
}

func (m *MockPC) SetCloseError(err error) {
	m.closeErr = err
}

func TestMediaState(t *testing.T) {
	t.Run("NewMediaState", func(t *testing.T) {
		mockPC := NewMockPC()
		state := NewMediaState(mockPC)

		assert.Equal(t, mockPC, state.pc, "Closer should be set correctly")
		assert.NotNil(t, state.cond, "Cond should be initialized")
	})

	t.Run("Close", func(t *testing.T) {
		mockPC := NewMockPC()
		state := NewMediaState(mockPC)

		state.Close()

		assert.True(t, mockPC.WasCloseCalled(), "Close should be called on the closer")

		// Test that calling Close again doesn't call the underlying Close again
		mockPC.closeCalled = false
		state.Close()

		assert.False(t, mockPC.WasCloseCalled(), "Close should not be called again")
	})

	t.Run("CloseWithError", func(t *testing.T) {
		mockPC := NewMockPC()
		mockPC.SetCloseError(errors.New("test error"))
		state := NewMediaState(mockPC)

		state.Close()

		assert.True(t, mockPC.WasCloseCalled(), "Close should be called on the closer even if it returns an error")
	})

	t.Run("AwaitClose", func(t *testing.T) {
		mockPC := NewMockPC()
		state := NewMediaState(mockPC)

		// Start a goroutine that will close after a short delay
		go func() {
			time.Sleep(100 * time.Millisecond)
			state.Close()
		}()

		// This should block until Close is called
		done := make(chan bool)
		go func() {
			state.AwaitClose()
			done <- true
		}()

		select {
		case <-done:
			// Test passed
		case <-time.After(500 * time.Millisecond):
			assert.Fail(t, "AwaitClose did not return after Close was called")
		}
	})
}

func TestNewMediaStateError(t *testing.T) {
	t.Run("CreatesAlreadyClosedState", func(t *testing.T) {
		testErr := errors.New("test error")
		state := NewMediaStateError(testErr)

		// The state should be immediately closed.
		assert.True(t, state.closed, "MediaState should be closed immediately when created with error")

		// The state should store the passed-in error properly.
		assert.Equal(t, testErr, state.err, "MediaState should have the error passed at creation")

		// Ensure calling Close again does not overwrite the error or reopen the state.
		state.Close()
		assert.True(t, state.closed, "MediaState must remain closed after subsequent Close")
		assert.Equal(t, testErr, state.err, "Error should remain unchanged after subsequent Close")

		// Ensure calling CloseError again does not overwrite the error or reopen the state.
		state.CloseError(errors.New("another error"))
		assert.True(t, state.closed, "MediaState must remain closed after subsequent Close")
		assert.Equal(t, testErr, state.err, "Error should remain unchanged after subsequent Close")

		// pc should be nil since we passed nil to NewMediaState.
		assert.Nil(t, state.pc, "PeerConnection should be nil for NewMediaStateError")
	})

	t.Run("AwaitCloseReturnsError", func(t *testing.T) {
		testErr := errors.New("another test error")
		state := NewMediaStateError(testErr)

		// AwaitClose should return immediately with the stored error, since it's already closed.
		returnedErr := state.AwaitClose()
		assert.Equal(t, testErr, returnedErr, "AwaitClose should return the stored error when closed immediately")
	})

	t.Run("ConcurrentAwaitCloseReturnsError", func(t *testing.T) {
		testErr := errors.New("another test error")
		state := NewMediaStateError(testErr)
		conn := NewWHIPConnection()
		conn.SetWHIPConnection(state)

		// AwaitClose should return immediately with the stored error, since it's already closed.
		var wg sync.WaitGroup
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				returnedErr := conn.AwaitClose()
				assert.Equal(t, testErr, returnedErr, "AwaitClose should return the stored error when closed immediately")
			}()
		}
		assert.True(t, wgWait(&wg), "timed out waiting for closes")
	})
}

func TestMediaStateCloseError(t *testing.T) {
	t.Run("ClosesPCIfNotAlreadyClosed", func(t *testing.T) {
		mockPC := NewMockPC()
		state := NewMediaState(mockPC)

		err := errors.New("some error")
		state.CloseError(err)

		assert.True(t, state.closed, "State should be marked closed")
		assert.Equal(t, err, state.err, "State should hold the provided error")
		assert.True(t, mockPC.WasCloseCalled(), "Close should be called on the peer connection")
	})

	t.Run("WhipConnectionNoDoubleCloseOnSubsequentCalls", func(t *testing.T) {
		mockPC := NewMockPC()
		state := NewMediaState(mockPC)
		conn := NewWHIPConnection()
		err1 := errors.New("first error")

		conn.SetWHIPConnection(state)
		state.CloseError(err1)
		assert.True(t, mockPC.WasCloseCalled(), "Peer connection should be closed the first time")
		assert.Equal(t, err1, conn.AwaitClose(), "Error should match the first provided error")

		// Reset the closeCalled
		mockPC.closeCalled = false
		err2 := errors.New("second error")
		state.CloseError(err2)

		assert.False(t, mockPC.WasCloseCalled(), "Peer connection should not be closed again")
		assert.Equal(t, err1, conn.AwaitClose(), "Error should remain the original error after subsequent calls")

		// Close via WHIP
		conn.Close()
		assert.False(t, mockPC.WasCloseCalled(), "Peer connection should not be closed again")
		assert.Equal(t, err1, conn.AwaitClose(), "Error should remain the original error after subsequent calls")
	})

	t.Run("WHIPConnectionConcurrentCloseError", func(t *testing.T) {
		mockPC := NewMockPC()
		state := NewMediaState(mockPC)
		conn := NewWHIPConnection()

		var wg sync.WaitGroup
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				err := conn.AwaitClose()
				assert.Error(t, err, "concurrent error")
			}()
		}
		time.Sleep(100 * time.Millisecond)
		conn.SetWHIPConnection(state)
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				state.CloseError(errors.New("concurrent error"))
			}()
		}
		assert.True(t, wgWait(&wg), "close errors timed out")

		// After concurrent CloseError calls, the peer should be closed exactly once
		assert.True(t, state.closed, "State should be marked closed")
		assert.True(t, mockPC.WasCloseCalled(), "Peer connection must have been closed")
		assert.Error(t, state.err, "concurrent error")
	})

	t.Run("RetainsFirstError", func(t *testing.T) {
		mockPC := NewMockPC()
		state := NewMediaState(mockPC)

		err1 := errors.New("first error")
		err2 := errors.New("second error")

		state.CloseError(err1)
		state.CloseError(err2)

		assert.Equal(t, err1, state.err, "Should retain the first passed-in error")
	})
}

func TestNewWHIPConnection(t *testing.T) {
	// Test case: Verify that NewWHIPConnection initializes correctly
	conn := NewWHIPConnection()

	assert.NotNil(t, conn.mu, "Mutex should be initialized")
	assert.NotNil(t, conn.cond, "Condition variable should be initialized")
	assert.Nil(t, conn.peer, "Peer should be nil initially")
	assert.False(t, conn.closed, "Closed should be false initially")
}

func TestSetWHIPConnection(t *testing.T) {
	// Test case: Verify that SetWHIPConnection properly sets the peer
	conn := NewWHIPConnection()
	mockPC := NewMockPC()
	mediaState := NewMediaState(mockPC)

	conn.SetWHIPConnection(mediaState)

	assert.Equal(t, mediaState, conn.peer, "Peer should be set to mediaState")
}

func TestGetWHIPConnection(t *testing.T) {
	// Test case 1: When peer is already set
	t.Run("PeerAlreadySet", func(t *testing.T) {
		conn := NewWHIPConnection()
		mockPC := NewMockPC()
		mediaState := NewMediaState(mockPC)

		conn.SetWHIPConnection(mediaState)
		result := conn.getWHIPConnection()

		assert.Equal(t, mediaState, result, "getWHIPConnection should return the set peer")
	})

	// Test case 2: When connection is closed before peer is set
	t.Run("ConnectionClosedBeforePeerSet", func(t *testing.T) {
		conn := NewWHIPConnection()
		conn.Close()
		result := conn.getWHIPConnection()
		if result != nil {
			t.Error("Expected getWHIPConnection to return nil when closed")
		}
	})

	// Test case 3: When peer is set while waiting
	t.Run("PeerSetWhileWaiting", func(t *testing.T) {
		conn := NewWHIPConnection()
		mockPC := NewMockPC()
		mediaState := NewMediaState(mockPC)

		// Start a goroutine that will set the peer after a short delay
		go func() {
			time.Sleep(100 * time.Millisecond)
			conn.SetWHIPConnection(mediaState)
		}()

		// This should block until the peer is set
		result := conn.getWHIPConnection()

		assert.Equal(t, mediaState, result, "getWHIPConnection should return the peer once set")
	})

	// Test case 4: When connection is closed while waiting
	t.Run("ConnectionClosedWhileWaiting", func(t *testing.T) {
		conn := NewWHIPConnection()

		// Start a goroutine that will close the connection after a short delay
		go func() {
			time.Sleep(100 * time.Millisecond)
			conn.Close()
		}()

		// This should block until the connection is closed
		result := conn.getWHIPConnection()

		assert.Nil(t, result, "getWHIPConnection should return nil when closed while waiting")
	})
}

func TestAwaitClose(t *testing.T) {
	// Test case 1: When peer is nil
	t.Run("NilPeer", func(t *testing.T) {
		conn := NewWHIPConnection()

		// Mark as closed to avoid blocking
		conn.Close()

		// This should return immediately without error
		conn.AwaitClose()
		// If we reach here, the test passes
	})

	// Test case 2: When peer is set and we await its closure
	t.Run("AwaitPeerClosure", func(t *testing.T) {
		conn := NewWHIPConnection()
		mockPC := NewMockPC()
		mediaState := NewMediaState(mockPC)

		conn.SetWHIPConnection(mediaState)

		// Start a goroutine that will close the peer after a short delay
		go func() {
			time.Sleep(100 * time.Millisecond)
			mediaState.Close()
		}()

		// This should block until the peer is closed
		done := make(chan bool)
		go func() {
			conn.AwaitClose()
			done <- true
		}()

		select {
		case <-done:
			// Test passed
			assert.True(t, true, "AwaitClose should return after peer is closed")
		case <-time.After(500 * time.Millisecond):
			assert.Fail(t, "AwaitClose did not return after peer was closed")
		}
	})

	// Test case 3: Multiple goroutines awaiting closure
	t.Run("MultipleAwaiters", func(t *testing.T) {
		conn := NewWHIPConnection()
		mockPC := NewMockPC()
		mediaState := NewMediaState(mockPC)

		conn.SetWHIPConnection(mediaState)

		// Start multiple goroutines that will await closure
		const numGoroutines = 12
		done := make(chan bool, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func() {
				conn.AwaitClose()
				done <- true
			}()
		}

		// Close the peer after a short delay
		go func() {
			time.Sleep(100 * time.Millisecond)
			mediaState.Close()
		}()

		// All goroutines should be notified
		for i := 0; i < numGoroutines; i++ {
			select {
			case <-done:
				// One goroutine completed
			case <-time.After(500 * time.Millisecond):
				assert.Fail(t, "Not all goroutines were notified of closure")
				return
			}
		}
	})

	// Test case 4: Awaiting after already closed
	t.Run("AwaitAfterClosed", func(t *testing.T) {
		conn := NewWHIPConnection()
		mockPC := NewMockPC()
		mediaState := NewMediaState(mockPC)

		conn.SetWHIPConnection(mediaState)
		mediaState.Close() // Close immediately

		// This should return immediately
		done := make(chan bool)
		go func() {
			conn.AwaitClose()
			done <- true
		}()

		select {
		case <-done:
			// Test passed
		case <-time.After(500 * time.Millisecond):
			assert.Fail(t, "AwaitClose did not return immediately for already closed peer")
		}
	})

	// Test case 5: Delayed peer connection setup before closing
	t.Run("DelayedPeerSetupBeforeClose", func(t *testing.T) {
		conn := NewWHIPConnection()

		// Start multiple goroutines that will await closure
		const numGoroutines = 12
		done := make(chan bool, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func() {
				conn.AwaitClose()
				done <- true
			}()
		}

		// Set up peer connection after a delay
		go func() {
			time.Sleep(50 * time.Millisecond)
			mockPC := NewMockPC()
			mediaState := NewMediaState(mockPC)
			conn.SetWHIPConnection(mediaState)

			// Then close it after another delay
			time.Sleep(50 * time.Millisecond)
			mediaState.Close()
		}()

		// All goroutines should be notified
		for i := 0; i < numGoroutines; i++ {
			select {
			case <-done:
				// One goroutine completed
			case <-time.After(500 * time.Millisecond):
				assert.Fail(t, "Not all goroutines were notified of closure with delayed setup")
				return
			}
		}
	})

	// Test case 6: Close without setting peer connection
	t.Run("CloseWithoutPeer", func(t *testing.T) {
		conn := NewWHIPConnection()

		// Start a goroutine that will await closure
		done := make(chan bool)
		go func() {
			conn.AwaitClose()
			done <- true
		}()

		// Close the connection without setting a peer
		go func() {
			time.Sleep(100 * time.Millisecond)
			conn.Close()
		}()

		select {
		case <-done:
			// Test passed
		case <-time.After(500 * time.Millisecond):
			assert.Fail(t, "AwaitClose did not return when connection was closed without a peer")
		}
	})

	// Test case 7: Null peer connection
	t.Run("NullPeerConnection", func(t *testing.T) {
		conn := NewWHIPConnection()
		mediaState := NewMediaState(nil) // Null peer connection

		conn.SetWHIPConnection(mediaState)

		// This should not panic
		require.NotPanics(t, func() {
			// Start a goroutine that will await closure
			done := make(chan bool)
			go func() {
				conn.AwaitClose()
				done <- true
			}()

			// Close the media state
			mediaState.Close()

			select {
			case <-done:
				// Test passed
			case <-time.After(500 * time.Millisecond):
				assert.Fail(t, "AwaitClose did not return for null peer connection")
			}
		})
	})

	// Test case 7: Null mediastate
	t.Run("NullMediaState", func(t *testing.T) {
		conn := NewWHIPConnection()
		conn.SetWHIPConnection(nil)

		// This should not panic
		require.NotPanics(t, func() {
			// Start a goroutine that will await closure
			done := make(chan bool)
			go func() {
				conn.AwaitClose()
				done <- true
			}()

			select {
			case <-done:
				// Test passed
			case <-time.After(500 * time.Millisecond):
				assert.Fail(t, "AwaitClose did not return for null peer connection")
			}
		})
	})

}

func TestClose(t *testing.T) {
	// Test case 1: When peer is nil
	t.Run("NilPeer", func(t *testing.T) {
		conn := NewWHIPConnection()

		// This should return immediately without error
		conn.Close()

		assert.True(t, conn.closed, "Connection should be marked as closed")
	})

	// Test case 2: When peer is set
	t.Run("PeerSet", func(t *testing.T) {
		conn := NewWHIPConnection()
		mockPC := NewMockPC()
		mediaState := NewMediaState(mockPC)

		conn.SetWHIPConnection(mediaState)
		conn.Close()

		assert.True(t, mockPC.WasCloseCalled(), "Close should be called on the peer")
		assert.True(t, conn.closed, "Connection should be marked as closed")
	})

	// Test case 3: Verify that Close broadcasts to waiting goroutines
	t.Run("BroadcastToWaiters", func(t *testing.T) {
		conn := NewWHIPConnection()
		mockPC := NewMockPC()
		mediaState := NewMediaState(mockPC)

		conn.SetWHIPConnection(mediaState)

		// Start a goroutine that will be waiting on the condition variable
		waitDone := make(chan bool)
		go func() {
			conn.mu.Lock()
			for !conn.closed {
				conn.cond.Wait()
			}
			conn.mu.Unlock()
			waitDone <- true
		}()

		// Close the connection
		conn.Close()

		// Verify that the waiting goroutine was signaled
		select {
		case <-waitDone:
			// Test passed
			assert.True(t, true, "Close should broadcast to waiting goroutines")
		case <-time.After(500 * time.Millisecond):
			assert.Fail(t, "Close did not broadcast to waiting goroutines")
		}
	})

	// Test case 4: Verify that multiple calls to Close work correctly
	t.Run("MultipleCloseCalls", func(t *testing.T) {
		conn := NewWHIPConnection()
		mockPC := NewMockPC()
		mediaState := NewMediaState(mockPC)

		conn.SetWHIPConnection(mediaState)

		// Call Close multiple times
		require.NotPanics(t, func() {
			conn.Close()
			conn.Close()
		}, "Multiple calls to Close should not panic")
	})
}

func TestConcurrentOperations(t *testing.T) {
	// Test case: Verify that concurrent operations don't cause race conditions
	conn := NewWHIPConnection()

	// Start multiple goroutines that perform operations concurrently
	const numGoroutines = 10
	var wg sync.WaitGroup
	wg.Add(numGoroutines * 4)

	// Some goroutines get the peer
	for i := 0; i < numGoroutines; i++ {
		go func(i int) {
			defer wg.Done()
			_ = conn.getWHIPConnection()
		}(i)
	}

	// Some goroutines close the connection
	for i := 0; i < numGoroutines; i++ {
		go func(i int) {
			defer wg.Done()
			assert.Nil(t, conn.AwaitClose(), "expected await close to not return an error")
		}(i)
	}

	// Some goroutines set the peer
	for i := 0; i < numGoroutines; i++ {
		go func(i int) {
			defer wg.Done()
			mockPC := NewMockPC()
			mediaState := NewMediaState(mockPC)
			conn.SetWHIPConnection(mediaState)
		}(i)
	}

	// Some goroutines close the connection
	for i := 0; i < numGoroutines; i++ {
		go func(i int) {
			defer wg.Done()
			conn.Close()
		}(i)
	}

	doneCh := make(chan bool)
	go func() {
		require.NotPanics(t, func() {
			wg.Wait()
		}, "Concurrent operations should not cause deadlocks or panics")
		close(doneCh)
	}()

	select {
	case <-doneCh:
	// Test passed
	case <-time.After(500 * time.Millisecond):
		assert.Fail(t, "Goroutines did not complete on time")
	}

}
