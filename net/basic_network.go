package net

import (
	"container/list"
	"context"
	"errors"
	"sync"

	"time"

	"github.com/golang/glog"
	crypto "github.com/libp2p/go-libp2p-crypto"
	host "github.com/libp2p/go-libp2p-host"
	net "github.com/libp2p/go-libp2p-net"
	peer "github.com/libp2p/go-libp2p-peer"
	protocol "github.com/libp2p/go-libp2p-protocol"
)

/**
The basic network is a push-based streaming protocol.  It works as follow:
	- When a video is broadcasted, it's stored at a local broadcaster
	- When a viewer wants to view a video, it sends a subscribe request to the network
	- The network routes the request towards the broadcast node via kademlia routing
	-
**/

var Protocol = protocol.ID("/livepeer_video/0.0.1")
var ErrNoClosePeers = errors.New("NoClosePeers")
var ErrUnknownMsg = errors.New("UnknownMsgType")
var ErrProtocol = errors.New("ProtocolError")

type VideoMuxer interface {
	WriteSegment(seqNo uint64, data []byte)
}

//BasicVideoNetwork creates a kademlia network using libp2p.  It does push-based video delivery, and handles the protocol in the background.
type BasicVideoNetwork struct {
	NetworkNode  *NetworkNode
	broadcasters map[string]*BasicBroadcaster
	subscribers  map[string]*BasicSubscriber

	// streams           map[string]*stream.VideoStream
	// streamSubscribers map[string]*stream.StreamSubscriber
	// cancellation      map[string]context.CancelFunc
}

//BasicBroadcaster keeps track of a list of listeners and a queue of video chunks.  It doesn't start keeping track of things until there is at least 1 listner.
type BasicBroadcaster struct {
	Network *BasicVideoNetwork
	// host    host.Host
	q    *list.List
	lock *sync.Mutex

	// listeners    map[string]*WrappedStream
	listeners    map[string]VideoMuxer
	StrmID       string
	working      bool
	cancelWorker context.CancelFunc
}

//BasicSubscriber keeps track of
type BasicSubscriber struct {
	Network       *BasicVideoNetwork
	host          host.Host
	msgChan       chan StreamDataMsg
	networkStream net.Stream
	// q       *list.List
	// lock    *sync.Mutex
	StrmID       string
	cancelWorker context.CancelFunc

	// listeners map[string]net.Stream
}

//NewBasicNetwork creates a libp2p node, handle the basic (push-based) video protocol.
func NewBasicNetwork(port int, priv crypto.PrivKey, pub crypto.PubKey) (*BasicVideoNetwork, error) {
	n, err := NewNode(port, priv, pub)
	if err != nil {
		glog.Errorf("Error creating a new node: %v", err)
		return nil, err
	}

	nw := &BasicVideoNetwork{NetworkNode: n, broadcasters: make(map[string]*BasicBroadcaster), subscribers: make(map[string]*BasicSubscriber)}
	// if err = nw.setupProtocol(n); err != nil {
	// 	glog.Errorf("Error setting up video protocol: %v", err)
	// 	return nil, err
	// }

	return nw, nil
}

func (n *BasicVideoNetwork) NewBroadcaster(strmID string) *BasicBroadcaster {
	// b := &BasicBroadcaster{Network: n, StrmID: strmID, q: list.New(), host: n.NetworkNode.PeerHost, lock: &sync.Mutex{}, listeners: make(map[string]peerstore.PeerInfo)}
	b := &BasicBroadcaster{Network: n, StrmID: strmID, q: list.New(), lock: &sync.Mutex{}, listeners: make(map[string]VideoMuxer)}
	n.broadcasters[strmID] = b
	return b
}

func (n *BasicVideoNetwork) NewSubscriber(strmID string) *BasicSubscriber {
	s := &BasicSubscriber{Network: n, StrmID: strmID, host: n.NetworkNode.PeerHost, msgChan: make(chan StreamDataMsg)}
	n.subscribers[strmID] = s
	return s
}

//Broadcast sends a video chunk to the stream
func (b *BasicBroadcaster) Broadcast(seqNo uint64, data []byte) error {
	b.q.PushBack(&StreamDataMsg{SeqNo: seqNo, Data: data})

	//This should only get invoked once per broadcaster
	if b.working == false {
		ctxB, cancel := context.WithCancel(context.Background())
		b.cancelWorker = cancel
		go b.broadcastToListeners(ctxB)
		b.working = true
	}
	return nil
}

//Finish signals the stream is finished
func (b *BasicBroadcaster) Finish() error {
	b.cancelWorker()
	//TODO: Need to figure out a place to close the stream listners
	// for _, l := range b.listeners {
	// 	l.Stream.Close()
	// }
	return nil
}

func (b *BasicBroadcaster) broadcastToListeners(ctx context.Context) {
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
			for _, l := range b.listeners {
				// glog.Infof("Peerstore entry: for %v: %v", p.ID.Pretty(), n.Peerstore.Addrs(p.ID))
				// if n.PeerHost.Network().Connectedness(p.ID) != net.Connected {
				// 	// p.ID
				// 	glog.Infof("%v Creating new connection to :%v", n.PeerHost.ID().Pretty(), p.ID.Pretty())
				// 	n.Peerstore.AddAddrs(p.ID, p.Addrs, peerstore.PermanentAddrTTL)
				// 	// err := n.PeerHost.Connect(context.Background(), p)
				// 	// if err != nil {
				// 	// 	glog.Errorf("Cannot create connection: %v", err)
				// 	// }
				// }

				// s, err := n.PeerHost.NewStream(context.Background(), p.ID, Protocol)
				// if err != nil {
				// 	log.Fatal(err)
				// }

				// n.SendMessage(l, p.ID, StreamDataID, StreamDataMsg{SeqNo: 0, StrmID: b.StrmID, Data: msg.Data})
				l.WriteSegment(msg.SeqNo, msg.Data)

				//This logic should be moved to WrappedStream.WriteSegment()
				// nwMsg := Msg{Op: StreamDataID, Data: StreamDataMsg{SeqNo: msg.SeqNo, StrmID: b.StrmID, Data: msg.Data}}
				// glog.Infof("Sending: %v to %v", nwMsg, l.Stream.Conn().RemotePeer().Pretty())

				// err := l.Enc.Encode(nwMsg)
				// if err != nil {
				// 	glog.Errorf("send message encode error: %v", err)
				// 	delete(b.listeners, id)
				// }

				// err = l.W.Flush()
				// if err != nil {
				// 	glog.Errorf("send message flush error: %v", err)
				// 	delete(b.listeners, id)
				// }
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
	//Do we already have the broadcaster locally?
	b := s.Network.broadcasters[s.StrmID]

	//If we do, just subscribe to it and listen.
	if b != nil {
		glog.Infof("Broadcaster is present - let's just read from that...")
		//TODO: read from broadcaster
		return nil
	}

	//If we don't, send subscribe request, listen for response
	peerc, err := s.Network.NetworkNode.Kad.GetClosestPeers(ctx, s.StrmID)
	if err != nil {
		glog.Errorf("Network Subscribe Error: %v", err)
		return err
	}

	//We can range over peerc because we know it'll be closed by libp2p
	//We'll keep track of all the connections on the
	peers := make([]peer.ID, 0)
	for p := range peerc {
		peers = append(peers, p)
	}

	//Send SubReq to one of the peers
	if len(peers) > 0 {
		for _, p := range peers {
			//Question: Where do we close the stream? If we only close on "Unsubscribe", we may leave some streams open...
			ns := s.Network.NetworkNode.GetStream(p)
			if ns != nil {
				//Set up handler for the stream
				go func() {
					for {
						err := streamHandler(s.Network, ns)
						if err != nil {
							glog.Errorf("Got error handling stream: %v", err)
							return
						}
					}
				}()

				//Send SubReq
				s.Network.NetworkNode.SendMessage(ns, p, SubReqID, SubReqMsg{StrmID: s.StrmID})
				ctxW, cancel := context.WithCancel(context.Background())
				s.cancelWorker = cancel
				s.networkStream = ns

				//We expect DataStreamMsg to come back
				go func() {
					for {
						//Get message from the broadcaster
						//Call gotData(seqNo, data)
						//Question: What happens if the handler gets stuck?
						select {
						case msg := <-s.msgChan:
							// glog.Infof("Got data from msgChan: %v", msg)
							gotData(msg.SeqNo, msg.Data)
						case <-ctxW.Done():
							glog.Infof("Done with subscription, sending CancelSubMsg")
							s.Network.NetworkNode.SendMessage(ns, p, CancelSubID, CancelSubMsg{StrmID: s.StrmID})
							return
						}
					}
				}()

				return nil
			}
		}
		glog.Errorf("Cannot send message to any peer")
		return ErrNoClosePeers
	}
	glog.Errorf("Cannot find any close peers")
	return ErrNoClosePeers

	//Call gotData for every new piece of data
}

//Unsubscribe unsubscribes from the broadcast
func (s *BasicSubscriber) Unsubscribe() error {
	if s.cancelWorker != nil {
		s.cancelWorker()
	}
	return nil
}

func streamHandler(nw *BasicVideoNetwork, stream net.Stream) error {
	glog.Infof("%v Received a stream from %v", stream.Conn().LocalPeer().Pretty(), stream.Conn().RemotePeer().Pretty())
	var msg Msg

	ws := WrapStream(stream)
	err := ws.Dec.Decode(&msg)

	if err != nil {
		glog.Errorf("Got error decoding msg: %v", err)
		return err
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
		return nw.handleSubReq(sr, ws)
	case StreamDataID:
		// glog.Infof("Got Stream Data: %v", msg.Data)
		//Enque it into the subscriber
		sd, ok := msg.Data.(StreamDataMsg)
		if !ok {
			glog.Errorf("Cannot convert SubReqMsg: %v", msg.Data)
		}

		return nw.handleStreamData(sd)
	default:
		glog.Infof("Data: %v", msg)
		stream.Close()
		return ErrUnknownMsg
	}

	return nil
}

func (nw *BasicVideoNetwork) setupProtocol(node *NetworkNode) error {

	node.PeerHost.SetStreamHandler(Protocol, func(stream net.Stream) {
		streamHandler(nw, stream)
	})

	return nil
}

func (nw *BasicVideoNetwork) handleSubReq(subReq SubReqMsg, ws *WrappedStream) error {
	b := nw.broadcasters[subReq.StrmID]
	if b == nil {
		//This is when you are a relay node
		glog.Infof("Cannot find local broadcaster for stream: %v.  Forwarding along to the network", subReq.StrmID)

		//Create a local broadcaster
		b = nw.NewBroadcaster(subReq.StrmID)

		//Kick off the broadcaster worker
		ctxB, cancel := context.WithCancel(context.Background())
		b.cancelWorker = cancel
		go b.broadcastToListeners(ctxB)
		b.working = true

		//Subscribe from the network
	}

	//TODO: Add verification code for the SubNodeID (Make sure the message is not spoofed)
	remotePid := peer.IDHexEncode(ws.Stream.Conn().RemotePeer())
	b.listeners[remotePid] = ws
	return nil
}

func (nw *BasicVideoNetwork) handleStreamData(sd StreamDataMsg) error {
	if b := nw.broadcasters[sd.StrmID]; b != nil {
		//TODO: This is questionable.  Do we every have this case?
		glog.Infof("Calling broadcast")
		b.Broadcast(sd.SeqNo, sd.Data)
		return nil
	} else if s := nw.subscribers[sd.StrmID]; s != nil {
		// glog.Infof("Inserting into subscriber msg queue: %v", sd)
		s.msgChan <- sd
		return nil
	} else {
		//TODO: Add relay case
		glog.Errorf("Something is wrong.  Expect broadcaster or subscriber to exist at this point (should have been setup when SubReq came in)")
		return ErrProtocol
	}
}
