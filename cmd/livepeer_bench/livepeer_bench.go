package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	//"runtime/pprof"

	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/lpms/ffmpeg"
	"github.com/livepeer/m3u8"
)

func main() {
	/*
		cprof, err := os.Create("bench.prof")
		if err != nil {
			panic(err)
		}
		pprof.StartCPUProfile(cprof)
		defer pprof.StopCPUProfile()
	*/
	// Override the default flag set since there are dependencies that
	// incorrectly add their own flags (specifically, due to the 'testing'
	// package being linked)
	flag.Set("logtostderr", "true")
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

	fname := flag.String("fname", "", "Input m3u8 manifest file")
	conc := flag.Int("conc", 1, "# of concurrent transcode sessions")
	segs := flag.Int("segs", 0, "Maximum # of segments to transcode (default all)")
	profs := flag.String("profs", "P240p30fps16x9,P360p30fps16x9,P720p30fps16x9", "Transcoding options for broadcast job, or path to json config")
	nvidia := flag.String("nvidia", "", "Comma-separated list of Nvidia GPU device IDs to use for transcoding")

	flag.Parse()

	if *fname == "" {
		panic("Please provide the input manifest as `-fname <input.m3u8>`. See -h or -help for more.")
	}

	profiles := parseVideoProfiles(*profs)

	f, err := os.Open(*fname)
	if err != nil {
		panic(err)
	}
	p, _, err := m3u8.DecodeFrom(bufio.NewReader(f), true)
	if err != nil {
		panic(err)
	}
	pl, ok := p.(*m3u8.MediaPlaylist)
	if !ok {
		panic("Expecting media PL")
	}

	accel := ffmpeg.Software
	devices := []string{}
	if *nvidia != "" {
		accel = ffmpeg.Nvidia
		devices = strings.Split(*nvidia, ",")
	}

	ffmpeg.InitFFmpeg()
	segCount := 0
	var wg sync.WaitGroup
	dir := path.Dir(*fname)
	start := time.Now()
	fmt.Fprintf(os.Stderr, "Program %s Source %s Concurrency %d Profiles %s\n", os.Args[0], *fname, *conc, *profs)
	fmt.Println("time,stream,segment,length")
	for i := 0; i < *conc; i++ {
		wg.Add(1)
		go func(k int, wg *sync.WaitGroup) {
			tc := ffmpeg.NewTranscoder()
			for j, v := range pl.Segments {
				if *segs > 0 && j >= *segs {
					break
				}
				if v == nil {
					continue
				}
				u := path.Join(dir, v.URI)
				//u := v.URI
				in := &ffmpeg.TranscodeOptionsIn{
					Fname: u,
					Accel: accel,
				}
				if ffmpeg.Software != accel {
					in.Device = devices[k%len(devices)]
				}
				profs2opts := func(profs []ffmpeg.VideoProfile) []ffmpeg.TranscodeOptions {
					opts := []ffmpeg.TranscodeOptions{}
					//for n, p := range profs {
					for _, p := range profs {
						o := ffmpeg.TranscodeOptions{
							//Oname: fmt.Sprintf("%s%s_%s_%d_%d_%d.ts", pfx, accelStr, p.Name, n, k, j),
							Oname:        "-",
							Profile:      p,
							Accel:        accel,
							AudioEncoder: ffmpeg.ComponentOptions{Name: "drop"},
							Muxer:        ffmpeg.ComponentOptions{Name: "null"},
						}
						opts = append(opts, o)
					}
					return opts
				}
				out := profs2opts(profiles)
				t := time.Now()
				_, err := tc.Transcode(in, out)
				end := time.Now()
				fmt.Printf("%s,%d,%d,%0.2v\n", end.Format("2006-01-02 15:04:05.999999999"), k, j, end.Sub(t).Seconds())
				if err != nil {
					panic(err)
				}
				segCount++
			}
			tc.StopTranscoder()
			wg.Done()
		}(i, &wg)
		time.Sleep(300 * time.Millisecond)
	}
	wg.Wait()
	fmt.Fprintf(os.Stderr, "Took %v to transcode %v segments\n",
		time.Now().Sub(start).Seconds(), segCount)
}

func parseVideoProfiles(inp string) []ffmpeg.VideoProfile {
	type profilesJson struct {
		Profiles []struct {
			Name    string `json:"name"`
			Width   int    `json:"width"`
			Height  int    `json:"height"`
			Bitrate int    `json:"bitrate"`
			FPS     uint   `json:"fps"`
			FPSDen  uint   `json:"fpsDen"`
			Profile string `json:"profile"`
			GOP     string `json:"gop"`
		} `json:"profiles"`
	}
	profs := []ffmpeg.VideoProfile{}
	if inp != "" {
		// try opening up json file with profiles
		content, err := ioutil.ReadFile(inp)
		if err == nil && len(content) > 0 {
			// parse json profiles
			resp := &profilesJson{}
			err = json.Unmarshal(content, &resp.Profiles)
			if err != nil {
				panic(err)
			}
			for _, profile := range resp.Profiles {
				name := profile.Name
				if name == "" {
					name = "custom_" + common.DefaultProfileName(
						profile.Width,
						profile.Height,
						profile.Bitrate)
				}
				var gop time.Duration
				if profile.GOP != "" {
					if profile.GOP == "intra" {
						gop = ffmpeg.GOPIntraOnly
					} else {
						gopFloat, err := strconv.ParseFloat(profile.GOP, 64)
						if err != nil {
							panic(err)
						}
						if gopFloat <= 0.0 {
							panic("invalid gop value")
						}
						gop = time.Duration(gopFloat * float64(time.Second))
					}
				}
				encodingProfile, err := common.EncoderProfileNameToValue(profile.Profile)
				if err != nil {
					panic(err)
				}
				prof := ffmpeg.VideoProfile{
					Name:         name,
					Bitrate:      fmt.Sprint(profile.Bitrate),
					Framerate:    profile.FPS,
					FramerateDen: profile.FPSDen,
					Resolution:   fmt.Sprintf("%dx%d", profile.Width, profile.Height),
					Profile:      encodingProfile,
					GOP:          gop,
				}
				profs = append(profs, prof)
			}
		} else {
			// check the built-in profiles
			profs = make([]ffmpeg.VideoProfile, 0)
			presets := strings.Split(inp, ",")
			for _, v := range presets {
				if p, ok := ffmpeg.VideoProfileLookup[strings.TrimSpace(v)]; ok {
					profs = append(profs, p)
				}
			}
		}
		if len(profs) <= 0 {
			panic(fmt.Errorf("No transcoding profiles found"))
		}
	}
	return profs
}
