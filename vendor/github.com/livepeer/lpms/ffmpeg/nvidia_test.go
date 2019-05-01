// +build nvidia

package ffmpeg

import (
	"fmt"
	"os"
	"testing"
)

func TestNvidia_Transcoding(t *testing.T) {
	// Various Nvidia GPU tests for encoding + decoding
	// XXX what is missing is a way to verify these are *actually* running on GPU!

	run, dir := setupTest(t)
	defer os.RemoveAll(dir)

	cmd := `
    set -eux
    cd "$0"

    # set up initial input; truncate test.ts file
    ffmpeg -loglevel warning -i "$1"/../transcoder/test.ts -c:a copy -c:v copy -t 1 test.ts
  `
	run(cmd)

	var err error
	fname := dir + "/test.ts"
	oname := dir + "/out.ts"
	prof := P240p30fps16x9

	// hw dec, sw enc
	err = Transcode2(&TranscodeOptionsIn{
		Fname: fname,
		Accel: Nvidia,
	}, []TranscodeOptions{
		TranscodeOptions{
			Oname:   oname,
			Profile: prof,
			Accel:   Software,
		},
	})
	if err != nil {
		t.Error(err)
	}

	// sw dec, hw enc
	err = Transcode2(&TranscodeOptionsIn{
		Fname: fname,
		Accel: Software,
	}, []TranscodeOptions{
		TranscodeOptions{
			Oname:   oname,
			Profile: prof,
			Accel:   Nvidia,
		},
	})
	if err != nil {
		t.Error(err)
	}

	// hw enc + dec
	err = Transcode2(&TranscodeOptionsIn{
		Fname: fname,
		Accel: Nvidia,
	}, []TranscodeOptions{
		TranscodeOptions{
			Oname:   oname,
			Profile: prof,
			Accel:   Nvidia,
		},
	})
	if err != nil {
		t.Error(err)
	}

	// software transcode for image quality check
	err = Transcode2(&TranscodeOptionsIn{
		Fname: fname,
		Accel: Software,
	}, []TranscodeOptions{
		TranscodeOptions{
			Oname:   dir + "/sw.ts",
			Profile: prof,
			Accel:   Software,
		},
	})
	if err != nil {
		t.Error(err)
	}

	cmd = `
    set -eux
    cd "$0"
    # compare using ssim and generate stats file
    ffmpeg -loglevel warning -i out.ts -i sw.ts -lavfi '[0:v][1:v]ssim=stats.log' -f null -
    # check image quality; ensure that no more than 5 frames have ssim < 0.95
    grep -Po 'All:\K\d+.\d+' stats.log | awk '{ if ($1 < 0.95) count=count+1 } END{ exit count > 5 }'
  `
	run(cmd)

}

func TestNvidia_Pixfmts(t *testing.T) {

	run, dir := setupTest(t)
	defer os.RemoveAll(dir)

	prof := P240p30fps16x9

	// check valid and invalid pixel formats
	cmd := `
    set -eux
    cd "$0"
    cp "$1/../transcoder/test.ts" test.ts

    # sanity check original input type is 420p
    ffmpeg -loglevel warning -i test.ts -an -c:v copy -t 1 in420p.mp4
    ffprobe -loglevel warning in420p.mp4  -show_streams -select_streams v | grep pix_fmt=yuv420p

    # generate unsupported 422p type
    ffmpeg -loglevel warning -i test.ts -an -c:v libx264 -pix_fmt yuv422p -t 1 in422p.mp4
    ffprobe -loglevel warning in422p.mp4  -show_streams -select_streams v | grep pix_fmt=yuv422p

    # generate semi-supported 444p type (encoding only)
    ffmpeg -loglevel warning -i test.ts -an -c:v libx264 -pix_fmt yuv444p -t 1 in444p.mp4
    ffprobe -loglevel warning in444p.mp4  -show_streams -select_streams v | grep pix_fmt=yuv444p
  `
	run(cmd)

	// sanity check
	err := Transcode2(&TranscodeOptionsIn{
		Fname: dir + "/in420p.mp4",
		Accel: Nvidia,
	}, []TranscodeOptions{
		TranscodeOptions{
			Oname:   dir + "/out420p.mp4",
			Profile: prof,
			Accel:   Nvidia,
		},
	})
	if err != nil {
		t.Error(err)
	}

	// check an input pixel format that is not GPU decodeable
	err = Transcode2(&TranscodeOptionsIn{
		Fname: dir + "/in422p.mp4",
		Accel: Nvidia,
	}, []TranscodeOptions{
		TranscodeOptions{
			Oname:   dir + "/out422p.mp4",
			Profile: prof,
			Accel:   Software,
		},
	})
	if err == nil || err.Error() != "Unsupported input pixel format" {
		t.Error(err)
	}

	// 444p is encodeable but not decodeable; produces a different error
	// that is only caught at decode time. Attempt to detect decode bailout.
	err = Transcode2(&TranscodeOptionsIn{
		Fname: dir + "/in444p.mp4",
		Accel: Nvidia,
	}, []TranscodeOptions{
		TranscodeOptions{
			Oname:   dir + "/out444p.mp4",
			Profile: prof,
			Accel:   Nvidia,
		},
	})
	if err == nil || err.Error() != "Invalid data found when processing input" {
		t.Error(err)
	}

	// Software decode an and attempt to encode an invalid GPU pixfmt
	// This implicitly selects a supported pixfmt for output
	// The default Seems to be 444p for P100 but may be 420p for others
	err = Transcode2(&TranscodeOptionsIn{
		Fname: dir + "/in422p.mp4",
		Accel: Software,
	}, []TranscodeOptions{
		TranscodeOptions{
			Oname:   dir + "/out422_to_default.mp4",
			Profile: prof,
			Accel:   Nvidia,
		},
	})
	if err != nil {
		t.Error(err)
	}

	cmd = `
    set -eux
    cd "$0"

    # Check that 420p input produces 420p output for hw -> hw
    ffprobe -loglevel warning out420p.mp4  -show_streams -select_streams v | grep pix_fmt=yuv420p

    # 422p input (with sw decode) produces 444p on P100.
    # Cards that don't do 444p may do 420p instead (untested)
    ffprobe -loglevel warning out422_to_default.mp4  -show_streams -select_streams v | grep 'pix_fmt=\(yuv420p\|yuv444p\)'
  `
	run(cmd)
}

func TestNvidia_Transcoding_Multiple(t *testing.T) {

	// Tests multiple encoding profiles.
	// May be skipped in 'short' mode.

	if testing.Short() {
		t.Skip("Skipping encoding multiple profiles")
	}

	run, dir := setupTest(t)
	defer os.RemoveAll(dir)

	cmd := `
    set -eux
    cd "$0"

    # set up initial input; truncate test.ts file
    ffmpeg -loglevel warning -i "$1"/../transcoder/test.ts -c:a copy -c:v copy -t 1 test.ts

    # sanity check input dimensions for resolution pass through test
    ffprobe -loglevel warning -show_streams -select_streams v test.ts | grep width=1280
    ffprobe -loglevel warning -show_streams -select_streams v test.ts | grep height=720
  `
	run(cmd)

	fname := dir + "/test.ts"
	prof := P240p30fps16x9
	orig := P720p30fps16x9

	mkoname := func(i int) string { return fmt.Sprintf("%s/%d.ts", dir, i) }
	out := []TranscodeOptions{
		TranscodeOptions{
			Oname: mkoname(0),
			// pass through resolution has different behavior;
			// basically bypasses the scale filter
			Profile: orig,
			Accel:   Nvidia,
		},
		TranscodeOptions{
			Oname:   mkoname(1),
			Profile: prof,
			Accel:   Nvidia,
		},
		TranscodeOptions{
			Oname:   mkoname(2),
			Profile: orig,
			Accel:   Software,
		},
		TranscodeOptions{
			Oname:   mkoname(3),
			Profile: prof,
			Accel:   Software,
		},
		// another gpu rendition for good measure?
		TranscodeOptions{
			Oname:   mkoname(4),
			Profile: prof,
			Accel:   Nvidia,
		},
	}

	// generate the above outputs with the given decoder
	test := func(decoder Acceleration) {
		err := Transcode2(&TranscodeOptionsIn{
			Fname: fname,
			Accel: decoder,
		}, out)
		if err != nil {
			t.Error(err)
		}
		// XXX should compare ssim image quality of results
	}
	test(Nvidia)
	test(Software)
}

func TestNvidia_Devices(t *testing.T) {

	// XXX need to verify these are running on the correct GPU
	//     not just that the code runs

	device := os.Getenv("GPU_DEVICE")
	if device == "" {
		t.Skip("Skipping device specific tests; no GPU_DEVICE set")
	}

	_, dir := setupTest(t)
	defer os.RemoveAll(dir)

	var err error
	fname := "../transcoder/test.ts"
	oname := dir + "/out.ts"
	prof := P240p30fps16x9

	// hw enc, sw dec
	err = Transcode2(&TranscodeOptionsIn{
		Fname:  fname,
		Accel:  Nvidia,
		Device: device,
	}, []TranscodeOptions{
		TranscodeOptions{
			Oname:   oname,
			Profile: prof,
			Accel:   Software,
		},
	})
	if err != nil {
		t.Error(err)
	}

	// sw dec, hw enc
	err = Transcode2(&TranscodeOptionsIn{
		Fname: fname,
		Accel: Software,
	}, []TranscodeOptions{
		TranscodeOptions{
			Oname:   oname,
			Profile: prof,
			Accel:   Nvidia,
			Device:  device,
		},
	})
	if err != nil {
		t.Error(err)
	}

	// hw enc + dec
	err = Transcode2(&TranscodeOptionsIn{
		Fname:  fname,
		Accel:  Nvidia,
		Device: device,
	}, []TranscodeOptions{
		TranscodeOptions{
			Oname:   oname,
			Profile: prof,
			Accel:   Nvidia,
		},
	})
	if err != nil {
		t.Error(err)
	}

	// hw enc + hw dec, separate devices
	err = Transcode2(&TranscodeOptionsIn{
		Fname:  fname,
		Accel:  Nvidia,
		Device: "0",
	}, []TranscodeOptions{
		TranscodeOptions{
			Oname:   oname,
			Profile: prof,
			Accel:   Nvidia,
			Device:  "1",
		},
	})
	if err != ErrTranscoderInp {
		t.Error(err)
	}

	// invalid device for decoding
	err = Transcode2(&TranscodeOptionsIn{
		Fname:  fname,
		Accel:  Nvidia,
		Device: "9999",
	}, []TranscodeOptions{
		TranscodeOptions{
			Oname:   oname,
			Profile: prof,
			Accel:   Software,
		},
	})
	if err == nil || err.Error() != "Unknown error occurred" {
		t.Error(fmt.Errorf(fmt.Sprintf("\nError being: '%v'\n", err)))
	}

	// invalid device for encoding
	err = Transcode2(&TranscodeOptionsIn{
		Fname: fname,
		Accel: Software,
	}, []TranscodeOptions{
		TranscodeOptions{
			Oname:   oname,
			Profile: prof,
			Accel:   Nvidia,
			Device:  "9999",
		},
	})
	if err == nil || err.Error() != "Unknown error occurred" {
		t.Error(fmt.Errorf(fmt.Sprintf("\nError being: '%v'\n", err)))
	}
}
