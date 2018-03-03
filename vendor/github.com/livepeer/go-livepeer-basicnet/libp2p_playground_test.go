package basicnet

import (
	"context"
	"encoding/hex"
	"fmt"
	"testing"
	"time"

	net "gx/ipfs/QmNa31VPzC561NWwRsJLE7nGYZYuuD2QfpK2b1q9BK54J1/go-libp2p-net"
	cid "gx/ipfs/QmNp85zy9RLrQ5oQD4hPyS39ezrrXpcaa7R4Y9kxdWQLLQ/go-cid"
	peerstore "gx/ipfs/QmPgDWmTmuzvP7QE5zwo1TmjbJme9pmZHNujB2453jkCTr/go-libp2p-peerstore"
	netutil "gx/ipfs/QmQGX417WoxKxDJeHqouMEmmH4G1RCENNSzkZYHrXy3Xb3/go-libp2p-netutil"
	"gx/ipfs/QmU9a9NV9RdPNwZQDYd5uKsm6N6LJLSvLbywDDYFbaaC6P/go-multihash"
	ds "gx/ipfs/QmVSase1JP7cq9QkPT46oNwdp9pT6kBkG3oqS14y3QcZjG/go-datastore"
	dssync "gx/ipfs/QmVSase1JP7cq9QkPT46oNwdp9pT6kBkG3oqS14y3QcZjG/go-datastore/sync"
	swarm "gx/ipfs/QmWpJ4y2vxJ6GZpPfQbpVpQxAYS3UeR6AKNbAHxw7wN3qw/go-libp2p-swarm"
	ma "gx/ipfs/QmXY77cVe7rVRQXZZQRioukUM7aRW3BTcAgJe12MCtb3Ji/go-multiaddr"
	peer "gx/ipfs/QmXYjuNuxVzXKJCfWasQk1RqkhVLDM9jtUKhqc2WPQmFSB/go-libp2p-peer"
	kad "gx/ipfs/QmYi2NvTAiv2xTNJNcnuz3iXDDT1ViBwLFXmDb2g7NogAD/go-libp2p-kad-dht"
	crypto "gx/ipfs/QmaPbCnUMBohSGo3KnxEa2bHqyJVVeEEcwtqJAYxerieBo/go-libp2p-crypto"
	bhost "gx/ipfs/Qmbgce14YTWE2qhE49JVvTBPaHTyz3FaFmqQPyuZAz6C28/go-libp2p/p2p/host/basic"
	rhost "gx/ipfs/Qmbgce14YTWE2qhE49JVvTBPaHTyz3FaFmqQPyuZAz6C28/go-libp2p/p2p/host/routed"
	record "gx/ipfs/QmbxkgUceEcuSZ4ZdBA3x74VUDSSYjHYmmeEqkjxbtZ6Jg/go-libp2p-record"
	host "gx/ipfs/Qmc1XhrFEiSeBNn3mpfg6gEuYCt5im2gYmNVmncsvmpeAk/go-libp2p-host"

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

func simpleHandler(host host.Host, ns net.Stream, txt string) error {
	ws := NewBasicInStream(ns)

	msg, err := ws.ReceiveMessage()

	if err != nil {
		glog.Errorf("Got error decoding msg: %v", err)
		return err
	}
	glog.Infof("%v Got msg: %v", ws.Stream.Conn().LocalPeer().Pretty(), msg)
	// time.Sleep(100 * time.Millisecond)

	glog.Infof("Sending %v", txt)
	os, err := host.NewStream(context.Background(), ns.Conn().RemotePeer())
	if err != nil {
		glog.Errorf("Error creating out stream: %v", err)
		return err
	}
	outStrm := NewBasicOutStream(os)
	outStrm.SendMessage(0, StreamDataMsg{Data: []byte(txt)})
	return nil
}

func simpleHandlerLoop(host host.Host, ws *BasicInStream, txt string) {
	msg, err := ws.ReceiveMessage()
	os, err := host.NewStream(context.Background(), ws.Stream.Conn().RemotePeer())
	if err != nil {
		glog.Errorf("Error creating out stream: %v", err)
	}
	outStrm := NewBasicOutStream(os)

	for {

		if err != nil {
			glog.Errorf("Got error decoding msg: %v", err)
			return
		}
		glog.Infof("%v Got msg: %v", ws.Stream.Conn().LocalPeer().Pretty(), msg)

		time.Sleep(50 * time.Millisecond)

		newMsg := Msg{Data: string(msg.Data.([]byte)) + "|" + txt, Op: StreamDataID}

		glog.Infof("Sending %v", newMsg)
		err = outStrm.SendMessage(0, newMsg)
		if err != nil {
			glog.Errorf("Failed to send message %v: %v", newMsg, err)
		}
	}
}

func simpleSend(ns net.Stream, txt string, t *testing.T) {
	ws := NewBasicOutStream(ns)
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

func makeRandomHost(port int) (*kad.IpfsDHT, host.Host) {
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
	return dht, rHost
}

func TestBasic(t *testing.T) {
	glog.Infof("\n\nTest Basic...")
	_, h1 := makeRandomHost(10000)
	defer h1.Close()
	_, h2 := makeRandomHost(10001)
	defer h2.Close()
	h1.Peerstore().AddAddrs(h2.ID(), h2.Addrs(), peerstore.PermanentAddrTTL)
	h2.Peerstore().AddAddrs(h1.ID(), h1.Addrs(), peerstore.PermanentAddrTTL)

	h2.SetStreamHandler(Protocol, func(stream net.Stream) {
		glog.Infof("h2 handler...")
		for {
			if err := simpleHandler(h2, stream, "pong"); err != nil {
				stream.Close()
				return
			}
		}
	})

	stream, err := h1.NewStream(context.Background(), h2.ID(), Protocol)
	if err != nil {
		glog.Fatal(err)
	}
	s1 := NewBasicOutStream(stream)
	s1.SendMessage(0, StreamDataMsg{Data: []byte("ping1")})
	time.Sleep(time.Millisecond * 200)

	s1.SendMessage(0, StreamDataMsg{Data: []byte("ping2")})
	time.Sleep(time.Millisecond * 500)
	s1.Stream.Reset()
	time.Sleep(time.Millisecond * 100)
}

func TestUniDirection(t *testing.T) {
	glog.Infof("\n\nTest Unidirection...")
	dht1, h1 := makeRandomHost(10002)
	defer h1.Close()
	dht2, h2 := makeRandomHost(10003)
	defer h2.Close()
	connect(t, context.Background(), dht1, dht2, h1, h2)

	h2.SetStreamHandler(Protocol, func(stream net.Stream) {
		glog.Infof("h2 handler...")
		for {
			ws := NewBasicInStream(stream)
			msg, err := ws.ReceiveMessage()
			if err != nil {
				glog.Errorf("Got error decoding msg: %v", err)
				return
			}
			glog.Infof("%v Got msg: %v", ws.Stream.Conn().LocalPeer().Pretty(), msg)
		}
	})

	h1.SetStreamHandler(Protocol, func(stream net.Stream) {
		glog.Infof("h1 handler...")
		for {
			ws := NewBasicInStream(stream)
			msg, err := ws.ReceiveMessage()
			if err != nil {
				glog.Errorf("Got error decoding msg: %v", err)
				return
			}
			glog.Infof("%v Got msg: %v", ws.Stream.Conn().LocalPeer().Pretty(), msg)
		}
	})

	// time.Sleep(time.Millisecond * 2000)

	stream, err := h1.NewStream(context.Background(), h2.ID(), Protocol)
	if err != nil {
		glog.Fatal(err)
	}
	s1 := NewBasicOutStream(stream)
	if err := s1.SendMessage(0, StreamDataMsg{Data: []byte("ping1")}); err != nil {
		glog.Infof("Error: %v", err)
	}
	time.Sleep(time.Millisecond * 100)
	if err := s1.SendMessage(0, StreamDataMsg{Data: []byte("ping2")}); err != nil {
		glog.Infof("Error: %v", err)
	}
	time.Sleep(time.Millisecond * 100)
	s1.Stream.Reset()

	stream2, err := h2.NewStream(context.Background(), h1.ID(), Protocol)
	if err != nil {
		glog.Fatal(err)
	}
	s2 := NewBasicOutStream(stream2)
	s2.SendMessage(0, StreamDataMsg{Data: []byte("pong1")})
	s2.SendMessage(0, StreamDataMsg{Data: []byte("pong2")})
	time.Sleep(time.Millisecond * 100)
	s2.Stream.Reset()
}

func TestProvider(t *testing.T) {
	n1, n2 := simpleNodes(15010, 15011)
	defer n1.PeerHost.Close()
	defer n2.PeerHost.Close()
	n3, n4 := simpleNodes(15012, 15013)
	defer n3.PeerHost.Close()
	defer n4.PeerHost.Close()
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

func TestConcurrentSend(t *testing.T) {
	n1, n2 := simpleNodes(15000, 15001)
	defer n1.PeerHost.Close()
	defer n2.PeerHost.Close()
	n3, _ := simpleNodes(15002, 15003)
	defer n3.PeerHost.Close()
	connectHosts(n1.PeerHost, n2.PeerHost)
	connectHosts(n2.PeerHost, n3.PeerHost)
	n1.PeerHost.SetStreamHandler(Protocol, func(stream net.Stream) {
	})
	c := make(chan string, 20)
	n2.PeerHost.SetStreamHandler(Protocol, func(stream net.Stream) {
		strm := NewBasicInStream(stream)
		defer stream.Reset()
		for {
			if msg, err := strm.ReceiveMessage(); err != nil {
				glog.Infof("Error: %v", err)
				break
			} else {
				c <- string(msg.Data.(StreamDataMsg).Data)
			}
		}
	})
	n3.PeerHost.SetStreamHandler(Protocol, func(stream net.Stream) {
	})

	go func() {
		for i := 0; i < 10; i++ {
			strm := n1.GetOutStream(n2.Identity)
			strm.SendMessage(StreamDataID, StreamDataMsg{Data: []byte(fmt.Sprintf("%v", i))})
		}
	}()

	go func() {
		for i := 10; i < 20; i++ {
			strm := n3.GetOutStream(n2.Identity)
			strm.SendMessage(StreamDataID, StreamDataMsg{Data: []byte(fmt.Sprintf("%v", i))})
			// time.Sleep(time.Millisecond * 2)
		}
	}()

	for i := 0; i < 20; i++ {
		select {
		case i := <-c:
			fmt.Printf("got %v\n", i)
		case <-time.After(5 * time.Second):
			t.Errorf("Timed out")
		}
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
