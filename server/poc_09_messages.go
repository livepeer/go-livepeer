package server

import (
	"encoding/json"
	"fmt"

	"github.com/gorilla/websocket"
	"github.com/livepeer/lpms/ffmpeg"
)

// All messages are used as pointers.
// There are helper constructor functions newEndOfInput(), newTranscoderLogin(), etc.
// sendMessage() and recvMessage() deal with all messages.
// When introducing a new message add it here.

// Common part for all messages. Should be hidden from upper layers.
type MessageHeader struct {
	Id string
}

// Send after last frame of segment, before first frame of next segment.
type VirtualSegmentBoundary struct {
	MessageHeader
	// Signature      []byte
	// Hash           []byte
	StreamIndex         int
	FrameSequenceNumber int
	BytesProduced       int
}

func newVirtualSegmentBoundary() *VirtualSegmentBoundary {
	return &VirtualSegmentBoundary{MessageHeader: MessageHeader{"VirtualSegmentBoundary"}}
}

// Send when Mist stream ends.
// Sent when after all frames are flushed from transcoder.
type EndOfInput struct {
	MessageHeader
	StreamIndex int
}

func newEndOfInput() *EndOfInput { return &EndOfInput{MessageHeader: MessageHeader{"EndOfInput"}} }

// TranscoderConnection sends to O on connect.
type TranscoderLogin struct {
	MessageHeader
	Passcode string
}

func newTranscoderLogin() *TranscoderLogin {
	return &TranscoderLogin{MessageHeader: MessageHeader{"TranscoderLogin"}}
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

// Info accompanying each input frame
type SegmentInfo struct {
	StartTimestampMs    int  // Timestamp of single frame in InputChunk.
	DurationMs          int  // This is duration of current virtual-segment. Next segment boundary(.FirstFrameInSegment==true) shall reset this.
	SequenceNumber      int  // incremented for each new InputChunk sent.
	BytesProduced       int  // Total sum of bytes in current virtual-segment.
	FirstFrameInSegment bool // true when this InputChunk starts new virtual segment boundary. `VirtualSegmentBoundary` message should precede this message unless this is first frame of transcoding job.
}

// Sent as input to transcoding, json encoded
type InputChunk struct {
	MessageHeader
	Bytes     []byte `json:"-"` // this field is not marshaled as json, its sent as ws binary message.
	Info      SegmentInfo
	Format    MediaFormatInfo
	Signature []byte // will be encoded as a Base64 string
}

func newInputChunk() *InputChunk { return &InputChunk{MessageHeader: MessageHeader{"InputChunk"}} }

func (c *InputChunk) SignalFirstFrameInSegment() {
	c.Info.FirstFrameInSegment = true
}

func (c *InputChunk) Sign(wallet *TestWallet) (err error) {
	if c.Signature, err = wallet.SignMediaData(c.Bytes); err != nil {
		return err
	}
	return nil
}

type RenditionSpec struct {
	Index   int
	Name    string
	Profile ffmpeg.VideoProfile
}

// Sent at start of encoding job.
// Encoder creates outputs based on this info.
type OutputSpec struct {
	MessageHeader
	Outputs []RenditionSpec
}

func newOutputSpec() *OutputSpec { return &OutputSpec{MessageHeader: MessageHeader{"OutputSpec"}} }

// Sent back as result, json encoded
type OutputChunk struct {
	MessageHeader
	Bytes     []byte `json:"-"` // this field is not marshaled as json, its sent as ws binary message.
	Index     int
	Name      string
	Signature []byte // will be encoded as a Base64 string
}

func newOutputChunk() *OutputChunk { return &OutputChunk{MessageHeader: MessageHeader{"OutputChunk"}} }

// Exploring ...
type FatalError struct {
	MessageHeader
	Reason string
}

func newFatalError() *FatalError { return &FatalError{MessageHeader: MessageHeader{"FatalError"}} }

// Only func for sending message over the wire
func sendMessage(connection *websocket.Conn, _message interface{}) error {
	switch message := _message.(type) {
	case *OutputChunk:
		if err := connection.WriteJSON(message); err != nil {
			return err
		}
		if err := connection.WriteMessage(websocket.BinaryMessage, message.Bytes); err != nil {
			return err
		}
	case *InputChunk:
		if err := connection.WriteJSON(message); err != nil {
			return err
		}
		if err := connection.WriteMessage(websocket.BinaryMessage, message.Bytes); err != nil {
			return err
		}
	case *EndOfInput:
		if err := connection.WriteJSON(message); err != nil {
			return err
		}
	case *VirtualSegmentBoundary:
		if err := connection.WriteJSON(message); err != nil {
			return err
		}
	case *FatalError:
		if err := connection.WriteJSON(message); err != nil {
			return err
		}
	case *OutputSpec:
		if err := connection.WriteJSON(message); err != nil {
			return err
		}
	case *TranscoderLogin:
		if err := connection.WriteJSON(message); err != nil {
			return err
		}
	default:
		return fmt.Errorf("sendMessage NTI %T", _message)
	}
	return nil
}

// Only function for receiving message over the wire.
// Returns pointer to message struct or error. Use switch statement on returned type. Skips all unknown messages.
func recvMessage(connection *websocket.Conn) interface{} {
	for {
		mt, bytes, err := connection.ReadMessage()
		if err != nil {
			return err
		}
		if mt != websocket.TextMessage {
			fmt.Printf("recvMessage() skipping unexpected binary message\n")
			continue
		}
		var header MessageHeader
		err = json.Unmarshal(bytes, &header)
		if err != nil {
			return err
		}

		// helper for json decode
		decodeControlMsg := func(msgPointer interface{}) interface{} {
			var err = json.Unmarshal(bytes, msgPointer)
			if err != nil {
				return err
			}
			return msgPointer
		}

		// Use a switch to unmarshal different messages:
		switch header.Id {
		case "OutputChunk":
			// Common data path:
			message := newOutputChunk()
			err = json.Unmarshal(bytes, message)
			if err != nil {
				return err
			}
			mt, bytes, err = connection.ReadMessage()
			if err != nil {
				return err
			}
			if mt != websocket.BinaryMessage {
				return fmt.Errorf("recvMessage OutputChunk: expected binary message")
			}
			message.Bytes = bytes
			return message
		case "InputChunk":
			// Common data path:
			message := newInputChunk()
			err = json.Unmarshal(bytes, message)
			if err != nil {
				return err
			}
			mt, bytes, err = connection.ReadMessage()
			if err != nil {
				return err
			}
			if mt != websocket.BinaryMessage {
				return fmt.Errorf("recvMessage InputChunk: expected binary message")
			}
			message.Bytes = bytes
			return message
		case "EndOfInput":
			return decodeControlMsg(newEndOfInput())
		case "VirtualSegmentBoundary":
			return decodeControlMsg(newVirtualSegmentBoundary())
		case "OutputSpec":
			return decodeControlMsg(newOutputSpec())
		case "FatalError":
			return decodeControlMsg(newFatalError())
		case "TranscoderLogin":
			return decodeControlMsg(newTranscoderLogin())
		default:
			// may be message from future version of protocol, skip it
			fmt.Printf("recvMessage() skipping unknown control message\n")
			continue
		}
	}
}

// Helper function
func recvFirstFrame(connection *websocket.Conn) (*InputChunk, error) {
	switch message := recvMessage(connection).(type) {
	case *InputChunk:
		return message, nil
	case error:
		err := fmt.Errorf("protocol error: %v", message)
		return newInputChunk(), err
	default:
		return newInputChunk(), fmt.Errorf("protocol error: unexpected message %v", message)
	}
}
