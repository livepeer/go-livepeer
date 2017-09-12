/*
The BasicVideoNetwork is a push-based streaming protocol.  It works as follow:
	- When a video is broadcasted, it's stored at a local broadcaster
	- When a viewer wants to view a video, it sends a subscribe request to the network
	- The network routes the request towards the broadcast node via kademlia routing
*/
package basicnet

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	peerstore "gx/ipfs/QmPgDWmTmuzvP7QE5zwo1TmjbJme9pmZHNujB2453jkCTr/go-libp2p-peerstore"
	kb "gx/ipfs/QmSAFA8v42u4gpJNy1tb7vW3JiiXiaYDC2b845c2RnNSJL/go-libp2p-kbucket"
	ma "gx/ipfs/QmXY77cVe7rVRQXZZQRioukUM7aRW3BTcAgJe12MCtb3Ji/go-multiaddr"
	peer "gx/ipfs/QmXYjuNuxVzXKJCfWasQk1RqkhVLDM9jtUKhqc2WPQmFSB/go-libp2p-peer"
	protocol "gx/ipfs/QmZNkThpqfVXs9GNbexPrfBbXSLNYeKrE7jwFM2oqHbyqN/go-libp2p-protocol"
	net "gx/ipfs/QmahYsGWry85Y7WUe2SX5G4JkH2zifEQAUtJVLZ24aC9DF/go-libp2p-net"

	"github.com/ericxtang/m3u8"
	"github.com/golang/glog"
	lpmon "github.com/livepeer/go-livepeer/monitor"
	lpnet "github.com/livepeer/go-livepeer/net"
)

var Protocol = protocol.ID("/livepeer_video/0.0.1")
var ErrNoClosePeers = errors.New("NoClosePeers")
var ErrUnknownMsg = errors.New("UnknownMsgType")
var ErrProtocol = errors.New("ProtocolError")
var ErrTranscodeResponse = errors.New("TranscodeResponseError")
var ErrGetMasterPlaylist = errors.New("ErrGetMasterPlaylist")

const RelayGCTime = 60 * time.Second
const RelayTicker = 10 * time.Second

type VideoMuxer interface {
	WriteSegment(seqNo uint64, strmID string, data []byte) error
}

//BasicVideoNetwork implements the VideoNetwork interface.  It creates a kademlia network using libp2p.  It does push-based video delivery, and handles the protocol in the background.
type BasicVideoNetwork struct {
	NetworkNode            *NetworkNode
	broadcasters           map[string]*BasicBroadcaster
	subscribers            map[string]*BasicSubscriber
	relayers               map[string]*BasicRelayer
	mplMap                 map[string]*m3u8.MasterPlaylist
	mplChans               map[string]chan *m3u8.MasterPlaylist
	transResponseCallbacks map[string]func(transcodeResult map[string]string)
}

func (n *BasicVideoNetwork) String() string {
	return fmt.Sprintf("\n\nbroadcasters:%v\n\nsubscribers:%v\n\nrelayers:%v\n\n", n.broadcasters, n.subscribers, n.relayers)
}

//NewBasicVideoNetwork creates a libp2p node, handle the basic (push-based) video protocol.
func NewBasicVideoNetwork(n *NetworkNode) (*BasicVideoNetwork, error) {
	nw := &BasicVideoNetwork{
		NetworkNode:            n,
		broadcasters:           make(map[string]*BasicBroadcaster),
		subscribers:            make(map[string]*BasicSubscriber),
		relayers:               make(map[string]*BasicRelayer),
		mplMap:                 make(map[string]*m3u8.MasterPlaylist),
		mplChans:               make(map[string]chan *m3u8.MasterPlaylist),
		transResponseCallbacks: make(map[string]func(transcodeResult map[string]string))}
	n.Network = nw
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
		b = &BasicBroadcaster{Network: n, StrmID: strmID, q: make(chan *StreamDataMsg), listeners: make(map[string]*BasicStream)}
		n.broadcasters[strmID] = b
		lpmon.Instance().LogBroadcaster(strmID)
	}
	return b, nil
}

//GetSubscriber gets a subscriber for a streamID.  If it doesn't exist, create a new one.
func (n *BasicVideoNetwork) GetSubscriber(strmID string) (lpnet.Subscriber, error) {
	s, ok := n.subscribers[strmID]
	if !ok {
		s = &BasicSubscriber{Network: n, StrmID: strmID, host: n.NetworkNode.PeerHost, msgChan: make(chan StreamDataMsg)}
		n.subscribers[strmID] = s
		lpmon.Instance().LogSub(strmID)
	}
	return s, nil
}

//NewRelayer creates a new relayer.
func (n *BasicVideoNetwork) NewRelayer(strmID string) *BasicRelayer {
	r := &BasicRelayer{listeners: make(map[string]*BasicStream)}
	n.relayers[strmID] = r
	go func() {
		timer := time.NewTicker(RelayTicker)
		for {
			select {
			case <-timer.C:
				if time.Since(r.LastRelay) > RelayGCTime {
					delete(n.relayers, strmID)
					return
				}
			}
		}

	}()

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

//SendTranscodeResponse tsends the transcode result to the broadcast node.
func (n *BasicVideoNetwork) SendTranscodeResponse(broadcaster string, strmID string, transcodedVideos map[string]string) error {
	pid, err := peer.IDHexDecode(broadcaster)
	if err != nil {
		glog.Errorf("Bad broadcaster id %v - %v", broadcaster, err)
		return err
	}
	ws := n.NetworkNode.GetStream(pid)
	if ws != nil {
		if err = ws.SendMessage(TranscodeResponseID, TranscodeResponseMsg{StrmID: strmID, Result: transcodedVideos}); err != nil {
			glog.Errorf("Error sending transcode result message: %v", err)
			return err
		}
		return nil
	}

	return ErrTranscodeResponse
}

//ReceivedTranscodeResponse registers the callback for when the broadcaster receives transcode results.
func (n *BasicVideoNetwork) ReceivedTranscodeResponse(strmID string, gotResult func(transcodeResult map[string]string)) {
	n.transResponseCallbacks[strmID] = gotResult
}

//GetMasterPlaylist issues a request to the broadcaster for the MasterPlaylist and returns the channel to the playlist. The broadcaster should send the response back as soon as it gets the request.
func (n *BasicVideoNetwork) GetMasterPlaylist(p string, strmID string) (chan *m3u8.MasterPlaylist, error) {
	return n.getMasterPlaylistWithRelay(strmID)
}

func (n *BasicVideoNetwork) getMasterPlaylistWithRelay(strmID string) (chan *m3u8.MasterPlaylist, error) {
	c := make(chan *m3u8.MasterPlaylist)
	n.mplChans[strmID] = c

	go func() {
		//Check to see if we have the playlist locally
		mpl := n.mplMap[strmID]
		if mpl != nil {
			c <- mpl
			return
		}

		//Ask the network for the playlist
		peers, err := closestLocalPeers(n.NetworkNode.PeerHost.Peerstore(), strmID)
		if err != nil {
			glog.Errorf("Error getting closest local peers: %v", err)
			return
		}
		for _, pid := range peers {
			if pid == n.NetworkNode.Identity {
				continue
			}

			s := n.NetworkNode.GetStream(pid)
			if s != nil {
				if err := s.SendMessage(GetMasterPlaylistReqID, GetMasterPlaylistReqMsg{StrmID: strmID}); err != nil {
					continue
				}
			}
			return
		}
	}()

	return c, nil
}

func (n *BasicVideoNetwork) getMasterPlaylistWithDHT(p string, strmID string) (chan *m3u8.MasterPlaylist, error) {
	c := make(chan *m3u8.MasterPlaylist)
	n.mplChans[strmID] = c

	go func() {
		// We cannot control where the DHT stores the playlist.  The key takes the form of a/b/c, the nodeID is a regular string.
		// nid, err := extractNodeID(strmID)
		// if err != nil {
		// 	return
		// }
		// pl, err := n.NetworkNode.Kad.GetValue(context.Background(), string([]byte(nid)))
		pl, err := n.NetworkNode.Kad.GetValue(context.Background(), fmt.Sprintf("/v/%v", strmID))
		if err != nil {
			glog.Errorf("Error getting value for %v: %v", strmID, err)
			return
		}
		mpl := m3u8.NewMasterPlaylist()
		if err := mpl.DecodeFrom(bytes.NewReader(pl), true); err == nil {
			c <- mpl
		} else {
			glog.Errorf("Error decoding master playlist: %v", err)
		}
	}()

	return c, nil
}

//UpdateMasterPlaylist updates the copy of the master playlist so any node can request it.
func (n *BasicVideoNetwork) UpdateMasterPlaylist(strmID string, mpl *m3u8.MasterPlaylist) error {
	return n.updateMasterPlaylistWithRelay(strmID, mpl)
}

func (n *BasicVideoNetwork) updateMasterPlaylistWithRelay(strmID string, mpl *m3u8.MasterPlaylist) error {
	n.mplMap[strmID] = mpl
	return nil
}

func (n *BasicVideoNetwork) updateMasterPlaylistWithDHT(strmID string, mpl *m3u8.MasterPlaylist) error {
	// nid, err := extractNodeID(strmID)
	// if err != nil {
	// 	return err
	// }
	// if err := n.NetworkNode.Kad.PutValue(context.Background(), string([]byte(nid)), mpl.Encode().Bytes()); err != nil {
	if err := n.NetworkNode.Kad.PutValue(context.Background(), fmt.Sprintf("/v/%v", strmID), mpl.Encode().Bytes()); err != nil {
		glog.Errorf("Error putting playlist into DHT: %v", err)
		return err
	}
	return nil
}

//SetupProtocol sets up the protocol so we can handle incoming messages
func (n *BasicVideoNetwork) SetupProtocol() error {
	glog.V(4).Infof("\n\nSetting up protocol: %v", Protocol)
	n.NetworkNode.PeerHost.SetStreamHandler(Protocol, func(stream net.Stream) {
		ws := NewBasicStream(stream)
		n.NetworkNode.streams[stream.Conn().RemotePeer()] = ws

		for {
			if err := streamHandler(n, ws); err != nil {
				glog.Errorf("Error handling stream: %v", err)
				n.NetworkNode.RemoveStream(stream.Conn().RemotePeer())
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
		glog.Errorf("%v Got error decoding msg: %v", peer.IDHexEncode(ws.Stream.Conn().LocalPeer()), err)
		return err
	}
	glog.V(4).Infof("%v Received a message %v from %v", peer.IDHexEncode(ws.Stream.Conn().LocalPeer()), msg.Op, peer.IDHexEncode(ws.Stream.Conn().RemotePeer()))
	switch msg.Op {
	case SubReqID:
		sr, ok := msg.Data.(SubReqMsg)
		if !ok {
			glog.Errorf("Cannot convert SubReqMsg: %v", msg.Data)
			return ErrProtocol
		}
		glog.V(5).Infof("Got Sub Req: %v", sr)
		return handleSubReq(nw, sr, ws)
	case CancelSubID:
		cr, ok := msg.Data.(CancelSubMsg)
		if !ok {
			glog.Errorf("Cannot convert CancelSubMsg: %v", msg.Data)
			return ErrProtocol
		}
		return handleCancelSubReq(nw, cr, ws.Stream.Conn().RemotePeer())
	case StreamDataID:
		glog.V(5).Infof("Got Stream Data: %v", msg.Data)
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
	case TranscodeResponseID:
		tr, ok := msg.Data.(TranscodeResponseMsg)
		if !ok {
			glog.Errorf("Cannot convert TranscodeResponseMsg: %v", msg.Data)
		}
		return handleTranscodeResponse(nw, tr)
	case GetMasterPlaylistReqID:
		//Get the local master playlist from a broadcaster and send it back
		mplr, ok := msg.Data.(GetMasterPlaylistReqMsg)
		if !ok {
			glog.Errorf("Cannot convert GetMasterPlaylistReqMsg: %v", msg.Data)
		}
		return handleGetMasterPlaylistReq(nw, ws, mplr)
	case MasterPlaylistDataID:
		mpld, ok := msg.Data.(MasterPlaylistDataMsg)
		if !ok {
			glog.Errorf("Cannot convert MasterPlaylistDataMsg: %v", msg.Data)
		}
		return handleMasterPlaylistDataMsg(nw, mpld)
	default:
		glog.V(2).Infof("Unknown Data: %v -- closing stream", msg)
		// stream.Close()
		return ErrUnknownMsg
	}
}

func handleSubReq(nw *BasicVideoNetwork, subReq SubReqMsg, ws *BasicStream) error {
	//If we have local broadcaster, just listen.
	if b := nw.broadcasters[subReq.StrmID]; b != nil {
		glog.V(5).Infof("Handling subReq, adding listener %v to broadcaster", peer.IDHexEncode(ws.Stream.Conn().RemotePeer()))
		//TODO: Add verification code for the SubNodeID (Make sure the message is not spoofed)
		remotePid := peer.IDHexEncode(ws.Stream.Conn().RemotePeer())
		b.listeners[remotePid] = ws

		//Send the last video chunk so we don't have to wait for the next one.
		b.sendDataMsg(remotePid, ws, b.lastMsg)
		return nil
	}

	//If we don't have local broadcaster, forward the sub request to the closest peer
	peers, err := closestLocalPeers(nw.NetworkNode.PeerHost.Peerstore(), subReq.StrmID)
	if err != nil {
		glog.Errorf("Error getting closest local node: %v", err)
		return err
	}

	//Send Sub Req to the network
	for _, p := range peers {
		//Don't send it back to the requesting peer
		if p == ws.Stream.Conn().RemotePeer() || p == nw.NetworkNode.Identity {
			continue
		}

		if p == "" {
			glog.Errorf("Got empty peer from libp2p")
			return nil
		}

		ns := nw.NetworkNode.GetStream(p)
		if ns != nil {
			if err := ns.SendMessage(SubReqID, subReq); err != nil {
				//Question: Do we want to close the stream here?
				glog.Errorf("Error relaying subReq to %v: %v.", p, err)
				continue
			}

			if r := nw.relayers[subReq.StrmID]; r != nil {
				remotePid := peer.IDHexEncode(ws.Stream.Conn().RemotePeer())
				r.listeners[remotePid] = ws
			} else {
				r := nw.NewRelayer(subReq.StrmID)
				r.UpstreamPeer = p
				lpmon.Instance().LogRelay(subReq.StrmID, peer.IDHexEncode(p))
				remotePid := peer.IDHexEncode(ws.Stream.Conn().RemotePeer())
				r.listeners[remotePid] = ws
			}
			return nil
		} else {
			glog.Errorf("Cannot get stream for peer: %v", peer.IDHexEncode(p))
		}
	}

	glog.Errorf("%v Cannot forward Sub req to any of the peers: %v", nw.GetNodeID(), peers)
	return ErrProtocol
}

func handleCancelSubReq(nw *BasicVideoNetwork, cr CancelSubMsg, rpeer peer.ID) error {
	if b := nw.broadcasters[cr.StrmID]; b != nil {
		//Remove from broadcast listener
		delete(b.listeners, peer.IDHexEncode(rpeer))
		return nil
	} else if r := nw.relayers[cr.StrmID]; r != nil {
		//Remove from relayer listener
		delete(r.listeners, peer.IDHexEncode(rpeer))
		lpmon.Instance().RemoveRelay(cr.StrmID)
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
		glog.V(5).Infof("Inserting into subscriber msg queue: %v", sd)
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
		glog.Errorf("%v, Something is wrong.  Expect subscriber or relayer to exist at this point (should have been setup when SubReq came in)", peer.IDHexEncode(nw.NetworkNode.Identity))
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
		lpmon.Instance().RemoveRelay(fs.StrmID)
	}

	if s == nil && r == nil {
		glog.Errorf("Error: cannot find subscriber or relayer")
		return ErrProtocol
	}
	return nil
}

func handleTranscodeResponse(nw *BasicVideoNetwork, tr TranscodeResponseMsg) error {
	glog.V(5).Infof("Transcode Result StreamIDs: %v", tr)
	callback, ok := nw.transResponseCallbacks[tr.StrmID]
	if !ok {
		glog.Errorf("Error handling transcode result - cannot find callback for stream: %v", tr.StrmID)
		return ErrTranscodeResponse
	}

	callback(tr.Result)
	return nil
}

func handleGetMasterPlaylistReq(nw *BasicVideoNetwork, ws *BasicStream, mplr GetMasterPlaylistReqMsg) error {
	mpl, ok := nw.mplMap[mplr.StrmID]
	if !ok {
		//Don't have the playlist locally. Forward to a peer
		peers, err := closestLocalPeers(nw.NetworkNode.PeerHost.Peerstore(), mplr.StrmID)
		if err != nil {
			return ws.SendMessage(MasterPlaylistDataID, MasterPlaylistDataMsg{StrmID: mplr.StrmID, NotFound: true})
		}
		for _, p := range peers {
			//Don't send it back to the requesting peer
			if p == ws.Stream.Conn().RemotePeer() || p == nw.NetworkNode.Identity {
				continue
			}

			if p == "" {
				glog.Errorf("Got empty peer from libp2p")
				return nil
			}

			s := nw.NetworkNode.GetStream(p)
			if s != nil {
				if err := s.SendMessage(GetMasterPlaylistReqID, GetMasterPlaylistReqMsg{StrmID: mplr.StrmID}); err != nil {
					continue
				}

				r := nw.NewRelayer(mplr.StrmID)
				r.UpstreamPeer = p
				lpmon.Instance().LogRelay(mplr.StrmID, peer.IDHexEncode(p))
				remotePid := peer.IDHexEncode(ws.Stream.Conn().RemotePeer())
				r.listeners[remotePid] = ws
				return nil
			}
		}
	}

	return ws.SendMessage(MasterPlaylistDataID, MasterPlaylistDataMsg{StrmID: mplr.StrmID, MPL: mpl.String()})
}

func handleMasterPlaylistDataMsg(nw *BasicVideoNetwork, mpld MasterPlaylistDataMsg) error {
	ch, ok := nw.mplChans[mpld.StrmID]
	if !ok {
		r := nw.relayers[mpld.StrmID]
		if r != nil {
			//Relay the data
			return r.RelayMasterPlaylistData(nw, mpld)
		} else {
			glog.Errorf("Got master playlist data, but don't have a channel")
			return ErrGetMasterPlaylist
		}
	}

	//Decode the playlist from a string
	mpl := m3u8.NewMasterPlaylist()
	if err := mpl.DecodeFrom(strings.NewReader(mpld.MPL), true); err != nil {
		glog.Errorf("Error decoding playlist: %v", err)
		return ErrGetMasterPlaylist
	}

	//insert into channel
	ch <- mpl
	//close channel, delete it
	close(ch)
	delete(nw.mplChans, mpld.StrmID)
	return nil
}

func extractNodeID(strmID string) (peer.ID, error) {
	if len(strmID) < 68 {
		return "", ErrProtocol
	}

	nid := strmID[:68]
	return peer.IDHexDecode(nid)
}

func closestLocalPeers(ps peerstore.Peerstore, strmID string) ([]peer.ID, error) {
	targetPid, err := extractNodeID(strmID)
	if err != nil {
		glog.Errorf("Error extracting node id from streamID: %v", strmID)
		return nil, ErrSubscriber
	}
	localPeers := ps.Peers()
	if len(localPeers) == 1 {
		glog.Errorf("No local peers")
		return nil, ErrSubscriber
	}

	return kb.SortClosestPeers(localPeers, kb.ConvertPeerID(targetPid)), nil
}
