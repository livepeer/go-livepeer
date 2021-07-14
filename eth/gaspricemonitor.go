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

	// gasPriceMu protects access to gasPrice and minGasPrice
	gasPriceMu sync.RWMutex
	// gasPrice is the current gas price to be returned to users
	gasPrice *big.Int
	// minGasPrice is the minimum gas price below which polled values will be discarded
	minGasPrice *big.Int
	// maxGasPrice is the max acceptable gas price defined by the user
	maxGasPrice *big.Int

	// update is a channel used to send notifications to a listener
	// when the gas price is updated
	update chan struct{}
}

// NewGasPriceMonitor returns a GasPriceMonitor
func NewGasPriceMonitor(gpo GasPriceOracle, pollingInterval time.Duration, minGasPrice *big.Int, maxGasPrice *big.Int) *GasPriceMonitor {
	minGasP := big.NewInt(0)
	if minGasPrice != nil {
		minGasP = minGasPrice
	}
	if monitor.Enabled {
		monitor.MinGasPrice(minGasP)
		if maxGasPrice != nil {
			monitor.MaxGasPrice(maxGasPrice)
		}
	}

	return &GasPriceMonitor{
		gpo:             gpo,
		pollingInterval: pollingInterval,
		gasPrice:        big.NewInt(0),
		minGasPrice:     minGasP,
		maxGasPrice:     maxGasPrice,
	}
}

// GasPrice returns the current gas price
func (gpm *GasPriceMonitor) GasPrice() *big.Int {
	gpm.gasPriceMu.RLock()
	defer gpm.gasPriceMu.RUnlock()

	if gpm.gasPrice.Cmp(gpm.minGasPrice) < 0 {
		return gpm.minGasPrice
	}

	return gpm.gasPrice
}

func (gpm *GasPriceMonitor) SetMinGasPrice(minGasPrice *big.Int) {
	gpm.gasPriceMu.Lock()
	defer gpm.gasPriceMu.Unlock()
	gpm.minGasPrice = minGasPrice

	if monitor.Enabled {
		monitor.MinGasPrice(minGasPrice)
	}
}

func (gpm *GasPriceMonitor) MinGasPrice() *big.Int {
	gpm.gasPriceMu.RLock()
	defer gpm.gasPriceMu.RUnlock()
	return gpm.minGasPrice
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

func (gpm *GasPriceMonitor) SetMaxGasPrice(gp *big.Int) {
	gpm.gasPriceMu.Lock()
	defer gpm.gasPriceMu.Unlock()
	gpm.maxGasPrice = gp

	if monitor.Enabled {
		monitor.MaxGasPrice(gp)
	}
}

func (gpm *GasPriceMonitor) MaxGasPrice() *big.Int {
	gpm.gasPriceMu.RLock()
	defer gpm.gasPriceMu.RUnlock()
	return gpm.maxGasPrice
}

func (gpm *GasPriceMonitor) fetchAndUpdateGasPrice(ctx context.Context) error {
	gasPrice, err := gpm.gpo.SuggestGasPrice(ctx)
	if err != nil {
		return err
	}

	if gasPrice.Cmp(gpm.minGasPrice) >= 0 {
		gpm.updateGasPrice(gasPrice)

		if monitor.Enabled {
			monitor.SuggestedGasPrice(gasPrice)
		}
	}

	return nil
}

func (gpm *GasPriceMonitor) updateGasPrice(gasPrice *big.Int) {
	gpm.gasPriceMu.Lock()
	defer gpm.gasPriceMu.Unlock()

	gpm.gasPrice = gasPrice
}
