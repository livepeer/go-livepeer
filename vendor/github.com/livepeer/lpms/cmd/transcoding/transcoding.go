package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/livepeer/lpms/ffmpeg"
)

func validRenditions() []string {
	valids := make([]string, len(ffmpeg.VideoProfileLookup))
	for p, _ := range ffmpeg.VideoProfileLookup {
		valids = append(valids, p)
	}
	return valids
}

func main() {
	if len(os.Args) <= 3 {
		panic("Usage: <input file> <output renditions, comma separated> <sw/nv>")
	}
	str2accel := func(inp string) (ffmpeg.Acceleration, string) {
		if inp == "nv" {
			return ffmpeg.Nvidia, "nv"
		}
		return ffmpeg.Software, "sw"
	}
	str2profs := func(inp string) []ffmpeg.VideoProfile {
		profs := []ffmpeg.VideoProfile{}
		strs := strings.Split(inp, ",")
		for _, k := range strs {
			p, ok := ffmpeg.VideoProfileLookup[k]
			if !ok {
				panic(fmt.Sprintf("Invalid rendition %s. Valid renditions are:\n%s", k, validRenditions()))
			}
			profs = append(profs, p)
		}
		return profs
	}
	fname := os.Args[1]
	profiles := str2profs(os.Args[2])
	accel, lbl := str2accel(os.Args[3])

	profs2opts := func(profs []ffmpeg.VideoProfile) []ffmpeg.TranscodeOptions {
		opts := []ffmpeg.TranscodeOptions{}
		for i := range profs {
			o := ffmpeg.TranscodeOptions{
				Oname:   fmt.Sprintf("out_%s_%d_out.mkv", lbl, i),
				Profile: profs[i],
				Accel:   accel,
			}
			opts = append(opts, o)
		}
		return opts
	}
	options := profs2opts(profiles)

	var dev string
	if accel == ffmpeg.Nvidia {
		if len(os.Args) <= 4 {
			panic("Expected device number")
		}
		dev = os.Args[4]
	}

	ffmpeg.InitFFmpeg()

	fmt.Printf("Setting fname %s encoding %d renditions with %v\n", fname, len(options), lbl)
	err := ffmpeg.Transcode2(&ffmpeg.TranscodeOptionsIn{
		Fname:  fname,
		Accel:  accel,
		Device: dev,
	}, options)
	if err != nil {
		panic(err)
	}
}
