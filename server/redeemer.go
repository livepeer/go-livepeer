package server

import (
	"context"
	"fmt"
	"io"
	"math/big"
	gonet "net"
	"net/url"
	"sync"
	"time"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/event"
	"github.com/golang/glog"
	"github.com/golang/protobuf/ptypes/empty"
	"github.com/livepeer/go-livepeer/eth"
	"github.com/livepeer/go-livepeer/net"
	"github.com/livepeer/go-livepeer/pm"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

const rpcTimeout = 8 * time.Second

var cleanupLoopTime = 1 * time.Hour

type redeemer struct {
	recipient   ethcommon.Address
	subs        sync.Map
	eth         eth.LivepeerEthClient
	sm          pm.SenderMonitor
	quit        chan struct{}
	liveSenders sync.Map // ethCommon.Address => time.Time lastAccess
}

// NewRedeemer creates a new ticket redemption service instance
func NewRedeemer(recipient ethcommon.Address, eth eth.LivepeerEthClient, sm pm.SenderMonitor) (net.TicketRedeemerServer, error) {

	if recipient == (ethcommon.Address{}) {
		return nil, fmt.Errorf("must provide a recipient")
	}

	if eth == nil {
		return nil, fmt.Errorf("must provide a LivepeerEthClient")
	}

	if sm == nil {
		return nil, fmt.Errorf("must provide a SenderMonitor")
	}

	return &redeemer{
		recipient: recipient,
		eth:       eth,
		sm:        sm,
		quit:      make(chan struct{}),
	}, nil
}

func (r *redeemer) Start() error {
	listener, err := gonet.Listen("tcp", ":50051")
	defer listener.Close()
	if err != nil {
		return err
	}
	// slice of gRPC options
	// Here we can configure things like TLS
	opts := []grpc.ServerOption{}
	// var s *grpc.Server
	s := grpc.NewServer(opts...)
	defer s.Stop()

	net.RegisterTicketRedeemerServer(s, r)

	go r.startCleanupLoop()

	return s.Serve(listener)
}

func (r *redeemer) Stop() {
	close(r.quit)
}

func (r *redeemer) QueueTicket(ctx context.Context, ticket *net.Ticket) (*empty.Empty, error) {
	t := pmTicket(ticket)
	if err := r.sm.QueueTicket(t); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	go r.monitorMaxFloat(ethcommon.BytesToAddress(ticket.Sender))
	return &empty.Empty{}, nil
}

func (r *redeemer) monitorMaxFloat(sender ethcommon.Address) {
	_, ok := r.liveSenders.Load(sender)
	if ok {
		// update last access
		r.liveSenders.Store(sender, time.Now())
		return
	}
	r.liveSenders.Store(sender, time.Now())
	sink := make(chan *big.Int, 10)
	sub := r.sm.MonitorMaxFloat(sender, sink)
	defer sub.Unsubscribe()
	for {
		select {
		case <-r.quit:
			return
		case err := <-sub.Err():
			glog.Error(err)
		case mf := <-sink:
			r.sendMaxFloatUpdate(sender, mf)
		}
	}
}

func (r *redeemer) sendMaxFloatUpdate(sender ethcommon.Address, maxFloat *big.Int) {
	r.subs.Range(
		func(key, value interface{}) bool {
			var maxFloatB []byte
			if maxFloat != nil {
				maxFloatB = maxFloat.Bytes()
			}
			value.(chan *net.MaxFloatUpdate) <- &net.MaxFloatUpdate{
				Sender:   sender.Bytes(),
				MaxFloat: maxFloatB,
			}
			return true
		},
	)
}

func (r *redeemer) MonitorMaxFloat(req *empty.Empty, stream net.TicketRedeemer_MonitorMaxFloatServer) error {
	// The client address will serve as the ID for the stream
	p, ok := peer.FromContext(stream.Context())
	if !ok {
		return status.Error(codes.Internal, "context is nil")
	}

	// Make a channel to receive max float updates
	//  This check allows to overwrite the channel for testing purposes
	var maxFloatUpdates chan *net.MaxFloatUpdate
	maxFloatUpdatesI, ok := r.subs.Load(p.Addr.String())
	if !ok {
		maxFloatUpdates = make(chan *net.MaxFloatUpdate)
		r.subs.Store(p.Addr.String(), maxFloatUpdates)
		glog.Infof("new MonitorMaxFloat subscriber: %v", p.Addr.String())
	} else {
		maxFloatUpdates, ok = maxFloatUpdatesI.(chan *net.MaxFloatUpdate)
		if !ok {
			return status.Error(codes.Internal, "maxFloatUpdates is of the wrong type")
		}
	}

	// Block so that the stream is over a long-lived connection
	for {
		select {
		case maxFloatUpdate := <-maxFloatUpdates:
			if err := stream.Send(maxFloatUpdate); err != nil {
				if err == io.EOF {
					r.subs.Delete(p.Addr.String())
					return status.Error(codes.Internal, err.Error())
				}
				glog.Errorf("Unable to send maxFloat update to client=%v err=%v", p.Addr.String(), err)
			}
		case <-r.quit:
			return nil
		case <-stream.Context().Done():
			return nil
		}
	}
}

func (r *redeemer) startCleanupLoop() {
	ticker := time.NewTicker(cleanupLoopTime)
	for {
		select {
		case <-ticker.C:
			// clean up map entries that haven't been cleared since the last cleanup loop ran
			r.liveSenders.Range(func(key, value interface{}) bool {
				if value.(time.Time).Add(cleanupLoopTime).Before(time.Now()) {
					r.liveSenders.Delete(key)
				}
				return true
			})
		case <-r.quit:
			return
		}
	}
}

func pmTicket(ticket *net.Ticket) *pm.SignedTicket {
	return &pm.SignedTicket{
		Ticket: &pm.Ticket{
			Recipient:              ethcommon.BytesToAddress(ticket.Recipient),
			Sender:                 ethcommon.BytesToAddress(ticket.Sender),
			FaceValue:              new(big.Int).SetBytes(ticket.FaceValue),
			WinProb:                new(big.Int).SetBytes(ticket.WinProb),
			SenderNonce:            ticket.SenderNonce,
			RecipientRandHash:      ethcommon.BytesToHash(ticket.RecipientRandHash),
			CreationRound:          ticket.CreationRound,
			CreationRoundBlockHash: ethcommon.BytesToHash(ticket.CreationRoundBlockHash),
			ParamsExpirationBlock:  new(big.Int).SetInt64(ticket.ParamsExpirationBlock),
		},
		RecipientRand: new(big.Int).SetBytes(ticket.RecipientRand),
		Sig:           ticket.Sig,
	}
}

func protoTicket(ticket *pm.SignedTicket) *net.Ticket {
	return &net.Ticket{
		Recipient:              ticket.Recipient.Bytes(),
		Sender:                 ticket.Sender.Bytes(),
		FaceValue:              ticket.FaceValue.Bytes(),
		WinProb:                ticket.WinProb.Bytes(),
		SenderNonce:            ticket.SenderNonce,
		CreationRound:          ticket.CreationRound,
		CreationRoundBlockHash: ticket.CreationRoundBlockHash.Bytes(),
		Sig:                    ticket.Sig,
		RecipientRand:          ticket.Recipient.Bytes(),
		ParamsExpirationBlock:  ticket.ParamsExpirationBlock.Int64(),
	}
}
