package server

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/livepeer/lpms/ffmpeg"
)

// Connects to O and takes one transcode job. Transcode job involves multiple segments.
// EndOfInput message signals job end. In turn we send back EndOfInput after all frames.
// On job completion O should reuse same connection for next job.
// On job interruption O should break connection, forcing resource cleanup. When something
//   goes wrong its best to break connection and cleanup all pending frames.
// When connection gets broken StreamingTranscoder will create new one.
type TranscoderConnection struct {
	hwDeviceIndex   int
	orchestratorUrl url.URL
	loginPassword   string
	deleteFromBooks func()

	connection   *websocket.Conn
	httpResponse *http.Response
	jobSpec      *OutputSpec
	outputs      GroupedRenditions
	transcoder   *ffmpeg.PipedTranscoding
	sendMutex    sync.Mutex
}

// We use this as endpoint to signal error and cause teardown.
// TODO: discuss how error-funnel looks like in production/staging.
func (t *TranscoderConnection) operationInterrupted(reason string) BreakOperation {
	fmt.Printf("TranscoderConnection %s breaking connection on error: %s \n", t.orchestratorUrl.String(), reason)
	t.connection.Close()
	t.deleteFromBooks()
	return true
}

// Connect to O. Do login step. Process jobs in a loop.
// Once connected we wait for O to delegate job to us.
// No GPU resources are in use while waiting.
func (t *TranscoderConnection) run() {
	// connect to O
	var err error
	t.connection, t.httpResponse, err = websocket.DefaultDialer.Dial(t.orchestratorUrl.String(), nil)
	if err != nil {
		t.operationInterrupted(err.Error())
		return
	}
	fmt.Printf("T send login on %s\n", t.orchestratorUrl.String())
	// send login
	login := newTranscoderLogin()
	login.Passcode = t.loginPassword
	sendMessage(t.connection, login)
	for {
		breakOperation := t.runTranscodingJob()
		if breakOperation {
			break
		}
	}
}

// Receive OutputSpec as job starts. Then allocate GPU resources.
func (t *TranscoderConnection) runTranscodingJob() BreakOperation {
	// Might be a while waiting on a job.
	switch message := recvMessage(t.connection).(type) {
	case *OutputSpec:
		fmt.Printf("T %s got job %v\n", t.orchestratorUrl.String(), message)
		t.jobSpec = message
	case error:
		return t.operationInterrupted(message.Error())
	default:
		return t.operationInterrupted("expected OutputSpec")
	}

	// Receive frame.
	fmt.Printf("T %s received first frame\n", t.orchestratorUrl.String())
	firstFrame, err := recvFirstFrame(t.connection)
	if err != nil {
		return t.operationInterrupted(err.Error())
	}
	// Prepare for variable number of output renditions
	t.outputs.setup(t.jobSpec.Outputs)

	t.transcoder = &ffmpeg.PipedTranscoding{}
	t.transcoder.SetInput(ffmpeg.TranscodeOptionsIn{Accel: ffmpeg.Nvidia})
	t.transcoder.SetOutputs(t.outputs.getOptions())
	t.outputs.assignPipes(t.transcoder.GetOutputs())

	// Keep spawned goroutines in our books
	spawned := SpawnedGoroutines{}
	// Spawn output goroutines that read ffmpeg pipe and send frames on our websocket.
	// Multiple goroutines should not call send method at the same time. We use t.sendMutex to fix this.
	fmt.Printf("T RENDITIONS: %d \n", len(t.outputs.renditions))
	for i := 0; i < len(t.outputs.renditions); i++ {
		go t.readFromOutput(t.outputs.renditions[i], spawned.Signal())
	}

	// Start transcoder in separate goroutine.
	// When .Transcode() returns ffmpeg is done processing.
	go func(ended chan int) {
		defer close(ended)
		// On ffmpeg completion, ffmpeg leaves pipes open so we close them to release goroutinnes
		//   blocking on pipe read. Pipe closed error is detected in .readFromOutput().
		defer t.transcoder.ClosePipes()

		fmt.Printf("Transcoder started\n")
		_, err := t.transcoder.Transcode()
		if err != nil {
			// We should propagate this error upstream.
			fmt.Printf("> Transcoder ERROR %v\n", err)
		}
		// ffmpeg resources are now released.
		fmt.Printf("Transcoder ended\n")
		fmt.Printf("Transcoder pipes closed\n")
	}(spawned.Signal())

	// Send first frame we got on Handshake()
	if err := t.pushToFfmpeg(firstFrame.Bytes); err != nil {
		return t.operationInterrupted(fmt.Sprintf("transcoder write error: %v", err))
	}

	fmt.Printf("T %s transcoding started\n", t.orchestratorUrl.String())
	// reading loop
	if err := t.readIntoFfmpegLoop(); err != nil {
		return t.operationInterrupted(fmt.Sprintf("transcoder input error: %v", err))
	}
	fmt.Printf("T %s transcoding ending\n", t.orchestratorUrl.String())
	// Wait for goroutines to complete
	spawned.WaitAll()
	fmt.Printf("TranscodingConnection all goroutines exited\n")
	// Send EndOfInput to indicate job well done. O would know to close last bill.
	// We don't use .StreamIndex yet.
	sendMessage(t.connection, newEndOfInput())
	return false
}

// Take frames from O and post to ffmpeg pipe.
// Back-pressure is applied here. O transport follows our processing pace.
func (t *TranscoderConnection) readIntoFfmpegLoop() error {
	// Make sure we close input pipe
	defer t.transcoder.WriteClose()
	for {
		// Recv next frame
		switch message := recvMessage(t.connection).(type) {
		case *InputChunk:
			if err := t.pushToFfmpeg(message.Bytes); err != nil {
				return err
			}
		case *EndOfInput:
			// We send EndOfInput back after all frames
			return nil
		case *VirtualSegmentBoundary:
			// TODO: Flush transcoder and send VirtualSegmentBoundary after all frames
		case error:
			return message
		}
	}
}

// Make sure entire buffer is pushed down the ffmpeg pipe. Handles partial writes.
func (t *TranscoderConnection) pushToFfmpeg(bytes []byte) error {
	for len(bytes) > 0 {
		bytesWritten, err := t.transcoder.Write(bytes)
		if err != nil {
			return err
		}
		// handle partial write
		bytes = bytes[bytesWritten:]
	}
	return nil
}

// One goroutine started for each rendition.
// Read ffmpeg output and send back to O.
func (t *TranscoderConnection) readFromOutput(output *RenditionOutput, ended chan int) {
	buffer := make([]byte, 32768)
	defer close(ended)
	for {
		byteCount, err := output.Pipe.Read(buffer)
		if err == io.EOF {
			// ffmpeg closed the pipe, havent seen this happen
			t.operationInterrupted("pipe EOF")
			break
		}
		if err != nil {
			var closeErr *fs.PathError
			if errors.As(err, &closeErr) {
				// Main goroutine closed the pipe, cleanup follows
				t.operationInterrupted("output ended")
				return
			}
			t.operationInterrupted(fmt.Sprintf("read error %v", err))
			return
		}
		if err := t.send(output, buffer[:byteCount]); err != nil {
			t.operationInterrupted(fmt.Sprintf("send error %v", err))
			return
		}
	}
}

// Ensures output frames are sent in proper order.
// socket.send() is not thread safe. Here we have rare case where multiple
//   goroutines produce output frames that go into single connection. Usually single goroutine
//   is sending and single goroutine is receiving from socket.
// Mutex is used to block concurrent sends, leading into sequence of messages.
func (t *TranscoderConnection) send(output *RenditionOutput, bytes []byte) error {
	frame := newOutputChunk()
	frame.Bytes = bytes
	frame.Index = output.spec.Index
	frame.Name = output.spec.Name
	// TODO: Consider buffered channel for `frame`, sendMessage() may block pipe read - blocking pipe write on ffmpeg side
	t.sendMutex.Lock()
	defer t.sendMutex.Unlock()
	return sendMessage(t.connection, frame)
}

// Envelope for adding pipe to RenditionSpec
type RenditionOutput struct {
	spec RenditionSpec
	Pipe ffmpeg.OutputReader
}

// Ordered renditions matching ffmpeg pipes
type GroupedRenditions struct {
	renditions []*RenditionOutput
}

// Helper to get matching pipes out of the way
func (r *GroupedRenditions) setup(outputs []RenditionSpec) {
	for i := 0; i < len(outputs); i++ {
		rendition := &RenditionOutput{spec: outputs[i]}
		r.renditions = append(r.renditions, rendition)
	}
}

// Helper to get matching pipes out of the way
func (r *GroupedRenditions) assignPipes(pipes []ffmpeg.OutputReader) {
	for i := 0; i < len(r.renditions); i++ {
		r.renditions[i].Pipe = pipes[i]
	}
}

// Helper to get matching pipes out of the way
func (r *GroupedRenditions) getOptions() []ffmpeg.TranscodeOptions {
	options := make([]ffmpeg.TranscodeOptions, 0, len(r.renditions))
	for i := 0; i < len(r.renditions); i++ {
		options = append(options, ffmpeg.TranscodeOptions{
			Profile: r.renditions[i].spec.Profile,
			Accel:   ffmpeg.Nvidia,
		})
	}
	return options
}
