/*
Package clog provides Context with logging information.
*/
package clog

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/golang/glog"
)

// unique type to prevent assignment.
type clogContextKeyT struct{}

var clogContextKey = clogContextKeyT{}

const (
	ClientIP = "clientIP"

	// standard keys
	manifestID    = "manifestID"
	sessionID     = "sessionID"
	nonce         = "nonce"
	seqNo         = "seqNo"
	orchSessionID = "orchSessionID" // session id generated on orchestrator for broadcaster
)

// Verbose is a boolean type that implements Infof (like Printf) etc.
type Verbose bool

var stdKeys map[string]bool
var stdKeysOrder = []string{manifestID, sessionID, nonce, seqNo, orchSessionID}

func init() {
	stdKeys = make(map[string]bool)
	for _, key := range stdKeysOrder {
		stdKeys[key] = true
	}
}

type values struct {
	mu   sync.RWMutex
	vals map[string]string
}

func newValues() *values {
	return &values{
		vals: make(map[string]string),
	}
}

// Clone creates new context with parentCtx as parent and
// logging details from logCtx
func Clone(parentCtx, logCtx context.Context) context.Context {
	cmap, _ := logCtx.Value(clogContextKey).(*values)
	newCmap := newValues()
	if cmap != nil {
		cmap.mu.RLock()
		for k, v := range cmap.vals {
			newCmap.vals[k] = v
		}
		cmap.mu.RUnlock()
	}
	return context.WithValue(parentCtx, clogContextKey, newCmap)
}

func AddManifestID(ctx context.Context, val string) context.Context {
	return AddVal(ctx, manifestID, val)
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

func AddOrchSessionID(ctx context.Context, val string) context.Context {
	return AddVal(ctx, orchSessionID, val)
}

func AddVal(ctx context.Context, key, val string) context.Context {
	cmap, _ := ctx.Value(clogContextKey).(*values)
	if cmap == nil {
		cmap = newValues()
		ctx = context.WithValue(ctx, clogContextKey, cmap)
	}
	cmap.mu.Lock()
	cmap.vals[key] = val
	cmap.mu.Unlock()
	return ctx
}

func GetManifestID(ctx context.Context) string {
	return GetVal(ctx, manifestID)
}

func GetVal(ctx context.Context, key string) string {
	var val string
	cmap, _ := ctx.Value(clogContextKey).(*values)
	if cmap == nil {
		return val
	}
	cmap.mu.Lock()
	val = cmap.vals[key]
	cmap.mu.Unlock()
	return val
}

func Warningf(ctx context.Context, format string, args ...interface{}) {
	msg, _ := formatMessage(ctx, false, format, args...)
	glog.WarningDepth(1, msg)
}

func Errorf(ctx context.Context, format string, args ...interface{}) {
	msg, _ := formatMessage(ctx, false, format, args...)
	glog.ErrorDepth(1, msg)
}

func Fatalf(ctx context.Context, format string, args ...interface{}) {
	msg, _ := formatMessage(ctx, false, format, args...)
	glog.FatalDepth(1, msg)
}

func Infof(ctx context.Context, format string, args ...interface{}) {
	infof(ctx, false, format, args...)
}

// InfofErr if last argument is not nil it will be printed as " err=%q"
func InfofErr(ctx context.Context, format string, args ...interface{}) {
	infof(ctx, true, format, args...)
}

func V(level glog.Level) Verbose {
	return Verbose(glog.V(level))
}

// Infof is equivalent to the global Infof function, guarded by the value of v.
// See the documentation of V for usage.
func (v Verbose) Infof(ctx context.Context, format string, args ...interface{}) {
	if v {
		infof(ctx, false, format, args...)
	}
}

func (v Verbose) InfofErr(ctx context.Context, format string, args ...interface{}) {
	var err interface{}
	if len(args) > 0 {
		err = args[len(args)-1]
	}
	if v || err != nil {
		infof(ctx, true, format, args...)
	}
}

func infof(ctx context.Context, lastErr bool, format string, args ...interface{}) {
	msg, isErr := formatMessage(ctx, lastErr, format, args...)
	if isErr {
		glog.ErrorDepth(2, msg)
	} else {
		glog.InfoDepth(2, msg)
	}
}

func messageFromContext(ctx context.Context, sb *strings.Builder) {
	if ctx == nil {
		return
	}
	cmap, _ := ctx.Value(clogContextKey).(*values)
	if cmap == nil {
		return
	}
	cmap.mu.RLock()
	for _, key := range stdKeysOrder {
		if val, ok := cmap.vals[key]; ok {
			sb.WriteString(key)
			sb.WriteString("=")
			sb.WriteString(val)
			sb.WriteString(" ")
		}
	}
	for key, val := range cmap.vals {
		if _, ok := stdKeys[key]; !ok {
			sb.WriteString(key)
			sb.WriteString("=")
			sb.WriteString(val)
			sb.WriteString(" ")
		}
	}
	cmap.mu.RUnlock()
}

func formatMessage(ctx context.Context, lastErr bool, format string, args ...interface{}) (string, bool) {
	var sb strings.Builder
	messageFromContext(ctx, &sb)
	var err interface{}
	if lastErr && len(args) > 0 {
		err = args[len(args)-1]
		args = args[:len(args)-1]
	}
	sb.WriteString(fmt.Sprintf(format, args...))
	if err != nil {
		sb.WriteString(fmt.Sprintf(" err=%q", err))
	}
	return sb.String(), err != nil
}
