package basicnet

import (
	"bufio"
	"errors"
	"fmt"
	"sync"

	net "gx/ipfs/QmXfkENeeBvh3zYA51MaSdGUdBjhQ99cP5WQe8zgr6wchG/go-libp2p-net"
	peer "gx/ipfs/QmZoWKhxUmZ2seW4BzX6fJkNR8hh9PsGModr7q171yq2SS/go-libp2p-peer"

	multicodec "gx/ipfs/QmRDePEiL4Yupq5EkcK3L3ko3iMgYaqUdLu7xc1kqs7dnV/go-multicodec"
	mcjson "gx/ipfs/QmRDePEiL4Yupq5EkcK3L3ko3iMgYaqUdLu7xc1kqs7dnV/go-multicodec/json"

	"github.com/golang/glog"
)

var ErrOutStream = errors.New("ErrOutStream")

type OutStream interface {
	GetRemotePeer() peer.ID
	SendMessage(opCode Opcode, data interface{}) error
}

type LocalOutStream struct {
	sub *BasicSubscriber
}

func NewLocalOutStream(s *BasicSubscriber) *LocalOutStream {
	return &LocalOutStream{sub: s}
}

func (bs *LocalOutStream) GetRemotePeer() peer.ID {
	return ""
}

func (bs *LocalOutStream) SendMessage(opCode Opcode, data interface{}) error {
	if opCode != StreamDataID {
		return ErrOutStream
	}
	sd, ok := data.(StreamDataMsg)
	if !ok {
		return ErrOutStream
	}

	return bs.sub.InsertData(&sd)
}

//BasicStream is a libp2p stream wrapped in a reader and a writer.
type BasicOutStream struct {
	Stream net.Stream
	enc    multicodec.Encoder
	w      *bufio.Writer
	el     *sync.Mutex
}

//NewBasicStream creates a stream from a libp2p raw stream.
func NewBasicOutStream(s net.Stream) *BasicOutStream {
	writer := bufio.NewWriter(s)
	// This is where we pick our specific multicodec. In order to change the
	// codec, we only need to change this place.
	// See https://godoc.org/github.com/multiformats/go-multicodec/json
	enc := mcjson.Multicodec(true).Encoder(writer)

	return &BasicOutStream{
		Stream: s,
		w:      writer,
		enc:    enc,
		el:     &sync.Mutex{},
	}
}

func (bs *BasicOutStream) GetRemotePeer() peer.ID {
	if bs == nil {
		return ""
	}
	return bs.Stream.Conn().RemotePeer()
}

//SendMessage writes a message into the stream.
func (bs *BasicOutStream) SendMessage(opCode Opcode, data interface{}) error {
	if bs == nil {
		return ErrOutStream
	}
	// glog.V(common.DEBUG).Infof("Sending msg %v to %v", opCode, peer.IDHexEncode(bs.Stream.Conn().RemotePeer()))
	msg := Msg{Op: opCode, Data: data}
	return bs.encodeAndFlush(msg)
}

//EncodeAndFlush writes a message into the stream.
func (bs *BasicOutStream) encodeAndFlush(n interface{}) error {
	if bs == nil {
		fmt.Println("stream is nil")
		return ErrOutStream
	}

	bs.el.Lock()
	defer bs.el.Unlock()
	err := bs.enc.Encode(n)
	if err != nil {
		glog.Errorf("send message encode error for peer %v: %v", peer.IDHexEncode(bs.Stream.Conn().RemotePeer()), err)
		return err
	}

	err = bs.w.Flush()
	if err != nil {
		glog.Errorf("send message flush error for peer %v: %v", peer.IDHexEncode(bs.Stream.Conn().RemotePeer()), err)
		return err
	}

	return nil
}
