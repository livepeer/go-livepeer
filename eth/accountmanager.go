package eth

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/accounts/keystore"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/console/prompt"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/signer/core/apitypes"
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
	CreateTransactOpts(gasLimit uint64) (*bind.TransactOpts, error)
	SignTx(tx *types.Transaction) (*types.Transaction, error)
	Sign(msg []byte) ([]byte, error)
	SignTypedData(typedData apitypes.TypedData) ([]byte, error)
	Account() accounts.Account
}

type accountManager struct {
	account  accounts.Account
	chainID  *big.Int
	unlocked bool
	keyStore *keystore.KeyStore
}

func NewAccountManager(accountAddr ethcommon.Address, keystoreDir string, chainID *big.Int, passphrase string) (AccountManager, error) {
	keyStore := keystore.NewKeyStore(keystoreDir, keystore.StandardScryptN, keystore.StandardScryptP)

	acctExists := keyStore.HasAddress(accountAddr)
	numAccounts := len(keyStore.Accounts())

	var acct accounts.Account
	var err error
	if numAccounts == 0 || ((accountAddr != ethcommon.Address{}) && !acctExists) {
		glog.Infof("No Ethereum account found. Creating a new account")
		glog.Infof("This process will create a new Ethereum account for this Livepeer node")

		if passphrase == "" {
			glog.Infof("Please enter a passphrase to encrypt the Private Keystore file for the Ethereum account.")
			glog.Infof("This process will ask for this passphrase every time it is launched")
			glog.Infof("(no characters will appear in Terminal when the passphrase is entered)")

			// Account does not exist yet, set it up
			acct, err = createAccount(keyStore)
			if err != nil {
				return nil, err
			}
		} else {
			acct, err = keyStore.NewAccount(passphrase)
			if err != nil {
				return nil, err
			}
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

	return &accountManager{
		account:  acct,
		chainID:  chainID,
		unlocked: false,
		keyStore: keyStore,
	}, nil
}

// Unlock account indefinitely using underlying keystore
func (am *accountManager) Unlock(pass string) error {
	var err error

	// We don't care if ReadFromFile() returns an error.
	// The string it returns will always be valid.
	passphrase, _ := common.ReadFromFile(pass)

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
func (am *accountManager) Lock() error {
	err := am.keyStore.Lock(am.account.Address)
	if err != nil {
		return err
	}

	am.unlocked = false

	return nil
}

// Create transact opts for client use - account must be unlocked
// Can optionally set gas limit and gas price used
func (am *accountManager) CreateTransactOpts(gasLimit uint64) (*bind.TransactOpts, error) {
	if !am.unlocked {
		return nil, ErrLocked
	}

	opts, err := bind.NewKeyStoreTransactorWithChainID(am.keyStore, am.account, am.chainID)
	if err != nil {
		return nil, err
	}
	opts.GasLimit = gasLimit

	return opts, nil
}

// Sign a transaction. Account must be unlocked
func (am *accountManager) SignTx(tx *types.Transaction) (*types.Transaction, error) {
	signer := types.LatestSignerForChainID(am.chainID)
	signature, err := am.keyStore.SignHash(am.account, signer.Hash(tx).Bytes())
	if err != nil {
		return nil, err
	}

	return tx.WithSignature(signer, signature)
}

// Sign byte array message. Account must be unlocked
func (am *accountManager) Sign(msg []byte) ([]byte, error) {
	return am.signHash(accounts.TextHash(msg))
}

// Based on https://github.com/ethereum/go-ethereum/blob/dddf73abbddb297e61cee6a7e6aebfee87125e49/signer/core/signed_data.go#L236
func (am *accountManager) SignTypedData(typedData apitypes.TypedData) ([]byte, error) {
	domainSeparator, err := typedData.HashStruct("EIP712Domain", typedData.Domain.Map())
	if err != nil {
		return nil, err
	}
	typedDataHash, err := typedData.HashStruct(typedData.PrimaryType, typedData.Message)
	if err != nil {
		return nil, err
	}
	rawData := []byte(fmt.Sprintf("\x19\x01%s%s", string(domainSeparator), string(typedDataHash)))
	sighash := crypto.Keccak256(rawData)

	return am.signHash(sighash)
}

func (am *accountManager) signHash(hash []byte) ([]byte, error) {
	sig, err := am.keyStore.SignHash(am.account, hash)
	if err != nil {
		return nil, err
	}

	// sig is in the [R || S || V] format where V is 0 or 1
	// Convert the V param to 27 or 28
	v := sig[64]
	if v == byte(0) || v == byte(1) {
		v += 27
	}

	return append(sig[:64], v), nil
}

func (am *accountManager) Account() accounts.Account {
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
	passphrase, err := prompt.Stdin.PromptPassword("Passphrase: ")
	if err != nil {
		return "", err
	}

	if shouldConfirm {
		confirmation, err := prompt.Stdin.PromptPassword("Repeat passphrase: ")
		if err != nil {
			return "", err
		}

		if passphrase != confirmation {
			return "", ErrPassphraseMismatch
		}
	}

	return passphrase, nil
}
