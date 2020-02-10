package pm

import (
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/livepeer/go-livepeer/crypto"
)

// SigVerifier is an interface which describes an object capable
// of verification of ECDSA signatures produced by ETH addresses
type SigVerifier interface {
	// Verify checks if a provided signature over a message
	// is valid for a given ETH address
	Verify(addr ethcommon.Address, msg, sig []byte) bool
}

// DefaultSigVerifier is client-side-only implementation of sig verification, i.e. not relying on
// any smart contract inputs.
type DefaultSigVerifier struct {
}

// Verify checks if a provided signature over a message
// is valid for a given ETH address
func (sv *DefaultSigVerifier) Verify(addr ethcommon.Address, msg, sig []byte) bool {
	return crypto.VerifySig(addr, msg, sig)
}

// ApprovedSigVerifier is an implementation of the SigVerifier interface
// that relies on an implementation of the Broker interface to provide a registry
// mapping ETH addresses to approved signer sets. This implementation will
// recover a ETH address from a signature and check if the recovered address
// is approved
// type ApprovedSigVerifier struct {
// 	broker Broker
// }

// // NewApprovedSigVerifier returns an instance of an approved signature verifier
// func NewApprovedSigVerifier(broker Broker) *ApprovedSigVerifier {
// 	return &ApprovedSigVerifier{
// 		broker: broker,
// 	}
// }

// // Verify checks if a provided signature over a message
// // is valid for a given ETH address
// func (sv *ApprovedSigVerifier) Verify(addr ethcommon.Address, msg, sig []byte) bool {
// 	personalMsg := fmt.Sprintf("\x19Ethereum Signed Message:\n%d%s", 32, msg)
// 	personalHash := crypto.Keccak256([]byte(personalMsg))

// 	pubkey, err := crypto.SigToPub(personalHash, sig)
// 	if err != nil {
// 		return false
// 	}

// 	rec := crypto.PubkeyToAddress(*pubkey)

// 	if addr == rec {
// 		// If recovered address matches, return early
// 		return true
// 	}

// 	approved, err := sv.broker.IsApprovedSigner(addr, rec)
// 	if err != nil {
// 		return false
// 	}

// 	return approved
// }
