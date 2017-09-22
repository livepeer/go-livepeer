package stream

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/nareix/joy4/av"
)

//Testing WriteRTMP errors
var ErrPacketRead = errors.New("packet read error")
var ErrStreams = errors.New("streams error")

type BadStreamsDemuxer struct{}

func (d BadStreamsDemuxer) Close() error                     { return nil }
func (d BadStreamsDemuxer) Streams() ([]av.CodecData, error) { return nil, ErrStreams }
func (d BadStreamsDemuxer) ReadPacket() (av.Packet, error)   { return av.Packet{Data: []byte{0, 0}}, nil }

type BadPacketsDemuxer struct{}

func (d BadPacketsDemuxer) Close() error                     { return nil }
func (d BadPacketsDemuxer) Streams() ([]av.CodecData, error) { return nil, nil }
func (d BadPacketsDemuxer) ReadPacket() (av.Packet, error) {
	return av.Packet{Data: []byte{0, 0}}, ErrPacketRead
}

type NoEOFDemuxer struct {
	c *Counter
}

type Counter struct {
	Count int
}

func (d NoEOFDemuxer) Close() error                     { return nil }
func (d NoEOFDemuxer) Streams() ([]av.CodecData, error) { return nil, nil }
func (d NoEOFDemuxer) ReadPacket() (av.Packet, error) {
	if d.c.Count == 10 {
		return av.Packet{}, nil
	}

	d.c.Count = d.c.Count + 1
	return av.Packet{Data: []byte{0}}, nil
}

func TestWriteBasicRTMPErrors(t *testing.T) {
	stream := NewBasicRTMPVideoStream("test")
	err := stream.WriteRTMPToStream(context.Background(), BadStreamsDemuxer{})
	if err != ErrStreams {
		t.Error("Expecting Streams Error, but got: ", err)
	}

	err = stream.WriteRTMPToStream(context.Background(), BadPacketsDemuxer{})
	if err != ErrPacketRead {
		t.Error("Expecting Packet Read Error, but got: ", err)
	}

	err = stream.WriteRTMPToStream(context.Background(), NoEOFDemuxer{c: &Counter{Count: 0}})
	if err != ErrDroppedRTMPStream {
		t.Error("Expecting RTMP Dropped Error, but got: ", err)
	}
}

//Testing WriteRTMP
type PacketsDemuxer struct {
	c *Counter
}

func (d PacketsDemuxer) Close() error                     { return nil }
func (d PacketsDemuxer) Streams() ([]av.CodecData, error) { return nil, nil }
func (d PacketsDemuxer) ReadPacket() (av.Packet, error) {
	if d.c.Count == 10 {
		return av.Packet{Data: []byte{0, 0}}, io.EOF
	}

	d.c.Count = d.c.Count + 1
	return av.Packet{Data: []byte{0, 0}}, nil
}

func TestWriteBasicRTMP(t *testing.T) {
	// stream := Stream{Buffer: NewStreamBuffer(), StreamID: "test"}
	stream := NewBasicRTMPVideoStream("test")
	err := stream.WriteRTMPToStream(context.Background(), PacketsDemuxer{c: &Counter{Count: 0}})

	if err != io.EOF {
		t.Error("Expecting EOF, but got: ", err)
	}

	if stream.buffer.len() != 12 { //10 packets, 1 header, 1 trailer
		t.Error("Expecting buffer length to be 12, but got: ", stream.buffer.len())
	}

	// fmt.Println(stream.buffer.q.Get(12))

	//TODO: Test what happens when the buffer is full (should evict everything before the last keyframe)
}

var ErrBadHeader = errors.New("BadHeader")
var ErrBadPacket = errors.New("BadPacket")

type BadHeaderMuxer struct{}

func (d BadHeaderMuxer) Close() error                     { return nil }
func (d BadHeaderMuxer) WriteHeader([]av.CodecData) error { return ErrBadHeader }
func (d BadHeaderMuxer) WriteTrailer() error              { return nil }
func (d BadHeaderMuxer) WritePacket(av.Packet) error      { return nil }

type BadPacketMuxer struct{}

func (d BadPacketMuxer) Close() error                     { return nil }
func (d BadPacketMuxer) WriteHeader([]av.CodecData) error { return nil }
func (d BadPacketMuxer) WriteTrailer() error              { return nil }
func (d BadPacketMuxer) WritePacket(av.Packet) error      { return ErrBadPacket }

func TestReadBasicRTMPError(t *testing.T) {
	stream := NewBasicRTMPVideoStream("test")
	err := stream.WriteRTMPToStream(context.Background(), PacketsDemuxer{c: &Counter{Count: 0}})
	if err != io.EOF {
		t.Error("Error setting up the test - while inserting packet.")
	}
	err = stream.ReadRTMPFromStream(context.Background(), BadHeaderMuxer{})

	if err != ErrBadHeader {
		t.Error("Expecting bad header error, but got ", err)
	}

	err = stream.ReadRTMPFromStream(context.Background(), BadPacketMuxer{})
	if err != ErrBadPacket {
		t.Error("Expecting bad packet error, but got ", err)
	}
}

//Test ReadRTMP
type PacketsMuxer struct{}

func (d PacketsMuxer) Close() error                     { return nil }
func (d PacketsMuxer) WriteHeader([]av.CodecData) error { return nil }
func (d PacketsMuxer) WriteTrailer() error              { return nil }
func (d PacketsMuxer) WritePacket(av.Packet) error      { return nil }

func TestReadBasicRTMP(t *testing.T) {
	stream := NewBasicRTMPVideoStream("test")
	err := stream.WriteRTMPToStream(context.Background(), PacketsDemuxer{c: &Counter{Count: 0}})
	if err != io.EOF {
		t.Error("Error setting up the test - while inserting packet.")
	}
	readErr := stream.ReadRTMPFromStream(context.Background(), PacketsMuxer{})

	if readErr != io.EOF {
		t.Error("Expecting buffer to be empty, but got ", err)
	}

	if stream.buffer.len() != 0 {
		t.Error("Expecting buffer length to be 0, but got ", stream.buffer.len())
	}

}
