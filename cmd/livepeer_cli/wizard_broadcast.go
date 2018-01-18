package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/golang/glog"
	lpcommon "github.com/livepeer/go-livepeer/common"
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
	maxPricePerSegment := w.readBigInt()

	opts := w.allTranscodingOptions()
	if opts == nil {
		return
	}

	fmt.Printf("Enter the identifiers of the video profiles you would like to use (use a comma as the delimiter in a list of identifiers) - ")
	idList := w.readString()

	transOpts, err := w.idListToVideoProfileList(idList, opts)
	if err != nil {
		glog.Error(err)
		return
	}

	val := url.Values{
		"maxPricePerSegment": {fmt.Sprintf("%v", maxPricePerSegment.String())},
		"transcodingOptions": {fmt.Sprintf("%v", transOpts)},
	}

	httpPostWithParams(fmt.Sprintf("http://%v:%v/setBroadcastConfig", w.host, w.httpPort), val)
}

func (w *wizard) idListToVideoProfileList(idList string, opts map[int]string) (string, error) {
	ids := strings.Split(idList, ",")

	var pListBuf bytes.Buffer
	for i, id := range ids {
		val, err := strconv.Atoi(strings.TrimSpace(id))
		if err != nil {
			return "", err
		}

		p, ok := opts[val]
		if !ok {
			return "", fmt.Errorf("not a valid identifier")
		} else {
			pListBuf.WriteString(p)

			if i < len(ids)-1 {
				pListBuf.WriteString(",")
			}
		}
	}

	return pListBuf.String(), nil
}

func (w *wizard) broadcast() {
	depositStr := w.getDeposit()
	deposit, err := lpcommon.ParseBigInt(depositStr)
	if err != nil {
		glog.Error(err)
		return
	}

	if deposit.Cmp(big.NewInt(0)) == 0 {
		fmt.Printf("Please deposit some ETH using the CLI in order broadcast\n")

		w.deposit()
	}

	if runtime.GOOS == "darwin" {
		fmt.Println()
		if w.rtmpPort != "" && w.httpPort != "" {
			fmt.Printf("Current RTMP setting: http://localhost:%v/streams\n", w.rtmpPort)
			fmt.Printf("Current HTTP setting: http://localhost:%v/streams\n", w.httpPort)
			fmt.Printf("Keep it? (Y/n) ")
			if strings.ToLower(w.readDefaultString("y")) != "y" {
				fmt.Printf("New rtmp port? (default 1935)")
				w.rtmpPort = w.readDefaultString("1935")
				fmt.Printf("New http port? (default 8935)")
				w.httpPort = w.readDefaultString("8935")
				fmt.Printf("New RTMP setting: http://localhost:%v/streams\n", w.rtmpPort)
				fmt.Printf("New HTTP setting: http://localhost:%v/streams\n", w.httpPort)
			}
		}
		cmd := exec.Command("ffmpeg", "-f", "avfoundation", "-framerate", "30", "-pixel_format", "uyvy422", "-i", "0:0", "-c:v", "libx264", "-pix_fmt", "yuv420p", "-tune", "zerolatency", "-s", "426x240", "-b:v", "400k", "-x264-params", "keyint=60:min-keyint=60", "-c:a", "aac", "-ac", "1", "-b:a", "96k", "-f", "flv", fmt.Sprintf("rtmp://localhost:%v/movie", w.rtmpPort))

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
		resp, err := http.Get(fmt.Sprintf("http://localhost:%v/manifestID", w.httpPort))
		if err != nil {
			glog.Errorf("Error getting manifest ID: %v", err)
			return
		}

		defer resp.Body.Close()
		id, err := ioutil.ReadAll(resp.Body)
		if err != nil || string(id) == "" {
			glog.Errorf("Error reading manifest ID: %v", err)
			return
		}
		fmt.Printf("ManifestID: %v\n", string(id))

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
