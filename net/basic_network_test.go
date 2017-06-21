package net

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/golang/glog"
	crypto "github.com/libp2p/go-libp2p-crypto"
	host "github.com/libp2p/go-libp2p-host"
	net "github.com/libp2p/go-libp2p-net"

	peerstore "github.com/libp2p/go-libp2p-peerstore"
)

func setupNodes() (*BasicVideoNetwork, *BasicVideoNetwork) {
	priv1, pub1, _ := crypto.GenerateKeyPair(crypto.RSA, 2048)
	n1, _ := NewBasicNetwork(15000, priv1, pub1)

	priv2, pub2, _ := crypto.GenerateKeyPair(crypto.RSA, 2048)
	n2, _ := NewBasicNetwork(15001, priv2, pub2)

	// n1.NetworkNode.PeerHost.Peerstore().AddAddrs(n2.NetworkNode.Identity, n2.NetworkNode.PeerHost.Addrs(), peerstore.PermanentAddrTTL)
	// n2.NetworkNode.PeerHost.Peerstore().AddAddrs(n1.NetworkNode.Identity, n1.NetworkNode.PeerHost.Addrs(), peerstore.PermanentAddrTTL)
	// n1.NetworkNode.PeerHost.Connect(context.Background(), peerstore.PeerInfo{ID: n2.NetworkNode.Identity})
	// n2.NetworkNode.PeerHost.Connect(context.Background(), peerstore.PeerInfo{ID: n1.NetworkNode.Identity})
	return n1, n2
}

func connect(h1, h2 host.Host) {
	h1.Peerstore().AddAddrs(h2.ID(), h2.Addrs(), peerstore.PermanentAddrTTL)
	h2.Peerstore().AddAddrs(h1.ID(), h1.Addrs(), peerstore.PermanentAddrTTL)
	err := h1.Connect(context.Background(), peerstore.PeerInfo{ID: h2.ID()})
	if err != nil {
		glog.Errorf("Cannot connect h1 with h2: %v", err)
	}
	err = h2.Connect(context.Background(), peerstore.PeerInfo{ID: h1.ID()})
	if err != nil {
		glog.Errorf("Cannot connect h2 with h1: %v", err)
	}

	// Connection might not be formed right away under high load.  See https://github.com/libp2p/go-libp2p-kad-dht/blob/master/dht_test.go (func connect)
	time.Sleep(time.Millisecond * 500)
}

// func TestSettingUpNetwork(t *testing.T) {
// 	priv, pub, _ := crypto.GenerateKeyPair(crypto.RSA, 2048)
// 	_, err := NewBasicNetwork(10000, priv, pub)
// 	if err != nil {
// 		t.Errorf("Error setting up basic network: %v", err)
// 	}
// }

// func TestSendingMessage(t *testing.T) {
// 	n1, n2 := setupNodes()

// 	ns, err := n1.NetworkNode.PeerHost.NewStream(context.Background(), n2.NetworkNode.Identity, Protocol)
// 	if err != nil {
// 		t.Errorf("Error creating stream: %v", err)
// 	}

// 	glog.Infof("Sending message...")
// 	n1.NetworkNode.SendMessage(ns, n2.NetworkNode.Identity, SubReqID, SubReqMsg{StrmID: "strm"})

// 	//Should wait/check for state in n2 to make sure it got the message.
// }

// // func TestBroadcasting(t *testing.T) {
// // 	glog.Infof("\n\nTesting Broadcast Stream...")
// // 	n1, _ := setupNodes()
// // 	//n2 is simple node so we can register our own handler and inspect the incoming messages
// // 	n2, _ := simpleNodes()
// // 	connect(n1.NetworkNode.PeerHost, n2.PeerHost)

// // 	var result StreamDataMsg
// // 	//Set up handler
// // 	n2.PeerHost.SetStreamHandler(Protocol, func(s net.Stream) {
// // 		ws := WrapStream(s)
// // 		var msg Msg
// // 		err := ws.Dec.Decode(&msg)
// // 		if err != nil {
// // 			glog.Errorf("Got error decoding msg: %v", err)
// // 			return
// // 		}
// // 		result, _ = msg.Data.(StreamDataMsg)
// // 	})

// // 	b1 := n1.NewBroadcaster("strm")
// // 	//Create a new stream, this is the communication channel
// // 	ns1, err := n1.NetworkNode.PeerHost.NewStream(context.Background(), n2.Identity, Protocol)
// // 	if err != nil {
// // 		t.Errorf("Cannot create stream: %v", err)
// // 	}

// // 	//Add the stream as a listner in the broadcaster so it can be used to send out the message
// // 	b1.listeners[peer.IDHexEncode(ns1.Conn().RemotePeer())] = WrapStream(ns1)

// // 	if b1.working != false {
// // 		t.Errorf("broadcaster shouldn't be working yet")
// // 	}

// // 	//Send out the message, this should kick off the broadcaster worker
// // 	b1.Broadcast(0, []byte("test bytes"))

// // 	if b1.working == false {
// // 		t.Errorf("broadcaster shouldn be working yet")
// // 	}

// // 	//Wait until the result var is assigned
// // 	start := time.Now()
// // 	for time.Since(start) < 1*time.Second {
// // 		if result.StrmID == "" {
// // 			time.Sleep(time.Millisecond * 500)
// // 		} else {
// // 			break
// // 		}
// // 	}

// // 	if result.StrmID == "" {
// // 		t.Errorf("Never got the message")
// // 	}

// // 	if result.SeqNo != 0 {
// // 		t.Errorf("Expecting seqno to be 0, but got %v", result.SeqNo)
// // 	}

// // 	if result.StrmID != "strm" {
// // 		t.Errorf("Expecting strmID to be 'strm', but got %v", result.StrmID)
// // 	}

// // 	if string(result.Data) != "test bytes" {
// // 		t.Errorf("Expecting data to be 'test bytes', but got %v", result.Data)
// // 	}
// // }

func TestSubscribe(t *testing.T) {
	glog.Infof("\n\nTesting Subscriber...")
	n1, _ := setupNodes()
	n2, _ := simpleNodes()
	connect(n1.NetworkNode.PeerHost, n2.PeerHost)

	var subReq SubReqMsg
	//Set up handler for simple node (get a subReqMsg, write a streamDataMsg back)
	n2.PeerHost.SetStreamHandler(Protocol, func(s net.Stream) {
		ws := WrapStream(s)
		var msg Msg
		err := ws.Dec.Decode(&msg)
		if err != nil {
			glog.Errorf("Got error decoding msg: %v", err)
			return
		}
		subReq, _ = msg.Data.(SubReqMsg)
		glog.Infof("Got %v", subReq)

		for i := 0; i < 10; i++ {
			//TODO: Sleep here is needed, because we can't handle the messages fast enough.
			//I think we need to re-organize our code to kick off goroutines / workers instead of handling everything in a for loop.
			time.Sleep(time.Millisecond * 100)
			nwMsg := Msg{Op: StreamDataID, Data: StreamDataMsg{SeqNo: uint64(i), StrmID: subReq.StrmID, Data: []byte("test data")}}
			glog.Infof("Sending %v", nwMsg)
			err = ws.Enc.Encode(nwMsg)
			if err != nil {
				glog.Errorf("Cannot encode msg: %v", err)
			}
			err = ws.W.Flush()
			if err != nil {
				glog.Errorf("Cannot flush: %v", err)
			}
		}
	})

	s1 := n1.NewSubscriber("strmID")
	result := make(map[uint64][]byte)
	lock := &sync.Mutex{}
	s1.Subscribe(context.Background(), func(seqNo uint64, data []byte) {
		glog.Infof("Got response: %v, %v", seqNo, data)
		lock.Lock()
		result[seqNo] = data
		lock.Unlock()
	})

	if s1.cancelWorker == nil {
		t.Errorf("Cancel function should be assigned")
	}

	//Wait until the result var is assigned
	start := time.Now()
	for time.Since(start) < 1*time.Second {
		if subReq.StrmID == "" {
			time.Sleep(time.Millisecond * 100)
		} else {
			break
		}
	}

	if subReq.StrmID != "strmID" {
		t.Errorf("Expecting subReq.StrmID to be 'strmID', but got %v", subReq.StrmID)
	}

	time.Sleep(time.Millisecond * 3000)

	if len(result) != 10 {
		t.Errorf("Expecting length of result to be 10, but got %v: %v", len(result), result)
	}

	for _, d := range result {
		if string(d) != "test data" {
			t.Errorf("Expecting data to be 'test data', but got %v", d)
		}
	}

	//Call cancel
	//try to broadcast
	//see things not work
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
