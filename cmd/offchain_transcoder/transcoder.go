package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"os/signal"
	"path"
	"strconv"
	"sync"
	"time"

	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/server"
	"github.com/livepeer/lpms/ffmpeg"
)

type TranscodeOutput struct {
	Profile *ffmpeg.JsonProfile
	Mpegts  []byte
}

const RESULTS_SIZE = 50

// Mock for integration tests to skip all network logic but retain transcoding functionality.
type TranscodingServer struct {
	port int

	httpMux *http.ServeMux
	server  *http.Server
	mutex   sync.Mutex
	Results chan ffmpeg.TranscodeResults

	dumpInput bool
	jobIndex  int
}

func (s *TranscodingServer) Init() {
	s.Results = make(chan ffmpeg.TranscodeResults, RESULTS_SIZE+1)

	s.httpMux = http.NewServeMux()
	s.httpMux.HandleFunc("/live/", s.handler)
	s.server = &http.Server{Addr: fmt.Sprintf("0.0.0.0:%d", s.port), Handler: s.httpMux}
	go func() {
		if err := s.server.ListenAndServe(); err != nil {
			if err.Error() == "http: Server closed" {
				return // normal exit
			}
			fmt.Printf("server.ListenAndServe() %v\n", err)
		}
	}()
}

func (s *TranscodingServer) DumpInput() {
	s.dumpInput = true
}

func (s *TranscodingServer) nextIndex() int {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	index := s.jobIndex
	s.jobIndex += 1
	return index
}

func (s *TranscodingServer) handler(w http.ResponseWriter, req *http.Request) {
	name := fmt.Sprintf("%s %s", req.Method, req.URL.Path)
	fail := func(where string, err error) {
		problem := fmt.Sprintf("Error %s: %s %v\n", name, where, err)
		fmt.Print(problem)
		http.Error(w, problem, 400)
	}
	start := time.Now()
	transcodeConfigurationHeader := req.Header.Get(server.LIVERPEER_TRANSCODE_CONFIG_HEADER)
	if transcodeConfigurationHeader == "" {
		fail("missing transcodeConfigurationHeader", nil)
		return
	}
	var transcodeConfiguration server.AuthWebhookResponse
	if err := json.Unmarshal([]byte(transcodeConfigurationHeader), &transcodeConfiguration); err != nil {
		fail("AuthWebhookResponse decode", err)
		return
	}
	profileCount := len(transcodeConfiguration.Profiles)
	// // outputs := make([]TranscodeOutput, profileCount)
	outputOptions := make([]ffmpeg.TranscodeOptions, profileCount)
	// Do the transcoding ...
	videoProfiles, err := ffmpeg.ParseProfilesFromJsonProfileArray(transcodeConfiguration.Profiles)
	if err != nil {
		fail("ParseProfilesFromJsonProfileArray", err)
		return
	}

	wd, err := os.Getwd()
	if err != nil {
		fail("os.Getwd()", err)
		return
	}
	jobIndex := s.nextIndex()

	for i := 0; i < profileCount; i++ {
		p := &transcodeConfiguration.Profiles[i]
		outputOptions[i] = ffmpeg.TranscodeOptions{
			Profile: videoProfiles[i],
			Accel:   ffmpeg.Nvidia,
			Oname:   path.Join(wd, fmt.Sprintf("_output_%d_%d_%dx%d_%dfps_%dKbps.ts", time.Now().Unix(), jobIndex, p.Width, p.Height, p.FPS, p.Bitrate/1000)),
		}
		// outputs[i].Profile = p
		name += fmt.Sprintf(" %dx%d@%d_%dKbps", p.Width, p.Height, p.FPS, p.Bitrate/1000)
	}

	// fmt.Printf("transcode task %s \n", name)
	// transcoder := &ffmpeg.PipedTranscoding{}
	// transcoder.SetInput(ffmpeg.TranscodeOptionsIn{Accel: ffmpeg.Nvidia})
	// transcoder.SetOutputs(outputOptions)
	// defer transcoder.ClosePipes()
	// // stream input chunks
	// savePath := ""
	// if s.dumpInput {
	// 	savePath = fmt.Sprintf("%s", strings.ReplaceAll(req.URL.Path, "/", "_"))
	// }
	// go httpBodyToFfmpeg(transcoder, req.Body, savePath)
	// // read output streams into memory to later compose multipart/mixed response
	// ffmpegOutputs := transcoder.GetOutputs()
	// for i := 0; i < len(ffmpegOutputs); i++ {
	// 	go renditionToOutput(&outputs[i], &ffmpegOutputs[i])
	// }
	// // start transcode
	// result, err := transcoder.Transcode()

	fname := path.Join(wd, fmt.Sprintf("_input_%d_%d.ts", time.Now().Unix(), jobIndex))
	inputFile, err := os.Create(fname)
	if err != nil {
		fail("os.Create", err)
		return
	}
	_, err = io.Copy(inputFile, req.Body)
	inputFile.Close()
	if err != nil {
		fail("io.Copy(inputFile, req.Body)", err)
		return
	}
	inOptions := &ffmpeg.TranscodeOptionsIn{Fname: fname, Accel: ffmpeg.Nvidia}
	result, err := ffmpeg.Transcode3(inOptions, outputOptions)
	if err != nil {
		fail(fmt.Sprintf("ffmpeg.Transcode3() in=%v out=%v", inOptions, outputOptions), err)
		return
	}

	// Return response
	// if err := returnResponse(w, req, outputs); err != nil {
	if err := returnResponseFromFiles(w, req, outputOptions); err != nil {
		fmt.Printf("error %s sending multipart response %v\n", name, err)
		return
	}
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
	roundtripTime := time.Since(start)
	fmt.Printf("%s segment transcoded time=%v encoded=%v\n", name, roundtripTime, result.Encoded)
	if len(s.Results) < RESULTS_SIZE {
		s.Results <- *result
	}
	// Remove files if everything went ok:
	os.Remove(fname)
	for i := 0; i < len(outputOptions); i++ {
		os.Remove(outputOptions[i].Oname)
	}
}

func (s *TranscodingServer) Stop() {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if err := s.server.Shutdown(ctx); err != nil {
		fmt.Printf("server.Shutdown() %v\n", err)
	}
}

// Streaming from one of ffmpeg output pipes into output.Mpegts byte array.
func renditionToOutput(output *TranscodeOutput, rendition *ffmpeg.OutputReader) {
	defer rendition.Close()
	chunk := make([]byte, 4096)
	for {
		byteCount, err := rendition.Read(chunk)
		if err == io.EOF {
			return
		}
		if err != nil {
			fmt.Printf("transcode output read error %v\n", err)
			return
		}
		output.Mpegts = append(output.Mpegts, chunk[:byteCount]...)
	}
}

// Streaming from HTTP request body into ffmpeg input pipe.
func httpBodyToFfmpeg(transcoder *ffmpeg.PipedTranscoding, body io.ReadCloser, savePath string) {
	chunk := make([]byte, 4096)
	defer transcoder.WriteClose()

	// Debug code portion:
	var tee func(b []byte) (n int, err error) = func(b []byte) (n int, err error) { return len(b), nil }
	if savePath != "" {
		dumpFile, _ := os.Create(savePath)
		defer dumpFile.Close()
		tee = dumpFile.Write
	}

	for {
		// read HTTP body
		size, err := body.Read(chunk)
		if err == io.EOF {
			time.Sleep(300 * time.Millisecond)
			return
		}
		if err != nil {
			fmt.Printf("transcode input read error %v\n", err)
			return
		}
		// maybe dump to file
		tee(chunk[:size])
		// forward to ffmpeg
		for size > 0 {
			bytesWritten, err := transcoder.Write(chunk[:size])
			if err != nil {
				fmt.Printf("transcode input chunk to ffmpeg error %v\n", err)
				return
			}
			size -= bytesWritten
		}
	}
}

// Compose multipart/mixed HTTP response to deliver several rendition files in same response.
func returnResponseFromFiles(w http.ResponseWriter, req *http.Request, outputs []ffmpeg.TranscodeOptions) error {
	seq := 0 // TODO: this should represent segment sequence number
	boundary := common.RandName()
	accept := req.Header.Get("Accept")
	if accept != "multipart/mixed" {
		w.WriteHeader(http.StatusOK)
		return nil
	}
	contentType := "multipart/mixed; boundary=" + boundary
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(http.StatusOK)
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
	multipart := multipart.NewWriter(w)
	defer multipart.Close()
	for i := 0; i < len(outputs); i++ {
		mediaData, err := os.ReadFile(outputs[i].Oname)
		multipart.SetBoundary(boundary)
		fileName := fmt.Sprintf(`"%s_%d%s"`, outputs[i].Profile.Name, seq, ".ts")
		hdrs := textproto.MIMEHeader{
			"Content-Type":        {"video/mp2t" + "; name=" + fileName},
			"Content-Length":      {strconv.Itoa(len(mediaData))},
			"Content-Disposition": {"attachment; filename=" + fileName},
			"Rendition-Name":      {outputs[i].Profile.Name},
		}
		part, err := multipart.CreatePart(hdrs)
		if err != nil {
			return err
		}
		_, err = io.Copy(part, bytes.NewBuffer(mediaData))
		if err != nil {
			return err
		}
	}
	return nil
}

// Compose multipart/mixed HTTP response to deliver several rendition files in same response.
func returnResponse(w http.ResponseWriter, req *http.Request, outputs []TranscodeOutput) error {
	seq := 0 // TODO: this should represent segment sequence number
	boundary := common.RandName()
	accept := req.Header.Get("Accept")
	if accept != "multipart/mixed" {
		w.WriteHeader(http.StatusOK)
		return nil
	}
	contentType := "multipart/mixed; boundary=" + boundary
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(http.StatusOK)
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
	multipart := multipart.NewWriter(w)
	defer multipart.Close()
	for i := 0; i < len(outputs); i++ {
		multipart.SetBoundary(boundary)
		fileName := fmt.Sprintf(`"%s_%d%s"`, outputs[i].Profile.Name, seq, ".ts")
		hdrs := textproto.MIMEHeader{
			"Content-Type":        {"video/mp2t" + "; name=" + fileName},
			"Content-Length":      {strconv.Itoa(len(outputs[i].Mpegts))},
			"Content-Disposition": {"attachment; filename=" + fileName},
			"Rendition-Name":      {outputs[i].Profile.Name},
		}
		part, err := multipart.CreatePart(hdrs)
		if err != nil {
			return err
		}
		_, err = io.Copy(part, bytes.NewBuffer(outputs[i].Mpegts))
		if err != nil {
			return err
		}
	}
	return nil
}

func NewTranscodingServer(port int) *TranscodingServer {
	server := &TranscodingServer{port: port}
	server.Init()
	return server
}

func main() {
	port := flag.Int("port", 8935, "port to serve HandlePush() on /live/")
	dump := flag.Bool("dump", false, "specify to dump input data to files")
	flag.Parse()
	server := NewTranscodingServer(*port)
	if *dump {
		fmt.Printf("Dumping input data to files ..\n")
		server.DumpInput()
	}

	fmt.Printf("Server started on http://0.0.0.0:%d/live/*\n", *port)

	// Wait for break
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt)
	_ = <-c
	fmt.Printf("Stopping TranscodingServer\n")
	server.Stop()
}
