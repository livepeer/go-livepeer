//go:build nvidia
// +build nvidia

package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func toSeconds(duration time.Duration) float64 {
	return float64(duration) / 1000000000.0
}
func fromSeconds(seconds float64) time.Duration {
	return time.Duration(seconds * 1000000000.0)
}

type SampleSegment struct {
	Index      int
	FrameCount int
	Duration   time.Duration
	Path       string
}

var sampleSegments = []SampleSegment{
	{0, 360, 15000 * time.Millisecond, "./samples/16/sample_1_360_15000.ts"},
	{1, 69, 2875 * time.Millisecond, "./samples/2/sample_0_69_2875.ts"},
	{2, 53, 2208 * time.Millisecond, "./samples/2/sample_1_53_2208.ts"},
	{3, 30, 1250 * time.Millisecond, "./samples/2/sample_2_30_1250.ts"},
	{4, 409, 17041 * time.Millisecond, "./samples/16/sample_0_409_17041.ts"},
}

var manifestID = "13f63dab-5984-4580-b50d-7ab6753dc7cb"

type ChunkLatency struct {
	ByteCount int
	Duration  time.Duration
}

type TransferLatency struct {
	Chunks []ChunkLatency
	Start  time.Time
}

func (l *TransferLatency) TotalTimeBytes() (bytes int, duration time.Duration) {
	for _, chunk := range l.Chunks {
		bytes += chunk.ByteCount
		duration += chunk.Duration
	}
	return
}

func (l *TransferLatency) lerpToReceiveTimeline(t *testing.T, recvFraction float64, recvTimestamp time.Time) (milliseconds float64) {
	totalSentBytes, _ := l.TotalTimeBytes()
	sentTimestamp := l.Start
	sentBytes := int(0)
	for _, chunk := range l.Chunks {
		nextBytes := sentBytes + chunk.ByteCount
		sentFraction := float64(nextBytes) / float64(totalSentBytes)
		if sentFraction >= recvFraction {
			targetBytes := recvFraction * float64(totalSentBytes)
			timeIncrement := (float64(chunk.Duration) / float64(time.Millisecond)) * ((targetBytes - float64(sentBytes)) / float64(chunk.ByteCount))
			targetTimestamp := sentTimestamp.Add(time.Duration(timeIncrement * float64(time.Millisecond)))
			return float64(recvTimestamp.Sub(targetTimestamp)) / float64(time.Millisecond)
		}
		sentTimestamp = sentTimestamp.Add(chunk.Duration)
		sentBytes = nextBytes
	}
	assert.True(t, false, "Interpolate error")
	return 0
}

type TranscodingLatencyCheckpoint struct {
	Milliseconds float64
	ByteCount    int
}

type TranscodingLatency struct {
	Checkpoints []TranscodingLatencyCheckpoint
}

func calculateTranscodingLatency(t *testing.T, sending, receiving TransferLatency) (r TranscodingLatency) {
	totalRecvBytes, _ := receiving.TotalTimeBytes()

	recvTimestamp := receiving.Start
	recvBytes := int(0)

	for _, chunk := range receiving.Chunks {
		recvTimestamp = recvTimestamp.Add(chunk.Duration)
		recvBytes += chunk.ByteCount
		recvFraction := float64(recvBytes) / float64(totalRecvBytes)
		// `latency` should be calculated on frames. We base our calculation on data size.
		// This is not precise as some frames are larger than others.
		// Should be good for our test here.
		latency := sending.lerpToReceiveTimeline(t, recvFraction, recvTimestamp)
		r.Checkpoints = append(r.Checkpoints, TranscodingLatencyCheckpoint{
			Milliseconds: latency,
			ByteCount:    chunk.ByteCount,
		})
	}
	return
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

type BreakOperation bool

type SegmentHTTPPublisher struct {
	t            *testing.T
	sample       SampleSegment
	hostname     string
	port         int
	inputData    []byte
	url          string
	body         *io.PipeWriter
	httpResponse *http.Response
	// Using channels to signal send&recv goroutine completion
	sendingErrorChannel chan error
	receiveErrorChannel chan error
	// `sendTiming` & `recvTiming` are used by main goroutine and sending/receiving goroutines.
	// channels above ensure usage is sequential so we don't need mutexes
	sendTiming TransferLatency
	recvTiming TransferLatency
}

func (p *SegmentHTTPPublisher) init() {
	p.url = fmt.Sprintf("http://%s:%d/live/%s/%d.ts", p.hostname, p.port, manifestID, p.sample.Index)
	p.sendingErrorChannel, p.receiveErrorChannel = make(chan error), make(chan error)
}
func (p *SegmentHTTPPublisher) readInput() {
	var err error
	p.inputData, err = ioutil.ReadFile(p.sample.Path)
	require.NoError(p.t, err, "failed to read sample file %s", p.sample.Path)
}
func (p *SegmentHTTPPublisher) sendHttpRequest() {
	client := &http.Client{
		Timeout: p.sample.Duration * 2 * time.Second,
		Transport: &http.Transport{
			DisableKeepAlives:  true,
			DisableCompression: true,
		},
	}
	var readEndOfPipe *io.PipeReader
	readEndOfPipe, p.body = io.Pipe()
	request, err := http.NewRequest("POST", p.url, readEndOfPipe)
	assert.NoError(p.t, err, "failed http.NewRequest %s", p.url)
	request.Close = true
	request.ContentLength = -1
	request.TransferEncoding = append(request.TransferEncoding, "chunked")
	request.Header.Add("Content-Type", "video/mp2t")
	request.Header.Add("Accept", "multipart/mixed")

	p.httpResponse, err = client.Do(request)
	assert.NoError(p.t, err, "failed client.Do %s", p.url)
}

func (p *SegmentHTTPPublisher) recvRoutine() {
	defer p.httpResponse.Body.Close()
	p.recvTiming.Start = time.Now()
	lastChunkStartTs := p.recvTiming.Start
	chunk := make([]byte, 16384)
	for {
		byteCount, err := p.httpResponse.Body.Read(chunk)
		if err != nil || byteCount == 0 {
			assert.Greater(p.t, len(p.recvTiming.Chunks), 0, "transcoding refused")
			fmt.Printf("> recv ended with %v chunks=%d\n", err, len(p.recvTiming.Chunks))
			if err == io.EOF {
				p.receiveErrorChannel <- nil
			} else {
				p.receiveErrorChannel <- err
			}
			return
		}
		now := time.Now()
		duration := now.Sub(lastChunkStartTs)
		lastChunkStartTs = now
		p.recvTiming.Chunks = append(p.recvTiming.Chunks, ChunkLatency{byteCount, duration})
	}
}

type pushChunkContext struct {
	originalSize     int
	lastChunkStartTs time.Time
	lastPrintedTime  float64
	sentSize         int
}

func (p *SegmentHTTPPublisher) pushChunk(c *pushChunkContext) BreakOperation {
	var chunkSize int = min(4096, len(p.inputData))
	var toSend int = chunkSize
	for {
		if toSend <= 0 {
			break
		}
		bytesSent, err := p.body.Write(p.inputData[:toSend])
		p.inputData = p.inputData[bytesSent:]
		toSend -= bytesSent
		c.sentSize += bytesSent
		if err != nil {
			fmt.Printf("> sending error=%v \n", err)
			return true
		}
	}

	now := time.Now()
	duration := now.Sub(c.lastChunkStartTs)
	c.lastChunkStartTs = now
	p.sendTiming.Chunks = append(p.sendTiming.Chunks, ChunkLatency{chunkSize, duration})

	realtimeSpent := toSeconds(time.Now().Sub(p.sendTiming.Start)) // * 10.0
	uploadedTime := toSeconds(p.sample.Duration) * float64(c.sentSize) / float64(c.originalSize)
	if timeToSleep := uploadedTime - realtimeSpent; timeToSleep > 0 {
		amount := fromSeconds(timeToSleep)
		// fmt.Printf("> sleeping=%v %v realtime=%v uploadtime=%v\n", timeToSleep, amount, realtimeSpent, uploadedTime)
		time.Sleep(amount)
	}
	// else {
	// 	fmt.Printf("> NOT sleeping realtime=%v uploadtime=%v \n", realtimeSpent, uploadedTime)
	// }
	if diff := uploadedTime - c.lastPrintedTime; diff > 1.0 {
		// print progress to show test is not stuck
		c.lastPrintedTime = uploadedTime
		fmt.Printf("> realtime streaming %v sec \n", realtimeSpent)
	}
	return false
}
func (p *SegmentHTTPPublisher) sendRoutine() {
	defer p.body.Close()
	p.sendTiming.Start = time.Now()
	c := &pushChunkContext{len(p.inputData), p.sendTiming.Start, 0, 0}
	for {
		if p.pushChunk(c) || len(p.inputData) == 0 {
			fmt.Printf("> all data sent \n")
			p.sendingErrorChannel <- nil
			break
		}
	}
}
func (p *SegmentHTTPPublisher) Push() TranscodingLatency {
	p.init()
	p.readInput()
	p.sendHttpRequest()
	go p.sendRoutine()
	go p.recvRoutine()
	// wait for HTTP transaction complete
	sendingError := <-p.sendingErrorChannel
	receiveError := <-p.receiveErrorChannel
	// validate result
	assert.NoError(p.t, sendingError)
	assert.NoError(p.t, receiveError)
	// return latency measurements
	return calculateTranscodingLatency(p.t, p.sendTiming, p.recvTiming)
}

func minimalLatency(l *TranscodingLatency) (latencyValue float64) {
	for _, checkpoint := range l.Checkpoints {
		// // fmt.Printf(" latency %v ms; bytes=%v \n", checkpoint.Milliseconds, checkpoint.ByteCount)
		if latencyValue < checkpoint.Milliseconds {
			latencyValue = checkpoint.Milliseconds
		}
	}
	return
}

func TestIntegration_TranscodingLatency(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	flag.Set("logtostderr", "true")
	flag.Set("v", "6")

	orchestratorArguments := NewSimulatedProgramArguments()
	transcoderArguments := NewSimulatedProgramArguments()
	broadcasterArguments := NewSimulatedProgramArguments()
	// Setup Orchestrator
	orchestratorArguments.Set("v", "6")
	orchestratorArguments.Set("datadir", "./_test_data_o")
	orchestratorArguments.Set("orchestrator", "true")
	orchestratorArguments.Set("orchSecret", "foo")
	orchestratorArguments.Set("serviceAddr", "0.0.0.0:18935")
	// Setup transcoder
	transcoderArguments.Set("v", "6")
	transcoderArguments.Set("orchAddr", "127.0.0.1:18935")
	transcoderArguments.Set("datadir", "./_test_data_t")
	transcoderArguments.Set("transcoder", "true")
	transcoderArguments.Set("orchSecret", "foo")
	transcoderArguments.Set("cliAddr", ":17936")
	transcoderArguments.Set("nvidia", "0")
	// Setup Broadcaster
	broadcasterArguments.Set("v", "6")
	broadcasterArguments.Set("orchAddr", "127.0.0.1:18935")
	broadcasterArguments.Set("network", "offchain")
	broadcasterArguments.Set("datadir", "./_test_data_b")
	broadcasterArguments.Set("broadcaster", "true")
	broadcasterArguments.Set("cliAddr", ":17937")
	broadcasterArguments.Set("httpIngest", "true")
	broadcasterArguments.Set("httpAddr", "0.0.0.0:18936")

	// TODO: use signal on all 3 nodes ready instead of using sleep

	// Start all 3 nodes
	go func() {
		InvokeLivepeerMain(orchestratorArguments, ctx, cancel)
	}()
	time.Sleep(500 * time.Millisecond)
	go func() {
		InvokeLivepeerMain(transcoderArguments, ctx, cancel)
	}()
	time.Sleep(2500 * time.Millisecond)
	go func() {
		InvokeLivepeerMain(broadcasterArguments, ctx, cancel)
	}()
	time.Sleep(500 * time.Millisecond)

	for _, sample := range sampleSegments {
		publisher := SegmentHTTPPublisher{t: t, sample: sample, hostname: "127.0.0.1", port: 18936}
		latency := publisher.Push()
		latencyValue := minimalLatency(&latency)
		fmt.Printf(" latency %v ms; segment-duration=%v \n", latencyValue, sample.Duration)
	}
}
