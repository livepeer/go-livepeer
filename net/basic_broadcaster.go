package net

import (
	"container/list"
	"context"
	"sync"
	"time"

	"github.com/golang/glog"
)

//BasicBroadcaster keeps track of a list of listeners and a queue of video chunks.  It doesn't start keeping track of things until there is at least 1 listner.
type BasicBroadcaster struct {
	Network *BasicVideoNetwork
	// host    host.Host
	q    *list.List
	lock *sync.Mutex

	listeners    map[string]VideoMuxer //A VideoMuxer can be a BasicStream
	StrmID       string
	working      bool
	cancelWorker context.CancelFunc
}

//Broadcast sends a video chunk to the stream
func (b *BasicBroadcaster) Broadcast(seqNo uint64, data []byte) error {
	glog.Infof("Broadcasting data: %v (%v), storing in q: %v", seqNo, len(data), b)
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

			// glog.Infof("broadcasting msg:%v to network.  listeners: %v", msg, b.listeners)
			for id, l := range b.listeners {
				err := l.WriteSegment(msg.SeqNo, b.StrmID, msg.Data)
				if err != nil {
					delete(b.listeners, id)
				}
			}
		}
	}()
	select {
	case <-ctx.Done():
		return
	}
}
