package server

import (
	"fmt"
	"math/big"
	"os"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/accounts/keystore"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/livepeer/go-livepeer/crypto"
	"github.com/livepeer/go-livepeer/eth"
)

type SignatureValid = bool

// We create one wallet for B and O nodes. Used for signing media data.
// In production this is configured properly to work on-chain.
// In our tests we use TestWallet.CreatePublicInfo() to pass to other nodes constructors.
//   In production other nodes get to this info from chain.
type TestWallet struct {
	keystoreDir    string
	keystore       *keystore.KeyStore
	account        accounts.Account
	accountManager eth.AccountManager
}

// This struct is used in test to verify peer's signature. This info is public and can be used to verify signature.
type SignatureChecker struct {
	account accounts.Account
}

func (s *TestWallet) SignMediaData(bytes []byte) ([]byte, error) {
	return s.accountManager.Sign(bytes)
}

func (s *SignatureChecker) Check(media, signature []byte) SignatureValid {
	return crypto.VerifySig(s.account.Address, media, signature)
}

// Used by testing code to give public info to everyone, off chain
func (s *TestWallet) CreatePublicInfo() SignatureChecker {
	return SignatureChecker{s.account}
}

// Used by testing code to create crypto identity for B & O
func newTestWallet() TestWallet {
	wallet := TestWallet{}
	wallet.Init()
	return wallet
}

func (s *TestWallet) Init() {
	// I have seen this in unit tests. No idea if its correct.
	var err error
	s.keystoreDir, s.keystore = createTempKeystore()
	s.account, err = s.keystore.NewAccount("") //"foo")
	if err != nil {
		fmt.Printf("Failed to create account: %v\n", err)
		os.Exit(-1)
	}
	s.accountManager, err = eth.NewAccountManager(s.account.Address, s.keystoreDir, big.NewInt(777), "")
	if err != nil {
		fmt.Printf("Failed to create account manager: %v\n", err)
		os.Exit(-1)
	}
	err = s.accountManager.Unlock("") //"foo")
	if err != nil {
		fmt.Printf("Failed to unlock account manager: %v\n", err)
		os.Exit(-1)
	}
}

func createTempKeystore() (string, *keystore.KeyStore) {
	dir := os.TempDir()
	new := func(kdir string) *keystore.KeyStore {
		return keystore.NewKeyStore(kdir, keystore.LightScryptN, keystore.LightScryptP)
	}
	return dir, new(dir)
}

// Instead of using `hash := crypto.Keccak256(data)` we call StreamingHash.Append(partialData) multiple times
type StreamingHash struct {
	hashMachine ethcrypto.KeccakState
}

func (h *StreamingHash) Init() {
	h.hashMachine = ethcrypto.NewKeccakState()
}

func (h *StreamingHash) Append(data []byte) {
	// Why would we ignore return values?
	// n, err :=
	h.hashMachine.Write(data)
}

func (h *StreamingHash) GetHash() []byte {
	// Hash is 32 bytes. Signature should be 64 bytes.
	hash := make([]byte, 32)
	h.hashMachine.Read(hash)
	return hash
}
