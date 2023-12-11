package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os/exec"
	"time"

	"github.com/golang/glog"
)

func (w *wizard) stream() {
	if w.httpPort == "" {
		fmt.Println("Need to specify http port")
		return
	}
	fmt.Println("Enter StreamID:")
	streamID := w.read()

	//Fetch local stream playlist - this will request from the network so we know the stream is here
	url := fmt.Sprintf("http://localhost:%v/stream/%v.m3u8", w.httpPort, streamID)
	fmt.Printf("Getting stream from: %v\n", url)
	res, err := http.Get(url)
	if err != nil || res.StatusCode != 200 {
		fmt.Printf("err: %v, status:%v", err, res.StatusCode)
		return
	}
	_, err = ioutil.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println("Here...")
	cmd := exec.Command("ffplay", "-timeout", "180000000", url) //timeout in 3 mins
	err = cmd.Start()
	if err != nil {
		glog.Infof("Couldn't start the stream.  Make sure a local Livepeer node is running on port %v", w.httpPort)
		return
	}

	go func() {
		err = cmd.Wait()
		if err != nil {
			glog.Infof("Couldn't start the stream.  Make sure a local Livepeer node is running on port %v", w.httpPort)
			fmt.Printf("Type `q` to stop streaming\n")
			return
		}
	}()

	fmt.Printf("Type `q` to stop streaming\n")
	end := w.read()
	if end == "q" {
		fmt.Println("Quitting broadcast...")
		cmd.Process.Kill()
		time.Sleep(time.Second)
		return
	}

	return
}
