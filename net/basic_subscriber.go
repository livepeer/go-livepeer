package net

import (
	"context"
	"time"

	"github.com/golang/glog"
	host "github.com/libp2p/go-libp2p-host"
	peer "github.com/libp2p/go-libp2p-peer"
)

var SubscriberDataInsertTimeout = time.Second * 5

//BasicSubscriber keeps track of
type BasicSubscriber struct {
	Network       *BasicVideoNetwork
	host          host.Host
	msgChan       chan StreamDataMsg
	networkStream *BasicStream
	// q       *list.List
	// lock    *sync.Mutex
	StrmID       string
	working      bool
	cancelWorker context.CancelFunc

	// listeners map[string]net.Stream
}

//Subscribe kicks off a go routine that calls the gotData func for every new video chunk
func (s *BasicSubscriber) Subscribe(ctx context.Context, gotData func(seqNo uint64, data []byte, eof bool)) error {
	// glog.Infof("s: %v", s)
	// glog.Infof("s.Network: %v", s.Network)
	// glog.Infof("s.Network.broadcasters:%v", s.Network.broadcasters)
	//Do we already have the broadcaster locally?
	b := s.Network.broadcasters[s.StrmID]

	//If we do, just subscribe to it and listen.
	if b != nil {
		glog.Infof("Broadcaster is present - let's just read from that...")
		//TODO: read from broadcaster
		return nil
	}

	//If we don't, send subscribe request, listen for response
	peerc, err := s.Network.NetworkNode.Kad.GetClosestPeers(ctx, s.StrmID)
	if err != nil {
		glog.Errorf("Network Subscribe Error: %v", err)
		return err
	}

	//We can range over peerc because we know it'll be closed by libp2p
	//We'll keep track of all the connections on the
	for p := range peerc {
		//Question: Where do we close the stream? If we only close on "Unsubscribe", we may leave some streams open...
		ns := s.Network.NetworkNode.GetStream(p)
		if ns != nil {
			//Set up handler for the stream
			go func() {
				for {
					err := streamHandler(s.Network, ns)
					if err != nil {
						glog.Errorf("Got error handling stream: %v", err)
						return
					}
				}
			}()

			//Send SubReq
			s.Network.NetworkNode.SendMessage(ns, p, SubReqID, SubReqMsg{StrmID: s.StrmID})
			ctxW, cancel := context.WithCancel(context.Background())
			s.cancelWorker = cancel
			s.working = true
			s.networkStream = ns

			s.startWorker(ctxW, p, ns, gotData)
			return nil
		}
	}

	glog.Errorf("Cannot find any close peers")
	return ErrNoClosePeers

	//Call gotData for every new piece of data
}

func (s *BasicSubscriber) startWorker(ctxW context.Context, p peer.ID, ws *BasicStream, gotData func(seqNo uint64, data []byte, eof bool)) {
	//We expect DataStreamMsg to come back
	go func() {
		for {
			//Get message from the broadcaster
			//Call gotData(seqNo, data)
			//Question: What happens if the handler gets stuck?
			select {
			case msg := <-s.msgChan:
				// glog.Infof("Got data from msgChan: %v", msg)
				gotData(msg.SeqNo, msg.Data, false)
			case <-ctxW.Done():
				s.networkStream = nil
				s.working = false
				glog.Infof("Done with subscription, sending CancelSubMsg")
				//Send EOF
				gotData(0, nil, true)
				err := s.Network.NetworkNode.SendMessage(ws, p, CancelSubID, CancelSubMsg{StrmID: s.StrmID})
				if err != nil {
					glog.Errorf("Error sending CancelSubMsg during worker cancellation: %v", err)
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
	return nil
}
