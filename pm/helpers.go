package pm

import (
	"fmt"
	"math/rand"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

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

func RandHash() ethcommon.Hash {
	return ethcommon.BytesToHash(RandBytes(32))
}

func RandAddress() ethcommon.Address {
	return ethcommon.BytesToAddress(RandBytes(addressSize))
}

func RandBytes(size uint) []byte {
	x := make([]byte, size, size)
	for i := 0; i < len(x); i++ {
		x[i] = byte(rand.Uint32())
	}
	return x
}
