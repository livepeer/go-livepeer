package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"io/ioutil"
)

type segment_properties struct {
    video_codec_name string
    video_resolution string
    display_aspect_ratio string
    avg_frame_rate string
    audio_codec_name string
}

func Inspect(HlsSegment string) segment_properties {
	fmt.Println("Inspecting ", HlsSegment)

        cmd := exec.Command("./ffprobe.sh", HlsSegment)

        stdout, err := cmd.StdoutPipe()
        if err != nil {
                log.Fatal(err)
        }
        if err := cmd.Start(); err != nil {
                log.Fatal(err)
        }

        slurp, _ := ioutil.ReadAll(stdout)


        if err := cmd.Wait(); err != nil {
                log.Fatal(err)
        }

	var properties segment_properties

        var x map[string]interface{}
        json.Unmarshal([]byte(slurp), &x)

        stream := x["streams"].([]interface{})

	for _, element := range stream {
		if "video" == element.(map[string]interface{})["codec_type"].(string){
	        	properties.video_codec_name = element.(map[string]interface{})["codec_name"].(string)
	
       			width := element.(map[string]interface{})["width"].(float64)
        		height := element.(map[string]interface{})["height"].(float64)

        		properties.video_resolution = fmt.Sprintf("%g x %g", width, height)
        		properties.display_aspect_ratio = element.(map[string]interface{})["display_aspect_ratio"].(string)
        		properties.avg_frame_rate = element.(map[string]interface{})["avg_frame_rate"].(string)
		}

		if "audio" == element.(map[string]interface{})["codec_type"].(string){
        		properties.audio_codec_name = element.(map[string]interface{})["codec_name"].(string)
		}
	}

	return properties
}

func main() {
	if len(os.Args) != 2 {
		fmt.Println("HLS segment is not specified")
		os.Exit(-1)
	}

	arg := os.Args[1]

	properties := Inspect(arg)
	fmt.Printf("video_codec : %s\n", properties.video_codec_name)
	fmt.Printf("video_resolution : %s\n", properties.video_resolution)
	fmt.Printf("display_aspect_ratio : %s\n", properties.display_aspect_ratio)
	fmt.Printf("avg_frame_rate : %s\n", properties.avg_frame_rate)
	fmt.Printf("audio_codec : %s\n", properties.audio_codec_name)
}
