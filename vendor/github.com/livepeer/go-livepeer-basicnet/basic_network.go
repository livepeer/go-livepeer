/*
The BasicVideoNetwork is a push-based streaming protocol.  It works as follow:
	- When a video is broadcasted, it's stored at a local broadcaster
	- When a viewer wants to view a video, it sends a subscribe request to the network
	- The network routes the request towards the broadcast node via kademlia routing
*/
package basicnet

import (
	"context"
	"errors"

	peerstore "gx/ipfs/QmPgDWmTmuzvP7QE5zwo1TmjbJme9pmZHNujB2453jkCTr/go-libp2p-peerstore"
	ma "gx/ipfs/QmXY77cVe7rVRQXZZQRioukUM7aRW3BTcAgJe12MCtb3Ji/go-multiaddr"
	peer "gx/ipfs/QmXYjuNuxVzXKJCfWasQk1RqkhVLDM9jtUKhqc2WPQmFSB/go-libp2p-peer"
	protocol "gx/ipfs/QmZNkThpqfVXs9GNbexPrfBbXSLNYeKrE7jwFM2oqHbyqN/go-libp2p-protocol"
	net "gx/ipfs/QmahYsGWry85Y7WUe2SX5G4JkH2zifEQAUtJVLZ24aC9DF/go-libp2p-net"

	"github.com/golang/glog"
	lpnet "github.com/livepeer/go-livepeer/net"
	"github.com/livepeer/go-livepeer/types"
)

var Protocol = protocol.ID("/livepeer_video/0.0.1")
var ErrNoClosePeers = errors.New("NoClosePeers")
var ErrUnknownMsg = errors.New("UnknownMsgType")
var ErrProtocol = errors.New("ProtocolError")
var ErrTranscodeResult = errors.New("TranscodeResultError")

type VideoMuxer interface {
	WriteSegment(seqNo uint64, strmID string, data []byte) error
}

//BasicVideoNetwork implements the VideoNetwork interface.  It creates a kademlia network using libp2p.  It does push-based video delivery, and handles the protocol in the background.
type BasicVideoNetwork struct {
	NetworkNode  *NetworkNode
	broadcasters map[string]*BasicBroadcaster
	subscribers  map[string]*BasicSubscriber
	relayers     map[string]*BasicRelayer
}

//NewBasicVideoNetwork creates a libp2p node, handle the basic (push-based) video protocol.
func NewBasicVideoNetwork(n *NetworkNode) (*BasicVideoNetwork, error) {
	nw := &BasicVideoNetwork{NetworkNode: n, broadcasters: make(map[string]*BasicBroadcaster), subscribers: make(map[string]*BasicSubscriber), relayers: make(map[string]*BasicRelayer)}

	return nw, nil
}

//GetNodeID gets the node id
func (n *BasicVideoNetwork) GetNodeID() string {
	return peer.IDHexEncode(n.NetworkNode.Identity)
}

//GetBroadcaster gets a broadcaster for a streamID.  If it doesn't exist, create a new one.
func (n *BasicVideoNetwork) GetBroadcaster(strmID string) (lpnet.Broadcaster, error) {
	b, ok := n.broadcasters[strmID]
	if !ok {
		b = &BasicBroadcaster{Network: n, StrmID: strmID, q: make(chan *StreamDataMsg), listeners: make(map[string]*BasicStream), TranscodedIDs: make(map[string]types.VideoProfile)}
		n.broadcasters[strmID] = b
	}
	return b, nil
}

//GetSubscriber gets a subscriber for a streamID.  If it doesn't exist, create a new one.
func (n *BasicVideoNetwork) GetSubscriber(strmID string) (lpnet.Subscriber, error) {
	s, ok := n.subscribers[strmID]
	if !ok {
		s = &BasicSubscriber{Network: n, StrmID: strmID, host: n.NetworkNode.PeerHost, msgChan: make(chan StreamDataMsg)}
		n.subscribers[strmID] = s
	}
	return s, nil
}

//NewRelayer creates a new relayer.
func (n *BasicVideoNetwork) NewRelayer(strmID string) *BasicRelayer {
	r := &BasicRelayer{listeners: make(map[string]*BasicStream)}
	n.relayers[strmID] = r
	return r
}

//Connect connects a node to the Livepeer network.
func (n *BasicVideoNetwork) Connect(nodeID, addr string) error {
	pid, err := peer.IDHexDecode(nodeID)
	if err != nil {
		glog.Errorf("Invalid node ID - %v: %v", nodeID, err)
		return err
	}

	var paddr ma.Multiaddr
	paddr, err = ma.NewMultiaddr(addr)
	if err != nil {
		glog.Errorf("Invalid addr: %v", err)
		return err
	}

	n.NetworkNode.PeerHost.Peerstore().AddAddr(pid, paddr, peerstore.PermanentAddrTTL)
	return n.NetworkNode.PeerHost.Connect(context.Background(), peerstore.PeerInfo{ID: pid})
}

//SendTranscodeResult sends the transcode result to the broadcast node.
func (n *BasicVideoNetwork) SendTranscodeResult(broadcaster string, strmID string, transcodedVideos map[string]string) error {
	pid, err := peer.IDHexDecode(broadcaster)
	if err != nil {
		glog.Errorf("Bad broadcaster id %v - %v", broadcaster, err)
	}
	ws := n.NetworkNode.GetStream(pid)
	if ws != nil {
		if err = ws.SendMessage(TranscodeResultID, TranscodeResultMsg{StrmID: strmID, Result: transcodedVideos}); err != nil {
			glog.Errorf("Error sending transcode result message: %v", err)
			return err
		}
		return nil
	}

	return ErrTranscodeResult
}

//SetupProtocol sets up the protocol so we can handle incoming messages
func (n *BasicVideoNetwork) SetupProtocol() error {
	glog.Infof("\n\nSetting up protocol: %v", Protocol)
	n.NetworkNode.PeerHost.SetStreamHandler(Protocol, func(stream net.Stream) {
		ws := NewBasicStream(stream)
		n.NetworkNode.streams[stream.Conn().RemotePeer()] = ws

		for {
			if err := streamHandler(n, ws); err != nil {
				glog.Errorf("Error handling stream: %v", err)
				delete(n.NetworkNode.streams, stream.Conn().RemotePeer())
				stream.Close()
				return
			}
		}
	})

	return nil
}

func streamHandler(nw *BasicVideoNetwork, ws *BasicStream) error {
	var msg Msg

	if err := ws.ReceiveMessage(&msg); err != nil {
		glog.Errorf("Got error decoding msg: %v", err)
		return err
	}
	glog.Infof("%v Received a message %v from %v", peer.IDHexEncode(ws.Stream.Conn().LocalPeer()), msg.Op, peer.IDHexEncode(ws.Stream.Conn().RemotePeer()))
	switch msg.Op {
	case SubReqID:
		sr, ok := msg.Data.(SubReqMsg)
		if !ok {
			glog.Errorf("Cannot convert SubReqMsg: %v", msg.Data)
			return ErrProtocol
		}
		glog.Infof("Got Sub Req: %v", sr)
		return handleSubReq(nw, sr, ws)
	case CancelSubID:
		cr, ok := msg.Data.(CancelSubMsg)
		if !ok {
			glog.Errorf("Cannot convert CancelSubMsg: %v", msg.Data)
			return ErrProtocol
		}
		return handleCancelSubReq(nw, cr, ws.Stream.Conn().RemotePeer())
	case StreamDataID:
		// glog.Infof("Got Stream Data: %v", msg.Data)
		//Enque it into the subscriber
		sd, ok := msg.Data.(StreamDataMsg)
		if !ok {
			glog.Errorf("Cannot convert SubReqMsg: %v", msg.Data)
		}
		return handleStreamData(nw, sd)
	case FinishStreamID:
		fs, ok := msg.Data.(FinishStreamMsg)
		if !ok {
			glog.Errorf("Cannot convert FinishStreamMsg: %v", msg.Data)
		}
		return handleFinishStream(nw, fs)
	case TranscodeResultID:
		tr, ok := msg.Data.(TranscodeResultMsg)
		if !ok {
			glog.Errorf("Cannot convert TranscodeResultMsg: %v", msg.Data)
		}
		return handleTranscodeResult(nw, tr)
	default:
		glog.Infof("Unknown Data: %v -- closing stream", msg)
		// stream.Close()
		return ErrUnknownMsg
	}
}

func handleSubReq(nw *BasicVideoNetwork, subReq SubReqMsg, ws *BasicStream) error {
	if b := nw.broadcasters[subReq.StrmID]; b != nil {
		glog.Infof("Handling subReq, adding listener %v to broadcaster", peer.IDHexEncode(ws.Stream.Conn().RemotePeer()))
		//TODO: Add verification code for the SubNodeID (Make sure the message is not spoofed)
		remotePid := peer.IDHexEncode(ws.Stream.Conn().RemotePeer())
		b.listeners[remotePid] = ws
		return nil
	} else if r := nw.relayers[subReq.StrmID]; r != nil {
		//Already a relayer in place.  Subscribe as a listener.
		remotePid := peer.IDHexEncode(ws.Stream.Conn().RemotePeer())
		r.listeners[remotePid] = ws
		return nil
	} else {
		glog.Infof("Cannot find local broadcaster or relayer for stream: %v.  Creating a local relayer, and forwarding along to the network", subReq.StrmID)
		ctx := context.Background()
		peerc, err := nw.NetworkNode.Kad.GetClosestPeers(ctx, subReq.StrmID)
		if err != nil {
			glog.Errorf("Error finding closer peer: %v", err)
			return err
		}

		// var upstrmPeer peer.ID
		//Subscribe from the network
		//We can range over peerc because we know it'll be closed by libp2p
		for {
			select {
			case p := <-peerc:
				//Don't send it back to the requesting peer
				if p == ws.Stream.Conn().RemotePeer() {
					continue
				}

				ns := nw.NetworkNode.GetStream(p)
				if ns != nil {
					// if err := nw.NetworkNode.SendMessage(ns, p, SubReqID, subReq); err != nil {
					if err := ns.SendMessage(SubReqID, subReq); err != nil {
						//Question: Do we want to close the stream here?
						glog.Errorf("Error relaying subReq to %v: %v", p, err)
						continue
					}

					//TODO: Figure out when to return from this routine
					go func() {
						for {
							if err := streamHandler(nw, ns); err != nil {
								glog.Errorf("Error handing stream:%v", err)
								return
							}
						}
					}()

					//Create a relayer, register the listener
					r := nw.NewRelayer(subReq.StrmID)
					r.UpstreamPeer = p
					remotePid := peer.IDHexEncode(ws.Stream.Conn().RemotePeer())
					r.listeners[remotePid] = ws
					return nil
				} else {
					glog.Errorf("Cannot find stream: %v", peer.IDHexEncode(p))
				}
			case <-ctx.Done():
				glog.Errorf("Didn't find any peer from network")
				return ErrNoClosePeers
			}
		}
	}

}

func handleCancelSubReq(nw *BasicVideoNetwork, cr CancelSubMsg, rpeer peer.ID) error {
	if b := nw.broadcasters[cr.StrmID]; b != nil {
		//Remove from broadcast listener
		delete(b.listeners, peer.IDHexEncode(rpeer))
		return nil
	} else if r := nw.relayers[cr.StrmID]; r != nil {
		//Remove from relayer listener
		delete(r.listeners, peer.IDHexEncode(rpeer))
		//Pass on the cancel req and remove relayer if relayer has no more listeners
		if len(r.listeners) == 0 {
			ns := nw.NetworkNode.GetStream(r.UpstreamPeer)
			if ns != nil {
				if err := ns.SendMessage(CancelSubID, cr); err != nil {
					glog.Errorf("Error relaying cancel message to %v: %v ", peer.IDHexEncode(r.UpstreamPeer), err)
				}
				delete(nw.relayers, cr.StrmID)
				return nil
			}
		}
		return ErrProtocol
	} else {
		glog.Errorf("Cannot find broadcaster or relayer.  Error!")
		return ErrProtocol
	}
}

func handleStreamData(nw *BasicVideoNetwork, sd StreamDataMsg) error {
	//A node can have a subscriber AND a relayer for the same stream.
	s := nw.subscribers[sd.StrmID]
	if s != nil {
		// glog.Infof("Inserting into subscriber msg queue: %v", sd)
		ctx, _ := context.WithTimeout(context.Background(), SubscriberDataInsertTimeout)
		go func() {
			select {
			case s.msgChan <- sd:
			case <-ctx.Done():
				glog.Errorf("Subscriber data insert done: %v", ctx.Err())
			}
		}()
	}

	r := nw.relayers[sd.StrmID]
	if r != nil {
		if err := r.RelayStreamData(sd); err != nil {
			glog.Errorf("Error relaying stream data: %v", err)
			return err
		}
	}

	if s == nil && r == nil {
		glog.Errorf("Something is wrong.  Expect subscriber or relayer to exist at this point (should have been setup when SubReq came in)")
		return ErrProtocol
	}
	return nil
}

func handleFinishStream(nw *BasicVideoNetwork, fs FinishStreamMsg) error {
	//A node can have a subscriber AND a relayer for the same stream.
	s := nw.subscribers[fs.StrmID]
	if s != nil {
		//Cancel subscriber worker, delete subscriber
		s.cancelWorker()
		delete(nw.subscribers, fs.StrmID)
	}

	r := nw.relayers[fs.StrmID]
	if r != nil {
		if err := r.RelayFinishStream(nw, fs); err != nil {
			glog.Errorf("Error relaying finish stream: %v", err)
		}
		delete(nw.relayers, fs.StrmID)
	}

	if s == nil && r == nil {
		glog.Errorf("Error: cannot find subscriber or relayer")
		return ErrProtocol
	}
	return nil
}

func handleTranscodeResult(nw *BasicVideoNetwork, tr TranscodeResultMsg) error {
	glog.Infof("Transcode Result StreamIDs: %v", tr)
	b := nw.broadcasters[tr.StrmID]

	if b == nil {
		glog.Errorf("Error Handling Transcode Result - cannot find broadcaster with streamID: %v", tr.StrmID)
		return ErrTranscodeResult
	}

	for s, v := range tr.Result {
		if types.VideoProfileLookup[v].Name == "" {
			glog.Errorf("Error Handling Transcode Result - cannot find video profile: %v", v)
		} else {
			b.TranscodedIDs[s] = types.VideoProfileLookup[v]
		}
	}

	return nil
}
