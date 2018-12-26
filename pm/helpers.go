package pm

import (
	"crypto/rand"
	"testing"

	ethcommon "github.com/ethereum/go-ethereum/common"
)

func randHashOrFatal(t *testing.T) ethcommon.Hash {
	key, err := randBytes(32)

	if err != nil {
		t.Fatalf("failed generating random hash: %v", err)
		return ethcommon.Hash{}
	}

	return ethcommon.BytesToHash(key[:])
}

func randAddressOrFatal(t *testing.T) ethcommon.Address {
	key, err := randBytes(addressSize)

	if err != nil {
		t.Fatalf("failed generating random address: %v", err)
		return ethcommon.Address{}
	}

	return ethcommon.BytesToAddress(key[:])
}

func randBytesOrFatal(size int, t *testing.T) []byte {
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
