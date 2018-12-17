package pm

import (
	"math/big"
	"testing"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

func TestIsValidTicket(t *testing.T) {
	recipient := ethcommon.HexToAddress("73AEd7b5dEb30222fa896f399d46cC99c7BEe57F")
	sender := ethcommon.HexToAddress("A69cdA26600c155cF2c150964Bdb5371ac3f606F")
	sig := []byte("foo")
	recipientRand := big.NewInt(10)
	recipientRandHash := crypto.Keccak256Hash(ethcommon.LeftPadBytes(recipientRand.Bytes(), uint256Size))

	b := &stubBroker{
		usedTickets: make(map[ethcommon.Hash]bool),
	}
	sv := &stubSigVerifier{
		shouldVerify: true,
	}

	v := NewValidator(recipient, b, sv)

	// Test invalid recipient (null address)
	ticket := &Ticket{
		Recipient:         ethcommon.Address{},
		Sender:            ethcommon.Address{},
		FaceValue:         big.NewInt(0),
		WinProb:           big.NewInt(0),
		SenderNonce:       0,
		RecipientRandHash: recipientRandHash,
	}

	valid, err := v.IsValidTicket(ticket, sig, recipientRand)
	if err == nil || valid {
		t.Fatal("expected invalid recipient (null address)")
	}

	// Test invalid recipient (non-null address)
	ticket = &Ticket{
		Recipient:         sender,
		Sender:            ethcommon.Address{},
		FaceValue:         big.NewInt(0),
		WinProb:           big.NewInt(0),
		SenderNonce:       0,
		RecipientRandHash: recipientRandHash,
	}

	valid, err = v.IsValidTicket(ticket, sig, recipientRand)
	if err == nil || valid {
		t.Fatal("expected invalid recipient (non-null address)")
	}

	// Test invalid sender
	ticket = &Ticket{
		Recipient:         recipient,
		Sender:            ethcommon.Address{},
		FaceValue:         big.NewInt(0),
		WinProb:           big.NewInt(0),
		SenderNonce:       0,
		RecipientRandHash: recipientRandHash,
	}

	valid, err = v.IsValidTicket(ticket, sig, recipientRand)
	if err == nil || valid {
		t.Fatal("expected invalid sender")
	}

	// Test invalid preimage for commitment recipientRandHash
	ticket = &Ticket{
		Recipient:         recipient,
		Sender:            sender,
		FaceValue:         big.NewInt(0),
		WinProb:           big.NewInt(0),
		SenderNonce:       0,
		RecipientRandHash: ethcommon.Hash{},
	}

	valid, err = v.IsValidTicket(ticket, sig, recipientRand)
	if err == nil || valid {
		t.Fatal("expected invalid preimage for commitment recipientRandHash")
	}

	// Test used ticket
	ticket = &Ticket{
		Recipient:         recipient,
		Sender:            sender,
		FaceValue:         big.NewInt(0),
		WinProb:           big.NewInt(0),
		SenderNonce:       0,
		RecipientRandHash: recipientRandHash,
	}
	// Set ticket as used in stub broker
	b.RedeemWinningTicket(ticket, sig, recipientRand)

	valid, err = v.IsValidTicket(ticket, sig, recipientRand)
	if err == nil || valid {
		t.Fatal("expected used ticket")
	}

	// Test invalid signature
	// Create ticket with new senderNonce
	ticket = &Ticket{
		Recipient:         recipient,
		Sender:            sender,
		FaceValue:         big.NewInt(0),
		WinProb:           big.NewInt(0),
		SenderNonce:       1,
		RecipientRandHash: recipientRandHash,
	}
	// Set signature verification to return false
	sv.shouldVerify = false

	valid, err = v.IsValidTicket(ticket, sig, recipientRand)
	if err == nil || valid {
		t.Fatal("expected invalid signature")
	}

	// Test valid ticket
	// Set signature verification to return true
	sv.shouldVerify = true

	valid, err = v.IsValidTicket(ticket, sig, recipientRand)
	if err == nil || valid {
		t.Fatal("expected valid ticket")
	}
}

func TestIsWinningTicket(t *testing.T) {
	recipient := ethcommon.HexToAddress("73AEd7b5dEb30222fa896f399d46cC99c7BEe57F")
	sender := ethcommon.HexToAddress("A69cdA26600c155cF2c150964Bdb5371ac3f606F")
	sig := []byte("foo")
	recipientRand := big.NewInt(10)
	recipientRandHash := crypto.Keccak256Hash(ethcommon.LeftPadBytes(recipientRand.Bytes(), uint256Size))

	b := &stubBroker{
		usedTickets: make(map[ethcommon.Hash]bool),
	}
	sv := &stubSigVerifier{
		shouldVerify: true,
	}

	v := NewValidator(recipient, b, sv)

	// Test non-winning ticket
	ticket := &Ticket{
		Recipient:         recipient,
		Sender:            sender,
		FaceValue:         big.NewInt(0),
		WinProb:           big.NewInt(0),
		SenderNonce:       0,
		RecipientRandHash: recipientRandHash,
	}

	if v.IsWinningTicket(ticket, sig, recipientRand) {
		t.Fatal("expected non-winning ticket")
	}

	// Test winning ticket
	maxUint256 := new(big.Int).Sub(new(big.Int).Exp(big.NewInt(2), big.NewInt(256), nil), big.NewInt(1))
	ticket = &Ticket{
		Recipient:         recipient,
		Sender:            sender,
		FaceValue:         big.NewInt(0),
		WinProb:           maxUint256,
		SenderNonce:       0,
		RecipientRandHash: recipientRandHash,
	}

	if !v.IsWinningTicket(ticket, sig, recipientRand) {
		t.Fatal("expected winning ticket")
	}
}
