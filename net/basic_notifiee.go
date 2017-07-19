package net

import (
	"github.com/golang/glog"
	net "github.com/libp2p/go-libp2p-net"
	peer "github.com/libp2p/go-libp2p-peer"
	ma "github.com/multiformats/go-multiaddr"
)

type BasicNotifiee struct{}

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
}

// called when a connection closed
func (bn *BasicNotifiee) Disconnected(n net.Network, conn net.Conn) {
	glog.Infof("Notifiee - Disconnected")
}

// called when a stream opened
func (bn *BasicNotifiee) OpenedStream(n net.Network, s net.Stream) {
	// glog.Infof("Notifiee - OpenedStream: %v - %v", peer.IDHexEncode(s.Conn().LocalPeer()), peer.IDHexEncode(s.Conn().RemotePeer()))
}

// called when a stream closed
func (bn *BasicNotifiee) ClosedStream(n net.Network, s net.Stream) {
	// glog.Infof("Notifiee - ClosedStream: %v - %v", peer.IDHexEncode(s.Conn().LocalPeer()), peer.IDHexEncode(s.Conn().RemotePeer()))
}
