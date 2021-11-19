package eth

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/common"
)

// The default price bump required by geth is 10%
// We add a little extra in addition to the 10% price bump just to be safe
// priceBump is a % value from 0-100
const priceBump uint64 = 11

type transactionSenderReader interface {
	ethereum.TransactionSender
	ethereum.TransactionReader
	// Required for bind.DeployBackend argument in bind.WaitMined
	CodeAt(context.Context, ethcommon.Address, *big.Int) ([]byte, error)
}

type transactionSigner interface {
	SignTx(tx *types.Transaction) (*types.Transaction, error)
}

type TransactionManager struct {
	txTimeout       time.Duration
	maxReplacements int

	queue transactionQueue

	// subscriptions
	feed  event.Feed
	scope event.SubscriptionScope

	eth transactionSenderReader
	gpm *GasPriceMonitor
	sig transactionSigner

	cond *sync.Cond

	quit chan struct{}
}

type transactionQueue []*types.Transaction

type transactionReceipt struct {
	originTxHash ethcommon.Hash
	types.Receipt
	err error
}

func (tq *transactionQueue) add(tx *types.Transaction) {
	*tq = append(*tq, tx)
}

func (tq *transactionQueue) pop() *types.Transaction {
	if tq.length() == 0 {
		return nil
	}
	tx := (*tq)[0]
	*tq = (*tq)[1:]
	return tx
}

func (tq transactionQueue) length() int {
	return len(tq)
}

func (tq transactionQueue) peek() *types.Transaction {
	if tq.length() == 0 {
		return nil
	}
	return tq[0]
}

func NewTransactionManager(eth transactionSenderReader, gpm *GasPriceMonitor, signer transactionSigner, txTimeout time.Duration, maxReplacements int) *TransactionManager {
	return &TransactionManager{
		cond:            sync.NewCond(&sync.Mutex{}),
		txTimeout:       txTimeout,
		maxReplacements: maxReplacements,
		eth:             eth,
		gpm:             gpm,
		sig:             signer,
		queue:           transactionQueue{},
		quit:            make(chan struct{}),
	}
}

func (tm *TransactionManager) SendTransaction(ctx context.Context, tx *types.Transaction) error {
	adjustedRawTx := tm.newAdjustedTx(tx)
	adjustedTx, err := tm.sig.SignTx(adjustedRawTx)
	if err != nil {
		glog.Infof("%s\n", err.Error())
		return err
	}

	sendErr := tm.eth.SendTransaction(ctx, adjustedTx)

	txLog, err := newTxLog(adjustedTx)
	if err != nil {
		txLog.method = "unknown"
	}

	if sendErr != nil {
		glog.Infof("\n%vEth Transaction%v\n\nInvoking transaction: \"%v\". Inputs: \"%v\"   \nTransaction Failed: %v\n\n%v\n", strings.Repeat("*", 30), strings.Repeat("*", 30), txLog.method, txLog.inputs, sendErr, strings.Repeat("*", 75))
		return sendErr
	}

	// Add transaction to queue
	tm.cond.L.Lock()
	tm.queue.add(adjustedTx)
	tm.cond.L.Unlock()
	tm.cond.Signal()

	glog.Infof("\n%vEth Transaction%v\n\nInvoking transaction: \"%v\". Inputs: \"%v\"  Hash: \"%v\". \n\n%v\n", strings.Repeat("*", 30), strings.Repeat("*", 30), txLog.method, txLog.inputs, adjustedTx.Hash().String(), strings.Repeat("*", 75))

	return nil
}

func (tm *TransactionManager) Subscribe(sink chan<- *transactionReceipt) event.Subscription {
	return tm.scope.Track(tm.feed.Subscribe(sink))
}

func (tm *TransactionManager) Start() {
	tm.checkTxLoop()
}

func (tm *TransactionManager) Stop() {
	tm.scope.Close()
	close(tm.quit)
}

func (tm *TransactionManager) wait(tx *types.Transaction) (*types.Receipt, error) {
	ctx, cancel := context.WithTimeout(context.Background(), tm.txTimeout)
	defer cancel()

	return bind.WaitMined(ctx, tm.eth, tx)
}

func (tm *TransactionManager) replace(tx *types.Transaction) (*types.Transaction, error) {
	_, pending, err := tm.eth.TransactionByHash(context.Background(), tx.Hash())
	// Only return here if the error is not related to the tx not being found
	// Presumably the provided tx was already broadcasted at some point, so even if for some reason the
	// node being used cannot find it, the originally broadcasted tx is still valid and might be sitting somewhere
	if err != nil && err != ethereum.NotFound {
		return nil, err
	}
	// If tx was found
	// If `pending` is false, the tx was mined and included in a block
	if err == nil && !pending {
		return nil, ErrReplacingMinedTx
	}

	newRawTx := newReplacementTx(tx)

	// Bump gas price exceeds max gas price, return early
	max := tm.gpm.MaxGasPrice()
	newGasPrice := newRawTx.GasFeeCap()
	if max != nil && newGasPrice.Cmp(max) > 0 {
		return nil, fmt.Errorf("replacement gas price exceeds max gas price suggested=%v max=%v", newGasPrice, max)
	}

	newSignedTx, err := tm.sig.SignTx(newRawTx)
	if err != nil {
		return nil, err
	}

	sendErr := tm.eth.SendTransaction(context.Background(), newSignedTx)
	txLog, err := newTxLog(tx)
	if err != nil {
		txLog.method = "unknown"
	}
	if sendErr != nil {
		glog.Infof("\n%vEth Transaction%v\n\nReplacement transaction: \"%v\".  Priority Fee: %v Max Fee: %v \nTransaction Failed: %v\n\n%v\n", strings.Repeat("*", 30), strings.Repeat("*", 30), txLog.method, newSignedTx.GasTipCap().String(), newSignedTx.GasFeeCap().String(), sendErr, strings.Repeat("*", 75))
	} else {
		glog.Infof("\n%vEth Transaction%v\n\nReplacement transaction: \"%v\".  Hash: \"%v\".  Priority Fee: %v Max Fee: %v \n\n%v\n", strings.Repeat("*", 30), strings.Repeat("*", 30), txLog.method, newSignedTx.Hash().String(), newSignedTx.GasTipCap().String(), newSignedTx.GasFeeCap().String(), strings.Repeat("*", 75))
	}

	return newSignedTx, sendErr
}

func (tm *TransactionManager) checkTxLoop() {
	for {
		tm.cond.L.Lock()
		for tm.queue.length() == 0 {
			tm.cond.Wait()

			select {
			case <-tm.quit:
				tm.cond.L.Unlock()
				glog.V(common.DEBUG).Info("Stopping transaction manager")
				return
			default:
			}
		}

		tx := tm.queue.pop()
		tm.cond.L.Unlock()

		originHash := tx.Hash()

		var txReceipt types.Receipt

		receipt, err := tm.wait(tx)

		// context.DeadlineExceeded indicates that we hit the txTimeout
		// If we hit the txTimeout, replace the tx up to maxReplacements times
		for i := 0; err == context.DeadlineExceeded && i < tm.maxReplacements; i++ {
			tx, err = tm.replace(tx)
			// Do not attempt additional replacements if there was an error submitting this
			// replacement tx
			if err != nil {
				break
			}
			receipt, err = tm.wait(tx)
		}

		if receipt == nil {
			txReceipt = types.Receipt{}
		} else {
			txReceipt = *(receipt)
		}

		tm.feed.Send(&transactionReceipt{
			originTxHash: originHash,
			Receipt:      txReceipt,
			err:          err,
		})

	}
}

func (tm *TransactionManager) newAdjustedTx(tx *types.Transaction) *types.Transaction {
	baseTx := newDynamicFeeTx(tx)
	if tm.gpm.MaxGasPrice() != nil {
		baseTx.GasFeeCap = tm.gpm.MaxGasPrice()
	}

	return types.NewTx(baseTx)
}

func applyPriceBump(val *big.Int, priceBump uint64) *big.Int {
	a := big.NewInt(100 + int64(priceBump))
	b := new(big.Int).Mul(a, val)
	return b.Div(b, big.NewInt(100))
}

func newReplacementTx(tx *types.Transaction) *types.Transaction {
	baseTx := newDynamicFeeTx(tx)
	baseTx.GasFeeCap = applyPriceBump(tx.GasFeeCap(), priceBump)
	baseTx.GasTipCap = applyPriceBump(tx.GasTipCap(), priceBump)

	return types.NewTx(baseTx)
}

func newDynamicFeeTx(tx *types.Transaction) *types.DynamicFeeTx {
	return &types.DynamicFeeTx{
		To:        tx.To(),
		Nonce:     tx.Nonce(),
		GasFeeCap: tx.GasFeeCap(),
		GasTipCap: tx.GasTipCap(),
		Gas:       tx.Gas(),
		Value:     tx.Value(),
		Data:      tx.Data(),
	}
}
