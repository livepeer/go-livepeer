package eth

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/ethereum/go-ethereum/signer/core/apitypes"
	"github.com/golang/glog"
)

// web3signerAccountManager is an AccountManager that delegates signing to a
// remote signer speaking the standard Ethereum eth_* JSON-RPC namespace
// (eth_sign, eth_signTransaction, eth_signTypedData), instead of a local
// keystore. This lets the node reach key backends fronted by Web3Signer (AWS
// KMS, Azure Key Vault, HashiCorp Vault, HSM) with no provider-specific code,
// as well as MPC/enclave custody sidecars (e.g. Turnkey) that present the same
// API.
//
// Output is byte-identical to the keystore accountManager: EIP-191 message
// signing, latest-signer transactions, and recovery id in {27,28}.
// defaultWeb3SignerTimeout bounds each signing round-trip so a hung or slow
// remote signer cannot block the PM ticket hot path indefinitely.
const defaultWeb3SignerTimeout = 5 * time.Second

type web3signerAccountManager struct {
	rpc     *rpc.Client
	account accounts.Account
	chainID *big.Int
	timeout time.Duration
}

// NewWeb3SignerAccountManager connects to a Web3Signer-compatible endpoint and
// signs on behalf of accountAddr (required, since there is no local keystore).
// timeout bounds each signing call; values <= 0 fall back to a sane default.
func NewWeb3SignerAccountManager(accountAddr ethcommon.Address, endpoint string, chainID *big.Int, timeout time.Duration) (AccountManager, error) {
	if (accountAddr == ethcommon.Address{}) {
		return nil, fmt.Errorf("web3signer requires an explicit -ethAcctAddr")
	}
	if timeout <= 0 {
		timeout = defaultWeb3SignerTimeout
	}

	rpcClient, err := rpc.Dial(endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to dial web3signer at %s: %w", endpoint, err)
	}

	// eth_accounts doubles as a reachability check and lets us warn if the
	// remote signer does not hold the configured address.
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	var addrs []ethcommon.Address
	if err := rpcClient.CallContext(ctx, &addrs, "eth_accounts"); err != nil {
		return nil, fmt.Errorf("failed to reach web3signer at %s: %w", endpoint, err)
	}
	if !containsAddress(addrs, accountAddr) {
		glog.Warningf("Web3Signer at %s did not list account %v; signing requests may be rejected", endpoint, accountAddr.Hex())
	}

	glog.Infof("Using web3signer at %s for Ethereum account: %v (timeout %s)", endpoint, accountAddr.Hex(), timeout)

	return &web3signerAccountManager{
		rpc:     rpcClient,
		account: accounts.Account{Address: accountAddr},
		chainID: chainID,
		timeout: timeout,
	}, nil
}

// callContext returns a context bounded by the configured signing timeout.
func (m *web3signerAccountManager) callContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), m.timeout)
}

func containsAddress(addrs []ethcommon.Address, want ethcommon.Address) bool {
	for _, a := range addrs {
		if a == want {
			return true
		}
	}
	return false
}

func (m *web3signerAccountManager) Unlock(string) error { return nil }

func (m *web3signerAccountManager) Lock() error { return nil }

func (m *web3signerAccountManager) Account() accounts.Account { return m.account }

func (m *web3signerAccountManager) CreateTransactOpts(gasLimit uint64) (*bind.TransactOpts, error) {
	return &bind.TransactOpts{
		From:     m.account.Address,
		GasLimit: gasLimit,
		Signer: func(addr ethcommon.Address, tx *types.Transaction) (*types.Transaction, error) {
			if addr != m.account.Address {
				return nil, fmt.Errorf("not authorized to sign for address %v", addr.Hex())
			}
			return m.SignTx(tx)
		},
	}, nil
}

// Sign signs msg with the EIP-191 personal-message prefix. Web3Signer's eth_sign
// applies that prefix, matching the keystore accountManager.
func (m *web3signerAccountManager) Sign(msg []byte) ([]byte, error) {
	ctx, cancel := m.callContext()
	defer cancel()
	var res hexutil.Bytes
	if err := m.rpc.CallContext(ctx, &res, "eth_sign", m.account.Address, hexutil.Encode(msg)); err != nil {
		return nil, err
	}
	return toEthV(res), nil
}

func (m *web3signerAccountManager) SignTypedData(typedData apitypes.TypedData) ([]byte, error) {
	ctx, cancel := m.callContext()
	defer cancel()
	var res hexutil.Bytes
	if err := m.rpc.CallContext(ctx, &res, "eth_signTypedData", m.account.Address, typedData); err != nil {
		return nil, err
	}
	return toEthV(res), nil
}

// SignTx asks the remote signer to sign tx and returns the decoded signed
// transaction. go-livepeer still owns nonce/gas and broadcasts the result.
func (m *web3signerAccountManager) SignTx(tx *types.Transaction) (*types.Transaction, error) {
	ctx, cancel := m.callContext()
	defer cancel()
	var raw hexutil.Bytes
	if err := m.rpc.CallContext(ctx, &raw, "eth_signTransaction", m.toSendTxArgs(tx)); err != nil {
		return nil, err
	}
	signed := new(types.Transaction)
	if err := signed.UnmarshalBinary(raw); err != nil {
		return nil, fmt.Errorf("web3signer returned an undecodable transaction: %w", err)
	}
	return signed, nil
}

func (m *web3signerAccountManager) toSendTxArgs(tx *types.Transaction) *apitypes.SendTxArgs {
	data := hexutil.Bytes(tx.Data())
	args := &apitypes.SendTxArgs{
		Data:  &data,
		Nonce: hexutil.Uint64(tx.Nonce()),
		Value: hexutil.Big(*tx.Value()),
		Gas:   hexutil.Uint64(tx.Gas()),
		From:  ethcommon.NewMixedcaseAddress(m.account.Address),
	}
	if tx.To() != nil {
		to := ethcommon.NewMixedcaseAddress(*tx.To())
		args.To = &to
	}
	switch tx.Type() {
	case types.LegacyTxType, types.AccessListTxType:
		args.GasPrice = (*hexutil.Big)(tx.GasPrice())
	case types.DynamicFeeTxType:
		args.MaxFeePerGas = (*hexutil.Big)(tx.GasFeeCap())
		args.MaxPriorityFeePerGas = (*hexutil.Big)(tx.GasTipCap())
	}
	if m.chainID != nil && m.chainID.Sign() != 0 {
		args.ChainID = (*hexutil.Big)(m.chainID)
	}
	return args
}

// toEthV normalizes the recovery id to {27,28} so output matches the keystore
// accountManager regardless of whether the remote signer returns {0,1} or
// {27,28}.
func toEthV(sig []byte) []byte {
	if len(sig) == 65 && sig[64] < 27 {
		sig[64] += 27
	}
	return sig
}
