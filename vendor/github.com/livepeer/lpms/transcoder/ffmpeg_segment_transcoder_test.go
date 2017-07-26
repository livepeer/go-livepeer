package transcoder

import (
	"io/ioutil"
	"testing"
)

func TestTrans(t *testing.T) {
	testSeg, err := ioutil.ReadFile("./test.ts")
	if err != nil {
		t.Errorf("Error reading test segment: %v", err)
	}

	tr := NewFFMpegSegmentTranscoder("700k", 30, "426:240", "", "./")
	r, err := tr.Transcode(testSeg)
	if err != nil {
		t.Errorf("Error transcoding: %v", err)
	}

	if r == nil {
		t.Errorf("Did not get output")
	}

	if len(r) != 523768 {
		t.Errorf("Expecting output size to be 523768, got %v", len(r))
	}
}
