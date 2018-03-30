package transcoder

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"

	"github.com/golang/glog"
	"github.com/livepeer/lpms/ffmpeg"
)

//SegmentTranscoder transcodes segments individually.  This is a simple wrapper for calling FFMpeg on the command line.
type FFMpegSegmentTranscoder struct {
	tProfiles []ffmpeg.VideoProfile
	workDir   string
}

func NewFFMpegSegmentTranscoder(ps []ffmpeg.VideoProfile, workd string) *FFMpegSegmentTranscoder {
	return &FFMpegSegmentTranscoder{tProfiles: ps, workDir: workd}
}

func (t *FFMpegSegmentTranscoder) Transcode(fname string) ([][]byte, error) {
	//Invoke ffmpeg
	err := ffmpeg.Transcode(fname, t.workDir, t.tProfiles)
	if err != nil {
		glog.Errorf("Error transcoding: %v", err)
		return nil, err
	}

	dout := make([][]byte, len(t.tProfiles), len(t.tProfiles))
	for i, _ := range t.tProfiles {
		ofile := path.Join(t.workDir, fmt.Sprintf("out%v%v", i, filepath.Base(fname)))
		d, err := ioutil.ReadFile(ofile)
		if err != nil {
			glog.Errorf("Cannot read transcode output: %v", err)
		}
		dout[i] = d
		os.Remove(ofile)
	}

	return dout, nil
}
