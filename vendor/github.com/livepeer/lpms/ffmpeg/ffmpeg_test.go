package ffmpeg

import (
	"math"
	"os/exec"
	"strconv"
	"strings"
	"testing"
)

func TestLength(t *testing.T) {
	InitFFmpeg()
	defer DeinitFFmpeg()
	inp := "../transcoder/test.ts"
	// Extract packet count of sample from ffprobe
	// XXX enhance MediaLength to actually return media stats
	cmd := "ffprobe -loglevel quiet -hide_banner "
	cmd += "-select_streams v  -show_streams -count_packets "
	cmd += inp + " | grep -oP 'nb_read_packets=\\K.*$'"
	out, err := exec.Command("bash", "-c", cmd).Output()
	nb_packets, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		t.Error("Could not extract packet count from sample", err)
	}

	// Extract length of test vid (in seconds) from ffprobe
	cmd = "ffprobe -loglevel quiet -hide_banner "
	cmd += "-select_streams v  -show_streams -count_packets "
	cmd += inp + " | grep -oP 'duration=\\K.*$'"
	out, err = exec.Command("bash", "-c", cmd).Output()
	ts_f, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	if err != nil {
		t.Error("Could not extract timestamp from sample", err)
	}
	ts := int(math.Ceil(ts_f * 1000.0))

	// sanity check baseline numbers
	err = CheckMediaLen(inp, ts, nb_packets)
	if err != nil {
		t.Error("Media sanity check failed")
	}

	err = CheckMediaLen(inp, ts/2, nb_packets)
	if err == nil {
		t.Error("Did not get an error on ts check where one was expected")
	}

	err = CheckMediaLen(inp, ts, nb_packets/2)
	if err == nil {
		t.Error("Did not get an error on nb packets check where one was expected")
	}

	// check invalid file
	err = CheckMediaLen("nonexistent", ts, nb_packets)
	if err == nil || err.Error() != "No such file or directory" {
		t.Error("Did not get the expected error: ", err)
	}
}
