package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"text/tabwriter"
	"time"

	"github.com/golang/glog"
)

func (w *wizard) allTranscodingOptions() map[int]string {
	resp, err := http.Get(fmt.Sprintf("http://%v:%v/getAvailableTranscodingOptions", w.host, w.httpPort))
	if err != nil {
		glog.Errorf("Error getting all transcoding options: %v", err)
		return nil
	}

	defer resp.Body.Close()
	result, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		glog.Errorf("Error reading response: %v", err)
		return nil
	}

	var opts []string
	err = json.Unmarshal(result, &opts)
	if err != nil {
		glog.Errorf("Error unmarshalling all transcoding options: %v", err)
		return nil
	}

	optIds := make(map[int]string)

	wtr := tabwriter.NewWriter(os.Stdout, 0, 8, 1, '\t', 0)
	fmt.Fprintln(wtr, "Identifier\tTranscoding Options")
	for idx, opt := range opts {
		fmt.Fprintf(wtr, "%v\t%v\n", idx, opt)
		optIds[idx] = opt
	}

	wtr.Flush()

	return optIds
}

func (w *wizard) setBroadcastConfig() {
	fmt.Printf("Enter broadcast max price per segment - ")
	maxPricePerSegment := w.readInt()

	opts := w.allTranscodingOptions()
	if opts == nil {
		return
	}

	fmt.Printf("Enter the identifier of the transcoding options you would like to use - ")
	id := w.readInt()

	val := url.Values{
		"maxPricePerSegment": {fmt.Sprintf("%v", maxPricePerSegment)},
		"transcodingOptions": {fmt.Sprintf("%v", opts[id])},
	}

	httpPostWithParams(fmt.Sprintf("http://%v:%v/setBroadcastConfig", w.host, w.httpPort), val)
}

func (w *wizard) broadcast() {
	if runtime.GOOS == "darwin" {
		fmt.Println()
		if w.rtmpPort != "" && w.httpPort != "" {
			fmt.Printf("Current RTMP setting: http://localhost:%v/streams\n", w.rtmpPort)
			fmt.Printf("Current HTTP setting: http://localhost:%v/streams\n", w.httpPort)
			fmt.Printf("Keep it? (Y/n) ")
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
