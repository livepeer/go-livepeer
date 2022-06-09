package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/livepeer/lpms/ffmpeg"
)

func verifyMediaMetadata(frame *MpegtsChunk) error {
	_, acodec, vcodec, _, err := ffmpeg.GetCodecInfoBytes(frame.Bytes)
	if err != nil {
		fmt.Printf("Media metadata detect error: %v\n", err)
		return err
	}
	if acodec != frame.Format.AudioCodec {
		fmt.Printf("Metadata mismatch acodec: %s; signaled to be %s\n", acodec, frame.Format.AudioCodec)
	}
	if vcodec != frame.Format.VideoCodec {
		fmt.Printf("Metadata mismatch vcodec: %s; signaled to be %s\n", vcodec, frame.Format.VideoCodec)
	}
	return nil
}

type ResultSendingState struct {
	sendMutex     sync.Mutex
	currentOutput *EncodedOutput
}

type TranscodingConnection struct {
	connection  *websocket.Conn
	outputs     EncodedOutputList
	firstFrame  MpegtsChunk
	transcoder  *ffmpeg.PipedTranscoding
	sending     ResultSendingState
	segmentHash *StreamingHash
}

func (c *TranscodingConnection) Close() {
	c.connection.Close()
}

// Make sure entire buffer is pushed down the ffmpeg pipe
func (c *TranscodingConnection) pushMedia(bytes []byte) error {
	for len(bytes) > 0 {
		bytesWritten, err := c.transcoder.Write(bytes)
		if err != nil {
			return err
		}
		// handle partial write
		bytes = bytes[bytesWritten:]
	}
	return nil
}

//
// We send json message to specify output and follow with binary messages
func (c *TranscodingConnection) sendTranscodedChunk(output *EncodedOutput, bytes []byte) error {
	// Use lock to sequence messages instead of intertwining partial messages over the wire
	c.sending.sendMutex.Lock()
	defer c.sending.sendMutex.Unlock()
	sameOutput := output == c.sending.currentOutput
	if !sameOutput {
		// switch to this output
		encoded, err := json.Marshal(&SelectOutput{
			Index: output.Index,
			Name:  output.Name,
		})
		if err != nil {
			return err
		}
		c.connection.WriteMessage(websocket.TextMessage, encoded)
	}
	// if switch did not happen we send only binary data.
	// This handles small pipe reads efficiently
	c.connection.WriteMessage(websocket.BinaryMessage, bytes)
	return nil
}

func (c *TranscodingConnection) readFromOutput(output *EncodedOutput, ended chan int, myIndex int) {
	buffer := make([]byte, 32768)
	defer close(ended)
	defer fmt.Printf("ffmpeg %d output exited \n", myIndex)
	for {
		byteCount, err := output.Pipe.Read(buffer)
		if err == io.EOF {
			// ffmpeg closed the pipe, havent seen this happen
			fmt.Printf("ffmpeg %d output EOF\n", myIndex)
			break
		}
		// fmt.Printf("< ffmpeg %d output got %d\n", myIndex, byteCount)
		if err != nil {
			var closeErr *fs.PathError
			if errors.As(err, &closeErr) {
				// Main goroutine closed the pipe, cleanup follows
				fmt.Printf("ffmpeg %d output ended\n", myIndex)
				return
			}
			fmt.Printf("ffmpeg %d output read error %v\n", myIndex, err)
			return
		}
		if err := c.sendTranscodedChunk(output, buffer[:byteCount]); err != nil {
			fmt.Printf("output %d send error %v\n", myIndex, err)
			return
		}
	}
}

// Get message received over the wire and take action on each message we support.
func (c *TranscodingConnection) processMessage() (BreakOperation, error) {
	_message := recvMessage(c.connection)
	switch message := _message.(type) {
	case MpegtsChunk:
		// update segment wide hash:
		c.segmentHash.Append(message.Bytes)
		// // fmt.Printf(" < %d hash=%x\n", message.Info.SequenceNumber, c.segmentHash.GetHash()) why this causes panic ?

		// Check frame signature:
		frameSignatureValid := Notary.CheckMediaDataSignature(message.Bytes, message.Signature)
		if !frameSignatureValid {
			fmt.Printf("error: frame signature problem! sn=%d bytes=%d \n", message.Info.SequenceNumber, message.Info.BytesProduced)
			return true, fmt.Errorf("frame signature problem")
		}

		// stream to ffmpeg
		// fmt.Printf("ffmpeg push %d\n", len(message.Bytes))
		if err := c.pushMedia(message.Bytes); err != nil {
			fmt.Printf("transcoder write error: %v\n", err)
			return true, err
		}
		return false, nil
	case error:
		fmt.Printf("protocol error: %v\n", message)
		return true, message
	case EndOfInput:
		return true, nil
	case VirtualSegmentBoundary:
		// check entire segment signature
		calculatedHash := c.segmentHash.GetHash()
		hashProblem := !bytes.Equal(calculatedHash, message.Hash)
		if hashProblem {
			// For some reason upstream calculated wrong hash on same bytes we have!
			fmt.Printf("warn: hash mismatch %x should be %x\n", string(message.Hash), string(calculatedHash))
		}
		segmentSignatureValid := Notary.CheckMediaDataSignature(calculatedHash, message.Signature)
		if !segmentSignatureValid {
			// Take security measures: firewall this ip or similar
			return true, fmt.Errorf("Segment signature problem sn=%d bytes=%d", message.SequenceNumber, message.BytesProduced)
		}
		// Start new hash calculation now
		c.segmentHash = &StreamingHash{}
		c.segmentHash.Init()
		fmt.Printf("# Segment signature valid ! sn=%d bytes=%d\n", message.SequenceNumber, message.BytesProduced)
		return false, nil
	default:
		// `recvMessage()` skips unknown messages. Here we raise error - you forgot to implement a case above.
		return true, fmt.Errorf("Code problem: missing switch case for new message")
	}
}

// Loop over message processing call
func (c *TranscodingConnection) readIntoFfmpegLoop() error {
	// Make sure we close input pipe
	defer c.transcoder.WriteClose()
	// var frame MpegtsChunk = c.firstFrame
	for {
		// Recv next frame
		breakOperation, err := c.processMessage()
		if err != nil {
			return err
		}
		if breakOperation {
			break
		}
	}
	fmt.Printf(" # TranscodingConnection input completed\n")
	return nil
}

//
// Runs recv loop and starts goroutines for each output.
// Here we need one goroutine to read each output pipe from transcoder.
// In O and B nodes we would need single goroutine inspecting messages and just forwarding them.
func (c *TranscodingConnection) RunUntilCompletion() error {
	// Keep spawned goroutines in our books
	spawned := SpawnedGoroutines{}

	// Spawn output goroutines that read ffmpeg pipe and send frames on our websocket.
	// Multiple goroutines should not call send method at the same time. We use c.sendMutex to fix this.
	for i := 0; i < len(c.outputs.Outputs); i++ { // outputs.Outputs - do better here
		go c.readFromOutput(&(c.outputs.Outputs[i]), spawned.Signal(), i)
	}

	// Start transcoder in separate goroutine.
	// When .Transcode() returns ffmpeg is done processing.
	go func(ended chan int) {
		defer close(ended)
		// On ffmpeg completion, ffmpeg leaves pipes open so we close them to release goroutinnes
		//   blocking on pipe read. Pipe closed error is detected in .readFromOutput().
		defer c.transcoder.ClosePipes()

		fmt.Printf("Transcoder started\n")
		_, err := c.transcoder.Transcode()
		if err != nil {
			// We should propagate this error upstream.
			fmt.Printf("> Transcoder ERROR %v\n", err)
		}
		// ffmpeg resources are now released.
		fmt.Printf("Transcoder ended\n")
		fmt.Printf("Transcoder pipes closed\n")
	}(spawned.Signal())

	// Send first frame we got on Handshake()
	if err := c.pushMedia(c.firstFrame.Bytes); err != nil {
		fmt.Printf("transcoder write error: %v\n", err)
		return err
	}

	// reading loop
	if err := c.readIntoFfmpegLoop(); err != nil {
		fmt.Printf("transcoder input error: %v\n", err)
		return err
	}

	// Wait for goroutines to complete
	spawned.WaitAll()
	fmt.Printf("TranscodingConnection all goroutines exited\n")

	return nil
}

// Runs all logic to prepare for media stream, including validation
func (c *TranscodingConnection) Handshake(r *http.Request) error {
	// Prepare for first segment hash
	c.segmentHash = &StreamingHash{}
	c.segmentHash.Init()

	// payment ticket and other info about encoding job would be received now

	// We expect Mist to give us output profile info. Here we just hardcode some profiles.
	c.outputs.Append(ffmpeg.TranscodeOptions{Profile: ffmpeg.VideoProfile{Name: "P240", Bitrate: "3000k", Framerate: 24, AspectRatio: "15:6", Resolution: "569x240"}, Accel: ffmpeg.Nvidia}, 0, "w569")
	c.outputs.Append(ffmpeg.TranscodeOptions{Profile: ffmpeg.VideoProfile{Name: "P240", Bitrate: "2500k", Framerate: 24, AspectRatio: "15:6", Resolution: "474x200"}, Accel: ffmpeg.Nvidia}, 1, "w474")
	c.outputs.Append(ffmpeg.TranscodeOptions{Profile: ffmpeg.VideoProfile{Name: "P240", Bitrate: "1500k", Framerate: 24, AspectRatio: "15:6", Resolution: "356x150"}, Accel: ffmpeg.Nvidia}, 2, "w356")

	// After all setup & administration logic receive first frame of data.
	// We can use first frame to calculate job pricing, placement, etc.
	_message := recvMessage(c.connection)
	switch message := _message.(type) {
	case MpegtsChunk:
		c.firstFrame = message
		c.segmentHash.Append(message.Bytes)
		// We get MediaFormatInfo over wire and if we want to verify it:
		if err := verifyMediaMetadata(&message); err != nil {
			return err
		}

		// todo: check peer enc signature
		if !Notary.CheckMediaDataSignature(message.Bytes, message.Signature) {
			return fmt.Errorf("Bad signature, failing early on frame %d", message.Info.SequenceNumber)
		}

		// Create transcoding "session" coupled to websocket connection.
		// This new kind of session lasts exactly as websocket connection.
		c.transcoder = &(ffmpeg.PipedTranscoding{})
		c.transcoder.SetInput(ffmpeg.TranscodeOptionsIn{Accel: ffmpeg.Nvidia})
		c.transcoder.SetOutputs(c.outputs.GetOptions())
		c.outputs.AssignPipes(c.transcoder.GetOutputs())

		// We are good.
		return nil
	case error:
		fmt.Printf("protocol error: %v", message)
		return message
	default:
		return fmt.Errorf("protocol error: unexpected message %v", message)
	}
}
