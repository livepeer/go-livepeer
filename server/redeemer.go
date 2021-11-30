package server

import (
	"context"
	"fmt"
	"math/big"
	gonet "net"
	"net/url"
	"sync"
	"time"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/event"
	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/eth"
	"github.com/livepeer/go-livepeer/net"
	"github.com/livepeer/go-livepeer/pm"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"
)

var cleanupLoopTime = 5 * time.Minute

type localSenderMonitor interface {
	pm.SenderMonitor
	SubscribeMaxFloatChange(sender ethcommon.Address, sink chan<- struct{}) event.Subscription
}

type Redeemer struct {
	server    *grpc.Server
	recipient ethcommon.Address
	eth       eth.LivepeerEthClient
	sm        localSenderMonitor
	quit      chan struct{}
}

// NewRedeemer creates a new ticket redemption service instance
func NewRedeemer(recipient ethcommon.Address, eth eth.LivepeerEthClient, sm *pm.LocalSenderMonitor) (*Redeemer, error) {

	if recipient == (ethcommon.Address{}) {
		return nil, fmt.Errorf("must provide a recipient")
	}

	if eth == nil {
		return nil, fmt.Errorf("must provide a LivepeerEthClient")
	}

	if sm == nil {
		return nil, fmt.Errorf("must provide a SenderMonitor")
	}

	return &Redeemer{
		recipient: recipient,
		eth:       eth,
		sm:        sm,
		quit:      make(chan struct{}),
	}, nil
}

// Start starts a Redeemer server
// This method will block
func (r *Redeemer) Start(url *url.URL, workDir string) error {
	listener, err := gonet.Listen("tcp", url.Host)
	if err != nil {
		return err
	}
	defer listener.Close()

	certFile, keyFile, err := getCert(url, workDir)
	if err != nil {
		return err
	}

	creds, err := credentials.NewServerTLSFromFile(certFile, keyFile)
	if err != nil {
		return err
	}

	s := grpc.NewServer(grpc.Creds(creds))
	r.server = s

	net.RegisterTicketRedeemerServer(s, r)

	return s.Serve(listener)
}

// Stop stops the Redeemer server
func (r *Redeemer) Stop() {
	close(r.quit)
	r.server.Stop()
}

// QueueTicket adds a ticket to the ticket queue
func (r *Redeemer) QueueTicket(ctx context.Context, ticket *net.Ticket) (*net.QueueTicketRes, error) {
	t := pmTicket(ticket)
	if r.recipient != t.Recipient {
		return nil, status.Error(codes.PermissionDenied, fmt.Sprintf("invalid ticket recipient 0x%x, expected %v", ticket.TicketParams.Recipient, r.recipient.Hex()))
	}
	if err := r.sm.QueueTicket(t); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	glog.Infof("Ticket queued sender=0x%x", ticket.Sender)

	return &net.QueueTicketRes{}, nil
}

// MaxFloat is a unary RPC method to request the max float value for a sender
func (r *Redeemer) MaxFloat(ctx context.Context, req *net.MaxFloatReq) (*net.MaxFloatUpdate, error) {
	maxFloat, err := r.sm.MaxFloat(ethcommon.BytesToAddress(req.Sender))
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &net.MaxFloatUpdate{MaxFloat: maxFloat.Bytes()}, nil
}

// MonitorMaxFloat starts a server-side stream to the client to send max float updates for sender specified in the request
func (r *Redeemer) MonitorMaxFloat(req *net.MaxFloatReq, stream net.TicketRedeemer_MonitorMaxFloatServer) error {
	sender := ethcommon.BytesToAddress(req.Sender)

	sink := make(chan struct{}, 10)
	sub := r.sm.SubscribeMaxFloatChange(sender, sink)
	defer sub.Unsubscribe()
	for {
		select {
		case <-r.quit:
			return nil
		case <-stream.Context().Done():
			return nil
		case err := <-sub.Err():
			if err == nil {
				return status.Error(codes.Canceled, "subscription closed")
			}
			return status.Error(codes.Internal, err.Error())
		case <-sink:
			maxFloat, err := r.sm.MaxFloat(sender)
			if err != nil {
				glog.Error(err)
				continue
			}
			if err := stream.Send(&net.MaxFloatUpdate{MaxFloat: maxFloat.Bytes()}); err != nil {
				if err != nil {
					return status.Error(codes.Internal, err.Error())
				}
			}
		}
	}
}

type RedeemerClient struct {
	conn *grpc.ClientConn
	rpc  net.TicketRedeemerClient

	senders map[ethcommon.Address]*remoteSender
	mu      sync.RWMutex
	quit    chan struct{}
	sm      pm.SenderManager
	tm      pm.TimeManager
}

type remoteSender struct {
	maxFloat   *big.Int
	stream     net.TicketRedeemer_MonitorMaxFloatClient
	done       chan struct{}
	lastAccess time.Time
}

// NewRedeemerClient instantiates a new client for the ticket redemption service
// The client implements the pm.SenderMonitor interface
func NewRedeemerClient(uri string, sm pm.SenderManager, tm pm.TimeManager) (*RedeemerClient, error) {
	conn, err := grpc.Dial(
		uri,
		grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)),
		grpc.WithBlock(),
		grpc.WithTimeout(GRPCConnectTimeout),
	)

	// TODO: PROVIDE KEEPALIVE SETTINGS
	if err != nil {
		return nil, fmt.Errorf("Did not connect to redeemer=%v err=%q", uri, err)
	}
	return &RedeemerClient{
		conn:    conn,
		rpc:     net.NewTicketRedeemerClient(conn),
		sm:      sm,
		tm:      tm,
		senders: make(map[ethcommon.Address]*remoteSender),
		quit:    make(chan struct{}),
	}, nil
}

func (r *RedeemerClient) Start() {
	go r.startCleanupLoop()
}

// Stop stops the Redeemer client
func (r *RedeemerClient) Stop() {
	close(r.quit)
	r.conn.Close()
}

// QueueTicket sends a winning ticket to the Redeemer
func (r *RedeemerClient) QueueTicket(ticket *pm.SignedTicket) error {
	ctx, cancel := context.WithTimeout(context.Background(), GRPCTimeout)
	defer cancel()
	_, err := r.rpc.QueueTicket(ctx, protoTicket(ticket))
	return err
}

// MaxFloat returns the max float for 'sender'
// If no local cache is available this method will remotely request
// max float from the Redeemer server and start watching for subsequent updates from the Redeemer server
func (r *RedeemerClient) MaxFloat(sender ethcommon.Address) (*big.Int, error) {
	// Ensure thread-safe access to 'RedeemerClient.senders'
	r.mu.Lock()

	// Check whether local cache exists for 'sender'
	if mf, ok := r.senders[sender]; ok && mf.maxFloat != nil {
		r.senders[sender].lastAccess = time.Now()
		r.mu.Unlock()
		return mf.maxFloat, nil
	}

	// release the lock before executing an RPC call
	r.mu.Unlock()

	// Retrieve max float from Redeemer and cache it if no local cache exists
	ctx, cancel := context.WithTimeout(context.Background(), GRPCTimeout)
	mfu, err := r.rpc.MaxFloat(ctx, &net.MaxFloatReq{Sender: sender.Bytes()})
	cancel()
	if err != nil {
		return nil, err
	}
	mf := new(big.Int).SetBytes(mfu.MaxFloat)

	// Request updates for sender from Redeemer
	ctx, cancel = context.WithCancel(context.Background())
	stream, err := r.rpc.MonitorMaxFloat(ctx, &net.MaxFloatReq{Sender: sender.Bytes()})
	if err != nil {
		cancel()
		// An error means we won't be receiving updates from the Redeemer for 'sender'
		// So don't update the local cache and retry subscribing to updates from 'sender' on the next MaxFloat() call
		// Return the current retrieved max float
		glog.Error(err)
		return mf, nil
	}

	r.mu.Lock()
	r.senders[sender] = &remoteSender{
		maxFloat:   mf,
		stream:     stream,
		done:       make(chan struct{}),
		lastAccess: time.Now(),
	}
	// release the lock before starting a watchdog on the stream for the 'sender'
	r.mu.Unlock()

	go r.monitorMaxFloat(cancel, sender)

	return mf, nil
}

// ValidateSender checks whether a sender has not recently unlocked its deposit and reserve
func (r *RedeemerClient) ValidateSender(sender ethcommon.Address) error {
	info, err := r.sm.GetSenderInfo(sender)
	if err != nil {
		return fmt.Errorf("could not get sender info for %v: %v", sender.Hex(), err)
	}
	maxWithdrawRound := new(big.Int).Add(r.tm.LastInitializedRound(), big.NewInt(1))
	if info.WithdrawRound.Int64() != 0 && info.WithdrawRound.Cmp(maxWithdrawRound) != 1 {
		return fmt.Errorf("deposit and reserve for sender %v is set to unlock soon", sender.Hex())
	}
	return nil
}

func (r *RedeemerClient) monitorMaxFloat(cancel context.CancelFunc, sender ethcommon.Address) {
	defer cancel()
	r.mu.Lock()
	s, ok := r.senders[sender]
	r.mu.Unlock()
	// sanity check
	if !ok {
		return
	}

	updateC := make(chan *net.MaxFloatUpdate)
	errC := make(chan error)
	go func() {
		for {
			update, err := s.stream.Recv()
			if err != nil {
				errC <- err
				return
			} else {
				updateC <- update
			}
		}
	}()

	for {
		select {
		case <-r.quit:
			return
		case <-s.done:
			glog.Infof("Closing stream for sender=%v", sender.Hex())
			return
		case update := <-updateC:
			r.mu.Lock()
			r.senders[sender].maxFloat = new(big.Int).SetBytes(update.MaxFloat)
			r.senders[sender].lastAccess = time.Now()
			r.mu.Unlock()
		case err := <-errC:
			glog.Error(err)
			r.sm.Clear(sender)
			r.mu.Lock()
			delete(r.senders, sender)
			r.mu.Unlock()
			return
		}
	}
}

func (r *RedeemerClient) startCleanupLoop() {
	ticker := time.NewTicker(cleanupLoopTime)
	for {
		select {
		case <-ticker.C:
			// clean up map entries that haven't been cleared since the last cleanup loop ran
			r.mu.Lock()
			for sender, mf := range r.senders {
				if mf.lastAccess.Add(cleanupLoopTime).Before(time.Now()) {
					r.sm.Clear(sender)
					delete(r.senders, sender)
					close(mf.done)
				}
			}
			r.mu.Unlock()
		case <-r.quit:
			return
		}
	}
}

func pmTicket(ticket *net.Ticket) *pm.SignedTicket {
	return &pm.SignedTicket{
		Ticket: &pm.Ticket{
			Recipient:              ethcommon.BytesToAddress(ticket.TicketParams.Recipient),
			Sender:                 ethcommon.BytesToAddress(ticket.Sender),
			FaceValue:              new(big.Int).SetBytes(ticket.TicketParams.FaceValue),
			WinProb:                new(big.Int).SetBytes(ticket.TicketParams.WinProb),
			SenderNonce:            ticket.SenderParams.SenderNonce,
			RecipientRandHash:      ethcommon.BytesToHash(ticket.TicketParams.RecipientRandHash),
			CreationRound:          ticket.ExpirationParams.CreationRound,
			CreationRoundBlockHash: ethcommon.BytesToHash(ticket.ExpirationParams.CreationRoundBlockHash),
			ParamsExpirationBlock:  new(big.Int).SetBytes(ticket.TicketParams.ExpirationBlock),
		},
		RecipientRand: new(big.Int).SetBytes(ticket.RecipientRand),
		Sig:           ticket.SenderParams.Sig,
	}
}

func protoTicket(ticket *pm.SignedTicket) *net.Ticket {
	return &net.Ticket{
		Sender:        ticket.Sender.Bytes(),
		RecipientRand: ticket.RecipientRand.Bytes(),
		TicketParams: &net.TicketParams{
			Recipient:         ticket.Recipient.Bytes(),
			FaceValue:         ticket.FaceValue.Bytes(),
			WinProb:           ticket.WinProb.Bytes(),
			RecipientRandHash: ticket.RecipientRandHash.Bytes(),
			ExpirationBlock:   ticket.ParamsExpirationBlock.Bytes(),
		},
		SenderParams: &net.TicketSenderParams{
			SenderNonce: ticket.SenderNonce,
			Sig:         ticket.Sig,
		},
		ExpirationParams: &net.TicketExpirationParams{
			CreationRound:          ticket.CreationRound,
			CreationRoundBlockHash: ticket.CreationRoundBlockHash.Bytes(),
		},
	}
}
