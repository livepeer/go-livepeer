package types

import (
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

var (
	ErrDuplicatedHash = fmt.Errorf("MerkleTree: duplicated hash provided")
)

type MerkleTreeNode struct {
	Hash     common.Hash
	Parent   *MerkleTreeNode
	LeftSib  *MerkleTreeNode
	RightSib *MerkleTreeNode
}

func (n *MerkleTreeNode) String() string {
	return fmt.Sprintf("Hash: %v", n.Hash.Hex())
}

type MerkleProof struct {
	Hashes []common.Hash
}

func (p *MerkleProof) Bytes() []byte {
	var proofBytes []byte

	for _, hash := range p.Hashes {
		proofBytes = append(proofBytes, hash.Bytes()...)
	}

	return proofBytes
}

func NewMerkleTree(hashes []common.Hash) (*MerkleTreeNode, []*MerkleProof, error) {
	if !checkHashDups(hashes) {
		return nil, nil, ErrDuplicatedHash
	}

	leaves, root := buildMerkleTree(hashes)
	proofs := make([]*MerkleProof, len(hashes))

	for i, leaf := range leaves {
		proofs[i] = NewMerkleProof(leaf)
	}

	return root, proofs, nil
}

func NewMerkleProof(node *MerkleTreeNode) *MerkleProof {
	var hashes []common.Hash

	for node != nil {
		if node.LeftSib != nil {
			hashes = append(hashes, node.LeftSib.Hash)
		} else if node.RightSib != nil {
			hashes = append(hashes, node.RightSib.Hash)
		}

		node = node.Parent
	}

	return &MerkleProof{
		hashes,
	}
}

func buildMerkleTree(hashes []common.Hash) ([]*MerkleTreeNode, *MerkleTreeNode) {
	switch len(hashes) {
	case 0:
		return nil, nil
	case 1:
		leaf := &MerkleTreeNode{hashes[0], nil, nil, nil}
		return []*MerkleTreeNode{leaf}, leaf
	default:
		firstLeaves, firstChild := buildMerkleTree(hashes[:(len(hashes)+1)/2])
		secondLeaves, secondChild := buildMerkleTree(hashes[(len(hashes)+1)/2:])

		root := &MerkleTreeNode{combinedHash(firstChild.Hash, secondChild.Hash), nil, nil, nil}

		firstChild.Parent = root
		secondChild.Parent = root

		if hashCmp(firstChild.Hash, secondChild.Hash) == -1 {
			firstChild.RightSib = secondChild
			secondChild.LeftSib = firstChild
		} else {
			firstChild.LeftSib = secondChild
			secondChild.RightSib = firstChild
		}

		return append(firstLeaves, secondLeaves...), root
	}
}

func combinedHash(first common.Hash, second common.Hash) common.Hash {
	if hashCmp(first, second) == -1 {
		return common.BytesToHash(crypto.Keccak256(first.Bytes(), second.Bytes()))
	}
	return common.BytesToHash(crypto.Keccak256(second.Bytes(), first.Bytes()))
}

func checkHashDups(hashes []common.Hash) bool {
	hashMap := make(map[common.Hash]bool)

	for _, hash := range hashes {
		if hashMap[hash] {
			return false
		}
		hashMap[hash] = true
	}

	return true
}

func hashCmp(h1 common.Hash, h2 common.Hash) int {
	return strings.Compare(h1.Hex(), h2.Hex())
}

func VerifyProof(rootHash common.Hash, hash common.Hash, proof *MerkleProof) bool {
	currHash := hash

	for _, h := range proof.Hashes {
		if hashCmp(currHash, h) == -1 {
			currHash = common.BytesToHash(crypto.Keccak256(currHash.Bytes(), h.Bytes()))
		} else {
			currHash = common.BytesToHash(crypto.Keccak256(h.Bytes(), currHash.Bytes()))
		}
	}

	return currHash == rootHash
}
