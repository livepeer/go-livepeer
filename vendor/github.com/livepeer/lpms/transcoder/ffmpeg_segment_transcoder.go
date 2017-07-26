package transcoder

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"os/exec"
	"path"
	"time"

	"github.com/golang/glog"
)

//SegmentTranscoder transcodes segments individually.  This is a simple wrapper for calling FFMpeg on the command line.
type FFMpegSegmentTranscoder struct {
	Bitrate    string
	Framerate  uint
	Resolution string
	ffmpegPath string
	workDir    string
}

func NewFFMpegSegmentTranscoder(bitr string, framer uint, res string, ffmpegp string, workd string) *FFMpegSegmentTranscoder {
	return &FFMpegSegmentTranscoder{Bitrate: bitr, Framerate: framer, Resolution: res, ffmpegPath: ffmpegp, workDir: workd}
}

func (t *FFMpegSegmentTranscoder) Transcode(d []byte) ([]byte, error) {
	//Assume d is in the right format, write it to disk
	inName := randName()
	outName := fmt.Sprintf("out%v", inName)
	if _, err := os.Stat(t.workDir); os.IsNotExist(err) {
		err := os.Mkdir(t.workDir, 0700)
		if err != nil {
			glog.Errorf("Transcoder cannot create workdir: %v", err)
			return nil, err
		}
	}

	if err := ioutil.WriteFile(path.Join(t.workDir, inName), d, 0644); err != nil {
		glog.Errorf("Transcoder cannot write file: %v", err)
		return nil, err
	}

	//Invoke ffmpeg
	var cmd *exec.Cmd
	//ffmpeg -i seg.ts -c:v libx264 -s 426:240 -r 30 -mpegts_copyts 1 -minrate 700k -maxrate 700k -bufsize 700k -threads 1 out3.ts
	cmd = exec.Command(path.Join(t.ffmpegPath, "ffmpeg"), "-i", path.Join(t.workDir, inName), "-c:v", "libx264", "-s", t.Resolution, "-mpegts_copyts", "1", "-minrate", t.Bitrate, "-maxrate", t.Bitrate, "-bufsize", t.Bitrate, "-r", fmt.Sprintf("%d", t.Framerate), "-threads", "1", path.Join(t.workDir, outName))
	if err := cmd.Run(); err != nil {
		glog.Errorf("Cannot start ffmpeg command: %v", err)
		return nil, err
	}

	dout, err := ioutil.ReadFile(path.Join(t.workDir, outName))
	if err != nil {
		glog.Errorf("Cannot read transcode output: %v", err)
	}

	os.Remove(path.Join(t.workDir, inName))
	os.Remove(path.Join(t.workDir, outName))
	return dout, nil
}

func randName() string {
	rand.Seed(time.Now().UnixNano())
	x := make([]byte, 10, 10)
	for i := 0; i < len(x); i++ {
		x[i] = byte(rand.Uint32())
	}
	return fmt.Sprintf("%x.ts", x)
}
