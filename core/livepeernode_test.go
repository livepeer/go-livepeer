package core

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/livepeer/go-livepeer/drivers"
	"github.com/livepeer/go-livepeer/pm"
	"github.com/livepeer/lpms/ffmpeg"
)

type StubTranscoder struct {
	Profiles      []ffmpeg.VideoProfile
	SegCount      int
	FailTranscode bool
}

func (t *StubTranscoder) Transcode(fname string, profiles []ffmpeg.VideoProfile) ([][]byte, error) {
	if t.FailTranscode {
		return nil, ErrTranscode
	}

	t.SegCount++

	result := make([][]byte, 0)
	for _, p := range t.Profiles {
		result = append(result, []byte(fmt.Sprintf("Transcoded_%v", p.Name)))
	}
	return result, nil
}

func TestTranscodeAndBroadcast(t *testing.T) {
	ffmpeg.InitFFmpeg()
	p := []ffmpeg.VideoProfile{ffmpeg.P720p60fps16x9, ffmpeg.P144p30fps16x9}
	tr := &StubTranscoder{Profiles: p}
	storage := drivers.NewMemoryDriver("").NewSession("")
	config := transcodeConfig{LocalOS: storage, OS: storage}

	tmpdir, _ := ioutil.TempDir("", "")
	n, err := NewLivepeerNode(nil, tmpdir, nil, new(pm.MockRecipient))
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	defer os.RemoveAll(tmpdir)
	n.Transcoder = tr

	md := &SegTranscodingMetadata{Profiles: p}
	ss := StubSegment()
	res := n.transcodeSeg(config, ss, md)
	if res.Err != nil {
		t.Errorf("Error: %v", res.Err)
	}

	if len(res.Data) != len(p) {
		t.Errorf("Expecting %v profiles, got %v", len(p), len(res.Data))
	}

	//Should have 1 transcoded segment into 2 different profiles
	if tr.SegCount != 1 {
		t.Errorf("Expecting 1 segment to be transcoded, got %v", tr.SegCount)
	}

	// Check playlist was updated
	// XXX Fix this
	/*for _, v := range strmIds {
		pl := n.VideoSource.GetHLSMediaPlaylist(v)
		if pl == nil {
			t.Error("Expected media playlist; got none")
		}
		if len(pl.Segments) != 1 && pl.SeqNo != 100 {
			t.Error("Mismatched segments (expected 1) or seq (expected 100), got ", pl.Segments, pl.SeqNo)
		}
	}*/

	// TODO check sig?

	// Test when transcoder fails
	tr.FailTranscode = true
	res = n.transcodeSeg(config, ss, md)
	if res.Err == nil {
		t.Error("Expecting a transcode error")
	}
	tr.FailTranscode = false

	// Test when the number of results mismatchches expectations
	tr.Profiles = []ffmpeg.VideoProfile{p[0]}
	res = n.transcodeSeg(config, ss, md)
	if res.Err == nil || res.Err.Error() != "MismatchedSegments" {
		t.Error("Did not get mismatched segments as expected")
	}
	tr.Profiles = p
}
