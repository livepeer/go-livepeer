package main

import (
	"flag"
	"fmt"
	"time"

	ma "gx/ipfs/QmWWQ2Txc2c6tqjsBpzg5Ar652cHPGNsQQp2SejkNmkUMb/go-multiaddr"
	peerstore "gx/ipfs/QmXauCuJzmzapetmC6W4TuDJLL1yFFrVzSHoWv8YdbmnxH/go-libp2p-peerstore"
	net "gx/ipfs/QmXfkENeeBvh3zYA51MaSdGUdBjhQ99cP5WQe8zgr6wchG/go-libp2p-net"
	"gx/ipfs/QmZoWKhxUmZ2seW4BzX6fJkNR8hh9PsGModr7q171yq2SS/go-libp2p-peer"
	crypto "gx/ipfs/QmaPbCnUMBohSGo3KnxEa2bHqyJVVeEEcwtqJAYxerieBo/go-libp2p-crypto"

	"github.com/golang/glog"
	basicnet "github.com/livepeer/go-livepeer-basicnet"
)

/**
To run the example, run:

Node1: go run cmd/example.go -ping
Node2: go run cmd/example.go -ping -p 15001 -id {Node1ID} -addr {Node1Addr} -i

**/
var timer time.Time

func main() {
	p := flag.Int("p", 15000, "port")
	id := flag.String("id", "", "id")
	addr := flag.String("addr", "", "addr")
	init := flag.Bool("i", false, "initialize message sending")
	ping := flag.Bool("ping", false, "ping test")

	flag.Parse()
	flag.Lookup("logtostderr").Value.Set("true")

	priv, pub, _ := crypto.GenerateKeyPair(crypto.RSA, 2048)
	sourceMultiAddr, _ := ma.NewMultiaddr(fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", *p))
	node, _ := basicnet.NewNode([]ma.Multiaddr{sourceMultiAddr}, priv, pub, &basicnet.BasicNotifiee{})

	pid, _ := peer.IDHexDecode(*id)
	if *id != "" {
		paddr, _ := ma.NewMultiaddr(*addr)
		node.PeerHost.Peerstore().AddAddr(pid, paddr, peerstore.PermanentAddrTTL)
	}

	if *ping {
		pingtest(*init, node, pid)
	}

}

func pingtest(init bool, node *basicnet.BasicNetworkNode, pid peer.ID) {
	if init {
		timer = time.Now()
		strm := node.GetOutStream(pid)
		glog.Infof("Sending message")
		strm.SendMessage(basicnet.SubReqID, basicnet.SubReqMsg{StrmID: "test"})
		for {
			setHandler(node)
		}
	} else {
		setHandler(node)
		glog.Infof("Done setting handler")
	}

	glog.Infof("NodeID: %v", peer.IDHexEncode(node.Identity))
	glog.Infof("NodeAddr: %v", node.PeerHost.Addrs())
	select {}
}

func setHandler(n *basicnet.BasicNetworkNode) {
	n.PeerHost.SetStreamHandler(basicnet.Protocol, func(stream net.Stream) {
		ws := basicnet.NewBasicInStream(stream)

		for {
			if err := streamHandler(ws); err != nil {
				glog.Errorf("Error handling stream: %v", err)
				// delete(n.NetworkNode.streams, stream.Conn().RemotePeer())
				stream.Close()
				return
			}
		}
	})
}

func streamHandler(ws *basicnet.BasicInStream) error {
	msg, err := ws.ReceiveMessage()
	if err != nil {
		glog.Errorf("Got error decoding msg: %v", err)
		return err
	}

	glog.Infof("%v Recieved msg %v from %v", peer.IDHexEncode(ws.Stream.Conn().LocalPeer()), msg.Op, peer.IDHexEncode(ws.Stream.Conn().RemotePeer()))
	glog.Infof("Time since last message recieved: %v", time.Since(timer))

	// timer = time.Now()
	// vid, _ := ioutil.ReadFile("./test.ts")
	// return ws.SendMessage(basicnet.StreamDataID, basicnet.StreamDataMsg{Data: vid, SeqNo: 0, StrmID: "test"})
	return nil
}
