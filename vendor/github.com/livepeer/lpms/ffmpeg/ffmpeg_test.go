package ffmpeg

import (
	"io/ioutil"
	"os"
	"os/exec"
	"testing"
)

func setupTest(t *testing.T) (func(cmd string), string) {
	dir, err := ioutil.TempDir("", t.Name())
	if err != nil {
		t.Fatal(err)
	}
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	InitFFmpeg() // hide some log noise

	// Executes the given bash script and checks the results.
	// The script is passed two arguments:
	// a tempdir and the current working directory.
	cmdFunc := func(cmd string) {
		out, err := exec.Command("bash", "-c", cmd, dir, wd).CombinedOutput()
		if err != nil {
			t.Error(string(out[:]))
		}
	}
	return cmdFunc, dir
}

func TestSegmenter_DeleteSegments(t *testing.T) {
	// Ensure that old segments are deleted as they fall off the playlist

	run, dir := setupTest(t)
	defer os.RemoveAll(dir)

	// sanity check that segmented outputs > playlist length
	cmd := `
		set -eux
		cd "$0"
		# default test.ts is a bit short so make it a bit longer
		cp "$1/../transcoder/test.ts" test.ts
		ffmpeg -loglevel warning -i "concat:test.ts|test.ts|test.ts" -c copy long.ts
		ffmpeg -loglevel warning -i long.ts -c copy -f hls -hls_time 1 long.m3u8
		# ensure we have more segments than playlist length
		[ $(ls long*.ts | wc -l) -ge 6 ]
	`
	run(cmd)

	// actually do the segmentation
	err := RTMPToHLS(dir+"/long.ts", dir+"/out.m3u8", dir+"/out_%d.ts", "1", 0)
	if err != nil {
		t.Error(err)
	}

	// check that segments have been deleted by counting output ts files
	cmd = `
		set -eux
		cd "$0"
		[ $(ls out_*.ts | wc -l) -eq 6 ]
	`
	run(cmd)
}

func TestSegmenter_StreamOrdering(t *testing.T) {
	// Ensure segmented output contains [video, audio] streams in that order
	// regardless of stream ordering in the input

	run, dir := setupTest(t)
	defer os.RemoveAll(dir)

	// Craft an input that has a subtitle, audio and video stream, in that order
	cmd := `
	    set -eux
	    cd "$0"

		# generate subtitle file
		cat <<- EOF > inp.srt
			1
			00:00:00,000 --> 00:00:01,000
			hi
		EOF

		# borrow the test.ts from the transcoder dir, output with 3 streams
		ffmpeg -loglevel warning -i inp.srt -i "$1/../transcoder/test.ts" -c:a copy -c:v copy -c:s mov_text -t 1 -map 0:s -map 1:a -map 1:v test.mp4

		# some sanity checks. these will exit early on a nonzero code
		# check stream count, then indexes of subtitle, audio and video
		[ $(ffprobe -loglevel warning -i test.mp4 -show_streams | grep index | wc -l) -eq 3 ]
		ffprobe -loglevel warning -i test.mp4 -show_streams -select_streams s | grep index=0
		ffprobe -loglevel warning -i test.mp4 -show_streams -select_streams a | grep index=1
		ffprobe -loglevel warning -i test.mp4 -show_streams -select_streams v | grep index=2
	`
	run(cmd)

	// actually do the segmentation
	err := RTMPToHLS(dir+"/test.mp4", dir+"/out.m3u8", dir+"/out_%d.ts", "1", 0)
	if err != nil {
		t.Error(err)
	}

	// check stream ordering in output file. Should be video, then audio
	cmd = `
		set -eux
		cd $0
		[ $(ffprobe -loglevel warning -i out_0.ts -show_streams | grep index | wc -l) -eq 2 ]
		ffprobe -loglevel warning -i out_0.ts -show_streams -select_streams v | grep index=0
		ffprobe -loglevel warning -i out_0.ts -show_streams -select_streams a | grep index=1
	`
	run(cmd)
}

func TestSegmenter_DropLatePackets(t *testing.T) {
	// Certain sources sometimes send packets with out-of-order FLV timestamps
	// (eg, ManyCam on Android when the phone can't keep up)
	// Ensure we drop these packets

	run, dir := setupTest(t)
	defer os.RemoveAll(dir)

	// Craft an input with an out-of-order timestamp
	cmd := `
		set -eux
		cd "$0"
		# borrow segmenter test file, rewrite a timestamp
		cp "$1/../segmenter/test.flv" test.flv

		# Sanity check the last few timestamps are monotonic : 18867,18900,18933
		ffprobe -loglevel quiet -show_packets -select_streams v test.flv | grep dts= | tail -3 | tr '\n' ',' | grep dts=18867,dts=18900,dts=18933,

		# replace ts 18900 at position 2052736 with ts 18833 (0x4991 hex)
		printf '\x49\x91' | dd of=test.flv bs=1 seek=2052736 count=2 conv=notrunc
		# sanity check timestamps are now 18867,18833,18933
		ffprobe -loglevel quiet -show_packets -select_streams v test.flv | grep dts= | tail -3 | tr '\n' ',' | grep dts=18867,dts=18833,dts=18933,

		# sanity check number of frames
		ffprobe -loglevel quiet -count_packets -show_streams -select_streams v test.flv | grep nb_read_packets=569
	`
	run(cmd)

	err := RTMPToHLS(dir+"/test.flv", dir+"/out.m3u8", dir+"/out_%d.ts", "100", 0)
	if err != nil {
		t.Error(err)
	}

	// Now ensure things are as expected
	cmd = `
		set -eux
		cd "$0"

		# check monotonic timestamps (rescaled for the 90khz mpegts timebase)
		ffprobe -loglevel quiet -show_packets -select_streams v out_0.ts | grep dts= | tail -3 | tr '\n' ',' | grep dts=1694970,dts=1698030,dts=1703970,

		# check that we dropped the packet
		ffprobe -loglevel quiet -count_packets -show_streams -select_streams v out_0.ts | grep nb_read_packets=568
	`
	run(cmd)
}

func TestTranscoder_UnevenRes(t *testing.T) {
	// Ensure transcoding still works on input with uneven resolutions
	// and that aspect ratio is maintained

	run, dir := setupTest(t)
	defer os.RemoveAll(dir)

	// Craft an input with an uneven res
	cmd := `
	    set -eux
	    cd "$0"

		# borrow the test.ts from the transcoder dir, output with 123x456 res
		ffmpeg -loglevel warning -i "$1/../transcoder/test.ts" -c:a copy -c:v mpeg4 -s 123x456 test.mp4

		# sanity check resulting resolutions
		ffprobe -loglevel warning -i test.mp4 -show_streams -select_streams v | grep width=123
		ffprobe -loglevel warning -i test.mp4 -show_streams -select_streams v | grep height=456

		# and generate another sample with an odd value in the larger dimension
		ffmpeg -loglevel warning -i "$1/../transcoder/test.ts" -c:a copy -c:v mpeg4 -s 123x457 test_larger.mp4
		ffprobe -loglevel warning -i test_larger.mp4 -show_streams -select_streams v | grep width=123
		ffprobe -loglevel warning -i test_larger.mp4 -show_streams -select_streams v | grep height=457

	`
	run(cmd)

	err := Transcode(dir+"/test.mp4", dir, []VideoProfile{P240p30fps16x9})
	if err != nil {
		t.Error(err)
	}

	err = Transcode(dir+"/test_larger.mp4", dir, []VideoProfile{P240p30fps16x9})
	if err != nil {
		t.Error(err)
	}

	// Check output resolutions
	cmd = `
		set -eux
		cd "$0"
		ffprobe -loglevel warning -show_streams -select_streams v out0test.mp4 | grep width=64
		ffprobe -loglevel warning -show_streams -select_streams v out0test.mp4 | grep height=240
		ffprobe -loglevel warning -show_streams -select_streams v out0test_larger.mp4 | grep width=64
		ffprobe -loglevel warning -show_streams -select_streams v out0test_larger.mp4 | grep height=240
	`
	run(cmd)

	// Transpose input and do the same checks as above.
	cmd = `
		set -eux
		cd "$0"
		ffmpeg -loglevel warning -i test.mp4 -c:a copy -c:v mpeg4 -vf transpose transposed.mp4

		# sanity check resolutions
		ffprobe -loglevel warning -show_streams -select_streams v transposed.mp4 | grep width=456
		ffprobe -loglevel warning -show_streams -select_streams v transposed.mp4 | grep height=123
	`
	run(cmd)

	err = Transcode(dir+"/transposed.mp4", dir, []VideoProfile{P240p30fps16x9})
	if err != nil {
		t.Error(err)
	}

	// Check output resolutions for transposed input
	cmd = `
		set -eux
		cd "$0"
		ffprobe -loglevel warning -show_streams -select_streams v out0transposed.mp4 | grep width=426
		ffprobe -loglevel warning -show_streams -select_streams v out0transposed.mp4 | grep height=114
	`
	run(cmd)

	// check special case of square resolutions
	cmd = `
		set -eux
		cd "$0"
		ffmpeg -loglevel warning -i test.mp4 -c:a copy -c:v mpeg4 -s 123x123 square.mp4

		# sanity check resolutions
		ffprobe -loglevel warning -show_streams -select_streams v square.mp4 | grep width=123
		ffprobe -loglevel warning -show_streams -select_streams v square.mp4 | grep height=123
	`
	run(cmd)

	err = Transcode(dir+"/square.mp4", dir, []VideoProfile{P240p30fps16x9})
	if err != nil {
		t.Error(err)
	}

	// Check output resolutions are still square
	cmd = `
		set -eux
		cd "$0"
		ls
		ffprobe -loglevel warning -i out0square.mp4 -show_streams -select_streams v | grep width=426
		ffprobe -loglevel warning -i out0square.mp4 -show_streams -select_streams v | grep height=426
	`
	run(cmd)

	// TODO set / check sar/dar values?
}

func TestTranscoder_SampleRate(t *testing.T) {

	run, dir := setupTest(t)
	defer os.RemoveAll(dir)

	// Craft an input with 48khz audio
	cmd := `
		set -eux
		cd $0

		# borrow the test.ts from the transcoder dir, output with 48khz audio
		ffmpeg -loglevel warning -i "$1/../transcoder/test.ts" -c:v copy -af 'aformat=sample_fmts=fltp:channel_layouts=stereo:sample_rates=48000' -c:a aac -t 1.1 test.ts

		# sanity check results to ensure preconditions
		ffprobe -loglevel warning -show_streams -select_streams a test.ts | grep sample_rate=48000

		# output timestamp check as a script to reuse for post-transcoding check
		cat <<- 'EOF' > check_ts
			set -eux
			# ensure 1 second of timestamps add up to within 2.1% of 90khz (mpegts timebase)
			# 2.1% is the margin of error, 1024 / 48000 (% increase per frame)
			# 1024 = samples per frame, 48000 = samples per second

			# select last frame pts, subtract from first frame pts, check diff
			ffprobe -loglevel warning -show_frames  -select_streams a "$2"  | grep pkt_pts= | head -"$1" | awk 'BEGIN{FS="="} ; NR==1 { fst = $2 } ; END{ diff=(($2-fst)/90000); exit diff <= 0.979 || diff >= 1.021 }'
		EOF
		chmod +x check_ts

		# check timestamps at the given frame offsets. 47 = ceil(48000/1024)
		./check_ts 47 test.ts

		# check failing cases; use +2 since we may be +/- the margin of error
		[ $(./check_ts 45 test.ts || echo "shouldfail") = "shouldfail" ]
		[ $(./check_ts 49 test.ts || echo "shouldfail") = "shouldfail" ]
	`
	run(cmd)

	err := Transcode(dir+"/test.ts", dir, []VideoProfile{P240p30fps16x9})
	if err != nil {
		t.Error(err)
	}

	// Ensure transcoded sample rate is 44k.1hz and check timestamps
	cmd = `
		set -eux
		cd "$0"
		ffprobe -loglevel warning -show_streams -select_streams a out0test.ts | grep sample_rate=44100

		# Sample rate = 44.1khz, samples per frame = 1024
		# Frames per second = ceil(44100/1024) = 44

		# Technically check_ts margin of error is 2.1% due to 48khz rate
		# At 44.1khz, error is 2.3% so we'll just accept the tighter bounds

		# check timestamps at the given frame offsets. 44 = ceil(48000/1024)
		./check_ts 44 out0test.ts

		# check failing cases; use +2 since we may be +/- the margin of error
		[ $(./check_ts 46 out0test.ts || echo "shouldfail") = "shouldfail" ]
		[ $(./check_ts 42 out0test.ts || echo "shouldfail") = "shouldfail" ]
	`
	run(cmd)

}

func TestTranscoder_Timestamp(t *testing.T) {
	run, dir := setupTest(t)
	defer os.RemoveAll(dir)

	cmd := `
		set -eux
		cd $0

		# prepare the input and sanity check 60fps
		cp "$1/../transcoder/test.ts" inp.ts
		ffprobe -loglevel warning -select_streams v -show_streams -count_frames inp.ts > inp.out
		grep avg_frame_rate=60 inp.out
		grep r_frame_rate=60 inp.out

		# reduce 60fps original to 30fps indicated but 15fps real
		ffmpeg -loglevel warning -i inp.ts -t 1 -c:v libx264 -an -vf select='not(mod(n\,4))' -r 30 test.ts
		ffprobe -loglevel warning -select_streams v -show_streams -count_frames test.ts > test.out

		# sanity check some properties. hard code numbers for now.
		grep avg_frame_rate=30 test.out
		grep r_frame_rate=15 test.out
		grep nb_read_frames=15 test.out
		grep duration_ts=90000 test.out
		grep start_pts=138000 test.out
	`
	run(cmd)

	err := Transcode(dir+"/test.ts", dir, []VideoProfile{P240p30fps16x9})
	if err != nil {
		t.Error(err)
	}

	cmd = `
		set -eux
		cd $0

		# hardcode some checks for now. TODO make relative to source.
		ffprobe -loglevel warning -select_streams v -show_streams -count_frames out0test.ts > test.out
		grep avg_frame_rate=30 test.out
		grep r_frame_rate=30 test.out
		grep nb_read_frames=28 test.out
		grep duration_ts=84000 test.out
		grep start_pts=138000 test.out
	`
	run(cmd)
}
