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
