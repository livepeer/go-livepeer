// Copyright (C) 2017  Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package gnmi

import (
	"testing"

	"github.com/aristanetworks/goarista/test"
)

func p(s ...string) []string {
	return s
}

func TestSplitPath(t *testing.T) {
	for i, tc := range []struct {
		in  string
		exp []string
	}{{
		in:  "/foo/bar",
		exp: p("foo", "bar"),
	}, {
		in:  "/foo/bar/",
		exp: p("foo", "bar"),
	}, {
		in:  "//foo//bar//",
		exp: p("", "foo", "", "bar", ""),
	}, {
		in:  "/foo[name=///]/bar",
		exp: p("foo[name=///]", "bar"),
	}, {
		in:  `/foo[name=[\\\]/]/bar`,
		exp: p(`foo[name=[\\\]/]`, "bar"),
	}, {
		in:  `/foo[name=[\\]/bar`,
		exp: p(`foo[name=[\\]`, "bar"),
	}, {
		in:  "/foo[a=1][b=2]/bar",
		exp: p("foo[a=1][b=2]", "bar"),
	}} {
		got := SplitPath(tc.in)
		if !test.DeepEqual(tc.exp, got) {
			t.Errorf("[%d] unexpect split for %q. Expected: %v, Got: %v",
				i, tc.in, tc.exp, got)
		}
	}
}
