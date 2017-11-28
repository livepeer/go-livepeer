package basicnet

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/common"
)

func TestRestream(t *testing.T) {
	n1, n2 := setupNodes(15000, 15001)
	defer n1.NetworkNode.PeerHost.Close()
	defer n2.NetworkNode.PeerHost.Close()

	connectHosts(n1.NetworkNode.PeerHost, n2.NetworkNode.PeerHost)
	go n1.SetupProtocol()
	go n2.SetupProtocol()

	//Set up 1 broadcaster on n1
	strmID1 := fmt.Sprintf("%vOriginalStrm", n1.GetNodeID())
	n1b1, err := n1.GetBroadcaster(strmID1)
	if err != nil {
		t.Errorf("Error: %v", err)
	}

	//Set up n2
	sub, err := n2.GetSubscriber(strmID1)
	if err != nil {
		t.Errorf("Error: %v", err)
	}

	common.WaitUntil(time.Second, func() bool {
		return len(n1.broadcasters) > 0
	})
	subChan := make(chan bool)
	subEofChan := make(chan bool)
	sub.Subscribe(context.Background(), func(seqNo uint64, data []byte, eof bool) {
		if !eof {
			subChan <- true
		} else {
			subEofChan <- true
		}
	})

	//Wait until the broadcaster's listener is set up
	common.WaitUntil(time.Second, func() bool {
		return len(n1.broadcasters[strmID1].listeners) > 0
	})

	//Broadcast stream
	for i := 0; i < 10; i++ {
		if err := n1b1.Broadcast(0, []byte(fmt.Sprintf("hello, %v", i))); err != nil {
			t.Errorf("Error broadcasting: %v", err)
		}
		time.Sleep(time.Millisecond * 50)
	}

	//Should get the broadcasted message
	select {
	case <-subChan:
		//Success!
	case <-time.After(time.Second):
		t.Errorf("Timed out")
	}

	//Send Finish
	if err := n1b1.Finish(); err != nil {
		t.Errorf("Error finishing stream: %v", err)
	}

	//Should get an EOF message
	select {
	case <-subEofChan:
		//Success!
	case <-time.After(time.Second):
		t.Errorf("Timed out")
	}

	glog.Infof("\n\nDone with stream1, now stream2\n\n")

	strmID2 := fmt.Sprintf("%vOriginalStrm2", n1.GetNodeID())
	n1b2, err := n1.GetBroadcaster(strmID2)
	if err != nil {
		t.Errorf("Error: %v", err)
	}

	sub2, err := n2.GetSubscriber(strmID2)
	if err != nil {
		t.Errorf("Error: %v", err)
	}

	common.WaitUntil(time.Second, func() bool {
		return len(n1.broadcasters) > 1
	})
	sub2Chan := make(chan bool)
	sub2EofChan := make(chan bool)
	sub2.Subscribe(context.Background(), func(seqNo uint64, data []byte, eof bool) {
		if !eof {
			glog.Infof("n2 got data: %v", data)
			sub2Chan <- true
		} else {
			glog.Infof("n2 go eof")
			sub2EofChan <- true
		}
	})

	//Wait until the broadcaster's listener is set up
	common.WaitUntil(time.Second, func() bool {
		return len(n1.broadcasters[strmID2].listeners) > 0
	})
	n1b2.Broadcast(0, []byte("hola"))
	select {
	case <-sub2Chan:
		//Success!
	case <-time.After(time.Second):
		t.Errorf("Timed out")
	}

	n1b2.Finish()
	select {
	case <-sub2EofChan:
		//Success!
	case <-time.After(time.Second):
		t.Errorf("Timed out")
	}
}

func TestABS(t *testing.T) {
	// //Set up 3 nodes.  n1=broadcaster, n2=transcoder, n3=subscriber
	// n1, n2 := setupNodes(15000, 15001)
	// n3, n4 := setupNodes(15002, 15003)
	// defer n1.NetworkNode.PeerHost.Close()
	// defer n2.NetworkNode.PeerHost.Close()
	// defer n3.NetworkNode.PeerHost.Close()
	// defer n4.NetworkNode.PeerHost.Close()
	// connectHosts(n1.NetworkNode.PeerHost, n2.NetworkNode.PeerHost)
	// connectHosts(n2.NetworkNode.PeerHost, n3.NetworkNode.PeerHost)
	// go n1.SetupProtocol()
	// go n2.SetupProtocol()
	// go n3.SetupProtocol()

	// //Broadcast 1 stream to n1
	// strmID1 := fmt.Sprintf("%vOriginalStrm", n1.GetNodeID())
	// n1b1, err := n1.GetBroadcaster(strmID1)
	// if err != nil {
	// 	glog.Errorf("Error: %v", err)
	// }
	// if len(n1.broadcasters) != 1 {
	// 	t.Errorf("Expecting 1 broadcaster for n1 but got :%v", n1.broadcasters)
	// }
	// if len(n1.relayers) != 0 {
	// 	t.Errorf("Expecting 0 relayers for n1 but got :%v", n1.broadcasters)
	// }
	// if len(n1.subscribers) != 0 {
	// 	t.Errorf("Expecting 0 subscribers for n1 but got :%v", n1.broadcasters)
	// }
	// if len(n1.broadcasters[strmID1].listeners) != 0 {
	// 	t.Errorf("Expecting 0 listeners for n1 broadcaster, but got %v", n1.broadcasters[strmID1].listeners)
	// }

	// //n2 subscribes to the stream, creates 2 streams locally (trasncoded streams)
	// sub, err := n2.GetSubscriber(strmID1)
	// if err != nil {
	// 	glog.Errorf("Error: %v", err)
	// }
	// strmID2 := fmt.Sprintf("%vTranscodedStrm1", n1.GetNodeID())
	// strmID3 := fmt.Sprintf("%vTranscodedStrm2", n1.GetNodeID())
	// n2b1, err := n2.GetBroadcaster(strmID2)
	// if err != nil {
	// 	glog.Errorf("Error: %v", err)
	// }
	// n2b2, err := n2.GetBroadcaster(strmID3)
	// if err != nil {
	// 	glog.Errorf("Error: %v", err)
	// }
	// n2gotdata := make(chan struct{})
	// sub.Subscribe(context.Background(), func(seqNo uint64, data []byte, eof bool) {
	// 	glog.Infof("n2 got video data: %v", seqNo)
	// 	n2gotdata <- struct{}{}
	// 	n2b1.Broadcast(seqNo, []byte(fmt.Sprintf("%strans1", data)))
	// 	n2b2.Broadcast(seqNo, []byte(fmt.Sprintf("%strans2", data)))
	// })
	// //Wait until n2 gets data, this ensures everything is hooked up.
	// n1b1.Broadcast(0, []byte("test"))
	// timer := time.NewTimer(time.Millisecond * 500)
	// select {
	// case <-n2gotdata:
	// case <-timer.C:
	// 	t.Errorf("Timed out")
	// }
	// if len(n2.broadcasters) != 2 {
	// 	t.Errorf("Expecting 2 broadcaster for n2 but got :%v", n1.broadcasters)
	// }
	// for _, b := range n2.broadcasters {
	// 	if len(b.listeners) != 0 {
	// 		t.Errorf("Expecting 0 listeners in n2 broadcasters, got %v", b.listeners)
	// 	}
	// }
	// if len(n2.relayers) != 0 {
	// 	t.Errorf("Expecting 0 relayers for n2 but got :%v", n1.broadcasters)
	// }
	// if len(n2.subscribers) != 1 {
	// 	t.Errorf("Expecting 1 subscribers for n2 but got :%v", n1.broadcasters)
	// }
	// if len(n1.broadcasters[strmID1].listeners) != 1 {
	// 	t.Errorf("Expecting 1 listener for n1 broadcaster, but got %v", n1.broadcasters[strmID1].listeners)
	// }
	// if l, ok := n1.broadcasters[strmID1].listeners[n2.GetNodeID()]; !ok {
	// 	t.Errorf("Expecting listener for n1 broadcaster to be %v, but got %v", n2.GetNodeID(), l)
	// }

	// //n3 subscribes to all 3 streams first
	// n3sub1, err := n3.GetSubscriber(strmID1)
	// if err != nil {
	// 	glog.Errorf("Error: %v", err)
	// }
	// n3sub2, err := n3.GetSubscriber(strmID2)
	// if err != nil {
	// 	glog.Errorf("Error: %v", err)
	// }
	// n3sub3, err := n3.GetSubscriber(strmID3)
	// if err != nil {
	// 	glog.Errorf("Error: %v", err)
	// }
	// n3sub1.Subscribe(context.Background(), func(seqNo uint64, data []byte, eof bool) {
	// 	glog.Infof("n3sub1 got data: %v", string(data))
	// })
	// n3sub2.Subscribe(context.Background(), func(seqNo uint64, data []byte, eof bool) {
	// 	glog.Infof("n3sub2 got data: %v", string(data))
	// })
	// time.Sleep(time.Millisecond * 500)
	// n3gotdata := make(chan struct{})
	// n3sub3.Subscribe(context.Background(), func(seqNo uint64, data []byte, eof bool) {
	// 	glog.Infof("n3sub3 got data: %v", string(data))
	// 	n3gotdata <- struct{}{}
	// })
	// n1b1.Broadcast(0, []byte("test"))
	// timer = time.NewTimer(time.Millisecond * 2000)
	// select {
	// case <-n3gotdata:
	// case <-timer.C:
	// 	t.Errorf("Timed out")
	// }
	// if len(n3.subscribers) != 3 {
	// 	t.Errorf("Expecting 3 subscribers in n3, but got %v", n3.subscribers)
	// }
	// if len(n3.relayers) != 0 {
	// 	t.Errorf("Expecting 0 relayers in n3, but got %v", n3.relayers)
	// }
	// if len(n3.broadcasters) != 0 {
	// 	t.Errorf("Expecting 0 broadcasters in n3, but got %v", n3.broadcasters)
	// }
	// if len(n2.relayers) != 1 {
	// 	t.Errorf("Expecting 1 relayer in n2, but got %v", n2.relayers)
	// }
	// if len(n2.relayers[relayerMapKey(strmID1, SubReqID)].listeners) != 1 {
	// 	t.Errorf("Expecting 1 listener in n2 relayer, got %v", n2.relayers[relayerMapKey(strmID1, SubReqID)].listeners)
	// }
	// if len(n2.broadcasters) != 2 {
	// 	t.Errorf("Expecting 2 broadcasters in n2, got %v", n2.broadcasters)
	// }
	// if len(n2.broadcasters[strmID2].listeners) != 1 {
	// 	t.Errorf("Expecting 1 listener in n2 broadcaster, got %v", n2.broadcasters[strmID2].listeners)
	// }
	// if len(n2.broadcasters[strmID3].listeners) != 1 {
	// 	t.Errorf("Expecting 1 listener in n2 broadcaster, got %v", n2.broadcasters[strmID3].listeners)
	// }

	// //Send some segments to n1b1 to make sure they go all the way through
	// n1b1.Broadcast(0, []byte("seg1"))

	// //n3 drops the subscription for 2 streams half-way through
}
