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
	"math/rand"
	"reflect"
	"strings"
	"time"

	ma "gx/ipfs/QmWWQ2Txc2c6tqjsBpzg5Ar652cHPGNsQQp2SejkNmkUMb/go-multiaddr"
	peerstore "gx/ipfs/QmXauCuJzmzapetmC6W4TuDJLL1yFFrVzSHoWv8YdbmnxH/go-libp2p-peerstore"
	net "gx/ipfs/QmXfkENeeBvh3zYA51MaSdGUdBjhQ99cP5WQe8zgr6wchG/go-libp2p-net"
	protocol "gx/ipfs/QmZNkThpqfVXs9GNbexPrfBbXSLNYeKrE7jwFM2oqHbyqN/go-libp2p-protocol"
	peer "gx/ipfs/QmZoWKhxUmZ2seW4BzX6fJkNR8hh9PsGModr7q171yq2SS/go-libp2p-peer"

	"github.com/ericxtang/m3u8"
	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/common"
	lpmon "github.com/livepeer/go-livepeer/monitor"
	lpnet "github.com/livepeer/go-livepeer/net"
	"github.com/livepeer/lpms/stream"
)

var Protocol = protocol.ID("/livepeer_video/0.0.1")
var ErrNoClosePeers = errors.New("NoClosePeers")
var ErrUnknownMsg = errors.New("UnknownMsgType")
var ErrProtocol = errors.New("ProtocolError")
var ErrHandleMsg = errors.New("ErrHandleMsg")
var ErrTranscodeResponse = errors.New("TranscodeResponseError")
var ErrGetMasterPlaylist = errors.New("ErrGetMasterPlaylist")
var ErrSendMsg = errors.New("ErrSendMsg")
var GetMasterPlaylistRelayWait = 10 * time.Second
var GetResponseWithRelayWait = 10 * time.Second

const RelayGCTime = 60 * time.Second
const RelayTicker = 10 * time.Second
const DefaultBroadcasterBufferSize = 3
const DefaultBroadcasterBufferSegSendInterval = time.Second
const DefaultTranscodeResponseRelayDuplication = 2
const ConnTimeout = 3 * time.Second

var ConnFileWriteFreq = time.Duration(60) * time.Second

type VideoMuxer interface {
	WriteSegment(seqNo uint64, strmID string, data []byte) error
}

//BasicVideoNetwork implements the VideoNetwork interface.  It creates a kademlia network using libp2p.  It does push-based video delivery, and handles the protocol in the background.
type BasicVideoNetwork struct {
	NetworkNode            NetworkNode
	broadcasters           map[string]*BasicBroadcaster
	subscribers            map[string]*BasicSubscriber
	mplMap                 map[string]*m3u8.MasterPlaylist
	mplChans               map[string]chan *m3u8.MasterPlaylist
	msgChans               map[string]chan *Msg
	transResponseCallbacks map[string]func(transcodeResult map[string]string)
	relayers               map[relayerID]*BasicRelayer
	msgCounts              map[Opcode]int64
	pingChs                map[string]chan struct{}
}

func (n *BasicVideoNetwork) String() string {
	peers := make([]string, 0)
	for _, p := range n.NetworkNode.GetPeers() {
		peers = append(peers, fmt.Sprintf("%v[%v]", peer.IDHexEncode(p), n.NetworkNode.GetPeerInfo(p)))
	}
	return fmt.Sprintf("\n\nbroadcasters:%v\n\nsubscribers:%v\n\nrelayers:%v\n\npeers:%v\n\nmasterPlaylists:%v\n\n", n.broadcasters, n.subscribers, n.relayers, peers, n.mplMap)
}

func (n *BasicVideoNetwork) GetLocalStreams() []string {
	result := make([]string, 0)
	for strmID, _ := range n.broadcasters {
		result = append(result, strmID)
	}
	for strmID, _ := range n.subscribers {
		result = append(result, strmID)
	}
	return result
}

//NewBasicVideoNetwork creates a libp2p node, handle the basic (push-based) video protocol.
func NewBasicVideoNetwork(n *BasicNetworkNode, workDir string) (*BasicVideoNetwork, error) {
	nw := &BasicVideoNetwork{
		NetworkNode:            n,
		broadcasters:           make(map[string]*BasicBroadcaster),
		subscribers:            make(map[string]*BasicSubscriber),
		relayers:               make(map[relayerID]*BasicRelayer),
		mplMap:                 make(map[string]*m3u8.MasterPlaylist),
		mplChans:               make(map[string]chan *m3u8.MasterPlaylist),
		msgChans:               make(map[string]chan *Msg),
		msgCounts:              make(map[Opcode]int64),
		pingChs:                make(map[string]chan struct{}),
		transResponseCallbacks: make(map[string]func(transcodeResult map[string]string))}
	n.Network = nw

	//Set up a worker to write connections
	if workDir != "" {
		peerCache := NewPeerCache(n.PeerHost.Peerstore(), fmt.Sprintf("%v/conn", workDir))
		peers := peerCache.LoadPeers()
		for _, p := range peers {
			glog.Infof("Connecting to cached peer: %v", p)
			nw.connectPeerInfo(p)
		}
		go peerCache.Record(context.Background())
	}
	return nw, nil
}

//GetNodeID gets the node id
func (n *BasicVideoNetwork) GetNodeID() string {
	return peer.IDHexEncode(n.NetworkNode.ID())
}

//GetBroadcaster gets a broadcaster for a streamID.  If it doesn't exist, create a new one.
func (n *BasicVideoNetwork) GetBroadcaster(strmID string) (stream.Broadcaster, error) {
	b, ok := n.broadcasters[strmID]
	if !ok {
		b = &BasicBroadcaster{
			Network:   n,
			StrmID:    strmID,
			q:         make(chan *StreamDataMsg),
			listeners: make(map[string]OutStream),
			lastMsgs:  make([]*StreamDataMsg, DefaultBroadcasterBufferSize, DefaultBroadcasterBufferSize)}

		n.broadcasters[strmID] = b
		lpmon.Instance().LogBroadcaster(strmID)
	}
	return b, nil
}

func (n *BasicVideoNetwork) SetBroadcaster(strmID string, b *BasicBroadcaster) {
	n.broadcasters[strmID] = b
}

//GetSubscriber gets a subscriber for a streamID.  If it doesn't exist, create a new one.
func (n *BasicVideoNetwork) GetSubscriber(strmID string) (stream.Subscriber, error) {
	s, ok := n.subscribers[strmID]
	if !ok {
		s = &BasicSubscriber{Network: n, StrmID: strmID, msgChan: make(chan StreamDataMsg)}
		n.subscribers[strmID] = s
		lpmon.Instance().LogSub(strmID)
	}
	return s, nil
}

func (n *BasicVideoNetwork) SetSubscriber(strmID string, s *BasicSubscriber) {
	n.subscribers[strmID] = s
}

func (n *BasicVideoNetwork) getSubscriber(strmID string) *BasicSubscriber {
	if s, ok := n.subscribers[strmID]; ok {
		return s
	}
	return nil
}

//NewRelayer creates a new relayer.
func (n *BasicVideoNetwork) NewRelayer(strmID string, opcode Opcode) *BasicRelayer {
	r := &BasicRelayer{listeners: make(map[string]*BasicOutStream), Network: n}
	n.relayers[relayerMapKey(strmID, opcode)] = r
	go func() {
		timer := time.NewTicker(RelayTicker)
		for {
			select {
			case <-timer.C:
				if time.Since(r.LastRelay) > RelayGCTime {
					delete(n.relayers, relayerMapKey(strmID, opcode))
					return
				}
			}
		}

	}()

	return r
}

//Connect connects a node to the Livepeer network.
func (n *BasicVideoNetwork) Connect(nodeID string, addrs []string) error {
	pid, err := peer.IDHexDecode(nodeID)
	if err != nil {
		glog.Errorf("Invalid node ID - %v: %v", nodeID, err)
		return err
	}

	paddrs := make([]ma.Multiaddr, 0)
	for _, addr := range addrs {
		paddr, err := ma.NewMultiaddr(addr)
		if err != nil {
			glog.Errorf("Invalid addr: %v", err)
			return err
		}
		paddrs = append(paddrs, paddr)
	}

	info := peerstore.PeerInfo{ID: pid, Addrs: paddrs}
	return n.connectPeerInfo(info)
}

func (n *BasicVideoNetwork) connectPeerInfo(info peerstore.PeerInfo) error {
	ctx, _ := context.WithTimeout(context.Background(), ConnTimeout)
	if err := n.NetworkNode.Connect(ctx, info); err == nil {
		n.NetworkNode.AddPeer(info, peerstore.PermanentAddrTTL)
		return nil
	} else {
		n.NetworkNode.RemovePeer(info.ID)
		return err
	}
}

//SendTranscodeResponse sends the transcode result to the broadcast node.
func (n *BasicVideoNetwork) SendTranscodeResponse(broadcaster string, strmID string, transcodedVideos map[string]string) error {
	//Don't do anything if the node is the transcoder and the broadcaster at the same time.
	if n.GetNodeID() == broadcaster {
		glog.Infof("CurrentNode: %v, broadcaster: %v", n.GetNodeID(), broadcaster)
		return nil
	}

	peers, err := n.NetworkNode.ClosestLocalPeers(strmID)
	if err != nil {
		glog.Errorf("Error getting closest local peers: %v", err)
		return ErrTranscodeResponse
	}

	for _, pid := range peers {
		if pid == n.NetworkNode.ID() {
			continue
		}

		s := n.NetworkNode.GetOutStream(pid)
		if s != nil {
			if err = n.sendMessageWithRetry(pid, s, TranscodeResponseID, TranscodeResponseMsg{StrmID: strmID, Result: transcodedVideos}); err != nil {
				continue
			}
			return nil
		}
	}

	return ErrTranscodeResponse
}

//ReceivedTranscodeResponse sends a request and registers the callback for when the broadcaster receives transcode results.
func (n *BasicVideoNetwork) ReceivedTranscodeResponse(strmID string, gotResult func(transcodeResult map[string]string)) {
	n.transResponseCallbacks[strmID] = gotResult
}

func (n *BasicVideoNetwork) getResponseWithRelay(msg Msg, msgKey string, nodeID string) chan *Msg {
	returnC := make(chan *Msg)
	c := make(chan *Msg)
	n.msgChans[msgKey] = c

	go func(c chan *Msg, returnC chan *Msg, msgChans map[string]chan *Msg, msgKey string, nodeID string) {
		defer close(c)
		defer delete(msgChans, msgKey)

		peers, err := n.NetworkNode.ClosestLocalPeers(nodeID)
		if err != nil {
			glog.Errorf("Error getting closest local peers; %v", err)
			return
		}
		for _, pid := range peers {
			if pid == n.NetworkNode.ID() {
				continue
			}

			s := n.NetworkNode.GetOutStream(pid)
			if s != nil {
				if err := n.sendMessageWithRetry(pid, s, msg.Op, msg.Data); err != nil {
					continue
				}
			}
			timer := time.NewTimer(GetResponseWithRelayWait)
			select {
			case response := <-c:
				if response != nil {
					returnC <- response
					return
				}
			case <-timer.C:
				continue
			}
		}

	}(c, returnC, n.msgChans, msgKey, nodeID)

	return returnC
}

//GetMasterPlaylist issues a request to the broadcaster for the MasterPlaylist and returns the channel to the playlist. The broadcaster should send the response back as soon as it gets the request.
func (n *BasicVideoNetwork) GetMasterPlaylist(p string, manifestID string) (chan *m3u8.MasterPlaylist, error) {
	//Don't need to call out to the network if we already have it.
	if mpl, ok := n.mplMap[manifestID]; ok {
		returnC := make(chan *m3u8.MasterPlaylist)
		go func(returnC chan *m3u8.MasterPlaylist, mpl *m3u8.MasterPlaylist) {
			defer close(returnC)
			returnC <- mpl
		}(returnC, mpl)

		return returnC, nil
	}
	nid, err := extractNodeID(manifestID)
	if err != nil {
		glog.Errorf("Error extracting NodeID from: %v", manifestID)
		return nil, ErrGetMasterPlaylist
	}
	if n.GetNodeID() == peer.IDHexEncode(nid) {
		glog.Errorf("Node ID from manifest:%v is the same as the current node ID: %v", peer.IDHexEncode(nid), n.GetNodeID())
		return nil, ErrGetMasterPlaylist
	}
	return n.getMasterPlaylistWithRelay(manifestID)
}

func (n *BasicVideoNetwork) getMasterPlaylistWithRelay(manifestID string) (chan *m3u8.MasterPlaylist, error) {
	returnC := make(chan *m3u8.MasterPlaylist)
	pid, err := extractNodeID(manifestID)
	if err != nil {
		return returnC, nil
	}

	msgC := n.getResponseWithRelay(Msg{Op: GetMasterPlaylistReqID, Data: GetMasterPlaylistReqMsg{ManifestID: manifestID}}, msgChansKey(GetMasterPlaylistReqID, manifestID), peer.IDHexEncode(pid))
	go func(msgC chan *Msg, returnC chan *m3u8.MasterPlaylist) {
		defer close(returnC)

		select {
		case msg := <-msgC:
			mpld := msg.Data.(MasterPlaylistDataMsg)
			if mpld.NotFound {
				returnC <- nil
				return
			}

			//Decode the playlist from a string
			mpl := m3u8.NewMasterPlaylist()
			if err := mpl.DecodeFrom(strings.NewReader(mpld.MPL), true); err != nil {
				glog.Errorf("Error decoding playlist: %v", err)
				return
			}

			returnC <- mpl
		}
	}(msgC, returnC)

	return returnC, nil
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
		pl, err := n.NetworkNode.GetDHT().GetValue(context.Background(), fmt.Sprintf("/v/%v", strmID))
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

//Simple relay method to get master playlist
func (n *BasicVideoNetwork) updateMasterPlaylistWithRelay(strmID string, mpl *m3u8.MasterPlaylist) error {
	if mpl != nil {
		n.mplMap[strmID] = mpl
	} else {
		delete(n.mplMap, strmID)
	}
	return nil
}

//DHT-style master playlist query.  Not using it for now because it's been pretty slow.
func (n *BasicVideoNetwork) updateMasterPlaylistWithDHT(strmID string, mpl *m3u8.MasterPlaylist) error {
	if err := n.NetworkNode.GetDHT().PutValue(context.Background(), fmt.Sprintf("/v/%v", strmID), mpl.Encode().Bytes()); err != nil {
		glog.Errorf("Error putting playlist into DHT: %v", err)
		return err
	}
	return nil
}

func (n *BasicVideoNetwork) GetNodeStatus(nodeID string) (chan *lpnet.NodeStatus, error) {
	if n.GetNodeID() == nodeID {
		returnC := make(chan *lpnet.NodeStatus)
		go func(chan *lpnet.NodeStatus) {
			defer close(returnC)
			returnC <- n.nodeStatus()
		}(returnC)
		return returnC, nil
	} else {
		return n.getNodeStatusWithRelay(nodeID), nil
	}
}

func (n *BasicVideoNetwork) getNodeStatusWithRelay(nodeID string) chan *lpnet.NodeStatus {
	returnC := make(chan *lpnet.NodeStatus)

	msgC := n.getResponseWithRelay(Msg{Op: NodeStatusReqID, Data: NodeStatusReqMsg{NodeID: nodeID}}, msgChansKey(NodeStatusReqID, nodeID), nodeID)
	go func(msgC chan *Msg, returnC chan *lpnet.NodeStatus) {
		defer close(returnC)

		select {
		case msg := <-msgC:
			if msg == nil {
				return
			}

			if msg.Data.(NodeStatusDataMsg).NotFound {
				returnC <- nil
				return
			}

			mdata := string(msg.Data.(NodeStatusDataMsg).Data)
			status := &lpnet.NodeStatus{}
			if err := status.FromString(mdata); err != nil {
				return
			}

			returnC <- status
		}
	}(msgC, returnC)
	return returnC
}

func (n *BasicVideoNetwork) nodeStatus() *lpnet.NodeStatus {
	mpls := make(map[string]*m3u8.MasterPlaylist, 0)
	for mid, mpl := range n.mplMap {
		mpls[mid] = mpl
	}
	return &lpnet.NodeStatus{
		NodeID:    n.GetNodeID(),
		Manifests: mpls,
	}
}

func (n *BasicVideoNetwork) Ping(nid, addr string) (chan struct{}, error) {
	returnCh := make(chan struct{})
	id, err := peer.IDHexDecode(nid)
	if err != nil {
		return nil, errors.New("ErrBadNodeID")
	}
	s := n.NetworkNode.GetOutStream(id)
	if s == nil {
		return nil, ErrSendMsg
	}
	nonce := randStr()
	//Send the message, try to reconnect if connection was reset
	if err := s.SendMessage(PingID, PingDataMsg(nonce)); err != nil {
		if err.Error() == "connection reset" {
			if err := s.Stream.Conn().Close(); err != nil {
				glog.Errorf("Error closing conn: %v", err)
				return nil, err
			}
			if err := n.Connect(nid, []string{addr}); err != nil {
				return nil, err
			}
		}
		s = n.NetworkNode.GetOutStream(id)
		if s == nil {
			return nil, ErrSendMsg
		}
		if err := s.SendMessage(PingID, ""); err != nil {
			return nil, err
		}
	}
	n.pingChs[nonce] = returnCh
	return returnCh, nil
}

//SetupProtocol sets up the protocol so we can handle incoming messages
func (n *BasicVideoNetwork) SetupProtocol() error {
	glog.V(4).Infof("\n\nSetting up protocol: %v", Protocol)
	n.NetworkNode.SetStreamHandler(Protocol, func(stream net.Stream) {
		ws := NewBasicInStream(stream)
		for {
			if err := streamHandler(n, ws); err != nil {
				if err != ErrHandleMsg {
					glog.Errorf("Error handling stream: %v", err)
					n.NetworkNode.RemoveStream(stream.Conn().RemotePeer())
					stream.Reset()
					return
				}
			}
		}
	})

	return nil
}

func streamHandler(nw *BasicVideoNetwork, ws *BasicInStream) error {
	msg, err := ws.ReceiveMessage()
	if err != nil {
		glog.Errorf("Got error decoding msg from %v: %v (%v).", peer.IDHexEncode(ws.Stream.Conn().RemotePeer()), err, reflect.TypeOf(err))
		return err
	}
	// glog.V(4).Infof("%v Received a message %v from %v", peer.IDHexEncode(ws.Stream.Conn().LocalPeer()), msg.Op, peer.IDHexEncode(ws.Stream.Conn().RemotePeer()))
	glog.V(4).Infof("Received a message %v from %v", msg.Op, peer.IDHexEncode(ws.Stream.Conn().RemotePeer()))
	switch msg.Op {
	case SubReqID:
		sr, ok := msg.Data.(SubReqMsg)
		if !ok {
			glog.Errorf("Cannot convert SubReqMsg: %v", msg.Data)
			return ErrProtocol
		}
		glog.V(5).Infof("Got Sub Req: %v", sr)
		nw.msgCounts[msg.Op]++
		return handleSubReq(nw, sr, ws.Stream.Conn().RemotePeer())
	case CancelSubID:
		cr, ok := msg.Data.(CancelSubMsg)
		if !ok {
			glog.Errorf("Cannot convert CancelSubMsg: %v", msg.Data)
			return ErrProtocol
		}
		nw.msgCounts[msg.Op]++
		return handleCancelSubReq(nw, cr, ws.Stream.Conn().RemotePeer())
	case StreamDataID:
		//Enque it into the subscriber
		sd, ok := msg.Data.(StreamDataMsg)
		if !ok {
			glog.Errorf("Cannot convert SubReqMsg: %v", msg.Data)
			return ErrProtocol
		}
		err := handleStreamData(nw, ws.Stream.Conn().RemotePeer(), &sd)
		nw.msgCounts[msg.Op]++
		if err == ErrProtocol {
			glog.Errorf("Got protocol error, but ignoring it for now")
			return nil
		} else {
			return err
		}
	case FinishStreamID:
		fs, ok := msg.Data.(FinishStreamMsg)
		if !ok {
			glog.Errorf("Cannot convert FinishStreamMsg: %v", msg.Data)
			return ErrProtocol
		}
		nw.msgCounts[msg.Op]++
		return handleFinishStream(nw, fs)
	case TranscodeResponseID:
		tr, ok := msg.Data.(TranscodeResponseMsg)
		if !ok {
			glog.Errorf("Cannot convert TranscodeResponseMsg: %v", msg.Data)
			return ErrProtocol
		}
		nw.msgCounts[msg.Op]++
		return handleTranscodeResponse(nw, ws.Stream.Conn().RemotePeer(), tr)
	case GetMasterPlaylistReqID:
		//Get the local master playlist from a broadcaster and send it back
		mplr, ok := msg.Data.(GetMasterPlaylistReqMsg)
		if !ok {
			glog.Errorf("Cannot convert GetMasterPlaylistReqMsg: %v", msg.Data)
			return ErrProtocol
		}
		nw.msgCounts[msg.Op]++
		return handleGetMasterPlaylistReq(nw, ws.Stream.Conn().RemotePeer(), mplr)
	case MasterPlaylistDataID:
		mpld, ok := msg.Data.(MasterPlaylistDataMsg)
		if !ok {
			glog.Errorf("Cannot convert MasterPlaylistDataMsg: %v", msg.Data)
			return ErrProtocol
		}
		nw.msgCounts[msg.Op]++
		return handleMasterPlaylistDataMsg(nw, mpld)
	case NodeStatusReqID:
		nsr, ok := msg.Data.(NodeStatusReqMsg)
		if !ok {
			glog.Errorf("Cannot convert NodeStatusReqMsg: %v", msg)
			return ErrProtocol
		}
		nw.msgCounts[msg.Op]++
		return handleNodeStatusReqMsg(nw, ws.Stream.Conn().RemotePeer(), nsr)
	case NodeStatusDataID:
		nsd, ok := msg.Data.(NodeStatusDataMsg)
		if !ok {
			glog.Errorf("Cannot convert NodeStatusDataMsg: %v", msg.Data)
			return ErrProtocol
		}
		nw.msgCounts[msg.Op]++
		return handleNodeStatusDataMsg(nw, nsd)
	case PingID:
		pd, ok := msg.Data.(PingDataMsg)
		if !ok {
			glog.Errorf("Cannot convert PingDataMsg: %v", msg.Data)
			return ErrProtocol
		}
		return handlePing(nw, ws.Stream.Conn().RemotePeer(), pd)
	case PongID:
		pd, ok := msg.Data.(PongDataMsg)
		if !ok {
			glog.Errorf("Cannot convert PongDataMsg: %v", msg.Data)
			return ErrProtocol
		}
		return handlePong(nw, ws.Stream.Conn().RemotePeer(), pd)
	default:
		glog.V(2).Infof("Unknown Data: %v -- closing stream", msg)
		// stream.Close()
		return ErrUnknownMsg
	}
}

func handleSubReq(nw *BasicVideoNetwork, subReq SubReqMsg, remotePID peer.ID) error {
	glog.Infof("Handling sub req for %v", subReq.StrmID)
	//If we have local broadcaster, just listen.
	if b := nw.broadcasters[subReq.StrmID]; b != nil {
		glog.V(5).Infof("Handling subReq, adding listener %v to broadcaster", peer.IDHexEncode(remotePID))
		//TODO: Add verification code for the SubNodeID (Make sure the message is not spoofed)
		b.AddListeningPeer(nw, remotePID)

		//Send the last video chunk so we don't have to wait for the next one.
		for _, msg := range b.lastMsgs {
			if msg != nil {
				b.sendDataMsg(peer.IDHexEncode(remotePID), nw.NetworkNode.GetOutStream(remotePID), msg)
				time.Sleep(DefaultBroadcasterBufferSegSendInterval)
			}
		}
		return nil
	}

	//If we have a local relayer, add to the listener
	if r := nw.relayers[relayerMapKey(subReq.StrmID, SubReqID)]; r != nil {
		r.AddListener(nw, remotePID)
		return nil
	}

	//If we have a local subscriber (and not a relayer), create a relayer
	if s := nw.subscribers[subReq.StrmID]; s != nil {
		r := nw.NewRelayer(subReq.StrmID, SubReqID)
		r.UpstreamPeer = s.UpstreamPeer
		lpmon.Instance().LogRelay(subReq.StrmID, peer.IDHexEncode(remotePID))
		r.AddListener(nw, remotePID)
	}

	//If we don't have local broadcaster, relayer, or a subscriber, forward the sub request to the closest peer
	peers, err := nw.NetworkNode.ClosestLocalPeers(subReq.StrmID)
	if err != nil {
		glog.Errorf("Error getting closest local node: %v", err)
		return ErrHandleMsg
	}

	//Send Sub Req to the network
	for _, p := range peers {
		//Don't send it back to the requesting peer
		if p == remotePID || p == nw.NetworkNode.ID() {
			continue
		}

		if p == "" {
			glog.Errorf("Got empty peer from libp2p")
			return nil
		}

		ns := nw.NetworkNode.GetOutStream(p)
		if ns != nil {
			if err := nw.sendMessageWithRetry(p, ns, SubReqID, subReq); err != nil {
				//Question: Do we want to close the stream here?
				glog.Errorf("Error relaying subReq to %v: %v.", p, err)
				continue
			}

			if r := nw.relayers[relayerMapKey(subReq.StrmID, SubReqID)]; r != nil {
				r.AddListener(nw, remotePID)
			} else {
				glog.V(common.VERBOSE).Infof("Creating relayer for sub req")
				r := nw.NewRelayer(subReq.StrmID, SubReqID)
				r.UpstreamPeer = p
				lpmon.Instance().LogRelay(subReq.StrmID, peer.IDHexEncode(p))
				r.AddListener(nw, remotePID)
			}
			return nil
		} else {
			glog.Errorf("Cannot get stream for peer: %v", peer.IDHexEncode(p))
		}
	}

	glog.Errorf("%v Cannot forward Sub req to any of the peers: %v", nw.GetNodeID(), peers)
	return ErrHandleMsg
}

func handleCancelSubReq(nw *BasicVideoNetwork, cr CancelSubMsg, rpeer peer.ID) error {
	if b, ok := nw.broadcasters[cr.StrmID]; ok {
		//Remove from broadcast listener
		glog.V(common.DEBUG).Infof("Removing listener from broadcaster for stream: %v", cr.StrmID)
		delete(b.listeners, peer.IDHexEncode(rpeer))
		return nil
	} else if r, ok := nw.relayers[relayerMapKey(cr.StrmID, SubReqID)]; ok {
		//Remove from relayer listener
		glog.V(common.DEBUG).Infof("Removing listener from relayer for stream: %v", cr.StrmID)
		delete(r.listeners, peer.IDHexEncode(rpeer))
		lpmon.Instance().RemoveRelay(cr.StrmID)
		//Pass on the cancel req and remove relayer if relayer has no more listeners, unless we still have a subscriber - in which case, just remove the relayer.
		if len(r.listeners) == 0 {
			ns := nw.NetworkNode.GetOutStream(r.UpstreamPeer)
			if ns != nil {
				if err := nw.sendMessageWithRetry(r.UpstreamPeer, ns, CancelSubID, cr); err != nil {
					glog.Errorf("Error relaying cancel message to %v: %v ", peer.IDHexEncode(r.UpstreamPeer), err)
				}
				return nil
			}
			if _, ok := nw.subscribers[cr.StrmID]; !ok {
				delete(nw.relayers, relayerMapKey(cr.StrmID, CancelSubID))
			}
		}
		return nil
	} else {
		glog.Errorf("Cannot find broadcaster or relayer.  Error!")
		return nil //Cancel could be sent because of many reasons. (for example, Finish is sent, and at the same time, viewer cancels subscription) Let's not return an error for now.
	}
}

func handleStreamData(nw *BasicVideoNetwork, remotePID peer.ID, sd *StreamDataMsg) error {
	//A node can have a subscriber AND a relayer for the same stream.
	s := nw.getSubscriber(sd.StrmID)
	if s != nil {
		if err := s.InsertData(sd); err != nil {
			glog.Errorf("Error inserting data into subscriber: %v", err)
		}
	}

	r := nw.relayers[relayerMapKey(sd.StrmID, SubReqID)]
	if r != nil {
		if err := r.RelayStreamData(sd); err != nil {
			glog.Errorf("Error relaying stream data: %v", err)
			return ErrHandleMsg
		}
	}

	if s == nil && r == nil {
		glog.Errorf("Something is wrong.  Expect subscriber or relayer for seg:%v strm:%v to exist at this point (should have been setup when SubReq came in)", sd.SeqNo, sd.StrmID)
		return ErrHandleMsg
	}
	return nil
}

func handleFinishStream(nw *BasicVideoNetwork, fs FinishStreamMsg) error {
	//A node can have a subscriber AND a relayer for the same stream.
	s := nw.subscribers[fs.StrmID]
	if s != nil {
		//Unsubscribe, delete subscriber
		s.Unsubscribe()
		delete(nw.subscribers, fs.StrmID)
	}

	r := nw.relayers[relayerMapKey(fs.StrmID, SubReqID)]
	if r != nil {
		if err := r.RelayFinishStream(nw, fs); err != nil {
			glog.Errorf("Error relaying finish stream: %v", err)
		}
		delete(nw.relayers, relayerMapKey(fs.StrmID, SubReqID))
		lpmon.Instance().RemoveRelay(fs.StrmID)
	}

	if s == nil && r == nil {
		glog.Errorf("Error: cannot find subscriber or relayer")
		return ErrHandleMsg
	}
	return nil
}

func handleTranscodeResponse(nw *BasicVideoNetwork, remotePID peer.ID, tr TranscodeResponseMsg) error {
	glog.V(5).Infof("Transcode Result StreamIDs: %v", tr)
	callback, ok := nw.transResponseCallbacks[tr.StrmID]
	if ok {
		callback(tr.Result)
		return nil
	}

	//If we are suppose to be the broadcasting node, don't need to forward the message.
	nid, err := extractNodeID(tr.StrmID)
	if err != nil {
		return ErrHandleMsg
	}
	if peer.IDHexEncode(nid) == nw.GetNodeID() {
		return nil
	}

	//Don't have a local callback.  Forward to a peer
	peers, err := nw.NetworkNode.ClosestLocalPeers(tr.StrmID)
	if err != nil {
		return ErrTranscodeResponse
	}
	dupCount := DefaultTranscodeResponseRelayDuplication
	for _, p := range peers {
		//Don't send it back to the requesting peer
		if p == remotePID || p == nw.NetworkNode.ID() {
			continue
		}

		if p == "" {
			glog.Errorf("Got empty peer from libp2p")
			return nil
		}

		s := nw.NetworkNode.GetOutStream(p)
		if s != nil {
			if err := nw.sendMessageWithRetry(p, s, TranscodeResponseID, tr); err != nil {
				glog.Errorf("Error sending Transcoding Response Message to %v", peer.IDHexEncode(p))
				continue
			} else {
				return nil
			}

			//Don't need to set up a relayer here because we don't expect any response.
			if dupCount == 0 {
				return nil
			}
		}
		dupCount--
	}
	glog.Info("Cannot relay TranscodeResponse to peers")
	return ErrHandleMsg
}

func handleGetMasterPlaylistReq(nw *BasicVideoNetwork, remotePID peer.ID, mplr GetMasterPlaylistReqMsg) error {
	mpl, ok := nw.mplMap[mplr.ManifestID]
	if !ok {
		//This IS the node. If we can't find it here, we can't find it anywhere. (NEW YORK NEW YORK)
		if nid, err := extractNodeID(mplr.ManifestID); err == nil {
			if peer.IDHexEncode(nid) == nw.GetNodeID() {
				return nw.sendMessageWithRetry(remotePID, nw.NetworkNode.GetOutStream(remotePID), MasterPlaylistDataID, MasterPlaylistDataMsg{ManifestID: mplr.ManifestID, NotFound: true})
			}
		}

		//Don't have the playlist locally. Forward to a peer
		peers, err := nw.NetworkNode.ClosestLocalPeers(mplr.ManifestID)
		if err != nil {
			return nw.sendMessageWithRetry(remotePID, nw.NetworkNode.GetOutStream(remotePID), MasterPlaylistDataID, MasterPlaylistDataMsg{ManifestID: mplr.ManifestID, NotFound: true})
		}
		for _, p := range peers {
			//Don't send it back to the requesting peer
			if p == remotePID || p == nw.NetworkNode.ID() {
				continue
			}

			if p == "" {
				glog.Errorf("Got empty peer from libp2p")
				return nil
			}

			s := nw.NetworkNode.GetOutStream(p)
			if s != nil {
				glog.Infof("Sending msg to %v", peer.IDHexEncode(p))
				if err := nw.sendMessageWithRetry(p, s, GetMasterPlaylistReqID, GetMasterPlaylistReqMsg{ManifestID: mplr.ManifestID}); err != nil {
					continue
				}

				r, ok := nw.relayers[relayerMapKey(mplr.ManifestID, GetMasterPlaylistReqID)]
				if !ok {
					glog.V(common.VERBOSE).Infof("Creating relayer for get master playlist req")
					r = nw.NewRelayer(mplr.ManifestID, GetMasterPlaylistReqID)
					r.UpstreamPeer = p
					lpmon.Instance().LogRelay(mplr.ManifestID, peer.IDHexEncode(p))
				}
				r.AddListener(nw, remotePID)
				return nil
			}
		}
		glog.Info("Cannot relay GetMasterPlaylist req to peers")
		if err := nw.sendMessageWithRetry(remotePID, nw.NetworkNode.GetOutStream(remotePID), MasterPlaylistDataID, MasterPlaylistDataMsg{ManifestID: mplr.ManifestID, NotFound: true}); err != nil {
			glog.Errorf("Error sending MasterPlaylistData-NotFound: %v", err)
			return ErrHandleMsg
		}
		return nil
	}

	if err := nw.sendMessageWithRetry(remotePID, nw.NetworkNode.GetOutStream(remotePID), MasterPlaylistDataID, MasterPlaylistDataMsg{ManifestID: mplr.ManifestID, MPL: mpl.String()}); err != nil {
		glog.Errorf("Error sending MasterPlaylistData: %v", err)
		return ErrHandleMsg
	}
	return nil
}

func handleMasterPlaylistDataMsg(nw *BasicVideoNetwork, mpld MasterPlaylistDataMsg) error {
	ch, ok := nw.msgChans[msgChansKey(GetMasterPlaylistReqID, mpld.ManifestID)]
	if !ok {
		r := nw.relayers[relayerMapKey(mpld.ManifestID, GetMasterPlaylistReqID)]
		if r != nil {
			//Relay the data
			return r.RelayMasterPlaylistData(nw, mpld)
		} else {
			glog.Errorf("Got master playlist data, but don't have a channel")
			return ErrHandleMsg
		}
	}

	ch <- &Msg{Op: MasterPlaylistDataID, Data: mpld}
	return nil
}

func handleNodeStatusReqMsg(nw *BasicVideoNetwork, remotePID peer.ID, nsr NodeStatusReqMsg) error {
	if nsr.NodeID == nw.GetNodeID() {
		status := nw.nodeStatus().String()
		if err := nw.sendMessageWithRetry(remotePID, nw.NetworkNode.GetOutStream(remotePID), NodeStatusDataID, NodeStatusDataMsg{NodeID: nw.GetNodeID(), Data: []byte(status)}); err != nil {
			glog.Errorf("Error sending NodeStatusData: %v", err)
			return ErrHandleMsg
		}
		return nil
	} else {
		//Don't have the node status locally. Forward to a peer
		peers, err := nw.NetworkNode.ClosestLocalPeers(nsr.NodeID)
		if err != nil {
			return nw.sendMessageWithRetry(remotePID, nw.NetworkNode.GetOutStream(remotePID), NodeStatusDataID, NodeStatusDataMsg{NodeID: nsr.NodeID, NotFound: true})
		}

		for _, p := range peers {
			//Don't send it back to the requesting peer
			if p == remotePID || p == nw.NetworkNode.ID() {
				continue
			}

			if p == "" {
				glog.Errorf("Got empty peer from libp2p")
				return nil
			}

			s := nw.NetworkNode.GetOutStream(p)
			if s != nil {
				glog.Infof("Sending msg to %v", peer.IDHexEncode(p))
				if err := nw.sendMessageWithRetry(p, s, NodeStatusReqID, NodeStatusReqMsg{NodeID: nsr.NodeID}); err != nil {
					continue
				}

				r, ok := nw.relayers[relayerMapKey(nsr.NodeID, NodeStatusReqID)]
				if !ok {
					glog.V(common.VERBOSE).Infof("Creating relayer for get master playlist req")
					r = nw.NewRelayer(nsr.NodeID, NodeStatusReqID)
					r.UpstreamPeer = p
				}
				r.AddListener(nw, remotePID)
				return nil
			}
		}
		glog.Info("Cannot relay node status req to peers")
		if err := nw.sendMessageWithRetry(remotePID, nw.NetworkNode.GetOutStream(remotePID), NodeStatusDataID, NodeStatusDataMsg{NodeID: nsr.NodeID, NotFound: true}); err != nil {
			glog.Errorf("Error sending MasterPlaylistData-NotFound: %v", err)
			return ErrHandleMsg
		}
		return nil
	}
	return nil
}

func handleNodeStatusDataMsg(nw *BasicVideoNetwork, nsd NodeStatusDataMsg) error {
	ch, ok := nw.msgChans[msgChansKey(NodeStatusReqID, nsd.NodeID)]
	if !ok {
		r := nw.relayers[relayerMapKey(nsd.NodeID, NodeStatusReqID)]
		if r != nil {
			return r.RelayNodeStatusData(nw, nsd)
		} else {
			glog.Errorf("Got node status data, but don't have a channel")
			return ErrHandleMsg
		}
	}

	ch <- &Msg{Op: NodeStatusDataID, Data: nsd}
	return nil
}

func handlePing(nw *BasicVideoNetwork, remotePID peer.ID, pd PingDataMsg) error {
	if err := nw.NetworkNode.GetOutStream(remotePID).SendMessage(PongID, PongDataMsg(pd)); err != nil {
		glog.Errorf("Error sending Pong: %v", err)
		return ErrHandleMsg
	}
	return nil
}

func handlePong(nw *BasicVideoNetwork, remotePID peer.ID, pd PongDataMsg) error {
	if nw.pingChs[string(pd)] == nil {
		glog.Errorf("Error handling Pong - cannot find pingCh")
		return ErrHandleMsg
	}
	nw.pingChs[string(pd)] <- struct{}{}
	close(nw.pingChs[string(pd)])
	delete(nw.pingChs, string(pd))
	return nil
}

func extractNodeID(strmOrManifestID string) (peer.ID, error) {
	if len(strmOrManifestID) < 68 {
		return "", ErrProtocol
	}

	nid := strmOrManifestID[:68]
	return peer.IDHexDecode(nid)
}

type relayerID string

func relayerMapKey(strmID string, opcode Opcode) relayerID {
	return relayerID(fmt.Sprintf("%v-%v", opcode, strmID))
}

func msgChansKey(opcode Opcode, key string) string {
	return fmt.Sprintf("%v|%v", opcode, key)
}

func (n *BasicVideoNetwork) sendMessageWithRetry(pid peer.ID, strm OutStream, op Opcode, msg interface{}) error {
	if strm == nil {
		return ErrSendMsg
	}
	if err := strm.SendMessage(op, msg); err != nil {
		newStrm := n.NetworkNode.RefreshOutStream(pid)
		if err := newStrm.SendMessage(op, msg); err != nil {
			return err
		}
	}

	return nil
}

func randStr() string {
	rand.Seed(time.Now().UnixNano())
	x := make([]byte, 10, 10)
	for i := 0; i < len(x); i++ {
		x[i] = byte(rand.Uint32())
	}
	return fmt.Sprintf("%x", x)
}
