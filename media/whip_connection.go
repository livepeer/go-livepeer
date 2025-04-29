package media

import (
	"errors"
	"io"
	"sync"
	"time"

	"github.com/pion/interceptor/pkg/stats"
	"github.com/pion/webrtc/v4"
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

func (w *WHIPConnection) AwaitClose() error {
	p := w.getWHIPConnection()
	if p == nil {
		return nil
	}
	return p.AwaitClose()
}

func (w *WHIPConnection) Stats() (*MediaStats, error) {
	p := w.getWHIPConnection()
	if p == nil {
		return nil, errors.New("whip connection was nil")
	}
	return p.Stats()
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
	GetStats() webrtc.StatsReport
}

type PeerConnStats struct {
	ID              string
	BytesReceived   uint64
	BytesSent       uint64
	PacketsReceived uint32
	PacketsSent     uint32
}

type TrackStats struct {
	Kind            webrtc.RTPCodecType
	Jitter          float64
	PacketsLost     int64
	PacketsReceived uint64
	RTT             time.Duration
}

type MediaStats struct {
	PeerConnStats PeerConnStats
	TrackStats    []TrackStats
}

// MediaState manages the lifecycle of a media connection
type MediaState struct {
	pc     WHIPPeerConnection
	getter stats.Getter
	tracks []RTPTrack
	mu     *sync.Mutex
	cond   *sync.Cond
	closed bool
	err    error
}

// NewMediaState creates a new MediaState with the given peerconnection
func NewMediaState(pc WHIPPeerConnection) *MediaState {
	mu := &sync.Mutex{}
	return &MediaState{
		pc:   pc,
		mu:   mu,
		cond: sync.NewCond(mu),
	}
}

func (m *MediaState) SetTracks(getter stats.Getter, tracks []RTPTrack) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.getter = getter
	m.tracks = tracks
}

// Returns a mediastate that is already closed with an error
func NewMediaStateError(err error) *MediaState {
	m := NewMediaState(nil)
	m.CloseError(err)
	return m
}

// Close closes the underlying connection and signals any waiters
func (m *MediaState) Close() {
	m.CloseError(nil)
}

func (m *MediaState) CloseError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return
	}
	if m.pc != nil {
		m.pc.Close()
	}
	m.closed = true
	m.err = err
	m.cond.Broadcast()
}

// AwaitClose blocks until the connection is closed
func (m *MediaState) AwaitClose() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for !m.closed {
		m.cond.Wait()
	}
	return m.err
}

func (m *MediaState) IsClosed() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.closed
}

func (m *MediaState) Stats() (*MediaStats, error) {
	m.mu.Lock()
	if m.closed || m.pc == nil {
		m.mu.Unlock()
		return nil, errors.New("peerconnection closed")
	}
	var (
		pc     = m.pc
		getter = m.getter
		tracks = m.tracks
	)
	m.mu.Unlock()
	pcStatsReport := pc.GetStats()
	var pcStats PeerConnStats
	for _, stat := range pcStatsReport {
		if s, ok := stat.(webrtc.TransportStats); ok {
			pcStats = PeerConnStats{
				ID:              s.ID,
				BytesReceived:   s.BytesReceived,
				BytesSent:       s.BytesSent,
				PacketsReceived: s.PacketsReceived,
				PacketsSent:     s.PacketsSent,
			}
			break
		}
	}

	if getter == nil {
		// tracks haven't been initialized yet
		return &MediaStats{
			PeerConnStats: pcStats,
		}, nil
	}
	trackStats := make([]TrackStats, 0, len(tracks))
	for _, t := range tracks {
		s := getter.Get(uint32(t.SSRC()))
		if s == nil {
			continue
		}
		trackStats = append(trackStats, TrackStats{
			Kind:            t.Kind(),
			Jitter:          s.InboundRTPStreamStats.Jitter,
			PacketsLost:     s.InboundRTPStreamStats.PacketsLost,
			PacketsReceived: s.InboundRTPStreamStats.PacketsReceived,
			RTT:             s.RemoteInboundRTPStreamStats.RoundTripTime,
		})
	}
	return &MediaStats{
		PeerConnStats: pcStats,
		TrackStats:    trackStats,
	}, nil
}
