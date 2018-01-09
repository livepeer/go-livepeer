package eth

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/keystore"
)

func TestAccountManager(t *testing.T) {
	dir, ks := tmpKeyStore(t, true)
	defer os.RemoveAll(dir)

	a, err := ks.NewAccount("foo")
	if err != nil {
		t.Fatal(err)
	}

	_, err = NewAccountManager(a.Address, dir)
	if err != nil {
		t.Fatal(err)
	}
}

func tmpKeyStore(t *testing.T, encrypted bool) (string, *keystore.KeyStore) {
	d, err := ioutil.TempDir("", "eth-keystore-test")
	if err != nil {
		t.Fatal(err)
	}

	new := keystore.NewPlaintextKeyStore
	if encrypted {
		new = func(kd string) *keystore.KeyStore {
			return keystore.NewKeyStore(kd, keystore.LightScryptN, keystore.LightScryptP)
		}
	}

	return d, new(d)
}
