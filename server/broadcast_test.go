package server

import (
	"errors"
	"testing"

	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/drivers"
	"github.com/livepeer/go-livepeer/net"

	"github.com/stretchr/testify/assert"
)

func TestStopSessionErrors(t *testing.T) {

	// check error cases
	errs := []string{
		"Unable to read response body for segment 4 : unexpected EOF",
		"Unable to submit segment 5 Post https://127.0.0.1:8936/segment: dial tcp 127.0.0.1:8936: getsockopt: connection refused",
		core.ErrOrchBusy.Error(),
		core.ErrOrchCap.Error(),
	}

	// Sanity check that we're checking each failure case
	if len(errs) != len(sessionErrStrings) {
		t.Error("Mismatched error cases for stop session")
	}
	for _, v := range errs {
		if !shouldStopSession(errors.New(v)) {
			t.Error("Should have stopped session but didn't: ", v)
		}
	}

	// check non-error cases
	errs = []string{
		"",
		"not really an error",
	}
	for _, v := range errs {
		if shouldStopSession(errors.New(v)) {
			t.Error("Should not have stopped session but did: ", v)
		}
	}

}

func TestNewSessionManager(t *testing.T) {
	n, _ := core.NewLivepeerNode(nil, "", nil)
	assert := assert.New(t)

	mid := core.RandomManifestID()
	storage := drivers.NewMemoryDriver(nil).NewSession(string(mid))
	pl := core.NewBasicPlaylistManager(mid, storage)

	// Check empty pool produces expected numOrchs
	sess := NewSessionManager(n, pl)
	assert.Equal(0, sess.numOrchs)

	// Check numOrchs up to maximum and a bit beyond
	sd := &stubDiscovery{}
	n.OrchestratorPool = sd
	max := int(HTTPTimeout.Seconds()/SegLen.Seconds()) * 2
	for i := 0; i < 10; i++ {
		sess = NewSessionManager(n, pl)
		if i < max {
			assert.Equal(i, sess.numOrchs)
		} else {
			assert.Equal(max, sess.numOrchs)
		}
		sd.infos = append(sd.infos, &net.OrchestratorInfo{})
	}
	// sanity check some expected postconditions
	assert.Equal(sess.numOrchs, max)
	assert.True(sd.Size() > max, "pool should be greater than max numOrchs")
}

// Note: Add processSegment tests, including:
//     assert an error from transcoder removes sess from BroadcastSessionManager
//     assert a success re-adds sess to BroadcastSessionManager
