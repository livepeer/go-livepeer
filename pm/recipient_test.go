package pm

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"math/big"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/golang/glog"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newRecipientFixtureOrFatal(t *testing.T) (ethcommon.Address, *stubBroker, *stubValidator, *stubTicketStore, *stubGasPriceMonitor, *stubSenderMonitor, *stubErrorMonitor, TicketParamsConfig, []byte) {
	sender := RandAddress()

	b := newStubBroker()

	v := &stubValidator{}
	v.SetIsValidTicket(true)

	gm := &stubGasPriceMonitor{gasPrice: big.NewInt(100)}
	sm := newStubSenderMonitor()
	sm.maxFloat = big.NewInt(10000000000)
	em := &stubErrorMonitor{}
	cfg := TicketParamsConfig{
		EV:               big.NewInt(5),
		RedeemGas:        10000,
		TxCostMultiplier: 100,
	}

	return sender, b, v, newStubTicketStore(), gm, sm, em, cfg, []byte("foo")
}

func newRecipientOrFatal(t *testing.T, addr ethcommon.Address, b Broker, v Validator, ts TicketStore, gpm GasPriceMonitor, sm SenderMonitor, em ErrorMonitor, cfg TicketParamsConfig) Recipient {
	r, err := NewRecipient(addr, b, v, ts, gpm, sm, em, cfg)
	if err != nil {
		t.Fatal(err)
	}

	return r
}

func ticketParamsOrFatal(t *testing.T, r Recipient, sender ethcommon.Address) *TicketParams {
	params, err := r.TicketParams(sender)
	if err != nil {
		t.Fatal(err)
	}

	return params
}

func newTicket(sender ethcommon.Address, params *TicketParams, senderNonce uint32) *Ticket {
	return &Ticket{
		Recipient:         ethcommon.Address{},
		Sender:            sender,
		FaceValue:         params.FaceValue,
		WinProb:           params.WinProb,
		SenderNonce:       senderNonce,
		RecipientRandHash: params.RecipientRandHash,
	}
}

func genRecipientRand(sender ethcommon.Address, secret [32]byte, seed *big.Int) *big.Int {
	h := hmac.New(sha256.New, secret[:])
	h.Write(append(seed.Bytes(), sender.Bytes()...))
	return new(big.Int).SetBytes(h.Sum(nil))
}

func TestReceiveTicket_InvalidFaceValue(t *testing.T) {
	sender, b, v, ts, gm, sm, em, cfg, sig := newRecipientFixtureOrFatal(t)
	r := newRecipientOrFatal(t, RandAddress(), b, v, ts, gm, sm, em, cfg)
	params, err := r.TicketParams(sender)
	require.Nil(t, err)

	// Test invalid faceValue
	ticket := newTicket(sender, params, 0)
	ticket.FaceValue = big.NewInt(0) // Using invalid faceValue

	sessionID, won, err := r.ReceiveTicket(ticket, sig, params.Seed)
	if err == nil {
		t.Error("expected invalid faceValue error")
	}
	if err != nil && !strings.Contains(err.Error(), "invalid ticket faceValue") {
		t.Errorf("expected invalid faceValue error, got %v", err)
	}
	if sessionID != "" {
		t.Errorf("expected sessionID , got %v", sessionID)
	}
	if won {
		t.Errorf("expected non-winning ticket")
	}

	// Test invalid faceValue when ticket is winning
	v.SetIsWinningTicket(true)

	ticket = newTicket(sender, params, 1)
	ticket.FaceValue = big.NewInt(0) // Using invalid faceValue

	sessionID, won, err = r.ReceiveTicket(ticket, sig, params.Seed)
	if err == nil {
		t.Error("expected invalid faceValue error")
	}
	if err != nil && !strings.Contains(err.Error(), "invalid ticket faceValue") {
		t.Errorf("expected invalid faceValue error, got %v", err)
	}
	if sessionID != ticket.RecipientRandHash.Hex() {
		t.Errorf("expected sessionID %v, got %v", ticket.RecipientRandHash.Hex(), sessionID)
	}
	if !won {
		t.Errorf("expected winning ticket")
	}
}

func TestReceiveTicket_InvalidFaceValue_AcceptableError(t *testing.T) {
	sender, b, v, ts, gm, sm, em, cfg, sig := newRecipientFixtureOrFatal(t)
	r := newRecipientOrFatal(t, RandAddress(), b, v, ts, gm, sm, em, cfg)
	params, err := r.TicketParams(sender)
	require.Nil(t, err)

	assert := assert.New(t)

	// Test unacceptable error
	em.acceptable = false

	ticket := newTicket(sender, params, 0)
	ticket.FaceValue = big.NewInt(0) // Using invalid faceValue

	_, _, err = r.ReceiveTicket(ticket, sig, params.Seed)
	assert.NotNil(err)
	assert.Contains(err.Error(), "invalid ticket faceValue")

	rerr, ok := err.(acceptableError)
	assert.True(ok)
	assert.False(rerr.Acceptable())

	// Test acceptable error

	em.acceptable = true
	ticket = newTicket(sender, params, 1)
	ticket.FaceValue = big.NewInt(0) // Using invalid faceValue

	_, _, err = r.ReceiveTicket(ticket, sig, params.Seed)
	assert.NotNil(err)
	assert.Contains(err.Error(), "invalid ticket faceValue")

	rerr, ok = err.(acceptableError)
	assert.True(ok)
	assert.True(rerr.Acceptable())

}

func TestReceiveTicket_InvalidFaceValue_GasPriceChange(t *testing.T) {
	sender, b, v, ts, gm, sm, em, cfg, sig := newRecipientFixtureOrFatal(t)
	r := newRecipientOrFatal(t, RandAddress(), b, v, ts, gm, sm, em, cfg)
	params, err := r.TicketParams(sender)
	require.Nil(t, err)

	// Test invalid faceValue
	ticket := newTicket(sender, params, 0)
	// Change the gas price so that the expected face value is different
	gm.gasPrice = big.NewInt(99999999)

	_, _, err = r.ReceiveTicket(ticket, sig, params.Seed)
	if err == nil {
		t.Error("expected invalid faceValue error")
	}
	if err != nil && !strings.Contains(err.Error(), "invalid ticket faceValue") {
		t.Errorf("expected invalid faceValue error, got %v", err)
	}
}

func TestReceiveTicket_InvalidWinProb(t *testing.T) {
	sender, b, v, ts, gm, sm, em, cfg, sig := newRecipientFixtureOrFatal(t)
	r := newRecipientOrFatal(t, RandAddress(), b, v, ts, gm, sm, em, cfg)
	params, err := r.TicketParams(sender)
	require.Nil(t, err)

	// Test invalid winProb
	ticket := newTicket(sender, params, 0)
	ticket.WinProb = big.NewInt(0) // Using invalid winProb

	sessionID, won, err := r.ReceiveTicket(ticket, sig, params.Seed)
	if err == nil {
		t.Error("expected invalid winProb error")
	}
	if err != nil && !strings.Contains(err.Error(), "invalid ticket winProb") {
		t.Errorf("expected invalid winProb error, got %v", err)
	}
	if sessionID != "" {
		t.Errorf("expected sessionID , got %v", sessionID)
	}
	if won {
		t.Errorf("expected non-winning ticket")
	}

	// Test invalid winProb when ticket is winning
	v.SetIsWinningTicket(true)

	ticket = newTicket(sender, params, 1)
	ticket.WinProb = big.NewInt(0) // Using invalid faceValue

	sessionID, won, err = r.ReceiveTicket(ticket, sig, params.Seed)
	if err == nil {
		t.Error("expected invalid winProb error")
	}
	if err != nil && !strings.Contains(err.Error(), "invalid ticket winProb") {
		t.Errorf("expected invalid winProb error, got %v", err)
	}
	if sessionID != ticket.RecipientRandHash.Hex() {
		t.Errorf("expected sessionID %v, got %v", ticket.RecipientRandHash.Hex(), sessionID)
	}
	if !won {
		t.Errorf("expected winning ticket")
	}
}

func TestReceiveTicket_InvalidWinProb_AcceptableError(t *testing.T) {
	sender, b, v, ts, gm, sm, em, cfg, sig := newRecipientFixtureOrFatal(t)
	r := newRecipientOrFatal(t, RandAddress(), b, v, ts, gm, sm, em, cfg)
	params, err := r.TicketParams(sender)
	require.Nil(t, err)

	assert := assert.New(t)

	// Test unacceptable error
	em.acceptable = false
	ticket := newTicket(sender, params, 0)
	ticket.WinProb = big.NewInt(0) // Using invalid winProb

	_, _, err = r.ReceiveTicket(ticket, sig, params.Seed)
	assert.NotNil(err)
	assert.Contains(err.Error(), "invalid ticket winProb")

	rerr, ok := err.(acceptableError)
	assert.True(ok)
	assert.False(rerr.Acceptable())

	// Test acceptable error

	em.acceptable = true
	ticket = newTicket(sender, params, 1)
	ticket.WinProb = big.NewInt(0) // Using invalid winProb

	_, _, err = r.ReceiveTicket(ticket, sig, params.Seed)
	assert.NotNil(err)
	assert.Contains(err.Error(), "invalid ticket winProb")

	rerr, ok = err.(acceptableError)
	assert.True(ok)
	assert.True(rerr.Acceptable())

}

func TestReceiveTicket_InvalidSender(t *testing.T) {
	assert := assert.New(t)
	sender, b, v, ts, gm, sm, em, cfg, sig := newRecipientFixtureOrFatal(t)
	sm.validateSenderErr = errors.New("Invalid Sender")
	r := newRecipientOrFatal(t, RandAddress(), b, v, ts, gm, sm, em, cfg)

	params, err := r.TicketParams(sender)
	require.Nil(t, err)

	// Test valid non-winning ticket
	newSenderNonce := uint32(3)
	ticket := newTicket(sender, params, newSenderNonce)

	_, _, err = r.ReceiveTicket(ticket, sig, params.Seed)
	assert.EqualError(err, "Invalid Sender")
}

func TestReceiveTicket_InvalidTicket(t *testing.T) {
	sender, b, v, ts, gm, sm, em, cfg, sig := newRecipientFixtureOrFatal(t)
	r := newRecipientOrFatal(t, RandAddress(), b, v, ts, gm, sm, em, cfg)
	params, err := r.TicketParams(sender)
	require.Nil(t, err)

	// Test invalid ticket
	ticket := newTicket(sender, params, 0)

	// Config stub validator with invalid non-winning tickets
	v.SetIsValidTicket(false)

	_, _, err = r.ReceiveTicket(ticket, sig, params.Seed)
	if err == nil {
		t.Error("expected validator invalid ticket error")
	}
	if err != nil && !strings.Contains(err.Error(), "validator invalid ticket error") {
		t.Errorf("expected validator invalid ticket error, got %v", err)
	}
}

func TestReceiveTicket_ValidNonWinningTicket(t *testing.T) {
	sender, b, v, ts, gm, sm, em, cfg, sig := newRecipientFixtureOrFatal(t)
	secret := [32]byte{3}
	r := NewRecipientWithSecret(RandAddress(), b, v, ts, gm, sm, em, secret, cfg)
	params, err := r.TicketParams(sender)
	require.Nil(t, err)

	// Test valid non-winning ticket
	newSenderNonce := uint32(3)
	ticket := newTicket(sender, params, newSenderNonce)

	sessionID, won, err := r.ReceiveTicket(ticket, sig, params.Seed)
	if err != nil {
		t.Error(err)
	}
	if won {
		t.Errorf("expected valid non-winning ticket")
	}
	if sessionID != "" {
		t.Errorf("expected empty sessionID for valid non-winning ticket")
	}

	recipientRand := genRecipientRand(sender, secret, params.Seed)
	senderNonce := r.(*recipient).senderNonces[recipientRand.String()]

	if senderNonce != newSenderNonce {
		t.Errorf("expected senderNonce to be %d, got %d", newSenderNonce, senderNonce)
	}
}

func TestReceiveTicket_ValidWinningTicket(t *testing.T) {
	sender, b, v, ts, gm, sm, em, cfg, sig := newRecipientFixtureOrFatal(t)
	secret := [32]byte{3}
	r := NewRecipientWithSecret(RandAddress(), b, v, ts, gm, sm, em, secret, cfg)
	params, err := r.TicketParams(sender)
	require.Nil(t, err)

	// Test valid winning ticket
	newSenderNonce := uint32(3)
	ticket := newTicket(sender, params, newSenderNonce)

	// Config stub validator with valid winning tickets
	v.SetIsWinningTicket(true)

	sessionID, won, err := r.ReceiveTicket(ticket, sig, params.Seed)
	if err != nil {
		t.Error(err)
	}
	if !won {
		t.Errorf("expected valid winning ticket")
	}
	if sessionID != ticket.RecipientRandHash.Hex() {
		t.Errorf("expected sessionID %s, got %s", ticket.RecipientRandHash.Hex(), sessionID)
	}

	recipientRand := genRecipientRand(sender, secret, params.Seed)
	senderNonce := r.(*recipient).senderNonces[recipientRand.String()]

	if senderNonce != newSenderNonce {
		t.Errorf("expected senderNonce to be %d, got %d", newSenderNonce, senderNonce)
	}

	storeTickets, storeSigs, storeRecipientRands, err := ts.LoadWinningTickets([]string{ticket.RecipientRandHash.Hex()})
	if err != nil {
		t.Fatal(err)
	}

	if len(storeTickets) != 1 {
		t.Errorf("expected 1 stored tickets, got %d", len(storeTickets))
	}
	if len(storeSigs) != 1 {
		t.Errorf("expected 1 stored sigs, got %d", len(storeSigs))
	}
	if len(storeRecipientRands) != 1 {
		t.Errorf("expected 1 stored recipientRands, got %d", len(storeRecipientRands))
	}

	if storeTickets[0].Hash() != ticket.Hash() {
		t.Errorf("expected store ticket hash %v, got %v", ticket.Hash(), storeTickets[0].Hash())
	}

	if !bytes.Equal(storeSigs[0], sig) {
		t.Errorf("expected store sig 0x%x, got 0x%x", sig, storeSigs[0])
	}

	if crypto.Keccak256Hash(ethcommon.LeftPadBytes(storeRecipientRands[0].Bytes(), uint256Size)) != ticket.RecipientRandHash {
		t.Error("expected store recipientRand to match ticket recipientRandHash")
	}
}

func TestReceiveTicket_ValidWinningTicket_StoreError(t *testing.T) {
	sender, b, v, ts, gm, sm, em, cfg, sig := newRecipientFixtureOrFatal(t)
	secret := [32]byte{3}
	r := NewRecipientWithSecret(RandAddress(), b, v, ts, gm, sm, em, secret, cfg)
	params, err := r.TicketParams(sender)
	require.Nil(t, err)

	// Test valid winning ticket
	newSenderNonce := uint32(3)
	ticket := newTicket(sender, params, newSenderNonce)

	// Config stub validator with valid winning tickets
	v.SetIsWinningTicket(true)
	// Config stub ticket store to fail store
	ts.storeShouldFail = true

	errorLogsBefore := glog.Stats.Error.Lines()

	_, _, err = r.ReceiveTicket(ticket, sig, params.Seed)

	errorLogsAfter := glog.Stats.Error.Lines()

	assert := assert.New(t)
	assert.Nil(err)
	assert.Equal(int64(1), errorLogsAfter-errorLogsBefore)

	recipientRand := genRecipientRand(sender, secret, params.Seed)
	senderNonce := r.(*recipient).senderNonces[recipientRand.String()]

	if senderNonce != newSenderNonce {
		t.Errorf("expected senderNonce to be %d, got %d", newSenderNonce, senderNonce)
	}

	storeTickets, storeSigs, storeRecipientRands, err := ts.LoadWinningTickets([]string{ticket.RecipientRandHash.Hex()})
	if err != nil {
		t.Fatal(err)
	}

	if len(storeTickets) != 0 {
		t.Errorf("expected 0 stored tickets, got %d", len(storeTickets))
	}
	if len(storeSigs) != 0 {
		t.Errorf("expected 0 stored sigs, got %d", len(storeSigs))
	}
	if len(storeRecipientRands) != 0 {
		t.Errorf("expected 0 stored recipientRands, got %d", len(storeRecipientRands))
	}
}

func TestReceiveTicket_InvalidRecipientRand_AlreadyRevealed(t *testing.T) {
	sender, b, v, ts, gm, sm, em, cfg, sig := newRecipientFixtureOrFatal(t)
	r := newRecipientOrFatal(t, RandAddress(), b, v, ts, gm, sm, em, cfg)
	params, err := r.TicketParams(sender)
	require.Nil(t, err)

	// Test invalid recipientRand revealed
	ticket := newTicket(sender, params, 0)

	// Config stub validator with valid winning tickets
	v.SetIsWinningTicket(true)

	_, _, err = r.ReceiveTicket(ticket, sig, params.Seed)
	if err != nil {
		t.Fatal(err)
	}

	// Redeem ticket to invalidate recipientRand
	if err := r.RedeemWinningTickets([]string{ticket.RecipientRandHash.Hex()}); err != nil {
		t.Fatal(err)
	}

	// New ticket with same invalid recipientRand, but updated senderNonce
	ticket = newTicket(sender, params, 1)

	sessionID, won, err := r.ReceiveTicket(ticket, sig, params.Seed)
	if err == nil {
		t.Error("expected invalid recipientRand revealed error")
	}
	if err != nil && !strings.Contains(err.Error(), "invalid already revealed recipientRand") {
		t.Errorf("expected invalid recipientRand revealed error, got %v", err)
	}
	if sessionID != ticket.RecipientRandHash.Hex() {
		t.Errorf("expected sessionID %v, got %v", ticket.RecipientRandHash.Hex(), sessionID)
	}
	if !won {
		t.Errorf("expected winning ticket")
	}
}

func TestReceiveTicket_InvalidRecipientRand_AlreadyRevealed_AcceptableError(t *testing.T) {
	sender, b, v, ts, gm, sm, em, cfg, sig := newRecipientFixtureOrFatal(t)
	r := newRecipientOrFatal(t, RandAddress(), b, v, ts, gm, sm, em, cfg)
	params, err := r.TicketParams(sender)
	require.Nil(t, err)

	ticket := newTicket(sender, params, 0)

	// Config stub validator with valid winning tickets
	v.SetIsWinningTicket(true)

	_, _, err = r.ReceiveTicket(ticket, sig, params.Seed)
	require.Nil(t, err)

	// Redeem ticket to invalidate recipientRand
	err = r.RedeemWinningTickets([]string{ticket.RecipientRandHash.Hex()})
	require.Nil(t, err)

	// New ticket with same invalid recipientRand, but updated senderNonce
	ticket = newTicket(sender, params, 1)

	assert := assert.New(t)

	// Test unacceptable error
	em.acceptable = false
	_, _, err = r.ReceiveTicket(ticket, sig, params.Seed)
	assert.NotNil(err)
	assert.Contains(err.Error(), "invalid already revealed recipientRand")

	rerr, ok := err.(acceptableError)
	assert.True(ok)
	assert.False(rerr.Acceptable())

	// Test acceptable error

	em.acceptable = true

	_, _, err = r.ReceiveTicket(ticket, sig, params.Seed)
	assert.NotNil(err)
	assert.Contains(err.Error(), "invalid already revealed recipientRand")
	rerr, ok = err.(acceptableError)
	assert.True(ok)
	assert.True(rerr.Acceptable())
}

func TestReceiveTicket_InvalidSenderNonce(t *testing.T) {
	sender, b, v, ts, gm, sm, em, cfg, sig := newRecipientFixtureOrFatal(t)
	r := newRecipientOrFatal(t, RandAddress(), b, v, ts, gm, sm, em, cfg)
	params, err := r.TicketParams(sender)
	require.Nil(t, err)

	// Test invalid senderNonce
	// Receive senderNonce = 0
	ticket0 := newTicket(sender, params, 0)

	_, _, err = r.ReceiveTicket(ticket0, sig, params.Seed)
	if err != nil {
		t.Fatal(err)
	}

	// Receive senderNonce = 1
	ticket1 := newTicket(sender, params, 1)

	_, _, err = r.ReceiveTicket(ticket1, sig, params.Seed)
	if err != nil {
		t.Fatal(err)
	}

	// Replay senderNonce = 1 (new nonce = highest seen nonce)
	_, _, err = r.ReceiveTicket(ticket1, sig, params.Seed)
	if err == nil {
		t.Error("expected invalid senderNonce (new nonce = highest seen nonce) error")
	}
	if err != nil && !strings.Contains(err.Error(), "invalid ticket senderNonce") {
		t.Errorf("expected invalid senderNonce (new nonce = highest seen nonce) error, got %v", err)
	}

	// Replay senderNonce = 0 (new nonce < highest seen nonce)
	_, _, err = r.ReceiveTicket(ticket0, sig, params.Seed)
	if err == nil {
		t.Error("expected invalid senderNonce (new nonce < highest seen nonce) error")
	}
	if err != nil && !strings.Contains(err.Error(), "invalid ticket senderNonce") {
		t.Errorf("expected invalid senderNonce (new nonce < highest seen nonce) error, got %v", err)
	}
}

func TestReceiveTicket_ValidNonWinningTicket_Concurrent(t *testing.T) {
	sender, b, v, ts, gm, sm, em, cfg, sig := newRecipientFixtureOrFatal(t)
	r := newRecipientOrFatal(t, RandAddress(), b, v, ts, gm, sm, em, cfg)
	params, err := r.TicketParams(sender)
	require.Nil(t, err)

	var wg sync.WaitGroup
	var errCount uint64

	for i := 0; i < 50; i++ {
		wg.Add(1)

		go func(senderNonce uint32) {
			defer wg.Done()

			ticket := newTicket(sender, params, senderNonce)

			_, _, err := r.ReceiveTicket(ticket, sig, params.Seed)
			if err != nil {
				atomic.AddUint64(&errCount, 1)
			}
		}(uint32(i))
	}

	wg.Wait()

	if errCount == 0 {
		t.Error("expected more than zero senderNonce errors for concurrent ticket receipt")
	}
}

func TestRedeemWinningTickets_InvalidSessionID(t *testing.T) {
	_, b, v, ts, gm, sm, em, cfg, _ := newRecipientFixtureOrFatal(t)
	r := newRecipientOrFatal(t, RandAddress(), b, v, ts, gm, sm, em, cfg)

	// Config stub ticket store to fail load
	ts.loadShouldFail = true

	err := r.RedeemWinningTickets([]string{"foo"})
	if err == nil {
		t.Error("expected ticket store load error")
	}
	if err != nil && !strings.Contains(err.Error(), "ticket store load error") {
		t.Errorf("expected ticket store load error, got %v", err)
	}
}

func TestRedeemWinningTickets_SingleTicket_ZeroMaxFloat(t *testing.T) {
	sender, b, v, ts, gm, sm, em, cfg, sig := newRecipientFixtureOrFatal(t)
	r := newRecipientOrFatal(t, RandAddress(), b, v, ts, gm, sm, em, cfg)
	params, err := r.TicketParams(sender)
	require.Nil(t, err)

	// Config stub validator with valid winning tickets
	v.SetIsWinningTicket(true)

	// Test zero maxfloat error
	ticket := newTicket(sender, params, 0)

	sessionID, won, err := r.ReceiveTicket(ticket, sig, params.Seed)
	if err != nil {
		t.Fatal(err)
	}
	if !won {
		t.Fatal("expected valid winning ticket")
	}

	sm.maxFloat = big.NewInt(0)
	err = r.RedeemWinningTickets([]string{sessionID})
	if err == nil {
		t.Error("expected zero max float error")
	}
	if err != nil && !strings.Contains(err.Error(), "max float is zero") {
		t.Errorf("expected zero max float error, got %v", err)
	}
}

func TestRedeemWinningTickets_SingleTicket_RedeemError(t *testing.T) {
	sender, b, v, ts, gm, sm, em, cfg, sig := newRecipientFixtureOrFatal(t)
	secret := [32]byte{3}
	r := NewRecipientWithSecret(RandAddress(), b, v, ts, gm, sm, em, secret, cfg)
	params, err := r.TicketParams(sender)
	require.Nil(t, err)

	// Config stub validator with valid winning tickets
	v.SetIsWinningTicket(true)

	// Test redeem error
	ticket := newTicket(sender, params, 2)

	sessionID, won, err := r.ReceiveTicket(ticket, sig, params.Seed)
	if err != nil {
		t.Fatal(err)
	}
	if !won {
		t.Fatal("expected valid winning ticket")
	}

	// Config stub broker to fail redeem
	b.redeemShouldFail = true

	err = r.RedeemWinningTickets([]string{sessionID})
	if err == nil {
		t.Error("expected broker redeem error")
	}
	if err != nil && !strings.Contains(err.Error(), "broker redeem error") {
		t.Errorf("expected broker redeem error, got %v", err)
	}

	used, err := b.IsUsedTicket(ticket)
	if err != nil {
		t.Fatal(err)
	}
	if used {
		t.Error("expected non-used ticket")
	}

	recipientRand := genRecipientRand(sender, secret, params.Seed)

	if _, ok := r.(*recipient).invalidRands.Load(recipientRand.String()); ok {
		t.Error("expected not to invalidate recipientRand")
	}

	if _, ok := r.(*recipient).senderNonces[recipientRand.String()]; !ok {
		t.Error("expected not to clear senderNonce memory")
	}
}

func TestRedeemWinningTickets_SingleTicket_CheckTxError(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	sender, b, v, ts, gm, sm, em, cfg, sig := newRecipientFixtureOrFatal(t)
	secret := [32]byte{3}
	r := NewRecipientWithSecret(RandAddress(), b, v, ts, gm, sm, em, secret, cfg)
	params, err := r.TicketParams(sender)
	require.Nil(err)

	// Config stub validator with valid winning tickets
	v.SetIsWinningTicket(true)

	ticket := newTicket(sender, params, 2)

	sessionID, won, err := r.ReceiveTicket(ticket, sig, params.Seed)
	require.Nil(err)
	require.True(won)

	// Config stub broker to fail CheckTx
	b.checkTxErr = errors.New("CheckTx error")

	err = r.RedeemWinningTickets([]string{sessionID})
	assert.EqualError(err, b.checkTxErr.Error())
}

func TestRedeemWinningTickets_SingleTicket(t *testing.T) {
	sender, b, v, ts, gm, sm, em, cfg, sig := newRecipientFixtureOrFatal(t)
	secret := [32]byte{3}
	r := NewRecipientWithSecret(RandAddress(), b, v, ts, gm, sm, em, secret, cfg)
	params, err := r.TicketParams(sender)
	require.Nil(t, err)

	// Config stub validator with valid winning tickets
	v.SetIsWinningTicket(true)

	// Test single ticket
	ticket := newTicket(sender, params, 2)

	sessionID, won, err := r.ReceiveTicket(ticket, sig, params.Seed)
	if err != nil {
		t.Fatal(err)
	}
	if !won {
		t.Fatal("expected valid winning ticket")
	}

	err = r.RedeemWinningTickets([]string{sessionID})
	if err != nil {
		t.Error(err)
	}

	used, err := b.IsUsedTicket(ticket)
	if err != nil {
		t.Fatal(err)
	}
	if !used {
		t.Error("expected used ticket")
	}

	recipientRand := genRecipientRand(sender, secret, params.Seed)

	if _, ok := r.(*recipient).invalidRands.Load(recipientRand.String()); !ok {
		t.Error("expected to invalidate recipientRand")
	}

	if _, ok := r.(*recipient).senderNonces[recipientRand.String()]; ok {
		t.Error("expected to clear senderNonce memory")
	}
}

func TestRedeemWinningTickets_MultipleTickets(t *testing.T) {
	sender, b, v, ts, gm, sm, em, cfg, sig := newRecipientFixtureOrFatal(t)
	secret := [32]byte{3}
	r := NewRecipientWithSecret(RandAddress(), b, v, ts, gm, sm, em, secret, cfg)
	params, err := r.TicketParams(sender)
	require.Nil(t, err)

	// Config stub validator with valid winning tickets
	v.SetIsWinningTicket(true)

	// Test multiple tickets
	// Receive ticket 0
	ticket0 := newTicket(sender, params, 2)

	sessionID, won, err := r.ReceiveTicket(ticket0, sig, params.Seed)
	if err != nil {
		t.Fatal(err)
	}
	if !won {
		t.Fatal("expected valid winning ticket")
	}

	// Receive ticket 1
	ticket1 := newTicket(sender, params, 3)

	_, won, err = r.ReceiveTicket(ticket1, sig, params.Seed)
	if err != nil {
		t.Fatal(err)
	}
	if !won {
		t.Fatal("expected valid winning ticket")
	}

	err = r.RedeemWinningTickets([]string{sessionID})
	if err != nil {
		t.Error(err)
	}

	used, err := b.IsUsedTicket(ticket0)
	if err != nil {
		t.Fatal(err)
	}
	if !used {
		t.Error("expected used ticket")
	}

	used, err = b.IsUsedTicket(ticket1)
	if err != nil {
		t.Fatal(err)
	}
	if !used {
		t.Error("expected used ticket")
	}

	recipientRand := genRecipientRand(sender, secret, params.Seed)

	if _, ok := r.(*recipient).invalidRands.Load(recipientRand.String()); !ok {
		t.Error("expected to invalidate recipientRand")
	}

	if _, ok := r.(*recipient).senderNonces[recipientRand.String()]; ok {
		t.Error("expected to clear senderNonce memory")
	}
}

func TestRedeemWinningTickets_MultipleTicketsFromMultipleSessions(t *testing.T) {
	sender, b, v, ts, gm, sm, em, cfg, sig := newRecipientFixtureOrFatal(t)
	secret := [32]byte{3}
	r := NewRecipientWithSecret(RandAddress(), b, v, ts, gm, sm, em, secret, cfg)
	// Config stub validator with valid winning tickets
	v.SetIsWinningTicket(true)
	require := require.New(t)

	params0 := ticketParamsOrFatal(t, r, sender)
	ticket0 := newTicket(sender, params0, 1)
	sessionID0, won, err := r.ReceiveTicket(ticket0, sig, params0.Seed)
	require.Nil(err)
	require.True(won)

	params1 := ticketParamsOrFatal(t, r, sender)
	ticket1 := newTicket(sender, params1, 1)
	sessionID1, won, err := r.ReceiveTicket(ticket1, sig, params1.Seed)
	require.Nil(err)
	require.True(won)

	require.NotEqual(sessionID0, sessionID1)

	err = r.RedeemWinningTickets([]string{sessionID0, sessionID1})

	assert := assert.New(t)
	assert.Nil(err)
	used, err := b.IsUsedTicket(ticket0)
	require.Nil(err)
	assert.True(used)
	used, err = b.IsUsedTicket(ticket1)
	require.Nil(err)
	assert.True(used)

	recipientRand0 := genRecipientRand(sender, secret, params0.Seed)
	recipientRand1 := genRecipientRand(sender, secret, params1.Seed)

	_, ok := r.(*recipient).invalidRands.Load(recipientRand0.String())
	assert.True(ok)
	_, ok = r.(*recipient).invalidRands.Load(recipientRand1.String())
	assert.True(ok)

	_, ok = r.(*recipient).senderNonces[recipientRand0.String()]
	assert.False(ok)
	_, ok = r.(*recipient).senderNonces[recipientRand1.String()]
	assert.False(ok)
}

func TestRedeemWinningTicket_MaxFloatError(t *testing.T) {
	assert := assert.New(t)

	sender, b, v, ts, gm, sm, em, cfg, sig := newRecipientFixtureOrFatal(t)
	secret := [32]byte{3}
	r := NewRecipientWithSecret(RandAddress(), b, v, ts, gm, sm, em, secret, cfg)

	params := ticketParamsOrFatal(t, r, sender)
	ticket := newTicket(sender, params, 1)

	sm.maxFloatErr = errors.New("MaxFloat error")
	err := r.RedeemWinningTicket(ticket, sig, params.Seed)
	assert.EqualError(err, sm.maxFloatErr.Error())
}

func TestRedeemWinningTicket_InsufficientMaxFloat_QueueTicket(t *testing.T) {
	assert := assert.New(t)

	sender, b, v, ts, gm, sm, em, cfg, sig := newRecipientFixtureOrFatal(t)
	secret := [32]byte{3}
	r := NewRecipientWithSecret(RandAddress(), b, v, ts, gm, sm, em, secret, cfg)

	params := ticketParamsOrFatal(t, r, sender)
	ticket := newTicket(sender, params, 1)
	ticket.FaceValue = big.NewInt(99999999999999)

	err := r.RedeemWinningTicket(ticket, sig, params.Seed)
	assert.Nil(err)

	recipientRand := genRecipientRand(sender, secret, params.Seed)
	assert.Equal(1, len(sm.queued))
	assert.Equal(&SignedTicket{ticket, sig, recipientRand}, sm.queued[0])
}

func TestRedeemWinningTicket_AddFloatError(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	sender, b, v, ts, gm, sm, em, cfg, sig := newRecipientFixtureOrFatal(t)
	secret := [32]byte{3}
	r := NewRecipientWithSecret(RandAddress(), b, v, ts, gm, sm, em, secret, cfg)

	params := ticketParamsOrFatal(t, r, sender)
	ticket := newTicket(sender, params, 1)

	_, _, err := r.ReceiveTicket(ticket, sig, params.Seed)
	require.Nil(err)

	errorLogsBefore := glog.Stats.Error.Lines()

	sm.addFloatErr = errors.New("AddFloat error")
	err = r.RedeemWinningTicket(ticket, sig, params.Seed)
	assert.Nil(err)

	errorLogsAfter := glog.Stats.Error.Lines()

	// Check that an error was logged
	assert.Equal(int64(1), errorLogsAfter-errorLogsBefore)

	used, err := b.IsUsedTicket(ticket)
	require.Nil(err)
	assert.True(used)

	recipientRand := genRecipientRand(sender, secret, params.Seed)

	_, ok := r.(*recipient).invalidRands.Load(recipientRand.String())
	assert.True(ok)

	_, ok = r.(*recipient).senderNonces[recipientRand.String()]
	assert.False(ok)
}

func TestRedeemWinningTicket(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	sender, b, v, ts, gm, sm, em, cfg, sig := newRecipientFixtureOrFatal(t)
	secret := [32]byte{3}
	r := NewRecipientWithSecret(RandAddress(), b, v, ts, gm, sm, em, secret, cfg)

	params := ticketParamsOrFatal(t, r, sender)
	ticket := newTicket(sender, params, 1)

	_, _, err := r.ReceiveTicket(ticket, sig, params.Seed)
	require.Nil(err)

	errorLogsBefore := glog.Stats.Error.Lines()

	err = r.RedeemWinningTicket(ticket, sig, params.Seed)
	assert.Nil(err)

	errorLogsAfter := glog.Stats.Error.Lines()

	// Check that no errors were logged
	assert.Zero(errorLogsAfter - errorLogsBefore)

	used, err := b.IsUsedTicket(ticket)
	require.Nil(err)
	assert.True(used)

	recipientRand := genRecipientRand(sender, secret, params.Seed)

	_, ok := r.(*recipient).invalidRands.Load(recipientRand.String())
	assert.True(ok)

	_, ok = r.(*recipient).senderNonces[recipientRand.String()]
	assert.False(ok)
}

func TestRedeemManager_Error(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	sender, b, v, ts, gm, sm, em, cfg, sig := newRecipientFixtureOrFatal(t)
	secret := [32]byte{3}
	r := NewRecipientWithSecret(RandAddress(), b, v, ts, gm, sm, em, secret, cfg)
	r.Start()
	defer r.Stop()

	params := ticketParamsOrFatal(t, r, sender)
	ticket := newTicket(sender, params, 1)
	recipientRand := genRecipientRand(sender, secret, params.Seed)

	_, _, err := r.ReceiveTicket(ticket, sig, params.Seed)
	require.Nil(err)

	errorLogsBefore := glog.Stats.Error.Lines()

	sm.maxFloatErr = errors.New("MaxFloat error")
	sm.redeemable <- &SignedTicket{ticket, sig, recipientRand}

	time.Sleep(time.Millisecond * 20)
	errorLogsAfter := glog.Stats.Error.Lines()

	// Check that an error was logged
	assert.Equal(int64(1), errorLogsAfter-errorLogsBefore)

	used, err := b.IsUsedTicket(ticket)
	require.Nil(err)
	assert.False(used)

	_, ok := r.(*recipient).invalidRands.Load(recipientRand.String())
	assert.False(ok)

	_, ok = r.(*recipient).senderNonces[recipientRand.String()]
	assert.True(ok)
}

func TestRedeemManager(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	sender, b, v, ts, gm, sm, em, cfg, sig := newRecipientFixtureOrFatal(t)
	secret := [32]byte{3}
	r := NewRecipientWithSecret(RandAddress(), b, v, ts, gm, sm, em, secret, cfg)
	r.Start()
	defer r.Stop()

	params := ticketParamsOrFatal(t, r, sender)
	ticket := newTicket(sender, params, 1)
	recipientRand := genRecipientRand(sender, secret, params.Seed)

	_, _, err := r.ReceiveTicket(ticket, sig, params.Seed)
	require.Nil(err)

	errorLogsBefore := glog.Stats.Error.Lines()

	sm.redeemable <- &SignedTicket{ticket, sig, recipientRand}

	time.Sleep(time.Millisecond * 20)
	errorLogsAfter := glog.Stats.Error.Lines()

	// Check that no errors were logged
	assert.Zero(errorLogsAfter - errorLogsBefore)

	used, err := b.IsUsedTicket(ticket)
	require.Nil(err)
	assert.True(used)

	_, ok := r.(*recipient).invalidRands.Load(recipientRand.String())
	assert.True(ok)

	r.(*recipient).senderNoncesLock.Lock()
	_, ok = r.(*recipient).senderNonces[recipientRand.String()]
	r.(*recipient).senderNoncesLock.Unlock()
	assert.False(ok)
}

func TestTicketParams(t *testing.T) {
	sender, b, v, ts, gm, sm, em, cfg, _ := newRecipientFixtureOrFatal(t)
	recipient := RandAddress()
	secret := [32]byte{3}
	r := NewRecipientWithSecret(recipient, b, v, ts, gm, sm, em, secret, cfg)

	require := require.New(t)
	assert := assert.New(t)

	// Test SenderMonitor.MaxFloat() error
	sm.maxFloatErr = errors.New("MaxFloat error")
	_, err := r.TicketParams(sender)
	assert.EqualError(err, sm.maxFloatErr.Error())

	// Test correct params returned when default faceValue < maxFloat
	sm.maxFloatErr = nil
	params1, err := r.TicketParams(sender)
	require.Nil(err)

	if params1.Recipient != recipient {
		t.Errorf("expected recipient %x got %x", recipient, params1.Recipient)
	}

	faceValue := big.NewInt(100000000)
	if params1.FaceValue.Cmp(faceValue) != 0 {
		t.Errorf("expected faceValue %d got %d", faceValue, params1.FaceValue)
	}

	winProb, _ := new(big.Int).SetString("5789604461865809771178549250434395392663499233282028201972879200395660", 10)
	if params1.WinProb.Cmp(winProb) != 0 {
		t.Errorf("expected winProb %d got %d", winProb, params1.WinProb)
	}

	// Might be slightly off due to truncation
	expEV := new(big.Int).Div(new(big.Int).Mul(faceValue, winProb), maxWinProb)
	assert.LessOrEqual(new(big.Int).Abs(new(big.Int).Sub(cfg.EV, expEV)).Int64(), int64(1))

	recipientRandHash := crypto.Keccak256Hash(ethcommon.LeftPadBytes(genRecipientRand(sender, secret, params1.Seed).Bytes(), uint256Size))

	if params1.RecipientRandHash != recipientRandHash {
		t.Errorf("expected recipientRandHash %x got %x", recipientRandHash, params1.RecipientRandHash)
	}

	// Test correct params returned and different seed + recipientRandHash
	params2, err := r.TicketParams(sender)
	require.Nil(err)

	if params2.Recipient != recipient {
		t.Errorf("expected recipient %x got %x", recipient, params2.Recipient)
	}

	if params2.FaceValue.Cmp(faceValue) != 0 {
		t.Errorf("expected faceValue %d got %d", faceValue, params2.FaceValue)
	}

	if params2.WinProb.Cmp(winProb) != 0 {
		t.Errorf("expected winProb %d got %d", winProb, params2.WinProb)
	}

	if params2.RecipientRandHash == params1.RecipientRandHash {
		t.Errorf("expected different recipientRandHash value for different params")
	}

	if params2.Seed == params1.Seed {
		t.Errorf("expected different seed value for different params")
	}

	recipientRandHash = crypto.Keccak256Hash(ethcommon.LeftPadBytes(genRecipientRand(sender, secret, params2.Seed).Bytes(), uint256Size))

	if params2.RecipientRandHash != recipientRandHash {
		t.Errorf("expected recipientRandHash %x got %x", recipientRandHash, params2.RecipientRandHash)
	}

	// Test correct params returned and different faceValue + winProb
	gm.gasPrice = big.NewInt(777)

	params3, err := r.TicketParams(sender)
	require.Nil(err)

	faceValue = big.NewInt(777000000)
	assert.Equal(faceValue, params3.FaceValue)

	winProb, _ = new(big.Int).SetString("745122839364969082519761808292714979750772102095499125093034646125570", 10)
	assert.Equal(winProb, params3.WinProb)

	// Might be slightly off due to truncation
	expEV = new(big.Int).Div(new(big.Int).Mul(faceValue, winProb), maxWinProb)
	assert.LessOrEqual(new(big.Int).Abs(new(big.Int).Sub(cfg.EV, expEV)).Int64(), int64(1))

	// Test correct params returned when default faceValue > maxFloat
	sm.maxFloat = big.NewInt(10000)

	params4, err := r.TicketParams(sender)
	require.Nil(err)

	faceValue = sm.maxFloat
	assert.Equal(faceValue, params4.FaceValue)

	winProb, _ = new(big.Int).SetString("57896044618658097711785492504343953926634992332820282019728792003956564820", 10)
	assert.Equal(winProb, params4.WinProb)

	// Might be slightly off due to truncation
	expEV = new(big.Int).Div(new(big.Int).Mul(faceValue, winProb), maxWinProb)
	assert.LessOrEqual(new(big.Int).Abs(new(big.Int).Sub(cfg.EV, expEV)).Int64(), int64(1))

	// Test insufficient sender reserve error
	sm.maxFloat = new(big.Int).Sub(cfg.EV, big.NewInt(1))

	_, err = r.TicketParams(sender)
	assert.EqualError(err, errInsufficientSenderReserve.Error())

	// Test default faceValue < EV and maxFloat > EV
	// Set gas price = 0 to set default faceValue = 0
	gm.gasPrice = big.NewInt(0)
	sm.maxFloat = maxWinProb // Set maxFloat to some really big number

	params5, err := r.TicketParams(sender)
	require.Nil(err)

	assert.Equal(cfg.EV, params5.FaceValue)
	assert.Equal(maxWinProb, params5.WinProb)

	// Test default faceValue < EV and maxFloat < EV
	sm.maxFloat = big.NewInt(0) // Set maxFloat to some value less than EV

	_, err = r.TicketParams(sender)
	assert.EqualError(err, errInsufficientSenderReserve.Error())
}

func TestTxCostMultiplier_UsingFaceValue_ReturnsDefaultMultiplier(t *testing.T) {
	sender, b, v, ts, gm, sm, em, cfg, _ := newRecipientFixtureOrFatal(t)
	recipient := RandAddress()
	secret := [32]byte{3}
	r := NewRecipientWithSecret(recipient, b, v, ts, gm, sm, em, secret, cfg)

	mul, err := r.TxCostMultiplier(sender)
	assert.Nil(t, err)
	assert.Equal(t, big.NewRat(int64(cfg.TxCostMultiplier), 1), mul)
}

func TestTxCostMultiplier_UsingMaxFloat_ReturnsScaledMultiplier(t *testing.T) {
	sender, b, v, ts, gm, sm, em, cfg, _ := newRecipientFixtureOrFatal(t)
	recipient := RandAddress()
	secret := [32]byte{3}
	r := NewRecipientWithSecret(recipient, b, v, ts, gm, sm, em, secret, cfg)

	sm.maxFloat = big.NewInt(500000)

	txCost := new(big.Int).Mul(gm.gasPrice, big.NewInt(int64(cfg.RedeemGas)))
	expMul := new(big.Rat).SetFrac(sm.maxFloat, txCost)

	mul, err := r.TxCostMultiplier(sender)
	assert.Nil(t, err)
	assert.Equal(t, expMul, mul)
}

func TestTxCostMultiplier_MaxFloatError_ReturnsError(t *testing.T) {
	sender, b, v, ts, gm, sm, em, cfg, _ := newRecipientFixtureOrFatal(t)
	recipient := RandAddress()
	secret := [32]byte{3}
	r := NewRecipientWithSecret(recipient, b, v, ts, gm, sm, em, secret, cfg)

	sm.maxFloatErr = errors.New("MaxFloat error")
	mul, err := r.TxCostMultiplier(sender)
	assert.Nil(t, mul)
	assert.EqualError(t, err, sm.maxFloatErr.Error())
}

func TestTxCostMultiplier_InsufficientReserve_ReturnsError(t *testing.T) {
	sender, b, v, ts, gm, sm, em, cfg, _ := newRecipientFixtureOrFatal(t)
	recipient := RandAddress()
	secret := [32]byte{3}
	r := NewRecipientWithSecret(recipient, b, v, ts, gm, sm, em, secret, cfg)

	sm.maxFloat = big.NewInt(0) // Set maxFloat to some value less than EV

	mul, err := r.TxCostMultiplier(sender)
	assert.Nil(t, mul)
	assert.EqualError(t, err, errInsufficientSenderReserve.Error())
}
