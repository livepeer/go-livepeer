package pm

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"math/big"
	"testing"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

func TestReceiveTicket(t *testing.T) {
	sender := ethcommon.HexToAddress("A69cdA26600c155cF2c150964Bdb5371ac3f606F")
	b := newStubBroker()
	v := &stubValidator{}
	ts := newStubTicketStore()
	secret := [32]byte{3}
	faceValue := big.NewInt(100)
	winProb := big.NewInt(100)
	sig := []byte("foo")

	r := NewRecipient(b, v, ts, secret, faceValue, winProb)

	// Config stub validator with valid non-winning tickets
	v.SetIsValidTicket(true)

	params, err := r.TicketParams(sender)
	if err != nil {
		t.Fatal(err)
	}

	// Test invalid recipientRand from seed (invalid seed)
	ticket := &Ticket{
		Recipient:         ethcommon.Address{},
		Sender:            sender,
		FaceValue:         faceValue,
		WinProb:           winProb,
		SenderNonce:       0,
		RecipientRandHash: params.RecipientRandHash,
	}

	// Using invalid seed
	_, err = r.ReceiveTicket(ticket, sig, big.NewInt(12312))
	if err == nil {
		t.Error("expected invalid recipientRand from seed error")
	}
	if err != nil && err != errInvalidRecipientRandFromSeed {
		t.Errorf("expected invalid recipientRand from seed error, got %v", err)
	}

	// Test invalid recipientRand from seed (invalid sender)
	ticket = &Ticket{
		Recipient:         ethcommon.Address{},
		Sender:            ethcommon.Address{}, // Using invalid sender
		FaceValue:         faceValue,
		WinProb:           winProb,
		SenderNonce:       0,
		RecipientRandHash: params.RecipientRandHash,
	}

	_, err = r.ReceiveTicket(ticket, sig, params.Seed)
	if err == nil {
		t.Error("expected invalid recipientRand from seed error")
	}
	if err != nil && err != errInvalidRecipientRandFromSeed {
		t.Errorf("expected invalid recipientRand from seed error, got %v", err)
	}

	// Test invalid faceValue
	ticket = &Ticket{
		Recipient:         ethcommon.Address{},
		Sender:            sender,
		FaceValue:         big.NewInt(0), // Using invalid faceValue
		WinProb:           winProb,
		SenderNonce:       0,
		RecipientRandHash: params.RecipientRandHash,
	}

	_, err = r.ReceiveTicket(ticket, sig, params.Seed)
	if err == nil {
		t.Error("expected invalid faceValue error")
	}
	if err != nil && err != errInvalidTicketFaceValue {
		t.Errorf("expected invalid faceValue error, got %v", err)
	}

	// Test invalid winProb
	ticket = &Ticket{
		Recipient:         ethcommon.Address{},
		Sender:            sender,
		FaceValue:         faceValue,
		WinProb:           big.NewInt(0), // Using invalid winProb
		SenderNonce:       0,
		RecipientRandHash: params.RecipientRandHash,
	}

	_, err = r.ReceiveTicket(ticket, sig, params.Seed)
	if err == nil {
		t.Error("expected invalid winProb error")
	}
	if err != nil && err != errInvalidTicketWinProb {
		t.Errorf("expected invalid winProb error, got %v", err)
	}

	// Test invalid ticket
	// Config stub validator with invalid non-winning tickets
	v.SetIsValidTicket(false)
	ticket = &Ticket{
		Recipient:         ethcommon.Address{},
		Sender:            sender,
		FaceValue:         faceValue,
		WinProb:           winProb,
		SenderNonce:       0,
		RecipientRandHash: params.RecipientRandHash,
	}

	if _, err := r.ReceiveTicket(ticket, sig, params.Seed); err == nil {
		t.Error("expected invalid ticket error")
	}

	// Test valid non-winning ticket
	// Config stub validator with valid non-winning tickets
	v.SetIsValidTicket(true)

	won, err := r.ReceiveTicket(ticket, sig, params.Seed)
	if err != nil {
		t.Error(err)
	}
	if won {
		t.Errorf("expected valid non-winning ticket")
	}

	// Test valid winning ticket
	// Config stub validator with valid winnning tickets
	v.SetIsWinningTicket(true)
	ticket = &Ticket{
		Recipient:         ethcommon.Address{},
		Sender:            sender,
		FaceValue:         faceValue,
		WinProb:           winProb,
		SenderNonce:       1,
		RecipientRandHash: params.RecipientRandHash,
	}

	won, err = r.ReceiveTicket(ticket, sig, params.Seed)
	if err != nil {
		t.Error(err)
	}
	if !won {
		t.Errorf("expected valid winning ticket")
	}

	storeTicket, storeSig, storeRecipientRand, err := ts.Load(ticket.Hash())
	if err != nil {
		t.Fatal(err)
	}

	if storeTicket.Hash() != ticket.Hash() {
		t.Errorf("expected store ticket hash %v, got %v", ticket.Hash(), storeTicket.Hash())
	}

	if hex.EncodeToString(storeSig) != hex.EncodeToString(sig) {
		t.Errorf("expected store sig 0x%x, got 0x%x", sig, storeSig)
	}

	if crypto.Keccak256Hash(ethcommon.LeftPadBytes(storeRecipientRand.Bytes(), uint256Size)) != ticket.RecipientRandHash {
		t.Error("expected store recipientRand to match ticket recipientRandHash")
	}

	// Test invalid recipientRand revealed
	// Config stub broker with zero deposit and non-zero penalty escrow
	b.SetDeposit(sender, big.NewInt(0))
	b.SetPenaltyEscrow(sender, big.NewInt(500))
	// Redeem ticket to invalidate recipientRand
	if err := r.RedeemWinningTicket(ticket.Hash()); err != nil {
		t.Fatal(err)
	}

	// New ticket with same invalid recipientRand, but updated senderNonce
	ticket = &Ticket{
		Recipient:         ethcommon.Address{},
		Sender:            sender,
		FaceValue:         faceValue,
		WinProb:           winProb,
		SenderNonce:       1,
		RecipientRandHash: params.RecipientRandHash,
	}

	_, err = r.ReceiveTicket(ticket, sig, params.Seed)
	if err == nil {
		t.Error("expected invalid recipientRand revealed error")
	}
	if err != nil && err != errInvalidRecipientRandRevealed {
		t.Errorf("expected invalid recipientRand revealed error, got %v", err)
	}

	// Test invalid senderNonce
	params, err = r.TicketParams(sender)
	if err != nil {
		t.Fatal(err)
	}

	// Receive senderNonce = 0
	ticket0 := &Ticket{
		Recipient:         ethcommon.Address{},
		Sender:            sender,
		FaceValue:         faceValue,
		WinProb:           winProb,
		SenderNonce:       0,
		RecipientRandHash: params.RecipientRandHash,
	}

	_, err = r.ReceiveTicket(ticket0, sig, params.Seed)
	if err != nil {
		t.Fatal(err)
	}

	// Receive senderNonce = 1
	ticket1 := &Ticket{
		Recipient:         ethcommon.Address{},
		Sender:            sender,
		FaceValue:         faceValue,
		WinProb:           winProb,
		SenderNonce:       1,
		RecipientRandHash: params.RecipientRandHash,
	}

	_, err = r.ReceiveTicket(ticket1, sig, params.Seed)
	if err != nil {
		t.Fatal(err)
	}

	// Replay senderNonce = 1 (new nonce = highest seen nonce)
	_, err = r.ReceiveTicket(ticket1, sig, params.Seed)
	if err == nil {
		t.Error("expected invalid senderNonce (new nonce = highest seen nonce) error")
	}
	if err != nil && err != errInvalidTicketSenderNonce {
		t.Errorf("expected invalid senderNonce (new nonce = highest seen nonce) error, got %v", err)
	}

	// Replay senderNonce = 0 (new nonce < highest seen nonce)
	_, err = r.ReceiveTicket(ticket0, sig, params.Seed)
	if err == nil {
		t.Error("expected invalid senderNonce (new nonce < highest seen nonce) error")
	}
	if err != nil && err != errInvalidTicketSenderNonce {
		t.Errorf("expected invalid senderNonce (new nonce < highest seen nonce) error, got %v", err)
	}
}

func TestRedeemWinningTicket(t *testing.T) {
	sender := ethcommon.HexToAddress("A69cdA26600c155cF2c150964Bdb5371ac3f606F")
	b := newStubBroker()
	v := &stubValidator{}
	ts := newStubTicketStore()
	secret := [32]byte{3}
	faceValue := big.NewInt(100)
	winProb := big.NewInt(100)
	sig := []byte("foo")

	r := NewRecipient(b, v, ts, secret, faceValue, winProb)

	params1, err := r.TicketParams(sender)
	if err != nil {
		t.Fatal(err)
	}

	// Config stub validator with valid winning tickets
	v.SetIsValidTicket(true)
	v.SetIsWinningTicket(true)
	// Config stub broker with zero deposit and penalty escrow
	b.SetDeposit(sender, big.NewInt(0))
	b.SetPenaltyEscrow(sender, big.NewInt(0))

	// Test zero deposit and penalty escrow error
	ticket := &Ticket{
		Recipient:         ethcommon.Address{},
		Sender:            sender,
		FaceValue:         faceValue,
		WinProb:           winProb,
		SenderNonce:       0,
		RecipientRandHash: params1.RecipientRandHash,
	}

	won, err := r.ReceiveTicket(ticket, sig, params1.Seed)
	if err != nil {
		t.Fatal(err)
	}
	if !won {
		t.Fail()
	}

	err = r.RedeemWinningTicket(ticket.Hash())
	if err == nil {
		t.Error("expected zero deposit and penalty escrow error")
	}
	if err != nil && err != errZeroDepositAndPenaltyEscrow {
		t.Errorf("expected zero deposit and penalty escrow error, got %v", err)
	}

	// Test zero deposit and non-zero penalty escrow
	// Config stub broker with zero deposit and non-zero penalty escrow
	b.SetDeposit(sender, big.NewInt(0))
	b.SetPenaltyEscrow(sender, big.NewInt(500))

	if err := r.RedeemWinningTicket(ticket.Hash()); err != nil {
		t.Error(err)
	}

	used, err := b.IsUsedTicket(ticket)
	if err != nil {
		t.Fatal(err)
	}
	if !used {
		t.Errorf("expected used ticket for sender with zero deposit and non-zero penalty escrow")
	}

	// Test non-zero deposit and zero penalty escrow
	// Config stub broker with non-zero deposit and zero penalty escrow
	b.SetDeposit(sender, big.NewInt(500))
	b.SetPenaltyEscrow(sender, big.NewInt(0))

	params2, err := r.TicketParams(sender)
	if err != nil {
		t.Fatal(err)
	}

	ticket = &Ticket{
		Recipient:         ethcommon.Address{},
		Sender:            sender,
		FaceValue:         faceValue,
		WinProb:           winProb,
		SenderNonce:       0,
		RecipientRandHash: params2.RecipientRandHash,
	}

	won, err = r.ReceiveTicket(ticket, sig, params2.Seed)
	if err != nil {
		t.Fatal(err)
	}
	if !won {
		t.Fatal()
	}

	if err := r.RedeemWinningTicket(ticket.Hash()); err != nil {
		t.Error(err)
	}

	used, err = b.IsUsedTicket(ticket)
	if err != nil {
		t.Fatal(err)
	}
	if !used {
		t.Errorf("expected used ticket for sender with non-zero deopsit and zero penalty escrow")
	}

	// Test invalid recipientRand revealed error
	// This should fail because we use the same ticket with a recipientRand
	// already invalidated
	err = r.RedeemWinningTicket(ticket.Hash())
	if err == nil {
		t.Error("expected invalid recipientRand revealed error")
	}
	if err != nil && err != errInvalidRecipientRandRevealed {
		t.Errorf("expected invalid recipientRand revealed error, got %v", err)
	}

	// TODO: Unit test for internal clearing of senderNonce memory?
}

func TestTicketParams(t *testing.T) {
	sender := ethcommon.HexToAddress("A69cdA26600c155cF2c150964Bdb5371ac3f606F")
	b := newStubBroker()
	v := &stubValidator{}
	ts := newStubTicketStore()
	secret := [32]byte{3}
	faceValue := big.NewInt(100)
	winProb := big.NewInt(100)

	r := NewRecipient(b, v, ts, secret, faceValue, winProb)

	// Test correct params returned
	params1, err := r.TicketParams(sender)
	if err != nil {
		t.Fatal(err)
	}

	if params1.FaceValue.Cmp(faceValue) != 0 {
		t.Errorf("expected faceValue %d got %d", faceValue, params1.FaceValue)
	}

	if params1.WinProb.Cmp(winProb) != 0 {
		t.Errorf("expected winProb %d got %d", winProb, params1.WinProb)
	}

	h := hmac.New(sha256.New, secret[:])
	h.Write(append(params1.Seed.Bytes(), sender.Bytes()...))
	recipientRandHash := crypto.Keccak256Hash(ethcommon.LeftPadBytes(h.Sum(nil), uint256Size))

	if params1.RecipientRandHash != recipientRandHash {
		t.Errorf("expected recipientRandHash %x got %x", recipientRandHash, params1.RecipientRandHash)
	}

	// Test correct params returned and different seed + recipientRandHash
	params2, err := r.TicketParams(sender)
	if err != nil {
		t.Fatal(err)
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

	h = hmac.New(sha256.New, secret[:])
	h.Write(append(params2.Seed.Bytes(), sender.Bytes()...))
	recipientRandHash = crypto.Keccak256Hash(ethcommon.LeftPadBytes(h.Sum(nil), uint256Size))

	if params2.RecipientRandHash != recipientRandHash {
		t.Errorf("expected recipientRandHash %x got %x", recipientRandHash, params2.RecipientRandHash)
	}
}
