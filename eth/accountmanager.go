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

type AccountManager interface {
	Unlock(passphrase string) error
	Lock() error
	CreateTransactOpts(gasLimit uint64, gasPrice *big.Int) (*bind.TransactOpts, error)
	SignTx(signer types.Signer, tx *types.Transaction) (*types.Transaction, error)
	Sign(msg []byte) ([]byte, error)
	Account() accounts.Account
}

type DefaultAccountManager struct {
	account accounts.Account

	unlocked bool
	keyStore *keystore.KeyStore
}

func NewAccountManager(accountAddr ethcommon.Address, keystoreDir string) (AccountManager, error) {
	keyStore := keystore.NewKeyStore(keystoreDir, keystore.StandardScryptN, keystore.StandardScryptP)

	acctExists := keyStore.HasAddress(accountAddr)
	numAccounts := len(keyStore.Accounts())

	var acct accounts.Account
	var err error
	if numAccounts == 0 || ((accountAddr != ethcommon.Address{}) && !acctExists) {
		glog.Infof("No Ethereum account found. Creating a new account")
		glog.Infof("This process will create a new Ethereum account for this Livepeer node")
		glog.Infof("Please enter a passphrase to encrypt the Private Keystore file for the Ethereum account.")
		glog.Infof("This process will ask for this passphrase every time it is launched")
		glog.Infof("(no characters will appear in Terminal when the passphrase is entered)")

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

	glog.Infof("Using Ethereum account: %v", acct.Address.Hex())

	return &DefaultAccountManager{
		account:  acct,
		unlocked: false,
		keyStore: keyStore,
	}, nil
}

// Unlock account indefinitely using underlying keystore
func (am *DefaultAccountManager) Unlock(passphrase string) error {
	var err error

	err = am.keyStore.Unlock(am.account, passphrase)
	if err != nil {
		if passphrase != "" {
			return err
		}
		glog.Infof("Please enter the passphrase to unlock Ethereum account %v", am.account.Address.Hex())

		passphrase, err = getPassphrase(false)
		err = am.keyStore.Unlock(am.account, passphrase)
		if err != nil {
			return err
		}
	}

	am.unlocked = true

	glog.Infof("Unlocked ETH account: %v", am.account.Address.Hex())

	return nil
}

// Lock account using underlying keystore and remove associated private key from memory
func (am *DefaultAccountManager) Lock() error {
	err := am.keyStore.Lock(am.account.Address)
	if err != nil {
		return err
	}

	am.unlocked = false

	return nil
}

// Create transact opts for client use - account must be unlocked
// Can optionally set gas limit and gas price used
func (am *DefaultAccountManager) CreateTransactOpts(gasLimit uint64, gasPrice *big.Int) (*bind.TransactOpts, error) {
	if !am.unlocked {
		return nil, ErrLocked
	}

	return &bind.TransactOpts{
		From:     am.account.Address,
		GasLimit: gasLimit,
		GasPrice: gasPrice,
		Signer: func(signer types.Signer, address ethcommon.Address, tx *types.Transaction) (*types.Transaction, error) {
			if address != am.account.Address {
				return nil, errors.New("not authorized to sign this account")
			}

			return am.SignTx(signer, tx)
		},
	}, nil
}

// Sign a transaction. Account must be unlocked
func (am *DefaultAccountManager) SignTx(signer types.Signer, tx *types.Transaction) (*types.Transaction, error) {
	signature, err := am.keyStore.SignHash(am.account, signer.Hash(tx).Bytes())
	if err != nil {
		return nil, err
	}

	return tx.WithSignature(signer, signature)
}

// Sign byte array message. Account must be unlocked
func (am *DefaultAccountManager) Sign(msg []byte) ([]byte, error) {
	personalMsg := fmt.Sprintf("\x19Ethereum Signed Message:\n%d%s", 32, msg)
	personalHash := crypto.Keccak256([]byte(personalMsg))

	return am.keyStore.SignHash(am.account, personalHash)
}

func (am *DefaultAccountManager) Account() accounts.Account {
	return am.account
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
