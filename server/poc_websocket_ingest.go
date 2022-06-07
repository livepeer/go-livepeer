package server

import (
	"encoding/json"
	"fmt"

	"github.com/gorilla/websocket"
	"github.com/livepeer/lpms/ffmpeg"
)

// Sent back as result, json encoded
// Specifies which output's binary data is to follow on the wire.
type SelectOutput struct {
	Index int
	Name  string
}

// Describes ffmpeg output
type EncodedOutput struct {
	TranscodeOptions ffmpeg.TranscodeOptions
	Pipe             ffmpeg.OutputReader
	// Here are fields describing transcoded output:
	Index int
	Name  string
}

// Groups all outputs of single transcoding operation/connection
type EncodedOutputList struct {
	Outputs []EncodedOutput
}

func (l *EncodedOutputList) Append(options ffmpeg.TranscodeOptions, index int, name string) {
	l.Outputs = append(l.Outputs, EncodedOutput{TranscodeOptions: options, Index: index, Name: name})
}

func (l *EncodedOutputList) AssignPipes(pipes []ffmpeg.OutputReader) {
	for i := 0; i < len(l.Outputs); i++ {
		l.Outputs[i].Pipe = pipes[i]
	}
}

func (l *EncodedOutputList) GetOptions() []ffmpeg.TranscodeOptions {
	options := make([]ffmpeg.TranscodeOptions, 0, len(l.Outputs))
	for i := 0; i < len(l.Outputs); i++ {
		options = append(options, l.Outputs[i].TranscodeOptions)
	}
	return options
}

// Media metadata we are interested in
type MediaFormatInfo struct {
	VideoCodec          string // We case about one video track
	AudioCodec          string // We case about one audio track
	FpKs                int    // fps x 1000
	Width               int
	Height              int
	ColorDepthBitMinus8 int
	ChromaSubsampling   int
}

// Info accompanying each frame
type SegmentInfo struct {
	StartTimestampMs    int  // Because we can support multiple frames. Mist does single frame for now.
	DurationMs          int  // This is duration of current virtual-segment. Next segment boundary(.FirstFrameInSegment==true) shall reset this.
	SequenceNumber      int  // incremented for each new MpegtsChunk sent.
	BytesProduced       int  // Total sum of bytes in current virtual-segment.
	FirstFrameInSegment bool // true when this MpegtsChunk starts new virtual segment boundary.
}

//
// Sent as input to transcoding, json encoded
type MpegtsChunk struct {
	Bytes  []byte `json:"-"` // this field is not marshaled as json, its sent as ws binary message.
	Info   SegmentInfo
	Format MediaFormatInfo
}

// Common part for all messages.
// Use it to detect which message is arriving.
type MessageHeader struct {
	Id string
}

var EndOfInputError error = fmt.Errorf("End of input")

//
// Expects JSON encoded info then binary data.
// Reacts to `EndOfInput` message and skips all unknown messages.
func recvMpegtsChunk(chunk *MpegtsChunk, connection *websocket.Conn) error {
	for {
		mt, bytes, err := connection.ReadMessage()
		if err != nil {
			return err
		}
		if mt != websocket.TextMessage {
			return fmt.Errorf("recvMpegtsChunk: expected text message")
		}
		var header MessageHeader
		err = json.Unmarshal(bytes, &header)
		if err != nil {
			return err
		}

		// check which message we got
		if header.Id != "MpegtsChunk" {
			if header.Id == "EndOfInput" {
				return EndOfInputError
			}
			fmt.Printf("skipping unknown message %s\n", header.Id)
			// may be message from future version of protocol, skip it
			continue
		}

		err = json.Unmarshal(bytes, chunk)
		if err != nil {
			return err
		}
		mt, bytes, err = connection.ReadMessage()
		if err != nil {
			return err
		}
		if mt != websocket.BinaryMessage {
			return fmt.Errorf("recvMpegtsChunk: expected binary message")
		}
		chunk.Bytes = bytes
		return nil
	}
}
