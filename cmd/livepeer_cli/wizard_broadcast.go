package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/golang/glog"
)

func (w *wizard) broadcast() {
	if runtime.GOOS == "darwin" {
		fmt.Println()
		if w.rtmpPort != "" && w.httpPort != "" {
			fmt.Printf("Current RTMP setting: http://localhost:%v/streams\n", w.rtmpPort)
			fmt.Printf("Current HTTP setting: http://localhost:%v/streams\n", w.httpPort)
			fmt.Println("Keep it? (y/n)")
			if w.readDefaultString("y") != "y" {
				fmt.Printf("New rtmp port? (default 1935)")
				w.rtmpPort = w.readDefaultString("1935")
				fmt.Printf("New http port? (default 8935)")
				w.httpPort = w.readDefaultString("8935")
				fmt.Printf("New RTMP setting: http://localhost:%v/streams\n", w.rtmpPort)
				fmt.Printf("New HTTP setting: http://localhost:%v/streams\n", w.httpPort)
			}
		}
		cmd := exec.Command("ffmpeg", "-f", "avfoundation", "-framerate", "30", "-pixel_format", "uyvy422", "-i", "0:0", "-vcodec", "libx264", "-tune", "zerolatency", "-b", "1000k", "-x264-params", "keyint=60:min-keyint=60", "-acodec", "aac", "-ac", "1", "-b:a", "96k", "-f", "flv", fmt.Sprintf("rtmp://localhost:%v/movie", w.rtmpPort))

		var out bytes.Buffer
		var stderr bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &stderr
		err := cmd.Start()
		if err != nil {
			glog.Infof("Couldn't broadcast the stream: %v %v", err, stderr.String())
			os.Exit(1)
		}

		fmt.Printf("Now broadcasting - %v%v\n", out.String(), stderr.String())
		go func() {
			if err = cmd.Wait(); err != nil {
				// glog.Errorf("Error running broadcast: %v\n%v", err, stderr.String())
				return
			}
		}()

		time.Sleep(3 * time.Second)
		resp, err := http.Get(fmt.Sprintf("http://localhost:%v/streamID", w.httpPort))
		if err != nil {
			glog.Errorf("Error getting stream ID: %v", err)
			return
		}

		defer resp.Body.Close()
		id, err := ioutil.ReadAll(resp.Body)
		if err != nil || string(id) == "" {
			glog.Errorf("Error reading stream ID: %v", err)
			return
		}
		fmt.Printf("StreamID: %v\n", string(id))

		for {
			fmt.Printf("Type `q` to stop broadcasting\n")
			end := w.read()
			if end == "q" {
				fmt.Println("Quitting broadcast...")
				cmd.Process.Kill()
				time.Sleep(time.Second)
				return
			}
		}

	} else {
		glog.Errorf("The broadcast command only support darwin for now.  Please download OBS to broadcast.")
	}
}
