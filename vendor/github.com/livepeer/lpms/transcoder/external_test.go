package transcoder

import (
	"context"
	"io"
	"testing"

	"github.com/ericxtang/m3u8"
	"github.com/livepeer/lpms/stream"
	"github.com/nareix/joy4/av"
)

type Counter struct {
	Count int
}
type PacketsDemuxer struct {
	c *Counter
}

func (d PacketsDemuxer) Close() error                     { return nil }
func (d PacketsDemuxer) Streams() ([]av.CodecData, error) { return []av.CodecData{}, nil }
func (d PacketsDemuxer) ReadPacket() (av.Packet, error) {
	if d.c.Count == 10 {
		return av.Packet{}, io.EOF
	}

	d.c.Count = d.c.Count + 1
	return av.Packet{Data: []byte{0, 0}}, nil
}

type PacketsMuxer struct{ NumWrites int32 }

func (d *PacketsMuxer) Close() error { return nil }
func (d *PacketsMuxer) WriteHeader([]av.CodecData) error {
	d.NumWrites = d.NumWrites + 1
	return nil
}
func (d *PacketsMuxer) WriteTrailer() error {
	// fmt.Println("writing Trailer")
	d.NumWrites = d.NumWrites + 1
	return nil
}
func (d *PacketsMuxer) WritePacket(av.Packet) error {
	// fmt.Println("writing packet")
	d.NumWrites = d.NumWrites + 1
	return nil
}

func TestStartUpload(t *testing.T) {
	tr := &ExternalTranscoder{}
	mux := &PacketsMuxer{}
	demux := &PacketsDemuxer{c: &Counter{}}
	stream := stream.NewVideoStream("test", stream.RTMP)
	stream.WriteRTMPToStream(context.Background(), demux)
	ctx := context.Background()

	err := tr.StartUpload(ctx, mux, stream)
	if err != io.EOF {
		t.Error("Should have gotten EOF, but got:", err)
	}

	if mux.NumWrites != 12 {
		t.Error("Should have written 12 packets. Instead we got:", mux.NumWrites)
	}
}

type Downloader struct{}

func (d Downloader) Download(pc chan *m3u8.MediaPlaylist, sc chan *stream.HLSSegment) error {
	pl := m3u8.MediaPlaylist{}
	pc <- &pl
	for i := 0; i < 9; i++ {
		seg := stream.HLSSegment{}
		sc <- &seg
	}
	return io.EOF
}

func TestStartDownload(t *testing.T) {
	// fmt.Println("Testing Download")
	d := Downloader{}
	s := stream.NewVideoStream("test", stream.RTMP)
	tr := &ExternalTranscoder{downloader: d}
	err := tr.StartDownload(context.Background(), s)

	if err != io.EOF {
		t.Error("Expecting EOF, got", err)
	}

	if s.Len() != 10 {
		t.Error("Expecting 10 packets, got ", s.Len())
	}

}

//Be running SRS when doing this integration test
// func TestDownloader(t *testing.T) {
// 	fmt.Println("Testing Downloader - Integration Test")
// 	m := cmap.New()
// 	d := SRSHLSDownloader{cache: &m, localEndpoint: "http://localhost:7936/stream/", streamID: "live.m3u8", startDownloadWaitTime: time.Second, hlsIntervalWaitTime: time.Second * 5}
// 	pc := make(chan *m3u8.MediaPlaylist)
// 	sc := make(chan *lpmsio.HLSSegment)
// 	ec := make(chan error, 1)
// 	hlsBuffer := lpmsio.NewHLSBuffer()

// 	//Do the download into the channel (refer to the end of the method for copying into hlsBuffer)
// 	go func() { ec <- d.Download(pc, sc) }()

// 	//Set up the player
// 	player := vidplayer.VidPlayer{}
// 	player.HandleHTTPPlay(func(ctx context.Context, reqPath string, writer io.Writer) error {
// 		if strings.HasSuffix(reqPath, ".m3u8") {
// 			fmt.Println("Got m3u8 req:", reqPath)
// 			pl, err := hlsBuffer.WaitAndGetPlaylist(ctx)
// 			buf := pl.Encode()
// 			bytes := buf.Bytes()
// 			_, werr := writer.Write(bytes)
// 			if werr != nil {
// 				fmt.Println("Error Writing m3u8 playlist: ", err)
// 			}
// 			return nil

// 		}

// 		if strings.HasSuffix(reqPath, ".ts") {
// 			fmt.Println("Got ts req:", reqPath)
// 			segID := strings.Split(reqPath, "/")[2]
// 			seg, err := hlsBuffer.WaitAndGetSegment(ctx, segID)
// 			fmt.Println("Got seg: ", len(seg))
// 			if err != nil {
// 				fmt.Println("Error Writing ts segs: ", err)
// 			}
// 			_, werr := writer.Write(seg)
// 			if werr != nil {
// 				fmt.Println("Error Writing ts segs: ", err)
// 			}
// 			return nil

// 		}

// 		return errors.New("Unrecognized req string: " + reqPath)
// 	})

// 	//Get the server running
// 	go http.ListenAndServe(":8000", nil)

// 	//Do the copying into the buffer
// 	for {
// 		select {
// 		case e := <-ec:
// 			fmt.Println(e)
// 			return
// 		case pl := <-pc:
// 			for _, s := range pl.Segments {
// 				if s != nil {
// 					fmt.Println(s)
// 				}
// 			}
// 			fmt.Println("Writing playlist to hlsBuffer")
// 			hlsBuffer.WritePlaylist(*pl)
// 		case seg := <-sc:
// 			if seg.Name != "" {
// 				fmt.Printf("Writing %v to hlsBuffer\n", seg.Name)
// 				hlsBuffer.WriteSegment(seg.Name, seg.Data)
// 			} else {
// 				fmt.Printf("Skipping writting %v:%v to hlsBuffer\n", seg.Name, len(seg.Data))
// 			}
// 		}
// 	}

// }
