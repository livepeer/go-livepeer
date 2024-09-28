package pm

import (
	"context"
	"crypto/tls"
	"fmt"
	"math/big"
	"sync"
	"time"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/monitor"
	"github.com/livepeer/go-livepeer/net"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// The minimum value for the ratio between a sender's deposit and its pending amount required for the
// pending amount to be ignored when calculating the sender's max float
const minDepositPendingRatio = 3.0

// unixNow returns the current unix time
// This is a wrapper function that can be stubbed in tests
var unixNow = func() int64 {
	return time.Now().Unix()
}

// SenderMonitor is an interface that describes methods used to
// monitor remote senders
type SenderMonitor interface {
	// Start initiates the helper goroutines for the monitor
	Start()

	// Stop signals the monitor to exit gracefully
	Stop()

	// CheckForRedeemableTickets checks for tickets that need to be redeemed
	CheckForRedeemableTickets() ([]SignedTicket, error)

	// QueueTicket adds a ticket to the queue for a remote sender
	QueueTicket(ticket *SignedTicket) error

	// MaxFloat returns a remote sender's max float
	MaxFloat(addr ethcommon.Address) (*big.Int, error)

	// ValidateSender checks whether a sender's unlock period ends the round after the next round
	ValidateSender(addr ethcommon.Address) error
}

type remoteSender struct {
	// pendingAmount is the sum of the face values of tickets that are
	// currently pending redemption on-chain
	pendingAmount *big.Int

	queue *ticketQueue

	// Max float subscriptions
	subFeed  event.Feed
	subScope event.SubscriptionScope

	done chan struct{}

	lastAccess int64
}

type LocalSenderMonitorConfig struct {
	// The address that will be claiming from senders' reserves
	Claimant ethcommon.Address
	// The interval at which to cleanup inactive remote senders from the cache
	CleanupInterval time.Duration
	// The interval at which to run the cleanup process
	TTL int

	// Gas cost estimate for ticket redemption
	RedeemGas       int
	SuggestGasPrice func(context.Context) (*big.Int, error)
	RPCTimeout      time.Duration
}

type LocalSenderMonitor struct {
	cfg *LocalSenderMonitorConfig

	mu      sync.Mutex
	senders map[ethcommon.Address]*remoteSender

	broker Broker
	smgr   SenderManager
	tm     TimeManager

	// redeemable is a channel that an external caller can use to
	// receive tickets that are fed from the ticket queues for
	// each of currently active remote senders
	redeemable chan *redemption

	ticketStore TicketStore

	redeemeruri string
	useredeemer bool
	conn        *grpc.ClientConn
	rpc         net.TicketRedeemerClient

	quit chan struct{}
}

// NewSenderMonitor returns a new SenderMonitor
func NewSenderMonitor(cfg *LocalSenderMonitorConfig, broker Broker, smgr SenderManager, tm TimeManager, store TicketStore, redeemeruri string, useredeemer bool) *LocalSenderMonitor {
	return &LocalSenderMonitor{
		cfg:         cfg,
		broker:      broker,
		smgr:        smgr,
		tm:          tm,
		senders:     make(map[ethcommon.Address]*remoteSender),
		redeemable:  make(chan *redemption),
		ticketStore: store,
		redeemeruri: redeemeruri,
		useredeemer: useredeemer,
		conn:        nil,
		rpc:         nil,
		quit:        make(chan struct{}),
	}
}

// Start initiates the helper goroutines for the monitor
func (sm *LocalSenderMonitor) Start() {
	go sm.startCleanupLoop()
	go sm.watchReserveChange()
	go sm.watchPoolSizeChange()

	if sm.useredeemer {
		conn, err := grpc.Dial(
			sm.redeemeruri,
			grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{InsecureSkipVerify: true})),
			grpc.WithBlock(),
			grpc.WithTimeout(3*time.Second),
		)

		// TODO: PROVIDE KEEPALIVE SETTINGS
		if err != nil {
			glog.Errorf("Did not connect to redeemer=%v err=%q", sm.redeemeruri, err)
		}

		rpc := net.NewTicketRedeemerClient(conn)
		sm.conn = conn
		sm.rpc = rpc
		glog.Infof("Redeemer client started for %v", sm.redeemeruri)
	}
}

// Stop signals the monitor to exit gracefully
func (sm *LocalSenderMonitor) Stop() {
	sm.conn.Close()
	close(sm.quit)
}

func (sm *LocalSenderMonitor) CheckForRedeemableTickets() ([]SignedTicket, error) {
	minRound := new(big.Int).Sub(sm.tm.LastInitializedRound(), big.NewInt(ticketValidityPeriod)).Int64()
	glog.Infof("checking for tickets created after %d that need to be redeemed", minRound)
	ticketsCnt, err := sm.ticketStore.WinningTicketsToRedeem(minRound)
	if err == nil && ticketsCnt > 0 {
		return sm.ticketStore.SelectEligibleWinningTickets(new(big.Int).Sub(sm.tm.LastInitializedRound(), big.NewInt(ticketValidityPeriod)).Int64())
	} else {
		glog.Infof("No tickets found to redeem", minRound)
		return []SignedTicket{}, nil
	}
}

// addFloat adds to a remote sender's max float
func (sm *LocalSenderMonitor) addFloat(addr ethcommon.Address, amount *big.Int) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.ensureCache(addr)

	// Subtracting from pendingAmount = adding to max float
	pendingAmount := sm.senders[addr].pendingAmount
	if pendingAmount.Cmp(amount) < 0 {
		return errors.New("cannot subtract from insufficient pendingAmount")
	}

	sm.senders[addr].pendingAmount.Sub(pendingAmount, amount)
	sm.sendMaxFloatChange(addr)
	return nil
}

// subFloat subtracts from a remote sender's max float
func (sm *LocalSenderMonitor) subFloat(addr ethcommon.Address, amount *big.Int) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.ensureCache(addr)

	// Adding to pendingAmount = subtracting from max float
	pendingAmount := sm.senders[addr].pendingAmount
	sm.senders[addr].pendingAmount.Add(pendingAmount, amount)
	sm.sendMaxFloatChange(addr)
}

// MaxFloat returns a remote sender's max float
func (sm *LocalSenderMonitor) MaxFloat(addr ethcommon.Address) (*big.Int, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.ensureCache(addr)

	return sm.maxFloat(addr)
}

// QueueTicket adds a ticket to the queue for a remote sender
func (sm *LocalSenderMonitor) QueueTicket(ticket *SignedTicket) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.ensureCache(ticket.Sender)

	return sm.senders[ticket.Sender].queue.Add(ticket)
}

// ValidateSender checks whether a sender's unlock period ends the round after the next round
func (sm *LocalSenderMonitor) ValidateSender(addr ethcommon.Address) error {
	info, err := sm.smgr.GetSenderInfo(addr)
	if err != nil {
		return fmt.Errorf("could not get sender info for %v: %v", addr.Hex(), err)
	}
	maxWithdrawRound := new(big.Int).Add(sm.tm.LastInitializedRound(), big.NewInt(1))
	if info.WithdrawRound.Int64() != 0 && info.WithdrawRound.Cmp(maxWithdrawRound) != 1 {
		return fmt.Errorf("deposit and reserve for sender %v is set to unlock soon", addr.Hex())
	}
	return nil
}

// maxFloat is a helper that returns the sender's max float
// If the sender's deposit pending ratio (deposit / pendingAmount) >= minDepositPendingRatio, then the max float is reserveAlloc
// Otherwise, the max float is reserveAlloc - pendingAmount
// Caller should hold the lock for LocalSenderMonitor
func (sm *LocalSenderMonitor) maxFloat(addr ethcommon.Address) (*big.Int, error) {
	reserveAlloc, err := sm.reserveAlloc(addr)
	if err != nil {
		return nil, err
	}

	info, err := sm.smgr.GetSenderInfo(addr)
	if err != nil {
		return nil, err
	}

	pendingAmount := sm.senders[addr].pendingAmount
	depositPendingRatio := big.NewFloat(0.0)
	if pendingAmount.Cmp(big.NewInt(0)) > 0 {
		depositPendingRatio.Quo(new(big.Float).SetInt(info.Deposit), new(big.Float).SetInt(pendingAmount))
	}
	// If the actual deposit pending ratio exceeds the minimum deposit pending ratio we ignore the sender's pendingAmount
	// and return the sender's reserve allocation as the max float
	if depositPendingRatio.Cmp(big.NewFloat(minDepositPendingRatio)) >= 0 {
		return reserveAlloc, nil
	}

	return new(big.Int).Sub(reserveAlloc, pendingAmount), nil
}

func (sm *LocalSenderMonitor) reserveAlloc(addr ethcommon.Address) (*big.Int, error) {
	info, err := sm.smgr.GetSenderInfo(addr)
	if err != nil {
		return nil, err
	}
	claimed, err := sm.smgr.ClaimedReserve(addr, sm.cfg.Claimant)
	if err != nil {
		return nil, err
	}
	poolSize := sm.tm.GetTranscoderPoolSize()
	if poolSize.Cmp(big.NewInt(0)) == 0 {
		return big.NewInt(0), nil
	}
	reserve := new(big.Int).Add(info.Reserve.FundsRemaining, info.Reserve.ClaimedInCurrentRound)
	return new(big.Int).Sub(new(big.Int).Div(reserve, poolSize), claimed), nil
}

// Returns the current available funds for a sender that could cover redemptions
func (sm *LocalSenderMonitor) availableFunds(addr ethcommon.Address) (*big.Int, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.ensureCache(addr)

	info, err := sm.smgr.GetSenderInfo(addr)
	if err != nil {
		return nil, err
	}

	reserveAlloc, err := sm.reserveAlloc(addr)
	if err != nil {
		return nil, err
	}

	pendingAmount := sm.senders[addr].pendingAmount

	return new(big.Int).Sub(new(big.Int).Add(reserveAlloc, info.Deposit), pendingAmount), nil
}

// ensureCache is a helper that checks if a remote sender is initialized
// and if not will fetch and cache the remote sender's reserve alloc
// Caller should hold the lock for LocalSenderMonitor
func (sm *LocalSenderMonitor) ensureCache(addr ethcommon.Address) {
	if sm.senders[addr] == nil {
		sm.cache(addr)
	}

	sm.senders[addr].lastAccess = unixNow()
}

// cache is a helper that caches a remote sender's reserve alloc and
// starts a ticket queue for the remote sender
// Caller should hold the lock for LocalSenderMonitor unless the caller is
// ensureCache() in which case the caller of ensureCache() should hold the lock
func (sm *LocalSenderMonitor) cache(addr ethcommon.Address) {
	queue := newTicketQueue(addr, sm)
	queue.Start()
	done := make(chan struct{})
	go sm.startTicketQueueConsumerLoop(queue, done)
	sm.senders[addr] = &remoteSender{
		pendingAmount: big.NewInt(0),
		queue:         queue,
		done:          done,
		lastAccess:    unixNow(),
	}
}

// startTicketQueueConsumerLoop initiates a loop that runs a consumer
// that receives redeemable tickets from a ticketQueue and feeds them into
// a single output channel in a fan-in manner
func (sm *LocalSenderMonitor) startTicketQueueConsumerLoop(queue *ticketQueue, done chan struct{}) {
	for {
		select {
		case red := <-queue.Redeemable():
			if sm.useredeemer == false {
				tx, err := sm.redeemWinningTicket(red.SignedTicket)
				res := struct {
					txHash ethcommon.Hash
					err    error
				}{
					ethcommon.Hash{},
					err,
				}
				// FIXME: If there are replacement txs then tx.Hash() could be different
				// from the hash of the replacement tx that was mined
				if tx != nil {
					res.txHash = tx.Hash()
				}

				red.resCh <- res
			} else {
				glog.Info("Sending ticket to redeemer")
				err := sm.sendTicketToRedeemer(red.SignedTicket)
				res := struct {
					txHash ethcommon.Hash
					err    error
				}{
					ethcommon.Hash{},
					err,
				}
				red.resCh <- res
			}
		case <-done:
			// When the ticket consumer exits, tell the ticketQueue
			// to exit as well
			queue.Stop()

			return
		case <-sm.quit:
			// When the monitor exits, tell the ticketQueue
			// to exit as well
			queue.Stop()

			return
		}
	}
}

// startCleanupLoop initiates a loop that runs a cleanup worker
// every cleanupInterval
func (sm *LocalSenderMonitor) startCleanupLoop() {
	ticker := time.NewTicker(sm.cfg.CleanupInterval)

	for {
		select {
		case <-ticker.C:
			sm.cleanup()
		case <-sm.quit:
			return
		}
	}
}

// cleanup removes tracked remote senders that have exceeded
// their ttl
func (sm *LocalSenderMonitor) cleanup() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	for k, v := range sm.senders {
		if unixNow()-v.lastAccess > int64(sm.cfg.TTL) && v.subScope.Count() == 0 {
			// Signal the ticket queue consumer to exit gracefully
			v.done <- struct{}{}
			v.subScope.Close() // close the maxfloat subscriptions
			delete(sm.senders, k)
			sm.smgr.Clear(k)
			glog.V(6).Infof("sender cleared from cache addr: %v last access: %v ttl: %vs", hexutil.Encode(k.Bytes()), v.lastAccess, sm.cfg.TTL)
		}
	}
}

// Returns a non-nil tx if one is sent. Otherwise, returns a nil tx
func (sm *LocalSenderMonitor) redeemWinningTicket(ticket *SignedTicket) (*types.Transaction, error) {
	availableFunds, err := sm.availableFunds(ticket.Sender)
	if err != nil {
		return nil, err
	}

	// Fail early if ticket is used
	used, err := sm.broker.IsUsedTicket(ticket.Ticket)
	if err != nil {
		if monitor.Enabled {
			monitor.TicketRedemptionError(ticket.Sender.Hex())
		}
		return nil, err
	}
	if used {
		if monitor.Enabled {
			monitor.TicketRedemptionError(ticket.Sender.Hex())
		}
		return nil, errIsUsedTicket
	}

	ctx, cancel := context.WithTimeout(context.Background(), sm.cfg.RPCTimeout)
	gasPrice, err := sm.cfg.SuggestGasPrice(ctx)
	if err != nil {
		cancel()
		return nil, err
	}
	cancel()

	// We only submit a redemption if availableFunds covers the redemption tx cost
	// Otherwise, we return an error so we can try the redemption later
	txCost := new(big.Int).Mul(big.NewInt(int64(sm.cfg.RedeemGas)), gasPrice)
	if availableFunds.Cmp(txCost) <= 0 {
		return nil, errors.New("insufficient sender funds for redeem tx cost")
	}
	if ticket.FaceValue.Cmp(txCost) <= 0 {
		return nil, errors.New("insufficient ticket face value for redeem tx cost")
	}

	// Subtract the ticket face value from the sender's current max float
	// This amount will be considered pending until the ticket redemption
	// transaction confirms on-chain
	sm.subFloat(ticket.Ticket.Sender, ticket.Ticket.FaceValue)

	defer func() {
		// Add the ticket face value back to the sender's current max float
		// This amount is no longer considered pending since the ticket
		// redemption transaction either confirmed on-chain or was not
		// submitted at all
		//
		// TODO(yondonfu): Should ultimately add back only the amount that
		// was actually successfully redeemed in order to take into account
		// the case where the ticket was not redeemed for its full face value
		// because the reserve was insufficient
		if err := sm.addFloat(ticket.Ticket.Sender, ticket.Ticket.FaceValue); err != nil {
			glog.Error(err)
		}
	}()

	// Assume that that this call will return immediately if there
	// is an error in transaction submission
	tx, err := sm.broker.RedeemWinningTicket(ticket.Ticket, ticket.Sig, ticket.RecipientRand)
	if err != nil {
		if monitor.Enabled {
			monitor.TicketRedemptionError(ticket.Sender.Hex())
		}
		return nil, err
	}

	// Wait for transaction to confirm
	if err := sm.broker.CheckTx(tx); err != nil {
		if monitor.Enabled {
			monitor.TicketRedemptionError(ticket.Sender.Hex())
		}
		// Return tx so caller can utilize the tx if it fails
		return tx, err
	}

	if monitor.Enabled {
		// TODO(yondonfu): Handle case where < ticket.FaceValue is actually
		// redeemed i.e. if sender reserve cannot cover the full ticket.FaceValue
		monitor.ValueRedeemed(ticket.Sender.Hex(), ticket.Ticket.FaceValue)
	}

	return tx, nil
}

func (sm *LocalSenderMonitor) sendTicketToRedeemer(ticket *SignedTicket) error {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	_, err := sm.rpc.QueueTicket(ctx, protoTicket(ticket))

	if err == nil {
		if monitor.Enabled {
			monitor.ValueRedeemed(ticket.Sender.Hex(), ticket.Ticket.FaceValue)
		}
	} else {
		if monitor.Enabled {
			monitor.TicketRedemptionError(ticket.Sender.Hex())
		}
	}
	return err
}

// SubscribeMaxFloatChange notifies subcribers when the max float for a sender has changed
// and that it should call LocalSenderMonitor.MaxFloat() to get the latest value
func (sm *LocalSenderMonitor) SubscribeMaxFloatChange(sender ethcommon.Address, sink chan<- struct{}) event.Subscription {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.ensureCache(sender)

	rs := sm.senders[sender]

	return rs.subScope.Track(rs.subFeed.Subscribe(sink))
}

// The caller of this function should hold the lock for sm.senders
func (sm *LocalSenderMonitor) sendMaxFloatChange(sender ethcommon.Address) {
	sm.senders[sender].subFeed.Send(struct{}{})
}

func (sm *LocalSenderMonitor) watchReserveChange() {
	sink := make(chan ethcommon.Address, 10)
	sub := sm.smgr.SubscribeReserveChange(sink)
	defer sub.Unsubscribe()

	for {
		select {
		case <-sm.quit:
			return
		case err := <-sub.Err():
			glog.Error(err)
			continue
		case sender := <-sink:
			go func() {
				sm.mu.Lock()
				sm.sendMaxFloatChange(sender)
				sm.mu.Unlock()
			}()
		}
	}
}

func (sm *LocalSenderMonitor) watchPoolSizeChange() {
	sink := make(chan types.Log, 10)
	sub := sm.tm.SubscribeRounds(sink)
	defer sub.Unsubscribe()

	lastPoolSize := sm.tm.GetTranscoderPoolSize()
	for {
		select {
		case <-sm.quit:
			return
		case err := <-sub.Err():
			glog.Error(err)
			continue
		case <-sink:
			poolSize := sm.tm.GetTranscoderPoolSize()
			if poolSize.Cmp(lastPoolSize) != 0 {
				sm.handlePoolSizeChange()
				lastPoolSize = poolSize
			}
		}
	}
}

func (sm *LocalSenderMonitor) handlePoolSizeChange() {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	for sender := range sm.senders {
		sm.sendMaxFloatChange(sender)
	}
}

func protoTicket(ticket *SignedTicket) *net.Ticket {
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
