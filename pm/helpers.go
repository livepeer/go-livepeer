package pm

import (
	"fmt"
	"math/rand"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

// VerifySig verifies that a ETH ECDSA signature over a given message
// is produced by a given ETH address
//
// TODO refactor to a package that both eth and pm can import
func VerifySig(addr ethcommon.Address, msg, sig []byte) bool {
	personalMsg := fmt.Sprintf("\x19Ethereum Signed Message:\n%d%s", 32, msg)
	personalHash := crypto.Keccak256([]byte(personalMsg))

	pubkey, err := crypto.SigToPub(personalHash, sig)
	if err != nil {
		return false
	}

	return crypto.PubkeyToAddress(*pubkey) == addr
}

// RandHash returns a random keccak256 hash
func RandHash() ethcommon.Hash {
	return ethcommon.BytesToHash(RandBytes(32))
}

// RandAddress returns a random ETH address
func RandAddress() ethcommon.Address {
	return ethcommon.BytesToAddress(RandBytes(addressSize))
}

// RandBytes returns a slice of random bytes with the size specified by the caller
func RandBytes(size uint) []byte {
	x := make([]byte, size, size)
	for i := 0; i < len(x); i++ {
		x[i] = byte(rand.Uint32())
	}
	return x
}
