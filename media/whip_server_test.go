package media

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestWHIPServerDeleteResource(t *testing.T) {
	ctx := context.Background()
	s := &WHIPServer{}
	ms := NewMediaState(&MockPC{})
	s.track("res-1", ms)

	require.False(t, s.DeleteWHIP(ctx, "unknown"), "unknown resource")
	require.True(t, s.DeleteWHIP(ctx, "res-1"), "known resource")
	require.True(t, ms.IsClosed(), "connection closed")

	// The entry is dropped once the session ends, so the resource is gone
	// for any later request.
	require.Eventually(t, func() bool {
		return !s.DeleteWHIP(ctx, "res-1")
	}, time.Second, 10*time.Millisecond)
}

// A session that ends on its own must not leave its resource behind.
func TestWHIPServerUntracksOnClose(t *testing.T) {
	ctx := context.Background()
	s := &WHIPServer{}
	ms := NewMediaState(&MockPC{})
	s.track("res-2", ms)

	ms.Close()

	require.Eventually(t, func() bool {
		return !s.DeleteWHIP(ctx, "res-2")
	}, time.Second, 10*time.Millisecond)
}
