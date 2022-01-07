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
	msg, _ := formatMessage(ctx, false, "testing message num=%d", 452)
	assert.Equal("manifestID=manID sessionID=sessionID nonce=1038 seqNo=9427 orchSessionID=orchID customKey=customVal testing message num=452", msg)
	ctxCloned := Clone(context.Background(), ctx)
	ctxCloned = AddManifestID(ctxCloned, "newManifest")
	msgCloned, _ := formatMessage(ctxCloned, false, "testing message num=%d", 4521)
	assert.Equal("manifestID=newManifest sessionID=sessionID nonce=1038 seqNo=9427 orchSessionID=orchID customKey=customVal testing message num=4521", msgCloned)
	// old context shouldn't change
	msg, _ = formatMessage(ctx, false, "testing message num=%d", 452)
	assert.Equal("manifestID=manID sessionID=sessionID nonce=1038 seqNo=9427 orchSessionID=orchID customKey=customVal testing message num=452", msg)
}

func TestLastErr(t *testing.T) {
	assert := assert.New(t)
	ctx := AddManifestID(context.Background(), "manID")
	var err error
	msg, isErr := formatMessage(ctx, true, "testing message num=%d", 452, err)
	assert.Equal("manifestID=manID testing message num=452", msg)
	assert.False(isErr)
	err = errors.New("test error")
	msg, isErr = formatMessage(ctx, true, "testing message num=%d", 452, err)
	assert.Equal("manifestID=manID testing message num=452 err=\"test error\"", msg)
	assert.True(isErr)
}
