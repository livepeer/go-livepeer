package eth

import (
	"encoding/json"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/signer/core/apitypes"
	"github.com/livepeer/go-livepeer/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var jsonTypedData = `
    {
      "types": {
        "EIP712Domain": [
          {
            "name": "name",
            "type": "string"
          },
          {
            "name": "version",
            "type": "string"
          },
          {
            "name": "chainId",
            "type": "uint256"
          },
          {
            "name": "verifyingContract",
            "type": "address"
          }
        ],
        "Person": [
          {
            "name": "name",
            "type": "string"
          },
          {
            "name": "wallet",
            "type": "address"
          }
        ],
        "Mail": [
          {
            "name": "from",
            "type": "Person"
          },
          {
            "name": "to",
            "type": "Person"
          },
          {
            "name": "contents",
            "type": "string"
          }
        ]
      },
      "primaryType": "Mail",
      "domain": {
        "name": "Ether Mail",
        "version": "1",
        "chainId": "1",
        "verifyingContract": "0xCCCcccccCCCCcCCCCCCcCcCccCcCCCcCcccccccC"
      },
      "message": {
        "from": {
          "name": "Cow",
          "wallet": "0xcD2a3d9F938E13CD947Ec05AbC7FE734Df8DD826"
        },
        "to": {
          "name": "Bob",
          "wallet": "0xbBbBBBBbbBBBbbbBbbBbbbbBBbBbbbbBbBbbBBbB"
        },
        "contents": "Hello, Bob!"
      }
    }
`

func TestAccountManager(t *testing.T) {
	dir, ks := tmpKeyStore(t, true)

	a, err := ks.NewAccount("foo")
	if err != nil {
		t.Fatal(err)
	}

	am, err := NewAccountManager(a.Address, dir, big.NewInt(777), "")
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

	a, err := ks.NewAccount("")
	if err != nil {
		t.Fatal(err)
	}

	am, err := NewAccountManager(a.Address, dir, big.NewInt(777), "")
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

func TestSign(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	dir, ks := tmpKeyStore(t, true)

	a, err := ks.NewAccount("")
	require.Nil(err)

	am, err := NewAccountManager(a.Address, dir, big.NewInt(777), "")
	require.Nil(err)

	_, err = am.Sign([]byte("foo"))
	assert.NotNil(err)
	assert.EqualError(err, "authentication needed: password or unlock")

	err = am.Unlock("")
	require.Nil(err)

	sig, err := am.Sign([]byte("foo"))
	assert.Nil(err)
	assert.True(crypto.VerifySig(a.Address, []byte("foo"), sig))
}

func TestSignTypedData(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	dir, ks := tmpKeyStore(t, true)

	a, err := ks.NewAccount("")
	require.Nil(err)

	am, err := NewAccountManager(a.Address, dir, big.NewInt(777), "")
	require.Nil(err)

	am.Unlock("")
	require.Nil(err)

	var d apitypes.TypedData
	err = json.Unmarshal([]byte(jsonTypedData), &d)
	require.Nil(err)

	sig, err := am.SignTypedData(d)
	assert.Nil(err)
	assert.NotNil(sig)
	assert.Len(sig, 65)
}

func tmpKeyStore(t *testing.T, encrypted bool) (string, *keystore.KeyStore) {
	d := t.TempDir()

	new := keystore.NewPlaintextKeyStore
	if encrypted {
		new = func(kd string) *keystore.KeyStore {
			return keystore.NewKeyStore(kd, keystore.LightScryptN, keystore.LightScryptP)
		}
	}

	return d, new(d)
}
