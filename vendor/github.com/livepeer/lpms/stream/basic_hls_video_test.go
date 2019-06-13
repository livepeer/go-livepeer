package stream

import (
	"testing"

	"github.com/livepeer/m3u8"
)

func TestAddAndRemove(t *testing.T) {
	manifest := NewBasicHLSVideoManifest("test_m")
	strm := NewBasicHLSVideoStream("test_s", DefaultHLSStreamWin)
	if err := manifest.AddVideoStream(strm, &m3u8.Variant{URI: "test_s", Chunklist: nil, VariantParams: m3u8.VariantParams{Bandwidth: 100}}); err != nil {
		t.Errorf("Error: %v", err)
	}
	ml, err := manifest.GetManifest()
	if len(ml.Variants) != 1 {
		t.Errorf("Expecting 1 variant, but got: %v", ml.Variants)
	}
	segs := make([]*HLSSegment, 0)
	eofRes := false
	strm.SetSubscriber(func(seg *HLSSegment, eof bool) {
		segs = append(segs, seg)
		eofRes = eof
	})

	//Add to the stream
	if err = strm.AddHLSSegment(&HLSSegment{Name: "test01.ts"}); err != nil {
		t.Errorf("Error adding segment: %v", err)
	}
	if err = strm.AddHLSSegment(&HLSSegment{Name: "test02.ts"}); err != nil {
		t.Errorf("Error adding segment: %v", err)
	}
	if err = strm.AddHLSSegment(&HLSSegment{Name: "test03.ts"}); err != nil {
		t.Errorf("Error adding segment: %v", err)
	}
	seg, err := strm.GetHLSSegment("test01.ts")
	if err != nil {
		t.Errorf("Error getting segment: %v", err)
	}
	if seg.Name != "test01.ts" {
		t.Errorf("Expecting test01.ts, got %v", seg.Name)
	}

	//Make sure the subscriber function is called
	if len(segs) != 3 {
		t.Errorf("Subscriber never called")
	}
	strm.End()
	if eofRes != true {
		t.Errorf("End didn't invoke callback")
	}

	//Should get playlist and the master playlist
	pl, err := strm.GetStreamPlaylist()
	if err != nil {
		t.Errorf("Error getting playlist: %v", err)
	}
	if pl.Segments[0].URI != "test01.ts" {
		t.Errorf("Expecting test01.ts, got %v", pl.Segments[0].URI)
	}
	mpl, err := manifest.GetManifest()
	if len(mpl.Variants) != 1 {
		t.Errorf("Expecting 1 variant, but got %v", ml.Variants)
	}

	//Add a stream
	pl, _ = m3u8.NewMediaPlaylist(3, 10)
	strm2 := NewBasicHLSVideoStream("test2", DefaultHLSStreamWin)
	if err := manifest.AddVideoStream(strm2, &m3u8.Variant{URI: "test2.m3u8", Chunklist: pl, VariantParams: m3u8.VariantParams{Bandwidth: 10, Resolution: "10x10"}}); err != nil {
		t.Errorf("Error adding media playlist: %v", err)
	}
	ml, err = manifest.GetManifest()
	if err != nil {
		t.Errorf("Error getting master playlist: %v", err)
	}
	if len(ml.Variants) != 2 {
		t.Errorf("Expecting 2 variant, but got: %v", ml.Variants)
	}

	//Add stream with duplicate name, should fail
	strm3 := NewBasicHLSVideoStream("test3", DefaultHLSStreamWin)
	if err := manifest.AddVideoStream(strm3, &m3u8.Variant{URI: "test3.m3u8", Chunklist: pl, VariantParams: m3u8.VariantParams{Bandwidth: 10, Resolution: "10x10"}}); err == nil {
		t.Errorf("Expecting error because of duplicate variant params")
	}
	_, err = manifest.GetVideoStream("wrongName")
	if err == nil {
		t.Errorf("Expecting NotFound error because the playlist name is wrong")
	}
	ml, err = manifest.GetManifest()
	if err != nil {
		t.Errorf("Error getting master playlist: %v", err)
	}
	if len(ml.Variants) != 2 {
		t.Errorf("Expecting 2 variant, but got: %v", ml.Variants)
	}

}

func TestWindowSize(t *testing.T) {
	manifest := NewBasicHLSVideoManifest("test_m")
	pl, _ := m3u8.NewMediaPlaylist(3, 10)
	strm2 := NewBasicHLSVideoStream("test2", DefaultHLSStreamWin)
	if err := manifest.AddVideoStream(strm2, &m3u8.Variant{URI: "test2.m3u8", Chunklist: pl, VariantParams: m3u8.VariantParams{Bandwidth: 10, Resolution: "10x10"}}); err != nil {
		t.Errorf("Error adding media playlist: %v", err)
	}

	//Add segments to the new stream stream, make sure it respects the window size
	vstrm, err := manifest.GetVideoStream("test2")
	segs := []*HLSSegment{}
	vstrm.SetSubscriber(func(seg *HLSSegment, eof bool) {
		segs = append(segs, seg)
	})
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	if err := vstrm.AddHLSSegment(&HLSSegment{SeqNo: 1, Name: "seg1.ts", Data: []byte("hello"), Duration: 8}); err != nil {
		t.Errorf("Error adding HLS Segment: %v", err)
	}
	if err = vstrm.AddHLSSegment(&HLSSegment{SeqNo: 2, Name: "seg2.ts", Data: []byte("hello"), Duration: 8}); err != nil {
		t.Errorf("Error adding HLS Segment: %v", err)
	}
	if err = vstrm.AddHLSSegment(&HLSSegment{SeqNo: 3, Name: "seg3.ts", Data: []byte("hello"), Duration: 8}); err != nil {
		t.Errorf("Error adding HLS Segment: %v", err)
	}
	if err = vstrm.AddHLSSegment(&HLSSegment{SeqNo: 4, Name: "seg4.ts", Data: []byte("hello"), Duration: 8}); err != nil {
		t.Errorf("Error adding HLS Segment: %v", err)
	}
	pltmp, err := vstrm.GetStreamPlaylist()
	if err != nil {
		t.Errorf("Error getting variant playlist: %v", err)
	}
	if pltmp.Segments[pltmp.SeqNo].URI != "seg2.ts" {
		t.Errorf("Expecting segment URI to be seg2.ts, but got %v", pltmp.Segments[pltmp.SeqNo].URI)
	}
	if pltmp.Segments[pltmp.SeqNo].Duration != 8 {
		t.Errorf("Expecting duration to be 8, but got %v", pltmp.Segments[pltmp.SeqNo].Duration)
	}
	if pltmp.Segments[pltmp.SeqNo].SeqId != 2 {
		t.Errorf("Expecting seqNo to be 1, but got %v", pltmp.Segments[pltmp.SeqNo].SeqId)
	}
	if pltmp.Segments[pltmp.SeqNo+1].URI != "seg3.ts" {
		t.Errorf("Expecting segment URI to be seg3.ts, but got %v", pltmp.Segments[1].URI)
	}
	if pltmp.Segments[pltmp.SeqNo+2].URI != "seg4.ts" {
		t.Errorf("Expecting segment URI to be seg4.ts, but got %v", pltmp.Segments[2].URI)
	}
	if pltmp.Count() != 3 {
		t.Errorf("Expecting to only have 3 segments, but got %v", pltmp.Count())
	}
	seg1, err := vstrm.GetHLSSegment("seg1.ts")
	if seg1 != nil {
		t.Errorf("Expecting seg1.ts to be nil (window is 3), but got %v", seg1)
	}
	seg2, err := vstrm.GetHLSSegment("seg2.ts")
	if seg2 == nil {
		t.Errorf("Expecting to find seg2")
	}
	if len(segs) != 4 {
		t.Errorf("Callback not invoked")
	}

	//Now master playlist should have 1 variant
	ml, err := manifest.GetManifest()
	if err != nil {
		t.Errorf("Error getting master playlist: %v", err)
	}
	if len(ml.Variants) != 1 {
		t.Errorf("Expecting 1 variant, but got: %v", ml.Variants)
	}
	if ml.Variants[0].URI != "test2.m3u8" {
		t.Errorf("Expecting test2, but got %v", ml.Variants[0].URI)
	}
}
