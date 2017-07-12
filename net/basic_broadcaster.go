package net

import (
	"context"

	"github.com/golang/glog"
	peer "github.com/libp2p/go-libp2p-peer"
)

//BasicBroadcaster keeps track of a list of listeners and a queue of video chunks.  It doesn't start keeping track of things until there is at least 1 listner.
type BasicBroadcaster struct {
	Network *BasicVideoNetwork
	// host    host.Host
	// q    *list.List
	// lock *sync.Mutex
	q chan *StreamDataMsg

	listeners     map[string]VideoMuxer //A VideoMuxer can be a BasicStream
	StrmID        string
	TranscodedIDs map[string]VideoProfile
	working       bool
	cancelWorker  context.CancelFunc
}

//Broadcast sends a video chunk to the stream
func (b *BasicBroadcaster) Broadcast(seqNo uint64, data []byte) error {
	// glog.Infof("Broadcasting data: %v (%v), storing in q: %v", seqNo, len(data), b)
	// b.q.PushBack(&StreamDataMsg{SeqNo: seqNo, Data: data})

	//This should only get invoked once per broadcaster
	if b.working == false {
		ctxB, cancel := context.WithCancel(context.Background())
		b.cancelWorker = cancel
		go b.broadcastToListeners(ctxB)
		b.working = true
	}

	b.q <- &StreamDataMsg{SeqNo: seqNo, Data: data}
	return nil
}

//Finish signals the stream is finished
func (b *BasicBroadcaster) Finish() error {
	//Cancel worker
	b.cancelWorker()

	//Send Finish to all the listeners
	for _, l := range b.listeners {
		ws, ok := l.(*BasicStream)
		if ok {
			glog.Infof("Broadcasting finish to %v", peer.IDHexEncode(ws.Stream.Conn().RemotePeer()))
			if err := b.Network.NetworkNode.SendMessage(ws, ws.Stream.Conn().RemotePeer(), FinishStreamID, FinishStreamMsg{StrmID: b.StrmID}); err != nil {
				glog.Errorf("Error broadcasting finish to listener %v: %v", peer.IDHexEncode(ws.Stream.Conn().RemotePeer()), err)
			}
		}
	}

	//Delete the broadcaster
	delete(b.Network.broadcasters, b.StrmID)

	//TODO: Need to figure out a place to close the stream listeners
	return nil
}

func (b *BasicBroadcaster) broadcastToListeners(ctx context.Context) {
	for {
		select {
		case msg := <-b.q:
			// glog.Infof("broadcasting msg:%v to network.  listeners: %v", msg, b.listeners)
			for id, l := range b.listeners {
				glog.Infof("Broadcasting segment %v to listener %v", msg.SeqNo, id)
				err := l.WriteSegment(msg.SeqNo, b.StrmID, msg.Data)
				if err != nil {
					glog.Errorf("Error broadcasting segment %v to listener %v: %v", msg.SeqNo, id, err)
					delete(b.listeners, id)
				}
			}
		case <-ctx.Done():
			glog.Infof("broadcast worker done")
			return
		}
	}

}
