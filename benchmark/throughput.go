package benchmark

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/lpms/ffmpeg"
)

const (
	bunnyVideo     = "official_test_source_2s_keys_24pfs.mp4"
	generatedVideo = "official_generated_test_source_24fps.mp4"
	// hdVideo        = "/Volumes/darkbookext/Downloads/Sintel.2010.1080p.mkv"
	hdVideo       = "/Volumes/darkbookext/Downloads/Sintel.2010.1080p.stereo.mp4"
	hdVideoLowBps = "/Volumes/darkbookext/Downloads/Sintel.2010.1080p.3700k.mp4"

	cvsHeaders = "Date,Video card,Acceleration, GPU device index, Source file, Simultaneous transcodes, Transcode duration (sec), MPixels/sec, Frames/sec, MB/sec, Profiles num, Profiles, In pixels, Out pixels, In frames, Out frames, In bytes, Out bytes"
)

var (
	p720p30fps16x9B8   = ffmpeg.VideoProfile{Name: "P720p30fps16x9", Bitrate: "8000k", Framerate: 30, AspectRatio: "16:9", Resolution: "1280x720"}
	p720p30fps16x9B4   = ffmpeg.VideoProfile{Name: "P720p30fps16x9", Bitrate: "4000k", Framerate: 30, AspectRatio: "16:9", Resolution: "1280x720"}
	p1080p24fps16x9B5  = ffmpeg.VideoProfile{Name: "P1080p24fps16x9", Bitrate: "5000k", Framerate: 24, AspectRatio: "16:9", Resolution: "1920x1080"}
	p1080p24fps16x9B10 = ffmpeg.VideoProfile{Name: "P1080p24fps16x9", Bitrate: "10000k", Framerate: 24, AspectRatio: "16:9", Resolution: "1920x1080"}

	// profilesSuite = [][]ffmpeg.VideoProfile{{ffmpeg.P144p30fps16x9}, {ffmpeg.P720p30fps16x9},
	// 	{ffmpeg.P144p30fps16x9, ffmpeg.P240p30fps16x9, ffmpeg.P360p30fps16x9, ffmpeg.P576p30fps16x9, ffmpeg.P720p30fps16x9}}
	// sourcesSuite = []string{generatedVideo, bunnyVideo}
	profilesSuite   = [][]ffmpeg.VideoProfile{{ffmpeg.P144p30fps16x9}}
	sourcesSuite    = []string{generatedVideo}
	repeatsNumber   = 2
	maxSimultaneous = 3
)

type (
	benchmarkResults struct {
		Acceleration           ffmpeg.Acceleration
		SimultaneousTranscodes int
		Device                 string
		InPixels               int64
		OutPixels              int64
		InFrames               int64
		OutFrames              int64
		InSize                 int64
		OutSize                int64
		VideoDuration          time.Duration
		TranscodeDuration      time.Duration
		Profiles               []ffmpeg.VideoProfile
		SourceFileName         string
		PixelsSec              float64
		FramesSec              float64
		BytesSec               float64
		Speed                  float64
	}
	benchmarkResultsArray []benchmarkResults
)

// StartThroughput starts the benchmark
func StartThroughput(rawDevices string) {
	glog.Infof("Starting throughput benchmark")
	checkIfTestVideoFilesExists()
	devices := strings.Split(rawDevices, ",")
	devicesNum := len(devices)
	if devicesNum == 0 {
		devicesNum = 1
	}

	// sourceFileName := hdVideo
	// sourceFileName := generatedVideo
	workDir := "/Volumes/darkbookext/tmp"

	// profiles := []ffmpeg.VideoProfile{p720p30fps16x9B8, p720p30fps16x9B4}
	// profiles := []ffmpeg.VideoProfile{p720p30fps16x9B8}
	// profiles := []ffmpeg.VideoProfile{p1080p24fps16x9B10}
	// profiles := []ffmpeg.VideoProfile{ffmpeg.P240p30fps4x3}
	videoCardName := getVideoCardName()
	now := time.Now()
	csvResults := make([]string, 0)
	var mux sync.Mutex
	for i, sourceFileName := range sourcesSuite {
		for j, profiles := range profilesSuite {
			for currentSimGPUS := 1; currentSimGPUS <= devicesNum; currentSimGPUS++ {
				for simNum := 1; simNum < maxSimultaneous; simNum++ {
					// now run on each GPU, simultaneously
					var gpuwg sync.WaitGroup
					for currentGPUIndex := 0; currentGPUIndex < currentSimGPUS; currentGPUIndex++ {
						gpuwg.Add(1)
						go func(gpuIndex int) {
							device := ""
							if gpuIndex < len(devices) {
								device = devices[gpuIndex]
							}
							bra := make(benchmarkResultsArray, 0)
							for k := 0; k < repeatsNumber; k++ {
								glog.Infof("Running benchmark %d:%d:%d on %d GPUs at once, current GPU %s %s - %s", i, j, k, currentSimGPUS, device,
									sourceFileName, profiles2str(profiles))
								var waitgroup sync.WaitGroup
								for stream := 0; stream <= simNum; stream++ {
									waitgroup.Add(1)
									go func(iStream int) {
										benchRes := doOneTranscode(sourceFileName, device, workDir, profiles, simNum)
										glog.Infof("Benchmark results (stream %d):", iStream)
										glog.Info(benchRes.String())
										mux.Lock()
										bra = append(bra, benchRes)
										mux.Unlock()
										waitgroup.Done()
									}(stream)
								}
								waitgroup.Wait()
							}
							benchRes := bra.avg()
							csvResults = append(csvResults, fmt.Sprintf("%s,%s,", now, videoCardName)+benchRes.CSV())
							gpuwg.Done()
						}(currentGPUIndex)
						gpuwg.Wait()
					}
				}
			}
		}
	}
	fmt.Println("====== RESULTS:")
	fmt.Println(cvsHeaders)
	fmt.Println(strings.Join(csvResults, "\n"))
}

func doOneTranscode(sourceFileName, device, workDir string, profiles []ffmpeg.VideoProfile, simultaneousTranscodes int) benchmarkResults {
	stats, err := os.Stat(sourceFileName)
	if err != nil {
		glog.Fatal(err)
	}
	sourceSize := stats.Size()
	res := benchmarkResults{
		InSize: sourceSize,
	}
	accel := ffmpeg.Software
	// accel := ffmpeg.Nvidia

	// Set up in / out config
	in := &ffmpeg.TranscodeOptionsIn{
		Fname:  sourceFileName,
		Accel:  accel,
		Device: device,
		// Accel:  ffmpeg.Nvidia,
		// Device: nv.getDevice(),
	}
	opts := profilesToTranscodeOptions(workDir, accel, profiles)

	// Do the Transcoding
	start := time.Now()
	transRes, err := ffmpeg.Transcode3(in, opts)
	took := time.Since(start)
	glog.Infof("Transcoding took %s", took)
	if err != nil {
		glog.Fatal(err)
	}
	glog.Infof("Res: %+v", transRes)
	res.InPixels = transRes.Decoded.Pixels
	res.InFrames = int64(transRes.Decoded.Frames)
	for i, tr := range transRes.Encoded {
		res.OutFrames += int64(tr.Frames)
		res.OutPixels += tr.Pixels
		outStats, err := os.Stat(opts[i].Oname)
		if err != nil {
			glog.Fatal(err)
		}
		res.OutSize += outStats.Size()
	}
	res.TranscodeDuration = took
	res.Profiles = make([]ffmpeg.VideoProfile, len(profiles))
	res.SourceFileName = filepath.Base(sourceFileName)
	res.Acceleration = in.Accel
	res.SimultaneousTranscodes = simultaneousTranscodes
	res.Device = device
	copy(res.Profiles, profiles)
	res.calc()

	return res
}

func (br *benchmarkResults) String() string {
	return fmt.Sprintf(`Transcode time: %s
MPixels/sec: %v In Pixels: %v Out pixels: %v
FramesSec: %v In frames: %v Out frames: %v
MBytesSec: %v In bytes: %v Out bytes: %v
`, br.TranscodeDuration, br.PixelsSec/1000000.0, br.InPixels, br.OutPixels, br.FramesSec, br.InFrames, br.OutFrames, br.BytesSec/1000000.0, br.InSize, br.OutSize)
}

func (br *benchmarkResults) CSV() string {
	// Acceleration, Gpu device, Source file, Simultaneous transcodes, Transcode duration (sec), MPixels/sec, Frames/sec, MB/sec, Profiles num, Profiles, In pixels, Out pixels, In frames, Out frames, In bytes, Out bytes
	device := br.Device
	if device == "" {
		device = "Unknown"
	}
	return fmt.Sprintf("%s,%s,%s,%d,%v,%v,%v,%v,%v,%v,%v,%v,%v,%v,%v,%v", accel2str(br.Acceleration), device, br.SourceFileName, br.SimultaneousTranscodes, br.TranscodeDuration.Seconds(),
		br.PixelsSec/1000000.0, br.FramesSec, br.BytesSec/1000000.0, len(br.Profiles), profiles2str(br.Profiles), br.InPixels, br.OutPixels,
		br.InFrames, br.OutFrames, br.InSize, br.OutSize)
}

func (bra benchmarkResultsArray) avg() benchmarkResults {
	res := benchmarkResults{}
	l := int64(len(bra))
	for _, br := range bra {
		res.InFrames += br.InFrames
		res.OutFrames += br.OutFrames
		res.InSize += br.InSize
		res.OutSize += br.OutSize
		res.InPixels += br.InPixels
		res.OutPixels += br.OutPixels
		res.Profiles = br.Profiles
		res.TranscodeDuration += br.TranscodeDuration
		res.VideoDuration += br.VideoDuration
		res.SourceFileName = br.SourceFileName
		res.Acceleration = br.Acceleration
		res.SimultaneousTranscodes = br.SimultaneousTranscodes
		res.Device = br.Device
	}
	res.InFrames /= l
	res.OutFrames /= l
	res.InPixels /= l
	res.OutPixels /= l
	res.InSize /= l
	res.OutSize /= l
	res.TranscodeDuration /= time.Duration(l)
	res.VideoDuration /= time.Duration(l)
	res.calc()
	return res
}

func profiles2str(profiles []ffmpeg.VideoProfile) string {
	pn := make([]string, len(profiles))
	for i, p := range profiles {
		pn[i] = p.Name
	}
	return strings.Join(pn, ";")
}

func (br *benchmarkResults) calc() {
	allPixels := br.InPixels + br.OutPixels
	allFrames := br.InFrames + br.OutFrames
	allSize := br.InSize + br.OutSize
	transcodeTime := br.TranscodeDuration.Seconds()
	br.PixelsSec = float64(allPixels) / transcodeTime
	br.FramesSec = float64(allFrames) / transcodeTime
	br.BytesSec = float64(allSize) / transcodeTime
	if br.VideoDuration > 0 {
		br.Speed = float64(br.TranscodeDuration) / float64(br.VideoDuration)
	}
}

func accel2str(accel ffmpeg.Acceleration) string {
	switch accel {
	case ffmpeg.Software:
		return "Software"
	case ffmpeg.Nvidia:
		return "Nvidia"
	case ffmpeg.Amd:
		return "Amd"
	}
	return "Unknown"
}

func checkIfTestVideoFilesExists() {
	if _, err := os.Stat(bunnyVideo); os.IsNotExist(err) {
		glog.Fatalf("Video file '%s' not found", bunnyVideo)
	}
	if _, err := os.Stat(generatedVideo); os.IsNotExist(err) {
		glog.Fatalf("Video file '%s' not found", generatedVideo)
	}
}

func getVideoCardName() string {
	output, err := exec.Command("nvidia-smi", "some args").CombinedOutput()
	if err != nil {
		// os.Stderr.WriteString(err.Error())
		return "Unknown"
	}
	fmt.Println(string(output))
	return string(output)
}

func profilesToTranscodeOptions(workDir string, accel ffmpeg.Acceleration, profiles []ffmpeg.VideoProfile) []ffmpeg.TranscodeOptions {
	opts := make([]ffmpeg.TranscodeOptions, len(profiles), len(profiles))
	for i := range profiles {
		o := ffmpeg.TranscodeOptions{
			Oname:   fmt.Sprintf("%s/out_%s.ts", workDir, common.RandName()),
			Profile: profiles[i],
			Accel:   accel,
			// AudioEncoder: ffmpeg.ComponentOptions{Name: "copy"},
			AudioEncoder: ffmpeg.ComponentOptions{Name: "aac"},
		}
		opts[i] = o
	}
	return opts
}
