package server

import (
	"errors"
	"testing"

	"github.com/livepeer/go-livepeer/core"
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

// Note: Add processSegment tests, including:
//     assert an error from transcoder removes sess from BroadcastSessionManager
//     assert a success re-adds sess to BroadcastSessionManager
