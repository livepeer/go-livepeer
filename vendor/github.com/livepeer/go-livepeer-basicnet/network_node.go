package basicnet

import (
	"context"
	bhost "gx/ipfs/QmNh1kGFFdsPu79KNSaL4NUKUPb4Eiz4KHdMtFY6664RDp/go-libp2p/p2p/host/basic"
	rhost "gx/ipfs/QmNh1kGFFdsPu79KNSaL4NUKUPb4Eiz4KHdMtFY6664RDp/go-libp2p/p2p/host/routed"
	host "gx/ipfs/QmNmJZL7FQySMtE2BQuLMuZg2EB2CLEunJJUSVSc9YnnbV/go-libp2p-host"
	swarm "gx/ipfs/QmSwZMWwFZSUpe5muU2xgTUwppH24KfMwdPXiwbEp2c6G5/go-libp2p-swarm"
	kb "gx/ipfs/QmTH6VLu3WXfbH3nuLdmscgPWuiPZv3GMJ2YCdzBS5z91T/go-libp2p-kbucket"
	ma "gx/ipfs/QmWWQ2Txc2c6tqjsBpzg5Ar652cHPGNsQQp2SejkNmkUMb/go-multiaddr"
	ds "gx/ipfs/QmXRKBQA4wXP7xWbFiZsR1GP4HV6wMDQ1aWFxZZ4uBcPX9/go-datastore"
	peerstore "gx/ipfs/QmXauCuJzmzapetmC6W4TuDJLL1yFFrVzSHoWv8YdbmnxH/go-libp2p-peerstore"
	inet "gx/ipfs/QmXfkENeeBvh3zYA51MaSdGUdBjhQ99cP5WQe8zgr6wchG/go-libp2p-net"
	kad "gx/ipfs/QmY1y2M1aCcVhy8UuTbZJBvuFbegZm47f9cDAdgxiehQfx/go-libp2p-kad-dht"
	protocol "gx/ipfs/QmZNkThpqfVXs9GNbexPrfBbXSLNYeKrE7jwFM2oqHbyqN/go-libp2p-protocol"
	peer "gx/ipfs/QmZoWKhxUmZ2seW4BzX6fJkNR8hh9PsGModr7q171yq2SS/go-libp2p-peer"
	crypto "gx/ipfs/QmaPbCnUMBohSGo3KnxEa2bHqyJVVeEEcwtqJAYxerieBo/go-libp2p-crypto"
	"sync"
	"time"

	"github.com/golang/glog"
)

type NetworkNode interface {
	ID() peer.ID
	GetOutStream(pid peer.ID) *BasicOutStream
	RefreshOutStream(pid peer.ID) *BasicOutStream
	RemoveStream(pid peer.ID)
	GetPeers() []peer.ID
	GetPeerInfo(peer.ID) peerstore.PeerInfo
	Connect(context.Context, peerstore.PeerInfo) error
	AddPeer(peerstore.PeerInfo, time.Duration)
	RemovePeer(peer.ID)
	ClosestLocalPeers(strmID string) ([]peer.ID, error)
	GetDHT() *kad.IpfsDHT
	SetStreamHandler(pid protocol.ID, handler inet.StreamHandler)
}

type BasicNetworkNode struct {
	Identity       peer.ID // the local node's identity
	Kad            *kad.IpfsDHT
	PeerHost       host.Host // the network host (server+client)
	Network        *BasicVideoNetwork
	outStreams     map[peer.ID]*BasicOutStream
	outStreamsLock *sync.Mutex
}

//NewNode creates a new Livepeerd node.
func NewNode(listenAddrs []ma.Multiaddr, priv crypto.PrivKey, pub crypto.PubKey, f *BasicNotifiee) (*BasicNetworkNode, error) {
	pid, err := peer.IDFromPublicKey(pub)
	if err != nil {
		return nil, err
	}

	streams := make(map[peer.ID]*BasicOutStream)

	// Create a peerstore
	store := peerstore.NewPeerstore()
	store.AddPrivKey(pid, priv)
	store.AddPubKey(pid, pub)

	// Create swarm (implements libP2P Network)
	netwrk, err := swarm.NewNetwork(
		context.Background(),
		listenAddrs,
		pid,
		store,
		&BasicReporter{})

	netwrk.Notify(f)
	basicHost := bhost.New(netwrk, bhost.NATPortMap)

	dht, err := constructDHTRouting(context.Background(), basicHost, ds.NewMapDatastore())
	if err != nil {
		glog.Errorf("Error constructing DHT: %v", err)
		return nil, err
	}
	rHost := rhost.Wrap(basicHost, dht)

	glog.V(2).Infof("Created node: %v at %v", peer.IDHexEncode(rHost.ID()), rHost.Addrs())
	nn := &BasicNetworkNode{Identity: pid, Kad: dht, PeerHost: rHost, outStreams: streams, outStreamsLock: &sync.Mutex{}}
	f.HandleDisconnect(func(pid peer.ID) {
		nn.RemoveStream(pid)
	})

	return nn, nil
}

func constructDHTRouting(ctx context.Context, host host.Host, dstore ds.Batching) (*kad.IpfsDHT, error) {
	dhtRouting := kad.NewDHT(ctx, host, dstore)

	return dhtRouting, nil
}

func (n *BasicNetworkNode) GetOutStream(pid peer.ID) *BasicOutStream {
	n.outStreamsLock.Lock()
	strm, ok := n.outStreams[pid]
	if !ok {
		strm = n.RefreshOutStream(pid)
	}
	n.outStreamsLock.Unlock()
	return strm
}

func (n *BasicNetworkNode) RefreshOutStream(pid peer.ID) *BasicOutStream {
	// glog.Infof("Creating stream from %v to %v", peer.IDHexEncode(n.Identity), peer.IDHexEncode(pid))
	if s, ok := n.outStreams[pid]; ok {
		if err := s.Stream.Reset(); err != nil {
			glog.Errorf("Error resetting connetion: %v", err)
		}
	}

	ns, err := n.PeerHost.NewStream(context.Background(), pid, Protocol)
	if err != nil {
		glog.Errorf("%v Error creating stream to %v: %v", peer.IDHexEncode(n.Identity), peer.IDHexEncode(pid), err)
		return nil
	}
	strm := NewBasicOutStream(ns)
	n.outStreams[pid] = strm
	return strm
}

func (n *BasicNetworkNode) RemoveStream(pid peer.ID) {
	// glog.Infof("Removing stream for %v", peer.IDHexEncode(pid))
	n.outStreamsLock.Lock()
	delete(n.outStreams, pid)
	n.outStreamsLock.Unlock()
}

func (n *BasicNetworkNode) ID() peer.ID {
	return n.Identity
}

func (n *BasicNetworkNode) GetPeers() []peer.ID {
	return n.PeerHost.Peerstore().Peers()
}

func (n *BasicNetworkNode) GetPeerInfo(p peer.ID) peerstore.PeerInfo {
	return n.PeerHost.Peerstore().PeerInfo(p)
}

func (n *BasicNetworkNode) Connect(ctx context.Context, pi peerstore.PeerInfo) error {
	return n.PeerHost.Connect(ctx, pi)
}

func (n *BasicNetworkNode) AddPeer(pi peerstore.PeerInfo, ttl time.Duration) {
	n.PeerHost.Peerstore().AddAddrs(pi.ID, pi.Addrs, ttl)
}

func (n *BasicNetworkNode) RemovePeer(id peer.ID) {
	n.PeerHost.Peerstore().ClearAddrs(id)
}

func (n *BasicNetworkNode) GetDHT() *kad.IpfsDHT {
	return n.Kad
}

func (n *BasicNetworkNode) SetStreamHandler(pid protocol.ID, handler inet.StreamHandler) {
	n.PeerHost.SetStreamHandler(pid, handler)
}

func (bn *BasicNetworkNode) ClosestLocalPeers(strmID string) ([]peer.ID, error) {
	targetPid, err := extractNodeID(strmID)
	if err != nil {
		glog.Errorf("Error extracting node id from streamID: %v", strmID)
		return nil, ErrSubscriber
	}
	localPeers := bn.PeerHost.Peerstore().Peers()
	if len(localPeers) == 1 {
		glog.Errorf("No local peers")
		return nil, ErrSubscriber
	}

	return kb.SortClosestPeers(localPeers, kb.ConvertPeerID(targetPid)), nil
}
