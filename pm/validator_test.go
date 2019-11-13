package pm

import (
	"math/big"
	"testing"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/assert"
)

func TestValidateTicket(t *testing.T) {
	recipient := ethcommon.HexToAddress("73AEd7b5dEb30222fa896f399d46cC99c7BEe57F")
	sender := ethcommon.HexToAddress("A69cdA26600c155cF2c150964Bdb5371ac3f606F")
	sig := []byte("foo")
	recipientRand := big.NewInt(10)
	recipientRandHash := crypto.Keccak256Hash(ethcommon.LeftPadBytes(recipientRand.Bytes(), uint256Size))

	sv := &stubSigVerifier{}
	sv.SetVerifyResult(true)

	rm := &stubRoundsManager{}

	v := NewValidator(sv, rm)

	// Test invalid recipient (null address)
	ticket := &Ticket{
		Recipient:         ethcommon.Address{},
		Sender:            ethcommon.Address{},
		FaceValue:         big.NewInt(0),
		WinProb:           big.NewInt(0),
		SenderNonce:       0,
		RecipientRandHash: recipientRandHash,
	}

	err := v.ValidateTicket(recipient, ticket, sig, recipientRand)
	if err == nil {
		t.Error("expected invalid recipient (null address) error")
	}
	if err != nil && err != errInvalidTicketRecipient {
		t.Errorf("expected invalid recipient (null address) error, got %v", err)
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

	err = v.ValidateTicket(recipient, ticket, sig, recipientRand)
	if err == nil {
		t.Error("expected invalid recipient (non-null address) error")
	}
	if err != nil && err != errInvalidTicketRecipient {
		t.Errorf("expected invalid recipient (non-null address) error, got %v", err)
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

	err = v.ValidateTicket(recipient, ticket, sig, recipientRand)
	if err == nil {
		t.Error("expected invalid sender error")
	}
	if err != nil && err != errInvalidTicketSender {
		t.Errorf("expected invalid sender error, got %v", err)
	}

	// Test invalid recipientRand for recipientRandHash
	ticket = &Ticket{
		Recipient:         recipient,
		Sender:            sender,
		FaceValue:         big.NewInt(0),
		WinProb:           big.NewInt(0),
		SenderNonce:       0,
		RecipientRandHash: ethcommon.Hash{},
	}

	err = v.ValidateTicket(recipient, ticket, sig, recipientRand)
	if err == nil {
		t.Error("expected invalid recipientRand for recipientRandHash error")
	}
	if err != nil && err != errInvalidTicketRecipientRand {
		t.Errorf("expected invalid recipientRand for recipientRandHash error, got %v", err)
	}

	// Test invalid signature
	// Set signature verification to return false
	sv.SetVerifyResult(false)

	ticket = &Ticket{
		Recipient:         recipient,
		Sender:            sender,
		FaceValue:         big.NewInt(0),
		WinProb:           big.NewInt(0),
		SenderNonce:       0,
		RecipientRandHash: recipientRandHash,
	}

	err = v.ValidateTicket(recipient, ticket, sig, recipientRand)
	if err == nil {
		t.Error("expected invalid signature error")
	}
	if err != nil && err != errInvalidTicketSignature {
		t.Errorf("expected invalid signature error, got %v", err)
	}

	assert := assert.New(t)

	// Test LastInitializedRound error
	// Set signature verification to return true
	sv.SetVerifyResult(true)

	rm.round = big.NewInt(7)

	creationRound := big.NewInt(5)
	ticket = &Ticket{
		Recipient:         recipient,
		Sender:            sender,
		FaceValue:         big.NewInt(0),
		WinProb:           big.NewInt(0),
		SenderNonce:       0,
		RecipientRandHash: recipientRandHash,
		CreationRound:     creationRound.Int64(),
	}

	err = v.ValidateTicket(recipient, ticket, sig, recipientRand)
	assert.EqualError(err, errInvalidCreationRound.Error())

	// Test BlockHashForRound error
	rm.round = creationRound

	// Test invalid creation round block hash
	rm.blkHash = [32]byte{5}

	creationRoundBlockHash := [32]byte{9}
	ticket = &Ticket{
		Recipient:              recipient,
		Sender:                 sender,
		FaceValue:              big.NewInt(0),
		WinProb:                big.NewInt(0),
		SenderNonce:            0,
		RecipientRandHash:      recipientRandHash,
		CreationRound:          5,
		CreationRoundBlockHash: ethcommon.BytesToHash(creationRoundBlockHash[:]),
	}

	err = v.ValidateTicket(recipient, ticket, sig, recipientRand)
	assert.EqualError(err, errInvalidCreationRoundBlockHash.Error())

	// Test valid ticket
	rm.blkHash = creationRoundBlockHash

	if err := v.ValidateTicket(recipient, ticket, sig, recipientRand); err != nil {
		t.Errorf("expected valid ticket, got error %v", err)
	}
}

func TestIsWinningTicket(t *testing.T) {
	recipient := ethcommon.HexToAddress("73AEd7b5dEb30222fa896f399d46cC99c7BEe57F")
	sender := ethcommon.HexToAddress("A69cdA26600c155cF2c150964Bdb5371ac3f606F")
	sig := []byte("foo")
	recipientRand := big.NewInt(10)
	recipientRandHash := crypto.Keccak256Hash(ethcommon.LeftPadBytes(recipientRand.Bytes(), uint256Size))

	sv := &stubSigVerifier{}
	sv.SetVerifyResult(true)

	rm := &stubRoundsManager{}

	v := NewValidator(sv, rm)

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
		t.Error("expected non-winning ticket")
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
		t.Error("expected winning ticket")
	}
}
