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

	am, err := NewAccountManager(a.Address, dir)
	if err != nil {
		t.Fatal(err)
	}

	// ensure password checking works
	err = am.Unlock("") // should prompt for pw. TODO expect-test this
	if err != keystore.ErrDecrypt {
		t.Fatal(err)
	}

	err = am.Unlock("foo!") // should not prompt for pw. TODO expect-test this
	if err != keystore.ErrDecrypt {
		t.Fatal(err)
	}

	err = am.Unlock("foo")
	if err != nil {
		t.Fatal(err)
	}
}

func TestEmptyPassphrase(t *testing.T) {
	dir, ks := tmpKeyStore(t, true)
	defer os.RemoveAll(dir)

	a, err := ks.NewAccount("")
	if err != nil {
		t.Fatal(err)
	}

	am, err := NewAccountManager(a.Address, dir)
	if err != nil {
		t.Fatal(err)
	}

	// This test ensures we don't prompt for pw. A bit artificial, but if
	// we prompt, `go test` current semantics would mean getPassphrase
	// returns an empty string, unlocking the wallet when it shouldn't.
	err = am.Unlock("should not prompt for pw")
	if err != keystore.ErrDecrypt {
		t.Fatal(err)
	}

	err = am.Unlock("")
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
