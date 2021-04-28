package eth

import (
	"context"
	"errors"
	"math/big"
	"sync"
	"time"

	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/monitor"
)

const gasPricesLength = 3

// GasPriceOracle defines methods for fetching a suggested gas price
// for submitting transactions
type GasPriceOracle interface {
	SuggestGasPrice(ctx context.Context) (*big.Int, error)
}

// GasPriceMonitor polls for gas price updates and updates its
// own view of the current gas price that can be used by others
type GasPriceMonitor struct {
	gpo GasPriceOracle
	// The following fields should be protected by `pollingMu`
	polling         bool
	pollingInterval time.Duration
	cancel          context.CancelFunc
	// pollingMu protects access to polling related fields
	pollingMu sync.Mutex

	// gasPriceMu protects access to gasPrice
	gasPriceMu sync.RWMutex
	// gasPrice is the current gas price to be returned to users, mean of the values in gasPrices
	gasPrice *big.Int
	// gasPrices is a queue of gas prices
	gasPrices *gasPrices
	// update is a channel used to send notifications to a listener
	// when the gas price is updated
	update chan struct{}
}

// NewGasPriceMonitor returns a GasPriceMonitor
func NewGasPriceMonitor(gpo GasPriceOracle, pollingInterval time.Duration) *GasPriceMonitor {
	return &GasPriceMonitor{
		gpo:             gpo,
		pollingInterval: pollingInterval,
		gasPrice:        big.NewInt(0),
		gasPrices:       newGasPrices(gasPricesLength),
	}
}

// GasPrice returns the current gas price
func (gpm *GasPriceMonitor) GasPrice() *big.Int {
	gpm.gasPriceMu.RLock()
	defer gpm.gasPriceMu.RUnlock()

	return gpm.gasPrice
}

// Start starts polling for gas price updates and returns a channel to receive
// notifications of gas price changes
func (gpm *GasPriceMonitor) Start(ctx context.Context) (chan struct{}, error) {
	gpm.pollingMu.Lock()
	defer gpm.pollingMu.Unlock()

	if gpm.polling {
		return nil, errors.New("already polling")
	}

	// Initialize gasPrice before starting to poll
	if err := gpm.fetchAndUpdateGasPrice(ctx); err != nil {
		return nil, err
	}

	gpm.update = make(chan struct{})

	cctx, cancel := context.WithCancel(ctx)
	gpm.cancel = cancel

	ticker := time.NewTicker(gpm.pollingInterval)

	go func(ctx context.Context) {
		for {
			select {
			case <-ticker.C:
				if err := gpm.fetchAndUpdateGasPrice(ctx); err != nil {
					glog.Errorf("error getting gas price: %v", err)
				}
			case <-ctx.Done():
				gpm.pollingMu.Lock()
				gpm.cancel = nil
				gpm.polling = false
				gpm.pollingMu.Unlock()
				return
			}
		}
	}(cctx)

	gpm.polling = true

	return gpm.update, nil
}

// Stop stops polling for gas price updates
func (gpm *GasPriceMonitor) Stop() error {
	gpm.pollingMu.Lock()
	defer gpm.pollingMu.Unlock()

	if !gpm.polling {
		return errors.New("not polling")
	}

	gpm.cancel()
	gpm.cancel = nil
	gpm.polling = false

	// Close the update channel when gpm is stopped
	close(gpm.update)

	return nil
}

func (gpm *GasPriceMonitor) fetchAndUpdateGasPrice(ctx context.Context) error {
	gasPrice, err := gpm.gpo.SuggestGasPrice(ctx)
	if err != nil {
		return err
	}

	// If the suggested gas price is not an outlier and it's not equal to the latest added value
	// Add it to the back of the queue and calculate a new mean
	if back := gpm.gasPrices.back(); back == nil || gasPrice.Cmp(back) != 0 && !gpm.gasPrices.isOutlier(gasPrice) {
		gpm.gasPrices.push(gasPrice)
		gpm.updateGasPrice(gpm.gasPrices.mean())
	}

	if monitor.Enabled {
		monitor.SuggestedGasPrice(gpm.GasPrice())
	}

	return nil
}

func (gpm *GasPriceMonitor) updateGasPrice(gasPrice *big.Int) {
	gpm.gasPriceMu.Lock()
	defer gpm.gasPriceMu.Unlock()

	gpm.gasPrice = gasPrice
}

type gasPrices struct {
	sync.RWMutex
	list   []*big.Int
	length int
}

func newGasPrices(length int) *gasPrices {
	return &gasPrices{
		length: length,
		list:   make([]*big.Int, 0),
	}
}

func (q *gasPrices) front() *big.Int {
	q.RLock()
	defer q.RUnlock()
	if len(q.list) == 0 {
		return nil
	}
	return q.list[0]
}

func (q *gasPrices) back() *big.Int {
	q.RLock()
	defer q.RUnlock()
	if len(q.list) == 0 {
		return nil
	}
	return q.list[len(q.list)-1]
}

func (q *gasPrices) isFull() bool {
	q.RLock()
	defer q.RUnlock()
	return len(q.list) == q.length
}

func (q *gasPrices) push(price *big.Int) {
	isFull := q.isFull()

	q.Lock()
	defer q.Unlock()

	// Queue is not full, simply append to back
	if !isFull {
		q.list = append(q.list, price)
		return
	}

	list := q.list[1:]
	list = append(list, price)
	q.list = list
}

func (q *gasPrices) mean() *big.Int {
	q.RLock()
	defer q.RUnlock()
	sum := big.NewInt(0)

	if len(q.list) == 0 {
		return sum
	}

	for _, v := range q.list {
		sum = sum.Add(sum, v)
	}
	return sum.Div(sum, big.NewInt(int64(len(q.list))))
}

func (q *gasPrices) isOutlier(price *big.Int) bool {
	q.RLock()
	defer q.RUnlock()

	if len(q.list) <= 1 {
		return false
	}

	var (
		variance   = big.NewInt(0)
		length     = big.NewInt(int64(len(q.list)))
		deviations = big.NewInt(3) // The max acceptable std, anything outside will be removed from the set
	)

	mean := q.mean()
	// Calculate variance (sum(x - mean)^2 )/ length
	for _, price := range q.list {
		// Calculate the summation
		x := new(big.Int).Sub(price, mean)
		square := new(big.Int).Mul(x, x)
		variance.Add(variance, square)
	}
	variance.Div(variance, length)
	// Calculate standard deviation from the variance
	sd := new(big.Int).Sqrt(variance)

	deviation := new(big.Int).Mul(deviations, sd)
	lowerBound := new(big.Int).Sub(mean, deviation)
	upperBound := new(big.Int).Add(mean, deviation)

	if price.Cmp(lowerBound) == 1 && price.Cmp(upperBound) == -1 {
		return false
	}

	return true
}
