package basicnet

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/golang/glog"

	inet "gx/ipfs/QmNa31VPzC561NWwRsJLE7nGYZYuuD2QfpK2b1q9BK54J1/go-libp2p-net"
	peerstore "gx/ipfs/QmPgDWmTmuzvP7QE5zwo1TmjbJme9pmZHNujB2453jkCTr/go-libp2p-peerstore"
	kb "gx/ipfs/QmSAFA8v42u4gpJNy1tb7vW3JiiXiaYDC2b845c2RnNSJL/go-libp2p-kbucket"
	addrutil "gx/ipfs/QmVJGsPeK3vwtEyyTxpCs47yjBYMmYsAhEouPDF3Gb2eK3/go-addr-util"
	ds "gx/ipfs/QmVSase1JP7cq9QkPT46oNwdp9pT6kBkG3oqS14y3QcZjG/go-datastore"
	swarm "gx/ipfs/QmWpJ4y2vxJ6GZpPfQbpVpQxAYS3UeR6AKNbAHxw7wN3qw/go-libp2p-swarm"
	ma "gx/ipfs/QmXY77cVe7rVRQXZZQRioukUM7aRW3BTcAgJe12MCtb3Ji/go-multiaddr"
	peer "gx/ipfs/QmXYjuNuxVzXKJCfWasQk1RqkhVLDM9jtUKhqc2WPQmFSB/go-libp2p-peer"
	kad "gx/ipfs/QmYi2NvTAiv2xTNJNcnuz3iXDDT1ViBwLFXmDb2g7NogAD/go-libp2p-kad-dht"
	protocol "gx/ipfs/QmZNkThpqfVXs9GNbexPrfBbXSLNYeKrE7jwFM2oqHbyqN/go-libp2p-protocol"
	crypto "gx/ipfs/QmaPbCnUMBohSGo3KnxEa2bHqyJVVeEEcwtqJAYxerieBo/go-libp2p-crypto"
	bhost "gx/ipfs/Qmbgce14YTWE2qhE49JVvTBPaHTyz3FaFmqQPyuZAz6C28/go-libp2p/p2p/host/basic"
	rhost "gx/ipfs/Qmbgce14YTWE2qhE49JVvTBPaHTyz3FaFmqQPyuZAz6C28/go-libp2p/p2p/host/routed"
	record "gx/ipfs/QmbxkgUceEcuSZ4ZdBA3x74VUDSSYjHYmmeEqkjxbtZ6Jg/go-libp2p-record"
	host "gx/ipfs/Qmc1XhrFEiSeBNn3mpfg6gEuYCt5im2gYmNVmncsvmpeAk/go-libp2p-host"
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
func NewNode(listenPort int, priv crypto.PrivKey, pub crypto.PubKey, f *BasicNotifiee) (*BasicNetworkNode, error) {
	pid, err := peer.IDFromPublicKey(pub)
	if err != nil {
		return nil, err
	}

	streams := make(map[peer.ID]*BasicOutStream)

	// Create a peerstore
	store := peerstore.NewPeerstore()
	store.AddPrivKey(pid, priv)
	store.AddPubKey(pid, pub)

	// Create multiaddresses.  I'm not sure if this is correct in all cases...
	uaddrs, err := addrutil.InterfaceAddresses()
	if err != nil {
		return nil, err
	}
	addrs := make([]ma.Multiaddr, len(uaddrs), len(uaddrs))
	for i, uaddr := range uaddrs {
		portAddr, err := ma.NewMultiaddr(fmt.Sprintf("/tcp/%d", listenPort))
		if err != nil {
			glog.Errorf("Error creating portAddr: %v %v", uaddr, err)
			return nil, err
		}
		addrs[i] = uaddr.Encapsulate(portAddr)
	}

	// Create swarm (implements libP2P Network)
	netwrk, err := swarm.NewNetwork(
		context.Background(),
		addrs,
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

	dhtRouting.Validator["v"] = &record.ValidChecker{
		Func: func(string, []byte) error {
			return nil
		},
		Sign: false,
	}
	dhtRouting.Selector["v"] = func(_ string, bs [][]byte) (int, error) { return 0, nil }

	// if err := dhtRouting.Bootstrap(context.Background()); err != nil {
	// 	glog.Errorf("Error bootstraping dht: %v", err)
	// 	return nil, err
	// }
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
