package basicnet

import (
	"context"
	"fmt"

	"github.com/golang/glog"

	peerstore "gx/ipfs/QmPgDWmTmuzvP7QE5zwo1TmjbJme9pmZHNujB2453jkCTr/go-libp2p-peerstore"
	addrutil "gx/ipfs/QmR1fAHJvEyYFdEGn5jVmU4NL5kNSVJ48cduXB2whWbJq2/go-addr-util"
	kad "gx/ipfs/QmTHyAbD9KzGrseLNzmEoNkVxA8F2h7LQG2iV6uhBqs6kX/go-libp2p-kad-dht"
	ds "gx/ipfs/QmVSase1JP7cq9QkPT46oNwdp9pT6kBkG3oqS14y3QcZjG/go-datastore"
	ma "gx/ipfs/QmXY77cVe7rVRQXZZQRioukUM7aRW3BTcAgJe12MCtb3Ji/go-multiaddr"
	peer "gx/ipfs/QmXYjuNuxVzXKJCfWasQk1RqkhVLDM9jtUKhqc2WPQmFSB/go-libp2p-peer"
	host "gx/ipfs/QmZy7c24mmkEHpNJndwgsEE3wcVxHd8yB969yTnAJFVw7f/go-libp2p-host"
	crypto "gx/ipfs/QmaPbCnUMBohSGo3KnxEa2bHqyJVVeEEcwtqJAYxerieBo/go-libp2p-crypto"
	swarm "gx/ipfs/QmaijwHnbD4SabGA8C2fN9gchptLvRe2RxqTU5XkjAGBw5/go-libp2p-swarm"
	bhost "gx/ipfs/QmapADMpK4e5kFGBxC2aHreaDqKP9vmMng5f91MA14Ces9/go-libp2p/p2p/host/basic"
	rhost "gx/ipfs/QmapADMpK4e5kFGBxC2aHreaDqKP9vmMng5f91MA14Ces9/go-libp2p/p2p/host/routed"
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

	netwrk.Notify(&BasicNotifiee{})
	basicHost := bhost.New(netwrk, bhost.NATPortMap)

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
	strm, ok := n.streams[pid]
	if !ok {
		glog.Infof("Creating stream from %v to %v", peer.IDHexEncode(n.Identity), peer.IDHexEncode(pid))
		ns, err := n.PeerHost.NewStream(context.Background(), pid, Protocol)
		if err != nil {
			glog.Errorf("Error creating stream to %v: %v", peer.IDHexEncode(pid), err)
			return nil
		}
		strm = NewBasicStream(ns)
		n.streams[pid] = strm
	}

	return strm
}
