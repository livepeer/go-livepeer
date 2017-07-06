package net

import (
	"github.com/golang/glog"
	peer "github.com/libp2p/go-libp2p-peer"
)

//BasicRelayer relays video segments to listners.  Unlike BasicBroadcaster, BasicRelayer is
//synchronous - it sends out the chunks to its listeners as soon as it gets one from the network.
type BasicRelayer struct {
	UpstreamPeer peer.ID
	listeners    map[string]VideoMuxer
}

func (br *BasicRelayer) RelayStreamData(sd StreamDataMsg) error {
	for _, l := range br.listeners {
		glog.Infof("Relaying stream data to listener: %v", l)
		err := l.WriteSegment(sd.SeqNo, sd.StrmID, sd.Data)
		if err != nil {
			glog.Errorf("Error writing data to relayer listener %v: %v", l, err)
		}
	}
	return nil
}

func (br *BasicRelayer) RelayFinishStream(nw *BasicVideoNetwork, fs FinishStreamMsg) error {
	for _, l := range br.listeners {
		bs, ok := l.(*BasicStream)
		if ok {
			err := nw.NetworkNode.SendMessage(bs, bs.Stream.Conn().RemotePeer(), FinishStreamID, fs)
			if err != nil {
				glog.Errorf("Error relaying finish stream to %v: %v", peer.IDHexEncode(bs.Stream.Conn().RemotePeer()), err)
			}
		}
	}
	return nil
}
