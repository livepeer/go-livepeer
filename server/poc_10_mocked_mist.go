package server

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
)

//
// Mockup of Mist ingest logic. Use prepared media files to publish to B. Sends media data in chunks.
// Limitation: chunks are not on frame boundaries. Mist would place entire frames in each chunk.
// On entire segment sent, we send VirtualSegmentBoundary message. MistMockup does this correctly
//   because segments are prepared in separate files and we insert VirtualSegmentBoundary when
//   moving to next sample file.
type MistMockup struct {
	// media data
	inputData      []*InputSpec
	inputDataIndex int
	segmentBytes   []byte

	inputFormat   MediaFormatInfo
	url           url.URL
	socket        WebsocketPublish
	storage       MistRenditionStorage
	recvEndSignal chan int
}

// Beacause of go reasons.
func (m *MistMockup) Init() {
	m.recvEndSignal = make(chan int)
	m.storage.Init()
}

// Handling encoded results. Store on disk for later inspection.
func (m *MistMockup) recvRoutine(t *testing.T) {
	defer close(m.recvEndSignal)
	// We receive until websocket is closed.
	// Transcoder will close connection when job is done.
	// If we wanted to process multiple segments we would use
	//   control message instead of breaking connection.
	for {
		switch message := m.socket.Recv().(type) {
		case *OutputChunk:
			m.storage.Append(t, message)
		case *VirtualSegmentBoundary:
			m.storage.SegmentEnded(t, message)
		case error:
			if closeErr := PeerClosed(message); closeErr != nil {
				fmt.Printf("Connection ended %d %s\n", closeErr.Code, closeErr.Text)
				break
			}
			require.NoError(t, message)
		}
	}
}

// Send chunks/frames to B.
func (m *MistMockup) Run(t *testing.T) {
	m.Init()
	fmt.Printf("Mist dial %s\n", m.url.String())
	err := m.socket.Dial(m.url)
	require.NoError(t, err)

	// Spawn receive routine.
	// Node sends us multiple encoded renditions over same connection.
	// In this Mock we just save received data to its own file.
	go m.recvRoutine(t)

	// Send loop:
	// We create first frame with format given by the test code.
	// Format is not changed at any time later in this test.
	chunk := newInputChunk()
	chunk.Format = m.inputFormat
	// This is always required for first chunk. Only Mist needs to do this.
	chunk.SignalFirstFrameInSegment()

	var newSegmentStarted bool
	chunk.Bytes, newSegmentStarted = m.nextChunk()
	// Here newSegmentStarted is always true, at start of operation.
	// On each next newSegmentStarted we send VirtualSegmentBoundary message.
	segmentsSent := 0
	for i := 0; len(m.inputData) > 0; i++ {
		// fmt.Printf("> Mist sending %d chunk size=%d\n", chunk.Info.SequenceNumber, len(chunk.Bytes))
		err = m.socket.Send(chunk)
		require.NoError(t, err)
		// fake realtime
		time.Sleep(fakeRealtimeInterval * time.Millisecond)
		// prepare next chunk
		chunk.Bytes, newSegmentStarted = m.nextChunk()
		if newSegmentStarted {
			// Next output file shall have new segment number in its name
			// We close output file on VirtualSegmentBoundary from T
			m.storage.segmentIndex += 1
			chunk.SignalFirstFrameInSegment()
			segmentsSent += 1
			if segmentsSent >= len(m.inputData) {
				// We sent each segment once. Complete transcoding job. Continue is also an option.
				break
			}
		}
	}

	m.socket.SendJobEnd(chunk)

	// Wait for .recvRoutine() to complete
	<-m.recvEndSignal
}

// Consume sample files, splitting data into chunks.
func (m *MistMockup) nextChunk() (bytes []byte, newSegment bool) {
	// At start we have read all data into buffer.
	// Here we simulate streaming by splitting into chunks
	if len(m.segmentBytes) == 0 {
		// start new segment
		newSegment = true
		input := m.inputData[m.inputDataIndex]
		m.segmentBytes = input.Data
		m.inputDataIndex = (m.inputDataIndex + 1) % len(m.inputData)
	}

	chunkSize := min(len(m.segmentBytes), fakeRealtimeAmount)
	bytes = m.segmentBytes[:chunkSize]
	m.segmentBytes = m.segmentBytes[chunkSize:]
	return bytes, newSegment
}

// Handles multiple output streams.
// Each chunk is written into corresponding file.
type MistRenditionStorage struct {
	outputs      map[int]*MistFileOutput
	segmentIndex int
}

func (o *MistRenditionStorage) Init() {
	o.outputs = make(map[int]*MistFileOutput)
}

func (o *MistRenditionStorage) SegmentEnded(t *testing.T, boundary *VirtualSegmentBoundary) {
	// Close file and remove the stream.
	// First frame of same rendition will open new file.
	outputFile, exist := o.outputs[boundary.StreamIndex]
	require.True(t, exist, "VirtualSegmentBoundary on invalid index %d", boundary.StreamIndex)
	outputFile.File.Close()
	delete(o.outputs, boundary.StreamIndex)
}

// Sometimes Mist doesn't know rendition spec, if B determined it via webhook.
// Encoded frame contains index and name of the output.
func (o *MistRenditionStorage) Append(t *testing.T, frame *OutputChunk) {
	// fmt.Printf("< Mist storing %d bytes\n", len(bytes))
	// Just write to open file. Allow for short writes.
	outputFile, exist := o.outputs[frame.Index]
	if !exist {
		// create output
		outputFile = &MistFileOutput{}
		// Init - opens output file.
		require.NoError(t, outputFile.Init(o.segmentIndex, frame.Name))
		// Store back into our books.
		o.outputs[frame.Index] = outputFile
	}
	bytes := frame.Bytes[:]
	for len(bytes) > 0 {
		written, err := outputFile.File.Write(bytes)
		require.NoError(t, err)
		bytes = bytes[written:]
	}
}

// We place resulting stream into file.
type MistFileOutput struct {
	File *os.File
}

// Beacause of go reasons.
func (f *MistFileOutput) Init(segmentNumber int, name string) error {
	// Open destination file
	path := fmt.Sprintf("./encoded-%d-%s.ts", segmentNumber, name)
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	f.File = file
	fmt.Printf("Mist created output file %s\n", path)
	return nil
}

type InputSpec struct {
	Name     string
	Data     []byte
	Duration int
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

// Because of go reasons
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Based on sample file:
const fakeRealtimeAmount = 6016 * 8 * 2
const fakeRealtimeInterval = 83
