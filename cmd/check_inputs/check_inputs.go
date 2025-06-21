package main

import (
	"flag"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/livepeer/lpms/ffmpeg"
)

func main() {
	inputs := getInputs()
	for _, input := range inputs {
		runTranscode(input)
	}
}

func runTranscode(input string) {
	accel := ffmpeg.Nvidia
	res, err := ffmpeg.Transcode3(&ffmpeg.TranscodeOptionsIn{
		Fname: input,
		Accel: accel,
	}, []ffmpeg.TranscodeOptions{{
		Oname:        "-",
		Accel:        accel,
		Profile:      ffmpeg.P240p30fps16x9,
		AudioEncoder: ffmpeg.ComponentOptions{Name: "copy"},
		Muxer:        ffmpeg.ComponentOptions{Name: "null"},
	}})
	if err != nil {
		slog.Error("Error transcoding", "input", input, "err", err)
	}
	if res.Encoded[0].Frames > res.Decoded.Frames*3 {
		slog.Warn("Too many frames!", "input", input, "decoded", res.Decoded.Frames, "encoded", res.Encoded[0].Frames)
	} else {
		slog.Info("Transcoding OK", "input", input, "decoded", res.Decoded.Frames, "encoded", res.Encoded[0].Frames)
	}
}

func getInputs() []string {
	// Command‐line flags
	location := flag.String("loc", "", "File or directory to scan for '*tempfile'")
	startStr := flag.String("start", "", "Start time in RFC3339 format (e.g. 2025-05-21T15:04:05Z)")
	endStr := flag.String("end", "", "End time in RFC3339 format")
	flag.Parse()

	if *location == "" {
		slog.Error("A -location must be provided")
		flag.Usage()
		os.Exit(1)
	}

	// Parse start/end times if given
	var (
		startTime time.Time
		endTime   time.Time
		err       error
		hasStart  = false
		hasEnd    = false
	)
	if *startStr != "" {
		startTime, err = time.Parse(time.RFC3339, *startStr)
		if err != nil {
			slog.Error("Invalid -start time", "err", err)
			os.Exit(1)
		}
		hasStart = true
	}
	if *endStr != "" {
		endTime, err = time.Parse(time.RFC3339, *endStr)
		if err != nil {
			slog.Error("Invalid -end time", "err", err)
			os.Exit(1)
		}
		hasEnd = true
	}

	// Collect matching inputs
	inputs := []string{}
	info, err := os.Stat(*location)
	if err != nil || info == nil {
		slog.Error("Unable to stat location", "location", *location, "err", err)
		os.Exit(1)
	}

	// Helper to decide if a file passes filters
	matches := func(path string, fi os.FileInfo) bool {
		if !strings.HasSuffix(fi.Name(), "tempfile") {
			return false
		}
		mtime := fi.ModTime()
		if hasStart && mtime.Before(startTime) {
			return false
		}
		if hasEnd && mtime.After(endTime) {
			return false
		}
		return true
	}

	if info.IsDir() {
		// Scan directory (non‐recursive)
		entries, err := os.ReadDir(*location)
		if err != nil {
			slog.Error("Failed to read directory", "dir", *location, "err", err)
			os.Exit(1)
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			// must end with "tempfile"
			if !strings.HasSuffix(name, "tempfile") {
				continue
			}
			// must not begin with "out_"
			if strings.HasPrefix(name, "out_") {
				continue
			}
			full := filepath.Join(*location, name)
			fi, err := e.Info()
			if err != nil {
				slog.Warn("Skipping file (cannot stat)", "file", full, "err", err)
				continue
			}
			if matches(full, fi) {
				inputs = append(inputs, full)
			}
		}
	} else {
		// Single file
		inputs = append(inputs, *location)
	}

	if len(inputs) == 0 {
		slog.Info("No matching tempfile inputs found")
		os.Exit(1)
	}
	return inputs
}
