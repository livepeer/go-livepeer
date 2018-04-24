package basicnet

import (
	ma "gx/ipfs/QmWWQ2Txc2c6tqjsBpzg5Ar652cHPGNsQQp2SejkNmkUMb/go-multiaddr"
	net "gx/ipfs/QmXfkENeeBvh3zYA51MaSdGUdBjhQ99cP5WQe8zgr6wchG/go-libp2p-net"
	peer "gx/ipfs/QmZoWKhxUmZ2seW4BzX6fJkNR8hh9PsGModr7q171yq2SS/go-libp2p-peer"

	"github.com/golang/glog"
	lpmon "github.com/livepeer/go-livepeer/monitor"
)

//BasicNotifiee gets called during important libp2p events
type BasicNotifiee struct {
	monitor           *lpmon.Monitor
	disconnectHandler func(pid peer.ID)
}

func NewBasicNotifiee(mon *lpmon.Monitor) *BasicNotifiee {
	return &BasicNotifiee{monitor: mon}
}

// called when network starts listening on an addr
func (bn *BasicNotifiee) Listen(n net.Network, addr ma.Multiaddr) {
	glog.V(4).Infof("Notifiee - Listen: %v", addr)
}

// called when network starts listening on an addr
func (bn *BasicNotifiee) ListenClose(n net.Network, addr ma.Multiaddr) {
	glog.V(4).Infof("Notifiee - Close: %v", addr)
}

// called when a connection opened
func (bn *BasicNotifiee) Connected(n net.Network, conn net.Conn) {
	glog.V(4).Infof("Notifiee - Connected.  Local: %v - Remote: %v", peer.IDHexEncode(conn.LocalPeer()), peer.IDHexEncode(conn.RemotePeer()))
	if bn.monitor != nil {
		bn.monitor.LogNewConn(peer.IDHexEncode(conn.LocalPeer()), peer.IDHexEncode(conn.RemotePeer()))
	}
}

// called when a connection closed
func (bn *BasicNotifiee) Disconnected(n net.Network, conn net.Conn) {
	glog.V(4).Infof("Notifiee - Disconnected. Local: %v - Remote: %v", peer.IDHexEncode(conn.LocalPeer()), peer.IDHexEncode(conn.RemotePeer()))
	if bn.monitor != nil {
		bn.monitor.RemoveConn(peer.IDHexEncode(conn.LocalPeer()), peer.IDHexEncode(conn.RemotePeer()))
	}
	if bn.disconnectHandler != nil {
		bn.disconnectHandler(conn.RemotePeer())
	}
}

func (bn *BasicNotifiee) HandleDisconnect(h func(pid peer.ID)) {
	bn.disconnectHandler = h
}

// called when a stream opened
func (bn *BasicNotifiee) OpenedStream(n net.Network, s net.Stream) {
	glog.V(4).Infof("Notifiee - OpenedStream: %v - %v", peer.IDHexEncode(s.Conn().LocalPeer()), peer.IDHexEncode(s.Conn().RemotePeer()))
}

// called when a stream closed
func (bn *BasicNotifiee) ClosedStream(n net.Network, s net.Stream) {
	glog.V(4).Infof("Notifiee - ClosedStream: %v - %v", peer.IDHexEncode(s.Conn().LocalPeer()), peer.IDHexEncode(s.Conn().RemotePeer()))
}
