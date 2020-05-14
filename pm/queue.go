package pm

import (
	"math/big"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/event"
	"github.com/golang/glog"
)

// RedeemableEmitter is an interface that describes methods for
// emitting redeemable tickets
type RedeemableEmitter interface {
	// Redeemable returns a channel that a consumer can use to receive tickets that
	// should be redeemed
	Redeemable() chan *redemption
}

type redemption struct {
	SignedTicket *SignedTicket
	resCh        chan struct {
		txHash ethcommon.Hash
		err    error
	}
}

// ticketQueue is a queue of winning tickets that are in line for redemption on-chain.
// A recipient will have a ticketQueue per sender that it is actively receiving tickets from.
// If a sender's max float is insufficient to cover the face value of a ticket it is added to the queue.
// A ticket is pulled from the queue by the recipient when a sender has sufficient max float to cover
// the next ticket in the queue
//
// Based off of: https://github.com/lightningnetwork/lnd/blob/master/htlcswitch/queue.go
type ticketQueue struct {
	// blockSub returns a subscription to receive the last seen block number
	blockSub func(chan<- *big.Int) event.Subscription

	// redeemable is a channel that a queue consumer will receive
	// redeemable tickets on as a sender's max float becomes
	// sufficient to cover the face value of tickets
	redeemable chan *redemption

	sender ethcommon.Address
	store  TicketStore

	quit chan struct{}
}

func newTicketQueue(store TicketStore, sender ethcommon.Address, blockSub func(chan<- *big.Int) event.Subscription) *ticketQueue {
	return &ticketQueue{
		blockSub:   blockSub,
		redeemable: make(chan *redemption),
		store:      store,
		sender:     sender,
		quit:       make(chan struct{}),
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
func (q *ticketQueue) Add(ticket *SignedTicket) error {
	return q.store.StoreWinningTicket(ticket)
}

// Redeemable returns a channel that a consumer can use to receive tickets that
// should be redeemed
// pm.SenderMonitor is the primary consumer of this channel
func (q *ticketQueue) Redeemable() chan *redemption {
	return q.redeemable
}

// Length returns the current length of the queue
func (q *ticketQueue) Length() (int, error) {
	return q.store.WinningTicketCount(q.sender)
}

// startQueueLoop blocks until the ticket queue is non-empty. When the queue is non-empty
// the loop will block until a value is received on q.maxFloatUpdate which should be the most
// up-to-date max float for the ticket sender associated with the queue. The loop should receive max float
// updates whenever a pending transaction for a ticket redemption confirms (thus tickets can only be popped
// from the queue as redemption transactions confirm). When a max float value is received, the loop checks if it
// is sufficient to cover the face value of the ticket at the head of the queue. If the max float is sufficient, we pop
// the ticket at the head of the queue and send it into q.redeemable which an external listener can use to receive redeemable tickets
func (q *ticketQueue) startQueueLoop() {
	blockNums := make(chan *big.Int, 10)
	sub := q.blockSub(blockNums)
	defer sub.Unsubscribe()

ticketLoop:
	for {
		select {
		case err := <-sub.Err():
			if err != nil {
				glog.Errorf("Block subscription error err=%v", err)
			}
		case latestBlock := <-blockNums:
			numTickets, err := q.Length()
			if err != nil {
				glog.Errorf("Error getting queue length err=%v", err)
				continue
			}
			for i := 0; i < int(numTickets); i++ {
				nextTicket, err := q.store.SelectEarliestWinningTicket(q.sender)
				if err != nil {
					glog.Errorf("Unable select earliest winning ticket err=%v", err)
					continue ticketLoop
				}
				if nextTicket == nil {
					continue ticketLoop
				}
				if nextTicket.ParamsExpirationBlock.Cmp(latestBlock) <= 0 {
					resCh := make(chan struct {
						txHash ethcommon.Hash
						err    error
					})
					select {
					case q.redeemable <- &redemption{nextTicket, resCh}:
						res := <-resCh
						if res.err != nil {
							glog.Error(err)
							continue
						}
						err := q.store.MarkWinningTicketRedeemed(nextTicket, res.txHash)
						if err != nil {
							glog.Error(err)
							continue
						}
					case <-q.quit:
						return
					}
				} else {
					q.Add(nextTicket)
				}
			}
		case <-q.quit:
			return
		default:
		}
	}
}
