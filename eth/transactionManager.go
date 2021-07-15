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
	*types.Receipt
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
	sendErr := tm.eth.SendTransaction(ctx, tx)

	txLog, err := newTxLog(tx)
	if err != nil {
		txLog.method = "unknown"
	}

	if sendErr != nil {
		glog.Infof("\n%vEth Transaction%v\n\nInvoking transaction: \"%v\". Inputs: \"%v\"   \nTransaction Failed: %v\n\n%v\n", strings.Repeat("*", 30), strings.Repeat("*", 30), txLog.method, txLog.inputs, sendErr, strings.Repeat("*", 75))
		return sendErr
	}

	// Add transaction to queue
	tm.cond.L.Lock()
	tm.queue.add(tx)
	tm.cond.L.Unlock()
	tm.cond.Signal()

	glog.Infof("\n%vEth Transaction%v\n\nInvoking transaction: \"%v\". Inputs: \"%v\"  Hash: \"%v\". \n\n%v\n", strings.Repeat("*", 30), strings.Repeat("*", 30), txLog.method, txLog.inputs, tx.Hash().String(), strings.Repeat("*", 75))

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

	gasPrice := calcReplacementGasPrice(tx)

	suggestedGasPrice := tm.gpm.GasPrice()

	// If the suggested gas price is higher than the bumped gas price, use the suggested gas price
	// This is to account for any wild market gas price increases between the time of the original tx submission and time
	// of replacement tx submission
	// Note: If the suggested gas price is lower than the bumped gas price because market gas prices have dropped
	// since the time of the original tx submission we cannot use the lower suggested gas price and we still need to use
	// the bumped gas price in order to properly replace a still pending tx
	if suggestedGasPrice.Cmp(gasPrice) == 1 {
		gasPrice = suggestedGasPrice
	}

	// Bump gas price exceeds max gas price, return early
	max := tm.gpm.MaxGasPrice()
	if gasPrice.Cmp(max) > 0 {
		return nil, fmt.Errorf("replacement gas price exceeds max gas price suggested=%v max=%v", gasPrice, max)
	}

	// Replacement raw tx uses same fields as old tx (reusing the same nonce is crucial) except the gas price is updated
	newRawTx := types.NewTransaction(tx.Nonce(), *tx.To(), tx.Value(), tx.Gas(), gasPrice, tx.Data())

	newSignedTx, err := tm.sig.SignTx(newRawTx)
	if err != nil {
		return nil, err
	}

	sendErr := tm.eth.SendTransaction(context.Background(), newSignedTx)
	txLog, err := newTxLog(tx)
	if err != nil {
		txLog.method = "unknown"
	}
	if sendErr == nil {
		glog.Infof("\n%vEth Transaction%v\n\nReplacement transaction: \"%v\".  Gas Price: %v \nTransaction Failed: %v\n\n%v\n", strings.Repeat("*", 30), strings.Repeat("*", 30), txLog.method, newSignedTx.GasPrice().String(), err, strings.Repeat("*", 75))
	} else {
		glog.Infof("\n%vEth Transaction%v\n\nReplacement transaction: \"%v\".  Hash: \"%v\".  Gas Price: %v \n\n%v\n", strings.Repeat("*", 30), strings.Repeat("*", 30), txLog.method, newSignedTx.Hash().String(), newSignedTx.GasPrice().String(), strings.Repeat("*", 75))
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

		var (
			receipt *types.Receipt
			err     error
		)

		receipt, err = tm.wait(tx)

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

		tm.feed.Send(&transactionReceipt{
			Receipt: receipt,
			err:     err,
		})

	}
}

// Updated gas price must be at least 10% greater than the gas price used for the original transaction in order
// to submit a replacement transaction with the same nonce. 10% is not defined by the protocol, but is the default required price bump
// used by many clients: https://github.com/ethereum/go-ethereum/blob/01a7e267dc6d7bbef94882542bbd01bd712f5548/core/tx_pool.go#L148
// We add a little extra in addition to the 10% price bump just to be sure
func calcReplacementGasPrice(tx *types.Transaction) *big.Int {
	return new(big.Int).Add(
		new(big.Int).Add(
			tx.GasPrice(),
			new(big.Int).Div(
				tx.GasPrice(),
				big.NewInt(10),
			),
		),
		big.NewInt(10),
	)
}
