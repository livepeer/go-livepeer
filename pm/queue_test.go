package pm

import (
	"math/big"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func defaultSignedTicket(senderNonce uint32) *SignedTicket {
	return &SignedTicket{
		&Ticket{FaceValue: big.NewInt(50), SenderNonce: senderNonce, ParamsExpirationBlock: big.NewInt(0)},
		[]byte("foo"),
		big.NewInt(7),
	}
}

type queueConsumer struct {
	redeemable []*SignedTicket
	mu         sync.Mutex
}

// Redeemable returns the consumed redeemable tickets from a ticket queue
func (qc *queueConsumer) Redeemable() []*SignedTicket {
	qc.mu.Lock()
	defer qc.mu.Unlock()

	return qc.redeemable
}

// Wait receives on the output channel from a ticket queue
// until it has received a certain number of tickets and then exits
func (qc *queueConsumer) Wait(num int, e RedeemableEmitter) {
	count := 0

	for count < num {
		select {
		case ticket := <-e.Redeemable():
			count++

			qc.mu.Lock()
			qc.redeemable = append(qc.redeemable, ticket)
			qc.mu.Unlock()
		}
	}
}

func TestTicketQueueLoop(t *testing.T) {
	assert := assert.New(t)

	tm := &stubTimeManager{}

	q := newTicketQueue(tm.SubscribeBlocks)
	q.Start()
	defer q.Stop()

	// Test adding tickets

	numTickets := 10

	for i := 0; i < numTickets; i++ {
		q.Add(defaultSignedTicket(uint32(i)))
	}

	// Add ticket with non-expired params
	nonExpTicket := defaultSignedTicket(10)
	nonExpTicket.ParamsExpirationBlock = big.NewInt(100)
	q.Add(nonExpTicket)
	assert.Equal(int32(numTickets+1), q.Length())
	time.Sleep(time.Millisecond * 20)

	qc := &queueConsumer{}

	// Wait for all numTickets tickets to be
	// received on the output channel returned by Redeemable()
	go qc.Wait(numTickets, q)

	// Test signaling a new blockNum and remove tickets
	tm.blockNumSink <- big.NewInt(1)

	// Queue should contain only the non-expired ticket now
	time.Sleep(time.Millisecond * 20)
	assert.Equal(int32(1), q.Length())
	assert.Equal(q.queue[0], nonExpTicket)

	// The popped tickets should be in the same order
	// that they were added i.e. since we added them
	// synchronously with sender nonces 0..9 the array
	// of popped tickets should have sender nonces 0..9
	// in order
	redeemable := qc.Redeemable()
	for i := 0; i < numTickets; i++ {
		assert.Equal(uint32(i), redeemable[i].SenderNonce)
	}
}

func TestTicketQueueLoopConcurrent(t *testing.T) {
	assert := assert.New(t)

	tm := &stubTimeManager{}

	q := newTicketQueue(tm.SubscribeBlocks)
	q.Start()
	defer q.Stop()

	// Initialize queue

	numTickets := 5

	for i := 0; i < numTickets; i++ {
		q.Add(defaultSignedTicket(uint32(i)))
	}
	assert.Equal(q.Length(), int32(numTickets))

	// Concurrently add tickets to the queue

	numAdds := 2
	for i := numTickets; i < numTickets+numAdds; i++ {
		go q.Add(defaultSignedTicket(uint32(i)))
	}
	time.Sleep(time.Millisecond * 20)
	assert.Equal(q.Length(), int32(numTickets+numAdds))

	// Concurrently signal block updates that do not remove tickets

	noRemoveSignals := 2
	for i := 0; i < noRemoveSignals; i++ {
		go func() {
			tm.blockNumSink <- big.NewInt(-1)
		}()
	}

	// Concurrently signal block updates that remove tickets
	removeSignals := 2

	qc := &queueConsumer{}
	go qc.Wait(numTickets+numAdds, q)

	for i := 0; i < removeSignals; i++ {
		go func() {
			tm.blockNumSink <- big.NewInt(1)
		}()
	}

	// Queue length should be empty
	time.Sleep(time.Millisecond * 20)
	assert.Equal(int32(0), q.Length())
}

func TestTicketQueueConsumeBlockNums(t *testing.T) {
	assert := assert.New(t)

	tm := &stubTimeManager{}

	q := newTicketQueue(tm.SubscribeBlocks)
	q.Start()
	defer q.Stop()
	time.Sleep(5 * time.Millisecond)

	tm.blockNumSink <- big.NewInt(10)
	time.Sleep(5 * time.Millisecond)
	// Check that the value is consumed
	assert.Len(tm.blockNumSink, 0)
}
