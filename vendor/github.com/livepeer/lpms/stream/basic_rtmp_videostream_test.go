package stream

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/golang/glog"
	"github.com/nareix/joy4/av"
	"github.com/nareix/joy4/codec/h264parser"
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
	_, err := stream.WriteRTMPToStream(context.Background(), BadStreamsDemuxer{})
	if err != ErrStreams {
		t.Error("Expecting Streams Error, but got: ", err)
	}
}

//Testing WriteRTMP
type PacketsDemuxer struct {
	c           *Counter
	CalledClose bool
}

func (d *PacketsDemuxer) Close() error {
	d.CalledClose = true
	return nil
}
func (d *PacketsDemuxer) Streams() ([]av.CodecData, error) {
	return []av.CodecData{h264parser.CodecData{}}, nil
}
func (d *PacketsDemuxer) ReadPacket() (av.Packet, error) {
	if d.c.Count == 10 {
		return av.Packet{}, io.EOF
	}

	p := av.Packet{Data: []byte{0, 0}, Idx: int8(d.c.Count)}
	d.c.Count = d.c.Count + 1
	return p, nil
}

func TestWriteBasicRTMP(t *testing.T) {
	glog.Infof("\n\nTestWriteBasicRTMP\n\n")
	stream := NewBasicRTMPVideoStream("test")
	//Add a listener
	l := &PacketsMuxer{}
	stream.listeners["rand"] = l
	eof, err := stream.WriteRTMPToStream(context.Background(), &PacketsDemuxer{c: &Counter{Count: 0}})
	if err != nil {
		t.Error("Expecting nil, but got: ", err)
	}

	// Wait for eof
	select {
	case <-eof:
	}

	time.Sleep(time.Millisecond * 100) //Sleep to make sure the last segment comes through
	if len(l.packets) != 10 {
		t.Errorf("Expecting packets length to be 10, but got: %v", l.packets)
	}
}

var ErrBadHeader = errors.New("BadHeader")
var ErrBadPacket = errors.New("BadPacket")

type BadHeaderMuxer struct{}

func (d *BadHeaderMuxer) Close() error                     { return nil }
func (d *BadHeaderMuxer) WriteHeader([]av.CodecData) error { return ErrBadHeader }
func (d *BadHeaderMuxer) WriteTrailer() error              { return nil }
func (d *BadHeaderMuxer) WritePacket(av.Packet) error      { return nil }

type BadPacketMuxer struct{}

func (d *BadPacketMuxer) Close() error                     { return nil }
func (d *BadPacketMuxer) WriteHeader([]av.CodecData) error { return nil }
func (d *BadPacketMuxer) WriteTrailer() error              { return nil }
func (d *BadPacketMuxer) WritePacket(av.Packet) error      { return ErrBadPacket }

func TestReadBasicRTMPError(t *testing.T) {
	glog.Infof("\nTestReadBasicRTMPError\n\n")
	stream := NewBasicRTMPVideoStream("test")
	done := make(chan struct{})
	go func() {
		if _, err := stream.ReadRTMPFromStream(context.Background(), &BadHeaderMuxer{}); err != ErrBadHeader {
			t.Error("Expecting bad header error, but got ", err)
		}
		close(done)
	}()
	eof, err := stream.WriteRTMPToStream(context.Background(), &PacketsDemuxer{c: &Counter{Count: 0}})
	if err != nil {
		t.Error("Error writing RTMP to stream")
	}
	select {
	case <-eof:
	}
	select {
	case <-done:
	}

	stream = NewBasicRTMPVideoStream("test")
	done = make(chan struct{})
	go func() {
		eof, err := stream.ReadRTMPFromStream(context.Background(), &BadPacketMuxer{})
		if err != nil {
			t.Errorf("Expecting nil, but got %v", err)
		}
		select {
		case <-eof:
		}
		start := time.Now()
		for time.Since(start) < time.Second*2 && len(stream.listeners) > 0 {
			time.Sleep(time.Millisecond * 100)
		}
		if len(stream.listeners) > 0 {
			t.Errorf("Expecting listener to be removed, but got: %v", stream.listeners)
		}
		close(done)
	}()
	eof, err = stream.WriteRTMPToStream(context.Background(), &PacketsDemuxer{c: &Counter{Count: 0}})
	if err != nil {
		t.Error("Error writing RTMP to stream")
	}
	select {
	case <-eof:
	}
	select {
	case <-done:
	}
}

//Test ReadRTMP
type PacketsMuxer struct {
	packets     []av.Packet
	header      []av.CodecData
	trailer     bool
	CalledClose bool
}

func (d *PacketsMuxer) Close() error {
	d.CalledClose = true
	return nil
}
func (d *PacketsMuxer) WriteHeader(h []av.CodecData) error {
	d.header = h
	return nil
}
func (d *PacketsMuxer) WriteTrailer() error {
	d.trailer = true
	return nil
}
func (d *PacketsMuxer) WritePacket(pkt av.Packet) error {
	if d.packets == nil {
		d.packets = make([]av.Packet, 0)
	}
	d.packets = append(d.packets, pkt)
	return nil
}

func TestReadBasicRTMP(t *testing.T) {
	stream := NewBasicRTMPVideoStream("test")
	_, err := stream.WriteRTMPToStream(context.Background(), &PacketsDemuxer{c: &Counter{Count: 0}})
	if err != nil {
		t.Error("Error setting up the test - while inserting packet.")
	}

	r := &PacketsMuxer{}
	eof, readErr := stream.ReadRTMPFromStream(context.Background(), r)
	select {
	case <-eof:
	}
	if readErr != nil {
		t.Errorf("Expecting no error, but got %v", err)
	}

	if len(r.header) != 1 {
		t.Errorf("Expecting header to be set, got %v", r.header)
	}

	if r.trailer != true {
		t.Errorf("Expecting trailer to be set, got %v", r.trailer)
	}

	if r.CalledClose {
		t.Errorf("The mux should not have been closed.")
	}
}
