package net

import (
	"context"
	"testing"
	"time"

	"github.com/golang/glog"
	crypto "github.com/libp2p/go-libp2p-crypto"
	net "github.com/libp2p/go-libp2p-net"
)

type SimpleMsg struct {
	Msg string
}

func simpleNodes() (*NetworkNode, *NetworkNode) {
	priv1, pub1, _ := crypto.GenerateKeyPair(crypto.RSA, 2048)
	priv2, pub2, _ := crypto.GenerateKeyPair(crypto.RSA, 2048)

	n1, _ := NewNode(15003, priv1, pub1)
	n2, _ := NewNode(15004, priv2, pub2)

	// n1.PeerHost.Peerstore().AddAddrs(n2.Identity, n2.PeerHost.Addrs(), peerstore.PermanentAddrTTL)
	// n2.PeerHost.Peerstore().AddAddrs(n1.Identity, n1.PeerHost.Addrs(), peerstore.PermanentAddrTTL)
	// n1.PeerHost.Connect(context.Background(), peerstore.PeerInfo{ID: n2.Identity})
	// n2.PeerHost.Connect(context.Background(), peerstore.PeerInfo{ID: n1.Identity})

	return n1, n2
}

func simpleHandler(ns net.Stream, txt string) {
	ws := WrapStream(ns)
	// err := ws.Enc.Encode(&SimpleMsg{Msg: txt})
	// if err != nil {
	// 	glog.Errorf("Encode error: %v", err)
	// 	return
	// }
	// for {
	var msg SimpleMsg
	err := ws.Dec.Decode(&msg)

	if err != nil {
		glog.Errorf("Got error decoding msg: %v", err)
		return
	}
	glog.Infof("%v Got msg: %v", ws.Stream.Conn().LocalPeer().Pretty(), msg)
	time.Sleep(500 * time.Millisecond)

	str := string(msg.Msg)
	newMsg := &SimpleMsg{Msg: str + "|" + txt}

	glog.Infof("Sending %v", newMsg)
	err = ws.Enc.Encode(newMsg)
	if err != nil {
		glog.Errorf("send message encode error: %v", err)
	}

	err = ws.W.Flush()
	if err != nil {
		glog.Errorf("send message flush error: %v", err)
	}
	// }
}

func simpleHandlerLoop(ws *BasicStream, txt string) {
	for {
		var msg SimpleMsg
		err := ws.Dec.Decode(&msg)

		if err != nil {
			glog.Errorf("Got error decoding msg: %v", err)
			return
		}
		glog.Infof("%v Got msg: %v", ws.Stream.Conn().LocalPeer().Pretty(), msg)

		time.Sleep(500 * time.Millisecond)

		str := string(msg.Msg)
		newMsg := &SimpleMsg{Msg: str + "|" + txt}

		glog.Infof("Sending %v", newMsg)
		err = ws.Enc.Encode(newMsg)
		if err != nil {
			glog.Errorf("send message encode error: %v", err)
		}

		err = ws.W.Flush()
		if err != nil {
			glog.Errorf("send message flush error: %v", err)
		}
	}
}

func simpleSend(ns net.Stream, txt string, t *testing.T) {
	ws := WrapStream(ns)
	err := ws.Enc.Encode(&SimpleMsg{Msg: txt})
	if err != nil {
		t.Errorf("Encoding error: %v", err)
	}
	err = ws.W.Flush()
	if err != nil {
		t.Errorf("Flush error: %v", err)
	}
}

func TestBackAndForth(t *testing.T) {
	glog.Infof("libp2p playground......")
	n1, n2 := simpleNodes()
	connectHosts(n1.PeerHost, n2.PeerHost)
	time.Sleep(time.Second)

	n2.PeerHost.SetStreamHandler("/test/1.0", func(stream net.Stream) {
		// ws := WrapStream(stream)
		// glog.Infof("ws in n2 handler: %p", ws)
		// defer stream.Close()
		simpleHandler(stream, "pong")
	})

	ns1, err := n1.PeerHost.NewStream(context.Background(), n2.Identity, "/test/1.0")
	if err != nil {
		t.Errorf("Cannot create stream: %v", err)
	}

	// n1.PeerHost.SetStreamHandler("/test/1.0", func(stream net.Stream) {
	// 	// simpleHandler(stream, "ping")
	// 	simpleHandler(ns1, "ping")
	// })

	simpleSend(ns1, "ns1", t)
	simpleHandler(ns1, "ping")
	// ns1.Close()

	// ns2, err := n1.PeerHost.NewStream(context.Background(), n2.Identity, "/test/1.0")
	// if err != nil {
	// 	t.Errorf("Cannot create stream: %v", err)
	// }
	// simpleSend(ns2, "ns2", t)

	// simpleHandler(ns1, "hi")
	// time.Sleep(time.Second)
	// ns1.Close()

	// n1.PeerHost.SetStreamHandler("/test/1.0", func(stream net.Stream) {
	// 	simpleHandler(stream, "ping")
	// })

	time.Sleep(time.Second * 5)
}
