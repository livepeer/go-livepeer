package media

import (
	"io"
	"sync"
)

// use this to set peerconnection state when we need it
// kinda convoluted because the peerconnection gets created async
// separate from the goroutines where it might be closed from
type WHIPConnection struct {
	mu     *sync.Mutex
	cond   *sync.Cond
	peer   *MediaState
	closed bool
}

func NewWHIPConnection() *WHIPConnection {
	mu := &sync.Mutex{}
	return &WHIPConnection{
		mu:   mu,
		cond: sync.NewCond(mu),
	}
}

func (w *WHIPConnection) SetWHIPConnection(p *MediaState) {
	w.mu.Lock()
	defer w.mu.Unlock()
	// immediately close if the peer is nil
	if p == nil {
		w.closed = true
	}
	// don't overwrite existing peers
	if w.peer == nil {
		w.peer = p
	}
	w.cond.Broadcast()
}

func (w *WHIPConnection) getWHIPConnection() *MediaState {
	w.mu.Lock()
	defer w.mu.Unlock()
	for w.peer == nil && !w.closed {
		w.cond.Wait()
	}
	return w.peer
}

func (w *WHIPConnection) AwaitClose() {
	p := w.getWHIPConnection()
	if p == nil {
		return
	}
	p.AwaitClose()
}

func (w *WHIPConnection) Close() {
	w.mu.Lock()
	// set closed = true so getWHIPConnection returns immediately
	// if we don't already have a peer - avoids a deadlock
	w.closed = true
	w.cond.Broadcast()
	w.mu.Unlock()

	p := w.getWHIPConnection()
	if p != nil {
		p.Close()
	}
}

type WHIPPeerConnection interface {
	io.Closer
}

// MediaState manages the lifecycle of a media connection
type MediaState struct {
	pc      WHIPPeerConnection
	closeCh chan bool
	once    sync.Once
}

// NewMediaState creates a new MediaState with the given peerconnection
func NewMediaState(pc WHIPPeerConnection) *MediaState {
	return &MediaState{
		pc:      pc,
		closeCh: make(chan bool),
	}
}

// Close closes the underlying connection and signals any waiters
func (m *MediaState) Close() {
	m.once.Do(func() {
		if m.pc != nil {
			m.pc.Close()
		}
		close(m.closeCh)
	})
}

// AwaitClose blocks until the connection is closed
func (m *MediaState) AwaitClose() {
	<-m.closeCh
}
