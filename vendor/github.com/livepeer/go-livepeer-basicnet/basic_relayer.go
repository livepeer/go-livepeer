package basicnet

import (
	"fmt"
	"time"

	peer "gx/ipfs/QmXYjuNuxVzXKJCfWasQk1RqkhVLDM9jtUKhqc2WPQmFSB/go-libp2p-peer"

	"github.com/golang/glog"
)

//BasicRelayer relays video segments to listners.  Unlike BasicBroadcaster, BasicRelayer is
//does NOT have a worker - it sends out the chunks to its listeners as soon as it gets one from the network.
type BasicRelayer struct {
	Network      *BasicVideoNetwork
	UpstreamPeer peer.ID
	listeners    map[string]*BasicOutStream
	LastRelay    time.Time
}

//RelayStreamData sends a StreamDataMsg to its listeners
func (br *BasicRelayer) RelayStreamData(sd *StreamDataMsg) error {
	for strmID, l := range br.listeners {
		// glog.V(5).Infof("Relaying stream data to listener: %v", l)
		// glog.Infof("Relaying stream data to listener: %v.", peer.IDHexEncode(l.Stream.Conn().RemotePeer()))
		if err := br.Network.sendMessageWithRetry(l.Stream.Conn().RemotePeer(), l, StreamDataID, *sd); err != nil {
			glog.Errorf("Error writing data to relayer listener %v: %v", l, err)
			delete(br.listeners, strmID)
		}
		br.LastRelay = time.Now()
	}
	return nil
}

func (br *BasicRelayer) RelayFinishStream(nw *BasicVideoNetwork, fs FinishStreamMsg) error {
	for strmID, l := range br.listeners {
		if err := br.Network.sendMessageWithRetry(l.Stream.Conn().RemotePeer(), l, FinishStreamID, fs); err != nil {
			glog.Errorf("Error relaying finish stream to %v: %v", peer.IDHexEncode(l.Stream.Conn().RemotePeer()), err)
			delete(br.listeners, strmID)
		}
		br.LastRelay = time.Now()
	}
	return nil
}

func (br *BasicRelayer) RelayMasterPlaylistData(nw *BasicVideoNetwork, mpld MasterPlaylistDataMsg) error {
	for strmID, l := range br.listeners {
		if err := br.Network.sendMessageWithRetry(l.Stream.Conn().RemotePeer(), l, MasterPlaylistDataID, mpld); err != nil {
			glog.Errorf("Error relaying master playlist data to %v: %v", peer.IDHexEncode(l.Stream.Conn().RemotePeer()), err)
			delete(br.listeners, strmID)
		}
		br.LastRelay = time.Now()
	}
	return nil
}

func (br *BasicRelayer) RelayNodeStatusData(nw *BasicVideoNetwork, nsd NodeStatusDataMsg) error {
	for id, l := range br.listeners {
		if err := br.Network.sendMessageWithRetry(l.Stream.Conn().RemotePeer(), l, NodeStatusDataID, nsd); err != nil {
			glog.Errorf("Error relaying node status data to %v: %v", peer.IDHexEncode(l.Stream.Conn().RemotePeer()), err)
			delete(br.listeners, id)
		}
		br.LastRelay = time.Now()
	}
	return nil
}

func (br *BasicRelayer) AddListener(nw *BasicVideoNetwork, pid peer.ID) {
	key := peer.IDHexEncode(pid)
	if _, ok := br.listeners[key]; !ok {
		br.listeners[key] = nw.NetworkNode.GetOutStream(pid)
	}
}

func (br BasicRelayer) String() string {
	return fmt.Sprintf("UpstreamPeer: %v, len:%v", peer.IDHexEncode(br.UpstreamPeer), len(br.listeners))
}
