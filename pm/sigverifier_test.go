package pm

// func TestVerify(t *testing.T) {
// 	msg := []byte("foo")
// 	personalMsg := fmt.Sprintf("\x19Ethereum Signed Message:\n%d%s", 32, msg)
// 	personalHash := crypto.Keccak256([]byte(personalMsg))

// 	senderPrivKey, err := crypto.GenerateKey()
// 	if err != nil {
// 		t.Fatal(err)
// 	}

// 	sender := crypto.PubkeyToAddress(senderPrivKey.PublicKey)

// 	senderSig, err := crypto.Sign(personalHash, senderPrivKey)
// 	if err != nil {
// 		t.Fatal(err)
// 	}

// 	unapprovedSignerPrivKey, err := crypto.GenerateKey()
// 	if err != nil {
// 		t.Fatal(err)
// 	}

// 	unapprovedSignerSig, err := crypto.Sign(personalHash, unapprovedSignerPrivKey)

// 	b := newStubBroker()

// 	sv := NewApprovedSigVerifier(b)

// 	// Test invalid signature for non-approved signer
// 	if valid := sv.Verify(sender, msg, unapprovedSignerSig); valid {
// 		t.Error("expected invalid signature for non-approved signer")
// 	}

// 	// Test valid signature for approved signer
// 	approvedSignerPrivKey, err := crypto.GenerateKey()
// 	if err != nil {
// 		t.Fatal(err)
// 	}

// 	approvedSigner := crypto.PubkeyToAddress(approvedSignerPrivKey.PublicKey)

// 	approvedSignerSig, err := crypto.Sign(personalHash, approvedSignerPrivKey)
// 	if err != nil {
// 		t.Fatal(err)
// 	}

// 	// Approve signer
// 	b.ApproveSigners([]ethcommon.Address{approvedSigner})

// 	if valid := sv.Verify(sender, msg, approvedSignerSig); !valid {
// 		t.Error("expected valid signature for approved signer")
// 	}

// 	// Test valid signature for sender
// 	if valid := sv.Verify(sender, msg, senderSig); !valid {
// 		t.Error("expected valid signature for sender")
// 	}
// }
