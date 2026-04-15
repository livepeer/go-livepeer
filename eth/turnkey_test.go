package eth

import (
	"crypto/ecdsa"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/accounts/keystore"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/signer/core/apitypes"
	lpcrypto "github.com/livepeer/go-livepeer/crypto"
	"github.com/livepeer/go-livepeer/pm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func turnkeyTestKeyAndKeystore(t *testing.T) (*keystore.KeyStore, *ecdsa.PrivateKey, accounts.Account, string) {
	t.Helper()
	dir := t.TempDir()
	ks := keystore.NewPlaintextKeyStore(dir)
	key, err := crypto.GenerateKey()
	require.NoError(t, err)
	acct, err := ks.ImportECDSA(key, "")
	require.NoError(t, err)
	return ks, key, acct, dir
}

func mockTurnkeyDigestSigner(t *testing.T, key *ecdsa.PrivateKey, onDigest func([]byte)) func(string, string, string) (string, string, string, error) {
	t.Helper()
	return func(_, _, payloadHex string) (string, string, string, error) {
		digest, err := hex.DecodeString(payloadHex)
		require.NoError(t, err)
		require.Len(t, digest, 32, "Turnkey must receive the final 32-byte Ethereum digest; KECCAK256 here would double-hash the payload")
		if onDigest != nil {
			onDigest(append([]byte(nil), digest...))
		}
		sig, err := crypto.Sign(digest, key)
		require.NoError(t, err)
		return hex.EncodeToString(sig[0:32]),
			hex.EncodeToString(sig[32:64]),
			hex.EncodeToString([]byte{sig[64]}),
			nil
	}
}

func TestTurnkeySignMatchesKeystore(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	_, key, acct, dir := turnkeyTestKeyAndKeystore(t)
	chainID := big.NewInt(777)
	am, err := NewAccountManager(acct.Address, dir, chainID, "")
	require.NoError(err)
	require.NoError(am.Unlock(""))

	tk := NewTurnkeyAccountManager(nil, "test-org", chainID, acct.Address)
	tk.signRawPayloadFn = mockTurnkeyDigestSigner(t, key, nil)

	msg := []byte("livepeer turnkey sign parity")
	sigKs, err := am.Sign(msg)
	require.NoError(err)
	sigTk, err := tk.Sign(msg)
	require.NoError(err)
	assert.Equal(sigKs, sigTk)
	assert.True(lpcrypto.VerifySig(acct.Address, msg, sigTk))
}

func TestTurnkeySignTypedDataMatchesKeystore(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	_, key, acct, dir := turnkeyTestKeyAndKeystore(t)
	chainID := big.NewInt(777)
	am, err := NewAccountManager(acct.Address, dir, chainID, "")
	require.NoError(err)
	require.NoError(am.Unlock(""))

	tk := NewTurnkeyAccountManager(nil, "test-org", chainID, acct.Address)
	tk.signRawPayloadFn = mockTurnkeyDigestSigner(t, key, nil)

	var td apitypes.TypedData
	require.NoError(json.Unmarshal([]byte(jsonTypedData), &td))

	sigKs, err := am.SignTypedData(td)
	require.NoError(err)
	sigTk, err := tk.SignTypedData(td)
	require.NoError(err)
	assert.Equal(sigKs, sigTk)
}

func TestTurnkeySignRawPayloadUsesPrecomputedEthereumDigests(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	_, key, acct, _ := turnkeyTestKeyAndKeystore(t)
	chainID := big.NewInt(777)
	tk := NewTurnkeyAccountManager(nil, "test-org", chainID, acct.Address)

	var seen [][]byte
	tk.signRawPayloadFn = mockTurnkeyDigestSigner(t, key, func(digest []byte) {
		seen = append(seen, digest)
	})

	msg := []byte("turnkey prehashed message")
	_, err := tk.Sign(msg)
	require.NoError(err)
	require.Len(seen, 1)
	assert.Equal(accounts.TextHash(msg), seen[0])

	var td apitypes.TypedData
	require.NoError(json.Unmarshal([]byte(jsonTypedData), &td))
	_, err = tk.SignTypedData(td)
	require.NoError(err)
	require.Len(seen, 2)

	domainSeparator, err := td.HashStruct("EIP712Domain", td.Domain.Map())
	require.NoError(err)
	typedDataHash, err := td.HashStruct(td.PrimaryType, td.Message)
	require.NoError(err)
	rawData := []byte("\x19\x01" + string(domainSeparator) + string(typedDataHash))
	assert.Equal(crypto.Keccak256(rawData), seen[1])
}

func TestTurnkeySignTxMatchesKeystore(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	_, key, acct, dir := turnkeyTestKeyAndKeystore(t)
	chainID := big.NewInt(777)
	am, err := NewAccountManager(acct.Address, dir, chainID, "")
	require.NoError(err)
	require.NoError(am.Unlock(""))

	to := ethcommon.HexToAddress("0x1111111111111111111111111111111111111111")
	tx := types.NewTx(&types.LegacyTx{
		Nonce:    3,
		To:       &to,
		Value:    big.NewInt(1000),
		Gas:      21000,
		GasPrice: big.NewInt(5),
		Data:     nil,
	})

	sigKs, err := am.SignTx(tx)
	require.NoError(err)
	ksRaw, err := sigKs.MarshalBinary()
	require.NoError(err)

	tk := NewTurnkeyAccountManager(nil, "test-org", chainID, acct.Address)
	tk.signTransactionFn = func(_, _, unsignedHex string) (string, error) {
		raw, err := hex.DecodeString(unsignedHex)
		require.NoError(err)
		var utx types.Transaction
		require.NoError(utx.UnmarshalBinary(raw))
		signer := types.LatestSignerForChainID(chainID)
		h := signer.Hash(&utx)
		sig, err := crypto.Sign(h.Bytes(), key)
		require.NoError(err)
		stx, err := utx.WithSignature(signer, sig)
		require.NoError(err)
		out, err := stx.MarshalBinary()
		require.NoError(err)
		return hex.EncodeToString(out), nil
	}

	sigTk, err := tk.SignTx(tx)
	require.NoError(err)
	tkRaw, err := sigTk.MarshalBinary()
	require.NoError(err)
	assert.Equal(ksRaw, tkRaw)
}

func TestTurnkeyAssembleRSV_LowSAndV27(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	n := crypto.S256().Params().N
	halfN := new(big.Int).Rsh(n, 1)

	key, err := crypto.GenerateKey()
	require.NoError(err)
	addr := crypto.PubkeyToAddress(key.PublicKey)
	msg := []byte("high-s scenario")
	digest := accounts.TextHash(msg)
	sig, err := crypto.Sign(digest, key)
	require.NoError(err)

	rHex := hex.EncodeToString(sig[0:32])
	sInt := new(big.Int).SetBytes(sig[32:64])
	// Mirror a signer that returned high-S: s' = N - s, flip recovery bit.
	sInt.Sub(n, sInt)
	sHex := hex.EncodeToString(fill32Bytes(sInt))
	vIn := sig[64] ^ 1
	vHex := hex.EncodeToString([]byte{vIn})

	out, err := assembleRSV(rHex, sHex, vHex)
	require.NoError(err)
	require.Len(out, 65)
	sOut := new(big.Int).SetBytes(out[32:64])
	assert.True(sOut.Cmp(halfN) <= 0)
	assert.Contains([]byte{27, 28}, out[64])
	assert.True(lpcrypto.VerifySig(addr, msg, out))
}

func fill32Bytes(x *big.Int) []byte {
	b := x.Bytes()
	out := make([]byte, 32)
	copy(out[32-len(b):], b)
	return out
}

func TestTurnkeyTicketAndInfoSigPathsMatchKeystore(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	_, key, acct, dir := turnkeyTestKeyAndKeystore(t)
	chainID := big.NewInt(777)
	am, err := NewAccountManager(acct.Address, dir, chainID, "")
	require.NoError(err)
	require.NoError(am.Unlock(""))

	tk := NewTurnkeyAccountManager(nil, "test-org", chainID, acct.Address)
	tk.signRawPayloadFn = mockTurnkeyDigestSigner(t, key, nil)

	ticket := &pm.Ticket{
		Recipient:              acct.Address,
		Sender:                 acct.Address,
		FaceValue:              big.NewInt(1),
		WinProb:                big.NewInt(2),
		SenderNonce:            3,
		RecipientRandHash:      ethcommon.Hash{4},
		CreationRound:          5,
		CreationRoundBlockHash: ethcommon.Hash{6},
	}
	th := ticket.Hash().Bytes()
	sigKs, err := am.Sign(th)
	require.NoError(err)
	sigTk, err := tk.Sign(th)
	require.NoError(err)
	assert.Equal(sigKs, sigTk)
	assert.True(lpcrypto.VerifySig(acct.Address, th, sigTk))

	addrHex := []byte(acct.Address.Hex())
	pre := crypto.Keccak256(addrHex)
	sigKs2, err := am.Sign(pre)
	require.NoError(err)
	sigTk2, err := tk.Sign(pre)
	require.NoError(err)
	assert.Equal(sigKs2, sigTk2)
	assert.True(lpcrypto.VerifySig(acct.Address, pre, sigTk2))

	stateJSON := []byte(`{"stateID":"x","orch":"y"}`)
	sigKs3, err := am.Sign(stateJSON)
	require.NoError(err)
	sigTk3, err := tk.Sign(stateJSON)
	require.NoError(err)
	assert.Equal(sigKs3, sigTk3)
	assert.True(lpcrypto.VerifySig(acct.Address, stateJSON, sigTk3))
}

func TestTurnkeyWithSigningAddress_SwitchesSignWith(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	_, _, acct, _ := turnkeyTestKeyAndKeystore(t)
	other := ethcommon.HexToAddress("0x2222222222222222222222222222222222222222")
	chainID := big.NewInt(1)

	tk := NewTurnkeyAccountManager(nil, "org", chainID, acct.Address)
	var seen string
	err := tk.WithSigningAddress(other, func() error {
		seen = tk.activeSignWith()
		return nil
	})
	require.NoError(err)
	assert.Equal(other.Hex(), seen)
	assert.Equal(acct.Address, tk.SigningAddress())
}
