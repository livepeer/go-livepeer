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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

func newRecipientFixtureOrFatal(t *testing.T) (ethcommon.Address, *stubBroker, *stubValidator, *stubTicketStore, *big.Int, *big.Int, []byte) {
	sender := RandAddress()

	b := newStubBroker()
	b.SetDeposit(sender, big.NewInt(500))
	b.SetReserve(sender, big.NewInt(500))

	v := &stubValidator{}
	v.SetIsValidTicket(true)

	return sender, b, v, newStubTicketStore(), big.NewInt(100), big.NewInt(100), []byte("foo")
}

func newRecipientOrFatal(t *testing.T, addr ethcommon.Address, b Broker, v Validator, ts TicketStore, faceValue *big.Int, winProb *big.Int) Recipient {
	r, err := NewRecipient(addr, b, v, ts, faceValue, winProb)
	if err != nil {
		t.Fatal(err)
	}

	return r
}

func ticketParamsOrFatal(t *testing.T, r Recipient, sender ethcommon.Address) *TicketParams {
	params := r.TicketParams(sender)

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

func TestReceiveTicket_InvalidRecipientRand_InvalidSeed(t *testing.T) {
	sender, b, v, ts, faceValue, winProb, sig := newRecipientFixtureOrFatal(t)
	r := newRecipientOrFatal(t, RandAddress(), b, v, ts, faceValue, winProb)
	params := r.TicketParams(sender)

	// Test invalid recipientRand from seed (invalid seed)
	ticket := newTicket(sender, params, 0)

	// Using invalid seed
	invalidSeed := new(big.Int).Add(params.Seed, big.NewInt(99))
	_, _, err := r.ReceiveTicket(ticket, sig, invalidSeed)
	if err == nil {
		t.Error("expected invalid recipientRand generated from seed error")
	}
	if err != nil && !strings.Contains(err.Error(), "invalid recipientRand generated from seed") {
		t.Errorf("expected invalid recipientRand generated from seed error, got %v", err)
	}
}

func TestReceiveTicket_InvalidRecipientRand_InvalidSender(t *testing.T) {
	sender, b, v, ts, faceValue, winProb, sig := newRecipientFixtureOrFatal(t)
	r := newRecipientOrFatal(t, RandAddress(), b, v, ts, faceValue, winProb)
	params := r.TicketParams(sender)

	// Test invalid recipientRand from seed (invalid sender)
	ticket := newTicket(ethcommon.Address{}, params, 0)

	_, _, err := r.ReceiveTicket(ticket, sig, params.Seed)
	if err == nil {
		t.Error("expected invalid recipientRand from seed error")
	}
	if err != nil && !strings.Contains(err.Error(), "invalid recipientRand generated from seed") {
		t.Errorf("expected invalid recipientRand from seed error, got %v", err)
	}
}

func TestReceiveTicket_InvalidRecipientRand_InvalidRecipientRandHash(t *testing.T) {
	sender, b, v, ts, faceValue, winProb, sig := newRecipientFixtureOrFatal(t)
	r := newRecipientOrFatal(t, RandAddress(), b, v, ts, faceValue, winProb)
	params := r.TicketParams(sender)

	// Test invalid recipientRand from seed (invalid recipientRandHash)
	ticket := newTicket(sender, params, 0)
	ticket.RecipientRandHash = RandHash() // Using invalid recipientRandHash

	_, _, err := r.ReceiveTicket(ticket, sig, params.Seed)
	if err == nil {
		t.Error("expected invalid recipientRand from seed error")
	}
	if err != nil && !strings.Contains(err.Error(), "invalid recipientRand generated from seed") {
		t.Errorf("expected invalid recipientRand from seed error, got %v", err)
	}
}

func TestReceiveTicket_InvalidFaceValue(t *testing.T) {
	sender, b, v, ts, faceValue, winProb, sig := newRecipientFixtureOrFatal(t)
	r := newRecipientOrFatal(t, RandAddress(), b, v, ts, faceValue, winProb)
	params := r.TicketParams(sender)

	// Test invalid faceValue
	ticket := newTicket(sender, params, 0)
	ticket.FaceValue = big.NewInt(0) // Using invalid faceValue

	_, _, err := r.ReceiveTicket(ticket, sig, params.Seed)
	if err == nil {
		t.Error("expected invalid faceValue error")
	}
	if err != nil && !strings.Contains(err.Error(), "invalid ticket faceValue") {
		t.Errorf("expected invalid faceValue error, got %v", err)
	}
}

func TestReceiveTicket_InvalidWinProb(t *testing.T) {
	sender, b, v, ts, faceValue, winProb, sig := newRecipientFixtureOrFatal(t)
	r := newRecipientOrFatal(t, RandAddress(), b, v, ts, faceValue, winProb)
	params := r.TicketParams(sender)

	// Test invalid winProb
	ticket := newTicket(sender, params, 0)
	ticket.WinProb = big.NewInt(0) // Using invalid winProb

	_, _, err := r.ReceiveTicket(ticket, sig, params.Seed)
	if err == nil {
		t.Error("expected invalid winProb error")
	}
	if err != nil && !strings.Contains(err.Error(), "invalid ticket winProb") {
		t.Errorf("expected invalid winProb error, got %v", err)
	}
}

func TestReceiveTicket_InvalidTicket(t *testing.T) {
	sender, b, v, ts, faceValue, winProb, sig := newRecipientFixtureOrFatal(t)
	r := newRecipientOrFatal(t, RandAddress(), b, v, ts, faceValue, winProb)
	params := r.TicketParams(sender)

	// Test invalid ticket
	ticket := newTicket(sender, params, 0)

	// Config stub validator with invalid non-winning tickets
	v.SetIsValidTicket(false)

	_, _, err := r.ReceiveTicket(ticket, sig, params.Seed)
	if err == nil {
		t.Error("expected validator invalid ticket error")
	}
	if err != nil && !strings.Contains(err.Error(), "validator invalid ticket error") {
		t.Errorf("expected validator invalid ticket error, got %v", err)
	}
}

func TestReceiveTicket_ValidNonWinningTicket(t *testing.T) {
	sender, b, v, ts, faceValue, winProb, sig := newRecipientFixtureOrFatal(t)
	secret := [32]byte{3}
	r := NewRecipientWithSecret(RandAddress(), b, v, ts, secret, faceValue, winProb)
	params := r.TicketParams(sender)

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
	sender, b, v, ts, faceValue, winProb, sig := newRecipientFixtureOrFatal(t)
	secret := [32]byte{3}
	r := NewRecipientWithSecret(RandAddress(), b, v, ts, secret, faceValue, winProb)
	params := r.TicketParams(sender)

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
	sender, b, v, ts, faceValue, winProb, sig := newRecipientFixtureOrFatal(t)
	secret := [32]byte{3}
	r := NewRecipientWithSecret(RandAddress(), b, v, ts, secret, faceValue, winProb)
	params := r.TicketParams(sender)

	// Test valid winning ticket
	newSenderNonce := uint32(3)
	ticket := newTicket(sender, params, newSenderNonce)

	// Config stub validator with valid winning tickets
	v.SetIsWinningTicket(true)
	// Config stub ticket store to fail store
	ts.storeShouldFail = true

	_, _, err := r.ReceiveTicket(ticket, sig, params.Seed)

	if err == nil {
		t.Error("expected ticket store store error")
	}
	if err != nil && !strings.Contains(err.Error(), "ticket store store error") {
		t.Errorf("expected ticket store store error, got %v", err)
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
	sender, b, v, ts, faceValue, winProb, sig := newRecipientFixtureOrFatal(t)
	r := newRecipientOrFatal(t, RandAddress(), b, v, ts, faceValue, winProb)
	params := r.TicketParams(sender)

	// Test invalid recipientRand revealed
	ticket := newTicket(sender, params, 0)

	// Config stub validator with valid winning tickets
	v.SetIsWinningTicket(true)

	_, _, err := r.ReceiveTicket(ticket, sig, params.Seed)
	if err != nil {
		t.Fatal(err)
	}

	// Redeem ticket to invalidate recipientRand
	if err := r.RedeemWinningTickets([]string{ticket.RecipientRandHash.Hex()}); err != nil {
		t.Fatal(err)
	}

	// New ticket with same invalid recipientRand, but updated senderNonce
	ticket = newTicket(sender, params, 1)

	_, _, err = r.ReceiveTicket(ticket, sig, params.Seed)
	if err == nil {
		t.Error("expected invalid recipientRand revealed error")
	}
	if err != nil && !strings.Contains(err.Error(), "invalid already revealed recipientRand") {
		t.Errorf("expected invalid recipientRand revealed error, got %v", err)
	}
}

func TestReceiveTicket_InvalidSenderNonce(t *testing.T) {
	sender, b, v, ts, faceValue, winProb, sig := newRecipientFixtureOrFatal(t)
	r := newRecipientOrFatal(t, RandAddress(), b, v, ts, faceValue, winProb)
	params := r.TicketParams(sender)

	// Test invalid senderNonce
	// Receive senderNonce = 0
	ticket0 := newTicket(sender, params, 0)

	_, _, err := r.ReceiveTicket(ticket0, sig, params.Seed)
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
	sender, b, v, ts, faceValue, winProb, sig := newRecipientFixtureOrFatal(t)
	r := newRecipientOrFatal(t, RandAddress(), b, v, ts, faceValue, winProb)
	params := r.TicketParams(sender)

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
	_, b, v, ts, faceValue, winProb, _ := newRecipientFixtureOrFatal(t)
	r := newRecipientOrFatal(t, RandAddress(), b, v, ts, faceValue, winProb)

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

func TestRedeemWinningTickets_SingleTicket_GetSenderInfoError(t *testing.T) {
	sender, b, v, ts, faceValue, winProb, sig := newRecipientFixtureOrFatal(t)
	r := newRecipientOrFatal(t, RandAddress(), b, v, ts, faceValue, winProb)
	params := r.TicketParams(sender)

	// Config stub validator with valid winning tickets
	v.SetIsWinningTicket(true)

	// Test get deposit error
	ticket := newTicket(sender, params, 0)

	sessionID, won, err := r.ReceiveTicket(ticket, sig, params.Seed)
	if err != nil {
		t.Fatal(err)
	}
	if !won {
		t.Fatal("expected valid winning ticket")
	}

	// Config stub broker to fail getting deposit
	b.getSenderInfoShouldFail = true

	err = r.RedeemWinningTickets([]string{sessionID})
	if err == nil {
		t.Error("expected broker GetSenderInfo error")
	}
	if err != nil && !strings.Contains(err.Error(), "broker GetSenderInfo error") {
		t.Errorf("execpted broker GetSenderInfo error, got %v", err)
	}
}

func TestRedeemWinningTickets_SingleTicket_ZeroDepositAndReserve(t *testing.T) {
	sender, b, v, ts, faceValue, winProb, sig := newRecipientFixtureOrFatal(t)
	r := newRecipientOrFatal(t, RandAddress(), b, v, ts, faceValue, winProb)
	params := r.TicketParams(sender)

	// Config stub validator with valid winning tickets
	v.SetIsWinningTicket(true)

	// Test zero deposit and reserve error
	ticket := newTicket(sender, params, 0)

	sessionID, won, err := r.ReceiveTicket(ticket, sig, params.Seed)
	if err != nil {
		t.Fatal(err)
	}
	if !won {
		t.Fatal("expected valid winning ticket")
	}

	// Config stub broker with zero deposit and reserve
	b.SetDeposit(sender, big.NewInt(0))
	b.SetReserve(sender, big.NewInt(0))

	err = r.RedeemWinningTickets([]string{sessionID})
	if err == nil {
		t.Error("expected zero deposit and reserve error")
	}
	if err != nil && !strings.Contains(err.Error(), "zero deposit and reserve") {
		t.Errorf("expected zero deposit and reserve error, got %v", err)
	}
}

func TestRedeemWinningTickets_SingleTicket_RedeemError(t *testing.T) {
	sender, b, v, ts, faceValue, winProb, sig := newRecipientFixtureOrFatal(t)
	secret := [32]byte{3}
	r := NewRecipientWithSecret(RandAddress(), b, v, ts, secret, faceValue, winProb)
	params := r.TicketParams(sender)

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

func TestRedeemWinningTickets_SingleTicket(t *testing.T) {
	sender, b, v, ts, faceValue, winProb, sig := newRecipientFixtureOrFatal(t)
	secret := [32]byte{3}
	r := NewRecipientWithSecret(RandAddress(), b, v, ts, secret, faceValue, winProb)
	params := r.TicketParams(sender)

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
	sender, b, v, ts, faceValue, winProb, sig := newRecipientFixtureOrFatal(t)
	secret := [32]byte{3}
	r := NewRecipientWithSecret(RandAddress(), b, v, ts, secret, faceValue, winProb)
	params := r.TicketParams(sender)

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
	sender, b, v, ts, faceValue, winProb, sig := newRecipientFixtureOrFatal(t)
	secret := [32]byte{3}
	r := NewRecipientWithSecret(RandAddress(), b, v, ts, secret, faceValue, winProb)
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

func TestTicketParams(t *testing.T) {
	sender, b, v, ts, faceValue, winProb, _ := newRecipientFixtureOrFatal(t)
	recipient := RandAddress()
	secret := [32]byte{3}
	r := NewRecipientWithSecret(recipient, b, v, ts, secret, faceValue, winProb)

	// Test correct params returned
	params1 := r.TicketParams(sender)

	if params1.Recipient != recipient {
		t.Errorf("expected recipient %x got %x", recipient, params1.Recipient)
	}

	if params1.FaceValue.Cmp(faceValue) != 0 {
		t.Errorf("expected faceValue %d got %d", faceValue, params1.FaceValue)
	}

	if params1.WinProb.Cmp(winProb) != 0 {
		t.Errorf("expected winProb %d got %d", winProb, params1.WinProb)
	}

	recipientRandHash := crypto.Keccak256Hash(ethcommon.LeftPadBytes(genRecipientRand(sender, secret, params1.Seed).Bytes(), uint256Size))

	if params1.RecipientRandHash != recipientRandHash {
		t.Errorf("expected recipientRandHash %x got %x", recipientRandHash, params1.RecipientRandHash)
	}

	// Test correct params returned and different seed + recipientRandHash
	params2 := r.TicketParams(sender)

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
}
