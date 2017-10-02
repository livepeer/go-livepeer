package transcoder

import (
	"io/ioutil"
	"testing"

	"github.com/livepeer/lpms/core"
)

func TestTrans(t *testing.T) {
	testSeg, err := ioutil.ReadFile("./test.ts")
	if err != nil {
		t.Errorf("Error reading test segment: %v", err)
	}

	configs := []core.VideoProfile{
		core.P144p30fps16x9,
		core.P240p30fps16x9,
		core.P576p30fps16x9,
	}
	tr := NewFFMpegSegmentTranscoder(configs, "", "./")
	r, err := tr.Transcode(testSeg)
	if err != nil {
		t.Errorf("Error transcoding: %v", err)
	}

	if r == nil {
		t.Errorf("Did not get output")
	}

	if len(r) != 3 {
		t.Errorf("Expecting 2 output segments, got %v", len(r))
	}

	if len(r[0]) < 400000 || len(r[0]) > 550000 {
		t.Errorf("Expecting output size to be between 400000 and 550000, got %v", len(r[0]))
	}

	if len(r[1]) < 500000 || len(r[1]) > 600000 {
		t.Errorf("Expecting output size to be between 50000 and 600000, got %v", len(r[1]))
	}

	if len(r[2]) < 800000 || len(r[2]) > 950000 {
		t.Errorf("Expecting output size to be between 800000 and 950000, got %v", len(r[2]))
	}
}
