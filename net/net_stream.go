package net

import (
	"bufio"
	"errors"

	"github.com/golang/glog"
	net "github.com/libp2p/go-libp2p-net"
	multicodec "github.com/multiformats/go-multicodec"
	mcjson "github.com/multiformats/go-multicodec/json"
)

var ErrStream = errors.New("ErrStream")

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

func (ws *WrappedStream) WriteSegment(seqNo uint64, strmID string, data []byte) error {
	glog.Infof("Writing Segment in WrappedStream")

	nwMsg := Msg{Op: StreamDataID, Data: StreamDataMsg{SeqNo: seqNo, StrmID: strmID, Data: data}}
	glog.Infof("Sending: %v to %v", nwMsg, ws.Stream.Conn().RemotePeer().Pretty())

	err := ws.Enc.Encode(nwMsg)
	if err != nil {
		glog.Errorf("send message encode error: %v", err)
		return ErrStream
		// delete(b.listeners, id)
	}

	err = ws.W.Flush()
	if err != nil {
		glog.Errorf("send message flush error: %v", err)
		return ErrStream
		// delete(b.listeners, id)
	}

	return nil
}
