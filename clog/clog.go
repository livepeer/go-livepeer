/*
Package clog provides Conext with logging information.
*/
package clog

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/golang/glog"
)

// unique type to prevent assignment.
type clogContextKeyT struct{}

var clogContextKey = clogContextKeyT{}

const (
	// standard keys
	manifestID    = "manifestID"
	sessionID     = "sessionID"
	nonce         = "nonce"
	seqNo         = "seqNo"
	orchSessionID = "orchSessionID" // session id generated on orchestrator for broadcaster
)

// Verbose is a boolean type that implements Infof (like Printf) etc.
// See the documentation of V for more information.
type Verbose bool

var stdKeys map[string]bool
var stdKeysOrder = []string{manifestID, sessionID, nonce, seqNo}

func init() {
	stdKeys = make(map[string]bool)
	for _, key := range stdKeysOrder {
		stdKeys[key] = true
	}
}

func V(level glog.Level) Verbose {
	return Verbose(bool(glog.V(level)))
}

// Clone creates new context with parentCtx as parent and
// logging details from logCtx
func Clone(parentCtx, logCtx context.Context) context.Context {
	cmap, _ := logCtx.Value(clogContextKey).(map[string]string)
	newCmap := make(map[string]string)
	for k, v := range cmap {
		newCmap[k] = v
	}
	return context.WithValue(parentCtx, clogContextKey, newCmap)
}

func AddManifestID(ctx context.Context, val string) context.Context {
	return AddVal(ctx, manifestID, val)
}

func AddOrchSessionID(ctx context.Context, val string) context.Context {
	return AddVal(ctx, orchSessionID, val)
}

func AddSessionID(ctx context.Context, val string) context.Context {
	return AddVal(ctx, sessionID, val)
}

func AddNonce(ctx context.Context, val uint64) context.Context {
	return AddVal(ctx, nonce, strconv.FormatUint(val, 10))
}

func AddSeqNo(ctx context.Context, val uint64) context.Context {
	return AddVal(ctx, seqNo, strconv.FormatUint(val, 10))
}

func AddVal(ctx context.Context, key, val string) context.Context {
	cmap, _ := ctx.Value(clogContextKey).(map[string]string)
	if cmap == nil {
		cmap = make(map[string]string)
		ctx = context.WithValue(ctx, clogContextKey, cmap)
	}
	cmap[key] = val
	return ctx
}

func Warningf(ctx context.Context, format string, args ...interface{}) {
	glog.WarningDepth(1, formatMessage(ctx, format, args...))
}

func Errorf(ctx context.Context, format string, args ...interface{}) {
	glog.ErrorDepth(1, formatMessage(ctx, format, args...))
}

func Fatalf(ctx context.Context, format string, args ...interface{}) {
	glog.FatalDepth(1, formatMessage(ctx, format, args...))
}

func Infof(ctx context.Context, format string, args ...interface{}) {
	infof(ctx, format, args...)
}

func infof(ctx context.Context, format string, args ...interface{}) {
	glog.InfoDepth(2, formatMessage(ctx, format, args...))
}

// Infof is equivalent to the global Infof function, guarded by the value of v.
// See the documentation of V for usage.
func (v Verbose) Infof(ctx context.Context, format string, args ...interface{}) {
	if v {
		infof(ctx, format, args...)
	}
}

func messageFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	cmap, _ := ctx.Value(clogContextKey).(map[string]string)
	if cmap == nil {
		return ""
	}
	var parts []string
	for _, key := range stdKeysOrder {
		if val, ok := cmap[key]; ok {
			parts = append(parts, key+"="+val)
		}
	}
	for key, val := range cmap {
		if _, ok := stdKeys[key]; !ok {
			parts = append(parts, key+"="+val)
		}
	}
	return strings.Join(parts, " ")
}

func formatMessage(ctx context.Context, format string, args ...interface{}) string {
	msg := fmt.Sprintf(format, args...)
	mfc := messageFromContext(ctx)
	if mfc != "" {
		msg = mfc + " " + msg
	}
	return msg
}
