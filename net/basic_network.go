package net

import (
	"container/list"
	"context"
	"log"
	"sync"

	"time"

	"github.com/golang/glog"
	crypto "github.com/libp2p/go-libp2p-crypto"
	host "github.com/libp2p/go-libp2p-host"
	net "github.com/libp2p/go-libp2p-net"
	peer "github.com/libp2p/go-libp2p-peer"
	peerstore "github.com/libp2p/go-libp2p-peerstore"
	protocol "github.com/libp2p/go-libp2p-protocol"
	ma "github.com/multiformats/go-multiaddr"
)

var Protocol = protocol.ID("/livepeer_video/0.0.1")

// type VideoMsg struct {
// 	SeqNo uint64
// 	Data  []byte
// }

//BasicVideoNetwork creates a kademlia network using libp2p.  It does push-based video delivery, and handles the protocol in the background.
type BasicVideoNetwork struct {
	NetworkNode  *NetworkNode
	broadcasters map[string]*BasicBroadcaster
	subscribers  map[string]*BasicSubscriber
}

//BasicBroadcaster keeps track of a list of listeners and a queue of video chunks.  It doesn't start keeping track of things until there is at least 1 listner.
type BasicBroadcaster struct {
	host      host.Host
	q         *list.List
	lock      *sync.Mutex
	listeners map[string]peerstore.PeerInfo
	StrmID    string
}

type BasicSubscriber struct {
	Network *BasicVideoNetwork
	host    host.Host
	StrmID  string
}

//NewBasicNetwork creates a libp2p node, handle the basic (push-based) video protocol.
func NewBasicNetwork(port int, priv crypto.PrivKey, pub crypto.PubKey) (*BasicVideoNetwork, error) {
	n, err := NewNode(port, priv, pub)
	if err != nil {
		glog.Errorf("Error creating a new node: %v", err)
		return nil, err
	}

	nw := &BasicVideoNetwork{NetworkNode: n, broadcasters: make(map[string]*BasicBroadcaster), subscribers: make(map[string]*BasicSubscriber)}
	if err = nw.setupProtocol(n); err != nil {
		glog.Errorf("Error setting up video protocol: %v", err)
		return nil, err
	}

	return nw, nil
}

func (n *BasicVideoNetwork) NewBroadcaster(strmID string) *BasicBroadcaster {
	b := &BasicBroadcaster{StrmID: strmID, q: list.New(), host: n.NetworkNode.PeerHost, lock: &sync.Mutex{}, listeners: make(map[string]peerstore.PeerInfo)}
	n.broadcasters[strmID] = b
	return b
}

func (n *BasicVideoNetwork) NewSubscriber(strmID string) *BasicSubscriber {
	s := &BasicSubscriber{StrmID: strmID, host: n.NetworkNode.PeerHost}
	n.subscribers[strmID] = s
	return s
}

//Broadcast sends a video chunk to the stream
func (b *BasicBroadcaster) Broadcast(seqNo uint64, data []byte) error {
	b.q.PushBack(&StreamDataMsg{SeqNo: seqNo, Data: data})
	// b.q = append(b.q, &VideoMsg{SeqNo: seqNo, Data: data})
	return nil
}

//Finish signals the stream is finished
func (b *BasicBroadcaster) Finish() error {
	return nil
}

func (b *BasicBroadcaster) broadcastToNetwork(ctx context.Context, n *NetworkNode) {
	go func() {
		for {
			b.lock.Lock()
			e := b.q.Front()
			if e != nil {
				b.q.Remove(e)
			}
			b.lock.Unlock()

			if e == nil {
				time.Sleep(time.Millisecond * 100)
				continue
			}

			msg, ok := e.Value.(*StreamDataMsg)
			if !ok {
				glog.Errorf("Cannot convert video msg during broadcast: %v", e.Value)
				continue
			}

			glog.Infof("broadcasting msg:%v to network.  listeners: %v", msg, b.listeners)
			for _, p := range b.listeners {
				glog.Infof("Peerstore entry: for %v: %v", p.ID.Pretty(), n.Peerstore.Addrs(p.ID))
				if n.PeerHost.Network().Connectedness(p.ID) != net.Connected {
					// p.ID
					glog.Infof("%v Creating new connection to :%v", n.PeerHost.ID().Pretty(), p.ID.Pretty())
					n.Peerstore.AddAddrs(p.ID, p.Addrs, peerstore.PermanentAddrTTL)
					// err := n.PeerHost.Connect(context.Background(), p)
					// if err != nil {
					// 	glog.Errorf("Cannot create connection: %v", err)
					// }
				}

				s, err := n.PeerHost.NewStream(context.Background(), p.ID, Protocol)
				if err != nil {
					log.Fatal(err)
				}

				n.SendMessage(s, p.ID, StreamDataID, StreamDataMsg{SeqNo: 0, StrmID: b.StrmID, Data: msg.Data})
			}
		}
	}()
	select {
	case <-ctx.Done():
		return
	}
}

//Subscribe kicks off a go routine that calls the gotData func for every new video chunk
func (s *BasicSubscriber) Subscribe(ctx context.Context, gotData func(seqNo uint64, data []byte)) error {
	//Send Subscribe Request
	for {
	}
	return nil
}

//Unsubscribe unsubscribes from the broadcast
func (s *BasicSubscriber) Unsubscribe() error {
	return nil
}

func (nw *BasicVideoNetwork) setupProtocol(node *NetworkNode) error {
	node.PeerHost.SetStreamHandler(Protocol, func(stream net.Stream) {
		glog.Infof("%s: Received a stream", node.PeerHost.ID().Pretty())
		wrappedStream := WrapStream(stream)
		defer stream.Close()
		nw.handleProtocol(wrappedStream)
	})

	return nil
}

func (nw *BasicVideoNetwork) handleProtocol(ws *WrappedStream) {
	var msg Msg
	err := ws.dec.Decode(&msg)

	if err != nil {
		glog.Errorf("Got error decoding msg: %v", err)
		return
	}

	//Video Protocol:
	//	- StreamData
	//	- FinishStream
	//	- SubReq
	//	- CancelSub

	//Livepeer Protocol:
	//	- TranscodeInfo
	//	- TranscodeInfoAck (TranscodeInfo will re-send until getting an Ack)
	switch msg.Op {
	case SubReqID:
		sr, ok := msg.Data.(SubReqMsg)
		if !ok {
			glog.Errorf("Cannot convert SubReqMsg: %v", msg.Data)
		}
		glog.Infof("Got Sub Req: %v", sr)
		nw.handleSubReq(sr)
	case StreamDataID:
		glog.Infof("Got Stream Data: %v", msg.Data)
		//Enque it into the subscriber
	default:
		glog.Infof("Data: %v", msg)
	}
}

func (nw *BasicVideoNetwork) handleSubReq(subReq SubReqMsg) {
	b := nw.broadcasters[subReq.StrmID]
	if b == nil {
		glog.Infof("Cannot find broadcast for stream: %v", subReq.StrmID)
		return
	}

	//If not in peerstore, add it.
	p := b.listeners[subReq.StrmID]
	if p.ID == "" {
		addr, err := ma.NewMultiaddr(subReq.SubNodeAddr)
		if err != nil {
			glog.Errorf("Bad addr in subReq: %v", subReq)
		}
		id, err := peer.IDHexDecode(subReq.SubNodeID)
		if err != nil {
			glog.Errorf("Bad peer id in subReq: %v", subReq)
		}
		p = peerstore.PeerInfo{ID: id, Addrs: []ma.Multiaddr{addr}}
		b.listeners[subReq.StrmID] = p
	}

	b.broadcastToNetwork(context.Background(), nw.NetworkNode)
	return

}
