package dconn

import (
	"io"
	"net"
	"time"

	iconn "gx/ipfs/QmToCvh5eJtoDheMggre7b2zeFCJ6tAyB82YVs457cqoUE/go-libp2p-interface-conn"
	tpt "gx/ipfs/QmVxtCwKFMmwcjhQXsGj6m4JAW7nGb9hRoErH9jpgqcLxA/go-libp2p-transport"
	ma "gx/ipfs/QmWWQ2Txc2c6tqjsBpzg5Ar652cHPGNsQQp2SejkNmkUMb/go-multiaddr"
	peer "gx/ipfs/QmZoWKhxUmZ2seW4BzX6fJkNR8hh9PsGModr7q171yq2SS/go-libp2p-peer"
	ic "gx/ipfs/QmaPbCnUMBohSGo3KnxEa2bHqyJVVeEEcwtqJAYxerieBo/go-libp2p-crypto"
	"gx/ipfs/QmdaFHcDk53RWnq4R6wH2Uy5YuBmvbSWLK8gFhQwqU3jZH/bufpipe"
)

func NewDummyConnPair() (conn1 iconn.Conn, conn2 iconn.Conn, err error) {
	pipe1 := bufpipe.NewBufferedPipe(1 << 20)
	pipe2 := bufpipe.NewBufferedPipe(1 << 20)

	conn1 = &dummyconn{pipe1, pipe2}
	conn2 = &dummyconn{pipe2, pipe1}
	return
}

type dummyconn struct {
	pipeR *bufpipe.Pipe
	pipeW *bufpipe.Pipe
}

var _ iconn.Conn = (*dummyconn)(nil)

func (d *dummyconn) Close() error {
	d.pipeW.Close(io.ErrClosedPipe)
	return nil
}

func (d *dummyconn) Read(p []byte) (n int, err error) {
	return d.pipeR.Read(p)
}

func (d *dummyconn) Write(p []byte) (n int, err error) {
	return d.pipeW.Write(p)
}

func (*dummyconn) LocalPeer() peer.ID {
	panic("not implemented")
}

func (*dummyconn) Transport() tpt.Transport {
	panic("not implemented")
}

func (*dummyconn) LocalPrivateKey() ic.PrivKey {
	panic("not implemented")
}

func (*dummyconn) LocalMultiaddr() ma.Multiaddr {
	panic("not implemented")
}

func (*dummyconn) RemotePeer() peer.ID {
	panic("not implemented")
}

func (*dummyconn) RemotePublicKey() ic.PubKey {
	panic("not implemented")
}

func (*dummyconn) RemoteMultiaddr() ma.Multiaddr {
	panic("not implemented")
}

func (*dummyconn) ID() string {
	panic("not implemented")
}

func (*dummyconn) LocalAddr() net.Addr {
	panic("not implemented")
}

func (*dummyconn) RemoteAddr() net.Addr {
	panic("not implemented")
}

func (*dummyconn) SetDeadline(t time.Time) error {
	panic("not implemented")
}

func (*dummyconn) SetReadDeadline(t time.Time) error {
	panic("not implemented")
}

func (*dummyconn) SetWriteDeadline(t time.Time) error {
	panic("not implemented")
}
