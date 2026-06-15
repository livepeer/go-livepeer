package eth

import (
	"crypto/ecdsa"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ethereum/go-ethereum/accounts"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	lpcrypto "github.com/livepeer/go-livepeer/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// web3signerStub is a minimal Web3Signer-style JSON-RPC server that signs with
// key, exposing the eth_* namespace the adapter uses.
func web3signerStub(t *testing.T, key *ecdsa.PrivateKey) *httptest.Server {
	addr := crypto.PubkeyToAddress(key.PublicKey)

	reply := func(w http.ResponseWriter, id json.RawMessage, result interface{}) {
		raw, err := json.Marshal(result)
		require.NoError(t, err)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"jsonrpc": "2.0", "id": json.RawMessage(id), "result": json.RawMessage(raw),
		})
	}

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID     json.RawMessage   `json:"id"`
			Method string            `json:"method"`
			Params []json.RawMessage `json:"params"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))

		switch req.Method {
		case "eth_accounts":
			reply(w, req.ID, []ethcommon.Address{addr})
		case "eth_sign":
			// params: [address, hexData]; Web3Signer applies the EIP-191 prefix.
			var hexData string
			require.NoError(t, json.Unmarshal(req.Params[1], &hexData))
			data, err := hexutil.Decode(hexData)
			require.NoError(t, err)
			sig, err := crypto.Sign(accounts.TextHash(data), key) // V in {0,1}
			require.NoError(t, err)
			sig[64] += 27 // Web3Signer returns V in {27,28}
			reply(w, req.ID, hexutil.Bytes(sig))
		default:
			http.Error(w, "unsupported method "+req.Method, http.StatusBadRequest)
		}
	}))
}

func TestWeb3Signer_SignMatchesKeystoreConvention(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	key, err := crypto.GenerateKey()
	require.NoError(err)
	addr := crypto.PubkeyToAddress(key.PublicKey)

	srv := web3signerStub(t, key)
	defer srv.Close()

	am, err := NewWeb3SignerAccountManager(addr, srv.URL, big.NewInt(1))
	require.NoError(err)
	assert.Equal(addr, am.Account().Address)

	msg := []byte("livepeer payment ticket hash")
	sig, err := am.Sign(msg)
	require.NoError(err)

	require.Len(sig, 65)
	assert.Contains([]byte{27, 28}, sig[64])
	assert.True(lpcrypto.VerifySig(addr, msg, sig), "signature must recover to signer address")
}

func TestWeb3Signer_RequiresExplicitAddress(t *testing.T) {
	_, err := NewWeb3SignerAccountManager(ethcommon.Address{}, "http://127.0.0.1:0", big.NewInt(1))
	assert.Error(t, err)
}

func TestWeb3Signer_UnreachableEndpoint(t *testing.T) {
	key, err := crypto.GenerateKey()
	require.NoError(t, err)
	addr := crypto.PubkeyToAddress(key.PublicKey)
	_, err = NewWeb3SignerAccountManager(addr, "http://127.0.0.1:1", big.NewInt(1))
	assert.Error(t, err)
}
