package basicnet

import (
	ma "gx/ipfs/QmXY77cVe7rVRQXZZQRioukUM7aRW3BTcAgJe12MCtb3Ji/go-multiaddr"
	peer "gx/ipfs/QmXYjuNuxVzXKJCfWasQk1RqkhVLDM9jtUKhqc2WPQmFSB/go-libp2p-peer"
	net "gx/ipfs/QmahYsGWry85Y7WUe2SX5G4JkH2zifEQAUtJVLZ24aC9DF/go-libp2p-net"

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
	glog.Infof("Notifiee - Listen: %v", addr)
}

// called when network starts listening on an addr
func (bn *BasicNotifiee) ListenClose(n net.Network, addr ma.Multiaddr) {
	glog.Infof("Notifiee - Close: %v", addr)
}

// called when a connection opened
func (bn *BasicNotifiee) Connected(n net.Network, conn net.Conn) {
	glog.Infof("Notifiee - Connected.  Local: %v - Remote: %v", peer.IDHexEncode(conn.LocalPeer()), peer.IDHexEncode(conn.RemotePeer()))
	if bn.monitor != nil {
		bn.monitor.LogNewConn(peer.IDHexEncode(conn.LocalPeer()), peer.IDHexEncode(conn.RemotePeer()))
	}
}

// called when a connection closed
func (bn *BasicNotifiee) Disconnected(n net.Network, conn net.Conn) {
	glog.Infof("Notifiee - Disconnected. Local: %v - Remote: %v", peer.IDHexEncode(conn.LocalPeer()), peer.IDHexEncode(conn.RemotePeer()))
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
	// glog.Infof("Notifiee - OpenedStream: %v - %v", peer.IDHexEncode(s.Conn().LocalPeer()), peer.IDHexEncode(s.Conn().RemotePeer()))
}

// called when a stream closed
func (bn *BasicNotifiee) ClosedStream(n net.Network, s net.Stream) {
	// glog.Infof("Notifiee - ClosedStream: %v - %v", peer.IDHexEncode(s.Conn().LocalPeer()), peer.IDHexEncode(s.Conn().RemotePeer()))
}
