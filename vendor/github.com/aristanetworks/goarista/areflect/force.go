// Copyright (C) 2014  Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

// Package areflect provides utilities to help with reflection.
package areflect

import (
	"reflect"
	"unsafe"
)

// ForceExport returns a new reflect.Value that is identical to the one passed
// in argument except that it's considered as an exported symbol even if in
// reality it isn't.
//
// The `reflect' package intentionally makes it impossible to access the value
// of an unexported attribute.  The implementation of reflect.DeepEqual() cheats
// as it bypasses this check.  Unfortunately, we can't use the same cheat, which
// prevents us from re-implementing DeepEqual properly or implementing some other
// reflection-based tools.  So this is our cheat on top of theirs.  It makes
// the given reflect.Value appear as if it was exported.
//
// This function requires go1.6 or newer.
func ForceExport(v reflect.Value) reflect.Value {
	// constants from reflect/value.go
	const flagStickyRO uintptr = 1 << 5
	const flagEmbedRO uintptr = 1 << 6 // new in go1.6 (was flagIndir before)
	const flagRO uintptr = flagStickyRO | flagEmbedRO
	ptr := unsafe.Pointer(&v)
	rv := (*struct {
		typ  unsafe.Pointer // a *reflect.rtype (reflect.Type)
		ptr  unsafe.Pointer // The value wrapped by this reflect.Value
		flag uintptr
	})(ptr)
	rv.flag &= ^flagRO // Unset the flag so this value appears to be exported.
	return v
}
