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
	hdVideo        = "Sintel.2010.1080p.stereo.mp4"
	hdVideoLowBps  = "Sintel.2010.1080p.3700k.mp4"
	bunnyHD30Video = "bbb_sunflower_1080p_30fps_normal.mp4"
	bunnyHD60Video = "bbb_sunflower_1080p_60fps_normal.mp4"

	csvHeaders = "Versions,Date,Video card,Acceleration, GPU device index,Simultaneous GPUs, Source file, Simultaneous transcodes, Transcode duration (sec), Video duration (sec), Speed, xRealtime, MPixels/sec, Frames/sec, MB/sec, Profiles num, Profiles, In pixels, Out pixels, In frames, Out frames, In bytes, Out bytes"
)

var (
	// videosDurations = make(map[string]time.Duration)
	videosDurations = map[string]time.Duration{bunnyVideo: 596480 * time.Millisecond,
		generatedVideo: 596500 * time.Millisecond, hdVideo: 888050 * time.Millisecond, hdVideoLowBps: 888050 * time.Millisecond,
		bunnyHD30Video: 634530 * time.Millisecond, bunnyHD60Video: 634530 * time.Millisecond}

	p720p30fps16x9B8   = ffmpeg.VideoProfile{Name: "P720p30fps16x9", Bitrate: "8000k", Framerate: 30, AspectRatio: "16:9", Resolution: "1280x720"}
	p720p30fps16x9B4   = ffmpeg.VideoProfile{Name: "P720p30fps16x9", Bitrate: "4000k", Framerate: 30, AspectRatio: "16:9", Resolution: "1280x720"}
	p1080p24fps16x9B5  = ffmpeg.VideoProfile{Name: "P1080p24fps16x9", Bitrate: "5000k", Framerate: 24, AspectRatio: "16:9", Resolution: "1920x1080"}
	p1080p24fps16x9B10 = ffmpeg.VideoProfile{Name: "P1080p24fps16x9", Bitrate: "10000k", Framerate: 24, AspectRatio: "16:9", Resolution: "1920x1080"}

	/* hanging config

	profilesSuite = [][]ffmpeg.VideoProfile{
		{ffmpeg.P720p30fps16x9, ffmpeg.P720p30fps16x9, ffmpeg.P720p30fps16x9, ffmpeg.P720p30fps16x9, ffmpeg.P720p30fps16x9, ffmpeg.P720p30fps16x9, ffmpeg.P720p30fps16x9, ffmpeg.P720p30fps16x9},
	}
	sourcesSuite = []string{generatedVideo, bunnyVideo}
	repeatsNumber = 1
	maxSimultaneous = 45
	minSimultaneous = 10
	simInc          = 4

	*/
	// profilesSuite = [][]ffmpeg.VideoProfile{{ffmpeg.P144p30fps16x9}, {ffmpeg.P720p30fps16x9},
	// 	{ffmpeg.P360p30fps16x9, ffmpeg.P576p30fps16x9},
	// 	{ffmpeg.P144p30fps16x9, ffmpeg.P240p30fps16x9, ffmpeg.P360p30fps16x9, ffmpeg.P576p30fps16x9, ffmpeg.P720p30fps16x9}}
	// profilesSuite = [][]ffmpeg.VideoProfile{{ffmpeg.P144p30fps16x9}, {ffmpeg.P720p30fps16x9},
	// 	{ffmpeg.P144p30fps16x9, ffmpeg.P240p30fps16x9, ffmpeg.P360p30fps16x9, ffmpeg.P576p30fps16x9, ffmpeg.P720p30fps16x9}}
	// profilesSuite = [][]ffmpeg.VideoProfile{{ffmpeg.P144p30fps16x9}, {ffmpeg.P720p30fps16x9}}
	// profilesSuite = [][]ffmpeg.VideoProfile{{ffmpeg.P144p30fps16x9, ffmpeg.P720p30fps16x9}}
	/*
		profilesSuite = [][]ffmpeg.VideoProfile{
			{ffmpeg.P144p30fps16x9},
			{ffmpeg.P720p30fps16x9},
			{ffmpeg.P144p30fps16x9, ffmpeg.P720p30fps16x9},
			{ffmpeg.P144p30fps16x9, ffmpeg.P240p30fps16x9, ffmpeg.P360p30fps16x9},
			{ffmpeg.P144p30fps16x9, ffmpeg.P240p30fps16x9, ffmpeg.P360p30fps16x9, ffmpeg.P576p30fps16x9},
			{ffmpeg.P144p30fps16x9, ffmpeg.P240p30fps16x9, ffmpeg.P360p30fps16x9, ffmpeg.P576p30fps16x9, ffmpeg.P720p30fps16x9},
		}
	*/
	profilesSuite = [][]ffmpeg.VideoProfile{
		{ffmpeg.P144p30fps16x9},
		{ffmpeg.P144p30fps16x9, ffmpeg.P720p30fps16x9},
	}
	// profilesSuite = [][]ffmpeg.VideoProfile{
	// 	{ffmpeg.P144p30fps16x9, ffmpeg.P240p30fps16x9, ffmpeg.P360p30fps16x9, ffmpeg.P576p30fps16x9, ffmpeg.P720p30fps16x9},
	// }
	// profilesSuite = [][]ffmpeg.VideoProfile{
	// 	{ffmpeg.P720p30fps16x9, ffmpeg.P720p30fps16x9, ffmpeg.P720p30fps16x9, ffmpeg.P720p30fps16x9, ffmpeg.P720p30fps16x9, ffmpeg.P720p30fps16x9, ffmpeg.P720p30fps16x9, ffmpeg.P720p30fps16x9},
	// }
	sourcesSuite = []string{generatedVideo, bunnyVideo}
	// sourcesSuite = []string{generatedVideo, bunnyVideo, hdVideo, hdVideoLowBps}
	// profilesSuite = [][]ffmpeg.VideoProfile{{ffmpeg.P144p30fps16x9}}
	// sourcesSuite    = []string{generatedVideo}
	// sourcesSuite    = []string{bunnyVideo}
	repeatsNumber = 1
	// maxSimultaneous = 1
	// minSimultaneous = 1
	// simInc          = 1
	// for P100
	maxSimultaneous = 45
	minSimultaneous = 30
	simInc          = 5
	// for K80
	// maxSimultaneous = 30
	// minSimultaneous = 1
	// simInc          = 3
)

type (
	benchmarker struct {
		acceleration     ffmpeg.Acceleration
		profiles         []ffmpeg.VideoProfile
		sourceFileName   string
		workDir          string
		device           string
		simultaneousGPUs int
		results          benchmarkResultsArray
		mux              sync.Mutex
	}

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
		SimultaneousGPUs       int
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
func StartThroughput(rawDevices string, _repeatsNumber, _minSimultaneous, _maxSimultaneous, _simInc int, sources, profiles string) {
	glog.Infof("Starting throughput benchmark repeatsNumber: %d min simultaneous: %d max simultaneous: %d simInc: %d", _repeatsNumber, _minSimultaneous,
		_maxSimultaneous, _simInc)
	checkIfTestVideoFilesExists(sourcesSuite)
	accel := ffmpeg.Software
	devicesNum := 1
	var devices []string
	if rawDevices != "" {
		devices = strings.Split(rawDevices, ",")
		devicesNum = len(devices)
		accel = ffmpeg.Nvidia
	}
	glog.Infof("devices num: %d rawDevices: %s", devicesNum, rawDevices)
	repeatsNumber = _repeatsNumber
	minSimultaneous = _minSimultaneous
	maxSimultaneous = _maxSimultaneous
	simInc = _simInc
	if sources != "" {
		sourcesSuite = strings.Split(sources, ",")
		for _, name := range sourcesSuite {
			if _, has := videosDurations[name]; !has {
				glog.Fatalf("Unknown source: %s", name)
			}
		}
	}
	if profiles != "" {
		profilesSuite = make([][]ffmpeg.VideoProfile, 0)
		prof1 := strings.Split(profiles, ";")
		for _, prof := range prof1 {
			profs := make([]ffmpeg.VideoProfile, 0)
			ps := strings.Split(prof, ",")
			for _, v := range ps {
				if p, ok := ffmpeg.VideoProfileLookup[strings.TrimSpace(v)]; ok {
					profs = append(profs, p)
				} else {
					glog.Fatalf("Unknown profile: %s", v)
				}
			}
			if len(profs) > 0 {
				profilesSuite = append(profilesSuite, profs)
			}
		}
	}

	glog.Infof("sources: %+v sources suite: %+v", sources, sourcesSuite)
	glog.Infof("profiles: %+v", profiles)

	workDir := "/disk-1-temp"
	if _, err := os.Stat(workDir); os.IsNotExist(err) {
		workDir = "/tmp"
	}

	// profiles := []ffmpeg.VideoProfile{p720p30fps16x9B8, p720p30fps16x9B4}
	// profiles := []ffmpeg.VideoProfile{p720p30fps16x9B8}
	// profiles := []ffmpeg.VideoProfile{p1080p24fps16x9B10}
	// profiles := []ffmpeg.VideoProfile{ffmpeg.P240p30fps4x3}
	videoCardName, version := getVideoCardName()
	glog.Infof("Using acceleration: %s video card: %s ver %s devices: %+v", accel2str(accel), videoCardName, version, devices)
	now := time.Now()
	csvResults := make([]string, 0)
	for _, sourceFileName := range sourcesSuite {
		for _, profiles := range profilesSuite {
			for currentSimGPUS := devicesNum; currentSimGPUS <= devicesNum; currentSimGPUS++ {
				benchmarkers := make([]*benchmarker, 0)
				for u := 0; u < currentSimGPUS; u++ {
					device := ""
					if u < len(devices) {
						device = devices[u]
					}
					benchmarkers = append(benchmarkers, newBenchmarker(accel, profiles, sourceFileName, device, workDir, currentSimGPUS))
				}
				glog.Infof("Number of benchmarkers: %d", len(benchmarkers))
				for simNum := minSimultaneous; simNum <= maxSimultaneous; simNum += simInc {
					for k := 0; k < repeatsNumber; k++ {
						// glog.Infof("Running benchmark %d:%d:%d on %d GPUs at once, current GPU %s %s - %s", i, j, k, currentSimGPUS, device,
						// 	sourceFileName, profiles2str(profiles))
						glog.Infof("Running benchmarks repeat %d simultaneous streams: %d", k, simNum)
						var waitgroup sync.WaitGroup
						for _, b := range benchmarkers {
							waitgroup.Add(simNum)
							go b.Start(simNum, &waitgroup)
						}
						waitgroup.Wait()
					}
					lastSpeed := 0.0
					for _, b := range benchmarkers {
						br := b.Results()
						csvResults = append(csvResults, fmt.Sprintf("%s,%s,%s,", version, now, videoCardName)+br.CSV())
						lastSpeed = br.Speed
						b.CleanResults()
					}
					fmt.Printf("====== INTERMEDIATE RESULTS (up to now tests took %s):\n", time.Since(now))
					fmt.Println(csvHeaders)
					fmt.Println(strings.Join(csvResults, "\n"))
					if 1/lastSpeed < float64(simNum) {
						glog.Infof("1/lastSpeed: %v simNum: %v", 1/lastSpeed, simNum)
						// stop increasing simultaneous streams if already slower than realtime
						break
					}
				}
			}
		}
	}
	fmt.Printf("====== RESULTS (whole tests took %s):\n", time.Since(now))
	fmt.Println(csvHeaders)
	fmt.Println(strings.Join(csvResults, "\n"))
}

func doOneTranscode(accel ffmpeg.Acceleration, sourceFileName, device, workDir string, profiles []ffmpeg.VideoProfile, simultaneousTranscodes int) benchmarkResults {
	stats, err := os.Stat(sourceFileName)
	if err != nil {
		glog.Fatal(err)
	}
	sourceSize := stats.Size()
	res := benchmarkResults{
		InSize:        sourceSize,
		VideoDuration: videosDurations[sourceFileName],
	}

	// Set up in / out config
	in := &ffmpeg.TranscodeOptionsIn{
		Fname:  sourceFileName,
		Accel:  accel,
		Device: device,
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
		err = os.Remove(opts[i].Oname)
		if err != nil {
			glog.Fatal(err)
		}
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
	// Acceleration, Gpu device, Simultaneous GPUs, Source file, Simultaneous transcodes, Transcode duration (sec), Video duration (sec), Speed, X Realtime, MPixels/sec, Frames/sec, MB/sec, Profiles num, Profiles, In pixels, Out pixels, In frames, Out frames, In bytes, Out bytes
	device := br.Device
	if device == "" {
		device = "Unknown"
	}
	return fmt.Sprintf("%s,%s,%d,%s,%d,%v,%v,%v,%v,%v,%v,%v,%v,%v,%v,%v,%v,%v,%v,%v", accel2str(br.Acceleration), device, br.SimultaneousGPUs, br.SourceFileName,
		br.SimultaneousTranscodes, br.TranscodeDuration.Seconds(), br.VideoDuration.Seconds(), br.Speed, 1/br.Speed, br.PixelsSec/1000000.0, br.FramesSec,
		br.BytesSec/1000000.0, len(br.Profiles), profiles2str(br.Profiles), br.InPixels, br.OutPixels, br.InFrames, br.OutFrames, br.InSize, br.OutSize)
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
		res.SimultaneousGPUs = br.SimultaneousGPUs
	}
	// res.InFrames /= l
	// res.OutFrames /= l
	// res.InPixels /= l
	// res.OutPixels /= l
	// res.InSize /= l
	// res.OutSize /= l
	// res.calc()
	allPixels := res.InPixels + res.OutPixels
	allFrames := res.InFrames + res.OutFrames
	allSize := res.InSize + res.OutSize
	res.TranscodeDuration /= time.Duration(l)
	res.VideoDuration /= time.Duration(l)
	transcodeTime := res.TranscodeDuration.Seconds()
	res.PixelsSec = float64(allPixels) / transcodeTime
	res.FramesSec = float64(allFrames) / transcodeTime
	res.BytesSec = float64(allSize) / transcodeTime
	if res.VideoDuration > 0 {
		res.Speed = (float64(res.TranscodeDuration) / float64(res.VideoDuration)) / float64(res.SimultaneousTranscodes)
	}
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
		br.Speed = (float64(br.TranscodeDuration) / float64(br.VideoDuration)) / float64(br.SimultaneousTranscodes)
	}
}

func newBenchmarker(acceleration ffmpeg.Acceleration, profiles []ffmpeg.VideoProfile, sourceFileName, device, workDir string, simGPUs int) *benchmarker {
	p := make([]ffmpeg.VideoProfile, len(profiles))
	copy(p, profiles)
	return &benchmarker{
		acceleration:     acceleration,
		sourceFileName:   sourceFileName,
		device:           device,
		profiles:         p,
		workDir:          workDir,
		simultaneousGPUs: simGPUs,
	}
}

func (bm *benchmarker) Start(simultaneously int, wg *sync.WaitGroup) {
	// glog.Infof("Running benchmark %d:%d:%d on %d GPUs at once, current GPU %s %s - %s", i, j, k, currentSimGPUS, device,
	// 	sourceFileName, profiles2str(profiles))
	glog.Infof("Running benchmark simultaneous streams %d current GPU %s sim GPUs %d %s - %s", simultaneously, bm.device, bm.simultaneousGPUs,
		bm.sourceFileName, profiles2str(bm.profiles))
	for stream := 0; stream < simultaneously; stream++ {
		go func(iStream int) {
			benchRes := doOneTranscode(bm.acceleration, bm.sourceFileName, bm.device, bm.workDir, bm.profiles, simultaneously)
			benchRes.SimultaneousGPUs = bm.simultaneousGPUs
			glog.Infof("Benchmark results (stream %d device %s):", iStream, bm.device)
			glog.Info(benchRes.String())
			bm.mux.Lock()
			bm.results = append(bm.results, benchRes)
			bm.mux.Unlock()
			wg.Done()
		}(stream)
	}
}

func (bm *benchmarker) Results() benchmarkResults {
	return bm.results.avg()
}

func (bm *benchmarker) CleanResults() {
	bm.results = nil
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

func checkIfTestVideoFilesExists(names []string) {
	for _, n := range names {
		if _, err := os.Stat(n); os.IsNotExist(err) {
			glog.Fatalf("Video file '%s' not found", n)
		}
	}
}

func getVideoCardName() (string, string) {
	name := "Unknown"
	driverVer := "Unknown"
	cudaVer := "Unknown"
	mem := ""
	sawMemLine := false
	output, err := exec.Command("nvidia-smi", "-q").CombinedOutput()
	if err != nil {
		return name, driverVer
	}
	fmt.Println(string(output))
	for _, line := range strings.Split(string(output), "\n") {
		if strings.Contains(line, "Product Name") {
			split := strings.Split(line, ":")
			name = strings.TrimSpace(split[1])
		}
		if strings.Contains(line, "Driver Version") {
			split := strings.Split(line, ":")
			driverVer = strings.TrimSpace(split[1])
		}
		if strings.Contains(line, "CUDA Version") {
			split := strings.Split(line, ":")
			cudaVer = strings.TrimSpace(split[1])
		}
		if strings.Contains(line, "FB Memory Usage") {
			sawMemLine = true
		}
		if sawMemLine && mem == "" && strings.Contains(line, "Total") {
			split := strings.Split(line, ":")
			mem = strings.TrimSpace(split[1])
		}
	}
	return name, driverVer + "/" + cudaVer + "/" + mem
}

func profilesToTranscodeOptions(workDir string, accel ffmpeg.Acceleration, profiles []ffmpeg.VideoProfile) []ffmpeg.TranscodeOptions {
	opts := make([]ffmpeg.TranscodeOptions, len(profiles), len(profiles))
	for i := range profiles {
		o := ffmpeg.TranscodeOptions{
			Oname:        fmt.Sprintf("%s/out_%s.ts", workDir, common.RandName()),
			Profile:      profiles[i],
			Accel:        accel,
			AudioEncoder: ffmpeg.ComponentOptions{Name: "copy"},
			// AudioEncoder: ffmpeg.ComponentOptions{Name: "aac"},
		}
		opts[i] = o
	}
	return opts
}
