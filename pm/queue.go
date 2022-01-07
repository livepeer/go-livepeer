package pm

import (
	"math/big"
	"strings"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/golang/glog"
)

const ticketValidityPeriod = 2

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
	tm TimeManager
	// redeemable is a channel that a queue consumer will receive
	// redeemable tickets on as a sender's max float becomes
	// sufficient to cover the face value of tickets
	redeemable chan *redemption

	sender ethcommon.Address
	store  TicketStore

	quit chan struct{}
}

func newTicketQueue(sender ethcommon.Address, sm *LocalSenderMonitor) *ticketQueue {
	return &ticketQueue{
		tm:         sm.tm,
		redeemable: make(chan *redemption),
		store:      sm.ticketStore,
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
	return q.store.WinningTicketCount(q.sender, new(big.Int).Sub(q.tm.LastInitializedRound(), big.NewInt(ticketValidityPeriod)).Int64())
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
	sub := q.tm.SubscribeBlocks(blockNums)
	defer sub.Unsubscribe()

ticketLoop:
	for {
		select {
		case err := <-sub.Err():
			if err != nil {
				glog.Errorf("Block subscription error err=%q", err)
			}
		case latestBlock := <-blockNums:
			numTickets, err := q.Length()
			if err != nil {
				glog.Errorf("Error getting queue length err=%q", err)
				continue
			}
			for i := 0; i < int(numTickets); i++ {
				nextTicket, err := q.store.SelectEarliestWinningTicket(q.sender, new(big.Int).Sub(q.tm.LastInitializedRound(), big.NewInt(ticketValidityPeriod)).Int64())
				if err != nil {
					glog.Errorf("Unable select earliest winning ticket err=%q", err)
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

					q.redeemable <- &redemption{nextTicket, resCh}
					select {
					case res := <-resCh:
						// after receiving the response we can close the channel so it can be GC'd
						close(resCh)
						if res.err != nil {
							glog.Errorf("Error redeeming err=%q", res.err)
							// If the error is non-retryable then we mark the ticket as redeemed
							if !isNonRetryableTicketErr(res.err) {
								continue
							}
						}
						if err := q.store.MarkWinningTicketRedeemed(nextTicket, res.txHash); err != nil {
							glog.Error(err)
							continue
						}
					case <-q.quit:
						return
					}
				}
			}
		case <-q.quit:
			return
		}
	}
}

func isNonRetryableTicketErr(err error) bool {
	// The latter check depends on logic in eth.client.CheckTx()
	return err == errIsUsedTicket || strings.Contains(err.Error(), "transaction failed")
}
