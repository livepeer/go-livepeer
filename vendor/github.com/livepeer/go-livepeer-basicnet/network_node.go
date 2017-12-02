package basicnet

import (
	"context"
	"fmt"
	"sync"

	"github.com/golang/glog"

	peerstore "gx/ipfs/QmPgDWmTmuzvP7QE5zwo1TmjbJme9pmZHNujB2453jkCTr/go-libp2p-peerstore"
	addrutil "gx/ipfs/QmVJGsPeK3vwtEyyTxpCs47yjBYMmYsAhEouPDF3Gb2eK3/go-addr-util"
	ds "gx/ipfs/QmVSase1JP7cq9QkPT46oNwdp9pT6kBkG3oqS14y3QcZjG/go-datastore"
	swarm "gx/ipfs/QmWpJ4y2vxJ6GZpPfQbpVpQxAYS3UeR6AKNbAHxw7wN3qw/go-libp2p-swarm"
	ma "gx/ipfs/QmXY77cVe7rVRQXZZQRioukUM7aRW3BTcAgJe12MCtb3Ji/go-multiaddr"
	peer "gx/ipfs/QmXYjuNuxVzXKJCfWasQk1RqkhVLDM9jtUKhqc2WPQmFSB/go-libp2p-peer"
	kad "gx/ipfs/QmYi2NvTAiv2xTNJNcnuz3iXDDT1ViBwLFXmDb2g7NogAD/go-libp2p-kad-dht"
	crypto "gx/ipfs/QmaPbCnUMBohSGo3KnxEa2bHqyJVVeEEcwtqJAYxerieBo/go-libp2p-crypto"
	bhost "gx/ipfs/Qmbgce14YTWE2qhE49JVvTBPaHTyz3FaFmqQPyuZAz6C28/go-libp2p/p2p/host/basic"
	rhost "gx/ipfs/Qmbgce14YTWE2qhE49JVvTBPaHTyz3FaFmqQPyuZAz6C28/go-libp2p/p2p/host/routed"
	record "gx/ipfs/QmbxkgUceEcuSZ4ZdBA3x74VUDSSYjHYmmeEqkjxbtZ6Jg/go-libp2p-record"
	host "gx/ipfs/Qmc1XhrFEiSeBNn3mpfg6gEuYCt5im2gYmNVmncsvmpeAk/go-libp2p-host"
)

type NetworkNode struct {
	Identity       peer.ID // the local node's identity
	Kad            *kad.IpfsDHT
	PeerHost       host.Host // the network host (server+client)
	Network        *BasicVideoNetwork
	outStreams     map[peer.ID]*BasicOutStream
	outStreamsLock *sync.Mutex
}

//NewNode creates a new Livepeerd node.
func NewNode(listenPort int, priv crypto.PrivKey, pub crypto.PubKey, f *BasicNotifiee) (*NetworkNode, error) {
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
	nn := &NetworkNode{Identity: pid, Kad: dht, PeerHost: rHost, outStreams: streams, outStreamsLock: &sync.Mutex{}}
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

func (n *NetworkNode) GetOutStream(pid peer.ID) *BasicOutStream {
	n.outStreamsLock.Lock()
	strm, ok := n.outStreams[pid]
	if !ok {
		strm = n.RefreshOutStream(pid)
	}
	n.outStreamsLock.Unlock()
	return strm
}

func (n *NetworkNode) RefreshOutStream(pid peer.ID) *BasicOutStream {
	// glog.Infof("Creating stream from %v to %v", peer.IDHexEncode(n.Identity), peer.IDHexEncode(pid))
	if s, ok := n.outStreams[pid]; ok {
		s.Stream.Reset()
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

func (n *NetworkNode) RemoveStream(pid peer.ID) {
	// glog.Infof("Removing stream for %v", peer.IDHexEncode(pid))
	n.outStreamsLock.Lock()
	delete(n.outStreams, pid)
	n.outStreamsLock.Unlock()
}
