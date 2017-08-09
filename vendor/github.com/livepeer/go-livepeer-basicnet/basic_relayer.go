package basicnet

import (
	"fmt"
	peer "gx/ipfs/QmXYjuNuxVzXKJCfWasQk1RqkhVLDM9jtUKhqc2WPQmFSB/go-libp2p-peer"

	"github.com/golang/glog"
)

//BasicRelayer relays video segments to listners.  Unlike BasicBroadcaster, BasicRelayer is
//does NOT have a worker - it sends out the chunks to its listeners as soon as it gets one from the network.
type BasicRelayer struct {
	UpstreamPeer peer.ID
	listeners    map[string]*BasicStream
}

//RelayStreamData sends a StreamDataMsg to its listeners
func (br *BasicRelayer) RelayStreamData(sd StreamDataMsg) error {
	for _, l := range br.listeners {
		// glog.Infof("Relaying stream data to listener: %v", l)
		if err := l.SendMessage(StreamDataID, sd); err != nil {
			glog.Errorf("Error writing data to relayer listener %v: %v", l, err)
		}
	}
	return nil
}

func (br *BasicRelayer) RelayFinishStream(nw *BasicVideoNetwork, fs FinishStreamMsg) error {
	for _, l := range br.listeners {
		if err := l.SendMessage(FinishStreamID, fs); err != nil {
			glog.Errorf("Error relaying finish stream to %v: %v", peer.IDHexEncode(l.Stream.Conn().RemotePeer()), err)
		}
	}
	return nil
}

func (br BasicRelayer) String() string {
	return fmt.Sprintf("UpstreamPeer: %v", peer.IDHexEncode(br.UpstreamPeer))
}
