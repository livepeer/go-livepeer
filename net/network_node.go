package net

import (
	"context"
	"fmt"

	"github.com/golang/glog"
	ds "github.com/ipfs/go-datastore"
	crypto "github.com/libp2p/go-libp2p-crypto"
	host "github.com/libp2p/go-libp2p-host"
	kad "github.com/libp2p/go-libp2p-kad-dht"
	peer "github.com/libp2p/go-libp2p-peer"
	peerstore "github.com/libp2p/go-libp2p-peerstore"
	swarm "github.com/libp2p/go-libp2p-swarm"
	bhost "github.com/libp2p/go-libp2p/p2p/host/basic"
	rhost "github.com/libp2p/go-libp2p/p2p/host/routed"
	ma "github.com/multiformats/go-multiaddr"
)

type NetworkNode struct {
	Identity peer.ID // the local node's identity
	Kad      *kad.IpfsDHT
	PeerHost host.Host // the network host (server+client)
	streams  map[peer.ID]*BasicStream
}

//NewNode creates a new Livepeerd node.
func NewNode(listenPort int, priv crypto.PrivKey, pub crypto.PubKey) (*NetworkNode, error) {
	pid, err := peer.IDFromPublicKey(pub)
	if err != nil {
		return nil, err
	}

	// Create a multiaddress
	addr, err := ma.NewMultiaddr(fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", listenPort))
	if err != nil {
		return nil, err
	}

	// Create a peerstore
	store := peerstore.NewPeerstore()
	store.AddPrivKey(pid, priv)
	store.AddPubKey(pid, pub)

	// Create swarm (implements libP2P Network)
	netwrk, err := swarm.NewNetwork(
		context.Background(),
		[]ma.Multiaddr{addr},
		pid,
		store,
		&BasicReporter{})

	netwrk.Notify(&BasicNotifiee{})
	basicHost := bhost.New(netwrk)

	dht, err := constructDHTRouting(context.Background(), basicHost, ds.NewMapDatastore())
	rHost := rhost.Wrap(basicHost, dht)

	glog.Infof("Created node: %v at %v", peer.IDHexEncode(rHost.ID()), rHost.Addrs())
	return &NetworkNode{Identity: pid, Kad: dht, PeerHost: rHost, streams: make(map[peer.ID]*BasicStream)}, nil
}

func constructDHTRouting(ctx context.Context, host host.Host, dstore ds.Batching) (*kad.IpfsDHT, error) {
	dhtRouting := kad.NewDHT(ctx, host, dstore)
	if err := dhtRouting.Bootstrap(context.Background()); err != nil {
		glog.Errorf("Error bootstraping dht: %v", err)
	}
	return dhtRouting, nil
}

func (n *NetworkNode) GetStream(pid peer.ID) *BasicStream {
	if n.streams[pid] == nil {
		glog.Infof("Creating stream from %v to %v", peer.IDHexEncode(n.Identity), peer.IDHexEncode(pid))
		ns, err := n.PeerHost.NewStream(context.Background(), pid, Protocol)
		if err != nil {
			glog.Errorf("Error creating stream: %v", err)
			return nil
		}
		n.streams[pid] = NewBasicStream(ns)
	}
	return n.streams[pid]
}

func (n *NetworkNode) SendMessage(wrappedStream *BasicStream, pid peer.ID, opCode Opcode, data interface{}) error {
	msg := Msg{Op: opCode, Data: data}
	glog.Infof("Sending: %v to %v", msg, peer.IDHexEncode(wrappedStream.Stream.Conn().RemotePeer()))
	return wrappedStream.EncodeAndFlush(msg)
}
