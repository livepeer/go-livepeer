package basicnet

import (
	"context"
	"encoding/hex"
	"fmt"
	"testing"
	"time"

	cid "gx/ipfs/QmNp85zy9RLrQ5oQD4hPyS39ezrrXpcaa7R4Y9kxdWQLLQ/go-cid"
	peerstore "gx/ipfs/QmPgDWmTmuzvP7QE5zwo1TmjbJme9pmZHNujB2453jkCTr/go-libp2p-peerstore"
	netutil "gx/ipfs/QmQ1bJEsmdEiGfTQRoj6CsshWmAKduAEDEbwzbvk5QT5Ui/go-libp2p-netutil"
	swarm "gx/ipfs/QmQUmDr1DMDDy6KMSsJuyV9nVD7dJZ9iWxXESQWPvte2NP/go-libp2p-swarm"
	"gx/ipfs/QmU9a9NV9RdPNwZQDYd5uKsm6N6LJLSvLbywDDYFbaaC6P/go-multihash"
	host "gx/ipfs/QmUwW8jMQDxXhLD2j4EfWqLEMX3MsvyWcWGvJPVDh1aTmu/go-libp2p-host"
	ds "gx/ipfs/QmVSase1JP7cq9QkPT46oNwdp9pT6kBkG3oqS14y3QcZjG/go-datastore"
	dssync "gx/ipfs/QmVSase1JP7cq9QkPT46oNwdp9pT6kBkG3oqS14y3QcZjG/go-datastore/sync"
	ma "gx/ipfs/QmXY77cVe7rVRQXZZQRioukUM7aRW3BTcAgJe12MCtb3Ji/go-multiaddr"
	peer "gx/ipfs/QmXYjuNuxVzXKJCfWasQk1RqkhVLDM9jtUKhqc2WPQmFSB/go-libp2p-peer"
	bhost "gx/ipfs/QmXZyBQMkqSYigxhJResC6fLWDGFhbphK67eZoqMDUvBmK/go-libp2p/p2p/host/basic"
	rhost "gx/ipfs/QmXZyBQMkqSYigxhJResC6fLWDGFhbphK67eZoqMDUvBmK/go-libp2p/p2p/host/routed"
	crypto "gx/ipfs/QmaPbCnUMBohSGo3KnxEa2bHqyJVVeEEcwtqJAYxerieBo/go-libp2p-crypto"
	net "gx/ipfs/QmahYsGWry85Y7WUe2SX5G4JkH2zifEQAUtJVLZ24aC9DF/go-libp2p-net"
	record "gx/ipfs/QmbxkgUceEcuSZ4ZdBA3x74VUDSSYjHYmmeEqkjxbtZ6Jg/go-libp2p-record"
	kad "gx/ipfs/QmeQMs9pr9Goci9xJ1Wo5ZQrknzBZwnmHYWJXA8stQDFMx/go-libp2p-kad-dht"

	"github.com/golang/glog"
)

type SimpleMsg struct {
	Msg string
}

func setupDHT(ctx context.Context, t *testing.T, client bool) (*kad.IpfsDHT, host.Host) {
	h := bhost.New(netutil.GenSwarmNetwork(t, ctx))

	dss := dssync.MutexWrap(ds.NewMapDatastore())
	var d *kad.IpfsDHT
	if client {
		d = kad.NewDHTClient(ctx, h, dss)
	} else {
		d = kad.NewDHT(ctx, h, dss)
	}

	d.Validator["v"] = &record.ValidChecker{
		Func: func(string, []byte) error {
			return nil
		},
		Sign: false,
	}
	d.Selector["v"] = func(_ string, bs [][]byte) (int, error) { return 0, nil }
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

func simpleNodes(p1, p2 int) (*NetworkNode, *NetworkNode) {
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

func simpleHandler(ns net.Stream, txt string) {
	ws := NewBasicStream(ns)

	msg, err := ws.ReceiveMessage()

	if err != nil {
		glog.Errorf("Got error decoding msg: %v", err)
		return
	}
	glog.Infof("%v Got msg: %v", ws.Stream.Conn().LocalPeer().Pretty(), msg)
	time.Sleep(500 * time.Millisecond)

	str := string(msg.Data.([]byte))

	newMsg := str + "|" + txt
	glog.Infof("Sending %v", newMsg)
	ws.SendMessage(0, newMsg)
}

func simpleHandlerLoop(ws *BasicStream, txt string) {
	for {
		msg, err := ws.ReceiveMessage()

		if err != nil {
			glog.Errorf("Got error decoding msg: %v", err)
			return
		}
		glog.Infof("%v Got msg: %v", ws.Stream.Conn().LocalPeer().Pretty(), msg)

		time.Sleep(500 * time.Millisecond)

		newMsg := Msg{Data: string(msg.Data.([]byte)) + "|" + txt, Op: StreamDataID}

		glog.Infof("Sending %v", newMsg)
		err = ws.SendMessage(0, newMsg)
		if err != nil {
			glog.Errorf("Failed to send message %v: %v", newMsg, err)
		}
	}
}

func simpleSend(ns net.Stream, txt string, t *testing.T) {
	ws := NewBasicStream(ns)
	ws.SendMessage(0, txt)
}

// func TestBackAndForth(t *testing.T) {
// 	glog.Infof("\n\nTest back and forth...")
// 	n1, n2 := simpleNodes(15003, 15004)
// 	connectHosts(n1.PeerHost, n2.PeerHost)
// 	time.Sleep(time.Second)

// 	n2.PeerHost.SetStreamHandler("/test/1.0", func(stream net.Stream) {
// 		simpleHandler(stream, "pong")
// 	})

// 	n1.PeerHost.SetStreamHandler("/test/1.0", func(stream net.Stream) {
// 		simpleHandler(stream, "ping")
// 	})

// 	ns1, err := n1.PeerHost.NewStream(context.Background(), n2.Identity, "/test/1.0")
// 	if err != nil {
// 		t.Errorf("Cannot create stream: %v", err)
// 	}
// 	simpleSend(ns1, "ns1", t)
// 	simpleHandler(ns1, "ping")
// }

func makeRandomHost(port int) host.Host {
	// Ignoring most errors for brevity
	// See echo example for more details and better implementation
	priv, pub, _ := crypto.GenerateKeyPair(crypto.RSA, 2048)
	pid, _ := peer.IDFromPublicKey(pub)
	listen, _ := ma.NewMultiaddr(fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", port))
	ps := peerstore.NewPeerstore()
	ps.AddPrivKey(pid, priv)
	ps.AddPubKey(pid, pub)
	n, _ := swarm.NewNetwork(context.Background(),
		[]ma.Multiaddr{listen}, pid, ps, nil)
	basicHost := bhost.New(n)
	// return basicHost
	dht, err := constructDHTRouting(context.Background(), basicHost, ds.NewMapDatastore())
	if err != nil {
		glog.Errorf("Error constructing DHTRouting: %v", err)
	}
	rHost := rhost.Wrap(basicHost, dht)
	return rHost
}

func TestBasic(t *testing.T) {
	glog.Infof("\n\nTest Basic...")
	h1 := makeRandomHost(10000)
	h2 := makeRandomHost(10001)
	h1.Peerstore().AddAddrs(h2.ID(), h2.Addrs(), peerstore.PermanentAddrTTL)
	h2.Peerstore().AddAddrs(h1.ID(), h1.Addrs(), peerstore.PermanentAddrTTL)

	h2.SetStreamHandler(Protocol, func(stream net.Stream) {
		glog.Infof("h2 handler...")
		simpleHandler(stream, "pong")
	})

	glog.Infof("Before stream")
	stream, err := h1.NewStream(context.Background(), h2.ID(), Protocol)
	if err != nil {
		glog.Fatal(err)
	}
	glog.Infof("After stream")

	s1 := NewBasicStream(stream)
	s1.SendMessage(0, SimpleMsg{Msg: "ping!"})
	time.Sleep(time.Second * 2)
}

func TestProvider(t *testing.T) {
	n1, n2 := simpleNodes(15010, 15011)
	n3, n4 := simpleNodes(15012, 15013)
	connectHosts(n1.PeerHost, n2.PeerHost)
	connectHosts(n2.PeerHost, n3.PeerHost)
	connectHosts(n3.PeerHost, n4.PeerHost)

	time.Sleep(time.Second)
	buf, _ := hex.DecodeString("hello")
	mhashBuf, _ := multihash.EncodeName(buf, "sha1")
	glog.Infof("Declaring provider: %v", peer.IDHexEncode(n1.Identity))
	// if err := n1.Kad.Provide(context.Background(), cid.NewCidV1(cid.Raw, []byte("hello")), true); err != nil {
	if err := n1.Kad.Provide(context.Background(), cid.NewCidV1(cid.Raw, mhashBuf), true); err != nil {
		glog.Errorf("Error declaring provide: %v", err)
	}

	time.Sleep(time.Second)
	// pidc := n4.Kad.FindProvidersAsync(context.Background(), cid.NewCidV1(cid.Raw, []byte("hello")), 10)
	pidc := n4.Kad.FindProvidersAsync(context.Background(), cid.NewCidV1(cid.Raw, mhashBuf), 1)
	// if err != nil {
	// 	glog.Errorf("Error finding providers: %v", err)
	// }
	select {
	case pid := <-pidc:
		glog.Infof("Provider for hello: %v", peer.IDHexEncode(pid.ID))
	}
}

// func TestCid(t *testing.T) {
// 	ctx := context.Background()
// 	nDHTs := 101
// 	dhts, hosts := setupDHTS(ctx, nDHTs, t)
// 	defer func() {
// 		for i := 0; i < nDHTs; i++ {
// 			dhts[i].Close()
// 			defer hosts[i].Close()
// 		}
// 	}()

// 	mrand := rand.New(rand.NewSource(42))
// 	guy := dhts[0]
// 	guyh := hosts[0]
// 	others := dhts[1:]
// 	othersh := hosts[1:]
// 	for i := 0; i < 20; i++ {
// 		for j := 0; j < 16; j++ { // 16, high enough to probably not have any partitions
// 			v := mrand.Intn(80)
// 			connect(t, ctx, others[i], others[20+v], othersh[i], othersh[20+v])
// 		}
// 	}

// 	for i := 0; i < 20; i++ {
// 		connect(t, ctx, guy, others[i], guyh, othersh[i])
// 	}

// 	pc, err := dhts[0].GetClosestPeers(context.Background(), peer.IDB58Encode(hosts[60].ID()))
// 	if err != nil {
// 		t.Errorf("Error: %v", err)
// 	}

// 	timer := time.NewTimer(time.Second * 2)
// 	select {
// 	case pid := <-pc:
// 		if pid == hosts[3].ID() {
// 			return
// 			// t.Errorf("Expecting %v, got %v", peer.IDHexEncode(hosts[3].ID()), peer.IDHexEncode(pid))
// 		}
// 	case <-timer.C:
// 		t.Errorf("Timed out, didn't find: %v", peer.IDHexEncode(hosts[3].ID()))
// 	}
// }
