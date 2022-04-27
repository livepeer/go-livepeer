//go:build nvidia
// +build nvidia

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/livepeer/go-livepeer/server"
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
	Chunks        []ChunkLatency
	Start         time.Time
	LastChunkTime time.Time
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

func calculateTranscodingLatency(t *testing.T, sending, receiving *TransferLatency) (r *TranscodingLatency) {
	totalRecvBytes, _ := receiving.TotalTimeBytes()
	r = &TranscodingLatency{}

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

type SegmentWebsocketPublisher struct {
	t              *testing.T
	sample         SampleSegment
	hostname       string
	port           int
	inputData      []byte
	url            url.URL
	ws             *websocket.Conn
	httpResponse   *http.Response
	results        map[string]*TransferLatency
	selectedResult *TransferLatency
	// Using channels to signal send&recv goroutine completion
	sendingErrorChannel chan error
	receiveErrorChannel chan error
	// `sendTiming` & `recvTiming` are used by main goroutine and sending/receiving goroutines.
	// channels above ensure usage is sequential so we don't need mutexes
	sendTiming   TransferLatency
	receiveStart time.Time
	// recvTiming TransferLatency
}

func (p *SegmentWebsocketPublisher) init() {
	p.selectedResult = nil
	p.results = make(map[string]*TransferLatency)
	p.url = url.URL{Scheme: "ws", Host: fmt.Sprintf("%s:%d", p.hostname, p.port), Path: fmt.Sprintf("/wslive/%s/%d.ts", manifestID, p.sample.Index)}
	p.sendingErrorChannel, p.receiveErrorChannel = make(chan error), make(chan error)
}
func (p *SegmentWebsocketPublisher) readInput() {
	var err error
	p.inputData, err = ioutil.ReadFile(p.sample.Path)
	require.NoError(p.t, err, "failed to read sample file %s", p.sample.Path)
}
func (p *SegmentWebsocketPublisher) connectWebsocket() {
	var err error
	p.ws, p.httpResponse, err = websocket.DefaultDialer.Dial(p.url.String(), nil)
	require.NoError(p.t, err, "failed ws connect %v", p.url)
	require.NotNil(p.t, p.ws)
}

func (p *SegmentWebsocketPublisher) recvRoutine() {
	require.NotNil(p.t, p.ws)
	// make sure we close socket
	defer p.ws.Close()
	// make sure we release test goroutine
	defer close(p.receiveErrorChannel)
	p.receiveStart = time.Now()
	// Receive messages in a loop.
	// TextMessage is json encoded. Prepares state for next BinaryMessage, for ex. signal end of transmission
	for {
		messageType, byteData, err := p.ws.ReadMessage()
		// Note: connection may break outside lab conditions
		assert.NoError(p.t, err, "ws.ReadMessage() %v", err)
		if messageType == websocket.TextMessage {
			// Text message type preceeds binary chunks and sets context for it
			message := server.PushControlMessage{}
			err := json.Unmarshal(byteData, &message)
			assert.NoError(p.t, err, "failed json decode on websocket.TextMessage %v", err)
			switch message.Stage {
			case server.PushStageResult:
				// select indicated result
				result, ok := p.results[message.Description]
				if !ok {
					result = &TransferLatency{}
					result.Start = p.receiveStart
					result.LastChunkTime = result.Start
					p.results[message.Description] = result
				}
				p.selectedResult = result
			case server.PushStageEof:
				// all done
				return
			case server.PushStageError:
				p.receiveErrorChannel <- fmt.Errorf("error: %s", message.Description)
				return
			}
			continue
		}
		// Is it possible to recv PingMessage,PongMessage,CloseMessage?
		if messageType == websocket.BinaryMessage {
			assert.NotNil(p.t, p.selectedResult, "Protocol error: server must PushStageResult before binary message")
			// we do not process byteData in our test
			now := time.Now()
			duration := now.Sub(p.selectedResult.LastChunkTime)
			p.selectedResult.LastChunkTime = now
			p.selectedResult.Chunks = append(p.selectedResult.Chunks, ChunkLatency{len(byteData), duration})
			continue
		}
	}
}

type pushChunkContext struct {
	originalSize     int
	lastChunkStartTs time.Time
	lastPrintedTime  float64
	sentSize         int
}

func (p *SegmentWebsocketPublisher) pushChunk(c *pushChunkContext) BreakOperation {
	var chunkSize int = min(4096, len(p.inputData))
	// When sending we send binary chunks and signal eof with json message
	err := p.ws.WriteMessage(websocket.BinaryMessage, p.inputData[:chunkSize])
	assert.NoError(p.t, err, "sending error")
	// websocket always sends entire message
	p.inputData = p.inputData[chunkSize:]
	c.sentSize += chunkSize

	now := time.Now()
	duration := now.Sub(c.lastChunkStartTs)
	c.lastChunkStartTs = now
	p.sendTiming.Chunks = append(p.sendTiming.Chunks, ChunkLatency{chunkSize, duration})

	realtimeSpent := toSeconds(time.Now().Sub(p.sendTiming.Start)) // * 10.0
	uploadedTime := toSeconds(p.sample.Duration) * float64(c.sentSize) / float64(c.originalSize)
	if timeToSleep := uploadedTime - realtimeSpent; timeToSleep > 0 {
		amount := fromSeconds(timeToSleep)
		time.Sleep(amount)
	}
	if diff := uploadedTime - c.lastPrintedTime; diff > 1.0 {
		// print progress to show test is not stuck
		c.lastPrintedTime = uploadedTime
		fmt.Printf("> realtime streaming %v sec \n", realtimeSpent)
	}
	return false
}
func (p *SegmentWebsocketPublisher) sendRoutine() {
	defer close(p.sendingErrorChannel)
	require.NotNil(p.t, p.ws)
	p.sendTiming.Start = time.Now()
	c := &pushChunkContext{len(p.inputData), p.sendTiming.Start, 0, 0}
	for {
		if p.pushChunk(c) || len(p.inputData) == 0 {
			fmt.Printf("> all data sent \n")
			p.ws.WriteJSON(server.PushControlMessage{Stage: server.PushStageEof})
			p.sendingErrorChannel <- nil
			break
		}
	}
}
func (p *SegmentWebsocketPublisher) Push() map[string]*TranscodingLatency {
	p.init()
	p.readInput()
	p.connectWebsocket()
	go p.sendRoutine()
	go p.recvRoutine()
	// wait for HTTP transaction complete
	sendingError := <-p.sendingErrorChannel
	receiveError := <-p.receiveErrorChannel
	// validate result
	assert.NoError(p.t, sendingError)
	assert.NoError(p.t, receiveError)
	// return latency measurements
	results := make(map[string]*TranscodingLatency)
	for name, recvTiming := range p.results {
		results[name] = calculateTranscodingLatency(p.t, &p.sendTiming, recvTiming)
	}
	return results
}

func minimalLatency(l *TranscodingLatency, resultName string) (latencyValue float64) {
	for _, checkpoint := range l.Checkpoints {
		fmt.Printf(" [%s] latency %v ms; bytes=%v \n", resultName, checkpoint.Milliseconds, checkpoint.ByteCount)
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
		publisher := SegmentWebsocketPublisher{t: t, sample: sample, hostname: "127.0.0.1", port: 18936}
		latencyResults := publisher.Push()
		worstLatency := float64(0)
		for resultName, latencies := range latencyResults {
			latencyValue := minimalLatency(latencies, resultName)
			if latencyValue > worstLatency {
				worstLatency = latencyValue
			}
		}
		fmt.Printf(" latency %v ms; segment-duration=%v \n", worstLatency, sample.Duration)
	}
}
