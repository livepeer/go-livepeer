package eth

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
	"sync"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/signer/core/apitypes"
	sdk "github.com/tkhq/go-sdk"
	"github.com/tkhq/go-sdk/pkg/api/client/signing"
	"github.com/tkhq/go-sdk/pkg/api/models"
	"github.com/tkhq/go-sdk/pkg/util"
)

// TurnkeyAccountManager implements AccountManager using Turnkey secure signing.
// Message signing matches the keystore path: ECDSA over accounts.TextHash(msg),65-byte sig with v in {27,28}, low-S.
type TurnkeyAccountManager struct {
	client  *sdk.Client
	orgID   string
	chainID *big.Int
	signer  types.Signer

	mu sync.Mutex
	// signWith is the Turnkey SignWith identifier (Ethereum address hex).
	signWith string
	signRawPayloadFn  func(orgID, signWith, payloadHex string) (r, s, v string, err error)
	signTransactionFn func(orgID, signWith, unsignedTxHex string) (signedTxHex string, err error)
}

// NewTurnkeyAccountManager constructs a Turnkey-backed account manager.
// signWith must be a Turnkey wallet account Ethereum address (0x-prefixed) that this API key may sign for.
func NewTurnkeyAccountManager(
	client *sdk.Client,
	orgID string,
	chainID *big.Int,
	signWith ethcommon.Address,
) *TurnkeyAccountManager {
	return &TurnkeyAccountManager{
		client:   client,
		orgID:    orgID,
		chainID:  chainID,
		signer:   types.LatestSignerForChainID(chainID),
		signWith: signWith.Hex(),
	}
}

// WithSigningAddress runs fn while temporarily using addr for Turnkey SignWith.
// The lock is not held during fn to avoid deadlock when fn calls Sign/SignTx.
func (t *TurnkeyAccountManager) WithSigningAddress(addr ethcommon.Address, fn func() error) error {
	if t == nil {
		return fmt.Errorf("turnkey account manager is nil")
	}
	t.mu.Lock()
	prev := t.signWith
	t.signWith = addr.Hex()
	t.mu.Unlock()
	defer func() {
		t.mu.Lock()
		t.signWith = prev
		t.mu.Unlock()
	}()
	return fn()
}

// SetSigningAddress updates the default signing address (e.g. CLI select).
func (t *TurnkeyAccountManager) SetSigningAddress(addr ethcommon.Address) {
	if t == nil {
		return
	}
	t.mu.Lock()
	t.signWith = addr.Hex()
	t.mu.Unlock()
}

// SigningAddress returns the active Turnkey SignWith address.
func (t *TurnkeyAccountManager) SigningAddress() ethcommon.Address {
	if t == nil {
		return ethcommon.Address{}
	}
	t.mu.Lock()
	s := t.signWith
	t.mu.Unlock()
	return ethcommon.HexToAddress(s)
}

func (t *TurnkeyAccountManager) activeSignWith() string {
	t.mu.Lock()
	s := t.signWith
	t.mu.Unlock()
	return s
}

func (t *TurnkeyAccountManager) Unlock(string) error {
	return nil
}

func (t *TurnkeyAccountManager) Lock() error {
	return nil
}

func (t *TurnkeyAccountManager) Account() accounts.Account {
	return accounts.Account{Address: t.SigningAddress()}
}

func (t *TurnkeyAccountManager) Sign(msg []byte) ([]byte, error) {
	digest := accounts.TextHash(msg)
	return t.signDigest(digest)
}

func (t *TurnkeyAccountManager) SignTypedData(typedData apitypes.TypedData) ([]byte, error) {
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
	return t.signDigest(sighash)
}

func (t *TurnkeyAccountManager) signDigest(digest []byte) ([]byte, error) {
	if len(digest) != 32 {
		return nil, fmt.Errorf("turnkey signDigest: expected 32-byte hash, got %d", len(digest))
	}
	payloadHex := hex.EncodeToString(digest)
	return t.signRawPayloadHex(payloadHex)
}

func (t *TurnkeyAccountManager) signRawPayloadHex(payloadHex string) ([]byte, error) {
	signWith := t.activeSignWith()
	var rStr, sStr, vStr string
	var err error
	if t.signRawPayloadFn != nil {
		rStr, sStr, vStr, err = t.signRawPayloadFn(t.orgID, signWith, payloadHex)
	} else {
		rStr, sStr, vStr, err = t.turnkeySignRawPayload(payloadHex)
	}
	if err != nil {
		return nil, err
	}
	return assembleRSV(rStr, sStr, vStr)
}

func (t *TurnkeyAccountManager) turnkeySignRawPayload(payloadHex string) (r, s, v string, err error) {
	actType := string(models.ActivityTypeSignRawPayloadV2)
	sw := t.activeSignWith()
	params := signing.NewSignRawPayloadParams().WithBody(&models.SignRawPayloadRequest{
		OrganizationID: &t.orgID,
		TimestampMs:    util.RequestTimestamp(),
		Parameters: &models.SignRawPayloadIntentV2{
			Encoding:     models.PayloadEncodingHexadecimal.Pointer(),
			// Ethereum-specific hashing is performed locally before this call
			// (EIP-191 text hash, EIP-712 digest, etc), so Turnkey must sign the
			// provided 32-byte digest without applying another hash function.
			HashFunction: models.HashFunctionNoOp.Pointer(),
			Payload:      &payloadHex,
			SignWith:     &sw,
		},
		Type: &actType,
	})

	resp, err := t.client.V0().Signing.SignRawPayload(params, t.client.Authenticator)
	if err != nil {
		return "", "", "", err
	}
	res := resp.Payload.Activity.Result.SignRawPayloadResult
	if res == nil || res.R == nil || res.S == nil || res.V == nil {
		return "", "", "", fmt.Errorf("turnkey SignRawPayload: missing r/s/v in response")
	}
	return *res.R, *res.S, *res.V, nil
}

func assembleRSV(rStr, sStr, vStr string) ([]byte, error) {
	rBytes, err := decodeHexUint256(rStr)
	if err != nil {
		return nil, fmt.Errorf("decode r: %w", err)
	}
	sBytes, err := decodeHexUint256(sStr)
	if err != nil {
		return nil, fmt.Errorf("decode s: %w", err)
	}
	if len(rBytes) > 32 || len(sBytes) > 32 {
		return nil, fmt.Errorf("r/s too long")
	}
	rPadded := make([]byte, 32)
	sPadded := make([]byte, 32)
	copy(rPadded[32-len(rBytes):], rBytes)
	copy(sPadded[32-len(sBytes):], sBytes)

	vByte, err := parseV(vStr)
	if err != nil {
		return nil, err
	}

	n := crypto.S256().Params().N
	halfN := new(big.Int).Rsh(n, 1)
	sInt := new(big.Int).SetBytes(sPadded)
	vNorm := vByte
	if sInt.Cmp(halfN) > 0 {
		sInt.Sub(n, sInt)
		fill32(sPadded, sInt)
		if vNorm == 27 || vNorm == 28 {
			vNorm ^= 1 // flip 27<->28
		} else if vNorm <= 1 {
			vNorm ^= 1
		}
	}
	if vNorm <= 1 {
		vNorm += 27
	}
	if vNorm != 27 && vNorm != 28 {
		return nil, fmt.Errorf("invalid v after normalization: %d", vNorm)
	}
	out := make([]byte, 65)
	copy(out[0:32], rPadded)
	copy(out[32:64], sPadded)
	out[64] = vNorm
	return out, nil
}

func fill32(dst []byte, x *big.Int) {
	b := x.Bytes()
	if len(b) > 32 {
		copy(dst, b[len(b)-32:])
		return
	}
	copy(dst[32-len(b):], b)
}

func decodeHexUint256(s string) ([]byte, error) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "0x")
	if s == "" {
		return nil, fmt.Errorf("empty hex")
	}
	return hex.DecodeString(s)
}

func parseV(vStr string) (byte, error) {
	vStr = strings.TrimSpace(vStr)
	vStr = strings.TrimPrefix(vStr, "0x")
	if vStr == "" {
		return 0, fmt.Errorf("empty v")
	}
	if len(vStr) <= 2 {
		b, err := hex.DecodeString(vStr)
		if err != nil {
			return 0, err
		}
		if len(b) != 1 {
			return 0, fmt.Errorf("invalid v")
		}
		return b[0], nil
	}
	// decimal integer string
	v := new(big.Int)
	if _, ok := v.SetString(vStr, 10); ok {
		return byte(v.Uint64()), nil
	}
	b, err := hex.DecodeString(vStr)
	if err != nil {
		return 0, err
	}
	if len(b) == 1 {
		return b[0], nil
	}
	return 0, fmt.Errorf("unrecognized v encoding")
}

func (t *TurnkeyAccountManager) SignTx(tx *types.Transaction) (*types.Transaction, error) {
	raw, err := t.signTransactionRLP(tx)
	if err != nil {
		return nil, err
	}
	var out types.Transaction
	if err := out.UnmarshalBinary(raw); err != nil {
		return nil, err
	}
	return &out, nil
}

func (t *TurnkeyAccountManager) signTransactionRLP(tx *types.Transaction) ([]byte, error) {
	signWith := t.activeSignWith()
	unsignedBytes, err := tx.MarshalBinary()
	if err != nil {
		return nil, err
	}
	unsignedHex := hex.EncodeToString(unsignedBytes)
	var rlpHex string
	if t.signTransactionFn != nil {
		rlpHex, err = t.signTransactionFn(t.orgID, signWith, unsignedHex)
		if err != nil {
			return nil, err
		}
	} else {
		actType := string(models.ActivityTypeSignTransactionV2)
		sw := signWith
		utx := unsignedHex
		params := signing.NewSignTransactionParams().WithBody(&models.SignTransactionRequest{
			OrganizationID: &t.orgID,
			TimestampMs:    util.RequestTimestamp(),
			Parameters: &models.SignTransactionIntentV2{
				SignWith:            &sw,
				Type:                models.TransactionTypeEthereum.Pointer(),
				UnsignedTransaction: &utx,
			},
			Type: &actType,
		})
		resp, err := t.client.V0().Signing.SignTransaction(params, t.client.Authenticator)
		if err != nil {
			return nil, err
		}
		if resp.Payload.Activity.Result.SignTransactionResult == nil ||
			resp.Payload.Activity.Result.SignTransactionResult.SignedTransaction == nil {
			return nil, fmt.Errorf("turnkey SignTransaction: missing signedTransaction")
		}
		rlpHex = *resp.Payload.Activity.Result.SignTransactionResult.SignedTransaction
	}
	rlpHex = strings.TrimPrefix(strings.TrimSpace(rlpHex), "0x")
	return hex.DecodeString(rlpHex)
}

func (t *TurnkeyAccountManager) CreateTransactOpts(gasLimit uint64) (*bind.TransactOpts, error) {
	from := t.SigningAddress()
	return &bind.TransactOpts{
		From:      from,
		Nonce:     nil,
		Signer:    t.makeSigner(from),
		Value:     nil,
		GasPrice:  nil,
		GasFeeCap: nil,
		GasTipCap: nil,
		GasLimit:  gasLimit,
		Context:   nil,
	}, nil
}

func (t *TurnkeyAccountManager) makeSigner(from ethcommon.Address) bind.SignerFn {
	return func(addr ethcommon.Address, transaction *types.Transaction) (*types.Transaction, error) {
		if addr != from {
			return nil, fmt.Errorf("turnkey transact signer: expected from %s, got %s", from.Hex(), addr.Hex())
		}
		return t.SignTx(transaction)
	}
}
