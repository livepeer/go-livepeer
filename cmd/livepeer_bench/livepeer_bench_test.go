package main

import (
	"path/filepath"
	"runtime"
	"testing"
)

func TestSegmentPath(t *testing.T) {
	// Windows-style manifest path with backslash separators, as reported in
	// issue #1844. On Windows the path package treats backslashes as ordinary
	// characters, so path.Dir/path.Join produce a broken segment path;
	// filepath handles them as separators.
	if runtime.GOOS == "windows" {
		const manifest = `C:\bench\media\input.m3u8`
		want := filepath.Join(`C:\bench\media`, "seg0.ts")
		if got := segmentPath(manifest, "seg0.ts"); got != want {
			t.Fatalf("segmentPath(%q) = %q, want %q", manifest, got, want)
		}
		return
	}

	// POSIX-style manifest path: forward slashes. filepath and path agree on
	// this input, but this guards the normal case against regression.
	const manifest = "/tmp/bench/media/input.m3u8"
	want := filepath.Join("/tmp/bench/media", "seg0.ts")
	if got := segmentPath(manifest, "seg0.ts"); got != want {
		t.Fatalf("segmentPath(%q) = %q, want %q", manifest, got, want)
	}
}
