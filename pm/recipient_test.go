package pm

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"fmt"
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

func newRecipientFixtureOrFatal(t *testing.T) (ethcommon.Address, *stubBroker, *stubValidator, *stubTicketStore, *stubGasPriceMonitor, *stubSenderMonitor, *stubTimeManager, TicketParamsConfig, []byte) {
	sender := RandAddress()

	b := newStubBroker()

	v := &stubValidator{}
	v.SetIsValidTicket(true)

	gm := &stubGasPriceMonitor{gasPrice: big.NewInt(100)}
	sm := newStubSenderMonitor()
	sm.maxFloat = big.NewInt(10000000000)
	tm := &stubTimeManager{lastSeenBlock: big.NewInt(1), round: big.NewInt(1), blkHash: RandHash()}
	cfg := TicketParamsConfig{
		EV:               big.NewInt(5),
		RedeemGas:        10000,
		TxCostMultiplier: 100,
	}

	return sender, b, v, newStubTicketStore(), gm, sm, tm, cfg, []byte("foo")
}

func newRecipientOrFatal(t *testing.T, addr ethcommon.Address, b Broker, v Validator, ts TicketStore, gpm GasPriceMonitor, sm SenderMonitor, tm TimeManager, cfg TicketParamsConfig) Recipient {
	r, err := NewRecipient(addr, b, v, ts, gpm, sm, tm, cfg)
	if err != nil {
		t.Fatal(err)
	}

	return r
}

func ticketParamsOrFatal(t *testing.T, r Recipient, sender ethcommon.Address) *TicketParams {
	params, err := r.TicketParams(sender, big.NewRat(1, 1))
	if err != nil {
		t.Fatal(err)
	}

	return params
}

func newTicket(sender ethcommon.Address, params *TicketParams, senderNonce uint32) *Ticket {
	return &Ticket{
		Recipient:              params.Recipient,
		Sender:                 sender,
		FaceValue:              params.FaceValue,
		WinProb:                params.WinProb,
		SenderNonce:            senderNonce,
		RecipientRandHash:      params.RecipientRandHash,
		ParamsExpirationBlock:  params.ExpirationBlock,
		PricePerPixel:          params.PricePerPixel,
		CreationRound:          params.ExpirationParams.CreationRound,
		CreationRoundBlockHash: params.ExpirationParams.CreationRoundBlockHash,
	}
}

func genRecipientRand(sender ethcommon.Address, secret [32]byte, params *TicketParams) *big.Int {
	h := hmac.New(sha256.New, secret[:])
	msg := append(params.Seed.Bytes(), sender.Bytes()...)
	msg = append(msg, params.FaceValue.Bytes()...)
	msg = append(msg, params.WinProb.Bytes()...)
	msg = append(msg, params.ExpirationBlock.Bytes()...)
	msg = append(msg, params.PricePerPixel.Num().Bytes()...)
	msg = append(msg, params.PricePerPixel.Denom().Bytes()...)
	msg = append(msg, params.ExpirationParams.AuxData()...)
	h.Write(msg)
	return new(big.Int).SetBytes(h.Sum(nil))
}

func TestReceiveTicket_InvalidRecipientRand(t *testing.T) {
	assert := assert.New(t)
	sender, b, _, ts, gm, sm, tm, cfg, sig := newRecipientFixtureOrFatal(t)

	sv := &stubSigVerifier{}
	sv.SetVerifyResult(true)
	v := NewValidator(sv, tm)
	secret := [32]byte{3}
	r := NewRecipientWithSecret(RandAddress(), b, v, ts, gm, sm, tm, secret, cfg)
	params, err := r.TicketParams(sender, big.NewRat(1, 1))
	require.Nil(t, err)

	// Test invalid faceValue
	ticket := newTicket(sender, params, 0)
	ticket.FaceValue = big.NewInt(0) // Using invalid FaceValue for generating recipientRand
	sessionID, won, err := r.ReceiveTicket(ticket, sig, params.Seed)
	require.NotEqual(t, params.FaceValue, ticket.FaceValue)
	assert.Equal(sessionID, "")
	assert.Equal(won, false)
	assert.Equal(err.Error(), errInvalidTicketRecipientRand.Error())
	_, ok := err.(*FatalReceiveErr)
	assert.True(ok)

	// Test invalid winProb
	ticket = newTicket(sender, params, 0)
	ticket.WinProb = big.NewInt(0) // Using invalid WinProb for generating recipientRand
	sessionID, won, err = r.ReceiveTicket(ticket, sig, params.Seed)
	require.NotEqual(t, params.WinProb, ticket.WinProb)
	assert.Equal(sessionID, "")
	assert.Equal(won, false)
	assert.Equal(err.Error(), errInvalidTicketRecipientRand.Error())
	_, ok = err.(*FatalReceiveErr)
	assert.True(ok)

	// Test invalid ParamsExpirationBlock
	ticket = newTicket(sender, params, 0)
	ticket.ParamsExpirationBlock = big.NewInt(0) // Using invalid ParamsExpirationBlock for generating recipientRand
	sessionID, won, err = r.ReceiveTicket(ticket, sig, params.Seed)
	require.NotEqual(t, params.ExpirationBlock, ticket.ParamsExpirationBlock)
	assert.Equal(sessionID, "")
	assert.Equal(won, false)
	assert.Equal(err.Error(), errInvalidTicketRecipientRand.Error())
	_, ok = err.(*FatalReceiveErr)
	assert.True(ok)

	// Test invalid PricePerPixel
	ticket = newTicket(sender, params, 0)
	ticket.PricePerPixel = big.NewRat(0, 1) // Using invalid PricePerPixel for generating recipientRand
	sessionID, won, err = r.ReceiveTicket(ticket, sig, params.Seed)
	assert.Equal(sessionID, "")
	assert.Equal(won, false)
	assert.Equal(err.Error(), errInvalidTicketRecipientRand.Error())
	_, ok = err.(*FatalReceiveErr)
	assert.True(ok)

	// Test invalid creation round
	ticket = newTicket(sender, params, 0)
	ticket.CreationRound = 999
	sessionID, won, err = r.ReceiveTicket(ticket, sig, params.Seed)
	assert.Equal(sessionID, "")
	assert.Equal(won, false)
	assert.Equal(err.Error(), errInvalidTicketRecipientRand.Error())
	_, ok = err.(*FatalReceiveErr)
	assert.True(ok)

	// Test invalid creation round blockhash
	ticket = newTicket(sender, params, 0)
	ticket.CreationRoundBlockHash = RandHash()
	sessionID, won, err = r.ReceiveTicket(ticket, sig, params.Seed)
	assert.Equal(sessionID, "")
	assert.Equal(won, false)
	assert.Equal(err.Error(), errInvalidTicketRecipientRand.Error())
	_, ok = err.(*FatalReceiveErr)
	assert.True(ok)
}

func TestReceiveTicket_InvalidSignature(t *testing.T) {
	assert := assert.New(t)
	sender, b, _, ts, gm, sm, tm, cfg, sig := newRecipientFixtureOrFatal(t)

	sv := &stubSigVerifier{}
	v := NewValidator(sv, tm)
	secret := [32]byte{3}
	r := NewRecipientWithSecret(RandAddress(), b, v, ts, gm, sm, tm, secret, cfg)
	params, err := r.TicketParams(sender, big.NewRat(1, 1))
	require.Nil(t, err)

	// Test invalid signature
	ticket := newTicket(sender, params, 0)
	sv.SetVerifyResult(false)
	sessionID, won, err := r.ReceiveTicket(ticket, sig, params.Seed)
	assert.Equal(sessionID, "")
	assert.Equal(won, false)
	assert.Equal(err.Error(), errInvalidTicketSignature.Error())
	_, ok := err.(*FatalReceiveErr)
	assert.False(ok)
}

func TestReceiveTicket_InvalidSender(t *testing.T) {
	assert := assert.New(t)
	sender, b, v, ts, gm, sm, tm, cfg, sig := newRecipientFixtureOrFatal(t)
	sm.validateSenderErr = errors.New("Invalid Sender")
	r := newRecipientOrFatal(t, RandAddress(), b, v, ts, gm, sm, tm, cfg)

	params, err := r.TicketParams(sender, big.NewRat(1, 1))
	require.Nil(t, err)

	// Test valid non-winning ticket
	newSenderNonce := uint32(3)
	ticket := newTicket(sender, params, newSenderNonce)

	_, _, err = r.ReceiveTicket(ticket, sig, params.Seed)
	assert.EqualError(err, "Invalid Sender")
}

func TestReceiveTicket_ValidNonWinningTicket(t *testing.T) {
	sender, b, v, ts, gm, sm, tm, cfg, sig := newRecipientFixtureOrFatal(t)
	secret := [32]byte{3}
	r := NewRecipientWithSecret(RandAddress(), b, v, ts, gm, sm, tm, secret, cfg)
	params, err := r.TicketParams(sender, big.NewRat(1, 1))
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

	recipientRand := genRecipientRand(sender, secret, params)
	senderNonce := r.(*recipient).senderNonces[recipientRand.String()]

	if senderNonce != newSenderNonce {
		t.Errorf("expected senderNonce to be %d, got %d", newSenderNonce, senderNonce)
	}
}

func TestReceiveTicket_ValidWinningTicket(t *testing.T) {
	sender, b, v, ts, gm, sm, tm, cfg, sig := newRecipientFixtureOrFatal(t)
	secret := [32]byte{3}
	r := NewRecipientWithSecret(RandAddress(), b, v, ts, gm, sm, tm, secret, cfg)
	params, err := r.TicketParams(sender, big.NewRat(1, 1))
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

	recipientRand := genRecipientRand(sender, secret, params)
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
	sender, b, v, ts, gm, sm, tm, cfg, sig := newRecipientFixtureOrFatal(t)
	secret := [32]byte{3}
	r := NewRecipientWithSecret(RandAddress(), b, v, ts, gm, sm, tm, secret, cfg)
	params, err := r.TicketParams(sender, big.NewRat(1, 1))
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

	recipientRand := genRecipientRand(sender, secret, params)
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
	assert := assert.New(t)
	sender, b, v, ts, gm, sm, tm, cfg, sig := newRecipientFixtureOrFatal(t)
	secret := [32]byte{3}
	r := NewRecipientWithSecret(RandAddress(), b, v, ts, gm, sm, tm, secret, cfg)
	r.Start()
	defer r.Stop()
	time.Sleep(20 * time.Millisecond)
	params, err := r.TicketParams(sender, big.NewRat(1, 1))
	require.Nil(t, err)

	// Test invalid recipientRand revealed
	ticket := newTicket(sender, params, 0)

	// Config stub validator with valid winning tickets
	v.SetIsWinningTicket(true)

	_, _, err = r.ReceiveTicket(ticket, sig, params.Seed)
	if err != nil {
		t.Fatal(err)
	}

	r.RedeemWinningTicket(ticket, sig, params.Seed)
	recipientRand := genRecipientRand(sender, secret, params)
	// Redeem ticket to invalidate recipientRand
	sm.Redeemable() <- &SignedTicket{
		Ticket:        ticket,
		Sig:           sig,
		RecipientRand: recipientRand,
	}
	time.Sleep(20 * time.Millisecond)

	// New ticket with same invalid recipientRand, but updated senderNonce
	ticket = newTicket(sender, params, 1)

	sessionID, won, err := r.ReceiveTicket(ticket, sig, params.Seed)
	assert.EqualError(err, fmt.Sprintf("recipientRand already revealed recipientRand=%v", recipientRand))
	assert.Equal(sessionID, ticket.RecipientRandHash.Hex())
	assert.True(won)
	_, ok := err.(*FatalReceiveErr)
	assert.False(ok)
}

func TestReceiveTicket_InvalidSenderNonce(t *testing.T) {
	sender, b, v, ts, gm, sm, tm, cfg, sig := newRecipientFixtureOrFatal(t)
	r := newRecipientOrFatal(t, RandAddress(), b, v, ts, gm, sm, tm, cfg)
	params, err := r.TicketParams(sender, big.NewRat(1, 1))
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
	_, ok := err.(*FatalReceiveErr)
	assert.False(t, ok)
	// Replay senderNonce = 0 (new nonce < highest seen nonce)
	_, _, err = r.ReceiveTicket(ticket0, sig, params.Seed)
	if err == nil {
		t.Error("expected invalid senderNonce (new nonce < highest seen nonce) error")
	}
	if err != nil && !strings.Contains(err.Error(), "invalid ticket senderNonce") {
		t.Errorf("expected invalid senderNonce (new nonce < highest seen nonce) error, got %v", err)
	}
	_, ok = err.(*FatalReceiveErr)
	assert.False(t, ok)
}

func TestReceiveTicket_ValidNonWinningTicket_Concurrent(t *testing.T) {
	sender, b, v, ts, gm, sm, tm, cfg, sig := newRecipientFixtureOrFatal(t)
	r := newRecipientOrFatal(t, RandAddress(), b, v, ts, gm, sm, tm, cfg)
	params, err := r.TicketParams(sender, big.NewRat(1, 1))
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

func TestReceiveTicket_ValidTicket_Expired(t *testing.T) {
	assert := assert.New(t)
	sender, b, v, ts, gm, sm, tm, cfg, sig := newRecipientFixtureOrFatal(t)
	secret := [32]byte{3}
	r := NewRecipientWithSecret(RandAddress(), b, v, ts, gm, sm, tm, secret, cfg)
	params, err := r.TicketParams(sender, big.NewRat(1, 1))
	tm.lastSeenBlock = big.NewInt(100)
	require.Nil(t, err)

	// Test valid winning ticket
	newSenderNonce := uint32(3)
	ticket := newTicket(sender, params, newSenderNonce)

	// Config stub validator with valid winning tickets
	v.SetIsWinningTicket(true)

	sessionID, won, err := r.ReceiveTicket(ticket, sig, params.Seed)
	assert.Equal(sessionID, params.RecipientRandHash.Hex())
	assert.True(won)
	assert.EqualError(err, ErrTicketParamsExpired.Error())
	_, ok := err.(*FatalReceiveErr)
	assert.False(ok)
}

func TestRedeemWinningTickets_InvalidSessionID(t *testing.T) {
	_, b, v, ts, gm, sm, tm, cfg, _ := newRecipientFixtureOrFatal(t)
	r := newRecipientOrFatal(t, RandAddress(), b, v, ts, gm, sm, tm, cfg)
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

func TestRedeemWinningTicket_SingleTicket_ZeroMaxFloat(t *testing.T) {
	assert := assert.New(t)
	sender, b, v, ts, gm, sm, tm, cfg, sig := newRecipientFixtureOrFatal(t)
	secret := [32]byte{3}
	r := NewRecipientWithSecret(RandAddress(), b, v, ts, gm, sm, tm, secret, cfg).(*recipient)

	params, err := r.TicketParams(sender, big.NewRat(1, 1))
	require.Nil(t, err)

	// Config stub validator with valid winning tickets
	v.SetIsWinningTicket(true)

	// Test zero maxfloat error
	ticket := newTicket(sender, params, 0)

	_, won, err := r.ReceiveTicket(ticket, sig, params.Seed)
	assert.NoError(err)
	assert.True(won)

	recipientRand := genRecipientRand(sender, secret, params)
	err = r.RedeemWinningTicket(ticket, sig, params.Seed)
	assert.Nil(err)
	assert.Equal(1, len(sm.queued))
	assert.Equal(&SignedTicket{ticket, sig, recipientRand}, sm.queued[0])

	sm.maxFloat = big.NewInt(0)
	err = r.redeemWinningTicket(ticket, sig, recipientRand)
	assert.EqualError(err, "max float is zero")
}

func TestRedeemWinningTicket_SingleTicket_RedeemError(t *testing.T) {
	assert := assert.New(t)
	sender, b, v, ts, gm, sm, tm, cfg, sig := newRecipientFixtureOrFatal(t)
	secret := [32]byte{3}
	r := NewRecipientWithSecret(RandAddress(), b, v, ts, gm, sm, tm, secret, cfg).(*recipient)
	r.Start()
	defer r.Stop()
	time.Sleep(20 * time.Millisecond)
	params, err := r.TicketParams(sender, big.NewRat(1, 1))
	require.Nil(t, err)

	// Config stub validator with valid winning tickets
	v.SetIsWinningTicket(true)

	// Sanity check that queue is empty
	assert.Equal(0, len(sm.queued))

	// Test redeem error
	ticket := newTicket(sender, params, 2)

	_, won, err := r.ReceiveTicket(ticket, sig, params.Seed)
	assert.NoError(err)
	assert.True(won)

	recipientRand := genRecipientRand(sender, secret, params)

	err = r.RedeemWinningTicket(ticket, sig, params.Seed)
	assert.Nil(err)
	assert.Equal(1, len(sm.queued))
	assert.Equal(&SignedTicket{ticket, sig, recipientRand}, sm.queued[0])

	// Config stub broker to fail redeem
	b.redeemShouldFail = true
	err = r.redeemWinningTicket(ticket, sig, recipientRand)
	assert.EqualError(err, "stub broker redeem error")

	used, err := b.IsUsedTicket(ticket)
	assert.NoError(err)
	assert.False(used)

	_, ok := r.invalidRands.Load(recipientRand.String())
	assert.False(ok)
	assert.Contains(r.senderNonces, recipientRand.String())
}

func TestRedeemWinningTicket_SingleTicket_CheckTxError(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	sender, b, v, ts, gm, sm, tm, cfg, sig := newRecipientFixtureOrFatal(t)
	secret := [32]byte{3}
	r := NewRecipientWithSecret(RandAddress(), b, v, ts, gm, sm, tm, secret, cfg).(*recipient)
	params, err := r.TicketParams(sender, big.NewRat(1, 1))
	require.Nil(err)

	// Config stub validator with valid winning tickets
	v.SetIsWinningTicket(true)

	ticket := newTicket(sender, params, 2)

	_, won, err := r.ReceiveTicket(ticket, sig, params.Seed)
	require.Nil(err)
	require.True(won)

	// Config stub broker to fail CheckTx
	b.checkTxErr = errors.New("CheckTx error")

	recipientRand := genRecipientRand(sender, secret, params)
	err = r.RedeemWinningTicket(ticket, sig, params.Seed)
	assert.Nil(err)
	assert.Equal(1, len(sm.queued))
	assert.Equal(&SignedTicket{ticket, sig, recipientRand}, sm.queued[0])

	err = r.redeemWinningTicket(ticket, sig, recipientRand)
	assert.EqualError(err, b.checkTxErr.Error())
}

func TestRedeemWinningTicket_SingleTicket(t *testing.T) {
	assert := assert.New(t)
	sender, b, v, ts, gm, sm, tm, cfg, sig := newRecipientFixtureOrFatal(t)
	secret := [32]byte{3}
	r := NewRecipientWithSecret(RandAddress(), b, v, ts, gm, sm, tm, secret, cfg).(*recipient)
	params, err := r.TicketParams(sender, big.NewRat(1, 1))
	require.Nil(t, err)

	// Config stub validator with valid winning tickets
	v.SetIsWinningTicket(true)

	// Test single ticket
	ticket := newTicket(sender, params, 2)

	_, won, err := r.ReceiveTicket(ticket, sig, params.Seed)
	assert.NoError(err)
	assert.True(won)

	recipientRand := genRecipientRand(sender, secret, params)
	err = r.RedeemWinningTicket(ticket, sig, params.Seed)
	assert.Nil(err)
	assert.Equal(1, len(sm.queued))
	assert.Equal(&SignedTicket{ticket, sig, recipientRand}, sm.queued[0])

	err = r.redeemWinningTicket(ticket, sig, recipientRand)
	assert.NoError(err)

	used, err := b.IsUsedTicket(ticket)
	assert.NoError(err)
	assert.True(used)

	_, ok := r.invalidRands.Load(recipientRand.String())
	assert.True(ok)

	_, ok = r.senderNonces[recipientRand.String()]
	assert.False(ok)
}

func TestRedeemWinningTickets_MultipleTickets(t *testing.T) {
	assert := assert.New(t)
	sender, b, v, ts, gm, sm, tm, cfg, sig := newRecipientFixtureOrFatal(t)
	secret := [32]byte{3}
	r := NewRecipientWithSecret(RandAddress(), b, v, ts, gm, sm, tm, secret, cfg).(*recipient)

	params, err := r.TicketParams(sender, big.NewRat(1, 1))
	require.Nil(t, err)

	// Config stub validator with valid winning tickets
	v.SetIsWinningTicket(true)

	// Test multiple tickets
	// Receive ticket 0
	ticket0 := newTicket(sender, params, 2)

	sessionID, won, err := r.ReceiveTicket(ticket0, sig, params.Seed)
	assert.NoError(err)
	assert.True(won)

	// Receive ticket 1
	ticket1 := newTicket(sender, params, 3)

	sessionID, won, err = r.ReceiveTicket(ticket1, sig, params.Seed)
	assert.NoError(err)
	assert.True(won)

	recipientRand := genRecipientRand(sender, secret, params)

	// Check that tickets are loaded from ticketStore and added to the queue
	err = r.RedeemWinningTickets([]string{sessionID})
	assert.NoError(err)
	assert.Equal(2, len(sm.queued))
	assert.Equal(&SignedTicket{ticket0, sig, recipientRand}, sm.queued[0])
	assert.Equal(&SignedTicket{ticket1, sig, recipientRand}, sm.queued[1])

	// Actually redeem the tickets
	err = r.redeemWinningTicket(ticket0, sig, recipientRand)
	assert.NoError(err)
	err = r.redeemWinningTicket(ticket1, sig, recipientRand)
	assert.NoError(err)

	used, err := b.IsUsedTicket(ticket0)
	assert.NoError(err)
	assert.True(used)

	used, err = b.IsUsedTicket(ticket1)
	assert.NoError(err)
	assert.True(used)

	_, ok := r.invalidRands.Load(recipientRand.String())
	assert.True(ok)

	_, ok = r.senderNonces[recipientRand.String()]
	assert.False(ok)
}

func TestRedeemWinningTickets_MultipleTicketsFromMultipleSessions(t *testing.T) {
	assert := assert.New(t)
	sender, b, v, ts, gm, sm, tm, cfg, sig := newRecipientFixtureOrFatal(t)
	secret := [32]byte{3}
	r := NewRecipientWithSecret(RandAddress(), b, v, ts, gm, sm, tm, secret, cfg).(*recipient)
	// Config stub validator with valid winning tickets
	v.SetIsWinningTicket(true)
	require := require.New(t)

	tm.lastSeenBlock = big.NewInt(3)
	params0 := ticketParamsOrFatal(t, r, sender)
	ticket0 := newTicket(sender, params0, 1)
	tm.lastSeenBlock = big.NewInt(0)
	sessionID0, won, err := r.ReceiveTicket(ticket0, sig, params0.Seed)
	require.Nil(err)
	require.True(won)

	tm.lastSeenBlock = big.NewInt(4)
	params1 := ticketParamsOrFatal(t, r, sender)
	ticket1 := newTicket(sender, params1, 1)
	tm.lastSeenBlock = big.NewInt(0)
	sessionID1, won, err := r.ReceiveTicket(ticket1, sig, params1.Seed)
	require.Nil(err)
	require.True(won)

	require.NotEqual(sessionID0, sessionID1)

	recipientRand0 := genRecipientRand(sender, secret, params0)
	recipientRand1 := genRecipientRand(sender, secret, params1)

	err = r.RedeemWinningTickets([]string{sessionID0, sessionID1})
	assert.NoError(err)
	assert.Equal(2, len(sm.queued))
	assert.Equal(&SignedTicket{ticket0, sig, recipientRand0}, sm.queued[0])
	assert.Equal(&SignedTicket{ticket1, sig, recipientRand1}, sm.queued[1])

	err = r.redeemWinningTicket(ticket0, sig, recipientRand0)
	assert.NoError(err)
	err = r.redeemWinningTicket(ticket1, sig, recipientRand1)
	assert.NoError(err)

	used, err := b.IsUsedTicket(ticket0)
	require.Nil(err)
	assert.True(used)
	used, err = b.IsUsedTicket(ticket1)
	require.Nil(err)
	assert.True(used)

	_, ok := r.invalidRands.Load(recipientRand0.String())
	assert.True(ok)
	_, ok = r.invalidRands.Load(recipientRand1.String())
	assert.True(ok)

	_, ok = r.senderNonces[recipientRand0.String()]
	assert.False(ok)
	_, ok = r.senderNonces[recipientRand1.String()]
	assert.False(ok)
}

func TestRedeemWinningTicket_MaxFloatError(t *testing.T) {
	assert := assert.New(t)
	sender, b, v, ts, gm, sm, tm, cfg, sig := newRecipientFixtureOrFatal(t)
	secret := [32]byte{3}
	r := NewRecipientWithSecret(RandAddress(), b, v, ts, gm, sm, tm, secret, cfg).(*recipient)

	params := ticketParamsOrFatal(t, r, sender)
	ticket := newTicket(sender, params, 1)

	sm.maxFloatErr = errors.New("MaxFloat error")
	recipientRand := genRecipientRand(sender, secret, params)
	err := r.RedeemWinningTicket(ticket, sig, params.Seed)
	assert.Nil(err)
	assert.Equal(1, len(sm.queued))
	assert.Equal(&SignedTicket{ticket, sig, recipientRand}, sm.queued[0])
	err = r.redeemWinningTicket(ticket, sig, recipientRand)
	assert.EqualError(err, sm.maxFloatErr.Error())
}

func TestRedeemWinningTicket_InsufficientMaxFloat_QueueTicket(t *testing.T) {
	assert := assert.New(t)

	sender, b, v, ts, gm, sm, tm, cfg, sig := newRecipientFixtureOrFatal(t)
	secret := [32]byte{3}
	r := NewRecipientWithSecret(RandAddress(), b, v, ts, gm, sm, tm, secret, cfg).(*recipient)

	params := ticketParamsOrFatal(t, r, sender)
	ticket := newTicket(sender, params, 1)
	ticket.FaceValue = big.NewInt(99999999999999)

	recipientRand := genRecipientRand(sender, secret, params)
	err := r.redeemWinningTicket(ticket, sig, recipientRand)
	assert.EqualError(err, "insufficient max float - faceValue=99999999999999 maxFloat=10000000000")

	assert.Equal(1, len(sm.queued))
	assert.Equal(&SignedTicket{ticket, sig, recipientRand}, sm.queued[0])
}

func TestRedeemWinningTicket_AddFloatError(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	sender, b, v, ts, gm, sm, tm, cfg, sig := newRecipientFixtureOrFatal(t)
	secret := [32]byte{3}
	r := NewRecipientWithSecret(RandAddress(), b, v, ts, gm, sm, tm, secret, cfg).(*recipient)

	params := ticketParamsOrFatal(t, r, sender)
	ticket := newTicket(sender, params, 1)

	_, _, err := r.ReceiveTicket(ticket, sig, params.Seed)
	require.Nil(err)

	sm.addFloatErr = errors.New("AddFloat error")

	errorLogsBefore := glog.Stats.Error.Lines()
	recipientRand := genRecipientRand(sender, secret, params)
	r.redeemWinningTicket(ticket, sig, recipientRand)
	errorLogsAfter := glog.Stats.Error.Lines()
	assert.Equal(int64(1), errorLogsAfter-errorLogsBefore)
	used, err := b.IsUsedTicket(ticket)
	require.Nil(err)
	assert.True(used)

	_, ok := r.invalidRands.Load(recipientRand.String())
	assert.True(ok)

	_, ok = r.senderNonces[recipientRand.String()]
	assert.False(ok)
}

func TestRedeemWinningTicket(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	sender, b, v, ts, gm, sm, tm, cfg, sig := newRecipientFixtureOrFatal(t)
	secret := [32]byte{3}
	r := NewRecipientWithSecret(RandAddress(), b, v, ts, gm, sm, tm, secret, cfg).(*recipient)

	params := ticketParamsOrFatal(t, r, sender)
	ticket := newTicket(sender, params, 1)

	_, _, err := r.ReceiveTicket(ticket, sig, params.Seed)
	require.Nil(err)

	errorLogsBefore := glog.Stats.Error.Lines()

	recipientRand := genRecipientRand(sender, secret, params)
	err = r.redeemWinningTicket(ticket, sig, recipientRand)
	assert.Nil(err)

	errorLogsAfter := glog.Stats.Error.Lines()

	// Check that no errors were logged
	assert.Zero(errorLogsAfter - errorLogsBefore)

	used, err := b.IsUsedTicket(ticket)
	require.Nil(err)
	assert.True(used)

	_, ok := r.invalidRands.Load(recipientRand.String())
	assert.True(ok)

	_, ok = r.senderNonces[recipientRand.String()]
	assert.False(ok)
}

func TestRedeemManager_Error(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	sender, b, v, ts, gm, sm, tm, cfg, sig := newRecipientFixtureOrFatal(t)
	secret := [32]byte{3}
	r := NewRecipientWithSecret(RandAddress(), b, v, ts, gm, sm, tm, secret, cfg)
	r.Start()
	defer r.Stop()

	params := ticketParamsOrFatal(t, r, sender)
	ticket := newTicket(sender, params, 1)
	recipientRand := genRecipientRand(sender, secret, params)

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

	sender, b, v, ts, gm, sm, tm, cfg, sig := newRecipientFixtureOrFatal(t)
	secret := [32]byte{3}
	r := NewRecipientWithSecret(RandAddress(), b, v, ts, gm, sm, tm, secret, cfg)
	r.Start()
	defer r.Stop()

	params := ticketParamsOrFatal(t, r, sender)
	ticket := newTicket(sender, params, 1)
	recipientRand := genRecipientRand(sender, secret, params)

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
	sender, b, v, ts, gm, sm, tm, cfg, _ := newRecipientFixtureOrFatal(t)
	recipient := RandAddress()
	secret := [32]byte{3}
	r := NewRecipientWithSecret(recipient, b, v, ts, gm, sm, tm, secret, cfg)

	require := require.New(t)
	assert := assert.New(t)

	// Test SenderMonitor.MaxFloat() error
	sm.maxFloatErr = errors.New("MaxFloat error")
	_, err := r.TicketParams(sender, big.NewRat(1, 1))
	assert.EqualError(err, sm.maxFloatErr.Error())

	// Test correct params returned when default faceValue < maxFloat
	sm.maxFloatErr = nil
	params1, err := r.TicketParams(sender, big.NewRat(1, 1))
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

	recipientRandHash := crypto.Keccak256Hash(ethcommon.LeftPadBytes(genRecipientRand(sender, secret, params1).Bytes(), uint256Size))

	if params1.RecipientRandHash != recipientRandHash {
		t.Errorf("expected recipientRandHash %x got %x", recipientRandHash, params1.RecipientRandHash)
	}

	assert.Equal(params1.ExpirationParams.CreationRound, tm.round.Int64())
	assert.Equal(params1.ExpirationParams.CreationRoundBlockHash.Bytes(), tm.blkHash[:])

	// Test correct params returned and different seed + recipientRandHash
	params2, err := r.TicketParams(sender, big.NewRat(1, 1))
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

	recipientRandHash = crypto.Keccak256Hash(ethcommon.LeftPadBytes(genRecipientRand(sender, secret, params2).Bytes(), uint256Size))

	if params2.RecipientRandHash != recipientRandHash {
		t.Errorf("expected recipientRandHash %x got %x", recipientRandHash, params2.RecipientRandHash)
	}

	// Test correct params returned and different faceValue + winProb
	gm.gasPrice = big.NewInt(777)

	params3, err := r.TicketParams(sender, big.NewRat(1, 1))
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

	params4, err := r.TicketParams(sender, big.NewRat(1, 1))
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
	_, err = r.TicketParams(sender, big.NewRat(1, 1))
	assert.EqualError(err, errInsufficientSenderReserve.Error())

	// Test default faceValue < EV and maxFloat > EV
	// Set gas price = 0 to set default faceValue = 0
	gm.gasPrice = big.NewInt(0)
	sm.maxFloat = maxWinProb // Set maxFloat to some really big number

	params5, err := r.TicketParams(sender, big.NewRat(1, 1))
	require.Nil(err)

	assert.Equal(cfg.EV, params5.FaceValue)
	assert.Equal(maxWinProb, params5.WinProb)

	// Test default faceValue < EV and maxFloat < EV
	sm.maxFloat = big.NewInt(0) // Set maxFloat to some value less than EV

	_, err = r.TicketParams(sender, big.NewRat(1, 1))
	assert.EqualError(err, errInsufficientSenderReserve.Error())
}

func TestTxCostMultiplier_UsingFaceValue_ReturnsDefaultMultiplier(t *testing.T) {
	sender, b, v, ts, gm, sm, tm, cfg, _ := newRecipientFixtureOrFatal(t)
	recipient := RandAddress()
	secret := [32]byte{3}
	r := NewRecipientWithSecret(recipient, b, v, ts, gm, sm, tm, secret, cfg)

	mul, err := r.TxCostMultiplier(sender)
	assert.Nil(t, err)
	assert.Equal(t, big.NewRat(int64(cfg.TxCostMultiplier), 1), mul)
}

func TestTxCostMultiplier_UsingMaxFloat_ReturnsScaledMultiplier(t *testing.T) {
	sender, b, v, ts, gm, sm, tm, cfg, _ := newRecipientFixtureOrFatal(t)
	recipient := RandAddress()
	secret := [32]byte{3}
	r := NewRecipientWithSecret(recipient, b, v, ts, gm, sm, tm, secret, cfg)

	sm.maxFloat = big.NewInt(500000)

	txCost := new(big.Int).Mul(gm.gasPrice, big.NewInt(int64(cfg.RedeemGas)))
	expMul := new(big.Rat).SetFrac(sm.maxFloat, txCost)

	mul, err := r.TxCostMultiplier(sender)
	assert.Nil(t, err)
	assert.Equal(t, expMul, mul)
}

func TestTxCostMultiplier_MaxFloatError_ReturnsError(t *testing.T) {
	sender, b, v, ts, gm, sm, tm, cfg, _ := newRecipientFixtureOrFatal(t)
	recipient := RandAddress()
	secret := [32]byte{3}
	r := NewRecipientWithSecret(recipient, b, v, ts, gm, sm, tm, secret, cfg)

	sm.maxFloatErr = errors.New("MaxFloat error")
	mul, err := r.TxCostMultiplier(sender)
	assert.Nil(t, mul)
	assert.EqualError(t, err, sm.maxFloatErr.Error())
}

func TestTxCostMultiplier_InsufficientReserve_ReturnsError(t *testing.T) {
	sender, b, v, ts, gm, sm, tm, cfg, _ := newRecipientFixtureOrFatal(t)
	recipient := RandAddress()
	secret := [32]byte{3}
	r := NewRecipientWithSecret(recipient, b, v, ts, gm, sm, tm, secret, cfg)

	sm.maxFloat = big.NewInt(0) // Set maxFloat to some value less than EV

	mul, err := r.TxCostMultiplier(sender)
	assert.Nil(t, mul)
	assert.EqualError(t, err, errInsufficientSenderReserve.Error())
}
