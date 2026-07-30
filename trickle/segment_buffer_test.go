package trickle

import (
	"bytes"
	"testing"
)

func TestSegmentBufferReadChunkAcrossPages(t *testing.T) {
	buf := newSegmentBuffer()
	chunkA := bytes.Repeat([]byte("a"), segmentBufferInitialPageSize)
	chunkB := bytes.Repeat([]byte("b"), segmentBufferInitialPageSize*2)
	want := append(append([]byte{}, chunkA...), chunkB...)

	buf.write(chunkA)
	buf.write(chunkB)

	var got []byte
	nextPos := 0
	for {
		data, next, atTail, invalid, _ := buf.readChunk(nextPos)
		if invalid {
			t.Fatalf("invalid cursor at pos=%d", nextPos)
		}
		if len(data) == 0 {
			break
		}
		got = append(got, data...)
		nextPos = next
		if atTail {
			break
		}
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("buffer mismatch: got=%d want=%d", len(got), len(want))
	}
	if nextPos != len(want) {
		t.Fatalf("unexpected tail cursor: pos=%d", nextPos)
	}
}

func TestSegmentBufferTailGrowth(t *testing.T) {
	buf := newSegmentBuffer()
	chunkA := []byte("hello")
	chunkB := []byte(" world")

	buf.write(chunkA)

	data, nextPos, atTail, invalid, _ := buf.readChunk(0)
	if len(data) == 0 || !atTail || invalid {
		t.Fatalf("initial read state len=%d atTail=%v invalid=%v", len(data), atTail, invalid)
	}
	if string(data) != "hello" {
		t.Fatalf("unexpected initial data %q", string(data))
	}

	buf.write(chunkB)

	data, nextPos, atTail, invalid, _ = buf.readChunk(nextPos)
	if len(data) == 0 || !atTail || invalid {
		t.Fatalf("growth read state len=%d atTail=%v invalid=%v", len(data), atTail, invalid)
	}
	if string(data) != " world" {
		t.Fatalf("unexpected growth data %q", string(data))
	}
	if nextPos != len(chunkA)+len(chunkB) {
		t.Fatalf("unexpected grown cursor: pos=%d", nextPos)
	}
}
