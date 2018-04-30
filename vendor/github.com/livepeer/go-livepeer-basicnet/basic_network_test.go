package basicnet

import (
	"bytes"
	"context"
	"fmt"
	host "gx/ipfs/QmNmJZL7FQySMtE2BQuLMuZg2EB2CLEunJJUSVSc9YnnbV/go-libp2p-host"
	kb "gx/ipfs/QmTH6VLu3WXfbH3nuLdmscgPWuiPZv3GMJ2YCdzBS5z91T/go-libp2p-kbucket"
	net "gx/ipfs/QmXfkENeeBvh3zYA51MaSdGUdBjhQ99cP5WQe8zgr6wchG/go-libp2p-net"
	kad "gx/ipfs/QmY1y2M1aCcVhy8UuTbZJBvuFbegZm47f9cDAdgxiehQfx/go-libp2p-kad-dht"
	peer "gx/ipfs/QmZoWKhxUmZ2seW4BzX6fJkNR8hh9PsGModr7q171yq2SS/go-libp2p-peer"
	crypto "gx/ipfs/QmaPbCnUMBohSGo3KnxEa2bHqyJVVeEEcwtqJAYxerieBo/go-libp2p-crypto"
	"reflect"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/ericxtang/m3u8"
	"github.com/golang/glog"
)

func init() {
	// flag.Set("alsologtostderr", fmt.Sprintf("%t", true))
	// var logLevel string
	// flag.StringVar(&logLevel, "logLevel", "6", "test")
	// flag.Lookup("v").Value.Set(logLevel)
}

type keyPair struct {
	Priv crypto.PrivKey
	Pub  crypto.PubKey
}

func TestReconnect(t *testing.T) {
	glog.Infof("\n\nTesting Reconnect...")
	n1, n2 := setupNodes(t, 15000, 15001)
	connectHosts(n1.NetworkNode.(*BasicNetworkNode).PeerHost, n2.NetworkNode.(*BasicNetworkNode).PeerHost)
	defer n1.NetworkNode.(*BasicNetworkNode).PeerHost.Close()
	defer n2.NetworkNode.(*BasicNetworkNode).PeerHost.Close()

	//Send a message, it should work
	s := n2.NetworkNode.(*BasicNetworkNode).GetOutStream(n1.NetworkNode.ID())
	if err := s.SendMessage(GetMasterPlaylistReqID, GetMasterPlaylistReqMsg{ManifestID: "strmID1"}); err != nil {
		t.Errorf("Error sending message: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	//Kill n2, create a new n2
	if err := n2.NetworkNode.(*BasicNetworkNode).PeerHost.Close(); err != nil {
		t.Errorf("Error closing host: %v", err)
	}
	priv2, pub2, _ := crypto.GenerateKeyPair(crypto.RSA, 2048)
	no2, _ := NewNode(addrs(15001), priv2, pub2, &BasicNotifiee{})
	n2, _ = NewBasicVideoNetwork(no2, "", "", 0)
	go n2.SetupProtocol()
	connectHosts(n1.NetworkNode.(*BasicNetworkNode).PeerHost, n2.NetworkNode.(*BasicNetworkNode).PeerHost)
	s = n2.NetworkNode.GetOutStream(n1.NetworkNode.(*BasicNetworkNode).Identity)
	if s == nil || s.Stream == nil {
		t.Errorf("Got nil for stream to: %v", n1.NetworkNode.(*BasicNetworkNode).Identity)
	}

	//Send should still work
	if err := s.SendMessage(GetMasterPlaylistReqID, GetMasterPlaylistReqMsg{ManifestID: "strmID2"}); err != nil {
		t.Errorf("Error sending message: %v", err)
	}

	time.Sleep(50 * time.Millisecond)
}

func TestStream(t *testing.T) {
	glog.Infof("\n\nTesting Stream...")
	n1, n2 := setupNodes(t, 15000, 15001)
	defer n1.NetworkNode.(*BasicNetworkNode).PeerHost.Close()
	defer n2.NetworkNode.(*BasicNetworkNode).PeerHost.Close()
	go n1.SetupProtocol()
	go n2.SetupProtocol()
	connectHosts(n1.NetworkNode.(*BasicNetworkNode).PeerHost, n2.NetworkNode.(*BasicNetworkNode).PeerHost)

	strmID1 := fmt.Sprintf("%vstrmID", peer.IDHexEncode(n1.NetworkNode.ID()))
	strmID2 := fmt.Sprintf("%vstrmID", peer.IDHexEncode(n2.NetworkNode.ID()))
	//Should be able to send messages back and forth
	s12 := n1.NetworkNode.GetOutStream(n2.NetworkNode.ID())
	if err := s12.SendMessage(SubReqID, SubReqMsg{StrmID: strmID2}); err != nil {
		t.Errorf("Error: %v", err)
	}
	if _, ok := n1.NetworkNode.(*BasicNetworkNode).outStreams[n2.NetworkNode.ID()]; !ok {
		t.Errorf("Expecting stream to be there")
	}
	start := time.Now()
	for ; n2.msgCounts[SubReqID] != 1 && time.Since(start) < time.Second; time.Sleep(100 * time.Millisecond) {
	}
	if n2.msgCounts[SubReqID] != 1 {
		t.Errorf("Expecting 1 message received by n2")
	}
	s21 := n2.NetworkNode.GetOutStream(n1.NetworkNode.ID())
	if err := s21.SendMessage(SubReqID, SubReqMsg{StrmID: strmID1}); err != nil {
		t.Errorf("Error: %v", err)
	}
	if _, ok := n2.NetworkNode.(*BasicNetworkNode).outStreams[n1.NetworkNode.ID()]; !ok {
		t.Errorf("Expecting stream to be there")
	}
	start = time.Now()
	for ; n1.msgCounts[SubReqID] != 1 && time.Since(start) < time.Second; time.Sleep(100 * time.Millisecond) {
	}
	if n1.msgCounts[SubReqID] != 1 {
		t.Errorf("Expecting 1 message received by n1")
	}
}

func TestStreamError(t *testing.T) {
	glog.Infof("\n\nTesting Stream Error...")
	n1, n2 := setupNodes(t, 15000, 15001)
	defer n1.NetworkNode.(*BasicNetworkNode).PeerHost.Close()
	defer n2.NetworkNode.(*BasicNetworkNode).PeerHost.Close()
	go n1.SetupProtocol()
	go n2.SetupProtocol()
	connectHosts(n1.NetworkNode.(*BasicNetworkNode).PeerHost, n2.NetworkNode.(*BasicNetworkNode).PeerHost)

	strmID2 := fmt.Sprintf("%vstrmID", peer.IDHexEncode(n2.NetworkNode.ID()))
	s12 := n1.NetworkNode.GetOutStream(n2.NetworkNode.ID())

	//Now cause a problem - use a bad message (Cannot use a StreamDataMsg for CancelSubID)
	glog.Infof("Sending bad message...")
	err := s12.enc.Encode("{}")
	if err != nil {
		t.Errorf("Error encoding to stream")
	}

	err = s12.w.Flush()
	if err != nil {
		t.Errorf("Error flushing")
	}
	time.Sleep(time.Millisecond * 100)
	if n2.msgCounts[SubReqID] != 0 {
		t.Errorf("Expecting 0 message received by n2, but got %v", n2.msgCounts[SubReqID])
	}

	//Should still be able to send even after a bad message was sent
	if err := n1.SendTranscodeResponse(n2.GetNodeID(), strmID2, map[string]string{"strm": "strm"}); err != nil {
		t.Errorf("Expecting to not get an error, but got: %v", err)
	}
	start := time.Now()
	for ; n2.msgCounts[TranscodeResponseID] != 1 && time.Since(start) < time.Second; time.Sleep(100 * time.Millisecond) {
	}
	if n2.msgCounts[TranscodeResponseID] != 1 {
		t.Errorf("Expecting 1 message received by n2, but got %v", n1.msgCounts[TranscodeResponseID])
	}
}

func TestSubPath(t *testing.T) {
	glog.Infof("\n\nTesting SubPath...")
	ctx := context.Background()
	nDHTs := 10
	dhts, hosts := setupDHTS(ctx, nDHTs, t)
	defer func() {
		for i := 0; i < nDHTs; i++ {
			dhts[i].Close()
			defer hosts[i].Close()
		}
	}()

	ids := make([]peer.ID, 10)
	dhtLookup := make(map[peer.ID]*kad.IpfsDHT)
	hostsLookup := make(map[peer.ID]host.Host)
	for i, dht := range dhts {
		id := hosts[i].ID()
		ids[i] = id
		dhtLookup[id] = dht
		hostsLookup[id] = hosts[i]
	}

	glog.Infof("id0: %v", peer.IDHexEncode(ids[0]))
	ids = kb.SortClosestPeers(ids, kb.ConvertPeerID(ids[0]))
	//Connect 9 with 6-8
	for i := 6; i < 9; i++ {
		connect(t, ctx, dhtLookup[ids[9]], dhtLookup[ids[i]], hostsLookup[ids[9]], hostsLookup[ids[i]])
	}
	//Connect 6 with 3-5
	for i := 3; i < 6; i++ {
		connect(t, ctx, dhtLookup[ids[6]], dhtLookup[ids[i]], hostsLookup[ids[6]], hostsLookup[ids[i]])
	}
	//Connect 3 with 0-2
	for i := 0; i < 3; i++ {
		connect(t, ctx, dhtLookup[ids[3]], dhtLookup[ids[i]], hostsLookup[ids[3]], hostsLookup[ids[i]])
	}

	for _, id := range ids {
		ps := hostsLookup[id].Peerstore().Peers()
		pstr := ""
		for _, p := range ps {
			pstr = fmt.Sprintf("%v, %v", pstr, peer.IDHexEncode(p))
		}
		// glog.Infof("ID: %v, Addrs: %v, Peers: %v", peer.IDHexEncode(id), hostsLookup[id].Addrs(), pstr)
		glog.Infof("ID: %v, Peers: %v", peer.IDHexEncode(id), pstr)
	}
	nodes := make([]*BasicVideoNetwork, 10, 10)
	for i, id := range ids {
		n_tmp := newNode(id, dhtLookup[id], hostsLookup[id])
		n, _ := NewBasicVideoNetwork(n_tmp, "", "", 0)
		nodes[i] = n
		if i != 0 {
			go n.SetupProtocol()
		}
	}

	strmID := fmt.Sprintf("%v%v", nodes[0].GetNodeID(), "strmID")
	hostsLookup[ids[0]].SetStreamHandler(Protocol, func(s net.Stream) {
		ws := NewBasicInStream(s)
		msg, err := ws.ReceiveMessage()
		if err != nil {
			t.Errorf("Error receiving msg: %v", err)
		}
		glog.Infof("n0 got msg: %v", msg)
		if msg.Op != SubReqID {
			t.Errorf("Expecting Sub")
		}
		time.Sleep(100 * time.Millisecond)
		if err := nodes[0].NetworkNode.GetOutStream(s.Conn().RemotePeer()).SendMessage(StreamDataID, StreamDataMsg{StrmID: strmID, Data: []byte("Hello from n0")}); err != nil {
			t.Errorf("Error sending message from n0: %v", err)
		}
	})

	glog.Infof("Sending Sub from %v, StrmID: %v", peer.IDHexEncode(nodes[0].NetworkNode.ID()), strmID)
	sub, err := nodes[9].GetSubscriber(strmID)
	if err != nil {
		t.Errorf("error: %v", err)
	}
	bc := make(chan bool)
	sub.Subscribe(ctx, func(segNo uint64, data []byte, eof bool) {
		glog.Infof("n9 got msg: %v", string(data))
		bc <- true
	})

	timer := time.NewTimer(time.Second * 2)
	select {
	case <-bc:
		//pass
	case <-timer.C:

		glog.Infof("n0: %v", nodes[0].relayers)
		glog.Infof("n3: %v", nodes[3].relayers)
		glog.Infof("n6: %v", nodes[6].relayers)
		glog.Infof("n9: %v", nodes[9].relayers)
		t.Errorf("Timed out")
	}
}

func newNode(pid peer.ID, dht *kad.IpfsDHT, rHost host.Host) *BasicNetworkNode {
	streams := make(map[peer.ID]*BasicOutStream)
	nn := &BasicNetworkNode{Identity: pid, Kad: dht, PeerHost: rHost, outStreams: streams, outStreamsLock: &sync.Mutex{}}
	return nn
}

func TestSubPeerForwardPath(t *testing.T) {
	// connect(t, ctx, )
	keys := make([]keyPair, 3)
	for i := 0; i < 3; i++ {
		priv, pub, _ := crypto.GenerateKeyPair(crypto.RSA, 2048)
		keys[i] = keyPair{Priv: priv, Pub: pub}
	}

	// glog.Infof("keys: %v", keys)
	sort.Slice(keys, func(i, j int) bool {
		ibytes, _ := keys[i].Pub.Bytes()
		jbytes, _ := keys[j].Pub.Bytes()
		return bytes.Compare(ibytes, jbytes) < 0
	})
	// glog.Infof("keys: %v", keys)

	no1, _ := NewNode(addrs(15000), keys[0].Priv, keys[0].Pub, &BasicNotifiee{})
	n1, _ := NewBasicVideoNetwork(no1, "", "", 0)
	no2, _ := NewNode(addrs(15001), keys[1].Priv, keys[1].Pub, &BasicNotifiee{})
	no3, _ := NewNode(addrs(15000), keys[2].Priv, keys[2].Pub, &BasicNotifiee{}) //Make this node unreachable from n1 because it's using the same port
	defer no1.PeerHost.Close()
	defer n1.NetworkNode.(*BasicNetworkNode).PeerHost.Close()
	defer no2.PeerHost.Close()
	defer no3.PeerHost.Close()

	connectHosts(n1.NetworkNode.(*BasicNetworkNode).PeerHost, no2.PeerHost)
	connectHosts(no2.PeerHost, no3.PeerHost)

	n3chan := make(chan bool)
	no3.PeerHost.SetStreamHandler(Protocol, func(s net.Stream) {
		ws := NewBasicInStream(s)
		_, err := ws.ReceiveMessage()
		if err != nil {
			t.Errorf("Error receiving msg: %v", err)
		}
		n3chan <- true
		// glog.Infof("no3 msg: %v", msg)
	})

	n2chan := make(chan bool)
	no2.PeerHost.SetStreamHandler(Protocol, func(s net.Stream) {
		ws := NewBasicInStream(s)
		_, err := ws.ReceiveMessage()
		if err != nil {
			t.Errorf("Error receiving msg: %v", err)
		}
		n2chan <- true
		// glog.Infof("no2 msg: %v", msg)
	})

	//n1 subscribe from n3 - should go through n2 because n3 is not directly reachable from n1
	strmID := fmt.Sprintf("%v%v", peer.IDHexEncode(no3.Identity), "strmID")
	s1tmp, _ := n1.GetSubscriber(strmID)
	s1, _ := s1tmp.(*BasicSubscriber)
	s1.Subscribe(context.Background(), func(seqNo uint64, data []byte, eof bool) {
		glog.Infof("Got response: %v, %v", seqNo, data)
	})

	timer := time.NewTimer(time.Second)
	select {
	case <-n2chan:
		//This is the good case
		return
	case <-n3chan:
		t.Errorf("Should go to n2 instead.")
	case <-timer.C:
		t.Errorf("Timeout")
	}

}

func TestSendBroadcast(t *testing.T) {
	glog.Infof("\n\nTesting Broadcast Stream...")
	n1, n3 := setupNodes(t, 15000, 15001)
	//n2 is simple node so we can register our own handler and inspect the incoming messages
	n2, n4 := simpleNodes(15002, 15003)
	defer n1.NetworkNode.(*BasicNetworkNode).PeerHost.Close()
	defer n3.NetworkNode.(*BasicNetworkNode).PeerHost.Close()
	defer n2.PeerHost.Close()
	defer n4.PeerHost.Close()
	connectHosts(n1.NetworkNode.(*BasicNetworkNode).PeerHost, n2.PeerHost)

	var strmData StreamDataMsg
	var finishStrm FinishStreamMsg
	//Set up handler
	n2.PeerHost.SetStreamHandler(Protocol, func(s net.Stream) {
		ws := NewBasicInStream(s)
		msg, err := ws.ReceiveMessage()
		if err != nil {
			glog.Errorf("Got error decoding msg: %v", err)
			return
		}
		switch msg.Data.(type) {
		case StreamDataMsg:
			strmData, _ = msg.Data.(StreamDataMsg)
		case FinishStreamMsg:
			finishStrm, _ = msg.Data.(FinishStreamMsg)

		}
	})

	b1tmp, _ := n1.GetBroadcaster("strm")
	b1, _ := b1tmp.(*BasicBroadcaster)
	//Create a new stream, this is the communication channel
	ns1, err := n1.NetworkNode.(*BasicNetworkNode).PeerHost.NewStream(context.Background(), n2.ID(), Protocol)
	if err != nil {
		t.Errorf("Cannot create stream: %v", err)
	}

	//Add the stream as a listner in the broadcaster so it can be used to send out the message
	b1.listeners[peer.IDHexEncode(ns1.Conn().RemotePeer())] = NewBasicOutStream(ns1)

	if b1.working != false {
		t.Errorf("broadcaster shouldn't be working yet")
	}

	//Send out the message, this should kick off the broadcaster worker
	b1.Broadcast(0, []byte("test bytes"))

	if b1.working == false {
		t.Errorf("broadcaster shouldn be working yet")
	}

	//Wait until the result var is assigned
	start := time.Now()
	for time.Since(start) < 1*time.Second {
		if strmData.StrmID == "" {
			time.Sleep(time.Millisecond * 100)
		} else {
			break
		}
	}

	if strmData.StrmID == "" {
		t.Errorf("Never got the message")
	}

	if strmData.SeqNo != 0 {
		t.Errorf("Expecting seqno to be 0, but got %v", strmData.SeqNo)
	}

	if strmData.StrmID != "strm" {
		t.Errorf("Expecting strmID to be 'strm', but got %v", strmData.StrmID)
	}

	if string(strmData.Data) != "test bytes" {
		t.Errorf("Expecting data to be 'test bytes', but got %v", strmData.Data)
	}
}

func TestHandleBroadcast(t *testing.T) {
	glog.Infof("\n\nTesting Handle Broadcast...")
	n1, _ := setupNodes(t, 15000, 15001)
	n2, _ := simpleNodes(15002, 15003)
	defer n1.NetworkNode.(*BasicNetworkNode).PeerHost.Close()
	defer n2.PeerHost.Close()
	connectHosts(n1.NetworkNode.(*BasicNetworkNode).PeerHost, n2.PeerHost)

	cancelChan := make(chan CancelSubMsg)
	//Set up n2 handler so n1 can create a stream to it.
	n2.PeerHost.SetStreamHandler(Protocol, func(s net.Stream) {
		ws := NewBasicInStream(s)
		msg, err := ws.ReceiveMessage()
		if err != nil {
			glog.Errorf("Got error decoding msg: %v", err)
			return
		}
		cancelMsg, ok := msg.Data.(CancelSubMsg)
		if !ok {
			t.Errorf("Expecting cancelSubMsg, but got %v", msg.Data)
		}
		cancelChan <- cancelMsg
	})

	//Error case - subscriber is not set yet.
	err := handleStreamData(n1, n2.Identity, &StreamDataMsg{SeqNo: 100, StrmID: "strmID", Data: []byte("hello")})
	if err != ErrHandleMsg {
		t.Errorf("Expecting error because no subscriber has been assigned, got %v", err)
	}

	s1tmp, _ := n1.GetSubscriber("strmID")
	s1, _ := s1tmp.(*BasicSubscriber)
	//Set up the subscriber to handle the streamData
	ctxW, cancel := context.WithCancel(context.Background())
	s1.cancelWorker = cancel
	s1.working = true
	outStrm := n1.NetworkNode.GetOutStream(n2.Identity)
	var seqNoResult uint64
	var dataResult []byte
	s1GotMsgChan := make(chan struct{})
	s1.startWorker(ctxW, outStrm, func(seqNo uint64, data []byte, eof bool) {
		seqNoResult = seqNo
		dataResult = data
		s1GotMsgChan <- struct{}{}
	})
	n1.SetSubscriber("strmID", s1)

	go func() {
		for {
			if n1.getSubscriber("strmID") != nil {
				err = handleStreamData(n1, n2.Identity, &StreamDataMsg{SeqNo: 100, StrmID: "strmID", Data: []byte("hello")})
				if err != nil {
					t.Errorf("handleStreamData error: %v", err)
				}
				return
			} else {
				time.Sleep(time.Millisecond * 50)
			}
		}
	}()

	//Wait until the result vars are assigned
	select {
	case <-s1GotMsgChan:
	case <-time.After(time.Second):
		t.Errorf("Timed out!")
	}
	if seqNoResult != 100 {
		t.Errorf("Expecting seqNo to be 100, but got: %v", seqNoResult)
	}

	if string(dataResult) != "hello" {
		t.Errorf("Expecting data to be 'hello', but got: %v", dataResult)
	}

	//Test Unsubscribe
	if err := s1.Unsubscribe(); err != nil {
		t.Errorf("Error unsubscribing: %v", err)
	}

	//Wait for cancelMsg to be assigned
	var cancelMsg CancelSubMsg
	select {
	case cancelMsg = <-cancelChan:
	case <-time.After(time.Second):
		t.Errorf("Timed out!")
	}
	if s1.working {
		t.Errorf("Subscriber worker shouldn't be working anymore")
	}
	// if s1.networkStream != nil {
	// 	t.Errorf("networkStream should be nil, but got: %v", s1.networkStream)
	// }
	if cancelMsg.StrmID != "strmID" {
		t.Errorf("Expecting cancelMsg.StrmID to be 'strmID' (cancelMsg to be sent because of cancelWorker()), but got %v", cancelMsg.StrmID)
	}
}

func TestSendSubscribe(t *testing.T) {
	glog.Infof("\n\nTesting Subscriber...")
	n1, _ := setupNodes(t, 15000, 15001)
	n2, _ := simpleNodes(15002, 15003)
	defer n1.NetworkNode.(*BasicNetworkNode).PeerHost.Close()
	defer n2.PeerHost.Close()
	connectHosts(n1.NetworkNode.(*BasicNetworkNode).PeerHost, n2.PeerHost)

	var subReq SubReqMsg
	var cancelMsg CancelSubMsg
	//Set up handler for simple node (get a subReqMsg, write a streamDataMsg back)
	n2.PeerHost.SetStreamHandler(Protocol, func(s net.Stream) {
		ws := NewBasicInStream(s)
		for {
			msg, err := ws.ReceiveMessage()
			if err != nil {
				glog.Errorf("Got error decoding msg: %v", err)
				return
			}
			switch msg.Data.(type) {
			case SubReqMsg:
				subReq, _ = msg.Data.(SubReqMsg)
				// glog.Infof("Got SubReq %v", subReq)

				for i := 0; i < 10; i++ {
					//TODO: Sleep here is needed, because we can't handle the messages fast enough.
					//I think we need to re-organize our code to kick off goroutines / workers instead of handling everything in a for loop.
					time.Sleep(time.Millisecond * 100)
					if err := n2.GetOutStream(ws.Stream.Conn().RemotePeer()).SendMessage(StreamDataID, StreamDataMsg{SeqNo: uint64(i), StrmID: subReq.StrmID, Data: []byte("test data")}); err != nil {
						t.Errorf("Error sending data back to n1: %v", err)
					}
				}
			case CancelSubMsg:
				cancelMsg, _ = msg.Data.(CancelSubMsg)
				glog.Infof("Got CancelMsg %v", cancelMsg)
			default:
				glog.Infof("Got unknown msg: %v", msg)
			}
		}
	})

	strmID := fmt.Sprintf("%vstrmID", peer.IDHexEncode(n2.Identity))
	s1tmp, _ := n1.GetSubscriber(strmID)
	s1, _ := s1tmp.(*BasicSubscriber)
	result := make(map[uint64][]byte)
	lock := &sync.Mutex{}
	s1.Subscribe(context.Background(), func(seqNo uint64, data []byte, eof bool) {
		// glog.Infof("Got response: %v, %v", seqNo, data)
		lock.Lock()
		result[seqNo] = data
		lock.Unlock()
	})

	if s1.cancelWorker == nil {
		t.Errorf("Cancel function should be assigned")
	}

	//Wait until the result var is assigned
	start := time.Now()
	for time.Since(start) < 3*time.Second {
		if subReq.StrmID == "" {
			time.Sleep(time.Millisecond * 100)
		} else {
			break
		}
	}

	if subReq.StrmID != strmID {
		t.Errorf("Expecting subReq.StrmID to be 'strmID', but got %v", subReq.StrmID)
	}

	if !s1.working {
		t.Errorf("Subscriber should be working")
	}

	for start := time.Now(); time.Since(start) < 3*time.Second; {
		if len(result) == 10 {
			break
		} else {
			time.Sleep(time.Millisecond * 50)
		}
	}
	if len(result) != 10 {
		t.Errorf("Expecting length of result to be 10, but got %v: %v", len(result), result)
	}

	for _, d := range result {
		if string(d) != "test data" {
			t.Errorf("Expecting data to be 'test data', but got %v", d)
		}
	}

	//Call cancel
	s1.cancelWorker()

	for start := time.Now(); time.Since(start) < 1*time.Second; {
		if cancelMsg.StrmID != "" {
			break
		} else {
			time.Sleep(50 * time.Millisecond)
		}
	}
	if cancelMsg.StrmID != strmID {
		t.Errorf("Expecting to get cancelMsg with StrmID: %v, but got %v", strmID, cancelMsg.StrmID)
	}

	if s1.working {
		t.Errorf("subscriber shouldn't be working after 'cancel' is called")
	}
}

func TestHandleCancel(t *testing.T) {
	n1, n2 := setupNodes(t, 15000, 15001)
	defer n1.NetworkNode.(*BasicNetworkNode).PeerHost.Close()
	defer n2.NetworkNode.(*BasicNetworkNode).PeerHost.Close()

	nid1, _ := peer.IDHexDecode("122024506e16a51b9853ae5a019d7d99549414e7c053116f6de533cac987ace38420")
	nid2, _ := peer.IDHexDecode("1220fd6156923c7138dc1b4388ab59a3eb0631c4e673499d35d47f1af32f2c92de66")
	//Put a broadcaster with a single listener in the node, make sure cancel removes the listener
	strmID1 := "strmID1"
	b := &BasicBroadcaster{listeners: map[string]OutStream{peer.IDHexEncode(nid1): nil}}
	n1.broadcasters[strmID1] = b
	if err := handleCancelSubReq(n1, CancelSubMsg{StrmID: strmID1}, nid1); err != nil {
		t.Errorf("Error handling req: %v", err)
	}
	if len(b.listeners) != 0 {
		t.Errorf("Expecting 0 listerns, but go %v", len(b.listeners))
	}
	delete(n1.broadcasters, strmID1)

	//Put a relayer with 2 listeners in the node, make sure cancel removes the listener, then the relayer
	r := &BasicRelayer{listeners: map[string]*BasicOutStream{peer.IDHexEncode(nid1): nil, peer.IDHexEncode(nid2): nil}, Network: n1}
	n1.relayers[relayerMapKey(strmID1, SubReqID)] = r
	if err := handleCancelSubReq(n1, CancelSubMsg{StrmID: strmID1}, nid1); err != nil {
		t.Errorf("Error handling req: %v", err)
	}
	if len(r.listeners) != 1 {
		t.Errorf("Expecting 1 listener, but got %v", len(r.listeners))
	}
	//Remove the same nid again, it shouldn't change anything
	if err := handleCancelSubReq(n1, CancelSubMsg{StrmID: strmID1}, nid1); err != nil {
		t.Errorf("Error handling req: %v", err)
	}
	if len(r.listeners) != 1 {
		t.Errorf("Expecting 1 listener, but got %v", len(r.listeners))
	}
	//Should have no listeners left
	if err := handleCancelSubReq(n1, CancelSubMsg{StrmID: strmID1}, nid2); err != nil {
		t.Errorf("Error handling req: %v", err)
	}
	if len(r.listeners) != 0 {
		t.Errorf("Expecting 0 listener, but got %v", len(r.listeners))
	}
}

func TestHandleSubscribe(t *testing.T) {
	glog.Infof("\n\nTesting Handle Subscribe...")
	n1, n3 := setupNodes(t, 15000, 15001)
	n2, n4 := simpleNodes(15002, 15003)
	defer n1.NetworkNode.(*BasicNetworkNode).PeerHost.Close()
	defer n3.NetworkNode.(*BasicNetworkNode).PeerHost.Close()
	defer n2.PeerHost.Close()
	defer n4.PeerHost.Close()
	connectHosts(n1.NetworkNode.(*BasicNetworkNode).PeerHost, n2.PeerHost)
	connectHosts(n1.NetworkNode.(*BasicNetworkNode).PeerHost, n4.PeerHost)

	n2chan := make(chan string)
	n2.PeerHost.SetStreamHandler(Protocol, func(s net.Stream) {
		defer s.Close()
		ws := NewBasicInStream(s)
		for {
			msg, err := ws.ReceiveMessage()
			if err != nil {
				glog.Errorf("N2 Got error decoding msg: %v", err)
				return
			}
			// glog.Infof("Got msg: %v", msg)
			n2chan <- msg.Data.(StreamDataMsg).StrmID
		}
	})

	n4chan := make(chan string)
	n4.PeerHost.SetStreamHandler(Protocol, func(s net.Stream) {
		defer s.Close()
		ws := NewBasicInStream(s)
		for {
			msg, err := ws.ReceiveMessage()
			if err != nil {
				glog.Errorf("N4 Got error decoding msg: %v", err)
				return
			}
			glog.Infof("Got msg: %v", msg)
			// t.Errorf("Shouldn't be here...")
			n4chan <- msg.Data.(SubReqMsg).StrmID
		}
	})

	//Test when the broadcaster is local (n2 should get a stream data back because n1 sends the last msg immediately)
	strmID := fmt.Sprintf("%vStrmID", n1.GetNodeID())
	b1tmp, _ := n1.GetBroadcaster(strmID)
	b1, _ := b1tmp.(*BasicBroadcaster)
	b1.lastMsgs = []*StreamDataMsg{&StreamDataMsg{SeqNo: 0, StrmID: strmID, Data: []byte("hello")}}
	n1.broadcasters[strmID] = b1
	ws := n1.NetworkNode.GetOutStream(n2.Identity)
	if err := handleSubReq(n1, SubReqMsg{StrmID: strmID}, n2.Identity); err != nil {
		t.Errorf("Error handling sub req: %v", err)
	}

	l := b1.listeners[peer.IDHexEncode(n2.Identity)]
	if l == nil || reflect.TypeOf(l) != reflect.TypeOf(&BasicOutStream{}) {
		t.Errorf("Expecting l to be assigned a BasicOutStream, but got :%v", reflect.TypeOf(l))
	}

	timer := time.NewTimer(time.Second)
	select {
	case n2ID := <-n2chan:
		if n2ID != strmID {
			t.Errorf("Expecting %v, got %v", strmID, n2ID)
		}
	case <-timer.C:
		t.Errorf("Timed out")
	}
	delete(n1.broadcasters, strmID)

	//Test relaying
	strmID2 := fmt.Sprintf("%vStrmID2", peer.IDHexEncode(n4.Identity))
	if len(n1.relayers) != 0 {
		t.Errorf("Should have 0 relayer")
	}
	r1 := n1.NewRelayer(strmID2, SubReqID)
	if n1.relayers[relayerMapKey(strmID2, SubReqID)] != r1 {
		t.Errorf("Should have assigned relayer")
	}
	ws = n1.NetworkNode.GetOutStream(n2.Identity)
	if err := handleSubReq(n1, SubReqMsg{StrmID: strmID2}, n2.Identity); err != nil {
		t.Errorf("Error handling sub req: %v", err)
	}
	pid := peer.IDHexEncode(ws.Stream.Conn().RemotePeer())
	if r1.listeners[pid] != ws {
		t.Errorf("Should have assigned listener to relayer")
	}
	timer = time.NewTimer(time.Millisecond * 100)
	select {
	case <-n4chan:
		t.Errorf("Should not recieve a message on n4chan - there is already a relayer, we just add as a listener")
	case <-timer.C:
		//This is good
	}
	delete(n1.relayers, relayerMapKey(strmID2, SubReqID))

	//Test when the broadcaster is remote, and there isn't a relayer yet.
	//TODO: This is hard to test because of the dependency to kad.IpfsDht.  We can get around it by creating an interface called "NetworkRouting"
	// handleSubReq(n1, SubReqMsg{StrmID: "strmID"}, ws)

}

func simpleRelayHandler(ws *BasicInStream, t *testing.T) Msg {
	msg, err := ws.ReceiveMessage()
	if err != nil {
		// glog.Errorf("Got error decoding msg: %v", err)
		return Msg{}
	}
	return msg
}
func TestRelaying(t *testing.T) {
	n1, n2 := setupNodes(t, 15000, 15001)
	n3, n4 := simpleNodes(15002, 15003)
	defer n1.NetworkNode.(*BasicNetworkNode).PeerHost.Close()
	defer n2.NetworkNode.(*BasicNetworkNode).PeerHost.Close()
	defer n3.PeerHost.Close()
	defer n4.PeerHost.Close()

	connectHosts(n1.NetworkNode.(*BasicNetworkNode).PeerHost, n2.NetworkNode.(*BasicNetworkNode).PeerHost)
	connectHosts(n2.NetworkNode.(*BasicNetworkNode).PeerHost, n3.PeerHost)

	strmID := peer.IDHexEncode(n1.NetworkNode.ID()) + "strmID"
	b1, _ := n1.GetBroadcaster(strmID)

	//Send Sub message from n3 to n1 (should relay through n2)
	s3 := n3.GetOutStream(n2.NetworkNode.ID())
	s3.SendMessage(SubReqID, SubReqMsg{StrmID: strmID})

	var strmDataResult StreamDataMsg
	var finishResult FinishStreamMsg
	var ok bool
	n3.PeerHost.SetStreamHandler(Protocol, func(s net.Stream) {
		strm := NewBasicInStream(s)
		for {
			// glog.Infof("Got msg: %v", msg)
			msg := simpleRelayHandler(strm, t)
			switch msg.Data.(type) {
			case StreamDataMsg:
				strmDataResult, ok = msg.Data.(StreamDataMsg)
				if !ok {
					t.Errorf("Expecting stream data to come back")
				}
			case FinishStreamMsg:
				finishResult, ok = msg.Data.(FinishStreamMsg)
				if !ok {
					t.Errorf("Expecting finish stream to come back")
				}
			}
		}
	})

	time.Sleep(time.Second * 1)
	err := b1.Broadcast(100, []byte("test data"))
	if err != nil {
		t.Errorf("Error broadcasting: %v", err)
	}

	start := time.Now()
	for time.Since(start) < time.Second*5 {
		if strmDataResult.SeqNo == 0 {
			time.Sleep(100 * time.Millisecond)
		} else {
			break
		}
	}

	if string(strmDataResult.Data) != "test data" {
		t.Errorf("Expecting 'test data', got %v", strmDataResult.Data)
	}

	if len(n1.broadcasters) != 1 {
		t.Errorf("Should be 1 broadcaster in n1")
	}

	if len(n1.broadcasters[strmID].listeners) != 1 {
		t.Errorf("Should be 1 listener in b1")
	}

	if len(n2.relayers) != 1 {
		t.Errorf("Should be 1 relayer in n2")
	}

	if len(n2.relayers[relayerMapKey(strmID, SubReqID)].listeners) != 1 {
		t.Errorf("Should be 1 listener in r2")
	}

	err = b1.Finish()
	// n1.DeleteBroadcaster(strmID)
	if err != nil {
		t.Errorf("Error when broadcasting Finish: %v", err)
	}

	//Wait for finish msg in n3
	start = time.Now()
	for time.Since(start) < time.Second*5 {
		if finishResult.StrmID == "" {
			time.Sleep(100 * time.Millisecond)
		} else {
			break
		}
	}

	if finishResult.StrmID != strmID {
		t.Errorf("Expecting finishResult to have strmID: %v, but got %v", strmID, finishResult)
	}

	if len(n1.broadcasters) != 0 {
		t.Errorf("Should have 0 broadcasters in n1")
	}

	if len(n2.relayers) != 0 {
		t.Errorf("Should have 0 relayers in n2, but got %v", n2.relayers)
	}
}

func TestSendTranscodeResponse(t *testing.T) {
	glog.Infof("\n\nTesting Handle Transcode Result...")
	//n1 -> n2 -> n3, n3 should get the result, n2 should relay it, n1 should send it.
	n1, n2 := setupNodes(t, 15000, 15001)
	n3, _ := simpleNodes(15003, 15004)
	defer n1.NetworkNode.(*BasicNetworkNode).PeerHost.Close()
	defer n2.NetworkNode.(*BasicNetworkNode).PeerHost.Close()
	defer n3.PeerHost.Close()
	connectHosts(n1.NetworkNode.(*BasicNetworkNode).PeerHost, n2.NetworkNode.(*BasicNetworkNode).PeerHost)
	connectHosts(n2.NetworkNode.(*BasicNetworkNode).PeerHost, n3.PeerHost)

	//Set up n3 to capture the message
	rc := make(chan map[string]string)
	n3.PeerHost.SetStreamHandler(Protocol, func(s net.Stream) {
		defer s.Close()
		ws := NewBasicInStream(s)
		for {
			msg, err := ws.ReceiveMessage()
			if err != nil {
				glog.Infof("Error: %v", err)
				break
			}
			glog.Infof("n3 got Msg: %v", msg)
			rc <- msg.Data.(TranscodeResponseMsg).Result
		}
	})

	strmID := fmt.Sprintf("%vstrmID", peer.IDHexEncode(n3.Identity))
	//Send the message
	go func() {
		for i := 0; i < 3; i++ {
			err := n1.SendTranscodeResponse(peer.IDHexEncode(n3.Identity), fmt.Sprintf("%v:%v", strmID, i), map[string]string{"strmid1": "P240p30fps4x3", "strmid2": "P360p30fps4x3"})
			if err != nil {
				t.Errorf("Error sending transcode result: %v", err)
			}
		}
	}()

	for i := 0; i < 3; i++ {
		select {
		case r := <-rc:
			if r["strmid1"] != "P240p30fps4x3" {
				t.Errorf("Expecting %v, got %v", "P240p30fps4x3", r["strmid1"])
			}
			if r["strmid2"] != "P360p30fps4x3" {
				t.Errorf("Expecting %v, got %v", "P360p30fps4x3", r["strmid2"])
			}
		case <-time.After(time.Second * 5):
			t.Errorf("Timed out")
		}
	}

	// r, ok := n2.relayers[relayerMapKey(strmID, TranscodeResponseID)]
	// if !ok {
	// 	glog.Infof("n2 should have created a relayer")
	// }
	// if _, ok := r.listeners[peer.IDHexEncode(n3.Identity)]; !ok {
	// 	glog.Infof("relayer should have 1 listener, but got: %v", r.listeners[peer.IDHexEncode(n3.Identity)])
	// }
}
func TestHandleGetMasterPlaylist(t *testing.T) {
	n1, n2 := setupNodes(t, 15000, 15001)
	n3, n4 := simpleNodes(15003, 15004)
	connectHosts(n1.NetworkNode.(*BasicNetworkNode).PeerHost, n3.PeerHost)
	defer n1.NetworkNode.(*BasicNetworkNode).PeerHost.Close()
	defer n2.NetworkNode.(*BasicNetworkNode).PeerHost.Close()
	defer n3.PeerHost.Close()
	defer n4.PeerHost.Close()
	n3Chan := make(chan MasterPlaylistDataMsg)
	n3.PeerHost.SetStreamHandler(Protocol, func(s net.Stream) {
		defer s.Reset()
		strm := NewBasicInStream(s)
		for {
			msg, err := strm.ReceiveMessage()
			if err != nil {
				break
			}
			switch msg.Data.(type) {
			case MasterPlaylistDataMsg:
				n3Chan <- msg.Data.(MasterPlaylistDataMsg)
			default:
			}
		}
	})

	glog.Infof("Case 1...")
	//Set up n1 without the playlist and with no other peer.  Should send a NotFound.
	strmID := fmt.Sprintf("%vstrmID", peer.IDHexEncode(n1.NetworkNode.ID()))
	_, ok := n1.mplMap[strmID]
	if ok {
		t.Errorf("Expecting to not have the playlist")
	}
	// strm := n1.NetworkNode.GetStream(n3.Identity)
	if err := handleGetMasterPlaylistReq(n1, n3.Identity, GetMasterPlaylistReqMsg{ManifestID: strmID}); err != nil {
		t.Errorf("Error: %v", err)
	}
	timer := time.NewTimer(time.Second)
	select {
	case n3data := <-n3Chan:
		if n3data.NotFound == false {
			t.Errorf("Expecting NotFound, but got: %v", n3data)
		}
	case <-timer.C:
		t.Errorf("timed out")
	}
	n1.NetworkNode.(*BasicNetworkNode).outStreams[n3.ID()].Stream.Reset()
	delete(n1.NetworkNode.(*BasicNetworkNode).outStreams, n3.Identity)

	glog.Infof("Case 2...")
	//Relay req from n3 to n2 (through n1).  Set up n2 to have a playlist.  n3 should recieve the playlist.
	connectHosts(n1.NetworkNode.(*BasicNetworkNode).PeerHost, n2.NetworkNode.(*BasicNetworkNode).PeerHost)
	strmID = fmt.Sprintf("%vstrmID", peer.IDHexEncode(n2.NetworkNode.ID()))
	pl := m3u8.NewMasterPlaylist()
	pl.Append("testurl", nil, m3u8.VariantParams{Bandwidth: 100})
	n2.mplMap[strmID] = pl
	if err := handleGetMasterPlaylistReq(n1, n3.Identity, GetMasterPlaylistReqMsg{ManifestID: strmID}); err != nil {
		t.Errorf("Error: %v", err)
	}

	timer = time.NewTimer(time.Second)
	select {
	case n3data := <-n3Chan:
		if n3data.MPL == "" {
			t.Errorf("Expecting n3 to receive playlist, but got %v", n3data)
		}
	case <-timer.C:
		t.Errorf("timed out")
	}
	if len(n1.relayers) != 1 {
		t.Errorf("Expecting 1 relayer, got %v", n2.relayers)
	}
	if len(n1.relayers[relayerMapKey(strmID, GetMasterPlaylistReqID)].listeners) != 1 {
		t.Errorf("Expecting 1 listener, got %v", n1.relayers[relayerMapKey(strmID, GetMasterPlaylistReqID)].listeners)
	}

	//Send another req for the same stream, make sure the relayer listener increased
	glog.Infof("Case 3...")
	n4Chan := make(chan struct{})
	n4.PeerHost.SetStreamHandler(Protocol, func(s net.Stream) {
		n4Chan <- struct{}{}
	})
	connectHosts(n1.NetworkNode.(*BasicNetworkNode).PeerHost, n4.PeerHost)
	if err := handleGetMasterPlaylistReq(n1, n4.Identity, GetMasterPlaylistReqMsg{ManifestID: strmID}); err != nil {
		t.Errorf("Error: %v", err)
	}
	timer = time.NewTimer(time.Second)
	select {
	case <-n4Chan:
	case <-timer.C:
		t.Errorf("timed out")
	}
	if len(n1.relayers) != 1 {
		t.Errorf("Expecting 1 relayer, got %v", n2.relayers)
	}
	if len(n1.relayers[relayerMapKey(strmID, GetMasterPlaylistReqID)].listeners) != 2 {
		t.Errorf("Expecting 2 listeners, got %v", n1.relayers[relayerMapKey(strmID, GetMasterPlaylistReqID)].listeners)
	}
}

func TestHandleMasterPlaylistData(t *testing.T) {
	n1, n2 := setupNodes(t, 15000, 15001)
	n3, n4 := simpleNodes(15003, 15004)
	connectHosts(n1.NetworkNode.(*BasicNetworkNode).PeerHost, n3.PeerHost)
	defer n1.NetworkNode.(*BasicNetworkNode).PeerHost.Close()
	defer n2.NetworkNode.(*BasicNetworkNode).PeerHost.Close()
	defer n3.PeerHost.Close()
	defer n4.PeerHost.Close()

	//Set up no relayer and no receiving playlist channel. Should get error
	manifestID := fmt.Sprintf("%vstrmID", peer.IDHexEncode(n1.NetworkNode.ID()))
	_, ok := n1.mplMap[manifestID]
	if ok {
		t.Errorf("Expecting to not have the playlist")
	}
	err := handleMasterPlaylistDataMsg(n1, MasterPlaylistDataMsg{ManifestID: manifestID, NotFound: true})
	if err != ErrHandleMsg {
		t.Errorf("Expecting ErrHandleMsg, got: %v", err)
	}

	//Set up a relayer, make sure it's relaying to the right destination
	n3Chan := make(chan MasterPlaylistDataMsg)
	n3.PeerHost.SetStreamHandler(Protocol, func(s net.Stream) {
		strm := NewBasicInStream(s)
		msg, _ := strm.ReceiveMessage()
		n3Chan <- msg.Data.(MasterPlaylistDataMsg)
	})
	strm := n1.NetworkNode.GetOutStream(n3.Identity)
	r := &BasicRelayer{listeners: map[string]*BasicOutStream{peer.IDHexEncode(n3.Identity): strm}}
	n1.relayers[relayerMapKey(manifestID, GetMasterPlaylistReqID)] = r
	if err := handleMasterPlaylistDataMsg(n1, MasterPlaylistDataMsg{ManifestID: manifestID, NotFound: true}); err != nil {
		t.Errorf("Error: %v", err)
	}
	timer := time.NewTimer(time.Second)
	select {
	case n3data := <-n3Chan:
		if n3data.ManifestID != manifestID {
			t.Errorf("Expecting %v, got %v", manifestID, n3data.ManifestID)
		}
	case <-timer.C:
		t.Errorf("Timed out")
	}
	delete(n1.relayers, relayerMapKey(manifestID, GetMasterPlaylistReqID))

	//No relayer and NotFound.  Should insert 'nil' into the channel.
	msgc := make(chan *Msg)
	n1.msgChans[msgChansKey(GetMasterPlaylistReqID, manifestID)] = msgc
	//handle in a go routine because we expect something on the channel
	go func() {
		if err := handleMasterPlaylistDataMsg(n1, MasterPlaylistDataMsg{ManifestID: manifestID, NotFound: true}); err != nil {
			t.Errorf("Error: %v", err)
		}
	}()
	timer = time.NewTimer(time.Second)
	select {
	case msg := <-msgc:
		if msg.Data.(MasterPlaylistDataMsg).NotFound == false {
			t.Errorf("Expecting NotFound to be true")
		}
	case <-timer.C:
		t.Errorf("Timed out")
	}

	//No relayer and have an actual playlist.  Should get the playlist.
	msgc = make(chan *Msg)
	n1.msgChans[msgChansKey(GetMasterPlaylistReqID, manifestID)] = msgc

	pl := m3u8.NewMasterPlaylist()
	pl.Append("someurl", nil, m3u8.VariantParams{Bandwidth: 100})
	//handle in a go routine because we expect something on the channel
	go func() {
		if err := handleMasterPlaylistDataMsg(n1, MasterPlaylistDataMsg{ManifestID: manifestID, MPL: pl.String()}); err != nil {
			t.Errorf("Error: %v", err)
		}
	}()
	timer = time.NewTimer(time.Second)
	select {
	case msg := <-msgc:
		mplstr := msg.Data.(MasterPlaylistDataMsg).MPL
		if mplstr != pl.String() {
			t.Errorf("Expecting %v, got %v", pl, mplstr)
		}
	case <-timer.C:
		t.Errorf("Timed out")
	}
}

func TestMasterPlaylistIntegration(t *testing.T) {
	glog.Infof("\n\nTesting handle master playlist")
	n1, n2 := setupNodes(t, 15000, 15001)
	n3, n4 := setupNodes(t, 15002, 15003)
	defer n1.NetworkNode.(*BasicNetworkNode).PeerHost.Close()
	defer n2.NetworkNode.(*BasicNetworkNode).PeerHost.Close()
	defer n3.NetworkNode.(*BasicNetworkNode).PeerHost.Close()
	defer n4.NetworkNode.(*BasicNetworkNode).PeerHost.Close()

	connectHosts(n1.NetworkNode.(*BasicNetworkNode).PeerHost, n2.NetworkNode.(*BasicNetworkNode).PeerHost)

	//Create Playlist
	mpl := m3u8.NewMasterPlaylist()
	pl, _ := m3u8.NewMediaPlaylist(10, 10)
	mpl.Append("test.m3u8", pl, m3u8.VariantParams{Bandwidth: 100000})
	strmID := fmt.Sprintf("%vba1637fd2531f50f9e8f99a37b48d7cfe12fa498ff6da8d6b63279b4632101d5e8b1c872c", peer.IDHexEncode(n2.NetworkNode.ID()))

	//n2 Updates Playlist
	if err := n2.UpdateMasterPlaylist(strmID, mpl); err != nil {
		t.Errorf("Error updating master playlist")
	}

	//n1 Gets Playlist
	mplc, err := n1.GetMasterPlaylist(n2.GetNodeID(), strmID)
	if err != nil {
		t.Errorf("Error getting master playlist: %v", err)
	}
	select {
	case r := <-mplc:
		vars := r.Variants
		if len(vars) != 1 {
			t.Errorf("Expecting 1 variants, but got: %v - %v", len(vars), r)
		}
	case <-time.After(time.Second * 3):
		glog.Infof("n2 mplMap: %v", n2.mplMap)
		t.Errorf("Timed out")
	}

	//Add a new node in the network
	connectHosts(n2.NetworkNode.(*BasicNetworkNode).PeerHost, n3.NetworkNode.(*BasicNetworkNode).PeerHost)
	go n3.SetupProtocol()

	//Create a playlist on n3, make sure n2 is relaying and n1 can still get the playlist
	mpl = m3u8.NewMasterPlaylist()
	pl, _ = m3u8.NewMediaPlaylist(10, 10)
	mpl.Append("test3.m3u8", pl, m3u8.VariantParams{Bandwidth: 100000})
	strmID = fmt.Sprintf("%vba1637fd2531f50f9e8f99a37b48d7cfe12fa498ff6da8d6b63279b4632101d5e8b1c872f", peer.IDHexEncode(n3.NetworkNode.ID()))
	if err := n3.UpdateMasterPlaylist(strmID, mpl); err != nil {
		t.Errorf("Error updating master playlist: %v", err)
	}

	//Get Playlist should still work
	mplc, err = n1.GetMasterPlaylist("", strmID)
	if err != nil {
		t.Errorf("Error getting master playlist: %v", err)
	}
	select {
	case r := <-mplc:
		vars := r.Variants
		if len(vars) != 1 {
			t.Errorf("Expecting 1 variants, but got: %v - %v", len(vars), r)
		}
		if r.Variants[0].URI != "test3.m3u8" {
			t.Errorf("Expecting test3.m3u8, got %v", r.Variants[0].URI)
		}
		if len(n2.relayers) != 1 {
			t.Errorf("Expecting 1 relayer in n2")
		}
	case <-time.After(time.Second * 5):
		t.Errorf("Timed out")
	}
}

func TestID(t *testing.T) {
	n1, _ := simpleNodes(15002, 15003)
	id := n1.Identity
	sid := peer.IDHexEncode(id)
	pid, err := peer.IDHexDecode(sid)
	if err != nil {
		t.Errorf("Error decoding id %v: %v", pid, err)
	}
}

func TestNodeStatus(t *testing.T) {
	glog.Infof("\n\nTesting node status")
	//Get status from local node

	//Shouldn't find a manifest
	n1, n2 := setupNodes(t, 15000, 15001)
	n3, _ := setupNodes(t, 15002, 15003)
	connectHosts(n1.NetworkNode.(*BasicNetworkNode).PeerHost, n2.NetworkNode.(*BasicNetworkNode).PeerHost)
	connectHosts(n2.NetworkNode.(*BasicNetworkNode).PeerHost, n3.NetworkNode.(*BasicNetworkNode).PeerHost)
	go n1.SetupProtocol()
	go n2.SetupProtocol()
	go n3.SetupProtocol()

	sc, err := n1.GetNodeStatus(n1.GetNodeID())
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	status := <-sc
	if len(status.Manifests) != 0 {
		t.Errorf("Expecting no manifests, but got %v", status.Manifests)
	}

	//Add a manifest
	mpl := m3u8.NewMasterPlaylist()
	pl, _ := m3u8.NewMediaPlaylist(10, 10)
	mpl.Append("test.m3u8", pl, m3u8.VariantParams{Bandwidth: 100000})
	n1.UpdateMasterPlaylist("testStrm", mpl)

	//Request from self
	sc, err = n1.GetNodeStatus(n1.GetNodeID())
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	status = <-sc
	if len(status.Manifests) != 1 {
		t.Errorf("Expecting 1 manifest, but got %v", status.Manifests)
	}

	//Request from neighbor
	sc, err = n2.GetNodeStatus(n1.GetNodeID())
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	status = <-sc
	if len(status.Manifests) != 1 {
		t.Errorf("Expecting 1 manifest, but got %v", status.Manifests)
	}

	//Request from neighbor's neighbor (need relaying)
	sc, err = n3.GetNodeStatus(n1.GetNodeID())
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	status = <-sc
	if len(status.Manifests) != 1 {
		t.Errorf("Expecting 1 manifest, but got %v", status.Manifests)
	}
}
