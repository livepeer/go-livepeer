package net

import (
	"bufio"
	"errors"
	"fmt"
	"sync"

	net "gx/ipfs/QmahYsGWry85Y7WUe2SX5G4JkH2zifEQAUtJVLZ24aC9DF/go-libp2p-net"

	multicodec "gx/ipfs/QmVRuqGJ881CFiNLgwWSfRVjTjqQ6FeCNufkftNC4fpACZ/go-multicodec"
	mcjson "gx/ipfs/QmVRuqGJ881CFiNLgwWSfRVjTjqQ6FeCNufkftNC4fpACZ/go-multicodec/json"

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
	// glog.Infof("Sending: %v to %v", msg, peer.IDHexEncode(bs.Stream.Conn().RemotePeer()))
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
		glog.Errorf("send message encode error: %v", err)
		return ErrStream
	}

	err = bs.w.Flush()
	if err != nil {
		glog.Errorf("send message flush error: %v", err)
		return ErrStream
	}

	return nil
}
