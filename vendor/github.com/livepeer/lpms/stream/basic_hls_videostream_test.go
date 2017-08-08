package stream

import (
	"testing"
	"time"

	"github.com/ericxtang/m3u8"
)

func TestAddAndRemove(t *testing.T) {
	stream := NewBasicHLSVideoStream("test", time.Second)
	ml, err := stream.GetMasterPlaylist()
	if len(ml.Variants) != 0 {
		t.Errorf("Expecting 0 variant, but got: %v", ml.Variants)
	}
	segs := make([]*HLSSegment, 0)
	stream.SetSubscriber(func(strm HLSVideoStream, strmID string, seg *HLSSegment) {
		segs = append(segs, seg)
	})

	//Add to the original stream
	err = stream.AddHLSSegment("test", &HLSSegment{Name: "test01.ts"})
	if err != nil {
		t.Errorf("Error adding segment: %v", err)
	}
	seg, err := stream.GetHLSSegment("test", "test01.ts")
	if err != nil {
		t.Errorf("Error getting segment: %v", err)
	}
	if seg.Name != "test01.ts" {
		t.Errorf("Expecting test01.ts, got %v", seg.Name)
	}

	//Make sure the subscriber function is called
	if len(segs) != 1 {
		t.Errorf("Subscriber never called")
	}

	//Should get original playlist (but still no master playlist)
	pl, err := stream.GetVariantPlaylist("test")
	if err != nil {
		t.Errorf("Error getting playlist: %v", err)
	}
	if pl.Segments[0].URI != "test01.ts" {
		t.Errorf("Expecting test01.ts, got %v", pl.Segments[0].URI)
	}
	mpl, err := stream.GetMasterPlaylist()
	if len(mpl.Variants) != 0 {
		t.Errorf("Expecting 0 variants, but got %v", ml.Variants)
	}

	//Add a variant
	pl, _ = m3u8.NewMediaPlaylist(10, 10)
	err = stream.AddVariant("test1", &m3u8.Variant{URI: "test1.m3u8", Chunklist: pl, VariantParams: m3u8.VariantParams{Bandwidth: 10, Resolution: "10x10"}})
	if err != nil {
		t.Errorf("Error adding media playlist: %v", err)
	}
	err = stream.AddVariant("test2", &m3u8.Variant{URI: "test2.m3u8", Chunklist: pl, VariantParams: m3u8.VariantParams{Bandwidth: 10, Resolution: "10x10"}})
	if err == nil {
		t.Errorf("Expecting error because of duplicate variant params")
	}
	pltmp, err := stream.GetVariantPlaylist("wrongName")
	if err == nil {
		t.Errorf("Expecting NotFound error because the playlist name is wrong")
	}
	segs = make([]*HLSSegment, 0)

	//Add to the wrong variant
	err = stream.AddHLSSegment("wrongStrm", &HLSSegment{})
	if err == nil {
		t.Errorf("Expecting error because strmID is wrong")
	}
	if len(segs) != 0 {
		t.Errorf("Callback should not have been called")
	}

	//Add segment to the new variant
	err = stream.AddHLSSegment("test1", &HLSSegment{SeqNo: 1, Name: "seg1.ts", Data: []byte("hello"), Duration: 8, EOF: false})
	if err != nil {
		t.Errorf("Error adding HLS Segment: %v", err)
	}
	pltmp, err = stream.GetVariantPlaylist("test1")
	if err != nil {
		t.Errorf("Error getting variant playlist: %v", err)
	}
	if pltmp.Segments[0].URI != "seg1.ts" {
		t.Errorf("Expecting segment URI to be seg1.ts, but got %v", pltmp.Segments[0].URI)
	}
	if pltmp.Segments[0].Duration != 8 {
		t.Errorf("Expecting duration to be 8, but got %v", pltmp.Segments[0].Duration)
	}
	if pltmp.Segments[0].SeqId != 1 {
		t.Errorf("Expecting seqNo to be 1, but got %v", pltmp.Segments[0].SeqId)
	}
	if pltmp.Segments[1] != nil {
		t.Errorf("Expecting to only have one segment, but the second segment is %v", pltmp.Segments[1])
	}
	if len(segs) != 1 {
		t.Errorf("Callback not invoked")
	}

	//Now master playlist should have 1 variant
	ml, err = stream.GetMasterPlaylist()
	if err != nil {
		t.Errorf("Error getting master playlist: %v", err)
	}
	if len(ml.Variants) != 1 {
		t.Errorf("Expecting 1 variant, but got: %v", ml.Variants)
	}
	if ml.Variants[0].URI != "test1.m3u8" {
		t.Errorf("Expecting test1.m3u8, but got %v", ml.Variants[0].URI)
	}

}

func TestTimeout(t *testing.T) {
	stream := NewBasicHLSVideoStream("test", time.Second)
	sc := make(chan *HLSSegment)
	ec := make(chan error)
	go func() {
		seg, err := stream.GetHLSSegment("test", "seg1.ts")
		if err != nil {
			ec <- err
		} else {
			sc <- seg
		}
	}()

	go func() {
		//Sleep for 2 sec - it's 1 sec longer than the max wait time
		time.Sleep(2 * time.Second)
		stream.AddHLSSegment("test", &HLSSegment{Name: "seg1.tx"})
	}()

	select {
	case <-ec:
	//This is what we want
	case seg := <-sc:
		t.Errorf("Expecting timeout, but got %v", seg)
	}
}
