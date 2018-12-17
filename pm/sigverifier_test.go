package pm

import (
	"fmt"
	"testing"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

func TestVerify(t *testing.T) {
	msg := []byte("foo")
	personalMsg := fmt.Sprintf("\x19Ethereum Signed Message:\n%d%s", 32, msg)
	personalHash := crypto.Keccak256([]byte(personalMsg))

	senderPrivKey, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}

	sender := crypto.PubkeyToAddress(senderPrivKey.PublicKey)

	senderSig, err := crypto.Sign(personalHash, senderPrivKey)
	if err != nil {
		t.Fatal(err)
	}

	signerPrivKey, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}

	signer := crypto.PubkeyToAddress(signerPrivKey.PublicKey)

	signerSig, err := crypto.Sign(personalHash, signerPrivKey)

	b := &stubBroker{
		usedTickets:     make(map[ethcommon.Hash]bool),
		approvedSigners: make(map[ethcommon.Address]bool),
	}

	sv := NewApprovedSigVerifier(b)

	// Test invalid signature for non-approved signer
	valid, err := sv.Verify(sender, signerSig, msg)
	if err != nil {
		t.Fatal(err)
	}

	if valid {
		t.Fatal("expected invalid signature for non-approved signer")
	}

	// Test valid signature for approved signer
	// Approve signer
	b.ApproveSigners([]ethcommon.Address{signer})

	valid, err = sv.Verify(sender, signerSig, msg)
	if err != nil {
		t.Fatal(err)
	}

	if !valid {
		t.Fatal("expected valid signature for approved signer")
	}

	// Test valid signature for sender
	valid, err = sv.Verify(sender, senderSig, msg)
	if err != nil {
		t.Fatal(err)
	}

	if !valid {
		t.Fatal("expected valid signature for sender")
	}
}
