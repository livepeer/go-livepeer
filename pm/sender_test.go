package pm

import (
	"bytes"
	"math/big"
	"strings"
	"sync"
	"testing"

	"github.com/ethereum/go-ethereum/accounts"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

func TestStartSession_GivenSomeRecipientRandHash_UsesItAsSessionId(t *testing.T) {
	sender := defaultSender(t)
	recipient := ethcommon.Address{}
	ticketParams := defaultTicketParams(t, recipient)
	expectedSessionID := ticketParams.RecipientRandHash.Hex()

	sessionID := sender.StartSession(TicketParams{
		Recipient:         recipient,
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
		ticketParams := defaultTicketParams(t, recipient)
		expectedSessionID := ticketParams.RecipientRandHash.Hex()
		sessions = append(sessions, expectedSessionID)

		go func() {
			sender.StartSession(TicketParams{
				Recipient:         recipient,
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

func TestCreateTicket_GetSenderInfoError_ReturnsError(t *testing.T) {
	sender := defaultSender(t)
	sm := sender.senderManager.(*stubSenderManager)
	sm.err = errors.New("GetSenderInfo error")

	sessionID := sender.StartSession(defaultTicketParams(t, RandAddress()))
	_, _, _, err := sender.CreateTicket(sessionID)
	assert.EqualError(t, err, sm.err.Error())
}

func TestCreateTicket_EVTooHigh_ReturnsError(t *testing.T) {
	sender := defaultSender(t)
	sender.maxEV = big.NewRat(100, 1)

	ticketParams := TicketParams{
		Recipient:         RandAddress(),
		FaceValue:         big.NewInt(202),
		WinProb:           new(big.Int).Div(maxWinProb, big.NewInt(2)),
		Seed:              big.NewInt(3333),
		RecipientRandHash: RandHash(),
	}
	sessionID := sender.StartSession(ticketParams)
	_, _, _, err := sender.CreateTicket(sessionID)
	assert.Contains(t, "ticket EV higher than max EV", err.Error())
}

func TestCreateTicket_FaceValueTooHigh_ReturnsError(t *testing.T) {
	sender := defaultSender(t)
	sm := sender.senderManager.(*stubSenderManager)
	sm.info = &SenderInfo{
		Deposit: big.NewInt(0),
	}

	ticketParams := TicketParams{
		Recipient:         RandAddress(),
		FaceValue:         big.NewInt(1111),
		WinProb:           big.NewInt(2222),
		Seed:              big.NewInt(3333),
		RecipientRandHash: RandHash(),
	}
	sessionID := sender.StartSession(ticketParams)
	_, _, _, err := sender.CreateTicket(sessionID)
	assert.Contains(t, "ticket faceValue higher than max faceValue", err.Error())
}

func TestCreateTicket_GivenLastInitializedRoundError_ReturnsError(t *testing.T) {
	sender := defaultSender(t)
	rm := sender.roundsManager.(*stubRoundsManager)
	expErr := errors.New("LastInitializedRound error")
	rm.lastInitializedRoundErr = expErr

	sessionID := sender.StartSession(defaultTicketParams(t, RandAddress()))
	_, _, _, err := sender.CreateTicket(sessionID)
	assert.EqualError(t, err, expErr.Error())
}

func TestCreateTicket_GivenBlockHashForRoundError_ReturnsError(t *testing.T) {
	sender := defaultSender(t)
	rm := sender.roundsManager.(*stubRoundsManager)
	expErr := errors.New("BlockHashForRound error")
	rm.blockHashForRoundErr = expErr

	sessionID := sender.StartSession(defaultTicketParams(t, RandAddress()))
	_, _, _, err := sender.CreateTicket(sessionID)
	assert.EqualError(t, err, expErr.Error())
}

func TestCreateTicket_GivenValidSessionId_UsesSessionParamsInTicket(t *testing.T) {
	sender := defaultSender(t)
	rm := sender.roundsManager.(*stubRoundsManager)
	creationRound := rm.round
	creationRoundBlkHash := rm.blkHash
	am := sender.signer.(*stubSigner)
	am.signShouldFail = false
	am.saveSignRequest = true
	am.signResponse = RandBytes(42)
	senderAddress := sender.signer.Account().Address
	recipient := RandAddress()
	recipientRandHash := RandHash()
	ticketParams := TicketParams{
		Recipient:         recipient,
		FaceValue:         big.NewInt(1111),
		WinProb:           big.NewInt(2222),
		Seed:              big.NewInt(3333),
		RecipientRandHash: recipientRandHash,
	}
	sessionID := sender.StartSession(ticketParams)

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
	if big.NewInt(ticket.CreationRound).Cmp(creationRound) != 0 {
		t.Errorf("expected creation round %v to be %v", ticket.CreationRound, creationRound)
	}
	if !bytes.Equal(ticket.CreationRoundBlockHash.Bytes(), creationRoundBlkHash[:]) {
		t.Errorf("expected creation round block hash %x to be %x", ticket.CreationRoundBlockHash, creationRoundBlkHash)
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
	recipient := RandAddress()
	ticketParams := defaultTicketParams(t, recipient)
	sessionID := sender.StartSession(ticketParams)
	am := sender.signer.(*stubSigner)
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
	recipient := RandAddress()
	ticketParams := defaultTicketParams(t, recipient)
	sessionID := sender.StartSession(ticketParams)

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
	if session.senderNonce != uint32(totalTickets) {
		t.Errorf("expected end state SenderNonce %d to be %d", session.senderNonce, totalTickets)
	}

	uniqueNonces := make(map[uint32]bool)
	for _, ticket := range tickets {
		uniqueNonces[ticket.SenderNonce] = true
	}
	if len(uniqueNonces) != totalTickets {
		t.Errorf("expected unique nonces count %d to be %d", len(uniqueNonces), totalTickets)
	}
}

func TestValidateTicketParams_EVTooHigh_ReturnsError(t *testing.T) {
	sender := defaultSender(t)
	sender.maxEV = big.NewRat(100, 1)

	ticketParams := &TicketParams{
		FaceValue: big.NewInt(202),
		WinProb:   new(big.Int).Div(maxWinProb, big.NewInt(2)),
	}
	err := sender.ValidateTicketParams(ticketParams)
	assert.Contains(t, "ticket EV higher than max EV", err.Error())
}

func TestValidateTicketParams_FaceValueTooHigh_ReturnsError(t *testing.T) {
	assert := assert.New(t)

	// Test when deposit = 0 and faceValue != 0
	sender := defaultSender(t)
	sm := sender.senderManager.(*stubSenderManager)
	sm.info = &SenderInfo{
		Deposit: big.NewInt(0),
	}

	ticketParams := &TicketParams{
		FaceValue: big.NewInt(1111),
		WinProb:   big.NewInt(2222),
	}
	err := sender.ValidateTicketParams(ticketParams)
	assert.Contains("ticket faceValue higher than max faceValue", err.Error())

	// Test when deposit / depositMultiplier < faceValue
	sm.info.Deposit = big.NewInt(300)
	sender.maxEV = big.NewRat(100, 1)
	sender.depositMultiplier = 5
	maxFaceValue := new(big.Int).Div(sm.info.Deposit, big.NewInt(int64(sender.depositMultiplier)))

	ticketParams.FaceValue = new(big.Int).Add(maxFaceValue, big.NewInt(1))
	err = sender.ValidateTicketParams(ticketParams)
	assert.Contains("ticket faceValue higher than max faceValue", err.Error())
}

func TestValidateTicketParams_AcceptableParams_NoError(t *testing.T) {
	// Test when ev < maxEV and faceValue < maxFaceValue
	// maxEV = 100
	// maxFaceValue = 300 / 2 = 150
	// faceValue = 150 - 1 = 149
	// ev = 149 * .5 = 74.5
	sender := defaultSender(t)
	sm := sender.senderManager.(*stubSenderManager)
	sm.info = &SenderInfo{
		Deposit: big.NewInt(300),
	}
	sender.maxEV = big.NewRat(100, 1)
	sender.depositMultiplier = 2
	maxFaceValue := new(big.Int).Div(sm.info.Deposit, big.NewInt(int64(sender.depositMultiplier)))

	ticketParams := &TicketParams{
		FaceValue: new(big.Int).Sub(maxFaceValue, big.NewInt(1)),
		WinProb:   new(big.Int).Div(maxWinProb, big.NewInt(2)),
	}
	err := sender.ValidateTicketParams(ticketParams)
	assert.Nil(t, err)

	// Test when ev = maxEV and faceValue < maxFaceValue
	// maxEV = 100
	// maxFaceValue = 402 / 2 = 201
	// faceValue = 201 - 1 = 200
	// ev = 200 * .5 = 100
	sm.info.Deposit = big.NewInt(402)
	maxFaceValue = new(big.Int).Div(sm.info.Deposit, big.NewInt(int64(sender.depositMultiplier)))

	ticketParams.FaceValue = new(big.Int).Sub(maxFaceValue, big.NewInt(1))
	err = sender.ValidateTicketParams(ticketParams)
	assert.Nil(t, err)

	// Test when ev < maxEV and faceValue = maxFaceValue
	// maxEV = 100
	// maxFaceValue = 399 / 2 = 199
	// faceValue = 199
	// ev = 199 * .5 = 99.5
	sm.info.Deposit = big.NewInt(399)
	maxFaceValue = new(big.Int).Div(sm.info.Deposit, big.NewInt(int64(sender.depositMultiplier)))

	ticketParams.FaceValue = maxFaceValue
	err = sender.ValidateTicketParams(ticketParams)
	assert.Nil(t, err)

	// Test when ev = maxEV and faceValue = maxFaceValue
	// maxEV = 100
	// maxFaceValue = 400 / 2 = 200
	// faceValue = 200
	// ev = 200 * .5 = 100
	sm.info.Deposit = big.NewInt(400)
	maxFaceValue = new(big.Int).Div(sm.info.Deposit, big.NewInt(int64(sender.depositMultiplier)))

	ticketParams.FaceValue = maxFaceValue
	err = sender.ValidateTicketParams(ticketParams)
	assert.Nil(t, err)
}

func defaultSender(t *testing.T) *sender {
	account := accounts.Account{
		Address: RandAddress(),
	}
	am := &stubSigner{
		account: account,
	}
	rm := &stubRoundsManager{round: big.NewInt(5), blkHash: [32]byte{5}}
	sm := &stubSenderManager{}
	sm.info = &SenderInfo{
		Deposit: big.NewInt(100000),
	}
	s := NewSender(am, rm, sm, big.NewRat(100, 1), 2)
	return s.(*sender)
}

func defaultTicketParams(t *testing.T, recipient ethcommon.Address) TicketParams {
	recipientRandHash := RandHash()
	return TicketParams{
		Recipient:         recipient,
		FaceValue:         big.NewInt(0),
		WinProb:           big.NewInt(0),
		Seed:              big.NewInt(0),
		RecipientRandHash: recipientRandHash,
	}
}
