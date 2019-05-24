package stream

import (
	"context"
	"errors"
	"io"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/golang/glog"
	"github.com/nareix/joy4/av"
	"github.com/nareix/joy4/codec/h264parser"
)

//Testing WriteRTMP errors
var ErrPacketRead = errors.New("packet read error")
var ErrStreams = errors.New("streams error")
var ErrMuxClosed = errors.New("packet muxer closed")

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

	wait         bool          // whether to pause before returning a packet
	startReading chan struct{} // signal to start returning packets
}

func (d *PacketsDemuxer) Close() error {
	d.CalledClose = true
	return nil
}
func (d *PacketsDemuxer) Streams() ([]av.CodecData, error) {
	return []av.CodecData{h264parser.CodecData{}}, nil
}
func (d *PacketsDemuxer) ReadPacket() (av.Packet, error) {
	if d.wait {
		<-d.startReading
		d.wait = false
	}
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
	stream.dirty = true
	eof, err := stream.WriteRTMPToStream(context.Background(), &PacketsDemuxer{c: &Counter{Count: 0}})
	if err != nil {
		t.Error("Expecting nil, but got: ", err)
	}

	// Wait for eof
	select {
	case <-eof:
	}

	time.Sleep(time.Millisecond * 100) //Sleep to make sure the last segment comes through
	if l.numPackets() != 10 {
		t.Errorf("Expecting packets length to be 10, but got: %v", l.numPackets())
	}
}

func TestRTMPConcurrency(t *testing.T) {
	// Tests adding and removing listeners from a source while mid-stream
	// Run under -race

	glog.Infof("\n\nTest%s\n", t.Name())
	st := NewBasicRTMPVideoStream(t.Name())
	demux := &PacketsDemuxer{c: &Counter{Count: 0}, wait: true, startReading: make(chan struct{})}
	eof, err := st.WriteRTMPToStream(context.Background(), demux)
	if err != nil {
		t.Error("Error setting up test")
	}
	rng := rand.New(rand.NewSource(1234))
	muxers := []*PacketsMuxer{}
	for i := 0; i < 200; i++ {
		close := rng.Int()%2 == 0
		muxer := &PacketsMuxer{}
		if i < 100 && !close {
			// these muxers should receive all frames from demuxer because
			// they start listening before packets arrive and aren't closed
			muxers = append(muxers, muxer)
		}
		go func(close bool, p *PacketsMuxer) {
			st.ReadRTMPFromStream(context.Background(), p)
			if close {
				p.Close()
			}
		}(close, muxer)
		if i == 100 {
			// start feeding in the middle of adding listeners
			demux.startReading <- struct{}{}
		}
		time.Sleep(100 * time.Microsecond) // force thread to yield
	}

	<-eof // wait for eof

	time.Sleep(time.Millisecond * 100) // ensure last segment comes through
	if len(muxers) < 0 {
		t.Error("Expected muxers to be nonzero") // sanity check
	}
	for i, m := range muxers {
		if m.numPackets() != 10 {
			t.Errorf("%d Expected packets length to be 10, but got %v", i, m.numPackets())
		}
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
	mu          sync.Mutex
}

func (d *PacketsMuxer) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.CalledClose = true
	return nil
}
func (d *PacketsMuxer) WriteHeader(h []av.CodecData) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.header = h
	return nil
}
func (d *PacketsMuxer) WriteTrailer() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.trailer = true
	return nil
}
func (d *PacketsMuxer) WritePacket(pkt av.Packet) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.CalledClose {
		return ErrMuxClosed
	}
	if d.packets == nil {
		d.packets = make([]av.Packet, 0)
	}
	d.packets = append(d.packets, pkt)
	return nil
}

func (d *PacketsMuxer) numPackets() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.packets)
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
