package eth

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/console"
	"github.com/ethereum/go-ethereum/core/types"
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

func NewAccountManager(accountAddr common.Address, keystoreDir string) (*AccountManager, error) {
	keyStore := keystore.NewKeyStore(keystoreDir, keystore.StandardScryptN, keystore.StandardScryptP)

	acctExists := keyStore.HasAddress(accountAddr)

	var acct accounts.Account
	var err error
	if (accountAddr != common.Address{}) && !acctExists {
		// Account does not exist yet, set it up
		acct, err = createAccount(keyStore)
		if err != nil {
			return nil, err
		}
	} else {
		// Account already exists or defaulting to first, load it from keystore
		acct, err = getAccount(accountAddr, keyStore)
		if err != nil {
			return nil, err
		}
	}

	return &AccountManager{
		Account:  acct,
		unlocked: false,
		keyStore: keyStore,
	}, nil
}

// Unlock account indefinitely using underlying keystore
func (am *AccountManager) Unlock() error {
	passphrase, err := getPassphrase(false)
	if err != nil {
		return err
	}

	err = am.keyStore.Unlock(am.Account, passphrase)
	if err != nil {
		return err
	}

	am.unlocked = true

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
func (am *AccountManager) CreateTransactOpts(gasLimit, gasPrice *big.Int) (*bind.TransactOpts, error) {
	if !am.unlocked {
		return nil, ErrLocked
	}

	return &bind.TransactOpts{
		From: am.Account.Address,
		Signer: func(signer types.Signer, address common.Address, tx *types.Transaction) (*types.Transaction, error) {
			if address != am.Account.Address {
				return nil, errors.New("not authorized to sign this account")
			}

			signature, err := am.Sign(signer.Hash(tx).Bytes())
			if err != nil {
				return nil, err
			}

			return tx.WithSignature(signer, signature)
		},
	}, nil
}

// Sign byte array message. Account must be unlocked
func (am *AccountManager) Sign(msg []byte) ([]byte, error) {
	return am.keyStore.SignHash(am.Account, msg)
}

// Get account from keystore using hex address
// If no hex address is provided, default to the first account
func getAccount(accountAddr common.Address, keyStore *keystore.KeyStore) (accounts.Account, error) {
	accts := keyStore.Accounts()

	if (accountAddr != common.Address{}) {
		for _, acct := range accts {
			if acct.Address == accountAddr {
				return acct, nil
			}
		}

		return accounts.Account{}, ErrAccountNotFound
	} else {
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
