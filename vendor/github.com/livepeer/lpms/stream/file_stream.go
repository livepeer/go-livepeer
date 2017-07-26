package stream

import (
	"context"
	"io/ioutil"

	"github.com/ericxtang/m3u8"
	"github.com/golang/glog"
	"github.com/nareix/joy4/av"
)

//For now, this class is only for testing purposes (so we can write out the transcoding results and compare)
type FileStream struct {
	StreamID string
	// RTMPTimeout time.Duration
	// HLSTimeout  time.Duration
	buffer *streamBuffer
}

var outputDir = "data"

func (s *FileStream) Len() int64 {
	return s.buffer.len()
}

func NewFileStream(id string) *FileStream {
	return &FileStream{buffer: newStreamBuffer(), StreamID: id}
}

func (s *FileStream) GetStreamID() string {
	return s.StreamID
}

//ReadRTMPFromStream reads the content from the RTMP stream out into the dst.
func (s *FileStream) ReadRTMPFromStream(ctx context.Context, dst av.MuxCloser) error {
	return nil
}

func (s *FileStream) WriteRTMPToStream(ctx context.Context, src av.DemuxCloser) error {
	return nil
}

func (s *FileStream) WriteHLSPlaylistToStream(pl m3u8.MediaPlaylist) error {
	glog.Infof("Writting HLS Playlist to File")
	return nil
}

func (s *FileStream) WriteHLSSegmentToStream(seg HLSSegment) error {
	glog.Infof("Writting HLS Segment to File")
	err := ioutil.WriteFile("./data/"+s.StreamID+"_"+seg.Name, seg.Data, 0644)
	if err != nil {
		glog.Infof("Cannot write to file: ", err)
	}

	return err
}

func (s *FileStream) ReadHLSFromStream(ctx context.Context, buffer HLSMuxer) error {
	return nil
}

func (s *FileStream) ReadHLSSegment() (HLSSegment, error) {
	glog.Info("Read HLS Segment")
	return HLSSegment{}, nil
}
