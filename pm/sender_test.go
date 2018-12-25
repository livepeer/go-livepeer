package pm

import (
	"bytes"
	"crypto/rand"
	"math/big"
	"strings"
	"sync"
	"testing"

	"github.com/ethereum/go-ethereum/accounts"
	ethcommon "github.com/ethereum/go-ethereum/common"
)

func TestStartSession_GivenSomeRecipientRandHash_UsesItAsSessionId(t *testing.T) {
	sender := defaultSender(t)
	recipient := ethcommon.Address{}
	ticketParams := defaultTicketParams(t)
	expectedSessionID := ticketParams.RecipientRandHash.Hex()

	sessionID := sender.StartSession(recipient, TicketParams{
		FaceValue:         big.NewInt(0),
		WinProb:           big.NewInt(0),
		Seed:              big.NewInt(0),
		RecipientRandHash: ticketParams.RecipientRandHash,
	})

	if sessionID != expectedSessionID {
		t.Errorf("expected %v to equal %v", sessionID, expectedSessionID)
	}
}

func TestStartSession_GivenConcurrentUsage_RecordsAllSessions(t *testing.T) {
	sender := defaultSender(t)
	recipient := ethcommon.Address{}

	var sessions []string
	var wg sync.WaitGroup
	wg.Add(100)
	for i := 0; i < 100; i++ {
		ticketParams := defaultTicketParams(t)
		expectedSessionID := ticketParams.RecipientRandHash.Hex()
		sessions = append(sessions, expectedSessionID)

		go func() {
			sender.StartSession(recipient, TicketParams{
				FaceValue:         big.NewInt(0),
				WinProb:           big.NewInt(0),
				Seed:              big.NewInt(0),
				RecipientRandHash: ticketParams.RecipientRandHash,
			})
			wg.Done()
		}()
	}
	wg.Wait()

	for _, sessionID := range sessions {
		_, ok := sender.sessions.Load(sessionID)
		if !ok {
			t.Errorf("expected to find sessionID in sender. sessionID: %v", sessionID)
		}
	}
}

func TestCreateTicket_GivenNonexistentSession_ReturnsError(t *testing.T) {
	sender := defaultSender(t)

	_, _, _, err := sender.CreateTicket("foo")

	if err == nil {
		t.Errorf("expected an error for nonexistent session")
	}
	if !strings.Contains(err.Error(), "unknown session") {
		t.Errorf("expected error to contain 'unknown session' but instead got: %v", err.Error())
	}
}

func TestCreateTicket_GivenValidSessionId_UsesSessionParamsInTicket(t *testing.T) {
	sender := defaultSender(t)
	am := sender.accountManager.(*stubAccountManager)
	am.signShouldFail = false
	am.saveSignRequest = true
	am.signResponse = randBytesOrFatal(42, t)
	senderAddress := sender.accountManager.Account().Address
	recipient := randAddressOrFatal(t)
	recipientRandHash := randHashOrFatal(t)
	ticketParams := TicketParams{
		FaceValue:         big.NewInt(1111),
		WinProb:           big.NewInt(2222),
		Seed:              big.NewInt(3333),
		RecipientRandHash: recipientRandHash,
	}
	sessionID := sender.StartSession(recipient, ticketParams)

	ticket, actualSeed, actualSig, err := sender.CreateTicket(sessionID)

	if err != nil {
		t.Errorf("error trying to create a ticket: %v", err)
	}
	if ticket.Sender != senderAddress {
		t.Errorf("expected ticket sender %v to be %v", ticket.Sender, senderAddress)
	}
	if ticket.Recipient != recipient {
		t.Errorf("expeceted ticket recipient %v to be %v", ticket.Recipient, recipient)
	}
	if ticket.RecipientRandHash != recipientRandHash {
		t.Errorf("expected ticket recipientRandHash %v to be %v", ticket.RecipientRandHash, recipientRandHash)
	}
	if ticket.FaceValue != ticketParams.FaceValue {
		t.Errorf("expected ticket FaceValue %v to be %v", ticket.FaceValue, ticketParams.FaceValue)
	}
	if ticket.WinProb != ticketParams.WinProb {
		t.Errorf("expected ticket WinProb %v to be %v", ticket.WinProb, ticketParams.WinProb)
	}
	if ticket.SenderNonce != 1 {
		t.Errorf("expected ticket SenderNonce %d to be 1", ticket.SenderNonce)
	}
	if actualSeed != ticketParams.Seed {
		t.Errorf("expected actual seed %d to be %d", actualSeed, ticketParams.Seed)
	}
	if !bytes.Equal(actualSig, am.signResponse) {
		t.Errorf("expected actual sig %v to be %v", actualSig, am.signResponse)
	}
	if !bytes.Equal(am.lastSignRequest, ticket.Hash().Bytes()) {
		t.Errorf("expected sig message bytes %v to be %v", am.lastSignRequest, ticket.Hash().Bytes())
	}
}

func TestCreateTicket_GivenSigningError_ReturnsError(t *testing.T) {
	sender := defaultSender(t)
	recipient := randAddressOrFatal(t)
	ticketParams := defaultTicketParams(t)
	sessionID := sender.StartSession(recipient, ticketParams)
	am := sender.accountManager.(*stubAccountManager)
	am.signShouldFail = true

	_, _, _, err := sender.CreateTicket(sessionID)

	if err == nil {
		t.Errorf("expected an error when trying to sign the ticket")
	}
	if !strings.Contains(err.Error(), "error signing") {
		t.Errorf("expected error to contain 'error signing' but instead got: %v", err.Error())
	}
}

func TestCreateTicket_GivenConcurrentCallsForSameSession_SenderNonceIncrementsCorrectly(t *testing.T) {
	totalTickets := 100
	lock := sync.RWMutex{}
	sender := defaultSender(t)
	recipient := randAddressOrFatal(t)
	ticketParams := defaultTicketParams(t)
	sessionID := sender.StartSession(recipient, ticketParams)

	var wg sync.WaitGroup
	wg.Add(totalTickets)
	var tickets []*Ticket
	for i := 0; i < totalTickets; i++ {

		go func() {
			ticket, _, _, _ := sender.CreateTicket(sessionID)

			lock.Lock()
			tickets = append(tickets, ticket)
			lock.Unlock()

			wg.Done()
		}()
	}
	wg.Wait()

	sessionUntyped, ok := sender.sessions.Load(sessionID)
	if !ok {
		t.Fatalf("failed to find session with ID %v", sessionID)
	}
	session := sessionUntyped.(*session)
	if session.senderNonce != uint64(totalTickets) {
		t.Errorf("expected end state SenderNonce %d to be %d", session.senderNonce, totalTickets)
	}

	uniqueNonces := make(map[uint64]bool)
	for _, ticket := range tickets {
		uniqueNonces[ticket.SenderNonce] = true
	}
	if len(uniqueNonces) != totalTickets {
		t.Errorf("expected unique nonces count %d to be %d", len(uniqueNonces), totalTickets)
	}
}

func defaultSender(t *testing.T) *sender {
	account := accounts.Account{
		Address: randAddressOrFatal(t),
	}
	am := &stubAccountManager{
		account: account,
	}
	s := NewSender(am)
	return s.(*sender)
}

func defaultTicketParams(t *testing.T) TicketParams {
	recipientRandHash := randHashOrFatal(t)
	return TicketParams{
		FaceValue:         big.NewInt(0),
		WinProb:           big.NewInt(0),
		Seed:              big.NewInt(0),
		RecipientRandHash: recipientRandHash,
	}
}

func randHashOrFatal(t *testing.T) ethcommon.Hash {
	key, err := randBytes(32)

	if err != nil {
		t.Fatalf("failed generating random hash: %v", err)
		return ethcommon.Hash{}
	}

	return ethcommon.BytesToHash(key[:])
}

func randAddressOrFatal(t *testing.T) ethcommon.Address {
	key, err := randBytes(addressSize)

	if err != nil {
		t.Fatalf("failed generating random address: %v", err)
		return ethcommon.Address{}
	}

	return ethcommon.BytesToAddress(key[:])
}

func randBytesOrFatal(size int, t *testing.T) []byte {
	res, err := randBytes(size)

	if err != nil {
		t.Fatalf("failed generating random bytes: %v", err)
		return nil
	}

	return res
}

func randBytes(size int) ([]byte, error) {
	key := make([]byte, size)
	_, err := rand.Read(key)

	return key, err
}
