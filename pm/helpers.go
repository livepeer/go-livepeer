package pm

import (
	"crypto/rand"
	"fmt"
	"testing"

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

func RandHashOrFatal(t *testing.T) ethcommon.Hash {
	key, err := randBytes(32)

	if err != nil {
		t.Fatalf("failed generating random hash: %v", err)
		return ethcommon.Hash{}
	}

	return ethcommon.BytesToHash(key[:])
}

func RandAddressOrFatal(t *testing.T) ethcommon.Address {
	key, err := randBytes(addressSize)

	if err != nil {
		t.Fatalf("failed generating random address: %v", err)
		return ethcommon.Address{}
	}

	return ethcommon.BytesToAddress(key[:])
}

func RandBytesOrFatal(size int, t *testing.T) []byte {
	res, err := randBytes(size)

	if err != nil {
		t.Fatalf("failed generating random bytes: %v", err)
		return nil
	}

	return res
}

func randBytes(size int) ([]byte, error) {
	key := make([]byte, size)
	_, err := rand.Read(key)

	return key, err
}
