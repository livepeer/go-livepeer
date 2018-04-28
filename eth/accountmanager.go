package eth

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/accounts/keystore"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/console"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/common"
)

var (
	ErrAccountNotFound    = fmt.Errorf("ETH account not found")
	ErrLocked             = fmt.Errorf("account locked")
	ErrPassphraseMismatch = fmt.Errorf("passphrases do not match")
)

type AccountManager struct {
	Account accounts.Account

	unlocked bool
	keyStore *keystore.KeyStore
}

func NewAccountManager(accountAddr ethcommon.Address, keystoreDir string) (*AccountManager, error) {
	keyStore := keystore.NewKeyStore(keystoreDir, keystore.StandardScryptN, keystore.StandardScryptP)

	acctExists := keyStore.HasAddress(accountAddr)
	numAccounts := len(keyStore.Accounts())

	var acct accounts.Account
	var err error
	if numAccounts == 0 || ((accountAddr != ethcommon.Address{}) && !acctExists) {
		glog.Infof("Please create a new ETH account")

		// Account does not exist yet, set it up
		acct, err = createAccount(keyStore)
		if err != nil {
			return nil, err
		}
	} else {
		glog.V(common.SHORT).Infof("Found existing ETH account")

		// Account already exists or defaulting to first, load it from keystore
		acct, err = getAccount(accountAddr, keyStore)
		if err != nil {
			return nil, err
		}
	}

	glog.V(common.SHORT).Infof("Using ETH account: %v", acct.Address.Hex())

	return &AccountManager{
		Account:  acct,
		unlocked: false,
		keyStore: keyStore,
	}, nil
}

// Unlock account indefinitely using underlying keystore
func (am *AccountManager) Unlock(passphrase string) error {
	var err error

	err = am.keyStore.Unlock(am.Account, passphrase)
	if err != nil {
		if passphrase != "" {
			return err
		}
		glog.Infof("Passphrase required to unlock ETH account")

		passphrase, err = getPassphrase(false)
		err = am.keyStore.Unlock(am.Account, passphrase)
		if err != nil {
			return err
		}
	}

	am.unlocked = true

	glog.V(common.SHORT).Infof("ETH account %v unlocked", am.Account.Address.Hex())

	return nil
}

// Lock account using underlying keystore and remove associated private key from memory
func (am *AccountManager) Lock() error {
	err := am.keyStore.Lock(am.Account.Address)
	if err != nil {
		return err
	}

	am.unlocked = false

	return nil
}

// Create transact opts for client use - account must be unlocked
// Can optionally set gas limit and gas price used
func (am *AccountManager) CreateTransactOpts(gasLimit uint64, gasPrice *big.Int) (*bind.TransactOpts, error) {
	if !am.unlocked {
		return nil, ErrLocked
	}

	return &bind.TransactOpts{
		From:     am.Account.Address,
		GasLimit: gasLimit,
		GasPrice: gasPrice,
		Signer: func(signer types.Signer, address ethcommon.Address, tx *types.Transaction) (*types.Transaction, error) {
			if address != am.Account.Address {
				return nil, errors.New("not authorized to sign this account")
			}

			signature, err := am.keyStore.SignHash(am.Account, signer.Hash(tx).Bytes())
			if err != nil {
				return nil, err
			}

			return tx.WithSignature(signer, signature)
		},
	}, nil
}

// Sign byte array message. Account must be unlocked
func (am *AccountManager) Sign(msg []byte) ([]byte, error) {
	personalMsg := fmt.Sprintf("\x19Ethereum Signed Message:\n%d%s", 32, msg)
	personalHash := crypto.Keccak256([]byte(personalMsg))

	return am.keyStore.SignHash(am.Account, personalHash)
}

// Get account from keystore using hex address
// If no hex address is provided, default to the first account
func getAccount(accountAddr ethcommon.Address, keyStore *keystore.KeyStore) (accounts.Account, error) {
	accts := keyStore.Accounts()

	if (accountAddr != ethcommon.Address{}) {
		for _, acct := range accts {
			if acct.Address == accountAddr {
				return acct, nil
			}
		}

		return accounts.Account{}, ErrAccountNotFound
	} else {
		glog.V(common.SHORT).Infof("Defaulting to first ETH account in keystore %v", accts[0].Address.Hex())

		// Default to first account
		return accts[0], nil
	}
}

// Create account in keystore
func createAccount(keyStore *keystore.KeyStore) (accounts.Account, error) {
	passphrase, err := getPassphrase(true)
	if err != nil {
		return accounts.Account{}, err
	}

	return keyStore.NewAccount(passphrase)
}

// Prompt for passphrase
func getPassphrase(shouldConfirm bool) (string, error) {
	passphrase, err := console.Stdin.PromptPassword("Passphrase: ")
	if err != nil {
		return "", err
	}

	if shouldConfirm {
		confirmation, err := console.Stdin.PromptPassword("Repeat passphrase: ")
		if err != nil {
			return "", err
		}

		if passphrase != confirmation {
			return "", ErrPassphraseMismatch
		}
	}

	return passphrase, nil
}
