package eth

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"log"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTxLog(t *testing.T) {
	assert := assert.New(t)

	// https://etherscan.io/tx/0x6afffe4d95789e7a27bf0ec8adb8324b2e481a5a6df468366e6efffad15746a6
	inputs := "_ticket: { Recipient: 0x9C10672CEE058Fd658103d90872fE431bb6C0AFa  Sender: 0x3eE860A4abA830AF84EbBCE2b381fc11DB8493e2  FaceValue: 24703611594215916  WinProb: 4687253472865791949219216447262131914283161967982656133076290000000000000000  SenderNonce: 1  RecipientRandHash: 0xd00ef48a6825d9ae93097ad55bc76057fb50afc203aa98fd4e01aa6576d71298  AuxData: 0x0000000000000000000000000000000000000000000000000000000000000689a9f81d59be53a90a12cb12f627aa9e87c505320bad20a98fd98d5ed23bfe9a54 }  _sig: 0x5ff5265992f1a6563851d085f363b16ad65416baaee090ee368b59082437df7a0237214659511fc188d6a4ef57c988bb18faa3e99826c78e9e75eb29366745891b  _recipientRand: 21160157592767011607011497540409649304524524744605816319294275445072062630355"
	tx := types.NewTransaction(
		1,
		common.HexToAddress("0x5b1ce829384eebfa30286f12d1e7a695ca45f5d2"),
		big.NewInt(0),
		167221,
		big.NewInt(5000000000),
		common.Hex2Bytes("ec8b3cb6000000000000000000000000000000000000000000000000000000000000006000000000000000000000000000000000000000000000000000000000000001a02ec8398aed129f376c296c8b999d2f9ee61494110f9a752494d2ef1b89a4c9d30000000000000000000000009c10672cee058fd658103d90872fe431bb6c0afa0000000000000000000000003ee860a4aba830af84ebbce2b381fc11db8493e20000000000000000000000000000000000000000000000000057c3cdc9be11ec0a5ce4361d251fc5d71ba8fa55eef46b5a59a967145c79aa4cdbe1f943ad00000000000000000000000000000000000000000000000000000000000000000001d00ef48a6825d9ae93097ad55bc76057fb50afc203aa98fd4e01aa6576d7129800000000000000000000000000000000000000000000000000000000000000e000000000000000000000000000000000000000000000000000000000000000400000000000000000000000000000000000000000000000000000000000000689a9f81d59be53a90a12cb12f627aa9e87c505320bad20a98fd98d5ed23bfe9a5400000000000000000000000000000000000000000000000000000000000000415ff5265992f1a6563851d085f363b16ad65416baaee090ee368b59082437df7a0237214659511fc188d6a4ef57c988bb18faa3e99826c78e9e75eb29366745891b00000000000000000000000000000000000000000000000000000000000000"),
	)

	txLog, err := newTxLog(tx)
	assert.Equal("redeemWinningTicket", txLog.method)
	assert.Equal(inputs, txLog.inputs)

	// test unknown ABI
	tx = types.NewTransaction(2, common.HexToAddress("foo"), big.NewInt(0), 200000, big.NewInt(1000000), common.Hex2Bytes("aaaabbbb"))
	txLog, err = newTxLog(tx)
	assert.EqualError(err, "unknown ABI")

	// test no method signature
	tx = types.NewTransaction(2, common.HexToAddress("foo"), big.NewInt(0), 200000, big.NewInt(1000000), nil)
	txLog, err = newTxLog(tx)
	assert.EqualError(err, "no method signature")
}

func TestSendTransaction_SendErr_DontUpdateNonce(t *testing.T) {
	// client := &ethclient.Client{}
	client, err := ethclient.Dial("https://rinkeby.infura.io")
	require.Nil(t, err)
	signer := types.NewEIP155Signer(big.NewInt(1234))

	privateKey, err := crypto.HexToECDSA("fad9c8855b740a0b7ed4c221dbad0f33a83a49cad6b3fe8d5817ac83d38b6a19")
	if err != nil {
		log.Fatal(err)
	}

	publicKey := privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	require.True(t, ok)

	fromAddress := crypto.PubkeyToAddress(*publicKeyECDSA)
	nonce := uint64(10)

	value := big.NewInt(1000000000000000000) // in wei (1 eth)
	gasLimit := uint64(21000)                // in units
	gasPrice := big.NewInt(10000)

	toAddress := common.HexToAddress("0x4592d8f8d7b001e72cb26a73e4fa1806a51ac79d")
	var data []byte
	tx := types.NewTransaction(nonce, toAddress, value, gasLimit, gasPrice, data)

	signedTx, err := types.SignTx(tx, signer, privateKey)
	require.Nil(t, err)

	gpm := NewGasPriceMonitor(&stubGasPriceOracle{
		gasPrice: big.NewInt(1),
	}, 1*time.Second, big.NewInt(0), nil)
	gpm.gasPrice = big.NewInt(1)

	tm := NewTransactionManager(client, gpm, &accountManager{}, 3*time.Second, 0)

	bi := NewBackend(client, signer, gpm, tm)

	nonceLockBefore := bi.(*backend).nonceManager.getNonceLock(fromAddress)

	err = bi.SendTransaction(context.TODO(), signedTx)
	// "projectID ID not provided" error because we didn't specify an infura "key"
	assert.NotNil(t, err)

	nonceLockAfter := bi.(*backend).nonceManager.getNonceLock(fromAddress)

	assert.Equal(t, nonceLockBefore.nonce, nonceLockAfter.nonce)
}

func TestIsRetryableRemoteCallError(t *testing.T) {
	assert := assert.New(t)

	assert.True(isRetryableRemoteCallError(errors.New("EOF")))
	assert.True(isRetryableRemoteCallError(errors.New("EOF a")))
	assert.True(isRetryableRemoteCallError(errors.New("tls: use of closed connection")))
	assert.True(isRetryableRemoteCallError(errors.New("tls: use of closed connection a")))
	assert.True(isRetryableRemoteCallError(errors.New("unsupported block number")))
	assert.True(isRetryableRemoteCallError(errors.New("unsupported block number a")))

	assert.False(isRetryableRemoteCallError(errors.New("not retryable")))
}

func TestRetryRemoteCall(t *testing.T) {
	assert := assert.New(t)

	// Reduce time between retries
	oldRemoteCallRetrySleep := remoteCallRetrySleep
	defer func() { remoteCallRetrySleep = oldRemoteCallRetrySleep }()
	remoteCallRetrySleep = 50 * time.Millisecond

	// The number of calls to remoteCall
	var numCalls int
	// The call that should succeed
	callSuccess := 1
	remoteCall := func() ([]byte, error) {
		numCalls++

		if numCalls == callSuccess {
			return []byte{}, nil
		}

		return nil, errors.New("EOF")
	}

	calcExpSleepTime := func(retries int) time.Duration {
		total := time.Duration(0)
		for i := 0; i < retries; i++ {
			total = total + (time.Duration(i+1) * remoteCallRetrySleep)
		}
		return total
	}

	b := &backend{}

	for i := 0; i < maxRemoteCallRetries; i++ {
		numCalls = 0
		callSuccess = i + 1

		expSleepTime := calcExpSleepTime(i)

		startTime := time.Now()
		out, err := b.retryRemoteCall(remoteCall)
		endTime := time.Now()

		assert.GreaterOrEqual(endTime.Sub(startTime), expSleepTime)
		assert.Nil(err)
		assert.Equal([]byte{}, out)
		assert.Equal(callSuccess, numCalls)
	}

	numCalls = 0
	callSuccess = maxRemoteCallRetries + 1

	expSleepTime := calcExpSleepTime(maxRemoteCallRetries)

	startTime := time.Now()
	out, err := b.retryRemoteCall(remoteCall)
	endTime := time.Now()

	assert.GreaterOrEqual(endTime.Sub(startTime), expSleepTime)
	assert.EqualError(err, "EOF")
	assert.Equal([]byte(nil), out)
	assert.Equal(maxRemoteCallRetries, numCalls)
}
