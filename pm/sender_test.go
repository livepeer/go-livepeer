package pm

import (
	"crypto/rand"
	"math/big"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/ethereum/go-ethereum/accounts"
	ethcommon "github.com/ethereum/go-ethereum/common"
)

func TestStartSession_GivenSomeRecipientRandHash_UsesItAsSessionId(t *testing.T) {
	sender := defaultSender(t)
	recipient := ethcommon.Address{}
	ticketParams := defaultTicketParams(t)
	expectedSessionId := hashToHex(ticketParams.RecipientRandHash)

	sessionId := sender.StartSession(recipient, TicketParams{
		FaceValue:         big.NewInt(0),
		WinProb:           big.NewInt(0),
		Seed:              big.NewInt(0),
		RecipientRandHash: ticketParams.RecipientRandHash,
	})

	if sessionId != expectedSessionId {
		t.Errorf("expected %v to equal %v", sessionId, expectedSessionId)
	}
}

func TestStartSession_GivenConcurrentUsage_RecordsAllSessions(t *testing.T) {
	sender := defaultSender(t)
	recipient := ethcommon.Address{}

	var sessions []string
	var sessionCount int32
	ch := make(chan struct{})
	for i := 0; i < 100; i++ {
		ticketParams := defaultTicketParams(t)
		expectedSessionId := hashToHex(ticketParams.RecipientRandHash)
		sessions = append(sessions, expectedSessionId)

		go func() {
			sender.StartSession(recipient, TicketParams{
				FaceValue:         big.NewInt(0),
				WinProb:           big.NewInt(0),
				Seed:              big.NewInt(0),
				RecipientRandHash: ticketParams.RecipientRandHash,
			})
			currentCount := atomic.AddInt32(&sessionCount, 1)
			if currentCount >= 100 {
				ch <- struct{}{}
			}
		}()
	}
	// waiting for all session to be created
	<-ch

	for _, sessionId := range sessions {
		_, ok := sender.sessions.Load(sessionId)
		if !ok {
			t.Errorf("expected to find sessionId in sender. sessionId: %v", sessionId)
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
		t.Errorf("error tryint to create a ticket: %v", err)
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
	if !reflect.DeepEqual(actualSig, am.signResponse) {
		t.Errorf("expected actual sig %v to be %v", actualSig, am.signResponse)
	}
	if !reflect.DeepEqual(am.lastSignRequest, ticket.Hash().Bytes()) {
		t.Errorf("expected sig message bytes %v to be %v", am.lastSignRequest, ticket.Hash().Bytes())
	}
}

// TODO sign error test
func TestCreateTicket_GivenConcurrentCallsForSameSession_SenderNonceIncrementsCorrectly(t *testing.T) {
	// TODO
}

func defaultSender(t *testing.T) *DefaultSender {
	account := accounts.Account{
		Address: randAddressOrFatal(t),
	}
	am := &stubAccountManager{
		account: account,
	}
	sender := NewSender(am)
	return sender.(*DefaultSender)
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
