package basicnet

import (
	"context"
	"fmt"

	peer "gx/ipfs/QmXYjuNuxVzXKJCfWasQk1RqkhVLDM9jtUKhqc2WPQmFSB/go-libp2p-peer"

	"github.com/golang/glog"
)

//BasicBroadcaster is unique for a specific video stream. It keeps track of a list of listeners and a queue of video chunks.  It won't start keeping track of things until there is at least 1 listener.
type BasicBroadcaster struct {
	Network      *BasicVideoNetwork
	q            chan *StreamDataMsg
	listeners    map[string]*BasicStream
	StrmID       string
	working      bool
	cancelWorker context.CancelFunc
}

//Broadcast sends a video chunk to the stream.  The very first call to Broadcast kicks off a worker routine to do the broadcasting.
func (b *BasicBroadcaster) Broadcast(seqNo uint64, data []byte) error {
	// glog.Infof("Broadcasting data: %v (%v), storing in q: %v", seqNo, len(data), b)

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

//Finish signals the stream is finished.  It cancels the broadcasting worker routine and sends the Finish message to all the listeners.
func (b *BasicBroadcaster) Finish() error {
	//Cancel worker
	b.cancelWorker()

	//Send Finish to all the listeners
	for _, l := range b.listeners {
		glog.Infof("Broadcasting finish to %v", peer.IDHexEncode(l.Stream.Conn().RemotePeer()))
		if err := l.SendMessage(FinishStreamID, FinishStreamMsg{StrmID: b.StrmID}); err != nil {
			glog.Errorf("Error broadcasting finish to listener %v: %v", peer.IDHexEncode(l.Stream.Conn().RemotePeer()), err)
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
				// glog.Infof("Broadcasting segment %v to listener %v", msg.SeqNo, id)
				if err := l.SendMessage(StreamDataID, StreamDataMsg{SeqNo: msg.SeqNo, StrmID: b.StrmID, Data: msg.Data}); err != nil {
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

func (b BasicBroadcaster) String() string {
	return fmt.Sprintf("StreamID: %v, working: %v, q: %v, listeners: %v", b.StrmID, b.working, len(b.q), len(b.listeners))
}

func (b *BasicBroadcaster) IsWorking() bool {
	return b.working
}
