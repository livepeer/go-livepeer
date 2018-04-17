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
	lastMsgs     []*StreamDataMsg
	q            chan *StreamDataMsg
	listeners    map[string]OutStream
	StrmID       string
	working      bool
	cancelWorker context.CancelFunc
}

//Broadcast sends a video chunk to the stream.  The very first call to Broadcast kicks off a worker routine to do the broadcasting.
func (b *BasicBroadcaster) Broadcast(seqNo uint64, data []byte) error {
	//This should only get invoked once per broadcaster
	if b.working == false {
		ctxB, cancel := context.WithCancel(context.Background())
		b.cancelWorker = cancel
		go b.broadcastToListeners(ctxB)
		b.working = true
	}

	latest := &StreamDataMsg{SeqNo: seqNo, Data: data}
	b.lastMsgs = append(b.lastMsgs, latest)
	b.lastMsgs = b.lastMsgs[1:]
	b.q <- latest
	return nil
}

//Finish signals the stream is finished.  It cancels the broadcasting worker routine and sends the Finish message to all the listeners.
func (b *BasicBroadcaster) Finish() error {
	//Cancel worker
	if b.cancelWorker != nil {
		b.cancelWorker()
	}

	//Send Finish to all the listeners
	for _, l := range b.listeners {
		glog.V(5).Infof("Broadcasting finish")
		if err := b.Network.sendMessageWithRetry(l.GetRemotePeer(), l, FinishStreamID, FinishStreamMsg{StrmID: b.StrmID}); err != nil {
			glog.Errorf("Error broadcasting finish: %v", err)
		}
	}

	//Delete the broadcaster
	delete(b.Network.broadcasters, b.StrmID)

	//TODO: Need to figure out a place to close the stream listeners
	return nil
}

func (br *BasicBroadcaster) AddListeningPeer(nw *BasicVideoNetwork, pid peer.ID) {
	key := peer.IDHexEncode(pid)
	if _, ok := br.listeners[key]; !ok {
		br.listeners[key] = nw.NetworkNode.GetOutStream(pid)
	}
}

func (br *BasicBroadcaster) AddListeningStream(key string, os OutStream) {
	if _, ok := br.listeners[key]; !ok {
		br.listeners[key] = os
	}
}

func (b *BasicBroadcaster) broadcastToListeners(ctx context.Context) {
	for {
		select {
		case msg := <-b.q:
			for id, l := range b.listeners {
				// glog.Infof("Broadcasting segment %v to listener %v", msg.SeqNo, id)
				b.sendDataMsg(id, l, msg)
			}
		case <-ctx.Done():
			glog.V(5).Infof("broadcast worker done")
			return
		}
	}
}

func (b *BasicBroadcaster) sendDataMsg(lid string, l OutStream, msg *StreamDataMsg) {
	if msg == nil {
		return
	}

	if err := b.Network.sendMessageWithRetry(l.GetRemotePeer(), l, StreamDataID, StreamDataMsg{SeqNo: msg.SeqNo, StrmID: b.StrmID, Data: msg.Data}); err != nil {
		glog.Errorf("Error broadcasting segment %v to listener %v: %v", msg.SeqNo, lid, err)
		delete(b.listeners, lid)
	}
}

func (b BasicBroadcaster) String() string {
	return fmt.Sprintf("StreamID: %v, working: %v, q: %v, listeners: %v", b.StrmID, b.working, len(b.q), len(b.listeners))
}

func (b *BasicBroadcaster) IsLive() bool {
	return b.working
}
