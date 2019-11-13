package pm

import (
	"math/big"
	"math/rand"

	"github.com/ethereum/go-ethereum/accounts"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/pkg/errors"
)

var (
	secp256k1N, _  = new(big.Int).SetString("fffffffffffffffffffffffffffffffebaaedce6af48a03bbfd25e8cd0364141", 16)
	secp256k1halfN = new(big.Int).Div(secp256k1N, big.NewInt(2))
)

// VerifySig verifies that a ETH ECDSA signature over a given message
// is produced by a given ETH address
//
// TODO refactor to a package that both eth and pm can import
func VerifySig(addr ethcommon.Address, msg, sig []byte) bool {
	recovered, err := ecrecover(msg, sig)
	if err != nil {
		return false
	}

	return recovered == addr
}

func ecrecover(msg, sig []byte) (ethcommon.Address, error) {
	if len(sig) != 65 {
		return ethcommon.Address{}, errors.New("invalid signature length")
	}

	s := new(big.Int).SetBytes(sig[32:64])
	if s.Cmp(secp256k1halfN) > 0 {
		return ethcommon.Address{}, errors.New("signature s value too high")
	}

	v := sig[64]
	if v != byte(27) && v != byte(28) {
		return ethcommon.Address{}, errors.New("signature v value must be 27 or 28")
	}

	// crypto.SigToPub() expects signature v value = 0/1
	// Copy the signature and convert its value to 0/1
	ethSig := make([]byte, 65)
	copy(ethSig[:], sig[:])
	ethSig[64] -= 27

	ethMsg := accounts.TextHash(msg)
	pubkey, err := crypto.SigToPub(ethMsg, ethSig)
	if err != nil {
		return ethcommon.Address{}, err
	}

	return crypto.PubkeyToAddress(*pubkey), nil
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
