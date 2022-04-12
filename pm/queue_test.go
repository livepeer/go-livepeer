package pm

import (
	"fmt"
	"math/big"
	"sync"
	"testing"
	"time"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

func defaultSignedTicket(sender ethcommon.Address, senderNonce uint32) *SignedTicket {
	return &SignedTicket{
		&Ticket{
			Recipient:              RandAddress(),
			Sender:                 sender,
			FaceValue:              big.NewInt(50),
			WinProb:                big.NewInt(500),
			SenderNonce:            senderNonce,
			RecipientRandHash:      RandHash(),
			CreationRound:          100,
			CreationRoundBlockHash: RandHash(),
			ParamsExpirationBlock:  big.NewInt(0),
			PricePerPixel:          big.NewRat(1, 1),
		},
		RandBytes(32),
		big.NewInt(7),
	}
}

type queueConsumer struct {
	redeemable    []*redemption
	mu            sync.Mutex
	redemptionErr error
}

// Redeemable returns the consumed redeemable tickets from a ticket queue
func (qc *queueConsumer) Redeemable() []*redemption {
	qc.mu.Lock()
	defer qc.mu.Unlock()

	return qc.redeemable
}

// Wait receives on the output channel from a ticket queue
// until it has received a certain number of tickets and then exits
func (qc *queueConsumer) Wait(num int, e RedeemableEmitter, done chan struct{}) {
	count := 0
	for count < num {
		select {
		case ticket := <-e.Redeemable():
			count++
			qc.mu.Lock()
			qc.redeemable = append(qc.redeemable, ticket)
			qc.mu.Unlock()
			ticket.resCh <- struct {
				txHash ethcommon.Hash
				err    error
			}{ticket.SignedTicket.Hash(), qc.redemptionErr}
		}
	}
	done <- struct{}{}
}

func TestTicketQueueLoop(t *testing.T) {
	assert := assert.New(t)

	sender := RandAddress()
	ts := newStubTicketStore()
	tm := &stubTimeManager{round: big.NewInt(100)}
	sm := &LocalSenderMonitor{
		ticketStore: ts,
		tm:          tm,
	}

	q := newTicketQueue(sender, sm)
	q.Start()
	defer q.Stop()

	// Test adding tickets

	numTickets := 10

	for i := 0; i < numTickets; i++ {
		q.Add(defaultSignedTicket(sender, uint32(i)))
	}

	// Add ticket with non-expired params
	nonExpTicket := defaultSignedTicket(sender, uint32(numTickets))
	nonExpTicket.ParamsExpirationBlock = big.NewInt(100)
	q.Add(nonExpTicket)
	qlen, err := q.Length()
	assert.Nil(err)
	assert.Equal(numTickets+1, qlen)
	time.Sleep(time.Millisecond * 20)
	qc := &queueConsumer{}
	done := make(chan struct{})
	// Wait for all numTickets tickets to be
	// received on the output channel returned by Redeemable()
	go qc.Wait(numTickets, q, done)
	time.Sleep(time.Millisecond * 20)

	// Test signaling a new blockNum and remove tickets
	tm.blockNumSink <- big.NewInt(1)
	<-done
	time.Sleep(20 * time.Millisecond)

	// Queue should contain only the non-expired ticket now
	qlen, err = q.Length()
	assert.Nil(err)
	assert.Equal(1, qlen)
	earliest, err := q.store.SelectEarliestWinningTicket(sender, nonExpTicket.CreationRound)
	assert.Nil(err)
	assert.Equal(earliest, nonExpTicket)

	// The popped tickets should be in the same order
	// that they were added i.e. since we added them
	// synchronously with sender nonces 0..9 the array
	// of popped tickets should have sender nonces 0..9
	// in order
	redeemable := qc.Redeemable()
	for i := 0; i < numTickets; i++ {
		assert.Equal(uint32(i), redeemable[i].SignedTicket.SenderNonce)
		assert.True(ts.submitted[fmt.Sprintf("%x", redeemable[i].SignedTicket.Sig)])
	}
}

func TestTicketQueueLoop_IsNonRetryableTicketErr_MarkAsRedeemed(t *testing.T) {
	assert := assert.New(t)

	sender := RandAddress()
	ts := newStubTicketStore()
	tm := &stubTimeManager{round: big.NewInt(100)}
	sm := &LocalSenderMonitor{
		ticketStore: ts,
		tm:          tm,
	}

	q := newTicketQueue(sender, sm)
	q.Start()
	defer q.Stop()

	addTicket := func(ticket *SignedTicket) {
		q.Add(ticket)

		qlen, err := q.Length()
		assert.Nil(err)
		assert.Equal(1, qlen)
	}

	consumeQueue := func(qc *queueConsumer) {
		done := make(chan struct{})
		go qc.Wait(1, q, done)
		time.Sleep(20 * time.Millisecond)

		tm.blockNumSink <- big.NewInt(1)
		<-done
		time.Sleep(100 * time.Millisecond)
	}

	// Test that ticket is marked as redeemed if ticket is used
	ticket := defaultSignedTicket(sender, 0)
	addTicket(ticket)

	qc := &queueConsumer{redemptionErr: errIsUsedTicket}
	consumeQueue(qc)
	assert.True(ts.submitted[fmt.Sprintf("%x", ticket.Sig)])

	// Test that ticket is marked as redeemed if tx fails
	ticket = defaultSignedTicket(sender, 1)
	addTicket(ticket)

	qc = &queueConsumer{
		redemptionErr: errors.New("transaction failed txHash=abc"),
	}
	consumeQueue(qc)
	assert.True(ts.submitted[fmt.Sprintf("%x", ticket.Sig)])

	// Test that ticket is marked as redeemed if it has zero as block hash
	ticket = defaultSignedTicket(sender, 1)
	addTicket(ticket)

	qc = &queueConsumer{
		redemptionErr: errors.New("ticket creationRound does not have a block hash"),
	}
	consumeQueue(qc)
	assert.True(ts.submitted[fmt.Sprintf("%x", ticket.Sig)])

	// Test that ticket is not marked as redeemed if there is an error checking the tx, but the tx did not fail
	ticket = defaultSignedTicket(sender, 2)
	addTicket(ticket)

	qc = &queueConsumer{
		redemptionErr: errors.New("some other error"),
	}
	consumeQueue(qc)
	assert.False(ts.submitted[fmt.Sprintf("%x", ticket.Sig)])
}

func TestTicketQueueLoopConcurrent(t *testing.T) {
	assert := assert.New(t)

	sender := RandAddress()
	ts := newStubTicketStore()
	tm := &stubTimeManager{round: big.NewInt(100)}
	sm := &LocalSenderMonitor{
		ticketStore: ts,
		tm:          tm,
	}

	q := newTicketQueue(sender, sm)
	q.Start()
	defer q.Stop()

	// Initialize queue

	numTickets := 5

	for i := 0; i < numTickets; i++ {
		q.Add(defaultSignedTicket(sender, uint32(i)))
	}
	qlen, err := q.Length()
	assert.Nil(err)
	assert.Equal(qlen, numTickets)

	// Concurrently add tickets to the queue

	numAdds := 2
	for i := numTickets; i < numTickets+numAdds; i++ {
		go q.Add(defaultSignedTicket(sender, uint32(i)))
	}
	time.Sleep(time.Millisecond * 20)
	qlen, err = q.Length()
	assert.Nil(err)
	assert.Equal(qlen, numTickets+numAdds)

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
	done := make(chan struct{})
	go qc.Wait(numTickets+numAdds, q, done)

	for i := 0; i < removeSignals; i++ {
		go func() {
			tm.blockNumSink <- big.NewInt(1)
		}()
	}

	// Queue length should be empty
	time.Sleep(20 * time.Millisecond)
	<-done
	qlen, err = q.Length()
	assert.Nil(err)
	assert.Equal(0, qlen)
}

func TestTicketQueueConsumeBlockNums(t *testing.T) {
	assert := assert.New(t)

	sender := RandAddress()
	ts := newStubTicketStore()
	tm := &stubTimeManager{round: big.NewInt(100)}
	sm := &LocalSenderMonitor{
		ticketStore: ts,
		tm:          tm,
	}

	q := newTicketQueue(sender, sm)
	q.Start()
	defer q.Stop()
	time.Sleep(20 * time.Millisecond)

	tm.blockNumSink <- big.NewInt(10)
	time.Sleep(20 * time.Millisecond)
	// Check that the value is consumed
	assert.Len(tm.blockNumSink, 0)
}

func TestTicketQueue_Add(t *testing.T) {
	assert := assert.New(t)

	sender := RandAddress()
	ts := newStubTicketStore()
	tm := &stubTimeManager{round: big.NewInt(100)}
	sm := &LocalSenderMonitor{
		ticketStore: ts,
		tm:          tm,
	}

	q := newTicketQueue(sender, sm)

	ticket := defaultSignedTicket(sender, 0)

	ts.storeShouldFail = true
	err := q.Add(ticket)
	assert.EqualError(err, "stub TicketStore store error")
	ts.storeShouldFail = false

	err = q.Add(ticket)
	assert.Nil(err)
	assert.Equal(ts.tickets[sender][0], ticket)
}

func TestTicketQueue_Length(t *testing.T) {
	assert := assert.New(t)

	sender := RandAddress()
	ts := newStubTicketStore()
	tm := &stubTimeManager{round: big.NewInt(100)}
	sm := &LocalSenderMonitor{
		ticketStore: ts,
		tm:          tm,
	}

	q := newTicketQueue(sender, sm)

	ts.tickets[sender] = []*SignedTicket{defaultSignedTicket(sender, 0), defaultSignedTicket(sender, 1), defaultSignedTicket(sender, 2)}

	ts.loadShouldFail = true
	qlen, err := q.Length()
	assert.Equal(0, qlen)
	assert.EqualError(err, "stub TicketStore load error")
	ts.loadShouldFail = false

	qlen, err = q.Length()
	assert.Nil(err)
	assert.Equal(qlen, 3)
}

func TestIsRecipientActive(t *testing.T) {
	assert := assert.New(t)

	ts := newStubTicketStore()
	tm := &stubTimeManager{round: big.NewInt(100)}
	sm := &LocalSenderMonitor{
		ticketStore: ts,
		tm:          tm,
	}
	sender := RandAddress()
	q := newTicketQueue(sender, sm)
	addr := RandAddress()

	// active
	ts.isActive = true
	assert.True(q.isRecipientActive(addr))

	// inactive
	ts.isActive = false
	assert.False(q.isRecipientActive(addr))

	// db error
	ts.err = errors.New("some error")
	assert.True(q.isRecipientActive(addr))
}
