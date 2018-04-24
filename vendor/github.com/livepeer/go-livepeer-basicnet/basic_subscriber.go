package basicnet

import (
	"context"
	"errors"
	"fmt"
	"time"

	kb "gx/ipfs/QmTH6VLu3WXfbH3nuLdmscgPWuiPZv3GMJ2YCdzBS5z91T/go-libp2p-kbucket"
	ma "gx/ipfs/QmWWQ2Txc2c6tqjsBpzg5Ar652cHPGNsQQp2SejkNmkUMb/go-multiaddr"
	inet "gx/ipfs/QmXfkENeeBvh3zYA51MaSdGUdBjhQ99cP5WQe8zgr6wchG/go-libp2p-net"
	peer "gx/ipfs/QmZoWKhxUmZ2seW4BzX6fJkNR8hh9PsGModr7q171yq2SS/go-libp2p-peer"

	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/common"
)

var SubscriberDataInsertTimeout = time.Second * 300
var InsertDataWaitTime = time.Second * 10
var ErrSubscriber = errors.New("ErrSubscriber")
var TranscodeSubTimeout = time.Second * 60

//BasicSubscriber keeps track of
type BasicSubscriber struct {
	Network *BasicVideoNetwork
	// host    host.Host
	msgChan      chan StreamDataMsg
	subResponded context.CancelFunc
	subAttempts  int
	// networkStream *BasicStream
	StrmID       string
	UpstreamPeer peer.ID
	working      bool
	cancelWorker context.CancelFunc
}

func SetTranscodeSubTimeout(to time.Duration) {
	TranscodeSubTimeout = to
}

func (s *BasicSubscriber) InsertData(sd *StreamDataMsg) error {
	go func(sd *StreamDataMsg) {
		if s.working {
			timer := time.NewTimer(InsertDataWaitTime)
			select {
			case s.msgChan <- *sd:
				// glog.V(4).Infof("Data segment %v for %v inserted. (%v)", sd.SeqNo, sd.StrmID, time.Since(start))
			case <-timer.C:
				glog.Errorf("Subscriber data insert timed out: %v", sd.StrmID)
			}
		}
	}(sd)
	return nil
}

//Subscribe kicks off a go routine that calls the gotData func for every new video chunk
func (s *BasicSubscriber) Subscribe(ctx context.Context, gotData func(seqNo uint64, data []byte, eof bool)) error {
	glog.Infof("%v Sending SubReq message", s.Network.NetworkNode.ID())
	return s.sendSub(ctx, SubReqID, SubReqMsg{StrmID: s.StrmID}, gotData)
}

func (s *BasicSubscriber) TranscoderSubscribe(ctx context.Context, gotData func(seqNo uint64, data []byte, eof bool)) error {
	ts := TranscodeSubMsg{
		MultiAddrs: s.Network.NetworkNode.Host().Addrs(),
		NodeID:     s.Network.NetworkNode.ID(),
		StrmID:     s.StrmID,
	}
	sig, err := s.Network.NetworkNode.Sign(ts.BytesForSigning())
	if err != nil {
		glog.Errorf("Error signing TranscodeSubMsg: %v", err)
		return err
	}
	ts.Sig = sig
	s.Network.NetworkNode.Host().Network().Notify(s)
	glog.Infof("%v Sending TranscodeSub message for %v", s.Network.NetworkNode.ID(), s.StrmID)

	// periodic retransmit in case the broadcaster misses the first message(s)
	// XXX should terminate after endBlock is reached if still going
	sendCtx, sendCancel := context.WithCancel(context.Background())
	s.subResponded = sendCancel

	go func() {
		for {
			s.subAttempts++
			s.sendSub(ctx, TranscodeSubID, ts, gotData)
			timer := time.NewTimer(TranscodeSubTimeout)
			select {
			case <-sendCtx.Done():
				glog.Infof("%v Stopping TranscodeSub retransmit timer for %v", s.Network.NetworkNode.ID(), s.StrmID)
				return
			case <-timer.C:
				glog.Infof("%v Retransmitting TranscodeSub for %v", s.Network.NetworkNode.ID(), s.StrmID)
			}
		}
	}()
	return nil
}

func (s *BasicSubscriber) sendSub(ctx context.Context, opCode Opcode, msg interface{}, gotData func(seqNo uint64, data []byte, eof bool)) error {
	//Do we already have the broadcaster locally? If we do, just subscribe to it and listen.
	if b := s.Network.broadcasters[s.StrmID]; b != nil {
		localS := NewLocalOutStream(s)
		b.AddListeningStream("localSub", localS)

		ctxW, cancel := context.WithCancel(context.Background())
		s.cancelWorker = cancel
		s.working = true
		s.startWorker(ctxW, nil, gotData)
		return nil
	}

	//If we don't, send subscribe request, listen for response
	localPeers := s.Network.NetworkNode.GetPeers()
	if len(localPeers) == 1 {
		glog.Errorf("No local peers")
		return ErrSubscriber
	}
	targetPid, err := extractNodeID(s.StrmID)
	if err != nil {
		glog.Errorf("Error extracting node id from streamID: %v", s.StrmID)
		return ErrSubscriber
	}
	if s.IsLive() {
		// this is a re-transmission; reuse the same relay
		// avoids clobbering the context/worker
		ns := s.Network.NetworkNode.GetOutStream(s.UpstreamPeer)
		if ns != nil {
			err = s.Network.sendMessageWithRetry(s.UpstreamPeer, ns, opCode, msg)
			if err != nil {
				glog.Errorf("%v Error sending message %v to %v", s.Network.NetworkNode.ID(), opCode, s.UpstreamPeer)
				return err
			}
			return nil
		} // no outstream? so continue and maybe find another peer?
	}

	peers := kb.SortClosestPeers(localPeers, kb.ConvertPeerID(targetPid))

	for _, p := range peers {
		if p == s.Network.NetworkNode.ID() {
			continue
		}
		//Question: Where do we close the stream? If we only close on "Unsubscribe", we may leave some streams open...
		glog.V(5).Infof("New peer from kademlia: %v", peer.IDHexEncode(p))
		ns := s.Network.NetworkNode.GetOutStream(p)
		if ns != nil {
			err = s.Network.sendMessageWithRetry(p, ns, opCode, msg)
			if err != nil {
				glog.Errorf("Error sending message %v to %v", opCode, p)
				return err
			}
			ctxW, cancel := context.WithCancel(context.Background())
			s.cancelWorker = cancel
			s.working = true
			// s.networkStream = ns
			s.UpstreamPeer = p
			s.startWorker(ctxW, ns, gotData)
			return nil
		}
	}

	glog.Errorf("Cannot subscribe from any of the peers: %v", peers)
	return ErrNoClosePeers

	//Call gotData for every new piece of data
}

func (s *BasicSubscriber) startWorker(ctxW context.Context, ws *BasicOutStream, gotData func(seqNo uint64, data []byte, eof bool)) {
	//We expect StreamDataMsg to come back
	go func() {
		for {
			//Get message from the msgChan (inserted from the network by StreamDataMsg)
			//Call gotData(seqNo, data)
			//Question: What happens if the handler gets stuck?
			start := time.Now()
			select {
			case msg := <-s.msgChan:
				networkWaitTime := time.Since(start)
				if s.subResponded != nil {
					s.subResponded()
				}
				go gotData(msg.SeqNo, msg.Data, false)
				glog.V(common.DEBUG).Infof("Subscriber worker inserted segment: %v - took %v in total, %v waiting for data", msg.SeqNo, time.Since(start), networkWaitTime)
			case <-ctxW.Done():
				// s.networkStream = nil
				if s.subResponded != nil {
					s.subResponded() // just in case it's still waiting
				}
				s.working = false
				glog.Infof("Done with subscription, sending CancelSubMsg")
				//Send EOF
				go gotData(0, nil, true)
				if ws != nil {
					if err := s.Network.sendMessageWithRetry(ws.Stream.Conn().RemotePeer(), ws, CancelSubID, CancelSubMsg{StrmID: s.StrmID}); err != nil {
						glog.Errorf("Error sending CancelSubMsg during worker cancellation: %v", err)
					}
				}
				return
			}
		}
	}()
}

//Unsubscribe unsubscribes from the broadcast
func (s *BasicSubscriber) Unsubscribe() error {
	if s.cancelWorker != nil {
		s.cancelWorker()
	}

	//Remove self from local broadcaster listener pool if it's in there
	if b := s.Network.broadcasters[s.StrmID]; b != nil {
		delete(b.listeners, "localSub")
	}

	//Remove self from network
	delete(s.Network.subscribers, s.StrmID)
	s.Network.NetworkNode.Host().Network().StopNotify(s)

	return nil
}

func (s BasicSubscriber) String() string {
	return fmt.Sprintf("StreamID: %v, working: %v", s.StrmID, s.working)
}

func (s *BasicSubscriber) IsLive() bool {
	return s.working
}

// Notifiee
func (s *BasicSubscriber) HandleConnection(conn inet.Conn) {
}
func (s *BasicSubscriber) Listen(n inet.Network, m ma.Multiaddr) {
}
func (s *BasicSubscriber) ListenClose(n inet.Network, m ma.Multiaddr) {
}
func (s *BasicSubscriber) OpenedStream(n inet.Network, st inet.Stream) {
}
func (s *BasicSubscriber) ClosedStream(n inet.Network, st inet.Stream) {
}
func (s *BasicSubscriber) Connected(n inet.Network, conn inet.Conn) {
	glog.Infof("%v Connected from %v; processing", conn.LocalPeer(), conn.RemotePeer())
	broadcasterPid, err := extractNodeID(s.StrmID)
	if err != nil {
		glog.Errorf("%v Unable to extract NodeID from %v", conn.LocalPeer(), s.StrmID)
		return
	}
	if conn.RemotePeer() != broadcasterPid {
		glog.Infof("%v subscriber got a connection from a non-sub %v", conn.LocalPeer(), conn.RemotePeer())
		return
	}
	glog.Infof("%v Being a good network citizen to %v; processing", conn.LocalPeer(), conn.RemotePeer())
	go func() {
		// Be a good network citizen -- cancel the original sub through the relay
		if ns := s.Network.NetworkNode.GetOutStream(s.UpstreamPeer); ns != nil {
			if err := s.Network.sendMessageWithRetry(s.UpstreamPeer, ns, CancelSubID, CancelSubMsg{StrmID: s.StrmID}); err != nil {
				glog.Errorf("%v Unable to cancel subscription to upstream relay %s", s.Network.NetworkNode.ID(), peer.IDHexEncode(s.UpstreamPeer))
			}
		}
		// check for duplicated cxns or subs?
		ns := s.Network.NetworkNode.GetOutStream(conn.RemotePeer())
		if ns == nil {
			glog.Errorf("%v Unable to create an outstream with %v", conn.LocalPeer(), conn.RemotePeer())
			return
		}
		s.UpstreamPeer = conn.RemotePeer()
		if s.subResponded != nil {
			s.subResponded()
		} else {
			glog.Errorf("Received a direct cxn from broadcaster without corresponding TranscodeSub; how did this happen? StrmID %v", s.StrmID)
		}
		glog.Infof("%v Subscriber got direct connection from %v", conn.LocalPeer(), conn.RemotePeer())
	}()
}
func (s *BasicSubscriber) Disconnected(n inet.Network, conn inet.Conn) {
	// Resend TranscodeSub periodically if necessary?
}
