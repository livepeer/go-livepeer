package stream

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func TestHLSBufferGeneratePlaylist(t *testing.T) {
	b := NewHLSBuffer(5, 10)
	//Write 6 segments out of order, should be sorted.
	b.WriteSegment(9, "s_9.ts", 2, nil)
	b.WriteSegment(10, "s_10.ts", 2, nil)
	b.WriteSegment(11, "s_11.ts", 2, nil)
	b.WriteSegment(13, "s_13.ts", 2, nil)
	b.WriteSegment(12, "s_12.ts", 2, nil)
	b.WriteSegment(14, "s_14.ts", 2, nil)
	pl, err := b.LatestPlaylist()
	// pl, err := b.GeneratePlaylist(5)

	if err != nil {
		t.Errorf("Got error %v when generating playlist", err)
	}

	if pl.Count() != 5 {
		t.Errorf("Expecting 5 segments, got %v", pl.Count())
	}

	// for i := 0; i < 5; i++ {
	// 	if pl.Segments[i].URI != fmt.Sprintf("s_%v.ts", i+10) {
	// 		t.Errorf("Unexpected order: %v, %v", pl.Segments[i].URI, fmt.Sprintf("s_%v.ts", i+10))
	// 	}
	// }

	// if pl.SeqNo != 10 {
	// 	t.Errorf("Expecting 10, got %v", pl.SeqNo)
	// }

	//Cache only keeps winSize amount of data
	// if len(b.sq.Keys()) != 5 {
	// 	t.Errorf("Expecting 5 segment data, got %v", len(b.sq.Keys()))
	// }

	for i := 0; i < 5; i++ {
		if _, got := b.sq.Get(fmt.Sprintf("s_%v.ts", i+10)); got == false {
			t.Errorf("Cannot find data for seg: %v", i+10)
		}
	}

	expectedStr := `#EXTM3U
#EXT-X-VERSION:3
#EXT-X-MEDIA-SEQUENCE:10
#EXT-X-TARGETDURATION:2
#EXTINF:2.000,
s_10.ts
#EXTINF:2.000,
s_11.ts
#EXTINF:2.000,
s_12.ts
#EXTINF:2.000,
s_13.ts
#EXTINF:2.000,
s_14.ts
`

	plStr := pl.Encode().String()
	if strings.Compare(expectedStr, plStr) != 0 {
		t.Errorf("Expecting pl to be \n%v\n\n, got: \n%v\n\n", expectedStr, plStr)
	}
}

func TestHLSPushPop(t *testing.T) {
	b := NewHLSBuffer(5, 10)
	//Insert 6 segments - 1 should be evicted
	b.WriteSegment(9, "s_9.ts", 2, []byte{0})
	b.WriteSegment(10, "s_10.ts", 2, []byte{0})
	b.WriteSegment(11, "s_11.ts", 2, []byte{0})
	b.WriteSegment(12, "s_12.ts", 2, []byte{0})
	b.WriteSegment(13, "s_13.ts", 2, []byte{0})
	b.WriteSegment(14, "s_14.ts", 2, []byte{0})

	ctx := context.Background()
	for i := 0; i < 5; i++ {
		name := fmt.Sprintf("s_%v.ts", i+10) //Should start from 10
		// fmt.Println("Name: ", name)
		seg, err := b.WaitAndPopSegment(ctx, name)
		if err != nil {
			t.Errorf("Error retrieving segment")
		}
		if seg == nil {
			t.Errorf("Segment is nil, expecting a non-nil segment")
		}
	}

	// segLen := len(b.sq.Keys())
	// if segLen != 0 {
	// 	t.Errorf("Expecting length of buffer to be 0, got %v", segLen)
	// }

}
