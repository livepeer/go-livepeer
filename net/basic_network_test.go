package net

import (
	"context"
	"testing"

	"time"

	"github.com/golang/glog"
	crypto "github.com/libp2p/go-libp2p-crypto"
	peer "github.com/libp2p/go-libp2p-peer"
	peerstore "github.com/libp2p/go-libp2p-peerstore"
)

// func TestSettingUpNetwork(t *testing.T) {
// 	priv, pub, _ := crypto.GenerateKeyPair(crypto.RSA, 2048)
// 	_, err := NewBasicNetwork(10000, priv, pub)
// 	if err != nil {
// 		t.Errorf("Error setting up basic network: %v", err)
// 	}
// }

// func TestSendingMessage(t *testing.T) {
// 	priv1, pub1, _ := crypto.GenerateKeyPair(crypto.RSA, 2048)
// 	n1, _ := NewBasicNetwork(10000, priv1, pub1)

// 	priv2, pub2, _ := crypto.GenerateKeyPair(crypto.RSA, 2048)
// 	n2, _ := NewBasicNetwork(10001, priv2, pub2)

// 	//connect h1 and h2
// 	n1.NetworkNode.Peerstore.AddAddr(n2.NetworkNode.Identity, n2.NetworkNode.PeerHost.Addrs()[0], peerstore.PermanentAddrTTL)
// 	n2.NetworkNode.Peerstore.AddAddr(n1.NetworkNode.Identity, n1.NetworkNode.PeerHost.Addrs()[0], peerstore.PermanentAddrTTL)

// 	ns, err := n1.NetworkNode.PeerHost.NewStream(context.Background(), n2.NetworkNode.Identity, Protocol)
// 	if err != nil {
// 		t.Errorf("Error creating stream: %v", err)
// 	}

// 	glog.Infof("Sending message...")
// 	n1.NetworkNode.SendMessage(ns, n2.NetworkNode.Identity, SubReqID, SubReqMsg{StrmID: "strm", SubNodeID: "nid", SubNodeAddr: "naddr"})

// 	//Should wait/check for state in n2 to make sure it got the message.
// }
func TestSendingStream(t *testing.T) {
	glog.Infof("\n\nTesting Sending Stream...")
	priv1, pub1, _ := crypto.GenerateKeyPair(crypto.RSA, 2048)
	n1, _ := NewBasicNetwork(15000, priv1, pub1)

	priv2, pub2, _ := crypto.GenerateKeyPair(crypto.RSA, 2048)
	n2, _ := NewBasicNetwork(15001, priv2, pub2)

	//connect h1 and h2
	n1.NetworkNode.Peerstore.AddAddr(n2.NetworkNode.Identity, n2.NetworkNode.PeerHost.Addrs()[0], peerstore.PermanentAddrTTL)
	n2.NetworkNode.Peerstore.AddAddr(n1.NetworkNode.Identity, n1.NetworkNode.PeerHost.Addrs()[0], peerstore.PermanentAddrTTL)
	glog.Infof("n1 id: %v, addr:%v", n1.NetworkNode.Identity.Pretty(), n1.NetworkNode.PeerHost.Addrs())
	glog.Infof("n1 peerstore: %v", n1.NetworkNode.Peerstore.Peers())
	glog.Infof("n2 id: %v, addr:%v", n2.NetworkNode.Identity.Pretty(), n2.NetworkNode.PeerHost.Addrs())
	glog.Infof("n2 peerstore: %v", n2.NetworkNode.Peerstore.Peers())

	//Put a broadcast message on the queue
	b1 := n1.NewBroadcaster("strm")
	// b1.listeners["p2"] = peerstore.PeerInfo{ID: n2.NetworkNode.Identity, Addrs: n2.NetworkNode.PeerHost.Addrs()}
	b1.Broadcast(0, []byte("test bytes"))

	//Subscribe from n2 to trigger the broadcast
	ns, _ := n2.NetworkNode.PeerHost.NewStream(context.Background(), n1.NetworkNode.Identity, Protocol)
	n2.NetworkNode.SendMessage(ns, n1.NetworkNode.Identity, SubReqID,
		SubReqMsg{StrmID: "strm", SubNodeID: peer.IDHexEncode(n2.NetworkNode.Identity), SubNodeAddr: n2.NetworkNode.PeerHost.Addrs()[0].String()})

	time.Sleep(time.Second * 5)
}

func TestQueue(t *testing.T) {
	priv, pub, _ := crypto.GenerateKeyPair(crypto.RSA, 2048)
	n, _ := NewBasicNetwork(10000, priv, pub)
	b := n.NewBroadcaster("test")
	b.q.PushBack(&StreamDataMsg{SeqNo: 5, Data: []byte("hello")})

	e, ok := b.q.Front().Value.(*StreamDataMsg)
	if !ok {
		t.Errorf("Cannot convert")
	}

	if e.SeqNo != 5 {
		t.Errorf("SeqNo should be 5, but got %v", e.SeqNo)
	}
	// fmt.Printf("%v", e)
}
