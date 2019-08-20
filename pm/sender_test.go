package pm

import (
	"math/big"
	"sync"
	"testing"

	"github.com/ethereum/go-ethereum/accounts"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestSenderEV_NonExistantSession_ReturnsError(t *testing.T) {
	sender := defaultSender(t)

	_, err := sender.EV("foo")
	assert.Contains(t, err.Error(), "error loading session")
}

func TestSenderEV(t *testing.T) {
	sender := defaultSender(t)

	assert := assert.New(t)

	ticketParams := defaultTicketParams(t, RandAddress())
	sessionID0 := sender.StartSession(ticketParams)
	ev, err := sender.EV(sessionID0)
	assert.Nil(err)
	assert.Zero(ticketEV(ticketParams.FaceValue, ticketParams.WinProb).Cmp(ev))

	ticketParams.FaceValue = big.NewInt(99)
	ticketParams.WinProb = big.NewInt(100)
	sessionID1 := sender.StartSession(ticketParams)
	ev, err = sender.EV(sessionID1)
	assert.Nil(err)
	assert.Zero(ticketEV(ticketParams.FaceValue, ticketParams.WinProb).Cmp(ev))
}

func TestCreateTicketBatch_NonExistantSession_ReturnsError(t *testing.T) {
	sender := defaultSender(t)

	_, err := sender.CreateTicketBatch("foo", 1)
	assert.Contains(t, err.Error(), "error loading session")
}

func TestCreateTicketBatch_GetSenderInfoError_ReturnsError(t *testing.T) {
	sender := defaultSender(t)
	sm := sender.senderManager.(*stubSenderManager)
	sm.err = errors.New("GetSenderInfo error")

	sessionID := sender.StartSession(defaultTicketParams(t, RandAddress()))
	_, err := sender.CreateTicketBatch(sessionID, 1)
	assert.EqualError(t, err, sm.err.Error())
}

func TestCreateTicketBatch_EVTooHigh_ReturnsError(t *testing.T) {
	// Test single ticket EV too high
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
	_, err := sender.CreateTicketBatch(sessionID, 1)
	assert.EqualError(t, err, "ticket EV higher than max EV")

	// Test multiple tickets EV too high
	sender.maxEV = big.NewRat(102, 1)

	_, err = sender.CreateTicketBatch(sessionID, 2)
	assert.EqualError(t, err, "ticket EV higher than max EV")

	// Check that EV is acceptable for a single ticket
	_, err = sender.CreateTicketBatch(sessionID, 1)
	assert.Nil(t, err)
}

func TestCreateTicketBatch_FaceValueTooHigh_ReturnsError(t *testing.T) {
	// Test single ticket faceValue too high
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
	_, err := sender.CreateTicketBatch(sessionID, 1)
	assert.EqualError(t, err, "ticket faceValue higher than max faceValue")

	// Test multiple tickets faceValue too high
	sender.depositMultiplier = 2
	sm.info.Deposit = big.NewInt(2224)

	_, err = sender.CreateTicketBatch(sessionID, 2)
	assert.EqualError(t, err, "ticket faceValue higher than max faceValue")

	// Check that faceValue is acceptable for a single ticket
	_, err = sender.CreateTicketBatch(sessionID, 1)
	assert.Nil(t, err)
}

func TestCreateTicketBatch_UsesSessionParamsInBatch(t *testing.T) {
	sender := defaultSender(t)
	rm := sender.roundsManager.(*stubRoundsManager)
	creationRound := rm.round.Int64()
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

	batch, err := sender.CreateTicketBatch(sessionID, 1)
	require.Nil(t, err)

	assert := assert.New(t)
	assert.Equal(senderAddress, batch.Sender)
	assert.Equal(recipient, batch.Recipient)
	assert.Equal(recipientRandHash, batch.RecipientRandHash)
	assert.Equal(ticketParams.FaceValue, batch.FaceValue)
	assert.Equal(ticketParams.WinProb, batch.WinProb)
	assert.Equal(creationRound, batch.CreationRound)
	assert.Equal(creationRoundBlkHash[:], batch.CreationRoundBlockHash.Bytes())
	assert.Equal(ticketParams.Seed, batch.Seed)
}

func TestCreateTicketBatch_SingleTicket(t *testing.T) {
	sender := defaultSender(t)
	am := sender.signer.(*stubSigner)
	am.signShouldFail = false
	am.saveSignRequest = true
	am.signResponse = RandBytes(42)
	ticketParams := TicketParams{
		Recipient:         RandAddress(),
		FaceValue:         big.NewInt(1111),
		WinProb:           big.NewInt(2222),
		Seed:              big.NewInt(3333),
		RecipientRandHash: RandHash(),
	}
	sessionID := sender.StartSession(ticketParams)

	batch, err := sender.CreateTicketBatch(sessionID, 1)
	require.Nil(t, err)

	assert := assert.New(t)
	assert.Equal(1, len(batch.SenderParams))
	assert.Equal(uint32(1), batch.SenderParams[0].SenderNonce)
	assert.Equal(am.signResponse, batch.SenderParams[0].Sig)
	assert.Equal(batch.Tickets()[0].Hash().Bytes(), am.signRequests[0])
}
func TestCreateTicketBatch_MultipleTickets(t *testing.T) {
	sender := defaultSender(t)
	am := sender.signer.(*stubSigner)
	am.signShouldFail = false
	am.saveSignRequest = true
	am.signResponse = RandBytes(42)
	ticketParams := TicketParams{
		Recipient:         RandAddress(),
		FaceValue:         big.NewInt(1111),
		WinProb:           big.NewInt(2222),
		Seed:              big.NewInt(3333),
		RecipientRandHash: RandHash(),
	}
	sessionID := sender.StartSession(ticketParams)

	batch, err := sender.CreateTicketBatch(sessionID, 4)
	require.Nil(t, err)

	assert := assert.New(t)
	assert.Equal(4, len(batch.SenderParams))

	tickets := batch.Tickets()

	for i := 0; i < 4; i++ {
		assert.Equal(uint32(i+1), batch.SenderParams[i].SenderNonce)
		assert.Equal(am.signResponse, batch.SenderParams[i].Sig)
		assert.Equal(tickets[i].Hash().Bytes(), am.signRequests[i])
	}
}

func TestCreateTicketBatch_SigningError_ReturnsError(t *testing.T) {
	sender := defaultSender(t)
	recipient := RandAddress()
	ticketParams := defaultTicketParams(t, recipient)
	sessionID := sender.StartSession(ticketParams)
	am := sender.signer.(*stubSigner)
	am.signShouldFail = true

	_, err := sender.CreateTicketBatch(sessionID, 1)
	assert.Contains(t, err.Error(), "error signing")
}

func TestCreateTicketBatch_ConcurrentCallsForSameSession_SenderNonceIncrementsCorrectly(t *testing.T) {
	totalBatches := 100
	lock := sync.RWMutex{}
	sender := defaultSender(t)
	ticketParams := defaultTicketParams(t, RandAddress())
	sessionID := sender.StartSession(ticketParams)

	var wg sync.WaitGroup
	wg.Add(totalBatches)
	var tickets []*Ticket
	for i := 0; i < totalBatches; i++ {
		go func() {
			batch, _ := sender.CreateTicketBatch(sessionID, 2)

			lock.Lock()
			tickets = append(tickets, batch.Tickets()...)
			lock.Unlock()

			wg.Done()
		}()
	}
	wg.Wait()

	totalTickets := totalBatches * 2

	assert := assert.New(t)

	sessionUntyped, ok := sender.sessions.Load(sessionID)
	require.True(t, ok)

	session := sessionUntyped.(*session)
	assert.Equal(uint32(totalTickets), session.senderNonce)

	uniqueNonces := make(map[uint32]bool)
	for _, ticket := range tickets {
		uniqueNonces[ticket.SenderNonce] = true
	}

	assert.Equal(totalTickets, len(uniqueNonces))
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
