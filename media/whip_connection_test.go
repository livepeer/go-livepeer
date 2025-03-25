package media

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockPC mocks a webrtc.PeerConnection for testing
type MockPC struct {
	closeCalled bool
	closeErr    error
}

func NewMockPC() *MockPC {
	return &MockPC{}
}

func (m *MockPC) Close() error {
	m.closeCalled = true
	return m.closeErr
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
		assert.NotNil(t, state.closeCh, "CloseCh should be initialized")
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
			conn.AwaitClose()
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
