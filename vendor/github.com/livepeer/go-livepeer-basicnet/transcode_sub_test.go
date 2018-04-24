package basicnet

import (
	"context"
	"fmt"
	peer "gx/ipfs/QmZoWKhxUmZ2seW4BzX6fJkNR8hh9PsGModr7q171yq2SS/go-libp2p-peer"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/golang/glog"
)

func (n *BasicVideoNetwork) peerListIs(plist []peer.ID) bool {
	a_sorted := make([]peer.ID, len(plist))
	b_sorted := []peer.ID{}
	copy(a_sorted, plist)
	// exclude self from b_list
	for _, v := range n.NetworkNode.GetPeers() {
		if string(v) == string(n.NetworkNode.ID()) {
			continue
		}
		b_sorted = append(b_sorted, v)
	}
	if len(a_sorted) != len(b_sorted) {
		return false
	}
	sort.Sort(peer.IDSlice(a_sorted))
	sort.Sort(peer.IDSlice(b_sorted))
	for i := range a_sorted {
		if string(a_sorted[i]) != string(b_sorted[i]) {
			return false
		}
	}
	return true
}

func TestTranscodeSubVerificationFail(t *testing.T) {
	glog.Infof("\n\nTesting TranscodesubVerificationFail...")
	// only way we really have to check for failed TranscodeSub sig verification
	// is to check some counters and ensure a connection isn't established
	n1, n2 := setupNodes(t, 15000, 15001)
	n3, _ := setupNodes(t, 15002, 15003)
	defer n1.NetworkNode.(*BasicNetworkNode).PeerHost.Close()
	defer n2.NetworkNode.(*BasicNetworkNode).PeerHost.Close()
	defer n3.NetworkNode.(*BasicNetworkNode).PeerHost.Close()
	connectHosts(n1.NetworkNode.(*BasicNetworkNode).PeerHost, n3.NetworkNode.(*BasicNetworkNode).PeerHost)
	connectHosts(n2.NetworkNode.(*BasicNetworkNode).PeerHost, n3.NetworkNode.(*BasicNetworkNode).PeerHost)

	// this should stop the transmission in the middle
	n3.NetworkNode.SetVerifyTranscoderSig(func([]byte, []byte, string) bool {
		return false
	})

	// Sanity check that we do have connections
	if !n2.peerListIs([]peer.ID{n3.NetworkNode.ID()}) || !n1.peerListIs([]peer.ID{n3.NetworkNode.ID()}) {
		t.Error("Did not have expected peer list")
	}

	strmID := fmt.Sprintf("%vstrmID", peer.IDHexEncode(n2.NetworkNode.(*BasicNetworkNode).Identity))
	result := make(map[uint64][]byte)
	lock := &sync.Mutex{}
	SetTranscodeSubTimeout(25 * time.Millisecond) // speed up for testing

	n1.TranscodeSub(context.Background(), strmID, func(seqNo uint64, data []byte, eof bool) {
		// glog.Infof("Got response: %v, %v", seqNo, data)
		lock.Lock()
		result[seqNo] = data
		lock.Unlock()
	})
	time.Sleep(time.Millisecond * 110) // allow TranscodeSub goroutine to run

	// check connection set is the same and a direct cxn wasn't established
	if !n2.peerListIs([]peer.ID{n3.NetworkNode.ID()}) || !n1.peerListIs([]peer.ID{n3.NetworkNode.ID()}) {
		t.Error("Did not have expected peer list")
	}

	s1tmp, _ := n1.GetSubscriber(strmID)
	s1, _ := s1tmp.(*BasicSubscriber)
	if n3.msgCounts[TranscodeSubID] < 2 || s1.subAttempts < 2 {
		t.Error("Did not try to re-transmit a few times")
	}
	if n3.msgCounts[TranscodeSubID] != int64(s1.subAttempts) {
		t.Error("Did not receive the expected number of transcode subs")
	}
	if n2.msgCounts[TranscodeSubID] > 0 {
		t.Error("Should not have received any transcode sub requests")
	}
	if len(result) != 0 {
		t.Error("Somehow got results when there should have been none")
	}
}

func TestTranscodeSubSeparateCxn(t *testing.T) {
	glog.Infof("\n\nTesting TranscodesubSeparateCxn...")
	n1, n3 := setupNodes(t, 15000, 15001)
	n2, _ := setupNodes(t, 15002, 15003)
	defer n1.NetworkNode.(*BasicNetworkNode).PeerHost.Close()
	defer n2.NetworkNode.(*BasicNetworkNode).PeerHost.Close()
	defer n3.NetworkNode.(*BasicNetworkNode).PeerHost.Close()
	connectHosts(n1.NetworkNode.(*BasicNetworkNode).PeerHost, n3.NetworkNode.(*BasicNetworkNode).PeerHost)
	connectHosts(n2.NetworkNode.(*BasicNetworkNode).PeerHost, n3.NetworkNode.(*BasicNetworkNode).PeerHost)

	// Sanity check that no connection exists between broadcaster (n2) and sub (n1)
	if !n2.peerListIs([]peer.ID{n3.NetworkNode.ID()}) {
		t.Error("Broadcaster did not have expected peer list (check #1)")
	}

	strmID := fmt.Sprintf("%vstrmID", peer.IDHexEncode(n2.NetworkNode.(*BasicNetworkNode).Identity))
	bcaster, _ := n2.GetBroadcaster(strmID)
	result := make(map[uint64][]byte)
	lock := &sync.Mutex{}
	n1.TranscodeSub(context.Background(), strmID, func(seqNo uint64, data []byte, eof bool) {
		// glog.Infof("Got response: %v, %v", seqNo, data)
		lock.Lock()
		result[seqNo] = data
		lock.Unlock()
	})
	time.Sleep(time.Millisecond * 50) // allow TranscodeSub goroutine to run

	s1tmp, _ := n1.GetSubscriber(strmID)
	s1, _ := s1tmp.(*BasicSubscriber)
	if s1.cancelWorker == nil {
		t.Errorf("Cancel function should be assigned")
	}
	if !s1.working {
		t.Errorf("Subscriber should be working")
	}

	// XXX find a way to check subMsg.StrmID
	time.Sleep(time.Millisecond * 50)

	for i := 0; i < 10; i++ {
		time.Sleep(time.Millisecond * 50)
		bcaster.Broadcast(uint64(i), []byte("test data"))
	}

	for start := time.Now(); time.Since(start) < 3*time.Second; {
		if len(result) == 10 {
			break
		} else {
			time.Sleep(time.Millisecond * 50)
		}
	}
	if len(result) != 10 {
		t.Errorf("Expecting length of result to be 10, but got %v: %v", len(result), result)
	}

	for _, d := range result {
		if string(d) != "test data" {
			t.Errorf("Expecting data to be 'test data', but got %v", d)
		}
	}

	// Sanity check that a connection exists between broadcaster (n2) and sub
	if !n2.peerListIs([]peer.ID{n1.NetworkNode.ID(), n3.NetworkNode.ID()}) {
		t.Error("Broadcaster did not have expected peer list (check #2)")
	}
	// Ensure that the relay no longer has the stream
	if n1.getSubscriber(strmID) == nil ||
		bcaster.(*BasicBroadcaster).listeners[peer.IDHexEncode(n1.NetworkNode.ID())] == nil ||
		n3.relayers[relayerMapKey(strmID, SubReqID)] != nil {
		t.Error("Subscriptions not set up as expected on nodes", n3.NetworkNode.ID(), n3.relayers)
	}

	//Call cancel
	s1.cancelWorker()

	// XXX find a way to check cancelMsg.StrmID

	time.Sleep(time.Millisecond * 100)

	if s1.working {
		t.Errorf("subscriber shouldn't be working after 'cancel' is called")
	}
}

func TestTranscodeSubExistingCxn(t *testing.T) {
	glog.Infof("\n\nTesting TranscodesubExistingCxn...")
	// test the case where they are previously directly connnected
	n1, n2 := setupNodes(t, 15000, 15001)
	defer n1.NetworkNode.(*BasicNetworkNode).PeerHost.Close()
	defer n2.NetworkNode.(*BasicNetworkNode).PeerHost.Close()
	connectHosts(n1.NetworkNode.(*BasicNetworkNode).PeerHost, n2.NetworkNode.(*BasicNetworkNode).PeerHost)

	// Sanity check that the connection exists between broadcaster (n2) and sub
	if !n2.peerListIs([]peer.ID{n1.NetworkNode.ID()}) {
		t.Error("Broadcaster did not have expected peer list (check #1)")
	}

	strmID := fmt.Sprintf("%vstrmID", peer.IDHexEncode(n2.NetworkNode.(*BasicNetworkNode).Identity))
	bcaster, _ := n2.GetBroadcaster(strmID)
	result := make(map[uint64][]byte)
	lock := &sync.Mutex{}
	n1.TranscodeSub(context.Background(), strmID, func(seqNo uint64, data []byte, eof bool) {
		// glog.Infof("Got response: %v, %v", seqNo, data)
		lock.Lock()
		result[seqNo] = data
		lock.Unlock()
	})
	time.Sleep(time.Millisecond * 50) // allow TranscodeSub goroutine to run

	s1tmp, _ := n1.GetSubscriber(strmID)
	s1, _ := s1tmp.(*BasicSubscriber)
	if s1.cancelWorker == nil {
		t.Errorf("Cancel function should be assigned")
	}
	if !s1.working {
		t.Errorf("Subscriber should be working")
	}

	// XXX find a way to check subMsg.StrmID
	time.Sleep(time.Millisecond * 50)

	for i := 0; i < 10; i++ {
		time.Sleep(time.Millisecond * 50)
		bcaster.Broadcast(uint64(i), []byte("test data"))
	}

	for start := time.Now(); time.Since(start) < 3*time.Second; {
		if len(result) == 10 {
			break
		} else {
			time.Sleep(time.Millisecond * 50)
		}
	}
	if len(result) != 10 {
		t.Errorf("Expecting length of result to be 10, but got %v: %v", len(result), result)
	}

	for _, d := range result {
		if string(d) != "test data" {
			t.Errorf("Expecting data to be 'test data', but got %v", d)
		}
	}

	// Check that a single connection exists between broadcaster (n2) and sub
	// and that there aren't eg, duplicate connections from the TranscodeSub
	if !n2.peerListIs([]peer.ID{n1.NetworkNode.ID()}) {
		t.Error("Broadcaster did not have expected peer list (check #2)")
	}

	//Call cancel
	s1.cancelWorker()

	// XXX find a way to check cancelMsg.StrmID

	time.Sleep(time.Millisecond * 100)

	if s1.working {
		t.Errorf("subscriber shouldn't be working after 'cancel' is called")
	}
}

//	the conflicting ports give very strange behavior
func TestTranscodeSubUnreachable(t *testing.T) {
	glog.Infof("\n\nTesting TranscodesubUnreachable...")
	// Tests whether a transcoder node can still receive data even if unreachable
	n2, n3 := setupNodes(t, 15000, 15001)
	n1, _ := setupNodes(t, 15000, 15002) // make subscriber (n1) unreachable
	defer n1.NetworkNode.(*BasicNetworkNode).PeerHost.Close()
	defer n2.NetworkNode.(*BasicNetworkNode).PeerHost.Close()
	defer n3.NetworkNode.(*BasicNetworkNode).PeerHost.Close()
	connectHosts(n1.NetworkNode.(*BasicNetworkNode).PeerHost, n3.NetworkNode.(*BasicNetworkNode).PeerHost)
	connectHosts(n2.NetworkNode.(*BasicNetworkNode).PeerHost, n3.NetworkNode.(*BasicNetworkNode).PeerHost)

	// Sanity check that no connection exists between broadcaster (n2) and sub (n1)
	if !n2.peerListIs([]peer.ID{n3.NetworkNode.ID()}) {
		t.Error("Broadcaster did not have expected peer list (check #1)")
	}

	strmID := fmt.Sprintf("%vstrmID", peer.IDHexEncode(n2.NetworkNode.(*BasicNetworkNode).Identity))
	bcaster, _ := n2.GetBroadcaster(strmID)
	result := make(map[uint64][]byte)
	lock := &sync.Mutex{}
	n1.TranscodeSub(context.Background(), strmID, func(seqNo uint64, data []byte, eof bool) {
		// glog.Infof("Got response: %v, %v", seqNo, data)
		lock.Lock()
		result[seqNo] = data
		lock.Unlock()
	})
	time.Sleep(time.Millisecond * 50) // allow TranscodeSub goroutine to run

	s1tmp, _ := n1.GetSubscriber(strmID)
	s1, _ := s1tmp.(*BasicSubscriber)
	if s1.cancelWorker == nil {
		t.Error("Cancel function should be assigned")
	}
	if !s1.working {
		t.Error("Subscriber should be working")
	}

	// XXX find a way to check subMsg.StrmID
	time.Sleep(time.Millisecond * 50)

	for i := 0; i < 10; i++ {
		time.Sleep(time.Millisecond * 50)
		bcaster.Broadcast(uint64(i), []byte("test data"))
	}

	for start := time.Now(); time.Since(start) < 3*time.Second; {
		if len(result) == 10 {
			break
		} else {
			time.Sleep(time.Millisecond * 50)
		}
	}
	if len(result) != 10 {
		t.Errorf("Expecting length of result to be 10, but got %v: %v", len(result), result)
	}

	for _, d := range result {
		if string(d) != "test data" {
			t.Errorf("Expecting data to be 'test data', but got %v", d)
		}
	}

	// Ensure that no connection exists between broadcaster (n2) and sub
	if !n2.peerListIs([]peer.ID{n3.NetworkNode.ID()}) {
		t.Error("Broadcaster did not have expected peer list (check #2)", n2.NetworkNode.GetPeers())
	}
	// Ensure that the relay still has the stream
	if n1.getSubscriber(strmID) == nil ||
		bcaster.(*BasicBroadcaster).listeners[peer.IDHexEncode(n3.NetworkNode.ID())] == nil ||
		n3.relayers[relayerMapKey(strmID, SubReqID)] == nil || n3.relayers[relayerMapKey(strmID, SubReqID)].listeners[peer.IDHexEncode(n1.NetworkNode.ID())] == nil {
		t.Error("Subscriptions not set up as expected on nodes")
	}

	//Call cancel
	s1.cancelWorker()

	// XXX find a way to check cancelMsg.StrmID

	time.Sleep(time.Millisecond * 100)

	if s1.working {
		t.Error("subscriber shouldn't be working after 'cancel' is called")
	}

}

func TestTranscodeSubDelayedDirect(t *testing.T) {
	// Check that TranscodeSub is OK with delayed direct connections
	glog.Infof("\n\nTesting TranscodesubDelayedDirect...")

	// XXX Add another 'direct' test: when already connected but the response
	//     is just slow to come through on the wire. In theory that should
	//     trigger cancellations by the *data* handler, not the connection
	//     notifiee.
	// XXX Is the flow the same with connects + disc + reconnects?
	// 	   This is where stubs would come in handy.

	n1, n2 := setupNodes(t, 15000, 15001)
	defer n1.NetworkNode.(*BasicNetworkNode).PeerHost.Close()
	defer n2.NetworkNode.(*BasicNetworkNode).PeerHost.Close()

	strmID := fmt.Sprintf("%vstrmID", peer.IDHexEncode(n2.NetworkNode.(*BasicNetworkNode).Identity))
	bcaster, _ := n2.GetBroadcaster(strmID)
	result := make(map[uint64][]byte)
	lock := &sync.Mutex{}
	SetTranscodeSubTimeout(25 * time.Millisecond) // speed up for testing
	n1.TranscodeSub(context.Background(), strmID, func(seqNo uint64, data []byte, eof bool) {
		// glog.Infof("Got response: %v, %v", seqNo, data)
		lock.Lock()
		result[seqNo] = data
		lock.Unlock()
	})
	time.Sleep(time.Millisecond * 100) // allow TranscodeSub goroutine to run

	s1tmp, _ := n1.GetSubscriber(strmID)
	s1, _ := s1tmp.(*BasicSubscriber)
	if s1.subAttempts < 2 {
		t.Error("Did not try to re-transmit a few times")
	}
	if len(result) != 0 {
		t.Error("Somehow got results when there should have been none")
	}
	// Sanity check that there are *no* connections; we want it to fail
	if !n2.peerListIs([]peer.ID{}) || !n1.peerListIs([]peer.ID{}) {
		t.Error("Did not have expected peer list")
	}

	connectHosts(n1.NetworkNode.(*BasicNetworkNode).PeerHost, n2.NetworkNode.(*BasicNetworkNode).PeerHost)
	time.Sleep(time.Millisecond * 100) // allow TranscodeSub goroutine to run
	nbRetries := s1.subAttempts

	for i := 0; i < 10; i++ {
		time.Sleep(time.Millisecond * 50)
		bcaster.Broadcast(uint64(i), []byte("test data"))
	}

	for start := time.Now(); time.Since(start) < 3*time.Second; {
		if len(result) == 10 {
			break
		} else {
			time.Sleep(time.Millisecond * 50)
		}
	}
	if len(result) != 10 {
		t.Errorf("Expecting length of result to be 10, but got %v: %v", len(result), result)
	}
	for _, d := range result {
		if string(d) != "test data" {
			t.Errorf("Expecting data to be 'test data', but got %v", d)
		}
	}
	if nbRetries != s1.subAttempts {
		t.Error("Re-transmission of TranscodeSub did not stop")
	}

	//Call cancel
	s1.cancelWorker()
	// XXX find a way to check cancelMsg.StrmID
	time.Sleep(time.Millisecond * 100)
	if s1.working {
		t.Error("subscriber shouldn't be working after 'cancel' is called")
	}
}

func TestTranscodeSubDelayedRelay(t *testing.T) {
	glog.Infof("\n\nTesting TranscodesubDelayedRelay...")
	n1, n2 := setupNodes(t, 15000, 15001)
	n3, _ := setupNodes(t, 15002, 15003)
	defer n1.NetworkNode.(*BasicNetworkNode).PeerHost.Close()
	defer n2.NetworkNode.(*BasicNetworkNode).PeerHost.Close()
	defer n3.NetworkNode.(*BasicNetworkNode).PeerHost.Close()
	connectHosts(n1.NetworkNode.(*BasicNetworkNode).PeerHost, n3.NetworkNode.(*BasicNetworkNode).PeerHost)

	strmID := fmt.Sprintf("%vstrmID", peer.IDHexEncode(n2.NetworkNode.(*BasicNetworkNode).Identity))
	bcaster, _ := n2.GetBroadcaster(strmID)
	result := make(map[uint64][]byte)
	lock := &sync.Mutex{}
	SetTranscodeSubTimeout(25 * time.Millisecond) // speed up for testing
	n1.TranscodeSub(context.Background(), strmID, func(seqNo uint64, data []byte, eof bool) {
		// glog.Infof("Got response: %v, %v", seqNo, data)
		lock.Lock()
		result[seqNo] = data
		lock.Unlock()
	})
	time.Sleep(time.Millisecond * 100) // allow TranscodeSub goroutine to run

	s1tmp, _ := n1.GetSubscriber(strmID)
	s1, _ := s1tmp.(*BasicSubscriber)
	if s1.subAttempts < 2 {
		t.Error("Did not try to re-transmit a few times")
	}
	if len(result) != 0 {
		t.Error("Somehow got results when there should have been none")
	}
	// Sanity check that there are *no* connections to n2; we want it to fail
	if !n2.peerListIs([]peer.ID{}) || !n1.peerListIs([]peer.ID{n3.NetworkNode.ID()}) {
		t.Error("Did not have expected peer list")
	}

	connectHosts(n3.NetworkNode.(*BasicNetworkNode).PeerHost, n2.NetworkNode.(*BasicNetworkNode).PeerHost)
	time.Sleep(time.Millisecond * 100) // allow TranscodeSub goroutine to run
	nbRetries := s1.subAttempts

	for i := 0; i < 10; i++ {
		time.Sleep(time.Millisecond * 50)
		bcaster.Broadcast(uint64(i), []byte("test data"))
	}

	for start := time.Now(); time.Since(start) < 3*time.Second; {
		if len(result) == 10 {
			break
		} else {
			time.Sleep(time.Millisecond * 50)
		}
	}
	if len(result) != 10 {
		t.Errorf("Expecting length of result to be 10, but got %v: %v", len(result), result)
	}
	for _, d := range result {
		if string(d) != "test data" {
			t.Errorf("Expecting data to be 'test data', but got %v", d)
		}
	}
	if nbRetries != s1.subAttempts {
		t.Error("Re-transmission of TranscodeSub did not stop")
	}
	if !n2.peerListIs([]peer.ID{n3.NetworkNode.ID(), n1.NetworkNode.ID()}) ||
		!n1.peerListIs([]peer.ID{n3.NetworkNode.ID(), n2.NetworkNode.ID()}) {
		t.Error("Did not have expected peer list")
	}

	//Call cancel
	s1.cancelWorker()
	// XXX find a way to check cancelMsg.StrmID
	time.Sleep(time.Millisecond * 100)
	if s1.working {
		t.Error("subscriber shouldn't be working after 'cancel' is called")
	}
}

/*
XXX make this test run smoothly alongside everything else.
    seems to run fine standalone?
	the conflicting ports give very strange behavior
func TestTranscodeSubDelayedUnreachable(t *testing.T) {
	glog.Infof("\n\nTesting TranscodesubDelayedUnreachable...")
	// This particular test tries to hit the case where the direct cxn is
	// never established, so we need to cancel the TranscodeSub retransmission
	// upon receipt of our first data message.
	n2, n3 := setupNodes(t, 15000, 15001)
	n1, _ := setupNodes(t, 15000, 15000) // make subscriber (n1) unreachable
	defer n1.NetworkNode.(*BasicNetworkNode).PeerHost.Close()
	defer n2.NetworkNode.(*BasicNetworkNode).PeerHost.Close()
	defer n3.NetworkNode.(*BasicNetworkNode).PeerHost.Close()
	connectHosts(n1.NetworkNode.(*BasicNetworkNode).PeerHost, n3.NetworkNode.(*BasicNetworkNode).PeerHost)

	strmID := fmt.Sprintf("%vstrmID", peer.IDHexEncode(n2.NetworkNode.(*BasicNetworkNode).Identity))
	bcaster, _ := n2.GetBroadcaster(strmID)
	result := make(map[uint64][]byte)
	lock := &sync.Mutex{}
	SetTranscodeSubTimeout(25 * time.Millisecond) // speed up for testing
	n1.TranscodeSub(context.Background(), strmID, func(seqNo uint64, data []byte, eof bool) {
		// glog.Infof("Got response: %v, %v", seqNo, data)
		lock.Lock()
		result[seqNo] = data
		lock.Unlock()
	})
	time.Sleep(time.Millisecond * 100) // allow TranscodeSub goroutine to run

	s1tmp, _ := n1.GetSubscriber(strmID)
	s1, _ := s1tmp.(*BasicSubscriber)
	if s1.subAttempts < 2 {
		t.Error("Did not try to re-transmit a few times")
	}
	if len(result) != 0 {
		t.Error("Somehow got results when there should have been none")
	}
	// Sanity check that there are *no* connections to n2; we want it to fail
	if !n2.peerListIs([]peer.ID{}) || !n1.peerListIs([]peer.ID{n3.NetworkNode.ID()}) {
		t.Error("Did not have expected peer list")
	}

	connectHosts(n3.NetworkNode.(*BasicNetworkNode).PeerHost, n2.NetworkNode.(*BasicNetworkNode).PeerHost)
	time.Sleep(time.Millisecond * 100) // allow TranscodeSub goroutine to run
	// s.subAttempts will iterate until receiving the first broadcasted segment
	var nbRetries int

	for i := 0; i < 10; i++ {
		time.Sleep(time.Millisecond * 50)
		bcaster.Broadcast(uint64(i), []byte("test data"))
		if i == 0 {
			nbRetries = s1.subAttempts // should approximately stop here
		}
	}

	for start := time.Now(); time.Since(start) < 3*time.Second; {
		if len(result) == 10 {
			break
		} else {
			time.Sleep(time.Millisecond * 50)
		}
	}
	if len(result) != 10 {
		t.Errorf("Expecting length of result to be 10, but got %v: %v", len(result), result)
	}
	for _, d := range result {
		if string(d) != "test data" {
			t.Errorf("Expecting data to be 'test data', but got %v", d)
		}
	}
	if nbRetries != s1.subAttempts {
		t.Error("Re-transmission of TranscodeSub did not stop")
	}
	// verify we're still being relayed
	if !n2.peerListIs([]peer.ID{n3.NetworkNode.ID()}) ||
		!n1.peerListIs([]peer.ID{n3.NetworkNode.ID()}) {
		t.Error("Did not have expected peer list")
	}

	//Call cancel
	s1.cancelWorker()
	// XXX find a way to check cancelMsg.StrmID
	time.Sleep(time.Millisecond * 100)
	if s1.working {
		t.Error("subscriber shouldn't be working after 'cancel' is called")
	}
}

func TestTranscodeSubDelayedCancel(t *testing.T) {
	glog.Infof("\n\nTesting TranscodesubDelayedCancel...")
	// Check that retransmission stops if we cancel before receiving a response
	n1, n2 := setupNodes(t, 15000, 15001)
	n3, _ := setupNodes(t, 15002, 15003)
	defer n1.NetworkNode.(*BasicNetworkNode).PeerHost.Close()
	defer n2.NetworkNode.(*BasicNetworkNode).PeerHost.Close()
	defer n3.NetworkNode.(*BasicNetworkNode).PeerHost.Close()
	connectHosts(n1.NetworkNode.(*BasicNetworkNode).PeerHost, n3.NetworkNode.(*BasicNetworkNode).PeerHost)

	strmID := fmt.Sprintf("%vstrmID", peer.IDHexEncode(n2.NetworkNode.(*BasicNetworkNode).Identity))
	SetTranscodeSubTimeout(25 * time.Millisecond) // speed up for testing
	n1.TranscodeSub(context.Background(), strmID, func(seqNo uint64, data []byte, eof bool) {
	})

	time.Sleep(time.Millisecond * 100) // allow TranscodeSub goroutine to run

	s1tmp, _ := n1.GetSubscriber(strmID)
	s1, _ := s1tmp.(*BasicSubscriber)
	if s1.subAttempts < 2 {
		t.Error("Did not try to re-transmit a few times")
	}
	s1.cancelWorker()
	nbRetries := s1.subAttempts
	time.Sleep(time.Millisecond * 100)
	if nbRetries != s1.subAttempts {
		t.Error("Re-transmission of TranscodeSub did not stop")
	}
	if s1.working {
		t.Error("subscriber shouldn't be working after 'cancel' is called")
	}
}
*/
