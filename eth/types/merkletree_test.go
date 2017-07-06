package types

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

func TestVerifyProof(t *testing.T) {
	hashes := []common.Hash{
		common.BytesToHash([]byte{5}),
		common.BytesToHash([]byte{6}),
		common.BytesToHash([]byte{7}),
		common.BytesToHash([]byte{8}),
		common.BytesToHash([]byte{9}),
	}

	root, proofs, err := NewMerkleTree(hashes)

	if err != nil {
		t.Fatalf("Failed to create merkle tree: %v", err)
	}

	for i, hash := range hashes {
		ok := VerifyProof(root.Hash, hash, proofs[i])

		if !ok {
			t.Fatalf("Failed to verify merkle proof for %v: %v", hash.Hex(), err)
		}
	}
}
