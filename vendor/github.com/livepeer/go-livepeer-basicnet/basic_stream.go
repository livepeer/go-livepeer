package basicnet

import (
	"bufio"
	"errors"
	"fmt"
	"io/ioutil"
	"sync"

	net "gx/ipfs/QmNa31VPzC561NWwRsJLE7nGYZYuuD2QfpK2b1q9BK54J1/go-libp2p-net"
	peer "gx/ipfs/QmXYjuNuxVzXKJCfWasQk1RqkhVLDM9jtUKhqc2WPQmFSB/go-libp2p-peer"

	multicodec "github.com/multiformats/go-multicodec"
	mcjson "github.com/multiformats/go-multicodec/json"

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
func (bs *BasicStream) ReceiveMessage() (Msg, error) {
	bs.dl.Lock()
	defer bs.dl.Unlock()
	var msg Msg
	err := bs.dec.Decode(&msg)
	if err != nil && err.Error() == "multicodec did not match" {
		msg, err := ioutil.ReadAll(bs.r)
		glog.Infof("\n\nmulticode did not match...\nmsg:%v\nmsgstr:%s\nerr:%v\n\n", msg, string(msg), err)
	}
	return msg, err
}

//SendMessage writes a message into the stream.
func (bs *BasicStream) SendMessage(opCode Opcode, data interface{}) error {
	// glog.V(common.DEBUG).Infof("Sending msg %v to %v", opCode, peer.IDHexEncode(bs.Stream.Conn().RemotePeer()))
	msg := Msg{Op: opCode, Data: data}
	return bs.encodeAndFlush(msg)
}

//EncodeAndFlush writes a message into the stream.
func (bs *BasicStream) encodeAndFlush(n interface{}) error {
	if bs == nil {
		fmt.Println("stream is nil")
	}

	bs.el.Lock()
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
	bs.el.Unlock()

	return nil
}
