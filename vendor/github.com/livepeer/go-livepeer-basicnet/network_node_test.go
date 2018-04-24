package basicnet

import (
	"fmt"
	net "gx/ipfs/QmXfkENeeBvh3zYA51MaSdGUdBjhQ99cP5WQe8zgr6wchG/go-libp2p-net"
	crypto "gx/ipfs/QmaPbCnUMBohSGo3KnxEa2bHqyJVVeEEcwtqJAYxerieBo/go-libp2p-crypto"
	"net/http"
	"testing"
	"time"

	"github.com/golang/glog"
)

func TestGetOutStream(t *testing.T) {
	go func() {
		http.ListenAndServe("localhost:6060", nil)
	}()

	priv1, pub1, _ := crypto.GenerateKeyPair(crypto.RSA, 2048)
	no1, _ := NewNode(15000, priv1, pub1, &BasicNotifiee{})
	priv2, pub2, _ := crypto.GenerateKeyPair(crypto.RSA, 2048)
	no2, _ := NewNode(15001, priv2, pub2, &BasicNotifiee{})
	no1.PeerHost.SetStreamHandler(Protocol, func(s net.Stream) {
	})
	msgChan := make(chan Msg)
	no2.PeerHost.SetStreamHandler(Protocol, func(s net.Stream) {
		is := NewBasicInStream(s)
		for {
			msg, err := is.ReceiveMessage()
			if err != nil {
				glog.Errorf("Error: %v", err)
				break
			}
			msgChan <- msg
		}
	})
	connectHosts(no1.PeerHost, no2.PeerHost)

	strm := no1.GetOutStream(no2.Identity)
	if strm == nil {
		t.Errorf("Expecting an outstream")
	}
	for i := 0; i < 4; i++ {
		go func(i int) {
			if err := strm.SendMessage(StreamDataID, StreamDataMsg{Data: []byte(fmt.Sprintf("%v", i))}); err != nil {
				t.Errorf("Error: %v", err)
			}
			time.Sleep(time.Duration(i) * 100 * time.Millisecond) //Sleep is bad, but we have a libp2p issue here.
		}(i)
	}

	for i := 0; i < 4; i++ {
		select {
		case msg := <-msgChan:
			glog.Infof("%v", string(msg.Data.(StreamDataMsg).Data))
		case <-time.After(time.Second * 5):
			t.Errorf("Timed out")
		}
	}
}
