package clog

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStdKeys(t *testing.T) {
	assert := assert.New(t)
	ctx := AddManifestID(context.Background(), "manID")
	ctx = AddSessionID(ctx, "sessionID")
	ctx = AddNonce(ctx, 1038)
	ctx = AddOrchSessionID(ctx, "orchID")
	ctx = AddSeqNo(ctx, 9427)
	ctx = AddVal(ctx, "customKey", "customVal")
	msg, _ := formatMessage(ctx, false, false, "testing message num=%d", 452)
	assert.Equal("manifestID=manID sessionID=sessionID nonce=1038 seqNo=9427 orchSessionID=orchID customKey=customVal testing message num=452", msg)
	ctxCloned := Clone(context.Background(), ctx)
	ctxCloned = AddManifestID(ctxCloned, "newManifest")
	msgCloned, _ := formatMessage(ctxCloned, false, false, "testing message num=%d", 4521)
	assert.Equal("manifestID=newManifest sessionID=sessionID nonce=1038 seqNo=9427 orchSessionID=orchID customKey=customVal testing message num=4521", msgCloned)
	// old context shouldn't change
	msg, _ = formatMessage(ctx, false, false, "testing message num=%d", 452)
	assert.Equal("manifestID=manID sessionID=sessionID nonce=1038 seqNo=9427 orchSessionID=orchID customKey=customVal testing message num=452", msg)
}

func TestLastErr(t *testing.T) {
	assert := assert.New(t)
	ctx := AddManifestID(context.Background(), "manID")
	var err error
	msg, isErr := formatMessage(ctx, true, false, "testing message num=%d", 452, err)
	assert.Equal("manifestID=manID testing message num=452", msg)
	assert.False(isErr)
	err = errors.New("test error")
	msg, isErr = formatMessage(ctx, true, false, "testing message num=%d", 452, err)
	assert.Equal("manifestID=manID testing message num=452 err=\"test error\"", msg)
	assert.True(isErr)
}

// Verify we do not leak contextual info inadvertently
func TestPublicLogs(t *testing.T) {
	assert := assert.New(t)
	// These should be visible:
	ctx := AddManifestID(context.Background(), "fooManID")
	ctx = AddSessionID(ctx, "fooSessionID")
	ctx = AddOrchSessionID(ctx, "fooOrchID")
	// These should not be visible:
	ctx = AddNonce(ctx, 999)
	ctx = AddSeqNo(ctx, 555)
	ctx = AddVal(ctx, "foo", "Bar")

	publicCtx := PublicCloneCtx(ctx, context.Background(), publicLogKeys)

	// Verify the keys in publicLogKeys list gets copied to logs:
	val := GetVal(publicCtx, manifestID)
	assert.Equal("fooManID", val)
	val = GetVal(publicCtx, sessionID)
	assert.Equal("fooSessionID", val)
	val = GetVal(publicCtx, orchSessionID)
	assert.Equal("fooOrchID", val)

	// Verify random keys cannot be leaked:
	val = GetVal(publicCtx, nonce)
	assert.Equal("", val)
	val = GetVal(publicCtx, seqNo)
	assert.Equal("", val)
	val = GetVal(publicCtx, "foo")
	assert.Equal("", val)

	// Verify [PublicLogs] gets pre-pended:
	msg, _ := formatMessage(ctx, false, true, "testing message num=%d", 123)
	assert.Equal("[PublicLogs] manifestID=fooManID sessionID=fooSessionID nonce=999 seqNo=555 orchSessionID=fooOrchID foo=Bar testing message num=123", msg)
}
