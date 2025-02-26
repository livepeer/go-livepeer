/*
Package clog provides Context with logging information.
*/
package clog

import (
	"context"
	"flag"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang/glog"
)

// unique type to prevent assignment.
type clogContextKeyT struct{}

var clogContextKey = clogContextKeyT{}

const (
	ClientIP     = "clientIP"
	publicLogTag = "[PublicLogs] "

	// standard keys
	manifestID    = "manifestID"
	sessionID     = "sessionID"
	nonce         = "nonce"
	seqNo         = "seqNo"
	orchSessionID = "orchSessionID" // session id generated on orchestrator for broadcaster
	ethaddress    = "ethaddress"
	orchestrator  = "orchestrator"
)

// Verbose is a boolean type that implements Infof (like Printf) etc.
type Verbose bool

var stdKeys map[string]bool
var stdKeysOrder = []string{manifestID, sessionID, nonce, seqNo, orchSessionID, ethaddress, orchestrator}
var publicLogKeys = []string{manifestID, sessionID, orchSessionID, ClientIP, seqNo, ethaddress, orchestrator}

func init() {
	stdKeys = make(map[string]bool)
	for _, key := range stdKeysOrder {
		stdKeys[key] = true
	}
	// Set default v level to 3; this is overridden in main() but is useful for tests
	vFlag := flag.Lookup("v")
	vFlag.Value.Set("3")
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

func WithTimeout(parentCtx, logCtx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	ctx := Clone(parentCtx, logCtx)
	return context.WithTimeout(ctx, timeout)
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
	if !glog.V(2) {
		return
	}
	msg, _ := formatMessage(ctx, false, false, format, args...)
	glog.WarningDepth(1, msg)
}

func Errorf(ctx context.Context, format string, args ...interface{}) {
	if !glog.V(1) {
		return
	}
	msg, _ := formatMessage(ctx, false, false, format, args...)
	glog.ErrorDepth(1, msg)
}

func Infof(ctx context.Context, format string, args ...interface{}) {
	infof(ctx, false, false, format, args...)
}

// InfofErr if last argument is not nil it will be printed as " err=%q"
func InfofErr(ctx context.Context, format string, args ...interface{}) {
	infof(ctx, true, false, format, args...)
}

func V(level glog.Level) Verbose {
	return Verbose(glog.V(level))
}

// Infof is equivalent to the global Infof function, guarded by the value of v.
// See the documentation of V for usage.
func (v Verbose) Infof(ctx context.Context, format string, args ...interface{}) {
	if v {
		infof(ctx, false, false, format, args...)
	}
}

func (v Verbose) InfofErr(ctx context.Context, format string, args ...interface{}) {
	var err interface{}
	if len(args) > 0 {
		err = args[len(args)-1]
	}
	if v || err != nil {
		infof(ctx, true, false, format, args...)
	}
}

func infof(ctx context.Context, lastErr bool, publicLog bool, format string, args ...interface{}) {
	msg, isErr := formatMessage(ctx, lastErr, publicLog, format, args...)
	if bool(glog.V(2)) && isErr {
		glog.ErrorDepth(2, msg)
	} else if glog.V(1) {
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

func formatMessage(ctx context.Context, lastErr bool, publicLog bool, format string, args ...interface{}) (string, bool) {
	var sb strings.Builder
	if publicLog {
		sb.WriteString(publicLogTag)
	}
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

func PublicInfof(ctx context.Context, format string, args ...interface{}) {
	publicCtx := context.Background()

	publicCtx = PublicCloneCtx(ctx, publicCtx, publicLogKeys)

	infof(publicCtx, false, true, format, args...)
}

// PublicCloneCtx creates a new context but only copies key/val pairs from the original context
// that are allowed to be published publicly (i.e. list in []publicLogKeys
func PublicCloneCtx(originalCtx context.Context, publicCtx context.Context, publicLogKeys []string) context.Context {
	cmap, _ := originalCtx.Value(clogContextKey).(*values)
	publicCmap := newValues()
	if cmap != nil {
		cmap.mu.RLock()
		for k, v := range cmap.vals {
			for _, key := range publicLogKeys {
				if key == k {
					publicCmap.vals[k] = v
				}
			}
		}
		cmap.mu.RUnlock()
	}
	return context.WithValue(publicCtx, clogContextKey, publicCmap)
}
