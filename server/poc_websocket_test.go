//go:build nvidia
// +build nvidia

package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"path"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
)

// Because of go reasons
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Is error websocket.CloseError. If not returns nil.
func PeerClosed(err error) *websocket.CloseError {
	var closeErr *websocket.CloseError
	if errors.As(err, &closeErr) {
		fmt.Printf("Connection ended %d %s\n", closeErr.Code, closeErr.Text)
		return closeErr
	}
	return nil
}

// We place resulting stream into file.
type MistFileOutput struct {
	File *os.File
}

// Beacause of go reasons.
func (f *MistFileOutput) Init(name string) error {
	// Open destination file
	path := fmt.Sprintf("./encoded-%s.ts", name)
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	f.File = file
	fmt.Printf("Mist created output file %s\n", path)
	return nil
}

// Handles multiple output streams.
// Each chunk is written into corresponding file.
type MistOutputMultiplexer struct {
	currentOutput *SelectOutput
	currentFile   *MistFileOutput
	outputs       map[int]*MistFileOutput
}

// Beacause of go reasons.
func (o *MistOutputMultiplexer) Init() {
	o.outputs = make(map[int]*MistFileOutput)
}

func (o *MistOutputMultiplexer) Append(t *testing.T, bytes []byte) {
	require.NotNil(t, o.currentFile)
	fmt.Printf("< Mist storing %d bytes\n", len(bytes))
	// Just write to open file. Allow for short writes.
	for len(bytes) > 0 {
		written, err := o.currentFile.File.Write(bytes)
		require.NoError(t, err)
		bytes = bytes[written:]
	}
}

func (o *MistOutputMultiplexer) Select(t *testing.T, selected *SelectOutput) {
	// If this is same output as current selected one, do nothing.
	// Same check happens on Node side so this should not happen in practice.
	if o.currentOutput != selected {
		fmt.Printf("< Mist receiving %s %d\n", selected.Name, selected.Index)
		outputFile, exist := o.outputs[selected.Index]
		if !exist {
			// Node is signaling new output. We see it for the first time. Create new output.
			outputFile = &MistFileOutput{}
			// Init - opens output file.
			require.NoError(t, outputFile.Init(selected.Name))
			// Store back into our books.
			o.outputs[selected.Index] = outputFile
		}
		// Switch to selected output
		o.currentOutput = selected
		o.currentFile = outputFile
	}
}

//
// Mockup of Mist ingest logic.
// POC of Mist networking logic.
type MistMockup struct {
	inputData     []byte
	inputFormat   MediaFormatInfo
	url           url.URL
	socket        WebsocketPublish
	outputMux     MistOutputMultiplexer
	recvEndSignal chan int
}

// Beacause of go reasons.
func (m *MistMockup) Init() {
	m.recvEndSignal = make(chan int)
	m.outputMux.Init()
}

func (m *MistMockup) recvRoutine(t *testing.T) {
	defer close(m.recvEndSignal)
	// We receive until websocket is closed.
	// Transcoder will close connection when job is done.
	// If we wanted to process multiple segments we would use
	//   control message instead of breaking connection.
	for {
		messageType, bytes, err := m.socket.Recv()
		// error handling
		if err != nil {
			if closeErr := PeerClosed(err); closeErr != nil {
				fmt.Printf("Connection ended %d %s\n", closeErr.Code, closeErr.Text)
				break
			}
			require.NoError(t, err)
		}
		// Handle control messages and binary messages:
		switch messageType {
		case websocket.TextMessage:
			var selected SelectOutput
			require.NoError(t, json.Unmarshal(bytes, &selected))
			// Control message specifies which output data will follow.
			m.outputMux.Select(t, &selected)
		case websocket.BinaryMessage:
			// Binary message does not require any decoding.
			// Output is already selected, store it now.
			m.outputMux.Append(t, bytes)
		}
	}
}

// Based on sample file:
const fakeRealtimeAmount = 6219 * 8
const fakeRealtimeInterval = 83

func (m *MistMockup) nextChunk() []byte {
	// At start we have read all data into buffer.
	// Here we simulate streaming by splitting into chunks
	chunkSize := min(len(m.inputData), fakeRealtimeAmount)
	bytes := m.inputData[:chunkSize]
	m.inputData = m.inputData[chunkSize:]
	return bytes
}

func (m *MistMockup) Run(t *testing.T) {
	m.Init()
	fmt.Printf("Mist dial %s\n", m.url.String())
	err := m.socket.Dial(m.url)
	require.NoError(t, err)

	// Spawn receive routine.
	// Node sends us multiple encoded streams over same connection.
	// In this Mock we just save received data to its own file.
	go m.recvRoutine(t)

	// Send loop:
	//
	// We create first frame with format given by the test code.
	// Format is not changed at any time later in this test.
	chunk := MpegtsChunk{Format: m.inputFormat}
	chunk.Info.FirstFrameInSegment = true
	chunk.Bytes = m.nextChunk()
	err = m.socket.Send(&chunk)
	require.NoError(t, err)
	fmt.Printf("> Mist sent first chunk \n")

	chunk.Info.FirstFrameInSegment = false
	for len(m.inputData) > 0 {
		time.Sleep(fakeRealtimeInterval * time.Millisecond) // fake realtime
		// Real Mist server would update all metadata.
		// For transcoder operation only media bytes are *required*. For our pixel & price
		//   calculation we depend on metadata. Transcoder node would double check using ffmpeg.
		chunk.Info.SequenceNumber += 1
		chunk.Info.BytesProduced += len(chunk.Bytes)
		chunk.Bytes = m.nextChunk()
		fmt.Printf("> Mist sent %d chunk size=%d\n", chunk.Info.SequenceNumber, len(chunk.Bytes))

		err = m.socket.Send(&chunk)
		require.NoError(t, err)
	}

	m.socket.SendJobEnd()

	// Wait for .recvRoutine() to complete
	<-m.recvEndSignal
}

func TestWebsocket_Publish(t *testing.T) {
	outputFiles := []string{"./encoded-w356.ts", "./encoded-w474.ts", "./encoded-w569.ts"}
	// remove outputs from previous run
	for _, name := range outputFiles {
		os.Remove(name)
	}

	// prepare input
	wd, err := os.Getwd()
	require.NoError(t, err)
	inputFileName := path.Join(wd, "..", "samples", "sample_0_409_17041.ts")
	data, err := ioutil.ReadFile(inputFileName)
	require.NoError(t, err)

	// run poc server
	mistPort := 9090
	go ServeHttp(mistPort)

	time.Sleep(100 * time.Millisecond)

	mist := MistMockup{
		// Live Mist would receive stream, here we read data from file
		inputData: data,
		// Live Mist would extract metadata, here we hardcoded it
		inputFormat: MediaFormatInfo{
			VideoCodec: "h264",
			AudioCodec: "",
			FpKs:       24000,
			Width:      1024,
			Height:     432,
		},
		// Pass Url to our websocket transcoder node
		url: url.URL{Scheme: "ws", Host: fmt.Sprintf("127.0.0.1:%d", mistPort), Path: "/wslive/testing/1.ts"},
	}
	mist.Run(t)

	// Output files should be produced
	for _, name := range outputFiles {
		outInfo, err := os.Stat(name)
		if os.IsNotExist(err) {
			t.Error(err)
		}
		require.NotEqual(t, outInfo.Size(), 0, "missing output %s", name)
	}
}
