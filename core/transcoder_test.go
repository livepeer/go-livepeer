package core

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/livepeer/lpms/ffmpeg"
)

func TestLocalTranscoder(t *testing.T) {
	tmp, _ := ioutil.TempDir("", "")
	defer os.RemoveAll(tmp)
	tc := NewLocalTranscoder(tmp)
	ffmpeg.InitFFmpeg()

	profiles := []ffmpeg.VideoProfile{ffmpeg.P144p30fps16x9, ffmpeg.P240p30fps16x9}
	res, err := tc.Transcode("test.ts", profiles)
	if err != nil {
		t.Error("Error transcoding ", err)
	}
	if len(res) != len(profiles) {
		t.Error("Mismatched results")
	}
	if Over1Pct(len(res[0]), 164312) {
		t.Errorf("Wrong data %v", len(res[0]))
	}
	if Over1Pct(len(res[1]), 227668) {
		t.Errorf("Wrong data %v", len(res[1]))
	}
}

func TestNvidiaTranscoder(t *testing.T) {
	tmp, _ := ioutil.TempDir("", "")
	defer os.RemoveAll(tmp)
	tc := NewNvidiaTranscoder("123,456", tmp)
	ffmpeg.InitFFmpeg()

	// test device selection
	nv, ok := tc.(*NvidiaTranscoder)
	if !ok || "456" != nv.getDevice() || "123" != nv.getDevice() ||
		"456" != nv.getDevice() || "123" != nv.getDevice() {
		t.Error("Error when getting devices")
	}

	// local sample isn't in a supported pixel format, so use this instead
	fname := "../vendor/github.com/livepeer/lpms/transcoder/test.ts"

	// transcoding should fail due to invalid devices
	profiles := []ffmpeg.VideoProfile{ffmpeg.P144p30fps16x9, ffmpeg.P240p30fps16x9}
	_, err := tc.Transcode(fname, profiles)
	if err == nil ||
		(err.Error() != "Unknown error occurred" &&
			err.Error() != "Cannot allocate memory") {
		t.Error(err)
	}

	dev := os.Getenv("NV_DEVICE")
	if dev == "" {
		t.Skip("No device specified; skipping remainder of Nvidia tests")
		return
	}
	tc = NewNvidiaTranscoder(dev, tmp)
	res, err := tc.Transcode(fname, profiles)
	if err != nil {
		t.Error(err)
	}
	if Over1Pct(len(res[0]), 650292) {
		t.Errorf("Wrong data %v", len(res[0]))
	}
	if Over1Pct(len(res[1]), 884164) {
		t.Errorf("Wrong data %v", len(res[1]))
	}
}
