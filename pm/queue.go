package pm

import (
	"math/big"
	"sync"
	"sync/atomic"
)

// ticketQueue is a queue of winning tickets that are in line for redemption on-chain.
// A recipient will have a ticketQueue per sender that it is actively receiving tickets from.
// If a sender's max float is insufficient to cover the face value of a ticket it is added to the queue.
// A ticket is pulled from the queue by the recipient when a sender has sufficient max float to cover
// the next ticket in the queue
//
// Based off of: https://github.com/lightningnetwork/lnd/blob/master/htlcswitch/queue.go
type ticketQueue struct {
	queue []*SignedTicket

	// queueLen is an internal length counter that keeps track
	// of the size of the queue. We maintain this counter instead
	// of reading len(queue) in order to avoid acquiring the main lock
	// used by the queue loop goroutine
	queueLen int32

	// cond is a conditional variable that is used by the main
	// queue loop goroutine to wait for new tickets added to the queue
	cond *sync.Cond

	// maxFloatUpdate is a channel that the queue uses
	// to receive the latest available max float value
	maxFloatUpdate chan *big.Int

	// redeemable is a channel that a recipient will receive
	// redeemable tickets on as a sender's max float becomes
	// sufficient to cover the face value of tickets
	redeemable chan *SignedTicket

	quit chan struct{}
}

func newTicketQueue() *ticketQueue {
	return &ticketQueue{
		cond:           sync.NewCond(&sync.Mutex{}),
		maxFloatUpdate: make(chan *big.Int),
		redeemable:     make(chan *SignedTicket),
		quit:           make(chan struct{}),
	}
}

// Start initiates the main queue loop goroutine for processing tickets
func (q *ticketQueue) Start() {
	go q.startQueueLoop()
}

// Stop signals the ticketQueue to gracefully shutdown
func (q *ticketQueue) Stop() {
	close(q.quit)
}

// Add adds a ticket to the queue
// An external caller should call this method whenever the sender's max float
// is insufficient to cover the ticket face value i.e. if the recipient received
// multiple winning tickets in close succession such that the sender's max float
// cannot cover all of the tickets at once. In this scenario, the recipient should
// submit transactions for tickets that can be covered by the sender's max float, add the
// other tickets to the queue and wait for the transactions to confirm to check if the sender's
// max float is sufficient to cover the tickets in the queue
func (q *ticketQueue) Add(ticket *SignedTicket) {
	// Lock conditional variable while adding to the queue
	q.cond.L.Lock()
	q.queue = append(q.queue, ticket)
	atomic.AddInt32(&q.queueLen, 1)
	q.cond.L.Unlock()

	// Signal that there are tickets in the queue
	q.cond.Signal()
}

// SignalMaxFloat signals to the queue the latest max float for the sender
// An external caller should call this method whenever a ticket redemption confirms
// with the most up-to-date max float for the sender
func (q *ticketQueue) SignalMaxFloat(amount *big.Int) {
	// If the queue is empty do not signal a new max float
	// value because there are no tickets to process
	if q.Length() == 0 {
		return
	}

	select {
	case q.maxFloatUpdate <- amount:
	case <-q.quit:
		return
	}
}

// Redeemable returns a channel that a consumer can use to receive tickets that
// should be redeemed
// pm.SenderMonitor is the primary consumer of this channel
func (q *ticketQueue) Redeemable() chan *SignedTicket {
	return q.redeemable
}

// Length returns the current length of the queue
func (q *ticketQueue) Length() int32 {
	return atomic.LoadInt32(&q.queueLen)
}

// startQueueLoop blocks until the ticket queue is non-empty. When the queue is non-empty
// the loop will block until a value is received on q.maxFloatUpdate which should be the most
// up-to-date max float for the ticket sender associated with the queue. The loop should receive max float
// updates whenever a pending transaction for a ticket redemption confirms (thus tickets can only be popped
// from the queue as redemption transactions confirm). When a max float value is received, the loop checks if it
// is sufficient to cover the face value of the ticket at the head of the queue. If the max float is sufficient, we pop
// the ticket at the head of the queue and send it into q.redeemable which an external listener can use to receive redeemable tickets
func (q *ticketQueue) startQueueLoop() {
	for {
		// Lock and wait until the queue is non-empty
		q.cond.L.Lock()
		for len(q.queue) == 0 {
			q.cond.Wait()

			select {
			case <-q.quit:
				// Unlock if we are exiting
				q.cond.L.Unlock()
				return
			default:
			}
		}

		nextTicket := q.queue[0]

		// Unlock since the queue is non-empty now
		q.cond.L.Unlock()

		select {
		case maxFloat := <-q.maxFloatUpdate:
			if q.sufficientMaxFloat(maxFloat, nextTicket) {
				select {
				case q.redeemable <- nextTicket:
					q.removeHead()
				case <-q.quit:
					return
				}
			}
		case <-q.quit:
			return
		default:
		}
	}
}

// removeHead removes the head of the queue
func (q *ticketQueue) removeHead() {
	// Lock conditional variable while removing from the queue
	q.cond.L.Lock()
	q.queue[0] = nil
	q.queue = q.queue[1:]
	atomic.AddInt32(&q.queueLen, -1)
	q.cond.L.Unlock()
}

// sufficientMaxFloat returns a boolean indicating whether the sender's
// current max float is sufficient to cover the given ticket
func (q *ticketQueue) sufficientMaxFloat(maxFloat *big.Int, ticket *SignedTicket) bool {
	return maxFloat.Cmp(ticket.FaceValue) >= 0
}
