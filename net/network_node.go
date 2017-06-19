package net

import (
	"context"
	"fmt"

	"github.com/golang/glog"
	ds "github.com/ipfs/go-datastore"
	crypto "github.com/libp2p/go-libp2p-crypto"
	host "github.com/libp2p/go-libp2p-host"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	net "github.com/libp2p/go-libp2p-net"
	peer "github.com/libp2p/go-libp2p-peer"
	peerstore "github.com/libp2p/go-libp2p-peerstore"
	routing "github.com/libp2p/go-libp2p-routing"
	swarm "github.com/libp2p/go-libp2p-swarm"
	bhost "github.com/libp2p/go-libp2p/p2p/host/basic"
	rhost "github.com/libp2p/go-libp2p/p2p/host/routed"
	ma "github.com/multiformats/go-multiaddr"
)

type NetworkNode struct {
	Identity  peer.ID             // the local node's identity
	Peerstore peerstore.Peerstore // storage for other Peer instances
	Routing   routing.IpfsRouting // the routing system. recommend ipfs-dht
	PeerHost  host.Host           // the network host (server+client)
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
		nil)

	basicHost := bhost.New(netwrk)

	r, err := constructDHTRouting(context.Background(), basicHost, ds.NewMapDatastore())
	rHost := rhost.Wrap(basicHost, r)

	return &NetworkNode{Identity: pid, Peerstore: store, Routing: r, PeerHost: rHost}, nil
}

func constructDHTRouting(ctx context.Context, host host.Host, dstore ds.Batching) (routing.IpfsRouting, error) {
	dhtRouting := dht.NewDHT(ctx, host, dstore)
	return dhtRouting, nil
}

func (n *NetworkNode) SendMessage(stream net.Stream, pid peer.ID, opCode Opcode, data interface{}) {
	wrappedStream := WrapStream(stream)
	msg := Msg{Op: opCode, Data: data}
	glog.Infof("Sending: %v", msg)
	err := wrappedStream.enc.Encode(msg)
	if err != nil {
		glog.Errorf("send message encode error: %v", err)
	}

	err = wrappedStream.w.Flush()
	if err != nil {
		glog.Errorf("send message flush error: %v", err)
	}
}
