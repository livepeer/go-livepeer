package main

import (
	"fmt"
	"os"

	"github.com/livepeer/lpms/ffmpeg"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: livepeer_ffmpeg <input_file> <output_pattern>")
		os.Exit(1)
	}

	in := os.Args[1]
	outFilePattern := os.Args[2]

	ffmpeg.FfmpegSetLogLevel(ffmpeg.FFLogWarning)
	_, err := ffmpeg.Transcode3(&ffmpeg.TranscodeOptionsIn{
		Fname: in,
	}, []ffmpeg.TranscodeOptions{{
		Oname:        outFilePattern,
		AudioEncoder: ffmpeg.ComponentOptions{Name: "copy"},
		VideoEncoder: ffmpeg.ComponentOptions{Name: "copy"},
		Muxer:        ffmpeg.ComponentOptions{Name: "segment"},
	}})
	if err != nil {
		fmt.Printf("Failed to run segmentation. in=%s err=%s\n", in, err)
		os.Exit(1)
	}
}
