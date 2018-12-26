package pm

import "github.com/ethereum/go-ethereum/accounts"

// Signer supports identifying as an Ethereum account owner, by providing the
// Account and enabling message signing.
type Signer interface {
	Sign(msg []byte) ([]byte, error)
	Account() accounts.Account
}
