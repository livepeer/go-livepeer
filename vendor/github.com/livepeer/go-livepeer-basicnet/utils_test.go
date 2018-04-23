package basicnet

import (
	"context"
	"testing"
	"time"

	bhost "gx/ipfs/QmNh1kGFFdsPu79KNSaL4NUKUPb4Eiz4KHdMtFY6664RDp/go-libp2p/p2p/host/basic"
	host "gx/ipfs/QmNmJZL7FQySMtE2BQuLMuZg2EB2CLEunJJUSVSc9YnnbV/go-libp2p-host"
	ds "gx/ipfs/QmXRKBQA4wXP7xWbFiZsR1GP4HV6wMDQ1aWFxZZ4uBcPX9/go-datastore"
	dssync "gx/ipfs/QmXRKBQA4wXP7xWbFiZsR1GP4HV6wMDQ1aWFxZZ4uBcPX9/go-datastore/sync"
	peerstore "gx/ipfs/QmXauCuJzmzapetmC6W4TuDJLL1yFFrVzSHoWv8YdbmnxH/go-libp2p-peerstore"
	kad "gx/ipfs/QmY1y2M1aCcVhy8UuTbZJBvuFbegZm47f9cDAdgxiehQfx/go-libp2p-kad-dht"
	netutil "gx/ipfs/QmYVR3C8DWPHdHxvLtNFYfjsXgaRAdh6hPMNH3KiwCgu4o/go-libp2p-netutil"
	crypto "gx/ipfs/QmaPbCnUMBohSGo3KnxEa2bHqyJVVeEEcwtqJAYxerieBo/go-libp2p-crypto"

	"github.com/golang/glog"
)

func setupDHT(ctx context.Context, t *testing.T, client bool) (*kad.IpfsDHT, host.Host) {
	h := bhost.New(netutil.GenSwarmNetwork(t, ctx))

	dss := dssync.MutexWrap(ds.NewMapDatastore())
	var d *kad.IpfsDHT
	if client {
		d = kad.NewDHTClient(ctx, h, dss)
	} else {
		d = kad.NewDHT(ctx, h, dss)
	}

	return d, h
}

func setupDHTS(ctx context.Context, n int, t *testing.T) ([]*kad.IpfsDHT, []host.Host) {
	dhts := make([]*kad.IpfsDHT, n)
	hosts := make([]host.Host, n)
	// addrs := make([]ma.Multiaddr, n)
	// peers := make([]peer.ID, n)

	sanityAddrsMap := make(map[string]struct{})
	sanityPeersMap := make(map[string]struct{})

	for i := 0; i < n; i++ {
		dht, h := setupDHT(ctx, t, false)
		dhts[i] = dht
		hosts[i] = h
		// peers[i] = h.ID()
		// addrs[i] = h.Addrs()[0]

		if _, lol := sanityAddrsMap[h.Addrs()[0].String()]; lol {
			t.Fatal("While setting up DHTs address got duplicated.")
		} else {
			sanityAddrsMap[h.Addrs()[0].String()] = struct{}{}
		}
		if _, lol := sanityPeersMap[h.ID().String()]; lol {
			t.Fatal("While setting up DHTs peerid got duplicated.")
		} else {
			sanityPeersMap[h.ID().String()] = struct{}{}
		}
	}

	return dhts, hosts
}

func setupNodes(t *testing.T, p1, p2 int) (*BasicVideoNetwork, *BasicVideoNetwork) {
	priv1, pub1, _ := crypto.GenerateKeyPair(crypto.RSA, 2048)
	no1, _ := NewNode(p1, priv1, pub1, &BasicNotifiee{})
	n1, _ := NewBasicVideoNetwork(no1, "")
	if err := n1.SetupProtocol(); err != nil {
		t.Errorf("Error creating node: %v", err)
	}

	priv2, pub2, _ := crypto.GenerateKeyPair(crypto.RSA, 2048)
	no2, _ := NewNode(p2, priv2, pub2, &BasicNotifiee{})
	n2, _ := NewBasicVideoNetwork(no2, "")
	if err := n2.SetupProtocol(); err != nil {
		t.Errorf("Error creating node: %v", err)
	}

	return n1, n2
}

func connectNoSync(t *testing.T, ctx context.Context, a, b host.Host) {
	idB := b.ID()
	addrB := b.Addrs()
	if len(addrB) == 0 {
		t.Fatal("peers setup incorrectly: no local address")
	}

	a.Peerstore().AddAddrs(idB, addrB, peerstore.TempAddrTTL)
	pi := peerstore.PeerInfo{ID: idB}
	if err := a.Connect(ctx, pi); err != nil {
		t.Fatal(err)
	}
}

func connect(t *testing.T, ctx context.Context, a, b *kad.IpfsDHT, ah, bh host.Host) {
	connectNoSync(t, ctx, ah, bh)

	// loop until connection notification has been received.
	// under high load, this may not happen as immediately as we would like.
	for a.FindLocal(bh.ID()).ID == "" {
		time.Sleep(time.Millisecond * 5)
	}

	for b.FindLocal(ah.ID()).ID == "" {
		time.Sleep(time.Millisecond * 5)
	}
}

func connectHosts(h1, h2 host.Host) {
	h1.Peerstore().AddAddrs(h2.ID(), h2.Addrs(), peerstore.PermanentAddrTTL)
	h2.Peerstore().AddAddrs(h1.ID(), h1.Addrs(), peerstore.PermanentAddrTTL)
	err := h1.Connect(context.Background(), peerstore.PeerInfo{ID: h2.ID()})
	if err != nil {
		glog.Errorf("Cannot connect h1 with h2: %v", err)
	}
	err = h2.Connect(context.Background(), peerstore.PeerInfo{ID: h1.ID()})
	if err != nil {
		glog.Errorf("Cannot connect h2 with h1: %v", err)
	}

	// Connection might not be formed right away under high load.  See https://github.com/libp2p/go-libp2p-kad-dht/blob/master/dht_test.go (func connect)
	time.Sleep(time.Millisecond * 100)
}

func simpleNodes(p1, p2 int) (*BasicNetworkNode, *BasicNetworkNode) {
	priv1, pub1, _ := crypto.GenerateKeyPair(crypto.RSA, 2048)
	priv2, pub2, _ := crypto.GenerateKeyPair(crypto.RSA, 2048)

	n1, _ := NewNode(p1, priv1, pub1, &BasicNotifiee{})
	n2, _ := NewNode(p2, priv2, pub2, &BasicNotifiee{})

	// n1.PeerHost.Peerstore().AddAddrs(n2.Identity, n2.PeerHost.Addrs(), peerstore.PermanentAddrTTL)
	// n2.PeerHost.Peerstore().AddAddrs(n1.Identity, n1.PeerHost.Addrs(), peerstore.PermanentAddrTTL)
	// n1.PeerHost.Connect(context.Background(), peerstore.PeerInfo{ID: n2.Identity})
	// n2.PeerHost.Connect(context.Background(), peerstore.PeerInfo{ID: n1.Identity})

	return n1, n2
}
