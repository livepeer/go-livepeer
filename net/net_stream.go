package net

import (
	"bufio"

	"github.com/golang/glog"
	net "github.com/libp2p/go-libp2p-net"
	multicodec "github.com/multiformats/go-multicodec"
	mcjson "github.com/multiformats/go-multicodec/json"
)

type WrappedStream struct {
	Stream net.Stream
	Enc    multicodec.Encoder
	Dec    multicodec.Decoder
	W      *bufio.Writer
	R      *bufio.Reader
}

func WrapStream(s net.Stream) *WrappedStream {
	reader := bufio.NewReader(s)
	writer := bufio.NewWriter(s)
	// This is where we pick our specific multicodec. In order to change the
	// codec, we only need to change this place.
	// See https://godoc.org/github.com/multiformats/go-multicodec/json
	dec := mcjson.Multicodec(false).Decoder(reader)
	enc := mcjson.Multicodec(false).Encoder(writer)
	return &WrappedStream{
		Stream: s,
		R:      reader,
		W:      writer,
		Enc:    enc,
		Dec:    dec,
	}
}

func (ws *WrappedStream) WriteSegment(seqNo uint64, data []byte) {
	glog.Infof("Writing Segment in WrappedStream")
}
