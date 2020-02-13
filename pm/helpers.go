package pm

import (
	"math/rand"

	ethcommon "github.com/ethereum/go-ethereum/common"
)

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
