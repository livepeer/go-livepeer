package pm

import (
	"math/big"
	"sync"
	"time"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"
)

// unixNow returns the current unix time
// This is a wrapper function that can be stubbed in tests
var unixNow = func() int64 {
	return time.Now().Unix()
}

// SenderMonitor is an interface that describes methods used to
// monitor remote senders
type SenderMonitor interface {
	RedeemableEmitter

	// Start initiates the helper goroutines for the monitor
	Start()

	// Stop signals the monitor to exit gracefully
	Stop()

	// QueueTicket adds a ticket to the queue for a remote sender
	QueueTicket(addr ethcommon.Address, ticket *SignedTicket) error

	// AddFloat adds to a remote sender's max float
	AddFloat(addr ethcommon.Address, amount *big.Int) error

	// SubFloat subtracts from a remote sender's max float
	SubFloat(addr ethcommon.Address, amount *big.Int) error

	// MaxFloat returns a remote sender's max float
	MaxFloat(addr ethcommon.Address) (*big.Int, error)

	// AcceptErr checks whether additional errors should be accepted and if so increments the acceptable error count
	// for a sender
	AcceptErr(addr ethcommon.Address) bool
}

type remoteSender struct {
	// reserveAlloc is the amount of the sender's reserve guaranteed to
	// the recipient
	reserveAlloc *big.Int
	// pendingAmount is the sum of the face values of tickets that are
	// currently pending redemption on-chain
	pendingAmount *big.Int

	queue *ticketQueue

	// errCount is the current number of acceptable errors counted
	// for the sender
	errCount int

	done chan struct{}

	lastAccess int64
}

type senderMonitor struct {
	claimant        ethcommon.Address
	cleanupInterval time.Duration
	ttl             int
	// maxErrCount is the maximum number of acceptable errors allowed
	// for a sender
	maxErrCount int

	mu      sync.RWMutex
	senders map[ethcommon.Address]*remoteSender

	broker Broker

	// redeemable is a channel that an external caller can use to
	// receive tickets that are fed from the ticket queues for
	// each of currently active remote senders
	redeemable chan *SignedTicket

	quit chan struct{}
}

// NewSenderMonitor returns a new SenderMonitor
func NewSenderMonitor(claimant ethcommon.Address, broker Broker, cleanupInterval time.Duration, ttl int, maxErrCount int) SenderMonitor {
	return &senderMonitor{
		claimant:        claimant,
		cleanupInterval: cleanupInterval,
		ttl:             ttl,
		maxErrCount:     maxErrCount,
		broker:          broker,
		senders:         make(map[ethcommon.Address]*remoteSender),
		redeemable:      make(chan *SignedTicket),
		quit:            make(chan struct{}),
	}
}

// Start initiates the helper goroutines for the monitor
func (sm *senderMonitor) Start() {
	go sm.startCleanupLoop()
}

// Stop signals the monitor to exit gracefully
func (sm *senderMonitor) Stop() {
	close(sm.quit)
}

// Redeemable returns a channel that a consumer can use to receive tickets that
// should be redeemed
func (sm *senderMonitor) Redeemable() chan *SignedTicket {
	return sm.redeemable
}

// AddFloat adds to a remote sender's max float
func (sm *senderMonitor) AddFloat(addr ethcommon.Address, amount *big.Int) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if err := sm.ensureCache(addr); err != nil {
		return err
	}

	// Subtracting from pendingAmount = adding to max float
	pendingAmount := sm.senders[addr].pendingAmount
	if pendingAmount.Cmp(amount) < 0 {
		return errors.New("cannot subtract from insufficient pendingAmount")
	}

	sm.senders[addr].pendingAmount.Sub(pendingAmount, amount)

	// Whenever a sender's max float increases, signal the updated max float to the
	// sender's associated ticket queue in case there are queued tickets that
	// can be redeemed
	sm.senders[addr].queue.SignalMaxFloat(sm.maxFloat(addr))

	return nil
}

// SubFloat subtracts from a remote sender's max float
func (sm *senderMonitor) SubFloat(addr ethcommon.Address, amount *big.Int) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if err := sm.ensureCache(addr); err != nil {
		return err
	}

	// Adding to pendingAmount = subtracting from max float
	pendingAmount := sm.senders[addr].pendingAmount
	sm.senders[addr].pendingAmount.Add(pendingAmount, amount)

	return nil
}

// MaxFloat returns a remote sender's max float
func (sm *senderMonitor) MaxFloat(addr ethcommon.Address) (*big.Int, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if err := sm.ensureCache(addr); err != nil {
		return nil, err
	}

	return sm.maxFloat(addr), nil
}

// QueueTicket adds a ticket to the queue for a remote sender
func (sm *senderMonitor) QueueTicket(addr ethcommon.Address, ticket *SignedTicket) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if err := sm.ensureCache(addr); err != nil {
		return err
	}

	sm.senders[addr].queue.Add(ticket)

	return nil
}

// AcceptErr checks whether additional errors should be accepted and if so increments the acceptable error count
// for a sender
func (sm *senderMonitor) AcceptErr(addr ethcommon.Address) bool {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.senders[addr].errCount >= sm.maxErrCount {
		return false
	}

	sm.senders[addr].errCount++

	return true
}

// maxFloat is a helper that returns the sender's max float as:
// reserveAlloc - pendingAmount
// Caller should hold the lock for senderMonitor
func (sm *senderMonitor) maxFloat(addr ethcommon.Address) *big.Int {
	return new(big.Int).Sub(sm.senders[addr].reserveAlloc, sm.senders[addr].pendingAmount)
}

// ensureCache is a helper that checks if a remote sender is initialized
// and if not will fetch and cache the remote sender's reserve alloc
// Caller should hold the lock for senderMonitor
func (sm *senderMonitor) ensureCache(addr ethcommon.Address) error {
	if sm.senders[addr] == nil {
		if err := sm.cache(addr); err != nil {
			return err
		}
	}

	sm.senders[addr].lastAccess = unixNow()

	return nil
}

// cache is a helper that caches a remote sender's reserve alloc and
// starts a ticket queue for the remote sender
// Caller should hold the lock for senderMonitor unless the caller is
// ensureCache() in which case the caller of ensureCache() should hold the lock
func (sm *senderMonitor) cache(addr ethcommon.Address) error {
	reserveAlloc, err := sm.broker.ClaimableReserve(addr, sm.claimant)
	if err != nil {
		return err
	}

	queue := newTicketQueue()
	queue.Start()
	done := make(chan struct{})
	go sm.startTicketQueueConsumerLoop(queue, done)

	sm.senders[addr] = &remoteSender{
		reserveAlloc:  reserveAlloc,
		pendingAmount: big.NewInt(0),
		queue:         queue,
		done:          done,
		lastAccess:    unixNow(),
	}

	return nil
}

// startTicketQueueConsumerLoop initiates a loop that runs a consumer
// that receives redeemable tickets from a ticketQueue and feeds them into
// a single output channel in a fan-in manner
func (sm *senderMonitor) startTicketQueueConsumerLoop(queue *ticketQueue, done chan struct{}) {
	for {
		select {
		case ticket := <-queue.Redeemable():
			sm.redeemable <- ticket
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
func (sm *senderMonitor) startCleanupLoop() {
	ticker := time.NewTicker(sm.cleanupInterval)

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
func (sm *senderMonitor) cleanup() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	for k, v := range sm.senders {
		if unixNow()-v.lastAccess > int64(sm.ttl) {
			// Signal the ticket queue consumer to exit gracefully
			v.done <- struct{}{}

			delete(sm.senders, k)
		}
	}
}
