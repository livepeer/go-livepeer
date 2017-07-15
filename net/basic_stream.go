package net

import (
	"bufio"
	"errors"
	"sync"

	"github.com/golang/glog"
	net "github.com/libp2p/go-libp2p-net"
	peer "github.com/libp2p/go-libp2p-peer"
	multicodec "github.com/multiformats/go-multicodec"
	mcjson "github.com/multiformats/go-multicodec/json"
)

var ErrStream = errors.New("ErrStream")

type BasicStream struct {
	Stream net.Stream
	Enc    multicodec.Encoder
	Dec    multicodec.Decoder
	W      *bufio.Writer
	R      *bufio.Reader
	l      *sync.Mutex
}

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
		R:      reader,
		W:      writer,
		Enc:    enc,
		Dec:    dec,
		l:      &sync.Mutex{},
	}
}

func (ws *BasicStream) Decode(n interface{}) error {
	// ws.l.Lock()
	// defer ws.l.Unlock()
	return ws.Dec.Decode(n)
}

func (ws *BasicStream) Encode(n interface{}) error {
	ws.l.Lock()
	defer ws.l.Unlock()
	return ws.Enc.Encode(n)
}

func (ws *BasicStream) WriteSegment(seqNo uint64, strmID string, data []byte) error {
	nwMsg := Msg{Op: StreamDataID, Data: StreamDataMsg{SeqNo: seqNo, StrmID: strmID, Data: data}}
	glog.Infof("Sending: %v::%v to %v", strmID, seqNo, peer.IDHexEncode(ws.Stream.Conn().RemotePeer()))

	err := ws.Encode(nwMsg)
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
