package basicnet

import (
	"bufio"
	"errors"
	"fmt"
	"sync"

	multicodec "github.com/multiformats/go-multicodec"
	mcjson "github.com/multiformats/go-multicodec/json"
	peer "gx/ipfs/QmXYjuNuxVzXKJCfWasQk1RqkhVLDM9jtUKhqc2WPQmFSB/go-libp2p-peer"
	net "gx/ipfs/QmahYsGWry85Y7WUe2SX5G4JkH2zifEQAUtJVLZ24aC9DF/go-libp2p-net"

	"github.com/golang/glog"
)

var ErrStream = errors.New("ErrStream")

//BasicStream is a libp2p stream wrapped in a reader and a writer.
type BasicStream struct {
	Stream net.Stream
	enc    multicodec.Encoder
	dec    multicodec.Decoder
	w      *bufio.Writer
	r      *bufio.Reader
	el     *sync.Mutex
	dl     *sync.Mutex
}

//NewBasicStream creates a stream from a libp2p raw stream.
func NewBasicStream(s net.Stream) *BasicStream {
	reader := bufio.NewReader(s)
	writer := bufio.NewWriter(s)
	// This is where we pick our specific multicodec. In order to change the
	// codec, we only need to change this place.
	// See https://godoc.org/github.com/multiformats/go-multicodec/json
	dec := mcjson.Multicodec(false).Decoder(reader)
	enc := mcjson.Multicodec(false).Encoder(writer)
	return &BasicStream{
		Stream: s,
		r:      reader,
		w:      writer,
		enc:    enc,
		dec:    dec,
		el:     &sync.Mutex{},
		dl:     &sync.Mutex{},
	}
}

//ReceiveMessage takes a message off the stream.
func (bs *BasicStream) ReceiveMessage(n interface{}) error {
	bs.dl.Lock()
	defer bs.dl.Unlock()
	return bs.dec.Decode(n)
}

//SendMessage writes a message into the stream.
func (bs *BasicStream) SendMessage(opCode Opcode, data interface{}) error {
	msg := Msg{Op: opCode, Data: data}
	glog.V(5).Infof("Sending: %v to %v", msg, peer.IDHexEncode(bs.Stream.Conn().RemotePeer()))
	return bs.encodeAndFlush(msg)
}

//EncodeAndFlush writes a message into the stream.
func (bs *BasicStream) encodeAndFlush(n interface{}) error {
	if bs == nil {
		fmt.Println("stream is nil")
	}
	bs.el.Lock()
	defer bs.el.Unlock()
	err := bs.enc.Encode(n)
	if err != nil {
		glog.Errorf("send message encode error for peer %v: %v", peer.IDHexEncode(bs.Stream.Conn().RemotePeer()), err)
		return ErrStream
	}

	err = bs.w.Flush()
	if err != nil {
		glog.Errorf("send message flush error for peer %v: %v", peer.IDHexEncode(bs.Stream.Conn().RemotePeer()), err)
		return ErrStream
	}

	return nil
}
