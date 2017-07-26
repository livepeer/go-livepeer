package main

import (
	"context"
	"io"
	"testing"

	"github.com/nareix/joy4/av"
)

type TestDemuxer struct{}

var count int

func (d TestDemuxer) Close() error                     { return nil }
func (d TestDemuxer) Streams() ([]av.CodecData, error) { return nil, nil }
func (d TestDemuxer) ReadPacket() (av.Packet, error) {
	if count < 207 {
		isKeyframe := false
		if count == 3 || count == 53 || count == 140 || count == 185 {
			isKeyframe = true
		}
		count = count + 1
		return av.Packet{IsKeyFrame: isKeyframe, Data: []byte{0, 0}}, nil
	}

	return av.Packet{}, io.EOF
}

type TestMuxer struct{}

var Header []av.CodecData
var Packets []av.Packet

func (d TestMuxer) Close() error { return nil }
func (d TestMuxer) WriteHeader(h []av.CodecData) error {
	Header = h
	return nil
}
func (d TestMuxer) WriteTrailer() error { return nil }
func (d TestMuxer) WritePacket(p av.Packet) error {
	Packets = append(Packets, p)
	return nil
}

func TestSegmentStream(t *testing.T) {
	// fmt.Printf("Testing Segment Stream")
	s := NewSegmentStream("test")
	s.WriteRTMPToStream(context.Background(), &TestDemuxer{})
	if len(s.Segments) != 5 {
		t.Errorf("Expecting 5 segments, got %v", len(s.Segments))
	}
	// glog.Infof("Done Inserting")

	Packets = make([]av.Packet, 0, 207)
	s.ReadRTMPFromStream(context.Background(), &TestMuxer{})
	if len(Packets) != 207 {
		t.Errorf("Expecting 207 packets, got %v", len(Packets))
	}
}
