package eth

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"sync"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/pm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubTransactionSenderReader struct {
	err             map[string]error
	pending         bool
	tx              *types.Transaction
	receipt         *types.Receipt
	callsToTxByHash int //reflects number of calls to replace()
}

func (stm *stubTransactionSenderReader) SendTransaction(ctx context.Context, tx *types.Transaction) error {
	return stm.err["SendTransaction"]
}

func (stm *stubTransactionSenderReader) TransactionByHash(ctx context.Context, txHash common.Hash) (tx *types.Transaction, isPending bool, err error) {
	stm.callsToTxByHash++
	return stm.tx, stm.pending, stm.err["TransactionByHash"]
}

func (stm *stubTransactionSenderReader) TransactionReceipt(ctx context.Context, txHash common.Hash) (*types.Receipt, error) {
	return stm.receipt, stm.err["TransactionReceipt"]
}

func (stm *stubTransactionSenderReader) CodeAt(ctx context.Context, account common.Address, blockNumber *big.Int) ([]byte, error) {
	return []byte{}, stm.err["CodeAt"]
}

type stubTransactionSigner struct {
	err error
}

func (sig *stubTransactionSigner) SignTx(tx *types.Transaction) (*types.Transaction, error) {
	if sig.err != nil {
		return nil, sig.err
	}
	return tx, nil
}

func TestTxQueue(t *testing.T) {
	assert := assert.New(t)
	tx := types.NewTransaction(1, pm.RandAddress(), big.NewInt(100), 1, big.NewInt(100), pm.RandBytes(32))
	q := transactionQueue{}
	q.add(tx)
	assert.Len(q, 1)
	assert.Equal(tx, q.peek())
	assert.Equal(tx, q.pop())
	assert.Len(q, 0)

	q = transactionQueue{}
	assert.Nil(q.pop())
	assert.Nil(q.peek())
}

func TestTransactionManager_SendTransaction(t *testing.T) {
	assert := assert.New(t)

	eth := &stubTransactionSenderReader{
		err: make(map[string]error),
	}
	q := transactionQueue{}
	tm := &TransactionManager{
		cond:  sync.NewCond(&sync.Mutex{}),
		eth:   eth,
		queue: q,
	}

	// Test error
	expErr := errors.New("SendTransaction error")
	eth.err["SendTransaction"] = expErr

	tx := types.NewTransaction(1, pm.RandAddress(), big.NewInt(100), 100000, big.NewInt(100), pm.RandBytes(68))

	errLogsBefore := glog.Stats.Info.Lines()
	assert.EqualError(
		tm.SendTransaction(context.Background(), tx),
		expErr.Error(),
	)
	errLogsAfter := glog.Stats.Info.Lines()
	assert.Equal(errLogsAfter-errLogsBefore, int64(1))

	// Test no error
	// Adds tx to queue
	qLenBefore := q.length()
	errLogsBefore = glog.Stats.Info.Lines()
	eth.err = nil
	assert.NoError(
		tm.SendTransaction(context.Background(), tx),
	)
	qLenAfter := q.length()
	errLogsAfter = glog.Stats.Info.Lines()
	assert.Equal(errLogsAfter-errLogsBefore, int64(1))
	assert.Equal(qLenAfter, qLenBefore, 1)
	assert.Equal(tm.queue.peek().Hash(), tx.Hash())
}

func TestTransactionManager_Wait(t *testing.T) {
	assert := assert.New(t)

	eth := &stubTransactionSenderReader{
		err: make(map[string]error),
	}
	q := transactionQueue{}
	tm := &TransactionManager{
		cond:      sync.NewCond(&sync.Mutex{}),
		eth:       eth,
		queue:     q,
		txTimeout: 2 * time.Second,
	}

	// Test error
	// This calls bind.WaitMined() on the ethereum client, which will never actually return the actual underlying error and only log it using go-ethereum's custom logger
	// The expected error should thus be 'context deadline exceeded'
	// https://github.com/ethereum/go-ethereum/blob/aa637fd38a379db6da98df0d520fb1c5139a18ce/accounts/abi/bind/util.go#L41
	expErr := errors.New("context deadline exceeded")
	eth.err["TransactionByHash"] = expErr

	tx := types.NewTransaction(1, pm.RandAddress(), big.NewInt(100), 100000, big.NewInt(100), pm.RandBytes(68))

	receipt, err := tm.wait(tx)
	assert.Nil(receipt)
	assert.EqualError(err, expErr.Error())

	// No error, stub a receipt
	eth.receipt = types.NewReceipt(pm.RandHash().Bytes(), false, 100000)
	eth.err = nil

	receipt, err = tm.wait(tx)
	assert.Equal(receipt.Status, uint64(1))
	assert.Equal(receipt.CumulativeGasUsed, uint64(100000))
	assert.Nil(err)
}

func TestTransactionManager_Replace(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	eth := &stubTransactionSenderReader{
		err: make(map[string]error),
	}
	q := transactionQueue{}
	gasPrice := big.NewInt(10)
	gpm := &GasPriceMonitor{
		minGasPrice: big.NewInt(0),
		maxGasPrice: big.NewInt(0),
		gasPrice:    big.NewInt(1),
	}
	tm := &TransactionManager{
		cond:      sync.NewCond(&sync.Mutex{}),
		eth:       eth,
		queue:     q,
		txTimeout: 2 * time.Second,
		gpm:       gpm,
	}

	stubTx := types.NewTransaction(1, pm.RandAddress(), big.NewInt(100), 100000, gasPrice, pm.RandBytes(68))

	// Test eth.TransactionByHash error
	expErr := errors.New("TransactionByHash error")
	eth.err["TransactionByHash"] = expErr

	tx, err := tm.replace(stubTx)
	assert.Nil(tx)
	assert.EqualError(err, expErr.Error())
	eth.err["TransactionByHash"] = nil

	// Test no error - ErrReplacingMinedTx
	eth.pending = false
	tx, err = tm.replace(stubTx)
	assert.Nil(tx)
	assert.EqualError(err, ErrReplacingMinedTx.Error())

	// Test error is ethereum.NotFound - fail at next step
	eth.pending = true
	gpm.maxGasPrice = big.NewInt(1)
	eth.err["TransactionByHash"] = ethereum.NotFound
	tx, err = tm.replace(stubTx)
	assert.Nil(tx)
	assert.EqualError(
		err,
		fmt.Sprintf("replacement gas price exceeds max gas price suggested=%v max=%v", calcReplacementGasPrice(stubTx), gpm.maxGasPrice),
	)
	eth.err["TransactionByHash"] = nil

	// Replacement gas price exceeds max gas price
	// Throw error
	tx, err = tm.replace(stubTx)
	assert.Nil(tx)
	assert.EqualError(
		err,
		fmt.Sprintf("replacement gas price exceeds max gas price suggested=%v max=%v", calcReplacementGasPrice(stubTx), gpm.maxGasPrice),
	)

	// Error signing replacement tx
	expErr = errors.New("SignTx error")
	sig := &stubTransactionSigner{
		err: nil,
	}
	tm.sig = sig
	sig.err = expErr
	gpm.maxGasPrice = big.NewInt(99999)
	tx, err = tm.replace(stubTx)
	assert.Nil(tx)
	assert.EqualError(err, expErr.Error())

	// Test when max gas price is nil - should still return signing replacement tx error
	gpm.maxGasPrice = nil
	tx, err = tm.replace(stubTx)
	assert.Nil(tx)
	assert.EqualError(err, expErr.Error())
	sig.err = nil

	// Error sending replacement tx
	expErr = errors.New("SendTx error")
	eth.err["SendTransaction"] = expErr
	logsBefore := glog.Stats.Info.Lines()
	tx, err = tm.replace(stubTx)
	logsAfter := glog.Stats.Info.Lines()
	assert.EqualError(err, expErr.Error())
	assert.Equal(logsAfter-logsBefore, int64(1))
	eth.err["SendTransaction"] = nil

	// Success
	logsBefore = glog.Stats.Info.Lines()
	tx, err = tm.replace(stubTx)
	logsAfter = glog.Stats.Info.Lines()
	assert.Nil(err)
	expTx := types.NewTransaction(1, *stubTx.To(), stubTx.Value(), 100000, calcReplacementGasPrice(stubTx), stubTx.Data())
	assert.Equal(tx.Hash(), expTx.Hash())
	assert.Equal(logsAfter-logsBefore, int64(1))

	// Replacement gas price lower than suggest gas price
	// Use market gas price
	gpm.gasPrice = big.NewInt(999)
	require.True(gpm.GasPrice().Cmp(calcReplacementGasPrice(stubTx)) > 0)
	logsBefore = glog.Stats.Info.Lines()
	tx, err = tm.replace(stubTx)
	logsAfter = glog.Stats.Info.Lines()
	assert.Nil(err)
	expTx = types.NewTransaction(1, *stubTx.To(), stubTx.Value(), 100000, gpm.gasPrice, stubTx.Data())
	assert.Equal(tx.Hash(), expTx.Hash())
	assert.Equal(logsAfter-logsBefore, int64(1))
}

func TestTransactionManager_CheckTxLoop(t *testing.T) {
	assert := assert.New(t)

	eth := &stubTransactionSenderReader{
		err: make(map[string]error),
	}
	q := transactionQueue{}
	gasPrice := big.NewInt(10)
	gpm := &GasPriceMonitor{
		minGasPrice: big.NewInt(0),
		maxGasPrice: big.NewInt(99999999),
		gasPrice:    big.NewInt(1),
	}
	sig := &stubTransactionSigner{
		err: nil,
	}

	tm := &TransactionManager{
		maxReplacements: 0,
		cond:            sync.NewCond(&sync.Mutex{}),
		eth:             eth,
		queue:           q,
		txTimeout:       2 * time.Second,
		gpm:             gpm,
		sig:             sig,
		quit:            make(chan struct{}),
	}

	eth.pending = true
	receipt := types.NewReceipt(pm.RandHash().Bytes(), false, 100000)
	eth.receipt = receipt

	stubTx := types.NewTransaction(1, pm.RandAddress(), big.NewInt(100), 100000, gasPrice, pm.RandBytes(68))

	go tm.Start()
	defer tm.Stop()

	sink := make(chan *transactionReceipt, 10)
	sub := tm.Subscribe(sink)
	tm.SendTransaction(context.Background(), stubTx)

	event := <-sink
	assert.NotNil(event)
	assert.Nil(event.err)
	sub.Unsubscribe()

	// Wait error no replacements
	eth.receipt = nil
	eth.err["TransactionReceipt"] = context.DeadlineExceeded
	sink = make(chan *transactionReceipt)
	sub = tm.Subscribe(sink)
	tm.SendTransaction(context.Background(), stubTx)
	event = <-sink
	assert.NotNil(event.Receipt)
	assert.Equal(event.originTxHash, stubTx.Hash())
	assert.NotNil(event)
	assert.EqualError(event.err, context.DeadlineExceeded.Error())
	eth.err["TransactionReceipt"] = nil
	sub.Unsubscribe()

	// Wait error, replacements
	// Replace tx error
	tm.maxReplacements = 1
	eth.err["TransactionReceipt"] = context.DeadlineExceeded
	eth.err["TransactionByHash"] = errors.New("TransactionByHash error")
	sink = make(chan *transactionReceipt)
	sub = tm.Subscribe(sink)

	tm.SendTransaction(context.Background(), stubTx)
	event = <-sink
	assert.NotNil(event.Receipt)
	assert.Equal(event.originTxHash, stubTx.Hash())
	assert.EqualError(event.err, eth.err["TransactionByHash"].Error())
	assert.Equal(eth.callsToTxByHash, 1)
	sub.Unsubscribe()

	// Replace multiple times, but fail replacement
	// Should replace only once
	eth.callsToTxByHash = 0
	tm.maxReplacements = 3

	sink = make(chan *transactionReceipt)
	sub = tm.Subscribe(sink)

	tm.SendTransaction(context.Background(), stubTx)
	event = <-sink
	assert.NotNil(event.Receipt)
	assert.Equal(event.originTxHash, stubTx.Hash())
	assert.EqualError(event.err, eth.err["TransactionByHash"].Error())
	assert.Equal(eth.callsToTxByHash, 1)
	assert.LessOrEqual(eth.callsToTxByHash, tm.maxReplacements)
	sub.Unsubscribe()

	// replace multiple times, time out
	// replace 'maxReplacements' times
	eth.callsToTxByHash = 0
	eth.err["TransactionByHash"] = nil
	sink = make(chan *transactionReceipt)
	sub = tm.Subscribe(sink)

	tm.SendTransaction(context.Background(), stubTx)
	event = <-sink
	assert.NotNil(event.Receipt)
	assert.Equal(event.originTxHash, stubTx.Hash())
	assert.EqualError(event.err, context.DeadlineExceeded.Error())
	assert.Equal(eth.callsToTxByHash, tm.maxReplacements)
	sub.Unsubscribe()
}
