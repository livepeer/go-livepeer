package basicnet

import (
	"bufio"

	net "gx/ipfs/QmXfkENeeBvh3zYA51MaSdGUdBjhQ99cP5WQe8zgr6wchG/go-libp2p-net"

	multicodec "gx/ipfs/QmRDePEiL4Yupq5EkcK3L3ko3iMgYaqUdLu7xc1kqs7dnV/go-multicodec"
	mcjson "gx/ipfs/QmRDePEiL4Yupq5EkcK3L3ko3iMgYaqUdLu7xc1kqs7dnV/go-multicodec/json"

	"github.com/golang/glog"
)

type InStream interface {
	ReceiveMessage() (Msg, error)
}

//BasicStream is a libp2p stream wrapped in a reader and a writer.
type BasicInStream struct {
	Stream net.Stream
	dec    multicodec.Decoder
	r      *bufio.Reader
}

//NewBasicStream creates a stream from a libp2p raw stream.
func NewBasicInStream(s net.Stream) *BasicInStream {
	reader := bufio.NewReader(s)
	// This is where we pick our specific multicodec. In order to change the
	// codec, we only need to change this place.
	// See https://godoc.org/github.com/multiformats/go-multicodec/json
	dec := mcjson.Multicodec(true).Decoder(reader)

	return &BasicInStream{
		Stream: s,
		r:      reader,
		dec:    dec,
	}
}

//ReceiveMessage takes a message off the stream.
func (bs *BasicInStream) ReceiveMessage() (Msg, error) {
	msg := Msg{}
	err := bs.dec.Decode(&msg)
	if err != nil && err.Error() == "multicodec did not match" {
		glog.Infof("\n\nmulticode did not match")
	}

	return msg, err
}
